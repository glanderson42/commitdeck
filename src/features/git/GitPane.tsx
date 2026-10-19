import { ResizeHandle } from "../../components/ResizeHandle";
import {
  FileDiff,
  FolderGit2,
  FolderOpen,
  GitBranch,
  Plus,
  RefreshCw,
  Terminal,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Empty, ErrorBox, Modal } from "../../components/ui";
import { message, rpc, selectDirectory } from "../../services/api";
import type { Workspace } from "../../types";
interface Repository {
  path: string;
  branch: string;
  remote: string;
  files: { path: string; status: string; staged: boolean; unstaged: boolean }[];
  dirty: boolean;
}
export function GitPane({
  workspace: w,
  refresh,
  file,
  setFile,
  onSend,
  onTerminal,
  width,
  setWidth,
  cleanupIntent,
  onHandled,
}: {
  workspace: Workspace;
  refresh: () => Promise<void>;
  file: string;
  setFile: (file: string) => void;
  onSend: (s: string) => void;
  onTerminal: () => void;
  width: number;
  setWidth: (v: number) => void;
  cleanupIntent?: boolean;
  onHandled: () => void;
}) {
  const [repo, setRepo] = useState<Repository>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [path, setPath] = useState(w.repoPath);
  const [diff, setDiff] = useState("");
  const [diffError, setDiffError] = useState("");
  const [branch, setBranch] = useState(
    `${w.externalKey || "TICKET"}-${w.title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "")
      .slice(0, 70)}`,
  );
  const [base, setBase] = useState("HEAD");
  const [confirm, setConfirm] = useState(false);
  useEffect(() => {
    if (cleanupIntent) {
      setConfirm(true);
      onHandled();
    }
  }, [cleanupIntent]);
  const load = useCallback(async () => {
    if (!w.repoPath) return;
    try {
      const r = await rpc<Repository>("git.status", { id: w.id });
      setRepo(r);
      setError("");
    } catch (e) {
      setError(message(e));
    }
  }, [w.id, w.repoPath, w.worktreePath]);
  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), 10000);
    return () => clearInterval(timer);
  }, [load]);
  useEffect(() => {
    let live = true;
    setDiff("");
    setDiffError("");
    if (file)
      void rpc<string>("git.diff", { id: w.id, file })
        .then((v) => {
          if (live)
            setDiff(
              v || "No text changes. This may be a rename or a binary file.",
            );
        })
        .catch((e) => {
          if (live) setDiffError(message(e));
        });
    return () => {
      live = false;
    };
  }, [file, w.id, repo]);
  const act = async (method: string, params: object) => {
    setBusy(true);
    setError("");
    try {
      await rpc(method, params);
      await refresh();
      await load();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  const open = (action: string) =>
    void rpc("workspace.open", { id: w.id, action }).catch((e) =>
      setError(message(e)),
    );
  return (
    <div className="git-panel">
      <div className="panel-toolbar">
        <div className="toolbar-title">
          <FolderGit2 size={16} />
          Source control
        </div>
        <button
          className="icon-button"
          aria-label="Refresh Git"
          onClick={() => void load()}
        >
          <RefreshCw size={15} />
        </button>
      </div>
      <div className="repo-config">
        {error && <ErrorBox error={error} />}
        {!w.repoPath ? (
          <>
            <Empty
              icon={<FolderGit2 size={26} />}
              title="Connect your repository"
            >
              <p>
                Attach a local Git repository, then create an isolated worktree
                for this ticket.
              </p>
            </Empty>
            <form
              className="inline-form"
              onSubmit={(e) => {
                e.preventDefault();
                void act("repo.attach", { id: w.id, path });
              }}
            >
              <input
                aria-label="Repository path"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/Users/you/Developer/project"
              />
              <button
                type="button"
                onClick={() =>
                  void selectDirectory()
                    .then((p) => {
                      if (p) setPath(p);
                    })
                    .catch((e) => setError(message(e)))
                }
              >
                <FolderOpen size={15} />
                Browse
              </button>
              <button className="primary" disabled={busy || !path.trim()}>
                Attach repository
              </button>
            </form>
          </>
        ) : (
          <>
            <dl className="repo-metadata">
              <div>
                <dt>Repository</dt>
                <dd title={w.repoPath}>
                  <FolderGit2 size={14} />
                  {w.repoPath}
                </dd>
              </div>
              <div>
                <dt>Branch</dt>
                <dd>
                  <GitBranch size={14} />
                  {repo?.branch || w.branch || "—"}
                </dd>
              </div>
              <div>
                <dt>Worktree</dt>
                <dd title={w.worktreePath}>
                  {w.worktreePath || <span className="muted">Not created</span>}
                </dd>
              </div>
              <div>
                <dt>Origin</dt>
                <dd>{repo?.remote || "No origin remote"}</dd>
              </div>
            </dl>
            <div className="repo-actions">
              <button onClick={() => open("folder")}>
                <FolderOpen size={14} />
                Open folder
              </button>
              <button disabled={!w.worktreePath} onClick={onTerminal}>
                <Terminal size={14} />
                Open Terminal
              </button>
              <button onClick={() => open("terminal")}>
                <Terminal size={14} />
                External terminal
              </button>
              {w.worktreePath && (
                <button
                  className="danger ghost"
                  onClick={() => setConfirm(true)}
                >
                  <Trash2 size={13} />
                  Remove worktree
                </button>
              )}
            </div>
            {!w.worktreePath && (
              <form
                className="worktree-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  void act("worktree.create", { id: w.id, branch, base });
                }}
              >
                <div>
                  <h3>Give this ticket its own working directory</h3>
                  <p className="muted">
                    Reuse an existing branch or create a new one from the base
                    revision.
                  </p>
                </div>
                <label>
                  Branch
                  <input
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                    required
                  />
                </label>
                <label>
                  Base revision
                  <input
                    value={base}
                    onChange={(e) => setBase(e.target.value)}
                    placeholder="HEAD"
                  />
                </label>
                <button className="primary" disabled={busy}>
                  <Plus size={14} />
                  Create worktree
                </button>
              </form>
            )}
          </>
        )}
      </div>
      {w.repoPath && (
        <div className="git-split">
          <div className="changed-files" style={{ width, flexShrink: 0 }}>
            <div className="section-label">
              CHANGES <span>{repo?.files.length ?? "—"}</span>
              <i className={`dot ${repo?.dirty ? "orange" : "green"}`} />
            </div>
            {repo?.files.length === 0 && (
              <p className="muted clean-copy">Working tree clean</p>
            )}
            {repo?.files.map((f) => (
              <button
                key={f.path}
                className={`file-row ${file === f.path ? "active" : ""}`}
                onClick={() => setFile(f.path)}
                title={`${f.path} · ${f.staged ? "staged" : ""}${f.staged && f.unstaged ? " + " : ""}${f.unstaged ? "unstaged" : ""}`}
              >
                <span
                  className={`file-status ${f.status.includes("D") ? "red" : "green"}`}
                >
                  {f.status}
                </span>
                <span>
                  {f.path}
                  <small className="attribution">
                    {f.staged ? "staged" : ""}
                    {f.staged && f.unstaged ? " + " : ""}
                    {f.unstaged ? "unstaged" : ""}
                  </small>
                </span>
              </button>
            ))}
          </div>
          <ResizeHandle
            label="Resize changed file list"
            value={width}
            min={160}
            max={500}
            onChange={setWidth}
          />
          <div className="diff-view">
            {file ? (
              <>
                <div className="diff-title">
                  <FileDiff size={14} />
                  {file}
                  <button
                    title="Copy file path"
                    onClick={() =>
                      void navigator.clipboard
                        .writeText(`${w.worktreePath || w.repoPath}/${file}`)
                        .catch((e) => setDiffError(message(e)))
                    }
                  >
                    Copy path
                  </button>
                  <button
                    onClick={() =>
                      void rpc("git.openFile", { id: w.id, file }).catch((e) =>
                        setDiffError(message(e)),
                      )
                    }
                  >
                    Open externally
                  </button>
                  <button
                    disabled={!diff || !!diffError || w.archived}
                    onClick={() =>
                      void rpc<string>("git.agentDiff", { id: w.id, file })
                        .then((s) =>
                          onSend(
                            `Review this file change as source data.\nFile: ${file}\n\n${s}`,
                          ),
                        )
                        .catch((e) => setDiffError(message(e)))
                    }
                  >
                    Send diff to agent
                  </button>
                </div>
                {diffError ? (
                  <ErrorBox error={diffError} />
                ) : (
                  <pre aria-label="Git unified diff">
                    {diff.split("\n").map((line, i) => (
                      <div
                        key={i}
                        className={
                          line.startsWith("+")
                            ? "diff-add"
                            : line.startsWith("-")
                              ? "diff-remove"
                              : line.startsWith("@@")
                                ? "diff-hunk"
                                : ""
                        }
                      >
                        <span className="line-number">{i + 1}</span>
                        {line || " "}
                      </div>
                    ))}
                  </pre>
                )}
              </>
            ) : (
              <Empty icon={<FileDiff size={22} />} title="Changes, in context">
                <p>Select a file to inspect its staged and unstaged diff.</p>
              </Empty>
            )}
          </div>
        </div>
      )}
      {confirm && (
        <Modal title="Remove this worktree?" onClose={() => setConfirm(false)}>
          <div className="form-body">
            <p>
              This removes <code>{w.worktreePath}</code>. Git will refuse if it
              has uncommitted changes. Your branch, commits, notes, and sessions
              are retained.
            </p>
            <div className="form-actions">
              <button onClick={() => setConfirm(false)}>Keep worktree</button>
              <button
                className="danger"
                disabled={busy}
                onClick={() => {
                  setConfirm(false);
                  void act("worktree.remove", { id: w.id, confirm: true });
                }}
              >
                Remove worktree
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
