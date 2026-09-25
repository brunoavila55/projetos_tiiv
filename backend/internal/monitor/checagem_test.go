package monitor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidarAlvo(t *testing.T) {
	casos := []struct {
		tipo, alvo string
		ok         bool
	}{
		{TipoPing, "192.168.0.1", true},
		{TipoPing, " servidor.local ", true},
		{TipoPing, "http://servidor", false},
		{TipoPing, "", false},
		{TipoTCP, "10.0.0.5:3389", true},
		{TipoTCP, "[::1]:22", true},
		{TipoTCP, "10.0.0.5", false},
		{TipoTCP, "10.0.0.5:70000", false},
		{TipoHTTP, "https://intranet/status", true},
		{TipoHTTP, "intranet", false},
		{TipoHTTP, "ftp://intranet", false},
		{TipoHTTP, "javascript:alert(1)", false},
		{"dns", "8.8.8.8", false},
	}
	for _, c := range casos {
		_, err := ValidarAlvo(c.tipo, c.alvo)
		if (err == nil) != c.ok {
			t.Errorf("ValidarAlvo(%q, %q): esperado ok=%v, erro=%v", c.tipo, c.alvo, c.ok, err)
		}
	}
}

func TestProximoEstado(t *testing.T) {
	// Uma falha isolada não derruba quem estava online
	if s, f := ProximoEstado(StatusOnline, 0, false); s != StatusOnline || f != 1 {
		t.Errorf("1ª falha: veio %s/%d", s, f)
	}
	if s, f := ProximoEstado(StatusOnline, 1, false); s != StatusOffline || f != 2 {
		t.Errorf("2ª falha: veio %s/%d", s, f)
	}
	if s, f := ProximoEstado(StatusOffline, 5, true); s != StatusOnline || f != 0 {
		t.Errorf("volta: veio %s/%d", s, f)
	}
	if s, _ := ProximoEstado(StatusPendente, 0, false); s != StatusPendente {
		t.Errorf("pendente com 1 falha deveria continuar pendente, veio %s", s)
	}
}

func TestVerificarHTTPeTCP(t *testing.T) {
	ctx := context.Background()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ok.Close()
	if _, err := Verificar(ctx, TipoHTTP, ok.URL); err != nil {
		t.Errorf("HTTP 200 deveria passar: %v", err)
	}
	if _, err := Verificar(ctx, TipoTCP, ok.Listener.Addr().String()); err != nil {
		t.Errorf("porta aberta deveria passar: %v", err)
	}

	erro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer erro.Close()
	if _, err := Verificar(ctx, TipoHTTP, erro.URL); err == nil || err.Error() != "respondeu HTTP 503" {
		t.Errorf("HTTP 503 deveria falhar com mensagem clara, veio %v", err)
	}

	// Porta fechada: reserva uma porta livre e solta
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	fechada := l.Addr().String()
	l.Close()
	if _, err := Verificar(ctx, TipoTCP, fechada); err == nil || err.Error() != "conexão recusada" {
		t.Errorf("porta fechada deveria dar conexão recusada, veio %v", err)
	}
}

func TestVerificarPingLocal(t *testing.T) {
	if _, err := Verificar(context.Background(), TipoPing, "127.0.0.1"); err != nil {
		t.Skipf("ICMP sem privilégio indisponível neste ambiente: %v", err)
	}
}
