package database

import (
	"database/sql"
	"os"
	"path/filepath"
)

// VACUUM INTO creates a consistent snapshot including committed WAL data.
// A byte copy of a live database would miss WAL transactions.
func backupDatabase(db *sql.DB, path string) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".pre-migration-*.db")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err = f.Close(); err == nil {
		_, err = db.Exec("VACUUM INTO ?", name)
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	f, err = os.OpenFile(name, os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	return name, closeErr
}
