package workspacetools

import (
	"bytes"
	"commitdeck/internal/activity"
	"commitdeck/internal/agents/protocol"
	"commitdeck/internal/database"
	"commitdeck/internal/permissions"
	"commitdeck/internal/workspace"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCapabilityOnlyAllowsBoundWorkspaceAndExpires(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Bound"})
	session, _ := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "claude", Title: "Tools"})
	activity := &activity.Service{Store: db}
	s, _ := New(db, permissions.New(db, activity, nil), activity)
	defer s.Close()
	env, revoke, err := s.Bind(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, token := "", ""
	for _, e := range env {
		if strings.HasPrefix(e, "COMMITDECK_TOOL_ENDPOINT=") {
			endpoint = strings.TrimPrefix(e, "COMMITDECK_TOOL_ENDPOINT=")
		}
		if strings.HasPrefix(e, "COMMITDECK_TOOL_TOKEN=") {
			token = strings.TrimPrefix(e, "COMMITDECK_TOOL_TOKEN=")
		}
	}
	post := func(auth, origin string) int {
		t.Helper()
		b, _ := json.Marshal(Request{Operation: "workspace.get"})
		req, _ := http.NewRequest("POST", endpoint+"/tool", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+auth)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode == 200 && !bytes.Contains(body, []byte("Bound")) {
			t.Fatal(string(body))
		}
		return res.StatusCode
	}
	if post("bad", "") != 401 {
		t.Fatal("unauthenticated tool access")
	}
	if post(token, "https://evil.example") != 403 {
		t.Fatal("browser origin accepted")
	}
	if post(token, "") != 200 {
		t.Fatal("scoped access failed")
	}
	revoke()
	if post(token, "") != 401 {
		t.Fatal("capability remained valid")
	}
}

func TestToolFailuresHaveErrorStatus(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Bound"})
	session, _ := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex"})
	a := &activity.Service{Store: db}
	p := permissions.New(db, a, nil)
	s, _ := New(db, p, a)
	defer s.Close()
	env, revoke, err := s.Bind(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		t.Setenv(k, v)
	}
	post := func(op Request) (int, string) {
		b, _ := json.Marshal(op)
		req, _ := http.NewRequest("POST", os.Getenv("COMMITDECK_TOOL_ENDPOINT")+"/tool", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+os.Getenv("COMMITDECK_TOOL_TOKEN"))
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Error(e)
			return 0, ""
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	if code, _ := post(Request{Operation: "sql.query"}); code != 400 {
		t.Fatal(code)
	}
	if code, _ := post(Request{Operation: "todo.add", Title: ""}); code != 422 {
		t.Fatal(code)
	}
	for _, allow := range []bool{false, true} {
		done := make(chan int, 1)
		go func() {
			code, _ := post(Request{Operation: "notes.update", Content: "Replace", ExpectedUpdatedAt: "stale"})
			done <- code
		}()
		deadline := time.Now().Add(3 * time.Second)
		for {
			rows, e := db.Permissions(w.ID, true)
			if e != nil {
				t.Fatal(e)
			}
			if len(rows) > 0 {
				if e = p.Resolve(rows[0].ID, permissions.Decision{Allow: allow, Scope: "once"}); e != nil {
					t.Fatal(e)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no approval request")
			}
			time.Sleep(5 * time.Millisecond)
		}
		want := 403
		if allow {
			want = 409
		}
		select {
		case code := <-done:
			if code != want {
				t.Fatal(code, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("tool blocked")
		}
	}
	if _, err = db.Exec("DROP TABLE todos"); err != nil {
		t.Fatal(err)
	}
	if code, body := post(Request{Operation: "todo.list"}); code != 422 || strings.Contains(body, "SQL") || strings.Contains(body, "todos") {
		t.Fatal(code, body)
	}
}

func TestToolTransportSupportsDocumentedNoteLimit(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, err := db.CreateWorkspace(workspace.Workspace{Title: "Large note"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := db.CreateSession(protocol.Session{WorkspaceID: w.ID, Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	a := &activity.Service{Store: db}
	s, _ := New(db, permissions.New(db, a, nil), a)
	defer s.Close()
	// Exercise the full HTTP handler without opening another listener.
	s.url = "http://127.0.0.1:12345"
	s.bindings["synthetic"] = binding{session, context.Background()}
	post := func(body []byte) int {
		r := httptest.NewRequest("POST", s.url+"/tool", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer synthetic")
		res := httptest.NewRecorder()
		s.ServeHTTP(res, r)
		return res.Code
	}
	for _, content := range []string{strings.Repeat("界", 30000), strings.Repeat("<", (4<<20)-2)} {
		if err = db.SaveNote(w.ID, ""); err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(Request{Operation: "notes.append", Content: content})
		if code := post(body); code != http.StatusOK {
			t.Fatalf("valid %d-byte note rejected by transport: %d", len(content), code)
		}
		note, err := db.Note(w.ID)
		if err != nil || !strings.Contains(note.Content, content) {
			t.Fatal("note not written", err)
		}
	}
	if err = db.SaveNote(w.ID, "Unchanged"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(Request{Operation: "notes.append", Content: strings.Repeat("x", 4<<20)})
	if code := post(body); code != http.StatusUnprocessableEntity {
		t.Fatalf("oversize note accepted: %d", code)
	}
	if code := post([]byte(strings.Repeat(" ", maxToolJSONBytes+1))); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize transport not rejected explicitly: %d", code)
	}
	note, err := db.Note(w.ID)
	if err != nil || note.Content != "Unchanged" {
		t.Fatal("failed operation modified note", err)
	}
}

func TestHelperSubprocess(t *testing.T) {
	if os.Getenv("COMMITDECK_HELPER_TEST") == "" {
		return
	}
	if err := RunHelper(context.Background(), "todo.add", `{"title":"Test"}`); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestHelperExitsNonzeroForFailuresIncludingLegacy200(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		ok     bool
	}{
		{422, `{"error":"invalid operation"}`, false}, {403, `{"error":"denied"}`, false},
		{409, `{"error":"stale"}`, false}, {401, `{"error":"expired"}`, false},
		{200, `{"error":"legacy error"}`, false}, {200, `not json`, false},
		{200, `{"result":{"id":"t"}}`, true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSubprocess$")
		cmd.Env = append(os.Environ(), "COMMITDECK_HELPER_TEST=1", "COMMITDECK_TOOL_ENDPOINT="+server.URL, "COMMITDECK_TOOL_TOKEN=synthetic")
		out, err := cmd.Output()
		server.Close()
		if (err == nil) != tc.ok {
			t.Fatalf("status %d: exit %v", tc.status, err)
		}
		if !tc.ok && len(out) != 0 {
			t.Fatal("failed tool printed success output")
		}
	}
}
