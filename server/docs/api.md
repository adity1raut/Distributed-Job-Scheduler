# API Reference

REST API for the Distributed Job Scheduler. Every endpoint accepts and
returns `application/json`, and every route except registration and login
requires a bearer token.

## Contents

- [Conventions](#conventions)
- [Endpoint index](#endpoint-index)
- [Endpoints that matter](#endpoints-that-matter)
- [Health](#health)

## Conventions

**Auth.** Registering creates an organization; every other user in that org
authenticates against it. Send the issued token on every request:

```
Authorization: Bearer <token>
```

The token encodes `user_id`, `org_id`, and `role`. Every lookup is
implicitly scoped to the caller's `org_id`. Reaching for another
organization's resource by ID returns `404 NOT_FOUND`, not `403`, so a
request can't be used to confirm whether a resource exists elsewhere.
Tokens expire after `JWT_EXPIRY_HOURS` (default 24).

**IDs & timestamps.** Every ID is a server-generated UUID (v4), safe to
expose without revealing row counts or creation order. Every timestamp is
RFC 3339 UTC, e.g. `2026-08-24T09:30:00Z`.

**Errors.** No handler ever returns a bare string. Every error looks like:

```json
{ "error": { "code": "NOT_FOUND", "message": "queue not found", "request_id": "b3f1c2a0-..." } }
```

`code` is the stable, machine-readable part to switch on; `message` is for
humans and may change wording between releases; `request_id` matches the
`X-Request-ID` response header and the server log line, so a bug report is
one lookup away from the exact request.

| Status | Code | Meaning |
|---|---|---|
| 400 | `BAD_REQUEST` | Validation failed |
| 401 | `UNAUTHORIZED` | Missing/invalid/expired token, or bad login credentials |
| 403 | `FORBIDDEN` | Authenticated, but your role doesn't permit this |
| 404 | `NOT_FOUND` | Doesn't exist, or belongs to another organization |
| 409 | `CONFLICT` | A uniqueness constraint was violated |
| 429 | `RATE_LIMITED` | Exceeded `RATE_LIMIT_PER_MIN` |
| 500 | `INTERNAL` | Unexpected server-side error |

**Pagination.** Job listings, the one collection that can genuinely grow
large, use an opaque keyset cursor over `(created_at, id)` instead of
`OFFSET`, so a page stays an index seek no matter how deep you page:

```json
{ "items": [ /* ... */ ], "next_cursor": "eyJ0Ijoi..." }
```

Omit `cursor` for the first page; pass back the exact `next_cursor` value
for the next one. No `next_cursor` in the response means you've reached
the last page.

**Rate limiting.** Every request is counted against a per-organization
budget (per-IP pre-auth), enforced with a Redis-backed counter so the limit
holds across replicas. `X-RateLimit-Limit` / `X-RateLimit-Remaining` are
sent on every response. If Redis is unreachable, requests are allowed
through rather than rejected. Losing a soft limit briefly is judged a
smaller failure than an outage over a non-critical dependency.

## Endpoint index

Every route in the API, for reference. Behavior worth knowing beyond the
method and path is covered for the starred (`*`) rows in
[Endpoints that matter](#endpoints-that-matter) below; the rest are
standard CRUD/list operations with no surprises.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/register` * | Create an organization and its first (`owner`) user |
| `POST` | `/api/auth/login` | Exchange credentials for a bearer token |
| `POST` | `/api/projects` | Create a project |
| `GET` | `/api/projects` | List projects in your organization (unpaginated) |
| `GET` | `/api/projects/{projectID}` | Get one project |
| `DELETE` | `/api/projects/{projectID}` * | Delete a project and everything under it (`owner`/`admin` only) |
| `POST` | `/api/projects/{projectID}/queues` | Create a queue |
| `GET` | `/api/projects/{projectID}/queues` | List queues in a project (unpaginated) |
| `GET` | `/api/queues/{queueID}` | Get one queue |
| `PATCH` | `/api/queues/{queueID}` | Update priority, concurrency limit, or retry policy |
| `DELETE` | `/api/queues/{queueID}` * | Delete a queue and its jobs (`owner`/`admin` only) |
| `POST` | `/api/queues/{queueID}/pause` · `/resume` | Stop/resume offering this queue's jobs to workers |
| `GET` | `/api/queues/{queueID}/stats` | Live per-status job counts |
| `POST` | `/api/queues/{queueID}/jobs` * | Submit one or more jobs |
| `GET` | `/api/queues/{queueID}/jobs` | List/filter jobs in a queue (paginated) |
| `GET` | `/api/jobs/{jobID}` | Get one job plus its execution history |
| `POST` | `/api/jobs/{jobID}/retry` * | Manually retry a job, any status |
| `GET` | `/api/executions/{executionID}/logs` | Structured logs for one attempt |
| `POST` | `/api/queues/{queueID}/scheduled-jobs` * | Create a cron definition |
| `GET` | `/api/queues/{queueID}/scheduled-jobs` | List cron definitions (unpaginated) |
| `POST` | `/api/scheduled-jobs/{id}/pause` · `/resume` | Stop/resume a cron definition |
| `GET` | `/api/queues/{queueID}/dlq` | List dead-lettered jobs in a queue |
| `POST` | `/api/dlq/{entryID}/replay` * | Requeue a permanently-failed job |
| `GET` | `/api/workers` * | Fleet status for your organization |
| `GET` | `/api/workers/{workerID}/heartbeats` | Heartbeat history for one worker |
| `GET` | `/api/dashboard/overview` | Org-wide counters |
| `GET` | `/api/dashboard/recent-jobs` | Recent jobs across every queue/project |
| `GET` | `/api/dashboard/throughput` | Hourly completed/failed counts, last 24h |
| `GET` | `/healthz` | Liveness probe, no auth |

## Endpoints that matter

The rest of this API is unsurprising REST CRUD: create/list/get/delete on
a resource, exactly as the table above says. These are the endpoints where
something non-obvious actually happens.

#### `POST /api/auth/register`

Not just user creation: it provisions a brand-new organization with you as
its `owner`, and silently seeds a default exponential-backoff retry policy
(`5s → 10s → 20s → 40s`, 5 max attempts) so you can create a queue and
submit a job immediately, without configuring a policy first.

#### `POST /api/queues/{queueID}/jobs`

The one endpoint that carries most of this API's real behavior. `type` is
one of `immediate | delayed | scheduled | batch`, and it decides how
`run_at` is computed:

| Type | Required field | `run_at` |
|---|---|---|
| `immediate` | none | `now()` |
| `delayed` | `delay_ms > 0` | `now() + delay_ms` |
| `scheduled` | `run_at` (ISO 8601) | exactly what you supplied |
| `batch` | `batch_count >= 2` | `now()`, applied to every row created |

Three behaviors worth knowing:

- **The response is always an array**, even for a single job. Only
  `batch` ever returns more than one element.
- **`idempotency_key` makes resubmission safe.** Resubmitting the same key
  returns the *original* job rather than creating a duplicate; this is
  enforced by a database constraint, not just application logic.
- **`recurring` is not submittable here.** You create a cron definition
  under `POST /scheduled-jobs` instead. Each due firing is what produces
  a job row with `type: "recurring"`.

#### `POST /api/jobs/{jobID}/retry`

Works regardless of current status, just as valid on `dead` as on
`failed`. Resets `attempts` to `0` and requeues immediately.

#### `POST /api/dlq/{entryID}/replay`

Does **not** create a new job. It takes the original job, resets it to
`queued` with `attempts` back to `0`, and marks the dead-letter entry
`replayed: true` so it's clear it's already been actioned.

#### `POST /api/queues/{queueID}/scheduled-jobs`

Creates a cron *definition*, not a job. Nothing runs the instant you
create it. The scheduler drops one `type: "recurring"` job into the
queue at each due firing, computed from the standard 5-field
`cron_expression` you provide.

#### `GET /api/workers`

`is_stale` can be `true` while `status` still reads `online`. That
combination means the worker missed its `STALE_JOB_SEC` heartbeat window.
It's very likely dead, just not yet formally reaped by the scheduler's
next tick.

#### A known gap: no endpoint to create additional retry policies

Every organization gets exactly one policy, seeded at registration, and
every queue that doesn't override it inherits that default. The repository
layer already supports creating more (`Create`, `GetByID`, `ListByOrg`),
and both queue creation and job submission already accept an optional
`retry_policy_id`, but no REST endpoint issues a new policy yet. In
practice, every queue today runs the same backoff strategy. Documented
here as a known gap, not an oversight.

## Health

`GET /healthz` needs no auth and returns `200 ok` as plain text. What a
container orchestrator or load balancer hits to decide whether this
replica is still alive.
