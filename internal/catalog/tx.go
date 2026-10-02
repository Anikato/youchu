package catalog

import (
	"context"
	"database/sql"
)

func withImmediate(ctx context.Context, db *sql.DB, fn func(*sql.Conn) error) error {
	return withConnTx(ctx, db, "BEGIN IMMEDIATE", fn)
}

func withRead(ctx context.Context, db *sql.DB, fn func(*sql.Conn) error) error {
	return withConnTx(ctx, db, "BEGIN", fn)
}

func withConnTx(ctx context.Context, db *sql.DB, begin string, fn func(*sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, begin); err != nil {
		return err
	}
	// A ROLLBACK after a successful COMMIT is an error and must not replace success.
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if err := fn(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
