package service

import "github.com/adity1raut/job-scheduler/internal/repository"

// JobService contains business logic for job submission, retries, and dead-letter handling.
type JobService struct {
	jobs repository.JobRepository
}

func NewJobService(jobs repository.JobRepository) *JobService {
	return &JobService{jobs: jobs}
}
