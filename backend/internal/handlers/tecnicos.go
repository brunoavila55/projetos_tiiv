package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // garante America/Sao_Paulo mesmo sem tzdata no sistema

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

var fusoSaoPaulo = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.Local
	}
	return loc
}()

type TecnicoHandler struct {
	db *database.DB
}

func NewTecnicoHandler(db *database.DB) *TecnicoHandler {
	return &TecnicoHandler{db: db}
}

type TecnicoResponse struct {
	ID               string  `json:"id"`
	Nome             string  `json:"nome"`
	Empresa          string  `json:"empresa"`
	Ativo            bool    `json:"ativo"`
	CriadoEm         string  `json:"criado_em"`
	RegistroAbertoID *string `json:"registro_aberto_id"`
	EntradaAberta    *string `json:"entrada_aberta"`
	// O que já foi anotado na visita em andamento
	AtividadesAbertas string `json:"atividades_abertas"`
}

type RegistroTecnicoResponse struct {
	ID                       string  `json:"id"`
	TecnicoID                string  `json:"tecnico_id"`
	TecnicoNome              string  `json:"tecnico_nome"`
	TecnicoEmpresa           string  `json:"tecnico_empresa"`
	Entrada                  string  `json:"entrada"`
	Saida                    *string `json:"saida"`
	DuracaoSegundos          *int64  `json:"duracao_segundos"`
	Observacao               string  `json:"observacao"`
	Atividades               string  `json:"atividades"`
	EntradaRegistradaPorNome string  `json:"entrada_registrada_por_nome"`
	SaidaRegistradaPorNome   *string `json:"saida_registrada_por_nome"`
}

type ListarRegistrosTecnicosResponse struct {
	Itens      []RegistroTecnicoResponse `json:"itens"`
	Total      int64                     `json:"total"`
	Pagina     int                       `json:"pagina"`
	Limite     int                       `json:"limite"`
	TotalPages int                       `json:"total_pages"`
}

type RelatorioTecnicoResponse struct {
	TecnicoID        string  `json:"tecnico_id"`
	TecnicoNome      string  `json:"tecnico_nome"`
	TecnicoEmpresa   string  `json:"tecnico_empresa"`
	TotalRegistros   int64   `json:"total_registros"`
	RegistrosAbertos int64   `json:"registros_abertos"`
	DiasPresentes    int64   `json:"dias_presentes"`
	SegundosTotais   int64   `json:"segundos_totais"`
	PrimeiraEntrada  *string `json:"primeira_entrada"`
	UltimaMarcacao   *string `json:"ultima_marcacao"`
}

func formatarTimestamptz(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	s := ts.Time.Format(time.RFC3339)
	return &s
}

// filtroPeriodoTecnicos lê tecnico_id, inicio e fim (YYYY-MM-DD, no fuso de
// São Paulo). O fim é inclusivo: vira o início do dia seguinte, exclusivo.
func filtroPeriodoTecnicos(r *http.Request) (tecnicoID pgtype.UUID, inicio, fim pgtype.Timestamptz, err error) {
	q := r.URL.Query()
	if s := q.Get("tecnico_id"); s != "" {
		if tecnicoID, err = database.StringToUUID(s); err != nil {
			return tecnicoID, inicio, fim, errors.New("tecnico_id inválido")
		}
	}
	if s := q.Get("inicio"); s != "" {
		t, perr := time.ParseInLocation("2006-01-02", s, fusoSaoPaulo)
		if perr != nil {
			return tecnicoID, inicio, fim, errors.New("data de início inválida (use AAAA-MM-DD)")
		}
		inicio = database.TimeToTimestamptz(t)
	}
	if s := q.Get("fim"); s != "" {
		t, perr := time.ParseInLocation("2006-01-02", s, fusoSaoPaulo)
		if perr != nil {
			return tecnicoID, inicio, fim, errors.New("data de fim inválida (use AAAA-MM-DD)")
		}
		fim = database.TimeToTimestamptz(t.AddDate(0, 0, 1))
	}
	return tecnicoID, inicio, fim, nil
}

// horarioInformado aceita um horário opcional em RFC3339 (para marcações
// retroativas); vazio significa "agora". Horários futuros são recusados.
func horarioInformado(s string) (time.Time, error) {
	agora := time.Now()
	if s == "" {
		return agora, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errors.New("horário inválido")
	}
	if t.After(agora.Add(time.Minute)) {
		return time.Time{}, errors.New("o horário não pode estar no futuro")
	}
	return t, nil
}

func codigoErroPg(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func tecnicoParaResponse(t sqlc.Tecnicos) TecnicoResponse {
	return TecnicoResponse{
		ID:       database.UUIDToString(t.ID),
		Nome:     t.Nome,
		Empresa:  t.Empresa,
		Ativo:    t.Ativo,
		CriadoEm: t.CriadoEm.Time.Format(time.RFC3339),
	}
}

// Listar: GET /api/tecnicos?ativo=
func (h *TecnicoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	var ativoParam *bool
	if s := r.URL.Query().Get("ativo"); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			ativoParam = &b
		}
	}

	tecnicos, err := h.db.Queries.ListarTecnicos(r.Context(), database.BoolToPgtypeBool(ativoParam))
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar técnicos")
		return
	}

	result := make([]TecnicoResponse, 0, len(tecnicos))
	for _, t := range tecnicos {
		var registroAberto *string
		if t.RegistroAbertoID.Valid {
			s := database.UUIDToString(t.RegistroAbertoID)
			registroAberto = &s
		}
		result = append(result, TecnicoResponse{
			ID:                database.UUIDToString(t.ID),
			Nome:              t.Nome,
			Empresa:           t.Empresa,
			Ativo:             t.Ativo,
			CriadoEm:          t.CriadoEm.Time.Format(time.RFC3339),
			RegistroAbertoID:  registroAberto,
			EntradaAberta:     formatarTimestamptz(t.EntradaAberta),
			AtividadesAbertas: t.AtividadesAbertas.String,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type SalvarTecnicoRequest struct {
	Nome    string `json:"nome"`
	Empresa string `json:"empresa"`
	Ativo   *bool  `json:"ativo"`
}

// Criar: POST /api/tecnicos
func (h *TecnicoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req SalvarTecnicoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	req.Nome = strings.TrimSpace(req.Nome)
	if req.Nome == "" {
		response.JSONError(w, http.StatusBadRequest, "nome do técnico é obrigatório")
		return
	}

	t, err := h.db.Queries.CriarTecnico(r.Context(), sqlc.CriarTecnicoParams{
		Nome:    req.Nome,
		Empresa: strings.TrimSpace(req.Empresa),
	})
	if err != nil {
		if codigoErroPg(err) == "23505" {
			response.JSONError(w, http.StatusConflict, "já existe um técnico com este nome")
			return
		}
		response.JSONError(w, http.StatusInternalServerError, "erro ao cadastrar técnico")
		return
	}

	response.JSON(w, http.StatusCreated, tecnicoParaResponse(t))
}

// Atualizar: PUT /api/tecnicos/{id}
func (h *TecnicoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	var req SalvarTecnicoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	req.Nome = strings.TrimSpace(req.Nome)
	if req.Nome == "" {
		response.JSONError(w, http.StatusBadRequest, "nome do técnico é obrigatório")
		return
	}

	atual, err := h.db.Queries.BuscarTecnicoPorID(r.Context(), id)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "técnico não encontrado")
		return
	}
	ativo := atual.Ativo
	if req.Ativo != nil {
		ativo = *req.Ativo
	}

	t, err := h.db.Queries.AtualizarTecnico(r.Context(), sqlc.AtualizarTecnicoParams{
		ID:      id,
		Nome:    req.Nome,
		Empresa: strings.TrimSpace(req.Empresa),
		Ativo:   ativo,
	})
	if err != nil {
		if codigoErroPg(err) == "23505" {
			response.JSONError(w, http.StatusConflict, "já existe um técnico com este nome")
			return
		}
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar técnico")
		return
	}

	response.JSON(w, http.StatusOK, tecnicoParaResponse(t))
}

type MarcacaoRequest struct {
	Horario    string `json:"horario"` // RFC3339 opcional; vazio = agora
	Observacao string `json:"observacao"`
	Atividades string `json:"atividades"` // só na saída: o que o técnico fez
}

// Limite do texto "o que foi feito" de uma visita
const maxAtividades = 4000

func lerMarcacao(r *http.Request) (MarcacaoRequest, error) {
	var req MarcacaoRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, errors.New("corpo da requisição inválido")
		}
	}
	req.Observacao = strings.TrimSpace(req.Observacao)
	var err error
	if req.Atividades, err = validarTexto(req.Atividades, "o que foi feito", false, maxAtividades); err != nil {
		return req, err
	}
	return req, nil
}

// RegistrarEntrada: POST /api/tecnicos/{id}/entrada
func (h *TecnicoHandler) RegistrarEntrada(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	tecnicoID, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	req, err := lerMarcacao(r)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	horario, err := horarioInformado(req.Horario)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	tecnico, err := h.db.Queries.BuscarTecnicoPorID(r.Context(), tecnicoID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "técnico não encontrado")
		return
	}
	if !tecnico.Ativo {
		response.JSONError(w, http.StatusBadRequest, "técnico está desativado")
		return
	}

	uID, _ := database.StringToUUID(user.ID)
	reg, err := h.db.Queries.RegistrarEntradaTecnico(r.Context(), sqlc.RegistrarEntradaTecnicoParams{
		TecnicoID:            tecnicoID,
		Entrada:              database.TimeToTimestamptz(horario),
		Observacao:           req.Observacao,
		EntradaRegistradaPor: uID,
	})
	if err != nil {
		if codigoErroPg(err) == "23505" {
			response.JSONError(w, http.StatusConflict, fmt.Sprintf("%s já está com entrada registrada; marque a saída primeiro", tecnico.Nome))
			return
		}
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar entrada")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"id":      database.UUIDToString(reg.ID),
		"entrada": reg.Entrada.Time.Format(time.RFC3339),
	})
}

// RegistrarSaida: POST /api/tecnicos/{id}/saida
func (h *TecnicoHandler) RegistrarSaida(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	tecnicoID, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	req, err := lerMarcacao(r)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	horario, err := horarioInformado(req.Horario)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	uID, _ := database.StringToUUID(user.ID)
	reg, err := h.db.Queries.RegistrarSaidaTecnico(r.Context(), sqlc.RegistrarSaidaTecnicoParams{
		TecnicoID:          tecnicoID,
		Saida:              database.TimeToTimestamptz(horario),
		SaidaRegistradaPor: uID,
		Observacao:         req.Observacao,
		Atividades:         req.Atividades,
	})
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			response.JSONError(w, http.StatusConflict, "o técnico não tem entrada em aberto")
		case codigoErroPg(err) == "23514":
			response.JSONError(w, http.StatusBadRequest, "a saída não pode ser anterior à entrada")
		default:
			response.JSONError(w, http.StatusInternalServerError, "erro ao registrar saída")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"id":               database.UUIDToString(reg.ID),
		"entrada":          reg.Entrada.Time.Format(time.RFC3339),
		"saida":            reg.Saida.Time.Format(time.RFC3339),
		"duracao_segundos": int64(reg.Saida.Time.Sub(reg.Entrada.Time).Seconds()),
	})
}

func (h *TecnicoHandler) consultarRegistros(r *http.Request, limite, offset int) ([]RegistroTecnicoResponse, error) {
	tecnicoID, inicio, fim, err := filtroPeriodoTecnicos(r)
	if err != nil {
		return nil, err
	}
	rows, err := h.db.Queries.ListarRegistrosTecnicos(r.Context(), sqlc.ListarRegistrosTecnicosParams{
		Limit:      int32(limite),
		Offset:     int32(offset),
		TecnicoID:  tecnicoID,
		DataInicio: inicio,
		DataFim:    fim,
	})
	if err != nil {
		return nil, errors.New("erro ao listar registros")
	}

	result := make([]RegistroTecnicoResponse, 0, len(rows))
	for _, row := range rows {
		var duracao *int64
		if row.Saida.Valid {
			d := int64(row.Saida.Time.Sub(row.Entrada.Time).Seconds())
			duracao = &d
		}
		result = append(result, RegistroTecnicoResponse{
			ID:                       database.UUIDToString(row.ID),
			TecnicoID:                database.UUIDToString(row.TecnicoID),
			TecnicoNome:              row.TecnicoNome,
			TecnicoEmpresa:           row.TecnicoEmpresa,
			Entrada:                  row.Entrada.Time.Format(time.RFC3339),
			Saida:                    formatarTimestamptz(row.Saida),
			DuracaoSegundos:          duracao,
			Observacao:               row.Observacao,
			Atividades:               row.Atividades,
			EntradaRegistradaPorNome: row.EntradaRegistradaPorNome,
			SaidaRegistradaPorNome:   database.TextToString(row.SaidaRegistradaPorNome),
		})
	}
	return result, nil
}

// ListarRegistros: GET /api/tecnicos/registros?tecnico_id=&inicio=&fim=&pagina=&limite=
func (h *TecnicoHandler) ListarRegistros(w http.ResponseWriter, r *http.Request) {
	tecnicoID, inicio, fim, err := filtroPeriodoTecnicos(r)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	q := r.URL.Query()
	pagina, _ := strconv.Atoi(q.Get("pagina"))
	if pagina < 1 {
		pagina = 1
	}
	limite, _ := strconv.Atoi(q.Get("limite"))
	if limite < 1 || limite > 100 {
		limite = 30
	}

	total, err := h.db.Queries.ContarRegistrosTecnicos(r.Context(), sqlc.ContarRegistrosTecnicosParams{
		TecnicoID:  tecnicoID,
		DataInicio: inicio,
		DataFim:    fim,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao contar registros")
		return
	}

	itens, err := h.consultarRegistros(r, limite, (pagina-1)*limite)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar registros")
		return
	}

	totalPages := int((total + int64(limite) - 1) / int64(limite))
	response.JSON(w, http.StatusOK, ListarRegistrosTecnicosResponse{
		Itens:      itens,
		Total:      total,
		Pagina:     pagina,
		Limite:     limite,
		TotalPages: totalPages,
	})
}

type AtualizarRegistroTecnicoRequest struct {
	Entrada    string  `json:"entrada"`
	Saida      *string `json:"saida"`
	Observacao string  `json:"observacao"`
}

// AtualizarRegistro: PUT /api/tecnicos/registros/{id} (Admin) — corrige horários
func (h *TecnicoHandler) AtualizarRegistro(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	var req AtualizarRegistroTecnicoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	entrada, err := time.Parse(time.RFC3339, req.Entrada)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "horário de entrada inválido")
		return
	}
	var saida pgtype.Timestamptz
	if req.Saida != nil && *req.Saida != "" {
		t, err := time.Parse(time.RFC3339, *req.Saida)
		if err != nil {
			response.JSONError(w, http.StatusBadRequest, "horário de saída inválido")
			return
		}
		if t.Before(entrada) {
			response.JSONError(w, http.StatusBadRequest, "a saída não pode ser anterior à entrada")
			return
		}
		saida = database.TimeToTimestamptz(t)
	}

	reg, err := h.db.Queries.AtualizarRegistroTecnico(r.Context(), sqlc.AtualizarRegistroTecnicoParams{
		ID:         id,
		Entrada:    database.TimeToTimestamptz(entrada),
		Saida:      saida,
		Observacao: strings.TrimSpace(req.Observacao),
	})
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			response.JSONError(w, http.StatusNotFound, "registro não encontrado")
		case codigoErroPg(err) == "23505":
			response.JSONError(w, http.StatusConflict, "o técnico já tem outra entrada em aberto")
		default:
			response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar registro")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{"id": database.UUIDToString(reg.ID)})
}

// DeletarRegistro: DELETE /api/tecnicos/registros/{id} (Admin)
func (h *TecnicoHandler) DeletarRegistro(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	if _, err := h.db.Queries.BuscarRegistroTecnicoPorID(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusNotFound, "registro não encontrado")
		return
	}
	if err := h.db.Queries.DeletarRegistroTecnico(r.Context(), id); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao excluir registro")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Relatorio: GET /api/tecnicos/relatorio?tecnico_id=&inicio=&fim=
func (h *TecnicoHandler) Relatorio(w http.ResponseWriter, r *http.Request) {
	result, err := h.consultarRelatorio(r)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *TecnicoHandler) consultarRelatorio(r *http.Request) ([]RelatorioTecnicoResponse, error) {
	tecnicoID, inicio, fim, err := filtroPeriodoTecnicos(r)
	if err != nil {
		return nil, err
	}
	rows, err := h.db.Queries.RelatorioTecnicos(r.Context(), sqlc.RelatorioTecnicosParams{
		TecnicoID:  tecnicoID,
		DataInicio: inicio,
		DataFim:    fim,
	})
	if err != nil {
		return nil, errors.New("erro ao gerar relatório")
	}

	result := make([]RelatorioTecnicoResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, RelatorioTecnicoResponse{
			TecnicoID:        database.UUIDToString(row.TecnicoID),
			TecnicoNome:      row.TecnicoNome,
			TecnicoEmpresa:   row.TecnicoEmpresa,
			TotalRegistros:   row.TotalRegistros,
			RegistrosAbertos: row.RegistrosAbertos,
			DiasPresentes:    row.DiasPresentes,
			SegundosTotais:   row.SegundosTotais,
			PrimeiraEntrada:  formatarTimestamptz(row.PrimeiraEntrada),
			UltimaMarcacao:   formatarTimestamptz(row.UltimaMarcacao),
		})
	}
	return result, nil
}

func formatarDuracao(segundos int64) string {
	return fmt.Sprintf("%d:%02d", segundos/3600, (segundos%3600)/60)
}

type AtividadesRequest struct {
	Atividades string `json:"atividades"`
}

// AtualizarAtividades: PATCH /api/tecnicos/registros/{id}/atividades
// Qualquer operador anota o que o técnico fez (horários continuam só com admin).
func (h *TecnicoHandler) AtualizarAtividades(w http.ResponseWriter, r *http.Request) {
	id, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}
	var req AtividadesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if req.Atividades, err = validarTexto(req.Atividades, "o que foi feito", false, maxAtividades); err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	reg, err := h.db.Queries.AtualizarAtividadesRegistro(r.Context(), sqlc.AtualizarAtividadesRegistroParams{
		ID:         id,
		Atividades: req.Atividades,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "registro não encontrado")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao salvar o que foi feito")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"atividades": reg.Atividades})
}

func iniciarCSV(w http.ResponseWriter, prefixo string) *csv.Writer {
	filename := fmt.Sprintf("%s_%s.csv", prefixo, time.Now().In(fusoSaoPaulo).Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	writer := csv.NewWriter(w)
	writer.Comma = ';'
	return writer
}

func dataHoraLocal(s *string) string {
	if s == nil {
		return ""
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return ""
	}
	return t.In(fusoSaoPaulo).Format("02/01/2006 15:04")
}

// ExportarRegistrosCSV: GET /api/tecnicos/registros/exportar.csv?tecnico_id=&inicio=&fim=
func (h *TecnicoHandler) ExportarRegistrosCSV(w http.ResponseWriter, r *http.Request) {
	registros, err := h.consultarRegistros(r, 100000, 0)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writer := iniciarCSV(w, "presenca_tecnicos")
	_ = writer.Write([]string{"Técnico", "Empresa", "Entrada", "Saída", "Duração (h:mm)", "O que foi feito", "Observação", "Entrada marcada por", "Saída marcada por"})
	for _, reg := range registros {
		duracao := ""
		if reg.DuracaoSegundos != nil {
			duracao = formatarDuracao(*reg.DuracaoSegundos)
		}
		saidaPor := ""
		if reg.SaidaRegistradaPorNome != nil {
			saidaPor = *reg.SaidaRegistradaPorNome
		}
		_ = writer.Write([]string{
			celulaCSV(reg.TecnicoNome),
			celulaCSV(reg.TecnicoEmpresa),
			dataHoraLocal(&reg.Entrada),
			dataHoraLocal(reg.Saida),
			duracao,
			celulaCSV(reg.Atividades),
			celulaCSV(reg.Observacao),
			celulaCSV(reg.EntradaRegistradaPorNome),
			celulaCSV(saidaPor),
		})
	}
	writer.Flush()
}

// ExportarRelatorioCSV: GET /api/tecnicos/relatorio/exportar.csv?tecnico_id=&inicio=&fim=
func (h *TecnicoHandler) ExportarRelatorioCSV(w http.ResponseWriter, r *http.Request) {
	linhas, err := h.consultarRelatorio(r)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writer := iniciarCSV(w, "relatorio_tecnicos")
	_ = writer.Write([]string{"Técnico", "Empresa", "Dias presentes", "Registros", "Em aberto", "Horas totais (h:mm)", "Primeira entrada", "Última marcação"})
	for _, l := range linhas {
		_ = writer.Write([]string{
			celulaCSV(l.TecnicoNome),
			celulaCSV(l.TecnicoEmpresa),
			strconv.FormatInt(l.DiasPresentes, 10),
			strconv.FormatInt(l.TotalRegistros, 10),
			strconv.FormatInt(l.RegistrosAbertos, 10),
			formatarDuracao(l.SegundosTotais),
			dataHoraLocal(l.PrimeiraEntrada),
			dataHoraLocal(l.UltimaMarcacao),
		})
	}
	writer.Flush()
}
