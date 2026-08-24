package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/testutil"
)

func TestJobRepository_Create_IdempotencyKeyDedupes(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	jobRepo := repository.NewJobRepository(pool)

	key := "checkout-session-42"
	first, err := jobRepo.Create(ctx, &models.Job{
		QueueID: fx.QueueID, Type: models.JobTypeImmediate, Payload: []byte(`{}`),
		IdempotencyKey: &key, MaxAttempts: 5, RunAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	second, err := jobRepo.Create(ctx, &models.Job{
		QueueID: fx.QueueID, Type: models.JobTypeImmediate, Payload: []byte(`{}`),
		IdempotencyKey: &key, MaxAttempts: 5, RunAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("second create: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected the same job back for a repeated idempotency key, got %s and %s", first.ID, second.ID)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE queue_id = $1 AND idempotency_key = $2`, fx.QueueID, key).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row for idempotency key %q, found %d", key, count)
	}
}

func TestJobRepository_ClaimNext_RespectsPause(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	jobRepo := repository.NewJobRepository(pool)

	testutil.Exec(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts) VALUES ($1, 'immediate', 'queued', '{}', 5)`,
		fx.QueueID)
	testutil.Exec(t, ctx, pool, `UPDATE queues SET is_paused = true WHERE id = $1`, fx.QueueID)

	_, err := jobRepo.ClaimNext(ctx, fx.QueueID, "worker-1")
	if err != repository.ErrNoJobAvailable {
		t.Fatalf("expected ErrNoJobAvailable on a paused queue, got %v", err)
	}
}

func TestJobRepository_ClaimNext_RespectsConcurrencyLimit(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{ConcurrencyLimit: 1})
	jobRepo := repository.NewJobRepository(pool)

	testutil.Exec(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts) VALUES ($1, 'immediate', 'queued', '{}', 5)`,
		fx.QueueID)
	testutil.Exec(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts) VALUES ($1, 'immediate', 'queued', '{}', 5)`,
		fx.QueueID)

	first, err := jobRepo.ClaimNext(ctx, fx.QueueID, "worker-1")
	if err != nil {
		t.Fatalf("expected the first claim to succeed: %v", err)
	}

	if _, err := jobRepo.ClaimNext(ctx, fx.QueueID, "worker-2"); err != repository.ErrNoJobAvailable {
		t.Fatalf("expected ErrNoJobAvailable while at concurrency_limit=1 with one job in flight, got %v", err)
	}

	if err := jobRepo.CompleteSuccess(ctx, first.ID); err != nil {
		t.Fatalf("complete first job: %v", err)
	}

	if _, err := jobRepo.ClaimNext(ctx, fx.QueueID, "worker-2"); err != nil {
		t.Fatalf("expected a claim to succeed once the in-flight job completed and freed a slot: %v", err)
	}
}

func TestJobRepository_ReapStale_RequeuesJobsWithNoRecentHeartbeat(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	jobRepo := repository.NewJobRepository(pool)

	testutil.Exec(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts) VALUES ($1, 'immediate', 'queued', '{}', 5)`,
		fx.QueueID)

	crashedWorker := testutil.SeedWorker(t, ctx, pool)
	job, err := jobRepo.ClaimNext(ctx, fx.QueueID, crashedWorker.String())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Simulate a worker that claimed the job, sent no heartbeat, and crashed:
	// push locked_at into the past with no corresponding heartbeat row.
	testutil.Exec(t, ctx, pool, `UPDATE jobs SET locked_at = now() - interval '5 minutes' WHERE id = $1`, job.ID)

	reaped, err := jobRepo.ReapStale(ctx, 60) // stale threshold: 60s, claim is 5 minutes old
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if reaped != 1 {
		t.Fatalf("expected 1 job reaped, got %d", reaped)
	}

	after, err := jobRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("get after reap: %v", err)
	}
	if after.Status != models.JobStatusQueued {
		t.Fatalf("expected status %q after reaping, got %q", models.JobStatusQueued, after.Status)
	}
	if after.LockedBy != nil {
		t.Fatalf("expected locked_by to be cleared after reaping, got %v", *after.LockedBy)
	}
}

func TestJobRepository_ReapStale_LeavesJobsWithRecentHeartbeat(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	jobRepo := repository.NewJobRepository(pool)

	testutil.Exec(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts) VALUES ($1, 'immediate', 'queued', '{}', 5)`,
		fx.QueueID)

	aliveWorker := testutil.SeedWorker(t, ctx, pool)
	job, err := jobRepo.ClaimNext(ctx, fx.QueueID, aliveWorker.String())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	testutil.Exec(t, ctx, pool, `UPDATE jobs SET locked_at = now() - interval '5 minutes' WHERE id = $1`, job.ID)
	testutil.Exec(t, ctx, pool, `INSERT INTO worker_heartbeats (worker_id, reported_at) VALUES ($1, now())`, aliveWorker)

	reaped, err := jobRepo.ReapStale(ctx, 60)
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if reaped != 0 {
		t.Fatalf("expected 0 jobs reaped for a worker with a recent heartbeat, got %d", reaped)
	}

	after, err := jobRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("get after reap: %v", err)
	}
	if after.Status != models.JobStatusClaimed {
		t.Fatalf("expected job to remain %q, got %q", models.JobStatusClaimed, after.Status)
	}
}

// TestJobRepository_ReapStale_HandlesNonUUIDLockedBy is a regression test:
// locked_by is a plain TEXT column, not a UUID FK, so nothing stops it from
// holding a non-UUID value. ReapStale used to cast it straight to uuid with
// no guard, which crashed the whole reap query outright once Postgres's
// query planner happened to evaluate that cast before the "is this even
// UUID-shaped" check — which it's under no obligation to avoid, since SQL
// doesn't guarantee left-to-right evaluation of AND-ed WHERE clauses.
func TestJobRepository_ReapStale_HandlesNonUUIDLockedBy(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	jobRepo := repository.NewJobRepository(pool)

	jobID := testutil.ScanUUID(t, ctx, pool, `
		INSERT INTO jobs (queue_id, type, status, payload, max_attempts, locked_by, locked_at)
		VALUES ($1, 'immediate', 'claimed', '{}', 5, 'not-a-uuid', now() - interval '5 minutes')
		RETURNING id`, fx.QueueID)

	reaped, err := jobRepo.ReapStale(ctx, 60)
	if err != nil {
		t.Fatalf("reap should tolerate a non-UUID locked_by, got error: %v", err)
	}
	if reaped != 1 {
		t.Fatalf("expected the malformed-locked_by job to be reaped, got %d reaped", reaped)
	}

	after, err := jobRepo.GetByID(ctx, jobID)
	if err != nil {
		t.Fatalf("get after reap: %v", err)
	}
	if after.Status != models.JobStatusQueued {
		t.Fatalf("expected status %q after reaping, got %q", models.JobStatusQueued, after.Status)
	}
}
