import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { rpc } from "../../services/api";
import type { CoreEvent } from "../../types";
import "@xterm/xterm/css/xterm.css";
type Output = { sequence: number; data: string };
type View = {
  term: Terminal;
  fit: FitAddon;
  element: HTMLDivElement;
  sequence: number;
  loading: boolean;
  pending: Output[];
  input: Promise<unknown>;
};
const views = new Map<string, View>();
const decode = (data: string) =>
  Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
function write(view: View, event: Output) {
  if (event.sequence > view.sequence) {
    view.sequence = event.sequence;
    view.term.write(decode(event.data));
  }
}
export function terminalEvent(event: CoreEvent) {
  if (event.type !== "terminal_output" || !event.terminalId) return;
  const v = views.get(event.terminalId);
  if (!v) return;
  const e = { sequence: event.sequence!, data: event.data! };
  if (v.loading) {
    v.pending.push(e);
    return;
  }
  write(v, e);
}
export async function attachTerminal(
  id: string,
  host: HTMLDivElement,
  onError: (e: unknown) => void,
  focus = true,
) {
  let view = views.get(id);
  if (!view) {
    const [{ Terminal }, { FitAddon }] = await Promise.all([
      import("@xterm/xterm"),
      import("@xterm/addon-fit"),
    ]);
    view = views.get(id);
    if (!view) {
      const term = new Terminal({
        fontSize: 12,
        fontFamily: "Menlo, Consolas, monospace",
        scrollback: 5000,
        cursorBlink: true,
        theme: {
          background: "#101214",
          foreground: "#d6d9df",
          cursor: "#b7b4ff",
          selectionBackground: "#494567",
        },
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      const element = document.createElement("div");
      element.className = "terminal-screen";
      host.append(element);
      term.open(element);
      view = {
        term,
        fit,
        element,
        sequence: 0,
        loading: true,
        pending: [],
        input: Promise.resolve(),
      };
      views.set(id, view);
      const v = view;
      term.onData((data) => {
        v.input = v.input
          .catch(() => undefined)
          .then(() => rpc("terminal.write", { id, data }))
          .catch(onError);
      });
      term.onResize(({ cols, rows }) => {
        void rpc("terminal.resize", { id, cols, rows }).catch(onError);
      });
    }
  }
  host.append(view.element);
  view.fit.fit();
  if (focus) view.term.focus();
  const v = view;
  if (v.loading) {
    try {
      const snapshot = await rpc<Output>("terminal.snapshot", { id });
      v.term.write(decode(snapshot.data));
      v.sequence = snapshot.sequence;
      v.pending
        .sort((a, b) => a.sequence - b.sequence)
        .forEach((e) => write(v, e));
      v.pending = [];
      v.loading = false;
    } catch (e) {
      v.loading = false;
      onError(e);
    }
  }
  return view;
}
export function forgetTerminal(id: string) {
  const v = views.get(id);
  v?.term.dispose();
  v?.element.remove();
  views.delete(id);
}
