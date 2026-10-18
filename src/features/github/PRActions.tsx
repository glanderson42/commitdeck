import { useEffect, useState } from "react";
import { rpc, message } from "../../services/api";
import { ErrorBox, Modal } from "../../components/ui";
import { Markdown } from "../../components/Markdown";
import type { PullRequest } from "../../types";
export function PRActions({
  id,
  pr,
  disabled,
  refresh,
  onSend,
  onJira,
  intent,
  onHandled,
}: {
  id: string;
  pr?: PullRequest | null;
  disabled: boolean;
  refresh: () => Promise<void>;
  onSend: (s: string) => void;
  onJira: () => void;
  intent?: boolean;
  onHandled?: () => void;
}) {
  const [action, setAction] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [base, setBase] = useState("main");
  const [draft, setDraft] = useState(true);
  const [reviewer, setReviewer] = useState("");
  const [feedback, setFeedback] =
    useState<PullRequest["unresolvedComments"][number]>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState("");
  const [created, setCreated] = useState(false);
  const open = async (kind: string) => {
    setAction(kind);
    setError("");
    setBody(kind === "edit" ? (pr?.description ?? "") : "");
    setTitle(pr?.title ?? "");
    if (kind === "create") {
      setBusy(true);
      try {
        const t = await rpc<{ title: string; body: string; base: string }>(
          "pr.template",
          { id },
        );
        setTitle(t.title);
        setBody(t.body);
        setBase(t.base);
      } catch (e) {
        setError(message(e));
      } finally {
        setBusy(false);
      }
    }
  };
  useEffect(() => {
    if (intent) {
      void open("create");
      onHandled?.();
    }
  }, [intent]);
  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await rpc<{ url: string; warning?: string }>("pr.write", {
        id,
        action,
        number: pr?.number,
        title,
        body,
        base,
        draft,
        reviewer,
        threadId: feedback?.threadId,
        commentId: feedback?.commentId,
        headOid: pr?.headOid,
        confirm: true,
      });
      setSuccess(result.warning || `GitHub action completed: ${action}`);
      if (action === "create" || action === "merge") setCreated(true);
      setAction("");
      await refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <div className="integration-actions">
        {!pr || pr.state !== "OPEN" ? (
          <button
            className="primary"
            disabled={disabled || busy}
            onClick={() => void open("create")}
          >
            Create PR
          </button>
        ) : (
          <>
            <button
              disabled={disabled || busy}
              onClick={() => void open("edit")}
            >
              Edit PR
            </button>
            <button
              disabled={disabled || busy}
              onClick={() => void open(pr.draft ? "ready" : "draft")}
            >
              {pr.draft ? "Mark ready" : "Convert to draft"}
            </button>
            <button
              disabled={disabled || busy}
              onClick={() => void open("comment")}
            >
              Comment
            </button>
            <button
              disabled={disabled || busy}
              onClick={() => void open("reviewer")}
            >
              Request reviewer
            </button>
            <button
              disabled={disabled || busy}
              onClick={() => void open("rerun")}
            >
              Re-run failed CI
            </button>
            <button
              disabled={
                disabled ||
                busy ||
                pr.draft ||
                pr.mergeable !== "MERGEABLE" ||
                pr.changesRequested > 0 ||
                pr.reviewState !== "APPROVED" ||
                pr.checks.some((c) => c.status !== "success")
              }
              className="primary"
              onClick={() => void open("merge")}
            >
              Merge…
            </button>
          </>
        )}
      </div>
      {success && (
        <p className="context-warning" role="status">
          {success}
        </p>
      )}
      {created && (
        <div className="integration-actions">
          <span className="muted">Update the linked Jira issue?</span>
          <button onClick={onJira}>Choose Jira transition…</button>
        </div>
      )}
      {!action && error && <ErrorBox error={error} />}{" "}
      {!!pr?.unresolvedComments?.length && (
        <section className="review-feedback">
          <h3>Unresolved review threads</h3>
          {pr.unresolvedComments.map((c, i) => (
            <article
              key={c.threadId + ":" + c.commentId + ":" + i}
              className="issue-comment"
            >
              <header>
                <strong>{c.author}</strong>
                <code>{c.path}</code>
              </header>
              <Markdown>{c.body}</Markdown>
              <div className="integration-actions">
                <button
                  disabled={busy || disabled}
                  onClick={() => {
                    setBusy(true);
                    void rpc("todo.save", {
                      workspaceId: id,
                      title: `Review: ${c.body.slice(0, 850)}${c.path ? " (" + c.path + ")" : ""}`,
                    })
                      .then(refresh)
                      .then(() => setSuccess("Review feedback added to TODOs"))
                      .catch((e) => setError(message(e)))
                      .finally(() => setBusy(false));
                  }}
                >
                  Create TODO
                </button>
                <button
                  disabled={disabled || busy || !c.commentId}
                  onClick={() => {
                    setBusy(true);
                    void rpc<string>("pr.reviewContext", {
                      id,
                      commentId: c.commentId,
                    })
                      .then(onSend)
                      .catch((e) => setError(message(e)))
                      .finally(() => setBusy(false));
                  }}
                >
                  Send to agent
                </button>
                <button
                  disabled={disabled || !c.commentId}
                  onClick={() => {
                    setFeedback(c);
                    void open("reply");
                  }}
                >
                  Reply
                </button>
                <button
                  disabled={disabled || !c.threadId}
                  onClick={() => {
                    setFeedback(c);
                    void open("resolve");
                  }}
                >
                  Resolve…
                </button>
              </div>
            </article>
          ))}
        </section>
      )}
      {pr?.state === "MERGED" && (
        <p className="context-warning">
          PR merged. Move Jira to Done, mark the workspace Done, then archive or
          clean up the worktree from Overview/Git.
        </p>
      )}
      {action && (
        <Modal
          wide
          title={`GitHub · ${action.replaceAll("_", " ")}`}
          onClose={() => setAction("")}
        >
          <form
            className="form-body"
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
          >
            {(action === "create" || action === "edit") && (
              <>
                <label>
                  Title
                  <input
                    required
                    maxLength={256}
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                  />
                </label>
                <label>
                  Description
                  <textarea
                    rows={11}
                    value={body}
                    onChange={(e) => setBody(e.target.value)}
                  />
                </label>
                {action === "create" && (
                  <>
                    <label>
                      Base branch
                      <input
                        required
                        value={base}
                        onChange={(e) => setBase(e.target.value)}
                      />
                    </label>
                    <label className="check-label">
                      <input
                        type="checkbox"
                        checked={draft}
                        onChange={(e) => setDraft(e.target.checked)}
                      />
                      Create as draft
                    </label>
                    <p className="hint">
                      Push the ticket branch first using Terminal. GitHub
                      validates that the branch has changes. Only record tests
                      you actually ran.
                    </p>
                  </>
                )}
              </>
            )}
            {(action === "comment" || action === "reply") && (
              <>
                <label>
                  {action === "reply"
                    ? "Reply to review comment"
                    : "PR comment"}
                  <textarea
                    required
                    rows={7}
                    value={body}
                    maxLength={60000}
                    onChange={(e) => setBody(e.target.value)}
                  />
                </label>
                {feedback && action === "reply" && (
                  <blockquote>{feedback.body}</blockquote>
                )}
              </>
            )}
            {action === "reviewer" && (
              <label>
                GitHub username
                <input
                  required
                  value={reviewer}
                  onChange={(e) => setReviewer(e.target.value)}
                />
              </label>
            )}
            {action === "merge" && (
              <p>
                Squash merge PR #{pr?.number} at{" "}
                <code>{pr?.headOid?.slice(0, 12)}</code>? GitHub branch
                protection still applies. The branch and worktree are retained.
              </p>
            )}
            {action === "resolve" && (
              <p>
                Mark this review thread resolved? Confirm the feedback was
                actually addressed.<blockquote>{feedback?.body}</blockquote>
              </p>
            )}
            {action === "rerun" && (
              <p>
                Re-run failed GitHub Actions jobs for this PR’s current head
                commit?
              </p>
            )}
            {(action === "draft" || action === "ready") && (
              <p>
                {action === "draft"
                  ? "Convert this pull request to a draft?"
                  : "Mark this pull request ready for review?"}
              </p>
            )}
            {error && <ErrorBox error={error} />}
            <div className="form-actions">
              <button type="button" onClick={() => setAction("")}>
                Cancel
              </button>
              <button
                disabled={busy}
                className={action === "merge" ? "danger" : "primary"}
              >
                Confirm {action}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
