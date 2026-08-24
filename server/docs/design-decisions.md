# Design Decisions

A handful of choices in this backend aren't obvious from reading the code
alone, so here's the reasoning behind them.

## Why Postgres claims jobs instead of a dedicated queue

Workers grab jobs directly from the `jobs` table. Postgres itself is the
queue; nothing else sits in front of it.

The upside: job state, retry history, and the rest of the application
data all live in one transactional store, so there's nothing to keep in
sync across two systems. The downside is throughput. A queue built for
exactly this purpose will out-scale it eventually. For what this system
needs to handle, that ceiling is nowhere close, so it wasn't worth the
extra moving part.

This isn't just a claim on paper. A concurrency test simulates 50 workers
racing for 200 jobs and checks that every job gets claimed exactly once.

## Concurrency limits lock the queue, not just the job

Claiming a job and checking the queue's concurrency limit happen as one
atomic step, not two separate checks that could race each other.

The nice side effect: workers pulling from *different* queues never
block each other. Only two workers hitting the *same* queue at the same
moment briefly wait on one another, which is exactly what a concurrency
limit is supposed to do anyway.

## Executions get their own table

A job's own row overwrites its own status every time it's retried. If
that were the only record kept, there'd be no way to answer "what
actually happened on attempt 2" once attempt 3 is underway. It would
just be gone.

A separate execution table fixes that: each attempt writes its own row,
with its own start time, finish time, and error. Logs hang off the
execution, not the job, for the same reason: they belong to one specific
attempt.

## The scheduler lives inside the API, not as its own service

Cron dispatch is just a background routine running inside every API
replica, which is one less thing to build, deploy, and monitor. The
obvious problem: if you run three API replicas, you don't want three
schedulers firing the same cron job three times.

The fix is a database-level lock. Each tick, whichever replica grabs it
first does the work that cycle, and the rest just skip and try again next
tick. It's a distributed lock with zero extra infrastructure, since
Postgres is already sitting right there.

## Rate limiting through Redis, not memory

If rate limits lived in each API process's memory, three replicas would
mean three separate quotas, and the real limit would effectively triple.
Redis counters keep it honest across replicas.

One deliberate compromise: if Redis goes down, requests are allowed
through rather than rejected. Losing rate limiting for a bit is a much
smaller problem than the whole API going down because of it.

## Structured errors and cursor-based pagination

Every error response comes back in the same shape, from every handler,
no exceptions, so the frontend can switch on an error code instead of
trying to pattern-match error strings.

Job listings paginate with a cursor instead of the usual page-number
offset. It's a small thing until the jobs table has a few million rows,
at which point offset-based paging gets slower the deeper you page, and a
cursor doesn't.

## Two binaries, one shared package

The API and the worker build separately so they can scale independently.
More API traffic doesn't mean you need more workers, and vice versa.
They still share the same models and data-access code, so there's no
duplicated logic between them.

## Deletes cascade, for now

Delete a project and everything under it goes: queues, jobs, executions,
logs. It's the simple option, and it's correct for what this brief asks
for.

The honest trade-off: it also deletes the audit trail along with the
data. If this were going into a setting where you need to keep records
after someone deletes a project (compliance, billing disputes, that kind
of thing), the fix is marking rows as deleted instead of actually
removing them. Flagging it here rather than building it, since nothing
in the brief calls for it yet.

## Role-based access control is enforced but not yet reachable

User roles (`owner` / `admin` / `member`) have existed since the start,
but nothing ever actually checked them. Destructive actions like
deleting a project or a queue now require an `owner` or `admin` role, so
a `member` gets a real permission error instead of silently having full
access.

The honest gap: there's no invite/add-teammate flow yet, and registering
always creates the registering user as `owner`. So today, every user in
every org is an `owner`. The permission check is real and tested, but a
`member` account can only exist if one is inserted directly into the
database. Adding a real invite flow that lets an owner add teammates at
lower roles was out of scope here. Flagging it rather than building a
half-finished invite UI to check a box.

## Workers belong to exactly one organization

Workers used to be one fleet shared across every organization in the
database: any worker process could claim any org's jobs, and the
dashboard's worker list leaked every org's hostnames to every other org.
Now a worker must be started with an organization ID and only ever
discovers and claims that org's queues, and every worker-facing endpoint
filters by the requester's org.

The real cost: this is no longer "start one worker, it serves everyone."
Each organization now needs its own worker process(es). That's the
correct trade-off for genuine multi-tenant isolation, but it does mean
the local dev setup grew one more required step.

## Every resource below a project is scoped by organization, not just the project

The organization check on a project was always there, but a queue, job,
scheduled job, or dead-letter entry had no organization of its own, and
every lookup that started from one of those IDs trusted the ID alone. In
practice that meant any authenticated user, in any organization, could
read or mutate any other org's queues, jobs, execution logs, scheduled
jobs, and dead-letter entries just by knowing (or guessing) an ID. A
textbook broken-access-control gap, and the one place the multi-tenancy
story didn't actually hold end-to-end.

The fix threads the caller's organization down through every one of
those lookups, resolving ownership by tracing back to the parent project.
A mismatched organization now returns the same "not found" response as a
nonexistent ID: cross-org access is indistinguishable from "that thing
doesn't exist," which is the correct behavior for both cases.

The worker's own execution pipeline is deliberately left unscoped. It has
no request context to scope by, and a worker only ever claims jobs from
its own organization's queues in the first place, so there's no
cross-tenant path through it to close.

A test covers this directly: it registers two unrelated organizations and
confirms one gets a "not found" response reaching for the other's queue,
job, scheduled job, or dead-letter entry, while still being able to reach
its own resources.

## How it all fits together

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](images/architecture.png)

The full component breakdown and the job lifecycle state machine live in
[architecture.md](architecture.md).
