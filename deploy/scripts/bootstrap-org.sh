#!/bin/sh
# Creates (or reuses) a first organization and writes its ID to $OUTPUT,
# where the worker containers read it.
#
# The worker needs an org UUID before it can start, but an org only exists
# once someone registers — a chicken-and-egg that otherwise makes
# `docker compose up` a two-step process with a manual copy-paste in the
# middle. This closes that loop for local and demo use.
#
# Idempotent: registering an email that already exists falls back to logging
# in, so restarting the stack reuses the same org rather than failing.
set -eu

: "${API_BASE:=http://api:8080}"
: "${OUTPUT:=/bootstrap/org_id}"
: "${OVERRIDE_ORG_ID:=}"

mkdir -p "$(dirname "$OUTPUT")"

# An explicit WORKER_ORG_ID in .env always wins — that is the production path,
# where the org already exists and must not be guessed at.
if [ -n "$OVERRIDE_ORG_ID" ]; then
  printf '%s' "$OVERRIDE_ORG_ID" > "$OUTPUT"
  echo "[bootstrap] using WORKER_ORG_ID from the environment: $OVERRIDE_ORG_ID"
  exit 0
fi

if [ -s "$OUTPUT" ]; then
  echo "[bootstrap] org id already resolved: $(cat "$OUTPUT")"
  exit 0
fi

# Extracts a top-level-or-nested string field without needing jq, which the
# curl image does not ship. Good enough for this API's flat response shape.
extract() {
  sed -n 's/.*"'"$1"'":"\([^"]*\)".*/\1/p'
}

payload() {
  printf '{"organization_name":"%s","email":"%s","password":"%s"}' \
    "$ORG_NAME" "$EMAIL" "$PASSWORD"
}

echo "[bootstrap] registering organization \"$ORG_NAME\" as $EMAIL"
status=$(curl -sS -o /tmp/resp.json -w '%{http_code}' \
  -X POST "$API_BASE/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "$(payload)" || echo 000)

if [ "$status" != "201" ] && [ "$status" != "200" ]; then
  echo "[bootstrap] register returned HTTP $status; trying login (the org may already exist)"
  status=$(curl -sS -o /tmp/resp.json -w '%{http_code}' \
    -X POST "$API_BASE/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "$(printf '{"email":"%s","password":"%s"}' "$EMAIL" "$PASSWORD")" || echo 000)
fi

if [ "$status" != "200" ] && [ "$status" != "201" ]; then
  echo "[bootstrap] FAILED: could not register or log in (HTTP $status)"
  cat /tmp/resp.json 2>/dev/null || true
  exit 1
fi

org_id=$(extract org_id < /tmp/resp.json)

if [ -z "$org_id" ]; then
  echo "[bootstrap] FAILED: no org_id in the API response"
  cat /tmp/resp.json
  exit 1
fi

printf '%s' "$org_id" > "$OUTPUT"
echo "[bootstrap] org_id=$org_id written to $OUTPUT"
echo "[bootstrap] sign in at the dashboard with $EMAIL / $PASSWORD"
