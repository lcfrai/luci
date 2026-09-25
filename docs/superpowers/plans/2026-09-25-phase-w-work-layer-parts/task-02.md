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

**`workcanonical.go`, complete** (in two blocks; the file is their concatenation). The content
types are declared **once, here**; task 03's `work.go` reuses them and declares no second copy.

```go
package hubapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Author classes (05-design §3.2; Phase T's text·H / text·A): who wrote a text.
const (
	AuthorHuman = "H"
	AuthorAgent = "A"
)

// Issue content limits (04-internal-fit §2.4).
const (
	MaxIssueTitleChars       = 200      // code points, as Postgres char_length counts them
	MaxIssueDescriptionBytes = 16 << 10 // UTF-8 bytes, as Postgres octet_length counts them
	MaxCriterionIDChars      = 128      // contract §7.10's criterionId rule
)

var (
	ErrIssueTitle       = errors.New("hubapi: an issue title is 1 to 200 characters")
	ErrIssueDescription = errors.New("hubapi: an issue description is at most 16 KiB")
	ErrAuthor           = errors.New(`hubapi: an author is "H" or "A"`)
	ErrIssueList        = errors.New("hubapi: serves and implements are arrays, never null")
	ErrCriterionID      = errors.New("hubapi: a criterionId is 1 to 128 characters")
	ErrServesRepeat     = errors.New("hubapi: a serves pair appears twice")
)

// AuthoredText is a text with its author class. The class travels with the
// text everywhere it is shown or handed to an agent.
type AuthoredText struct {
	Text   string `json:"text"`
	Author string `json:"author"` // AuthorHuman | AuthorAgent
}

// Serve joins an issue revision to one brief criterion: the thread's link.
type Serve struct {
	BriefKey    string `json:"briefKey"`    // brief:bf-…@n
	CriterionID string `json:"criterionId"` // the criterion's id in that revision
}

// IssueRevisionContent is what one issue revision fixes. Serves and Implements
// are always arrays: no omitempty, never null.
type IssueRevisionContent struct {
	Title       AuthoredText `json:"title"`
	Description AuthoredText `json:"description"`
	Serves      []Serve      `json:"serves"`
	Implements  []string     `json:"implements"` // design:… keys
}

// ValidateIssueRevisionContent checks shape and limits, naming the field that
// failed. It makes no judgement of meaning and no character-hygiene check.
func ValidateIssueRevisionContent(c IssueRevisionContent) error {
	title := strings.TrimSpace(c.Title.Text)
	switch {
	case title == "" || utf8.RuneCountInString(c.Title.Text) > MaxIssueTitleChars:
		return &FieldError{"title.text", ErrIssueTitle}
	case !validAuthor(c.Title.Author):
		return &FieldError{"title.author", ErrAuthor}
	case len(c.Description.Text) > MaxIssueDescriptionBytes:
		return &FieldError{"description.text", ErrIssueDescription}
	case !validAuthor(c.Description.Author):
		return &FieldError{"description.author", ErrAuthor}
	case c.Serves == nil:
		return &FieldError{"serves", ErrIssueList}
	case c.Implements == nil:
		return &FieldError{"implements", ErrIssueList}
	}
	seen := make(map[Serve]bool, len(c.Serves))
	for i, s := range c.Serves {
		at := "serves[" + strconv.Itoa(i) + "]"
		if _, err := ParseBriefKey(s.BriefKey); err != nil {
			return &FieldError{at + ".briefKey", ErrInvalidKey}
		}
		if n := utf8.RuneCountInString(s.CriterionID); n < 1 || n > MaxCriterionIDChars {
			return &FieldError{at + ".criterionId", ErrCriterionID}
		}
		if seen[s] {
			return &FieldError{at, ErrServesRepeat}
		}
		seen[s] = true
	}
	for i, k := range c.Implements {
		if _, err := ParseDesignKey(k); err != nil {
			return &FieldError{"implements[" + strconv.Itoa(i) + "]", ErrInvalidKey}
		}
	}
	return nil
}

func validAuthor(a string) bool { return a == AuthorHuman || a == AuthorAgent }
```

```go
// kindIssueRevision is the issue revision's envelope kind (contract §3.6's
// table gains this row through the W0 proposal's later ask; never "issue").
const kindIssueRevision = "issue-revision"

// IssueRevisionEnvelope is the canonical record of one issue revision. Its
// bytes are what issue_revisions.canonical stores (task 06) and what the
// digest covers. The head (state, priority, assignee, labels) is never in it.
type IssueRevisionEnvelope struct {
	IssueID   string // is-…
	Revision  int    // ≥ 1
	ProjectID string // pj-…
	Content   IssueRevisionContent
}

func (e IssueRevisionEnvelope) check() error {
	switch {
	case !ValidID(PrefixIssue, e.IssueID):
		return bad("issueId")
	case e.Revision < 1:
		return bad("revision")
	case !ValidID(PrefixProject, e.ProjectID):
		return bad("projectId")
	}
	return nil
}

// Canonical encodes {"v":1,"kind":"issue-revision","issueId",…,"content"}
// with the Hub's one encoder. It validates content first, so no invalid
// revision ever has bytes, and fails ErrEnvelopeTooLarge past MaxEnvelopeBytes.
func (e IssueRevisionEnvelope) Canonical() ([]byte, error) {
	if err := e.check(); err != nil {
		return nil, err
	}
	if err := ValidateIssueRevisionContent(e.Content); err != nil {
		return nil, under("content", err)
	}
	out, err := Canonical(map[string]any{
		"v": 1, "kind": kindIssueRevision, "issueId": e.IssueID, "revision": e.Revision,
		"projectId": e.ProjectID, "content": e.Content,
	})
	if err != nil {
		return nil, err
	}
	if len(out) > MaxEnvelopeBytes {
		return nil, &FieldError{"content", ErrEnvelopeTooLarge}
	}
	return out, nil
}

// IssueRevisionDigest is DigestOf(e.Canonical()): the one function the Hub
// writes with (task 09) and hubclient re-checks with (task 16).
func IssueRevisionDigest(e IssueRevisionEnvelope) (Digest, []byte, error) {
	c, err := e.Canonical()
	if err != nil {
		return Digest{}, nil, err
	}
	return DigestOf(c), c, nil
}

var issueRevisionKeys = []string{"v", "kind", "issueId", "revision", "projectId", "content"}

// ParseIssueRevisionEnvelope reads stored canonical bytes back: how the Hub
// serves content (by parsing C, never from a second copy) and how hubclient
// re-checks a revision it received. It accepts exactly what Canonical writes:
// the six members, v 1, the issue-revision kind, content with exactly the four
// members, and bytes that are already canonical.
func ParseIssueRevisionEnvelope(canonical []byte) (IssueRevisionEnvelope, error) {
	m, err := readEnvelope(canonical, kindIssueRevision, issueRevisionKeys)
	if err != nil {
		return IssueRevisionEnvelope{}, err
	}
	var e IssueRevisionEnvelope
	id, ok1 := m["issueId"].(string)
	rev, ok2 := m["revision"].(int64)
	pj, ok3 := m["projectId"].(string)
	for i, ok := range []bool{ok1, ok2, ok3} {
		if !ok {
			return IssueRevisionEnvelope{}, bad(issueRevisionKeys[2+i])
		}
	}
	e.IssueID, e.Revision, e.ProjectID = id, int(rev), pj
	dec := json.NewDecoder(bytes.NewReader(encode(m["content"])))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e.Content); err != nil {
		return IssueRevisionEnvelope{}, bad("content")
	}
	again, err := e.Canonical()
	if err != nil {
		return IssueRevisionEnvelope{}, err
	}
	if !bytes.Equal(again, canonical) {
		return IssueRevisionEnvelope{}, bad("") // parsed, but not as Canonical writes it
	}
	return e, nil
}
```

`encode`, `readEnvelope`, `bad` and `under` are `hubapi`'s own unexported helpers
(`canonical.go:183`, `envelope.go:234`, `:54`, `:57`), so this file needs no new plumbing. A
content that decodes but was written with a missing list (`"serves":null`) fails inside
`Canonical` as `content.serves`, never silently re-encoded as `[]`.

#### The golden vectors, complete

Computed on 2026-09-25 by hub-contract §3.5's method: Python `json.dumps(sort_keys=True,
separators=(",",":"), ensure_ascii=False)` over the envelope, UTF-8 encoded, SHA-256; vector 1
again with `printf '%s' '<canonical>' | sha256sum`. **Both methods agree on vector 1**
(`a77f7648…4ab6`), and `sha256sum` over the Python-written bytes agrees on both. Each file holds
the envelope as a writer would build it, the canonical text as a JSON string (decode it to get the
exact bytes), their length, and the digest. The files are written with `ensure_ascii=True` and
`indent=2`, so they are pure ASCII and survive any editor; the **canonical bytes** are UTF-8
(`é` is `C3 A9`).

**v1**: ASCII, one `serves` pair, an empty `implements`. 402 bytes,
`sha256:a77f7648208b0456120f438ceecde358da929e501f0bfeee5cad8246fb604ab6`.
`testdata/issue-revision/v1.json`:

```json
{
  "envelope": {
    "v": 1,
    "kind": "issue-revision",
    "issueId": "is-00000000000000c3",
    "revision": 1,
    "projectId": "pj-00000000000000b2",
    "content": {
      "title": {
        "text": "Resume an upload after the connection drops",
        "author": "H"
      },
      "description": {
        "text": "Uploads over 40 MB restart from zero when the network drops.",
        "author": "H"
      },
      "serves": [
        {
          "briefKey": "brief:bf-00000000000000a1@5",
          "criterionId": "c-91e2a0b3c4d5e6f7"
        }
      ],
      "implements": []
    }
  },
  "canonical": "{\"content\":{\"description\":{\"author\":\"H\",\"text\":\"Uploads over 40 MB restart from zero when the network drops.\"},\"implements\":[],\"serves\":[{\"briefKey\":\"brief:bf-00000000000000a1@5\",\"criterionId\":\"c-91e2a0b3c4d5e6f7\"}],\"title\":{\"author\":\"H\",\"text\":\"Resume an upload after the connection drops\"}},\"issueId\":\"is-00000000000000c3\",\"kind\":\"issue-revision\",\"projectId\":\"pj-00000000000000b2\",\"revision\":1,\"v\":1}",
  "bytes": 402,
  "digest": "sha256:a77f7648208b0456120f438ceecde358da929e501f0bfeee5cad8246fb604ab6"
}
```

The canonical text of v1, as bytes on one line:

```
{"content":{"description":{"author":"H","text":"Uploads over 40 MB restart from zero when the network drops."},"implements":[],"serves":[{"briefKey":"brief:bf-00000000000000a1@5","criterionId":"c-91e2a0b3c4d5e6f7"}],"title":{"author":"H","text":"Resume an upload after the connection drops"}},"issueId":"is-00000000000000c3","kind":"issue-revision","projectId":"pj-00000000000000b2","revision":1,"v":1}
```

**v2**: a title with `é` (U+00E9), a description with a TAB and U+0001 by an `"A"` author, two
`serves` pairs and one `implements` key. 451 bytes,
`sha256:528b7d48da2a74a65653828fc78e9cad0422b9ba2f8854aa8e0f6ef4f5c4eb01`. In the canonical bytes
the TAB is the two characters `\t` and U+0001 is the six characters `\u0001` (lowercase hex,
hub-contract §3.4 rule 2); `é` is its two UTF-8 bytes, not an escape.
`testdata/issue-revision/v2.json`:

```json
{
  "envelope": {
    "v": 1,
    "kind": "issue-revision",
    "issueId": "is-00000000000000c3",
    "revision": 2,
    "projectId": "pj-00000000000000b2",
    "content": {
      "title": {
        "text": "Café upload resumes",
        "author": "H"
      },
      "description": {
        "text": "Step one\tthen\u0001end",
        "author": "A"
      },
      "serves": [
        {
          "briefKey": "brief:bf-00000000000000a1@5",
          "criterionId": "c-91e2a0b3c4d5e6f7"
        },
        {
          "briefKey": "brief:bf-00000000000000a1@5",
          "criterionId": "c-0a1b2c3d4e5f6071"
        }
      ],
      "implements": [
        "design:cv-7c01d2e3f4a5b6c7@11"
      ]
    }
  },
  "canonical": "{\"content\":{\"description\":{\"author\":\"A\",\"text\":\"Step one\\tthen\\u0001end\"},\"implements\":[\"design:cv-7c01d2e3f4a5b6c7@11\"],\"serves\":[{\"briefKey\":\"brief:bf-00000000000000a1@5\",\"criterionId\":\"c-91e2a0b3c4d5e6f7\"},{\"briefKey\":\"brief:bf-00000000000000a1@5\",\"criterionId\":\"c-0a1b2c3d4e5f6071\"}],\"title\":{\"author\":\"H\",\"text\":\"Café upload resumes\"}},\"issueId\":\"is-00000000000000c3\",\"kind\":\"issue-revision\",\"projectId\":\"pj-00000000000000b2\",\"revision\":2,\"v\":1}",
  "bytes": 451,
  "digest": "sha256:528b7d48da2a74a65653828fc78e9cad0422b9ba2f8854aa8e0f6ef4f5c4eb01"
}
```

The canonical text of v2, with the two escapes as they appear in the bytes:

```
{"content":{"description":{"author":"A","text":"Step one\tthen\u0001end"},"implements":["design:cv-7c01d2e3f4a5b6c7@11"],"serves":[{"briefKey":"brief:bf-00000000000000a1@5","criterionId":"c-91e2a0b3c4d5e6f7"},{"briefKey":"brief:bf-00000000000000a1@5","criterionId":"c-0a1b2c3d4e5f6071"}],"title":{"author":"H","text":"Café upload resumes"}},"issueId":"is-00000000000000c3","kind":"issue-revision","projectId":"pj-00000000000000b2","revision":2,"v":1}
```

To regenerate either file, the implementer runs the same method and **must** get these digests;
a different digest is a bug in the regeneration, never a reason to change the file.

---

#### Tests first, each with the mutant it kills

**`keys_test.go`, appended** (the file's imports already cover these):

```go
// ── issue keys (Phase W; contract §3.2 as proposed in W0) ───────────────────

// Mutant: ParseKey without the issue case (an issue key reads as ErrInvalidKey).
func TestIssueKeyRoundTrips(t *testing.T) {
	const in = "issue:is-3f2a91c07b1e4d2a@3"
	if kind, err := ParseKey(in); kind != KeyIssue || err != nil {
		t.Fatalf("ParseKey(%q) = (%q, %v), want (%q, nil)", in, kind, err, KeyIssue)
	}
	k, err := ParseIssueKey(in)
	if want := (IssueKey{IssueID: "is-3f2a91c07b1e4d2a", Revision: 3}); err != nil || k != want {
		t.Fatalf("ParseIssueKey(%q) = %+v, %v; want %+v", in, k, err, want)
	}
	if got := k.String(); got != in {
		t.Fatalf("IssueKey.String() = %q, want the input %q back", got, in)
	}
	const max = "issue:is-3f2a91c07b1e4d2a@999999999"
	if k, err := ParseIssueKey(max); err != nil || k.Revision != 999_999_999 || k.String() != max {
		t.Fatalf("ParseIssueKey(%q) = %+v, %v; want revision 999999999 and the input back", max, k, err)
	}
}

// Mutant: a loosened issueKeyRe ([0-9]+, [0-9a-fA-F], \w, or no anchors).
func TestIssueKeyRejectsEverythingOutsideTheGrammar(t *testing.T) {
	const id = "is-3f2a91c07b1e4d2a"
	for _, s := range []string{
		"issue:" + id + "@0",          // revision 0 never appears in a key
		"issue:" + id + "@01",         // a leading zero
		"issue:" + id + "@1000000000", // ten digits
		"issue:is-3f2a91c07b1e4d2@1",  // 15 hex
		"issue:is-3F2A91C07B1E4D2A@1", // upper-case hex
		"issue:bf-3f2a91c07b1e4d2a@1", // a brief id under the issue kind
		"issue:ic-3f2a91c07b1e4d2a@1", // a comment id
		" issue:" + id + "@1",         // a leading space
		"issue:" + id + "@1\n",        // a trailing newline
		"issue:" + id,                 // no @
		"Issue:" + id + "@1",          // the kind is lower case
	} {
		if k, err := ParseIssueKey(s); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseIssueKey(%q) = %+v, %v; want ErrInvalidKey", s, k, err)
		}
		if kind, err := ParseKey(s); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseKey(%q) = %q, %v; want ErrInvalidKey", s, kind, err)
		}
	}
}

// Mutant: an alias accepted as a key, or ParseKey falling back to "looks like
// PROJECT-number". An admin can change the key; an alias is never a key.
func TestDisplayAliasIsNeverAKey(t *testing.T) {
	for _, s := range []string{DisplayAlias("EP", 142), "EP-142", "issue:EP-142", "issue:EP-142@1"} {
		if kind, err := ParseKey(s); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseKey(%q) = %q, %v; want ErrInvalidKey", s, kind, err)
		}
	}
}
```

**`workids_test.go`, complete.**

```go
package hubapi

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The control plane's local prefixes (contract §3.1, :111) and Phase T task
// 01's twelve (2026-09-25-phase-t-team-mode-parts/task-01.md:176-183, read at
// feat/team-mode af569a8). Written out, not imported: Phase T's constants are
// not on this branch's sync base, and the test must hold before they are.
var (
	controlPlanePrefixes = []string{"s", "p", "b", "w", "m"}
	phaseTPrefixes       = []string{"dl", "th", "hm", "ms", "nt", "es", "tk", "dv", "ag", "ot", "ls", "cf"}
)

// Mutant: a Work prefix that repeats one already taken ("ms" for milestones,
// "tm" for templates, "iv" for initiatives), or a repeat inside the twelve.
func TestWorkPrefixesAreDisjoint(t *testing.T) {
	if len(WorkPrefixes) != 12 {
		t.Fatalf("WorkPrefixes has %d entries, want the twelve of 05-design §3.6", len(WorkPrefixes))
	}
	owner := map[string]string{}
	claim := func(p, who string) {
		if prev, ok := owner[p]; ok {
			t.Errorf("prefix %q is both %s and %s", p+"-", prev, who)
		}
		owner[p] = who
	}
	for _, c := range constValues(t, "keys.go", "Prefix") { // §3.1's ten, from the Hub's own file
		claim(c.value, "the Hub's "+c.name)
	}
	for _, p := range controlPlanePrefixes {
		claim(p, "the control plane's")
	}
	for _, p := range phaseTPrefixes {
		claim(p, "Phase T's")
	}
	for _, p := range WorkPrefixes {
		claim(p, "Phase W's")
	}
	if len(owner) != 10+5+12+12 {
		t.Errorf("%d distinct prefixes, want 39", len(owner))
	}
}

// Mutant: a prefix constant that ids.Wide cannot mint into a ValidID (a
// trailing "-" in the constant, or an upper-case letter).
func TestWorkPrefixesMintValidIDs(t *testing.T) {
	for _, p := range WorkPrefixes {
		if id := ids.Wide(p); !ValidID(p, id) {
			t.Errorf("ids.Wide(%q) = %q, which ValidID refuses", p, id)
		}
	}
}

// Mutant: a lower-case key, a 1-letter or 6-letter key, digits, or no anchors.
func TestValidProjectKey(t *testing.T) {
	for _, k := range []string{"EP", "PAY", "ABCDE"} {
		if !ValidProjectKey(k) {
			t.Errorf("ValidProjectKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"", "E", "EPICS1", "ABCDEF", "ep", "Ep", "E1", "EP-", " EP", "EP\n", "ÉP"} {
		if ValidProjectKey(k) {
			t.Errorf("ValidProjectKey(%q) = true, want false", k)
		}
	}
}

// Mutant: a zero-padded or separator-less alias ("EP142", "EP-0142").
func TestDisplayAliasFormat(t *testing.T) {
	if got := DisplayAlias("EP", 142); got != "EP-142" {
		t.Fatalf(`DisplayAlias("EP", 142) = %q, want "EP-142"`, got)
	}
	if got := DisplayAlias("PAY", 1); got != "PAY-1" {
		t.Fatalf(`DisplayAlias("PAY", 1) = %q, want "PAY-1"`, got)
	}
}
```

`constValues` (`keys_test.go:683`) returns `[]namedConst` with `name` and `value`; it parses
`keys.go` with `go/parser`, so this test reads the Hub's ten from the Hub's own file and follows
any change there.
