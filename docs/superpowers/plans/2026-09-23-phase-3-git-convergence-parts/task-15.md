### Task 15: Evidence — records a human makes, the store seam the Hub fills (H-2), and the evaluator over git ancestry

Obligation 1 (contract §2.3): the criterion evaluator takes `(brief key, commit)` and returns
states, never a branch id. Nothing on file said what evidence **is**; 3-D5 settles it: **a human,
per criterion, at the commit in front of them, records a verdict and a required *because*.** A
recorded human judgement is a fact — who, when, at which commit, why — where an LLM's verdict would
be a claim (`AGENTIC-UX.md:202`). The evaluator is then mechanical (survey §8.4): the state of a
criterion at commit C is decided by the evidence whose commit is C or an ancestor of C and was not
carried by a land that a revert in C's ancestry took back (review I9), the descendant superseding
its ancestor, disagreement being `contested`, nothing being `open`. That
makes the three surfaces of contract §2.3 agree by construction — the Develop chip at a worktree
head, the Merge view at the branch head, the Deploy manifest at `Deployment.Commit` — and it gives
the Merge queue "landing settles criterion 3" from reachability alone.

**Matching is by criterion-text digest** (plan decision 5): the positional criterion id breaks on
insertion (risk R14), and matching on the key would need phase 3 to parse it (P5). The digest comes
from phase 4's resolver (ask P4-2), so evidence carries across revisions exactly when the
criterion's text did not change, and the surface can say "evidence recorded against rev 3" by
resolving the record's key.

**Where evidence lives** (OD-3, open question Q2): shared records belong to the Hub. Phase 3
declares `evidence.Store` (Hub ask H-2) and ships a local file store whose `Shared()` is false; the
Merge lens labels those records "recorded on this machine only".

**Files:**
- `control-plane/internal/evidence/record.go` — `Verdict`, `Record`, validation (new)
- `control-plane/internal/evidence/store.go` — `Store`, `FileStore` (new)
- `control-plane/internal/evidence/evaluate.go` — `State`, `Evaluator`, `Settles` (new)
- tests for each (new)
- `control-plane/internal/api/evidence.go` + test — POST, GET records, GET states (new)
- `control-plane/internal/converge/convergence.go` — fill `QueueEntry.Settles` (task 08's file)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go`, `routes_audit_test.go` — `// ── task 15 ──`

#### Interfaces — complete

```go
package evidence

type Verdict string

const (
	Settled   Verdict = "settled"
	Contested Verdict = "contested"
)

// Record is one human judgement about one criterion at one commit. Append-only: a record is
// never edited or deleted by phase 3 (retention is contract Q5).
type Record struct {
	ID              string    `json:"id"`              // "ev-" + 12 hex, minted by the store
	Repo            string    `json:"repo"`            // githost.Remote.RepoKey(), else "local:<projectId>" (Hub ask H-3)
	ProjectID       string    `json:"projectId"`       // this machine's id; lookups on this machine only
	BriefKey        string    `json:"briefKey"`        // verbatim; never parsed (P5)
	CriterionID     string    `json:"criterionId"`
	CriterionDigest string    `json:"criterionDigest"` // what the evaluator matches on
	Commit          string    `json:"commit"`          // 40 hex; pinned by refs/panacea/evidence/<id>
	Verdict         Verdict   `json:"verdict"`
	Because         string    `json:"because"`         // required, 1-2000 characters after trimming (P12)
	RecordedBy      string    `json:"recordedBy"`      // "Name <email>", the git identity (Hub ask H-4)
	RecordedAt      time.Time `json:"recordedAt"`
}

var ErrInvalid = errors.New("evidence: the record is not valid")

// Validate checks every field but ID and RecordedAt, and wraps ErrInvalid with the field.
func (r Record) Validate() error

// Store is Hub ask H-2.
type Store interface {
	// Record appends r, minting ID and RecordedAt.
	Record(ctx context.Context, r Record) (Record, error)
	// ByDigest returns every record in repo for any of the digests, oldest first.
	ByDigest(ctx context.Context, repo string, digests []string) ([]Record, error)
	// Shared reports whether a teammate sees these records (the Hub: true) or only this machine.
	Shared() bool
}

// FileStore is the local store: statepath.Dir("evidence")/evidence.json, schemaVersion 1, one
// document, written through statepath.WriteAtomic, append-only. Shared() is false.
type FileStore struct{ /* unexported */ }

func OpenFileStore(dir string) (*FileStore, error)
```

```go
package evidence

// State is one criterion at one subject commit.
type State struct {
	CriterionID     string   `json:"criterionId"`
	CriterionDigest string   `json:"criterionDigest"`
	State           string   `json:"state"`           // "open" | "settled" | "contested"
	Because         []string `json:"because"`         // the deciding records' becauses; open: [] with no evidence, or the one revert sentence (States)
	Evidence        []string `json:"evidence"`        // their ids
	RecordedAgainst []string `json:"recordedAgainst"` // their brief keys, verbatim, for the UI to resolve
	Commits         []string `json:"commits"`         // their commits
}

// Evaluator is the S3-P7.5 seam and ask P4-4: phase 4 calls it for DefinitionOfDone at the
// branch head, phase 7 for a release at Deployment.Commit.
type Evaluator struct {
	Machine gitx.Machine
	Store   Store
}

// States evaluates each criterion at subject, in the criteria's order:
//   records = Store.ByDigest(repo, digests)
//   reachable(r) = merge-base --is-ancestor r.Commit subject   (gitx.Read, OKExit 1; an object
//                  the repository does not have — a teammate's unfetched commit — is not reachable)
//   reverts = log --format=%H%x1f<gitx.TrailerFormat(gitx.TrailerReverts)> subject
//             (every ancestor of subject, not first-parent only; lines with an empty value dropped)
//   retired(r) = some M in reverts has: merge-base --is-ancestor r.Commit M (exit 0) and
//                merge-base --is-ancestor r.Commit M^1 (exit 1) — r lies in M^1..M, the land task
//                10's revert took back (its merge-tree uses --merge-base=M against M^1, so exactly
//                M and the lane commits M brought in are undone); an M the repository lacks
//                retires nothing (review I9)
//   per digest: drop retired records, then keep reachable records not a proper ancestor of
//   another kept record's commit; none kept and none retired → open, because [];
//   none kept, some retired → open, because ["the land that carried this evidence was reverted"],
//   Evidence/RecordedAgainst/Commits naming the retired records; one verdict → that verdict;
//   both verdicts → contested.
// Retiring before pruning is deliberate: an older record at a commit the revert did not take
// back (an ancestor of M^1) decides again, because the revert restored the code it judged.
// The reverts walk is the whole ancestry, not first-parent: a lane that merged origin/<branch>
// after the revert (task 19's resolve prompt) carries the revert off its first-parent line, and a
// first-parent walk would leave the Develop chip at that lane's head settled. Any ancestor's
// Panacea-Reverts trailer counts, an agent's forged one included; the only effect of a forgery is
// a criterion reading open, never settled, so this errs the safe way. Re-landing reverted work
// takes new commits, and they need new evidence. The walk is one git log per States call, run
// only when some record is reachable.
// It never takes a branch id (contract §2.3). merge-base and log run in the project with Scope
// Project (task-02 §HGH).
func (e *Evaluator) States(ctx context.Context, repo, projectPath, subject string, criteria []briefref.Criterion) ([]State, error)

// Settles returns the ids of criteria whose state is not settled at before and is settled at
// after — for a queue row, before is the branch tip and after the row's ForecastCommit (task 08).
func (e *Evaluator) Settles(ctx context.Context, repo, projectPath, before, after string, criteria []briefref.Criterion) ([]string, error)
```

```go
// gitdeps.go
	// ── task 15 ──
	Evidence  evidence.Store // *evidence.FileStore until the Hub (H-2)
	Evaluator *evidence.Evaluator
```

**Wire:**

```
POST /v1/projects/{projectId}/evidence
  request { "briefKey": "brief:b-42@6", "criterionId": "b-42:done:3", "criterionDigest": "sha256:…",
            "commit": "7c2e5b1…(40 hex)", "verdict": "settled",
            "because": "An upload killed at 60% resumed at chunk 13 in the run at 7c2e5b1." }
  201 Record, and refs/panacea/evidence/<id> now names commit
  400 bad_request   a field fails Validate, or the commit is not in the repository
  409 conflict      brief_unresolvable (details.why: no_resolver | missing | no_such_criterion | digest_mismatch)
                    no_git_identity

GET /v1/projects/{projectId}/evidence?digest=sha256:a&digest=sha256:b
  200 { "records": [Record…], "shared": false }

GET /v1/projects/{projectId}/evidence/states?commit=<40 hex>&briefKey=<key>
  200 { "commit": "…", "briefKey": "…", "states": [State…], "shared": false }
  409 brief_unresolvable — the criteria come from the resolver, never from the client
```

The POST resolves `briefKey` through `GitDeps.Resolver` and refuses a `(criterionId,
criterionDigest)` pair the revision does not have, so a client cannot record evidence against a
sentence the brief never contained. `RecordedBy` is `GitIdentity(projectPath)`. The convergence GET
(task 08), given `?briefKey=`, resolves it and fills each clean row's `Settles` with
`Evaluator.Settles(tip, row.Forecast.ForecastCommit)`; with no key or no resolver, `Settles` stays
`[]`. Audit lines for both GETs: *"Reads the evidence store, and git ancestry with merge-base.
Writes nothing."*

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestValidateRequiresABecause` | `because: "   "` → ErrInvalid naming `because` | a non-open state without its because (P12) |
| `TestValidateRefusesUnknownVerdicts` | `"passed"` → ErrInvalid | a third verdict the evaluator cannot combine |
| `TestFileStoreIsAppendOnly` | two records, reopen → both, oldest first; the type has no Update or Delete method (compile-time) | evidence rewritten after the fact |
| `TestFileStoreSharedIsFalse` | → false | a local record labelled as shared |
| `TestStatesOpenWithNoEvidence` | → `open`, `because: []` | an empty store reading as settled |
| `TestStatesSettledAtADescendant` | record at C1, subject C3 (C1 ← C2 ← C3) → settled | ancestry ignored |
| `TestStatesOpenAtAnAncestor` | record at C3, subject C1 → open | evidence leaking backwards in time |
| `TestStatesDescendantSupersedes` | contested at C1, settled at C2 (C2 descends from C1), subject C3 → settled, one because | old verdicts never retired |
| `TestStatesUnrelatedDisagreementIsContested` | settled on lane A, contested on lane B, subject a merge of both → contested, both becauses | one verdict silently winning |
| `TestStatesMatchOnTheDigestNotTheID` | a record for id `done:2` digest d; the criterion now `done:3` with digest d → settled, `recordedAgainst` the old key | evidence lost when a criterion is inserted above (R14) |
| `TestStatesIgnoreAChangedCriterion` | same id, a different digest → open | evidence attaching to a rewritten sentence |
| `TestStatesUnknownCommitIsUnreachable` | a record whose commit the repository lacks → open, no error | a 500 for a teammate's unfetched commit |
| `TestStatesARevertedLandUnsettles` (review I9) | real git: lane commit L1 records settled; land M (parents `[T, L1]`); a revert R of M built the way task 10 builds one (`commit-tree` of M^1's tree, one parent M, message from `gitx.Message` carrying `Panacea-Reverts: <M>`), so task 15 keeps its dependencies 07 and 09 and imports nothing from task 10; subject R → `open`, `because` exactly `["the land that carried this evidence was reverted"]`, `evidence` the L1 record; subject M (before R) → settled | a revert that keeps its land's commits as ancestors reading as done (contract §2.3, S3-P7.5) |
| `TestStatesARevertRestoresTheOlderVerdict` | the lane cloned at T (the tip before M), so T is an ancestor of L1; contested recorded at T, settled at L1, M then R; subject R → `contested`, T's because | retiring after pruning, so the older record stays hidden behind a retired one |
| `TestStatesARecordAtTheLandItselfIsRetired` | settled recorded at M, then R; subject R → open with the revert because | the range taken as `M^1..M^2`, missing a record made at the merge |
| `TestStatesARevertOffTheFirstParentLineCounts` | lane B merges the branch after R (R on B's second-parent side); subject B's head → open | a first-parent walk leaving the Develop chip settled |
| `TestStatesARevertOfAnotherLandRetiresNothing` | lands A and B, R reverts B only; settled at A's lane commit; subject R → settled | a revert unsettling every land on the branch |
| `TestSettlesOnlyNewlySettled` | a criterion settled before and after → not listed; open before, settled after → listed | "landing settles" claims for criteria already settled |
| `TestPostPinsTheCommit` | after POST, `refs/panacea/evidence/<id>` names the commit; the commit survives `git gc --prune=now` after its lane is rewritten | evidence pointing at a collected object |
| `TestPostRefusesACriterionTheBriefLacks` | a pair not in the resolver's revision → `no_such_criterion` | evidence about a sentence no brief contained |
| `TestPostWithoutAResolver` | nil → `no_resolver` | unverifiable records |
| `TestStatesRouteTakesCriteriaFromTheResolver` | the request carries no criteria; the response lists the resolver's, in order | the client defining the spec |
| `TestConvergenceFillsSettlesWithAKey` | a lane carrying a settled record, `?briefKey=` with a resolver → that row's `settles` names the criterion | the prototype's "Landing settles criterion 3" never appearing |
| `TestEvidenceGETsAreReadOnly` | the §10a audit passes with both lines; `for-each-ref` unchanged by a GET | a GET that pins |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 07 and 09 merged (lane refs, lands; `briefref` is task 03's since review I7). R2's `statepath.WriteAtomic` exists.
- [ ] **Step 2:** tests; `record.go`, `store.go`, `evaluate.go`; the handlers; `Settles` in the
  convergence read.
- [ ] **Step 3:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/evidence/ ./control-plane/internal/converge/ ./control-plane/internal/api/ -race -count=1 → PASS
```

#### What this task does not claim

- **Evidence recording is live only when phase 4 provides a resolver** (gate G3). Until then the
  POST answers `brief_unresolvable`/`no_resolver`, correctly: there is no brief to judge against.
- Local records are one machine's. The Hub's store (H-2) makes them shared; no migration of local
  records is built (Q2).
- Agent-proposed and test-linked evidence (3-D5 b and c) are later phases.
- `recordedBy` is a git identity, which the user can set to anything; the Hub's signed-in
  principal replaces it (H-4).
