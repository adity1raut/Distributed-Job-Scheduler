// Package testutil holds the fixtures shared by every integration test —
// tests that exercise real Postgres behavior (SKIP LOCKED, advisory locks,
// constraint enforcement) that a mock can't stand in for. Every helper here
// skips the calling test when TEST_DATABASE_URL isn't set, so `go test
// ./...` stays fast and dependency-free by default.
package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RequireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// MustParseUUID parses a UUID string coming out of an HTTP JSON response in
// an HTTP-layer test, failing the test immediately on a malformed value
// instead of propagating a zero-UUID silently into a later assertion.
func MustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("expected a valid UUID, got %q: %v", s, err)
	}
	return id
}

func ScanUUID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		t.Fatalf("fixture query failed: %v\nquery: %s", err, query)
	}
	return id
}

func Exec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, query, args...); err != nil {
		t.Fatalf("fixture exec failed: %v\nquery: %s", err, query)
	}
}

// Fixture is an org → user → project → retry policy → queue chain, unique
// per call so parallel or repeated test runs never collide on a UNIQUE
// constraint.
type Fixture struct {
	OrgID     uuid.UUID
	UserID    uuid.UUID
	ProjectID uuid.UUID
	PolicyID  uuid.UUID
	QueueID   uuid.UUID
}

// SeedFixtureOpts lets a test tune the knobs that matter for the behavior
// it's exercising (a tight retry policy to avoid real sleeps, a low
// concurrency limit to test throttling) without hand-rolling the whole
// fixture chain again.
type SeedFixtureOpts struct {
	ConcurrencyLimit int
	RetryStrategy    string // "fixed" | "linear" | "exponential"
	BaseDelayMS      int
	MaxDelayMS       int
	MaxAttempts      int
}

func SeedFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, opts SeedFixtureOpts) Fixture {
	t.Helper()
	suffix := uuid.NewString()[:8]

	if opts.ConcurrencyLimit <= 0 {
		opts.ConcurrencyLimit = 5
	}
	if opts.RetryStrategy == "" {
		opts.RetryStrategy = "fixed"
	}
	if opts.BaseDelayMS <= 0 {
		opts.BaseDelayMS = 50
	}
	if opts.MaxDelayMS <= 0 {
		opts.MaxDelayMS = 200
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}

	orgID := ScanUUID(t, ctx, pool, `INSERT INTO organizations (name) VALUES ($1) RETURNING id`, "test-org-"+suffix)
	userID := ScanUUID(t, ctx, pool, `
		INSERT INTO users (org_id, email, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		orgID, "test-"+suffix+"@example.com")
	policyID := ScanUUID(t, ctx, pool, `
		INSERT INTO retry_policies (org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier)
		VALUES ($1, 'test-policy', $2, $3, $4, $5, 2) RETURNING id`,
		orgID, opts.RetryStrategy, opts.BaseDelayMS, opts.MaxDelayMS, opts.MaxAttempts)
	projectID := ScanUUID(t, ctx, pool, `
		INSERT INTO projects (org_id, owner_id, name) VALUES ($1, $2, $3) RETURNING id`,
		orgID, userID, "test-project-"+suffix)
	queueID := ScanUUID(t, ctx, pool, `
		INSERT INTO queues (project_id, retry_policy_id, name, concurrency_limit)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		projectID, policyID, "test-queue-"+suffix, opts.ConcurrencyLimit)

	return Fixture{OrgID: orgID, UserID: userID, ProjectID: projectID, PolicyID: policyID, QueueID: queueID}
}

// SeedWorker inserts a worker row — job_executions and worker_heartbeats
// both have a FK to workers, so any test touching them needs one.
func SeedWorker(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return ScanUUID(t, ctx, pool,
		`INSERT INTO workers (hostname, status) VALUES ($1, 'online') RETURNING id`,
		"test-worker-"+uuid.NewString()[:8])
}
