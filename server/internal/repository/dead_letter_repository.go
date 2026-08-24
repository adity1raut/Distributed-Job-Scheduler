package repository

import (
	"context"
	"encoding/json"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeadLetterRepository struct {
	pool *pgxpool.Pool
}

func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{pool: pool}
}

func (r *DeadLetterRepository) Create(ctx context.Context, jobID uuid.UUID, finalErr string, payloadSnapshot json.RawMessage) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO dead_letter_queue (job_id, final_error, payload_snapshot) VALUES ($1, $2, $3)`,
		jobID, finalErr, payloadSnapshot)
	return err
}

// ListByQueue joins through jobs/queues/projects to scope by orgID.
func (r *DeadLetterRepository) ListByQueue(ctx context.Context, orgID, queueID uuid.UUID, limit int) ([]models.DeadLetterEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.job_id, d.final_error, d.payload_snapshot, d.failed_at, d.replayed
		FROM dead_letter_queue d
		JOIN jobs j ON j.id = d.job_id
		JOIN queues q ON q.id = j.queue_id
		JOIN projects p ON p.id = q.project_id
		WHERE j.queue_id = $1 AND p.org_id = $2
		ORDER BY d.failed_at DESC
		LIMIT $3`, queueID, orgID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.DeadLetterEntry])
}

// GetByID joins through jobs/queues/projects to scope by orgID.
func (r *DeadLetterRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.DeadLetterEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.job_id, d.final_error, d.payload_snapshot, d.failed_at, d.replayed
		FROM dead_letter_queue d
		JOIN jobs j ON j.id = d.job_id
		JOIN queues q ON q.id = j.queue_id
		JOIN projects p ON p.id = q.project_id
		WHERE d.id = $1 AND p.org_id = $2`, id, orgID)
	if err != nil {
		return nil, err
	}
	entry, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.DeadLetterEntry])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return entry, nil
}

func (r *DeadLetterRepository) MarkReplayed(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE dead_letter_queue SET replayed = true WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
