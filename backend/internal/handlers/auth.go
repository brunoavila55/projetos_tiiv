package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// PIN de acesso: exatamente 4 dígitos numéricos
var pinRegex = regexp.MustCompile(`^[0-9]{4}$`)

type AuthHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAuthHandler(db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:  db,
		cfg: cfg,
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

	user, err := h.db.Queries.BuscarUsuarioPorIDComPin(r.Context(), uID)
	if err != nil || !user.Ativo {
		response.JSONError(w, http.StatusUnauthorized, "usuário não encontrado ou inativo")
		return
	}

	now := time.Now()

	// 1. Verificar se está bloqueado
	if user.BloqueadoAte.Valid && now.Before(user.BloqueadoAte.Time) {
		minutosRestantes := int(math.Ceil(time.Until(user.BloqueadoAte.Time).Minutes()))
		if minutosRestantes < 1 {
			minutosRestantes = 1
		}
		response.JSONError(w, http.StatusLocked, fmt.Sprintf("usuário temporariamente bloqueado por excesso de tentativas. Tente novamente em %d minuto(s).", minutosRestantes))
		return
	}

	// 2. Validar o PIN com bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(req.PIN))
	if err != nil {
		// PIN incorreto -> incrementa falhas e potencialmente bloqueia
		statusTentativa, _ := h.db.Queries.IncrementarTentativasFalhas(r.Context(), uID)
		if statusTentativa.BloqueadoAte.Valid && now.Before(statusTentativa.BloqueadoAte.Time) {
			response.JSONError(w, http.StatusLocked, "PIN incorreto. Limite de 5 tentativas atingido. Usuário bloqueado por 5 minutos.")
			return
		}
		response.JSONError(w, http.StatusUnauthorized, "PIN incorreto")
		return
	}

	// 3. Sucesso: zerar contador de falhas e desbloquear
	_ = h.db.Queries.ZerarTentativasFalhas(r.Context(), uID)

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

	response.JSON(w, http.StatusOK, UserProfileResponse{
		ID:         database.UUIDToString(user.ID),
		Nome:       user.Nome,
		Cor:        user.Cor,
		Papel:      user.Papel,
		Tema:       user.Tema,
		FotoVersao: buscarFotoVersao(r.Context(), h.db.Queries, user.ID),
	})
}

// Logout: POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err == nil && cookie.Value != "" {
		tokenHash := middleware.HashToken(cookie.Value)
		_ = h.db.Queries.DeletarSessaoPorHash(r.Context(), tokenHash)
	}

	// Limpar o cookie
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	response.JSON(w, http.StatusOK, map[string]string{"message": "sessão encerrada com sucesso"})
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
