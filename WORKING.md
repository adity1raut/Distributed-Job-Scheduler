# How This Project Works — Distributed Job Scheduler

This document explains, in one place, what this project is, how it actually
solves job creation / delay / scheduling / concurrency scaling / retries
internally, and then walks through **verifying every one of those behaviors
yourself, locally, using the React dashboard** — no `curl` required.

It complements the shorter docs already in the repo:

| Doc | Focus |
|---|---|
| [`Readme.md`](Readme.md) | Setup commands, tech stack, test list |
| [`server/docs/architecture.md`](server/docs/architecture.md) | Component diagram, job lifecycle diagram |
| [`server/docs/design-decisions.md`](server/docs/design-decisions.md) | *Why* each trade-off was made |
| [`server/docs/api.md`](server/docs/api.md) | Every REST endpoint, request/response shapes |
| [`server/docs/er-diagram.md`](server/docs/er-diagram.md) | Database schema |
| [`ui-interface/README.md`](ui-interface/README.md) | Frontend feature list, structure |

This file is the one that ties concept → code → button-click together.

---

## Part 1 — What the system does and how

### 1.1 The one-sentence version

Two Go binaries (`cmd/api` and `cmd/worker`) share one PostgreSQL database
that acts as the queue itself — there is no separate broker (Redis, SQS,
RabbitMQ). Jobs are rows in a `jobs` table; "claiming" a job is a single
atomic SQL statement; "scaling" means running more worker processes or
raising a per-worker concurrency number; "delay" means writing a future
timestamp into `run_at` and never looking at it again until that time
passes.

### 1.2 The moving pieces

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

- **`cmd/api`** ([server/cmd/api/main.go](server/cmd/api/main.go)) — REST API:
  auth, projects, queues, jobs, scheduled jobs, DLQ, workers, dashboard
  stats. It is stateless, so you can run several replicas behind a load
  balancer with zero coordination between them.
- **`cmd/worker`** ([server/cmd/worker/main.go](server/cmd/worker/main.go)) —
  a standalone poller. It never receives HTTP requests; it just loops,
  claims jobs, executes them, and reports heartbeats.
- **PostgreSQL** is the only component that must stay consistent — every
  guarantee (no double-claim, concurrency limits, retry counts) is enforced
  by a SQL transaction, not application code.
- **Redis** is optional for correctness — it only holds cross-replica
  rate-limit counters. If it's down, the rate limiter fails *open* (see
  [server/internal/middleware/ratelimit.go](server/internal/middleware/ratelimit.go)).

### 1.3 The data model, in the order things happen

```
Organization ──▶ User (owner)
Organization ──▶ Project ──▶ Queue ──▶ Job ──▶ JobExecution ──▶ JobLog
                              │           │
                              │           └──▶ DeadLetterEntry (if all retries fail)
                              └──▶ ScheduledJob (cron definition)
```

- A `Job` row is mutated in place as it moves through its lifecycle
  (`status`, `attempts`, `run_at` all change on the same row).
- A `JobExecution` row is **never** mutated after it finishes — one row per
  attempt, so attempt 1's error is still readable after attempt 3 has run.
  This is why the Job Detail page can show a full attempt timeline instead
  of just "the last thing that happened."
- `JobLog` rows hang off the *execution*, not the job, for the same reason.

### 1.4 Job lifecycle (state machine)

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
snapped back to `queued` by the reaper (§1.8) — nothing is ever silently
lost, worst case it just runs again.

### 1.5 Creating jobs — the four submission types

Everything funnels through `JobService.Submit`
([server/internal/service/job_service.go:41](server/internal/service/job_service.go#L41)).
`type` is one of `immediate | delayed | scheduled | batch`:

| Type | Required field | What `Submit` does |
|---|---|---|
| `immediate` | — | `run_at = now()` — eligible to be claimed on the worker's very next poll |
| `delayed` | `delay_ms > 0` | `run_at = now() + delay_ms` — computed **once**, at creation time |
| `scheduled` | `run_at` (ISO 8601) | Stored as given — runs at that exact instant |
| `batch` | `batch_count >= 2` | Inserts N job rows sharing one `batch_id`; if an idempotency key is given it's suffixed per-row so the whole batch doesn't collapse into one row via the dedupe check |

Two extra behaviors worth knowing:

- **Idempotency:** `idempotency_key` is enforced by a partial unique index —
  `ON CONFLICT (queue_id, idempotency_key) WHERE idempotency_key IS NOT NULL
  DO NOTHING` in
  [job_repository.go:36](server/internal/repository/job_repository.go#L36).
  Resubmitting the same key returns the *original* row, never a duplicate.
- **`recurring` is not a submission type.** You don't submit a recurring job
  directly — you create a `ScheduledJob` (a cron definition), and the
  scheduler is what turns each cron fire into an actual `type: "recurring"`
  job row (§1.7).

### 1.6 How "delay" actually works

There's no timer, no sleep, no delayed-message feature anywhere. A delayed
job is just a normal row with a `run_at` in the future. The **only** place
that timestamp is checked is the claim query:

```sql
WHERE queue_id = $1 AND status = 'queued' AND run_at <= now()
ORDER BY priority DESC, run_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 1
```

([job_repository.go:191-197](server/internal/repository/job_repository.go#L191-L197))

So a job with `run_at` five minutes from now sits in `status = 'queued'`
completely inert — invisible to every worker's claim query — until the
clock passes that timestamp, at which point the very next poll from any
worker picks it up like any other queued job. Precision is bounded by
`WORKER_POLL_MS` (default 500 ms), not by any scheduler.

The retry backoff delay (§1.9) uses this exact same mechanism: a failed
job is requeued with `run_at = now() + backoff`, so "wait before retrying"
and "run this job later" are the same primitive.

### 1.7 How cron / recurring scheduling works

A `ScheduledJob` is a standing cron definition
(`cron_expression` + `payload_template`), not a job. The scheduler
([server/internal/scheduler/scheduler.go](server/internal/scheduler/scheduler.go))
is a goroutine started inside **every** `cmd/api` process. Every
`SCHEDULER_TICK_SEC` seconds (default 5s) it:

1. Calls `pg_try_advisory_lock(72176321)`. If another API replica already
   holds it this cycle, this replica does nothing and waits for the next
   tick — this is what stops three API replicas from firing the same cron
   job three times, with zero extra infrastructure (see
   [design-decisions.md](server/docs/design-decisions.md#the-scheduler-lives-inside-the-api-not-as-its-own-service)).
2. Fetches up to 50 `ScheduledJob`s whose `next_run_at <= now()` and are
   `is_active`.
3. For each one, inserts a new `jobs` row with `type = 'recurring'`,
   `run_at = now()`, then advances `next_run_at` using `robfig/cron`'s
   standard 5-field parser.
4. Also reaps stale claims in the same tick (§1.8).

So "every 5 minutes" (`*/5 * * * *`) means: the cron definition sits idle,
and once every 5 minutes the scheduler drops one fresh job into the queue,
which then goes through the exact same claim/execute pipeline as any other
job.

### 1.8 Fault tolerance — heartbeats and stale-job reaping

`cmd/worker` sends a heartbeat every `HEARTBEAT_SEC` (default 10s) with its
current active-job count
([worker/main.go:114-128](server/cmd/worker/main.go#L114-L128)). The
scheduler's tick also runs `ReapStale`
([job_repository.go:294-310](server/internal/repository/job_repository.go#L294-L310)):
any job stuck in `claimed`/`running` whose owning worker hasn't sent a
heartbeat inside `STALE_JOB_SEC` (default 60s) gets snapped back to
`queued`. This is what makes a crashed worker (kill -9, OOM, unplugged
network) safe — the job it was holding is picked up by someone else instead
of vanishing forever.

On graceful shutdown (`SIGTERM`), the worker stops claiming *new* jobs but
lets in-flight ones finish (up to a 30s grace window) — see
[worker/main.go:107-108](server/cmd/worker/main.go#L107-L108).

### 1.9 Retries and dead-lettering

After a failed attempt, `ExecutionService.Run`
([execution_service.go:86-102](server/internal/service/execution_service.go#L86-L102))
resolves the applicable `RetryPolicy` (per-job override → the queue's
policy → a hardcoded fallback) and asks it for the next delay:

```go
// server/internal/models/retry_policy.go
fixed:       delay = base_delay_ms
linear:      delay = base_delay_ms * attempt
exponential: delay = base_delay_ms * multiplier^(attempt-1)
             (all three clamped to max_delay_ms)
```

If `attempts < max_attempts`, the job is requeued with
`run_at = now() + delay` (using the exact delay mechanism from §1.6). Once
attempts are exhausted, the job moves to `status = dead` and a
`DeadLetterEntry` is created. From the dashboard, a dead-letter entry can be
**replayed** (`attempts` reset to 0, back to `queued` immediately) or a
job can be **manually retried** from its detail page regardless of current
status.

### 1.10 Concurrency and scaling — the part that answers "can it scale?"

There are **two independent knobs**, and it's worth being precise about
which is which:

**a) Per-queue `concurrency_limit`** — a hard ceiling on how many jobs from
*one queue* may be `claimed`/`running` at once, enforced entirely in SQL,
atomically, in `ClaimNext`
([job_repository.go:152-214](server/internal/repository/job_repository.go#L152-L214)):

```sql
SELECT concurrency_limit, is_paused FROM queues WHERE id = $1 FOR UPDATE;
-- (lock the queue row first)
SELECT count(*) FROM jobs WHERE queue_id = $1 AND status IN ('claimed','running');
-- (count in-flight jobs while holding that lock)
-- only if in_flight < concurrency_limit do we proceed to claim
```

Locking the **queue row** (not the whole table) before counting means the
count-and-claim happens as one indivisible step — two workers can never
both see "4 in flight, limit 5" and both claim, pushing it to 6. The lock is
scoped per-queue, so workers pulling from *different* queues never block
each other; only two workers racing for the *same* queue's next slot
briefly serialize, which is exactly the intended behavior. This is proven
under load by
[`job_repository_concurrency_test.go`](server/internal/repository/job_repository_concurrency_test.go)
— 50 simulated workers against 200 jobs, zero double-claims.

**b) Horizontal + vertical worker scaling** — independent of the above:

- *Vertical:* one `cmd/worker` process runs up to `WORKER_CONCURRENCY`
  (default 10) jobs at once via an in-process semaphore
  ([worker/main.go:58,77-81](server/cmd/worker/main.go#L58)).
- *Horizontal:* run more `cmd/worker` processes (that's the whole point of
  the "run multiple instances" step in the setup docs). They all poll the
  same queues and race for jobs with `SELECT ... FOR UPDATE SKIP LOCKED` —
  `SKIP LOCKED` means a worker never blocks waiting on a row another worker
  already grabbed, it just skips to the next one. So adding workers adds
  throughput almost linearly, right up until a queue's `concurrency_limit`
  caps it.

In short: **`concurrency_limit` caps how much of one queue's work can run
in parallel; the number of worker processes/threads is what actually
supplies that parallelism.** You can raise the limit but if you only have
one single-threaded worker, nothing changes — you need both. §2.7 below is
a hands-on way to see this exact interaction.

Priority also lives in this same query: `ORDER BY priority DESC, run_at
ASC` means a higher-priority job always jumps the queue ahead of an
older lower-priority one, but never ahead of one that isn't due yet.

### 1.11 Other cross-cutting behaviors

- **Auth:** `POST /api/auth/register` creates a brand-new organization and
  makes you its `owner` in one call, and seeds a default exponential
  retry policy so you can create a queue immediately
  ([api.md](server/docs/api.md#auth)). JWT is stored in `localStorage` on
  the frontend; any `401` clears it and bounces you to `/login`.
- **Rate limiting:** Redis-backed counters per org (or per IP pre-auth),
  `RATE_LIMIT_PER_MIN` (default 120/min). Fails open if Redis is down.
- **Pagination:** job listings use a keyset cursor on `(created_at, id)`,
  not `OFFSET` — stays a fast index seek no matter how deep you page.
- **Errors:** every failure is `{"error": {"code","message","request_id"}}`
  — the frontend switches on `code`, never on message text.
- **Cascading deletes:** deleting a project deletes its queues, jobs,
  executions, and logs. Intentional trade-off, documented in
  [design-decisions.md](server/docs/design-decisions.md#deletes-cascade--for-now).
- **The demo job handler:** what a job actually "does" when it runs is
  [`RunPayload`](server/internal/service/executor.go) — it reads three
  optional fields out of the job's JSON payload:

  | Payload field | Effect |
  |---|---|
  | `sleep_ms` | Sleeps that many milliseconds before finishing — use this to make a job "run" long enough to observe `running` state and concurrency limits |
  | `task: "fail"` | Always fails (deterministic — perfect for testing retries/DLQ) |
  | `fail_rate` | Fails with that probability (0.0–1.0) — useful for a mix of successes/failures |

  This is a stand-in for real application logic; a production deployment
  would swap this function for dispatch into real task code.

### 1.12 The frontend's part in all of this

The dashboard ([ui-interface/](ui-interface)) does no computation of its
own — it's a thin, polling REST client:

- Every live view uses [`usePolling`](ui-interface/src/hooks/usePolling.js)
  — fetch on mount, then again every N ms (5s for most lists, 4s for job
  detail) — no WebSocket, so "live" really means "refetches frequently."
- `src/api/*.js` — one thin fetch wrapper per resource
  (`jobs.js`, `queues.js`, `dlq.js`, `scheduledJobs.js`, `workers.js`,
  `dashboard.js`, `projects.js`, `auth.js`).
- `src/components/queue/*` — the four tabs on a queue's detail page:
  `JobSubmitForm` + `JobsTable` (Jobs tab), `ScheduledJobsPanel`,
  `DlqPanel`, `QueueConfigPanel`.
- Every mutation (create/delete/pause/resume/submit/retry/replay) shows a
  toast on success or failure, so you always get visual confirmation an
  action actually happened server-side.

---

## Part 2 — Verifying it all yourself, locally, in the UI

This section is a script you can follow start-to-finish. It assumes
Postgres, Redis, migrations, the API, and the frontend are not yet running
— if they already are, skip to §2.3.

### 2.0 Prerequisites check

```
Go 1.25+, PostgreSQL 15+, Redis 7+, Node.js 18+, golang-migrate CLI
```

### 2.1 Start the infrastructure

```bash
docker run -d --name js-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
docker run -d --name js-redis -p 6379:6379 redis:7
docker exec js-postgres createdb -U postgres jobscheduler
```

```bash
cd server
cp .env.example .env          # defaults are fine for local use
go mod tidy
migrate -path migrations -database "$DATABASE_URL" up
```

> If `$DATABASE_URL` isn't exported in your shell, either `export
> DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable`
> first, or pass the same value inline to `migrate -database`.

### 2.2 Start the API, two workers, and the frontend

Four separate terminals, all from the paths shown:

```bash
# Terminal 1 — server/
go run ./cmd/api

# Terminal 2 — server/   (worker A)
go run ./cmd/worker

# Terminal 3 — server/   (worker B — a second instance = horizontal scaling)
go run ./cmd/worker

# Terminal 4 — ui-interface/
cp .env.example .env   # VITE_API_URL=http://localhost:8080 is already correct
npm install
npm run dev
```

Open the URL Vite prints (typically `http://localhost:5173`).

Running **two** worker terminals from the start means every test below
that touches concurrency has real horizontal scaling to observe, not just
a single process.

### 2.3 Register and land on the dashboard

1. Go to `/register`. Fill in an organization name, an email, and a
   password (8+ characters) — submitting logs you straight in and lands on
   the **Overview** page.
2. Overview will show all-zero metric cards (`Projects`, `Queues`, `Online
   workers`, `Queued jobs`, etc.) — that's expected, there's nothing yet.
   `Online workers` should already read **2** — confirmation both worker
   processes registered and are heartbeating even with no jobs to run.

### 2.4 Create a project and a queue

1. Go to **Projects** → type a name (e.g. `demo`) → **Create project**.
2. Click into it → create a queue: name `emails`, priority `0`, concurrency
   `5` → **Create queue**. You land on the queue detail page with four
   tabs: **Jobs / Scheduled / Dead letters / Configuration**.

### 2.5 Verify immediate job creation end-to-end

1. On the **Jobs** tab, leave type as `immediate`, payload as
   `{"task":"echo"}` → **Submit job**. A toast confirms submission.
2. Watch the jobs table (it polls every 5s on the first page): the row
   should flip `queued` → `claimed` → `running` → `completed` within a
   couple of seconds — that's a live demonstration of the claim pipeline
   from §1.5/§1.10 actually running against your two worker processes.
3. Click the job's ID to open **Job detail**: you'll see one entry in the
   attempt timeline, `attempts: 1/5`, and one row in **Execution history**
   with a `duration_ms`. Click **View logs** to see the
   `"execution succeeded"` log line written by `ExecutionService.Run`.

### 2.6 Verify delayed jobs (the "delay" behavior from §1.6)

1. Set type to `delayed`, delay to e.g. `20000` (20s), submit.
2. Immediately check the **Run at** column in the jobs table — it shows a
   timestamp ~20s in the future, and the row's status stays `queued`.
3. Wait it out (or filter status to `queued` and watch the row disappear
   from that filter once claimed). Confirm it doesn't move to `claimed`
   before its `run_at` — that's the `run_at <= now()` guard from §1.6 doing
   its job. It will complete like any other job once its time comes.

### 2.7 Verify concurrency limits and scaling (the core "can they scale" question)

This is the most convincing one to actually watch:

1. Go to the **Configuration** tab, set **Concurrency limit** to `1`, and
   **Save**.
2. On the **Jobs** tab, submit a `batch` job with `batch_count = 5` and
   payload `{"task":"echo","sleep_ms":8000}` (8 seconds each, so they run
   long enough to observe).
3. Watch the **Live counts** metrics at the top of the Configuration tab
   (or the Jobs table filtered to `running`): even with two worker
   processes polling, **only one job is ever `running` or `claimed` at a
   time** — the rest sit `queued` even though workers are idle and able to
   pick them up. This is `concurrency_limit` from §1.10a in action.
4. Now raise **Concurrency limit** to `5` and **Save**, then submit another
   5-job batch with the same `sleep_ms` payload. This time you should see
   up to 2 running concurrently (bounded by having 2 worker processes ×
   `WORKER_CONCURRENCY=10` each — plenty of headroom now, so the queue's
   own limit of 5 is what's actually visible). Start a **third** worker
   terminal (`go run ./cmd/worker` again) mid-run and you'll see the
   running count able to climb further on the next batch — a live
   demonstration of horizontal scaling adding throughput.
5. Toggle **Pause queue**: submit one more job, confirm it sits `queued`
   forever and never claims (workers actively skip paused queues — see
   `activeQueueIDs` in [worker/main.go:130-146](server/cmd/worker/main.go#L130-L146)).
   **Resume queue** and watch it get picked up immediately.

### 2.8 Verify retries and dead-lettering

1. Submit an immediate job with payload `{"task":"fail"}` — this is a
   deterministic failure, not probabilistic (§1.11).
2. Open its Job Detail page and watch **Attempts** climb (`1/5`, `2/5`, …)
   with growing gaps between attempts — that's the exponential backoff
   from §1.9 delaying each retry. Each failed attempt gets its own node in
   the attempt timeline and its own **Execution history** row with a red
   status and a viewable error log.
3. Once attempts reach `5/5`, status flips to `dead`. A **Retry job**
   button appears on the detail page — but instead, go to the queue's
   **Dead letters** tab: you'll see the entry with its final error and a
   **Replay** button. Click it — the job goes back to `queued` with
   `attempts` reset to 0, and the entry now shows `Replayed: yes`.
4. Alternatively, click **Retry job** directly from the Job Detail page —
   same effect (`attempts` reset, requeued now), works from `failed` or
   `dead`.

### 2.9 Verify scheduled (cron) jobs

1. Go to the **Scheduled** tab. Leave the default `*/5 * * * *` or use
   something faster for testing, e.g. `* * * * *` (every minute — cron's
   granularity floor). Payload template `{"task":"echo"}` → **Add
   schedule**.
2. Note the **Next run** timestamp. Within `SCHEDULER_TICK_SEC` (5s) of
   that time passing, a new job appears on the **Jobs** tab with
   `type: recurring` — confirmation the scheduler goroutine actually fired
   (§1.7), not the UI.
3. Click **Pause** on the schedule — confirm no further jobs appear past
   the next expected fire time. **Resume** and confirm they resume.

### 2.10 Verify worker fleet monitoring and crash recovery

1. Go to **Workers** — both processes show `status: online`, a recent
   **Last heartbeat**, and `Active jobs` reflecting anything currently
   running. Click **History** on one to see its raw heartbeat log.
2. To see stale-detection and job reaping (§1.8) for real: submit a job
   with a long `sleep_ms` (e.g. 60000), then **kill the worker terminal**
   currently running it (Ctrl+C, or `kill -9` if you want to skip graceful
   shutdown entirely). Watch the **Workers** page — that worker's status
   flips toward `stale` once it misses `STALE_JOB_SEC` (60s) of
   heartbeats. Watch the job: instead of hanging in `running` forever, it
   gets reaped back to `queued` on the scheduler's next tick and the
   surviving worker picks it up and finishes it.

### 2.11 Verify the org-wide Overview page

Go back to **Overview** — after everything above, `Completed (24h)`,
`Failed (24h)`, `Dead-lettered`, and `Online workers` should all reflect
real numbers, not zeros. This confirms
`GET /api/dashboard/overview` is aggregating live data across every project
in your org, not just the one you were looking at.

### 2.12 If something doesn't behave as described

| Symptom | Likely cause |
|---|---|
| Jobs never leave `queued` | No worker running, or the queue is paused — check **Workers** page and the queue's paused badge |
| "Failed to fetch" / CORS error in browser console | `CORS_ALLOWED_ORIGINS` in `server/.env` doesn't include your Vite origin (default `http://localhost:5173`) |
| 401 immediately after login | Clock skew, or `JWT_SECRET` changed between token issue and now — log in again |
| Dashboard shows nothing at all | `VITE_API_URL` in `ui-interface/.env` doesn't point at the running API (default `http://localhost:8080`) |
| A job you expect to fail keeps succeeding | Payload typo — it must be exactly `{"task":"fail"}` (or a `fail_rate` between 0 and 1) for `RunPayload` to force a failure |

---

Once you've walked through §2.5–§2.11 you've exercised every mechanism
described in Part 1 against the real running system — not just read about
it.
