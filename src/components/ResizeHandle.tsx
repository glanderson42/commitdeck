export function ResizeHandle({
  value,
  onChange,
  min,
  max,
  reverse = false,
  label,
  horizontal = false,
}: {
  value: number;
  onChange: (v: number) => void;
  min: number;
  max: number;
  reverse?: boolean;
  label: string;
  horizontal?: boolean;
}) {
  return (
    <div
      className={`resize-handle ${horizontal ? "horizontal" : ""}`}
      role="separator"
      aria-label={label}
      aria-orientation={horizontal ? "horizontal" : "vertical"}
      aria-valuemin={min}
      aria-valuemax={max}
      aria-valuenow={value}
      tabIndex={0}
      onKeyDown={(e) => {
        if (
          e.key === (horizontal ? "ArrowUp" : "ArrowLeft") ||
          e.key === (horizontal ? "ArrowDown" : "ArrowRight")
        ) {
          e.preventDefault();
          const delta =
            (e.key === (horizontal ? "ArrowDown" : "ArrowRight") ? 10 : -10) *
            (reverse ? -1 : 1);
          onChange(Math.max(min, Math.min(max, value + delta)));
        }
      }}
      onPointerDown={(e) => {
        const start = horizontal ? e.clientY : e.clientX;
        e.currentTarget.setPointerCapture(e.pointerId);
        const el = e.currentTarget;
        const move = (ev: PointerEvent) =>
          onChange(
            Math.max(
              min,
              Math.min(
                max,
                value +
                  ((horizontal ? ev.clientY : ev.clientX) - start) *
                    (reverse ? -1 : 1),
              ),
            ),
          );
        const up = () => {
          el.removeEventListener("pointermove", move);
          el.removeEventListener("pointerup", up);
          el.removeEventListener("pointercancel", up);
        };
        el.addEventListener("pointermove", move);
        el.addEventListener("pointerup", up);
        el.addEventListener("pointercancel", up);
      }}
    />
  );
}
