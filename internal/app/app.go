package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"commitdeck/internal/agents"
	"commitdeck/internal/agents/discovery"
	"commitdeck/internal/config"
	"commitdeck/internal/dashboard"
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/github"
	"commitdeck/internal/issuetracker"
	"commitdeck/internal/jira"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/redact"
	"commitdeck/internal/secrets"
	"commitdeck/internal/terminal"
	"commitdeck/internal/workflows"
	"commitdeck/internal/workspace"
	"commitdeck/internal/workspacetools"
)

type App struct {
	Workflows *workflows.Service
	Discovery *discovery.Service
	Tools     *workspacetools.Service
	Terminals *terminal.Manager
	DB        *database.DB
	Agents    *agents.Manager
	Git       gitservice.GitService
	PR        pullrequest.PullRequestService
	Secrets   secrets.Store
	DataDir   string
	lifecycle sync.Mutex
}
type Settings struct {
	WorktreeRoot string `json:"worktreeRoot"`
	JiraBaseURL  string `json:"jiraBaseUrl"`
	JiraEmail    string `json:"jiraEmail"`
	JiraProject  string `json:"jiraProject"`
	JiraTokenSet bool   `json:"jiraTokenSet"`
}
type Summary = dashboard.Row

func Open(data string, emit func(any)) (*App, error) {
	if err := os.MkdirAll(data, 0700); err != nil {
		return nil, err
	}
	db, err := database.Open(filepath.Join(data, "commitdeck.db"))
	if err != nil {
		return nil, err
	}
	if err = db.RecoverSessions(); err != nil {
		db.Close()
		return nil, err
	}
	if err = db.RecoverPermissions(); err != nil {
		db.Close()
		return nil, err
	}
	a := &App{DB: db, Git: gitservice.CLI{}, PR: github.CLI{}, Secrets: secrets.Native{}, DataDir: data}
	a.Agents = agents.New(db, emit)
	a.Discovery = discovery.New(db)
	a.Tools, err = workspacetools.New(db, a.Agents.Permissions, a.Agents.Activity)
	if err != nil {
		a.Agents.Close()
		db.Close()
		return nil, err
	}
	a.Agents.Tools = a.Tools
	a.Workflows = &workflows.Service{DB: db, Agents: a.Agents, Context: a.Agents.Context, Activity: a.Agents.Activity}
	if err = a.Workflows.Recover(); err != nil {
		a.Tools.Close()
		a.Agents.Close()
		db.Close()
		return nil, err
	}
	a.Agents.OnFinished = a.Workflows.Reconcile
	a.Terminals, err = terminal.New(db, emit)
	if err != nil {
		a.Agents.Close()
		db.Close()
		return nil, err
	}
	return a, nil
}
func (a *App) Close() { a.Terminals.Close(); a.Agents.Close(); a.Tools.Close(); a.DB.Close() }
func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}
func (a *App) settings() (Settings, error) {
	var s Settings
	b, err := a.DB.Setting("integrations")
	if err != nil {
		return s, err
	}
	if b != "" {
		if err = json.Unmarshal([]byte(b), &s); err != nil {
			return s, err
		}
	}
	if s.WorktreeRoot == "" {
		s.WorktreeRoot = config.WorktreeRoot()
	}
	token, err := a.Secrets.Get("jira")
	s.JiraTokenSet = err == nil && token != ""
	return s, nil
}
func (a *App) issueTracker() (issuetracker.IssueTracker, error) {
	s, err := a.settings()
	if err != nil {
		return nil, err
	}
	token, err := a.Secrets.Get("jira")
	if err != nil {
		return nil, errors.New("Jira token unavailable in OS credential storage; configure it in Settings")
	}
	return jira.Client{BaseURL: s.JiraBaseURL, Email: s.JiraEmail, Token: token}, nil
}
func (a *App) summary(w workspace.Workspace) (Summary, error) {
	sessions, err := a.DB.Sessions(w.ID)
	if err != nil {
		return Summary{}, err
	}
	var p *pullrequest.PullRequest
	c, e := a.DB.Cache("pr", w.ID)
	if e == nil {
		_ = json.Unmarshal(c.Data, &p)
	}
	todos, err := a.DB.Todos(w.ID)
	if err != nil {
		return Summary{}, err
	}
	pending, err := a.DB.Permissions(w.ID, true)
	if err != nil {
		return Summary{}, err
	}
	completed := 0
	for _, t := range todos {
		if t.Completed {
			completed++
		}
	}
	var waiting, failed int
	if err = a.DB.QueryRow("SELECT COALESCE(SUM(CASE WHEN status='awaiting_approval' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0) FROM agent_workflows WHERE workspace_id=?", w.ID).Scan(&waiting, &failed); err != nil {
		return Summary{}, err
	}
	state := dashboard.StateWithWorkflows(w, sessions, p, len(pending), waiting, failed)
	return Summary{WorkflowWaiting: waiting, WorkflowFailed: failed, Workspace: w, Attention: state, Group: dashboard.Group(state), Sessions: sessions, PR: p, TasksTotal: len(todos), TasksCompleted: completed, PendingPermissions: len(pending), LastActivity: w.UpdatedAt}, nil
}
func (a *App) bootstrap() (any, error) {
	out, err := dashboard.Load(a.DB)
	if err != nil {
		return nil, err
	}
	s, err := a.settings()
	if err != nil {
		return nil, err
	}
	layout, err := a.DB.Setting("layout")
	if err != nil {
		return nil, err
	}
	var active int
	if err = a.DB.QueryRow("SELECT COUNT(*) FROM agent_sessions WHERE pid>0 AND status IN ('starting','running','waiting_permission')").Scan(&active); err != nil {
		return nil, err
	}
	permissions, err := a.DB.Permissions("", true)
	if err != nil {
		return nil, err
	}
	notifications, err := a.DB.Activity("", true)
	if err != nil {
		return nil, err
	}
	return map[string]any{"workspaces": out, "settings": s, "providers": a.Agents.Providers(), "layout": layout, "dataDir": a.DataDir, "activeAgents": active, "permissions": permissions, "notifications": notifications}, nil
}
func (a *App) detail(id string) (any, error) {
	w, err := a.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	s, err := a.summary(w)
	if err != nil {
		return nil, err
	}
	note, err := a.DB.Note(id)
	if err != nil {
		return nil, err
	}
	todos, err := a.DB.Todos(id)
	if err != nil {
		return nil, err
	}
	sessions, err := a.DB.Sessions(id)
	if err != nil {
		return nil, err
	}
	jc, err := a.DB.Cache("jira", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	pc, err := a.DB.Cache("pr", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	permissions, err := a.DB.Permissions(id, false)
	if err != nil {
		return nil, err
	}
	events, err := a.DB.Activity(id, false)
	if err != nil {
		return nil, err
	}
	rules, err := a.DB.PermissionRules(id)
	if err != nil {
		return nil, err
	}
	terminals, err := a.Terminals.List(id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"terminals": terminals, "workspace": s, "note": note, "todos": todos, "sessions": sessions, "jira": jc, "pr": pc, "permissions": permissions, "activity": events, "permissionRules": rules}, nil
}
func (a *App) Call(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		ID              string            `json:"id"`
		WorkspaceID     string            `json:"workspaceId"`
		ExpectedContent *string           `json:"expectedContent"`
		ExpectedHash    string            `json:"expectedHash"`
		Content         string            `json:"content"`
		IDs             []string          `json:"ids"`
		File            string            `json:"file"`
		Path            string            `json:"path"`
		Branch          string            `json:"branch"`
		Base            string            `json:"base"`
		Key             string            `json:"key"`
		Token           string            `json:"token"`
		Confirm         bool              `json:"confirm"`
		Archived        bool              `json:"archived"`
		Prompt          string            `json:"prompt"`
		Action          string            `json:"action"`
		Settings        Settings          `json:"settings"`
		Drafts          map[string]string `json:"drafts"`
	}
	if err := decode(raw, &p); err != nil {
		return nil, errors.New("invalid request")
	}
	if strings.HasPrefix(method, "skill.") || strings.HasPrefix(method, "discovery.") || strings.HasPrefix(method, "lifecycle.") || strings.HasPrefix(method, "pr.write") || strings.HasPrefix(method, "pr.template") || strings.HasPrefix(method, "jira.write") || method == "jira.transitions" || strings.HasPrefix(method, "workflow.") || method == "git.openFile" || method == "git.agentDiff" || method == "pr.reviewContext" {
		return a.daily(ctx, method, raw)
	}
	if strings.HasPrefix(method, "terminal.") {
		return a.terminalCall(ctx, method, raw)
	}
	switch method {
	case "drafts.redact":
		if len(p.Drafts) > 500 || len(raw) > 4<<20 {
			return nil, errors.New("prompt recovery backup exceeds its size limit")
		}
		out := make(map[string]string, len(p.Drafts))
		for id, content := range p.Drafts {
			if len(id) > 128 || len(content) > 1<<20 {
				return nil, errors.New("invalid prompt recovery draft")
			}
			out[id] = redact.Text(content)
		}
		return out, nil
	case "bootstrap":
		return a.bootstrap()
	case "workspace.get":
		return a.detail(p.ID)
	case "workspace.create":
		var w workspace.Workspace
		if err := decode(raw, &w); err != nil {
			return nil, err
		}
		w.RepoPath = ""
		w.WorktreePath = ""
		w.Branch = ""
		w.JiraTicketID = ""
		w.Archived = false
		return a.DB.CreateWorkspace(w)
	case "workspace.update":
		var edit workspace.Workspace
		if err := decode(raw, &edit); err != nil {
			return nil, err
		}
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		w.Title = edit.Title
		w.ExternalKey = edit.ExternalKey
		w.Status = edit.Status
		return a.DB.UpdateWorkspace(w)
	case "workspace.archive":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		if a.Agents.ActiveWorkspace(p.ID) || a.Terminals.Active(p.ID) {
			return nil, errors.New("cancel active agents and close running terminals before archiving")
		}
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		w.Archived = p.Archived
		if w.Archived {
			w.Lifecycle = "ARCHIVED"
		} else {
			w.Lifecycle = "BACKLOG"
			if w.WorktreePath != "" {
				w.Lifecycle = "CODING"
			}
		}
		return a.DB.UpdateWorkspace(w)
	case "workspace.delete":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		if !p.Confirm {
			return nil, errors.New("deleting workspace data requires confirmation")
		}
		if a.Agents.ActiveWorkspace(p.ID) || a.Terminals.Active(p.ID) {
			return nil, errors.New("cancel active agents and close running terminals before deleting")
		}
		return nil, a.DB.DeleteWorkspace(p.ID)
	case "note.hash":
		if len(p.Content) > 4<<20 {
			return nil, errors.New("note exceeds 4 MB")
		}
		return database.NoteHash(p.Content), nil
	case "note.save":
		if p.ExpectedHash != "" {
			return nil, a.DB.SaveNoteWithHash(p.WorkspaceID, p.Content, p.ExpectedHash)
		}
		if p.ExpectedContent != nil {
			return nil, a.DB.SaveNoteIfUnchanged(p.WorkspaceID, p.Content, *p.ExpectedContent)
		}
		return nil, a.DB.SaveNote(p.WorkspaceID, p.Content)
	case "todo.save":
		var t workspace.Todo
		if err := decode(raw, &t); err != nil {
			return nil, err
		}
		t.UpdatedBy = "user"
		t.SessionID = ""
		saved, err := a.DB.SaveTodo(t)
		if err == nil {
			kind := "todo_updated"
			title := "Updated TODO: " + t.Title
			if t.ID == "" {
				kind = "todo_added"
				title = "Added TODO: " + t.Title
			}
			if t.Completed {
				kind = "todo_completed"
				title = "Completed TODO: " + t.Title
			}
			_ = a.Agents.Activity.Record(t.WorkspaceID, kind, title, "", false)
		}
		return saved, err
	case "todo.delete":
		return nil, a.DB.DeleteTodo(p.WorkspaceID, p.ID)
	case "todo.reorder":
		return nil, a.DB.ReorderTodos(p.WorkspaceID, p.IDs)
	case "settings.save":
		s := p.Settings
		if s.JiraBaseURL != "" {
			if err := jira.ValidateURL(s.JiraBaseURL); err != nil {
				return nil, err
			}
		}
		if s.WorktreeRoot == "" {
			s.WorktreeRoot = config.WorktreeRoot()
		}
		if !filepath.IsAbs(s.WorktreeRoot) && !strings.HasPrefix(s.WorktreeRoot, "~/") {
			return nil, errors.New("worktree root must be absolute or start with ~/")
		}
		if p.Token != "" {
			if err := a.Secrets.Set("jira", p.Token); err != nil {
				return nil, fmt.Errorf("could not save token in native credential storage: %w", err)
			}
		}
		s.JiraTokenSet = false
		b, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		if err = a.DB.SetSetting("integrations", string(b)); err != nil {
			return nil, err
		}
		return a.settings()
	case "settings.clearToken":
		return nil, a.Secrets.Delete("jira")
	case "layout.save":
		if len(p.Content) > 32768 || !json.Valid([]byte(p.Content)) {
			return nil, errors.New("invalid layout")
		}
		return nil, a.DB.SetSetting("layout", p.Content)
	case "repo.attach":
		return a.attach(p.ID, p.Path)
	case "git.status":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		return a.Git.Status(workingPath(w))
	case "git.diff":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		return a.Git.Diff(workingPath(w), p.File)
	case "worktree.create":
		return a.createWorktree(p.ID, p.Branch, p.Base)
	case "worktree.remove":
		return a.removeWorktree(p.ID, p.Confirm)
	case "workspace.open":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		return nil, openDirectory(workingPath(w), p.Action)
	case "session.create":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		w, err := a.DB.Workspace(p.WorkspaceID)
		if err != nil {
			return nil, err
		}
		var options agents.SessionOptions
		if err = decode(raw, &options); err != nil {
			return nil, err
		}
		return a.Agents.StartSession(ctx, w, options)
	case "session.events":
		return a.DB.Events(p.ID)
	case "session.resume":
		return a.Agents.ResumeSession(ctx, p.ID)
	case "session.send":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		return nil, a.Agents.Send(ctx, p.ID, p.Prompt)
	case "session.cancel":
		return nil, a.Agents.Cancel(ctx, p.ID)
	case "context.build", "context.refresh", "session.context", "permissions.resolve", "permissions.rules.delete", "notifications.read":
		return a.control(ctx, method, raw)
	case "jira.fetch":
		tracker, err := a.issueTracker()
		if err != nil {
			return nil, err
		}
		return tracker.GetIssue(ctx, p.Key)
	case "jira.import":
		return a.importIssue(ctx, p.Key)
	case "jira.refresh":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		tracker, err := a.issueTracker()
		if err != nil {
			return nil, err
		}
		issue, err := tracker.GetIssue(ctx, w.ExternalKey)
		if err != nil {
			return nil, err
		}
		if err = a.DB.PutCache("jira", w.ID, issue); err != nil {
			return nil, err
		}
		_ = a.Agents.Activity.Transition(w.ID, "jira", issue.UpdatedAt, "workspace_refreshed", "Jira ticket refreshed", false)
		return a.DB.Cache("jira", w.ID)
	case "pr.refresh":
		return a.refreshPR(ctx, p.ID)
	}
	return nil, fmt.Errorf("unknown method: %s", method)
}
