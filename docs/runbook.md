# Delim — production-деплой

Это инструкция по деплою Delim. Все команды выполняются из корня репозитория.

## Требования

- Linux-сервер `x86_64` с публичными портами 80 и 443;
- DNS-запись публичного домена ведущая на сервер;
- Git, GNU Make, Docker Engine и Docker Compose v2;
- Go версии из `go.mod` — только для `make max-setup` и `make max-check`.

## 1. Получение кода

```bash
git clone https://github.com/BMSTU-LYGO/Delim.git
cd Delim
```

## 2. Production-окружение

```bash
cp deployments/prod/.env.example deployments/prod/.env
chmod 600 deployments/prod/.env
```

Заполните `deployments/prod/.env`:

- `DELIM_PUBLIC_HOST` — домен без `https://`;
- `MAX_WEBHOOK_URL=https://<домен>/api/v1/max/webhook`;
- `MAX_MINI_APP_URL=https://<домен>/`;
- учётные данные PostgreSQL и MinIO;
- `MAX_BOT_TOKEN` и `MAX_BOT_USERNAME`;
- четыре разных секрета длиной не менее 32 символов: `MAX_WEBHOOK_SECRET`, `GATEWAY_SESSION_SECRET`, `GATEWAY_INVITE_SECRET`, `GATEWAY_LAUNCH_SECRET`;
- `PROVERKACHECKA_TOKEN` - если необходимо получать данные по QR-коду на чеке.

`GATEWAY_CORS_ALLOWED_ORIGINS` при same-origin деплое оставьте пустым. `DELIM_GATEWAY_URL` в production не нужен: Mini App обращается к Gateway через тот же домен.

Секреты можно создать командой `openssl rand -hex 32`. Не выводите заполненный `.env` в логи и не коммитьте его.

## 3. Проверка конфигурации

```bash
make prod-check
```

Проверка валидирует обязательные значения, HTTPS-адреса, уникальность секретов и production Compose.

## 4. Первый запуск

```bash
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml up -d postgres minio minio-init
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml run --rm migrate
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml up -d --build
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml ps
```

`migrate` — одноразовый контейнер и после успешного выполнения завершается с кодом 0. Остальные сервисы должны перейти в `running/healthy`.

## 5. Настройка MAX

Команды требуют установленный Go версии из `go.mod`:

```bash
set -a
. deployments/prod/.env
set +a
make max-setup
make max-check
```

`max-setup` регистрирует webhook и единственную команду меню `/start`. Среди `update_types` должны присутствовать `bot_started` и `message_created`. `max-check` проверяет токен, точный набор команд, webhook и Mini App URL; старые `/help`, `/new` или `/balance` отображаются как `MISMATCH`.

## 6. Проверка после запуска

```bash
set -a
. deployments/prod/.env
set +a
curl -fsS "https://$DELIM_PUBLIC_HOST/health/live"
curl -fsS "https://$DELIM_PUBLIC_HOST/health/ready"
curl -I "https://$DELIM_PUBLIC_HOST/"
```

`/health/ready` возвращает состояния `core`, `document`, `ocr` и `postgres`. Значение OCR `degraded` не делает весь сервис неготовым.

## Обновление

```bash
git status --short
git pull --ff-only origin master
make prod-check
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml up -d postgres minio minio-init
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml run --rm migrate
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml up -d --build
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml ps
```

Не используйте `git stash` для постоянных серверных изменений: конфигурация должна находиться либо в репозитории, либо в `deployments/prod/.env`.

После обновления повторите health-проверки. `make max-setup` нужен повторно только при изменении webhook/бота; `make max-check` можно запускать после каждого релиза.

## Логи и остановка

```bash
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml logs --tail=100 gateway core document miniapp
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml logs -f
docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml down
```

`down` сохраняет volumes. Не добавляйте `-v`, если не требуется удалить production-данные.

## Эксплуатация и восстановление

Для сокращённых production-команд загрузите окружение и задайте Compose-команду:

```bash
COMPOSE="docker compose --env-file deployments/prod/.env -f deployments/prod/compose.yaml"
set -a
. deployments/prod/.env
set +a
```

Общая проверка:

```bash
$COMPOSE ps
$COMPOSE logs --tail=100 gateway core document postgres minio miniapp
curl -fsS "https://$DELIM_PUBLIC_HOST/health/live"
curl -fsS "https://$DELIM_PUBLIC_HOST/health/ready"
```

`ready` возвращает `status`, `core`, `document`, `ocr`, `postgres`. HTTP 503 означает недоступность Core, Document или PostgreSQL. OCR может быть `degraded` при общем HTTP 200.

## Core недоступен

Gateway возвращает 503 для финансовых операций; ledger не изменяется.

```bash
$COMPOSE logs --tail=100 core postgres
$COMPOSE restart core
```

## Document или OCR недоступны

При недоступном Document новые загрузки чеков и экспорты возвращают 503. Финансовые операции Core продолжают работать. `ocr=degraded` означает, что Document доступен, но OCR-провайдер работает ограниченно.

```bash
$COMPOSE logs --tail=150 document
$COMPOSE restart document
```

Задачи, оставшиеся в `processing`, при старте Document восстанавливаются согласно `worker.stale_after_minutes`.

```bash
$COMPOSE exec -T postgres psql -U "$POSTGRES_USER" -d delim -c \
  "SELECT status, count(*) FROM document_jobs GROUP BY status ORDER BY status"
```

## PostgreSQL недоступен

```bash
$COMPOSE logs --tail=100 postgres
$COMPOSE restart postgres
```

Перед восстановлением проверьте выбранный dump и остановите Gateway/Core/Document, чтобы исключить новые записи.

## MinIO недоступен

Недоступны загрузка оригиналов и файловые экспорты; финансовый ledger не затрагивается.

```bash
$COMPOSE logs --tail=100 minio minio-init
$COMPOSE restart minio
$COMPOSE run --rm minio-init
```

`minio-init` идемпотентно создаёт приватный бакет `receipts`.

## MAX webhook и доставка сообщений

```bash
make max-check
make max-setup   # повторная регистрация при неверной конфигурации
$COMPOSE logs --tail=150 gateway
```

Для этих Makefile-команд переменные из `deployments/prod/.env` должны быть экспортированы, а на хосте нужен Go версии из `go.mod`.

## Backup / Restore

`make db-backup` и `make db-restore` предназначены для dev-compose. Для production используйте тот же PostgreSQL custom format напрямую через production Compose:

```bash
$COMPOSE exec -T postgres pg_dump -U "$POSTGRES_USER" -d delim --format=custom --file=/tmp/delim.dump
$COMPOSE cp postgres:/tmp/delim.dump ./delim.dump
```

Восстановление заменяет данные и является разрушительной операцией. Перед ним остановите сервисы приложения и сохраните текущую БД:

```bash
$COMPOSE stop gateway core document
$COMPOSE cp ./delim.dump postgres:/tmp/delim.dump
$COMPOSE exec -T postgres pg_restore -U "$POSTGRES_USER" -d delim --clean --if-exists --no-owner --no-privileges /tmp/delim.dump
$COMPOSE start core document gateway
```

После восстановления проверьте `core_schema_migrations`, `document_schema_migrations`, `gateway_schema_migrations` и `/health/ready`.

## Откат приложения

Миграции проекта рассчитаны на совместимость назад, но автоматического downgrade нет. Перед релизом с изменением схемы сделайте backup.

```bash
git switch --detach <previous-commit-or-tag>
$COMPOSE up -d --build gateway core document miniapp
```

Если старая версия несовместима с текущими данными, восстановите предрелизный dump. После устранения инцидента вернитесь на нужную ветку/коммит и повторите процедуру из deployment-инструкции.
