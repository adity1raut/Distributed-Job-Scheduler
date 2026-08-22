package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobExecutionRepository struct {
	pool *pgxpool.Pool
}

func NewJobExecutionRepository(pool *pgxpool.Pool) *JobExecutionRepository {
	return &JobExecutionRepository{pool: pool}
}

func (r *JobExecutionRepository) Start(ctx context.Context, jobID, workerID uuid.UUID, attemptNumber int) (*models.JobExecution, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO job_executions (job_id, worker_id, attempt_number, status)
		 VALUES ($1, $2, $3, 'running')
		 RETURNING id, job_id, worker_id, attempt_number, status, started_at, finished_at, error_message, duration_ms`,
		jobID, workerID, attemptNumber)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.JobExecution])
}

func (r *JobExecutionRepository) Finish(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, errMsg *string, durationMS int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE job_executions SET status = $2, finished_at = now(), error_message = $3, duration_ms = $4 WHERE id = $1`,
		id, status, errMsg, durationMS)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *JobExecutionRepository) ListByJob(ctx context.Context, jobID uuid.UUID) ([]models.JobExecution, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, job_id, worker_id, attempt_number, status, started_at, finished_at, error_message, duration_ms
		 FROM job_executions WHERE job_id = $1 ORDER BY attempt_number`, jobID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.JobExecution])
}
