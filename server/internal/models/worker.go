package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkerStatus string

const (
	WorkerOnline  WorkerStatus = "online"
	WorkerOffline WorkerStatus = "offline"
)

type Worker struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	OrgID     uuid.UUID    `json:"org_id" db:"org_id"`
	Hostname  string       `json:"hostname" db:"hostname"`
	Status    WorkerStatus `json:"status" db:"status"`
	StartedAt time.Time    `json:"started_at" db:"started_at"`
}

type WorkerHeartbeat struct {
	ID             uuid.UUID `json:"id" db:"id"`
	WorkerID       uuid.UUID `json:"worker_id" db:"worker_id"`
	ReportedAt     time.Time `json:"reported_at" db:"reported_at"`
	ActiveJobCount int       `json:"active_job_count" db:"active_job_count"`
}

// WorkerWithStatus is a worker joined with its most recent heartbeat and a
// derived online/stale flag, as returned by the fleet-status query.
type WorkerWithStatus struct {
	ID              uuid.UUID    `json:"id" db:"id"`
	Hostname        string       `json:"hostname" db:"hostname"`
	Status          WorkerStatus `json:"status" db:"status"`
	StartedAt       time.Time    `json:"started_at" db:"started_at"`
	LastHeartbeatAt *time.Time   `json:"last_heartbeat_at,omitempty" db:"last_heartbeat_at"`
	ActiveJobCount  *int         `json:"active_job_count,omitempty" db:"active_job_count"`
	IsStale         bool         `json:"is_stale" db:"is_stale"`
}
