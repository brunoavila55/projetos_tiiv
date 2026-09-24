package handlers

import (
	"reflect"
	"strings"
	"testing"

	"tiiv/backend/internal/database/sqlc"
)

func TestExtrairContato(t *testing.T) {
	casos := []struct {
		nome  string
		corpo string
		quer  string // "" = sem seção
	}{
		{
			nome:  "seção até o próximo título",
			corpo: "## O que fazer\n1. Ligue\n\n## Contato\n**Álvaro** · (11) 99999-0000\n\n## Quando escalar\nDepois de 15 min",
			quer:  "**Álvaro** · (11) 99999-0000",
		},
		{
			nome:  "última seção, com subtítulo e plural",
			corpo: "## Contatos\n- Álvaro: ramal 214\n### Fora do horário\n- Plantão: 0800",
			quer:  "- Álvaro: ramal 214\n### Fora do horário\n- Plantão: 0800",
		},
		{
			nome:  "sem seção de contato",
			corpo: "## O que fazer\nReinicie o roteador",
		},
		{
			nome:  "seção vazia",
			corpo: "## Contato\n\n## Outra",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := extrairContato(c.corpo)
			if c.quer == "" {
				if got != nil {
					t.Fatalf("esperava nil, veio %q", *got)
				}
				return
			}
			if got == nil || *got != c.quer {
				t.Fatalf("esperava %q, veio %v", c.quer, got)
			}
		})
	}
}

func TestSepararFontes(t *testing.T) {
	casos := []struct {
		entrada string
		texto   string
		indices []int
	}{
		{"Ligue para o Álvaro.\nFONTES: 2", "Ligue para o Álvaro.", []int{2}},
		{"Faça X.\n\n**FONTES:** 1, 3, 1", "Faça X.", []int{1, 3}},
		{"Não encontrei.\nFONTES: nenhum", "Não encontrei.", nil},
		{"Sem a linha final", "Sem a linha final", nil},
	}
	for _, c := range casos {
		texto, indices := separarFontes(c.entrada)
		if texto != c.texto || !reflect.DeepEqual(indices, c.indices) {
			t.Errorf("separarFontes(%q) = %q, %v; esperado %q, %v", c.entrada, texto, indices, c.texto, c.indices)
		}
	}
}

func TestLimparPensamento(t *testing.T) {
	if got := limparPensamento("<think>\n\n</think>\n\nResposta"); got != "Resposta" {
		t.Errorf("bloco vazio: %q", got)
	}
	if got := limparPensamento("raciocínio solto</think>Resposta"); got != "Resposta" {
		t.Errorf("sem abertura: %q", got)
	}
}

func TestMontarMensagensLLM(t *testing.T) {
	var msgs []MensagemConversa
	for i := 0; i < 9; i++ {
		papel := "usuario"
		if i%2 == 1 {
			papel = "assistente"
		}
		msgs = append(msgs, MensagemConversa{Papel: papel, Texto: "m"})
	}
	procs := []sqlc.BuscarProcedimentosRelevantesRow{{Titulo: "Servidor fora do ar", Categoria: "Infra", Corpo: "Ligue pro Álvaro"}}

	out := montarMensagensLLM(msgs, procs)
	if len(out) != 1+mensagensEnviadasAoLLM {
		t.Fatalf("esperava sistema + %d mensagens, veio %d", mensagensEnviadasAoLLM, len(out))
	}
	if !strings.Contains(out[0].Content, "[1] Servidor fora do ar (Infra)\nLigue pro Álvaro") {
		t.Errorf("procedimento fora do prompt de sistema: %q", out[0].Content)
	}
	if ultima := out[len(out)-1]; ultima.Role != "user" || !strings.HasSuffix(ultima.Content, "/no_think") {
		t.Errorf("última mensagem deveria ser do usuário com /no_think: %+v", ultima)
	}
	if out[len(out)-2].Role != "assistant" {
		t.Errorf("papel assistente não foi convertido: %+v", out[len(out)-2])
	}
}
