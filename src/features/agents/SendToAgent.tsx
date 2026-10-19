import { useState } from "react";
import { Modal, ErrorBox } from "../../components/ui";
import { rpc, message } from "../../services/api";
import type { Detail, GeneratedContext, Provider, Session } from "../../types";
import { ContextControls, defaultContext } from "../context/ContextControls";
export function SendToAgent({
  detail,
  providers,
  text,
  onClose,
  onSent,
}: {
  detail: Detail;
  providers: Provider[];
  text: string;
  onClose: () => void;
  onSent: (s: Session) => void;
}) {
  const [target, setTarget] = useState("new");
  const [provider, setProvider] = useState(
    providers.find((p) => p.available)?.id ?? "codex",
  );
  const [prompt, setPrompt] = useState(text);
  const [opts, setOpts] = useState(defaultContext);
  const [generated, setGenerated] = useState<GeneratedContext>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <Modal wide title="Send to agent" onClose={onClose}>
      <form
        className="form-body"
        onSubmit={(e) => {
          e.preventDefault();
          setBusy(true);
          void (async () => {
            let session = detail.sessions.find((s) => s.id === target);
            if (!session) {
              if (!generated) throw new Error("Preview context first");
              session = await rpc<Session>("session.create", {
                workspaceId: detail.workspace.id,
                provider,
                title: "Follow-up",
                purpose: "implementation",
                contextOptions: opts,
                contextHash: generated.hash,
              });
            }
            await rpc("session.send", { id: session.id, prompt });
            onSent(session);
          })()
            .catch((e) => setError(message(e)))
            .finally(() => setBusy(false));
        }}
      >
        <label>
          Session
          <select value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="new">New session</option>
            {detail.sessions.map((s) => (
              <option
                key={s.id}
                value={s.id}
                disabled={!!s.pid || (s.imported && !s.resumeSupported)}
              >
                {s.provider} · {s.title} · {s.status}
              </option>
            ))}
          </select>
        </label>
        {target === "new" && (
          <>
            <label>
              Provider
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
              >
                {providers.map((p) => (
                  <option key={p.id} value={p.id} disabled={!p.available}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>
            <ContextControls
              workspaceId={detail.workspace.id}
              purpose="implementation"
              options={opts}
              onOptions={setOpts}
              onGenerated={setGenerated}
            />
          </>
        )}
        <label>
          Prompt
          <textarea
            rows={8}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
          />
        </label>
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className="primary"
            disabled={
              busy ||
              !prompt.trim() ||
              detail.workspace.archived ||
              !detail.workspace.worktreePath ||
              (target === "new" &&
                (!generated ||
                  !providers.find((p) => p.id === provider)?.available))
            }
          >
            Send prompt
          </button>
        </div>
        <p className="hint">
          Existing sessions keep their native context; use Refresh context to
          attach updated workspace sources.
        </p>
      </form>
    </Modal>
  );
}
