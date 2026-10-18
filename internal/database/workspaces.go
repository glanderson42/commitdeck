package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"commitdeck/internal/workspace"
)

func NoteHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
func (d *DB) SaveNoteWithHash(id, content, expectedHash string) error {
	current, err := d.Note(id)
	if err != nil {
		return err
	}
	if NoteHash(current.Content) != expectedHash {
		return errors.New("notes changed outside this editor; your draft is preserved. Use Compare with saved note to review and merge the latest changes.")
	}
	return d.SaveNoteIfUnchanged(id, content, current.Content)
}

const workspaceColumns = "id,external_key,title,status,repo_path,worktree_path,branch,jira_ticket_id,archived,created_at,updated_at,lifecycle"

type scanner interface{ Scan(...any) error }

func scanWorkspace(r scanner) (w workspace.Workspace, err error) {
	err = r.Scan(&w.ID, &w.ExternalKey, &w.Title, &w.Status, &w.RepoPath, &w.WorktreePath, &w.Branch, &w.JiraTicketID, &w.Archived, &w.CreatedAt, &w.UpdatedAt, &w.Lifecycle)
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
	w.Lifecycle = "BACKLOG"
	w.ID = workspace.ID()
	w.CreatedAt = workspace.Now()
	w.UpdatedAt = w.CreatedAt
	tx, err := d.Begin()
	if err != nil {
		return w, err
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO workspaces ("+workspaceColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", w.ID, w.ExternalKey, w.Title, w.Status, w.RepoPath, w.WorktreePath, w.Branch, w.JiraTicketID, w.Archived, w.CreatedAt, w.UpdatedAt, w.Lifecycle)
	if err != nil {
		return w, err
	}
	_, err = tx.Exec("INSERT INTO notes (id,workspace_id,content,created_at,updated_at) VALUES (?,?,?,?,?)", workspace.ID(), w.ID, "", w.CreatedAt, w.CreatedAt)
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
	r, err := d.Exec("UPDATE workspaces SET external_key=?,title=?,status=?,repo_path=?,worktree_path=?,branch=?,jira_ticket_id=?,archived=?,updated_at=?,lifecycle=? WHERE id=?", w.ExternalKey, w.Title, w.Status, w.RepoPath, w.WorktreePath, w.Branch, w.JiraTicketID, w.Archived, w.UpdatedAt, w.Lifecycle, w.ID)
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
	err = d.QueryRow("SELECT id,workspace_id,content,created_at,updated_at,updated_by,session_id FROM notes WHERE workspace_id=?", id).Scan(&n.ID, &n.WorkspaceID, &n.Content, &n.CreatedAt, &n.UpdatedAt, &n.UpdatedBy, &n.SessionID)
	return
}
func (d *DB) SaveNote(id, content string) error {
	if len(content) > 4<<20 {
		return errors.New("note exceeds 4 MB")
	}
	r, err := d.Exec("UPDATE notes SET content=?,updated_at=?,updated_by='user',session_id='' WHERE workspace_id=?", content, workspace.Now(), id)
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

// SaveNoteIfUnchanged prevents an autosave from overwriting an agent append or another window's edit.
func (d *DB) SaveNoteIfUnchanged(id, content, expected string) error {
	if len(content) > 4<<20 {
		return errors.New("note exceeds 4 MB")
	}
	result, err := d.Exec("UPDATE notes SET content=?,updated_at=?,updated_by='user',session_id='' WHERE workspace_id=? AND content=?", content, workspace.Now(), id, expected)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("notes changed outside this editor; your draft is preserved. Use Compare with saved note to review and merge the latest changes.")
	}
	return nil
}
