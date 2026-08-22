package service

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/google/uuid"
)

type ProjectService struct {
	projects *repository.ProjectRepository
}

func NewProjectService(projects *repository.ProjectRepository) *ProjectService {
	return &ProjectService{projects: projects}
}

func (s *ProjectService) Create(ctx context.Context, orgID, ownerID uuid.UUID, name string) (*models.Project, error) {
	if name == "" {
		return nil, apperr.BadRequest("name is required")
	}
	project, err := s.projects.Create(ctx, orgID, ownerID, name)
	if err != nil {
		return nil, apperr.Internal("failed to create project")
	}
	return project, nil
}

func (s *ProjectService) List(ctx context.Context, orgID uuid.UUID) ([]models.Project, error) {
	projects, err := s.projects.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, apperr.Internal("failed to list projects")
	}
	return projects, nil
}

func (s *ProjectService) Get(ctx context.Context, orgID, id uuid.UUID) (*models.Project, error) {
	project, err := s.projects.GetByID(ctx, orgID, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("project")
		}
		return nil, apperr.Internal("failed to fetch project")
	}
	return project, nil
}

func (s *ProjectService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	if err := s.projects.Delete(ctx, orgID, id); err != nil {
		if err == repository.ErrNotFound {
			return apperr.NotFound("project")
		}
		return apperr.Internal("failed to delete project")
	}
	return nil
}
