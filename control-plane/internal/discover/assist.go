package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

// Hint is the non-brief context a follow-up may use. It never carries file
// contents: the model sees names and the person's own answers, nothing else.
type Hint struct {
	ProjectName string
	BranchName  string // "" for a project brief
}

// AssistOutcome says why a follow-up was or was not proposed. The lens shows
// each one in its own words (prototype states 5 and 6); none is an error.
type AssistOutcome string

const (
	AssistProposed     AssistOutcome = "proposed"
	AssistModelOff     AssistOutcome = "model_off"      // Completer is nil: PANACEA_APP_MODEL_API_KEY or PANACEA_APP_MODEL_BASE_URL unset (task 02 amendment 4)
	AssistFailed       AssistOutcome = "model_failed"   // transport, refusal, no JSON, invalid JSON, a reply holding panacea:discover:
	AssistNothingToAsk AssistOutcome = "nothing_to_ask" // the model said the five answers suffice
	AssistAlreadyAsked AssistOutcome = "already_asked"  // the brief already holds a follow-up
	AssistNotYet       AssistOutcome = "not_yet"        // fewer than three base answers are settled
)

// FollowUpBudget bounds the one model call. The ops route runs a
// propose_follow_up under discoverapi Budgets.FollowUp (28 s, task 11),
// below the desktop's 30 s timeoutMs (task 15); the model gets 20 s of it
// and Apply's Hub reads before the model and its write after it (one
// WriteRevisions, up to three rebuilds on a fan-out conflict, then task 08's
// steps 8-9 post-commit re-pin checks, which only log when the deadline cuts
// them short and never fail the call, review I8) share
// the other 8 s, so a slow model reaches the desktop as model_failed and
// never as the desktop's timeout (review I4). A deadline that expires during
// that write can answer 503 after the Hub has committed; the person's Retry
// then carries a fresh Idempotency-Key (one per hubclient call, Hub contract
// §9.2) and meets revision_conflict against their own revision. The same
// ambiguity already exists when the desktop gives up; the budget does not add
// it (re-review b04-1 M4).
const FollowUpBudget = 20 * time.Second

// followUpMinSettled is how many base answers must be settled (a skip
// counts, as Answer.Settled says) before a follow-up is asked.
const followUpMinSettled = 3

// The reply schema's bounds. maxLength counts Unicode code points, so these
// are checked with utf8.RuneCountInString, never len. All sit inside task 03's
// rule 13 caps (MaxPromptRunes), so a follow-up never meets them in Apply.
const (
	followUpQuestionMax = 200
	followUpHelpMax     = 200
	followUpOptionMax   = 120
	followUpOptionsMax  = 4
)

// followUpEffort is the old plan's effort for this call (2026-09-18 plan,
// task 3): the person is waiting on it, and it must finish inside
// FollowUpBudget. The model's default (high) spends that on thinking.
const followUpEffort = "medium"

// followUpSchema is sent as Request.JSONSchema and checked again by
// decodeFollowUp, because a schema is a request, not a guarantee (a gateway
// may ignore output_config.format).
var followUpSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["ask", "question", "help", "options", "basedOn"],
  "properties": {
    "ask":      { "type": "boolean" },
    "question": { "type": "string", "maxLength": 200 },
    "help":     { "type": "string", "maxLength": 200 },
    "options":  { "type": "array", "maxItems": 4, "items": { "type": "string", "maxLength": 120 } },
    "basedOn":  { "type": "string", "enum": ["purpose", "audience", "constraints", "done", "out_of_scope"] }
  }
}`)

// followUpSystem asks for at most one extra question, and permits none.
const followUpSystem = `You are reviewing a person's answers to a short, fixed interview that sets up a brief for the coding agents working on their project or branch.

If one further question would materially change how an agent works here, propose exactly one. Otherwise set ask to false. Prefer false: every extra question costs the person time.

A proposed question is one sentence ending in a question mark, at most 200 characters, and must not repeat one of the interview's own questions. help is one short sentence the person sees under the question. options are up to four short answers the person could pick, each at most 120 characters, or none: a free-text field is always shown beside them. basedOn is the key of the one answer that prompted the question, and must be one of the keys shown in brackets.

When ask is false, send empty strings, an empty options list, and any listed key as basedOn.`

// followUpReply is a reply that passed the schema.
type followUpReply struct {
	ask      bool
	question string
	help     string
	options  []string
	basedOn  Key
}

// ProposeFollowUp asks the model for at most one extra question. On success
// the returned Question has FollowUp true, an empty Key (Apply mints it), at
// most four options, and a Because the SERVER composes - "Suggested because
// of your answer to “<prompt>”." - from the base key the model names, never
// model prose. It calls the model at most once, under a context derived from
// ctx with FollowUpBudget, and returns within that budget: an expired budget
// is AssistFailed, never an error the caller must map.
//
// It does not change c: the caller applies OpAddFollowUp through the one
// write path (task 08), so a follow-up is a revision like any other change.
// On every outcome but AssistProposed the Question is the zero value. The
// order of the checks is: a nil Completer (model_off, the one reason a
// person can act on); a follow-up already held (already_asked); fewer than
// three settled base answers (not_yet); then the one call. "Returns within
// the budget" holds for a Completer that honours ctx, as modelclient.Client
// does (task 02); a reply that lands after the budget is still AssistFailed.
func ProposeFollowUp(ctx context.Context, m modelclient.Completer, c Content, s Scope, h Hint) (Question, AssistOutcome) {
	if m == nil {
		return Question{}, AssistModelOff
	}
	for _, q := range c.Questions {
		if q.FollowUp {
			return Question{}, AssistAlreadyAsked
		}
	}
	settled := 0
	for _, q := range c.Questions {
		if a, ok := c.AnswerFor(q.Key); ok && a.Settled() {
			settled++
		}
	}
	if settled < followUpMinSettled {
		return Question{}, AssistNotYet
	}

	ctx, cancel := context.WithTimeout(ctx, FollowUpBudget)
	defer cancel()
	if ctx.Err() != nil {
		return Question{}, AssistFailed
	}
	resp, err := m.Complete(ctx, modelclient.Request{
		System:     followUpSystem,
		User:       followUpPrompt(c, s, h),
		Effort:     followUpEffort,
		JSONSchema: followUpSchema,
	})
	if err != nil || ctx.Err() != nil {
		return Question{}, AssistFailed
	}
	reply, ok := decodeFollowUp(resp.Text)
	if !ok {
		return Question{}, AssistFailed
	}
	if !reply.ask {
		return Question{}, AssistNothingToAsk
	}
	q, ok := buildFollowUp(reply, c, s)
	if !ok {
		return Question{}, AssistFailed
	}
	return q, AssistProposed
}

// personAnswered lists the base questions whose answer holds a person's
// words (settled and not skipped: typed, picked or an accepted draft), in
// question order. Only these reach the model, and only these may be the
// basedOn of a follow-up, because the reason reads "your answer to".
func personAnswered(c Content) []Question {
	out := []Question{}
	for _, q := range c.Questions {
		if q.FollowUp {
			continue
		}
		if a, ok := c.AnswerFor(q.Key); ok && a.Settled() && !a.Skipped {
			out = append(out, q)
		}
	}
	return out
}

// followUpPrompt is the user turn: the names in h, and each answered
// question's key, wording for s and the person's text. Nothing else from c -
// no help, options, held drafts, the names a draft was read from, criteria
// ids, pins or times - and no file contents, which Hint cannot carry.
func followUpPrompt(c Content, s Scope, h Hint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", h.ProjectName)
	if s == ScopeProject {
		b.WriteString("This is the project's brief.\n")
	} else {
		fmt.Fprintf(&b, "Branch: %s\nThis is the branch's brief.\n", h.BranchName)
	}
	b.WriteString("\nThe person's answers so far, each under its key and question:\n")
	keys := []string{}
	for _, q := range personAnswered(c) {
		a, _ := c.AnswerFor(q.Key)
		fmt.Fprintf(&b, "\n[%s] %s\n%s\n", q.Key, q.Prompt(s), a.Text)
		keys = append(keys, string(q.Key))
	}
	fmt.Fprintf(&b, "\nbasedOn must be one of: %s\n", strings.Join(keys, ", "))
	return b.String()
}

// decodeFollowUp finds the reply object and holds it to followUpSchema:
// exactly the five properties, by their exact names (encoding/json alone
// would match "BasedOn" and ignore an extra one), each of its JSON type (a
// null is none of them), the lengths in code points, at most four options,
// and basedOn one of the five base keys.
func decodeFollowUp(text string) (followUpReply, bool) {
	raw, err := modelclient.ExtractJSON(text)
	if err != nil {
		return followUpReply{}, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) != 5 {
		return followUpReply{}, false
	}
	var r followUpReply
	var basedOn string
	var options []json.RawMessage
	if !jsonBool(obj["ask"], &r.ask) ||
		!jsonString(obj["question"], followUpQuestionMax, &r.question) ||
		!jsonString(obj["help"], followUpHelpMax, &r.help) ||
		!jsonString(obj["basedOn"], 0, &basedOn) ||
		!jsonArray(obj["options"], &options) || len(options) > followUpOptionsMax {
		return followUpReply{}, false
	}
	r.options = make([]string, 0, len(options))
	for _, o := range options {
		var s string
		if !jsonString(o, followUpOptionMax, &s) {
			return followUpReply{}, false
		}
		r.options = append(r.options, s)
	}
	for _, q := range BaseQuestions() {
		if string(q.Key) == basedOn {
			r.basedOn = q.Key
			return r, true
		}
	}
	return followUpReply{}, false
}

func jsonBool(raw json.RawMessage, out *bool) bool {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		*out = true
	case "false":
		*out = false
	default:
		return false
	}
	return true
}

// jsonString reads a JSON string of at most max code points (0: no bound).
func jsonString(raw json.RawMessage, max int, out *string) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, out) != nil {
		return false
	}
	return max == 0 || utf8.RuneCountInString(*out) <= max
}

func jsonArray(raw json.RawMessage, out *[]json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, out) == nil
}

// buildFollowUp turns a schema-valid "ask" reply into the Question, or
// refuses it: an empty question, one that repeats a base prompt, an empty
// option, the reserved sequence anywhere in the model's text (plan §4
// Security, S3: model text never reaches Apply as a 400 on text the person
// did not write; task 03 rule 12 stays the backstop), or a basedOn with no
// words of the person's behind it. Every string is trimmed; nothing is cut
// or dropped, so the person sees what the model proposed or nothing.
func buildFollowUp(r followUpReply, c Content, s Scope) (Question, bool) {
	question := strings.TrimSpace(r.question)
	if question == "" || isBasePrompt(question, c) {
		return Question{}, false
	}
	help := strings.TrimSpace(r.help)
	options := make([]string, 0, len(r.options))
	for _, o := range r.options {
		if o = strings.TrimSpace(o); o == "" {
			return Question{}, false
		}
		options = append(options, o)
	}
	for _, t := range append([]string{question, help}, options...) {
		if strings.Contains(t, ReservedMarker) {
			return Question{}, false
		}
	}
	for _, based := range personAnswered(c) {
		if based.Key == r.basedOn {
			return Question{
				ProjectPrompt: question,
				BranchPrompt:  question,
				Help:          help,
				Options:       options,
				FollowUp:      true,
				Because:       "Suggested because of your answer to “" + based.Prompt(s) + "”.",
			}, true
		}
	}
	return Question{}, false
}

// isBasePrompt reports whether question repeats a base question, in either
// scope's wording, as this build words it or as c holds it (an older
// revision's wording), ignoring case and surrounding space.
func isBasePrompt(question string, c Content) bool {
	for _, q := range append(BaseQuestions(), c.Questions...) {
		for _, p := range []string{q.ProjectPrompt, q.BranchPrompt} {
			if p != "" && strings.EqualFold(strings.TrimSpace(p), question) {
				return true
			}
		}
	}
	return false
}
