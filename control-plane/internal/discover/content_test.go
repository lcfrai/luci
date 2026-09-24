package discover

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// clock is the one instant the tests stamp answers with. Its nanoseconds are
// not a round number, so FormatTime's truncation to three digits shows.
var clock = time.Date(2026, 9, 24, 10, 0, 0, 123456789, time.UTC)

const stamp = "2026-09-24T10:00:00.123Z"

// seqIDs is a deterministic NewID: prefix + "-" + a counter as 8 hex digits,
// one counter across prefixes, so a test can name every id it will see.
func seqIDs() func(string) string {
	n := 0
	return func(prefix string) string {
		n++
		return fmt.Sprintf("%s-%08x", prefix, n)
	}
}

func projectEnv() Env {
	return Env{Scope: ScopeProject, Now: clock, NewID: seqIDs()}
}

func branchEnv(in *Inherit) Env {
	return Env{Scope: ScopeBranch, Inherits: in, Now: clock, NewID: seqIDs()}
}

func mustApply(t *testing.T, c Content, op Op, env Env) Content {
	t.Helper()
	out, err := Apply(c, op, env)
	if err != nil {
		t.Fatalf("Apply(%s %q): %v", op.Kind, op.Key, err)
	}
	return out
}

// allAnswered records "<key> answer" for every base question, as a person.
func allAnswered(t *testing.T, env Env) Content {
	t.Helper()
	c := NewContent()
	for _, q := range BaseQuestions() {
		c = mustApply(t, c, Op{Kind: OpRecord, Key: q.Key, Text: string(q.Key) + " answer", Source: SourceTyped}, env)
	}
	return c
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

// wantBase is the 2026-09-18 plan's closed set (:872-935), byte for byte. It
// is written out again here, not derived from BaseQuestions, so a reworded
// question fails this test instead of silently changing every future digest.
var wantBase = []Question{
	{Key: "purpose", ProjectPrompt: "What is this project for?", BranchPrompt: "What is this branch for?",
		Help:    "One sentence. The first thing every agent here reads.",
		Options: []string{"Add a capability we do not have yet", "Fix something broken", "Change how something works", "Try an idea we might throw away"}},
	{Key: "audience", ProjectPrompt: "Who is this project for?", BranchPrompt: "Who is this branch for?",
		Help:    "Agents make different choices for an internal tool than a client-facing one.",
		Options: []string{"Our own team", "KPMG staff broadly", "Client users", "Nobody yet — a spike"}},
	{Key: "constraints", ProjectPrompt: "What must this project never break?", BranchPrompt: "What must this branch not break?",
		Help:    "Agents will refuse work that violates this instead of guessing.",
		Options: []string{"The published API", "Existing data and migrations", "The deployed environment", "Nothing — it is isolated"}},
	{Key: "done", ProjectPrompt: "What has to be true before work here is finished?", BranchPrompt: "What has to be true before this branch can merge?",
		Help:    "This becomes the definition of done, shown again at merge.",
		Options: []string{"It has tests that pass", "It does not break the published API", "It works on mobile", "Someone else has reviewed it"}},
	{Key: "out_of_scope", ProjectPrompt: "What is explicitly out of scope for this project?", BranchPrompt: "What is explicitly out of scope for this branch?",
		Help:    "Naming this is how you stop an agent widening the job while you are away.",
		Options: []string{"Performance work", "Visual design", "Migrations and data changes", "Nothing is off the table"}},
}

func TestBaseQuestionsAreTheFiveInFileOrder(t *testing.T) {
	got := BaseQuestions()
	if len(got) != len(wantBase) {
		t.Fatalf("len = %d, want %d", len(got), len(wantBase))
	}
	keys := []Key{KeyPurpose, KeyAudience, KeyConstraints, KeyDone, KeyOutOfScope}
	for i, q := range got {
		if q.Key != keys[i] {
			t.Errorf("question %d key = %q, want %q", i, q.Key, keys[i])
		}
		w := wantBase[i]
		w.Because = ""
		if !reflect.DeepEqual(q, w) {
			t.Errorf("question %d =\n %#v\nwant\n %#v", i, q, w)
		}
		if q.FollowUp || q.Because != "" {
			t.Errorf("question %d: FollowUp %v, Because %q; a base question has neither", i, q.FollowUp, q.Because)
		}
	}
	// A fresh copy each call: a caller that edits one cannot change the next.
	got[0].Options[0] = "changed"
	got[0].BranchPrompt = "changed"
	if again := BaseQuestions(); again[0].Options[0] != wantBase[0].Options[0] || again[0].BranchPrompt != wantBase[0].BranchPrompt {
		t.Errorf("BaseQuestions shares state between calls: %#v", again[0])
	}
}

func TestPromptPicksTheScopesWording(t *testing.T) {
	q := wantBase[0]
	if got := q.Prompt(ScopeProject); got != "What is this project for?" {
		t.Errorf("project prompt = %q", got)
	}
	if got := q.Prompt(ScopeBranch); got != "What is this branch for?" {
		t.Errorf("branch prompt = %q", got)
	}
	// A follow-up with no project wording falls back to the branch wording.
	f := Question{BranchPrompt: "How does this reach users?"}
	if got := f.Prompt(ScopeProject); got != "How does this reach users?" {
		t.Errorf("fallback prompt = %q", got)
	}
	g := Question{ProjectPrompt: "Project only?"}
	if got := g.Prompt(ScopeBranch); got != "" {
		t.Errorf("branch prompt of a project-only question = %q, want the rule's \"\"", got)
	}
}

func TestContentMarshalsEverySliceAsAnArrayNeverNull(t *testing.T) {
	check := func(name, s string) {
		t.Helper()
		if strings.Count(s, "null") != 1 || !strings.Contains(s, `"inherits":null`) {
			t.Errorf("%s: want exactly one null, the inherits one:\n%s", name, s)
		}
	}
	check("NewContent", marshal(t, NewContent()))

	skipped := mustApply(t, NewContent(), Op{Kind: OpSkip, Key: KeyDone}, projectEnv())
	check("a skipped done answer", marshal(t, skipped))

	// A literal with nil slices everywhere (a head decoded from a document
	// that held null): every result of Apply still marshals [] for each.
	literal := Content{V: 1, Status: StatusOpen,
		Questions: []Question{{Key: KeyPurpose, BranchPrompt: "Why?"}, {Key: KeyAudience, BranchPrompt: "Who?"}}}
	after := mustApply(t, literal, Op{Kind: OpSkip, Key: KeyPurpose}, projectEnv())
	after = mustApply(t, after, Op{Kind: OpRecord, Key: KeyAudience, Text: "Us", Source: SourcePicked}, projectEnv())
	check("a nil-sliced literal after two ops", marshal(t, after))

	// The step the desktop reads is JSON too: no null there either.
	for _, s := range []Step{NewContent().Step(ScopeBranch), allAnswered(t, projectEnv()).Step(ScopeProject)} {
		if js := marshal(t, s); strings.Contains(js, "null") {
			t.Errorf("step %q marshals a null: %s", s.Kind, js)
		}
	}
}

// goldenContent holds one answer of each author, one skipped answer, one
// follow-up and an Inherits, so every field of every type is on the wire.
func goldenContent() Content {
	return Content{
		V:      1,
		Status: StatusOpen,
		Questions: []Question{
			{Key: KeyPurpose, ProjectPrompt: "Why the project?", BranchPrompt: "Why the branch?", Help: "One line.", Options: []string{"Fix it"}},
			{Key: KeyAudience, ProjectPrompt: "Who?", BranchPrompt: "Who here?", Help: "", Options: []string{}},
			{Key: KeyDone, ProjectPrompt: "Done when?", BranchPrompt: "Mergeable when?", Help: "Checklist.", Options: []string{}},
			{Key: "f-0000abcd", BranchPrompt: "How does it ship?", Options: []string{"Behind a flag", "All at once"}, FollowUp: true, Because: "Suggested because of your answer to purpose."},
		},
		Answers: []Answer{
			{Key: KeyPurpose, Text: "Fix the export", Source: SourceTyped, Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp},
			{Key: KeyAudience, Text: "Client users", Source: SourceDrafted, Author: AuthorModelAccepted, Criteria: []Criterion{}, DraftedFrom: []string{"go.mod"}, AnsweredAt: stamp},
			{Key: KeyDone, Text: "Tests pass", Source: SourceDrafted, Author: AuthorModelDrafted, Criteria: []Criterion{{ID: "c-00000001", Text: "Tests pass"}}, DraftedFrom: []string{"package.json", "cmd"}, AnsweredAt: stamp},
			{Key: "f-0000abcd", Skipped: true, Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp},
		},
		Inherits: &Inherit{Key: "brief:bf-1@3", Digest: "sha256:ab12"},
		Change:   Change{Kind: ChangeEdit, Key: KeyDone},
	}
}

// goldenJSON is the checked-in wire shape. A Go string, not a testdata file,
// so a checkout's line-ending conversion cannot touch it.
const goldenJSON = `{"v":1,"status":"open","questions":[` +
	`{"key":"purpose","projectPrompt":"Why the project?","branchPrompt":"Why the branch?","help":"One line.","options":["Fix it"],"followUp":false,"because":""},` +
	`{"key":"audience","projectPrompt":"Who?","branchPrompt":"Who here?","help":"","options":[],"followUp":false,"because":""},` +
	`{"key":"done","projectPrompt":"Done when?","branchPrompt":"Mergeable when?","help":"Checklist.","options":[],"followUp":false,"because":""},` +
	`{"key":"f-0000abcd","projectPrompt":"","branchPrompt":"How does it ship?","help":"","options":["Behind a flag","All at once"],"followUp":true,"because":"Suggested because of your answer to purpose."}` +
	`],"answers":[` +
	`{"key":"purpose","text":"Fix the export","skipped":false,"source":"typed","author":"person","criteria":[],"draftedFrom":[],"answeredAt":"2026-09-24T10:00:00.123Z"},` +
	`{"key":"audience","text":"Client users","skipped":false,"source":"drafted","author":"model_accepted","criteria":[],"draftedFrom":["go.mod"],"answeredAt":"2026-09-24T10:00:00.123Z"},` +
	`{"key":"done","text":"Tests pass","skipped":false,"source":"drafted","author":"model_drafted","criteria":[{"id":"c-00000001","text":"Tests pass"}],"draftedFrom":["package.json","cmd"],"answeredAt":"2026-09-24T10:00:00.123Z"},` +
	`{"key":"f-0000abcd","text":"","skipped":true,"source":"","author":"person","criteria":[],"draftedFrom":[],"answeredAt":"2026-09-24T10:00:00.123Z"}` +
	`],"inherits":{"key":"brief:bf-1@3","digest":"sha256:ab12"},"change":{"kind":"edit","key":"done","restores":0}}`

func TestContentGoldenJSON(t *testing.T) {
	if got := marshal(t, goldenContent()); got != goldenJSON {
		t.Errorf("content marshals to\n%s\nwant\n%s", got, goldenJSON)
	}
	// And it reads back to the same value.
	back, err := Decode([]byte(goldenJSON))
	if err != nil {
		t.Fatalf("Decode(golden): %v", err)
	}
	if !reflect.DeepEqual(back, goldenContent()) {
		t.Errorf("Decode(golden) =\n%#v\nwant\n%#v", back, goldenContent())
	}
}

func TestDecodeRejectsANewerVersion(t *testing.T) {
	newer := strings.Replace(goldenJSON, `{"v":1,`, `{"v":2,`, 1)
	if _, err := Decode([]byte(newer)); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("Decode(v2) err = %v, want ErrUnknownVersion", err)
	}
	for name, raw := range map[string]string{
		"no version":   `{"status":"open","questions":[],"answers":[]}`,
		"version zero": `{"v":0,"status":"open","questions":[],"answers":[]}`,
		"not json":     `{"v":1,`,
		"not object":   `[1]`,
	} {
		_, err := Decode([]byte(raw))
		if err == nil {
			t.Errorf("%s: Decode succeeded, want an error", name)
		} else if errors.Is(err, ErrUnknownVersion) {
			t.Errorf("%s: err = %v; ErrUnknownVersion is for a newer version only", name, err)
		}
	}
}

func TestDecodeGivesEverySliceAValue(t *testing.T) {
	raw := `{"v":1,"status":"open","questions":[{"key":"purpose","branchPrompt":"Why?","options":null}],` +
		`"answers":[{"key":"purpose","text":"x","source":"typed","author":"person","criteria":null,"draftedFrom":null,"answeredAt":""}],"inherits":null,"change":{"kind":"record","key":"purpose","restores":0}}`
	c, err := Decode([]byte(raw))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.Questions[0].Options == nil || c.Answers[0].Criteria == nil || c.Answers[0].DraftedFrom == nil {
		t.Errorf("Decode left a nil slice: %#v", c)
	}
	if empty, err := Decode([]byte(`{"v":1}`)); err != nil || empty.Questions == nil || empty.Answers == nil {
		t.Errorf("Decode({v:1}) = %#v, %v; want empty, non-nil slices", empty, err)
	}
}

func TestNewContentIsTheBaseSetOpenAndUnanswered(t *testing.T) {
	c := NewContent()
	if ContentVersion != 1 || c.V != ContentVersion {
		t.Errorf("V = %d, want 1", c.V)
	}
	if c.Status != StatusOpen || c.Inherits != nil || len(c.Answers) != 0 || c.Answers == nil {
		t.Errorf("NewContent = %#v, want open, no inherits, answers []", c)
	}
	if !reflect.DeepEqual(c.Questions, BaseQuestions()) {
		t.Errorf("questions are not the base set")
	}
	if c.Change != (Change{}) {
		t.Errorf("change = %#v, want the zero change: NewContent is never written", c.Change)
	}
}

func TestStepWalksTheClosedSetInOrder(t *testing.T) {
	env := branchEnv(nil)
	c := NewContent()
	for i, q := range wantBase {
		s := c.Step(ScopeBranch)
		if s.Kind != StepQuestion || s.Ordinal != i+1 || s.Total != 5 || s.Key != q.Key {
			t.Fatalf("step %d = %s %d/%d %q, want question %d/5 %q", i, s.Kind, s.Ordinal, s.Total, s.Key, i+1, q.Key)
		}
		if s.Prompt != q.BranchPrompt || s.Help != q.Help || !reflect.DeepEqual(s.Options, q.Options) || s.Because != "" {
			t.Errorf("step %d shows %q / %q / %v / %q", i, s.Prompt, s.Help, s.Options, s.Because)
		}
		c = mustApply(t, c, Op{Kind: OpRecord, Key: q.Key, Text: "an answer", Source: SourceTyped}, env)
	}
	s := c.Step(ScopeBranch)
	if s.Kind != StepComplete || s.Ordinal != 0 || s.Total != 0 || s.Key != "" {
		t.Errorf("after five answers step = %#v, want complete with no count", s)
	}
	if got := NewContent().Step(ScopeProject).Prompt; got != "What is this project for?" {
		t.Errorf("project step prompt = %q, want the project wording", got)
	}
}

func TestFollowUpsCarryNoDenominator(t *testing.T) {
	env := branchEnv(nil)
	c := mustApply(t, NewContent(), Op{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpAddFollowUp, Question: &Question{
		BranchPrompt: "How does this reach users?",
		Help:         "You said this changes a published API.",
		Options:      []string{"Behind a flag", "All at once"},
		Because:      "Suggested because of your answer to purpose.",
	}}, env)

	// The next base step still counts the closed set only.
	if s := c.Step(ScopeBranch); s.Kind != StepQuestion || s.Ordinal != 2 || s.Total != 5 {
		t.Fatalf("base step after a follow-up = %s %d/%d, want question 2/5", s.Kind, s.Ordinal, s.Total)
	}
	for _, k := range []Key{KeyAudience, KeyConstraints, KeyDone, KeyOutOfScope} {
		c = mustApply(t, c, Op{Kind: OpRecord, Key: k, Text: "x", Source: SourceTyped}, env)
	}
	s := c.Step(ScopeBranch)
	if s.Kind != StepFollowUp {
		t.Fatalf("kind = %q, want follow_up", s.Kind)
	}
	if s.Total != 0 || s.Ordinal != 0 {
		t.Errorf("follow-up ordinal/total = %d/%d, want 0/0: a follow-up has no honest denominator", s.Ordinal, s.Total)
	}
	if s.Key != c.Questions[5].Key || s.Prompt != "How does this reach users?" || s.Because != "Suggested because of your answer to purpose." ||
		s.Help != "You said this changes a published API." || !reflect.DeepEqual(s.Options, []string{"Behind a flag", "All at once"}) {
		t.Errorf("follow-up step = %#v", s)
	}
	if c.Status != StatusOpen {
		t.Errorf("status = %q, want open while the follow-up is unanswered", c.Status)
	}
}

func TestReviewStepListsOnlyPendingDrafts(t *testing.T) {
	env := branchEnv(nil)
	c := mustApply(t, NewContent(), Op{Kind: OpDraft, Drafts: []Answer{
		{Key: KeyAudience, Text: "Client users"},
		{Key: KeyDone, Text: "Tests pass"},
	}}, env)

	// Drafts never hold up the questions still unanswered: purpose is asked
	// first, then constraints (ordinal 3), skipping the drafted audience.
	if s := c.Step(ScopeBranch); s.Kind != StepQuestion || s.Key != KeyPurpose || s.Ordinal != 1 {
		t.Fatalf("first step = %#v, want question 1 purpose", s)
	}
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyPurpose, Text: "x", Source: SourceTyped}, env)
	if s := c.Step(ScopeBranch); s.Kind != StepQuestion || s.Key != KeyConstraints || s.Ordinal != 3 || s.Total != 5 {
		t.Fatalf("second step = %#v, want question 3/5 constraints", s)
	}
	if s := c.Step(ScopeBranch); !reflect.DeepEqual(s.Pending, []Key{KeyAudience, KeyDone}) {
		t.Errorf("pending on a question step = %v, want [audience done]", s.Pending)
	}
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyConstraints, Text: "x", Source: SourceTyped}, env)
	c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyOutOfScope}, env)
	c = mustApply(t, c, Op{Kind: OpAccept, Key: KeyDone}, env)

	s := c.Step(ScopeBranch)
	if s.Kind != StepReview || s.Total != 0 || s.Ordinal != 0 {
		t.Fatalf("step = %#v, want review with no count", s)
	}
	if !reflect.DeepEqual(s.Pending, []Key{KeyAudience}) || !reflect.DeepEqual(c.Pending(), []Key{KeyAudience}) {
		t.Errorf("pending = %v / %v, want [audience]: the accepted and the person's answers are settled", s.Pending, c.Pending())
	}
	if c.Status != StatusOpen {
		t.Errorf("status = %q, want open while a draft awaits a person", c.Status)
	}
	c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyAudience, Text: "Our own team", Source: SourcePicked}, env)
	if s := c.Step(ScopeBranch); s.Kind != StepComplete || len(s.Pending) != 0 || s.Pending == nil {
		t.Errorf("after the last draft is changed, step = %#v, want complete, pending []", s)
	}
}

func TestPendingFollowsQuestionOrderNotAnswerOrder(t *testing.T) {
	env := projectEnv()
	c := mustApply(t, NewContent(), Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyOutOfScope, Text: "a"}}}, env)
	c = mustApply(t, c, Op{Kind: OpDraft, Drafts: []Answer{{Key: KeyPurpose, Text: "b"}}}, env)
	if got := c.Pending(); !reflect.DeepEqual(got, []Key{KeyPurpose, KeyOutOfScope}) {
		t.Errorf("Pending = %v, want [purpose out_of_scope]", got)
	}
	if got := NewContent().Pending(); got == nil || len(got) != 0 {
		t.Errorf("Pending of a fresh brief = %#v, want []", got)
	}
}

func TestSettledIsSkippedOrAPersonsText(t *testing.T) {
	for _, tc := range []struct {
		a    Answer
		want bool
	}{
		{Answer{Text: "x", Author: AuthorPerson}, true},
		{Answer{Text: "x", Author: AuthorModelAccepted}, true},
		{Answer{Skipped: true, Author: AuthorPerson}, true},
		{Answer{Text: "x", Author: AuthorModelDrafted}, false},
		{Answer{Text: "x", Author: ""}, false},
		{Answer{Text: "", Author: AuthorPerson}, false},
	} {
		if got := tc.a.Settled(); got != tc.want {
			t.Errorf("%#v.Settled() = %v, want %v", tc.a, got, tc.want)
		}
	}
}

func TestAnswerForFindsByKey(t *testing.T) {
	c := allAnswered(t, projectEnv())
	a, ok := c.AnswerFor(KeyDone)
	if !ok || a.Key != KeyDone || a.Text != "done answer" {
		t.Errorf("AnswerFor(done) = %#v, %v", a, ok)
	}
	if _, ok := NewContent().AnswerFor(KeyDone); ok {
		t.Errorf("AnswerFor on a fresh brief found an answer")
	}
}

func TestSplitCriteria(t *testing.T) {
	for in, want := range map[string][]string{
		"It has tests that pass":          {"It has tests that pass"},
		"- A\n- B\n\n* C":                 {"A", "B", "C"},
		"A; B ;; C":                       {"A", "B", "C"},
		"  \n ; \n":                       {},
		"- ":                              {"-"},
		"Line one\nLine two; and three\n": {"Line one", "Line two", "and three"},
		"*  Starred with two spaces":      {"Starred with two spaces"},
		"-no space is kept":               {"-no space is kept"},
	} {
		got := SplitCriteria(in)
		if got == nil || !reflect.DeepEqual(got, want) {
			t.Errorf("SplitCriteria(%q) = %#v, want %#v", in, got, want)
		}
	}
}

func TestFormatTimeIsUTCWithThreeFractionalDigits(t *testing.T) {
	if got := FormatTime(clock); got != stamp {
		t.Errorf("FormatTime = %q, want %q", got, stamp)
	}
	sydney := time.FixedZone("AEST", 10*60*60)
	if got := FormatTime(time.Date(2026, 9, 24, 20, 0, 0, 0, sydney)); got != "2026-09-24T10:00:00.000Z" {
		t.Errorf("FormatTime(AEST, no fraction) = %q, want UTC with .000", got)
	}
}
