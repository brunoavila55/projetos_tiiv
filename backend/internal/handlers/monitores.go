package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/monitor"
	"tiiv/backend/internal/response"
)

// Monitor de disponibilidade: toda a equipe vê o estado e pede verificação na
// hora; só admin cadastra, edita e exclui (o servidor passa a acessar o alvo).
type MonitorHandler struct {
	db          *database.DB
	verificador *monitor.Verificador
}

func NewMonitorHandler(db *database.DB) *MonitorHandler {
	return &MonitorHandler{db: db, verificador: monitor.NovoVerificador(db)}
}

type MonitorRequest struct {
	Nome         string `json:"nome"`
	Tipo         string `json:"tipo"`
	Alvo         string `json:"alvo"`
	IntervaloSeg int32  `json:"intervalo_seg"`
	AbrirTicket  bool   `json:"abrir_ticket"`
	Ativo        *bool  `json:"ativo"`
	// Fila que recebe o ticket de queda; vazio = setor de quem cadastra
	SetorTicketID string `json:"setor_ticket_id"`
}

type MonitorResponse struct {
	ID            string  `json:"id"`
	Nome          string  `json:"nome"`
	Tipo          string  `json:"tipo"`
	Alvo          string  `json:"alvo"`
	IntervaloSeg  int32   `json:"intervalo_seg"`
	AbrirTicket   bool    `json:"abrir_ticket"`
	SetorTicketID string  `json:"setor_ticket_id"`
	Ativo         bool    `json:"ativo"`
	Status        string  `json:"status"`
	LatenciaMs    *int32  `json:"latencia_ms"`
	UltimoErro    string  `json:"ultimo_erro"`
	VerificadoEm  *string `json:"verificado_em"`
	StatusDesde   string  `json:"status_desde"`
	// Percentual do tempo no ar nas últimas 24 h (ou desde o cadastro)
	Disponibilidade24h *float64 `json:"disponibilidade_24h"`
}

type QuedaResponse struct {
	ID           string  `json:"id"`
	Inicio       string  `json:"inicio"`
	Fim          *string `json:"fim"`
	Erro         string  `json:"erro"`
	TicketNumero *int64  `json:"ticket_numero"`
}

type MonitorForaResponse struct {
	ID          string `json:"id"`
	Nome        string `json:"nome"`
	Alvo        string `json:"alvo"`
	UltimoErro  string `json:"ultimo_erro"`
	StatusDesde string `json:"status_desde"`
}

// ResumoMonitoresResponse também vai no GET /api/painel
type ResumoMonitoresResponse struct {
	Ativos  int64                 `json:"ativos"`
	Online  int64                 `json:"online"`
	Offline int64                 `json:"offline"`
	Fora    []MonitorForaResponse `json:"fora"`
}

func paraMonitorResponse(m sqlc.Monitores, segundosFora float64) MonitorResponse {
	res := MonitorResponse{
		ID:            database.UUIDToString(m.ID),
		Nome:          m.Nome,
		Tipo:          m.Tipo,
		Alvo:          m.Alvo,
		IntervaloSeg:  m.IntervaloSeg,
		AbrirTicket:   m.AbrirTicket,
		SetorTicketID: database.UUIDToString(m.SetorTicketID),
		Ativo:         m.Ativo,
		Status:        m.Status,
		UltimoErro:    m.UltimoErro,
		VerificadoEm:  formatarTimestamptz(m.VerificadoEm),
		StatusDesde:   m.StatusDesde.Time.Format(time.RFC3339),
	}
	if m.LatenciaMs.Valid {
		res.LatenciaMs = &m.LatenciaMs.Int32
	}
	// Janela de 24 h, ou desde o cadastro se o monitor é mais novo
	if m.Ativo && m.VerificadoEm.Valid {
		janela := min(24*time.Hour, time.Since(m.CriadoEm.Time)).Seconds()
		if janela >= 60 {
			pct := math.Round(1000*100*(1-segundosFora/janela)) / 1000
			pct = math.Max(0, math.Min(100, pct))
			res.Disponibilidade24h = &pct
		}
	}
	return res
}

func resumoMonitores(r *http.Request, q *sqlc.Queries) (ResumoMonitoresResponse, error) {
	contagem, err := q.ResumoMonitores(r.Context())
	if err != nil {
		return ResumoMonitoresResponse{}, err
	}
	fora, err := q.ListarMonitoresOffline(r.Context())
	if err != nil {
		return ResumoMonitoresResponse{}, err
	}
	res := ResumoMonitoresResponse{
		Ativos:  contagem.Ativos,
		Online:  contagem.Online,
		Offline: contagem.Offline,
		Fora:    make([]MonitorForaResponse, 0, len(fora)),
	}
	for _, m := range fora {
		res.Fora = append(res.Fora, MonitorForaResponse{
			ID:          database.UUIDToString(m.ID),
			Nome:        m.Nome,
			Alvo:        m.Alvo,
			UltimoErro:  m.UltimoErro,
			StatusDesde: m.StatusDesde.Time.Format(time.RFC3339),
		})
	}
	return res, nil
}

func validarMonitor(req *MonitorRequest) error {
	var err error
	if req.Nome, err = validarTexto(req.Nome, "nome", true, 120); err != nil {
		return err
	}
	if req.Alvo, err = monitor.ValidarAlvo(req.Tipo, req.Alvo); err != nil {
		return err
	}
	if req.IntervaloSeg == 0 {
		req.IntervaloSeg = 60
	}
	if req.IntervaloSeg < 30 || req.IntervaloSeg > 3600 {
		return errors.New("o intervalo deve ficar entre 30 segundos e 1 hora")
	}
	return nil
}

func lerMonitorRequest(w http.ResponseWriter, r *http.Request) (MonitorRequest, bool) {
	var req MonitorRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return req, false
	}
	if err := validarMonitor(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return req, false
	}
	return req, true
}

// setorDoTicket resolve a fila dos tickets de queda (padrao quando não vem)
func (h *MonitorHandler) setorDoTicket(w http.ResponseWriter, r *http.Request, pedido string, padrao pgtype.UUID) (pgtype.UUID, bool) {
	if pedido == "" {
		return padrao, true
	}
	sID, err := database.StringToUUID(pedido)
	if err == nil {
		_, err = h.db.Queries.BuscarSetor(r.Context(), sID)
	}
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "setor dos tickets não encontrado")
		return pgtype.UUID{}, false
	}
	return sID, true
}

func (h *MonitorHandler) obterPorURL(w http.ResponseWriter, r *http.Request) (sqlc.Monitores, bool) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return sqlc.Monitores{}, false
	}
	m, err := h.db.Queries.ObterMonitor(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "monitor não encontrado")
		return m, false
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar monitor")
		return m, false
	}
	return m, true
}

func (h *MonitorHandler) segundosFora(r *http.Request) map[pgtype.UUID]float64 {
	fora := make(map[pgtype.UUID]float64)
	rows, err := h.db.Queries.SegundosForaUltimas24h(r.Context())
	if err != nil {
		return fora
	}
	for _, row := range rows {
		fora[row.MonitorID] = row.Segundos
	}
	return fora
}

// Listar: GET /api/monitores
func (h *MonitorHandler) Listar(w http.ResponseWriter, r *http.Request) {
	monitores, err := h.db.Queries.ListarMonitores(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar monitores")
		return
	}
	fora := h.segundosFora(r)
	result := make([]MonitorResponse, 0, len(monitores))
	for _, m := range monitores {
		result = append(result, paraMonitorResponse(m, fora[m.ID]))
	}
	response.JSON(w, http.StatusOK, result)
}

// Resumo: GET /api/monitores/resumo (contador do menu)
func (h *MonitorHandler) Resumo(w http.ResponseWriter, r *http.Request) {
	res, err := resumoMonitores(r, h.db.Queries)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao resumir monitores")
		return
	}
	response.JSON(w, http.StatusOK, res)
}

// Criar: POST /api/monitores (admin)
func (h *MonitorHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	req, ok := lerMonitorRequest(w, r)
	if !ok {
		return
	}
	setorTicket, ok := h.setorDoTicket(w, r, req.SetorTicketID, user.Setor)
	if !ok {
		return
	}
	uID, _ := database.StringToUUID(user.ID)
	m, err := h.db.Queries.CriarMonitor(r.Context(), sqlc.CriarMonitorParams{
		Nome:          req.Nome,
		Tipo:          req.Tipo,
		Alvo:          req.Alvo,
		IntervaloSeg:  req.IntervaloSeg,
		AbrirTicket:   req.AbrirTicket,
		CriadoPor:     uID,
		SetorTicketID: setorTicket,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar monitor")
		return
	}
	response.JSON(w, http.StatusCreated, paraMonitorResponse(m, 0))
}

// Atualizar: PUT /api/monitores/{id} (admin). Trocar tipo/alvo ou desligar
// zera o estado e fecha a queda aberta.
func (h *MonitorHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	antigo, ok := h.obterPorURL(w, r)
	if !ok {
		return
	}
	req, ok := lerMonitorRequest(w, r)
	if !ok {
		return
	}
	ativo := antigo.Ativo
	if req.Ativo != nil {
		ativo = *req.Ativo
	}
	setorTicket, ok := h.setorDoTicket(w, r, req.SetorTicketID, antigo.SetorTicketID)
	if !ok {
		return
	}

	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar monitor")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	q := h.db.Queries.WithTx(tx)

	if _, err := q.AtualizarMonitor(r.Context(), sqlc.AtualizarMonitorParams{
		ID:            antigo.ID,
		Nome:          req.Nome,
		Tipo:          req.Tipo,
		Alvo:          req.Alvo,
		IntervaloSeg:  req.IntervaloSeg,
		AbrirTicket:   req.AbrirTicket,
		Ativo:         ativo,
		SetorTicketID: setorTicket,
	}); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar monitor")
		return
	}
	if antigo.Tipo != req.Tipo || antigo.Alvo != req.Alvo || antigo.Ativo != ativo {
		if err := q.FecharQuedaAberta(r.Context(), antigo.ID); err != nil {
			response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar monitor")
			return
		}
		if err := q.ReiniciarEstadoMonitor(r.Context(), antigo.ID); err != nil {
			response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar monitor")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar monitor")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Deletar: DELETE /api/monitores/{id} (admin; o histórico de quedas vai junto)
func (h *MonitorHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	m, ok := h.obterPorURL(w, r)
	if !ok {
		return
	}
	if err := h.db.Queries.DeletarMonitor(r.Context(), m.ID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir monitor")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Verificar: POST /api/monitores/{id}/verificar — checa agora e devolve o estado
func (h *MonitorHandler) Verificar(w http.ResponseWriter, r *http.Request) {
	m, ok := h.obterPorURL(w, r)
	if !ok {
		return
	}
	if !m.Ativo {
		response.JSONError(w, http.StatusConflict, "o monitor está desligado")
		return
	}
	atual, err := h.verificador.VerificarEGravar(r.Context(), m)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao verificar monitor")
		return
	}
	response.JSON(w, http.StatusOK, paraMonitorResponse(atual, h.segundosFora(r)[atual.ID]))
}

// ListarQuedas: GET /api/monitores/{id}/quedas (últimas 20)
func (h *MonitorHandler) ListarQuedas(w http.ResponseWriter, r *http.Request) {
	m, ok := h.obterPorURL(w, r)
	if !ok {
		return
	}
	rows, err := h.db.Queries.ListarQuedasMonitor(r.Context(), m.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar quedas")
		return
	}
	result := make([]QuedaResponse, 0, len(rows))
	for _, q := range rows {
		item := QuedaResponse{
			ID:     database.UUIDToString(q.ID),
			Inicio: q.Inicio.Time.Format(time.RFC3339),
			Fim:    formatarTimestamptz(q.Fim),
			Erro:   q.Erro,
		}
		if q.TicketNumero.Valid {
			item.TicketNumero = &q.TicketNumero.Int64
		}
		result = append(result, item)
	}
	response.JSON(w, http.StatusOK, result)
}
