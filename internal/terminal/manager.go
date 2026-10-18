package terminal

import (
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/identity"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

type Tab struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Title       string `json:"title"`
	CWD         string `json:"cwd"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	LastUsedAt  string `json:"lastUsedAt"`
}
type process interface {
	io.ReadWriteCloser
	Resize(int, int) error
	Wait() error
}
type live struct {
	process
	buffer    []byte
	sequence  uint64
	done      chan struct{}
	lastTouch time.Time
}
type Manager struct {
	DB     *database.DB
	Emit   func(any)
	mu     sync.Mutex
	tabs   map[string]*live
	closed bool
}

func New(db *database.DB, emit func(any)) (*Manager, error) {
	_, err := db.Exec("UPDATE terminals SET status='stopped'")
	return &Manager{DB: db, Emit: emit, tabs: map[string]*live{}}, err
}
func (m *Manager) List(workspaceID string) ([]Tab, error) {
	rows, err := m.DB.Query("SELECT id,workspace_id,title,cwd,status,created_at,last_used_at FROM terminals WHERE workspace_id=? ORDER BY created_at", workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tab{}
	for rows.Next() {
		var t Tab
		if err = rows.Scan(&t.ID, &t.WorkspaceID, &t.Title, &t.CWD, &t.Status, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (m *Manager) Create(workspaceID string) (Tab, error) {
	w, err := m.DB.Workspace(workspaceID)
	if err != nil {
		return Tab{}, err
	}
	if w.Archived || w.WorktreePath == "" {
		return Tab{}, errors.New("create a worktree in an active workspace first")
	}
	cwd, err := gitservice.Canonical(w.WorktreePath)
	if err != nil {
		return Tab{}, err
	}
	t := Tab{ID: identity.ID(), WorkspaceID: workspaceID, Title: "shell", CWD: cwd, Status: "stopped", CreatedAt: identity.Now(), LastUsedAt: identity.Now()}
	_, err = m.DB.Exec("INSERT INTO terminals VALUES(?,?,?,?,?,?,?)", t.ID, t.WorkspaceID, t.Title, t.CWD, t.Status, t.CreatedAt, t.LastUsedAt)
	return t, err
}
func dimensions(cols, rows int) error {
	if cols < 2 || cols > 500 || rows < 1 || rows > 300 {
		return errors.New("invalid terminal size")
	}
	return nil
}
func (m *Manager) Start(id string, cols, rows int) error {
	if err := dimensions(cols, rows); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("terminal manager closed")
	}
	if m.tabs[id] != nil {
		return nil
	}
	var workspaceID, cwd string
	if err := m.DB.QueryRow("SELECT workspace_id,cwd FROM terminals WHERE id=?", id).Scan(&workspaceID, &cwd); err != nil {
		return err
	}
	w, err := m.DB.Workspace(workspaceID)
	if err != nil {
		return err
	}
	currentCwd, canonicalErr := gitservice.Canonical(w.WorktreePath)
	if w.Archived || canonicalErr != nil || cwd != currentCwd {
		return errors.New("terminal worktree changed; create a new terminal")
	}
	if err = (gitservice.CLI{}).ValidateRepo(cwd); err != nil {
		return err
	}
	p, err := start(cwd, cols, rows)
	if err != nil {
		return err
	}
	l := &live{process: p, done: make(chan struct{}), lastTouch: time.Now()}
	m.tabs[id] = l
	if _, err = m.DB.Exec("UPDATE terminals SET status='running',last_used_at=? WHERE id=?", identity.Now(), id); err != nil {
		p.Close()
		delete(m.tabs, id)
		return err
	}
	go m.read(id, workspaceID, l)
	m.changed(workspaceID)
	return nil
}
func (m *Manager) read(id, workspaceID string, l *live) {
	defer close(l.done)
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		buf := make([]byte, 8192)
		for {
			n, err := l.Read(buf)
			if n > 0 {
				m.mu.Lock()
				l.sequence++
				l.buffer = append(l.buffer, buf[:n]...)
				if len(l.buffer) > 256*1024 {
					l.buffer = append([]byte(nil), l.buffer[len(l.buffer)-256*1024:]...)
				}
				seq := l.sequence
				m.mu.Unlock()
				if m.Emit != nil {
					m.Emit(map[string]any{"type": "terminal_output", "terminalId": id, "sequence": seq, "data": base64.StdEncoding.EncodeToString(buf[:n])})
				}
			}
			if err != nil {
				return
			}
		}
	}()
	_ = l.Wait()
	_ = l.Close()
	readers.Wait()
	m.mu.Lock()
	if m.tabs[id] == l {
		delete(m.tabs, id)
	}
	m.mu.Unlock()
	_, _ = m.DB.Exec("UPDATE terminals SET status='stopped' WHERE id=?", id)
	m.changed(workspaceID)
}
func (m *Manager) Snapshot(id string) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.tabs[id]
	if l == nil {
		return map[string]any{"sequence": 0, "data": ""}
	}
	return map[string]any{"sequence": l.sequence, "data": base64.StdEncoding.EncodeToString(l.buffer)}
}
func (m *Manager) Write(id, data string) error {
	if len(data) > 65536 {
		return errors.New("terminal input too large")
	}
	m.mu.Lock()
	l := m.tabs[id]
	touch := l != nil && time.Since(l.lastTouch) > 30*time.Second
	if touch {
		l.lastTouch = time.Now()
	}
	m.mu.Unlock()
	if l == nil {
		return errors.New("terminal is stopped")
	}
	_, err := io.WriteString(l, data)
	if err == nil && touch {
		_, _ = m.DB.Exec("UPDATE terminals SET last_used_at=? WHERE id=?", identity.Now(), id)
	}
	return err
}
func (m *Manager) Resize(id string, cols, rows int) error {
	if err := dimensions(cols, rows); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if l := m.tabs[id]; l != nil {
		return l.Resize(cols, rows)
	}
	return nil
}
func (m *Manager) Rename(id, title string) error {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 80 {
		return errors.New("terminal name must contain 1–80 characters")
	}
	_, err := m.DB.Exec("UPDATE terminals SET title=? WHERE id=?", title, id)
	return err
}
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	l := m.tabs[id]
	m.mu.Unlock()
	if l != nil {
		_ = l.Close()
		<-l.done
	}
}
func (m *Manager) Delete(id string) error {
	m.Stop(id)
	_, err := m.DB.Exec("DELETE FROM terminals WHERE id=?", id)
	return err
}
func (m *Manager) Active(workspaceID string) bool {
	var n int
	_ = m.DB.QueryRow("SELECT COUNT(*) FROM terminals WHERE workspace_id=? AND status='running'", workspaceID).Scan(&n)
	return n > 0
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	ids := []string{}
	for id := range m.tabs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}
func (m *Manager) changed(id string) {
	if m.Emit != nil {
		m.Emit(map[string]any{"type": "terminals_changed", "workspaceId": id})
	}
}
