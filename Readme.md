

# Distributed Job Scheduler

A production-inspired distributed job scheduling platform for reliably executing asynchronous background jobs across multiple workers — with authentication, project/queue management, retries, dead-letter handling, and a live dashboard.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend API + Worker | Go |
| Frontend | React |
| Database | PostgreSQL |
| Rate Limiting | Redis |
| Migrations | golang-migrate |
| Router | chi |
| Auth | JWT |
| Cron parsing | robfig/cron |

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

## Project Structure

```
job-scheduler/
├── cmd/
│   ├── api/            # API server entry point
│   └── worker/         # Worker service entry point
├── internal/
│   ├── db/              # Postgres connection pool
│   ├── models/           # Domain types (Job, Queue, Worker, etc.)
│   ├── repository/       # DB queries
│   ├── service/           # Business logic
│   ├── handler/            # HTTP handlers
│   ├── middleware/          # Auth, rate limiting, logging
│   ├── scheduler/            # Cron/delayed job dispatch
│   └── config.go
├── migrations/          # Versioned SQL schema (golang-migrate)
├── frontend/             # React dashboard
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- Node.js 18+ (for frontend)
- [golang-migrate CLI](https://github.com/golang-migrate/migrate)

## Setup

### 1. Clone and install dependencies

```bash
git clone https://github.com/adity1raut/job-scheduler.git
cd job-scheduler
go mod tidy
```

### 2. Configure environment

Create a `.env` file (or export these directly):

```bash
DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobscheduler?sslmode=disable
REDIS_ADDR=localhost:6379
JWT_SECRET=replace-with-a-long-random-secret
API_PORT=8080
WORKER_POLL_MS=500
WORKER_CONCURRENCY=10
HEARTBEAT_SEC=10
STALE_JOB_SEC=60
```

### 3. Create the database

```bash
createdb jobscheduler
```

### 4. Run migrations

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

```bash
go run ./cmd/api
```

### 7. Run the worker (separate terminal, run multiple instances to test concurrency)

```bash
go run ./cmd/worker
```

### 8. Run the frontend

```bash
cd frontend
npm install
npm run dev
```

## Testing

```bash
go test ./...
```

Concurrency correctness tests spin up multiple simulated workers against the same queue and assert zero duplicate job claims.

## Rolling Back Migrations

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

## Design Documentation

See `docs/design-decisions.md` for schema rationale, trade-offs (e.g. Postgres `SKIP LOCKED` over an external queue, Redis for rate limiting), and architecture diagrams.