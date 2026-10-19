package workspace

import "commitdeck/internal/identity"

func ID() string  { return identity.ID() }
func Now() string { return identity.Now() }

type Workspace struct {
	Lifecycle    string `json:"lifecycle"`
	ID           string `json:"id"`
	ExternalKey  string `json:"externalKey"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	RepoPath     string `json:"repoPath"`
	WorktreePath string `json:"worktreePath"`
	Branch       string `json:"branch"`
	JiraTicketID string `json:"jiraTicketId"`
	Archived     bool   `json:"archived"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
type Note struct {
	UpdatedBy   string `json:"updatedBy"`
	SessionID   string `json:"sessionId"`
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Content     string `json:"content"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
type Todo struct {
	UpdatedBy   string `json:"updatedBy"`
	SessionID   string `json:"sessionId"`
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Title       string `json:"title"`
	Completed   bool   `json:"completed"`
	Position    int    `json:"position"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
