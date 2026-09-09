COMPOSE := docker compose -f deployments/dev/compose.yaml
COMPOSE_OBS := docker compose -f deployments/dev/compose.yaml -f deployments/observability/compose.yaml
NPM ?= npm
PYTHON ?= python3
SMOKE_GATEWAY_URL ?= http://localhost:8080
SMOKE_CORE_ADDR ?= localhost:50051
WEB_DIR := web/miniapp

.PHONY: build run up dev-init dev-up demo-seed down clean logs ps proto document-install document-proto document-run tidy fmt config max-check max-setup core-migrate document-migrate gateway-migrate smoke smoke-expense smoke-settlement smoke-adjustment smoke-receipt smoke-export web-install web-dev web-build web-typecheck audit generated-check explain-check db-backup db-restore perfcheck document-ocr-check prod-check e2e release-check observability-up observability-down

build: web-build
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/gatewayctl ./cmd/gatewayctl
	go build -o bin/core ./cmd/core
	PYTHONPYCACHEPREFIX=/tmp/delim-document-pycache $(PYTHON) -m compileall -q -f document/src document/gen

web-install:
	$(NPM) --prefix $(WEB_DIR) ci

web-dev:
	$(NPM) --prefix $(WEB_DIR) run dev

web-build:
	$(NPM) --prefix $(WEB_DIR) run build

web-typecheck:
	$(NPM) --prefix $(WEB_DIR) run typecheck

run:
	$(COMPOSE) up --build

up:
	$(COMPOSE) up -d

dev-init:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	$(COMPOSE) up -d --wait postgres minio
	$(COMPOSE) up minio-init
	$(MAKE) core-migrate
	$(MAKE) document-migrate
	$(MAKE) gateway-migrate

dev-up: dev-init
	$(COMPOSE) up -d --build --wait

demo-seed:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	go run ./cmd/devseed

down:
	$(COMPOSE) down

clean:
	$(COMPOSE) down -v --remove-orphans

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

proto:
	protoc -I . \
		--go_out=. --go_opt=module=delim \
		--go-grpc_out=. --go-grpc_opt=module=delim \
		proto/core/v1/*.proto proto/document/v1/document.proto

document-proto:
	cd document && $(PYTHON) -m grpc_tools.protoc -I .. \
		--python_out=gen --grpc_python_out=gen \
		../proto/document/v1/document.proto

document-install:
	$(PYTHON) -m pip install -e document

document-run:
	PYTHONPATH=document/src:document/gen $(PYTHON) -m delim_document.main configs/document.yaml

document-ocr-check:
	@set -e; \
	args="$(if $(filter live,$(OCR_MODE)),--live,)"; \
	PYTHONPATH=document/src:document/gen $(PYTHON) document/tools/ocr_check.py $$args

core-migrate:
	@$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db" -c "CREATE TABLE IF NOT EXISTS core_schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())"'
	@set -e; for migration in migrations/core/*.sql; do \
		version=$$(basename "$$migration"); \
		applied=$$($(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -U "$$POSTGRES_USER" -d "$$db" -Atc "SELECT EXISTS (SELECT 1 FROM core_schema_migrations WHERE version = '\''$$1'\'')"' sh "$$version"); \
		if [ "$$applied" != "t" ]; then \
			{ printf 'BEGIN;\n'; sed '$$a\' "$$migration"; printf "INSERT INTO core_schema_migrations(version) VALUES ('%s');\nCOMMIT;\n" "$$version"; } | \
				$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db"'; \
		fi; \
	done

document-migrate:
	@$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db" -c "CREATE TABLE IF NOT EXISTS document_schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())"'
	@set -e; for migration in migrations/document/*.sql; do \
		version=$$(basename "$$migration"); \
		applied=$$($(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -U "$$POSTGRES_USER" -d "$$db" -Atc "SELECT EXISTS (SELECT 1 FROM document_schema_migrations WHERE version = '\''$$1'\'')"' sh "$$version"); \
		if [ "$$applied" != "t" ]; then \
			{ printf 'BEGIN;\n'; sed '$$a\' "$$migration"; printf "INSERT INTO document_schema_migrations(version) VALUES ('%s');\nCOMMIT;\n" "$$version"; } | \
				$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db"'; \
		fi; \
	done

gateway-migrate:
	@$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db" -c "CREATE TABLE IF NOT EXISTS gateway_schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())"'
	@set -e; for migration in migrations/gateway/*.sql; do \
		version=$$(basename "$$migration"); \
		applied=$$($(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -U "$$POSTGRES_USER" -d "$$db" -Atc "SELECT EXISTS (SELECT 1 FROM gateway_schema_migrations WHERE version = '\''$$1'\'')"' sh "$$version"); \
		if [ "$$applied" != "t" ]; then \
			{ printf 'BEGIN;\n'; sed '$$a\' "$$migration"; printf "INSERT INTO gateway_schema_migrations(version) VALUES ('%s');\nCOMMIT;\n" "$$version"; } | \
				$(COMPOSE) exec -T postgres sh -ec 'db="$${POSTGRES_DB:-$$POSTGRES_USER}"; psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$db"'; \
		fi; \
	done

tidy:
	go mod tidy

fmt:
	go fmt ./...

config:
	$(COMPOSE) config

audit:
	@test -x scripts/audit.sh || chmod +x scripts/audit.sh
	@./scripts/audit.sh

generated-check:
	@test -x scripts/generated-check.sh || chmod +x scripts/generated-check.sh
	@./scripts/generated-check.sh

explain-check:
	@test -x scripts/explain-check.sh || chmod +x scripts/explain-check.sh
	@./scripts/explain-check.sh

db-backup:
	@test -x scripts/db-backup.sh || chmod +x scripts/db-backup.sh
	@./scripts/db-backup.sh

db-restore:
	@test -x scripts/db-restore.sh || chmod +x scripts/db-restore.sh
	@FILE="$(FILE)" ./scripts/db-restore.sh

perfcheck:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; \
		go run ./cmd/perfcheck -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)"

observability-up:
	@$(COMPOSE_OBS) up -d prometheus grafana
	@echo "prometheus: http://127.0.0.1:9099  grafana: http://127.0.0.1:3000 (admin/admin)"

observability-down:
	@$(COMPOSE_OBS) stop prometheus grafana
	@$(COMPOSE_OBS) rm -f prometheus grafana

max-check:
	go run ./cmd/gatewayctl max check

max-setup:
	go run ./cmd/gatewayctl max setup

smoke:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -eu; set -a; . ./.env; set +a; \
		go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories health,expense,settlement,adjustment,receipt,export; \
		trap '$(COMPOSE) start document >/dev/null' EXIT INT TERM; \
		$(COMPOSE) stop document >/dev/null; \
		go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,document-unavailable; \
		$(COMPOSE) start document >/dev/null; \
		trap - EXIT INT TERM; \
		$(COMPOSE) up -d --wait document gateway >/dev/null; \
		if [ -n "$${MAX_BOT_TOKEN:-}" ]; then \
			$(MAKE) max-check; \
		else \
			echo "MAX online smoke: skipped (MAX_BOT_TOKEN is not configured)"; \
		fi

smoke-expense:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense

smoke-settlement:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,settlement

smoke-adjustment:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,settlement,adjustment

smoke-receipt:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -eu; set -a; . ./.env; set +a; \
		go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,receipt; \
		trap '$(COMPOSE) start document >/dev/null' EXIT INT TERM; \
		$(COMPOSE) stop document >/dev/null; \
		go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,document-unavailable; \
		$(COMPOSE) start document >/dev/null; \
		trap - EXIT INT TERM

smoke-export:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,settlement,adjustment,export

prod-check:
	@test -x scripts/prod-check.sh || chmod +x scripts/prod-check.sh
	@./scripts/prod-check.sh

e2e:
	@$(NPM) --prefix $(WEB_DIR) run e2e

# Full release gate. Integration smoke + frontend e2e require the dev stack
# running (`make dev-up`). OCR full regression and MAX online checks are
# separate targets (document-ocr-check, max-check) because of weight/credentials.
release-check:
	@$(MAKE) audit
	@$(MAKE) generated-check
	@$(MAKE) smoke
	@$(MAKE) web-typecheck
	@$(MAKE) web-build
	@$(MAKE) e2e
	@$(MAKE) prod-check
	@echo "release-check: all mandatory gates passed (run document-ocr-check / max-check separately as needed)"
