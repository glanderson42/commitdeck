package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func oldDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
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
	_, err = db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES(1);
INSERT INTO workspaces VALUES('w','PROJ-421','Retry','Done','','','','',1,'1','1');
INSERT INTO notes VALUES('n','w','Preserved decision','1','1');
INSERT INTO todos VALUES('t','w','Preserved task',1,0,'1','1');
INSERT INTO agent_sessions VALUES('s','w','codex','native-id','History','testing','complete',0,'1','1');
INSERT INTO agent_events(id,session_id,payload) VALUES('e','s','{"id":"e","sessionId":"s","type":"text","text":"Preserved history"}');`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertKnowledge(t *testing.T, db *DB) {
	t.Helper()
	w, err := db.Workspace("w")
	if err != nil || !w.Archived || w.Lifecycle != "ARCHIVED" {
		t.Fatal(w, err)
	}
	n, err := db.Note("w")
	if err != nil || n.Content != "Preserved decision" {
		t.Fatal(n, err)
	}
	todos, err := db.Todos("w")
	if err != nil || len(todos) != 1 || !todos[0].Completed {
		t.Fatal(todos, err)
	}
	events, err := db.Events("s")
	if err != nil || len(events) != 1 || events[0].Text != "Preserved history" {
		t.Fatal(events, err)
	}
}

func TestMigrationBackupRestoreAndRepeatedStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	old := oldDatabase(t, path)
	if _, err := old.Exec("PRAGMA journal_mode=WAL; UPDATE notes SET content='Preserved decision'"); err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	assertKnowledge(t, db)
	if _, err = Open(path); err == nil {
		t.Fatal("second application acquired database lock")
	}
	db.Close()
	old.Close()
	backups, _ := filepath.Glob(path + ".pre-migration-*.db")
	if len(backups) != 1 {
		t.Fatal(backups)
	}
	for range 2 {
		db, err = Open(path)
		if err != nil {
			t.Fatal(err)
		}
		assertKnowledge(t, db)
		db.Close()
	}
	after, _ := filepath.Glob(path + ".pre-migration-*.db")
	if len(after) != 1 {
		t.Fatal("unnecessary startup backups", after)
	}
	// Restore into a different directory; never overwrite an active database.
	b, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	restore := filepath.Join(t.TempDir(), "restored.db")
	if err = os.WriteFile(restore, b, 0600); err != nil {
		t.Fatal(err)
	}
	db, err = Open(restore)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertKnowledge(t, db)
}

func TestFailedMigrationRollsBackAndKeepsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db := oldDatabase(t, path)
	if _, err := db.Exec("CREATE TABLE terminals(sentinel TEXT)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if opened, err := Open(path); err == nil {
		opened.Close()
		t.Fatal("conflicting migration accepted")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=3").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err = db.Exec("SELECT lifecycle FROM workspaces"); err == nil {
		t.Fatal("failed ALTER was not rolled back")
	}
	var note string
	if err = db.QueryRow("SELECT content FROM notes WHERE id='n'").Scan(&note); err != nil || note != "Preserved decision" {
		t.Fatal(note, err)
	}
	backups, _ := filepath.Glob(path + ".pre-migration-*.db")
	if len(backups) != 1 {
		t.Fatal("missing recovery backup")
	}
	if _, err = db.Exec("DROP TABLE terminals"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	good, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer good.Close()
	assertKnowledge(t, good)
}

func TestPhaseTwoMigrationKeepsArchivedKnowledge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	db := oldDatabase(t, path)
	body, err := migrations.ReadFile("migrations/002_control_center.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO schema_migrations VALUES(2)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	upgraded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	assertKnowledge(t, upgraded)
}
