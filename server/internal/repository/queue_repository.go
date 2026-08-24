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

// GetByID joins to projects to scope by orgID — a queue has no org_id column of its own.
func (r *QueueRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, q.project_id, q.retry_policy_id, q.name, q.priority, q.concurrency_limit, q.is_paused, q.created_at
		 FROM queues q
		 JOIN projects p ON p.id = q.project_id
		 WHERE q.id = $1 AND p.org_id = $2`, id, orgID)
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

// GetByIDInternal skips the org check — trusted-internal use only (ExecutionService), mirrors RetryPolicyRepository.GetByIDInternal.
func (r *QueueRepository) GetByIDInternal(ctx context.Context, id uuid.UUID) (*models.Queue, error) {
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

func (r *QueueRepository) UpdateConfig(ctx context.Context, orgID, id uuid.UUID, priority, concurrencyLimit *int, retryPolicyID *uuid.UUID) (*models.Queue, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE queues SET
		   priority = COALESCE($3, priority),
		   concurrency_limit = COALESCE($4, concurrency_limit),
		   retry_policy_id = COALESCE($5, retry_policy_id)
		 FROM projects
		 WHERE queues.id = $1 AND projects.id = queues.project_id AND projects.org_id = $2
		 RETURNING queues.id, queues.project_id, queues.retry_policy_id, queues.name,
		           queues.priority, queues.concurrency_limit, queues.is_paused, queues.created_at`,
		id, orgID, priority, concurrencyLimit, retryPolicyID)
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

func (r *QueueRepository) SetPaused(ctx context.Context, orgID, id uuid.UUID, paused bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE queues SET is_paused = $3
		FROM projects
		WHERE queues.id = $1 AND projects.id = queues.project_id AND projects.org_id = $2`,
		id, orgID, paused)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *QueueRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM queues
		USING projects
		WHERE queues.id = $1 AND projects.id = queues.project_id AND projects.org_id = $2`,
		id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Stats is org-unscoped by design: an aggregate with no GROUP BY always returns one row, so it
// can't signal "not found" for a foreign queue — the caller must verify ownership via GetByID first.
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
	stats, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.QueueStats])
	if err != nil {
		return nil, err
	}
	return stats, nil
}
