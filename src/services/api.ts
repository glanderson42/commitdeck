import { invoke, isTauri } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import type { CoreEvent } from "../types";
export async function rpc<T>(method: string, params: unknown = {}): Promise<T> {
  if (isTauri()) return invoke<T>("rpc", { method, params });
  const res = await fetch("/api/rpc", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ method, params }),
  });
  if (!res.ok) throw new Error(`Local backend unavailable (${res.status})`);
  const body = await res.json();
  if (body.error) throw new Error(body.error);
  return body.result as T;
}
export async function subscribe(
  onEvent: (event: CoreEvent) => void,
  onDisconnect: () => void,
): Promise<() => void> {
  if (isTauri()) {
    const un = await listen<CoreEvent>("core-event", (e) => onEvent(e.payload));
    const off = await listen("core-disconnected", onDisconnect);
    return () => {
      un();
      off();
    };
  }
  const source = new EventSource("/api/events");
  source.onmessage = (e) => onEvent(JSON.parse(e.data));
  source.onerror = onDisconnect;
  return () => source.close();
}
export function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
export async function selectDirectory(): Promise<string | null> {
  if (!isTauri()) return null;
  const { open } = await import("@tauri-apps/plugin-dialog");
  return open({
    directory: true,
    multiple: false,
    title: "Select a Git repository",
  });
}
export async function openLink(url: string) {
  if (!/^https?:\/\//i.test(url)) return;
  if (isTauri()) {
    const { openUrl } = await import("@tauri-apps/plugin-opener");
    await openUrl(url);
  } else window.open(url, "_blank", "noopener,noreferrer");
}
