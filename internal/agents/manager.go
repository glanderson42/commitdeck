package agents

import (
	"bufio"
	"commitdeck/internal/activity"
	contextengine "commitdeck/internal/context"
	"commitdeck/internal/permissions"
	"commitdeck/internal/pullrequest"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"commitdeck/internal/agents/claude"
	"commitdeck/internal/agents/codex"
	"commitdeck/internal/agents/opencode"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/workspace"
)

type AgentProvider interface {
	StartSession(context.Context, workspace.Workspace, SessionOptions) (protocol.Session, error)
	ResumeSession(context.Context, string) (protocol.Session, error)
	Send(context.Context, string, string) error
	Cancel(context.Context, string) error
}
type SessionOptions struct {
	Provider       string                 `json:"provider"`
	Title          string                 `json:"title"`
	Purpose        string                 `json:"purpose"`
	ContextOptions *contextengine.Options `json:"contextOptions"`
	ContextHash    string                 `json:"contextHash"`
}
type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}
type running struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}
type Manager struct {
	db          *database.DB
	Context     *contextengine.Engine
	Permissions *permissions.Service
	Activity    *activity.Service
	providers   map[string]protocol.Runtime
	mu          sync.Mutex
	active      map[string]*running
	emit        func(any)
	wg          sync.WaitGroup
	closed      bool
}

func New(db *database.DB, emit func(any)) *Manager {
	m := &Manager{db: db, emit: emit, providers: map[string]protocol.Runtime{}, active: map[string]*running{}}
	m.Context = &contextengine.Engine{DB: db, Git: gitservice.CLI{}}
	m.Activity = &activity.Service{Store: db, Emit: emit}
	m.Permissions = permissions.New(db, m.Activity, emit)
	for _, p := range []protocol.Runtime{codex.Provider{}, claude.Provider{}, opencode.Provider{}} {
		m.providers[p.ID()] = p
	}
	return m
}
func (m *Manager) Providers() []Info {
	out := []Info{}
	for _, id := range []string{"codex", "claude", "opencode"} {
		p := m.providers[id]
		out = append(out, Info{p.ID(), p.Name(), p.IsAvailable(context.Background())})
	}
	return out
}
func (m *Manager) StartSession(ctx context.Context, w workspace.Workspace, o SessionOptions) (protocol.Session, error) {
	p := m.providers[o.Provider]
	if p == nil {
		return protocol.Session{}, errors.New("unknown provider")
	}
	if !p.IsAvailable(ctx) {
		return protocol.Session{}, fmt.Errorf("%s is not installed or not on PATH", p.Name())
	}
	if w.Archived {
		return protocol.Session{}, errors.New("restore this workspace first")
	}
	if err := validateWorktree(w); err != nil {
		return protocol.Session{}, err
	}
	o.Title = strings.TrimSpace(o.Title)
	if o.Title == "" {
		o.Title = "Implementation"
	}
	if len(o.Title) > 200 {
		return protocol.Session{}, errors.New("session title too long")
	}
	if o.Purpose == "" {
		o.Purpose = "implementation"
	}
	o.Purpose = contextengine.Purpose(o.Purpose)
	opts := contextengine.Defaults()
	if o.ContextOptions != nil {
		opts = *o.ContextOptions
	}
	generated, err := m.Context.BuildWorkspaceContext(ctx, w.ID, opts, o.Purpose)
	if err != nil {
		return protocol.Session{}, err
	}
	if o.ContextHash != "" && o.ContextHash != generated.Hash {
		return protocol.Session{}, errors.New("workspace context changed; preview it again before creating the session")
	}
	s, err := m.db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: o.Provider, Title: o.Title, Purpose: o.Purpose})
	if err == nil {
		if err = m.Context.Save(s, generated); err != nil {
			_, _ = m.db.Exec("DELETE FROM agent_sessions WHERE id=?", s.ID)
			return protocol.Session{}, err
		}
		m.emit(map[string]any{"type": "sessions_changed", "workspaceId": w.ID})
	}
	return s, err
}
func validateWorktree(w workspace.Workspace) error {
	if w.WorktreePath == "" {
		return errors.New("create an isolated Git worktree before starting an agent")
	}
	if err := (gitservice.CLI{}).ValidateRepo(w.WorktreePath); err != nil {
		return fmt.Errorf("worktree unavailable: %w", err)
	}
	return nil
}
func (m *Manager) ResumeSession(ctx context.Context, id string) (protocol.Session, error) {
	s, err := m.db.Session(id)
	if err != nil {
		return s, err
	}
	if s.ExternalSessionID == "" {
		return s, errors.New("no provider session yet; send the first prompt")
	}
	return s, nil
}
func (m *Manager) event(id string, e protocol.Event) error {
	e.ID = workspace.ID()
	e.SessionID = id
	e.CreatedAt = workspace.Now()
	if err := m.db.AddEvent(e); err != nil {
		return err
	}
	m.emit(map[string]any{"type": "agent_event", "event": e})
	if e.Type == "file_changed" {
		session, err := m.db.Session(id)
		if err == nil {
			_ = m.Activity.Record(session.WorkspaceID, "file_changed", "Updated "+e.FilePath, e.ID, false)
		}
	}
	return nil
}
func (m *Manager) Send(_ context.Context, id, prompt string) error {
	if strings.TrimSpace(prompt) == "" || len(prompt) > 1<<20 {
		return errors.New("prompt must contain 1 byte to 1 MB")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("application is shutting down")
	}
	if m.active[id] != nil {
		return errors.New("session is already running")
	}
	s, err := m.db.Session(id)
	if err != nil {
		return err
	}
	w, err := m.db.Workspace(s.WorkspaceID)
	if err != nil {
		return err
	}
	if w.Archived {
		return errors.New("restore this workspace first")
	}
	if err = validateWorktree(w); err != nil {
		return err
	}
	p := m.providers[s.Provider]
	if p == nil {
		return errors.New("unknown provider")
	}
	if !p.IsAvailable(context.Background()) {
		return errors.New("provider binary not found")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := p.Command(ctx, w.WorktreePath, s.ExternalSessionID, prompt)
	contextPrompt, snapshotID, err := m.Context.Prepare(ctx, s, prompt)
	if err != nil {
		cancel()
		return err
	}
	var conversation protocol.Conversation
	if interactive, ok := p.(protocol.InteractiveRuntime); ok {
		conversation = interactive.NewConversation(s.ExternalSessionID, w.WorktreePath, contextPrompt)
	}
	var input io.WriteCloser
	var inputMu sync.Mutex
	var inputClosed bool
	var ackMu sync.Mutex
	acks := map[string]chan error{}
	writeInput := func(v any) error {
		inputMu.Lock()
		defer inputMu.Unlock()
		if inputClosed {
			return errors.New("provider input is closed")
		}
		return json.NewEncoder(input).Encode(v)
	}
	closeInput := func() {
		inputMu.Lock()
		defer inputMu.Unlock()
		if !inputClosed && input != nil {
			inputClosed = true
			_ = input.Close()
		}
	}
	if conversation != nil {
		input, err = cmd.StdinPipe()
		if err != nil {
			cancel()
			return err
		}
	} else {
		cmd.Stdin = strings.NewReader(contextPrompt)
	}
	isolate(cmd)
	cmd.Cancel = func() error { return terminate(cmd) }
	cmd.WaitDelay = 2 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	if err = m.event(id, protocol.Event{Type: "text", ToolName: "user", Text: prompt}); err != nil {
		cancel()
		return err
	}
	if err = m.db.SessionStatus(id, "starting", 0); err != nil {
		cancel()
		closeInput()
		return err
	}
	if err = cmd.Start(); err != nil {
		cancel()
		_ = m.db.SessionStatus(id, "failed", 0)
		return err
	}
	r := &running{cmd: cmd, cancel: cancel, done: make(chan struct{})}
	m.active[id] = r
	if err = m.db.SessionStatus(id, "running", cmd.Process.Pid); err != nil {
		cancel()
		_ = cmd.Wait()
		delete(m.active, id)
		return err
	}
	turnID := workspace.ID()
	_ = m.Activity.Record(w.ID, "agent_started", s.Provider+" started "+s.Purpose, turnID+":started", false)
	if conversation != nil {
		for _, frame := range conversation.Initial() {
			if err = writeInput(frame); err != nil {
				cancel()
				break
			}
		}
	} else {
		_ = m.Context.MarkSent(snapshotID)
	}
	m.emit(map[string]any{"type": "sessions_changed", "workspaceId": w.ID})
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(r.done)
		defer cancel()
		var readers sync.WaitGroup
		var stateMu sync.Mutex
		failed, waiting, finished := false, false, false
		var readErr error
		readers.Add(2)
		go func() {
			defer readers.Done()
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 4096), 8<<20)
			for scanner.Scan() {
				var frame protocol.Frame
				if conversation != nil {
					frame = conversation.Handle(scanner.Bytes())
				} else {
					frame.Events, frame.ExternalID = p.Normalize(scanner.Bytes())
				}
				events, external := frame.Events, frame.ExternalID
				if frame.AckID != "" {
					ackMu.Lock()
					if ch := acks[frame.AckID]; ch != nil {
						var ackErr error
						if frame.AckError != "" {
							ackErr = errors.New(frame.AckError)
						}
						ch <- ackErr
						delete(acks, frame.AckID)
					}
					ackMu.Unlock()
				}
				for _, v := range frame.Writes {
					if er := writeInput(v); er != nil {
						stateMu.Lock()
						readErr = er
						stateMu.Unlock()
						cancel()
					}
				}
				if frame.InputSent {
					_ = m.Context.MarkSent(snapshotID)
				}
				if frame.Permission != nil {
					request := *frame.Permission
					request.WorkspaceID = w.ID
					request.SessionID = id
					request.Provider = s.Provider
					if request.Cwd == "" {
						request.Cwd = w.WorktreePath
						request.UnverifiedCwd = true
					}
					var reply func(bool) error
					if frame.Reply != nil {
						reply = func(allow bool) error {
							if !frame.RequiresAck {
								return writeInput(frame.Reply(allow))
							}
							ch := make(chan error, 1)
							ackMu.Lock()
							acks[frame.WireID] = ch
							ackMu.Unlock()
							defer func() { ackMu.Lock(); delete(acks, frame.WireID); ackMu.Unlock() }()
							if err := writeInput(frame.Reply(allow)); err != nil {
								return err
							}
							select {
							case err := <-ch:
								return err
							case <-ctx.Done():
								return errors.New("approval process stopped")
							case <-time.After(15 * time.Second):
								return errors.New("provider did not acknowledge the permission decision")
							}
						}
					}
					if _, er := m.Permissions.Request(request, reply); er != nil {
						stateMu.Lock()
						readErr = er
						stateMu.Unlock()
						cancel()
					}
				}
				if frame.Done {
					stateMu.Lock()
					finished = true
					if frame.Failed {
						failed = true
					}
					stateMu.Unlock()
					closeInput()
				}
				if frame.Failed {
					stateMu.Lock()
					failed = true
					stateMu.Unlock()
				}
				if external != "" {
					if e := m.db.ExternalSession(id, external); e != nil {
						stateMu.Lock()
						readErr = e
						stateMu.Unlock()
					}
				}
				for _, e := range events {
					stateMu.Lock()
					if e.Type == "error" {
						failed = true
					}
					if e.Type == "permission_request" {
						waiting = true
						_ = m.db.SessionStatus(id, "waiting_permission", cmd.Process.Pid)
						_, _ = m.Permissions.Request(permissions.Request{WorkspaceID: w.ID, SessionID: id, Provider: s.Provider, Title: e.ToolName, Description: e.Text, Command: e.Command, FilePath: e.FilePath, Cwd: w.WorktreePath}, nil)
					}
					if er := m.event(id, e); er != nil {
						readErr = er
					}
					stateMu.Unlock()
				}
			}
			if e := scanner.Err(); e != nil {
				stateMu.Lock()
				readErr = e
				stateMu.Unlock()
			}
		}()
		go func() {
			defer readers.Done()
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 4096), 1<<20)
			var lines []string
			for scanner.Scan() {
				if len(lines) < 20 {
					line := scanner.Text()
					if !strings.HasPrefix(strings.TrimSpace(line), "{") {
						lines = append(lines, line)
					}
				}
			}
			if len(lines) > 0 {
				_ = m.event(id, protocol.Event{Type: "status", Text: strings.Join(lines, "\n")})
			}
		}()
		readers.Wait()
		err := cmd.Wait()
		status := "completed"
		stateMu.Lock()
		if readErr != nil {
			failed = true
			_ = m.event(id, protocol.Event{Type: "error", Error: "Event stream could not be persisted: " + readErr.Error()})
		}
		if failed || err != nil {
			status = "failed"
		}
		if waiting {
			status = "idle"
		}
		stateMu.Unlock()
		if ctx.Err() != nil && !finished {
			status = "cancelled"
		} else if err != nil {
			_ = m.event(id, protocol.Event{Type: "error", Error: err.Error() + ". Check provider authentication in a terminal."})
		}
		closeInput()
		m.Permissions.ExpireSession(id, w.ID)
		_ = m.event(id, protocol.Event{Type: "complete", Text: status})
		_ = m.Activity.Record(w.ID, "agent_"+status, s.Provider+" "+s.Purpose+" "+status, turnID+":"+status, status == "completed" || status == "failed")
		m.mu.Lock()
		_ = m.db.SessionStatus(id, status, 0)
		delete(m.active, id)
		m.mu.Unlock()
		if cached, e := m.db.Cache("pr", w.ID); e == nil {
			var pr *pullrequest.PullRequest
			if json.Unmarshal(cached.Data, &pr) == nil {
				sessions, _ := m.db.Sessions(w.ID)
				state := workspace.Attention(sessions, pr)
				_ = m.Activity.Transition(w.ID, "ready", state, "attention_changed", "Workspace "+state, state == "READY_TO_MERGE")
			}
		}
		m.emit(map[string]any{"type": "sessions_changed", "workspaceId": w.ID})
	}()
	return nil
}
func (m *Manager) Cancel(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.active[id]
	if r == nil {
		return errors.New("session is not running")
	}
	r.cancel()
	return nil
}
func (m *Manager) ActiveWorkspace(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for sid := range m.active {
		s, e := m.db.Session(sid)
		if e == nil && s.WorkspaceID == id {
			return true
		}
	}
	return false
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, r := range m.active {
		r.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}
