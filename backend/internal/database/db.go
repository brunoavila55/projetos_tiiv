package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"

	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database/sqlc"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

type DB struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

func ConnectAndMigrate(ctx context.Context, cfg *config.Config) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("falha no ping do PostgreSQL: %w", err)
	}

	slog.Info("Conectado ao PostgreSQL com sucesso")

	// Executar migrations com goose usando o pool via stdlib
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, fmt.Errorf("erro ao configurar dialeto do goose: %w", err)
	}

	slog.Info("Aplicando migrations do banco de dados...")
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return nil, fmt.Errorf("erro ao executar migrations: %w", err)
	}
	slog.Info("Migrations aplicadas com sucesso")

	queries := sqlc.New(pool)
	db := &DB{
		Pool:    pool,
		Queries: queries,
	}

	// Verificar se é necessário criar o primeiro admin
	if err := db.ensureInitialAdmin(ctx, cfg); err != nil {
		return nil, fmt.Errorf("erro ao verificar/criar primeiro admin: %w", err)
	}

	return db, nil
}

func (db *DB) ensureInitialAdmin(ctx context.Context, cfg *config.Config) error {
	count, err := db.Queries.ContarUsuarios(ctx)
	if err != nil {
		return fmt.Errorf("erro ao contar usuários: %w", err)
	}

	if count == 0 {
		if !regexp.MustCompile(`^[0-9]{4}$`).MatchString(cfg.AdminPIN) {
			return fmt.Errorf("ADMIN_PIN deve conter exatamente 4 dígitos numéricos")
		}
		slog.Info("Nenhum usuário cadastrado. Criando primeiro admin configurado...", "nome", cfg.AdminNome)
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPIN), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("erro ao gerar hash do PIN do admin: %w", err)
		}

		// Cor padrão bonita para o admin: Azul #2563EB
		admin, err := db.Queries.CriarUsuario(ctx, sqlc.CriarUsuarioParams{
			Nome:    cfg.AdminNome,
			Cor:     "#2563EB",
			PinHash: string(hash),
			Papel:   "admin",
			Ativo:   true,
		})
		if err != nil {
			return fmt.Errorf("erro ao inserir admin inicial: %w", err)
		}

		slog.Info("Primeiro admin criado com sucesso", "id", admin.ID, "nome", admin.Nome)
	}

	return nil
}
