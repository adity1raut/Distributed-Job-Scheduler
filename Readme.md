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

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; a fleet of workers polls PostgreSQL to claim jobs with SELECT FOR UPDATE SKIP LOCKED and sends heartbeats.](server/docs/images/architecture.png)

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