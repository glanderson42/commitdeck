package app

import (
	"commitdeck/internal/git"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type blockingPRProvider struct {
	pullrequest.PullRequestService
	stage   string
	started chan struct{}
	release chan struct{}
	finds   atomic.Int32
}

func (p *blockingPRProvider) wait(ctx context.Context) error {
	close(p.started)
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *blockingPRProvider) FindForBranch(ctx context.Context, _ git.Repository, _ string) (*pullrequest.PullRequest, error) {
	n := p.finds.Add(1)
	if (p.stage == "find" && n == 1) || (p.stage == "refresh" && n == 2) {
		if err := p.wait(ctx); err != nil {
			return nil, err
		}
	}
	return &pullrequest.PullRequest{Number: 1, State: "OPEN"}, nil
}
func (p *blockingPRProvider) Write(ctx context.Context, _ git.Repository, _ *pullrequest.PullRequest, _ pullrequest.Action) (string, error) {
	if p.stage == "write" {
		if err := p.wait(ctx); err != nil {
			return "", err
		}
	}
	return "https://github.com/example/repo/pull/1", nil
}

func TestRemotePRCallsDoNotBlockOtherWorkspaceLifecycle(t *testing.T) {
	for _, stage := range []string{"find", "write", "refresh"} {
		t.Run(stage, func(t *testing.T) {
			a, err := Open(t.TempDir(), func(any) {})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			repo := t.TempDir()
			if b, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
				t.Fatal(string(b), err)
			}
			w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Remote", RepoPath: repo})
			if err != nil {
				t.Fatal(err)
			}
			other, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Local"})
			if err != nil {
				t.Fatal(err)
			}
			provider := &blockingPRProvider{stage: stage, started: make(chan struct{}), release: make(chan struct{})}
			a.PR = provider
			params, _ := json.Marshal(map[string]any{"id": w.ID, "action": "comment", "body": "Test", "confirm": true})
			done := make(chan error, 1)
			go func() {
				_, err := a.Call(ctx, "pr.write", params)
				done <- err
			}()
			select {
			case <-provider.started:
			case <-ctx.Done():
				t.Fatal("remote fixture did not start")
			}
			archived := make(chan error, 1)
			go func() {
				params, _ := json.Marshal(map[string]any{"id": other.ID, "archived": true})
				_, err := a.Call(ctx, "workspace.archive", params)
				archived <- err
			}()
			var blocked bool
			select {
			case err := <-archived:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(2 * time.Second):
				blocked = true
				t.Error("unrelated workspace blocked by GitHub network call")
			}
			close(provider.release)
			if err := <-done; err != nil {
				t.Error(err)
			}
			if blocked {
				<-archived
			}
		})
	}
}

func TestPRRefreshCannotResurrectArchivedWorkspace(t *testing.T) {
	a, err := Open(t.TempDir(), func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := t.TempDir()
	if b, e := exec.Command("git", "init", repo).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Archive race", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	provider := &blockingPRProvider{stage: "write", started: make(chan struct{}), release: make(chan struct{})}
	a.PR = provider
	params, _ := json.Marshal(map[string]any{"id": w.ID, "action": "comment", "body": "Test", "confirm": true})
	done := make(chan error, 1)
	go func() {
		_, e := a.Call(ctx, "pr.write", params)
		done <- e
	}()
	select {
	case <-provider.started:
	case <-ctx.Done():
		t.Fatal("GitHub write did not start")
	}
	archiveParams, _ := json.Marshal(map[string]any{"id": w.ID, "archived": true})
	if _, err = a.Call(ctx, "workspace.archive", archiveParams); err != nil {
		close(provider.release)
		t.Fatal("archive failed while remote write was in flight:", err)
	}
	close(provider.release)
	if err = <-done; err != nil {
		t.Fatal("PR write failed:", err)
	}
	stored, err := a.DB.Workspace(w.ID)
	if err != nil || !stored.Archived || stored.Lifecycle != "ARCHIVED" {
		t.Fatalf("PR refresh overwrote archive state: %+v, %v", stored, err)
	}
}

func TestJiraWriteSuccessSurvivesRefreshFailures(t *testing.T) {
	for _, stage := range []string{"cache", "workspace", "fetch", "remote"} {
		t.Run(stage, func(t *testing.T) {
			a, err := Open(t.TempDir(), func(any) {})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Retry", ExternalKey: "PROJ-421"})
			if err != nil {
				t.Fatal(err)
			}
			var writes atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPost {
					writes.Add(1)
					if stage == "remote" {
						res.WriteHeader(500)
					} else {
						res.WriteHeader(201)
					}
					return
				}
				if stage == "fetch" {
					res.WriteHeader(500)
					return
				}
				_ = json.NewEncoder(res).Encode(map[string]any{"id": "421", "key": "PROJ-421", "fields": map[string]any{"summary": "Retry", "status": map[string]string{"name": "In Review"}}})
			}))
			defer server.Close()
			transport := http.DefaultTransport
			http.DefaultTransport = server.Client().Transport
			defer func() { http.DefaultTransport = transport }()
			a.Secrets = &memorySecrets{token: "synthetic-test-token"}
			settings, _ := json.Marshal(Settings{JiraBaseURL: server.URL, JiraEmail: "dev@example.test"})
			if err = a.DB.SetSetting("integrations", string(settings)); err != nil {
				t.Fatal(err)
			}
			if stage == "cache" {
				_, err = a.DB.Exec("CREATE TRIGGER reject_cache BEFORE INSERT ON jira_cache BEGIN SELECT RAISE(FAIL, 'synthetic cache failure'); END")
			} else if stage == "workspace" {
				_, err = a.DB.Exec("CREATE TRIGGER reject_status BEFORE UPDATE OF status ON workspaces BEGIN SELECT RAISE(FAIL, 'synthetic workspace failure'); END")
			}
			if err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(map[string]any{"id": w.ID, "action": "comment", "body": "Implementation complete", "confirm": true})
			result, err := a.Call(context.Background(), "jira.write", params)
			if writes.Load() != 1 {
				t.Fatalf("remote writes = %d, want 1", writes.Load())
			}
			if stage == "remote" {
				if err == nil {
					t.Fatal("remote failure reported as success")
				}
				return
			}
			if err != nil {
				t.Fatalf("successful remote action reported as failure: %v", err)
			}
			warning, ok := result.(map[string]string)
			if !ok || warning["status"] != "applied" || warning["refreshStatus"] != "failed" || !strings.Contains(warning["warning"], "Jira action succeeded") {
				t.Fatal(result)
			}
		})
	}
}

type branchProvider struct {
	pullrequest.PullRequestService
	branch string
}

func (p branchProvider) DefaultBranch(context.Context, git.Repository) (string, error) {
	return p.branch, nil
}

func TestPRTemplateUsesRepositoryDefaultOrManualInput(t *testing.T) {
	a, err := Open(t.TempDir(), func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	repo := t.TempDir()
	if b, e := exec.Command("git", "init", repo).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Retry", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(map[string]string{"id": w.ID})
	for _, branch := range []string{"main", "master", "release/trunk", ""} {
		a.PR = branchProvider{branch: branch}
		v, e := a.Call(context.Background(), "pr.template", params)
		if e != nil {
			t.Fatal(e)
		}
		result := v.(map[string]string)
		if result["base"] != branch {
			t.Fatal(result)
		}
		if branch == "" && result["baseSource"] != "manual" {
			t.Fatal(result)
		}
	}
	if b, e := exec.Command("git", "-C", repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/legacy-default").CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	a.PR = branchProvider{}
	v, err := a.Call(context.Background(), "pr.template", params)
	if err != nil {
		t.Fatal(err)
	}
	result := v.(map[string]string)
	if result["base"] != "legacy-default" || result["baseSource"] != "cached origin HEAD" {
		t.Fatal(result)
	}
}

func TestCleanedWorkspaceArchiveRestoreAndRestart(t *testing.T) {
	data := t.TempDir()
	a, err := Open(data, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "--allow-empty", "-m", "Initial"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			a.Close()
			t.Fatal(string(b), e)
		}
	}
	path := filepath.Join(t.TempDir(), "ticket")
	if err = a.Git.CreateWorktree(repo, path, "QA-1", "HEAD"); err != nil {
		a.Close()
		t.Fatal(err)
	}
	w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Restore", RepoPath: repo, WorktreePath: path, Branch: "QA-1"})
	if err != nil {
		a.Close()
		t.Fatal(err)
	}
	if _, err = a.removeWorktree(w.ID, true); err != nil {
		a.Close()
		t.Fatal(err)
	}
	archive, _ := json.Marshal(map[string]any{"id": w.ID, "archived": true})
	if _, err = a.Call(context.Background(), "workspace.archive", archive); err != nil {
		a.Close()
		t.Fatal(err)
	}
	a.Close()
	a, err = Open(data, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	stored, err := a.DB.Workspace(w.ID)
	if err != nil || stored.Lifecycle != "ARCHIVED" {
		t.Fatal(stored, err)
	}
	restore, _ := json.Marshal(map[string]any{"id": w.ID, "archived": false})
	if _, err = a.Call(context.Background(), "workspace.archive", restore); err != nil {
		t.Fatal(err)
	}
	stored, err = a.DB.Workspace(w.ID)
	if err != nil || stored.Lifecycle != "BACKLOG" || stored.WorktreePath != "" || stored.Branch != "" {
		t.Fatal(stored, err)
	}
}

func TestRemovingWorktreeRecalculatesLifecycle(t *testing.T) {
	a, err := Open(t.TempDir(), func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "--allow-empty", "-m", "Initial"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
	}
	for _, state := range []string{"CODING", "PR_OPEN", "MERGED", "DONE", "BLOCKED", "ARCHIVED"} {
		t.Run(state, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "worktree")
			if err = a.Git.CreateWorktree(repo, path, "test-"+strings.ToLower(state), "HEAD"); err != nil {
				t.Fatal(err)
			}
			w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: state, RepoPath: repo, WorktreePath: path, Branch: "test-" + strings.ToLower(state)})
			if err != nil {
				t.Fatal(err)
			}
			w.Lifecycle, w.Archived = state, state == "ARCHIVED"
			if _, err = a.DB.UpdateWorkspace(w); err != nil {
				t.Fatal(err)
			}
			if _, err = a.removeWorktree(w.ID, false); err == nil {
				t.Fatal("unconfirmed removal")
			}
			if _, err = a.removeWorktree(w.ID, true); err != nil {
				t.Fatal(err)
			}
			stored, err := a.DB.Workspace(w.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := "BACKLOG"
			if state == "DONE" || state == "BLOCKED" || state == "ARCHIVED" {
				want = state
			}
			if stored.WorktreePath != "" || stored.Branch != "" || stored.Lifecycle != want {
				t.Fatal(stored)
			}
			if _, err = os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("worktree still exists", err)
			}
		})
	}
}
