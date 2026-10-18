package agents

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/workspace"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRuntime struct{}

func (fakeRuntime) ID() string                       { return "fake" }
func (fakeRuntime) Name() string                     { return "Fake" }
func (fakeRuntime) IsAvailable(context.Context) bool { return true }
func (fakeRuntime) Command(ctx context.Context, cwd, id, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestAgentHelper")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "COMMITDECK_TEST_AGENT="+prompt)
	return cmd
}

func TestAgentEventsRedactedBeforeStreaming(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var emitted protocol.Event
	m := New(db, func(v any) {
		if event, ok := v.(map[string]any)["event"].(protocol.Event); ok {
			emitted = event
		}
	})
	defer m.Close()
	w, err := db.CreateWorkspace(workspace.Workspace{Title: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.event(s.ID, protocol.Event{Type: "text", Text: "api_key=synthetic-stream-value"}); err != nil {
		t.Fatal(err)
	}
	if emitted.Text != "[REDACTED]" {
		t.Fatal("credential-shaped output streamed")
	}
	events, err := db.Events(s.ID)
	if err != nil || len(events) != 1 || strings.Contains(events[0].Text, "synthetic-stream-value") {
		t.Fatal(events, err)
	}
}
func (fakeRuntime) Normalize(b []byte) ([]protocol.Event, string) {
	m := protocol.Object(b)
	if protocol.Str(m, "type") == "text" {
		return []protocol.Event{{Type: "text", Text: protocol.Str(m, "text")}}, "external-1"
	}
	return nil, ""
}
func TestAgentHelper(t *testing.T) {
	mode := os.Getenv("COMMITDECK_TEST_AGENT")
	if mode == "" {
		return
	}
	cwd, _ := os.Getwd()
	fmt.Printf("{\"type\":\"text\",\"text\":%q}\n", cwd)
	if mode == "crash" {
		os.Exit(3)
	}
	if mode == "wait" {
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}
func TestConcurrentSessionsAndCancellation(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := New(db, func(any) {})
	m.providers["fake"] = fakeRuntime{}
	defer m.Close()
	var sessions []protocol.Session
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		if b, e := exec.Command("git", "init", dir).CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
		w, e := db.CreateWorkspace(workspace.Workspace{Title: "Ticket", WorktreePath: dir})
		if e != nil {
			t.Fatal(e)
		}
		s, e := m.StartSession(context.Background(), w, SessionOptions{Provider: "fake"})
		if e != nil {
			t.Fatal(e)
		}
		sessions = append(sessions, s)
	}
	for _, s := range sessions {
		if err = m.Send(context.Background(), s.ID, "wait"); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range sessions {
		got, _ := db.Session(s.ID)
		if got.Status != "running" || got.PID == 0 {
			t.Fatal(got)
		}
		if !m.ActiveWorkspace(s.WorkspaceID) {
			t.Fatal("not tracked")
		}
	}
	if err = m.Send(context.Background(), sessions[0].ID, "again"); err == nil {
		t.Fatal("double send accepted")
	}
	if err = m.Cancel(context.Background(), sessions[0].ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, db, sessions[0].ID, "cancelled")
	other, _ := db.Session(sessions[1].ID)
	if other.Status != "running" {
		t.Fatal("cancelled unrelated session")
	}
	if err = m.Send(context.Background(), sessions[0].ID, "crash"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, db, sessions[0].ID, "failed")
	events, err := db.Events(sessions[0].ID)
	if err != nil || len(events) < 3 {
		t.Fatal(events, err)
	}
	s, _ := db.Session(sessions[0].ID)
	w, _ := db.Workspace(s.WorkspaceID)
	canonical, _ := filepath.EvalSymlinks(w.WorktreePath)
	found := false
	for _, e := range events {
		if e.Text == canonical {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent cwd mismatch: want %s, events %+v", canonical, events)
	}
	if s.ExternalSessionID != "external-1" {
		t.Fatal("resume ID not persisted")
	}
	m.Close()
	other, _ = db.Session(sessions[1].ID)
	if other.Status != "cancelled" || other.PID != 0 {
		t.Fatal("shutdown did not clean up", other)
	}
}
func waitStatus(t *testing.T, db *database.DB, id, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, err := db.Session(id)
		if err != nil {
			t.Fatal(err)
		}
		if s.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s, _ := db.Session(id)
	t.Fatalf("want %s, got %+v", want, s)
}
