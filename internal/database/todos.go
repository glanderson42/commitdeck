package database

import (
	"commitdeck/internal/workspace"
	"errors"
	"strings"
)

func (d *DB) Todos(id string) ([]workspace.Todo, error) {
	r, err := d.Query("SELECT id,workspace_id,title,completed,position,created_at,updated_at,updated_by,session_id FROM todos WHERE workspace_id=? ORDER BY position,id", id)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []workspace.Todo{}
	for r.Next() {
		var t workspace.Todo
		if err = r.Scan(&t.ID, &t.WorkspaceID, &t.Title, &t.Completed, &t.Position, &t.CreatedAt, &t.UpdatedAt, &t.UpdatedBy, &t.SessionID); err != nil {
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
	if t.UpdatedBy == "" {
		t.UpdatedBy = "user"
	}
	expected := t.UpdatedAt
	t.UpdatedAt = workspace.Now()
	if t.ID == "" {
		t.ID = workspace.ID()
		t.CreatedAt = t.UpdatedAt
		_, err := d.Exec("INSERT INTO todos (id,workspace_id,title,completed,position,created_at,updated_at,updated_by,session_id) VALUES (?,?,?,?,(SELECT COALESCE(MAX(position),-1)+1 FROM todos WHERE workspace_id=?),?,?,?,?)", t.ID, t.WorkspaceID, t.Title, t.Completed, t.WorkspaceID, t.CreatedAt, t.UpdatedAt, t.UpdatedBy, t.SessionID)
		return t, err
	}
	query := "UPDATE todos SET title=?,completed=?,updated_at=?,updated_by=?,session_id=? WHERE id=? AND workspace_id=?"
	args := []any{t.Title, t.Completed, t.UpdatedAt, t.UpdatedBy, t.SessionID, t.ID, t.WorkspaceID}
	if expected != "" {
		query += " AND updated_at=?"
		args = append(args, expected)
	}
	r, err := d.Exec(query, args...)
	if err == nil && expected != "" {
		n, e := r.RowsAffected()
		if e != nil {
			return t, e
		}
		if n == 0 {
			return t, errors.New("task changed; refresh and retry your edit")
		}
	}
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
