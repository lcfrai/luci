//go:build windows

package management

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"golang.org/x/sys/windows"
)

// Opt-in OS trust regression only. No process, GPU query, model, network or
// certificate acquisition. A separately reviewed runner supplies the exact
// installed-file identity; ordinary test runs deliberately skip this test.
func TestManagedInstalledGPUTrustAndVersionMetadata(t *testing.T) {
	if os.Getenv("LUCI_TEST_INSTALLED_GPU_TRUST") != "1" {
		t.Skip("requires explicit installed Windows GPU-tool trust scope")
	}
	expectedSHA := os.Getenv("LUCI_TEST_GPU_BINARY_SHA256")
	expectedBytes, err := strconv.ParseInt(os.Getenv("LUCI_TEST_GPU_BINARY_BYTES"), 10, 64)
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(expectedSHA) || err != nil || expectedBytes < 1 || expectedBytes > 512<<20 {
		t.Fatal("reviewed installed GPU-tool hash and byte pins required")
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal("Windows System32 identity unavailable")
	}
	pin, err := pinManagedModelBinary(filepath.Join(system, "nvidia-smi.exe"), "Microsoft Windows Hardware Compatibility Publisher")
	// Even an error may return a retained pin whose cleanup is unknown. Never
	// retry an attempted CloseHandle against a potentially reused handle value.
	defer func() {
		if pin == nil {
			return
		}
		if !pin.closeAttempted {
			pin.closeAttempted = true
			closeErr := windows.CloseHandle(pin.handle)
			pin.closed = closeErr == nil
			if closeErr != nil {
				t.Error("original installed GPU-tool lock closure failed")
			}
		}
		if !pin.closed || pin.cleanupUnknown {
			t.Error("installed GPU-tool closure remains unknown")
		}
	}()
	if err != nil {
		t.Fatalf("installed GPU-tool offline signature verification failed: %v", err)
	}
	if pin == nil || pin.digest != expectedSHA || pin.size != expectedBytes || pin.cleanupUnknown {
		t.Fatal("installed GPU-tool identity differs")
	}
	if err = verifyManagedNVIDIAMetadata(pin.path); err != nil {
		t.Fatalf("installed GPU-tool version verification failed: %v", err)
	}
}
