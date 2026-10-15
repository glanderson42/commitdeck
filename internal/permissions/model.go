package permissions

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
