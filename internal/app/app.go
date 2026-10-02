package app

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"youchu/internal/auth"
	"youchu/internal/config"
	"youchu/internal/httpapi"
	"youchu/internal/migrate"
	"youchu/internal/photo"
	"youchu/internal/store"
	"youchu/migrations"
)

func Build(ctx context.Context, getenv func(string) string) (*http.Server, *sql.DB, error) {
	cfg, err := config.Load(getenv)
	if err != nil {
		return nil, nil, err
	}
	files, err := LoadMigrations(migrations.FS)
	if err != nil {
		return nil, nil, err
	}
	db, err := Prepare(ctx, cfg, files)
	if err != nil {
		return nil, nil, err
	}
	dummy, err := auth.NewDummyHash()
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(db, cfg, dummy, time.Now)}, db, nil
}

func Prepare(ctx context.Context, cfg config.Config, files []migrate.File) (*sql.DB, error) {
	if err := migrate.CheckEmbedded(files); err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "youchu.db"))
	if err != nil {
		return nil, err
	}
	if err := migrate.Apply(ctx, db, files, time.Now()); err != nil {
		db.Close()
		return nil, err
	}
	if err := photo.EnsureDirs(cfg.DataDir); err != nil {
		db.Close()
		return nil, err
	}
	if err := auth.Bootstrap(ctx, db, cfg.Username, cfg.Password); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func LoadMigrations(fsys fs.FS) ([]migrate.File, error) {
	var files []migrate.File
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	return files, err
}
