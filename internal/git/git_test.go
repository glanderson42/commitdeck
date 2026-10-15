package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNamesAndStatus(t *testing.T) {
	if got := BranchName("PROJ-421", "Invoice Retry Logic"); got != "PROJ-421-invoice-retry-logic" {
		t.Fatal(got)
	}
	p, err := WorktreePath(t.TempDir(), "../../unsafe", "123456789")
	if err != nil || filepath.Base(p) != "UNSAFE-12345678" {
		t.Fatal(p, err)
	}
	f := ParseStatus(" M file with spaces\x00R  new\x00old\x00?? -flag\x00")
	if len(f) != 3 || f[0].Path != "file with spaces" || f[2].Path != "-flag" {
		t.Fatal(f)
	}
}
func TestWorktreeSafety(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%v: %s %v", args, b, e)
		}
	}
	git("init")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".gitignore")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial")
	g := CLI{}
	if err := g.ValidateRepo(t.TempDir()); err == nil {
		t.Fatal("accepted non-repo")
	}
	if err := g.ValidateRepo(repo); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "ticket")
	if err := g.CreateWorktree(repo, target, "PROJ-1-test", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if err := g.CreateWorktree(repo, target, "PROJ-1-test", "HEAD"); err != nil {
		t.Fatal("idempotence", err)
	}
	if err := os.WriteFile(filepath.Join(target, "changed"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := g.Status(target)
	if err != nil || !s.Dirty {
		t.Fatal(s, err)
	}
	diff, err := g.Diff(target, "changed")
	if err != nil || diff == "" {
		t.Fatal(diff, err)
	}
	nested := filepath.Join(target, "nested")
	if err = os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	fromSubdirectory, err := g.Diff(nested, "changed")
	if err != nil || fromSubdirectory != diff {
		t.Fatal("diff must use repository root", fromSubdirectory, err)
	}
	if _, err = g.Diff(target, "../secret"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err = g.RemoveWorktree(repo, target); err == nil {
		t.Fatal("removed dirty worktree")
	}
	if err = os.Remove(filepath.Join(target, "changed")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(target, "ignored.txt"), []byte("keep this"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = g.RemoveWorktree(repo, target); err == nil {
		t.Fatal("removed ignored files")
	}
	if err = os.Remove(filepath.Join(target, "ignored.txt")); err != nil {
		t.Fatal(err)
	}
	if err = g.RemoveWorktree(repo, target); err != nil {
		t.Fatal(err)
	}
	git("show-ref", "--verify", "refs/heads/PROJ-1-test")
}
