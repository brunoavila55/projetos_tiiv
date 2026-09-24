package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type UsuarioHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewUsuarioHandler(db *database.DB, cfg *config.Config) *UsuarioHandler {
	return &UsuarioHandler{
		db:  db,
		cfg: cfg,
	}
}

type UsuarioItemResponse struct {
	ID               string  `json:"id"`
	Nome             string  `json:"nome"`
	Cor              string  `json:"cor"`
	Papel            string  `json:"papel"`
	Ativo            bool    `json:"ativo"`
	TentativasFalhas int32   `json:"tentativas_falhas"`
	BloqueadoAte     *string `json:"bloqueado_ate"`
	CriadoEm         string  `json:"criado_em"`
	FotoVersao       *int64  `json:"foto_versao"`
}

// Listar: GET /api/usuarios (Admin)
func (h *UsuarioHandler) Listar(w http.ResponseWriter, r *http.Request) {
	usuarios, err := h.db.Queries.ListarTodosUsuarios(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar usuários")
		return
	}

	result := make([]UsuarioItemResponse, 0, len(usuarios))
	for _, u := range usuarios {
		var bloqueadoAte *string
		if u.BloqueadoAte.Valid {
			str := u.BloqueadoAte.Time.Format("2006-01-02T15:04:05Z07:00")
			bloqueadoAte = &str
		}

		result = append(result, UsuarioItemResponse{
			ID:               database.UUIDToString(u.ID),
			Nome:             u.Nome,
			Cor:              u.Cor,
			Papel:            u.Papel,
			Ativo:            u.Ativo,
			TentativasFalhas: u.TentativasFalhas,
			BloqueadoAte:     bloqueadoAte,
			CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
			FotoVersao:       fotoVersao(u.FotoAtualizadaEm),
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type CriarUsuarioRequest struct {
	Nome  string `json:"nome"`
	Cor   string `json:"cor"`
	PIN   string `json:"pin"`
	Papel string `json:"papel"`
}

// Criar: POST /api/usuarios (Admin)
func (h *UsuarioHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req CriarUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Cor == "" || req.PIN == "" {
		response.JSONError(w, http.StatusBadRequest, "nome, cor e PIN são obrigatórios")
		return
	}

	if !pinRegex.MatchString(req.PIN) {
		response.JSONError(w, http.StatusBadRequest, "PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	if req.Papel != "admin" && req.Papel != "usuario" {
		req.Papel = "usuario"
	}

	pinHash, err := bcrypt.GenerateFromPassword([]byte(req.PIN), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar PIN")
		return
	}

	u, err := h.db.Queries.CriarUsuario(r.Context(), sqlc.CriarUsuarioParams{
		Nome:    req.Nome,
		Cor:     req.Cor,
		PinHash: string(pinHash),
		Papel:   req.Papel,
		Ativo:   true,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}

	response.JSON(w, http.StatusCreated, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

type AtualizarUsuarioRequest struct {
	Nome  string `json:"nome"`
	Cor   string `json:"cor"`
	Papel string `json:"papel"`
	Ativo bool   `json:"ativo"`
}

// Atualizar: PUT /api/usuarios/{id} (Admin)
func (h *UsuarioHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	uID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	var req AtualizarUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Cor == "" {
		response.JSONError(w, http.StatusBadRequest, "nome e cor são obrigatórios")
		return
	}

	if req.Papel != "admin" && req.Papel != "usuario" {
		req.Papel = "usuario"
	}

	// Verificar se é o último admin ativo antes de permitir desativar ou rebaixar
	atual, err := h.db.Queries.BuscarUsuarioPorID(r.Context(), uID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}

	if atual.Papel == "admin" && atual.Ativo && (req.Papel != "admin" || !req.Ativo) {
		adminCount, err := h.db.Queries.ContarAdminsAtivos(r.Context())
		if err == nil && adminCount <= 1 {
			response.JSONError(w, http.StatusBadRequest, "não é possível desativar ou rebaixar o único administrador ativo do sistema")
			return
		}
	}

	u, err := h.db.Queries.AtualizarUsuario(r.Context(), sqlc.AtualizarUsuarioParams{
		ID:    uID,
		Nome:  req.Nome,
		Cor:   req.Cor,
		Papel: req.Papel,
		Ativo: req.Ativo,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar usuário")
		return
	}

	// Se o usuário foi desativado, encerrar todas as suas sessões imediatamente
	if !req.Ativo {
		_ = h.db.Queries.DeletarSessoesPorUsuario(r.Context(), uID)
	}

	response.JSON(w, http.StatusOK, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

type RedefinirPinRequest struct {
	NovoPIN string `json:"novo_pin"`
}

// RedefinirPIN: POST /api/usuarios/{id}/pin (Admin)
func (h *UsuarioHandler) RedefinirPIN(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	uID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	var req RedefinirPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if !pinRegex.MatchString(req.NovoPIN) {
		response.JSONError(w, http.StatusBadRequest, "PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	pinHash, err := bcrypt.GenerateFromPassword([]byte(req.NovoPIN), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar PIN")
		return
	}

	_, err = h.db.Queries.AtualizarPin(r.Context(), sqlc.AtualizarPinParams{
		ID:      uID,
		PinHash: string(pinHash),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao redefinir PIN")
		return
	}

	// Desbloquear usuário caso estivesse bloqueado
	_ = h.db.Queries.ZerarTentativasFalhas(r.Context(), uID)

	response.JSON(w, http.StatusOK, map[string]string{"message": "PIN redefinido com sucesso"})
}

// Desbloquear: POST /api/usuarios/{id}/desbloquear (Admin)
func (h *UsuarioHandler) Desbloquear(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	uID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	u, err := h.db.Queries.DesbloquearUsuario(r.Context(), uID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao desbloquear usuário")
		return
	}

	response.JSON(w, http.StatusOK, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

type TrocarProprioPinRequest struct {
	PinAtual string `json:"pin_atual"`
	NovoPin  string `json:"novo_pin"`
}

// TrocarProprioPIN: POST /api/auth/trocar-pin (Qualquer usuário autenticado)
func (h *UsuarioHandler) TrocarProprioPIN(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req TrocarProprioPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if !pinRegex.MatchString(req.NovoPin) {
		response.JSONError(w, http.StatusBadRequest, "Novo PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	uID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "ID de usuário inválido na sessão")
		return
	}

	dbUser, err := h.db.Queries.BuscarUsuarioPorIDComPin(r.Context(), uID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "usuário não encontrado")
		return
	}

	// Valida o PIN atual
	if err := bcrypt.CompareHashAndPassword([]byte(dbUser.PinHash), []byte(req.PinAtual)); err != nil {
		response.JSONError(w, http.StatusUnauthorized, "PIN atual incorreto")
		return
	}

	novoHash, err := bcrypt.GenerateFromPassword([]byte(req.NovoPin), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar novo PIN")
		return
	}

	_, err = h.db.Queries.AtualizarPin(r.Context(), sqlc.AtualizarPinParams{
		ID:      uID,
		PinHash: string(novoHash),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar PIN")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "PIN alterado com sucesso"})
}
