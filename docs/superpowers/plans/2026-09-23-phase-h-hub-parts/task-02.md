### Task 02: `hubapi.Canonical`, the two envelopes, `ValidateContent`, and the golden vectors

**Lane A. Depends on: 01. About 2.5 hours. Contract task H0.**

A brief revision's digest is SHA-256 over a canonical byte string (contract §3.4), and the audit
chain hashes each event's canonical bytes (§3.6). Two processes compute these — the Hub writes,
`hubclient` re-checks (§9.2) — so there must be **one function**, and a byte-exact test that it
reproduces the contract's vectors. This task is that function, the two envelopes built with it, and
the content rules phase 4's writes are held to. It carries correction **A10** (`teamId` in the
audit envelope).

**Files:**
- `control-plane/internal/hub/hubapi/canonical.go` + `canonical_test.go` (new)
- `control-plane/internal/hub/hubapi/envelope.go` + `envelope_test.go` (new)

#### Contract — complete

```go
// control-plane/internal/hub/hubapi/canonical.go
package hubapi

import "errors"

// Canonical returns the canonical encoding of v (contract §3.4): RFC 8785 (JCS)
// restricted to what the Hub accepts.
//
//   - v is a json.RawMessage or []byte of JSON, parsed here; or any other Go
//     value, first marshalled with encoding/json and then parsed here. Either way
//     the bytes are read token by token, so a duplicate object key is SEEN
//     (encoding/json's own decoding silently keeps the last one).
//   - Objects: members sorted by key as sequences of UTF-16 code units; no
//     duplicate keys; no whitespace anywhere.
//   - Strings: UTF-8. `"` → \", `\` → \\, U+0008 \b, U+0009 \t, U+000A \n,
//     U+000C \f, U+000D \r, every other code point below U+0020 → \u00xx in
//     LOWERCASE hex. Everything else — "/", U+007F, U+2028, U+2029, "<", ">",
//     "&", non-ASCII — is written as its UTF-8 bytes. No Unicode normalisation.
//     Invalid UTF-8 in the input has already become U+FFFD by the time the
//     decoder yields the string; that replacement is what is encoded.
//   - Numbers: integers only, in [-(2^53-1), 2^53-1], decimal, no leading zero,
//     no plus sign, no exponent; "-0" is written "0". A fraction or an exponent
//     is ErrNotInteger, even when its value is integral ("1.0", "1e3").
//   - true, false, null as themselves.
//   - Nesting deeper than MaxCanonicalDepth is ErrTooDeep.
//
// The output is itself canonical: Canonical(Canonical(x)) == Canonical(x).
func Canonical(v any) ([]byte, error)

// MaxCanonicalDepth bounds recursion. The brief envelope is depth 1 and its
// content may be 16 deep (ValidateContent), so 32 is never reached by a valid
// write and exists only so a hostile body cannot exhaust the stack.
const MaxCanonicalDepth = 32

// MaxSafeInteger is 2^53 − 1, the largest integer every JSON reader agrees on.
const MaxSafeInteger = 1<<53 - 1

// Every failure wraps exactly one of these; the Hub maps each to 400
// bad_request with details.field naming where it was found.
var (
	ErrNotJSON      = errors.New("hubapi: not one JSON value")
	ErrDuplicateKey = errors.New("hubapi: an object repeats a key")
	ErrNotInteger   = errors.New("hubapi: a number with a fraction or an exponent")
	ErrIntegerRange = errors.New("hubapi: an integer outside ±(2^53−1)")
	ErrTooDeep      = errors.New("hubapi: nested too deeply")
)
```

**How it is built (described, not verbatim).** `json.NewDecoder(bytes.NewReader(b))` with
`UseNumber()`; a recursive `readValue(dec, depth)` over `dec.Token()` returning a small tree
(`object` as a slice of key/value pairs checked for duplicates with a `map[string]struct{}`,
`array`, `string`, `json.Number`, `bool`, `nil`); after the root, `dec.Token()` must return
`io.EOF` or it is `ErrNotJSON`. Objects are sorted with a comparator over
`utf16.Encode([]rune(key))`. Numbers are checked against `^-?(0|[1-9][0-9]*)$` and parsed with
`strconv.ParseInt`; the literal `-0` becomes `0`. Strings are written by a hand loop over runes
using the escape table above — **never `json.Marshal`**, which escapes `<`, `>`, `&`, U+2028 and
U+2029 and would change every digest.

```go
// control-plane/internal/hub/hubapi/envelope.go
package hubapi

import (
	"encoding/json"
	"errors"
	"time"
)

// Brief statuses (contract §7.5). The Hub reads exactly this one field of content.
const (
	StatusOpen     = "open"
	StatusComplete = "complete"
	StatusSkipped  = "skipped"
)

// Brief scopes.
const (
	ScopeProject = "project"
	ScopeBranch  = "branch"
)

// Content limits (contract §7.5).
const (
	MaxContentDepth   = 16
	MaxEnvelopeBytes  = 64 << 10 // the canonical envelope, content included
)

var (
	ErrContentNotObject = errors.New("hubapi: content must be a JSON object")
	ErrContentStatus    = errors.New(`hubapi: content.status must be "open", "complete" or "skipped"`)
	ErrContentDepth     = errors.New("hubapi: content nests more than 16 deep")
	ErrEnvelopeTooLarge = errors.New("hubapi: the canonical revision exceeds 64 KiB")
)

// ValidateContent checks a revision's content as a writer sent it and returns
// its canonical bytes and its status. It is the Hub's only reading of content.
// Depth counts the content object itself as 1.
func ValidateContent(raw json.RawMessage) (canonical []byte, status string, err error)

// BriefEnvelope is contract §3.4's envelope. Its canonical bytes are what
// brief_revisions.canonical stores and what the digest covers.
type BriefEnvelope struct {
	BriefID   string          // bf-…
	Revision  int             // ≥ 1
	ProjectID string          // pj-…
	Scope     string          // ScopeProject | ScopeBranch
	BranchRef string          // "" exactly when Scope is ScopeProject
	Content   json.RawMessage // already canonical (from ValidateContent)
}

// Canonical encodes {"v":1,"kind":"brief-revision","briefId",…,"content"}.
// It fails ErrEnvelopeTooLarge past MaxEnvelopeBytes.
func (e BriefEnvelope) Canonical() ([]byte, error)

// Digest is DigestOf(Canonical()).
func (e BriefEnvelope) Digest() (Digest, []byte, error)

// ParseBriefEnvelope reads stored canonical bytes back. It is how the Hub
// serves content — by parsing C, never from a second copy (contract §3.4) —
// and how hubclient re-checks a revision it received.
func ParseBriefEnvelope(canonical []byte) (BriefEnvelope, error)

// AuditEnvelope is contract §3.6's audit event, WITH teamId (plan correction
// A10). Data values are strings, integers or booleans only (§8).
type AuditEnvelope struct {
	OrgID       string         // org-…
	Seq         int64          // ≥ 1
	At          string         // RFC 3339 UTC, three fractional digits
	ActorUserID string         // u-…, or "" for the janitor
	Event       string         // the kind, e.g. "brief.revised"
	TeamID      string         // tm-…, or ""
	ProjectID   string         // pj-…, or ""
	Subject     string
	Data        map[string]any // string | int64 | bool values only
}

// Canonical encodes {"v":1,"kind":"audit-event","orgId",…,"data"}.
func (a AuditEnvelope) Canonical() ([]byte, error)

// ChainHash is SHA-256(prevHash ‖ canonical) — audit_events.hash (contract §3.6,
// §10.2's CHECK). prevHash is 32 zero bytes for seq 1.
func ChainHash(prevHash [32]byte, canonical []byte) [32]byte

// FormatTime writes t in the Hub's one time format: RFC 3339, UTC, exactly three
// fractional digits ("2026-09-23T04:02:11.123Z"), truncating, never rounding.
func FormatTime(t time.Time) string
```

#### The golden vectors — every byte

Vectors 1 and 2 are the contract's (§3.5). **Re-computed for this plan** on 2026-09-23 with Python's
`json.dumps(sort_keys=True, separators=(",",":"), ensure_ascii=False)`: both lengths and both
digests agree with the contract. Vectors 3 and 4 are new, computed the same way, with correction
A10's `teamId`, and chained.

```text
VECTOR 1 — BriefEnvelope{BriefID:"bf-00000000000000a1", Revision:1, ProjectID:"pj-00000000000000b2",
           Scope:"branch", BranchRef:"refs/heads/evidence-upload", Content:{"answers":[],"status":"open"}}
bytes (209):
{"branchRef":"refs/heads/evidence-upload","briefId":"bf-00000000000000a1","content":{"answers":[],"status":"open"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":1,"scope":"branch","v":1}
digest: sha256:a72a06a155457ac321227e660a220ab030f0367a5184f3f8ac8e8f636cd5e9fa

VECTOR 2 — Revision:2, Scope:"project", BranchRef:"", same ids; the content INPUT (Go string literal):
           `{"status":"complete","note":"Café\tline\u0001end","n":-0,"big":9007199254740991,"ok":true,"none":null}`
bytes (256; "é" is C3 A9, and \t and \u0001 appear as the six and two escape characters shown):
{"branchRef":"","briefId":"bf-00000000000000a1","content":{"big":9007199254740991,"n":0,"none":null,"note":"Café\tline\u0001end","ok":true,"status":"complete"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":2,"scope":"project","v":1}
digest: sha256:fa6e8ac2a66bf14e2b0ad2ffb02afac6b6143d042e52f5bdc5a051a0c43d8537

VECTOR 3 — AuditEnvelope{OrgID:"org-00000000000000c3", Seq:1, At:"2026-09-23T04:02:11.123Z",
           ActorUserID:"u-00000000000000d4", Event:"team.created", TeamID:"tm-00000000000000e5",
           ProjectID:"", Subject:"tm-00000000000000e5", Data:{"name":"Engagement Portal"}}
bytes (271):
{"actorUserId":"u-00000000000000d4","at":"2026-09-23T04:02:11.123Z","data":{"name":"Engagement Portal"},"event":"team.created","kind":"audit-event","orgId":"org-00000000000000c3","projectId":"","seq":1,"subject":"tm-00000000000000e5","teamId":"tm-00000000000000e5","v":1}
ChainHash(32 zero bytes, bytes): c6a78d78c4d773a8cad34957f8c0f42cbc1e72e552c3dbf9eceee1f96455fbaa

VECTOR 4 — Seq:2, At:"2026-09-23T04:02:12.000Z", ActorUserID:"", Event:"release.expired",
           TeamID:"tm-00000000000000e5", ProjectID:"pj-00000000000000b2", Subject:"rl-00000000000000f6", Data:{}
bytes (249):
{"actorUserId":"","at":"2026-09-23T04:02:12.000Z","data":{},"event":"release.expired","kind":"audit-event","orgId":"org-00000000000000c3","projectId":"pj-00000000000000b2","seq":2,"subject":"rl-00000000000000f6","teamId":"tm-00000000000000e5","v":1}
ChainHash(vector 3's hash, bytes): bc7822374dfa797a915c66eb874ecdab5d763c77155b6c15a82f26faedd2010c
```

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestVector1` / `TestVector2` | the envelope's `Canonical()` equals the bytes above exactly (length and content) and `Digest()` equals the digest | any deviation from §3.4 — ordering, escaping, `-0`, spacing |
| `TestVector3` / `TestVector4` | the audit envelopes' bytes and `ChainHash` values above, vector 4 chained on vector 3 | an envelope without `teamId` (A10); a hash over `canonical` alone, not `prev ‖ canonical` |
| `TestKeysSortByUTF16CodeUnits` | `{"<U+E000>":1,"\U0001F600":2}` (the first key is the one character U+E000, spelled out here because it is invisible) → the 😀 key first | a bytewise (UTF-8) sort, which puts U+E000 first |
| `TestStringEscapesAreExactlyTheTable` | `"/"`, `"<>&"`, U+007F, U+2028, U+2029 come out raw; U+001F → `\u001f` (lowercase); `\b\t\n\f\r` short forms | `json.Marshal`'s HTML escaping; uppercase `\u001F` |
| `TestNumbersIntegersOnly` | `1.0`, `1e3`, `-1.5e-2` → `ErrNotInteger`; `9007199254740992` and `-9007199254740992` → `ErrIntegerRange`; `9007199254740991` and `-9007199254740991` pass; `-0` → `0` | float formatting leaking into digests; the range bound off by one |
| `TestDuplicateKeysAreSeen` | `{"a":1,"a":2}` at the root and nested three deep → `ErrDuplicateKey` | decoding through `map[string]any`, which keeps the last |
| `TestOneValueOnly` | `{"a":1}{"b":2}`, `{"a":1} x`, empty input → `ErrNotJSON` | a trailing second value silently dropped |
| `TestDepthGuard` | 33 nested arrays → `ErrTooDeep`; 32 pass | unbounded recursion on a hostile body |
| `TestCanonicalIsIdempotent` | for vectors 1–4 and 50 generated values, `Canonical(Canonical(x)) == Canonical(x)` | an encoder whose output it cannot itself read the same way |
| `TestGoValuesGoThroughTheSameRules` | `Canonical(map[string]any{"b":1,"a":"x"})` = `{"a":"x","b":1}`; a `float64(1.5)` inside → `ErrNotInteger` | a second code path for Go values with its own rules |
| `TestValidateContentRules` | not an object (`[]`, `"x"`, `null`) → `ErrContentNotObject`; missing `status`, `"status":"done"`, `"status":1` → `ErrContentStatus`; depth 17 → `ErrContentDepth`, 16 passes; returns the canonical bytes and the status | the Hub reading or requiring any content field but `status` (§7.5) |
| `TestEnvelopeSizeLimit` | a content that makes the envelope 65,537 bytes → `ErrEnvelopeTooLarge`; 65,536 passes | the limit applied to the content alone |
| `TestParseBriefEnvelopeRoundTrips` | `ParseBriefEnvelope(vector 1 bytes)` gives vector 1's struct back, `Content` byte-equal | a reader that re-serialises content and changes its bytes |
| `TestFormatTimeTruncates` | `…04:02:11.1239Z` → `…04:02:11.123Z`; `+10:00` input → UTC; a whole second → `.000Z` | `time.RFC3339Nano`, which drops trailing zeros and varies in length |

#### Steps

- [ ] **Step 1:** Re-read contract §3.4–§3.6. Recompute vectors 1–4 yourself with the Python
  one-liner above (any machine with Python) before trusting this file; they must match.
- [ ] **Step 2:** Write the tests; they fail to compile.
- [ ] **Step 3:** `canonical.go`, then `envelope.go`. All pass under `-race`.
- [ ] **Step 4:** Commit by pathspec, the four files.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubapi/ -run 'Vector|UTF16|Escapes|Numbers|Duplicate|OneValue|Depth|Idempotent|GoValues|ValidateContent|EnvelopeSize|ParseBrief|FormatTime' -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# 16
go test ./control-plane/internal/hub/hubapi/ -race -count=1        # ok
grep -n "json.Marshal" control-plane/internal/hub/hubapi/canonical.go   # at most one hit: the marshal of a non-JSON Go value, before parsing
```

#### What this task does not claim

- **That this is full RFC 8785.** It is JCS restricted to integers; the float rules never arise
  because floats are refused.
- That phase 4's content stays inside the rules; the Hub refuses what breaks them with 400
  `bad_request`, and phase 4's plan owns writing times inside content in the Hub's format.
- That the contract's text carries the envelope's `teamId`: task 44a folds A10 in before task
  01 runs; this task does not edit the contract, and if 44a has not merged, the two disagree by
  design (A10) until it does.

#### As built (task 02, commit af72f19)

The contract above stands; every name in it is built as written. Three additions, each declared
in `task-02-report.md`:

- **D1 `FieldError`.** "details.field naming where it was found" needs the field in the error.
  Every failure of `Canonical`, `ValidateContent`, the envelopes and the parsers is a
  `*FieldError{Field, Err}`: `Err` is exactly one of the `Err*` values (so `errors.Is` works),
  `Field` a path from the value's root (`answers[2].text`; `""` is the root). `ValidateContent`'s
  fields are relative to the content (`status`); the Hub puts `revisions[<i>].content` in front.
  The envelopes report content errors under `content` and `ErrEnvelopeTooLarge` at `content`.
- **D2 `ErrBadEnvelope`.** The part names no error for a malformed envelope, and
  `ParseBriefEnvelope` must refuse something. `BriefEnvelope.Canonical` checks `briefId` (bf-),
  `revision ≥ 1`, `projectId` (pj-), the scope, and that `branchRef` is `""` exactly for a
  project brief and a valid ref for a branch brief (`ValidBranchRef("")` is true, so the branch
  case adds `!= ""`); it runs `ValidateContent` itself, so non-canonical content gives the same
  bytes. `AuditEnvelope.Canonical` checks `orgId`, `seq ≥ 1`, `at` in `FormatTime`'s exact form,
  the optional `u-`/`tm-`/`pj-` ids, the event kind's spelling, and that each data value is a
  `string`, `bool`, `int` or `int64`; a nil `Data` is `{}`. Both parsers accept exactly what
  `Canonical` writes: the members, `v` 1, the kind, and bytes that are already canonical
  (`ParseX(b)` then `Canonical()` must give `b` back). `Content` from `ParseBriefEnvelope` is a
  fresh copy, never a slice of the caller's buffer.
- **D3 `ParseAuditEnvelope(canonical []byte) (AuditEnvelope, error)`.** Task 27's `hub verify`
  checks that "`canonical` parses as a `hubapi.AuditEnvelope`"; no task wrote that reader. Data
  integers come back as `int64`.

Five tests beyond the table cover them: `TestFailuresNameTheirField`,
`TestAStoredRevisionMustBeCanonical`, `TestEnvelopesRefuseMalformedFields`,
`TestAuditDataTakesScalarsOnly`, `TestParseAuditEnvelopeRoundTrips`. None matches the acceptance
`-run` pattern, so its count stays 16; the package holds 21 new top-level tests.

The `AuditEnvelope` block above is not gofmt-clean as printed (its field comments are realigned).
