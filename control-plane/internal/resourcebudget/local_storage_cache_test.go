package resourcebudget

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func storageCacheFixture(record LocalStorageConfirmation) LocalStorageObservationCache {
	now := time.Now().UTC()
	free := uint64(12345)
	bound := &HostStorageBound{Required: true, State: "confirmed", MountedVolumeCount: 2, Disk: &Disk{ObservedAt: now, Basis: LocalStorageBasis, AvailableBytes: &free}}
	return LocalStorageObservationCache{Version: 1, EngineID: record.EngineID, DaemonRoot: "/var/lib/docker", ConfirmationRevision: record.Revision, ConfirmationSHA256: LocalStorageConfirmationHash(record), ObservedAt: now, Status: LocalStorageStatus{State: "confirmed", Revision: record.Revision, ObservedAt: now}, HostStorage: bound, DaemonDisk: &Disk{ObservedAt: now, AvailableBytes: &free}}
}

func TestLocalStorageSharedCacheBindsWholeConfirmationAndMetricTime(t *testing.T) {
	r := storageRecordFixture()
	cache := storageCacheFixture(r)
	now := time.Now()
	if ValidateLocalStorageObservation(cache, r.EngineID, cache.DaemonRoot, r, now) != nil {
		t.Fatal("valid shared observation refused")
	}
	for _, mutate := range []func(*LocalStorageObservationCache){
		func(c *LocalStorageObservationCache) { c.EngineID = "another-engine" },
		func(c *LocalStorageObservationCache) { c.DaemonRoot = "/other" },
		func(c *LocalStorageObservationCache) { c.ConfirmationRevision++ },
		func(c *LocalStorageObservationCache) { c.ConfirmationSHA256 = "changed" },
		func(c *LocalStorageObservationCache) { c.ObservedAt = now.Add(-6 * time.Second) },
		func(c *LocalStorageObservationCache) { c.ObservedAt = now.Add(time.Second) },
		func(c *LocalStorageObservationCache) {
			d := *c.DaemonDisk
			d.ObservedAt = now.Add(-6 * time.Second)
			c.DaemonDisk = &d
		},
		func(c *LocalStorageObservationCache) {
			h := *c.HostStorage
			d := *h.Disk
			d.ObservedAt = now.Add(-6 * time.Second)
			h.Disk = &d
			c.HostStorage = &h
		},
	} {
		changed := cache
		mutate(&changed)
		if ValidateLocalStorageObservation(changed, r.EngineID, cache.DaemonRoot, r, now) == nil {
			t.Fatal("mismatched or stale shared observation accepted")
		}
	}
	changed := r
	changed.FileIdentity = "replaced-without-revision-change"
	if ValidateLocalStorageObservation(cache, r.EngineID, cache.DaemonRoot, changed, now) == nil {
		t.Fatal("same revision with different pin identity accepted")
	}
	// A restarted gateway cannot refresh timestamps merely by rereading disk.
	if ValidateLocalStorageObservation(cache, r.EngineID, cache.DaemonRoot, r, now.Add(6*time.Second)) == nil {
		t.Fatal("restart extended expired observation")
	}
}

func TestLocalStorageSharedCachePrivateAtomicPublication(t *testing.T) {
	record := storageRecordFixture()
	cache := storageCacheFixture(record)
	path := filepath.Join(t.TempDir(), "windows-storage-observation.json")
	for i := 0; i < 2; i++ {
		if e := WriteLocalStorageObservation(path, cache); e != nil {
			t.Fatal(e)
		}
		read, e := ReadLocalStorageObservation(path)
		if e != nil {
			t.Fatal(e)
		}
		if ValidateLocalStorageObservation(read, record.EngineID, cache.DaemonRoot, record, time.Now()) != nil {
			t.Fatal("published observation lost its binding")
		}
	}
	if runtime.GOOS != "windows" {
		if e := os.Chmod(path, 0644); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadLocalStorageObservation(path); e == nil {
			t.Fatal("publicly readable cache accepted")
		}
		if e := os.Chmod(path, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(path, []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	makeStorageRecordPrivate(t, path)
	if _, e := ReadLocalStorageObservation(path); e == nil {
		t.Fatal("partial cache accepted")
	}
}
