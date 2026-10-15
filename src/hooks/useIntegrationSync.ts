import { useCallback, useEffect, useRef, useState } from "react";
import { message, rpc } from "../services/api";
import type { Bootstrap } from "../types";

export function useIntegrationSync(
  boot: Bootstrap | undefined,
  refresh: () => Promise<void>,
  selected?: string,
) {
  const current = useRef(boot);
  current.current = boot;
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const attempted = useRef(new Map<string, number>());
  const inFlight = useRef(new Set<string>());
  const [errors, setErrors] = useState<Record<string, string>>({});
  const sync = useCallback(
    async (id: string, kind: "jira" | "pr") => {
      const key = `${id}:${kind}`;
      if (inFlight.current.has(key)) return;
      inFlight.current.add(key);
      attempted.current.set(key, Date.now());
      try {
        await rpc(`${kind}.refresh`, { id });
        setErrors((all) => ({ ...all, [key]: "" }));
        await refresh();
      } catch (e) {
        setErrors((all) => ({ ...all, [key]: message(e) }));
      } finally {
        inFlight.current.delete(key);
      }
    },
    [refresh],
  );
  const syncAll = useCallback(
    async (force = false) => {
      const b = current.current;
      if (!b || (!force && document.visibilityState === "hidden")) return;
      const jobs: { id: string; kind: "jira" | "pr" }[] = [];
      for (const w of b.workspaces.filter((w) => !w.archived)) {
        if (w.repoPath) jobs.push({ id: w.id, kind: "pr" });
        if (w.jiraTicketId && b.settings.jiraTokenSet)
          jobs.push({ id: w.id, kind: "jira" });
      }
      let next = 0;
      await Promise.all(
        [0, 1].map(async () => {
          while (next < jobs.length) {
            const job = jobs[next++];
            const interval = job.id === selectedRef.current ? 60000 : 180000;
            if (
              force ||
              Date.now() -
                (attempted.current.get(`${job.id}:${job.kind}`) ?? 0) >=
                interval
            )
              await sync(job.id, job.kind);
          }
        }),
      );
    },
    [sync],
  );
  const ready = !!boot;
  const targets =
    boot?.workspaces
      .filter((w) => !w.archived)
      .map((w) => `${w.id}:${w.repoPath}:${w.jiraTicketId}`)
      .join("|") ?? "";
  useEffect(() => {
    if (!ready) return;
    void syncAll();
    const timer = setInterval(() => void syncAll(), 15000);
    return () => clearInterval(timer);
  }, [ready, targets, syncAll]);
  return { integrationErrors: errors, sync, syncAll };
}
