import { ArrowRight, Plus, Ticket } from "lucide-react";
import { useState } from "react";
import { ErrorBox, Modal } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Issue, Workspace } from "../../types";
export function WorkspaceForm({
  workspace,
  project,
  onClose,
  onCreated,
}: {
  workspace?: Workspace;
  project?: string;
  onClose: () => void;
  onCreated: (w: Workspace) => Promise<void>;
}) {
  const [mode, setMode] = useState<"manual" | "jira">("manual");
  const [key, setKey] = useState(workspace?.externalKey ?? "");
  const [title, setTitle] = useState(workspace?.title ?? "");
  const [status, setStatus] = useState(workspace?.status ?? "In Progress");
  const [jiraKey, setJiraKey] = useState(project ? `${project}-` : "");
  const [issue, setIssue] = useState<Issue>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const w =
        mode === "jira"
          ? await rpc<Workspace>("jira.import", { key: issue?.key })
          : await rpc<Workspace>(
              workspace ? "workspace.update" : "workspace.create",
              { id: workspace?.id, externalKey: key, title, status },
            );
      await onCreated(w);
      onClose();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      title={workspace ? "Edit workspace" : "Create a workspace"}
      onClose={onClose}
    >
      <div className="form-body">
        {!workspace && (
          <div className="mode-switch">
            <button
              className={mode === "manual" ? "active" : ""}
              onClick={() => {
                setMode("manual");
                setError("");
              }}
            >
              <Plus size={15} />
              Manual
            </button>
            <button
              className={mode === "jira" ? "active" : ""}
              onClick={() => {
                setMode("jira");
                setError("");
              }}
            >
              <Ticket size={15} />
              Import from Jira
            </button>
          </div>
        )}
        {mode === "manual" ? (
          <form
            id="workspace-form"
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
          >
            <label>
              Ticket key <span className="muted">optional</span>
              <input
                autoFocus
                value={key}
                onChange={(e) => setKey(e.target.value)}
                maxLength={100}
                placeholder="PROJ-421"
              />
            </label>
            <label>
              Title
              <input
                required
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={500}
                placeholder="Fix invoice retry handling"
              />
            </label>
            <label>
              Status
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value)}
              >
                {[
                  ...new Set([
                    "Backlog",
                    "Blocked",
                    "To Do",
                    "In Progress",
                    "In Review",
                    "Done",
                    status,
                  ]),
                ].map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </select>
            </label>
          </form>
        ) : (
          <>
            <p className="muted">
              Pull in the issue, comments, and context. Jira stays read only.
            </p>
            <form
              className="inline-form"
              onSubmit={(e) => {
                e.preventDefault();
                setBusy(true);
                setError("");
                setIssue(undefined);
                void rpc<Issue>("jira.fetch", {
                  key: jiraKey.trim().toUpperCase(),
                })
                  .then(setIssue)
                  .catch((e) => setError(message(e)))
                  .finally(() => setBusy(false));
              }}
            >
              <input
                aria-label="Jira issue key"
                placeholder="PROJ-421"
                value={jiraKey}
                onChange={(e) => {
                  setJiraKey(e.target.value);
                  setIssue(undefined);
                }}
                required
              />
              <button disabled={busy}>
                Fetch <ArrowRight size={14} />
              </button>
            </form>
            {issue && (
              <div className="import-preview">
                <span className="eyebrow">{issue.key}</span>
                <h3>{issue.title}</h3>
                <dl>
                  <div>
                    <dt>Status</dt>
                    <dd>{issue.status}</dd>
                  </div>
                  <div>
                    <dt>Priority</dt>
                    <dd>{issue.priority || "—"}</dd>
                  </div>
                  <div>
                    <dt>Assignee</dt>
                    <dd>{issue.assignee || "Unassigned"}</dd>
                  </div>
                </dl>
                <p>
                  {issue.comments.length} comments · {issue.labels.length}{" "}
                  labels
                </p>
              </div>
            )}
          </>
        )}
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          <button disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button
            className="primary"
            type={mode === "manual" ? "submit" : "button"}
            form={mode === "manual" ? "workspace-form" : undefined}
            disabled={busy || (mode === "jira" ? !issue : !title.trim())}
            onClick={mode === "jira" ? () => void submit() : undefined}
          >
            {busy
              ? "Working…"
              : workspace
                ? "Update workspace"
                : "Create workspace"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
