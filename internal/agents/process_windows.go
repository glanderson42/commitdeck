//go:build windows

package agents

import (
	"os/exec"
	"strconv"
)

func isolate(cmd *exec.Cmd) {}
func terminate(cmd *exec.Cmd) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
}
