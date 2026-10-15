package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServerBridgePermissionRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan string, 8)
	var mu sync.Mutex
	replied := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "commitdeck" || password != "private" || r.Header.Get("x-opencode-directory") != "/ticket" {
			t.Error("missing scoped authentication")
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/session":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["permission"] == nil {
				t.Error("new sessions must request permissions")
			}
			_, _ = io.WriteString(w, `{"id":"ses-1"}`)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			for {
				select {
				case event := <-events:
					_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
					w.(http.Flusher).Flush()
				case <-r.Context().Done():
					return
				}
			}
		case "/session/ses-1/prompt_async":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.Contains(fmt.Sprint(body), "Exact preview") {
				t.Error("context/prompt missing")
			}
			w.WriteHeader(204)
			events <- `{"type":"permission.asked","properties":{"id":"p1","sessionID":"ses-1","permission":"bash","metadata":{"command":"go test ./..."}}}`
		case "/permission/p1/reply":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["reply"] != "once" {
				t.Error("broader native approval is forbidden")
			}
			mu.Lock()
			replied = true
			mu.Unlock()
			_, _ = io.WriteString(w, "true")
			events <- `{"type":"message.part.updated","properties":{"part":{"sessionID":"ses-1","id":"text1","type":"text","text":"Tests passed"}}}`
			events <- `{"type":"session.idle","properties":{"sessionID":"ses-1"}}`
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	reader, writer := io.Pipe()
	defer writer.Close()
	defer reader.Close()
	var got []map[string]any
	emit := func(v any) {
		b, _ := json.Marshal(v)
		var event map[string]any
		_ = json.Unmarshal(b, &event)
		got = append(got, event)
		if event["type"] == "permission" {
			go func() { _, _ = io.WriteString(writer, `{"kind":"decision","wireId":"p1","allow":true}`+"\n") }()
		}
	}
	api := serverAPI{base: server.URL, password: "private", cwd: "/ticket", client: server.Client()}
	if err := runAPI(ctx, api, bufio.NewScanner(reader), "Exact preview\nUser request", "", emit); err != nil {
		t.Fatal(err)
	}
	ack, done, text := false, false, false
	for _, e := range got {
		switch e["type"] {
		case "decision_ack":
			ack = true
		case "done":
			done = true
		case "text":
			text = true
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if !replied || !ack || !done || !text {
		t.Fatal("incomplete provider lifecycle", got)
	}
}
