# Job Scheduler API & Worker

Go backend for the **[Distributed Job Scheduler](Readme.md)**: two
independent binaries (`cmd/api`, `cmd/worker`) sharing a common
`internal/` package, backed by PostgreSQL (`SELECT ... FOR UPDATE SKIP
LOCKED` job claiming) and Redis (rate limiting). Lives under `server/`.

For setup and running instructions, see the
**[project README](Readme.md)**. Its **[Setup](Readme.md#setup)** and
**[Rolling Back Migrations](Readme.md#rolling-back-migrations)** sections
cover this, since the API, worker, and frontend are started together.

## Structure

```
server/
├── cmd/
│   ├── api/              # REST API server entrypoint
│   └── worker/           # Worker entrypoint, scoped to one org via WORKER_ORG_ID
├── internal/
│   ├── apperr/            # Typed application errors → HTTP status mapping
│   ├── authtoken/         # JWT issuing/parsing
│   ├── config.go          # Env-based config loading
│   ├── db/                # Postgres pool setup (pgx/v5 + pgxpool)
│   ├── handler/           # HTTP handlers (auth, projects, queues, jobs, dashboard, DLQ, workers)
│   ├── httpx/             # Shared response envelope + keyset pagination helpers
│   ├── middleware/        # Auth, request ID, logging, Redis rate limiting
│   ├── models/            # Domain structs (job, queue, project, org, execution, log, dead letter)
│   ├── repository/        # Postgres queries: claiming, reaping, dead-lettering, concurrency
│   ├── scheduler/         # Advisory-lock leader election + cron dispatch loop
│   ├── service/           # Business logic: execution pipeline, retries, auth, dashboard
│   └── testutil/          # Test router/DB helpers shared by handler + repository tests
├── migrations/            # golang-migrate SQL migrations
└── docs/                  # Architecture, ER diagram, API reference, design decisions
```

## Testing

From `server/`:

```bash
go test ./...
```

This runs the pure unit tests unconditionally, but every test that needs a
real Postgres (concurrency, claiming, reaping, the full HTTP router) skips
itself unless `TEST_DATABASE_URL` is set. The HTTP-layer tests also need a
reachable Redis (`TEST_REDIS_ADDR`, default `localhost:6379`), since the
real rate-limit middleware sits in that router too. Point it at a
throwaway database, not the one you're using for manual testing: these
tests don't clean up after themselves.

```bash
docker exec js-postgres createdb -U postgres jobscheduler_test
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/jobscheduler_test?sslmode=disable" up
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/jobscheduler_test?sslmode=disable" go test ./...
```

## Documentation

| Doc | Covers |
|---|---|
| **[`server/docs/architecture.md`](server/docs/architecture.md)** | Component diagram, job lifecycle state machine |
| **[`server/docs/er-diagram.md`](server/docs/er-diagram.md)** | Full ER diagram, keys, indexes, cascade behavior |
| **[`server/docs/api.md`](server/docs/api.md)** | Every REST endpoint: request/response shapes, error codes, pagination |
| **[`server/docs/design-decisions.md`](server/docs/design-decisions.md)** | Trade-offs: `SKIP LOCKED` vs. an external queue, per-queue concurrency locking, Redis rate limiting, the advisory-lock scheduler, cascade-vs-soft-delete |
