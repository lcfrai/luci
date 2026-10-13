package product

import (
	"context"
	"sync"
	"time"
)

type cloudTaskObserver struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// SetCloudTaskObserver gives remote output reads their own bounded turn.
// The local scheduler's three-second observation/admission turn must not be
// held by a slow VM or force every remote transfer into that deadline.
func (s *Server) SetCloudTaskObserver(observe func(context.Context) error) {
	s.StopCloudTaskObserver()
	if observe == nil {
		return
	}
	ctx, cancel := context.WithCancel(s.workspaceContext)
	done := make(chan struct{})
	b := &s.cloudTaskObserver
	b.mu.Lock()
	b.cancel, b.done = cancel, done
	b.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			bounded, stop := context.WithTimeout(ctx, 30*time.Second)
			_ = observe(bounded)
			stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// StopCloudTaskObserver drains the exact owned loop before its controller and
// guest transports close. Cancellation is propagated into every selected VM.
func (s *Server) StopCloudTaskObserver() {
	b := &s.cloudTaskObserver
	b.mu.Lock()
	cancel, done := b.cancel, b.done
	b.cancel, b.done = nil, nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
