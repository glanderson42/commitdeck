//go:build !windows

package terminal

import (
	"context"
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type unixPTY struct {
	*os.File
	cmd    *exec.Cmd
	once   sync.Once
	exited atomic.Bool
}

func start(cwd string, cols, rows int) (process, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-l")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &unixPTY{File: f, cmd: cmd}, nil
}
func (p *unixPTY) Resize(cols, rows int) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}
func (p *unixPTY) Wait() error { err := p.cmd.Wait(); p.exited.Store(true); return err }
func (p *unixPTY) Close() error {
	p.once.Do(func() {
		if !p.exited.Load() {
			// Job control creates additional process groups. Only groups in this owned PTY session may be signalled.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			output, _ := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,pgid=").Output()
			cancel()
			groups := map[int]bool{}
			for _, line := range strings.Split(string(output), "\n") {
				var pid, group int
				if _, err := fmt.Sscan(line, &pid, &group); err == nil && group > 0 {
					sid, err := unix.Getsid(pid)
					if err == nil && sid == p.cmd.Process.Pid {
						groups[group] = true
					}
				}
			}
			fg, err := unix.IoctlGetInt(int(p.Fd()), unix.TIOCGPGRP)
			if err == nil && fg > 0 {
				groups[fg] = true
			}
			for group := range groups {
				if group != p.cmd.Process.Pid {
					_ = syscall.Kill(-group, syscall.SIGKILL)
				}
			}
			_ = p.cmd.Process.Kill()
		}
		_ = p.File.Close()
	})
	return nil
}
