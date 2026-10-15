package registry

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHomeTerminalStampSurvivesRepeatedWritesAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	row := sampleSession()
	if err := store.Create(row); err != nil {
		t.Fatal(err)
	}
	first, err := store.Update(row.ID, func(s *Session) error { s.Status = StatusExited; return nil })
	if err != nil || first.StoppedAt.IsZero() {
		t.Fatal(first, err)
	}
	time.Sleep(time.Millisecond)
	second, err := store.Update(row.ID, func(s *Session) error { s.ExitReason = "completed"; return nil })
	if err != nil || !second.StoppedAt.Equal(first.StoppedAt) {
		t.Fatal("terminal identity changed on metadata write", second, err)
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := reopened.Get(row.ID)
	if err != nil || !read.StoppedAt.Equal(first.StoppedAt) {
		t.Fatal("terminal identity changed after restart", read, err)
	}
	next, err := reopened.Update(row.ID, func(s *Session) error { s.Status = StatusRunning; return nil })
	if err != nil || !next.StoppedAt.IsZero() {
		t.Fatal("old stop followed a new run", next, err)
	}
}
