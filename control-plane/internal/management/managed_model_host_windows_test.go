//go:build windows

package management

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func managedHostTestSnapshot(at time.Time, cpu *float64) resourcebudget.HostSnapshot {
	physical, commit := uint64(9<<30), uint64(10<<30)
	return resourcebudget.HostSnapshot{ObservedAt: at, Memory: resourcebudget.Memory{PhysicalAvailableBytes: &physical, CommitAvailableBytes: &commit}, CPU: resourcebudget.CPU{UtilizationPercent: cpu}}
}

func TestManagedHostInitialUnknownCPUUsesOneNewObservation(t *testing.T) {
	for _, phase := range []managedHostPhase{managedHostPreResume, managedHostInitialMonitor} {
		t.Run(string(phase), func(t *testing.T) {
			now := time.Unix(1000, 0)
			calls, waits := 0, 0
			// Models HostProbe's equal-ticks result without querying Windows:
			// ample memory but no measured CPU on the first observation.
			sample := func(context.Context) resourcebudget.HostSnapshot {
				calls++
				if calls == 1 {
					return managedHostTestSnapshot(now, nil)
				}
				cpu := 12.0
				return managedHostTestSnapshot(now, &cpu)
			}
			wait := func(ctx context.Context, delay time.Duration) error {
				waits++
				if delay != 100*time.Millisecond {
					t.Fatalf("recheck delay changed: %v", delay)
				}
				now = now.Add(delay)
				return ctx.Err()
			}
			h, err := observeManagedModelHost(context.Background(), sample, func() time.Time { return now }, wait, phase == managedHostPreResume, phase, true)
			if err != nil || calls != 2 || waits != 1 || h.CPU.UtilizationPercent == nil || *h.CPU.UtilizationPercent != 12 {
				t.Fatalf("new positive observation required: calls=%d waits=%d err=%v", calls, waits, err)
			}
		})
	}
}

func TestManagedHostInitialRecheckNeverBypassesOtherGates(t *testing.T) {
	for _, test := range []struct {
		name    string
		phase   managedHostPhase
		initial bool
		change  func(*resourcebudget.HostSnapshot)
	}{
		{"ordinary_monitor", managedHostMonitor, true, func(*resourcebudget.HostSnapshot) {}},
		{"startup_admission", managedHostStartupAdmission, true, func(*resourcebudget.HostSnapshot) {}},
		{"later_monitor", managedHostInitialMonitor, false, func(*resourcebudget.HostSnapshot) {}},
		{"physical_missing", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { h.Memory.PhysicalAvailableBytes = nil }},
		{"commit_missing", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { h.Memory.CommitAvailableBytes = nil }},
		{"physical_low", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { v := uint64(7 << 30); h.Memory.PhysicalAvailableBytes = &v }},
		{"commit_low", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { v := uint64(7 << 30); h.Memory.CommitAvailableBytes = &v }},
		{"stale", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { h.ObservedAt = h.ObservedAt.Add(-6 * time.Second) }},
		{"future", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { h.ObservedAt = h.ObservedAt.Add(time.Second) }},
		{"busy", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { v := 50.0; h.CPU.UtilizationPercent = &v }},
		{"nonfinite", managedHostPreResume, true, func(h *resourcebudget.HostSnapshot) { v := math.NaN(); h.CPU.UtilizationPercent = &v }},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(1000, 0)
			calls := 0
			sample := func(context.Context) resourcebudget.HostSnapshot {
				calls++
				h := managedHostTestSnapshot(now, nil)
				test.change(&h)
				return h
			}
			_, err := observeManagedModelHost(context.Background(), sample, func() time.Time { return now }, func(context.Context, time.Duration) error { t.Fatal("unsafe recheck"); return nil }, true, test.phase, test.initial)
			if err == nil || calls != 1 {
				t.Fatalf("gate was bypassed: calls=%d err=%v", calls, err)
			}
		})
	}
	for _, test := range []struct {
		name   string
		change func(*resourcebudget.HostSnapshot)
	}{
		{"still_unknown", func(h *resourcebudget.HostSnapshot) { h.CPU.UtilizationPercent = nil }},
		{"new_busy", func(h *resourcebudget.HostSnapshot) { v := 50.0; h.CPU.UtilizationPercent = &v }},
		{"new_physical_low", func(h *resourcebudget.HostSnapshot) { v := uint64(3 << 30); h.Memory.PhysicalAvailableBytes = &v }},
		{"new_commit_low", func(h *resourcebudget.HostSnapshot) { v := uint64(3 << 30); h.Memory.CommitAvailableBytes = &v }},
		{"new_stale", func(h *resourcebudget.HostSnapshot) { h.ObservedAt = h.ObservedAt.Add(-6 * time.Second) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(1000, 0)
			calls, waits := 0, 0
			sample := func(context.Context) resourcebudget.HostSnapshot {
				calls++
				if calls == 1 {
					return managedHostTestSnapshot(now, nil)
				}
				cpu := 12.0
				h := managedHostTestSnapshot(now, &cpu)
				test.change(&h)
				return h
			}
			_, err := observeManagedModelHost(context.Background(), sample, func() time.Time { return now }, func(context.Context, time.Duration) error { waits++; now = now.Add(100 * time.Millisecond); return nil }, false, managedHostInitialMonitor, true)
			if err == nil || calls != 2 || waits != 1 {
				t.Fatalf("recheck bypassed a gate or retried: %d/%d %v", calls, waits, err)
			}
		})
	}
}

func TestManagedHostRecheckKeepsCallerDeadlineAndCancellation(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			now := time.Unix(1000, 0)
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := observeManagedModelHost(ctx, func(context.Context) resourcebudget.HostSnapshot { calls++; return managedHostTestSnapshot(now, nil) }, func() time.Time { return now }, func(observed context.Context, delay time.Duration) error {
				if observed != ctx || delay != 100*time.Millisecond {
					t.Fatal("caller context or delay replaced")
				}
				return cause
			}, true, managedHostPreResume, true)
			if !errors.Is(err, cause) || calls != 1 {
				t.Fatalf("deadline/cancel ignored: %d %v", calls, err)
			}
		})
	}
	t.Run("cancel_after_wait", func(t *testing.T) {
		now := time.Unix(1000, 0)
		calls := 0
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := observeManagedModelHost(ctx, func(context.Context) resourcebudget.HostSnapshot { calls++; return managedHostTestSnapshot(now, nil) }, func() time.Time { return now }, func(context.Context, time.Duration) error { cancel(); return nil }, true, managedHostPreResume, true)
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("sample taken after cancellation: %d %v", calls, err)
		}
	})
	t.Run("original_short_deadline", func(t *testing.T) {
		now := time.Unix(1000, 0)
		calls := 0
		deadline := time.Now().Add(50 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		_, err := observeManagedModelHost(ctx, func(context.Context) resourcebudget.HostSnapshot { calls++; return managedHostTestSnapshot(now, nil) }, func() time.Time { return now }, func(observed context.Context, delay time.Duration) error {
			actual, ok := observed.Deadline()
			if observed != ctx || !ok || !actual.Equal(deadline) || delay != 100*time.Millisecond {
				t.Fatal("original short deadline extended")
			}
			return context.DeadlineExceeded
		}, true, managedHostPreResume, true)
		if !errors.Is(err, context.DeadlineExceeded) || calls > 1 {
			t.Fatalf("sample taken beyond short deadline: %d %v", calls, err)
		}
	})
}

func TestManagedHostCapacityDiagnosticIsBoundedAndDetached(t *testing.T) {
	now := time.Unix(1000, 0)
	cpu := 12.0
	h := managedHostTestSnapshot(now, &cpu)
	h.Unavailable = []resourcebudget.Issue{{Metric: "private-path", Reason: "private-error"}}
	h.Processes = []resourcebudget.ProcessUsage{{PID: 123, Role: "private-role"}}
	h.CPU.UtilizationPercent = nil
	err := managedModelHostCapacityAt(h, true, managedHostPreResume, now)
	detail, ok := err.(*managedHostCapacityError)
	if !ok {
		t.Fatal("missing typed rejection")
	}
	d := detail.HostCapacityDiagnostic()
	if d.Phase != "pre_resume" || d.Reason != "cpu_unavailable" || d.CPUPercent != nil || d.SampleAgeMillis == nil || *d.SampleAgeMillis != 0 {
		t.Fatalf("diagnostic changed: %+v", d)
	}
	*d.PhysicalAvailableBytes = 1
	*d.CommitAvailableBytes = 1
	*d.SampleAgeMillis = 99
	if next := detail.HostCapacityDiagnostic(); *next.PhysicalAvailableBytes != 9<<30 || *next.CommitAvailableBytes != 10<<30 || *next.SampleAgeMillis != 0 {
		t.Fatal("status caller mutated retained diagnostic")
	}
	*h.Memory.PhysicalAvailableBytes = 2
	if *detail.HostCapacityDiagnostic().PhysicalAvailableBytes != 9<<30 {
		t.Fatal("probe aliases retained diagnostic")
	}
	raw, e := json.Marshal(detail.HostCapacityDiagnostic())
	if e != nil || strings.Contains(string(raw), "private") || !strings.Contains(string(raw), `"cpuPercent":null`) {
		t.Fatalf("unsafe diagnostic projection: %s %v", raw, e)
	}
	v := uint64(1 << 53)
	h.Memory.PhysicalAvailableBytes = &v
	badCPU := math.Inf(1)
	h.CPU.UtilizationPercent = &badCPU
	h.ObservedAt = now.Add(-2 * time.Minute)
	bounded := managedHostDiagnostic(h, managedHostPhase("untrusted-phase"), "sample_time_invalid", now)
	if bounded.Phase != "monitor" || bounded.PhysicalAvailableBytes != nil || bounded.CPUPercent != nil || bounded.SampleAgeMillis != nil {
		t.Fatalf("unbounded fields leaked: %+v", bounded)
	}
	if _, e = json.Marshal(bounded); e != nil {
		t.Fatalf("nonfinite JSON exposed: %v", e)
	}
}
