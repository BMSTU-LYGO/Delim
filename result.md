# DELIM_NEXT_HARDENING — результат работ

Документ фиксирует, что было сделано при продолжении плана `DELIM_NEXT_HARDENING.md`.
Codex упёрся в лимит на середине **блока 4.1** (метрики): в рабочем дереве были
незакоммиченные изменения с 7 падающими unit-тестами (`document/tests/test_metrics.py`).
Работа продолжена с этой точки и доведена до конца плана.

- Ветка: `master` (без новых веток, как требует план).
- Коммитов добавлено: **27** (`git log 24b5d43..HEAD`).
- Отправлено в `origin` (BMSTU-LYGO/Delim): **нет** (коммиты локальные).

---

## Хронология коммитов (по блокам плана)

| # | Коммит | Пункт плана |
|---|--------|-------------|
| 1 | `observability: add service metrics` | 4.1 (доведён из WIP Codex) |
| 2 | `observability: add optional monitoring stack` | 4.2 |
| 3 | `security: add HTTP security headers` | 5.1 |
| 4 | `security: rate limit public API` | 5.2 |
| 5 | `security: validate production secrets` | 5.3 |
| 6 | `privacy: add document retention cleanup` | 6.1 |
| 7 | `privacy: allow deleting receipt originals` | 6.2 |
| 8 | `security: enforce private document access` | 6.3 |
| 9 | `database: optimize critical queries` | 7.1 |
| 10 | `database: add backup restore workflow` | 7.2 |
| 11 | `performance: add API latency check` | 8.1 |
| 12 | `performance: optimize critical user stories` | 8.2 |
| 13 | `document: add OCR regression fixtures` | 9.1 |
| 14 | `document: add OCR regression checks` | 9.2 |
| 15 | `miniapp: expand regression scenarios` | 10.1 |
| 16 | `miniapp: verify MAX viewport layouts` | 10.2 |
| 17 | `max: add integration diagnostics` | 11.1 |
| 18 | `deploy: add production compose` | 12.1 |
| 19 | `deploy: add HTTPS ingress` | 12.2 |
| 20 | `deploy: validate production configuration` | 12.3 |
| 21 | `docs: document service architecture` | 13.1 |
| 22 | `docs: add operational runbook` | 13.2 |
| 23 | `docs: add acceptance matrix` | 14.1 |
| 24 | `docs: add product demo script` | 14.2 |
| 25 | `quality: add release gate` | 15.1 |
| 26 | `release: harden Delim MVP` | 15.2 (+ исправленные дефекты) |
| — | `chore: ignore python cache artifacts` | технический (gitignore `__pycache__`) |

Блоки 1–3 (`quality`, `ci`, `observability` request_id/логи) уже были закоммичены
Codex ранее и переиспользованы как есть.

---

## Что сделано по блокам

### Блок 4 — метрики
- Доведён до рабочего состояния слой Prometheus-метрик (`pkg/metricsx`, лёгкая
  реализация text exposition без внешних runtime-зависимостей; Document — через
  `prometheus-client` на внутреннем порту). Исправлены 7 падающих тестов метрик
  (неверное сопоставление имён `metric.name`/`_total` и ожидаемая нормализация
  label-значений в `test_metrics.py`).
- Внутренние `/metrics`: Gateway :9090, Core :9091, Document :9092.
- Метрики ограничены по cardinality (без user_id/group_id/receipt_id в label-ах).
- Опциональный мониторинг: `deployments/observability/` (overlay-файл),
  `make observability-up/down` (НЕ стартуют с обычным `make dev-up`), один
  дашборд Grafana (Gateway latency/error rate, Core errors, OCR job health),
  `prometheus.yml` со скрейпом трёх сервисов.

### Блок 5 — безопасность Gateway
- Security headers: `X-Content-Type-Options`, `Referrer-Policy`,
  `Permissions-Policy` (Go + nginx для статик-раздачи Mini App),
  `Cache-Control: no-store` для приватного API.
- Rate limiting (`internal/gateway/ratelimit`): токен-бакет, ограниченный размер
  таблицы ключей; отдельные лимиты для `/auth/max` и `/max/webhook` (по IP),
  загрузки чеков и обычного API (по id пользователя из сессии); 429 + `Retry-After`.
  IP берётся только из транспорта (`RemoteAddr`), заголовки клиента не доверяются.
- Продакшен-секреты: `app.env != local` → fail-fast на короткие/заглушечные/
  совпадающие signing-секреты (session/invite/webhook ≥32 и различаются),
  обязателен `MAX_BOT_TOKEN`. Добавлена версия session-токена (`claims.v`).
  Покрыто юнит-тестами (`config_test.go`, `auth/session_test.go`).

### Блок 6 — приватность
- Retention (`privacy.receipt_retention_days`): фоновый `RetentionWorker` в
  Document; миграция `005_retention.sql` (`original_purged_at`, `object_purged_at`,
  частичные индексы). Чистит оригиналы удалённых/просроченных чеков и истёкшие
  экспорты; финансовая история в Core не трогается. Тесты `test_retention_worker.py`.
- `DeleteReceiptOriginal`: новый gRPC-контракт (proto→генерация), метод в
  Document-сервисе (OCR и Expense сохраняются, оригинал в MinIO удаляется,
  повтор идемпотентен), роут Gateway `DELETE /api/v1/receipts/{id}/original`,
  действие в Mini App «Удалить оригинал фото».
- Приватный доступ: бакет явно `private` (`minio-init`), постоянные/presigned
  MinIO-URL нигде не отдаются, скачивание — авторизованно через Gateway-стрим.
  Добавлены smoke-проверки на чужой receipt/OCR/original/delete.

### Блок 7 — PostgreSQL
- `make explain-check` (`scripts/explain-check.sh`): EXPLAIN по критическим
  запросам на синтетических данных (~40k расходов, ~100k аллокаций, 2 группы).
  Вывод: нужные индексы уже есть и используются; новых миграций не добавлял
  («индексы на всё» не заводил).
- `make db-backup` / `make db-restore FILE=...` (`scripts/db-*.sh`, `pg_dump -Fc`
  + `pg_restore`), с проверкой `*_schema_migrations` после restore; дампы в
  gitignore; описание в README.

### Блок 8 — производительность
- `cmd/perfcheck` + `make perfcheck`:median/p95/кол-во ошибок по Gateway API
  (readiness, GET groups, GetBalance, CreateExpense), цели SLO (300ms / 500ms).
  Юнит-тесты перцентилей/сводки.
- Убран реальный N+1: `buildExportRows` дёргал Core по каждому расходу. Добавлен
  `ListGroupAdjustments` (Core, батч-запрос аллокаций) и переписан сбор экспорта
  на один вызов.

### Блок 9 — надёжность OCR
- `document/testdata/` — 5 небольших синтетических фикстур (хороший чек, поворот,
  низкий контраст, без QR, QR + OCR-mismatch) + `manifest.json`; генератор
  `document/tools/gen_ocr_fixtures.py` (Pillow+qrcode). Реальных чеков нет.
- `make document-ocr-check` (`document/tools/ocr_check.py`): гоняет реальный
  детерминированный пайплайн (decode→preprocess→QR→fiscal→parser→confidence) со
  stub-провайдером; `--live` — реальный Paddle с проверкой только структурных
  инвариантов. Проверено в контейнере: opencv реально декодирует QR фикстур.

### Блок 10 — фронтенд-регрессия
- `web/miniapp/e2e/regression-stories.spec.ts` (без переписывания существующих):
  неавторизован→экран входа, forbidden-группа, 409-конфликт расхода,
  Document-недоступен, OCR-failed+retry, read-only архивированной группы.
  (Погашение вторым пользователем и экспорт уже покрыты существующими Story C/D.)
- `web/miniapp/e2e/viewport-modes.spec.ts` + новый проект `max-webview-large`
  (430×932) в `playwright.config.ts`: отсутствие горизонтального переполнения,
  sticky-действия при низкой высоте, фолбэк адаптера BackButton в MAX/вне MAX.

### Блок 11 — MAX-интеграция
- `gatewayctl max check` расширен до диагностики: GetMe/username, наличие и
  валидность webhook URL, ожидаемые update types, присутствие webhook secret и
  Mini App URL — **без вывода токенов/секретов**; offline-режим без credentials.
  Добавлена проводка `max.mini_app_url`. Юнит-тесты (`cmd/gatewayctl/main_test.go`).
- 11.2/11.3 (реальный webhook/deep-link) — не выполнялись: нет тестовых ключей
  (план разрешает online-проверки только при наличии credentials).

### Блок 12 — production-деплой
- `deployments/prod/compose.yaml`: публично — только reverse proxy; Core/Document/
  PostgreSQL/MinIO/Gateway/miniapp — во внутренней сети, без published-портов, без
  dev bind-mounts исходников и без дефолтных кредов (`${VAR:?}` + fail-fast Gateway).
  Отдельный one-shot сервис `migrate`. Prod-конфиги `deployments/prod/configs/*`
  (`app.env: production`), `.env.example`.
- `deployments/prod/Caddyfile`: HTTPS (ACME), `/api`→Gateway, остальное→miniapp,
  лимит тела 12MB (под лимит загрузки 10MB), стриминг без буфера, security headers,
  health; сертификаты не коммитятся (volume).
- `make prod-check` (`scripts/prod-check.sh`): обязательные ENV, HTTPS-URL,
  запрет localhost, запрет wildcard CORS, сильные и различные секреты, сверка
  webhook/mini-app URL с публичным хостом, `compose config`. Значения секретов не
  печатает. Проверено: ловит wildcard-CORS и слабый секрет.

### Блок 13 — документация
- `docs/architecture.md` — состав/границы сервисов, source of truth (Core),
  единая БД и три схемы, владение хранилищем, приватные загрузки, request-потоки,
  граница подтверждения OCR, граница MAX-адаптера, наблюдаемость.
- `docs/runbook.md` — деплой, health/готовность, недоступность Core/Document/
  PostgreSQL/MinIO, залипшие OCR-задачи, плохая webhook-подписка, откат,
  backup/restore, наблюдаемость. Конкретные короткие команды.

### Блок 14 — приёмка/демо
- `docs/acceptance.md` — таблица «Требование | Где реализовано | Как проверить |
  Статус» без «DONE» без команды проверки; реальные MAX online-проверки честно
  помечены BLOCKED (нет credentials).
- `docs/demo.md` — сценарий демо на 5–7 минут (MAX→группа→чек/расход→split→
  confirm→balance→settlement→export) + фолбэк на подготовленной локальной группе.

### Блок 15 — релизный гейт
- `make release-check`: audit → generated-check → smoke → web-typecheck →
  web-build → e2e → prod-check (OCR-regression и MAX-online — отдельные таргеты).
- Финальный прогон + **исправление реальных дефектов** (см. ниже) в
  `release: harden Delim MVP`.

---

## Исправленные реальные дефекты (по ходу финального прогона)

1. **Падение старта Document (незавершённый блок 4.1).**
   `document/src/delim_document/config.py::MetricsConfig` не имел свойства `enabled`,
   на которое ссылается `metrics.Recorder.start_endpoint` → `AttributeError` и crash
   на старте реального контейнера. Добавлено `enabled` (port>0) + регресс-тест в
   `test_metrics.py`.
2. **Сломанный тест блока 3.2 (из-за блока 4.1).**
   `DocumentGRPCServicer.__init__` сделал `recorder` обязательным позиционным
   аргументом, из-за чего `test_request_id.py` падал с `TypeError`. `recorder`
   сделан опциональным (noop по умолчанию), как в `create_grpc_server`.
   Валидировано в контейнере: python-тесты **45/45 OK**.
3. `test_metrics.py` — корректное сопоставление имён метрик Prometheus
   (Counter с `_total`) и ожидаемая нормализация неизвестных значений label.
4. Техническое: `__pycache__/*.pyc`, ошибочно попавшие в индекс, сняты с трекинга
   и добавлены в `.gitignore`.

---

## Что провалидировано и с какими результатами

- `make audit` — зелёные (proto/gofmt/build, core-тесты, python compile,
  miniapp typecheck/build, compose config).
- `make generated-check` — зелёные (нет generated-drift; Go + Python protoc).
- `make prod-check` — зелёные; корректно ловит плохие конфигурации.
- `go test ./...` (Core/Gateway/pkg/perfcheck/gatewayctl) — зелёные.
- `npm --prefix web/miniapp run typecheck` — зелёные.
- `Playwright --list` — 52 теста в 4 проектах компилируются и обнаруживаются.
- Python-тесты в **реальном контейнере** Document — 45/45.
- `document-ocr-check` в **реальном контейнере** — зелёные (нативный opencv).
- Интеграционный smoke (живые Gateway+Core+Document+PostgreSQL):
  `health, expense, settlement, adjustment` — зелёные.

## Не выполнено в этом окружении (ограничения среды, не кода)

- **11.2/11.3** — реальные MAX webhook/deep-link: нет тестовых `MAX_*` credentials.
- Полный `receipt→export` smoke и прогон E2E в браузере: нативный **PaddleOCR
  падает с SIGSEGV на CPU этого хоста** во время инференса (сервис после этого
  перезапускается и снова healthy; контрольные пути Document работают). Это
  воспроизводится и на **существующей** спеке `critical-stories Story A`
  (в песочнице не поднимается dev-рантайм `@maxhub/max-ui`: domain-config 404),
  то есть проблема харнеса, а не добавленных тестов. В CI (блок 2.2) для OCR
  допускается test-провайдер, а браузеры/dev-рантайм рабочие.

## Текущее состояние

- Рабочее дерево чистое (не считая игнорируемых `__pycache__`).
- Dev-стек поднят для проверок (postgres/minio/core/gateway/document — healthy).
  Остановить: `make down`.
- Локальный `.env` заполнен эфемерными dev-секретами (иначе Gateway не может
  выдавать сессии) — файл в gitignore, в коммиты не попал.
- Коммиты **не запушены** в `origin`.
