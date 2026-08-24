package testutil

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/adity1raut/job-scheduler/internal/handler"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// TestJWTSecret is the fixed secret every test router is issued with, so
// tests can decode/inspect tokens if they need to.
const TestJWTSecret = "test-secret-for-http-layer-tests"

// NewTestRouter wires the full HTTP router against pool exactly like
// cmd/api/main.go does — for tests that need to exercise real handlers and
// middleware (auth, RBAC, rate limiting), not just a service method in
// isolation. Requires TEST_DATABASE_URL (via RequireDB) and a reachable
// Redis at TEST_REDIS_ADDR (default localhost:6379); the rate limit is set
// high enough that a normal test run never trips it.
func NewTestRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()

	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

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

	authSvc := service.NewAuthService(orgRepo, userRepo, policyRepo, TestJWTSecret, time.Hour)
	projectSvc := service.NewProjectService(projectRepo)
	queueSvc := service.NewQueueService(queueRepo, projectRepo, policyRepo)
	jobSvc := service.NewJobService(jobRepo, queueRepo, execRepo, logRepo, policyRepo)
	scheduledJobSvc := service.NewScheduledJobService(scheduledJobRepo, queueRepo)
	workerSvc := service.NewWorkerService(workerRepo, 60)
	dlqSvc := service.NewDLQService(dlqRepo, jobRepo, queueRepo)
	dashboardSvc := service.NewDashboardService(pool)

	return handler.NewRouter(handler.Dependencies{
		JWTSecret:       TestJWTSecret,
		Redis:           rdb,
		RateLimitPerMin: 100000,
		AllowedOrigins:  []string{"*"},
		Auth:            handler.NewAuthHandler(authSvc),
		Project:         handler.NewProjectHandler(projectSvc),
		Queue:           handler.NewQueueHandler(queueSvc),
		Job:             handler.NewJobHandler(jobSvc),
		ScheduledJob:    handler.NewScheduledJobHandler(scheduledJobSvc),
		Worker:          handler.NewWorkerHandler(workerSvc),
		DLQ:             handler.NewDLQHandler(dlqSvc),
		Dashboard:       handler.NewDashboardHandler(dashboardSvc),
	})
}
