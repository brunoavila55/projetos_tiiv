package handlers

import "testing"

func TestCelulaCSV(t *testing.T) {
	casos := map[string]string{
		"=1+1":               "'=1+1",
		"+5":                 "'+5",
		"-2+3":               "'-2+3",
		"@SUM(A1)":           "'@SUM(A1)",
		"\t=cmd":             "'\t=cmd",
		"\r=cmd":             "'\r=cmd",
		"Reposição de cabos": "Reposição de cabos",
		"":                   "",
		"a=b":                "a=b",
	}
	for entrada, esperado := range casos {
		if got := celulaCSV(entrada); got != esperado {
			t.Errorf("celulaCSV(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
}
