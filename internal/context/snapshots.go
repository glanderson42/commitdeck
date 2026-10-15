package context

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/workspace"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Snapshot struct {
	ID              string   `json:"id"`
	SessionID       string   `json:"sessionId"`
	WorkspaceID     string   `json:"workspaceId"`
	GeneratedAt     string   `json:"generatedAt"`
	Hash            string   `json:"hash"`
	IncludedSources []string `json:"includedSources"`
	CharacterCount  int      `json:"characterCount"`
	Options         Options  `json:"options"`
	Markdown        string   `json:"markdown"`
	SentAt          string   `json:"sentAt"`
}

func (e *Engine) Save(session protocol.Session, c *WorkspaceContext) error {
	sources, _ := json.Marshal(c.IncludedSources)
	opts, _ := json.Marshal(c.Options)
	_, err := e.DB.Exec("INSERT INTO agent_session_context(id,session_id,workspace_id,generated_at,context_hash,included_sources,character_count,options,markdown) VALUES(?,?,?,?,?,?,?,?,?)", workspace.ID(), session.ID, session.WorkspaceID, workspace.Now(), c.Hash, string(sources), c.CharacterCount, string(opts), c.Markdown)
	return err
}
func (e *Engine) Snapshot(sessionID string) (*Snapshot, error) {
	var s Snapshot
	var sources, opts string
	var sent sql.NullString
	err := e.DB.QueryRow("SELECT id,session_id,workspace_id,generated_at,context_hash,included_sources,character_count,options,markdown,sent_at FROM agent_session_context WHERE session_id=? ORDER BY generated_at DESC LIMIT 1", sessionID).Scan(&s.ID, &s.SessionID, &s.WorkspaceID, &s.GeneratedAt, &s.Hash, &sources, &s.CharacterCount, &opts, &s.Markdown, &sent)
	if err != nil {
		return nil, err
	}
	s.SentAt = sent.String
	if err = json.Unmarshal([]byte(sources), &s.IncludedSources); err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(opts), &s.Options); err != nil {
		return nil, err
	}
	return &s, nil
}
func (e *Engine) Prepare(ctx context.Context, s protocol.Session, prompt string) (string, string, error) {
	snapshot, err := e.Snapshot(s.ID)
	if errors.Is(err, sql.ErrNoRows) {
		// Legacy sessions resume with their existing provider context. New context
		// must be explicitly previewed and attached through context.refresh.
		return prompt, "", nil
	}
	if err != nil {
		return "", "", err
	}
	if snapshot.SentAt != "" && s.ExternalSessionID != "" {
		return prompt, "", nil
	}
	return snapshot.Markdown + "\n\n## User request\n\n" + prompt, snapshot.ID, nil
}
func (e *Engine) MarkSent(id string) error {
	if id == "" {
		return nil
	}
	_, err := e.DB.Exec("UPDATE agent_session_context SET sent_at=? WHERE id=?", workspace.Now(), id)
	return err
}
