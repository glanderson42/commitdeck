package codex

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/permissions"
	"fmt"
)

type conversation struct {
	external, cwd, prompt string
	items                 map[string]map[string]any
}

func (Provider) NewConversation(id, cwd, prompt string) protocol.Conversation {
	return &conversation{external: id, cwd: cwd, prompt: prompt, items: map[string]map[string]any{}}
}
func (c *conversation) Initial() []any {
	return []any{map[string]any{"id": "cd_init", "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "commitdeck", "version": "0.1.0"}}}}
}
func rpc(id, method string, params any) any {
	return map[string]any{"id": id, "method": method, "params": params}
}
func (c *conversation) Handle(b []byte) protocol.Frame {
	m := protocol.Object(b)
	s := protocol.Str
	p := protocol.Map(m["params"])
	method := s(m, "method")
	f := protocol.Frame{}
	if e := protocol.Map(m["error"]); e != nil {
		f.Events = []protocol.Event{{Type: "error", Error: s(e, "message")}}
		f.Done = true
		f.Failed = true
		return f
	}
	switch s(m, "id") {
	case "cd_init":
		params := map[string]any{"cwd": c.cwd, "sandbox": "read-only", "approvalPolicy": "untrusted"}
		method := "thread/start"
		if c.external != "" {
			method = "thread/resume"
			params["threadId"] = c.external
		}
		f.Writes = []any{map[string]any{"method": "initialized"}, rpc("cd_thread", method, params)}
		return f
	case "cd_thread":
		thread := protocol.Map(protocol.Map(m["result"])["thread"])
		id := s(thread, "id")
		if id == "" {
			f.Done = true
			f.Failed = true
			f.Events = []protocol.Event{{Type: "error", Error: "Codex did not return a thread ID"}}
			return f
		}
		c.external = id
		f.ExternalID = id
		f.InputSent = true
		f.Writes = []any{rpc("cd_turn", "turn/start", map[string]any{"threadId": id, "input": []any{map[string]any{"type": "text", "text": c.prompt, "text_elements": []any{}}}})}
		return f
	}
	switch method {
	case "item/started", "item/completed":
		item := protocol.Map(p["item"])
		c.items[s(item, "id")] = item
		if method == "item/completed" {
			switch s(item, "type") {
			case "agentMessage":
				f.Events = append(f.Events, protocol.Event{Type: "text", Text: s(item, "text")})
			case "commandExecution":
				f.Events = append(f.Events, protocol.Event{Type: "command", Command: s(item, "command"), Text: s(item, "aggregatedOutput")})
			case "fileChange":
				for _, v := range protocol.List(item["changes"]) {
					change := protocol.Map(v)
					f.Events = append(f.Events, protocol.Event{Type: "file_changed", FilePath: s(change, "path"), Diff: s(change, "diff")})
				}
			}
		}
	case "turn/diff/updated":
		f.Events = []protocol.Event{{Type: "diff", Diff: s(p, "diff")}}
	case "error":
		f.Events = []protocol.Event{{Type: "error", Error: s(protocol.Map(p["error"]), "message")}}
		f.Failed = true
	case "turn/completed":
		turn := protocol.Map(p["turn"])
		f.Done = true
		f.Failed = s(turn, "status") == "failed"
		if f.Failed {
			f.Events = append(f.Events, protocol.Event{Type: "error", Error: s(protocol.Map(turn["error"]), "message")})
		}
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval":
		req := &permissions.Request{Category: "shell_command", Title: "Run command", Command: s(p, "command"), Description: s(p, "reason"), Cwd: s(p, "cwd")}
		if method == "item/fileChange/requestApproval" {
			req.Category = "file_write"
			req.Title = "Apply file changes"
			for _, v := range protocol.List(c.items[s(p, "itemId")]["changes"]) {
				change := protocol.Map(v)
				if req.FilePath != "" {
					req.FilePath += "\n"
				}
				req.FilePath += s(change, "path")
				kind := protocol.Map(change["kind"])
				if s(kind, "type") == "delete" {
					req.Category = "file_delete"
				}
				req.Description += "\n" + s(change, "diff")
			}
		}
		if network := protocol.Map(p["networkApprovalContext"]); network != nil {
			req.Category = "network_request"
			req.Title = "Network access to " + s(network, "host")
		}
		if method == "item/permissions/requestApproval" {
			req.Category = "unknown"
			req.Title = "Additional filesystem or network access"
			req.Description += "\n" + describePermissions(protocol.Map(p["permissions"]))
		}
		f.Permission = req
		wireID := m["id"]
		f.Reply = func(allow bool) any {
			decision := "decline"
			if allow {
				decision = "accept"
			}
			result := any(map[string]any{"decision": decision})
			if method == "item/permissions/requestApproval" {
				granted := any(map[string]any{})
				if allow {
					granted = p["permissions"]
				}
				result = map[string]any{"permissions": granted, "scope": "turn"}
			}
			return map[string]any{"id": wireID, "result": result}
		}
	default:
		// Every unknown server request is explicitly rejected instead of silently granting a capability.
		if _, exists := m["id"]; exists && method != "" {
			f.Writes = []any{map[string]any{"id": m["id"], "error": map[string]any{"code": -32601, "message": fmt.Sprintf("CommitDeck does not support %s", method)}}}
		}
	}
	return f
}

func describePermissions(p map[string]any) string {
	text := "Requested access lasts for the current Codex turn only."
	if network := protocol.Map(p["network"]); network != nil {
		if enabled, _ := network["enabled"].(bool); enabled {
			text += "\nNetwork access: enabled"
		}
	}
	fs := protocol.Map(p["fileSystem"])
	for _, access := range []string{"read", "write"} {
		for _, path := range protocol.List(fs[access]) {
			text += "\n" + access + ": " + protocol.String(path)
		}
	}
	for _, value := range protocol.List(fs["entries"]) {
		entry := protocol.Map(value)
		path := protocol.Map(entry["path"])
		target := protocol.Str(path, "path")
		if target == "" {
			target = protocol.Str(path, "pattern")
		}
		if target == "" {
			special := protocol.Map(path["value"])
			target = protocol.Str(special, "kind") + " " + protocol.Str(special, "subpath")
		}
		text += "\n" + protocol.Str(entry, "access") + ": " + target
	}
	return text
}
