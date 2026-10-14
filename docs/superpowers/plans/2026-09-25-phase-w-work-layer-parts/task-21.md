### Task 21: Phase 3 delta: the `Panacea-Issue` trailer; the lane index names its issue; AU-only branch names

Stream **WP**, seam **SW.9** (defined here), about 3 hours (an estimate, not a measurement). Depends on
task 19 (`Session.IssueRef`, SW.7) and on phase 3 tasks **P3-05**, **P3-07** and **P3-08**
sync-merged into `feat/work-layer`. If their files are not on the sync base, this task **waits**; it
never writes a second copy of a phase 3 file (rule H2). The owner phase wins every conflict: this
task's lines are re-applied on top of the phase 3 file and its tests re-run against it.

This is the task that puts the issue into git. Everything after it that says "this commit was for
EP-142" reads a trailer this task writes: the land (task 22), the release manifest (task 71), the
host webhooks (task 154). So the grammar is fixed here, once, and it is deliberately the narrowest
grammar that works.

**The trailer carries the bare id, never a key.** `Panacea-Issue: is-0123456789abcdef`. Not
`issue:is-…@3`, not `EP-142`. Three reasons, each of which kills a real bug:

1. `EP-142` is a display alias built from `project.key` and a number (05 §3.6, the skeleton's rule
   1). An admin can change the project key; a trailer naming `EP-142` would then name nothing, and
   git history cannot be rewritten to follow it.
2. A revision (`@r`) in the trailer would put a mutable fact ("which revision the agent was told")
   into an immutable place, and it would be wrong the moment the session read a later revision. The
   revision is recovered the other way: `Panacea-Session` (phase 3's trailer) → the session record
   → `Session.IssueRef` (task 19). 04 §5.2 wrote the trailer as `issue:is-…@r`; this plan follows the
   skeleton's SW.9 instead, and the return of this part lists the difference.
3. A bare id has a grammar a single anchored expression can check: `^is-[0-9a-f]{16}$`. Nothing
   parses it as a key (contract P5: `hubapi.ParseKey` is the one parser, and it is never called on a
   trailer value). A value that fails the check is dropped **before git sees it**, so a forged
   `--octopus` or `-h` in a trailer can never become an argument.

**The lane index names its issue.** A worktree made from an issue (Start, task 24) records the issue
id in phase 3's lane index, so land can add the issue even when an agent's commits somehow lack the
trailer. The skeleton files this under `converge`; the index is `lanes` (`Lane` is P3 task-03's
type), so this task edits `lanes/store.go` and says so.

**AU-only projects get no title slug in branch names** (`OD-24`, Q8). A branch name travels to a
customer's remote (Azure DevOps or GitHub, `OD-4`), and an org admin's erase (the Hub's erase rule)
cannot reach it there. An issue title is text; a branch name that holds it is a copy the erase will
miss. So for an *AU data only* project the name is `ep-142` and nothing else. Where a project's data
class lives is **UNVERIFIED until merge** (Phase T `task-01.md:484-486` at `04e6944`; the scope cited `:454-456`, which has since moved says the same), so `auOnly`
is a **parameter** here and task 24 supplies it.

**What this task does not touch:** the Hub. No move is sent (task 22), no route is added, and no
file outside phase 3's packages and task 19's `sessions.go` delta changes.

**One correction to the scope, made here so nobody re-derives it.** The scope says the hook stamps
`$PANACEA_APP_ISSUE_ID` "when the variable is set". That variable cannot come from `Spec.Env`:
`backend.go:62` reserves the whole `PANACEA_APP_` prefix, `ValidateSpec` rejects it loudly and
`EnvEntries` drops it silently (`backend.go:218-230`, read in this worktree at `c5841bf`), so a client could never choose it. P3-05
solves the same problem for the session id with a `Spec` field exported as `EnvSessionID`
(P3 `task-05.md:40-51`). This task follows that pattern exactly: `Spec.IssueID`, exported as
`EnvIssueID`, written only by the session-create handler from the Materialiser's `IssueRef`. So
`backend.go` is a file this task touches, and the skeleton's file map does not list it (a gap,
named in the return).

**Files** (every phase 3 path is **UNVERIFIED until merge**; the phase 3 part that plans it is cited):
- `control-plane/internal/gitx/trailers.go` ⚑ (P3 `task-02.md:305-342`) — `TrailerIssue`,
  `ValidIssueTrailer`, `IssueTrailerValues` (modified)
- `control-plane/internal/gitx/trailers_test.go` ⚑ — the validator's and the reader's rows (modified)
- `control-plane/internal/backend/backend.go` ⚑ (built; P3-05 amends it) — `EnvIssueID`,
  `Spec.IssueID`, one entry in `EnvEntries` (modified)
- `control-plane/internal/backend/backend_test.go` ⚑ — the order and the reserved-key rows (modified)
- `images/sandbox/githooks/dispatch` ⚑ (P3 `task-05.md:140-190`) — the second stamp (modified)
- `control-plane/internal/api/hooks_docker_test.go` ⚑ (P3-05, `//go:build docker`) — two rows (modified)
- `control-plane/internal/api/sessions.go` ⚑ (built; task 19's delta) — `Spec.IssueID` from
  `IssueRef`, at both `backend.Spec{` sites (`sessions.go:951`, `:1637`) (modified)
- `control-plane/internal/lanes/store.go` ⚑ (P3 `task-03.md:24`, `:61-69`) — `Lane.IssueID` (modified)
- `control-plane/internal/lanes/clone.go` ⚑ (P3 `task-03.md:25`, `:132-136`) — `CreateRequest.IssueID`,
  copied into the `Lane` that `Create` adds (modified)
- `control-plane/internal/lanes/issue_test.go` — the round trip and the copy; a new file, because
  P3 names its tests only as `lanes/*_test.go` (`task-03.md:27`) (new)
- `control-plane/internal/lanes/issuebranch.go` (+ `issuebranch_test.go`) — `IssueBranchName` (new)
- `control-plane/internal/converge/land.go` ⚑ (P3 `task-09.md:36-132`) — step 9a, the trailers in
  step 10, `LandResult.Issues` (modified)
- `control-plane/internal/converge/land_test.go` ⚑ (P3 `task-09.md:16`) — one row (modified)

Phase 3 anchors were re-read in the `phase-3` worktree at `9935278` (the scope read `7c71b7d`; the
cited ranges were re-checked at the newer commit).

#### Interfaces — complete

```go
// gitx/trailers.go — appended to phase 3's const block and file. Nothing else changes.
const (
	// TrailerIssue names the Hub issue a commit was made for. Its value is the bare issue id,
	// "is-" + 16 lowercase hex: never an "issue:" key, never a display alias such as EP-142,
	// and never parsed as a key (contract P5). The hook stamps it on every commit of a bound
	// session (task 21); land adds one per distinct issue in the landed range (task 21).
	TrailerIssue = "Panacea-Issue"
)

var issueTrailerRe = regexp.MustCompile(`^is-[0-9a-f]{16}$`)

// ValidIssueTrailer reports whether v may be written or read as a Panacea-Issue value. It is
// the whole grammar of SW.9. A value that fails it is dropped before git sees it, on write,
// and ignored on read.
func ValidIssueTrailer(v string) bool { return issueTrailerRe.MatchString(v) }

// IssueTrailerValues reads the stdout of
//   git log --format=<RecordSep-terminated TrailerFormat(TrailerIssue)> <range>
// and returns every valid value once, sorted. Invalid values are dropped, never repaired.
// The result is [] (never nil) when there are none, so a JSON field built from it is [].
func IssueTrailerValues(stdout []byte) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, rec := range strings.Split(string(stdout), RecordSep) {
		for _, v := range strings.Split(strings.TrimSpace(rec), ValueSep) {
			v = strings.TrimSpace(v)
			if ValidIssueTrailer(v) && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	sort.Strings(out)
	return out
}
```

`regexp`, `sort` and `strings` join `trailers.go`'s imports if P3-02's file lacks them. The
validator is a separate expression from `hubapi.ValidID(hubapi.PrefixIssue, v)` on purpose:
`gitx` must not import `hubapi` (it is a leaf package every phase 3 package imports), and the two
must agree, which `TestValidIssueTrailerAgreesWithHubapi` in `internal/api` (which imports both
after task 16) pins.

```go
// backend/backend.go — one constant beside P3-05's EnvSessionID, one Spec field, one entry.
const (
	// EnvIssueID is the bare Hub issue id this session was started for (Start, task 24). The
	// image's git hook stamps it as "Panacea-Issue: <id>" beside Panacea-Session. Written from
	// Spec.IssueID, never from Spec.Env (the PANACEA_APP_ prefix is reserved, backend.go:62),
	// so neither a client nor an agent chooses it.
	EnvIssueID = "PANACEA_APP_ISSUE_ID"
)

// In Spec, after SessionID:
//	IssueID string // "" or a bare "is-" id; exported as EnvIssueID; a backend never reads it back

// EnvEntries: the three contract variables, then EnvSessionID when Spec.SessionID is not
// empty, then EnvIssueID when Spec.IssueID is not empty, then Spec.Env sorted with reserved
// keys dropped. The capacity hint grows from 3 to 5. sandbox.containerEnv produces the same
// sequence (P3-05's TestThePodEnvironmentIsExactlySpecEnvEntries pins it, unchanged).
```

```go
// api/sessions.go — task 19 stamps Session.IssueRef from the Materialiser's result. At both
// backend.Spec{ sites (create, sessions.go:951; restart, :1637) this task adds one field:
//	IssueID: issueIDOf(sess.IssueRef),

// issueIDOf returns the bare id of an issue: key, or "" for "" or anything ParseIssueKey
// refuses. It is the only place the control plane turns a key into a trailer value, and it
// goes through the one parser (task 02); it never slices the string.
func issueIDOf(key string) string {
	if key == "" {
		return ""
	}
	k, err := hubapi.ParseIssueKey(key)
	if err != nil || !gitx.ValidIssueTrailer(k.IssueID) {
		return ""
	}
	return k.IssueID
}
```

```sh
# images/sandbox/githooks/dispatch — appended inside P3-05's prepare-commit-msg block, after
# the Panacea-Session stamp and before its closing `fi`. The case is the SW.9 grammar in sh:
# "is-" then exactly 16 of [0-9a-f]. --if-exists doNothing keeps an amend's first value, as the
# session stamp does; an unset or empty variable writes no line at all.
	case "${PANACEA_APP_ISSUE_ID:-}" in
		is-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f])
			git interpret-trailers --in-place --if-exists doNothing \
				--trailer "Panacea-Issue: $PANACEA_APP_ISSUE_ID" "$1" ;;
		*) ;; # unset, empty or not an id the control plane minted: stamp nothing
	esac
```

The pattern has no `*` after the sixteenth class, so `is-<16 hex>x` falls to `*)`. POSIX `case`
patterns are anchored at both ends, which is what makes this the whole grammar.

```go
// lanes/store.go — one field appended to Lane. omitempty keeps an index written before this
// task byte-identical on the next write, so schemaVersion stays 1 and no migration exists.
//	IssueID string `json:"issueId,omitempty"` // the bare Hub issue id a Start made this lane for

// lanes/clone.go — one field appended to CreateRequest, copied into the Lane Create adds.
//	IssueID string // "" or a bare "is-" id; Create refuses anything else with ErrBadIssue

// ErrBadIssue joins clone.go's error block. It is checked in Create's step 1, before any git
// call, so a refused id leaves nothing on disk.
var ErrBadIssue = errors.New("lanes: an issue id is \"is-\" and 16 lowercase hex")
```

```go
// lanes/issuebranch.go
package lanes

import "strings"

// issueNameMax is phase 3's worktree-name limit (ErrBadName: 1-40 characters).
const issueNameMax = 40

// IssueBranchName is the worktree name a Start gives the lane it makes for an issue. It is
// the display alias in lower case ("ep-142"), then, unless the project is AU data only, "-"
// and a slug of the title: a-z, 0-9 and "-", no leading, trailing or doubled "-", and the
// whole name at most 40 characters, cut at a "-" when one falls in the last 12. Letters
// outside a-z (é, ß, 中) become "-" and then collapse; they are never transliterated, because
// a transliteration table is a copy of someone's data. An empty slug gives the alias alone.
//
// auOnly drops the slug entirely (OD-24, Q8): a branch name reaches a customer's remote, and
// the Hub's erase cannot follow it there. The alias is a display value (DisplayAlias, task
// 02), so it is lower-cased here and never parsed back.
func IssueBranchName(alias, title string, auOnly bool) string {
	base := slugOf(alias)
	if len(base) > issueNameMax {
		base = strings.Trim(base[:issueNameMax], "-")
	}
	if auOnly || base == "" {
		return base
	}
	room := issueNameMax - len(base) - 1
	s := slugOf(title)
	if room <= 0 || s == "" {
		return base
	}
	if len(s) > room {
		s = s[:room]
		if i := strings.LastIndexByte(s, '-'); i >= room-12 && i > 0 {
			s = s[:i]
		}
		s = strings.Trim(s, "-")
	}
	if s == "" {
		return base
	}
	return base + "-" + s
}

// slugOf maps s to [a-z0-9-] with no leading, trailing or doubled "-". It is byte-safe: the
// output is ASCII, so len() counts characters and a cut can never split a rune.
func slugOf(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
```

`IssueBranchName` never returns a name `ErrBadName` refuses when `alias` is a `DisplayAlias`
(2 to 5 capitals, a hyphen, a number: `ValidProjectKey`, task 02): the first character is a
letter, every character is in `a-z 0-9 -`, and the length is at most 40. An empty alias returns
`""`, which `CreateRequest.Name`'s rule turns into a derived name; task 24 never passes one.

```go
// converge/land.go — LandResult gains one field, after BroughtBack. Made with []string{}
// before step 9a runs, so it is [] in the body when the land names no issue (P3's
// "never null" rule, task-09.md:66-71).
//	Issues []string `json:"issues"` // the Panacea-Issue values step 10 wrote; task 22 moves them

// Land's comment gains step 9a, and step 10's trailer list gains the issues:
//  9a. issues = landIssues(ctx, req.ProjectPath, old, laneTip, lane.IssueID)   (git_failed)
//      Read, Scope Project, in the PROJECT: the lane is read only through LaneRef, never in
//      its clone (R4.2). Not a refusal: a lane with no issue lands exactly as before.
// 10. msg = gitx.Message(…, [Panacea-Land] + [Panacea-Brief, Panacea-Brief-Digest] when a key
//                        + one Panacea-Issue per entry of issues, in their sorted order)

// landIssues is every distinct valid Panacea-Issue value on the commits the merge brings in
// (old..laneTip, which is merge^1..laneTip once step 11 has run), plus the lane's own IssueID
// when valid, sorted. An invalid value never reaches gitx.Message or git's argv.
func (l *Lander) landIssues(ctx context.Context, projectPath, old, laneTip, laneIssue string) ([]string, error) {
	res, err := l.Machine.Git().Run(ctx, gitx.Call{
		Dir: projectPath,
		Args: []string{"log",
			"--format=" + gitx.TrailerFormat(gitx.TrailerIssue) + "%x1e",
			old + ".." + laneTip, "--"},
		Class: gitx.Read,
		Scope: gitx.Project,
	})
	if err != nil {
		return nil, err
	}
	out := gitx.IssueTrailerValues(res.Stdout)
	if gitx.ValidIssueTrailer(laneIssue) && !slices.Contains(out, laneIssue) {
		out = append(out, laneIssue)
		slices.Sort(out)
	}
	return out, nil
}
```

`old` and `laneTip` are 40-hex shas from `rev-parse` (steps 6 and 7), so the range argument can
never start with `-`; the trailing `--` keeps git from reading it as a path either way. Merges
inside the lane **are** walked (no `--no-merges`, no `--first-parent`): a lane that merged a
sibling's branch carries that sibling's commits, and a land names every issue it lands. `slices`
joins `land.go`'s imports (Go 1.21+, inside the `go 1.23` directive).

- [ ] **Step 1: Write the failing `gitx` tests**

Append to `control-plane/internal/gitx/trailers_test.go`:

```go
func TestValidIssueTrailerRejectsKeysAndFlags(t *testing.T) {
	good := "is-3f2a91c07b1e4d2a"
	if !gitx.ValidIssueTrailer(good) {
		t.Fatalf("ValidIssueTrailer(%q) = false, want true", good)
	}
	for _, v := range []string{
		"", "is-", "issue:is-3f2a91c07b1e4d2a@3", "EP-142", "-h", "--octopus",
		"is-3F2A91C07B1E4D2A", "is-3f2a91c07b1e4d2", "is-3f2a91c07b1e4d2aa",
		"is-3f2a91c07b1e4d2a\nPanacea-Land: w-1", " is-3f2a91c07b1e4d2a", "bf-3f2a91c07b1e4d2a",
	} {
		if gitx.ValidIssueTrailer(v) {
			t.Errorf("ValidIssueTrailer(%q) = true, want false", v)
		}
	}
}

func TestIssueTrailerValuesSortsDedupesAndDropsForgeries(t *testing.T) {
	a, b := "is-aaaaaaaaaaaaaaaa", "is-0000000000000001"
	rs, vs := gitx.RecordSep, gitx.ValueSep
	stdout := []byte(a + rs + "\n" + b + vs + a + rs + "\n" + "--octopus" + vs + "EP-9" + rs + "\n" + rs)
	got := gitx.IssueTrailerValues(stdout)
	if want := []string{b, a}; !slices.Equal(got, want) {
		t.Fatalf("IssueTrailerValues = %q, want %q", got, want)
	}
	if got := gitx.IssueTrailerValues(nil); got == nil || len(got) != 0 {
		t.Fatalf("IssueTrailerValues(nil) = %#v, want an empty non-nil slice", got)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestValidIssueTrailerRejectsKeysAndFlags` | a key, an alias, a flag, upper case, 15 and 17 hex, a newline, a leading space and another prefix are all false | a validator that accepts any `is-` prefix, or an unanchored expression |
| `TestIssueTrailerValuesSortsDedupesAndDropsForgeries` | two records, one repeat, two forgeries → exactly the two valid ids, sorted; `nil` in → `[]` out | a reader that copies every value, keeps order, or returns `nil` (a `null` in `LandResult`) |

Run: `go test ./control-plane/internal/gitx/... -race -count=1 -run 'IssueTrailer'`
Expected: FAIL, `undefined: gitx.ValidIssueTrailer`.

- [ ] **Step 2: Implement the `gitx` block**

Append the `trailers.go` block above. Re-run step 1's command. Expected: `ok`.

- [ ] **Step 3: Write the failing environment tests**

Append to `control-plane/internal/backend/backend_test.go`, beside P3-05's three `EnvEntries` rows:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEnvEntriesPutsTheIssueAfterTheSession` | `Spec{SessionID:"s-1", IssueID:"is-3f2a91c07b1e4d2a"}` → the three, then `PANACEA_APP_SESSION_ID=s-1`, then `PANACEA_APP_ISSUE_ID=is-…`, then `Env` sorted | the id written anywhere else in the order |
| `TestEnvEntriesOmitsAnEmptyIssueID` | `IssueID == ""` → no `PANACEA_APP_ISSUE_ID` entry at all | an empty `Panacea-Issue:` value reaching the hook |
| `TestEnvEntriesIgnoresAnIssueIDInEnv` | `Env{"PANACEA_APP_ISSUE_ID":"is-evil…"}` with `IssueID:"is-1…"` → exactly one entry, `IssueID`'s value; `ValidateSpec` on the same Spec returns `ErrUnsupported` | a client-chosen issue id |

And to the `internal/api` test file task 19 extends for session create:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSessionCreateExportsTheIssueID` | a create whose Materialiser result has `IssueRef: "issue:is-3f2a91c07b1e4d2a@3"` → the fake backend's recorded `Spec.IssueID == "is-3f2a91c07b1e4d2a"`; the restart path records the same | the revision (`@3`) or the whole key reaching the trailer; the restart site forgotten |
| `TestSessionCreateWithoutIssueExportsNothing` | `IssueRef == ""` → `Spec.IssueID == ""` | an id carried over from a previous create |
| `TestValidIssueTrailerAgreesWithHubapi` | for twenty ids built as `"is-" + hex.EncodeToString(8 bytes of crypto/rand)` (the test may not import the Hub's `ids` package, `hub-contract.md:78-82`) and the twelve bad values of step 1, `gitx.ValidIssueTrailer(v) == hubapi.ValidID(hubapi.PrefixIssue, v)` | the two grammars drifting apart |

Run: `go test ./control-plane/internal/backend/... ./control-plane/internal/api/... -race -count=1 -run 'IssueID|IssueTrailer'`
Expected: FAIL, `unknown field IssueID in struct literal of type backend.Spec`.

- [ ] **Step 4: Implement the environment and the stamp**

Add `EnvIssueID`, `Spec.IssueID` and the `EnvEntries` entry to `backend.go`; add `issueIDOf` and
the two `IssueID:` lines to `sessions.go`. Re-run step 3's command. Expected: `ok`.

Then append the `case` block to `images/sandbox/githooks/dispatch` and add two rows to P3-05's
`hooks_docker_test.go`, on its rig (a clone mounted at `/home/dev/work`, the session environment):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestDispatchStampsTheIssueOnce` (docker) | with `PANACEA_APP_ISSUE_ID=is-3f2a91c07b1e4d2a`: `git log -1 --format=%(trailers:key=Panacea-Issue,valueonly)` is that id, once; after `git commit --amend` with `PANACEA_APP_ISSUE_ID=is-0000000000000001` it is still the first id, once | a hook that stamps again on amend, or replaces the value |
| `TestDispatchOmitsTheIssueWhenUnset` (docker) | the variable unset, then set to `""`, then to `is-3f2a91c07b1e4d2a Panacea-Land: w-x` → no `Panacea-Issue` line in any of the three, and `Panacea-Session` is still stamped | an empty `Panacea-Issue:` line; trailer injection through the variable |

Run: `go test -tags docker ./control-plane/internal/api/... -race -count=1 -run 'TestDispatch'`
Expected: `ok` (needs Docker and the rebuilt sandbox image; P3-05's rig builds it).

- [ ] **Step 5: The lane index and the branch name, tests first**

Create `control-plane/internal/lanes/issue_test.go` and `issuebranch_test.go`:

```go
func TestLaneRecordsItsIssue(t *testing.T) {
	dir := t.TempDir()
	s, err := lanes.OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	l := lanes.Lane{ID: "w-0000000a", ProjectID: "p-1", Branch: "main", Name: "ep-142",
		Path: filepath.Join(dir, "p-1", "ep-142-abc123"), MachineID: "local",
		IssueID: "is-3f2a91c07b1e4d2a", CreatedAt: time.Unix(1790000000, 0).UTC()}
	if err := s.Add(l); err != nil {
		t.Fatal(err)
	}
	re, err := lanes.OpenFileStore(dir) // a second process reading the same file
	if err != nil {
		t.Fatal(err)
	}
	got, ok := re.ByID(l.ID)
	if !ok || got.IssueID != l.IssueID {
		t.Fatalf("reopened lane = %+v (ok %v), want IssueID %q", got, ok, l.IssueID)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "lanes.json"))
	if !bytes.Contains(raw, []byte(`"issueId":"is-3f2a91c07b1e4d2a"`)) {
		t.Fatalf("lanes.json does not hold the issue id: %s", raw)
	}
}

func TestIssueBranchNameAUOnlyHasNoSlug(t *testing.T) {
	if got := lanes.IssueBranchName("EP-142", "Resume upload fails on Safari", true); got != "ep-142" {
		t.Fatalf("AU only: got %q, want %q", got, "ep-142")
	}
	if got := lanes.IssueBranchName("EP-142", "Resume upload fails on Safari", false); got != "ep-142-resume-upload-fails-on-safari" {
		t.Fatalf("Global: got %q", got)
	}
}

func TestIssueBranchNameSlugIsBounded(t *testing.T) {
	title := strings.Repeat("Überprüfung der Lebensläufe 中文 ", 8) // 200+ characters, non-ASCII
	got := lanes.IssueBranchName("EP-142", title, false)
	if len(got) > 40 || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(got) ||
		strings.Contains(got, "--") {
		t.Fatalf("IssueBranchName = %q (%d bytes): not a bounded, legal worktree name", got, len(got))
	}
	if got := lanes.IssueBranchName("EP-142", "中文 !!!", false); got != "ep-142" {
		t.Fatalf("an all-symbol title: got %q, want the alias alone", got)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLaneRecordsItsIssue` | the id survives a reopen and is in the file | a field the store's on-disk schema drops |
| `TestCreateCopiesTheIssueIntoTheLane` (on P3-03's `gitxtest.Repo` rig) | `Create(CreateRequest{…, IssueID: "is-…"})` → the returned and stored `Lane.IssueID` equal it; `IssueID: "EP-142"` → `ErrBadIssue` and no directory under the clone root | an id the request carries but the lane loses; a bad id checked after the clone |
| `TestIssueBranchNameAUOnlyHasNoSlug` | AU only → the alias alone; Global → alias and slug | a slug on an AU project |
| `TestIssueBranchNameSlugIsBounded` | a 200+ character non-ASCII title → ≤ 40 bytes, `[a-z0-9-]`, no `--`, no edge `-`; an all-symbol title → the alias | an unbounded or illegal ref name; a trailing `-` after the cut |

Run: `go test ./control-plane/internal/lanes/... -race -count=1 -run 'Issue'`
Expected: FAIL, `unknown field IssueID in struct literal of type lanes.Lane`. Implement the two
fields, `ErrBadIssue`, the copy in `Create` and `issuebranch.go`; re-run. Expected: `ok`.

- [ ] **Step 6: Land writes one trailer per issue, test first**

Append to `control-plane/internal/converge/land_test.go`, on P3-09's rig (a project, one lane
clone, the lane synced to `LaneRef`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLandAddsOnePerDistinctIssue` | the lane (its `IssueID` is `is-bbbbbbbbbbbbbbbb`) holds three commits: two with `Panacea-Issue: is-aaaaaaaaaaaaaaaa`, one with `Panacea-Issue: --octopus` written by `git interpret-trailers` directly. After `Land`: `git log -1 --format=%(trailers:key=Panacea-Issue,valueonly,separator=%x2C) <branch>` is exactly `is-aaaaaaaaaaaaaaaa,is-bbbbbbbbbbbbbbbb`, and `LandResult.Issues` is the same two, in that order | a land that copies every value, keeps a forgery, forgets the lane's own id, or leaves them unsorted |
| `TestLandWithoutIssuesIsUnchanged` | a lane with no `IssueID` and no trailers → the land commit's message is byte-identical to P3-09's golden for the same inputs, and the raw body holds `"issues":[]` | a land that writes an empty trailer, or `null` |
| `TestLandIssueReadRunsInTheProject` | the recording Runner shows the `log` call with `Dir == ProjectPath`, `Class == gitx.Read`, `Scope == gitx.Project`, and no call with `Dir` under the clone root | reading the lane's own clone (R4.2) |

Run: `go test ./control-plane/internal/converge/... -race -count=1 -run 'Issue'`
Expected: FAIL, `LandResult has no field or method Issues`. Implement `landIssues`, step 9a, the
trailers in step 10 and `Issues`; re-run. Expected: `ok`.

- [ ] **Step 7: The whole acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/gitx/... ./control-plane/internal/lanes/... ./control-plane/internal/converge/... ./control-plane/internal/backend/... ./control-plane/internal/api/... -race -count=1'
```

Expected: one `ok` line per package, no `FAIL`. With Docker available, also run step 4's
`-tags docker` command; expected `ok`. Check `git diff -- go.work control-plane/go.mod` is empty
(the `go 1.23` directive holds), then commit with `git commit -F <msgfile> -- <the paths above>`.

#### What SW.9 fixes, in one place

| Where | Key | Value | How many |
|---|---|---|---|
| every commit of a session bound to an issue | `Panacea-Issue` | the bare `is-` id, exactly `^is-[0-9a-f]{16}$` | once, beside `Panacea-Session`; an amend keeps the first |
| a land commit | `Panacea-Issue` | each distinct valid value on `merge^1..laneTip`, plus the lane's `IssueID` | one line per issue, sorted |
| recovering the revision the agent was told | — | `Panacea-Session` → the session record → `Session.IssueRef` (task 19) | never from the trailer |

Readers (tasks 22, 71, 131, 154) read the value with `gitx.IssueTrailerValues` or re-check it with
`gitx.ValidIssueTrailer`; none parses it as a key or an alias.

#### What this task does not claim

- **Not the `landed` move** or any Hub write: task 22 reads `LandResult.Issues` and sends it.
- **Not where a project's data class lives.** That home is **UNVERIFIED until merge** (Phase T
  `task-01.md:484-486` at `04e6944`; the scope cited `:454-456`, which has since moved); `auOnly` is a parameter and task 24 supplies it.
- **Not that the trailer is present.** P3-05's hook is advisory (P3 `task-05.md:151-152`): an
  agent can unset the variable or skip hooks. That is why land also adds the lane's own
  `IssueID`, and why a commit without the trailer is still attributed through its lane.
- **Not that a phase 3 file looks as cited.** Every phase 3 path and line here is from the plan
  at `9935278`, **UNVERIFIED until merge**; the implementer re-reads each at the sync base.
- Design: 05 §2.3, §3.3 (`landed`), §3.6 (trailers carry ids); 04 §5.2, §7.3. Parity: with task
  22 it closes the W1 parts of **L-146** and **L-200**; with task 24 the W1 part of **L-198**.

**Interfaces:**
- Produces (seam **SW.9**):
  - `gitx.TrailerIssue = "Panacea-Issue"`; `gitx.ValidIssueTrailer(v string) bool`;
    `gitx.IssueTrailerValues(stdout []byte) []string`
  - `backend.EnvIssueID = "PANACEA_APP_ISSUE_ID"`; `backend.Spec.IssueID string`
  - `lanes.Lane.IssueID`, `lanes.CreateRequest.IssueID`; `lanes.IssueBranchName(alias, title
    string, auOnly bool) string`
  - `converge.LandResult.Issues []string` — the sorted, deduplicated, valid issue ids this land
    wrote as trailers; `[]`, never `null`. Task 22's `Landed` takes it as its `issueIDs`
- Consumes:
  - task 02 — `hubapi.ParseIssueKey(s) (hubapi.IssueKey, error)`, `hubapi.IssueKey{IssueID,
    Revision}`, `hubapi.PrefixIssue`, `hubapi.ValidID(prefix, id)` (`keys.go:31-35` on `feat/hub`
    at `9f2fa31`, verified)
  - task 19 — `registry.Session.IssueRef` and `MaterialiseResult.IssueRef`, an `issue:is-…@r` key
    or `""`
  - P3-02 — `gitx.Message`, `gitx.Trailer`, `gitx.TrailerFormat`, `gitx.ValueSep`,
    `gitx.RecordSep`, `gitx.Call`, `gitx.Read`, `gitx.Project`
  - P3-03 — `lanes.Store`, `lanes.FileStore`, `lanes.Creator.Create`, `lanes.ErrBadName` (a name
    is 1-40 of `a-z 0-9 . _ -`, starting with a letter or digit, P3 `task-03.md:128`)
  - P3-09 — `converge.Lander.Land`, `LandRequest`, `LandResult`
- Amends: nothing in the Hub. SW.9 is git-side only.
