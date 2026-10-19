package database

import (
	"commitdeck/internal/workspace"
	"path/filepath"
	"testing"
)

func TestAutosaveCannotOverwriteAgentAppend(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Notes"})
	_ = db.SaveNote(w.ID, "Original")
	_ = db.SaveNote(w.ID, "Original\nAgent decision")
	if err = db.SaveNoteIfUnchanged(w.ID, "User draft", "Original"); err == nil {
		t.Fatal("overwrote agent append")
	}
	n, _ := db.Note(w.ID)
	if n.Content != "Original\nAgent decision" {
		t.Fatal(n)
	}
	if err = db.SaveNoteIfUnchanged(w.ID, "Merged notes", n.Content); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveNoteWithHash(w.ID, "Stale replacement", NoteHash("Original")); err == nil {
		t.Fatal("stale note hash accepted")
	}
	if err = db.SaveNoteWithHash(w.ID, "Reviewed replacement", NoteHash("Merged notes")); err != nil {
		t.Fatal(err)
	}
}

func TestStaleTaskEditCannotUndoCompletion(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Tasks"})
	original, err := db.SaveTodo(workspace.Todo{WorkspaceID: w.ID, Title: "Add regression test"})
	if err != nil {
		t.Fatal(err)
	}
	completed := original
	completed.Completed = true
	completed.UpdatedBy = "codex"
	if _, err = db.SaveTodo(completed); err != nil {
		t.Fatal(err)
	}
	original.Title = "User edit from an older view"
	if _, err = db.SaveTodo(original); err == nil {
		t.Fatal("stale edit overwrote completion")
	}
	tasks, _ := db.Todos(w.ID)
	if len(tasks) != 1 || !tasks[0].Completed || tasks[0].UpdatedBy != "codex" {
		t.Fatal(tasks)
	}
}
