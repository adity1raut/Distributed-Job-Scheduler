# Design Decisions

## Job claiming: Postgres `SKIP LOCKED` vs. an external queue

Workers claim jobs with `SELECT ... FOR UPDATE SKIP LOCKED` against the `jobs`
table rather than fronting Postgres with a dedicated queue (SQS, RabbitMQ,
Kafka). This keeps job state, retries, and application data in one
transactional store, at the cost of not scaling to the throughput a
purpose-built queue offers — acceptable for this system's expected volume.

## Rate limiting: Redis

Per-project API rate limits are enforced with Redis counters rather than
in-process limiters, so limits hold correctly across multiple API server
instances.

## Two binaries, one `internal/` package

`cmd/api` and `cmd/worker` are separate binaries so the API and worker fleet
can be deployed and scaled independently, while sharing domain logic through
`internal/`.

## Architecture

```
React Dashboard  →  Go API  →  PostgreSQL
                                    ↑
                Go Worker(s)  ──────┘
                     ↓
                   Redis (rate limiting)
```
