package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// probeTimeout caps how long a readiness check may block. Whatever is
// polling — a Docker healthcheck, a load balancer, an uptime monitor — gives
// up on its own schedule, so a check that hangs on a wedged dependency must
// fail fast rather than pile up goroutines behind it.
const probeTimeout = 2 * time.Second

// HealthHandler answers the two questions a deployment needs to tell apart:
// is the process alive, and is it ready to serve traffic.
//
// Liveness deliberately checks nothing external: a restart cannot fix a down
// database, so failing liveness on it would turn one outage into a restart
// loop across every container.
type HealthHandler struct {
	db      *pgxpool.Pool
	redis   *redis.Client
	version string
}

func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client, version string) *HealthHandler {
	return &HealthHandler{db: db, redis: rdb, version: version}
}

// Live reports that the process is running and able to serve HTTP.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": h.version,
	})
}

// Ready reports whether every dependency needed to serve a request is
// reachable. Docker marks the container unhealthy while this fails, which is
// what keeps a deploy from switching traffic to a container that cannot
// actually reach Postgres.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	checks := h.runChecks(r.Context())

	status := http.StatusOK
	overall := "ok"
	for _, c := range checks {
		// Redis outages degrade to "allow" in the rate limiter rather than
		// failing requests, so they must not pull a replica out of service.
		if c.Status != "ok" && c.Critical {
			status = http.StatusServiceUnavailable
			overall = "unavailable"
		}
	}

	httpx.WriteJSON(w, status, map[string]any{
		"status":  overall,
		"version": h.version,
		"checks":  checks,
	})
}

type checkResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
	Critical  bool   `json:"critical"`
}

func (h *HealthHandler) runChecks(ctx context.Context) []checkResult {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	results := make([]checkResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		results[0] = timed("postgres", true, func() error { return h.db.Ping(ctx) })
	}()
	go func() {
		defer wg.Done()
		results[1] = timed("redis", false, func() error { return h.redis.Ping(ctx).Err() })
	}()

	wg.Wait()
	return results
}

func timed(name string, critical bool, fn func() error) checkResult {
	start := time.Now()
	err := fn()
	res := checkResult{
		Name:      name,
		Status:    "ok",
		LatencyMS: time.Since(start).Milliseconds(),
		Critical:  critical,
	}
	if err != nil {
		res.Status = "error"
		res.Error = err.Error()
	}
	return res
}
