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
