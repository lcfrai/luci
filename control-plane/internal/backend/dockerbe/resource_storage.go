package dockerbe

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type desktopVersionClient interface {
	ServerVersion(context.Context) (types.Version, error)
}
type storageInfoClient interface {
	Info(context.Context) (system.Info, error)
}
type localStorageProbe func(context.Context, string) (resourcebudget.LocalStorageObservation, error)

// LocalStorageStatus reads the observer's cache only. It must be exposed only
// through current host-operator authorization; it includes the confirmed path.
func (b *Backend) LocalStorageStatus() resourcebudget.LocalStorageStatus {
	return b.localStorageStatusAt(time.Now())
}

func (b *Backend) localStorageStatusAt(now time.Time) resourcebudget.LocalStorageStatus {
	b.storageStatusMu.RLock()
	s := b.storageStatus
	b.storageStatusMu.RUnlock()
	if s.Platform == "" {
		s = initialLocalStorageStatus(runtime.GOOS)
	}
	if s.State == "confirmed" && (s.ObservedAt.IsZero() || s.ObservedAt.After(now) || now.Sub(s.ObservedAt) > 5*time.Second) {
		s.State = "unavailable"
		s.ReasonCode = "observation_stale"
		s.Message = "The last storage observation is stale. Start or reconnect Docker Desktop and wait for a fresh sample; the saved operator confirmation has not changed."
		s.HostAvailableBytes = nil
		s.GuestAvailableBytes = nil
		s.MountedVolumeCount = 0
		s.LimitingVolume = ""
	}
	if s.HostAvailableBytes != nil {
		n := *s.HostAvailableBytes
		s.HostAvailableBytes = &n
	}
	if s.GuestAvailableBytes != nil {
		n := *s.GuestAvailableBytes
		s.GuestAvailableBytes = &n
	}
	return s
}

func initialLocalStorageStatus(platform string) resourcebudget.LocalStorageStatus {
	s := resourcebudget.LocalStorageStatus{Platform: platform, Scope: "not-required", State: "not-required", Basis: resourcebudget.LocalStorageBasis, Message: "Windows Docker Desktop backing-storage confirmation is not required on this platform."}
	if platform == "windows" {
		s.Required = true
		s.Scope = "local-docker-desktop"
		s.State = "unavailable"
		s.ReasonCode = "observation_required"
		s.Message = "Start the local Docker Desktop Linux engine, then review its active and pending disk locations in host Settings."
	}
	return s
}

func (b *Backend) storeLocalStorageStatus(s resourcebudget.LocalStorageStatus) {
	b.storeLocalStorageStatusForConfirmation(s, "")
}
func (b *Backend) storeLocalStorageStatusForConfirmation(s resourcebudget.LocalStorageStatus, hash string) {
	b.storageStatusMu.Lock()
	b.storageStatus = s
	b.storageStatusConfirmationHash = hash
	b.storageStatusMu.Unlock()
}

func localDesktopTarget(api APIClient, info system.Info) bool {
	endpoint, ok := api.(interface{ DaemonHost() string })
	return ok && localDesktopEndpoint(endpoint.DaemonHost()) && info.OSType == "linux" && strings.Contains(strings.ToLower(info.OperatingSystem), "docker desktop") && validDaemonDiskTarget(info.DockerRootDir, info.ID)
}

func localDesktopEndpoint(endpoint string) bool {
	const prefix = "npipe:////./pipe/"
	if !strings.HasPrefix(strings.ToLower(endpoint), prefix) {
		return false
	}
	name := endpoint[len(prefix):]
	if len(name) == 0 || len(name) > 128 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return name != "." && name != ".."
}

func desktopStorageVersion(ctx context.Context, api APIClient) (string, error) {
	versions, ok := api.(desktopVersionClient)
	if !ok {
		return "", errors.New("Docker Desktop version could not be verified from this engine")
	}
	v, e := versions.ServerVersion(ctx)
	if e != nil {
		return "", errors.New("Docker Desktop version could not be verified from this engine")
	}
	name := strings.TrimSpace(v.Platform.Name)
	if !strings.HasPrefix(name, "Docker Desktop ") || len(name) > 200 || strings.ContainsAny(name, "\x00\r\n") {
		return "", errors.New("This engine did not report a Docker Desktop release; confirm that the selected local endpoint belongs to Docker Desktop")
	}
	return name, nil
}

// PrepareLocalStorageConfirmation only inspects. The management gateway must
// recheck current host authority, compare the saved revision and privately
// publish the resulting pins; this method never writes configuration.
func (b *Backend) PrepareLocalStorageConfirmation(ctx context.Context, path string) (resourcebudget.LocalStorageConfirmation, error) {
	if runtime.GOOS != "windows" {
		return resourcebudget.LocalStorageConfirmation{}, errors.New("Local Windows Docker storage confirmation is not required on this platform")
	}
	return b.prepareLocalStorageConfirmation(ctx, path, resourcebudget.ProbeLocalStorage, probeDaemonDisk)
}

func (b *Backend) prepareLocalStorageConfirmation(ctx context.Context, path string, probe localStorageProbe, guestProbe func(context.Context, APIClient, system.Info) (resourcebudget.Disk, error)) (resourcebudget.LocalStorageConfirmation, error) {
	var r resourcebudget.LocalStorageConfirmation
	if b.resourceStorageReadOnly {
		return r, errors.New("Only the host gateway can inspect and prepare a Windows storage confirmation")
	}
	if b.resourceStorageConfirmationPath == "" || !resourcebudget.ValidLocalVHDPath(path) {
		return r, errors.New("Choose the existing local VHD path in host Settings; storage confirmation must use the trusted host configuration directory")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api, ok := b.api.(storageInfoClient)
	if !ok {
		return r, errors.New("Docker Desktop observation is unavailable")
	}
	info, e := api.Info(ctx)
	if e != nil || !localDesktopTarget(b.api, info) {
		return r, errors.New("Storage confirmation requires the local Docker Desktop Linux engine")
	}
	version, e := desktopStorageVersion(ctx, b.api)
	if e != nil {
		return r, e
	}
	// This proves the guest observation belongs to the selected engine. It
	// does not prove the operator-selected host VHD backs that engine.
	if _, e = guestProbe(ctx, b.api, info); e != nil {
		return r, errors.New("The selected Docker engine's guest filesystem could not be observed through WSL; start Docker Desktop and retry")
	}
	observed, e := probe(ctx, path)
	if e != nil {
		return r, e
	}
	latest, e := api.Info(ctx)
	if e != nil || latest.ID != info.ID || latest.DockerRootDir != info.DockerRootDir || !localDesktopTarget(b.api, latest) || ctx.Err() != nil {
		return r, errors.New("Docker changed during storage review; retry confirmation")
	}
	latestVersion, e := desktopStorageVersion(ctx, b.api)
	if e != nil || latestVersion != version {
		return r, errors.New("Docker Desktop changed during storage review; retry confirmation")
	}
	return resourcebudget.LocalStorageConfirmation{Version: 1, EngineID: info.ID, DesktopVersion: version, WindowsSID: observed.WindowsSID, VHDPath: observed.VHDPath, FileIdentity: observed.FileIdentity, VolumeGUID: observed.VolumeGUID, SettingsSHA256: observed.SettingsSHA256}, nil
}

func (b *Backend) observeHostStorage(ctx context.Context, info system.Info, guest *resourcebudget.Disk) *resourcebudget.HostStorageBound {
	return b.observeHostStorageForPlatform(ctx, info, guest, runtime.GOOS, resourcebudget.ProbeLocalStorage)
}

func (b *Backend) observeHostStorageForPlatform(ctx context.Context, info system.Info, guest *resourcebudget.Disk, platform string, probe localStorageProbe) *resourcebudget.HostStorageBound {
	s := initialLocalStorageStatus(platform)
	if platform != "windows" {
		b.storeLocalStorageStatus(s)
		return nil
	}
	bound := &resourcebudget.HostStorageBound{Required: true}
	confirmationHash := ""
	defer func() {
		if s.ObservedAt.IsZero() {
			s.ObservedAt = time.Now().UTC()
		}
		b.storeLocalStorageStatusForConfirmation(s, confirmationHash)
		bound.State = s.State
		bound.ReasonCode = s.ReasonCode
		bound.Message = s.Message
	}()
	if !localDesktopTarget(b.api, info) {
		s.Scope = "unsupported"
		s.ReasonCode = "unsupported_engine"
		s.Message = "This Windows capacity adapter supports the local Docker Desktop Linux engine with operator-confirmed mounted local storage."
		return bound
	}
	if guest != nil && guest.AvailableBytes != nil {
		n := *guest.AvailableBytes
		s.GuestAvailableBytes = &n
	}
	record, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath)
	if e != nil {
		if errors.Is(e, os.ErrNotExist) || b.resourceStorageConfirmationPath == "" {
			s.State = "confirmation-required"
			s.ReasonCode = "confirmation_required"
			s.Message = "A host Super Admin must confirm the active and pending local Docker disk locations in Settings before new heavy work."
		} else {
			s.ReasonCode = "confirmation_unreadable"
			s.Message = resourcebudget.ErrLocalStorageConfirmation.Error()
		}
		return bound
	}
	s.Revision = record.Revision
	confirmationHash = resourcebudget.LocalStorageConfirmationHash(record)
	s.ConfirmedAt = record.ConfirmedAt
	s.VHDPath = record.VHDPath
	s.DesktopVersion = record.DesktopVersion
	version, e := desktopStorageVersion(ctx, b.api)
	if e != nil {
		s.ReasonCode = "desktop_version_unavailable"
		s.Message = e.Error()
		return bound
	}
	observation, e := probe(ctx, record.VHDPath)
	if e != nil {
		s.ReasonCode = "host_storage_unavailable"
		s.Message = e.Error()
		return bound
	}
	if e = resourcebudget.ValidateLocalStoragePins(record, info.ID, version, observation); e != nil {
		s.State = "invalidated"
		s.ReasonCode = "confirmation_changed"
		s.Message = e.Error()
		return bound
	}
	// The background observation must still refer to the same private record;
	// a simultaneous operator update cannot inherit the old measured pins.
	latest, e := resourcebudget.ReadLocalStorageConfirmation(b.resourceStorageConfirmationPath)
	if e != nil || latest != record || ctx.Err() != nil {
		s.ReasonCode = "confirmation_changed_during_observation"
		s.Message = "Storage confirmation changed during observation; wait for the next fresh sample."
		return bound
	}
	if observation.ObservedAt.IsZero() || observation.ObservedAt.After(time.Now()) || time.Since(observation.ObservedAt) > 5*time.Second || observation.MountedVolumeCount < 1 {
		s.ReasonCode = "host_storage_unavailable"
		s.Message = "A fresh complete mounted local-volume observation is required."
		return bound
	}
	s.State = "confirmed"
	s.ObservedAt = observation.ObservedAt
	if guest != nil && !guest.ObservedAt.IsZero() && guest.ObservedAt.Before(s.ObservedAt) {
		s.ObservedAt = guest.ObservedAt
	}
	s.ReasonCode = ""
	s.Message = "Local layout is operator-confirmed. Physical headroom is the lowest available space across every mounted writable local volume, including unrelated drives; guest capacity is measured separately."
	value := observation.MinimumAvailableBytes
	s.HostAvailableBytes = &value
	s.MountedVolumeCount = observation.MountedVolumeCount
	s.LimitingVolume = observation.LimitingVolume
	bound.Disk = &resourcebudget.Disk{ObservedAt: observation.ObservedAt, Basis: resourcebudget.LocalStorageBasis, Role: "docker-host-storage", AvailableBytes: &value}
	bound.MountedVolumeCount = observation.MountedVolumeCount
	return bound
}
