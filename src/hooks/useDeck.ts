import { useCallback, useEffect, useRef, useState } from "react";
import { message, rpc, subscribe } from "../services/api";
import { notes } from "../store/notes";
import type {
  AgentEvent,
  Bootstrap,
  Detail,
  Layout,
  Workspace,
} from "../types";
import { useIntegrationSync } from "./useIntegrationSync";

const defaults: Layout = {
  view: "dashboard",
  dashboardCollapsed: {},
  tabs: {},
  sessions: {},
  sidebarWidth: 260,
  contextWidth: 280,
  noteMode: "split",
  gitFiles: {},
  showContext: true,
};
export function useDeck() {
  const [boot, setBoot] = useState<Bootstrap>();
  const [layout, setLayout] = useState<Layout>(defaults);
  const [details, setDetails] = useState<Record<string, Detail>>({});
  const [events, setEvents] = useState<Record<string, AgentEvent[]>>({});
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState("");
  const [offline, setOffline] = useState(false);
  const selected = useRef<string | undefined>(undefined);
  const layoutReady = useRef(false);
  const report = useCallback((e: unknown) => setError(message(e)), []);
  const load = useCallback(async (id: string) => {
    try {
      const d = await rpc<Detail>("workspace.get", { id });
      notes.seed(id, d.note.content);
      setDetails((all) => ({ ...all, [id]: d }));
      setLoadError("");
    } catch (e) {
      setLoadError(message(e));
    }
  }, []);
  const refresh = useCallback(async () => {
    try {
      const b = await rpc<Bootstrap>("bootstrap");
      setBoot(b);
      setOffline(false);
      if (selected.current) await load(selected.current);
    } catch (e) {
      report(e);
    }
  }, [load, report]);
  useEffect(() => {
    let disposed = false;
    let unsubscribe: (() => void) | undefined;
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    void (async () => {
      try {
        const b = await rpc<Bootstrap>("bootstrap");
        if (disposed) return;
        setBoot(b);
        let stored: Partial<Layout> = {};
        try {
          stored = JSON.parse(b.layout || "{}");
        } catch {
          /* Defaults repair invalid layout. */
        }
        const id =
          b.workspaces.find((w) => w.id === stored.selected)?.id ??
          b.workspaces.find((w) => !w.archived)?.id;
        setLayout({
          ...defaults,
          ...stored,
          tabs: stored.tabs ?? {},
          sessions: stored.sessions ?? {},
          gitFiles: stored.gitFiles ?? {},
          selected: id,
          view: "dashboard",
          dashboardCollapsed: stored.dashboardCollapsed ?? {},
        });
        layoutReady.current = true;
      } catch (e) {
        if (!disposed) report(e);
      }
    })();
    void subscribe(
      (e) => {
        if (e.event) {
          const ev = e.event;
          setEvents((all) => {
            const old = all[ev.sessionId] ?? [];
            if (old.some((x) => x.id === ev.id)) return all;
            return { ...all, [ev.sessionId]: [...old, ev] };
          });
        }
        if (
          e.type === "sessions_changed" ||
          e.type === "permissions_changed" ||
          e.type === "activity" ||
          e.event?.type === "permission_request" ||
          e.event?.type === "complete"
        ) {
          clearTimeout(refreshTimer);
          refreshTimer = setTimeout(() => void refresh(), 120);
        }
      },
      () => setOffline(true),
    ).then((fn) => {
      if (disposed) fn();
      else unsubscribe = fn;
    });
    return () => {
      disposed = true;
      clearTimeout(refreshTimer);
      unsubscribe?.();
    };
  }, [refresh, report]);
  useEffect(() => {
    selected.current = layout.selected;
    if (layout.selected) void load(layout.selected);
  }, [layout.selected, load]);
  useEffect(() => {
    if (!layoutReady.current) return;
    const timer = setTimeout(
      () =>
        void rpc("layout.save", { content: JSON.stringify(layout) }).catch(
          report,
        ),
      300,
    );
    return () => clearTimeout(timer);
  }, [layout, report]);
  useEffect(() => {
    const timer = setInterval(() => {
      if (document.visibilityState !== "hidden") void refresh();
    }, 15000);
    return () => clearInterval(timer);
  }, [refresh]);
  const select = (
    id: string,
    tab?: import("../types").Tab,
    sessionId?: string,
  ) => {
    if (layout.selected) void notes.flush(layout.selected);
    setLoadError("");
    setLayout((l) => ({
      ...l,
      selected: id,
      view: "workspace",
      tabs: tab ? { ...l.tabs, [id]: tab } : l.tabs,
      sessions: sessionId ? { ...l.sessions, [id]: sessionId } : l.sessions,
    }));
  };
  const loadEvents = useCallback(
    async (id: string) => {
      try {
        const list = await rpc<AgentEvent[]>("session.events", { id });
        setEvents((all) => {
          const byID = new Map(list.map((e) => [e.id, e]));
          for (const e of all[id] ?? []) byID.set(e.id, e);
          return {
            ...all,
            [id]: [...byID.values()].sort((a, b) =>
              a.createdAt.localeCompare(b.createdAt),
            ),
          };
        });
      } catch (e) {
        report(e);
      }
    },
    [report],
  );
  const integrations = useIntegrationSync(
    boot,
    refresh,
    layout.view === "workspace" ? layout.selected : undefined,
  );
  const [permissionsOpen, setPermissionsOpen] = useState(false);
  const created = async (w: Workspace) => {
    await refresh();
    select(w.id);
  };
  return {
    ...integrations,
    permissionsOpen,
    setPermissionsOpen,
    boot,
    layout,
    setLayout,
    details,
    detail: layout.selected ? details[layout.selected] : undefined,
    events,
    error,
    setError,
    loadError,
    offline,
    report,
    load,
    refresh,
    select,
    loadEvents,
    created,
  };
}
export type Deck = ReturnType<typeof useDeck>;
