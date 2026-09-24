package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIPReal(t *testing.T) {
	proxy := []netip.Prefix{netip.MustParsePrefix("172.28.0.10/32")}

	casos := []struct {
		nome, remote, xff, esperado string
		confiaveis                  []netip.Prefix
	}{
		{"sem proxies configurados ignora XFF", "10.0.0.5:1234", "1.2.3.4", "10.0.0.5:1234", nil},
		{"cliente direto não escolhe o IP", "10.0.0.5:1234", "1.2.3.4", "10.0.0.5:1234", proxy},
		{"via proxy usa o IP que ele anexou", "172.28.0.10:5555", "1.2.3.4, 192.168.0.20", "192.168.0.20:0", proxy},
		{"via proxy sem XFF mantém", "172.28.0.10:5555", "", "172.28.0.10:5555", proxy},
		{"lixo no XFF mantém o proxy", "172.28.0.10:5555", "abc", "172.28.0.10:5555", proxy},
	}
	for _, c := range casos {
		var visto string
		h := IPReal(c.confiaveis)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visto = r.RemoteAddr }))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if visto != c.esperado {
			t.Errorf("%s: RemoteAddr = %q, esperado %q", c.nome, visto, c.esperado)
		}
	}
}
