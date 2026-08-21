package main

import (
	"log"

	"github.com/adity1raut/job-scheduler/internal"
)

func main() {
	cfg := config.Load()

	log.Printf("api server starting on port %s", cfg.APIPort)

	// TODO: open db pool, wire repositories/services/handlers, mount chi router, listen and serve
}
