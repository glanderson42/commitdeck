package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type File struct {
	Staged   bool   `json:"staged"`
	Unstaged bool   `json:"unstaged"`
	Path     string `json:"path"`
	Status   string `json:"status"`
}
type Repository struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Remote string `json:"remote"`
	Files  []File `json:"files"`
	Dirty  bool   `json:"dirty"`
}
type GitService interface {
	ValidateRepo(string) error
	CurrentBranch(string) (string, error)
	Status(string) (*Repository, error)
	Diff(string, string) (string, error)
	CreateWorktree(string, string, string, string) error
	RemoveWorktree(string, string) error
}
type CLI struct{}

type limitedOutput struct {
	bytes.Buffer
	truncated bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	space := 8<<20 - b.Len()
	if n > space {
		b.truncated = true
		p = p[:max(0, space)]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func run(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var output limitedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if output.truncated {
		return "", errors.New("Git output exceeds the 8 MB preview limit; inspect this repository in a terminal")
	}
	out := output.Bytes()
	if err != nil {
		return "", fmt.Errorf("git: %s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
func Canonical(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		path = filepath.Join(home, path[2:])
	}
	if path == "" || strings.ContainsRune(path, 0) {
		return "", errors.New("a directory is required")
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}
func (CLI) ValidateRepo(path string) error {
	p, err := Canonical(path)
	if err != nil {
		return err
	}
	out, err := run(p, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "true" {
		return errors.New("select a non-bare Git repository")
	}
	return nil
}
func (CLI) CurrentBranch(path string) (string, error) {
	out, err := run(path, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "" {
		return "(detached HEAD)", nil
	}
	return b, nil
}
func ParseStatus(out string) []File {
	files := []File{}
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len(p) < 4 {
			continue
		}
		status := p[:2]
		files = append(files, File{Path: p[3:], Status: strings.TrimSpace(status), Staged: status[0] != ' ' && status[0] != '?', Unstaged: status[1] != ' '})
		if strings.ContainsAny(status, "RC") {
			i++
		}
	}
	return files
}
func (g CLI) Status(path string) (*Repository, error) {
	p, err := Canonical(path)
	if err != nil {
		return nil, err
	}
	if err = g.ValidateRepo(p); err != nil {
		return nil, err
	}
	root, err := run(p, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	p, err = Canonical(strings.TrimSpace(root))
	if err != nil {
		return nil, err
	}
	out, err := run(p, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	b, err := g.CurrentBranch(p)
	if err != nil {
		return nil, err
	}
	remote, _ := run(p, "remote", "get-url", "origin")
	files := ParseStatus(out)
	remote = strings.TrimSpace(remote)
	if u, e := url.Parse(remote); e == nil && u.Host != "" {
		u.User = nil
		u.RawQuery = ""
		u.Fragment = ""
		remote = u.String()
	}
	return &Repository{Path: p, Branch: b, Remote: remote, Files: files, Dirty: len(files) > 0}, nil
}
func (g CLI) Diff(path, file string) (string, error) {
	if file == "" || filepath.IsAbs(file) {
		return "", errors.New("select a changed file")
	}
	clean := filepath.Clean(file)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("file is outside repository")
	}
	s, err := g.Status(path)
	if err != nil {
		return "", err
	}
	path = s.Path
	var found *File
	for _, f := range s.Files {
		if f.Path == file {
			copy := f
			found = &copy
			break
		}
	}
	if found == nil {
		return "", errors.New("file is no longer changed")
	}
	if found.Status == "??" {
		p, err := Canonical(filepath.Join(path, clean))
		if err != nil {
			return "", err
		}
		root, err := Canonical(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("symlink leaves repository")
		}
		info, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return "", errors.New("preview supports regular files up to 2 MB")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		if strings.ContainsRune(string(b), 0) {
			return "Binary file\n", nil
		}
		return "--- /dev/null\n+++ b/" + file + "\n+" + strings.ReplaceAll(string(b), "\n", "\n+"), nil
	}
	staged, err := run(path, "diff", "--no-ext-diff", "--no-textconv", "--cached", "--", file)
	if err != nil {
		return "", err
	}
	unstaged, err := run(path, "diff", "--no-ext-diff", "--no-textconv", "--", file)
	if err != nil {
		return "", err
	}
	return staged + unstaged, nil
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func BranchName(key, title string) string {
	k := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(key), "-"), "-")
	if k == "" {
		k = "ticket"
	}
	k = strings.ToUpper(k)
	t := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(t) > 70 {
		t = strings.Trim(t[:70], "-")
	}
	if t == "" {
		return k
	}
	return k + "-" + t
}
func WorktreePath(root, key, id string) (string, error) {
	if root == "" {
		return "", errors.New("worktree root is required")
	}
	if strings.HasPrefix(root, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(h, root[2:])
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	slug := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(key), "-"), "-")
	if slug == "" {
		slug = "ticket"
	}
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return filepath.Join(r, strings.ToUpper(slug)+"-"+id[:min(8, len(id))]), nil
}

type Worktree struct {
	Path   string
	Branch string
}

func worktrees(path string) ([]Worktree, error) {
	out, err := run(path, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var current Worktree
	for _, line := range strings.Split(out, "\x00") {
		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				list = append(list, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		}
		if strings.HasPrefix(line, "branch refs/heads/") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	if current.Path != "" {
		list = append(list, current)
	}
	return list, nil
}
func (g CLI) CreateWorktree(repo, target, branch, base string) error {
	if err := g.ValidateRepo(repo); err != nil {
		return err
	}
	if branch == "" || strings.HasPrefix(branch, "-") {
		return errors.New("invalid branch")
	}
	if _, err := run(repo, "check-ref-format", "--branch", branch); err != nil {
		return errors.New("invalid branch name")
	}
	if base == "" {
		base = "HEAD"
	}
	if strings.HasPrefix(base, "-") {
		return errors.New("invalid base revision")
	}
	if _, err := run(repo, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		return errors.New("base must identify an existing commit")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	list, err := worktrees(repo)
	if err != nil {
		return err
	}
	for _, w := range list {
		p, e := Canonical(w.Path)
		if e != nil {
			p = w.Path
		}
		t, e := Canonical(abs)
		if e != nil {
			t = abs
		}
		if p == t {
			if w.Branch == branch {
				return nil
			}
			return errors.New("worktree already exists with a different branch")
		}
		if w.Branch == branch {
			return fmt.Errorf("branch already checked out at %s", w.Path)
		}
	}
	if _, err = os.Lstat(abs); !errors.Is(err, os.ErrNotExist) {
		return errors.New("target already exists; choose another worktree root")
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	_, exists := run(repo, "show-ref", "--verify", "refs/heads/"+branch)
	if exists == nil {
		_, err = run(repo, "worktree", "add", "--", abs, branch)
	} else {
		_, err = run(repo, "worktree", "add", "-b", branch, "--", abs, base)
	}
	return err
}
func (g CLI) RemoveWorktree(repo, target string) error {
	r, err := Canonical(repo)
	if err != nil {
		return err
	}
	t, err := Canonical(target)
	if err != nil {
		return err
	}
	list, err := worktrees(r)
	if err != nil {
		return err
	}
	valid := false
	for i, w := range list {
		p, e := Canonical(w.Path)
		if e == nil && p == t {
			if i == 0 {
				return errors.New("cannot remove the primary repository")
			}
			valid = true
		}
	}
	if !valid {
		return errors.New("path is not a registered worktree")
	}
	s, err := g.Status(t)
	if err != nil {
		return err
	}
	if s.Dirty {
		return errors.New("worktree has uncommitted changes; cleanup refused")
	}
	ignored, err := run(t, "ls-files", "--others", "-z")
	if err != nil {
		return err
	}
	if ignored != "" {
		return errors.New("worktree contains untracked or ignored files; cleanup refused")
	}
	_, err = run(r, "worktree", "remove", "--", t)
	return err
}

func (CLI) Fetch(path string) (string, error) {
	if err := (CLI{}).ValidateRepo(path); err != nil {
		return "", err
	}
	return run(path, "fetch", "origin")
}

// DefaultBranch uses the locally cached origin HEAD without making network requests.
func DefaultBranch(path string) (string, error) {
	ref, err := run(path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	ref = strings.TrimSpace(ref)
	branch := strings.TrimPrefix(ref, "origin/")
	if branch == "" || branch == ref {
		return "", errors.New("origin default branch unavailable")
	}
	return branch, nil
}
