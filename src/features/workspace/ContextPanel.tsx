import {
  ArrowUpRight,
  FolderGit2,
  GitBranch,
  Plus,
  Terminal,
} from "lucide-react";
import { AttentionBadge, timeAgo } from "../../components/ui";
import type { Deck } from "../../hooks/useDeck";
import type { Tab } from "../../types";
export function ContextPanel({
  deck,
  setTab,
}: {
  deck: Deck;
  setTab: (tab: Tab) => void;
}) {
  const { detail, layout, setLayout } = deck;
  if (!detail) return null;
  const w = detail.workspace,
    completed = detail.todos.filter((t) => t.completed).length;
  return (
    <aside className="context-panel" style={{ width: layout.contextWidth }}>
      <div className="context-heading">TICKET CONTEXT</div>
      <section>
        <div className="section-label">ATTENTION</div>
        <AttentionBadge value={w.attention} />
        <p className="context-description">
          {attentionDescription(w.attention)}
        </p>
      </section>
      <section>
        <div className="section-label">DEVELOPMENT</div>
        <div className="context-item">
          <FolderGit2 size={14} />
          <span title={w.repoPath}>
            {w.repoPath ? w.repoPath.split(/[\\/]/).pop() : "No repository"}
          </span>
          {!w.repoPath && (
            <button onClick={() => setTab("Git")}>
              <Plus size={13} />
            </button>
          )}
        </div>
        <div className="context-item">
          <GitBranch size={14} />
          <span title={w.branch}>{w.branch || "No branch"}</span>
        </div>
        <div className="context-item">
          <span className={`dot ${w.worktreePath ? "green" : "muted"}`} />
          <span>{w.worktreePath ? "Isolated worktree" : "No worktree"}</span>
        </div>
        {w.worktreePath && (
          <button className="context-open" onClick={() => setTab("Terminal")}>
            <Terminal size={13} />
            Open terminal
            <ArrowUpRight size={12} />
          </button>
        )}
      </section>
      <section>
        <div className="section-label">
          TASK PROGRESS
          <button
            className="icon-button"
            aria-label="Open tasks"
            onClick={() => setTab("Tasks")}
          >
            <ArrowUpRight size={13} />
          </button>
        </div>
        <div className="context-progress">
          <strong>
            {completed}
            <span> / {detail.todos.length}</span>
          </strong>
          <span>
            {detail.todos.length
              ? Math.round((completed / detail.todos.length) * 100)
              : 0}
            %
          </span>
        </div>
        <div className="progress-track">
          <div
            style={{
              width: `${detail.todos.length ? (completed / detail.todos.length) * 100 : 0}%`,
            }}
          />
        </div>
        {detail.todos
          .filter((t) => !t.completed)
          .slice(0, 3)
          .map((t) => (
            <div key={t.id} className="context-todo">
              <span className="mini-checkbox" />
              {t.title}
            </div>
          ))}
      </section>
      <section>
        <div className="section-label">
          AGENTS<span>{detail.sessions.length}</span>
        </div>
        {detail.sessions.length ? (
          detail.sessions.slice(0, 6).map((s) => (
            <button
              key={s.id}
              className="context-session"
              onClick={() => {
                setTab("Agent");
                setLayout((l) => ({
                  ...l,
                  sessions: { ...l.sessions, [w.id]: s.id },
                }));
              }}
            >
              <span className={`provider-symbol small provider-${s.provider}`}>
                ◈
              </span>
              <span>{s.title}</span>
              <i
                className={`dot ${s.status === "running" ? "purple pulse" : s.status === "waiting" ? "yellow" : s.status === "error" ? "red" : "muted"}`}
              />
            </button>
          ))
        ) : (
          <p className="muted tiny">No sessions yet.</p>
        )}
      </section>
      <div className="context-bottom">
        <span>Created {new Date(w.createdAt).toLocaleDateString()}</span>
        {detail.pr && <span>GitHub synced {timeAgo(detail.pr.updatedAt)}</span>}
        {detail.jira && (
          <span>Jira synced {timeAgo(detail.jira.updatedAt)}</span>
        )}
      </div>
    </aside>
  );
}
function attentionDescription(value: string) {
  return (
    (
      {
        AGENT_WAITING: "An agent needs your input before it can continue.",
        CHANGES_REQUESTED: "A reviewer requested changes on this pull request.",
        CI_FAILED: "One or more checks need attention.",
        CODING: "Development is in progress in this worktree.",
        READY_TO_MERGE:
          "Reviews and checks are complete. Ready for your final look.",
        WAITING_FOR_REVIEW:
          "The pull request is open. Track reviews and checks here.",
        NO_PR: "Focus on the work. Connect a pull request when it’s ready.",
        IDLE: "Everything is up to date.",
      } as Record<string, string>
    )[value] ?? ""
  );
}
