import { Check, GripVertical, ListTodo, Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Empty, ErrorBox } from "../../components/ui";
import { message, rpc } from "../../services/api";
import type { Todo } from "../../types";
export function TasksPane({
  id,
  todos,
  refresh,
}: {
  id: string;
  todos: Todo[];
  refresh: () => Promise<void>;
}) {
  const [items, setItems] = useState(todos);
  const [title, setTitle] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dragged, setDragged] = useState("");
  useEffect(() => {
    setItems(todos);
    setTitle("");
    setError("");
  }, [todos, id]);
  const mutate = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
      await refresh();
    } catch (e) {
      setError(message(e));
      setItems(todos);
    } finally {
      setBusy(false);
    }
  };
  const save = (t: Todo) => void mutate(() => rpc("todo.save", t));
  const reorder = (from: number, to: number) => {
    if (
      busy ||
      from < 0 ||
      from >= items.length ||
      to < 0 ||
      to >= items.length ||
      from === to
    )
      return;
    const list = [...items];
    list.splice(to, 0, list.splice(from, 1)[0]);
    setItems(list);
    void mutate(() =>
      rpc("todo.reorder", { workspaceId: id, ids: list.map((t) => t.id) }),
    );
  };
  const completed = items.filter((t) => t.completed).length;
  return (
    <div className="tasks-panel">
      <div className="panel-toolbar">
        <div className="toolbar-title">
          <ListTodo size={16} />
          Implementation
        </div>
        <span className="muted">
          {completed} / {items.length} completed
        </span>
      </div>
      <div className="tasks-content">
        <div className="progress-track">
          <div
            style={{
              width: `${items.length ? (completed / items.length) * 100 : 0}%`,
            }}
          />
        </div>
        {error && <ErrorBox error={error} />}
        {items.length === 0 && (
          <Empty icon={<ListTodo size={22} />} title="A plan for this ticket">
            <p>Break the work into steps. Keep your place between sessions.</p>
          </Empty>
        )}
        <div className="todo-list" role="list" aria-label="Workspace tasks">
          {items.map((todo, i) => (
            <div
              role="listitem"
              key={todo.id}
              className={`todo-row ${todo.completed ? "done" : ""} ${dragged === todo.id ? "dragging" : ""}`}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                reorder(
                  items.findIndex((t) => t.id === dragged),
                  i,
                );
                setDragged("");
              }}
            >
              <button
                className="drag-handle"
                aria-label={`Move ${todo.title}. Alt plus arrow keys to reorder.`}
                draggable={!busy}
                onDragStart={(e) => {
                  e.dataTransfer.setData("text/plain", todo.id);
                  e.dataTransfer.effectAllowed = "move";
                  setDragged(todo.id);
                }}
                onDragEnd={() => setDragged("")}
                onKeyDown={(e) => {
                  if (
                    e.altKey &&
                    (e.key === "ArrowUp" || e.key === "ArrowDown")
                  ) {
                    e.preventDefault();
                    reorder(i, i + (e.key === "ArrowUp" ? -1 : 1));
                  } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                    e.preventDefault();
                    const rows = e.currentTarget
                      .closest(".todo-list")
                      ?.querySelectorAll<HTMLButtonElement>(".drag-handle");
                    rows?.[
                      Math.max(
                        0,
                        Math.min(
                          items.length - 1,
                          i + (e.key === "ArrowUp" ? -1 : 1),
                        ),
                      )
                    ]?.focus();
                  }
                }}
              >
                <GripVertical size={15} />
              </button>
              <button
                className="todo-check"
                role="checkbox"
                aria-checked={todo.completed}
                aria-label={`Complete ${todo.title}`}
                disabled={busy}
                onClick={() => {
                  setItems((list) =>
                    list.map((t) =>
                      t.id === todo.id ? { ...t, completed: !t.completed } : t,
                    ),
                  );
                  save({ ...todo, completed: !todo.completed });
                }}
              >
                {todo.completed && <Check size={13} />}
              </button>
              <input
                aria-label={`Task ${i + 1}`}
                defaultValue={todo.title}
                key={`${todo.id}-${todo.title}`}
                onBlur={(e) => {
                  if (e.target.value.trim() && e.target.value !== todo.title)
                    save({ ...todo, title: e.target.value });
                  else e.target.value = todo.title;
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") e.currentTarget.blur();
                  if (e.key === "Escape") {
                    e.currentTarget.value = todo.title;
                    e.currentTarget.blur();
                  }
                }}
              />
              <button
                className="icon-button delete-task"
                aria-label={`Delete ${todo.title}`}
                disabled={busy}
                onClick={() =>
                  void mutate(() =>
                    rpc("todo.delete", { workspaceId: id, id: todo.id }),
                  )
                }
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
        <form
          className="add-task"
          onSubmit={(e) => {
            e.preventDefault();
            if (!title.trim() || busy) return;
            const value = title;
            void mutate(async () => {
              await rpc("todo.save", { workspaceId: id, title: value });
              setTitle("");
            });
          }}
        >
          <Plus size={16} />
          <input
            aria-label="New task title"
            placeholder="Add a task…"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <button
            disabled={busy || !title.trim()}
            type="submit"
            className="small-button"
          >
            Add <kbd>↵</kbd>
          </button>
        </form>
        <p className="hint">
          Drag to reorder · Alt + ↑ ↓ on a handle · Enter to finish editing
        </p>
      </div>
    </div>
  );
}
