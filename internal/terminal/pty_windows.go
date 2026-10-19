package terminal

import (
	"context"
	"fmt"
	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
	"os"
	"sync"
)

type windowsPTY struct {
	*conpty.ConPty
	once sync.Once
}

func start(cwd string, cols, rows int) (process, error) {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}
	p, err := conpty.Start(windows.EscapeArg(shell), conpty.ConPtyDimensions(cols, rows), conpty.ConPtyWorkDir(cwd))
	if err != nil {
		return nil, err
	}
	return &windowsPTY{ConPty: p}, nil
}
func (p *windowsPTY) Wait() error {
	code, err := p.ConPty.Wait(context.Background())
	if err == nil && code != 0 {
		return fmt.Errorf("shell exit %d", code)
	}
	return err
}
func (p *windowsPTY) Close() error {
	var err error
	p.once.Do(func() { err = p.ConPty.Close() })
	return err
}
