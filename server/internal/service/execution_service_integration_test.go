package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/adity1raut/job-scheduler/internal/testutil"
)

func TestExecutionService_Run_SuccessCompletesJob(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{})
	worker := testutil.SeedWorker(t, ctx, pool)

	jobRepo := repository.NewJobRepository(pool)
	execRepo := repository.NewJobExecutionRepository(pool)
	logRepo := repository.NewJobLogRepository(pool)
	dlqRepo := repository.NewDeadLetterRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	policyRepo := repository.NewRetryPolicyRepository(pool)
	execSvc := service.NewExecutionService(jobRepo, execRepo, logRepo, dlqRepo, queueRepo, policyRepo)

	created, err := jobRepo.Create(ctx, &models.Job{
		QueueID: fx.QueueID, Type: models.JobTypeImmediate, Payload: []byte(`{"task":"echo"}`),
		MaxAttempts: 5, RunAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	claimed, err := execSvc.Claim(ctx, fx.QueueID, worker)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	execSvc.Run(ctx, claimed, worker)

	final, err := jobRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after run: %v", err)
	}
	if final.Status != models.JobStatusCompleted {
		t.Fatalf("expected status %q, got %q (last_error=%v)", models.JobStatusCompleted, final.Status, final.LastError)
	}

	executions, err := execRepo.ListByJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected exactly 1 execution row, got %d", len(executions))
	}
	if executions[0].Status != models.ExecutionSucceeded {
		t.Fatalf("expected execution status %q, got %q", models.ExecutionSucceeded, executions[0].Status)
	}
	if executions[0].DurationMS == nil {
		t.Fatal("expected duration_ms to be recorded")
	}
}

func TestExecutionService_Run_RetriesThenDeadLetters(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()

	// A tight, fast retry policy so the test doesn't need to sleep for
	// real production-sized backoff windows.
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{
		RetryStrategy: "fixed", BaseDelayMS: 20, MaxDelayMS: 20, MaxAttempts: 2,
	})
	worker := testutil.SeedWorker(t, ctx, pool)

	jobRepo := repository.NewJobRepository(pool)
	execRepo := repository.NewJobExecutionRepository(pool)
	logRepo := repository.NewJobLogRepository(pool)
	dlqRepo := repository.NewDeadLetterRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	policyRepo := repository.NewRetryPolicyRepository(pool)
	execSvc := service.NewExecutionService(jobRepo, execRepo, logRepo, dlqRepo, queueRepo, policyRepo)

	created, err := jobRepo.Create(ctx, &models.Job{
		QueueID: fx.QueueID, Type: models.JobTypeImmediate, Payload: []byte(`{"task":"fail"}`),
		RetryPolicyID: &fx.PolicyID, MaxAttempts: 2, RunAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Attempt 1: fails, should requeue for retry rather than dead-letter
	// immediately (attempts=1 < max_attempts=2).
	claimed, err := execSvc.Claim(ctx, fx.QueueID, worker)
	if err != nil {
		t.Fatalf("claim (attempt 1): %v", err)
	}
	execSvc.Run(ctx, claimed, worker)

	afterFirst, err := jobRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after attempt 1: %v", err)
	}
	if afterFirst.Status != models.JobStatusQueued {
		t.Fatalf("expected status %q after a retryable failure, got %q", models.JobStatusQueued, afterFirst.Status)
	}
	if afterFirst.LastError == nil {
		t.Fatal("expected last_error to be set after a failed attempt")
	}
	if !afterFirst.RunAt.After(created.RunAt) {
		t.Fatalf("expected run_at to be pushed into the future by the retry backoff, original=%v got=%v", created.RunAt, afterFirst.RunAt)
	}

	// Wait out the backoff window, then attempt 2: fails again, and
	// attempts (2) now meets max_attempts (2) — should dead-letter.
	time.Sleep(40 * time.Millisecond)

	claimed2, err := execSvc.Claim(ctx, fx.QueueID, worker)
	if err != nil {
		t.Fatalf("claim (attempt 2): %v", err)
	}
	execSvc.Run(ctx, claimed2, worker)

	final, err := jobRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after attempt 2: %v", err)
	}
	if final.Status != models.JobStatusDead {
		t.Fatalf("expected status %q after exhausting retries, got %q", models.JobStatusDead, final.Status)
	}

	entries, err := dlqRepo.ListByQueue(ctx, fx.QueueID, 10)
	if err != nil {
		t.Fatalf("list dlq: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 dead-letter entry, got %d", len(entries))
	}
	if entries[0].JobID != created.ID {
		t.Fatalf("dead-letter entry points at job %s, want %s", entries[0].JobID, created.ID)
	}

	executions, err := execRepo.ListByJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 2 {
		t.Fatalf("expected 2 execution rows (one per attempt), got %d", len(executions))
	}
	for _, e := range executions {
		if e.Status != models.ExecutionFailed {
			t.Errorf("expected execution %s to be failed, got %q", e.ID, e.Status)
		}
	}
}
