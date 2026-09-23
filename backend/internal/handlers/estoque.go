package handlers

import (
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

type EstoqueHandler struct {
	db *database.DB
}

func NewEstoqueHandler(db *database.DB) *EstoqueHandler {
	return &EstoqueHandler{db: db}
}

type ItemEstoqueResponse struct {
	ID             string `json:"id"`
	Nome           string `json:"nome"`
	Unidade        string `json:"unidade"`
	Categoria      string `json:"categoria"`
	EstoqueMinimo  int32  `json:"estoque_minimo"`
	Saldo          int32  `json:"saldo"`
	Ativo          bool   `json:"ativo"`
	CriadoEm       string `json:"criado_em"`
	AbaixoDoMinimo bool   `json:"abaixo_do_minimo"`
}

// ListarItens: GET /api/estoque/itens?busca=&categoria=&ativo=
func (h *EstoqueHandler) ListarItens(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	busca := q.Get("busca")
	categoria := q.Get("categoria")
	ativoStr := q.Get("ativo")

	buscaParam := database.StringToText(busca)
	categoriaParam := database.StringToText(categoria)

	var ativoParam *bool
	if ativoStr != "" {
		b, err := strconv.ParseBool(ativoStr)
		if err == nil {
			ativoParam = &b
		}
	}

	itens, err := h.db.Queries.ListarItensEstoque(r.Context(), sqlc.ListarItensEstoqueParams{
		Busca:     buscaParam,
		Categoria: categoriaParam,
		Ativo:     database.BoolToPgtypeBool(ativoParam),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar itens de estoque")
		return
	}

	result := make([]ItemEstoqueResponse, 0, len(itens))
	for _, it := range itens {
		result = append(result, ItemEstoqueResponse{
			ID:             database.UUIDToString(it.ID),
			Nome:           it.Nome,
			Unidade:        it.Unidade,
			Categoria:      it.Categoria,
			EstoqueMinimo:  it.EstoqueMinimo,
			Saldo:          it.Saldo,
			Ativo:          it.Ativo,
			CriadoEm:       it.CriadoEm.Time.Format(time.RFC3339),
			AbaixoDoMinimo: it.AbaixoDoMinimo,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

// ListarCategorias: GET /api/estoque/categorias
func (h *EstoqueHandler) ListarCategorias(w http.ResponseWriter, r *http.Request) {
	categorias, err := h.db.Queries.ListarCategoriasEstoque(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar categorias")
		return
	}
	response.JSON(w, http.StatusOK, categorias)
}

// ObterItem: GET /api/estoque/itens/{id}
func (h *EstoqueHandler) ObterItem(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	itID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	it, err := h.db.Queries.BuscarItemEstoquePorID(r.Context(), itID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "item de estoque não encontrado")
		return
	}

	response.JSON(w, http.StatusOK, ItemEstoqueResponse{
		ID:             database.UUIDToString(it.ID),
		Nome:           it.Nome,
		Unidade:        it.Unidade,
		Categoria:      it.Categoria,
		EstoqueMinimo:  it.EstoqueMinimo,
		Saldo:          it.Saldo,
		Ativo:          it.Ativo,
		CriadoEm:       it.CriadoEm.Time.Format(time.RFC3339),
		AbaixoDoMinimo: it.Saldo < it.EstoqueMinimo,
	})
}

type CriarItemEstoqueRequest struct {
	Nome          string `json:"nome"`
	Unidade       string `json:"unidade"`
	Categoria     string `json:"categoria"`
	EstoqueMinimo int32  `json:"estoque_minimo"`
}

// CriarItem: POST /api/estoque/itens (Admin)
func (h *EstoqueHandler) CriarItem(w http.ResponseWriter, r *http.Request) {
	var req CriarItemEstoqueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Unidade == "" {
		response.JSONError(w, http.StatusBadRequest, "nome e unidade são obrigatórios")
		return
	}

	if req.Categoria == "" {
		req.Categoria = "Geral"
	}
	if req.EstoqueMinimo < 0 {
		req.EstoqueMinimo = 0
	}

	it, err := h.db.Queries.CriarItemEstoque(r.Context(), sqlc.CriarItemEstoqueParams{
		Nome:          req.Nome,
		Unidade:       req.Unidade,
		Categoria:     req.Categoria,
		EstoqueMinimo: req.EstoqueMinimo,
	})
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "erro ao criar item (já pode existir item com este nome)")
		return
	}

	response.JSON(w, http.StatusCreated, ItemEstoqueResponse{
		ID:             database.UUIDToString(it.ID),
		Nome:           it.Nome,
		Unidade:        it.Unidade,
		Categoria:      it.Categoria,
		EstoqueMinimo:  it.EstoqueMinimo,
		Saldo:          it.Saldo,
		Ativo:          it.Ativo,
		CriadoEm:       it.CriadoEm.Time.Format(time.RFC3339),
		AbaixoDoMinimo: it.Saldo < it.EstoqueMinimo,
	})
}

type AtualizarItemEstoqueRequest struct {
	Nome          string `json:"nome"`
	Unidade       string `json:"unidade"`
	Categoria     string `json:"categoria"`
	EstoqueMinimo int32  `json:"estoque_minimo"`
	Ativo         bool   `json:"ativo"`
}

// AtualizarItem: PUT /api/estoque/itens/{id} (Admin)
func (h *EstoqueHandler) AtualizarItem(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	itID, err := database.StringToUUID(idStr)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	var req AtualizarItemEstoqueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Unidade == "" {
		response.JSONError(w, http.StatusBadRequest, "nome e unidade são obrigatórios")
		return
	}

	if req.Categoria == "" {
		req.Categoria = "Geral"
	}
	if req.EstoqueMinimo < 0 {
		req.EstoqueMinimo = 0
	}

	it, err := h.db.Queries.AtualizarItemEstoque(r.Context(), sqlc.AtualizarItemEstoqueParams{
		ID:            itID,
		Nome:          req.Nome,
		Unidade:       req.Unidade,
		Categoria:     req.Categoria,
		EstoqueMinimo: req.EstoqueMinimo,
		Ativo:         req.Ativo,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar item")
		return
	}

	response.JSON(w, http.StatusOK, ItemEstoqueResponse{
		ID:             database.UUIDToString(it.ID),
		Nome:           it.Nome,
		Unidade:        it.Unidade,
		Categoria:      it.Categoria,
		EstoqueMinimo:  it.EstoqueMinimo,
		Saldo:          it.Saldo,
		Ativo:          it.Ativo,
		CriadoEm:       it.CriadoEm.Time.Format(time.RFC3339),
		AbaixoDoMinimo: it.Saldo < it.EstoqueMinimo,
	})
}

type MovimentacaoRequest struct {
	ItemID     string `json:"item_id"`
	Tipo       string `json:"tipo"` // "entrada", "saida", "ajuste"
	Quantidade int32  `json:"quantidade"`
	Motivo     string `json:"motivo"`
}

type MovimentacaoResponse struct {
	ID              string `json:"id"`
	ItemID          string `json:"item_id"`
	ItemNome        string `json:"item_nome"`
	ItemUnidade     string `json:"item_unidade"`
	Tipo            string `json:"tipo"`
	Quantidade      int32  `json:"quantidade"`
	SaldoResultante int32  `json:"saldo_resultante"`
	Motivo          string `json:"motivo"`
	UsuarioID       string `json:"usuario_id"`
	UsuarioNome     string `json:"usuario_nome"`
	CriadoEm        string `json:"criado_em"`
}

// RegistrarMovimentacao: POST /api/estoque/movimentacoes
// Executa em transação estrita com SELECT ... FOR UPDATE para concorrência segura
func (h *EstoqueHandler) RegistrarMovimentacao(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req MovimentacaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.ItemID == "" {
		response.JSONError(w, http.StatusBadRequest, "item_id é obrigatório")
		return
	}

	if req.Tipo != "entrada" && req.Tipo != "saida" && req.Tipo != "ajuste" {
		response.JSONError(w, http.StatusBadRequest, "tipo de movimentação inválido. Use entrada, saida ou ajuste")
		return
	}

	if req.Tipo == "entrada" && req.Quantidade <= 0 {
		response.JSONError(w, http.StatusBadRequest, "quantidade de entrada deve ser maior que zero")
		return
	}

	if req.Tipo == "saida" && req.Quantidade <= 0 {
		response.JSONError(w, http.StatusBadRequest, "quantidade de saída deve ser maior que zero")
		return
	}

	if req.Tipo == "ajuste" && req.Quantidade < 0 {
		response.JSONError(w, http.StatusBadRequest, "saldo no ajuste não pode ser negativo")
		return
	}

	// Motivo é obrigatório para saída e ajuste
	if (req.Tipo == "saida" || req.Tipo == "ajuste") && req.Motivo == "" {
		response.JSONError(w, http.StatusBadRequest, fmt.Sprintf("motivo é obrigatório para movimentações do tipo %s", req.Tipo))
		return
	}

	itUUID, err := database.StringToUUID(req.ItemID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID do item inválido")
		return
	}

	uUUID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao identificar usuário")
		return
	}

	// Iniciar Transação com SELECT ... FOR UPDATE
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao iniciar transação")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.db.Queries.WithTx(tx)

	// Bloqueio pessimista do item
	item, err := qtx.BloquearItemEstoqueParaAtualizacao(r.Context(), itUUID)
	if err != nil {
		response.JSONError(w, http.StatusNotFound, "item de estoque não encontrado")
		return
	}

	if !item.Ativo {
		response.JSONError(w, http.StatusBadRequest, "não é possível movimentar um item desativado")
		return
	}

	var novoSaldo int32
	var qtdRegistrada int32

	switch req.Tipo {
	case "entrada":
		novoSaldo = item.Saldo + req.Quantidade
		qtdRegistrada = req.Quantidade
	case "saida":
		if req.Quantidade > item.Saldo {
			response.JSONError(w, http.StatusBadRequest, fmt.Sprintf("saldo insuficiente: item possui %d %s e foi solicitada saída de %d %s", item.Saldo, item.Unidade, req.Quantidade, item.Unidade))
			return
		}
		novoSaldo = item.Saldo - req.Quantidade
		qtdRegistrada = req.Quantidade
	case "ajuste":
		novoSaldo = req.Quantidade
		qtdRegistrada = novoSaldo - item.Saldo
	}

	// Atualizar saldo do item
	_, err = qtx.AtualizarSaldoItemEstoque(r.Context(), sqlc.AtualizarSaldoItemEstoqueParams{
		ID:    item.ID,
		Saldo: novoSaldo,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar saldo do item")
		return
	}

	// Inserir registro imutável da movimentação
	mov, err := qtx.CriarMovimentacaoEstoque(r.Context(), sqlc.CriarMovimentacaoEstoqueParams{
		ItemID:          item.ID,
		Tipo:            req.Tipo,
		Quantidade:      qtdRegistrada,
		SaldoResultante: novoSaldo,
		Motivo:          req.Motivo,
		UsuarioID:       uUUID,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao registrar movimentação")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao confirmar transação")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"id":               database.UUIDToString(mov.ID),
		"saldo_anterior":   item.Saldo,
		"saldo_resultante": novoSaldo,
		"message":          "movimentação realizada com sucesso",
	})
}

// ListarMovimentacoes: GET /api/estoque/movimentacoes?item_id=&usuario_id=&tipo=&inicio=&fim=&pagina=&limite=
func (h *EstoqueHandler) ListarMovimentacoes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	itemIDStr := q.Get("item_id")
	usuarioIDStr := q.Get("usuario_id")
	tipoStr := q.Get("tipo")
	inicioStr := q.Get("inicio")
	fimStr := q.Get("fim")

	pagina, _ := strconv.Atoi(q.Get("pagina"))
	if pagina < 1 {
		pagina = 1
	}
	limite, _ := strconv.Atoi(q.Get("limite"))
	if limite < 1 || limite > 100 {
		limite = 30
	}
	offset := (pagina - 1) * limite

	var itemID, usuarioID pgtype.UUID
	if itemIDStr != "" {
		itemID, _ = database.StringToUUID(itemIDStr)
	}
	if usuarioIDStr != "" {
		usuarioID, _ = database.StringToUUID(usuarioIDStr)
	}

	tipoParam := database.StringToText(tipoStr)

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
			dataFim = database.TimeToTimestamptz(t.Add(23*time.Hour + 59*time.Minute))
		}
	}

	movs, err := h.db.Queries.ListarMovimentacoesEstoque(r.Context(), sqlc.ListarMovimentacoesEstoqueParams{
		Limit:      int32(limite),
		Offset:     int32(offset),
		ItemID:     itemID,
		UsuarioID:  usuarioID,
		Tipo:       tipoParam,
		DataInicio: dataInicio,
		DataFim:    dataFim,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar histórico de movimentações")
		return
	}

	result := make([]MovimentacaoResponse, 0, len(movs))
	for _, m := range movs {
		result = append(result, MovimentacaoResponse{
			ID:              database.UUIDToString(m.ID),
			ItemID:          database.UUIDToString(m.ItemID),
			ItemNome:        m.ItemNome,
			ItemUnidade:     m.ItemUnidade,
			Tipo:            m.Tipo,
			Quantidade:      m.Quantidade,
			SaldoResultante: m.SaldoResultante,
			Motivo:          m.Motivo,
			UsuarioID:       database.UUIDToString(m.UsuarioID),
			UsuarioNome:     m.UsuarioNome,
			CriadoEm:        m.CriadoEm.Time.Format(time.RFC3339),
		})
	}

	response.JSON(w, http.StatusOK, result)
}
