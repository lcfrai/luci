package discoverhub

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

func mustInherit(t *testing.T, s *Service, tg Target) Inheritable {
	t.Helper()
	inh, err := s.Inheritable(bg, tg)
	if err != nil {
		t.Fatalf("Inheritable(%+v): %v", tg, err)
	}
	return inh
}

func sectionOf(doc discover.Document, k discover.Key) (discover.Section, bool) {
	for _, s := range doc.Sections {
		if s.Key == k {
			return s, true
		}
	}
	return discover.Section{}, false
}

func TestInheritableIsTheBranchsLatestComplete(t *testing.T) {
	w := newWorld(t)
	w.save(w.b42, step{person(discover.KeyPurpose, "Upload evidence, and say why a file was refused."), discover.ChangeEdit, discover.StatusOpen},
		w.pinTo(w.b7, 6))
	inh := mustInherit(t, w.service(), w.branch())
	want := w.key(w.b42, 6)
	if inh.Key != want || inh.PinErased {
		t.Fatalf("Inheritable = %s (pinErased %v), want %s: the head, 7, is open", inh.Key, inh.PinErased, want)
	}
	if !reflect.DeepEqual(inh.Document, w.document(want)) {
		t.Errorf("Document = %+v, want %s resolved against its pin", inh.Document, want)
	}
}

func TestInheritableFallsBackToTheProject(t *testing.T) {
	w := newWorld(t)
	timeout := w.createBrief(w.project, hubapi.ScopeBranch, timeoutRef)
	w.save(timeout, b42Steps()[0], w.pinTo(w.b7, 6))
	s := w.service()
	want := w.key(w.b7, 6)
	for _, tg := range []Target{{ProjectID: localProject, BranchID: localTimeout}, {ProjectID: localProject, BranchID: localMain}} {
		inh := mustInherit(t, s, tg)
		if inh.Key != want || !reflect.DeepEqual(inh.Document, w.document(want)) {
			t.Errorf("Inheritable(%s) = %s, want the project's latest complete %s", tg.BranchID, inh.Key, want)
		}
	}
}

func TestInheritableIsNoneWithNoCompleteBrief(t *testing.T) {
	w := newBareWorld(t)
	w.project = w.createProject(projectName, teamKey)
	w.locate.addProject(localProject, projectDir, "engagement-portal")
	w.repos.set(projectDir, gh("origin", teamKey))
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	w.save(w.b7, b7Steps()[0], nil)
	w.b42 = w.createBrief(w.project, hubapi.ScopeBranch, evidenceRef)
	w.save(w.b42, b42Steps()[0], nil)
	s := w.service()
	for _, tg := range []Target{w.branch(), w.projectT()} {
		if inh := mustInherit(t, s, tg); !reflect.DeepEqual(inh, Inheritable{}) {
			t.Errorf("Inheritable(%+v) = %+v, want none: key \"\", zero document", tg, inh)
		}
	}
}

func TestInheritableUsesThePinnedProjectRevision(t *testing.T) {
	w := newWorld(t)
	w.save(w.b7, step{person(discover.KeyAudience, "Everyone who touches an engagement"), discover.ChangeEdit, discover.StatusComplete}, nil)
	inh := mustInherit(t, w.service(), w.branch())
	if inh.Key != w.key(w.b42, 6) || inh.Document.ProjectRevision != 6 {
		t.Fatalf("Inheritable = %s (project revision %d), want b-42@6 against b-7@6", inh.Key, inh.Document.ProjectRevision)
	}
	if a, ok := sectionOf(inh.Document, discover.KeyAudience); !ok || a.Body != b7Audience6 || a.Origin != discover.OriginProject {
		t.Errorf("audience = %+v, want rev 6's %q, inherited: never the project head's", a, b7Audience6)
	}
}

func TestResolveBatchesInTwoCallsAtMost(t *testing.T) {
	w := newWorld(t)
	w.save(w.b7, step{person(discover.KeyAudience, "Everyone who touches an engagement"), discover.ChangeEdit, discover.StatusComplete}, nil)
	timeout := w.createBrief(w.project, hubapi.ScopeBranch, timeoutRef)
	w.save(timeout, b42Steps()[0], w.pinTo(w.b7, 7))
	w.save(timeout, b42Steps()[1], w.pinTo(w.b7, 7))
	// Six branch revisions pinning three project revisions (b-7@5, @6, @7),
	// and six project keys: twelve keys, one Hub project.
	keys := []string{w.key(w.b42, 3), w.key(w.b42, 4), w.key(w.b42, 5), w.key(w.b42, 6), w.key(timeout, 1), w.key(timeout, 2)}
	for n := 1; n <= 6; n++ {
		keys = append(keys, w.key(w.b7, n))
	}
	for _, withEval := range []bool{false, true} {
		t.Run(fmt.Sprint("evaluate=", withEval), func(t *testing.T) {
			rec := &evalRecorder{state: provenance.StateOpen}
			var opts []func(*Config)
			if withEval {
				opts = append(opts, withEvaluate(rec.eval))
			}
			before, lists := len(w.hub.resolves()), len(w.hub.lists())
			refs, _, _ := w.service(opts...).Resolve(bg, keys)
			for i, r := range refs {
				if r.Resolution != provenance.Resolved {
					t.Errorf("refs[%d] %s = %s, want resolved", i, keys[i], r.Resolution)
				}
			}
			if n := len(w.hub.resolves()) - before; n > 2 {
				t.Errorf("Resolve calls = %d, want at most 2", n)
			}
			wantLists := 0
			if withEval {
				wantLists = 1
			}
			if n := len(w.hub.lists()) - lists; n != wantLists {
				t.Errorf("List calls = %d, want %d", n, wantLists)
			}
		})
	}
}

func TestResolveNeverSendsLocalForms(t *testing.T) {
	w := newWorld(t)
	refs, heads, states := w.service().Resolve(bg, []string{"", provenance.Declined, "b-42"})
	want := []provenance.Ref{provenance.NoneRef(""), provenance.NoneRef(provenance.Declined), provenance.InvalidRef("b-42")}
	if !reflect.DeepEqual(refs, want) {
		t.Errorf("refs = %+v, want none, none (declined), invalid", refs)
	}
	if len(w.hub.resolves()) != 0 || len(heads) != 0 || len(states) != 0 || heads == nil || states == nil {
		t.Errorf("the Hub saw %v; heads %v, states %v: want no call and two empty maps", w.hub.resolves(), heads, states)
	}
}

func TestResolveCachesOnlyResolved(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	k7 := w.key(w.b7, 5)
	refs, _, _ := s.Resolve(bg, []string{missingKey40, k7})
	if refs[0].Resolution != provenance.Missing || refs[1].Resolution != provenance.Resolved {
		t.Fatalf("healthy = %s, %s; want missing, resolved", refs[0].Resolution, refs[1].Resolution)
	}
	w.hub.SetUnavailable(true)
	again, _, _ := s.Resolve(bg, []string{missingKey40, k7})
	if again[0].Resolution != provenance.Unavailable {
		t.Errorf("the missing key with the Hub down = %s, want unavailable: missing is never cached", again[0].Resolution)
	}
	if again[1].Resolution != provenance.Resolved || !reflect.DeepEqual(again[1].Sections, refs[1].Sections) {
		t.Errorf("the resolved key with the Hub down = %+v, want it from the cache", again[1])
	}
}

func TestResolveCarriesCachedKeysForTheirHeads(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	k42 := w.key(w.b42, 6)
	first, heads1, _ := s.Resolve(bg, []string{k42})
	if first[0].Resolution != provenance.Resolved || heads1[w.b7] != (provenance.Head{HeadRevision: 6, LatestCompleteRevision: 6}) {
		t.Fatalf("first = %s, heads[b-7] %+v; want resolved and {6, 6}", first[0].Resolution, heads1[w.b7])
	}
	// A teammate's project edit: b-7@7, complete. The asserted head differs
	// from the first answer's (re-review b06-2 R2-M1).
	w.save(w.b7, step{person(discover.KeyAudience, "Everyone who touches an engagement"), discover.ChangeEdit, discover.StatusComplete}, nil)
	mark := len(w.hub.resolves())
	second, heads2, _ := s.Resolve(bg, []string{k42})
	calls := w.hub.resolves()[mark:]
	if len(calls) != 1 || !slices.Equal(calls[0], []string{k42, w.key(w.b7, 6)}) {
		t.Errorf("second Resolve's Hub calls = %v, want one carrying %s and its pin", calls, k42)
	}
	if heads2[w.b7] != (provenance.Head{HeadRevision: 7, LatestCompleteRevision: 7}) {
		t.Errorf("heads[b-7] = %+v, want {7, 7}: the moved head", heads2[w.b7])
	}
	if _, ok := heads2[w.b42]; !ok {
		t.Errorf("heads = %v, want b-42's too", heads2)
	}
	if !reflect.DeepEqual(second, first) {
		t.Errorf("the cached slot = %+v, want the first answer's %+v", second[0], first[0])
	}
}

func wantAll(t *testing.T, what string, got []provenance.CriterionReading, state provenance.CriterionState, because string, n int) {
	t.Helper()
	if len(got) != n {
		t.Fatalf("%s: %d readings %+v, want %d", what, len(got), got, n)
	}
	for _, r := range got {
		if r.State != state || r.Because != because {
			t.Errorf("%s: reading %+v, want %s %q", what, r, state, because)
		}
	}
}

func TestCriterionStatesAreRecomputedOnACachedKey(t *testing.T) {
	w := newWorld(t)
	rec := &evalRecorder{state: provenance.StateOpen}
	s := w.service(withEvaluate(rec.eval))
	k42 := w.key(w.b42, 6)
	refs1, _, st1 := s.Resolve(bg, []string{k42})
	wantAll(t, "first", st1[k42], provenance.StateOpen, "", 4)
	rec.set(provenance.StateSettled, "CI run 88 passed")
	refs2, _, st2 := s.Resolve(bg, []string{k42})
	if !reflect.DeepEqual(refs2, refs1) {
		t.Errorf("the cached slot changed: %+v, was %+v", refs2[0], refs1[0])
	}
	wantAll(t, "second", st2[k42], provenance.StateSettled, "CI run 88 passed", 4)

	// The Hub down: resolved from the cache, and no states entry at all.
	calls := len(rec.calls())
	w.hub.SetUnavailable(true)
	refs3, _, st3 := s.Resolve(bg, []string{k42, w.key(w.b42, 5)})
	w.hub.SetUnavailable(false)
	if refs3[0].Resolution != provenance.Resolved || refs3[1].Resolution != provenance.Unavailable {
		t.Errorf("Hub down = %s, %s; want resolved (cached), unavailable", refs3[0].Resolution, refs3[1].Resolution)
	}
	if len(st3) != 0 || len(rec.calls()) != calls {
		t.Errorf("Hub down: states %v, Evaluate calls %d; want no entry and no call", st3, len(rec.calls())-calls)
	}

	// 200 more branch briefs after b-42's: the walk has two pages, and its
	// second fails. No verdict from the first page's partial map.
	for i := 0; i < 200; i++ {
		w.createBrief(w.project, hubapi.ScopeBranch, fmt.Sprintf("refs/heads/extra-%03d", i))
	}
	lists := len(w.hub.lists())
	w.hub.failList(2, hubclient.ErrUnavailable)
	refs4, _, st4 := s.Resolve(bg, []string{k42, missingKey40, ""})
	if refs4[0].Resolution != provenance.Resolved || refs4[1].Resolution != provenance.Missing {
		t.Errorf("failed walk = %s, %s; want resolved, missing", refs4[0].Resolution, refs4[1].Resolution)
	}
	if len(st4) != 0 || len(rec.calls()) != calls || len(w.hub.lists())-lists != 2 {
		t.Errorf("failed walk: states %v, Evaluate calls %d, List calls %d; want none, none, 2",
			st4, len(rec.calls())-calls, len(w.hub.lists())-lists)
	}
	// The control: with the walk healthy again, the reading is back.
	_, _, st5 := s.Resolve(bg, []string{k42})
	wantAll(t, "healthy again", st5[k42], provenance.StateSettled, "CI run 88 passed", 4)
}

func TestResolveEvaluatesABranchKeyWhereItsBriefIsBoundNow(t *testing.T) {
	w := newWorld(t)
	rec := &evalRecorder{state: provenance.StateSettled, because: []string{"CI run 88 passed"}}
	s := w.service(withEvaluate(rec.eval))
	k42, k7 := w.key(w.b42, 5), w.key(w.b7, 5)
	const v2 = "refs/heads/evidence-upload-v2"
	if _, err := w.hub.Fake.Rebind(bg, w.b42, v2); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	m, lists := w.mark(), len(w.hub.lists())
	refs, _, _ := s.Resolve(bg, []string{k42, k7})
	if q := rec.calls(); refs[0].Resolution != provenance.Resolved || len(q) != 1 || q[0].BranchRef != v2 || q[0].BriefKey != k42 {
		t.Fatalf("after the rebind: %s, Evaluate %+v; want one call for %s at %s", refs[0].Resolution, q, k42, v2)
	}
	// The old name, reused by a new brief.
	w.createBrief(w.project, hubapi.ScopeBranch, evidenceRef)
	s.Resolve(bg, []string{k42, k7})
	if q := rec.calls(); len(q) != 2 || q[1].BranchRef != v2 {
		t.Errorf("with the old name reused: Evaluate %+v, want a second call at %s", q, v2)
	}
	// Archived: its binding is known to be gone, so open readings, no call.
	if _, err := w.hub.Fake.Archive(bg, w.b42); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	refs, _, st := s.Resolve(bg, []string{k42, k7})
	if refs[0].Resolution != provenance.Resolved || len(rec.calls()) != 2 {
		t.Errorf("archived: %s, %d Evaluate calls; want resolved and still 2", refs[0].Resolution, len(rec.calls()))
	}
	wantAll(t, "archived", st[k42], provenance.StateOpen, "", 3)
	for _, q := range rec.calls() {
		if q.BriefKey != k42 {
			t.Errorf("Evaluate was called for %s: a project key is never evaluated by key", q.BriefKey)
		}
	}
	wantAll(t, "the project key", st[k7], provenance.StateOpen, "", 2)
	got := w.hub.lists()[lists:]
	if len(got) != 3 {
		t.Fatalf("List calls = %d, want one per Resolve", len(got))
	}
	for _, q := range got {
		if q.Scope != hubapi.ScopeBranch || q.IncludeArchived || q.Page.Limit != 200 {
			t.Errorf("List query = %+v, want branch scope, live briefs, pages of 200", q)
		}
	}
	for _, c := range w.since(m) {
		if strings.HasPrefix(c, "List(") && !strings.HasPrefix(c, "List("+w.project+",") {
			t.Errorf("List call %s, want b-42's Hub project", c)
		}
	}
}

// openOnlyWorld is the engagement-portal clone bound to a Hub project whose
// briefs have one open revision each: nothing is complete.
func openOnlyWorld(t *testing.T) *world {
	t.Helper()
	w := newBareWorld(t)
	w.project = w.createProject(projectName, teamKey)
	w.locate.addProject(localProject, projectDir, "engagement-portal")
	w.repos.set(projectDir, gh("origin", teamKey))
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	w.save(w.b7, b7Steps()[0], nil)
	w.b42 = w.createBrief(w.project, hubapi.ScopeBranch, evidenceRef)
	w.save(w.b42, b42Steps()[0], nil)
	return w
}

func TestHeadRefServesCriterionStatesBesideTheRef(t *testing.T) {
	w := newWorld(t)
	rec := &evalRecorder{state: provenance.StateSettled, because: []string{"CI run 88 passed"}}
	s := w.service(withEvaluate(rec.eval))
	ref, states, err := s.HeadRef(bg, w.branch())
	if err != nil || ref.Key != w.key(w.b42, 6) || ref.Resolution != provenance.Resolved {
		t.Fatalf("HeadRef = %s %s, %v; want b-42@6 resolved", ref.Key, ref.Resolution, err)
	}
	wantAll(t, "settled", states, provenance.StateSettled, "CI run 88 passed", len(ref.Criteria))
	for i, c := range ref.Criteria {
		if states[i].ID != c.ID {
			t.Errorf("states[%d] = %s, want %s: same ids, same order", i, states[i].ID, c.ID)
		}
	}
	if q := rec.calls(); len(q) != 1 || q[0].BranchRef != evidenceRef {
		t.Errorf("Evaluate %+v, want one call at the target's branch", q)
	}
	raw, _ := json.Marshal(ref)
	if strings.Contains(string(raw), `"state"`) {
		t.Errorf("the Ref's JSON holds a state: %s", raw)
	}
	rec.set(provenance.StateContested, "run 91 failed")
	ref2, states2, err := s.HeadRef(bg, w.branch())
	raw2, _ := json.Marshal(ref2)
	if err != nil || string(raw2) != string(raw) {
		t.Errorf("second HeadRef's Ref differs:\n%s\n%s", raw2, raw)
	}
	wantAll(t, "contested", states2, provenance.StateContested, "run 91 failed", len(ref.Criteria))

	none := openOnlyWorld(t)
	nref, nstates, err := none.service(withEvaluate(rec.eval)).HeadRef(bg, none.branch())
	if err != nil || nref.Resolution != provenance.None || nstates != nil {
		t.Errorf("HeadRef with nothing complete = %s, %v, %v; want none, nil states", nref.Resolution, nstates, err)
	}
}

func TestErasedIsNeverServedFromCache(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	k := w.key(w.b7, 5)
	if first, _, _ := s.Resolve(bg, []string{k}); first[0].Resolution != provenance.Resolved {
		t.Fatalf("first = %s, want resolved", first[0].Resolution)
	}
	w.erase(w.b7)
	again, _, _ := s.Resolve(bg, []string{k})
	if again[0].Resolution != provenance.Erased || len(again[0].Sections) != 0 || again[0].Headline != "" || !again[0].Present {
		t.Errorf("after the erasure = %+v, want erased, no text", again[0])
	}
	if _, ok := s.cache.get(k, w.clock.Now()); ok {
		t.Errorf("%s is still cached: the Hub's erased answer must evict it", k)
	}

	w2 := newWorld(t)
	s2 := w2.service()
	k2 := w2.key(w2.b7, 5)
	s2.Resolve(bg, []string{k2})
	w2.erase(w2.b7)
	w2.hub.SetUnavailable(true)
	w2.clock.Advance(DefaultCacheTTL + 1)
	if got, _, _ := s2.Resolve(bg, []string{k2}); got[0].Resolution != provenance.Unavailable {
		t.Errorf("erased, Hub down, past the TTL = %s, want unavailable", got[0].Resolution)
	}
}

// projectOnly is b-7's text that no b-42 revision also holds.
var projectOnly = []string{b7Purpose, b7Audience, b7Audience6, b7Constraints, b7Done[1], b7OutOfScope}

func holdsProjectText(t *testing.T, what, text string) {
	t.Helper()
	for _, p := range projectOnly {
		if strings.Contains(text, p) {
			t.Errorf("%s holds the erased project text %q", what, p)
		}
	}
}

func TestABranchKeyWhosePinIsErasedKeepsOnlyItsOwnText(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	k42, k7 := w.key(w.b42, 6), w.key(w.b7, 6)
	if first, _, _ := s.Resolve(bg, []string{k42}); first[0].Audience != b7Audience6 {
		t.Fatalf("first = %+v, want b-7@6's audience inherited", first[0])
	}
	w.erase(w.b7)
	refs, _, _ := s.Resolve(bg, []string{k42})
	r := refs[0]
	wantPin := &provenance.InheritedRef{Key: k7, BriefID: w.b7, Revision: 6, Digest: w.written[k7].Digest.String(), Erased: true}
	if r.Resolution != provenance.Resolved || !reflect.DeepEqual(r.Inherits, wantPin) || r.Audience != "" {
		t.Fatalf("after the erasure = %s, inherits %+v, audience %q; want resolved, %+v, \"\"", r.Resolution, r.Inherits, r.Audience, wantPin)
	}
	for _, sec := range r.Sections {
		if sec.Origin == discover.OriginProject || sec.Replaced != "" {
			t.Errorf("section %+v comes from the erased pin", sec)
		}
	}
	raw, _ := json.Marshal(r)
	holdsProjectText(t, "the Ref", string(raw))

	href, _, err := s.HeadRef(bg, w.branch())
	hraw, _ := json.Marshal(href)
	if err != nil || string(hraw) != string(raw) {
		t.Errorf("HeadRef = %s, %v; want the same Ref as Resolve's %s", hraw, err, raw)
	}
	inh := mustInherit(t, s, w.branch())
	if inh.Key != k42 || !inh.PinErased {
		t.Errorf("Inheritable = %s pinErased %v, want %s, true", inh.Key, inh.PinErased, k42)
	}
	holdsProjectText(t, "Inheritable's block", claudemd.Render(inh.Document))
}

func TestViewOfABranchShowsWhatItInherits(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	m := w.mark()
	v, err := s.View(bg, w.branch())
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	k42, k7 := w.key(w.b42, 6), w.key(w.b7, 6)
	if v.Scope != discover.ScopeBranch || v.ProjectID != localProject || v.BranchID != localBranch || v.Binding.State != BindingBound {
		t.Errorf("View = scope %s, %s/%s, binding %s", v.Scope, v.ProjectID, v.BranchID, v.Binding.State)
	}
	c42 := w.content[k42]
	b := v.Brief
	if b == nil || b.ID != w.b42 || b.HeadRevision != 6 || b.HeadKey != k42 || b.Status != discover.StatusComplete ||
		b.LatestCompleteRevision != 6 || b.LatestCompleteKey != k42 || !reflect.DeepEqual(b.Answers, c42.Answers) ||
		!reflect.DeepEqual(b.Inherits, c42.Inherits) || b.Change != c42.Change || b.SavedBy != hubclient.FakeAda.DisplayName {
		t.Errorf("Brief = %+v, want b-42's head, rev 6", b)
	}
	in := v.Inherited
	if in == nil || in.Key != k7 || in.BriefID != w.b7 || in.Revision != 6 || !reflect.DeepEqual(in.Answers, w.content[k7].Answers) {
		t.Errorf("Inherited = %+v, want b-7's latest complete, rev 6", in)
	}
	inh := mustInherit(t, s, w.branch())
	if v.Preview == nil || v.Preview.Key != inh.Key || v.Preview.Block != claudemd.Render(inh.Document) {
		t.Errorf("Preview = %+v, want Render of Inheritable's %s", v.Preview, inh.Key)
	}
	if !reflect.DeepEqual(v.Step, c42.Step(discover.ScopeBranch)) {
		t.Errorf("Step = %+v, want the head's", v.Step)
	}
	// Rule 1: local ids never reach the Hub; Hub ids never reach Locator.
	for _, c := range w.since(m) {
		if strings.Contains(c, localProject) || strings.Contains(c, localBranch) {
			t.Errorf("Hub call %s carries a local id", c)
		}
	}
	for _, id := range w.locate.seen() {
		if strings.HasPrefix(id, hubapi.PrefixProject+"-") || strings.HasPrefix(id, hubapi.PrefixBrief+"-") {
			t.Errorf("Locator was asked about the Hub id %s", id)
		}
	}

	pv, err := s.View(bg, w.projectT())
	if err != nil || pv.Scope != discover.ScopeProject || pv.Brief == nil || pv.Brief.ID != w.b7 || pv.Brief.HeadRevision != 6 ||
		pv.Inherited != nil || pv.Preview == nil || pv.Preview.Key != k7 {
		t.Errorf("project View = %+v, %v; want b-7's head, nothing inherited, a preview of b-7@6", pv, err)
	}
}

func TestHistoryIsOldestFirstWithChanges(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	h, err := s.History(bg, w.b42, "", 0)
	if err != nil || len(h.Rows) != 6 || h.Next != "" {
		t.Fatalf("History = %d rows, next %q, %v; want 6, \"\"", len(h.Rows), h.Next, err)
	}
	kinds := []discover.ChangeKind{discover.ChangeRecord, discover.ChangeSkip, discover.ChangeRecord, discover.ChangeRecord, discover.ChangeRecord, discover.ChangeEdit}
	for i, r := range h.Rows {
		k := w.key(w.b42, i+1)
		if r.Key != k || r.Revision != i+1 || r.Change != w.content[k].Change || r.Change.Kind != kinds[i] ||
			r.Status != w.written[k].Status || r.SavedBy != hubclient.FakeAda.DisplayName || r.Erased {
			t.Errorf("row %d = %+v, want %s, change %s", i, r, k, kinds[i])
		}
	}
	p1, err := s.History(bg, w.b42, "", 2)
	if err != nil || len(p1.Rows) != 2 || p1.Rows[0].Revision != 1 || p1.Next != "2" {
		t.Errorf("first page = %+v, %v; want revisions 1-2, next 2", p1, err)
	}
	if p2, err := s.History(bg, w.b42, p1.Next, 2); err != nil || len(p2.Rows) != 2 || p2.Rows[0].Revision != 3 {
		t.Errorf("second page = %+v, %v; want revisions 3-4", p2, err)
	}
	w.erase(w.b42)
	h, err = s.History(bg, w.b42, "", 0)
	if err != nil || len(h.Rows) != 6 {
		t.Fatalf("History after the erasure = %+v, %v", h, err)
	}
	for i, r := range h.Rows {
		if !r.Erased || r.Change != (discover.Change{}) || r.Status != w.written[w.key(w.b42, i+1)].Status {
			t.Errorf("erased row %d = %+v, want erased with no change", i, r)
		}
	}
}

func TestHubErrorsPassThroughUnchanged(t *testing.T) {
	conflict := &hubclient.Error{Status: 409, Problem: httpx.Problem{Error: hubapi.CodeConflict, Message: "conflict"}}
	calls := []struct {
		name string
		call func(s *Service, w *world) error
	}{
		{"View", func(s *Service, w *world) error { _, err := s.View(bg, w.branch()); return err }},
		{"History", func(s *Service, w *world) error { _, err := s.History(bg, w.b42, "", 0); return err }},
		{"HeadRef", func(s *Service, w *world) error { _, _, err := s.HeadRef(bg, w.branch()); return err }},
		{"Inheritable", func(s *Service, w *world) error { _, err := s.Inheritable(bg, w.branch()); return err }},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.hub.failResolve(1, conflict)
			var he *hubclient.Error
			if err := c.call(w.service(), w); !errors.As(err, &he) || he != conflict {
				t.Errorf("a 409 = %v, want the Hub's *Error unchanged", err)
			}
			w2 := newWorld(t)
			w2.hub.SetUnavailable(true)
			if err := c.call(w2.service(), w2); !errors.Is(err, hubclient.ErrUnavailable) {
				t.Errorf("unavailable = %v, want errors.Is ErrUnavailable", err)
			}
			w3 := newWorld(t)
			w3.hub.SetSignedOut(true)
			if err := c.call(w3.service(), w3); !errors.Is(err, hubclient.ErrSignedOut) {
				t.Errorf("signed out = %v, want errors.Is ErrSignedOut", err)
			}
		})
	}
}

// noMissingAndEveryPinInHand is what every row of the Hub-off table holds:
// no slot is missing, and no resolved branch slot lacks its pin's sections.
func noMissingAndEveryPinInHand(t *testing.T, refs []provenance.Ref) {
	t.Helper()
	for _, r := range refs {
		if r.Resolution == provenance.Missing {
			t.Errorf("%s is missing: only the Hub's own answer says missing", r.Key)
		}
		if r.Resolution != provenance.Resolved || r.Scope != string(discover.ScopeBranch) {
			continue
		}
		inherited := false
		for _, s := range r.Sections {
			inherited = inherited || s.Origin == discover.OriginProject
		}
		if r.Inherits == nil || r.Inherits.Erased || !inherited || r.Audience == "" {
			t.Errorf("%s is resolved without its pinned project revision: %+v", r.Key, r)
		}
	}
}

func resolutions(refs []provenance.Ref) []provenance.Resolution {
	out := make([]provenance.Resolution, len(refs))
	for i, r := range refs {
		out[i] = r.Resolution
	}
	return out
}

func TestHubOffIsUnavailableNotMissing(t *testing.T) {
	status500 := func() error {
		return &hubclient.Error{Status: 500, Problem: httpx.Problem{Error: hubapi.CodeInternal, Message: "boom"}}
	}
	whole := []struct {
		name   string
		nilHub bool
		fail   func(w *world)
	}{
		{"no Hub configured", true, nil},
		{"unavailable", false, func(w *world) { w.hub.SetUnavailable(true) }},
		{"a 500", false, func(w *world) { w.hub.failResolve(1, status500()) }},
		{"signed out", false, func(w *world) { w.hub.SetSignedOut(true) }},
	}
	for _, r := range whole {
		t.Run(r.name, func(t *testing.T) {
			w := newWorld(t)
			k6, k5 := w.key(w.b42, 6), w.key(w.b42, 5)
			want := []provenance.Resolution{provenance.Resolved, provenance.Unavailable, provenance.None, provenance.None, provenance.Invalid}
			var s *Service
			if r.nilHub {
				s = w.service(withoutHub())
				want[0] = provenance.Unavailable // nothing can have been cached
			} else {
				s = w.service()
				if refs, _, _ := s.Resolve(bg, []string{k6}); refs[0].Resolution != provenance.Resolved {
					t.Fatalf("the healthy call = %s", refs[0].Resolution)
				}
				r.fail(w)
			}
			refs, heads, _ := s.Resolve(bg, []string{k6, k5, "", provenance.Declined, "b-42"})
			if got := resolutions(refs); !slices.Equal(got, want) {
				t.Errorf("resolutions = %v, want %v", got, want)
			}
			if len(heads) != 0 {
				t.Errorf("heads = %v, want none from a failed call", heads)
			}
			noMissingAndEveryPinInHand(t, refs)
		})
	}

	t.Run("the second call fails", func(t *testing.T) {
		w := newWorld(t)
		s := w.service()
		k5, k6 := w.key(w.b42, 5), w.key(w.b42, 6)
		s.Resolve(bg, []string{w.key(w.b7, 5)}) // b-42@5's pin is cached; b-42@6's is not
		mark := len(w.hub.resolves())
		w.hub.failResolve(2, status500())
		refs, _, _ := s.Resolve(bg, []string{k5, k6})
		if got := resolutions(refs); !slices.Equal(got, []provenance.Resolution{provenance.Resolved, provenance.Unavailable}) {
			t.Errorf("resolutions = %v, want resolved (pin cached), unavailable", got)
		}
		want := [][]string{{k5, k6}, {w.key(w.b7, 5), w.key(w.b7, 6)}}
		if got := w.hub.resolves()[mark:]; !reflect.DeepEqual(got, want) {
			t.Errorf("Hub calls = %v, want %v", got, want)
		}
		noMissingAndEveryPinInHand(t, refs)
	})

	t.Run("the branch is cached and its pin is not", func(t *testing.T) {
		w := newWorld(t)
		s := w.service()
		k6, p6 := w.key(w.b42, 6), w.key(w.b7, 6)
		s.Resolve(bg, []string{p6}) // t0
		w.clock.Advance(DefaultCacheTTL / 2)
		s.Resolve(bg, []string{k6}) // carries the cached pin; its expiry stays t0 + TTL
		w.clock.Advance(DefaultCacheTTL/2 + time.Second)
		mark := len(w.hub.resolves())
		w.hub.failResolve(1, hubclient.ErrUnavailable)
		refs, _, _ := s.Resolve(bg, []string{k6})
		if refs[0].Resolution != provenance.Unavailable {
			t.Errorf("%s = %s, want unavailable: its pin is not in hand", k6, refs[0].Resolution)
		}
		if got := w.hub.resolves()[mark:]; !reflect.DeepEqual(got, [][]string{{k6, p6}}) {
			t.Errorf("Hub calls = %v, want one carrying %s and its pin %s", got, k6, p6)
		}
		noMissingAndEveryPinInHand(t, refs)
	})

	t.Run("a healthy Hub answers what it does not hold missing", func(t *testing.T) {
		w := newWorld(t)
		if refs, _, _ := w.service().Resolve(bg, []string{missingKey40}); refs[0].Resolution != provenance.Missing {
			t.Errorf("%s = %s, want missing", missingKey40, refs[0].Resolution)
		}
	})
}

// TestAnExpiredPinIsFetchedAgain is the healthy half of the "branch cached,
// pin not" row (re-review b05-2 R2-I1, item 3): no hook, so the kill does not
// rest on the wrapper's log.
func TestAnExpiredPinIsFetchedAgain(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	k6, p6 := w.key(w.b42, 6), w.key(w.b7, 6)
	s.Resolve(bg, []string{p6})
	w.clock.Advance(DefaultCacheTTL / 2)
	s.Resolve(bg, []string{k6})
	w.clock.Advance(DefaultCacheTTL/2 + time.Second)
	m := w.mark()
	refs, _, _ := s.Resolve(bg, []string{k6})
	if refs[0].Resolution != provenance.Resolved || refs[0].Audience != b7Audience6 {
		t.Errorf("%s = %s (audience %q), want resolved against its pin", k6, refs[0].Resolution, refs[0].Audience)
	}
	want := []string{"Resolve(" + mustJSON(t, []string{k6, p6}) + ")"}
	if got := w.since(m); !slices.Equal(got, want) {
		t.Errorf("Hub calls = %v, want %v", got, want)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRenameDoesNotLookLikeAHandEdit(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	before := mustInherit(t, s, w.branch())
	a := claudemd.Render(before.Document)
	if !strings.Contains(a, "# Branch brief — evidence-upload") {
		t.Fatalf("block A has no envelope title:\n%s", a)
	}
	proj := claudemd.Render(mustInherit(t, s, w.projectT()).Document)

	w.hub.renameProject(w.project, "Evidence Portal")
	if _, err := w.hub.Fake.Rebind(bg, w.b42, "refs/heads/evidence-intake"); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	w.locate.setBranch(localBranch, "evidence-intake")
	m := w.mark()
	after := mustInherit(t, s, w.branch())
	if after.Key != before.Key || after.Key != w.key(w.b42, 6) {
		t.Fatalf("after the rename = %s, want %s", after.Key, before.Key)
	}
	if got := claudemd.Render(after.Document); got != a {
		t.Errorf("the block changed after a rename:\n%s\nwas\n%s", got, a)
	}
	blk, ok, err := claudemd.Inspect(a)
	if err != nil || !ok || blk.Marker.BriefID != after.Document.BriefID || blk.Marker.Revision != after.Document.Revision ||
		!claudemd.SameText(blk.Text, claudemd.Render(after.Document)) {
		t.Errorf("Inspect(A) = %+v, %v, %v: a file holding A reads edited", blk.Marker, ok, err)
	}
	if got := claudemd.Render(mustInherit(t, s, w.projectT()).Document); got != proj {
		t.Errorf("the project key's block changed after a rename")
	}
	// The guards: the hooks did what the test says they did.
	if !slices.Contains(w.since(m), "ForBranch("+w.project+", refs/heads/evidence-intake)") {
		t.Errorf("ForBranch was not asked for the rebound ref: %v", w.since(m))
	}
	w.clock.Advance(61 * time.Second)
	if b := mustBinding(t, s, localProject); b.HubProjectName != "Evidence Portal" {
		t.Errorf("Binding after the memo = %q, want the renamed Evidence Portal", b.HubProjectName)
	}
}

func TestRevisionCacheExpiresAndCaps(t *testing.T) {
	c := newRevisionCache(2)
	t0 := newClock().Now()
	c.put("a", piece{digest: "a"}, t0, time.Minute)
	c.put("b", piece{digest: "b"}, t0.Add(1*time.Second), time.Minute)
	c.put("c", piece{digest: "c"}, t0.Add(2*time.Second), time.Minute) // full: a, the soonest to expire, goes
	if _, ok := c.get("a", t0.Add(3*time.Second)); ok || c.len() != 2 {
		t.Errorf("a survived the cap; len %d", c.len())
	}
	c.put("b", piece{digest: "b2"}, t0.Add(3*time.Second), time.Hour) // held and live: left as it is
	if p, ok := c.get("b", t0.Add(4*time.Second)); !ok || p.digest != "b" {
		t.Errorf("b = %+v, %v; want the first bytes", p, ok)
	}
	if _, ok := c.get("b", t0.Add(61*time.Second)); ok {
		t.Errorf("b outlived its first expiry: put extended it")
	}
	if _, ok := c.get("c", t0.Add(61*time.Second)); !ok {
		t.Errorf("c expired early")
	}
	c.evict("c")
	if _, ok := c.get("c", t0.Add(61*time.Second)); ok || c.len() != 0 {
		t.Errorf("evict left c; len %d", c.len())
	}
}
