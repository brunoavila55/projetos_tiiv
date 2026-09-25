package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// Links úteis da equipe: qualquer operador cadastra; só o autor ou um admin
// editam e excluem (mesma regra do mural de avisos).
type LinkHandler struct {
	db *database.DB
}

func NewLinkHandler(db *database.DB) *LinkHandler {
	return &LinkHandler{db: db}
}

type LinkRequest struct {
	Titulo    string `json:"titulo"`
	URL       string `json:"url"`
	Descricao string `json:"descricao"`
	Categoria string `json:"categoria"`
}

type LinkResponse struct {
	ID          string `json:"id"`
	Titulo      string `json:"titulo"`
	URL         string `json:"url"`
	Descricao   string `json:"descricao"`
	Categoria   string `json:"categoria"`
	CriadorNome string `json:"criador_nome"`
	PodeEditar  bool   `json:"pode_editar"`
}

func podeAlterarLink(user *middleware.AuthUser, criadoPor pgtype.UUID) bool {
	return user.EhAdmin() || database.UUIDToString(criadoPor) == user.ID
}

// validarLink normaliza o corpo; só aceita http(s), o que barra javascript: e afins
func validarLink(req *LinkRequest) error {
	var err error
	if req.Titulo, err = validarTexto(req.Titulo, "título", true, 120); err != nil {
		return err
	}
	if req.URL, err = validarTexto(req.URL, "endereço", true, 2000); err != nil {
		return err
	}
	if req.Descricao, err = validarTexto(req.Descricao, "descrição", false, 300); err != nil {
		return err
	}
	if req.Categoria, err = validarTexto(req.Categoria, "categoria", false, 60); err != nil {
		return err
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(req.URL, " \t\r\n") {
		return errors.New("endereço inválido: use uma URL começando com http:// ou https://")
	}
	return nil
}

func lerLinkRequest(w http.ResponseWriter, r *http.Request) (LinkRequest, bool) {
	var req LinkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return req, false
	}
	if err := validarLink(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return req, false
	}
	return req, true
}

// Listar: GET /api/links
func (h *LinkHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	rows, err := h.db.Queries.ListarLinks(r.Context(), user.Setor)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar links")
		return
	}
	result := make([]LinkResponse, 0, len(rows))
	for _, l := range rows {
		result = append(result, LinkResponse{
			ID:          database.UUIDToString(l.ID),
			Titulo:      l.Titulo,
			URL:         l.Url,
			Descricao:   l.Descricao,
			Categoria:   l.Categoria,
			CriadorNome: l.CriadorNome,
			PodeEditar:  podeAlterarLink(user, l.CriadoPor),
		})
	}
	response.JSON(w, http.StatusOK, result)
}

// Criar: POST /api/links
func (h *LinkHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	req, ok := lerLinkRequest(w, r)
	if !ok {
		return
	}
	uID, _ := database.StringToUUID(user.ID)
	l, err := h.db.Queries.CriarLink(r.Context(), sqlc.CriarLinkParams{
		Titulo:    req.Titulo,
		Url:       req.URL,
		Descricao: req.Descricao,
		Categoria: req.Categoria,
		CriadoPor: uID,
		SetorID:   user.Setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar link")
		return
	}
	response.JSON(w, http.StatusCreated, map[string]string{"id": database.UUIDToString(l.ID)})
}

// buscarLinkAlteravel carrega o link e confere se o usuário é autor ou admin
func (h *LinkHandler) buscarLinkAlteravel(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
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
	l, err := h.db.Queries.ObterLink(r.Context(), sqlc.ObterLinkParams{ID: id, SetorID: user.Setor})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "link não encontrado")
		return pgtype.UUID{}, false
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar link")
		return pgtype.UUID{}, false
	}
	if !podeAlterarLink(user, l.CriadoPor) {
		response.JSONError(w, http.StatusForbidden, "só quem cadastrou o link ou um admin pode alterá-lo")
		return pgtype.UUID{}, false
	}
	return id, true
}

// Atualizar: PUT /api/links/{id} (autor ou admin)
func (h *LinkHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := h.buscarLinkAlteravel(w, r)
	if !ok {
		return
	}
	req, ok := lerLinkRequest(w, r)
	if !ok {
		return
	}
	if _, err := h.db.Queries.AtualizarLink(r.Context(), sqlc.AtualizarLinkParams{
		ID:        id,
		Titulo:    req.Titulo,
		Url:       req.URL,
		Descricao: req.Descricao,
		Categoria: req.Categoria,
	}); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Deletar: DELETE /api/links/{id} (autor ou admin)
func (h *LinkHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	id, ok := h.buscarLinkAlteravel(w, r)
	if !ok {
		return
	}
	if err := h.db.Queries.DeletarLink(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
