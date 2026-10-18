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
import type { Issue, Workspace, PullRequest } from "../../types";
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

it.each([false, true])(
  "closes a successful Jira write despite a cache warning (refresh failure: %s)",
  async (failRefresh) => {
    const warning =
      "Jira action succeeded; refresh the cached ticket manually.";
    vi.mocked(rpc).mockResolvedValue({ warning });
    const refresh = failRefresh
      ? vi.fn().mockRejectedValue(new Error("Local refresh failed"))
      : vi.fn().mockResolvedValue(undefined);
    render(
      <JiraActions
        id="w"
        issue={{ key: "PROJ-421", title: "Retry" } as Issue}
        disabled={false}
        refresh={refresh}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Add comment" }));
    fireEvent.change(screen.getByLabelText("Comment"), {
      target: { value: "Implementation complete" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Confirm Jira update" }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Confirm Jira update" }),
      ).toBeNull(),
    );
    expect(await screen.findByRole("status")).toHaveProperty(
      "textContent",
      warning,
    );
    expect(rpc).toHaveBeenCalledTimes(1);
    expect(refresh).toHaveBeenCalledTimes(1);
  },
);

it.each([
  {
    name: "long body and deep path",
    body: "Detailed feedback ".repeat(100),
    path: "src/" + "deep-directory/".repeat(30) + "retry.ts",
  },
  {
    name: "multibyte body and path",
    body: "請修正重試流程 🚀".repeat(100),
    path: "src/支払い/" + "階層/".repeat(80) + "retry.ts",
  },
  {
    name: "path longer than the title budget",
    body: "Fix retry",
    path: "src/" + "nested/".repeat(200) + "retry.ts",
  },
])(
  "fits review TODOs within the backend byte limit: $name",
  async ({ body, path }) => {
    vi.mocked(rpc).mockResolvedValue(null);
    const pr: PullRequest = {
      headOid: "sha",
      number: 1,
      title: "Retry",
      url: "https://github.com/o/r/pull/1",
      author: "dev",
      state: "OPEN",
      draft: true,
      mergeable: "UNKNOWN",
      reviewState: "REVIEW_REQUIRED",
      reviewers: [],
      approvals: 0,
      changesRequested: 0,
      reviewCommentCount: 1,
      headBranch: "retry",
      baseBranch: "main",
      checks: [],
      reviews: [],
      description: "",
      unresolvedComments: [
        {
          threadId: "thread",
          commentId: 1,
          author: "Reviewer",
          body,
          path,
          url: "https://github.com/o/r/pull/1#comment",
        },
      ],
    };
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(
      <PRActions
        id="w"
        pr={pr}
        disabled={false}
        refresh={refresh}
        onSend={vi.fn()}
        onJira={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Create TODO" }));
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
    expect(rpc).toHaveBeenCalledOnce();
    const [method, params] = vi.mocked(rpc).mock.calls[0];
    expect(method).toBe("todo.save");
    expect(params).toEqual({ workspaceId: "w", title: expect.any(String) });
    const { title } = params as { title: string };
    expect(new TextEncoder().encode(title).length).toBeLessThanOrEqual(1000);
    expect(title.startsWith("Review: ")).toBe(true);
    expect(title.endsWith(")")).toBe(true);
    expect(title).not.toContain("\uFFFD");
    if (new TextEncoder().encode(path).length < 989)
      expect(title).toContain(path);
  },
);
