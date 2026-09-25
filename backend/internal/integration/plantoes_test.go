package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// Escala: só admin monta; quem fica escalado é só um nome (não precisa ser
// usuário); conflito do mesmo nome no mesmo tipo é recusado; rodízio alterna as
// pessoas; o painel e a TV do plantão mostram quem está de plantão hoje.
func TestPlantoes_EscalaERodizio(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	opID := env.criarOperador(t, adminCookie, "Op Plantao A", "8787")
	opCookie, _, _ := env.login(t, opID, "8787")

	_, me, _ := env.doRequest(http.MethodGet, "/api/auth/me", opCookie, nil)
	opNome, _ := me["nome"].(string)
	if opNome == "" {
		t.Fatalf("nome do operador não veio em /api/auth/me: %v", me)
	}

	const ana, bea = "Ana Plantonista Teste", "Bea Plantonista Teste"
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM plantoes WHERE nome ILIKE '%Plantonista Teste' OR nome = $1", opNome)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	hoje := time.Now().In(loc)
	dia := func(n int) string { return hoje.AddDate(0, 0, n).Format("2006-01-02") }

	// Operador não monta escala
	turnoHoje := map[string]any{"nome": ana, "tipo": "plantao", "inicio": dia(0), "fim": dia(1)}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", opCookie, turnoHoje); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 para operador criar plantão, veio %d", resp.StatusCode)
	}

	// Validações
	for _, c := range []map[string]any{
		{"nome": ana, "tipo": "folga", "inicio": dia(0)},
		{"nome": ana, "inicio": "ontem"},
		{"nome": ana, "inicio": dia(2), "fim": dia(1)},
		{"nome": "   ", "inicio": dia(0)},
	} {
		if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, c); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d", c, resp.StatusCode)
		}
	}

	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, turnoHoje); resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar plantão, veio %d", resp.StatusCode)
	}

	// Mesmo nome (sem diferenciar maiúsculas e espaços), mesmo tipo, dias
	// sobrepostos: conflito. Sobreaviso no mesmo dia pode.
	sobreposto := map[string]any{"nome": "  ana  PLANTONISTA teste ", "tipo": "plantao", "inicio": dia(1), "fim": dia(3)}
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, sobreposto); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para plantão sobreposto, veio %d (%v)", resp.StatusCode, res)
	}
	sobreaviso := map[string]any{"nome": ana, "tipo": "sobreaviso", "inicio": dia(1)}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, sobreaviso); resp.StatusCode != http.StatusCreated {
		t.Errorf("esperado 201 para sobreaviso no mesmo dia, veio %d", resp.StatusCode)
	}

	// O operador também pode estar na escala: o painel liga pelo nome
	meuTurno := map[string]any{"nome": opNome, "tipo": "sobreaviso", "inicio": dia(0)}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, meuTurno); resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao escalar o operador, veio %d", resp.StatusCode)
	}

	// Painel: Ana está de plantão hoje; o turno do operador é o de hoje
	_, painel, _ := env.doRequest(http.MethodGet, "/api/painel", opCookie, nil)
	plantao := painel["plantao"].(map[string]any)
	achou := false
	for _, p := range plantao["hoje"].([]any) {
		if p.(map[string]any)["nome"] == ana {
			achou = true
		}
	}
	if !achou {
		t.Error("plantão de hoje não apareceu no painel")
	}
	if meu, ok := plantao["meu_proximo"].(map[string]any); !ok || meu["inicio"] != dia(0) || meu["tipo"] != "sobreaviso" {
		t.Errorf("meu_proximo deveria ser o sobreaviso de hoje, veio %v", plantao["meu_proximo"])
	}

	// TV do plantão: sem sessão nem chave é recusada; com sessão traz a escala
	if resp, _, _ := env.doRequest(http.MethodGet, "/api/tv/plantao", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("esperado 401 na TV do plantão sem sessão, veio %d", resp.StatusCode)
	}
	resp, tv, _ := env.doRequest(http.MethodGet, "/api/tv/plantao", opCookie, nil)
	if resp.StatusCode != http.StatusOK || tv["hoje"] != dia(0) {
		t.Fatalf("esperado 200 com o dia de hoje na TV do plantão, veio %d (%v)", resp.StatusCode, tv)
	}
	achou = false
	for _, p := range tv["escala"].([]any) {
		if p.(map[string]any)["nome"] == ana {
			achou = true
		}
	}
	if !achou {
		t.Error("plantão de hoje não apareceu na TV do plantão")
	}

	// Rodízio semanal de 4 turnos alternando Bea e Ana, a partir de daqui a 30 dias
	rodizio := map[string]any{"pessoas": []string{bea, ana}, "tipo": "plantao", "inicio": dia(30), "dias_por_turno": 7, "turnos": 4}
	resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, rodizio)
	if resp.StatusCode != http.StatusCreated || res["criados"] != float64(4) {
		t.Fatalf("esperado 201 com 4 turnos criados, veio %d (%v)", resp.StatusCode, res)
	}
	lista := env.getLista(t, "/api/plantoes?inicio="+dia(30)+"&fim="+dia(57), opCookie)
	var ordem []string
	for _, p := range lista {
		if p["nome"] == ana || p["nome"] == bea {
			ordem = append(ordem, p["nome"].(string)+"@"+p["inicio"].(string)+".."+p["fim"].(string))
		}
	}
	esperado := []string{
		bea + "@" + dia(30) + ".." + dia(36),
		ana + "@" + dia(37) + ".." + dia(43),
		bea + "@" + dia(44) + ".." + dia(50),
		ana + "@" + dia(51) + ".." + dia(57),
	}
	if len(ordem) != 4 {
		t.Fatalf("esperados 4 turnos do rodízio, veio %v", ordem)
	}
	for i := range esperado {
		if ordem[i] != esperado[i] {
			t.Errorf("turno %d: esperado %s, veio %s", i, esperado[i], ordem[i])
		}
	}

	// Mesmo nome duas vezes no rodízio é recusado
	repetido := map[string]any{"pessoas": []string{ana, "ANA plantonista teste"}, "inicio": dia(90), "dias_por_turno": 1, "turnos": 2}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, repetido); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado 400 para nome repetido no rodízio, veio %d", resp.StatusCode)
	}

	// Repetir o rodízio conflita e não grava nada (tudo ou nada)
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, rodizio); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 ao repetir o rodízio, veio %d", resp.StatusCode)
	}
	if n := len(env.getLista(t, "/api/plantoes?inicio="+dia(30)+"&fim="+dia(57), opCookie)); n != 4 {
		t.Errorf("rodízio conflitante não deveria gravar nada; turnos no período: %d", n)
	}

	// Sugestões: nomes já usados na escala, sem repetir
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/plantoes/pessoas", nil)
	req.AddCookie(opCookie)
	respPessoas, err := env.client.Do(req)
	if err != nil || respPessoas.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200 nas sugestões de nomes (%v)", err)
	}
	var pessoas []string
	_ = json.NewDecoder(respPessoas.Body).Decode(&pessoas)
	respPessoas.Body.Close()
	vezes := map[string]int{}
	for _, n := range pessoas {
		vezes[n]++
	}
	for _, n := range []string{ana, bea} {
		if vezes[n] != 1 {
			t.Errorf("%s deveria aparecer uma vez nas sugestões, veio %v", n, pessoas)
		}
	}

	// Editar: trocar a pessoa do turno de hoje; excluir
	var idHoje string
	for _, p := range env.getLista(t, "/api/plantoes?inicio="+dia(0)+"&fim="+dia(0), opCookie) {
		if p["nome"] == ana && p["tipo"] == "plantao" {
			idHoje = p["id"].(string)
		}
	}
	troca := map[string]any{"nome": bea, "tipo": "plantao", "inicio": dia(0), "fim": dia(1), "observacao": "troca com Ana"}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/plantoes/"+idHoje, adminCookie, troca); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 ao editar turno, veio %d", resp.StatusCode)
	}
	// Editar o próprio turno sem mudar datas não conflita consigo mesmo
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/plantoes/"+idHoje, adminCookie, troca); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 ao salvar o turno de novo, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/plantoes/"+idHoje, adminCookie, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperado 204 ao excluir turno, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodDelete, "/api/plantoes/"+idHoje, adminCookie, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("esperado 404 ao excluir turno inexistente, veio %d", resp.StatusCode)
	}
}

// Folga opcional: vai junto com o turno, aparece na listagem pelos dias de
// folga, não conta como "de plantão hoje" e ninguém é escalado na própria folga.
func TestPlantoes_Folga(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	const caio, duda = "Caio Folguista Teste", "Duda Folguista Teste"
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM plantoes WHERE nome ILIKE '%Folguista Teste'")
	}()

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	hoje := time.Now().In(loc)
	dia := func(n int) string { return hoje.AddDate(0, 0, n).Format("2006-01-02") }
	criar := func(corpo map[string]any) (int, map[string]any) {
		resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, corpo)
		return resp.StatusCode, res
	}

	// Folga inválida ou dentro do próprio turno
	for _, c := range []map[string]any{
		{"nome": caio, "inicio": dia(-3), "fim": dia(-2), "folga_inicio": dia(-2)},
		{"nome": caio, "inicio": dia(-3), "fim": dia(-2), "folga_inicio": dia(1), "folga_fim": dia(0)},
		{"nome": caio, "inicio": dia(-3), "fim": dia(-2), "folga_inicio": "amanhã"},
		{"nome": caio, "inicio": dia(-3), "fim": dia(-2), "folga_inicio": dia(0), "folga_fim": dia(40)},
	} {
		if st, res := criar(c); st != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d (%v)", c, st, res)
		}
	}

	// Plantão até ontem, folga hoje e amanhã
	if st, res := criar(map[string]any{"nome": caio, "inicio": dia(-3), "fim": dia(-1), "folga_inicio": dia(0), "folga_fim": dia(1)}); st != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar plantão com folga, veio %d (%v)", st, res)
	}

	// A listagem de hoje traz o turno pela folga, com as datas da folga
	var achou map[string]any
	for _, p := range env.getLista(t, "/api/plantoes?inicio="+dia(0)+"&fim="+dia(0), adminCookie) {
		if p["nome"] == caio {
			achou = p
		}
	}
	if achou == nil || achou["folga_inicio"] != dia(0) || achou["folga_fim"] != dia(1) {
		t.Fatalf("turno com folga hoje deveria vir na listagem com a folga, veio %v", achou)
	}

	// De folga não é "de plantão hoje" (painel e TV)
	_, painel, _ := env.doRequest(http.MethodGet, "/api/painel", adminCookie, nil)
	for _, p := range painel["plantao"].(map[string]any)["hoje"].([]any) {
		if p.(map[string]any)["nome"] == caio {
			t.Error("quem está de folga não deveria aparecer de plantão hoje no painel")
		}
	}
	_, tv, _ := env.doRequest(http.MethodGet, "/api/tv/painel", adminCookie, nil)
	for _, p := range tv["plantao_hoje"].([]any) {
		if p.(map[string]any)["nome"] == caio {
			t.Error("quem está de folga não deveria aparecer de plantão hoje na TV")
		}
	}

	// Ninguém trabalha na folga: nem plantão nem sobreaviso
	for _, tipo := range []string{"plantao", "sobreaviso"} {
		if st, res := criar(map[string]any{"nome": "caio folguista TESTE", "tipo": tipo, "inicio": dia(1)}); st != http.StatusConflict {
			t.Errorf("esperado 409 para %s na folga, veio %d (%v)", tipo, st, res)
		}
	}
	// E a folga nova não cai em turno já marcado
	if st, _ := criar(map[string]any{"nome": caio, "tipo": "sobreaviso", "inicio": dia(5)}); st != http.StatusCreated {
		t.Fatalf("esperado 201 para sobreaviso fora da folga, veio %d", st)
	}
	if st, res := criar(map[string]any{"nome": caio, "inicio": dia(3), "fim": dia(4), "folga_inicio": dia(5)}); st != http.StatusConflict {
		t.Errorf("esperado 409 para folga sobre o sobreaviso, veio %d (%v)", st, res)
	}

	// Rodízio com folga antes de cada turno: com uma pessoa só, a folga do
	// segundo turno cairia no primeiro
	sozinho := map[string]any{"pessoas": []string{duda}, "inicio": dia(60), "dias_por_turno": 2, "turnos": 2, "folga_dias_antes": 1}
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, sozinho); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para rodízio sozinho com folga, veio %d (%v)", resp.StatusCode, res)
	}
	// Folga 3 dias antes: o turno que começa no dia 63 tem folga no dia 60,
	// como plantão no domingo e folga na quinta
	dupla := map[string]any{"pessoas": []string{duda, "Eva Folguista Teste"}, "inicio": dia(63), "dias_por_turno": 2, "turnos": 2, "folga_dias_antes": 3}
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, dupla); resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 para rodízio com folga, veio %d (%v)", resp.StatusCode, res)
	}
	achouFolga := false
	for _, p := range env.getLista(t, "/api/plantoes?inicio="+dia(63)+"&fim="+dia(64), adminCookie) {
		if p["nome"] == duda {
			achouFolga = true
			if p["folga_inicio"] != dia(60) || p["folga_fim"] != dia(60) {
				t.Errorf("folga do primeiro turno do rodízio deveria ser só %s, veio %v", dia(60), p)
			}
		}
	}
	if !achouFolga {
		t.Error("primeiro turno do rodízio com folga não apareceu")
	}

	// Editar tira a folga
	var id string
	for _, p := range env.getLista(t, "/api/plantoes?inicio="+dia(-1)+"&fim="+dia(-1), adminCookie) {
		if p["nome"] == caio {
			id = p["id"].(string)
		}
	}
	semFolga := map[string]any{"nome": caio, "inicio": dia(-3), "fim": dia(-1)}
	if resp, _, _ := env.doRequest(http.MethodPut, "/api/plantoes/"+id, adminCookie, semFolga); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperado 204 ao tirar a folga, veio %d", resp.StatusCode)
	}
	if st, res := criar(map[string]any{"nome": caio, "tipo": "plantao", "inicio": dia(1)}); st != http.StatusCreated {
		t.Errorf("sem a folga, o plantão de amanhã deveria entrar, veio %d (%v)", st, res)
	}
}
