package app

import (
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestStartWorkPreviewConfirmationAndArchivePreservesKnowledge(t *testing.T) {
	a, err := Open(t.TempDir(), func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
	}
	w, err := a.DB.CreateWorkspace(workspace.Workspace{Title: "Retry", ExternalKey: "PROJ-421", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	settings := Settings{WorktreeRoot: t.TempDir()}
	b, _ := json.Marshal(settings)
	_ = a.DB.SetSetting("integrations", string(b))
	call := func(method string, p any) (any, error) {
		b, _ := json.Marshal(p)
		return a.Call(context.Background(), method, b)
	}
	v, err := call("lifecycle.preview", map[string]any{"id": w.ID})
	if err != nil {
		t.Fatal(err)
	}
	path := v.(map[string]any)["worktree"].(string)
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("preview created worktree")
	}
	if _, err = call("lifecycle.start", map[string]any{"id": w.ID}); err == nil {
		t.Fatal("unconfirmed start")
	}
	if _, err = call("lifecycle.start", map[string]any{"id": w.ID, "confirm": true, "base": "HEAD"}); err != nil {
		t.Fatal(err)
	}
	w, _ = a.DB.Workspace(w.ID)
	if w.Lifecycle != "CODING" || w.WorktreePath != path || w.Branch != "PROJ-421-retry" {
		t.Fatal(w)
	}
	_ = a.DB.SaveNote(w.ID, "Keep this decision")
	_, _ = call("todo.save", map[string]any{"workspaceId": w.ID, "title": "Keep this task"})
	if _, err = call("workspace.archive", map[string]any{"id": w.ID, "archived": true}); err != nil {
		t.Fatal(err)
	}
	note, _ := a.DB.Note(w.ID)
	todos, _ := a.DB.Todos(w.ID)
	if note.Content != "Keep this decision" || len(todos) != 1 {
		t.Fatal("archive lost knowledge")
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("archive removed worktree", err)
	}
}
