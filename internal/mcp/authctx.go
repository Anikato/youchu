package mcp

import (
	"context"

	"youchu/internal/auth"
)

type accessTokenKey struct{}

func WithAccessToken(ctx context.Context, tok auth.AccessToken) context.Context {
	return context.WithValue(ctx, accessTokenKey{}, tok)
}

func AccessTokenFrom(ctx context.Context) (auth.AccessToken, bool) {
	tok, ok := ctx.Value(accessTokenKey{}).(auth.AccessToken)
	return tok, ok
}
