import { NewSessionDialog } from "./NewSessionDialog";
import { SessionContextDialog } from "../context/ContextControls";
import { PermissionQueue } from "../permissions/PermissionQueue";
import type { Purpose } from "../../types";
import {
  AlertCircle,
  ArrowUp,
  Bot,
  Download,
  ChevronDown,
  CircleCheck,
  FileCode2,
  GitBranch,
  Plus,
  ShieldQuestion,
  Square,
  Terminal,
  Wrench,
} from "lucide-react";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { promptDrafts } from "../../store/prompts";
import { Markdown } from "../../components/Markdown";
import { Empty, ErrorBox } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { AgentEvent, Detail, Provider } from "../../types";

function EventCard({ event: e }: { event: AgentEvent }) {
  const icon =
    e.type === "command" ? (
      <Terminal size={14} />
    ) : e.type === "file_changed" || e.type === "diff" ? (
      <FileCode2 size={14} />
    ) : e.type === "error" ? (
      <AlertCircle size={14} />
    ) : e.type === "permission_request" ? (
      <ShieldQuestion size={15} />
    ) : e.type === "complete" ? (
      <CircleCheck size={14} />
    ) : (
      <Wrench size={14} />
    );
  if (e.type === "text")
    return (
      <div className={`agent-message ${e.toolName === "user" ? "user" : ""}`}>
        <div className="message-label">
          {e.toolName === "user" ? "YOU" : "AGENT"}
          <time>
            {new Date(e.createdAt).toLocaleTimeString([], {
              hour: "2-digit",
              minute: "2-digit",
            })}
          </time>
        </div>
        <Markdown>{e.text ?? ""}</Markdown>
      </div>
    );
  return (
    <div className={`agent-event event-${e.type}`}>
      <div className="event-heading">
        {icon}
        <span>
          {e.type === "file_changed"
            ? "File changed"
            : e.type === "permission_request"
              ? "Permission required"
              : e.type === "complete"
                ? "Turn finished"
                : e.type === "command"
                  ? "Command"
                  : e.toolName || e.type}
        </span>
        {e.filePath && <code>{e.filePath}</code>}
      </div>
      {e.command && <pre className="command-text">{e.command}</pre>}
      {e.text && <pre>{e.text}</pre>}
      {e.error && <p>{e.error}</p>}
      {e.diff && <pre className="inline-diff">{e.diff}</pre>}
    </div>
  );
}
export function AgentPane({
  detail,
  providers,
  events,
  selected,
  selectSession,
  loadEvents,
  refresh,
  requestedProvider,
  clearRequest,
  requestedPurpose,
  onImport,
}: {
  detail: Detail;
  providers: Provider[];
  events: Record<string, AgentEvent[]>;
  selected?: string;
  selectSession: (id: string) => void;
  loadEvents: (id: string) => Promise<void>;
  refresh: () => Promise<void>;
  requestedProvider?: string;
  clearRequest: () => void;
  requestedPurpose?: Purpose;
  onImport: () => void;
}) {
  const w = detail.workspace;
  const sessions = detail.sessions;
  const session = sessions.find((s) => s.id === selected) ?? sessions[0];
  const [showNew, setShowNew] = useState(false);
  const [provider, setProvider] = useState("codex");
  const [showContext, setShowContext] = useState(false);
  const prompts = useSyncExternalStore(
    promptDrafts.subscribe,
    promptDrafts.get,
  );
  const setPrompts = promptDrafts.set;
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [visible, setVisible] = useState(200);
  useEffect(() => {
    void promptDrafts.persist();
  }, []);
  const bottom = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  const requestHandled = useRef("");
  useEffect(() => {
    if (requestedProvider && requestHandled.current !== requestedProvider) {
      requestHandled.current = requestedProvider;
      setProvider(requestedProvider);
      setShowNew(true);
      clearRequest();
    }
    if (!requestedProvider) requestHandled.current = "";
  }, [requestedProvider, clearRequest]);
  useEffect(() => {
    if (session) void loadEvents(session.id);
    setVisible(200);
    setError("");
    stick.current = true;
  }, [session?.id, loadEvents]);
  const history = session ? (events[session.id] ?? []) : [];
  useEffect(() => {
    if (stick.current) bottom.current?.scrollIntoView({ block: "end" });
  }, [history.length, session?.id]);
  const active =
    ["starting", "running", "waiting_permission"].includes(
      session?.status ?? "",
    ) && !!session?.pid;
  const prompt = session ? (prompts[session.id] ?? "") : "";
  const send = async () => {
    if (!session || !prompt.trim() || busy || active) return;
    setBusy(true);
    setError("");
    try {
      await rpc("session.send", { id: session.id, prompt });
      setPrompts((p) => ({ ...p, [session.id]: "" }));
      await refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="agent-panel">
      <aside className="session-sidebar">
        <div className="section-label">
          SESSIONS <span>{sessions.length}</span>
          <button
            className="icon-button"
            aria-label="New agent session"
            onClick={() => setShowNew(true)}
          >
            <Plus size={15} />
          </button>
        </div>
        <div className="session-list">
          {sessions.map((s) => (
            <button
              key={s.id}
              className={`session-row ${session?.id === s.id ? "active" : ""}`}
              onClick={() => selectSession(s.id)}
            >
              <div className={`provider-symbol provider-${s.provider}`}>
                {s.provider === "claude"
                  ? "✳"
                  : s.provider === "codex"
                    ? "◈"
                    : "◇"}
              </div>
              <div>
                <strong>{s.title}</strong>
                <span>
                  {providers.find((p) => p.id === s.provider)?.name ??
                    s.provider}
                </span>
              </div>
              <i
                className={`dot ${s.status === "running" ? "purple pulse" : s.status === "waiting_permission" ? "yellow" : s.status === "failed" ? "red" : s.status === "completed" ? "green" : "muted"}`}
              />
            </button>
          ))}
        </div>
        <button className="new-session" onClick={() => setShowNew(true)}>
          <Plus size={14} />
          New session
        </button>
        <button className="new-session" onClick={onImport}>
          <Download size={14} />
          Import session
        </button>
        <div className="session-sidebar-bottom">
          <GitBranch size={13} />
          <span>{w.branch || "No worktree connected"}</span>
        </div>
      </aside>
      <div className="session-main">
        {!session ? (
          <Empty
            icon={<Bot size={28} />}
            title="A focused agent for every task"
          >
            <p>
              Investigate with Claude. Implement with Codex. Test with OpenCode.
              Each session stays with this ticket.
            </p>
            {w.worktreePath ? (
              <button className="primary" onClick={() => setShowNew(true)}>
                <Plus size={15} />
                Start your first session
              </button>
            ) : (
              <p className="hint">
                Open the Git tab to attach a repository and create a worktree.
              </p>
            )}
          </Empty>
        ) : (
          <>
            <div className="panel-toolbar">
              <div>
                <div className="toolbar-title">
                  {session.title}
                  <span
                    className={`status-pill ${session.status === "running" ? "purple" : session.status === "failed" ? "red" : session.status === "waiting_permission" ? "yellow" : "green"}`}
                  >
                    <i className="dot" />
                    {session.status}
                  </span>
                </div>
                {session.imported && (
                  <p className="session-purpose">
                    Imported ·{" "}
                    {session.resumeSupported ? "Native resume" : "Read only"} ·
                    Original directory: {session.sourceCwd || "unavailable"}
                  </p>
                )}
                {session.purpose && (
                  <p className="session-purpose">{session.purpose}</p>
                )}
              </div>
              <span className="muted tiny">
                {session.pid
                  ? `PID ${session.pid}`
                  : providers.find((p) => p.id === session.provider)?.name}
              </span>
            </div>
            <div
              className="event-stream"
              onScroll={(e) => {
                const el = e.currentTarget;
                stick.current =
                  el.scrollHeight - el.scrollTop - el.clientHeight < 100;
              }}
            >
              {history.length > visible && (
                <button
                  className="load-older"
                  onClick={() => setVisible((v) => v + 200)}
                >
                  Show earlier activity <ChevronDown size={14} />
                </button>
              )}
              {history.length === 0 && (
                <div className="session-start">
                  <Bot size={23} />
                  <h3>Ready in your worktree</h3>
                  <p>
                    Give{" "}
                    {providers.find((p) => p.id === session.provider)?.name} a
                    specific task to start working.
                  </p>
                  <code>{w.worktreePath}</code>
                </div>
              )}
              {history.slice(-visible).map((e) => (
                <EventCard key={e.id} event={e} />
              ))}
              {active && (
                <div className="working-indicator">
                  <i className="dot purple pulse" />
                  Agent is working in this ticket’s worktree…
                </div>
              )}
              <PermissionQueue
                requests={detail.permissions.filter(
                  (r) => r.sessionId === session.id && r.status === "pending",
                )}
                refresh={refresh}
              />
              <div ref={bottom} />
            </div>
            <div className="composer-area">
              <button
                className="context-session-action"
                onClick={() => setShowContext(true)}
              >
                View / refresh session context
              </button>
              {error && <ErrorBox error={error} />}
              <form
                className="composer"
                onSubmit={(e) => {
                  e.preventDefault();
                  void send();
                }}
              >
                <textarea
                  aria-label="Agent prompt"
                  placeholder={
                    session.externalSessionId
                      ? "Continue this session…"
                      : "What should the agent work on?"
                  }
                  value={prompt}
                  onChange={(e) =>
                    setPrompts((p) => ({ ...p, [session.id]: e.target.value }))
                  }
                  onKeyDown={(e) => {
                    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                      e.preventDefault();
                      void send();
                    }
                  }}
                  disabled={
                    active ||
                    w.archived ||
                    (!!session.imported && !session.resumeSupported)
                  }
                />
                <p className="hint">
                  Recovery copies redact recognized credentials. Keep secrets
                  out of prompts.
                </p>
                <div className="composer-bottom">
                  <span>
                    <span
                      className={`provider-symbol small provider-${session.provider}`}
                    >
                      ◈
                    </span>
                    {providers.find((p) => p.id === session.provider)?.name}
                    <span className="muted">·</span>
                    <span className="muted">
                      {session.externalSessionId
                        ? "Resume session"
                        : "New conversation"}
                    </span>
                  </span>
                  {active ? (
                    <button
                      type="button"
                      onClick={() =>
                        void rpc("session.cancel", { id: session.id })
                          .then(refresh)
                          .catch((e) => setError(message(e)))
                      }
                    >
                      <Square size={12} />
                      Stop
                    </button>
                  ) : (
                    <button
                      type="submit"
                      className="send-button"
                      disabled={
                        !prompt.trim() ||
                        busy ||
                        w.archived ||
                        (!!session.imported && !session.resumeSupported)
                      }
                      aria-label="Send prompt"
                    >
                      <ArrowUp size={17} />
                    </button>
                  )}
                </div>
              </form>
              <p className="composer-hint">
                ⌘ / Ctrl + Enter to send
                <span>Workspace context included · CLI authentication</span>
              </p>
            </div>
          </>
        )}
      </div>
      {showNew && (
        <NewSessionDialog
          workspace={w}
          providers={providers}
          initialProvider={provider}
          initialPurpose={requestedPurpose}
          onClose={() => setShowNew(false)}
          onCreated={async (s) => {
            await refresh();
            selectSession(s.id);
          }}
        />
      )}
      {showContext && session && (
        <SessionContextDialog
          session={session}
          onClose={() => setShowContext(false)}
          refresh={refresh}
        />
      )}
    </div>
  );
}
