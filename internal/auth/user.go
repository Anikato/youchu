package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"youchu/internal/config"
)

var ErrCurrentPassword = errors.New("current password")

func Bootstrap(ctx context.Context, db *sql.DB, username, password string) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	name, err := config.ValidateNewUser(username, password)
	if err != nil {
		return err
	}
	hash, err := Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO users(id, username, password_hash, created_at, updated_at) VALUES (1, ?, ?, ?, ?)`, name, hash, now, now)
	return err
}

func UpdateUsername(ctx context.Context, db *sql.DB, username string, now time.Time) error {
	stamp := now.UTC().Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx, `UPDATE users SET username = ?, updated_at = ? WHERE id = 1`, username, stamp)
	return err
}

func ChangePassword(ctx context.Context, db *sql.DB, keepToken, currentPassword, newPassword string, now time.Time) error {
	return withImmediate(ctx, db, func(conn *sql.Conn) error {
		var hash string
		if err := conn.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = 1`).Scan(&hash); err != nil {
			return err
		}
		if !Verify(hash, currentPassword) {
			return ErrCurrentPassword
		}
		newHash, err := Hash(newPassword)
		if err != nil {
			return err
		}
		stamp := now.UTC().Format(time.RFC3339Nano)
		if _, err := conn.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = 1`, newHash, stamp); err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash <> ?`, tokenHash(keepToken))
		return err
	})
}

func withImmediate(ctx context.Context, db *sql.DB, fn func(*sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
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
