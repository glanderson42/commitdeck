package database

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/workspace"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

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
