package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

// fakeModel is a scripted Completer. It records every call, the request, the
// moment of the call and the deadline of the context it was handed.
type fakeModel struct {
	reply string
	err   error
	block bool          // wait for ctx.Done(), then return ctx.Err()
	sleep time.Duration // sleep this long IGNORING ctx, then reply

	calls       int
	last        modelclient.Request
	calledAt    time.Time
	deadline    time.Time
	hasDeadline bool
}

func (f *fakeModel) Complete(ctx context.Context, req modelclient.Request) (modelclient.Response, error) {
	f.calls++
	f.last = req
	f.calledAt = time.Now()
	f.deadline, f.hasDeadline = ctx.Deadline()
	if f.block {
		<-ctx.Done()
		return modelclient.Response{}, ctx.Err()
	}
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
	return modelclient.Response{Text: f.reply}, f.err
}

var testHint = Hint{ProjectName: "Engagement Portal", BranchName: "evidence-upload"}

// answered records "<key> answer" as a person's typed answer for each key,
// in order, on a fresh content.
func answered(t *testing.T, env Env, keys ...Key) Content {
	t.Helper()
	c := NewContent()
	for _, k := range keys {
		c = mustApply(t, c, Op{Kind: OpRecord, Key: k, Text: string(k) + " answer", Source: SourceTyped}, env)
	}
	return c
}

// three is a branch content with three settled base answers: purpose,
// audience and constraints. done and out_of_scope have none.
func three(t *testing.T) Content {
	t.Helper()
	return answered(t, branchEnv(nil), KeyPurpose, KeyAudience, KeyConstraints)
}

// proposal is a schema-valid reply that asks one question, based on the
// constraints answer. Each call returns a fresh map, so a row may change it.
func proposal() map[string]any {
	return map[string]any{
		"ask":      true,
		"question": "Which clients call the published API today?",
		"help":     "You said the published API must not break.",
		"options":  []any{"Only our web app", "The mobile app too", "Partners integrate with it"},
		"basedOn":  "constraints",
	}
}

func with(m map[string]any, k string, v any) map[string]any {
	m[k] = v
	return m
}

func without(m map[string]any, k string) map[string]any {
	delete(m, k)
	return m
}

// replyJSON is the model's reply text for v.
func replyJSON(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

// wantNone asserts an outcome that proposes nothing: the zero Question.
func wantNone(t *testing.T, name string, q Question, got, want AssistOutcome) {
	t.Helper()
	if got != want {
		t.Errorf("%s: outcome = %q, want %q", name, got, want)
	}
	if !reflect.DeepEqual(q, Question{}) {
		t.Errorf("%s: question = %#v, want the zero Question", name, q)
	}
}

// withDraft adds a model_drafted answer for key by hand, the shape task 13's
// on-ramp leaves: held, not settled, read from repository names.
func withDraft(c Content, key Key, text string) Content {
	c.Answers = append(append([]Answer{}, c.Answers...), Answer{Key: key, Text: text, Source: SourceDrafted,
		Author: AuthorModelDrafted, Criteria: []Criterion{}, DraftedFrom: []string{"package.json"}, AnsweredAt: stamp})
	return c
}

func TestNilCompleterIsModelOff(t *testing.T) {
	withFollowUp := mustApply(t, three(t), Op{Kind: OpAddFollowUp,
		Question: &Question{BranchPrompt: "Which clients call it?", Options: []string{}}}, branchEnv(nil))
	// model_off is named before the brief's own reasons: it is the one a
	// person can act on (the lens names both variables), so it wins whatever
	// the content holds.
	for name, c := range map[string]Content{
		"three settled answers": three(t),
		"no answers yet":        NewContent(),
		"a follow-up already":   withFollowUp,
	} {
		q, out := ProposeFollowUp(context.Background(), nil, c, ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistModelOff)
	}
}

func TestAFailingModelDegrades(t *testing.T) {
	valid := replyJSON(t, proposal())
	runes := func(n int) string { return strings.Repeat("界", n) } // three bytes each
	skippedConstraints := func() Content {
		c := answered(t, branchEnv(nil), KeyPurpose, KeyAudience)
		return mustApply(t, c, Op{Kind: OpSkip, Key: KeyConstraints}, branchEnv(nil))
	}
	draftedDone := func() Content { return withDraft(three(t), KeyDone, "It has tests that pass") }

	for name, tc := range map[string]struct {
		content func() Content // nil: three(t)
		fake    *fakeModel
	}{
		"a transport error":              {fake: &fakeModel{err: errors.New("gateway down")}},
		"a refusal":                      {fake: &fakeModel{err: fmt.Errorf("%w (stop_reason refusal)", modelclient.ErrRefused)}},
		"a valid reply beside an error":  {fake: &fakeModel{reply: valid, err: errors.New("stream cut")}},
		"no text":                        {fake: &fakeModel{reply: ""}},
		"prose with no JSON":             {fake: &fakeModel{reply: "I would ask about the rollout plan."}},
		"JSON that does not parse":       {fake: &fakeModel{reply: `{"ask": true, "question": }`}},
		"ask missing":                    {fake: &fakeModel{reply: replyJSON(t, without(proposal(), "ask"))}},
		"question missing":               {fake: &fakeModel{reply: replyJSON(t, without(proposal(), "question"))}},
		"help missing":                   {fake: &fakeModel{reply: replyJSON(t, without(proposal(), "help"))}},
		"options missing":                {fake: &fakeModel{reply: replyJSON(t, without(proposal(), "options"))}},
		"basedOn missing":                {fake: &fakeModel{reply: replyJSON(t, without(proposal(), "basedOn"))}},
		"ask null":                       {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "ask", nil))}},
		"ask a string":                   {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "ask", "true"))}},
		"question null":                  {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", nil))}},
		"question a number":              {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", 7))}},
		"help null":                      {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "help", nil))}},
		"basedOn null":                   {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "basedOn", nil))}},
		"options null":                   {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", nil))}},
		"options a string":               {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", "Only our web app"))}},
		"an option null":                 {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", []any{"Only our web app", nil}))}},
		"an option not a string":         {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", []any{"Only our web app", 2}))}},
		"an unknown field":               {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "reason", "the API is public"))}},
		"a field in the wrong case":      {fake: &fakeModel{reply: replyJSON(t, with(without(proposal(), "basedOn"), "BasedOn", "constraints"))}},
		"basedOn outside the enum":       {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "basedOn", "rollout"))}},
		"question over 200 code points":  {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", runes(200)+"?"))}},
		"help over 200 code points":      {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "help", runes(201)))}},
		"an option over 120 code points": {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", []any{"Only our web app", runes(121)}))}},
		"question empty after trimming":  {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", " \n\t "))}},
		"an option empty after trimming": {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "options", []any{"Only our web app", "   "}))}},
		"question is a base prompt":      {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", "What must this branch not break?"))}},
		"question is a base prompt in the project's words":  {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", "What must this project never break?"))}},
		"question is a base prompt but for case and spaces": {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", "  what is THIS branch for?\n"))}},
		"question is a base prompt the content no longer holds": {
			content: func() Content {
				c := three(t)
				c.Questions = c.Questions[:3] // done and out_of_scope gone, as on a forged head
				return c
			},
			fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", "What is explicitly out of scope for this branch?"))}},
		"question is the content's own wording of a base question": {
			content: func() Content {
				c := three(t)
				c.Questions[1].BranchPrompt = "Who will use what this branch builds?" // an older wording, kept by its revision
				return c
			},
			fake: &fakeModel{reply: replyJSON(t, with(proposal(), "question", "Who will use what this branch builds?"))}},
		"basedOn a question with no answer": {fake: &fakeModel{reply: replyJSON(t, with(proposal(), "basedOn", "done"))}},
		"basedOn a skipped question": {content: skippedConstraints,
			fake: &fakeModel{reply: valid}},
		"basedOn a drafted answer": {content: draftedDone,
			fake: &fakeModel{reply: replyJSON(t, with(proposal(), "basedOn", "done"))}},
	} {
		c := three(t)
		if tc.content != nil {
			c = tc.content()
		}
		q, out := ProposeFollowUp(context.Background(), tc.fake, c, ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistFailed)
		if tc.fake.calls != 1 {
			t.Errorf("%s: model calls = %d, want exactly 1", name, tc.fake.calls)
		}
	}
}

func TestAtMostOneFollowUp(t *testing.T) {
	valid := replyJSON(t, proposal())
	asked := mustApply(t, three(t), Op{Kind: OpAddFollowUp,
		Question: &Question{BranchPrompt: "Which clients call it?", Options: []string{}}}, branchEnv(nil))
	fu := asked.Questions[len(asked.Questions)-1].Key
	answeredFU := mustApply(t, asked, Op{Kind: OpRecord, Key: fu, Text: "Only the web app", Source: SourceTyped}, branchEnv(nil))
	first := three(t)
	first.Questions = append([]Question{asked.Questions[len(asked.Questions)-1]}, first.Questions...)
	// already_asked is checked before not_yet (fix round 1, review M3): a
	// follow-up beside fewer than three settled answers is already asked.
	// Apply never un-settles an answer, but it does not refuse this head.
	besideTwo := mustApply(t, answered(t, branchEnv(nil), KeyPurpose, KeyAudience), Op{Kind: OpAddFollowUp,
		Question: &Question{BranchPrompt: "Which clients call it?", Options: []string{}}}, branchEnv(nil))

	for name, c := range map[string]Content{
		"an unanswered follow-up":                asked,
		"an answered follow-up":                  answeredFU,
		"a follow-up ahead of the base ones":     first,
		"a follow-up beside two settled answers": besideTwo,
	} {
		f := &fakeModel{reply: valid}
		q, out := ProposeFollowUp(context.Background(), f, c, ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistAlreadyAsked)
		if f.calls != 0 {
			t.Errorf("%s: model calls = %d, want 0", name, f.calls)
		}
	}

	// The one follow-up a brief does get costs exactly one call.
	f := &fakeModel{reply: valid}
	if _, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint); out != AssistProposed || f.calls != 1 {
		t.Errorf("first follow-up: outcome %q after %d calls, want proposed after exactly 1", out, f.calls)
	}
}

func TestNoFollowUpBeforeThreeSettledAnswers(t *testing.T) {
	based := replyJSON(t, with(proposal(), "basedOn", "purpose"))
	two := func() Content { return answered(t, branchEnv(nil), KeyPurpose, KeyAudience) }
	orphan := func() Content {
		c := two()
		c.Answers = append(c.Answers, Answer{Key: "f-0000abcd", Text: "an answer to no question",
			Source: SourceTyped, Author: AuthorPerson, Criteria: []Criterion{}, DraftedFrom: []string{}, AnsweredAt: stamp})
		return c
	}
	accepted := func() Content {
		c := withDraft(two(), KeyConstraints, "The published API")
		return mustApply(t, c, Op{Kind: OpAccept, Key: KeyConstraints}, branchEnv(nil))
	}
	skipped := func() Content { return mustApply(t, two(), Op{Kind: OpSkip, Key: KeyConstraints}, branchEnv(nil)) }
	skips := func(c Content, keys ...Key) Content {
		for _, k := range keys {
			c = mustApply(t, c, Op{Kind: OpSkip, Key: k}, branchEnv(nil))
		}
		return c
	}
	// Three skips are three settled answers, but none holds the person's
	// words: an ask reply could only be model_failed (basedOn needs an
	// answer), so the model is not called (fix round 1, review M5).
	threeSkips := func() Content { return skips(NewContent(), KeyPurpose, KeyAudience, KeyConstraints) }
	oneTyped := func() Content {
		return skips(answered(t, branchEnv(nil), KeyPurpose), KeyAudience, KeyConstraints)
	}

	for name, tc := range map[string]struct {
		content func() Content
		want    AssistOutcome
	}{
		"no answers":                               {content: NewContent, want: AssistNotYet},
		"two settled answers":                      {content: two, want: AssistNotYet},
		"two settled and a held draft":             {content: func() Content { return withDraft(two(), KeyConstraints, "The published API") }, want: AssistNotYet},
		"two settled and an orphan answer":         {content: orphan, want: AssistNotYet},
		"three settled answers":                    {content: func() Content { return three(t) }, want: AssistProposed},
		"two settled and an accepted draft":        {content: accepted, want: AssistProposed},
		"two settled and a skip, which is settled": {content: skipped, want: AssistProposed},
		"three skips: none of the person's words":  {content: threeSkips, want: AssistNothingToAsk},
		"one typed answer and two skips":           {content: oneTyped, want: AssistProposed},
	} {
		f := &fakeModel{reply: based}
		_, out := ProposeFollowUp(context.Background(), f, tc.content(), ScopeBranch, testHint)
		wantCalls := 1
		if tc.want == AssistNotYet || tc.want == AssistNothingToAsk {
			wantCalls = 0
		}
		if out != tc.want || f.calls != wantCalls {
			t.Errorf("%s: outcome %q after %d calls, want %q after %d", name, out, f.calls, tc.want, wantCalls)
		}
	}
}

func TestBecauseIsComposedByTheServer(t *testing.T) {
	// The model argues its case in the help, and around the JSON; none of it
	// becomes the reason the person is shown.
	r := with(proposal(), "help", "  Because you rely on it, I am certain this matters.  ")
	r = with(r, "question", "  Which clients call the published API today?\n")
	r = with(r, "options", []any{"  Only our web app ", "The mobile app too"})
	text := "Because of their constraints answer, I suggest:\n```json\n" + replyJSON(t, r) + "\n```\nBecause: trust me."
	older := three(t)
	older.Questions[2].BranchPrompt = "What must this branch keep working?" // an older wording, kept by its revision

	for name, tc := range map[string]struct {
		scope   Scope
		content Content
		because string
	}{
		"branch":  {ScopeBranch, three(t), "Suggested because of your answer to “What must this branch not break?”."},
		"project": {ScopeProject, answered(t, projectEnv(), KeyPurpose, KeyAudience, KeyConstraints), "Suggested because of your answer to “What must this project never break?”."},
		// The reason quotes the question as the person saw it, from the content.
		"an older wording the content holds": {ScopeBranch, older, "Suggested because of your answer to “What must this branch keep working?”."},
	} {
		f := &fakeModel{reply: text}
		q, out := ProposeFollowUp(context.Background(), f, tc.content, tc.scope, testHint)
		want := Question{
			ProjectPrompt: "Which clients call the published API today?",
			BranchPrompt:  "Which clients call the published API today?",
			Help:          "Because you rely on it, I am certain this matters.",
			Options:       []string{"Only our web app", "The mobile app too"},
			FollowUp:      true,
			Because:       tc.because,
		}
		if out != AssistProposed || !reflect.DeepEqual(q, want) {
			t.Fatalf("%s: got %q %#v,\nwant proposed %#v", name, out, q, want)
		}

		// The caller applies it through the one write path: Apply mints the
		// key, and the next step is the follow-up with the server's reason.
		env := branchEnv(nil)
		if tc.scope == ScopeProject {
			env = projectEnv()
		}
		c := mustApply(t, tc.content, Op{Kind: OpAddFollowUp, Question: &q}, env)
		c = mustApply(t, c, Op{Kind: OpRecord, Key: KeyDone, Text: "It has tests that pass", Source: SourceTyped}, env)
		c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyOutOfScope}, env)
		step := c.Step(tc.scope)
		if step.Kind != StepFollowUp || step.Because != tc.because || step.Prompt != want.BranchPrompt || !strings.HasPrefix(string(step.Key), "f-") {
			t.Errorf("%s: step after Apply = %#v", name, step)
		}
	}
}

func TestThePromptCarriesNoFileContents(t *testing.T) {
	type side struct {
		user, system string
		allowed      []string // the person's answers, their questions' wording, and the Hint's names
		forbidden    []string // everything else the content or its questions carry
	}
	capture := func(tag string) side {
		env := branchEnv(nil)
		c := NewContent()
		var s side
		// The questions' wording, help and options are sentinels, so a leak
		// shows. The wording is the content's (an older revision may hold
		// another), and only the answered questions' wording may reach the
		// model: purpose, audience and constraints (indexes 0-2).
		for i := range c.Questions {
			n := fmt.Sprint(i)
			c.Questions[i].BranchPrompt = "PROMPT-" + tag + "-" + n + "?"
			c.Questions[i].Help = "HELP-" + tag + "-" + n
			c.Questions[i].Options = []string{"OPTION-" + tag + "-" + n}
			s.forbidden = append(s.forbidden, c.Questions[i].Help, c.Questions[i].Options[0])
			if i < 3 {
				s.allowed = append(s.allowed, c.Questions[i].BranchPrompt)
			} else {
				s.forbidden = append(s.forbidden, c.Questions[i].BranchPrompt)
			}
		}
		for _, k := range []Key{KeyPurpose, KeyAudience, KeyConstraints} {
			text := "ANSWER-" + tag + "-" + string(k) + "\nits second line"
			c = mustApply(t, c, Op{Kind: OpRecord, Key: k, Text: text, Source: SourceTyped}, env)
			s.allowed = append(s.allowed, text)
		}
		// done holds a draft read from the repository: held, not the person's.
		c.Answers = append(c.Answers, Answer{Key: KeyDone, Text: "DRAFT-" + tag, Source: SourceDrafted,
			Author: AuthorModelDrafted, Criteria: []Criterion{{ID: "c-" + tag + "000000", Text: "CRITERION-" + tag}},
			DraftedFrom: []string{"MANIFEST-" + tag + "/package.json", "DIR-" + tag + "/cmd"}, AnsweredAt: stamp})
		c = mustApply(t, c, Op{Kind: OpSkip, Key: KeyOutOfScope}, env)
		c.Inherits = &Inherit{Key: "brief:bf-INHERIT-" + tag + "@3", Digest: "DIGEST-" + tag}
		s.forbidden = append(s.forbidden, "DRAFT-"+tag, "CRITERION-"+tag, "c-"+tag+"000000", "MANIFEST-"+tag,
			"DIR-"+tag, "INHERIT-"+tag, "DIGEST-"+tag, stamp)
		h := Hint{ProjectName: "PROJECT-" + tag, BranchName: "BRANCH-" + tag}
		s.allowed = append(s.allowed, h.ProjectName, h.BranchName)

		f := &fakeModel{reply: `{"ask":false,"question":"","help":"","options":[],"basedOn":"purpose"}`}
		if _, out := ProposeFollowUp(context.Background(), f, c, ScopeBranch, h); out != AssistNothingToAsk || f.calls != 1 {
			t.Fatalf("side %s: outcome %q after %d calls", tag, out, f.calls)
		}
		if f.last.Effort != "medium" {
			t.Errorf("effort = %q, want medium: a bounded call inside the 20 s budget", f.last.Effort)
		}
		var got, want any
		if err := json.Unmarshal(f.last.JSONSchema, &got); err != nil {
			t.Fatalf("JSONSchema does not decode: %v", err)
		}
		_ = json.Unmarshal([]byte(partSchema), &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("JSONSchema = %s, want the part's schema", f.last.JSONSchema)
		}
		// The model names basedOn by key, so each answered question reaches it
		// under its key, and exactly those keys are offered (fix round 1,
		// review M1): purpose, audience and constraints, indexes 0-2.
		for i, k := range []Key{KeyPurpose, KeyAudience, KeyConstraints} {
			if want := "\n[" + string(k) + "] " + c.Questions[i].BranchPrompt + "\n"; !strings.Contains(f.last.User, want) {
				t.Errorf("side %s: User does not carry %q:\n%s", tag, want, f.last.User)
			}
		}
		if want := "\nbasedOn must be one of: purpose, audience, constraints\n"; !strings.Contains(f.last.User, want) {
			t.Errorf("side %s: User does not offer exactly the answered keys, %q:\n%s", tag, want, f.last.User)
		}
		s.user, s.system = f.last.User, f.last.System
		return s
	}

	a, b := capture("A"), capture("B")
	for _, s := range []side{a, b} {
		for _, p := range s.allowed {
			if !strings.Contains(s.user, p) {
				t.Errorf("User does not carry %q:\n%s", p, s.user)
			}
			if strings.Contains(s.system, p) {
				t.Errorf("System carries %q; it is a constant", p)
			}
		}
		for _, p := range s.forbidden {
			if strings.Contains(s.user, p) || strings.Contains(s.system, p) {
				t.Errorf("the request carries %q, which is not the person's answer or a name:\n%s", p, s.user)
			}
		}
		// Nor is the key of a question with none of the person's words
		// offered as a basedOn: a draft (done) or a skip (out_of_scope).
		for _, k := range []Key{KeyDone, KeyOutOfScope} {
			if strings.Contains(s.user, string(k)) {
				t.Errorf("User offers the key %q:\n%s", k, s.user)
			}
		}
	}
	// With the allowed pieces cut out, both prompts are the same template: no
	// other value from the content reaches the model.
	cut := func(s side) string {
		out := s.user
		for _, p := range s.allowed {
			out = strings.ReplaceAll(out, p, "<>")
		}
		return out
	}
	if cut(a) != cut(b) || a.system != b.system {
		t.Errorf("the prompt depends on more than the answers and the names:\n%s\n----\n%s", cut(a), cut(b))
	}

	// A project brief is shown to the model in the words the person saw.
	f := &fakeModel{reply: `{"ask":false,"question":"","help":"","options":[],"basedOn":"purpose"}`}
	c := answered(t, projectEnv(), KeyPurpose, KeyAudience, KeyConstraints)
	ProposeFollowUp(context.Background(), f, c, ScopeProject, Hint{ProjectName: "Engagement Portal"})
	if !strings.Contains(f.last.User, "What is this project for?") || strings.Contains(f.last.User, "What is this branch for?") ||
		strings.Contains(f.last.User, "Branch") {
		t.Errorf("project brief prompt (the project's wording, and no branch):\n%s", f.last.User)
	}
}

// partSchema is task 04's reply schema as sent, as the part writes it. The
// limits are not in it (fix round 1, review I1): they are checked after
// ExtractJSON and stated in the system prompt.
const partSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["ask", "question", "help", "options", "basedOn"],
  "properties": {
    "ask":      { "type": "boolean" },
    "question": { "type": "string" },
    "help":     { "type": "string" },
    "options":  { "type": "array", "items": { "type": "string" } },
    "basedOn":  { "type": "string", "enum": ["purpose", "audience", "constraints", "done", "out_of_scope"] }
  }
}`

// TestTheFollowUpSchemaSendsOnlyWhatStructuredOutputsSupport walks the schema
// the model is sent (fix round 1, review I1). Structured outputs support no
// string or array length keyword (the structured-outputs page's JSON Schema
// limitations: not minLength or maxLength, and no array constraint beyond a
// limited minItems), and modelclient is raw HTTP, so nothing strips one on
// the way as the SDKs do. Every keyword must be one the API supports, and
// every object must say additionalProperties false.
func TestTheFollowUpSchemaSendsOnlyWhatStructuredOutputsSupport(t *testing.T) {
	f := &fakeModel{reply: replyJSON(t, proposal())}
	if _, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint); out != AssistProposed {
		t.Fatalf("outcome %q, want proposed", out)
	}
	var schema map[string]any
	if err := json.Unmarshal(f.last.JSONSchema, &schema); err != nil {
		t.Fatalf("JSONSchema does not decode: %v", err)
	}
	supported := map[string]bool{"type": true, "additionalProperties": true, "required": true,
		"properties": true, "items": true, "enum": true}
	var walk func(path string, s map[string]any)
	sub := func(path string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			t.Errorf("%s is not a schema object: %#v", path, v)
			return
		}
		walk(path, m)
	}
	walk = func(path string, s map[string]any) {
		if s["type"] == "object" && s["additionalProperties"] != false {
			t.Errorf("%s: an object without additionalProperties false", path)
		}
		for k, v := range s {
			if !supported[k] {
				t.Errorf("%s: sends %q, which structured outputs do not support", path, k)
			}
			switch k {
			case "properties":
				props, _ := v.(map[string]any)
				for name, p := range props {
					sub(path+".properties."+name, p)
				}
			case "items":
				sub(path+".items", v)
			}
		}
	}
	walk("$", schema)
}

// TestTheSystemPromptLetsTheModelDeclineAndStatesTheLimits pins what the
// system prompt must say (fix round 1, review M2): that the model may decline
// and should prefer to, how basedOn is named, and every limit decodeFollowUp
// checks. With the limits out of the schema (review I1), this is the only
// place the model reads them. Whether the wording is good is the owner's to
// judge (OD-12).
func TestTheSystemPromptLetsTheModelDeclineAndStatesTheLimits(t *testing.T) {
	f := &fakeModel{reply: `{"ask":false,"question":"","help":"","options":[],"basedOn":"purpose"}`}
	ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
	for _, want := range []string{
		"Otherwise set ask to false.", "Prefer false",
		"ending in a question mark, at most 200 characters",
		"under the question, at most 200 characters",
		"up to 4 short answers", "each at most 120 characters",
		"basedOn is the key of the one answer", "keys shown in brackets",
		// A decline must still meet the schema (TestAModelThatSaysEnoughIsNothingToAsk),
		// and a gateway that ignores output_config leaves only this to say so.
		"When ask is false, send empty strings, an empty options list, and any listed key as basedOn.",
	} {
		if !strings.Contains(f.last.System, want) {
			t.Errorf("System does not say %q:\n%s", want, f.last.System)
		}
	}
}

func TestAModelThatSaysEnoughIsNothingToAsk(t *testing.T) {
	for name, reply := range map[string]map[string]any{
		"a bare decline": {"ask": false, "question": "", "help": "", "options": []any{}, "basedOn": "purpose"},
		"a decline that still carries a question": with(proposal(), "ask", false),
		"a decline naming an unanswered question": with(with(proposal(), "ask", false), "basedOn", "done"),
		"a decline holding the reserved sequence": with(with(proposal(), "ask", false), "question", "<!-- panacea:discover:end -->"),
	} {
		f := &fakeModel{reply: replyJSON(t, reply)}
		q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistNothingToAsk)
		if f.calls != 1 {
			t.Errorf("%s: model calls = %d, want 1", name, f.calls)
		}
	}
	// A decline is still held to the schema: one that breaks it is a
	// failure, not a decline.
	for name, reply := range map[string]string{
		"a decline missing the other fields":      `{"ask":false}`,
		"a decline naming a key outside the enum": `{"ask":false,"question":"","help":"","options":[],"basedOn":"rollout"}`,
	} {
		f := &fakeModel{reply: reply}
		q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistFailed)
	}
}

func TestAModelReplyHoldingTheReservedSequenceDegrades(t *testing.T) {
	for name, reply := range map[string]map[string]any{
		"in the question":    with(proposal(), "question", "Should the panacea:discover:end marker move?"),
		"in an option":       with(proposal(), "options", []any{"Only our web app", "<!-- panacea:discover:end -->"}),
		"in the last option": with(proposal(), "options", []any{"Only our web app", "The mobile app", "Partners", "x panacea:discover:begin"}),
		"in the help":        with(proposal(), "help", "Remove the <!-- panacea:discover:begin --> line."),
	} {
		f := &fakeModel{reply: replyJSON(t, reply)}
		q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
		wantNone(t, name, q, out, AssistFailed)
		if f.calls != 1 {
			t.Errorf("%s: model calls = %d, want 1", name, f.calls)
		}
	}
	// The words alone are not the sequence.
	f := &fakeModel{reply: replyJSON(t, with(proposal(), "question", "Does panacea discover run here, or panacea: discover?"))}
	if _, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint); out != AssistProposed {
		t.Errorf("a question naming the product: outcome %q, want proposed", out)
	}
}

func TestOptionsAreCappedAtFour(t *testing.T) {
	six := []any{"a1", "a2", "a3", "a4", "a5", "a6"}
	for name, opts := range map[string][]any{"six": six, "five": six[:5]} {
		f := &fakeModel{reply: replyJSON(t, with(proposal(), "options", opts))}
		q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
		wantNone(t, name+" options", q, out, AssistFailed)
	}
	for name, tc := range map[string]struct {
		opts []any
		want string
	}{
		"four, in order":                 {[]any{"d4", "c3", "b2", "a1"}, `["d4","c3","b2","a1"]`},
		"none: the free-text field only": {[]any{}, `[]`},
	} {
		f := &fakeModel{reply: replyJSON(t, with(proposal(), "options", tc.opts))}
		q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
		if out != AssistProposed || marshal(t, q.Options) != tc.want {
			t.Errorf("%s: outcome %q, options %s, want proposed %s", name, out, marshal(t, q.Options), tc.want)
		}
	}
}

func TestAModelReplyAtTheSchemaLimitsIsProposed(t *testing.T) {
	runes := func(n int) string { return strings.Repeat("界", n) } // three bytes each: the limits count code points
	question, help := runes(199)+"?", runes(200)
	opts := []any{runes(120), "b" + runes(119), "c" + runes(119), "d" + runes(119)}
	r := with(with(with(proposal(), "question", question), "help", help), "options", opts)
	f := &fakeModel{reply: replyJSON(t, r)}
	q, out := ProposeFollowUp(context.Background(), f, three(t), ScopeBranch, testHint)
	if out != AssistProposed || q.BranchPrompt != question || q.Help != help || len(q.Options) != 4 || q.Options[0] != runes(120) {
		t.Fatalf("outcome %q, question %d runes, help %d, options %d", out, len([]rune(q.BranchPrompt)), len([]rune(q.Help)), len(q.Options))
	}
	// And Apply takes it: the schema's limits sit inside rule 13's caps.
	mustApply(t, three(t), Op{Kind: OpAddFollowUp, Question: &q}, branchEnv(nil))
}

func TestTheFollowUpRunsUnderItsOwnBudget(t *testing.T) {
	if FollowUpBudget != 20*time.Second {
		t.Fatalf("FollowUpBudget = %v, want 20 s: inside the route's 28 s and the desktop's 30 s (review I4)", FollowUpBudget)
	}
	valid := replyJSON(t, proposal())

	// The budget is set whatever the caller's context says...
	hour, cancelHour := context.WithTimeout(context.Background(), time.Hour)
	defer cancelHour()
	for name, ctx := range map[string]context.Context{"no deadline": context.Background(), "an hour": hour} {
		f := &fakeModel{reply: valid}
		begin := time.Now()
		if _, out := ProposeFollowUp(ctx, f, three(t), ScopeBranch, testHint); out != AssistProposed {
			t.Errorf("%s: outcome %q, want proposed", name, out)
		}
		switch {
		case !f.hasDeadline:
			t.Errorf("%s: the model call has no deadline", name)
		case f.deadline.Before(begin.Add(FollowUpBudget)) || f.deadline.After(f.calledAt.Add(FollowUpBudget)):
			t.Errorf("%s: deadline %v is not FollowUpBudget after the call (began %v, called %v)", name, f.deadline, begin, f.calledAt)
		}
	}

	// ...and derives from it: a caller's shorter deadline stands.
	short, cancelShort := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShort()
	wantDeadline, _ := short.Deadline()
	f := &fakeModel{reply: valid}
	ProposeFollowUp(short, f, three(t), ScopeBranch, testHint)
	if !f.hasDeadline || !f.deadline.Equal(wantDeadline) {
		t.Errorf("deadline %v under a caller's 5 s, want the caller's %v", f.deadline, wantDeadline)
	}

	// A model that holds the call until it is cut off is a failure, and the
	// cut comes from the caller's context (cancelled after 50 ms).
	parent, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	f = &fakeModel{block: true}
	begin := time.Now()
	q, out := ProposeFollowUp(parent, f, three(t), ScopeBranch, testHint)
	wantNone(t, "a model that holds the call", q, out, AssistFailed)
	if el := time.Since(begin); el > 5*time.Second {
		t.Errorf("returned after %v; the caller cancelled at 50 ms", el)
	}

	// A reply that lands after the budget is spent is a failure, even from a
	// Completer that ignored the context.
	late, cancelLate := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelLate()
	f = &fakeModel{reply: valid, sleep: 80 * time.Millisecond}
	q, out = ProposeFollowUp(late, f, three(t), ScopeBranch, testHint)
	wantNone(t, "a reply after the budget", q, out, AssistFailed)

	// A budget already spent makes no call at all.
	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	f = &fakeModel{reply: valid}
	q, out = ProposeFollowUp(done, f, three(t), ScopeBranch, testHint)
	wantNone(t, "a spent budget", q, out, AssistFailed)
	if f.calls != 0 {
		t.Errorf("a spent budget: model calls = %d, want 0", f.calls)
	}

	// The brief's own reasons come before the budget, in the stated order:
	// under a spent context each is still named, and none makes a call (fix
	// round 1: the three-skips check after the spent-budget check survived).
	skipAll := NewContent()
	for _, k := range []Key{KeyPurpose, KeyAudience, KeyConstraints} {
		skipAll = mustApply(t, skipAll, Op{Kind: OpSkip, Key: k}, branchEnv(nil))
	}
	for name, tc := range map[string]struct {
		content Content
		want    AssistOutcome
	}{
		"a follow-up already": {mustApply(t, three(t), Op{Kind: OpAddFollowUp,
			Question: &Question{BranchPrompt: "Which clients call it?", Options: []string{}}}, branchEnv(nil)), AssistAlreadyAsked},
		"two settled answers": {answered(t, branchEnv(nil), KeyPurpose, KeyAudience), AssistNotYet},
		"three skips":         {skipAll, AssistNothingToAsk},
	} {
		f := &fakeModel{reply: valid}
		q, out := ProposeFollowUp(done, f, tc.content, ScopeBranch, testHint)
		wantNone(t, name+" under a spent budget", q, out, tc.want)
		if f.calls != 0 {
			t.Errorf("%s under a spent budget: model calls = %d, want 0", name, f.calls)
		}
	}
}
