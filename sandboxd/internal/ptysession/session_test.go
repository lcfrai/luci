//go:build linux || darwin

package ptysession_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/lcfrai/luci/sandboxd/internal/ptysession"
)

// readUntil reads from r until want appears or the deadline passes.
func readUntil(t *testing.T, r io.Reader, want []byte, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	var acc bytes.Buffer
	buf := make([]byte, 1024)
	for time.Now().Before(deadline) {
		n, err := r.Read(buf)
		if n > 0 {
			acc.Write(buf[:n])
			if bytes.Contains(acc.Bytes(), want) {
				return
			}
		}
		if err != nil {
			break
		}
	}
	t.Fatalf("did not see %q within %s; got %q", want, d, acc.String())
}

func TestSessionEchoesCommandOutput(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("echo luci-ok\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, s, []byte("luci-ok"), 5*time.Second)
}

func TestSessionResizeIsReportedToTheProcess(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	if err := s.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := s.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readUntil(t, s, []byte("40 120"), 5*time.Second)
}

func TestSessionCloseTerminatesTheProcess(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	done := make(chan struct{})
	go func() { _ = s.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit within 5s of Close")
	}
}

func TestReadAfterCloseReturnsPromptlyWithError(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		_, rerr := s.Read(buf)
		done <- rerr
	}()
	select {
	case rerr := <-done:
		if rerr == nil {
			t.Fatal("Read after Close returned nil error; expected an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Read after Close blocked for 5s; Task 4's read goroutine would leak")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	s, err := ptysession.Start("/bin/sh", nil, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	first := s.Close()
	second := s.Close()
	if second != first {
		t.Fatalf("second Close returned %v, want the same result as the first (%v)", second, first)
	}
}
