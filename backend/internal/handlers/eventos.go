package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type EventoHandler struct {
	db *database.DB
}

func NewEventoHandler(db *database.DB) *EventoHandler {
	return &EventoHandler{db: db}
}

type ParticipanteResponse struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	Cor  string `json:"cor"`
}

type EventoResponse struct {
	ID            string                 `json:"id"`
	Titulo        string                 `json:"titulo"`
	Descricao     string                 `json:"descricao"`
	Inicio        string                 `json:"inicio"`
	Fim           string                 `json:"fim"`
	DiaInteiro    bool                   `json:"dia_inteiro"`
	CriadoPor     string                 `json:"criado_por"`
	CriadorNome   string                 `json:"criador_nome"`
	CriadorCor    string                 `json:"criador_cor"`
	Participantes []ParticipanteResponse `json:"participantes"`
	PodeEditar    bool                   `json:"pode_editar"`
}

// Listar: GET /api/eventos?inicio=&fim=&meus=
func (h *EventoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	inicioStr := r.URL.Query().Get("inicio")
	fimStr := r.URL.Query().Get("fim")
	apenasMeus := r.URL.Query().Get("meus") == "true"

	var inicio, fim time.Time
	var err error

	if inicioStr != "" {
		inicio, err = time.Parse(time.RFC3339, inicioStr)
		if err != nil {
			inicio, err = time.Parse("2006-01-02", inicioStr)
		}
	}
	if inicio.IsZero() {
		inicio = time.Now().AddDate(0, -1, 0)
	}

	if fimStr != "" {
		fim, err = time.Parse(time.RFC3339, fimStr)
		if err != nil {
			fim, err = time.Parse("2006-01-02", fimStr)
		}
	}
	if fim.IsZero() {
		fim = time.Now().AddDate(0, 2, 0)
	}

	eventos, err := h.db.Queries.ListarEventosIntervalo(r.Context(), sqlc.ListarEventosIntervaloParams{
		Fim:    database.TimeToTimestamptz(inicio),
		Inicio: database.TimeToTimestamptz(fim),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar eventos")
		return
	}

	if len(eventos) == 0 {
		response.JSON(w, http.StatusOK, []EventoResponse{})
		return
	}

	// Carregar todos os participantes dos eventos retornados
	eventoIDs := make([]pgtype.UUID, 0, len(eventos))
	for _, e := range eventos {
		eventoIDs = append(eventoIDs, e.ID)
	}

	participantes, err := h.db.Queries.ListarParticipantesPorEventos(r.Context(), eventoIDs)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao consultar participantes")
		return
	}

	participantesPorEvento := make(map[string][]ParticipanteResponse)
	for _, p := range participantes {
		eID := database.UUIDToString(p.EventoID)
		participantesPorEvento[eID] = append(participantesPorEvento[eID], ParticipanteResponse{
			ID:   database.UUIDToString(p.UsuarioID),
			Nome: p.Nome,
			Cor:  p.Cor,
		})
	}

	result := make([]EventoResponse, 0, len(eventos))
	for _, e := range eventos {
		eID := database.UUIDToString(e.ID)
		eCriadoPor := database.UUIDToString(e.CriadoPor)
		partList := participantesPorEvento[eID]

		// Se o filtro for apenas "Meus eventos", verificar se o usuário é participante
		if apenasMeus {
			souParticipante := false
			for _, p := range partList {
				if p.ID == user.ID {
					souParticipante = true
					break
				}
			}
			if !souParticipante {
				continue
			}
		}

		podeEditar := (user.Papel == "admin") || (eCriadoPor == user.ID)

		result = append(result, EventoResponse{
			ID:            eID,
			Titulo:        e.Titulo,
			Descricao:     e.Descricao,
			Inicio:        e.Inicio.Time.Format(time.RFC3339),
			Fim:           e.Fim.Time.Format(time.RFC3339),
			DiaInteiro:    e.DiaInteiro,
			CriadoPor:     eCriadoPor,
			CriadorNome:   e.CriadorNome,
			CriadorCor:    e.CriadorCor,
			Participantes: partList,
			PodeEditar:    podeEditar,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type EventoRequest struct {
	Titulo        string   `json:"titulo"`
	Descricao     string   `json:"descricao"`
	Inicio        string   `json:"inicio"`
	Fim           string   `json:"fim"`
	DiaInteiro    bool     `json:"dia_inteiro"`
	Participantes []string `json:"participantes"`
}

// Criar: POST /api/eventos
func (h *EventoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req EventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Titulo == "" || req.Inicio == "" || req.Fim == "" {
		response.JSONError(w, http.StatusBadRequest, "título, início e fim são obrigatórios")
		return
	}

	inicio, err := time.Parse(time.RFC3339, req.Inicio)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "formato inválido para início")
		return
	}

	fim, err := time.Parse(time.RFC3339, req.Fim)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "formato inválido para fim")
		return
	}

	if fim.Before(inicio) {
		response.JSONError(w, http.StatusBadRequest, "o horário de término não pode ser anterior ao horário de início")
		return
	}

	criadorUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar criador")
		return
	}

	evento, err := h.db.Queries.CriarEvento(r.Context(), sqlc.CriarEventoParams{
		Titulo:     req.Titulo,
		Descricao:  req.Descricao,
		Inicio:     database.TimeToTimestamptz(inicio),
		Fim:        database.TimeToTimestamptz(fim),
		DiaInteiro: req.DiaInteiro,
		CriadoPor:  criadorUUID,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar evento")
		return
	}

	// Adicionar o criador como participante obrigatoriamente
	_ = h.db.Queries.AdicionarParticipanteEvento(r.Context(), sqlc.AdicionarParticipanteEventoParams{
		EventoID:  evento.ID,
		UsuarioID: criadorUUID,
	})

	// Adicionar demais participantes selecionados
	for _, pIDStr := range req.Participantes {
		if pIDStr == user.ID {
			continue // Já adicionado
		}
		pUUID, err := database.StringToUUID(pIDStr)
		if err == nil {
			_ = h.db.Queries.AdicionarParticipanteEvento(r.Context(), sqlc.AdicionarParticipanteEventoParams{
				EventoID:  evento.ID,
				UsuarioID: pUUID,
			})
		}
	}

	response.JSON(w, http.StatusCreated, map[string]string{
		"id":      database.UUIDToString(evento.ID),
		"message": "evento criado com sucesso",
	})
}

// Atualizar: PUT /api/eventos/{id}
func (h *EventoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	eID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	evento, err := h.db.Queries.BuscarEventoPorID(r.Context(), eID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "evento não encontrado")
		return
	}

	criadorStr := database.UUIDToString(evento.CriadoPor)
	if user.Papel != "admin" && criadorStr != user.ID {
		response.JSONError(w, http.StatusForbidden, "apenas o criador do evento ou administrador pode alterá-lo")
		return
	}

	var req EventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	inicio, err := time.Parse(time.RFC3339, req.Inicio)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "formato inválido para início")
		return
	}

	fim, err := time.Parse(time.RFC3339, req.Fim)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "formato inválido para fim")
		return
	}

	if fim.Before(inicio) {
		response.JSONError(w, http.StatusBadRequest, "o horário de término não pode ser anterior ao horário de início")
		return
	}

	_, err = h.db.Queries.AtualizarEvento(r.Context(), sqlc.AtualizarEventoParams{
		ID:         eID,
		Titulo:     req.Titulo,
		Descricao:  req.Descricao,
		Inicio:     database.TimeToTimestamptz(inicio),
		Fim:        database.TimeToTimestamptz(fim),
		DiaInteiro: req.DiaInteiro,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar evento")
		return
	}

	// Se forneceu lista de participantes, atualizar
	if req.Participantes != nil {
		_ = h.db.Queries.RemoverParticipantesEvento(r.Context(), eID)
		// Criador permanece participante
		_ = h.db.Queries.AdicionarParticipanteEvento(r.Context(), sqlc.AdicionarParticipanteEventoParams{
			EventoID:  eID,
			UsuarioID: evento.CriadoPor,
		})

		for _, pIDStr := range req.Participantes {
			if pIDStr == criadorStr {
				continue
			}
			pUUID, err := database.StringToUUID(pIDStr)
			if err == nil {
				_ = h.db.Queries.AdicionarParticipanteEvento(r.Context(), sqlc.AdicionarParticipanteEventoParams{
					EventoID:  eID,
					UsuarioID: pUUID,
				})
			}
		}
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "evento atualizado com sucesso"})
}

// Deletar: DELETE /api/eventos/{id}
func (h *EventoHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	idStr := chi.URLParam(r, "id")
	eID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	evento, err := h.db.Queries.BuscarEventoPorID(r.Context(), eID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "evento não encontrado")
		return
	}

	criadorStr := database.UUIDToString(evento.CriadoPor)
	if user.Papel != "admin" && criadorStr != user.ID {
		response.JSONError(w, http.StatusForbidden, "apenas o criador do evento ou administrador pode excluí-lo")
		return
	}

	if err := h.db.Queries.DeletarEvento(r.Context(), eID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir evento")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "evento excluído com sucesso"})
}
