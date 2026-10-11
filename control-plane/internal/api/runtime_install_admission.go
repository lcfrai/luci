package api

import (
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Project only operands consulted by freshExecutionCapacity, using the same
// snapshot and decision time. This does not alter any admission predicate.
func runtimeAdmissionObservation(snap *resourcebudget.Snapshot, engine string, now time.Time) *runtimeinstall.AdmissionObservation {
	v := &runtimeinstall.AdmissionObservation{DecidedAt: now.UTC()}
	if snap == nil {
		return v
	}
	v.SamplerAvailable = true
	v.HostObservedAt, v.ExecutionObservedAt = snap.Host.ObservedAt.UTC(), snap.Execution.ObservedAt.UTC()
	v.HostFresh, v.ExecutionFresh = snap.Host.Fresh(now, 5*time.Second), snap.Execution.Fresh(now, 5*time.Second)
	v.HostMemoryKnown = snap.Host.Memory.EffectiveAvailableBytes != nil && snap.Host.Memory.EffectiveTotalBytes != nil
	v.HostCPUKnown = snap.Host.CPU.EffectiveMillis != nil && *snap.Host.CPU.EffectiveMillis > 0
	v.ExecutionMemoryKnown = snap.Execution.TotalMemoryBytes != nil
	v.ExecutionCPUKnown = snap.Execution.CPUMillis != nil && *snap.Execution.CPUMillis > 0
	v.InventoryComplete = snap.Execution.InventoryComplete
	v.EngineMatched = snap.Execution.EngineID == engine
	return v
}

func runtimeAdmissionRefusal(decision executionDecision, stage string) *runtimeinstall.AdmissionObservation {
	if decision.State == "admitted" || decision.RuntimeAdmission == nil {
		return nil
	}
	// Copy before publication; later decisions cannot modify the retained value.
	v := *decision.RuntimeAdmission
	v.Stage, v.ReasonCode = stage, decision.Code
	e := decision.Evidence
	v.Metric, v.MetricObservedAt = e.Metric, e.ObservedAt
	switch e.Metric {
	case "host_available_memory", "execution_total_memory":
		v.ObservedBytes, v.RequiredBytes = &e.ObservedBytes, &e.RequiredBytes
	case "host_cpu_utilization_percent":
		v.ObservedPercent, v.MaximumPercent = &e.ObservedPercent, &e.MaximumPercent
	}
	return &v
}
