package database

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/workspace"
	"encoding/json"
)

const sessionColumns = "id,workspace_id,provider,external_session_id,title,purpose,status,pid,created_at,updated_at"

func scanSession(r scanner) (s protocol.Session, err error) {
	err = r.Scan(&s.ID, &s.WorkspaceID, &s.Provider, &s.ExternalSessionID, &s.Title, &s.Purpose, &s.Status, &s.PID, &s.CreatedAt, &s.UpdatedAt)
	return
}
func (d *DB) Sessions(id string) ([]protocol.Session, error) {
	r, err := d.Query("SELECT "+sessionColumns+" FROM agent_sessions WHERE workspace_id=? ORDER BY created_at", id)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []protocol.Session{}
	for r.Next() {
		s, e := scanSession(r)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, r.Err()
}
func (d *DB) Session(id string) (protocol.Session, error) {
	return scanSession(d.QueryRow("SELECT "+sessionColumns+" FROM agent_sessions WHERE id=?", id))
}
func (d *DB) CreateSession(s protocol.Session) (protocol.Session, error) {
	s.ID = workspace.ID()
	s.CreatedAt = workspace.Now()
	s.UpdatedAt = s.CreatedAt
	s.Status = "idle"
	_, err := d.Exec("INSERT INTO agent_sessions ("+sessionColumns+") VALUES (?,?,?,?,?,?,?,?,?,?)", s.ID, s.WorkspaceID, s.Provider, s.ExternalSessionID, s.Title, s.Purpose, s.Status, s.PID, s.CreatedAt, s.UpdatedAt)
	return s, err
}
func (d *DB) SessionStatus(id, status string, pid int) error {
	r, err := d.Exec("UPDATE agent_sessions SET status=?,pid=?,updated_at=? WHERE id=?", status, pid, workspace.Now(), id)
	return changed(r, err)
}
func (d *DB) ExternalSession(id, external string) error {
	r, err := d.Exec("UPDATE agent_sessions SET external_session_id=?,updated_at=? WHERE id=?", external, workspace.Now(), id)
	return changed(r, err)
}
func (d *DB) RecoverSessions() error {
	_, err := d.Exec("UPDATE agent_sessions SET status='idle',pid=0,updated_at=? WHERE status IN ('running','starting','waiting_permission') OR pid>0", workspace.Now())
	return err
}
func (d *DB) AddEvent(e protocol.Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = d.Exec("INSERT INTO agent_events (id,session_id,payload) VALUES (?,?,?)", e.ID, e.SessionID, string(b))
	return err
}
func (d *DB) Events(id string) ([]protocol.Event, error) {
	r, err := d.Query("SELECT payload FROM agent_events WHERE session_id=? ORDER BY sequence", id)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	events := []protocol.Event{}
	for r.Next() {
		var b string
		if err = r.Scan(&b); err != nil {
			return nil, err
		}
		var e protocol.Event
		if err = json.Unmarshal([]byte(b), &e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, r.Err()
}
