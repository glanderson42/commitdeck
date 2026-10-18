package app

import (
	"context"
	"encoding/json"
)

func (a *App) terminalCall(_ context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		ID          string `json:"id"`
		WorkspaceID string `json:"workspaceId"`
		Title       string `json:"title"`
		Data        string `json:"data"`
		Cols        int    `json:"cols"`
		Rows        int    `json:"rows"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch method {
	case "terminal.list":
		return a.Terminals.List(p.WorkspaceID)
	case "terminal.create":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		return a.Terminals.Create(p.WorkspaceID)
	case "terminal.start":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		return nil, a.Terminals.Start(p.ID, p.Cols, p.Rows)
	case "terminal.snapshot":
		return a.Terminals.Snapshot(p.ID), nil
	case "terminal.write":
		return nil, a.Terminals.Write(p.ID, p.Data)
	case "terminal.resize":
		return nil, a.Terminals.Resize(p.ID, p.Cols, p.Rows)
	case "terminal.rename":
		return nil, a.Terminals.Rename(p.ID, p.Title)
	case "terminal.delete":
		return nil, a.Terminals.Delete(p.ID)
	}
	return nil, nil
}
