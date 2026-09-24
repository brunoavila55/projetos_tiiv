package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
)

// loginAdmin loga com o admin ativo do banco de teste
func (e *TestEnv) loginAdmin(t *testing.T) (*http.Cookie, string) {
	t.Helper()
	users, err := e.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	for _, u := range users {
		if u.Papel == "admin" && u.Ativo {
			id := database.UUIDToString(u.ID)
			cookie, status, _ := e.login(t, id, e.cfg.AdminPIN)
			if status == http.StatusOK {
				return cookie, id
			}
		}
	}
	t.Fatal("nenhum admin ativo aceitou ADMIN_PIN")
	return nil, ""
}

// criarOperador cria um usuário comum com o PIN informado e devolve o ID
func (e *TestEnv) criarOperador(t *testing.T, adminCookie *http.Cookie, prefixo, pin string) string {
	t.Helper()
	resp, res, err := e.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome":  fmt.Sprintf("%s %d", prefixo, time.Now().UnixNano()),
		"cor":   "#3B82F6",
		"pin":   pin,
		"papel": "usuario",
	})
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("falha ao criar operador: status %d, err %v", resp.StatusCode, err)
	}
	return res["id"].(string)
}

// loginDe faz login a partir de outro endereço de loopback (outro "terminal")
func (e *TestEnv) loginDe(t *testing.T, ipOrigem, usuarioID, pin string, cabecalhos map[string]string) int {
	t.Helper()
	cliente := &http.Client{Transport: &http.Transport{
		DialContext: (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(ipOrigem)}}).DialContext,
	}}
	body, _ := json.Marshal(map[string]string{"usuario_id": usuarioID, "pin": pin})
	req, _ := http.NewRequest(http.MethodPost, e.server.URL+"/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	resp, err := cliente.Do(req)
	if err != nil {
		t.Fatalf("erro no login a partir de %s: %v", ipOrigem, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// F14: logins errados simultâneos não passam juntos pela checagem de bloqueio
// (os que chegam com a conta bloqueada recebem 423, ou 429 do limite global)
func TestSeguranca_LoginConcorrenteRespeitaBloqueio(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	adminCookie, _ := env.loginAdmin(t)
	userID := env.criarOperador(t, adminCookie, "Operador Corrida", "4817")

	const total = 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	comparados, bloqueados := 0, 0
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, status, res := env.login(t, userID, fmt.Sprintf("%04d", 5000+i))
			msg := fmt.Sprintf("%v", res["error"])
			mu.Lock()
			defer mu.Unlock()
			// "PIN incorreto..." só aparece quando o bcrypt rodou
			if strings.HasPrefix(msg, "PIN incorreto") {
				comparados++
			} else if status == http.StatusLocked || status == http.StatusTooManyRequests {
				bloqueados++
			} else {
				t.Errorf("resposta inesperada: %d %s", status, msg)
			}
		}(i)
	}
	wg.Wait()

	if comparados != 5 {
		t.Errorf("esperava exatamente 5 PINs comparados antes do bloqueio, obteve %d", comparados)
	}
	if comparados+bloqueados != total {
		t.Errorf("respostas contabilizadas: %d de %d", comparados+bloqueados, total)
	}
}

// F15/F16: o limite é por IP real (X-Forwarded-For ignorado) e um terminal
// abusivo é barrado antes de conseguir bloquear a conta
func TestSeguranca_LimitePorIP(t *testing.T) {
	env := setupTestEnvCom(t, func(c *config.Config) { c.LoginFalhasPorIP = 4 })
	defer env.teardown()

	adminCookie, _ := env.loginAdmin(t)
	userID := env.criarOperador(t, adminCookie, "Operador IP", "4817")

	for i := 1; i <= 4; i++ {
		xff := map[string]string{"X-Forwarded-For": fmt.Sprintf("10.9.9.%d", i), "X-Real-IP": fmt.Sprintf("10.8.8.%d", i)}
		if st := env.loginDe(t, "127.0.0.2", userID, "0000", xff); st != http.StatusUnauthorized {
			t.Fatalf("falha %d: esperado 401, obteve %d", i, st)
		}
	}

	// Trocar os cabeçalhos não adianta: o IP TCP é o mesmo
	xff := map[string]string{"X-Forwarded-For": "10.9.9.99", "True-Client-IP": "10.7.7.7"}
	if st := env.loginDe(t, "127.0.0.2", userID, "0000", xff); st != http.StatusTooManyRequests {
		t.Fatalf("5ª falha do mesmo IP: esperado 429, obteve %d", st)
	}

	// A conta não foi bloqueada: o operador entra de outro terminal
	if st := env.loginDe(t, "127.0.0.3", userID, "4817", nil); st != http.StatusOK {
		t.Fatalf("login legítimo de outro IP: esperado 200, obteve %d", st)
	}
}

// F15: tickets públicos com X-Forwarded-For variado continuam limitados
func TestSeguranca_TicketsIgnoramXForwardedFor(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	cliente := &http.Client{Transport: &http.Transport{
		DialContext: (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.4")}}).DialContext,
	}}
	ultimo := 0
	for i := 0; i < 6; i++ {
		body, _ := json.Marshal(map[string]string{
			"solicitante_nome": "Teste XFF", "titulo": "Spam", "descricao": "x", "prioridade": "baixa",
		})
		req, _ := http.NewRequest(http.MethodPost, env.server.URL+"/api/tickets/publico", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.1.1.%d", i))
		resp, err := cliente.Do(req)
		if err != nil {
			t.Fatalf("erro ao abrir ticket: %v", err)
		}
		resp.Body.Close()
		ultimo = resp.StatusCode
	}
	if ultimo != http.StatusTooManyRequests {
		t.Fatalf("6º ticket com XFF variado: esperado 429, obteve %d", ultimo)
	}
}

// F17: troca do próprio PIN conta tentativas e derruba as outras sessões
func TestSeguranca_TrocarProprioPin(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	adminCookie, _ := env.loginAdmin(t)
	userID := env.criarOperador(t, adminCookie, "Operador Troca", "4817")

	sessaoA, _, _ := env.login(t, userID, "4817")
	sessaoB, _, _ := env.login(t, userID, "4817")

	trocar := func(cookie *http.Cookie, atual, novo string) (int, string) {
		resp, res, err := env.doRequest(http.MethodPost, "/api/auth/trocar-pin", cookie, map[string]string{"pin_atual": atual, "novo_pin": novo})
		if err != nil {
			t.Fatalf("erro ao trocar PIN: %v", err)
		}
		return resp.StatusCode, fmt.Sprintf("%v", res["error"])
	}

	if st, _ := trocar(sessaoA, "4817", "1234"); st != http.StatusBadRequest {
		t.Errorf("novo PIN trivial: esperado 400, obteve %d", st)
	}
	for i := 1; i <= 4; i++ {
		if st, _ := trocar(sessaoA, "0000", "5930"); st != http.StatusUnauthorized {
			t.Fatalf("PIN atual errado %d: esperado 401, obteve %d", i, st)
		}
	}

	// Troca correta zera as tentativas e derruba a sessão B, mantendo a A
	if st, msg := trocar(sessaoA, "4817", "5930"); st != http.StatusOK {
		t.Fatalf("troca correta: esperado 200, obteve %d (%s)", st, msg)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/auth/me", sessaoB, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("sessão B após troca: esperado 401, obteve %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/auth/me", sessaoA, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("sessão A após troca: esperado 200, obteve %d", resp.StatusCode)
	}

	// Força bruta pela rota: a 5ª falha bloqueia e encerra a sessão
	for i := 1; i <= 4; i++ {
		if st, _ := trocar(sessaoA, "0000", "6284"); st != http.StatusUnauthorized {
			t.Fatalf("força bruta %d: esperado 401, obteve %d", i, st)
		}
	}
	if st, _ := trocar(sessaoA, "0000", "6284"); st != http.StatusLocked {
		t.Fatalf("5ª falha: esperado 423, obteve %d", st)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/auth/me", sessaoA, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("sessão após bloqueio: esperado 401, obteve %d", resp.StatusCode)
	}
}

// F02: usuário marcado para trocar o PIN só acessa /me e /trocar-pin
func TestSeguranca_TrocaDePinObrigatoria(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	adminCookie, _ := env.loginAdmin(t)
	userID := env.criarOperador(t, adminCookie, "Operador Primeiro Acesso", "4817")
	uID, _ := database.StringToUUID(userID)
	if err := env.db.Queries.MarcarTrocaPinObrigatoria(context.Background(), uID); err != nil {
		t.Fatalf("falha ao marcar troca obrigatória: %v", err)
	}

	cookie, status, res := env.login(t, userID, "4817")
	if status != http.StatusOK || res["deve_trocar_pin"] != true {
		t.Fatalf("login: esperado 200 com deve_trocar_pin, obteve %d %v", status, res)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tarefas", cookie, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("rota comum antes da troca: esperado 403, obteve %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/auth/me", cookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("/me antes da troca: esperado 200, obteve %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/auth/trocar-pin", cookie, map[string]string{"pin_atual": "4817", "novo_pin": "5930"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("troca: esperado 200, obteve %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tarefas", cookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("rota comum após a troca: esperado 200, obteve %d", resp.StatusCode)
	}
}

// F08: foto de usuário desativado não é pública, mas o admin ainda a vê
func TestSeguranca_FotoDeUsuarioInativo(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	adminCookie, _ := env.loginAdmin(t)
	userID := env.criarOperador(t, adminCookie, "Operador Inativo", "4817")

	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	req, _ := http.NewRequest(http.MethodPut, env.server.URL+"/api/usuarios/"+userID+"/foto", bytes.NewReader(png))
	req.AddCookie(adminCookie)
	if resp, err := env.client.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha ao enviar foto: %v", err)
	}

	resp, _, _ := env.doRequest(http.MethodPut, "/api/usuarios/"+userID, adminCookie, map[string]any{
		"nome": "Operador Inativo", "cor": "#3B82F6", "papel": "usuario", "ativo": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("falha ao desativar: status %d", resp.StatusCode)
	}

	url := "/api/auth/usuarios/" + userID + "/foto"
	if resp, _, _ := env.doRequest(http.MethodGet, url, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("foto de inativo sem sessão: esperado 404, obteve %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodGet, url, adminCookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("foto de inativo para admin: esperado 200, obteve %d", resp.StatusCode)
	}
}

// F09/F13/F10/F11: validações menores, CSV e cabeçalhos
func TestSeguranca_ValidacoesECabecalhos(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	adminCookie, adminID := env.loginAdmin(t)

	// Cor fora de #RRGGBB
	resp, _, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome": "Cor ruim", "cor": "red;x", "pin": "4817", "papel": "usuario",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("cor inválida: esperado 400, obteve %d", resp.StatusCode)
	}

	// Comentário apagado pela rota de outra tarefa
	novaTarefa := func() string {
		_, res, _ := env.doRequest(http.MethodPost, "/api/tarefas", adminCookie, map[string]any{
			"titulo": "Tarefa segurança", "prioridade": "baixa", "status": "pendente", "responsaveis": []string{adminID},
		})
		return res["id"].(string)
	}
	t1, t2 := novaTarefa(), novaTarefa()
	_, com, _ := env.doRequest(http.MethodPost, "/api/tarefas/"+t1+"/comentarios", adminCookie, map[string]string{"conteudo": "oi"})
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/tarefas/"+t2+"/comentarios/"+com["id"].(string), adminCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("comentário via outra tarefa: esperado 404, obteve %d", resp.StatusCode)
	}

	// Fórmula no motivo sai neutralizada no CSV
	_, item, _ := env.doRequest(http.MethodPost, "/api/estoque/itens", adminCookie, map[string]any{
		"nome": fmt.Sprintf("Item CSV %d", time.Now().UnixNano()), "unidade": "un", "categoria": "Teste", "estoque_minimo": 0,
	})
	itemID, _ := item["id"].(string)
	resp, _, _ = env.doRequest(http.MethodPost, "/api/estoque/movimentacoes", adminCookie, map[string]any{
		"item_id": itemID, "tipo": "entrada", "quantidade": 1, "motivo": "=1+1",
	})
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("falha ao registrar movimentação: status %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/estoque/movimentacoes/exportar.csv?item_id="+itemID, nil)
	req.AddCookie(adminCookie)
	respCSV, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(respCSV.Body)
	respCSV.Body.Close()
	if !strings.Contains(buf.String(), ";'=1+1;") {
		t.Errorf("motivo com fórmula não neutralizado no CSV: %s", buf.String())
	}

	// Cabeçalhos de segurança na API e na SPA; /api/health não expõe erro do banco
	for _, rota := range []string{"/api/health", "/"} {
		resp, err := env.client.Get(env.server.URL + rota)
		if err != nil {
			t.Fatalf("erro em %s: %v", rota, err)
		}
		resp.Body.Close()
		csp := resp.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "'sha256-") {
			t.Errorf("%s: CSP ausente ou sem hash do script inline: %q", rota, csp)
		}
		if resp.Header.Get("X-Frame-Options") != "DENY" || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
			resp.Header.Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: cabeçalhos de segurança ausentes: %v", rota, resp.Header)
		}
	}
}
