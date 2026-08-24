# Distributed Job Scheduler

A production-inspired distributed job scheduling platform for reliably executing asynchronous background jobs across multiple workers, with authentication, project/queue management, retries, dead-letter handling, and a live dashboard.

## Contents

- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
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

Two independent Go binaries share a common `internal/` package:

- **`cmd/api`**: the REST API server. Auth, projects, queues, jobs, dashboard data.
- **`cmd/worker`**: belongs to exactly one org (`WORKER_ORG_ID`), polls that org's queues, atomically claims jobs with `SELECT ... FOR UPDATE SKIP LOCKED`, executes them concurrently, sends heartbeats, and shuts down gracefully on `SIGTERM`.

This split mirrors a real deployment, where the API and each org's worker fleet need to scale independently of each other.

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](server/docs/images/architecture.png)

The job lifecycle state machine lives in
[`server/docs/architecture.md`](server/docs/architecture.md).


## Project Structure

```
server/                      # Go API + worker, see server/README.md
ui-interface/                # React dashboard (Vite), see ui-interface/README.md
```

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
response. It's visible in your browser's dev tools: the Network tab on the
register call, or Local Storage under the `user` key. A worker belongs to
exactly one organization, so this ID is required before one can start.

### 9. Run the worker (separate terminal, run multiple instances to test concurrency)

From `server/`, with the org ID from the previous step:

```bash
WORKER_ORG_ID=<your-org-id> go run ./cmd/worker
```

Or set `WORKER_ORG_ID` in `server/.env` instead of passing it inline.
Either way, `cmd/worker` refuses to start without it.

## Testing

From `server/`:

```bash
go test ./...
```

See [`server/README.md`](server/README.md#testing) for what runs
unconditionally vs. what needs a real Postgres/Redis, and how to point
tests at a throwaway database.

From `ui-interface/`, there's no test runner configured, only lint:

```bash
npm run lint
```

## Rolling Back Migrations

From `server/`:

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

## Documentation

| Doc | Covers |
|---|---|
| [`server/README.md`](server/README.md) | Backend structure, `cmd/`/`internal/` breakdown |
| [`server/docs/architecture.md`](server/docs/architecture.md) | Component diagram, job lifecycle state machine |
| [`server/docs/er-diagram.md`](server/docs/er-diagram.md) | Full ER diagram, keys, indexes, cascade behavior |
| [`server/docs/api.md`](server/docs/api.md) | Every REST endpoint: request/response shapes, error codes, pagination |
| [`server/docs/design-decisions.md`](server/docs/design-decisions.md) | Trade-offs: `SKIP LOCKED` vs. an external queue, per-queue concurrency locking, Redis rate limiting, the advisory-lock scheduler, cascade-vs-soft-delete |
| [`ui-interface/README.md`](ui-interface/README.md) | Frontend feature list, structure, env vars |
| [`WORKING.md`](WORKING.md) | How the system works end to end, plus a click-by-click local verification script |
