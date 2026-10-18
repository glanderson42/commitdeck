package database

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/workspace"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryPreservesPermissionWaitAndNativeSession(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w, err := d.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	running, err := d.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", Title: "Implementation"})
	if err != nil {
		t.Fatal(err)
	}
	waiting, err := d.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "claude", Title: "Investigation"})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.ExternalSession(running.ID, "thread-123"); err != nil {
		t.Fatal(err)
	}
	if err = d.SessionStatus(running.ID, "running", 123456); err != nil {
		t.Fatal(err)
	}
	if err = d.SessionStatus(waiting.ID, "waiting_permission", 0); err != nil {
		t.Fatal(err)
	}
	if err = d.RecoverSessions(); err != nil {
		t.Fatal(err)
	}
	running, err = d.Session(running.ID)
	if err != nil || running.Status != "idle" || running.PID != 0 || running.ExternalSessionID != "thread-123" {
		t.Fatal(running, err)
	}
	waiting, err = d.Session(waiting.ID)
	if err != nil || waiting.Status != "idle" {
		t.Fatal(waiting, err)
	}
}

func TestAgentEventIsRedactedBeforePersistence(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w, err := d.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := d.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic credentials exist only as input; the serialized row must contain none.
	e := protocol.Event{ID: workspace.ID(), SessionID: s.ID, Type: "text", CreatedAt: workspace.Now(),
		Text:    "Decision: stable invoice ID. api_key=synthetic-test-value",
		Command: "curl -H 'Authorization: Bearer synthetic-command-value' https://example.test",
		Diff:    "+password=synthetic-diff-value", Error: "token=synthetic-error-value",
		FilePath: "https://user:synthetic-url-value@example.test/file", ToolName: "secret=synthetic-tool-value"}
	if err = d.AddEvent(e); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err = d.QueryRow("SELECT payload FROM agent_events WHERE id=?", e.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "synthetic-") {
		t.Fatal("credential-shaped input persisted")
	}
	var stored protocol.Event
	if err = json.Unmarshal([]byte(payload), &stored); err != nil {
		t.Fatal(err)
	}
	if stored != e.Redacted() || !strings.Contains(stored.Text, "stable invoice ID") || !strings.Contains(stored.Text, "[REDACTED]") {
		t.Fatal(stored)
	}
}
