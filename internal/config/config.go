package config

import (
	"os"
	"path/filepath"
)

func DataDir() (string, error) {
	if p := os.Getenv("COMMITDECK_DATA_DIR"); p != "" {
		return filepath.Abs(p)
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "commitdeck"), nil
}
func WorktreeRoot() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Developer", ".commitdeck", "worktrees")
}
