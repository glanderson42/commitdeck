import { ArrowRight, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "../../components/ui";
export interface Command {
  id: string;
  label: string;
  group: string;
  hint?: string;
  searchText?: string;
  run: () => void;
}
export function CommandPalette({
  commands,
  onClose,
}: {
  commands: Command[];
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const matches = useMemo(
    () =>
      commands.filter((c) =>
        fuzzyMatch(query, `${c.label} ${c.group} ${c.searchText ?? ""}`),
      ),

    [commands, query],
  );
  useEffect(() => {
    input.current?.focus();
  }, []);
  useEffect(() => {
    document
      .getElementById(`command-${active}`)
      ?.scrollIntoView({ block: "nearest" });
  }, [active]);
  const execute = (c?: Command) => {
    if (c) {
      onClose();
      c.run();
    }
  };
  return (
    <Modal title="Command palette" onClose={onClose}>
      <div className="palette-search">
        <Search size={19} />
        <input
          ref={input}
          aria-label="Search commands and workspaces"
          role="combobox"
          aria-expanded="true"
          aria-controls="command-results"
          aria-activedescendant={
            matches[active] ? `command-${active}` : undefined
          }
          value={query}
          placeholder="Search workspaces or type a command…"
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || e.key === "ArrowUp") {
              e.preventDefault();
              setActive((a) =>
                Math.max(
                  0,
                  Math.min(
                    matches.length - 1,
                    a + (e.key === "ArrowDown" ? 1 : -1),
                  ),
                ),
              );
            }
            if (e.key === "Enter") {
              e.preventDefault();
              execute(matches[active]);
            }
          }}
        />
        <kbd>esc</kbd>
      </div>
      <div className="command-results" role="listbox" id="command-results">
        {matches.length === 0 && <p className="muted">No matching commands.</p>}
        {matches.map((c, i) => (
          <div key={c.id}>
            {i === 0 || matches[i - 1].group !== c.group ? (
              <div className="section-label">{c.group}</div>
            ) : null}
            <button
              role="option"
              aria-selected={i === active}
              id={`command-${i}`}
              className={`command-row ${i === active ? "active" : ""}`}
              onMouseMove={() => setActive(i)}
              onClick={() => execute(c)}
            >
              <span>{c.label}</span>
              {c.hint ? <kbd>{c.hint}</kbd> : <ArrowRight size={14} />}
            </button>
          </div>
        ))}
      </div>
      <div className="palette-footer">
        <span>
          <kbd>↑</kbd>
          <kbd>↓</kbd> to navigate
        </span>
        <span>
          <kbd>↵</kbd> to select
        </span>
      </div>
    </Modal>
  );
}

export function fuzzyMatch(query: string, text: string) {
  let cursor = 0;
  const target = text.toLowerCase();
  for (const ch of query.toLowerCase().replace(/\s+/g, "")) {
    const next = target.indexOf(ch, cursor);
    if (next < 0) return false;
    cursor = next + 1;
  }
  return true;
}
