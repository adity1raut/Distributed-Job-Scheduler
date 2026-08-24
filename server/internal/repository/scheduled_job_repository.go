package repository

import (
	"context"
	"time"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduledJobRepository struct {
	pool *pgxpool.Pool
}

func NewScheduledJobRepository(pool *pgxpool.Pool) *ScheduledJobRepository {
	return &ScheduledJobRepository{pool: pool}
}

func (r *ScheduledJobRepository) Create(ctx context.Context, s *models.ScheduledJob) (*models.ScheduledJob, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO scheduled_jobs (queue_id, cron_expression, payload_template, next_run_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, queue_id, cron_expression, payload_template, next_run_at, is_active, created_at`,
		s.QueueID, s.CronExpression, s.PayloadTemplate, s.NextRunAt)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.ScheduledJob])
}

func (r *ScheduledJobRepository) ListByQueue(ctx context.Context, queueID uuid.UUID) ([]models.ScheduledJob, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, queue_id, cron_expression, payload_template, next_run_at, is_active, created_at
		 FROM scheduled_jobs WHERE queue_id = $1 ORDER BY created_at DESC`, queueID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.ScheduledJob])
}

// DueForDispatch returns active scheduled jobs whose next_run_at has passed,
// locking each row (SKIP LOCKED) so multiple API replicas ticking at once
// never dispatch the same cron fire twice even without the advisory lock.
func (r *ScheduledJobRepository) DueForDispatch(ctx context.Context, tx pgx.Tx, limit int) ([]models.ScheduledJob, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, queue_id, cron_expression, payload_template, next_run_at, is_active, created_at
		 FROM scheduled_jobs
		 WHERE is_active AND next_run_at <= now()
		 ORDER BY next_run_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.ScheduledJob])
}

func (r *ScheduledJobRepository) SetNextRunAt(ctx context.Context, tx pgx.Tx, id uuid.UUID, next time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET next_run_at = $2 WHERE id = $1`, id, next)
	return err
}

// SetActive joins through queues/projects to scope by orgID — a scheduled job has no org_id column of its own.
func (r *ScheduledJobRepository) SetActive(ctx context.Context, orgID, id uuid.UUID, active bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE scheduled_jobs SET is_active = $3
		FROM queues, projects
		WHERE scheduled_jobs.id = $1
		  AND queues.id = scheduled_jobs.queue_id
		  AND projects.id = queues.project_id
		  AND projects.org_id = $2`,
		id, orgID, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
