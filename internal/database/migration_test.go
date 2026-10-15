package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPhaseOneDatabaseMigratesWithoutLosingContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES(1); INSERT INTO workspaces(id,external_key,title,created_at,updated_at) VALUES('w','PROJ-421','Retry','1','1'); INSERT INTO notes VALUES('n','w','Keep this note','1','1'); INSERT INTO agent_sessions(id,workspace_id,provider,title,status,created_at,updated_at) VALUES('s','w','codex','Implementation','complete','1','1')"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	migrated, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	note, err := migrated.Note("w")
	if err != nil || note.Content != "Keep this note" {
		t.Fatal(note, err)
	}
	session, err := migrated.Session("s")
	if err != nil || session.Status != "completed" {
		t.Fatal(session, err)
	}
	for _, table := range []string{"permission_requests", "permission_rules", "workspace_events", "agent_session_context"} {
		var count int
		if err = migrated.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(table, err)
		}
	}
}
