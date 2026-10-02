package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesPragmasOnEachConnection(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxIdleConns(0)
	if _, err := db.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY CHECK (id = 1))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE sessions (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id))`); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var fk, busy int
		if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
			t.Fatalf("foreign_keys=%d err=%v", fk, err)
		}
		if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busy); err != nil || busy != 5000 {
			t.Fatalf("busy_timeout=%d err=%v", busy, err)
		}
	}
	check()
	check()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `INSERT INTO sessions(user_id) VALUES (9)`); err == nil {
		t.Fatal("foreign key was not enforced")
	}
}
