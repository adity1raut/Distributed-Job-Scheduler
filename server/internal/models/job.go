package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JobType string

const (
	JobTypeImmediate JobType = "immediate"
	JobTypeDelayed   JobType = "delayed"
	JobTypeScheduled JobType = "scheduled"
	JobTypeRecurring JobType = "recurring"
	JobTypeBatch     JobType = "batch"
)

type JobStatus string

const (
	JobStatusScheduled JobStatus = "scheduled"
	JobStatusQueued    JobStatus = "queued"
	JobStatusClaimed   JobStatus = "claimed"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDead      JobStatus = "dead"
)

type Job struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	QueueID        uuid.UUID       `json:"queue_id" db:"queue_id"`
	ScheduledJobID *uuid.UUID      `json:"scheduled_job_id,omitempty" db:"scheduled_job_id"`
	RetryPolicyID  *uuid.UUID      `json:"retry_policy_id,omitempty" db:"retry_policy_id"`
	Type           JobType         `json:"type" db:"type"`
	Status         JobStatus       `json:"status" db:"status"`
	Payload        json.RawMessage `json:"payload" db:"payload"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty" db:"idempotency_key"`
	Priority       int             `json:"priority" db:"priority"`
	Attempts       int             `json:"attempts" db:"attempts"`
	MaxAttempts    int             `json:"max_attempts" db:"max_attempts"`
	BatchID        *uuid.UUID      `json:"batch_id,omitempty" db:"batch_id"`
	RunAt          time.Time       `json:"run_at" db:"run_at"`
	LockedBy       *string         `json:"locked_by,omitempty" db:"locked_by"`
	LockedAt       *time.Time      `json:"locked_at,omitempty" db:"locked_at"`
	LastError      *string         `json:"last_error,omitempty" db:"last_error"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}
