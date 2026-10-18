# Session history: design notes, not implemented features

Agent transcripts, context snapshots and activity persist locally. Terminal buffers remain memory-only. There is no automatic transcript retention policy or export UI; existing history is not deleted by this release.

A future export should require explicit workspace/session selection, preview recognized-secret redaction, include versioned normalized JSON plus readable Markdown, exclude credentials/capabilities/environment/terminal buffers, and warn that redaction is incomplete. Write to a user-selected destination with private permissions and no network upload. Test restore/import independently before claiming backups can round-trip through export.

A future retention policy should be opt-in, preview affected sessions and byte counts, preserve notes/tasks/ticket state, handle workflow references transactionally, and require backup/confirmation before deletion. Never silently prune active sessions. SQLite free-page/WAL behavior and historical backups mean deletion is not a secure-erasure guarantee.
