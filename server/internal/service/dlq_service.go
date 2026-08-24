package service

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

type DLQService struct {
	dlq    *repository.DeadLetterRepository
	jobs   *repository.JobRepository
	queues *repository.QueueRepository
}

func NewDLQService(dlq *repository.DeadLetterRepository, jobs *repository.JobRepository, queues *repository.QueueRepository) *DLQService {
	return &DLQService{dlq: dlq, jobs: jobs, queues: queues}
}

// List verifies the queue belongs to the caller's org first.
func (s *DLQService) List(ctx context.Context, orgID, queueID uuid.UUID, limit int) ([]models.DeadLetterEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if _, err := s.queues.GetByID(ctx, orgID, queueID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}
	entries, err := s.dlq.ListByQueue(ctx, orgID, queueID, limit)
	if err != nil {
		return nil, apperr.Internal("failed to list dead-letter entries")
	}
	return entries, nil
}

// Replay resets the original job back to queued with a fresh attempt count
// and marks the dead-letter entry as replayed, rather than fabricating a
// second job row — the execution history stays attached to one job.
func (s *DLQService) Replay(ctx context.Context, orgID, entryID uuid.UUID) (*models.Job, error) {
	entry, err := s.dlq.GetByID(ctx, orgID, entryID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("dead-letter entry")
		}
		return nil, apperr.Internal("failed to fetch dead-letter entry")
	}

	job, err := s.jobs.Retry(ctx, orgID, entry.JobID)
	if err != nil {
		return nil, apperr.Internal("failed to requeue job")
	}

	if err := s.dlq.MarkReplayed(ctx, entryID); err != nil {
		return nil, apperr.Internal("failed to mark entry replayed")
	}
	return job, nil
}
