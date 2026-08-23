# Design Decisions

A handful of choices in this backend aren't obvious from reading the code
alone, so here's the reasoning behind them.

## Why Postgres claims jobs instead of a dedicated queue

Workers grab jobs with a single `SELECT ... FOR UPDATE SKIP LOCKED` against
the `jobs` table — Postgres itself is the queue, nothing else sits in front
of it.

The upside: job state, retry history, and the rest of the application data
all live in one transactional store, so there's nothing to keep in sync
across two systems. The downside is throughput — a queue built for exactly
this purpose will out-scale it eventually. For what this system needs to
handle, that ceiling is nowhere close, so it wasn't worth the extra moving
part.

This isn't just a claim on paper — `job_repository_concurrency_test.go` runs
50 simulated workers against 200 jobs and checks that every job gets claimed
exactly once.

## Concurrency limits lock the queue, not just the job

`ClaimNext` locks the `queues` row itself (`FOR UPDATE`) before it counts
in-flight jobs and claims the next one. That means the concurrency check and
the claim happen as one atomic step per queue.

The nice side effect: workers pulling from *different* queues never block
each other. Only two workers hitting the *same* queue at the same moment
briefly wait on one another — which is exactly what a concurrency limit is
supposed to do anyway.

## Executions get their own table

A `jobs` row overwrites its own `status` every time it's retried. If that
were the only record kept, there'd be no way to answer "what actually
happened on attempt 2" once attempt 3 is underway — it's just gone.

`job_executions` fixes that: each attempt writes a new row with its own
start time, finish time, and error. `job_logs` hangs off the execution
(not the job) for the same reason — logs belong to one specific attempt.

## The scheduler lives inside the API, not as its own service

Cron dispatch is just a goroutine running inside every `cmd/api` replica —
one less binary to build, deploy, and monitor. The obvious problem with that
is if you run three API replicas, you don't want three schedulers firing the
same cron job three times.

The fix is a Postgres advisory lock. Each tick, the goroutine calls
`pg_try_advisory_lock` first; whichever replica gets it does the work that
cycle, the rest just skip and try again next tick. It's a distributed lock
with zero extra infrastructure — Postgres already happens to be sitting
right there.

## Rate limiting through Redis, not memory

If rate limits lived in each API process's memory, three replicas would mean
three separate quotas, and the real limit would effectively triple. Redis
counters keep it honest across replicas.

One deliberate compromise: if Redis goes down, requests are allowed through
rather than rejected. Losing rate limiting for a bit is a much smaller
problem than the whole API going down because of it.

## Structured errors and cursor-based pagination

Every error response comes back as `{"error": {"code", "message",
"request_id"}}`, from every handler, no exceptions. The frontend can switch
on `code` instead of trying to pattern-match error strings.

Job listings paginate with a cursor over `(created_at, id)` instead of the
usual `OFFSET`. It's a small thing until the `jobs` table has a few million
rows — `OFFSET` gets slower the deeper you page, a cursor doesn't.

## Two binaries, one shared package

`cmd/api` and `cmd/worker` build separately so they can scale independently
— more API traffic doesn't mean you need more workers, and vice versa. They
still share every model and repository through `internal/`, so there's no
duplicated logic between them.

## Deletes cascade — for now

Delete a project and everything under it goes: queues, jobs, executions,
logs. It's the simple option, and it's correct for what this brief asks for.

The honest trade-off: it also deletes the audit trail along with the data.
If this were going into a setting where you need to keep records after
someone deletes a project — compliance, billing disputes, that kind of thing
— the fix is a `deleted_at` column instead of a real delete. Flagging it here
rather than building it, since nothing in the brief calls for it yet.

## How it all fits together

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; a fleet of workers polls PostgreSQL to claim jobs with SELECT FOR UPDATE SKIP LOCKED and sends heartbeats.](images/architecture.png)

The full component breakdown and the job lifecycle state machine live in
[architecture.md](architecture.md).
