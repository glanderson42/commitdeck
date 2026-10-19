import type { Plugin } from "vite";
import { spawn, type ChildProcessByStdio } from "node:child_process";
import { createInterface } from "node:readline";
import type { ServerResponse } from "node:http";
import type { Readable, Writable } from "node:stream";

// Browser development uses the same Go IPC as Tauri, exposed only on the local Vite origin.
export function devCore(): Plugin {
  let closeCore: (() => Promise<void>) | undefined;
  return {
    name: "commitdeck-core",
    apply: "serve",
    async configureServer(server) {
      await closeCore?.();
      if (server.config.mode === "desktop" || process.env.VITEST) return;
      let child: ChildProcessByStdio<Writable, Readable, null> | undefined;
      let next = 0;
      const pending = new Map<
        number,
        { res: ServerResponse; timer: ReturnType<typeof setTimeout> }
      >();
      const subscribers = new Set<ServerResponse>();
      let closing: Promise<void> | undefined;
      const shutdown = () =>
        (closing ??= new Promise<void>((resolve) => {
          for (const res of subscribers) res.end();
          subscribers.clear();
          if (!child || child.exitCode !== null || child.signalCode !== null) {
            resolve();
            return;
          }
          const timeout = setTimeout(() => child?.kill("SIGTERM"), 3000);
          child.once("exit", () => {
            clearTimeout(timeout);
            resolve();
          });
          child.stdin.end();
        }));
      closeCore = shutdown;
      const startCore = () => {
        if (closing || child) return;
        child = spawn(
          `./bin/commitdeck-core${process.platform === "win32" ? ".exe" : ""}`,
          [],
          { stdio: ["pipe", "pipe", "inherit"] },
        );
        child.stdin.on("error", () => {
          /* Exit handling rejects pending requests. */
        });
        createInterface({ input: child.stdout }).on("line", (line) => {
          try {
            const msg = JSON.parse(line);
            if (msg.event) {
              for (const res of subscribers)
                res.write(`data: ${JSON.stringify(msg.event)}\n\n`);
            } else {
              const p = pending.get(msg.id);
              if (p) {
                clearTimeout(p.timer);
                p.res.end(JSON.stringify(msg));
                pending.delete(msg.id);
              }
            }
          } catch {
            /* Ignore non-protocol diagnostics. */
          }
        });
        child.on("error", (err) =>
          server.config.logger.error(
            `Core: ${err.message}. Run npm run build:core first.`,
          ),
        );
        child.on("exit", () => {
          for (const p of pending.values()) {
            clearTimeout(p.timer);
            p.res.end(JSON.stringify({ error: "Backend disconnected" }));
          }
          pending.clear();
          for (const res of subscribers) res.end();
        });
      };
      // Vite configures the replacement before closing the old server and releasing its DB lock.
      server.httpServer?.once("listening", startCore);
      server.httpServer?.on("close", () => {
        void shutdown();
      });
      server.middlewares.use("/api", (req, res) => {
        if (
          !child ||
          closing ||
          child.exitCode !== null ||
          child.signalCode !== null
        ) {
          res.writeHead(503);
          res.end();
          return;
        }
        const origin = req.headers.origin;
        if (
          req.headers.host !== "127.0.0.1:1420" ||
          (origin && origin !== "http://127.0.0.1:1420")
        ) {
          res.writeHead(403);
          res.end();
          return;
        }
        if (req.url === "/events" && req.method === "GET") {
          res.writeHead(200, {
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
            Connection: "keep-alive",
          });
          res.write(": connected\n\n");
          subscribers.add(res);
          req.on("close", () => subscribers.delete(res));
          return;
        }
        if (
          req.url !== "/rpc" ||
          req.method !== "POST" ||
          req.headers["content-type"] !== "application/json"
        ) {
          res.writeHead(400);
          res.end();
          return;
        }
        let body = "";
        req.on("data", (chunk) => {
          body += chunk;
          if (body.length > 8 * 1024 * 1024) req.destroy();
        });
        req.on("end", () => {
          try {
            const request = JSON.parse(body);
            const id = ++next;
            res.setHeader("Content-Type", "application/json");
            const timer = setTimeout(() => {
              pending.delete(id);
              res.end(JSON.stringify({ error: "Backend timed out" }));
            }, 120000);
            pending.set(id, { res, timer });
            child!.stdin.write(
              JSON.stringify({
                id,
                method: request.method,
                params: request.params,
              }) + "\n",
            );
          } catch {
            res.writeHead(400);
            res.end();
          }
        });
      });
    },
    async closeBundle() {
      await closeCore?.();
    },
  };
}
