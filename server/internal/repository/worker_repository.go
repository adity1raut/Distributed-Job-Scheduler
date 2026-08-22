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

func (r *WorkerRepository) Register(ctx context.Context, hostname string) (*models.Worker, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO workers (hostname, status) VALUES ($1, 'online')
		 RETURNING id, hostname, status, started_at`, hostname)
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

// ListWithStatus returns every worker joined with its most recent heartbeat;
// a worker is "stale" if that heartbeat is older than staleSec (or missing).
func (r *WorkerRepository) ListWithStatus(ctx context.Context, staleSec int) ([]models.WorkerWithStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			w.id, w.hostname, w.status, w.started_at,
			hb.reported_at AS last_heartbeat_at,
			hb.active_job_count,
			(hb.reported_at IS NULL OR hb.reported_at < now() - make_interval(secs => $1)) AS is_stale
		FROM workers w
		LEFT JOIN LATERAL (
			SELECT reported_at, active_job_count
			FROM worker_heartbeats
			WHERE worker_id = w.id
			ORDER BY reported_at DESC
			LIMIT 1
		) hb ON true
		ORDER BY w.started_at DESC`, staleSec)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.WorkerWithStatus])
}

func (r *WorkerRepository) HeartbeatHistory(ctx context.Context, workerID uuid.UUID, limit int) ([]models.WorkerHeartbeat, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, worker_id, reported_at, active_job_count
		 FROM worker_heartbeats WHERE worker_id = $1 ORDER BY reported_at DESC LIMIT $2`,
		workerID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.WorkerHeartbeat])
}
