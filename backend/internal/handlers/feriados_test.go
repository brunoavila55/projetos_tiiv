package handlers

import (
	"testing"
	"time"
)

func TestPascoa(t *testing.T) {
	for ano, esperado := range map[int]string{2024: "2024-03-31", 2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := pascoa(ano).Format(formatoDia); got != esperado {
			t.Errorf("Páscoa de %d: esperado %s, veio %s", ano, esperado, got)
		}
	}
}

func TestDiaDePlantaoInterno(t *testing.T) {
	dia := func(s string) time.Time { d, _ := time.Parse(formatoDia, s); return d }
	for s, esperado := range map[string]bool{
		"2026-09-27": true,  // domingo
		"2026-09-28": false, // segunda
		"2026-09-07": true,  // Independência
		"2026-09-21": false, // segunda depois do 20 de setembro
		"2026-04-03": true,  // Sexta-feira Santa
		"2026-11-20": true,  // Consciência Negra
		"2026-02-17": false, // Carnaval é ponto facultativo
	} {
		if got := diaDePlantaoInterno(dia(s)); got != esperado {
			t.Errorf("%s: esperado %v, veio %v", s, esperado, got)
		}
	}
	if n := len(feriadosEntre(dia("2026-01-01"), dia("2026-12-31"))); n != 11 {
		t.Errorf("esperados 11 feriados em 2026, veio %d", n)
	}
}
