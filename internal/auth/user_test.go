package auth

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"youchu/internal/migrate"
	"youchu/internal/store"
	"youchu/migrations"
)

func TestBootstrapCreatesOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	if err := Bootstrap(ctx, db, "", ""); err == nil {
		t.Fatal("accepted empty credentials")
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
	if err := Bootstrap(ctx, db, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	var id int
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT id, password_hash FROM users`).Scan(&id, &hash); err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id=%d", id)
	}
	if err := Bootstrap(ctx, db, "ada", "another-password"); err != nil {
		t.Fatal(err)
	}
	var again string
	if err := db.QueryRowContext(ctx, `SELECT password_hash FROM users`).Scan(&again); err != nil || again != hash {
		t.Fatal("password hash changed")
	}
}

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var files []migrate.File
	err = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	return db
}
