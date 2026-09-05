COMPOSE := docker compose -f deployments/dev/compose.yaml

.PHONY: build run up down clean logs ps proto document-proto tidy fmt config max-check max-setup core-migrate document-migrate

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/gatewayctl ./cmd/gatewayctl
	go build -o bin/core ./cmd/core
	go build -o bin/document ./cmd/document

run:
	$(COMPOSE) up --build

up:
	$(COMPOSE) up -d

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
	cd document && python -m grpc_tools.protoc -I .. \
		--python_out=gen --grpc_python_out=gen \
		../proto/document/v1/document.proto

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
