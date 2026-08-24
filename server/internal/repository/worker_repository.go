package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerRepository struct {
	pool *pgxpool.Pool
}

func NewWorkerRepository(pool *pgxpool.Pool) *WorkerRepository {
	return &WorkerRepository{pool: pool}
}

func (r *WorkerRepository) Register(ctx context.Context, hostname string, orgID uuid.UUID) (*models.Worker, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO workers (hostname, org_id, status) VALUES ($1, $2, 'online')
		 RETURNING id, org_id, hostname, status, started_at`, hostname, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Worker])
}

func (r *WorkerRepository) SetOffline(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE workers SET status = 'offline' WHERE id = $1`, id)
	return err
}

func (r *WorkerRepository) Heartbeat(ctx context.Context, workerID uuid.UUID, activeJobCount int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO worker_heartbeats (worker_id, active_job_count) VALUES ($1, $2)`,
		workerID, activeJobCount)
	return err
}

// ListWithStatus returns every worker belonging to orgID, joined with its
// most recent heartbeat; a worker is "stale" if that heartbeat is older
// than staleSec (or missing).
func (r *WorkerRepository) ListWithStatus(ctx context.Context, orgID uuid.UUID, staleSec int) ([]models.WorkerWithStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			w.id, w.hostname, w.status, w.started_at,
			hb.reported_at AS last_heartbeat_at,
			hb.active_job_count,
			(hb.reported_at IS NULL OR hb.reported_at < now() - make_interval(secs => $2)) AS is_stale
		FROM workers w
		LEFT JOIN LATERAL (
			SELECT reported_at, active_job_count
			FROM worker_heartbeats
			WHERE worker_id = w.id
			ORDER BY reported_at DESC
			LIMIT 1
		) hb ON true
		WHERE w.org_id = $1
		ORDER BY w.started_at DESC`, orgID, staleSec)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.WorkerWithStatus])
}

// HeartbeatHistory returns heartbeats for workerID, scoped to orgID so one
// org can't page through another org's worker by guessing its UUID.
func (r *WorkerRepository) HeartbeatHistory(ctx context.Context, workerID, orgID uuid.UUID, limit int) ([]models.WorkerHeartbeat, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT wh.id, wh.worker_id, wh.reported_at, wh.active_job_count
		 FROM worker_heartbeats wh
		 JOIN workers w ON w.id = wh.worker_id
		 WHERE wh.worker_id = $1 AND w.org_id = $2
		 ORDER BY wh.reported_at DESC LIMIT $3`,
		workerID, orgID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.WorkerHeartbeat])
}
