# Delim — архитектура сервисов

Delim — это MVP для разделения совместных расходов внутри MAX-мини-приложения.
Три сервиса с чёткими границами ответственности, единый PostgreSQL и объектное
хранилище MinIO.

## Компоненты

```
MAX (bot + WebView)
   │  initData / webhook / Deep Link
   ▼
Gateway (Go)  ── HTTP + gRPC-metadata ──►  Core (Go)  ──► PostgreSQL (ledger)
   │                                          ▲
   │  gRPC (receipt/export, stream)           │ source of truth
   ├────────────────► Document (Python) ──────┤   финансовая история
   │                        │                 │
   │                        ▼                 │
   └── MinIO (оригиналы чеков, экспорты)   PostgreSQL (document_/gateway_ схемы)
```

- **Gateway** — публичный HTTP API (`/api/v1/*`), адаптер MAX (auth/initData,
  webhook inbox, invite/deep-link), сессии, rate limiting, security headers,
  request-id, проксирование операций к Core и Document по gRPC.
- **Core** — единственный source of truth по деньгам: пользователи, группы,
  участники, расходы, позиции, аллокации, взаиморасчёты, корректировки, ledger,
  аудит. Финансовая математика и проверки версий (оптимистичный `version`) живут
  только здесь.
- **Document** — обработка документов: приём оригинала в MinIO, OCR-воркер
  (PaddleOCR → предобработка → распознавание → парсинг → уверенность),
  фискальный QR, экспорт отчёта, retention-очистка. Не хранит финансовую историю.

## Единый PostgreSQL, три схемы

Одна база `delim`, разделённая по префиксу таблиц и таблице миграций:

| Владелец | Таблицы | Миграции |
|----------|---------|----------|
| Core | `users`, `groups`, `group_members`, `expenses`, `expense_items`, `allocations`, `settlements`, `adjustments`, `adjustment_allocations`, `audit_log` | `core_schema_migrations` |
| Document | `document_receipts`, `document_jobs`, `document_ocr_results`, `document_exports` | `document_schema_migrations` |
| Gateway | `gateway_max_updates`, `gateway_max_chats` | `gateway_schema_migrations` |

Пороги версий и финансы читаются/пишутся только Core; Document и Gateway не
продуцируют и не меняют экономические операции.

## Хранилище объектов (MinIO)

Приватный бакет `receipts`. Ключи объектов (`object_key`) никогда не отдаются
наружу и не становятся публичными/presigned URL. Скачивание оригинала и экспорта
идёт авторизованно через Gateway (`GET /api/v1/exports/{id}/download`), который
стримит содержимое из Document. Бакет явно переведён в `private` (`minio-init`).

## Request-потоки

- **Авторизация.** MAX отдаёт `initData` → Gateway проверяет подпись
  (`maxauth.InitDataVerifier`, HMAC с `bot_token`) → upsert пользователя в Core
  → выдаётся локальная HMAC-сессия (`auth.session_secret`). Invite/deep-link:
  подпись `invite.secret`, проверка срока.
- **Webhook.** MAX дергает `POST /api/v1/max/webhook` с секретом
  (`maxauth.WebhookVerifier`, constant-time). Gateway идемпотентно кладёт событие
  в `gateway_max_updates` (ключ `event_key`) и обрабатывает асинхронно воркером
  `maxupdate` (подписка участников, синк чатов). Дубликаты отбрасываются.
- **Чек → расход (OCR confirmation boundary).** Загрузка оригинала → job OCR в
  Document. Результат (позиции, итог, дата, QR) возвращается в мини-апп и НЕ
  создаёт расход. Расход создаётся в Core только после явного подтверждения и
  финального submit пользователем. Правки OCR — ручные, до подтверждения.
- **Split / balance / settlement / export.** Расчёт долей, баланса и плана
  погашений выполняет Core (конфликты → HTTP 409 по `version`). Экспорт
  формируется Gateway (готовые строки отчёта из Core) и рендерится Document в
  CSV/PDF/XLSX → MinIO → стриминг авторизованно.

## Границы адаптеров

- **MAX adapter** (`pkg/maxapi`, `internal/gateway/maxupdate`, `pkg/maxauth`) —
  единственное место, где известна схема MAX. Бизнес-сервисы Core/Document от
  неё не зависят; при расхождении схемы правки вносятся только в адаптер.
- **Границы Gateway / Core / Document** — только gRPC с metadata
  (`x-request-id`, `x-user-id`); контракты — `proto/*`. Generated code не
  правится вручную (проверяется `make generated-check`).

## Наблюдаемость и надёжность

- Структурированные логи (slog / Python logging) с `service`, `request_id`,
  `operation`, `duration_ms`, `status/result`; секреты, initData, сырой OCR-текст
  и Authorization не логируются.
- Prometheus-метрики на внутренних `/metrics` (9090/9091/9092) с ограниченным
  набором label'ов; опциональный локальный Prometheus/Grafana (`make
  observability-up`) не входит в обычный `make dev-up`.
