package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tiiv/backend/embeds"
)

func TestSPAHandlerFallback(t *testing.T) {
	spa, err := embeds.SPAHandler()
	if err != nil {
		t.Fatalf("falha ao inicializar SPAHandler: %v", err)
	}

	routes := []string{"/", "/estoque", "/calendario", "/tarefas", "/qualquer-rota-spa"}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rec := httptest.NewRecorder()

		spa.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("para rota %s esperava status 200, recebeu %d", route, rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "html") {
			t.Errorf("para rota %s esperava conteúdo html, recebeu: %s", route, body[:min(len(body), 50)])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
