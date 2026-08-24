package service

import (
	"context"
	"time"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardOverview struct {
	TotalProjects   int64 `json:"total_projects"`
	TotalQueues     int64 `json:"total_queues"`
	QueuedJobs      int64 `json:"queued_jobs"`
	RunningJobs     int64 `json:"running_jobs"`
	CompletedJobs24 int64 `json:"completed_jobs_24h"`
	FailedJobs24    int64 `json:"failed_jobs_24h"`
	DeadJobs        int64 `json:"dead_jobs"`
	OnlineWorkers   int64 `json:"online_workers"`
}

// RecentJob is a job row enriched with the queue/project it belongs to, so
// the dashboard can show "what happened" without a client-side join across
// every project's queues.
type RecentJob struct {
	ID          uuid.UUID        `json:"id" db:"id"`
	QueueID     uuid.UUID        `json:"queue_id" db:"queue_id"`
	QueueName   string           `json:"queue_name" db:"queue_name"`
	ProjectID   uuid.UUID        `json:"project_id" db:"project_id"`
	ProjectName string           `json:"project_name" db:"project_name"`
	Type        models.JobType   `json:"type" db:"type"`
	Status      models.JobStatus `json:"status" db:"status"`
	Attempts    int              `json:"attempts" db:"attempts"`
	MaxAttempts int              `json:"max_attempts" db:"max_attempts"`
	RunAt       time.Time        `json:"run_at" db:"run_at"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`
}

// DashboardService runs cross-entity aggregate queries for the landing
// view, scoped to one organization via the projects/queues/jobs chain.
type DashboardService struct {
	pool *pgxpool.Pool
}

func NewDashboardService(pool *pgxpool.Pool) *DashboardService {
	return &DashboardService{pool: pool}
}

func (s *DashboardService) Overview(ctx context.Context, orgID uuid.UUID) (*DashboardOverview, error) {
	var o DashboardOverview
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM projects WHERE org_id = $1),
			(SELECT count(*) FROM queues q JOIN projects p ON p.id = q.project_id WHERE p.org_id = $1),
			(SELECT count(*) FROM jobs j JOIN queues q ON q.id = j.queue_id JOIN projects p ON p.id = q.project_id
			   WHERE p.org_id = $1 AND j.status = 'queued'),
			(SELECT count(*) FROM jobs j JOIN queues q ON q.id = j.queue_id JOIN projects p ON p.id = q.project_id
			   WHERE p.org_id = $1 AND j.status = 'running'),
			(SELECT count(*) FROM jobs j JOIN queues q ON q.id = j.queue_id JOIN projects p ON p.id = q.project_id
			   WHERE p.org_id = $1 AND j.status = 'completed' AND j.updated_at > now() - interval '24 hours'),
			(SELECT count(*) FROM jobs j JOIN queues q ON q.id = j.queue_id JOIN projects p ON p.id = q.project_id
			   WHERE p.org_id = $1 AND j.status = 'failed' AND j.updated_at > now() - interval '24 hours'),
			(SELECT count(*) FROM jobs j JOIN queues q ON q.id = j.queue_id JOIN projects p ON p.id = q.project_id
			   WHERE p.org_id = $1 AND j.status = 'dead'),
			(SELECT count(*) FROM workers WHERE status = 'online' AND org_id = $1)
	`, orgID).Scan(
		&o.TotalProjects, &o.TotalQueues, &o.QueuedJobs, &o.RunningJobs,
		&o.CompletedJobs24, &o.FailedJobs24, &o.DeadJobs, &o.OnlineWorkers,
	)
	if err != nil {
		return nil, apperr.Internal("failed to compute dashboard overview")
	}
	return &o, nil
}

// RecentJobs returns the most recently created jobs across every queue in
// the org, newest first — the org-wide equivalent of a queue's job list, for
// a landing view that shouldn't require drilling into a specific queue.
// An empty status matches every status.
func (s *DashboardService) RecentJobs(ctx context.Context, orgID uuid.UUID, status models.JobStatus, limit int) ([]RecentJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			j.id, j.queue_id, q.name AS queue_name, p.id AS project_id, p.name AS project_name,
			j.type, j.status, j.attempts, j.max_attempts, j.run_at, j.created_at, j.updated_at
		FROM jobs j
		JOIN queues q ON q.id = j.queue_id
		JOIN projects p ON p.id = q.project_id
		WHERE p.org_id = $1 AND ($2 = '' OR j.status = $2)
		ORDER BY j.created_at DESC
		LIMIT $3`, orgID, status, limit)
	if err != nil {
		return nil, apperr.Internal("failed to fetch recent jobs")
	}
	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[RecentJob])
	if err != nil {
		return nil, apperr.Internal("failed to fetch recent jobs")
	}
	return jobs, nil
}

// ThroughputBucket is one hour's worth of finished-job counts — the
// "visualize throughput and system health" requirement, as a time series
// instead of a single rolled-up number.
type ThroughputBucket struct {
	Hour      time.Time `json:"hour" db:"hour"`
	Completed int64     `json:"completed" db:"completed"`
	Failed    int64     `json:"failed" db:"failed"`
}

// Throughput returns one bucket per hour for the last 24 hours, oldest
// first, zero-filled for hours with no activity — generate_series supplies
// every hour up front so the chart's x-axis is always a continuous 24-hour
// strip, not just whichever hours happened to have a finished job.
func (s *DashboardService) Throughput(ctx context.Context, orgID uuid.UUID) ([]ThroughputBucket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			gs.hour,
			COALESCE(SUM(CASE WHEN j.status = 'completed' THEN 1 ELSE 0 END), 0) AS completed,
			COALESCE(SUM(CASE WHEN j.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed
		FROM generate_series(
			date_trunc('hour', now()) - interval '23 hours',
			date_trunc('hour', now()),
			interval '1 hour'
		) AS gs(hour)
		LEFT JOIN jobs j
			ON date_trunc('hour', j.updated_at) = gs.hour
			AND j.status IN ('completed', 'failed')
			AND j.queue_id IN (
				SELECT q.id FROM queues q JOIN projects p ON p.id = q.project_id WHERE p.org_id = $1
			)
		GROUP BY gs.hour
		ORDER BY gs.hour`, orgID)
	if err != nil {
		return nil, apperr.Internal("failed to compute throughput")
	}
	buckets, err := pgx.CollectRows(rows, pgx.RowToStructByName[ThroughputBucket])
	if err != nil {
		return nil, apperr.Internal("failed to compute throughput")
	}
	return buckets, nil
}
