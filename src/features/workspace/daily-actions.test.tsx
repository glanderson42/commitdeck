import { afterEach, it, expect, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { rpc } from "../../services/api";
import { JiraActions } from "../jira/JiraActions";
import { PRActions } from "../github/PRActions";
import { ImportSessions } from "../agents/ImportSessions";
import { fuzzyMatch } from "./CommandPalette";
import type { Issue, Workspace } from "../../types";
vi.mock("../../services/api", () => ({ rpc: vi.fn(), message: String }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
it("fetches real Jira transitions and waits for confirmation before writing", async () => {
  vi.mocked(rpc).mockImplementation(async (method) =>
    method === "jira.transitions"
      ? [{ id: "73", name: "Submit review", status: "Team Review" }]
      : null,
  );
  render(
    <JiraActions
      id="w"
      issue={{ key: "PROJ-421", title: "Retry" } as Issue}
      refresh={vi.fn().mockResolvedValue(undefined)}
      disabled={false}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Change status" }));
  await screen.findByText("Submit review → Team Review");
  expect(rpc).not.toHaveBeenCalledWith("jira.write", expect.anything());
  fireEvent.click(screen.getByRole("button", { name: "Confirm Jira update" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith(
      "jira.write",
      expect.objectContaining({
        id: "w",
        action: "transition",
        transitionId: "73",
        confirm: true,
      }),
    ),
  );
});
it("allows editing the PR draft and only submits the reviewed payload", async () => {
  vi.mocked(rpc).mockImplementation(async (method) =>
    method === "pr.template"
      ? {
          title: "PROJ-421 Retry",
          body: "## Testing\nRecord actual results.",
          base: "main",
        }
      : { url: "https://github.com/o/r/pull/1" },
  );
  render(
    <PRActions
      id="w"
      disabled={false}
      refresh={vi.fn().mockResolvedValue(undefined)}
      onSend={vi.fn()}
      onJira={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Create PR" }));
  await waitFor(() =>
    expect(screen.getByLabelText("Title")).toHaveProperty(
      "value",
      "PROJ-421 Retry",
    ),
  );
  fireEvent.change(screen.getByLabelText("Description"), {
    target: { value: "## Testing\ngo test ./... passed" },
  });
  expect(rpc).not.toHaveBeenCalledWith("pr.write", expect.anything());
  fireEvent.click(screen.getByRole("button", { name: "Confirm create" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith(
      "pr.write",
      expect.objectContaining({
        confirm: true,
        body: "## Testing\ngo test ./... passed",
        draft: true,
      }),
    ),
  );
});
it("suggests a workspace without automatically importing a native session", async () => {
  vi.mocked(rpc).mockResolvedValue({
    sessions: [
      {
        provider: "codex",
        externalSessionId: "native-123",
        title: "Retry investigation",
        cwd: "/ticket",
        updatedAt: new Date().toISOString(),
        resumeSupported: true,
        suggestedWorkspaceId: "w",
        confidence: "High",
        source: "thread/list",
        imported: false,
      },
    ],
    warnings: [],
  });
  const imported = vi.fn();
  render(
    <ImportSessions
      workspaces={[
        {
          id: "w",
          title: "Retry",
          externalKey: "PROJ-421",
          archived: false,
        } as Workspace,
      ]}
      onClose={vi.fn()}
      onImported={imported}
    />,
  );
  fireEvent.click(
    await screen.findByRole("button", { name: /Retry investigation/ }),
  );
  expect(screen.getByLabelText("Attach to workspace")).toHaveProperty(
    "value",
    "w",
  );
  expect(rpc).not.toHaveBeenCalledWith("discovery.import", expect.anything());
  fireEvent.click(screen.getByRole("button", { name: "Attach this session" }));
  await waitFor(() =>
    expect(rpc).toHaveBeenCalledWith("discovery.import", {
      provider: "codex",
      externalSessionId: "native-123",
      workspaceId: "w",
    }),
  );
});
it("matches abbreviated commands in order without requiring a substring", () => {
  expect(fuzzyMatch("op trm", "Open Terminal")).toBe(true);
  expect(fuzzyMatch("jira done", "Move Jira to Done")).toBe(true);
  expect(fuzzyMatch("xyz", "Open Terminal")).toBe(false);
});
