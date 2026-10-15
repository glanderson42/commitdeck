CREATE TABLE permission_requests (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
 status TEXT NOT NULL DEFAULT 'pending', payload TEXT NOT NULL, created_at TEXT NOT NULL, resolved_at TEXT
);
CREATE INDEX permissions_pending ON permission_requests(status,workspace_id,created_at);
CREATE TABLE permission_rules (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 session_id TEXT REFERENCES agent_sessions(id) ON DELETE CASCADE, scope TEXT NOT NULL CHECK(scope IN ('workspace','session')),
 executable TEXT NOT NULL, arguments TEXT NOT NULL, cwd TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE agent_session_context (
 id TEXT PRIMARY KEY, session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
 workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 generated_at TEXT NOT NULL, context_hash TEXT NOT NULL, included_sources TEXT NOT NULL,
 character_count INTEGER NOT NULL, options TEXT NOT NULL, markdown TEXT NOT NULL, sent_at TEXT
);
CREATE INDEX context_session ON agent_session_context(session_id,generated_at DESC);
CREATE TABLE workspace_events (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE,
 workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 type TEXT NOT NULL, title TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}',
 dedupe_key TEXT UNIQUE, notify INTEGER NOT NULL DEFAULT 0, read INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
CREATE INDEX activity_workspace ON workspace_events(workspace_id,sequence DESC);
CREATE INDEX activity_notifications ON workspace_events(notify,read,sequence DESC);
CREATE TABLE workspace_signals (
 workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 source TEXT NOT NULL, value TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 1,
 PRIMARY KEY(workspace_id,source)
);
UPDATE agent_sessions SET status=CASE status WHEN 'complete' THEN 'completed' WHEN 'error' THEN 'failed' WHEN 'waiting' THEN 'idle' WHEN 'interrupted' THEN 'idle' ELSE status END;
