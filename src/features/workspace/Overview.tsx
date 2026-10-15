import { GitPullRequest, Bot, ListTodo, ArrowUpRight } from "lucide-react";
import type { Detail, Tab } from "../../types";
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
}: {
  detail: Detail;
  setTab: (t: Tab) => void;
  refresh: () => Promise<void>;
  onSession: (id: string) => void;
}) {
  const complete = d.todos.filter((t) => t.completed).length,
    p = d.pr?.data;
  return (
    <div className="overview">
      <header className="overview-status">
        <AttentionBadge value={d.workspace.attention} />
        <span className="muted">{d.workspace.status}</span>
      </header>
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
