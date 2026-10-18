package github

import (
	"commitdeck/internal/git"
	"commitdeck/internal/pullrequest"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func fmtInt(n int) string { return fmt.Sprint(n) }

type CLI struct{}

var repoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func Repository(remote string) (string, error) {
	var path string
	if strings.HasPrefix(remote, "git@github.com:") {
		path = strings.TrimPrefix(remote, "git@github.com:")
	} else {
		u, err := url.Parse(remote)
		if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
			return "", errors.New("origin is not a github.com remote")
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	path = strings.TrimSuffix(path, ".git")
	if !repoRE.MatchString(path) || strings.Contains(path, "..") {
		return "", errors.New("invalid GitHub repository remote")
	}
	return path, nil
}
func (c CLI) FindForBranch(ctx context.Context, repo git.Repository, branch string) (*pullrequest.PullRequest, error) {
	name, err := Repository(repo.Remote)
	if err != nil {
		return nil, err
	}
	if branch == "" {
		return nil, errors.New("workspace has no branch")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--repo", name, "--head", branch, "--state", "all", "--limit", "20", "--json", "number,title,body,url,author,state,isDraft,mergeable,reviewDecision,reviewRequests,reviews,statusCheckRollup,headRefName,baseRefName,headRefOid,isCrossRepository")
	data, err := cmd.Output()
	if err != nil {
		return nil, errors.New("GitHub unavailable; run gh auth login and check repository access")
	}
	var rows []map[string]any
	if err = json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		crossRepository, _ := r["isCrossRepository"].(bool)
		if !crossRepository && str(r, "headRefName") == branch {
			p := Normalize(r)
			count, err := reviewComments(ctx, name, p.Number)
			if err != nil {
				return nil, err
			}
			p.ReviewCommentCount = count
			p.UnresolvedComments, err = unresolved(ctx, name, p.Number)
			if err != nil {
				return nil, err
			}
			return &p, nil
		}
	}
	return nil, nil
}
func reviewComments(ctx context.Context, repo string, number int) (int, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "--paginate", "repos/"+repo+"/pulls/"+fmtInt(number)+"/comments", "--jq", "length")
	b, err := cmd.Output()
	if err != nil {
		return 0, errors.New("GitHub review comments unavailable")
	}
	total := 0
	for _, line := range strings.Fields(string(b)) {
		var n int
		_, _ = fmt.Sscan(line, &n)
		total += n
	}
	return total, nil
}
func (CLI) GetChecks(_ context.Context, p pullrequest.PullRequest) ([]pullrequest.Check, error) {
	return p.Checks, nil
}
func (CLI) GetReviews(_ context.Context, p pullrequest.PullRequest) ([]pullrequest.Review, error) {
	return p.Reviews, nil
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func obj(v any) map[string]any              { m, _ := v.(map[string]any); return m }
func arr(v any) []any                       { a, _ := v.([]any); return a }
func Normalize(m map[string]any) pullrequest.PullRequest {
	p := pullrequest.PullRequest{HeadOID: str(m, "headRefOid"), Title: str(m, "title"), URL: str(m, "url"), Author: str(obj(m["author"]), "login"), State: str(m, "state"), Mergeable: str(m, "mergeable"), ReviewState: str(m, "reviewDecision"), HeadBranch: str(m, "headRefName"), BaseBranch: str(m, "baseRefName"), Reviewers: []string{}, Checks: []pullrequest.Check{}, Reviews: []pullrequest.Review{}}
	if n, ok := m["number"].(float64); ok {
		p.Number = int(n)
	}
	p.Draft, _ = m["isDraft"].(bool)
	p.Description = str(m, "body")
	p.UnresolvedComments = []pullrequest.Feedback{}
	latest := map[string]string{}
	for _, v := range arr(m["reviews"]) {
		r := obj(v)
		review := pullrequest.Review{Author: str(obj(r["author"]), "login"), State: str(r, "state"), Body: str(r, "body")}
		p.Reviews = append(p.Reviews, review)
		if review.State == "APPROVED" || review.State == "CHANGES_REQUESTED" || review.State == "DISMISSED" {
			latest[review.Author] = review.State
		}
	}
	for _, s := range latest {
		if s == "APPROVED" {
			p.Approvals++
		}
		if s == "CHANGES_REQUESTED" {
			p.ChangesRequested++
		}
	}
	for _, v := range arr(m["reviewRequests"]) {
		r := obj(v)
		name := str(r, "login")
		if name == "" {
			name = str(r, "name")
		}
		p.Reviewers = append(p.Reviewers, name)
	}
	for _, v := range arr(m["statusCheckRollup"]) {
		c := obj(v)
		name := str(c, "name")
		state := str(c, "conclusion")
		if name == "" {
			name = str(c, "context")
			state = str(c, "state")
		}
		status := "pending"
		switch state {
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			status = "success"
		case "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
			status = "failure"
		}
		link := str(c, "detailsUrl")
		if link == "" {
			link = str(c, "targetUrl")
		}
		p.Checks = append(p.Checks, pullrequest.Check{Name: name, Status: status, URL: link})
	}
	return p
}
