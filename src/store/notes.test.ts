import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NoteDrafts } from "./notes";
describe("note autosave", () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    vi.stubGlobal("localStorage", {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
      clear: () => values.clear(),
    });
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });
  it("debounces each workspace independently when switching tickets", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const drafts = new NoteDrafts(save, 100);
    drafts.seed("a", "A");
    drafts.seed("b", "B");
    drafts.change("a", "# Investigation");
    drafts.change("b", "# Other");
    drafts.change("a", "# Decision");
    expect(save).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(100);
    expect(save).toHaveBeenCalledTimes(2);
    expect(save).toHaveBeenCalledWith("a", "# Decision");
    expect(save).toHaveBeenCalledWith("b", "# Other");
    expect(drafts.get("a")?.state).toBe("saved");
    expect(localStorage.getItem("commitdeck.note.a")).toBeNull();
  });
  it("serializes writes so a slow earlier save cannot overwrite newer text", async () => {
    let finish!: () => void;
    const save = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<void>((resolve) => {
            finish = resolve;
          }),
      )
      .mockResolvedValue(undefined);
    const drafts = new NoteDrafts(save, 100);
    drafts.seed("a", "");
    drafts.change("a", "old");
    const saving = drafts.flush("a");
    drafts.change("a", "latest");
    drafts.seed("a", "stale server result");
    expect(drafts.get("a")?.content).toBe("latest");
    finish();
    await saving;
    expect(save.mock.calls.map((args) => args[1])).toEqual(["old", "latest"]);
    expect(drafts.get("a")).toEqual({ content: "latest", state: "saved" });
  });
  it("recovers a failed draft after restart and retries persistence", async () => {
    const save = vi.fn().mockRejectedValue(new Error("disk unavailable"));
    const drafts = new NoteDrafts(save);
    drafts.seed("a", "");
    drafts.change("a", "important");
    await drafts.flush("a");
    expect(drafts.get("a")?.state).toBe("error");
    expect(localStorage.getItem("commitdeck.note.a")).toBe("important");
    const recovered = new NoteDrafts(vi.fn().mockResolvedValue(undefined));
    recovered.seed("a", "old DB");
    await recovered.flush("a");
    expect(recovered.get("a")?.content).toBe("important");
    expect(recovered.get("a")?.state).toBe("saved");
  });
});
