import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { rpc } from "../services/api";
import { NoteDrafts } from "./notes";
vi.mock("../services/api", () => ({ rpc: vi.fn() }));
describe("note autosave", () => {
  beforeEach(() => {
    vi.mocked(rpc).mockReset();
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
    expect(save).toHaveBeenCalledWith("a", "# Decision", "A");
    expect(save).toHaveBeenCalledWith("b", "# Other", "B");
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
    recovered.seed("a", "");
    await recovered.flush("a");
    expect(recovered.get("a")?.content).toBe("important");
    expect(recovered.get("a")?.state).toBe("saved");
  });
  it("never replays a recovered draft over an agent append and requires an explicit merge", async () => {
    const failed = new NoteDrafts(
      vi.fn().mockRejectedValue(new Error("notes changed outside this editor")),
    );
    failed.seed("a", "Original");
    failed.change("a", "User draft");
    await failed.flush("a");
    const save = vi.fn().mockResolvedValue(undefined),
      recovered = new NoteDrafts(save, 100);
    recovered.seed("a", "Original\nAgent decision");
    await recovered.flush("a");
    expect(save).not.toHaveBeenCalled();
    expect(recovered.get("a")?.state).toBe("error");
    recovered.change("a", "Edited draft");
    await vi.advanceTimersByTimeAsync(100);
    expect(save).not.toHaveBeenCalled();
    await recovered.merge(
      "a",
      "Edited draft\nAgent decision",
      "Original\nAgent decision",
    );
    expect(save).toHaveBeenCalledWith(
      "a",
      "Edited draft\nAgent decision",
      "Original\nAgent decision",
    );
    expect(recovered.get("a")?.state).toBe("saved");
  });
  it("keeps legacy backups with no known base for review instead of overwriting saved notes", async () => {
    localStorage.setItem("commitdeck.note.a", "Legacy draft");
    const save = vi.fn(),
      recovered = new NoteDrafts(save);
    recovered.seed("a", "Latest saved note");
    recovered.change("a", "Edited legacy draft");
    await recovered.flush("a");
    expect(save).not.toHaveBeenCalled();
    const afterAnotherRestart = new NoteDrafts(save);
    afterAnotherRestart.seed("a", "Latest saved note");
    await afterAnotherRestart.flush("a");
    expect(save).not.toHaveBeenCalled();
    expect(afterAnotherRestart.get("a")?.state).toBe("error");
  });
  it("hashes the last saved content before each write through the real save callback", async () => {
    vi.mocked(rpc)
      .mockResolvedValueOnce("original-hash")
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce("first-edit-hash")
      .mockResolvedValueOnce(undefined);
    const drafts = new NoteDrafts();
    drafts.seed("a", "Original");
    drafts.change("a", "First edit");
    await drafts.flush("a");
    drafts.change("a", "Second edit");
    await drafts.flush("a");
    expect(vi.mocked(rpc).mock.calls).toEqual([
      ["note.hash", { content: "Original" }],
      [
        "note.save",
        {
          workspaceId: "a",
          content: "First edit",
          expectedHash: "original-hash",
        },
      ],
      ["note.hash", { content: "First edit" }],
      [
        "note.save",
        {
          workspaceId: "a",
          content: "Second edit",
          expectedHash: "first-edit-hash",
        },
      ],
    ]);
    expect(drafts.get("a")).toEqual({ content: "Second edit", state: "saved" });
    expect(localStorage.getItem("commitdeck.note.a")).toBeNull();
    expect(localStorage.getItem("commitdeck.note-base.a")).toBeNull();
  });
  it("keeps a recoverable draft and never writes when hashing fails", async () => {
    vi.mocked(rpc).mockRejectedValueOnce(new Error("Backend disconnected"));
    const drafts = new NoteDrafts();
    drafts.seed("a", "Original");
    drafts.change("a", "Unsaved edit");
    await drafts.flush("a");
    expect(rpc).toHaveBeenCalledExactlyOnceWith("note.hash", {
      content: "Original",
    });
    expect(drafts.get("a")).toEqual({
      content: "Unsaved edit",
      state: "error",
      error: "Error: Backend disconnected",
    });
    expect(localStorage.getItem("commitdeck.note.a")).toBe("Unsaved edit");
    expect(localStorage.getItem("commitdeck.note-base.a")).toBe("Original");
  });
  it("preserves both the draft and its original base after a rejected concurrent write", async () => {
    vi.mocked(rpc)
      .mockResolvedValueOnce("original-hash")
      .mockRejectedValueOnce(new Error("Notes changed outside this editor"));
    const drafts = new NoteDrafts();
    drafts.seed("a", "Original");
    drafts.change("a", "User draft");
    await drafts.flush("a");
    drafts.seed("a", "Original\nAgent decision");
    expect(drafts.get("a")?.content).toBe("User draft");
    expect(drafts.get("a")?.state).toBe("error");
    expect(localStorage.getItem("commitdeck.note-base.a")).toBe("Original");
    const recovered = new NoteDrafts();
    recovered.seed("a", "Original\nAgent decision");
    await recovered.flush("a");
    expect(rpc).toHaveBeenCalledTimes(2);
    expect(recovered.get("a")?.state).toBe("error");
  });
});
