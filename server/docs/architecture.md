# Architecture

Two Go binaries (`cmd/api` and `cmd/worker`) share one PostgreSQL database
that acts as the queue itself. There is no separate broker, no Redis
queue, no SQS, no RabbitMQ. Jobs are rows in a `jobs` table; claiming one
is a single atomic step. Scaling means running more worker processes or
raising a per-worker concurrency number.

## System Diagram

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](images/architecture.png)

PostgreSQL is what every piece agrees through. Redis is a soft cache for
rate limits, not required for correctness. A worker belongs to exactly
one organization and only ever sees that org's queues: "add more
workers" always means "for a specific org," never a global capacity dial
(see **[design-decisions.md](design-decisions.md#workers-belong-to-exactly-one-organization)**).

## The moving pieces

- **`cmd/api`**: the REST server: auth, projects, queues, jobs, scheduled
  jobs, DLQ, workers, dashboard stats. Stateless, so it scales behind a
  load balancer with no coordination between replicas.
- **The scheduler**: a background routine inside every `cmd/api`
  replica, not a separate service. It wakes up on a timer, grabs a
  database lock first, and only does real work if it gets it, which is
  what stops a cron job from firing three times just because three API
  replicas are running. The same tick reaps stale claims (see
  **[Reliability](#reliability-heartbeats-retries-and-dead-lettering)**).
- **`cmd/worker`**: a standalone poller. Each instance starts scoped to
  one organization, polls only that organization's non-paused queues,
  atomically claims the next runnable job, runs it, and sends
  heartbeats. On shutdown it stops claiming new jobs but lets whatever's
  running finish first.
- **PostgreSQL** is the only component that must stay consistent. Every
  guarantee (no double-claim, concurrency limits, retry counts) is
  enforced by the database itself, not application code (see
  **[design-decisions.md](design-decisions.md)** for why that's the
  design instead of a dedicated queue).
- **Redis** is optional for correctness. It only holds cross-replica
  rate-limit counters, and fails *open* if it's down.

Data flows in one direction: an organization owns projects, a project
owns queues, a queue holds jobs, and each job produces one or more
executions as it's attempted. A job that exhausts its retries also
produces a dead-letter entry, and a queue can define scheduled (cron)
jobs that spawn new jobs on a repeating schedule.

A job's own row is mutated in place as it moves through its lifecycle,
but each attempt keeps its own execution record that's never changed
afterward, which is why the Job Detail page can show a full attempt
timeline instead of just the last thing that happened. Full schema, keys,
and cascade behavior: **[er-diagram.md](er-diagram.md)**.

## Job Lifecycle

![Job lifecycle state diagram: a job starts at Scheduled or Queued, moves to Claimed via an atomic SKIP LOCKED select, then Running; from Running it either completes, retries back to Queued with a backoff delay, or moves to Dead once retries are exhausted; a Claimed job with no heartbeat is reaped back to Queued.](images/job-scheduling.png)

A job is claimed atomically, runs, and either completes, gets requeued
with a backoff delay, or, once retries are exhausted, moves to `dead` and
produces a dead-letter entry. A `claimed`/`running` job whose worker
stops sending heartbeats gets snapped back to `queued` by the reaper.
Nothing is ever silently lost; worst case it just runs again.

## Creating jobs: the four submission types

`type` is one of `immediate | delayed | scheduled | batch`:

| Type | Required field | What happens |
|---|---|---|
| `immediate` | none | Eligible on the worker's next poll |
| `delayed` | a delay in ms | Runs after that delay, computed once, at creation time |
| `scheduled` | a run-at timestamp | Runs at that exact instant |
| `batch` | a batch count | Inserts N job rows sharing one batch ID |

There is no timer or sleep behind any of these. A delayed or scheduled
job is just a row that isn't due yet, and the claim query simply skips
anything not due. `recurring` isn't a submission type here: it's produced
by the scheduler each time a cron definition fires.

## Reliability: heartbeats, retries, and dead-lettering

This is the part of the system that answers "what happens when something
goes wrong":

- **Crash recovery.** Each worker sends a heartbeat on a short interval.
  Any job stuck as claimed or running whose worker hasn't heartbeat
  recently is reaped back to `queued`, so a crashed worker's job is
  picked up by someone else instead of vanishing.
- **Retries.** After a failed attempt, the applicable retry policy
  (per-job override, then the queue's default, then a fallback) decides
  the next delay using fixed, linear, or exponential backoff. If
  attempts remain, the job is requeued after that delay. Once exhausted,
  it moves to `dead` and a dead-letter entry is created. It's replayable
  from the dashboard (attempts reset to zero) or manually retryable from
  any status.

## Concurrency and scaling

There are **two independent knobs**:

**a) Per-queue concurrency limit**, a hard ceiling on how many jobs from
*one queue* may be claimed or running at once. Checking that limit and
claiming the next job happen as a single atomic step, so two workers can
never both see "under the limit" and both claim, pushing it over. The
lock is scoped per-queue, so workers pulling from *different* queues
never block each other. Proven under load by a dedicated concurrency
test: 50 simulated workers against 200 jobs, zero double-claims.

**b) Worker count**: vertically, one worker process runs several jobs at
once up to a configurable concurrency setting. Horizontally, running
more worker processes adds throughput almost linearly, since they never
block on a job another worker already grabbed, up to a queue's
concurrency limit.

The concurrency limit caps how much of one queue's work can run in
parallel; the number of worker processes/threads is what actually
supplies that parallelism; you need both. Scaling is also
per-organization, not global: a worker belongs to exactly one org and
only claims that org's queues, so a busy org never borrows or steals
capacity from anyone else's fleet.

---

Cross-cutting behaviors not specific to job execution (auth, rate
limiting, pagination, RBAC, cascading deletes, org-scoped access) are
covered in **[api.md](api.md)** and **[design-decisions.md](design-decisions.md)**
rather than repeated here.
