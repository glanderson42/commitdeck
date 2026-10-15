package database

import (
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"database/sql"
	"encoding/json"
)

func (d *DB) PermissionRoot(id string) (string, error) {
	var path string
	err := d.QueryRow("SELECT worktree_path FROM workspaces WHERE id=?", id).Scan(&path)
	return path, err
}

func (d *DB) SavePermission(r permissions.Request) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = d.Exec("INSERT INTO permission_requests(id,workspace_id,session_id,status,payload,created_at) VALUES(?,?,?,?,?,?)", r.ID, r.WorkspaceID, r.SessionID, r.Status, string(b), r.CreatedAt)
	if err == nil {
		_, err = d.Exec("UPDATE agent_sessions SET status='waiting_permission' WHERE id=? AND pid>0", r.SessionID)
	}
	return err
}
func scanPermission(row scanner) (r permissions.Request, err error) {
	var b string
	var resolved sql.NullString
	var status string
	err = row.Scan(&b, &status, &resolved)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(b), &r)
	r.Status = status
	r.ResolvedAt = resolved.String
	if status != "pending" {
		r.CanRespond = false
	}
	return
}
func (d *DB) Permission(id string) (permissions.Request, error) {
	return scanPermission(d.QueryRow("SELECT payload,status,resolved_at FROM permission_requests WHERE id=?", id))
}
func (d *DB) Permissions(id string, pending bool) ([]permissions.Request, error) {
	query := "SELECT payload,status,resolved_at FROM permission_requests WHERE 1=1"
	args := []any{}
	if id != "" {
		query += " AND workspace_id=?"
		args = append(args, id)
	}
	if pending {
		query += " AND status='pending'"
	}
	query += " ORDER BY created_at DESC LIMIT 200"
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []permissions.Request{}
	for rows.Next() {
		r, e := scanPermission(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (d *DB) FinishPermission(id, status string) error {
	r, err := d.Exec("UPDATE permission_requests SET status=?,resolved_at=? WHERE id=? AND status='pending'", status, workspace.Now(), id)
	if err = changed(r, err); err != nil {
		return err
	}
	_, err = d.Exec("UPDATE agent_sessions SET status=CASE WHEN EXISTS(SELECT 1 FROM permission_requests p WHERE p.session_id=agent_sessions.id AND p.status='pending') THEN 'waiting_permission' WHEN pid>0 THEN 'running' ELSE status END WHERE id=(SELECT session_id FROM permission_requests WHERE id=?)", id)
	return err
}
func (d *DB) ExpirePermissions(id string) error {
	_, err := d.Exec("UPDATE permission_requests SET status='expired',resolved_at=? WHERE session_id=? AND status='pending'", workspace.Now(), id)
	return err
}
func (d *DB) PermissionRules(id string) ([]permissions.Rule, error) {
	rows, err := d.Query("SELECT id,workspace_id,COALESCE(session_id,''),scope,executable,arguments,cwd,created_at FROM permission_rules WHERE workspace_id=? ORDER BY created_at", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []permissions.Rule{}
	for rows.Next() {
		var r permissions.Rule
		var b string
		if err = rows.Scan(&r.ID, &r.WorkspaceID, &r.SessionID, &r.Scope, &r.Executable, &b, &r.Cwd, &r.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &r.ArgPrefix); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (d *DB) SavePermissionRule(r permissions.Rule) error {
	b, err := json.Marshal(r.ArgPrefix)
	if err != nil {
		return err
	}
	var sid any
	if r.Scope == "session" {
		sid = r.SessionID
	}
	_, err = d.Exec("INSERT INTO permission_rules VALUES(?,?,?,?,?,?,?,?)", r.ID, r.WorkspaceID, sid, r.Scope, r.Executable, string(b), r.Cwd, r.CreatedAt)
	return err
}
func (d *DB) DeletePermissionRule(id string) error {
	_, err := d.Exec("DELETE FROM permission_rules WHERE id=?", id)
	return err
}
func (d *DB) RecoverPermissions() error {
	_, err := d.Exec("UPDATE permission_requests SET status='expired',resolved_at=? WHERE status='pending'", workspace.Now())
	if err != nil {
		return err
	}
	_, err = d.Exec("DELETE FROM permission_rules WHERE scope='session'")
	return err
}
