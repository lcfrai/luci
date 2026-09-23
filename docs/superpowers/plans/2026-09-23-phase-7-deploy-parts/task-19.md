### Task 19: Sessions in a release, from `Panacea-Session` trailers

**Half:** 7b. **Hours:** 2. **Depends on:** 08, 17. **Elsewhere:** phase 3 task 02 (`gitx`:
`Runner`, `TrailerSession`, `TrailerFormat`, the separators) and task 05 (the hook that stamps
`Panacea-Session: <id>` on every agent commit; seam `S3-P7.1`).

Decision `D3` (b): the manifest lists **the sessions whose commits a release shipped**, read from
the trailers in the shipped range — never inferred from a commit's time (`S3-P7.1`, *Must not*).
The range is **the previous release's commit on the same environment, exclusive, to this release's
commit, inclusive**: what this release put in front of people that the one before it did not
(prototype `viewThread`, `:861`: *"Sessions whose commits fall between the previous release on
this environment and `<sha>`"*). Git is read in the local clone; nothing is asked of Panacea or the
Hub.

**Files:**
- `control-plane/internal/deploy/sessions/sessions.go` (new) — `Range`, `Result`, `Reader`, `Read`
- `control-plane/internal/deploy/sessions/sessions_test.go` (new) — over `gitxtest.Repo`
- `control-plane/internal/api/deploy_drill.go` (modified, task 12's) — `GET {P}/sessions`
- `control-plane/internal/api/deploy_drill_test.go` (modified)
- `control-plane/internal/api/routes_audit_test.go` (modified) — one `GET` line
- `control-plane/internal/api/api.go`, `control-plane/cmd/api/main.go` (modified) — `Deps.DeploySessions`, the wiring
- `desktop/src/deploy/types.ts`, `api.ts` (modified, task 13's) — `ReleaseSessions`, `DeployApi.sessions`
- `desktop/src/deploy/ReleaseSessions.tsx` (new) and its test — the drawer's sessions block
- `desktop/src/deploy/DeployLens.tsx` (modified) — passes `<ReleaseSessions>` into task 17's `sessions` slot

#### Interfaces — complete

```go
// Package sessions reads which cockpit sessions produced a release, from the Panacea-Session
// trailers phase 3's hook writes. It reads git only (gitx Class Read, Scope Project) and derives
// nothing from time, authorship or PTY bytes.
package sessions

// MaxCommits bounds one read. A range longer than this is read to this many and marked Truncated.
const MaxCommits = 200

// Range is one release's shipped range. From "" means there is no earlier release on the
// environment: the range is Commit's own history, to MaxCommits.
type Range struct {
	Dir    string // the local project's path (project.Project.Path)
	From   string // 40 hex, or ""
	Commit string // 40 hex
}

type Session struct {
	ID      string `json:"id"`
	Commits int    `json:"commits"` // commits in the range carrying this id
	// Known is the local session registry's record, or nil for an id this control plane does
	// not hold — a teammate's session, or one since deleted. Never guessed.
	Known *Known `json:"known"`
}

type Known struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
}

type Result struct {
	State string `json:"state"` // "ok" | "commit_not_local" | "previous_not_local"
	// Message is "" for "ok"; else the sentence the drawer shows.
	Message   string    `json:"message"`
	From      string    `json:"from"` // "" when there is no earlier release
	Commit    string    `json:"commit"`
	Commits   int       `json:"commits"`   // commits read, ≤ MaxCommits
	Truncated bool      `json:"truncated"` // true when the range held more than MaxCommits
	Sessions  []Session `json:"sessions"`  // newest first by first appearance; [] never null
	// Outside counts commits in the range with no Panacea-Session trailer.
	Outside int `json:"outside"`
}

// Registry is the one method of registry.Store this package calls.
type Registry interface {
	Get(id string) (registry.Session, error) // registry.ErrNotFound → Known nil
}

type Reader struct {
	Git      gitx.Runner
	Registry Registry
}

// Read lists the sessions in r. It runs at most three git commands, each Class Read:
//  1. git cat-file -e <Commit>^{commit}      absent → State "commit_not_local"
//  2. git cat-file -e <From>^{commit}        (From != "") absent → State "previous_not_local"
//  3. git log --no-color --max-count=<MaxCommits+1>
//        --format=%H%x00<gitx.TrailerFormat(gitx.TrailerSession)>%x1e  <From>..<Commit> | <Commit>
// Any other git failure is returned as an error (the route's 500), never as an empty list.
func (rd *Reader) Read(ctx context.Context, r Range) (Result, error)
```

Every git call is `gitx.Call{Dir: r.Dir, Class: gitx.Read, Scope: gitx.Project, Args: …}` — `Read`
never takes an optional lock, so it cannot race an agent's own git for `index.lock` (phase 3 task
02). Records split on `gitx.RecordSep`, fields on `gitx.FieldSep`, values on `gitx.ValueSep`. A
commit whose trailer carries two ids counts once for each. `MaxCommits + 1` rows read →
`Truncated true`, and the extra row is dropped.

**The two sentences:**
- `commit_not_local`: *"This clone does not have commit `<short>`. Fetch, then look again."*
- `previous_not_local`: *"This clone does not have the previous release's commit `<short>`, so the
  range cannot be drawn. Fetch, then look again."* — **never** a fallback to the commit's whole
  history, which would credit this release with sessions an earlier one shipped.

#### Which release is "the previous one" — in the route, from the view

The route assembles the view (`deploywatch.Assemble(snapshot, w.SameEnvironment, now)`, task 08)
and finds the row whose `deploymentId` is the query's. **From** is the `commit` of the newest row
that is older (`at`) than it, has the **same `environmentRow`** (the view's own join, so `prod`
and `production` rows are one environment; when both are `null`, the same raw `environment`, which
is how `unmatched` groups them — review I8, never a second join on raw names), `kind "run"`, `build "succeeded"` and a 40-hex
`commit`. Rollback rows are skipped (they ship no new code); failed builds are skipped (their
commit never served). None → `From ""`.

#### Control-plane route — complete wire

```
GET {P}/sessions?deploymentId=engagement-portal-build-x7k2p
  200 Result, e.g.
      { "state": "ok", "message": "", "from": "<40 hex>", "commit": "<40 hex>",
        "commits": 7, "truncated": false, "outside": 2,
        "sessions": [ { "id": "s-4f1e", "commits": 4,
                        "known": { "title": "Upload form", "status": "idle",
                                   "projectId": "p-1a2b3c4d", "branchId": "b-…" } },
                      { "id": "s-9c07", "commits": 1, "known": null } ] }
  400 bad_request       deploymentId missing, or outside [a-z0-9.-]
  404 deploy_not_found  no row with that deploymentId in the view
  409 deploy_precondition  details.reason "no_commit" — the row has no 40-hex commit
                        ("This release did not record a full commit.", task 16's sentence)
  503 upstream_unavailable  the watcher has no snapshot yet (details.upstream "panacea")
```

`catalogReadOnlyRoutes` line: *"Reads Panacea-Session trailers from the local clone with git log
(Class Read, no optional locks); writes nothing."*

#### Desktop — contract

```ts
// types.ts — mirrors sessions.Result field for field.
export interface ReleaseSessions {
  state: "ok" | "commit_not_local" | "previous_not_local";
  message: string;
  from: string;
  commit: string;
  commits: number;
  truncated: boolean;
  outside: number;
  sessions: Array<{
    id: string;
    commits: number;
    known: { title: string; status: string; projectId: string; branchId: string } | null;
  }>;
}
// api.ts — DeployApi gains:
//   sessions(projectId: string, deploymentId: string): Promise<ReleaseSessions>;

export function ReleaseSessions(props: { projectId: string; row: ReleaseRow }): JSX.Element | null;
// null when row.deploymentId is null (a pending or refused row has no range).
```

**Words** (the heading is the prototype's, *"SESSIONS THAT PRODUCED IT"*):

| Case | Drawn |
|---|---|
| a known session | its `StatusDot` and `title`, *"`N` commits"*; a click runs phase 2's `selectBranch(known.projectId, known.branchId)` then `focusSession(id)` (`uiSlice.ts:57-60`) |
| an unknown id | the id in mono and *"a session this cockpit does not hold — a teammate's, or since deleted"*; no click |
| `outside > 0` | a hollow dot and *"`N` commits carry no session record — made outside any session, or before sessions were recorded"* |
| `sessions` empty and `outside == commits` | only the line above — never *"no sessions"* as if that were a finding |
| `truncated` | *"Showing the newest 200 commits of the range."* |
| `from == ""` | under the list: *"The first release on this environment: its whole history, to 200 commits."* |
| not `ok` | `message` verbatim, and **Look again** |
| always | *"Commits between the previous release on this environment and `<short commit>`."* |

The block reads once when the drawer opens on the row — never on the cursor, never on a timer: a
release's range does not change after it ships.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestRangeIsExclusiveInclusive` (real git, `gitxtest.Repo`) | commits c1(s-1), c2(s-2), c3(s-2); From c1, Commit c3 → sessions `[s-2 ×2]`; `s-1` absent | counting the previous release's own commit as this one's |
| `TestNoTrailerCountsOutside` | c2 with no trailer → `outside 1`, `sessions` without an entry for it | an unattributed commit credited to a nearby session |
| `TestTwoIdsOnOneCommit` | one commit with two `Panacea-Session` trailers → both ids, one commit each | dropping the second value of a multi-valued trailer |
| `TestFirstReleaseHasNoFrom` | From "" → the whole history of Commit, to `MaxCommits` | a first release listing nothing |
| `TestTruncatedAtMax` | 205 commits → `commits 200`, `truncated true` | an unbounded `git log` on a large repository |
| `TestCommitNotLocal` | Commit absent → `commit_not_local` and its sentence; `git log` never run | an empty list presented as "no sessions" |
| `TestPreviousNotLocalNeverWidens` | From absent → `previous_not_local`; `git log` never run | falling back to the whole history and crediting earlier releases' sessions |
| `TestReadClassOnly` (`gitxtest.Script`) | every call is `Class Read`, `Scope Project` | a lock-taking call racing an agent's git |
| `TestUnknownSessionNotGuessed` | registry `ErrNotFound` for `s-9` → `known: null` | a teammate's session shown as a local one, or dropped |
| `TestGitFailureIsAnError` | the script fails `git log` (exit 128) → `Read` returns an error; the route `500` | a broken read drawn as an empty, confident list |
| `TestPreviousSkipsRollbackAndFailed` (route) | history: run d1 ok (c1), run d2 failed (c2), rollback, run d3 ok (c3) → the read for d3 has `From c1` | a range starting at a commit that never served |
| `TestPreviousIsSameEnvironment` (route) | an older run on another environment is ignored | two environments' histories spliced into one range |
| `TestSessionsRouteIsReadOnly` | the route is in `catalogReadOnlyRoutes`; the §10a audit passes | a GET registered outside the recorder |
| `reads once` (desktop) | drawer open → one `api.sessions`; three cursor changes → still one | a git log per cursor tick |
| `known session navigates` (desktop) | click → `selectBranch(p, b)` then `focusSession("s-4f1e")`; an unknown id is not a button | a link that lands on the wrong branch, or a dead click |

#### Steps

- [ ] Check first: phase 3's `control-plane/internal/gitx` exists with `TrailerSession` and
  `TrailerFormat`. Absent → this task is built against `gitxtest.Script` only and **not merged**
  (plan §8, 7b).
- [ ] Tests first; `sessions.go`; the route in `deploy_drill.go` with its audit line; `main.go`
  wiring (`Reader{Git: <the gitx.Exec phase 3 builds>, Registry: deps.Store}`).
- [ ] Desktop: the type and client method; `ReleaseSessions`; the `DeployLens` slot.
- [ ] `go test ./control-plane/internal/deploy/sessions/ ./control-plane/internal/api/ -race
  -count=1`; `npx tsc --noEmit`; `npx vitest run`; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/sessions/ ./control-plane/internal/api/ -race -count=1
→ ok
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass
```
**Owner's eyes (batched, task 20):** Ship a branch that two sessions committed to; open the
release; read both sessions and the outside count. **Unverified until then.**

#### What this task does NOT claim

- That a session with no trailer did no work. The trailer is written by phase 3's hook on
  `git commit`; a squash, a rebase that rewrites messages, or work done outside a sandbox carries
  none, and the drawer says *"no session record"*, not *"no session"*.
- That the range is what Panacea built. It is `From..Commit` in **this** clone; Panacea built
  `Commit`. They agree whenever the clone has fetched both commits, which the two refusals enforce.
- A range across a rewritten history. If `From` is not an ancestor of `Commit` (a force-push),
  `git log From..Commit` lists what `Commit` has that `From` lacks — still true, and not labelled.
