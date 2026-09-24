package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type TarefaHandler struct {
	db *database.DB
}

func NewTarefaHandler(db *database.DB) *TarefaHandler {
	return &TarefaHandler{db: db}
}

type TarefaItemResponse struct {
	ID              string  `json:"id"`
	Titulo          string  `json:"titulo"`
	Descricao       string  `json:"descricao"`
	Status          string  `json:"status"`
	Prioridade      string  `json:"prioridade"`
	Prazo           *string `json:"prazo"`
	CriadoPor       string  `json:"criado_por"`
	CriadorNome     string  `json:"criador_nome"`
	CriadorCor      string  `json:"criador_cor"`
	ResponsavelID   *string `json:"responsavel_id"`
	ResponsavelNome *string `json:"responsavel_nome"`
	ResponsavelCor  *string `json:"responsavel_cor"`
	ConcluidaEm     *string `json:"concluida_em"`
	CriadoEm        string  `json:"criado_em"`
	AtualizadoEm    string  `json:"atualizado_em"`
	Atrasada        bool    `json:"atrasada"`
	PodeEditar      bool    `json:"pode_editar"`
	PodeExcluir     bool    `json:"pode_excluir"`
}

// Listar: GET /api/tarefas?visao=minhas|criadas|todas&status=
func (h *TarefaHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	visao := r.URL.Query().Get("visao") // "minhas", "criadas", "todas"
	statusStr := r.URL.Query().Get("status")

	var responsavelID, criadoPor pgtype.UUID
	userUUID, _ := database.StringToUUID(user.ID)

	switch visao {
	case "minhas":
		responsavelID = userUUID
	case "criadas":
		criadoPor = userUUID
	}

	// Operador comum só enxerga as próprias tarefas; admin vê todas
	var visivelPara pgtype.UUID
	if user.Papel != "admin" {
		visivelPara = userUUID
	}

	statusParam := database.StringToText(statusStr)

	tarefas, err := h.db.Queries.ListarTarefas(r.Context(), sqlc.ListarTarefasParams{
		ResponsavelID: responsavelID,
		CriadoPor:     criadoPor,
		Status:        statusParam,
		VisivelPara:   visivelPara,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar tarefas")
		return
	}

	now := time.Now()
	result := make([]TarefaItemResponse, 0, len(tarefas))
	for _, t := range tarefas {
		tCriadoPor := database.UUIDToString(t.CriadoPor)

		var respID *string
		var respNome *string
		var respCor *string
		if t.ResponsavelID.Valid {
			str := database.UUIDToString(t.ResponsavelID)
			respID = &str
			respNome = database.TextToString(t.ResponsavelNome)
			respCor = database.TextToString(t.ResponsavelCor)
		}

		var prazoStr *string
		atrasada := false
		if t.Prazo.Valid {
			str := t.Prazo.Time.Format(time.RFC3339)
			prazoStr = &str
			if t.Status != "concluida" && t.Prazo.Time.Before(now) {
				atrasada = true
			}
		}

		var concluidaEmStr *string
		if t.ConcluidaEm.Valid {
			str := t.ConcluidaEm.Time.Format(time.RFC3339)
			concluidaEmStr = &str
		}

		podeEditar := (user.Papel == "admin") || (tCriadoPor == user.ID) || (respID != nil && *respID == user.ID)
		podeExcluir := (user.Papel == "admin") || (tCriadoPor == user.ID)

		result = append(result, TarefaItemResponse{
			ID:              database.UUIDToString(t.ID),
			Titulo:          t.Titulo,
			Descricao:       t.Descricao,
			Status:          t.Status,
			Prioridade:      t.Prioridade,
			Prazo:           prazoStr,
			CriadoPor:       tCriadoPor,
			CriadorNome:     t.CriadorNome,
			CriadorCor:      t.CriadorCor,
			ResponsavelID:   respID,
			ResponsavelNome: respNome,
			ResponsavelCor:  respCor,
			ConcluidaEm:     concluidaEmStr,
			CriadoEm:        t.CriadoEm.Time.Format(time.RFC3339),
			AtualizadoEm:    t.AtualizadoEm.Time.Format(time.RFC3339),
			Atrasada:        atrasada,
			PodeEditar:      podeEditar,
			PodeExcluir:     podeExcluir,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type TarefaRequest struct {
	Titulo        string  `json:"titulo"`
	Descricao     string  `json:"descricao"`
	Prioridade    string  `json:"prioridade"`
	Prazo         *string `json:"prazo"`
	ResponsavelID *string `json:"responsavel_id"`
}

// Criar: POST /api/tarefas
func (h *TarefaHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req TarefaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Titulo == "" {
		response.JSONError(w, http.StatusBadRequest, "título da tarefa é obrigatório")
		return
	}

	if req.Prioridade != "baixa" && req.Prioridade != "media" && req.Prioridade != "alta" {
		req.Prioridade = "media"
	}

	var prazo pgtype.Timestamptz
	if req.Prazo != nil && *req.Prazo != "" {
		if t, err := time.Parse(time.RFC3339, *req.Prazo); err == nil {
			prazo = database.TimeToTimestamptz(t)
		} else if t, err := time.Parse("2006-01-02", *req.Prazo); err == nil {
			prazo = database.TimeToTimestamptz(t.Add(23*time.Hour + 59*time.Minute))
		}
	}

	var responsavelUUID pgtype.UUID
	if req.ResponsavelID != nil && *req.ResponsavelID != "" {
		responsavelUUID, _ = database.StringToUUID(*req.ResponsavelID)
	}

	criadorUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar criador")
		return
	}

	tarefa, err := h.db.Queries.CriarTarefa(r.Context(), sqlc.CriarTarefaParams{
		Titulo:        req.Titulo,
		Descricao:     req.Descricao,
		Prioridade:    req.Prioridade,
		Prazo:         prazo,
		CriadoPor:     criadorUUID,
		ResponsavelID: responsavelUUID,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar tarefa")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]string{
		"id":      database.UUIDToString(tarefa.ID),
		"message": "tarefa criada com sucesso",
	})
}

// Atualizar: PUT /api/tarefas/{id}
func (h *TarefaHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	tID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	tarefa, err := h.db.Queries.BuscarTarefaPorID(r.Context(), tID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}

	criadorStr := database.UUIDToString(tarefa.CriadoPor)
	respStr := ""
	if tarefa.ResponsavelID.Valid {
		respStr = database.UUIDToString(tarefa.ResponsavelID)
	}

	podeEditar := (user.Papel == "admin") || (criadorStr == user.ID) || (respStr == user.ID)
	if !podeEditar {
		response.JSONError(w, http.StatusForbidden, "apenas o criador, o responsável ou um administrador podem editar esta tarefa")
		return
	}

	var req TarefaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Titulo == "" {
		response.JSONError(w, http.StatusBadRequest, "título da tarefa é obrigatório")
		return
	}

	if req.Prioridade != "baixa" && req.Prioridade != "media" && req.Prioridade != "alta" {
		req.Prioridade = "media"
	}

	var prazo pgtype.Timestamptz
	if req.Prazo != nil && *req.Prazo != "" {
		if t, err := time.Parse(time.RFC3339, *req.Prazo); err == nil {
			prazo = database.TimeToTimestamptz(t)
		} else if t, err := time.Parse("2006-01-02", *req.Prazo); err == nil {
			prazo = database.TimeToTimestamptz(t.Add(23*time.Hour + 59*time.Minute))
		}
	}

	var responsavelUUID pgtype.UUID
	if req.ResponsavelID != nil && *req.ResponsavelID != "" {
		responsavelUUID, _ = database.StringToUUID(*req.ResponsavelID)
	}

	_, err = h.db.Queries.AtualizarTarefa(r.Context(), sqlc.AtualizarTarefaParams{
		ID:            tID,
		Titulo:        req.Titulo,
		Descricao:     req.Descricao,
		Prioridade:    req.Prioridade,
		Prazo:         prazo,
		ResponsavelID: responsavelUUID,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar tarefa")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "tarefa atualizada com sucesso"})
}

type AtualizarStatusRequest struct {
	Status string `json:"status"` // "pendente", "em_andamento", "concluida"
}

// AtualizarStatus: PATCH /api/tarefas/{id}/status
func (h *TarefaHandler) AtualizarStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	tID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	tarefa, err := h.db.Queries.BuscarTarefaPorID(r.Context(), tID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}

	criadorStr := database.UUIDToString(tarefa.CriadoPor)
	respStr := ""
	if tarefa.ResponsavelID.Valid {
		respStr = database.UUIDToString(tarefa.ResponsavelID)
	}

	podeEditar := (user.Papel == "admin") || (criadorStr == user.ID) || (respStr == user.ID)
	if !podeEditar {
		response.JSONError(w, http.StatusForbidden, "permissão negada para alterar o status desta tarefa")
		return
	}

	var req AtualizarStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Status != "pendente" && req.Status != "em_andamento" && req.Status != "concluida" {
		response.JSONError(w, http.StatusBadRequest, "status inválido. Use pendente, em_andamento ou concluida")
		return
	}

	atualizada, err := h.db.Queries.AtualizarStatusTarefa(r.Context(), sqlc.AtualizarStatusTarefaParams{
		ID:     tID,
		Status: req.Status,
	})
	if err != nil {
		slog.Error("erro ao atualizar status da tarefa", "erro", err, "id", idStr, "status", req.Status)
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar status da tarefa")
		return
	}

	var concluidaEm *string
	if atualizada.ConcluidaEm.Valid {
		str := atualizada.ConcluidaEm.Time.Format(time.RFC3339)
		concluidaEm = &str
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message":      "status atualizado com sucesso",
		"status":       atualizada.Status,
		"concluida_em": concluidaEm,
	})
}

// Deletar: DELETE /api/tarefas/{id}
func (h *TarefaHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	tID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	tarefa, err := h.db.Queries.BuscarTarefaPorID(r.Context(), tID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}

	criadorStr := database.UUIDToString(tarefa.CriadoPor)
	podeExcluir := (user.Papel == "admin") || (criadorStr == user.ID)
	if !podeExcluir {
		response.JSONError(w, http.StatusForbidden, "apenas o criador ou administrador podem excluir esta tarefa")
		return
	}

	if err := h.db.Queries.DeletarTarefa(r.Context(), tID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir tarefa")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "tarefa excluída com sucesso"})
}

type ComentarioResponse struct {
	ID          string `json:"id"`
	TarefaID    string `json:"tarefa_id"`
	UsuarioID   string `json:"usuario_id"`
	UsuarioNome string `json:"usuario_nome"`
	UsuarioCor  string `json:"usuario_cor"`
	Conteudo    string `json:"conteudo"`
	CriadoEm    string `json:"criado_em"`
	PodeExcluir bool   `json:"pode_excluir"`
}

type CriarComentarioRequest struct {
	Conteudo string `json:"conteudo"`
}

// ListarComentarios: GET /api/tarefas/{id}/comentarios
func (h *TarefaHandler) ListarComentarios(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	tID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	if !h.podeVerTarefa(r, user, tID) {
		response.JSONError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}

	comentarios, err := h.db.Queries.ListarComentariosTarefa(r.Context(), tID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar comentários")
		return
	}

	result := make([]ComentarioResponse, 0, len(comentarios))
	for _, c := range comentarios {
		uID := database.UUIDToString(c.UsuarioID)
		podeExcluir := (user.Papel == "admin") || (uID == user.ID)
		result = append(result, ComentarioResponse{
			ID:          database.UUIDToString(c.ID),
			TarefaID:    database.UUIDToString(c.TarefaID),
			UsuarioID:   uID,
			UsuarioNome: c.UsuarioNome,
			UsuarioCor:  c.UsuarioCor,
			Conteudo:    c.Conteudo,
			CriadoEm:    c.CriadoEm.Time.Format(time.RFC3339),
			PodeExcluir: podeExcluir,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

// CriarComentario: POST /api/tarefas/{id}/comentarios
func (h *TarefaHandler) CriarComentario(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	tID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	if !h.podeVerTarefa(r, user, tID) {
		response.JSONError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}

	var req CriarComentarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Conteudo == "" {
		response.JSONError(w, http.StatusBadRequest, "conteúdo do comentário é obrigatório")
		return
	}

	userUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "usuário inválido")
		return
	}

	comentario, err := h.db.Queries.CriarComentarioTarefa(r.Context(), sqlc.CriarComentarioTarefaParams{
		TarefaID:  tID,
		UsuarioID: userUUID,
		Conteudo:  req.Conteudo,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar comentário")
		return
	}

	response.JSON(w, http.StatusCreated, ComentarioResponse{
		ID:          database.UUIDToString(comentario.ID),
		TarefaID:    database.UUIDToString(comentario.TarefaID),
		UsuarioID:   user.ID,
		UsuarioNome: user.Nome,
		UsuarioCor:  user.Cor,
		Conteudo:    comentario.Conteudo,
		CriadoEm:    comentario.CriadoEm.Time.Format(time.RFC3339),
		PodeExcluir: true,
	})
}

// DeletarComentario: DELETE /api/tarefas/{id}/comentarios/{cid}
func (h *TarefaHandler) DeletarComentario(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	cidStr := chi.URLParam(r, "cid")
	cID, err := database.StringToUUID(cidStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de comentário inválido")
		return
	}

	tID, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de tarefa inválido")
		return
	}

	c, err := h.db.Queries.BuscarComentarioPorID(r.Context(), cID)
	if err != nil || c.TarefaID != tID {
		response.JSONError(w, http.StatusNotFound, "comentário não encontrado")
		return
	}

	uID := database.UUIDToString(c.UsuarioID)
	if user.Papel != "admin" && uID != user.ID {
		response.JSONError(w, http.StatusForbidden, "apenas o autor ou administrador pode excluir este comentário")
		return
	}

	if err := h.db.Queries.DeletarComentarioTarefa(r.Context(), cID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir comentário")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "comentário excluído com sucesso"})
}

// podeVerTarefa: admin vê qualquer tarefa; operador só as que criou ou das quais é responsável
func (h *TarefaHandler) podeVerTarefa(r *http.Request, user *middleware.AuthUser, tID pgtype.UUID) bool {
	t, err := h.db.Queries.BuscarTarefaPorID(r.Context(), tID)
	if err != nil {
		return false
	}
	if user.Papel == "admin" || database.UUIDToString(t.CriadoPor) == user.ID {
		return true
	}
	return t.ResponsavelID.Valid && database.UUIDToString(t.ResponsavelID) == user.ID
}
