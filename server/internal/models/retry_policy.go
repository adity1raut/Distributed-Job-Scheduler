package models

import (
	"time"

	"github.com/google/uuid"
)

type RetryStrategy string

const (
	RetryFixed       RetryStrategy = "fixed"
	RetryLinear      RetryStrategy = "linear"
	RetryExponential RetryStrategy = "exponential"
)

type RetryPolicy struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	OrgID       uuid.UUID     `json:"org_id" db:"org_id"`
	Name        string        `json:"name" db:"name"`
	Strategy    RetryStrategy `json:"strategy" db:"strategy"`
	BaseDelayMS int           `json:"base_delay_ms" db:"base_delay_ms"`
	MaxDelayMS  int           `json:"max_delay_ms" db:"max_delay_ms"`
	MaxAttempts int           `json:"max_attempts" db:"max_attempts"`
	Multiplier  float64       `json:"multiplier" db:"multiplier"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
}

// NextDelay computes the backoff before the given attempt number (1-indexed),
// clamped to MaxDelayMS. See docs/design-decisions.md for the formulas.
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	var ms float64
	switch p.Strategy {
	case RetryFixed:
		ms = float64(p.BaseDelayMS)
	case RetryLinear:
		ms = float64(p.BaseDelayMS) * float64(attempt)
	case RetryExponential:
		multiplier := p.Multiplier
		if multiplier <= 0 {
			multiplier = 2
		}
		ms = float64(p.BaseDelayMS)
		for i := 1; i < attempt; i++ {
			ms *= multiplier
		}
	default:
		ms = float64(p.BaseDelayMS)
	}

	if p.MaxDelayMS > 0 && ms > float64(p.MaxDelayMS) {
		ms = float64(p.MaxDelayMS)
	}
	return time.Duration(ms) * time.Millisecond
}
