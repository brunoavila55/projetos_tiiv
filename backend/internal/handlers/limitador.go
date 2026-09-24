package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// limitadorPorIP limita quantas ações cada IP faz numa janela deslizante;
// usado nas rotas públicas (sem login) da tela de acesso.
type limitadorPorIP struct {
	max    int
	janela time.Duration

	mu     sync.Mutex
	envios map[string][]time.Time
}

func novoLimitadorPorIP(max int, janela time.Duration) *limitadorPorIP {
	return &limitadorPorIP{max: max, janela: janela, envios: map[string][]time.Time{}}
}

// permitir registra uma ação do IP e diz se ainda está dentro do limite
func (l *limitadorPorIP) permitir(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	agora := time.Now()
	recentes := l.envios[ip][:0]
	for _, t := range l.envios[ip] {
		if agora.Sub(t) < l.janela {
			recentes = append(recentes, t)
		}
	}
	if len(recentes) >= l.max {
		l.envios[ip] = recentes
		return false
	}
	l.envios[ip] = append(recentes, agora)

	// Evita crescer sem limite com IPs que não voltam
	if len(l.envios) > 1000 {
		for k, v := range l.envios {
			if len(v) == 0 || agora.Sub(v[len(v)-1]) >= l.janela {
				delete(l.envios, k)
			}
		}
	}
	return true
}

func ipDaRequisicao(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
