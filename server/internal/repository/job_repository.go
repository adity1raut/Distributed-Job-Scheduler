package repository

import "github.com/adity1raut/job-scheduler/internal/models"

// JobRepository handles persistence and atomic claiming of jobs.
type JobRepository interface {
	Create(job *models.Job) error
	GetByID(id string) (*models.Job, error)
	ListByQueue(queueID string) ([]*models.Job, error)

	// ClaimNext atomically claims the next runnable job on a queue using
	// SELECT ... FOR UPDATE SKIP LOCKED so concurrent workers never double-claim.
	ClaimNext(workerID string, queueID string) (*models.Job, error)

	Heartbeat(jobID string, workerID string) error
	MarkSucceeded(jobID string) error
	MarkFailed(jobID string, requeue bool) error
	ReapStale(staleSec int) (int, error)
}
