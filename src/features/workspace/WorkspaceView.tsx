import { useState } from "react";
import { TerminalPane } from "../terminal/TerminalPane";
import { SendToAgent } from "../agents/SendToAgent";
import { StartWork } from "./StartWork";
import { ActivityTimeline } from "../activity/ActivityTimeline";
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
  dailyAction,
  onAction,
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
  dailyAction?: string;
  onAction: (action?: string) => void;
}) {
  const [sendText, setSendText] = useState<string>();
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
        onAction={onAction}
      />
      <div className="workspace-content">
        <div className="workspace-editor-stack">
          <div className="workspace-editor">
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
                  providers={boot.providers}
                  onAction={onAction}
                  workflowIntent={dailyAction === "workflow"}
                  onHandled={() => onAction(undefined)}
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
                  onImport={() => onAction("import")}
                  clearRequest={() => setProviderRequest(undefined)}
                />
              )}
              {tab === "Terminal" && (
                <TerminalPane
                  key={w.id}
                  detail={detail}
                  refresh={deck.refresh}
                  onSend={setSendText}
                  selected={layout.terminals?.[w.id]}
                  onSelect={(id) =>
                    setLayout((l) => ({
                      ...l,
                      terminals: { ...l.terminals, [w.id]: id },
                    }))
                  }
                />
              )}
              {tab === "Activity" && (
                <div className="activity-page">
                  <ActivityTimeline events={detail.activity} />
                </div>
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
                  updatedBy={detail.note.updatedBy}
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
                  cleanupIntent={dailyAction === "cleanup"}
                  onHandled={() => onAction(undefined)}
                  width={layout.gitWidth ?? 250}
                  setWidth={(width) =>
                    setLayout((l) => ({ ...l, gitWidth: width }))
                  }
                  onSend={setSendText}
                  onTerminal={() => setTab("Terminal")}
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
                  disabled={w.archived || !w.repoPath}
                  onSend={setSendText}
                  onJira={() => {
                    setTab("Jira");
                    onAction(
                      detail.pr?.data?.state === "MERGED"
                        ? "jira:Done"
                        : "jira:In Review",
                    );
                  }}
                  createIntent={dailyAction === "create-pr"}
                  onHandled={() => onAction(undefined)}
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
                  intent={
                    dailyAction?.startsWith("jira:")
                      ? dailyAction.slice(5)
                      : undefined
                  }
                  onHandled={() => onAction(undefined)}
                />
              )}
            </div>
          </div>
          {layout.terminalSplit && tab !== "Terminal" && (
            <>
              <ResizeHandle
                label="Resize terminal panel"
                horizontal
                reverse
                value={layout.terminalHeight ?? 260}
                min={160}
                max={600}
                onChange={(v) =>
                  setLayout((l) => ({ ...l, terminalHeight: v }))
                }
              />
              <div
                className="terminal-dock"
                style={{ height: layout.terminalHeight ?? 260 }}
              >
                <TerminalPane
                  autoFocus={false}
                  key={w.id}
                  detail={detail}
                  refresh={deck.refresh}
                  onSend={setSendText}
                  selected={layout.terminals?.[w.id]}
                  onSelect={(id) =>
                    setLayout((l) => ({
                      ...l,
                      terminals: { ...l.terminals, [w.id]: id },
                    }))
                  }
                />
              </div>
            </>
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
      {dailyAction === "start-work" && (
        <StartWork
          workspace={w}
          onClose={() => onAction(undefined)}
          onStarted={async (terminal, jira) => {
            await deck.refresh();
            setTab("Overview");
            if (terminal) await deck.newTerminal(w.id);
            if (jira) {
              setTab("Jira");
              onAction("jira:In Progress");
            } else onAction(undefined);
          }}
        />
      )}
      {sendText !== undefined && (
        <SendToAgent
          detail={detail}
          providers={boot.providers}
          text={sendText}
          onClose={() => setSendText(undefined)}
          onSent={(s) => {
            setSendText(undefined);
            deck.select(w.id, "Agent", s.id);
            void deck.refresh();
          }}
        />
      )}
    </>
  );
}
