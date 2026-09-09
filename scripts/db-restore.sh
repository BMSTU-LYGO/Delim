#!/usr/bin/env bash
# Dev/demo PostgreSQL restore from a pg_dump custom-format file. Usage:
#   make db-restore FILE=backups/delim-20260909-120000.dump
# Restores into the single Delim database, replacing existing objects.
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE="docker compose -f deployments/dev/compose.yaml"
if [ -f .env ]; then set -a; . ./.env; set +a; fi

DB="${POSTGRES_DB:-${POSTGRES_USER:-delim}}"
USER="${POSTGRES_USER:-delim}"
TMP="/tmp/delim-restore-$$.dump"

if [ -z "${FILE:-}" ]; then
  echo "usage: make db-restore FILE=<dump.dump>" >&2
  exit 1
fi
if [ ! -f "$FILE" ]; then
  echo "dump file not found: $FILE" >&2
  exit 1
fi

trap '$COMPOSE exec -T postgres rm -f "$TMP" >/dev/null 2>&1 || true' EXIT

# shellcheck disable=SC2086
$COMPOSE cp "$FILE" postgres:$TMP
# shellcheck disable=SC2086
$COMPOSE exec -T postgres sh -c \
  "pg_restore -U $USER -d $DB --clean --if-exists --no-owner --no-privileges $TMP"

# Post-restore verification: all three migration ledgers must be present.
# shellcheck disable=SC2086
$COMPOSE exec -T postgres sh -c \
  "psql -v ON_ERROR_STOP=1 -U $USER -d $DB -Atc \"
     SELECT 'core='||(SELECT count(*) FROM core_schema_migrations)
         || ' document='||(SELECT count(*) FROM document_schema_migrations)
         || ' gateway='||(SELECT count(*) FROM gateway_schema_migrations)\""

echo "restore completed from $FILE"
