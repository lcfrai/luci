//go:build linux

package statelease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLeaseBlocksSecondWriterAndSurvivesHolderExit(t *testing.T) {
	dir := privateLeaseFixture(t)
	first, err := Acquire(dir, "0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Acquire(dir, "0.5.0"); err == nil {
		second.Close()
		t.Fatal("second writer obtained a live lease")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Acquire(dir, "0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Version        int
		ReleaseVersion string
	}
	if json.Unmarshal(raw, &value) != nil || value.Version != 1 || value.ReleaseVersion != "0.5.0" {
		t.Fatal("lease did not record compatible snapshot protocol and release")
	}
}

func TestLeaseRejectsLinksAndBroadPermissions(t *testing.T) {
	for _, kind := range []string{"linked-root", "linked-parent", "linked-file", "hardlinked-file", "public-root", "public-file"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "state")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			lock := filepath.Join(dir, FileName)
			switch kind {
			case "linked-root":
				link := filepath.Join(parent, "link")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				dir = link
			case "linked-parent":
				link := filepath.Join(parent, "link")
				if err := os.Symlink(parent, link); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(link, "state")
			case "linked-file":
				if err := os.Symlink(filepath.Join(parent, "target"), lock); err != nil {
					t.Fatal(err)
				}
			case "hardlinked-file":
				if err := os.WriteFile(lock, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(lock, filepath.Join(parent, "alias")); err != nil {
					t.Fatal(err)
				}
			case "public-root":
				if err := os.Chmod(dir, 0755); err != nil {
					t.Fatal(err)
				}
			case "public-file":
				if err := os.WriteFile(lock, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(lock, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if lease, err := Acquire(dir, "0.5.0"); err == nil {
				lease.Close()
				t.Fatal("unsafe path accepted")
			}
		})
	}
}

func TestLeaseRefusesInterruptedRestoreBeforeWriting(t *testing.T) {
	dir := privateLeaseFixture(t)
	if err := os.WriteFile(filepath.Join(dir, ".luci-host-restore-incomplete"), []byte("interrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if lease, err := Acquire(dir, "0.5.0"); err == nil {
		lease.Close()
		t.Fatal("server would start on a partial restoration")
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatal("lease wrote over partial restoration")
	}
}

func TestLeaseRefusesDeletedHostBeforeFreshBootstrap(t *testing.T) {
	dir := privateLeaseFixture(t)
	if err := os.WriteFile(filepath.Join(dir, ".luci-host-deleted"), []byte("erasure"), 0600); err != nil {
		t.Fatal(err)
	}
	if lease, err := Acquire(dir, "0.5.0"); err == nil {
		lease.Close()
		t.Fatal("deleted host would silently bootstrap again")
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatal("startup wrote into a retired host")
	}
}

func privateLeaseFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
