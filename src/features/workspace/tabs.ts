import {
  Bot,
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
  { name: "Tasks", icon: ListTodo },
  { name: "Notes", icon: FileText },
  { name: "Git", icon: FolderGit2 },
  { name: "PR", icon: GitPullRequest },
  { name: "Jira", icon: Ticket },
] as const;
