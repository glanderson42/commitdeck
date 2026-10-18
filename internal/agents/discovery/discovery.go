// Package discovery isolates provider-owned metadata formats from CommitDeck.
package discovery

import (
	"bufio"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/identity"
	"commitdeck/internal/redact"
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Session struct {
	Provider             string `json:"provider"`
	ExternalSessionID    string `json:"externalSessionId"`
	Title                string `json:"title"`
	CWD                  string `json:"cwd"`
	RepoPath             string `json:"repoPath"`
	Branch               string `json:"branch"`
	UpdatedAt            string `json:"updatedAt"`
	ResumeSupported      bool   `json:"resumeSupported"`
	Source               string `json:"source"`
	SuggestedWorkspaceID string `json:"suggestedWorkspaceId"`
	Confidence           string `json:"confidence"`
	Imported             bool   `json:"imported"`
}
type Result struct {
	Sessions []Session `json:"sessions"`
	Warnings []string  `json:"warnings"`
}
type Service struct {
	DB    *database.DB
	mu    sync.Mutex
	found map[string]Session
}

func New(db *database.DB) *Service { return &Service{DB: db, found: map[string]Session{}} }
func key(s Session) string         { return s.Provider + ":" + s.ExternalSessionID }

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,200}$`)

func (s *Service) Discover(ctx context.Context) Result {
	out := Result{Sessions: []Session{}, Warnings: []string{}}
	ws, _ := s.DB.Workspaces()
	paths := pathCache{}
	candidates := make([]candidate, 0, len(ws))
	for _, w := range ws {
		if !w.Archived {
			candidates = append(candidates, prepareCandidate(w, paths))
		}
	}
	type result struct {
		rows []Session
		err  error
		name string
	}
	ch := make(chan result, 3)
	for name, fn := range map[string]func(context.Context) ([]Session, error){"codex": Codex, "claude": Claude, "opencode": OpenCode} {
		go func() {
			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			r, e := fn(ctx)
			ch <- result{r, e, name}
		}()
	}
	for range 3 {
		r := <-ch
		if r.err != nil {
			out.Warnings = append(out.Warnings, r.name+": "+r.err.Error())
		}
		for _, v := range r.rows {
			if !validID.MatchString(v.ExternalSessionID) {
				continue
			}
			v.Title = redact.Text(v.Title)
			v.CWD = redact.Text(v.CWD)
			v.RepoPath = redact.Text(v.RepoPath)
			v.Branch = redact.Text(v.Branch)
			if len(v.Title) > 200 {
				v.Title = string([]rune(v.Title)[:min(160, len([]rune(v.Title)))])
			}
			if v.Title == "" {
				v.Title = "Imported session"
			}
			best := 0
			match := v
			match.CWD, match.RepoPath = paths.resolve(v.CWD), paths.resolve(v.RepoPath)
			for _, c := range candidates {
				score := c.score(match)
				if score > best {
					best = score
					v.SuggestedWorkspaceID = c.workspace.ID
					v.Confidence = "Low"
					if score >= 80 {
						v.Confidence = "High"
					} else if score >= 40 {
						v.Confidence = "Medium"
					}
				}
			}
			var count int
			_ = s.DB.QueryRow("SELECT COUNT(*) FROM agent_sessions WHERE provider=? AND external_session_id=?", v.Provider, v.ExternalSessionID).Scan(&count)
			v.Imported = count > 0
			out.Sessions = append(out.Sessions, v)
		}
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].UpdatedAt > out.Sessions[j].UpdatedAt })
	sort.Strings(out.Warnings)
	s.mu.Lock()
	s.found = map[string]Session{}
	for _, v := range out.Sessions {
		s.found[key(v)] = v
	}
	s.mu.Unlock()
	return out
}

// Paths and matchers are prepared once per discovery, never per session/workspace pair.
type pathCache map[string]string

func (c pathCache) resolve(path string) string {
	if path == "" {
		return ""
	}
	if resolved, ok := c[path]; ok {
		return resolved
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = ""
	}
	c[path] = resolved
	return resolved
}

type candidate struct {
	workspace workspace.Workspace
	ticketKey *regexp.Regexp
}

func prepareCandidate(w workspace.Workspace, paths pathCache) candidate {
	w.WorktreePath, w.RepoPath = paths.resolve(w.WorktreePath), paths.resolve(w.RepoPath)
	c := candidate{workspace: w}
	if w.ExternalKey != "" {
		c.ticketKey = regexp.MustCompile(`(^|[^a-z0-9])` + regexp.QuoteMeta(strings.ToLower(w.ExternalKey)) + `([^a-z0-9]|$)`)
	}
	return c
}

func withinPath(child, parent string) bool {
	if child == "" || parent == "" {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func Score(s Session, w workspace.Workspace) int {
	paths := pathCache{}
	c := prepareCandidate(w, paths)
	s.CWD, s.RepoPath = paths.resolve(s.CWD), paths.resolve(s.RepoPath)
	return c.score(s)
}

func (c candidate) score(s Session) int {
	w := c.workspace
	score := 0
	if s.CWD != "" && s.CWD == w.WorktreePath {
		score += 100
	} else if withinPath(s.CWD, w.WorktreePath) {
		score += 90
	}
	if withinPath(s.CWD, w.RepoPath) || (s.RepoPath != "" && s.RepoPath == w.RepoPath) {
		score += 40
	}
	if s.Branch != "" && s.Branch == w.Branch {
		score += 40
	}
	if c.ticketKey != nil && c.ticketKey.MatchString(strings.ToLower(s.Title+" "+s.Branch)) {
		score += 20
	}
	return score
}

func (s *Service) Import(provider, id, workspaceID string) (protocol.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.found[provider+":"+id]
	if !ok {
		return protocol.Session{}, errors.New("discover sessions again before importing")
	}
	w, err := s.DB.Workspace(workspaceID)
	if err != nil {
		return protocol.Session{}, err
	}
	if w.Archived {
		return protocol.Session{}, errors.New("restore workspace first")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return protocol.Session{}, err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRow("SELECT id FROM agent_sessions WHERE provider=? AND external_session_id=?", provider, id).Scan(&existing)
	if err == nil {
		return protocol.Session{}, errors.New("session is already attached to a workspace")
	}
	result := protocol.Session{ID: identity.ID(), WorkspaceID: workspaceID, Provider: provider, ExternalSessionID: id, Title: d.Title, Purpose: "custom", Status: "idle", Imported: true, ResumeSupported: d.ResumeSupported, SourceCWD: d.CWD, CreatedAt: identity.Now(), UpdatedAt: identity.Now()}
	_, err = tx.Exec("INSERT INTO agent_sessions(id,workspace_id,provider,external_session_id,title,purpose,status,created_at,updated_at,imported,resume_supported,source_cwd) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", result.ID, workspaceID, provider, id, d.Title, result.Purpose, result.Status, result.CreatedAt, result.UpdatedAt, true, d.ResumeSupported, d.CWD)
	if err != nil {
		return result, err
	}
	_, err = tx.Exec("INSERT INTO agent_imports VALUES(?,?,?,?,?)", provider, id, result.ID, d.Source, result.CreatedAt)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
func Codex(ctx context.Context) ([]Session, error) {
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--listen", "stdio://")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, errors.New("CLI not installed")
	}
	defer func() {
		in.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	enc := json.NewEncoder(in)
	if err = enc.Encode(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "commitdeck", "version": "0.1.0-beta.1"}}}); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	rows := []Session{}
	page := 0
	fallback := false
	for scanner.Scan() {
		var m struct {
			ID     int             `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Data []struct {
					ID, Name, Preview, Cwd string
					UpdatedAt              int64 `json:"updatedAt"`
					GitInfo                struct {
						Branch string `json:"branch"`
					} `json:"gitInfo"`
				} `json:"data"`
				NextCursor string `json:"nextCursor"`
			} `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			continue
		}
		if len(m.Error) > 0 {
			if m.ID == 2 && page == 0 && !fallback {
				fallback = true
				_ = enc.Encode(map[string]any{"id": 2, "method": "thread/list", "params": map[string]any{"limit": 100}})
				continue
			}
			return rows, errors.New("session listing unsupported by this Codex version")
		}
		if m.ID == 1 {
			_ = enc.Encode(map[string]any{"method": "initialized"})
			_ = enc.Encode(map[string]any{"id": 2, "method": "thread/list", "params": map[string]any{"limit": 100, "sortKey": "updated_at", "sourceKinds": []string{"cli", "vscode", "appServer"}}})
		} else if m.ID == 2 {
			for _, v := range m.Result.Data {
				title := v.Name
				if title == "" {
					title = v.Preview
				}
				rows = append(rows, Session{Provider: "codex", ExternalSessionID: v.ID, Title: title, CWD: v.Cwd, Branch: v.GitInfo.Branch, UpdatedAt: time.Unix(v.UpdatedAt, 0).UTC().Format(time.RFC3339Nano), ResumeSupported: true, Source: "Codex thread/list"})
			}
			page++
			if m.Result.NextCursor == "" || page >= 5 {
				return rows, nil
			}
			_ = enc.Encode(map[string]any{"id": 2, "method": "thread/list", "params": map[string]any{"limit": 100, "cursor": m.Result.NextCursor, "sortKey": "updated_at", "sourceKinds": []string{"cli", "vscode", "appServer"}}})
		}
	}
	if ctx.Err() != nil {
		return rows, ctx.Err()
	}
	return rows, errors.New("Codex discovery ended unexpectedly")
}

// Claude metadata has no stable public listing CLI. Parse only allowlisted fields in bounded project transcripts.
func Claude(ctx context.Context) ([]Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		root = filepath.Join(home, ".claude")
	}
	files, err := filepath.Glob(filepath.Join(root, "projects", "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := []Session{}
	for i, path := range files {
		if i >= 2000 || ctx.Err() != nil {
			break
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		s := ParseClaude(io.LimitReader(f, 256<<10), strings.TrimSuffix(filepath.Base(path), ".jsonl"))
		f.Close()
		if s.CWD == "" {
			continue
		}
		s.UpdatedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		out = append(out, s)
	}
	return out, nil
}
func ParseClaude(r io.Reader, id string) Session {
	s := Session{Provider: "claude", ExternalSessionID: id, ResumeSupported: true, Source: "Claude project metadata (JSONL)"}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		var m struct{ Type, SessionID, Cwd, GitBranch, Summary, CustomTitle string }
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			continue
		}
		if m.SessionID != "" && m.SessionID != id {
			continue
		}
		if m.Cwd != "" {
			s.CWD = m.Cwd
		}
		if m.GitBranch != "" {
			s.Branch = m.GitBranch
		}
		if m.Type == "summary" && m.Summary != "" {
			s.Title = m.Summary
		}
		if m.Type == "custom-title" && m.CustomTitle != "" {
			s.Title = m.CustomTitle
		}
	}
	if s.Title == "" {
		s.Title = "Claude · " + filepath.Base(s.CWD)
	}
	return s
}
func OpenCode(ctx context.Context) ([]Session, error) {
	cmd := exec.CommandContext(ctx, "opencode", "session", "list", "--format", "json", "--max-count", "200")
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("CLI session list unavailable")
	}
	if len(out) > 4<<20 {
		return nil, errors.New("session list exceeds metadata limit")
	}
	return ParseOpenCode(out)
}
func ParseOpenCode(b []byte) ([]Session, error) {
	var rows []struct {
		ID, Title, Directory string
		Updated              int64
		Time                 struct{ Updated int64 }
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, errors.New("unrecognized OpenCode session metadata")
	}
	out := []Session{}
	for _, v := range rows {
		if !validID.MatchString(v.ID) {
			return nil, errors.New("unrecognized OpenCode session ID format")
		}
		updated := v.Time.Updated
		if updated == 0 {
			updated = v.Updated
		}
		out = append(out, Session{Provider: "opencode", ExternalSessionID: v.ID, Title: v.Title, CWD: v.Directory, UpdatedAt: time.UnixMilli(updated).UTC().Format(time.RFC3339Nano), ResumeSupported: true, Source: "OpenCode session list"})
	}
	return out, nil
}
