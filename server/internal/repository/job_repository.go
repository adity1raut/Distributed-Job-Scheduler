package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoJobAvailable = errors.New("no claimable job on this queue")

const jobColumns = `id, queue_id, scheduled_job_id, retry_policy_id, type, status, payload,
	idempotency_key, priority, attempts, max_attempts, batch_id, run_at,
	locked_by, locked_at, last_error, created_at, updated_at`

type JobRepository struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

func (r *JobRepository) Create(ctx context.Context, j *models.Job) (*models.Job, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		INSERT INTO jobs (queue_id, scheduled_job_id, retry_policy_id, type, payload,
			idempotency_key, priority, max_attempts, batch_id, run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (queue_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING %s`, jobColumns),
		j.QueueID, j.ScheduledJobID, j.RetryPolicyID, j.Type, j.Payload,
		j.IdempotencyKey, j.Priority, j.MaxAttempts, j.BatchID, j.RunAt)
	if err != nil {
		return nil, err
	}
	created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
	if err == nil {
		return created, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	// ON CONFLICT DO NOTHING means an identical idempotency key already
	// exists for this queue — return that row instead of erroring, so a
	// client retrying a submit call is safe.
	if j.IdempotencyKey == nil {
		return nil, err
	}
	return r.getByIdempotencyKey(ctx, j.QueueID, *j.IdempotencyKey)
}

func (r *JobRepository) getByIdempotencyKey(ctx context.Context, queueID uuid.UUID, key string) (*models.Job, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM jobs WHERE queue_id = $1 AND idempotency_key = $2`, jobColumns),
		queueID, key)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
}

func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1`, jobColumns), id)
	if err != nil {
		return nil, err
	}
	j, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return j, nil
}

type JobFilter struct {
	QueueID uuid.UUID
	Status  *models.JobStatus
	Type    *models.JobType
	From    *time.Time
	To      *time.Time
	Cursor  *httpx.Cursor
	Limit   int
}

// List returns jobs newest-first with a keyset cursor on (created_at, id),
// which stays a cheap index seek on the idx_jobs_claim-adjacent covering
// index no matter how deep the page is — unlike OFFSET, which rescans
// every prior row.
func (r *JobRepository) List(ctx context.Context, f JobFilter) ([]models.Job, string, error) {
	var conditions []string
	args := []any{f.QueueID}
	conditions = append(conditions, "queue_id = $1")

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Type != nil {
		args = append(args, *f.Type)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if f.Cursor != nil {
		args = append(args, f.Cursor.CreatedAt, f.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	limit := httpx.ClampLimit(f.Limit)
	args = append(args, limit+1)

	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		jobColumns, strings.Join(conditions, " AND "), len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Job])
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(jobs) > limit {
		last := jobs[limit-1]
		nextCursor = httpx.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
		jobs = jobs[:limit]
	}
	return jobs, nextCursor, nil
}

// ClaimNext atomically claims the highest-priority runnable job on a queue.
// The queue row is locked first so the concurrency_limit check and the
// SKIP LOCKED claim happen inside one serialized window per queue — two
// workers claiming from *different* queues never block each other.
func (r *JobRepository) ClaimNext(ctx context.Context, queueID uuid.UUID, workerID string) (*models.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var concurrencyLimit int
	var isPaused bool
	err = tx.QueryRow(ctx, `SELECT concurrency_limit, is_paused FROM queues WHERE id = $1 FOR UPDATE`, queueID).
		Scan(&concurrencyLimit, &isPaused)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if isPaused {
		return nil, ErrNoJobAvailable
	}

	var inFlight int
	err = tx.QueryRow(ctx,
		`SELECT count(*) FROM jobs WHERE queue_id = $1 AND status IN ('claimed', 'running')`, queueID).
		Scan(&inFlight)
	if err != nil {
		return nil, err
	}
	if inFlight >= concurrencyLimit {
		return nil, ErrNoJobAvailable
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		UPDATE jobs SET
			status = 'claimed',
			locked_by = $2,
			locked_at = now(),
			attempts = attempts + 1,
			updated_at = now()
		WHERE id = (
			SELECT id FROM jobs
			WHERE queue_id = $1 AND status = 'queued' AND run_at <= now()
			ORDER BY priority DESC, run_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING %s`, jobColumns), queueID, workerID)
	if err != nil {
		return nil, err
	}
	job, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNoJobAvailable
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *JobRepository) MarkRunning(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE jobs SET status = 'running', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *JobRepository) CompleteSuccess(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'completed', locked_by = NULL, locked_at = NULL, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *JobRepository) RequeueForRetry(ctx context.Context, id uuid.UUID, nextRunAt time.Time, lastErr string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'queued', locked_by = NULL, locked_at = NULL,
		   run_at = $2, last_error = $3, updated_at = now()
		 WHERE id = $1`, id, nextRunAt, lastErr)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *JobRepository) MoveToDead(ctx context.Context, id uuid.UUID, lastErr string) (*models.Job, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		UPDATE jobs SET status = 'dead', locked_by = NULL, locked_at = NULL, last_error = $2, updated_at = now()
		WHERE id = $1
		RETURNING %s`, jobColumns), id, lastErr)
	if err != nil {
		return nil, err
	}
	job, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return job, nil
}

// Retry re-queues a job from failed/dead — used for a manual retry from the
// dashboard, distinct from the automatic backoff retry after a transient failure.
func (r *JobRepository) Retry(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		UPDATE jobs SET status = 'queued', attempts = 0, last_error = NULL,
			locked_by = NULL, locked_at = NULL, run_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING %s`, jobColumns), id)
	if err != nil {
		return nil, err
	}
	job, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Job])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return job, nil
}

// ReapStale requeues jobs stuck in claimed/running whose owning worker has
// sent no heartbeat inside staleSec — the worker most likely crashed.
// ReapStale requeues any claim whose worker has gone quiet. locked_by is a
// plain TEXT column (not a UUID FK — see design-decisions.md), so it's
// guarded with a format check before ever being cast: Postgres doesn't
// guarantee left-to-right evaluation of AND-ed WHERE clauses, so a bare
// `locked_by::uuid` can still get evaluated (and error out) on a
// non-UUID value even sitting behind a `locked_by IS NOT NULL` check,
// depending on the query plan the row count happens to pick. A value that
// isn't UUID-shaped at all can't belong to any real worker anyway, so it's
// treated as immediately reapable rather than crashing the whole tick.
func (r *JobRepository) ReapStale(ctx context.Context, staleSec int) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'queued', locked_by = NULL, locked_at = NULL, updated_at = now()
		WHERE status IN ('claimed', 'running')
		  AND locked_by IS NOT NULL
		  AND locked_at < now() - make_interval(secs => $1)
		  AND (
		    locked_by !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
		    OR NOT EXISTS (
		      SELECT 1 FROM worker_heartbeats wh
		      WHERE wh.worker_id = jobs.locked_by::uuid
		        AND wh.reported_at > now() - make_interval(secs => $1)
		    )
		  )`, staleSec)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
