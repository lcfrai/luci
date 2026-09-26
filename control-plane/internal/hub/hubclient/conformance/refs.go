package conformance

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The refs group: server tasks 20 (brief keys) and 22 (design keys), contract
// §7.5's resolver. The last row is carried/task-05.md item 13's refs half.
var refRows = []row{
	{name: "five_states", fn: fiveStates},
	{name: "design_key_resolves", fn: designKeyResolves},
	{name: "org_admin_is_a_non_member", fn: orgAdminRefs},
}

// wantResult asserts one result's key, kind and resolution.
func wantResult(t *testing.T, r hubapi.RefResult, key string, kind hubapi.KeyKind, res hubapi.Resolution) {
	t.Helper()
	gotKind := hubapi.KeyKind("null")
	if r.Kind != nil {
		gotKind = *r.Kind
	}
	wantKind := kind
	if res == hubapi.Invalid {
		wantKind = "null"
	}
	if r.Key != key || gotKind != wantKind || r.Resolution != res {
		t.Errorf("result %s = kind %s, %s; want %s kind %s, %s", r.Key, gotKind, r.Resolution, key, wantKind, res)
	}
}

// resolved, missing, unreadable-is-missing, erased, invalid; request order
// kept and a repeat answered in its slot; heads only for what resolved.
func fiveStates(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	r1 := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusOpen)
	u := newTeam(t, e.Outsider)
	up := newProjectIn(t, e.Outsider, u.ID)
	theirs := newBrief(t, e.Outsider, up.ID, featRef)
	tr := write(t, e.Outsider, up.ID, theirs.ID, 0, hubapi.StatusComplete)
	missing := hubapi.BriefKey{BriefID: b.ID, Revision: 9}.String()
	keys := []string{r1.Key, missing, tr.Key, "b-42", r1.Key}
	var erasedKey, erasedID string
	if e.Erase != nil {
		gone := newBrief(t, e.Member, p.ID, "refs/heads/gone")
		erasedKey, erasedID = write(t, e.Member, p.ID, gone.ID, 0, hubapi.StatusComplete).Key, gone.ID
		ok(t, e.Erase(gone.ID, "Conformance: the erased state"))
		keys = append(keys, erasedKey)
	} else {
		t.Log("conformance: this Env has no Erase; the erased state is not run")
	}
	got := must(e.Member.Resolve(bg, keys))(t)
	if len(got.Results) != len(keys) {
		t.Fatalf("Resolve answered %d results for %d keys", len(got.Results), len(keys))
	}
	wantResult(t, got.Results[0], r1.Key, hubapi.KeyBrief, hubapi.Resolved)
	wantResult(t, got.Results[1], missing, hubapi.KeyBrief, hubapi.Missing)
	wantResult(t, got.Results[2], tr.Key, hubapi.KeyBrief, hubapi.Missing)
	wantResult(t, got.Results[3], "b-42", "", hubapi.Invalid)
	wantResult(t, got.Results[4], r1.Key, hubapi.KeyBrief, hubapi.Resolved)
	for _, i := range []int{0, 4} {
		if rv := got.Results[i].Revision; rv == nil || rv.Digest != r1.Digest || len(rv.Content) == 0 || got.Results[i].Approval != nil {
			t.Errorf("slot %d: revision %+v, approval %v; want revision 1 with its digest and content, no approval", i, rv, got.Results[i].Approval)
		}
	}
	for _, i := range []int{1, 2, 3} {
		if got.Results[i].Revision != nil || got.Results[i].Approval != nil {
			t.Errorf("slot %d (%s) carries a record", i, got.Results[i].Resolution)
		}
	}
	if h, in := got.Heads[b.ID]; !in || h.HeadRevision != 2 || h.LatestCompleteRevision != 1 || h.Archived {
		t.Errorf("heads[%s] = %+v (%v), want head 2, latest complete 1, live", b.ID, h, in)
	}
	if _, in := got.Heads[theirs.ID]; in {
		t.Errorf("heads names %s, a brief the caller cannot read", theirs.ID)
	}
	if erasedKey != "" {
		r := got.Results[5]
		wantResult(t, r, erasedKey, hubapi.KeyBrief, hubapi.Erased)
		if r.Revision == nil || r.Revision.ErasedAt == nil || !erasedContent(r.Revision.Content) || r.Revision.Digest.IsZero() {
			t.Errorf("the erased slot = %+v, want the revision with its digest, erasedAt set and no content", r.Revision)
		}
		if h, in := got.Heads[erasedID]; !in || !h.Archived {
			t.Errorf("heads[%s] = %+v (%v), want the erased brief, archived", erasedID, h, in)
		}
	}
	if len(got.Heads) != 1+len(keys)-5 {
		t.Errorf("heads = %v, want exactly the resolved and erased briefs", got.Heads)
	}
}

// an approved design key → resolved with the approval, and its brief in heads;
// an unapproved one → missing; to the outsider → missing, heads {}.
func designKeyResolves(t *testing.T, e Env) {
	a, ap := approve(t, e)
	unapproved := newDesignKey()
	got := must(e.Viewer.Resolve(bg, []string{ap.DesignKey, unapproved}))(t)
	if len(got.Results) != 2 {
		t.Fatalf("Resolve answered %d results for 2 keys", len(got.Results))
	}
	wantResult(t, got.Results[0], ap.DesignKey, hubapi.KeyDesign, hubapi.Resolved)
	if r := got.Results[0]; r.Approval == nil || r.Approval.ID != ap.ID || r.Approval.BriefKey != a.rev.Key || r.Revision != nil {
		t.Errorf("the design slot = approval %+v, revision %v; want approval %s and no revision", r.Approval, r.Revision, ap.ID)
	}
	wantResult(t, got.Results[1], unapproved, hubapi.KeyDesign, hubapi.Missing)
	if got.Results[1].Approval != nil {
		t.Errorf("an unapproved design key carries an approval")
	}
	if h, in := got.Heads[a.rev.BriefID]; !in || h.HeadRevision != 1 || h.LatestCompleteRevision != 1 {
		t.Errorf("heads[%s] = %+v (%v), want the approval's brief, head 1", a.rev.BriefID, h, in)
	}
	if len(got.Heads) != 1 {
		t.Errorf("heads = %v, want only the approval's brief", got.Heads)
	}
	for _, s := range strangers(t, e) {
		out := must(s.hub.Resolve(bg, []string{ap.DesignKey}))(t)
		wantResult(t, out.Results[0], ap.DesignKey, hubapi.KeyDesign, hubapi.Missing)
		if out.Results[0].Approval != nil || len(out.Heads) != 0 {
			t.Errorf("%s: the design slot = %+v, heads %v; want missing and heads {}", s.name, out.Results[0], out.Heads)
		}
	}
}

// carried item 13, refs' half: Resolve of T's brief key is missing, no
// revision, heads {}, to an org admin in no team as to the outsider.
func orgAdminRefs(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	r := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	for _, s := range strangers(t, e) {
		got := must(s.hub.Resolve(bg, []string{r.Key}))(t)
		if len(got.Results) != 1 {
			t.Fatalf("%s: %d results for one key", s.name, len(got.Results))
		}
		wantResult(t, got.Results[0], r.Key, hubapi.KeyBrief, hubapi.Missing)
		if got.Results[0].Revision != nil || got.Heads == nil || len(got.Heads) != 0 {
			t.Errorf("%s: revision %v, heads %#v; want none and {}", s.name, got.Results[0].Revision, got.Heads)
		}
	}
}
