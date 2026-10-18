package jira

import (
	"bytes"
	"commitdeck/internal/issuetracker"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c Client) writeRequest(ctx context.Context, method, path string, payload, out any) error {
	if err := ValidateURL(c.BaseURL); err != nil {
		return err
	}
	if c.Email == "" || c.Token == "" {
		return errors.New("configure Jira credentials first")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("Jira unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Jira returned HTTP %d; verify write permissions and required fields", res.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out)
	}
	return nil
}
func issuePath(key string) (string, error) {
	if !issueKey.MatchString(key) {
		return "", errors.New("invalid Jira key")
	}
	return "/rest/api/3/issue/" + url.PathEscape(key), nil
}
func (c Client) Transitions(ctx context.Context, key string) ([]issuetracker.Transition, error) {
	path, err := issuePath(key)
	if err != nil {
		return nil, err
	}
	if err = ValidateURL(c.BaseURL); err != nil {
		return nil, err
	}
	var raw struct {
		Transitions []struct {
			ID, Name string
			To       struct{ Name string }
		}
	}
	if err = c.request(ctx, path+"/transitions", &raw); err != nil {
		return nil, err
	}
	out := []issuetracker.Transition{}
	for _, r := range raw.Transitions {
		out = append(out, issuetracker.Transition{ID: r.ID, Name: r.Name, Status: r.To.Name})
	}
	return out, nil
}
func (c Client) Transition(ctx context.Context, key, id string) error {
	available, err := c.Transitions(ctx, key)
	if err != nil {
		return err
	}
	found := false
	for _, t := range available {
		if t.ID == id {
			found = true
		}
	}
	if !found {
		return errors.New("transition is no longer available; refresh Jira transitions")
	}
	path, _ := issuePath(key)
	return c.writeRequest(ctx, "POST", path+"/transitions", map[string]any{"transition": map[string]string{"id": id}}, nil)
}
func (c Client) Comment(ctx context.Context, key, body string) error {
	path, err := issuePath(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" || len(body) > 32000 {
		return errors.New("comment must contain 1–32000 characters")
	}
	paragraphs := []any{}
	for _, line := range strings.Split(body, "\n") {
		content := []any{}
		if line != "" {
			content = append(content, map[string]any{"type": "text", "text": line})
		}
		paragraphs = append(paragraphs, map[string]any{"type": "paragraph", "content": content})
	}
	return c.writeRequest(ctx, "POST", path+"/comment", map[string]any{"body": map[string]any{"type": "doc", "version": 1, "content": paragraphs}}, nil)
}
func (c Client) UpdateTitle(ctx context.Context, key, title string) error {
	path, err := issuePath(key)
	if err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 255 {
		return errors.New("summary must contain 1–255 characters")
	}
	return c.writeRequest(ctx, "PUT", path, map[string]any{"fields": map[string]string{"summary": title}}, nil)
}
