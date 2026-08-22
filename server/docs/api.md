# API Reference

Base URL: `http://localhost:8080` (or whatever `API_PORT` is set to).

All request/response bodies are JSON. All endpoints under `/api` except
`/api/auth/*` require a bearer token:

```
Authorization: Bearer <token>
```

## Errors

Every error response has the same shape:

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

`code` is stable and safe to switch on; `message` is human-readable and may change.

## Pagination

List endpoints that can grow large (jobs) use **keyset pagination**, not
`OFFSET` — cheap to page through no matter how deep, unlike offset pagination
on a large table. Response shape:

```json
{ "items": [...], "next_cursor": "eyJ0Ijoi..." }
```

Pass the cursor back as `?cursor=...` to get the next page; omit it (or leave
empty) for the first page. `next_cursor` is absent once there's no more data.

---

## Auth

### `POST /api/auth/register`
Creates a new organization and its first user (role `owner`), and seeds a
default exponential-backoff retry policy for the org.

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
`retry_policy_id` is optional — omitted, it falls back to the org's default policy.

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
Stops/resumes workers claiming new jobs from this queue. In-flight jobs finish either way.

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
`type` is one of `immediate | delayed | scheduled | batch` (recurring jobs are
created via the scheduled-jobs endpoints below, not here — a cron fire is
what turns into a job row of type `recurring`).

| type | extra field required | behavior |
|---|---|---|
| `immediate` | — | `run_at = now()` |
| `delayed` | `delay_ms > 0` | `run_at = now() + delay_ms` |
| `scheduled` | `run_at` (ISO 8601) | runs at that instant |
| `batch` | `batch_count >= 2` | creates N job rows sharing a `batch_id` |

`idempotency_key`, if set, is unique per queue — resubmitting the same key
returns the original job instead of creating a duplicate. Response is always
an **array** of jobs (length 1 except for `batch`):
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
Resets `attempts` to 0 and requeues immediately, regardless of current status
(works from `failed` or `dead`).

### `GET /api/executions/{executionID}/logs` — structured logs for one attempt

---

## Scheduled jobs (cron / recurring)

### `POST /api/queues/{queueID}/scheduled-jobs`
```json
{ "cron_expression": "*/5 * * * *", "payload_template": { "task": "echo" } }
```
Standard 5-field cron (`robfig/cron` standard parser). The scheduler ticks
every `SCHEDULER_TICK_SEC` seconds and inserts a `type: "recurring"` job for
every definition whose `next_run_at` has passed.

### `GET /api/queues/{queueID}/scheduled-jobs` — list

### `POST /api/scheduled-jobs/{scheduledJobID}/pause` / `.../resume`

---

## Dead-letter queue

### `GET /api/queues/{queueID}/dlq` — list dead-lettered jobs for this queue
Query param: `limit` (default 50).

### `POST /api/dlq/{entryID}/replay`
Resets the original job to `queued` with `attempts = 0` and marks the
dead-letter entry `replayed: true`. Response is the requeued job.

---

## Workers

### `GET /api/workers` — fleet status
```json
[{ "id": "...", "hostname": "...", "status": "online", "started_at": "...", "last_heartbeat_at": "...", "active_job_count": 2, "is_stale": false }]
```
`is_stale` is true when `status` is `online` but the last heartbeat is older
than `STALE_JOB_SEC` — the worker is likely dead but hasn't been reaped yet.

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
No auth. Returns `200 ok` — used for container/orchestrator liveness checks.
