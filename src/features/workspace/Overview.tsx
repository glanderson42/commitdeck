import { WorkspaceSkill } from "../agents/WorkspaceSkill";
import { WorkflowsPane } from "../workflows/WorkflowsPane";
import { rpc } from "../../services/api";
import { GitPullRequest, Bot, ListTodo, ArrowUpRight } from "lucide-react";
import type { Detail, Tab, Provider } from "../../types";
import { AttentionBadge, ExternalLink } from "../../components/ui";
import {
  PermissionQueue,
  PermissionRules,
} from "../permissions/PermissionQueue";
import { ActivityTimeline } from "../activity/ActivityTimeline";
export function Overview({
  detail: d,
  setTab,
  refresh,
  onSession,
  providers,
  onAction,
  workflowIntent,
  onHandled,
}: {
  detail: Detail;
  setTab: (t: Tab) => void;
  refresh: () => Promise<void>;
  onSession: (id: string) => void;
  providers: Provider[];
  onAction: (s: string) => void;
  workflowIntent?: boolean;
  onHandled: () => void;
}) {
  const complete = d.todos.filter((t) => t.completed).length,
    p = d.pr?.data;
  return (
    <div className="overview">
      <header className="overview-status">
        <AttentionBadge value={d.workspace.attention} />
        <span className="muted">{d.workspace.lifecycle || "BACKLOG"}</span>
      </header>
      <div className="integration-actions">
        <WorkspaceSkill
          id={d.workspace.id}
          disabled={d.workspace.archived || !d.workspace.worktreePath}
        />
        <button
          disabled={
            d.workspace.archived ||
            !d.workspace.repoPath ||
            !!d.workspace.worktreePath
          }
          onClick={() => onAction("start-work")}
        >
          Start Work
        </button>
        <button
          disabled={d.workspace.archived}
          onClick={() => onAction("start-agent")}
        >
          Start Agent
        </button>
        <button
          disabled={!d.workspace.worktreePath || d.workspace.archived}
          onClick={() => onAction("new-terminal")}
        >
          New Terminal
        </button>
        <button
          disabled={d.workspace.archived || !d.workspace.branch}
          onClick={() => onAction("create-pr")}
        >
          Create PR
        </button>
        <button
          disabled={d.workspace.archived}
          onClick={() =>
            void rpc("lifecycle.set", {
              id: d.workspace.id,
              action:
                d.workspace.lifecycle === "BLOCKED" ? "CODING" : "BLOCKED",
              confirm: true,
            })
              .then(refresh)
              .catch((e) => window.alert(String(e)))
          }
        >
          {" "}
          {d.workspace.lifecycle === "BLOCKED" ? "Unblock" : "Mark Blocked"}
        </button>
        <button
          disabled={d.workspace.archived}
          onClick={() =>
            void rpc("lifecycle.set", {
              id: d.workspace.id,
              action: "DONE",
              confirm: true,
            })
              .then(refresh)
              .catch((e) => window.alert(String(e)))
          }
        >
          Mark Done
        </button>
      </div>
      <div className="overview-summary">
        <button onClick={() => setTab("Tasks")}>
          <ListTodo size={17} />
          <div>
            <strong>
              {complete} / {d.todos.length}
            </strong>
            <span>Tasks complete</span>
          </div>
          <ArrowUpRight size={13} />
        </button>
        <button onClick={() => setTab("Agent")}>
          <Bot size={17} />
          <div>
            <strong>
              {
                d.sessions.filter((s) =>
                  ["running", "starting"].includes(s.status),
                ).length
              }{" "}
              running
            </strong>
            <span>{d.sessions.length} agent sessions</span>
          </div>
          <ArrowUpRight size={13} />
        </button>
        <button onClick={() => setTab("PR")}>
          <GitPullRequest size={17} />
          <div>
            <strong>{p ? "PR #" + p.number : "No pull request"}</strong>
            <span>
              {p
                ? `${p.checks.filter((c) => c.status === "success").length} passed · ${p.checks.filter((c) => c.status === "failure").length} failed`
                : "Connect a branch to track review"}
            </span>
          </div>
          <ArrowUpRight size={13} />
        </button>
      </div>
      <section className="overview-terminals">
        <h3>
          Terminals{" "}
          <span className="muted">
            {d.terminals?.length ?? 0} tabs ·{" "}
            {d.terminals?.filter((t) => t.status === "running").length ?? 0}{" "}
            shells running
          </span>
        </h3>
        {d.terminals?.map((t) => (
          <button key={t.id} onClick={() => setTab("Terminal")}>
            <i
              className={`dot ${t.status === "running" ? "green" : "muted"}`}
            />
            {t.title} <span className="muted">{t.status}</span>
          </button>
        ))}
      </section>
      <WorkflowsPane
        workspaceId={d.workspace.id}
        providers={providers}
        onSession={onSession}
        refresh={refresh}
        disabled={d.workspace.archived || !d.workspace.worktreePath}
        intent={workflowIntent}
        onHandled={onHandled}
      />
      {d.permissions.some((r) => r.status === "pending") && (
        <section>
          <h3>Needs approval</h3>
          <PermissionQueue
            requests={d.permissions.filter((r) => r.status === "pending")}
            refresh={refresh}
          />
        </section>
      )}
      {p && (p.unresolvedComments?.length > 0 || p.changesRequested > 0) && (
        <section className="overview-feedback">
          <h3>Unresolved feedback</h3>
          {p.unresolvedComments?.slice(0, 5).map((c, i) => (
            <blockquote key={i}>
              <strong>{c.author}</strong>
              <p>{c.body}</p>
              {c.url && <ExternalLink url={c.url}>View thread</ExternalLink>}
            </blockquote>
          ))}
          {!p.unresolvedComments?.length &&
            p.reviews
              .filter((r) => r.state === "CHANGES_REQUESTED")
              .map((r, i) => (
                <blockquote key={i}>
                  <strong>{r.author}</strong>
                  <p>{r.body || "Changes requested"}</p>
                </blockquote>
              ))}
          <button onClick={() => setTab("PR")}>Open pull request</button>
        </section>
      )}
      <div className="overview-columns">
        <section>
          <h3>Remaining work</h3>
          {d.todos
            .filter((t) => !t.completed)
            .slice(0, 8)
            .map((t) => (
              <button
                className="overview-todo"
                key={t.id}
                onClick={() => setTab("Tasks")}
              >
                <span className="mini-checkbox" />
                {t.title}
              </button>
            ))}
          {!d.todos.some((t) => !t.completed) && (
            <p className="muted">No unfinished tasks.</p>
          )}
          <h3 className="overview-agents-heading">Agent sessions</h3>
          {d.sessions.map((s) => (
            <button
              key={s.id}
              className="overview-session"
              onClick={() => onSession(s.id)}
            >
              <span className={`provider-symbol small provider-${s.provider}`}>
                ◈
              </span>
              <span>
                {s.title}
                <small>
                  {s.provider} · {s.purpose}
                </small>
              </span>
              <span className="muted">{s.status.replace("_", " ")}</span>
            </button>
          ))}
        </section>
        <ActivityTimeline events={d.activity} />
      </div>
      <PermissionRules rules={d.permissionRules} refresh={refresh} />
    </div>
  );
}
