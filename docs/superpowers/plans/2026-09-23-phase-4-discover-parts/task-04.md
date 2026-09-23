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
	AssistModelOff     AssistOutcome = "model_off"      // Completer is nil: no PANACEA_APP_MODEL_API_KEY
	AssistFailed       AssistOutcome = "model_failed"   // transport, refusal, no JSON, invalid JSON, a reply holding panacea:discover:
	AssistNothingToAsk AssistOutcome = "nothing_to_ask" // the model said the five answers suffice
	AssistAlreadyAsked AssistOutcome = "already_asked"  // the brief already holds a follow-up
	AssistNotYet       AssistOutcome = "not_yet"        // fewer than three base answers are settled
)

// ProposeFollowUp asks the model for at most one extra question. On success
// the returned Question has FollowUp true, an empty Key (Apply mints it), at
// most four options, and a Because the SERVER composes - "Suggested because
// of your answer to “<prompt>”." - from the base key the model names, never
// model prose. It calls the model at most once and returns within 20 s.
func ProposeFollowUp(ctx context.Context, m modelclient.Completer, c Content, s Scope, h Hint) (Question, AssistOutcome)
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
| `TestNotBeforeThreeSettledAnswers` | two settled answers → `AssistNotYet`, no call | a follow-up asked before there is anything to follow |
| `TestBecauseIsComposedByTheServer` | with `basedOn: "constraints"`, `Because == "Suggested because of your answer to “What must this branch not break?”."` whatever prose the model adds | a model-written justification rendered as fact |
| `TestThePromptCarriesNoFileContents` | the captured request's `User` contains the answers' text and the names in `Hint` and nothing else | a later caller stuffing repository source into the prompt (the ISM reason the old plan's task 6 records) |
| `TestAModelThatSaysEnoughIsNothingToAsk` | `ask: false` → `AssistNothingToAsk` | a follow-up forced out of a model that declined |
| `TestAReplyHoldingTheReservedSequenceDegrades` | a reply whose prompt, or any option, contains `panacea:discover:` is `AssistFailed` and an empty question (plan §4 Security, S3; task 03 rule 12 stays the backstop) | model text reaching `Apply` and surfacing to the person as a 400 on text they never wrote, or a picked option that forges a marker |
| `TestOptionsAreCappedAtFour` | a reply with six options is `AssistFailed` (schema), never truncated | silent truncation that changes what the model proposed |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail to compile.
- [ ] **Step 2:** Port the old plan's `ProposeFollowUp` body (`:1360-1555`) to the new signature:
  build the prompt from `Content` and `Hint`, one `Complete` call with a 20 s context, validate,
  compose `Because`.
- [ ] **Step 3:** Run the acceptance command; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discover/... -race -count=1 -run 'Assist|FollowUp|Model|Because|Prompt|Options|Enough'
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover
```

**Does NOT claim:** that a real model proposes useful follow-ups (a quality question for the owner's
batched session, `OD-12`); tailored options (dropped, C16).
