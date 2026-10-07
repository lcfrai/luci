//go:build unix

package product

import (
	"os"
	"syscall"
	"testing"
)

func TestPreviewArchiveCanonicalModesWithRestrictiveUmask(t *testing.T) {
	// Product tests do not run in parallel. Keep this process-local umask
	// bounded to the regression and restore it for every subsequent test.
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	checkPreviewArchiveCanonicalModes(t)
}

func TestPreviewArchiveRejectsPublicStagingRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractPreviewArchive(previewArchiveBytes(t), root); err == nil {
		t.Fatal("public outer build directory was accepted")
	}
	assertPreviewArchiveMode(t, root, 0755)
}
