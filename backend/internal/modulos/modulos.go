// Package modulos lista as funcionalidades que o superadmin liga e desliga por
// setor. Painel, operadores e setores não entram: fazem parte do núcleo.
package modulos

import (
	"fmt"
	"slices"
)

const (
	Tickets     = "tickets"
	Tarefas     = "tarefas"
	Calendario  = "calendario"
	Plantao     = "plantao"
	Estoque     = "estoque"
	Tecnicos    = "tecnicos"
	Monitor     = "monitor"
	Links       = "links"
	Avisos      = "avisos"
	TiraDuvidas = "tira_duvidas"
	TV          = "tv"
	Radio       = "radio"
)

// Todos, na ordem em que aparecem no menu e na tela de setores
var Todos = []string{
	Tickets, Tarefas, Calendario, Plantao, Estoque, Tecnicos,
	Monitor, Links, Avisos, TiraDuvidas, TV, Radio,
}

// dependencias: o módulo só funciona com os listados ligados (resgatar um
// ticket cria uma tarefa)
var dependencias = map[string][]string{
	Tickets: {Tarefas},
}

func Existe(m string) bool {
	return slices.Contains(Todos, m)
}

// Ativos devolve os módulos que não estão na lista de desativados
func Ativos(desativados []string) []string {
	ativos := make([]string, 0, len(Todos))
	for _, m := range Todos {
		if !slices.Contains(desativados, m) {
			ativos = append(ativos, m)
		}
	}
	return ativos
}

// Validar confere a lista de desativados e devolve ela normalizada (sem
// repetidos, na ordem do catálogo)
func Validar(desativados []string) ([]string, error) {
	for _, m := range desativados {
		if !Existe(m) {
			return nil, fmt.Errorf("módulo desconhecido: %s", m)
		}
	}
	normalizada := make([]string, 0, len(desativados))
	for _, m := range Todos {
		if slices.Contains(desativados, m) {
			normalizada = append(normalizada, m)
		}
	}
	for m, precisa := range dependencias {
		if slices.Contains(normalizada, m) {
			continue
		}
		for _, p := range precisa {
			if slices.Contains(normalizada, p) {
				return nil, fmt.Errorf("o módulo %s depende de %s: desligue os dois ou mantenha %s ligado", m, p, p)
			}
		}
	}
	return normalizada, nil
}
