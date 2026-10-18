# Ticket lifecycle and agent workflows

Start from **Overview → Start Work** after attaching a local repository. Review the branch, base revision, proposed worktree directory, optional origin fetch, terminal option, and optional Jira-transition follow-up. Confirming creates/reuses the ticket branch and creates its isolated worktree, then sets the local development lifecycle to `CODING`. `HEAD` is the default base; choose `origin/main` (or your team's base) when you want the fetched remote revision. Fetching does not change an existing checkout. Existing workspaces with a worktree continue through Agent or Terminal.

Local lifecycle is independent of Jira:

```text
BACKLOG → CODING → PR_OPEN → IN_REVIEW → CHANGES_REQUESTED
                                  ↘ READY_TO_MERGE → MERGED → DONE → ARCHIVED
```

PR/review/merge states come from normalized GitHub data. Mark Blocked/Unblock and Mark Done are explicit local actions. Jira can follow a different workflow. PR/Jira caches survive network failure; successful Jira writes refresh cached issue data and its displayed status without changing local lifecycle. After merge, choose a Jira transition, mark Done, archive, or remove the worktree yourself. Archive preserves notes, tasks, activity, agent/import/workflow metadata, and integration caches. It does not remove a worktree. Cleanup requires confirmation and refuses active agents/terminals, dirty files, and untracked or ignored files. Branches remain.

## GitHub actions

Use the PR tab with an authenticated `gh` CLI and a github.com origin. PR discovery/actions currently target branches within that repository; cross-repository fork PRs are excluded so an identically named fork branch cannot be changed accidentally. You can create/edit a PR, change draft state, add a comment, request a reviewer, reply to an unresolved thread, resolve it, rerun failed GitHub Actions jobs, and squash merge. Each remote write has an editable preview or confirmation dialog. The backend requires confirmation as well and re-reads the branch PR before executing the write.

PR creation pre-fills title, ticket, completed TODOs, and a Testing placeholder. It never claims that tests ran. Push your branch from Terminal first; PR creation does not push, commit, reset, or delete branches. Reviewers are individual GitHub usernames; team reviewer requests are not included yet. CI rerun is limited to the most recent 30 GitHub Actions runs at the PR's exact head commit; other CI providers expose their own links.

Merge requires the reviewed head SHA to still match, an open nondraft mergeable PR, approved review state, no changes requested, and all reported checks successful. GitHub branch protection and merge-queue rules remain authoritative. No admin bypass, auto-merge flag, automatic branch deletion, or automatic worktree cleanup is used. Zero checks does not itself count as review approval.

Review feedback has **Create TODO**, **Send to agent**, **Reply**, and **Resolve** actions. Agent handoff previews the review text/path/link, a redacted bounded local diff for that file against the available local base branch, and selected workspace context (including tasks, acceptance criteria, and other unresolved feedback). Sensitive paths are excluded. Missing base revisions are reported; fetch them in Terminal. The user chooses the target session/provider and edits the prompt. Completion never posts a reply or resolves a thread automatically. Replies target the thread's root comment even when acting on a later reply.

## Jira actions

The Jira tab fetches the issue's **currently available transitions** and lets you choose one. Quick palette commands for In Review/Done select a matching destination when it exists, otherwise show the available choices. They never invent transition IDs. Transitions are validated again before posting. You can also edit the issue summary and add an editable comment using Jira Cloud ADF. Workflows with required transition fields may reject a change; the issue remains usable locally. API credentials stay in the OS credential store, and the configured Jira account needs the corresponding write permissions.

PR creation/merge can suggest opening Jira transition selection. They do not transition the issue or post comments automatically. GitHub and Jira write verification uses fixtures; no production PR, review thread, CI run, or Jira issue was modified during implementation.

## Agent workflows

Overview offers **Implement Feature**, **Fix Bug**, **Investigate**, and **Review PR** presets. Choose each step's provider before creating the workflow. Creating it starts no agents. **Review next step** shows source selections, exact context preview, and an editable instruction/handoff prompt; **Approve and start this step** launches one owned session.

Each step receives fresh selected workspace sources and a compact handoff extracted from the previous step's last assistant text. Handoffs are redacted, limited to 3,000 characters, and visibly marked if truncated. Agents are instructed to summarize findings, actual changes, tests actually run, blockers, and recommended next work. This is deterministic extraction rather than an extra summarization-model request; review the result before relying on it.

On completion the workflow waits for another explicit approval. On failure it stops progression and offers Retry, Change Provider, Open Session, Skip, or Stop. Retry creates a new session while retaining the earlier session history. Skipping requires confirmation and does not fabricate results. Stopping cancels its active turn and remains stopped after the process exits. Restart marks in-flight steps failed so the user can review/retry; it does not pretend an old process resumed.

Workflow state and step/session links are persisted. Meaningful step completion/failure, PR merge, Jira updates, and review-count transitions feed the existing deduplicated activity/notification system. Manual sessions continue to work alongside workflows. There are no automatic multi-step chains or automatically resolved review items.
