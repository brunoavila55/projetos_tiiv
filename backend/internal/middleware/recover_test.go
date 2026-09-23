package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tiiv/backend/internal/middleware"
)

func TestRecoverer_HandlesForcedPanicWithoutCrashing(t *testing.T) {
	// Cria um handler que dispara um panic intencional
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("erro catastrófico simulado para teste de recuperação")
	})

	// Envolve o handler no middleware Recoverer
	protectedHandler := middleware.Recoverer(panickingHandler)

	req := httptest.NewRequest(http.MethodGet, "/rota-com-panic", nil)
	rec := httptest.NewRecorder()

	// Executa a requisição. Se o middleware não recuperar, o teste falha/crasha o processo.
	protectedHandler.ServeHTTP(rec, req)

	// Verifica se respondeu com status 500 Internal Server Error
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado status HTTP 500, obteve %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}

	if body["error"] != "erro interno do servidor" {
		t.Errorf("mensagem de erro esperada 'erro interno do servidor', obteve '%s'", body["error"])
	}
}
