package discoverhub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// ── the fan-out world ───────────────────────────────────────────────────────

// newProjectWorld is Engagement Portal, bound, with no brief yet.
func newProjectWorld(t *testing.T) *world {
	t.Helper()
	w := newBareWorld(t)
	w.project = w.createProject(projectName, teamKey)
	w.locate.addProject(localProject, projectDir, "engagement-portal")
	w.repos.set(projectDir, gh("origin", teamKey))
	return w
}

// newFanWorld is task 08's starting point: b-7 at 5, complete (fact sheet
// §3's first five revisions), and no branch brief yet.
func newFanWorld(t *testing.T) *world {
	t.Helper()
	w := newProjectWorld(t)
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	for _, s := range b7Steps()[:5] {
		w.save(w.b7, s, nil)
	}
	return w
}

// branchAt creates a branch brief on ref and writes b-42's first n steps to
// it, every one pinning pin. n 5 and 6 are complete; 1-4 are open.
func (w *world) branchAt(ref string, n int, pin *discover.Inherit) string {
	w.t.Helper()
	id := w.createBrief(w.project, hubapi.ScopeBranch, ref)
	for _, s := range b42Steps()[:n] {
		w.save(id, s, pin)
	}
	return id
}

// completeInOne creates a branch brief on ref whose revision 1 is complete:
// b-42's five answers in one revision, pinning pin.
func (w *world) completeInOne(ref string, pin *discover.Inherit) string {
	w.t.Helper()
	id := w.createBrief(w.project, hubapi.ScopeBranch, ref)
	var answers []discover.Answer
	for _, s := range b42Steps()[:5] {
		answers = append(answers, s.a)
	}
	w.saveContent(id, discover.Content{V: discover.ContentVersion, Status: discover.StatusComplete,
		Questions: discover.BaseQuestions(), Answers: answers, Inherits: pin,
		Change: discover.Change{Kind: discover.ChangeRecord, Key: discover.KeyOutOfScope}})
	return id
}

// saveContent writes c as briefID's next revision through the embedded fake.
func (w *world) saveContent(briefID string, c discover.Content) hubapi.BriefRevision {
	w.t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		w.t.Fatalf("marshal content: %v", err)
	}
	revs, err := w.hub.Fake.WriteRevisions(w.ctx, w.inBrief[briefID],
		[]hubapi.RevisionWrite{{BriefID: briefID, BaseRevision: w.head[briefID], Content: raw}})
	if err != nil {
		w.t.Fatalf("WriteRevisions(%s): %v", briefID, err)
	}
	w.head[briefID]++
	w.answers[briefID] = slices.Clone(c.Answers)
	r := revs[0]
	w.written[r.Key], w.pins[r.Key], w.content[r.Key] = r, c.Inherits, c
	return r
}

// headOf is briefID's head as the fake holds it now.
func (w *world) headOf(briefID string) hubapi.BriefHead {
	w.t.Helper()
	bw, err := w.hub.Fake.Brief(w.ctx, briefID)
	if err != nil {
		w.t.Fatalf("Brief(%s): %v", briefID, err)
	}
	return bw.Head
}

// at is revision n of briefID as the fake holds it, and its content decoded.
func (w *world) at(briefID string, n int) (hubapi.BriefRevision, discover.Content) {
	w.t.Helper()
	key := w.key(briefID, n)
	resp, err := w.hub.Fake.Resolve(w.ctx, []string{key})
	if err != nil || len(resp.Results) != 1 || resp.Results[0].Resolution != hubapi.Resolved {
		w.t.Fatalf("Resolve(%s) = %+v, %v", key, resp, err)
	}
	r := *resp.Results[0].Revision
	c, err := discover.Decode(r.Content)
	if err != nil {
		w.t.Fatalf("Decode(%s): %v", key, err)
	}
	return r, c
}

// pinned asserts revision n of briefID pins want, key and digest, and that
// its change is kind.
func (w *world) pinned(briefID string, n int, want *discover.Inherit, kind discover.ChangeKind) discover.Content {
	w.t.Helper()
	_, c := w.at(briefID, n)
	if !reflect.DeepEqual(c.Inherits, want) {
		w.t.Fatalf("%s pins %+v, want %+v", w.key(briefID, n), c.Inherits, want)
	}
	if c.Change.Kind != kind {
		w.t.Fatalf("%s change = %q, want %q", w.key(briefID, n), c.Change.Kind, kind)
	}
	return c
}

// hubPin is the pin to revision n of briefID with the digest the Hub holds.
func (w *world) hubPin(briefID string, n int) *discover.Inherit {
	w.t.Helper()
	r, _ := w.at(briefID, n)
	return &discover.Inherit{Key: r.Key, Digest: r.Digest.String()}
}

// writes is how many WriteRevisions calls reached the fake since mark m.
func (w *world) writes(m int) int {
	n := 0
	for _, c := range w.since(m) {
		if strings.HasPrefix(c, "WriteRevisions(") {
			n++
		}
	}
	return n
}

// branchLists is how many branch-scope Lists went through the wrapper: the
// fan-out's reads. The project brief's own read is a List too (task 07's
// briefsAt), with scope project.
func (w *world) branchLists() int {
	n := 0
	for _, q := range w.hub.lists() {
		if q.Scope == hubapi.ScopeBranch {
			n++
		}
	}
	return n
}

func editAudience(base int) Request {
	return Request{Target: Target{ProjectID: localProject}, BaseRevision: base,
		Op: discover.Op{Kind: discover.OpEdit, Key: discover.KeyAudience, Text: b7Audience6}}
}

func record(t Target, base int, k discover.Key, text string) Request {
	return Request{Target: t, BaseRevision: base,
		Op: discover.Op{Kind: discover.OpRecord, Key: k, Text: text, Source: discover.SourceTyped}}
}

func mustApply(t *testing.T, s *Service, w *world, r Request) Result {
	t.Helper()
	res, err := s.Apply(w.ctx, r)
	if err != nil {
		t.Fatalf("Apply(%+v): %v", r.Op, err)
	}
	return res
}

// ── one op, one revision ────────────────────────────────────────────────────

func TestFirstAnswerCreatesTheBriefAndWritesRevisionOne(t *testing.T) {
	w := newProjectWorld(t)
	s := w.service()
	m := w.mark()
	res := mustApply(t, s, w, record(w.projectT(), 0, discover.KeyPurpose, b7Purpose))
	var calls []string
	for _, c := range w.since(m) {
		if strings.HasPrefix(c, "CreateBrief(") || strings.HasPrefix(c, "WriteRevisions(") {
			calls = append(calls, c[:strings.IndexByte(c, '(')])
		}
	}
	if !slices.Equal(calls, []string{"CreateBrief", "WriteRevisions"}) {
		t.Fatalf("writes = %v, want CreateBrief then one WriteRevisions", calls)
	}
	if res.View.Brief == nil || res.View.Brief.HeadRevision != 1 {
		t.Fatalf("View.Brief = %+v, want head 1", res.View.Brief)
	}
	id := res.View.Brief.ID
	if want := []string{w.key(id, 1)}; !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	if _, c := w.at(id, 1); c.Change.Kind != discover.ChangeRecord || c.Status != discover.StatusOpen {
		t.Fatalf("revision 1 = %+v", c)
	}
}

// withCountedIDs gives each NewID call a fresh id, for ops that mint more
// than one (a done answer's criteria).
func withCountedIDs() func(*Config) {
	var mu sync.Mutex
	n := 0
	return func(c *Config) {
		c.NewID = func(p string) string {
			mu.Lock()
			defer mu.Unlock()
			n++
			return fmt.Sprintf("%s-%08x", p, n)
		}
	}
}

func TestFiveAnswersAreFiveRevisions(t *testing.T) {
	w := newProjectWorld(t)
	s := w.service(withCountedIDs())
	ops := []discover.Op{
		{Kind: discover.OpRecord, Key: discover.KeyPurpose, Text: b7Purpose, Source: discover.SourceTyped},
		{Kind: discover.OpSkip, Key: discover.KeyAudience},
		{Kind: discover.OpRecord, Key: discover.KeyConstraints, Text: b7Constraints, Source: discover.SourceTyped},
		{Kind: discover.OpRecord, Key: discover.KeyDone, Text: strings.Join(b7Done, "\n"), Source: discover.SourceTyped},
		{Kind: discover.OpRecord, Key: discover.KeyOutOfScope, Text: b7OutOfScope, Source: discover.SourceTyped},
	}
	var id string
	for i, op := range ops {
		res := mustApply(t, s, w, Request{Target: w.projectT(), BaseRevision: i, Op: op})
		if res.View.Brief == nil || res.View.Brief.HeadRevision != i+1 || len(res.Written) != 1 {
			t.Fatalf("op %d: brief %+v, written %v", i+1, res.View.Brief, res.Written)
		}
		id = res.View.Brief.ID
		want := discover.StatusOpen
		if i == 4 {
			want = discover.StatusComplete
		}
		if res.View.Brief.Status != want {
			t.Fatalf("revision %d status = %q, want %q", i+1, res.View.Brief.Status, want)
		}
	}
	h, err := s.History(w.ctx, id, "", 0)
	if err != nil || len(h.Rows) != 5 {
		t.Fatalf("History = %+v, %v; want five rows", h, err)
	}
	kinds := []discover.ChangeKind{discover.ChangeRecord, discover.ChangeSkip, discover.ChangeRecord, discover.ChangeRecord, discover.ChangeRecord}
	for i, row := range h.Rows {
		if row.Revision != i+1 || row.Change.Kind != kinds[i] {
			t.Fatalf("row %d = %+v, want revision %d, change %q", i, row, i+1, kinds[i])
		}
	}
}

func TestNoChangeWritesNothing(t *testing.T) {
	w := newProjectWorld(t)
	s := w.service()
	first := mustApply(t, s, w, record(w.projectT(), 0, discover.KeyPurpose, b7Purpose))
	m := w.mark()
	again := mustApply(t, s, w, record(w.projectT(), 1, discover.KeyPurpose, b7Purpose))
	if again.Written == nil || len(again.Written) != 0 {
		t.Fatalf("Written = %#v, want []", again.Written)
	}
	if n := w.writes(m); n != 0 {
		t.Fatalf("%d WriteRevisions for no change, want 0", n)
	}
	if again.View.Brief == nil || again.View.Brief.HeadRevision != 1 || w.headOf(first.View.Brief.ID).HeadRevision != 1 {
		t.Fatalf("head moved: view %+v", again.View.Brief)
	}
}

func TestProposeFollowUpWithNoModelWritesNothing(t *testing.T) {
	w := newFanWorld(t)
	s := w.service() // Config.Model nil
	m := w.mark()
	res := mustApply(t, s, w, Request{Target: w.projectT(), BaseRevision: 5, Op: discover.Op{Kind: OpProposeFollowUp}})
	if res.Assist != discover.AssistModelOff || res.Written == nil || len(res.Written) != 0 {
		t.Fatalf("Result = assist %q, written %#v; want model_off, []", res.Assist, res.Written)
	}
	if n := w.writes(m); n != 0 || w.headOf(w.b7).HeadRevision != 5 {
		t.Fatalf("%d writes, head %d; want none at 5", n, w.headOf(w.b7).HeadRevision)
	}
}

func TestRestoreWritesANewRevision(t *testing.T) {
	w := newFanWorld(t)
	w.save(w.b7, b7Steps()[5], nil) // b-7@6: the audience edit
	s := w.service()
	res := mustApply(t, s, w, Request{Target: w.projectT(), BaseRevision: 6,
		Op: discover.Op{Kind: discover.OpRestore, RestoreRevision: 5}})
	if want := []string{w.key(w.b7, 7)}; !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	_, five := w.at(w.b7, 5)
	_, seven := w.at(w.b7, 7)
	if !reflect.DeepEqual(seven.Answers, five.Answers) || !reflect.DeepEqual(seven.Questions, five.Questions) {
		t.Fatalf("revision 7's answers are not revision 5's:\n%+v\n%+v", seven.Answers, five.Answers)
	}
	if seven.Change != (discover.Change{Kind: discover.ChangeRestore, Restores: 5}) {
		t.Fatalf("change = %+v, want restore of 5", seven.Change)
	}
	if h := w.headOf(w.b7); h.HeadRevision != 7 {
		t.Fatalf("head = %d, want 7: an undo writes, never deletes", h.HeadRevision)
	}
}

func TestRestoreOfAnErasedRevisionIsRefused(t *testing.T) {
	w := newFanWorld(t)
	w.save(w.b7, b7Steps()[5], nil)
	// The fake erases a whole brief and archives it (contract §7.12), so a
	// live brief never holds an erased revision there; eraseInResolve makes
	// the Hub answer this one key erased, which is the case being refused.
	w.hub.eraseInResolve(w.key(w.b7, 5))
	s := w.service()
	m := w.mark()
	_, err := s.Apply(w.ctx, Request{Target: w.projectT(), BaseRevision: 6,
		Op: discover.Op{Kind: discover.OpRestore, RestoreRevision: 5}})
	if !errors.Is(err, ErrErasedRestore) {
		t.Fatalf("Apply = %v, want ErrErasedRestore", err)
	}
	if n := w.writes(m); n != 0 {
		t.Fatalf("%d writes, want 0", n)
	}
	_, err = s.Apply(w.ctx, Request{Target: w.projectT(), BaseRevision: 6,
		Op: discover.Op{Kind: discover.OpRestore, RestoreRevision: 9}})
	if !errors.Is(err, ErrNoBrief) {
		t.Fatalf("restore of a revision that does not exist = %v, want ErrNoBrief", err)
	}
}

// ── the 4-D3 fan-out ────────────────────────────────────────────────────────

func TestAProjectEditFansOutInOneBatch(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	b42 := w.branchAt(evidenceRef, 6, pin5)
	b51 := w.branchAt(timeoutRef, 5, pin5)
	s := w.service()
	res := mustApply(t, s, w, editAudience(5))
	batches := w.hub.batches()
	if len(batches) != 1 || len(batches[0]) != 3 {
		t.Fatalf("batches = %d (first has %d items), want one batch of 3", len(batches), len(batches[0]))
	}
	want := []string{w.key(w.b7, 6), w.key(b42, 7), w.key(b51, 6)}
	if !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	pin6 := w.hubPin(w.b7, 6)
	w.pinned(b42, 7, pin6, discover.ChangeReinherit)
	w.pinned(b51, 6, pin6, discover.ChangeReinherit)
	if h := w.headOf(b42); h.LatestCompleteRevision != 7 {
		t.Fatalf("b-42's latest complete = %d, want 7: a complete head's reinherit is complete", h.LatestCompleteRevision)
	}
}

func TestPinnedDigestEqualsTheHubsDigest(t *testing.T) {
	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 6, w.pinTo(w.b7, 5))
	mustApply(t, w.service(), w, editAudience(5))
	project, _ := w.at(w.b7, 6)
	_, c := w.at(b42, 7)
	if c.Inherits == nil || c.Inherits.Digest != project.Digest.String() || c.Inherits.Key != project.Key {
		t.Fatalf("b-42@7 pins %+v; the Hub holds %s at %s", c.Inherits, project.Digest, project.Key)
	}
}

// TestADigestDisagreementIsAnInternalError: when the Hub answers the project
// revision with a digest that is not the one the fan-out pinned, every
// branch revision in the batch names a digest nothing answers. Apply logs an
// internal error naming both and returns ErrDigestDisagreement, with the
// keys written, and runs no post-commit re-pin on the bad pin. With no
// fan-out item, nothing pinned it: the error is logged and Apply succeeds.
func TestADigestDisagreementIsAnInternalError(t *testing.T) {
	var log bytes.Buffer
	withLog := func(c *Config) { c.Log = slog.New(slog.NewTextHandler(&log, nil)) }

	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 6, w.pinTo(w.b7, 5))
	w.hub.skewNextDigest()
	res, err := w.service(withLog).Apply(w.ctx, editAudience(5))
	if !errors.Is(err, ErrDigestDisagreement) {
		t.Fatalf("Apply = %v, want ErrDigestDisagreement", err)
	}
	if want := []string{w.key(w.b7, 6), w.key(b42, 7)}; !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v: the batch committed", res.Written, want)
	}
	stored := w.hubPin(w.b7, 6).Digest
	if !strings.Contains(log.String(), "level=ERROR") || !strings.Contains(log.String(), "internal") ||
		!strings.Contains(log.String(), stored) {
		t.Fatalf("log = %q, want an internal error naming the pinned digest %s", log.String(), stored)
	}
	if n := len(w.hub.batches()); n != 1 || w.branchLists() != 1 {
		t.Fatalf("%d batches, %d branch Lists; want 1 and 1: no re-pin on a bad pin", n, w.branchLists())
	}

	log.Reset()
	alone := newFanWorld(t)
	alone.hub.skewNextDigest()
	if _, err := alone.service(withLog).Apply(alone.ctx, editAudience(5)); err != nil {
		t.Fatalf("Apply with no fan-out = %v, want nil: nothing pinned the digest", err)
	}
	if !strings.Contains(log.String(), "level=ERROR") {
		t.Fatalf("log = %q, want the disagreement logged", log.String())
	}
}

// openHeadWorld is TestFanOutReinheritsOpenHeadsAndSkipsArchived's world:
// b-42 complete at 6; b-51 open at 7, its revision 6 complete; b-60
// archived; all three pinning b-7@5.
func openHeadWorld(t *testing.T) (w *world, b42, b51, b60 string) {
	t.Helper()
	w = newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	b42 = w.branchAt(evidenceRef, 6, pin5)
	b51 = w.branchAt(timeoutRef, 6, pin5)
	c := w.content[w.key(b51, 6)]
	c.Questions = append(slices.Clone(c.Questions), discover.Question{Key: "f-00000001", FollowUp: true,
		BranchPrompt: "Which file types must a resumed upload keep?"})
	c.Status, c.Change = discover.StatusOpen, discover.Change{Kind: discover.ChangeAddFollowUp, Key: "f-00000001"}
	w.saveContent(b51, c) // b-51@7, open
	b60 = w.branchAt("refs/heads/main", 5, pin5)
	if _, err := w.hub.Fake.Archive(w.ctx, b60); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	return w, b42, b51, b60
}

func TestFanOutReinheritsOpenHeadsAndSkipsArchived(t *testing.T) {
	w, b42, b51, b60 := openHeadWorld(t)
	res := mustApply(t, w.service(), w, editAudience(5))
	batches := w.hub.batches()
	if len(batches) != 1 || len(batches[0]) != 3 {
		t.Fatalf("batches = %v, want one batch of 3", batches)
	}
	want := []string{w.key(w.b7, 6), w.key(b42, 7), w.key(b51, 8)}
	if !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	pin6 := w.hubPin(w.b7, 6)
	if c := w.pinned(b42, 7, pin6, discover.ChangeReinherit); c.Status != discover.StatusComplete {
		t.Fatalf("b-42@7 status = %q, want complete", c.Status)
	}
	c8 := w.pinned(b51, 8, pin6, discover.ChangeReinherit)
	_, c7 := w.at(b51, 7)
	if c8.Status != discover.StatusOpen || !reflect.DeepEqual(c8.Answers, c7.Answers) || !reflect.DeepEqual(c8.Questions, c7.Questions) {
		t.Fatalf("b-51@8 = %+v, want rev 7's questions and answers, open", c8)
	}
	if h := w.headOf(b60); h.HeadRevision != 5 {
		t.Fatalf("archived b-60 head = %d, want 5, untouched", h.HeadRevision)
	}
}

// The 4-D3 narrowing (C4, OQ-8): an open head carries the new project
// revision, but its latest complete revision is immutable and still pins the
// old one, and a session inherits that until the edit completes.
func TestFanOutLeavesAnOpenHeadsLatestCompletePinned(t *testing.T) {
	w, _, b51, _ := openHeadWorld(t)
	pin5 := w.hubPin(w.b7, 5)
	mustApply(t, w.service(), w, editAudience(5))
	if h := w.headOf(b51); h.LatestCompleteRevision != 6 || h.HeadRevision != 8 {
		t.Fatalf("b-51 head %d, latest complete %d; want 8 and 6", h.HeadRevision, h.LatestCompleteRevision)
	}
	w.pinned(b51, 6, pin5, discover.ChangeEdit)
}

func TestAnIncompleteProjectEditDoesNotFanOut(t *testing.T) {
	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 6, w.pinTo(w.b7, 5))
	q := discover.Question{ProjectPrompt: "Which client systems does the portal call?"}
	res := mustApply(t, w.service(), w, Request{Target: w.projectT(), BaseRevision: 5,
		Op: discover.Op{Kind: discover.OpAddFollowUp, Question: &q}})
	if b := w.hub.batches(); len(b) != 1 || len(b[0]) != 1 {
		t.Fatalf("batches = %v, want one batch of one item", b)
	}
	if res.View.Brief == nil || res.View.Brief.Status != discover.StatusOpen || w.headOf(b42).HeadRevision != 6 {
		t.Fatalf("project %+v; b-42 head %d, want 6", res.View.Brief, w.headOf(b42).HeadRevision)
	}
}

func TestBranchOpsPinTheCurrentProjectRevision(t *testing.T) {
	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 5, w.pinTo(w.b7, 5))
	w.save(w.b7, b7Steps()[5], nil) // b-7 moves to 6 with no fan-out
	res := mustApply(t, w.service(), w, Request{Target: w.branch(), BaseRevision: 5,
		Op: discover.Op{Kind: discover.OpEdit, Key: discover.KeyDone, Text: strings.Join(b42DoneB, "\n")}})
	if want := []string{w.key(b42, 6)}; !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	w.pinned(b42, 6, w.hubPin(w.b7, 6), discover.ChangeEdit)
}

func TestConflictOnThePersonsBriefIsReturned(t *testing.T) {
	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 6, w.pinTo(w.b7, 5))
	s := w.service()
	w.hub.beforeWrite(1, func() { // a teammate saves b-42@7 between our read and our write
		w.save(b42, step{person(discover.KeyPurpose, "A teammate's purpose"), discover.ChangeEdit, discover.StatusComplete}, w.pinTo(w.b7, 5))
	})
	res, err := s.Apply(w.ctx, Request{Target: w.branch(), BaseRevision: 6,
		Op: discover.Op{Kind: discover.OpEdit, Key: discover.KeyConstraints, Text: "Our constraints"}})
	var he *hubclient.Error
	if !errors.As(err, &he) || he.Status != 409 || he.Problem.Error != hubapi.CodeRevisionConflict || he.Problem.Details["briefId"] != b42 {
		t.Fatalf("Apply = %+v, %v; want the Hub's 409 revision_conflict naming b-42", res, err)
	}
	if len(res.Written) != 0 || w.headOf(b42).HeadRevision != 7 || len(w.hub.batches()) != 1 {
		t.Fatalf("written %v, head %d, %d batches; want nothing of ours written, no retry", res.Written, w.headOf(b42).HeadRevision, len(w.hub.batches()))
	}
}

func TestConflictOnAFanOutItemIsRetried(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	b42 := w.branchAt(evidenceRef, 6, pin5)
	b51 := w.branchAt(timeoutRef, 5, pin5)
	w.hub.beforeWrite(1, func() { // a teammate saves b-51@6 during the first attempt
		w.save(b51, b42Steps()[5], pin5)
	})
	res := mustApply(t, w.service(), w, editAudience(5))
	if n := len(w.hub.batches()); n != 2 {
		t.Fatalf("%d batches, want 2: one refused, one rebuilt", n)
	}
	want := []string{w.key(w.b7, 6), w.key(b42, 7), w.key(b51, 7)}
	if !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v", res.Written, want)
	}
	c7 := w.pinned(b51, 7, w.hubPin(w.b7, 6), discover.ChangeReinherit)
	_, c6 := w.at(b51, 6)
	if !reflect.DeepEqual(c7.Answers, c6.Answers) {
		t.Fatalf("b-51@7 is not based on the teammate's b-51@6")
	}
}

// ── the two post-commit checks (review I8, re-review b06-1) ─────────────────
//
// A project edit Lists twice through the wrapper before its batch: List 1 is
// the project brief's own read (task 07's briefsAt, scope project) and List 2
// is step 6's (scope branch). So "after step 6's List" is afterList(2).

func TestARacingBranchWriteCommittedAfterTheListIsRepinned(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	w.branchAt(evidenceRef, 6, pin5)
	var b70 string
	w.hub.afterList(2, func() { b70 = w.completeInOne("refs/heads/b70", pin5) })
	mustApply(t, w.service(), w, editAudience(5))
	if b70 == "" {
		t.Fatal("the hook never ran")
	}
	if n := w.branchLists(); n != 2 {
		t.Fatalf("%d branch Lists, want 2: step 6's and step 8's", n)
	}
	w.pinned(b70, 2, w.hubPin(w.b7, 6), discover.ChangeReinherit)
}

func TestABranchWriteThatLosesTheRaceRepinsItself(t *testing.T) {
	w := newFanWorld(t)
	s := w.service()
	// The case step 9 covers: a brief that the project's step-6 List and
	// step-8 re-list both missed, whose first revision commits after them.
	// The hook's bare b-7@6 stands in for that project batch.
	w.hub.beforeWrite(1, func() { w.save(w.b7, b7Steps()[5], nil) })
	res := mustApply(t, s, w, record(w.branch(), 0, discover.KeyPurpose, b42Purpose))
	if res.View.Brief == nil {
		t.Fatal("no brief in the view")
	}
	b80 := res.View.Brief.ID
	if want := []string{w.key(b80, 1)}; !slices.Equal(res.Written, want) {
		t.Fatalf("Written = %v, want %v: only the step-7 batch", res.Written, want)
	}
	w.pinned(b80, 1, w.hubPin(w.b7, 5), discover.ChangeRecord)
	w.pinned(b80, 2, w.hubPin(w.b7, 6), discover.ChangeReinherit)
	if res.View.Brief.HeadRevision != 2 {
		t.Fatalf("View head = %d, want 2: read after step 9", res.View.Brief.HeadRevision)
	}
}

func TestANullPinIsRepinned(t *testing.T) {
	t.Run("step 6", func(t *testing.T) {
		w := newProjectWorld(t)
		w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
		for _, s := range b7Steps()[:4] {
			w.save(w.b7, s, nil) // b-7 at 4, never complete
		}
		b90 := w.completeInOne("refs/heads/b90", nil) // written before b-7 ever completed
		res := mustApply(t, w.service(), w, record(w.projectT(), 4, discover.KeyOutOfScope, b7OutOfScope))
		if want := []string{w.key(w.b7, 5), w.key(b90, 2)}; !slices.Equal(res.Written, want) {
			t.Fatalf("Written = %v, want %v", res.Written, want)
		}
		w.pinned(b90, 2, w.hubPin(w.b7, 5), discover.ChangeReinherit)
	})
	t.Run("step 9", func(t *testing.T) {
		w := newProjectWorld(t)
		w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
		for _, s := range b7Steps()[:4] {
			w.save(w.b7, s, nil)
		}
		w.hub.beforeWrite(1, func() { w.save(w.b7, b7Steps()[4], nil) }) // b-7@5, its first complete revision
		res := mustApply(t, w.service(), w, record(w.branch(), 0, discover.KeyPurpose, b42Purpose))
		id := res.View.Brief.ID
		w.pinned(id, 1, nil, discover.ChangeRecord)
		w.pinned(id, 2, w.hubPin(w.b7, 5), discover.ChangeReinherit)
		if res.View.Brief.HeadRevision != 2 {
			t.Fatalf("View head = %d, want 2", res.View.Brief.HeadRevision)
		}
	})
}

func TestProjectPostCommitCheckRunsOnce(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	w.branchAt(evidenceRef, 6, pin5)
	var b71, b72 string
	w.hub.afterList(2, func() { b71 = w.completeInOne("refs/heads/b71", pin5) })   // after step 6's List
	w.hub.beforeWrite(2, func() { b72 = w.completeInOne("refs/heads/b72", pin5) }) // during step 8's write
	mustApply(t, w.service(), w, editAudience(5))
	if b71 == "" || b72 == "" {
		t.Fatal("a hook never ran")
	}
	if n := w.branchLists(); n != 2 {
		t.Fatalf("%d branch Lists, want exactly 2", n)
	}
	w.pinned(b71, 2, w.hubPin(w.b7, 6), discover.ChangeReinherit)
	if h := w.headOf(b72); h.HeadRevision != 1 {
		t.Fatalf("b-72 head = %d, want 1: step 8 runs once", h.HeadRevision)
	}
	w.pinned(b72, 1, pin5, discover.ChangeRecord)
}

func TestBranchPostCommitCheckRunsOnce(t *testing.T) {
	w := newFanWorld(t)
	b42 := w.branchAt(evidenceRef, 6, w.pinTo(w.b7, 5))
	// Each bare project revision stands in for a project batch (re-review
	// b06-2 N2): b-7@6 commits before the branch write, b-7@7 during step 9's.
	w.hub.beforeWrite(1, func() { w.save(w.b7, b7Steps()[5], nil) })
	w.hub.beforeWrite(2, func() {
		w.save(w.b7, step{person(discover.KeyConstraints, "Existing client data"), discover.ChangeEdit, discover.StatusComplete}, nil)
	})
	m := w.mark()
	res := mustApply(t, w.service(), w, Request{Target: w.branch(), BaseRevision: 6,
		Op: discover.Op{Kind: discover.OpEdit, Key: discover.KeyConstraints, Text: "Our constraints"}})
	if n := len(w.hub.batches()); n != 2 {
		t.Fatalf("%d batches, want 2: the answer and one re-pin", n)
	}
	w.pinned(b42, 7, w.hubPin(w.b7, 5), discover.ChangeEdit)
	w.pinned(b42, 8, w.hubPin(w.b7, 6), discover.ChangeReinherit)
	if h := w.headOf(b42); h.HeadRevision != 8 || res.View.Brief.HeadRevision != 8 {
		t.Fatalf("b-42 head %d, view %d; want 8", h.HeadRevision, res.View.Brief.HeadRevision)
	}
	// The project is read three times: step 3, step 9 (once), and the view.
	reads := 0
	for _, c := range w.since(m) {
		if strings.HasPrefix(c, "ForBranch(") {
			reads++
		}
	}
	if reads != 3 {
		t.Fatalf("%d ForBranch reads, want 3 (step 3, step 9, the view)", reads)
	}
}

// ── the size checks (review M6, S4; HA4-3 landed: 64 items, 4 MiB) ─────────

func TestFanOutTooLargeWritesNothing(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	for i := range MaxBatchItems { // 64: 48 complete, 16 open
		n := 5
		if i%4 == 3 {
			n = 4
		}
		w.branchAt(fmt.Sprintf("refs/heads/f%02d", i), n, pin5)
	}
	m := w.mark()
	_, err := w.service().Apply(w.ctx, editAudience(5))
	var fe *FanoutTooLargeError
	if !errors.As(err, &fe) || *fe != (FanoutTooLargeError{Needed: 64, Limit: 63}) {
		t.Fatalf("Apply = %v (%+v), want FanoutTooLargeError{Needed: 64, Limit: 63}", err, fe)
	}
	if n := w.writes(m); n != 0 || w.headOf(w.b7).HeadRevision != 5 {
		t.Fatalf("%d writes, b-7 head %d; want none, 5", n, w.headOf(w.b7).HeadRevision)
	}
}

// lessThans writes a branch brief on ref whose head's answers hold 40,000
// "<" in all, straight to the fake: text the Hub stored unchecked, which a
// reinherit carries as it is. "<" is one canonical byte and six in
// json.Marshal (a backslash, then u003c).
func (w *world) lessThans(ref string, pin *discover.Inherit) string {
	w.t.Helper()
	id := w.createBrief(w.project, hubapi.ScopeBranch, ref)
	answers := []discover.Answer{person(discover.KeyPurpose, strings.Repeat("<", 20000)),
		person(discover.KeyConstraints, strings.Repeat("<", 20000))}
	w.saveContent(id, discover.Content{V: discover.ContentVersion, Status: discover.StatusOpen,
		Questions: discover.BaseQuestions(), Answers: answers, Inherits: pin,
		Change: discover.Change{Kind: discover.ChangeRecord, Key: discover.KeyConstraints}})
	return id
}

func TestAnOversizedBatchWritesNothing(t *testing.T) {
	w := newFanWorld(t)
	pin5 := w.pinTo(w.b7, 5)
	const n = 18 // 18 × ~240,000 json.Marshal bytes > 4 MiB
	for i := range n {
		w.lessThans(fmt.Sprintf("refs/heads/lt%02d", i), pin5)
	}
	m := w.mark()
	_, err := w.service().Apply(w.ctx, editAudience(5))
	var fe *FanoutTooLargeError
	if !errors.As(err, &fe) || fe.Needed != n || fe.Limit != 63 || fe.Bytes <= MaxBatchBytes ||
		fe.LimitBytes != MaxBatchBytes || fe.BranchRef != "" {
		t.Fatalf("Apply = %v (%+v), want a FanoutTooLargeError on the body", err, fe)
	}
	if n := w.writes(m); n != 0 {
		t.Fatalf("%d writes, want 0", n)
	}

	two := newFanWorld(t)
	pin5 = two.pinTo(two.b7, 5)
	two.lessThans("refs/heads/lt00", pin5)
	two.lessThans("refs/heads/lt01", pin5)
	if res := mustApply(t, two.service(), two, editAudience(5)); len(res.Written) != 3 {
		t.Fatalf("two such branches: Written = %v, want 3 keys", res.Written)
	}
}

// cjk is n code points of CJK text, three UTF-8 bytes each.
func cjk(n int) string { return strings.Repeat("監", n) }

func TestAnOversizedRevisionNamesTheAnswer(t *testing.T) {
	w := newProjectWorld(t)
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	lines := make([]string, 40)
	done := person(discover.KeyDone, "")
	for i := range lines {
		lines[i] = cjk(99)
		done.Criteria = append(done.Criteria, discover.Criterion{ID: fmt.Sprintf("c-%08d", i+1), Text: lines[i]})
	}
	done.Text = strings.Join(lines, "\n") // 3,999 code points
	w.saveContent(w.b7, discover.Content{V: discover.ContentVersion, Status: discover.StatusOpen,
		Questions: discover.BaseQuestions(), Change: discover.Change{Kind: discover.ChangeRecord, Key: discover.KeyDone},
		Answers: []discover.Answer{person(discover.KeyAudience, cjk(4000)), person(discover.KeyConstraints, cjk(4000)),
			person(discover.KeyOutOfScope, cjk(3000)), done}})
	s := w.service()
	bw, err := w.hub.Fake.Brief(w.ctx, w.b7)
	if err != nil {
		t.Fatal(err)
	}
	head, err := discover.Decode(bw.Revision.Content)
	if err != nil {
		t.Fatal(err)
	}
	// size is the canonical envelope of revision 2 with purpose = text, as
	// H0's own builder measures it (under the limit only).
	size := func(text string) int {
		t.Helper()
		c, err := discover.Apply(head, discover.Op{Kind: discover.OpRecord, Key: discover.KeyPurpose, Text: text,
			Source: discover.SourceTyped}, discover.Env{Scope: discover.ScopeProject, Now: w.clock.Now(), NewID: s.cfg.NewID})
		if err != nil {
			t.Fatalf("discover.Apply: %v", err)
		}
		raw, _ := json.Marshal(c)
		env, err := hubapi.BriefEnvelope{BriefID: w.b7, Revision: 2, ProjectID: w.project, Scope: hubapi.ScopeProject,
			Content: raw}.Canonical()
		if err != nil {
			t.Fatalf("Canonical: %v", err)
		}
		return len(env)
	}
	// A purpose of "a" gives size("a"); every further byte of text adds one
	// canonical byte ("監" is three, "a" one). CJK first, then ASCII to the
	// exact size, within task 03's 4,000 code points.
	need := MaxEnvelopeBytes - size("a")
	k := min(need/3, 3900)
	base := cjk(k)
	pad := MaxEnvelopeBytes - size(base+"a") + 1
	if pad < 1 || k+pad+1 > discover.MaxAnswerRunes || size(base+strings.Repeat("a", pad)) != MaxEnvelopeBytes {
		t.Fatalf("k %d, pad %d do not fit the fixture", k, pad)
	}
	m := w.mark()
	_, err = s.Apply(w.ctx, record(w.projectT(), 1, discover.KeyPurpose, base+strings.Repeat("a", pad+1)))
	var re *RevisionTooLargeError
	if !errors.As(err, &re) || *re != (RevisionTooLargeError{Key: "purpose", Bytes: 65537, Limit: 65536}) {
		t.Fatalf("Apply = %v (%+v), want RevisionTooLargeError{purpose, 65537, 65536}", err, re)
	}
	if n := w.writes(m); n != 0 {
		t.Fatalf("%d writes, want 0", n)
	}
	res := mustApply(t, s, w, record(w.projectT(), 1, discover.KeyPurpose, base+strings.Repeat("a", pad)))
	if want := []string{w.key(w.b7, 2)}; !slices.Equal(res.Written, want) {
		t.Fatalf("at exactly 65,536: Written = %v, want %v", res.Written, want)
	}
}

// TestAnOversizedFanOutItemNamesTheBranch: b-51's head, stored unchecked,
// is 50 bytes under 64 KiB with inherits null. The project's first complete
// revision makes its reinherit pin {key, digest}, about 110 bytes more (the
// part said 150 under; a pin adds less than 150, so 50 it is).
func TestAnOversizedFanOutItemNamesTheBranch(t *testing.T) {
	w := newProjectWorld(t)
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	for _, s := range b7Steps()[:4] {
		w.save(w.b7, s, nil)
	}
	b51 := w.createBrief(w.project, hubapi.ScopeBranch, timeoutRef)
	content := func(pad int) discover.Content {
		return discover.Content{V: discover.ContentVersion, Status: discover.StatusOpen, Questions: discover.BaseQuestions(),
			Answers: []discover.Answer{person(discover.KeyPurpose, strings.Repeat("a", pad))},
			Change:  discover.Change{Kind: discover.ChangeRecord, Key: discover.KeyPurpose}}
	}
	envSize := func(c discover.Content) int {
		raw, _ := json.Marshal(c)
		env, err := hubapi.BriefEnvelope{BriefID: b51, Revision: 1, ProjectID: w.project, Scope: hubapi.ScopeBranch,
			BranchRef: timeoutRef, Content: raw}.Canonical()
		if err != nil {
			t.Fatalf("Canonical: %v", err)
		}
		return len(env)
	}
	pad := MaxEnvelopeBytes - 50 - envSize(content(0))
	c := content(pad)
	if got := envSize(c); got != MaxEnvelopeBytes-50 {
		t.Fatalf("fixture envelope = %d, want %d", got, MaxEnvelopeBytes-50)
	}
	w.saveContent(b51, c)
	m := w.mark()
	_, err := w.service().Apply(w.ctx, record(w.projectT(), 4, discover.KeyOutOfScope, b7OutOfScope))
	var fe *FanoutTooLargeError
	if !errors.As(err, &fe) || fe.Needed != 1 || fe.BranchRef != timeoutRef || fe.Bytes <= MaxEnvelopeBytes ||
		fe.LimitBytes != MaxEnvelopeBytes {
		t.Fatalf("Apply = %v (%+v), want FanoutTooLargeError naming %s", err, fe, timeoutRef)
	}
	var re *RevisionTooLargeError
	if errors.As(err, &re) {
		t.Fatalf("a branch's overflow reported to the project editor as %+v", re)
	}
	if n := w.writes(m); n != 0 {
		t.Fatalf("%d writes, want 0", n)
	}
}
