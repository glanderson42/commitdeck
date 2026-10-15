package permissions_test

import (
	"commitdeck/internal/activity"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCommandMatchingAndClassification(t *testing.T) {
	for _, command := range []string{"rm -rf ./tmp", "git reset --hard", "go test ./...; rm -rf .", "go test $(whoami)", "go test -exec=sh", "npm test && curl example.com", "echo go test", "go test \"$TOKEN\"", "go test > file", "FOO=1 go test", "/bin/zsh -lc 'go test ./...; rm -rf .'", "/bin/sh -c 'go test $(whoami)'", "/bin/zsh -lc 'go test' extra", "/untrusted/zsh -c 'go test'"} {
		r := permissions.Request{Category: "shell_command", Command: command}
		permissions.Classify(&r)
		if !r.HighRisk {
			t.Fatal("unsafe reusable command", command)
		}
	}
	r := permissions.Request{WorkspaceID: "w", SessionID: "s", Category: "shell_command", Command: "go test './...'", Cwd: "/w", CanRespond: true}
	permissions.Classify(&r)
	if r.HighRisk {
		t.Fatal("test command should support a constrained rule")
	}
	rule := permissions.Rule{WorkspaceID: "w", Scope: "workspace", Executable: "go", ArgPrefix: []string{"test", "./..."}, Cwd: "/w"}
	if !permissions.Matches(rule, r) {
		t.Fatal("exact match")
	}
	r.Arguments = append(r.Arguments, "-exec=sh")
	if permissions.Matches(rule, r) {
		t.Fatal("matched additional arguments")
	}
	r.Arguments = []string{"test", "./..."}
	r.Cwd = "/other"
	if permissions.Matches(rule, r) {
		t.Fatal("cwd escaped rule")
	}
	r.Cwd = "/w"
	rule.Scope = "session"
	rule.SessionID = "different"
	if permissions.Matches(rule, r) {
		t.Fatal("session escaped rule")
	}
}
func TestPendingLifecycleScopesAndRecovery(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := t.TempDir()
	w, err := db.CreateWorkspace(workspace.Workspace{Title: "Ticket", WorktreePath: root})
	if err != nil {
		t.Fatal(err)
	}
	session, err := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex", Title: "Tests"})
	if err != nil {
		t.Fatal(err)
	}
	a := &activity.Service{Store: db}
	s := permissions.New(db, a, nil)
	var calls atomic.Int32
	request := permissions.Request{WorkspaceID: w.ID, SessionID: session.ID, Provider: "codex", Category: "shell_command", Title: "Run tests", Command: "go test ./...", Cwd: root}
	reply := func(allow bool) error {
		if allow {
			calls.Add(1)
		}
		return nil
	}
	id, err := s.Request(request, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Resolve(id, permissions.Decision{Allow: true, Scope: "once"}); err != nil {
		t.Fatal(err)
	}
	rules, _ := db.PermissionRules(w.ID)
	if len(rules) != 0 {
		t.Fatal("once persisted a rule")
	}
	if err = s.Resolve(id, permissions.Decision{Allow: true}); err == nil {
		t.Fatal("double approval")
	}
	id, err = s.Request(request, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Resolve(id, permissions.Decision{Allow: true, Scope: "workspace"}); err != nil {
		t.Fatal(err)
	}
	id, err = s.Request(request, reply)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		r, _ := db.Permission(id)
		if r.Status == "approved" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	r, _ := db.Permission(id)
	if r.Status != "approved" || calls.Load() != 3 {
		t.Fatal("remembered command did not reach provider", r, calls.Load())
	}
	unverified := request
	unverified.UnverifiedCwd = true
	unknownID, err := s.Request(unverified, reply)
	if err != nil {
		t.Fatal(err)
	}
	unknown, _ := db.Permission(unknownID)
	if !unknown.HighRisk || unknown.Status != "pending" {
		t.Fatal("unreported tool cwd reused a rule", unknown)
	}
	if err = s.Resolve(unknownID, permissions.Decision{Allow: true, Scope: "workspace"}); err == nil {
		t.Fatal("remembered unknown cwd")
	}
	if err = s.Resolve(unknownID, permissions.Decision{Allow: false}); err != nil {
		t.Fatal(err)
	}
	dangerous := request
	dangerous.Command = "rm -rf ./tmp"
	id, err = s.Request(dangerous, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Resolve(id, permissions.Decision{Allow: true, Scope: "workspace"}); err == nil {
		t.Fatal("remembered a destructive command")
	}
	if err = s.Resolve(id, permissions.Decision{Allow: false, Scope: "once"}); err != nil {
		t.Fatal(err)
	}
	r, _ = db.Permission(id)
	if r.Status != "denied" {
		t.Fatal(r)
	}
	id, err = s.Request(dangerous, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.RecoverPermissions(); err != nil {
		t.Fatal(err)
	}
	restarted := permissions.New(db, a, nil)
	if err = restarted.Resolve(id, permissions.Decision{Allow: true}); err == nil {
		t.Fatal("resumed a nonexistent approval channel")
	}
	r, _ = db.Permission(id)
	if r.Status != "expired" || r.CanRespond {
		t.Fatal(r)
	}
}

func TestConstrainedShellLauncherMatchesWholeInvocation(t *testing.T) {
	r := permissions.Request{WorkspaceID: "w", Category: "shell_command", Command: "/bin/zsh -lc 'go test ./...'", Cwd: "/w", CanRespond: true}
	permissions.Classify(&r)
	if r.HighRisk || r.Executable != "/bin/zsh" {
		t.Fatal("plain Codex test launcher should support exact rules", r)
	}
	rule := permissions.Rule{WorkspaceID: "w", Scope: "workspace", Executable: r.Executable, ArgPrefix: r.Arguments, Cwd: r.Cwd}
	if !permissions.Matches(rule, r) {
		t.Fatal("exact launcher did not match")
	}
	other := r
	other.Command = "/bin/zsh -lc 'go test ./... -exec=sh'"
	permissions.Classify(&other)
	if !other.HighRisk || permissions.Matches(rule, other) {
		t.Fatal("shell launcher widened permission")
	}
}
