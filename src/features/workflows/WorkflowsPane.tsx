import { useEffect, useState } from "react";
import { GitFork, Play, Plus } from "lucide-react";
import { rpc, message } from "../../services/api";
import { ErrorBox, Modal } from "../../components/ui";
import { ContextControls, defaultContext } from "../context/ContextControls";
import type {
  AgentWorkflow,
  WorkflowStep,
  Provider,
  GeneratedContext,
  Session,
} from "../../types";
const presets: Record<string, string[]> = {
  "Implement Feature": ["investigation", "implementation", "testing", "review"],
  "Fix Bug": ["debugging", "implementation", "testing", "review"],
  Investigate: ["investigation"],
  "Review PR": ["review"],
};
export function WorkflowsPane({
  workspaceId,
  providers,
  onSession,
  refresh,
  disabled,
  intent,
  onHandled,
}: {
  workspaceId: string;
  providers: Provider[];
  onSession: (id: string) => void;
  refresh: () => Promise<void>;
  disabled: boolean;
  intent?: boolean;
  onHandled: () => void;
}) {
  const [workflows, setWorkflows] = useState<AgentWorkflow[]>([]);
  const [creating, setCreating] = useState(false);
  const [preset, setPreset] = useState("Implement Feature");
  const [mapping, setMapping] = useState([
    "claude",
    "codex",
    "opencode",
    "claude",
  ]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [review, setReview] = useState<{
    workflow: AgentWorkflow;
    step: WorkflowStep;
    prompt: string;
  }>();
  const [opts, setOpts] = useState(defaultContext);
  const [generated, setGenerated] = useState<GeneratedContext>();
  useEffect(() => {
    if (intent) {
      setCreating(true);
      onHandled();
    }
  }, [intent]);
  const load = () =>
    rpc<AgentWorkflow[]>("workflow.list", { workspaceId }).then(setWorkflows);
  useEffect(() => {
    void load().catch((e) => setError(message(e)));
  }, [workspaceId, refresh]);
  useEffect(() => {
    const t = setInterval(() => void load().catch(() => undefined), 5000);
    return () => clearInterval(t);
  }, [workspaceId]);
  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
      await load();
      await refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="workflows">
      <h3>
        <GitFork size={15} />
        Agent workflows
        <button disabled={disabled} onClick={() => setCreating(true)}>
          <Plus size={13} />
          New workflow
        </button>
      </h3>
      {error && <ErrorBox error={error} />}{" "}
      {!workflows.length && (
        <p className="muted">
          Investigation, implementation, testing, and review with your approval
          between steps.
        </p>
      )}
      {workflows.map((w) => {
        const current = w.steps.find(
          (s) => !["completed", "skipped"].includes(s.status),
        );
        return (
          <article className="workflow-row" key={w.id}>
            <header>
              <strong>{w.name}</strong>
              <span>{w.status.replaceAll("_", " ")}</span>
            </header>
            <ol>
              {w.steps.map((s) => (
                <li key={s.id}>
                  <span
                    className={
                      s.status === "failed"
                        ? "red"
                        : s.status === "completed"
                          ? "green"
                          : "muted"
                    }
                  >
                    {s.status === "completed" ? "✓" : s.position + 1}
                  </span>
                  <strong>{s.purpose}</strong>
                  <span>{s.provider}</span>
                  <span className="muted">{s.status}</span>
                  {s.sessionId && (
                    <button onClick={() => onSession(s.sessionId)}>
                      Open session
                    </button>
                  )}
                </li>
              ))}
            </ol>
            {current && !["stopped", "completed"].includes(w.status) && (
              <div className="workflow-actions">
                {current.status !== "running" && (
                  <>
                    <select
                      aria-label="Step provider"
                      value={current.provider}
                      disabled={busy}
                      onChange={(e) =>
                        void act(() =>
                          rpc("workflow.action", {
                            id: w.id,
                            action: "provider",
                            provider: e.target.value,
                            confirm: true,
                          }),
                        )
                      }
                    >
                      {providers.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                          {p.available ? "" : " · Not installed"}
                        </option>
                      ))}
                    </select>
                    {current.status === "failed" ? (
                      <button
                        disabled={busy}
                        onClick={() =>
                          void act(() =>
                            rpc("workflow.action", {
                              id: w.id,
                              action: "retry",
                              confirm: true,
                            }),
                          )
                        }
                      >
                        Retry step
                      </button>
                    ) : (
                      <button
                        disabled={busy || disabled}
                        onClick={() =>
                          void act(async () => {
                            const plan = await rpc<{
                              step: WorkflowStep;
                              prompt: string;
                            }>("workflow.action", {
                              id: w.id,
                              action: "preview",
                              options: opts,
                            });
                            setReview({ workflow: w, ...plan });
                          })
                        }
                      >
                        <Play size={13} />
                        Review next step
                      </button>
                    )}
                    <button
                      disabled={busy}
                      onClick={() => {
                        if (
                          window.confirm(
                            "Skip this step without running an agent?",
                          )
                        )
                          void act(() =>
                            rpc("workflow.action", {
                              id: w.id,
                              action: "skip",
                              confirm: true,
                            }),
                          );
                      }}
                    >
                      Skip
                    </button>
                  </>
                )}
                <button
                  disabled={busy}
                  onClick={() => {
                    if (
                      window.confirm(
                        "Stop this workflow and cancel its current agent turn?",
                      )
                    )
                      void act(() =>
                        rpc("workflow.action", {
                          id: w.id,
                          action: "stop",
                          confirm: true,
                        }),
                      );
                  }}
                >
                  Stop workflow
                </button>
              </div>
            )}
          </article>
        );
      })}
      {creating && (
        <Modal title="New agent workflow" onClose={() => setCreating(false)}>
          <form
            className="form-body"
            onSubmit={(e) => {
              e.preventDefault();
              void act(async () => {
                await rpc("workflow.create", {
                  workspaceId,
                  name: preset,
                  providers: mapping,
                });
                setCreating(false);
              });
            }}
          >
            <label>
              Preset
              <select
                value={preset}
                onChange={(e) => {
                  setPreset(e.target.value);
                  setMapping(
                    presets[e.target.value].map(
                      (_, i) => ["claude", "codex", "opencode", "claude"][i],
                    ),
                  );
                }}
              >
                {Object.keys(presets).map((p) => (
                  <option key={p}>{p}</option>
                ))}
              </select>
            </label>
            {presets[preset].map((purpose, i) => (
              <label key={purpose}>
                {i + 1}. {purpose}
                <select
                  value={mapping[i]}
                  onChange={(e) =>
                    setMapping((m) =>
                      m.map((p, j) => (j === i ? e.target.value : p)),
                    )
                  }
                >
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                      {p.available ? "" : " · Not installed"}
                    </option>
                  ))}
                </select>
              </label>
            ))}
            <p className="hint">
              Each step requires a separate context preview and approval. Failed
              steps never advance automatically.
            </p>
            <button className="primary" disabled={busy}>
              Create workflow
            </button>
          </form>
        </Modal>
      )}
      {review && (
        <Modal
          wide
          title={`Start ${review.step.purpose}`}
          onClose={() => setReview(undefined)}
        >
          <form
            className="form-body"
            onSubmit={(e) => {
              e.preventDefault();
              void act(async () => {
                if (!generated) return;
                const s = await rpc<Session>("workflow.action", {
                  id: review.workflow.id,
                  action: "advance",
                  confirm: true,
                  hash: generated.hash,
                  options: opts,
                  handoff: review.prompt,
                });
                setReview(undefined);
                onSession(s.id);
              });
            }}
          >
            <p>Provider: {review.step.provider}</p>
            <ContextControls
              workspaceId={workspaceId}
              purpose={review.step.purpose}
              options={opts}
              onOptions={setOpts}
              onGenerated={setGenerated}
            />
            <label>
              Step instruction and previous handoff
              <textarea
                rows={9}
                value={review.prompt}
                onChange={(e) =>
                  setReview({ ...review, prompt: e.target.value })
                }
              />
            </label>
            {error && <ErrorBox error={error} />}
            <button className="primary" disabled={busy || !generated}>
              Approve and start this step
            </button>
          </form>
        </Modal>
      )}
    </section>
  );
}
