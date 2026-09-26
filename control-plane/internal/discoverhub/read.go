package discoverhub

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// Target is one scope, by LOCAL ids. BranchID "" is the project scope.
type Target struct {
	ProjectID string
	BranchID  string
}

type BriefState struct {
	ID                     string              `json:"id"`
	Scope                  discover.Scope      `json:"scope"`
	HeadRevision           int                 `json:"headRevision"` // 0: created, nothing answered
	HeadKey                string              `json:"headKey"`      // "" while HeadRevision is 0
	Status                 discover.Status     `json:"status"`
	LatestCompleteRevision int                 `json:"latestCompleteRevision"`
	LatestCompleteKey      string              `json:"latestCompleteKey"` // "" when none
	Questions              []discover.Question `json:"questions"`
	Answers                []discover.Answer   `json:"answers"`
	Inherits               *discover.Inherit   `json:"inherits"`
	Change                 discover.Change     `json:"change"`
	SavedAt                string              `json:"savedAt"`
	SavedBy                string              `json:"savedBy"`
}

// InheritedState is, for a branch, the project revision a session on it
// would inherit through right now: the project's latest complete revision.
type InheritedState struct {
	Key       string              `json:"key"`
	BriefID   string              `json:"briefId"`
	Revision  int                 `json:"revision"`
	Questions []discover.Question `json:"questions"`
	Answers   []discover.Answer   `json:"answers"`
}

// Preview is the managed block a session started now would get.
type Preview struct {
	Key   string `json:"key"`
	Block string `json:"block"` // claudemd.Render output, verbatim
}

type View struct {
	Scope     discover.Scope  `json:"scope"`
	ProjectID string          `json:"projectId"` // local
	BranchID  string          `json:"branchId"`  // local; "" for project scope
	Binding   Binding         `json:"binding"`
	Brief     *BriefState     `json:"brief"`     // nil: no brief at this scope yet
	Inherited *InheritedState `json:"inherited"` // branch scope with a complete project brief; else nil
	Step      discover.Step   `json:"step"`      // over Brief, or over NewContent() when Brief is nil
	Preview   *Preview        `json:"preview"`   // nil: a session started now inherits nothing
}

type HistoryRow struct {
	Key      string          `json:"key"`
	Revision int             `json:"revision"`
	Status   string          `json:"status"`
	Change   discover.Change `json:"change"` // from the revision's content; zero for an erased one
	SavedAt  string          `json:"savedAt"`
	SavedBy  string          `json:"savedBy"`
	Erased   bool            `json:"erased"`
}

type History struct {
	Rows []HistoryRow `json:"rows"` // oldest first
	Next string       `json:"next"` // "" when there is no more
}

// Inheritable is P4-A, the Materialiser's one question: the latest complete
// branch revision, else the latest complete project revision, else none.
type Inheritable struct {
	Key      string            // "" when none
	Document discover.Document // what claudemd.Render writes; zero when none
	// PinErased: Key is a branch revision whose pinned project revision is
	// erased, so Document holds the branch's own text alone (rule 7). Task 09's
	// Inspect reads it, so a block written before the erasure is not called
	// edited (re-review b06-2 R2-I1; task 07 as built).
	PinErased bool
}

// ── conversions ─────────────────────────────────────────────────────────────

// toPiece is a resolved Hub revision as this package holds it. An erased
// revision, or content this build cannot decode, is not a piece.
func toPiece(r *hubapi.BriefRevision) (piece, error) {
	if r == nil || r.ErasedAt != nil || r.Content == nil {
		return piece{}, errors.New("discoverhub: the Hub answered a revision with no content")
	}
	c, err := discover.Decode(r.Content)
	if err != nil {
		return piece{}, err
	}
	return piece{
		rev: discover.Revision{BriefID: r.BriefID, Number: r.Revision, Key: r.Key, Scope: discover.Scope(r.Scope),
			BranchRef: r.BranchRef, Content: c},
		digest: r.Digest.String(), projectID: r.ProjectID, savedAt: r.SavedAt, savedBy: r.SavedBy.DisplayName,
	}, nil
}

func sourceOf(p piece) provenance.Source {
	return provenance.Source{Revision: p.rev, Digest: p.digest, ProjectID: p.projectID, SavedAt: p.savedAt, SavedBy: p.savedBy}
}

func headOf(h hubapi.BriefHead) provenance.Head {
	return provenance.Head{HeadRevision: h.HeadRevision, LatestCompleteRevision: h.LatestCompleteRevision, Archived: h.ArchivedAt != nil}
}

// pinOf is the brief key a branch revision pins, or "" when it pins none or
// pins something that is not a brief key (Build refuses that source).
func pinOf(p piece) string {
	in := p.rev.Content.Inherits
	if p.rev.Scope != discover.ScopeBranch || in == nil {
		return ""
	}
	if k, err := provenance.Parse(in.Key); err != nil || k.Kind != provenance.KindBrief {
		return ""
	}
	return in.Key
}

// refs is one /v1/refs call, or ErrHubOff with no Hub.
func (s *Service) refs(ctx context.Context, keys []string) (hubapi.RefsResponse, error) {
	if s.cfg.Hub == nil {
		return hubapi.RefsResponse{}, ErrHubOff
	}
	return s.cfg.Hub.Resolve(ctx, keys)
}

// ── Resolve (rules 4, 5, 7, 8) ──────────────────────────────────────────────

// resolveRun is one Resolve call's state: what the Hub answered, and the
// pieces in hand, cached or answered, never erased or missing.
type resolveRun struct {
	s       *Service
	answers map[string]hubapi.RefResult
	have    map[string]piece
	heads   map[string]provenance.Head
	asked   map[string]bool
}

// call sends keys to the Hub once. On success every answer is recorded, the
// cache follows the Hub (rule 5: resolved leaves a live entry as it is,
// erased and missing evict it) and heads takes every entry the answer
// carried. On failure nothing is recorded and false is returned.
func (r *resolveRun) call(ctx context.Context, keys []string) bool {
	if len(keys) == 0 {
		return true
	}
	for _, k := range keys {
		r.asked[k] = true
	}
	resp, err := r.s.refs(ctx, keys)
	if err != nil {
		r.s.cfg.Log.Warn("discoverhub: the Hub did not answer a refs call", "keys", len(keys), "err", err)
		return false
	}
	now := r.s.cfg.Now()
	for _, res := range resp.Results {
		r.answers[res.Key] = res
		switch res.Resolution {
		case hubapi.Resolved:
			if _, ok := r.have[res.Key]; ok {
				continue // cached: same bytes, same expiry
			}
			if res.Kind == nil || *res.Kind != hubapi.KeyBrief {
				continue
			}
			p, err := toPiece(res.Revision)
			if err != nil {
				r.s.cfg.Log.Warn("discoverhub: a resolved revision does not decode", "key", res.Key, "err", err)
				continue
			}
			r.s.cache.put(res.Key, p, now, r.s.cfg.CacheTTL)
			r.have[res.Key] = p
		case hubapi.Erased, hubapi.Missing:
			r.s.cache.evict(res.Key)
			delete(r.have, res.Key)
		}
	}
	for id, h := range resp.Heads {
		r.heads[id] = provenance.Head{HeadRevision: h.HeadRevision, LatestCompleteRevision: h.LatestCompleteRevision, Archived: h.Archived}
	}
	return true
}

// Resolve answers every key, in order, in at most two /v1/refs calls, plus,
// only when Config.Evaluate is set, one List walk per Hub project that a
// resolved branch slot names (rule 8). The first /v1/refs call carries
// EVERY requested brief key, cached or not, and the pin key (Inherits.Key)
// of every cached branch revision among them, whether or not the pin is
// still cached. The second carries the pins of the other branch revisions the
// first call returned, cached or not. So heads names every brief a requested
// key or its pin names (Hub contract §7.5): content is immutable, heads are
// not, and a cache hit must not cost the desktop its head (re-review b06-1
// R-I1). A cached piece's content comes from the cache; the cache still saves
// the pin's bytes, and keeps a slot resolved while the Hub fails (rule 5).
// "", none:declined and invalid keys never reach the Hub. A design key rides
// the first call and gets the Hub's resolution with phase 4's fields empty
// (task 12's "Does NOT claim"); it is never cached.
//
// heads holds one entry per brief the Hub's answers named. states holds
// provenance.Readings for every resolved slot whose readings were taken, and
// nothing for any other slot: at is "" for a project key, which Resolve never
// evaluates, and for a branch key the branch its brief is bound to now, read
// by rule 8's List (never the revision's envelope BranchRef; ask HA4-13); an
// archived brief gets at "" and is not evaluated; a branch slot whose
// binding the walk could not read gets NO states entry (task 06 fix round 2).
// Criterion state moves, so it is computed on every call and is never part
// of the cached Ref (review I14).
//
// There is no error result: a Hub failure, a signed-out user or a nil Hub
// makes each slot the cache cannot answer "unavailable" (rule 5, review I6).
func (s *Service) Resolve(ctx context.Context, keys []string) (refs []provenance.Ref, heads map[string]provenance.Head, states map[string][]provenance.CriterionReading) {
	refs = make([]provenance.Ref, len(keys))
	states = map[string][]provenance.CriterionReading{}
	r := &resolveRun{s: s, answers: map[string]hubapi.RefResult{}, have: map[string]piece{},
		heads: map[string]provenance.Head{}, asked: map[string]bool{}}
	now := s.cfg.Now()

	kinds := make([]provenance.Kind, len(keys))
	var first []string
	add := func(list *[]string, k string) {
		if !r.asked[k] {
			r.asked[k] = true
			*list = append(*list, k)
		}
	}
	for i, k := range keys {
		if k == "" {
			refs[i] = provenance.NoneRef("")
			continue
		}
		pk, err := provenance.Parse(k)
		switch {
		case err != nil:
			refs[i] = provenance.InvalidRef(k)
			continue
		case pk.Kind == provenance.KindNone:
			refs[i] = provenance.NoneRef(k)
			continue
		}
		kinds[i] = pk.Kind
		add(&first, k)
		if pk.Kind != provenance.KindBrief {
			continue
		}
		if p, ok := s.cache.get(k, now); ok {
			r.have[k] = p
			if pin := pinOf(p); pin != "" {
				add(&first, pin)
				if pp, ok := s.cache.get(pin, now); ok {
					r.have[pin] = pp
				}
			}
		}
	}
	cachedAtStart := make(map[string]bool, len(r.have))
	for k := range r.have {
		cachedAtStart[k] = true
	}

	if r.call(ctx, first) {
		var second []string
		for i, k := range keys {
			if kinds[i] != provenance.KindBrief || cachedAtStart[k] {
				continue
			}
			if p, ok := r.have[k]; ok {
				if pin := pinOf(p); pin != "" {
					add(&second, pin)
					if pp, ok := s.cache.get(pin, now); ok {
						if _, answered := r.answers[pin]; !answered {
							r.have[pin] = pp
						}
					}
				}
			}
		}
		r.call(ctx, second)
	}

	srcs := map[string]provenance.Source{}
	for i, k := range keys {
		switch kinds[i] {
		case "":
			continue
		case provenance.KindDesign:
			refs[i] = r.designRef(k)
			continue
		}
		ref, src, ok := r.slot(k)
		refs[i] = ref
		if ok {
			srcs[k] = src
		}
	}
	s.readings(ctx, keys, srcs, states)
	return refs, r.heads, states
}

// slot is one brief key's Ref (rule 5). missing and erased come only from
// the Hub's answer; resolved needs every piece in hand, the pin included
// (rule 3), or the pin answered erased (rule 7); anything else is
// unavailable. ok reports a resolved slot, with the Source it was built from.
func (r *resolveRun) slot(k string) (provenance.Ref, provenance.Source, bool) {
	if a, answered := r.answers[k]; answered {
		switch a.Resolution {
		case hubapi.Missing:
			return provenance.MissingRef(k), provenance.Source{}, false
		case hubapi.Erased:
			return r.erasedRef(k, a.Revision), provenance.Source{}, false
		case hubapi.Invalid:
			return provenance.InvalidRef(k), provenance.Source{}, false
		}
	}
	p, ok := r.have[k]
	if !ok {
		return provenance.UnavailableRef(k), provenance.Source{}, false
	}
	src := sourceOf(p)
	if pin := pinOf(p); pin != "" {
		if a, answered := r.answers[pin]; answered && a.Resolution == hubapi.Erased {
			src.PinErased = true
		} else if pp, ok := r.have[pin]; ok {
			src.Project = &pp.rev
		} else {
			return provenance.UnavailableRef(k), provenance.Source{}, false
		}
	}
	ref, err := provenance.Build(k, src, r.heads[p.rev.BriefID])
	if err != nil {
		r.s.cfg.Log.Error("discoverhub: a Hub revision does not build", "key", k, "err", err)
		return ref, provenance.Source{}, false
	}
	return ref, src, true
}

// erasedRef is an erased key's Ref: what the Hub still says of it, or the
// key's own id and revision when the answer carries no revision.
func (r *resolveRun) erasedRef(k string, rev *hubapi.BriefRevision) provenance.Ref {
	if rev == nil {
		pk, _ := provenance.Parse(k)
		return provenance.ErasedRef(k, pk.ID, pk.Revision, "", "", "", "", r.heads[pk.ID])
	}
	erasedAt := ""
	if rev.ErasedAt != nil {
		erasedAt = *rev.ErasedAt
	}
	return provenance.ErasedRef(k, rev.BriefID, rev.Revision, rev.Digest.String(), rev.SavedAt, rev.SavedBy.DisplayName,
		erasedAt, r.heads[rev.BriefID])
}

// designRef passes a design key's resolution through with phase 4's fields
// empty; phase 5 adds its own (task 12's "Does NOT claim", S4-P5.1).
func (r *resolveRun) designRef(k string) provenance.Ref {
	a, answered := r.answers[k]
	if !answered {
		return provenance.UnavailableRef(k)
	}
	switch a.Resolution {
	case hubapi.Resolved, hubapi.Erased:
		ref := provenance.MissingRef(k)
		ref.Resolution, ref.Present = provenance.Resolution(a.Resolution), true
		return ref
	case hubapi.Invalid:
		return provenance.InvalidRef(k)
	}
	return provenance.MissingRef(k)
}

// readings fills states for the resolved slots (rule 8). A project key is
// read at "" with no Evaluate: Resolve never evaluates one. With
// Config.Evaluate nil, a branch key is read the same way, all open, and no
// List is made. Otherwise each distinct Hub project among the resolved
// branch slots gets one List walk; a walk that fails leaves that project's
// branch slots with no entry, and a brief the walk does not list (archived)
// is read at "".
func (s *Service) readings(ctx context.Context, keys []string, srcs map[string]provenance.Source, states map[string][]provenance.CriterionReading) {
	bound := map[string]map[string]string{} // Hub project -> brief id -> the ref it is bound to now
	failed := map[string]bool{}
	for _, k := range keys {
		src, ok := srcs[k]
		if !ok || src.Revision.Scope != discover.ScopeBranch || s.cfg.Evaluate == nil {
			continue
		}
		if _, done := bound[src.ProjectID]; done || failed[src.ProjectID] {
			continue
		}
		m, err := s.branchRefs(ctx, src.ProjectID)
		if err != nil {
			s.cfg.Log.Warn("discoverhub: the branch briefs could not be listed; their readings are not taken", "err", err)
			failed[src.ProjectID] = true
			continue
		}
		bound[src.ProjectID] = m
	}
	for _, k := range keys {
		src, ok := srcs[k]
		if !ok {
			continue
		}
		if _, done := states[k]; done {
			continue
		}
		switch {
		case src.Revision.Scope != discover.ScopeBranch || s.cfg.Evaluate == nil:
			states[k] = provenance.Readings(ctx, k, src, "", nil)
		case failed[src.ProjectID]:
			// binding unknown: not a reading (task 06 fix round 2, R1-I1)
		default:
			states[k] = provenance.Readings(ctx, k, src, bound[src.ProjectID][src.Revision.BriefID], s.cfg.Evaluate)
		}
	}
}

// branchRefs walks List over projectID's live branch briefs, 200 a page,
// following next until it is null, and maps each brief id to the ref it is
// bound to now. A failure on any page fails the whole walk: a partial map
// would call a brief on a later page archived. The answer is not cached.
func (s *Service) branchRefs(ctx context.Context, projectID string) (map[string]string, error) {
	out := map[string]string{}
	q := hubapi.BriefQuery{Scope: hubapi.ScopeBranch, Page: hubapi.PageRequest{Limit: 200}}
	for {
		page, err := s.cfg.Hub.List(ctx, projectID, q)
		if err != nil {
			return nil, err
		}
		for _, h := range page.Items {
			if h.ArchivedAt == nil {
				out[h.ID] = h.BranchRef
			}
		}
		if page.Next == nil {
			return out, nil
		}
		if *page.Next == "" || *page.Next == q.Page.After {
			return nil, errors.New("discoverhub: the Hub's brief list did not advance")
		}
		q.Page.After = *page.Next
	}
}

// ── what a target inherits (rules 3, 6, 7) ──────────────────────────────────

// errUnknownBranch: a Target names a local branch the Locator does not know.
var errUnknownBranch = errors.New("discoverhub: no such local branch")

// bound is projectID's Hub project: ErrNotBound, ErrNoRemote, or the error
// Binding returned, unchanged. The Hub is configured.
func (s *Service) bound(ctx context.Context, projectID string) (Binding, error) {
	b, err := s.Binding(ctx, projectID)
	if err != nil {
		return Binding{}, err
	}
	switch b.State {
	case BindingBound:
		return b, nil
	case BindingNoRemote:
		return b, ErrNoRemote
	case BindingHubOff:
		return b, ErrHubOff
	}
	return b, ErrNotBound
}

// briefsAt reads t's briefs, mutable heads, never cached: ForBranch for a
// branch target, once; for the project scope, the live project brief (List,
// then Brief).
func (s *Service) briefsAt(ctx context.Context, hubProject string, t Target) (branch, project *hubapi.BriefWithRevision, err error) {
	if t.BranchID != "" {
		ref := s.BranchRef(t.BranchID)
		if ref == "" {
			return nil, nil, errUnknownBranch
		}
		fb, err := s.cfg.Hub.ForBranch(ctx, hubProject, ref)
		if err != nil {
			return nil, nil, err
		}
		return fb.BranchBrief, fb.ProjectBrief, nil
	}
	page, err := s.cfg.Hub.List(ctx, hubProject, hubapi.BriefQuery{Scope: hubapi.ScopeProject, Page: hubapi.PageRequest{Limit: 1}})
	if err != nil || len(page.Items) == 0 {
		return nil, nil, err
	}
	bw, err := s.cfg.Hub.Brief(ctx, page.Items[0].ID)
	if err != nil {
		return nil, nil, err
	}
	return nil, &bw, nil
}

// inheritance is what a session inherits: the key, what Build needs, and the
// document Render writes.
type inheritance struct {
	key  string
	src  provenance.Source
	head provenance.Head
	doc  discover.Document
}

// latestComplete is bw's latest complete revision, when it has one that is
// not erased (HA4-4: BriefWithRevision carries it, so no /v1/refs call).
func latestComplete(bw *hubapi.BriefWithRevision) *hubapi.BriefRevision {
	if bw == nil || bw.LatestComplete == nil || bw.LatestComplete.ErasedAt != nil {
		return nil
	}
	return bw.LatestComplete
}

// inheritFrom is P4-A over what briefsAt read: the branch's latest complete
// revision, resolved against the project revision it pins, fetched by its
// inherits.key and never by the project's current head (rule 3); else the
// project's latest complete revision; else none (ok false).
func (s *Service) inheritFrom(ctx context.Context, branch, project *hubapi.BriefWithRevision) (inheritance, bool, error) {
	if lc := latestComplete(branch); lc != nil {
		p, err := toPiece(lc)
		if err != nil {
			return inheritance{}, false, err
		}
		src := sourceOf(p)
		if pin := pinOf(p); pin != "" {
			pp, erased, err := s.pin(ctx, pin)
			switch {
			case err != nil:
				return inheritance{}, false, err
			case erased:
				src.PinErased = true
			default:
				src.Project = &pp.rev
			}
		}
		doc, err := documentOf(src)
		if err != nil {
			return inheritance{}, false, err
		}
		return inheritance{key: lc.Key, src: src, head: headOf(branch.Head), doc: doc}, true, nil
	}
	if lc := latestComplete(project); lc != nil {
		p, err := toPiece(lc)
		if err != nil {
			return inheritance{}, false, err
		}
		doc, err := discover.Resolve(&p.rev, nil)
		if err != nil {
			return inheritance{}, false, err
		}
		return inheritance{key: lc.Key, src: sourceOf(p), head: headOf(project.Head), doc: doc}, true, nil
	}
	return inheritance{}, false, nil
}

// documentOf is the document a branch source names: resolved against its
// pin, or its own text alone when the pin is erased (rule 7), exactly as
// provenance.Build resolves it.
func documentOf(src provenance.Source) (discover.Document, error) {
	if src.PinErased {
		own := src.Revision
		own.Content.Inherits = nil
		return discover.Resolve(nil, &own)
	}
	return discover.Resolve(src.Project, &src.Revision)
}

// pin is one pinned project revision, by its key, in one /v1/refs call. The
// Hub's answer wins over the cache (rule 5): resolved takes the cached bytes
// when they are held, erased and missing evict them. erased is rule 7's
// case; missing is ErrPinUnreadable; a Hub error is returned unchanged.
func (s *Service) pin(ctx context.Context, key string) (piece, bool, error) {
	resp, err := s.refs(ctx, []string{key})
	if err != nil {
		return piece{}, false, err
	}
	now := s.cfg.Now()
	for _, res := range resp.Results {
		if res.Key != key {
			continue
		}
		switch res.Resolution {
		case hubapi.Resolved:
			if p, ok := s.cache.get(key, now); ok {
				return p, false, nil
			}
			p, err := toPiece(res.Revision)
			if err != nil {
				return piece{}, false, err
			}
			s.cache.put(key, p, now, s.cfg.CacheTTL)
			return p, false, nil
		case hubapi.Erased:
			s.cache.evict(key)
			return piece{}, true, nil
		}
		s.cache.evict(key)
		return piece{}, false, ErrPinUnreadable
	}
	return piece{}, false, ErrPinUnreadable
}

// Inheritable is the Materialiser's one question (P4-A). A nil Hub answers
// none with no error (rule 6; re-review b05-2 N-c); an unbound project is
// ErrNotBound and one with no recognised remote ErrNoRemote, which task 09
// maps to "nothing to inherit"; a Hub error is returned unchanged.
func (s *Service) Inheritable(ctx context.Context, t Target) (Inheritable, error) {
	if s.cfg.Hub == nil {
		return Inheritable{}, nil
	}
	b, err := s.bound(ctx, t.ProjectID)
	if err != nil {
		return Inheritable{}, err
	}
	branch, project, err := s.briefsAt(ctx, b.HubProjectID, t)
	if err != nil {
		return Inheritable{}, err
	}
	in, ok, err := s.inheritFrom(ctx, branch, project)
	if err != nil || !ok {
		return Inheritable{}, err
	}
	return Inheritable{Key: in.key, Document: in.doc, PinErased: in.src.PinErased}, nil
}

// HeadRef is the Ref of what a session on t would inherit now - the head
// reading for DefinitionOfDone and Discover (contract §3.1). resolution
// "none" when nothing would be inherited: with a nil Hub (rule 6) and, as
// built, for a project that is not shared (unbound or no_remote), where a
// session started now inherits nothing too - none of the three is an error.
// states is provenance.Readings(ctx, ref.Key, src, <t's branch ref>,
// Config.Evaluate), a project key inherited by t included, when the Ref is
// resolved, and nil otherwise, computed on every call (review I14). Every
// Hub error is returned unchanged.
func (s *Service) HeadRef(ctx context.Context, t Target) (provenance.Ref, []provenance.CriterionReading, error) {
	if s.cfg.Hub == nil {
		return provenance.NoneRef(""), nil, nil
	}
	b, err := s.bound(ctx, t.ProjectID)
	if errors.Is(err, ErrNotBound) || errors.Is(err, ErrNoRemote) {
		return provenance.NoneRef(""), nil, nil
	}
	if err != nil {
		return provenance.Ref{}, nil, err
	}
	branch, project, err := s.briefsAt(ctx, b.HubProjectID, t)
	if err != nil {
		return provenance.Ref{}, nil, err
	}
	in, ok, err := s.inheritFrom(ctx, branch, project)
	if err != nil {
		return provenance.Ref{}, nil, err
	}
	if !ok {
		return provenance.NoneRef(""), nil, nil
	}
	ref, err := provenance.Build(in.key, in.src, in.head)
	if err != nil {
		return ref, nil, err
	}
	return ref, provenance.Readings(ctx, in.key, in.src, s.BranchRef(t.BranchID), s.cfg.Evaluate), nil
}

// ── the Discover view and the history ───────────────────────────────────────

// briefState is a brief's head as the lens shows it, and the content its
// Step walks.
func briefState(bw hubapi.BriefWithRevision) (BriefState, discover.Content, error) {
	h := bw.Head
	st := BriefState{ID: h.ID, Scope: discover.Scope(h.Scope), HeadRevision: h.HeadRevision,
		LatestCompleteRevision: h.LatestCompleteRevision}
	c := discover.NewContent()
	if h.HeadKey != nil {
		st.HeadKey = *h.HeadKey
	}
	if bw.Revision != nil && bw.Revision.ErasedAt == nil {
		dc, err := discover.Decode(bw.Revision.Content)
		if err != nil {
			return BriefState{}, discover.Content{}, err
		}
		c = dc
		st.SavedAt, st.SavedBy = bw.Revision.SavedAt, bw.Revision.SavedBy.DisplayName
	}
	if bw.LatestComplete != nil {
		st.LatestCompleteKey = bw.LatestComplete.Key
	}
	st.Status, st.Questions, st.Answers, st.Inherits, st.Change = c.Status, c.Questions, c.Answers, c.Inherits, c.Change
	return st, c, nil
}

// View is the Discover lens's read of one scope. A binding that is not bound
// (hub_off, unbound, no_remote) is a view with that binding and no brief,
// and no error (rule 6). Every Hub error is returned unchanged.
func (s *Service) View(ctx context.Context, t Target) (View, error) {
	scope := discover.ScopeProject
	if t.BranchID != "" {
		scope = discover.ScopeBranch
	}
	b, err := s.Binding(ctx, t.ProjectID)
	if err != nil {
		return View{}, err
	}
	v := View{Scope: scope, ProjectID: t.ProjectID, BranchID: t.BranchID, Binding: b, Step: discover.NewContent().Step(scope)}
	if b.State != BindingBound {
		return v, nil
	}
	branch, project, err := s.briefsAt(ctx, b.HubProjectID, t)
	if err != nil {
		return View{}, err
	}
	own := project
	if scope == discover.ScopeBranch {
		own = branch
	}
	if own != nil {
		st, c, err := briefState(*own)
		if err != nil {
			return View{}, err
		}
		v.Brief, v.Step = &st, c.Step(scope)
	}
	if lc := latestComplete(project); scope == discover.ScopeBranch && lc != nil {
		c, err := discover.Decode(lc.Content)
		if err != nil {
			return View{}, err
		}
		v.Inherited = &InheritedState{Key: lc.Key, BriefID: lc.BriefID, Revision: lc.Revision, Questions: c.Questions, Answers: c.Answers}
	}
	in, ok, err := s.inheritFrom(ctx, branch, project)
	if err != nil {
		return View{}, err
	}
	if ok {
		v.Preview = &Preview{Key: in.key, Block: claudemd.Render(in.doc)}
	}
	return v, nil
}

// History is a brief's revisions, oldest first, a page at a time. Change
// comes from each revision's own content (rule: no diffing), read from the
// cache or in one /v1/refs call for the rows it does not hold; an erased row
// is marked and has no change. A nil Hub is ErrHubOff; every Hub error is
// returned unchanged.
func (s *Service) History(ctx context.Context, briefID string, after string, limit int) (History, error) {
	if s.cfg.Hub == nil {
		return History{}, ErrHubOff
	}
	page, err := s.cfg.Hub.Revisions(ctx, briefID, hubapi.PageRequest{After: after, Limit: limit})
	if err != nil {
		return History{}, err
	}
	now := s.cfg.Now()
	have := map[string]piece{}
	var need []string
	for _, r := range page.Items {
		switch p, ok := s.cache.get(r.Key, now); {
		case r.ErasedAt != nil:
			s.cache.evict(r.Key)
		case ok:
			have[r.Key] = p
		default:
			need = append(need, r.Key)
		}
	}
	if len(need) > 0 {
		resp, err := s.cfg.Hub.Resolve(ctx, need)
		if err != nil {
			return History{}, err
		}
		for _, res := range resp.Results {
			if res.Resolution != hubapi.Resolved {
				s.cache.evict(res.Key)
				continue
			}
			p, err := toPiece(res.Revision)
			if err != nil {
				s.cfg.Log.Warn("discoverhub: a history revision does not decode", "key", res.Key, "err", err)
				continue
			}
			s.cache.put(res.Key, p, now, s.cfg.CacheTTL)
			have[res.Key] = p
		}
	}
	h := History{Rows: make([]HistoryRow, 0, len(page.Items))}
	for _, r := range page.Items {
		row := HistoryRow{Key: r.Key, Revision: r.Revision, Status: r.Status, SavedAt: r.SavedAt,
			SavedBy: r.SavedBy.DisplayName, Erased: r.ErasedAt != nil}
		if p, ok := have[r.Key]; ok && !row.Erased {
			row.Change = p.rev.Content.Change
		}
		h.Rows = append(h.Rows, row)
	}
	if page.Next != nil {
		h.Next = *page.Next
	}
	return h, nil
}
