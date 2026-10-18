import { PRActions } from "./PRActions";
import {
  Check,
  CircleCheck,
  Clock,
  GitBranch,
  GitPullRequest,
  MessageSquare,
  RefreshCw,
  X,
} from "lucide-react";
import { useState } from "react";
import { Empty, ErrorBox, ExternalLink, timeAgo } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Cache, PullRequest } from "../../types";
export function PRPane({
  id,
  cache,
  refresh,
  syncError,
  onRefresh,
  onSend,
  onJira,
  disabled,
  createIntent,
  onHandled,
}: {
  id: string;
  cache: Cache<PullRequest | null> | null;
  refresh: () => Promise<void>;
  syncError?: string;
  onRefresh?: () => Promise<void>;
  onSend: (s: string) => void;
  onJira: () => void;
  disabled: boolean;
  createIntent?: boolean;
  onHandled: () => void;
}) {
  const [localError, setError] = useState("");
  const error = localError || syncError || "";
  const [busy, setBusy] = useState(false);
  const fetch = async () => {
    setBusy(true);
    setError("");
    try {
      if (onRefresh) await onRefresh();
      else {
        await rpc("pr.refresh", { id });
        await refresh();
      }
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  const p = cache?.data;
  return (
    <div className="integration-panel">
      <div className="panel-toolbar">
        <div className="toolbar-title">
          <GitPullRequest size={16} />
          Pull request
        </div>
        <button disabled={busy} onClick={() => void fetch()}>
          <RefreshCw size={14} className={busy ? "spin" : ""} />
          Refresh GitHub
        </button>
      </div>
      <div className="integration-content">
        {error && (
          <ErrorBox
            error={`GitHub unavailable · ${error}`}
            retry={() => void fetch()}
          />
        )}
        {cache && (
          <div className="cache-status">
            {error ? "Showing cached pull request" : "Cached locally"} · Last
            updated {timeAgo(cache.updatedAt)}
          </div>
        )}
        <PRActions
          id={id}
          pr={p}
          disabled={disabled}
          refresh={refresh}
          onSend={onSend}
          onJira={onJira}
          intent={createIntent}
          onHandled={onHandled}
        />
        {!p ? (
          <Empty
            icon={<GitPullRequest size={26} />}
            title={
              cache
                ? "No open pull request for this branch"
                : "Follow this branch to review"
            }
          >
            <p>
              CommitDeck uses your authenticated gh CLI to find the pull
              request, reviews, and CI checks for this branch.
            </p>
            <button
              className="primary"
              disabled={busy}
              onClick={() => void fetch()}
            >
              <RefreshCw size={14} />
              Find pull request
            </button>
            <code className="setup-command">gh auth login</code>
          </Empty>
        ) : (
          <>
            <div className="pr-heading">
              <div className="pr-number">
                <GitPullRequest size={19} />#{p.number}
                <span className={`status-pill ${p.draft ? "muted" : "green"}`}>
                  {p.draft ? "DRAFT" : p.state}
                </span>
              </div>
              <h2>{p.title}</h2>
              <p className="muted">
                {p.author} wants to merge <code>{p.headBranch}</code> into{" "}
                <code>{p.baseBranch}</code>
              </p>
              <ExternalLink url={p.url}>Open on GitHub</ExternalLink>
            </div>
            <div className="pr-summary">
              <span>
                <GitBranch size={15} />
                {p.mergeable === "MERGEABLE"
                  ? "Mergeable"
                  : p.mergeable === "CONFLICTING"
                    ? "Merge conflicts"
                    : "Mergeability pending"}
              </span>
              <span>
                <CircleCheck size={15} />
                {p.approvals} approvals
              </span>
              <span>
                <MessageSquare size={15} />
                {p.reviewCommentCount} review comments
              </span>
            </div>
            <section>
              <h3>
                Checks{" "}
                <span className="count">
                  {p.checks.filter((c) => c.status === "success").length}/
                  {p.checks.length}
                </span>
              </h3>
              {p.checks.length === 0 && (
                <p className="muted">No checks reported.</p>
              )}
              <div className="checks-list">
                {p.checks.map((c, i) => (
                  <div className="check-row" key={`${c.name}-${i}`}>
                    <span
                      className={
                        c.status === "success"
                          ? "green"
                          : c.status === "failure"
                            ? "red"
                            : "yellow"
                      }
                    >
                      {c.status === "success" ? (
                        <Check size={17} />
                      ) : c.status === "failure" ? (
                        <X size={17} />
                      ) : (
                        <Clock size={16} />
                      )}
                    </span>
                    <strong>{c.name}</strong>
                    <span className="muted">{c.status}</span>
                    {c.url && <ExternalLink url={c.url}>Details</ExternalLink>}
                  </div>
                ))}
              </div>
            </section>
            <section>
              <h3>
                Reviews <span className="count">{p.reviews.length}</span>
              </h3>
              {p.reviewers.length > 0 && (
                <p className="muted">Requested from {p.reviewers.join(", ")}</p>
              )}
              {p.reviews.length === 0 && (
                <p className="muted">Waiting for the first review.</p>
              )}
              {p.reviews.map((r, i) => (
                <article className="review-card" key={i}>
                  <div>
                    <span className="avatar">
                      {r.author.slice(0, 2).toUpperCase()}
                    </span>
                    <strong>{r.author}</strong>
                    <span
                      className={`status-pill ${r.state === "APPROVED" ? "green" : r.state === "CHANGES_REQUESTED" ? "orange" : "muted"}`}
                    >
                      {r.state.replaceAll("_", " ").toLowerCase()}
                    </span>
                  </div>
                  {r.body && <p>{r.body}</p>}
                </article>
              ))}
            </section>
          </>
        )}
      </div>
    </div>
  );
}
