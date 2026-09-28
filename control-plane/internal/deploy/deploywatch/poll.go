package deploywatch

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

type pollOutcome int

const (
	pollDone    pollOutcome = iota
	pollStopped             // the budget ran out; nothing this poll read is published
	pollOutage              // the failure rule: Panacea is down, so no other project is asked
)

// draft is one poll's reads, held apart from the snapshot until the poll runs to
// the end. A budget stop discards it whole: a project's partial reads are never
// published as current.
type draft struct {
	envs          []deploy.Environment
	branchEnv     map[string]string
	branchErr     map[string]*deploy.Error
	branchRead    []string
	latest        *deploy.Deployment
	latestRead    bool
	deployments   []deploy.Deployment
	deploysRead   bool
	runs          map[string]deploy.RunProgress
	untrack       []string
	scans         map[string]deploy.ScanReport
	scansDone     map[string]deploy.BuildState
	scanFail      map[string]scanAttempt
	owner         string
	ownerProbed   bool
	environsRead  bool
	runTerminated bool
}

func newDraft() *draft {
	return &draft{
		branchEnv: map[string]string{},
		branchErr: map[string]*deploy.Error{},
		runs:      map[string]deploy.RunProgress{},
		scans:     map[string]deploy.ScanReport{},
		scansDone: map[string]deploy.BuildState{},
		scanFail:  map[string]scanAttempt{},
	}
}

// pollProject is steps 2-5 and 8 for one project in a cycle in which it is due.
func (w *Watcher) pollProject(ctx context.Context, b budget, st *state) pollOutcome {
	w.mu.Lock()
	bnd := st.snap.Binding
	w.mu.Unlock()
	if bnd.State != binding.Bound {
		// Not bound: the snapshot carries the binding and nothing else.
		w.mu.Lock()
		st.lastPoll, st.nudged = w.cfg.Now(), false
		w.mu.Unlock()
		return pollDone
	}

	d := newDraft()
	// Step 2: Environments first, because a missing project surfaces here as
	// its own error, which is what makes task 03's "Latest 404 means never
	// deployed" safe.
	if !b.canRead(w.cfg.ReadTimeout) {
		return w.stop("environments", st, b)
	}
	envs, err := w.cfg.Deployer.Environments(ctx, bnd.Panacea)
	if err != nil {
		if w.cycleEnding(st, err) {
			return pollOutage
		}
		w.refused(st, err)
		return pollDone
	}
	d.envs, d.environsRead = envs, true

	if out := w.readBranches(ctx, b, st, bnd, d); out != pollDone {
		return out
	}
	// Step 3: Latest.
	if !b.canRead(w.cfg.ReadTimeout) {
		return w.stop("latest", st, b)
	}
	latest, ok, err := w.cfg.Deployer.Latest(ctx, bnd.Panacea)
	if err != nil {
		if w.cycleEnding(st, err) {
			return pollOutage
		}
		w.refused(st, err)
		return pollDone
	}
	d.latestRead = true
	if ok {
		d.latest = &latest
	}
	if out := w.readDeployments(ctx, b, st, bnd, d); out != pollDone {
		return out
	}
	if out := w.readRuns(ctx, b, st, bnd, d); out != pollDone {
		return out
	}
	if out := w.readScans(ctx, b, st, bnd, d); out != pollDone {
		return out
	}
	if out := w.probeOwner(ctx, b, st, bnd, d); out != pollDone {
		return out
	}
	w.commit(st, d)
	return pollDone
}

// cycleEnding reports the kinds that are about PANACEA rather than about this
// project, and so end the cycle for every project. KindUnauthorized is reported
// to the connection as well: a lens that keeps drawing "connected" while every
// read 401s is the failure this catches. The project the failure came in keeps
// its last data and says so, exactly as the ones never reached do.
func (w *Watcher) cycleEnding(st *state, err error) bool {
	switch deploy.KindOf(err) {
	case deploy.KindUnauthorized:
		w.cfg.Connection.Observe(err)
	case deploy.KindUnavailable, deploy.KindNotConnected:
	default:
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	st.snap.Stale, st.snap.LastError = true, asDeployError(err)
	return true
}

// refused is step 2's rule: a refusal of THIS binding stales this project and
// nothing else. A NEW refusal re-resolves the binding in the next cycle; a
// repeated one waits for the recheck, so a project refused for good costs the
// normal rate and not five times it.
func (w *Watcher) refused(st *state, err error) {
	var e *deploy.Error
	if de, ok := err.(*deploy.Error); ok {
		e = de
	} else {
		e = &deploy.Error{Kind: deploy.KindOf(err), Message: err.Error()}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	repeat := st.snap.LastError != nil && st.snap.LastError.Kind == e.Kind && st.snap.Stale
	st.snap.Stale, st.snap.LastError = true, e
	st.lastPoll, st.nudged = w.cfg.Now(), false
	if !repeat {
		st.reResolve, st.nudged = true, true
	}
}

// stop records a budget stop for one project. Nothing is marked Stale, no
// LastError is set and Connection.Observe is not called: the budget is this
// cockpit's, not Panacea's.
func (w *Watcher) stop(phase string, st *state, b budget) pollOutcome {
	w.logStop(phase, []string{st.snap.ProjectID}, b)
	return pollStopped
}

// commit publishes one poll that ran to the end.
func (w *Watcher) commit(st *state, d *draft) {
	now := w.cfg.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	if d.environsRead {
		st.snap.Environments = d.envs
	}
	for br, name := range d.branchEnv {
		if st.snap.BranchEnvs == nil {
			st.snap.BranchEnvs = map[string]string{}
		}
		st.snap.BranchEnvs[br] = name
		delete(st.snap.BranchEnvErrs, br)
	}
	for br, e := range d.branchErr {
		if st.snap.BranchEnvErrs == nil {
			st.snap.BranchEnvErrs = map[string]*deploy.Error{}
		}
		st.snap.BranchEnvErrs[br] = e
		delete(st.snap.BranchEnvs, br)
	}
	for _, br := range d.branchRead {
		st.branchReadAt[br] = now
	}
	if d.latestRead {
		st.snap.Latest = d.latest
	}
	if d.deploysRead {
		st.snap.Deployments, st.snap.DeploymentsAt = d.deployments, now
		st.deploymentsDue = false
		// A history re-read is also new ground for the release page: a
		// deployment the page has no row for is what step 6's targeted reads
		// are for.
		st.releasesDue = true
	}
	if d.runTerminated {
		st.deploymentsDue = true
		st.releasesDue = true
	}
	for run, p := range d.runs {
		if st.snap.Runs == nil {
			st.snap.Runs = map[string]deploy.RunProgress{}
		}
		st.snap.Runs[run] = p
	}
	for _, run := range d.untrack {
		delete(st.tracked, run)
	}
	for id, r := range d.scans {
		if st.snap.Scans == nil {
			st.snap.Scans = map[string]deploy.ScanReport{}
		}
		st.snap.Scans[id] = r
		delete(st.scansTried, id)
	}
	for id, bs := range d.scansDone {
		st.scansDone[id] = bs
	}
	for id, a := range d.scanFail {
		st.scansTried[id] = a
	}
	if d.ownerProbed {
		st.snap.OwnerControls, st.ownerProbeAt = d.owner, now
	}
	st.snap.FetchedAt, st.snap.Stale, st.snap.LastError = now, false, nil
	st.lastPoll, st.nudged = now, false
	st.published = true
	w.cfg.Connection.Observe(nil)
}
