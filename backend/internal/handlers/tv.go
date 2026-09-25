package handlers

import (
	"crypto/rand"
	"encoding/hex"
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

// CabecalhoChaveTV leva a chave da tela; vai em cabeçalho (e não na URL) para
// não aparecer em logs nem no histórico do navegador.
const CabecalhoChaveTV = "X-TV-Chave"

// Chaves erradas por IP antes de recusar por um tempo
const (
	falhasChaveTVPorJanela = 20
	janelaChaveTV          = 10 * time.Minute
)

// Quantos tickets abertos a TV lista (o total vem à parte)
const ticketsNaTV = 8

// Modo TV: painel só de leitura para o telão do NOC. Abre com a chave de uma
// tela cadastrada pelo admin ou com a sessão de um operador.
type TVHandler struct {
	db      *database.DB
	limites *limitadorPorIP
}

func NewTVHandler(db *database.DB) *TVHandler {
	return &TVHandler{db: db, limites: novoLimitadorPorIP(falhasChaveTVPorJanela, janelaChaveTV)}
}

type TelaTVResponse struct {
	ID             string  `json:"id"`
	Nome           string  `json:"nome"`
	CriadorNome    string  `json:"criador_nome"`
	CriadoEm       string  `json:"criado_em"`
	UltimoAcessoEm *string `json:"ultimo_acesso_em"`
	UltimoIP       string  `json:"ultimo_ip"`
	// Só na criação: a chave não fica guardada, apenas o hash
	Chave string `json:"chave,omitempty"`
}

type TVMonitorResponse struct {
	ID                 string   `json:"id"`
	Nome               string   `json:"nome"`
	Tipo               string   `json:"tipo"`
	Status             string   `json:"status"`
	LatenciaMs         *int32   `json:"latencia_ms"`
	UltimoErro         string   `json:"ultimo_erro"`
	StatusDesde        string   `json:"status_desde"`
	Disponibilidade24h *float64 `json:"disponibilidade_24h"`
}

type TVTicketResponse struct {
	Numero          int64  `json:"numero"`
	Titulo          string `json:"titulo"`
	SolicitanteNome string `json:"solicitante_nome"`
	Prioridade      string `json:"prioridade"`
	CriadoEm        string `json:"criado_em"`
}

type TVAvisoResponse struct {
	ID          string `json:"id"`
	Titulo      string `json:"titulo"`
	Mensagem    string `json:"mensagem"`
	Nivel       string `json:"nivel"`
	CriadorNome string `json:"criador_nome"`
}

type TVPainelResponse struct {
	Tela          string              `json:"tela"`
	Setor         string              `json:"setor"`
	AgoraServidor string              `json:"agora_servidor"`
	Monitores     []TVMonitorResponse `json:"monitores"`
	TicketsTotal  int                 `json:"tickets_total"`
	Tickets       []TVTicketResponse  `json:"tickets"`
	PlantaoHoje   []PlantaoResponse   `json:"plantao_hoje"`
	Avisos        []TVAvisoResponse   `json:"avisos"`
}

// painelAutorizado: nome exibido no rodapé da TV e o setor cujos tickets,
// plantão e avisos ela mostra
type painelAutorizado struct {
	nome      string
	setor     pgtype.UUID
	setorNome string
}

// autorizarPainel aceita a chave de uma tela (setor da tela) ou uma sessão
// válida (setor de trabalho do operador)
func (h *TVHandler) autorizarPainel(w http.ResponseWriter, r *http.Request) (painelAutorizado, bool) {
	chave := r.Header.Get(CabecalhoChaveTV)
	if chave == "" {
		if user, ok := middleware.GetAuthUser(r.Context()); ok && user != nil && !user.DeveTrocarPin {
			return painelAutorizado{user.Nome, user.Setor, user.SetorNome}, true
		}
		response.JSONError(w, http.StatusUnauthorized, "tela não autorizada")
		return painelAutorizado{}, false
	}

	ip := ipDaRequisicao(r)
	if !h.limites.permitir(ip) {
		response.JSONError(w, http.StatusTooManyRequests, "muitas chaves inválidas deste computador; aguarde alguns minutos")
		return painelAutorizado{}, false
	}
	tela, err := h.db.Queries.BuscarTelaTVPorHash(r.Context(), middleware.HashToken(chave))
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusUnauthorized, "chave da tela inválida ou revogada")
		return painelAutorizado{}, false
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao validar a tela")
		return painelAutorizado{}, false
	}
	// Chave certa não conta como falha
	h.limites.liberar(ip)

	_ = h.db.Queries.RegistrarAcessoTelaTV(r.Context(), sqlc.RegistrarAcessoTelaTVParams{ID: tela.ID, UltimoIp: ip})
	return painelAutorizado{tela.Nome, tela.SetorID, tela.SetorNome}, true
}

// Painel: GET /api/tv/painel (chave da tela no cabeçalho X-TV-Chave, ou sessão)
func (h *TVHandler) Painel(w http.ResponseWriter, r *http.Request) {
	tela, ok := h.autorizarPainel(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	q := h.db.Queries

	res := TVPainelResponse{
		Tela:          tela.nome,
		Setor:         tela.setorNome,
		AgoraServidor: time.Now().Format(time.RFC3339),
		Monitores:     []TVMonitorResponse{},
		Tickets:       []TVTicketResponse{},
		PlantaoHoje:   []PlantaoResponse{},
		Avisos:        []TVAvisoResponse{},
	}

	// 1. Monitores ativos (fora do ar primeiro, pela ordem da query)
	monitores, err := q.ListarMonitores(ctx)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar monitores")
		return
	}
	fora := make(map[pgtype.UUID]float64)
	if rows, err := q.SegundosForaUltimas24h(ctx); err == nil {
		for _, row := range rows {
			fora[row.MonitorID] = row.Segundos
		}
	}
	for _, m := range monitores {
		if !m.Ativo {
			continue
		}
		mr := paraMonitorResponse(m, fora[m.ID])
		res.Monitores = append(res.Monitores, TVMonitorResponse{
			ID:                 mr.ID,
			Nome:               mr.Nome,
			Tipo:               mr.Tipo,
			Status:             mr.Status,
			LatenciaMs:         mr.LatenciaMs,
			UltimoErro:         mr.UltimoErro,
			StatusDesde:        mr.StatusDesde,
			Disponibilidade24h: mr.Disponibilidade24h,
		})
	}

	// 2. Tickets aguardando (prioridade alta e mais antigos primeiro)
	tickets, err := q.ListarTickets(ctx, sqlc.ListarTicketsParams{
		SetorID: tela.setor,
		Status:  pgtype.Text{String: "aberto", Valid: true},
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar tickets")
		return
	}
	res.TicketsTotal = len(tickets)
	for _, tk := range tickets[:min(len(tickets), ticketsNaTV)] {
		res.Tickets = append(res.Tickets, TVTicketResponse{
			Numero:          tk.Numero,
			Titulo:          tk.Titulo,
			SolicitanteNome: tk.SolicitanteNome,
			Prioridade:      tk.Prioridade,
			CriadoEm:        tk.CriadoEm.Time.Format(time.RFC3339),
		})
	}

	// 3. Quem está de plantão hoje
	hoje := hojeSaoPaulo()
	if lista, err := listarPlantoes(ctx, q, tela.setor, hoje, hoje); err == nil {
		res.PlantaoHoje = escaladosNoDia(lista, hoje)
	}

	// 4. Mural de avisos
	if avisos, err := q.ListarAvisosAtivos(ctx, tela.setor); err == nil {
		for _, a := range avisos {
			res.Avisos = append(res.Avisos, TVAvisoResponse{
				ID:          database.UUIDToString(a.ID),
				Titulo:      a.Titulo,
				Mensagem:    a.Mensagem,
				Nivel:       a.Nivel,
				CriadorNome: a.CriadorNome,
			})
		}
	}

	response.JSON(w, http.StatusOK, res)
}

// ListarTelas: GET /api/tv/telas (admin)
func (h *TVHandler) ListarTelas(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Queries.ListarTelasTV(r.Context(), setorDe(r))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar telas")
		return
	}
	result := make([]TelaTVResponse, 0, len(rows))
	for _, t := range rows {
		result = append(result, TelaTVResponse{
			ID:             database.UUIDToString(t.ID),
			Nome:           t.Nome,
			CriadorNome:    t.CriadorNome,
			CriadoEm:       t.CriadoEm.Time.Format(time.RFC3339),
			UltimoAcessoEm: formatarTimestamptz(t.UltimoAcessoEm),
			UltimoIP:       t.UltimoIp,
		})
	}
	response.JSON(w, http.StatusOK, result)
}

// CriarTela: POST /api/tv/telas (admin). A chave só é mostrada nesta resposta.
func (h *TVHandler) CriarTela(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	uID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar usuário")
		return
	}

	var req struct {
		Nome string `json:"nome"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if req.Nome, err = validarTexto(req.Nome, "nome da tela", true, 80); err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	bruta := make([]byte, 32)
	if _, err := rand.Read(bruta); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao gerar a chave")
		return
	}
	chave := hex.EncodeToString(bruta)

	t, err := h.db.Queries.CriarTelaTV(r.Context(), sqlc.CriarTelaTVParams{
		Nome:      req.Nome,
		ChaveHash: middleware.HashToken(chave),
		CriadoPor: uID,
		SetorID:   user.Setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao cadastrar a tela")
		return
	}

	response.JSON(w, http.StatusCreated, TelaTVResponse{
		ID:          database.UUIDToString(t.ID),
		Nome:        t.Nome,
		CriadorNome: user.Nome,
		CriadoEm:    t.CriadoEm.Time.Format(time.RFC3339),
		Chave:       chave,
	})
}

// DeletarTela: DELETE /api/tv/telas/{id} (admin). Revoga a chave na hora.
func (h *TVHandler) DeletarTela(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	n, err := h.db.Queries.DeletarTelaTV(r.Context(), sqlc.DeletarTelaTVParams{ID: id, SetorID: setorDe(r)})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir a tela")
		return
	}
	if n == 0 {
		response.JSONError(w, http.StatusNotFound, "tela não encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Dias de escala que a TV do plantão mostra a partir de hoje
const diasEscalaNaTV = 28

type TVPlantaoResponse struct {
	Tela          string            `json:"tela"`
	Setor         string            `json:"setor"`
	AgoraServidor string            `json:"agora_servidor"`
	Hoje          string            `json:"hoje"`
	Escala        []PlantaoResponse `json:"escala"`
	Feriados      []Feriado         `json:"feriados"`
}

// Plantao: GET /api/tv/plantao (chave da tela no cabeçalho X-TV-Chave, ou sessão).
// Escala do setor de hoje até quatro semanas à frente, para o telão do plantão.
func (h *TVHandler) Plantao(w http.ResponseWriter, r *http.Request) {
	tela, ok := h.autorizarPainel(w, r)
	if !ok {
		return
	}
	hoje := hojeSaoPaulo()
	escala, err := listarPlantoes(r.Context(), h.db.Queries, tela.setor, hoje, hoje.AddDate(0, 0, diasEscalaNaTV-1))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar a escala")
		return
	}
	response.JSON(w, http.StatusOK, TVPlantaoResponse{
		Tela:          tela.nome,
		Setor:         tela.setorNome,
		AgoraServidor: time.Now().Format(time.RFC3339),
		Hoje:          hoje.Format(formatoDia),
		Escala:        escala,
		Feriados:      feriadosEntre(hoje, hoje.AddDate(0, 0, diasEscalaNaTV-1)),
	})
}
