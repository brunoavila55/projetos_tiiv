package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// Mural de avisos do painel: qualquer operador fixa um recado; só o autor ou
// um admin editam e excluem. Avisos vencidos deixam de ser listados.
type AvisoHandler struct {
	db *database.DB
}

func NewAvisoHandler(db *database.DB) *AvisoHandler {
	return &AvisoHandler{db: db}
}

type AvisoRequest struct {
	Titulo   string  `json:"titulo"`
	Mensagem string  `json:"mensagem"`
	Nivel    string  `json:"nivel"`
	ExpiraEm *string `json:"expira_em"`
}

type AvisoResponse struct {
	ID          string  `json:"id"`
	Titulo      string  `json:"titulo"`
	Mensagem    string  `json:"mensagem"`
	Nivel       string  `json:"nivel"`
	ExpiraEm    *string `json:"expira_em"`
	CriadoPor   string  `json:"criado_por"`
	CriadorNome string  `json:"criador_nome"`
	CriadorCor  string  `json:"criador_cor"`
	CriadoEm    string  `json:"criado_em"`
	PodeEditar  bool    `json:"pode_editar"`
}

func podeAlterarAviso(user *middleware.AuthUser, criadoPor pgtype.UUID) bool {
	return user.EhAdmin() || database.UUIDToString(criadoPor) == user.ID
}

// listarAvisosAtivos também alimenta o GET /api/painel.
func listarAvisosAtivos(ctx context.Context, q *sqlc.Queries, user *middleware.AuthUser) ([]AvisoResponse, error) {
	rows, err := q.ListarAvisosAtivos(ctx, user.Setor)
	if err != nil {
		return nil, err
	}
	result := make([]AvisoResponse, 0, len(rows))
	for _, a := range rows {
		result = append(result, AvisoResponse{
			ID:          database.UUIDToString(a.ID),
			Titulo:      a.Titulo,
			Mensagem:    a.Mensagem,
			Nivel:       a.Nivel,
			ExpiraEm:    formatarTimestamptz(a.ExpiraEm),
			CriadoPor:   database.UUIDToString(a.CriadoPor),
			CriadorNome: a.CriadorNome,
			CriadorCor:  a.CriadorCor,
			CriadoEm:    a.CriadoEm.Time.Format(time.RFC3339),
			PodeEditar:  podeAlterarAviso(user, a.CriadoPor),
		})
	}
	return result, nil
}

// validarAviso normaliza o corpo e devolve a expiração (Valid=false = sem prazo).
func validarAviso(req *AvisoRequest) (pgtype.Timestamptz, error) {
	var err error
	if req.Titulo, err = validarTexto(req.Titulo, "título", true, 200); err != nil {
		return pgtype.Timestamptz{}, err
	}
	if req.Mensagem, err = validarTexto(req.Mensagem, "mensagem", false, 2000); err != nil {
		return pgtype.Timestamptz{}, err
	}
	if req.Nivel == "" {
		req.Nivel = "info"
	}
	if req.Nivel != "info" && req.Nivel != "atencao" && req.Nivel != "critico" {
		return pgtype.Timestamptz{}, errors.New("nível inválido")
	}

	if req.ExpiraEm == nil || *req.ExpiraEm == "" {
		return pgtype.Timestamptz{}, nil
	}
	expira, err := time.Parse(time.RFC3339, *req.ExpiraEm)
	if err != nil {
		return pgtype.Timestamptz{}, errors.New("data de expiração inválida")
	}
	if !expira.After(time.Now()) {
		return pgtype.Timestamptz{}, errors.New("a data de expiração precisa estar no futuro")
	}
	return database.TimeToTimestamptz(expira), nil
}

// Listar: GET /api/avisos — só os que ainda não expiraram
func (h *AvisoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	avisos, err := listarAvisosAtivos(r.Context(), h.db.Queries, user)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar avisos")
		return
	}
	response.JSON(w, http.StatusOK, avisos)
}

// Criar: POST /api/avisos
func (h *AvisoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req AvisoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	expira, err := validarAviso(&req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	uID, _ := database.StringToUUID(user.ID)
	a, err := h.db.Queries.CriarAviso(r.Context(), sqlc.CriarAvisoParams{
		Titulo:    req.Titulo,
		Mensagem:  req.Mensagem,
		Nivel:     req.Nivel,
		ExpiraEm:  expira,
		CriadoPor: uID,
		SetorID:   user.Setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar aviso")
		return
	}
	response.JSON(w, http.StatusCreated, map[string]string{"id": database.UUIDToString(a.ID)})
}

// buscarAvisoAlteravel carrega o aviso e confere se o usuário é autor ou admin.
func (h *AvisoHandler) buscarAvisoAlteravel(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return pgtype.UUID{}, false
	}
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return pgtype.UUID{}, false
	}

	a, err := h.db.Queries.ObterAviso(r.Context(), sqlc.ObterAvisoParams{ID: id, SetorID: user.Setor})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "aviso não encontrado")
		return pgtype.UUID{}, false
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar aviso")
		return pgtype.UUID{}, false
	}
	if !podeAlterarAviso(user, a.CriadoPor) {
		response.JSONError(w, http.StatusForbidden, "só quem criou o aviso ou um admin pode alterá-lo")
		return pgtype.UUID{}, false
	}
	return id, true
}

// Atualizar: PUT /api/avisos/{id} (autor ou admin)
func (h *AvisoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := h.buscarAvisoAlteravel(w, r)
	if !ok {
		return
	}

	var req AvisoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	expira, err := validarAviso(&req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.db.Queries.AtualizarAviso(r.Context(), sqlc.AtualizarAvisoParams{
		ID:       id,
		Titulo:   req.Titulo,
		Mensagem: req.Mensagem,
		Nivel:    req.Nivel,
		ExpiraEm: expira,
	}); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar aviso")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Deletar: DELETE /api/avisos/{id} (autor ou admin)
func (h *AvisoHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	id, ok := h.buscarAvisoAlteravel(w, r)
	if !ok {
		return
	}
	if err := h.db.Queries.DeletarAviso(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir aviso")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
