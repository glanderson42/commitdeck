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
	"path/filepath"
	"strings"
	"testing"
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
