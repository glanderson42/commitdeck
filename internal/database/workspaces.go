package database

import (
	"database/sql"
	"errors"
	"strings"

	"commitdeck/internal/workspace"
)

const workspaceColumns = "id,external_key,title,status,repo_path,worktree_path,branch,jira_ticket_id,archived,created_at,updated_at"

type scanner interface{ Scan(...any) error }

func scanWorkspace(r scanner) (w workspace.Workspace, err error) {
	err = r.Scan(&w.ID, &w.ExternalKey, &w.Title, &w.Status, &w.RepoPath, &w.WorktreePath, &w.Branch, &w.JiraTicketID, &w.Archived, &w.CreatedAt, &w.UpdatedAt)
	return
}
func (d *DB) Workspaces() ([]workspace.Workspace, error) {
	r, err := d.Query("SELECT " + workspaceColumns + " FROM workspaces ORDER BY archived,updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []workspace.Workspace{}
	for r.Next() {
		w, e := scanWorkspace(r)
		if e != nil {
			return nil, e
		}
		out = append(out, w)
	}
	return out, r.Err()
}
func (d *DB) Workspace(id string) (workspace.Workspace, error) {
	return scanWorkspace(d.QueryRow("SELECT "+workspaceColumns+" FROM workspaces WHERE id=?", id))
}
func validateWorkspace(w *workspace.Workspace) error {
	w.Title = strings.TrimSpace(w.Title)
	w.ExternalKey = strings.TrimSpace(w.ExternalKey)
	if w.Title == "" || len(w.Title) > 500 {
		return errors.New("title must contain 1–500 characters")
	}
	if len(w.ExternalKey) > 100 {
		return errors.New("ticket key is too long")
	}
	if w.Status == "" {
		w.Status = "In Progress"
	}
	return nil
}
func (d *DB) CreateWorkspace(w workspace.Workspace) (workspace.Workspace, error) {
	if err := validateWorkspace(&w); err != nil {
		return w, err
	}
	w.ID = workspace.ID()
	w.CreatedAt = workspace.Now()
	w.UpdatedAt = w.CreatedAt
	tx, err := d.Begin()
	if err != nil {
		return w, err
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO workspaces ("+workspaceColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?)", w.ID, w.ExternalKey, w.Title, w.Status, w.RepoPath, w.WorktreePath, w.Branch, w.JiraTicketID, w.Archived, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return w, err
	}
	_, err = tx.Exec("INSERT INTO notes VALUES (?,?,?,?,?)", workspace.ID(), w.ID, "", w.CreatedAt, w.CreatedAt)
	if err != nil {
		return w, err
	}
	return w, tx.Commit()
}
func (d *DB) UpdateWorkspace(w workspace.Workspace) (workspace.Workspace, error) {
	if err := validateWorkspace(&w); err != nil {
		return w, err
	}
	w.UpdatedAt = workspace.Now()
	r, err := d.Exec("UPDATE workspaces SET external_key=?,title=?,status=?,repo_path=?,worktree_path=?,branch=?,jira_ticket_id=?,archived=?,updated_at=? WHERE id=?", w.ExternalKey, w.Title, w.Status, w.RepoPath, w.WorktreePath, w.Branch, w.JiraTicketID, w.Archived, w.UpdatedAt, w.ID)
	if err = changed(r, err); err != nil {
		return w, err
	}
	return d.Workspace(w.ID)
}
func changed(r sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (d *DB) DeleteWorkspace(id string) error {
	r, err := d.Exec("DELETE FROM workspaces WHERE id=?", id)
	return changed(r, err)
}
func (d *DB) Note(id string) (n workspace.Note, err error) {
	err = d.QueryRow("SELECT id,workspace_id,content,created_at,updated_at FROM notes WHERE workspace_id=?", id).Scan(&n.ID, &n.WorkspaceID, &n.Content, &n.CreatedAt, &n.UpdatedAt)
	return
}
func (d *DB) SaveNote(id, content string) error {
	if len(content) > 4<<20 {
		return errors.New("note exceeds 4 MB")
	}
	r, err := d.Exec("UPDATE notes SET content=?,updated_at=? WHERE workspace_id=?", content, workspace.Now(), id)
	return changed(r, err)
}
func (d *DB) Setting(key string) (string, error) {
	var s string
	err := d.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return s, err
}
func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec("INSERT INTO settings VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}
