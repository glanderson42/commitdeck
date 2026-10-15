package context

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/issuetracker"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/workspace"
	"context"
)

type Options struct {
	MaxCharacters         int  `json:"maxCharacters"`
	IncludeIssue          bool `json:"includeIssue"`
	IncludeNotes          bool `json:"includeNotes"`
	IncludeTodos          bool `json:"includeTodos"`
	IncludeGit            bool `json:"includeGit"`
	IncludePR             bool `json:"includePR"`
	IncludeSessionSummary bool `json:"includeSessionSummary"`
}

func Defaults() Options           { return Options{12000, true, true, true, true, true, true} }
func Normalize(o Options) Options { o.MaxCharacters = max(1200, min(64000, o.MaxCharacters)); return o }

type GitContext struct {
	Branch        string   `json:"branch"`
	Dirty         bool     `json:"dirty"`
	ChangedFiles  []string `json:"changedFiles"`
	Summary       string   `json:"summary"`
	RecentCommits []string `json:"recentCommits"`
}
type WorkspaceContext struct {
	Workspace       workspace.Workspace      `json:"workspace"`
	Issue           *issuetracker.Issue      `json:"issue,omitempty"`
	Notes           string                   `json:"notes,omitempty"`
	Todos           []workspace.Todo         `json:"todos"`
	Git             *GitContext              `json:"git,omitempty"`
	PullRequest     *pullrequest.PullRequest `json:"pullRequest,omitempty"`
	RecentSessions  []protocol.Session       `json:"recentSessions"`
	Markdown        string                   `json:"markdown"`
	Hash            string                   `json:"hash"`
	GeneratedAt     string                   `json:"generatedAt"`
	CharacterCount  int                      `json:"characterCount"`
	EstimatedTokens int                      `json:"estimatedTokens"`
	IncludedSources []string                 `json:"includedSources"`
	Truncated       []string                 `json:"truncated"`
	Warnings        []string                 `json:"warnings"`
	Options         Options                  `json:"options"`
	Purpose         string                   `json:"purpose"`
}
type ContextEngine interface {
	BuildWorkspaceContext(context.Context, string, Options, string) (*WorkspaceContext, error)
}

var Purposes = []string{"investigation", "implementation", "testing", "review", "debugging", "documentation", "custom"}

func Purpose(value string) string {
	for _, p := range Purposes {
		if p == value {
			return p
		}
	}
	return "custom"
}
