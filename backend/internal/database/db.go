package database

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"
	"log/slog"
	"math/big"
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
		pin := cfg.AdminPIN
		if pin == "" {
			// Sem ADMIN_PIN, nada de PIN conhecido: gera um e mostra só neste log
			pin, err = pinAleatorio()
			if err != nil {
				return fmt.Errorf("erro ao gerar PIN do admin: %w", err)
			}
			slog.Warn("ADMIN_PIN não definido: PIN inicial do admin gerado. Anote-o, ele não será mostrado de novo e deve ser trocado no primeiro acesso.",
				"nome", cfg.AdminNome, "pin", pin)
		}
		if !regexp.MustCompile(`^[0-9]{4}$`).MatchString(pin) {
			return fmt.Errorf("ADMIN_PIN deve conter exatamente 4 dígitos numéricos")
		}
		if config.PinTrivial(pin) {
			return fmt.Errorf("ADMIN_PIN é trivial; escolha outro ou deixe vazio para gerar um aleatório")
		}
		slog.Info("Nenhum usuário cadastrado. Criando primeiro admin configurado...", "nome", cfg.AdminNome)
		hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("erro ao gerar hash do PIN do admin: %w", err)
		}

		// A migração de setores já cria o primeiro setor (NOC)
		setor, err := db.Queries.PrimeiroSetor(ctx)
		if err != nil {
			return fmt.Errorf("erro ao buscar setor do admin inicial: %w", err)
		}

		// Cor padrão bonita para o admin: Azul #2563EB. Nasce superadmin, que
		// é quem cria os demais setores.
		admin, err := db.Queries.CriarUsuario(ctx, sqlc.CriarUsuarioParams{
			Nome:    cfg.AdminNome,
			Cor:     "#2563EB",
			PinHash: string(hash),
			Papel:   "superadmin",
			Ativo:   true,
			SetorID: setor.ID,
		})
		if err != nil {
			return fmt.Errorf("erro ao inserir admin inicial: %w", err)
		}

		// O PIN inicial passou por variável de ambiente ou log: troca obrigatória
		if err := db.Queries.MarcarTrocaPinObrigatoria(ctx, admin.ID); err != nil {
			return fmt.Errorf("erro ao marcar troca de PIN do admin inicial: %w", err)
		}

		slog.Info("Primeiro admin criado com sucesso", "id", admin.ID, "nome", admin.Nome)
	}

	return nil
}

// pinAleatorio sorteia um PIN de 4 dígitos que não seja trivial
func pinAleatorio() (string, error) {
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err != nil {
			return "", err
		}
		if pin := fmt.Sprintf("%04d", n.Int64()); !config.PinTrivial(pin) {
			return pin, nil
		}
	}
}
