package agents

import (
	"commitdeck/internal/agents/claude"
	"commitdeck/internal/agents/codex"
	"commitdeck/internal/agents/opencode"
	"commitdeck/internal/agents/protocol"
	"context"
	"strings"
	"testing"
)

func TestProviderNormalization(t *testing.T) {
	cases := []struct {
		provider protocol.Runtime
		line     string
		kind     string
		external string
	}{
		{codex.Provider{}, `{"type":"thread.started","thread_id":"thread-1"}`, "", "thread-1"},
		{codex.Provider{}, `{"type":"item.completed","item":{"type":"agent_message","text":"Implemented"}}`, "text", ""},
		{codex.Provider{}, `{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","aggregated_output":"ok"}}`, "command", ""},
		{claude.Provider{}, `{"type":"assistant","session_id":"session-1","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"main.go"}}]}}`, "file_changed", "session-1"},
		{claude.Provider{}, `{"type":"result","permission_denials":[{"tool_name":"Bash"}]}`, "permission_request", ""},
		{opencode.Provider{}, `{"type":"text","sessionID":"ses_123","part":{"text":"Done"}}`, "text", "ses_123"},
		{opencode.Provider{}, `{"type":"tool_use","part":{"tool":"bash","state":{"input":{"command":"go test"}}}}`, "command", ""},
	}
	for _, c := range cases {
		events, id := c.provider.Normalize([]byte(c.line))
		if id != c.external {
			t.Fatal(id)
		}
		if c.kind != "" && (len(events) == 0 || events[0].Type != c.kind) {
			t.Fatalf("%s: %+v", c.provider.ID(), events)
		}
	}
	for _, p := range []protocol.Runtime{codex.Provider{}, claude.Provider{}, opencode.Provider{}} {
		prompt := "-x; $(touch /tmp/never)"
		cmd := p.Command(context.Background(), "/tmp/worktree", "session-1", prompt)
		if cmd.Dir != "/tmp/worktree" {
			t.Fatal("cwd")
		}
		if strings.Contains(strings.Join(cmd.Args, " "), prompt) {
			t.Fatal("prompt should travel through stdin")
		}
		events, _ := p.Normalize([]byte(`{"unknown":"secret-wire-protocol"}`))
		if len(events) > 0 {
			t.Fatal("unknown protocol displayed")
		}
	}
}
