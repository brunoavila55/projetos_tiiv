package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"tiiv/backend/internal/database"
	"tiiv/backend/internal/response"
)

type HealthHandler struct {
	db *database.DB
}

func NewHealthHandler(db *database.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Pool.Ping(ctx); err != nil {
		// O erro do pgx traz host, porta e usuário do banco; fica só no log
		slog.Error("health: banco indisponível", "erro", err)
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "error",
			"db":     "disconnected",
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"db":     "connected",
	})
}
