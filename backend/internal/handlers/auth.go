package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/modulos"
	"tiiv/backend/internal/response"
)

// PIN de acesso: exatamente 4 dígitos numéricos
var pinRegex = regexp.MustCompile(`^[0-9]{4}$`)

// Cor do usuário: vai para atributos style no frontend, então só #RRGGBB
var corRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Limites de PIN errado por origem, somados ao bloqueio por conta: um único
// host não chega às 5 falhas que travam a conta, e o total da rede fica contido.
const (
	janelaFalhasIP      = 15 * time.Minute
	falhasGlobaisMinuto = 30
)

type AuthHandler struct {
	db  *database.DB
	cfg *config.Config

	falhasPorIP   *limitadorPorIP
	falhasGlobais *limitadorPorIP
}

func NewAuthHandler(db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:            db,
		cfg:           cfg,
		falhasPorIP:   novoLimitadorPorIP(cfg.LoginFalhasPorIP, janelaFalhasIP),
		falhasGlobais: novoLimitadorPorIP(falhasGlobaisMinuto, time.Minute),
	}
}

type UsuarioPublico struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	Cor        string `json:"cor"`
	FotoVersao *int64 `json:"foto_versao"`
}

// ListarUsuariosPublico: GET /api/auth/usuarios (pública)
func (h *AuthHandler) ListarUsuariosPublico(w http.ResponseWriter, r *http.Request) {
	usuarios, err := h.db.Queries.ListarUsuariosAtivos(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar usuários")
		return
	}

	result := make([]UsuarioPublico, 0, len(usuarios))
	for _, u := range usuarios {
		result = append(result, UsuarioPublico{
			ID:         database.UUIDToString(u.ID),
			Nome:       u.Nome,
			Cor:        u.Cor,
			FotoVersao: fotoVersao(u.FotoAtualizadaEm),
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type LoginRequest struct {
	UsuarioID string `json:"usuario_id"`
	PIN       string `json:"pin"`
}

type UserProfileResponse struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	Cor        string `json:"cor"`
	Papel      string `json:"papel"`
	Tema       string `json:"tema"`
	FotoVersao *int64 `json:"foto_versao"`
	// Admin inicial: precisa trocar o PIN antes de usar o sistema
	DeveTrocarPin bool `json:"deve_trocar_pin"`
	// Setor de trabalho (o superadmin pode trocar), com os módulos ligados
	Setor SetorTrabalho `json:"setor"`
}

// Login: POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.UsuarioID == "" || req.PIN == "" {
		response.JSONError(w, http.StatusBadRequest, "usuario_id e pin são obrigatórios")
		return
	}

	if !pinRegex.MatchString(req.PIN) {
		response.JSONError(w, http.StatusBadRequest, "PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	uID, err := database.StringToUUID(req.UsuarioID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de usuário inválido")
		return
	}

	// 1. Reservar uma falha para o IP e para o total antes de testar o PIN;
	// é devolvida se o PIN estiver certo ou se a conta já estava bloqueada.
	ip := ipDaRequisicao(r)
	if !h.falhasPorIP.permitir(ip) {
		response.JSONError(w, http.StatusTooManyRequests, "muitas tentativas de PIN erradas neste terminal. Aguarde alguns minutos.")
		return
	}
	if !h.falhasGlobais.permitir("") {
		h.falhasPorIP.liberar(ip)
		response.JSONError(w, http.StatusTooManyRequests, "muitas tentativas de login no momento. Aguarde um minuto.")
		return
	}
	liberarFalha := func() {
		h.falhasPorIP.liberar(ip)
		h.falhasGlobais.liberar("")
	}

	// 2. Reservar a tentativa na conta e validar o PIN com bcrypt
	user, resultado, bloqueadoAte, err := conferirPin(r.Context(), h.db.Queries, uID, req.PIN)
	if err != nil {
		slog.Error("erro ao conferir PIN no login", "erro", err)
		response.JSONError(w, http.StatusInternalServerError, "erro ao autenticar")
		return
	}
	switch resultado {
	case pinUsuarioInvalido:
		response.JSONError(w, http.StatusUnauthorized, "usuário não encontrado ou inativo")
		return
	case pinJaBloqueado:
		liberarFalha()
		response.JSONError(w, http.StatusLocked, "usuário temporariamente bloqueado por excesso de tentativas. Tente novamente em "+minutosAte(bloqueadoAte)+".")
		return
	case pinBloqueou:
		slog.Warn("conta bloqueada por PIN errado", "usuario_id", req.UsuarioID, "papel", user.Papel, "ip", ip, "ate", bloqueadoAte)
		response.JSONError(w, http.StatusLocked, "PIN incorreto. Limite de 5 tentativas atingido. Usuário bloqueado por "+minutosAte(bloqueadoAte)+".")
		return
	case pinIncorreto:
		response.JSONError(w, http.StatusUnauthorized, "PIN incorreto")
		return
	}

	// 3. Sucesso: a falha reservada não conta
	liberarFalha()
	now := time.Now()

	// 4. Gerar token de sessão seguro (32 bytes aleatórios)
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao gerar sessão")
		return
	}
	rawToken := hex.EncodeToString(rawBytes)
	tokenHash := middleware.HashToken(rawToken)

	expiraEm := now.Add(h.cfg.SessionMaxTTL)
	_, err = h.db.Queries.CriarSessao(r.Context(), sqlc.CriarSessaoParams{
		TokenHash:   tokenHash,
		UsuarioID:   uID,
		ExpiraEm:    database.TimeToTimestamptz(expiraEm),
		UltimoUsoEm: database.TimeToTimestamptz(now),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao persistir sessão")
		return
	}

	// 5. Definir cookie de sessão HttpOnly
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.cfg.SessionMaxTTL.Seconds()),
	})

	// Sessão nova começa no setor da própria pessoa
	var setor SetorTrabalho
	if s, err := h.db.Queries.BuscarSetor(r.Context(), user.SetorID); err == nil {
		setor = SetorTrabalho{ID: database.UUIDToString(s.ID), Nome: s.Nome, Modulos: modulos.Ativos(s.ModulosDesativados)}
	}

	response.JSON(w, http.StatusOK, UserProfileResponse{
		ID:         database.UUIDToString(user.ID),
		Nome:       user.Nome,
		Cor:        user.Cor,
		Papel:      user.Papel,
		Tema:       user.Tema,
		FotoVersao: buscarFotoVersao(r.Context(), h.db.Queries, user.ID),

		DeveTrocarPin: user.DeveTrocarPin,
		Setor:         setor,
	})
}

// Logout: POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err == nil && cookie.Value != "" {
		tokenHash := middleware.HashToken(cookie.Value)
		_ = h.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
	}

	limparCookieSessao(w, h.cfg.CookieSecure)

	response.JSON(w, http.StatusOK, map[string]string{"message": "sessão encerrada com sucesso"})
}

func limparCookieSessao(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// Me: GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var fotoV *int64
	if uID, err := database.StringToUUID(user.ID); err == nil {
		fotoV = buscarFotoVersao(r.Context(), h.db.Queries, uID)
	}

	response.JSON(w, http.StatusOK, UserProfileResponse{
		ID:         user.ID,
		Nome:       user.Nome,
		Cor:        user.Cor,
		Papel:      user.Papel,
		Tema:       user.Tema,
		FotoVersao: fotoV,

		DeveTrocarPin: user.DeveTrocarPin,
		Setor: SetorTrabalho{
			ID:      database.UUIDToString(user.Setor),
			Nome:    user.SetorNome,
			Modulos: modulos.Ativos(user.ModulosDesativados),
		},
	})
}

type AtualizarTemaRequest struct {
	Tema string `json:"tema"`
}

// AtualizarTema: PUT /api/auth/tema
func (h *AuthHandler) AtualizarTema(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req AtualizarTemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Tema != "claro" && req.Tema != "escuro" && req.Tema != "sistema" {
		response.JSONError(w, http.StatusBadRequest, "tema inválido. Use claro, escuro ou sistema")
		return
	}

	uID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de usuário inválido")
		return
	}

	res, err := h.db.Queries.AtualizarTemaUsuario(r.Context(), sqlc.AtualizarTemaUsuarioParams{
		ID:   uID,
		Tema: req.Tema,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar tema")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "tema atualizado com sucesso",
		"tema":    res.Tema,
	})
}
