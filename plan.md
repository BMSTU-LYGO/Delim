# DELIM_DOCUMENT_2_FINAL

Цель: полностью закончить Document Service проекта Delim.

Репозиторий:
https://github.com/BMSTU-LYGO/Delim

Текущее состояние НЕ переписывать с нуля.
Python foundation уже существует.

После этого плана Document должен быть готов для интеграции:

Gateway
→ gRPC
→ Document Python
→ QR / OCR / Export
→ PostgreSQL + MinIO

Document НЕ создаёт Expense и НЕ изменяет Ledger.
Результат OCR всегда сначала подтверждается пользователем.

---

# ПРАВИЛА РАБОТЫ

- Работать в текущей ветке.
- Не создавать новые ветки.
- После КАЖДОГО пункта отдельный commit.
- Commit называется ровно как указано.
- После пункта писать только:

`<commit> — done`

- Минимизировать расход токенов.
- Не писать промежуточные отчёты.
- Не пересказывать код.
- Не перечитывать весь repo после каждого пункта.
- Читать только файлы текущего блока.
- Не трогать Core.
- Не добавлять бизнес-логику в Gateway.
- Gateway не менять, кроме regenerated Go proto.
- Не писать mocks.
- Не добавлять CI/CD.
- Не добавлять Redis/Kafka/RabbitMQ.
- Не использовать ORM.
- PostgreSQL — asyncpg.
- gRPC — grpc.aio.
- Деньги — только int64 minor units.
- Не использовать float для денежных значений.
- Confidence может быть float.
- Generated protobuf руками не менять.
- Не хранить секреты в YAML.
- Не добавлять абстракции без текущего использования.
- Тесты писать только для QR/parser/финальных OCR invariants.
- После каждого пункта выполнять минимальную проверку затронутого кода.
- Полный Docker/OCR запуск делать только в финальных блоках.

---

# БЛОК 1. Починить текущий foundation

## 1.1. Синхронизировать Document proto

Сейчас Python gRPC server использует RPC, которых нет
в source `proto/document/v1/document.proto`.

Сделать source of truth нормальным.

DocumentService:

- Ping
- CreateReceipt
- GetReceipt
- GetDocumentJob
- DeleteReceipt

Описать:

Receipt
DocumentJob
ReceiptStatus
DocumentJobStatus
DocumentJobType

Использовать:

google.protobuf.Timestamp

Деньги здесь пока не добавлять.

После изменения:

make proto
make document-proto

Проверить, что:
- Go client генерируется;
- Python server stubs генерируются;
- generated код руками не изменён.

Commit:

`document: synchronize protobuf contract`

---

## 1.2. Исправить document.yaml

Текущий Python config loader уже ожидает больше полей,
чем находится в YAML.

Добавить:

postgres:
  min_connections: 1
  max_connections: 10

upload:
  max_size_mb: 10

Также добавить секции для следующих блоков:

worker:
  poll_interval_ms: 500
  max_attempts: 3
  retry_base_seconds: 2
  stale_after_minutes: 10

ocr:
  language: ru
  confidence_threshold: 0.45

Не хранить secrets.

Commit:

`document: complete service configuration`

---

## 1.3. Удалить старый Go Document

Python Document уже является основной реализацией.

Удалить:

- cmd/document/**
- internal/document/**

Не удалять:
- proto/document;
- pkg/gen/document;
- configs/document.yaml.

Исправить Makefile:

`make build`

должен собирать:
- gateway;
- gatewayctl;
- core;

и проверять Python Document отдельно.

Добавить:

- document-install
- document-proto
- document-run
- document-migrate

Не пытаться делать `go build ./cmd/document`.

README обновить минимально.

Commit:

`document: remove legacy Go service`

---

# БЛОК 2. Зависимости OCR/CV

## 2.1. Добавить CV/OCR dependencies

В `document/pyproject.toml` добавить только:

- opencv-python-headless
- numpy
- Pillow
- paddleocr
- paddlepaddle

Использовать CPU-вариант Paddle.

Зафиксировать совместимые версии после проверки установки.

Для русского OCR использовать:

- lang = ru
- PP-OCRv5

Не использовать GPU.

Не добавлять:
- torch;
- transformers;
- EasyOCR;
- pytesseract.

Commit:

`document: add OCR dependencies`

---

# БЛОК 3. Preprocessing

## 3.1. Реализовать загрузку изображения

Создать:

`document/src/delim_document/image/`

Минимально:

- decoder.py
- preprocess.py

Decoder:

bytes
→ OpenCV image.

Проверить:
- изображение реально декодируется;
- width/height > 0;
- ограничить аномально большое resolution.

Не доверять только magic bytes upload validation.

Commit:

`document: add image decoding`

---

## 3.2. Реализовать preprocessing

Подготовить изображение для OCR:

- orientation normalization при возможности;
- grayscale;
- лёгкое denoise;
- contrast normalization / CLAHE;
- deskew при уверенном определении угла;
- resize слишком маленьких чеков.

Не применять агрессивное thresholding всегда.

Сделать два варианта:

1. normal/enhanced;
2. binarized fallback.

OCR позднее выбирает лучший результат по confidence.

Не сохранять промежуточные изображения в PostgreSQL/MinIO.

Commit:

`document: implement receipt preprocessing`

---

# БЛОК 4. QR российских чеков

## 4.1. Реализовать QR reader

Создать:

`qr/reader.py`

Использовать OpenCV QRCodeDetector.

Проверять QR:

1. на original image;
2. на enhanced image;
3. при необходимости на grayscale.

Возвращать:

- raw payload;
- decoded flag.

Отсутствие QR НЕ является ошибкой OCR job.

Commit:

`document: implement receipt QR reading`

---

## 4.2. Разобрать fiscal QR

Создать:

`qr/fiscal.py`

Поддержать типичный payload российского чека:

`t=...&s=...&fn=...&i=...&fp=...&n=...`

Извлечь при наличии:

- timestamp/date;
- total_minor из `s`;
- fn;
- fiscal document number;
- fiscal sign;
- operation type.

Парсинг денег:
через Decimal → int minor units.

НЕ float.

Не обращаться к ФНС/ОФД API.

Не считать QR обязательным.

Добавить небольшие unit tests для parser.

Commit:

`document: parse fiscal receipt QR`

---

# БЛОК 5. Реальный OCR Provider

## 5.1. Реализовать PaddleOCRProvider

Заменить Stub как production provider.

Stub можно оставить только как отдельный dev implementation.

PaddleOCRProvider:

- создаёт PaddleOCR model ОДИН РАЗ при startup;
- не загружает модель на каждый job;
- русский язык;
- PP-OCRv5;
- возвращает recognized lines;
- text;
- bounding box;
- confidence.

Не смешивать Paddle-specific result с domain OCRResult.

Создать внутреннюю модель:

OCRLine:
- text
- confidence
- bbox

Commit:

`document: implement PaddleOCR provider`

---

## 5.2. Не блокировать asyncio

PaddleOCR синхронный/CPU-heavy.

Не выполнять inference непосредственно в event loop.

Использовать:

`asyncio.to_thread`

или эквивалентный bounded executor.

По умолчанию:

worker concurrency = 1.

Не создавать новый model instance в executor для каждого job.

Commit:

`document: isolate OCR inference`

---

# БЛОК 6. Разбор структуры чека

## 6.1. Нормализовать OCR lines

Создать:

`ocr/parser.py`

Перед парсингом:

- сортировать строки сверху вниз;
- учитывать bbox X/Y;
- trim whitespace;
- нормализовать повторные пробелы;
- исправлять только безопасные OCR-артефакты вокруг денежных сумм;
- не "угадывать" текст товара без основания.

Сохранять raw lines отдельно.

Commit:

`document: normalize OCR output`

---

## 6.2. Определять продавца

Merchant искать преимущественно в верхней части чека.

Исключать строки типа:

- КАССОВЫЙ ЧЕК;
- ИНН;
- ФН;
- ФД;
- ФП;
- КАССИР;
- СМЕНА.

Не считать merchant обязательным.

Возвращать:
- value;
- confidence.

Commit:

`document: extract receipt merchant`

---

## 6.3. Определять дату

Источники по приоритету:

1. fiscal QR;
2. OCR.

Поддержать основные форматы:

- DD.MM.YYYY HH:MM
- DD-MM-YYYY
- YYYY-MM-DD

Не брать произвольное число за дату.

QR date при корректном fiscal QR имеет более высокий confidence.

Commit:

`document: extract receipt date`

---

## 6.4. Определять итог

Искать строки:

- ИТОГ
- ИТОГО
- К ОПЛАТЕ
- ВСЕГО

Учитывать положение суммы справа.

Если есть fiscal QR total:
он имеет приоритет.

Если QR и OCR total расходятся:
- сохранить QR total как основной;
- отметить mismatch;
- понизить overall confidence.

Деньги:
Decimal → int minor.

Commit:

`document: extract receipt total`

---

## 6.5. Извлекать позиции

Реализовать простой deterministic receipt parser.

Для item искать:

- название;
- amount;
- quantity если распознана;
- unit price если распознана.

Использовать геометрию bbox:
денежные значения обычно справа.

Поддержать перенос названия на несколько OCR lines.

Не считать item строками:

- ИТОГ;
- НДС;
- НАЛИЧНЫМИ;
- БЕЗНАЛИЧНЫМИ;
- СДАЧА;
- ФН;
- ФД;
- ФП;
- ИНН;
- кассир;
- служебные строки.

Если позицию нельзя уверенно распарсить:
оставить её только в raw text,
а не создавать ложный OCRItem.

Commit:

`document: extract receipt items`

---

# БЛОК 7. Confidence

## 7.1. Сделать confidence model

Не выдумывать случайный confidence.

Использовать:

- confidence PaddleOCR lines;
- источник поля;
- наличие fiscal QR;
- согласованность QR/OCR total;
- согласованность суммы items и total.

Для каждого field:
0.0–1.0.

OCRItem:
0.0–1.0.

Overall result:
0.0–1.0.

Не отклонять чек только из-за низкого confidence.

Low confidence означает:
пользователь должен внимательнее проверить результат.

Commit:

`document: calculate OCR confidence`

---

# БЛОК 8. Хранение OCR результата

## 8.1. Добавить migration

Не менять `001_init.sql`.

Создать:

`migrations/document/002_ocr_results.sql`

### document_ocr_results

- id
- receipt_id UNIQUE FK
- qr_raw NULL
- merchant NULL
- receipt_date NULL
- total_minor NULL
- currency NULL
- confidence
- raw_text
- raw_lines JSONB
- created_at
- updated_at

### document_ocr_items

- id
- result_id FK
- position
- name
- quantity NUMERIC NULL
- unit_price_minor BIGINT NULL
- amount_minor BIGINT NOT NULL
- confidence
- created_at

Добавить CHECK:
- confidence 0..1;
- money >= 0;
- position >= 0.

Не хранить money float.

Commit:

`document: add OCR result schema`

---

## 8.2. Реализовать OCR result repository

Создать:

`repository/ocr_result.py`

Методы:

- replace_result;
- get_by_receipt.

`replace_result` должен одной transaction:

- upsert result;
- заменить items.

Result доступен только actor,
которому принадлежит Receipt.

Commit:

`document: persist OCR results`

---

# БЛОК 9. Background worker

## 9.1. Сделать атомарный claim jobs

Расширить JobRepository.

Добавить:

`claim_pending`

Использовать PostgreSQL:

`FOR UPDATE SKIP LOCKED`

Claim должен атомарно:

pending
→ processing
→ attempts + 1

Добавить migration при необходимости:

`003_job_retry.sql`

с:

- next_attempt_at;
- updated_at при необходимости.

Не допускать обработки одного Job двумя worker.

Commit:

`document: claim OCR jobs safely`

---

## 9.2. Реализовать OCR worker

Создать:

`worker/ocr.py`

Flow:

pending job
→ load Receipt
→ load image from MinIO
→ decode
→ QR
→ preprocess
→ OCR
→ parse
→ confidence
→ save OCRResult
→ receipt ready
→ job completed

При processing:

Receipt status:
processing.

При success:

Receipt:
ready.

Job:
completed.

Commit:

`document: implement OCR worker`

---

## 9.3. Retry и failures

Разделить ошибки.

Retryable:

- PostgreSQL temporary failure;
- MinIO temporary failure;
- временная OCR runtime ошибка.

Non-retryable:

- corrupted image;
- unsupported image;
- deterministic parser/input error.

Retry:

- max_attempts из YAML;
- exponential backoff;
- next_attempt_at.

После лимита:

job → failed
receipt → failed

error_message:
короткое безопасное сообщение.

Не сохранять stack trace в БД.

Commit:

`document: add OCR retry policy`

---

## 9.4. Recovery после падения

При startup либо периодически:

processing job старше `stale_after_minutes`
→ вернуть в pending.

Не трогать свежие processing jobs.

Это должно позволить пережить crash контейнера.

Commit:

`document: recover stale OCR jobs`

---

## 9.5. Подключить worker в App

App должен запускать:

- PostgreSQL;
- MinIO;
- OCR provider;
- gRPC server;
- OCR worker.

Shutdown:

1. перестать брать новые jobs;
2. корректно закончить/прервать worker;
3. остановить gRPC;
4. закрыть PostgreSQL.

Не запускать OCR provider до проверки config/dependencies.

Commit:

`document: wire OCR processing`

---

# БЛОК 10. OCR gRPC API

## 10.1. Добавить GetOCRResult

Расширить proto:

`GetOCRResult`

Request:

- actor_user_id
- receipt_id

Response:

- merchant
- date
- total_minor
- currency
- items
- confidence
- qr_found
- processing status при необходимости

Не возвращать:
- MinIO object key;
- raw internal errors.

Если job ещё processing:
вернуть корректное состояние,
не INTERNAL.

Commit:

`document: expose OCR result API`

---

## 10.2. Добавить RetryReceiptOCR

Добавить:

`RetryReceiptOCR`

Разрешить только:
- failed receipt.

Создать новый pending job
или безопасно переиспользовать модель jobs.

Не позволять одновременно создавать много active OCR jobs
для одного Receipt.

Commit:

`document: expose OCR retry API`

---

# БЛОК 11. Export subsystem

Document должен только РЕНДЕРИТЬ данные,
которые ему передал Gateway/Core.

Document не читает Core PostgreSQL напрямую.

---

## 11.1. Добавить Export contract

Добавить proto:

ExportFormat:
- CSV
- PDF
- XLSX

`CreateExport`

Request:
- actor_user_id;
- group_id;
- group_name;
- format;
- normalized report rows/data.

Document не проверяет финансовую математику.

Также:

- GetExport
- DownloadExport streaming.

Не возвращать permanent public MinIO URL.

Commit:

`document: define export contract`

---

## 11.2. Добавить Export schema

Создать migration:

`004_exports.sql`

document_exports:

- id
- actor_user_id
- group_id
- format
- status
- object_key NULL
- filename
- error_code NULL
- created_at
- finished_at NULL

Status:
- pending
- processing
- ready
- failed

Сырые финансовые данные после генерации отчёта
без необходимости постоянно не хранить.

Commit:

`document: add export schema`

---

## 11.3. CSV renderer

Использовать Python stdlib `csv`.

UTF-8.

Экспорт должен корректно поддерживать:
- русский текст;
- деньги minor → printable decimal;
- даты;
- все строки отчёта.

Финансовую математику не выполнять.

Commit:

`document: implement CSV export`

---

## 11.4. XLSX renderer

Добавить только:

`openpyxl`

Сделать простой читаемый workbook:

- заголовок группы;
- таблица операций;
- фиксированные headers;
- auto width в разумных пределах;
- monetary cells без float-потери.

Не делать сложный дизайн.

Commit:

`document: implement XLSX export`

---

## 11.5. PDF renderer

Добавить:

`reportlab`

Поддержать кириллицу через установленный
в Docker системный Unicode-шрифт.

НЕ коммитить font binary в repository.

PDF:
- название группы;
- дата формирования;
- простая таблица операций;
- перенос на несколько страниц.

Не делать сложный дизайн.

Commit:

`document: implement PDF export`

---

## 11.6. Хранение и скачивание exports

Готовые файлы сохранять в private MinIO:

`exports/<group_id>/<export_id>/...`

`DownloadExport`:

server-streaming chunks.

Не загружать весь большой export в память Gateway.

Проверять actor_user_id.

Commit:

`document: expose export files`

---

# БЛОК 12. Docker

## 12.1. Обновить Python image

Обновить:

`build/Dockerfile.document`

Установить только необходимые system packages
для PaddleOCR/OpenCV/PDF.

CPU only.

Использовать non-root user.

Модель OCR не должна загружаться заново
на каждый restart, если это можно избежать.

Предпочтительно:
предзагрузить нужную PP-OCRv5 ru модель
на build stage либо иметь persistent model cache.

Runtime не должен зависеть от случайной загрузки другой версии модели.

Не добавлять GPU/CUDA.

Commit:

`document: finalize OCR Docker image`

---

## 12.2. Проверить resource limits

Document значительно тяжелее Core/Gateway.

В dev compose не менять остальные сервисы.

Для Document:
- не запускать много OCR workers;
- default concurrency = 1;
- модель одна на process.

Не делать autoscaling infrastructure.

Commit:

`document: configure OCR resources`

---

# БЛОК 13. Критические тесты

## 13.1. Parser tests

Добавить только маленькие pure tests:

- fiscal QR;
- money parsing;
- date parsing;
- TOTAL detection;
- merchant;
- item parsing;
- Cyrillic;
- QR/OCR mismatch.

Не тестировать Paddle model.

Не добавлять огромные fixture images в git.

Commit:

`document: test receipt parsing`

---

## 13.2. Result invariants

Проверить:

- confidence 0..1;
- amount_minor integer;
- item money integer;
- item order deterministic;
- empty OCR не падает;
- receipt без QR работает;
- receipt без items работает;
- QR-only total/date работает.

Commit:

`document: test OCR invariants`

---

# БЛОК 14. Runtime smoke

## 14.1. Проверить reproducible build

Выполнить:

make proto
make document-proto
make document-migrate
python -m compileall document/src
go build ./cmd/gateway
go build ./cmd/core
docker compose -f deployments/dev/compose.yaml config

Проверить отсутствие generated drift.

Commit:

`document: verify reproducible build`

---

## 14.2. Реальный OCR smoke

Поднять:

- PostgreSQL;
- MinIO;
- Document.

Использовать локальный тестовый русский чек,
НЕ коммитить его.

Проверить:

CreateReceipt
→ queued
→ processing
→ ready
→ GetOCRResult

Проверить минимум:

- OCR возвращает текст;
- total либо найден, либо корректно NULL;
- date либо найдена, либо NULL;
- items не содержат очевидные ФН/ФД/ИТОГ как товары;
- confidence валиден.

Не требовать идеального OCR от плохого фото.

Commit:

`document: verify OCR pipeline`

---

## 14.3. QR smoke

На чеке с fiscal QR проверить:

QR
→ parse
→ total/date

Если OCR total отличается:
QR остаётся источником primary total.

Commit:

`document: verify QR pipeline`

---

## 14.4. Failure smoke

Проверить:

- битое изображение;
- unsupported file;
- удалённый MinIO;
- restart во время processing.

Ожидание:

- gRPC не падает;
- job получает правильный status;
- retry работает;
- stale processing восстанавливается;
- INTERNAL не содержит secrets/stacktrace.

Commit:

`document: verify failure recovery`

---

## 14.5. Export smoke

Проверить:

CSV
PDF
XLSX

на русском тестовом наборе.

Проверить:

CreateExport
→ ready
→ DownloadExport

Не коммитить generated файлы.

Commit:

`document: verify exports`

---

# БЛОК 15. Финальная зачистка

## 15.1. Проверить границы сервиса

Document НЕ должен:

- обращаться к Core DB;
- создавать Expense;
- рассчитывать split;
- изменять Ledger;
- работать с MAX API;
- содержать HTTP frontend API.

Document отвечает только за:

- receipt upload/storage;
- QR;
- preprocessing;
- OCR;
- structured OCR result;
- jobs;
- exports.

Commit:

`document: enforce service boundaries`

---

## 15.2. README

Коротко обновить README.

Указать:

Document — Python.

Функции:

- receipt upload;
- private MinIO;
- QR;
- PaddleOCR Russian;
- OCR items;
- confidence;
- background jobs;
- CSV/PDF/XLSX;
- gRPC.

Убрать старый текст про временный Go Document.

Добавить команды:

make document-proto
make document-migrate
make document-run

Не писать длинный README.

Commit:

`document: update service documentation`

---

## 15.3. Финальная проверка

Выполнить:

make proto
make document-proto
make document-migrate

python -m compileall document/src

go build ./cmd/gateway
go build ./cmd/core

docker compose -f deployments/dev/compose.yaml config
docker compose -f deployments/dev/compose.yaml up --build

Проверить:

- gateway running;
- core running;
- document running;
- postgres healthy;
- minio healthy;
- document Ping;
- OCR flow;
- export flow.

После проверки:

make down

Commit:

`document: finalize document service`

---

# DEFINITION OF DONE

После этого Document Service считается DONE.

Должно работать:

1. Document полностью Python.
2. Старой Go-реализации нет.
3. Source proto соответствует Python и Go generated code.
4. Отдельный YAML config работает.
5. Secrets только ENV.
6. PostgreSQL migrations работают.
7. MinIO private storage работает.
8. Upload JPEG работает.
9. Upload PNG работает.
10. Upload WebP работает.
11. Size limit работает.
12. Corrupted images отклоняются.
13. QR detection работает.
14. Fiscal QR parser работает.
15. Preprocessing работает.
16. PaddleOCR ru работает.
17. OCR model создаётся один раз.
18. OCR не блокирует gRPC event loop.
19. Merchant извлекается.
20. Date извлекается.
21. Total извлекается.
22. QR имеет приоритет для fiscal total/date.
23. Items извлекаются.
24. OCR raw result сохраняется.
25. Structured OCR result сохраняется.
26. Confidence сохраняется.
27. Money не хранится float.
28. Worker работает в фоне.
29. Jobs атомарно claim-ятся.
30. Один job не обрабатывается дважды.
31. Retry работает.
32. Stale jobs восстанавливаются.
33. Failed status работает.
34. Ready status работает.
35. GetOCRResult работает.
36. RetryReceiptOCR работает.
37. CSV export работает.
38. XLSX export работает.
39. PDF export с кириллицей работает.
40. Export хранится приватно.
41. DownloadExport streaming работает.
42. Document не обращается к Core DB.
43. Document не создаёт финансовые операции.
44. Docker Compose поднимает сервис.
45. Gateway/Core не сломаны.

После этого изменения Document допускаются только как:
- OCR quality improvement;
- bugfix;
- новый provider;
- новая продуктовая функция.

В конце вывести ТОЛЬКО:

`DELIM_DOCUMENT_2_FINAL done — <N commits>`