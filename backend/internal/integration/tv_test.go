package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// painelTV faz o GET do modo TV com a chave da tela no cabeçalho
func (e *TestEnv) painelTV(t *testing.T, chave string, cookie *http.Cookie) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.server.URL+"/api/tv/painel", nil)
	if chave != "" {
		req.Header.Set("X-TV-Chave", chave)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/tv/painel falhou: %v", err)
	}
	defer resp.Body.Close()
	var res map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

// Modo TV: só admin cadastra telas; a chave abre o painel sem sessão e para de
// funcionar quando a tela é excluída.
func TestTV_ChaveDaTela(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	opID := env.criarOperador(t, adminCookie, "Op TV", "7373")
	opCookie, _, _ := env.login(t, opID, "7373")

	var telaID string
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM telas_tv WHERE id = $1", telaID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	// Operador não cadastra tela; nome é obrigatório
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tv/telas", opCookie, map[string]any{"nome": "Sala"}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 para operador criar tela, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tv/telas", adminCookie, map[string]any{"nome": "  "}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado 400 para tela sem nome, veio %d", resp.StatusCode)
	}

	resp, res, _ := env.doRequest(http.MethodPost, "/api/tv/telas", adminCookie, map[string]any{"nome": "Telão da sala"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar tela, veio %d", resp.StatusCode)
	}
	telaID = res["id"].(string)
	chave, _ := res["chave"].(string)
	if len(chave) != 64 {
		t.Fatalf("a criação deveria devolver a chave, veio %q", chave)
	}

	// A listagem nunca devolve a chave
	for _, tela := range env.getLista(t, "/api/tv/telas", adminCookie) {
		if _, tem := tela["chave"]; tem {
			t.Error("a listagem de telas não pode trazer a chave")
		}
	}

	// Sem chave e sem sessão: recusa. Chave errada também.
	if st, _ := env.painelTV(t, "", nil); st != http.StatusUnauthorized {
		t.Errorf("esperado 401 sem chave, veio %d", st)
	}
	if st, _ := env.painelTV(t, "chave-errada", nil); st != http.StatusUnauthorized {
		t.Errorf("esperado 401 com chave errada, veio %d", st)
	}

	// Com a chave, sem sessão
	st, painel := env.painelTV(t, chave, nil)
	if st != http.StatusOK {
		t.Fatalf("esperado 200 com a chave, veio %d", st)
	}
	if painel["tela"] != "Telão da sala" {
		t.Errorf("tela esperada 'Telão da sala', veio %v", painel["tela"])
	}
	for _, campo := range []string{"monitores", "tickets", "plantao_hoje", "avisos"} {
		if _, ok := painel[campo].([]any); !ok {
			t.Errorf("campo %s deveria ser uma lista, veio %v", campo, painel[campo])
		}
	}
	for _, m := range painel["monitores"].([]any) {
		if _, tem := m.(map[string]any)["alvo"]; tem {
			t.Error("a TV não deve expor o alvo dos monitores")
		}
	}

	// O acesso fica registrado na tela
	var acessou bool
	_ = env.db.Pool.QueryRow(ctx, "SELECT ultimo_acesso_em IS NOT NULL FROM telas_tv WHERE id = $1", telaID).Scan(&acessou)
	if !acessou {
		t.Error("o acesso da TV deveria ficar registrado")
	}

	// Operador logado também abre o modo TV
	if st, _ := env.painelTV(t, "", opCookie); st != http.StatusOK {
		t.Errorf("esperado 200 para operador logado, veio %d", st)
	}

	// Excluir revoga a chave
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/tv/telas/"+telaID, adminCookie, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperado 204 ao excluir tela, veio %d", resp.StatusCode)
	}
	if st, _ := env.painelTV(t, chave, nil); st != http.StatusUnauthorized {
		t.Errorf("esperado 401 com chave revogada, veio %d", st)
	}
}
