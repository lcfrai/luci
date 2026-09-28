package deploywatch

import "reflect"

// publish is step 9: compare each project's new snapshot with the last
// published one, ignoring FetchedAt; if anything else differs, bump that
// project's Version and the global cursor once.
//
// FetchedAt is ignored on purpose: a poll that read the same facts again is not
// a change the desktop needs to fetch for, and a cursor that moved every three
// seconds would make the cursor worthless.
func (w *Watcher) publish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range w.order {
		st := w.states[id]
		if st == nil {
			continue
		}
		if !st.published && st.resolvedAt.IsZero() {
			continue
		}
		next := st.snap
		next.FetchedAt = st.lastPublished.FetchedAt
		next.Version = st.lastPublished.Version
		if st.published && reflect.DeepEqual(next, st.lastPublished) {
			continue
		}
		st.snap.Version++
		st.lastPublished = st.snap.clone()
		st.published = true
		w.cursor.Add(1)
	}
}
