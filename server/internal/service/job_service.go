package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

type JobService struct {
	jobs       *repository.JobRepository
	queues     *repository.QueueRepository
	executions *repository.JobExecutionRepository
	logs       *repository.JobLogRepository
	policies   *repository.RetryPolicyRepository
}

func NewJobService(jobs *repository.JobRepository, queues *repository.QueueRepository, executions *repository.JobExecutionRepository, logs *repository.JobLogRepository, policies *repository.RetryPolicyRepository) *JobService {
	return &JobService{jobs: jobs, queues: queues, executions: executions, logs: logs, policies: policies}
}

// fallbackMaxAttempts mirrors ExecutionService's fallbackRetryPolicy — used
// only if the resolved policy can't be looked up, so job creation never
// fails outright over a policy read hiccup.
const fallbackMaxAttempts = 5

type SubmitJobInput struct {
	Type           models.JobType
	Payload        json.RawMessage
	IdempotencyKey *string
	Priority       int
	DelayMS        int
	RunAt          *time.Time
	RetryPolicyID  *uuid.UUID
	BatchCount     int
}

// Submit handles every job type the brief asks for except "recurring" —
// recurring jobs are cron *definitions* (ScheduledJobService), not job rows;
// the scheduler is what turns a due cron fire into a job row via this same
// queue.
func (s *JobService) Submit(ctx context.Context, orgID, queueID uuid.UUID, in SubmitJobInput) ([]models.Job, error) {
	queue, err := s.queues.GetByID(ctx, orgID, queueID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}
	if len(in.Payload) == 0 {
		in.Payload = json.RawMessage(`{}`)
	}

	// The job's max_attempts must reflect whichever policy will actually
	// govern it — a per-job override if given, else the queue's own policy
	// — so the UI's "attempts / max_attempts" and the point it dead-letters
	// always agree, even when someone isn't using the org's default policy.
	maxAttempts := fallbackMaxAttempts
	resolvePolicyID := queue.RetryPolicyID
	if in.RetryPolicyID != nil {
		resolvePolicyID = *in.RetryPolicyID
	}
	if policy, err := s.policies.GetByIDInternal(ctx, resolvePolicyID); err == nil {
		maxAttempts = policy.MaxAttempts
	}

	runAt := time.Now()
	switch in.Type {
	case models.JobTypeDelayed:
		if in.DelayMS <= 0 {
			return nil, apperr.BadRequest("delayed jobs require delay_ms > 0")
		}
		runAt = time.Now().Add(time.Duration(in.DelayMS) * time.Millisecond)
	case models.JobTypeScheduled:
		if in.RunAt == nil {
			return nil, apperr.BadRequest("scheduled jobs require run_at")
		}
		runAt = *in.RunAt
	case models.JobTypeImmediate, models.JobTypeBatch:
		// runs now
	default:
		return nil, apperr.BadRequest("type must be one of: immediate, delayed, scheduled, batch")
	}

	count := 1
	var batchID *uuid.UUID
	if in.Type == models.JobTypeBatch {
		if in.BatchCount < 2 {
			return nil, apperr.BadRequest("batch jobs require batch_count >= 2")
		}
		count = in.BatchCount
		id := uuid.New()
		batchID = &id
	}

	jobs := make([]models.Job, 0, count)
	for i := 0; i < count; i++ {
		key := in.IdempotencyKey
		// A shared idempotency key across a batch would collapse every
		// member into the first row via ON CONFLICT — scope it per index.
		if key != nil && count > 1 {
			scoped := *key + ":" + uuid.NewString()[:8]
			key = &scoped
		}

		job, err := s.jobs.Create(ctx, &models.Job{
			QueueID:        queueID,
			RetryPolicyID:  in.RetryPolicyID,
			Type:           in.Type,
			Payload:        in.Payload,
			IdempotencyKey: key,
			Priority:       in.Priority,
			MaxAttempts:    maxAttempts,
			BatchID:        batchID,
			RunAt:          runAt,
		})
		if err != nil {
			return nil, apperr.Internal("failed to create job")
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func (s *JobService) Get(ctx context.Context, orgID, id uuid.UUID) (*models.Job, error) {
	job, err := s.jobs.GetByID(ctx, orgID, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("job")
		}
		return nil, apperr.Internal("failed to fetch job")
	}
	return job, nil
}

// Executions trusts jobID with no further org check — the caller always calls Get first, which already verified it.
func (s *JobService) Executions(ctx context.Context, jobID uuid.UUID) ([]models.JobExecution, error) {
	executions, err := s.executions.ListByJob(ctx, jobID)
	if err != nil {
		return nil, apperr.Internal("failed to fetch job executions")
	}
	return executions, nil
}

func (s *JobService) Logs(ctx context.Context, orgID, executionID uuid.UUID) ([]models.JobLog, error) {
	logs, err := s.logs.ListByExecution(ctx, orgID, executionID)
	if err != nil {
		return nil, apperr.Internal("failed to fetch execution logs")
	}
	return logs, nil
}

// List verifies the queue belongs to the caller's org before running the listing query.
func (s *JobService) List(ctx context.Context, orgID uuid.UUID, filter repository.JobFilter) (*httpx.Page[models.Job], error) {
	if _, err := s.queues.GetByID(ctx, orgID, filter.QueueID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}
	jobs, next, err := s.jobs.List(ctx, filter)
	if err != nil {
		return nil, apperr.Internal("failed to list jobs")
	}
	return &httpx.Page[models.Job]{Items: jobs, NextCursor: next}, nil
}

func (s *JobService) Retry(ctx context.Context, orgID, id uuid.UUID) (*models.Job, error) {
	job, err := s.jobs.Retry(ctx, orgID, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("job")
		}
		return nil, apperr.Internal("failed to retry job")
	}
	return job, nil
}
