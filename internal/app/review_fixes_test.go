package app

import (
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
)

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
			if !ok || !strings.Contains(warning["warning"], "Jira action succeeded") {
				t.Fatal(result)
			}
		})
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
