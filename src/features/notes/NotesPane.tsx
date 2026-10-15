import {
  Check,
  Code2,
  Columns2,
  Eye,
  FileText,
  LoaderCircle,
} from "lucide-react";
import { useSyncExternalStore } from "react";
import { Markdown } from "../../components/Markdown";
import { notes } from "../../store/notes";
import type { Layout } from "../../types";
export function NotesPane({
  id,
  mode,
  setMode,
}: {
  id: string;
  mode: Layout["noteMode"];
  setMode: (v: Layout["noteMode"]) => void;
}) {
  const draft = useSyncExternalStore(notes.subscribe, () => notes.get(id));
  if (!draft) return <div className="panel-padding muted">Loading notes…</div>;
  return (
    <div className="notes-panel">
      <div className="panel-toolbar">
        <div className="toolbar-title">
          <FileText size={16} />
          Workspace notes
        </div>
        <div className="toolbar-actions">
          <span
            className={`save-state ${draft.state === "error" ? "red" : ""}`}
          >
            {draft.state === "saved" ? (
              <>
                <Check size={12} />
                Saved locally
              </>
            ) : draft.state === "saving" ? (
              <>
                <LoaderCircle size={12} className="spin" />
                Saving…
              </>
            ) : (
              <button onClick={() => void notes.flush(id)}>
                Save failed · Retry
              </button>
            )}
          </span>
          <div className="segmented" aria-label="Note view">
            {(
              [
                ["edit", Code2],
                ["split", Columns2],
                ["preview", Eye],
              ] as const
            ).map(([value, Icon]) => (
              <button
                key={value}
                aria-label={`${value} notes`}
                aria-pressed={mode === value}
                onClick={() => setMode(value)}
              >
                <Icon size={15} />
              </button>
            ))}
          </div>
        </div>
      </div>
      {draft.error && (
        <div role="alert" className="error-box">
          {draft.error} · Your draft is kept on this device.
        </div>
      )}
      <div className={`note-columns ${mode}`}>
        {mode !== "preview" && (
          <div className="note-editor">
            <div className="subheading">MARKDOWN</div>
            <textarea
              aria-label="Workspace Markdown notes"
              spellCheck={false}
              value={draft.content}
              placeholder={
                "# Investigation\n\nCapture what you learn.\n\n## Decisions\n\n## Next steps"
              }
              onChange={(e) => notes.change(id, e.target.value)}
              onBlur={() => void notes.flush(id)}
            />
          </div>
        )}
        {mode !== "edit" && (
          <div className="note-preview">
            <div className="subheading">PREVIEW</div>
            {draft.content ? (
              <Markdown>{draft.content}</Markdown>
            ) : (
              <p className="muted">Your notes will take shape here.</p>
            )}
          </div>
        )}
      </div>
      <div className="panel-bottom muted">
        Markdown supported
        <span>
          {draft.content.length.toLocaleString()} characters · Autosave enabled
        </span>
      </div>
    </div>
  );
}
