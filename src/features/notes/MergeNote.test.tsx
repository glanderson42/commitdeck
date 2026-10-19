import { afterEach, it, expect, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { MergeNote } from "./MergeNote";
import { notes } from "../../store/notes";
import { rpc } from "../../services/api";
vi.mock("../../store/notes", () => ({
  notes: { get: vi.fn(), merge: vi.fn() },
}));
vi.mock("../../services/api", () => ({ rpc: vi.fn(), message: String }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
it("shows saved text and preserves a recovered draft until the user confirms a merge", async () => {
  vi.mocked(notes.get).mockReturnValue({
    content: "My recovered draft",
    state: "error",
  });
  vi.mocked(rpc).mockResolvedValue({
    note: { content: "Original\nAgent decision" },
  });
  vi.mocked(notes.merge).mockImplementation(async () => {
    vi.mocked(notes.get).mockReturnValue({
      content: "My draft\nAgent decision",
      state: "saved",
    });
  });
  const close = vi.fn();
  render(<MergeNote id="w" onClose={close} />);
  await waitFor(() =>
    expect(screen.getByLabelText("Latest saved note")).toHaveProperty(
      "value",
      "Original\nAgent decision",
    ),
  );
  expect(screen.getByLabelText("Merged note to save")).toHaveProperty(
    "value",
    "My recovered draft",
  );
  expect(notes.merge).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("Merged note to save"), {
    target: { value: "My draft\nAgent decision" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save merged note" }));
  await waitFor(() =>
    expect(notes.merge).toHaveBeenCalledWith(
      "w",
      "My draft\nAgent decision",
      "Original\nAgent decision",
    ),
  );
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
});
