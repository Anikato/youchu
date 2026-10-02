package store

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	u := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(abs),
		RawQuery: "_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL",
	}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	return db, nil
}
