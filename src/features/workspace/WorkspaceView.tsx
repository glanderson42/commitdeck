import { Overview } from "./Overview";
import type { Purpose } from "../../types";
import { ResizeHandle } from "../../components/ResizeHandle";
import type { Deck } from "../../hooks/useDeck";
import type { Tab } from "../../types";
import { AgentPane } from "../agents/AgentPane";
import { GitPane } from "../git/GitPane";
import { PRPane } from "../github/PRPane";
import { JiraPane } from "../jira/JiraPane";
import { NotesPane } from "../notes/NotesPane";
import { TasksPane } from "../tasks/TasksPane";
import { ContextPanel } from "./ContextPanel";
import { WorkspaceHeader } from "./WorkspaceHeader";
export function WorkspaceView({
  deck,
  tab,
  setTab,
  providerRequest,
  requestedPurpose,
  setProviderRequest,
  onEdit,
  onDelete,
  onSettings,
  onRefresh,
}: {
  deck: Deck;
  tab: Tab;
  setTab: (tab: Tab) => void;
  providerRequest?: string;
  requestedPurpose?: Purpose;
  setProviderRequest: (v?: string) => void;
  onEdit: () => void;
  onDelete: () => void;
  onSettings: () => void;
  onRefresh: () => void;
}) {
  const { detail, boot, layout, setLayout } = deck;
  if (!detail || !boot) return null;
  const w = detail.workspace;
  const context = layout.showContext;
  return (
    <>
      <WorkspaceHeader
        deck={deck}
        tab={tab}
        setTab={setTab}
        onEdit={onEdit}
        onDelete={onDelete}
        onRefresh={onRefresh}
      />
      <div className="workspace-content">
        <div
          className="tab-content"
          role="tabpanel"
          id="workspace-tabpanel"
          aria-labelledby={`tab-${tab}`}
        >
          {tab === "Overview" && (
            <Overview
              detail={detail}
              setTab={setTab}
              refresh={deck.refresh}
              onSession={(id) => {
                setTab("Agent");
                setLayout((l) => ({
                  ...l,
                  sessions: { ...l.sessions, [w.id]: id },
                }));
              }}
            />
          )}
          {tab === "Agent" && (
            <AgentPane
              detail={detail}
              providers={boot.providers}
              events={deck.events}
              selected={layout.sessions[w.id]}
              selectSession={(id) =>
                setLayout((l) => ({
                  ...l,
                  sessions: { ...l.sessions, [w.id]: id },
                }))
              }
              loadEvents={deck.loadEvents}
              refresh={deck.refresh}
              requestedProvider={providerRequest}
              requestedPurpose={requestedPurpose}
              clearRequest={() => setProviderRequest(undefined)}
            />
          )}
          {tab === "Tasks" && (
            <TasksPane
              key={w.id}
              id={w.id}
              todos={detail.todos}
              refresh={deck.refresh}
            />
          )}
          {tab === "Notes" && (
            <NotesPane
              id={w.id}
              mode={layout.noteMode}
              setMode={(v) => setLayout((l) => ({ ...l, noteMode: v }))}
            />
          )}
          {tab === "Git" && (
            <GitPane
              key={w.id}
              workspace={w}
              refresh={deck.refresh}
              file={layout.gitFiles[w.id] ?? ""}
              setFile={(file) =>
                setLayout((l) => ({
                  ...l,
                  gitFiles: { ...l.gitFiles, [w.id]: file },
                }))
              }
            />
          )}
          {tab === "PR" && (
            <PRPane
              key={w.id}
              id={w.id}
              cache={detail.pr}
              onRefresh={() => deck.sync(w.id, "pr")}
              syncError={deck.integrationErrors[`${w.id}:pr`]}
              refresh={deck.refresh}
            />
          )}
          {tab === "Jira" && (
            <JiraPane
              key={w.id}
              workspace={w}
              cache={detail.jira}
              onRefresh={() => deck.sync(w.id, "jira")}
              syncError={deck.integrationErrors[`${w.id}:jira`]}
              refresh={deck.refresh}
              settings={() => onSettings()}
            />
          )}
        </div>
        {context && (
          <>
            <ResizeHandle
              label="Resize context panel"
              value={layout.contextWidth}
              min={220}
              max={380}
              reverse
              onChange={(v) => setLayout((l) => ({ ...l, contextWidth: v }))}
            />
            <ContextPanel deck={deck} setTab={setTab} />
          </>
        )}
      </div>
    </>
  );
}
