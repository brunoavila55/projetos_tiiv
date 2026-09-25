package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

type UsuarioHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewUsuarioHandler(db *database.DB, cfg *config.Config) *UsuarioHandler {
	return &UsuarioHandler{
		db:  db,
		cfg: cfg,
	}
}

type UsuarioItemResponse struct {
	ID               string  `json:"id"`
	Nome             string  `json:"nome"`
	Cor              string  `json:"cor"`
	Papel            string  `json:"papel"`
	Ativo            bool    `json:"ativo"`
	TentativasFalhas int32   `json:"tentativas_falhas"`
	BloqueadoAte     *string `json:"bloqueado_ate"`
	CriadoEm         string  `json:"criado_em"`
	FotoVersao       *int64  `json:"foto_versao"`
	SetorID          string  `json:"setor_id"`
	SetorNome        string  `json:"setor_nome,omitempty"`
}

// alvoGerenciavel carrega o usuário da URL e confere se quem pede pode geri-lo:
// o superadmin gere todos; o admin só as pessoas do próprio setor, e nunca um
// superadmin.
func (h *UsuarioHandler) alvoGerenciavel(w http.ResponseWriter, r *http.Request) (sqlc.BuscarUsuarioPorIDRow, bool) {
	user, _ := middleware.GetAuthUser(r.Context())
	uID, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return sqlc.BuscarUsuarioPorIDRow{}, false
	}
	alvo, err := h.db.Queries.BuscarUsuarioPorID(r.Context(), uID)
	if err != nil || (!user.EhSuperadmin() && alvo.SetorID != user.Setor) {
		response.JSONError(w, http.StatusNotFound, "usuário não encontrado")
		return sqlc.BuscarUsuarioPorIDRow{}, false
	}
	if !user.EhSuperadmin() && alvo.Papel == "superadmin" {
		response.JSONError(w, http.StatusForbidden, "só um superadmin altera outro superadmin")
		return sqlc.BuscarUsuarioPorIDRow{}, false
	}
	return alvo, true
}

// papelPermitido: admin cria admins e operadores do setor; só o superadmin
// cria outro superadmin
func papelPermitido(user *middleware.AuthUser, papel string) bool {
	switch papel {
	case "usuario", "admin":
		return true
	case "superadmin":
		return user.EhSuperadmin()
	}
	return false
}

// setorEscolhido: o superadmin põe a pessoa em qualquer setor (vazio mantém
// o atual); o admin só no próprio
func (h *UsuarioHandler) setorEscolhido(r *http.Request, user *middleware.AuthUser, pedido string, atual pgtype.UUID) (pgtype.UUID, error) {
	if !user.EhSuperadmin() {
		return user.Setor, nil
	}
	if pedido == "" {
		return atual, nil
	}
	sID, err := database.StringToUUID(pedido)
	if err != nil {
		return pgtype.UUID{}, errors.New("setor inválido")
	}
	if _, err := h.db.Queries.BuscarSetor(r.Context(), sID); err != nil {
		return pgtype.UUID{}, errors.New("setor não encontrado")
	}
	return sID, nil
}

// Listar: GET /api/usuarios (Admin)
func (h *UsuarioHandler) Listar(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	// Superadmin vê todo mundo (com o setor de cada um); admin, o próprio setor
	var filtro pgtype.UUID
	if !user.EhSuperadmin() {
		filtro = user.Setor
	}
	usuarios, err := h.db.Queries.ListarTodosUsuarios(r.Context(), filtro)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao listar usuários")
		return
	}

	result := make([]UsuarioItemResponse, 0, len(usuarios))
	for _, u := range usuarios {
		var bloqueadoAte *string
		if u.BloqueadoAte.Valid {
			str := u.BloqueadoAte.Time.Format("2006-01-02T15:04:05Z07:00")
			bloqueadoAte = &str
		}

		result = append(result, UsuarioItemResponse{
			ID:               database.UUIDToString(u.ID),
			Nome:             u.Nome,
			Cor:              u.Cor,
			Papel:            u.Papel,
			Ativo:            u.Ativo,
			TentativasFalhas: u.TentativasFalhas,
			BloqueadoAte:     bloqueadoAte,
			CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
			FotoVersao:       fotoVersao(u.FotoAtualizadaEm),
			SetorID:          database.UUIDToString(u.SetorID),
			SetorNome:        u.SetorNome,
		})
	}

	response.JSON(w, http.StatusOK, result)
}

type CriarUsuarioRequest struct {
	Nome  string `json:"nome"`
	Cor   string `json:"cor"`
	PIN   string `json:"pin"`
	Papel string `json:"papel"`
	// Só o superadmin escolhe; vazio = o setor que ele está vendo
	SetorID string `json:"setor_id"`
}

// Criar: POST /api/usuarios (Admin)
func (h *UsuarioHandler) Criar(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	var req CriarUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Cor == "" || req.PIN == "" {
		response.JSONError(w, http.StatusBadRequest, "nome, cor e PIN são obrigatórios")
		return
	}

	if !corRegex.MatchString(req.Cor) {
		response.JSONError(w, http.StatusBadRequest, "cor deve estar no formato #RRGGBB")
		return
	}

	if !pinRegex.MatchString(req.PIN) {
		response.JSONError(w, http.StatusBadRequest, "PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	if req.Papel == "" {
		req.Papel = "usuario"
	}
	if !papelPermitido(user, req.Papel) {
		response.JSONError(w, http.StatusBadRequest, "papel inválido")
		return
	}

	setor, err := h.setorEscolhido(r, user, req.SetorID, user.Setor)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	pinHash, err := bcrypt.GenerateFromPassword([]byte(req.PIN), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar PIN")
		return
	}

	u, err := h.db.Queries.CriarUsuario(r.Context(), sqlc.CriarUsuarioParams{
		Nome:    req.Nome,
		Cor:     req.Cor,
		PinHash: string(pinHash),
		Papel:   req.Papel,
		Ativo:   true,
		SetorID: setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}

	response.JSON(w, http.StatusCreated, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
		SetorID:          database.UUIDToString(u.SetorID),
	})
}

type AtualizarUsuarioRequest struct {
	Nome  string `json:"nome"`
	Cor   string `json:"cor"`
	Papel string `json:"papel"`
	Ativo bool   `json:"ativo"`
	// Só o superadmin move a pessoa de setor; vazio mantém
	SetorID string `json:"setor_id"`
}

// Atualizar: PUT /api/usuarios/{id} (Admin)
func (h *UsuarioHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.GetAuthUser(r.Context())
	atual, ok := h.alvoGerenciavel(w, r)
	if !ok {
		return
	}
	uID := atual.ID

	var req AtualizarUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if req.Nome == "" || req.Cor == "" {
		response.JSONError(w, http.StatusBadRequest, "nome e cor são obrigatórios")
		return
	}

	if !corRegex.MatchString(req.Cor) {
		response.JSONError(w, http.StatusBadRequest, "cor deve estar no formato #RRGGBB")
		return
	}

	if req.Papel == "" {
		req.Papel = "usuario"
	}
	if !papelPermitido(user, req.Papel) {
		response.JSONError(w, http.StatusBadRequest, "papel inválido")
		return
	}

	setor, err := h.setorEscolhido(r, user, req.SetorID, atual.SetorID)
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// O sistema precisa de ao menos um superadmin ativo (é quem cria setores)
	if atual.Papel == "superadmin" && atual.Ativo && (req.Papel != "superadmin" || !req.Ativo) {
		total, err := h.db.Queries.ContarSuperadminsAtivos(r.Context())
		if err == nil && total <= 1 {
			response.JSONError(w, http.StatusBadRequest, "não é possível desativar ou rebaixar o único superadmin ativo do sistema")
			return
		}
	}

	u, err := h.db.Queries.AtualizarUsuario(r.Context(), sqlc.AtualizarUsuarioParams{
		ID:      uID,
		Nome:    req.Nome,
		Cor:     req.Cor,
		Papel:   req.Papel,
		Ativo:   req.Ativo,
		SetorID: setor,
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar usuário")
		return
	}

	// Se o usuário foi desativado, encerrar todas as suas sessões imediatamente
	if !req.Ativo {
		_ = h.db.Queries.DeletarSessoesPorUsuario(r.Context(), uID)
	}

	response.JSON(w, http.StatusOK, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
		SetorID:          database.UUIDToString(u.SetorID),
	})
}

type RedefinirPinRequest struct {
	NovoPIN string `json:"novo_pin"`
}

// RedefinirPIN: POST /api/usuarios/{id}/pin (Admin)
func (h *UsuarioHandler) RedefinirPIN(w http.ResponseWriter, r *http.Request) {
	alvo, ok := h.alvoGerenciavel(w, r)
	if !ok {
		return
	}
	uID := alvo.ID

	var req RedefinirPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if !pinRegex.MatchString(req.NovoPIN) {
		response.JSONError(w, http.StatusBadRequest, "PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	pinHash, err := bcrypt.GenerateFromPassword([]byte(req.NovoPIN), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar PIN")
		return
	}

	_, err = h.db.Queries.AtualizarPin(r.Context(), sqlc.AtualizarPinParams{
		ID:      uID,
		PinHash: string(pinHash),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao redefinir PIN")
		return
	}

	// Desbloquear usuário caso estivesse bloqueado
	_ = h.db.Queries.ZerarTentativasFalhas(r.Context(), uID)

	response.JSON(w, http.StatusOK, map[string]string{"message": "PIN redefinido com sucesso"})
}

// Desbloquear: POST /api/usuarios/{id}/desbloquear (Admin)
func (h *UsuarioHandler) Desbloquear(w http.ResponseWriter, r *http.Request) {
	alvo, ok := h.alvoGerenciavel(w, r)
	if !ok {
		return
	}
	uID := alvo.ID

	u, err := h.db.Queries.DesbloquearUsuario(r.Context(), uID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao desbloquear usuário")
		return
	}

	response.JSON(w, http.StatusOK, UsuarioItemResponse{
		ID:               database.UUIDToString(u.ID),
		Nome:             u.Nome,
		Cor:              u.Cor,
		Papel:            u.Papel,
		Ativo:            u.Ativo,
		TentativasFalhas: u.TentativasFalhas,
		CriadoEm:         u.CriadoEm.Time.Format("2006-01-02T15:04:05Z07:00"),
		SetorID:          database.UUIDToString(u.SetorID),
	})
}

type TrocarProprioPinRequest struct {
	PinAtual string `json:"pin_atual"`
	NovoPin  string `json:"novo_pin"`
}

// TrocarProprioPIN: POST /api/auth/trocar-pin (Qualquer usuário autenticado)
func (h *UsuarioHandler) TrocarProprioPIN(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetAuthUser(r.Context())
	if !ok || user == nil {
		response.JSONError(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	var req TrocarProprioPinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.JSONError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}

	if !pinRegex.MatchString(req.PinAtual) {
		response.JSONError(w, http.StatusBadRequest, "PIN atual deve conter exatamente 4 dígitos numéricos")
		return
	}

	if !pinRegex.MatchString(req.NovoPin) {
		response.JSONError(w, http.StatusBadRequest, "Novo PIN deve conter exatamente 4 dígitos numéricos")
		return
	}

	if config.PinTrivial(req.NovoPin) {
		response.JSONError(w, http.StatusBadRequest, "Novo PIN é fácil demais de adivinhar (repetido ou sequência). Escolha outro.")
		return
	}

	if req.NovoPin == req.PinAtual {
		response.JSONError(w, http.StatusBadRequest, "Novo PIN deve ser diferente do atual")
		return
	}

	uID, err := database.StringToUUID(user.ID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "ID de usuário inválido na sessão")
		return
	}
	sessaoID, err := database.StringToUUID(user.SessaoID)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "sessão inválida")
		return
	}

	// Valida o PIN atual com o mesmo limite de tentativas do login
	_, resultado, _, err := conferirPin(r.Context(), h.db.Queries, uID, req.PinAtual)
	if err != nil {
		slog.Error("erro ao conferir PIN atual", "erro", err)
		response.JSONError(w, http.StatusInternalServerError, "erro ao validar PIN atual")
		return
	}
	switch resultado {
	case pinBloqueou, pinJaBloqueado:
		// Quem está chutando o PIN num terminal aberto perde a sessão
		_ = h.db.Queries.DeletarSessoesPorUsuario(r.Context(), uID)
		limparCookieSessao(w, h.cfg.CookieSecure)
		response.JSONError(w, http.StatusLocked, "PIN atual incorreto muitas vezes. A sessão foi encerrada e o usuário bloqueado temporariamente.")
		return
	case pinIncorreto, pinUsuarioInvalido:
		response.JSONError(w, http.StatusUnauthorized, "PIN atual incorreto")
		return
	}

	novoHash, err := bcrypt.GenerateFromPassword([]byte(req.NovoPin), bcrypt.DefaultCost)
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao processar novo PIN")
		return
	}

	err = h.db.Queries.AtualizarProprioPin(r.Context(), sqlc.AtualizarProprioPinParams{
		ID:      uID,
		PinHash: string(novoHash),
	})
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao atualizar PIN")
		return
	}

	// O PIN antigo pode ter vazado: as outras sessões do usuário caem
	if err := h.db.Queries.DeletarOutrasSessoesUsuario(r.Context(), sqlc.DeletarOutrasSessoesUsuarioParams{
		UsuarioID: uID,
		ID:        sessaoID,
	}); err != nil {
		slog.Error("erro ao encerrar outras sessões após troca de PIN", "erro", err)
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "PIN alterado com sucesso"})
}
