package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adity1raut/job-scheduler/internal"
	"github.com/adity1raut/job-scheduler/internal/db"
	"github.com/adity1raut/job-scheduler/internal/handler"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/scheduler"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
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

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	orgRepo := repository.NewOrganizationRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	projectRepo := repository.NewProjectRepository(pool)
	policyRepo := repository.NewRetryPolicyRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	scheduledJobRepo := repository.NewScheduledJobRepository(pool)
	jobRepo := repository.NewJobRepository(pool)
	execRepo := repository.NewJobExecutionRepository(pool)
	logRepo := repository.NewJobLogRepository(pool)
	workerRepo := repository.NewWorkerRepository(pool)
	dlqRepo := repository.NewDeadLetterRepository(pool)

	authSvc := service.NewAuthService(orgRepo, userRepo, policyRepo, cfg.JWTSecret, time.Duration(cfg.JWTExpiryHours)*time.Hour)
	projectSvc := service.NewProjectService(projectRepo)
	queueSvc := service.NewQueueService(queueRepo, projectRepo, policyRepo)
	jobSvc := service.NewJobService(jobRepo, queueRepo, execRepo, logRepo, policyRepo)
	scheduledJobSvc := service.NewScheduledJobService(scheduledJobRepo, queueRepo)
	workerSvc := service.NewWorkerService(workerRepo, cfg.StaleJobSec)
	dlqSvc := service.NewDLQService(dlqRepo, jobRepo, queueRepo)
	dashboardSvc := service.NewDashboardService(pool)

	router := handler.NewRouter(handler.Dependencies{
		JWTSecret:       cfg.JWTSecret,
		Redis:           rdb,
		RateLimitPerMin: cfg.RateLimitPerMin,
		AllowedOrigins:  cfg.CORSAllowedOrigins,
		Auth:            handler.NewAuthHandler(authSvc),
		Project:         handler.NewProjectHandler(projectSvc),
		Queue:           handler.NewQueueHandler(queueSvc),
		Job:             handler.NewJobHandler(jobSvc),
		ScheduledJob:    handler.NewScheduledJobHandler(scheduledJobSvc),
		Worker:          handler.NewWorkerHandler(workerSvc),
		DLQ:             handler.NewDLQHandler(dlqSvc),
		Dashboard:       handler.NewDashboardHandler(dashboardSvc),
	})

	sched := scheduler.New(pool, scheduledJobRepo, jobRepo, time.Duration(cfg.SchedulerTickSec)*time.Second, cfg.StaleJobSec)
	sched.Start(ctx)
	defer sched.Stop()

	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api server starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down api server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
