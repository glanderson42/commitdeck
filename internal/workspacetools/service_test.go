package workspacetools

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScopedKnowledgeEditsAttributionAndApproval(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Tools", WorktreePath: t.TempDir()})
	other, _ := db.CreateWorkspace(workspace.Workspace{Title: "Other"})
	session, _ := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", Title: "Implementation"})
	a := &activity.Service{Store: db}
	p := permissions.New(db, a, nil)
	service, err := New(db, p, a)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	call := func(r Request) (any, error) { return service.Call(context.Background(), session, r) }
	result, err := call(Request{Operation: "todo.add", Title: "Add integration test"})
	if err != nil {
		t.Fatal(err)
	}
	todo := result.(workspace.Todo)
	if todo.UpdatedBy != "codex" || todo.SessionID != session.ID {
		t.Fatal(todo)
	}
	if _, err = call(Request{Operation: "todo.complete", ID: todo.ID, Completed: true}); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Todos(w.ID)
	if !rows[0].Completed || rows[0].UpdatedBy != "codex" {
		t.Fatal(rows)
	}
	foreign, _ := db.SaveTodo(workspace.Todo{WorkspaceID: other.ID, Title: "Keep private"})
	if _, err = call(Request{Operation: "todo.update", ID: foreign.ID, Title: "Wrong workspace"}); err == nil {
		t.Fatal("cross-workspace edit")
	}
	_ = db.SaveNote(w.ID, "User notes")
	if _, err = call(Request{Operation: "notes.append", Content: "Decision: stable idempotency."}); err != nil {
		t.Fatal(err)
	}
	n, _ := db.Note(w.ID)
	if !strings.Contains(n.Content, "User notes") || n.UpdatedBy != "codex" {
		t.Fatal(n)
	}
	if _, err = call(Request{Operation: "sql.query", Content: "SELECT * FROM settings"}); err == nil {
		t.Fatal("arbitrary operation")
	}
	done := make(chan error, 1)
	go func() { _, err := call(Request{Operation: "todo.delete", ID: todo.ID}); done <- err }()
	var pending permissions.Request
	deadline := time.Now().Add(2 * time.Second)
	for {
		requests, _ := db.Permissions(w.ID, true)
		if len(requests) > 0 {
			pending = requests[0]
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("approval not queued")
		}
		time.Sleep(time.Millisecond * 5)
	}
	if !pending.HighRisk || !pending.CanRespond {
		t.Fatal(pending)
	}
	if err = p.Resolve(pending.ID, permissions.Decision{Allow: true, Scope: "workspace"}); err == nil {
		t.Fatal("destructive workspace rule")
	}
	if err = p.Resolve(pending.ID, permissions.Decision{Allow: false, Scope: "once"}); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil {
		t.Fatal("denial ignored")
	}
	rows, _ = db.Todos(w.ID)
	if len(rows) != 1 {
		t.Fatal("denied delete removed task")
	}
	go func() {
		_, err := call(Request{Operation: "notes.update", Content: "Replace user notes", ExpectedUpdatedAt: n.UpdatedAt})
		done <- err
	}()
	var id string
	deadline = time.Now().Add(2 * time.Second)
	for {
		requests, _ := db.Permissions(w.ID, true)
		if len(requests) > 0 {
			id = requests[0].ID
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("note replacement not queued")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = db.SaveNote(w.ID, "User edited while waiting")
	if err = p.Resolve(id, permissions.Decision{Allow: true, Scope: "once"}); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil {
		t.Fatal("stale replacement accepted")
	}
	n, _ = db.Note(w.ID)
	if n.Content != "User edited while waiting" {
		t.Fatal(n)
	}
	var edits int
	_ = db.QueryRow("SELECT COUNT(*) FROM workspace_tool_events").Scan(&edits)
	if edits != 3 {
		t.Fatal("unexpected tool audit count", edits)
	}
	for attempt := 0; attempt < 2; attempt++ {
		go func() { _, err := call(Request{Operation: "todo.delete", ID: todo.ID}); done <- err }()
		deadline = time.Now().Add(2 * time.Second)
		requestID := ""
		for {
			requests, _ := db.Permissions(w.ID, true)
			if len(requests) > 0 {
				requestID = requests[0].ID
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("delete approval missing")
			}
			time.Sleep(5 * time.Millisecond)
		}
		if attempt == 0 {
			current, _ := db.Todos(w.ID)
			current[0].Title = "User changed this task while approval was pending"
			current[0].UpdatedBy = "user"
			if _, err = db.SaveTodo(current[0]); err != nil {
				t.Fatal(err)
			}
		}
		if err = p.Resolve(requestID, permissions.Decision{Allow: true, Scope: "once"}); err != nil {
			t.Fatal(err)
		}
		editErr := <-done
		if attempt == 0 && editErr == nil {
			t.Fatal("deleted a task changed after the approval preview")
		}
		if attempt == 1 && editErr != nil {
			t.Fatal(editErr)
		}
	}
	rows, _ = db.Todos(w.ID)
	if len(rows) != 0 {
		t.Fatal("explicitly approved unchanged task was not deleted")
	}
}
