package provenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// The fact sheet's world (prototype-fact-sheet.md §2-§3), with Hub-shaped
// ids as task 05 uses them: b-7 is bf-00000000000000b7, b-42 is
// bf-00000000000000a2, both in the Hub project pj-00000000000000e1.
const (
	projectBrief = "bf-00000000000000b7"
	branchBrief  = "bf-00000000000000a2"
	hubProject   = "pj-00000000000000e1"
	stamp        = "2026-09-24T10:00:00.000Z"
	evidenceRef  = "refs/heads/evidence-upload"
)

var (
	key7  = BriefKey(projectBrief, 5)
	key42 = BriefKey(branchBrief, 5)
	dig7  = "sha256:" + strings.Repeat("7", 64)
	dig42 = "sha256:" + strings.Repeat("42", 32)
	// head42 is b-42's head "now" (fact sheet §2: rev 6 is the head).
	head42 = Head{HeadRevision: 6, LatestCompleteRevision: 6}
)

func person(k discover.Key, text string) discover.Answer {
	return discover.Answer{Key: k, Text: text, Source: discover.SourceTyped, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

func skip(k discover.Key) discover.Answer {
	return discover.Answer{Key: k, Skipped: true, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
}

func done(tag string, items ...string) discover.Answer {
	a := person(discover.KeyDone, strings.Join(items, "\n"))
	for i, it := range items {
		a.Criteria = append(a.Criteria, discover.Criterion{ID: fmt.Sprintf("c-%s%07d", tag, i+1), Text: it})
	}
	return a
}

func content(status discover.Status, answers ...discover.Answer) discover.Content {
	return discover.Content{V: discover.ContentVersion, Status: status,
		Questions: discover.BaseQuestions(), Answers: answers}
}

// b7 is b-7@5, the project revision b-42@5 pins.
func b7() discover.Revision {
	return discover.Revision{BriefID: projectBrief, Number: 5, Key: key7, Scope: discover.ScopeProject,
		Content: content(discover.StatusComplete,
			person(discover.KeyPurpose, "Client-facing portal for audit engagements: requests, evidence and status."),
			person(discover.KeyAudience, "KPMG staff broadly"),
			person(discover.KeyConstraints, "Existing client data; the published API"),
			done("b", "Tests pass", "No change to the published API without a version bump"),
			person(discover.KeyOutOfScope, "Billing and invoicing"),
		)}
}

// b42 is b-42@5 on evidence-upload: audience skipped, so it inherits b-7's.
func b42() discover.Revision {
	c := content(discover.StatusComplete,
		person(discover.KeyPurpose, "Let client users upload evidence files against an open request."),
		skip(discover.KeyAudience),
		person(discover.KeyConstraints, "Existing uploads and their download links"),
		done("a", "Upload works for PDF, XLSX and images", "A virus scan runs before any file is visible", "Tests pass"),
		person(discover.KeyOutOfScope, "Bulk upload; the reviewer side of evidence"),
	)
	c.Inherits = &discover.Inherit{Key: key7, Digest: dig7}
	return discover.Revision{BriefID: branchBrief, Number: 5, Key: key42, Scope: discover.ScopeBranch,
		BranchRef: evidenceRef, Content: c}
}

// src42 is b-42@5 as discoverhub hands it over, with its pin in hand.
func src42() Source {
	p := b7()
	return Source{Revision: b42(), Project: &p, Digest: dig42, ProjectID: hubProject,
		SavedAt: "2026-09-22T09:12:00.000Z", SavedBy: "Dylan Mackie"}
}

// src7 is b-7@5 alone: a project key.
func src7() Source {
	return Source{Revision: b7(), Digest: dig7, ProjectID: hubProject,
		SavedAt: "2026-09-21T16:40:00.000Z", SavedBy: "Dylan Mackie"}
}

func mustBuild(t *testing.T, key string, src Source, head Head) Ref {
	t.Helper()
	ref, err := Build(key, src, head)
	if err != nil {
		t.Fatalf("Build(%s): %v", key, err)
	}
	return ref
}

func sectionFor(ref Ref, k discover.Key) (discover.Section, bool) {
	for _, s := range ref.Sections {
		if s.Key == k {
			return s, true
		}
	}
	return discover.Section{}, false
}

func TestEmptyIsNotAKeyButIsNone(t *testing.T) {
	if _, err := Parse(""); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf(`Parse("") error = %v, want ErrInvalidKey`, err)
	}
	ref := NoneRef("")
	if ref.Resolution != None || ref.NoneReason != "" || ref.Present || ref.Key != "" {
		t.Errorf(`NoneRef("") = %+v, want resolution none, noneReason "", present false`, ref)
	}
}

func TestDeclinedIsNoneWithAReason(t *testing.T) {
	ref := NoneRef(Declined)
	if ref.Resolution != None || ref.NoneReason != "declined" || ref.Present || ref.Key != Declined {
		t.Errorf("NoneRef(Declined) = %+v, want resolution none, noneReason declined, present false", ref)
	}
	if NoneRef("").NoneReason == ref.NoneReason {
		t.Error("the override reads the same as no brief")
	}
}

// TestNoneRefRefusesAKey: NoneRef is for "" and none:declined only; any
// other string is a caller that skipped Parse, a programming error.
func TestNoneRefRefusesAKey(t *testing.T) {
	for _, in := range []string{key42, "none:other", "none:", " ", "declined"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NoneRef(%q) did not panic", in)
				}
			}()
			_ = NoneRef(in)
		}()
	}
}

func TestBuildBranchKeyShowsInheritedText(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	if ref.Audience != "KPMG staff broadly" {
		t.Errorf("Audience = %q, want the project's text (critique-2 §1.1)", ref.Audience)
	}
	if s, ok := sectionFor(ref, discover.KeyAudience); !ok || s.Origin != discover.OriginProject {
		t.Errorf("audience section = %+v (present %v), want OriginProject", s, ok)
	}
	if ref.Headline != "Let client users upload evidence files against an open request." ||
		ref.Constraints != "Existing uploads and their download links" ||
		ref.OutOfScope != "Bulk upload; the reviewer side of evidence" {
		t.Errorf("headline/constraints/outOfScope = %q / %q / %q, want the branch's own", ref.Headline, ref.Constraints, ref.OutOfScope)
	}
	want := Ref{Key: key42, Resolution: Resolved, Present: true, BriefID: branchBrief, Revision: 5,
		Digest: dig42, Scope: "branch", ProjectID: hubProject, BranchRef: evidenceRef, Status: "complete",
		HeadRevision: 6, LatestCompleteRevision: 6, Title: "Branch brief — evidence-upload",
		SavedAt: "2026-09-22T09:12:00.000Z", SavedBy: "Dylan Mackie"}
	got := ref
	got.Inherits, got.Headline, got.Audience, got.Constraints, got.OutOfScope, got.Sections, got.Criteria = nil, "", "", "", "", nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Build scalars =\n%+v\nwant\n%+v", got, want)
	}
	wantPin := InheritedRef{Key: key7, BriefID: projectBrief, Revision: 5, Digest: dig7}
	if ref.Inherits == nil || *ref.Inherits != wantPin {
		t.Errorf("Inherits = %+v, want %+v", ref.Inherits, wantPin)
	}
}

// TestBuildWithAnErasedPinHoldsNoInheritedText (re-review b06-1 R-I2).
func TestBuildWithAnErasedPinHoldsNoInheritedText(t *testing.T) {
	src := src42()
	src.Project, src.PinErased = nil, true
	ref, err := Build(key42, src, head42)
	if err != nil {
		t.Fatalf("Build with an erased pin: %v (want no ErrInheritMismatch: it would blank every chip on the branch)", err)
	}
	wantPin := InheritedRef{Key: key7, BriefID: projectBrief, Revision: 5, Digest: dig7, Erased: true}
	if ref.Inherits == nil || *ref.Inherits != wantPin {
		t.Errorf("Inherits = %+v, want %+v", ref.Inherits, wantPin)
	}
	if ref.Resolution != Resolved || !ref.Present || ref.Audience != "" {
		t.Errorf("resolution %q present %v audience %q, want resolved, true, \"\"", ref.Resolution, ref.Present, ref.Audience)
	}
	// The title is the key's, erased pin or not (mutant R4-P2).
	if ref.Title != "Branch brief — evidence-upload" {
		t.Errorf("Title = %q, want the branch revision's title", ref.Title)
	}
	for _, s := range ref.Sections {
		if s.Origin == discover.OriginProject || s.Replaced != "" {
			t.Errorf("section %q origin %q replaced %q: erased project text is in the Ref", s.Key, s.Origin, s.Replaced)
		}
	}
	raw, _ := json.Marshal(ref)
	for _, text := range []string{"KPMG staff broadly", "Client-facing portal", "Existing client data", "No change to the published API", "Billing and invoicing"} {
		if strings.Contains(string(raw), text) {
			t.Errorf("the Ref's JSON holds the erased pin's text %q", text)
		}
	}
	if ref.Headline == "" || len(ref.Criteria) != 3 {
		t.Errorf("headline %q, %d criteria: the branch's own answers are gone too", ref.Headline, len(ref.Criteria))
	}

	// An open history row whose pin was erased builds too: open, own answers,
	// the pin marked erased (mutant X2: the erased case ahead of the open one).
	open := src42()
	open.Revision.Content.Status = discover.StatusOpen
	open.Project, open.PinErased = nil, true
	oref, err := Build(key42, open, head42)
	if err != nil {
		t.Fatalf("Build of an open revision with an erased pin: %v", err)
	}
	if oref.Status != "open" || oref.Inherits == nil || !oref.Inherits.Erased || oref.Audience != "" || len(oref.Criteria) != 3 {
		t.Errorf("open revision, erased pin: %+v", oref)
	}
}

// TestBuildTitlesAnOpenRevisionByTheOneRule (re-review b05-1 R-M2).
func TestBuildTitlesAnOpenRevisionByTheOneRule(t *testing.T) {
	branch := b42()
	branch.Number, branch.Key = 3, BriefKey(branchBrief, 3)
	branch.Content = content(discover.StatusOpen,
		person(discover.KeyPurpose, "Let client users upload evidence files against an open request."),
		skip(discover.KeyAudience),
		discover.Answer{Key: discover.KeyConstraints, Text: "held draft", Source: discover.SourceDrafted,
			Author: discover.AuthorModelDrafted, Criteria: []discover.Criterion{}, DraftedFrom: []string{"go.mod"}, AnsweredAt: stamp},
	)
	branch.Content.Inherits = &discover.Inherit{Key: key7, Digest: dig7}
	p := b7()
	ref := mustBuild(t, branch.Key, Source{Revision: branch, Project: &p, Digest: dig42, ProjectID: hubProject}, head42)
	if want := discover.Title(&branch); ref.Title != want || want != "Branch brief — evidence-upload" {
		t.Errorf("Title = %q, discover.Title = %q, want both %q", ref.Title, want, "Branch brief — evidence-upload")
	}
	// An open revision is resolved and present like any other (mutant R4-P1).
	if ref.BranchRef != branch.BranchRef || ref.Status != "open" || ref.Resolution != Resolved || !ref.Present {
		t.Errorf("branchRef %q status %q resolution %q present %v", ref.BranchRef, ref.Status, ref.Resolution, ref.Present)
	}
	if len(ref.Sections) != 1 || ref.Sections[0].Key != discover.KeyPurpose || ref.Sections[0].Origin != discover.OriginBranch {
		t.Errorf("sections = %+v, want the one settled answer, no inheritance, no draft", ref.Sections)
	}
	if ref.Audience != "" || ref.Constraints != "" {
		t.Errorf("audience %q constraints %q: an open revision inherits nothing and renders no draft", ref.Audience, ref.Constraints)
	}
	// The pin's revision is its key's (5), never the branch revision's own
	// number (3): mutant R6-P2, which every other fixture's 5 == 5 hides.
	if want := (InheritedRef{Key: key7, BriefID: projectBrief, Revision: 5, Digest: dig7}); ref.Inherits == nil || *ref.Inherits != want {
		t.Errorf("Inherits = %+v, want %+v: the pin the revision's content holds", ref.Inherits, want)
	}

	project := b7()
	project.Number, project.Key = 4, BriefKey(projectBrief, 4)
	project.Content = content(discover.StatusOpen, person(discover.KeyPurpose, "Client-facing portal."))
	pref := mustBuild(t, project.Key, Source{Revision: project, Digest: dig7, ProjectID: hubProject}, Head{})
	if want := discover.Title(nil); pref.Title != want || want != "Project brief" {
		t.Errorf("project Title = %q, discover.Title(nil) = %q, want both Project brief", pref.Title, want)
	}
	if pref.BranchRef != "" || pref.Scope != "project" || pref.Headline != "Client-facing portal." || pref.Inherits != nil {
		t.Errorf("open project Ref = %+v", pref)
	}
	// Neither revision has a definition of done, and a built Ref's empty
	// slices are [] on the wire like a slot's, never null: task 15 mirrors
	// the shape and the desktop maps over it (mutant RVB).
	for name, r := range map[string]Ref{"branch": ref, "project": pref} {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"criteria":[]`) || !strings.Contains(string(raw), `"sections":[`) {
			t.Errorf("%s: the built Ref's JSON has no \"criteria\":[] or no sections array: %s", name, raw)
		}
	}
}

func TestBuildCarriesCriterionDigests(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	src := b42()
	a, _ := src.Content.AnswerFor(discover.KeyDone)
	if len(ref.Criteria) != len(a.Criteria) || len(a.Criteria) != 3 {
		t.Fatalf("%d criteria, want %d", len(ref.Criteria), len(a.Criteria))
	}
	for i, c := range ref.Criteria {
		if c.ID != a.Criteria[i].ID || c.Text != a.Criteria[i].Text || c.Digest != CriterionDigest(a.Criteria[i].Text) {
			t.Errorf("criterion %d = %+v, want id %q text %q digest %s", i, c, a.Criteria[i].ID, a.Criteria[i].Text, CriterionDigest(a.Criteria[i].Text))
		}
	}
	// A project key's criteria are its own.
	pref := mustBuild(t, key7, src7(), Head{})
	if len(pref.Criteria) != 2 || pref.Criteria[0].ID != "c-b0000001" || pref.Criteria[1].Text != "No change to the published API without a version bump" {
		t.Errorf("project criteria = %+v", pref.Criteria)
	}
}

// TestCriterionDigestVector: the literal is `printf 'Tests pass' | sha256sum`,
// run when this test was written.
func TestCriterionDigestVector(t *testing.T) {
	const want = "sha256:c88379191a17bec72f18cb6d2e1777bee5ebce79e416dfd8e96009b0109cdc20"
	if got := CriterionDigest("Tests pass"); got != want {
		t.Errorf("CriterionDigest(\"Tests pass\") = %s, want %s", got, want)
	}
	for _, other := range []string{"tests pass", "Tests pass ", " Tests pass", `"Tests pass"`, "Tests pass\n"} {
		if CriterionDigest(other) == want {
			t.Errorf("CriterionDigest(%q) equals the digest of \"Tests pass\": the text is not hashed verbatim", other)
		}
	}
}

// TestBuildRefusesASourceTheKeyDoesNotName: a Ref names one immutable text,
// so a source that is not the revision the key names, or a pin that is not
// the one the revision holds, is refused with an error and an unavailable
// Ref - never a Ref that puts one key on another revision's text. Every
// refusal is ErrBadSource, so a caller picks its status with one errors.Is;
// discover's own refusal stays in the chain beside it (review m5).
func TestBuildRefusesASourceTheKeyDoesNotName(t *testing.T) {
	type row struct {
		name  string
		key   string
		src   func() Source
		cause error // discover's refusal, also in the chain; nil: refused before discover
	}
	withPin := func(k string) func() Source {
		return func() Source {
			s := src42()
			s.Revision.Content.Inherits = &discover.Inherit{Key: k, Digest: dig7}
			p := b7()
			p.Key = k
			s.Project = &p
			return s
		}
	}
	for _, tc := range []row{
		{"a later revision's key", BriefKey(branchBrief, 6), src42, nil},
		{"another brief's key", BriefKey("bf-00000000000000a3", 5), src42, nil},
		{"a design key", "design:cv-7c01d2e3f4a5b6c7@5", src42, nil},
		// Mutant R9-P2: the key's kind is checked before anything reaches
		// discover, even on a revision relabelled to agree with the key.
		{"a design key on a revision relabelled to match it", "design:cv-7c01d2e3f4a5b6c7@5", func() Source {
			s := src42()
			s.Revision.Key, s.Revision.BriefID = "design:cv-7c01d2e3f4a5b6c7@5", "cv-7c01d2e3f4a5b6c7"
			return s
		}, nil},
		{"not a key", "brief:b-42@5", src42, nil},
		{"the empty string", "", src42, nil},
		{"none:declined", Declined, src42, nil},
		{"an envelope key that differs", key42, func() Source { s := src42(); s.Revision.Key = BriefKey(branchBrief, 6); return s }, nil},
		{"a project revision with a project passed", key7, func() Source { s := src7(); p := b7(); s.Project = &p; return s }, nil},
		{"an erased pin with the pin passed", key42, func() Source { s := src42(); s.PinErased = true; return s }, nil},
		{"an erased pin on a revision that pins nothing", key7, func() Source { s := src7(); s.PinErased = true; return s }, nil},
		{"a pin that is not a brief key", key42, withPin("brief:b-7@5"), nil},
		// Mutant R9-P1: a pin that parses, as another kind, with the project
		// passed under that key.
		{"a pin that is a design key", key42, withPin("design:cv-7c01d2e3f4a5b6c7@5"), nil},
		{"the project head, not the pin", key42, func() Source {
			s := src42()
			p := b7()
			p.Number, p.Key = 6, BriefKey(projectBrief, 6)
			s.Project = &p
			return s
		}, discover.ErrInheritMismatch},
		{"no pin passed, none erased", key42, func() Source { s := src42(); s.Project = nil; return s }, discover.ErrInheritMismatch},
		// Mutant P1: a project revision whose content holds a pin.
		{"a project revision that holds a pin", key7, func() Source {
			s := src7()
			s.Revision.Content.Inherits = &discover.Inherit{Key: BriefKey(projectBrief, 4), Digest: dig7}
			return s
		}, nil},
		// Mutant X1: the envelope's Key agrees with the key, its id or number does not.
		{"an envelope whose id is another brief's", key42, func() Source { s := src42(); s.Revision.BriefID = "bf-00000000000000a3"; return s }, nil},
		{"an envelope whose number is another revision's", key42, func() Source { s := src42(); s.Revision.Number = 6; return s }, nil},
		// Review m5: what discover refuses is a bad source too. A pin whose
		// stored status is open (ErrIncomplete), and envelopes discover's
		// checkRevisions refuses: a branch revision with no branchRef, and a
		// scope that is neither.
		{"a pin that is open", key42, func() Source {
			s := src42()
			s.Project.Content.Status = discover.StatusOpen
			return s
		}, discover.ErrIncomplete},
		{"a branch revision with no branchRef", key42, func() Source { s := src42(); s.Revision.BranchRef = ""; return s }, nil},
		{"a revision in a scope that is neither", key7, func() Source { s := src7(); s.Revision.Scope = "team"; return s }, nil},
	} {
		ref, err := Build(tc.key, tc.src(), head42)
		if !errors.Is(err, ErrBadSource) {
			t.Errorf("%s: Build error = %v, want ErrBadSource", tc.name, err)
		}
		if tc.cause != nil && !errors.Is(err, tc.cause) {
			t.Errorf("%s: Build error = %v, want %v in its chain too", tc.name, err, tc.cause)
		}
		if tc.cause == nil && errors.Is(err, discover.ErrInheritMismatch) {
			t.Errorf("%s: Build error = %v, which is not a pin mismatch", tc.name, err)
		}
		if ref.Resolution != Unavailable || ref.Present || ref.Key != tc.key || ref.Title != "" || len(ref.Sections) != 0 || ref.Inherits != nil {
			t.Errorf("%s: Build Ref = %+v, want an unavailable Ref for the key and nothing else", tc.name, ref)
		}
	}
}

// verdicts is one Verdict per criterion of ref, in order, all in one state.
func verdicts(ref Ref, state CriterionState, because ...string) []Verdict {
	out := make([]Verdict, len(ref.Criteria))
	for i, c := range ref.Criteria {
		out[i] = Verdict{CriterionID: c.ID, State: state, Because: because}
	}
	return out
}

// counting is an Evaluate that records every query and answers with fn.
func counting(fn func(q EvalQuery) ([]Verdict, error)) (Evaluate, *[]EvalQuery) {
	var calls []EvalQuery
	return func(_ context.Context, q EvalQuery) ([]Verdict, error) {
		calls = append(calls, q)
		return fn(q)
	}, &calls
}

func assertAllOpen(t *testing.T, name string, ref Ref, got []CriterionReading) {
	t.Helper()
	if len(got) != len(ref.Criteria) {
		t.Fatalf("%s: %d readings for %d criteria", name, len(got), len(ref.Criteria))
	}
	for i, r := range got {
		if r != (CriterionReading{ID: ref.Criteria[i].ID, State: StateOpen}) {
			t.Errorf("%s: reading %d = %+v, want %s open with because \"\"", name, i, r, ref.Criteria[i].ID)
		}
	}
}

func TestNilEvaluateMeansOpen(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	assertAllOpen(t, "nil Evaluate", ref, Readings(context.Background(), key42, src42(), evidenceRef, nil))
}

func TestUnexplainedVerdictIsShownOpen(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	// Both non-open states (mutant R3-P1: the rule held for settled only).
	for _, state := range []CriterionState{StateSettled, StateContested} {
		for _, because := range [][]string{nil, {}, {"  "}, {"", "\t\n"}} {
			eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return verdicts(ref, state, because...), nil })
			assertAllOpen(t, fmt.Sprintf("%s because %q", state, because), ref, Readings(context.Background(), key42, src42(), evidenceRef, eval))
			if len(*calls) != 1 {
				t.Errorf("%s because %q: %d calls, want 1", state, because, len(*calls))
			}
		}
	}
}

// TestAnOpenRevisionIsEvaluated: Readings runs on every answer, and an open
// history row's criteria are criteria like any other (mutant R3-P2).
func TestAnOpenRevisionIsEvaluated(t *testing.T) {
	src := src42()
	src.Revision.Number, src.Revision.Key = 4, BriefKey(branchBrief, 4)
	src.Revision.Content = content(discover.StatusOpen,
		person(discover.KeyPurpose, "Let client users upload evidence files against an open request."),
		done("a", "Upload works for PDF, XLSX and images", "Tests pass"),
	)
	ref := mustBuild(t, src.Revision.Key, src, head42)
	if ref.Status != "open" || len(ref.Criteria) != 2 {
		t.Fatalf("open revision Ref: status %q, %d criteria", ref.Status, len(ref.Criteria))
	}
	eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return verdicts(ref, StateSettled, "CI run 88 passed"), nil })
	got := Readings(context.Background(), src.Revision.Key, src, evidenceRef, eval)
	if len(*calls) != 1 || len(got) != 2 || got[0].State != StateSettled || got[1] != (CriterionReading{ID: ref.Criteria[1].ID, State: StateSettled, Because: "CI run 88 passed"}) {
		t.Errorf("open revision: %d calls, readings %+v", len(*calls), got)
	}
}

// TestAnUnknownStateIsShownOpen: a state outside open, settled and
// contested is not shown, on that criterion alone.
func TestAnUnknownStateIsShownOpen(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	eval, _ := counting(func(EvalQuery) ([]Verdict, error) {
		v := verdicts(ref, StateSettled, "CI run 88 passed")
		v[1].State = "SETTLED"
		return v, nil
	})
	got := Readings(context.Background(), key42, src42(), evidenceRef, eval)
	want := []CriterionReading{
		{ID: ref.Criteria[0].ID, State: StateSettled, Because: "CI run 88 passed"},
		{ID: ref.Criteria[1].ID, State: StateOpen},
		{ID: ref.Criteria[2].ID, State: StateSettled, Because: "CI run 88 passed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Readings = %+v, want %+v", got, want)
	}
}

// TestOneEvaluateCallPerKey (re-review b08-1 R-I1).
func TestOneEvaluateCallPerKey(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "the caller's")
	var seen any
	var calls []EvalQuery
	eval := func(c context.Context, q EvalQuery) ([]Verdict, error) {
		seen, calls = c.Value(ctxKey{}), append(calls, q)
		return verdicts(ref, StateOpen), nil
	}
	Readings(ctx, key42, src42(), evidenceRef, eval)
	if len(calls) != 1 {
		t.Fatalf("%d Evaluate calls for one key, want 1", len(calls))
	}
	want := EvalQuery{BriefKey: key42, ProjectID: hubProject, BranchRef: evidenceRef, Criteria: ref.Criteria}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("query = %+v\nwant %+v", calls[0], want)
	}
	if seen != "the caller's" {
		t.Errorf("Evaluate saw ctx value %v, want the caller's ctx", seen)
	}
}

// TestEvaluateFailureShowsOpen (re-review b08-1 R-I1).
func TestEvaluateFailureShowsOpen(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	good := func() []Verdict { return verdicts(ref, StateSettled, "CI run 88 passed") }
	for _, tc := range []struct {
		name string
		fn   func(EvalQuery) ([]Verdict, error)
	}{
		{"an error", func(EvalQuery) ([]Verdict, error) { return good(), errors.New("git: no such ref") }},
		{"two verdicts for three", func(EvalQuery) ([]Verdict, error) { return good()[:2], nil }},
		{"four verdicts for three", func(EvalQuery) ([]Verdict, error) { v := good(); return append(v, v[0]), nil }},
		{"none", func(EvalQuery) ([]Verdict, error) { return nil, nil }},
		{"out of order", func(EvalQuery) ([]Verdict, error) { v := good(); v[0], v[1] = v[1], v[0]; return v, nil }},
		{"a stranger's id", func(EvalQuery) ([]Verdict, error) { v := good(); v[2].CriterionID = "c-ffffffff"; return v, nil }},
		// Mutant R2-P1: an open verdict's because is never shown.
		{"open verdicts that give a because", func(EvalQuery) ([]Verdict, error) { return verdicts(ref, StateOpen, "no CI run yet"), nil }},
		// Mutant P2: an Evaluate that reorders the query's criteria in place
		// and answers in its new order. The readings keep Build's order.
		{"a query reordered in place", func(q EvalQuery) ([]Verdict, error) {
			slices.Reverse(q.Criteria)
			v := make([]Verdict, len(q.Criteria))
			for i, c := range q.Criteria {
				v[i] = Verdict{CriterionID: c.ID, State: StateSettled, Because: []string{"CI run 88 passed"}}
			}
			return v, nil
		}},
	} {
		eval, _ := counting(tc.fn)
		assertAllOpen(t, tc.name, ref, Readings(context.Background(), key42, src42(), evidenceRef, eval))
	}
}

// TestBecausesJoin (re-review b08-1 R-I1).
func TestBecausesJoin(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	eval, _ := counting(func(EvalQuery) ([]Verdict, error) {
		return verdicts(ref, StateContested, " CI run 88 passed ", "", "review said no"), nil
	})
	for i, r := range Readings(context.Background(), key42, src42(), evidenceRef, eval) {
		if r.ID != ref.Criteria[i].ID || r.State != StateContested || r.Because != "CI run 88 passed; review said no" {
			t.Errorf("reading %d = %+v, want contested, because %q", i, r, "CI run 88 passed; review said no")
		}
	}
}

// TestNoBranchIsNotEvaluated: Readings evaluates only at a branch its caller
// names. With none - Resolve's call for a project key passes "" - a subject
// is never guessed, and Evaluate is never called. Replaces
// TestProjectKeyIsNotEvaluated (re-review b08-2 R2-I1: the rule is the
// caller's, not the key's).
func TestNoBranchIsNotEvaluated(t *testing.T) {
	for _, tc := range []struct {
		key string
		src Source
	}{{key7, src7()}, {key42, src42()}} {
		ref := mustBuild(t, tc.key, tc.src, Head{})
		for _, at := range []string{"", "main", "refs/heads/", "refs/tags/v1.5.2", "heads/evidence-upload", "refs/remotes/origin/evidence-upload"} {
			eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return verdicts(ref, StateSettled, "CI run 88 passed"), nil })
			assertAllOpen(t, fmt.Sprintf("%s at %q", tc.key, at), ref, Readings(context.Background(), tc.key, tc.src, at, eval))
			if len(*calls) != 0 {
				t.Errorf("%s at %q: Evaluate called %d times, want never", tc.key, at, len(*calls))
			}
		}
	}
}

// TestAProjectKeyIsEvaluatedAtTheCallersBranch (re-review b08-2 R2-I1): a
// branch with no complete branch brief inherits the project key, and its
// head reading (HeadRef(t), task 07) shows verdicts at t's head.
func TestAProjectKeyIsEvaluatedAtTheCallersBranch(t *testing.T) {
	ref := mustBuild(t, key7, src7(), Head{})
	eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return verdicts(ref, StateSettled, "CI run 88 passed"), nil })
	got := Readings(context.Background(), key7, src7(), "refs/heads/fix-session-timeout", eval)
	want := EvalQuery{BriefKey: key7, ProjectID: hubProject, BranchRef: "refs/heads/fix-session-timeout", Criteria: ref.Criteria}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Fatalf("calls = %+v, want exactly %+v", *calls, want)
	}
	for i, r := range got {
		if r != (CriterionReading{ID: ref.Criteria[i].ID, State: StateSettled, Because: "CI run 88 passed"}) {
			t.Errorf("reading %d = %+v", i, r)
		}
	}
}

// TestTheBranchEvaluatedIsTheCallers (re-review b08-2 R2-M1): a rebind writes
// no revision, so b-42@5's envelope still says evidence-upload after the
// branch is renamed. The subject is the branch the caller names now.
func TestTheBranchEvaluatedIsTheCallers(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return verdicts(ref, StateOpen), nil })
	Readings(context.Background(), key42, src42(), "refs/heads/evidence-upload-v2", eval)
	if len(*calls) != 1 || (*calls)[0].BranchRef != "refs/heads/evidence-upload-v2" {
		t.Errorf("calls = %+v, want one at refs/heads/evidence-upload-v2, not the envelope's %s", *calls, evidenceRef)
	}
}

// TestReadingsOfAKeyWithNoCriteria: nothing to evaluate is no call and no
// reading; a source Build refuses has no criteria to read either.
func TestReadingsOfAKeyWithNoCriteria(t *testing.T) {
	src := src42()
	src.Revision.Content.Answers[3] = skip(discover.KeyDone)
	src.Project.Content.Answers[3] = skip(discover.KeyDone)
	eval, calls := counting(func(EvalQuery) ([]Verdict, error) { return nil, nil })
	if got := Readings(context.Background(), key42, src, evidenceRef, eval); got == nil || len(got) != 0 || len(*calls) != 0 {
		t.Errorf("no criteria: readings %#v, %d calls, want [] and none", got, len(*calls))
	}
	bad := src42()
	bad.Project = nil
	if got := Readings(context.Background(), key42, bad, evidenceRef, eval); got == nil || len(got) != 0 || len(*calls) != 0 {
		t.Errorf("refused source: readings %#v, %d calls, want [] and none", got, len(*calls))
	}
}

// jsonKeys walks a JSON value and calls fn with each object key and the
// path of object keys above it.
func jsonKeys(v any, path []string, fn func(path []string, k string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			fn(path, k)
			jsonKeys(child, append(append([]string{}, path...), k), fn)
		}
	case []any:
		for _, child := range x {
			jsonKeys(child, path, fn)
		}
	}
}

func decode(t *testing.T, ref Ref) any {
	t.Helper()
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestRefHoldsNoCriterionState (review I14).
func TestRefHoldsNoCriterionState(t *testing.T) {
	ref := mustBuild(t, key42, src42(), head42)
	jsonKeys(decode(t, ref), nil, func(path []string, k string) {
		if len(path) > 0 && path[0] == "criteria" && (k == "state" || k == "because") {
			t.Errorf("the Ref's criteria hold %q at %v", k, path)
		}
	})
	answers := []CriterionState{StateSettled, StateContested}
	n := 0
	eval := func(context.Context, EvalQuery) ([]Verdict, error) {
		v := verdicts(ref, answers[n], "review said no")
		n++
		return v, nil
	}
	first := Readings(context.Background(), key42, src42(), evidenceRef, eval)
	before, _ := json.Marshal(mustBuild(t, key42, src42(), head42))
	second := Readings(context.Background(), key42, src42(), evidenceRef, eval)
	after, _ := json.Marshal(mustBuild(t, key42, src42(), head42))
	if reflect.DeepEqual(first, second) || first[0].State != StateSettled || second[0].State != StateContested {
		t.Errorf("readings did not move with the verdicts: %+v then %+v", first, second)
	}
	if string(before) != string(after) {
		t.Error("Build's JSON moved with the verdicts")
	}
}

// TestRefHasNoDenominator (G5): no count in any Ref the package makes.
func TestRefHasNoDenominator(t *testing.T) {
	bad := regexp.MustCompile(`(?i)met|total|percent|ratio|count`)
	for name, ref := range map[string]Ref{
		"built": mustBuild(t, key42, src42(), head42), "missing": MissingRef(key42), "invalid": InvalidRef("b-42"),
		"unavailable": UnavailableRef(key42), "none": NoneRef(Declined),
		"erased": ErasedRef(key42, branchBrief, 5, dig42, stamp, "Dylan Mackie", stamp, head42),
	} {
		jsonKeys(decode(t, ref), nil, func(path []string, k string) {
			if bad.MatchString(k) {
				t.Errorf("%s Ref has a denominator-shaped key %q at %v", name, k, path)
			}
		})
	}
}

// TestHeadsAreCarriedAsGiven: a head at an open revision above the latest
// complete one tells headRevision from latestCompleteRevision, which
// superseded is derived from (P4-B; mutants R2-P2, R2-P3: the two swapped).
func TestHeadsAreCarriedAsGiven(t *testing.T) {
	h := Head{HeadRevision: 7, LatestCompleteRevision: 6, Archived: true}
	for name, ref := range map[string]Ref{
		"built":  mustBuild(t, key42, src42(), h),
		"erased": ErasedRef(key42, branchBrief, 5, dig42, stamp, "Dylan Mackie", stamp, h),
	} {
		if ref.HeadRevision != 7 || ref.LatestCompleteRevision != 6 {
			t.Errorf("%s: headRevision %d latestCompleteRevision %d, want 7 and 6", name, ref.HeadRevision, ref.LatestCompleteRevision)
		}
	}
}

func TestErasedKeepsKeyAndDigest(t *testing.T) {
	ref := ErasedRef(key42, branchBrief, 5, dig42, "2026-09-22T09:12:00.000Z", "Dylan Mackie", "2026-09-25T08:00:00.000Z", head42)
	want := Ref{Key: key42, Resolution: Erased, Present: true, BriefID: branchBrief, Revision: 5, Digest: dig42,
		HeadRevision: 6, LatestCompleteRevision: 6, Sections: []discover.Section{}, Criteria: []Criterion{},
		SavedAt: "2026-09-22T09:12:00.000Z", SavedBy: "Dylan Mackie", ErasedAt: "2026-09-25T08:00:00.000Z"}
	if !reflect.DeepEqual(ref, want) {
		t.Errorf("ErasedRef =\n%+v\nwant\n%+v", ref, want)
	}
}

// TestSlotRefs: the Refs a slot gets when there is no revision to build.
// A brief key's id and revision are filled from the key, so the desktop, which
// never parses a key (P5), can still say which revision it is; nothing else is.
func TestSlotRefs(t *testing.T) {
	bare := func(key string, r Resolution, id string, rev int) Ref {
		return Ref{Key: key, Resolution: r, BriefID: id, Revision: rev, Sections: []discover.Section{}, Criteria: []Criterion{}}
	}
	for name, tc := range map[string]struct{ got, want Ref }{
		"missing":            {MissingRef(key42), bare(key42, Missing, branchBrief, 5)},
		"unavailable":        {UnavailableRef(key42), bare(key42, Unavailable, branchBrief, 5)},
		"missing design":     {MissingRef("design:cv-7c01d2e3f4a5b6c7@11"), bare("design:cv-7c01d2e3f4a5b6c7@11", Missing, "", 0)},
		"unavailable design": {UnavailableRef("design:cv-7c01d2e3f4a5b6c7@11"), bare("design:cv-7c01d2e3f4a5b6c7@11", Unavailable, "", 0)},
		"invalid":            {InvalidRef("b-42"), bare("b-42", Invalid, "", 0)},
		"invalid brief form": {InvalidRef("brief:bf-3f2a91c07b1e4d2a@0"), bare("brief:bf-3f2a91c07b1e4d2a@0", Invalid, "", 0)},
		"none":               {NoneRef(""), bare("", None, "", 0)},
		"declined": {NoneRef(Declined), func() Ref {
			r := bare(Declined, None, "", 0)
			r.NoneReason = "declined"
			return r
		}()},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s =\n%+v\nwant\n%+v", name, tc.got, tc.want)
		}
		raw, _ := json.Marshal(tc.got)
		for _, frag := range []string{`"sections":[]`, `"criteria":[]`, `"inherits":null`, `"present":false`} {
			if !strings.Contains(string(raw), frag) {
				t.Errorf("%s JSON lacks %s: %s", name, frag, raw)
			}
		}
	}
}

// TestRefGoldenJSON: the fact sheet's b-42@5, built, is the wire contract
// task 15 mirrors field for field. testdata/.gitattributes is * -text, so
// the checkout never converts it.
func TestRefGoldenJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "ref-golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\r") {
		t.Fatal("testdata/ref-golden.json holds a \\r: the checkout converted it")
	}
	got, err := json.MarshalIndent(mustBuild(t, key42, src42(), head42), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(got)+"\n" != string(raw) {
		t.Errorf("Build(b-42@5) JSON differs from testdata/ref-golden.json:\n%s", got)
	}
}

// TestReadingAndHeadJSON: the two moving shapes task 12 serves beside a Ref
// (RefsResponse.criterionStates and heads) are wire contract too, and the
// golden holds only the Ref (mutants R7-P1, R7-P2: a tag renamed on one side).
func TestReadingAndHeadJSON(t *testing.T) {
	eval := func(_ context.Context, q EvalQuery) ([]Verdict, error) {
		vs := make([]Verdict, len(q.Criteria))
		for i, c := range q.Criteria {
			vs[i] = Verdict{CriterionID: c.ID, State: StateOpen}
		}
		vs[0].State, vs[0].Because = StateSettled, []string{"CI run 88 passed"}
		return vs, nil
	}
	got, err := json.Marshal(Readings(context.Background(), key42, src42(), evidenceRef, eval))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"c-a0000001","state":"settled","because":"CI run 88 passed"},` +
		`{"id":"c-a0000002","state":"open","because":""},{"id":"c-a0000003","state":"open","because":""}]`
	if string(got) != want {
		t.Errorf("Readings JSON =\n%s\nwant\n%s", got, want)
	}
	head, err := json.Marshal(Head{HeadRevision: 7, LatestCompleteRevision: 6, Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"headRevision":7,"latestCompleteRevision":6,"archived":true}`; string(head) != want {
		t.Errorf("Head JSON = %s, want %s", head, want)
	}
}
