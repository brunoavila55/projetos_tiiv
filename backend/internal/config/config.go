package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	SessionTTL    time.Duration
	SessionMaxTTL time.Duration
	CookieSecure  bool
	AdminNome     string
	AdminPIN      string

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
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tiiv?sslmode=disable")
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
	adminPIN := getEnv("ADMIN_PIN", "1234")

	return &Config{
		DatabaseURL:   dbURL,
		ListenAddr:    listenAddr,
		SessionTTL:    sessionTTL,
		SessionMaxTTL: sessionMaxTTL,
		CookieSecure:  cookieSecure,
		AdminNome:     adminNome,
		AdminPIN:      adminPIN,

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
