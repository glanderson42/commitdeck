ALTER TABLE workspaces ADD COLUMN lifecycle TEXT NOT NULL DEFAULT 'BACKLOG';
UPDATE workspaces SET lifecycle=CASE WHEN archived=1 THEN 'ARCHIVED' WHEN worktree_path<>'' THEN 'CODING' ELSE 'BACKLOG' END;
ALTER TABLE notes ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'user';
ALTER TABLE notes ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE todos ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'user';
ALTER TABLE todos ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN imported INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_sessions ADD COLUMN resume_supported INTEGER NOT NULL DEFAULT 1;
ALTER TABLE agent_sessions ADD COLUMN source_cwd TEXT NOT NULL DEFAULT '';
CREATE TABLE terminals (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 title TEXT NOT NULL, cwd TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'stopped',
 created_at TEXT NOT NULL, last_used_at TEXT NOT NULL
);
CREATE INDEX terminals_workspace ON terminals(workspace_id,created_at);
CREATE TABLE agent_imports (
 provider TEXT NOT NULL, external_session_id TEXT NOT NULL,
 session_id TEXT NOT NULL UNIQUE REFERENCES agent_sessions(id) ON DELETE CASCADE,
 source TEXT NOT NULL, imported_at TEXT NOT NULL,
 PRIMARY KEY(provider,external_session_id)
);
CREATE TABLE agent_workflows (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 name TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX workflows_workspace ON agent_workflows(workspace_id,created_at);
CREATE TABLE agent_workflow_steps (
 id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL REFERENCES agent_workflows(id) ON DELETE CASCADE,
 position INTEGER NOT NULL, purpose TEXT NOT NULL, provider TEXT NOT NULL, status TEXT NOT NULL,
 session_id TEXT REFERENCES agent_sessions(id) ON DELETE SET NULL,
 handoff TEXT NOT NULL DEFAULT '', UNIQUE(workflow_id,position)
);
CREATE TABLE workspace_tool_events (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
 operation TEXT NOT NULL, target_id TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
