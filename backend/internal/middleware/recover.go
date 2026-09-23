package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"tiiv/backend/internal/response"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				stack := string(debug.Stack())
				slog.Error("panic recuperado no servidor",
					"erro", fmt.Sprintf("%v", rvr),
					"rota", r.URL.Path,
					"metodo", r.Method,
					"stack", stack,
				)

				response.JSONError(w, http.StatusInternalServerError, "erro interno do servidor")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
