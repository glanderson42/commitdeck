import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { rpc } from "../../services/api";
import type { Todo } from "../../types";
import { TasksPane } from "./TasksPane";
vi.mock("../../services/api", () => ({
  rpc: vi.fn().mockResolvedValue(null),
  message: String,
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const task = (id: string, position: number): Todo => ({
  id,
  workspaceId: "ticket-a",
  title: id,
  completed: false,
  position,
  createdAt: "",
  updatedAt: "",
});
it("persists the complete task order from keyboard reordering", async () => {
  render(
    <TasksPane
      id="ticket-a"
      todos={[task("first", 0), task("second", 1)]}
      refresh={vi.fn().mockResolvedValue(undefined)}
    />,
  );
  fireEvent.keyDown(
    screen.getByRole("button", {
      name: "Move first. Alt plus arrow keys to reorder.",
    }),
    { key: "ArrowDown", altKey: true },
  );
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("todo.reorder", {
      workspaceId: "ticket-a",
      ids: ["second", "first"],
    }),
  );
  expect((screen.getByLabelText("Task 1") as HTMLInputElement).value).toBe(
    "second",
  );
});
it("adds a task scoped to the selected ticket", async () => {
  render(
    <TasksPane
      id="ticket-b"
      todos={[]}
      refresh={vi.fn().mockResolvedValue(undefined)}
    />,
  );
  fireEvent.change(screen.getByLabelText("New task title"), {
    target: { value: "Add integration tests" },
  });
  fireEvent.submit(screen.getByLabelText("New task title").closest("form")!);
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("todo.save", {
      workspaceId: "ticket-b",
      title: "Add integration tests",
    }),
  );
});
it("keeps a new task draft during background updates and clears it when switching tickets", () => {
  const refresh = vi.fn().mockResolvedValue(undefined);
  const { rerender } = render(
    <TasksPane id="ticket-a" todos={[task("first", 0)]} refresh={refresh} />,
  );
  fireEvent.change(screen.getByLabelText("New task title"), {
    target: { value: "My unfinished task" },
  });
  rerender(
    <TasksPane
      id="ticket-a"
      todos={[
        task("first", 0),
        { ...task("agent task", 1), updatedBy: "codex" },
      ]}
      refresh={refresh}
    />,
  );
  expect(screen.getByLabelText("New task title")).toHaveProperty(
    "value",
    "My unfinished task",
  );
  expect(screen.getByText("Updated by codex")).toBeTruthy();
  expect(rpc).not.toHaveBeenCalled();
  rerender(<TasksPane id="ticket-b" todos={[]} refresh={refresh} />);
  expect(screen.getByLabelText("New task title")).toHaveProperty("value", "");
});
it("rolls back a rejected completion and keeps its error visible after a refresh", async () => {
  const refresh = vi.fn().mockResolvedValue(undefined);
  const original = { ...task("first", 0), updatedAt: "2026-10-16T10:00:00Z" };
  vi.mocked(rpc).mockRejectedValueOnce(
    new Error("Task changed outside this editor"),
  );
  const { rerender } = render(
    <TasksPane id="ticket-a" todos={[original]} refresh={refresh} />,
  );
  fireEvent.click(screen.getByRole("checkbox", { name: "Complete first" }));
  await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  expect(rpc).toHaveBeenCalledWith("todo.save", {
    ...original,
    completed: true,
  });
  expect(
    screen
      .getByRole("checkbox", { name: "Complete first" })
      .getAttribute("aria-checked"),
  ).toBe("false");
  rerender(
    <TasksPane
      id="ticket-a"
      todos={[{ ...original, title: "Updated by agent" }]}
      refresh={refresh}
    />,
  );
  expect(
    screen.getByText("Error: Task changed outside this editor"),
  ).toBeTruthy();
  expect(screen.getByLabelText("Task 1")).toHaveProperty(
    "value",
    "Updated by agent",
  );
});
