import { useEffect, useState } from "react";
import { Download, RefreshCw } from "lucide-react";
import { Modal, ErrorBox, timeAgo } from "../../components/ui";
import { rpc, message } from "../../services/api";
import type { DiscoveredSession, Workspace, Session } from "../../types";
export function ImportSessions({
  workspaces,
  selected,
  onClose,
  onImported,
}: {
  workspaces: Workspace[];
  selected?: string;
  onClose: () => void;
  onImported: (s: Session) => void;
}) {
  const [sessions, setSessions] = useState<DiscoveredSession[]>([]);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState<DiscoveredSession>();
  const [target, setTarget] = useState(selected ?? "");
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await rpc<{
        sessions: DiscoveredSession[];
        warnings: string[];
      }>("discovery.list");
      setSessions(result.sessions);
      setWarnings(result.warnings);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  return (
    <Modal wide title="Import agent session" onClose={onClose}>
      <div className="form-body">
        {error && <ErrorBox error={error} />}
        <p className="muted">
          Attach native history to a workspace. External processes are not
          adopted; send a new prompt here only after the external session has
          stopped.
        </p>
        <div className="inline-form">
          <input
            aria-label="Search discovered sessions"
            placeholder="Filter title, provider, or directory…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button disabled={busy} onClick={() => void load()}>
            <RefreshCw size={14} />
            {busy ? "Discovering…" : "Refresh"}
          </button>
        </div>
        {warnings.map((w) => (
          <p className="context-warning" key={w}>
            {w}
          </p>
        ))}
        {!sessions.length && !busy && (
          <p>
            No supported sessions found. Provider history stays on this machine.
          </p>
        )}
        <div className="import-list">
          {sessions
            .filter((s) =>
              `${s.title} ${s.provider} ${s.cwd}`
                .toLowerCase()
                .includes(query.toLowerCase()),
            )
            .map((s) => (
              <button
                key={s.provider + s.externalSessionId}
                className={`import-row ${chosen === s ? "active" : ""}`}
                disabled={s.imported}
                onClick={() => {
                  setChosen(s);
                  setTarget(s.suggestedWorkspaceId || selected || "");
                }}
              >
                <span
                  className={`provider-symbol small provider-${s.provider}`}
                >
                  ◈
                </span>
                <div>
                  <strong>{s.title}</strong>
                  <code>{s.cwd || "Directory unavailable"}</code>
                  <small>
                    {s.provider} · {s.branch || "Branch unavailable"} ·{" "}
                    {timeAgo(s.updatedAt)} ·{" "}
                    {s.imported
                      ? "Already attached"
                      : s.resumeSupported
                        ? "Native resume"
                        : "Read only"}
                  </small>
                  {s.suggestedWorkspaceId && (
                    <small>
                      Suggested:{" "}
                      {workspaces.find((w) => w.id === s.suggestedWorkspaceId)
                        ?.externalKey || "Local workspace"}{" "}
                      · {s.confidence} confidence
                    </small>
                  )}
                </div>
              </button>
            ))}
        </div>
        {chosen && (
          <>
            <label>
              Attach to workspace
              <select
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              >
                <option value="">Choose workspace</option>
                {workspaces
                  .filter((w) => !w.archived)
                  .map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.externalKey} · {w.title}
                    </option>
                  ))}
              </select>
            </label>
            <p className="hint">
              Source: {chosen.source}. Native history can reference the original
              directory. New processes start in the selected workspace’s
              worktree.
            </p>
            <button
              className="primary"
              disabled={busy || !target}
              onClick={() => {
                setBusy(true);
                void rpc<Session>("discovery.import", {
                  provider: chosen.provider,
                  externalSessionId: chosen.externalSessionId,
                  workspaceId: target,
                })
                  .then(onImported)
                  .catch((e) => setError(message(e)))
                  .finally(() => setBusy(false));
              }}
            >
              <Download size={14} />
              Attach this session
            </button>
          </>
        )}
      </div>
    </Modal>
  );
}
