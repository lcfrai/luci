package registry

import (
	"math"
	"time"
)

// StartAdmissionEvidence records the operands of one actual final refusal.
// It is private host telemetry, not start, retry, absence or release authority.
// A value (no maps, slices or pointers) keeps the retained observation immutable
// across registry copies. Historical failures can legitimately have no evidence.
type StartAdmissionEvidence struct {
	Metric          string    `json:"metric"`
	ObservedAt      time.Time `json:"observedAt"`
	DecidedAt       time.Time `json:"decidedAt"`
	ObservedPercent float64   `json:"observedPercent"`
	MaximumPercent  float64   `json:"maximumPercent"`
	ObservedBytes   uint64    `json:"observedBytes"`
	RequiredBytes   uint64    `json:"requiredBytes"`
}

func (e StartAdmissionEvidence) IsZero() bool { return e == (StartAdmissionEvidence{}) }

// Validation is against the recorded decision time, never today's capacity.
// Only the specific CPU/memory predicate which refused admission is retained.
func (e StartAdmissionEvidence) ValidFor(f SessionStartFailure) bool {
	if !f.Valid() || f.Stage != "admission" || e.ObservedAt.IsZero() || e.DecidedAt.IsZero() ||
		e.ObservedAt.After(e.DecidedAt) || e.DecidedAt.Sub(e.ObservedAt) > 5*time.Second {
		return false
	}
	switch e.Metric {
	case "host_cpu_utilization_percent":
		return f.ReasonCode == "cpu_pressure" && e.ObservedBytes == 0 && e.RequiredBytes == 0 &&
			!math.IsNaN(e.ObservedPercent) && !math.IsInf(e.ObservedPercent, 0) &&
			!math.IsNaN(e.MaximumPercent) && !math.IsInf(e.MaximumPercent, 0) &&
			e.MaximumPercent >= 0 && e.MaximumPercent <= 100 && e.ObservedPercent <= 100 &&
			e.ObservedPercent > e.MaximumPercent
	case "host_available_memory", "execution_total_memory":
		return f.ReasonCode == "memory_pressure" && e.ObservedPercent == 0 && e.MaximumPercent == 0 &&
			e.RequiredBytes > 0 && e.ObservedBytes < e.RequiredBytes
	}
	return false
}
