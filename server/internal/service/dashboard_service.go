package service

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/google/uuid"
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
			(SELECT count(*) FROM workers WHERE status = 'online')
	`, orgID).Scan(
		&o.TotalProjects, &o.TotalQueues, &o.QueuedJobs, &o.RunningJobs,
		&o.CompletedJobs24, &o.FailedJobs24, &o.DeadJobs, &o.OnlineWorkers,
	)
	if err != nil {
		return nil, apperr.Internal("failed to compute dashboard overview")
	}
	return &o, nil
}
