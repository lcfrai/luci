package dockerbe

import (
	"context"
	"errors"
	"os"
	"runtime"
	"time"

	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func (b *Backend) observeStorage(ctx context.Context, info system.Info) (*resourcebudget.Disk, *resourcebudget.HostStorageBound) {
	return b.observeStorageForPlatform(ctx, info, runtime.GOOS, b.observeDaemonDisk, b.observeHostStorage)
}

// Keep platform dispatch testable without adding a production override. In
// particular, worker readers must not invoke either expensive probe callback.
func (b *Backend) observeStorageForPlatform(ctx context.Context, info system.Info, platform string,
	guestProbe func(context.Context, system.Info) *resourcebudget.Disk,
	hostProbe func(context.Context, system.Info, *resourcebudget.Disk) *resourcebudget.HostStorageBound,
) (*resourcebudget.Disk, *resourcebudget.HostStorageBound) {
	if platform == "windows" {
		if b.resourceStorageReadOnly {
			return b.readSharedStorage(info)
		}
		// Before operator confirmation no new Windows heavy work is admissible.
		// Avoid periodically launching WSL just to rediscover that missing setup.
		if _, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath); e != nil {
			return nil, hostProbe(ctx, info, nil)
		}
	}
	guest := guestProbe(ctx, info)
	bound := hostProbe(ctx, info, guest)
	if platform == "windows" {
		if e := b.publishSharedStorage(info, guest, bound); e != nil {
			return nil, b.sharedStorageUnavailable(resourcebudget.LocalStorageConfirmation{}, "observation_publication_unavailable", "The gateway could not publish its private Windows storage observation. New heavy work waits for a fresh shared observation.")
		}
	}
	return guest, bound
}

func (b *Backend) sharedStorageUnavailable(record resourcebudget.LocalStorageConfirmation, code, message string) *resourcebudget.HostStorageBound {
	s := initialLocalStorageStatus("windows")
	s.ObservedAt = time.Now().UTC()
	s.ReasonCode = code
	s.Message = message
	if record.Valid() {
		s.Revision = record.Revision
		s.ConfirmedAt = record.ConfirmedAt
		s.VHDPath = record.VHDPath
		s.DesktopVersion = record.DesktopVersion
	}
	if code == "confirmation_required" {
		s.State = "confirmation-required"
	}
	b.storeLocalStorageStatus(s)
	return &resourcebudget.HostStorageBound{Required: true, State: s.State, ReasonCode: s.ReasonCode, Message: s.Message}
}

// A worker never falls back to WSL, settings hashing or volume enumeration.
// It binds the gateway's private observation to its own selected engine and
// the entire current operator confirmation, then preserves source timestamps.
func (b *Backend) readSharedStorage(info system.Info) (*resourcebudget.Disk, *resourcebudget.HostStorageBound) {
	record, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath)
	if e != nil {
		code := "confirmation_unreadable"
		message := resourcebudget.ErrLocalStorageConfirmation.Error()
		if errors.Is(e, os.ErrNotExist) || b.resourceStorageConfirmationPath == "" {
			code = "confirmation_required"
			message = "A host Super Admin must confirm the active and pending local Docker storage locations in Settings."
		}
		return nil, b.sharedStorageUnavailable(record, code, message)
	}
	if !localDesktopTarget(b.api, info) {
		return nil, b.sharedStorageUnavailable(record, "unsupported_engine", "The confirmed Windows storage observation applies only to the selected local Docker Desktop Linux engine.")
	}
	cache, e := resourcebudget.ReadLocalStorageObservation(resourcebudget.LocalStorageObservationPath(b.resourceStorageConfirmationPath))
	if e != nil || resourcebudget.ValidateLocalStorageObservation(cache, info.ID, info.DockerRootDir, record, time.Now()) != nil {
		return nil, b.sharedStorageUnavailable(record, "shared_observation_unavailable", resourcebudget.ErrLocalStorageObservation.Error())
	}
	// Refuse concurrent confirmation replacement, even with an unchanged
	// revision; the private record's full canonical identity hash must match.
	latest, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath)
	if e != nil || resourcebudget.LocalStorageConfirmationHash(latest) != cache.ConfirmationSHA256 {
		return nil, b.sharedStorageUnavailable(record, "shared_observation_changed", "Storage confirmation changed during observation. Wait for the gateway to publish a fresh observation.")
	}
	b.storeLocalStorageStatus(cache.Status)
	return cache.DaemonDisk, cache.HostStorage
}

func (b *Backend) publishSharedStorage(info system.Info, guest *resourcebudget.Disk, bound *resourcebudget.HostStorageBound) error {
	if b.resourceStorageReadOnly {
		return errors.New("workspace workers cannot publish host storage observations")
	}
	record, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath)
	if e != nil {
		return e
	}
	status := b.LocalStorageStatus()
	b.storageStatusMu.RLock()
	observedHash := b.storageStatusConfirmationHash
	b.storageStatusMu.RUnlock()
	if observedHash == "" || observedHash != resourcebudget.LocalStorageConfirmationHash(record) || status.Revision != record.Revision || status.VHDPath != record.VHDPath || bound == nil {
		return resourcebudget.ErrLocalStorageObservation
	}
	cache := resourcebudget.LocalStorageObservationCache{Version: 1, EngineID: info.ID, DaemonRoot: info.DockerRootDir, ConfirmationRevision: record.Revision, ConfirmationSHA256: resourcebudget.LocalStorageConfirmationHash(record), ObservedAt: time.Now().UTC(), Status: status, HostStorage: bound, DaemonDisk: guest}
	if resourcebudget.ValidateLocalStorageObservation(cache, info.ID, info.DockerRootDir, record, time.Now()) != nil {
		return resourcebudget.ErrLocalStorageObservation
	}
	return resourcebudget.WriteLocalStorageObservation(resourcebudget.LocalStorageObservationPath(b.resourceStorageConfirmationPath), cache)
}
