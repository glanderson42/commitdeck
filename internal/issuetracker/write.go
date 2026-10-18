package issuetracker

import "context"

type Transition struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type WritableIssueTracker interface {
	IssueTracker
	Transitions(context.Context, string) ([]Transition, error)
	Transition(context.Context, string, string) error
	Comment(context.Context, string, string) error
	UpdateTitle(context.Context, string, string) error
}
