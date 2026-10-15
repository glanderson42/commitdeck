import {
  LayoutDashboard,
  ShieldQuestion,
  Archive,
  CircleHelp,
  Plus,
  Search,
  Settings,
} from "lucide-react";
import { useState } from "react";
import { AttentionBadge, attention } from "../../components/ui";
import type { Deck } from "../../hooks/useDeck";
export function WorkspaceSidebar({
  deck,
  onCreate,
  onSettings,
  onPalette,
}: {
  deck: Deck;
  onCreate: () => void;
  onSettings: () => void;
  onPalette: () => void;
}) {
  const { boot, layout } = deck;
  const [search, setSearch] = useState("");
  const [archived, setArchived] = useState(false);
  const list =
    boot?.workspaces.filter(
      (x) =>
        x.archived === archived &&
        `${x.externalKey} ${x.title} ${attention[x.attention]?.label}`
          .toLowerCase()
          .includes(search.toLowerCase()),
    ) ?? [];
  return (
    <aside className="workspace-sidebar" style={{ width: layout.sidebarWidth }}>
      <button
        className={`sidebar-dashboard ${layout.view === "dashboard" ? "active" : ""}`}
        onClick={() => deck.setLayout((l) => ({ ...l, view: "dashboard" }))}
      >
        <LayoutDashboard size={16} />
        Dashboard<kbd>⌘ 1</kbd>
      </button>
      {!!boot?.permissions.length && (
        <button
          className="sidebar-permissions"
          onClick={() => deck.setPermissionsOpen(true)}
        >
          <ShieldQuestion size={15} />
          {boot.permissions.length} approvals waiting
        </button>
      )}
      <div className="sidebar-heading">
        <span>WORKSPACES</span>
        <button
          className="icon-button"
          aria-label="Create workspace"
          onClick={() => onCreate()}
        >
          <Plus size={16} />
        </button>
      </div>
      <div className="sidebar-filter">
        <Search size={14} />
        <input
          aria-label="Filter workspaces"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter tickets…"
        />
        <span>{list.length}</span>
      </div>
      <div className="sidebar-section">
        <button
          className={!archived ? "active" : ""}
          onClick={() => setArchived(false)}
        >
          My work{" "}
          <span>{boot?.workspaces.filter((x) => !x.archived).length ?? 0}</span>
        </button>
        <button
          className={archived ? "active" : ""}
          onClick={() => setArchived(true)}
          aria-label="Archived workspaces"
        >
          <Archive size={13} />
        </button>
      </div>
      <nav className="workspace-list" aria-label="Ticket workspaces">
        {list.map((x) => (
          <button
            key={x.id}
            className={`workspace-row ${layout.view === "workspace" && layout.selected === x.id ? "selected" : ""}`}
            onClick={() => deck.select(x.id)}
          >
            <div className="ticket-top">
              <i
                className={`dot ${attention[x.attention]?.color ?? "muted"}`}
              />
              <span className="ticket-key">{x.externalKey || "LOCAL"}</span>
              {x.attention === "AGENT_RUNNING" && (
                <span className="running-waves">▥</span>
              )}
            </div>
            <strong>{x.title}</strong>
            <AttentionBadge value={x.attention} />
          </button>
        ))}
        {boot && list.length === 0 && (
          <p className="sidebar-empty">
            {search
              ? "No matching tickets."
              : archived
                ? "No archived workspaces."
                : "Your next piece of work starts here."}
          </p>
        )}
      </nav>
      <div className="sidebar-bottom">
        <button onClick={() => onSettings()}>
          <Settings size={16} />
          Settings & integrations
        </button>
        <button onClick={() => onPalette()}>
          <CircleHelp size={16} />
          Commands & shortcuts<kbd>⌘ K</kbd>
        </button>
        <div className="sidebar-profile">
          <div className="avatar">CD</div>
          <div>
            <strong>Your workspace</strong>
            <span>Local first. Yours always.</span>
          </div>
          <i className={`dot ${deck.offline ? "red" : "green"}`} />
        </div>
      </div>
    </aside>
  );
}
