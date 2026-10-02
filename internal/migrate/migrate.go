package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrDatabaseNewer = errors.New("database version is newer than this program")
var ErrAppliedPrefix = errors.New("applied migrations are not a contiguous prefix")

type File struct {
	Version int
	Name    string
	SQL     string
}

func CheckEmbedded(files []File) error {
	if len(files) == 0 {
		return fmt.Errorf("no migrations")
	}
	seen := append([]File(nil), files...)
	sort.Slice(seen, func(i, j int) bool { return seen[i].Version < seen[j].Version })
	for i, f := range seen {
		if f.Version != i+1 {
			return fmt.Errorf("embedded migrations must be contiguous from 001")
		}
	}
	return nil
}

func Statements(script string) []string {
	var b strings.Builder
	var out []string
	inSingle := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if c == '\'' {
			b.WriteByte(c)
			if inSingle && i+1 < len(script) && script[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if c == ';' && !inSingle {
			if s := strings.TrimSpace(b.String()); s != "" {
				out = append(out, s)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func Apply(ctx context.Context, db *sql.DB, files []File, now time.Time) error {
	if err := CheckEmbedded(files); err != nil {
		return err
	}
	ordered := append([]File(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var applied []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied = append(applied, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	embeddedMax := ordered[len(ordered)-1].Version
	appliedMax := 0
	for i, v := range applied {
		if v > embeddedMax {
			return ErrDatabaseNewer
		}
		if v != i+1 {
			return ErrAppliedPrefix
		}
		appliedMax = v
	}
	for _, f := range ordered {
		if f.Version <= appliedMax {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range Statements(f.SQL) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				tx.Rollback()
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`, f.Version, f.Name, now.UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func ParseVersion(name string) (int, error) {
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	if len(base) < 8 || base[3] != '_' || !strings.HasSuffix(base, ".sql") {
		return 0, fmt.Errorf("bad migration name %s", name)
	}
	n, err := strconv.Atoi(base[:3])
	if err != nil || n < 1 || fmt.Sprintf("%03d", n) != base[:3] {
		return 0, fmt.Errorf("bad migration name %s", name)
	}
	return n, nil
}
