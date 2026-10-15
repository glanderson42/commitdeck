package agents

import (
	"bufio"
	"commitdeck/internal/agents/claude"
	"commitdeck/internal/agents/codex"
	"commitdeck/internal/agents/opencode"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderApprovalWireProtocols(t *testing.T) {
	c := (codex.Provider{}).NewConversation("thread-1", "/ticket", "Hello")
	if len(c.Initial()) != 1 {
		t.Fatal("Codex initialization")
	}
	f := c.Handle([]byte(`{"id":"cd_init","result":{}}`))
	if len(f.Writes) != 2 {
		t.Fatal("Codex resume handshake")
	}
	f = c.Handle([]byte(`{"id":"cd_thread","result":{"thread":{"id":"thread-1"}}}`))
	if !f.InputSent || f.ExternalID != "thread-1" {
		t.Fatal("Codex resumed thread")
	}
	f = c.Handle([]byte(`{"id":42,"method":"item/commandExecution/requestApproval","params":{"command":"go test ./...","cwd":"/ticket","itemId":"i"}}`))
	if f.Permission == nil || f.Permission.Command != "go test ./..." {
		t.Fatal("missing permission")
	}
	b, _ := json.Marshal(f.Reply(false))
	if string(b) != `{"id":42,"result":{"decision":"decline"}}` {
		t.Fatal(string(b))
	}
	a := (claude.Provider{}).NewConversation("session-1", "/ticket", "Hello")
	_ = a.Initial()
	f = a.Handle([]byte(`{"type":"control_response","response":{"subtype":"success","request_id":"cd_init","response":{}}}`))
	if !f.InputSent {
		t.Fatal("Claude user prompt not written")
	}
	f = a.Handle([]byte(`{"type":"control_request","request_id":"p1","request":{"subtype":"can_use_tool","tool_name":"Write","input":{"file_path":"config.yaml","content":"test"}}}`))
	if f.Permission == nil || f.Permission.Category != "file_write" {
		t.Fatal(f)
	}
	b, _ = json.Marshal(f.Reply(true))
	var wire map[string]any
	_ = json.Unmarshal(b, &wire)
	response := protocol.Map(protocol.Map(wire["response"])["response"])
	if protocol.Str(response, "behavior") != "allow" || protocol.Str(protocol.Map(response["updatedInput"]), "file_path") != "config.yaml" {
		t.Fatal(string(b))
	}
	o := (opencode.Provider{}).NewConversation("ses-1", "/ticket", "Hello")
	f = o.Handle([]byte(`{"type":"permission","wireId":"p1","request":{"category":"shell_command","command":"go test ./..."}}`))
	if f.Permission == nil || !f.RequiresAck {
		t.Fatal("OpenCode decisions must wait for HTTP acknowledgement")
	}
	f = o.Handle([]byte(`{"type":"decision_ack","wireId":"p1","error":"expired"}`))
	if f.AckError != "expired" {
		t.Fatal("lost provider rejection")
	}
}

type approvalRuntime struct{ fakeRuntime }

func (approvalRuntime) Command(ctx context.Context, cwd, id, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestPermissionProcessHelper")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "COMMITDECK_PERMISSION_HELPER=1")
	return cmd
}
func (approvalRuntime) NewConversation(id, cwd, prompt string) protocol.Conversation {
	return &approvalConversation{cwd: cwd, prompt: prompt}
}

type approvalConversation struct{ cwd, prompt string }

func (c *approvalConversation) Initial() []any { return []any{map[string]any{"prompt": c.prompt}} }
func (c *approvalConversation) Handle(b []byte) protocol.Frame {
	m := protocol.Object(b)
	switch protocol.Str(m, "type") {
	case "permission":
		return protocol.Frame{Permission: &permissions.Request{Category: "shell_command", Command: "go test ./...", Cwd: c.cwd}, Reply: func(yes bool) any { return map[string]any{"allow": yes} }}
	case "done":
		return protocol.Frame{Done: true, ExternalID: "native-thread-1", Events: []protocol.Event{{Type: "text", Text: protocol.Str(m, "text")}}}
	}
	return protocol.Frame{}
}
func TestPermissionProcessHelper(t *testing.T) {
	if os.Getenv("COMMITDECK_PERMISSION_HELPER") == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if !containsJSONPrompt(scanner.Bytes()) {
		os.Exit(3)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "permission"})
	if !scanner.Scan() {
		os.Exit(4)
	}
	m := protocol.Object(scanner.Bytes())
	allow, _ := m["allow"].(bool)
	text := "denied"
	if allow {
		text = "approved"
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "done", "text": text})
	os.Exit(0)
}
func containsJSONPrompt(b []byte) bool { return protocol.Str(protocol.Object(b), "prompt") != "" }
func TestLivePermissionResponseUnblocksOnlyOwnedProcess(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := New(db, func(any) {})
	defer m.Close()
	m.providers["fake"] = approvalRuntime{}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	w, err := db.CreateWorkspace(workspace.Workspace{Title: "Ticket", WorktreePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	session, err := m.StartSession(context.Background(), w, SessionOptions{Provider: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Send(context.Background(), session.ID, "Run tests"); err != nil {
		t.Fatal(err)
	}
	var request permissions.Request
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		list, _ := db.Permissions(w.ID, true)
		if len(list) > 0 {
			request = list[0]
			break
		}
		time.Sleep(time.Millisecond)
	}
	if request.ID == "" || !request.CanRespond {
		t.Fatal("no live request")
	}
	waiting, _ := db.Session(session.ID)
	if waiting.Status != "waiting_permission" {
		t.Fatal(waiting)
	}
	if err = m.Permissions.Resolve(request.ID, permissions.Decision{Allow: true, Scope: "workspace"}); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, db, session.ID, "completed")
	saved, _ := db.Permission(request.ID)
	if saved.Status != "approved" {
		t.Fatal(saved)
	}
	events, _ := db.Events(session.ID)
	found := false
	for _, e := range events {
		if e.Text == "approved" {
			found = true
		}
	}
	if !found {
		t.Fatal("response never reached subprocess")
	}
}
