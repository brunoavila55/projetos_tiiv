# TIIV — Aplicação Interna do Setor

Aplicação web interna para uso de equipe em rede local (LAN), englobando:
1. **Calendário**: Reuniões e eventos entre membros da equipe.
2. **Atendimentos**: Registro ágil de atendimentos (nome de cliente texto livre + descrição).
3. **Tarefas**: Gestão de pendências com status, prioridade e responsáveis.
4. **Estoque**: Controle material baseado em movimentações transacionais (entrada/saída/ajuste).
5. **Acesso PDV**: Terminal com seleção visual de operador e PIN numérico de 4 a 6 dígitos.

---

## 🛠️ Stack Tecnológica

- **Backend:** Go 1.26 / 1.23+ com roteamento `chi/v5`, driver PostgreSQL `pgx/v5`, queries tipadas com `sqlc` e migrations com `goose`.
- **Banco de Dados:** PostgreSQL 16 com extensão `unaccent` para busca insensível a acentos/maiúsculas.
- **Frontend:** SvelteKit em modo SPA (`adapter-static`), TypeScript, Tailwind CSS v4, `@lucide/svelte` e `@event-calendar/core`.
- **Entrega & Deploy:** Binário único em Go com a SPA estática embutida (`go:embed`), orquestrado via `Docker Compose` multi-stage com container dedicado de backup diário (retenção de 7 dias).

---

## 🚀 Como Executar com Docker Compose (Produção / LAN)

### 1. Clonar e configurar ambiente
```bash
cp .env.example .env
```

### 2. Subir o sistema do zero
```bash
docker compose up -d --build
```

A aplicação subirá automaticamente:
- O banco PostgreSQL inicializará com healthcheck.
- As migrations do Goose serão executadas na inicialização.
- O primeiro administrador será criado automaticamente com as credenciais definidas em `ADMIN_NOME` e `ADMIN_PIN`.
- O serviço de backup iniciará com rotação de 7 dias.

Acesse no navegador:
👉 **`http://localhost:8080`** (ou pelo IP da máquina na rede local: `http://<IP_LOCAL>:8080`)

---

## 💻 Desenvolvimento Local

O projeto possui um `Makefile` com todos os comandos essenciais:

```bash
# Iniciar o banco no Docker e rodar backend + frontend em modo dev
make dev

# Compilar frontend, embutir no backend e gerar o binário único em bin/tiiv-server
make build

# Regenerar queries tipadas com sqlc
make sqlc

# Executar migrations no banco
make migrate

# Reverter última migration
make migrate-down

# Rodar os testes automatizados
make test
```

---

## ⚙️ Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/tiiv?sslmode=disable` | String de conexão do PostgreSQL |
| `LISTEN_ADDR` | `:8080` | Endereço e porta HTTP do servidor |
| `SESSION_TTL` | `30m` | Tempo limite de inatividade da sessão |
| `SESSION_MAX_TTL` | `12h` | Tempo de vida máximo absoluto da sessão |
| `COOKIE_SECURE` | `false` | Se `true`, exige HTTPS para o cookie de sessão (manter `false` em LAN HTTP) |
| `ADMIN_NOME` | `Administrador` | Nome do primeiro admin criado na inicialização |
| `ADMIN_PIN` | `1234` | PIN numérico do primeiro admin (4 a 6 dígitos) |
| `TZ` | `America/Sao_Paulo` | Fuso horário padrão da aplicação |

---

## 💾 Backup e Restauração

### Backup Automático
O container `tiiv_backup` executa diariamente um `pg_dump -Fc` e armazena os arquivos no volume persistente `backup_data`, apagando automaticamente arquivos com mais de 7 dias.

### Gerar Backup Manual Imediato
```bash
docker compose exec postgres pg_dump -U postgres -Fc tiiv > backup_manual_$(date +%Y%m%d_%H%M%S).dump
```

### Restaurar Backup
Para restaurar um dump em um banco de dados vazio:
```bash
# 1. Copiar o arquivo de dump para o container do postgres (se necessário)
docker cp backup.dump tiiv_postgres:/tmp/backup.dump

# 2. Restaurar usando pg_restore
docker compose exec postgres pg_restore -U postgres -d tiiv --clean --if-exists /tmp/backup.dump
```

---

## 🔐 Autenticação Estilo PDV

- **Teclado Numérico:** Teclado grande na interface otimizado para telas sensíveis ao toque e compatível com teclado físico (`0`-`9`, `Backspace`, `Enter`, `Esc`).
- **Segurança de PIN:** O PIN é criptografado com `bcrypt`. Dois usuários podem ter o mesmo PIN sem conflito, cada um acessando estritamente sua conta.
- **Proteção contra Força Bruta:** Após 5 tentativas erradas consecutivas, o operador é bloqueado por 5 minutos (retornando status `HTTP 423 Locked`).
- **Sessão Segura:** Cookie `HttpOnly`, `SameSite=Strict`. No banco de dados armazena-se apenas o hash SHA-256 do token da sessão.
- **Inatividade:** Sessão encerrada após 30 minutos de inatividade do operador.
