//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedStagingRefusesChangedSymlinkAndHardlinkedInputs(t *testing.T) {
	source := t.TempDir()
	data := []byte("closed-input-data")
	sum := sha256.Sum256(data)
	item := DocumentImageAssemblyInput{Path: "file", Bytes: int64(len(data)), Digest: hex.EncodeToString(sum[:])}
	name := filepath.Join(source, item.Path)
	if os.WriteFile(name, data, 0600) != nil {
		t.Fatal("fixture file")
	}
	if copyPinned(context.Background(), source, t.TempDir(), item) != nil {
		t.Fatal("valid benign fixture refused")
	}
	if os.WriteFile(name, []byte("changed-input-data"), 0600) != nil {
		t.Fatal("fixture change")
	}
	if copyPinned(context.Background(), source, t.TempDir(), item) == nil {
		t.Fatal("changed bytes staged as pinned")
	}
	if os.WriteFile(name, data, 0600) != nil || os.Link(name, filepath.Join(source, "second-name")) != nil {
		t.Fatal("hardlink fixture")
	}
	if copyPinned(context.Background(), source, t.TempDir(), item) == nil {
		t.Fatal("hardlinked input staged")
	}
	if os.Remove(filepath.Join(source, "second-name")) != nil || os.Remove(name) != nil || os.Symlink("foreign", name) != nil {
		t.Fatal("symlink fixture")
	}
	if copyPinned(context.Background(), source, t.TempDir(), item) == nil {
		t.Fatal("symlink input staged")
	}
}
