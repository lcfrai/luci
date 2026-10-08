//go:build windows

package resourcebudget

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicit native acceptance hook: it never contacts Docker, changes its
// settings, reads VHD contents or publishes an operator confirmation.
func TestWindowsLocalStorageReadOnlyProbe(t *testing.T) {
	path := os.Getenv("LUCI_WINDOWS_STORAGE_PROBE_VHD")
	if path == "" {
		t.Skip("opt in with the actual existing VHD path; read-only native storage observation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	observed, e := ProbeLocalStorage(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	if observed.MountedVolumeCount < 1 || observed.FileIdentity == "" || observed.WindowsSID == "" || observed.SettingsSHA256 == "" {
		t.Fatal("native probe returned incomplete corroborating facts")
	}
	// Raw SID, path, volume GUID, file identity and settings hash stay private
	// in memory. The optional fresh receipt only exports allowlisted results.
	result := map[string]any{
		"passed":                true,
		"scope":                 "Native Windows read-only file attributes, OS identity, Desktop settings fingerprint and complete mounted-local-volume lower bound; no Docker connection, VHD contents or engine-to-VHD binding",
		"observedAt":            observed.ObservedAt,
		"mountedVolumeCount":    observed.MountedVolumeCount,
		"minimumAvailableBytes": observed.MinimumAvailableBytes,
		"limitingVolume":        observed.LimitingVolume,
		"confirmationSaved":     false,
	}
	data, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	data = append(data, '\n')
	if report := os.Getenv("LUCI_WINDOWS_STORAGE_PROBE_REPORT"); report != "" {
		if !filepath.IsAbs(report) {
			t.Fatal("receipt path must be absolute and fresh")
		}
		root, e := OpenLocalStorageDirectory(filepath.Dir(report))
		if e != nil {
			t.Fatal(e)
		}
		defer root.Close()
		f, e := root.OpenFile(filepath.Base(report), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		makeStorageRecordPrivate(t, report)
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	t.Log(string(data))
}
