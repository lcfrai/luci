package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Common fixtures describe successful synthetic host measurements. They never
// impersonate a native Windows probe; specific tests below mutate the evidence.
func setupStorageFixtureSnapshot() resourcebudget.Snapshot {
	available := uint64(100) << 30
	now := time.Now()
	return resourcebudget.Snapshot{Host: resourcebudget.HostSnapshot{Platform: runtime.GOOS}, Execution: resourcebudget.ExecutionSnapshot{ObservedAt: now, EngineID: "synthetic-local-engine", DaemonDisk: &resourcebudget.Disk{ObservedAt: now, AvailableBytes: &available, Basis: "synthetic-guest-fixture", Role: "execution"}, HostStorage: &resourcebudget.HostStorageBound{Required: runtime.GOOS == "windows", State: "confirmed", MountedVolumeCount: 1, Disk: &resourcebudget.Disk{ObservedAt: now, AvailableBytes: &available, Basis: "synthetic-confirmed-fixture", Role: "docker-host-storage"}}}}
}

func windowsStorageFixtureSnapshot() resourcebudget.Snapshot {
	snapshot := setupStorageFixtureSnapshot()
	snapshot.Host.Platform = "windows"
	snapshot.Execution.HostStorage.Required = true
	return snapshot
}

func TestRuntimePreparationRefusesUnconfirmedStaleAndInsufficientHostStorage(t *testing.T) {
	for _, test := range []struct {
		name, code string
		change     func(*resourcebudget.Snapshot)
	}{
		{"missing", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.HostStorage = nil }},
		{"confirmation", "confirmation_required", func(s *resourcebudget.Snapshot) { s.Execution.HostStorage.State = "confirmation-required" }},
		{"invalidated", "confirmation_required", func(s *resourcebudget.Snapshot) { s.Execution.HostStorage.State = "invalidated" }},
		{"stale", "unavailable", func(s *resourcebudget.Snapshot) {
			s.Execution.HostStorage.Disk.ObservedAt = time.Now().Add(-time.Minute)
		}},
		{"unmeasured", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.HostStorage.Disk.AvailableBytes = nil }},
		{"low-host-large-guest", "insufficient_capacity", func(s *resourcebudget.Snapshot) {
			available, guest := runtimeInstallStorageDemand-1, uint64(1000)<<30
			s.Execution.HostStorage.Disk.AvailableBytes = &available
			s.Execution.DaemonDisk = &resourcebudget.Disk{ObservedAt: time.Now(), AvailableBytes: &guest}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, mux := journeyFixture(t)
			s.registerSetupTools(mux)
			runtimeInstallerFixture(t, s)
			s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
				snapshot := windowsStorageFixtureSnapshot()
				test.change(&snapshot)
				return snapshot
			})
			commands := 0
			s.runner = func(context.Context, string, []string, string) (string, error) {
				commands++
				return "", errors.New("unexpected Docker command")
			}
			before := auditTree(t, s.stateDir)
			w := journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 409)
			if !strings.Contains(w.Body.String(), "runtime_host_storage_"+test.code) || !strings.Contains(w.Body.String(), "40 GiB") || commands != 0 || readRuntimeJob(t, mux).Status != "idle" || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("refused preparation changed state, ran Docker or hid recovery", w.Body.String(), commands)
			}
		})
	}
}

func TestRuntimePreparationRechecksBeforeBuildWithoutImageDependency(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	runtimeInstallerFixture(t, s)
	var samples, commands atomic.Int32
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		if samples.Add(1) > 1 {
			snapshot.Execution.HostStorage.State = "invalidated"
		}
		return snapshot // Fresh storage observations do not require an installed sandbox image.
	})
	s.runner = func(context.Context, string, []string, string) (string, error) {
		commands.Add(1)
		return "", errors.New("unexpected build")
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 202)
	job := awaitRuntimeJob(t, mux)
	if job.Status != "failed" || commands.Load() != 0 || samples.Load() != 2 || !strings.Contains(job.Message, "confirm") {
		t.Fatal("preparation bypassed final fresh confirmation", job, samples.Load(), commands.Load())
	}
	// A new explicit attempt with confirmed host capacity can prepare an absent
	// image. The cold-build allowance must not depend on image inspection.
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		available := runtimeInstallStorageDemand
		snapshot.Execution.HostStorage.Disk.AvailableBytes = &available
		snapshot.Execution.DaemonDisk.AvailableBytes = &available
		return snapshot
	})
	s.runner = func(_ context.Context, _ string, args []string, _ string) (string, error) {
		commands.Add(1)
		if args[0] == "build" {
			return "built", nil
		}
		return "sha256:verified-fixture", nil
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 202)
	if job = awaitRuntimeJob(t, mux); job.Status != "ready" || commands.Load() != 2 {
		t.Fatal("confirmed image-independent preparation could not finish", job, commands.Load())
	}
}

func TestRuntimePreparationSeparatelyRequiresFreshDockerGuestCapacity(t *testing.T) {
	for _, test := range []struct {
		name, code string
		change     func(*resourcebudget.Snapshot)
	}{
		{"missing guest", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.DaemonDisk = nil }},
		{"missing measurement", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.DaemonDisk.AvailableBytes = nil }},
		{"stale guest", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.DaemonDisk.ObservedAt = time.Now().Add(-time.Minute) }},
		{"stale engine", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.ObservedAt = time.Now().Add(-time.Minute) }},
		{"unknown engine", "unavailable", func(s *resourcebudget.Snapshot) { s.Execution.EngineID = "" }},
		{"zero guest", "insufficient_capacity", func(s *resourcebudget.Snapshot) { value := uint64(0); s.Execution.DaemonDisk.AvailableBytes = &value }},
		{"roomy host full guest", "insufficient_capacity", func(s *resourcebudget.Snapshot) {
			value := runtimeInstallStorageDemand - 1
			s.Execution.DaemonDisk.AvailableBytes = &value
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, mux := journeyFixture(t)
			s.registerSetupTools(mux)
			runtimeInstallerFixture(t, s)
			s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
				snapshot := windowsStorageFixtureSnapshot()
				test.change(&snapshot)
				return snapshot // Physical Windows capacity remains 100 GiB.
			})
			commands := 0
			s.runner = func(context.Context, string, []string, string) (string, error) {
				commands++
				return "", errors.New("unexpected Docker command")
			}
			before := auditTree(t, s.stateDir)
			w := journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 409)
			if !strings.Contains(w.Body.String(), "runtime_guest_storage_"+test.code) || commands != 0 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("host room substituted for guest capacity", w.Body.String(), commands)
			}
		})
	}
}

func TestRuntimePreparationRechecksGuestCapacityBeforeStartingBuild(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	runtimeInstallerFixture(t, s)
	var samples, commands atomic.Int32
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		if samples.Add(1) > 1 {
			available := uint64(0)
			snapshot.Execution.DaemonDisk.AvailableBytes = &available
		}
		return snapshot
	})
	s.runner = func(context.Context, string, []string, string) (string, error) {
		commands.Add(1)
		return "", errors.New("unexpected build")
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 202)
	job := awaitRuntimeJob(t, mux)
	if job.Status != "failed" || commands.Load() != 0 || samples.Load() != 2 || !strings.Contains(job.Message, "Docker's own filesystem") {
		t.Fatal("preparation bypassed second guest-space check", job, samples.Load(), commands.Load())
	}
}

func TestRuntimeStorageLossPreservesExistingBuildAndReadOnlyReadiness(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	runtimeInstallerFixture(t, s)
	var missing atomic.Bool
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		if missing.Load() {
			snapshot.Execution.HostStorage = nil
		}
		return snapshot
	})
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var commands atomic.Int32
	s.runner = func(ctx context.Context, _ string, args []string, _ string) (string, error) {
		commands.Add(1)
		if args[0] == "build" {
			close(started)
			select {
			case <-release:
				return "built", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return "sha256:verified-fixture", nil
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 202)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("confirmed build did not start")
	}
	initial := readRuntimeJob(t, mux)
	missing.Store(true)
	before := auditTree(t, s.stateDir)
	for i := 0; i < 2; i++ {
		status := s.status()
		last := status.Checks[len(status.Checks)-1]
		if last.ID != "docker-backing-storage" || last.Status != "missing" || !strings.Contains(last.Action, "Settings") {
			t.Fatal("storage uncertainty was hidden", last)
		}
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/setup/runtime/install", map[string]any{}, 202)
	retained := readRuntimeJob(t, mux)
	if retained.ID != initial.ID || retained.Status != "installing" || commands.Load() != 1 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("storage loss stopped/restarted existing build or readiness wrote state", retained, commands.Load())
	}
	unblock()
	if finished := awaitRuntimeJob(t, mux); finished.Status != "ready" || finished.ID != initial.ID {
		t.Fatal("existing build was cancelled after storage confirmation became unavailable", finished)
	}
}

func TestRuntimeStorageReadinessSeparatesInstalledImageAndPreparationAllowance(t *testing.T) {
	s, _ := journeyFixture(t)
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		available := uint64(10) << 30
		snapshot.Execution.HostStorage.Disk.AvailableBytes = &available
		return snapshot
	})
	status := s.status()
	if status.Checks[len(status.Checks)-1].Status != "missing" {
		t.Fatal("missing image appeared ready for underfunded preparation")
	}
	s.mu.Lock()
	s.checks[2].Status = "ready"
	s.mu.Unlock()
	status = s.status()
	if status.Checks[len(status.Checks)-1].Status != "ready" {
		t.Fatal("existing image was incorrectly required to reserve a cold build")
	}
	s.SetRuntimeResourceSnapshot(func() resourcebudget.Snapshot {
		snapshot := windowsStorageFixtureSnapshot()
		snapshot.Execution.DaemonDisk = nil
		return snapshot
	})
	status = s.status()
	if status.Checks[len(status.Checks)-1].Status != "missing" {
		t.Fatal("installed image concealed missing guest-storage evidence")
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "runtime-install.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("readiness created an install receipt", err)
	}
}
