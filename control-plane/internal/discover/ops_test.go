package discover

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRecordTrimsSetsThePersonAndStamps(t *testing.T) {
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyAudience, Text: "  Client users \n", Source: SourcePicked}, projectEnv())
	want := Answer{Key: KeyAudience, Text: "Client users", Source: SourcePicked, Author: AuthorPerson,
		Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
	if len(c.Answers) != 1 || !reflect.DeepEqual(c.Answers[0], want) {
		t.Fatalf("answers = %#v, want [%#v]", c.Answers, want)
	}
	if c.Change != (Change{Kind: ChangeRecord, Key: KeyAudience}) || c.V != 1 || c.Status != StatusOpen {
		t.Errorf("content = v%d %q %#v", c.V, c.Status, c.Change)
	}
	// A second key appends after the first, whatever the question order.
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyPurpose, Text: "p", Source: SourceTyped}, projectEnv())
	if c.Answers[0].Key != KeyAudience || c.Answers[1].Key != KeyPurpose {
		t.Errorf("answer order = %q, %q; want audience then purpose (append)", c.Answers[0].Key, c.Answers[1].Key)
	}
}

func TestRecordRefusals(t *testing.T) {
	base := NewContent()
	for name, tc := range map[string]struct {
		op   Op
		want error
	}{
		"empty":           {Op{Kind: OpRecord, Key: KeyPurpose, Text: "", Source: SourceTyped}, ErrEmptyAnswer},
		"blank":           {Op{Kind: OpRecord, Key: KeyPurpose, Text: " \t\r\n ", Source: SourceTyped}, ErrEmptyAnswer},
		"unknown key":     {Op{Kind: OpRecord, Key: "nope", Text: "x", Source: SourceTyped}, ErrUnknownKey},
		"drafted source":  {Op{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceDrafted}, ErrBadOp},
		"no source":       {Op{Kind: OpRecord, Key: KeyPurpose, Text: "x"}, ErrBadOp},
		"done, no items":  {Op{Kind: OpRecord, Key: KeyDone, Text: " ; ;\n", Source: SourceTyped}, ErrEmptyAnswer},
		"unknown op kind": {Op{Kind: "rename", Key: KeyPurpose, Text: "x", Source: SourceTyped}, ErrBadOp},
		"empty op kind":   {Op{Key: KeyPurpose, Text: "x", Source: SourceTyped}, ErrBadOp},
	} {
		if _, err := Apply(base, tc.op, projectEnv()); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestEditChangesOneAnswerAndKeepsTheRest(t *testing.T) {
	env := projectEnv()
	before := allAnswered(t, env)
	after := mustApply(t, before, Op{Kind: OpEdit, Key: KeyAudience, Text: "Client users", Source: SourcePicked}, env)

	if len(after.Answers) != 5 {
		t.Fatalf("answers = %d, want 5 (the prototype's splice lost the tail)", len(after.Answers))
	}
	for i := range before.Answers {
		if i == 1 {
			continue
		}
		if !reflect.DeepEqual(after.Answers[i], before.Answers[i]) {
			t.Errorf("answer %d changed: %#v -> %#v", i, before.Answers[i], after.Answers[i])
		}
	}
	got := after.Answers[1]
	if got.Key != KeyAudience || got.Text != "Client users" || got.Source != SourceTyped || got.Author != AuthorPerson {
		t.Errorf("edited answer = %#v, want audience, the new text, typed (edit is always typed), person", got)
	}
	if after.Change != (Change{Kind: ChangeEdit, Key: KeyAudience}) || after.Status != StatusComplete {
		t.Errorf("change %#v, status %q", after.Change, after.Status)
	}
}

func TestEditNeedsASettledTypedOrPickedAnswer(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpSkip, Key: KeyAudience}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyDone, Text: "Tests pass"}}}, env)
	for _, k := range []Key{KeyPurpose, KeyAudience, KeyDone} { // none, skipped, drafted
		if _, err := Apply(c, Op{Kind: OpEdit, Key: k, Text: "y"}, env); !errors.Is(err, ErrNotAnswered) {
			t.Errorf("edit of %s: err = %v, want ErrNotAnswered", k, err)
		}
	}
	if _, err := Apply(c, Op{Kind: OpEdit, Key: "nope", Text: "y"}, env); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("edit of an unknown key: err = %v", err)
	}
	// An accepted draft is settled, so it can be edited, and becomes the person's.
	c = mustApply(t, c, Op{Kind: OpAccept, Key: KeyDone}, env)
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "Tests pass; Reviewed"}, env)
	if a, _ := c.AnswerFor(KeyDone); a.Author != AuthorPerson || a.Source != SourceTyped || len(a.DraftedFrom) != 0 {
		t.Errorf("edited accepted draft = %#v, want person, typed, draftedFrom []", a)
	}
	if _, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: "  "}, env); !errors.Is(err, ErrNotAnswered) {
		t.Errorf("an edit of an unanswered question with blank text: err = %v, want ErrNotAnswered first", err)
	}
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyPurpose, Text: "p", Source: SourceTyped}, env)
	if _, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: "  "}, env); !errors.Is(err, ErrEmptyAnswer) {
		t.Errorf("an edit to blank text: err = %v, want ErrEmptyAnswer", err)
	}
}

func TestSkipIsSettledAndCompletesTheBrief(t *testing.T) {
	env := branchEnv(nil)
	c := NewContent()
	for _, k := range []Key{KeyPurpose, KeyAudience, KeyConstraints, KeyDone} {
		c = mustApply(t, c, Op{Kind: OpRecord, Key: k, Text: "x", Source: SourceTyped}, env)
	}
	if c.Status != StatusOpen {
		t.Fatalf("four answers: status = %q, want open", c.Status)
	}
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyOutOfScope}, env)
	if c.Status != StatusComplete {
		t.Errorf("four answers and a skip: status = %q, want complete", c.Status)
	}
	want := Answer{Key: KeyOutOfScope, Skipped: true, Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
	if a, _ := c.AnswerFor(KeyOutOfScope); !reflect.DeepEqual(a, want) {
		t.Errorf("skipped answer = %#v, want %#v", a, want)
	}
	if c.Change != (Change{Kind: ChangeSkip, Key: KeyOutOfScope}) {
		t.Errorf("change = %#v", c.Change)
	}
	// Skipping an answered done drops its text and its criteria, in place.
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyDone}, env)
	if a := c.Answers[3]; a.Key != KeyDone || !a.Skipped || a.Text != "" || a.Source != "" || len(a.Criteria) != 0 || a.Criteria == nil {
		t.Errorf("skipped done = %#v", a)
	}
	if _, err := Apply(c, Op{Kind: OpSkip, Key: "nope"}, env); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("skip of an unknown key: err = %v, want ErrUnknownKey", err)
	}
	if _, err := Apply(c, Op{Kind: OpSkip, Key: KeyDone}, env); !errors.Is(err, ErrNoChange) {
		t.Errorf("skip of a skipped answer: err = %v, want ErrNoChange", err)
	}
	// Recording over a skip un-skips it.
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyOutOfScope, Text: "Perf", Source: SourceTyped}, env)
	if a, _ := c.AnswerFor(KeyOutOfScope); a.Skipped || a.Text != "Perf" {
		t.Errorf("record over a skip = %#v", a)
	}
}

func TestStatusIsDerivedNeverCarried(t *testing.T) {
	env := projectEnv()
	c := allAnswered(t, env)
	lie := c
	lie.Status = StatusOpen // a head whose stored status disagrees with its answers
	lie.V = 0
	out := mustApply(t, lie, Op{Kind: OpRecord, Key: KeyPurpose, Text: "again", Source: SourceTyped}, env)
	if out.Status != StatusComplete || out.V != ContentVersion {
		t.Errorf("status %q v%d, want complete v1 from the answers", out.Status, out.V)
	}
	open := NewContent()
	open.Status = StatusComplete
	out = mustApply(t, open, Op{Kind: OpSkip, Key: KeyPurpose}, env)
	if out.Status != StatusOpen {
		t.Errorf("status %q, want open: four questions have no answer", out.Status)
	}
	// Restoring a complete revision completes the brief; restoring an open one reopens it.
	out = mustApply(t, NewContent(), Op{Kind: OpRestore, RestoreRevision: 5, Restore: &c}, env)
	if out.Status != StatusComplete {
		t.Errorf("restore of a complete revision: status %q", out.Status)
	}
	back := NewContent()
	out = mustApply(t, c, Op{Kind: OpRestore, RestoreRevision: 1, Restore: &back}, env)
	if out.Status != StatusOpen || len(out.Answers) != 0 || out.Answers == nil {
		t.Errorf("restore of an empty revision: status %q, answers %#v", out.Status, out.Answers)
	}
}

func TestAnEnvWithNoScopeIsRefused(t *testing.T) {
	env := projectEnv()
	env.Scope = ""
	_, err := Apply(NewContent(), Op{Kind: OpSkip, Key: KeyPurpose}, env)
	if err == nil || errors.Is(err, ErrBadOp) {
		t.Errorf("err = %v, want a server fault, not a 400 that blames the request", err)
	}
}

// A nil NewID is the server's fault, not the person's (review M-2): every op
// refuses it, one that mints nothing included, and the pure package never
// falls back to a random generator of its own.
func TestAnEnvWithNoNewIDIsAServerFault(t *testing.T) {
	env := projectEnv()
	env.NewID = nil
	for name, op := range map[string]Op{
		"skip, which mints nothing":   {Kind: OpSkip, Key: KeyPurpose},
		"record of done, which mints": {Kind: OpRecord, Key: KeyDone, Text: "A", Source: SourceTyped},
		"add_follow_up, which mints":  {Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "More?"}},
		"reinherit on a project":      {Kind: OpReinherit}, // the fault outranks the op's own ErrBadOp
	} {
		got, err := Apply(NewContent(), op, env)
		if !errors.Is(err, errNoNewID) || errors.Is(err, ErrBadOp) {
			t.Errorf("%s: err = %v, want errNoNewID (a 500), not a 400 that blames the request", name, err)
		}
		if !reflect.DeepEqual(got, Content{}) {
			t.Errorf("%s: value beside the error = %#v, want the zero Content", name, got)
		}
	}
}

// A content with no questions is never complete (review I-1): it can only
// come from a head or revision written straight to route 26, and a complete
// one would become the latest complete revision every branch pins. Step
// agrees with the status: nothing to ask, and not complete.
func TestAContentWithNoQuestionsIsNeverComplete(t *testing.T) {
	orphan := Answer{Key: KeyPurpose, Text: "settled, but no question asks it", Source: SourceTyped,
		Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp}
	for name, tc := range map[string]struct {
		c   Content
		op  Op
		env Env
	}{
		"restore of {v:1}":   {NewContent(), Op{Kind: OpRestore, RestoreRevision: 1, Restore: &Content{V: 1}}, projectEnv()},
		"reinherit of {v:1}": {Content{V: 1}, Op{Kind: OpReinherit}, branchEnv(projRev2)},
		// Answers with no questions: a guard keyed on the answers passes these.
		"restore of an answer with no question":   {NewContent(), Op{Kind: OpRestore, RestoreRevision: 1, Restore: &Content{V: 1, Answers: []Answer{orphan}}}, projectEnv()},
		"reinherit of an answer with no question": {Content{V: 1, Answers: []Answer{orphan}}, Op{Kind: OpReinherit}, branchEnv(projRev2)},
	} {
		out := mustApply(t, tc.c, tc.op, tc.env)
		if len(out.Questions) != 0 || out.Status != StatusOpen {
			t.Errorf("%s: %d questions, status %q; want 0 and open", name, len(out.Questions), out.Status)
		}
		s := out.Step(ScopeBranch)
		if s.Kind != StepReview || s.Pending == nil || len(s.Pending) != 0 || s.Options == nil || s.Key != "" {
			t.Errorf("%s: step = %#v, want review with nothing pending: never complete while the status is open", name, s)
		}
	}
}

// On any error Apply returns the zero Content (as built, task 03), so a
// caller that reads the value before the error has nothing half-applied to
// write.
func TestApplyReturnsTheZeroContentOnAnyError(t *testing.T) {
	env := projectEnv()
	full := allAnswered(t, env)
	purpose, _ := full.AnswerFor(KeyPurpose)
	noScope := projectEnv()
	noScope.Scope = ""
	for name, tc := range map[string]struct {
		c    Content
		op   Op
		env  Env
		want error // nil: an unexported server fault
	}{
		"bad op":                    {full, Op{Kind: OpRecord, Key: KeyPurpose, Text: "x"}, env, ErrBadOp},
		"a draft that fails midway": {NewContent(), Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyPurpose, Text: "p"}, {Key: "nope", Text: "x"}}}, env, ErrUnknownKey},
		"reserved text":             {full, Op{Kind: OpEdit, Key: KeyPurpose, Text: "see panacea:discover: here"}, env, ErrReservedText},
		"too long":                  {full, Op{Kind: OpEdit, Key: KeyPurpose, Text: strings.Repeat("a", MaxAnswerRunes+1)}, env, ErrTooLong},
		"no change":                 {full, Op{Kind: OpRecord, Key: KeyPurpose, Text: purpose.Text, Source: purpose.Source}, env, ErrNoChange},
		"no scope":                  {full, Op{Kind: OpSkip, Key: KeyPurpose}, noScope, nil},
	} {
		got, err := Apply(tc.c, tc.op, tc.env)
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
			continue
		}
		if !reflect.DeepEqual(got, Content{}) {
			t.Errorf("%s: value beside the error = %#v, want the zero Content", name, got)
		}
	}
}

// A forged answer that is neither settled nor a draft (no author) is asked
// again: the step and the derived status never disagree.
func TestStepAndStatusAgreeOnAnAnswerWithNoAuthor(t *testing.T) {
	c := allAnswered(t, projectEnv())
	c.Answers[1].Author = ""
	if s := c.Step(ScopeProject); s.Kind != StepQuestion || s.Key != KeyAudience || s.Ordinal != 2 {
		t.Errorf("step = %#v, want question 2 audience", s)
	}
	out := mustApply(t, c, Op{Kind: OpSkip, Key: KeyOutOfScope}, projectEnv())
	if out.Status != StatusOpen || out.Step(ScopeProject).Kind == StepComplete {
		t.Errorf("status %q, step %q: want open and not complete", out.Status, out.Step(ScopeProject).Kind)
	}
}

func TestAFollowUpKeyNeverMatchesAnExistingAnswer(t *testing.T) {
	c := NewContent()
	c.Answers = []Answer{{Key: "f-00000001", Text: "orphan", Author: AuthorPerson, Source: SourceTyped}} // a forged head
	out := mustApply(t, c, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "More?"}}, projectEnv())
	if k := out.Questions[5].Key; k != "f-00000002" {
		t.Errorf("follow-up key = %q, want f-00000002: f-00000001 already keys an answer", k)
	}
}

func TestADraftsDraftedFromIsCopied(t *testing.T) {
	drafts := []Answer{{Key: KeyPurpose, Text: "p", DraftedFrom: []string{"go.mod", "cmd"}}}
	out := mustApply(t, NewContent(), Op{Kind: OpDraft, Drafts: drafts}, projectEnv())
	out.Answers[0].DraftedFrom[0] = "scribble"
	if drafts[0].DraftedFrom[0] != "go.mod" {
		t.Errorf("the result's DraftedFrom aliases the op's")
	}
}

func TestADraftedAnswerIsNotSettled(t *testing.T) {
	env := branchEnv(nil)
	c := NewContent()
	for _, k := range []Key{KeyPurpose, KeyAudience, KeyConstraints, KeyOutOfScope} {
		c = mustApply(t, c, Op{Kind: OpRecord, Key: k, Text: "x", Source: SourceTyped}, env)
	}
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyDone, Text: "Tests pass"}}}, env)
	if len(c.Answers) != 5 || c.Status != StatusOpen {
		t.Errorf("five answers, one model_drafted: %d answers, status %q; want open", len(c.Answers), c.Status)
	}
}

func TestAcceptChangesTheAuthorAndNothingElse(t *testing.T) {
	env := projectEnv()
	before := mustApply(t, NewContent(), Op{Kind: OpDraft, Drafts: []Answer{
		{Key: KeyDone, Text: "Tests pass\nReviewed", DraftedFrom: []string{"package.json"}},
	}}, env)
	later := env
	later.Now = clock.Add(90 * time.Second)
	after := mustApply(t, before, Op{Kind: OpAccept, Key: KeyDone}, later)

	b, a := before.Answers[0], after.Answers[0]
	if a.Author != AuthorModelAccepted {
		t.Fatalf("author = %q, want model_accepted", a.Author)
	}
	b.Author = AuthorModelAccepted
	if !reflect.DeepEqual(a, b) {
		t.Errorf("accept changed more than the author:\n %#v\nwas\n %#v", a, before.Answers[0])
	}
	if reflect.DeepEqual(after.Answers, before.Answers) {
		t.Errorf("accepted content equals the drafted content: no new digest")
	}
	if after.Change != (Change{Kind: ChangeAccept, Key: KeyDone}) {
		t.Errorf("change = %#v", after.Change)
	}
}

func TestAcceptOfAPersonsAnswerIsRefused(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "mine", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyAudience}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyDone, Text: "d"}}}, env)
	c = mustApply(t, c, Op{Kind: OpAccept, Key: KeyDone}, env)
	for _, k := range []Key{KeyPurpose, KeyAudience, KeyDone, KeyConstraints} { // typed, skipped, accepted, none
		if _, err := Apply(c, Op{Kind: OpAccept, Key: k}, env); !errors.Is(err, ErrNotDrafted) {
			t.Errorf("accept of %s: err = %v, want ErrNotDrafted", k, err)
		}
	}
	if _, err := Apply(c, Op{Kind: OpAccept, Key: "nope"}, env); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("accept of an unknown key: err = %v, want ErrUnknownKey", err)
	}
}

func criteriaOf(t *testing.T, c Content) []Criterion {
	t.Helper()
	a, ok := c.AnswerFor(KeyDone)
	if !ok {
		t.Fatalf("no done answer")
	}
	return a.Criteria
}

func TestCriterionIdsSurviveAnInsertion(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "A\nB", Source: SourceTyped}, env)
	first := criteriaOf(t, c)
	if !reflect.DeepEqual(first, []Criterion{{ID: "c-00000001", Text: "A"}, {ID: "c-00000002", Text: "B"}}) {
		t.Fatalf("criteria = %#v", first)
	}
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "A\nX\nB"}, env)
	got := criteriaOf(t, c)
	want := []Criterion{{ID: "c-00000001", Text: "A"}, {ID: "c-00000003", Text: "X"}, {ID: "c-00000002", Text: "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after an insertion criteria = %#v, want %#v", got, want)
	}
	// A reorder keeps every id and follows the new text's order.
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "- B\n- A; X"}, env)
	want = []Criterion{{ID: "c-00000002", Text: "B"}, {ID: "c-00000001", Text: "A"}, {ID: "c-00000003", Text: "X"}}
	if got := criteriaOf(t, c); !reflect.DeepEqual(got, want) {
		t.Errorf("after a reorder criteria = %#v, want %#v", got, want)
	}
	// Only the done answer has criteria.
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyConstraints, Text: "A\nB", Source: SourceTyped}, env)
	if a, _ := c.AnswerFor(KeyConstraints); len(a.Criteria) != 0 {
		t.Errorf("constraints has criteria %#v", a.Criteria)
	}
}

func TestCriterionIdsAreNotReusedForChangedText(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "A", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "A!"}, env)
	if got := criteriaOf(t, c); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000002", Text: "A!"}}) {
		t.Errorf("changed text criteria = %#v, want a new id", got)
	}
	// A duplicate line uses the old id once; the second copy gets a new one.
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "A!\nA!"}, env)
	if got := criteriaOf(t, c); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000002", Text: "A!"}, {ID: "c-00000003", Text: "A!"}}) {
		t.Errorf("duplicate lines criteria = %#v", got)
	}
	// A skip drops the ids; recording the same text again mints new ones.
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyDone}, env)
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyDone, Text: "A!", Source: SourceTyped}, env)
	if got := criteriaOf(t, c); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000004", Text: "A!"}}) {
		t.Errorf("after a skip criteria = %#v, want a fresh id", got)
	}
}

func TestMintedIdsNeverCollide(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "A", Source: SourceTyped}, env)
	// A generator that repeats itself: the id already in use is never minted twice.
	calls := 0
	env.NewID = func(prefix string) string {
		calls++
		if calls <= 2 {
			return prefix + "-00000001"
		}
		return prefix + "-0000000f"
	}
	c = mustApply(t, c, Op{Kind: OpEdit, Key: KeyDone, Text: "A\nB"}, env)
	if got := criteriaOf(t, c); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000001", Text: "A"}, {ID: "c-0000000f", Text: "B"}}) {
		t.Errorf("criteria = %#v, want the repeated id refused", got)
	}
	// A dropped criterion's id is never minted again either (R6): evidence
	// recorded against "B" must not attach to "C".
	d := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "A\nB", Source: SourceTyped}, projectEnv())
	if got := criteriaOf(t, d); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000001", Text: "A"}, {ID: "c-00000002", Text: "B"}}) {
		t.Fatalf("criteria = %#v", got)
	}
	dropped := projectEnv()
	next := []string{"c-00000002", "c-00000009"}
	dropped.NewID = func(prefix string) string {
		id := next[0]
		next = next[1:]
		return id
	}
	d = mustApply(t, d, Op{Kind: OpEdit, Key: KeyDone, Text: "C"}, dropped)
	if got := criteriaOf(t, d); !reflect.DeepEqual(got, []Criterion{{ID: "c-00000009", Text: "C"}}) {
		t.Errorf("criteria = %#v, want c-00000009: c-00000002 was B's, which the edit dropped", got)
	}
	calls = 0
	env.NewID = func(prefix string) string { return prefix + "-00000001" }
	c = mustApply(t, c, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "One?"}}, env)
	if _, err := Apply(c, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "Two?"}}, env); err == nil {
		t.Errorf("a generator stuck on one id added a second follow-up with a duplicate key")
	}
}

func TestAddFollowUpMintsTheKeyAndAppends(t *testing.T) {
	env := branchEnv(nil)
	c := allAnswered(t, env) // uses c-00000001 for the done answer
	if c.Status != StatusComplete {
		t.Fatalf("status = %q", c.Status)
	}
	in := &Question{Key: "chosen-by-caller", BranchPrompt: "How does it ship?", Options: nil}
	c = mustApply(t, c, Op{Kind: OpAddFollowUp, Question: in}, env)
	if len(c.Questions) != 6 {
		t.Fatalf("questions = %d, want 6", len(c.Questions))
	}
	q := c.Questions[5]
	if q.Key != "f-00000002" || !q.FollowUp || q.Options == nil || q.BranchPrompt != "How does it ship?" {
		t.Errorf("follow-up = %#v, want key f-00000002, followUp, options []", q)
	}
	if !reflect.DeepEqual(c.Questions[:5], BaseQuestions()) {
		t.Errorf("the base questions moved")
	}
	if c.Status != StatusOpen || c.Change != (Change{Kind: ChangeAddFollowUp, Key: "f-00000002"}) {
		t.Errorf("status %q, change %#v; a new question reopens the brief", c.Status, c.Change)
	}
	if in.Key != "chosen-by-caller" || in.FollowUp {
		t.Errorf("Apply wrote into the caller's question: %#v", in)
	}
	for name, bad := range map[string]Op{
		"nil question":  {Kind: OpAddFollowUp},
		"no prompt":     {Kind: OpAddFollowUp, Question: &Question{Help: "h"}},
		"blank prompt":  {Kind: OpAddFollowUp, Question: &Question{BranchPrompt: " \n "}},
		"project-only?": {Kind: OpAddFollowUp, Question: &Question{ProjectPrompt: "Only for projects?"}},
	} {
		if _, err := Apply(c, bad, env); !errors.Is(err, ErrBadOp) {
			t.Errorf("%s (branch): err = %v, want ErrBadOp", name, err)
		}
	}
	// On a project brief, project wording alone is enough.
	p := mustApply(t, NewContent(), Op{Kind: OpAddFollowUp, Question: &Question{ProjectPrompt: "Only for projects?"}}, projectEnv())
	if got := p.Step(ScopeProject); got.Kind != StepQuestion || got.Key != KeyPurpose {
		t.Errorf("project step = %#v", got)
	}
}

func TestDismissFollowUpRemovesTheQuestionAndItsAnswer(t *testing.T) {
	env := branchEnv(nil)
	c := allAnswered(t, env)
	c = mustApply(t, c, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "Ship how?"}}, env)
	fk := c.Questions[5].Key
	c = mustApply(t, c, Op{Kind: OpRecord, Key: fk, Text: "Behind a flag", Source: SourcePicked}, env)
	before := c
	c = mustApply(t, c, Op{Kind: OpDismissFollowUp, Key: fk}, env)
	if len(c.Questions) != 5 || len(c.Answers) != 5 {
		t.Fatalf("after dismiss: %d questions, %d answers; want 5 and 5", len(c.Questions), len(c.Answers))
	}
	if _, ok := c.AnswerFor(fk); ok {
		t.Errorf("the follow-up's answer survived")
	}
	if !reflect.DeepEqual(c.Answers, before.Answers[:5]) || c.Status != StatusComplete {
		t.Errorf("dismiss touched another answer, or status %q is not complete", c.Status)
	}
	if c.Change != (Change{Kind: ChangeDismissFollowUp, Key: fk}) {
		t.Errorf("change = %#v", c.Change)
	}
	if _, err := Apply(c, Op{Kind: OpDismissFollowUp, Key: KeyAudience}, env); !errors.Is(err, ErrNotFollowUp) {
		t.Errorf("dismiss of a base question: err = %v, want ErrNotFollowUp", err)
	}
	if _, err := Apply(c, Op{Kind: OpDismissFollowUp, Key: fk}, env); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("dismiss of a gone follow-up: err = %v, want ErrUnknownKey", err)
	}
}

var (
	projRev2 = &Inherit{Key: "brief:bf-p@2", Digest: "sha256:0002"}
	projRev4 = &Inherit{Key: "brief:bf-p@4", Digest: "sha256:0004"}
)

func TestRestoreKeepsTheCurrentInherits(t *testing.T) {
	// Revision 3 of a branch brief, written while the project stood at rev 2.
	rev3 := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "old", Source: SourceTyped}, branchEnv(projRev2))
	rev3 = mustApply(t, rev3, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "Ship how?"}}, branchEnv(projRev2))
	if rev3.Inherits == nil || *rev3.Inherits != *projRev2 {
		t.Fatalf("rev 3 inherits = %#v", rev3.Inherits)
	}
	// The head moved on, and so did the project (rev 4).
	head := mustApply(t, rev3, Op{Kind: OpEdit, Key: KeyPurpose, Text: "new"}, branchEnv(projRev4))
	head = mustApply(t, head, Op{Kind: OpDismissFollowUp, Key: rev3.Questions[5].Key}, branchEnv(projRev4))

	got := mustApply(t, head, Op{Kind: OpRestore, RestoreRevision: 3, Restore: &rev3}, branchEnv(projRev4))
	if got.Inherits == nil || *got.Inherits != *projRev4 {
		t.Errorf("restored inherits = %#v, want the project's rev 4 from env, not rev 3's", got.Inherits)
	}
	if !reflect.DeepEqual(got.Questions, rev3.Questions) || !reflect.DeepEqual(got.Answers, rev3.Answers) {
		t.Errorf("restore did not bring back rev 3's questions and answers")
	}
	if got.Change != (Change{Kind: ChangeRestore, Restores: 3}) {
		t.Errorf("change = %#v, want restore of 3", got.Change)
	}
	if got.Inherits == projRev4 {
		t.Errorf("result shares env's Inherits pointer")
	}
	// The result owns its slices: writing through it leaves the source alone.
	got.Answers[0].Text = "scribble"
	got.Questions[5].Options = append(got.Questions[5].Options, "scribble")
	if rev3.Answers[0].Text != "old" || len(rev3.Questions[5].Options) != 0 {
		t.Errorf("restore aliases op.Restore")
	}
	for name, bad := range map[string]Op{
		"no content":  {Kind: OpRestore, RestoreRevision: 3},
		"no revision": {Kind: OpRestore, Restore: &rev3},
	} {
		if _, err := Apply(head, bad, branchEnv(projRev4)); !errors.Is(err, ErrBadOp) {
			t.Errorf("%s: err = %v, want ErrBadOp", name, err)
		}
	}
	if _, err := Apply(head, Op{Kind: OpRestore, RestoreRevision: 9, Restore: &head}, branchEnv(projRev4)); !errors.Is(err, ErrNoChange) {
		t.Errorf("restoring the head's own answers: err = %v, want ErrNoChange", err)
	}
}

func TestProjectContentNeverInherits(t *testing.T) {
	env := projectEnv()
	env.Inherits = projRev4
	c := NewContent()
	c.Inherits = projRev2 // a forged project head
	ops := []Op{
		{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceTyped},
		{Kind: OpEdit, Key: KeyPurpose, Text: "y"},
		{Kind: OpSkip, Key: KeyAudience},
		{Kind: OpDraft, Drafts: []Answer{{Key: KeyDone, Text: "d"}}},
		{Kind: OpAccept, Key: KeyDone},
		{Kind: OpAddFollowUp, Question: &Question{ProjectPrompt: "More?"}},
	}
	for _, op := range ops {
		c = mustApply(t, c, op, env)
		if c.Inherits != nil {
			t.Errorf("%s on a project: inherits = %#v, want nil", op.Kind, c.Inherits)
		}
	}
	c = mustApply(t, c, Op{Kind: OpRestore, RestoreRevision: 1, Restore: &Content{Questions: BaseQuestions(), Answers: []Answer{}, Inherits: projRev2}}, env)
	if c.Inherits != nil {
		t.Errorf("restore on a project: inherits = %#v, want nil", c.Inherits)
	}
}

func TestEveryBranchOpPinsTheCurrentProjectRevision(t *testing.T) {
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceTyped}, branchEnv(projRev2))
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyAudience}, branchEnv(projRev4))
	if c.Inherits == nil || *c.Inherits != *projRev4 {
		t.Errorf("inherits = %#v, want rev 4", c.Inherits)
	}
	// With no complete project revision, a branch revision pins nothing.
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyConstraints}, branchEnv(nil))
	if c.Inherits != nil {
		t.Errorf("inherits = %#v, want nil", c.Inherits)
	}
	// Same text, but the project moved: a new revision, so the pin moves.
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceTyped}, branchEnv(projRev2))
	if c.Inherits == nil || *c.Inherits != *projRev2 || c.Change.Kind != ChangeRecord {
		t.Errorf("inherits = %#v, change %#v", c.Inherits, c.Change)
	}
}

func TestReinheritOnAProjectIsRefused(t *testing.T) {
	env := projectEnv()
	env.Inherits = projRev4
	if _, err := Apply(allAnswered(t, env), Op{Kind: OpReinherit}, env); !errors.Is(err, ErrBadOp) {
		t.Errorf("reinherit on a project: err = %v, want ErrBadOp", err)
	}
	b := allAnswered(t, branchEnv(projRev2))
	got := mustApply(t, b, Op{Kind: OpReinherit}, branchEnv(projRev4))
	if got.Inherits == nil || *got.Inherits != *projRev4 || got.Change != (Change{Kind: ChangeReinherit}) {
		t.Errorf("reinherit: inherits %#v, change %#v", got.Inherits, got.Change)
	}
	if !reflect.DeepEqual(got.Questions, b.Questions) || !reflect.DeepEqual(got.Answers, b.Answers) || got.Status != b.Status {
		t.Errorf("reinherit changed more than Inherits")
	}
	if _, err := Apply(got, Op{Kind: OpReinherit}, branchEnv(&Inherit{Key: projRev4.Key, Digest: projRev4.Digest})); !errors.Is(err, ErrNoChange) {
		t.Errorf("reinherit to the same revision: err = %v, want ErrNoChange", err)
	}
	if _, err := Apply(got, Op{Kind: OpReinherit}, branchEnv(&Inherit{Key: projRev4.Key, Digest: "sha256:other"})); err != nil {
		t.Errorf("reinherit to a different digest: err = %v, want a new revision", err)
	}
}

func TestDraftNeverOverwritesAPerson(t *testing.T) {
	env := branchEnv(nil)
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "Mine", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyAudience}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyConstraints, Text: "Old draft"}}}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyDone, Text: "Accepted"}}}, env)
	c = mustApply(t, c, Op{Kind: OpAccept, Key: KeyDone}, env)
	before := c

	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{
		{Key: KeyPurpose, Text: "The model's purpose"},
		{Key: KeyAudience, Text: "The model's audience"},
		{Key: KeyConstraints, Text: " New draft ", DraftedFrom: []string{"go.mod", "cmd"}, Author: AuthorPerson, Source: SourceTyped, Skipped: true},
		{Key: KeyDone, Text: "The model's done"},
		{Key: KeyOutOfScope, Text: "Perf"},
	}}, env)
	for i, k := range []Key{KeyPurpose, KeyAudience, KeyDone} {
		if !reflect.DeepEqual(c.Answers[[]int{0, 1, 3}[i]], before.Answers[[]int{0, 1, 3}[i]]) {
			t.Errorf("draft overwrote the settled %s answer: %#v", k, c.Answers[[]int{0, 1, 3}[i]])
		}
	}
	want := Answer{Key: KeyConstraints, Text: "New draft", Source: SourceDrafted, Author: AuthorModelDrafted,
		Criteria: []Criterion{}, DraftedFrom: []string{"go.mod", "cmd"}, AnsweredAt: stamp}
	if !reflect.DeepEqual(c.Answers[2], want) {
		t.Errorf("an unsettled draft is replaced in place:\n %#v\nwant\n %#v", c.Answers[2], want)
	}
	if a := c.Answers[4]; a.Key != KeyOutOfScope || a.Author != AuthorModelDrafted || a.Source != SourceDrafted || a.DraftedFrom == nil {
		t.Errorf("new draft = %#v", a)
	}
	if c.Change != (Change{Kind: ChangeDraft}) {
		t.Errorf("change = %#v", c.Change)
	}
	// A draft for settled questions only changes nothing.
	if _, err := Apply(c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyPurpose, Text: "again"}}}, env); !errors.Is(err, ErrNoChange) {
		t.Errorf("a draft over a person's answer only: err = %v, want ErrNoChange", err)
	}
	for name, tc := range map[string]struct {
		drafts []Answer
		want   error
	}{
		"unknown key": {[]Answer{{Key: "nope", Text: "x"}}, ErrUnknownKey},
		"empty text":  {[]Answer{{Key: KeyOutOfScope, Text: "  "}}, ErrEmptyAnswer},
		"done, empty": {[]Answer{{Key: KeyDone, Text: ";"}}, ErrEmptyAnswer},
	} {
		if _, err := Apply(NewContent(), Op{Kind: OpDraft, Drafts: tc.drafts}, env); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := Apply(NewContent(), Op{Kind: OpDraft}, env); !errors.Is(err, ErrNoChange) {
		t.Errorf("a draft op with no drafts: err = %v, want ErrNoChange", err)
	}
}

func TestNoChangeIsReported(t *testing.T) {
	env := branchEnv(projRev2)
	c := allAnswered(t, env)
	later := branchEnv(projRev2)
	later.Now = clock.Add(time.Hour)
	if _, err := Apply(c, Op{Kind: OpRecord, Key: KeyAudience, Text: "audience answer", Source: SourceTyped}, later); !errors.Is(err, ErrNoChange) {
		t.Errorf("re-record, same text and source, an hour later: err = %v, want ErrNoChange", err)
	}
	if _, err := Apply(c, Op{Kind: OpEdit, Key: KeyDone, Text: " done answer "}, later); !errors.Is(err, ErrNoChange) {
		t.Errorf("edit to the same text after the trim: err = %v, want ErrNoChange", err)
	}
	if _, err := Apply(c, Op{Kind: OpRecord, Key: KeyAudience, Text: "audience answer", Source: SourcePicked}, later); err != nil {
		t.Errorf("same text, new source: err = %v, want a new revision", err)
	}
}

// opCase builds one op over one content: the table rows of the marker, cap
// and line-ending tests. key is the question the refusal must name.
type opCase struct {
	name string
	c    Content
	op   Op
	key  Key
}

// textCases puts s where each op kind adds or changes a string.
func textCases(t *testing.T, s string) []opCase {
	t.Helper()
	env := projectEnv()
	answered := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "ok", Source: SourceTyped}, env)
	restore := NewContent()
	restore.Answers = []Answer{{Key: KeyConstraints, Text: s, Source: SourceTyped, Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}}}
	return []opCase{
		{"record", NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: s, Source: SourceTyped}, KeyPurpose},
		{"edit", answered, Op{Kind: OpEdit, Key: KeyPurpose, Text: s}, KeyPurpose},
		{"draft", NewContent(), Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyAudience, Text: s}}}, KeyAudience},
		{"restore", answered, Op{Kind: OpRestore, RestoreRevision: 1, Restore: &restore}, KeyConstraints},
	}
}

// promptCases puts s in a follow-up's prompts.
func promptCases(s string) []opCase {
	restore := NewContent()
	restore.Questions = append(restore.Questions, Question{Key: "f-0000beef", BranchPrompt: s, Options: []string{}, FollowUp: true})
	return []opCase{
		{"add_follow_up branch prompt", NewContent(), Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: s}}, "f-00000001"},
		{"add_follow_up project prompt", NewContent(), Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "ok?", ProjectPrompt: s}}, "f-00000001"},
		{"restore of a follow-up", NewContent(), Op{Kind: OpRestore, RestoreRevision: 1, Restore: &restore}, "f-0000beef"},
	}
}

func TestAnswerCannotContainAMarker(t *testing.T) {
	forged := []string{
		"<!-- panacea:discover:begin brief=bf-1 rev=1 -->",
		"<!-- panacea:discover:end -->",
		"we keep panacea:discover: in the middle of a sentence",
	}
	for _, s := range forged {
		for _, tc := range append(textCases(t, s), promptCases(s)...) {
			before := marshal(t, tc.c)
			_, err := Apply(tc.c, tc.op, projectEnv())
			var rt *ReservedTextError
			if !errors.Is(err, ErrReservedText) || !errors.As(err, &rt) {
				t.Errorf("%s holding %q: err = %v, want a *ReservedTextError", tc.name, s, err)
				continue
			}
			if rt.Key != string(tc.key) {
				t.Errorf("%s: refusal names %q, want %q", tc.name, rt.Key, tc.key)
			}
			if !strings.Contains(err.Error(), string(tc.key)) {
				t.Errorf("%s: message %q does not name the question", tc.name, err.Error())
			}
			if marshal(t, tc.c) != before {
				t.Errorf("%s: the input content changed", tc.name)
			}
		}
	}
	// A criterion that holds the sequence while its answer text does not (a
	// forged revision, restored) is refused too, naming done.
	head := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "A", Source: SourceTyped}, projectEnv())
	forgedRev := head
	forgedRev.Answers = []Answer{{Key: KeyDone, Text: "A", Source: SourceTyped, Author: AuthorPerson,
		Criteria: []Criterion{{ID: "c-0000beef", Text: "A panacea:discover:end"}}, DraftedFrom: []string{}}}
	_, err := Apply(head, Op{Kind: OpRestore, RestoreRevision: 1, Restore: &forgedRev}, projectEnv())
	var rt *ReservedTextError
	if !errors.As(err, &rt) || rt.Key != string(KeyDone) {
		t.Errorf("restored forged criterion: err = %v, want *ReservedTextError{done}", err)
	}
	for _, ok := range []string{"panacea:discover without the colon", "Panacea:Discover: in other case", "panacea: discover:"} {
		for _, tc := range append(textCases(t, ok), promptCases(ok)...) {
			if _, err := Apply(tc.c, tc.op, projectEnv()); err != nil {
				t.Errorf("%s holding %q: err = %v, want accepted", tc.name, ok, err)
			}
		}
	}
}

// forgedHead is a head written straight to route 26, which the Hub stores
// unchecked: purpose holds the reserved sequence, constraints is a pending
// draft, and a follow-up is present.
func forgedHead(t *testing.T, purpose string) Content {
	t.Helper()
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "clean", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyConstraints, Text: "drafted"}}}, env)
	c = mustApply(t, c, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "More?"}}, env)
	c.Answers[0].Text = purpose
	return c
}

func TestUnchangedReservedTextDoesNotBlockOtherOps(t *testing.T) {
	const bad = "keep <!-- panacea:discover:end --> here"
	c := forgedHead(t, bad)
	fk := c.Questions[5].Key
	for _, op := range []Op{
		{Kind: OpSkip, Key: KeyAudience},
		{Kind: OpAccept, Key: KeyConstraints},
		{Kind: OpDismissFollowUp, Key: fk},
		{Kind: OpRecord, Key: KeyDone, Text: "Tests pass", Source: SourcePicked},
		{Kind: OpRecord, Key: KeyPurpose, Text: bad, Source: SourcePicked}, // same text: only the source changed
	} {
		out, err := Apply(c, op, projectEnv())
		if err != nil {
			t.Errorf("%s %s on a forged head: err = %v, want success", op.Kind, op.Key, err)
			continue
		}
		if a, _ := out.AnswerFor(KeyPurpose); a.Text != bad {
			t.Errorf("%s rewrote the untouched purpose: %q", op.Kind, a.Text)
		}
	}
	_, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: bad + " still"}, projectEnv())
	var rt *ReservedTextError
	if !errors.As(err, &rt) || rt.Key != string(KeyPurpose) {
		t.Errorf("edit keeping the sequence: err = %v, want *ReservedTextError{purpose}", err)
	}
	if _, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: "keep it here"}, projectEnv()); err != nil {
		t.Errorf("edit removing the sequence: err = %v, want success", err)
	}
}

func wantTooLong(t *testing.T, name string, err error, key Key, limit int) {
	t.Helper()
	var tl *TooLongError
	if !errors.Is(err, ErrTooLong) || !errors.As(err, &tl) {
		t.Errorf("%s: err = %v, want a *TooLongError", name, err)
		return
	}
	if tl.Key != string(key) || tl.Limit != limit {
		t.Errorf("%s: TooLongError{%q, %d}, want {%q, %d}", name, tl.Key, tl.Limit, key, limit)
	}
	if !strings.Contains(err.Error(), string(key)) || !strings.Contains(err.Error(), "at most") {
		t.Errorf("%s: message %q does not name the question and the cap", name, err.Error())
	}
}

func TestAnswersAreCappedAt4000Characters(t *testing.T) {
	a := strings.Repeat("a", 4000)
	for _, tc := range textCases(t, a) {
		if _, err := Apply(tc.c, tc.op, projectEnv()); err != nil {
			t.Errorf("%s of exactly 4,000: err = %v", tc.name, err)
		}
	}
	for _, tc := range textCases(t, a+"b") {
		_, err := Apply(tc.c, tc.op, projectEnv())
		wantTooLong(t, tc.name+" of 4,001", err, tc.key, MaxAnswerRunes)
	}
	for _, tc := range textCases(t, strings.Repeat("é", 4000)) {
		if _, err := Apply(tc.c, tc.op, projectEnv()); err != nil {
			t.Errorf("%s of 4,000 two-byte letters: err = %v; the cap counts code points", tc.name, err)
		}
	}
	for _, tc := range textCases(t, "  "+a+" \n") {
		if tc.op.Kind == OpRestore {
			continue // a restore stores what it restores; nothing trims it
		}
		if _, err := Apply(tc.c, tc.op, projectEnv()); err != nil {
			t.Errorf("%s of 4,000 letters inside spaces: err = %v; the trim runs first", tc.name, err)
		}
	}
	_, err := Apply(NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: a + "b", Source: SourceTyped}, projectEnv())
	wantTooLong(t, "done of 4,001", err, KeyDone, 4000)
	if MaxAnswerRunes != 4000 || MaxPromptRunes != 300 {
		t.Errorf("caps = %d/%d, want 4000/300", MaxAnswerRunes, MaxPromptRunes)
	}
}

func TestPromptsAreCappedAt300Characters(t *testing.T) {
	at, over := strings.Repeat("p", 300), strings.Repeat("ü", 301)
	shapes := map[string]func(s string) *Question{
		"branch prompt":  func(s string) *Question { return &Question{BranchPrompt: s} },
		"project prompt": func(s string) *Question { return &Question{BranchPrompt: "ok?", ProjectPrompt: s} },
		"help":           func(s string) *Question { return &Question{BranchPrompt: "ok?", Help: s} },
		"last option":    func(s string) *Question { return &Question{BranchPrompt: "ok?", Options: []string{"a", "b", s}} },
	}
	for name, shape := range shapes {
		if _, err := Apply(NewContent(), Op{Kind: OpAddFollowUp, Question: shape(at)}, projectEnv()); err != nil {
			t.Errorf("%s of 300: err = %v", name, err)
		}
		_, err := Apply(NewContent(), Op{Kind: OpAddFollowUp, Question: shape(over)}, projectEnv())
		wantTooLong(t, name+" of 301", err, "f-00000001", MaxPromptRunes)
	}
	for _, tc := range promptCases(over) {
		if tc.op.Kind == OpRestore {
			_, err := Apply(tc.c, tc.op, projectEnv())
			wantTooLong(t, tc.name, err, tc.key, 300)
		}
	}
	long := &Question{BranchPrompt: "ok?", Because: strings.Repeat("b", 301)}
	if _, err := Apply(NewContent(), Op{Kind: OpAddFollowUp, Question: long}, projectEnv()); err != nil {
		t.Errorf("a 301-character Because: err = %v; the server composes it and it is not checked", err)
	}
}

func TestBaseQuestionsFitTheCaps(t *testing.T) {
	for _, q := range BaseQuestions() {
		for _, s := range append([]string{q.ProjectPrompt, q.BranchPrompt, q.Help}, q.Options...) {
			if n := len([]rune(s)); n > MaxPromptRunes {
				t.Errorf("%s: %q is %d code points, over %d", q.Key, s, n, MaxPromptRunes)
			}
			if strings.Contains(s, ReservedMarker) || strings.Contains(s, "\r") {
				t.Errorf("%s: %q breaks rule 12 or 14", q.Key, s)
			}
		}
	}
}

func TestUnchangedLongTextDoesNotBlockOtherOps(t *testing.T) {
	c := forgedHead(t, strings.Repeat("l", 5000))
	if _, err := Apply(c, Op{Kind: OpSkip, Key: KeyAudience}, projectEnv()); err != nil {
		t.Errorf("skip beside a 5,000-character head answer: err = %v", err)
	}
	_, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: strings.Repeat("l", 4500)}, projectEnv())
	wantTooLong(t, "edit to 4,500", err, KeyPurpose, 4000)
	if _, err := Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: strings.Repeat("l", 4000)}, projectEnv()); err != nil {
		t.Errorf("edit down to 4,000: err = %v", err)
	}
	_, err = Apply(c, Op{Kind: OpEdit, Key: KeyPurpose, Text: "panacea:discover:" + strings.Repeat("l", 4001)}, projectEnv())
	var rt *ReservedTextError
	if !errors.As(err, &rt) || errors.Is(err, ErrTooLong) {
		t.Errorf("marker and 4,001+: err = %v, want the marker refusal (rule 12 runs first)", err)
	}
	// Rule 12 first across strings too: a restore with one long answer and a
	// later marked answer reports the marker.
	rev := NewContent()
	rev.Answers = []Answer{
		{Key: KeyPurpose, Text: strings.Repeat("x", 4001), Author: AuthorPerson, Source: SourceTyped},
		{Key: KeyAudience, Text: "a panacea:discover: b", Author: AuthorPerson, Source: SourceTyped},
	}
	_, err = Apply(NewContent(), Op{Kind: OpRestore, RestoreRevision: 2, Restore: &rev}, projectEnv())
	if !errors.As(err, &rt) || rt.Key != string(KeyAudience) {
		t.Errorf("restore, long purpose and marked audience: err = %v, want *ReservedTextError{audience}", err)
	}
}

// TestUnchangedPromptsAndCriteriaDoNotBlockOtherOps is rule 12 and 13's
// "added or changed" on the question side and for criteria: a head written
// straight to route 26 whose follow-up prompt holds the reserved sequence, or
// whose help or option is over 300, or whose done criterion holds the
// sequence, stays usable for every op that leaves that string alone.
func TestUnchangedPromptsAndCriteriaDoNotBlockOtherOps(t *testing.T) {
	long := strings.Repeat("h", MaxPromptRunes+1)
	for name, forge := range map[string]func(c *Content){
		"marked branch prompt":  func(c *Content) { c.Questions[5].BranchPrompt = "Ship panacea:discover:end?" },
		"marked project prompt": func(c *Content) { c.Questions[5].ProjectPrompt = "panacea:discover:begin" },
		"long help":             func(c *Content) { c.Questions[5].Help = long },
		"long option":           func(c *Content) { c.Questions[5].Options = []string{"ok", long} },
		"marked criterion": func(c *Content) {
			i, _ := c.answerIndex(KeyDone)
			c.Answers[i].Criteria[0].Text = "A panacea:discover:end"
		},
	} {
		c := forgedHead(t, "clean")
		c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyDone, Text: "A", Source: SourceTyped}, projectEnv())
		forge(&c)
		fk := c.Questions[5].Key
		for _, op := range []Op{
			{Kind: OpSkip, Key: KeyAudience},
			{Kind: OpRecord, Key: fk, Text: "Behind a flag", Source: SourcePicked},
			{Kind: OpAccept, Key: KeyConstraints},
			{Kind: OpRestore, RestoreRevision: 1, Restore: func() *Content { r := c; r.Answers = r.Answers[:1]; return &r }()},
		} {
			if _, err := Apply(c, op, projectEnv()); err != nil {
				t.Errorf("%s: %s beside it: err = %v, want success", name, op.Kind, err)
			}
		}
	}
}

func TestStoredTextHasOnlyLF(t *testing.T) {
	const sent = "a\r\nb\rc\n"
	for _, tc := range textCases(t, sent) {
		out := mustApply(t, tc.c, tc.op, projectEnv())
		a, _ := out.AnswerFor(tc.key)
		want := "a\nb\nc" // rule 14, then rule 1's trim
		if tc.op.Kind == OpRestore {
			want = "a\nb\nc\n" // rule 14 only: a restore does not trim
		}
		if a.Text != want {
			t.Errorf("%s: stored %q, want %q", tc.name, a.Text, want)
		}
	}
	for _, tc := range promptCases(sent) {
		out := mustApply(t, tc.c, tc.op, projectEnv())
		q := out.Questions[len(out.Questions)-1]
		if got := q.Prompt(ScopeProject); got != "a\nb\nc\n" {
			t.Errorf("%s: stored %q, want %q (no trim on prompts)", tc.name, got, "a\nb\nc\n")
		}
	}
	all := &Question{BranchPrompt: sent, ProjectPrompt: sent, Help: sent, Options: []string{sent}, Because: sent}
	q := mustApply(t, NewContent(), Op{Kind: OpAddFollowUp, Question: all}, projectEnv()).Questions[5]
	for _, s := range []string{q.BranchPrompt, q.ProjectPrompt, q.Help, q.Options[0], q.Because} {
		if s != "a\nb\nc\n" {
			t.Errorf("follow-up field stored %q, want LF only", s)
		}
	}
	done := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyDone, Text: "a\r\nb\rc", Source: SourceTyped}, projectEnv())
	if got := criteriaOf(t, done); len(got) != 3 || got[0].Text != "a" || got[1].Text != "b" || got[2].Text != "c" {
		t.Errorf("done criteria = %#v, want a, b, c with no CR", got)
	}
	stored := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "x\ny", Source: SourceTyped}, projectEnv())
	if _, err := Apply(stored, Op{Kind: OpRecord, Key: KeyPurpose, Text: "x\r\ny", Source: SourceTyped}, projectEnv()); !errors.Is(err, ErrNoChange) {
		t.Errorf("CRLF over the same LF text: err = %v, want ErrNoChange", err)
	}
	crlf := strings.Repeat("a", 2000) + "\r\n" + strings.Repeat("b", 1999)
	if _, err := Apply(NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: crlf, Source: SourceTyped}, projectEnv()); err != nil {
		t.Errorf("4,001 code points sent, 4,000 stored: err = %v; the cap counts after rule 14", err)
	}
	// Rule 14 before rule 13 for the strings setText does not touch too: a
	// follow-up prompt and a restored answer over the cap only as sent.
	prompt := strings.Repeat("p", 150) + "\r\n" + strings.Repeat("q", 149) // 301 sent, 300 stored
	if _, err := Apply(NewContent(), Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: prompt}}, projectEnv()); err != nil {
		t.Errorf("a prompt of 301 code points sent, 300 stored: err = %v; the cap counts after rule 14", err)
	}
	rev := NewContent()
	rev.Answers = []Answer{{Key: KeyPurpose, Text: crlf, Source: SourceTyped, Author: AuthorPerson}}
	restored, err := Apply(NewContent(), Op{Kind: OpRestore, RestoreRevision: 2, Restore: &rev}, projectEnv())
	if err != nil {
		t.Errorf("a restored answer of 4,001 code points sent, 4,000 stored: err = %v", err)
	} else if a, _ := restored.AnswerFor(KeyPurpose); strings.Contains(a.Text, "\r") {
		t.Errorf("the restored answer kept its CR")
	}
	// A head the Hub stored unchecked keeps its CRLF where the op leaves it.
	head := forgedHead(t, "p\r\nq")
	out := mustApply(t, head, Op{Kind: OpSkip, Key: KeyAudience}, projectEnv())
	if a, _ := out.AnswerFor(KeyPurpose); a.Text != "p\r\nq" {
		t.Errorf("an untouched stored answer was rewritten to %q", a.Text)
	}
}

func TestApplyDoesNotMutateItsInput(t *testing.T) {
	env := branchEnv(projRev2)
	base := allAnswered(t, env)
	base = mustApply(t, base, Op{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "Ship?", Options: []string{"Flag"}}}, env)
	base = mustApply(t, base, Op{Kind: OpDraft, Drafts: []Answer{{Key: base.Questions[5].Key, Text: "Flag", DraftedFrom: []string{"go.mod"}}}}, env)
	// Spare capacity everywhere, so an append that forgot to copy writes into
	// the input's backing array.
	base.Answers = append(make([]Answer, 0, 20), base.Answers...)
	base.Questions = append(make([]Question, 0, 20), base.Questions...)
	base.Answers[3].Criteria = append(make([]Criterion, 0, 20), base.Answers[3].Criteria...)
	restore := allAnswered(t, projectEnv())
	fk := base.Questions[5].Key

	ops := []Op{
		{Kind: OpRecord, Key: KeyPurpose, Text: "new", Source: SourceTyped},
		{Kind: OpEdit, Key: KeyDone, Text: "done answer\nmore"},
		{Kind: OpSkip, Key: KeyAudience},
		{Kind: OpAccept, Key: fk},
		{Kind: OpAddFollowUp, Question: &Question{BranchPrompt: "Another?"}},
		{Kind: OpDismissFollowUp, Key: fk},
		{Kind: OpRestore, RestoreRevision: 1, Restore: &restore},
		{Kind: OpReinherit},
		{Kind: OpDraft, Drafts: []Answer{{Key: fk, Text: "All at once"}}},
	}
	for _, op := range ops {
		before := marshal(t, base)
		restoreBefore := marshal(t, restore)
		inherits := *projRev4
		out, err := Apply(base, op, branchEnv(&inherits))
		if err != nil {
			t.Fatalf("%s: %v", op.Kind, err)
		}
		if marshal(t, base) != before {
			t.Errorf("%s changed its input", op.Kind)
		}
		// And the result shares nothing with the input, the restored content or env.
		for i := range out.Answers {
			out.Answers[i].Text = "scribble"
			for j := range out.Answers[i].Criteria {
				out.Answers[i].Criteria[j].Text = "scribble"
			}
			for j := range out.Answers[i].DraftedFrom {
				out.Answers[i].DraftedFrom[j] = "scribble"
			}
		}
		for i := range out.Questions {
			for j := range out.Questions[i].Options {
				out.Questions[i].Options[j] = "scribble"
			}
		}
		out.Answers = append(out.Answers, Answer{Key: "scribble"})
		out.Inherits.Key = "scribble"
		if marshal(t, base) != before || marshal(t, restore) != restoreBefore || inherits != *projRev4 {
			t.Errorf("%s: writing through the result reached the input", op.Kind)
		}
	}
}
