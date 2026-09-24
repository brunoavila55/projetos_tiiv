package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// IPReal troca r.RemoteAddr pelo IP do cliente informado em X-Forwarded-For,
// mas só quando a conexão vem de um proxy confiável. Percorre a lista da
// direita para a esquerda e para no primeiro IP que não é proxy: o que estiver
// antes dele pode ter sido escrito pelo próprio cliente.
// Diferente de chimiddleware.RealIP, que aceita o cabeçalho de qualquer um.
func IPReal(confiaveis []netip.Prefix) func(http.Handler) http.Handler {
	confiavel := func(a netip.Addr) bool {
		a = a.Unmap()
		for _, p := range confiaveis {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		if len(confiaveis) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			origem, err := netip.ParseAddr(host)
			if err != nil || !confiavel(origem) {
				next.ServeHTTP(w, r)
				return
			}

			ips := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
			for i := len(ips) - 1; i >= 0; i-- {
				a, err := netip.ParseAddr(strings.TrimSpace(ips[i]))
				if err != nil {
					break
				}
				if !confiavel(a) {
					r.RemoteAddr = net.JoinHostPort(a.Unmap().String(), "0")
					break
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
