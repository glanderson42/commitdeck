package protocol

import (
	"commitdeck/internal/permissions"
	"commitdeck/internal/redact"
	"context"
	"encoding/json"
	"os/exec"
)

type Event struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ToolName  string `json:"toolName,omitempty"`
	Command   string `json:"command,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Diff      string `json:"diff,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// Redacted removes credential-shaped values before display or persistence.
func (e Event) Redacted() Event {
	e.Text = redact.Text(e.Text)
	e.ToolName = redact.Text(e.ToolName)
	e.Command = redact.Text(e.Command)
	e.FilePath = redact.Text(e.FilePath)
	e.Diff = redact.Text(e.Diff)
	e.Error = redact.Text(e.Error)
	return e
}

type Session struct {
	Imported          bool   `json:"imported"`
	ResumeSupported   bool   `json:"resumeSupported"`
	SourceCWD         string `json:"sourceCwd"`
	ID                string `json:"id"`
	WorkspaceID       string `json:"workspaceId"`
	Provider          string `json:"provider"`
	ExternalSessionID string `json:"externalSessionId"`
	Title             string `json:"title"`
	Purpose           string `json:"purpose"`
	Status            string `json:"status"`
	PID               int    `json:"pid"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// RedactedMetadata protects descriptive fields without changing provider IDs
// or other values needed to resume the underlying session.
func (s Session) RedactedMetadata() Session {
	s.Title = redact.Text(s.Title)
	s.Purpose = redact.Text(s.Purpose)
	s.SourceCWD = redact.Text(s.SourceCWD)
	return s
}

// Runtime isolates CLI arguments and wire formats from lifecycle and UI code.
type Runtime interface {
	ID() string
	Name() string
	IsAvailable(context.Context) bool
	Command(context.Context, string, string, string) *exec.Cmd
	Normalize([]byte) ([]Event, string)
}
type Frame struct {
	AckID       string
	AckError    string
	RequiresAck bool
	WireID      string
	Writes      []any
	Events      []Event
	ExternalID  string
	Permission  *permissions.Request
	Reply       func(bool) any
	Done        bool
	Failed      bool
	InputSent   bool
}
type Conversation interface {
	Initial() []any
	Handle([]byte) Frame
}
type InteractiveRuntime interface {
	NewConversation(string, string, string) Conversation
}

func Available(binary string) bool          { _, err := exec.LookPath(binary); return err == nil }
func Object(b []byte) map[string]any        { var m map[string]any; _ = json.Unmarshal(b, &m); return m }
func Map(v any) map[string]any              { m, _ := v.(map[string]any); return m }
func Str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func List(v any) []any                      { a, _ := v.([]any); return a }
func String(v any) string                   { s, _ := v.(string); return s }
func Tool(name string, input map[string]any) Event {
	e := Event{Type: "tool", ToolName: name}
	e.Command = Str(input, "command")
	e.FilePath = Str(input, "file_path")
	if e.FilePath == "" {
		e.FilePath = Str(input, "filePath")
	}
	if e.Command != "" {
		e.Type = "command"
	}
	if name == "Edit" || name == "Write" || name == "edit" || name == "write" {
		e.Type = "file_changed"
	}
	return e
}
