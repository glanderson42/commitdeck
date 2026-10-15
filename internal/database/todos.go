package database

import (
	"commitdeck/internal/workspace"
	"errors"
	"strings"
)

func (d *DB) Todos(id string) ([]workspace.Todo, error) {
	r, err := d.Query("SELECT id,workspace_id,title,completed,position,created_at,updated_at FROM todos WHERE workspace_id=? ORDER BY position,id", id)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []workspace.Todo{}
	for r.Next() {
		var t workspace.Todo
		if err = r.Scan(&t.ID, &t.WorkspaceID, &t.Title, &t.Completed, &t.Position, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, r.Err()
}
func (d *DB) SaveTodo(t workspace.Todo) (workspace.Todo, error) {
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" || len(t.Title) > 1000 {
		return t, errors.New("task title must contain 1–1000 characters")
	}
	t.UpdatedAt = workspace.Now()
	if t.ID == "" {
		t.ID = workspace.ID()
		t.CreatedAt = t.UpdatedAt
		_, err := d.Exec("INSERT INTO todos VALUES (?,?,?,?,(SELECT COALESCE(MAX(position),-1)+1 FROM todos WHERE workspace_id=?),?,?)", t.ID, t.WorkspaceID, t.Title, t.Completed, t.WorkspaceID, t.CreatedAt, t.UpdatedAt)
		return t, err
	}
	r, err := d.Exec("UPDATE todos SET title=?,completed=?,updated_at=? WHERE id=? AND workspace_id=?", t.Title, t.Completed, t.UpdatedAt, t.ID, t.WorkspaceID)
	return t, changed(r, err)
}
func (d *DB) DeleteTodo(workspaceID, id string) error {
	r, err := d.Exec("DELETE FROM todos WHERE id=? AND workspace_id=?", id, workspaceID)
	return changed(r, err)
}
func (d *DB) ReorderTodos(id string, ids []string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM todos WHERE workspace_id=?", id).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return errors.New("task list changed; refresh and retry")
	}
	seen := map[string]bool{}
	for i, tid := range ids {
		if seen[tid] {
			return errors.New("duplicate task in ordering")
		}
		seen[tid] = true
		r, e := tx.Exec("UPDATE todos SET position=?,updated_at=? WHERE id=? AND workspace_id=?", i, workspace.Now(), tid, id)
		if e = changed(r, e); e != nil {
			return e
		}
	}
	return tx.Commit()
}
