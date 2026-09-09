#!/usr/bin/env bash
# EXPLAIN checks for critical queries (Block 7.1 evidence).
# Requires the dev PostgreSQL from `make dev-up` (or at least `dev-init`).
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE="docker compose -f deployments/dev/compose.yaml"
if [ -f .env ]; then set -a; . ./.env; set +a; fi

PSQL=($COMPOSE exec -T postgres psql -U "${POSTGRES_USER:-delim}" -d "${POSTGRES_DB:-delim}" -c)

"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT e.id FROM expenses e JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=1 WHERE e.group_id=1 AND (0=0 OR e.id<9223372036854775807) ORDER BY e.id DESC LIMIT 20"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT g.id FROM group_members gm JOIN groups g ON g.id=gm.group_id WHERE gm.user_id=1 AND g.id<9223372036854775807 ORDER BY g.id DESC LIMIT 20"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT s.id FROM settlements s JOIN group_members gm ON gm.group_id=s.group_id AND gm.user_id=1 WHERE s.group_id=1 AND (0=0 OR s.id<9223372036854775807) ORDER BY s.id DESC LIMIT 20"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT e.id,e.payer_user_id,a.id,a.user_id,a.amount_minor FROM expenses e LEFT JOIN allocations a ON a.expense_id=e.id WHERE e.group_id=1 AND e.status='confirmed' ORDER BY e.id,a.id"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT a.id FROM adjustments a JOIN expenses e ON e.id=a.expense_id WHERE a.group_id=1 ORDER BY a.id LIMIT 50"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT event_key FROM gateway_max_updates WHERE status='pending' AND next_attempt_at<=NOW() ORDER BY next_attempt_at,received_at LIMIT 50"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT id FROM document_jobs j WHERE status='pending' AND next_attempt_at<=NOW() ORDER BY next_attempt_at,created_at LIMIT 10"
"${PSQL[@]}" "EXPLAIN (ANALYZE, COSTS OFF) SELECT id, object_key FROM document_receipts WHERE original_purged_at IS NULL AND (deleted_at IS NOT NULL OR created_at < NOW() - make_interval(days => 30)) ORDER BY id LIMIT 200"

echo "explain-check: done (existing indexes cover critical queries; add migrations only on regressions)"
