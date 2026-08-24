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

## Role-based access control is enforced but not yet reachable

`users.role` (`owner` / `admin` / `member`) has existed in the schema and
the JWT since the start, but nothing ever actually checked it —
`FORBIDDEN` was a documented error code with no code path that returned it.
Destructive routes (`DELETE /projects/{id}`, `DELETE /queues/{id}`) now go
through `middleware.RequireRole(owner, admin)`, so a `member` gets a real
`403 FORBIDDEN` instead of silently having full access.

The honest gap: there's no invite/add-teammate endpoint yet, and
`POST /api/auth/register` always creates the registering user as `owner`.
So today, every user in every org is an `owner` — the gate is real and
tested, but a `member` account can only exist if one is inserted directly
(which is exactly how the test for this exercises it). Adding a real invite
flow that lets an `owner` add teammates at `admin`/`member` was out of
scope here — flagging it rather than building a half-finished invite UI to
check a box.

## Workers belong to exactly one organization

Workers used to be one fleet shared across every org in the database — any
worker process could claim any org's jobs, and the dashboard's worker list
leaked every org's hostnames to every other org. `workers.org_id` (added in
`000002_worker_org_scoping`) fixes both: `cmd/worker` now requires
`WORKER_ORG_ID` at startup, only discovers and claims that org's queues,
and every worker-facing endpoint filters by the requester's org.

The column is nullable at the schema level even though the application
always sets it on new rows — existing worker rows from before this
migration have no org they can be correctly backfilled to, and deleting
them would cascade-delete their `job_executions` and `job_logs` history.
They simply stop matching any org-scoped query instead, which is
equivalent to retiring them.

The real cost: this is no longer "start one worker, it serves everyone" —
each organization now needs its own worker process(es) pointed at its own
`WORKER_ORG_ID`. That's the correct trade-off for genuine multi-tenant
isolation, but it does mean the local dev setup in the README grew one
more required step.

## Every resource below Project is scoped by org_id, not just Project

`projects.org_id` was always checked — but a queue, job, scheduled job, or
dead-letter entry has no `org_id` column of its own, and every repository
method that started from one of their IDs (`GetByID`, `Retry`, `SetActive`,
`ListByQueue`, ...) trusted the ID alone. In practice that meant any
authenticated user, in any organization, could read or mutate any other
org's queues, jobs, execution logs, scheduled jobs, and dead-letter entries
just by knowing (or guessing/enumerating) a UUID — a textbook IDOR / broken
access control gap, and the one place the multi-tenancy story didn't
actually hold end-to-end.

The fix threads `orgID` from the JWT down through handler → service →
repository for every one of those lookups, resolving org ownership via a
join back to `projects` (`queues.project_id → projects.org_id`, and one
join further for jobs/scheduled jobs/dead-letter entries). A mismatched org
now returns the same `404 NOT_FOUND` as a nonexistent ID — cross-org access
is indistinguishable from "that thing doesn't exist," which is the correct
behavior for both cases.

One query needed a second look even after adding the join:
`QueueRepository.Stats` aggregates with `count(*) FILTER (...)` and no
`GROUP BY`, and an aggregate with no `GROUP BY` always returns exactly one
row — even when the `WHERE` clause matches zero rows. Scoping that query by
org directly would have kept returning `200` with all-zero counts for a
foreign queue instead of `404`. The fix follows the same pattern already
used for `DLQService.List` and `ScheduledJobService.List`: verify ownership
with `QueueRepository.GetByID` first, then run the (now org-unscoped, since
ownership is already established) aggregate.

The worker's own execution pipeline (`ExecutionService`) is deliberately
left unscoped — it has no HTTP-request org context, and a worker only ever
claims jobs from its own org's queues in the first place (see
`activeQueueIDs` in `cmd/worker`), so there's no cross-tenant path through
it to close. Those call sites go through `GetByIDInternal` /
`GetByIDInternal`-style methods, explicitly named to flag that they skip
the org check and are only safe for that trusted internal caller.

Covered by `TestRouter_CrossOrgAccess_Returns404` in `router_test.go`,
which registers two unrelated orgs and asserts a `404` for every one of
these endpoints when org B reaches for org A's queue, job, scheduled job,
or dead-letter entry — plus that org A can still reach its own resources
afterward.

## How it all fits together

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](images/architecture.png)

The full component breakdown and the job lifecycle state machine live in
[architecture.md](architecture.md).
