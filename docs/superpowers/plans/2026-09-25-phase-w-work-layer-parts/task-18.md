### Task 18: phase 4 delta — `issue:` in `provenance/key.go`; `Ref.Kind` `"issue"`

W1, stream WP, seam **SW.1** (the parser side). Depends on task 02 (the grammar and
`hubapi.KeyIssue`) and phase 4 task 06 (`provenance/key.go` and `ref.go`, the only key parser in
the product, P5; phase 4 plan `2026-09-23-phase-4-discover.md`, file map rows `provenance/key.go`
and `provenance/ref.go`).

Phase 4 made one rule and it is the reason this task exists: **nothing but `provenance` may split a
key** (P5, P4 `task-06.md:3-4`). An issue key, `issue:is-3f2a91c07b1e4d2a@3`, is about to reach the
control plane from three places (the Materialiser, task 19; Start, task 24; the desktop's loader,
task 19) and every one of them would be tempted to `strings.Cut` it. This task gives them the one
parser instead, with the Hub's grammar verbatim, and it gives `Ref` a **kind**, which it has never
needed: until now every `Ref` was a brief, so a surface could assume it. From this task on a
surface that reads a `Ref` must look at `kind` first, and a zero-valued kind would be read as a
brief by every surface phase 4 built.

**Rule H2 applies in full.** The owner of `key.go` and `ref.go` is phase 4. This task **waits**
until P4-06 is on the sync base, never writes a second copy of either file, and the owner's file
wins every conflict; this part describes additions only.

**What is on the tree today (verified 2026-09-25).** `.claude/worktrees/phase-4` is at `f700613`;
its `control-plane/internal/` holds `api`, `backend`, `claudemd`, `discover`, `harness`, `httpx`,
`ids`, `modelclient`, `project`, `proxy`, `reaper`, `registry`, `sandbox`, `statepath` and
`worktree`, and **no `provenance` directory**: P4-06 is planned, not built. Its part
(`task-06.md:24-54` for `key.go`, `:122-148` for `Ref`, `:234-238` for the constructors) is what
this part is written against, **UNVERIFIED until merge**. `desktop/src/api/discover.ts` is P4-15's,
also planned (`task-15.md:161-168` declares `ProvenanceRef`). On `feat/hub` at `9f2fa31`,
`hubapi.RefResult` (`hubapi/types.go:322-328`) has `Kind *KeyKind`, null exactly when the
resolution is `invalid`; task 09 adds `Issue *IssueRevision` to it.

**Files:**
- `control-plane/internal/provenance/key.go` ⚑ — `KindIssue`, the grammar line, `Parse`'s case,
  `String`'s case, `IssueKey`
- `control-plane/internal/provenance/key_test.go` ⚑ — the issue rows; the two-parser table
- `control-plane/internal/provenance/ref.go` ⚑ — `Ref.Kind`; the issue field group; `Serve`;
  `IssueSource` and `BuildIssue`; `ErasedIssueRef`; every constructor sets `Kind`
- `control-plane/internal/provenance/ref_test.go` ⚑ — `kind` on every constructor; the issue golden
- `control-plane/internal/provenance/testdata/ref-issue.golden.json` — the issue `Ref` on the wire
  (new)
- `desktop/src/api/discover.ts` ⚑ — `ProvenanceRef` gains `kind` and the issue fields; `Serve`

**Interfaces:**
- Produces: `provenance.KindIssue`, `provenance.IssueKey(issueID, revision) string`, `Parse`
  accepting `issue:`; `Ref.Kind` and `Ref.{IssueID, IssueNumber, IssueAlias, Serves}`;
  `provenance.Serve`, `provenance.IssueSource`, `provenance.BuildIssue`,
  `provenance.ErasedIssueRef`; the TypeScript mirror of each.
- Consumes: P4-06's `Kind`, `Key`, `Parse`, `String`, `BriefKey`, `ErrInvalidKey`, `Ref` and its six
  constructors; task 02's `hubapi.KeyIssue` and `ParseIssueKey` (for the two-parser table only,
  in the test).
- Required by: task 19 (the Materialiser, `Session.IssueRef` and the resolver's `issue:` branch),
  task 24 (Start stamps an issue key), task 67 (the issue chip reads `kind`), task 37 (the desktop's
  Work types reuse `Serve`).

#### Contract — the additions to `key.go`, complete

```go
// control-plane/internal/provenance/key.go ⚑ — the const block gains one line
const (
	KindBrief  Kind = "brief"
	KindDesign Kind = "design"
	KindIssue  Kind = "issue" // task 18, W1
	KindNone   Kind = "none"
)

// The grammar comment gains one line, verbatim the Hub's (task 02's
// issueKeyRe in hubapi/keys.go; hub-contract §3.2 once task 01's proposal
// lands):
//
//	issue   ^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$

var issueKeyRe = regexp.MustCompile(`^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$`)

// IssueKey is BriefKey for an issue. It panics on an id that is not is- and
// 16 lower-case hex, or on a revision below 1: a programming error, never
// input — input goes through Parse.
func IssueKey(issueID string, revision int) string {
	k := Key{Kind: KindIssue, ID: issueID, Revision: revision}
	if _, err := Parse(k.String()); err != nil || revision < 1 {
		panic(fmt.Sprintf("provenance: IssueKey(%q, %d): not an issue key", issueID, revision))
	}
	return k.String()
}
```

`Parse` gains one `case`, placed with the brief and design cases and matched the same way (the
regular expression, then `strconv.Atoi` on the second group), filling
`Key{Kind: KindIssue, ID: m[1], Revision: rev}`. `String` gains the matching case,
`"issue:" + k.ID + "@" + strconv.Itoa(k.Revision)`. **`EP-142` is not a key** and no parser for it
is written here or anywhere (task 02: `DisplayAlias` has no inverse, because an admin can change
the project key and every alias with it).

#### Contract — the additions to `ref.go`, complete

```go
// control-plane/internal/provenance/ref.go ⚑

// Serve is one pair of an issue revision's serves: the brief key and the
// criterion of it the issue says it serves (05 §2.3). hubapi.Serve's pair; not its Check.
type Serve struct {
	BriefKey    string `json:"briefKey"`
	CriterionID string `json:"criterionId"`
}

// In type Ref, directly after Key (so the golden reads kind second):
	// Kind is the key's kind: brief, design, issue or none. "" only for an
	// invalid key, as hubapi.RefResult.kind is null exactly then. A surface
	// switches on it before reading any other field (task 18).
	Kind Kind `json:"kind"`

// In type Ref, after ErasedAt — the issue group, zero for every other kind:
	IssueID     string  `json:"issueId"`     // is-…; "" unless kind is issue
	IssueNumber int     `json:"issueNumber"` // 142; 0 unless kind is issue
	IssueAlias  string  `json:"issueAlias"`  // "EP-142" at resolve time; display only, never parsed
	Serves      []Serve `json:"serves"`      // the revision's serves; [] (never null) for every kind
```

```go
// control-plane/internal/provenance/ref.go ⚑ (continued)

// IssueSource is one resolved issue revision, as the resolver hands it over
// (task 19 fills it from hubapi.RefResult.Issue and RefsResponse.issueHeads).
// provenance imports no Hub type, as P4-06's Source does not.
type IssueSource struct {
	IssueID      string
	Number       int
	Alias        string // hubapi.DisplayAlias at resolve time
	Revision     int
	Digest       string // "sha256:" + hex, as Ref.Digest for briefs
	ProjectID    string // the HUB project id, pj-…
	Title        string // the revision's title text
	Serves       []Serve
	SavedAt      string
	SavedBy      string // display name
	HeadRevision int
}

// BuildIssue is Build for an issue key: kind issue, resolution resolved,
// present, the issue group filled, Title the revision's title. It carries NO
// brief fields: Sections and Criteria are [] and every brief string is "".
// The description is not in a Ref: a Ref is what a chip shows, and the text
// an agent reads is the Materialiser's (task 19), fetched by key.
func BuildIssue(key string, src IssueSource) Ref

// ErasedIssueRef is ErasedRef for an issue: key, digest, savedBy, erasedAt,
// the issue id, number and alias kept; title "", serves [], present true.
func ErasedIssueRef(key string, src IssueSource, erasedAt string) Ref
```

**Every constructor now sets `Kind`**, by one rule: `kindOf(key)` is `Parse(key).Kind` when the key
parses, `KindNone` for `""` and `Declined`, and `""` otherwise. `Build` and `ErasedRef` set
`KindBrief` (they take brief sources only); `BuildIssue` and `ErasedIssueRef` set `KindIssue`;
`MissingRef(key)` and `UnavailableRef(key)` set `kindOf(key)`, so a missing issue key reads kind
`issue` and a missing design key `design`; `NoneRef` sets `KindNone`; `InvalidRef` sets `""`.
Every constructor also sets `Serves` to `[]Serve{}`, never nil, beside P4's existing
`Sections`/`Criteria` empties, so the wire never carries `null` where the desktop indexes.

#### Contract — the desktop mirror

```ts
// desktop/src/api/discover.ts ⚑ (P4-15's file)
export type RefKind = "brief" | "design" | "issue" | "none" | "";
/** provenance.Serve: one pair of an issue revision's serves. */
export interface Serve { briefKey: string; criterionId: string }

// ProvenanceRef gains, in the same order as the Go struct:
//   kind: RefKind;                    (after key)
//   issueId: string; issueNumber: number; issueAlias: string; serves: Serve[];   (after erasedAt)
```

P4-15's rule, *"Every field is always present"* (`task-15.md:160`), holds: a brief `Ref` carries
`issueId: ""`, `issueNumber: 0`, `issueAlias: ""` and `serves: []`. P4-17's `ContextChip` keeps
rendering briefs exactly as before because it is only ever handed a brief ref; task 67 adds the
branch on `kind` for the issue chip.

#### Tests to write first

In `key_test.go` ⚑ and `ref_test.go` ⚑, beside P4-06's own rows, which stay unchanged.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestParseAcceptsAnIssueKey` | `Parse("issue:is-3f2a91c07b1e4d2a@3")` is `Key{Kind: KindIssue, ID: "is-3f2a91c07b1e4d2a", Revision: 3}`; `String()` returns the input byte for byte | a missing `case`; a `String` that drops the kind |
| `TestParseRejectsIssueLookalikes` — table | `issue:is-3f2a91c07b1e4d2a@0`, `@01`, `@1000000000`, upper-case hex, 15 hex, 17 hex, `issue:bf-3f2a91c07b1e4d2a@1`, `issue:is-3f2a91c07b1e4d2a`, a trailing space, `EP-142`, `ep-142@3` — each `ErrInvalidKey` | a grammar looser than the Hub's, which P5 forbids; a parser for the display alias |
| `TestBothParsersAgree` — table | every golden key string of `hubapi`'s key tests (`hubapi/keys_test.go`, H01 and task 02) and every row of this package's parse tables fed to `provenance.Parse` and to `hubapi.ParseKey` (`hubapi/keys.go:96`, which returns the kind only): both accept or both refuse, and on acceptance the kinds agree; for an issue key `hubapi.ParseIssueKey` (task 02) also gives the same id and revision | the two grammars drifting — the one bug P5 exists to prevent |
| `TestIssueKeyPanicsOnBadInput` | `IssueKey("bf-3f2a91c07b1e4d2a", 1)`, `IssueKey("is-3f2a91c07b1e4d2a", 0)` and `IssueKey("is-XYZ", 1)` each panic; `IssueKey("is-3f2a91c07b1e4d2a", 3)` returns the key above | a constructor that builds a key the parser refuses |
| `TestEveryConstructorSetsKind` — table | `Build` → `brief`; `ErasedRef` → `brief`; `BuildIssue`, `ErasedIssueRef` → `issue`; `MissingRef` and `UnavailableRef` of a brief, a design and an issue key → that key's kind; `NoneRef("")`, `NoneRef(Declined)` → `none`; `InvalidRef("x")` → `""`. For each, the JSON has a top-level `"kind"` key | a zero kind the desktop would read as a brief |
| `TestServesIsNeverNull` | the JSON of one ref from every constructor holds `"serves":[]` or a non-empty array, never `null` | `null` reaching a desktop that maps over it |
| `TestAnIssueRefCarriesNoBriefFields` | `BuildIssue`'s JSON has `sections: []`, `criteria: []`, `briefId: ""`, `headline: ""`, `inherits: null` | an issue chip rendering brief text that is not there |
| `TestIssueRefGoldenJSON` | `BuildIssue` for EP-142 revision 3 (serving `c-91e2…` of `brief:bf-…@2`) marshals to `testdata/ref-issue.golden.json` byte for byte — **the wire task 37 and 67 mirror** | a field renamed on one side only |
| `TestRefHasNoDenominator` (P4's, unchanged) | still passes with the issue fields present: none matches `(?i)met\|total\|percent\|ratio\|count` | an issue field named `criteriaMet` or `serveCount` |

The desktop side is a type change only; `npm --prefix desktop run typecheck` is its test, and P4-15's
own `discover.test.ts` round-trip of `ProvenanceRef` gains the four fields and `kind` in its
fixture (one line each), so a missing field fails that test.

#### Steps

- [ ] **Step 1: confirm the sync base.** `grep -n "func Parse" control-plane/internal/provenance/key.go`
  and `grep -n "type Ref struct" control-plane/internal/provenance/ref.go` each print one line, and
  `grep -n "KeyIssue" control-plane/internal/hub/hubapi/keys.go` prints task 02's line. If any is
  empty, stop (rule H2).
- [ ] **Step 2: write the key tests.** `go test ./control-plane/internal/provenance/ -run 'Issue|BothParsers' -count=1`
  fails to compile on `KindIssue` and `IssueKey`.
- [ ] **Step 3: add the key code.** Re-run step 2's command; expect `ok`.
- [ ] **Step 4: write the ref tests and the golden** (the golden is written by hand from the
  struct above, never generated by the code under test). `go test ./control-plane/internal/provenance/ -count=1`
  fails on `Kind`, `BuildIssue` and the golden.
- [ ] **Step 5: add the ref code.** Re-run; expect `ok`, with every P4-06 test still passing.
- [ ] **Step 6: the desktop mirror.** Add the fields; run `npm --prefix desktop run typecheck`
  and `npm --prefix desktop test -- src/api/discover.test.ts`; expect both to pass.
- [ ] **Step 7: commit** the six paths above by path.

#### Acceptance

```bash
go test ./control-plane/internal/provenance/... -race -count=1
# expect: ok  …/control-plane/internal/provenance
go test ./control-plane/... -count=1
# expect: no FAIL line (phase 4's tests pass unchanged)
npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc
```

#### What this task does NOT claim

- The Materialiser, `Session.IssueRef`, the loader's second key or the resolver's `issue:` branch
  (task 19).
- Any surface: no chip renders an issue here (task 67).
- That a resolved issue ref is served anywhere yet: until task 19, `BuildIssue` has no caller but
  its tests.

Parity rows served: the **parser half of L-191** (what the agent was told is named by key; closed
with task 19) and of 05 §2.3's thread. No L row closes here alone.

#### Gaps found while writing this part

1. **The resolver's `issue:` branch has no task.** P4-07's `discoverhub/read.go` `Resolve(keys)`
   (P4 plan file map) maps Hub results to `Ref`s; an `issue:` key through it needs a case calling
   `BuildIssue` / `ErasedIssueRef`. Neither scope 18 nor 19 names that file; this part assigns it
   to task 19, whose loader test needs it.
2. **`ErasedIssueRef` and `IssueSource`** are named here, not in the skeleton; they are the
   smallest additions that let `kind` be set by every constructor as scope 18 requires.

