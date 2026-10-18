package app

import (
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/workspace"
	"errors"
)

func workingPath(w workspace.Workspace) string {
	if w.WorktreePath != "" {
		return w.WorktreePath
	}
	return w.RepoPath
}
func (a *App) attach(id, path string) (any, error) {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	w, err := a.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	if w.WorktreePath != "" || (a.Agents.ActiveWorkspace(id) || a.Terminals.Active(id)) {
		return nil, errors.New("remove the existing worktree before changing repositories")
	}
	p, err := gitservice.Canonical(path)
	if err != nil {
		return nil, err
	}
	if err = a.Git.ValidateRepo(p); err != nil {
		return nil, err
	}
	repo, err := a.Git.Status(p)
	if err != nil {
		return nil, err
	}
	w.RepoPath = repo.Path
	w.Branch = repo.Branch
	return a.DB.UpdateWorkspace(w)
}
func (a *App) createWorktree(id, branch, base string) (any, error) {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	w, err := a.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	if a.Agents.ActiveWorkspace(id) || a.Terminals.Active(id) {
		return nil, errors.New("cancel active agents and close running terminals before changing worktrees")
	}
	if w.RepoPath == "" {
		return nil, errors.New("attach a repository first")
	}
	if branch == "" {
		branch = gitservice.BranchName(w.ExternalKey, w.Title)
	}
	target := w.WorktreePath
	if target != "" && branch != w.Branch {
		return nil, errors.New("workspace already has a worktree on another branch")
	}
	if target == "" {
		s, err := a.settings()
		if err != nil {
			return nil, err
		}
		target, err = gitservice.WorktreePath(s.WorktreeRoot, w.ExternalKey, w.ID)
		if err != nil {
			return nil, err
		}
	}
	if err = a.Git.CreateWorktree(w.RepoPath, target, branch, base); err != nil {
		return nil, err
	}
	w.Lifecycle = "CODING"
	w.WorktreePath = target
	w.Branch = branch
	return a.DB.UpdateWorkspace(w)
}
func (a *App) removeWorktree(id string, confirmed bool) (any, error) {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	if !confirmed {
		return nil, errors.New("worktree removal requires confirmation")
	}
	if a.Agents.ActiveWorkspace(id) || a.Terminals.Active(id) {
		return nil, errors.New("cancel active agents and close running terminals before removing worktree")
	}
	w, err := a.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	if w.WorktreePath == "" {
		return nil, errors.New("workspace has no worktree")
	}
	if err = a.Git.RemoveWorktree(w.RepoPath, w.WorktreePath); err != nil {
		return nil, err
	}
	w.WorktreePath = ""
	w.Branch = ""
	return a.DB.UpdateWorkspace(w)
}
