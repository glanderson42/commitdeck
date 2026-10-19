import { useState, useEffect } from "react";
import { Modal, ErrorBox } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Workspace } from "../../types";
export function StartWork({
  workspace: w,
  onClose,
  onStarted,
}: {
  workspace: Workspace;
  onClose: () => void;
  onStarted: (terminal: boolean, jira: boolean) => Promise<void>;
}) {
  const [branch, setBranch] = useState("");
  const [base, setBase] = useState("HEAD");
  const [fetch, setFetch] = useState(false);
  const [terminal, setTerminal] = useState(true);
  const [jira, setJira] = useState(false);
  const [preview, setPreview] = useState<{
    branch: string;
    base: string;
    worktree: string;
  }>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    setPreview(undefined);
    const t = setTimeout(
      () =>
        void rpc<typeof preview>("lifecycle.preview", {
          id: w.id,
          branch,
          base,
          fetch,
        })
          .then((v) => {
            if (live) {
              setPreview(v);
              setError("");
            }
          })
          .catch((e) => {
            if (live) setError(message(e));
          }),
      200,
    );
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [w.id, branch, base, fetch]);
  return (
    <Modal title="Start work" wide onClose={onClose}>
      <form
        className="form-body"
        onSubmit={(e) => {
          e.preventDefault();
          if (!preview) return;
          setBusy(true);
          void rpc("lifecycle.start", {
            id: w.id,
            branch: preview.branch,
            base: preview.base,
            fetch,
            confirm: true,
          })
            .then(() => onStarted(terminal, jira))
            .catch((e) => setError(message(e)))
            .finally(() => setBusy(false));
        }}
      >
        <p>
          {w.externalKey} · {w.title}
        </p>
        <label>
          Ticket branch
          <input
            value={branch}
            placeholder={preview?.branch || "Generate from ticket"}
            onChange={(e) => setBranch(e.target.value)}
          />
        </label>
        <label>
          Base revision
          <input
            required
            value={base}
            onChange={(e) => setBase(e.target.value)}
          />
        </label>
        <label className="check-label">
          <input
            type="checkbox"
            checked={fetch}
            onChange={(e) => setFetch(e.target.checked)}
          />
          Fetch origin before creating the worktree
        </label>
        {preview && (
          <div className="operation-preview">
            <ol>
              {fetch && <li>Fetch origin</li>}
              <li>
                Create or reuse <code>{preview.branch}</code> from{" "}
                <code>{preview.base}</code>
              </li>
              <li>
                Create isolated worktree <code>{preview.worktree}</code>
              </li>
              <li>Move local lifecycle to CODING</li>
            </ol>
          </div>
        )}
        <label className="check-label">
          <input
            type="checkbox"
            checked={terminal}
            onChange={(e) => setTerminal(e.target.checked)}
          />
          Open an integrated terminal afterward
        </label>
        {w.jiraTicketId && (
          <label className="check-label">
            <input
              type="checkbox"
              checked={jira}
              onChange={(e) => setJira(e.target.checked)}
            />
            Choose a Jira transition afterward (requires confirmation)
          </label>
        )}
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className="primary"
            disabled={busy || !preview || w.archived || !w.repoPath}
          >
            {busy ? "Starting…" : "Confirm Start Work"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
