package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Escala: só admin monta; conflito da mesma pessoa no mesmo tipo é recusado;
// rodízio alterna as pessoas; o painel mostra quem está de plantão hoje.
func TestPlantoes_EscalaERodizio(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	anaID := env.criarOperador(t, adminCookie, "Op Plantao A", "8787")
	beaID := env.criarOperador(t, adminCookie, "Op Plantao B", "8888")
	anaCookie, _, _ := env.login(t, anaID, "8787")

	defer func() {
		for _, id := range []string{anaID, beaID} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM plantoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	hoje := time.Now().In(loc)
	dia := func(n int) string { return hoje.AddDate(0, 0, n).Format("2006-01-02") }

	// Operador não monta escala
	turnoHoje := map[string]any{"usuario_id": anaID, "tipo": "plantao", "inicio": dia(0), "fim": dia(1)}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", anaCookie, turnoHoje); resp.StatusCode != http.StatusForbidden {
		t.Errorf("esperado 403 para operador criar plantão, veio %d", resp.StatusCode)
	}

	// Validações
	for _, c := range []map[string]any{
		{"usuario_id": anaID, "tipo": "folga", "inicio": dia(0)},
		{"usuario_id": anaID, "inicio": "ontem"},
		{"usuario_id": anaID, "inicio": dia(2), "fim": dia(1)},
		{"usuario_id": "00000000-0000-0000-0000-000000000000", "inicio": dia(0)},
	} {
		if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, c); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado 400 para %v, veio %d", c, resp.StatusCode)
		}
	}

	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, turnoHoje); resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201 ao criar plantão, veio %d", resp.StatusCode)
	}

	// Mesma pessoa, mesmo tipo, dias sobrepostos: conflito. Sobreaviso no mesmo dia pode.
	sobreposto := map[string]any{"usuario_id": anaID, "tipo": "plantao", "inicio": dia(1), "fim": dia(3)}
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, sobreposto); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 para plantão sobreposto, veio %d (%v)", resp.StatusCode, res)
	}
	sobreaviso := map[string]any{"usuario_id": anaID, "tipo": "sobreaviso", "inicio": dia(1)}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes", adminCookie, sobreaviso); resp.StatusCode != http.StatusCreated {
		t.Errorf("esperado 201 para sobreaviso no mesmo dia, veio %d", resp.StatusCode)
	}

	// Painel: Ana está de plantão hoje e é o turno atual dela
	_, painel, _ := env.doRequest(http.MethodGet, "/api/painel", anaCookie, nil)
	plantao := painel["plantao"].(map[string]any)
	achou := false
	for _, p := range plantao["hoje"].([]any) {
		if p.(map[string]any)["usuario_id"] == anaID {
			achou = true
		}
	}
	if !achou {
		t.Error("plantão de hoje não apareceu no painel")
	}
	if meu, ok := plantao["meu_proximo"].(map[string]any); !ok || meu["inicio"] != dia(0) {
		t.Errorf("meu_proximo deveria ser o turno de hoje, veio %v", plantao["meu_proximo"])
	}

	// Rodízio semanal de 4 turnos alternando Bea e Ana, a partir de daqui a 30 dias
	rodizio := map[string]any{"usuarios": []string{beaID, anaID}, "tipo": "plantao", "inicio": dia(30), "dias_por_turno": 7, "turnos": 4}
	resp, res, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, rodizio)
	if resp.StatusCode != http.StatusCreated || res["criados"] != float64(4) {
		t.Fatalf("esperado 201 com 4 turnos criados, veio %d (%v)", resp.StatusCode, res)
	}
	lista := env.getLista(t, "/api/plantoes?inicio="+dia(30)+"&fim="+dia(57), anaCookie)
	var ordem []string
	for _, p := range lista {
		if p["usuario_id"] == anaID || p["usuario_id"] == beaID {
			ordem = append(ordem, p["usuario_id"].(string)+"@"+p["inicio"].(string)+".."+p["fim"].(string))
		}
	}
	esperado := []string{
		beaID + "@" + dia(30) + ".." + dia(36),
		anaID + "@" + dia(37) + ".." + dia(43),
		beaID + "@" + dia(44) + ".." + dia(50),
		anaID + "@" + dia(51) + ".." + dia(57),
	}
	if len(ordem) != 4 {
		t.Fatalf("esperados 4 turnos do rodízio, veio %v", ordem)
	}
	for i := range esperado {
		if ordem[i] != esperado[i] {
			t.Errorf("turno %d: esperado %s, veio %s", i, esperado[i], ordem[i])
		}
	}

	// Repetir o rodízio conflita e não grava nada (tudo ou nada)
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/plantoes/rodizio", adminCookie, rodizio); resp.StatusCode != http.StatusConflict {
		t.Errorf("esperado 409 ao repetir o rodízio, veio %d", resp.StatusCode)
	}
	if n := len(env.getLista(t, "/api/plantoes?inicio="+dia(30)+"&fim="+dia(57), anaCookie)); n != 4 {
		t.Errorf("rodízio conflitante não deveria gravar nada; turnos no período: %d", n)
	}

	// Editar: trocar a pessoa do turno de hoje; excluir
	var idHoje string
	for _, p := range env.getLista(t, "/api/plantoes?inicio="+dia(0)+"&fim="+dia(0), anaCookie) {
		if p["usuario_id"] == anaID && p["tipo"] == "plantao" {
			idHoje = p["id"].(string)
		}
	}
	troca := map[string]any{"usuario_id": beaID, "tipo": "plantao", "inicio": dia(0), "fim": dia(1), "observacao": "troca com Ana"}
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
