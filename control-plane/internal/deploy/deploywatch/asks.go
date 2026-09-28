package deploywatch

import (
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// This file is everything another task asks of the watcher between cycles.
// NONE of it makes a request: each one marks and returns, so a caller on the
// Hub poller's goroutine, or in an HTTP handler, never waits for Panacea.

// Track makes a run this user started (Ship, rebuild) drive the active cadence
// until its build is terminal. Task 09 and task 10 call it right after an act.
func (w *Watcher) Track(projectID, run string) {
	if projectID == "" || run == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.stateFor(projectID)
	st.tracked[run] = true
	st.nudged = true
}

// Nudge polls a project in the next cycle, whatever its cadence says.
func (w *Watcher) Nudge(projectID string) {
	if projectID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stateFor(projectID).nudged = true
}

// Recheck marks a project's binding for re-resolve in the next cycle and polls
// it. Task 06's Choose and task 09's Ship call it, so a binding the user just
// changed is not left to the 60 s recheck (carried b09-2 M-A names the marks).
func (w *Watcher) Recheck(projectID string) {
	if projectID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.stateFor(projectID)
	st.reResolve, st.nudged = true, true
}

// WantBranch asks for a git branch's environment name to be resolved and kept
// in Snapshot.BranchEnvs. Task 08's route calls it for the lens's selected
// branch. At most 16 branches per project are kept, the least recently wanted
// dropped first, from both branch maps.
func (w *Watcher) WantBranch(projectID, branch string) {
	if projectID == "" || branch == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.stateFor(projectID)
	_, known := st.wantedAt[branch]
	st.wantedAt[branch] = w.cfg.Now()
	st.forget()
	if !known {
		st.nudged = true
	}
}

// HubEvents is the watcher's side of the Hub change poller. The poller hands it
// each page's events; it makes no request.
//
// A "release." event for a bound project's Hub project marks that project's
// releases due and Nudges it, so a teammate's rollback is on the lens within
// one read of the change feed plus one cycle, not after HistoryInterval.
//
// A "project." event marks for re-resolve every project whose binding missed
// the Hub for want of a link, and the one already bound to the event's Hub
// project: a Share turns Ship on in seconds, not at the 60 s recheck. A binding
// that missed on key_spelling is NEVER marked - no share can fix a key the Hub
// itself refuses.
func (w *Watcher) HubEvents(events []Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, e := range events {
		switch {
		case strings.HasPrefix(e.Kind, "release."):
			for _, st := range w.states {
				if e.ProjectID != "" && st.snap.Binding.HubProjectID == e.ProjectID {
					st.releasesDue, st.nudged = true, true
				}
			}
		case strings.HasPrefix(e.Kind, "project."):
			for _, st := range w.states {
				b := st.snap.Binding
				if b.HubMiss == binding.MissNotLinked || (e.ProjectID != "" && b.HubProjectID == e.ProjectID) {
					st.reResolve, st.nudged = true, true
				}
			}
		}
	}
}

// stateFor is the only place a state is created. The caller holds w.mu.
func (w *Watcher) stateFor(projectID string) *state {
	st, ok := w.states[projectID]
	if !ok {
		st = newState(projectID)
		w.states[projectID] = st
		w.order = append(w.order, projectID)
	}
	return st
}
