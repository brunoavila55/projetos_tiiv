package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type AtendimentoHandler struct {
	db *database.DB
}

func NewAtendimentoHandler(db *database.DB) *AtendimentoHandler {
	return &AtendimentoHandler{db: db}
}

type AtendimentoItemResponse struct {
	ID              string `json:"id"`
	ClienteNome     string `json:"cliente_nome"`
	Descricao       string `json:"descricao"`
	DataAtendimento string `json:"data_atendimento"`
	UsuarioID       string `json:"usuario_id"`
	UsuarioNome     string `json:"usuario_nome"`
	UsuarioCor      string `json:"usuario_cor"`
	CriadoEm        string `json:"criado_em"`
	AtualizadoEm    string `json:"atualizado_em"`
	PodeEditar      bool   `json:"pode_editar"`
}

type ListarAtendimentosResponse struct {
	Itens      []AtendimentoItemResponse `json:"itens"`
	Total      int64                     `json:"total"`
	Pagina     int                       `json:"pagina"`
	Limite     int                       `json:"limite"`
	TotalPages int                       `json:"total_pages"`
}

// Listar: GET /api/atendimentos?busca=&usuario_id=&inicio=&fim=&pagina=&limite=
func (h *AtendimentoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	q := r.URL.Query()
	busca := q.Get("busca")
	usuarioIDStr := q.Get("usuario_id")
	inicioStr := q.Get("inicio")
	fimStr := q.Get("fim")

	pagina, _ := strconv.Atoi(q.Get("pagina"))
	if pagina < 1 {
		pagina = 1
	}

	limite, _ := strconv.Atoi(q.Get("limite"))
	if limite < 1 || limite > 100 {
		limite = 20
	}
	offset := (pagina - 1) * limite

	var usuarioID pgtype.UUID
	if usuarioIDStr != "" {
		usuarioID, _ = database.StringToUUID(usuarioIDStr)
	}

	var dataInicio, dataFim pgtype.Timestamptz
	if inicioStr != "" {
		if t, err := time.Parse(time.RFC3339, inicioStr); err == nil {
			dataInicio = database.TimeToTimestamptz(t)
		} else if t, err := time.Parse("2006-01-02", inicioStr); err == nil {
			dataInicio = database.TimeToTimestamptz(t)
		}
	}
	if fimStr != "" {
		if t, err := time.Parse(time.RFC3339, fimStr); err == nil {
			dataFim = database.TimeToTimestamptz(t)
		} else if t, err := time.Parse("2006-01-02", fimStr); err == nil {
			// Definir até o final do dia
			dataFim = database.TimeToTimestamptz(t.Add(23*time.Hour + 59*time.Minute + 59*time.Second))
		}
	}

	buscaParam := database.StringToText(busca)

	total, err := h.db.Queries.ContarAtendimentos(r.Context(), sqlc.ContarAtendimentosParams{
		Busca:         buscaParam,
		UsuarioID:     usuarioID,
		DataInicio:    dataInicio,
		DataFim:       dataFim,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao contar atendimentos")
		return
	}

	itens, err := h.db.Queries.ListarAtendimentos(r.Context(), sqlc.ListarAtendimentosParams{
		Limit:         int32(limite),
		Offset:        int32(offset),
		Busca:         buscaParam,
		UsuarioID:     usuarioID,
		DataInicio:    dataInicio,
		DataFim:       dataFim,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar atendimentos")
		return
	}

	result := make([]AtendimentoItemResponse, 0, len(itens))
	for _, item := range itens {
		uID := database.UUIDToString(item.UsuarioID)
		podeEditar := (user.Papel == "admin") || (uID == user.ID)

		result = append(result, AtendimentoItemResponse{
			ID:              database.UUIDToString(item.ID),
			ClienteNome:     item.ClienteNome,
			Descricao:       item.Descricao,
			DataAtendimento: item.DataAtendimento.Time.Format(time.RFC3339),
			UsuarioID:       uID,
			UsuarioNome:     item.UsuarioNome,
			UsuarioCor:      item.UsuarioCor,
			CriadoEm:        item.CriadoEm.Time.Format(time.RFC3339),
			AtualizadoEm:    item.AtualizadoEm.Time.Format(time.RFC3339),
			PodeEditar:      podeEditar,
		})
	}

	totalPages := int((total + int64(limite) - 1) / int64(limite))
	if totalPages < 1 {
		totalPages = 1
	}

	response.JSON(w, http.StatusOK, ListarAtendimentosResponse{
		Itens:      result,
		Total:      total,
		Pagina:     pagina,
		Limite:     limite,
		TotalPages: totalPages,
	})
}

// Obter: GET /api/atendimentos/{id}
func (h *AtendimentoHandler) Obter(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	aID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	item, err := h.db.Queries.BuscarAtendimentoPorID(r.Context(), aID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "atendimento não encontrado")
		return
	}

	uID := database.UUIDToString(item.UsuarioID)
	podeEditar := (user.Papel == "admin") || (uID == user.ID)

	response.JSON(w, http.StatusOK, AtendimentoItemResponse{
		ID:              database.UUIDToString(item.ID),
		ClienteNome:     item.ClienteNome,
		Descricao:       item.Descricao,
		DataAtendimento: item.DataAtendimento.Time.Format(time.RFC3339),
		UsuarioID:       uID,
		UsuarioNome:     item.UsuarioNome,
		UsuarioCor:      item.UsuarioCor,
		CriadoEm:        item.CriadoEm.Time.Format(time.RFC3339),
		AtualizadoEm:    item.AtualizadoEm.Time.Format(time.RFC3339),
		PodeEditar:      podeEditar,
	})
}

type AtendimentoRequest struct {
	ClienteNome     string  `json:"cliente_nome"`
	Descricao       string  `json:"descricao"`
	DataAtendimento *string `json:"data_atendimento"`
}

// Criar: POST /api/atendimentos
func (h *AtendimentoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req AtendimentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.ClienteNome == "" || req.Descricao == "" {
		response.JSONError(w, http.StatusBadRequest, "nome do cliente e descrição são obrigatórios")
		return
	}

	dataAtendimento := time.Now()
	if req.DataAtendimento != nil && *req.DataAtendimento != "" {
		if t, err := time.Parse(time.RFC3339, *req.DataAtendimento); err == nil {
			dataAtendimento = t
		}
	}

	uUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar usuário")
		return
	}

	item, err := h.db.Queries.CriarAtendimento(r.Context(), sqlc.CriarAtendimentoParams{
		ClienteNome:     req.ClienteNome,
		Descricao:       req.Descricao,
		DataAtendimento: database.TimeToTimestamptz(dataAtendimento),
		UsuarioID:       uUUID,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar atendimento")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]string{
		"id":      database.UUIDToString(item.ID),
		"message": "atendimento registrado com sucesso",
	})
}

// Atualizar: PUT /api/atendimentos/{id}
func (h *AtendimentoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	aID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	item, err := h.db.Queries.BuscarAtendimentoPorID(r.Context(), aID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "atendimento não encontrado")
		return
	}

	uID := database.UUIDToString(item.UsuarioID)
	if user.Papel != "admin" && uID != user.ID {
		response.JSONError(w, http.StatusForbidden, "apenas o autor ou administrador pode editar este atendimento")
		return
	}

	var req AtendimentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.ClienteNome == "" || req.Descricao == "" {
		response.JSONError(w, http.StatusBadRequest, "nome do cliente e descrição são obrigatórios")
		return
	}

	dataAtendimento := item.DataAtendimento.Time
	if req.DataAtendimento != nil && *req.DataAtendimento != "" {
		if t, err := time.Parse(time.RFC3339, *req.DataAtendimento); err == nil {
			dataAtendimento = t
		}
	}

	_, err = h.db.Queries.AtualizarAtendimento(r.Context(), sqlc.AtualizarAtendimentoParams{
		ID:              aID,
		ClienteNome:     req.ClienteNome,
		Descricao:       req.Descricao,
		DataAtendimento: database.TimeToTimestamptz(dataAtendimento),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar atendimento")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "atendimento atualizado com sucesso"})
}

// Deletar: DELETE /api/atendimentos/{id}
func (h *AtendimentoHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	aID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	item, err := h.db.Queries.BuscarAtendimentoPorID(r.Context(), aID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "atendimento não encontrado")
		return
	}

	uID := database.UUIDToString(item.UsuarioID)
	if user.Papel != "admin" && uID != user.ID {
		response.JSONError(w, http.StatusForbidden, "apenas o autor ou administrador pode excluir este atendimento")
		return
	}

	if err := h.db.Queries.DeletarAtendimento(r.Context(), aID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir atendimento")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "atendimento excluído com sucesso"})
}

// ExportarCSV: GET /api/atendimentos/exportar.csv
func (h *AtendimentoHandler) ExportarCSV(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	q := r.URL.Query()
	busca := q.Get("busca")
	usuarioIDStr := q.Get("usuario_id")
	inicioStr := q.Get("inicio")
	fimStr := q.Get("fim")

	var usuarioID pgtype.UUID
	if usuarioIDStr != "" {
		if uid, err := database.StringToUUID(usuarioIDStr); err == nil {
			usuarioID = uid
		}
	}

	var dataInicio, dataFim pgtype.Timestamptz
	if inicioStr != "" {
		if t, err := time.Parse("2006-01-02", inicioStr); err == nil {
			dataInicio = database.TimeToTimestamptz(t)
		} else if t, err := time.Parse(time.RFC3339, inicioStr); err == nil {
			dataInicio = database.TimeToTimestamptz(t)
		}
	}
	if fimStr != "" {
		if t, err := time.Parse("2006-01-02", fimStr); err == nil {
			fimDoDia := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			dataFim = database.TimeToTimestamptz(fimDoDia)
		} else if t, err := time.Parse(time.RFC3339, fimStr); err == nil {
			dataFim = database.TimeToTimestamptz(t)
		}
	}

	// Exportar sem paginação (limite grande)
	itens, err := h.db.Queries.ListarAtendimentos(r.Context(), sqlc.ListarAtendimentosParams{
		Busca:      database.StringToText(busca),
		UsuarioID:  usuarioID,
		DataInicio: dataInicio,
		DataFim:    dataFim,
		Limit:      100000,
		Offset:     0,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar atendimentos para exportação")
		return
	}

	filename := fmt.Sprintf("atendimentos_%s.csv", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// UTF-8 BOM para garantir correta abertura com acentos no Excel e LibreOffice
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';' // Separador padrão para compatibilidade em português com Excel

	_ = writer.Write([]string{"Data/Hora", "Cliente", "Descrição", "Técnico Responsável"})
	for _, it := range itens {
		dataStr := ""
		if it.DataAtendimento.Valid {
			dataStr = it.DataAtendimento.Time.Format("02/01/2006 15:04")
		}
		_ = writer.Write([]string{
			dataStr,
			it.ClienteNome,
			it.Descricao,
			it.UsuarioNome,
		})
	}
	writer.Flush()
}
