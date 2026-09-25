package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

const maxCorpoProcedimento = 20000

type ProcedimentoHandler struct {
	db *database.DB
}

func NewProcedimentoHandler(db *database.DB) *ProcedimentoHandler {
	return &ProcedimentoHandler{db: db}
}

type ProcedimentoResumo struct {
	ID                string `json:"id"`
	Titulo            string `json:"titulo"`
	Categoria         string `json:"categoria"`
	Ativo             bool   `json:"ativo"`
	AtualizadoEm      string `json:"atualizado_em"`
	AtualizadoPorNome string `json:"atualizado_por_nome"`
}

type ProcedimentoResponse struct {
	ProcedimentoResumo
	Corpo    string `json:"corpo"`
	CriadoEm string `json:"criado_em"`
}

type RevisaoResponse struct {
	ID             string `json:"id"`
	Titulo         string `json:"titulo"`
	Categoria      string `json:"categoria"`
	Corpo          string `json:"corpo"`
	Ativo          bool   `json:"ativo"`
	Nota           string `json:"nota"`
	EditadoPorNome string `json:"editado_por_nome"`
	CriadoEm       string `json:"criado_em"`
}

type SalvarProcedimentoRequest struct {
	Titulo    string `json:"titulo"`
	Categoria string `json:"categoria"`
	Corpo     string `json:"corpo"`
	Ativo     *bool  `json:"ativo"`
}

// Listar: GET /api/procedimentos?busca=&categoria=&ativo=
func (h *ProcedimentoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var ativo *bool
	if b, err := strconv.ParseBool(q.Get("ativo")); err == nil {
		ativo = &b
	}

	rows, err := h.db.Queries.ListarProcedimentos(r.Context(), sqlc.ListarProcedimentosParams{
		Busca:     database.StringToText(q.Get("busca")),
		Categoria: database.StringToText(q.Get("categoria")),
		Ativo:     database.BoolToPgtypeBool(ativo),
		SetorID:   setorDe(r),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar procedimentos")
		return
	}

	result := make([]ProcedimentoResumo, 0, len(rows))
	for _, p := range rows {
		result = append(result, ProcedimentoResumo{
			ID:                database.UUIDToString(p.ID),
			Titulo:            p.Titulo,
			Categoria:         p.Categoria,
			Ativo:             p.Ativo,
			AtualizadoEm:      p.AtualizadoEm.Time.Format(time.RFC3339),
			AtualizadoPorNome: p.AtualizadoPorNome,
		})
	}
	response.JSON(w, http.StatusOK, result)
}

// ListarCategorias: GET /api/procedimentos/categorias
func (h *ProcedimentoHandler) ListarCategorias(w http.ResponseWriter, r *http.Request) {
	cats, err := h.db.Queries.ListarCategoriasProcedimentos(r.Context(), setorDe(r))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar categorias")
		return
	}
	if cats == nil {
		cats = []string{}
	}
	response.JSON(w, http.StatusOK, cats)
}

// Obter: GET /api/procedimentos/{id}
func (h *ProcedimentoHandler) Obter(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	h.responderProcedimento(w, r, id, http.StatusOK)
}

func (h *ProcedimentoHandler) responderProcedimento(w http.ResponseWriter, r *http.Request, id pgtype.UUID, status int) {
	p, err := h.db.Queries.BuscarProcedimentoPorID(r.Context(), sqlc.BuscarProcedimentoPorIDParams{ID: id, SetorID: setorDe(r)})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "procedimento não encontrado")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar procedimento")
		return
	}
	response.JSON(w, status, ProcedimentoResponse{
		ProcedimentoResumo: ProcedimentoResumo{
			ID:                database.UUIDToString(p.ID),
			Titulo:            p.Titulo,
			Categoria:         p.Categoria,
			Ativo:             p.Ativo,
			AtualizadoEm:      p.AtualizadoEm.Time.Format(time.RFC3339),
			AtualizadoPorNome: p.AtualizadoPorNome,
		},
		Corpo:    p.Corpo,
		CriadoEm: p.CriadoEm.Time.Format(time.RFC3339),
	})
}

func lerProcedimento(w http.ResponseWriter, r *http.Request) (SalvarProcedimentoRequest, bool) {
	var req SalvarProcedimentoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return req, false
	}
	var err error
	campos := []struct {
		valor       *string
		nome        string
		obrigatorio bool
		max         int
	}{
		{&req.Titulo, "título", true, 200},
		{&req.Categoria, "categoria", false, 80},
		{&req.Corpo, "texto do procedimento", true, maxCorpoProcedimento},
	}
	for _, c := range campos {
		if *c.valor, err = validarTexto(*c.valor, c.nome, c.obrigatorio, c.max); err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return req, false
		}
	}
	return req, true
}

func usuarioLogado(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return pgtype.UUID{}, false
	}
	id, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return pgtype.UUID{}, false
	}
	return id, true
}

// Criar: POST /api/procedimentos (Admin)
func (h *ProcedimentoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	uID, ok := usuarioLogado(w, r)
	if !ok {
		return
	}
	req, ok := lerProcedimento(w, r)
	if !ok {
		return
	}
	ativo := req.Ativo == nil || *req.Ativo

	var id pgtype.UUID
	err := h.emTransacao(r, func(qtx *sqlc.Queries) error {
		p, err := qtx.CriarProcedimento(r.Context(), sqlc.CriarProcedimentoParams{
			Titulo:    req.Titulo,
			Categoria: req.Categoria,
			Corpo:     req.Corpo,
			Ativo:     ativo,
			CriadoPor: uID,
			SetorID:   setorDe(r),
		})
		if err != nil {
			return err
		}
		id = p.ID
		return qtx.CriarRevisaoProcedimento(r.Context(), revisaoDe(p, "Criado", uID))
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar procedimento")
		return
	}
	h.responderProcedimento(w, r, id, http.StatusCreated)
}

// Atualizar: PUT /api/procedimentos/{id} (Admin)
func (h *ProcedimentoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	uID, ok := usuarioLogado(w, r)
	if !ok {
		return
	}
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	req, ok := lerProcedimento(w, r)
	if !ok {
		return
	}

	err = h.emTransacao(r, func(qtx *sqlc.Queries) error {
		atual, err := qtx.BuscarProcedimentoPorID(r.Context(), sqlc.BuscarProcedimentoPorIDParams{ID: id, SetorID: setorDe(r)})
		if err != nil {
			return err
		}
		ativo := atual.Ativo
		if req.Ativo != nil {
			ativo = *req.Ativo
		}
		// Salvar sem mudanças não gera revisão
		if atual.Titulo == req.Titulo && atual.Categoria == req.Categoria && atual.Corpo == req.Corpo && atual.Ativo == ativo {
			return nil
		}

		nota := "Editado"
		switch {
		case atual.Ativo && !ativo:
			nota = "Desativado"
		case !atual.Ativo && ativo:
			nota = "Reativado"
		}
		return h.gravar(r, qtx, id, req.Titulo, req.Categoria, req.Corpo, ativo, nota, uID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "procedimento não encontrado")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao salvar procedimento")
		return
	}
	h.responderProcedimento(w, r, id, http.StatusOK)
}

// ListarRevisoes: GET /api/procedimentos/{id}/revisoes (Admin)
func (h *ProcedimentoHandler) ListarRevisoes(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	rows, err := h.db.Queries.ListarRevisoesProcedimento(r.Context(), sqlc.ListarRevisoesProcedimentoParams{ProcedimentoID: id, SetorID: setorDe(r)})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar histórico")
		return
	}
	result := make([]RevisaoResponse, 0, len(rows))
	for _, rv := range rows {
		result = append(result, RevisaoResponse{
			ID:             database.UUIDToString(rv.ID),
			Titulo:         rv.Titulo,
			Categoria:      rv.Categoria,
			Corpo:          rv.Corpo,
			Ativo:          rv.Ativo,
			Nota:           rv.Nota,
			EditadoPorNome: rv.EditadoPorNome,
			CriadoEm:       rv.CriadoEm.Time.Format(time.RFC3339),
		})
	}
	response.JSON(w, http.StatusOK, result)
}

// Restaurar: POST /api/procedimentos/{id}/revisoes/{rid}/restaurar (Admin)
// Copia a versão antiga para o procedimento e gera uma revisão nova; o
// histórico nunca é reescrito.
func (h *ProcedimentoHandler) Restaurar(w http.ResponseWriter, r *http.Request) {
	uID, ok := usuarioLogado(w, r)
	if !ok {
		return
	}
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	rid, err := database.StringToUUID(chi.URLParam(r, "rid"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de revisão inválido")
		return
	}

	err = h.emTransacao(r, func(qtx *sqlc.Queries) error {
		rv, err := qtx.BuscarRevisaoProcedimento(r.Context(), sqlc.BuscarRevisaoProcedimentoParams{ID: rid, ProcedimentoID: id, SetorID: setorDe(r)})
		if err != nil {
			return err
		}
		nota := fmt.Sprintf("Restaurado da versão de %s", rv.CriadoEm.Time.In(fusoSaoPaulo).Format("02/01/2006 15:04"))
		return h.gravar(r, qtx, id, rv.Titulo, rv.Categoria, rv.Corpo, rv.Ativo, nota, uID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "versão não encontrada")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao restaurar versão")
		return
	}
	h.responderProcedimento(w, r, id, http.StatusOK)
}

// gravar atualiza o procedimento e registra a revisão correspondente
func (h *ProcedimentoHandler) gravar(r *http.Request, qtx *sqlc.Queries, id pgtype.UUID, titulo, categoria, corpo string, ativo bool, nota string, uID pgtype.UUID) error {
	p, err := qtx.AtualizarProcedimento(r.Context(), sqlc.AtualizarProcedimentoParams{
		ID:            id,
		Titulo:        titulo,
		Categoria:     categoria,
		Corpo:         corpo,
		Ativo:         ativo,
		AtualizadoPor: uID,
		SetorID:       setorDe(r),
	})
	if err != nil {
		return err
	}
	return qtx.CriarRevisaoProcedimento(r.Context(), revisaoDe(p, nota, uID))
}

func revisaoDe(p sqlc.Procedimentos, nota string, uID pgtype.UUID) sqlc.CriarRevisaoProcedimentoParams {
	return sqlc.CriarRevisaoProcedimentoParams{
		ProcedimentoID: p.ID,
		Titulo:         p.Titulo,
		Categoria:      p.Categoria,
		Corpo:          p.Corpo,
		Ativo:          p.Ativo,
		Nota:           nota,
		EditadoPor:     uID,
	}
}

func (h *ProcedimentoHandler) emTransacao(r *http.Request, fn func(qtx *sqlc.Queries) error) error {
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err := fn(h.db.Queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}
