package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	ListenAddr     string
	SessionTTL     time.Duration
	SessionMaxTTL  time.Duration
	CookieSecure   bool
	AdminNome      string
	AdminPIN       string
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
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
