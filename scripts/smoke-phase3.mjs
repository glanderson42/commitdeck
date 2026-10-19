import { spawn, execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, delimiter } from "node:path";
import { createInterface } from "node:readline";
import assert from "node:assert/strict";

const root = mkdtempSync(join(tmpdir(), "commitdeck-phase3-"));
const repo = join(root, "repo"),
  fixtureBin = join(root, "fixtures"),
  data = join(root, "data");
mkdirSync(fixtureBin);
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
    "Fixture",
  ],
  { stdio: "ignore" },
);
// An authenticated provider account is never used by this test.
writeFileSync(
  join(fixtureBin, "claude"),
  `#!/usr/bin/env node
const {createInterface}=require('node:readline');
const {execFile}=require('node:child_process');
const {promisify}=require('node:util');
const output=value=>process.stdout.write(JSON.stringify(value)+'\\n');
const tool=async(operation,params)=>{const {stdout}=await promisify(execFile)(process.env.COMMITDECK_TOOL_BIN,['--workspace-tool',operation,JSON.stringify(params)],{timeout:20000});return JSON.parse(stdout)};
createInterface({input:process.stdin}).on('line',async line=>{const value=JSON.parse(line);
 if(value.type==='control_request'){output({type:'control_response',response:{subtype:'success',request_id:value.request_id}});return;}
 if(value.type==='user'){try{
 const added=await tool('todo.add',{title:'Fixture: add regression test'});
 if(!added.result?.id)throw Error('TODO tool did not return an ID');
 await tool('todo.complete',{id:added.result.id,completed:true});
 await tool('notes.append',{content:'Fixture decision: preserve stable invoice IDs.'});
 let denied=false;
 try{await tool('todo.delete',{id:added.result.id})}catch(error){denied=error.code===1}
 if(!denied)throw Error('Destructive workspace edit did not fail with a nonzero exit');
 output({type:'assistant',session_id:'fixture-native-'+process.pid,message:{content:[{type:'text',text:'Findings: stable invoice ID. TODO completed and decision recorded. No repository tests were run by this fixture.'}]}});
 output({type:'result',session_id:'fixture-native-'+process.pid,is_error:false});
 }catch(error){output({type:'result',is_error:true,result:error.message})}}
});
`,
  { mode: 0o700 },
);
let child,
  next = 0;
const pending = new Map();
function start() {
  child = spawn(
    `./bin/commitdeck-core${process.platform === "win32" ? ".exe" : ""}`,
    [],
    {
      env: {
        ...process.env,
        COMMITDECK_DATA_DIR: data,
        PATH: fixtureBin + delimiter + process.env.PATH,
      },
      stdio: ["pipe", "pipe", "inherit"],
    },
  );
  createInterface({ input: child.stdout }).on("line", (line) => {
    const message = JSON.parse(line);
    const p = pending.get(message.id);
    if (!p) return;
    pending.delete(message.id);
    clearTimeout(p.timer);
    message.error
      ? p.reject(new Error(message.error))
      : p.resolve(message.result);
  });
}
function rpc(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++next;
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`Timeout: ${method}`));
    }, 25000);
    pending.set(id, { resolve, reject, timer });
    child.stdin.write(JSON.stringify({ id, method, params }) + "\n");
  });
}
async function until(fn) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    if (await fn()) return;
    await new Promise((resolve) => setTimeout(resolve, 30));
  }
  throw new Error("Phase 3 state did not settle");
}
async function stop() {
  if (!child || child.exitCode !== null) return;
  await new Promise((resolve) => {
    child.once("exit", resolve);
    child.stdin.end();
  });
}
async function denyKnowledgeDelete(workspaceId) {
  await until(async () => {
    const b = await rpc("bootstrap");
    const request = b.permissions.find(
      (p) => p.workspaceId === workspaceId && p.title === "Delete TODO",
    );
    if (!request) return false;
    assert.equal(request.highRisk, true);
    await assert.rejects(
      rpc("permissions.resolve", {
        id: request.id,
        allow: true,
        scope: "workspace",
      }),
      /individual approval/,
    );
    await rpc("permissions.resolve", {
      id: request.id,
      allow: false,
      scope: "once",
    });
    return true;
  });
}
try {
  start();
  const w = await rpc("workspace.create", {
    title: "Phase 3 end-to-end fixture",
    externalKey: "QA-3",
  });
  await rpc("settings.save", {
    settings: { worktreeRoot: join(root, "trees") },
  });
  await rpc("repo.attach", { id: w.id, path: repo });
  const plan = await rpc("lifecycle.preview", { id: w.id });
  await assert.rejects(rpc("lifecycle.start", { id: w.id }), /confirm/);
  await rpc("lifecycle.start", {
    id: w.id,
    branch: plan.branch,
    base: plan.base,
    confirm: true,
  });
  const terminal = await rpc("terminal.create", { workspaceId: w.id });
  await rpc("terminal.start", { id: terminal.id, cols: 100, rows: 28 });
  await rpc("terminal.resize", { id: terminal.id, cols: 112, rows: 35 });
  await rpc("terminal.write", {
    id: terminal.id,
    data:
      process.platform === "win32"
        ? "echo COMMITDECK_TERMINAL_OK\r"
        : "printf 'COMMITDECK_TERMINAL_OK\\n'; pwd\n",
  });
  await until(async () => {
    const s = await rpc("terminal.snapshot", { id: terminal.id });
    return Buffer.from(s.data, "base64")
      .toString()
      .includes("COMMITDECK_TERMINAL_OK");
  });
  await rpc("terminal.rename", { id: terminal.id, title: "tests" });
  if (process.platform !== "win32") {
    const generated = await rpc("context.build", {
      workspaceId: w.id,
      purpose: "implementation",
    });
    assert.match(generated.markdown, /CommitDeck workspace tools/);
    const session = await rpc("session.create", {
      workspaceId: w.id,
      provider: "claude",
      title: "Fixture implementation",
      purpose: "implementation",
      contextOptions: generated.options,
      contextHash: generated.hash,
    });
    await rpc("session.send", {
      id: session.id,
      prompt: "Exercise the scoped workspace tools fixture.",
    });
    await denyKnowledgeDelete(w.id);
    await until(async () => {
      const d = await rpc("workspace.get", { id: w.id });
      return (
        d.sessions.find((s) => s.id === session.id)?.status === "completed"
      );
    });
    const detail = await rpc("workspace.get", { id: w.id });
    assert.equal(detail.todos[0].completed, true);
    assert.equal(detail.todos[0].updatedBy, "claude");
    assert.equal(detail.note.updatedBy, "claude");
    assert.match(detail.note.content, /stable invoice IDs/);
    const flow = await rpc("workflow.create", {
      workspaceId: w.id,
      name: "Fix Bug",
      providers: ["claude", "claude", "claude", "claude"],
    });
    let workflows = await rpc("workflow.list", { workspaceId: w.id });
    assert.equal(workflows[0].steps[0].status, "pending");
    const preview = await rpc("workflow.action", {
      id: flow.id,
      action: "preview",
    });
    await rpc("workflow.action", {
      id: flow.id,
      action: "advance",
      hash: preview.context.hash,
      options: preview.context.options,
      handoff: preview.prompt,
      confirm: true,
    });
    await denyKnowledgeDelete(w.id);
    await until(async () => {
      workflows = await rpc("workflow.list", { workspaceId: w.id });
      return workflows[0].status === "awaiting_approval";
    });
    assert.equal(workflows[0].steps[1].status, "pending");
    assert.match(workflows[0].steps[0].handoff, /stable invoice ID/);
    await rpc("workflow.action", {
      id: flow.id,
      action: "stop",
      confirm: true,
    });
  }
  await stop();
  start();
  const restored = await rpc("workspace.get", { id: w.id });
  assert.equal(restored.terminals[0].title, "tests");
  assert.equal(restored.terminals[0].status, "stopped");
  assert.equal(restored.workspace.lifecycle, "CODING");
  console.log(
    "Phase 3 IPC smoke passed: PTY, scoped agent tools, destructive approvals, manual workflow handoff, and restart metadata.",
  );
} finally {
  await stop();
}
