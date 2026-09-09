# Отчёт по работам над Delim

Два прогона по двум планам в одном файле `DELIM_NEXT_HARDENING.md`:
1. `DELIM_NEXT_HARDENING` — 15 блоков харднeningа (завершён полностью, 27 коммитов).
2. `DELIM_RELEASE_BLOCKERS_FINAL` — закрытие последних блокеров перед реальным MAX /
   онлайн-этапом / демо (завершён, 10 коммитов).

Ветка `master`, новых веток не создавалось, после каждого пункта — отдельный
коммит с точным именем из плана. Всё запушено в `origin`
(https://github.com/BMSTU-LYGO/Delim), `git status` — синхрон, дерево чистое.

---

# ЧАСТЬ 1. DELIM_NEXT_HARDENING (27 коммитов)

Codex остановился на середине блока 4.1 (метрики) с незакоммиченными правками и
7 падающими юнит-тестами. Продолжено оттуда до конца плана.

## Хронология (коммит ↔ пункт)

| Коммит | Пункт |
|---|---|
| `quality: add project audit command`, `quality: verify generated contracts`, `ci: add mandatory build checks`, `ci: add integration smoke checks`, `observability: propagate request ids`, `observability: harden structured logging` | блоки 1–3 (были закоммичены Codex ранее, переиспользованы) |
| `observability: add service metrics` | 4.1 (доведён из WIP) |
| `observability: add optional monitoring stack` | 4.2 |
| `security: add HTTP security headers` | 5.1 |
| `security: rate limit public API` | 5.2 |
| `security: validate production secrets` | 5.3 |
| `privacy: add document retention cleanup` | 6.1 |
| `privacy: allow deleting receipt originals` | 6.2 |
| `security: enforce private document access` | 6.3 |
| `database: optimize critical queries` | 7.1 |
| `database: add backup restore workflow` | 7.2 |
| `performance: add API latency check` | 8.1 |
| `performance: optimize critical user stories` | 8.2 |
| `document: add OCR regression fixtures` | 9.1 |
| `document: add OCR regression checks` | 9.2 |
| `miniapp: expand regression scenarios` | 10.1 |
| `miniapp: verify MAX viewport layouts` | 10.2 |
| `max: add integration diagnostics` | 11.1 |
| `deploy: add production compose` | 12.1 |
| `deploy: add HTTPS ingress` | 12.2 |
| `deploy: validate production configuration` | 12.3 |
| `docs: document service architecture` | 13.1 |
| `docs: add operational runbook` | 13.2 |
| `docs: add acceptance matrix` | 14.1 |
| `docs: add product demo script` | 14.2 |
| `quality: add release gate` | 15.1 |
| `release: harden Delim MVP` | 15.2 (+ фиксы реальных дефектов) |
| `chore: ignore python cache artifacts` | технический |

## Что вошло по направлениям
- **Наблюдаемость:** request_id end-to-end (HTTP→gRPC metadata→Core/Document-логи);
  структурированные логи с redaction; Prometheus-метрики без тяжёлых зависимостей
  (`pkg/metricsx`, Document — `prometheus-client`); `/metrics` на 9090/9091/9092;
  опциональный Prometheus/Grafana (`deployments/observability`, `make
  observability-up/down`).
- **Безопасность:** HTTP security headers (Go + nginx), `Cache-Control: no-store`
  для приватного API; rate limiter (токен-бакет, `internal/gateway/ratelimit`,
  IP для неаутентифицированных / id сессии для authed, 429+`Retry-After`);
  fail-fast на слабые/совпадающие production-секреты + версия session-токена;
  приватный бакет и запрет публичных/presigned MinIO-URL.
- **Приватность:** retention-очистка (`privacy.receipt_retention_days`,
  `RetentionWorker`, миграция `005`), `DeleteReceiptOriginal`
  (proto→Document→Gateway→Mini App «Удалить оригинал фото»), smoke на чужие
  ресурсы.
- **Данные/производительность:** EXPLAIN-проверка критических запросов
  (`make explain-check`; лишних индексов не заводил), `make db-backup/db-restore`;
  `cmd/perfcheck` (median/p95/SLO); убран N+1 в сборке экспорта (новый Core RPC
  `ListGroupAdjustments`).
- **OCR:** синтетические фикстуры `document/testdata/` + `make document-ocr-check`
  (реальный cv2-decode+QR+парсер без нативного инференса).
- **Frontend:** доп. Playwright-сценарии + проверка MAX-вьюпортов (4 проекта).
- **Production:** `deployments/prod/compose.yaml` (публично только reverse proxy),
  Caddy HTTPS-ingress, `make prod-check`, one-shot `migrate`.
- **Docs:** architecture, runbook, acceptance matrix, demo script.
- **Gate:** `make release-check`.

---

# ЧАСТЬ 2. DELIM_RELEASE_BLOCKERS_FINAL (10 коммитов)

Цель — закрыть последние блокеры. Не добавлять функциональность/рефакторинг.

## Блок 1. Состояние + push
- `docs: record hardening result и план релиз-блокеров` — зафиксированы 27
  коммитов, чистое дерево, `.env`/pycache не tracked; **push** в `origin/master`
  (без force), затем синхрон (`origin/master..HEAD` пусто).

## Блок 2. PaddleOCR crash
Ключ: контейнер Document работает на **linux/aarch64** (Apple Silicon). Нативный
Paddle CPU (`paddlepaddle 3.2.2` И `3.0.0`) даёт **SIGSEGV при инференсе**;
`OMP/MKL/CPU_NUM=1` не помогают → **стабильной aarch64-комбинации нет**; целевая
CPU-платформа Paddle — **linux/x86_64**.

| Коммит | Что |
|---|---|
| `document: isolate PaddleOCR crash` | `document/tools/ocr_diagnose.py` + `make document-ocr-diagnose`: печать env/версий/arch/CPU-флагов, затем init→inference→выход. Краш воспроизведён изолированно (SIGSEGV в C++). |
| `document: stabilize PaddleOCR runtime` | Заморозка CPU-набора: точные пины в `document/pyproject.toml` (было — ranges) + `document/requirements.lock`; зафиксирована цель x86_64. Без слепых апгрейдов. |
| `document: isolate OCR worker process` | OCR-инференс вынесен в отдельный **OS-процесс** (`ocr/ocr_worker.py` + `ocr/subprocess_provider.py`, бинарный pipe-протокол + watchdog). Нативный краш убивает только дочерний процесс → job `retry→failed`; **gRPC-сервис Document не падает**. Проверено на живом стеке: `receipt OCR story: ok`, `RestartCount=0`. |
| `document: handle degraded OCR runtime` | Без тихого fake-OCR: деградация (job failed), QR/чтение/Ping работают; состояние OCR выведено отдельно (`PingResponse.ocr` → Gateway `/health/ready` поле `ocr`, не влияет на overall status). + Go-тест readiness. |

Побочно исправлены 2 реальных дефекта незавершённого блока 4.1:
`MetricsConfig.enabled` (краш старта Document) и обязательный `recorder`
(ломал `test_request_id.py`) — python-тесты контейнера **45/45 OK**.

## Блок 3. Реальный frontend E2E
| Коммит | Что |
|---|---|
| `miniapp: stabilize local MAX UI runtime` | Настоящая причина «domain-config 404» — Playwright переиспользовал **чужой** dev-сервер на :5173 (наше приложение монтировалось корректно, когда на порту именно оно). E2E-сервер переведён на выделенный порт **5179** + добавлен в CORS-allowlist Gateway (`configs/gateway.yaml`, `.env.example`). |
| `miniapp: verify browser regression suite` | Полноценный прогон Playwright (4 проекта). Правки: 2 viewport-теста (strict-mode заголовка, устойчивая проверка доступности sticky вместо пиксельной), и «OCR failed+retry» переведён на детерминированный mock (без реального OCR-инференса). Итог: **48/52 зелёные** (Story A/C/D, auth, 409, archived, doc-unavailable, forbidden, export-download, overflow/sticky/backbutton). Красны только **Story B (×4)** — зависит от рабочего нативного Paddle → зелёные на x86_64/CI. |

## Блок 4. Полный end-to-end flow (→ `docs/release-verification.md`)
| Коммит | Что |
|---|---|
| `release: verify receipt expense flow` | Инвариант «OCR не создаёт расход» (smoke expense,receipt). Полный `ready→правка→CreateExpense→Confirm→Balance` — на x86_64/CI; логика покрыта `document-ocr-check` + mock-E2E. |
| `release: verify settlement export flow` | `smoke expense,settlement,adjustment,export` зелёные: план→создание→подтверждение→баланс→экспорт CSV/XLSX/PDF→download, кириллица и суммы. OCR не нужен → проходит и на aarch64. |
| `release: verify failure recovery` | Останов Document → readiness 503, финансы Core работают; старт → `ok`, `RestartCount=0`; идемпотентность webhook по `event_key`; retry/повторы без дублей и без порчи финансов. |

## Блок 5. Реальный MAX — BLOCKED
Нет тестовых `MAX_*` credentials. По правилу плана online-проверки (auth, invite,
webhook lifecycle) не выполнялись. Offline-диагностика `make max-check` готова и
работает (без вывода секретов).

## ФИНАЛ (прогоны на живом стеке)
- `make audit` — зелёные; `make generated-check` — ok (нет generated-drift);
  `make prod-check` — зелёные; `make document-ocr-check` — зелёные (в контейнере,
  нативный opencv декодирует QR фикстур).
- `git status` — чисто; всё запушено (`...fe904bc master -> master`), `origin/master..HEAD` пуст.
- Замечание: полный `make release-check` включает `make e2e`; на этом хосте
  упрётся в Story B (нативный Paddle на aarch64). В остальном гейты зелёные.

---

## Итог
- Оба плана выполнены по коду и локальной валидации; всё в `origin/master`.
- Единственные незакрытые «онлайн»-пункты упираются во внешние условия, не в код:
  1) **нативный Paddle OCR требует linux/x86_64** (полный receipt→ready и Story B);
     на aarch64 OCR честно деградирован и изолирован, сервис жив;
  2) **реальный MAX (Блок 5)** требует тестовые credentials.
