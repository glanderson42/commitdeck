package main

import (
	"bufio"
	"commitdeck/internal/agents/opencode"
	"commitdeck/internal/app"
	"commitdeck/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "CommitDeck:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 3 && os.Args[1] == "--agent-bridge" && os.Args[2] == "opencode" {
		err := opencode.RunBridge()
		if err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "error", "error": map[string]any{"message": err.Error()}})
		}
		return err
	}
	dir, err := config.DataDir()
	if err != nil {
		return err
	}
	var writeMu sync.Mutex
	encoder := json.NewEncoder(os.Stdout)
	write := func(v any) { writeMu.Lock(); defer writeMu.Unlock(); _ = encoder.Encode(v) }
	a, err := app.Open(dir, func(v any) { write(map[string]any{"event": v}) })
	if err != nil {
		return err
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); _ = os.Stdin.Close() }()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var wg sync.WaitGroup
	slots := make(chan struct{}, 32)
	for scanner.Scan() {
		var req struct {
			ID     uint64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer func() {
				if recover() != nil {
					write(map[string]any{"id": req.ID, "error": "Internal backend error"})
				}
			}()
			requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()
			v, e := a.Call(requestCtx, req.Method, req.Params)
			if e != nil {
				write(map[string]any{"id": req.ID, "error": e.Error()})
			} else {
				write(map[string]any{"id": req.ID, "result": v})
			}
		}()
	}
	stop()
	wg.Wait()
	return nil
}
