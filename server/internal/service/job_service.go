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
}

func NewJobService(jobs *repository.JobRepository, queues *repository.QueueRepository, executions *repository.JobExecutionRepository, logs *repository.JobLogRepository) *JobService {
	return &JobService{jobs: jobs, queues: queues, executions: executions, logs: logs}
}

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
func (s *JobService) Submit(ctx context.Context, queueID uuid.UUID, in SubmitJobInput) ([]models.Job, error) {
	if _, err := s.queues.GetByID(ctx, queueID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}
	if len(in.Payload) == 0 {
		in.Payload = json.RawMessage(`{}`)
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
			MaxAttempts:    5,
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

func (s *JobService) Get(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	job, err := s.jobs.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("job")
		}
		return nil, apperr.Internal("failed to fetch job")
	}
	return job, nil
}

func (s *JobService) Executions(ctx context.Context, jobID uuid.UUID) ([]models.JobExecution, error) {
	executions, err := s.executions.ListByJob(ctx, jobID)
	if err != nil {
		return nil, apperr.Internal("failed to fetch job executions")
	}
	return executions, nil
}

func (s *JobService) Logs(ctx context.Context, executionID uuid.UUID) ([]models.JobLog, error) {
	logs, err := s.logs.ListByExecution(ctx, executionID)
	if err != nil {
		return nil, apperr.Internal("failed to fetch execution logs")
	}
	return logs, nil
}

func (s *JobService) List(ctx context.Context, filter repository.JobFilter) (*httpx.Page[models.Job], error) {
	jobs, next, err := s.jobs.List(ctx, filter)
	if err != nil {
		return nil, apperr.Internal("failed to list jobs")
	}
	return &httpx.Page[models.Job]{Items: jobs, NextCursor: next}, nil
}

func (s *JobService) Retry(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	job, err := s.jobs.Retry(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("job")
		}
		return nil, apperr.Internal("failed to retry job")
	}
	return job, nil
}
