# Delim — операционный runbook

Все команды — из корня репозитория. Локально стек — `deployments/dev/compose.yaml`
(ярлык `make`), production — `deployments/prod/compose.yaml` (имя проекта
`delim-prod`).

## Деплой (production)

```bash
cp deployments/prod/.env.example deployments/prod/.env   # заполнить сильные секреты
make prod-check                                          # fail-fast на плохой конфиг
docker compose -f deployments/prod/compose.yaml up -d --build postgres minio minio-init
docker compose -f deployments/prod/compose.yaml run --rm migrate
docker compose -f deployments/prod/compose.yaml up -d --build
docker compose -f deployments/prod/compose.yaml ps        # все сервисы healthy
make max-setup                                            # подписка MAX (нужен токен)
make max-check                                            # диагностика интеграции
```

Caddy сам выпускает/продлевает сертификат ACME для `DELIM_PUBLIC_HOST`. Сертификаты
в репозиторий не попадают (volume `caddy-data`).

## Health failure

```bash
curl -fsS https://$DELIM_PUBLIC_HOST/health/live         # процесс жив
curl -fsS https://$DELIM_PUBLIC_HOST/health/ready         # зависимости: core/document/postgres
docker compose -f deployments/prod/compose.yaml ps
docker compose -f deployments/prod/compose.yaml logs --tail=100 gateway core document
```

`ready` отдаёт 503, если любой из Core/Document/PostgreSQL недоступен (в ответе —
статусы по компонентам, без деталей внутренних ошибок).

## Core недоступен

Gateway отвечает 503 на финансовые операции; ledger не меняется.

```bash
docker compose -f deployments/prod/compose.yaml logs core | tail -50
docker compose -f deployments/prod/compose.yaml restart core
# если БД лежит — см. "PostgreSQL недоступен"
```

## Document недоступен

OCR/экспорт встают в очередь (`pending`); обычные финансовые операции продолжают
работать. Мини-апп показывает «Сервис временно недоступен».

```bash
docker compose -f deployments/prod/compose.yaml logs document | tail -80
docker compose -f deployments/prod/compose.yaml restart document
```

## PostgreSQL недоступен

```bash
docker compose -f deployments/prod/compose.yaml logs postgres | tail -50
docker compose -f deployments/prod/compose.yaml restart postgres
# восстановление из дампа (см. Backup/Restore) если данные повреждены
```

## MinIO недоступен

Загрузка/скачивание оригиналов и экспортов падает; ledger не затронут.

```bash
docker compose -f deployments/prod/compose.yaml logs minio | tail -50
docker compose -f deployments/prod/compose.yaml restart minio
# бакет приватный; при случайной публичности пере-применить:
docker compose -f deployments/prod/compose.yaml run --rm minio-init
```

## Залипшие OCR-задачи

Job в `processing` дольше `worker.stale_after_minutes` восстанавливается при старте
Document (`recover_stale`) в `pending`/`failed`.

```bash
# проверить очередь
docker compose -f deployments/prod/compose.yaml exec -T postgres \
  psql -U "$POSTGRES_USER" -d delim -c \
  "SELECT status, count(*) FROM document_jobs GROUP BY status"
# перезапуск Document перенимает залипшие задачи
docker compose -f deployments/prod/compose.yaml restart document
```

Метрики очереди: `delim_ocr_jobs_total{state}`, `delim_ocr_duration_seconds`
(job=document).

## Плохая webhook-подписка

```bash
make max-check     # покажет subscriptions, update types, несовпадения URL/секрета
make max-setup     # пересоздаёт подписку на MAX_WEBHOOK_URL с нужными update types
```

Дубликаты/отказы вебхуков видны в `delim_webhook_events_total{result}`.

## Откат релиза

```bash
# образы собираются из кода; откат = checkout предыдущего тега/коммита и пересборка
git checkout <previous-tag>
docker compose -f deployments/prod/compose.yaml up -d --build gateway core document miniapp
# миграции Delim обратно совместимы (additive). При откате схемы — восстановить
# бэкап, снятый до миграции (см. ниже).
```

## Backup / Restore (dev/demo)

```bash
make db-backup                                   # backups/delim-<ts>.dump (git-ignored)
make db-backup FILE=/tmp/delim.dump
make db-restore FILE=/tmp/delim.dump             # pg_restore + сверка *_schema_migrations
```

Для production аналогично через `docker compose -f deployments/prod/compose.yaml exec
postgres pg_dump/pg_restore`. Файлы бэкапов не коммитить.

## Наблюдаемость (опционально, локально)

```bash
make observability-up     # Prometheus :9099, Grafana :3000 (admin/admin), профильно
make observability-down
```

Не входит в обычный `make dev-up`; скрейпит внутренние `/metrics` сервисов.
