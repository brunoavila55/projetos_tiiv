package middleware

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
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

	DeveTrocarPin bool `json:"deve_trocar_pin"`

	// Setor de trabalho: o do usuário, ou o escolhido na sessão pelo superadmin.
	// Tudo o que é separado por setor é filtrado por ele.
	Setor     pgtype.UUID `json:"-"`
	SetorNome string      `json:"-"`
}

// EhAdmin: admin do setor ou superadmin
func (u *AuthUser) EhAdmin() bool {
	return u.Papel == "admin" || u.Papel == "superadmin"
}

func (u *AuthUser) EhSuperadmin() bool {
	return u.Papel == "superadmin"
}

func SetAuthUser(ctx context.Context, u *AuthUser) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func GetAuthUser(ctx context.Context) (*AuthUser, bool) {
	u, ok := ctx.Value(userContextKey).(*AuthUser)
	return u, ok
}
