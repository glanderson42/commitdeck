import { MessageSquare, RefreshCw, Ticket } from "lucide-react";
import { useState } from "react";
import { Markdown } from "../../components/Markdown";
import { Empty, ErrorBox, ExternalLink, timeAgo } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Cache, Issue, Workspace } from "../../types";
export function JiraPane({
  workspace: w,
  cache,
  refresh,
  syncError,
  onRefresh,
  settings,
}: {
  workspace: Workspace;
  cache: Cache<Issue> | null;
  refresh: () => Promise<void>;
  syncError?: string;
  onRefresh?: () => Promise<void>;
  settings: () => void;
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
        await rpc("jira.refresh", { id: w.id });
        await refresh();
      }
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  const issue = cache?.data;
  return (
    <div className="integration-panel">
      <div className="panel-toolbar">
        <div className="toolbar-title">
          <Ticket size={16} />
          Issue details<span className="read-only">READ ONLY</span>
        </div>
        <button disabled={busy} onClick={() => void fetch()}>
          <RefreshCw size={14} className={busy ? "spin" : ""} />
          Refresh Jira
        </button>
      </div>
      <div className="integration-content">
        {error && (
          <ErrorBox
            error={`Jira unavailable · ${error}`}
            retry={() => void fetch()}
          />
        )}
        {cache && (
          <div className="cache-status">
            {error ? "Showing cached issue" : "Cached locally"} · Last updated{" "}
            {timeAgo(cache.updatedAt)}
          </div>
        )}
        {!issue ? (
          <Empty
            icon={<Ticket size={26} />}
            title="Bring the issue into your workspace"
          >
            <p>
              Configure Jira in integrations, then refresh this ticket or import
              a new one by key.
            </p>
            <button onClick={settings}>Configure Jira</button>
          </Empty>
        ) : (
          <>
            <div className="issue-heading">
              <span className="eyebrow">{issue.key}</span>
              <h2>{issue.title}</h2>
              <ExternalLink url={issue.url}>View in Jira</ExternalLink>
            </div>
            <dl className="issue-fields">
              <div>
                <dt>Status</dt>
                <dd>
                  <span className="status-pill blue">{issue.status}</span>
                </dd>
              </div>
              <div>
                <dt>Priority</dt>
                <dd>{issue.priority || "—"}</dd>
              </div>
              <div>
                <dt>Assignee</dt>
                <dd>{issue.assignee || "Unassigned"}</dd>
              </div>
              <div>
                <dt>Reporter</dt>
                <dd>{issue.reporter || "—"}</dd>
              </div>
              <div>
                <dt>Labels</dt>
                <dd className="labels">
                  {issue.labels.length
                    ? issue.labels.map((label) => (
                        <span key={label}>{label}</span>
                      ))
                    : "—"}
                </dd>
              </div>
              <div>
                <dt>Issue updated</dt>
                <dd>
                  {issue.updatedAt
                    ? new Date(issue.updatedAt).toLocaleString()
                    : "—"}
                </dd>
              </div>
            </dl>
            <section>
              <h3>Description</h3>
              <Markdown>{issue.description || "No description."}</Markdown>
            </section>
            {issue.acceptanceCriteria && (
              <section>
                <h3>Acceptance criteria</h3>
                <Markdown>{issue.acceptanceCriteria}</Markdown>
              </section>
            )}
            {issue.links.length > 0 && (
              <section>
                <h3>Linked issues</h3>
                {issue.links.map((l) => (
                  <div key={l.url}>
                    <ExternalLink url={l.url}>{l.title}</ExternalLink>
                  </div>
                ))}
              </section>
            )}
            <section>
              <h3>
                <MessageSquare size={15} />
                Comments <span className="muted">{issue.comments.length}</span>
              </h3>
              {issue.comments.length === 0 && (
                <p className="muted">No comments yet.</p>
              )}
              {issue.comments.map((c) => (
                <article className="issue-comment" key={c.id}>
                  <header>
                    <strong>{c.author}</strong>
                    <time>
                      {c.createdAt
                        ? new Date(c.createdAt).toLocaleDateString()
                        : ""}
                    </time>
                  </header>
                  <Markdown>{c.body}</Markdown>
                </article>
              ))}
            </section>
          </>
        )}
      </div>
    </div>
  );
}
