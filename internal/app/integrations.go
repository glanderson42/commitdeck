package app

import (
	"commitdeck/internal/pullrequest"
	"commitdeck/internal/workspace"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (a *App) importIssue(ctx context.Context, key string) (any, error) {
	tracker, err := a.issueTracker()
	if err != nil {
		return nil, err
	}
	issue, err := tracker.GetIssue(ctx, key)
	if err != nil {
		return nil, err
	}
	w, err := a.DB.CreateWorkspace(workspace.Workspace{ExternalKey: issue.Key, Title: issue.Title, Status: issue.Status, JiraTicketID: issue.ID})
	if err != nil {
		return nil, err
	}
	if err = a.DB.PutCache("jira", w.ID, issue); err != nil {
		_ = a.DB.DeleteWorkspace(w.ID)
		return nil, err
	}
	return w, nil
}
func (a *App) refreshPR(ctx context.Context, id string) (any, error) {
	w, err := a.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	if w.RepoPath == "" {
		return nil, errors.New("attach a repository first")
	}
	repo, err := a.Git.Status(workingPath(w))
	if err != nil {
		return nil, err
	}
	p, err := a.PR.FindForBranch(ctx, *repo, repo.Branch)
	if err != nil {
		return nil, err
	}
	if err = a.DB.PutCache("pr", id, p); err != nil {
		return nil, err
	}
	a.recordPR(id, p)
	return a.DB.Cache("pr", id)
}
func (a *App) recordPR(id string, p *pullrequest.PullRequest) {
	activity := a.Agents.Activity
	if p == nil {
		_ = activity.Transition(id, "pr", "none", "workspace_refreshed", "No open pull request for this branch", false)
		return
	}
	_ = activity.Transition(id, "pr", fmt.Sprint(p.Number), "pr_detected", fmt.Sprintf("PR #%d detected: %s", p.Number, p.Title), false)
	ci := "none"
	if len(p.Checks) > 0 {
		ci = "passed"
	}
	for _, check := range p.Checks {
		if check.Status == "pending" && ci != "failed" {
			ci = "pending"
		}
		if check.Status == "failure" {
			ci = "failed"
		}
	}
	_ = activity.Transition(id, "ci", ci, "ci_"+ci, "CI "+ci, ci == "failed" || ci == "passed")
	review := p.ReviewState
	if p.ChangesRequested > 0 {
		review = "CHANGES_REQUESTED"
	}
	kind := "review_updated"
	title := "Review status: " + review
	if review == "CHANGES_REQUESTED" {
		kind = "changes_requested"
		title = "Changes requested on pull request"
	}
	_ = activity.Transition(id, "reviews", review, kind, title, review == "CHANGES_REQUESTED")
	reviewers := append([]string{}, p.Reviewers...)
	sort.Strings(reviewers)
	reviewTitle := "No reviewers assigned"
	if len(reviewers) > 0 {
		reviewTitle = "Requested reviewers: " + strings.Join(reviewers, ", ")
	}
	_ = activity.Transition(id, "reviewers", strings.Join(reviewers, ","), "review_requested", reviewTitle, false)
	if summary, err := a.summaryForAttention(id); err == nil {
		_ = activity.Transition(id, "ready", summary, "attention_changed", "Workspace "+summary, summary == "READY_TO_MERGE")
	}
}
func (a *App) summaryForAttention(id string) (string, error) {
	w, err := a.DB.Workspace(id)
	if err != nil {
		return "", err
	}
	s, err := a.summary(w)
	return s.Attention, err
}
