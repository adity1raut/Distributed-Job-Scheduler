# Entity-Relationship Diagram

Twelve tables, matching the brief exactly: Users, Organizations, Projects,
Queues, Jobs, Job Executions, Retry Policies, Workers, Worker Heartbeats,
Job Logs, Scheduled Jobs, Dead Letter Queue.

```mermaid
erDiagram
    ORGANIZATIONS ||--o{ USERS : "has"
    ORGANIZATIONS ||--o{ PROJECTS : "owns"
    ORGANIZATIONS ||--o{ RETRY_POLICIES : "defines"
    ORGANIZATIONS ||--o{ WORKERS : "scopes"
    USERS ||--o{ PROJECTS : "owns (owner_id)"
    PROJECTS ||--o{ QUEUES : "contains"
    RETRY_POLICIES ||--o{ QUEUES : "default policy for"
    RETRY_POLICIES |o--o{ JOBS : "per-job override"
    QUEUES ||--o{ JOBS : "holds"
    QUEUES ||--o{ SCHEDULED_JOBS : "defines"
    SCHEDULED_JOBS |o--o{ JOBS : "spawns (recurring)"
    JOBS ||--o{ JOB_EXECUTIONS : "produces"
    JOBS |o--o{ DEAD_LETTER_QUEUE : "terminates in"
    JOB_EXECUTIONS ||--o{ JOB_LOGS : "emits"
    WORKERS ||--o{ JOB_EXECUTIONS : "executes"
    WORKERS ||--o{ WORKER_HEARTBEATS : "reports"

    ORGANIZATIONS {
        uuid id PK
        string name
        timestamp created_at
    }
    USERS {
        uuid id PK
        uuid org_id FK
        string email UK
        string password_hash
        enum role "owner | admin | member"
        timestamp created_at
    }
    PROJECTS {
        uuid id PK
        uuid org_id FK
        uuid owner_id FK
        string name
        timestamp created_at
    }
    RETRY_POLICIES {
        uuid id PK
        uuid org_id FK
        string name UK "unique per org"
        enum strategy "fixed | linear | exponential"
        int base_delay_ms
        int max_delay_ms
        int max_attempts
        numeric multiplier
    }
    QUEUES {
        uuid id PK
        uuid project_id FK
        uuid retry_policy_id FK
        string name UK "unique per project"
        int priority
        int concurrency_limit
        boolean is_paused
    }
    SCHEDULED_JOBS {
        uuid id PK
        uuid queue_id FK
        string cron_expression
        jsonb payload_template
        timestamp next_run_at
        boolean is_active
    }
    JOBS {
        uuid id PK
        uuid queue_id FK
        uuid scheduled_job_id FK "nullable, SET NULL"
        uuid retry_policy_id FK "nullable override, SET NULL"
        enum type "immediate | delayed | scheduled | recurring | batch"
        enum status "scheduled..queued..claimed..running..completed | failed | dead"
        jsonb payload
        string idempotency_key UK "unique per queue, nullable"
        int priority
        int attempts
        int max_attempts
        uuid batch_id "nullable, groups a batch submission"
        timestamp run_at
        string locked_by "nullable, plain text worker id"
        timestamp locked_at
        string last_error
    }
    JOB_EXECUTIONS {
        uuid id PK
        uuid job_id FK
        uuid worker_id FK
        int attempt_number
        enum status "running | succeeded | failed"
        timestamp started_at
        timestamp finished_at
        string error_message
        int duration_ms
    }
    JOB_LOGS {
        uuid id PK
        uuid job_execution_id FK
        timestamp logged_at
        enum level "debug | info | warn | error"
        string message
    }
    WORKERS {
        uuid id PK
        uuid org_id FK "added in 000002_worker_org_scoping, nullable"
        string hostname
        enum status "online | offline"
        timestamp started_at
    }
    WORKER_HEARTBEATS {
        uuid id PK
        uuid worker_id FK
        timestamp reported_at
        int active_job_count
    }
    DEAD_LETTER_QUEUE {
        uuid id PK
        uuid job_id FK
        string final_error
        jsonb payload_snapshot
        timestamp failed_at
        boolean replayed
    }
```

Full DDL, if you want the exact constraints and defaults:
[`migrations/000001_init_schema.up.sql`](../migrations/000001_init_schema.up.sql)
and [`migrations/000002_worker_org_scoping.up.sql`](../migrations/000002_worker_org_scoping.up.sql).

## A few things worth knowing about this schema

Every table uses a `uuid` primary key from `gen_random_uuid()` instead of a
serial integer — mainly so IDs are safe to hand back in API responses
without leaking how many rows exist.

It's normalized to 3NF throughout, and the clearest example is
`retry_policies` being its own table instead of a few columns bolted onto
`queues`. A policy needs to be reusable across queues, and a single job
occasionally needs to override its queue's default — neither of those work
if the policy is just inline columns. `jobs.retry_policy_id` is nullable
for exactly this reason: unset, it falls back to whatever `queues.retry_policy_id`
says.

`job_executions` and `job_logs` are append-only. A job's own row gets
overwritten every time it retries, but each *attempt* keeps its own row —
otherwise there'd be no way to look back and see what actually happened on
attempt 2 once attempt 3 has already started.

### `workers.org_id` is nullable — the one intentional exception

Every other FK in this schema is `NOT NULL`. `workers.org_id` is the
exception: it was added by a later migration
(`000002_worker_org_scoping`), and existing worker rows from before that
migration have no organization they can be correctly backfilled to.
Deleting them instead of leaving them null would have cascade-deleted their
`job_executions` and `job_logs` history along with them. They simply stop
matching any org-scoped query going forward — equivalent to being retired,
not deleted. Every worker row the application creates going forward always
sets it; see [design-decisions.md](design-decisions.md#workers-belong-to-exactly-one-organization).

### The indexes that actually matter

Most of the FK columns have an obvious supporting index and aren't worth
listing individually. A few carry real weight on the write path, though:

- `jobs(queue_id, status, priority DESC, run_at)` — this is the index the
  atomic-claim query runs on. Without it, claiming a job means scanning the
  whole table.
- `UNIQUE (queue_id, idempotency_key) WHERE idempotency_key IS NOT NULL` —
  idempotent submission isn't just an application-layer promise, the
  database physically won't allow a duplicate.
- `worker_heartbeats(worker_id, reported_at DESC)` — "is this worker still
  alive" is always a query for the single most recent row, so that's what
  the index is shaped for.
- `scheduled_jobs(next_run_at) WHERE is_active` — a partial index, since the
  scheduler only ever cares about active, due rows and there's no reason to
  index the rest.
- `workers(org_id)` — every worker-facing endpoint and the dashboard's
  online-worker count filter by it.

### What happens when something gets deleted

Most deletes cascade down the natural containment tree:
`organizations → projects → queues → jobs → job_executions → job_logs`,
plus `dead_letter_queue` and `worker_heartbeats` cascading with their
parent row. That's the right default here — an execution log with no job
to belong to isn't useful to anyone. The trade-off is that deleting a
project takes its whole audit history with it, which is fine for this
brief but flagged in [design-decisions.md](design-decisions.md) as the
first thing to change (to a `deleted_at` soft-delete) if this ever needed
to hold onto records for compliance reasons.

Three foreign keys deliberately don't follow that pattern:

- `queues.retry_policy_id → retry_policies` is `RESTRICT` — you can't
  delete a policy while a queue is still using it.
- `projects.owner_id → users` is `RESTRICT` — same idea, you can't delete a
  user who still owns a project.
- `jobs.scheduled_job_id` and `jobs.retry_policy_id` (the per-job override)
  are both `SET NULL` — a job that's already running should keep running
  even if the cron definition or the policy override behind it gets
  deleted later.
