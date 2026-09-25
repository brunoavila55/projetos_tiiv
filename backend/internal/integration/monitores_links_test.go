package integration_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// getLista faz um GET autenticado de uma rota que devolve um array JSON
func (e *TestEnv) getLista(t *testing.T, path string, cookie *http.Cookie) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.server.URL+path, nil)
	req.AddCookie(cookie)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("GET %s falhou: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: esperado 200, veio %d", path, resp.StatusCode)
	}
	var lista []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&lista); err != nil {
		t.Fatalf("GET %s: resposta não é uma lista: %v", path, err)
	}
	return lista
}

// Monitor: só admin cadastra; duas falhas seguidas derrubam, abrem queda e
// ticket; a volta fecha a queda.
func TestMonitores_QuedaEVolta(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	opID := env.criarOperador(t, adminCookie, "Op Monitor", "8484")
	opCookie, _, _ := env.login(t, opID, "8484")

	// Alvo HTTP controlado pelo teste
	var fora atomic.Bool
	alvo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fora.Load() {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer alvo.Close()

	var monitorID string
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tickets WHERE id IN (SELECT ticket_id FROM monitor_quedas WHERE monitor_id = $1)", monitorID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM monitores WHERE id = $1", monitorID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	novo := map[string]any{"nome": "Intranet", "tipo": "http", "alvo": alvo.URL, "intervalo_seg": 60, "abrir_ticket": true}

	// Operador não cadastra
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/monitores", opCookie, novo); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 para operador criar monitor, veio %d", resp.StatusCode)
	}

	// Validações
	for _, c := range []map[string]any{
		{"nome": "Sem alvo", "tipo": "http", "alvo": ""},
		{"nome": "Tipo errado", "tipo": "dns", "alvo": "8.8.8.8"},
		{"nome": "TCP sem porta", "tipo": "tcp", "alvo": "10.0.0.1"},
		{"nome": "Intervalo curto", "tipo": "ping", "alvo": "10.0.0.1", "intervalo_seg": 5},
	} {
		if resp, _, _ := env.doRequest(http.MethodPost, "/api/monitores", adminCookie, c); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d", c, resp.StatusCode)
		}
	}

	resp, res, _ := env.doRequest(http.MethodPost, "/api/monitores", adminCookie, novo)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar monitor, veio %d", resp.StatusCode)
	}
	monitorID = res["id"].(string)
	if res["status"] != "pendente" {
		t.Errorf("monitor novo deveria nascer pendente, veio %v", res["status"])
	}

	verificar := func(esperado string) map[string]any {
		t.Helper()
		resp, res, _ := env.doRequest(http.MethodPost, "/api/monitores/"+monitorID+"/verificar", opCookie, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 ao verificar, veio %d", resp.StatusCode)
		}
		if res["status"] != esperado {
			t.Fatalf("esperado status %s, veio %v (erro: %v)", esperado, res["status"], res["ultimo_erro"])
		}
		return res
	}

	if res := verificar("online"); res["latencia_ms"] == nil {
		t.Error("verificação online deveria trazer latência")
	}

	// Uma falha só não derruba; a segunda sim
	fora.Store(true)
	if res := verificar("online"); res["ultimo_erro"] != "respondeu HTTP 502" {
		t.Errorf("erro esperado 'respondeu HTTP 502', veio %v", res["ultimo_erro"])
	}
	verificar("offline")
	verificar("offline") // continua fora sem abrir outra queda

	quedas := env.getLista(t, "/api/monitores/"+monitorID+"/quedas", opCookie)
	if len(quedas) != 1 || quedas[0]["fim"] != nil || quedas[0]["ticket_numero"] == nil {
		t.Fatalf("esperada 1 queda aberta com ticket, veio %v", quedas)
	}

	_, painel, _ := env.doRequest(http.MethodGet, "/api/painel", opCookie, nil)
	resumo := painel["monitores"].(map[string]any)
	achou := false
	for _, f := range resumo["fora"].([]any) {
		if f.(map[string]any)["id"] == monitorID {
			achou = true
		}
	}
	if !achou {
		t.Error("monitor fora do ar não apareceu no painel")
	}

	// Volta
	fora.Store(false)
	verificar("online")
	quedas = env.getLista(t, "/api/monitores/"+monitorID+"/quedas", opCookie)
	if len(quedas) != 1 || quedas[0]["fim"] == nil {
		t.Errorf("queda deveria estar fechada, veio %v", quedas)
	}

	// Trocar o alvo zera o estado; desligar impede verificação
	porta, _ := net.Listen("tcp", "127.0.0.1:0")
	defer porta.Close()
	editado := map[string]any{"nome": "Intranet TCP", "tipo": "tcp", "alvo": porta.Addr().String(), "intervalo_seg": 120, "ativo": false}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/monitores/"+monitorID, adminCookie, editado); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperado 204 ao editar, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/monitores/"+monitorID+"/verificar", opCookie, nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 ao verificar monitor desligado, veio %d", resp.StatusCode)
	}
	for _, m := range env.getLista(t, "/api/monitores", opCookie) {
		if m["id"] == monitorID && (m["status"] != "pendente" || m["ativo"] != false) {
			t.Errorf("após editar: esperado pendente e desligado, veio %v", m)
		}
	}
	editado["ativo"] = true
	env.doRequest(http.MethodPut, "/api/monitores/"+monitorID, adminCookie, editado)
	verificar("online")

	// Operador não exclui; admin sim
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/monitores/"+monitorID, opCookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 para operador excluir, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/monitores/"+monitorID, adminCookie, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 para admin excluir, veio %d", resp.StatusCode)
	}
}

// Links: qualquer operador cadastra; só http(s); autor ou admin alteram.
func TestLinks_CadastroEPermissoes(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	autorID := env.criarOperador(t, adminCookie, "Op Links A", "8585")
	outroID := env.criarOperador(t, adminCookie, "Op Links B", "8686")
	autorCookie, _, _ := env.login(t, autorID, "8585")
	outroCookie, _, _ := env.login(t, outroID, "8686")

	defer func() {
		for _, id := range []string{autorID, outroID} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM links WHERE criado_por = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()

	for _, c := range []map[string]any{
		{"titulo": "", "url": "https://zabbix.local"},
		{"titulo": "Script", "url": "javascript:alert(1)"},
		{"titulo": "Sem esquema", "url": "zabbix.local"},
		{"titulo": "Arquivo", "url": "file:///etc/passwd"},
	} {
		if resp, _, _ := env.doRequest(http.MethodPost, "/api/links", autorCookie, c); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d", c, resp.StatusCode)
		}
	}

	resp, res, _ := env.doRequest(http.MethodPost, "/api/links", autorCookie, map[string]any{
		"titulo": " Zabbix ", "url": "https://zabbix.local/", "descricao": "Monitoramento", "categoria": "Monitoramento",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar link, veio %d", resp.StatusCode)
	}
	linkID := res["id"].(string)

	acharLink := func(cookie *http.Cookie) map[string]any {
		for _, l := range env.getLista(t, "/api/links", cookie) {
			if l["id"] == linkID {
				return l
			}
		}
		return nil
	}
	if l := acharLink(outroCookie); l == nil || l["titulo"] != "Zabbix" || l["pode_editar"] != false {
		t.Errorf("para outro operador: esperado link visível, sem edição, veio %v", l)
	}

	editado := map[string]any{"titulo": "Zabbix NOC", "url": "http://10.0.0.5/zabbix"}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/links/"+linkID, outroCookie, editado); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 na edição por outro operador, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/links/"+linkID, autorCookie, editado); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 na edição pelo autor, veio %d", resp.StatusCode)
	}
	if l := acharLink(autorCookie); l == nil || l["url"] != "http://10.0.0.5/zabbix" || l["categoria"] != "" {
		t.Errorf("edição não refletiu: %v", l)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/links/"+linkID, adminCookie, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 na exclusão pelo admin, veio %d", resp.StatusCode)
	}
}
