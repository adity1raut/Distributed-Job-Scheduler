package repository_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/testutil"
)

// TestClaimNext_NoDuplicateClaims is the concurrency-correctness test the
// brief asks for: many simulated workers race to claim from one queue, and
// no job may ever be returned to two workers. This requires a real Postgres
// (SKIP LOCKED is a database guarantee, not something a mock can prove) —
// set TEST_DATABASE_URL to run it; it's skipped otherwise.
func TestClaimNext_NoDuplicateClaims(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()

	const jobCount = 200
	fx := testutil.SeedFixture(t, ctx, pool, testutil.SeedFixtureOpts{ConcurrencyLimit: jobCount})

	for i := 0; i < jobCount; i++ {
		testutil.Exec(t, ctx, pool, `
			INSERT INTO jobs (queue_id, type, status, payload, max_attempts)
			VALUES ($1, 'immediate', 'queued', '{}', 5)`, fx.QueueID)
	}

	jobRepo := repository.NewJobRepository(pool)

	const workerCount = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed = make(map[string]int) // job ID -> number of times claimed
	)

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				job, err := jobRepo.ClaimNext(ctx, fx.QueueID, "worker-"+strconv.Itoa(workerID))
				if err != nil {
					return // ErrNoJobAvailable once the queue is drained
				}
				mu.Lock()
				claimed[job.ID.String()]++
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()

	if len(claimed) != jobCount {
		t.Fatalf("expected %d distinct jobs claimed, got %d", jobCount, len(claimed))
	}
	for id, n := range claimed {
		if n != 1 {
			t.Fatalf("job %s was claimed %d times, want exactly 1 — duplicate claim under concurrency", id, n)
		}
	}
}
