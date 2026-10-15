package workspace

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/pullrequest"
)

func Attention(sessions []protocol.Session, p *pullrequest.PullRequest) string {
	for _, s := range sessions {
		if s.Status == "waiting" || s.Status == "waiting_permission" {
			return "AGENT_WAITING"
		}
	}
	if p != nil && p.State == "OPEN" {
		if p.ChangesRequested > 0 || p.ReviewState == "CHANGES_REQUESTED" {
			return "CHANGES_REQUESTED"
		}
		for _, c := range p.Checks {
			if c.Status == "failure" {
				return "CI_FAILED"
			}
		}
		if p.Mergeable == "CONFLICTING" {
			return "MERGE_CONFLICT"
		}
	}
	for _, s := range sessions {
		if s.Status == "running" || s.Status == "starting" {
			return "AGENT_RUNNING"
		}
	}
	if p != nil && p.State == "OPEN" {
		ready := !p.Draft && p.Mergeable == "MERGEABLE" && p.ReviewState == "APPROVED"
		for _, c := range p.Checks {
			if c.Status != "success" {
				ready = false
			}
		}
		if ready {
			return "READY_TO_MERGE"
		}
		return "WAITING_FOR_REVIEW"
	}
	if p == nil {
		return "NO_PR"
	}
	return "IDLE"
}
