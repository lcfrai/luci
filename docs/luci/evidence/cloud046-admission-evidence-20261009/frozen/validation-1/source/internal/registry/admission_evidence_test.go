package registry

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestStartAdmissionEvidenceValidatesBoundedOperands(t *testing.T) {
	now := time.Now().UTC()
	cpu := StartAdmissionEvidence{Metric: "host_cpu_utilization_percent", ObservedAt: now.Add(-time.Second), DecidedAt: now, ObservedPercent: 81.25, MaximumPercent: 75}
	failure := NewSessionStartFailure("admission", "cpu_pressure")
	if !cpu.ValidFor(failure) {
		t.Fatal("actual CPU refusal rejected")
	}
	for _, tc := range []struct {
		name   string
		change func(*StartAdmissionEvidence)
	}{
		{"unknown metric", func(e *StartAdmissionEvidence) { e.Metric = "/private/host/context" }},
		{"absent time", func(e *StartAdmissionEvidence) { e.ObservedAt = time.Time{} }},
		{"absent decision", func(e *StartAdmissionEvidence) { e.DecidedAt = time.Time{} }},
		{"future sample", func(e *StartAdmissionEvidence) { e.ObservedAt = now.Add(time.Second) }},
		{"stale sample", func(e *StartAdmissionEvidence) { e.ObservedAt = now.Add(-6 * time.Second) }},
		{"NaN observation", func(e *StartAdmissionEvidence) { e.ObservedPercent = math.NaN() }},
		{"infinite observation", func(e *StartAdmissionEvidence) { e.ObservedPercent = math.Inf(1) }},
		{"NaN maximum", func(e *StartAdmissionEvidence) { e.MaximumPercent = math.NaN() }},
		{"infinite maximum", func(e *StartAdmissionEvidence) { e.MaximumPercent = math.Inf(-1) }},
		{"out of range observation", func(e *StartAdmissionEvidence) { e.ObservedPercent = 101 }},
		{"negative maximum", func(e *StartAdmissionEvidence) { e.MaximumPercent = -1 }},
		{"negative observation", func(e *StartAdmissionEvidence) { e.ObservedPercent = -1 }},
		{"out of range maximum", func(e *StartAdmissionEvidence) { e.MaximumPercent = 101 }},
		{"equal does not refuse", func(e *StartAdmissionEvidence) { e.ObservedPercent = 75 }},
		{"unused operand", func(e *StartAdmissionEvidence) { e.ObservedBytes = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := cpu
			tc.change(&e)
			if e.ValidFor(failure) {
				t.Fatal("malformed evidence admitted")
			}
		})
	}
	for _, f := range []SessionStartFailure{NewSessionStartFailure("admission", "memory_pressure"), NewSessionStartFailure("run-revalidation", ""), {Stage: "admission", ReasonCode: "cpu_pressure", Message: "arbitrary"}} {
		if cpu.ValidFor(f) {
			t.Fatal("evidence attached to a different or noncanonical refusal")
		}
	}
	for _, metric := range []string{"host_available_memory", "execution_total_memory"} {
		e := StartAdmissionEvidence{Metric: metric, ObservedAt: now, DecidedAt: now, ObservedBytes: 0, RequiredBytes: 1024}
		f := NewSessionStartFailure("admission", "memory_pressure")
		if !e.ValidFor(f) {
			t.Fatal("zero available bytes must remain a measured value")
		}
		raw, err := json.Marshal(e)
		if err != nil || !strings.Contains(string(raw), `"observedBytes":0`) {
			t.Fatal("measured zero was omitted", err)
		}
		e.ObservedBytes = e.RequiredBytes
		if e.ValidFor(f) {
			t.Fatal("equal memory operands cannot explain refusal")
		}
		e.ObservedBytes, e.RequiredBytes = 0, 0
		if e.ValidFor(f) {
			t.Fatal("absent memory threshold admitted")
		}
	}
}
