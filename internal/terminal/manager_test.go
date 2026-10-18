//go:build !windows

package terminal

import (
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/workspace"
	"encoding/base64"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPTYLifecycleCwdResizeAndRecovery(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if b, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatal(string(b), err)
		}
	}
	tree := filepath.Join(t.TempDir(), "ticket")
	if err := (gitservice.CLI{}).CreateWorktree(repo, tree, "TICKET-1", "HEAD"); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, err := db.CreateWorkspace(workspace.Workspace{Title: "Terminal", RepoPath: repo, WorktreePath: tree})
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(db, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	tab, err := m.Create(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(tab.ID, 80, 24); err != nil {
		t.Fatal(err)
	}
	if !m.Active(w.ID) {
		t.Fatal("shell not tracked")
	}
	if err = m.Resize(tab.ID, 110, 42); err != nil {
		t.Fatal(err)
	}
	if err = m.Resize(tab.ID, -1, 0); err == nil {
		t.Fatal("invalid resize accepted")
	}
	if err = m.Write(tab.ID, "printf '__CWD__'; pwd; stty size\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot := m.Snapshot(tab.ID)
		b, _ := base64.StdEncoding.DecodeString(fmt.Sprint(snapshot["data"]))
		if strings.Contains(string(b), tree) && strings.Contains(string(b), "42 110") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("PTY output/cwd/resize missing", string(b))
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.Stop(tab.ID)
	if m.Active(w.ID) {
		t.Fatal("shell still active")
	}
	tabs, err := m.List(w.ID)
	if err != nil || len(tabs) != 1 || tabs[0].Status != "stopped" {
		t.Fatal(tabs, err)
	}
	_, err = db.Exec("UPDATE terminals SET status='running'")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := New(db, func(any) {})
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	tabs, _ = recovered.List(w.ID)
	if tabs[0].Status != "stopped" {
		t.Fatal("false process survival")
	}
	if err = recovered.Delete(tab.ID); err != nil {
		t.Fatal(err)
	}
}
