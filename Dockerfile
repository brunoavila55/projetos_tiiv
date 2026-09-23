# Estágio 1: Build do Frontend (SvelteKit SPA)
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

RUN corepack enable && corepack prepare pnpm@latest --activate

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Estágio 2: Compilação do Backend em Go com os arquivos da SPA embutidos
FROM golang:alpine AS backend-builder
WORKDIR /app/backend

ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git ca-certificates

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# Copia o build da SPA estática para o diretório de embeds
COPY --from=frontend-builder /app/frontend/build ./embeds/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/tiiv-server ./cmd/server

# Estágio 3: Imagem final mínima
FROM alpine:3.20 AS runner
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=America/Sao_Paulo

WORKDIR /app
COPY --from=backend-builder /app/tiiv-server ./tiiv-server

# Usuário não-root para segurança
RUN adduser -D -u 1000 tiivuser && chown -R tiivuser:tiivuser /app
USER tiivuser

EXPOSE 8080
ENTRYPOINT ["./tiiv-server"]
