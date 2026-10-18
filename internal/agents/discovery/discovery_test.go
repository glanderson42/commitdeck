package discovery

import (
	"commitdeck/internal/database"
	"commitdeck/internal/workspace"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderMetadataAndWorkspaceMatch(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	claude := ParseClaude(strings.NewReader(`{"type":"user","sessionId":"native-123","cwd":"`+dir+`","gitBranch":"PROJ-421-retry","message":{"content":"must not import chat or token"}}`+"\n"+`{"type":"summary","summary":"PROJ-421 retry investigation"}`), "native-123")
	if claude.CWD != dir || claude.Branch != "PROJ-421-retry" || strings.Contains(claude.Title, "token") {
		t.Fatal(claude)
	}
	if Score(claude, workspace.Workspace{WorktreePath: dir}) < 100 {
		t.Fatal("worktree not matched")
	}
	if Score(claude, workspace.Workspace{ExternalKey: "OTHER-1", Branch: "unrelated"}) != 0 {
		t.Fatal("false match")
	}
	rows, err := ParseOpenCode([]byte(`[{"id":"ses_test123","title":"Tests","directory":"/ticket","time":{"updated":1750000000000}}]`))
	if err != nil || len(rows) != 1 || rows[0].CWD != "/ticket" || !rows[0].ResumeSupported {
		t.Fatal(rows, err)
	}
	if _, err = ParseOpenCode([]byte(`{"version":"changed"}`)); err == nil {
		t.Fatal("unknown format accepted")
	}
}
func TestImportIsExplicitScopedAndDuplicateSafe(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w, _ := db.CreateWorkspace(workspace.Workspace{Title: "Import"})
	service := New(db)
	source := Session{Provider: "codex", ExternalSessionID: "native-123", Title: "History", CWD: "/original", ResumeSupported: false, Source: "fixture"}
	service.found[key(source)] = source
	if _, err = service.Import("codex", "unknown", w.ID); err == nil {
		t.Fatal("undiscovered import")
	}
	session, err := service.Import("codex", source.ExternalSessionID, w.ID)
	if err != nil || !session.Imported || session.ResumeSupported || session.ExternalSessionID != "native-123" || session.SourceCWD != "/original" {
		t.Fatal(session, err)
	}
	if _, err = service.Import("codex", source.ExternalSessionID, w.ID); err == nil {
		t.Fatal("duplicate import")
	}
	sessions, _ := db.Sessions(w.ID)
	if len(sessions) != 1 {
		t.Fatal(sessions)
	}
}
