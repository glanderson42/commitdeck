package workflows

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents"
	contextengine "commitdeck/internal/context"
	"commitdeck/internal/database"
	"commitdeck/internal/identity"
	"commitdeck/internal/redact"
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
)

type Step struct {
	ID         string `json:"id"`
	WorkflowID string `json:"workflowId"`
	Position   int    `json:"position"`
	Purpose    string `json:"purpose"`
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	SessionID  string `json:"sessionId"`
	Handoff    string `json:"handoff"`
}
type Workflow struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Steps       []Step `json:"steps"`
}
type Service struct {
	DB       *database.DB
	Agents   agents.AgentProvider
	Context  contextengine.ContextEngine
	Activity *activity.Service
	mu       sync.Mutex
}

func (s *Service) List(workspaceID string) ([]Workflow, error) {
	rows, err := s.DB.Query("SELECT id,workspace_id,name,status,created_at,updated_at FROM agent_workflows WHERE workspace_id=? ORDER BY created_at DESC", workspaceID)
	if err != nil {
		return nil, err
	}
	out := []Workflow{}
	for rows.Next() {
		var w Workflow
		if err = rows.Scan(&w.ID, &w.WorkspaceID, &w.Name, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Steps, err = s.steps(out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Service) steps(id string) ([]Step, error) {
	rows, err := s.DB.Query("SELECT id,workflow_id,position,purpose,provider,status,COALESCE(session_id,''),handoff FROM agent_workflow_steps WHERE workflow_id=? ORDER BY position", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Step{}
	for rows.Next() {
		var v Step
		if err = rows.Scan(&v.ID, &v.WorkflowID, &v.Position, &v.Purpose, &v.Provider, &v.Status, &v.SessionID, &v.Handoff); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) Create(workspaceID, name string, providers []string) (Workflow, error) {
	purposes := map[string][]string{"Implement Feature": {"investigation", "implementation", "testing", "review"}, "Fix Bug": {"debugging", "implementation", "testing", "review"}, "Investigate": {"investigation"}, "Review PR": {"review"}}
	ps := purposes[name]
	if len(ps) == 0 || len(providers) != len(ps) {
		return Workflow{}, errors.New("choose a supported preset and a provider for each step")
	}
	w, err := s.DB.Workspace(workspaceID)
	if err != nil {
		return Workflow{}, err
	}
	if w.Archived || w.WorktreePath == "" {
		return Workflow{}, errors.New("an active worktree is required")
	}
	for _, p := range providers {
		if p != "codex" && p != "claude" && p != "opencode" {
			return Workflow{}, errors.New("unknown provider")
		}
	}
	out := Workflow{ID: identity.ID(), WorkspaceID: workspaceID, Name: name, Status: "awaiting_approval", CreatedAt: identity.Now(), UpdatedAt: identity.Now(), Steps: []Step{}}
	tx, err := s.DB.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO agent_workflows VALUES(?,?,?,?,?,?)", out.ID, workspaceID, name, out.Status, out.CreatedAt, out.UpdatedAt)
	if err != nil {
		return out, err
	}
	for i, p := range ps {
		st := Step{ID: identity.ID(), WorkflowID: out.ID, Position: i, Purpose: p, Provider: providers[i], Status: "pending"}
		_, err = tx.Exec("INSERT INTO agent_workflow_steps(id,workflow_id,position,purpose,provider,status) VALUES(?,?,?,?,?,?)", st.ID, out.ID, i, p, st.Provider, st.Status)
		if err != nil {
			return out, err
		}
		out.Steps = append(out.Steps, st)
	}
	return out, tx.Commit()
}
func (s *Service) Recover() error {
	_, err := s.DB.Exec("UPDATE agent_workflow_steps SET status='failed' WHERE status='running'; UPDATE agent_workflows SET status='failed' WHERE status='running'")
	return err
}
func (s *Service) Reconcile(sessionID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var workflowID string
	err := s.DB.QueryRow("SELECT workflow_id FROM agent_workflow_steps WHERE session_id=? AND status='running'", sessionID).Scan(&workflowID)
	if err != nil {
		return
	}
	var workflowStatus string
	_ = s.DB.QueryRow("SELECT status FROM agent_workflows WHERE id=?", workflowID).Scan(&workflowStatus)
	if workflowStatus == "stopped" {
		_, _ = s.DB.Exec("UPDATE agent_workflow_steps SET status='cancelled' WHERE session_id=? AND status='running'", sessionID)
		return
	}
	next := "failed"
	if status == "completed" {
		next = "completed"
	}
	summary := ""
	events, err := s.DB.Events(sessionID)
	if err == nil {
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Type == "text" && events[i].ToolName != "user" {
				summary = events[i].Text
				break
			}
		}
	}
	summary = redact.Text(summary)
	runes := []rune(summary)
	if len(runes) > 3000 {
		summary = string(runes[:3000]) + "\n[Handoff truncated; inspect the session for full findings.]"
	}
	_, _ = s.DB.Exec("UPDATE agent_workflow_steps SET status=?,handoff=? WHERE session_id=? AND status='running'", next, summary, sessionID)
	var remaining int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM agent_workflow_steps WHERE workflow_id=? AND status NOT IN ('completed','skipped')", workflowID).Scan(&remaining)
	wfStatus := "awaiting_approval"
	if next == "failed" {
		wfStatus = "failed"
	}
	if remaining == 0 {
		wfStatus = "completed"
	}
	_, _ = s.DB.Exec("UPDATE agent_workflows SET status=?,updated_at=? WHERE id=?", wfStatus, identity.Now(), workflowID)
	var workspaceID string
	_ = s.DB.QueryRow("SELECT workspace_id FROM agent_workflows WHERE id=?", workflowID).Scan(&workspaceID)
	_ = s.Activity.Record(workspaceID, "workflow_"+next, "Workflow step "+next, sessionID+":workflow:"+next, true)
}
func (s *Service) Action(ctx context.Context, id, action, provider, hash, handoff string, opts contextengine.Options, confirm bool) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var workspaceID, status string
	if err := s.DB.QueryRow("SELECT workspace_id,status FROM agent_workflows WHERE id=?", id).Scan(&workspaceID, &status); err != nil {
		return nil, err
	}
	steps, err := s.steps(id)
	if err != nil {
		return nil, err
	}
	if status == "stopped" || status == "completed" {
		return nil, errors.New("workflow is finished")
	}
	var current *Step
	for i := range steps {
		if steps[i].Status != "completed" && steps[i].Status != "skipped" {
			current = &steps[i]
			break
		}
	}
	if current == nil {
		return nil, sql.ErrNoRows
	}
	if action == "stop" {
		if !confirm {
			return nil, errors.New("confirm stopping this workflow")
		}
		if current.SessionID != "" && current.Status == "running" {
			_ = s.Agents.Cancel(ctx, current.SessionID)
		}
		_, err = s.DB.Exec("UPDATE agent_workflows SET status='stopped',updated_at=? WHERE id=?", identity.Now(), id)
		return nil, err
	}
	if current.Status == "running" {
		return nil, errors.New("wait for or cancel the current step")
	}
	if action == "retry" || action == "skip" || action == "provider" {
		if !confirm {
			return nil, errors.New("confirm workflow change")
		}
		if action == "retry" && current.Status != "failed" {
			return nil, errors.New("only failed steps can be retried")
		}
		if provider != "" {
			if provider != "codex" && provider != "claude" && provider != "opencode" {
				return nil, errors.New("unknown provider")
			}
			_, err = s.DB.Exec("UPDATE agent_workflow_steps SET provider=? WHERE id=?", provider, current.ID)
			if err != nil {
				return nil, err
			}
		}
		newStatus := "pending"
		if action == "skip" {
			newStatus = "skipped"
		}
		_, err = s.DB.Exec("UPDATE agent_workflow_steps SET status=? WHERE id=?", newStatus, current.ID)
		if err != nil {
			return nil, err
		}
		wf := "awaiting_approval"
		var remaining int
		_ = s.DB.QueryRow("SELECT COUNT(*) FROM agent_workflow_steps WHERE workflow_id=? AND status NOT IN ('completed','skipped')", id).Scan(&remaining)
		if remaining == 0 {
			wf = "completed"
		}
		_, err = s.DB.Exec("UPDATE agent_workflows SET status=?,updated_at=? WHERE id=?", wf, identity.Now(), id)
		return nil, err
	}
	prompt := "Perform the " + current.Purpose + " step for this ticket. Keep TODOs and notes current where workspace tools are enabled. Finish with a concise handoff: findings, actual changes, tests actually run, blockers, and recommended next work."
	if current.Position > 0 {
		previous := steps[current.Position-1]
		if previous.Handoff != "" {
			prompt += "\n\n## Previous step (agent output; verify against the repository)\n" + previous.Handoff
		}
	}
	if action == "preview" {
		generated, e := s.Context.BuildWorkspaceContext(ctx, workspaceID, opts, current.Purpose)
		return map[string]any{"step": current, "prompt": prompt, "context": generated}, e
	}
	if action != "advance" || !confirm || hash == "" {
		return nil, errors.New("preview and confirm the next step")
	}
	if current.Status == "failed" {
		return nil, errors.New("retry or skip the failed step first")
	}
	if strings.TrimSpace(handoff) == "" || len(handoff) > 16000 {
		return nil, errors.New("review the step prompt before starting")
	}
	w, err := s.DB.Workspace(workspaceID)
	if err != nil {
		return nil, err
	}
	session, err := s.Agents.StartSession(ctx, w, agents.SessionOptions{Provider: current.Provider, Title: "Workflow · " + current.Purpose, Purpose: current.Purpose, ContextOptions: &opts, ContextHash: hash})
	if err != nil {
		return nil, err
	}
	_, err = s.DB.Exec("UPDATE agent_workflow_steps SET status='running',session_id=? WHERE id=?", session.ID, current.ID)
	if err != nil {
		return nil, err
	}
	_, err = s.DB.Exec("UPDATE agent_workflows SET status='running',updated_at=? WHERE id=?", identity.Now(), id)
	if err != nil {
		return nil, err
	}
	if err = s.Agents.Send(ctx, session.ID, handoff); err != nil {
		_, _ = s.DB.Exec("UPDATE agent_workflow_steps SET status='failed' WHERE id=?", current.ID)
		_, _ = s.DB.Exec("UPDATE agent_workflows SET status='failed' WHERE id=?", id)
		_ = s.Activity.Record(workspaceID, "workflow_failed", "Workflow step could not start: "+err.Error(), session.ID+":workflow:failed", true)
		if persisted, e := s.DB.Session(session.ID); e == nil && persisted.PID == 0 {
			_ = s.DB.SessionStatus(session.ID, "failed", 0)
		}
		return nil, err
	}
	_ = s.Activity.Record(workspaceID, "workflow_started", "Started "+current.Provider+" "+current.Purpose, session.ID+":workflow:start", false)
	return session, nil
}
