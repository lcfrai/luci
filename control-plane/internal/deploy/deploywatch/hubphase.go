package deploywatch

import (
	"context"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// hubPhase is step 6 and step 7, for every bound project, AFTER the Panacea
// phase (or before it, after a stop). It runs whether or not the Panacea phase
// ended on the failure rule: the Hub is not Panacea.
//
// It reports whether it ended on a budget stop.
func (w *Watcher) hubPhase(ctx context.Context, b budget, order []string) bool {
	if w.cfg.Releases == nil {
		w.setNoHub(order)
		return false
	}
	w.mu.Lock()
	down := w.hubDownUntil
	w.mu.Unlock()
	now := w.cfg.Now()
	if !down.IsZero() && now.Before(down) {
		// The Hub failure rule's retry floor: while the Hub is unavailable it
		// is asked once per poll interval, not once per cycle.
		return false
	}
	stopped, unavailable := w.releasePages(ctx, b, order)
	if !stopped && !unavailable {
		stopped, unavailable = w.targetedPins(ctx, b, order)
	}
	if !stopped && !unavailable {
		stopped = w.joinPhase(ctx, b, order)
	}
	if unavailable {
		w.mu.Lock()
		w.hubDownUntil = now.Add(w.cfg.Interval)
		w.mu.Unlock()
	}
	return stopped
}

// setNoHub is the no-Hub-configured answer: one state, said once, for every
// project. It is NOT "unavailable" - nothing is down.
func (w *Watcher) setNoHub(order []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range order {
		if st := w.states[id]; st != nil {
			st.snap.ReleasesState = ReleasesNoHub
		}
	}
}

// releasePages is step 6's page read for every bound project whose releases are
// due. It returns (stopped, hubUnavailable).
func (w *Watcher) releasePages(ctx context.Context, b budget, order []string) (bool, bool) {
	for _, id := range order {
		w.mu.Lock()
		st := w.states[id]
		if st == nil || st.snap.Binding.State != binding.Bound {
			w.mu.Unlock()
			continue
		}
		bnd, due, at := st.snap.Binding, st.releasesDue, st.snap.ReleasesAt
		w.mu.Unlock()

		if state, decided := releasesStateFor(bnd); decided {
			// A sign-out, a miss and a key the Hub refuses are all ANSWERS: no
			// read is made, and neither Releases nor ReleasesAt is touched.
			w.setReleasesState(st, state)
			continue
		}
		now := w.cfg.Now()
		if !due && !at.IsZero() && now.Before(at.Add(w.cfg.HistoryInterval)) {
			continue
		}
		if !b.canRead(w.cfg.HubReadTimeout) {
			w.logStop("hub-releases", []string{id}, b)
			return true, false
		}
		page, err := w.hubRead(ctx, bnd.HubProjectID, ReleaseQuery{
			PanaceaProject: string(bnd.Panacea),
			Page:           PageRequest{Limit: 200},
		})
		if err != nil {
			if w.cfg.ClassifyHubError != nil && w.cfg.ClassifyHubError(err) == binding.HubSignedOut {
				w.setReleasesState(st, ReleasesSignedOut)
				continue
			}
			// The Hub failure rule: one unavailable Hub ends the phase for
			// every project, and the previous releases are kept.
			w.setReleasesState(st, ReleasesUnavailable)
			w.cfg.Log.Warn("deploywatch: hub releases read failed", "projectId", id, "err", err)
			return false, true
		}
		w.applyPage(st, page, w.cfg.Now())
	}
	return false, false
}

// releasesStateFor is the binding's own answer, when it has one. The sign-out
// is read FIRST, whatever HubProjectID holds: a configured, reachable Hub that
// refuses for want of a session looks exactly like an outage, and "the Hub is
// not answering" sends the user to wait instead of to sign in (carried b05-2
// N-2).
func releasesStateFor(bnd binding.Binding) (string, bool) {
	if bnd.HubMiss == binding.MissSignedOut {
		return ReleasesSignedOut, true
	}
	if bnd.HubProjectID != "" {
		return "", false
	}
	switch bnd.HubMiss {
	case binding.MissNotLinked:
		return ReleasesNoHubProject, true
	case binding.MissKeySpelling:
		return ReleasesHubKeySpelling, true
	default:
		// A configured Hub that has not yet answered for this key. Saying "not
		// a shared project" here would name a cause the Hub never gave.
		return ReleasesUnavailable, true
	}
}

// setReleasesState changes the state and nothing else: Releases and ReleasesAt
// are kept, so a sign-out or a blip leaves the pins already on the lens.
func (w *Watcher) setReleasesState(st *state, s string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	st.snap.ReleasesState = s
}

func (w *Watcher) applyPage(st *state, page ReleasePage, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	st.snap.Releases = page.Items
	st.snap.ReleasesMore = page.Next != nil
	st.snap.ReleasesAt = now
	st.snap.ReleasesState = ReleasesOK
	st.releasesDue = false
}

// hubRead is the ONE place a Hub read is bounded, so "a Hub that does not
// answer ends as that read's own timeout" is true of every call the watcher
// makes, and a dead Hub can never be reported as a budget stop or as Panacea.
func (w *Watcher) hubRead(ctx context.Context, hubProjectID string, q ReleaseQuery) (ReleasePage, error) {
	rctx, cancel := context.WithTimeout(ctx, w.cfg.HubReadTimeout)
	defer cancel()
	return w.cfg.Releases.Releases(rctx, hubProjectID, q)
}
