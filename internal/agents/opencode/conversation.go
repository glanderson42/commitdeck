package opencode

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/permissions"
	"context"
	"os"
	"os/exec"
)

func bridgeCommand(ctx context.Context, cwd string) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		exe = "commitdeck-core"
	}
	cmd := exec.CommandContext(ctx, exe, "--agent-bridge", "opencode")
	cmd.Dir = cwd
	return cmd
}

type conversation struct{ external, prompt string }

func (Provider) NewConversation(id, cwd, prompt string) protocol.Conversation {
	return &conversation{id, prompt}
}
func (c *conversation) Initial() []any {
	return []any{map[string]any{"kind": "start", "external": c.external, "prompt": c.prompt}}
}
func (c *conversation) Handle(b []byte) protocol.Frame {
	m := protocol.Object(b)
	f := protocol.Frame{}
	switch protocol.Str(m, "type") {
	case "decision_ack":
		f.AckID = protocol.Str(m, "wireId")
		f.AckError = protocol.Str(m, "error")
	case "permission":
		r := protocol.Map(m["request"])
		request := &permissions.Request{Category: protocol.Str(r, "category"), Title: protocol.Str(r, "title"), Description: protocol.Str(r, "description"), Command: protocol.Str(r, "command"), FilePath: protocol.Str(r, "filePath"), Cwd: protocol.Str(r, "cwd")}
		f.Permission = request
		id := protocol.Str(m, "wireId")
		f.WireID = id
		f.RequiresAck = true
		f.Reply = func(allow bool) any { return map[string]any{"kind": "decision", "wireId": id, "allow": allow} }
	case "input_sent":
		f.InputSent = true
		f.ExternalID = protocol.Str(m, "sessionID")
	case "done":
		f.Done = true
		f.Failed, _ = m["failed"].(bool)
	default:
		f.Events, f.ExternalID = (Provider{}).Normalize(b)
	}
	return f
}
