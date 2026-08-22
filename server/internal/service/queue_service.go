package service

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

type QueueService struct {
	queues   *repository.QueueRepository
	projects *repository.ProjectRepository
	policies *repository.RetryPolicyRepository
}

func NewQueueService(queues *repository.QueueRepository, projects *repository.ProjectRepository, policies *repository.RetryPolicyRepository) *QueueService {
	return &QueueService{queues: queues, projects: projects, policies: policies}
}

type CreateQueueInput struct {
	Name             string
	Priority         int
	ConcurrencyLimit int
	RetryPolicyID    *uuid.UUID
}

func (s *QueueService) Create(ctx context.Context, orgID, projectID uuid.UUID, in CreateQueueInput) (*models.Queue, error) {
	if in.Name == "" {
		return nil, apperr.BadRequest("name is required")
	}
	if _, err := s.projects.GetByID(ctx, orgID, projectID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("project")
		}
		return nil, apperr.Internal("failed to verify project")
	}

	retryPolicyID := in.RetryPolicyID
	if retryPolicyID == nil {
		defaultPolicy, err := s.policies.EnsureDefault(ctx, orgID)
		if err != nil {
			return nil, apperr.Internal("failed to resolve default retry policy")
		}
		retryPolicyID = &defaultPolicy.ID
	}

	concurrencyLimit := in.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = 5
	}

	queue, err := s.queues.Create(ctx, &models.Queue{
		ProjectID:        projectID,
		RetryPolicyID:    *retryPolicyID,
		Name:             in.Name,
		Priority:         in.Priority,
		ConcurrencyLimit: concurrencyLimit,
	})
	if err != nil {
		return nil, apperr.Conflict("a queue with this name already exists in the project")
	}
	return queue, nil
}

func (s *QueueService) List(ctx context.Context, orgID, projectID uuid.UUID) ([]models.Queue, error) {
	if _, err := s.projects.GetByID(ctx, orgID, projectID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("project")
		}
		return nil, apperr.Internal("failed to verify project")
	}
	queues, err := s.queues.ListByProject(ctx, projectID)
	if err != nil {
		return nil, apperr.Internal("failed to list queues")
	}
	return queues, nil
}

func (s *QueueService) Get(ctx context.Context, id uuid.UUID) (*models.Queue, error) {
	queue, err := s.queues.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to fetch queue")
	}
	return queue, nil
}

type UpdateQueueInput struct {
	Priority         *int
	ConcurrencyLimit *int
	RetryPolicyID    *uuid.UUID
}

func (s *QueueService) UpdateConfig(ctx context.Context, id uuid.UUID, in UpdateQueueInput) (*models.Queue, error) {
	queue, err := s.queues.UpdateConfig(ctx, id, in.Priority, in.ConcurrencyLimit, in.RetryPolicyID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to update queue")
	}
	return queue, nil
}

func (s *QueueService) SetPaused(ctx context.Context, id uuid.UUID, paused bool) error {
	if err := s.queues.SetPaused(ctx, id, paused); err != nil {
		if err == repository.ErrNotFound {
			return apperr.NotFound("queue")
		}
		return apperr.Internal("failed to update queue")
	}
	return nil
}

func (s *QueueService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.queues.Delete(ctx, id); err != nil {
		if err == repository.ErrNotFound {
			return apperr.NotFound("queue")
		}
		return apperr.Internal("failed to delete queue")
	}
	return nil
}

func (s *QueueService) Stats(ctx context.Context, id uuid.UUID) (*models.QueueStats, error) {
	stats, err := s.queues.Stats(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to compute queue stats")
	}
	return stats, nil
}
