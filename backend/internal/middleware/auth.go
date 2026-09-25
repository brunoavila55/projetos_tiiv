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

// sessaoDaRequisicao valida o cookie de sessão; msg explica a recusa quando user é nil
func (m *AuthMiddleware) sessaoDaRequisicao(r *http.Request) (user *AuthUser, msg string) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, "não autenticado"
	}

	tokenHash := HashToken(cookie.Value)
	sessao, err := m.db.Queries.BuscarSessaoPorHash(r.Context(), tokenHash)
	if err != nil {
		return nil, "sessão inválida ou expirada"
	}

	now := time.Now()

	// 1. Expiração absoluta (padrão 12h)
	if now.After(sessao.ExpiraEm.Time) {
		_ = m.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
		return nil, "sessão expirada"
	}

	// 2. Expiração por inatividade (padrão 30m)
	if now.Sub(sessao.UltimoUsoEm.Time) > m.cfg.SessionTTL {
		_ = m.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
		return nil, "sessão expirada por inatividade"
	}

	// Atualizar último uso se passou mais de 30 segundos desde o último registro
	if now.Sub(sessao.UltimoUsoEm.Time) > 30*time.Second {
		_ = m.db.Queries.AtualizarUltimoUsoSessao(r.Context(), sqlc.AtualizarUltimoUsoSessaoParams{
			ID:          sessao.ID,
			UltimoUsoEm: database.TimeToTimestamptz(now),
		})
	}

	return &AuthUser{
		ID:            database.UUIDToString(sessao.UsuarioID),
		Nome:          sessao.UsuarioNome,
		Cor:           sessao.UsuarioCor,
		Papel:         sessao.UsuarioPapel,
		Tema:          sessao.UsuarioTema,
		SessaoID:      database.UUIDToString(sessao.ID),
		DeveTrocarPin: sessao.UsuarioDeveTrocarPin,
		Setor:         sessao.SetorID,
		SetorNome:     sessao.SetorNome,
	}, ""
}

// Rotas liberadas enquanto o usuário ainda precisa trocar o PIN inicial
var rotasTrocaPinPendente = map[string]bool{
	"/api/auth/me":         true,
	"/api/auth/trocar-pin": true,
}

// RequireAuth valida o cookie de sessão para rotas protegidas
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authUser, msg := m.sessaoDaRequisicao(r)
		if authUser == nil {
			response.JSONError(w, http.StatusUnauthorized, msg)
			return
		}

		if authUser.DeveTrocarPin && !rotasTrocaPinPendente[r.URL.Path] {
			response.JSONError(w, http.StatusForbidden, "troque o PIN inicial antes de continuar")
			return
		}

		ctx := SetAuthUser(r.Context(), authUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth identifica o usuário quando há sessão válida, sem exigir login
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authUser, _ := m.sessaoDaRequisicao(r); authUser != nil {
			r = r.WithContext(SetAuthUser(r.Context(), authUser))
		}
		next.ServeHTTP(w, r)
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

		if !user.EhAdmin() {
			response.JSONError(w, http.StatusForbidden, "acesso negado: privilégios de administrador necessários")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireSuperadmin: gestão de setores e de pessoas entre setores
func (m *AuthMiddleware) RequireSuperadmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetAuthUser(r.Context())
		if !ok || user == nil {
			response.JSONError(w, http.StatusUnauthorized, "não autenticado")
			return
		}

		if !user.EhSuperadmin() {
			response.JSONError(w, http.StatusForbidden, "acesso negado: apenas o superadmin gerencia setores")
			return
		}

		next.ServeHTTP(w, r)
	})
}
