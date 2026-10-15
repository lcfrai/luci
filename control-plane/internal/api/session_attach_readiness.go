package api

import (
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// sessionAttachReady projects only a known allocation's recorded live endpoint.
// A starting endpoint remains attachable before the health watch completes.
// An absent endpoint is unknown here: after controller loss, Backend.Get can
// still recover a live allocation whose endpoint write never completed.
func sessionAttachReady(s registry.Session, generation string) *bool {
	if generation == "" || s.Endpoint == "" || !sessionAttachLive(s) {
		return nil
	}
	ready := true
	return &ready
}

func sessionAttachLive(s registry.Session) bool {
	return s.Status == registry.StatusStarting || s.Status == registry.StatusRunning || s.Status == registry.StatusIdle
}

type sessionPreparation struct{ generation string }

// This transient observation belongs to one API. It is not an execution lock,
// persisted state, resource reservation, backend health result or authority.
type sessionPreparationSet struct {
	mu      sync.Mutex
	entries map[string]*sessionPreparation
}

// beginSessionPreparation is armed before publishing the allocation's record.
// Only its matching allocation may be cleared, including when an older defer
// runs after a replacement. The cleanup also handles every failed/canceled path.
func (a *API) beginSessionPreparation(s registry.Session) func() {
	generation := sessionExecutionGeneration(s)
	if generation == "" {
		return func() {}
	}
	entry := &sessionPreparation{generation: generation}
	a.sessionPreparations.mu.Lock()
	if a.sessionPreparations.entries == nil {
		a.sessionPreparations.entries = make(map[string]*sessionPreparation)
	}
	a.sessionPreparations.entries[s.ID] = entry
	a.sessionPreparations.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			a.sessionPreparations.mu.Lock()
			defer a.sessionPreparations.mu.Unlock()
			if current := a.sessionPreparations.entries[s.ID]; current == entry && current.generation == generation {
				delete(a.sessionPreparations.entries, s.ID)
			}
		})
	}
}

// sessionJSON adds a negative readiness hint only while this API owns the exact
// allocation preparation. A reopened API or untracked allocation omits it and
// preserves the existing backend-based attach recovery and authorization checks.
func (a *API) sessionJSON(s registry.Session, base string) sessionJSON {
	view := toJSON(s, base)
	if view.ImplementationOutcome != nil {
		if turn := observeNativeOpenCodeTurn(s); turn != nil {
			view.ImplementationOutcome.NativeTurnID = turn.TurnID
			view.ImplementationOutcome.NativeToolClass = turn.ToolClass
		}
	}
	if view.ExecutionGeneration == "" || view.AttachReady != nil || !sessionAttachLive(s) {
		return view
	}
	a.sessionPreparations.mu.Lock()
	entry := a.sessionPreparations.entries[s.ID]
	preparing := entry != nil && entry.generation == view.ExecutionGeneration
	a.sessionPreparations.mu.Unlock()
	if preparing {
		ready := false
		view.AttachReady = &ready
	}
	return view
}
