#!/usr/bin/env bash
# Shared helpers for fork-internal e2e (not for upstream PRs).
set -euo pipefail

INTERNAL_E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$INTERNAL_E2E_DIR/../.." && pwd)"

PASS=0
FAIL=0
SKIP=0

log()  { printf '%s\n' "$*"; }
ok()   { PASS=$((PASS + 1)); printf '  PASS  %s\n' "$*"; }
fail() { FAIL=$((FAIL + 1)); printf '  FAIL  %s\n' "$*"; }
skip() { SKIP=$((SKIP + 1)); printf '  SKIP  %s\n' "$*"; }

load_env() {
  # Preserve caller exports over file values.
  local _pre_token="${INSTANCE_TOKEN:-}"
  local _pre_id="${INSTANCE_ID:-}"
  local _pre_name="${INSTANCE_NAME:-}"
  local _pre_gid="${GROUP_JID:-}"
  local _pre_skip="${SKIP_LIVE:-}"
  local _pre_unit="${RUN_UNIT:-}"
  local _pre_send="${RUN_SEND_GROUP:-}"
  local _pre_base="${BASE_URL:-}"

  if [[ -f "$INTERNAL_E2E_DIR/.env" ]]; then
    # shellcheck disable=SC1091
    set -a
    source "$INTERNAL_E2E_DIR/.env"
    set +a
  fi
  if [[ -f "$REPO_ROOT/.env" ]]; then
    if [[ -z "${POSTGRES_PASSWORD:-}" ]]; then
      POSTGRES_PASSWORD="$(grep -E '^POSTGRES_PASSWORD=' "$REPO_ROOT/.env" | cut -d= -f2- || true)"
    fi
    if [[ -z "${GLOBAL_API_KEY:-}" ]]; then
      GLOBAL_API_KEY="$(grep -E '^GLOBAL_API_KEY=' "$REPO_ROOT/.env" | cut -d= -f2- | tr -d ' ' || true)"
    fi
  fi

  [[ -n "$_pre_token" ]] && INSTANCE_TOKEN="$_pre_token"
  [[ -n "$_pre_id" ]] && INSTANCE_ID="$_pre_id"
  [[ -n "$_pre_name" ]] && INSTANCE_NAME="$_pre_name"
  [[ -n "$_pre_gid" ]] && GROUP_JID="$_pre_gid"
  [[ -n "$_pre_skip" ]] && SKIP_LIVE="$_pre_skip"
  [[ -n "$_pre_unit" ]] && RUN_UNIT="$_pre_unit"
  [[ -n "$_pre_send" ]] && RUN_SEND_GROUP="$_pre_send"
  [[ -n "$_pre_base" ]] && BASE_URL="$_pre_base"

  BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
  INSTANCE_NAME="${INSTANCE_NAME:-cesar-teste}"
  GROUP_JID="${GROUP_JID:-556699050312-1524496157@g.us}"
  POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-evolution-go-postgres}"
  POSTGRES_USER="${POSTGRES_USER:-evogo}"
  POSTGRES_DB="${POSTGRES_DB:-evogo_users}"
  RUN_UNIT="${RUN_UNIT:-1}"
  RUN_SEND_GROUP="${RUN_SEND_GROUP:-1}"
  SKIP_LIVE="${SKIP_LIVE:-0}"
  export PATH="/usr/local/go/bin:${PATH}"
}

resolve_instance() {
  if [[ -n "${INSTANCE_TOKEN:-}" && -n "${INSTANCE_ID:-}" ]]; then
    return 0
  fi
  if [[ -z "${POSTGRES_PASSWORD:-}" ]]; then
    fail "set INSTANCE_TOKEN+INSTANCE_ID or POSTGRES_PASSWORD to resolve by name"
    return 1
  fi
  if ! command -v docker >/dev/null 2>&1; then
    fail "docker not available; export INSTANCE_TOKEN and INSTANCE_ID"
    return 1
  fi
  local row
  if ! row="$(docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" "$POSTGRES_CONTAINER" \
    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \
    "SELECT id || '|' || token FROM instances WHERE name = '${INSTANCE_NAME}' LIMIT 1;" 2>/dev/null)"; then
    fail "docker exec failed; export INSTANCE_TOKEN and INSTANCE_ID instead"
    return 1
  fi
  if [[ -z "$row" || "$row" == "|" ]]; then
    fail "instance not found: $INSTANCE_NAME"
    return 1
  fi
  INSTANCE_ID="${INSTANCE_ID:-${row%%|*}}"
  INSTANCE_TOKEN="${INSTANCE_TOKEN:-${row#*|}}"
  export INSTANCE_ID INSTANCE_TOKEN
}

api() {
  # api METHOD PATH [curl args...]
  local method="$1" path="$2"
  shift 2
  curl -sS -w '\n%{http_code}' -X "$method" \
    -H "apikey: ${INSTANCE_TOKEN}" \
    -H 'Content-Type: application/json' \
    "$@" \
    "${BASE_URL}${path}"
}

split_body_code() {
  # reads curl -w '\n%{http_code}' output into BODY and CODE
  local raw="$1"
  CODE="${raw##*$'\n'}"
  BODY="${raw%$'\n'*}"
}

json_get() {
  # json_get BODY expr  (python one-liner)
  local body="$1" expr="$2"
  BODY="$body" EXPR="$expr" python3 - <<'PY'
import json, os, sys
body = os.environ["BODY"]
expr = os.environ["EXPR"]
try:
    d = json.loads(body)
except Exception as e:
    print("", end="")
    sys.exit(0)
cur = d
for part in expr.split("."):
    if cur is None:
        break
    if isinstance(cur, dict):
        cur = cur.get(part)
    else:
        cur = None
        break
if cur is None:
    print("", end="")
elif isinstance(cur, bool):
    print("true" if cur else "false", end="")
else:
    print(cur, end="")
PY
}

summary() {
  log ""
  log "=== summary: PASS=$PASS FAIL=$FAIL SKIP=$SKIP ==="
  if [[ "$FAIL" -gt 0 ]]; then
    return 1
  fi
  return 0
}
