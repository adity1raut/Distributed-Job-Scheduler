# Design Decisions

Trade-offs behind the schema and reliability mechanisms in this backend,
kept close to the code they justify.

## Job claiming: Postgres `SKIP LOCKED` vs. an external queue

Workers claim jobs with `SELECT ... FOR UPDATE SKIP LOCKED` against the `jobs`
table rather than fronting Postgres with a dedicated queue (SQS, RabbitMQ,
Kafka). This keeps job state, retries, and application data in one
transactional store, at the cost of not scaling to the throughput a
purpose-built queue offers — acceptable for this system's expected volume.
Verified under real concurrency in
`internal/repository/job_repository_concurrency_test.go`: 50 simulated
workers racing for 200 jobs claim each job exactly once.

## Concurrency limits: lock the queue row, not just the job row

`ClaimNext` (`internal/repository/job_repository.go`) locks the `queues` row
with `FOR UPDATE` before counting in-flight jobs and claiming — so the
`concurrency_limit` check and the claim happen inside one serialized window
per queue. Two workers claiming from *different* queues never contend; only
claims on the *same* queue briefly serialize, which is exactly the limit's
intended scope.

## job_executions separate from jobs

A job overwrites its own `status` on every retry; a `job_executions` row
never does — each attempt gets its own row with its own start/finish time and
error. Without this split, "what happened on attempt 2" is unanswerable once
attempt 3 starts. `job_logs` hangs off the execution, not the job, for the
same reason.

## Scheduler embedded in the API process, gated by an advisory lock

The cron/recurring dispatcher (`internal/scheduler`) runs as a goroutine
inside every `cmd/api` replica rather than as a third binary — one fewer
thing to deploy. Each tick calls `pg_try_advisory_lock` first, so only one
replica's tick does work per cycle; the rest no-op. This is also the
"distributed locking" bonus item — solved with Postgres itself rather than
adding etcd/Zookeeper for a lock needed in exactly one place.

## Rate limiting: Redis, not in-process

Per-organization API rate limits are enforced with Redis counters
(`internal/middleware/ratelimit.go`) rather than in-process limiters, so
limits hold correctly across multiple API replicas. If Redis is unreachable,
the middleware fails open (allows the request) rather than taking the API
down over a non-critical dependency.

## Structured errors and keyset pagination

Every handler returns `*apperr.Error` (`internal/apperr`), rendered as
`{"error": {"code", "message", "request_id"}}` — a stable machine-readable
`code` the frontend can switch on. List endpoints (`GET .../jobs`) paginate
with an opaque cursor over `(created_at, id)` instead of `OFFSET`, so paging
deep into a large jobs table stays an index seek instead of a rescan.

## Two binaries, one `internal/` package

`cmd/api` and `cmd/worker` are separate binaries so the API and worker fleet
can be deployed and scaled independently, while sharing every repository and
model through `internal/`.

## Cascade deletes, not soft-delete

Deletes cascade down `organizations → projects → queues → jobs →
job_executions → job_logs`, since an orphaned execution log is meaningless
without its parent job. Trade-off: deleting a project destroys its audit
history. For a compliance-sensitive deployment, swap the cascade for a
`deleted_at` soft-delete on `projects`/`queues` — flagged here as the first
thing to change, not built speculatively.

## Architecture

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; a fleet of workers polls PostgreSQL to claim jobs with SELECT FOR UPDATE SKIP LOCKED and sends heartbeats.](images/architecture.png)

Full component breakdown and the job lifecycle state machine: [architecture.md](architecture.md).
