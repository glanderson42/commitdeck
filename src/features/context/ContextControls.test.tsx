import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { rpc } from "../../services/api";
import { notes } from "../../store/notes";
import { NewSessionDialog } from "../agents/NewSessionDialog";
import type { Workspace } from "../../types";
vi.mock("../../services/api", () => ({ rpc: vi.fn(), message: String }));
vi.mock("../../store/notes", () => ({
  notes: { flush: vi.fn().mockResolvedValue(undefined), get: vi.fn() },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
it("previews the selected context and binds session creation to its exact hash", async () => {
  vi.mocked(rpc).mockImplementation(async (method) =>
    method === "context.build"
      ? {
          markdown: "# Exact ticket context",
          hash: "stable-hash",
          characterCount: 22,
          estimatedTokens: 6,
          truncated: [],
          warnings: [],
        }
      : { id: "session1" },
  );
  const created = vi.fn().mockResolvedValue(undefined);
  render(
    <NewSessionDialog
      workspace={
        {
          id: "w1",
          title: "Invoice retry",
          worktreePath: "/ticket",
        } as Workspace
      }
      providers={[{ id: "codex", name: "Codex", available: true }]}
      onClose={vi.fn()}
      onCreated={created}
    />,
  );
  await waitFor(() =>
    expect(
      screen
        .getByRole("button", { name: "Start session" })
        .hasAttribute("disabled"),
    ).toBe(false),
  );
  fireEvent.click(screen.getByLabelText("Notes"));
  await waitFor(() =>
    expect(rpc).toHaveBeenLastCalledWith(
      "context.build",
      expect.objectContaining({
        options: expect.objectContaining({ includeNotes: false }),
      }),
    ),
  );
  await waitFor(() =>
    expect(
      screen
        .getByRole("button", { name: "Preview exact context" })
        .hasAttribute("disabled"),
    ).toBe(false),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Preview exact context" }),
  );
  expect(screen.getByText("Exact ticket context")).toBeTruthy();
  expect(notes.flush).toHaveBeenCalledWith("w1");
  fireEvent.click(screen.getByRole("button", { name: "Start session" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith(
      "session.create",
      expect.objectContaining({
        workspaceId: "w1",
        contextHash: "stable-hash",
        contextOptions: expect.objectContaining({ includeNotes: false }),
        purpose: "implementation",
      }),
    ),
  );
  await waitFor(() => expect(created).toHaveBeenCalledOnce());
});
