package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionStoresHashAndExpires(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	if err := Bootstrap(ctx, db, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	token, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "" || stored == token {
		t.Fatal("stored the raw token")
	}
	name, err := Lookup(ctx, db, token, now.Add(time.Minute))
	if err != nil || name != "ada" {
		t.Fatalf("name=%s err=%v", name, err)
	}
	past := now.Add(-time.Second).Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `UPDATE sessions SET expires_at = ?`, past); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup(ctx, db, token, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
	first, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup(ctx, db, first, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	if name, err := Lookup(ctx, db, second, now); err != nil || name != "ada" {
		t.Fatalf("second session name=%s err=%v", name, err)
	}
}
