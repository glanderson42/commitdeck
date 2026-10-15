package database

import (
	"commitdeck/internal/workspace"
	"encoding/json"
	"errors"
)

type Cache struct {
	Data      json.RawMessage `json:"data"`
	UpdatedAt string          `json:"updatedAt"`
}

func cacheTable(kind string) (string, error) {
	switch kind {
	case "jira":
		return "jira_cache", nil
	case "pr":
		return "pull_request_cache", nil
	}
	return "", errors.New("unknown cache")
}
func (d *DB) Cache(kind, id string) (*Cache, error) {
	table, err := cacheTable(kind)
	if err != nil {
		return nil, err
	}
	var c Cache
	var s string
	err = d.QueryRow("SELECT payload,updated_at FROM "+table+" WHERE workspace_id=?", id).Scan(&s, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Data = json.RawMessage(s)
	return &c, nil
}
func (d *DB) PutCache(kind, id string, v any) error {
	table, err := cacheTable(kind)
	if err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = d.Exec("INSERT INTO "+table+" VALUES (?,?,?) ON CONFLICT(workspace_id) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at", id, string(b), workspace.Now())
	return err
}
