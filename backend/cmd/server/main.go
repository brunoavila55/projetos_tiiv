package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/routes"
)

func main() {
	// Configurar slog estruturado em JSON
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Iniciando aplicação TIIV...")

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conectar ao Postgres e rodar migrations automaticamente
	db, err := database.ConnectAndMigrate(ctx, cfg)
	if err != nil {
		slog.Error("falha fatal ao inicializar banco de dados", "erro", err)
		os.Exit(1)
	}
	defer db.Pool.Close()

	// Configurar rotas
	router, err := routes.SetupRouter(cfg, db)
	if err != nil {
		slog.Error("falha ao configurar rotas", "erro", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para captura de sinais de encerramento
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Servidor HTTP ouvindo", "endereco", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("erro no servidor HTTP", "erro", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Sinal de encerramento recebido. Desligando servidor graciosamente...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("falha ao desligar servidor graciosamente", "erro", err)
	}

	slog.Info("Servidor encerrado com sucesso.")
}
