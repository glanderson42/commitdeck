import { useState } from "react";
import { ShieldQuestion, AlertTriangle, Check, X, Trash2 } from "lucide-react";
import { ErrorBox, Modal, timeAgo } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { PermissionRequest, PermissionRule } from "../../types";
export function PermissionQueue({
  requests,
  refresh,
  onOpenWorkspace,
  workspaceNames,
}: {
  requests: PermissionRequest[];
  workspaceNames?: Record<string, string>;
  refresh: () => Promise<void>;
  onOpenWorkspace?: (r: PermissionRequest) => void;
}) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [confirm, setConfirm] = useState<{
    request: PermissionRequest;
    scope: string;
  }>();
  const decide = async (
    r: PermissionRequest,
    allow: boolean,
    scope = "once",
  ) => {
    setBusy(r.id);
    setError("");
    try {
      await rpc("permissions.resolve", { id: r.id, allow, scope });
      setConfirm(undefined);
      await refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  };
  return (
    <div className="permission-queue">
      {error && <ErrorBox error={error} />}{" "}
      {!requests.length && (
        <p className="muted">No pending permission requests.</p>
      )}
      {requests.map((r) => (
        <article
          className={`permission-card ${r.highRisk ? "individual" : ""}`}
          key={r.id}
          id={`permission-${r.id}`}
        >
          <header>
            <ShieldQuestion size={16} />
            <strong>
              {r.provider} · {r.title || r.category.replaceAll("_", " ")}
            </strong>
            <time>{timeAgo(r.createdAt)}</time>
          </header>
          <div className="permission-category">
            {r.category.replaceAll("_", " ")}
            {r.highRisk && (
              <span className="orange">
                <AlertTriangle size={12} />
                Individual approval required
              </span>
            )}
          </div>
          {r.command && <pre>{r.command}</pre>}
          {r.filePath && <code>{r.filePath}</code>}
          {r.description && (
            <p className="permission-description">{r.description}</p>
          )}
          {workspaceNames && (
            <p className="permission-workspace">
              Workspace:{" "}
              <strong>{workspaceNames[r.workspaceId] ?? r.workspaceId}</strong>
            </p>
          )}
          <div className="permission-location">
            Working directory: <code>{r.cwd}</code>
          </div>
          {onOpenWorkspace && (
            <button
              className="ghost small-button"
              onClick={() => onOpenWorkspace(r)}
            >
              Open ticket and session
            </button>
          )}
          {r.status !== "pending" ? (
            <p className="muted">
              {r.status} · The previous approval channel is closed. Resume the
              session to obtain a fresh request.
            </p>
          ) : !r.canRespond ? (
            <p className="muted">
              This provider’s current runtime requires approval in its terminal.
              CommitDeck cannot approve this request.
            </p>
          ) : (
            <div className="permission-actions">
              <button disabled={!!busy} onClick={() => void decide(r, false)}>
                <X size={13} />
                Deny
              </button>
              <button
                className="primary"
                disabled={!!busy}
                onClick={() => void decide(r, true)}
              >
                <Check size={13} />
                Allow once
              </button>
              {!r.highRisk && (
                <>
                  <button
                    disabled={!!busy}
                    onClick={() => setConfirm({ request: r, scope: "session" })}
                  >
                    Allow for session…
                  </button>
                  <button
                    disabled={!!busy}
                    onClick={() =>
                      setConfirm({ request: r, scope: "workspace" })
                    }
                  >
                    Always allow in workspace…
                  </button>
                </>
              )}
            </div>
          )}
        </article>
      ))}
      {confirm && (
        <Modal
          title="Remember this exact command?"
          onClose={() => setConfirm(undefined)}
        >
          <div className="form-body">
            <p>
              Allow this executable, exact arguments, and working directory
              automatically for this {confirm.scope}. Test commands can execute
              code from this repository. Other commands still require approval.
            </p>
            <pre>{confirm.request.command}</pre>
            <code>{confirm.request.cwd}</code>
            <div className="form-actions">
              <button onClick={() => setConfirm(undefined)}>Cancel</button>
              <button
                className="primary"
                disabled={!!busy}
                onClick={() =>
                  void decide(confirm.request, true, confirm.scope)
                }
              >
                Allow exact command
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
export function PermissionRules({
  rules,
  refresh,
}: {
  rules: PermissionRule[];
  refresh: () => Promise<void>;
}) {
  const [error, setError] = useState("");
  return (
    <div className="permission-rules">
      <h3>
        Remembered commands <span className="muted">{rules.length}</span>
      </h3>
      {error && <ErrorBox error={error} />}{" "}
      {!rules.length && (
        <p className="muted">
          No reusable permissions. Sensitive actions always require individual
          approval.
        </p>
      )}
      {rules.map((r) => (
        <div className="permission-rule" key={r.id}>
          <div>
            <code>
              {r.executable} {r.argPrefix.join(" ")}
            </code>
            <p>
              {r.scope} · Exact arguments · {r.cwd}
            </p>
          </div>
          <button
            className="icon-button"
            aria-label={`Revoke ${r.executable} rule`}
            onClick={() =>
              void rpc("permissions.rules.delete", { id: r.id })
                .then(refresh)
                .catch((e) => setError(message(e)))
            }
          >
            <Trash2 size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
