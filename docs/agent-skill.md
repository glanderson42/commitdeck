# CommitDeck workspace skill

Enable **Workspace tools (TODOs and notes)** in a new-session context selection or an explicit context refresh. The exact instructions appear in the Markdown preview and saved context snapshot. At budgets below 4,000 characters the engine visibly disables tools so their protocol instructions cannot consume the whole context or be silently omitted. Existing/imported sessions without a tool-enabled snapshot do not receive a capability.

The agent invokes the bundled sidecar as a small CLI client, using the executable path in its environment:

```sh
"$COMMITDECK_TOOL_BIN" --workspace-tool todo.add '{"title":"Add permanent failure test"}'
"$COMMITDECK_TOOL_BIN" --workspace-tool notes.append '{"content":"Decision: preserve invoice ID as the idempotency seed."}'
```

PowerShell uses `& $env:COMMITDECK_TOOL_BIN --workspace-tool OPERATION JSON`. Agents use their provider's ordinary command/tool approval flow to invoke the helper. Provider sandboxes may require an additional explicit approval for the helper or its loopback connection. CommitDeck does not bypass those restrictions.

| Operation       | Payload / behavior                                                                                     |
| --------------- | ------------------------------------------------------------------------------------------------------ |
| `workspace.get` | Normalized ticket key, title, status, local lifecycle, branch, and worktree                            |
| `todo.list`     | Current workspace tasks                                                                                |
| `todo.add`      | `title`                                                                                                |
| `todo.update`   | Existing task `id`, `title`                                                                            |
| `todo.complete` | Existing task `id`, `completed` boolean                                                                |
| `todo.delete`   | Existing task `id`; requires individual permission                                                     |
| `notes.get`     | Main Markdown note with update timestamp                                                               |
| `notes.append`  | `content`; preserves existing note text                                                                |
| `notes.update`  | Replacement `content`, `expectedUpdatedAt`; requires individual permission and rejects stale revisions |
| `activity.add`  | Concise `content` (up to 1,000 characters)                                                             |

Add concrete discovered work, complete only finished tasks, and append material decisions and blockers. Avoid chat dumps and trivial progress spam. Never delete or replace user notes without explicit instruction. There is no note-delete operation. Tool-created tasks and note edits record `updated_by`, agent session ID, update time, a workspace-tool audit event, and a meaningful activity entry. Attribution is shown in Tasks and Notes. A later user edit is attributed to the user.

An optional **Overview → Workspace skill…** action previews and creates a discoverable `SKILL.md` under `.agents/skills/commitdeck-workspace` (Codex), `.claude/skills/commitdeck-workspace` (Claude), or `.opencode/skills/commitdeck-workspace` (OpenCode). File creation requires confirmation, is confined with Go's filesystem root API, and refuses to overwrite an existing file. These ordinary worktree files appear in Git. They contain instructions only and have no credentials. The prompt protocol works without installing a skill file; discovery support depends on the provider version.

Each managed turn receives an independent 256-bit random capability through environment variables. The helper contacts an ephemeral server bound only to `127.0.0.1`. That server accepts only its exact Host, no browser Origin, a valid bearer capability, a POST to `/tool`, and a bounded JSON body. Capabilities bind to a single workspace and session, are never written to SQLite/context/skill files, and are revoked when the turn ends. The helper refuses non-loopback endpoints and redirects. User-facing desktop RPC continues over private pipes.

Caller-supplied workspace/session IDs, SQL, filesystem paths, credential reads, and Jira/GitHub operations are not part of this protocol. Unknown operations fail. Known secret patterns are redacted from note/task reads and newly supplied knowledge text. Redaction is best effort; do not put secrets into workspace notes. Safe edits can happen automatically once the user enables tools and the provider allows helper execution. TODO deletion and **every** note replacement enter the existing permission queue with individual approval only; workspace/session remembered rules cannot approve them.

Notes use optimistic conflict checks. Agent appends and replacements cannot overwrite an intervening user save, and editor autosave cannot overwrite an agent append. A conflicting user draft stays in local backup with a visible error. Choose **Compare with saved note**, merge the latest saved text with your draft, and explicitly save the reviewed result. Backups preserve their original base version; a legacy or conflicting recovered draft is never replayed automatically over newer notes.

This API provides a constrained way to maintain workspace knowledge. A general coding agent already has its provider's filesystem/process capabilities; the API does not turn that provider into an OS sandbox. Never ask agents to locate CommitDeck database files or credentials as a fallback for an unavailable capability.
