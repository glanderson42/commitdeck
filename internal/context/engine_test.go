package context

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/issuetracker"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/workspace"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureGit struct{ gitservice.CLI }

func (fixtureGit) Status(string) (*gitservice.Repository, error) {
	return &gitservice.Repository{Path: "/nonexistent/commitdeck-test", Branch: "PROJ-421-retry", Dirty: true, Files: []gitservice.File{{Path: "retry_test.go", Status: "M"}, {Path: ".env", Status: "M"}, {Path: "private.pem", Status: "M"}}}, nil
}
func fixture(t *testing.T) (*Engine, workspace.Workspace) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	w, err := db.CreateWorkspace(workspace.Workspace{ExternalKey: "PROJ-421", Title: "Invoice retry", RepoPath: "/repo", WorktreePath: "/ticket", Branch: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveNote(w.ID, "Use invoice ID as seed. API_KEY=sk-test-secret-value"); err != nil {
		t.Fatal(err)
	}
	for _, todo := range []workspace.Todo{{WorkspaceID: w.ID, Title: "Add integration tests"}, {WorkspaceID: w.ID, Title: "Understand flow", Completed: true}} {
		if _, err = db.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.PutCache("jira", w.ID, issuetracker.Issue{Description: "Retry payments", AcceptanceCriteria: "At most three retries"}); err != nil {
		t.Fatal(err)
	}
	if err = db.PutCache("pr", w.ID, pullrequest.PullRequest{Number: 842, Title: "Retries", State: "OPEN", ReviewState: "CHANGES_REQUESTED", UnresolvedComments: []pullrequest.Feedback{{Author: "Peter", Body: "Make delay configurable", Path: "retry.go"}}}); err != nil {
		t.Fatal(err)
	}
	return &Engine{DB: db, Git: fixtureGit{}}, w
}
func TestSourcesPurposeAndStableHash(t *testing.T) {
	e, w := fixture(t)
	a, err := e.BuildWorkspaceContext(context.Background(), w.ID, Defaults(), "testing")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.BuildWorkspaceContext(context.Background(), w.ID, Defaults(), "testing")
	if err != nil || a.Hash != b.Hash {
		t.Fatal("unstable hash", err)
	}
	for _, want := range []string{"PROJ-421", "At most three retries", "- [ ] Add integration tests", "- [x] Understand flow", "Use invoice ID", "Make delay configurable", "retry_test.go"} {
		if !strings.Contains(a.Markdown, want) {
			t.Fatal("missing", want, a.Markdown)
		}
	}
	for _, secret := range []string{"sk-test-secret-value", "private.pem", "M .env"} {
		if strings.Contains(a.Markdown, secret) {
			t.Fatal("secret source leaked", secret)
		}
	}
	if strings.Index(a.Markdown, "## Git") > strings.Index(a.Markdown, "## Notes") {
		t.Fatal("testing purpose did not prioritize Git")
	}
	opts := Defaults()
	opts.IncludeNotes = false
	opts.IncludeIssue = false
	opts.IncludeGit = false
	c, err := e.BuildWorkspaceContext(context.Background(), w.ID, opts, "review")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Markdown, "Use invoice ID") || strings.Contains(c.Markdown, "At most three retries") {
		t.Fatal("source selection ignored")
	}
	if !strings.Contains(c.Markdown, "Make delay configurable") {
		t.Fatal("review feedback missing")
	}
}
func TestBudgetPreservesFeedbackAndMarksTruncation(t *testing.T) {
	e, w := fixture(t)
	if err := e.DB.PutCache("jira", w.ID, issuetracker.Issue{Description: strings.Repeat("Huge ticket ", 10000), AcceptanceCriteria: "At most three retries"}); err != nil {
		t.Fatal(err)
	}
	opts := Defaults()
	opts.MaxCharacters = 3000
	c, err := e.BuildWorkspaceContext(context.Background(), w.ID, opts, "implementation")
	if err != nil {
		t.Fatal(err)
	}
	if c.CharacterCount > 3000 || len([]rune(c.Markdown)) > 3000 {
		t.Fatal("budget exceeded", c.CharacterCount)
	}
	if !strings.Contains(c.Markdown, "Make delay configurable") || !strings.Contains(c.Markdown, "TRUNCATED") {
		t.Fatal("critical feedback silently lost", c.Markdown)
	}
	if !strings.Contains(c.Markdown, "Add integration tests") {
		t.Fatal("long description erased remaining tasks", c.Markdown)
	}
	if c.Hash == "" || len(c.Truncated) == 0 {
		t.Fatal("metadata missing")
	}
}
func TestSnapshotInjectionAndExplicitRefresh(t *testing.T) {
	e, w := fixture(t)
	s, err := e.DB.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", Title: "Implementation", Purpose: "implementation"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.BuildWorkspaceContext(context.Background(), w.ID, Defaults(), s.Purpose)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Save(s, c); err != nil {
		t.Fatal(err)
	}
	first, id, err := e.Prepare(context.Background(), s, "Implement")
	if err != nil || !strings.Contains(first, c.Markdown) {
		t.Fatal("snapshot not injected", err)
	}
	if err = e.MarkSent(id); err != nil {
		t.Fatal(err)
	}
	s.ExternalSessionID = "thread-1"
	next, _, err := e.Prepare(context.Background(), s, "Continue")
	if err != nil || next != "Continue" {
		t.Fatal("context repeated on resume", err)
	}
	if err = e.DB.SaveNote(w.ID, "Updated decision"); err != nil {
		t.Fatal(err)
	}
	fresh, err := e.BuildWorkspaceContext(context.Background(), w.ID, Defaults(), s.Purpose)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Save(s, fresh); err != nil {
		t.Fatal(err)
	}
	next, _, err = e.Prepare(context.Background(), s, "Continue")
	if err != nil || !strings.Contains(next, "Updated decision") {
		t.Fatal("explicit refresh not injected", err)
	}
}

func TestLegacySessionDoesNotReceiveHiddenContext(t *testing.T) {
	e, w := fixture(t)
	s, err := e.DB.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", ExternalSessionID: "existing"})
	if err != nil {
		t.Fatal(err)
	}
	prompt, id, err := e.Prepare(context.Background(), s, "Continue")
	if err != nil || prompt != "Continue" || id != "" {
		t.Fatal("hidden legacy context", prompt, id, err)
	}
}

func TestGitContextSummarizesChangesWithoutSensitivePaths(t *testing.T) {
	e, w := fixture(t)
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if b, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatal(string(b), err)
		}
	}
	run("init")
	file := filepath.Join(repo, "retry.go")
	if err := os.WriteFile(file, []byte("package retry\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "retry.go")
	run("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Initial")
	if err := os.WriteFile(file, []byte("package retry\nfunc Retry() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("SECRET=never-collect-this"), 0600); err != nil {
		t.Fatal(err)
	}
	w.RepoPath = repo
	w.WorktreePath = repo
	if _, err := e.DB.UpdateWorkspace(w); err != nil {
		t.Fatal(err)
	}
	e.Git = gitservice.CLI{}
	generated, err := e.BuildWorkspaceContext(context.Background(), w.ID, Defaults(), "review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(generated.Markdown, "1 insertion(+)") {
		t.Fatal("missing short diff summary", generated.Markdown)
	}
	if strings.Contains(generated.Markdown, "never-collect-this") || strings.Contains(generated.Markdown, "M .env") {
		t.Fatal("sensitive file included")
	}
}
