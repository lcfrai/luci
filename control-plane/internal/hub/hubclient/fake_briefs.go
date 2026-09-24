package hubclient

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The fake's briefs, revisions and resolver, brief half (contract §7.5), and
// the org admin's erasure (§7.12).

type fakeBrief struct {
	id, projectID  string
	scope          string // hubapi.ScopeProject | hubapi.ScopeBranch
	branchRef      string // the current ref; "" for a project brief
	createdAt      string
	createdBy      string // a user id
	updatedAt      string
	archivedAt     string // "" while live
	latestComplete int
	revisions      []*fakeRevision // revisions[i] is revision i+1
	seq            int64
}

// fakeRevision is immutable once written, except that an erasure removes its
// canonical bytes and sets erasedAt.
type fakeRevision struct {
	revision  int
	scope     string // the envelope's
	branchRef string // the ref it was written under
	digest    hubapi.Digest
	status    string
	canonical []byte // the §3.4 envelope; nil once erased
	savedAt   string
	savedBy   string // a user id
	erasedAt  string // "" unless erased
}

func (b *fakeBrief) order() int64 { return b.seq }

// briefOf is briefID's brief and the caller's role in its project's team; a
// brief the caller cannot read is 404, as if it did not exist (§5.3).
func (w *fakeWorld) briefOf(briefID, me string) (*fakeBrief, hubapi.Role, error) {
	if b := w.briefs[briefID]; b != nil {
		if m := w.teams[w.projects[b.projectID].teamID].members[me]; m != nil {
			return b, m.role, nil
		}
	}
	return nil, "", briefNotFound(briefID)
}

func briefNotFound(briefID string) error {
	return problem(404, hubapi.CodeBriefNotFound, "There is no such brief that you can see.", "briefId", briefID)
}

func briefArchived(briefID string) error {
	return problem(409, hubapi.CodeBriefArchived, "That brief is archived and takes no more writes.", "briefId", briefID)
}

// liveBrief is the project's one live brief of that scope and ref, or nil.
func (w *fakeWorld) liveBrief(projectID, scope, ref string) *fakeBrief {
	for _, b := range w.briefs {
		if b.projectID == projectID && b.scope == scope && b.branchRef == ref && b.archivedAt == "" {
			return b
		}
	}
	return nil
}

// branchRefOK is a ref a branch brief may hold: non-empty and §3.3's grammar.
func branchRefOK(ref string) bool { return ref != "" && hubapi.ValidBranchRef(ref) }

func badBranchRef() error {
	return badRequest("branchRef", "A branch ref is a full refs/heads/ name git would accept.")
}

func (w *fakeWorld) renderHead(b *fakeBrief) hubapi.BriefHead {
	h := hubapi.BriefHead{ID: b.id, ProjectID: b.projectID, Scope: b.scope, BranchRef: b.branchRef,
		HeadRevision: len(b.revisions), LatestCompleteRevision: b.latestComplete,
		CreatedAt: b.createdAt, CreatedBy: w.users[b.createdBy], UpdatedAt: b.updatedAt}
	if n := len(b.revisions); n > 0 {
		r := b.revisions[n-1]
		key, digest, status := hubapi.BriefKey{BriefID: b.id, Revision: n}.String(), r.digest, r.status
		h.HeadKey, h.HeadDigest, h.HeadStatus = &key, &digest, &status
	}
	if b.archivedAt != "" {
		at := b.archivedAt
		h.ArchivedAt = &at
	}
	return h
}

// renderRevision serves content by parsing the stored envelope, never from a
// second copy (contract §3.4), so the caller holds its own copy. An erased
// revision keeps key, digest, status, savedAt and savedBy; its content is null.
func (w *fakeWorld) renderRevision(b *fakeBrief, r *fakeRevision) hubapi.BriefRevision {
	out := hubapi.BriefRevision{Key: hubapi.BriefKey{BriefID: b.id, Revision: r.revision}.String(), BriefID: b.id,
		Revision: r.revision, Digest: r.digest, ProjectID: b.projectID, Scope: r.scope, BranchRef: r.branchRef,
		Status: r.status, SavedAt: r.savedAt, SavedBy: w.users[r.savedBy]}
	if r.erasedAt != "" {
		at := r.erasedAt
		out.ErasedAt = &at
		return out
	}
	env, err := hubapi.ParseBriefEnvelope(r.canonical)
	if err != nil {
		panic("hubclient.Fake: a stored revision does not parse: " + err.Error()) // the fake wrote it; a bug
	}
	out.Content = env.Content
	return out
}

// revisionAt is revision n of b rendered, or nil for 0.
func (w *fakeWorld) revisionAt(b *fakeBrief, n int) *hubapi.BriefRevision {
	if n == 0 {
		return nil
	}
	r := w.renderRevision(b, b.revisions[n-1])
	return &r
}

// withRevision is routes 21's and 22's shape: the head, its revision, and the
// latest complete revision (HA4-4: null exactly when none is complete).
func (w *fakeWorld) withRevision(b *fakeBrief) hubapi.BriefWithRevision {
	return hubapi.BriefWithRevision{Head: w.renderHead(b), Revision: w.revisionAt(b, len(b.revisions)),
		LatestComplete: w.revisionAt(b, b.latestComplete)}
}

// ── briefs (§7.5, task 18) ──────────────────────────────────────────────────

// CreateBrief: one live project brief per project, one live branch brief per
// (project, branchRef); a second is brief_exists naming the live one.
func (f *Fake) CreateBrief(_ context.Context, projectID string, req hubapi.CreateBriefRequest) (hubapi.BriefHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateBrief", projectID, req)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.BriefHead{}, err
	}
	switch req.Scope {
	case hubapi.ScopeProject:
		if req.BranchRef != "" {
			return hubapi.BriefHead{}, badRequest("branchRef", "A project brief has no branch ref.")
		}
	case hubapi.ScopeBranch:
		if !branchRefOK(req.BranchRef) {
			return hubapi.BriefHead{}, badBranchRef()
		}
	default:
		return hubapi.BriefHead{}, badRequest("scope", "A scope is project or branch.")
	}
	if live := w.liveBrief(p.id, req.Scope, req.BranchRef); live != nil {
		return hubapi.BriefHead{}, problem(409, hubapi.CodeBriefExists, "A live brief already has that scope and branch.", "briefId", live.id)
	}
	now := w.stamp()
	b := &fakeBrief{id: ids.Wide(hubapi.PrefixBrief), projectID: p.id, scope: req.Scope, branchRef: req.BranchRef,
		createdAt: now, createdBy: me, updatedAt: now, seq: w.next()}
	w.briefs[b.id] = b
	return w.renderHead(b), nil
}

// List is route 19 (HA4-2): the project's heads oldest first, filtered, paged;
// the cursor is the last head's id.
func (f *Fake) List(_ context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("List", projectID, q)
	if err != nil {
		return hubapi.Page[hubapi.BriefHead]{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Page[hubapi.BriefHead]{}, err
	}
	if q.Scope != "" && q.Scope != hubapi.ScopeProject && q.Scope != hubapi.ScopeBranch {
		return hubapi.Page[hubapi.BriefHead]{}, badRequest("scope", "A scope is project or branch.")
	}
	if q.BranchRef != "" && !hubapi.ValidBranchRef(q.BranchRef) {
		return hubapi.Page[hubapi.BriefHead]{}, badBranchRef()
	}
	limit, err := pageLimit(q.Page.Limit)
	if err != nil {
		return hubapi.Page[hubapi.BriefHead]{}, err
	}
	var after int64
	if q.Page.After != "" {
		c := w.briefs[q.Page.After]
		if c == nil || c.projectID != p.id {
			return hubapi.Page[hubapi.BriefHead]{}, badRequest("after", "That cursor is not one this list gave.")
		}
		after = c.seq
	}
	var rows []*fakeBrief
	for _, b := range inOrder(w.briefs) {
		if b.projectID == p.id && b.seq > after && (q.Scope == "" || b.scope == q.Scope) &&
			(q.BranchRef == "" || b.branchRef == q.BranchRef) && (q.IncludeArchived || b.archivedAt == "") {
			rows = append(rows, b)
		}
	}
	return pageOf(rows, limit, func(b *fakeBrief) string { return b.id }, w.renderHead), nil
}

// ForBranch is never a 404 for an absent brief; archived ones do not answer.
func (f *Fake) ForBranch(_ context.Context, projectID, branchRef string) (hubapi.ForBranch, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ForBranch", projectID, branchRef)
	if err != nil {
		return hubapi.ForBranch{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.ForBranch{}, err
	}
	if !branchRefOK(branchRef) {
		return hubapi.ForBranch{}, badBranchRef()
	}
	var out hubapi.ForBranch
	if b := w.liveBrief(p.id, hubapi.ScopeBranch, branchRef); b != nil {
		bw := w.withRevision(b)
		out.BranchBrief = &bw
	}
	if b := w.liveBrief(p.id, hubapi.ScopeProject, ""); b != nil {
		bw := w.withRevision(b)
		out.ProjectBrief = &bw
	}
	return out, nil
}

// Brief reads one brief by id, archived ones included.
func (f *Fake) Brief(_ context.Context, briefID string) (hubapi.BriefWithRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Brief", briefID)
	if err != nil {
		return hubapi.BriefWithRevision{}, err
	}
	b, _, err := w.briefOf(briefID, me)
	if err != nil {
		return hubapi.BriefWithRevision{}, err
	}
	return w.withRevision(b), nil
}

// Rebind moves a branch brief to a renamed ref and writes no revision: the next
// revision's envelope carries the new ref; earlier ones keep theirs.
func (f *Fake) Rebind(_ context.Context, briefID, branchRef string) (hubapi.BriefHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Rebind", briefID, branchRef)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	b, mine, err := w.briefOf(briefID, me)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.BriefHead{}, err
	}
	if b.scope == hubapi.ScopeProject {
		return hubapi.BriefHead{}, badRequest("branchRef", "A project brief has no branch to rebind.")
	}
	if !branchRefOK(branchRef) {
		return hubapi.BriefHead{}, badBranchRef()
	}
	if b.archivedAt != "" {
		return hubapi.BriefHead{}, briefArchived(b.id)
	}
	if live := w.liveBrief(b.projectID, hubapi.ScopeBranch, branchRef); live != nil && live != b {
		return hubapi.BriefHead{}, problem(409, hubapi.CodeBriefExists, "A live brief already has that branch.", "briefId", live.id)
	}
	if b.branchRef != branchRef {
		b.branchRef, b.updatedAt = branchRef, w.stamp()
	}
	return w.renderHead(b), nil
}

// Archive frees the ref and touches no revision; there is no un-archive.
func (f *Fake) Archive(_ context.Context, briefID string) (hubapi.BriefHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Archive", briefID)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	b, mine, err := w.briefOf(briefID, me)
	if err != nil {
		return hubapi.BriefHead{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.BriefHead{}, err
	}
	if b.archivedAt != "" {
		return hubapi.BriefHead{}, briefArchived(b.id)
	}
	now := w.stamp()
	b.archivedAt, b.updatedAt = now, now
	return w.renderHead(b), nil
}

// Revisions is the history, oldest first, without content; the cursor is the
// last revision number in decimal (task 19: after=1&limit=1 is [2]).
func (f *Fake) Revisions(_ context.Context, briefID string, page hubapi.PageRequest) (hubapi.Page[hubapi.RevisionSummary], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Revisions", briefID, page)
	if err != nil {
		return hubapi.Page[hubapi.RevisionSummary]{}, err
	}
	b, _, err := w.briefOf(briefID, me)
	if err != nil {
		return hubapi.Page[hubapi.RevisionSummary]{}, err
	}
	limit, err := pageLimit(page.Limit)
	if err != nil {
		return hubapi.Page[hubapi.RevisionSummary]{}, err
	}
	after := 0
	if page.After != "" {
		n, err := strconv.Atoi(page.After)
		if err != nil || n < 0 || strconv.Itoa(n) != page.After {
			return hubapi.Page[hubapi.RevisionSummary]{}, badRequest("after", "That cursor is not one this list gave.")
		}
		after = min(n, len(b.revisions))
	}
	summary := func(r *fakeRevision) hubapi.RevisionSummary {
		s := hubapi.RevisionSummary{Key: hubapi.BriefKey{BriefID: b.id, Revision: r.revision}.String(), Revision: r.revision,
			Digest: r.digest, Status: r.status, SavedAt: r.savedAt, SavedBy: w.users[r.savedBy]}
		if r.erasedAt != "" {
			at := r.erasedAt
			s.ErasedAt = &at
		}
		return s
	}
	cursor := func(r *fakeRevision) string { return strconv.Itoa(r.revision) }
	return pageOf(b.revisions[after:], limit, cursor, summary), nil
}

// ── revisions (§7.5, task 19) ───────────────────────────────────────────────

// fieldAt is prefix + "." + err's hubapi.FieldError path, or prefix alone.
func fieldAt(prefix string, err error) string {
	var fe *hubapi.FieldError
	if errors.As(err, &fe) && fe.Field != "" {
		return prefix + "." + fe.Field
	}
	return prefix
}

func badItem(field string, err error) error {
	return badRequest(field, field+": "+strings.TrimPrefix(err.Error(), "hubapi: "))
}

// WriteRevisions is route 26, in task 19's order, all or nothing: 1–64 items
// with distinct brief ids; member of the project; every content valid; every
// brief this project's (404) and live (409); every base its head, checked in
// brief-id order (409 revision_conflict naming the first); every envelope built
// and digested. Only then does any head move. Revisions answer in request order.
func (f *Fake) WriteRevisions(_ context.Context, projectID string, writes []hubapi.RevisionWrite) ([]hubapi.BriefRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("WriteRevisions", projectID, writes)
	if err != nil {
		return nil, err
	}
	if len(writes) < 1 || len(writes) > hubapi.MaxRevisionBatch {
		return nil, badRequest("revisions", "A batch is 1 to "+strconv.Itoa(hubapi.MaxRevisionBatch)+" revisions.")
	}
	seen := map[string]bool{}
	for _, wr := range writes {
		if seen[wr.BriefID] {
			return nil, badRequest("revisions", "Each brief appears once in a batch.")
		}
		seen[wr.BriefID] = true
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return nil, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return nil, err
	}
	contents, statuses := make([][]byte, len(writes)), make([]string, len(writes))
	for i, wr := range writes {
		if contents[i], statuses[i], err = hubapi.ValidateContent(wr.Content); err != nil {
			return nil, badItem(fieldAt("revisions["+strconv.Itoa(i)+"].content", err), err)
		}
	}
	// The heads, in brief-id order: every one this project's, then every one live,
	// then every base its head.
	order := make([]int, len(writes))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(writes[a].BriefID, writes[b].BriefID) })
	briefs := make([]*fakeBrief, len(writes))
	for _, i := range order {
		if briefs[i] = w.briefs[writes[i].BriefID]; briefs[i] == nil || briefs[i].projectID != p.id {
			return nil, briefNotFound(writes[i].BriefID)
		}
	}
	for _, i := range order {
		if briefs[i].archivedAt != "" {
			return nil, briefArchived(briefs[i].id)
		}
	}
	for _, i := range order {
		if head := len(briefs[i].revisions); writes[i].BaseRevision != head {
			return nil, problem(409, hubapi.CodeRevisionConflict, "Someone saved this brief first; read it again and retry.",
				"briefId", briefs[i].id, "headRevision", strconv.Itoa(head))
		}
	}
	revs := make([]*fakeRevision, len(writes))
	now := w.stamp()
	for i, b := range briefs {
		env := hubapi.BriefEnvelope{BriefID: b.id, Revision: writes[i].BaseRevision + 1, ProjectID: p.id,
			Scope: b.scope, BranchRef: b.branchRef, Content: contents[i]}
		digest, canonical, err := env.Digest()
		if err != nil {
			return nil, badItem(fieldAt("revisions["+strconv.Itoa(i)+"]", err), err)
		}
		revs[i] = &fakeRevision{revision: env.Revision, scope: b.scope, branchRef: b.branchRef, digest: digest,
			status: statuses[i], canonical: canonical, savedAt: now, savedBy: me}
	}
	out := make([]hubapi.BriefRevision, len(writes))
	for i, b := range briefs {
		b.revisions = append(b.revisions, revs[i])
		if revs[i].status == hubapi.StatusComplete {
			b.latestComplete = revs[i].revision
		}
		b.updatedAt = now
		out[i] = w.renderRevision(b, revs[i])
	}
	return out, nil
}

// ── the resolver, brief half (§7.5, task 20) ────────────────────────────────

// Resolve answers one result per key, in order, repeats in each slot. A key
// naming nothing, or anything the caller cannot read, is missing — the two are
// indistinguishable on purpose. Design keys are missing until task 05. heads
// holds every brief named by a resolved or erased key. The fake re-checks no
// digest on read: nothing but the fake writes its bytes, so none can drift.
func (f *Fake) Resolve(_ context.Context, keys []string) (hubapi.RefsResponse, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Resolve", keys)
	if err != nil {
		return hubapi.RefsResponse{}, err
	}
	out := hubapi.RefsResponse{Results: make([]hubapi.RefResult, 0, len(keys)), Heads: map[string]hubapi.HeadInfo{}}
	for _, key := range keys {
		kind, err := hubapi.ParseKey(key)
		if err != nil {
			out.Results = append(out.Results, hubapi.RefResult{Key: key, Resolution: hubapi.Invalid})
			continue
		}
		res := hubapi.RefResult{Key: key, Kind: &kind, Resolution: hubapi.Missing}
		if kind == hubapi.KeyBrief {
			k, _ := hubapi.ParseBriefKey(key)
			if b, _, err := w.briefOf(k.BriefID, me); err == nil && k.Revision <= len(b.revisions) {
				r := b.revisions[k.Revision-1]
				rev := w.renderRevision(b, r)
				res.Revision, res.Resolution = &rev, hubapi.Resolved
				if r.erasedAt != "" {
					res.Resolution = hubapi.Erased
				}
				out.Heads[b.id] = hubapi.HeadInfo{HeadRevision: len(b.revisions),
					LatestCompleteRevision: b.latestComplete, Archived: b.archivedAt != ""}
			}
		}
		out.Results = append(out.Results, res)
	}
	return out, nil
}

// ── erasure (§7.12) ─────────────────────────────────────────────────────────

// EraseBrief is the org-admin erasure (contract §7.12), for tests of the erased
// state. It needs a view whose FakeUser is OrgAdmin (else 403 forbidden,
// required org_admin, as §5.3's matrix says), a reason of 1–500 characters, and
// a brief of the org, readable by its teams or not. Every revision loses its
// content and gains erasedAt; key, digest, status, savedAt and savedBy stay;
// the head is archived if it was not. A second erase erases 0.
func (f *Fake) EraseBrief(briefID, reason string) (hubapi.EraseResponse, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := f.enter("EraseBrief", briefID, reason); err != nil {
		return hubapi.EraseResponse{}, err
	}
	if !f.as.OrgAdmin {
		return hubapi.EraseResponse{}, forbidden("org_admin")
	}
	if n := utf8.RuneCountInString(reason); n < 1 || n > 500 {
		return hubapi.EraseResponse{}, badRequest("reason", "A reason is 1 to 500 characters.")
	}
	b := w.briefs[briefID]
	if b == nil {
		return hubapi.EraseResponse{}, briefNotFound(briefID)
	}
	now, erased := w.stamp(), 0
	for _, r := range b.revisions {
		if r.erasedAt == "" {
			r.canonical, r.erasedAt = nil, now
			erased++
		}
	}
	if b.archivedAt == "" {
		b.archivedAt, b.updatedAt = now, now
	}
	return hubapi.EraseResponse{BriefID: b.id, ErasedRevisions: erased, ErasedAt: now}, nil
}
