package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"commitdeck/internal/issuetracker"
)

type Client struct {
	BaseURL string
	Email   string
	Token   string
	HTTP    *http.Client
}

var issueKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-[0-9]+$`)

func ValidateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("Jira base URL must be HTTPS without credentials, query, or fragment")
	}
	return nil
}
func (c Client) request(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("Jira unavailable; check your connection and base URL")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Jira returned HTTP %d; check access and issue key", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}
func (c Client) GetIssue(ctx context.Context, key string) (*issuetracker.Issue, error) {
	if err := ValidateURL(c.BaseURL); err != nil {
		return nil, err
	}
	if !issueKey.MatchString(key) {
		return nil, errors.New("enter a Jira key such as PROJ-421")
	}
	if c.Token == "" || c.Email == "" {
		return nil, errors.New("configure Jira email and API token in Settings")
	}
	var raw map[string]any
	if err := c.request(ctx, "/rest/api/3/issue/"+url.PathEscape(key), &raw); err != nil {
		return nil, err
	}
	issue := Normalize(raw, c.BaseURL)
	// Issue responses contain a comment page; read the remaining pages as needed.
	f := obj(raw["fields"])
	comments := obj(f["comment"])
	total := integer(comments["total"])
	for len(issue.Comments) < total {
		var page map[string]any
		if err := c.request(ctx, fmt.Sprintf("/rest/api/3/issue/%s/comment?startAt=%d&maxResults=100", url.PathEscape(key), len(issue.Comments)), &page); err != nil {
			return nil, err
		}
		rows := array(page["comments"])
		if len(rows) == 0 {
			break
		}
		for _, v := range rows {
			issue.Comments = append(issue.Comments, comment(obj(v)))
		}
	}
	return &issue, nil
}
func integer(v any) int                     { n, _ := v.(float64); return int(n) }
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func obj(v any) map[string]any              { m, _ := v.(map[string]any); return m }
func array(v any) []any                     { a, _ := v.([]any); return a }
func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	m := obj(v)
	kind := str(m, "type")
	if kind == "text" {
		return str(m, "text")
	}
	if kind == "hardBreak" {
		return "\n"
	}
	if kind == "mention" {
		return str(obj(m["attrs"]), "text")
	}
	var b strings.Builder
	for _, c := range array(m["content"]) {
		b.WriteString(text(c))
	}
	s := b.String()
	switch kind {
	case "heading":
		level := max(1, min(6, integer(obj(m["attrs"])["level"])))
		return strings.Repeat("#", level) + " " + s + "\n"
	case "listItem":
		return "- " + strings.TrimSpace(s) + "\n"
	case "codeBlock":
		return "```\n" + s + "\n```\n"
	case "paragraph", "blockquote":
		return s + "\n"
	}
	return s
}
func comment(m map[string]any) issuetracker.Comment {
	return issuetracker.Comment{ID: str(m, "id"), Author: str(obj(m["author"]), "displayName"), Body: strings.TrimSpace(text(m["body"])), CreatedAt: str(m, "created")}
}
func Normalize(raw map[string]any, base string) issuetracker.Issue {
	f := obj(raw["fields"])
	i := issuetracker.Issue{ID: str(raw, "id"), Key: str(raw, "key"), Title: str(f, "summary"), Description: strings.TrimSpace(text(f["description"])), Status: str(obj(f["status"]), "name"), Priority: str(obj(f["priority"]), "name"), Assignee: str(obj(f["assignee"]), "displayName"), Reporter: str(obj(f["reporter"]), "displayName"), UpdatedAt: str(f, "updated"), Labels: []string{}, Comments: []issuetracker.Comment{}, Links: []issuetracker.Link{}}
	i.URL = strings.TrimRight(base, "/") + "/browse/" + url.PathEscape(i.Key)
	for _, v := range array(f["labels"]) {
		if s, ok := v.(string); ok {
			i.Labels = append(i.Labels, s)
		}
	}
	for _, v := range array(obj(f["comment"])["comments"]) {
		i.Comments = append(i.Comments, comment(obj(v)))
	}
	for _, v := range array(f["issuelinks"]) {
		link := obj(v)
		target := obj(link["outwardIssue"])
		if target == nil {
			target = obj(link["inwardIssue"])
		}
		if k := str(target, "key"); k != "" {
			i.Links = append(i.Links, issuetracker.Link{Title: k + " · " + str(obj(target["fields"]), "summary"), URL: strings.TrimRight(base, "/") + "/browse/" + url.PathEscape(k)})
		}
	}
	lines := strings.Split(i.Description, "\n")
	collect := false
	var criteria []string
	for _, line := range lines {
		trim := strings.ToLower(strings.Trim(strings.TrimSpace(line), "# :"))
		if trim == "acceptance criteria" {
			collect = true
			continue
		}
		if collect {
			if strings.HasPrefix(line, "#") {
				break
			}
			criteria = append(criteria, line)
		}
	}
	i.AcceptanceCriteria = strings.TrimSpace(strings.Join(criteria, "\n"))
	return i
}
