package repository

import "github.com/adity1raut/job-scheduler/internal/models"

// QueueRepository handles persistence for queues.
type QueueRepository interface {
	Create(queue *models.Queue) error
	GetByID(id string) (*models.Queue, error)
	ListByProject(projectID string) ([]*models.Queue, error)
	Delete(id string) error
}
