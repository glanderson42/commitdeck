import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PromptDrafts } from "./prompts";
import { rpc } from "../services/api";
vi.mock("../services/api", () => ({ rpc: vi.fn() }));
beforeEach(() => {
  const values = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  });
  vi.useFakeTimers();
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  vi.resetAllMocks();
});

it("keeps full drafts in memory but only persists centrally redacted recovery copies", async () => {
  vi.mocked(rpc).mockResolvedValue({
    a: "Investigate [REDACTED]",
    b: "Write tests",
  });
  const drafts = new PromptDrafts();
  drafts.set(() => ({
    a: "Investigate api_key=synthetic-credential",
    b: "Write tests",
  }));
  await vi.advanceTimersByTimeAsync(450);
  expect(rpc).toHaveBeenCalledWith("drafts.redact", {
    drafts: { a: "Investigate api_key=synthetic-credential", b: "Write tests" },
  });
  expect(drafts.get().a).toContain("synthetic-credential");
  expect(window.localStorage.getItem("commitdeck.prompts")).not.toContain(
    "synthetic-credential",
  );
  expect(new PromptDrafts().get()).toEqual({
    a: "Investigate [REDACTED]",
    b: "Write tests",
  });
});
it("does not persist plaintext when the backend is unavailable", async () => {
  const drafts = new PromptDrafts(
    vi.fn().mockRejectedValue(new Error("offline")),
  );
  drafts.set(() => ({ a: "api_key=synthetic-credential" }));
  await drafts.persist();
  expect(window.localStorage.getItem("commitdeck.prompts")).toBeNull();
  expect(drafts.get().a).toContain("synthetic-credential");
});
it("ignores an older sanitizer result after a newer draft edit", async () => {
  let complete!: (drafts: Record<string, string>) => void;
  const sanitize = vi
    .fn()
    .mockImplementationOnce(
      () =>
        new Promise<Record<string, string>>((resolve) => {
          complete = resolve;
        }),
    )
    .mockResolvedValue({ a: "Newest" });
  const drafts = new PromptDrafts(sanitize);
  drafts.set(() => ({ a: "Old" }));
  const pending = drafts.persist();
  drafts.set(() => ({ a: "Newest" }));
  complete({ a: "Old" });
  await pending;
  expect(window.localStorage.getItem("commitdeck.prompts")).toBeNull();
  await drafts.persist();
  expect(
    JSON.parse(window.localStorage.getItem("commitdeck.prompts")!),
  ).toEqual({
    a: "Newest",
  });
});
