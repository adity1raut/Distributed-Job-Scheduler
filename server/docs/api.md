# API Reference

REST API for the Distributed Job Scheduler. Every endpoint accepts and
returns `application/json`, and every route except registration and login
requires a bearer token.

## Contents

- [Getting started](#getting-started)
- [Conventions](#conventions)
  - [Authentication](#authentication)
  - [Identifiers and timestamps](#identifiers-and-timestamps)
  - [Errors](#errors)
  - [Pagination](#pagination)
  - [Rate limiting](#rate-limiting)
- [Quickstart](#quickstart)
- [Auth](#auth)
- [Projects](#projects)
- [Queues](#queues)
- [Retry policies](#retry-policies)
- [Jobs](#jobs)
- [Scheduled jobs (cron)](#scheduled-jobs-cron)
- [Dead letter queue](#dead-letter-queue)
- [Workers](#workers)
- [Dashboard](#dashboard)
- [Health](#health)

## Getting started

| | |
|---|---|
| **Base URL** | `http://localhost:8080` in development (whatever `API_PORT` is set to) |
| **Format** | JSON request and response bodies throughout |
| **Auth** | Bearer JWT on every route under `/api`, except `POST /api/auth/register` and `POST /api/auth/login` |
| **Versioning** | Unversioned — there is one API surface, at the root of `/api` |

## Conventions

### Authentication

Every organization is a tenant. Registering creates one; every other user
in that org authenticates against the same organization. Send the token
issued at register/login on every subsequent request:

```
Authorization: Bearer <token>
```

The token encodes `user_id`, `org_id`, and `role`. Every resource lookup in
this API is implicitly scoped to the caller's `org_id` — you cannot read or
modify another organization's data by guessing an ID; you'll get `404 NOT
FOUND` rather than `403 FORBIDDEN`, so a request can't be used to confirm
whether a resource exists in someone else's org.

Tokens expire after `JWT_EXPIRY_HOURS` (default 24). An expired or invalid
token returns `401 UNAUTHORIZED` on any protected route.

### Identifiers and timestamps

- Every resource ID is a UUID (v4), generated server-side. IDs are safe to
  expose in URLs and API responses — they don't reveal row counts or
  creation order the way a sequential integer would.
- Every timestamp is RFC 3339 / ISO 8601, in UTC, e.g. `2026-08-24T09:30:00Z`.

### Errors

No handler ever returns a bare string or an ad hoc shape. Every error,
from every endpoint, looks like this:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "queue not found",
    "request_id": "b3f1c2a0-7e4d-4a1a-9c3e-1f2b3c4d5e6f"
  }
}
```

`code` is the stable, machine-readable part — switch on it in client code.
`message` is for a human and its exact wording may change between
releases. `request_id` matches the `X-Request-ID` response header and the
structured server log line for that request — hand it over when reporting
a bug and the exact request is one lookup away.

| Status | Code | Meaning |
|---|---|---|
| 400 | `BAD_REQUEST` | Validation failed — a missing field, malformed JSON, or an invalid enum value |
| 401 | `UNAUTHORIZED` | Missing, invalid, or expired bearer token, or incorrect login credentials |
| 403 | `FORBIDDEN` | Authenticated, but your role doesn't permit this action |
| 404 | `NOT_FOUND` | The resource doesn't exist, or doesn't belong to your organization |
| 409 | `CONFLICT` | A uniqueness constraint was violated (e.g. a queue name already used in this project) |
| 429 | `RATE_LIMITED` | You've exceeded `RATE_LIMIT_PER_MIN` requests/minute |
| 500 | `INTERNAL` | An unexpected server-side error |

### Pagination

Job listings are the one collection that can genuinely grow large, so
they're paginated with an opaque, keyset cursor over `(created_at, id)`
rather than `OFFSET` — a cursor stays an index seek no matter how deep you
page, where `OFFSET` degrades into a full scan once the table has millions
of rows.

```json
{ "items": [ /* ... */ ], "next_cursor": "eyJ0Ijoi..." }
```

Request the first page with no `cursor` parameter. To fetch the next page,
pass the exact `next_cursor` value you were given back as the `cursor`
query parameter. When a response has no `next_cursor` field, you've reached
the last page.

### Rate limiting

Every request under `/api` is counted against a per-organization budget
(per-IP for the two pre-auth routes), enforced with a Redis-backed counter
so the limit holds even when the API is running multiple replicas. Every
response carries:

```
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 117
```

Exceeding `RATE_LIMIT_PER_MIN` (default 120) returns `429 RATE_LIMITED`. If
Redis itself is unreachable, requests are allowed through rather than
rejected — a temporarily-unenforced soft limit is judged the smaller
failure compared to the whole API going down over a non-critical
dependency.

## Quickstart

An end-to-end walkthrough — register, create a project and a queue, submit
a job, and check on it:

```bash
# 1. Register — this creates both your organization and your user
curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"organization_name":"Acme","email":"you@acme.com","password":"at-least-8-chars"}'
# => { "token": "eyJhbGci...", "user": { "id": "...", "org_id": "...", "role": "owner", ... } }

TOKEN="eyJhbGci..."   # the token from above

# 2. Create a project
curl -s -X POST http://localhost:8080/api/projects \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"checkout-service"}'
# => { "id": "<projectID>", ... }

# 3. Create a queue in that project
curl -s -X POST http://localhost:8080/api/projects/<projectID>/queues \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"emails","priority":0,"concurrency_limit":5}'
# => { "id": "<queueID>", ... }

# 4. Submit a job
curl -s -X POST http://localhost:8080/api/queues/<queueID>/jobs \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"type":"immediate","payload":{"task":"echo"}}'
# => [ { "id": "<jobID>", "status": "queued", ... } ]

# 5. Check on it (needs a worker running to actually complete — see the README)
curl -s http://localhost:8080/api/jobs/<jobID> -H "Authorization: Bearer $TOKEN"
# => { "status": "completed", "executions": [ ... ], ... }
```

---

## Auth

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/register` | Create an organization and its first (`owner`) user |
| `POST` | `/api/auth/login` | Exchange credentials for a bearer token |

#### `POST /api/auth/register`

Registering is not just creating a user — it provisions a brand-new
organization with you as its `owner`, and silently seeds a sensible
exponential-backoff retry policy for that org so you can create a queue
immediately without configuring a policy yourself first.

**Authorization:** none required.

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `organization_name` | string | yes | Display name for the new organization |
| `email` | string | yes | Must be unique across the whole system |
| `password` | string | yes | Minimum 8 characters |

```json
// Request
{ "organization_name": "Acme", "email": "you@acme.com", "password": "at-least-8-chars" }
```

**Response** `201 Created`

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": "5b1e...", "org_id": "9c4a...", "email": "you@acme.com",
    "role": "owner", "created_at": "2026-08-24T09:30:00Z"
  }
}
```

**Errors:** `400 BAD_REQUEST` (password under 8 characters, missing field) ·
`409 CONFLICT` (email already registered)

#### `POST /api/auth/login`

**Authorization:** none required.

**Request body**

| Field | Type | Required |
|---|---|---|
| `email` | string | yes |
| `password` | string | yes |

**Response** `200 OK` — identical shape to register's response.

**Errors:** `401 UNAUTHORIZED` (wrong email or password — deliberately not
distinguished, so a failed attempt can't be used to confirm which emails
are registered)

---

## Projects

A project is a namespace that owns one or more queues. Every project
belongs to exactly one organization.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/projects` | Create a project |
| `GET` | `/api/projects` | List every project in your organization |
| `GET` | `/api/projects/{projectID}` | Get one project |
| `DELETE` | `/api/projects/{projectID}` | Delete a project and everything under it |
| `POST` | `/api/projects/{projectID}/queues` | Create a queue in this project |
| `GET` | `/api/projects/{projectID}/queues` | List queues in this project |

#### `POST /api/projects`

**Request body**

| Field | Type | Required |
|---|---|---|
| `name` | string | yes |

**Response** `201 Created` — the created project:

```json
{ "id": "...", "org_id": "...", "owner_id": "...", "name": "checkout-service", "created_at": "..." }
```

#### `GET /api/projects`
**Response** `200 OK` — an array of projects, no pagination (project counts
per org are small enough not to need it).

#### `GET /api/projects/{projectID}`
**Response** `200 OK` — a single project. `404 NOT_FOUND` if it doesn't
exist or belongs to a different organization.

#### `DELETE /api/projects/{projectID}`

Cascades to every queue, job, job execution, and log under this project —
irreversibly.

**Authorization:** `owner` or `admin` only. A `member` receives `403 FORBIDDEN`.

**Response** `204 No Content`

#### `POST /api/projects/{projectID}/queues`

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique within the project |
| `priority` | int | no | Higher runs first; default `0` |
| `concurrency_limit` | int | no | Max jobs `claimed`/`running` at once; default `5` |
| `retry_policy_id` | uuid | no | See [Retry policies](#retry-policies). Omit to use the org's default policy |

```json
{ "name": "emails", "priority": 0, "concurrency_limit": 5, "retry_policy_id": null }
```

**Response** `201 Created` — the created queue. `409 CONFLICT` if the name
is already used in this project.

#### `GET /api/projects/{projectID}/queues`
**Response** `200 OK` — an array of queues, unpaginated.

---

## Queues

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/queues/{queueID}` | Get one queue |
| `PATCH` | `/api/queues/{queueID}` | Update its configuration |
| `DELETE` | `/api/queues/{queueID}` | Delete the queue and its jobs |
| `POST` | `/api/queues/{queueID}/pause` | Stop it from claiming new jobs |
| `POST` | `/api/queues/{queueID}/resume` | Resume claiming |
| `GET` | `/api/queues/{queueID}/stats` | Live per-status job counts |

#### `GET /api/queues/{queueID}`
**Response** `200 OK` — the queue, including its current `priority`,
`concurrency_limit`, `retry_policy_id`, and `is_paused`.

#### `PATCH /api/queues/{queueID}`

All fields are optional — only the ones present in the request body are
changed; omitted fields keep their current value.

**Request body**

| Field | Type | Description |
|---|---|---|
| `priority` | int | |
| `concurrency_limit` | int | |
| `retry_policy_id` | uuid | |

```json
{ "priority": 2, "concurrency_limit": 10, "retry_policy_id": null }
```

**Response** `200 OK` — the updated queue.

#### `DELETE /api/queues/{queueID}`

Cascades to every job, execution, and log in this queue — irreversibly.

**Authorization:** `owner` or `admin` only. A `member` receives `403 FORBIDDEN`.

**Response** `204 No Content`

#### `POST /api/queues/{queueID}/pause` · `POST /api/queues/{queueID}/resume`

Pausing only stops the queue from being offered to workers for new claims
— nothing already running is interrupted, and it finishes normally either
way.

**Response** `200 OK`

```json
{ "is_paused": true }
```

#### `GET /api/queues/{queueID}/stats`

**Response** `200 OK` — a live count of jobs in this queue by status:

```json
{ "scheduled": 0, "queued": 4, "claimed": 1, "running": 1, "completed": 82, "failed": 3, "dead": 1 }
```

---

## Retry policies

A retry policy defines a backoff strategy (`fixed`, `linear`, or
`exponential`), a base/max delay, and a maximum attempt count. Every
organization gets exactly one, seeded automatically at registration, and
every job that doesn't specify otherwise inherits its queue's policy.

> **No REST endpoint exists yet to create additional policies.** The
> underlying repository supports it (`Create`, `GetByID`, `ListByOrg`), and
> both queue creation and job submission already accept an optional
> `retry_policy_id`, but nothing in this API currently issues one beyond
> the org's auto-seeded default. In practice this means every queue today
> runs the same backoff strategy unless that changes. Documented here
> rather than silently left implicit — this is a known gap, not an
> oversight.

The default policy, seeded at registration:

```json
{
  "name": "default", "strategy": "exponential",
  "base_delay_ms": 5000, "max_delay_ms": 60000,
  "max_attempts": 5, "multiplier": 2
}
```

With this policy, a failing job retries after 5s, 10s, 20s, then 40s,
dead-lettering after the 5th failed attempt — roughly 75 seconds from
first failure to dead letter.

---

## Jobs

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/queues/{queueID}/jobs` | Submit one or more jobs |
| `GET` | `/api/queues/{queueID}/jobs` | List / filter jobs in a queue |
| `GET` | `/api/jobs/{jobID}` | Get one job, with its execution history |
| `POST` | `/api/jobs/{jobID}/retry` | Manually retry a job |
| `GET` | `/api/executions/{executionID}/logs` | Structured logs for one attempt |

#### `POST /api/queues/{queueID}/jobs`

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | One of `immediate`, `delayed`, `scheduled`, `batch` (see below) |
| `payload` | object | no | Arbitrary JSON handed to the job at execution time; defaults to `{}` |
| `priority` | int | no | Higher claims first within the queue; default `0` |
| `idempotency_key` | string | no | See below |
| `delay_ms` | int | required if `type` is `delayed` | Must be `> 0` |
| `run_at` | timestamp | required if `type` is `scheduled` | ISO 8601 |
| `retry_policy_id` | uuid | no | Overrides the queue's policy for this job only |
| `batch_count` | int | required if `type` is `batch` | Must be `>= 2` |

| `type` | Effect on `run_at` |
|---|---|
| `immediate` | `now()` |
| `delayed` | `now() + delay_ms` |
| `scheduled` | exactly the `run_at` you supplied |
| `batch` | `now()`, applied to every row created |

> **`recurring` is not a submittable type.** You don't submit a recurring
> job directly — you create a cron definition under
> [Scheduled jobs](#scheduled-jobs-cron), and each due cron fire is what
> creates a job row with `type: "recurring"`.

Supplying an `idempotency_key` makes submission safe to retry: resubmitting
the exact same key returns the original job rather than creating a
duplicate.

> **The response is always an array**, even for a single job — only
> `batch` ever returns more than one element.

```json
// 201 Created
[
  {
    "id": "...", "queue_id": "...", "type": "immediate", "status": "queued",
    "payload": { "task": "echo" }, "priority": 0, "attempts": 0, "max_attempts": 5,
    "run_at": "2026-08-24T09:30:00Z", "created_at": "...", "updated_at": "..."
  }
]
```

**Errors:** `400 BAD_REQUEST` — an invalid `type`, or a type-specific field
missing (`delay_ms` for `delayed`, `run_at` for `scheduled`, `batch_count`
for `batch`).

#### `GET /api/queues/{queueID}/jobs`

**Query parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `status` | string | — | One of `scheduled`, `queued`, `claimed`, `running`, `completed`, `failed`, `dead` |
| `type` | string | — | One of `immediate`, `delayed`, `scheduled`, `recurring`, `batch` |
| `cursor` | string | — | Opaque cursor from a previous page's `next_cursor` |
| `limit` | int | `20` | Max `100` |

**Response** `200 OK` — see [Pagination](#pagination) for the envelope shape.

#### `GET /api/jobs/{jobID}`

**Response** `200 OK` — the job plus its full execution history:

```json
{
  "id": "...", "status": "completed", "payload": { "task": "echo" }, "attempts": 1,
  "max_attempts": 5, "run_at": "...", "created_at": "...", "updated_at": "...",
  "executions": [
    {
      "id": "...", "attempt_number": 1, "status": "succeeded",
      "started_at": "...", "finished_at": "...", "duration_ms": 12
    }
  ]
}
```

#### `POST /api/jobs/{jobID}/retry`

Works regardless of the job's current status — resets `attempts` to `0`
and requeues it immediately. Just as valid on a job that's `dead` as one
that's merely `failed`.

**Response** `200 OK` — the updated job.

#### `GET /api/executions/{executionID}/logs`

**Response** `200 OK` — an array of structured log entries
(`debug`/`info`/`warn`/`error`) written during that one attempt.

---

## Scheduled jobs (cron)

A scheduled job is a standing cron *definition*, not a job row. It
describes a recurring pattern; the scheduler (a goroutine inside every API
replica, coordinated across replicas with a Postgres advisory lock) checks
every `SCHEDULER_TICK_SEC` seconds for definitions that are due and drops
one `type: "recurring"` job into the queue per firing.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/queues/{queueID}/scheduled-jobs` | Create a cron definition |
| `GET` | `/api/queues/{queueID}/scheduled-jobs` | List cron definitions in a queue |
| `POST` | `/api/scheduled-jobs/{scheduledJobID}/pause` | Stop firing |
| `POST` | `/api/scheduled-jobs/{scheduledJobID}/resume` | Resume firing |

#### `POST /api/queues/{queueID}/scheduled-jobs`

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `cron_expression` | string | yes | Standard 5-field cron syntax |
| `payload_template` | object | yes | Copied as-is into every job this definition creates |

```json
{ "cron_expression": "*/5 * * * *", "payload_template": { "task": "echo" } }
```

Nothing runs the instant you create this definition — the first job row
appears at its next scheduled tick.

**Response** `201 Created` — the created definition, including its computed
`next_run_at`.

#### `GET /api/queues/{queueID}/scheduled-jobs`
**Response** `200 OK` — an array, unpaginated.

#### `POST /api/scheduled-jobs/{scheduledJobID}/pause` · `.../resume`
**Response** `200 OK` — `{ "is_active": false }` (or `true`).

---

## Dead letter queue

A job lands here once it has exhausted every retry attempt under its
policy — a permanent failure that needs a human or an automated replay to
move again.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/queues/{queueID}/dlq` | List dead-lettered jobs in this queue |
| `POST` | `/api/dlq/{entryID}/replay` | Requeue the original job |

#### `GET /api/queues/{queueID}/dlq`

**Query parameters**

| Name | Type | Default |
|---|---|---|
| `limit` | int | `50` |

**Response** `200 OK`:

```json
[
  {
    "id": "...", "job_id": "...", "final_error": "connection timed out",
    "failed_at": "...", "replayed": false
  }
]
```

#### `POST /api/dlq/{entryID}/replay`

This does not create a new job — it takes the original job, resets it to
`queued` with `attempts` back to `0`, and flags this dead-letter entry as
`replayed: true` so it's clear it's already been actioned.

**Response** `200 OK` — the requeued job.

---

## Workers

A worker process belongs to exactly one organization (its `WORKER_ORG_ID`
at startup) and only ever claims that organization's jobs. This endpoint
only ever returns workers belonging to your own org.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/workers` | Fleet status for your organization |
| `GET` | `/api/workers/{workerID}/heartbeats` | Heartbeat history for one worker |

#### `GET /api/workers`

**Response** `200 OK`:

```json
[
  {
    "id": "...", "hostname": "worker-1.internal", "status": "online",
    "started_at": "...", "last_heartbeat_at": "...",
    "active_job_count": 2, "is_stale": false
  }
]
```

> `is_stale` can be `true` while `status` still reads `online`. That
> combination means the worker has missed its `STALE_JOB_SEC` heartbeat
> window — it's very likely dead, just not yet formally reaped by the
> scheduler's next tick.

#### `GET /api/workers/{workerID}/heartbeats`

**Query parameters**

| Name | Type | Default |
|---|---|---|
| `limit` | int | `50` |

**Response** `200 OK` — an array of `{ reported_at, active_job_count }` samples,
newest first. Scoped to your org: a `workerID` belonging to another
organization returns an empty array rather than another org's data.

---

## Dashboard

Aggregate, cross-project views for the landing page — everything here is
scoped to your organization across every project in it.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/dashboard/overview` | Org-wide counters |
| `GET` | `/api/dashboard/recent-jobs` | Recent jobs across every queue |
| `GET` | `/api/dashboard/throughput` | Hourly completed/failed counts, last 24h |

#### `GET /api/dashboard/overview`

**Response** `200 OK`:

```json
{
  "total_projects": 3, "total_queues": 7,
  "queued_jobs": 12, "running_jobs": 2,
  "completed_jobs_24h": 340, "failed_jobs_24h": 4, "dead_jobs": 1,
  "online_workers": 2
}
```

#### `GET /api/dashboard/recent-jobs`

The most recently created jobs across every queue in every project in your
org, newest first — the org-wide equivalent of a single queue's job list,
so you don't need to open a specific queue to see recent activity.

**Query parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `limit` | int | `10` | |
| `status` | string | — (all statuses) | Same status values as job listing |

**Response** `200 OK`:

```json
[
  {
    "id": "...", "queue_id": "...", "queue_name": "emails",
    "project_id": "...", "project_name": "checkout-service",
    "type": "immediate", "status": "completed",
    "attempts": 1, "max_attempts": 5,
    "run_at": "...", "created_at": "...", "updated_at": "..."
  }
]
```

#### `GET /api/dashboard/throughput`

One bucket per hour for the last 24 hours, oldest first, zero-filled for
hours with no finished jobs — the time-series counterpart to
`completed_jobs_24h` / `failed_jobs_24h` on the overview, for actually
visualizing throughput rather than reading a single rolled-up number.

**Response** `200 OK`:

```json
[
  { "hour": "2026-08-23T14:00:00Z", "completed": 12, "failed": 1 },
  { "hour": "2026-08-23T15:00:00Z", "completed": 8, "failed": 0 }
]
```

---

## Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness probe |

#### `GET /healthz`

**Authorization:** none required.

Returns `200 ok` as plain text with no body parsing needed — this is the
endpoint a container orchestrator or load balancer hits to decide whether
this replica is still alive.
