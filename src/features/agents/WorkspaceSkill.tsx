import { useEffect, useState } from "react";
import { rpc, message } from "../../services/api";
import { Modal, ErrorBox } from "../../components/ui";
import { Markdown } from "../../components/Markdown";
export function WorkspaceSkill({
  id,
  disabled,
}: {
  id: string;
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [provider, setProvider] = useState("codex");
  const [preview, setPreview] = useState<{ path: string; markdown: string }>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!open) return;
    setPreview(undefined);
    setError("");
    void rpc<{ path: string; markdown: string }>("skill.preview", {
      id,
      provider,
    })
      .then(setPreview)
      .catch((e) => setError(message(e)));
  }, [open, id, provider]);
  return (
    <>
      <button disabled={disabled} onClick={() => setOpen(true)}>
        Workspace skill…
      </button>
      {open && (
        <Modal
          wide
          title="Install workspace skill"
          onClose={() => setOpen(false)}
        >
          <div className="form-body">
            <p>
              Tool instructions are already included in session context when
              enabled. Optionally install a discoverable skill file for your
              provider. Existing files are preserved.
            </p>
            <label>
              Provider
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
              >
                <option value="codex">Codex</option>
                <option value="claude">Claude Code</option>
                <option value="opencode">OpenCode</option>
              </select>
            </label>
            {preview && (
              <>
                <code>{preview.path}</code>
                <div className="context-preview">
                  <Markdown>{preview.markdown}</Markdown>
                </div>
              </>
            )}
            {error && <ErrorBox error={error} />}
            <div className="form-actions">
              <button onClick={() => setOpen(false)}>Cancel</button>
              <button
                className="primary"
                disabled={busy || !preview}
                onClick={() => {
                  setBusy(true);
                  void rpc("skill.install", { id, provider, confirm: true })
                    .then(() => setOpen(false))
                    .catch((e) => setError(message(e)))
                    .finally(() => setBusy(false));
                }}
              >
                Confirm file creation
              </button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
}
