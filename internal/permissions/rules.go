package permissions

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// ParseSimpleCommand deliberately rejects shell evaluation. It never executes input.
func ParseSimpleCommand(command string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	var quote rune
	started := false
	for _, r := range command {
		if strings.ContainsRune("\n\r\x00$`;|&<>(){}*?[]~\\", r) {
			return nil, errors.New("shell evaluation requires individual approval")
		}
		if r == '\'' || r == '"' {
			if quote == 0 {
				quote = r
				started = true
				continue
			}
			if quote == r {
				quote = 0
				continue
			}
		}
		if (r == ' ' || r == '\t') && quote == 0 {
			if started {
				tokens = append(tokens, b.String())
				b.Reset()
				started = false
			}
			continue
		}
		b.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote")
	}
	if started {
		tokens = append(tokens, b.String())
	}
	if len(tokens) == 0 || strings.Contains(tokens[0], "=") {
		return nil, errors.New("not a simple executable")
	}
	return tokens, nil
}
func Classify(r *Request) {
	r.HighRisk = true
	r.Arguments = []string{}
	if r.Category == "" {
		r.Category = "unknown"
	}
	if r.Command == "" {
		return
	}
	argv, err := ParseSimpleCommand(r.Command)
	if err != nil {
		return
	}
	r.Executable = argv[0]
	r.Arguments = argv[1:]
	classified := argv
	// Codex reports commands through its shell launcher. Remember the entire
	// launcher argv, but permit reuse only when the script is one plain test
	// command. No shell expression, expansion, or additional argument is allowed.
	if len(argv) == 3 && slices.Contains([]string{"/bin/sh", "/bin/bash", "/bin/zsh"}, argv[0]) && (argv[1] == "-c" || argv[1] == "-lc") {
		classified, err = ParseSimpleCommand(argv[2])
		if err != nil {
			return
		}
	}
	exe := filepath.Base(classified[0])
	if exe == "git" {
		r.Category = "git_operation"
		return
	}
	if r.Category == "unknown" {
		r.Category = "shell_command"
	}
	if r.Category != "shell_command" || len(classified) < 2 {
		return
	}
	allowed := map[string][]string{"go": {"test", "vet"}, "npm": {"test"}, "pnpm": {"test"}, "yarn": {"test"}, "cargo": {"test", "check", "clippy"}}
	if !slices.Contains(allowed[exe], classified[1]) {
		return
	}
	// Environment/exec overrides can turn an otherwise ordinary test invocation into a different action.
	for _, a := range classified[2:] {
		if strings.HasPrefix(a, "-exec") || strings.HasPrefix(a, "-toolexec") || strings.HasPrefix(a, "-overlay") || strings.HasPrefix(a, "--config") || strings.HasPrefix(a, "--prefix") || strings.HasPrefix(a, "--cwd") || strings.HasPrefix(a, "--manifest-path") {
			return
		}
	}
	r.HighRisk = false
}

// Rules remember exact executable + argv + cwd. ArgPrefix is exact in this version; no substring/glob matching.
func Matches(rule Rule, r Request) bool {
	if r.HighRisk || !r.CanRespond || rule.WorkspaceID != r.WorkspaceID || rule.Cwd != r.Cwd || rule.Executable != r.Executable {
		return false
	}
	if rule.Scope == "session" && rule.SessionID != r.SessionID {
		return false
	}
	return slices.Equal(rule.ArgPrefix, r.Arguments)
}
