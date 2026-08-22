package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobLogRepository struct {
	pool *pgxpool.Pool
}

func NewJobLogRepository(pool *pgxpool.Pool) *JobLogRepository {
	return &JobLogRepository{pool: pool}
}

func (r *JobLogRepository) Append(ctx context.Context, executionID uuid.UUID, level models.LogLevel, message string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO job_logs (job_execution_id, level, message) VALUES ($1, $2, $3)`,
		executionID, level, message)
	return err
}

func (r *JobLogRepository) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]models.JobLog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, job_execution_id, logged_at, level, message
		 FROM job_logs WHERE job_execution_id = $1 ORDER BY logged_at`, executionID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.JobLog])
}
