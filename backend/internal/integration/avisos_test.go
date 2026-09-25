package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Mural de avisos: qualquer operador publica; só autor ou admin alteram;
// avisos vencidos somem do painel.
func TestAvisos_MuralNoPainel(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	autorID := env.criarOperador(t, adminCookie, "Op Avisos A", "8282")
	outroID := env.criarOperador(t, adminCookie, "Op Avisos B", "8383")
	autorCookie, _, _ := env.login(t, autorID, "8282")
	outroCookie, _, _ := env.login(t, outroID, "8383")

	defer func() {
		for _, id := range []string{autorID, outroID} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM avisos WHERE criado_por = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()

	avisoNoPainel := func(cookie *http.Cookie, id string) map[string]any {
		t.Helper()
		resp, res, err := env.doRequest(http.MethodGet, "/api/painel", cookie, nil)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/painel falhou: %v", err)
		}
		for _, item := range res["avisos"].([]any) {
			a := item.(map[string]any)
			if a["id"] == id {
				return a
			}
		}
		return nil
	}

	// Validações
	amanha := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	ontem := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	casos := []map[string]any{
		{"titulo": "  ", "nivel": "info"},
		{"titulo": "Nível errado", "nivel": "urgente"},
		{"titulo": "Já vencido", "nivel": "info", "expira_em": ontem},
		{"titulo": "Data torta", "nivel": "info", "expira_em": "amanhã"},
	}
	for _, c := range casos {
		resp, _, _ := env.doRequest(http.MethodPost, "/api/avisos", autorCookie, c)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d", c, resp.StatusCode)
		}
	}

	// Criação e visibilidade para toda a equipe
	resp, res, _ := env.doRequest(http.MethodPost, "/api/avisos", autorCookie, map[string]any{
		"titulo": "Link da operadora instável", "mensagem": "Abrir chamado se cair", "nivel": "critico", "expira_em": amanha,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar aviso, veio %d", resp.StatusCode)
	}
	avisoID := res["id"].(string)

	if a := avisoNoPainel(outroCookie, avisoID); a == nil {
		t.Fatal("aviso não apareceu no painel de outro operador")
	} else if a["pode_editar"] != false || a["nivel"] != "critico" {
		t.Errorf("para outro operador: esperado pode_editar=false e nível crítico, veio %v", a)
	}
	if a := avisoNoPainel(autorCookie, avisoID); a == nil || a["pode_editar"] != true {
		t.Errorf("autor deveria poder editar o próprio aviso: %v", a)
	}

	// Outro operador não altera nem exclui
	editado := map[string]any{"titulo": "Link normalizado", "nivel": "info"}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/avisos/"+avisoID, outroCookie, editado); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 na edição por outro operador, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/avisos/"+avisoID, outroCookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 na exclusão por outro operador, veio %d", resp.StatusCode)
	}

	// Autor edita (sem expiração = fica até excluir)
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/avisos/"+avisoID, autorCookie, editado); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperado 204 na edição pelo autor, veio %d", resp.StatusCode)
	}
	if a := avisoNoPainel(autorCookie, avisoID); a == nil || a["titulo"] != "Link normalizado" || a["expira_em"] != nil {
		t.Errorf("edição não refletiu no painel: %v", a)
	}

	// Aviso vencido some do painel
	if _, err := env.db.Pool.Exec(ctx, "UPDATE avisos SET expira_em = now() - interval '1 minute' WHERE id = $1", avisoID); err != nil {
		t.Fatalf("falha ao vencer aviso: %v", err)
	}
	if a := avisoNoPainel(autorCookie, avisoID); a != nil {
		t.Error("aviso vencido ainda aparece no painel")
	}

	// Admin exclui aviso de qualquer um
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/avisos/"+avisoID, adminCookie, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 na exclusão pelo admin, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/avisos/"+avisoID, adminCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("esperado 404 ao excluir aviso inexistente, veio %d", resp.StatusCode)
	}
}
