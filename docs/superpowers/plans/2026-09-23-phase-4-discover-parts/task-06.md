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

type Criterion struct {
	ID      string         `json:"id"`
	Text    string         `json:"text"`
	Digest  string         `json:"digest"`  // "sha256:" + hex(SHA-256(UTF-8 of Text)) — phase 3's P4-2
	State   CriterionState `json:"state"`   // "open" throughout phase 4
	Because string         `json:"because"` // required when State != open; "" when open
}

type InheritedRef struct {
	Key      string `json:"key"`
	BriefID  string `json:"briefId"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
}

// Ref is one brief key, resolved. Cacheable for ever EXCEPT headRevision and
// latestCompleteRevision, which the desktop keeps per brief (contract §6.4).
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

// Evaluate is phase 3's seam (ask P4-4): the state of one criterion of one
// brief key. Nil means every criterion is open - phase 4's only behaviour.
type Evaluate func(briefKey string, c discover.Criterion) (CriterionState, string)

// Source is a resolved revision as discoverhub hands it over.
type Source struct {
	Revision  discover.Revision  // the revision the key names
	Project   *discover.Revision // its pinned project revision (branch keys), or nil
	Digest    string
	ProjectID string
	SavedAt   string
	SavedBy   string
	// No Names: discover.Resolve takes none, and the title comes from
	// Revision.BranchRef, the envelope's own ref (review I5, task 05).
	// No BranchRef either: Ref.BranchRef is Revision.BranchRef, so the fact
	// has one carrier and two copies cannot disagree (re-review b05-1 R-M2).
}

// Build resolves a complete revision through discover.Resolve and fills
// every field; Ref.Title is the Document's Title and Ref.BranchRef is
// src.Revision.BranchRef. An open revision (only a history row can name one)
// gets Sections from its own settled answers, no inheritance, Status "open",
// and Ref.Title = discover.Title(&src.Revision) for a branch revision or
// discover.Title(nil) for a project one - task 05's one copy of the rule,
// never a string written here (re-review b05-1 R-M2).
// An Evaluate that returns a non-open state with an empty because is
// overridden to open: an unexplained verdict is not shown (P12).
func Build(key string, src Source, head Head, eval Evaluate) Ref

func MissingRef(key string) Ref
func InvalidRef(key string) Ref
func UnavailableRef(key string) Ref
func NoneRef(contextRef string) Ref // "" or none:declined
func ErasedRef(key string, briefID string, revision int, digest, savedAt, savedBy, erasedAt string, head Head) Ref

func CriterionDigest(text string) string
```

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestParseAcceptsTheHubsForms` | Hub contract §3.2's examples parse; `String()` round-trips | a grammar narrower than the Hub's |
| `TestParseRejects` — table | `brief:b-42@3` (a pre-Hub id), `brief:bf-…@0`, `@01`, uppercase hex, 15 hex, trailing space, `design:cv-7@11` (too short: 4–32 hex), `none:`, `none:other`, `""` | a lenient parser the desktop could lean on |
| `TestEmptyIsNotAKeyButIsNone` | `Parse("")` is `ErrInvalidKey`; `NoneRef("")` is `resolution: none`, `noneReason: ""` | an empty `contextRef` read as data loss |
| `TestDeclinedIsNoneWithAReason` | `NoneRef(Declined)` → `none`, `declined`, `present: false` | the override indistinguishable from "no brief" |
| `TestBuildBranchKeyShowsInheritedText` | a branch revision that skips audience, with its pinned project revision: `Audience` is the project's text and its section is `OriginProject` | critique-2 §1.1's second consequence — a chip with no audience |
| `TestBuildTitlesAnOpenRevisionByTheOneRule` (re-review b05-1 R-M2) | `Build` over an open branch revision on `refs/heads/evidence-upload` → `Title` equals `discover.Title` of it (`Branch brief — evidence-upload`) and `BranchRef` equals `Revision.BranchRef`; an open project revision → `Project brief` | an open history row titled `""`, or by a second hand-written copy of task 05's rule that drifts from it |
| `TestBuildCarriesCriterionDigests` | each criterion's digest is `CriterionDigest(text)`; ids are the content's | phase 3 matching evidence by a digest phase 4 never computed |
| `TestCriterionDigestVector` | `CriterionDigest("Tests pass")` is `sha256:` + the SHA-256 of those 10 bytes, checked against a literal computed by `printf 'Tests pass' \| sha256sum` when the test is written | a digest over a trimmed, lower-cased or JSON-quoted text |
| `TestNilEvaluateMeansOpen` | every state `open`, every `because` `""` | a default of `settled` |
| `TestUnexplainedVerdictIsShownOpen` | an `Evaluate` returning `settled, ""` gives `open` | P12 |
| `TestRefHasNoDenominator` | the JSON of a built `Ref` has no key matching `(?i)met\|total\|percent\|ratio\|count` | a count sneaking back (G5) |
| `TestErasedKeepsKeyAndDigest` | `ErasedRef` has key, digest, savedBy and erasedAt, no sections, `present: true` | an erased brief shown as missing (critique-2 §6.2 item 5) |
| `TestRefGoldenJSON` | a built `Ref` for the fact sheet's `b-42@5` marshals to a checked-in golden — **the wire contract task 15 mirrors** | a field renamed on one side only |

**Steps:**

- [ ] **Step 1:** Write both test files and `testdata/ref-golden.json`; confirm they fail.
- [ ] **Step 2:** `key.go`: three anchored regexps, compiled once.
- [ ] **Step 3:** `ref.go`: `Build` calls `discover.Resolve`, maps sections to the four named
  fields, criteria from the done section's `Items`, digests, `eval`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/provenance/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance
$ grep -rn "strings.Split\|strings.Cut\|regexp" --include=*.go control-plane/internal | grep -i "brief:\|design:\|@" | grep -v "internal/provenance/"
(no output — no other package splits a key; re-run in task 14 and task 23)
```

**Does NOT claim:** criterion evaluation (phase 3's, S4-P3.4); the `design:` kind's resolution
(the Hub resolves design keys for phase 5; phase 4 only parses them so the grammar is one).
