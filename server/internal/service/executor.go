package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// demoPayload is the pluggable job handler for this reference platform. A
// real deployment swaps RunPayload for dispatch into application code (per
// queue or per job type); this implementation exists so the claim → retry →
// DLQ pipeline is exercisable end-to-end without any external dependency.
type demoPayload struct {
	Task     string  `json:"task"`
	SleepMS  int     `json:"sleep_ms"`
	FailRate float64 `json:"fail_rate"`
}

func RunPayload(payload json.RawMessage) error {
	var p demoPayload
	_ = json.Unmarshal(payload, &p)

	if p.SleepMS > 0 {
		time.Sleep(time.Duration(p.SleepMS) * time.Millisecond)
	}

	if p.Task == "fail" {
		return errors.New("job payload requested a forced failure")
	}
	if p.FailRate > 0 && rand.Float64() < p.FailRate {
		return fmt.Errorf("simulated failure (fail_rate=%.2f)", p.FailRate)
	}
	return nil
}
