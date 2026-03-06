package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

func Open(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}

	// Incremental migrations — silently ignored if column already exists
	db.Exec("ALTER TABLE messages ADD COLUMN content_json TEXT")
	db.Exec("ALTER TABLE messages ADD COLUMN role TEXT")

	return db, nil
}

func RebuildNeeded(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
