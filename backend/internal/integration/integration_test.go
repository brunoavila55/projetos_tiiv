package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	return setupTestEnvCom(t, nil)
}

// setupTestEnvCom permite ajustar a configuração antes de montar o roteador
func setupTestEnvCom(t *testing.T, ajustar func(*config.Config)) *TestEnv {
	t.Helper()

	cfg := config.Load()
	if cfg.DatabaseURL == "" || cfg.AdminPIN == "" {
		t.Fatal("defina DATABASE_URL e ADMIN_PIN (o PIN do admin do banco de teste); `make test` lê do .env")
	}
	// Todos os testes saem de 127.0.0.1: o limite por IP fica alto, exceto
	// no teste que trata dele
	cfg.LoginFalhasPorIP = 1000
	if ajustar != nil {
		ajustar(cfg)
	}
	ctx := context.Background()

	db, err := database.ConnectAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("falha ao conectar e migrar banco de dados de teste: %v", err)
	}

	// Num banco novo o admin inicial nasce com troca de PIN obrigatória
	if _, err := db.Pool.Exec(ctx, `UPDATE usuarios SET deve_trocar_pin = false WHERE papel = 'admin'`); err != nil {
		t.Fatalf("falha ao preparar admin de teste: %v", err)
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
	if errMsg != "PIN incorreto. Limite de 5 tentativas atingido. Usuário bloqueado por 5 minuto(s)." {
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

// -------------------------------------------------------------
// P3.1: Teste 7 - Atualização e persistência de Tema
// -------------------------------------------------------------
func TestAuth_TemaUpdate(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	adminCookie, status, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	// 1. Atualizar para escuro
	resp, res, err := env.doRequest(http.MethodPut, "/api/auth/tema", adminCookie, map[string]string{
		"tema": "escuro",
	})
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha ao atualizar tema para escuro: status %d, err %v", resp.StatusCode, err)
	}
	if res["tema"] != "escuro" {
		t.Errorf("esperado tema 'escuro', obteve '%v'", res["tema"])
	}

	// 2. Verificar persistência em /api/auth/me
	reqMe, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/auth/me", nil)
	reqMe.AddCookie(adminCookie)
	respMe, err := env.client.Do(reqMe)
	if err != nil || respMe.StatusCode != http.StatusOK {
		t.Fatalf("falha ao consultar /me: status %d", respMe.StatusCode)
	}
	var meData map[string]any
	_ = json.NewDecoder(respMe.Body).Decode(&meData)
	respMe.Body.Close()

	if meData["tema"] != "escuro" {
		t.Errorf("esperado tema 'escuro' no perfil, obteve '%v'", meData["tema"])
	}

	// 3. Atualizar para valor inválido deve retornar 400
	respInv, _, _ := env.doRequest(http.MethodPut, "/api/auth/tema", adminCookie, map[string]string{
		"tema": "azul-neon",
	})
	if respInv.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado status 400 para tema inválido, obteve %d", respInv.StatusCode)
	}
}

// -------------------------------------------------------------
// P3.2: Teste 8 - Exportação CSV com UTF-8 BOM e delimitador ;
// -------------------------------------------------------------
func TestCSV_Exports(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	adminCookie, status, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	// Exportar Movimentações Estoque CSV
	reqEst, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/estoque/movimentacoes/exportar.csv", nil)
	reqEst.AddCookie(adminCookie)
	respEst, err := env.client.Do(reqEst)
	if err != nil || respEst.StatusCode != http.StatusOK {
		t.Fatalf("falha ao exportar estoque CSV: status %d, err %v", respEst.StatusCode, err)
	}
	defer respEst.Body.Close()

	bufEst := new(bytes.Buffer)
	_, _ = bufEst.ReadFrom(respEst.Body)
	bytesEst := bufEst.Bytes()

	if len(bytesEst) < 3 || bytesEst[0] != 0xEF || bytesEst[1] != 0xBB || bytesEst[2] != 0xBF {
		t.Errorf("exportação de estoque não possui UTF-8 BOM no início")
	}

	conteudoEst := string(bytesEst[3:])
	if !bytes.Contains([]byte(conteudoEst), []byte("Data/Hora;Material;Unidade;Tipo;Quantidade;Saldo Resultante;Motivo;Operador")) {
		t.Errorf("cabeçalhos com ';' esperados não encontrados em estoque CSV: %s", conteudoEst[:min(len(conteudoEst), 100)])
	}
}

// -------------------------------------------------------------
// P3.3: Teste 9 - Comentários em Tarefas
// -------------------------------------------------------------
func TestTarefas_Comentarios(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	adminCookie, status, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	// 1. Criar tarefa
	_, resT, err := env.doRequest(http.MethodPost, "/api/tarefas", adminCookie, map[string]any{
		"titulo":       "Tarefa para teste de comentários",
		"prioridade":   "alta",
		"status":       "pendente",
		"responsaveis": []string{database.UUIDToString(users[0].ID)},
	})
	if err != nil {
		t.Fatalf("falha ao criar tarefa: %v", err)
	}
	tarefaID := resT["id"].(string)

	// 2. Adicionar comentário
	respCom, resCom, err := env.doRequest(http.MethodPost, fmt.Sprintf("/api/tarefas/%s/comentarios", tarefaID), adminCookie, map[string]string{
		"conteudo": "Este é um comentário de teste no chamado",
	})
	if err != nil || respCom.StatusCode != http.StatusCreated {
		t.Fatalf("falha ao criar comentário: status %d, err %v", respCom.StatusCode, err)
	}
	comID := resCom["id"].(string)

	// 3. Listar comentários
	reqList, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/tarefas/%s/comentarios", env.server.URL, tarefaID), nil)
	reqList.AddCookie(adminCookie)
	respList, err := env.client.Do(reqList)
	if err != nil || respList.StatusCode != http.StatusOK {
		t.Fatalf("falha ao listar comentários: status %d", respList.StatusCode)
	}
	var comList []map[string]any
	_ = json.NewDecoder(respList.Body).Decode(&comList)
	respList.Body.Close()

	if len(comList) != 1 {
		t.Fatalf("esperava 1 comentário, obteve %d", len(comList))
	}
	if comList[0]["conteudo"] != "Este é um comentário de teste no chamado" {
		t.Errorf("conteúdo inesperado: %v", comList[0]["conteudo"])
	}

	// 4. Deletar comentário
	respDel, _, err := env.doRequest(http.MethodDelete, fmt.Sprintf("/api/tarefas/%s/comentarios/%s", tarefaID, comID), adminCookie, nil)
	if err != nil || respDel.StatusCode != http.StatusOK {
		t.Fatalf("falha ao deletar comentário: status %d", respDel.StatusCode)
	}

	// 5. Verificar lista vazia
	reqList2, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/tarefas/%s/comentarios", env.server.URL, tarefaID), nil)
	reqList2.AddCookie(adminCookie)
	respList2, _ := env.client.Do(reqList2)
	var comList2 []map[string]any
	_ = json.NewDecoder(respList2.Body).Decode(&comList2)
	respList2.Body.Close()

	if len(comList2) != 0 {
		t.Errorf("esperava 0 comentários após deleção, obteve %d", len(comList2))
	}
}

// -------------------------------------------------------------
// P3.4: Teste 10 - Eventos Recorrentes no Calendário
// -------------------------------------------------------------
func TestEventos_Recurrence(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	adminCookie, status, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	now := time.Now().Truncate(time.Hour)
	inicio := now.Format(time.RFC3339)
	fim := now.Add(1 * time.Hour).Format(time.RFC3339)

	// Criar evento semanal
	_, resEv, err := env.doRequest(http.MethodPost, "/api/eventos", adminCookie, map[string]any{
		"titulo":      "Reunião Semanal TIIV",
		"descricao":   "Alinhamento de rotina recorrente",
		"inicio":      inicio,
		"fim":         fim,
		"dia_inteiro": false,
		"recorrencia": "semanal",
	})
	if err != nil {
		t.Fatalf("falha ao criar evento recorrente: %v", err)
	}
	eventoID := resEv["id"].(string)

	// Listar eventos abrangendo 3 semanas (deve retornar 3 ou 4 ocorrências)
	janelaInicio := now.AddDate(0, 0, -1).Format(time.RFC3339)
	janelaFim := now.AddDate(0, 0, 25).Format(time.RFC3339)

	reqList, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/eventos?inicio=%s&fim=%s", env.server.URL, url.QueryEscape(janelaInicio), url.QueryEscape(janelaFim)), nil)
	reqList.AddCookie(adminCookie)
	respList, err := env.client.Do(reqList)
	if err != nil || respList.StatusCode != http.StatusOK {
		t.Fatalf("falha ao listar eventos com recorrência: status %d", respList.StatusCode)
	}

	var eventosRetornados []map[string]any
	_ = json.NewDecoder(respList.Body).Decode(&eventosRetornados)
	respList.Body.Close()

	ocorrencias := 0
	for _, ev := range eventosRetornados {
		if ev["original_id"] == eventoID || ev["id"] == eventoID {
			ocorrencias++
		}
	}

	if ocorrencias < 3 {
		t.Errorf("esperava pelo menos 3 ocorrências expandidas do evento semanal, encontrou %d", ocorrencias)
	}
}

// -------------------------------------------------------------
// Fotos de perfil: envio, validação de tipo, leitura pública e remoção
// -------------------------------------------------------------
func TestUsuarios_Foto(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	var adminID string
	for _, u := range users {
		if u.Papel == "admin" && u.Ativo {
			adminID = database.UUIDToString(u.ID)
			break
		}
	}
	adminCookie, status, _ := env.login(t, adminID, env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	enviar := func(conteudo []byte, cookie *http.Cookie) (*http.Response, map[string]any) {
		req, _ := http.NewRequest(http.MethodPut, env.server.URL+"/api/usuarios/"+adminID+"/foto", bytes.NewReader(conteudo))
		req.Header.Set("Content-Type", "application/octet-stream")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("erro ao enviar foto: %v", err)
		}
		defer resp.Body.Close()
		var res map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&res)
		return resp, res
	}

	// PNG 1x1 válido
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}

	// 1. Sem sessão não pode enviar
	if resp, _ := enviar(png, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("esperado 401 sem sessão, obteve %d", resp.StatusCode)
	}

	// 2. Conteúdo que não é imagem (ex.: SVG/HTML) é recusado
	if resp, _ := enviar([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), adminCookie); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado 400 para conteúdo não suportado, obteve %d", resp.StatusCode)
	}

	// 3. PNG válido é aceito e devolve a versão
	resp, res := enviar(png, adminCookie)
	if resp.StatusCode != http.StatusOK || res["foto_versao"] == nil {
		t.Fatalf("falha ao enviar foto: status %d, resposta %v", resp.StatusCode, res)
	}

	// 4. A lista pública informa a versão da foto
	respLista, err := env.client.Get(env.server.URL + "/api/auth/usuarios")
	if err != nil {
		t.Fatalf("erro ao listar usuários: %v", err)
	}
	var lista []map[string]any
	_ = json.NewDecoder(respLista.Body).Decode(&lista)
	respLista.Body.Close()
	encontrou := false
	for _, u := range lista {
		if u["id"] == adminID && u["foto_versao"] != nil {
			encontrou = true
		}
	}
	if !encontrou {
		t.Errorf("foto_versao ausente na lista pública para o admin")
	}

	// 5. Leitura pública devolve a imagem com o tipo detectado
	respFoto, err := env.client.Get(fmt.Sprintf("%s/api/auth/usuarios/%s/foto?v=1", env.server.URL, adminID))
	if err != nil {
		t.Fatalf("erro ao buscar foto: %v", err)
	}
	var corpo bytes.Buffer
	_, _ = corpo.ReadFrom(respFoto.Body)
	respFoto.Body.Close()
	if respFoto.StatusCode != http.StatusOK || respFoto.Header.Get("Content-Type") != "image/png" || !bytes.Equal(corpo.Bytes(), png) {
		t.Errorf("foto inválida: status %d, tipo %q, %d bytes", respFoto.StatusCode, respFoto.Header.Get("Content-Type"), corpo.Len())
	}
	if respFoto.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("esperado cabeçalho nosniff na foto")
	}

	// 6. Remover e confirmar 404
	respDel, _, _ := env.doRequest(http.MethodDelete, "/api/usuarios/"+adminID+"/foto", adminCookie, nil)
	if respDel.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 ao remover foto, obteve %d", respDel.StatusCode)
	}
	respFoto2, _ := env.client.Get(env.server.URL + "/api/auth/usuarios/" + adminID + "/foto")
	respFoto2.Body.Close()
	if respFoto2.StatusCode != http.StatusNotFound {
		t.Errorf("esperado 404 após remover foto, obteve %d", respFoto2.StatusCode)
	}
}

// -------------------------------------------------------------
// Técnicos: cadastro, entrada/saída e relatório
// -------------------------------------------------------------
func TestTecnicos_EntradaSaidaRelatorio(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("usuários não encontrados: %v", err)
	}
	cookie, status, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)
	if status != http.StatusOK {
		t.Fatalf("falha ao logar como admin: status %d", status)
	}

	nome := fmt.Sprintf("Técnico Teste %d", time.Now().UnixNano())
	resp, res, _ := env.doRequest(http.MethodPost, "/api/tecnicos", cookie, map[string]string{"nome": nome, "empresa": "ACME"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao cadastrar técnico, obteve %d: %v", resp.StatusCode, res)
	}
	tecID := res["id"].(string)
	// defer (e não t.Cleanup) para rodar antes do teardown fechar o pool
	defer func() {
		ctx := context.Background()
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnico_registros WHERE tecnico_id = $1", tecID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnicos WHERE id = $1", tecID)
	}()

	// Nome duplicado (sem diferenciar maiúsculas) é recusado
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tecnicos", cookie, map[string]string{"nome": strings.ToUpper(nome)})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para nome duplicado, obteve %d", resp.StatusCode)
	}

	// Saída sem entrada aberta
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", cookie, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para saída sem entrada, obteve %d", resp.StatusCode)
	}

	entrada := time.Now().Add(-2*time.Hour - 30*time.Minute).Truncate(time.Second)
	resp, res, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/entrada", cookie, map[string]string{"horario": entrada.Format(time.RFC3339)})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 na entrada, obteve %d: %v", resp.StatusCode, res)
	}

	// Segunda entrada com a primeira em aberto
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/entrada", cookie, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para entrada duplicada, obteve %d", resp.StatusCode)
	}

	// Saída anterior à entrada
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", cookie, map[string]string{"horario": entrada.Add(-time.Hour).Format(time.RFC3339)})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado 400 para saída antes da entrada, obteve %d", resp.StatusCode)
	}

	saida := entrada.Add(2*time.Hour + 30*time.Minute)
	resp, res, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", cookie, map[string]string{"horario": saida.Format(time.RFC3339)})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200 na saída, obteve %d: %v", resp.StatusCode, res)
	}
	if res["duracao_segundos"].(float64) != 9000 {
		t.Errorf("duração esperada 9000s, obteve %v", res["duracao_segundos"])
	}

	hoje := time.Now().Format("2006-01-02")
	ontem := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/tecnicos/relatorio?tecnico_id="+tecID+"&inicio="+ontem+"&fim="+hoje, nil)
	req.AddCookie(cookie)
	httpResp, err := env.client.Do(req)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		t.Fatalf("falha ao gerar relatório: %v", err)
	}
	defer httpResp.Body.Close()
	var relatorio []map[string]any
	_ = json.NewDecoder(httpResp.Body).Decode(&relatorio)
	if len(relatorio) != 1 {
		t.Fatalf("esperada 1 linha no relatório, obteve %d", len(relatorio))
	}
	if relatorio[0]["segundos_totais"].(float64) != 9000 || relatorio[0]["total_registros"].(float64) != 1 {
		t.Errorf("relatório inesperado: %v", relatorio[0])
	}

	// CSV de registros
	reqCSV, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/tecnicos/registros/exportar.csv?tecnico_id="+tecID, nil)
	reqCSV.AddCookie(cookie)
	respCSV, err := env.client.Do(reqCSV)
	if err != nil || respCSV.StatusCode != http.StatusOK {
		t.Fatalf("falha ao exportar CSV de técnicos: %v", err)
	}
	defer respCSV.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(respCSV.Body)
	if !bytes.Contains(buf.Bytes(), []byte(nome)) || !bytes.Contains(buf.Bytes(), []byte("2:30")) {
		t.Errorf("CSV não contém o registro esperado: %s", buf.String())
	}
}

// -------------------------------------------------------------
// Visibilidade: operador vê só as próprias tarefas; admin vê tudo
// -------------------------------------------------------------
func TestVisibilidade_TarefasIndividuais(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	users, err := env.db.Queries.ListarTodosUsuarios(ctx)
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	sufixo := time.Now().UnixNano()
	criarOperador := func(nome, pin string) string {
		_, res, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
			"nome": fmt.Sprintf("%s %d", nome, sufixo), "cor": "#10B981", "pin": pin, "papel": "usuario",
		})
		return res["id"].(string)
	}
	idA := criarOperador("Visib A", "3131")
	idB := criarOperador("Visib B", "4242")
	idC := criarOperador("Visib C", "5353")
	defer func() {
		for _, id := range []string{idA, idB, idC} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tarefas WHERE criado_por = $1 OR responsavel_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()
	cookieA, _, _ := env.login(t, idA, "3131")
	cookieB, _, _ := env.login(t, idB, "4242")
	cookieC, _, _ := env.login(t, idC, "5353")

	getJSON := func(path string, cookie *http.Cookie, out any) int {
		req, _ := http.NewRequest(http.MethodGet, env.server.URL+path, nil)
		req.AddCookie(cookie)
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if out != nil {
			_ = json.NewDecoder(resp.Body).Decode(out)
		}
		return resp.StatusCode
	}

	// Tarefa criada por A e atribuída a B: A e B veem, C não
	titulo := fmt.Sprintf("Tarefa Visib %d", sufixo)
	_, resT, _ := env.doRequest(http.MethodPost, "/api/tarefas", cookieA, map[string]any{
		"titulo": titulo, "prioridade": "media", "responsavel_id": idB,
	})
	tarefaID, _ := resT["id"].(string)

	contem := func(cookie *http.Cookie) bool {
		var tarefas []map[string]any
		getJSON("/api/tarefas?visao=todas", cookie, &tarefas)
		for _, tf := range tarefas {
			if tf["titulo"] == titulo {
				return true
			}
		}
		return false
	}
	if !contem(cookieA) || !contem(cookieB) || !contem(adminCookie) {
		t.Errorf("criador, responsável e admin deveriam ver a tarefa")
	}
	if contem(cookieC) {
		t.Errorf("operador sem vínculo não deveria ver a tarefa")
	}
	if st := getJSON("/api/tarefas/"+tarefaID+"/comentarios", cookieC, nil); st != http.StatusNotFound {
		t.Errorf("comentários de tarefa alheia: esperado 404, obteve %d", st)
	}
}

// -------------------------------------------------------------
// Estoque: item sem movimentações é excluído; com movimentações é desativado
// -------------------------------------------------------------
func TestEstoque_ExcluirItem(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	users, err := env.db.Queries.ListarTodosUsuarios(ctx)
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	criarItem := func(nome string) string {
		resp, res, _ := env.doRequest(http.MethodPost, "/api/estoque/itens", adminCookie, map[string]any{
			"nome": fmt.Sprintf("%s %d", nome, time.Now().UnixNano()), "unidade": "un",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("falha ao criar item: %d %v", resp.StatusCode, res)
		}
		return res["id"].(string)
	}

	// Sem movimentações: excluído de verdade
	semMov := criarItem("Item Sem Mov")
	resp, res, _ := env.doRequest(http.MethodDelete, "/api/estoque/itens/"+semMov, adminCookie, nil)
	if resp.StatusCode != http.StatusOK || res["excluido"] != true {
		t.Errorf("item sem movimentações deveria ser excluído: %d %v", resp.StatusCode, res)
	}
	resp, _, _ = env.doRequest(http.MethodGet, "/api/estoque/itens/"+semMov, adminCookie, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("item excluído ainda encontrado: %d", resp.StatusCode)
	}

	// Com movimentação: desativado e histórico preservado
	comMov := criarItem("Item Com Mov")
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM movimentacoes_estoque WHERE item_id = $1", comMov)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM itens_estoque WHERE id = $1", comMov)
	}()
	env.doRequest(http.MethodPost, "/api/estoque/movimentacoes", adminCookie, map[string]any{
		"item_id": comMov, "tipo": "entrada", "quantidade": 3,
	})
	resp, res, _ = env.doRequest(http.MethodDelete, "/api/estoque/itens/"+comMov, adminCookie, nil)
	if resp.StatusCode != http.StatusOK || res["excluido"] != false {
		t.Errorf("item com movimentações deveria ser desativado: %d %v", resp.StatusCode, res)
	}
	_, res, _ = env.doRequest(http.MethodGet, "/api/estoque/itens/"+comMov, adminCookie, nil)
	if res["ativo"] != false {
		t.Errorf("item deveria estar desativado: %v", res)
	}

	// Operador comum não pode excluir
	nomeOp := fmt.Sprintf("Op Estoque %d", time.Now().UnixNano())
	_, resU, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome": nomeOp, "cor": "#10B981", "pin": "6464", "papel": "usuario",
	})
	opID := resU["id"].(string)
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()
	opCookie, _, _ := env.login(t, opID, "6464")
	outro := criarItem("Item Protegido")
	resp, _, _ = env.doRequest(http.MethodDelete, "/api/estoque/itens/"+outro, opCookie, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("operador excluindo item: esperado 403, obteve %d", resp.StatusCode)
	}
	env.doRequest(http.MethodDelete, "/api/estoque/itens/"+outro, adminCookie, nil)
}

// -------------------------------------------------------------
// PIN: exatamente 4 dígitos no cadastro, redefinição e login
// -------------------------------------------------------------
func TestPIN_ExatamenteQuatroDigitos(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	users, err := env.db.Queries.ListarTodosUsuarios(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminID := database.UUIDToString(users[0].ID)
	adminCookie, _, _ := env.login(t, adminID, env.cfg.AdminPIN)

	for _, pin := range []string{"123", "12345", "123456", "12a4"} {
		resp, _, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
			"nome": fmt.Sprintf("PIN inválido %d", time.Now().UnixNano()), "cor": "#10B981", "pin": pin, "papel": "usuario",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("cadastro com PIN %q: esperado 400, obteve %d", pin, resp.StatusCode)
		}
		resp, _, _ = env.doRequest(http.MethodPost, "/api/usuarios/"+adminID+"/pin", adminCookie, map[string]any{"novo_pin": pin})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("redefinição com PIN %q: esperado 400, obteve %d", pin, resp.StatusCode)
		}
		if _, status, _ := env.login(t, adminID, pin); status != http.StatusBadRequest {
			t.Errorf("login com PIN %q: esperado 400, obteve %d", pin, status)
		}
	}
}

// -------------------------------------------------------------
// Tickets: abertura sem login, resgate vira tarefa, sem resgate duplo
// -------------------------------------------------------------
func TestTickets_AberturaPublicaEResgate(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	users, err := env.db.Queries.ListarTodosUsuarios(ctx)
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	_, resU, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome": fmt.Sprintf("Op Tickets %d", time.Now().UnixNano()), "cor": "#10B981", "pin": "7575", "papel": "usuario",
	})
	opID := resU["id"].(string)
	opCookie, _, _ := env.login(t, opID, "7575")

	var criados []string
	defer func() {
		for _, id := range criados {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tarefas WHERE id = (SELECT tarefa_id FROM tickets WHERE id = $1)", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tickets WHERE id = $1", id)
		}
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	abrir := func(titulo string) (int, map[string]any) {
		resp, res, _ := env.doRequest(http.MethodPost, "/api/tickets/publico", nil, map[string]any{
			"solicitante_nome": "Recepção", "titulo": titulo, "descricao": "Impressora sem papel", "prioridade": "alta",
		})
		if id, ok := res["id"].(string); ok {
			criados = append(criados, id)
		}
		return resp.StatusCode, res
	}

	// Validação: campos obrigatórios
	resp, _, _ := env.doRequest(http.MethodPost, "/api/tickets/publico", nil, map[string]any{"solicitante_nome": "X"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ticket sem assunto/descrição: esperado 400, obteve %d", resp.StatusCode)
	}

	// Abertura sem login
	status, res := abrir("Impressora")
	if status != http.StatusCreated || res["numero"] == nil {
		t.Fatalf("abertura pública: esperado 201 com número, obteve %d %v", status, res)
	}
	ticketID := res["id"].(string)

	// Listagem exige login
	resp, _, _ = env.doRequest(http.MethodGet, "/api/tickets/resumo", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("resumo sem login: esperado 401, obteve %d", resp.StatusCode)
	}

	// Operador comum não descarta
	resp, _, _ = env.doRequest(http.MethodPost, "/api/tickets/"+ticketID+"/descartar", opCookie, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("operador descartando: esperado 403, obteve %d", resp.StatusCode)
	}

	// 10 resgates simultâneos: só um vence
	var wg sync.WaitGroup
	var mu sync.Mutex
	codigos := map[int]int{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _, err := env.doRequest(http.MethodPost, "/api/tickets/"+ticketID+"/resgatar", opCookie, nil)
			if err == nil {
				mu.Lock()
				codigos[r.StatusCode]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if codigos[http.StatusOK] != 1 || codigos[http.StatusConflict] != 9 {
		t.Errorf("resgates simultâneos: esperado 1x200 e 9x409, obteve %v", codigos)
	}

	// A tarefa gerada aparece em "Minhas tarefas" do operador
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/tarefas?visao=minhas", nil)
	req.AddCookie(opCookie)
	httpResp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("erro ao listar tarefas: %v", err)
	}
	defer httpResp.Body.Close()
	var tarefas []map[string]any
	_ = json.NewDecoder(httpResp.Body).Decode(&tarefas)
	achou := false
	for _, tf := range tarefas {
		if titulo, _ := tf["titulo"].(string); strings.HasSuffix(titulo, ": Impressora") && tf["prioridade"] == "alta" {
			achou = true
		}
	}
	if !achou {
		t.Errorf("tarefa do ticket não apareceu em Minhas tarefas: %v", tarefas)
	}

	// Limite de envios por IP (a primeira abertura já contou)
	for i := 0; i < 4; i++ {
		if st, _ := abrir("Spam"); st != http.StatusCreated {
			t.Fatalf("abertura %d dentro do limite: esperado 201, obteve %d", i+2, st)
		}
	}
	if st, _ := abrir("Spam"); st != http.StatusTooManyRequests {
		t.Errorf("6ª abertura: esperado 429, obteve %d", st)
	}
}

func TestProcedimentos_HistoricoETiraDuvidas(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	users, err := env.db.Queries.ListarTodosUsuarios(ctx)
	if err != nil || len(users) == 0 {
		t.Fatalf("nenhum usuário no banco")
	}
	adminCookie, _, _ := env.login(t, database.UUIDToString(users[0].ID), env.cfg.AdminPIN)

	_, resU, _ := env.doRequest(http.MethodPost, "/api/usuarios", adminCookie, map[string]any{
		"nome": fmt.Sprintf("Op Proc %d", time.Now().UnixNano()), "cor": "#10B981", "pin": "7676", "papel": "usuario",
	})
	opID := resU["id"].(string)
	opCookie, _, _ := env.login(t, opID, "7676")

	// O teste usa o banco de desenvolvimento: guarda o uso do dia para devolver no fim
	var usoAntes *float64
	var perguntasAntes int
	_ = env.db.Pool.QueryRow(ctx, "SELECT neurons, perguntas FROM assistente_uso WHERE dia = (now() AT TIME ZONE 'UTC')::date").Scan(&usoAntes, &perguntasAntes)

	var criados []string
	defer func() {
		for _, id := range criados {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM procedimentos WHERE id = $1", id)
		}
		if usoAntes == nil {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM assistente_uso WHERE dia = (now() AT TIME ZONE 'UTC')::date")
		} else {
			_, _ = env.db.Pool.Exec(ctx, "UPDATE assistente_uso SET neurons = $1, perguntas = $2 WHERE dia = (now() AT TIME ZONE 'UTC')::date", *usoAntes, perguntasAntes)
		}
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	sufixo := time.Now().UnixNano()
	titulo := fmt.Sprintf("Servidor fora do ar %d", sufixo)
	corpoV1 := "## Como as pessoas descrevem\n- o servidor caiu\n\n## O que fazer\nLigue pro Álvaro.\n\n## Contato\n**Álvaro** · ramal 214"

	// Só admin cria
	resp, _, _ := env.doRequest(http.MethodPost, "/api/procedimentos", opCookie, map[string]any{"titulo": titulo, "corpo": corpoV1})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("operador criando procedimento: esperado 403, obteve %d", resp.StatusCode)
	}
	resp, _, _ = env.doRequest(http.MethodPost, "/api/procedimentos", adminCookie, map[string]any{"titulo": titulo})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("procedimento sem texto: esperado 400, obteve %d", resp.StatusCode)
	}

	resp, res, _ := env.doRequest(http.MethodPost, "/api/procedimentos", adminCookie, map[string]any{
		"titulo": titulo, "categoria": "Infra", "corpo": corpoV1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar procedimento: esperado 201, obteve %d %v", resp.StatusCode, res)
	}
	id := res["id"].(string)
	criados = append(criados, id)

	salvar := func(corpo string, ativo bool) map[string]any {
		t.Helper()
		resp, res, _ := env.doRequest(http.MethodPut, "/api/procedimentos/"+id, adminCookie, map[string]any{
			"titulo": titulo, "categoria": "Infra", "corpo": corpo, "ativo": ativo,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("salvar procedimento: esperado 200, obteve %d %v", resp.StatusCode, res)
		}
		return res
	}
	revisoes := func() []map[string]any {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/procedimentos/"+id+"/revisoes", nil)
		req.AddCookie(adminCookie)
		r, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("erro ao listar revisões: %v", err)
		}
		defer r.Body.Close()
		var lista []map[string]any
		_ = json.NewDecoder(r.Body).Decode(&lista)
		return lista
	}

	salvar(corpoV1+"\n\nAtualizado.", true)
	salvar(corpoV1+"\n\nAtualizado.", true) // sem mudança: não gera revisão
	revs := revisoes()
	if len(revs) != 2 || revs[0]["nota"] != "Editado" || revs[1]["nota"] != "Criado" {
		t.Fatalf("histórico após editar: esperado [Editado, Criado], obteve %v", revs)
	}

	// Restaurar a primeira versão gera uma revisão nova
	resp, res, _ = env.doRequest(http.MethodPost, "/api/procedimentos/"+id+"/revisoes/"+revs[1]["id"].(string)+"/restaurar", adminCookie, nil)
	if resp.StatusCode != http.StatusOK || res["corpo"] != corpoV1 {
		t.Fatalf("restaurar: esperado 200 com o texto original, obteve %d %v", resp.StatusCode, res)
	}
	revs = revisoes()
	if nota, _ := revs[0]["nota"].(string); len(revs) != 3 || !strings.HasPrefix(nota, "Restaurado da versão de") {
		t.Fatalf("histórico após restaurar: %v", revs)
	}

	// Tira-dúvidas desligado sem credenciais
	resp, res, _ = env.doRequest(http.MethodGet, "/api/assistente/status", nil, nil)
	if resp.StatusCode != http.StatusOK || res["disponivel"] != false || res["motivo"] != "desligado" {
		t.Errorf("status sem credenciais: obteve %d %v", resp.StatusCode, res)
	}

	// Workers AI falsa: confere o prompt e cita o nosso procedimento
	var chamadas int
	var promptSistema, ultimaPergunta string
	falsa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		if r.URL.Path != "/accounts/conta-teste/ai/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer token-teste" {
			http.Error(w, "rota ou token errado", http.StatusUnauthorized)
			return
		}
		var body struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		promptSistema = body.Messages[0].Content
		ultimaPergunta = body.Messages[len(body.Messages)-1].Content

		// Número com que o nosso procedimento entrou no prompt
		indice := "0"
		for _, linha := range strings.Split(promptSistema, "\n") {
			if strings.HasSuffix(linha, "] "+titulo+" (Infra)") {
				indice = strings.TrimPrefix(strings.SplitN(linha, "]", 2)[0], "[")
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{
				"content": "<think>\n\n</think>\n\nLigue para o **Álvaro**.\nFONTES: " + indice,
			}}},
			"usage": map[string]any{"prompt_tokens": 1000, "completion_tokens": 100},
		})
	}))
	defer falsa.Close()
	env.cfg.CFAccountID, env.cfg.CFAPIToken, env.cfg.CFAPIBaseURL = "conta-teste", "token-teste", falsa.URL

	perguntar := func(texto string) (*http.Response, map[string]any) {
		resp, res, _ := env.doRequest(http.MethodPost, "/api/assistente/publico", nil, map[string]any{
			"mensagens": []map[string]string{{"papel": "usuario", "texto": texto}},
		})
		return resp, res
	}

	resp, res = perguntar("o servidor caiu, o que eu faço?")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pergunta: esperado 200, obteve %d %v", resp.StatusCode, res)
	}
	if res["resposta"] != "Ligue para o **Álvaro**." {
		t.Errorf("resposta deveria vir sem <think> e sem a linha FONTES: %q", res["resposta"])
	}
	fontes, _ := res["fontes"].([]any)
	if len(fontes) != 1 {
		t.Fatalf("esperava 1 fonte, obteve %v (prompt: %q)", res["fontes"], promptSistema)
	}
	fonte := fontes[0].(map[string]any)
	if fonte["id"] != id || fonte["contato"] != "**Álvaro** · ramal 214" {
		t.Errorf("fonte deveria trazer o contato do procedimento: %v", fonte)
	}
	if !strings.HasSuffix(ultimaPergunta, "/no_think") {
		t.Errorf("pergunta enviada sem /no_think: %q", ultimaPergunta)
	}

	var neurons float64
	_ = env.db.Pool.QueryRow(ctx, "SELECT neurons FROM assistente_uso WHERE dia = (now() AT TIME ZONE 'UTC')::date").Scan(&neurons)
	esperado := 1000*env.cfg.NeuronsEntradaPorM/1e6 + 100*env.cfg.NeuronsSaidaPorM/1e6
	if usoAntes != nil {
		esperado += *usoAntes
	}
	if diff := neurons - esperado; diff > 0.001 || diff < -0.001 {
		t.Errorf("neurons do dia: esperado %.3f, obteve %.3f", esperado, neurons)
	}

	// Sem procedimento parecido, responde sem chamar o modelo
	antes := chamadas
	resp, res = perguntar("zzqx wqpv")
	if resp.StatusCode != http.StatusOK || chamadas != antes {
		t.Errorf("pergunta sem procedimento não deveria chamar o modelo: %d %v (chamadas %d→%d)", resp.StatusCode, res, antes, chamadas)
	}

	// Procedimento desativado sai da busca
	salvar(corpoV1, false)
	if revs = revisoes(); revs[0]["nota"] != "Desativado" {
		t.Errorf("desativar deveria gerar a nota Desativado: %v", revs[0])
	}
	antes = chamadas
	perguntar("o servidor caiu, o que eu faço?")
	if chamadas > antes && strings.Contains(promptSistema, titulo) {
		t.Errorf("procedimento desativado apareceu no prompt")
	}

	// Última pergunta precisa ser do usuário
	resp, _, _ = env.doRequest(http.MethodPost, "/api/assistente/publico", nil, map[string]any{
		"mensagens": []map[string]string{{"papel": "assistente", "texto": "oi"}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("conversa terminando no assistente: esperado 400, obteve %d", resp.StatusCode)
	}

	// Cota esgotada
	env.cfg.AssistenteNeuronsDia = 0.001
	resp, res, _ = env.doRequest(http.MethodGet, "/api/assistente/status", nil, nil)
	if res["disponivel"] != false || res["motivo"] != "cota" {
		t.Errorf("status com cota esgotada: %v", res)
	}
	resp, _ = perguntar("o servidor caiu")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("pergunta com cota esgotada: esperado 503, obteve %d", resp.StatusCode)
	}
}
