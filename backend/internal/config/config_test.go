package config

import (
	"strings"
	"testing"
)

func TestPinTrivial(t *testing.T) {
	for _, pin := range []string{"0000", "1111", "1234", "4321", "6789", "9012", "3210", "1212", "2580"} {
		if !PinTrivial(pin) {
			t.Errorf("PinTrivial(%q) = false, esperado true", pin)
		}
	}
	for _, pin := range []string{"4817", "7391", "1357", "0582"} {
		if PinTrivial(pin) {
			t.Errorf("PinTrivial(%q) = true, esperado false", pin)
		}
	}
}

func TestValidate(t *testing.T) {
	boa := "postgres://tiiv_app:3f9c1a7e5b2d4c6f8a0e@postgres:5432/tiiv?sslmode=disable"

	casos := []struct {
		nome   string
		cfg    Config
		trecho string // vazio = deve passar
	}{
		{"ok", Config{DatabaseURL: boa, AdminPIN: "4817"}, ""},
		{"ok sem ADMIN_PIN", Config{DatabaseURL: boa}, ""},
		{"sem DATABASE_URL", Config{}, "DATABASE_URL não definido"},
		{"senha postgres", Config{DatabaseURL: "postgres://tiiv_app:postgres@db/tiiv"}, "senha do banco"},
		{"senha curta", Config{DatabaseURL: "postgres://tiiv_app:abc123@db/tiiv"}, "senha do banco"},
		{"superusuário", Config{DatabaseURL: "postgres://postgres:3f9c1a7e5b2d4c6f8a0e@db/tiiv"}, "superusuário"},
		{"PIN 1234", Config{DatabaseURL: boa, AdminPIN: "1234"}, "ADMIN_PIN"},
	}
	for _, c := range casos {
		err := c.cfg.Validate()
		if c.trecho == "" {
			if err != nil {
				t.Errorf("%s: erro inesperado: %v", c.nome, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.trecho) {
			t.Errorf("%s: esperado erro contendo %q, obteve %v", c.nome, c.trecho, err)
		}
	}
}
