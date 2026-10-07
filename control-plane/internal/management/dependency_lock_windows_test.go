//go:build windows

package management

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDependencyHostMutexSerializesIndependentInstallersAndHonorsCancellation(t *testing.T) {
	release, err := acquireDependencyHostInstallation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		second, err := acquireDependencyHostInstallation(ctx)
		if err == nil {
			second()
			result <- errors.New("second installer acquired an already-owned host mutex")
			return
		}
		result <- err
	}()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting installer did not cancel correctly: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiting installer did not honor cancellation")
	}
	release()
	release = nil
	third, err := acquireDependencyHostInstallation(context.Background())
	if err != nil {
		t.Fatalf("host lock was not released: %v", err)
	}
	third()
}
