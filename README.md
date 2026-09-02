# Delim Backend

Backend проекта Delim, разделённый на сервисы `gateway`, `core` и `document`.

## Зависимости

- Go
- Docker
- Make
- protoc

## Запуск

```sh
cp .env.example .env
make proto
make run
```

Остановка: `make down`.

## Структура

```text
build/                  единый Dockerfile для backend-сервисов
cmd/                    точки входа gateway, core и document
configs/                отдельный YAML-конфиг каждого сервиса
deployments/dev/        локальный Docker Compose
internal/
  gateway/              HTTP API, MAX и gRPC-клиенты
  core/                 основной домен и доступ к PostgreSQL
  document/             документы, OCR и Object Storage
migrations/             миграции общей PostgreSQL
pkg/
  configenv/            загрузка YAML и ENV override
  gen/                  сгенерированный protobuf-код
  grpcx/                общая настройка gRPC
  logger/               настройка slog
  maxapi/, maxauth/      инфраструктура интеграции с MAX
  postgresx/            подключение к PostgreSQL через pgxpool
proto/                  контракты CoreService и DocumentService
web/miniapp/            место для Mini App
```

`cmd` только загружает конфигурацию и запускает соответствующий `internal/<service>/app`. Сборка зависимостей и жизненный цикл сервиса находятся в `app`, транспортный код — в `delivery` и gRPC-клиентах, а общая инфраструктура — в `pkg`.
