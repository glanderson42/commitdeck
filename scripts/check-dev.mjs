import { createServer } from "vite";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import assert from "node:assert/strict";

process.env.COMMITDECK_DATA_DIR = mkdtempSync(
  join(tmpdir(), "commitdeck-dev-check-"),
);
const server = await createServer({
  configFile: "vite.config.ts",
  // This test exercises IPC/restart, not frontend dependency optimization.
  optimizeDeps: { noDiscovery: true, include: [] },
  server: { host: "127.0.0.1" },
});
const bootstrap = async () => {
  const response = await fetch("http://127.0.0.1:1420/api/rpc", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ method: "bootstrap", params: {} }),
  });
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.error, undefined);
  assert.deepEqual(body.result.workspaces, []);
};
try {
  await server.listen();
  await bootstrap();
  const stream = await fetch("http://127.0.0.1:1420/api/events");
  const reader = stream.body.getReader();
  await reader.read();
  const ended = (async () => {
    try {
      while (!(await reader.read()).done) {
        /* Drain until restart closes SSE. */
      }
    } catch {
      /* Vite may close the socket before its final chunk. */
    }
  })();
  await server.restart();
  await bootstrap();
  await ended;
  await server.restart();
  await bootstrap();
  console.log(
    "PASS: Vite restarts release the previous backend and database lock, including with a live event stream.",
  );
} finally {
  await server.close();
}
