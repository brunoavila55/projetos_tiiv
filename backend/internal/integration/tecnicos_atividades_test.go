package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// O que o técnico fez: anotado durante a visita por qualquer operador,
// revisado na saída, listado nos registros e exportado no CSV.
func TestTecnicos_OQueFoiFeito(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	adminCookie, _ := env.loginAdmin(t)
	opID := env.criarOperador(t, adminCookie, "Op Atividades", "8989")
	opCookie, _, _ := env.login(t, opID, "8989")

	nome := fmt.Sprintf("Técnico Atividades %d", time.Now().UnixNano())
	_, res, _ := env.doRequest(http.MethodPost, "/api/tecnicos", opCookie, map[string]string{"nome": nome})
	tecID := res["id"].(string)
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnico_registros WHERE tecnico_id = $1", tecID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnicos WHERE id = $1", tecID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", opID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", opID)
	}()

	_, res, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/entrada", opCookie, nil)
	regID := res["id"].(string)

	tecnico := func() map[string]any {
		for _, tc := range env.getLista(t, "/api/tecnicos", opCookie) {
			if tc["id"] == tecID {
				return tc
			}
		}
		t.Fatal("técnico não encontrado na lista")
		return nil
	}

	// Operador (não admin) anota durante a visita
	anotado := "Troca do switch do rack 2"
	resp, _, _ := env.doRequest(http.MethodPatch, "/api/tecnicos/registros/"+regID+"/atividades", opCookie, map[string]string{"atividades": "  " + anotado + "  "})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200 ao anotar atividades, veio %d", resp.StatusCode)
	}
	if got := tecnico()["atividades_abertas"]; got != anotado {
		t.Errorf("atividades da visita em aberto: esperado %q, veio %q", anotado, got)
	}

	// Texto grande demais e registro inexistente
	if resp, _, _ := env.doRequest(http.MethodPatch, "/api/tecnicos/registros/"+regID+"/atividades", opCookie, map[string]string{"atividades": strings.Repeat("a", 4001)}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperado 400 para texto acima do limite, veio %d", resp.StatusCode)
	}
	if resp, _, _ := env.doRequest(http.MethodPatch, "/api/tecnicos/registros/00000000-0000-0000-0000-000000000000/atividades", opCookie, map[string]string{"atividades": "x"}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("esperado 404 para registro inexistente, veio %d", resp.StatusCode)
	}

	// Na saída, o texto revisado substitui o anotado
	final := anotado + "\nCabeamento do andar 3 organizado"
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", opCookie, map[string]string{"atividades": final}); resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200 na saída, veio %d", resp.StatusCode)
	}
	if got := tecnico()["atividades_abertas"]; got != "" {
		t.Errorf("sem visita aberta, atividades_abertas deveria vir vazio, veio %q", got)
	}

	_, lista, _ := env.doRequest(http.MethodGet, "/api/tecnicos/registros?tecnico_id="+tecID, opCookie, nil)
	itens := lista["itens"].([]any)
	if len(itens) != 1 || itens[0].(map[string]any)["atividades"] != final {
		t.Errorf("registro deveria trazer o que foi feito, veio %v", itens)
	}

	// Saída sem atividades não apaga o que já estava anotado
	env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/entrada", opCookie, nil)
	_, res2, _ := env.doRequest(http.MethodGet, "/api/tecnicos/registros?tecnico_id="+tecID, opCookie, nil)
	reg2 := res2["itens"].([]any)[0].(map[string]any)["id"].(string)
	env.doRequest(http.MethodPatch, "/api/tecnicos/registros/"+reg2+"/atividades", opCookie, map[string]string{"atividades": "Vistoria"})
	env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", opCookie, nil)
	_, res2, _ = env.doRequest(http.MethodGet, "/api/tecnicos/registros?tecnico_id="+tecID, opCookie, nil)
	if got := res2["itens"].([]any)[0].(map[string]any)["atividades"]; got != "Vistoria" {
		t.Errorf("saída sem texto não deveria apagar o anotado, veio %q", got)
	}

	// CSV
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/tecnicos/registros/exportar.csv?tecnico_id="+tecID, nil)
	req.AddCookie(opCookie)
	respCSV, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("falha ao exportar CSV: %v", err)
	}
	defer respCSV.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(respCSV.Body)
	if !strings.Contains(buf.String(), "O que foi feito") || !strings.Contains(buf.String(), "Cabeamento do andar 3") {
		t.Errorf("CSV deveria ter a coluna e o texto do que foi feito: %s", buf.String())
	}
}
