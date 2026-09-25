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

// setorDe devolve o setor de trabalho de quem fez a requisição (rotas com
// sessão). Sem sessão devolve um UUID inválido, que não casa com nenhum setor.
func setorDe(r *http.Request) pgtype.UUID {
	if user, ok := middleware.GetAuthUser(r.Context()); ok && user != nil {
		return user.Setor
	}
	return pgtype.UUID{}
}

var errSetorPedido = errors.New("escolha para qual setor é o pedido")

// setorDoPedido resolve o setor escolhido na tela de acesso. Sem escolha, vale
// o único setor que aceita pedidos (a tela nem mostra a opção nesse caso).
func setorDoPedido(ctx context.Context, q *sqlc.Queries, id string) (pgtype.UUID, error) {
	if id == "" {
		setores, err := q.ListarSetoresPedidos(ctx)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if len(setores) != 1 {
			return pgtype.UUID{}, errSetorPedido
		}
		return setores[0].ID, nil
	}
	sID, err := database.StringToUUID(id)
	if err != nil {
		return pgtype.UUID{}, errSetorPedido
	}
	s, err := q.BuscarSetor(ctx, sID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !s.AceitaPedidos) {
		return pgtype.UUID{}, errSetorPedido
	}
	return s.ID, err
}

// Setores: listagem para todos os logados; cadastro só do superadmin.
type SetorHandler struct {
	db *database.DB
}

func NewSetorHandler(db *database.DB) *SetorHandler {
	return &SetorHandler{db: db}
}

type SetorResponse struct {
	ID             string `json:"id"`
	Nome           string `json:"nome"`
	AceitaPedidos  bool   `json:"aceita_pedidos"`
	UsuariosAtivos int64  `json:"usuarios_ativos"`
	CriadoEm       string `json:"criado_em"`
}

type SetorPublico struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
}

// ListarPublico: GET /api/setores/publico — setores da tela de acesso
func (h *SetorHandler) ListarPublico(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Queries.ListarSetoresPedidos(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar setores")
		return
	}
	result := make([]SetorPublico, 0, len(rows))
	for _, s := range rows {
		result = append(result, SetorPublico{ID: database.UUIDToString(s.ID), Nome: s.Nome})
	}
	response.JSON(w, http.StatusOK, result)
}

// Listar: GET /api/setores
func (h *SetorHandler) Listar(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Queries.ListarSetores(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar setores")
		return
	}
	result := make([]SetorResponse, 0, len(rows))
	for _, s := range rows {
		result = append(result, SetorResponse{
			ID:             database.UUIDToString(s.ID),
			Nome:           s.Nome,
			AceitaPedidos:  s.AceitaPedidos,
			UsuariosAtivos: s.UsuariosAtivos,
			CriadoEm:       s.CriadoEm.Time.Format(time.RFC3339),
		})
	}
	response.JSON(w, http.StatusOK, result)
}

type SetorRequest struct {
	Nome          string `json:"nome"`
	AceitaPedidos *bool  `json:"aceita_pedidos"`
}

func lerSetor(w http.ResponseWriter, r *http.Request) (SetorRequest, bool) {
	var req SetorRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return req, false
	}
	var err error
	if req.Nome, err = validarTexto(req.Nome, "nome do setor", true, 60); err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return req, false
	}
	return req, true
}

// Criar: POST /api/setores (superadmin)
func (h *SetorHandler) Criar(w http.ResponseWriter, r *http.Request) {
	req, ok := lerSetor(w, r)
	if !ok {
		return
	}
	s, err := h.db.Queries.CriarSetor(r.Context(), sqlc.CriarSetorParams{
		Nome:          req.Nome,
		AceitaPedidos: req.AceitaPedidos == nil || *req.AceitaPedidos,
	})
	if codigoErroPg(err) == "23505" {
		response.JSONError(w, http.StatusConflict, "já existe um setor com esse nome")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar setor")
		return
	}
	response.JSON(w, http.StatusCreated, SetorResponse{
		ID:            database.UUIDToString(s.ID),
		Nome:          s.Nome,
		AceitaPedidos: s.AceitaPedidos,
		CriadoEm:      s.CriadoEm.Time.Format(time.RFC3339),
	})
}

// Atualizar: PUT /api/setores/{id} (superadmin)
func (h *SetorHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	req, ok := lerSetor(w, r)
	if !ok {
		return
	}
	atual, err := h.db.Queries.BuscarSetor(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "setor não encontrado")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar setor")
		return
	}
	aceita := atual.AceitaPedidos
	if req.AceitaPedidos != nil {
		aceita = *req.AceitaPedidos
	}
	s, err := h.db.Queries.AtualizarSetor(r.Context(), sqlc.AtualizarSetorParams{ID: id, Nome: req.Nome, AceitaPedidos: aceita})
	if codigoErroPg(err) == "23505" {
		response.JSONError(w, http.StatusConflict, "já existe um setor com esse nome")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar setor")
		return
	}
	response.JSON(w, http.StatusOK, SetorResponse{
		ID:            database.UUIDToString(s.ID),
		Nome:          s.Nome,
		AceitaPedidos: s.AceitaPedidos,
		CriadoEm:      s.CriadoEm.Time.Format(time.RFC3339),
	})
}

// Deletar: DELETE /api/setores/{id} (superadmin). Só apaga setor vazio: o
// banco recusa enquanto houver pessoas, registros ou monitores apontando nele.
func (h *SetorHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	n, err := h.db.Queries.DeletarSetor(r.Context(), id)
	if codigoErroPg(err) == "23503" {
		response.JSONError(w, http.StatusConflict, "o setor ainda tem pessoas ou registros (tickets, tarefas, calendário...); mova ou apague antes de excluir")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir setor")
		return
	}
	if n == 0 {
		response.JSONError(w, http.StatusNotFound, "setor não encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TrocarSetor: PUT /api/auth/setor (superadmin) — escolhe o setor que a
// sessão está vendo
func (h *SetorHandler) TrocarSetor(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	var req struct {
		SetorID string `json:"setor_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	sID, err := database.StringToUUID(req.SetorID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "setor inválido")
		return
	}
	s, err := h.db.Queries.BuscarSetor(r.Context(), sID)
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "setor não encontrado")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar setor")
		return
	}
	sessaoID, err := database.StringToUUID(user.SessaoID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "sessão inválida")
		return
	}
	if err := h.db.Queries.DefinirSetorSessao(r.Context(), sqlc.DefinirSetorSessaoParams{ID: sessaoID, SetorID: s.ID}); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao trocar de setor")
		return
	}
	response.JSON(w, http.StatusOK, SetorPublico{ID: database.UUIDToString(s.ID), Nome: s.Nome})
}

// ListarEquipe: GET /api/equipe — operadores ativos do setor atual
func (h *SetorHandler) ListarEquipe(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Queries.ListarEquipeSetor(r.Context(), setorDe(r))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar a equipe")
		return
	}
	result := make([]UsuarioPublico, 0, len(rows))
	for _, u := range rows {
		result = append(result, UsuarioPublico{
			ID:         database.UUIDToString(u.ID),
			Nome:       u.Nome,
			Cor:        u.Cor,
			FotoVersao: fotoVersao(u.FotoAtualizadaEm),
		})
	}
	response.JSON(w, http.StatusOK, result)
}
