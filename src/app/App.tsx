import { Dashboard } from "../features/dashboard/Dashboard";
import { PermissionQueue } from "../features/permissions/PermissionQueue";
import {
  WorkspaceContextDialog,
  SessionContextDialog,
} from "../features/context/ContextControls";
import { Bell } from "lucide-react";
import type { Purpose } from "../types";
import {
  Bot,
  Check,
  GitBranch,
  Layers3,
  Plus,
  Search,
  Ticket,
  WifiOff,
} from "lucide-react";
import { useEffect, useState } from "react";
import { ResizeHandle } from "../components/ResizeHandle";
import { Empty, ErrorBox, Modal } from "../components/ui";
import {
  CommandPalette,
  type Command,
} from "../features/workspace/CommandPalette";
import { SettingsDialog } from "../features/workspace/SettingsDialog";
import { WorkspaceForm } from "../features/workspace/WorkspaceForm";
import { WorkspaceSidebar } from "../features/workspace/WorkspaceSidebar";
import { WorkspaceView } from "../features/workspace/WorkspaceView";
import { tabs } from "../features/workspace/tabs";
import { useDeck } from "../hooks/useDeck";
import { rpc } from "../services/api";
import type { Tab } from "../types";

export function App() {
  const deck = useDeck();
  const { boot, detail, layout, setLayout } = deck;
  const [palette, setPalette] = useState(false);
  const [create, setCreate] = useState(false);
  const [edit, setEdit] = useState(false);
  const [settings, setSettings] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [providerRequest, setProviderRequest] = useState<string>();
  const [requestedPurpose, setRequestedPurpose] = useState<Purpose>();
  const [contextPreview, setContextPreview] = useState(false);
  const [sessionContext, setSessionContext] = useState(false);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const goDashboard = () => setLayout((l) => ({ ...l, view: "dashboard" }));
  const w = detail?.workspace;
  const tab = layout.selected
    ? (layout.tabs[layout.selected] ?? "Overview")
    : "Overview";
  const setTab = (value: Tab) => {
    if (layout.selected)
      setLayout((l) => ({
        ...l,
        view: "workspace",
        tabs: { ...l.tabs, [l.selected!]: value },
      }));
  };
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      const key = e.key.toLowerCase();
      if (key === "k") {
        e.preventDefault();
        setPalette((v) => !v);
      }
      if (key === "n" && !e.shiftKey) {
        e.preventDefault();
        setCreate(true);
      }
      if (key === "1" && !e.shiftKey) {
        e.preventDefault();
        goDashboard();
      }
      if (e.shiftKey && key === "p") {
        e.preventDefault();
        deck.setPermissionsOpen(true);
      }
      if (e.shiftKey && key === "r") {
        e.preventDefault();
        refreshAll();
      }
      if (e.shiftKey && key === "a") {
        e.preventDefault();
        start(boot?.providers.find((p) => p.available)?.id ?? "codex");
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });
  useEffect(() => {
    setProviderRequest(undefined);
  }, [layout.selected]);
  const start = (provider: string, purpose?: Purpose) => {
    setRequestedPurpose(purpose);
    setTab("Agent");
    setProviderRequest(provider);
  };
  const refreshAll = () => {
    void deck.refresh();
    if (w) {
      if (w.repoPath) void deck.sync(w.id, "pr");
      if (w.jiraTicketId) void deck.sync(w.id, "jira");
    }
  };
  const commands: Command[] = [
    {
      id: "dashboard",
      label: "Go to Dashboard",
      group: "Navigate",
      hint: "⌘ 1",
      run: goDashboard,
    },
    {
      id: "permissions",
      label: "Show pending permissions",
      group: "Permissions",
      run: () => deck.setPermissionsOpen(true),
    },
    {
      id: "approve",
      label: "Approve current permission…",
      group: "Permissions",
      run: () => deck.setPermissionsOpen(true),
    },
    ...(boot?.workspaces
      .filter((x) => !x.archived)
      .map((x) => ({
        id: `switch-${x.id}`,
        group: "Switch workspace",
        label: `${x.externalKey || "LOCAL"} · ${x.title}`,
        searchText: x.searchText,
        run: () => deck.select(x.id),
      })) ?? []),
    {
      id: "create",
      group: "Workspace",
      label: "Create workspace",
      hint: "⌘ N",
      run: () => setCreate(true),
    },
    ...(w
      ? [
          {
            id: "refresh",
            group: "Workspace",
            label: "Refresh workspace",
            run: refreshAll,
          },
          {
            id: "context",
            group: "Context",
            label: "Preview workspace context",
            run: () => setContextPreview(true),
          },
          {
            id: "refresh-context",
            group: "Context",
            label: "Refresh context",
            run: () => {
              if (detail?.sessions.length) setSessionContext(true);
              else setContextPreview(true);
            },
          },
          ...(["investigation", "implementation", "testing"] as const).map(
            (p) => ({
              id: `purpose-${p}`,
              group: "Agents",
              label: `Start ${p === "testing" ? "test" : p} session`,
              run: () =>
                start(
                  boot?.providers.find((x) => x.available)?.id ?? "codex",
                  p,
                ),
            }),
          ),
          ...tabs.map((t) => ({
            id: `tab-${t.name}`,
            group: "Navigate",
            label: `Open ${t.name}`,
            run: () => setTab(t.name),
          })),
          ...["codex", "claude", "opencode"].map((p) => ({
            id: `start-${p}`,
            group: "Agents",
            label: `Start ${boot?.providers.find((x) => x.id === p)?.name ?? p} session`,
            run: () => start(p),
          })),
        ]
      : []),
    {
      id: "settings",
      group: "Application",
      label: "Settings & integrations",
      run: () => setSettings(true),
    },
  ];
  return (
    <div className="app-shell">
      <header className="app-titlebar">
        <div className="brand">
          <div className="brand-mark">
            <Layers3 size={20} strokeWidth={1.8} />
          </div>
          <strong>CommitDeck</strong>
          <span className="version">ALPHA</span>
        </div>
        <div className="top-center">
          <span className="local-indicator">
            <i className="dot green" />
            Local workspace
          </span>
        </div>
        <button className="global-search" onClick={() => setPalette(true)}>
          <Search size={14} />
          <span>Search or jump to…</span>
          <kbd>⌘ K</kbd>
        </button>
        <button
          className="notification-button"
          aria-label={`Notifications: ${boot?.notifications.filter((n) => !n.read).length ?? 0} unread`}
          onClick={() => setNotificationsOpen(true)}
        >
          <Bell size={16} />
          {!!boot?.notifications.some((n) => !n.read) && (
            <i className="dot purple" />
          )}
        </button>
        <button className="new-ticket" onClick={() => setCreate(true)}>
          <Plus size={15} />
          New ticket
        </button>
      </header>
      <div className="app-body">
        <WorkspaceSidebar
          deck={deck}
          onCreate={() => setCreate(true)}
          onSettings={() => setSettings(true)}
          onPalette={() => setPalette(true)}
        />
        <ResizeHandle
          label="Resize workspace sidebar"
          value={layout.sidebarWidth}
          min={210}
          max={380}
          onChange={(v) => setLayout((l) => ({ ...l, sidebarWidth: v }))}
        />
        <main className="workspace-main">
          {deck.offline && (
            <div className="offline-banner">
              <WifiOff size={14} />
              Local backend disconnected. Your unsaved notes are kept on this
              device.
              <button onClick={() => window.location.reload()}>
                Reconnect
              </button>
            </div>
          )}
          {deck.error && (
            <div className="global-error">
              <ErrorBox error={deck.error} />
              <button
                className="icon-button"
                aria-label="Dismiss error"
                onClick={() => deck.setError("")}
              >
                ×
              </button>
            </div>
          )}
          {!boot ? (
            <div className="welcome">
              <div className="brand-mark large">
                <Layers3 size={34} />
              </div>
              <h1>
                {deck.error
                  ? "Unable to open CommitDeck"
                  : "Opening your workspace…"}
              </h1>
              {deck.error && (
                <button onClick={() => window.location.reload()}>
                  Try again
                </button>
              )}
            </div>
          ) : layout.view === "dashboard" ? (
            <Dashboard deck={deck} onCreate={() => setCreate(true)} />
          ) : !layout.selected ? (
            <div className="welcome">
              <div className="welcome-kicker">
                <span className="dot green" />
                YOUR DEVELOPMENT, IN CONTEXT
              </div>
              <div className="brand-mark large">
                <Layers3 size={34} />
              </div>
              <h1>
                More tickets.
                <br />
                <span>Less context switching.</span>
              </h1>
              <p>
                One workspace for the issue, the code, and the agents.
                <br />
                Pick up exactly where you left off.
              </p>
              <button className="primary" onClick={() => setCreate(true)}>
                <Plus size={16} />
                Create your first workspace
              </button>
              <div className="welcome-steps">
                <div>
                  <span>01</span>
                  <Ticket size={18} />
                  <strong>Bring your ticket</strong>
                  <p>Start locally or import from Jira.</p>
                </div>
                <div>
                  <span>02</span>
                  <GitBranch size={18} />
                  <strong>Isolate the work</strong>
                  <p>A branch and worktree for every task.</p>
                </div>
                <div>
                  <span>03</span>
                  <Bot size={18} />
                  <strong>Put agents to work</strong>
                  <p>Run sessions in parallel. Stay in control.</p>
                </div>
              </div>
            </div>
          ) : !detail ? (
            <Empty
              title={
                deck.loadError ? "Workspace unavailable" : "Opening ticket…"
              }
            >
              {deck.loadError ? (
                <ErrorBox
                  error={deck.loadError}
                  retry={() => void deck.load(layout.selected!)}
                />
              ) : (
                <p>Restoring your context.</p>
              )}
            </Empty>
          ) : (
            <>
              <WorkspaceView
                deck={deck}
                tab={tab}
                setTab={setTab}
                providerRequest={providerRequest}
                requestedPurpose={requestedPurpose}
                setProviderRequest={setProviderRequest}
                onEdit={() => setEdit(true)}
                onDelete={() => setDeleteConfirm(true)}
                onSettings={() => setSettings(true)}
                onRefresh={refreshAll}
              />
            </>
          )}
        </main>
      </div>
      <footer className="statusbar">
        <span>
          <i className={`dot ${deck.offline ? "red" : "green"}`} />
          {deck.offline ? "Backend offline" : "Local workspace"}
        </span>
        <span>
          {boot?.workspaces.filter((x) => !x.archived).length ?? 0} workspaces
        </span>
        <span>{boot?.activeAgents ?? 0} agents running</span>
        <span className="statusbar-right">
          <Check size={12} />
          {w?.externalKey || "CommitDeck"}
          <span>v0.1.0</span>
        </span>
      </footer>
      {deck.permissionsOpen && (
        <Modal
          wide
          title="Pending permissions"
          onClose={() => deck.setPermissionsOpen(false)}
        >
          <div className="form-body">
            <PermissionQueue
              requests={boot?.permissions ?? []}
              workspaceNames={Object.fromEntries(
                (boot?.workspaces ?? []).map((w) => [
                  w.id,
                  `${w.externalKey} · ${w.title}`,
                ]),
              )}
              refresh={deck.refresh}
              onOpenWorkspace={(r) => {
                deck.select(r.workspaceId, "Agent", r.sessionId);
                deck.setPermissionsOpen(false);
              }}
            />
          </div>
        </Modal>
      )}
      {contextPreview && w && (
        <WorkspaceContextDialog
          workspaceId={w.id}
          onClose={() => setContextPreview(false)}
        />
      )}
      {sessionContext && detail?.sessions.length && (
        <SessionContextDialog
          session={
            detail.sessions.find(
              (s) => s.id === layout.sessions[detail.workspace.id],
            ) ?? detail.sessions[0]
          }
          refresh={deck.refresh}
          onClose={() => setSessionContext(false)}
        />
      )}
      {notificationsOpen && (
        <Modal
          title="Notifications"
          onClose={() => setNotificationsOpen(false)}
        >
          <div className="form-body notification-list">
            <button
              onClick={() =>
                void rpc("notifications.read")
                  .then(deck.refresh)
                  .catch(deck.report)
              }
            >
              Mark all as read
            </button>
            {!boot?.notifications.length && (
              <p className="muted">
                Important agent, permission, CI, and review transitions will
                appear here.
              </p>
            )}
            {boot?.notifications.map((n) => (
              <button
                key={n.id}
                className={`notification-row ${n.read ? "read" : ""}`}
                onClick={() => {
                  deck.select(n.workspaceId, "Overview");
                  setNotificationsOpen(false);
                }}
              >
                <i
                  className={`dot ${n.read ? "muted" : n.type.includes("failed") ? "red" : "purple"}`}
                />
                <span>
                  {n.title}
                  <small>{new Date(n.createdAt).toLocaleString()}</small>
                </span>
              </button>
            ))}
          </div>
        </Modal>
      )}
      {palette && (
        <CommandPalette commands={commands} onClose={() => setPalette(false)} />
      )}
      {create && (
        <WorkspaceForm
          project={boot?.settings.jiraProject}
          onClose={() => setCreate(false)}
          onCreated={deck.created}
        />
      )}
      {edit && w && (
        <WorkspaceForm
          workspace={w}
          onClose={() => setEdit(false)}
          onCreated={deck.created}
        />
      )}
      {settings && boot && (
        <SettingsDialog
          boot={boot}
          onClose={() => setSettings(false)}
          refresh={deck.refresh}
        />
      )}
      {deleteConfirm && w && (
        <Modal
          title="Delete workspace?"
          onClose={() => setDeleteConfirm(false)}
        >
          <div className="form-body">
            <p>
              Delete{" "}
              <strong>
                {w.externalKey} · {w.title}
              </strong>{" "}
              and its notes, tasks, cached integrations, and session history?
              This cannot be undone.
            </p>
            <p className="muted">
              The Git repository, worktree, branches, commits, and provider CLI
              history are retained on disk.
            </p>
            <div className="form-actions">
              <button onClick={() => setDeleteConfirm(false)}>
                Keep workspace
              </button>
              <button
                className="danger"
                onClick={() => {
                  void rpc("workspace.delete", { id: w.id, confirm: true })
                    .then(async () => {
                      setDeleteConfirm(false);
                      setLayout((l) => ({
                        ...l,
                        selected: undefined,
                        view: "dashboard",
                      }));
                      await deck.refresh();
                    })
                    .catch(deck.report);
                }}
              >
                Delete workspace
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
