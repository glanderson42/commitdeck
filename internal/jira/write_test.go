package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDynamicTransitionsCommentsAndSummary(t *testing.T) {
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, _ := r.BasicAuth()
		if user != "dev" || token != "test-token" {
			t.Error("auth not used")
		}
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []any{map[string]any{"id": "73", "name": "Submit review", "to": map[string]string{"name": "Team Review"}}}})
			return
		}
		posts++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/rest/api/3/issue/PROJ-421/transitions":
			if obj(body["transition"])["id"] != "73" {
				t.Error(body)
			}
		case "/rest/api/3/issue/PROJ-421/comment":
			doc := obj(body["body"])
			if text(doc) != "PR opened: https://example.com/1\nAwaiting review.\n" || doc["type"] != "doc" {
				t.Error(body)
			}
		case "/rest/api/3/issue/PROJ-421":
			if obj(body["fields"])["summary"] != "New summary" {
				t.Error(body)
			}
		default:
			t.Error(r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Email: "dev", Token: "test-token", HTTP: server.Client()}
	ctx := context.Background()
	transitions, err := c.Transitions(ctx, "PROJ-421")
	if err != nil || len(transitions) != 1 || transitions[0].Status != "Team Review" {
		t.Fatal(transitions, err)
	}
	if err = c.Transition(ctx, "PROJ-421", "unavailable"); err == nil || posts != 0 {
		t.Fatal("invalid transition posted")
	}
	if err = c.Transition(ctx, "PROJ-421", "73"); err != nil {
		t.Fatal(err)
	}
	if err = c.Comment(ctx, "PROJ-421", "PR opened: https://example.com/1\nAwaiting review."); err != nil {
		t.Fatal(err)
	}
	if err = c.UpdateTitle(ctx, "PROJ-421", "New summary"); err != nil {
		t.Fatal(err)
	}
	if posts != 3 {
		t.Fatal(posts)
	}
}
