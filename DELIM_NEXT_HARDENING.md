# DELIM_NEXT_HARDENING

Цель: после завершения основных backend-интеграций и Mini App довести Delim до состояния стабильного, проверяемого и демонстрационно/production-ready MVP.

Репозиторий:
https://github.com/BMSTU-LYGO/Delim

Текущий проект уже содержит:
- Gateway + Core + Python Document;
- PostgreSQL + MinIO;
- полноценный Mini App;
- smoke-сценарии;
- Playwright E2E;
- dev bootstrap;
- production static build Mini App;
- OCR/export flow;
- MAX auth/webhook/invite infrastructure.

В этом плане НЕ добавлять новую большую продуктовую функциональность.
Главный фокус:
- стабильность;
- наблюдаемость;
- безопасность;
- воспроизводимая проверка;
- производительность;
- production deployment;
- реальная MAX-интеграция;
- качество демонстрации.

---

# ПРАВИЛА ДЛЯ CODEX

- Работать в текущей ветке.
- Не создавать новые ветки.
- После КАЖДОГО пункта делать отдельный commit.
- Commit называется ровно как указано.
- После пункта писать в чат только:

`<commit> — done`

- Минимизировать расход токенов.
- Не писать длинных отчётов.
- Не перечитывать весь репозиторий после каждого пункта.
- Перед каждым блоком читать только затрагиваемые каталоги.
- Не переписывать Core/Document/Mini App ради стиля.
- Не добавлять новую бизнес-логику без требования этого плана.
- Не менять финансовую математику без обнаруженного бага.
- Не добавлять Redis/Kafka/RabbitMQ.
- Не добавлять Kubernetes.
- Не добавлять тяжёлую observability-инфраструктуру без необходимости.
- Generated protobuf не редактировать вручную.
- Секреты не коммитить.
- После каждого пункта запускать минимальный набор проверок изменённой части.
- После каждого блока продолжать самостоятельно, не ждать подтверждения.

---

# БЛОК 1. Зафиксировать реальное состояние проекта

## 1.1. Сделать project audit script

Создать:

`scripts/audit.sh`

или компактную Go/Python-утилиту, если shell получается неудобным.

Она должна последовательно проверять:

```text
proto generation
Go format/build
Core tests
Python compile
Mini App typecheck/build
Docker Compose config
```

Не поднимать stack.

Добавить:

`make audit`

Результат:
- exit 0 если всё хорошо;
- exit != 0 при первой ошибке;
- короткий вывод.

Commit:

`quality: add project audit command`

---

## 1.2. Проверить generated drift

Добавить:

`make generated-check`

Flow:

1. `make proto`
2. `make document-proto`
3. проверить `git diff --exit-code` только для generated proto.

Цель:
невозможно забыть regenerated contracts.

Не менять generated code вручную.

Commit:

`quality: verify generated contracts`

---

# БЛОК 2. CI

## 2.1. Добавить минимальный GitHub Actions CI

Создать:

`.github/workflows/ci.yml`

Проверки:

### Go
- setup Go;
- go mod download;
- gofmt check;
- `go test ./internal/core/...`;
- `go build ./cmd/gateway`;
- `go build ./cmd/gatewayctl`;
- `go build ./cmd/core`.

### Python
- Python 3.12;
- install только Document dependencies;
- `python -m compileall document/src document/gen`.

### Frontend
- Node 22;
- `npm ci`;
- `npm run typecheck`;
- `npm run build`.

### Contracts
- protoc;
- generated drift check.

Не запускать PaddleOCR model inference в CI.

Не поднимать полный Docker stack в этом пункте.

Commit:

`ci: add mandatory build checks`

---

## 2.2. Добавить integration CI

Отдельный job:

- docker compose build;
- `make dev-init`;
- `make dev-up`;
- `make smoke`;
- cleanup всегда.

MAX online checks должны skip-аться без secrets.

OCR smoke в CI:
допускается test provider/config, если production Paddle model делает CI слишком тяжёлым,
НО путь Gateway → Document → Job → Result должен быть реальным.

Commit:

`ci: add integration smoke checks`

---

# БЛОК 3. Структурированные логи и request_id

## 3.1. Протащить request_id end-to-end

Gateway:

- принимать валидный входящий `X-Request-ID` или генерировать новый;
- возвращать `X-Request-ID` клиенту;
- класть request_id в context;
- передавать его в gRPC metadata Core/Document.

Core:
- читать request_id из gRPC metadata;
- добавлять в structured log.

Document:
- читать request_id из gRPC metadata;
- добавлять в Python logs.

Не использовать request_id как security primitive.

Commit:

`observability: propagate request ids`

---

## 3.2. Нормализовать structured logs

Проверить Gateway/Core slog и Document logger.

Обязательные поля:

- service;
- request_id;
- operation;
- duration_ms;
- status/result.

Для ошибок:
- error class/code;
- без stack trace в пользовательских ответах.

Не логировать:

- MAX_BOT_TOKEN;
- session token;
- invite token;
- webhook secret;
- raw initData;
- полный чек;
- OCR raw text;
- Authorization header;
- MinIO credentials.

Добавить redaction helper только если реально нужен.

Commit:

`observability: harden structured logging`

---

# БЛОК 4. Метрики

## 4.1. Добавить лёгкие Prometheus metrics

Добавить `/metrics` только на внутреннем/конфигурируемом endpoint Gateway/Core.
Для Document — отдельный internal HTTP metrics port либо минимальный exporter.

Метрики минимум:

Gateway:
- HTTP request count;
- HTTP duration histogram;
- status codes;
- Core gRPC errors;
- Document gRPC errors;
- MAX API errors;
- webhook accepted/duplicate/failed.

Core:
- gRPC requests/duration;
- conflicts;
- financial operation errors.

Document:
- OCR jobs pending/processing/completed/failed;
- OCR duration;
- export jobs;
- MinIO errors.

Не добавлять десятки low-value labels.
Не использовать user_id/group_id/receipt_id как metric labels.

Commit:

`observability: add service metrics`

---

## 4.2. Добавить локальные Prometheus/Grafana опционально

Создать:

`deployments/observability/`

или compose profile.

Сервисы:
- prometheus;
- grafana.

Они НЕ должны запускаться обычным `make dev-up`.

Добавить:

`make observability-up`
`make observability-down`

Один простой dashboard:
- Gateway latency/error rate;
- Core errors;
- OCR job health.

Commit:

`observability: add optional monitoring stack`

---

# БЛОК 5. Security hardening Gateway

## 5.1. Добавить HTTP security headers

Для Mini App API responses:

- `X-Content-Type-Options: nosniff`;
- `Referrer-Policy`;
- разумный `Permissions-Policy`;
- `Cache-Control: no-store` для auth/private API.

Не добавлять CSP к JSON API бессмысленно.

Для static Mini App nginx:
добавить безопасные headers без поломки MAX WebView.

Commit:

`security: add HTTP security headers`

---

## 5.2. Добавить rate limiting внешнего API

Сделать простой bounded in-memory rate limiter Gateway.

Отдельно:

- `/api/v1/auth/max`;
- `/api/v1/max/webhook`;
- receipt upload;
- обычный authenticated API.

Ключ:
- IP для unauthenticated;
- session/Core user id для authenticated.

Не использовать user-controlled header как identity.

При превышении:
429 + `Retry-After`.

Не вводить distributed limiter для MVP.

Commit:

`security: rate limit public API`

---

## 5.3. Усилить session/invite security

Проверить:

- constant-time HMAC compare;
- expiry;
- version;
- malformed token handling;
- session/invite secrets разные;
- minimum secret length в production;
- `app.env != local` запрещает слабые/default secrets.

При старте production:
fail-fast на небезопасных secrets.

Commit:

`security: validate production secrets`

---

# БЛОК 6. Data/privacy hardening

## 6.1. Добавить retention чеков

Добавить config:

```yaml
privacy:
  receipt_retention_days: 30
```

Для MVP реализовать background cleanup:

- soft-deleted receipts;
- старые originals, если retention истёк;
- связанные expired exports.

Не удалять структурированный финансовый Expense из Core.

Не запускать cleanup слишком часто.

Commit:

`privacy: add document retention cleanup`

---

## 6.2. Удаление оригинала чека пользователем

Если текущий DeleteReceipt удаляет всю Receipt сущность,
добавить отдельную возможность:

`DeleteReceiptOriginal`

или подходящий существующей модели эквивалент.

После удаления:
- OCR result сохраняется;
- Expense сохраняется;
- MinIO original удалён;
- повторное удаление idempotent.

Gateway + Mini App:
добавить действие `Удалить оригинал фото`.

Commit:

`privacy: allow deleting receipt originals`

---

## 6.3. Проверить отсутствие public object URLs

Поискать все обращения MinIO/export.

Гарантировать:
- bucket private;
- backend не возвращает постоянный MinIO URL;
- download идёт авторизованно через Gateway/stream.

Добавить smoke на чужой export/receipt access.

Commit:

`security: enforce private document access`

---

# БЛОК 7. PostgreSQL надёжность

## 7.1. Проверить индексы по реальным запросам

Изучить только SQL repositories.

Для частых запросов проверить indexes:

- group members;
- expenses by group;
- settlements by group;
- adjustments by expense;
- gateway webhook pending;
- document jobs pending;
- receipts by actor/group.

Использовать `EXPLAIN` на нескольких ключевых запросах.

Добавить миграцию только если индекс реально нужен.

Не добавлять индексы "на всё".

Commit:

`database: optimize critical queries`

---

## 7.2. Добавить backup/restore dev procedure

Добавить:

`make db-backup`
`make db-restore FILE=...`

Для dev/demo PostgreSQL.

Backup не коммитить.

README:
коротко описать команды.

После restore проверить migrations tables.

Commit:

`database: add backup restore workflow`

---

# БЛОК 8. Производительность

## 8.1. Добавить backend benchmark/smoke timing

Расширить `cmd/smoke` или создать `cmd/perfcheck`.

Проверить локально:

- GET groups;
- CreateExpense;
- GetBalance;
- readiness.

Собирать:
- median;
- p95;
- error count.

Не строить полноценный load-testing framework.

Цели из проектных требований:

- обычные API p95 ≤ 300ms;
- простой CreateExpense p95 ≤ 500ms

на локальном стабильном окружении без OCR.

Commit:

`performance: add API latency check`

---

## 8.2. Проверить N+1 и connection pools

Профилировать только основные user-story:

- group dashboard;
- expense details;
- balance;
- members;
- exports source loading.

Убрать очевидные N+1.

Проверить pool sizes Gateway/Core/Document.

Не делать premature optimization.

Commit:

`performance: optimize critical user stories`

---

# БЛОК 9. OCR reliability/quality

## 9.1. Добавить небольшой набор обезличенных receipt fixtures

Создать `document/testdata/` с несколькими синтетическими/разрешёнными тестовыми изображениями:

- хороший русский чек;
- повернутый;
- слабый контраст;
- без QR;
- QR + OCR mismatch.

Не использовать реальные персональные чеки без права на публикацию.

Размер fixtures держать небольшим.

Commit:

`document: add OCR regression fixtures`

---

## 9.2. Добавить OCR regression command

Создать:

`make document-ocr-check`

Проверять:
- pipeline не падает;
- confidence в диапазоне;
- QR parser;
- total/date для известных fixture;
- служебные строки не становятся товарами.

Не требовать идеально одинакового текста от каждой версии Paddle,
если это нестабильно.

Commit:

`document: add OCR regression checks`

---

# БЛОК 10. Frontend regression

## 10.1. Усилить Playwright user stories

Текущие E2E не переписывать.

Добавить сценарии:

- unauthenticated → auth screen;
- forbidden group;
- 409 expense conflict;
- Document unavailable;
- OCR failed + retry;
- archived group read-only;
- settlement confirm second user;
- export download.

Не делать pixel-perfect screenshots.

Commit:

`miniapp: expand regression scenarios`

---

## 10.2. Проверить MAX viewport modes

Добавить в Playwright viewport projects:

- compact mobile;
- large mobile;
- desktop.

Проверить:
- sticky actions;
- no horizontal overflow;
- forms with virtual-keyboard-like small height;
- BackButton adapter fallback.

Commit:

`miniapp: verify MAX viewport layouts`

---

# БЛОК 11. Реальная MAX интеграция

Выполнять реальные online проверки ТОЛЬКО если доступны test credentials.

## 11.1. Добавить MAX integration checklist command

Расширить `gatewayctl max check`.

Проверять:

- GetMe;
- bot username;
- webhook subscription URL;
- expected update types;
- configured webhook secret presence;
- Mini App URL config presence.

Не выводить token/secrets.

Commit:

`max: add integration diagnostics`

---

## 11.2. Проверить webhook lifecycle

При реальном MAX token:

- `make max-setup`;
- убедиться, что subscription создана;
- bot_started;
- bot_added;
- message_created;
- message_callback;
- duplicate webhook;
- bot_removed.

Зафиксировать найденные несовместимости только в MAX adapter.

Business services не должны зависеть от MAX schema.

Commit:

`max: harden real webhook integration`

---

## 11.3. Проверить startapp/deep link

В реальном MAX:

- invite link;
- open Mini App;
- initData;
- start_param;
- login;
- JoinGroup;
- повторный переход;
- expired invite.

Если Bridge/клиент отличается iOS/Android/Web — fallback должен работать.

Commit:

`max: verify invite user story`

---

# БЛОК 12. Production deployment

## 12.1. Добавить production compose/reference deployment

Создать:

`deployments/prod/compose.yaml`

Без:
- dev bind mounts;
- открытых Core/Document/PostgreSQL/MinIO портов;
- default credentials.

Публично:
- reverse proxy;
- Mini App static;
- Gateway.

Internal network:
- Core;
- Document;
- PostgreSQL;
- MinIO.

Не добавлять Kubernetes.

Commit:

`deploy: add production compose`

---

## 12.2. HTTPS reverse proxy

Добавить Caddy или nginx config.

Требования:
- HTTPS termination;
- `/api` → Gateway;
- Mini App static;
- websocket не нужен, если проект его не использует;
- upload size согласован;
- timeouts;
- health endpoint.

TLS certificates не коммитить.

Commit:

`deploy: add HTTPS ingress`

---

## 12.3. Production config validation

Добавить:

`make prod-check`

Он должен проверять:
- обязательные ENV;
- HTTPS URLs;
- no localhost;
- no wildcard CORS;
- strong secrets;
- MAX webhook URL;
- Mini App/Gateway URL;
- compose config.

Не печатать secret values.

Commit:

`deploy: validate production configuration`

---

# БЛОК 13. Документация

## 13.1. Создать короткий architecture doc

Создать:

`docs/architecture.md`

Только полезное:

- схема сервисов;
- кто source of truth;
- request flows;
- storage ownership;
- границы Gateway/Core/Document;
- OCR confirmation boundary;
- MAX adapter boundary.

Не дублировать весь README.

Commit:

`docs: document service architecture`

---

## 13.2. Создать runbook

Создать:

`docs/runbook.md`

Сценарии:

- deploy;
- health failure;
- Core unavailable;
- Document unavailable;
- PostgreSQL unavailable;
- MinIO unavailable;
- stuck OCR jobs;
- bad webhook subscription;
- rollback;
- backup/restore.

Команды конкретные и короткие.

Commit:

`docs: add operational runbook`

---

# БЛОК 14. Хакатон acceptance matrix

## 14.1. Сопоставить требования с доказательством

Создать:

`docs/acceptance.md`

Таблица:

```text
Требование | Где реализовано | Как проверить | Статус
```

Покрыть:

- auth;
- webhook/idempotency;
- groups/invites;
- all splits;
- ledger;
- settlement/refund;
- OCR/manual correction;
- export;
- security;
- readiness;
- E2E.

Не писать "DONE" без команды/сценария проверки.

Commit:

`docs: add acceptance matrix`

---

## 14.2. Добавить demo script

Создать:

`docs/demo.md`

Демо на 5–7 минут:

1. открыть Delim;
2. создать/открыть поездку;
3. показать участников;
4. загрузить чек;
5. OCR;
6. ручная правка;
7. split;
8. confirm;
9. balance;
10. settlement;
11. export.

Также fallback demo:
если OCR/MAX временно недоступен — использовать заранее подготовленную локальную группу.

Не добавлять fake production behavior.

Commit:

`docs: add product demo script`

---

# БЛОК 15. Финальный release gate

## 15.1. Создать `make release-check`

Команда запускает:

```text
generated-check
Go tests/build
Python compile
frontend typecheck/build
compose config
integration smoke
frontend E2E
prod-check
```

OCR full regression можно отдельным флагом/target из-за веса.

MAX online check:
только если credentials доступны.

Commit:

`quality: add release gate`

---

## 15.2. Финальный прогон

Выполнить:

```bash
make audit
make generated-check
make dev-up
make smoke
make web-typecheck
make web-build
npm --prefix web/miniapp run e2e
make release-check
```

При наличии MAX credentials:

```bash
make max-check
make max-setup
```

При возможности:

```bash
make document-ocr-check
```

Исправить только реальные дефекты.

Commit:

`release: harden Delim MVP`

---

# DEFINITION OF DONE

После этого апдейта проект должен:

1. Воспроизводимо собираться одной командой.
2. Иметь CI на Go/Python/Frontend/contracts.
3. Иметь integration smoke CI.
4. Не допускать generated drift.
5. Протаскивать request_id через Gateway/Core/Document.
6. Иметь безопасные структурированные логи.
7. Не логировать secrets/initData/receipt contents.
8. Иметь базовые service metrics.
9. Иметь optional local monitoring.
10. Иметь HTTP security headers.
11. Иметь rate limiting внешнего Gateway.
12. Валидировать production secrets.
13. Иметь retention чеков/экспортов.
14. Позволять удалить оригинал чека без удаления финансовой истории.
15. Не иметь public MinIO URLs.
16. Иметь проверенные индексы критических запросов.
17. Иметь backup/restore procedure.
18. Иметь API latency regression check.
19. Не иметь очевидных N+1 в основных flows.
20. Иметь OCR regression fixtures/check.
21. Иметь расширенные Playwright E2E.
22. Быть проверенным на mobile/desktop viewport.
23. Иметь MAX diagnostics.
24. При credentials проходить реальный webhook/deep-link flow.
25. Иметь production compose.
26. Иметь HTTPS ingress.
27. Иметь fail-fast production config validation.
28. Иметь architecture doc.
29. Иметь operational runbook.
30. Иметь acceptance matrix.
31. Иметь готовый demo script.
32. Иметь единый `make release-check`.
33. Не нарушать границы Gateway/Core/Document.
34. Не добавлять новую финансовую бизнес-логику вне Core.
35. Основной сценарий
   `MAX → группа → чек/расход → split → confirm → balance → settlement → export`
   проходить без ручных обходов.

После завершения вывести только:

`DELIM_NEXT_HARDENING done — <N commits>`
