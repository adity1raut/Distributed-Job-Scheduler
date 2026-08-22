package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DeadLetterEntry struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	JobID           uuid.UUID       `json:"job_id" db:"job_id"`
	FinalError      *string         `json:"final_error,omitempty" db:"final_error"`
	PayloadSnapshot json.RawMessage `json:"payload_snapshot" db:"payload_snapshot"`
	FailedAt        time.Time       `json:"failed_at" db:"failed_at"`
	Replayed        bool            `json:"replayed" db:"replayed"`
}
