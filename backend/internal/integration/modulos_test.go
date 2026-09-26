package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
)

// Módulos por setor: o superadmin desliga funcionalidades de um setor; as
// rotas delas passam a responder 403 para quem trabalha nele.
func TestModulos_PorSetor(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	superCookie, _ := env.loginAdmin(t)

	// 1. Setor nasce com o estoque desligado
	resp, res, _ := env.doRequest(http.MethodPost, "/api/setores", superCookie, map[string]any{
		"nome":                fmt.Sprintf("Suporte %d", time.Now().UnixNano()),
		"modulos_desativados": []string{"estoque"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar setor: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	setorID := res["id"].(string)
	nomeSetor := res["nome"].(string)

	resp, res, _ = env.doRequest(http.MethodPost, "/api/usuarios", superCookie, map[string]any{
		"nome": fmt.Sprintf("Admin Sup %d", time.Now().UnixNano()), "cor": "#10B981", "pin": "8787",
		"papel": "admin", "setor_id": setorID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar admin do setor: status %d (%v)", resp.StatusCode, res)
	}
	adminID := res["id"].(string)
	cookie, _, _ := env.login(t, adminID, "8787")

	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", adminID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", adminID)
		_, _ = env.db.Pool.Exec(ctx, "UPDATE sessoes SET setor_id = NULL WHERE setor_id = $1", setorID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM setores WHERE id = $1", setorID)
	}()

	modulosDoMe := func() []any {
		_, me, _ := env.doRequest(http.MethodGet, "/api/auth/me", cookie, nil)
		setor, _ := me["setor"].(map[string]any)
		lista, _ := setor["modulos"].([]any)
		return lista
	}
	if m := modulosDoMe(); slices.Contains(m, any("estoque")) || !slices.Contains(m, any("tarefas")) {
		t.Errorf("/me deveria listar tarefas e não estoque, veio %v", m)
	}

	// 2. Rotas do módulo desligado recusam; as demais seguem normais
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/estoque/itens", cookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("estoque desligado: esperado 403, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tarefas", cookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("tarefas ligadas: esperado 200, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/estoque/itens", superCookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("estoque no NOC continua ligado: esperado 200, veio %d", resp.StatusCode)
	}

	// 3. Validação: módulo inexistente e dependência (tickets precisa de tarefas)
	atualizar := func(desativados []string) (int, map[string]any) {
		resp, res, _ := env.doRequest(http.MethodPut, "/api/setores/"+setorID, superCookie, map[string]any{
			"nome": nomeSetor, "modulos_desativados": desativados,
		})
		return resp.StatusCode, res
	}
	if st, _ := atualizar([]string{"foguete"}); st != http.StatusBadRequest {
		t.Errorf("módulo inexistente: esperado 400, veio %d", st)
	}
	if st, _ := atualizar([]string{"tarefas"}); st != http.StatusBadRequest {
		t.Errorf("tarefas desligadas com tickets ligados: esperado 400, veio %d", st)
	}

	// 4. Desligar tickets: some da tela de acesso e o pedido público é recusado
	if st, res := atualizar([]string{"tickets", "tarefas", "estoque"}); st != http.StatusOK {
		t.Fatalf("desligar tickets e tarefas: esperado 200, veio %d (%v)", st, res)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tickets", cookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("tickets desligados: esperado 403, veio %d", resp.StatusCode)
	}
	for _, s := range env.getLista(t, "/api/setores/publico", superCookie) {
		if s["id"] == setorID && s["tickets"] != false {
			t.Errorf("setor sem tickets não deveria receber tickets pela tela de acesso: %v", s)
		}
	}
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tickets/publico", nil, map[string]any{
		"solicitante_nome": "Recepção", "titulo": "Teste", "descricao": "Teste", "setor_id": setorID,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ticket público para setor sem tickets: esperado 400, veio %d", resp.StatusCode)
	}

	// 5. O painel continua abrindo, só sem os blocos desligados
	resp, res, _ = env.doRequest(http.MethodGet, "/api/painel", cookie, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("painel: esperado 200, veio %d", resp.StatusCode)
	}
	if lista, _ := res["tarefas_pendentes"].([]any); len(lista) != 0 {
		t.Errorf("painel sem o módulo de tarefas não deveria listar tarefas: %v", lista)
	}

	// 6. A TV do setor mostra só os módulos ligados; sem o módulo TV, recusa
	resp, res, _ = env.doRequest(http.MethodGet, "/api/tv/painel", cookie, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TV: esperado 200, veio %d", resp.StatusCode)
	}
	if m, _ := res["modulos"].([]any); slices.Contains(m, any("tickets")) {
		t.Errorf("TV não deveria listar tickets entre os módulos: %v", m)
	}
	if st, res := atualizar([]string{"tv"}); st != http.StatusOK {
		t.Fatalf("desligar TV: esperado 200, veio %d (%v)", st, res)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tv/painel", cookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("TV desligada: esperado 403, veio %d", resp.StatusCode)
	}
	if m := modulosDoMe(); !slices.Contains(m, any("estoque")) || slices.Contains(m, any("tv")) {
		t.Errorf("depois de religar tudo menos a TV, /me veio %v", m)
	}
}
