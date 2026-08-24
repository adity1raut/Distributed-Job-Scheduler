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

// ListByExecution joins up through job_executions/jobs/queues/projects to scope by orgID.
func (r *JobLogRepository) ListByExecution(ctx context.Context, orgID, executionID uuid.UUID) ([]models.JobLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT jl.id, jl.job_execution_id, jl.logged_at, jl.level, jl.message
		FROM job_logs jl
		JOIN job_executions je ON je.id = jl.job_execution_id
		JOIN jobs j ON j.id = je.job_id
		JOIN queues q ON q.id = j.queue_id
		JOIN projects p ON p.id = q.project_id
		WHERE jl.job_execution_id = $1 AND p.org_id = $2
		ORDER BY jl.logged_at`, executionID, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.JobLog])
}
