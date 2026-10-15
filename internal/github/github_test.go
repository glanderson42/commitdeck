package github

import (
	"encoding/json"
	"testing"
)

func TestRemote(t *testing.T) {
	for _, remote := range []string{"git@github.com:owner/repo.git", "https://github.com/owner/repo.git", "ssh://git@github.com/owner/repo.git"} {
		got, err := Repository(remote)
		if err != nil || got != "owner/repo" {
			t.Fatal(got, err)
		}
	}
	for _, remote := range []string{"git@evil.com:owner/repo.git", "https://github.com.evil.com/o/r", "https://github.com/../evil"} {
		if _, err := Repository(remote); err == nil {
			t.Fatal("accepted", remote)
		}
	}
}
func TestNormalization(t *testing.T) {
	var m map[string]any
	err := json.Unmarshal([]byte(`{"number":842,"title":"Retries","state":"OPEN","mergeable":"MERGEABLE","reviewDecision":"CHANGES_REQUESTED","author":{"login":"mark"},"reviews":[{"author":{"login":"sarah"},"state":"CHANGES_REQUESTED"},{"author":{"login":"sarah"},"state":"APPROVED"},{"author":{"login":"peter"},"state":"CHANGES_REQUESTED"}],"statusCheckRollup":[{"name":"lint","conclusion":"SUCCESS"},{"name":"integration","conclusion":"FAILURE"},{"name":"build","status":"IN_PROGRESS"}]}`), &m)
	if err != nil {
		t.Fatal(err)
	}
	p := Normalize(m)
	if p.Number != 842 || p.Approvals != 1 || p.ChangesRequested != 1 || len(p.Checks) != 3 || p.Checks[2].Status != "pending" {
		t.Fatalf("%+v", p)
	}
}
