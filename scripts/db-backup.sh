#!/usr/bin/env bash
# Dev/demo PostgreSQL backup via pg_dump (custom format). Usage:
#   make db-backup [FILE=path.dump]
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE="docker compose -f deployments/dev/compose.yaml"
if [ -f .env ]; then set -a; . ./.env; set +a; fi

DB="${POSTGRES_DB:-${POSTGRES_USER:-delim}}"
USER="${POSTGRES_USER:-delim}"
OUT="${FILE:-backups/delim-$(date +%Y%m%d-%H%M%S).dump}"
TMP="/tmp/delim-backup-$$.dump"

mkdir -p "$(dirname "$OUT")"
trap '$COMPOSE exec -T postgres rm -f "$TMP" >/dev/null 2>&1 || true' EXIT

# shellcheck disable=SC2086
$COMPOSE exec -T postgres sh -c \
  "pg_dump -U $USER -d $DB --format=custom --compress=6 --file=$TMP"
# shellcheck disable=SC2086
$COMPOSE exec -T postgres sh -c "pg_restore --list $TMP > /dev/null"
# shellcheck disable=SC2086
$COMPOSE cp postgres:$TMP "$OUT"

echo "backup written: $OUT ($(du -h "$OUT" | cut -f1))"
