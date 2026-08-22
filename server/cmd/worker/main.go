package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/adity1raut/job-scheduler/internal"
	"github.com/adity1raut/job-scheduler/internal/db"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	_ = godotenv.Load() // optional: falls back to real env vars / defaults if .env is absent
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	workerRepo := repository.NewWorkerRepository(pool)
	jobRepo := repository.NewJobRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	execRepo := repository.NewJobExecutionRepository(pool)
	logRepo := repository.NewJobLogRepository(pool)
	dlqRepo := repository.NewDeadLetterRepository(pool)
	policyRepo := repository.NewRetryPolicyRepository(pool)

	execSvc := service.NewExecutionService(jobRepo, execRepo, logRepo, dlqRepo, queueRepo, policyRepo)

	hostname, _ := os.Hostname()
	w, err := workerRepo.Register(ctx, hostname)
	if err != nil {
		slog.Error("failed to register worker", "error", err)
		os.Exit(1)
	}
	slog.Info("worker registered", "worker_id", w.ID, "hostname", hostname, "concurrency", cfg.WorkerConcurrency)

	var activeJobs int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.WorkerConcurrency)

	go sendHeartbeats(ctx, workerRepo, w.ID, time.Duration(cfg.HeartbeatSec)*time.Second, &activeJobs)

pollLoop:
	for {
		select {
		case <-ctx.Done():
			break pollLoop
		case <-time.After(time.Duration(cfg.WorkerPollMS) * time.Millisecond):
		}

		queueIDs, err := activeQueueIDs(ctx, pool)
		if err != nil {
			slog.Error("failed to list queues", "error", err)
			continue
		}

		for _, queueID := range queueIDs {
			select {
			case sem <- struct{}{}:
			default:
				continue // at capacity this cycle
			}

			job, err := execSvc.Claim(ctx, queueID, w.ID)
			if err != nil {
				<-sem
				if err != repository.ErrNoJobAvailable && err != repository.ErrNotFound {
					slog.Error("claim failed", "queue_id", queueID, "error", err)
				}
				continue
			}

			wg.Add(1)
			atomic.AddInt64(&activeJobs, 1)
			go func(job *models.Job) {
				defer func() {
					<-sem
					atomic.AddInt64(&activeJobs, -1)
					wg.Done()
				}()
				// Deliberately backgrounded: SIGTERM should let an
				// in-flight job finish, not cut it off mid-execution.
				execSvc.Run(context.Background(), job, w.ID)
			}(job)
		}
	}

	slog.Info("shutting down: waiting for in-flight jobs")
	waitWithTimeout(&wg, 30*time.Second)

	_ = workerRepo.SetOffline(context.Background(), w.ID)
	slog.Info("worker stopped")
}

func sendHeartbeats(ctx context.Context, workerRepo *repository.WorkerRepository, workerID uuid.UUID, interval time.Duration, activeJobs *int64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count := int(atomic.LoadInt64(activeJobs))
			if err := workerRepo.Heartbeat(context.Background(), workerID, count); err != nil {
				slog.Error("heartbeat failed", "error", err)
			}
		}
	}
}

func activeQueueIDs(ctx context.Context, pool *pgxpool.Pool) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `SELECT id FROM queues WHERE is_paused = false ORDER BY priority DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("shutdown timed out with jobs still in flight")
	}
}
