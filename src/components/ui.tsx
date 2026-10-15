import { ArrowUpRight, Inbox, X } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import { openLink } from "../services/api";
import type { Attention } from "../types";
export const attention: Record<Attention, { label: string; color: string }> = {
  AGENT_RUNNING: { label: "Agent running", color: "purple" },
  MERGE_CONFLICT: { label: "Merge conflict", color: "orange" },
  BLOCKED: { label: "Blocked", color: "orange" },
  AGENT_WAITING: { label: "Agent waiting", color: "yellow" },
  CHANGES_REQUESTED: { label: "Changes requested", color: "orange" },
  CI_FAILED: { label: "CI failed", color: "red" },
  CODING: { label: "In progress", color: "purple" },
  READY_TO_MERGE: { label: "Ready to merge", color: "green" },
  WAITING_FOR_REVIEW: { label: "Waiting for review", color: "blue" },
  NO_PR: { label: "No pull request", color: "muted" },
  IDLE: { label: "Idle", color: "muted" },
};
export function AttentionBadge({ value }: { value: Attention }) {
  const a = attention[value] ?? attention.IDLE;
  return (
    <span className={`attention ${a.color}`}>
      <i className="dot" />
      {a.label}
    </span>
  );
}
export function Empty({
  icon = <Inbox size={24} />,
  title,
  children,
}: {
  icon?: ReactNode;
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">{icon}</div>
      <h2>{title}</h2>
      <div className="empty-copy">{children}</div>
    </div>
  );
}
export function ExternalLink({
  url,
  children,
}: {
  url: string;
  children: ReactNode;
}) {
  return (
    <a
      href={/^https?:\/\//i.test(url) ? url : undefined}
      onClick={(e) => {
        e.preventDefault();
        void openLink(url);
      }}
      className="external-link"
    >
      {children}
      <ArrowUpRight size={13} />
    </a>
  );
}
export function Modal({
  title,
  onClose,
  children,
  wide = false,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  const root = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const old = document.activeElement as HTMLElement;
    const el = root.current;
    const focusable = () =>
      Array.from(
        el?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select, textarea, [tabindex="0"]',
        ) ?? [],
      );
    focusable()[0]?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        close.current();
      }
      if (e.key === "Tab") {
        const list = focusable();
        const first = list[0],
          last = list.at(-1);
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    el?.addEventListener("keydown", key);
    return () => {
      el?.removeEventListener("keydown", key);
      old?.focus();
    };
  }, []);
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={root}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`modal ${wide ? "wide" : ""}`}
      >
        <header>
          <h2>{title}</h2>
          <button
            className="icon-button"
            aria-label="Close dialog"
            onClick={onClose}
          >
            <X size={18} />
          </button>
        </header>
        {children}
      </div>
    </div>
  );
}
export function ErrorBox({
  error,
  retry,
}: {
  error: string;
  retry?: () => void;
}) {
  return (
    <div role="alert" className="error-box">
      {error}
      {retry && <button onClick={retry}>Retry</button>}
    </div>
  );
}
export function timeAgo(value: string) {
  const minutes = Math.max(
    0,
    Math.floor((Date.now() - Date.parse(value)) / 60000),
  );
  return minutes < 1
    ? "just now"
    : minutes < 60
      ? `${minutes}m ago`
      : minutes < 1440
        ? `${Math.floor(minutes / 60)}h ago`
        : `${Math.floor(minutes / 1440)}d ago`;
}
