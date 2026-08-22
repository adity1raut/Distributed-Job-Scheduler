CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_projects_org ON projects(org_id);

CREATE TABLE retry_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    strategy TEXT NOT NULL CHECK (strategy IN ('fixed', 'linear', 'exponential')),
    base_delay_ms INT NOT NULL DEFAULT 5000,
    max_delay_ms INT NOT NULL DEFAULT 60000,
    max_attempts INT NOT NULL DEFAULT 5,
    multiplier NUMERIC NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE queues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    retry_policy_id UUID NOT NULL REFERENCES retry_policies(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    concurrency_limit INT NOT NULL DEFAULT 5,
    is_paused BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX idx_queues_project ON queues(project_id);

CREATE TABLE scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    queue_id UUID NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    cron_expression TEXT NOT NULL,
    payload_template JSONB NOT NULL DEFAULT '{}',
    next_run_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_scheduled_jobs_due ON scheduled_jobs(next_run_at) WHERE is_active;

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    queue_id UUID NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    scheduled_job_id UUID REFERENCES scheduled_jobs(id) ON DELETE SET NULL,
    retry_policy_id UUID REFERENCES retry_policies(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('immediate', 'delayed', 'scheduled', 'recurring', 'batch')),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('scheduled', 'queued', 'claimed', 'running', 'completed', 'failed', 'dead')),
    payload JSONB NOT NULL DEFAULT '{}',
    idempotency_key TEXT,
    priority INT NOT NULL DEFAULT 0,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    batch_id UUID,
    run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by TEXT,
    locked_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_claim ON jobs(queue_id, status, priority DESC, run_at);
CREATE INDEX idx_jobs_batch ON jobs(batch_id) WHERE batch_id IS NOT NULL;
CREATE UNIQUE INDEX idx_jobs_idempotency ON jobs(queue_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE workers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE job_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    worker_id UUID NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    error_message TEXT,
    duration_ms INT
);
CREATE INDEX idx_executions_job ON job_executions(job_id, attempt_number);
CREATE INDEX idx_executions_worker ON job_executions(worker_id);

CREATE TABLE job_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_execution_id UUID NOT NULL REFERENCES job_executions(id) ON DELETE CASCADE,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    level TEXT NOT NULL DEFAULT 'info' CHECK (level IN ('debug', 'info', 'warn', 'error')),
    message TEXT NOT NULL
);
CREATE INDEX idx_logs_execution ON job_logs(job_execution_id, logged_at);

CREATE TABLE worker_heartbeats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    worker_id UUID NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    reported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    active_job_count INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_heartbeats_worker ON worker_heartbeats(worker_id, reported_at DESC);

CREATE TABLE dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    final_error TEXT,
    payload_snapshot JSONB,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    replayed BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX idx_dlq_job ON dead_letter_queue(job_id);
