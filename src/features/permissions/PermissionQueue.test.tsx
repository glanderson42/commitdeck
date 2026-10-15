import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { rpc } from "../../services/api";
import type { PermissionRequest } from "../../types";
import { PermissionQueue } from "./PermissionQueue";
vi.mock("../../services/api", () => ({
  rpc: vi.fn().mockResolvedValue(null),
  message: String,
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const request: PermissionRequest = {
  id: "p1",
  workspaceId: "w1",
  sessionId: "s1",
  provider: "codex",
  category: "shell_command",
  title: "Run tests",
  description: "",
  command: "go test ./...",
  filePath: "",
  cwd: "/ticket",
  executable: "go",
  arguments: ["test", "./..."],
  highRisk: false,
  canRespond: true,
  status: "pending",
  createdAt: new Date().toISOString(),
};
it("sends an individual decision and refreshes only after provider success", async () => {
  const refresh = vi.fn().mockResolvedValue(undefined);
  render(<PermissionQueue requests={[request]} refresh={refresh} />);
  fireEvent.click(screen.getByRole("button", { name: "Allow once" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("permissions.resolve", {
      id: "p1",
      allow: true,
      scope: "once",
    }),
  );
  await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
});
it("does not offer remembered permissions for destructive commands", () => {
  render(
    <PermissionQueue
      requests={[{ ...request, command: "rm -rf ./generated", highRisk: true }]}
      refresh={vi.fn()}
    />,
  );
  expect(screen.queryByText("Always allow in workspace…")).toBeNull();
  expect(screen.queryByText("Allow for session…")).toBeNull();
  expect(screen.getByRole("button", { name: "Deny" })).toBeTruthy();
});
it("requires confirmation before remembering a workspace command", async () => {
  render(
    <PermissionQueue
      requests={[request]}
      refresh={vi.fn().mockResolvedValue(undefined)}
    />,
  );
  fireEvent.click(screen.getByText("Always allow in workspace…"));
  expect(rpc).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Allow exact command" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("permissions.resolve", {
      id: "p1",
      allow: true,
      scope: "workspace",
    }),
  );
});
it("keeps provider failures visible without claiming approval", async () => {
  vi.mocked(rpc).mockRejectedValueOnce(new Error("Approval channel closed"));
  const refresh = vi.fn();
  render(<PermissionQueue requests={[request]} refresh={refresh} />);
  fireEvent.click(screen.getByRole("button", { name: "Allow once" }));
  await screen.findByText(/Approval channel closed/);
  expect(refresh).not.toHaveBeenCalled();
});
