package workspace

import (
	"testing"

	"commitdeck/internal/pullrequest"
)

func TestDevelopmentLifecycle(t *testing.T) {
	w := Workspace{Lifecycle: "BACKLOG"}
	if DevelopmentState(w, nil) != "BACKLOG" {
		t.Fatal("backlog")
	}
	w.WorktreePath = "/ticket"
	if DevelopmentState(w, nil) != "CODING" {
		t.Fatal("coding")
	}
	p := &pullrequest.PullRequest{State: "OPEN"}
	if DevelopmentState(w, p) != "PR_OPEN" {
		t.Fatal("pr")
	}
	p.Reviewers = []string{"reviewer"}
	if DevelopmentState(w, p) != "IN_REVIEW" {
		t.Fatal("review")
	}
	p.ChangesRequested = 1
	if DevelopmentState(w, p) != "CHANGES_REQUESTED" {
		t.Fatal("changes")
	}
	p.ChangesRequested = 0
	p.ReviewState = "APPROVED"
	p.Mergeable = "MERGEABLE"
	if DevelopmentState(w, p) != "READY_TO_MERGE" {
		t.Fatal("ready")
	}
	p.State = "MERGED"
	if DevelopmentState(w, p) != "MERGED" {
		t.Fatal("merged")
	}
	w.Lifecycle = "DONE"
	if DevelopmentState(w, p) != "DONE" {
		t.Fatal("done")
	}
	w.Archived = true
	if DevelopmentState(w, p) != "ARCHIVED" {
		t.Fatal("archive")
	}
}

func TestDevelopmentLifecycleOverrides(t *testing.T) {
	merged := &pullrequest.PullRequest{State: "MERGED"}
	for _, tt := range []struct {
		name      string
		workspace Workspace
		pr        *pullrequest.PullRequest
		want      string
	}{
		{"archived takes precedence over done and merged", Workspace{Archived: true, Lifecycle: "DONE"}, merged, "ARCHIVED"},
		{"done remains done with a merged PR", Workspace{Lifecycle: "DONE"}, merged, "DONE"},
		{"blocked remains blocked with a merged PR", Workspace{Lifecycle: "BLOCKED"}, merged, "BLOCKED"},
		{"blocked without PR", Workspace{Lifecycle: "BLOCKED", WorktreePath: "/ticket"}, nil, "BLOCKED"},
		{"closed unmerged PR with worktree returns to coding", Workspace{WorktreePath: "/ticket"}, &pullrequest.PullRequest{State: "CLOSED"}, "CODING"},
		{"closed unmerged PR without worktree returns to backlog", Workspace{}, &pullrequest.PullRequest{State: "CLOSED"}, "BACKLOG"},
		{"requested changes from review state", Workspace{}, &pullrequest.PullRequest{State: "OPEN", ReviewState: "CHANGES_REQUESTED"}, "CHANGES_REQUESTED"},
		{"existing review without a requested reviewer", Workspace{}, &pullrequest.PullRequest{State: "OPEN", Reviews: []pullrequest.Review{{Author: "reviewer", State: "COMMENTED"}}}, "IN_REVIEW"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DevelopmentState(tt.workspace, tt.pr); got != tt.want {
				t.Fatalf("DevelopmentState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDevelopmentLifecycleMergeReadiness(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*pullrequest.PullRequest)
		want   string
	}{
		{"all checks successful", func(p *pullrequest.PullRequest) {}, "READY_TO_MERGE"},
		{"no configured checks", func(p *pullrequest.PullRequest) { p.Checks = nil }, "READY_TO_MERGE"},
		{"draft", func(p *pullrequest.PullRequest) { p.Draft = true }, "IN_REVIEW"},
		{"review still required", func(p *pullrequest.PullRequest) { p.ReviewState = "REVIEW_REQUIRED" }, "IN_REVIEW"},
		{"merge conflict", func(p *pullrequest.PullRequest) { p.Mergeable = "CONFLICTING" }, "IN_REVIEW"},
		{"unknown mergeability", func(p *pullrequest.PullRequest) { p.Mergeable = "UNKNOWN" }, "IN_REVIEW"},
		{"pending check", func(p *pullrequest.PullRequest) { p.Checks[1].Status = "pending" }, "IN_REVIEW"},
		{"failed check", func(p *pullrequest.PullRequest) { p.Checks[1].Status = "failure" }, "IN_REVIEW"},
		{"cancelled check", func(p *pullrequest.PullRequest) { p.Checks[1].Status = "cancelled" }, "IN_REVIEW"},
		{"unknown check outcome", func(p *pullrequest.PullRequest) { p.Checks[1].Status = "" }, "IN_REVIEW"},
		{"changes requested despite approval", func(p *pullrequest.PullRequest) { p.ChangesRequested = 1 }, "CHANGES_REQUESTED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &pullrequest.PullRequest{
				State: "OPEN", ReviewState: "APPROVED", Mergeable: "MERGEABLE",
				Reviewers: []string{"reviewer"},
				Checks:    []pullrequest.Check{{Name: "lint", Status: "success"}, {Name: "tests", Status: "success"}},
			}
			tt.change(p)
			if got := DevelopmentState(Workspace{WorktreePath: "/ticket"}, p); got != tt.want {
				t.Fatalf("DevelopmentState() = %q, want %q", got, tt.want)
			}
		})
	}
}
