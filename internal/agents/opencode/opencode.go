package opencode

import (
	"commitdeck/internal/agents/protocol"
	"context"
	"os/exec"
)

type Provider struct{}

func (Provider) ID() string                       { return "opencode" }
func (Provider) Name() string                     { return "OpenCode" }
func (Provider) IsAvailable(context.Context) bool { return protocol.Available("opencode") }
func (Provider) Command(ctx context.Context, cwd, id, prompt string) *exec.Cmd {
	return bridgeCommand(ctx, cwd)
}
func (Provider) Normalize(b []byte) ([]protocol.Event, string) {
	m := protocol.Object(b)
	s := protocol.Str
	id := s(m, "sessionID")
	part := protocol.Map(m["part"])
	var events []protocol.Event
	switch s(m, "type") {
	case "text":
		events = append(events, protocol.Event{Type: "text", Text: s(part, "text")})
	case "tool_use":
		state := protocol.Map(part["state"])
		e := protocol.Tool(s(part, "tool"), protocol.Map(state["input"]))
		e.Text = s(state, "title")
		events = append(events, e)
		if s(state, "status") == "error" {
			events = append(events, protocol.Event{Type: "error", Error: s(state, "error")})
		}
	case "error":
		e := protocol.Map(m["error"])
		msg := s(protocol.Map(e["data"]), "message")
		if msg == "" {
			msg = s(e, "message")
		}
		events = append(events, protocol.Event{Type: "error", Error: msg})
	case "permission_request", "permission.asked":
		events = append(events, protocol.Event{Type: "permission_request", Text: "Permission required. Resume OpenCode in the worktree terminal."})
	}
	return events, id
}
