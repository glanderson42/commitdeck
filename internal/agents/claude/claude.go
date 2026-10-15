package claude

import (
	"commitdeck/internal/agents/protocol"
	"context"
	"os/exec"
)

type Provider struct{}

func (Provider) ID() string                       { return "claude" }
func (Provider) Name() string                     { return "Claude Code" }
func (Provider) IsAvailable(context.Context) bool { return protocol.Available("claude") }
func (Provider) Command(ctx context.Context, cwd, id, prompt string) *exec.Cmd {
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--permission-mode", "default", "--permission-prompt-tool", "stdio"}
	if id != "" {
		args = append(args, "--resume="+id)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cwd
	return cmd
}
func (Provider) Normalize(b []byte) ([]protocol.Event, string) {
	m := protocol.Object(b)
	s := protocol.Str
	id := s(m, "session_id")
	var events []protocol.Event
	switch s(m, "type") {
	case "assistant":
		for _, v := range protocol.List(protocol.Map(m["message"])["content"]) {
			c := protocol.Map(v)
			switch s(c, "type") {
			case "text":
				events = append(events, protocol.Event{Type: "text", Text: s(c, "text")})
			case "tool_use":
				events = append(events, protocol.Tool(s(c, "name"), protocol.Map(c["input"])))
			}
		}
	case "result":
		for _, v := range protocol.List(m["permission_denials"]) {
			d := protocol.Map(v)
			events = append(events, protocol.Event{Type: "permission_request", ToolName: s(d, "tool_name"), Text: "Permission denied. Open the worktree terminal and resume Claude to approve this action."})
		}
		if failed, _ := m["is_error"].(bool); failed {
			msg := s(m, "result")
			if msg == "" {
				msg = "Claude could not complete this turn"
			}
			events = append(events, protocol.Event{Type: "error", Error: msg})
		}
	case "error":
		events = append(events, protocol.Event{Type: "error", Error: s(protocol.Map(m["error"]), "message")})
	}
	return events, id
}
