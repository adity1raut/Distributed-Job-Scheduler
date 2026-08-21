package repository

import "github.com/adity1raut/job-scheduler/internal/models"

// ProjectRepository handles persistence for projects.
type ProjectRepository interface {
	Create(project *models.Project) error
	GetByID(id string) (*models.Project, error)
	ListByOwner(ownerID string) ([]*models.Project, error)
	Delete(id string) error
}
