#!/usr/bin/env bash
# Validate the production deployment configuration without printing secrets.
# Exit non-zero on the first real problem. Usage: make prod-check
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE="deployments/prod/.env"
COMPOSE="docker compose -f deployments/prod/compose.yaml"

fail() { echo "prod-check: FAIL: $*" >&2; exit 1; }
pass() { echo "prod-check: ok: $*"; }

[ -f "$ENV_FILE" ] || fail "$ENV_FILE is missing (copy .env.example and fill it in)"

get() {
  # read a single KEY=value from the env file (last wins), no export/eval
  local key="$1"
  awk -F= -v k="$key" '$1==k {sub(/^[^=]*=/,""); v=$0} END{print v}' "$ENV_FILE"
}

require() {
  local key="$1" value
  value="$(get "$key")"
  [ -n "$value" ] || fail "required variable $key is empty"
}

looks_placeholder() {
  local value="$1"
  echo "$value" | grep -qiE 'change_me|changeme|replace|example|your[_-]|^dev-|password123|^xxx+$'
}

strong_secret() {
  local key="$1" value
  value="$(get "$key")"
  [ "${#value}" -ge 32 ] || fail "$key must be at least 32 characters"
  ! looks_placeholder "$value" || fail "$key uses a placeholder value"
}

no_localhost() {
  local label="$1" value
  value="$(get "$1")"
  case "$value" in
    *://localhost*|*://127.0.0.1*|*://0.0.0.0*) fail "$label must not point at localhost" ;;
  esac
  case "$value" in
    *localhost*|*127.0.0.1*) fail "$label must not reference localhost" ;;
  esac
}

https_url() {
  local label="$1" value
  value="$(get "$label")"
  case "$value" in
    https://*) : ;;
    *) fail "$label must be an https:// URL (got a non-HTTPS value)" ;;
  esac
  case "$value" in
    *:80/*|*:8080*|*:8081*|*://*:8*) fail "$label must not use an insecure port" ;;
  esac
}

# --- presence -------------------------------------------------------------
for key in POSTGRES_USER POSTGRES_PASSWORD MINIO_ROOT_USER MINIO_ROOT_PASSWORD \
           MAX_BOT_TOKEN MAX_BOT_USERNAME MAX_WEBHOOK_SECRET MAX_WEBHOOK_URL \
           MAX_MINI_APP_URL GATEWAY_SESSION_SECRET GATEWAY_INVITE_SECRET DELIM_PUBLIC_HOST; do
  require "$key"
done
pass "all required variables present"

# --- strong, distinct signing secrets ------------------------------------
for key in GATEWAY_SESSION_SECRET GATEWAY_INVITE_SECRET MAX_WEBHOOK_SECRET; do
  strong_secret "$key"
done
if [ "$(get GATEWAY_SESSION_SECRET)" = "$(get GATEWAY_INVITE_SECRET)" ]; then
  fail "GATEWAY_SESSION_SECRET and GATEWAY_INVITE_SECRET must differ"
fi
if [ "$(get GATEWAY_SESSION_SECRET)" = "$(get MAX_WEBHOOK_SECRET)" ] ||
   [ "$(get GATEWAY_INVITE_SECRET)" = "$(get MAX_WEBHOOK_SECRET)" ]; then
  fail "signing secrets must be mutually distinct"
fi
pass "signing secrets are strong and distinct"

# --- storage/db credentials are non-default ------------------------------
if [ "$(get MINIO_ROOT_PASSWORD)" = "change_me" ] || [ "$(get MINIO_ROOT_PASSWORD)" = "minioadmin" ]; then
  fail "MINIO_ROOT_PASSWORD is a default value"
fi
if [ "$(get POSTGRES_PASSWORD)" = "delim" ] || [ -z "$(get POSTGRES_PASSWORD)" ]; then
  fail "POSTGRES_PASSWORD is a default/empty value"
fi
pass "storage/database credentials are non-default"

# --- public host & HTTPS URLs --------------------------------------------
host="$(get DELIM_PUBLIC_HOST)"
case "$host" in
  localhost|127.0.0.1|0.0.0.0|"") fail "DELIM_PUBLIC_HOST must be a real public host name" ;;
esac
case "$host" in
  *://*) fail "DELIM_PUBLIC_HOST must not include a scheme" ;;
esac
for label in MAX_WEBHOOK_URL MAX_MINI_APP_URL; do
  https_url "$label"
  no_localhost "$label"
  case "$(get "$label")" in
    *"$host"*) : ;;
    *) fail "$label must reference DELIM_PUBLIC_HOST ($host)" ;;
  esac
done
pass "MAX webhook/mini-app URLs are HTTPS and match DELIM_PUBLIC_HOST"

# --- CORS: no wildcard, HTTPS only ---------------------------------------
cors="$(get GATEWAY_CORS_ALLOWED_ORIGINS)"
case "$cors" in
  *"*"*) fail "GATEWAY_CORS_ALLOWED_ORIGINS must not contain a wildcard" ;;
esac
if [ -n "$cors" ]; then
  IFS=',' read -ra origins <<< "$cors"
  for origin in "${origins[@]}"; do
    [ -n "$origin" ] || continue
    case "$origin" in
      https://*) : ;;
      *) fail "CORS origin '$origin' must be HTTPS" ;;
    esac
    case "$origin" in
      *localhost*|*127.0.0.1*) fail "CORS origin '$origin' must not be localhost" ;;
    esac
  done
fi
pass "CORS is same-origin/HTTPS with no wildcard"

# --- compose config validity ---------------------------------------------
$COMPOSE config -q
pass "production compose config is valid"

echo "prod-check: all production configuration checks passed"
