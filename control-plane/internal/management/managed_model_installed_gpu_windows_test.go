//go:build windows

package management

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
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
	path := filepath.Join(system, "nvidia-smi.exe")
	if err = managedModelOrdinaryPath(path, false); err != nil {
		t.Fatal("ordinary System32 tool required")
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal("System32 tool path unavailable")
	}
	identity, err := windows.CreateFile(wide, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal("installed tool identity lock unavailable")
	}
	reader := os.NewFile(uintptr(identity), path)
	if reader == nil {
		if closeErr := windows.CloseHandle(identity); closeErr != nil {
			t.Error("installed tool identity lock closure failed")
		}
		t.Fatal("installed tool identity reader unavailable")
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error("installed tool identity lock closure failed")
		}
	}()
	info, err := reader.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedBytes {
		t.Fatal("installed tool byte identity differs")
	}
	sum := sha256.New()
	n, err := io.Copy(sum, io.LimitReader(reader, expectedBytes+1))
	if err != nil || n != expectedBytes || hex.EncodeToString(sum.Sum(nil)) != expectedSHA {
		t.Fatal("installed tool digest identity differs")
	}
	// Retain the checked exact-file identity lock through trust, even when the
	// production pin rejects before its own hash step. No writes/deletes can race.
	pin, err := pinManagedModelBinary(path, "Microsoft Windows Hardware Compatibility Publisher")
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
		var mismatch *managedModelVendorMismatch
		if errors.As(err, &mismatch) {
			// Only a successful offline WinTrust state creates this detail. Keep
			// diagnostic evidence to a bounded printable public certificate name.
			printable := len(mismatch.publisher) > 0 && len(mismatch.publisher) <= 128
			for _, c := range mismatch.publisher {
				if c < 32 || c > 126 {
					printable = false
				}
			}
			if printable && (mismatch.verifiedVia == "embedded" || mismatch.verifiedVia == "catalog") {
				t.Logf("verified GPU-tool signer route=%s publicLeaf=%q; vendor policy still refused", mismatch.verifiedVia, mismatch.publisher)
			}
		}
		t.Fatalf("installed GPU-tool offline signature verification failed: %v", err)
	}
	if pin == nil || pin.digest != expectedSHA || pin.size != expectedBytes || pin.cleanupUnknown {
		t.Fatal("installed GPU-tool identity differs")
	}
	if err = verifyManagedNVIDIAMetadata(pin.path); err != nil {
		t.Fatalf("installed GPU-tool version verification failed: %v", err)
	}
}
