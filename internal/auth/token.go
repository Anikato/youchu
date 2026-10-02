package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type AccessToken struct {
	ID          int64
	Name        string
	Token       string // 仅创建时非空
	TokenPrefix string
	Scopes      []string
	CreatedAt   string
}

var (
	ErrTokenLimit    = errors.New("token limit")
	ErrNotFound      = errors.New("not found")
	ErrInvalidScopes = errors.New("权限范围不正确")
)

const maxAccessTokens = 20

func ParseScopes(in []string) ([]string, error) {
	if in == nil {
		return nil, ErrInvalidScopes
	}
	seen := map[string]bool{}
	for _, s := range in {
		switch s {
		case "read", "organize", "write":
			if seen[s] {
				return nil, ErrInvalidScopes
			}
			seen[s] = true
		default:
			return nil, ErrInvalidScopes
		}
	}
	if !seen["read"] {
		return nil, ErrInvalidScopes
	}
	out := make([]string, 0, 3)
	for _, s := range []string{"read", "organize", "write"} {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

func NormalizeTokenName(s string) (string, error) {
	for _, r := range s {
		if unicode.Is(unicode.Cc, r) {
			return "", errors.New("名称不能包含控制字符")
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("名称不能为空")
	}
	if utf8.RuneCountInString(s) > 80 {
		return "", errors.New("名称不能超过 80 个字符")
	}
	return s, nil
}

func CreateAccessToken(ctx context.Context, db *sql.DB, name string, scopes []string, now time.Time) (AccessToken, error) {
	name, err := NormalizeTokenName(name)
	if err != nil {
		return AccessToken{}, err
	}
	scopes, err = ParseScopes(scopes)
	if err != nil {
		return AccessToken{}, err
	}
	var created AccessToken
	err = withImmediate(ctx, db, func(conn *sql.Conn) error {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_tokens`).Scan(&n); err != nil {
			return err
		}
		if n >= maxAccessTokens {
			return ErrTokenLimit
		}
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		token := "yc_" + base64.RawURLEncoding.EncodeToString(buf)
		prefix := token[:12]
		stamp := now.UTC().Format(time.RFC3339Nano)
		res, err := conn.ExecContext(ctx, `INSERT INTO access_tokens(name, token_hash, token_prefix, scopes, created_at) VALUES (?, ?, ?, ?, ?)`,
			name, tokenHash(token), prefix, strings.Join(scopes, ","), stamp)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = AccessToken{
			ID:          id,
			Name:        name,
			Token:       token,
			TokenPrefix: prefix,
			Scopes:      scopes,
			CreatedAt:   stamp,
		}
		return nil
	})
	if err != nil {
		return AccessToken{}, err
	}
	return created, nil
}

func ListAccessTokens(ctx context.Context, db *sql.DB) ([]AccessToken, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, token_prefix, scopes, created_at FROM access_tokens ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AccessToken, 0)
	for rows.Next() {
		tok, err := scanAccessToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	return out, rows.Err()
}

func DeleteAccessToken(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM access_tokens WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func LookupAccessToken(ctx context.Context, db *sql.DB, token string) (AccessToken, error) {
	var tok AccessToken
	var scopeText string
	err := db.QueryRowContext(ctx, `SELECT id, name, token_prefix, scopes, created_at FROM access_tokens WHERE token_hash = ?`, tokenHash(token)).
		Scan(&tok.ID, &tok.Name, &tok.TokenPrefix, &scopeText, &tok.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AccessToken{}, ErrUnauthenticated
	}
	if err != nil {
		return AccessToken{}, err
	}
	tok.Scopes = strings.Split(scopeText, ",")
	return tok, nil
}

type tokenRow interface {
	Scan(dest ...any) error
}

func scanAccessToken(row tokenRow) (AccessToken, error) {
	var tok AccessToken
	var scopeText string
	if err := row.Scan(&tok.ID, &tok.Name, &tok.TokenPrefix, &scopeText, &tok.CreatedAt); err != nil {
		return AccessToken{}, err
	}
	tok.Scopes = strings.Split(scopeText, ",")
	return tok, nil
}
