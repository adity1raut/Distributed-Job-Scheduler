-- Workers were previously a single fleet shared across every organization —
-- any worker process could claim jobs from any org's queues, and the
-- dashboard's worker list/count leaked every org's worker hostnames to
-- every other org. This scopes each worker to exactly one organization.
--
-- Nullable, not backfilled: existing worker rows predate this column and
-- have no org they can be correctly assigned to. They simply won't match
-- any org-scoped query going forward (equivalent to being retired) rather
-- than being deleted, which would cascade-delete their job_executions and
-- job_logs history.
ALTER TABLE workers ADD COLUMN org_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX idx_workers_org ON workers(org_id);
