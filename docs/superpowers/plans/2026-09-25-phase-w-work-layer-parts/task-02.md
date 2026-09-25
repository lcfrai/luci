### Task 02: `hubapi`: Work id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision` canonical kind and golden vectors

**Slice:** W1. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 01 (SW.0's grammar line); Hub tasks H01-H03 (`ids.Wide`, `hubapi.Canonical`,
`DigestOf`, the wire types), sync-merged from `feat/hub` into `feat/work-layer` by the orchestrator
first (rule H1). **Seams:** defines **SW.1**, first half (the id prefixes, the `issue:` key,
`DisplayAlias`, the `issue-revision` canonical form and its golden vectors). Task 03 adds the
second half; both freeze at WS0 with task 05.

An issue is on the provenance thread only if a key can name one revision of it, and that key can
be checked. This task gives `hubapi` the three things every later Work task leans on: **the ids**
the work layer mints, **the key** `issue:is-…@r` in the Hub's grammar, and **the digest** of an
issue revision, computed by the same one function the Hub uses for briefs (hub-contract §3.4: *"No
TypeScript implementation exists or may be written"*). Nothing here stores or serves an issue.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`** (paths under
`control-plane/internal/hub/hubapi/`; the scope read `48351f8`, and every anchor still holds):
`keys.go:15-27` (the `Prefix*` const block), `:31-35` (`ValidID`), `:47-52` (`KeyKind`),
`:54` (`ErrInvalidKey`), `:56-59` (`briefKeyRe`, `designKeyRe`), `:61-77` (`BriefKey`,
`ParseBriefKey`), `:96-105` (`ParseKey`), `:114` (`DigestOf`); `canonical.go:39`
(`Canonical(v any) ([]byte, error)`); `envelope.go:29-32` (`MaxEnvelopeBytes = 64 << 10`),
`:34-46` (`ErrBadEnvelope`), `:49-52` (`kindBrief`, `kindAudit`), `:54` (`bad`), `:57-66`
(`under`), `:127-191` (`BriefEnvelope`, `Canonical`, `Digest`), `:234-262` (`readEnvelope`);
`keys_test.go:683` (`constValues`), `:719` (`prefixEntity`), `:737-775`
(`TestPrefixesAndReasonsAreTheContracts`); `ids/ids.go:61` (`Wide`). Contract: §3.1
(`hub-contract:91`), §3.2 (`:114`), §3.4 (`:139`), §3.5 (`:176`). 04 §2.4 is
`04-internal-fit.md:159-190`.

Six decisions the scope left open, made here.

**(a) The Work prefixes live in `workids.go`, never in `keys.go`'s const block.**
`TestPrefixesAndReasonsAreTheContracts` (`keys_test.go:756-758`) reads every `Prefix*` constant
**of `keys.go`** and fails unless there are exactly ten, matching §3.1's ten rows. A Work prefix in
`keys.go` breaks a Hub test; one in `workids.go` does not, and task 02 edits neither §3.1 nor that
test (rule H1, additive only). Phase W's own disjointness test (below) holds the twelve.

**(b) `KeyIssue`, `issueKeyRe`, `IssueKey` and `ParseIssueKey` do go in `keys.go`**, because the
grammar is one table (hub-contract §3.2) and `ParseKey` is one `switch`. These are appended lines;
no existing line of `keys.go` changes. The `feat/hub` owner has the proposal (task 01) and the
orchestrator's merge carries it; on a conflict the Hub's file wins whole and these lines are
re-applied (rule H1).

**(c) `DisplayAlias` has no inverse.** It formats `"EP-142"`, and a test proves `ParseKey` refuses
it. No `ParseDisplayAlias` exists; `IssueByNumber(projectID, number)` (task 04) is how a person's
`EP-142` becomes an issue, with the project already known from context (05 §3.6).

**(d) The envelope is `{v, kind, issueId, revision, projectId, content}`**, six members. It carries
no `scope` or `branchRef` (an issue is project-scoped; a feature's branch is W2's `featureId` on
the head, never in a digest) and no state, priority or assignee (the head, never digested: 04
§2.4, *"reprioritising all afternoon writes no revision"*).

**(e) Title length counts code points; description length counts UTF-8 bytes.** A title is a
name a person reads (≤ 200 characters, `utf8.RuneCountInString`, the same count Postgres's
`char_length` gives, so task 06's `CHECK` refuses exactly what this does). A description is a
document (≤ 16 KiB = 16,384 bytes, `len`, Postgres's `octet_length`). A title is also 1 or more
characters after trimming spaces; a description may be empty.

**(f) Character hygiene is not this task's.** `ValidateIssueRevisionContent` checks shape and
limits. The Unicode checks (the tag block, bidi controls) run where the Hub accepts a write
(task 09) with the same `teamtext` rules Phase T's task 02 builds, if that package has landed on
the sync base, else as task 09 names. This task makes no claim about them.

**Files** (all under `control-plane/internal/hub/hubapi/`, on `feat/work-layer` after the sync):

| Path | Change |
|---|---|
| `keys.go` ⚑ | **append only**: `KeyIssue`, `issueKeyRe`, `IssueKey`, `ParseIssueKey`, one `ParseKey` case |
| `keys_test.go` ⚑ | **append only**: `TestIssueKey*` (three functions) |
| `workids.go` | new: the twelve Work prefixes, `WorkPrefixes`, `ValidProjectKey`, `DisplayAlias` |
| `workids_test.go` | new |
| `workcanonical.go` | new: `AuthoredText`, `Serve`, `IssueRevisionContent`, `IssueRevisionEnvelope`, `IssueRevisionDigest`, `ParseIssueRevisionEnvelope`, `ValidateIssueRevisionContent` |
| `workcanonical_test.go` | new |
| `testdata/issue-revision/v1.json`, `v2.json` | new: the golden vectors |

No other file changes; `git diff --stat` after the task lists exactly these nine.

---

#### Interfaces, complete

**`keys.go`: the lines appended.** Three edits, each additive. Into the `KeyKind` const block
(`keys.go:49-52`), after `KeyDesign`:

```go
	KeyIssue  KeyKind = "issue" // Phase W; contract §3.2 as proposed in w0-hub-contract-proposal.md
```

Into the regex block (`keys.go:56-59`), after `designKeyRe`:

```go
	issueKeyRe  = regexp.MustCompile(`^issue:(is-[0-9a-f]{16})@([1-9][0-9]{0,8})$`)
```

Into `ParseKey`'s `switch` (`keys.go:97-102`), after the `designKeyRe` case:

```go
	case issueKeyRe.MatchString(s):
		return KeyIssue, nil
```

And after `ParseDesignKey` (before `ParseKey`), the type and its parser, shaped exactly like
`BriefKey`:

```go
// IssueKey names one immutable issue revision: issue:<issueId>@<revision>.
// The revision is the Hub's, gapless per issue from 1; 0 never appears in a key.
// "EP-142" is a display alias, never a key: see DisplayAlias.
type IssueKey struct {
	IssueID  string
	Revision int // 1 … 999,999,999
}

func (k IssueKey) String() string { return "issue:" + k.IssueID + "@" + strconv.Itoa(k.Revision) }

func ParseIssueKey(s string) (IssueKey, error) {
	m := issueKeyRe.FindStringSubmatch(s)
	if m == nil {
		return IssueKey{}, ErrInvalidKey
	}
	n, _ := strconv.Atoi(m[2]) // the grammar bounds it to nine digits
	return IssueKey{IssueID: m[1], Revision: n}, nil
}
```

`KeyIssue` and `issueKeyRe` are as long as `KeyBrief` and `briefKeyRe`, so `gofmt` realigns
nothing: `git diff keys.go` shows only added lines (`git diff --numstat` deletions `0`).

**`workids.go`, complete.**

```go
package hubapi

import (
	"regexp"
	"strconv"
)

// Work id prefixes (Phase W, 05-design §3.6). Each id is minted with
// ids.Wide(prefix) and checked with ValidID(prefix, id). They are declared here,
// not in keys.go, because TestPrefixesAndReasonsAreTheContracts holds keys.go
// and contract §3.1 to exactly ten prefixes each. Disjoint from §3.1's ten, the
// control plane's s- p- b- w- m-, and Phase T's twelve (TestWorkPrefixesAreDisjoint).
// Status updates reuse Phase T's nt- (note); "dc" (free documents, L-120) is
// reserved for a later row and is not declared until that row is built.
const (
	PrefixIssue        = "is"
	PrefixIssueComment = "ic"
	PrefixFeature      = "ft"
	PrefixMilestone    = "ml"
	PrefixCycle        = "cy"
	PrefixInitiative   = "in"
	PrefixRequest      = "rq"
	PrefixLabel        = "lb"
	PrefixTemplate     = "tp"
	PrefixView         = "vw"
	PrefixLoop         = "lp"
	PrefixDraft        = "df"
)

// WorkPrefixes is every prefix above, in declaration order.
var WorkPrefixes = []string{PrefixIssue, PrefixIssueComment, PrefixFeature, PrefixMilestone,
	PrefixCycle, PrefixInitiative, PrefixRequest, PrefixLabel, PrefixTemplate, PrefixView,
	PrefixLoop, PrefixDraft}

var projectKeyRe = regexp.MustCompile(`^[A-Z]{2,5}$`)

// ValidProjectKey reports whether k is a project key: 2 to 5 capital letters
// A-Z, set by a team admin (04-internal-fit §2.4). Unique per team (the store's).
func ValidProjectKey(k string) bool { return projectKeyRe.MatchString(k) }

// DisplayAlias is how a person names an issue: "EP-142". It is formatted at
// read time from the project's current key and the issue's number, never
// stored and never accepted where a key is expected: an admin can change the
// project key, and every alias changes with it while every key stays.
//
// No parser for an alias exists or may be written (05-design §3.6). A person's
// "EP-142" becomes an issue through IssueByNumber, with the project already
// known from where they typed it.
func DisplayAlias(projectKey string, number int) string {
	return projectKey + "-" + strconv.Itoa(number)
}
```

`ValidID` (`keys.go:32`) already takes any prefix, so Work ids need no checker of their own:
`ValidID(PrefixIssue, id)`.
