package workspacetools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillInstallationRequiresConsentAndPreservesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallSkill(dir, "codex", false); err == nil {
		t.Fatal("unconfirmed skill write")
	}
	path, err := InstallSkill(dir, "codex", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil || string(b) != SkillMarkdown {
		t.Fatal(err)
	}
	if _, err = InstallSkill(dir, "codex", true); err == nil {
		t.Fatal("overwrote skill")
	}
	if _, err = InstallSkill(dir, "unknown", true); err == nil {
		t.Fatal("unknown provider")
	}
}
