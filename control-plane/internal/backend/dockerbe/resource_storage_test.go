package dockerbe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type storageAPI struct {
	APIClient
	info                    system.Info
	version                 string
	infoError               error
	infoCalls, versionCalls int
	changeAfterFirst        bool
}

func TestLocalDesktopEndpointRejectsRemotePipes(t *testing.T) {
	for _, endpoint := range []string{"npipe:////./pipe/docker_engine", "npipe:////./pipe/dockerDesktopLinuxEngine"} {
		if !localDesktopEndpoint(endpoint) {
			t.Fatal("canonical local pipe rejected", endpoint)
		}
	}
	for _, endpoint := range []string{"npipe:////server/pipe/docker_engine", "npipe:////./pipe/", "npipe:////./pipe/../docker", "tcp://127.0.0.1:2375", "npipe:////./pipe/docker?other"} {
		if localDesktopEndpoint(endpoint) {
			t.Fatal("remote or ambiguous pipe treated as local", endpoint)
		}
	}
}

func (a *storageAPI) DaemonHost() string { return "npipe:////./pipe/docker_engine" }
func (a *storageAPI) Info(context.Context) (system.Info, error) {
	a.infoCalls++
	v := a.info
	if a.changeAfterFirst && a.infoCalls > 1 {
		v.ID = "changed"
	}
	return v, a.infoError
}
func (a *storageAPI) ServerVersion(context.Context) (types.Version, error) {
	a.versionCalls++
	var v types.Version
	v.Platform.Name = a.version
	return v, nil
}
func (a *storageAPI) ContainerStatsOneShot(context.Context, string) (container.StatsResponseReader, error) {
	return container.StatsResponseReader{}, errors.New("unexpected stats")
}

func storageBackendFixture(t *testing.T) (*Backend, *storageAPI, resourcebudget.LocalStorageConfirmation, resourcebudget.LocalStorageObservation) {
	t.Helper()
	api := &storageAPI{info: system.Info{ID: "engine", OSType: "linux", OperatingSystem: "Docker Desktop", DockerRootDir: "/var/lib/docker"}, version: "Docker Desktop 4.48.0 (207573)"}
	record := resourcebudget.LocalStorageConfirmation{Version: 1, Revision: 1, ConfirmedAt: time.Now().UTC(), ConfirmedBy: "operator", EngineID: "engine", DesktopVersion: api.version, WindowsSID: "S-1-5-21-1", VHDPath: "E:\\Docker\\disk\\docker_data.vhdx", FileIdentity: "file-identity", VolumeGUID: "\\\\?\\Volume{12345678-1234-1234-1234-123456789abc}\\", SettingsSHA256: strings.Repeat("a", 64)}
	observed := resourcebudget.LocalStorageObservation{WindowsSID: record.WindowsSID, VHDPath: record.VHDPath, FileIdentity: record.FileIdentity, VolumeGUID: record.VolumeGUID, SettingsSHA256: record.SettingsSHA256, ObservedAt: time.Now().UTC(), MinimumAvailableBytes: 3 << 30, MountedVolumeCount: 3, LimitingVolume: "C:\\"}
	b := &Backend{api: api, resourceStorageConfirmationPath: filepath.Join(t.TempDir(), "local-storage-confirmation.json")}
	return b, api, record, observed
}
func writeStorageRecord(t *testing.T, path string, record resourcebudget.LocalStorageConfirmation) {
	t.Helper()
	data, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	privateStorageTestFile(t, path)
}

func TestHostStorageBoundPreservesGuestAndExpiresPins(t *testing.T) {
	b, api, record, observed := storageBackendFixture(t)
	writeStorageRecord(t, b.resourceStorageConfirmationPath, record)
	guestFree := uint64(900 << 30)
	guest := &resourcebudget.Disk{ObservedAt: time.Now(), AvailableBytes: &guestFree}
	probe := func(context.Context, string) (resourcebudget.LocalStorageObservation, error) { return observed, nil }
	bound := b.observeHostStorageForPlatform(context.Background(), api.info, guest, "windows", probe)
	if bound.State != "confirmed" || bound.Disk == nil || *bound.Disk.AvailableBytes != 3<<30 || bound.Disk.TotalBytes != nil || *guest.AvailableBytes != 900<<30 {
		t.Fatalf("guest and host scopes confused: %+v", bound)
	}
	admin := b.LocalStorageStatus()
	if admin.LimitingVolume != "C:\\" || admin.VHDPath != record.VHDPath || admin.GuestAvailableBytes == nil || *admin.GuestAvailableBytes != guestFree {
		t.Fatalf("operator explanation absent %+v", admin)
	}
	public, _ := json.Marshal(bound)
	for _, private := range []string{record.VHDPath, record.WindowsSID, record.SettingsSHA256, record.FileIdentity, "C:\\"} {
		if strings.Contains(string(public), private) {
			t.Fatal("private host identity entered team observation")
		}
	}
	observed.SettingsSHA256 = strings.Repeat("b", 64)
	bound = b.observeHostStorageForPlatform(context.Background(), api.info, guest, "windows", probe)
	if bound.State != "invalidated" || bound.Disk != nil {
		t.Fatal("changed Desktop settings retained capacity", bound)
	}
}

func TestHostStorageCacheExpiresWithoutProbeOrLosingConfirmation(t *testing.T) {
	b, api, record, observed := storageBackendFixture(t)
	writeStorageRecord(t, b.resourceStorageConfirmationPath, record)
	probeCalls := 0
	b.observeHostStorageForPlatform(context.Background(), api.info, nil, "windows", func(context.Context, string) (resourcebudget.LocalStorageObservation, error) {
		probeCalls++
		return observed, nil
	})
	cached := b.LocalStorageStatus()
	at := cached.ObservedAt
	api.infoError = errors.New("engine stopped")
	result := b.ObserveResources(context.Background())
	if result.Status != "unavailable" {
		t.Fatal("failed Info reported ready")
	}
	infoCalls, versionCalls := api.infoCalls, api.versionCalls
	stale := b.localStorageStatusAt(at.Add(6 * time.Second))
	if stale.State != "unavailable" || stale.ReasonCode != "observation_stale" || stale.HostAvailableBytes != nil || stale.GuestAvailableBytes != nil || stale.VHDPath != record.VHDPath || stale.Revision != record.Revision {
		t.Fatalf("stale observation leaked or confirmation lost: %+v", stale)
	}
	if api.infoCalls != infoCalls || api.versionCalls != versionCalls || probeCalls != 1 {
		t.Fatal("status getter performed a probe")
	}
	fresh := b.localStorageStatusAt(at.Add(time.Second))
	*fresh.HostAvailableBytes = 0
	if *b.localStorageStatusAt(at.Add(time.Second)).HostAvailableBytes != observed.MinimumAvailableBytes {
		t.Fatal("caller mutated cached available bytes")
	}
}

func TestHostStorageMissingPartialAndLinuxScopes(t *testing.T) {
	b, api, record, observed := storageBackendFixture(t)
	called := 0
	probe := func(context.Context, string) (resourcebudget.LocalStorageObservation, error) {
		called++
		return observed, nil
	}
	if got := b.observeHostStorageForPlatform(context.Background(), api.info, nil, "linux", probe); got != nil || called != 0 || api.versionCalls != 0 {
		t.Fatal("Linux acquired Windows confirmation dependency")
	}
	missing := b.observeHostStorageForPlatform(context.Background(), api.info, nil, "windows", probe)
	if missing.State != "confirmation-required" || missing.Disk != nil || called != 0 {
		t.Fatal("missing confirmation measured capacity", missing)
	}
	writeStorageRecord(t, b.resourceStorageConfirmationPath, record)
	unavailable := b.observeHostStorageForPlatform(context.Background(), api.info, nil, "windows", func(context.Context, string) (resourcebudget.LocalStorageObservation, error) {
		return resourcebudget.LocalStorageObservation{}, errors.New("complete mounted inventory unavailable")
	})
	if unavailable.State != "unavailable" || unavailable.Disk != nil {
		t.Fatal("incomplete inventory was ready")
	}
}

func TestPrepareStorageConfirmationIsReadOnlyAndDetectsEngineChange(t *testing.T) {
	b, api, _, observed := storageBackendFixture(t)
	probe := func(context.Context, string) (resourcebudget.LocalStorageObservation, error) { return observed, nil }
	guest := func(context.Context, APIClient, system.Info) (resourcebudget.Disk, error) {
		return resourcebudget.Disk{}, nil
	}
	prepared, e := b.prepareLocalStorageConfirmation(context.Background(), observed.VHDPath, probe, guest)
	if e != nil || prepared.EngineID != api.info.ID || prepared.Revision != 0 || prepared.ConfirmedBy != "" {
		t.Fatal("read-only preparation invented authority", prepared, e)
	}
	if _, e = os.Lstat(b.resourceStorageConfirmationPath); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("preparation wrote configuration")
	}
	api.infoCalls = 0
	api.changeAfterFirst = true
	if _, e = b.prepareLocalStorageConfirmation(context.Background(), observed.VHDPath, probe, guest); e == nil {
		t.Fatal("engine swap during confirmation accepted")
	}
	api.infoCalls = 0
	api.changeAfterFirst = false
	api.version = "Docker Engine - Community"
	if _, e = b.prepareLocalStorageConfirmation(context.Background(), observed.VHDPath, probe, guest); e == nil {
		t.Fatal("engine version substituted for Desktop version")
	}
}

func TestSharedWindowsStorageTwoWorkersReadWithoutProbes(t *testing.T) {
	producer, api, record, observed := storageBackendFixture(t)
	writeStorageRecord(t, producer.resourceStorageConfirmationPath, record)
	free := uint64(100 << 30)
	guest := &resourcebudget.Disk{ObservedAt: time.Now().UTC(), AvailableBytes: &free}
	bound := producer.observeHostStorageForPlatform(context.Background(), api.info, guest, "windows", func(context.Context, string) (resourcebudget.LocalStorageObservation, error) { return observed, nil })
	if e := producer.publishSharedStorage(api.info, guest, bound); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		workerAPI := &storageAPI{info: api.info, version: api.version}
		worker := &Backend{api: workerAPI, resourceStorageConfirmationPath: producer.resourceStorageConfirmationPath, resourceStorageReadOnly: true}
		disk, host := worker.readSharedStorage(api.info)
		if disk == nil || host.State != "confirmed" || *host.Disk.AvailableBytes != observed.MinimumAvailableBytes || workerAPI.infoCalls != 0 || workerAPI.versionCalls != 0 {
			t.Fatal("worker did not use the shared observation without probes")
		}
		forbidden := func(context.Context, string) (resourcebudget.LocalStorageObservation, error) {
			t.Fatal("worker called native probe")
			return observed, nil
		}
		if _, e := worker.prepareLocalStorageConfirmation(context.Background(), observed.VHDPath, forbidden, func(context.Context, APIClient, system.Info) (resourcebudget.Disk, error) {
			t.Fatal("worker called WSL probe")
			return resourcebudget.Disk{}, nil
		}); e == nil {
			t.Fatal("worker prepared host confirmation")
		}
		other := api.info
		other.ID = "foreign"
		disk, host = worker.readSharedStorage(other)
		if disk != nil || host.State == "confirmed" {
			t.Fatal("cross-engine shared storage accepted")
		}
	}
	// Changing a pin without increasing revision must invalidate cache readers
	// and also prevent the producer relabelling its older observation.
	record.SettingsSHA256 = strings.Repeat("c", 64)
	writeStorageRecord(t, producer.resourceStorageConfirmationPath, record)
	worker := &Backend{api: api, resourceStorageConfirmationPath: producer.resourceStorageConfirmationPath, resourceStorageReadOnly: true}
	if disk, host := worker.readSharedStorage(api.info); disk != nil || host.State == "confirmed" {
		t.Fatal("old confirmation identity accepted")
	}
	if e := producer.publishSharedStorage(api.info, guest, bound); e == nil {
		t.Fatal("producer relabelled old observation with new confirmation hash")
	}
}
