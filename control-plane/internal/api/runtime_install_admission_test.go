package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func TestRuntimeInstallerRetainsActualUnavailableAdmissionWithoutBuild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*resourcebudget.Snapshot)
		check  func(*runtimeinstall.AdmissionObservation) bool
	}{
		{"host-stale", func(s *resourcebudget.Snapshot) { s.Host.ObservedAt = time.Now().Add(-time.Minute) }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.HostFresh && v.ExecutionFresh }},
		{"host-missing", func(s *resourcebudget.Snapshot) { s.Host.ObservedAt = time.Time{} }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.HostFresh && v.HostObservedAt.IsZero() }},
		{"engine-future", func(s *resourcebudget.Snapshot) { s.Execution.ObservedAt = time.Now().Add(time.Minute) }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.ExecutionFresh && v.HostFresh }},
		{"host-memory", func(s *resourcebudget.Snapshot) { s.Host.Memory.EffectiveAvailableBytes = nil }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.HostMemoryKnown }},
		{"host-cpu", func(s *resourcebudget.Snapshot) { s.Host.CPU.EffectiveMillis = nil }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.HostCPUKnown }},
		{"engine-memory", func(s *resourcebudget.Snapshot) { s.Execution.TotalMemoryBytes = nil }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.ExecutionMemoryKnown }},
		{"engine-cpu", func(s *resourcebudget.Snapshot) { s.Execution.CPUMillis = executionPointer(int64(0)) }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.ExecutionCPUKnown }},
		{"inventory", func(s *resourcebudget.Snapshot) { s.Execution.InventoryComplete = false }, func(v *runtimeinstall.AdmissionObservation) bool {
			return !v.InventoryComplete && v.HostFresh && v.ExecutionFresh
		}},
		{"engine-changed", func(s *resourcebudget.Snapshot) { s.Execution.EngineID = "private-changed-engine" }, func(v *runtimeinstall.AdmissionObservation) bool { return !v.EngineMatched && v.InventoryComplete }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, f, req := installerRuntimeFixture(t)
			snap := a.deps.ResourceSnapshot()
			tc.mutate(&snap)
			snap.Execution.Unavailable = []resourcebudget.Issue{{Metric: "private-path", Reason: "credential-like-private-value"}}
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return snap }
			var published runtimeinstall.Outcome
			out, err := a.installRuntimeImage(context.Background(), req, func(v runtimeinstall.Outcome) error { published = v; return nil })
			v := out.Admission
			if err != nil || out.Phase != "blocked" || out.ReasonCode != "resource_observation_unavailable" || v == nil || !tc.check(v) || v.Stage != "initial" || v.ReasonCode != out.ReasonCode || !v.SamplerAvailable || v.DecidedAt.IsZero() {
				t.Fatalf("missing original admission projection: %+v, %v", out, err)
			}
			if f.builds != 0 || f.publishes != 0 || f.observes != 0 || out.ResourcesHeld || out.Retained() || published.Admission == nil || *published.Admission != *v {
				t.Fatalf("observation changed allocation or publication: %+v %+v", f, published)
			}
			before, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			snap = executionFixtureSnapshot() // A later healthy observation cannot rewrite the retained refusal.
			after, err := json.Marshal(out)
			if err != nil || string(before) != string(after) || strings.Contains(string(after), "private-changed-engine") || strings.Contains(string(after), "credential-like-private-value") {
				t.Fatal("late observation or private sampler detail changed the refusal", string(after), err)
			}
			previewRetained(t, a, 0)
		})
	}
}

func TestRuntimeInstallerMissingSamplerIsDistinctFromMissingMetric(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	a.deps.ResourceSnapshot = nil
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err != nil || out.Admission == nil || out.Admission.SamplerAvailable || out.Admission.Stage != "initial" || !out.Admission.HostObservedAt.IsZero() || out.ResourcesHeld || f.builds != 0 {
		t.Fatal(out, err, f)
	}
}

func TestRuntimeInstallerRefusalRetainsCapacityOperandsAndSecondAdmission(t *testing.T) {
	for _, stage := range []string{"initial", "before-build"} {
		for _, code := range []string{"memory_pressure", "cpu_pressure"} {
			t.Run(stage+"/"+code, func(t *testing.T) {
				a, f, req := installerRuntimeFixture(t)
				base := a.deps.ResourceSnapshot
				refuse := stage == "initial"
				a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					s := base()
					if refuse && code == "memory_pressure" {
						s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(0))
					}
					if refuse && code == "cpu_pressure" {
						s.Host.CPU.UtilizationPercent = executionPointer(float64(99))
					}
					return s
				}
				var terminal runtimeinstall.Outcome
				out, _ := a.installRuntimeImage(context.Background(), req, func(v runtimeinstall.Outcome) error {
					if v.Phase == "reserved" {
						refuse = true
					}
					if v.Phase == "blocked" {
						terminal = v
					}
					return nil
				})
				v := out.Admission
				if v == nil || v.Stage != stage || v.ReasonCode != code || !v.HostFresh || !v.ExecutionFresh || !v.InventoryComplete || !v.EngineMatched || v.MetricObservedAt.IsZero() || v.MetricObservedAt.After(v.DecidedAt) {
					t.Fatalf("original refusal operands absent: %+v", v)
				}
				if code == "memory_pressure" && (v.Metric != "host_available_memory" || v.ObservedBytes == nil || *v.ObservedBytes != 0 || v.RequiredBytes == nil || *v.RequiredBytes == 0 || v.ObservedPercent != nil) {
					t.Fatal(v)
				}
				if code == "cpu_pressure" && (v.Metric != "host_cpu_utilization_percent" || v.ObservedPercent == nil || *v.ObservedPercent != 99 || v.MaximumPercent == nil || *v.MaximumPercent != 75 || v.ObservedBytes != nil) {
					t.Fatal(v)
				}
				if f.builds != 0 || f.publishes != 0 || out.ResourcesHeld || terminal.Admission == nil || *terminal.Admission != *v {
					t.Fatal("refusal started work or lost its receipt", f, out, terminal)
				}
				previewRetained(t, a, 0)
			})
		}
	}
}

func TestRuntimeInstallerSuccessDoesNotInventRefusalEvidence(t *testing.T) {
	a, _, req := installerRuntimeFixture(t)
	out, err := a.installRuntimeImage(context.Background(), req, func(v runtimeinstall.Outcome) error {
		if v.Admission != nil {
			t.Fatal("successful phase had refusal evidence", v)
		}
		return nil
	})
	if err != nil || out.Phase != "ready" || out.Admission != nil {
		t.Fatal(out, err)
	}
}
