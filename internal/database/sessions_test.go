package database

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/workspace"
	"path/filepath"
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
