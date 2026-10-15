package dashboard

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/workspace"
	"path/filepath"
	"testing"
)

func TestPriorityGroupingAndDetection(t *testing.T) {
	w := workspace.Workspace{Status: "Blocked"}
	p := &pullrequest.PullRequest{State: "OPEN", Mergeable: "CONFLICTING"}
	s := []protocol.Session{{Status: "running"}}
	if State(w, s, p, 1) != "AGENT_WAITING" {
		t.Fatal("pending priority")
	}
	if State(w, s, p, 0) != "MERGE_CONFLICT" {
		t.Fatal("conflict priority")
	}
	p.Checks = []pullrequest.Check{{Status: "failure"}}
	if State(w, s, p, 0) != "CI_FAILED" {
		t.Fatal("CI priority")
	}
	p.ChangesRequested = 1
	if State(w, s, p, 0) != "CHANGES_REQUESTED" {
		t.Fatal("review priority")
	}
	if State(w, s, nil, 0) != "BLOCKED" {
		t.Fatal("blocked")
	}
	w.Status = "In Progress"
	if State(w, s, nil, 0) != "AGENT_RUNNING" {
		t.Fatal("active")
	}
	rows := []Row{{Workspace: workspace.Workspace{ID: "b"}, Attention: "READY_TO_MERGE", LastActivity: "2"}, {Workspace: workspace.Workspace{ID: "a"}, Attention: "CI_FAILED", LastActivity: "1"}, {Workspace: workspace.Workspace{ID: "c"}, Attention: "CI_FAILED", LastActivity: "3"}}
	Sort(rows)
	if rows[0].ID != "c" || rows[1].ID != "a" || rows[2].ID != "b" {
		t.Fatal(rows)
	}
	if Group("AGENT_RUNNING") != "Active work" || Group("READY_TO_MERGE") != "Ready" || Group("WAITING_FOR_REVIEW") != "Waiting" {
		t.Fatal("groups")
	}
}
func TestBulkRowsIncludeTasksNotesAndSessions(t *testing.T) {
	d, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w, err := d.CreateWorkspace(workspace.Workspace{Title: "Retry", ExternalKey: "PROJ-421"})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.SaveNote(w.ID, "invoice idempotency"); err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveTodo(workspace.Todo{WorkspaceID: w.ID, Title: "Tests", Completed: true}); err != nil {
		t.Fatal(err)
	}
	s, err := d.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", Title: "Implementation"})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.SessionStatus(s.ID, "running", 123); err != nil {
		t.Fatal(err)
	}
	rows, err := Load(d)
	if err != nil || len(rows) != 1 || rows[0].TasksCompleted != 1 || rows[0].Attention != "AGENT_RUNNING" || len(rows[0].Sessions) != 1 {
		t.Fatal(rows, err)
	}
	if rows[0].SearchText == "" {
		t.Fatal("notes not searchable")
	}
}
