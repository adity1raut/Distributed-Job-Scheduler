package handler

import (
	"net/http"
	"time"

	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
)

// Dependencies bundles every handler and the cross-cutting config the
// router needs to wire routes and middleware in one place.
type Dependencies struct {
	JWTSecret       string
	Redis           *redis.Client
	RateLimitPerMin int
	AllowedOrigins  []string

	Auth         *AuthHandler
	Project      *ProjectHandler
	Queue        *QueueHandler
	Job          *JobHandler
	ScheduledJob *ScheduledJobHandler
	Worker       *WorkerHandler
	DLQ          *DLQHandler
	Dashboard    *DashboardHandler
}

func NewRouter(d Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: false,
		MaxAge:           int(10 * time.Minute / time.Second),
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimit(d.Redis, d.RateLimitPerMin))
			r.Post("/auth/register", d.Auth.Register)
			r.Post("/auth/login", d.Auth.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(d.JWTSecret))
			r.Use(middleware.RateLimit(d.Redis, d.RateLimitPerMin))

			r.Route("/projects", func(r chi.Router) {
				r.Post("/", d.Project.Create)
				r.Get("/", d.Project.List)
				r.Route("/{projectID}", func(r chi.Router) {
					r.Get("/", d.Project.Get)
					r.Delete("/", d.Project.Delete)
					r.Post("/queues", d.Queue.Create)
					r.Get("/queues", d.Queue.List)
				})
			})

			r.Route("/queues/{queueID}", func(r chi.Router) {
				r.Get("/", d.Queue.Get)
				r.Patch("/", d.Queue.UpdateConfig)
				r.Delete("/", d.Queue.Delete)
				r.Post("/pause", d.Queue.Pause)
				r.Post("/resume", d.Queue.Resume)
				r.Get("/stats", d.Queue.Stats)

				r.Post("/jobs", d.Job.Submit)
				r.Get("/jobs", d.Job.List)

				r.Post("/scheduled-jobs", d.ScheduledJob.Create)
				r.Get("/scheduled-jobs", d.ScheduledJob.List)

				r.Get("/dlq", d.DLQ.List)
			})

			r.Route("/scheduled-jobs/{scheduledJobID}", func(r chi.Router) {
				r.Post("/pause", d.ScheduledJob.Pause)
				r.Post("/resume", d.ScheduledJob.Resume)
			})

			r.Route("/jobs/{jobID}", func(r chi.Router) {
				r.Get("/", d.Job.Get)
				r.Post("/retry", d.Job.Retry)
			})
			r.Get("/executions/{executionID}/logs", d.Job.Logs)

			r.Get("/workers", d.Worker.List)
			r.Get("/workers/{workerID}/heartbeats", d.Worker.Heartbeats)

			r.Post("/dlq/{entryID}/replay", d.DLQ.Replay)

			r.Get("/dashboard/overview", d.Dashboard.Overview)
			r.Get("/dashboard/recent-jobs", d.Dashboard.RecentJobs)
		})
	})

	return r
}
