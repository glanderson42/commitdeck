package codex

import (
	"commitdeck/internal/agents/protocol"
	"context"
	"os/exec"
)

type Provider struct{}

func (Provider) ID() string                       { return "codex" }
func (Provider) Name() string                     { return "Codex" }
func (Provider) IsAvailable(context.Context) bool { return protocol.Available("codex") }
func (Provider) Command(ctx context.Context, cwd, id, prompt string) *exec.Cmd {
	args := []string{"app-server", "--listen", "stdio://"}
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = cwd
	return cmd
}
func (Provider) Normalize(b []byte) ([]protocol.Event, string) {
	m := protocol.Object(b)
	s := protocol.Str
	var events []protocol.Event
	id := s(m, "thread_id")
	kind := s(m, "type")
	item := protocol.Map(m["item"])
	switch kind {
	case "error", "turn.failed":
		msg := s(m, "message")
		if msg == "" {
			msg = s(protocol.Map(m["error"]), "message")
		}
		events = append(events, protocol.Event{Type: "error", Error: msg})
	case "turn.started":
		events = append(events, protocol.Event{Type: "status", Text: "Working"})
	case "item.started", "item.completed":
		switch s(item, "type") {
		case "agent_message", "reasoning":
			if kind == "item.completed" {
				events = append(events, protocol.Event{Type: "text", Text: s(item, "text")})
			}
		case "command_execution":
			e := protocol.Event{Type: "command", Command: s(item, "command"), Text: s(item, "aggregated_output")}
			events = append(events, e)
		case "file_change":
			if kind == "item.completed" {
				for _, v := range protocol.List(item["changes"]) {
					f := protocol.Map(v)
					events = append(events, protocol.Event{Type: "file_changed", FilePath: s(f, "path"), Text: s(f, "kind")})
				}
			}
		case "mcp_tool_call":
			events = append(events, protocol.Event{Type: "tool", ToolName: s(item, "tool"), Text: s(item, "status")})
		}
	case "permission_request":
		events = append(events, protocol.Event{Type: "permission_request", Text: "Permission required. Continue in the provider CLI."})
	}
	return events, id
}
