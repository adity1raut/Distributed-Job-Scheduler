package models

import "time"

type Worker struct {
	ID            string    `json:"id"`
	Hostname      string    `json:"hostname"`
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}
