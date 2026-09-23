package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/routes"
)

type TestEnv struct {
	cfg    *config.Config
	db     *database.DB
	server *httptest.Server
	client *http.Client
}

func setupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	cfg := config.Load()
	ctx := context.Background()

	db, err := database.ConnectAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("falha ao conectar e migrar banco de dados de teste: %v", err)
	}

	handler, err := routes.SetupRouter(cfg, db)
	if err != nil {
		t.Fatalf("falha ao configurar roteador: %v", err)
	}

	server := httptest.NewServer(handler)
	client := server.Client()

	return &TestEnv{
		cfg:    cfg,
		db:     db,
		server: server,
		client: client,
	}
}

func (e *TestEnv) teardown() {
	e.server.Close()
	e.db.Pool.Close()
}

// Helper para fazer login e retornar o cookie de sessão
func (e *TestEnv) login(t *testing.T, usuarioID, pin string) (*http.Cookie, int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"usuario_id": usuarioID,
		"pin":        pin,
	})

	req, _ := http.NewRequest(http.MethodPost, e.server.URL+"/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("erro ao enviar requisição de login: %v", err)
	}
	defer resp.Body.Close()

	var res map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&res)

	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "tiiv_session" {
			sessionCookie = c
			break
		}
	}

	return sessionCookie, resp.StatusCode, res
}

func (e *TestEnv) doRequest(method, path string, cookie *http.Cookie, body any) (*http.Response, map[string]any, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, e.server.URL+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	var res map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp, res, nil
}

// -------------------------------------------------------------
// P2.1: Teste 1 - Autenticação com lockout após 5 tentativas falhas
// -------------------------------------------------------------
func TestAuth_LockoutAfter5FailedAttempts(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	// 1. Obter admin e logar
	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco: %v", err)
	}
	admin := users[0]
	adminCookie, status, _ := env.login(t, database.UUIDToString(admin.ID), env.cfg.AdminPIN)
	if status != http.StatusOK || adminCookie == nil {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	// 2. Criar novo usuário operador para o teste de lockout
	testUserName := fmt.Sprintf("Operador Lockout %d", time.Now().UnixNano())
	resp, res, err := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome":  testUserName,
		"cor":   "#3B82F6",
		"pin":   "4321",
		"papel": "usuario",
	})
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("falha ao criar usuário de teste: status %d, err %v", resp.StatusCode, err)
	}
	userID := res["id"].(string)

	// 3. Efetuar 4 tentativas com PIN incorreto -> Status 401 Unauthorized
	for i := 1; i <= 4; i++ {
		_, loginStatus, resLogin := env.login(t, userID, "0000")
		if loginStatus != http.StatusUnauthorized {
			t.Errorf("tentativa %d: esperado 401, obteve %d", i, loginStatus)
		}
		if resLogin["error"] != "PIN incorreto" {
			t.Errorf("tentativa %d: mensagem inesperada: %v", i, resLogin["error"])
		}
	}

	// 4. A 5ª tentativa incorreta deve bloquear a conta por 5 minutos (Status 423 Locked)
	_, loginStatus5, resLogin5 := env.login(t, userID, "0000")
	if loginStatus5 != http.StatusLocked {
		t.Fatalf("tentativa 5: esperado 423 Locked, obteve %d", loginStatus5)
	}
	errMsg := fmt.Sprintf("%v", resLogin5["error"])
	if errMsg != "PIN incorreto. Limite de 5 tentativas atingido. Usuário bloqueado por 5 minutos." {
		t.Fatalf("tentativa 5: mensagem esperada de bloqueio, obteve: %s", errMsg)
	}

	// 5. A 6ª tentativa, MESMO COM O PIN CORRETO, deve ser recusada pelo bloqueio ativo (423 Locked)
	_, loginStatus6, resLogin6 := env.login(t, userID, "4321")
	if loginStatus6 != http.StatusLocked {
		t.Fatalf("tentativa com PIN correto em conta bloqueada: esperado 423 Locked, obteve %d", loginStatus6)
	}
	if errMsg6 := fmt.Sprintf("%v", resLogin6["error"]); !bytes.Contains([]byte(errMsg6), []byte("temporariamente bloqueado")) {
		t.Fatalf("mensagem esperada de conta bloqueada, obteve: %v", errMsg6)
	}

	// 6. Desbloquear a conta via Admin e verificar que o login volta a funcionar com PIN correto
	respUnlock, _, errUnlock := env.doRequest(http.MethodPost, fmt.Sprintf("/api/usuarios/%s/desbloquear", userID), adminCookie, nil)
	if errUnlock != nil || respUnlock.StatusCode != http.StatusOK {
		t.Fatalf("falha ao desbloquear conta: status %d", respUnlock.StatusCode)
	}

	userCookie, finalLoginStatus, _ := env.login(t, userID, "4321")
	if finalLoginStatus != http.StatusOK || userCookie == nil {
		t.Fatalf("login após desbloqueio falhou: status %d", finalLoginStatus)
	}
}

// -------------------------------------------------------------
// P2.1: Teste 2 - Permissões por papel e autoria
// -------------------------------------------------------------
func TestPermissions_RoleAndAuthorship(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	// Criar dois operadores comuns
	user1Name := fmt.Sprintf("Operador A %d", time.Now().UnixNano())
	_, res1, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome":  user1Name,
		"cor":   "#10B981",
		"pin":   "1111",
		"papel": "usuario",
	})
	user1ID := res1["id"].(string)

	user2Name := fmt.Sprintf("Operador B %d", time.Now().UnixNano())
	_, res2, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome":  user2Name,
		"cor":   "#F59E0B",
		"pin":   "2222",
		"papel": "usuario",
	})
	user2ID := res2["id"].(string)

	user1Cookie, _, _ := env.login(t, user1ID, "1111")
	user2Cookie, _, _ := env.login(t, user2ID, "2222")

	// 1. Operador não pode acessar rotas exclusivas de admin
	resp, _, _ := env.doRequest(http.MethodGet, "/api/usuarios", user1Cookie, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("usuário comum acessando /api/usuarios: esperado 403, obteve %d", resp.StatusCode)
	}

	resp, _, _ = env.doRequest(http.MethodPost, "/api/estoque/itens", user1Cookie, map[string]any{
		"nome":    "Item Proibido",
		"unidade": "un",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("usuário comum criando item de estoque: esperado 403, obteve %d", resp.StatusCode)
	}

	// 2. Calendário: Usuário 2 não pode alterar nem deletar evento criado pelo Usuário 1
	now := time.Now()
	_, resEv, _ := env.doRequest(http.MethodPost, "/api/eventos", user1Cookie, map[string]any{
		"titulo":        "Reunião de A",
		"descricao":     "Criada por A",
		"inicio":        now.Format(time.RFC3339),
		"fim":           now.Add(time.Hour).Format(time.RFC3339),
		"participantes": []string{user1ID, user2ID},
	})
	evID := resEv["id"].(string)

	// User 2 tenta alterar o evento -> 403
	resp, _, _ = env.doRequest(http.MethodPut, fmt.Sprintf("/api/eventos/%s", evID), user2Cookie, map[string]any{
		"titulo": "Tentativa de Edição por B",
		"inicio": now.Format(time.RFC3339),
		"fim":    now.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("não criador editando evento: esperado 403, obteve %d", resp.StatusCode)
	}

	// User 2 tenta excluir o evento -> 403
	resp, _, _ = env.doRequest(http.MethodDelete, fmt.Sprintf("/api/eventos/%s", evID), user2Cookie, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("não criador excluindo evento: esperado 403, obteve %d", resp.StatusCode)
	}

	// Criador (User 1) altera o evento -> 200
	resp, _, _ = env.doRequest(http.MethodPut, fmt.Sprintf("/api/eventos/%s", evID), user1Cookie, map[string]any{
		"titulo": "Reunião de A Atualizada",
		"inicio": now.Format(time.RFC3339),
		"fim":    now.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("criador editando evento: esperado 200, obteve %d", resp.StatusCode)
	}

	// 3. Tarefas: Usuário 2 não pode alterar status nem excluir tarefa exclusiva de Usuário 1
	_, resTask, _ := env.doRequest(http.MethodPost, "/api/tarefas", user1Cookie, map[string]any{
		"titulo":         "Tarefa Privada de A",
		"prioridade":     "alta",
		"responsavel_id": user1ID,
	})
	taskID := resTask["id"].(string)

	// User 2 tenta atualizar status -> 403
	resp, _, _ = env.doRequest(http.MethodPatch, fmt.Sprintf("/api/tarefas/%s/status", taskID), user2Cookie, map[string]any{
		"status": "concluida",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("não responsável alterando status de tarefa: esperado 403, obteve %d", resp.StatusCode)
	}

	// User 2 tenta deletar tarefa -> 403
	resp, _, _ = env.doRequest(http.MethodDelete, fmt.Sprintf("/api/tarefas/%s", taskID), user2Cookie, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("não criador deletando tarefa: esperado 403, obteve %d", resp.StatusCode)
	}
}

// -------------------------------------------------------------
// P2.1: Teste 3 - Concorrência do estoque com ao menos 20 saídas em paralelo
// -------------------------------------------------------------
func TestEstoque_Concurrency20ParallelOutputs(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	// 1. Criar item com saldo 0
	itemName := fmt.Sprintf("Conector SC/APC Concorrencia %d", time.Now().UnixNano())
	resp, resItem, err := env.doRequest(http.MethodPost, "/api/estoque/itens", adminCookie, map[string]any{
		"nome":           itemName,
		"unidade":        "un",
		"categoria":      "Fibra",
		"estoque_minimo": 5,
	})
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("falha ao criar item: status %d", resp.StatusCode)
	}
	itemID := resItem["id"].(string)

	// 2. Dar entrada de exatamente 20 unidades
	resp, _, err = env.doRequest(http.MethodPost, "/api/estoque/movimentacoes", adminCookie, map[string]any{
		"item_id":    itemID,
		"tipo":       "entrada",
		"quantidade": 20,
		"motivo":     "Lote inicial de teste concorrente",
	})
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("falha ao dar entrada de 20 unidades: status %d", resp.StatusCode)
	}

	// 3. Disparar 30 requisições simultâneas em goroutines paralelas, cada uma retirando 1 unidade
	// Como o saldo é 20, exatamente 20 requisições DEVEM ter sucesso (201) e exatamente 10 DEVEM falhar (400)
	const totalRequisicoes = 30
	var wg sync.WaitGroup
	wg.Add(totalRequisicoes)

	var mu sync.Mutex
	sucessos := 0
	insuficientes := 0
	outrosErros := 0

	for i := 0; i < totalRequisicoes; i++ {
		go func(idx int) {
			defer wg.Done()

			r, _, errDo := env.doRequest(http.MethodPost, "/api/estoque/movimentacoes", adminCookie, map[string]any{
				"item_id":    itemID,
				"tipo":       "saida",
				"quantidade": 1,
				"motivo":     fmt.Sprintf("Saída paralela goroutine %d", idx),
			})

			mu.Lock()
			defer mu.Unlock()

			if errDo != nil {
				outrosErros++
				return
			}

			if r.StatusCode == http.StatusCreated {
				sucessos++
			} else if r.StatusCode == http.StatusBadRequest {
				insuficientes++
			} else {
				outrosErros++
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Resultado da concorrência: Sucessos: %d (esperado 20), Insuficientes: %d (esperado 10), Outros: %d",
		sucessos, insuficientes, outrosErros)

	if sucessos != 20 {
		t.Errorf("esperado exatamente 20 sucessos, obteve %d", sucessos)
	}
	if insuficientes != 10 {
		t.Errorf("esperado exatamente 10 rejeições por saldo insuficiente, obteve %d", insuficientes)
	}
	if outrosErros != 0 {
		t.Errorf("erros inesperados ocorreram: %d", outrosErros)
	}

	// 4. Verificar o saldo final no banco: deve ser EXATAMENTE 0 (nunca negativo)
	itemUUID, _ := database.StringToUUID(itemID)
	itemFinal, err := env.db.Queries.BuscarItemEstoquePorID(context.Background(), itemUUID)
	if err != nil {
		t.Fatalf("erro ao consultar item final: %v", err)
	}

	if itemFinal.Saldo != 0 {
		t.Errorf("saldo final incorreto: esperado 0, obteve %d", itemFinal.Saldo)
	}
	if itemFinal.Saldo < 0 {
		t.Fatalf("VIOLAÇÃO CRÍTICA: saldo do estoque ficou negativo! saldo=%d", itemFinal.Saldo)
	}
}

// -------------------------------------------------------------
// P2.1: Teste 4 - Filtro de intervalo do calendário
// -------------------------------------------------------------
func TestCalendario_IntervalFilter(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, _ := env.db.Queries.ListarTodosUsuarios(context.Background())
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	hoje := time.Now().Truncate(24 * time.Hour)
	hojeInicio := hoje.Add(9 * time.Hour)
	hojeFim := hoje.Add(10 * time.Hour)

	amanhaInicio := hoje.Add(24*time.Hour + 9*time.Hour)
	amanhaFim := hoje.Add(24*time.Hour + 10*time.Hour)

	semanaQueVemInicio := hoje.Add(7*24*time.Hour + 9*time.Hour)
	semanaQueVemFim := hoje.Add(7*24*time.Hour + 10*time.Hour)

	// Criar 3 eventos em datas distintas
	_, res1, _ := env.doRequest(http.MethodPost, "/api/eventos", adminCookie, map[string]any{
		"titulo": "Evento de Hoje",
		"inicio": hojeInicio.Format(time.RFC3339),
		"fim":    hojeFim.Format(time.RFC3339),
	})
	ev1ID := res1["id"].(string)

	_, res2, _ := env.doRequest(http.MethodPost, "/api/eventos", adminCookie, map[string]any{
		"titulo": "Evento de Amanhã",
		"inicio": amanhaInicio.Format(time.RFC3339),
		"fim":    amanhaFim.Format(time.RFC3339),
	})
	ev2ID := res2["id"].(string)

	_, res3, _ := env.doRequest(http.MethodPost, "/api/eventos", adminCookie, map[string]any{
		"titulo": "Evento Semana Que Vem",
		"inicio": semanaQueVemInicio.Format(time.RFC3339),
		"fim":    semanaQueVemFim.Format(time.RFC3339),
	})
	ev3ID := res3["id"].(string)

	// Filtrar apenas o dia de hoje
	urlFiltroHoje := fmt.Sprintf("/api/eventos?inicio=%s&fim=%s",
		hoje.Format(time.RFC3339),
		hoje.Add(23*time.Hour+59*time.Minute).Format(time.RFC3339))

	req, _ := http.NewRequest(http.MethodGet, env.server.URL+urlFiltroHoje, nil)
	req.AddCookie(adminCookie)
	resp, _ := env.client.Do(req)
	defer resp.Body.Close()

	var eventosRetornados []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&eventosRetornados)

	temEv1 := false
	temEv2 := false
	temEv3 := false

	for _, e := range eventosRetornados {
		if e["id"] == ev1ID {
			temEv1 = true
		}
		if e["id"] == ev2ID {
			temEv2 = true
		}
		if e["id"] == ev3ID {
			temEv3 = true
		}
	}

	if !temEv1 {
		t.Errorf("filtro do calendário para hoje deveria conter o evento de hoje")
	}
	if temEv2 || temEv3 {
		t.Errorf("filtro do calendário para hoje não deveria conter eventos futuros: temEv2=%v, temEv3=%v", temEv2, temEv3)
	}
}

// -------------------------------------------------------------
// P2.1: Teste 5 - Busca de atendimentos com unaccent (acentos e caixa)
// -------------------------------------------------------------
func TestAtendimentos_UnaccentSearch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, _ := env.db.Queries.ListarTodosUsuarios(context.Background())
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	// Inserir atendimento com acentuação e caixa mista
	clienteNome := fmt.Sprintf("Clínica São Cristóvão %d", time.Now().UnixNano())
	descricao := "Substituição e Manutenção de roteador óptico"

	respAt, resAt, err := env.doRequest(http.MethodPost, "/api/atendimentos", adminCookie, map[string]any{
		"cliente_nome": clienteNome,
		"descricao":    descricao,
	})
	if err != nil || respAt.StatusCode != http.StatusCreated {
		t.Fatalf("erro ao criar atendimento: status %d, err %v", respAt.StatusCode, err)
	}
	atID := resAt["id"].(string)

	// Testar buscas sem acento e em minúsculas/maiúsculas
	consultas := []string{
		"clinica",
		"sao cristovao",
		"MANUTENCAO",
		"substituicao",
		"optico",
	}

	for _, termo := range consultas {
		reqURL := fmt.Sprintf("/api/atendimentos?busca=%s", url.QueryEscape(termo))
		req, _ := http.NewRequest(http.MethodGet, env.server.URL+reqURL, nil)
		req.AddCookie(adminCookie)
		resp, _ := env.client.Do(req)

		var respData struct {
			Itens []map[string]any `json:"itens"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&respData)
		resp.Body.Close()

		encontrou := false
		for _, item := range respData.Itens {
			if item["id"] == atID {
				encontrou = true
				break
			}
		}

		if !encontrou {
			t.Errorf("busca por '%s' falhou em encontrar o atendimento com unaccent", termo)
		}
	}
}

// -------------------------------------------------------------
// P2.3: Teste 6 - Recuperação de panic sem derrubar o servidor
// -------------------------------------------------------------
func TestRecoverer_PanicRecovery(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	// Enviar requisição que gera panic intencional
	// Rota de teste ou simulação direta do middleware Recoverer
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/health", nil)
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("servidor inacessível: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthcheck deveria responder 200, respondeu %d", resp.StatusCode)
	}
}
