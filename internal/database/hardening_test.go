package database

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPermissionPersistenceProtectsDiagnosticsAndRememberedRules(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	s, err := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	r := permissions.Request{ID: workspace.ID(), WorkspaceID: w.ID, SessionID: s.ID, Provider: "codex", Status: "pending", CreatedAt: workspace.Now(),
		Title: "Test API_KEY=synthetic-permission-title", Description: "password=synthetic-permission-description", FilePath: "/tmp/token=synthetic-permission-path",
		Command: "go test --token synthetic-permission-command", Executable: "go", Arguments: []string{"test", "--token", "synthetic-permission-command"}, Cwd: "/repo"}
	if err = db.SavePermission(r); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err = db.QueryRow("SELECT payload FROM permission_requests WHERE id=?", r.ID).Scan(&payload); err != nil || strings.Contains(payload, "synthetic-") {
		t.Fatal("unsafe permission persisted", err)
	}
	stored, err := db.Permission(r.ID)
	if err != nil || !stored.HighRisk || stored.Executable != "" || len(stored.Arguments) != 0 {
		t.Fatal("credential-bearing permission can be remembered", err)
	}
	rule := permissions.Rule{ID: workspace.ID(), WorkspaceID: w.ID, Scope: "workspace", Executable: "go", ArgPrefix: []string{"test", "--token", "synthetic-rule-value"}, Cwd: "/repo", CreatedAt: workspace.Now()}
	if err = db.SavePermissionRule(rule); err == nil {
		t.Fatal("credential-bearing rule persisted")
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM permission_rules").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	// Model an old database without making its unsafe rule reusable or visible.
	args, _ := json.Marshal(rule.ArgPrefix)
	if _, err = db.Exec("INSERT INTO permission_rules VALUES(?,?,?,?,?,?,?,?)", rule.ID, w.ID, nil, rule.Scope, rule.Executable, string(args), rule.Cwd, rule.CreatedAt); err != nil {
		t.Fatal(err)
	}
	rules, err := db.PermissionRules(w.ID)
	if err != nil || len(rules) != 0 {
		t.Fatal("legacy credential rule exposed", err)
	}
}

func TestActivityPersistenceRedactsMetadataAndSignals(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	e := activity.Event{ID: workspace.ID(), WorkspaceID: w.ID, Type: "test", Title: "Diagnostic password=synthetic-title", Metadata: json.RawMessage(`{"api_key":"synthetic-metadata","safe":"kept"}`), CreatedAt: workspace.Now()}
	if _, err = db.TransitionActivity(w.ID, "test", "token=synthetic-signal", e); err != nil {
		t.Fatal(err)
	}
	var title, metadata, signal string
	if err = db.QueryRow("SELECT title,metadata FROM workspace_events WHERE id=?", e.ID).Scan(&title, &metadata); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT value FROM workspace_signals WHERE workspace_id=?", w.ID).Scan(&signal); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(title+metadata+signal, "synthetic-") || !strings.Contains(metadata, "kept") || !json.Valid([]byte(metadata)) {
		t.Fatal("activity credentials persisted")
	}
}

func TestTodoUTF8ByteBoundaries(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	for _, tc := range []struct {
		title string
		ok    bool
	}{
		{strings.Repeat("x", 1000), true}, {strings.Repeat("x", 1001), false},
		{strings.Repeat("🚀", 250), true}, {strings.Repeat("🚀", 251), false},
		{strings.Repeat("界", 333) + "a", true}, {strings.Repeat("界", 334), false},
	} {
		if _, err = db.SaveTodo(workspace.Todo{WorkspaceID: w.ID, Title: tc.title}); (err == nil) != tc.ok {
			t.Fatalf("%d bytes: %v", len(tc.title), err)
		}
	}
}
