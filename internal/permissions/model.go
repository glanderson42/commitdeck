package permissions

import (
	"commitdeck/internal/redact"
	"strings"
)

type Request struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspaceId"`
	SessionID     string   `json:"sessionId"`
	Provider      string   `json:"provider"`
	Category      string   `json:"category"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Command       string   `json:"command"`
	FilePath      string   `json:"filePath"`
	Cwd           string   `json:"cwd"`
	UnverifiedCwd bool     `json:"-"`
	Executable    string   `json:"executable"`
	Arguments     []string `json:"arguments"`
	HighRisk      bool     `json:"highRisk"`
	CanRespond    bool     `json:"canRespond"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"createdAt"`
	ResolvedAt    string   `json:"resolvedAt,omitempty"`
}

// Redacted retains approval diagnostics but prevents secret-bearing executable
// metadata from becoming a reusable rule. Original wire inputs stay in memory.
func (r Request) Redacted() Request {
	originalCommand, originalPath, originalCwd := r.Command, r.FilePath, r.Cwd
	r.Title = redact.Text(r.Title)
	r.Description = redact.Text(r.Description)
	r.Command = redact.Text(r.Command)
	r.FilePath = redact.Text(r.FilePath)
	r.Cwd = redact.Text(r.Cwd)
	argv := strings.Join(append([]string{r.Executable}, r.Arguments...), " ")
	if originalCommand != r.Command || originalPath != r.FilePath || originalCwd != r.Cwd || redact.Text(argv) != argv {
		r.HighRisk = true
		r.Executable = ""
		r.Arguments = []string{}
	}
	return r
}

type Rule struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspaceId"`
	SessionID   string   `json:"sessionId,omitempty"`
	Scope       string   `json:"scope"`
	Executable  string   `json:"executable"`
	ArgPrefix   []string `json:"argPrefix"`
	Cwd         string   `json:"cwd"`
	CreatedAt   string   `json:"createdAt"`
}
type Decision struct {
	Allow bool   `json:"allow"`
	Scope string `json:"scope"`
}
