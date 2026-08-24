# Getting Started with the Dashboard

Full walkthrough for going from a fresh checkout to a project, a queue,
and a worker actually running jobs for it. See
[the frontend overview](../../ui_interface.md) for the feature list and
structure — this doc is just the step-by-step.

## 1. Install and run

```bash
npm install
cp .env.example .env   # set VITE_API_URL if the API isn't on localhost:8080
npm run dev
```

Open the URL Vite prints (default `http://localhost:5173`). Requires
Postgres, Redis, and the API already running — see the
[root setup guide](../../Readme.md#setup) for that. The dashboard has
nothing to render without a running API.

## 2. Register and start a worker

The dashboard only talks to the API, which just enqueues jobs — it
doesn't run them. A separate `cmd/worker` process is what actually claims
and executes queued jobs, and nothing in the UI runs it for you.

1. Go to `/register`, create an org (name + email + password). You land
   on **Overview**, all-zero since nothing exists yet.
2. Grab your org ID: open DevTools (F12) → Application/Storage → Local
   Storage → this page's origin → the `user` key → copy `org_id`.
3. From `server/`, start a worker for that org:
   ```bash
   WORKER_ORG_ID=<org-id> go run ./cmd/worker
   ```
   Until a worker is running for your org, submitted jobs sit in
   `queued` forever — that's expected, not a bug: there's genuinely
   nothing consuming the queue yet.

## 3. Create a project and a queue

1. **Projects** → type a name → **Create project**.
2. Click into it → create a queue (name, priority, concurrency limit) →
   **Create queue**. You land on the queue's detail page with four tabs:
   **Jobs / Scheduled / Dead letters / Configuration**.

## 4. Submit a job — the four types

On the **Jobs** tab, the submit form's type dropdown picks between four
ways of creating job row(s). All four end up going through the exact
same execution pipeline once they're due (§5) — the type only changes
*when* a job becomes eligible to run and *how many* rows get created.

| Type | Extra field you fill in | What actually happens |
|---|---|---|
| `immediate` | none | `run_at` is set to right now — eligible to be picked up on a worker's very next poll |
| `delayed` | **Delay (ms)** | `run_at = now + delay_ms`, computed the moment you hit submit, not when a worker gets to it |
| `scheduled` | **Run at** (date/time picker) | One job that runs once, at that exact instant — not to be confused with §6's recurring cron schedules |
| `batch` | **Batch count** | Creates that many separate job rows sharing one `batch_id`; each row runs, retries, and can fail independently of the others |

The payload box takes JSON. The demo worker only reacts to three
optional fields in it, useful for testing:

- `"task": "fail"` — always fails, deterministically (good for testing
  retries/dead-letters, §5)
- `"sleep_ms": 8000` — sleeps that long before finishing (good for
  watching `running` state and concurrency limits, §7)
- `"fail_rate": 0.3` — random chance (0–1) of failing each attempt

Anything else in the payload is stored and shown back on the job, but
has no effect on execution.

## 5. How a submitted job actually performs (its lifecycle)

Every job row moves through the same states, regardless of which of the
four types created it:

```
queued ──(a worker claims it)──▶ claimed ──▶ running ──▶ completed
                                                 │
                                                 └─(fails)─▶ retry, backed off, back to queued
                                                              (until attempts run out)
                                                                       │
                                                                       ▼
                                                                     dead
```

What you'll see for this in the dashboard:

- **Jobs tab / Job detail page.** Watch the row's `status` column flip
  live (it polls). Click a job's ID to open **Job detail** for the full
  attempt timeline, per-attempt execution history with durations, and a
  **View logs** link per attempt.
- **Retries.** A failed attempt doesn't just disappear — the job goes
  back to `queued` with a growing delay before the next attempt
  (exponential backoff by default), visible as `attempts: 2/5`, `3/5`,
  etc. climbing on the Job detail page.
- **Dead-lettering.** Once attempts are exhausted, status becomes `dead`
  and the job shows up on the queue's **Dead letters** tab with its
  final error. Click **Replay** there (or **Retry job** on the detail
  page) to reset attempts to 0 and requeue it immediately.

## 6. Create a recurring (scheduled/cron) job

This is different from the `scheduled` job *type* in §4, which fires
once. A **Scheduled job** (the queue's **Scheduled** tab) is a standing
cron definition that keeps producing new jobs on a repeating schedule.

1. On the queue's **Scheduled** tab, enter a cron expression (e.g.
   `*/5 * * * *` for every 5 minutes, or `* * * * *` for every minute to
   test faster) and a payload template → **Add schedule**.
2. Within `SCHEDULER_TICK_SEC` (default 5s) of the **Next run** time
   passing, a new job with `type: recurring` appears on the **Jobs** tab
   automatically — the API's scheduler goroutine dispatches it, not the
   UI. From there it goes through the exact same lifecycle as §5.
3. **Pause** / **Resume** on the schedule stops/resumes future fires
   without deleting the definition.

## 7. Concurrency — how many jobs actually run at once

Two independent things control this, both visible from the UI. This is
the most convincing one to actually watch happen:

1. Go to the queue's **Configuration** tab, set **Concurrency limit** to
   `1`, and **Save**.
2. On **Jobs**, submit a `batch` job with `batch_count = 5` and payload
   `{"task":"echo","sleep_ms":8000}` (8s each, so they run long enough
   to watch).
3. Watch the **Live counts** at the top of the Configuration tab (or the
   Jobs table filtered to `running`). Even with more than one worker
   process running, **only one job is ever `running`/`claimed` at a
   time** — the rest sit `queued` even though a worker is idle and able
   to pick them up. That's the **per-queue `Concurrency limit`** — a
   hard cap on this queue's jobs specifically, independent of how many
   workers you have.
4. Now raise **Concurrency limit** to `5`, **Save**, and submit another
   5-job batch with the same payload. You should see more running at
   once this time, bounded by how many `cmd/worker` processes you have
   (each one polls and claims independently, up to `WORKER_CONCURRENCY`
   — default 10 — jobs per process). Start another worker terminal with
   the same command from [step 2](#2-register-and-start-a-worker) mid-run
   and watch the running count able to climb further on the next batch —
   that's horizontal scaling adding throughput live.
5. Toggle **Pause queue**: submit one more job, confirm it sits `queued`
   forever and never gets claimed (workers actively skip paused
   queues). **Resume queue** and watch it picked up immediately.

**In short:** `concurrency_limit` caps how much of *one queue's* work
can run in parallel, and the number of worker processes/threads is what
actually supplies that parallelism — raising the limit with only one
worker changes nothing, you need both.

## 8. Verify worker crash recovery

1. Go to **Workers**. Running processes show `status: online`, a recent
   **Last heartbeat**, and `Active jobs`. Click **History** on one to
   see its raw heartbeat log.
2. Submit a job with a long `sleep_ms` (e.g. `60000`), then **kill the
   worker terminal** currently running it (`Ctrl+C`, or `kill -9` to
   skip graceful shutdown entirely).
3. Watch **Workers**: that worker's status flips toward `stale` once it
   misses `STALE_JOB_SEC` (default 60s) of heartbeats. Watch the job:
   instead of hanging in `running` forever, it gets reaped back to
   `queued` on the scheduler's next tick, and a surviving worker (if you
   have one running) picks it up and finishes it. If you don't have a
   second worker, it just waits in `queued` until you start one.

## 9. Verify the org-wide Overview page

Go to **Overview**. After working through the steps above, `Completed
(24h)`, `Failed (24h)`, `Dead-lettered`, and `Online workers` should all
reflect real numbers, not zeros — confirming the overview endpoint
aggregates live data across every project in your org, not just the one
you were looking at.

The **throughput chart** below the stat cards should show a bar over the
current hour with a green (completed) segment, and a red (failed)
segment stacked on top if you triggered any failures in §5. Hover a bar
for the exact counts.

## 10. Verify org-scoping and role-based access

1. Register a **second** organization (different email, different org
   name) in a private/incognito window, without starting a worker for
   it. Its **Workers** page and Overview's Worker Pool should both be
   empty — workers from your first org aren't visible here, even though
   the same Postgres instance and machine are running both.
2. While logged into that second org, open a queue or job that belongs
   to your first org by pasting its URL directly (copy an ID from your
   first org's browser tab). You should get a `404`, not the resource's
   real data and not a `403` — cross-org access is indistinguishable
   from the resource not existing at all.
3. Back in your original org, deleting a project/queue as normal still
   works, because the account that registered an org is always its
   `owner`. There's no invite flow yet to create a `member` account
   through the UI (see the
   [RBAC design note](../../server/docs/design-decisions.md#role-based-access-control-is-enforced-but-not-yet-reachable)),
   so the `403` path for a non-owner isn't click-through-able today —
   it's covered by automated tests instead:
   ```bash
   export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable"
   go test ./internal/handler/... -run TestRouter_DeleteProject_RoleGate -v
   go test ./internal/handler/... -run TestRouter_CrossOrgAccess_Returns404 -v
   ```
   (run from `server/`; point `TEST_DATABASE_URL` at a throwaway
   database if you don't want test fixtures landing in your real one)

For the deeper mechanics behind all of this (why `SELECT ... FOR UPDATE
SKIP LOCKED`, how backoff is computed, the advisory-lock scheduler), see
the [backend architecture doc](../../server/docs/architecture.md).

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Jobs never leave `queued` | No worker running, or the queue is paused: check **Workers** page and the queue's paused badge |
| "Failed to fetch" / CORS error in browser console | `CORS_ALLOWED_ORIGINS` in `server/.env` doesn't include your Vite origin (default `http://localhost:5173`) |
| 401 immediately after login | Clock skew, or `JWT_SECRET` changed between token issue and now; log in again |
| Dashboard shows nothing at all | `VITE_API_URL` in `ui-interface/.env` doesn't point at the running API (default `http://localhost:8080`) |
| A job you expect to fail keeps succeeding | Payload typo: it must be exactly `{"task":"fail"}` (or a `fail_rate` between 0 and 1) for `RunPayload` to force a failure |

## Running more than one org at once

Each `cmd/worker` process is scoped to exactly **one** org via
`WORKER_ORG_ID` — deliberate multi-tenant isolation (see the
[worker org-scoping design note](../../server/docs/design-decisions.md#workers-belong-to-exactly-one-organization)),
not a limitation to work around. If you register a second org and want
its jobs to run too, repeat the [step 2](#2-register-and-start-a-worker)
worker command in another terminal with that org's ID instead — one
terminal per org, each running alongside the others with no conflict.

One `cmd/api` instance already serves every org — only `cmd/worker` is
org-scoped, so you never need a second API process, just one worker
terminal per org.
