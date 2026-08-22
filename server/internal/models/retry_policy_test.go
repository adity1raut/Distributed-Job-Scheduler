package models_test

import (
	"testing"

	"github.com/adity1raut/job-scheduler/internal/models"
)

func TestRetryPolicy_NextDelay(t *testing.T) {
	tests := []struct {
		name    string
		policy  models.RetryPolicy
		attempt int
		wantMS  int64
	}{
		{
			name:    "fixed stays constant across attempts",
			policy:  models.RetryPolicy{Strategy: models.RetryFixed, BaseDelayMS: 5000, MaxDelayMS: 60000},
			attempt: 4,
			wantMS:  5000,
		},
		{
			name:    "linear scales with attempt number",
			policy:  models.RetryPolicy{Strategy: models.RetryLinear, BaseDelayMS: 5000, MaxDelayMS: 60000},
			attempt: 3,
			wantMS:  15000,
		},
		{
			name:    "exponential doubles each attempt",
			policy:  models.RetryPolicy{Strategy: models.RetryExponential, BaseDelayMS: 5000, MaxDelayMS: 60000, Multiplier: 2},
			attempt: 3,
			wantMS:  20000, // 5000 * 2^(3-1)
		},
		{
			name:    "exponential clamps to max_delay_ms",
			policy:  models.RetryPolicy{Strategy: models.RetryExponential, BaseDelayMS: 5000, MaxDelayMS: 60000, Multiplier: 2},
			attempt: 6, // uncapped would be 5000*2^5 = 160000
			wantMS:  60000,
		},
		{
			name:    "linear also clamps to max_delay_ms",
			policy:  models.RetryPolicy{Strategy: models.RetryLinear, BaseDelayMS: 5000, MaxDelayMS: 20000},
			attempt: 10,
			wantMS:  20000,
		},
		{
			name:    "exponential defaults multiplier to 2 when unset",
			policy:  models.RetryPolicy{Strategy: models.RetryExponential, BaseDelayMS: 1000, MaxDelayMS: 60000},
			attempt: 2,
			wantMS:  2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.NextDelay(tt.attempt).Milliseconds()
			if got != tt.wantMS {
				t.Errorf("NextDelay(%d) = %dms, want %dms", tt.attempt, got, tt.wantMS)
			}
		})
	}
}
