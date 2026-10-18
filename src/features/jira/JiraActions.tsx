import { useEffect, useState } from "react";
import { rpc, message } from "../../services/api";
import { Modal, ErrorBox } from "../../components/ui";
import type { Issue } from "../../types";
type Transition = { id: string; name: string; status: string };
export function JiraActions({
  id,
  issue,
  refresh,
  disabled,
  intent,
  onHandled,
}: {
  id: string;
  issue: Issue;
  refresh: () => Promise<void>;
  disabled: boolean;
  intent?: string;
  onHandled?: () => void;
}) {
  const [action, setAction] = useState("");
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [transition, setTransition] = useState("");
  const [body, setBody] = useState("");
  const [title, setTitle] = useState(issue.title);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const open = async (kind: string, wanted?: string) => {
    setAction(kind);
    setError("");
    setBody("");
    if (kind === "transition") {
      setBusy(true);
      try {
        const rows = await rpc<Transition[]>("jira.transitions", { id });
        setTransitions(rows);
        const match = rows.find(
          (t) => t.status.toLowerCase() === wanted?.toLowerCase(),
        );
        setTransition(match?.id ?? rows[0]?.id ?? "");
        if (wanted && !match)
          setError(
            `No transition to ${wanted} is currently available. Choose an available transition.`,
          );
      } catch (e) {
        setError(message(e));
      } finally {
        setBusy(false);
      }
    }
  };
  useEffect(() => {
    if (intent) {
      void open("transition", intent);
      onHandled?.();
    }
  }, [intent]);
  return (
    <>
      <div className="integration-actions">
        <button disabled={disabled} onClick={() => void open("transition")}>
          Change status
        </button>
        <button disabled={disabled} onClick={() => void open("comment")}>
          Add comment
        </button>
        <button
          disabled={disabled}
          onClick={() => {
            setTitle(issue.title);
            void open("title");
          }}
        >
          Edit summary
        </button>
      </div>
      {action && (
        <Modal
          title={`Jira · ${action === "transition" ? "Change status" : action === "comment" ? "Add comment" : "Edit summary"}`}
          onClose={() => setAction("")}
        >
          <form
            className="form-body"
            onSubmit={(e) => {
              e.preventDefault();
              setBusy(true);
              setError("");
              void rpc<{ warning?: string } | null>("jira.write", {
                id,
                action,
                confirm: true,
                transitionId: transition,
                body,
                title,
              })
                .then(async (result) => {
                  await refresh();
                  if (result?.warning) setError(result.warning);
                  else setAction("");
                })
                .catch((e) => setError(message(e)))
                .finally(() => setBusy(false));
            }}
          >
            <p>
              {issue.key} · {issue.title}
            </p>
            {action === "transition" ? (
              <label>
                Available transition
                <select
                  value={transition}
                  onChange={(e) => setTransition(e.target.value)}
                >
                  {transitions.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name} → {t.status}
                    </option>
                  ))}
                </select>
              </label>
            ) : action === "comment" ? (
              <>
                <label>
                  Comment
                  <textarea
                    rows={7}
                    required
                    maxLength={32000}
                    value={body}
                    onChange={(e) => setBody(e.target.value)}
                  />
                </label>
                <div className="quick-comments">
                  <button
                    type="button"
                    onClick={() =>
                      setBody("Implementation complete, awaiting review.")
                    }
                  >
                    Awaiting review
                  </button>
                  <button type="button" onClick={() => setBody("Blocked by ")}>
                    Blocked by…
                  </button>
                </div>
              </>
            ) : (
              <label>
                Summary
                <input
                  required
                  maxLength={255}
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                />
              </label>
            )}
            {error && <ErrorBox error={error} />}
            <div className="form-actions">
              <button type="button" onClick={() => setAction("")}>
                Cancel
              </button>
              <button
                className="primary"
                disabled={busy || (action === "transition" && !transition)}
              >
                Confirm Jira update
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
