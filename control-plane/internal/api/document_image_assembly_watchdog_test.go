package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestAssemblyWatchdogRetiresBeforeBlockedForegroundHooksReturn(t *testing.T) {
	for _, hook := range []string{"running_transition", "running_headroom", "running_headroom_unknown", "unknown_retirement"} {
		t.Run(hook, func(t *testing.T) {
			req, h, f, _ := assemblyRuntimeFixture(t)
			if hook == "unknown_retirement" {
				f.retireErr = errors.New("removal unknown")
			}
			entered, release, unlocked, finished := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
			var releaseOnce sync.Once
			releaseHook := func() { releaseOnce.Do(func() { close(release) }) }
			defer releaseHook()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			block := func() { close(entered); <-release }
			h.Lock = func(context.Context, backend.DocumentImageAssemblyIdentity) (func(), error) {
				return func() { close(unlocked) }, nil
			}
			h.Finish = func(_ context.Context, _ backend.DocumentImageAssemblyIdentity, o documentImageAssemblyOutcome) error {
				if !o.CleanupConfirmed || !o.ReaderSettled || !o.WatchdogSettled || o.ResourcesHeld {
					return errors.New("unsafe finish")
				}
				finished <- struct{}{}
				return nil
			}
			if strings.HasPrefix(hook, "running_headroom") {
				h.Headroom = func(_ context.Context, _ string, bound uint64) error {
					if bound == 4<<30 {
						block()
					}
					return nil
				}
			} else {
				h.Transition = func(_ context.Context, _ backend.DocumentImageAssemblyIdentity, _, next, _ string) error {
					if next == "running" {
						block()
					}
					return nil
				}
			}
			results := make(chan documentImageAssemblyOutcome, 1)
			go func() { o, _ := runDocumentImageAssembly(ctx, f, h, req); results <- o }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("hook not reached")
			}
			stopWait := time.Second
			if hook == "running_headroom_unknown" {
				stopWait = assemblyForegroundUnknownBudget + 2*time.Second
			} else {
				cancel()
			}
			select {
			case <-f.retired:
			case <-time.After(stopWait):
				t.Fatal("blocked hook defeated independent exact stop")
			}
			identity := req.Spec.Identity
			identity.EngineID, identity.InstanceID = "fixture-engine", "fixture-instance"
			if f.retiredIdentity != identity || f.retiredRef != strings.Repeat("b", 64) {
				t.Fatal("retirement changed allocation/engine/scope", f.retiredIdentity, f.retiredRef)
			}
			select {
			case <-unlocked:
				t.Fatal("blocked foreground released lock")
			default:
			}
			select {
			case <-finished:
				t.Fatal("blocked foreground released lease")
			default:
			}
			releaseHook()
			var out documentImageAssemblyOutcome
			select {
			case out = <-results:
			case <-time.After(time.Second):
				t.Fatal("foreground failed to settle")
			}
			if !out.WatchdogTriggered || !out.WatchdogSettled || out.Capture != (backend.DocumentImageAssemblyCapture{}) {
				t.Fatal("emergency fabricated assembly/omitted watchdog proof", out)
			}
			if hook == "unknown_retirement" {
				if !out.ResourcesHeld || out.CleanupConfirmed || out.WatchdogRetirementConfirmed {
					t.Fatal("unknown stop released capacity", out)
				}
				select {
				case <-finished:
					t.Fatal("unknown stop finished lease")
				default:
				}
				select {
				case <-unlocked:
					t.Fatal("unknown retirement released lock")
				default:
				}
			} else if out.ResourcesHeld || !out.CleanupConfirmed || !out.WatchdogRetirementConfirmed {
				t.Fatal("positive settlement missing", out)
			}
		})
	}
}

func TestAssemblyWatchdogUnknownForegroundBrakesWithoutCallerCancellation(t *testing.T) {
	req, _, f, _ := assemblyRuntimeFixture(t)
	prepared, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := startAssemblyStopWatchdog(ctx, cancel, f, prepared.Spec.Identity, strings.Repeat("b", 64), req.WholeDeadline)
	// Advance only the inert progress observation; no real socket/process or
	// arbitrary duration is substituted into the fixed production policy.
	past := time.Now().Add(-assemblyForegroundUnknownBudget - time.Second)
	w.progress.Store(&past)
	select {
	case <-f.retired:
	case <-time.After(time.Second):
		t.Fatal("unknown foreground failed to brake")
	}
	bounded, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	var out documentImageAssemblyOutcome
	if !w.wait(bounded, &out) || !out.WatchdogTriggered || !out.WatchdogRetirementConfirmed || out.ReasonCode != "foreground_hook_unsettled" || ctx.Err() == nil {
		t.Fatal("unknown hook did not settle exact cancellation/stop", out)
	}
}

func TestAssemblyWatchdogBoundedWaitCannotClaimBlockedRetirementSettled(t *testing.T) {
	req, _, f, _ := assemblyRuntimeFixture(t)
	prepared, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	defer release()
	f.retireGate = gate
	ctx, cancel := context.WithCancel(context.Background())
	w := startAssemblyStopWatchdog(ctx, cancel, f, prepared.Spec.Identity, strings.Repeat("b", 64), req.WholeDeadline)
	cancel()
	select {
	case <-f.retired:
	case <-time.After(time.Second):
		t.Fatal("stop not requested")
	}
	bounded, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	out := documentImageAssemblyOutcome{ResourcesHeld: true}
	if w.wait(bounded, &out) || out.WatchdogSettled || out.WatchdogRetirementConfirmed || !out.ResourcesHeld {
		t.Fatal("bounded wait fabricated stop acknowledgement", out)
	}
	release()
	settle, settleCancel := context.WithTimeout(context.Background(), time.Second)
	defer settleCancel()
	if !w.wait(settle, &out) || !out.WatchdogRetirementConfirmed || !out.ResourcesHeld {
		t.Fatal("actual late settlement missing or released capacity", out)
	}
}
