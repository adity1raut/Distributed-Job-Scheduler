package models

import (
	"time"

	"github.com/google/uuid"
)

type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

type JobLog struct {
	ID             uuid.UUID `json:"id" db:"id"`
	JobExecutionID uuid.UUID `json:"job_execution_id" db:"job_execution_id"`
	LoggedAt       time.Time `json:"logged_at" db:"logged_at"`
	Level          LogLevel  `json:"level" db:"level"`
	Message        string    `json:"message" db:"message"`
}
