import { spawn, execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";
import assert from "node:assert/strict";

const root = mkdtempSync(join(tmpdir(), "commitdeck-smoke-"));
const data = join(root, "data"),
  repo = join(root, "repo");
execFileSync("git", ["init", repo], { stdio: "ignore" });
execFileSync(
  "git",
  [
    "-C",
    repo,
    "-c",
    "user.name=CommitDeck Test",
    "-c",
    "user.email=test@example.com",
    "commit",
    "--allow-empty",
    "-m",
    "Initial",
  ],
  { stdio: "ignore" },
);
let child,
  next = 0;
const pending = new Map();
function start() {
  child = spawn(
    `./bin/commitdeck-core${process.platform === "win32" ? ".exe" : ""}`,
    [],
    {
      env: { ...process.env, COMMITDECK_DATA_DIR: data },
      stdio: ["pipe", "pipe", "inherit"],
    },
  );
  createInterface({ input: child.stdout }).on("line", (line) => {
    const msg = JSON.parse(line),
      request = pending.get(msg.id);
    if (request) {
      pending.delete(msg.id);
      clearTimeout(request.timer);
      msg.error
        ? request.reject(new Error(msg.error))
        : request.resolve(msg.result);
    }
  });
}
function rpc(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++next;
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`Timeout: ${method}`));
    }, 10000);
    pending.set(id, { resolve, reject, timer });
    child.stdin.write(JSON.stringify({ id, method, params }) + "\n");
  });
}
function stop() {
  return new Promise((resolve) => {
    child.once("exit", resolve);
    child.stdin.end();
  });
}
try {
  start();
  let boot = await rpc("bootstrap");
  assert.equal(boot.workspaces.length, 0);
  const w = await rpc("workspace.create", {
    externalKey: "PROJ-421",
    title: "Invoice retry handling",
  });
  await rpc("settings.save", {
    settings: { worktreeRoot: join(root, "worktrees") },
  });
  await rpc("repo.attach", { id: w.id, path: repo });
  const isolated = await rpc("worktree.create", { id: w.id });
  assert.match(isolated.branch, /^PROJ-421-invoice-retry-handling$/);
  assert.ok(existsSync(isolated.worktreePath));
  writeFileSync(join(isolated.worktreePath, "retry.go"), "package retry\n");
  const status = await rpc("git.status", { id: w.id });
  assert.equal(status.dirty, true);
  assert.equal(status.files[0].path, "retry.go");
  assert.match(
    await rpc("git.diff", { id: w.id, file: "retry.go" }),
    /\+package retry/,
  );
  await assert.rejects(
    rpc("worktree.remove", { id: w.id, confirm: true }),
    /uncommitted/,
  );
  await rpc("note.save", {
    workspaceId: w.id,
    content: "# Investigation\nUse invoice ID.",
  });
  const a = await rpc("todo.save", {
    workspaceId: w.id,
    title: "Understand retries",
  });
  const b = await rpc("todo.save", {
    workspaceId: w.id,
    title: "Add integration tests",
  });
  await rpc("todo.reorder", { workspaceId: w.id, ids: [b.id, a.id] });
  const context = await rpc("context.build", {
    workspaceId: w.id,
    purpose: "testing",
    options: {
      maxCharacters: 3000,
      includeNotes: true,
      includeTodos: true,
      includeGit: true,
    },
  });
  assert.ok(context.characterCount <= 3000);
  assert.match(context.markdown, /Add integration tests/);
  assert.match(context.markdown, /Use invoice ID/);
  assert.equal(context.hash.length, 64);
  boot = await rpc("bootstrap");
  assert.match(boot.workspaces[0].searchText, /Use invoice ID/i);
  assert.equal(boot.workspaces[0].tasksTotal, 2);
  assert.ok(Array.isArray(boot.permissions));
  assert.ok(Array.isArray(boot.notifications));
  const activity = (await rpc("workspace.get", { id: w.id })).activity;
  assert.ok(activity.some((event) => event.type === "todo_added"));
  await rpc("layout.save", {
    content: JSON.stringify({ dashboardCollapsed: { Waiting: true } }),
  });
  await rpc("workspace.archive", { id: w.id, archived: true });
  await stop();
  start();
  const detail = await rpc("workspace.get", { id: w.id });
  assert.equal(detail.note.content, "# Investigation\nUse invoice ID.");
  assert.equal(detail.todos[0].id, b.id);
  assert.equal(detail.workspace.archived, true);
  boot = await rpc("bootstrap");
  assert.equal(JSON.parse(boot.layout).dashboardCollapsed.Waiting, true);
  assert.ok(detail.activity.length >= 2);
  await assert.rejects(rpc("jira.refresh", { id: w.id }));
  await rpc("note.save", {
    workspaceId: w.id,
    content: "# Still works offline",
  });
  await assert.rejects(rpc("workspace.delete", { id: w.id }), /confirmation/);
  await rpc("workspace.delete", { id: w.id, confirm: true });
  assert.ok(existsSync(isolated.worktreePath));
  boot = await rpc("bootstrap");
  assert.equal(boot.workspaces.length, 0);
  console.log(
    "PASS: real IPC, workspace CRUD, worktree, Git status/diff, dirty cleanup refusal, notes, TODO reorder, dashboard search/progress, budgeted context, activity, collapse persistence, archive, restart, offline isolation, confirmed deletion.",
  );
  console.log(`Disposable test artifacts: ${root}`);
} finally {
  if (child && !child.killed) await stop();
}
