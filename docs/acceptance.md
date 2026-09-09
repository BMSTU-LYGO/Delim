# Delim — матрица приёмки (хакатон)

Статус `DONE` ставится только при наличии команды/сценария проверки.
Локально перед прогоном интеграционных проверок поднимается стек: `make dev-up`.

| Требование | Где реализовано | Как проверить | Статус |
|---|---|---|---|
| Авторизация через MAX (initData) | Gateway `pkg/maxauth`, `internal/gateway/delivery/http/auth.go`, сессии `internal/gateway/auth` | `make smoke` (login-поток через fixture) + `npm --prefix web/miniapp run e2e` (история «не аутентифицирован → экран входа») | DONE |
| Webhook + идемпотентность | Gateway `maxupdate` + `gateway_max_updates(event_key)`, `pkg/maxauth.WebhookVerifier` | `make smoke`; метрика `delim_webhook_events_total{result="duplicate"}`; `make max-check` | DONE |
| Группы и инвайты | Core `groups`/`group_members`; Gateway `internal/gateway/invite`, `delivery/http/invites.go` | `make smoke` (settlement-история через группу); E2E Story A (создание группы, участники) | DONE |
| Все типы сплитов (equal/shares/percentage/fixed/item) | Core `internal/core/domain/split`, `expense.go` | E2E Story A (переключение режимов `Как разделить`); `go test ./internal/core/...` | DONE |
| Ledger (баланс, breakdown) | Core `usecase/ledger`, `repository/postgres/ledger.go` | `make smoke` (проверка баланса после расхода/погашения/возврата); E2E Story A/D | DONE |
| Settlement / refund | Core `settlement.go`, `adjustment.go` (оптимистичный `version`) | `make smoke`; E2E Story C (подтверждение вторым пользователем), Story D (возврат меняет баланс) | DONE |
| OCR + ручная корректировка | Document `worker/ocr.py`, `ocr/parser.py`, `confidence.py`; мини-апп `OCRReview.tsx` (граница подтверждения) | `make smoke` (receipt-история); `make document-ocr-check`; E2E Story B | DONE |
| Экспорт (CSV/PDF/XLSX) | Gateway `exports.go` (`buildExportRows`), Document `export/` → MinIO → стриминг | `make smoke -stories export`; E2E Story D (скачать CSV) | DONE |
| Безопасность (headers, rate-limit, секреты, приватный доступ) | Gateway `delivery/http/{middleware,ratelimit}.go`, `config` prod-секреты; приватный MinIO; `noStore` | `go test ./internal/gateway/...`; `make prod-check`; smoke-отказ «чужой receipt/export/original» (`cmd/smoke`) | DONE |
| Readiness/health | Gateway `delivery/http/health.go` (`/health`, `/health/ready`) | `curl /health/ready`; `make smoke -stories health`; `make perfcheck` (readiness p95) | DONE |
| Наблюдаемость (request_id, логи, метрики) | `pkg/grpcx`, `pkg/metricsx`, Document `metrics.py`; `/metrics` 9090-9092 | `make audit`; E2E проверка `X-Request-ID`; `make observability-up` | DONE |
| Воспроизводимая сборка | `make audit`, `make generated-check`, CI `.github/workflows/ci.yml` | `make audit`; `make generated-check` | DONE |
| Production-развёртывание | `deployments/prod/compose.yaml`, `Caddyfile`, `make prod-check` | `make prod-check` | DONE |
| E2E мини-аппа | `web/miniapp/e2e/*` (4 проекта: desktop/mobile/…) | `npm --prefix web/miniapp run e2e` | DONE |
| MAX chat binding | Gateway `maxchat.go`, `gateway_max_chat_groups` | `go run ./cmd/smoke -stories max-offline` (bind/get/unbind, permissions) | DONE |
| Команды бота `/new`, `/balance` | `maxupdate/commands.go`, launch tokens | offline dispatcher-тесты (`go test ./internal/gateway/maxupdate/`); live — Block 14 | DONE (offline) |
| Member sync chat→group | `membersync.Sync` + `POST /max-chat/sync`, `user_added` | `max-offline` smoke + dispatcher-тест user_added | DONE |
| Settlement callback | `callback` + `confirm_settlement` в dispatcher | `go test ./internal/gateway/maxupdate/` (receiver/foreign/idempotent) | DONE |
| Durable notifications | `gateway_notifications` outbox + `notifications` worker | метрика `delim_notification_total`; fire-and-forget (не ломает Core) | DONE |
| Bounded MAX-метрики | `pkg/metricsx` (chat_bind/member_sync/bot_command/notification/callback) | `curl /metrics` (без user/chat/group/expense label) | DONE |
| E2E не зависит от Paddle CPU | `document: deterministic E2E OCR provider` (`app.env=test`) | `npm --prefix web/miniapp run e2e` (52/52 на arm64) | DONE |
| Отдельный x86_64 OCR gate | `.github/workflows/ci.yml` `ocr-live-x86` + `make release-check-live-ocr` | CI job; `make release-check-live-ocr` (на arm64 → `unsupported architecture`) | DONE |
| Реальный MAX webhook/deep-link | `make max-setup`/`max-check`, invite/deep-link | `make max-check` (нужны `MAX_*` credentials) | BLOCKED: нет тестовых credentials |

Примечание: пункт «Реальный MAX webhook/deep-link» помечен BLOCKED, так как
требует реальных тестовых ключей MAX; без них online-проверки пропускаются
согласно правилам плана (см. `make max-check` — offline-диагностика конфигурации).
