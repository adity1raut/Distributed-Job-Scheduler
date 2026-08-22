package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ScheduledJob struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	QueueID         uuid.UUID       `json:"queue_id" db:"queue_id"`
	CronExpression  string          `json:"cron_expression" db:"cron_expression"`
	PayloadTemplate json.RawMessage `json:"payload_template" db:"payload_template"`
	NextRunAt       time.Time       `json:"next_run_at" db:"next_run_at"`
	IsActive        bool            `json:"is_active" db:"is_active"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}
