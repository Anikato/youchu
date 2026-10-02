package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"youchu/internal/config"
	"youchu/internal/migrate"
)

func TestBuildRejectsBadTTLBeforeCreatingDatabase(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173",
		"YOUCHU_DATA_DIR":      dir,
		"YOUCHU_SESSION_TTL":   "0s",
		"YOUCHU_USERNAME":      "ada",
		"YOUCHU_PASSWORD":      "correct-horse",
	}
	if _, _, err := Build(context.Background(), func(k string) string { return env[k] }); err == nil {
		t.Fatal("accepted 0s ttl")
	}
	if _, err := os.Stat(filepath.Join(dir, "youchu.db")); !os.IsNotExist(err) {
		t.Fatal("database was created")
	}
}

func TestPrepareRejectsGapBeforeCreatingDatabase(t *testing.T) {
	dir := t.TempDir()
	_, err := Prepare(context.Background(), config.Config{DataDir: dir}, []migrate.File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
		{Version: 3, Name: "003_c.sql", SQL: "SELECT 1;"},
	})
	if err == nil {
		t.Fatal("accepted a gap")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "youchu.db")); !os.IsNotExist(statErr) {
		t.Fatal("database was created")
	}
}

func TestBuildCreatesUserAndKeepsPassword(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173",
		"YOUCHU_DATA_DIR":      dir,
		"YOUCHU_USERNAME":      "ada",
		"YOUCHU_PASSWORD":      "correct-horse",
	}
	server, db, err := Build(context.Background(), func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("users=%d err=%v", n, err)
	}
	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(`{"username":"ada","password":"correct-horse"}`))
	req.RemoteAddr = "203.0.113.5:1"
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login=%d %s", rec.Code, rec.Body.String())
	}
	for _, name := range []string{"originals", "thumbnails", "tmp"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.IsDir() {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	db.Close()
	env["YOUCHU_PASSWORD"] = "different-password"
	_, db2, err := Build(context.Background(), func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db2.Close() })
	var again string
	if err := db2.QueryRow(`SELECT password_hash FROM users`).Scan(&again); err != nil || again != hash {
		t.Fatal("password hash changed")
	}
}

func TestPreparePhotoDirsFail(t *testing.T) {
	dir := t.TempDir()
	block := filepath.Join(dir, "originals")
	if err := os.WriteFile(block, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Prepare(context.Background(), config.Config{DataDir: dir}, []migrate.File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
	})
	if err == nil {
		t.Fatal("accepted blocked originals path")
	}
}
