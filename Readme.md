# Distributed Job Scheduler

A production-inspired distributed job scheduling platform for reliably executing asynchronous background jobs across multiple workers — with authentication, project/queue management, retries, dead-letter handling, and a live dashboard.

## Contents

- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Dashboard](#dashboard)
- [Project Structure](#project-structure)
- [Setup](#setup)
- [Testing](#testing)
- [Verifying the Frontend UI](#verifying-the-frontend-ui)
- [Rolling Back Migrations](#rolling-back-migrations)
- [Documentation](#documentation)

## Tech Stack

| Layer | Technology |
|---|---|
| Backend API + Worker | Go |
| Frontend | React |
| Database | PostgreSQL (via pgx/v5 + pgxpool) |
| Rate Limiting | Redis |
| Migrations | golang-migrate |
| Router | chi |
| Auth | JWT (golang-jwt) + bcrypt |
| Cron parsing | robfig/cron |
| Distributed locking | Postgres advisory locks (scheduler leader election) |

## Architecture

Two independent Go binaries sharing a common `internal/` package:

- **`cmd/api`** — REST API server (auth, projects, queues, jobs, dashboard data)
- **`cmd/worker`** — belongs to exactly one org (`WORKER_ORG_ID`), polls that org's queues, atomically claims jobs (`SELECT ... FOR UPDATE SKIP LOCKED`), executes them concurrently, sends heartbeats, and shuts down gracefully on `SIGTERM`

This split mirrors a real deployment where the API and each org's worker fleet scale independently.

```mermaid
flowchart TB
    Dashboard["React Dashboard<br/>Browser client"]

    subgraph API["API — N replicas"]
        Server["API Server<br/>Go · chi router · JWT · stateless"]
        Scheduler["Scheduler goroutine<br/>ticks · cron dispatch · reaps stale claims"]
    end

    Redis[("Redis<br/>rate-limit counters only")]
    Postgres[("PostgreSQL<br/>single source of truth")]

    subgraph WorkersA["Workers — Org A (WORKER_ORG_ID=A)"]
        WorkerA["Worker<br/>polls · claims · executes · heartbeats"]
    end

    subgraph WorkersB["Workers — Org B (WORKER_ORG_ID=B)"]
        WorkerB["Worker<br/>polls · claims · executes · heartbeats"]
    end

    Dashboard -- "HTTPS + JWT" --> Server
    Dashboard -. "poll every 5s" .-> Server
    Server -- "check / incr rate limit" --> Redis
    Server -- "reads / writes" --> Postgres
    Scheduler -- "pg_try_advisory_lock, dispatch due scheduled_jobs" --> Postgres

    WorkerA -- "poll & claim — SELECT ... FOR UPDATE SKIP LOCKED, org A's queues only" --> Postgres
    WorkerA -- "heartbeat every 10s" --> Postgres
    WorkerB -- "poll & claim — org B's queues only" --> Postgres
    WorkerB -- "heartbeat every 10s" --> Postgres
```

The job lifecycle state machine lives in
[`server/docs/architecture.md`](server/docs/architecture.md).

## Dashboard

See [`ui-interface/README.md`](ui-interface/README.md) for the frontend's
feature list, structure, and setup instructions.

## Project Structure

```
ui-interface/                # React dashboard (Vite) — see ui-interface/README.md
```

Backend structure and docs live under `server/` — see the
[Documentation](#documentation) table below for the full breakdown
(architecture, ER diagram, API reference, design decisions).

## Prerequisites

- Go 1.25+
- PostgreSQL 15+
- Redis 7+
- Node.js 18+ (for the frontend)
- [golang-migrate CLI](https://github.com/golang-migrate/migrate)

## Setup

### 1. Install dependencies

```bash
cd server
go mod tidy
```

### 2. Configure environment

```bash
cp .env.example .env                                   # from server/
cp ../ui-interface/.env.example ../ui-interface/.env    # frontend
```

### 3. Start Redis and Postgres

Make sure both are running locally, or via Docker:

```bash
docker run -d --name js-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
docker run -d --name js-redis -p 6379:6379 redis:7
```

### 4. Create the database

```bash
docker exec js-postgres createdb -U postgres jobscheduler
```

### 5. Run migrations

From `server/`:

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

### 6. Run the API server

From `server/`:

```bash
go run ./cmd/api
```

### 7. Run the frontend

```bash
cd ui-interface
npm install
npm run dev
```

### 8. Register an account and grab your org ID

Open the frontend, register an org, and copy the `org_id` from the
response (visible in your browser's dev tools — Network tab on the
register call, or Local Storage under the `user` key). A worker belongs to
exactly one organization, so this ID is required before it can start.

### 9. Run the worker (separate terminal, run multiple instances to test concurrency)

From `server/`, with the org ID from the previous step:

```bash
WORKER_ORG_ID=<your-org-id> go run ./cmd/worker
```

Or set `WORKER_ORG_ID` in `server/.env` instead of passing it inline —
either way, `cmd/worker` refuses to start without it.

## Testing

From `server/`:

```bash
go test ./...
```

This runs the pure unit tests unconditionally, but every test that needs a
real Postgres (concurrency, claiming, reaping, the full HTTP router) skips
itself unless `TEST_DATABASE_URL` is set — and the HTTP-layer tests also
need a reachable Redis (`TEST_REDIS_ADDR`, default `localhost:6379`), since
the real rate-limit middleware sits in that router too. Point it at a
throwaway database, not the one you're using for manual testing — these
tests don't clean up after themselves:

```bash
docker exec js-postgres createdb -U postgres jobscheduler_test
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/jobscheduler_test?sslmode=disable" up
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/jobscheduler_test?sslmode=disable" go test ./...
```

| Test | Proves |
|---|---|
| `TestClaimNext_NoDuplicateClaims` | 50 workers racing for 200 jobs never double-claim |
| `TestJobRepository_Create_IdempotencyKeyDedupes` | A repeated idempotency key returns the same job, not a duplicate |
| `TestJobRepository_ClaimNext_RespectsPause` | A paused queue yields no claims |
| `TestJobRepository_ClaimNext_RespectsConcurrencyLimit` | `concurrency_limit` is enforced, then releases once a job completes |
| `TestJobRepository_ReapStale_*` | A claim with no recent heartbeat is requeued; one with a live heartbeat is left alone |
| `TestJobRepository_ReapStale_HandlesNonUUIDLockedBy` | A malformed `locked_by` value is reaped instead of crashing the query |
| `TestExecutionService_Run_SuccessCompletesJob` | The full claim → execute → complete pipeline |
| `TestExecutionService_Run_RetriesThenDeadLetters` | A failing job retries with backoff, then dead-letters once attempts are exhausted |
| `TestRouter_*` (`internal/handler`) | HTTP-layer tests against the real router: register/login, 401 with no bearer token, 400 on an invalid job type, and the RBAC gate — a `member` gets 403 deleting a project, its `owner` gets 200 |

## Verifying the Frontend UI

`go test` and the API only prove the backend is correct — none of it touches
the dashboard. With the API, a worker, and `npm run dev` running (see
[Setup](#setup)), open the app and walk through this list; each row is
something to click, not just read.

| Area | What to check |
|---|---|
| **Dropdowns** | Open the job-type selector (Jobs tab) or the status filter — it opens a themed popover menu that matches light/dark mode, not the browser's native OS-style option list. |
| **Toasts** | Do anything that mutates state (create a project, submit a job, pause a queue). A toast slides in from the top-right with a colored left rule and a shrinking progress bar; hovering it pauses the auto-dismiss timer. |
| **Confirm dialog** | Projects → **Delete** on a project card. A centered modal with a warning icon and a solid-red **Delete** button appears — not the browser's native `confirm()` popup. Escape or clicking outside cancels it. |
| **Sidebar nav** | Overview / Projects / Workers each have an icon, and the active page is a filled amber pill, not just a text color change. |
| **Section tabs** | On a queue's detail page, Jobs / Scheduled / Dead letters / Configuration render as a segmented pill control — the active tab sits raised on its own background inside a bordered track. |
| **Auth tabs** | On `/login` or `/register`, a "Log in / Register" tab pair sits above the form and switches pages when clicked. |
| **Tables** | Job/worker/queue listings have a filled header bar and roomy rows (not a cramped, thin-text grid). |
| **Number fields** | Priority, concurrency, delay (ms), and batch-count inputs show no up/down spinner arrows — plain numeric fields only. |
| **Scrollbars** | Open a dropdown with more options than fit (e.g. the status filter) — the scrollbar is a thin, theme-colored bar, not the platform default. |
| **Headings** | Page titles ("Overview", "Projects", a queue's name, "Job detail") are visibly larger and bolder than the body text under them. |

For the underlying job-scheduling behavior itself (delays, concurrency
limits, retries, dead-lettering, cron dispatch) rather than the UI chrome,
follow the full click-by-click script in [`WORKING.md`](WORKING.md#part-2--verifying-it-all-yourself-locally-in-the-ui).

## Rolling Back Migrations

From `server/`:

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

## Documentation

| Doc | Covers |
|---|---|
| [`server/docs/architecture.md`](server/docs/architecture.md) | Component diagram, job lifecycle state machine |
| [`server/docs/er-diagram.md`](server/docs/er-diagram.md) | Full ER diagram, keys, indexes, cascade behavior |
| [`server/docs/api.md`](server/docs/api.md) | Every REST endpoint — request/response shapes, error codes, pagination |
| [`server/docs/design-decisions.md`](server/docs/design-decisions.md) | Trade-offs: `SKIP LOCKED` vs. an external queue, per-queue concurrency locking, Redis rate limiting, the advisory-lock scheduler, cascade-vs-soft-delete |
| [`ui-interface/README.md`](ui-interface/README.md) | Frontend feature list, structure, env vars |
| [`WORKING.md`](WORKING.md) | How the system works end to end, plus a click-by-click local verification script |