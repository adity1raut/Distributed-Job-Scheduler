package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueueRepository struct {
	pool *pgxpool.Pool
}

func NewQueueRepository(pool *pgxpool.Pool) *QueueRepository {
	return &QueueRepository{pool: pool}
}

func (r *QueueRepository) Create(ctx context.Context, q *models.Queue) (*models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO queues (project_id, retry_policy_id, name, priority, concurrency_limit)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, retry_policy_id, name, priority, concurrency_limit, is_paused, created_at`,
		q.ProjectID, q.RetryPolicyID, q.Name, q.Priority, q.ConcurrencyLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Queue])
}

func (r *QueueRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, retry_policy_id, name, priority, concurrency_limit, is_paused, created_at
		 FROM queues WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	q, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Queue])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

func (r *QueueRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, retry_policy_id, name, priority, concurrency_limit, is_paused, created_at
		 FROM queues WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Queue])
}

func (r *QueueRepository) UpdateConfig(ctx context.Context, id uuid.UUID, priority, concurrencyLimit *int, retryPolicyID *uuid.UUID) (*models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE queues SET
		   priority = COALESCE($2, priority),
		   concurrency_limit = COALESCE($3, concurrency_limit),
		   retry_policy_id = COALESCE($4, retry_policy_id)
		 WHERE id = $1
		 RETURNING id, project_id, retry_policy_id, name, priority, concurrency_limit, is_paused, created_at`,
		id, priority, concurrencyLimit, retryPolicyID)
	if err != nil {
		return nil, err
	}
	q, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Queue])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

func (r *QueueRepository) SetPaused(ctx context.Context, id uuid.UUID, paused bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE queues SET is_paused = $2 WHERE id = $1`, id, paused)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *QueueRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM queues WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *QueueRepository) Stats(ctx context.Context, id uuid.UUID) (*models.QueueStats, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
		   count(*) FILTER (WHERE status = 'scheduled') AS scheduled,
		   count(*) FILTER (WHERE status = 'queued')    AS queued,
		   count(*) FILTER (WHERE status = 'claimed')   AS claimed,
		   count(*) FILTER (WHERE status = 'running')   AS running,
		   count(*) FILTER (WHERE status = 'completed') AS completed,
		   count(*) FILTER (WHERE status = 'failed')    AS failed,
		   count(*) FILTER (WHERE status = 'dead')      AS dead
		 FROM jobs WHERE queue_id = $1`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.QueueStats])
}
