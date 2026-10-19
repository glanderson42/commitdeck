import {
  Archive,
  ChevronRight,
  GitBranch,
  GitPullRequest,
  Layers3,
  MoreHorizontal,
  PanelRightClose,
  PanelRightOpen,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";
import { AttentionBadge, ExternalLink } from "../../components/ui";
import type { Deck } from "../../hooks/useDeck";
import { rpc } from "../../services/api";
import type { Tab } from "../../types";
import { tabs } from "./tabs";
export function WorkspaceHeader({
  deck,
  tab,
  setTab,
  onEdit,
  onDelete,
  onRefresh,
  onAction,
}: {
  deck: Deck;
  tab: Tab;
  setTab: (tab: Tab) => void;
  onEdit: () => void;
  onDelete: () => void;
  onRefresh: () => void;
  onAction: (s: string) => void;
}) {
  const { detail, layout, setLayout } = deck;
  const [menu, setMenu] = useState(false);
  useEffect(() => setMenu(false), [layout.selected]);
  if (!detail) return null;
  const w = detail.workspace,
    pr = detail.pr?.data,
    completed = detail.todos.filter((t) => t.completed).length,
    context = layout.showContext;
  const setContext = (fn: (v: boolean) => boolean) =>
    setLayout((l) => ({ ...l, showContext: fn(l.showContext) }));
  return (
    <>
      {" "}
      <div className="workspace-heading">
        <div className="breadcrumb">
          <Layers3 size={13} />
          <span>My work</span>
          <ChevronRight size={12} />
          <span>{w.externalKey || "Local workspace"}</span>
          <div className="heading-actions">
            <button
              disabled={w.archived || !w.repoPath || !!w.worktreePath}
              onClick={() => onAction("start-work")}
            >
              Start Work
            </button>
            <button
              title="Show terminal alongside this pane"
              aria-pressed={!!layout.terminalSplit}
              onClick={() =>
                setLayout((l) => ({ ...l, terminalSplit: !l.terminalSplit }))
              }
            >
              Split terminal
            </button>
            <button
              className="icon-button"
              title="Refresh workspace"
              aria-label="Refresh workspace"
              onClick={onRefresh}
            >
              <RefreshCw size={15} />
            </button>
            <button
              className="icon-button"
              aria-label="Toggle context panel"
              onClick={() => setContext((v) => !v)}
            >
              {context ? (
                <PanelRightClose size={16} />
              ) : (
                <PanelRightOpen size={16} />
              )}
            </button>
            <div className="menu-container">
              <button
                className="icon-button"
                aria-label="Workspace actions"
                aria-expanded={menu}
                onClick={() => setMenu((v) => !v)}
              >
                <MoreHorizontal size={18} />
              </button>
              {menu && (
                <>
                  <div
                    className="menu-dismiss"
                    onClick={() => setMenu(false)}
                  />
                  <div className="dropdown-menu">
                    <button
                      disabled={w.archived}
                      onClick={() => {
                        onAction("start-agent");
                        setMenu(false);
                      }}
                    >
                      Start Agent
                    </button>
                    <button
                      onClick={() => {
                        setTab("Terminal");
                        setMenu(false);
                      }}
                    >
                      Open Terminal
                    </button>
                    <button
                      onClick={() => {
                        onAction("import");
                        setMenu(false);
                      }}
                    >
                      Import Session
                    </button>
                    <button
                      onClick={() => {
                        setTab("PR");
                        setMenu(false);
                      }}
                    >
                      Open PR
                    </button>
                    <button
                      onClick={() => {
                        setTab("Jira");
                        setMenu(false);
                      }}
                    >
                      Open Jira
                    </button>
                    <button
                      onClick={() => {
                        onEdit();
                        setMenu(false);
                      }}
                    >
                      <Pencil size={14} />
                      Edit workspace
                    </button>
                    <button
                      onClick={() => {
                        setMenu(false);
                        void rpc("workspace.archive", {
                          id: w.id,
                          archived: !w.archived,
                        })
                          .then(deck.refresh)
                          .catch(deck.report);
                      }}
                    >
                      <Archive size={14} />
                      {w.archived ? "Restore workspace" : "Archive workspace"}
                    </button>
                    <button
                      className="red"
                      onClick={() => {
                        onDelete();
                        setMenu(false);
                      }}
                    >
                      <Trash2 size={14} />
                      Delete workspace
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
        <div className="ticket-title">
          <span>{w.externalKey || "LOCAL"}</span>
          <h1>{w.title}</h1>
        </div>
        <div className="workspace-meta">
          <span className="status-pill blue">
            <i className="dot" />
            {w.status}
          </span>
          <span className="muted tiny" title="Local development lifecycle">
            {w.lifecycle || "BACKLOG"}
          </span>
          {w.archived && <span className="status-pill muted">Archived</span>}
          <span className="metadata-branch" title={w.branch}>
            <GitBranch size={13} />
            {w.branch || "No branch"}
          </span>
          {pr && (
            <>
              <ExternalLink url={pr.url}>
                <GitPullRequest size={13} />
                PR #{pr.number}
              </ExternalLink>
              <span
                className={
                  pr.checks.some((c) => c.status === "failure")
                    ? "red"
                    : pr.checks.some((c) => c.status === "pending")
                      ? "yellow"
                      : pr.checks.length
                        ? "green"
                        : "muted"
                }
              >
                {pr.checks.some((c) => c.status === "failure")
                  ? "×"
                  : pr.checks.some((c) => c.status === "pending")
                    ? "◷"
                    : pr.checks.length
                      ? "✓"
                      : ""}{" "}
                CI{pr.checks.length === 0 ? " —" : ""}
              </span>
              <span>
                <MessageCount count={pr.reviewCommentCount} />
              </span>
            </>
          )}
        </div>
      </div>
      <div
        className="workspace-tabs"
        role="tablist"
        aria-label="Workspace sections"
      >
        {tabs.map((t) => (
          <button
            key={t.name}
            id={`tab-${t.name}`}
            role="tab"
            aria-selected={tab === t.name}
            aria-controls="workspace-tabpanel"
            tabIndex={tab === t.name ? 0 : -1}
            className={tab === t.name ? "active" : ""}
            onClick={() => setTab(t.name)}
            onKeyDown={(e) => {
              if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
                e.preventDefault();
                const i = tabs.findIndex((x) => x.name === t.name);
                const next =
                  tabs[
                    (i + (e.key === "ArrowRight" ? 1 : -1) + tabs.length) %
                      tabs.length
                  ];
                setTab(next.name);
                document.getElementById(`tab-${next.name}`)?.focus();
              }
            }}
          >
            <t.icon size={15} />
            {t.name}
            {t.name === "Tasks" && detail.todos.length > 0 && (
              <span className="tab-count">
                {detail.todos.length - completed}
              </span>
            )}
            {t.name === "Agent" &&
              detail.sessions.some((s) => s.status === "running") && (
                <i className="dot purple pulse" />
              )}
          </button>
        ))}
        <div className="tabs-right">
          <AttentionBadge value={w.attention} />
        </div>
      </div>
    </>
  );
}
function MessageCount({ count }: { count: number }) {
  return <>{count} review comments</>;
}
