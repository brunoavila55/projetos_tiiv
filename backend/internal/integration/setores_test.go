package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Setores: cada setor só enxerga os próprios tickets, avisos, tarefas e
// pessoas; o admin fica preso ao setor e o superadmin troca de setor.
func TestSetores_Isolamento(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	superCookie, _ := env.loginAdmin(t)
	nocOpID := env.criarOperador(t, superCookie, "Op NOC", "8484")
	nocCookie, _, _ := env.login(t, nocOpID, "8484")

	// 1. Superadmin cria o setor e o admin dele
	resp, res, _ := env.doRequest(http.MethodPost, "/api/setores", superCookie, map[string]any{
		"nome": fmt.Sprintf("Agendamento %d", time.Now().UnixNano()),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar setor: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	agID := res["id"].(string)

	resp, res, _ = env.doRequest(http.MethodPost, "/api/usuarios", superCookie, map[string]any{
		"nome": fmt.Sprintf("Admin Ag %d", time.Now().UnixNano()), "cor": "#10B981", "pin": "8585",
		"papel": "admin", "setor_id": agID,
	})
	if resp.StatusCode != http.StatusCreated || res["setor_id"] != agID {
		t.Fatalf("criar admin do setor: status %d (%v)", resp.StatusCode, res)
	}
	agAdminID := res["id"].(string)
	agCookie, _, _ := env.login(t, agAdminID, "8585")

	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tickets WHERE setor_id = $1", agID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM avisos WHERE criado_por = $1", nocOpID)
		for _, id := range []string{nocOpID, agAdminID} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
		_, _ = env.db.Pool.Exec(ctx, "UPDATE sessoes SET setor_id = NULL WHERE setor_id = $1", agID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM setores WHERE id = $1", agID)
	}()

	_, me, _ := env.doRequest(http.MethodGet, "/api/auth/me", agCookie, nil)
	if setor, _ := me["setor"].(map[string]any); setor["id"] != agID {
		t.Fatalf("admin do Agendamento deveria estar no setor dele, veio %v", me["setor"])
	}

	// 2. Com dois setores aceitando pedidos, o ticket público precisa dizer o setor
	ticket := map[string]any{"solicitante_nome": "Recepção", "titulo": "Remarcar consulta", "descricao": "Paciente pediu outra data"}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tickets/publico", nil, ticket); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ticket sem setor: esperado 400, veio %d", resp.StatusCode)
	}
	ticket["setor_id"] = agID
	resp, res, _ = env.doRequest(http.MethodPost, "/api/tickets/publico", nil, ticket)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("ticket para o Agendamento: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	ticketID := res["id"].(string)

	temTicket := func(cookie *http.Cookie) bool {
		for _, tk := range env.getLista(t, "/api/tickets?status=aberto", cookie) {
			if tk["id"] == ticketID {
				return true
			}
		}
		return false
	}
	if !temTicket(agCookie) {
		t.Error("o Agendamento deveria ver o próprio ticket")
	}
	if temTicket(nocCookie) {
		t.Error("o NOC não deveria ver ticket do Agendamento")
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tickets/"+ticketID+"/resgatar", nocCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("NOC resgatando ticket do Agendamento: esperado 404, veio %d", resp.StatusCode)
	}

	// 3. Aviso do NOC não aparece nem é alterável no Agendamento
	resp, res, _ = env.doRequest(http.MethodPost, "/api/avisos", nocCookie, map[string]any{"titulo": "Janela de manutenção", "nivel": "info"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar aviso: esperado 201, veio %d", resp.StatusCode)
	}
	avisoID := res["id"].(string)
	for _, a := range env.getLista(t, "/api/avisos", agCookie) {
		if a["id"] == avisoID {
			t.Error("aviso do NOC apareceu no Agendamento")
		}
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/avisos/"+avisoID, agCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("admin do Agendamento apagando aviso do NOC: esperado 404, veio %d", resp.StatusCode)
	}

	// 4. Tarefa não pode ir para alguém de outro setor
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tarefas", agCookie, map[string]any{
		"titulo": "Ligar para paciente", "prioridade": "media", "responsavel_id": nocOpID,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("tarefa para operador do NOC: esperado 400, veio %d", resp.StatusCode)
	}
	for _, u := range env.getLista(t, "/api/equipe", agCookie) {
		if u["id"] == nocOpID {
			t.Error("operador do NOC apareceu na equipe do Agendamento")
		}
	}

	// 5. Admin do setor só gere o próprio setor e não cria superadmin
	for _, u := range env.getLista(t, "/api/usuarios", agCookie) {
		if u["setor_id"] != agID {
			t.Errorf("admin do Agendamento viu usuário de outro setor: %v", u["nome"])
		}
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/usuarios/"+nocOpID+"/desbloquear", agCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("admin do Agendamento mexendo em operador do NOC: esperado 404, veio %d", resp.StatusCode)
	}
	resp, _, _ = env.doRequest(http.MethodPost, "/api/usuarios", agCookie, map[string]any{
		"nome": "Quer ser super", "cor": "#10B981", "pin": "8686", "papel": "superadmin",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("admin criando superadmin: esperado 400, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/auth/setor", agCookie, map[string]any{"setor_id": agID}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("admin trocando de setor: esperado 403, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/setores", agCookie, map[string]any{"nome": "Outro"}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("admin criando setor: esperado 403, veio %d", resp.StatusCode)
	}

	// 6. Superadmin troca de setor e passa a ver o ticket do Agendamento
	if temTicket(superCookie) {
		t.Error("superadmin no NOC não deveria ver ticket do Agendamento")
	}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/auth/setor", superCookie, map[string]any{"setor_id": agID}); resp.StatusCode != http.StatusOK {
		t.Fatalf("superadmin trocando de setor: esperado 200, veio %d", resp.StatusCode)
	}
	if !temTicket(superCookie) {
		t.Error("superadmin no Agendamento deveria ver o ticket")
	}

	// 7. Setor com gente não pode ser excluído
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/setores/"+agID, superCookie, nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("excluir setor com pessoas: esperado 409, veio %d", resp.StatusCode)
	}
}
