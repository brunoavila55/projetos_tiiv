package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/database/sqlc"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

// O navegador reduz a foto antes de enviar (~50 KB); o limite só barra abusos.
const fotoTamanhoMaximo = 2 << 20

var fotoTiposPermitidos = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// fotoVersao converte a data da foto em um número usado para invalidar cache
// no frontend (?v=). Retorna nil quando o usuário não tem foto.
func fotoVersao(t pgtype.Timestamptz) *int64 {
	if !t.Valid {
		return nil
	}
	v := t.Time.UnixMilli()
	return &v
}

// buscarFotoVersao consulta a versão da foto de um único usuário.
func buscarFotoVersao(ctx context.Context, q *sqlc.Queries, id pgtype.UUID) *int64 {
	t, err := q.ObterVersaoFotoUsuario(ctx, id)
	if err != nil {
		return nil
	}
	return fotoVersao(t)
}

// ObterFoto: GET /api/auth/usuarios/{id}/foto (pública, usada na tela de login)
func (h *AuthHandler) ObterFoto(w http.ResponseWriter, r *http.Request) {
	uID, err := database.StringToUUID(chi.URLParam(r, "id"))
	if err != nil {
		response.JSONError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	foto, err := h.db.Queries.ObterFotoUsuario(r.Context(), uID)
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(w, http.StatusNotFound, "usuário sem foto")
		return
	}
	if err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao carregar foto")
		return
	}

	// Ex-operador: a foto (dado pessoal) só aparece para admin logado
	if !foto.UsuarioAtivo {
		if user, ok := middleware.GetAuthUser(r.Context()); !ok || !user.EhAdmin() {
			response.JSONError(w, http.StatusNotFound, "usuário sem foto")
			return
		}
	}

	w.Header().Set("Content-Type", foto.Mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch {
	case !foto.UsuarioAtivo:
		// Visto por admin: não pode ficar em cache compartilhado
		w.Header().Set("Cache-Control", "private, no-store")
	case r.URL.Query().Get("v") != "":
		// A URL muda a cada nova foto, então pode ficar em cache para sempre.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, "", foto.AtualizadoEm.Time, bytes.NewReader(foto.Conteudo))
}

type FotoResponse struct {
	FotoVersao *int64 `json:"foto_versao"`
}

// EnviarFoto: PUT /api/usuarios/{id}/foto (Admin). Corpo: a imagem crua.
func (h *UsuarioHandler) EnviarFoto(w http.ResponseWriter, r *http.Request) {
	alvo, ok := h.alvoGerenciavel(w, r)
	if !ok {
		return
	}
	uID := alvo.ID

	conteudo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, fotoTamanhoMaximo))
	if err != nil {
		response.JSONError(w, http.StatusRequestEntityTooLarge, "foto muito grande (máximo 2 MB)")
		return
	}
	if len(conteudo) == 0 {
		response.JSONError(w, http.StatusBadRequest, "nenhuma imagem enviada")
		return
	}

	// O tipo vem do conteúdo, não do cabeçalho enviado pelo cliente.
	mime := http.DetectContentType(conteudo)
	if !fotoTiposPermitidos[mime] {
		response.JSONError(w, http.StatusBadRequest, "formato não suportado. Use JPEG, PNG ou WebP")
		return
	}

	atualizadoEm, err := h.db.Queries.SalvarFotoUsuario(r.Context(), sqlc.SalvarFotoUsuarioParams{
		UsuarioID: uID,
		Conteudo:  conteudo,
		Mime:      mime,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			response.JSONError(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		response.JSONError(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}

	response.JSON(w, http.StatusOK, FotoResponse{FotoVersao: fotoVersao(atualizadoEm)})
}

// RemoverFoto: DELETE /api/usuarios/{id}/foto (Admin)
func (h *UsuarioHandler) RemoverFoto(w http.ResponseWriter, r *http.Request) {
	alvo, ok := h.alvoGerenciavel(w, r)
	if !ok {
		return
	}
	uID := alvo.ID

	if err := h.db.Queries.RemoverFotoUsuario(r.Context(), uID); err != nil {
		response.JSONError(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
