package github

import (
	"commitdeck/internal/pullrequest"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
)

func unresolved(ctx context.Context, repo string, number int) ([]pullrequest.Feedback, error) {
	parts := strings.Split(repo, "/")
	cursor := ""
	out := []pullrequest.Feedback{}
	query := `query($owner:String!,$name:String!,$number:Int!,$cursor:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$cursor){nodes{isResolved comments(first:100){totalCount nodes{body path url author{login}}}} pageInfo{hasNextPage endCursor}}}}}`
	for {
		args := []string{"api", "graphql", "-f", "query=" + query, "-F", "owner=" + parts[0], "-F", "name=" + parts[1], "-F", "number=" + fmtInt(number)}
		if cursor != "" {
			args = append(args, "-F", "cursor="+cursor)
		}
		b, err := exec.CommandContext(ctx, "gh", args...).Output()
		if err != nil {
			return nil, errors.New("GitHub unresolved review feedback unavailable")
		}
		var raw map[string]any
		if err = json.Unmarshal(b, &raw); err != nil {
			return nil, err
		}
		if len(arr(raw["errors"])) > 0 {
			return nil, errors.New("GitHub could not read review threads")
		}
		threads := obj(obj(obj(obj(raw["data"])["repository"])["pullRequest"])["reviewThreads"])
		for _, v := range arr(threads["nodes"]) {
			thread := obj(v)
			resolved, _ := thread["isResolved"].(bool)
			if resolved {
				continue
			}
			comments := obj(thread["comments"])
			for _, value := range arr(comments["nodes"]) {
				c := obj(value)
				out = append(out, pullrequest.Feedback{Author: str(obj(c["author"]), "login"), Body: str(c, "body"), Path: str(c, "path"), URL: str(c, "url")})
			}
			if total, _ := comments["totalCount"].(float64); int(total) > len(arr(comments["nodes"])) {
				out = append(out, pullrequest.Feedback{Author: "CommitDeck", Body: "WARNING: Additional comments in this unresolved thread are not cached. Open GitHub for the complete discussion."})
			}
		}
		info := obj(threads["pageInfo"])
		more, _ := info["hasNextPage"].(bool)
		if !more {
			return out, nil
		}
		next := str(info, "endCursor")
		if next == "" || next == cursor {
			return nil, errors.New("GitHub review pagination did not advance")
		}
		cursor = next
	}
}
