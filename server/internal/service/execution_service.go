package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

var fallbackRetryPolicy = models.RetryPolicy{
	Strategy:    models.RetryExponential,
	BaseDelayMS: 5000,
	MaxDelayMS:  60000,
	MaxAttempts: 5,
	Multiplier:  2,
}

// ExecutionService is the worker-side counterpart to JobService: it owns
// the claim → run → complete/retry/dead-letter pipeline. It has no HTTP
// surface — cmd/worker's poll loop is the only caller.
type ExecutionService struct {
	jobs       *repository.JobRepository
	executions *repository.JobExecutionRepository
	logs       *repository.JobLogRepository
	dlq        *repository.DeadLetterRepository
	queues     *repository.QueueRepository
	policies   *repository.RetryPolicyRepository
}

func NewExecutionService(
	jobs *repository.JobRepository,
	executions *repository.JobExecutionRepository,
	logs *repository.JobLogRepository,
	dlq *repository.DeadLetterRepository,
	queues *repository.QueueRepository,
	policies *repository.RetryPolicyRepository,
) *ExecutionService {
	return &ExecutionService{jobs: jobs, executions: executions, logs: logs, dlq: dlq, queues: queues, policies: policies}
}

// Claim atomically claims the next runnable job on queueID, or returns
// repository.ErrNoJobAvailable if there's nothing to do right now.
func (s *ExecutionService) Claim(ctx context.Context, queueID, workerID uuid.UUID) (*models.Job, error) {
	return s.jobs.ClaimNext(ctx, queueID, workerID.String())
}

// Run executes one attempt of an already-claimed job to completion: marks
// it running, records the execution and its logs, then resolves success,
// backoff-retry, or dead-letter based on the applicable retry policy.
func (s *ExecutionService) Run(ctx context.Context, job *models.Job, workerID uuid.UUID) {
	if err := s.jobs.MarkRunning(ctx, job.ID); err != nil {
		slog.Error("mark running failed", "job_id", job.ID, "error", err)
		return
	}

	exec, err := s.executions.Start(ctx, job.ID, workerID, job.Attempts)
	if err != nil {
		slog.Error("start execution failed", "job_id", job.ID, "error", err)
		return
	}

	start := time.Now()
	runErr := RunPayload(job.Payload)
	durationMS := int(time.Since(start).Milliseconds())

	if runErr == nil {
		_ = s.logs.Append(ctx, exec.ID, models.LogInfo, "execution succeeded")
		if err := s.executions.Finish(ctx, exec.ID, models.ExecutionSucceeded, nil, durationMS); err != nil {
			slog.Error("finish execution failed", "job_id", job.ID, "error", err)
		}
		if err := s.jobs.CompleteSuccess(ctx, job.ID); err != nil {
			slog.Error("complete job failed", "job_id", job.ID, "error", err)
		}
		return
	}

	errMsg := runErr.Error()
	_ = s.logs.Append(ctx, exec.ID, models.LogError, errMsg)
	if err := s.executions.Finish(ctx, exec.ID, models.ExecutionFailed, &errMsg, durationMS); err != nil {
		slog.Error("finish execution failed", "job_id", job.ID, "error", err)
	}

	policy := s.resolveRetryPolicy(ctx, job)
	if job.Attempts < policy.MaxAttempts {
		delay := policy.NextDelay(job.Attempts)
		if err := s.jobs.RequeueForRetry(ctx, job.ID, time.Now().Add(delay), errMsg); err != nil {
			slog.Error("requeue failed", "job_id", job.ID, "error", err)
		}
		return
	}

	dead, err := s.jobs.MoveToDead(ctx, job.ID, errMsg)
	if err != nil {
		slog.Error("move to dead failed", "job_id", job.ID, "error", err)
		return
	}
	if err := s.dlq.Create(ctx, dead.ID, errMsg, dead.Payload); err != nil {
		slog.Error("dead-letter insert failed", "job_id", job.ID, "error", err)
	}
}

func (s *ExecutionService) resolveRetryPolicy(ctx context.Context, job *models.Job) models.RetryPolicy {
	policyID := job.RetryPolicyID
	if policyID == nil {
		queue, err := s.queues.GetByIDInternal(ctx, job.QueueID)
		if err != nil {
			return fallbackRetryPolicy
		}
		policyID = &queue.RetryPolicyID
	}

	policy, err := s.policies.GetByIDInternal(ctx, *policyID)
	if err != nil {
		return fallbackRetryPolicy
	}
	return *policy
}
