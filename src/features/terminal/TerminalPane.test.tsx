import { afterEach, it, expect, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { TerminalPane } from "./TerminalPane";
import { rpc } from "../../services/api";
import type { Detail } from "../../types";
vi.mock("../../services/api", () => ({ rpc: vi.fn(), message: String }));
vi.mock("./runtime", () => ({
  attachTerminal: vi.fn().mockResolvedValue({
    fit: { fit: vi.fn() },
    term: {
      onSelectionChange: () => ({ dispose: vi.fn() }),
      getSelection: () => "",
    },
  }),
  forgetTerminal: vi.fn(),
}));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
  vi.unstubAllGlobals();
});
it("restores stopped tab metadata without claiming that the process survived", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  const detail = {
    workspace: { id: "w", worktreePath: "/ticket", archived: false },
    terminals: [
      { id: "term1", title: "tests", cwd: "/ticket", status: "stopped" },
    ],
  } as Detail;
  const select = vi.fn();
  vi.mocked(rpc).mockResolvedValue(undefined);
  render(
    <TerminalPane
      detail={detail}
      refresh={vi.fn().mockResolvedValue(undefined)}
      onSend={vi.fn()}
      onSelect={select}
    />,
  );
  expect(screen.getByText(/processes do not survive restart/)).toBeTruthy();
  expect(rpc).not.toHaveBeenCalledWith("terminal.start", expect.anything());
  fireEvent.click(screen.getByRole("button", { name: "Restart shell" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("terminal.start", {
      id: "term1",
      cols: 100,
      rows: 28,
    }),
  );
  expect(select).toHaveBeenCalledWith("term1");
});
