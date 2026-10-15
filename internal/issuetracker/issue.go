package issuetracker

import "context"

type Comment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}
type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type Issue struct {
	ID                 string    `json:"id"`
	Key                string    `json:"key"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	Status             string    `json:"status"`
	Priority           string    `json:"priority"`
	Assignee           string    `json:"assignee"`
	Reporter           string    `json:"reporter"`
	Labels             []string  `json:"labels"`
	Comments           []Comment `json:"comments"`
	Links              []Link    `json:"links"`
	AcceptanceCriteria string    `json:"acceptanceCriteria"`
	URL                string    `json:"url"`
	UpdatedAt          string    `json:"updatedAt"`
}
type IssueTracker interface {
	GetIssue(context.Context, string) (*Issue, error)
}
