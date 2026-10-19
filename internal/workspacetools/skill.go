package workspacetools

import (
	"errors"
	"os"
	"path/filepath"
)

const SkillMarkdown = "---\nname: commitdeck-workspace\ndescription: Read and maintain the current CommitDeck ticket's TODOs, decisions, notes, and blockers through scoped local tools.\n---\n\n" + Instructions + "\nThese tools require an active CommitDeck-managed turn with workspace tools enabled. Outside that environment, report that the workspace capability is unavailable. Never search for database files or credentials as a fallback.\n"

func SkillPath(provider string) (string, error) {
	directory := map[string]string{"codex": ".agents", "claude": ".claude", "opencode": ".opencode"}[provider]
	if directory == "" {
		return "", errors.New("unknown skill provider")
	}
	return filepath.Join(directory, "skills", "commitdeck-workspace", "SKILL.md"), nil
}
func InstallSkill(worktree, provider string, confirm bool) (string, error) {
	if !confirm {
		return "", errors.New("review and confirm writing the workspace skill file")
	}
	path, err := SkillPath(provider)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err = root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", errors.New("skill file already exists or its path is unsafe; CommitDeck will not overwrite it")
	}
	_, err = f.WriteString(SkillMarkdown)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	return path, closeErr
}
