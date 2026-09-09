# DELIM_MAX_PRODUCT_FINAL

Цель: закончить Delim как MAX-first продукт.

Основные задачи:
1. связать MAX group chat с Delim group;
2. сделать полезные команды бота;
3. добавить уведомления и callbacks;
4. синхронизировать участников;
5. улучшить переходы MAX → Mini App;
6. отделить browser E2E от native PaddleOCR;
7. получить отдельный обязательный x86_64 OCR gate;
8. после появления credentials пройти настоящий MAX flow.

Репозиторий:
https://github.com/BMSTU-LYGO/Delim

---

# ПРАВИЛА CODEX

- Работать только в текущей ветке `master`.
- Новые ветки не создавать.
- После КАЖДОГО пункта отдельный commit.
- Commit называется ровно как указано.
- После пункта писать только:

`<commit> — done`

- Минимизировать расход токенов.
- Не перечитывать весь repository.
- Перед блоком читать только затрагиваемые каталоги.
- Не переделывать Core.
- Не менять финансовую математику.
- Не переносить business logic в Gateway.
- Document не связывать с Core DB.
- Не добавлять Redis/Kafka/RabbitMQ.
- Не добавлять новый UI-kit.
- Не переписывать существующий CI.
- Generated proto руками не менять.
- Реальные MAX secrets не коммитить.
- Online MAX checks выполнять только при настоящих credentials.
- Если credentials отсутствуют — код и offline tests сделать, live verification оставить BLOCKED.

---

# БЛОК 1. MAX chat ↔ Delim group

## 1.1. Добавить binding schema

Создать:

`migrations/gateway/002_chat_group_bindings.sql`

Таблица:

`gateway_max_chat_groups`

Поля:

- chat_id BIGINT PRIMARY KEY;
- group_id BIGINT NOT NULL UNIQUE;
- bound_by_user_id BIGINT NOT NULL;
- status TEXT NOT NULL;
- created_at;
- updated_at.

status:

- active;
- unbound.

Не делать FK в Core schema.

Добавить repository:

- BindChatGroup;
- GetGroupByChat;
- GetChatByGroup;
- UnbindChatGroup.

Один MAX chat ↔ одна Delim group.

Повторный bind той же пары идемпотентен.

Commit:

`gateway: add MAX chat group bindings`

---

## 1.2. Протащить verified chat context

Если проверенный MAX `initData` содержит chat context:

сохранять optional `chat_id` в Gateway session.

Правила:

- chat_id только из server-verified initData;
- не принимать trusted chat_id из frontend body;
- Mini App вне chat должен продолжать работать;
- `/me` либо отдельный context endpoint возвращает только необходимый chat context.

Commit:

`gateway: add verified MAX chat context`

---

## 1.3. Добавить HTTP API binding

Добавить:

POST   /api/v1/groups/{groupID}/max-chat
GET    /api/v1/groups/{groupID}/max-chat
DELETE /api/v1/groups/{groupID}/max-chat

Bind:

1. actor из session;
2. verified chat_id из session;
3. Core проверяет membership/role;
4. разрешить owner/admin;
5. chat должен быть active в Gateway registry;
6. сохранить binding.

Frontend не передаёт chat_id вручную.

Commit:

`gateway: expose MAX chat binding API`

---

# БЛОК 2. Launch tokens

Invite token означает право join.
Не использовать его для `/new`, `/balance` и навигации.

## 2.1. Создать launch token

Создать:

`internal/gateway/launch/`

Token:

- version;
- group_id;
- action;
- entity_id optional;
- expires_at;
- nonce;
- HMAC.

Prefix:

`dl_`

Actions:

- group;
- new_expense;
- balance;
- expense;
- settlement.

Secret:

`GATEWAY_LAUNCH_SECRET`

Он должен отличаться от:

- session secret;
- invite secret.

Добавить production validation.

Commit:

`gateway: add signed Mini App launch tokens`

---

## 2.2. Поддержать launch token в start_param

После MAX initData verification:

invite token:
→ существующий join flow.

launch token:
→ только navigation intent.

Launch token НЕ даёт membership.

Перед group-specific navigation Gateway должен проверить доступ пользователя через Core.

В auth response вернуть optional:

- launch_action;
- group_id;
- entity_id.

Commit:

`gateway: handle Mini App launch actions`

---

# БЛОК 3. Команды бота

Сейчас полезно обрабатываются только `/start` и `/help`.

## 3.1. `/new`

В bound chat:

`/new`

Flow:

chat_id
→ group binding
→ signed launch token `new_expense`
→ open_app button.

Если chat не привязан:

сообщить:

`Привяжите этот чат к группе Делим`

и дать кнопку:

`Открыть Делим`.

Expense из сообщения автоматически НЕ создавать.

Commit:

`max: add new expense command`

---

## 3.2. `/balance`

Flow:

MAX sender
→ Core user
→ bound group
→ Core.GetBalance
→ форматированный ответ.

Тексты:

`Тебе должны 1 240 ₽`

`Ты должен 850 ₽`

`Расчёты закрыты`

Добавить:

`Открыть баланс`

через launch token.

Никакой финансовой математики Gateway.

Commit:

`max: add balance command`

---

## 3.3. Обновить bot setup

`gatewayctl max setup` должен регистрировать:

- /start
- /help
- /new
- /balance

`max check` должен проверять expected command set.

Commit:

`max: register product bot commands`

---

# БЛОК 4. Синхронизация участников

## 4.1. Реализовать sync chat → group

Flow:

bound chat
→ GetAllChatMembers
→ Core.UpsertUser
→ Core.AddGroupMembers

Запуск только owner/admin.

Повторный sync:
idempotent.

Не удалять Core member, если он вышел из MAX-чата:
финансовая история должна сохраниться.

Commit:

`gateway: sync MAX chat members to group`

---

## 4.2. Добавить HTTP endpoint

POST:

`/api/v1/groups/{groupID}/max-chat/sync`

Response:

- discovered;
- added;
- already_present;
- unavailable.

Если MAX members API вернул 403:

возвращать понятную ошибку:

`bot_admin_required`

Не ломать группу.

Commit:

`gateway: expose MAX member synchronization`

---

## 4.3. Обрабатывать user_added

Если:

- chat bound;
- user_added;

то:

UpsertUser
→ AddGroupMembers.

Идемпотентно.

`user_removed`:

не удалять Core member автоматически.

Только log + metric.

Commit:

`max: synchronize added chat members`

---

# БЛОК 5. Durable notifications

## 5.1. Notification outbox

Создать migration:

`003_notifications.sql`

Таблица:

gateway_notifications

Поля:

- id;
- dedupe_key UNIQUE;
- chat_id;
- kind;
- payload JSONB;
- status;
- attempts;
- next_attempt_at;
- last_error;
- created_at;
- sent_at.

status:

- pending;
- processing;
- sent;
- failed.

Использовать:

`FOR UPDATE SKIP LOCKED`.

Commit:

`gateway: add notification outbox`

---

## 5.2. Notification worker

Worker:

pending
→ processing
→ MAX API
→ sent.

Retry:
- transient MAX error;
- 429;
- временная сеть.

Exponential backoff.

После лимита:
failed.

MAX token отсутствует:
Gateway продолжает работать.

Не ломать Core operation из-за notification.

Commit:

`gateway: add MAX notification worker`

---

# БЛОК 6. Финансовые уведомления

## 6.1. Confirmed Expense

После успешного ConfirmExpense:

если group bound:

отправить:

`Алексей добавил расход «Такси» — 1 240 ₽`

Кнопка:

`Посмотреть`

→ launch action `expense`.

Не отправлять allocations целиком.

Commit:

`max: notify confirmed expenses`

---

## 6.2. Settlement

После CreateSettlement:

`Мария отметила погашение 850 ₽. Получателю нужно подтвердить.`

Кнопки:

- Подтвердить;
- Открыть.

Open:
→ Mini App settlement.

Confirm:
→ signed callback.

Commit:

`max: notify settlement confirmation`

---

## 6.3. Adjustment

После refund/correction:

отправить:

- тип;
- сумму;
- исходный расход.

Кнопка:

`Посмотреть расход`.

Commit:

`max: notify expense adjustments`

---

# БЛОК 7. Signed callbacks

## 7.1. Подписать callback payload

Callback payload:

- version;
- action;
- entity_id;
- expires_at;
- nonce;
- signature.

Actions MVP:

`confirm_settlement`

Не доверять unsigned entity id.

Commit:

`max: sign callback actions`

---

## 7.2. Confirm Settlement из MAX

Flow:

callback webhook
→ callback verify
→ MAX sender
→ Core user
→ Core.ConfirmSettlement.

Core остаётся source of truth permissions.

Если callback нажал не receiver:
→ отказ.

Если settlement уже confirmed:
→ idempotent response.

После success:

`Погашение подтверждено`

через AnswerCallback.

Commit:

`max: confirm settlements from callbacks`

---

# БЛОК 8. Mini App UX для MAX-чата

## 8.1. Group settings

Добавить секцию:

`MAX-чат`

Состояния:

- не привязан;
- можно привязать текущий чат;
- привязан;
- bot removed;
- sync unavailable.

Owner/admin:

- Привязать;
- Синхронизировать участников;
- Отвязать.

Member:
read-only.

Commit:

`miniapp: add MAX chat integration UX`

---

## 8.2. Launch navigation

После login обрабатывать:

group
→ GroupDashboard

new_expense
→ ExpenseForm

balance
→ BalancePage

expense
→ ExpenseDetail

settlement
→ Settlement screen.

После первого применения action очистить ephemeral intent.

Refresh не должен повторять mutation.

Commit:

`miniapp: handle MAX launch navigation`

---

## 8.3. Edge states

Показать нормальный UX, если:

- приложение открыто не из чата;
- chat ещё не bound;
- bot удалён из чата;
- пользователь не admin;
- sync требует admin rights бота.

Никаких технических chat_id в UI.

Commit:

`miniapp: handle MAX chat edge states`

---

# БЛОК 9. Offline MAX tests

## 9.1. Dispatcher tests

Через local `httptest.Server` проверить:

- /start;
- /help;
- /new bound;
- /new unbound;
- /balance;
- user_added;
- confirm settlement callback;
- чужой callback;
- duplicate callback.

Не генерировать mocks.

Commit:

`max: test bot product flows`

---

## 9.2. Offline MAX smoke

Расширить `cmd/smoke`.

Story:

`max-offline`

Проверить:

- active chat;
- group binding;
- Get binding;
- sync;
- owner permission;
- member forbidden;
- unbind;
- repeated webhook.

Не требовать настоящего MAX token.

Commit:

`integration: add offline MAX chat smoke`

---

# БЛОК 10. Исправить разделение E2E и OCR

Сейчас browser Story B зависит от native Paddle,
из-за чего arm64 получает 48/52,
хотя frontend/backend flow исправен.

Это надо разделить правильно.

## 10.1. Test-only OCR provider

Document должен иметь deterministic OCR provider ТОЛЬКО для:

`app.env=test`

Никакого автоматического fallback production → fake.

Test provider должен проходить реальную цепочку:

Gateway
→ gRPC
→ CreateReceipt
→ Job
→ worker
→ OCRResult.

Он заменяет только inference.

Commit:

`document: add deterministic E2E OCR provider`

---

## 10.2. Story B сделать architecture-independent

Playwright Story B запускается с test Document provider.

Он проверяет:

upload
→ processing
→ ready
→ OCR review
→ manual edit
→ CreateExpense
→ Confirm
→ Balance.

Paddle совместимость здесь НЕ тестируется.

После этого:

все 52 E2E должны проходить на arm64.

Commit:

`miniapp: decouple E2E from native OCR`

---

# БЛОК 11. Отдельный x86_64 OCR gate

## 11.1. GitHub Actions live OCR job

Добавить CI job:

`ocr-live-x86`

Runner:

`ubuntu-latest`

Проверить:

`uname -m == x86_64`

Запустить реальный:

PaddleOCR
→ fixture
→ receipt
→ ready.

Не использовать test provider.

Минимум:

- `OCR_MODE=live make document-ocr-check`;
- один real receipt job.

Commit:

`ci: add x86 live OCR gate`

---

## 11.2. Разделить release checks

`make release-check`

должен быть architecture-independent:

- audit;
- generated;
- smoke;
- E2E;
- prod-check.

Добавить:

`make release-check-live-ocr`

Он:
- требует x86_64;
- запускает native Paddle;
- на arm64 явно пишет `unsupported architecture`.

Не скрывать отсутствие OCR проверки.

Commit:

`quality: separate native OCR release gate`

---

# БЛОК 12. Метрики MAX product flows

Добавить bounded metrics:

- chat_bind_total{result};
- member_sync_total{result};
- bot_command_total{command,result};
- notification_total{kind,result};
- notifications_pending;
- callback_total{action,result}.

НЕ использовать labels:

- user_id;
- chat_id;
- group_id;
- expense_id.

Добавить небольшую MAX-секцию Grafana.

Commit:

`observability: monitor MAX product flows`

---

# БЛОК 13. OpenAPI / docs

## 13.1. OpenAPI

Добавить frontend endpoints:

- MAX chat bind;
- get binding;
- unbind;
- sync;
- session/launch context.

Bot webhook internals в OpenAPI не описывать.

Commit:

`docs: document MAX chat API`

---

## 13.2. Acceptance matrix

Добавить требования:

- chat binding;
- /new;
- /balance;
- member sync;
- settlement callback;
- notifications;
- arm64 E2E;
- x86 live OCR.

Live MAX:

оставить BLOCKED,
пока реально не проверено.

Commit:

`docs: extend MAX acceptance matrix`

---

## 13.3. Demo flow

Обновить `docs/demo.md`.

Главный demo при наличии MAX:

MAX group chat
→ /new
→ Mini App
→ Expense
→ Bot notification
→ /balance
→ Settlement callback
→ Receipt OCR
→ Export.

Fallback local demo сохранить.

Commit:

`docs: add MAX first demo flow`

---

# БЛОК 14. Реальный MAX

ВЫПОЛНЯТЬ ТОЛЬКО ПРИ НАЛИЧИИ CREDENTIALS.

## 14.1. Chat binding

Реальный групповой чат:

1. добавить бота;
2. открыть Mini App;
3. owner bind;
4. sync members;
5. повторный sync;
6. member пытается unbind → отказ.

Commit только после реальной проверки:

`max: verify live chat binding`

---

## 14.2. Commands

Проверить:

- /start;
- /help;
- /new;
- /balance.

Сценарии:

- bound;
- unbound;
- новый пользователь.

Commit:

`max: verify live bot commands`

---

## 14.3. Notifications/callback

Flow:

Expense confirm
→ MAX notification.

Settlement create
→ MAX notification
→ receiver Confirm
→ Core settlement confirmed
→ balance changed.

Другой пользователь нажимает Confirm:
→ отказ.

Повторный callback:
→ idempotent.

Commit:

`max: verify live financial callbacks`

---

# БЛОК 15. Финальный прогон

## 15.1. Architecture-independent gate

Выполнить:

make audit
make generated-check
make dev-up
make smoke
make release-check

Ожидание:

полностью зелёный и на arm64.

Commit:

`release: verify architecture independent gate`

---

## 15.2. x86 OCR

На CI/x86_64:

make release-check-live-ocr

Ожидание:

- real Paddle;
- real inference;
- receipt ready;
- Document main process жив.

Commit:

`release: verify x86 OCR runtime`

---

## 15.3. MAX gate

При credentials:

make max-check
make max-setup

и live flow блока 14.

Commit делать только после реального прогона:

`release: verify live MAX integration`

---

# DEFINITION OF DONE

После этого:

1. MAX chat можно привязать к Delim group.
2. Один chat соответствует одной group.
3. Frontend не может подделать chat_id.
4. Есть отдельные invite и launch tokens.
5. `/new` открывает нужную форму.
6. `/balance` использует Core balance.
7. `/start /help /new /balance` зарегистрированы.
8. Owner/admin может sync chat members.
9. user_added sync работает.
10. user_removed не удаляет financial history.
11. Есть durable notification outbox.
12. Expense notification работает.
13. Settlement notification работает.
14. Adjustment notification работает.
15. Callback payload подписан.
16. Receiver может подтвердить settlement в MAX.
17. Чужой пользователь не может подтвердить.
18. Повторный callback идемпотентен.
19. Mini App показывает MAX chat integration.
20. Mini App понимает launch actions.
21. Offline MAX flows покрыты smoke/tests.
22. Offline проверки не требуют MAX credentials.
23. Browser E2E не зависит от Paddle CPU.
24. Все E2E проходят на arm64.
25. Production никогда не использует test OCR provider.
26. Реальный Paddle отдельно проверяется на linux/x86_64.
27. `make release-check` зелёный на arm64/x86_64.
28. Native OCR gate отдельно обязателен на x86_64.
29. Новые MAX flows имеют bounded metrics.
30. OpenAPI соответствует реализации.
31. Acceptance matrix различает offline DONE и live MAX BLOCKED.
32. После credentials реальный сценарий:

MAX chat
→ /new
→ Mini App
→ expense
→ notification
→ /balance
→ settlement callback

работает end-to-end.

После завершения вывести только:

`DELIM_MAX_PRODUCT_FINAL done — <N commits>`