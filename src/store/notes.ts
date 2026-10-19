import { rpc } from "../services/api";
export interface Draft {
  content: string;
  state: "saved" | "saving" | "error";
  error?: string;
}
interface Entry {
  base: string | null;
  blocked: boolean;
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
    private save = async (
      id: string,
      content: string,
      expectedContent: string,
    ) => {
      const expectedHash = await rpc<string>("note.hash", {
        content: expectedContent,
      });
      await rpc<void>("note.save", { workspaceId: id, content, expectedHash });
    },
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
    const existing = this.entries.get(id);
    if (existing) {
      if (
        existing.saved === existing.revision &&
        !existing.pending &&
        existing.snapshot.content !== content
      ) {
        existing.base = content;
        existing.snapshot = { content, state: "saved" };
        this.emit();
      }
      return;
    }
    let backup: string | null = null;
    let backupBase: string | null = null;
    try {
      backup = window.localStorage.getItem(`commitdeck.note.${id}`);
      backupBase = window.localStorage.getItem(`commitdeck.note-base.${id}`);
      if (backup === content) {
        window.localStorage.removeItem(`commitdeck.note.${id}`);
        window.localStorage.removeItem(`commitdeck.note-base.${id}`);
        backup = null;
      }
    } catch {
      /* Storage may be unavailable in a restricted webview. */
    }
    const blocked =
      backup !== null && (backupBase === null || backupBase !== content);
    this.entries.set(id, {
      base: backup === null ? content : backupBase,
      blocked,
      snapshot: {
        content: backup ?? content,
        state: blocked ? "error" : backup === null ? "saved" : "saving",
        ...(blocked
          ? {
              error:
                "Recovered draft differs from the saved note. Compare and merge before saving.",
            }
          : {}),
      },
      revision: backup === null ? 0 : 1,
      saved: 0,
    });
    if (backup !== null && !blocked) void this.flush(id);
    this.emit();
  }
  get = (id: string) => this.entries.get(id)?.snapshot;
  change(id: string, content: string) {
    const entry = this.entries.get(id);
    if (!entry) return;
    entry.revision++;
    entry.snapshot = {
      content,
      state: entry.blocked ? "error" : "saving",
      ...(entry.blocked
        ? {
            error:
              "Compare and merge this recovered draft with the saved note before saving.",
          }
        : {}),
    };
    try {
      window.localStorage.setItem(`commitdeck.note.${id}`, content);
      if (entry.base === null)
        window.localStorage.removeItem(`commitdeck.note-base.${id}`);
      else
        window.localStorage.setItem(`commitdeck.note-base.${id}`, entry.base);
    } catch {
      /* SQLite autosave remains active. */
    }
    clearTimeout(entry.timer);
    if (!entry.blocked)
      entry.timer = setTimeout(() => void this.flush(id), this.delay);
    this.emit();
  }
  async flush(id: string): Promise<void> {
    const entry = this.entries.get(id);
    if (!entry || entry.blocked || entry.base === null) return;
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
        await this.save(id, content, entry.base!);
        entry.base = content;
        entry.saved = revision;
        if (entry.revision === revision) {
          try {
            window.localStorage.removeItem(`commitdeck.note.${id}`);
            window.localStorage.removeItem(`commitdeck.note-base.${id}`);
          } catch {
            /* SQLite has already persisted the content. */
          }
          entry.snapshot = { content, state: "saved" };
        } else {
          try {
            window.localStorage.setItem(`commitdeck.note-base.${id}`, content);
          } catch {
            /* Keep the newer draft in memory. */
          }
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
  async merge(id: string, content: string, expectedContent: string) {
    const entry = this.entries.get(id);
    if (!entry) return;
    if (entry.pending) await entry.pending;
    entry.base = expectedContent;
    entry.blocked = false;
    this.change(id, content);
    await this.flush(id);
  }
}
export const notes = new NoteDrafts();
