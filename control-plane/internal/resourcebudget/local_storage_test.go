package resourcebudget

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const storageTestVolume = "\\\\?\\Volume{12345678-1234-1234-1234-123456789abc}\\"
const storageOtherVolume = "\\\\?\\Volume{22345678-1234-1234-1234-123456789abc}\\"

func TestLocalStorageStatusOmitsUnobservedTimes(t *testing.T) {
	data, e := json.Marshal(LocalStorageStatus{State: "confirmation-required"})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "observedAt") || strings.Contains(string(data), "confirmedAt") {
		t.Fatal("zero timestamps presented as observations", string(data))
	}
}

func storageRecordFixture() LocalStorageConfirmation {
	return LocalStorageConfirmation{Version: 1, Revision: 1, ConfirmedAt: time.Now().UTC(), ConfirmedBy: "operator", EngineID: "engine", DesktopVersion: "Docker Desktop 4.48.0 (207573)", WindowsSID: "S-1-5-21-1", VHDPath: "E:\\Docker\\disk\\docker_data.vhdx", FileIdentity: "1234:abcd:created", VolumeGUID: storageTestVolume, SettingsSHA256: strings.Repeat("a", 64)}
}

func TestLocalStorageRejectsNonLocalAndAmbiguousPaths(t *testing.T) {
	for _, path := range []string{"E:\\Docker\\disk\\docker_data.vhdx", "C:\\mounted-folder\\disk.vhd"} {
		if !ValidLocalVHDPath(path) {
			t.Fatalf("normal path rejected %q", path)
		}
	}
	for _, path := range []string{"", "E:disk.vhdx", "\\\\server\\share\\disk.vhdx", "\\\\?\\E:\\disk.vhdx", "\\\\.\\PhysicalDrive0", "E:\\disk.vhdx:stream", "E:\\..\\disk.vhdx", "E:\\\\disk.vhdx", "E:\\space \\disk.vhdx", "E:\\COM1.vhdx", "E:\\COM¹.vhdx", "E:\\NUL\\disk.vhdx", "E:\\line\nbreak.vhdx", "E:\\disk.txt"} {
		if ValidLocalVHDPath(path) {
			t.Fatalf("ambiguous path accepted %q", path)
		}
	}
}

func TestLocalStorageCompleteInventoryIncludesUnrelatedLowVolume(t *testing.T) {
	rows := []localVolume{{guid: storageTestVolume, signature: "E:\\", label: "E:\\", available: 900}, {guid: storageOtherVolume, signature: "C:\\mounted\\", label: "Mounted local volume 2", available: 3}}
	minimum, label, e := localStorageVolumeMinimum(rows, rows, storageTestVolume)
	if e != nil || minimum != 3 || label != "Mounted local volume 2" {
		t.Fatalf("unrelated mounted low volume omitted: %d %s %v", minimum, label, e)
	}
	rows[1].available = 0
	if n, _, e := localStorageVolumeMinimum(rows, rows, storageTestVolume); e != nil || n != 0 {
		t.Fatal("measured full volume was not retained", n, e)
	}
	changed := append([]localVolume(nil), rows...)
	changed[1].signature = "F:\\"
	for _, after := range [][]localVolume{nil, rows[:1], changed} {
		if _, _, e := localStorageVolumeMinimum(rows, after, storageTestVolume); e == nil {
			t.Fatal("partial/changed volume inventory accepted")
		}
	}
	if _, _, e := localStorageVolumeMinimum(rows, rows, "\\\\?\\Volume{99999999-1234-1234-1234-123456789abc}\\"); e == nil {
		t.Fatal("unobserved target accepted")
	}
	for _, kind := range []uint32{2, 3, 6} {
		if yes, e := localStorageVolumeEligible(true, kind, 0); e != nil || !yes {
			t.Fatal("local writable volume omitted", kind)
		}
	}
	for _, test := range []struct {
		mounted     bool
		kind, flags uint32
	}{{false, 0, 0}, {true, 5, 0}, {true, 3, 0x00080000}} {
		if yes, e := localStorageVolumeEligible(test.mounted, test.kind, test.flags); e != nil || yes {
			t.Fatal("ineligible volume counted", test)
		}
	}
	for _, kind := range []uint32{0, 1, 4, 7} {
		if _, e := localStorageVolumeEligible(true, kind, 0); e == nil {
			t.Fatal("ambiguous/remote mounted volume accepted", kind)
		}
	}
}

func TestLocalStoragePinsExpireIndependently(t *testing.T) {
	r := storageRecordFixture()
	o := LocalStorageObservation{WindowsSID: r.WindowsSID, VHDPath: r.VHDPath, FileIdentity: r.FileIdentity, VolumeGUID: r.VolumeGUID, SettingsSHA256: r.SettingsSHA256}
	if !r.Valid() || ValidateLocalStoragePins(r, r.EngineID, r.DesktopVersion, o) != nil {
		t.Fatal("valid confirmation rejected")
	}
	for _, mutate := range []func(*LocalStorageObservation){func(o *LocalStorageObservation) { o.WindowsSID += "2" }, func(o *LocalStorageObservation) { o.VHDPath = "F:\\disk.vhdx" }, func(o *LocalStorageObservation) { o.FileIdentity += "2" }, func(o *LocalStorageObservation) { o.VolumeGUID = storageOtherVolume }, func(o *LocalStorageObservation) { o.SettingsSHA256 = strings.Repeat("b", 64) }} {
		changed := o
		mutate(&changed)
		if ValidateLocalStoragePins(r, r.EngineID, r.DesktopVersion, changed) == nil {
			t.Fatal("changed native pin accepted")
		}
	}
	if ValidateLocalStoragePins(r, "another-engine", r.DesktopVersion, o) == nil || ValidateLocalStoragePins(r, r.EngineID, "Docker Desktop 4.49.0", o) == nil {
		t.Fatal("engine/version change retained confirmation")
	}
}

func TestLocalStorageAdmissionDoesNotConfuseMissingAndZero(t *testing.T) {
	now := time.Now()
	zero := uint64(0)
	good := HostStorageBound{Required: true, State: "confirmed", MountedVolumeCount: 2, Disk: &Disk{ObservedAt: now, AvailableBytes: &zero, Basis: LocalStorageBasis}}
	if e := ValidateHostStorage(&good, true, 1, now, 5*time.Second); e == nil {
		t.Fatal("full volume admitted demand")
	} else {
		var typed *LocalStorageError
		if !errors.As(e, &typed) || typed.Code != "insufficient_capacity" {
			t.Fatal(e)
		}
	}
	if e := ValidateHostStorage(nil, false, 1, now, 5*time.Second); e != nil {
		t.Fatal("non-Windows behavior changed", e)
	}
	for _, bound := range []*HostStorageBound{nil, {}, {Required: true, State: "confirmed"}, {Required: true, State: "confirmed", MountedVolumeCount: 1, Disk: &Disk{ObservedAt: now.Add(-6 * time.Second), AvailableBytes: &zero}}} {
		if ValidateHostStorage(bound, true, 0, now, 5*time.Second) == nil {
			t.Fatal("missing/stale Windows capacity accepted")
		}
	}
	good.Disk.AvailableBytes = ptr(uint64(100))
	if ValidateHostStorage(&good, true, 100, now, 5*time.Second) != nil {
		t.Fatal("exact measured capacity refused")
	}
	good.State = "invalidated"
	var typed *LocalStorageError
	if !errors.As(ValidateHostStorage(&good, true, 1, now, 5*time.Second), &typed) || typed.Code != "confirmation_required" {
		t.Fatal("invalidation not actionable")
	}
}

func TestLocalStorageRecordRejectsUnsafePublication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-storage-confirmation.json")
	data, _ := json.Marshal(storageRecordFixture())
	write := func(b []byte) {
		t.Helper()
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		makeStorageRecordPrivate(t, path)
	}
	write(data)
	if _, e := ReadLocalStorageConfirmation(path); e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{append(append([]byte(nil), data...), []byte("{}")...), []byte("{\"version\":1}"), []byte(strings.Repeat(" ", (16<<10)+1))} {
		write(bad)
		if _, e := ReadLocalStorageConfirmation(path); e == nil {
			t.Fatal("invalid record accepted")
		}
	}
	write(data)
	t.Run("hardlink", func(t *testing.T) {
		link := filepath.Join(dir, "other.json")
		if e := os.Link(path, link); e != nil {
			t.Skip(e)
		}
		defer os.Remove(link)
		if _, e := ReadLocalStorageConfirmation(path); e == nil {
			t.Fatal("multiply linked policy accepted")
		}
	})
	t.Run("parent-symlink", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "alias")
		if e := os.Symlink(dir, link); e != nil {
			t.Skip(e)
		}
		if _, e := ReadLocalStorageConfirmation(filepath.Join(link, filepath.Base(path))); e == nil {
			t.Fatal("linked parent accepted")
		}
	})
}
