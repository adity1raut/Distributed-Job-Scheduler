package models

import (
	"time"

	"github.com/google/uuid"
)

type Queue struct {
	ID               uuid.UUID `json:"id" db:"id"`
	ProjectID        uuid.UUID `json:"project_id" db:"project_id"`
	RetryPolicyID    uuid.UUID `json:"retry_policy_id" db:"retry_policy_id"`
	Name             string    `json:"name" db:"name"`
	Priority         int       `json:"priority" db:"priority"`
	ConcurrencyLimit int       `json:"concurrency_limit" db:"concurrency_limit"`
	IsPaused         bool      `json:"is_paused" db:"is_paused"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type QueueStats struct {
	Scheduled int64 `json:"scheduled" db:"scheduled"`
	Queued    int64 `json:"queued" db:"queued"`
	Claimed   int64 `json:"claimed" db:"claimed"`
	Running   int64 `json:"running" db:"running"`
	Completed int64 `json:"completed" db:"completed"`
	Failed    int64 `json:"failed" db:"failed"`
	Dead      int64 `json:"dead" db:"dead"`
}
