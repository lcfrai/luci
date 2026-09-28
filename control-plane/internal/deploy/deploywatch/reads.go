package deploywatch

import (
	"context"
	"slices"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// readBranches is step 2's second half: EnvironmentFor for each wanted branch
// not yet answered for, and for every wanted branch at its recheck - a changed
// production-branch label remaps main. One request per new branch, not per poll.
func (w *Watcher) readBranches(ctx context.Context, b budget, st *state, bnd binding.Binding, d *draft) pollOutcome {
	w.mu.Lock()
	now := w.cfg.Now()
	var want []string
	for br := range st.wantedAt {
		_, haveName := st.snap.BranchEnvs[br]
		_, haveErr := st.snap.BranchEnvErrs[br]
		read := st.branchReadAt[br]
		if (!haveName && !haveErr) || read.IsZero() || !now.Before(read.Add(w.recheckInterval())) {
			want = append(want, br)
		}
	}
	w.mu.Unlock()
	slices.Sort(want)
	for _, br := range want {
		if !b.canRead(w.cfg.ReadTimeout) {
			return w.stop("environmentFor", st, b)
		}
		name, err := w.cfg.Deployer.EnvironmentFor(ctx, bnd.Panacea, br)
		if err != nil {
			if w.cycleEnding(err) {
				return pollOutage
			}
			// The refusal is about this ONE name: it does not stale the
			// snapshot and does not end the cycle, and it is retried at the
			// branch's recheck, not in every poll.
			d.branchErr[br] = asDeployError(err)
			d.branchRead = append(d.branchRead, br)
			continue
		}
		d.branchEnv[br] = name
		d.branchRead = append(d.branchRead, br)
	}
	return pollDone
}

func asDeployError(err error) *deploy.Error {
	if de, ok := err.(*deploy.Error); ok {
		return de
	}
	return &deploy.Error{Kind: deploy.KindOf(err), Message: err.Error()}
}

// readDeployments is step 4. The list is unpaginated and runs several
// enrichment passes on the target, so it is never read every poll.
func (w *Watcher) readDeployments(ctx context.Context, b budget, st *state, bnd binding.Binding, d *draft) pollOutcome {
	w.mu.Lock()
	now := w.cfg.Now()
	due := st.snap.DeploymentsAt.IsZero() ||
		st.deploymentsDue ||
		latestMoved(st.snap.Latest, d.latest) ||
		environmentsMoved(st.snap.Environments, d.envs) ||
		!now.Before(st.snap.DeploymentsAt.Add(w.cfg.HistoryInterval))
	w.mu.Unlock()
	if !due {
		return pollDone
	}
	if !b.canRead(w.cfg.ReadTimeout) {
		return w.stop("deployments", st, b)
	}
	list, err := w.cfg.Deployer.Deployments(ctx, bnd.Panacea)
	if err != nil {
		if w.cycleEnding(err) {
			return pollOutage
		}
		w.refused(st, err)
		return pollDone
	}
	d.deployments, d.deploysRead = list, true
	return pollDone
}

// latestMoved is (a): the newest deployment's id or build changed.
func latestMoved(old, next *deploy.Deployment) bool {
	switch {
	case old == nil && next == nil:
		return false
	case old == nil || next == nil:
		return true
	}
	return old.ID != next.ID || old.Build != next.Build
}

// environmentsMoved is (b): names, statuses or replica pairs changed.
func environmentsMoved(old, next []deploy.Environment) bool {
	if len(old) != len(next) {
		return true
	}
	for i := range old {
		a, c := old[i], next[i]
		if a.Name != c.Name || a.Status != c.Status || (a.Replicas == nil) != (c.Replicas == nil) {
			return true
		}
		if a.Replicas != nil && *a.Replicas != *c.Replicas {
			return true
		}
	}
	return false
}

// readRuns is step 5's first half: Run for each tracked run whose build is not
// terminal. Terminal untracks it, and asks step 4 for the history next poll.
func (w *Watcher) readRuns(ctx context.Context, b budget, st *state, bnd binding.Binding, d *draft) pollOutcome {
	w.mu.Lock()
	runs := make([]string, 0, len(st.tracked))
	for run := range st.tracked {
		runs = append(runs, run)
	}
	w.mu.Unlock()
	slices.Sort(runs)
	for _, run := range runs {
		if !b.canRead(w.cfg.ReadTimeout) {
			return w.stop("run", st, b)
		}
		p, err := w.cfg.Deployer.Run(ctx, bnd.Panacea, run)
		if err != nil {
			if w.cycleEnding(err) {
				return pollOutage
			}
			// A run the target will not answer for is not a fact about the
			// project: it is untracked so the fast cadence ends.
			d.untrack = append(d.untrack, run)
			continue
		}
		d.runs[run] = p
		if p.Build.Terminal() {
			d.untrack = append(d.untrack, run)
			d.runTerminated = true
		}
	}
	return pollDone
}

// readScans is step 5's second half. The groups are by the RAW Environment
// string - the very word Scans is called with - so that "" is a real group and
// the guard below is reached rather than defensive (carried b03-2 m-3).
func (w *Watcher) readScans(ctx context.Context, b budget, st *state, bnd binding.Binding, d *draft) pollOutcome {
	w.mu.Lock()
	list := st.snap.Deployments
	if d.deploysRead {
		list = d.deployments
	}
	now := w.cfg.Now()
	type want struct {
		id, env string
		build   deploy.BuildState
	}
	seen := map[string]bool{}
	var wants []want
	for _, dep := range list { // newest first
		if seen[dep.Environment] {
			continue
		}
		seen[dep.Environment] = true
		if dep.Environment == "" {
			// A legacy row with no environment. Panacea reads a missing env as
			// production, so asking would show production's verdict as this
			// release's: the fact reads "not read" instead.
			continue
		}
		if st.scansDone[dep.ID] == dep.Build {
			continue
		}
		if a, tried := st.scansTried[dep.ID]; tried && a.build == dep.Build &&
			now.Before(a.at.Add(w.cfg.HistoryInterval)) {
			// The attempt failed and nothing has changed since: retry at the
			// history interval, not in every poll (carried b03-2 m-2).
			continue
		}
		wants = append(wants, want{id: dep.ID, env: dep.Environment, build: dep.Build})
	}
	w.mu.Unlock()
	for _, wa := range wants {
		if !b.canRead(w.cfg.ReadTimeout) {
			return w.stop("scans", st, b)
		}
		r, err := w.cfg.Deployer.Scans(ctx, bnd.Panacea, wa.env, wa.id)
		if err != nil {
			if w.cycleEnding(err) {
				return pollOutage
			}
			d.scanFail[wa.id] = scanAttempt{build: wa.build, at: now}
			continue
		}
		d.scans[wa.id], d.scansDone[wa.id] = r, wa.build
	}
	return pollDone
}

// probeOwner is step 8 (7P-9): does this credential control the project? It is
// asked of the target, never inferred from a group membership. Its failures are
// never the cycle's: the probe is a question about permission, and "unknown" is
// a true answer to it whatever went wrong.
func (w *Watcher) probeOwner(ctx context.Context, b budget, st *state, bnd binding.Binding, d *draft) pollOutcome {
	if w.cfg.Governor == nil {
		return pollDone
	}
	w.mu.Lock()
	now, at := w.cfg.Now(), st.ownerProbeAt
	w.mu.Unlock()
	if !at.IsZero() && now.Before(at.Add(ownerProbeInterval)) {
		return pollDone
	}
	branch := primaryBranch(d.envs)
	if branch == "" {
		d.owner, d.ownerProbed = "unknown", true
		return pollDone
	}
	if !b.canRead(w.cfg.ReadTimeout) {
		return w.stop("autoDeploy", st, b)
	}
	_, err := w.cfg.Governor.AutoDeploy(ctx, bnd.Panacea, branch)
	switch {
	case err == nil, deploy.KindOf(err) == deploy.KindConflict:
		// A conflict is the target saying "that branch has no environment yet",
		// which only a caller allowed to ask could be told.
		d.owner = "yes"
	case deploy.KindOf(err) == deploy.KindForbidden:
		d.owner = "no"
	default:
		d.owner = "unknown"
	}
	d.ownerProbed = true
	return pollDone
}

func primaryBranch(envs []deploy.Environment) string {
	for _, e := range envs {
		if e.Primary {
			return e.Branch
		}
	}
	return ""
}
