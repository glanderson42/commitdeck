export type Tab =
  | "Overview"
  | "Agent"
  | "Terminal"
  | "Tasks"
  | "Notes"
  | "Git"
  | "PR"
  | "Jira"
  | "Activity";
export type Attention =
  | "AGENT_RUNNING"
  | "MERGE_CONFLICT"
  | "BLOCKED"
  | "AGENT_WAITING"
  | "CHANGES_REQUESTED"
  | "CI_FAILED"
  | "CODING"
  | "READY_TO_MERGE"
  | "WAITING_FOR_REVIEW"
  | "NO_PR"
  | "IDLE";
export interface Workspace {
  lifecycle: string;
  id: string;
  externalKey: string;
  title: string;
  status: string;
  repoPath: string;
  worktreePath: string;
  branch: string;
  jiraTicketId: string;
  archived: boolean;
  createdAt: string;
  updatedAt: string;
  attention: Attention;
}
export interface Note {
  updatedBy?: string;
  sessionId?: string;
  id: string;
  workspaceId: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}
export interface Todo {
  updatedBy?: string;
  sessionId?: string;
  id: string;
  workspaceId: string;
  title: string;
  completed: boolean;
  position: number;
  createdAt: string;
  updatedAt: string;
}
export interface Session {
  imported?: boolean;
  resumeSupported?: boolean;
  sourceCwd?: string;
  id: string;
  workspaceId: string;
  provider: string;
  externalSessionId: string;
  title: string;
  purpose: string;
  status: string;
  pid: number;
  createdAt: string;
  updatedAt: string;
}
export type EventType =
  | "text"
  | "status"
  | "command"
  | "tool"
  | "file_changed"
  | "diff"
  | "permission_request"
  | "error"
  | "complete";
export interface AgentEvent {
  id: string;
  sessionId: string;
  type: EventType;
  text?: string;
  toolName?: string;
  command?: string;
  filePath?: string;
  diff?: string;
  error?: string;
  createdAt: string;
}
export interface Provider {
  id: string;
  name: string;
  available: boolean;
}
export interface Settings {
  worktreeRoot: string;
  jiraBaseUrl: string;
  jiraEmail: string;
  jiraProject: string;
  jiraTokenSet: boolean;
}
export interface Issue {
  id: string;
  key: string;
  title: string;
  description: string;
  status: string;
  priority: string;
  assignee: string;
  reporter: string;
  labels: string[];
  comments: { id: string; author: string; body: string; createdAt: string }[];
  links: { title: string; url: string }[];
  acceptanceCriteria: string;
  url: string;
  updatedAt: string;
}
export interface PullRequest {
  headOid: string;
  number: number;
  title: string;
  url: string;
  author: string;
  state: string;
  draft: boolean;
  mergeable: string;
  reviewState: string;
  reviewers: string[];
  approvals: number;
  changesRequested: number;
  reviewCommentCount: number;
  headBranch: string;
  baseBranch: string;
  checks: { name: string; status: string; url: string }[];
  reviews: { author: string; state: string; body: string }[];
  description: string;
  unresolvedComments: {
    threadId?: string;
    commentId?: number;
    author: string;
    body: string;
    path: string;
    url: string;
  }[];
}
export interface Cache<T> {
  data: T;
  updatedAt: string;
}
export interface Detail {
  terminals: TerminalTab[];
  workspace: Workspace;
  note: Note;
  todos: Todo[];
  sessions: Session[];
  permissions: PermissionRequest[];
  permissionRules: PermissionRule[];
  activity: ActivityEvent[];
  jira: Cache<Issue> | null;
  pr: Cache<PullRequest | null> | null;
}
export interface Layout {
  terminalSplit?: boolean;
  terminalHeight?: number;
  terminals?: Record<string, string>;
  gitWidth?: number;
  view: "dashboard" | "workspace";
  dashboardCollapsed: Record<string, boolean>;
  selected?: string;
  tabs: Record<string, Tab>;
  sessions: Record<string, string>;
  sidebarWidth: number;
  contextWidth: number;
  noteMode: "split" | "edit" | "preview";
  gitFiles: Record<string, string>;
  showContext: boolean;
}
export interface Bootstrap {
  activeAgents: number;
  workspaces: DashboardRow[];
  permissions: PermissionRequest[];
  notifications: ActivityEvent[];
  settings: Settings;
  providers: Provider[];
  layout: string;
  dataDir: string;
}
export interface CoreEvent {
  type:
    | "agent_event"
    | "sessions_changed"
    | "permissions_changed"
    | "activity"
    | "terminal_output"
    | "terminals_changed";
  terminalId?: string;
  sequence?: number;
  data?: string;
  event?: AgentEvent;
  workspaceId?: string;
}

export interface DashboardRow extends Workspace {
  workflowWaiting?: number;
  workflowFailed?: number;
  group: string;
  sessions: Session[];
  pr: PullRequest | null;
  tasksTotal: number;
  tasksCompleted: number;
  pendingPermissions: number;
  searchText: string;
  lastActivity: string;
}
export type Purpose =
  | "investigation"
  | "implementation"
  | "testing"
  | "review"
  | "debugging"
  | "documentation"
  | "custom";
export interface ContextOptions {
  includeWorkspaceTools?: boolean;
  maxCharacters: number;
  includeIssue: boolean;
  includeNotes: boolean;
  includeTodos: boolean;
  includeGit: boolean;
  includePR: boolean;
  includeSessionSummary: boolean;
}
export interface GeneratedContext {
  markdown: string;
  hash: string;
  generatedAt: string;
  characterCount: number;
  estimatedTokens: number;
  includedSources: string[];
  truncated: string[];
  warnings: string[];
  options: ContextOptions;
  purpose: Purpose;
}
export interface ContextSnapshot {
  id: string;
  sessionId: string;
  workspaceId: string;
  sentAt: string;
  generatedAt: string;
  markdown: string;
  hash: string;
  characterCount: number;
  includedSources: string[];
  options: ContextOptions;
}
export interface PermissionRequest {
  id: string;
  workspaceId: string;
  sessionId: string;
  provider: string;
  category: string;
  title: string;
  description: string;
  command: string;
  filePath: string;
  cwd: string;
  executable: string;
  arguments: string[];
  highRisk: boolean;
  canRespond: boolean;
  status: string;
  createdAt: string;
}
export interface PermissionRule {
  id: string;
  workspaceId: string;
  sessionId?: string;
  scope: "workspace" | "session";
  executable: string;
  argPrefix: string[];
  cwd: string;
  createdAt: string;
}
export interface ActivityEvent {
  id: string;
  workspaceId: string;
  type: string;
  title: string;
  metadata: Record<string, unknown>;
  notify: boolean;
  read: boolean;
  createdAt: string;
}

export interface TerminalTab {
  id: string;
  workspaceId: string;
  title: string;
  cwd: string;
  status: string;
  createdAt: string;
  lastUsedAt: string;
}
export interface DiscoveredSession {
  provider: string;
  externalSessionId: string;
  title: string;
  cwd: string;
  repoPath: string;
  branch: string;
  updatedAt: string;
  resumeSupported: boolean;
  source: string;
  suggestedWorkspaceId: string;
  confidence: string;
  imported: boolean;
}
export interface WorkflowStep {
  id: string;
  workflowId: string;
  position: number;
  purpose: Purpose;
  provider: string;
  status: string;
  sessionId: string;
  handoff: string;
}
export interface AgentWorkflow {
  id: string;
  workspaceId: string;
  name: string;
  status: string;
  steps: WorkflowStep[];
  createdAt: string;
  updatedAt: string;
}
