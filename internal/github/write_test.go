//go:build !windows

package github

import (
	"commitdeck/internal/git"
	"commitdeck/internal/pullrequest"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePayloadsAndMergeGuards(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload")
	args := filepath.Join(dir, "args")
	script := "#!/bin/sh\ncat > \"$CD_TEST_PAYLOAD\"\nprintf '%s\\n' \"$@\" > \"$CD_TEST_ARGS\"\nprintf '%s' \"$CD_TEST_RESPONSE\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CD_TEST_PAYLOAD", payload)
	t.Setenv("CD_TEST_ARGS", args)
	t.Setenv("CD_TEST_RESPONSE", `{"html_url":"https://github.com/o/r/pull/1"}`)
	ctx := context.Background()
	repo := git.Repository{Remote: "git@github.com:o/r.git", Branch: "PROJ-1"}
	c := CLI{}
	action := pullrequest.Action{Action: "create", Title: "PROJ-1 Fix $() `literal`", Body: "## Testing\nNot run", Base: "main", Confirm: true}
	url, err := c.Write(ctx, repo, nil, action)
	if err != nil || url != "https://github.com/o/r/pull/1" {
		t.Fatal(url, err)
	}
	b, _ := os.ReadFile(payload)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	if got["title"] != action.Title || got["head"] != "PROJ-1" || got["base"] != "main" || got["body"] != action.Body {
		t.Fatal(got)
	}
	pr := &pullrequest.PullRequest{Number: 1, State: "OPEN", Mergeable: "MERGEABLE", HeadBranch: "PROJ-1", HeadOID: strings.Repeat("a", 40), UnresolvedComments: []pullrequest.Feedback{{ThreadID: "thread-1", CommentID: 42}}}
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "merge", Number: 1, HeadOID: pr.HeadOID}); err == nil {
		t.Fatal("unconfirmed merge")
	}
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "merge", Number: 1, Confirm: true, HeadOID: strings.Repeat("b", 40)}); err == nil {
		t.Fatal("stale merge")
	}
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "reply", Number: 1, Confirm: true, CommentID: 42, Body: "Fixed with test"}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(args)
	if !strings.Contains(string(b), "pulls/1/comments/42/replies") {
		t.Fatal(string(b))
	}
	t.Setenv("CD_TEST_RESPONSE", `{"data":{"resolveReviewThread":{"thread":{"isResolved":true}}}}`)
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "resolve", Number: 1, Confirm: true, ThreadID: "thread-1"}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(payload)
	if !strings.Contains(string(b), "resolveReviewThread") {
		t.Fatal(string(b))
	}
	t.Setenv("CD_TEST_RESPONSE", `[{"databaseId":9,"conclusion":"failure"}]`)
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "rerun", Number: 1, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(args)
	if !strings.Contains(string(b), "rerun\n9") || !strings.Contains(string(b), "--failed") {
		t.Fatal(string(b))
	}
	pr.Checks = []pullrequest.Check{{Status: "failure"}}
	if _, err = c.Write(ctx, repo, pr, pullrequest.Action{Action: "merge", Number: 1, Confirm: true, HeadOID: pr.HeadOID}); err == nil {
		t.Fatal("red checks merged")
	}
}

func TestBranchLookupExcludesAnIdenticallyNamedForkBranch(t *testing.T) {
	dir := t.TempDir()
	fixture := `#!/bin/sh
 printf '%s' '[{"number":42,"title":"Other fork","headRefName":"PROJ-421","state":"OPEN","isCrossRepository":true}]'
 `
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	pr, err := (CLI{}).FindForBranch(context.Background(), git.Repository{Remote: "git@github.com:o/r.git"}, "PROJ-421")
	if err != nil || pr != nil {
		t.Fatal("fork PR must not become a target for local branch writes", pr, err)
	}
}
