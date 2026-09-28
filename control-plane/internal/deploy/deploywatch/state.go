package deploywatch

import (
	"maps"
	"slices"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// state is one local project's bookkeeping. The Snapshot is what the view
// reads; everything else here is why the next poll does what it does.
type state struct {
	snap      Snapshot
	published bool // a cycle has reached it at least once
	// lastPublished is what step 9 compares against, so "did anything change"
	// is asked of the reading the desktop has, not of the one before the last
	// write.
	lastPublished Snapshot

	tracked  map[string]bool // run names Track was called with
	lastPoll time.Time       // the last poll that ran to the end
	nudged   bool            // poll in the next cycle, whatever the cadence says

	resolvedAt time.Time // the last ResolveMany this project was in
	reResolve  bool      // step 2, HubEvents or Choose marked it

	// wantedAt is branch -> when WantBranch last asked for it (the LRU key);
	// branchReadAt is branch -> when EnvironmentFor last answered for it,
	// whether with a name or a refusal (the recheck key).
	wantedAt     map[string]time.Time
	branchReadAt map[string]time.Time

	// scansDone is deployment id -> the build the successful read answered for;
	// scansTried is the same for a FAILED attempt, with its time, so a failure
	// is retried at the history interval and not in every poll (carried b03-2
	// m-2).
	scansDone  map[string]deploy.BuildState
	scansTried map[string]scanAttempt

	releasesDue  bool // a release.* event, or a change, asked for a re-read
	ownerProbeAt time.Time
	// deploymentsDue is step 4's rule (c): a tracked run became terminal in the
	// last poll, so the history has a new row to carry.
	deploymentsDue bool

	// consecutiveStops counts cycles in which this project was due and was not
	// polled to the end.
	consecutiveStops int
}

type scanAttempt struct {
	build deploy.BuildState
	at    time.Time
}

func newState(projectID string) *state {
	return &state{
		snap:         Snapshot{ProjectID: projectID, OwnerControls: "unknown"},
		tracked:      map[string]bool{},
		wantedAt:     map[string]time.Time{},
		branchReadAt: map[string]time.Time{},
		scansDone:    map[string]deploy.BuildState{},
		scansTried:   map[string]scanAttempt{},
	}
}

// active reports whether this project is on the fast cadence: it has a tracked
// run whose build is not terminal.
func (s *state) active() bool { return len(s.tracked) > 0 }

// duePoll reports whether this project's steps 2-8 run in a cycle at now.
func (s *state) duePoll(now time.Time, interval, activeInterval time.Duration) bool {
	if s.nudged || s.lastPoll.IsZero() {
		return true
	}
	every := interval
	if s.active() {
		every = activeInterval
	}
	return !now.Before(s.lastPoll.Add(every))
}

// dueResolve reports whether this project is in the cycle's ResolveMany. An
// unbound binding is NOT due in every cycle: it is due at its recheck, or at
// once after a mark (carried b09-2 M-A).
func (s *state) dueResolve(now time.Time, recheck time.Duration) bool {
	return s.resolvedAt.IsZero() || s.reResolve || !now.Before(s.resolvedAt.Add(recheck))
}

// resetPanacea drops every reading that belonged to another Panacea project, so
// one project's rows can never appear under another's binding (review M13).
func (s *state) resetPanacea() {
	s.snap.Environments = nil
	s.snap.BranchEnvs = nil
	s.snap.BranchEnvErrs = nil
	s.snap.Latest = nil
	s.snap.Deployments = nil
	s.snap.DeploymentsAt = time.Time{}
	s.snap.Runs = nil
	s.snap.Scans = nil
	s.snap.OwnerControls = "unknown"
	s.branchReadAt = map[string]time.Time{}
	s.scansDone = map[string]deploy.BuildState{}
	s.scansTried = map[string]scanAttempt{}
	s.ownerProbeAt = time.Time{}
	s.resetHub()
}

// resetHub drops every Hub reading, which is everything the binding's Hub
// project was the key to.
func (s *state) resetHub() {
	s.snap.Releases = nil
	s.snap.ReleasesMore = false
	s.snap.ReleasesAt = time.Time{}
	s.snap.PinsByRun = nil
	s.releasesDue = true
}

// forget drops the least recently wanted branch when the cap is passed. It
// leaves BOTH maps, because a branch is in one or the other and the cap is on
// the pair (carried b02-2 M-5).
func (s *state) forget() {
	for len(s.wantedAt) > maxWantedBranches {
		oldest, at := "", time.Time{}
		for b, t := range s.wantedAt {
			if at.IsZero() || t.Before(at) {
				oldest, at = b, t
			}
		}
		delete(s.wantedAt, oldest)
		delete(s.branchReadAt, oldest)
		delete(s.snap.BranchEnvs, oldest)
		delete(s.snap.BranchEnvErrs, oldest)
	}
}

func (s Snapshot) clone() Snapshot {
	out := s
	out.Environments = slices.Clone(s.Environments)
	out.Deployments = slices.Clone(s.Deployments)
	out.Releases = slices.Clone(s.Releases)
	out.BranchEnvs = maps.Clone(s.BranchEnvs)
	out.BranchEnvErrs = maps.Clone(s.BranchEnvErrs)
	out.Runs = maps.Clone(s.Runs)
	out.Scans = maps.Clone(s.Scans)
	out.PinsByRun = maps.Clone(s.PinsByRun)
	if s.Latest != nil {
		d := *s.Latest
		out.Latest = &d
	}
	if s.LastError != nil {
		e := *s.LastError
		out.LastError = &e
	}
	return out
}
