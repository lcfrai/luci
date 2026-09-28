package deploywatch

import (
	"context"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// targetedPins is step 6's "beyond the page". The Hub returns the 200 newest
// releases, runs and rollbacks together, so a project with more has deployments
// whose release is not in the page - and the view would call a real pin absent.
//
// A targeted read is made only while ReleasesMore, only for a deployment the
// page has no row for, only once per deployment for the binding's life, and
// only once the deployment is SETTLED: before that a still-reserved release
// would answer zero items and cache a false nil.
func (w *Watcher) targetedPins(ctx context.Context, b budget, order []string) (bool, bool) {
	for _, id := range order {
		w.mu.Lock()
		st := w.states[id]
		if st == nil || st.snap.Binding.State != binding.Bound || !st.snap.ReleasesMore {
			w.mu.Unlock()
			continue
		}
		bnd, snap := st.snap.Binding, st.snap.clone()
		w.mu.Unlock()
		if _, decided := releasesStateFor(bnd); decided {
			continue
		}
		wants := pinsWanted(snap, w.cfg.Now())
		for _, dep := range wants {
			if !b.canRead(w.cfg.HubReadTimeout) {
				w.logStop("hub-pins", []string{id}, b)
				return true, false
			}
			page, err := w.hubRead(ctx, bnd.HubProjectID, ReleaseQuery{
				PanaceaProject: string(bnd.Panacea),
				PipelineRun:    dep,
				State:          "confirmed",
				Kind:           "run",
				Page:           PageRequest{Limit: 1},
			})
			if err != nil {
				if w.cfg.ClassifyHubError != nil && w.cfg.ClassifyHubError(err) == binding.HubSignedOut {
					w.setReleasesState(st, ReleasesSignedOut)
					break
				}
				// The id stays absent, so the view says "pin not loaded", which
				// is true; the read is owed again next cycle.
				return false, true
			}
			w.rememberPin(st, dep, page)
		}
	}
	return false, false
}

// pinsWanted is the deployments owed a targeted read, newest first, at most 20
// per project per cycle: 200 history rows cost at most 200 reads once, spread
// over ten cycles, and a project with fewer than 200 releases costs none.
func pinsWanted(s Snapshot, now time.Time) []string {
	inPage := map[string]bool{}
	for _, r := range s.Releases {
		if r.PipelineRun != "" {
			inPage[r.PipelineRun] = true
		}
	}
	var want []string
	for _, d := range s.Deployments { // newest first
		if len(want) >= targetedPinReadsPerCycle {
			break
		}
		if inPage[d.ID] {
			continue
		}
		if _, read := s.PinsByRun[d.ID]; read {
			continue
		}
		if !s.SettledBy(d, now) {
			continue
		}
		want = append(want, d.ID)
	}
	return want
}

// rememberPin stores the Hub's answer: the confirmed run release, or nil for
// "none". A nil is final only because the deployment was settled when the read
// was made.
func (w *Watcher) rememberPin(st *state, deploymentID string, page ReleasePage) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if st.snap.PinsByRun == nil {
		st.snap.PinsByRun = map[string]*Release{}
	}
	for _, r := range page.Items {
		if r.Kind == "run" && r.State == "confirmed" && r.PipelineRun == deploymentID {
			row := r
			st.snap.PinsByRun[deploymentID] = &row
			return
		}
	}
	st.snap.PinsByRun[deploymentID] = nil
}

// joinPhase is step 7. Only the reserver's own watcher joins; the view never
// needs the join to show a pin, because it matches pipelineRun == Deployment.ID
// directly, so a join written late loses nothing.
func (w *Watcher) joinPhase(ctx context.Context, b budget, order []string) bool {
	if w.cfg.Me == nil {
		return false
	}
	me, err := w.meIdentity(ctx, b)
	if err != nil || me == "" {
		return false
	}
	for _, id := range order {
		w.mu.Lock()
		st := w.states[id]
		if st == nil || st.snap.Binding.State != binding.Bound {
			w.mu.Unlock()
			continue
		}
		snap := st.snap.clone()
		w.mu.Unlock()
		for _, r := range joinable(snap, me) {
			if !b.canRead(w.cfg.HubReadTimeout) {
				w.logStop("hub-join", []string{id}, b)
				return true
			}
			jctx, cancel := context.WithTimeout(ctx, w.cfg.HubReadTimeout)
			err := w.cfg.Releases.JoinDeployment(jctx, r.ID, r.PipelineRun)
			cancel()
			if err != nil {
				// A conflict is another writer having got there first, which is
				// the outcome this wanted; it is logged and not retried.
				w.cfg.Log.Info("deploywatch: deployment join not written",
					"releaseId", r.ID, "deploymentId", r.PipelineRun, "err", err)
			}
			w.markJoined(st, r.ID, r.PipelineRun)
		}
	}
	return false
}

// meIdentity reads the Hub principal once and keeps it: it changes only when
// the user signs in as somebody else, and that re-reads it.
func (w *Watcher) meIdentity(ctx context.Context, b budget) (string, error) {
	w.mu.Lock()
	me := w.meID
	w.mu.Unlock()
	if me != "" {
		return me, nil
	}
	if !b.canRead(w.cfg.HubReadTimeout) {
		return "", nil
	}
	mctx, cancel := context.WithTimeout(ctx, w.cfg.HubReadTimeout)
	defer cancel()
	id, err := w.cfg.Me.MeID(mctx)
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	w.meID = id
	w.mu.Unlock()
	return id, nil
}

// joinable is step 7's rule, exactly: mine, confirmed, a run (never a rollback:
// a rollback has no deployment of its own), with a pipelineRun, with no
// deployments[] entry, and with that deployment actually read.
func joinable(s Snapshot, meID string) []Release {
	known := map[string]bool{}
	for _, d := range s.Deployments {
		known[d.ID] = true
	}
	if s.Latest != nil {
		known[s.Latest.ID] = true
	}
	var out []Release
	for _, r := range all(s) {
		switch {
		case r.Kind != "run", r.State != "confirmed", r.ReservedByID != meID,
			r.PipelineRun == "", len(r.DeploymentIDs) > 0, !known[r.PipelineRun]:
			continue
		}
		out = append(out, r)
	}
	return out
}

// all is the page and the targeted answers together: a release beyond the page
// is as joinable as one in it.
func all(s Snapshot) []Release {
	out := append([]Release(nil), s.Releases...)
	for _, r := range s.PinsByRun {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out
}

// markJoined records the join locally so it is written once, whatever the Hub
// answered: a 409 means it is already there.
func (w *Watcher) markJoined(st *state, releaseID, deploymentID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i := range st.snap.Releases {
		if st.snap.Releases[i].ID == releaseID {
			st.snap.Releases[i].DeploymentIDs = append(st.snap.Releases[i].DeploymentIDs, deploymentID)
		}
	}
	if r, ok := st.snap.PinsByRun[deploymentID]; ok && r != nil && r.ID == releaseID {
		r.DeploymentIDs = append(r.DeploymentIDs, deploymentID)
	}
}
