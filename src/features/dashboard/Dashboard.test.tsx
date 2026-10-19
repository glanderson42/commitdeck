import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Deck } from "../../hooks/useDeck";
import type { DashboardRow } from "../../types";
import { Dashboard } from "./Dashboard";
afterEach(cleanup);
const row = (
  id: string,
  title: string,
  group: string,
  archived = false,
): DashboardRow => ({
  id,
  title,
  externalKey: id,
  status: "In Progress",
  lifecycle: "CODING",
  repoPath: "",
  worktreePath: "",
  branch: "retry-branch",
  jiraTicketId: "",
  archived,
  createdAt: "",
  updatedAt: "",
  attention: group === "Active work" ? "AGENT_RUNNING" : "CI_FAILED",
  group,
  sessions: [],
  pr: null,
  tasksTotal: 7,
  tasksCompleted: 5,
  pendingPermissions: 0,
  searchText: "idempotency decision",
  lastActivity: "2026-10-15T12:00:00Z",
});
function setup() {
  const select = vi.fn(),
    setLayout = vi.fn();
  const deck = {
    boot: {
      workspaces: [
        row("PROJ-1", "Invoice retry", "Needs attention"),
        row("PROJ-2", "Usage API", "Active work"),
        row("OLD-1", "Archived ticket", "Waiting", true),
      ],
      permissions: [],
    },
    layout: { dashboardCollapsed: {} },
    select,
    setLayout,
  } as unknown as Deck;
  render(<Dashboard deck={deck} onCreate={vi.fn()} />);
  return { select, setLayout, deck };
}
it("filters actionable groups and searches notes/branches", () => {
  setup();
  expect(screen.queryByText("Archived ticket")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Active" }));
  expect(screen.queryByText("Invoice retry")).toBeNull();
  expect(screen.getAllByText("Usage API").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByRole("button", { name: "All" }));
  fireEvent.change(screen.getByLabelText("Search all workspace context"), {
    target: { value: "idempotency" },
  });
  expect(screen.getAllByText("Invoice retry").length).toBeGreaterThan(0);
  fireEvent.change(screen.getByLabelText("Search all workspace context"), {
    target: { value: "no result" },
  });
  expect(screen.getByText("No matching workspaces")).toBeTruthy();
});
it("moves keyboard focus across workspace rows and opens the selected ticket", () => {
  const { select } = setup();
  const first = screen
    .getAllByRole("button")
    .find((b) => b.classList.contains("dashboard-row"))!;
  first.focus();
  fireEvent.keyDown(first, { key: "ArrowDown" });
  expect(document.activeElement?.textContent).toContain("Usage API");
  fireEvent.click(document.activeElement!);
  expect(select).toHaveBeenCalledWith("PROJ-2", "Overview", undefined);
});
it("persists collapse choices through the shared layout store", () => {
  const { setLayout } = setup();
  fireEvent.click(
    screen.getByRole("button", { name: "Needs attention, 1 workspaces" }),
  );
  const update = setLayout.mock.calls[0][0];
  expect(
    update({ dashboardCollapsed: { Ready: true } }).dashboardCollapsed,
  ).toEqual({ Ready: true, "Needs attention": true });
});
