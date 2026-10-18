package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
	*sql.DB
	lock *os.File
}

func (d *DB) Close() error {
	err := d.DB.Close()
	if d.lock != nil {
		unlockFile(d.lock)
		d.lock = nil
	}
	return err
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := lockFile(path + ".lock")
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		unlockFile(lock)
		return nil, err
	}
	info, err := f.Stat()
	f.Close()
	if err != nil {
		unlockFile(lock)
		return nil, err
	}
	existing := info.Size() > 0
	db, err := sql.Open("sqlite", path)
	if err != nil {
		unlockFile(lock)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*DB, error) { db.Close(); unlockFile(lock); return nil, err }
	for _, p := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000"} {
		if _, err = db.Exec(p); err != nil {
			return fail(err)
		}
	}
	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)"); err != nil {
		return fail(err)
	}
	entries, _ := migrations.ReadDir("migrations")
	backedUp := false
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		v, _ := strconv.Atoi(strings.Split(e.Name(), "_")[0])
		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", v).Scan(&count); err != nil {
			return fail(err)
		}
		if count > 0 {
			continue
		}
		if existing && !backedUp {
			if _, err = backupDatabase(db, path); err != nil {
				return fail(fmt.Errorf("pre-migration backup failed; database not migrated: %w", err))
			}
			backedUp = true
		}
		body, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fail(err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fail(err)
		}
		if _, err = tx.Exec(string(body)); err == nil {
			_, err = tx.Exec("INSERT INTO schema_migrations VALUES (?)", v)
		}
		if err != nil {
			tx.Rollback()
			return fail(fmt.Errorf("migration %d: %w", v, err))
		}
		if err = tx.Commit(); err != nil {
			return fail(err)
		}
	}
	return &DB{DB: db, lock: lock}, nil
}
