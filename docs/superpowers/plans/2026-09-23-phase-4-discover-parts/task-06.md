### Task 06: `provenance` — the key grammar and the one type every surface renders

Contract §1.2–§1.4 and P5, fixed as code. **The only parser of a key in the product.** Phases 2, 3,
5 and 7 and the desktop hold keys as opaque strings and compare them for equality. **Kept from the
2026-09-18 plan's task 8** (`:3349-3590`): `Ref` as the one type, no `Met`/`Total`/percentage, a
mandatory `because` on any non-`open` criterion state, every state `open` in this phase. **Changed:**
everything in change C14, and `Build` works from a resolved revision plus its pinned project
revision rather than from a branch.

**Files:**
- `control-plane/internal/provenance/key.go` + `key_test.go` (new)
- `control-plane/internal/provenance/ref.go` + `ref_test.go` (new)

**Interfaces — complete source:**

```go
// control-plane/internal/provenance/key.go

// Package provenance owns the key grammar (contract P5): nothing else in the
// product may split a key. It also owns Ref, the one type Develop, Merge,
// Design and Deploy render (contract §1.4).
package provenance

type Kind string

const (
	KindBrief  Kind = "brief"
	KindDesign Kind = "design"
	KindNone   Kind = "none"
)

// The grammar. brief and design are the Hub's, verbatim (Hub contract §3.2);
// none is local to the control plane and never reaches the Hub (P4-C).
//
//	brief   ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$
//	design  ^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$
//	none    ^none:(declined)$
type Key struct {
	Kind       Kind
	ID         string // bf-… or the canvas id; "" for none
	Revision   int    // >= 1; 0 for none
	NoneReason string // "declined"; "" otherwise
}

// Declined is the contextRef of a session the person chose to start without
// its brief (4-D1). The empty string keeps its phase 2 meaning: no brief was
// materialised because none existed.
const Declined = "none:declined"

var ErrInvalidKey = errors.New("provenance: not a key")

func Parse(s string) (Key, error)
func (k Key) String() string
func BriefKey(briefID string, revision int) string // panics on a bad id or revision < 1: a programming error, not input
```

```go
// control-plane/internal/provenance/ref.go
package provenance

type Resolution string

const (
	Resolved Resolution = "resolved" // the revision exists, is readable, and its bytes match its digest
	Missing  Resolution = "missing"  // no such revision, or one the caller cannot read (Hub §7.5: indistinguishable on purpose)
	Erased   Resolution = "erased"   // an org admin erased it; key, digest, savedAt, savedBy remain (ADR-H1 §5.6)
	Invalid  Resolution = "invalid"  // not a key in the grammar
	None     Resolution = "none"     // "" or none:declined — no brief was inherited
	// Unavailable is a slot the server could not answer because the Hub did
	// not: any Hub error, signed_out, or no Hub configured (task 07 rule 5,
	// review I6). A key already cached is still answered (ADR-H1 §5.4); a
	// branch key counts as cached only with its pinned project revision
	// cached too (task 07 rule 5, re-review b05-1 R-M1).
	// Missing is never used for these: it comes only from the Hub's answer.
	// Never cached by the desktop; the loader retries it (task 16).
	Unavailable Resolution = "unavailable"
)

type CriterionState string

const (
	StateOpen      CriterionState = "open"
	StateSettled   CriterionState = "settled"
	StateContested CriterionState = "contested"
)

// Criterion is the part of a criterion the key fixes: a function of the key
// alone, so it lives in the Ref and is cached for ever (review I14).
type Criterion struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Digest string `json:"digest"` // "sha256:" + hex(SHA-256(UTF-8 of Text)) — phase 3's P4-2
}

// CriterionReading is the part that MOVES: phase 3's verdict on one
// criterion of one key, which changes as evidence arrives (S4-P3.4). It is
// never inside a Ref. It is served beside heads, per key
// (RefsResponse.criterionStates, task 12), and beside each head reading, per
// target (HeadReading.criterionStates, task 12), and the desktop overwrites
// it on every answer (task 16). Review I14.
type CriterionReading struct {
	ID      string         `json:"id"`      // a Criterion.ID of the same key
	State   CriterionState `json:"state"`   // "open" throughout phase 4
	Because string         `json:"because"` // required when State != open; "" when open
}

type InheritedRef struct {
	Key      string `json:"key"`
	BriefID  string `json:"briefId"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
	// Erased: the pinned project revision was erased, so the Ref holds none
	// of its text (task 07 rule 7; re-review b06-1 R-I2).
	Erased bool `json:"erased"`
}

// Ref is one brief key, resolved. Cacheable for ever EXCEPT headRevision and
// latestCompleteRevision, which the desktop keeps per brief (contract §6.4).
// It holds no criterion state: that moves, so it is a CriterionReading
// served beside the Ref, never cached with it (review I14).
// Superseded is not a resolution: it is revision < latestCompleteRevision,
// derived by the desktop from the per-brief heads (P4-B).
type Ref struct {
	Key                    string            `json:"key"`
	Resolution             Resolution        `json:"resolution"`
	NoneReason             string            `json:"noneReason"` // "" | "declined"
	Present                bool              `json:"present"`    // resolution is resolved or erased
	BriefID                string            `json:"briefId"`
	Revision               int               `json:"revision"`
	Digest                 string            `json:"digest"`
	Scope                  string            `json:"scope"`     // "project" | "branch" | ""
	ProjectID              string            `json:"projectId"` // the HUB project id, pj-…
	BranchRef              string            `json:"branchRef"` // refs/heads/… or ""
	Status                 string            `json:"status"`    // the revision's content status
	HeadRevision           int               `json:"headRevision"`
	LatestCompleteRevision int               `json:"latestCompleteRevision"`
	Inherits               *InheritedRef     `json:"inherits"`
	Title                  string            `json:"title"`    // discover.Title of the revision the key names: a function of the key alone (task 05, review I5), open revisions included
	Headline               string            `json:"headline"` // the purpose section's body
	Audience               string            `json:"audience"`
	Constraints            string            `json:"constraints"`
	OutOfScope             string            `json:"outOfScope"`
	Sections               []discover.Section `json:"sections"`
	Criteria               []Criterion       `json:"criteria"`
	SavedAt                string            `json:"savedAt"`
	SavedBy                string            `json:"savedBy"` // display name
	ErasedAt               string            `json:"erasedAt"`
}

// Head is the time-varying part, per brief.
type Head struct {
	HeadRevision           int  `json:"headRevision"`
	LatestCompleteRevision int  `json:"latestCompleteRevision"`
	Archived               bool `json:"archived"`
}

// EvalQuery is one brief key's criteria, and the branch they are evaluated
// at. BriefKey, ProjectID and Criteria are functions of the key. BranchRef is
// not: it is the caller's (Readings' at), because a branch is where evidence
// is, and a key names a revision, not a branch (re-review b08-2 R2-I1, R2-M1).
type EvalQuery struct {
	BriefKey  string      // brief:<id>@<rev>
	ProjectID string      // the HUB project id, pj-… (Source.ProjectID)
	BranchRef string      // refs/heads/<name>: Readings' at; never "" here
	Criteria  []Criterion // Build's, digests included (P4-2), in order
}

// Verdict is phase 3's evidence.State narrowed to what a reading shows
// (phase 3 task-15.md:87-95 at a255408).
type Verdict struct {
	CriterionID string
	State       CriterionState
	Because     []string // evidence.State.Because, verbatim; Readings joins it
}

// Evaluate is the seam phase 3's evidence.Evaluator fills (ask P4-4, S4-P3.4).
// It has the shape of (*evidence.Evaluator).States (phase 3
// task-15.md:112 at a255408): one call for every criterion of one key,
// a ctx, an error, and a []string because. The adapter
// (control-plane/cmd/api/evidence.go, below) maps the query to States'
// (repo, projectPath, subject) and converts types; it decides nothing else.
// Nil means every criterion is open - phase 4's only behaviour.
type Evaluate func(ctx context.Context, q EvalQuery) ([]Verdict, error)

// Source is a resolved revision as discoverhub hands it over.
type Source struct {
	Revision  discover.Revision  // the revision the key names
	Project   *discover.Revision // its pinned project revision (branch keys), or nil
	// PinErased: Revision pins a project revision the Hub answered erased.
	// Project is then nil, and Build renders the branch's own answers alone
	// (task 07 rule 7; re-review b06-1 R-I2).
	PinErased bool
	Digest    string
	ProjectID string
	SavedAt   string
	SavedBy   string
	// No Names: discover.Resolve takes none, and the title comes from
	// Revision.BranchRef, the envelope's own ref (review I5, task 05).
	// No BranchRef either: Ref.BranchRef is Revision.BranchRef, so the fact
	// has one carrier and two copies cannot disagree (re-review b05-1 R-M2).
	// The branch criteria are evaluated at is not a fact of the revision at
	// all: it is Readings' at.
}

// ErrBadSource: Build was handed a source that is not the revision its key
// names, or a pin that cannot be the one the revision holds. Every error
// Build returns is one. No Hub answer produces one; it is a caller fault (a
// 500), never the person's.
var ErrBadSource = errors.New("provenance: the source is not the revision the key names")

// Build resolves the revision the key names through discover.Resolve and
// fills every field; Ref.Title is the Document's Title and Ref.BranchRef is
// src.Revision.BranchRef. An open revision (only a history row can name one)
// gets Sections from its own settled answers, no inheritance, Status "open",
// and Ref.Title = discover.Title(&src.Revision) for a branch revision or
// discover.Title(nil) for a project one - task 05's one copy of the rule,
// never a string written here (re-review b05-1 R-M2). Resolve refuses an
// incomplete revision, so Build hands it a copy marked complete with no pin;
// the stored status is what Ref.Status says, and a held draft still renders
// nothing (task 05). Ref.Inherits is the pin the content holds, open or not.
// Build takes no Evaluate: nothing in a Ref may depend on evidence
// (review I14).
// With src.PinErased, Build calls discover.Resolve on a copy of src.Revision
// whose Content.Inherits is nil (so a nil project is legal, task 05), then
// sets Ref.Inherits from the original Content.Inherits with Erased true: no
// OriginProject section, and an audience, constraint or out-of-scope the
// branch skipped stays "".
//
// The error (task 06 as built; fix round 1, review m5): every error is
// ErrBadSource, so a caller picks its status with one errors.Is. Build
// refuses a source the key does not name - another revision or brief, a key
// that is not a brief key, a pin passed to or held by a project revision, an
// erased pin beside a pin in hand or on a revision that pins nothing, a pin
// that is not a brief key - itself. What discover.Resolve refuses keeps
// discover's error in the chain beside ErrBadSource: a pin that is not the
// one the revision holds (discover.ErrInheritMismatch), a pin whose stored
// status is not complete (discover.ErrIncomplete), and an envelope its scope
// never holds. Whatever the error, the Ref is UnavailableRef(key): never one
// key on another revision's text.
func Build(key string, src Source, head Head) (Ref, error)

// Readings is the moving part of one resolved key: one CriterionReading per
// Criterion of Build(key, src, …), in the same order ([] when Build would
// refuse src, or the key has no definition of done). at is the branch the
// criteria are evaluated at, named by the caller when it calls, never read
// from the key: a key names a revision, not a branch (re-review b08-2).
//   - HeadRef(t) (task 07) passes t's branch for whatever key t inherits, a
//     project key included: a branch with no complete branch brief inherits
//     the project key, and its head reading shows verdicts at its own head
//     (R2-I1). That reading is per target, and so are its states.
//   - Resolve (task 07, by key) passes "" for a project key, which it never
//     evaluates: one project key inherited by two branches has two sets of
//     verdicts, and a by-key answer holds one. For a branch key it passes the
//     branch the key's brief is bound to now, not the revision's envelope
//     BranchRef: a rebind writes no revision, so after a rename the envelope
//     names a ref that is gone, or an unrelated branch that took the name
//     (R2-M1). That ref is the brief's live BriefHead.BranchRef, which
//     /v1/refs does not carry, so task 07 reads it with List (its rule 8,
//     ask HA4-13), and passes "" for an archived brief or a failed read.
//
// Its rules (S4-P3.4):
//  1. ONE eval call per key, with every criterion of the done section in
//     order, never one call per criterion; none at all when there are no
//     criteria.
//  2. Every state is open with because "" when eval is nil, when at is not
//     "refs/heads/" and a name (no branch named: eval is not called, so a
//     subject is never guessed), when eval returns an error, and when its
//     verdicts do not match the criteria one for one, by id and in order. An
//     error is not the refs call's error.
//  3. because is the verdict's Because, each element trimmed, empty ones
//     dropped, joined with "; " in phase 3's order.
//  4. A non-open verdict whose because is then "" is shown open: an
//     unexplained verdict is not shown (P12). So is a state that is not
//     open, settled or contested, on that criterion alone.
//
// Called on EVERY answer, cached revision or not, so a verdict is never older
// than the refs call that carried it.
func Readings(ctx context.Context, key string, src Source, at string, eval Evaluate) []CriterionReading

func MissingRef(key string) Ref
func InvalidRef(key string) Ref
func UnavailableRef(key string) Ref
func NoneRef(contextRef string) Ref // "" or none:declined
func ErasedRef(key string, briefID string, revision int, digest, savedAt, savedBy, erasedAt string, head Head) Ref

func CriterionDigest(text string) string
```

**The adapter to phase 3's evaluator (S4-P3.4; re-review b08-1 R-I1).** Phase 3's counterpart is
`func (e *Evaluator) States(ctx context.Context, repo, projectPath, subject string, criteria
[]briefref.Criterion) ([]State, error)` (phase 3 `task-15.md:112`, its answer to ask P4-4 at
phase 3 `plan.md:382`, both at phase 3 worktree `a255408`). `Evaluate` above has its shape, so the
adapter holds only the one rule `Readings` cannot: **key to subject**.
- **File:** `control-plane/cmd/api/evidence.go` (new),
  `func evidenceEvaluate(ev evidenceStates, hub *discoverhub.Service, scan *project.Scanner, loc discoverhub.Locator, git gitx.Machine) provenance.Evaluate`,
  where `evidenceStates` is a one-method interface holding `States`' signature, so that
  `*evidence.Evaluator` satisfies it and a test can pass a fake
  (`gitx.Machine` is phase 3 plan text, the type of `Evaluator.Machine`),
  with `evidence_test.go`, wired as `discoverhub.Config.Evaluate` in `main.go`.
- **Who writes it:** whichever of phase 4 task 14 (the wiring) and phase 3 task 15 (the
  evaluator) lands **second**, since it needs both. Task 14 checks with `go doc
  ./control-plane/internal/evidence Evaluator`. No match means `Config.Evaluate` stays nil, every
  criterion is open, and the adapter goes to phase 3's task 15 under ask P4-4, which must carry
  this section (cross-phase ask, below).
- **Key to subject**, in order, each failure returning an error so `Readings` shows `open`:
  1. The local project is the one project in `scan.Projects(ctx)` (`project.go:184`) whose
     `hub.Binding` has `HubProjectID == q.ProjectID` (memoised 60 s, task 07). None, or more
     than one, is an error: an ambiguous clone is shown open, never guessed.
  2. `projectPath` is `loc.ProjectPath(ctx, id)`.
  3. `repo` is the key phase 3 records evidence under for that project, computed by phase 3's
     own rule (`githost.Remote.RepoKey()`, else `"local:<projectId>"`, phase 3 `task-15.md:49`),
     never a phase 4 copy of it. **Phase 3 exports no function for that rule** (re-review b08-2
     R2-M2): it is only the `Record.Repo` field comment (`task-15.md:49` at `a255408`, `:50` at
     phase 3 `e7d2281`), and it does not say which remote it takes when a clone has several. A
     different pick from phase 3's POST makes `ByDigest(repo)` find nothing, so every criterion
     reads `open` with no error anywhere. The cross-phase ask below therefore carries an exported
     rule, for example `evidence.RepoFor(ctx, projectPath, projectID string) (string, error)`,
     and the adapter calls it. Until phase 3 exports one, the adapter is not written and
     `Config.Evaluate` stays nil.
  4. `subject` is the commit `q.BranchRef` names in that clone at call time: `git rev-parse
     --verify --end-of-options <ref>^{commit}` through `gitx.Read` with Scope Project. A ref the
     clone lacks is an error. `q.BranchRef` is `Readings`' `at`, the caller's branch (the
     target's for a head reading, the brief's current one for a by-key answer), never the
     revision's envelope ref (re-review b08-2 R2-I1, R2-M1). A key names a revision, not a
     commit, so an older revision's criteria are evaluated at today's head, which is phase 3's
     "at the branch head".
- **Types:** `briefref.Criterion{ID: c.ID, Digest: c.Digest}` in; `Verdict{CriterionID,
  State, Because}` from each `evidence.State` out. `Evidence`, `RecordedAgainst` and `Commits` are
  dropped, because a `CriterionReading` shows none of them.
- **Cross-phase ask (phase 3's stream; this plan cannot edit it):** phase 3's P4-4 row and task
  15 name this adapter, its file and its second-lander rule, so both plans carry one shape.
  Recorded in the ledger (`progress.md`, b08 round 2; the amended ask is its own entry,
  `CROSS-PHASE ASK (P4-4, amended by t06)`, written in fix round 1, review m4). Task 06 as built
  adds two items to the ask: phase 3 exports its repo rule (rule 3 above, R2-M2), and its evaluator is called at the
  branch the caller names, a project key's criteria included (R2-I1), which `States` already
  allows: it takes a commit and never a branch id.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestParseAcceptsTheHubsForms` | Hub contract §3.2's examples parse; `String()` round-trips; as built also each form's edges, the design form's nine-digit revision included (mutant R8-P1) | a grammar narrower than the Hub's |
| `TestParseRejects` — table | `brief:b-42@3` (a pre-Hub id), `brief:bf-…@0`, `@01`, uppercase hex, 15 hex, trailing space, `design:cv-7@11` (too short: 4–32 hex), `none:`, `none:other`, `""`; as built also a design key `@01` and `@1000000000` (mutant R8-P2), and (fix round 1) a lower-case letter past `f` in the brief id and in the design hex (mutants RVC, RVD) and a digit among the design letters | a lenient parser the desktop could lean on |
| `TestEmptyIsNotAKeyButIsNone` | `Parse("")` is `ErrInvalidKey`; `NoneRef("")` is `resolution: none`, `noneReason: ""` | an empty `contextRef` read as data loss |
| `TestDeclinedIsNoneWithAReason` | `NoneRef(Declined)` → `none`, `declined`, `present: false` | the override indistinguishable from "no brief" |
| `TestBuildBranchKeyShowsInheritedText` | a branch revision that skips audience, with its pinned project revision: `Audience` is the project's text and its section is `OriginProject` | critique-2 §1.1's second consequence — a chip with no audience |
| `TestBuildWithAnErasedPinHoldsNoInheritedText` (re-review b06-1 R-I2) | the same branch revision with `PinErased: true` and `Project` nil → `Inherits.Erased` true with the pin's key and digest, `Audience` `""`, no `OriginProject` section, and no `ErrInheritMismatch`; as built also the title is the key's (mutant R4-P2), and an open revision with an erased pin builds too, `open`, with its own answers (mutant X2) | a branch `Ref` carrying erased project text, or one that fails to build and blanks every chip on the branch |
| `TestBuildTitlesAnOpenRevisionByTheOneRule` (re-review b05-1 R-M2) | `Build` over an open branch revision on `refs/heads/evidence-upload` → `Title` equals `discover.Title` of it (`Branch brief — evidence-upload`) and `BranchRef` equals `Revision.BranchRef`; an open project revision → `Project brief`; as built also `status` `open`, `resolved` and `present` (mutant R4-P1), the one settled answer as its only section, no draft and no inherited text, and `inherits` the whole pin, its revision the pin key's 5 and not the branch revision's own 3 (mutant R6-P2); (fix round 1) both Refs, which have no definition of done, marshal `"criteria":[]` (mutant RVB) | an open history row titled `""`, or by a second hand-written copy of task 05's rule that drifts from it |
| `TestBuildCarriesCriterionDigests` | each criterion's digest is `CriterionDigest(text)`; ids are the content's | phase 3 matching evidence by a digest phase 4 never computed |
| `TestCriterionDigestVector` | `CriterionDigest("Tests pass")` is `sha256:` + the SHA-256 of those 10 bytes, checked against a literal computed by `printf 'Tests pass' \| sha256sum` when the test is written | a digest over a trimmed, lower-cased or JSON-quoted text |
| `TestNilEvaluateMeansOpen` | `Readings(ctx, key, src, at, nil)`: one reading per criterion of `Build`, same ids and order, every state `open`, every `because` `""` | a default of `settled` |
| `TestUnexplainedVerdictIsShownOpen` | an `Evaluate` returning `settled` with `Because` `nil`, and another with `[]string{"  "}`, each gives a reading `open`; as built also `{}` and `{"", "\t\n"}`, and each for `contested` too (mutant R3-P1) | P12 |
| `TestOneEvaluateCallPerKey` (re-review b08-1 R-I1) | a counting `Evaluate` over a key with three criteria is called once, with the caller's `ctx`, all three in order, `BriefKey` the key, `ProjectID` from `src` and `BranchRef` the caller's `at` | a per-criterion loop: three git walks for one chip |
| `TestEvaluateFailureShowsOpen` (re-review b08-1 R-I1) | an `Evaluate` returning an error, one returning two verdicts for three criteria, and one returning them out of order, each gives every reading `open` with `because` `""`; as built also four for three, none, a stranger's id, a query the `Evaluate` reorders in place (mutant P2: the readings keep `Build`'s order), and `open` verdicts that give a because (mutant R2-P1) | a git error surfacing as a verdict, or a verdict pinned to the wrong criterion |
| `TestBecausesJoin` (re-review b08-1 R-I1) | `contested` with `Because` `{" CI run 88 passed ", "", "review said no"}` gives `because` `"CI run 88 passed; review said no"` | only the first because shown, or an empty one joined in |
| `TestNoBranchIsNotEvaluated` (re-review b08-1 R-I1; replaces `TestProjectKeyIsNotEvaluated`, re-review b08-2 R2-I1) | a project key and a branch key, each with a non-nil `Evaluate` and `at` `""` (Resolve's call for a project key), `main`, `refs/heads/`, a tag, `heads/…` or a remote ref: `Evaluate` is never called and every reading is `open` | a subject guessed where the caller named no branch |
| `TestAProjectKeyIsEvaluatedAtTheCallersBranch` (re-review b08-2 R2-I1) | `b-7@5` with `at` `refs/heads/fix-session-timeout`: one call, `BranchRef` that branch, and its verdicts shown | every branch that inherits the project brief showing `DefinitionOfDone` all `open` for ever |
| `TestTheBranchEvaluatedIsTheCallers` (re-review b08-2 R2-M1) | `b-42@5` (envelope `evidence-upload`) with `at` `refs/heads/evidence-upload-v2`: the query's `BranchRef` is `at` | a subject read from the envelope, gone after a rename or taken by an unrelated branch |
| `TestRefHoldsNoCriterionState` (review I14) | the JSON of a built `Ref` has no `state` or `because` key at any depth under `criteria`; an `Evaluate` that answers `settled` then `contested` for the same key gives two different `Readings` and byte-identical `Build` JSON | a verdict frozen inside a `Ref` the desktop caches for ever |
| `TestRefHasNoDenominator` | the JSON of a built `Ref` has no key matching `(?i)met\|total\|percent\|ratio\|count` | a count sneaking back (G5) |
| `TestErasedKeepsKeyAndDigest` | `ErasedRef` has key, digest, savedBy and erasedAt, no sections, `present: true` | an erased brief shown as missing (critique-2 §6.2 item 5) |
| `TestRefGoldenJSON` | a built `Ref` for the fact sheet's `b-42@5` marshals to a checked-in golden — **the wire contract task 15 mirrors** | a field renamed on one side only |

Added as built (each pins a rule the table's rows leave open):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestKeyStringNeverWritesANonKey` | a hand-built `Key` with a bad id, revision 0 or -1, an unknown kind or reason: `String()` is `""`; (fix round 1) so is a `Key` whose string parses back to a different `Key`: a stray reason on a brief or design key, a stray id on a none key (mutant RVA) | `String` minting a string that looks like a key and is not one |
| `TestBriefKey` | the one writer: `brief:<id>@<n>`, 999999999 accepted; `b-42`, uppercase, a lower-case letter past `f` (fix round 1), a canvas id, `""`, a 17-hex id, a trailing line feed, a prefix before `bf-` (mutants R6-P1, R6-P1b: the id check unanchored), 0, -1 and 1000000000 panic | a writer laxer than the parser |
| `TestNoneRefRefusesAKey` | `NoneRef` of a brief key, `none:other`, `none:`, `" "`, `declined` panics | a real key shown as "no brief" by a caller that skipped `Parse` |
| `TestBuildRefusesASourceTheKeyDoesNotName` | 21 rows, every one `ErrBadSource` (fix round 1, review m5), discover's own error beside it where discover refused: another revision's or brief's key, a design key (also on a revision relabelled to agree with it, mutant R9-P2), `b-42`, `""`, `none:declined`, an envelope key that differs, an envelope whose id or number is another's (mutant X1), a pin passed to a project revision or held by one (mutant P1), an erased pin beside a pin in hand or on a revision that pins nothing, a pin that is not a brief key, `brief:b-7@5` or a design key with the project passed under it (mutant R9-P1) (refused before discover, never `discover.ErrInheritMismatch`); the project head instead of the pin, no pin passed (`discover.ErrInheritMismatch` too); and (fix round 1) an open pin (`discover.ErrIncomplete` too), a branch revision with no branchRef and a scope that is neither (discover's unexported refusal). Each Ref is `unavailable` with no text | one key on another revision's text |
| `TestAnOpenRevisionIsEvaluated` | an open branch revision with two criteria: one call, both readings shown | an open history row never evaluated (mutant R3-P2), though `Readings` runs on every answer |
| `TestHeadsAreCarriedAsGiven` | head `{7, 6}` (an open head above the latest complete): `Build` and `ErasedRef` carry 7 and 6 | the two swapped (mutants R2-P2, R2-P3): every other row's head is `{6, 6}` |
| `TestAnUnknownStateIsShownOpen` | a verdict `SETTLED` on one criterion: that one `open`, the others shown | a state outside the three reaching the chip |
| `TestReadingsOfAKeyWithNoCriteria` | a key with no definition of done, and a source `Build` refuses: `[]`, no call | a git walk for nothing, or readings of criteria no `Ref` holds |
| `TestReadingAndHeadJSON` | `Readings` over `b-42@5` (first criterion `settled` because `CI run 88 passed`) marshals to `[{"id":…,"state":"settled","because":"CI run 88 passed"},{…,"state":"open","because":""},…]`, and `Head{7, 6, true}` to `{"headRevision":7,"latestCompleteRevision":6,"archived":true}`: the two shapes task 12 serves beside the `Ref`, which the golden does not hold | a reading's or a head's tag renamed on one side only (mutants R7-P1, R7-P2) |
| `TestSlotRefs` | `MissingRef`, `UnavailableRef` (a brief key's id and revision filled from the key; a design key's not), `InvalidRef`, `NoneRef`: every other field zero, `sections` and `criteria` `[]`, `inherits` `null` | a `null` slice on the wire, or text on a slot with no revision |

**Steps:**

- [ ] **Step 1:** Write both test files and `testdata/ref-golden.json`; confirm they fail.
- [ ] **Step 2:** `key.go`: three anchored regexps, compiled once.
- [ ] **Step 3:** `ref.go`: `Build` calls `discover.Resolve`, maps sections to the four named
  fields, criteria from the done section's `Items`, digests. `Readings` builds the same
  criteria, calls `eval` once with all of them, and applies its four rules. The adapter is not
  this task's; see "The adapter to phase 3's evaluator" above, and task 14.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/provenance/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance
$ grep -rn "strings.Split\|strings.Cut\|regexp" --include=*.go control-plane/internal | grep -i "brief:\|design:\|@" | grep -v "internal/provenance/"
(no output — no other package splits a key; re-run in task 14 and task 23)
```

**Does NOT claim:** criterion evaluation (phase 3's, S4-P3.4); the adapter's key-to-subject rule
(`cmd/api/evidence.go`, written by the second of phase 4 task 14 and phase 3 task 15); the `design:` kind's resolution
(the Hub resolves design keys for phase 5; phase 4 only parses them so the grammar is one).

**As built** (code `05d32e7`, mutation killers `86f07f3` and after; each departure is in the source
above and in a test):

1. **`Build` returns `(Ref, error)`.** The part gave it no error, yet `discover.Resolve` refuses a
   pin that is not the revision's (`ErrInheritMismatch`), an incomplete pin and a malformed
   revision, and nothing said what `Build` does then. A silent fallback would put one key on
   another revision's text, or blank every chip on the branch. So a source the key does not name
   (another revision's or brief's key, a key that is not a brief key, an envelope `Key` that
   differs, a pin passed to a project revision or held by one, an erased pin beside a pin in hand
   or on a revision that pins nothing, a pin that is not a brief key) is `ErrBadSource`, and a
   pin that is not the revision's is `discover.ErrInheritMismatch`. The `Ref` beside either is
   `UnavailableRef(key)`. **Fix round 1 (review m5):** every error `Build` returns is now
   `ErrBadSource`. What `discover.Resolve` refuses (a pin that is not the revision's, an open
   pin, an envelope its scope never holds) keeps discover's error in the chain beside it, so
   `errors.Is(err, discover.ErrInheritMismatch)` still holds where it did. Before, an open pin
   (`discover.ErrIncomplete`) and discover's unexported `errBadRevision` came back as neither
   class the contract named. Task 07 never builds such a source (its rule 3 fetches the pin by
   `inherits.key`), so the error is a caller fault: task 07 logs it and serves the slot
   `unavailable`.
2. **`Readings` takes `at`, the branch it evaluates at** (re-review b08-2 R2-I1 and R2-M1, both
   carried to this task). "A project key is never evaluated" left `DefinitionOfDone` all `open`
   for ever on every branch that inherits the project brief, which is every new branch. It also
   misread phase 3, whose `States` takes a commit and never a branch id. And the envelope's
   `BranchRef` names a gone or reused ref after a rename. The rule is now the caller's:
   `HeadRef(t)` passes `t`'s branch, a project key included, and `Resolve` passes `""` for a
   project key and, for a branch key, the branch its brief is bound to now. An `at` that is not
   `refs/heads/<name>` is never evaluated. `TestProjectKeyIsNotEvaluated` became
   `TestNoBranchIsNotEvaluated`, and two tests were added. **Carried, not built here:**
   - task 07 must pass `at` as above, reading the brief's current `branchRef` for `Resolve`
     (`BriefHead.branchRef`, Hub contract §7.1 `:641-642` at hub `da8c7da`; `/v1/refs`' `heads`
     do not carry it, §7.5 `:829`). **Fix round 1 (review I1)** wrote where it comes from, which
     this bullet left open while task 07 still said "at most two Hub calls", both `/v1/refs`:
     task 07's rule 8 reads it with `List`, once per Hub project, only when `Config.Evaluate`
     is set, and gives `""` to an archived brief or a failed read. Ask HA4-13 (plan §9) puts
     `branchRef` on `heads`. Task 07 has the row that kills `at = src.Revision.BranchRef`
     (`TestResolveEvaluatesABranchKeyWhereItsBriefIsBoundNow`);
   - task 16 must store a head reading's states by target, not by key, since one project key
     inherited by two branches has two sets (plan S4-P3.4, amended);
   - phase 3's cross-phase ask gains two items (the adapter section above).
3. **`Readings` edges.** With no criteria there is no call and the result is `[]`. Where `Build`
   would refuse the source, the result is `[]` with no call. A verdict state outside `open`,
   `settled` and `contested` is shown `open`, on that criterion alone.
4. **An open revision** is resolved by `discover.Resolve` on a copy marked complete with no pin,
   so the one section builder serves both paths and a held draft still renders nothing.
   `Ref.Status` stays the stored status. `Ref.Inherits` is the content's pin, open or not. A
   project revision passed beside an open **branch** revision is ignored: an open revision
   inherits nothing.
5. **`MissingRef` and `UnavailableRef`** fill `briefId` and `revision` from a brief key. The
   desktop never parses a key (P5), and this way it can still say which revision a slot is. A
   design key fills neither.
6. **`NoneRef`** panics on anything but `""` or a `none:` key, as `BriefKey` does on a bad id: a
   caller that skipped `Parse` would otherwise show a real key as *no brief*.
7. **`Key.String`** writes `""` for a `Key` the grammar would not parse back. **`BriefKey`**
   refuses revisions above 999999999, the grammar's nine digits.
8. **The golden** is the fact sheet's `b-42@5` (§2) over `b-7@5` (§3), with task 05's ids (`b-7`
   = `bf-00000000000000b7`, `b-42` = `bf-00000000000000a2`), Hub project `pj-00000000000000e1`,
   head `{6, 6}`, placeholder revision digests, and criterion digests computed with `printf |
   sha256sum`. It was written by hand and matched `Build` on the first run.
   `testdata/.gitattributes` is `* -text`.
9. **Adapter rule 3** (re-review b08-2 R2-M2): phase 3 exports no repo rule, so the ask carries
   one. The adapter is still not this task's.
