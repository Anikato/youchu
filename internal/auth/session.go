package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

var ErrUnauthenticated = errors.New("unauthenticated")

func Create(ctx context.Context, db *sql.DB, ttl time.Duration, now time.Time) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	created := now.UTC()
	_, err := db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES (?, 1, ?, ?)`,
		tokenHash(token), created.Add(ttl).Format(time.RFC3339Nano), created.Format(time.RFC3339Nano))
	return token, err
}

func Lookup(ctx context.Context, db *sql.DB, token string, now time.Time) (string, error) {
	var username, expText string
	err := db.QueryRowContext(ctx, `SELECT users.username, sessions.expires_at FROM sessions JOIN users ON users.id = sessions.user_id WHERE sessions.token_hash = ?`, tokenHash(token)).Scan(&username, &expText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", err
	}
	exp, err := time.Parse(time.RFC3339Nano, expText)
	if err != nil {
		return "", err
	}
	if !now.Before(exp) {
		_, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
		return "", ErrUnauthenticated
	}
	return username, nil
}

func Delete(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
