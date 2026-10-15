//go:build !windows

package agents

import (
	"os/exec"
	"syscall"
)

func isolate(cmd *exec.Cmd)         { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func terminate(cmd *exec.Cmd) error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
