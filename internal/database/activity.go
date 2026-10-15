package database

import (
	"commitdeck/internal/activity"
	"database/sql"
	"errors"
)

func (d *DB) AppendActivity(e activity.Event) (bool, error) {
	var key any
	if e.DedupeKey != "" {
		key = e.DedupeKey
	}
	r, err := d.Exec("INSERT OR IGNORE INTO workspace_events(id,workspace_id,type,title,metadata,dedupe_key,notify,created_at) VALUES(?,?,?,?,?,?,?,?)", e.ID, e.WorkspaceID, e.Type, e.Title, string(e.Metadata), key, e.Notify, e.CreatedAt)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}
func (d *DB) TransitionActivity(id, source, value string, e activity.Event) (bool, error) {
	tx, err := d.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var old string
	err = tx.QueryRow("SELECT value FROM workspace_signals WHERE workspace_id=? AND source=?", id, source).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil && old == value {
		return false, nil
	}
	_, err = tx.Exec("INSERT INTO workspace_signals(workspace_id,source,value) VALUES(?,?,?) ON CONFLICT(workspace_id,source) DO UPDATE SET value=excluded.value,revision=revision+1", id, source, value)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec("INSERT INTO workspace_events(id,workspace_id,type,title,metadata,notify,created_at) VALUES(?,?,?,?,?,?,?)", e.ID, id, e.Type, e.Title, string(e.Metadata), e.Notify, e.CreatedAt)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (d *DB) Activity(id string, notifications bool) ([]activity.Event, error) {
	query := "SELECT id,workspace_id,type,title,metadata,notify,read,created_at FROM workspace_events WHERE workspace_id=? ORDER BY sequence DESC LIMIT 100"
	var arg any = id
	if notifications {
		query = "SELECT id,workspace_id,type,title,metadata,notify,read,created_at FROM workspace_events WHERE notify=1 ORDER BY sequence DESC LIMIT 100"
		arg = nil
	}
	var rows *sql.Rows
	var err error
	if arg == nil {
		rows, err = d.Query(query)
	} else {
		rows, err = d.Query(query, arg)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []activity.Event{}
	for rows.Next() {
		var e activity.Event
		var raw string
		if err = rows.Scan(&e.ID, &e.WorkspaceID, &e.Type, &e.Title, &raw, &e.Notify, &e.Read, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Metadata = []byte(raw)
		out = append(out, e)
	}
	return out, rows.Err()
}
func (d *DB) ReadNotifications() error {
	_, err := d.Exec("UPDATE workspace_events SET read=1 WHERE notify=1 AND read=0")
	return err
}
