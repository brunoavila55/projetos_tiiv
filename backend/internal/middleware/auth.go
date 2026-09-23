package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/response"
)

const SessionCookieName = "tiiv_session"

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

type AuthMiddleware struct {
	db  *database.DB
	cfg *config.Config
}

func NewAuthMiddleware(db *database.DB, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		db:  db,
		cfg: cfg,
	}
}

// RequireAuth valida o cookie de sessão para rotas protegidas
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			response.JSONError(w, http.StatusUnauthorized, "não autenticado")
			return
		}

		tokenHash := HashToken(cookie.Value)
		sessao, err := m.db.Queries.BuscarSessaoPorHash(r.Context(), tokenHash)
		if err != nil {
			response.JSONError(w, http.StatusUnauthorized, "sessão inválida ou expirada")
			return
		}

		now := time.Now()

		// 1. Expiração absoluta (padrão 12h)
		if now.After(sessao.ExpiraEm.Time) {
			_ = m.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
			response.JSONError(w, http.StatusUnauthorized, "sessão expirada")
			return
		}

		// 2. Expiração por inatividade (padrão 30m)
		if now.Sub(sessao.UltimoUsoEm.Time) > m.cfg.SessionTTL {
			_ = m.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
			response.JSONError(w, http.StatusUnauthorized, "sessão expirada por inatividade")
			return
		}

		// Atualizar último uso se passou mais de 30 segundos desde o último registro
		if now.Sub(sessao.UltimoUsoEm.Time) > 30*time.Second {
			_ = m.db.Queries.AtualizarUltimoUsoSessao(r.Context(), sqlc.AtualizarUltimoUsoSessaoParams{
				ID:          sessao.ID,
				UltimoUsoEm: database.TimeToTimestamptz(now),
			})
		}

		authUser := &AuthUser{
			ID:       database.UUIDToString(sessao.UsuarioID),
			Nome:     sessao.UsuarioNome,
			Cor:      sessao.UsuarioCor,
			Papel:    sessao.UsuarioPapel,
			SessaoID: database.UUIDToString(sessao.ID),
		}

		ctx := SetAuthUser(r.Context(), authUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin garante que apenas administradores acessem a rota
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetAuthUser(r.Context())
		if !ok || user == nil {
			response.JSONError(w, http.StatusUnauthorized, "não autenticado")
			return
		}

		if user.Papel != "admin" {
			response.JSONError(w, http.StatusForbidden, "acesso negado: privilégios de administrador necessários")
			return
		}

		next.ServeHTTP(w, r)
	})
}
