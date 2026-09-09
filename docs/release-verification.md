# Delim — релизная end-to-end верификация (Block 4)

Прогон на живом dev-стеке (Gateway + Core + Document + PostgreSQL + MinIO).
Платформа этого хоста: **linux/aarch64**, где нативный Paddle-инференс
деградирован (Block 2). Флаг `ocr` в readiness это честно показывает.

## 4.1 Receipt → OCR → Expense

Инвариант «OCR сам по себе расход НЕ создаёт» подтверждён `make smoke`
(receipt-история: счётчик расходов группы не меняется до явного `confirm`).

Живой прогон контроля пути чека (деградированный OCR на aarch64):
- CreateGroup → Upload receipt → job `queued` → `processing` → `failed`
  (нативный Paddle SIGSEGV изолирован в subprocess, Document остаётся healthy);
- GetReceipt/GetOCRResult/RetryReceiptOCR/DeleteReceiptOriginal обслуживаются;
- расход НЕ создан автоматически (инвариант соблюдён).

Команда: `go run ./cmd/smoke -stories expense,receipt` → `receipt OCR story: ok`.

Полный сценарий до `ready` → ручная правка → CreateExpense → ConfirmExpense →
GetBalance выполняется там, где нативный Paddle работает (целевая платформа
**linux/x86_64**, CI). Логику парсинга/подтверждения независимо покрывает
`make document-ocr-check` (6/6 фикстур, реальный cv2-decode + QR + parser) и
Playwright «OCR failed и повтор распознавания» (детерминированный mock).

## 4.2 Expense → Settlement → Export

`go run ./cmd/smoke -stories expense,settlement,adjustment,export` — зелёные:
GetSettlementPlan → CreateSettlement → ConfirmSettlement → GetBalance →
CreateExport (CSV/XLSX/PDF) → Download с проверкой содержимого, кириллицы и сумм.
Рендер экспорта — чистый Document (reportlab/openpyxl/csv), OCR не требуется,
поэтому проходит и на aarch64.

## 4.3 Failure recovery

Recoverable failure не повреждает финансовые данные:
- Останов Document: `/health/ready` → 503 (`document: unavailable`, `ocr: unknown`),
  при этом `expense/settlement/adjustment` на Core проходят полностью;
- Старт Document: readiness → `ok` (`document: ok`), `RestartCount=0`
  (нативный краш OCR не роняет gRPC-сервис — subprocess-изоляция Block 2.3);
- Повторная доставка webhook идемпотентна по `event_key`
  (`gateway_max_updates` PK + счётчик `delim_webhook_events_total{result="duplicate"}`);
- Повтор create/retry чеков после сбоя даёт retry→failed, без дублей расхода
  (проверки версий в Core + `ConflictError` на повторный retry активного job).
