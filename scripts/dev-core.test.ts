// @vitest-environment node
import { EventEmitter } from "node:events";
import { PassThrough } from "node:stream";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ChildProcess, spawn } from "node:child_process";
import { devCore } from "./dev-core";

vi.mock("node:child_process", async (importOriginal) => ({
  ...(await importOriginal<typeof import("node:child_process")>()),
  spawn: vi.fn(),
}));

function childFixture() {
  return Object.assign(new ChildProcess(), {
    stdin: new PassThrough(),
    stdout: new PassThrough(),
    exitCode: null,
    signalCode: null,
    kill: vi.fn(),
  });
}

async function serverFixture() {
  const server = {
    config: { mode: "development", logger: { error: vi.fn() } },
    httpServer: new EventEmitter(),
    middlewares: { use: vi.fn() },
  };
  const plugin = devCore();
  if (typeof plugin.configureServer !== "function")
    throw new Error("Missing server hook");
  await Reflect.apply(plugin.configureServer, {}, [server]);
  return {
    server,
    close: async () => {
      if (typeof plugin.closeBundle !== "function")
        throw new Error("Missing close hook");
      await Reflect.apply(plugin.closeBundle, {}, []);
    },
  };
}

beforeEach(() => {
  vi.stubEnv("VITEST", "");
});
afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetAllMocks();
});

it("starts a replacement sidecar only after Vite closes the old server and listens again", async () => {
  const oldChild = childFixture(),
    newChild = childFixture();
  vi.mocked(spawn).mockImplementationOnce(() => oldChild);
  vi.mocked(spawn).mockImplementationOnce(() => newChild);
  const old = await serverFixture();
  old.server.httpServer.emit("listening");
  expect(spawn).toHaveBeenCalledTimes(1);

  // Vite loads a fresh config/plugin before closing the previous server.
  const replacement = await serverFixture();
  const spawnedBeforeClose = vi.mocked(spawn).mock.calls.length;
  const stopped = old.close();
  oldChild.emit("exit", 0);
  await stopped;
  replacement.server.httpServer.emit("listening");
  const totalSpawned = vi.mocked(spawn).mock.calls.length;
  const closed = replacement.close();
  newChild.emit("exit", 0);
  await closed;

  expect(spawnedBeforeClose).toBe(1);
  expect(totalSpawned).toBe(2);
});

it("does not start a backend when a configured server closes before listening", async () => {
  const server = await serverFixture();
  await server.close();
  server.server.httpServer.emit("listening");
  expect(spawn).not.toHaveBeenCalled();
});
