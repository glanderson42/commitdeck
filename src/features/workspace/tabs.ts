import {
  Bot,
  Terminal,
  Activity,
  LayoutDashboard,
  FileText,
  FolderGit2,
  GitPullRequest,
  ListTodo,
  Ticket,
} from "lucide-react";
export const tabs = [
  { name: "Overview", icon: LayoutDashboard },
  { name: "Agent", icon: Bot },
  { name: "Terminal", icon: Terminal },
  { name: "Tasks", icon: ListTodo },
  { name: "Notes", icon: FileText },
  { name: "Git", icon: FolderGit2 },
  { name: "PR", icon: GitPullRequest },
  { name: "Jira", icon: Ticket },
  { name: "Activity", icon: Activity },
] as const;
