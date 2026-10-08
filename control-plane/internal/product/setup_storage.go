package product

import (
	"errors"
	"runtime"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The bundled images/sandbox/Dockerfile downloads two Debian-based images
// (Go 1.26 and Node 24), apt packages and three pinned native agent distributions,
// compiles sandboxd, and copies the Go toolchain into the final image. Docker
// can retain compressed downloads, unpacked layers and intermediate caches.
// 32 GiB of cold-build working space plus an 8 GiB host reserve is a conservative
// prerequisite, matching the Windows acceptance launcher's 40 GiB floor. It is
// not a measured peak, Docker quota, or guarantee about subsequent worker writes.
const runtimeBuildWorkingSpaceBytes uint64 = 32 << 30
const runtimeBuildHostReserveBytes uint64 = 8 << 30
const runtimeInstallStorageDemand = runtimeBuildWorkingSpaceBytes + runtimeBuildHostReserveBytes

// SetRuntimeResourceSnapshot accepts only an already-sampled cache reader.
// Setup GETs never execute a native probe, discover a VHD or infer host capacity
// from free space inside the Docker guest.
func (s *Server) SetRuntimeResourceSnapshot(snapshot func() resourcebudget.Snapshot) {
	s.mu.Lock()
	s.runtimeResourceSnapshot = snapshot
	s.mu.Unlock()
}

func (s *Server) runtimeStorageSnapshot() resourcebudget.Snapshot {
	s.mu.RLock()
	snapshot := s.runtimeResourceSnapshot
	s.mu.RUnlock()
	if snapshot == nil {
		return resourcebudget.Snapshot{}
	}
	return snapshot()
}

func runtimeStorageRequired(snapshot resourcebudget.Snapshot) bool {
	return runtime.GOOS == "windows" || snapshot.Host.Platform == "windows" || snapshot.Execution.HostStorage != nil && snapshot.Execution.HostStorage.Required
}

func (s *Server) runtimeStoragePreflight() error {
	snapshot := s.runtimeStorageSnapshot()
	err := validateRuntimeStorage(snapshot, runtimeInstallStorageDemand)
	if err != nil {
		return &resourcebudget.LocalStorageError{Code: runtimeStorageErrorCode(err), Message: err.Error() + " Runtime preparation requires a conservative 40 GiB working-space prerequisite (32 GiB build allowance plus 8 GiB reserve). This is an estimate, not a build-size guarantee."}
	}
	return nil
}

// Physical host space and the Docker guest filesystem are independent limits.
// A growing VHD can run out of host space; a full guest can reject a build even
// when its host volume is roomy. Neither observation substitutes for the other.
func validateRuntimeStorage(snapshot resourcebudget.Snapshot, demand uint64) error {
	if !runtimeStorageRequired(snapshot) {
		return nil
	}
	now := time.Now()
	if err := resourcebudget.ValidateHostStorage(snapshot.Execution.HostStorage, true, demand, now, 5*time.Second); err != nil {
		return &resourcebudget.LocalStorageError{Code: "host_storage_" + runtimeStorageErrorCode(err), Message: err.Error()}
	}
	disk := snapshot.Execution.DaemonDisk
	if snapshot.Execution.EngineID == "" || !snapshot.Execution.Fresh(now, 5*time.Second) || disk == nil || disk.AvailableBytes == nil || disk.ObservedAt.IsZero() || disk.ObservedAt.After(now) || now.Sub(disk.ObservedAt) > 5*time.Second {
		return &resourcebudget.LocalStorageError{Code: "guest_storage_unavailable", Message: "Fresh filesystem capacity from the current Docker engine is required before new heavy work. Refresh Docker readiness; Windows host free space is a separate observation."}
	}
	if *disk.AvailableBytes == 0 || *disk.AvailableBytes < demand {
		return &resourcebudget.LocalStorageError{Code: "guest_storage_insufficient_capacity", Message: "Docker's own filesystem has insufficient free space for this operation. Review Docker storage and preserve existing work; free Windows host space does not make a full Docker filesystem usable."}
	}
	return nil
}

func runtimeStorageErrorCode(err error) string {
	var detail *resourcebudget.LocalStorageError
	if errors.As(err, &detail) {
		return detail.Code
	}
	return "unavailable"
}

func (s *Server) withRuntimeStorageReadiness(status Status) Status {
	if status.Workspace.Mode == "cloud" {
		// The selected backing reports its own prerequisites. Local Windows
		// storage must not block planning or execution on another machine.
		return status
	}
	snapshot := s.runtimeStorageSnapshot()
	if !runtimeStorageRequired(snapshot) {
		return status
	}
	demand := runtimeInstallStorageDemand
	for _, check := range status.Checks {
		if check.ID == "sandbox-image" && check.Status == "ready" {
			demand = 0 // Installed images still require fresh confirmation; starts have their own demand.
		}
	}
	check := Check{ID: "docker-backing-storage", Label: "Docker storage", Status: "ready", Message: "Host storage is confirmed and Docker filesystem capacity is fresh. Each new operation checks its disk allowance again."}
	if err := validateRuntimeStorage(snapshot, demand); err != nil {
		check.Status, check.Message = "missing", err.Error()
		check.Action = "Review Docker storage in host Settings and inside Docker. Runtime preparation requires a conservative 40 GiB free-space prerequisite on both."
	}
	status.Checks = append(status.Checks, check)
	return status
}
