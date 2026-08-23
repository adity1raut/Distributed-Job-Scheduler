# API Reference

Base URL is `http://localhost:8080` by default (whatever `API_PORT` is set
to). Everything is JSON in, JSON out. Every route under `/api` needs a
bearer token except the two auth endpoints:

```
Authorization: Bearer <token>
```

## Errors

Handlers never return a bare error string — it's always the same shape, so
the frontend has exactly one thing to parse:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "queue not found",
    "request_id": "b3f1c2a0-..."
  }
}
```

| Status | Code | Meaning |
|---|---|---|
| 400 | `BAD_REQUEST` | Validation failed (missing field, invalid JSON, bad enum value) |
| 401 | `UNAUTHORIZED` | Missing/invalid/expired bearer token, or bad credentials |
| 403 | `FORBIDDEN` | Authenticated but not permitted |
| 404 | `NOT_FOUND` | Resource doesn't exist, or doesn't belong to your org |
| 409 | `CONFLICT` | Duplicate (e.g. queue name already used in the project) |
| 429 | `RATE_LIMITED` | Over `RATE_LIMIT_PER_MIN` requests/minute for your org (or IP, pre-auth) |
| 500 | `INTERNAL` | Unexpected server error |

Treat `code` as the stable part — safe to switch on in code. `message` is
just for a human to read and can change wording over time.

## Pagination

The one endpoint that can genuinely grow large — job listings — pages with
a cursor instead of `OFFSET`. It's the difference between an index seek and
a full rescan once that table has a few million rows in it:

```json
{ "items": [...], "next_cursor": "eyJ0Ijoi..." }
```

First page: leave `cursor` off. Next page: pass back whatever `next_cursor`
you got. Once `next_cursor` stops showing up, you're at the end.

---

## Auth

### `POST /api/auth/register`
Registering isn't just creating a user — it stands up a brand-new
organization with you as its `owner`, and quietly seeds a sensible
exponential-backoff retry policy so you can create a queue right away
without configuring one yourself first.

```json
// request
{ "organization_name": "Acme", "email": "you@acme.com", "password": "at-least-8-chars" }

// response 201
{
  "token": "eyJhbGciOi...",
  "user": { "id": "...", "org_id": "...", "email": "you@acme.com", "role": "owner", "created_at": "..." }
}
```

### `POST /api/auth/login`
```json
// request
{ "email": "you@acme.com", "password": "..." }

// response 200 — same shape as register
```

---

## Projects

### `POST /api/projects` — create
`{ "name": "checkout-service" }` → `201` project.

### `GET /api/projects` — list all projects in your org

### `GET /api/projects/{projectID}` — get one

### `DELETE /api/projects/{projectID}` — delete (cascades to its queues, jobs, executions, logs)

### `POST /api/projects/{projectID}/queues` — create a queue under this project
```json
{ "name": "emails", "priority": 0, "concurrency_limit": 5, "retry_policy_id": null }
```
Leave `retry_policy_id` out and it just uses the org's default policy — no
need to look one up first if you don't care about a custom retry strategy yet.

### `GET /api/projects/{projectID}/queues` — list queues in this project

---

## Queues

### `GET /api/queues/{queueID}` — get one

### `PATCH /api/queues/{queueID}` — update config
```json
{ "priority": 2, "concurrency_limit": 10, "retry_policy_id": null }
```
All fields optional; only the ones present are changed.

### `DELETE /api/queues/{queueID}` — delete (cascades to its jobs)

### `POST /api/queues/{queueID}/pause` / `POST /api/queues/{queueID}/resume`
Pausing just stops new claims — nothing gets killed. Whatever's already
running finishes normally either way.

### `GET /api/queues/{queueID}/stats`
```json
{ "scheduled": 0, "queued": 4, "claimed": 1, "running": 1, "completed": 82, "failed": 3, "dead": 1 }
```

---

## Jobs

### `POST /api/queues/{queueID}/jobs` — submit
```json
{
  "type": "immediate",
  "payload": { "task": "echo" },
  "priority": 0,
  "idempotency_key": null,
  "delay_ms": 0,
  "run_at": null,
  "retry_policy_id": null,
  "batch_count": 0
}
```
`type` is one of `immediate | delayed | scheduled | batch`. Notice
`recurring` isn't in that list — you don't submit a recurring job directly,
you define a schedule (below), and each cron fire is what actually creates
a job row with `type: "recurring"`.

| type | extra field required | behavior |
|---|---|---|
| `immediate` | — | `run_at = now()` |
| `delayed` | `delay_ms > 0` | `run_at = now() + delay_ms` |
| `scheduled` | `run_at` (ISO 8601) | runs at that instant |
| `batch` | `batch_count >= 2` | creates N job rows sharing a `batch_id` |

Set `idempotency_key` and resubmitting the exact same key later won't create
a second job — you'll just get the original one back. One thing that trips
people up: the response is always an **array**, even for a single job. Only
`batch` gives you more than one element back.

```json
// 201
[{ "id": "...", "queue_id": "...", "type": "immediate", "status": "queued", "payload": {...}, "attempts": 0, "max_attempts": 5, "run_at": "...", "created_at": "...", "updated_at": "..." }]
```

### `GET /api/queues/{queueID}/jobs` — list / filter
Query params: `status`, `type`, `cursor`, `limit` (default 20, max 100).
`status` is one of `scheduled | queued | claimed | running | completed | failed | dead`.

### `GET /api/jobs/{jobID}` — get one, with its execution history
```json
{ "id": "...", "status": "completed", "payload": {...}, "attempts": 1, ..., "executions": [ { "id": "...", "attempt_number": 1, "status": "succeeded", "started_at": "...", "finished_at": "...", "duration_ms": 12 } ] }
```

### `POST /api/jobs/{jobID}/retry` — manual retry
Doesn't care what state the job is currently in — resets `attempts` back to
0 and requeues it right away. Works just as well on something that's `dead`
as something that's merely `failed`.

### `GET /api/executions/{executionID}/logs` — structured logs for one attempt

---

## Scheduled jobs (cron / recurring)

### `POST /api/queues/{queueID}/scheduled-jobs`
```json
{ "cron_expression": "*/5 * * * *", "payload_template": { "task": "echo" } }
```
Standard 5-field cron syntax. Nothing runs the instant you create this —
the scheduler wakes up every `SCHEDULER_TICK_SEC` seconds, checks which
definitions are due, and drops a `type: "recurring"` job into the queue for
each one.

### `GET /api/queues/{queueID}/scheduled-jobs` — list

### `POST /api/scheduled-jobs/{scheduledJobID}/pause` / `.../resume`

---

## Dead-letter queue

### `GET /api/queues/{queueID}/dlq` — list dead-lettered jobs for this queue
Query param: `limit` (default 50).

### `POST /api/dlq/{entryID}/replay`
Doesn't create a new job — it takes the original one, puts it back to
`queued` with a clean `attempts = 0`, and flags the dead-letter entry as
`replayed: true` so you can tell it's already been retried. Returns the job.

---

## Workers

### `GET /api/workers` — fleet status
```json
[{ "id": "...", "hostname": "...", "status": "online", "started_at": "...", "last_heartbeat_at": "...", "active_job_count": 2, "is_stale": false }]
```
Worth knowing: `is_stale` can be `true` even while `status` still says
`online`. That combination means the worker hasn't sent a heartbeat within
`STALE_JOB_SEC` — it's probably dead, just hasn't been formally reaped yet.

### `GET /api/workers/{workerID}/heartbeats` — heartbeat history
Query param: `limit` (default 50).

---

## Dashboard

### `GET /api/dashboard/overview`
```json
{
  "total_projects": 3, "total_queues": 7,
  "queued_jobs": 12, "running_jobs": 2,
  "completed_jobs_24h": 340, "failed_jobs_24h": 4, "dead_jobs": 1,
  "online_workers": 2
}
```

---

## Health

### `GET /healthz`
No token needed. Just returns `200 ok` — this is the one a container
orchestrator hits to decide if the API is still alive.
