package pullrequest

import (
	"commitdeck/internal/git"
	"context"
)

type Action struct {
	Action    string `json:"action"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Base      string `json:"base"`
	Draft     bool   `json:"draft"`
	Reviewer  string `json:"reviewer"`
	ThreadID  string `json:"threadId"`
	CommentID int    `json:"commentId"`
	HeadOID   string `json:"headOid"`
	Confirm   bool   `json:"confirm"`
}
type WritableService interface {
	Write(context.Context, git.Repository, *PullRequest, Action) (string, error)
}
