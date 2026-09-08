#!/usr/bin/env bash
# Project audit: sequential fast checks, exits non-zero on first failure. Never starts services.
set -euo pipefail
cd "$(dirname "$0")/.."

NPM="${NPM:-npm}"
PYTHON="${PYTHON:-python3}"
COMPOSE_FILE="${COMPOSE_FILE:-deployments/dev/compose.yaml}"

GEN_DIR="$(mktemp -d)"
trap 'rm -rf "$GEN_DIR"' EXIT

check() {
	local label="$1"; shift
	local output
	if output="$("$@" 2>&1)"; then
		printf 'ok   %s\n' "$label"
	else
		printf 'FAIL %s\n%s\n' "$label" "$output" >&2
		exit 1
	fi
}

proto_check() {
	protoc -I . \
		--go_out="$GEN_DIR" --go_opt=module=delim \
		--go-grpc_out="$GEN_DIR" --go-grpc_opt=module=delim \
		proto/core/v1/*.proto proto/document/v1/document.proto
}

go_check() {
	local files
	files="$(gofmt -l .)"
	if [ -n "$files" ]; then
		printf 'unformatted:\n%s\n' "$files" >&2
		return 1
	fi
	go build ./...
}

python_check() {
	PYTHONPYCACHEPREFIX="$GEN_DIR/pycache" "$PYTHON" -m compileall -q document/src document/gen
}

miniapp_check() {
	if [ ! -x web/miniapp/node_modules/.bin/tsc ]; then
		echo "miniapp dependencies missing; run 'make web-install'" >&2
		return 1
	fi
	"$NPM" --prefix web/miniapp run typecheck
	"$NPM" --prefix web/miniapp run build
}

check "proto generation" proto_check
check "go fmt/build" go_check
check "core tests" go test ./internal/core/...
check "python compile" python_check
check "miniapp typecheck/build" miniapp_check
check "compose config" docker compose -f "$COMPOSE_FILE" config -q

echo "audit: all checks passed"
