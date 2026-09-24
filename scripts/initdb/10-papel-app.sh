#!/bin/sh
# Cria (ou atualiza) o papel de banco usado pela aplicação, sem superusuário,
# e o torna dono do banco e das tabelas do app.
#
# Roda sozinho na primeira subida do container (docker-entrypoint-initdb.d).
# Em instalações que já têm dados, ou depois de restaurar um backup, rode:
#   docker compose exec postgres sh /docker-entrypoint-initdb.d/10-papel-app.sh
set -eu

APP_DB_USER="${APP_DB_USER:-tiiv_app}"
: "${APP_DB_PASSWORD:?defina APP_DB_PASSWORD}"
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_DB:=tiiv}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v app_user="$APP_DB_USER" -v app_password="$APP_DB_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN', :'app_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user') \gexec

SELECT format('ALTER ROLE %I WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD %L',
              :'app_user', :'app_password') \gexec

-- Dono do banco = dono do schema public (pg_database_owner): pode rodar as
-- migrations e criar as extensões confiáveis (unaccent, pgcrypto, pg_trgm)
SELECT format('ALTER DATABASE %I OWNER TO %I', current_database(), :'app_user') \gexec

-- Tabelas já existentes (instalação antiga ou backup restaurado); as
-- sequências ligadas a colunas mudam de dono junto com a tabela
SELECT format('ALTER TABLE public.%I OWNER TO %I', tablename, :'app_user')
FROM pg_tables WHERE schemaname = 'public' \gexec
SQL

echo "Papel ${APP_DB_USER} pronto no banco ${POSTGRES_DB}."
