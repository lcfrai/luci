//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPDFPilotDriverBoundedOriginalOpenRefusesLinksFIFOAndOversize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ordinary"), []byte("exact"), 0600); err != nil { t.Fatal(err) }
	if err := os.Symlink("ordinary", filepath.Join(root, "linked")); err != nil { t.Fatal(err) }
	if err := os.Link(filepath.Join(root, "ordinary"), filepath.Join(root, "hardlinked")); err != nil { t.Fatal(err) }
	if err := unix.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil { t.Fatal(err) }
	fd, err := openDirectory(root)
	if err != nil { t.Fatal(err) }
	defer unix.Close(fd)
	// The hard link raises the original's link count too. Both names must now
	// refuse; removing the extra link restores the original bounded read.
	for _, name := range []string{"ordinary", "linked", "hardlinked", "pipe"} {
		if _, err := readRegularAt(fd, name, 16); err == nil { t.Fatalf("unsafe original %q was opened", name) }
	}
	if err := os.Remove(filepath.Join(root, "hardlinked")); err != nil { t.Fatal(err) }
	data, err := readRegularAt(fd, "ordinary", 16)
	if err != nil || !bytes.Equal(data, []byte("exact")) { t.Fatal("ordinary bounded original refused", err) }
	if _, err := readRegularAt(fd, "ordinary", 4); err == nil { t.Fatal("oversized original accepted") }
}
