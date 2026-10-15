package workspace

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/pullrequest"
	"testing"
)

func TestAttentionPriority(t *testing.T) {
	p := &pullrequest.PullRequest{State: "OPEN", Mergeable: "MERGEABLE", ReviewState: "APPROVED", Checks: []pullrequest.Check{{Status: "success"}}}
	running := []protocol.Session{{Status: "running"}}
	if Attention(nil, p) != "READY_TO_MERGE" {
		t.Fatal("ready")
	}
	p.Checks[0].Status = "pending"
	if Attention(nil, p) != "WAITING_FOR_REVIEW" {
		t.Fatal("pending")
	}
	if Attention(running, p) != "AGENT_RUNNING" {
		t.Fatal("coding")
	}
	p.Checks[0].Status = "failure"
	if Attention(running, p) != "CI_FAILED" {
		t.Fatal("CI")
	}
	p.ChangesRequested = 1
	if Attention(running, p) != "CHANGES_REQUESTED" {
		t.Fatal("reviews")
	}
	if Attention([]protocol.Session{{Status: "waiting"}}, p) != "AGENT_WAITING" {
		t.Fatal("waiting")
	}
	if Attention(nil, nil) != "NO_PR" {
		t.Fatal("no PR")
	}
}
