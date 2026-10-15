package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudReadAndCommentPagination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, ok := r.BasicAuth()
		if !ok || user != "dev@example.com" || token != "test-token" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			if r.URL.Query().Get("startAt") != "1" {
				t.Error("wrong comment offset")
			}
			fmt.Fprint(w, `{"comments":[{"id":"2","body":"Second comment"}]}`)
			return
		}
		fmt.Fprint(w, `{"id":"42","key":"PROJ-421","fields":{"summary":"Retry","comment":{"total":2,"comments":[{"id":"1","body":"First comment"}]}}}`)
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, Email: "dev@example.com", Token: "test-token", HTTP: server.Client()}
	issue, err := client.GetIssue(context.Background(), "PROJ-421")
	if err != nil || len(issue.Comments) != 2 || issue.Comments[1].Body != "Second comment" {
		t.Fatal(issue, err)
	}
	if _, err = client.GetIssue(context.Background(), "../../secret"); err == nil {
		t.Fatal("accepted invalid issue key")
	}
}

func TestNormalizeIssue(t *testing.T) {
	var m map[string]any
	err := json.Unmarshal([]byte(`{"id":"42","key":"PROJ-421","fields":{"summary":"Invoice retry","status":{"name":"In Progress"},"priority":{"name":"High"},"assignee":{"displayName":"Mark"},"description":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Acceptance criteria"}]},{"type":"paragraph","content":[{"type":"text","text":"Retries are idempotent"}]}]},"labels":["billing"],"comment":{"comments":[{"id":"c1","author":{"displayName":"Sarah"},"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Use invoice ID"}]}]}}]}}}`), &m)
	if err != nil {
		t.Fatal(err)
	}
	i := Normalize(m, "https://example.atlassian.net")
	if i.Key != "PROJ-421" || i.Assignee != "Mark" || i.AcceptanceCriteria != "Retries are idempotent" || len(i.Comments) != 1 || i.Comments[0].Body != "Use invoice ID" {
		t.Fatalf("%+v", i)
	}
	for _, url := range []string{"http://example.com", "https://secret@example.com", "https://example.com?token=secret"} {
		if ValidateURL(url) == nil {
			t.Fatal("accepted unsafe URL", url)
		}
	}
}
