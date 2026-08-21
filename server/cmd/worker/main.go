package main

import (
	"log"

	"github.com/adity1raut/job-scheduler/internal"
)

func main() {
	cfg := config.Load()

	log.Printf("worker starting: poll=%dms concurrency=%d", cfg.WorkerPollMS, cfg.WorkerConcurrency)

	// TODO: open db pool, poll queues, claim jobs (SELECT ... FOR UPDATE SKIP LOCKED),
	// execute concurrently, send heartbeats, shut down gracefully on SIGTERM
}
