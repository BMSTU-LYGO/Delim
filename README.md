# Delim Backend

Backend проекта Delim, разделённый на сервисы `gateway`, `core` и `document`.

## Зависимости

- Go
- Python 3.12
- Node.js 22.12+
- Docker
- Make
- protoc

## Запуск

```sh
cp .env.example .env
make proto
make dev-up
```

Остановка: `make down`.

`make dev-init` поднимает PostgreSQL и MinIO и идемпотентно применяет миграции
Core, Document и Gateway. `make dev-up` после этого запускает весь локальный stack
и возвращается, когда сервисы проходят healthchecks.

После запуска `make smoke` проверяет readiness, группы, расходы, баланс,
погашения, корректировки, OCR чеков, приватные экспорты и поведение при
недоступном Document. Проверка MAX API запускается автоматически только при
непустом `MAX_BOT_TOKEN`; для обычного локального smoke token не нужен.

Для быстрой локальной демонстрации после запуска stack выполните:

```sh
make demo-seed
make web-install
make web-dev
```

`demo-seed` создаёт новую группу с Марией, Алексеем, Еленой, несколькими
подтверждёнными расходами и одним расходом на проверке. Dev-сессия Марии
записывается в игнорируемый `web/miniapp/.env.local`; production-сборка эти
данные не содержит. Повторный запуск создаёт новую независимую демо-группу.

`make dev-up` также собирает и поднимает production-вариант Mini App на
`http://localhost:8081`. Он предназначен для проверки статической раздачи и
авторизации из MAX. Vite dev server с локальной demo-сессией работает отдельно
на `http://localhost:5173`.

## Структура

```text
build/                  Dockerfile для Go-сервисов и Python Document
cmd/                    точки входа gateway и core
configs/                отдельный YAML-конфиг каждого сервиса
document/               Python Document Service
deployments/dev/        локальный Docker Compose
internal/
  gateway/              HTTP API, MAX и gRPC-клиенты
  core/                 основной домен и доступ к PostgreSQL
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


## Сервисы

Backend разделён на три независимых сервиса. Внешние запросы проходят через `gateway`, финансовая логика находится только в `core`, а работа с документами и распознаванием вынесена в `document`.

### Gateway — Go

Точка входа backend и слой интеграции с MAX.

Отвечает за:
- HTTP API для Mini App;
- авторизацию пользователя через MAX `initData`;
- приём и обработку MAX Webhook;
- работу с MAX Bot API;
- проверку доступа к внешним запросам;
- вызов `core` и `document` по gRPC;
- преобразование внутренних ошибок в HTTP-ответы.

Gateway не должен содержать финансовую бизнес-логику или OCR.

Gateway проверяет MAX `initData`, выдаёт подписанные сессии и startapp invites,
идемпотентно сохраняет Webhook в PostgreSQL, обрабатывает события worker-ом,
ведёт технический registry MAX-чатов и поддерживает Bot, Chat и Messages API.

Полный внешний HTTP-контракт, включая Core facade, receipts, exports и invites,
описан в `api/openapi.yaml`.

Gateway запускается без `MAX_BOT_TOKEN`; недоступны только операции, которым нужен
MAX API. После настройки MAX secrets и HTTPS `MAX_WEBHOOK_URL` используйте
`make max-check` для проверки token и `make max-setup` для команд и Webhook.

### Core — Go

Основной сервис бизнес-логики и источник истины для совместных расходов.

Отвечает за:
- группы и участников;
- роли и права внутри группы;
- создание и изменение расходов;
- распределение суммы поровну, по долям, процентам, фиксированным суммам и позициям;
- расчёт ledger и баланса «кто кому должен»;
- подтверждение операций;
- погашения;
- возвраты и корректировки;
- минимизацию количества переводов;
- журнал финансовых изменений;
- работу с финансовыми данными в PostgreSQL.

Все правила расчётов должны находиться в Core и не зависеть от MAX или Document Service.

### Document — Python

Сервис обработки чеков и других документов.

Document Service реализован на Python 3.12 и доступен внутри системы по gRPC на
порту `50052`. PostgreSQL хранит metadata чеков и OCR jobs, а private bucket
MinIO — оригиналы изображений и готовые экспорты. OCR выполняется PaddleOCR
через изолированный provider interface.

Document отвечает за:
- загрузку чеков;
- хранение оригиналов в Object Storage;
- чтение QR-кодов;
- preprocessing изображений;
- OCR чеков;
- распознавание продавца, даты, итоговой суммы и позиций;
- confidence результата распознавания;
- фоновые задачи обработки документов;
- формирование CSV/PDF/XLSX-экспортов;
- выдачу результата обработки через gRPC.

Document не изменяет финансовый ledger самостоятельно. После распознавания данные сначала подтверждаются пользователем, после чего финансовая операция создаётся через Core.

Получение OCR-результата — строго read-only операция. Ни Document worker, ни Receipt
HTTP handlers Gateway не вызывают `Core.CreateExpense`: клиент должен показать
распознанные поля пользователю и отдельным запросом создать расход после подтверждения.

## Взаимодействие сервисов

Основной поток:

`MAX / Mini App → Gateway → Core`

Для чеков:

`MAX / Mini App → Gateway → Document → результат OCR → Gateway → подтверждение пользователя → Core`

Внутреннее взаимодействие сервисов выполняется по gRPC с контрактами из `proto/`.

Основной стек:
- Mini App — React + TypeScript;
- Gateway — Go;
- Core — Go;
- Document — Python;
- API — HTTP/JSON;
- межсервисное взаимодействие — gRPC + Protobuf;
- БД — PostgreSQL;
- файлы чеков — Object Storage / MinIO.


## Web / Mini App

`web/miniapp` — клиентская часть приложения внутри MAX.

Стек:
- React
- TypeScript
- Vite
- MAX UI
- MAX Bridge

Отвечает за:
- запуск Mini App внутри MAX;
- получение контекста через MAX Bridge;
- авторизацию через `gateway`;
- интерфейс групп и расходов;
- загрузку чеков;
- отображение и исправление результата OCR;
- выбор участников и способа распределения;
- отображение баланса и истории операций;
- подтверждение расходов и погашений.

Frontend не содержит финансовую бизнес-логику и не выполняет OCR самостоятельно.

Команды для разработки frontend:

```sh
make web-install    # установить зафиксированные зависимости
make web-dev        # запустить Vite dev server
make web-typecheck  # проверить TypeScript
make web-build      # собрать production bundle
```

Корневая команда `make build` собирает Go-сервисы, проверяет Python-модули и
создаёт production bundle Mini App в `web/miniapp/dist`.

Production bundle также упакован в `build/Dockerfile.web`: nginx раздаёт только
статические файлы и SPA fallback. Адрес Gateway задаётся контейнеру переменной
`DELIM_GATEWAY_URL` при запуске, поэтому окружение не зашивается в bundle.
Frontend origin должен быть разрешён в Gateway через список
`GATEWAY_CORS_ALLOWED_ORIGINS` (значения разделяются запятыми).

Все запросы идут только через:

`Mini App → Gateway → Core / Document`
