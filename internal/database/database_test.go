package database

import (
	"commitdeck/internal/workspace"
	"path/filepath"
	"testing"
)

func TestPersistenceAndCascade(t *testing.T) {
	p := filepath.Join(t.TempDir(), "app.db")
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := d.CreateWorkspace(workspace.Workspace{Title: "Retry", ExternalKey: "PROJ-421"})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.SaveNote(w.ID, "# Keep this\n"); err != nil {
		t.Fatal(err)
	}
	a, err := d.SaveTodo(workspace.Todo{WorkspaceID: w.ID, Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.SaveTodo(workspace.Todo{WorkspaceID: w.ID, Title: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.ReorderTodos(w.ID, []string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	if err = d.ReorderTodos(w.ID, []string{a.ID, a.ID}); err == nil {
		t.Fatal("accepted duplicates")
	}
	w.Archived = true
	if _, err = d.UpdateWorkspace(w); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	n, err := d.Note(w.ID)
	if err != nil || n.Content != "# Keep this\n" {
		t.Fatalf("lost note: %+v %v", n, err)
	}
	todos, err := d.Todos(w.ID)
	if err != nil || len(todos) != 2 || todos[0].ID != b.ID || todos[1].Position != 1 {
		t.Fatalf("ordering: %+v %v", todos, err)
	}
	saved, err := d.Workspace(w.ID)
	if err != nil || !saved.Archived {
		t.Fatal("archive not persisted")
	}
	if err = d.DeleteWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Note(w.ID); err == nil {
		t.Fatal("note not cascaded")
	}
	todos, _ = d.Todos(w.ID)
	if len(todos) != 0 {
		t.Fatal("tasks not cascaded")
	}
}
func TestExclusiveApplicationLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := Open(path); err == nil {
		b.Close()
		t.Fatal("second application opened same data directory")
	}
	a.Close()
	b, err := Open(path)
	if err != nil {
		t.Fatal("lock not released", err)
	}
	b.Close()
}
