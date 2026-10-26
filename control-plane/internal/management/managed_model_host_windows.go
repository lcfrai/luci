//go:build windows

package management

import (
	"context"
	"math"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type managedHostPhase string

const (
	managedHostStartupAdmission managedHostPhase = "startup_admission"
	managedHostPreResume        managedHostPhase = "pre_resume"
	managedHostInitialMonitor   managedHostPhase = "initial_monitor"
	managedHostMonitor          managedHostPhase = "monitor"
	managedHostRecheckDelay                      = 100 * time.Millisecond
)

type managedHostCapacityError struct {
	diagnostic managedmodel.HostCapacityDiagnostic
}

func (e *managedHostCapacityError) Error() string { return "host capacity unavailable" }
func (e *managedHostCapacityError) HostCapacityDiagnostic() *managedmodel.HostCapacityDiagnostic {
	copy := e.diagnostic
	if copy.PhysicalAvailableBytes != nil {
		value := *copy.PhysicalAvailableBytes
		copy.PhysicalAvailableBytes = &value
	}
	if copy.CommitAvailableBytes != nil {
		value := *copy.CommitAvailableBytes
		copy.CommitAvailableBytes = &value
	}
	if copy.CPUPercent != nil {
		value := *copy.CPUPercent
		copy.CPUPercent = &value
	}
	if copy.SampleAgeMillis != nil {
		value := *copy.SampleAgeMillis
		copy.SampleAgeMillis = &value
	}
	return &copy
}

// Keep diagnostics numeric and closed: never copy probe issues, process inventory,
// paths or exception text into a service status. Values outside JSON's exact
// integer range, nonfinite CPU and unbounded ages remain explicitly unknown.
func managedHostDiagnostic(h resourcebudget.HostSnapshot, phase managedHostPhase, reason string, now time.Time) managedmodel.HostCapacityDiagnostic {
	switch phase {
	case managedHostStartupAdmission, managedHostPreResume, managedHostInitialMonitor, managedHostMonitor:
	default:
		phase = managedHostMonitor
	}
	d := managedmodel.HostCapacityDiagnostic{Phase: string(phase), Reason: reason}
	boundedBytes := func(v *uint64) *uint64 {
		if v == nil || *v > 1<<53-1 {
			return nil
		}
		copy := *v
		return &copy
	}
	d.PhysicalAvailableBytes = boundedBytes(h.Memory.PhysicalAvailableBytes)
	d.CommitAvailableBytes = boundedBytes(h.Memory.CommitAvailableBytes)
	if v := h.CPU.UtilizationPercent; v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 && *v <= 100 {
		copy := *v
		d.CPUPercent = &copy
	}
	if !h.ObservedAt.IsZero() {
		age := now.Sub(h.ObservedAt)
		if age >= -time.Minute && age <= time.Minute {
			millis := age.Milliseconds()
			d.SampleAgeMillis = &millis
		}
	}
	return d
}

func managedModelHostCapacityAt(h resourcebudget.HostSnapshot, start bool, phase managedHostPhase, now time.Time) error {
	min := uint64(4 << 30)
	if start {
		min = 8 << 30
	}
	reason := ""
	switch {
	case h.ObservedAt.IsZero(), now.Sub(h.ObservedAt) < 0, now.Sub(h.ObservedAt) > 5*time.Second:
		reason = "sample_time_invalid"
	case h.Memory.PhysicalAvailableBytes == nil:
		reason = "physical_unavailable"
	case h.Memory.CommitAvailableBytes == nil:
		reason = "commit_unavailable"
	case *h.Memory.PhysicalAvailableBytes < min:
		reason = "physical_low"
	case *h.Memory.CommitAvailableBytes < min:
		reason = "commit_low"
	case h.CPU.UtilizationPercent == nil:
		reason = "cpu_unavailable"
	case math.IsNaN(*h.CPU.UtilizationPercent), math.IsInf(*h.CPU.UtilizationPercent, 0), *h.CPU.UtilizationPercent < 0, *h.CPU.UtilizationPercent > 100:
		reason = "cpu_invalid"
	case *h.CPU.UtilizationPercent >= 50:
		reason = "cpu_busy"
	}
	if reason == "" {
		return nil
	}
	return &managedHostCapacityError{diagnostic: managedHostDiagnostic(h, phase, reason, now)}
}

func managedModelHostCapacity(h resourcebudget.HostSnapshot, start bool) error {
	phase := managedHostMonitor
	if start {
		phase = managedHostStartupAdmission
	}
	return managedModelHostCapacityAt(h, start, phase, time.Now())
}

func waitManagedModelHostRecheck(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Equal cumulative CPU ticks can make a rapid sample unknown. Only the two
// initial seams may take one new sample after 100ms, inside the caller's original
// deadline. No prior utilization is reused and every gate is checked anew.
func observeManagedModelHost(ctx context.Context, sample func(context.Context) resourcebudget.HostSnapshot, now func() time.Time, wait func(context.Context, time.Duration) error, start bool, phase managedHostPhase, initialRecheck bool) (resourcebudget.HostSnapshot, error) {
	if e := ctx.Err(); e != nil {
		return resourcebudget.HostSnapshot{}, e
	}
	h := sample(ctx)
	if e := ctx.Err(); e != nil {
		return h, e
	}
	e := managedModelHostCapacityAt(h, start, phase, now())
	detail, rejected := e.(*managedHostCapacityError)
	if !rejected || !initialRecheck || (phase != managedHostPreResume && phase != managedHostInitialMonitor) || detail.diagnostic.Reason != "cpu_unavailable" {
		return h, e
	}
	if e = wait(ctx, managedHostRecheckDelay); e != nil {
		return h, e
	}
	if e = ctx.Err(); e != nil {
		return h, e
	}
	h = sample(ctx)
	if e = ctx.Err(); e != nil {
		return h, e
	}
	return h, managedModelHostCapacityAt(h, start, phase, now())
}
