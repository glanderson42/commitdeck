import { useEffect, useState } from "react";
import { Eye, RefreshCw, AlertTriangle } from "lucide-react";
import type {
  ContextOptions,
  ContextSnapshot,
  GeneratedContext,
  Purpose,
  Session,
} from "../../types";
import { message, rpc } from "../../services/api";
import { notes } from "../../store/notes";
import { Markdown } from "../../components/Markdown";
import { ErrorBox, Modal } from "../../components/ui";
export const defaultContext: ContextOptions = {
  maxCharacters: 12000,
  includeIssue: true,
  includeNotes: true,
  includeTodos: true,
  includeGit: true,
  includePR: true,
  includeSessionSummary: true,
};
export const purposes: Purpose[] = [
  "investigation",
  "implementation",
  "testing",
  "review",
  "debugging",
  "documentation",
  "custom",
];
const sources = [
  ["includeIssue", "Jira ticket"],
  ["includeTodos", "TODOs"],
  ["includeNotes", "Notes"],
  ["includeGit", "Git state"],
  ["includePR", "PR feedback"],
  ["includeSessionSummary", "Previous session summaries"],
] as const;
export function ContextControls({
  workspaceId,
  purpose,
  options,
  onOptions,
  onGenerated,
}: {
  workspaceId: string;
  purpose: Purpose;
  options: ContextOptions;
  onOptions: (o: ContextOptions) => void;
  onGenerated: (g: GeneratedContext | undefined) => void;
}) {
  const [generated, setGenerated] = useState<GeneratedContext>();
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let live = true;
    onGenerated(undefined);
    setGenerated(undefined);
    setError("");
    const timer = setTimeout(() => {
      void (async () => {
        await notes.flush(workspaceId);
        if (notes.get(workspaceId)?.state === "error")
          throw new Error(
            "Save workspace notes successfully before generating context.",
          );
        return rpc<GeneratedContext>("context.build", {
          workspaceId,
          purpose,
          options,
        });
      })()
        .then((g) => {
          if (live) {
            setGenerated(g);
            onGenerated(g);
          }
        })
        .catch((e) => {
          if (live) setError(message(e));
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [workspaceId, purpose, options, onGenerated, revision]);
  return (
    <fieldset className="context-controls">
      <legend>Workspace context</legend>
      <div className="context-source-grid">
        {sources.map(([key, label]) => (
          <label key={key}>
            <input
              type="checkbox"
              checked={options[key]}
              onChange={(e) =>
                onOptions({ ...options, [key]: e.target.checked })
              }
            />
            {label}
          </label>
        ))}
      </div>
      <label className="context-budget">
        Character budget
        <select
          value={options.maxCharacters}
          onChange={(e) =>
            onOptions({ ...options, maxCharacters: Number(e.target.value) })
          }
        >
          {[3000, 6000, 12000, 24000, 48000, 64000].map((n) => (
            <option key={n} value={n}>
              {n.toLocaleString()} characters
            </option>
          ))}
        </select>
      </label>
      <div className="context-preview-toolbar">
        <span>
          {generated
            ? `${generated.characterCount.toLocaleString()} characters · ~${generated.estimatedTokens.toLocaleString()} tokens`
            : "Building context…"}
        </span>
        <button
          type="button"
          onClick={() => setRevision((v) => v + 1)}
          aria-label="Regenerate context preview"
        >
          <RefreshCw size={13} />
        </button>
        <button
          type="button"
          disabled={!generated}
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          <Eye size={13} />
          {open ? "Hide preview" : "Preview exact context"}
        </button>
      </div>
      {error && <ErrorBox error={error} />}{" "}
      {generated?.truncated.length ? (
        <p className="context-warning">
          <AlertTriangle size={13} />
          Truncated: {generated.truncated.join(", ")}. The preview shows the
          exact content that will be sent.
        </p>
      ) : null}
      {generated?.warnings.map((w) => (
        <p className="context-warning" key={w}>
          {w}
        </p>
      ))}
      {open && generated && (
        <div className="context-preview">
          <Markdown>{generated.markdown}</Markdown>
        </div>
      )}
      <p className="hint">
        Only selected sources are included. Known secret patterns are redacted;
        review the preview before sharing sensitive notes.
      </p>
    </fieldset>
  );
}
export function SessionContextDialog({
  session,
  onClose,
  refresh,
}: {
  session: Session;
  onClose: () => void;
  refresh: () => Promise<void>;
}) {
  const [snapshot, setSnapshot] = useState<ContextSnapshot>();
  const [editing, setEditing] = useState(false);
  const [options, setOptions] = useState(defaultContext);
  const [generated, setGenerated] = useState<GeneratedContext>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    void rpc<ContextSnapshot>("session.context", { id: session.id })
      .then((s) => {
        setSnapshot(s);
        setOptions(s.options);
      })
      .catch(() => setEditing(true));
  }, [session.id]);
  return (
    <Modal wide title="Session context" onClose={onClose}>
      <div className="form-body">
        {error && <ErrorBox error={error} />}{" "}
        {!editing && snapshot && (
          <>
            <div className="context-snapshot-meta">
              <span>
                {snapshot.sentAt
                  ? "Sent " + new Date(snapshot.sentAt).toLocaleString()
                  : "Prepared · included with the next prompt"}
              </span>
              <span>{snapshot.characterCount.toLocaleString()} characters</span>
              <code>SHA-256 {snapshot.hash.slice(0, 16)}…</code>
            </div>
            <div className="context-preview">
              <Markdown>{snapshot.markdown}</Markdown>
            </div>
          </>
        )}
        {editing && (
          <ContextControls
            workspaceId={session.workspaceId}
            purpose={
              purposes.includes(session.purpose as Purpose)
                ? (session.purpose as Purpose)
                : "custom"
            }
            options={options}
            onOptions={setOptions}
            onGenerated={setGenerated}
          />
        )}
        <div className="form-actions">
          <button onClick={onClose}>Close</button>
          {!editing ? (
            <button disabled={!!session.pid} onClick={() => setEditing(true)}>
              <RefreshCw size={14} />
              Refresh context
            </button>
          ) : (
            <button
              className="primary"
              disabled={!generated || busy}
              onClick={() => {
                if (!generated) return;
                setBusy(true);
                void rpc<ContextSnapshot>("context.refresh", {
                  id: session.id,
                  options,
                  hash: generated.hash,
                })
                  .then(async (s) => {
                    setSnapshot(s);
                    setError("");
                    setEditing(false);
                    await refresh();
                  })
                  .catch((e) => setError(message(e)))
                  .finally(() => setBusy(false));
              }}
            >
              Use on next prompt
            </button>
          )}
        </div>
        <p className="hint">
          Identical context is not resent on normal resume. Refresh explicitly
          queues a new snapshot for the next prompt.
        </p>
      </div>
    </Modal>
  );
}
export function WorkspaceContextDialog({
  workspaceId,
  onClose,
}: {
  workspaceId: string;
  onClose: () => void;
}) {
  const [options, setOptions] = useState(defaultContext);
  const [purpose, setPurpose] = useState<Purpose>("implementation");
  const [generated, setGenerated] = useState<GeneratedContext>();
  return (
    <Modal wide title="Preview workspace context" onClose={onClose}>
      <div className="form-body">
        <label>
          Session purpose
          <select
            value={purpose}
            onChange={(e) => setPurpose(e.target.value as Purpose)}
          >
            {purposes.map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
        </label>
        <ContextControls
          workspaceId={workspaceId}
          purpose={purpose}
          options={options}
          onOptions={setOptions}
          onGenerated={setGenerated}
        />
        {generated && (
          <div className="context-preview">
            <Markdown>{generated.markdown}</Markdown>
          </div>
        )}
        <div className="form-actions">
          <button onClick={onClose}>Close</button>
        </div>
      </div>
    </Modal>
  );
}
