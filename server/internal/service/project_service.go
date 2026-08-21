package service

import "github.com/adity1raut/job-scheduler/internal/repository"

// ProjectService contains business logic for project management.
type ProjectService struct {
	projects repository.ProjectRepository
}

func NewProjectService(projects repository.ProjectRepository) *ProjectService {
	return &ProjectService{projects: projects}
}
