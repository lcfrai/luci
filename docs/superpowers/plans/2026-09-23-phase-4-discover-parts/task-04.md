### Task 04: `discover` — the model's follow-up question, degrading to nothing

The one model path inside the question flow (brief S4). **Kept from the 2026-09-18 plan's task 3**
(`:1221-1561`): the degrade-on-any-failure rule, the JSON schema, `ExtractJSON`, at most one
follow-up per brief. **Dropped:** `TailorOptions` (change C16). **Changed:** the function no longer
mutates a brief. It returns a question, and the caller applies `OpAddFollowUp` (task 03) through
the one write path (task 08), so a follow-up is a revision like any other change.

**Files:**
- `control-plane/internal/discover/assist.go` + `assist_test.go` (new)

**Interfaces — complete source:**

```go
package discover

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

// ProposeFollowUp asks the model for at most one extra question. On success
// the returned Question has FollowUp true, an empty Key (Apply mints it), at
// most four options, and a Because the SERVER composes - "Suggested because
// of your answer to “<prompt>”." - from the base key the model names, never
// model prose. It calls the model at most once, under a context derived from
// ctx with FollowUpBudget, and returns within that budget: an expired budget
// is AssistFailed, never an error the caller must map.
func ProposeFollowUp(ctx context.Context, m modelclient.Completer, c Content, s Scope, h Hint) (Question, AssistOutcome)

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
```

**The model's reply schema** (sent as `Request.JSONSchema`; validated again after `ExtractJSON`,
because a schema is a request, not a guarantee):

```json
{
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
}
```

`ask: false` → `AssistNothingToAsk`. A `question` that is empty after trimming, or that equals a
base prompt, → `AssistFailed`. `basedOn` naming a question with no settled answer → `AssistFailed`.
A `question` or any option that contains `discover.ReservedMarker` (`panacea:discover:`) →
`AssistFailed` and an empty question (plan §4 Security, S3; task 03 rule 12 stays the backstop), so
model text never reaches `Apply` as a 400 on text the person did not write.

**Tests first** (`assist_test.go`, a fake `Completer`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNilCompleterIsModelOff` | `(Question{}, AssistModelOff)` with no call | a nil dereference, or a call to a default client |
| `TestAFailingModelDegrades` | an error, `ErrRefused`, non-JSON text and schema-invalid JSON each give `AssistFailed` and an empty question | a follow-up built from half a reply |
| `TestAtMostOneFollowUp` | a content that already holds a follow-up is `AssistAlreadyAsked` with **no** model call | a second follow-up, and the second `OF 5`-free step the owner never asked for |
| `TestNoFollowUpBeforeThreeSettledAnswers` | two settled answers → `AssistNotYet`, no call | a follow-up asked before there is anything to follow |
| `TestBecauseIsComposedByTheServer` | with `basedOn: "constraints"`, `Because == "Suggested because of your answer to “What must this branch not break?”."` whatever prose the model adds | a model-written justification rendered as fact |
| `TestThePromptCarriesNoFileContents` | the captured request's `User` contains the answers' text and the names in `Hint` and nothing else | a later caller stuffing repository source into the prompt (the ISM reason the old plan's task 6 records) |
| `TestAModelThatSaysEnoughIsNothingToAsk` | `ask: false` → `AssistNothingToAsk` | a follow-up forced out of a model that declined |
| `TestAModelReplyHoldingTheReservedSequenceDegrades` | a reply whose prompt, or any option, contains `panacea:discover:` is `AssistFailed` and an empty question (plan §4 Security, S3; task 03 rule 12 stays the backstop) | model text reaching `Apply` and surfacing to the person as a 400 on text they never wrote, or a picked option that forges a marker |
| `TestOptionsAreCappedAtFour` | a reply with six options is `AssistFailed` (schema), never truncated | silent truncation that changes what the model proposed |
| `TestTheFollowUpRunsUnderItsOwnBudget` | the fake `Completer` records `ctx.Deadline()`: it is set, and no later than `FollowUpBudget` after the call began, even when the caller's `ctx` has no deadline; a fake that blocks until `ctx.Done()` and returns `ctx.Err()` gives `AssistFailed` and an empty question (the test passes a parent `ctx` it cancels after 50 ms, so it never waits 20 s) | a call passed the request's own context, which a slow model holds past the desktop's 30 s timeout (review I4) |
| `TestAModelReplyAtTheSchemaLimitsIsProposed` (added as built) | a question of 200 code points, help of 200 and four options of 120, all in three-byte runes, are proposed, and `Apply` takes the result | a limit counted in bytes (`len`), or off by one |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail to compile.
- [ ] **Step 2:** Port the old plan's `ProposeFollowUp` body (`:1360-1555`) to the new signature:
  build the prompt from `Content` and `Hint`, one `Complete` call under
  `context.WithTimeout(ctx, FollowUpBudget)` (20 s), validate, compose `Because`.
- [ ] **Step 3:** Run the acceptance command; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discover/... -race -count=1 -run 'Assist|FollowUp|Model|Because|Prompt|Options|Enough'
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover
```

**Does NOT claim:** that a real model proposes useful follow-ups (a quality question for the owner's
batched session, `OD-12`); tailored options (dropped, C16).

**As built** (task t04, commits `3eceb55` and `9a86bd0`; the rules the part left open, in the
code's words):

- **The order of the checks.** A nil `Completer` is `AssistModelOff` first, whatever the content
  holds: it is the one reason a person can act on (the lens names both variables), and task 08's
  `TestProposeFollowUpWithNoModelWritesNothing` (`task-08.md:255`) expects `model_off` for a nil
  model without naming what its content holds. Then a
  follow-up anywhere in `Questions` is `AssistAlreadyAsked`; then fewer than three base questions
  whose answer is `Settled()` (a skip counts, a held draft and an orphan answer do not) is
  `AssistNotYet`. None of the three calls the model.
- **The call.** `context.WithTimeout(ctx, FollowUpBudget)`; a context already done makes no call;
  an error, or a reply that lands after the budget is spent (from a `Completer` that ignored
  `ctx`), is `AssistFailed`. `Effort` is `"medium"`, the 2026-09-18 plan's value for this call,
  because the model's default (high) spends the 20 s on thinking (the t02 concern). "Returns
  within the budget" holds for a `Completer` that honours `ctx`, as `modelclient.Client` does.
- **The schema, checked again.** Exactly the five properties, by their exact names (plain
  `encoding/json` would read `"BasedOn"` and ignore an extra field); each of its JSON type, and
  `null` is none of them; lengths in code points; at most four options, never truncated;
  `basedOn` one of the five base keys. A decline (`ask: false`) is held to the schema too: one
  that breaks it (a missing field, or a `basedOn` outside the enum) is `AssistFailed`, not
  `AssistNothingToAsk`. The rules below run only on `ask: true`.
- **The text rules.** Every string is trimmed; nothing is cut or dropped. An empty question, an
  empty option, or a question that repeats a base prompt (either scope's wording, this build's or
  the content's own, ignoring case and surrounding space) is `AssistFailed`.
  `panacea:discover:` in the question, **the help** or any option is `AssistFailed`. Help is
  added to the part's list: `Apply`'s rule 12 does not check help, so this is the only place that
  keeps the sequence out of a revision.
- **`basedOn`** must name a base question whose answer holds the person's words: settled **and
  not skipped** (typed, picked, or an accepted draft). An unanswered, skipped or held-draft
  question is `AssistFailed`, because the reason reads "your answer to". `Because` uses the
  content's own wording of that question for the scope (an older revision may hold another
  wording than this build's; `TestBecauseIsComposedByTheServer` has that row).
- **The question.** `ProjectPrompt` and `BranchPrompt` both hold the model's question, `Options`
  is `[]` (never `nil`) when the model offers none, `FollowUp` is true, and `Key` is empty.
- **The prompt** carries the `Hint`'s names and, for each base question the person answered in
  their own words, its key, its wording for the scope and the answer text. It carries no help,
  options, held drafts, `DraftedFrom` names, criterion ids, pins or times.
  `TestThePromptCarriesNoFileContents` pins "nothing else" in two ways: forbidden sentinels
  (every question's wording is a sentinel too, and only the answered ones may reach the model;
  the keys of a drafted or skipped question are not offered as a `basedOn`), and two contents
  whose prompts are the same template once the allowed pieces are cut out. A project brief's
  prompt names no branch.
- **Two tests renamed** so the acceptance `-run` pattern selects them: neither
  `TestNotBeforeThreeSettledAnswers` nor `TestAReplyHoldingTheReservedSequenceDegrades` contains a
  word of `Assist|FollowUp|Model|Because|Prompt|Options|Enough`.
- **Not closed here:** "at most one follow-up per brief" holds only while the follow-up is in the
  content. `OpDismissFollowUp` removes the question, and `Content` keeps no record that one was
  asked, so a second `propose_follow_up` after a dismissal is asked again. The trigger is task
  18's ("after the third settled base answer", once), and task 08 could refuse it from the
  history. Carried to the orchestrator.
