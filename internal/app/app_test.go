package app

import (
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type memorySecrets struct{ token string }

func (s *memorySecrets) Get(string) (string, error) {
	if s.token == "" {
		return "", errors.New("not set")
	}
	return s.token, nil
}
func TestSecretsStayOutOfDatabase(t *testing.T) {
	a, err := Open(t.TempDir(), func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	store := &memorySecrets{}
	a.Secrets = store
	params, _ := json.Marshal(map[string]any{"settings": Settings{JiraBaseURL: "https://example.atlassian.net", JiraEmail: "dev@example.com"}, "token": "sensitive-test-token"})
	if _, err = a.Call(context.Background(), "settings.save", params); err != nil {
		t.Fatal(err)
	}
	stored, err := a.DB.Setting("integrations")
	if err != nil || strings.Contains(stored, "sensitive-test-token") {
		t.Fatal("secret persisted in settings")
	}
	if store.token != "sensitive-test-token" {
		t.Fatal("native store not used")
	}
	settings, err := a.settings()
	if err != nil || !settings.JiraTokenSet {
		t.Fatal("credential presence missing")
	}
}
func (s *memorySecrets) Set(_ string, v string) error { s.token = v; return nil }
func (s *memorySecrets) Delete(string) error          { s.token = ""; return nil }
func TestLocalWorkflowSurvivesIntegrationFailure(t *testing.T) {
	data := t.TempDir()
	a, err := Open(data, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	a.Secrets = &memorySecrets{}
	call := func(method string, p any) (any, error) {
		b, _ := json.Marshal(p)
		return a.Call(context.Background(), method, b)
	}
	v, err := call("workspace.create", map[string]any{"title": "Retry", "externalKey": "PROJ-421"})
	if err != nil {
		t.Fatal(err)
	}
	w := v.(workspace.Workspace)
	if _, err = call("jira.refresh", map[string]string{"id": w.ID}); err == nil {
		t.Fatal("unconfigured Jira should fail")
	}
	if _, err = call("pr.refresh", map[string]string{"id": w.ID}); err == nil {
		t.Fatal("unconfigured GitHub should fail")
	}
	if _, err = call("note.save", map[string]string{"workspaceId": w.ID, "content": "# Investigation"}); err != nil {
		t.Fatal(err)
	}
	if _, err = call("todo.save", map[string]string{"workspaceId": w.ID, "title": "Test offline behavior"}); err != nil {
		t.Fatal(err)
	}
	if _, err = call("workspace.delete", map[string]string{"id": w.ID}); err == nil {
		t.Fatal("delete without confirmation")
	}
	if _, err = call("session.create", map[string]string{"workspaceId": w.ID, "provider": "codex"}); err == nil {
		t.Fatal("agent without worktree")
	}
	a.Close()
	a, err = Open(data, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Secrets = &memorySecrets{}
	n, err := a.DB.Note(w.ID)
	if err != nil || n.Content != "# Investigation" {
		t.Fatal(n, err)
	}
	tasks, _ := a.DB.Todos(w.ID)
	if len(tasks) != 1 {
		t.Fatal(tasks)
	}
	if _, err = call("workspace.delete", map[string]any{"id": w.ID, "confirm": true}); err != nil {
		t.Fatal(err)
	}
}
