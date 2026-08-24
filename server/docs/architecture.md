# Architecture

Two Go binaries (`cmd/api` and `cmd/worker`) share one PostgreSQL
database that acts as the queue itself. There is no separate broker: no
Redis queue, no SQS, no RabbitMQ. Jobs are rows in a `jobs` table.
Claiming a job is a single atomic SQL statement. Scaling means running
more worker processes or raising a per-worker concurrency number. Delay
means writing a future timestamp into `run_at` and never looking at it
again until that time passes.

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](images/architecture.png)

No message broker, no external queue. Postgres is what every piece
agrees through. Redis is a soft cache for rate limits, not required for
correctness (see below). Two example org worker fleets are shown because
that's the part that's easy to miss: a worker belongs to exactly one
organization and only ever sees that org's queues. "Add more workers"
always means "for a specific org," never a global capacity dial. See
[design-decisions.md](design-decisions.md#workers-belong-to-exactly-one-organization).

## The moving pieces

```
React dashboard (ui-interface)
        │  HTTPS + JWT, polls every 5s
        ▼
cmd/api  (stateless REST server)  ──checks──▶  Redis (rate-limit counters)
        │  reads/writes
        ▼
PostgreSQL  ◀──polls, claims, heartbeats──  cmd/worker (N processes)
        ▲
        │  advisory lock, one winner per tick
   scheduler goroutine (lives inside every cmd/api replica)
```

- **`cmd/api`** ([../cmd/api/main.go](../cmd/api/main.go)) is the REST
  server: auth, projects, queues, jobs, scheduled jobs, DLQ, workers,
  dashboard stats — the data the dashboard reads. It's stateless, so it
  scales behind a load balancer with zero coordination needed between
  replicas. CORS is locked down to `CORS_ALLOWED_ORIGINS` rather than
  left wide open.
- **The scheduler** isn't a separate service. It's a goroutine running
  inside every `cmd/api` replica ([../internal/scheduler/scheduler.go](../internal/scheduler/scheduler.go)).
  It wakes up every `SCHEDULER_TICK_SEC` seconds, grabs a Postgres
  advisory lock first, and only does real work if it gets the lock —
  that's what stops a cron job from firing three times just because
  you're running three API replicas. The same tick also reaps jobs
  stuck in `claimed`/`running` whose worker has gone quiet (see
  [Fault tolerance](#fault-tolerance--heartbeats-and-stale-job-reaping)
  below).
- **`cmd/worker`** ([../cmd/worker/main.go](../cmd/worker/main.go)): a
  standalone poller, never receives HTTP requests. Each instance is
  started with a required `WORKER_ORG_ID` and only polls that
  organization's non-paused queues, atomically claims the next runnable
  job, runs it, and sends heartbeats while it works. On `SIGTERM` it
  stops claiming new jobs but lets whatever's already running finish
  before it exits.
- **PostgreSQL** is the only component that must stay consistent. Every
  guarantee (no double-claim, concurrency limits, retry counts) is
  enforced by a SQL transaction, not application code. Why that's the
  design instead of a dedicated queue is explained in
  [design-decisions.md](design-decisions.md).
- **Redis** is optional for correctness. It only holds cross-replica
  rate-limit counters
  ([../internal/middleware/ratelimit.go](../internal/middleware/ratelimit.go)),
  and if it's down the rate limiter fails *open* — losing a soft limit
  for a while beats an outage.

## The data model, in the order things happen

```
Organization ──▶ User (owner)
Organization ──▶ Project ──▶ Queue ──▶ Job ──▶ JobExecution ──▶ JobLog
                              │           │
                              │           └──▶ DeadLetterEntry (if all retries fail)
                              └──▶ ScheduledJob (cron definition)
```

- A `Job` row is mutated in place as it moves through its lifecycle:
  `status`, `attempts`, and `run_at` all change on the same row.
- A `JobExecution` row is **never** mutated after it finishes. One row
  per attempt, so attempt 1's error is still readable after attempt 3
  has run. This is why the Job Detail page can show a full attempt
  timeline instead of just "the last thing that happened."
- `JobLog` rows hang off the *execution*, not the job, for the same
  reason.

The full schema (keys, indexes, cascade behavior) is in
[er-diagram.md](er-diagram.md); the REST endpoints that drive it are in
[api.md](api.md).

## Job lifecycle

![Job lifecycle state diagram: a job starts at Scheduled or Queued, moves to Claimed via an atomic SKIP LOCKED select, then Running; from Running it either completes, retries back to Queued with a backoff delay, or moves to Dead once retries are exhausted; a Claimed job with no heartbeat is reaped back to Queued.](images/job-scheduling.png)

```
scheduled/queued ──(ClaimNext, SKIP LOCKED)──▶ claimed ──▶ running
                                                                │
                                        ┌── success ────────────┤
                                        │                       │
                                        ▼                       ▼
                                   completed          fail → retry?
                                                          │        │
                                                    yes (backoff)  no
                                                          │        │
                                                          ▼        ▼
                                                       queued     dead → DeadLetterEntry
```

A `claimed`/`running` job whose worker stops sending heartbeats gets
snapped back to `queued` by the reaper (see
[Fault tolerance](#fault-tolerance--heartbeats-and-stale-job-reaping)).
Nothing is ever silently lost; worst case it just runs again.

## Creating jobs — the four submission types

Everything funnels through `JobService.Submit`
([../internal/service/job_service.go:47](../internal/service/job_service.go#L47)).
`type` is one of `immediate | delayed | scheduled | batch`:

| Type | Required field | What `Submit` does |
|---|---|---|
| `immediate` | — | `run_at = now()`, eligible to be claimed on the worker's very next poll |
| `delayed` | `delay_ms > 0` | `run_at = now() + delay_ms`, computed **once**, at creation time |
| `scheduled` | `run_at` (ISO 8601) | Stored as given, runs at that exact instant |
| `batch` | `batch_count >= 2` | Inserts N job rows sharing one `batch_id`. If an idempotency key is given it's suffixed per-row, so the whole batch doesn't collapse into one row via the dedupe check. |

Two extra behaviors worth knowing:

- **Idempotency.** `idempotency_key` is enforced by a partial unique
  index, `ON CONFLICT (queue_id, idempotency_key) WHERE idempotency_key
  IS NOT NULL DO NOTHING` in
  [../internal/repository/job_repository.go:46](../internal/repository/job_repository.go#L46).
  Resubmitting the same key returns the *original* row, never a
  duplicate.
- **`recurring` is not a submission type.** You don't submit a recurring
  job directly. You create a `ScheduledJob` (a cron definition), and the
  scheduler is what turns each cron fire into an actual `type:
  "recurring"` job row (see
  [Cron/recurring scheduling](#how-croncurring-scheduling-works)).

## How "delay" actually works

There's no timer, no sleep, no delayed-message feature anywhere. A
delayed job is just a normal row with a `run_at` in the future. The
**only** place that timestamp gets checked is the claim query:

```sql
WHERE queue_id = $1 AND status = 'queued' AND run_at <= now()
ORDER BY priority DESC, run_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 1
```

([../internal/repository/job_repository.go:206-212](../internal/repository/job_repository.go#L206-L212))

So a job with `run_at` five minutes from now sits in `status = 'queued'`
completely inert, invisible to every worker's claim query, until the
clock passes that timestamp. At that point the very next poll from any
worker picks it up like any other queued job. Precision is bounded by
`WORKER_POLL_MS` (default 500 ms), not by any scheduler.

The retry backoff delay (see
[Retries and dead-lettering](#retries-and-dead-lettering)) uses this
exact same mechanism: a failed job is requeued with `run_at = now() +
backoff`, so "wait before retrying" and "run this job later" are the
same primitive.

## How cron/recurring scheduling works

A `ScheduledJob` is a standing cron definition (`cron_expression` +
`payload_template`), not a job. The scheduler
([../internal/scheduler/scheduler.go](../internal/scheduler/scheduler.go))
is a goroutine started inside **every** `cmd/api` process. Every
`SCHEDULER_TICK_SEC` seconds (default 5s) it:

1. Calls `pg_try_advisory_lock(72176321)`. If another API replica
   already holds it this cycle, this replica does nothing and waits for
   the next tick. This is what stops three API replicas from firing the
   same cron job three times, with zero extra infrastructure (see
   [design-decisions.md](design-decisions.md#the-scheduler-lives-inside-the-api-not-as-its-own-service)).
2. Fetches up to 50 `ScheduledJob`s whose `next_run_at <= now()` and are
   `is_active`.
3. For each one, inserts a new `jobs` row with `type = 'recurring'`,
   `run_at = now()`, then advances `next_run_at` using `robfig/cron`'s
   standard 5-field parser.
4. Also reaps stale claims in the same tick (see
   [Fault tolerance](#fault-tolerance--heartbeats-and-stale-job-reaping)).

So "every 5 minutes" (`*/5 * * * *`) means the cron definition sits idle,
and once every 5 minutes the scheduler drops one fresh job into the
queue, which then goes through the exact same claim/execute pipeline as
any other job.

## Fault tolerance — heartbeats and stale-job reaping

`cmd/worker` sends a heartbeat every `HEARTBEAT_SEC` (default 10s) with
its current active-job count
([../cmd/worker/main.go:120-134](../cmd/worker/main.go#L120-L134)). The
scheduler's tick also runs `ReapStale`
([../internal/repository/job_repository.go:314-333](../internal/repository/job_repository.go#L314-L333)):
any job stuck in `claimed`/`running` whose owning worker hasn't sent a
heartbeat inside `STALE_JOB_SEC` (default 60s) gets snapped back to
`queued`. This is what makes a crashed worker (kill -9, OOM, unplugged
network) safe: the job it was holding is picked up by someone else
instead of vanishing forever.

On graceful shutdown (`SIGTERM`), the worker stops claiming *new* jobs
but lets in-flight ones finish, up to a 30s grace window. See
[../cmd/worker/main.go:106-108](../cmd/worker/main.go#L106-L108).

## Retries and dead-lettering

After a failed attempt, `ExecutionService.Run`
([../internal/service/execution_service.go:86-102](../internal/service/execution_service.go#L86-L102))
resolves the applicable `RetryPolicy` (per-job override, then the
queue's policy, then a hardcoded fallback) and asks it for the next
delay:

```go
// internal/models/retry_policy.go
fixed:       delay = base_delay_ms
linear:      delay = base_delay_ms * attempt
exponential: delay = base_delay_ms * multiplier^(attempt-1)
             (all three clamped to max_delay_ms)
```

If `attempts < max_attempts`, the job is requeued with
`run_at = now() + delay`, using the exact delay mechanism above. Once
attempts are exhausted, the job moves to `status = dead` and a
`DeadLetterEntry` is created. From the dashboard, a dead-letter entry can
be **replayed** (`attempts` reset to 0, back to `queued` immediately), or
a job can be **manually retried** from its detail page regardless of
current status.

## Concurrency and scaling — the part that answers "can it scale?"

There are **two independent knobs**, and it's worth being precise about
which is which.

**a) Per-queue `concurrency_limit`.** A hard ceiling on how many jobs
from *one queue* may be `claimed`/`running` at once, enforced entirely in
SQL, atomically, in `ClaimNext`
([../internal/repository/job_repository.go:167-229](../internal/repository/job_repository.go#L167-L229)):

```sql
SELECT concurrency_limit, is_paused FROM queues WHERE id = $1 FOR UPDATE;
-- (lock the queue row first)
SELECT count(*) FROM jobs WHERE queue_id = $1 AND status IN ('claimed','running');
-- (count in-flight jobs while holding that lock)
-- only if in_flight < concurrency_limit do we proceed to claim
```

Locking the **queue row** (not the whole table) before counting means
the count-and-claim happens as one indivisible step: two workers can
never both see "4 in flight, limit 5" and both claim, pushing it to 6.
The lock is scoped per-queue, so workers pulling from *different* queues
never block each other. Only two workers racing for the *same* queue's
next slot briefly serialize, which is exactly the intended behavior.
This is proven under load by
[`job_repository_concurrency_test.go`](../internal/repository/job_repository_concurrency_test.go):
50 simulated workers against 200 jobs, zero double-claims.

**b) Horizontal + vertical worker scaling.** Independent of the above:

- *Vertical:* one `cmd/worker` process runs up to `WORKER_CONCURRENCY`
  (default 10) jobs at once via an in-process semaphore
  ([../cmd/worker/main.go:64](../cmd/worker/main.go#L64)).
- *Horizontal:* run more `cmd/worker` processes. They all poll the same
  queues and race for jobs with `SELECT ... FOR UPDATE SKIP LOCKED`,
  where `SKIP LOCKED` means a worker never blocks waiting on a row
  another worker already grabbed; it just skips to the next one. So
  adding workers adds throughput almost linearly, right up until a
  queue's `concurrency_limit` caps it.

In short: **`concurrency_limit` caps how much of one queue's work can run
in parallel, and the number of worker processes/threads is what actually
supplies that parallelism.** You can raise the limit but if you only have
one single-threaded worker, nothing changes. You need both — see
[`ui-interface/docs/getting-started.md`](../../ui-interface/docs/getting-started.md#7-concurrency--how-many-jobs-actually-run-at-once)
for a hands-on way to watch this interaction from the dashboard.

Priority also lives in this same query: `ORDER BY priority DESC, run_at
ASC` means a higher-priority job always jumps the queue ahead of an
older lower-priority one, but never ahead of one that isn't due yet.

**c) Scaling is per-organization, not global.** A worker belongs to
exactly one org (`WORKER_ORG_ID`, required at startup) and only
discovers and claims that org's queues. See
[design-decisions.md](design-decisions.md#workers-belong-to-exactly-one-organization).
So "add more workers" really means "add more workers *for the org whose
throughput you're trying to raise*." A busy org doesn't silently borrow
capacity from, or steal capacity from, anyone else's fleet.

## Other cross-cutting behaviors

- **Auth.** `POST /api/auth/register` creates a brand-new organization
  and makes you its `owner` in one call, and seeds a default exponential
  retry policy so you can create a queue immediately
  ([api.md](api.md#auth)). JWT is stored in `localStorage` on the
  frontend; any `401` clears it and bounces you to `/login`.
- **Rate limiting.** Redis-backed counters per org (or per IP pre-auth),
  `RATE_LIMIT_PER_MIN` (default 120/min). Fails open if Redis is down.
- **Pagination.** Job listings use a keyset cursor on `(created_at, id)`,
  not `OFFSET`, so it stays a fast index seek no matter how deep you
  page.
- **Errors.** Every failure is `{"error": {"code","message","request_id"}}`.
  The frontend switches on `code`, never on message text.
- **Cascading deletes.** Deleting a project deletes its queues, jobs,
  executions, and logs. This is an intentional trade-off, documented in
  [design-decisions.md](design-decisions.md#deletes-cascade-for-now).
- **Role-based access control.** `DELETE /projects/{id}` and
  `DELETE /queues/{id}` require `owner` or `admin`. A `member` gets a
  real `403 FORBIDDEN` via `middleware.RequireRole`. There's no invite
  endpoint yet, so every account created through `/auth/register` is
  `owner`. See
  [design-decisions.md](design-decisions.md#role-based-access-control-is-enforced-but-not-yet-reachable)
  for the honest gap.
- **Org-scoped access.** Every resource below Project (queues, jobs,
  scheduled jobs, dead-letter entries) is scoped by the caller's org via
  a join back to `projects`. Reaching for another org's resource by ID
  returns `404`, identical to a nonexistent one. See
  [design-decisions.md](design-decisions.md#every-resource-below-project-is-scoped-by-org_id-not-just-project).
- **Throughput visualization.** `GET /api/dashboard/throughput` buckets
  completed/failed jobs by hour for the last 24h (zero-filled, so the
  chart always has a continuous strip), rendered as a stacked bar chart
  on the Overview page instead of just the two rolled-up 24h counters.
- **The demo job handler.** What a job actually "does" when it runs is
  [`RunPayload`](../internal/service/executor.go). It reads three
  optional fields out of the job's JSON payload:

  | Payload field | Effect |
  |---|---|
  | `sleep_ms` | Sleeps that many milliseconds before finishing, useful for making a job "run" long enough to observe `running` state and concurrency limits |
  | `task: "fail"` | Always fails deterministically, useful for testing retries/DLQ |
  | `fail_rate` | Fails with that probability (0.0–1.0), useful for a mix of successes/failures |

  This stands in for real application logic. A production deployment
  would swap this function for dispatch into real task code.
