# Distributed Job Scheduler

A production-inspired distributed job scheduling platform for reliably executing asynchronous background jobs across multiple workers — with authentication, project/queue management, retries, dead-letter handling, and a live dashboard.

## Contents

- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Dashboard](#dashboard)
- [Project Structure](#project-structure)
- [Setup](#setup)
- [Testing](#testing)
- [Rolling Back Migrations](#rolling-back-migrations)
- [Documentation](#documentation)
- [Deliverables](#deliverables)

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
- **`cmd/worker`** — polls queues, atomically claims jobs (`SELECT ... FOR UPDATE SKIP LOCKED`), executes them concurrently, sends heartbeats, and shuts down gracefully on `SIGTERM`

This split mirrors a real deployment where the API and worker fleet scale independently.

```
React Dashboard  →  Go API  →  PostgreSQL
                                    ↑
                Go Worker(s)  ──────┘
                     ↓
                   Redis (rate limiting)
```

A fuller diagram (with the scheduler's advisory-lock leader election drawn
in) and the job lifecycle state machine live in
[`server/docs/architecture.md`](server/docs/architecture.md).

## Dashboard

The `ui-interface/` React app is a full dashboard against the API above, not
a mockup: live overview stats, project/queue management, a filterable job
explorer with per-attempt execution logs, cron schedule management, a
dead-letter queue view with one-click replay, and worker fleet status —
everything polls the API every few seconds. Every mutation reports success
or failure via toast. See [`ui-interface/README.md`](ui-interface/README.md)
for the full feature list and frontend structure.

## Project Structure

```
server/
├── cmd/
│   ├── api/                # API server entry point
│   └── worker/             # Worker service entry point
├── internal/
│   ├── apperr/              # Structured error type ({code, message, request_id})
│   ├── authtoken/            # JWT issue/verify, shared by auth service + auth middleware
│   ├── db/                   # Postgres connection pool (pgxpool)
│   ├── httpx/                  # JSON response helpers, keyset pagination
│   ├── models/                  # Domain types for all 12 entities
│   ├── repository/               # DB queries — atomic claiming lives in job_repository.go
│   ├── service/                    # Business logic, incl. the worker's execution engine
│   ├── handler/                     # HTTP handlers + chi router wiring
│   ├── middleware/                   # Auth (JWT), rate limiting (Redis), logging, request ID
│   ├── scheduler/                     # Cron dispatch + advisory-lock leader election + reaper
│   └── config.go
├── migrations/              # Versioned SQL schema (golang-migrate)
├── docs/
│   ├── architecture.md      # Component diagram + job lifecycle state machine (Mermaid)
│   ├── er-diagram.md        # Full ER diagram + index/cascade rationale (Mermaid)
│   ├── api.md               # REST API reference — every endpoint, request/response shapes
│   └── design-decisions.md  # Trade-offs behind the schema and reliability mechanisms
├── go.mod
└── .gitignore

ui-interface/                # React dashboard (Vite) — see ui-interface/README.md
```

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

Create a `.env` file (or export these directly):

```bash
DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable
REDIS_ADDR=localhost:6379
JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRY_HOURS=24
API_PORT=8080
WORKER_POLL_MS=500
WORKER_CONCURRENCY=10
HEARTBEAT_SEC=10
STALE_JOB_SEC=60
SCHEDULER_TICK_SEC=5
RATE_LIMIT_PER_MIN=120
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

All of the above have sane defaults in `internal/config.go` — a `.env` file is
only needed to override them (e.g. a non-default DB URL). `.env` loads
automatically via `godotenv`; copy `server/.env.example` to get started.
`CORS_ALLOWED_ORIGINS` is comma-separated if the dashboard is served from
more than one origin.

The frontend has its own env file — `ui-interface/.env`
(`cp ui-interface/.env.example ui-interface/.env`), one variable:
`VITE_API_URL`, defaulting to `http://localhost:8080`.

### 3. Create the database

```bash
createdb jobscheduler
```

### 4. Run migrations

From `server/`:

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

### 5. Start Redis and Postgres

Make sure both are running locally, or via Docker:

```bash
docker run -d --name js-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
docker run -d --name js-redis -p 6379:6379 redis:7
```

### 6. Run the API server

From `server/`:

```bash
go run ./cmd/api
```

### 7. Run the worker (separate terminal, run multiple instances to test concurrency)

From `server/`:

```bash
go run ./cmd/worker
```

### 8. Run the frontend

```bash
cd ui-interface
npm install
npm run dev
```

## Testing

From `server/`:

```bash
go test ./...
```

Unit tests (retry backoff math) run with no setup. The integration tests —
concurrency-correctness, queue pause/concurrency-limit enforcement,
stale-claim reaping, idempotency, and the full retry → dead-letter pipeline
— need a real Postgres, since `SKIP LOCKED` and row-level locking are
database guarantees no mock can prove. They all `t.Skip` automatically when
`TEST_DATABASE_URL` isn't set, so `go test ./...` stays usable with no setup;
point it at a throwaway instance to run them:

```bash
docker run -d --name js-test-postgres -e POSTGRES_PASSWORD=postgres -p 55432:5432 postgres:15
createdb -h localhost -p 55432 -U postgres jobscheduler
migrate -path migrations -database "postgres://postgres:postgres@localhost:55432/jobscheduler?sslmode=disable" up
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55432/jobscheduler?sslmode=disable" go test ./... -v
```

| Test | Proves |
|---|---|
| `TestClaimNext_NoDuplicateClaims` | 50 workers racing for 200 jobs never double-claim |
| `TestJobRepository_Create_IdempotencyKeyDedupes` | A repeated idempotency key returns the same job, not a duplicate |
| `TestJobRepository_ClaimNext_RespectsPause` | A paused queue yields no claims |
| `TestJobRepository_ClaimNext_RespectsConcurrencyLimit` | `concurrency_limit` is enforced, then releases once a job completes |
| `TestJobRepository_ReapStale_*` | A claim with no recent heartbeat is requeued; one with a live heartbeat is left alone |
| `TestExecutionService_Run_SuccessCompletesJob` | The full claim → execute → complete pipeline |
| `TestExecutionService_Run_RetriesThenDeadLetters` | A failing job retries with backoff, then dead-letters once attempts are exhausted |

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

## Deliverables

| Required by the brief | Where |
|---|---|
| Source code with setup instructions | This file — [Setup](#setup) |
| Architecture diagram | [`server/docs/architecture.md`](server/docs/architecture.md) |
| ER diagram | [`server/docs/er-diagram.md`](server/docs/er-diagram.md) |
| API documentation | [`server/docs/api.md`](server/docs/api.md) |
| Design decisions document | [`server/docs/design-decisions.md`](server/docs/design-decisions.md) |
| Automated tests | [Testing](#testing) — unit + integration, see table above |