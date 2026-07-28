#!/usr/bin/env bash
# Fork-internal e2e runner — do NOT include in upstream PRs.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$DIR/lib.sh"

load_env

log "=== internal e2e (fork) ==="
log "BASE_URL=$BASE_URL INSTANCE_NAME=$INSTANCE_NAME"
log "GROUP_JID=$GROUP_JID SKIP_LIVE=$SKIP_LIVE RUN_UNIT=$RUN_UNIT RUN_SEND_GROUP=$RUN_SEND_GROUP"
log ""

# ---------------------------------------------------------------------------
# Unit / race (no live WhatsApp)
# ---------------------------------------------------------------------------
if [[ "$RUN_UNIT" == "1" ]]; then
  log "--- unit: #99 websocket race + #111 connect/advanced helpers ---"
  if (cd "$REPO_ROOT" && go test -race -count=1 \
      ./pkg/events/websocket/... \
      ./pkg/instance/service/... \
      ./pkg/instance/repository/...); then
    ok "go test -race (websocket + instance)"
  else
    fail "go test -race (websocket + instance)"
  fi
else
  skip "unit tests (RUN_UNIT=0)"
fi

if [[ "$SKIP_LIVE" == "1" ]]; then
  skip "live API tests (SKIP_LIVE=1)"
  summary
  exit $?
fi

resolve_instance || { summary; exit 1; }
ok "resolved instance id=${INSTANCE_ID:0:8}… token_len=${#INSTANCE_TOKEN}"

# ---------------------------------------------------------------------------
# Live: instance status
# ---------------------------------------------------------------------------
log ""
log "--- live: instance status ---"
raw="$(api GET /instance/status)"
split_body_code "$raw"
connected="$(json_get "$BODY" "data.Connected")"
logged_in="$(json_get "$BODY" "data.LoggedIn")"
if [[ "$CODE" == "200" && "$connected" == "true" && "$logged_in" == "true" ]]; then
  ok "instance LoggedIn (HTTP $CODE)"
else
  fail "instance not ready HTTP=$CODE Connected=$connected LoggedIn=$logged_in body=$BODY"
fi

# ---------------------------------------------------------------------------
# #97 group: list / info / participant middleware
# ---------------------------------------------------------------------------
log ""
log "--- live: #97 group list/info/participant middleware ---"

raw="$(api GET /group/list)"
split_body_code "$raw"
if [[ "$CODE" == "200" ]]; then
  ok "GET /group/list HTTP 200"
else
  fail "GET /group/list HTTP $CODE body=$BODY"
fi

raw="$(api POST /group/info -d "{\"groupJid\":\"${GROUP_JID}\"}")"
split_body_code "$raw"
gname="$(json_get "$BODY" "data.Name")"
if [[ "$CODE" == "200" && -n "$gname" ]]; then
  ok "POST /group/info → Name=$gname"
else
  fail "POST /group/info HTTP $CODE body=$BODY"
fi

# Middleware regression: array participants must NOT yield old 400 text.
raw="$(api POST /group/participant -d "{\"groupJid\":\"${GROUP_JID}\",\"action\":\"add\",\"participants\":[\"5511999999999\"]}")"
split_body_code "$raw"
if [[ "$CODE" == "400" ]] && echo "$BODY" | grep -qi 'participants is required'; then
  fail "#97 middleware still rejects array (HTTP 400 participants required): $BODY"
elif [[ "$CODE" == "400" ]] && echo "$BODY" | grep -qi 'Invalid participants'; then
  fail "#97 middleware type error on array: $BODY"
else
  # 200 = added; 403/500 = WhatsApp/session — all prove middleware + handler ran
  ok "#97 participant array reached handler/WA (HTTP $CODE) body=$(echo "$BODY" | head -c 160)"
fi

# Handler still validates required fields
raw="$(api POST /group/participant -d "{\"groupJid\":\"${GROUP_JID}\",\"participants\":[\"5511999999999\"]}")"
split_body_code "$raw"
if [[ "$CODE" == "400" ]] && echo "$BODY" | grep -qi 'action is required'; then
  ok "handler validates missing action (HTTP 400)"
else
  fail "expected handler 400 action is required, got HTTP $CODE body=$BODY"
fi

# ---------------------------------------------------------------------------
# Optional: send text to group (integration smoke)
# ---------------------------------------------------------------------------
if [[ "$RUN_SEND_GROUP" == "1" && -n "$GROUP_JID" ]]; then
  log ""
  log "--- live: send text to group ---"
  msg="[internal-e2e] group smoke $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  raw="$(api POST /send/text -d "{\"number\":\"${GROUP_JID}\",\"text\":\"${msg}\"}")"
  split_body_code "$raw"
  if [[ "$CODE" == "200" ]]; then
    ok "POST /send/text to group HTTP 200"
  else
    fail "POST /send/text HTTP $CODE body=$BODY"
  fi
else
  skip "send group text (RUN_SEND_GROUP=0 or empty GROUP_JID)"
fi

# ---------------------------------------------------------------------------
# #111 advanced-settings partial PUT (restore after)
# ---------------------------------------------------------------------------
log ""
log "--- live: #111 advanced-settings partial update ---"

raw="$(api GET "/instance/${INSTANCE_ID}/advanced-settings")"
split_body_code "$raw"
if [[ "$CODE" != "200" ]]; then
  fail "GET advanced-settings HTTP $CODE body=$BODY"
else
  ok "GET advanced-settings HTTP 200"
  # GET returns AdvancedSettings at root; PUT wraps under "settings".
  before_ig="$(json_get "$BODY" "ignoreGroups")"
  before_is="$(json_get "$BODY" "ignoreStatus")"
  before_ao="$(json_get "$BODY" "alwaysOnline")"
  before_rc="$(json_get "$BODY" "rejectCall")"
  before_rm="$(json_get "$BODY" "readMessages")"

  # Flip only alwaysOnline; others must stay
  if [[ "$before_ao" == "true" ]]; then new_ao=false; else new_ao=true; fi

  raw="$(api PUT "/instance/${INSTANCE_ID}/advanced-settings" -d "{\"alwaysOnline\":${new_ao}}")"
  split_body_code "$raw"
  after_ao="$(json_get "$BODY" "settings.alwaysOnline")"
  after_ig="$(json_get "$BODY" "settings.ignoreGroups")"
  after_is="$(json_get "$BODY" "settings.ignoreStatus")"
  after_rc="$(json_get "$BODY" "settings.rejectCall")"
  after_rm="$(json_get "$BODY" "settings.readMessages")"

  if [[ "$CODE" == "200" && "$after_ao" == "$new_ao" \
      && "$after_ig" == "$before_ig" && "$after_is" == "$before_is" \
      && "$after_rc" == "$before_rc" && "$after_rm" == "$before_rm" ]]; then
    ok "#111 partial PUT kept other fields (alwaysOnline $before_ao→$after_ao)"
  else
    fail "#111 partial PUT wiped fields HTTP=$CODE ao=$after_ao ig=$after_ig/$before_ig is=$after_is/$before_is body=$BODY"
  fi

  # Restore
  raw="$(api PUT "/instance/${INSTANCE_ID}/advanced-settings" -d "{\"alwaysOnline\":${before_ao:-false}}")"
  split_body_code "$raw"
  if [[ "$CODE" == "200" ]]; then
    ok "restored alwaysOnline=$before_ao"
  else
    fail "restore alwaysOnline failed HTTP $CODE body=$BODY"
  fi
fi

summary
exit $?
