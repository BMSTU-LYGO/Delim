# Delim — архитектура сервисов

Delim — Mini App и бот MAX для учёта совместных расходов. Публичным является только HTTPS Gateway за Caddy; внутренние сервисы взаимодействуют по gRPC.

## Компоненты

```text
MAX Bot / MAX Mini App
          │ initData, webhook, HTTPS/JSON
          ▼
        Caddy
          │
          ├── Mini App (React/nginx)
          └── Gateway (Go) ── gRPC ──► Core (Go)
                    │                    │
                    └──── gRPC ──► Document (Python)
                                         │
                               PostgreSQL + MinIO
```

- **Gateway** — HTTP API `/api/v1/*`, проверка MAX initData и webhook, HMAC-сессии, приглашения/deep links, приветствие бота и техническая доставка сообщений в MAX, rate limiting и вызовы Core/Document.
- **Core** — единственный источник истины для пользователей, групп, расходов, распределений, корректировок, погашений, баланса и финансового аудита.
- **Document** — приватное хранение чеков и экспортов, фискальный QR, OCR и retention. OCR сам не создаёт финансовых операций.
- **Mini App** — React-интерфейс; в production использует same-origin `/api`.
- **Caddy** — TLS, security headers и маршрутизация публичного трафика.

## Данные

Одна база `delim` использует три набора таблиц и три журнала миграций:

| Владелец | Основные таблицы | Журнал миграций |
|---|---|---|
| Core | `users`, `groups`, `group_members`, `expenses`, `expense_items`, `allocations`, `settlements`, `adjustments`, `adjustment_allocations`, `audit_log` | `core_schema_migrations` |
| Document | `document_receipts`, `document_jobs`, `document_ocr_results`, `document_ocr_items`, `document_exports` | `document_schema_migrations` |
| Gateway | `gateway_max_chats`, `gateway_max_updates`, `gateway_max_chat_groups`, `gateway_notifications`, `gateway_max_personal_subscriptions` | `gateway_schema_migrations` |

Деньги хранятся целыми значениями в копейках. MVP использует `RUB`.

## MinIO

Приватный бакет `receipts` хранит оригиналы чеков и готовые экспорты. Object keys наружу не выдаются. Экспорты скачиваются через авторизованный Gateway или короткоживущую подписанную ссылку. Оригинал чека можно удалить отдельно, сохранив метаданные и OCR-результат.

## Основные потоки

- **Авторизация:** MAX `initData` → проверка подписи → upsert пользователя в Core → локальная HMAC-сессия Gateway.
- **Webhook:** MAX → `POST /api/v1/max/webhook` с секретом → идемпотентная запись в `gateway_max_updates` → асинхронная обработка. Для каждого `message_created` с `/start` Gateway берёт адресата из `message.sender.user_id` и отправляет простое текстовое приветствие через `POST /messages?user_id=...`; `chat_id=0` личного входящего сообщения не используется как адрес назначения.
- **Чек:** изображение или фискальный QR → Document job → OCR/получение позиций → пользователь проверяет данные → отдельный запрос создаёт расход в Core.
- **Расход:** новый расход имеет статус `pending`; только `confirmed` влияет на баланс. Отмена исключает расход из расчёта.
- **Погашение:** отправитель создаёт погашение, получатель подтверждает его в Mini App или подписанным callback из MAX.
- **Экспорт:** Gateway собирает операции Core, Document создаёт CSV/PDF/XLSX и сохраняет файл в MinIO.
- **MAX-сообщения:** `bot_started` технически регистрирует личный чат для возможной доставки уведомлений и экспорта, но не отправляет приветствие. В Mini App нет блока ручного подключения уведомлений. Служебные endpoints `/api/v1/max-subscription` сохранены в публичном контракте, а бизнес-уведомления доставляются best-effort и не меняют результат финансовой операции.

## Контракты и безопасность

- публичный контракт: `api/openapi.yaml`;
- внутренние контракты: `proto/core/v1/*` и `proto/document/v1/document.proto`;
- Gateway передаёт `x-request-id` и доверенный пользовательский контекст по gRPC;
- generated-код не редактируется вручную и проверяется `make generated-check`;
- секреты, Authorization, initData и сырой OCR-текст не логируются;
- runtime-образ Gateway добавляет системные `ca-certificates` и доверенные российские root/sub CA для проверки TLS MAX API; TLS verification не отключается;
- публичные MinIO/PostgreSQL/gRPC-порты в production не открываются.
