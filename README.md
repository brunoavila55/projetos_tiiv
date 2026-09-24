# TIIV — Aplicação Interna do Setor

Aplicação web interna para uso de equipe em rede local (LAN), englobando:
1. **Calendário**: Reuniões e eventos entre membros da equipe.
2. **Tarefas**: Gestão de pendências com status, prioridade e responsáveis.
3. **Estoque**: Controle material baseado em movimentações transacionais (entrada/saída/ajuste).
4. **Técnicos**: Cadastro de técnicos, marcação de entrada/saída e relatório de horas.
5. **Acesso PDV**: Terminal com seleção visual de operador e PIN numérico de 4 dígitos.

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

Edite o `.env` e troque **`POSTGRES_PASSWORD`** e **`APP_DB_PASSWORD`** por senhas geradas (uma diferente para cada):
```bash
openssl rand -hex 24
```
O compose não sobe sem elas, e o app se recusa a iniciar com senhas padrão/curtas, com o superusuário `postgres` no `DATABASE_URL` ou com `ADMIN_PIN` trivial (`1234`, `0000`, sequências).

### 2. Subir o sistema do zero
```bash
docker compose up -d --build
```

A aplicação subirá automaticamente:
- O banco PostgreSQL inicializará com healthcheck.
- As migrations do Goose serão executadas na inicialização.
- Na primeira subida, `scripts/initdb/10-papel-app.sh` cria o papel `tiiv_app` (sem superusuário), que é o usado pelo app.
- O primeiro administrador é criado com `ADMIN_NOME` e `ADMIN_PIN`. Com `ADMIN_PIN` vazio, o PIN é sorteado e aparece **uma única vez** no log (`docker compose logs app | grep "PIN inicial"`). Em qualquer caso, o sistema exige trocar esse PIN no primeiro acesso.
- O PostgreSQL só é publicado em `127.0.0.1:5432` (para o `make dev`); na rede, só o app responde.
- O serviço de backup iniciará com rotação de 7 dias.

Acesse no navegador:
👉 **`http://localhost:8080`** (ou pelo IP da máquina na rede local: `http://<IP_LOCAL>:8080`)

### Atualizando uma instalação existente
Instalações anteriores usavam `postgres/postgres` e o app conectava como superusuário. Depois de preencher o `.env` como acima:
```bash
# 1. Trocar a senha do superusuário (o volume antigo mantém a senha antiga)
docker compose exec postgres psql -U postgres -c "ALTER ROLE postgres PASSWORD '<POSTGRES_PASSWORD do .env>'"
# 2. Recriar os containers (a sub-rede do compose mudou) e criar o papel do app
docker compose down && docker compose up -d --build postgres
docker compose exec postgres sh /docker-entrypoint-initdb.d/10-papel-app.sh
docker compose up -d --build
```
Se o admin ainda usa o PIN `1234`, troque-o pela tela de usuários.

### HTTPS (opcional, recomendado)
Sem TLS, PINs e o cookie de sessão trafegam em claro na LAN. O perfil `tls` sobe um Caddy com certificado da própria CA interna:
```bash
# no .env
TIIV_HOST=192.168.0.10   # IP ou nome pelo qual os terminais acessam
COOKIE_SECURE=true       # cookie só por HTTPS e HSTS ligado
APP_BIND=127.0.0.1       # a porta 8080 (HTTP) deixa de ser exposta na rede

docker compose --profile tls up -d
```
Acesse por `https://<TIIV_HOST>`; HTTP é redirecionado para HTTPS. Para os navegadores confiarem no certificado, instale a CA raiz do Caddy em cada terminal:
```bash
docker compose cp caddy:/data/caddy/pki/authorities/local/root.crt ./tiiv-ca.crt
```
(Windows: `certmgr.msc` → Autoridades de Certificação Raiz Confiáveis; Linux: `/usr/local/share/ca-certificates/` + `update-ca-certificates`; Android: Configurações → Segurança → Instalar certificado.)

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

# Rodar os testes automatizados (usam o banco local; ADMIN_PIN = PIN do admin desse banco)
make test
```

O `Makefile` lê o `.env` e monta o `DATABASE_URL` local com `APP_DB_USER`/`APP_DB_PASSWORD`.

---

## ⚙️ Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `DATABASE_URL` | — (obrigatório) | String de conexão com o papel `tiiv_app`. No compose e no `make` é montada a partir de `APP_DB_USER`/`APP_DB_PASSWORD` |
| `POSTGRES_PASSWORD` | — (obrigatório) | Senha do superusuário `postgres` (administração e backup) |
| `APP_DB_USER` / `APP_DB_PASSWORD` | `tiiv_app` / — (obrigatório) | Papel sem superusuário usado pelo app |
| `LISTEN_ADDR` | `:8080` | Endereço e porta HTTP do servidor |
| `SESSION_TTL` | `30m` | Tempo limite de inatividade da sessão |
| `SESSION_MAX_TTL` | `12h` | Tempo de vida máximo absoluto da sessão |
| `COOKIE_SECURE` | `false` | `true` com HTTPS (perfil `tls`): cookie só por HTTPS e HSTS ligado |
| `TIIV_HOST` / `APP_BIND` | `localhost` / `0.0.0.0` | Perfil `tls`: nome/IP do certificado e interface onde a porta 8080 é publicada |
| `LOGIN_FALHAS_POR_IP` | `4` | PINs errados aceitos por terminal (IP) em 15 min, antes de responder 429 |
| `ADMIN_NOME` | `Administrador` | Nome do primeiro admin criado na inicialização |
| `ADMIN_PIN` | — (sorteado) | PIN do primeiro admin (4 dígitos, não trivial). Vazio: sorteado e exibido uma vez no log. Troca obrigatória no primeiro acesso |
| `CF_ACCOUNT_ID` | — | Conta da Cloudflare usada pelo tira-dúvidas (vazio desliga o chat) |
| `CF_API_TOKEN` | — | Token da Cloudflare com permissão Workers AI |
| `CF_AI_MODEL` | `@cf/qwen/qwen3-30b-a3b-fp8` | Modelo da Workers AI |
| `CF_AI_NEURONS_ENTRADA_M` / `CF_AI_NEURONS_SAIDA_M` | `4625` / `30475` | Neurons por milhão de tokens do modelo (ajuste se trocar o modelo) |
| `ASSISTENTE_NEURONS_DIA` | `9500` | Limite diário de neurons; ao atingir, o chat fica indisponível até 00:00 UTC |
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
Para restaurar um dump de backup (restauração testada e validada):

**Opção 1: Usando o comando make (mais fácil)**
```bash
# Restaura o backup mais recente do volume dedicado no banco padrão (tiiv)
make restore

# Ou restaura um arquivo específico em um banco específico
make restore FILE=/backups/tiiv_backup_2026-09-23_14-44-07.dump DB=tiiv
```

**Opção 2: Usando o script direto**
```bash
# Restaura o backup mais recente
./scripts/restore.sh

# Restaura dump específico em um banco alvo (ex: para testes)
./scripts/restore.sh /backups/tiiv_backup_2026-09-23_14-44-07.dump tiiv_teste
```

**Opção 3: Manual via pg_restore**
```bash
docker compose exec postgres pg_restore -U postgres -d tiiv --clean --if-exists /backups/arquivo.dump
# As tabelas restauradas ficam com o superusuário; devolva-as ao papel do app
docker compose exec postgres sh /docker-entrypoint-initdb.d/10-papel-app.sh
```

---

## 🔐 Autenticação Estilo PDV

- **Teclado Numérico:** Teclado grande na interface otimizado para telas sensíveis ao toque e compatível com teclado físico (`0`-`9`, `Backspace`, `Enter`, `Esc`).
- **Segurança de PIN:** O PIN é criptografado com `bcrypt`. Dois usuários podem ter o mesmo PIN sem conflito, cada um acessando estritamente sua conta.
- **Proteção contra Força Bruta:** A tentativa é contada no banco antes de conferir o PIN, então requisições simultâneas não furam o limite. A 5ª tentativa errada em 15 minutos bloqueia a conta (`HTTP 423 Locked`) por 5 min, e bloqueios seguidos sobem para 15 min, 45 min e 1 h. A troca do próprio PIN usa o mesmo contador e encerra a sessão ao atingir o limite; trocar o PIN derruba as demais sessões do usuário.
- **Limite por terminal:** Cada IP pode errar no máximo `LOGIN_FALHAS_POR_IP` PINs em 15 minutos (`HTTP 429`), abaixo das 5 que bloqueiam a conta; assim um único terminal não consegue travar o login de ninguém. O IP é o da conexão TCP (`X-Forwarded-For` só é aceito do Caddy do perfil `tls`).
- **Sessão Segura:** Cookie `HttpOnly`, `SameSite=Strict`. No banco de dados armazena-se apenas o hash SHA-256 do token da sessão.
- **Inatividade:** Sessão encerrada após 30 minutos de inatividade do operador.
