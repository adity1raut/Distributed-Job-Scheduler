# Distributed Job Scheduler

A production-inspired distributed job scheduling platform for reliably executing asynchronous background jobs across multiple workers, with authentication, project/queue management, retries, dead-letter handling, and a live dashboard.

## Contents

- **[Quick Start](#quick-start)**
- **[Tech Stack](#tech-stack)**
- **[Architecture](#architecture)**
- **[Project Structure](#project-structure)**
- **[Manual Setup](#manual-setup-without-docker)**
- **[Testing](#testing)**
- **[Deployment](#deployment)**
- **[Rolling Back Migrations](#rolling-back-migrations)**
- **[Documentation](#documentation)**

## Quick Start

Docker is the only prerequisite. One command brings up the database, cache,
migrations, API, worker fleet and dashboard:

```bash
make up
```

Then open **http://localhost:3000** and sign in with `admin@example.com` /
`password123` (set in `.env`).

To verify the whole system really works — submit a job and watch a worker
execute it:

```bash
make smoke
```

Other common commands:

```bash
make logs                  # tail every service
make logs SVC=worker       # tail just the workers
make scale-workers N=5     # run five workers
make down                  # stop, keeping data
make clean                 # stop and delete the database
make help                  # everything else
```

Full details, including how to deploy to a server, are in
**[DEPLOYMENT.md](DEPLOYMENT.md)**.

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
| Containers | Docker + Docker Compose |
| CI/CD | GitHub Actions, images published to GHCR |

## Architecture

Two independent Go binaries share a common `internal/` package:

- **`cmd/api`**: the REST API server. Auth, projects, queues, jobs, dashboard data.
- **`cmd/worker`**: belongs to exactly one org (`WORKER_ORG_ID`), polls that org's queues, atomically claims jobs with `SELECT ... FOR UPDATE SKIP LOCKED`, executes them concurrently, sends heartbeats, and shuts down gracefully on `SIGTERM`.

This split mirrors a real deployment, where the API and each org's worker fleet need to scale independently of each other.

![Architecture diagram: React dashboard talks to a horizontally scaled API server over HTTPS with JWT auth and polls it every 5 seconds for live updates; the API reads and writes PostgreSQL and checks Redis for rate limits; a scheduler goroutine inside the API dispatches due scheduled jobs into PostgreSQL under a Postgres advisory lock; two separate per-org worker fleets each poll PostgreSQL to claim only their own organization's jobs with SELECT FOR UPDATE SKIP LOCKED and send heartbeats.](server/docs/images/architecture.png)

The job lifecycle state machine lives in the
**[backend architecture doc](server/docs/architecture.md)**.


## Project Structure

```
server/                      # Go API + worker, see server.md
  Dockerfile                 # builds both binaries into one image
ui-interface/                # React dashboard (Vite), see ui_interface.md
  Dockerfile                 # builds the SPA, serves it with nginx
deploy/scripts/              # bootstrap and smoke-test helpers
.github/workflows/           # CI, CD and CodeQL pipelines
docker-compose.yml           # the stack
docker-compose.override.yml  # local development (applied automatically)
docker-compose.prod.yml      # production overlay
Makefile                     # every task; run `make help`
```

## Manual Setup (without Docker)

`make up` handles all of this for you — these steps are for running the
services directly on your machine instead.

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

See the **[backend doc](server.md#testing)** for what runs
unconditionally vs. what needs a real Postgres/Redis, and how to point
tests at a throwaway database.

From `ui-interface/`, there's no test runner configured, only lint:

```bash
npm run lint
```

## Deployment

The project ships as two Docker images (`api` and `web`) built and published
by GitHub Actions. `docker-compose.prod.yml` runs them on any Linux box with
Docker installed — no orchestrator needed.

**[DEPLOYMENT.md](DEPLOYMENT.md)** covers it end to end: how the containers fit
together, every configuration variable, deploying to a server, what each CI/CD
workflow does, rollbacks, backups and troubleshooting.

## Rolling Back Migrations

From `server/`:

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

## Documentation

| Doc | Covers |
|---|---|
| **[`server.md`](server.md)** | Backend structure, `cmd/`/`internal/` breakdown |
| **[`server/docs/architecture.md`](server/docs/architecture.md)** | Component diagram, job lifecycle state machine |
| **[`server/docs/er-diagram.md`](server/docs/er-diagram.md)** | Full ER diagram, keys, indexes, cascade behavior |
| **[`server/docs/api.md`](server/docs/api.md)** | Every REST endpoint: request/response shapes, error codes, pagination |
| **[`server/docs/design-decisions.md`](server/docs/design-decisions.md)** | Trade-offs: `SKIP LOCKED` vs. an external queue, per-queue concurrency locking, Redis rate limiting, the advisory-lock scheduler, cascade-vs-soft-delete |
| **[`ui_interface.md`](ui_interface.md)** | Frontend feature list, structure, env vars |
| **[`ui-interface/docs/getting-started.md`](ui-interface/docs/getting-started.md)** | Registering an org, starting a worker, creating a project/queue, all four job types, cron schedules, concurrency |
| **[`DEPLOYMENT.md`](DEPLOYMENT.md)** | Docker images, compose stack, configuration, deploying to a server, CI/CD pipelines, rollbacks, troubleshooting |
