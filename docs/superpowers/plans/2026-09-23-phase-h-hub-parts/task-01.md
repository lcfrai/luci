### Task 01: `ids.Wide`, and the `hubapi` grammar — codes, keys, digests, branch refs, repo keys

**Lane A. Depends on: 44a by the plan's order; built before it (see *As built*), so this task wrote
§7.4's A1 itself and `TestTheContractsRepoKeyRegexIsTheCodes` reads that. About 2 hours. Contract
task H0.**

> **As built (2026-09-24, commits `caf18ae`, `25c503b`).** Task 44a had **not** landed when this
> task ran: §7.4 still held the old regexes. This task wrote A1 into §7.4 in place, marked
> `(Phase H plan, A1; task 01)`, with the lowercase rule and task 17's `repo_already_linked`
> message; 44a lists it in §16 and does not rewrite it. Three corrections to this part, each
> shown red first: `ValidBranchRef` refuses `.lock` at the end of **any** component (git's rule 1;
> the part checked only the whole ref); the Azure DevOps segments exclude ASCII control characters
> and DEL (RE2's `[^/]` matches `\n`, so `…/repo\n` was a second spelling); the boundary test is
> task 09's, not 08's. Two tests were added after the mutation run (the predicted survivors).
>
> **Fix round 1 (2026-09-24, review `task-01-review.md`).** I1: a new test holds the 38 codes to
> §4.2's table (spelling, order, Go name, status). m1: the prefix test compares (prefix, entity)
> pairs, not a set. m2: `ValidBranchRef` refuses a ref that is not UTF-8, and the test covers
> `\x1f`. m3: contract §3.3 now lists every rule the code enforces, in place, at the same line
> count, so no later line citation moves. m4, m5: stale sentences and comment hygiene.

Every later task names a Hub id, a key, a digest, a branch or a repository. This task is the one
place each of those is parsed and formatted, so no handler, no store and no client ever writes a
regular expression of its own. It also carries correction **A1** (the Azure DevOps repo key) and
**A9** (`hubapi` may import `httpx`).

**Files:**
- `control-plane/internal/ids/ids.go` — append `Wide` (modify; ⚑ shared with every phase that mints ids)
- `control-plane/internal/ids/ids_test.go` — append two tests (modify)
- `control-plane/internal/hub/hubapi/doc.go` — the package comment (new)
- `control-plane/internal/hub/hubapi/codes.go` (new)
- `control-plane/internal/hub/hubapi/keys.go` + `keys_test.go` (new)

#### Contract — complete

```go
// control-plane/internal/ids/ids.go — appended at the END of the file, after Token and UUID.

// Wide returns prefix + "-" + 16 random lowercase hex characters, e.g.
// "bf-3f2a91c07b1e4d2a". It is for ids shared across every user of the Hub,
// where New's 32 bits would collide: the Hub also retries once on a unique
// violation, so a collision costs a retry, never a wrong row.
func Wide(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	return prefix + "-" + hex.EncodeToString(b)
}
```

```go
// control-plane/internal/hub/hubapi/doc.go

// Package hubapi is the Hub's wire contract in Go: the error codes, the key and
// id grammar, the canonical encoding, and one struct per JSON object
// (docs/superpowers/contracts/2026-09-23-hub-contract.md §3, §4, §7).
//
// Both sides import it: the Hub server to write, and the control plane's
// hubclient to read and to re-check digests. It imports the standard library and
// internal/httpx only (plan correction A9); the boundary test (task 09,
// control-plane/internal/hub/boundary_test.go) holds it to that. There is no
// TypeScript twin, and there must never be one: the desktop never computes a
// digest (provenance contract P5).
package hubapi
```

```go
// control-plane/internal/hub/hubapi/codes.go
package hubapi

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"

// Shared codes: spelled, and meaning, exactly what httpx means (contract §4.2).
// They are httpx's constants, not copies, so the two can never drift.
const (
	CodeInvalidJSON     = httpx.CodeInvalidJSON     // 400
	CodeBadRequest      = httpx.CodeBadRequest      // 400
	CodeForbiddenOrigin = httpx.CodeForbiddenOrigin // 403
	CodeConflict        = httpx.CodeConflict        // 409
	CodeInternal        = httpx.CodeInternal        // 500
)

// The Hub's own codes, contract §4.2, in its table order. project_not_found is
// spelled like httpx.CodeProjectNotFound on purpose: the relay passes a Hub 404
// to the desktop unchanged, and "no such project that you can see" reads the
// same from either process.
const (
	CodeUnauthenticated       = "unauthenticated"         // 401, details.reason
	CodeTenantNotEnrolled     = "tenant_not_enrolled"     // 403, details.tenantId
	CodeForbidden             = "forbidden"               // 403, details.required
	CodeNotReserver           = "not_reserver"            // 403, details.releaseId
	CodeTeamNotFound          = "team_not_found"          // 404
	CodeProjectNotFound       = "project_not_found"       // 404
	CodeBriefNotFound         = "brief_not_found"         // 404
	CodeMemberNotFound        = "member_not_found"        // 404
	CodeInviteNotFound        = "invite_not_found"        // 404
	CodeApprovalNotFound      = "approval_not_found"      // 404
	CodeReleaseNotFound       = "release_not_found"       // 404
	CodeBlobNotFound          = "blob_not_found"          // 404
	CodeRouteNotFound         = "route_not_found"         // 404
	CodeMethodNotAllowed      = "method_not_allowed"      // 405
	CodeRevisionConflict      = "revision_conflict"       // 409
	CodeBriefExists           = "brief_exists"            // 409
	CodeBriefArchived         = "brief_archived"          // 409
	CodeDesignAlreadyApproved = "design_already_approved" // 409
	CodeRepoAlreadyLinked     = "repo_already_linked"     // 409
	CodeLastAdmin             = "last_admin"              // 409
	CodeReleaseState          = "release_state"           // 409
	CodeReleaseExpired        = "release_expired"         // 409
	CodeQuotaExceeded         = "quota_exceeded"          // 409
	CodeBlobTooLarge          = "blob_too_large"          // 413
	CodeBriefIncomplete       = "brief_incomplete"        // 422
	CodeKeyUnresolvable       = "key_unresolvable"        // 422
	CodeDigestMismatch        = "digest_mismatch"         // 422
	CodeBlobMissing           = "blob_missing"            // 422
	CodeDesignNotApproved     = "design_not_approved"     // 422
	CodeRepoNotLinked         = "repo_not_linked"         // 422
	CodeIdempotencyMismatch   = "idempotency_mismatch"    // 422
	CodeRateLimited           = "rate_limited"            // 429, details.scope
	CodeUnavailable           = "unavailable"             // 503
)

// Reasons carried in details.reason on a 401 unauthenticated (contract §4.2).
const (
	ReasonMissing     = "missing"
	ReasonMalformed   = "malformed"
	ReasonSignature   = "signature"
	ReasonExpired     = "expired"
	ReasonNotYetValid = "not_yet_valid"
	ReasonIssuer      = "issuer"
	ReasonAudience    = "audience"
	ReasonScope       = "scope"
	ReasonClient      = "client"
	ReasonVersion     = "version"
)
```

```go
// control-plane/internal/hub/hubapi/keys.go
package hubapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Id prefixes the Hub mints (contract §3.1). Disjoint from the control plane's
// s-, p-, b-, w-, m-.
const (
	PrefixOrg      = "org"
	PrefixUser     = "u"
	PrefixTeam     = "tm"
	PrefixInvite   = "iv"
	PrefixProject  = "pj"
	PrefixBrief    = "bf"
	PrefixApproval = "ap"
	PrefixLink     = "ln"
	PrefixRelease  = "rl"
	PrefixEvidence = "ev"
)

var wideHex = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ValidID reports whether id is prefix + "-" + 16 lowercase hex.
func ValidID(prefix, id string) bool {
	rest, ok := strings.CutPrefix(id, prefix+"-")
	return ok && wideHex.MatchString(rest)
}

var (
	sessionIDRe = regexp.MustCompile(`^[a-z]{1,4}-[0-9a-f]{8,32}$`) // contract §7.8
	canvasIDRe  = regexp.MustCompile(`^[a-z]{2,4}-[0-9a-f]{4,32}$`) // contract §3.2
)

func ValidSessionID(id string) bool { return sessionIDRe.MatchString(id) }
func ValidCanvasID(id string) bool  { return canvasIDRe.MatchString(id) }

// ── keys (contract §3.2) ────────────────────────────────────────────────────

type KeyKind string

const (
	KeyBrief  KeyKind = "brief"
	KeyDesign KeyKind = "design"
)

var ErrInvalidKey = errors.New("hubapi: not a key in the Hub's grammar")

var (
	briefKeyRe  = regexp.MustCompile(`^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$`)
	designKeyRe = regexp.MustCompile(`^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$`)
)

// BriefKey names one immutable brief revision: brief:<briefId>@<revision>.
type BriefKey struct {
	BriefID  string
	Revision int // 1 … 999,999,999
}

func (k BriefKey) String() string { return "brief:" + k.BriefID + "@" + strconv.Itoa(k.Revision) }

func ParseBriefKey(s string) (BriefKey, error) {
	m := briefKeyRe.FindStringSubmatch(s)
	if m == nil {
		return BriefKey{}, ErrInvalidKey
	}
	n, _ := strconv.Atoi(m[2]) // the grammar bounds it to nine digits
	return BriefKey{BriefID: m[1], Revision: n}, nil
}

// DesignKey names one approved canvas revision: design:<canvasId>@<designRevision>.
type DesignKey struct {
	CanvasID string
	Revision int
}

func (k DesignKey) String() string { return "design:" + k.CanvasID + "@" + strconv.Itoa(k.Revision) }

func ParseDesignKey(s string) (DesignKey, error) {
	m := designKeyRe.FindStringSubmatch(s)
	if m == nil {
		return DesignKey{}, ErrInvalidKey
	}
	n, _ := strconv.Atoi(m[2])
	return DesignKey{CanvasID: m[1], Revision: n}, nil
}

// ParseKey classifies a key without keeping it. ErrInvalidKey for anything else.
func ParseKey(s string) (KeyKind, error) {
	switch {
	case briefKeyRe.MatchString(s):
		return KeyBrief, nil
	case designKeyRe.MatchString(s):
		return KeyDesign, nil
	}
	return "", ErrInvalidKey
}

// ── digests ─────────────────────────────────────────────────────────────────

// Digest is a SHA-256. Its wire form is "sha256:" + 64 lowercase hex; Postgres
// stores the 32 raw bytes (contract §10.2).
type Digest [32]byte

var ErrInvalidDigest = errors.New(`hubapi: a digest is "sha256:" and 64 lowercase hex characters`)

func DigestOf(b []byte) Digest { return sha256.Sum256(b) }

func (d Digest) String() string { return "sha256:" + hex.EncodeToString(d[:]) }

func (d Digest) Hex() string { return hex.EncodeToString(d[:]) }

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func ParseDigest(s string) (Digest, error) {
	if !digestRe.MatchString(s) {
		return Digest{}, ErrInvalidDigest
	}
	var d Digest
	_, _ = hex.Decode(d[:], []byte(s[len("sha256:"):]))
	return d, nil
}

// ParseDigestHex accepts the bare 64-hex form a blob route's {sha} carries.
func ParseDigestHex(s string) (Digest, error) { return ParseDigest("sha256:" + s) }

func (d Digest) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d *Digest) UnmarshalText(b []byte) error {
	p, err := ParseDigest(string(b))
	if err != nil {
		return err
	}
	*d = p
	return nil
}

// ── branch refs (contract §3.3) ─────────────────────────────────────────────

const BranchRefMaxBytes = 255

// ValidBranchRef reports whether ref is "" (project-scoped) or a full
// refs/heads/<name> that git check-ref-format would accept, by the rules that
// need no git: at most 255 bytes; a non-empty name after "refs/heads/"; no "..",
// no "@{", no "//"; no ASCII control character or DEL; none of space ~ ^ : ? * [ \;
// not ending in "/" or "."; no component beginning with "." or ending in
// ".lock". Those are check-ref-format's rules 1 to 8 and 10; rule 9, a lone
// "@", cannot follow "refs/heads/". One rule is the Hub's, not git's: the ref
// is UTF-8. It sits in the canonical envelope, which is UTF-8 only (contract
// §3.4 rule 2), and encoding/json rewrites a bad byte as U+FFFD, so a ref that
// is not UTF-8 would be stored as a ref git does not have. Contract §3.3 lists
// every one of these rules.
func ValidBranchRef(ref string) bool {
	if ref == "" {
		return true
	}
	name, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok || name == "" || len(ref) > BranchRefMaxBytes || !utf8.ValidString(ref) {
		return false
	}
	if strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") {
		return false
	}
	if strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c < 0x20 || c == 0x7f || strings.IndexByte(" ~^:?*[\\", c) >= 0 {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// ── repository keys (contract §7.4, plan correction A1) ─────────────────────

type GitHost string

const (
	HostGitHub      GitHost = "github"
	HostAzureDevOps GitHost = "azure-devops"
)

// An Azure DevOps project or repository segment may hold spaces, so it is "not
// a slash" rather than a letter class; it also excludes the ASCII control
// characters and DEL, which no Azure DevOps name holds, so "…/repo\n" is not a
// second spelling of "…/repo" (RE2's [^/] alone matches a newline).
var (
	githubKeyRe = regexp.MustCompile(`^github\.com/[a-z0-9-]{1,39}/[a-z0-9._-]{1,100}$`)
	adoKeyRe    = regexp.MustCompile(`^azure-devops/[a-z0-9-]{1,50}/[^/\x00-\x1f\x7f]{1,64}/[^/\x00-\x1f\x7f]{1,64}$`)
)

// ValidRepoKey reports whether key is phase 3's githost.RepoKey() for that host,
// in its canonical, lowercased form: "github.com/<owner>/<repo>" or
// "azure-devops/<org>/<project>/<repo>" (phase 3's plan, task-13.md:140-143 on
// feat/phase-3-git; its review I10). A key that is not already lowercase is
// refused, never folded: the Hub stores one spelling of a repository or it
// links two projects to it. A key that is not UTF-8 fails the same check,
// because strings.ToLower rewrites an invalid byte as U+FFFD.
func ValidRepoKey(host GitHost, key string) bool {
	if key != strings.ToLower(key) {
		return false
	}
	switch host {
	case HostGitHub:
		return githubKeyRe.MatchString(key)
	case HostAzureDevOps:
		return adoKeyRe.MatchString(key)
	}
	return false
}

// HostOfRepoKey says which host a valid key belongs to.
func HostOfRepoKey(key string) (GitHost, bool) {
	for _, h := range []GitHost{HostGitHub, HostAzureDevOps} {
		if ValidRepoKey(h, key) {
			return h, true
		}
	}
	return "", false
}
```

#### Tests to write first

`ids_test.go` (appended) and `hubapi/keys_test.go`. Each is table-driven where it can be.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWideIsThePrefixAndSixteenLowercaseHex` | `Wide("bf")` matches `^bf-[0-9a-f]{16}$` for 1,000 calls | `Wide` reusing `New`'s 4 bytes; uppercase hex |
| `TestWideDoesNotCollideOnFastSuccessiveCalls` | 100,000 calls, no duplicate | a time-seeded generator |
| `TestValidIDTakesOnlyItsOwnPrefix` | `ValidID("bf","bf-00000000000000a1")` true; `ValidID("bf","pj-00000000000000a1")`, 15 or 17 hex, uppercase hex, `"bf-"+16hex+"\n"` all false | a regex without `^…$`; a prefix check by `Contains` |
| `TestBriefKeyRoundTrips` | `ParseBriefKey("brief:bf-3f2a91c07b1e4d2a@5")` → `{bf-3f2a91c07b1e4d2a 5}` and `.String()` gives the input back | a formatter that zero-pads or drops the prefix |
| `TestBriefKeyRejectsEverythingOutsideTheGrammar` | `@0`, `@01`, `@1000000000`, `bf-` + 15 hex, uppercase hex, `brief:b-42@1`, a leading space, a trailing `"\n"`, `design:…` → `ErrInvalidKey` | revision 0 in a key; leading zeros (two spellings of one key); an unanchored match |
| `TestDesignKeyGrammar` | `design:cv-7c01d2e3f4a5b6c7@11` round-trips; `cv-abc@1` (3 hex), 33 hex, `c-…` (one letter), `cvxyz-…` (five) → `ErrInvalidKey` | the canvas-id bounds widened or narrowed |
| `TestParseKeyClassifies` | brief → `KeyBrief`; design → `KeyDesign`; `b-42` → `("", ErrInvalidKey)` | a classifier that defaults to brief |
| `TestDigestWireForm` | `DigestOf([]byte("abc")).String()` = `sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad`; round-trips through `ParseDigest`; `MarshalText`/`UnmarshalText` via `encoding/json` give the string form; `SHA256:`, uppercase hex, 63 hex, a bare hex string → `ErrInvalidDigest` | a digest rendered as base64 or raw bytes in JSON; case-folding |
| `TestParseDigestHexTakesTheBareForm` | 64 hex → the same digest as the prefixed form; the prefixed form → error | a blob route that accepts `sha256:` in its path |
| `TestBranchRefRules` | valid: `""`, `refs/heads/main`, `refs/heads/feat/evidence-upload`, `refs/heads/café` (UTF-8), a 255-byte ref. Invalid: `main`, `refs/heads/`, `refs/tags/v1`, 256 bytes, `refs/heads/a..b`, `refs/heads/a@{b`, `refs/heads/a//b`, `refs/heads/a/`, `refs/heads/a.`, `refs/heads/a.lock`, `refs/heads/a.lock/b`, `refs/heads/.hidden`, `refs/heads/a/.b`, and one each with space, `~`, `^`, `:`, `?`, `*`, `[`, `\`, `\x01`, `\x1f`, `\x7f`; not UTF-8: `refs/heads/\xff`, `refs/heads/caf\xc3` *(the last four rows added in fix round 1, review m2)* | a local `b-…` BranchID reaching the Hub; each rule's deletion; `.lock` checked only at the end of the whole ref; the control bound as `< 0x1f`; a non-UTF-8 ref, which JSON would store as U+FFFD |
| `TestRepoKeyTakesPhase3sForms` | `ValidRepoKey(github, "github.com/lcfrai/panaceaapp")`, `ValidRepoKey(azure-devops, "azure-devops/kpmgcomau/panacea app/panaceaapp")` true; `HostOfRepoKey` names each | **the contract's §7.4 regex** (correction A1), which refuses the second |
| `TestRepoKeyRefusesTheOldAndTheNonCanonical` | `kpmgcomau.visualstudio.com/p/r` (the old form), `github.com/LCFRAI/x` (not lowercase), `azure-devops/a/b` (three segments), `github.com/a/b/c`, `azure-devops/a/b/c\n`, a tab and a DEL inside a segment, a non-UTF-8 byte, the wrong host for a valid key → false | a key folded to lowercase and stored under a second spelling; the Azure DevOps segment as bare `[^/]` |
| `TestTheContractsRepoKeyRegexIsTheCodes` | reads `docs/superpowers/contracts/2026-09-23-hub-contract.md` (found by walking up from the test's directory to the repo root), extracts the two repo-key patterns from §7.4, and asserts they equal `githubKeyRe` and `adoKeyRe`'s sources. Task 01 wrote A1 into §7.4 itself (commit `caf18ae`: 44a had not run), so the test passes against the amended text and goes red if either side drifts (moved here from task 44) | the drift correction A1 had to fix once |
| `TestHubCodesNeverShadowAControlPlaneCode` | every `hubapi` code except the five shared ones and `project_not_found` differs from each of `httpx`'s sixteen constants (`problem.go:45-60`), listed in the test by name | a Hub code named `backend_unavailable`, which the desktop reads as Docker (`desktop/src/api/problems.ts:39`) |
| `TestSessionAndCanvasIDsTakeTheContractsBounds` *(added after the mutation run)* | `ValidSessionID`: `s-3f2a91c0` and 4 letters + 32 hex true; 7 or 33 hex, 5 letters, uppercase, a trailing `"\n"` false. `ValidCanvasID`: `cv-abcd`, 4 letters + 32 hex true; one or five letters, 3 or 33 hex, uppercase, `"\n"` false. `designKeyRe` carries `canvasIDRe`'s pattern | either bound widened (predicted survivor P1: the part had no test for either function); the design key and the canvas id drifting apart |
| `TestPrefixesAndReasonsAreTheContracts` *(added after the mutation run)* | each `Prefix*` constant, read from `keys.go`'s source, is the prefix contract §3.1's table gives its entity (ten; the test maps `PrefixTeam` to "team" and so on, *pairs since fix round 1, review m1*: a set once); the `Reason*` constants, read from `codes.go`, are §4.2's `reason` list in its order | a respelled prefix or reason (predicted survivor P2: nothing read them); `PrefixTeam = "iv"`, `PrefixInvite = "tm"`, which keeps the set |
| `TestHubCodesAreContractSection42s` *(fix round 1, review I1)* | reads §4.2's table: the 33 string literals in `codes.go`, in declaration order, are its non-shared codes in table order; the five `httpx.*` selectors resolve (through the test's `httpx` list) to its **shared** rows in order; every constant is named `Code` + the code in camel case (`JSON` as the acronym); every constant's comment starts with its row's status, and any `details.x` it names is in its row's `details` | a respelled code (`revision_conflicts`); two codes' values swapped; two constants' **names** swapped, which keeps the order; a wrong status in a comment |

#### Steps

- [ ] **Step 1:** Re-read contract §3.1–§3.3, §4.2 and §7.4. If the contract has since been
  amended to phase 3's repo-key form, correction A1 is already in; keep the tests.
- [ ] **Step 2:** Write the tests above; `go test` fails to compile (no `Wide`, no `hubapi`).
- [ ] **Step 3:** Append `Wide`; write `doc.go`, `codes.go`, `keys.go`. All pass under `-race`.
- [ ] **Step 4:** `git diff --stat control-plane/go.mod` is empty (this task adds no dependency).
  Commit by pathspec, the five paths above.

#### Acceptance

```bash
go test ./control-plane/internal/ids/ ./control-plane/internal/hub/hubapi/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# 24   (the 7 existing ids tests, ids_test.go:15-82, plus the 17 above: 2 in ids, 15 in hubapi.
#       Was "20 ... plus the 13": the table had 14 rows once the contract test moved in from task 44;
#       23 until fix round 1 added TestHubCodesAreContractSection42s)
go vet ./control-plane/internal/ids/ ./control-plane/internal/hub/hubapi/        # no output
go list -deps ./control-plane/internal/hub/hubapi | grep panacea-app               # exactly two lines:
#   github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx
#   github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi
git diff --stat control-plane/go.mod control-plane/go.sum                          # empty
```

#### What this task does not claim

- That phase 3's `githost.RepoKey()` produces exactly these strings for every remote form; phase 3
  task 13's `TestRepoKeyIsMachineIndependent` owns that. This task only accepts what it emits.
- That the branch-ref rules are all of `git check-ref-format`'s; the rules needing git's config
  (`--allow-onelevel`, `--normalize`) are not reproduced.
- Anything about the canonical encoding (task 02) or the wire structs (task 03).
