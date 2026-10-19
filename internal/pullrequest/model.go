package pullrequest

import (
	"commitdeck/internal/git"
	"context"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}
type Review struct {
	Author string `json:"author"`
	State  string `json:"state"`
	Body   string `json:"body"`
}
type Feedback struct {
	ReplyToID int    `json:"replyToId"`
	ThreadID  string `json:"threadId"`
	CommentID int    `json:"commentId"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Path      string `json:"path"`
	URL       string `json:"url"`
}
type PullRequest struct {
	HeadOID            string     `json:"headOid"`
	Number             int        `json:"number"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	UnresolvedComments []Feedback `json:"unresolvedComments"`
	URL                string     `json:"url"`
	Author             string     `json:"author"`
	State              string     `json:"state"`
	Draft              bool       `json:"draft"`
	Mergeable          string     `json:"mergeable"`
	ReviewState        string     `json:"reviewState"`
	Reviewers          []string   `json:"reviewers"`
	Approvals          int        `json:"approvals"`
	ChangesRequested   int        `json:"changesRequested"`
	ReviewCommentCount int        `json:"reviewCommentCount"`
	HeadBranch         string     `json:"headBranch"`
	BaseBranch         string     `json:"baseBranch"`
	Checks             []Check    `json:"checks"`
	Reviews            []Review   `json:"reviews"`
}
type PullRequestService interface {
	FindForBranch(context.Context, git.Repository, string) (*PullRequest, error)
	GetChecks(context.Context, PullRequest) ([]Check, error)
	GetReviews(context.Context, PullRequest) ([]Review, error)
}
