package opencode

import (
	"bufio"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/identity"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type serverAPI struct {
	base, password, cwd string
	client              *http.Client
}

func (a serverAPI) request(ctx context.Context, method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.SetBasicAuth("commitdeck", a.password)
	req.Header.Set("x-opencode-directory", a.cwd)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		return errors.New("OpenCode server unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("OpenCode server returned HTTP %d", res.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out)
	}
	return nil
}

// RunBridge translates an owned, password-protected loopback server into the same private JSONL transport as the other providers.
func RunBridge() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	if !scanner.Scan() {
		return errors.New("missing OpenCode bridge prompt")
	}
	start := protocol.Object(scanner.Bytes())
	prompt := protocol.Str(start, "prompt")
	external := protocol.Str(start, "external")
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	password := identity.ID() + identity.ID()
	cmd := exec.CommandContext(ctx, "opencode", "serve", "--hostname", "127.0.0.1", "--port", "0")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "OPENCODE_SERVER_USERNAME=commitdeck", "OPENCODE_SERVER_PASSWORD="+password)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	addresses := make(chan string, 1)
	listenURL := regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+`)
	readLog := func(r io.Reader) {
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			if strings.Contains(strings.ToLower(line), "listen") {
				if address := listenURL.FindString(line); address != "" {
					select {
					case addresses <- address:
					default:
					}
				}
			}
		}
	}
	go readLog(stdout)
	go readLog(stderr)
	var base string
	select {
	case base = <-addresses:
	case <-time.After(15 * time.Second):
		return errors.New("OpenCode did not report a loopback server address; this adapter requires the v1-compatible serve API")
	}
	api := serverAPI{base: base, password: password, cwd: cwd, client: &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 10 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	var writeMu sync.Mutex
	emit := func(v any) { writeMu.Lock(); defer writeMu.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(v) }
	return runAPI(ctx, api, scanner, prompt, external, emit)
}

func runAPI(ctx context.Context, api serverAPI, scanner *bufio.Scanner, prompt, external string, emit func(any)) error {
	var err error
	if external == "" {
		var session map[string]any
		requestCtx, stop := context.WithTimeout(ctx, 15*time.Second)
		err = api.request(requestCtx, "POST", "/session", map[string]any{"title": "CommitDeck", "permission": []any{map[string]any{"permission": "*", "pattern": "*", "action": "ask"}}}, &session)
		stop()
		if err != nil {
			return err
		}
		external = protocol.Str(session, "id")
		if external == "" {
			return errors.New("OpenCode did not return a session ID")
		}
	}
	request, err := http.NewRequestWithContext(ctx, "GET", api.base+"/event", nil)
	if err != nil {
		return err
	}
	request.SetBasicAuth("commitdeck", api.password)
	request.Header.Set("x-opencode-directory", api.cwd)
	stream, err := api.client.Do(request)
	if err != nil {
		return errors.New("could not subscribe to OpenCode events")
	}
	defer stream.Body.Close()
	if stream.StatusCode != 200 {
		return errors.New("OpenCode event API unavailable")
	}
	eventQueue := make(chan map[string]any, 64)
	streamErrors := make(chan error, 1)
	go func() {
		s := bufio.NewScanner(stream.Body)
		s.Buffer(make([]byte, 4096), 8<<20)
		for s.Scan() {
			line := s.Text()
			if strings.HasPrefix(line, "data:") {
				event := protocol.Object([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))))
				select {
				case eventQueue <- event:
				case <-ctx.Done():
					return
				}
			}
		}
		streamErrors <- errors.New("OpenCode event stream ended")
	}()
	decisions := make(chan map[string]any)
	go func() {
		defer close(decisions)
		for scanner.Scan() {
			m := protocol.Object(scanner.Bytes())
			select {
			case decisions <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	sendCtx, stop := context.WithTimeout(ctx, 30*time.Second)
	err = api.request(sendCtx, "POST", "/session/"+url.PathEscape(external)+"/prompt_async", map[string]any{"parts": []any{map[string]any{"type": "text", "text": prompt}}}, nil)
	stop()
	if err != nil {
		return err
	}
	emit(map[string]any{"type": "input_sent", "sessionID": external})
	texts := map[string]string{}
	var order []string
	emitted := map[string]bool{}
	for {
		select {
		case d, ok := <-decisions:
			if !ok {
				return nil
			}
			if protocol.Str(d, "kind") != "decision" {
				continue
			}
			reply := "reject"
			if yes, _ := d["allow"].(bool); yes {
				reply = "once"
			}
			requestCtx, stop := context.WithTimeout(ctx, 10*time.Second)
			err = api.request(requestCtx, "POST", "/permission/"+url.PathEscape(protocol.Str(d, "wireId"))+"/reply", map[string]any{"reply": reply}, nil)
			stop()
			if err != nil {
				emit(map[string]any{"type": "decision_ack", "wireId": protocol.Str(d, "wireId"), "error": err.Error()})
				return err
			}
			emit(map[string]any{"type": "decision_ack", "wireId": protocol.Str(d, "wireId")})
		case event := <-eventQueue:
			p := protocol.Map(event["properties"])
			sid := protocol.Str(p, "sessionID")
			part := protocol.Map(p["part"])
			if sid == "" {
				sid = protocol.Str(part, "sessionID")
			}
			if sid != "" && sid != external {
				continue
			}
			switch protocol.Str(event, "type") {
			case "permission.asked":
				permission := protocol.Str(p, "permission")
				metadata := protocol.Map(p["metadata"])
				category := "unknown"
				if permission == "bash" {
					category = "shell_command"
				}
				if permission == "edit" {
					category = "file_write"
				}
				if permission == "webfetch" || permission == "websearch" {
					category = "network_request"
				}
				patterns := protocol.List(p["patterns"])
				command := protocol.Str(metadata, "command")
				if category == "shell_command" && command == "" && len(patterns) == 1 {
					command = protocol.String(patterns[0])
				}
				path := protocol.Str(metadata, "filePath")
				emit(map[string]any{"type": "permission", "wireId": protocol.Str(p, "id"), "request": map[string]any{"category": category, "title": permission, "command": command, "filePath": path, "description": "OpenCode requests " + permission + " access"}})
			case "message.part.updated":
				id := protocol.Str(part, "id")
				if protocol.Str(part, "type") == "text" {
					if _, ok := texts[id]; !ok {
						order = append(order, id)
					}
					texts[id] = protocol.Str(part, "text")
				}
				if protocol.Str(part, "type") == "tool" {
					state := protocol.Map(part["state"])
					status := protocol.Str(state, "status")
					key := id + ":" + status
					if (status == "completed" || status == "error") && !emitted[key] {
						emitted[key] = true
						emit(map[string]any{"type": "tool_use", "sessionID": external, "part": part})
					}
				}
			case "message.part.delta":
				if protocol.Str(p, "field") == "text" {
					id := protocol.Str(p, "partID")
					if _, ok := texts[id]; !ok {
						order = append(order, id)
					}
					texts[id] += protocol.Str(p, "delta")
				}
			case "session.error":
				emit(map[string]any{"type": "error", "error": p["error"]})
				emit(map[string]any{"type": "done", "failed": true})
				return nil
			case "session.idle":
				for _, id := range order {
					if texts[id] != "" {
						emit(map[string]any{"type": "text", "sessionID": external, "part": map[string]any{"text": texts[id]}})
					}
				}
				emit(map[string]any{"type": "done"})
				return nil
			}
		case err := <-streamErrors:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
