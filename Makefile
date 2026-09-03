COMPOSE := docker compose -f deployments/dev/compose.yaml

.PHONY: build run up down clean logs ps proto tidy fmt config max-check max-setup

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
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
		proto/core/v1/core.proto proto/document/v1/document.proto

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
