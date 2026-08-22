# Entity-Relationship Diagram

All twelve entities the brief names: Users, Organizations, Projects, Queues,
Jobs, Job Executions, Retry Policies, Workers, Worker Heartbeats, Job Logs,
Scheduled Jobs, Dead Letter Queue.

```mermaid
erDiagram
    ORGANIZATIONS ||--o{ USERS : employs
    ORGANIZATIONS ||--o{ PROJECTS : owns
    USERS ||--o{ PROJECTS : creates
    PROJECTS ||--o{ QUEUES : contains
    RETRY_POLICIES ||--o{ QUEUES : "default policy"
    QUEUES ||--o{ JOBS : holds
    QUEUES ||--o{ SCHEDULED_JOBS : defines
    SCHEDULED_JOBS ||--o{ JOBS : spawns
    RETRY_POLICIES ||--o{ JOBS : "override (nullable)"
    JOBS ||--o{ JOB_EXECUTIONS : "attempted as"
    JOB_EXECUTIONS ||--o{ JOB_LOGS : emits
    WORKERS ||--o{ JOB_EXECUTIONS : executes
    WORKERS ||--o{ WORKER_HEARTBEATS : reports
    JOBS ||--o| DEAD_LETTER_QUEUE : "terminates in"

    ORGANIZATIONS {
        uuid id PK
        text name
        timestamptz created_at
    }
    USERS {
        uuid id PK
        uuid org_id FK
        text email UK
        text password_hash
        text role
        timestamptz created_at
    }
    PROJECTS {
        uuid id PK
        uuid org_id FK
        uuid owner_id FK
        text name
        timestamptz created_at
    }
    RETRY_POLICIES {
        uuid id PK
        uuid org_id FK
        text name
        text strategy
        int base_delay_ms
        int max_delay_ms
        int max_attempts
        numeric multiplier
    }
    QUEUES {
        uuid id PK
        uuid project_id FK
        uuid retry_policy_id FK
        text name
        int priority
        int concurrency_limit
        bool is_paused
    }
    SCHEDULED_JOBS {
        uuid id PK
        uuid queue_id FK
        text cron_expression
        jsonb payload_template
        timestamptz next_run_at
        bool is_active
    }
    JOBS {
        uuid id PK
        uuid queue_id FK
        uuid scheduled_job_id FK
        uuid retry_policy_id FK
        text type
        text status
        jsonb payload
        text idempotency_key
        int priority
        int attempts
        int max_attempts
        uuid batch_id
        timestamptz run_at
        text locked_by
        timestamptz locked_at
        text last_error
    }
    JOB_EXECUTIONS {
        uuid id PK
        uuid job_id FK
        uuid worker_id FK
        int attempt_number
        text status
        timestamptz started_at
        timestamptz finished_at
        text error_message
        int duration_ms
    }
    JOB_LOGS {
        uuid id PK
        uuid job_execution_id FK
        timestamptz logged_at
        text level
        text message
    }
    WORKERS {
        uuid id PK
        text hostname
        text status
        timestamptz started_at
    }
    WORKER_HEARTBEATS {
        uuid id PK
        uuid worker_id FK
        timestamptz reported_at
        int active_job_count
    }
    DEAD_LETTER_QUEUE {
        uuid id PK
        uuid job_id FK
        text final_error
        jsonb payload_snapshot
        timestamptz failed_at
        bool replayed
    }
```

## Keys, indexes, cascades

Full DDL: [`migrations/000001_init_schema.up.sql`](../migrations/000001_init_schema.up.sql).

- Every table uses a `uuid` surrogate primary key (`gen_random_uuid()`) — safe
  to expose in the API, doesn't leak row counts.
- **3NF throughout.** `retry_policies` is its own table, not columns on
  `queues`, because a policy is reusable across queues and a job can override
  its queue's default (`jobs.retry_policy_id` is nullable — falls back to
  `queues.retry_policy_id` when unset).
- `job_executions` and `job_logs` are append-only — a job's own row is
  overwritten on every retry, but each attempt keeps its own execution row,
  so retry history stays a real audit trail.
- **Indexes that carry the write path:**
  - `jobs(queue_id, status, priority DESC, run_at)` — covers the atomic-claim
    query with no sequential scan.
  - `UNIQUE (queue_id, idempotency_key) WHERE idempotency_key IS NOT NULL` —
    idempotent job submission enforced at the database, not just in code.
  - `worker_heartbeats(worker_id, reported_at DESC)` — "is this worker alive"
    is always a most-recent-row lookup.
  - `scheduled_jobs(next_run_at) WHERE is_active` — partial index; the
    scheduler only ever scans active, due rows.
- **Cascades** run down the containment tree —
  `organizations → projects → queues → jobs → job_executions → job_logs` — an
  orphaned execution log with no parent job is meaningless.
  `dead_letter_queue` cascades with its job too. Trade-off: deleting a
  project destroys its audit history; noted in
  [design-decisions.md](design-decisions.md) as the first thing to swap for a
  `deleted_at` soft-delete in a compliance-sensitive deployment.
