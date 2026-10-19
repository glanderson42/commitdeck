// Package workspacetools exposes a narrow, session-scoped API. It never accepts a workspace ID or SQL from a caller.
package workspacetools

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/identity"
	"commitdeck/internal/permissions"
	"commitdeck/internal/redact"
	"commitdeck/internal/workspace"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Instructions = `## CommitDeck workspace tools

You may maintain this workspace through the session-scoped local tool helper. Never read or modify CommitDeck's database, credentials, or provider stores.
Invoke the executable in COMMITDECK_TOOL_BIN with arguments --workspace-tool OPERATION JSON. The helper obtains its scoped connection from the environment; never print or save those environment values.
Operations: workspace.get, todo.list, todo.add {"title":"..."}, todo.update {"id":"...","title":"..."}, todo.complete {"id":"...","completed":true}, todo.delete {"id":"..."}, notes.get, notes.append {"content":"..."}, notes.update {"content":"...","expectedUpdatedAt":"..."}, activity.add {"content":"..."}.
Add concrete discovered work to TODOs; complete only finished work. Append important decisions and blockers to notes, avoiding chat dumps and trivial updates. Do not overwrite user notes without explicit instruction. Deletes and all note replacements require individual approval in CommitDeck. Read tools return only this workspace. Provider tool permissions still apply.
PowerShell uses & $env:COMMITDECK_TOOL_BIN --workspace-tool OPERATION JSON.
Example shell invocation: "$COMMITDECK_TOOL_BIN" --workspace-tool todo.add '{"title":"Add permanent failure test"}'
`

type binding struct {
	session protocol.Session
	ctx     context.Context
}
type Service struct {
	DB          *database.DB
	Permissions *permissions.Service
	Activity    *activity.Service
	server      *http.Server
	url         string
	mu          sync.Mutex
	bindings    map[string]binding
	edit        sync.Mutex
}

func New(db *database.DB, p *permissions.Service, a *activity.Service) (*Service, error) {
	return &Service{DB: db, Permissions: p, Activity: a, bindings: map[string]binding{}}, nil
}
func (s *Service) listen() error {
	if s.server != nil {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("workspace tool service could not bind a local port")
	}
	s.url = "http://" + ln.Addr().String()
	s.server = &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second}
	go s.server.Serve(ln)
	return nil
}
func (s *Service) Close() {
	s.mu.Lock()
	s.bindings = map[string]binding{}
	s.mu.Unlock()
	if s.server != nil {
		_ = s.server.Close()
	}
}
func (s *Service) Bind(ctx context.Context, session protocol.Session) ([]string, func(), error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, nil, err
	}
	token := hex.EncodeToString(b[:])
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	if err = s.listen(); err != nil {
		s.mu.Unlock()
		return nil, nil, err
	}
	s.bindings[token] = binding{session, ctx}
	s.mu.Unlock()
	cleanup := func() { s.mu.Lock(); delete(s.bindings, token); s.mu.Unlock() }
	return []string{"COMMITDECK_TOOL_ENDPOINT=" + s.url, "COMMITDECK_TOOL_TOKEN=" + token, "COMMITDECK_TOOL_BIN=" + exe}, cleanup, nil
}

type Request struct {
	Operation         string `json:"operation"`
	ID                string `json:"id"`
	Title             string `json:"title"`
	Content           string `json:"content"`
	Completed         bool   `json:"completed"`
	ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
}

var (
	ErrDenied    = errors.New("workspace edit denied")
	ErrConflict  = errors.New("workspace content changed; refresh before retrying")
	ErrOperation = errors.New("unsupported workspace tool")
)

// A 4 MiB note may expand sixfold in JSON (for example, escaped '<' bytes).
const maxToolJSONBytes = 6*(4<<20) + (64 << 10)

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/tool" || r.Header.Get("Origin") != "" || r.Host != strings.TrimPrefix(s.url, "http://") {
		http.Error(w, "forbidden", 403)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		http.Error(w, "missing tool capability", 401)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	b, ok := s.bindings[token]
	s.mu.Unlock()
	if !ok || b.ctx.Err() != nil {
		http.Error(w, "expired tool capability", 401)
		return
	}
	var p Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxToolJSONBytes))
	err := decoder.Decode(&p)
	if err == nil {
		var trailing any
		if err = decoder.Decode(&trailing); err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("multiple request values")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "workspace tool request exceeds its size limit", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(b.ctx, cancel)
	defer stop()
	v, err := s.Call(ctx, b.session, p)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status, message := http.StatusUnprocessableEntity, "workspace operation failed; refresh and check the input before retrying"
		switch {
		case errors.Is(err, ErrDenied):
			status, message = http.StatusForbidden, ErrDenied.Error()
		case errors.Is(err, ErrConflict):
			status, message = http.StatusConflict, ErrConflict.Error()
		case errors.Is(err, ErrOperation):
			status, message = http.StatusBadRequest, ErrOperation.Error()
		case ctx.Err() != nil:
			status, message = http.StatusRequestTimeout, "workspace operation cancelled"
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": v})
}
func (s *Service) approve(ctx context.Context, session protocol.Session, p *Request) error {
	w, err := s.DB.Workspace(session.WorkspaceID)
	if err != nil {
		return err
	}
	ch := make(chan bool, 1)
	title := "Replace workspace notes"
	description := "Requested replacement:\n" + p.Content
	if p.Operation == "todo.delete" {
		title = "Delete TODO"
		todos, e := s.DB.Todos(w.ID)
		if e != nil {
			return e
		}
		found := false
		for _, todo := range todos {
			if todo.ID == p.ID {
				p.ExpectedUpdatedAt = todo.UpdatedAt
				description = "Delete this workspace task:\n" + todo.Title + "\nID: " + todo.ID
				found = true
				break
			}
		}
		if !found {
			return errors.New("task does not belong to this workspace")
		}
	}
	id, err := s.Permissions.Request(permissions.Request{WorkspaceID: session.WorkspaceID, SessionID: session.ID, Provider: session.Provider, Category: "unknown", Title: title, Description: redact.Text(description), Cwd: w.WorktreePath, HighRisk: true}, func(allow bool) error { ch <- allow; return nil })
	if err != nil {
		return err
	}
	select {
	case allow := <-ch:
		if !allow {
			return ErrDenied
		}
		return nil
	case <-ctx.Done():
		_ = s.DB.FinishPermission(id, "expired")
		return errors.New("workspace edit cancelled")
	}
}
func (s *Service) Call(ctx context.Context, session protocol.Session, p Request) (any, error) {
	w, err := s.DB.Workspace(session.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if w.Archived {
		return nil, errors.New("workspace is archived")
	}
	if p.Operation == "todo.delete" || p.Operation == "notes.update" {
		if err = s.approve(ctx, session, &p); err != nil {
			return nil, err
		}
	}
	s.edit.Lock()
	defer s.edit.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, errors.New("workspace tool cancelled")
	}
	var result any
	target := ""
	title := ""
	switch p.Operation {
	case "workspace.get":
		return map[string]any{"key": redact.Text(w.ExternalKey), "title": redact.Text(w.Title), "status": redact.Text(w.Status), "lifecycle": w.Lifecycle, "branch": redact.Text(w.Branch), "worktree": redact.Text(w.WorktreePath)}, nil
	case "todo.list":
		rows, err := s.DB.Todos(w.ID)
		for i := range rows {
			rows[i].Title = redact.Text(rows[i].Title)
		}
		return rows, err
	case "notes.get":
		n, e := s.DB.Note(w.ID)
		n.Content = redact.Text(n.Content)
		return n, e
	case "todo.add", "todo.update", "todo.complete", "todo.delete":
		todos, e := s.DB.Todos(w.ID)
		if e != nil {
			return nil, e
		}
		t := workspace.Todo{WorkspaceID: w.ID, Title: p.Title}
		if p.Operation != "todo.add" {
			found := false
			for _, v := range todos {
				if v.ID == p.ID {
					t = v
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("task does not belong to this workspace")
			}
		}
		if p.Operation == "todo.delete" {
			deleted, e := s.DB.Exec("DELETE FROM todos WHERE workspace_id=? AND id=? AND updated_at=?", w.ID, p.ID, p.ExpectedUpdatedAt)
			err = e
			if err == nil {
				count, e := deleted.RowsAffected()
				err = e
				if err == nil && count != 1 {
					err = ErrConflict
				}
			}
			target = p.ID
			title = "deleted TODO: " + t.Title
		} else {
			if p.Operation == "todo.update" {
				t.Title = p.Title
			}
			if p.Operation == "todo.complete" {
				t.Completed = p.Completed
			}
			if p.Operation != "todo.complete" {
				t.Title = redact.Text(t.Title)
			}
			t.UpdatedBy = session.Provider
			t.SessionID = session.ID
			t, err = s.DB.SaveTodo(t)
			safe := t
			safe.Title = redact.Text(safe.Title)
			result = safe
			target = t.ID
			title = strings.TrimPrefix(p.Operation, "todo.") + " TODO: " + t.Title
		}
	case "notes.append", "notes.update":
		n, e := s.DB.Note(w.ID)
		if e != nil {
			return nil, e
		}
		content := redact.Text(p.Content)
		if strings.TrimSpace(content) == "" {
			return nil, errors.New("note content is empty")
		}
		if p.Operation == "notes.append" {
			content = strings.TrimRight(n.Content, "\n") + "\n\n" + content
		} else if p.ExpectedUpdatedAt == "" || p.ExpectedUpdatedAt != n.UpdatedAt {
			return nil, ErrConflict
		}
		if len(content) > 4<<20 {
			return nil, errors.New("note exceeds 4 MB")
		}
		updated, updateErr := s.DB.Exec("UPDATE notes SET content=?,updated_at=?,updated_by=?,session_id=? WHERE workspace_id=? AND content=?", content, identity.After(n.UpdatedAt), session.Provider, session.ID, w.ID, n.Content)
		err = updateErr
		if err == nil {
			count, _ := updated.RowsAffected()
			if count == 0 {
				err = ErrConflict
			}
		}
		target = n.ID
		title = "appended workspace notes"
		if p.Operation == "notes.update" {
			title = "replaced workspace notes"
		}
		result = map[string]bool{"updated": err == nil}
	case "activity.add":
		if strings.TrimSpace(p.Content) == "" || len(p.Content) > 1000 {
			return nil, errors.New("activity must contain 1–1000 characters")
		}
		title = p.Content
	default:
		return nil, ErrOperation
	}
	if err != nil {
		return nil, err
	}
	_, err = s.DB.Exec("INSERT INTO workspace_tool_events VALUES(?,?,?,?,?,?)", identity.ID(), w.ID, session.ID, p.Operation, target, identity.Now())
	if err != nil {
		return nil, err
	}
	_ = s.Activity.Record(w.ID, "workspace_tool", session.Provider+" "+title, "", false)
	return result, nil
}

// RunHelper is the CLI client, not a second database process.
func RunHelper(ctx context.Context, operation, body string) error {
	endpoint := os.Getenv("COMMITDECK_TOOL_ENDPOINT")
	token := os.Getenv("COMMITDECK_TOOL_TOKEN")
	parsed, parseErr := url.Parse(endpoint)
	validEndpoint := parseErr == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
	if validEndpoint {
		port, e := strconv.Atoi(parsed.Port())
		validEndpoint = e == nil && port > 0 && port <= 65535
	}
	if !validEndpoint || token == "" {
		return errors.New("workspace tools are available only inside a CommitDeck agent turn")
	}
	var p Request
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		return errors.New("invalid tool JSON")
	}
	p.Operation = operation
	b, _ := json.Marshal(p)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/tool", strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("workspace tool connection unavailable")
	}
	defer res.Body.Close()
	b, err = io.ReadAll(io.LimitReader(res.Body, maxToolJSONBytes))
	if err != nil {
		return errors.New("workspace tool response unavailable")
	}
	var response struct {
		Error  string          `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	decodeErr := json.Unmarshal(b, &response)
	if res.StatusCode != http.StatusOK || response.Error != "" {
		return errors.New("workspace tool failed or capability expired; check the operation in CommitDeck")
	}
	if decodeErr != nil || len(response.Result) == 0 {
		return errors.New("invalid workspace tool response")
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}
