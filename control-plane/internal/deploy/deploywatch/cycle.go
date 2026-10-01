package deploywatch

import (
	"context"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
)

// Run blocks until ctx is done. ONE goroutine, one ticker at ActiveInterval;
// each project is polled when it is due. Each cycle runs under one
// context.WithTimeout(ctx, CycleDeadline).
func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(w.cfg.ActiveInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Cycle(ctx)
		}
	}
}

// Cycle is one tick's work. It is exported so the tests drive the loop with
// their own clock rather than sleeping: a test that waits for a real ticker
// pays real seconds for every cadence it pins.
func (w *Watcher) Cycle(ctx context.Context) {
	cycleCtx, cancel := context.WithTimeout(ctx, w.cfg.CycleDeadline)
	defer cancel()
	b := budget{deadline: w.cfg.Now().Add(w.cfg.CycleDeadline), now: w.cfg.Now}

	if !w.gate(cycleCtx) {
		return
	}
	// The local projects come first, before the rotation is taken: a project
	// added since the last cycle is in this one, and one that has gone is in
	// none. A local scan failure changes nothing about Panacea, so the cycle
	// goes on with the projects it already knew.
	_ = w.discover(cycleCtx)
	due := w.rotation()
	hubFirst := w.takeHubFirst()
	stopped := false
	if hubFirst {
		stopped = w.hubPhase(cycleCtx, b, due)
		if !stopped {
			stopped = w.panaceaPhase(cycleCtx, b, due)
		}
	} else {
		stopped = w.panaceaPhase(cycleCtx, b, due)
		if !stopped {
			stopped = w.hubPhase(cycleCtx, b, due)
		}
	}
	// A cycle that stopped runs the phase it did not finish first next time, so
	// under sustained load the two phases alternate at the front and neither
	// goes more than one cycle without the whole budget ahead of it.
	w.mu.Lock()
	w.hubFirst = stopped && !hubFirst
	w.mu.Unlock()
	w.publish()
}

// gate is step 0. It answers whether any project is polled this cycle.
func (w *Watcher) gate(ctx context.Context) bool {
	st := w.cfg.Connection.Status()
	switch st.State {
	case connection.Connected:
		return true
	case connection.AwaitingApproval, connection.TokenRejected:
		// The lens says this screen checks again on its own, so it must, and
		// without a desktop timer.
		w.mu.Lock()
		last := w.lastGate
		w.mu.Unlock()
		now := w.cfg.Now()
		if last.IsZero() || !now.Before(last.Add(w.recheckInterval())) {
			w.mu.Lock()
			w.lastGate = now
			w.mu.Unlock()
			w.cfg.Connection.Check(ctx)
		}
		return false
	default:
		return false
	}
}

// rotation is this cycle's project order: tracked runs first, then the projects
// stopped twice in a row (carried b09-2 I-1, point 4), then the rest in
// rotation from where the last cycle left off.
func (w *Watcher) rotation() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(w.order)
	if n == 0 {
		return nil
	}
	if w.startAt >= n {
		w.startAt = 0
	}
	var tracked, starved, rest []string
	for i := range w.order {
		id := w.order[(w.startAt+i)%n]
		st := w.states[id]
		switch {
		case st.active():
			tracked = append(tracked, id)
		case st.consecutiveStops >= stopsBeforeFront:
			starved = append(starved, id)
		default:
			rest = append(rest, id)
		}
	}
	return append(append(tracked, starved...), rest...)
}

func (w *Watcher) takeHubFirst() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hubFirst
}

// panaceaPhase is steps 1-5 and 8 for every due project. It reports whether the
// phase ended on a budget stop.
func (w *Watcher) panaceaPhase(ctx context.Context, b budget, order []string) bool {
	if stopped := w.resolvePhase(ctx, b, order); stopped {
		return true
	}
	now := w.cfg.Now()
	for i, id := range order {
		w.mu.Lock()
		st, ok := w.states[id]
		poll := ok && st.duePoll(now, w.cfg.Interval, w.cfg.ActiveInterval)
		w.mu.Unlock()
		if !poll {
			continue
		}
		outcome := w.pollProject(ctx, b, st)
		switch outcome {
		case pollStopped:
			w.markStopped(order[i:])
			w.setStartAt(id)
			return true
		case pollOutage:
			// The failure rule: Panacea is down, so no other project is asked.
			w.staleRest(order[i+1:])
			w.setStartAt(id)
			return false
		}
		w.mu.Lock()
		st.consecutiveStops = 0
		w.mu.Unlock()
		w.setStartAtAfter(id)
	}
	return false
}

// discover adds a state for every local project and drops the ones that have
// gone. A local scan failure changes nothing: it is not a fact about Panacea.
func (w *Watcher) discover(ctx context.Context) error {
	projects, err := w.cfg.Projects.Projects(ctx)
	if err != nil {
		w.cfg.Log.Warn("deploywatch: local project scan failed", "err", err)
		return err
	}
	live := make(map[string]bool, len(projects))
	for _, p := range projects {
		live[p.ID] = true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range projects {
		w.stateFor(p.ID)
	}
	kept := w.order[:0]
	for _, id := range w.order {
		if live[id] {
			kept = append(kept, id)
			continue
		}
		delete(w.states, id)
	}
	w.order = kept
	return nil
}

// resolvePhase is step 1: ONE ResolveMany for every due project, however many
// there are. It begins only with ReadTimeout + HubReadTimeout of the budget
// left, because ResolveMany is a Hub call as well as a Panacea one (carried
// b09-2 I-1, trigger 1).
func (w *Watcher) resolvePhase(ctx context.Context, b budget, order []string) bool {
	now := w.cfg.Now()
	var due []string
	w.mu.Lock()
	for _, id := range order {
		if st := w.states[id]; st != nil && st.dueResolve(now, w.recheckInterval()) {
			due = append(due, id)
		}
	}
	w.mu.Unlock()
	if len(due) == 0 {
		return false
	}
	// ResolveMany is one Panacea read and, when a Hub is configured, one Hub
	// lookup per repository key. With no Hub it is a Panacea read alone, and
	// asking for the Hub's share of the budget as well would stop cycles that
	// had the room for the work they were actually going to do.
	span := w.cfg.ReadTimeout
	if w.cfg.Releases != nil {
		span += w.cfg.HubReadTimeout
	}
	if !b.canRead(span) {
		w.logStop("binding", due, b)
		w.markStopped(order)
		return true
	}
	rctx, cancel := context.WithTimeout(ctx, span)
	defer cancel()
	bound, err := w.cfg.Bindings.ResolveMany(rctx, due)
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range due {
		st := w.states[id]
		if st == nil {
			continue
		}
		bnd, ok := bound[id]
		if !ok {
			continue
		}
		w.applyBinding(st, bnd, now)
	}
	if err != nil {
		w.cfg.Log.Warn("deploywatch: binding resolve failed", "err", err)
	}
	return false
}

// applyBinding is step 1's reset rule: a binding that names a different Panacea
// project drops every reading the old one produced. The caller holds w.mu.
func (w *Watcher) applyBinding(st *state, bnd binding.Binding, now time.Time) {
	prev := st.snap.Binding
	st.resolvedAt, st.reResolve = now, false
	switch {
	case prev.Panacea != bnd.Panacea:
		st.resetPanacea()
	case decidedUnbound(bnd.State):
		// The target itself says this repository has no project now, so the old
		// project's rows are DROPPED, not kept stale: a lens showing another
		// project's deployments under this binding is worse than an empty one.
		st.resetPanacea()
	case prev.HubProjectID != bnd.HubProjectID:
		st.resetHub()
	}
	st.snap.Binding = bnd
}

// decidedUnbound is a state the target ANSWERED, as against Unavailable and
// NotConnected, which are "we could not ask" and keep the last reading.
func decidedUnbound(s binding.State) bool {
	switch s {
	case binding.NoOrigin, binding.UnknownHost, binding.NoMatch, binding.Ambiguous, binding.NoRepoInfo:
		return true
	}
	return false
}

func (w *Watcher) setStartAt(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, o := range w.order {
		if o == id {
			w.startAt = i
			return
		}
	}
}

func (w *Watcher) setStartAtAfter(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, o := range w.order {
		if o == id {
			w.startAt = (i + 1) % len(w.order)
			return
		}
	}
}

// markStopped counts a budget stop against every project this cycle did not
// reach, so a project stopped twice in a row goes to the front next time.
func (w *Watcher) markStopped(ids []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range ids {
		if st := w.states[id]; st != nil {
			st.consecutiveStops++
		}
	}
}

// staleRest is the failure rule's other half: every project not reached keeps
// its last data and says so.
func (w *Watcher) staleRest(ids []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range ids {
		st := w.states[id]
		if st == nil || st.snap.Binding.State != binding.Bound {
			continue
		}
		st.snap.Stale = true
		if st.snap.LastError == nil {
			st.snap.LastError = &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments"}
		}
	}
}

func (w *Watcher) logStop(phase string, notReached []string, b budget) {
	w.cfg.Log.Info("deploywatch: budget stop",
		"phase", phase, "notReached", notReached, "budgetLeft", b.left().String())
}
