//go:build linux || darwin

package statelease

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLeaseRejectsSpecialFileWithoutBlocking(t *testing.T) {
	dir := privateLeaseFixture(t)
	if err := unix.Mkfifo(filepath.Join(dir, FileName), 0600); err != nil {
		t.Fatal(err)
	}
	if lease, err := Acquire(dir, "test"); err == nil {
		lease.Close()
		t.Fatal("FIFO accepted as a lease")
	}
}

func TestLeaseRejectsInvalidRootsBeforeWriting(t *testing.T) {
	for _, dir := range []string{"", "/", "relative/state", "/tmp/../state"} {
		if lease, err := Acquire(dir, "test"); err == nil {
			lease.Close()
			t.Fatalf("invalid root accepted: %q", dir)
		}
	}
	dir := filepath.Join(privateLeaseFixture(t), "uncreated")
	if lease, err := Acquire(dir, ""); err == nil {
		lease.Close()
		t.Fatal("empty release accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("invalid request created a state directory")
	}
}

// The offline host-recovery script opens the persistent file directly and uses
// flock. Exercise both directions without calling Acquire for the other side.
func TestLeaseCompatibleWithOfflineFlock(t *testing.T) {
	dir := privateLeaseFixture(t)
	live, err := Acquire(dir, "live-release")
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	offline, err := os.OpenFile(filepath.Join(dir, FileName), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer offline.Close()
	if err := unix.Flock(int(offline.Fd()), unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("offline recovery was not excluded by live writer: %v", err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(offline.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if writer, err := Acquire(dir, "must-not-overwrite"); !errors.Is(err, ErrInUse) {
		if writer != nil {
			writer.Close()
		}
		t.Fatalf("live writer was not excluded by offline recovery: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil || string(data) != `{"version":1,"releaseVersion":"live-release"}` {
		t.Fatal("offline-held metadata changed")
	}
}
