package claude

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/permissions"
)

type conversation struct{ prompt string }

func (Provider) NewConversation(id, cwd, prompt string) protocol.Conversation {
	return &conversation{prompt}
}
func (c *conversation) Initial() []any {
	return []any{map[string]any{"type": "control_request", "request_id": "cd_init", "request": map[string]any{"subtype": "initialize", "hooks": map[string]any{}}}}
}
func (c *conversation) Handle(b []byte) protocol.Frame {
	m := protocol.Object(b)
	s := protocol.Str
	f := protocol.Frame{}
	if s(m, "type") == "control_response" {
		response := protocol.Map(m["response"])
		if s(response, "subtype") == "error" {
			f.Done = true
			f.Failed = true
			f.Events = []protocol.Event{{Type: "error", Error: s(response, "error")}}
			return f
		}
		if s(response, "request_id") == "cd_init" {
			f.InputSent = true
			f.Writes = []any{map[string]any{"type": "user", "session_id": "", "message": map[string]any{"role": "user", "content": c.prompt}, "parent_tool_use_id": nil}}
		}
		return f
	}
	if s(m, "type") == "control_request" {
		req := protocol.Map(m["request"])
		wireID := s(m, "request_id")
		tool := s(req, "tool_name")
		input := protocol.Map(req["input"])
		if s(req, "subtype") != "can_use_tool" || tool == "AskUserQuestion" {
			f.Writes = []any{map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": wireID, "error": "This interaction requires the Claude terminal UI"}}}
			f.Events = []protocol.Event{{Type: "status", Text: "Claude requested an interaction that needs its terminal UI."}}
			return f
		}
		r := &permissions.Request{Category: "unknown", Title: tool, Description: s(req, "description"), Command: s(input, "command"), FilePath: s(input, "file_path")}
		switch tool {
		case "Bash":
			r.Category = "shell_command"
		case "Edit", "Write", "MultiEdit", "NotebookEdit":
			r.Category = "file_write"
		case "WebFetch", "WebSearch":
			r.Category = "network_request"
		}
		f.Permission = r
		f.Reply = func(allow bool) any {
			response := map[string]any{"behavior": "deny", "message": "Denied in CommitDeck"}
			if allow {
				response = map[string]any{"behavior": "allow", "updatedInput": input}
			}
			return map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": wireID, "response": response}}
		}
		return f
	}
	f.Events, f.ExternalID = (Provider{}).Normalize(b)
	if s(m, "type") == "result" {
		f.Done = true
		f.Failed, _ = m["is_error"].(bool)
	}
	return f
}
