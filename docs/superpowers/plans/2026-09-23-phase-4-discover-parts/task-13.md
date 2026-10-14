### Task 13: `repoanalysis` — the scan and the drafted brief

The on-ramp for an existing repository (brief S4; prototype state 7): a mechanical scan, a model
drafts the five answers from the scan, and every drafted answer waits for a person (`4-D2`).
**Kept from the 2026-09-18 plan's task 6** (`:2485-3004`): the scan is mechanical so the facts
shown are facts; the model sees `Findings` — counts, manifest names, directory names — **never file
contents** (the ISM reason the old plan records); `Scan` and `Findings` as declared at `:2495-2502`.
**Changed:** `DraftBrief` no longer mutates a brief; it returns answers, and the route writes them
as **one** `draft` revision through `discoverhub.Service.Apply`, so the draft is a revision like any
other and is never inherited until each answer is accepted or changed.

**Files:**
- `control-plane/internal/repoanalysis/scan.go`, `draft.go` + `scan_test.go`, `draft_test.go` (new)
- `control-plane/internal/discoverapi/analyse.go` + `analyse_test.go` (new)

**Interfaces — complete source:**

```go
// control-plane/internal/repoanalysis/scan.go — the old plan's declarations, kept
package repoanalysis

type LanguageCount struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
}

type Findings struct {
	Root        string          `json:"root"`
	FilesSeen   int             `json:"filesSeen"`
	Truncated   bool            `json:"truncated"`
	Languages   []LanguageCount `json:"languages"`
	Manifests   []string        `json:"manifests"` // "go.mod", "desktop/package.json", …
	TopLevel    []string        `json:"topLevel"`  // top-level directory names
	TestFiles   int             `json:"testFiles"`
	HasCLAUDEMD bool            `json:"hasClaudeMd"`
}

func (f Findings) Summary() string
func Scan(root string) (Findings, error) // skips .git, node_modules, vendor, target, dist; stops at 20 000 files (Truncated)

var ErrNotADir = errors.New("repoanalysis: not a directory")
```

```go
// control-plane/internal/repoanalysis/draft.go
package repoanalysis

type DraftOutcome string

const (
	DraftProposed DraftOutcome = "proposed"
	DraftModelOff DraftOutcome = "model_off"
	DraftFailed   DraftOutcome = "model_failed"
)

// DraftAnswers asks the model once for the five base answers from f alone.
// Each returned answer has Author model_drafted, Source drafted, and
// DraftedFrom = the Manifests and TopLevel names the prompt carried (the
// prototype's "read from" list), at most 8. An answer the model left empty
// is omitted, not sent as "", and so is a done answer whose
// discover.SplitCriteria is empty (";"), which task 03's Apply refuses as
// ErrEmptyAnswer for the whole op (task 03 as built). An answer whose text contains
// discover.ReservedMarker ("panacea:discover:") is dropped, alone, so the
// other drafts still land and Apply's marker rule never refuses model text
// (plan §4 Security, S3; task 03 rule 12 stays the backstop). Nil model →
// (nil, DraftModelOff). The one Complete runs under
// context.WithTimeout(ctx, DraftBudget); an expired budget is DraftFailed.
func DraftAnswers(ctx context.Context, m modelclient.Completer, f Findings, now time.Time) ([]discover.Answer, DraftOutcome)

// DraftBudget bounds the one model call (review I4). The analyse route runs
// under discoverapi Budgets.Analyse (55 s, task 11), below the desktop's 60 s timeoutMs
// (task 15); the model gets 35 s of it and the scan plus Apply's Hub read and
// write the other 20 s, so a slow model reaches the desktop as
// draft: "model_failed" and never as the desktop's timeout.
const DraftBudget = 35 * time.Second
```

```go
// control-plane/internal/discoverapi/analyse.go
package discoverapi

// POST /v1/discover/projects/{projectId}/analyse   body: {}
type AnalyseResponse struct {
	Findings repoanalysis.Findings     `json:"findings"`
	Draft    repoanalysis.DraftOutcome `json:"draft"`
	View     discoverhub.View          `json:"view"`    // the project scope, after the draft revision (or unchanged)
	Written  []string                  `json:"written"` // [] unless a draft revision was written
	Note     string                    `json:"note"`    // model_off: "No model is configured, so nothing was drafted. Set PANACEA_APP_MODEL_API_KEY and PANACEA_APP_MODEL_BASE_URL to have Discover draft a brief from the scan."
}

```

The route runs under `context.WithTimeout(r.Context(), Config.Budgets.Analyse)` (55 s, task 11's
`Budgets`; review I4), below the desktop's 60 s `timeoutMs` for analyse (task 15). Its expiry while
a Hub call is in flight is 503 `upstream_unavailable`, `upstream: "hub"` (task 11's route-deadline
row). A `Scan` error is 500 `internal` whatever it wraps: the scan runs before any Hub call and is
bounded at 20 000 entries, so an expiry there is not the Hub's. It scans the project's path
(`Locator.ProjectPath`), drafts (under `repoanalysis.DraftBudget`, 35 s), and — when answers came
back — calls `Service.Apply` with `Op{Kind: discover.OpDraft, Drafts: answers}` on the project scope
at its current head. Errors map as in task 11, including the route-deadline row. It is a POST: it
costs a model call and writes a revision.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestScanCountsWithoutReadingContents` (kept) | findings for a fixture tree whose files are all mode `0o000` (unreadable) on POSIX: the scan still succeeds, because it reads directory entries and names only | a scanner that reads source |
| `TestScanSkipsVendoredTrees` | `node_modules` and `target` contribute 0 files | a scan that walks a million files in `node_modules` |
| `TestScanTruncates` | a tree of 20 001 files → `Truncated`, `FilesSeen == 20000` | an unbounded walk |
| `TestDraftAnswersAreHeld` | every answer is `model_drafted` / `drafted` with a non-empty `DraftedFrom` | `4-D2` broken at the source |
| `TestEmptyDraftsAreOmitted` (task 03 as built) | a reply whose `audience` is `"  "` and whose `done` is `" ; "` returns neither; the other three land, and `Apply` of the result is not `ErrEmptyAnswer` | one empty model answer turning the whole on-ramp into a 400 on text the person did not write |
| `TestDraftPromptHasNoFileContents` | the captured prompt holds only `Findings.Summary()` | client source in a prompt |
| `TestDraftWithNoModelIsModelOff` | `(nil, model_off)`, no call | a nil dereference |
| `TestAnalyseWritesOneDraftRevision` | a bound project with no brief → one revision, `change.kind == "draft"`, status `open`, `step.kind == "review"` | a draft that completes the brief and is inherited unread |
| `TestAnalyseNeverOverwritesAPerson` | a project brief with a person's purpose → the draft revision keeps it | the on-ramp clobbering typed text |
| `TestADraftHoldingTheReservedSequenceIsDropped` | the model drafts five answers, one containing `panacea:discover:` → the revision holds the other four drafts and no fifth; the request is 200, not a 400 (plan §4 Security, S3; task 03 rule 12 stays the backstop) | model text reaching `Apply` and failing the whole on-ramp as a 400 on text the person never wrote |
| `TestAnalyseWithNoModelWritesNothing` | `written: []`, `note` names both variables | a revision with no answers |
| `TestADraftRunsUnderItsOwnBudget` | the fake `Completer` records `ctx.Deadline()`: set, and no later than `DraftBudget` after the call began, when the caller's `ctx` has none; a fake that blocks until `ctx.Done()` gives `(nil, model_failed)` (the parent `ctx` is cancelled after 50 ms, so no test waits 35 s) | a model call holding the request past the desktop's 60 s timeout (review I4) |
| `TestAnalyseAgainstAHubThatNeverAnswersIs503BeforeTheDesktopGivesUp` | a fake `Source` whose Hub calls block until `ctx.Done()`; the route is built with `Config.Budgets.Analyse` set to 200 ms → 503 `upstream_unavailable`, `upstream: "hub"`, within 1 s | a route with no deadline, which a black-holed Hub turns into the desktop's `timeout` (review I4) |

**Steps:**

- [ ] **Step 1:** Port the old plan's scan tests and `Scan` (`:2504-2890`); write the new tests.
- [ ] **Step 2:** `draft.go`: one `Complete` with a JSON schema of the five keys (strings, each at
  most 400 characters); validate; build answers. The 400 is checked in code after `ExtractJSON`
  and stated in the system prompt, **never sent as `maxLength`**: structured outputs support no
  `minLength`, `maxLength` or `maxItems`, and `modelclient` sends the schema raw (task 04's fix
  round 1, review I1; `TestTheFollowUpSchemaSendsOnlyWhatStructuredOutputsSupport` is the pattern
  to copy).
- [ ] **Step 3:** `analyse.go`; add the route to `Mount`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/repoanalysis/... ./control-plane/internal/discoverapi/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/repoanalysis
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverapi
$ grep -n "os.ReadFile\|os.Open(" control-plane/internal/repoanalysis/*.go | grep -v _test
(no output — the scan reads directory entries, never contents)
```

**Does NOT claim:** the quality of a real draft (the owner's batched session); a branch-scope draft
(the on-ramp is for a repository, prototype state 7).
