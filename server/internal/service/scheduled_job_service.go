package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type ScheduledJobService struct {
	scheduledJobs *repository.ScheduledJobRepository
	queues        *repository.QueueRepository
}

func NewScheduledJobService(scheduledJobs *repository.ScheduledJobRepository, queues *repository.QueueRepository) *ScheduledJobService {
	return &ScheduledJobService{scheduledJobs: scheduledJobs, queues: queues}
}

func (s *ScheduledJobService) Create(ctx context.Context, orgID, queueID uuid.UUID, cronExpr string, payloadTemplate json.RawMessage) (*models.ScheduledJob, error) {
	if _, err := s.queues.GetByID(ctx, orgID, queueID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}

	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return nil, apperr.BadRequest("invalid cron expression: " + err.Error())
	}
	if len(payloadTemplate) == 0 {
		payloadTemplate = json.RawMessage(`{}`)
	}

	sj, err := s.scheduledJobs.Create(ctx, &models.ScheduledJob{
		QueueID:         queueID,
		CronExpression:  cronExpr,
		PayloadTemplate: payloadTemplate,
		NextRunAt:       schedule.Next(time.Now()),
	})
	if err != nil {
		return nil, apperr.Internal("failed to create scheduled job")
	}
	return sj, nil
}

// List verifies the queue belongs to the caller's org first.
func (s *ScheduledJobService) List(ctx context.Context, orgID, queueID uuid.UUID) ([]models.ScheduledJob, error) {
	if _, err := s.queues.GetByID(ctx, orgID, queueID); err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("queue")
		}
		return nil, apperr.Internal("failed to verify queue")
	}
	list, err := s.scheduledJobs.ListByQueue(ctx, queueID)
	if err != nil {
		return nil, apperr.Internal("failed to list scheduled jobs")
	}
	return list, nil
}

func (s *ScheduledJobService) SetActive(ctx context.Context, orgID, id uuid.UUID, active bool) error {
	if err := s.scheduledJobs.SetActive(ctx, orgID, id, active); err != nil {
		if err == repository.ErrNotFound {
			return apperr.NotFound("scheduled job")
		}
		return apperr.Internal("failed to update scheduled job")
	}
	return nil
}
