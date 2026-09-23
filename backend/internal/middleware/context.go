package middleware

import (
	"context"
)

type contextKey string

const userContextKey contextKey = "authUser"

type AuthUser struct {
	ID       string `json:"id"`
	Nome     string `json:"nome"`
	Cor      string `json:"cor"`
	Papel    string `json:"papel"`
	Tema     string `json:"tema"`
	SessaoID string `json:"sessao_id"`
}

func SetAuthUser(ctx context.Context, u *AuthUser) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func GetAuthUser(ctx context.Context) (*AuthUser, bool) {
	u, ok := ctx.Value(userContextKey).(*AuthUser)
	return u, ok
}
