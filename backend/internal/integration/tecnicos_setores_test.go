package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Técnicos por setor: cada setor só vê e marca os próprios técnicos, os
// registros e o relatório não vazam, e o mesmo nome pode existir nos dois.
func TestTecnicos_PorSetor(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()
	ctx := context.Background()

	superCookie, _ := env.loginAdmin(t)
	nocOpID := env.criarOperador(t, superCookie, "Op NOC Tec", "8686")
	nocCookie, _, _ := env.login(t, nocOpID, "8686")

	resp, res, _ := env.doRequest(http.MethodPost, "/api/setores", superCookie, map[string]any{
		"nome": fmt.Sprintf("Manutenção %d", time.Now().UnixNano()),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar setor: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	setorID := res["id"].(string)
	resp, res, _ = env.doRequest(http.MethodPost, "/api/usuarios", superCookie, map[string]any{
		"nome": fmt.Sprintf("Admin Man %d", time.Now().UnixNano()), "cor": "#10B981", "pin": "8787",
		"papel": "admin", "setor_id": setorID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar admin do setor: status %d (%v)", resp.StatusCode, res)
	}
	outroAdminID := res["id"].(string)
	outroCookie, _, _ := env.login(t, outroAdminID, "8787")

	nome := fmt.Sprintf("Técnico Setor %d", time.Now().UnixNano())
	defer func() {
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnico_registros WHERE tecnico_id IN (SELECT id FROM tecnicos WHERE nome = $1)", nome)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM tecnicos WHERE nome = $1", nome)
		for _, id := range []string{nocOpID, outroAdminID} {
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM sessoes WHERE usuario_id = $1", id)
			_, _ = env.db.Pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id)
		}
		_, _ = env.db.Pool.Exec(ctx, "UPDATE sessoes SET setor_id = NULL WHERE setor_id = $1", setorID)
		_, _ = env.db.Pool.Exec(ctx, "DELETE FROM setores WHERE id = $1", setorID)
	}()

	// O NOC cadastra e dá entrada no técnico
	resp, res, _ = env.doRequest(http.MethodPost, "/api/tecnicos", nocCookie, map[string]string{"nome": nome})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar técnico no NOC: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	tecID := res["id"].(string)
	resp, res, _ = env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/entrada", nocCookie, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("entrada no NOC: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	regID := res["id"].(string)

	// O outro setor não vê o técnico, os registros nem o relatório
	temTecnico := func(cookie *http.Cookie) bool {
		for _, tc := range env.getLista(t, "/api/tecnicos", cookie) {
			if tc["id"] == tecID {
				return true
			}
		}
		return false
	}
	if !temTecnico(nocCookie) {
		t.Error("o NOC deveria ver o próprio técnico")
	}
	if temTecnico(outroCookie) {
		t.Error("o outro setor não deveria ver o técnico do NOC")
	}
	_, regs, _ := env.doRequest(http.MethodGet, "/api/tecnicos/registros?tecnico_id="+tecID, outroCookie, nil)
	if total, _ := regs["total"].(float64); total != 0 {
		t.Errorf("o outro setor não deveria ver registros do técnico do NOC, veio %v", regs)
	}
	if rel := env.getLista(t, "/api/tecnicos/relatorio?tecnico_id="+tecID, outroCookie); len(rel) != 0 {
		t.Errorf("o relatório do outro setor não deveria trazer o técnico do NOC, veio %v", rel)
	}

	// Nem mexe nele: editar, marcar, anotar, corrigir e excluir registro
	for _, c := range []struct {
		metodo, rota string
		corpo        any
	}{
		{http.MethodPut, "/api/tecnicos/" + tecID, map[string]string{"nome": "Outro nome"}},
		{http.MethodPost, "/api/tecnicos/" + tecID + "/entrada", nil},
		{http.MethodPatch, "/api/tecnicos/registros/" + regID + "/atividades", map[string]string{"atividades": "invasão"}},
		{http.MethodPut, "/api/tecnicos/registros/" + regID, map[string]any{"entrada": time.Now().Add(-time.Hour).Format(time.RFC3339)}},
		{http.MethodDelete, "/api/tecnicos/registros/" + regID, nil},
	} {
		if resp, res, _ := env.doRequest(c.metodo, c.rota, outroCookie, c.corpo); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s pelo outro setor: esperado 404, veio %d (%v)", c.metodo, c.rota, resp.StatusCode, res)
		}
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", outroCookie, nil); resp.StatusCode == http.StatusOK {
		t.Error("o outro setor não deveria marcar a saída do técnico do NOC")
	}

	// A entrada continua aberta no NOC, que marca a saída normalmente
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/tecnicos/"+tecID+"/saida", nocCookie, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("saída no NOC: esperado 200, veio %d (%v)", resp.StatusCode, res)
	}

	// Mesmo nome no outro setor pode; repetido no mesmo setor, não
	if resp, res, _ := env.doRequest(http.MethodPost, "/api/tecnicos", outroCookie, map[string]string{"nome": nome}); resp.StatusCode != http.StatusCreated {
		t.Errorf("mesmo nome em outro setor: esperado 201, veio %d (%v)", resp.StatusCode, res)
	}
	if resp, _, _ := env.doRequest(http.MethodPost, "/api/tecnicos", nocCookie, map[string]string{"nome": nome}); resp.StatusCode != http.StatusConflict {
		t.Errorf("nome repetido no mesmo setor: esperado 409, veio %d", resp.StatusCode)
	}
}
