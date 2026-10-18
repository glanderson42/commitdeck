package github

import (
	"bytes"
	"commitdeck/internal/git"
	"commitdeck/internal/pullrequest"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func execute(ctx context.Context, args []string, payload any) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		cmd.Stdin = bytes.NewReader(b)
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.New("GitHub action failed; check gh authentication, access, branch protection, and repository state")
	}
	return b, nil
}
func api(ctx context.Context, method, path string, payload any) ([]byte, error) {
	args := []string{"api", "--method", method, path}
	if payload != nil {
		args = append(args, "--input", "-")
	}
	return execute(ctx, args, payload)
}

var oidRE = regexp.MustCompile(`^[a-fA-F0-9]{40,64}$`)

func (CLI) DefaultBranch(ctx context.Context, repo git.Repository) (string, error) {
	name, err := Repository(repo.Remote)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := api(ctx, "GET", "repos/"+name, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		DefaultBranch string `json:"default_branch"`
	}
	if json.Unmarshal(b, &response) != nil || response.DefaultBranch == "" || strings.HasPrefix(response.DefaultBranch, "-") {
		return "", errors.New("GitHub default branch unavailable")
	}
	return response.DefaultBranch, nil
}

func (CLI) Write(ctx context.Context, repo git.Repository, p *pullrequest.PullRequest, a pullrequest.Action) (string, error) {
	if !a.Confirm {
		return "", errors.New("review and confirm this GitHub action first")
	}
	name, err := Repository(repo.Remote)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	prefix := "repos/" + name
	if a.Action == "create" {
		if p != nil && p.State == "OPEN" {
			return "", errors.New("this branch already has a pull request")
		}
		if strings.TrimSpace(a.Title) == "" || len(a.Title) > 256 || len(a.Body) > 60000 || repo.Branch == "" || a.Base == "" || a.Base == repo.Branch || strings.HasPrefix(a.Base, "-") {
			return "", errors.New("provide a title, distinct base branch, and head branch")
		}
		b, e := api(ctx, "POST", prefix+"/pulls", map[string]any{"title": a.Title, "body": a.Body, "head": repo.Branch, "base": a.Base, "draft": a.Draft})
		if e != nil {
			return "", e
		}
		var out struct {
			URL string `json:"html_url"`
		}
		if e = json.Unmarshal(b, &out); e != nil {
			return "", e
		}
		return out.URL, nil
	}
	if p == nil || p.Number != a.Number {
		return "", errors.New("pull request changed; refresh before acting")
	}
	path := prefix + "/pulls/" + strconv.Itoa(p.Number)
	switch a.Action {
	case "edit":
		if strings.TrimSpace(a.Title) == "" || len(a.Title) > 256 || len(a.Body) > 60000 {
			return "", errors.New("invalid pull request title/body")
		}
		_, err = api(ctx, "PATCH", path, map[string]string{"title": a.Title, "body": a.Body})
	case "ready", "draft":
		args := []string{"pr", "ready", strconv.Itoa(p.Number), "--repo", name}
		if a.Action == "draft" {
			args = append(args, "--undo")
		}
		_, err = execute(ctx, args, nil)
	case "comment":
		if strings.TrimSpace(a.Body) == "" || len(a.Body) > 60000 {
			return "", errors.New("provide a comment under 60000 characters")
		}
		_, err = api(ctx, "POST", prefix+"/issues/"+strconv.Itoa(p.Number)+"/comments", map[string]string{"body": a.Body})
	case "reply", "resolve":
		found := false
		replyTo := a.CommentID
		for _, c := range p.UnresolvedComments {
			if a.Action == "reply" && c.CommentID == a.CommentID && a.CommentID > 0 {
				found = true
				if c.ReplyToID > 0 {
					replyTo = c.ReplyToID
				}
			}
			if a.Action == "resolve" && c.ThreadID == a.ThreadID && a.ThreadID != "" {
				found = true
			}
		}
		if !found {
			return "", errors.New("review thread no longer available; refresh pull request")
		}
		if a.Action == "reply" {
			if strings.TrimSpace(a.Body) == "" || len(a.Body) > 60000 {
				return "", errors.New("provide a reply under 60000 characters")
			}
			_, err = api(ctx, "POST", path+"/comments/"+strconv.Itoa(replyTo)+"/replies", map[string]string{"body": a.Body})
		} else {
			var b []byte
			b, err = api(ctx, "POST", "graphql", map[string]any{"query": `mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}`, "variables": map[string]string{"id": a.ThreadID}})
			if err == nil {
				var response struct {
					Errors []any
					Data   struct {
						Result struct {
							Thread struct {
								Resolved bool `json:"isResolved"`
							} `json:"thread"`
						} `json:"resolveReviewThread"`
					} `json:"data"`
				}
				if json.Unmarshal(b, &response) != nil || len(response.Errors) > 0 || !response.Data.Result.Thread.Resolved {
					err = errors.New("GitHub refused thread resolution")
				}
			}
		}
	case "reviewer":
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`).MatchString(a.Reviewer) {
			return "", errors.New("enter a GitHub username")
		}
		_, err = api(ctx, "POST", path+"/requested_reviewers", map[string]any{"reviewers": []string{a.Reviewer}})
	case "rerun":
		if !oidRE.MatchString(p.HeadOID) {
			return "", errors.New("refresh pull request head first")
		}
		b, e := execute(ctx, []string{"run", "list", "--repo", name, "--branch", p.HeadBranch, "--commit", p.HeadOID, "--limit", "30", "--json", "databaseId,conclusion"}, nil)
		if e != nil {
			return "", e
		}
		var runs []struct {
			ID         int    `json:"databaseId"`
			Conclusion string `json:"conclusion"`
		}
		if e = json.Unmarshal(b, &runs); e != nil {
			return "", e
		}
		count := 0
		for _, r := range runs {
			if r.ID > 0 && (r.Conclusion == "failure" || r.Conclusion == "timed_out" || r.Conclusion == "cancelled") {
				if _, e = execute(ctx, []string{"run", "rerun", fmt.Sprint(r.ID), "--repo", name, "--failed"}, nil); e != nil {
					return "", e
				}
				count++
			}
		}
		if count == 0 {
			return "", errors.New("no failed GitHub Actions runs found for this PR head")
		}
	case "merge":
		if a.HeadOID != p.HeadOID || !oidRE.MatchString(a.HeadOID) {
			return "", errors.New("PR head changed; refresh before merging")
		}
		if p.State != "OPEN" || p.Draft || p.Mergeable != "MERGEABLE" || p.ChangesRequested > 0 || p.ReviewState != "APPROVED" {
			return "", errors.New("PR is not ready to merge")
		}
		for _, c := range p.Checks {
			if c.Status != "success" {
				return "", errors.New("all checks must pass before merging")
			}
		}
		_, err = execute(ctx, []string{"pr", "merge", fmt.Sprint(p.Number), "--repo", name, "--squash", "--match-head-commit", a.HeadOID}, nil)
	default:
		return "", errors.New("unsupported GitHub action")
	}
	return p.URL, err
}
