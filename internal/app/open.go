package app

import (
	gitservice "commitdeck/internal/git"
	"errors"
	"os/exec"
	"runtime"
)

func openDirectory(path, action string) error {
	p, err := gitservice.Canonical(path)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch action {
	case "folder":
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", "--", p)
		case "windows":
			cmd = exec.Command("explorer.exe", p)
		default:
			cmd = exec.Command("xdg-open", p)
		}
	case "terminal":
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", "-a", "Terminal", p)
		case "windows":
			cmd = exec.Command("wt.exe", "-d", p)
		default:
			cmd = exec.Command("x-terminal-emulator", "--working-directory", p)
		}
	default:
		return errors.New("unknown open action")
	}
	return cmd.Run()
}
