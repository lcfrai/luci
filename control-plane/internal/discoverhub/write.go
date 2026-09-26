package discoverhub

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// OpProposeFollowUp is the one op the server expands itself: it asks the
// model (task 04) and, if a question comes back, applies
// discover.OpAddFollowUp. It is never a discover.OpKind, because the domain
// never calls a model.
const OpProposeFollowUp discover.OpKind = "propose_follow_up"

// Request is one change a person asked for, at the head they saw.
type Request struct {
	Target       Target
	BaseRevision int         // the head the person saw; 0 when no brief exists yet
	Op           discover.Op // Kind, Key, Text, Source, RestoreRevision from the wire
}

// Result is the view after the change, and what happened.
type Result struct {
	View    View
	Written []string               // the keys written, this brief's first; [] when nothing changed
	Assist  discover.AssistOutcome // set for OpProposeFollowUp; "" otherwise
}

var (
	ErrDigestDisagreement = errors.New("discoverhub: the Hub's digest is not the one the branch revisions pinned")
	ErrErasedRestore      = errors.New("discoverhub: that revision was erased and cannot be restored")
)

// The Hub's size limits (review M6, plan §4 Security, S4). HA4-3 has landed
// in H0, so they are the Hub's own constants, not local copies:
//   - MaxEnvelopeBytes: a revision's canonical §3.4 envelope
//     (hubapi.MaxEnvelopeBytes, 64 KiB);
//   - MaxBatchBytes: the body of POST …/brief-revisions
//     (hubapi.MaxRevisionBatchBytes, 4 MiB);
//   - MaxBatchItems: the items one batch carries (hubapi.MaxRevisionBatch, 64).
const (
	MaxEnvelopeBytes = hubapi.MaxEnvelopeBytes
	MaxBatchBytes    = hubapi.MaxRevisionBatchBytes
	MaxBatchItems    = hubapi.MaxRevisionBatch
)

// FanoutTooLargeError is a project edit whose batch one Hub call cannot
// carry, for one of three causes: more branch revisions than the batch limit
// (HA4-3); a request body over MaxBatchBytes; or one fan-out item, a branch
// brief's reinherit, whose own envelope is over MaxEnvelopeBytes (b10
// re-review M-2). The third is the branch's text, stored unchecked, not the
// project editor's, so it is never a RevisionTooLargeError that tells the
// editor to shorten their own answer. Nothing is written.
type FanoutTooLargeError struct {
	Needed     int    // branch briefs that would need a new revision
	Limit      int    // the Hub's batch limit minus one
	Bytes      int    // the size that failed: the body's, or BranchRef's envelope; 0 when the count failed
	LimitBytes int    // MaxBatchBytes, or MaxEnvelopeBytes when BranchRef is set; 0 when the count failed
	BranchRef  string // the fan-out item's branchRef when its envelope failed; "" otherwise
}

func (e *FanoutTooLargeError) Error() string {
	switch {
	case e.BranchRef != "":
		return "discoverhub: the branch brief on " + e.BranchRef + " would be " + strconv.Itoa(e.Bytes) +
			" bytes, over the Hub's " + strconv.Itoa(e.LimitBytes) + "-byte revision limit"
	case e.Bytes > 0:
		return "discoverhub: the batch for " + strconv.Itoa(e.Needed) + " branch briefs would be " +
			strconv.Itoa(e.Bytes) + " bytes, over the Hub's " + strconv.Itoa(e.LimitBytes) + "-byte limit"
	}
	return "discoverhub: " + strconv.Itoa(e.Needed) + " branch briefs would need a new revision; one batch carries " +
		strconv.Itoa(e.Limit)
}

// RevisionTooLargeError is a revision whose canonical §3.4 envelope would be
// over MaxEnvelopeBytes: the refusal the Hub would give as a 400 that names
// no answer (review M6), given here first with the op's question key. Task
// 03's caps make it unreachable for ordinary text; this check is the bound,
// because the caps count code points, not bytes: 4,000 CJK code points are
// 12,000 canonical bytes (Hub contract §3.4 writes non-ASCII as UTF-8), so
// six answers plus the done answer's criteria (28,000 code points) reach
// 84,000 bytes, past 65,536. It is only ever this brief's own item: a
// fan-out item over the limit is a *FanoutTooLargeError with BranchRef.
// Nothing is written.
type RevisionTooLargeError struct {
	Key   string // Request.Op.Key; "" for an op with none (restore, draft, propose_follow_up): the brief, not one answer
	Bytes int
	Limit int // MaxEnvelopeBytes
}

func (e *RevisionTooLargeError) Error() string {
	at := "this brief"
	if e.Key != "" {
		at = "the answer to " + e.Key
	}
	return "discoverhub: " + at + " makes a revision of " + strconv.Itoa(e.Bytes) + " bytes, over the Hub's " +
		strconv.Itoa(e.Limit) + "-byte limit"
}

// maxRebuilds bounds step 7's retry: a revision_conflict naming a fan-out
// item re-reads the heads and rebuilds the batch at most this many times.
const maxRebuilds = 3

// Apply writes one change as one batch.
//
//  1. Hub off → ErrHubOff; project not bound → ErrNotBound (ErrNoRemote for
//     a repository with no recognised remote).
//  2. No brief at the target and BaseRevision 0 → CreateBrief, but only once
//     step 3 has a change to write, so an op that writes nothing creates no
//     empty brief (brief_exists from a race re-reads and continues with that
//     brief). No brief and BaseRevision above 0 → ErrNoBrief.
//  3. The head content is read (ForBranch, or the project brief's List and
//     Brief: the head revision rides on the read); the op is applied with
//     discover.Apply and Env.Inherits = the project's latest complete
//     revision now (branch scope only). ErrNoChange → Result with
//     Written = [] and no Hub write.
//  4. OpProposeFollowUp: ProposeFollowUp; anything but AssistProposed →
//     Result with that outcome, no write.
//  5. OpRestore: the target revision's content is read by key; an erased
//     target is ErrErasedRestore, and a missing one ErrNoBrief.
//  6. Project scope, and the new content is complete: every live (not
//     archived) branch brief of the project whose head content pins a
//     different project key gets discover.OpReinherit at its head, whether
//     that head is complete OR OPEN (review I8). An open head keeps its
//     status; only Inherits moves, so the person's edit in progress now
//     carries the new project revision. Their next op meets
//     revision_conflict and re-reads, exactly as on a complete head. A
//     brief with no revision yet is left alone: it has no text to pin, and
//     its first op pins the project revision current then (steps 3 and 9).
//  7. One writeBatch call. Before its one WriteRevisions, writeBatch checks
//     the item count (MaxBatchItems) and then the sizes (review M6, S4):
//     each item's canonical §3.4 envelope, built by the builder the pin
//     uses (digest.go's revisionEnvelope), over MaxEnvelopeBytes →
//     *RevisionTooLargeError{Key: r.Op.Key} for this brief's item, and
//     *FanoutTooLargeError{BranchRef, Bytes, LimitBytes: MaxEnvelopeBytes}
//     for a fan-out item (b10 re-review M-2); then the request body,
//     measured as len(json.Marshal({"revisions": writes})), over
//     MaxBatchBytes → *FanoutTooLargeError{Needed, Limit, Bytes,
//     LimitBytes}. json.Marshal escapes <, > and & where the canonical form
//     does not, so it is never shorter than the canonical bytes; H0 states
//     that hubclient sends content canonical without HTML escaping
//     (hubapi.MaxRevisionBatchBytes), which task 33 has not built, so the
//     measure stays the longer one. Steps 8 and 9 go through the same
//     checks, and a failure there is logged like any other failure in
//     those steps. A revision_conflict naming THIS brief is returned
//     unchanged (the person re-reads). One naming a fan-out item re-reads
//     heads and rebuilds the batch, at most 3 times, then returns the
//     conflict. After the write, the Hub's digest for the project revision
//     must equal the one the fan-out pinned, else ErrDigestDisagreement.
//  8. Project-side post-commit check (steps 6-7 ran): List again, AFTER the
//     batch committed. Any live branch head whose content pins a project
//     revision below the one just written - a brief created, or a branch
//     op committed, between step 6's List and the commit - gets one more
//     fan-out, built by the same fanOut and written by writeBatch with
//     step 7's retry rule. It runs once and never re-lists again, except
//     for that retry's own re-read.
//  9. Branch-side post-commit check (branch scope, a revision was
//     written): read the project's latest complete revision AFTER the
//     write committed. If it is above the pin just written - a project
//     batch committed between step 3 and this write - write one
//     discover.OpReinherit at the new head through writeBatch. A
//     revision_conflict here is dropped: whoever moved the head pinned
//     the project revision current at their own step 3.
//     In steps 8 and 9 a NULL pin (Content.Inherits nil) counts as below
//     every revision, so it is re-pinned like any stale one (re-review
//     b06-1 R-M1), and so does a pin to another project brief.
//     Result.View is read AFTER steps 8 and 9, so it shows the head step 9
//     moved (re-review b06-1 R-M3). A failed step 8 or 9 is logged at warn
//     and does not fail Apply: the person's change is written, and
//     Result.View shows the step-7 head. Result.Written lists only the
//     step-7 batch. A failed View read after a write returns its error with
//     Result.Written set: the change is written, and the same op again is
//     ErrNoChange.
func (s *Service) Apply(ctx context.Context, r Request) (Result, error) {
	if s.cfg.Hub == nil {
		return Result{}, ErrHubOff
	}
	b, err := s.bound(ctx, r.Target.ProjectID)
	if err != nil {
		return Result{}, err
	}
	hp := b.HubProjectID
	scope, ref := discover.ScopeProject, ""
	if r.Target.BranchID != "" {
		scope, ref = discover.ScopeBranch, s.BranchRef(r.Target.BranchID)
	}
	branch, project, err := s.briefsAt(ctx, hp, r.Target)
	if err != nil {
		return Result{}, err
	}
	own := project
	if scope == discover.ScopeBranch {
		own = branch
	}
	if own == nil && r.BaseRevision != 0 {
		return Result{}, ErrNoBrief
	}
	head := discover.NewContent()
	if own != nil && own.Revision != nil {
		p, err := toPiece(own.Revision)
		if err != nil {
			return Result{}, err
		}
		head = p.rev.Content
	}
	env := discover.Env{Scope: scope, Now: s.cfg.Now(), NewID: s.cfg.NewID}
	if lc := latestComplete(project); scope == discover.ScopeBranch && lc != nil {
		env.Inherits = &discover.Inherit{Key: lc.Key, Digest: lc.Digest.String()}
	}

	op := r.Op
	var assist discover.AssistOutcome
	switch op.Kind {
	case OpProposeFollowUp:
		hint := discover.Hint{ProjectName: b.HubProjectName, BranchName: s.cfg.Locate.BranchName(r.Target.BranchID)}
		q, outcome := discover.ProposeFollowUp(ctx, s.cfg.Model, head, scope, hint)
		assist = outcome
		if outcome != discover.AssistProposed {
			return s.result(ctx, r.Target, []string{}, assist)
		}
		op = discover.Op{Kind: discover.OpAddFollowUp, Question: &q}
	case discover.OpRestore:
		if own == nil {
			return Result{}, ErrNoBrief
		}
		c, err := s.restoreContent(ctx, own.Head.ID, op.RestoreRevision)
		if err != nil {
			return Result{}, err
		}
		op.Restore = &c
	}
	next, err := discover.Apply(head, op, env)
	if errors.Is(err, discover.ErrNoChange) {
		return s.result(ctx, r.Target, []string{}, assist)
	}
	if err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return Result{}, err
	}

	var briefID string
	if own == nil {
		if briefID, ref, err = s.createBrief(ctx, hp, r.Target, scope, ref); err != nil {
			return Result{}, err
		}
	} else {
		briefID, ref = own.Head.ID, own.Head.BranchRef
	}
	mine := item{write: hubapi.RevisionWrite{BriefID: briefID, BaseRevision: r.BaseRevision, Content: raw},
		scope: string(scope), branchRef: ref, own: true}
	build := func(context.Context) ([]item, error) { return []item{mine}, nil }

	var pin *discover.Inherit // set when step 6 runs
	if scope == discover.ScopeProject && next.Status == discover.StatusComplete {
		rev := r.BaseRevision + 1
		digest, err := revisionDigest(briefID, rev, hp, hubapi.ScopeProject, "", raw)
		if err != nil {
			return Result{}, err
		}
		pin = &discover.Inherit{Key: provenance.BriefKey(briefID, rev), Digest: digest}
		build = func(ctx context.Context) ([]item, error) {
			heads, err := s.branchHeads(ctx, hp)
			if err != nil {
				return nil, err
			}
			fan, err := fanOut(*pin, heads, env.Now, s.cfg.NewID)
			if err != nil {
				return nil, err
			}
			return append([]item{mine}, fan...), nil
		}
	}
	items, revs, err := s.writeBatch(ctx, hp, string(r.Op.Key), build)
	if err != nil {
		return Result{}, err
	}
	written := make([]string, len(revs))
	for i, rev := range revs {
		written[i] = rev.Key
	}
	if pin != nil {
		if got := revs[0].Digest.String(); got != pin.Digest {
			s.cfg.Log.Error("discoverhub: internal: the Hub's digest for the project revision is not the pinned one",
				"key", pin.Key, "pinned", pin.Digest, "hub", got, "fanOut", len(items)-1)
			if len(items) > 1 {
				return Result{Written: written}, ErrDigestDisagreement
			}
		}
		s.repinProject(ctx, hp, briefID, revs[0])
	}
	if scope == discover.ScopeBranch {
		s.repinBranch(ctx, hp, mine, revs[0], next)
	}
	return s.result(ctx, r.Target, written, assist)
}

// result is Apply's answer: the view read now, after every write.
func (s *Service) result(ctx context.Context, t Target, written []string, assist discover.AssistOutcome) (Result, error) {
	v, err := s.View(ctx, t)
	if err != nil {
		return Result{Written: written, Assist: assist}, err
	}
	return Result{View: v, Written: written, Assist: assist}, nil
}

// createBrief is step 2: the target's brief, created now. brief_exists (a
// teammate created it first) re-reads and continues with the live one; its
// head may have moved past 0, and then the write meets revision_conflict
// and the person re-reads, as for any head they did not see.
func (s *Service) createBrief(ctx context.Context, hp string, t Target, scope discover.Scope, ref string) (id, branchRef string, err error) {
	h, err := s.cfg.Hub.CreateBrief(ctx, hp, hubapi.CreateBriefRequest{Scope: string(scope), BranchRef: ref})
	if err == nil {
		return h.ID, h.BranchRef, nil
	}
	if hubclient.Code(err) != hubapi.CodeBriefExists {
		return "", "", err
	}
	branch, project, rerr := s.briefsAt(ctx, hp, t)
	if rerr != nil {
		return "", "", rerr
	}
	live := project
	if scope == discover.ScopeBranch {
		live = branch
	}
	if live == nil {
		return "", "", err // archived again between the two calls: the Hub's answer stands
	}
	return live.Head.ID, live.Head.BranchRef, nil
}

// restoreContent is step 5: revision n of briefID, read by key (the cache,
// else one /v1/refs call, the Hub's answer winning; task 07's pin read).
func (s *Service) restoreContent(ctx context.Context, briefID string, n int) (discover.Content, error) {
	if n < 1 {
		return discover.Content{}, ErrNoBrief
	}
	p, erased, err := s.pin(ctx, provenance.BriefKey(briefID, n))
	switch {
	case errors.Is(err, ErrPinUnreadable):
		return discover.Content{}, ErrNoBrief
	case err != nil:
		return discover.Content{}, err
	case erased:
		return discover.Content{}, ErrErasedRestore
	}
	return p.rev.Content, nil
}

// item is one revision of a batch, with what the size check needs to build
// its envelope: the scope and ref the Hub will stamp on it.
type item struct {
	write     hubapi.RevisionWrite
	scope     string // hubapi.ScopeProject | hubapi.ScopeBranch
	branchRef string // "" for a project brief
	own       bool   // the brief the person's op is on (steps 7 and 9); false for a fan-out item
}

// branchHead is one branch brief's head as the fan-out reads it.
type branchHead struct {
	briefID   string
	branchRef string
	revision  int // 0: created, nothing written
	archived  bool
	content   discover.Content
}

// branchHeads lists every live branch brief of hp with its head content:
// List (paged), then the heads' revisions from the cache, else in one
// /v1/refs call. A head the Hub no longer answers resolved (erased since
// the List), or content this build cannot read, is left out and logged:
// no reinherit could carry it without losing what it holds.
func (s *Service) branchHeads(ctx context.Context, hp string) ([]branchHead, error) {
	var heads []branchHead
	q := hubapi.BriefQuery{Scope: hubapi.ScopeBranch, Page: hubapi.PageRequest{Limit: 200}}
	for {
		page, err := s.cfg.Hub.List(ctx, hp, q)
		if err != nil {
			return nil, err
		}
		for _, h := range page.Items {
			heads = append(heads, branchHead{briefID: h.ID, branchRef: h.BranchRef, revision: h.HeadRevision,
				archived: h.ArchivedAt != nil})
		}
		if page.Next == nil {
			break
		}
		if *page.Next == "" || *page.Next == q.Page.After {
			return nil, errors.New("discoverhub: the Hub's brief list did not advance")
		}
		q.Page.After = *page.Next
	}
	now := s.cfg.Now()
	have := map[string]discover.Content{}
	var need []string
	for _, h := range heads {
		if h.archived || h.revision == 0 {
			continue
		}
		key := provenance.BriefKey(h.briefID, h.revision)
		if p, ok := s.cache.get(key, now); ok {
			have[key] = p.rev.Content
		} else {
			need = append(need, key)
		}
	}
	if len(need) > 0 {
		resp, err := s.refs(ctx, need)
		if err != nil {
			return nil, err
		}
		for _, res := range resp.Results {
			if res.Resolution != hubapi.Resolved {
				s.cache.evict(res.Key)
				continue
			}
			p, err := toPiece(res.Revision)
			if err != nil {
				s.cfg.Log.Warn("discoverhub: a branch head does not decode; the fan-out leaves it", "key", res.Key, "err", err)
				continue
			}
			s.cache.put(res.Key, p, now, s.cfg.CacheTTL)
			have[res.Key] = p.rev.Content
		}
	}
	out := heads[:0]
	for _, h := range heads {
		if h.archived || h.revision == 0 {
			out = append(out, h) // fanOut drops these itself
			continue
		}
		c, ok := have[provenance.BriefKey(h.briefID, h.revision)]
		if !ok {
			s.cfg.Log.Warn("discoverhub: a branch head was not readable; the fan-out leaves it", "briefId", h.briefID)
			continue
		}
		h.content = c
		out = append(out, h)
	}
	return out, nil
}

// fanOut is the 4-D3 fan-out, a pure function: discover.OpReinherit to pin
// at each head, complete or open alike (review I8). An archived head is
// dropped (nobody inherits it), so is a brief with no revision yet, and so
// is a head that already pins pin (discover.Apply's ErrNoChange). Each item
// is based on the head it was built from.
func fanOut(pin discover.Inherit, heads []branchHead, now time.Time, newID func(string) string) ([]item, error) {
	env := discover.Env{Scope: discover.ScopeBranch, Inherits: &pin, Now: now, NewID: newID}
	var out []item
	for _, h := range heads {
		if h.archived || h.revision == 0 {
			continue
		}
		next, err := discover.Apply(h.content, discover.Op{Kind: discover.OpReinherit}, env)
		if errors.Is(err, discover.ErrNoChange) {
			continue
		}
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		out = append(out, item{write: hubapi.RevisionWrite{BriefID: h.briefID, BaseRevision: h.revision, Content: raw},
			scope: hubapi.ScopeBranch, branchRef: h.branchRef})
	}
	return out, nil
}

// writeBatch holds the package's only WriteRevisions call: steps 7, 8 and 9
// all write through it. build gives the batch; an empty one writes nothing.
// checkBatch runs before every call. A revision_conflict naming a fan-out
// item calls build again, which re-reads the heads, at most maxRebuilds
// times; one naming this brief's own item, or any other error, is returned
// unchanged. key is the op's question key, for RevisionTooLargeError.
func (s *Service) writeBatch(ctx context.Context, hp, key string, build func(context.Context) ([]item, error)) ([]item, []hubapi.BriefRevision, error) {
	for rebuild := 0; ; rebuild++ {
		items, err := build(ctx)
		if err != nil || len(items) == 0 {
			return nil, nil, err
		}
		if err := checkBatch(hp, key, items); err != nil {
			return nil, nil, err
		}
		writes := make([]hubapi.RevisionWrite, len(items))
		for i, it := range items {
			writes[i] = it.write
		}
		revs, err := s.cfg.Hub.WriteRevisions(ctx, hp, writes)
		if err == nil {
			if len(revs) != len(items) {
				return nil, nil, errors.New("discoverhub: the Hub answered a batch with the wrong number of revisions")
			}
			return items, revs, nil
		}
		id, conflict := conflictOn(err)
		if !conflict || rebuild == maxRebuilds || !fanOutItem(items, id) {
			return nil, nil, err
		}
	}
}

// conflictOn is the brief a 409 revision_conflict names.
func conflictOn(err error) (briefID string, ok bool) {
	var he *hubclient.Error
	if !errors.As(err, &he) || he.Status != 409 || he.Problem.Error != hubapi.CodeRevisionConflict {
		return "", false
	}
	return he.Problem.Details["briefId"], true
}

// fanOutItem reports whether briefID is one of the batch's fan-out items.
func fanOutItem(items []item, briefID string) bool {
	for _, it := range items {
		if it.write.BriefID == briefID {
			return !it.own
		}
	}
	return false
}

// checkBatch is the item count, then the size checks (review M6, S4), before
// any Hub write: the count against MaxBatchItems; each item's canonical §3.4
// envelope against MaxEnvelopeBytes; the request body, as json.Marshal
// writes it, against MaxBatchBytes.
func checkBatch(hp, key string, items []item) error {
	needed := 0
	for _, it := range items {
		if !it.own {
			needed++
		}
	}
	limit := MaxBatchItems - 1
	if len(items) > MaxBatchItems {
		return &FanoutTooLargeError{Needed: needed, Limit: limit}
	}
	writes := make([]hubapi.RevisionWrite, len(items))
	for i, it := range items {
		writes[i] = it.write
		env, err := revisionEnvelope(it.write.BriefID, it.write.BaseRevision+1, hp, it.scope, it.branchRef, it.write.Content)
		if err != nil {
			return err
		}
		if len(env) <= MaxEnvelopeBytes {
			continue
		}
		if it.own {
			return &RevisionTooLargeError{Key: key, Bytes: len(env), Limit: MaxEnvelopeBytes}
		}
		return &FanoutTooLargeError{Needed: needed, Limit: limit, Bytes: len(env), LimitBytes: MaxEnvelopeBytes, BranchRef: it.branchRef}
	}
	body, err := json.Marshal(hubapi.WriteRevisionsRequest{Revisions: writes})
	if err != nil {
		return err
	}
	if len(body) > MaxBatchBytes {
		return &FanoutTooLargeError{Needed: needed, Limit: limit, Bytes: len(body), LimitBytes: MaxBatchBytes}
	}
	return nil
}

// stale is steps 8's and 9's rule: a pin below revision n of the project
// brief projectBrief. A null pin, and a pin to another project brief, are
// below every revision (re-review b06-1 R-M1).
func stale(pin *discover.Inherit, projectBrief string, n int) bool {
	if pin == nil {
		return true
	}
	k, err := provenance.Parse(pin.Key)
	if err != nil || k.Kind != provenance.KindBrief || k.ID != projectBrief {
		return true
	}
	return k.Revision < n
}

// repinProject is step 8: one re-list after the project batch committed,
// and one more fan-out for every live head still pinning below rev. It logs
// and never fails Apply.
func (s *Service) repinProject(ctx context.Context, hp, projectBrief string, rev hubapi.BriefRevision) {
	pin := discover.Inherit{Key: rev.Key, Digest: rev.Digest.String()}
	build := func(ctx context.Context) ([]item, error) {
		heads, err := s.branchHeads(ctx, hp)
		if err != nil {
			return nil, err
		}
		var behind []branchHead
		for _, h := range heads {
			if !h.archived && h.revision > 0 && stale(h.content.Inherits, projectBrief, rev.Revision) {
				behind = append(behind, h)
			}
		}
		return fanOut(pin, behind, s.cfg.Now(), s.cfg.NewID)
	}
	if _, _, err := s.writeBatch(ctx, hp, "", build); err != nil {
		s.cfg.Log.Warn("discoverhub: the project-side re-pin failed; the edit is written", "key", rev.Key, "err", err)
	}
}

// repinBranch is step 9: the project's latest complete revision read once
// after the branch write committed; if it is above the pin just written,
// one reinherit at the revision just written. A revision_conflict (the head
// moved again) is dropped; anything else is logged. It never fails Apply.
func (s *Service) repinBranch(ctx context.Context, hp string, mine item, rev hubapi.BriefRevision, written discover.Content) {
	fb, err := s.cfg.Hub.ForBranch(ctx, hp, mine.branchRef)
	if err != nil {
		s.cfg.Log.Warn("discoverhub: the branch-side re-pin could not read the project; the change is written", "key", rev.Key, "err", err)
		return
	}
	lc := latestComplete(fb.ProjectBrief)
	if lc == nil || !stale(written.Inherits, lc.BriefID, lc.Revision) {
		return
	}
	env := discover.Env{Scope: discover.ScopeBranch, Inherits: &discover.Inherit{Key: lc.Key, Digest: lc.Digest.String()},
		Now: s.cfg.Now(), NewID: s.cfg.NewID}
	next, err := discover.Apply(written, discover.Op{Kind: discover.OpReinherit}, env)
	if errors.Is(err, discover.ErrNoChange) {
		return
	}
	var raw []byte
	if err == nil {
		raw, err = json.Marshal(next)
	}
	if err == nil {
		again := item{write: hubapi.RevisionWrite{BriefID: rev.BriefID, BaseRevision: rev.Revision, Content: raw},
			scope: hubapi.ScopeBranch, branchRef: mine.branchRef, own: true}
		_, _, err = s.writeBatch(ctx, hp, "", func(context.Context) ([]item, error) { return []item{again}, nil })
	}
	if _, conflict := conflictOn(err); err != nil && !conflict {
		s.cfg.Log.Warn("discoverhub: the branch-side re-pin failed; the change is written", "key", rev.Key, "err", err)
	}
}
