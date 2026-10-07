//go:build windows

package platform

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestDataInitializationPromotionWaitsForConcurrentStatusRead(t *testing.T) {
	s, _ := newDataFake(t)
	install := dataInstallation{ProjectID: "owned", StackID: "stack-initialization", InitializationState: "pending"}
	if err := s.write("owned-data-installation", install); err != nil {
		t.Fatal("could not seed the initialization receipt")
	}
	status := emptyData("owned")
	status.Status = "ready"
	for _, name := range dataServiceNames {
		status.Services = append(status.Services, DataService{Name: name, Status: "running", Health: "healthy"})
	}
	if err := s.write("owned-data", status); err != nil {
		t.Fatal("could not seed the verified health receipt")
	}
	file, err := s.path("owned-data-installation")
	if err != nil {
		t.Fatal("could not locate the active receipt")
	}

	// Status readers hold s.mu while loadData reads this receipt. A native
	// Windows os.Open handle does not allow replacing its file while open.
	// Keep that real handle open to make the polling/promotion interleaving
	// deterministic instead of depending on a fast scheduler or filesystem.
	s.mu.Lock()
	reader, err := os.Open(file)
	if err != nil {
		s.mu.Unlock()
		t.Fatal("could not open the status reader")
	}
	var release sync.Once
	finishRead := func() { release.Do(func() { _ = reader.Close(); s.mu.Unlock() }) }
	defer finishRead()
	started, finished := make(chan struct{}), make(chan error, 1)
	go func() {
		close(started)
		finished <- s.markDataInitialized(install)
	}()
	<-started
	select {
	case err := <-finished:
		finishRead()
		var rename *os.LinkError
		t.Fatalf("initialization raced the held status reader: rename=%t accessDenied=%t sharingViolation=%t", errors.As(err, &rename), errors.Is(err, windows.ERROR_ACCESS_DENIED), errors.Is(err, windows.ERROR_SHARING_VIOLATION))
	case <-time.After(200 * time.Millisecond):
	}
	finishRead()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal("initialization did not complete after the status reader released its receipt")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("initialization remained blocked after the status reader finished")
	}
	var active dataInstallation
	if err := s.read("owned-data-installation", &active); err != nil || active.StackID != install.StackID || active.InitializationState != "complete" {
		t.Fatal("the healthy active installation was not marked complete")
	}
}
