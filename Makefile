COMPOSE := docker compose -f deployments/dev/compose.yaml
PYTHON ?= python3
SMOKE_GATEWAY_URL ?= http://localhost:8080
SMOKE_CORE_ADDR ?= localhost:50051

.PHONY: build run up dev-init dev-up down clean logs ps proto document-install document-proto document-run tidy fmt config max-check max-setup core-migrate document-migrate gateway-migrate smoke-expense smoke-settlement smoke-adjustment

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/gatewayctl ./cmd/gatewayctl
	go build -o bin/core ./cmd/core
	PYTHONPYCACHEPREFIX=/tmp/delim-document-pycache $(PYTHON) -m compileall -q -f document/src document/gen

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
	$(COMPOSE) up -d --build

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

max-check:
	go run ./cmd/gatewayctl max check

max-setup:
	go run ./cmd/gatewayctl max setup

smoke-expense:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense

smoke-settlement:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,settlement

smoke-adjustment:
	@test -f .env || { echo "missing .env; copy .env.example to .env and configure it" >&2; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/smoke -gateway-url "$(SMOKE_GATEWAY_URL)" -core-addr "$(SMOKE_CORE_ADDR)" -stories expense,settlement,adjustment
