package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	SessionTTL    time.Duration
	SessionMaxTTL time.Duration
	CookieSecure  bool
	AdminNome     string
	AdminPIN      string // vazio: o bootstrap gera um PIN aleatório

	// Falhas de login aceitas por IP em 15 minutos. Fica abaixo das 5 que
	// bloqueiam a conta, para um único host não conseguir travar ninguém.
	LoginFalhasPorIP int

	// Proxies reversos (IPs ou CIDRs) cujo X-Forwarded-For é aceito, como o
	// Caddy do perfil tls. Vazio: o IP é sempre o da conexão TCP.
	TrustedProxies string

	// Tira-dúvidas (Cloudflare Workers AI). Sem conta/token, fica desligado.
	CFAccountID          string
	CFAPIToken           string
	CFAIModel            string
	CFAPIBaseURL         string
	NeuronsEntradaPorM   float64
	NeuronsSaidaPorM     float64
	AssistenteNeuronsDia float64
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	listenAddr := getEnv("LISTEN_ADDR", ":8080")

	sessionTTLStr := getEnv("SESSION_TTL", "30m")
	sessionTTL, err := time.ParseDuration(sessionTTLStr)
	if err != nil {
		sessionTTL = 30 * time.Minute
	}

	sessionMaxTTLStr := getEnv("SESSION_MAX_TTL", "12h")
	sessionMaxTTL, err := time.ParseDuration(sessionMaxTTLStr)
	if err != nil {
		sessionMaxTTL = 12 * time.Hour
	}

	cookieSecureStr := getEnv("COOKIE_SECURE", "false")
	cookieSecure, _ := strconv.ParseBool(cookieSecureStr)

	adminNome := getEnv("ADMIN_NOME", "Administrador")
	adminPIN := os.Getenv("ADMIN_PIN")

	return &Config{
		DatabaseURL:   dbURL,
		ListenAddr:    listenAddr,
		SessionTTL:    sessionTTL,
		SessionMaxTTL: sessionMaxTTL,
		CookieSecure:  cookieSecure,
		AdminNome:     adminNome,
		AdminPIN:      adminPIN,

		LoginFalhasPorIP: int(getEnvFloat("LOGIN_FALHAS_POR_IP", 4)),
		TrustedProxies:   os.Getenv("TRUSTED_PROXIES"),

		CFAccountID: os.Getenv("CF_ACCOUNT_ID"),
		CFAPIToken:  os.Getenv("CF_API_TOKEN"),
		CFAIModel:   getEnv("CF_AI_MODEL", "@cf/qwen/qwen3-30b-a3b-fp8"),
		// Trocado nos testes por um servidor falso
		CFAPIBaseURL: getEnv("CF_API_BASE_URL", "https://api.cloudflare.com/client/v4"),
		// Preço em neurons do modelo padrão; ajuste junto se trocar CF_AI_MODEL
		NeuronsEntradaPorM: getEnvFloat("CF_AI_NEURONS_ENTRADA_M", 4625),
		NeuronsSaidaPorM:   getEnvFloat("CF_AI_NEURONS_SAIDA_M", 30475),
		// Abaixo dos 10.000/dia grátis, com folga para a estimativa
		AssistenteNeuronsDia: getEnvFloat("ASSISTENTE_NEURONS_DIA", 9500),
	}
}

// senhasFracas são recusadas no DATABASE_URL: estão em exemplos, READMEs e dicionários
var senhasFracas = map[string]bool{
	"": true, "postgres": true, "password": true, "senha": true, "admin": true,
	"tiiv": true, "tiiv_app": true, "123456": true, "12345678": true, "root": true,
	"troque-por-uma-senha-forte": true,
}

// Validate recusa subir com segredos padrão ou triviais.
func (c *Config) Validate() error {
	var erros []string

	if c.DatabaseURL == "" {
		erros = append(erros, "DATABASE_URL não definido")
	} else if u, err := url.Parse(c.DatabaseURL); err == nil && u.User != nil {
		senha, _ := u.User.Password()
		if senhasFracas[strings.ToLower(senha)] || len(senha) < 12 {
			erros = append(erros, "a senha do banco em DATABASE_URL é padrão ou curta demais (mínimo 12 caracteres); gere uma com `openssl rand -hex 24`")
		}
		if u.User.Username() == "postgres" {
			erros = append(erros, "DATABASE_URL usa o superusuário postgres; use o papel do app (tiiv_app), veja o README")
		}
	}

	if c.AdminPIN != "" && PinTrivial(c.AdminPIN) {
		erros = append(erros, fmt.Sprintf("ADMIN_PIN %q é trivial (repetido, sequência ou comum); escolha outro ou deixe vazio para gerar um aleatório", c.AdminPIN))
	}

	if _, err := c.ProxiesConfiaveis(); err != nil {
		erros = append(erros, err.Error())
	}

	if len(erros) > 0 {
		return errors.New("configuração insegura:\n  - " + strings.Join(erros, "\n  - "))
	}
	return nil
}

// ProxiesConfiaveis interpreta TRUSTED_PROXIES (lista separada por vírgula)
func (c *Config) ProxiesConfiaveis() ([]netip.Prefix, error) {
	var prefixos []netip.Prefix
	for _, item := range strings.Split(c.TrustedProxies, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			addr, err := netip.ParseAddr(item)
			if err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES: %q não é IP nem CIDR", item)
			}
			prefixos = append(prefixos, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q não é IP nem CIDR", item)
		}
		prefixos = append(prefixos, p.Masked())
	}
	return prefixos, nil
}

// PinTrivial diz se o PIN é fácil de adivinhar: todos os dígitos iguais,
// sequência crescente/decrescente ou um dos mais usados.
func PinTrivial(pin string) bool {
	switch pin {
	case "1212", "1122", "6969", "2580", "0852", "1004", "2000", "4321", "1010", "2001":
		return true
	}
	iguais, sobe, desce := true, true, true
	for i := 1; i < len(pin); i++ {
		d := int(pin[i]) - int(pin[i-1])
		iguais = iguais && d == 0
		sobe = sobe && (d == 1 || d == -9)
		desce = desce && (d == -1 || d == 9)
	}
	return iguais || sobe || desce
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && v > 0 {
		return v
	}
	return defaultVal
}
