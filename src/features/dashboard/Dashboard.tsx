import { useMemo, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  Search,
  GitBranch,
  GitPullRequest,
  Plus,
  Layers3,
  RotateCw,
  ArrowUpRight,
} from "lucide-react";
import type { Deck } from "../../hooks/useDeck";
import type { DashboardRow } from "../../types";
import { AttentionBadge, Empty, timeAgo } from "../../components/ui";
const filters = [
  "All",
  "Needs attention",
  "Active",
  "Ready",
  "Waiting",
  "Archived",
] as const;
const groups = [
  "Needs attention",
  "Active work",
  "Ready",
  "Waiting",
  "Recently updated",
];
export function Dashboard({
  deck,
  onCreate,
}: {
  deck: Deck;
  onCreate: () => void;
}) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<(typeof filters)[number]>("All");
  const root = useRef<HTMLDivElement>(null);
  const rows = useMemo(
    () =>
      deck.boot?.workspaces.filter(
        (w) =>
          w.archived === (filter === "Archived") &&
          `${w.searchText} ${w.externalKey} ${w.title} ${w.branch} #${w.pr?.number ?? ""}`
            .toLowerCase()
            .includes(query.toLowerCase()) &&
          (filter === "All" ||
            filter === "Archived" ||
            w.group === (filter === "Active" ? "Active work" : filter)),
      ) ?? [],
    [deck.boot?.workspaces, query, filter],
  );
  const recent = [...rows]
    .sort((a, b) => b.lastActivity.localeCompare(a.lastActivity))
    .slice(0, 5);
  const keyboard = (e: React.KeyboardEvent) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    if (e.target instanceof HTMLInputElement) return;
    e.preventDefault();
    const items = Array.from(
      root.current?.querySelectorAll<HTMLButtonElement>(".dashboard-row") ?? [],
    );
    const index = items.indexOf(document.activeElement as HTMLButtonElement);
    items[
      Math.max(
        0,
        Math.min(items.length - 1, index + (e.key === "ArrowDown" ? 1 : -1)),
      )
    ]?.focus();
  };
  return (
    <div className="dashboard" ref={root} onKeyDown={keyboard}>
      <header className="dashboard-heading">
        <div>
          <div className="eyebrow">YOUR DEVELOPMENT CONTROL CENTER</div>
          <h1>My work</h1>
          <p>Pick the next action. Keep every ticket in context.</p>
        </div>
        <button
          onClick={() => {
            void deck.syncAll(true);
            void deck.refresh();
          }}
        >
          <RotateCw size={14} />
          Refresh
        </button>
        <button className="primary" onClick={onCreate}>
          <Plus size={14} />
          New ticket
        </button>
      </header>
      <div className="dashboard-tools">
        <div className="dashboard-filters">
          {filters.map((f) => (
            <button
              key={f}
              aria-pressed={filter === f}
              onClick={() => setFilter(f)}
            >
              {f}
            </button>
          ))}
        </div>
        <div className="dashboard-search">
          <Search size={14} />
          <input
            aria-label="Search all workspace context"
            placeholder="Search tickets, branches, PRs, notes…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>
      {deck.boot?.permissions.length ? (
        <button
          className="permission-banner"
          onClick={() => deck.setPermissionsOpen(true)}
        >
          <i className="dot yellow" />
          {deck.boot.permissions.length} approval
          {deck.boot.permissions.length === 1 ? "" : "s"} waiting
          <span>
            Review requests <ArrowUpRight size={13} />
          </span>
        </button>
      ) : null}
      {!deck.boot?.workspaces.length ? (
        <Empty icon={<Layers3 size={26} />} title="Your work, in one place">
          <p>
            Create a workspace or import a Jira ticket to start tracking code,
            agents, reviews, and the next action.
          </p>
          <button className="primary" onClick={onCreate}>
            Create a workspace
          </button>
        </Empty>
      ) : rows.length === 0 ? (
        <Empty title="No matching workspaces">
          <p>Try another search or filter.</p>
        </Empty>
      ) : (
        groups.map((group) => {
          const list =
            group === "Recently updated"
              ? recent
              : rows.filter((r) => r.group === group);
          const collapsed = deck.layout.dashboardCollapsed[group] ?? false;
          return (
            <section className="dashboard-section" key={group}>
              <button
                className="dashboard-section-heading"
                aria-label={`${group}, ${list.length} workspaces`}
                aria-expanded={!collapsed}
                onClick={() =>
                  deck.setLayout((l) => ({
                    ...l,
                    dashboardCollapsed: {
                      ...l.dashboardCollapsed,
                      [group]: !collapsed,
                    },
                  }))
                }
              >
                {collapsed ? (
                  <ChevronRight size={14} />
                ) : (
                  <ChevronDown size={14} />
                )}
                <strong>{group}</strong>
                <span>{list.length}</span>
              </button>
              {!collapsed &&
                (list.length ? (
                  list.map((w) => (
                    <WorkspaceRow
                      key={w.id}
                      row={w}
                      onClick={() =>
                        deck.select(
                          w.id,
                          w.pendingPermissions ? "Agent" : "Overview",
                          w.pendingPermissions
                            ? deck.boot?.permissions.find(
                                (p) => p.workspaceId === w.id,
                              )?.sessionId
                            : undefined,
                        )
                      }
                    />
                  ))
                ) : (
                  <p className="dashboard-section-empty">
                    {group === "Needs attention"
                      ? "Nothing needs your attention."
                      : group === "Active work"
                        ? "No agents or tickets are active."
                        : group === "Ready"
                          ? "Ready workspaces will appear here."
                          : "No waiting workspaces."}
                  </p>
                ))}
            </section>
          );
        })
      )}
      <p className="dashboard-shortcuts">
        ↑ ↓ to navigate · Enter to open · ⌘ / Ctrl + K for commands
      </p>
    </div>
  );
}
function WorkspaceRow({
  row: w,
  onClick,
}: {
  row: DashboardRow;
  onClick: () => void;
}) {
  return (
    <button className="dashboard-row" onClick={onClick}>
      <div className="dashboard-ticket">
        <span className="ticket-key">{w.externalKey || "LOCAL"}</span>
        <strong>{w.title}</strong>
        <div className="dashboard-row-meta">
          {w.branch && (
            <span>
              <GitBranch size={11} />
              {w.branch}
            </span>
          )}
          {w.pr && (
            <span>
              <GitPullRequest size={11} />#{w.pr.number} ·{" "}
              {w.pr.reviewCommentCount} comments
            </span>
          )}
        </div>
      </div>
      <div className="dashboard-state">
        <AttentionBadge value={w.attention} />
        <span>
          {w.tasksCompleted} / {w.tasksTotal} tasks
        </span>
      </div>
      <div className="dashboard-agents">
        {w.sessions
          .filter(
            (s) =>
              s.status === "running" ||
              s.status === "starting" ||
              s.status === "waiting_permission",
          )
          .slice(0, 2)
          .map((s) => (
            <span key={s.id}>
              <i
                className={`dot ${s.status === "waiting_permission" ? "yellow" : "purple"}`}
              />
              {s.provider}
              <span>{s.status.replace("_", " ")}</span>
            </span>
          ))}
        {!w.sessions.some((s) =>
          ["running", "starting", "waiting_permission"].includes(s.status),
        ) && (
          <span className="muted">
            {w.workflowWaiting
              ? "Workflow needs review"
              : w.workflowFailed
                ? "Workflow failed"
                : w.sessions.length
                  ? w.sessions.at(-1)?.provider +
                    " · " +
                    w.sessions.at(-1)?.status
                  : "No agent sessions"}
          </span>
        )}
      </div>
      <time>{timeAgo(w.lastActivity)}</time>
      <ChevronRight size={14} />
    </button>
  );
}
