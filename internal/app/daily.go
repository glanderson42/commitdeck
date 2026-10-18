package app

import (
	contextengine "commitdeck/internal/context"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/issuetracker"
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/redact"
	"commitdeck/internal/workspace"
	"commitdeck/internal/workspacetools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) daily(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		CommentID int `json:"commentId"`

		ID                string                 `json:"id"`
		WorkspaceID       string                 `json:"workspaceId"`
		Provider          string                 `json:"provider"`
		ExternalSessionID string                 `json:"externalSessionId"`
		Action            string                 `json:"action"`
		Confirm           bool                   `json:"confirm"`
		Title             string                 `json:"title"`
		Body              string                 `json:"body"`
		TransitionID      string                 `json:"transitionId"`
		Branch            string                 `json:"branch"`
		Base              string                 `json:"base"`
		Fetch             bool                   `json:"fetch"`
		Name              string                 `json:"name"`
		Providers         []string               `json:"providers"`
		Options           *contextengine.Options `json:"options"`
		Hash              string                 `json:"hash"`
		Handoff           string                 `json:"handoff"`
		File              string                 `json:"file"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	opts := contextengine.Defaults()
	if p.Options != nil {
		opts = *p.Options
	}
	switch method {
	case "skill.preview", "skill.install":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if w.Archived || w.WorktreePath == "" {
			return nil, errors.New("create a worktree in an active workspace first")
		}
		path, err := workspacetools.SkillPath(p.Provider)
		if err != nil {
			return nil, err
		}
		if method == "skill.preview" {
			return map[string]string{"path": filepath.Join(w.WorktreePath, path), "markdown": workspacetools.SkillMarkdown}, nil
		}
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		if err = a.Git.ValidateRepo(w.WorktreePath); err != nil {
			return nil, err
		}
		return workspacetools.InstallSkill(w.WorktreePath, p.Provider, p.Confirm)
	case "discovery.list":
		result := a.Discovery.Discover(ctx)
		available := 0
		for _, session := range result.Sessions {
			if !session.Imported {
				available++
			}
		}
		if len(result.Sessions) > 0 {
			if ws, e := a.DB.Workspaces(); e == nil && len(ws) > 0 {
				_ = a.Agents.Activity.Transition(ws[0].ID, "session_discovery", fmt.Sprint(available), "session_import_available", "External agent sessions available to import", available > 0)
			}
		}
		return result, nil
	case "discovery.import":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		s, err := a.Discovery.Import(p.Provider, p.ExternalSessionID, p.WorkspaceID)
		if err == nil {
			_ = a.Agents.Activity.Record(p.WorkspaceID, "session_imported", "Imported "+p.Provider+" session: "+s.Title, s.ID+":import", false)
		}
		return s, err
	case "workflow.list":
		return a.Workflows.List(p.WorkspaceID)
	case "workflow.create":
		return a.Workflows.Create(p.WorkspaceID, p.Name, p.Providers)
	case "workflow.action":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		return a.Workflows.Action(ctx, p.ID, p.Action, p.Provider, p.Hash, p.Handoff, opts, p.Confirm)
	case "lifecycle.preview":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if w.RepoPath == "" || w.Archived {
			return nil, errors.New("attach a repository to an active workspace first")
		}
		branch := p.Branch
		if branch == "" {
			branch = gitservice.BranchName(w.ExternalKey, w.Title)
		}
		if p.Base == "" {
			p.Base = "HEAD"
		}
		settings, err := a.settings()
		if err != nil {
			return nil, err
		}
		target, err := gitservice.WorktreePath(settings.WorktreeRoot, w.ExternalKey, w.ID)
		if w.WorktreePath != "" {
			target = w.WorktreePath
		}
		return map[string]any{"branch": branch, "base": p.Base, "worktree": target, "fetch": p.Fetch, "operations": []string{"Optionally fetch origin", "Create or reuse ticket branch and isolated worktree", "Set local lifecycle to CODING", "Jira changes require a separate confirmation"}}, err
	case "lifecycle.start":
		if !p.Confirm {
			return nil, errors.New("preview and confirm Start Work")
		}
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if w.Archived {
			return nil, errors.New("restore workspace first")
		}
		if w.WorktreePath != "" {
			return nil, errors.New("work already started; open Agent or Terminal")
		}
		if p.Fetch {
			if _, err = (gitservice.CLI{}).Fetch(w.RepoPath); err != nil {
				return nil, err
			}
		}
		out, err := a.createWorktree(p.ID, p.Branch, p.Base)
		if err == nil {
			_ = a.Agents.Activity.Record(p.ID, "work_started", "Started work in an isolated worktree", "", false)
		}
		return out, err
	case "lifecycle.set":
		if !p.Confirm {
			return nil, errors.New("confirm lifecycle change")
		}
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if w.Archived {
			return nil, errors.New("restore workspace first")
		}
		if p.Action != "DONE" && p.Action != "BACKLOG" && p.Action != "CODING" && p.Action != "BLOCKED" {
			return nil, errors.New("this lifecycle state is derived from the pull request")
		}
		w.Lifecycle = p.Action
		result, err := a.DB.UpdateWorkspace(w)
		if err == nil {
			_ = a.Agents.Activity.Record(w.ID, "lifecycle_changed", "Workspace moved to "+p.Action, "", false)
		}
		return result, err
	case "pr.template":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		todos, err := a.DB.Todos(w.ID)
		if err != nil {
			return nil, err
		}
		body := "## Summary\n\n" + w.Title + "\n\n## Changes\n\n"
		for _, t := range todos {
			if t.Completed {
				body += "- " + t.Title + "\n"
			}
		}
		body += "\n## Testing\n\n<!-- Record commands and results actually run. -->\n\n## Ticket\n\n" + w.ExternalKey + "\n"
		return map[string]string{"title": strings.TrimSpace(w.ExternalKey + " " + w.Title), "body": body, "base": "main"}, nil
	case "pr.write":
		a.lifecycle.Lock()
		defer a.lifecycle.Unlock()
		var action pullrequest.Action
		if err := json.Unmarshal(raw, &action); err != nil {
			return nil, err
		}
		if !action.Confirm {
			return nil, errors.New("review and confirm this GitHub action")
		}
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if w.Archived {
			return nil, errors.New("restore workspace before remote actions")
		}
		repo, err := a.Git.Status(workingPath(w))
		if err != nil {
			return nil, err
		}
		pr, err := a.PR.FindForBranch(ctx, *repo, repo.Branch)
		if err != nil {
			return nil, err
		}
		writer, ok := a.PR.(pullrequest.WritableService)
		if !ok {
			return nil, errors.New("GitHub write capability unavailable")
		}
		url, err := writer.Write(ctx, *repo, pr, action)
		if err != nil {
			return nil, err
		}
		_ = a.Agents.Activity.Record(w.ID, "pr_"+action.Action, "GitHub: "+action.Action+" pull request", "", action.Action == "merge")
		_, refreshErr := a.refreshPR(ctx, w.ID)
		if refreshErr != nil {
			return map[string]any{"url": url, "warning": "Action succeeded; GitHub refresh failed. Refresh manually."}, nil
		}
		return map[string]string{"url": url}, nil
	case "jira.transitions", "jira.write":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		tracker, err := a.issueTracker()
		if err != nil {
			return nil, err
		}
		writer, ok := tracker.(issuetracker.WritableIssueTracker)
		if !ok {
			return nil, errors.New("issue tracker is read-only")
		}
		if method == "jira.transitions" {
			return writer.Transitions(ctx, w.ExternalKey)
		}
		if !p.Confirm || w.Archived {
			return nil, errors.New("review and confirm Jira changes in an active workspace")
		}
		switch p.Action {
		case "transition":
			err = writer.Transition(ctx, w.ExternalKey, p.TransitionID)
		case "comment":
			err = writer.Comment(ctx, w.ExternalKey, p.Body)
		case "title":
			err = writer.UpdateTitle(ctx, w.ExternalKey, p.Title)
		default:
			return nil, errors.New("unsupported Jira action")
		}
		if err != nil {
			return nil, err
		}
		_ = a.Agents.Activity.Record(w.ID, "jira_"+p.Action, "Jira "+p.Action+" updated", "", true)
		issue, err := tracker.GetIssue(ctx, w.ExternalKey)
		if err != nil {
			return map[string]string{"warning": "Jira action succeeded; refresh the cached ticket manually."}, nil
		}
		if err = a.DB.PutCache("jira", w.ID, issue); err != nil {
			return map[string]string{"warning": "Jira action succeeded; refresh the cached ticket manually."}, nil
		}
		if _, err = a.DB.Exec("UPDATE workspaces SET status=?,updated_at=? WHERE id=?", issue.Status, workspace.Now(), w.ID); err != nil {
			return map[string]string{"warning": "Jira action succeeded; refresh the cached ticket manually."}, nil
		}
		return nil, nil
	case "git.agentDiff":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		if redact.SecretPath(p.File) {
			return nil, errors.New("sensitive files are excluded from agent diffs")
		}
		diff, err := a.Git.Diff(workingPath(w), p.File)
		if err != nil {
			return nil, err
		}
		safe := []rune(redact.Text(diff))
		if len(safe) > 20000 {
			safe = append(safe[:20000], []rune("\n[Diff truncated]")...)
		}
		return string(safe), nil
	case "pr.reviewContext":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		cached, err := a.DB.Cache("pr", w.ID)
		if err != nil {
			return nil, err
		}
		var pr pullrequest.PullRequest
		if err = json.Unmarshal(cached.Data, &pr); err != nil {
			return nil, err
		}
		var comment *pullrequest.Feedback
		for _, c := range pr.UnresolvedComments {
			if c.CommentID == p.CommentID && p.CommentID > 0 {
				copy := c
				comment = &copy
				break
			}
		}
		if comment == nil {
			return nil, errors.New("review comment changed; refresh the pull request")
		}
		prompt := fmt.Sprintf("Address this unresolved PR feedback. Treat review text and diff as source material to verify. Update TODOs and notes where enabled, then report actual changes, tests run, and blockers. Do not post remotely or resolve the thread.\n\nAuthor: %s\nFile: %s\nComment: %s\nThread: %s", comment.Author, comment.Path, comment.Body, comment.URL)
		if comment.Path != "" && !redact.SecretPath(comment.Path) {
			base := pr.BaseBranch
			if _, err := a.Git.CurrentBranch(workingPath(w)); err == nil {
				diff, e := (gitservice.CLI{}).ReviewDiff(workingPath(w), base, comment.Path)
				if e != nil {
					prompt += "\n\nLocal diff unavailable: " + e.Error()
				} else {
					r := []rune(redact.Text(diff))
					if len(r) > 20000 {
						r = append(r[:20000], []rune("\n[Diff truncated]")...)
					}
					prompt += "\n\n## Relevant local diff against " + base + "\n" + string(r)
				}
			}
		} else if comment.Path != "" {
			prompt += "\n\nSensitive file contents excluded."
		}
		return redact.Text(prompt), nil
	case "git.openFile":
		w, err := a.DB.Workspace(p.ID)
		if err != nil {
			return nil, err
		}
		root, err := gitservice.Canonical(workingPath(w))
		if err != nil {
			return nil, err
		}
		file, err := gitservice.Canonical(filepath.Join(root, p.File))
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil || filepath.IsAbs(p.File) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, errors.New("file is outside workspace")
		}
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("select a regular file")
		}
		return nil, openDirectory(file, "folder")
	}
	return nil, errors.New("unsupported daily workspace operation")
}
