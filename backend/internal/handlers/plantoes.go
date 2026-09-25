package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// Escala de plantão e sobreaviso: módulo próprio (calendário, dash e modo TV),
// que toda a equipe vê; só admin monta, avulsa ou em rodízio. Quem fica
// escalado é só um nome em texto livre, não precisa ser usuário do sistema.
// Datas por dia, fim inclusivo.
type PlantaoHandler struct {
	db *database.DB
}

func NewPlantaoHandler(db *database.DB) *PlantaoHandler {
	return &PlantaoHandler{db: db}
}

const (
	formatoDia = "2006-01-02"
	// Limites do rodízio: até 2 anos de escala gerados de uma vez
	maxTurnosRodizio = 104
	maxDiasRodizio   = 730
	maxDiasFolga     = 31
)

type PlantaoRequest struct {
	Nome       string `json:"nome"`
	Tipo       string `json:"tipo"`
	Inicio     string `json:"inicio"`
	Fim        string `json:"fim"`
	Observacao string `json:"observacao"`
	// Folga opcional da pessoa; sem fim, é um dia só
	FolgaInicio string `json:"folga_inicio"`
	FolgaFim    string `json:"folga_fim"`
}

type RodizioRequest struct {
	Pessoas      []string `json:"pessoas"`
	Tipo         string   `json:"tipo"`
	Inicio       string   `json:"inicio"`
	DiasPorTurno int      `json:"dias_por_turno"`
	Turnos       int      `json:"turnos"`
	Observacao   string   `json:"observacao"`
	// Folga de um dia, tantos dias antes do começo de cada turno (0 = sem
	// folga). Ex.: plantão no domingo com 3 → folga na quinta.
	FolgaDiasAntes int `json:"folga_dias_antes"`
}

type PlantaoResponse struct {
	ID          string  `json:"id"`
	Nome        string  `json:"nome"`
	Tipo        string  `json:"tipo"`
	Inicio      string  `json:"inicio"`
	Fim         string  `json:"fim"`
	Observacao  string  `json:"observacao"`
	FolgaInicio *string `json:"folga_inicio"`
	FolgaFim    *string `json:"folga_fim"`
}

// PainelPlantaoResponse vai no GET /api/painel
type PainelPlantaoResponse struct {
	Hoje       []PlantaoResponse `json:"hoje"`
	MeuProximo *PlantaoResponse  `json:"meu_proximo"`
}

// turno é um plantão validado, pronto para gravar
type turno struct {
	nome       string
	tipo       string
	inicio     time.Time
	fim        time.Time
	observacao string
	// Folga opcional; zero quando não há
	folgaInicio time.Time
	folgaFim    time.Time
}

func (t turno) temFolga() bool { return !t.folgaInicio.IsZero() }

func dateOpcional(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return paraDate(t)
}

func diaOpcional(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(formatoDia)
	return &s
}

var errConflitoEscala = errors.New("conflito na escala")

func hojeSaoPaulo() time.Time {
	a, m, d := time.Now().In(fusoSaoPaulo).Date()
	return time.Date(a, m, d, 0, 0, 0, 0, time.UTC)
}

func parseDia(s, campo string) (time.Time, error) {
	t, err := time.Parse(formatoDia, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s inválido (use AAAA-MM-DD)", campo)
	}
	return t, nil
}

func paraDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func diaCurto(d pgtype.Date) string {
	return d.Time.Format("02/01")
}

func rotuloTipoPlantao(tipo string) string {
	if tipo == "sobreaviso" {
		return "sobreaviso"
	}
	return "plantão"
}

func validarTipoPlantao(tipo string) (string, error) {
	if tipo == "" {
		return "plantao", nil
	}
	if tipo != "plantao" && tipo != "sobreaviso" {
		return "", errors.New("tipo inválido (plantao ou sobreaviso)")
	}
	return tipo, nil
}

// validarNomePlantao junta espaços repetidos, para "Ana  Paula" e
// "Ana Paula" serem a mesma pessoa na checagem de conflito
func validarNomePlantao(nome string) (string, error) {
	return validarTexto(strings.Join(strings.Fields(nome), " "), "nome de quem fica de plantão", true, 80)
}

func validarPlantao(req PlantaoRequest) (turno, error) {
	var t turno
	var err error
	if t.tipo, err = validarTipoPlantao(req.Tipo); err != nil {
		return t, err
	}
	if t.nome, err = validarNomePlantao(req.Nome); err != nil {
		return t, err
	}
	if t.inicio, err = parseDia(req.Inicio, "dia de início"); err != nil {
		return t, err
	}
	if req.Fim == "" {
		t.fim = t.inicio
	} else if t.fim, err = parseDia(req.Fim, "dia de término"); err != nil {
		return t, err
	}
	if t.fim.Before(t.inicio) {
		return t, errors.New("o último dia não pode ser anterior ao primeiro")
	}
	if t.fim.Sub(t.inicio) > maxDiasRodizio*24*time.Hour {
		return t, errors.New("um turno não pode passar de 2 anos")
	}
	if t.observacao, err = validarTexto(req.Observacao, "observação", false, 300); err != nil {
		return t, err
	}

	if req.FolgaInicio == "" {
		return t, nil
	}
	if t.folgaInicio, err = parseDia(req.FolgaInicio, "primeiro dia de folga"); err != nil {
		return t, err
	}
	if req.FolgaFim == "" {
		t.folgaFim = t.folgaInicio
	} else if t.folgaFim, err = parseDia(req.FolgaFim, "último dia de folga"); err != nil {
		return t, err
	}
	if t.folgaFim.Before(t.folgaInicio) {
		return t, errors.New("o último dia de folga não pode ser anterior ao primeiro")
	}
	if t.folgaFim.Sub(t.folgaInicio) >= maxDiasFolga*24*time.Hour {
		return t, fmt.Errorf("a folga pode ter no máximo %d dias", maxDiasFolga)
	}
	if !t.folgaFim.Before(t.inicio) && !t.folgaInicio.After(t.fim) {
		return t, errors.New("a folga não pode cair nos dias do próprio turno")
	}
	return t, nil
}

// conferirTurno barra turno do mesmo tipo sobreposto e trabalho em dia de
// folga da pessoa; no conflito devolve errConflitoEscala e a mensagem.
func conferirTurno(ctx context.Context, q *sqlc.Queries, setor pgtype.UUID, t turno, ignorarID pgtype.UUID) (string, error) {
	c, err := q.BuscarConflitoPlantao(ctx, sqlc.BuscarConflitoPlantaoParams{
		SetorID:   setor,
		Nome:      t.nome,
		Tipo:      t.tipo,
		Inicio:    paraDate(t.inicio),
		Fim:       paraDate(t.fim),
		IgnorarID: ignorarID,
	})
	if err == nil {
		return fmt.Sprintf("%s já está de %s de %s a %s", c.Nome, rotuloTipoPlantao(t.tipo), diaCurto(c.Inicio), diaCurto(c.Fim)), errConflitoEscala
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	f, err := q.BuscarConflitoFolga(ctx, sqlc.BuscarConflitoFolgaParams{
		SetorID:         setor,
		Nome:            t.nome,
		IgnorarID:       ignorarID,
		Inicio:          paraDate(t.inicio),
		Fim:             paraDate(t.fim),
		NovaFolgaInicio: dateOpcional(t.folgaInicio),
		NovaFolgaFim:    dateOpcional(t.folgaFim),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if f.FolgaInicio.Valid && !f.FolgaFim.Time.Before(t.inicio) && !f.FolgaInicio.Time.After(t.fim) {
		return fmt.Sprintf("%s está de folga de %s a %s", t.nome, diaCurto(f.FolgaInicio), diaCurto(f.FolgaFim)), errConflitoEscala
	}
	return fmt.Sprintf("a folga de %s cai no %s de %s a %s", t.nome, rotuloTipoPlantao(f.Tipo), diaCurto(f.Inicio), diaCurto(f.Fim)), errConflitoEscala
}

// gravarTurnos grava tudo ou nada; um conflito devolve errConflitoEscala
// com a mensagem para o usuário.
func (h *PlantaoHandler) gravarTurnos(ctx context.Context, setor pgtype.UUID, turnos []turno, criadoPor pgtype.UUID, atualizarID pgtype.UUID) (string, error) {
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := h.db.Queries.WithTx(tx)

	if err := q.TravarEscala(ctx); err != nil {
		return "", err
	}
	// Confere e grava um a um: os turnos do mesmo rodízio também se checam
	for _, t := range turnos {
		if msg, err := conferirTurno(ctx, q, setor, t, atualizarID); err != nil {
			return msg, err
		}
		if atualizarID.Valid {
			_, err = q.AtualizarPlantao(ctx, sqlc.AtualizarPlantaoParams{
				ID: atualizarID, Nome: t.nome, Tipo: t.tipo,
				Inicio: paraDate(t.inicio), Fim: paraDate(t.fim), Observacao: t.observacao,
				FolgaInicio: dateOpcional(t.folgaInicio), FolgaFim: dateOpcional(t.folgaFim),
			})
		} else {
			_, err = q.CriarPlantao(ctx, sqlc.CriarPlantaoParams{
				Nome: t.nome, Tipo: t.tipo,
				Inicio: paraDate(t.inicio), Fim: paraDate(t.fim), Observacao: t.observacao,
				FolgaInicio: dateOpcional(t.folgaInicio), FolgaFim: dateOpcional(t.folgaFim),
				CriadoPor: criadoPor, SetorID: setor,
			})
		}
		if err != nil {
			return "", err
		}
	}
	return "", tx.Commit(ctx)
}

func paraPlantaoResponse(p sqlc.ListarPlantoesIntervaloRow) PlantaoResponse {
	return PlantaoResponse{
		ID:          database.UUIDToString(p.ID),
		Nome:        p.Nome,
		Tipo:        p.Tipo,
		Inicio:      p.Inicio.Time.Format(formatoDia),
		Fim:         p.Fim.Time.Format(formatoDia),
		Observacao:  p.Observacao,
		FolgaInicio: diaOpcional(p.FolgaInicio),
		FolgaFim:    diaOpcional(p.FolgaFim),
	}
}

// escaladosNoDia deixa só quem trabalha no dia (a lista também traz quem
// aparece só pela folga)
func escaladosNoDia(lista []PlantaoResponse, dia time.Time) []PlantaoResponse {
	d := dia.Format(formatoDia)
	res := make([]PlantaoResponse, 0, len(lista))
	for _, p := range lista {
		if p.Inicio <= d && p.Fim >= d {
			res = append(res, p)
		}
	}
	return res
}

func listarPlantoes(ctx context.Context, q *sqlc.Queries, setor pgtype.UUID, de, ate time.Time) ([]PlantaoResponse, error) {
	rows, err := q.ListarPlantoesIntervalo(ctx, sqlc.ListarPlantoesIntervaloParams{SetorID: setor, De: paraDate(de), Ate: paraDate(ate)})
	if err != nil {
		return nil, err
	}
	result := make([]PlantaoResponse, 0, len(rows))
	for _, p := range rows {
		result = append(result, paraPlantaoResponse(p))
	}
	return result, nil
}

// plantaoNoPainel: quem está escalado hoje e o turno atual/próximo de quem
// está na escala com o mesmo nome do usuário
func plantaoNoPainel(ctx context.Context, q *sqlc.Queries, user *middleware.AuthUser) PainelPlantaoResponse {
	hoje := hojeSaoPaulo()
	res := PainelPlantaoResponse{Hoje: []PlantaoResponse{}}
	if lista, err := listarPlantoes(ctx, q, user.Setor, hoje, hoje); err == nil {
		res.Hoje = escaladosNoDia(lista, hoje)
	}

	p, err := q.ProximoPlantaoPorNome(ctx, sqlc.ProximoPlantaoPorNomeParams{SetorID: user.Setor, Nome: user.Nome, Dia: paraDate(hoje)})
	if err == nil {
		meu := paraPlantaoResponse(sqlc.ListarPlantoesIntervaloRow(p))
		res.MeuProximo = &meu
	}
	return res
}

// Listar: GET /api/plantoes?inicio=AAAA-MM-DD&fim=AAAA-MM-DD
func (h *PlantaoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	hoje := hojeSaoPaulo()
	de, ate := hoje.AddDate(0, -1, 0), hoje.AddDate(0, 2, 0)
	var err error
	if s := r.URL.Query().Get("inicio"); s != "" {
		if de, err = parseDia(s, "início"); err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if s := r.URL.Query().Get("fim"); s != "" {
		if ate, err = parseDia(s, "fim"); err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if ate.Before(de) || ate.Sub(de) > 400*24*time.Hour {
		response.JSONError(w, http.StatusBadRequest, "intervalo inválido (máximo de 400 dias)")
		return
	}

	lista, err := listarPlantoes(r.Context(), h.db.Queries, user.Setor, de, ate)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar a escala")
		return
	}
	response.JSON(w, http.StatusOK, lista)
}

func responderGravacao(w http.ResponseWriter, msg string, err error, status int, corpo any) {
	if errors.Is(err, errConflitoEscala) {
		response.JSONError(w, http.StatusConflict, msg)
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao gravar a escala")
		return
	}
	if corpo == nil {
		w.WriteHeader(status)
		return
	}
	response.JSON(w, status, corpo)
}

// Criar: POST /api/plantoes (admin)
func (h *PlantaoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	var req PlantaoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	t, err := validarPlantao(req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	criador, _ := database.StringToUUID(user.ID)
	msg, err := h.gravarTurnos(r.Context(), user.Setor, []turno{t}, criador, pgtype.UUID{})
	responderGravacao(w, msg, err, http.StatusCreated, map[string]int{"criados": 1})
}

// Rodizio: POST /api/plantoes/rodizio (admin) — gera turnos seguidos
// alternando as pessoas na ordem enviada
func (h *PlantaoHandler) Rodizio(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	var req RodizioRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	tipo, err := validarTipoPlantao(req.Tipo)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	inicio, err := parseDia(req.Inicio, "dia de início")
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Pessoas) == 0 || len(req.Pessoas) > 50 {
		response.JSONError(w, http.StatusBadRequest, "o rodízio deve ter de 1 a 50 pessoas")
		return
	}
	if req.DiasPorTurno < 1 || req.DiasPorTurno > 31 {
		response.JSONError(w, http.StatusBadRequest, "cada turno deve durar de 1 a 31 dias")
		return
	}
	if req.Turnos < 1 || req.Turnos > maxTurnosRodizio || req.Turnos*req.DiasPorTurno > maxDiasRodizio {
		response.JSONError(w, http.StatusBadRequest, "o rodízio deve ter de 1 a 104 turnos e cobrir no máximo 2 anos")
		return
	}
	observacao, err := validarTexto(req.Observacao, "observação", false, 300)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.FolgaDiasAntes < 0 || req.FolgaDiasAntes > maxDiasFolga {
		response.JSONError(w, http.StatusBadRequest, fmt.Sprintf("a folga deve ficar de 1 a %d dias antes do turno", maxDiasFolga))
		return
	}

	pessoas := make([]string, 0, len(req.Pessoas))
	vistos := make(map[string]bool)
	for _, n := range req.Pessoas {
		nome, err := validarNomePlantao(n)
		if err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		chave := strings.ToLower(nome)
		if vistos[chave] {
			response.JSONError(w, http.StatusBadRequest, fmt.Sprintf("%s aparece duas vezes no rodízio", nome))
			return
		}
		vistos[chave] = true
		pessoas = append(pessoas, nome)
	}

	turnos := make([]turno, 0, req.Turnos)
	for i := 0; i < req.Turnos; i++ {
		ini := inicio.AddDate(0, 0, i*req.DiasPorTurno)
		t := turno{
			nome: pessoas[i%len(pessoas)], tipo: tipo,
			inicio: ini, fim: ini.AddDate(0, 0, req.DiasPorTurno-1),
			observacao: observacao,
		}
		// Folga de um dia antes do turno
		if req.FolgaDiasAntes > 0 {
			t.folgaInicio = t.inicio.AddDate(0, 0, -req.FolgaDiasAntes)
			t.folgaFim = t.folgaInicio
		}
		turnos = append(turnos, t)
	}

	criador, _ := database.StringToUUID(user.ID)
	msg, err := h.gravarTurnos(r.Context(), user.Setor, turnos, criador, pgtype.UUID{})
	responderGravacao(w, msg, err, http.StatusCreated, map[string]int{"criados": len(turnos)})
}

// idDaURL confere que o turno existe no setor de quem está pedindo
func (h *PlantaoHandler) idDaURL(w http.ResponseWriter, r *http.Request, user *middleware.AuthUser) (pgtype.UUID, bool) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return id, false
	}
	if _, err := h.db.Queries.ObterPlantao(r.Context(), sqlc.ObterPlantaoParams{ID: id, SetorID: user.Setor}); errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "turno não encontrado")
		return id, false
	} else if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao buscar turno")
		return id, false
	}
	return id, true
}

// Atualizar: PUT /api/plantoes/{id} (admin)
func (h *PlantaoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	id, ok := h.idDaURL(w, r, user)
	if !ok {
		return
	}
	var req PlantaoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	t, err := validarPlantao(req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, err := h.gravarTurnos(r.Context(), user.Setor, []turno{t}, pgtype.UUID{}, id)
	responderGravacao(w, msg, err, http.StatusNoContent, nil)
}

// Deletar: DELETE /api/plantoes/{id} (admin)
func (h *PlantaoHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	id, ok := h.idDaURL(w, r, user)
	if !ok {
		return
	}
	if err := h.db.Queries.DeletarPlantao(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir turno")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Pessoas: GET /api/plantoes/pessoas — nomes já usados na escala do setor
func (h *PlantaoHandler) Pessoas(w http.ResponseWriter, r *http.Request) {
	nomes, err := h.db.Queries.ListarPessoasPlantao(r.Context(), setorDe(r))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar as pessoas da escala")
		return
	}
	if nomes == nil {
		nomes = []string{}
	}
	response.JSON(w, http.StatusOK, nomes)
}
