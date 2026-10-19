import { useEffect, useState } from "react";
import { notes } from "../../store/notes";
import { rpc, message } from "../../services/api";
import { Modal, ErrorBox } from "../../components/ui";
import type { Detail } from "../../types";
export function MergeNote({
  id,
  onClose,
}: {
  id: string;
  onClose: () => void;
}) {
  const [latest, setLatest] = useState<string>();
  const [merged, setMerged] = useState(notes.get(id)?.content ?? "");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const d = await rpc<Detail>("workspace.get", { id });
      setLatest(d.note.content);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    void load();
  }, [id]);
  return (
    <Modal wide title="Compare and merge notes" onClose={onClose}>
      <form
        className="form-body"
        onSubmit={(e) => {
          e.preventDefault();
          if (latest === undefined) return;
          setBusy(true);
          void notes
            .merge(id, merged, latest)
            .then(() => {
              if (notes.get(id)?.state === "error") {
                setError(notes.get(id)?.error ?? "Save failed");
                return;
              }
              onClose();
            })
            .finally(() => setBusy(false));
        }}
      >
        <p>
          Merge the latest saved note into your draft below. Saving uses a
          conflict check and preserves your draft if someone edits the note
          again.
        </p>
        <label>
          Latest saved note
          <textarea rows={9} readOnly value={latest ?? "Loading saved note…"} />
        </label>
        <button type="button" disabled={busy} onClick={() => void load()}>
          Reload saved version
        </button>
        <label>
          Merged note to save
          <textarea
            rows={10}
            value={merged}
            onChange={(e) => setMerged(e.target.value)}
          />
        </label>
        {error && <ErrorBox error={error} />}
        <div className="form-actions">
          <button type="button" onClick={onClose}>
            Keep draft
          </button>
          <button className="primary" disabled={busy || latest === undefined}>
            Save merged note
          </button>
        </div>
      </form>
    </Modal>
  );
}
