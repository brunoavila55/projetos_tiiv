#!/bin/bash
set -e

# ==============================================================================
# Script de Restauração de Backup do PostgreSQL para o TIIV
# Uso:
#   ./scripts/restore.sh [caminho_do_dump] [nome_do_banco]
#
# Exemplos:
#   ./scripts/restore.sh                               # Restaura o backup mais recente no banco tiiv
#   ./scripts/restore.sh /backups/tiiv_backup_xxx.dump # Restaura dump específico no banco tiiv
#   ./scripts/restore.sh /backups/tiiv_backup_xxx.dump tiiv_teste # Restaura em banco tiiv_teste
# ==============================================================================

BACKUP_CONTAINER="tiiv_backup"
POSTGRES_CONTAINER="tiiv_postgres"
TARGET_DB="${2:-tiiv}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"

# Verificar se os containers necessários estão em execução
if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
  echo "Erro: Container ${POSTGRES_CONTAINER} não está em execução."
  echo "Inicie os containers com: docker compose up -d"
  exit 1
fi

DUMP_FILE="$1"

# Se nenhum arquivo foi especificado, pega o dump mais recente do volume
if [ -z "$DUMP_FILE" ]; then
  echo "Buscando o arquivo de backup mais recente em ${BACKUP_CONTAINER}:/backups..."
  DUMP_FILE=$(docker exec "$BACKUP_CONTAINER" /bin/sh -c 'ls -t /backups/tiiv_backup_*.dump 2>/dev/null | head -n 1')
  
  if [ -z "$DUMP_FILE" ]; then
    echo "Erro: Nenhum arquivo de backup encontrado no volume de backups."
    exit 1
  fi
fi

echo "Arquivo de backup selecionado: $DUMP_FILE"
echo "Banco de dados alvo: $TARGET_DB"

# Garantir que o banco de dados de destino existe
echo "Garantindo existência do banco de dados '$TARGET_DB'..."
docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -tc "SELECT 1 FROM pg_database WHERE datname = '$TARGET_DB'" | grep -q 1 || \
docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -c "CREATE DATABASE $TARGET_DB;"

# Executar a restauração usando pg_restore
echo "Iniciando restauração via pg_restore..."
docker exec -i "$BACKUP_CONTAINER" pg_restore \
  -h "$POSTGRES_CONTAINER" \
  -U "$POSTGRES_USER" \
  -d "$TARGET_DB" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  "$DUMP_FILE" || true

echo "================================================================="
echo "Restauração do backup concluída com sucesso no banco '$TARGET_DB'!"
echo "================================================================="
