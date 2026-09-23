.PHONY: dev build sqlc migrate migrate-down test docker-up docker-down docker-logs

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/tiiv?sslmode=disable

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

docker-up:
	@echo "Subindo containers com Docker Compose..."
	@docker compose up -d --build

docker-down:
	@echo "Parando containers..."
	@docker compose down

docker-logs:
	@docker compose logs -f
