package models

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionStatus string

const (
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
)

type JobExecution struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	JobID         uuid.UUID       `json:"job_id" db:"job_id"`
	WorkerID      uuid.UUID       `json:"worker_id" db:"worker_id"`
	AttemptNumber int             `json:"attempt_number" db:"attempt_number"`
	Status        ExecutionStatus `json:"status" db:"status"`
	StartedAt     time.Time       `json:"started_at" db:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty" db:"finished_at"`
	ErrorMessage  *string         `json:"error_message,omitempty" db:"error_message"`
	DurationMS    *int            `json:"duration_ms,omitempty" db:"duration_ms"`
}
