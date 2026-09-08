#!/usr/bin/env bash
# End-to-end smoke test: proves a deployment can actually do the one thing
# this system exists for — accept a job and have a worker execute it.
#
# A health endpoint only says the process is up. This walks the full path:
# auth -> project -> queue -> job submission -> worker claim -> completion.
# Run it after every deploy; CD gates the release on it.
#
#   ./deploy/scripts/smoke-test.sh [base-url]
#
# Exits non-zero on the first failed step so CD can roll back.
set -euo pipefail

BASE="${1:-${SMOKE_BASE_URL:-http://localhost:3000}}"
TIMEOUT="${SMOKE_TIMEOUT:-60}"

# The credentials matter: a worker belongs to exactly one organization and
# only claims that org's jobs, so a smoke test that registers a fresh org
# would submit into a queue no worker is watching and always time out. Sign
# in as a user of the org the worker fleet actually serves — the compose
# bootstrap org by default.
EMAIL="${SMOKE_EMAIL:-admin@example.com}"
PASSWORD="${SMOKE_PASSWORD:-password123}"
ORG="${SMOKE_ORG:-Demo Org}"

# Project and queue names are per-run so repeat runs never collide.
SUFFIX="$(date +%s)-$$"

pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail() { printf '  \033[31m✗\033[0m %s\n' "$1" >&2; exit 1; }
step() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# First match, not last: the job-detail response embeds an "executions"
# array whose entries carry their own "status", and a greedy match would
# report an attempt's status as the job's.
json_field() {
  grep -o "\"$1\":\"[^\"]*\"" <<< "$2" | head -1 | sed 's/^[^:]*:"//; s/"$//'
}

echo "Smoke testing ${BASE}"

# ---------------------------------------------------------------------------
step "1. Health"
# Through the web tier this is proxied to the API, so a pass here already
# proves nginx -> API -> Postgres connectivity.
health="$(curl -fsS "${BASE}/readyz")" || fail "readiness probe unreachable"
grep -q '"status":"ok"' <<< "$health" || fail "not ready: $health"
pass "readyz reports ok"

# ---------------------------------------------------------------------------
step "2. Authentication"
# Log in first; register only if this org does not exist yet (first deploy).
auth="$(curl -fsS -X POST "${BASE}/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" 2>/dev/null)" || auth=""

if [ -z "$auth" ]; then
  echo "  login failed, registering ${ORG}"
  auth="$(curl -fsS -X POST "${BASE}/api/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"organization_name\":\"${ORG}\",\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")" \
    || fail "could not log in or register as ${EMAIL}"
fi

TOKEN="$(json_field token "$auth")"
ORG_ID="$(json_field org_id "$auth")"
[ -n "$TOKEN" ] || fail "no token in auth response"
pass "authenticated as ${EMAIL} (org ${ORG_ID})"

AUTH_HEADER="Authorization: Bearer ${TOKEN}"

# ---------------------------------------------------------------------------
step "3. Project and queue"
project="$(curl -fsS -X POST "${BASE}/api/projects" \
  -H "$AUTH_HEADER" -H 'Content-Type: application/json' \
  -d "{\"name\":\"smoke-${SUFFIX}\"}")" || fail "create project failed"
PROJECT_ID="$(json_field id "$project")"
[ -n "$PROJECT_ID" ] || fail "no project id returned"
pass "project ${PROJECT_ID}"

queue="$(curl -fsS -X POST "${BASE}/api/projects/${PROJECT_ID}/queues" \
  -H "$AUTH_HEADER" -H 'Content-Type: application/json' \
  -d "{\"name\":\"smoke-queue-${SUFFIX}\"}")" || fail "create queue failed"
QUEUE_ID="$(json_field id "$queue")"
[ -n "$QUEUE_ID" ] || fail "no queue id returned"
pass "queue ${QUEUE_ID}"

# ---------------------------------------------------------------------------
step "4. Worker fleet"
workers="$(curl -fsS "${BASE}/api/workers" -H "$AUTH_HEADER")" || fail "list workers failed"
online="$(grep -o '"status":"online"' <<< "$workers" | wc -l | tr -d ' ')"
if [ "$online" -eq 0 ]; then
  if [ "${SMOKE_SKIP_WORKER:-0}" = "1" ]; then
    printf '  \033[33m!\033[0m no online workers in org %s — skipping execution check\n' "$ORG_ID"
    printf '\n\033[32mAPI smoke tests passed (worker check skipped).\033[0m\n'
    exit 0
  fi
  fail "no online workers registered for org ${ORG_ID}. A worker only claims its own org's jobs — start one with WORKER_ORG_ID=${ORG_ID}, or set SMOKE_SKIP_WORKER=1 to check the API alone."
fi
pass "${online} worker(s) online in this org"

step "5. Job execution"
job="$(curl -fsS -X POST "${BASE}/api/queues/${QUEUE_ID}/jobs" \
  -H "$AUTH_HEADER" -H 'Content-Type: application/json' \
  -d '{"type":"immediate","payload":{"task":"noop","sleep_ms":10}}')" \
  || fail "submit job failed"
# Submit returns an array (one job, or batch_count of them); the first id is ours.
JOB_ID="$(json_field id "$job")"
[ -n "$JOB_ID" ] || fail "no job id returned"
pass "submitted job ${JOB_ID}"

# This is the assertion that matters. A job only reaches "completed" if a
# worker polled this org's queue, won the claim, ran the payload and wrote
# the result back — every moving part in one check.
echo "  waiting up to ${TIMEOUT}s for a worker to complete it..."
deadline=$(( $(date +%s) + TIMEOUT ))
status=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  detail="$(curl -fsS "${BASE}/api/jobs/${JOB_ID}" -H "$AUTH_HEADER")" || true
  status="$(json_field status "$detail")"
  case "$status" in
    completed) pass "job completed — worker fleet is live"; break ;;
    failed|dead_letter) fail "job ended in status '${status}'" ;;
  esac
  sleep 2
done
[ "$status" = "completed" ] || fail "job stuck in '${status:-unknown}' after ${TIMEOUT}s (is a worker running with the right WORKER_ORG_ID?)"

printf '\n\033[32mAll smoke tests passed.\033[0m\n'
