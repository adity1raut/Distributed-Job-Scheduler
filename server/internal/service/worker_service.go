package service

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

type WorkerService struct {
	workers  *repository.WorkerRepository
	staleSec int
}

func NewWorkerService(workers *repository.WorkerRepository, staleSec int) *WorkerService {
	return &WorkerService{workers: workers, staleSec: staleSec}
}

func (s *WorkerService) Fleet(ctx context.Context, orgID uuid.UUID) ([]models.WorkerWithStatus, error) {
	workers, err := s.workers.ListWithStatus(ctx, orgID, s.staleSec)
	if err != nil {
		return nil, apperr.Internal("failed to list workers")
	}
	return workers, nil
}

func (s *WorkerService) Heartbeats(ctx context.Context, workerID, orgID uuid.UUID, limit int) ([]models.WorkerHeartbeat, error) {
	if limit <= 0 {
		limit = 50
	}
	beats, err := s.workers.HeartbeatHistory(ctx, workerID, orgID, limit)
	if err != nil {
		return nil, apperr.Internal("failed to fetch heartbeat history")
	}
	return beats, nil
}
