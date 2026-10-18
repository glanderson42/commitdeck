import { rpc } from "../services/api";

type Drafts = Record<string, string>;
export class PromptDrafts {
  private snapshot: Drafts = {};
  private revision = 0;
  private listeners = new Set<() => void>();
  private timer?: ReturnType<typeof setTimeout>;
  constructor(
    private sanitize = (drafts: Drafts) =>
      rpc<Drafts>("drafts.redact", { drafts }),
  ) {
    try {
      const value: unknown = JSON.parse(
        window.localStorage.getItem("commitdeck.prompts") || "{}",
      );
      if (value && typeof value === "object" && !Array.isArray(value)) {
        this.snapshot = Object.fromEntries(
          Object.entries(value).filter(([, v]) => typeof v === "string"),
        );
      }
    } catch {
      /* In-memory drafting works without webview storage. */
    }
  }
  get = () => this.snapshot;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  set = (update: (drafts: Drafts) => Drafts) => {
    this.snapshot = update(this.snapshot);
    this.revision++;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.persist(), 450);
    this.listeners.forEach((listener) => listener());
  };
  persist = async () => {
    clearTimeout(this.timer);
    const revision = this.revision;
    try {
      const safe = await this.sanitize(this.snapshot);
      if (revision === this.revision)
        window.localStorage.setItem("commitdeck.prompts", JSON.stringify(safe));
    } catch {
      /* Never fall back to persisting the raw prompt. Keep it in memory. */
    }
  };
}
export const promptDrafts = new PromptDrafts();
