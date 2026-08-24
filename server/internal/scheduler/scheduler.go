// Package scheduler dispatches due cron/recurring jobs onto their queues
// and reaps stale claims. It runs inside every API replica, but a Postgres
// advisory lock ensures only one replica's tick does work per cycle — see
// docs/design-decisions.md for why this avoids a dedicated lock service.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

// advisoryLockKey is an arbitrary fixed int64 — any value works as long as
// every replica uses the same one, since pg_try_advisory_lock keys are
// process-agnostic within the database.
const advisoryLockKey = 72176321

type Scheduler struct {
	pool          *pgxpool.Pool
	scheduledJobs *repository.ScheduledJobRepository
	jobs          *repository.JobRepository
	tickInterval  time.Duration
	staleSec      int

	stop chan struct{}
	done chan struct{}
}

func New(pool *pgxpool.Pool, scheduledJobs *repository.ScheduledJobRepository, jobs *repository.JobRepository, tickInterval time.Duration, staleSec int) *Scheduler {
	return &Scheduler{
		pool:          pool,
		scheduledJobs: scheduledJobs,
		jobs:          jobs,
		tickInterval:  tickInterval,
		staleSec:      staleSec,
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	var acquired bool
	if err := s.pool.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, advisoryLockKey).Scan(&acquired); err != nil {
		slog.Error("advisory lock check failed", "error", err)
		return
	}
	if !acquired {
		// Another API replica is ticking this cycle.
		return
	}
	defer func() {
		if _, err := s.pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey); err != nil {
			slog.Error("advisory unlock failed", "error", err)
		}
	}()

	s.dispatchDue(ctx)

	reaped, err := s.jobs.ReapStale(ctx, s.staleSec)
	if err != nil {
		slog.Error("reap stale jobs failed", "error", err)
	} else if reaped > 0 {
		slog.Info("reaped stale jobs", "count", reaped)
	}
}

func (s *Scheduler) dispatchDue(ctx context.Context) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		slog.Error("scheduler tx begin failed", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	due, err := s.scheduledJobs.DueForDispatch(ctx, tx, 50)
	if err != nil {
		slog.Error("fetch due scheduled jobs failed", "error", err)
		return
	}

	for _, sj := range due {
		schedule, err := cron.ParseStandard(sj.CronExpression)
		if err != nil {
			slog.Error("invalid cron expression, skipping", "scheduled_job_id", sj.ID, "error", err)
			continue
		}

		// max_attempts is read from the queue's current retry policy in the
		// same statement, not left to the column default — otherwise a
		// recurring job's displayed "attempts / max_attempts" (and the
		// point it actually dead-letters) would silently disagree with
		// whatever policy governs it, the same bug fixed in JobService.Submit.
		_, err = tx.Exec(ctx, `
			INSERT INTO jobs (queue_id, scheduled_job_id, type, payload, run_at, max_attempts)
			SELECT $1, $2, 'recurring', $3, now(), rp.max_attempts
			FROM queues q JOIN retry_policies rp ON rp.id = q.retry_policy_id
			WHERE q.id = $1`,
			sj.QueueID, sj.ID, sj.PayloadTemplate)
		if err != nil {
			slog.Error("dispatch scheduled job failed", "scheduled_job_id", sj.ID, "error", err)
			continue
		}

		if err := s.scheduledJobs.SetNextRunAt(ctx, tx, sj.ID, schedule.Next(time.Now())); err != nil {
			slog.Error("advance next_run_at failed", "scheduled_job_id", sj.ID, "error", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("scheduler tx commit failed", "error", err)
	}
}
