import { rpc } from "../services/api";
export interface Draft {
  content: string;
  state: "saved" | "saving" | "error";
  error?: string;
}
interface Entry {
  snapshot: Draft;
  revision: number;
  saved: number;
  timer?: ReturnType<typeof setTimeout>;
  pending?: Promise<void>;
}
export class NoteDrafts {
  private entries = new Map<string, Entry>();
  private listeners = new Set<() => void>();
  constructor(
    private save = (id: string, content: string) =>
      rpc<void>("note.save", { workspaceId: id, content }),
    private delay = 450,
  ) {}
  subscribe = (fn: () => void) => {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  };
  private emit() {
    this.listeners.forEach((fn) => fn());
  }
  seed(id: string, content: string) {
    if (this.entries.has(id)) return;
    let backup: string | null = null;
    try {
      backup = window.localStorage.getItem(`commitdeck.note.${id}`);
    } catch {
      /* Storage may be unavailable in a restricted webview. */
    }
    this.entries.set(id, {
      snapshot: {
        content: backup ?? content,
        state: backup === null ? "saved" : "saving",
      },
      revision: backup === null ? 0 : 1,
      saved: 0,
    });
    if (backup !== null) void this.flush(id);
    this.emit();
  }
  get = (id: string) => this.entries.get(id)?.snapshot;
  change(id: string, content: string) {
    const entry = this.entries.get(id);
    if (!entry) return;
    entry.revision++;
    entry.snapshot = { content, state: "saving" };
    try {
      window.localStorage.setItem(`commitdeck.note.${id}`, content);
    } catch {
      /* SQLite autosave remains active. */
    }
    clearTimeout(entry.timer);
    entry.timer = setTimeout(() => void this.flush(id), this.delay);
    this.emit();
  }
  async flush(id: string): Promise<void> {
    const entry = this.entries.get(id);
    if (!entry) return;
    clearTimeout(entry.timer);
    if (entry.pending) {
      await entry.pending;
      if (entry.saved < entry.revision && entry.snapshot.state !== "error")
        await this.flush(id);
      return;
    }
    if (entry.saved === entry.revision) return;
    const revision = entry.revision,
      content = entry.snapshot.content;
    entry.pending = (async () => {
      try {
        await this.save(id, content);
        entry.saved = revision;
        if (entry.revision === revision) {
          try {
            window.localStorage.removeItem(`commitdeck.note.${id}`);
          } catch {
            /* SQLite has already persisted the content. */
          }
          entry.snapshot = { content, state: "saved" };
        }
      } catch (error) {
        entry.snapshot = {
          content: entry.snapshot.content,
          state: "error",
          error: String(error),
        };
      } finally {
        entry.pending = undefined;
        this.emit();
      }
    })();
    await entry.pending;
    if (entry.saved < entry.revision && entry.snapshot.state !== "error")
      await this.flush(id);
  }
}
export const notes = new NoteDrafts();
