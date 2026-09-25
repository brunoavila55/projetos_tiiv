// Package monitor verifica a disponibilidade dos alvos cadastrados (ping, porta
// TCP ou URL HTTP) e mantém o estado de cada monitor e o histórico de quedas.
package monitor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	TipoPing = "ping"
	TipoTCP  = "tcp"
	TipoHTTP = "http"
)

// Tempo máximo de uma verificação, qualquer que seja o tipo
const tempoLimiteVerificacao = 10 * time.Second

// ValidarAlvo normaliza o alvo e confere se ele faz sentido para o tipo:
// ping recebe host ou IP, tcp recebe host:porta e http recebe uma URL http(s).
func ValidarAlvo(tipo, alvo string) (string, error) {
	alvo = strings.TrimSpace(alvo)
	if alvo == "" {
		return "", errors.New("alvo é obrigatório")
	}
	if utf8.RuneCountInString(alvo) > 500 {
		return "", errors.New("alvo deve ter no máximo 500 caracteres")
	}

	switch tipo {
	case TipoPing:
		if !hostValido(alvo) {
			return "", errors.New("para ping, informe só o host ou IP (ex.: 192.168.0.1)")
		}
	case TipoTCP:
		host, porta, err := net.SplitHostPort(alvo)
		if err != nil || !hostValido(host) {
			return "", errors.New("para porta TCP, informe host:porta (ex.: 192.168.0.10:3389)")
		}
		if n, err := strconv.Atoi(porta); err != nil || n < 1 || n > 65535 {
			return "", errors.New("porta TCP inválida")
		}
	case TipoHTTP:
		u, err := url.Parse(alvo)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return "", errors.New("para HTTP, informe a URL completa (ex.: http://intranet/)")
		}
	default:
		return "", errors.New("tipo de monitor inválido")
	}
	return alvo, nil
}

func hostValido(h string) bool {
	return h != "" && len(h) <= 253 && !strings.ContainsAny(h, " /\\?#@")
}

// Verificar faz uma checagem do alvo e devolve a latência ou o motivo da falha
func Verificar(ctx context.Context, tipo, alvo string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, tempoLimiteVerificacao)
	defer cancel()

	var (
		latencia time.Duration
		err      error
	)
	switch tipo {
	case TipoPing:
		latencia, err = verificarPing(ctx, alvo)
	case TipoTCP:
		latencia, err = verificarTCP(ctx, alvo)
	case TipoHTTP:
		latencia, err = verificarHTTP(ctx, alvo)
	default:
		err = errors.New("tipo de monitor inválido")
	}
	return latencia, traduzirErro(err)
}

func verificarTCP(ctx context.Context, alvo string) (time.Duration, error) {
	var d net.Dialer
	inicio := time.Now()
	conn, err := d.DialContext(ctx, "tcp", alvo)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(inicio), nil
}

// Muitos serviços internos usam certificado próprio: aqui só interessa se o
// serviço responde, então o certificado não é validado e nada é enviado.
var clienteHTTP = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		DisableKeepAlives: true,
		Proxy:             nil,
	},
}

func verificarHTTP(ctx context.Context, alvo string) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, alvo, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "TIIV-Monitor/1.0")

	inicio := time.Now()
	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 64<<10)
	latencia := time.Since(inicio)

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("respondeu HTTP %d", resp.StatusCode)
	}
	return latencia, nil
}

// verificarPing usa ICMP sem privilégio (socket "udp4"), permitido pelo
// net.ipv4.ping_group_range, que o Docker libera por padrão nos containers.
// Três tentativas; basta uma resposta.
func verificarPing(ctx context.Context, host string) (time.Duration, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return 0, err
	}
	if len(ips) == 0 {
		return 0, errors.New("host sem endereço IPv4")
	}
	ip := ips[0]

	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return 0, fmt.Errorf("ping indisponível neste servidor: %w", err)
	}
	defer conn.Close()

	buf := make([]byte, 1500)
	for seq := 1; seq <= 3; seq++ {
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Body: &icmp.Echo{ID: 1, Seq: seq, Data: []byte("tiiv-monitor")},
		}
		pacote, err := msg.Marshal(nil)
		if err != nil {
			return 0, err
		}

		inicio := time.Now()
		if _, err := conn.WriteTo(pacote, &net.UDPAddr{IP: ip}); err != nil {
			return 0, err
		}
		prazo := inicio.Add(2 * time.Second)
		if limite, ok := ctx.Deadline(); ok && limite.Before(prazo) {
			prazo = limite
		}
		_ = conn.SetReadDeadline(prazo)

		for {
			n, origem, err := conn.ReadFrom(buf)
			if err != nil {
				break // tempo esgotado: próxima tentativa
			}
			resp, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), buf[:n])
			if err != nil || resp.Type != ipv4.ICMPTypeEchoReply {
				continue
			}
			eco, ok := resp.Body.(*icmp.Echo)
			udp, okAddr := origem.(*net.UDPAddr)
			if ok && okAddr && eco.Seq == seq && udp.IP.Equal(ip) {
				return time.Since(inicio), nil
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return 0, errors.New("sem resposta ao ping")
}

// traduzirErro troca as mensagens de rede mais comuns por texto legível
func traduzirErro(err error) error {
	if err == nil {
		return nil
	}
	var dnsErr *net.DNSError
	var netErr net.Error
	switch {
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return errors.New("nome não encontrado no DNS")
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return errors.New("sem resposta (tempo esgotado)")
	case errors.Is(err, syscall.ECONNREFUSED):
		return errors.New("conexão recusada")
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return errors.New("host inalcançável")
	}
	msg := err.Error()
	if utf8.RuneCountInString(msg) > 300 {
		msg = string([]rune(msg)[:300])
	}
	return errors.New(msg)
}
