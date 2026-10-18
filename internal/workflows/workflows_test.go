package workflows

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents"
	"commitdeck/internal/agents/protocol"
	contextengine "commitdeck/internal/context"
	"commitdeck/internal/database"
	"commitdeck/internal/workspace"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type runner struct {
	db   *database.DB
	sent []string
	fail bool
}

func (r *runner) StartSession(_ context.Context, w workspace.Workspace, o agents.SessionOptions) (protocol.Session, error) {
	if o.ContextHash != "previewed" {
		return protocol.Session{}, errors.New("context changed")
	}
	return r.db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: o.Provider, Title: o.Title, Purpose: o.Purpose})
}
func (r *runner) ResumeSession(context.Context, string) (protocol.Session, error) {
	return protocol.Session{}, nil
}
func (r *runner) Send(_ context.Context, _ string, prompt string) error {
	if r.fail {
		return errors.New("provider failed")
	}
	r.sent = append(r.sent, prompt)
	return nil
}
func (r *runner) Cancel(context.Context, string) error { return nil }

type contextBuilder struct{}

func (contextBuilder) BuildWorkspaceContext(context.Context, string, contextengine.Options, string) (*contextengine.WorkspaceContext, error) {
	return &contextengine.WorkspaceContext{Hash: "previewed", Markdown: "Exact context"}, nil
}
func TestManualStepOrderingHandoffFailureAndRetry(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Feature", WorktreePath: t.TempDir()})
	r := &runner{db: db}
	service := &Service{DB: db, Agents: r, Context: contextBuilder{}, Activity: &activity.Service{Store: db}}
	wf, err := service.Create(w.ID, "Implement Feature", []string{"claude", "codex", "opencode", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(wf.Steps) != 4 || wf.Steps[0].Purpose != "investigation" || wf.Steps[2].Purpose != "testing" {
		t.Fatal(wf)
	}
	opts := contextengine.Defaults()
	act := func(action, hash, handoff string, confirm bool) (any, error) {
		return service.Action(context.Background(), wf.ID, action, "", hash, handoff, opts, confirm)
	}
	if len(r.sent) != 0 {
		t.Fatal("auto-started")
	}
	if _, err = act("advance", "previewed", "do work", false); err == nil {
		t.Fatal("missing approval accepted")
	}
	if _, err = act("advance", "stale", "do work", true); err == nil {
		t.Fatal("stale preview")
	}
	result, err := act("advance", "previewed", "Investigate", true)
	if err != nil {
		t.Fatal(err)
	}
	first := result.(protocol.Session)
	if _, err = act("advance", "previewed", "Implement", true); err == nil {
		t.Fatal("advanced running step")
	}
	_ = db.AddEvent(protocol.Event{ID: workspace.ID(), SessionID: first.ID, Type: "text", Text: "Findings: stable invoice ID. api_key=secret123", CreatedAt: workspace.Now()})
	service.Reconcile(first.ID, "completed")
	rows, _ := service.List(w.ID)
	if rows[0].Status != "awaiting_approval" || rows[0].Steps[1].Status != "pending" || len(r.sent) != 1 {
		t.Fatal("automatic progression", rows)
	}
	plan, err := act("preview", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	prompt := plan.(map[string]any)["prompt"].(string)
	if !strings.Contains(prompt, "stable invoice ID") || strings.Contains(prompt, "secret123") {
		t.Fatal(prompt)
	}
	r.fail = true
	if _, err = act("advance", "previewed", prompt, true); err == nil {
		t.Fatal("provider failure ignored")
	}
	rows, _ = service.List(w.ID)
	if rows[0].Status != "failed" || rows[0].Steps[1].Status != "failed" {
		t.Fatal(rows)
	}
	if _, err = act("advance", "previewed", prompt, true); err == nil {
		t.Fatal("continued failed step")
	}
	if _, err = act("retry", "", "", true); err != nil {
		t.Fatal(err)
	}
	r.fail = false
	result, err = act("advance", "previewed", prompt, true)
	if err != nil {
		t.Fatal(err)
	}
	second := result.(protocol.Session)
	if _, err = act("stop", "", "", true); err != nil {
		t.Fatal(err)
	}
	service.Reconcile(second.ID, "cancelled")
	rows, _ = service.List(w.ID)
	if rows[0].Status != "stopped" {
		t.Fatal("stopped workflow resumed", rows)
	}
}
