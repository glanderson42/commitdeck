package app

import (
	contextengine "commitdeck/internal/context"
	"commitdeck/internal/permissions"
	"context"
	"encoding/json"
	"errors"
)

func (a *App) control(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		ID          string                 `json:"id"`
		WorkspaceID string                 `json:"workspaceId"`
		Options     *contextengine.Options `json:"options"`
		Purpose     string                 `json:"purpose"`
		Hash        string                 `json:"hash"`
		Allow       bool                   `json:"allow"`
		Scope       string                 `json:"scope"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch method {
	case "context.build":
		opts := contextengine.Defaults()
		if p.Options != nil {
			opts = *p.Options
		}
		return a.Agents.Context.BuildWorkspaceContext(ctx, p.WorkspaceID, opts, p.Purpose)
	case "session.context":
		return a.Agents.Context.Snapshot(p.ID)
	case "context.refresh":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		session, err := a.DB.Session(p.ID)
		if err != nil {
			return nil, err
		}
		if session.PID > 0 {
			return nil, errors.New("wait for the current turn to finish before refreshing context")
		}
		opts := contextengine.Defaults()
		if p.Options != nil {
			opts = *p.Options
		}
		generated, err := a.Agents.Context.BuildWorkspaceContext(ctx, session.WorkspaceID, opts, session.Purpose)
		if err != nil {
			return nil, err
		}
		if p.Hash == "" || generated.Hash != p.Hash {
			return nil, errors.New("context changed; preview it again before refreshing")
		}
		if err = a.Agents.Context.Save(session, generated); err != nil {
			return nil, err
		}
		return a.Agents.Context.Snapshot(session.ID)
	case "permissions.resolve":
		return nil, a.Agents.Permissions.Resolve(p.ID, permissions.Decision{Allow: p.Allow, Scope: p.Scope})
	case "permissions.rules.delete":
		return nil, a.DB.DeletePermissionRule(p.ID)
	case "notifications.read":
		return nil, a.DB.ReadNotifications()
	}
	return nil, errors.New("unsupported control operation")
}
