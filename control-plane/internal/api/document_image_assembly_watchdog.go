package api

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblyForegroundUnknownBudget = 5 * time.Second

// One exact-allocation emergency lane. It never calls authority/ledger/headroom/
// capture/publication hooks, and cannot allocate/start/fence a replacement.
// Its own deadline is the original whole deadline, not another serial budget.
type assemblyStopWatchdog struct {
	done      chan struct{}
	disarm    chan struct{}
	once      sync.Once
	progress  atomic.Pointer[time.Time]
	mu        sync.Mutex
	triggered bool
	confirmed bool
	reason    string
}

func startAssemblyStopWatchdog(compute context.Context, cancelCompute context.CancelFunc, executor backend.DocumentImageAssemblyExecutor, identity backend.DocumentImageAssemblyIdentity, ref string, whole time.Time) *assemblyStopWatchdog {
	w := &assemblyStopWatchdog{done: make(chan struct{}), disarm: make(chan struct{})}
	w.beat()
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		reason := ""
		for reason == "" {
			select {
			case <-w.disarm:
				return
			case <-compute.Done():
				reason = "compute_interrupted"
			case <-ticker.C:
				at := w.progress.Load()
				if at == nil || time.Since(*at) >= assemblyForegroundUnknownBudget {
					reason = "foreground_hook_unsettled"
				}
			}
		}
		w.mu.Lock()
		w.triggered = true
		w.reason = reason
		w.mu.Unlock()
		// Cancel any in-flight Start/foreground SDK request before retiring.
		// Receipt, headroom and capture hooks are deliberately absent here.
		cancelCompute()
		cleanup, cancel := context.WithDeadline(context.Background(), whole)
		defer cancel()
		err := executor.RetireDocumentImageAssembly(cleanup, identity, ref)
		w.mu.Lock()
		w.confirmed = err == nil
		w.mu.Unlock()
	}()
	return w
}
func (w *assemblyStopWatchdog) beat() {
	if w != nil {
		at := time.Now()
		w.progress.Store(&at)
	}
}
func (w *assemblyStopWatchdog) stop() {
	if w != nil {
		w.once.Do(func() { close(w.disarm) })
	}
}
func (w *assemblyStopWatchdog) observe(out *documentImageAssemblyOutcome) {
	if w == nil {
		out.WatchdogSettled = true
		return
	}
	select {
	case <-w.done:
		out.WatchdogSettled = true
	default:
		out.WatchdogSettled = false
	}
	w.mu.Lock()
	out.WatchdogTriggered = w.triggered
	out.WatchdogRetirementConfirmed = w.confirmed
	reason := w.reason
	w.mu.Unlock()
	if out.ReasonCode == "" && reason != "" {
		out.ReasonCode = reason
	}
}
func (w *assemblyStopWatchdog) wait(ctx context.Context, out *documentImageAssemblyOutcome) bool {
	if w == nil {
		out.WatchdogSettled = true
		return true
	}
	select {
	case <-w.done:
	case <-ctx.Done():
	}
	w.observe(out)
	return out.WatchdogSettled
}
