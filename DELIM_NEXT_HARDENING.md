# DELIM_RELEASE_BLOCKERS_FINAL

Цель: закрыть последние блокирующие проблемы Delim перед реальным MAX,
онлайн-этапом и демонстрацией.

Не добавлять новые продуктовые функции.
Не делать архитектурный рефакторинг.
Работать только с фактически незакрытыми release blockers.

После каждого пункта отдельный commit.
После пункта писать только:

`<commit> — done`

---

# БЛОК 1. Зафиксировать текущее состояние

## 1.1. Проверить HEAD

Выполнить:

git status
git log --oneline -35

Убедиться:

- все 27 hardening-коммитов присутствуют;
- рабочее дерево чистое;
- `.env` не tracked;
- pycache не tracked.

Выполнить:

make audit
make generated-check

Commit не нужен.

## 1.2. Push checkpoint

Если все проверки зелёные:

git push origin master

Не force push.

После push проверить:

git status
git log origin/master..HEAD

Ожидание:
пусто.

Commit не нужен.

---

# БЛОК 2. Устранить PaddleOCR crash

## 2.1. Воспроизвести SIGSEGV отдельно

Не запускать весь Delim.

Создать минимальный диагностический script:

image
→ PaddleOCR initialization
→ inference
→ exit

Проверить:

- Python version;
- paddlepaddle version;
- paddleocr version;
- CPU architecture;
- доступные instruction sets;
- OpenMP/MKL runtime.

Script после диагностики оставить только если полезен как
`document-ocr-diagnose`.

Не менять parser/worker.

Commit:

`document: isolate PaddleOCR crash`

## 2.2. Зафиксировать стабильную dependency combination

Подобрать минимально совместимую CPU-конфигурацию.

Приоритет:

1. актуальный совместимый Paddle/PaddleOCR;
2. фиксированные версии;
3. CPU-only;
4. отсутствие SIGSEGV на целевом окружении.

Не обновлять зависимости вслепую.

После выбора зафиксировать версии в pyproject/lock.

Проверить:

- startup;
- 20 последовательных inference;
- rotated fixture;
- low contrast fixture.

Commit:

`document: stabilize PaddleOCR runtime`

## 2.3. Изолировать native OCR crash

Даже после исправления Paddle не позволять native OCR
ронять основной gRPC service.

Вынести inference в отдельный subprocess/process worker.

Document main process:
- gRPC;
- jobs;
- database;
- MinIO

не должен падать при SIGSEGV OCR worker.

При аварии worker:

job
→ retry

после лимита:
→ failed

Document остаётся healthy.

Не строить отдельный микросервис.

Commit:

`document: isolate OCR worker process`

## 2.4. Добавить fallback

Если production Paddle provider не может стартовать:

не переключаться тихо на fake OCR.

Допустимо:

- degraded status;
- OCR job failed с понятным internal code;
- QR parsing продолжает работать;
- Document Ping/read operations продолжают работать.

Readiness должна показывать состояние OCR отдельно.

Commit:

`document: handle degraded OCR runtime`

---

# БЛОК 3. Починить реальный frontend E2E

## 3.1. Разобраться с MAX UI runtime 404

Воспроизвести:

npm ci
npm run dev
Playwright Story A

Найти источник:

`domain-config 404`

Проверить:

- @maxhub/max-ui initialization;
- runtime asset/config requests;
- Vite base URL;
- local fallback.

Не mock-ать весь MAX UI.

Локальный браузер должен работать без внешнего нестабильного runtime.

Commit:

`miniapp: stabilize local MAX UI runtime`

## 3.2. Прогнать полный Playwright

Выполнить все 52+ теста.

Исправить только реальные regressions.

Обязательно зелёные:

- Story A;
- Story B;
- Story C;
- Story D;
- auth;
- 409;
- archived;
- OCR retry;
- export;
- viewport projects.

Commit:

`miniapp: verify browser regression suite`

---

# БЛОК 4. Полный end-to-end release flow

## 4.1. Receipt → OCR → Expense

На реальном stack:

1. Create group
2. Upload receipt
3. queued
4. processing
5. ready
6. GetOCRResult
7. исправить данные как клиент
8. CreateExpense
9. ConfirmExpense
10. GetBalance

Проверить, что OCR сам Expense не создаёт.

Commit:

`release: verify receipt expense flow`

## 4.2. Expense → Settlement → Export

Продолжить тот же сценарий:

11. GetSettlementPlan
12. CreateSettlement
13. ConfirmSettlement
14. GetBalance
15. CreateExport CSV
16. Download
17. CreateExport XLSX
18. Download
19. CreateExport PDF
20. Download

Проверить кириллицу и суммы.

Commit:

`release: verify settlement export flow`

## 4.3. Failure recovery

Проверить:

- убить OCR worker во время inference;
- перезапустить Document;
- остановить MinIO;
- остановить Core;
- повторить webhook event;
- повторить create/retry запрос.

Ни один recoverable failure не должен повреждать финансовые данные.

Commit:

`release: verify failure recovery`

---

# БЛОК 5. Реальный MAX

Выполнять только после получения credentials.

## 5.1. MAX setup

Заполнить реальные:

MAX_BOT_TOKEN
MAX_BOT_USERNAME
MAX_WEBHOOK_SECRET
MAX_WEBHOOK_URL
MAX_MINI_APP_URL

Не коммитить.

Выполнить:

make prod-check
make max-check
make max-setup

Commit не нужен.

## 5.2. Проверить auth

В реальном Mini App:

MAX
→ Mini App
→ initData
→ Gateway
→ Core.UpsertUser
→ session
→ /me

Проверить:

- iOS/Android/Web хотя бы на доступных клиентах;
- raw initData не логируется;
- повторный вход не создаёт нового Core user.

Commit:

`max: verify production authentication`

## 5.3. Проверить invite

Пользователь A:

CreateGroup
→ invite
→ share

Пользователь B:

deep link
→ startapp
→ login
→ JoinGroup

Повторный переход:
idempotent.

Expired token:
не добавляет пользователя.

Commit:

`max: verify production invitation flow`

## 5.4. Проверить Webhook

Реально проверить:

- bot_started;
- bot_added;
- message_created;
- callback;
- duplicate delivery;
- bot_removed.

Gateway должен отвечать быстро,
а повторная доставка не выполнять действие второй раз.

Commit:

`max: verify production webhook flow`

---

# ФИНАЛ

Выполнить:

make release-check
make document-ocr-check

и при credentials:

make max-check

Также:

git status

Ожидание:
чисто.

После этого основной MVP считать release-ready.

Не добавлять новые функции до появления:
- реального задания 15 сентября;
- feedback от тестовых пользователей;
- конкретного бага.

В конце вывести:

`DELIM_RELEASE_BLOCKERS_FINAL done — <N commits>`