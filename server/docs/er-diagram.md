# Entity-Relationship Diagram

All twelve entities the brief names: Users, Organizations, Projects, Queues,
Jobs, Job Executions, Retry Policies, Workers, Worker Heartbeats, Job Logs,
Scheduled Jobs, Dead Letter Queue.

![Entity-relationship diagram of all twelve tables: organizations own users, projects, and retry policies; projects contain queues; queues hold jobs and define scheduled jobs which spawn more jobs; jobs produce job executions which emit job logs and can terminate in the dead letter queue; workers execute job executions and report worker heartbeats.](images/er-diagram.png)

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
  `dead_letter_queue` and `worker_heartbeats` cascade with their parent
  (`jobs`, `workers`) too. Trade-off: deleting a project destroys its audit
  history; noted in [design-decisions.md](design-decisions.md) as the first
  thing to swap for a `deleted_at` soft-delete in a compliance-sensitive
  deployment.
- **Not every FK cascades** — three are deliberately `RESTRICT` or
  `SET NULL` instead:
  - `queues.retry_policy_id → retry_policies` is `RESTRICT` — a policy can't
    be deleted while a queue still references it, so an in-use retry
    strategy never silently disappears out from under a queue.
  - `projects.owner_id → users` is `RESTRICT` — a user can't be deleted
    while they still own a project.
  - `jobs.scheduled_job_id → scheduled_jobs` and `jobs.retry_policy_id →
    retry_policies` (the per-job override) are both `SET NULL` — a job
    that already exists keeps running even if the cron definition or
    policy override that created it is later removed.
