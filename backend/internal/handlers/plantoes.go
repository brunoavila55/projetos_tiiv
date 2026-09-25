package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Escala de plantão e sobreaviso: toda a equipe vê (no calendário e no painel);
// só admin monta, avulsa ou em rodízio. Datas por dia, fim inclusivo.
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
)

type PlantaoRequest struct {
	UsuarioID  string `json:"usuario_id"`
	Tipo       string `json:"tipo"`
	Inicio     string `json:"inicio"`
	Fim        string `json:"fim"`
	Observacao string `json:"observacao"`
}

type RodizioRequest struct {
	Usuarios     []string `json:"usuarios"`
	Tipo         string   `json:"tipo"`
	Inicio       string   `json:"inicio"`
	DiasPorTurno int      `json:"dias_por_turno"`
	Turnos       int      `json:"turnos"`
	Observacao   string   `json:"observacao"`
}

type PlantaoResponse struct {
	ID          string `json:"id"`
	UsuarioID   string `json:"usuario_id"`
	UsuarioNome string `json:"usuario_nome"`
	UsuarioCor  string `json:"usuario_cor"`
	Tipo        string `json:"tipo"`
	Inicio      string `json:"inicio"`
	Fim         string `json:"fim"`
	Observacao  string `json:"observacao"`
}

// PainelPlantaoResponse vai no GET /api/painel
type PainelPlantaoResponse struct {
	Hoje       []PlantaoResponse `json:"hoje"`
	MeuProximo *PlantaoResponse  `json:"meu_proximo"`
}

// turno é um plantão validado, pronto para gravar
type turno struct {
	usuarioID  pgtype.UUID
	nome       string
	tipo       string
	inicio     time.Time
	fim        time.Time
	observacao string
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

// usuarioEscalavel confere se o operador existe e está ativo
func (h *PlantaoHandler) usuarioEscalavel(ctx context.Context, id string) (pgtype.UUID, string, error) {
	uID, err := database.StringToUUID(id)
	if err != nil {
		return uID, "", errors.New("operador inválido")
	}
	u, err := h.db.Queries.BuscarUsuarioPorID(ctx, uID)
	if err != nil || !u.Ativo {
		return uID, "", errors.New("operador não encontrado ou inativo")
	}
	return uID, u.Nome, nil
}

func (h *PlantaoHandler) validarPlantao(ctx context.Context, req PlantaoRequest) (turno, error) {
	var t turno
	var err error
	if t.tipo, err = validarTipoPlantao(req.Tipo); err != nil {
		return t, err
	}
	if t.usuarioID, t.nome, err = h.usuarioEscalavel(ctx, req.UsuarioID); err != nil {
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
	t.observacao, err = validarTexto(req.Observacao, "observação", false, 300)
	return t, err
}

// gravarTurnos grava tudo ou nada; um conflito devolve errConflitoEscala
// com a mensagem para o usuário.
func (h *PlantaoHandler) gravarTurnos(ctx context.Context, turnos []turno, criadoPor pgtype.UUID, atualizarID pgtype.UUID) (string, error) {
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := h.db.Queries.WithTx(tx)

	if err := q.TravarEscala(ctx); err != nil {
		return "", err
	}
	for _, t := range turnos {
		c, err := q.BuscarConflitoPlantao(ctx, sqlc.BuscarConflitoPlantaoParams{
			UsuarioID: t.usuarioID,
			Tipo:      t.tipo,
			Inicio:    paraDate(t.inicio),
			Fim:       paraDate(t.fim),
			IgnorarID: atualizarID,
		})
		if err == nil {
			return fmt.Sprintf("%s já está de %s de %s a %s", c.UsuarioNome, rotuloTipoPlantao(t.tipo), diaCurto(c.Inicio), diaCurto(c.Fim)), errConflitoEscala
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}

	for _, t := range turnos {
		if atualizarID.Valid {
			_, err = q.AtualizarPlantao(ctx, sqlc.AtualizarPlantaoParams{
				ID: atualizarID, UsuarioID: t.usuarioID, Tipo: t.tipo,
				Inicio: paraDate(t.inicio), Fim: paraDate(t.fim), Observacao: t.observacao,
			})
		} else {
			_, err = q.CriarPlantao(ctx, sqlc.CriarPlantaoParams{
				UsuarioID: t.usuarioID, Tipo: t.tipo,
				Inicio: paraDate(t.inicio), Fim: paraDate(t.fim), Observacao: t.observacao,
				CriadoPor: criadoPor,
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
		UsuarioID:   database.UUIDToString(p.UsuarioID),
		UsuarioNome: p.UsuarioNome,
		UsuarioCor:  p.UsuarioCor,
		Tipo:        p.Tipo,
		Inicio:      p.Inicio.Time.Format(formatoDia),
		Fim:         p.Fim.Time.Format(formatoDia),
		Observacao:  p.Observacao,
	}
}

func listarPlantoes(ctx context.Context, q *sqlc.Queries, de, ate time.Time) ([]PlantaoResponse, error) {
	rows, err := q.ListarPlantoesIntervalo(ctx, sqlc.ListarPlantoesIntervaloParams{De: paraDate(de), Ate: paraDate(ate)})
	if err != nil {
		return nil, err
	}
	result := make([]PlantaoResponse, 0, len(rows))
	for _, p := range rows {
		result = append(result, paraPlantaoResponse(p))
	}
	return result, nil
}

// plantaoNoPainel: quem está escalado hoje e o turno atual/próximo do usuário
func plantaoNoPainel(ctx context.Context, q *sqlc.Queries, user *middleware.AuthUser) PainelPlantaoResponse {
	hoje := hojeSaoPaulo()
	res := PainelPlantaoResponse{Hoje: []PlantaoResponse{}}
	if lista, err := listarPlantoes(ctx, q, hoje, hoje); err == nil {
		res.Hoje = lista
	}

	uID, err := database.StringToUUID(user.ID)
	if err != nil {
		return res
	}
	p, err := q.ProximoPlantaoUsuario(ctx, sqlc.ProximoPlantaoUsuarioParams{UsuarioID: uID, Dia: paraDate(hoje)})
	if err == nil {
		res.MeuProximo = &PlantaoResponse{
			ID:          database.UUIDToString(p.ID),
			UsuarioID:   user.ID,
			UsuarioNome: user.Nome,
			UsuarioCor:  user.Cor,
			Tipo:        p.Tipo,
			Inicio:      p.Inicio.Time.Format(formatoDia),
			Fim:         p.Fim.Time.Format(formatoDia),
		}
	}
	return res
}

// Listar: GET /api/plantoes?inicio=AAAA-MM-DD&fim=AAAA-MM-DD
func (h *PlantaoHandler) Listar(w http.ResponseWriter, r *http.Request) {
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

	lista, err := listarPlantoes(r.Context(), h.db.Queries, de, ate)
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
	t, err := h.validarPlantao(r.Context(), req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	criador, _ := database.StringToUUID(user.ID)
	msg, err := h.gravarTurnos(r.Context(), []turno{t}, criador, pgtype.UUID{})
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
	if len(req.Usuarios) == 0 || len(req.Usuarios) > 50 {
		response.JSONError(w, http.StatusBadRequest, "escolha de 1 a 50 operadores para o rodízio")
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

	type pessoa struct {
		id   pgtype.UUID
		nome string
	}
	pessoas := make([]pessoa, 0, len(req.Usuarios))
	vistos := make(map[string]bool)
	for _, id := range req.Usuarios {
		if vistos[id] {
			response.JSONError(w, http.StatusBadRequest, "o mesmo operador aparece duas vezes no rodízio")
			return
		}
		vistos[id] = true
		uID, nome, err := h.usuarioEscalavel(r.Context(), id)
		if err != nil {
			response.JSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		pessoas = append(pessoas, pessoa{uID, nome})
	}

	turnos := make([]turno, 0, req.Turnos)
	for i := 0; i < req.Turnos; i++ {
		p := pessoas[i%len(pessoas)]
		ini := inicio.AddDate(0, 0, i*req.DiasPorTurno)
		turnos = append(turnos, turno{
			usuarioID: p.id, nome: p.nome, tipo: tipo,
			inicio: ini, fim: ini.AddDate(0, 0, req.DiasPorTurno-1),
			observacao: observacao,
		})
	}

	criador, _ := database.StringToUUID(user.ID)
	msg, err := h.gravarTurnos(r.Context(), turnos, criador, pgtype.UUID{})
	responderGravacao(w, msg, err, http.StatusCreated, map[string]int{"criados": len(turnos)})
}

func (h *PlantaoHandler) idDaURL(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return id, false
	}
	if _, err := h.db.Queries.ObterPlantao(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
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
	id, ok := h.idDaURL(w, r)
	if !ok {
		return
	}
	var req PlantaoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	t, err := h.validarPlantao(r.Context(), req)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, err := h.gravarTurnos(r.Context(), []turno{t}, pgtype.UUID{}, id)
	responderGravacao(w, msg, err, http.StatusNoContent, nil)
}

// Deletar: DELETE /api/plantoes/{id} (admin)
func (h *PlantaoHandler) Deletar(w http.ResponseWriter, r *http.Request) {
	id, ok := h.idDaURL(w, r)
	if !ok {
		return
	}
	if err := h.db.Queries.DeletarPlantao(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir turno")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
