package middleware

import (
	"net/http"
	"strings"
)

// CabecalhosSeguranca envia CSP e demais cabeçalhos de defesa em todas as respostas.
// scriptsInline são as fontes 'sha256-...' dos scripts inline da SPA.
// hsts só deve ser ligado quando o acesso é por HTTPS.
func CabecalhosSeguranca(scriptsInline []string, hsts bool) func(http.Handler) http.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src " + strings.Join(append([]string{"'self'"}, scriptsInline...), " "),
		// Svelte aplica estilos inline (style=, transições)
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' blob: data:",
		"connect-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"base-uri 'none'",
		"form-action 'self'",
	}, "; ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=31536000")
			}
			next.ServeHTTP(w, r)
		})
	}
}
