package discovery

import (
	"commitdeck/internal/database"
	"commitdeck/internal/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderMetadataAndWorkspaceMatch(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	claude := ParseClaude(strings.NewReader(`{"type":"user","sessionId":"native-123","cwd":"`+dir+`","gitBranch":"PROJ-421-retry","message":{"content":"must not import chat or token"}}`+"\n"+`{"type":"summary","summary":"PROJ-421 retry investigation"}`), "native-123")
	if claude.CWD != dir || claude.Branch != "PROJ-421-retry" || strings.Contains(claude.Title, "token") {
		t.Fatal(claude)
	}
	if Score(claude, workspace.Workspace{WorktreePath: dir}) < 100 {
		t.Fatal("worktree not matched")
	}
	if Score(claude, workspace.Workspace{ExternalKey: "OTHER-1", Branch: "unrelated"}) != 0 {
		t.Fatal("false match")
	}
	rows, err := ParseOpenCode([]byte(`[{"id":"ses_test123","title":"Tests","directory":"/ticket","time":{"updated":1750000000000}}]`))
	if err != nil || len(rows) != 1 || rows[0].CWD != "/ticket" || !rows[0].ResumeSupported {
		t.Fatal(rows, err)
	}
	if _, err = ParseOpenCode([]byte(`{"version":"changed"}`)); err == nil {
		t.Fatal("unknown format accepted")
	}
}

func BenchmarkPreparedDiscoveryMatching(b *testing.B) {
	root := b.TempDir()
	paths := pathCache{}
	candidates := make([]candidate, 50)
	for i := range candidates {
		candidates[i] = prepareCandidate(workspace.Workspace{RepoPath: root, WorktreePath: root, ExternalKey: "PROJ-42"}, paths)
	}
	sessions := make([]Session, 2000)
	for i := range sessions {
		sessions[i] = Session{CWD: paths.resolve(root), Title: "PROJ-42 investigate"}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, s := range sessions {
			for _, c := range candidates {
				if c.score(s) != 160 {
					b.Fatal("matching semantics changed")
				}
			}
		}
	}
}

func TestPreparedMatcherCachesPathsAndPreservesTicketBoundaries(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "ticket")
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	paths := pathCache{}
	c := prepareCandidate(workspace.Workspace{RepoPath: root, WorktreePath: worktree, ExternalKey: "PROJ-42", Branch: "PROJ-42-retry"}, paths)
	s := Session{CWD: paths.resolve(filepath.Join(worktree, "src")), RepoPath: paths.resolve(root), Title: "Investigate PROJ-42", Branch: "PROJ-42-retry"}
	if got := c.score(s); got != 190 {
		t.Fatalf("nested match = %d, want 190", got)
	}
	if len(paths) != 3 {
		t.Fatalf("resolved %d paths, want 3 unique paths", len(paths))
	}
	// Matching prepared values does not touch the filesystem again.
	if err := os.Rename(worktree, worktree+"-moved"); err != nil {
		t.Fatal(err)
	}
	if paths.resolve(worktree) != c.workspace.WorktreePath || c.score(s) != 190 {
		t.Fatal("prepared paths were resolved again")
	}
	for _, title := range []string{"PROJ-420", "XPROJ-42", "PROJ-42suffix"} {
		if got := c.score(Session{Title: title}); got != 0 {
			t.Fatalf("false ticket match %q: %d", title, got)
		}
	}
	if got := c.score(Session{Title: "(proj-42): retry"}); got != 20 {
		t.Fatal(got)
	}
	if got := c.score(Session{CWD: filepath.Join(c.workspace.RepoPath, "ticket-sibling")}); got != 40 {
		t.Fatalf("sibling matched as worktree: %d", got)
	}
	if paths.resolve(filepath.Join(root, "missing")) != "" {
		t.Fatal("missing path matched")
	}
}
func TestImportIsExplicitScopedAndDuplicateSafe(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Import"})
	service := New(db)
	source := Session{Provider: "codex", ExternalSessionID: "native-123", Title: "History", CWD: "/original", ResumeSupported: false, Source: "fixture"}
	service.found[key(source)] = source
	if _, err = service.Import("codex", "unknown", w.ID); err == nil {
		t.Fatal("undiscovered import")
	}
	session, err := service.Import("codex", source.ExternalSessionID, w.ID)
	if err != nil || !session.Imported || session.ResumeSupported || session.ExternalSessionID != "native-123" || session.SourceCWD != "/original" {
		t.Fatal(session, err)
	}
	if _, err = service.Import("codex", source.ExternalSessionID, w.ID); err == nil {
		t.Fatal("duplicate import")
	}
	sessions, _ := db.Sessions(w.ID)
	if len(sessions) != 1 {
		t.Fatal(sessions)
	}
}
