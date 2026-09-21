package ring_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ring"
)

func TestSnapshotOfEmptyRingIsEmpty(t *testing.T) {
	r := ring.New(16)
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("Snapshot() = %q, want empty", got)
	}
	if r.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", r.Len())
	}
}

func TestWritesUnderCapacityAreReturnedInOrder(t *testing.T) {
	r := ring.New(16)
	r.Write([]byte("abc"))
	r.Write([]byte("def"))
	if got := string(r.Snapshot()); got != "abcdef" {
		t.Fatalf("Snapshot() = %q, want %q", got, "abcdef")
	}
	if r.Len() != 6 {
		t.Fatalf("Len() = %d, want 6", r.Len())
	}
}

func TestOldestBytesAreDiscardedOnOverflow(t *testing.T) {
	r := ring.New(4)
	r.Write([]byte("abcdef"))
	if got := string(r.Snapshot()); got != "cdef" {
		t.Fatalf("Snapshot() = %q, want %q", got, "cdef")
	}
	if r.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", r.Len())
	}
}

func TestWriteLargerThanCapacityKeepsTheTail(t *testing.T) {
	r := ring.New(4)
	r.Write([]byte("0123456789"))
	if got := string(r.Snapshot()); got != "6789" {
		t.Fatalf("Snapshot() = %q, want %q", got, "6789")
	}
}

func TestWrapAroundPreservesOrder(t *testing.T) {
	r := ring.New(5)
	r.Write([]byte("abc"))
	r.Write([]byte("de"))
	r.Write([]byte("fg"))
	if got := string(r.Snapshot()); got != "cdefg" {
		t.Fatalf("Snapshot() = %q, want %q", got, "cdefg")
	}
}

func TestWriteReportsFullLengthConsumed(t *testing.T) {
	r := ring.New(2)
	n, err := r.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write n = %d, want 6 (io.Writer must report all bytes consumed)", n)
	}
}

func TestSnapshotDoesNotAliasInternalStorage(t *testing.T) {
	r := ring.New(8)
	r.Write([]byte("abcd"))
	snap := r.Snapshot()
	snap[0] = 'X'
	if got := string(r.Snapshot()); got != "abcd" {
		t.Fatalf("Snapshot aliased internal storage: second call = %q", got)
	}
}

func TestBinarySafe(t *testing.T) {
	r := ring.New(8)
	payload := []byte{0x00, 0x1b, 0x5b, 0xff}
	r.Write(payload)
	if !bytes.Equal(r.Snapshot(), payload) {
		t.Fatalf("Snapshot() = %v, want %v", r.Snapshot(), payload)
	}
}

func TestConcurrentWritesDoNotRace(t *testing.T) {
	r := ring.New(1024)
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				r.Write([]byte(strings.Repeat("x", 7)))
				_ = r.Snapshot()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
