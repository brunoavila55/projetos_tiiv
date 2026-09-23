package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			duration := time.Since(start)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			// Não logar tokens, cookies ou PINs
			attrs := []any{
				"metodo", r.Method,
				"rota", r.URL.Path,
				"status", status,
				"duracao_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			}

			if user, ok := GetAuthUser(r.Context()); ok && user != nil {
				attrs = append(attrs, "usuario_id", user.ID)
			}

			if status >= 500 {
				slog.Error("requisição HTTP", attrs...)
			} else if status >= 400 {
				slog.Warn("requisição HTTP", attrs...)
			} else {
				slog.Info("requisição HTTP", attrs...)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}
