package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type PainelHandler struct {
	db *database.DB
}

func NewPainelHandler(db *database.DB) *PainelHandler {
	return &PainelHandler{db: db}
}

// DiasAntecedenciaPainel é quantos dias à frente o painel avisa das marcações.
const DiasAntecedenciaPainel = 7

type PainelResponse struct {
	ProximosEventos  []EventoResponse     `json:"proximos_eventos"`
	TarefasPendentes []TarefaItemResponse `json:"tarefas_pendentes"`
}

// ObterDadosPainel: GET /api/painel (única chamada para o painel inicial)
func (h *PainelHandler) ObterDadosPainel(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	userUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar usuário")
		return
	}

	now := time.Now()
	ano, mes, dia := now.Date()
	inicioHoje := time.Date(ano, mes, dia, 0, 0, 0, 0, now.Location())
	// Antecedência de uma semana: hoje + os próximos 7 dias
	fimJanela := inicioHoje.AddDate(0, 0, DiasAntecedenciaPainel+1).Add(-time.Nanosecond)

	// 1. Marcações do usuário na próxima semana (recorrências desdobradas)
	eventos, err := h.db.Queries.ListarEventosIntervalo(r.Context(), sqlc.ListarEventosIntervaloParams{
		Fim:    database.TimeToTimestamptz(inicioHoje),
		Inicio: database.TimeToTimestamptz(fimJanela),
	})
	if err != nil {
		eventos = []sqlc.ListarEventosIntervaloRow{}
	}

	eventoIDs := make([]pgtype.UUID, 0, len(eventos))
	for _, e := range eventos {
		eventoIDs = append(eventoIDs, e.ID)
	}

	participantes, _ := h.db.Queries.ListarParticipantesPorEventos(r.Context(), eventoIDs)
	participantesPorEvento := make(map[string][]ParticipanteResponse)
	for _, p := range participantes {
		eID := database.UUIDToString(p.EventoID)
		participantesPorEvento[eID] = append(participantesPorEvento[eID], ParticipanteResponse{
			ID:   database.UUIDToString(p.UsuarioID),
			Nome: p.Nome,
			Cor:  p.Cor,
		})
	}

	proximosEventos := make([]EventoResponse, 0)
	for _, e := range eventos {
		parts := participantesPorEvento[database.UUIDToString(e.ID)]

		// Inclui apenas se o usuário for participante
		souParticipante := false
		for _, p := range parts {
			if p.ID == user.ID {
				souParticipante = true
				break
			}
		}
		if !souParticipante {
			continue
		}

		podeEditar := (user.Papel == "admin") || (database.UUIDToString(e.CriadoPor) == user.ID)
		proximosEventos = append(proximosEventos, expandirOcorrencias(e, parts, podeEditar, inicioHoje, fimJanela)...)
	}
	sort.SliceStable(proximosEventos, func(i, j int) bool {
		return proximosEventos[i].Inicio < proximosEventos[j].Inicio
	})

	// 2. Tarefas pendentes atribuídas ao usuário (atrasadas primeiro)
	tarefas, err := h.db.Queries.ListarTarefasPendentesUsuario(r.Context(), userUUID)
	if err != nil {
		tarefas = []sqlc.ListarTarefasPendentesUsuarioRow{}
	}

	tarefasPendentes := make([]TarefaItemResponse, 0, len(tarefas))
	for _, t := range tarefas {
		var prazoStr *string
		atrasada := false
		if t.Prazo.Valid {
			str := t.Prazo.Time.Format(time.RFC3339)
			prazoStr = &str
			if t.Prazo.Time.Before(now) {
				atrasada = true
			}
		}

		tarefasPendentes = append(tarefasPendentes, TarefaItemResponse{
			ID:              database.UUIDToString(t.ID),
			Titulo:          t.Titulo,
			Descricao:       t.Descricao,
			Status:          t.Status,
			Prioridade:      t.Prioridade,
			Prazo:           prazoStr,
			CriadoPor:       database.UUIDToString(t.CriadoPor),
			CriadorNome:     t.CriadorNome,
			CriadorCor:      t.CriadorCor,
			ResponsavelID:   &user.ID,
			ResponsavelNome: &user.Nome,
			ResponsavelCor:  &user.Cor,
			CriadoEm:        t.CriadoEm.Time.Format(time.RFC3339),
			AtualizadoEm:    t.AtualizadoEm.Time.Format(time.RFC3339),
			Atrasada:        atrasada,
			PodeEditar:      true,
			PodeExcluir:     (user.Papel == "admin") || (database.UUIDToString(t.CriadoPor) == user.ID),
		})
	}

	response.JSON(w, http.StatusOK, PainelResponse{
		ProximosEventos:  proximosEventos,
		TarefasPendentes: tarefasPendentes,
	})
}
