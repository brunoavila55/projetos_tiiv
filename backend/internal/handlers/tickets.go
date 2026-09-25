package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// Limite de abertura de tickets sem login, por IP
const (
	ticketsPorJanela = 5
	janelaTickets    = 10 * time.Minute
)

type TicketHandler struct {
	db      *database.DB
	limites *limitadorPorIP
}

func NewTicketHandler(db *database.DB) *TicketHandler {
	return &TicketHandler{db: db, limites: novoLimitadorPorIP(ticketsPorJanela, janelaTickets)}
}

type CriarTicketRequest struct {
	SolicitanteNome string `json:"solicitante_nome"`
	Titulo          string `json:"titulo"`
	Descricao       string `json:"descricao"`
	Prioridade      string `json:"prioridade"`
	// Setor escolhido na tela de acesso; vazio vale quando só um aceita pedidos
	SetorID string `json:"setor_id"`
}

func validarTexto(valor, campo string, obrigatorio bool, max int) (string, error) {
	valor = strings.TrimSpace(valor)
	if obrigatorio && valor == "" {
		return "", fmt.Errorf("%s é obrigatório", campo)
	}
	if utf8.RuneCountInString(valor) > max {
		return "", fmt.Errorf("%s deve ter no máximo %d caracteres", campo, max)
	}
	return valor, nil
}

// CriarPublico: POST /api/tickets/publico (sem login)
func (h *TicketHandler) CriarPublico(w http.ResponseWriter, r *http.Request) {
	var req CriarTicketRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	var err error
	campos := []struct {
		valor       *string
		nome        string
		obrigatorio bool
		max         int
	}{
		{&req.SolicitanteNome, "seu nome", true, 120},
		{&req.Titulo, "assunto", true, 200},
		{&req.Descricao, "descrição", true, 4000},
	}
	for _, c := range campos {
		if *c.valor, err = validarTexto(*c.valor, c.nome, c.obrigatorio, c.max); err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.Prioridade == "" {
		req.Prioridade = "media"
	}
	if req.Prioridade != "baixa" && req.Prioridade != "media" && req.Prioridade != "alta" {
		response.JSONError(w, http.StatusBadRequest, "urgência inválida")
		return
	}

	setor, err := setorDoPedido(r.Context(), h.db.Queries, req.SetorID)
	if errors.Is(err, errSetorPedido) {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar o ticket")
		return
	}

	ip := ipDaRequisicao(r)
	if !h.limites.permitir(ip) {
		response.JSONError(w, http.StatusTooManyRequests, "muitos tickets enviados deste computador; aguarde alguns minutos")
		return
	}

	tk, err := h.db.Queries.CriarTicket(r.Context(), sqlc.CriarTicketParams{
		SolicitanteNome: req.SolicitanteNome,
		Titulo:          req.Titulo,
		Descricao:       req.Descricao,
		Prioridade:      req.Prioridade,
		OrigemIp:        ip,
		SetorID:         setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar o ticket")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"id":     database.UUIDToString(tk.ID),
		"numero": tk.Numero,
	})
}

type TicketResponse struct {
	ID              string  `json:"id"`
	Numero          int64   `json:"numero"`
	SolicitanteNome string  `json:"solicitante_nome"`
	Titulo          string  `json:"titulo"`
	Descricao       string  `json:"descricao"`
	Prioridade      string  `json:"prioridade"`
	Status          string  `json:"status"`
	TarefaID        *string `json:"tarefa_id"`
	TarefaStatus    *string `json:"tarefa_status"`
	TratadoPorNome  *string `json:"tratado_por_nome"`
	TratadoEm       *string `json:"tratado_em"`
	CriadoEm        string  `json:"criado_em"`
}

// Listar: GET /api/tickets?status=aberto|resgatado|descartado
func (h *TicketHandler) Listar(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "aberto" && status != "resgatado" && status != "descartado" {
		response.JSONError(w, http.StatusBadRequest, "status inválido")
		return
	}

	rows, err := h.db.Queries.ListarTickets(r.Context(), sqlc.ListarTicketsParams{
		SetorID: setorDe(r),
		Status:  database.StringToText(status),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar tickets")
		return
	}

	result := make([]TicketResponse, 0, len(rows))
	for _, t := range rows {
		var tarefaID *string
		if t.TarefaID.Valid {
			s := database.UUIDToString(t.TarefaID)
			tarefaID = &s
		}
		result = append(result, TicketResponse{
			ID:              database.UUIDToString(t.ID),
			Numero:          t.Numero,
			SolicitanteNome: t.SolicitanteNome,
			Titulo:          t.Titulo,
			Descricao:       t.Descricao,
			Prioridade:      t.Prioridade,
			Status:          t.Status,
			TarefaID:        tarefaID,
			TarefaStatus:    database.TextToString(t.TarefaStatus),
			TratadoPorNome:  database.TextToString(t.TratadoPorNome),
			TratadoEm:       formatarTimestamptz(t.TratadoEm),
			CriadoEm:        t.CriadoEm.Time.Format(time.RFC3339),
		})
	}

	response.JSON(w, http.StatusOK, result)
}

// Resumo: GET /api/tickets/resumo — contador para o menu
func (h *TicketHandler) Resumo(w http.ResponseWriter, r *http.Request) {
	abertos, err := h.db.Queries.ContarTicketsAbertos(r.Context(), setorDe(r))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao contar tickets")
		return
	}
	response.JSON(w, http.StatusOK, map[string]int64{"abertos": abertos})
}

var errTicketJaTratado = errors.New("ticket já tratado")

// tratar bloqueia o ticket, confere que ainda está aberto e aplica a ação na
// mesma transação, para que dois operadores não resgatem o mesmo ticket.
func (h *TicketHandler) tratar(r *http.Request, id pgtype.UUID, acao func(qtx *sqlc.Queries, tk sqlc.Tickets) error) error {
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	qtx := h.db.Queries.WithTx(tx)

	tk, err := qtx.BloquearTicket(r.Context(), sqlc.BloquearTicketParams{ID: id, SetorID: setorDe(r)})
	if err != nil {
		return err
	}
	if tk.Status != "aberto" {
		return errTicketJaTratado
	}
	if err := acao(qtx, tk); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func responderErroTratamento(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		response.JSONError(w, http.StatusNotFound, "ticket não encontrado")
	case errors.Is(err, errTicketJaTratado):
		response.JSONError(w, http.StatusConflict, "este ticket já foi resgatado ou descartado")
	default:
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar o ticket")
	}
}

// Resgatar: POST /api/tickets/{id}/resgatar — vira tarefa do operador logado
func (h *TicketHandler) Resgatar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	uID, _ := database.StringToUUID(user.ID)

	var tarefaID pgtype.UUID
	var numero int64
	err = h.tratar(r, id, func(qtx *sqlc.Queries, tk sqlc.Tickets) error {
		descricao := fmt.Sprintf("Ticket #%d aberto por %s em %s.\n\n%s",
			tk.Numero, tk.SolicitanteNome, tk.CriadoEm.Time.In(fusoSaoPaulo).Format("02/01/2006 15:04"), tk.Descricao)

		tarefa, err := qtx.CriarTarefa(r.Context(), sqlc.CriarTarefaParams{
			Titulo:        fmt.Sprintf("Ticket #%d: %s", tk.Numero, tk.Titulo),
			Descricao:     descricao,
			Prioridade:    tk.Prioridade,
			CriadoPor:     uID,
			ResponsavelID: uID,
			SetorID:       tk.SetorID,
		})
		if err != nil {
			return err
		}
		_, err = qtx.MarcarTicketTratado(r.Context(), sqlc.MarcarTicketTratadoParams{
			ID:         tk.ID,
			Status:     "resgatado",
			TarefaID:   tarefa.ID,
			TratadoPor: uID,
		})
		tarefaID, numero = tarefa.ID, tk.Numero
		return err
	})
	if err != nil {
		responderErroTratamento(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tarefa_id": database.UUIDToString(tarefaID),
		"numero":    numero,
	})
}

// Descartar: POST /api/tickets/{id}/descartar (Admin) — spam ou duplicado
func (h *TicketHandler) Descartar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	uID, _ := database.StringToUUID(user.ID)

	err = h.tratar(r, id, func(qtx *sqlc.Queries, tk sqlc.Tickets) error {
		_, err := qtx.MarcarTicketTratado(r.Context(), sqlc.MarcarTicketTratadoParams{
			ID:         tk.ID,
			Status:     "descartado",
			TratadoPor: uID,
		})
		return err
	})
	if err != nil {
		responderErroTratamento(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
