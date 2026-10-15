CREATE TABLE workspaces (
 id TEXT PRIMARY KEY, external_key TEXT NOT NULL, title TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'In Progress', repo_path TEXT NOT NULL DEFAULT '',
 worktree_path TEXT NOT NULL DEFAULT '', branch TEXT NOT NULL DEFAULT '', jira_ticket_id TEXT NOT NULL DEFAULT '',
 archived INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX workspaces_updated ON workspaces(archived, updated_at DESC);
CREATE TABLE notes (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL UNIQUE REFERENCES workspaces(id) ON DELETE CASCADE,
 content TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE todos (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 title TEXT NOT NULL, completed INTEGER NOT NULL DEFAULT 0, position INTEGER NOT NULL,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX todos_workspace_position ON todos(workspace_id, position);
CREATE TABLE agent_sessions (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 provider TEXT NOT NULL, external_session_id TEXT NOT NULL DEFAULT '', title TEXT NOT NULL,
 purpose TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, pid INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX sessions_workspace ON agent_sessions(workspace_id, created_at);
CREATE TABLE agent_events (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 id TEXT NOT NULL UNIQUE, session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
 payload TEXT NOT NULL
);
CREATE INDEX events_session ON agent_events(session_id, sequence);
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE jira_cache (
 workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
 payload TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE pull_request_cache (
 workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
 payload TEXT NOT NULL, updated_at TEXT NOT NULL
);
