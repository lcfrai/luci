package product

import (
	"context"
	"testing"
	"time"
)

func TestCloudTaskObserverDrainsBeforeOwnedRuntimeShutdown(t *testing.T) {
	s, _ := journeyFixture(t)
	entered := make(chan struct{})
	exited := make(chan struct{})
	s.SetCloudTaskObserver(func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("owned cloud observer did not start")
	}
	stopped := make(chan struct{})
	go func() { s.StopCloudTaskObserver(); close(stopped) }()
	select {
	case <-stopped:
		select {
		case <-exited:
		default:
			t.Fatal("observer stop returned before the remote operation drained")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer did not honor shutdown cancellation")
	}
}
