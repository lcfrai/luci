package conformance

import (
	"slices"
	"strconv"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The briefs group: server tasks 18 and 19 (contract §7.5). The rows after the
// part's are carried/task-05.md items 6, 7, 11 and 13's briefs half.
var briefRows = []row{
	{name: "brief_exists", fn: briefExists},
	{name: "archive_frees_ref", fn: archiveFreesRef},
	{name: "archived_refuses_writes", fn: archivedRefusesWrites},
	{name: "numbering_and_conflict", fn: numberingAndConflict},
	{name: "batch_all_or_nothing", fn: batchAllOrNothing},
	{name: "viewer_cannot_write", fn: viewerCannotWrite},
	{name: "list_filters", fn: listFilters},
	{name: "latest_complete", fn: latestComplete},
	{name: "batch_of_sixty_four", fn: batchOfSixtyFour},
	{name: "self_rebind_is_noop", fn: selfRebindIsNoop},
	{name: "rebind_archived_first", fn: rebindArchivedFirst},
	{name: "resolve_zero_keys", fn: resolveZeroKeys},
	{name: "list_cursor_is_this_projects", fn: listCursorIsThisProjects},
	{name: "revisions_cursor", fn: revisionsCursor},
	{name: "org_admin_is_a_non_member", fn: orgAdminBriefs},
}

const featRef = "refs/heads/feat-x"

// a second live brief of one scope and ref → 409 brief_exists naming the live one.
func briefExists(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	_, err := e.Member.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: featRef})
	wantErr(t, err, 409, hubapi.CodeBriefExists, "briefId", b.ID)
	pb := newBrief(t, e.Member, p.ID, "")
	_, err = e.Admin.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject})
	wantErr(t, err, 409, hubapi.CodeBriefExists, "briefId", pb.ID)
	if other := newBrief(t, e.Member, p.ID, "refs/heads/other"); other.ID == b.ID {
		t.Errorf("a brief on another ref reused %s", b.ID)
	}
}

// archiving frees the ref: a new brief on it is 201, and the old one stays readable.
func archiveFreesRef(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	if h := must(e.Member.Archive(bg, b.ID))(t); h.ArchivedAt == nil || h.HeadRevision != 1 {
		t.Errorf("Archive = %+v, want archivedAt set and the revision untouched", h)
	}
	next := newBrief(t, e.Member, p.ID, featRef)
	if next.ID == b.ID {
		t.Fatalf("the ref's new brief reused the archived one's id")
	}
	if got := must(e.Viewer.Brief(bg, b.ID))(t); got.Head.ArchivedAt == nil || got.Revision == nil {
		t.Errorf("the archived brief reads as %+v, want archived with its revision", got.Head)
	}
}

// an archived brief takes no revision, no second archive and no rebind.
func archivedRefusesWrites(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	must(e.Member.Archive(bg, b.ID))(t)
	_, err := e.Member.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 0, Content: content(hubapi.StatusOpen)}})
	wantErr(t, err, 409, hubapi.CodeBriefArchived, "briefId", b.ID)
	_, err = e.Member.Archive(bg, b.ID)
	wantErr(t, err, 409, hubapi.CodeBriefArchived, "briefId", b.ID)
	_, err = e.Member.Rebind(bg, b.ID, "refs/heads/renamed")
	wantErr(t, err, 409, hubapi.CodeBriefArchived, "briefId", b.ID)
	if got := must(e.Member.Brief(bg, b.ID))(t); got.Head.HeadRevision != 0 || got.Head.BranchRef != featRef {
		t.Errorf("after the refusals the head is %+v, want revision 0 on %s", got.Head, featRef)
	}
}

// base 0 → 1, base 1 → 2; a stale base → 409 revision_conflict with the head.
func numberingAndConflict(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	for base := 0; base < 2; base++ {
		r := write(t, e.Member, p.ID, b.ID, base, hubapi.StatusOpen)
		if want := (hubapi.BriefKey{BriefID: b.ID, Revision: base + 1}).String(); r.Revision != base+1 || r.Key != want {
			t.Errorf("base %d wrote %d %s, want %s", base, r.Revision, r.Key, want)
		}
	}
	_, err := e.Member.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 1, Content: content(hubapi.StatusOpen)}})
	wantErr(t, err, 409, hubapi.CodeRevisionConflict, "briefId", b.ID, "headRevision", "2")
	if h := must(e.Member.Brief(bg, b.ID))(t).Head; h.HeadRevision != 2 {
		t.Errorf("after the conflict the head is %d, want 2", h.HeadRevision)
	}
}

// four items, the third stale: nothing is written.
func batchAllOrNothing(t *testing.T, e Env) {
	p := newProject(t, e)
	var bs []hubapi.BriefHead
	for i := range 4 {
		bs = append(bs, newBrief(t, e.Member, p.ID, "refs/heads/b"+strconv.Itoa(i)))
	}
	write(t, e.Member, p.ID, bs[2].ID, 0, hubapi.StatusOpen) // item 3's head is now 1
	var writes []hubapi.RevisionWrite
	for _, b := range bs {
		writes = append(writes, hubapi.RevisionWrite{BriefID: b.ID, BaseRevision: 0, Content: content(hubapi.StatusComplete)})
	}
	_, err := e.Member.WriteRevisions(bg, p.ID, writes)
	wantErr(t, err, 409, hubapi.CodeRevisionConflict, "briefId", bs[2].ID, "headRevision", "1")
	for i, b := range bs {
		want := 0
		if i == 2 {
			want = 1
		}
		if h := must(e.Member.Brief(bg, b.ID))(t).Head; h.HeadRevision != want || h.LatestCompleteRevision != 0 {
			t.Errorf("item %d after a refused batch: head %d, latest complete %d; want %d, 0", i+1, h.HeadRevision, h.LatestCompleteRevision, want)
		}
	}
}

// a viewer writes nothing: 403 forbidden, required member.
func viewerCannotWrite(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	_, err := e.Viewer.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 0, Content: content(hubapi.StatusOpen)}})
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	_, err = e.Viewer.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject})
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	if h := must(e.Viewer.Brief(bg, b.ID))(t).Head; h.HeadRevision != 0 {
		t.Errorf("a viewer's refused write moved the head to %d", h.HeadRevision)
	}
}

// briefIDs is a page's ids in order.
func briefIDs(t *testing.T, p hubapi.Page[hubapi.BriefHead]) []string {
	t.Helper()
	if p.Items == nil {
		t.Errorf("a page's items are nil, want []")
	}
	var out []string
	for _, h := range p.Items {
		out = append(out, h.ID)
	}
	return out
}

// List filters by scope, ref and archived, oldest first, and pages by 1 (HA4-2).
// The briefs are made on distinct clock ticks (carried item 3).
func listFilters(t *testing.T, e Env) {
	p := newProject(t, e)
	var made []hubapi.BriefHead
	for _, ref := range []string{"", "refs/heads/one", "refs/heads/gone", "refs/heads/two"} {
		made = append(made, newBrief(t, e.Member, p.ID, ref))
		tick(e)
	}
	pb, one, gone, two := made[0].ID, made[1].ID, made[2].ID, made[3].ID
	must(e.Member.Archive(bg, gone))(t)
	for _, c := range []struct {
		q    hubapi.BriefQuery
		want []string
	}{
		{hubapi.BriefQuery{}, []string{pb, one, two}},
		{hubapi.BriefQuery{Scope: hubapi.ScopeBranch}, []string{one, two}},
		{hubapi.BriefQuery{Scope: hubapi.ScopeProject}, []string{pb}},
		{hubapi.BriefQuery{IncludeArchived: true}, []string{pb, one, gone, two}},
		{hubapi.BriefQuery{BranchRef: "refs/heads/one"}, []string{one}},
		{hubapi.BriefQuery{BranchRef: "refs/heads/gone"}, nil},
		{hubapi.BriefQuery{BranchRef: "refs/heads/gone", IncludeArchived: true}, []string{gone}},
	} {
		if got := briefIDs(t, must(e.Viewer.List(bg, p.ID, c.q))(t)); !slices.Equal(got, c.want) {
			t.Errorf("List(%+v) = %v, want %v", c.q, got, c.want)
		}
	}
	var walked []string
	q := hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 1}}
	for range 4 {
		page := must(e.Viewer.List(bg, p.ID, q))(t)
		walked = append(walked, briefIDs(t, page)...)
		if page.Next == nil {
			break
		}
		q.Page.After = *page.Next
	}
	if !slices.Equal(walked, []string{pb, one, two}) {
		t.Errorf("paging by 1 walked %v, want %v", walked, []string{pb, one, two})
	}
	for _, bad := range []struct {
		q     hubapi.BriefQuery
		field string
	}{
		{hubapi.BriefQuery{Scope: "repo"}, "scope"},
		{hubapi.BriefQuery{BranchRef: "one"}, "branchRef"},
		{hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 201}}, "limit"},
	} {
		_, err := e.Viewer.List(bg, p.ID, bad.q)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", bad.field)
	}
}

// latestComplete is null until a complete revision, then that revision, and
// moves only on complete (HA4-4); ForBranch and Brief agree.
func latestComplete(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	check := func(head, complete int) {
		t.Helper()
		fb := must(e.Viewer.ForBranch(bg, p.ID, featRef))(t)
		one := must(e.Viewer.Brief(bg, b.ID))(t)
		for name, bw := range map[string]*hubapi.BriefWithRevision{"ForBranch": fb.BranchBrief, "Brief": &one} {
			if bw == nil {
				t.Fatalf("%s: no brief", name)
			}
			if revAt(bw.Revision) != head || revAt(bw.LatestComplete) != complete || bw.Head.LatestCompleteRevision != complete {
				t.Errorf("%s: revision @%d, latestComplete @%d (head says %d); want @%d, @%d", name,
					revAt(bw.Revision), revAt(bw.LatestComplete), bw.Head.LatestCompleteRevision, head, complete)
			} else if complete > 0 && (bw.LatestComplete.Status != hubapi.StatusComplete || len(bw.LatestComplete.Content) == 0) {
				t.Errorf("%s: latestComplete = %+v, want a complete revision with content", name, bw.LatestComplete)
			}
		}
	}
	check(0, 0)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusOpen)
	check(2, 1)
	write(t, e.Member, p.ID, b.ID, 2, hubapi.StatusSkipped)
	check(3, 1)
	write(t, e.Member, p.ID, b.ID, 3, hubapi.StatusComplete)
	check(4, 4)
}

func revAt(r *hubapi.BriefRevision) int {
	if r == nil {
		return 0
	}
	return r.Revision
}

// 64 revisions in one call → 201 in request order; 65 → 400 field revisions (HA4-3).
func batchOfSixtyFour(t *testing.T, e Env) {
	p := newProject(t, e)
	var writes []hubapi.RevisionWrite
	for i := range hubapi.MaxRevisionBatch + 1 {
		b := newBrief(t, e.Member, p.ID, "refs/heads/n"+strconv.Itoa(i))
		writes = append(writes, hubapi.RevisionWrite{BriefID: b.ID, BaseRevision: 0, Content: content(hubapi.StatusOpen)})
	}
	_, err := e.Member.WriteRevisions(bg, p.ID, writes)
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "revisions")
	got := must(e.Member.WriteRevisions(bg, p.ID, writes[:hubapi.MaxRevisionBatch]))(t)
	if len(got) != hubapi.MaxRevisionBatch {
		t.Fatalf("64 writes answered %d revisions", len(got))
	}
	for i, r := range got {
		if r.BriefID != writes[i].BriefID || r.Revision != 1 {
			t.Errorf("revision %d = %s@%d, want %s@1 in request order", i, r.BriefID, r.Revision, writes[i].BriefID)
		}
	}
}

// carried item 6: Rebind onto the brief's own ref is 200 and changes nothing.
func selfRebindIsNoop(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	tick(e)
	h := must(e.Member.Rebind(bg, b.ID, featRef))(t)
	if h.BranchRef != featRef || h.UpdatedAt != b.UpdatedAt {
		t.Errorf("a self-rebind = ref %s updatedAt %s, want %s and %s unchanged", h.BranchRef, h.UpdatedAt, featRef, b.UpdatedAt)
	}
}

// carried item 6: an archived brief's rebind is brief_archived, before
// brief_exists for the live brief holding the ref.
func rebindArchivedFirst(t *testing.T, e Env) {
	p := newProject(t, e)
	b1 := newBrief(t, e.Member, p.ID, featRef)
	must(e.Member.Archive(bg, b1.ID))(t)
	newBrief(t, e.Member, p.ID, featRef)
	_, err := e.Member.Rebind(bg, b1.ID, featRef)
	wantErr(t, err, 409, hubapi.CodeBriefArchived, "briefId", b1.ID)
}

// carried item 6: Resolve of no keys is empty results and empty heads, both non-nil.
func resolveZeroKeys(t *testing.T, e Env) {
	got := must(e.Member.Resolve(bg, nil))(t)
	if got.Results == nil || len(got.Results) != 0 || got.Heads == nil || len(got.Heads) != 0 {
		t.Errorf("Resolve(nil) = %#v, want results [] and heads {}", got)
	}
}

// carried item 7: a brief of another project, of this team or another, is no
// cursor of this project's list: the same 400 as an id naming nothing.
func listCursorIsThisProjects(t *testing.T, e Env) {
	p := newProject(t, e)
	newBrief(t, e.Member, p.ID, featRef)
	sibling := newBrief(t, e.Member, newProject(t, e).ID, featRef)
	u := newTeam(t, e.Outsider)
	foreign := newBrief(t, e.Outsider, newProjectIn(t, e.Outsider, u.ID).ID, featRef)
	for _, after := range []string{sibling.ID, foreign.ID, "bf-0000000000000000"} {
		_, err := e.Member.List(bg, p.ID, hubapi.BriefQuery{Page: hubapi.PageRequest{After: after}})
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "after")
	}
}

// carried items 7 and 11: the revisions cursor is a revision number as a list
// gave it; past the end is an empty page; any other spelling is 400.
func revisionsCursor(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	for base := range 3 {
		write(t, e.Member, p.ID, b.ID, base, hubapi.StatusOpen)
	}
	for after, want := range map[string]int{"0": 3, "3": 0, "99": 0} {
		page := must(e.Viewer.Revisions(bg, b.ID, hubapi.PageRequest{After: after}))(t)
		if len(page.Items) != want || page.Items == nil || page.Next != nil {
			t.Errorf("Revisions(after %s) = %d items, next %v; want %d, next null", after, len(page.Items), page.Next, want)
		}
	}
	for _, after := range []string{"01", "-1", "+1", "99999999999999999999"} {
		_, err := e.Viewer.Revisions(bg, b.ID, hubapi.PageRequest{After: after})
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "after")
	}
}

// carried item 13, briefs' half: every brief route answers an org admin in no
// team as it answers the outsider, and changes nothing.
func orgAdminBriefs(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	for _, s := range strangers(t, e) {
		t.Run(s.name, func(t *testing.T) {
			h := s.hub
			noProject := func(err error) {
				t.Helper()
				wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
			}
			noBrief := func(err error) { t.Helper(); wantErr(t, err, 404, hubapi.CodeBriefNotFound, "briefId", b.ID) }
			_, err := h.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject})
			noProject(err)
			_, err = h.List(bg, p.ID, hubapi.BriefQuery{})
			noProject(err)
			_, err = h.ForBranch(bg, p.ID, featRef)
			noProject(err)
			_, err = h.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 1, Content: content(hubapi.StatusOpen)}})
			noProject(err)
			_, err = h.Brief(bg, b.ID)
			noBrief(err)
			_, err = h.Rebind(bg, b.ID, "refs/heads/stolen")
			noBrief(err)
			_, err = h.Archive(bg, b.ID)
			noBrief(err)
			_, err = h.Revisions(bg, b.ID, hubapi.PageRequest{})
			noBrief(err)
		})
	}
	if h := must(e.Member.Brief(bg, b.ID))(t).Head; h.HeadRevision != 1 || h.ArchivedAt != nil || h.BranchRef != featRef {
		t.Errorf("after the strangers' calls the head is %+v, want @1, live, on %s", h, featRef)
	}
}
