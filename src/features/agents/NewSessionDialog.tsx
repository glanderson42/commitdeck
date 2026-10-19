import { useState } from "react";
import type {
  GeneratedContext,
  Provider,
  Purpose,
  Session,
  Workspace,
} from "../../types";
import { Modal, ErrorBox } from "../../components/ui";
import {
  ContextControls,
  defaultContext,
  purposes,
} from "../context/ContextControls";
import { rpc, message } from "../../services/api";
export function NewSessionDialog({
  workspace: w,
  providers,
  initialProvider,
  initialPurpose,
  onClose,
  onCreated,
}: {
  workspace: Workspace;
  providers: Provider[];
  initialProvider?: string;
  initialPurpose?: Purpose;
  onClose: () => void;
  onCreated: (s: Session) => Promise<void>;
}) {
  const [provider, setProvider] = useState(
    initialProvider || providers.find((p) => p.available)?.id || "codex",
  );
  const [purpose, setPurpose] = useState<Purpose>(
    initialPurpose ?? "implementation",
  );
  const [title, setTitle] = useState(initialPurpose ?? "Implementation");
  const [options, setOptions] = useState(defaultContext);
  const [generated, setGenerated] = useState<GeneratedContext>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <Modal wide title="New agent session" onClose={onClose}>
      <form
        className="form-body"
        onSubmit={(e) => {
          e.preventDefault();
          if (!generated) return;
          setBusy(true);
          setError("");
          void rpc<Session>("session.create", {
            workspaceId: w.id,
            provider,
            title,
            purpose,
            contextOptions: options,
            contextHash: generated.hash,
          })
            .then(onCreated)
            .then(onClose)
            .catch((e) => setError(message(e)))
            .finally(() => setBusy(false));
        }}
      >
        <p className="muted">
          A persistent session in {w.externalKey || w.title}. Review the context
          before starting.
        </p>
        {!w.worktreePath && (
          <ErrorBox error="Create a worktree in the Git tab before starting an agent." />
        )}
        <div className="form-grid">
          <label>
            Provider
            <select
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
            >
              {providers.map((p) => (
                <option value={p.id} key={p.id} disabled={!p.available}>
                  {p.name}
                  {p.available ? "" : " · Not installed"}
                </option>
              ))}
            </select>
          </label>
          <label>
            Purpose
            <select
              value={purpose}
              onChange={(e) => setPurpose(e.target.value as Purpose)}
            >
              {purposes.map((p) => (
                <option key={p}>{p}</option>
              ))}
            </select>
          </label>
        </div>
        <label>
          Session title
          <input
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={200}
          />
        </label>
        <ContextControls
          workspaceId={w.id}
          purpose={purpose}
          options={options}
          onOptions={setOptions}
          onGenerated={setGenerated}
        />
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className="primary"
            disabled={
              busy ||
              !generated ||
              !w.worktreePath ||
              w.archived ||
              !providers.find((p) => p.id === provider)?.available
            }
          >
            {busy ? "Creating…" : "Start session"}
          </button>
        </div>
        <p className="hint">
          The first prompt receives this exact snapshot. Starting a session does
          not issue a model request until you send a prompt.
        </p>
      </form>
    </Modal>
  );
}
