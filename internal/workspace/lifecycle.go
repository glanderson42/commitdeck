package workspace

import "commitdeck/internal/pullrequest"

// DevelopmentState describes local progress independently of Jira's workflow.
func DevelopmentState(w Workspace, p *pullrequest.PullRequest) string {
	if w.Archived {
		return "ARCHIVED"
	}
	if w.Lifecycle == "DONE" || w.Lifecycle == "BLOCKED" {
		return w.Lifecycle
	}
	if p != nil {
		if p.State == "MERGED" {
			return "MERGED"
		}
		if p.State == "OPEN" {
			if p.ChangesRequested > 0 || p.ReviewState == "CHANGES_REQUESTED" {
				return "CHANGES_REQUESTED"
			}
			if !p.Draft && p.ReviewState == "APPROVED" && p.Mergeable == "MERGEABLE" {
				ready := true
				for _, c := range p.Checks {
					if c.Status != "success" {
						ready = false
					}
				}
				if ready {
					return "READY_TO_MERGE"
				}
			}
			if len(p.Reviewers) > 0 || len(p.Reviews) > 0 {
				return "IN_REVIEW"
			}
			return "PR_OPEN"
		}
	}
	if w.WorktreePath != "" {
		return "CODING"
	}
	return "BACKLOG"
}
