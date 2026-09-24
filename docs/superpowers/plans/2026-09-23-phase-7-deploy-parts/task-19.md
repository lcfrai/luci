### Task 19: Sessions in a release, from `Panacea-Session` trailers

**Half:** 7b. **Hours:** 2. **Depends on:** 08, 17. **Elsewhere:** phase 3 task 02 (`gitx`:
`Runner`, `TrailerSession`, `TrailerFormat`, the separators) and task 05 (the hook that stamps
`Panacea-Session: <id>` on every agent commit; seam `S3-P7.1`).

Decision `D3` (b): the manifest lists **the sessions whose commits a release shipped**, read from
the trailers in the shipped range — never inferred from a commit's time (`S3-P7.1`, *Must not*).
The range is **the commit the environment served just before this release went live, exclusive
(task 08 rule 6 at that instant, so a rollback's target counts; review M29), to this release's
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

// Range is one release's shipped range. From is the commit the environment served just before
// this release went live (the route's rule, below). From "" means the environment has no earlier
// row at all: the range is Commit's own history, to MaxCommits.
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
	State string `json:"state"` // "ok" | "commit_not_local" | "previous_not_local" | "previous_unknown" (the route's, no git read; review M29)
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

**The three sentences** (the first is the route's, from the rule below; the other two are `Read`'s):
- `previous_unknown`: *"The cockpit cannot tell which release was serving before this one, so
  the range cannot be drawn."* — no git read, and **Look again** re-reads after the next poll.
- `commit_not_local`: *"This clone does not have commit `<short>`. Fetch, then look again."*
- `previous_not_local`: *"This clone does not have the previous release's commit `<short>`, so the
  range cannot be drawn. Fetch, then look again."* — **never** a fallback to the commit's whole
  history, which would credit this release with sessions an earlier one shipped.

#### Which release is "the previous one" — in the route, from the view

The route assembles the view (`deploywatch.Assemble(snapshot, w.SameEnvironment, now)`, task 08)
and finds the row whose `deploymentId` is the query's. **From** is the commit of **whatever the
environment served just before this row went live** (review M29): task 08 rule 6's own function,
`deploywatch.CurrentAt(snapshot, w.SameEnvironment, env, cut)`, never a second copy of it.
- `env` is the row's `environmentRow` (the view's own join, so `prod` and `production` rows are
  one environment), or, when it is `null`, the row's raw `environment`, which is how `unmatched`
  groups them (review I8; never a second join on raw names).
- `cut` is the row's go-live instant: its deployment's `CompletedAt` when `build "succeeded"` and
  non-zero; otherwise (running, failed, historical) its `CreatedAt`. `CurrentAt` counts only
  events strictly before it, so the row never counts itself.
- `current.source "deployment"` → From is that deployment's `Commit`.
- `current.source "rollback"` → From is the `Commit` of the deployment it returned **to**
  (`current.deploymentId`, looked up in the snapshot). After d1 (c1), d2 (c2), a rollback to d1,
  then d3 (c3), c1 was serving before d3, so d3's range is c1..c3 and credits the sessions
  between c1 and c2 that d3 put back in front of people. The old rule, *"the newest older
  succeeded run"*, took c2 and left them uncredited.
- `current.source "none"` and **no row that could have served before `cut`** → `From ""` (no
  earlier release is recorded here). A row that could have served is a row of this environment in
  `snapshot.Deployments`, **other than this one**, that is `succeeded` with a `CompletedAt` before
  `cut` or zero, or `historical`, or `unknown` (plan re-review b11-1, I-2). A `failed`, `running`
  or `pending` row never served, so it never blocks `From ""`: d1 failed, then d2 succeeded,
  gives d2 `From ""`, not *"cannot tell"*. The row's own `CreatedAt` is before its own `cut`,
  which is why "other than this one" is part of the rule.
- Any other case → `state "previous_unknown"`, and **no git read**: `current.source "none"`
  while a row that could have served exists (backfilled `historical` rows, never current by rule
  6), a rollback whose target is not in the snapshot (past the 200-row history, or pruned), or a
  From commit that is not 40 hex (`deploy.ValidCommit`). Never a fallback to an older row or to
  the whole history, which would credit this release with sessions an earlier one shipped.

**The rollbacks before `cut` must be known** (plan re-review b11-1, I-1). `CurrentAt`'s rollback
candidate comes from `Snapshot.Releases`, which is the Hub's first page only (task 07,
`ReleasesMore`), and is empty or stale when the Hub has not been read. A missing rollback would
make `CurrentAt` return the build before it and the route would answer `ok` with the M29 defect.
So the route reads From, or answers `From ""`, **only** when one of these holds; otherwise it
answers `previous_unknown` before any git read:
- (a) `ReleasesState` is `no_hub` or `no_hub_project`. No cockpit rollback can exist, because
  task 10 refuses a rollback without a Hub project (`no_hub_project`). A rollback made in
  Panacea's portal is declared invisible (task 10, *What this task does NOT claim*; plan 7P-4).
- (b) `ReleasesAt` is non-zero and not before `cut` (the Hub was read after this row went live,
  so a rollback confirmed before `cut` is in that read; this compares the Hub's and Panacea's
  instants with the local read time, the same comparison task 08 rule 5b (c) makes), **and**
  the page covers `cut`:
  - either the page is complete (`!ReleasesMore`);
  - or the page's oldest release was reserved at least one reservation lifetime before the
    `since` that `CurrentAt` found. The lifetime is that release's `expiresAt − reservedAt`
    (Hub contract §7.9: `expiresAt = reservedAt + PANACEA_APP_HUB_RESERVATION_TTL`). The Hub
    lists newest `reservedAt` first, so a release off the page was reserved no later than the
    page's oldest. Its confirm came before its own `expiresAt`, so before that `since`, and it
    loses to what `CurrentAt` found. A `current.source "none"` with `ReleasesMore` never meets
    this, because there is no `since` to compare.
  The review's version of this clause, "the page's oldest release is older than `cut`", is not
  enough on its own. A rollback reserved just before the page's oldest can confirm up to one
  lifetime later, which can still be before `cut` (Hub contract §7.9, and newest `reservedAt`
  first at `GET /v1/projects/{projectId}/releases`).
- `signed_out`, `hub_key_spelling`, `unavailable` with a `ReleasesAt` before `cut`, and a zero
  `ReleasesState` all fail (b), so they answer `previous_unknown`. This matches rule 5b's list of
  states that are not loaded.

A failed build is never current, so it is never a From (its commit never served). A running
row's range is **provisional**: `cut` is its `CreatedAt` until it goes live, and then becomes its
`CompletedAt`, so a rollback that lands in between moves From. A range is fixed once the row is
`succeeded` and a Hub read at or after its `cut` has been taken, which is condition (b).

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
  state: "ok" | "commit_not_local" | "previous_not_local" | "previous_unknown";
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
| `from == ""` | under the list: *"No earlier release is recorded on this environment: its whole history, to 200 commits."* (not *"the first release"*: Panacea may have pruned an older one, and the cockpit cannot tell) |
| not `ok` | `message` verbatim, and **Look again** |
| the row's `build` is not `succeeded` | above the list: *"Provisional: this build has not gone live, so the release before it may still change."* |
| always | *"Commits between the release that was serving on this environment before it and `<short commit>`."* (review M29: a rollback's target counts) |

The block reads once when the drawer opens on the row, and once more when that row's `build`
turns `succeeded` while the drawer is open. It never reads on the cursor or on a timer. A range
is fixed only once the row has gone live **and** a Hub read at or after its go-live has been
taken (condition (b) above). Until then the route answers either a provisional range (a running
row) or `previous_unknown`, whose **Look again** re-reads (plan re-review b11-1, m-4).

#### Tests to write first

Every route test's snapshot has `ReleasesState "ok"`, a `ReleasesAt` after the latest `cut` it
reads, and `ReleasesMore false`, unless its row says otherwise, so condition (b) holds by default.

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
| `TestPreviousSkipsFailed` (route) | history: run d1 succeeded (c1, completed 09:04), run d2 failed (c2), run d3 succeeded (c3, completed 10:14) → the read for d3 has `From c1` | a range starting at a commit that never served |
| `TestPreviousIsWhatServedAfterRollback` (route) | d1 (c1, completed 09:04), d2 (c2, completed 09:30), a rollback to d1 confirmed 10:05, d3 (c3, completed 10:14) → the read for d3 has `From c1`, not `c2`; the read for d2 has `From c1` (review M29) | *"the newest older succeeded run"*, which skips the rollback and leaves the c1..c2 sessions d3 re-shipped uncredited |
| `TestPreviousUsesGoLiveNotCreation` (route) | d1 (c1, completed 09:04), d2 (c2, completed 09:30), d3 created 10:00, a rollback to d1 confirmed 10:05, d3 completed 10:14 → `From c1` (the rollback's target was serving when d3 went live); with the rollback confirmed at exactly 10:14 → `From c2` (rule 6 orders a tie after the build, so the rollback did not serve before d3) | cutting at `CreatedAt`, which takes d2 because the rollback came after d3 was created |
| `TestPreviousUnknownNeverWidens` (route) | a rollback serving before d3 whose target `d0` is not in the snapshot → `state "previous_unknown"`, its sentence, `sessions []`, and the `gitxtest.Script` saw no call; the same for an environment whose older rows are all `historical` | a fallback to an older row or to the whole history, crediting sessions an earlier release shipped |
| `TestPreviousUnknownWhenRollbacksUnread` (route) | the `TestPreviousIsWhatServedAfterRollback` history, read for d3 (cut 10:14), with the rollback **absent** from `Releases` and `ReleasesState "signed_out"` → `previous_unknown`, and the `gitxtest.Script` saw no call. The same for a zero `ReleasesState`, and for `"unavailable"` with `ReleasesAt` 10:00. With the rollback present, `ReleasesState "ok"`, `ReleasesAt` 10:20 and `ReleasesMore true`: the page's oldest release reserved 09:50 with `expiresAt` 10:05 → `From c1`; reserved 09:55 with `expiresAt` 10:10 → `previous_unknown` (plan re-review b11-1, I-1) | the route trusting an unread or partial `Releases`, which misses the rollback, takes d2 and answers `ok` with From c2 |
| `TestNoHubNeedsNoReleases` (route) | d1 (c1), d2 (c2), d3 (c3) with `ReleasesState "no_hub"` and `ReleasesAt` zero → d3 `From c2`, one `git log` | a rule so strict that a project with no Hub never gets a range |
| `TestFirstReleaseFromEmpty` (route) | one row d1 succeeded, `ReleasesState "ok"`, `ReleasesAt` after its completion → `From ""`; d1 failed (created 09:00), d2 succeeded (completed 09:20) → d2 `From ""`; d0 `historical`, then d1 succeeded → d1 `previous_unknown` (plan re-review b11-1, I-2) | the route's `From ""` branch replaced by `previous_unknown` whenever `current` is `none`, or the row counting itself, or a failed row blocking it |
| `TestPreviousIsSameEnvironment` (route) | an older run on another environment is ignored | two environments' histories spliced into one range |
| `TestSessionsRouteIsReadOnly` | the route is in `catalogReadOnlyRoutes`; the §10a audit passes | a GET registered outside the recorder |
| `reads once` (desktop) | drawer open → one `api.sessions`; three cursor changes → still one. The same row re-rendered with `build "succeeded"` → exactly two reads, and a running row shows the *"Provisional: …"* line (m-4) | a git log per cursor tick, or a range read before go-live that is never read again |
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
  After a rollback, `From` can be older than a build that served in between: d3 after a rollback
  from d2 to d1 reads `c1..c3`, which lists d2's commits again. That is the point of review M29:
  those sessions reach people again with d3, so d3 credits them, and so does d2.
- That what served before a row is exact to the second. `CompletedAt` can lag go-live (task 08's
  *Does NOT claim*), so a rollback confirmed inside that lag can be ordered either side of it.
