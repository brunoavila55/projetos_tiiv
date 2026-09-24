.PHONY: dev build sqlc migrate migrate-down test restore docker-up docker-down docker-logs

# Segredos e demais variáveis vêm do .env (copie de .env.example)
-include .env
export

APP_DB_USER ?= tiiv_app
POSTGRES_DB ?= tiiv
DATABASE_URL ?= postgres://$(APP_DB_USER):$(APP_DB_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable

dev:
	@echo "Iniciando ambiente local..."
	@docker compose up -d postgres
	@echo "Aguardando PostgreSQL..."
	@sleep 2
	@echo "Iniciando backend em segundo plano..."
	@cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server & \
	BACKEND_PID=$$!; \
	echo "Iniciando frontend em modo de desenvolvimento..."; \
	cd frontend && pnpm run dev; \
	kill $$BACKEND_PID 2>/dev/null || true

build:
	@echo "Construindo frontend (SvelteKit SPA)..."
	@cd frontend && pnpm run build
	@echo "Copiando build da SPA para o diretório de embeds do backend..."
	@rm -rf backend/embeds/dist && cp -r frontend/build backend/embeds/dist
	@echo "Compilando binário Go..."
	@mkdir -p bin
	@cd backend && go build -ldflags="-s -w" -o ../bin/tiiv-server ./cmd/server
	@echo "Build completo gerado em bin/tiiv-server"

sqlc:
	@echo "Gerando queries tipadas com sqlc..."
	@cd backend && sqlc generate
	@echo "Queries geradas com sucesso."

migrate:
	@echo "Executando migrations..."
	@goose -dir backend/internal/database/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	@echo "Revertendo última migration..."
	@goose -dir backend/internal/database/migrations postgres "$(DATABASE_URL)" down

test:
	@echo "Executando suíte de testes..."
	@cd backend && go test -v -race ./...

restore:
	@echo "Executando restauração de backup..."
	@./scripts/restore.sh $(FILE) $(DB)

docker-up:
	@echo "Subindo containers com Docker Compose..."
	@docker compose up -d --build

docker-down:
	@echo "Parando containers..."
	@docker compose down

docker-logs:
	@docker compose logs -f
