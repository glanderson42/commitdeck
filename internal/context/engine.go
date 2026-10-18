package context

import (
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	gitservice "commitdeck/internal/git"
	"commitdeck/internal/issuetracker"
	"commitdeck/internal/redact"
	"commitdeck/internal/workspace"
	"commitdeck/internal/workspacetools"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Engine struct {
	DB  *database.DB
	Git gitservice.GitService
}

func (e *Engine) BuildWorkspaceContext(ctx context.Context, id string, opts Options, purpose string) (*WorkspaceContext, error) {
	opts = Normalize(opts)
	w, err := e.DB.Workspace(id)
	if err != nil {
		return nil, err
	}
	c := &WorkspaceContext{Workspace: w, Options: opts, Purpose: Purpose(purpose), GeneratedAt: workspace.Now(), Todos: []workspace.Todo{}, RecentSessions: []protocol.Session{}, IncludedSources: []string{}, Truncated: []string{}, Warnings: []string{}}
	if opts.IncludeIssue {
		if cached, er := e.DB.Cache("jira", id); er == nil {
			var issue issuetracker.Issue
			if json.Unmarshal(cached.Data, &issue) == nil {
				c.Issue = &issue
				c.IncludedSources = append(c.IncludedSources, "issue")
			}
		}
	}
	if opts.IncludeNotes {
		n, er := e.DB.Note(id)
		if er != nil {
			return nil, er
		}
		c.Notes = n.Content
		c.IncludedSources = append(c.IncludedSources, "notes")
	}
	if opts.IncludeTodos {
		c.Todos, err = e.DB.Todos(id)
		if err != nil {
			return nil, err
		}
		c.IncludedSources = append(c.IncludedSources, "todos")
	}
	if opts.IncludePR {
		if cached, er := e.DB.Cache("pr", id); er == nil {
			_ = json.Unmarshal(cached.Data, &c.PullRequest)
			if c.PullRequest != nil {
				c.IncludedSources = append(c.IncludedSources, "pr")
			}
		}
	}
	if opts.IncludeSessionSummary {
		c.RecentSessions, err = e.DB.Sessions(id)
		if err != nil {
			return nil, err
		}
		if len(c.RecentSessions) > 8 {
			c.RecentSessions = c.RecentSessions[len(c.RecentSessions)-8:]
		}
		c.IncludedSources = append(c.IncludedSources, "sessions")
	}
	if opts.IncludeGit && w.RepoPath != "" {
		path := w.WorktreePath
		if path == "" {
			path = w.RepoPath
		}
		status, er := e.Git.Status(path)
		if er != nil {
			c.Warnings = append(c.Warnings, "Git unavailable; local notes, tasks, and cached integrations are still included.")
		} else {
			g := &GitContext{Branch: status.Branch, Dirty: status.Dirty, ChangedFiles: []string{}, RecentCommits: []string{}}
			paths := []string{}
			hidden := 0
			for _, f := range status.Files {
				if redact.SecretPath(f.Path) {
					hidden++
					continue
				}
				if len(g.ChangedFiles) < 40 {
					g.ChangedFiles = append(g.ChangedFiles, f.Status+" "+f.Path)
					paths = append(paths, f.Path)
				}
			}
			sort.Strings(g.ChangedFiles)
			g.Summary = fmt.Sprintf("%d changed files; %d sensitive paths excluded. File contents and full diffs are not collected.", len(status.Files), hidden)
			local, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if len(paths) > 0 {
				args := []string{"-C", status.Path, "diff", "--stat=80,40", "--no-ext-diff", "--no-textconv", "HEAD", "--"}
				if b, er := exec.CommandContext(local, "git", append(args, paths...)...).Output(); er == nil && len(b) > 0 {
					g.Summary += "\nTracked change summary:\n" + clip(string(b), 5000)
				}
			}
			cmd := exec.CommandContext(local, "git", "-C", status.Path, "log", "-5", "--format=%h %s", "--no-show-signature")
			if b, er := cmd.Output(); er == nil {
				g.RecentCommits = strings.Split(strings.TrimSpace(string(b)), "\n")
			}
			c.Git = g
			c.IncludedSources = append(c.IncludedSources, "git")
		}
	}
	c.Render()
	return c, nil
}

type section struct{ name, body string }

func (c *WorkspaceContext) Render() {
	var sections []section
	var description, criteria, feedback, pr, git, notes, tasks, completed, sessions string
	if c.Issue != nil {
		i := c.Issue
		description = i.Description
		criteria = i.AcceptanceCriteria
		if criteria == "" {
			criteria = "No explicit acceptance-criteria section was detected."
		}
		description += "\n\nIssue status: " + i.Status + " · Priority: " + i.Priority
		for _, comment := range i.Comments[max(0, len(i.Comments)-3):] {
			description += "\n\nComment — " + comment.Author + ": " + comment.Body
		}
	}
	for _, t := range c.Todos {
		if t.Completed {
			completed += "- [x] " + t.Title + "\n"
		} else {
			tasks += "- [ ] " + t.Title + "\n"
		}
	}
	notes = c.Notes
	if c.PullRequest != nil {
		p := c.PullRequest
		pr = fmt.Sprintf("PR #%d — %s\n\n%s\n\nState: %s · Review: %s · Mergeability: %s\n\n%s", p.Number, p.Title, p.URL, p.State, p.ReviewState, p.Mergeable, p.Description)
		for _, check := range p.Checks {
			pr += "\n- " + check.Name + ": " + check.Status
		}
		for _, f := range p.UnresolvedComments {
			feedback += "- " + f.Author + " (" + f.Path + "): " + f.Body + "\n"
		}
		if p.ChangesRequested > 0 || p.ReviewState == "CHANGES_REQUESTED" {
			feedback = "Changes have been requested.\n\n" + feedback
			for _, review := range p.Reviews {
				if review.State == "CHANGES_REQUESTED" && review.Body != "" {
					feedback += "- " + review.Author + ": " + review.Body + "\n"
				}
			}
		}
	}
	if c.Git != nil {
		g := c.Git
		git = fmt.Sprintf("Branch: %s\n\nDirty: %t\n\n%s\n\nChanged files:\n", g.Branch, g.Dirty, g.Summary)
		for _, f := range g.ChangedFiles {
			git += "- " + f + "\n"
		}
		git += "\nRecent commits:\n" + strings.Join(g.RecentCommits, "\n")
	}
	for _, s := range c.RecentSessions {
		sessions += fmt.Sprintf("- %s: %s (%s, %s)\n", s.Provider, s.Title, s.Purpose, s.Status)
	}
	order := []section{{"Acceptance Criteria", criteria}, {"Issue Description", description}, {"Remaining Tasks", tasks}, {"Notes", notes}, {"Unresolved PR Feedback", feedback}, {"Git", git}, {"Pull Request", pr}, {"Previous Sessions", sessions}, {"Completed Tasks", completed}}
	if c.Purpose == "review" {
		order = []section{{"Unresolved PR Feedback", feedback}, {"Acceptance Criteria", criteria}, {"Git", git}, {"Remaining Tasks", tasks}, {"Issue Description", description}, {"Notes", notes}, {"Pull Request", pr}, {"Previous Sessions", sessions}, {"Completed Tasks", completed}}
	}
	if c.Purpose == "testing" || c.Purpose == "debugging" {
		order = []section{{"Acceptance Criteria", criteria}, {"Remaining Tasks", tasks}, {"Git", git}, {"Unresolved PR Feedback", feedback}, {"Issue Description", description}, {"Notes", notes}, {"Pull Request", pr}, {"Previous Sessions", sessions}, {"Completed Tasks", completed}}
	}
	if c.Purpose == "investigation" {
		order = []section{{"Issue Description", description}, {"Acceptance Criteria", criteria}, {"Notes", notes}, {"Remaining Tasks", tasks}, {"Unresolved PR Feedback", feedback}, {"Git", git}, {"Previous Sessions", sessions}, {"Pull Request", pr}, {"Completed Tasks", completed}}
	}
	for _, s := range order {
		if strings.TrimSpace(s.body) != "" {
			s.body = redact.Text(s.body)
			sections = append(sections, s)
		}
	}
	w := c.Workspace
	header := redact.Text(fmt.Sprintf("# CommitDeck Workspace Context\n\n## Ticket\n\n%s — %s\n\nStatus: %s\n\nRepository: %s\nBranch: %s\nWorktree: %s\n\nSession purpose: %s\n", w.ExternalKey, w.Title, w.Status, w.RepoPath, w.Branch, w.WorktreePath, c.Purpose))
	maxChars := c.Options.MaxCharacters
	c.Truncated = []string{}
	footerReserve := 260 + len(sections)*90
	if len([]rune(header)) > maxChars/3 {
		header = clip(header, maxChars/3) + "\n[Ticket metadata truncated]\n"
		c.Truncated = append(c.Truncated, "Ticket")
	}
	if c.Options.IncludeWorkspaceTools {
		if maxChars < 4000 {
			c.Options.IncludeWorkspaceTools = false
			c.Warnings = append(c.Warnings, "Workspace tools require at least a 4,000 character budget; tools disabled in this snapshot.")
		} else {
			header += "\n" + workspacetools.Instructions + "\n"
			c.IncludedSources = append(c.IncludedSources, "workspaceTools")
		}
	}
	remaining := maxChars - len([]rune(header)) - footerReserve
	for _, s := range sections {
		remaining -= len([]rune("\n## " + s.name + "\n\n"))
	}
	// Preserve a useful minimum from each unresolved source. Extra space follows
	// the purpose-specific ordering, so a long issue cannot erase remaining work.
	reserves := map[string]int{}
	for _, s := range sections {
		switch s.name {
		case "Unresolved PR Feedback":
			reserves[s.name] = min(len([]rune(s.body)), max(80, remaining/3))
		case "Remaining Tasks", "Acceptance Criteria":
			reserves[s.name] = min(len([]rune(s.body)), max(40, remaining/8))
		}
	}
	var b strings.Builder
	b.WriteString(header)
	for _, s := range sections {
		heading := "\n## " + s.name + "\n\n"
		available := max(0, remaining)
		delete(reserves, s.name)
		for _, reserve := range reserves {
			available = max(0, available-reserve)
		}
		take := min(len([]rune(s.body)), available)
		b.WriteString(heading)
		b.WriteString(clip(s.body, take))
		remaining -= take
		if take < len([]rune(s.body)) {
			c.Truncated = append(c.Truncated, s.name)
			b.WriteString("\n[TRUNCATED: " + s.name + ". Review the full source in CommitDeck before proceeding.]\n")
		}
	}
	if len(c.Truncated) > 0 {
		b.WriteString("\nContext budget reached. Truncated sections: " + strings.Join(c.Truncated, ", ") + ".\n")
	}
	if feedback != "" && contains(c.Truncated, "Unresolved PR Feedback") {
		b.WriteString("\nWARNING: Unresolved review feedback is incomplete. Open PR feedback before acting.\n")
	}
	// Fragments can form a new credential-shaped pattern at their boundary.
	// Redact the assembled document, not just the individual source bodies.
	c.Markdown = redact.Text(b.String())
	if len([]rune(c.Markdown)) > maxChars { // Markers are retained even at the minimum budget.
		notice := "\n\n[CONTEXT TRUNCATED TO BUDGET. Unresolved feedback may be incomplete; inspect PR and source tabs.]\n"
		c.Markdown = clip(c.Markdown, maxChars-len([]rune(notice))) + notice
		if !contains(c.Truncated, "Budget") {
			c.Truncated = append(c.Truncated, "Budget")
		}
	}
	c.Markdown = redact.Text(c.Markdown)
	c.CharacterCount = len([]rune(c.Markdown))
	c.EstimatedTokens = (len(c.Markdown) + 3) / 4
	sum := sha256.Sum256([]byte(c.Markdown))
	c.Hash = hex.EncodeToString(sum[:])
	// Only the budgeted, redacted Markdown is transportable. Raw source data is never returned to the UI or persisted as context.
	c.Notes = ""
	c.Issue = nil
	c.Todos = []workspace.Todo{}
	c.Git = nil
	c.PullRequest = nil
	c.RecentSessions = []protocol.Session{}
	c.Workspace = workspace.Workspace{ID: w.ID, ExternalKey: redact.Text(w.ExternalKey), Title: redact.Text(w.Title)}
}
func clip(s string, n int) string { r := []rune(s); return string(r[:max(0, min(n, len(r)))]) }
func contains(a []string, v string) bool {
	for _, s := range a {
		if s == v {
			return true
		}
	}
	return false
}
