package migrate

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"youchu/internal/store"
	"youchu/migrations"
)

func TestCheckEmbeddedContiguous(t *testing.T) {
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 2, Name: "002_b.sql"}}); err != nil {
		t.Fatal(err)
	}
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 3, Name: "003_c.sql"}}); err == nil {
		t.Fatal("accepted a gap")
	}
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 1, Name: "001_b.sql"}}); err == nil {
		t.Fatal("accepted a duplicate")
	}
}

func TestStatementsKeepQuotedSemicolon(t *testing.T) {
	got := Statements("CREATE TABLE a(n TEXT); INSERT INTO a(n) VALUES ('a;b');")
	if len(got) != 2 || got[1] != "INSERT INTO a(n) VALUES ('a;b')" {
		t.Fatalf("%#v", got)
	}
}

func TestApplyTwiceKeepsOneRow(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	files := []File{{Version: 1, Name: "001_auth.sql", SQL: "CREATE TABLE marker(id INTEGER);"}}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}

func TestApplyRunsOnlyRemainder(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	if err := Apply(ctx, db, []File{{Version: 1, Name: "001_old.sql", SQL: "CREATE TABLE old(id INTEGER);"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	files := []File{
		{Version: 1, Name: "001_new.sql", SQL: "CREATE TABLE ignored(id INTEGER);"},
		{Version: 2, Name: "002_new.sql", SQL: "CREATE TABLE added(id INTEGER);"},
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 1`).Scan(&name); err != nil || name != "001_old.sql" {
		t.Fatalf("name=%s err=%v", name, err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM added`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'ignored'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("re-executed 001")
	}
}

func TestApplyRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	files := []File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
		{Version: 2, Name: "002_b.sql", SQL: "SELECT 1;"},
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 2`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	err := Apply(ctx, db, []File{{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"}}, time.Now())
	if !errors.Is(err, ErrDatabaseNewer) {
		t.Fatalf("err=%v", err)
	}
	var after string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 2`).Scan(&after); err != nil || after != before {
		t.Fatalf("row changed: %s -> %s err=%v", before, after, err)
	}
}

func TestApplyRejectsBrokenPrefix(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	if err := Apply(ctx, db, []File{{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (2, '002_x.sql', '2026-09-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	err := Apply(ctx, db, []File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
		{Version: 2, Name: "002_b.sql", SQL: "SELECT 1;"},
	}, time.Now())
	if !errors.Is(err, ErrAppliedPrefix) {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyEmbeddedAuthSchema(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	var files []File
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		files = append(files, File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.SetMaxIdleConns(0)
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
	if _, err := conn.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES ('x', 9, '2026-09-29T00:00:00Z', '2026-09-29T00:00:00Z')`); err == nil {
		t.Fatal("foreign key was not enforced on sessions")
	}
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
