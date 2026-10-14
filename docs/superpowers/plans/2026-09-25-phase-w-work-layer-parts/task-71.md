### Task 71: shipped: issues in the release manifest; *shipped to production, 14:02*

**Slice:** W1b. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WD2 with
phase 7's files. **Depends on** task 22 (the `landed` move and `IssueHead.landCommit`), task 64
(the Desk's LANDED section), task 67 (the issue chip), and phase 7 tasks **17** (the release
drawer) and **19** (sessions from trailers) on the sync base. A delta on phase 7 (rule H2:
phase 7's files win, and this task waits for P7-17 and P7-19). **Seams:** SW.9 (the trailer, read
here), SW.17 (one Desk row kind).

**Shipped is derived, never a state and never stored on the issue** (05 §3.3; rule 3). The
release manifest lists the issues whose `Panacea-Issue` trailers sit in the release's shipped
range, read from the local clone the way phase 7 task 19 reads `Panacea-Session`. The issue view
and the Desk read *"shipped to production, 14:02"* from that, and nothing is written to the Hub.

**Anchors, re-read on 2026-09-26** (phase 7 at `.claude/worktrees/phase-7`, `40219eee`):
P7 `task-19.md:29-92` (`sessions.Range`, `Result`, `Reader.Read` and its three git commands),
`:108-171` (the route's *previous release* rule, `deploywatch.CurrentAt`), `:175-192` (the
`{P}/sessions` wire and its catalog line); P7 `task-12.md:52` (`{P}` is
`/v1/projects/{projectId}/deploy`); P7 `task-17.md:67-76` (`ReleaseDrawer` and its `sessions`
slot); P7 `task-13.md:68-89` (`EnvironmentView.primary`, `current`), `:94-109` (`ReleasePin`:
`state`, `briefKey`, `confirmedAt`), `:111-138` (`ReleaseRow`); the P7 plan's file map
(`2026-09-23-phase-7-deploy.md:588-601`). This branch's parts: task 21's `gitx.TrailerIssue` and
`ValidIssueTrailer` (`task-21.md:81-104`; land stamps one trailer per distinct issue on the land
commit, `:268-284`); task 03's `IssueHead.LandCommit` (`task-03.md:290`); task 65's
`registerDeskSource` and `registerDeskRow` idiom (`task-65.md:784-794`); the prototype's RELEASE
fact row (`prototype/work-layer.html:687-689`). **UNVERIFIED until merge:** every phase 7 name
(none is code on this branch), and tasks 62 and 64's snapshot members (no part on the tree).

#### Decisions made here, so nobody re-derives them

1. **One walk, two trailers.** `sessions.Reader`'s private `walk` runs `Read`'s three git
   commands once with both trailers in the format, and returns each commit's session ids and
   issue ids. `Read` (P7-19) keeps its wire byte for byte; `ReadIssues` is new and answers
   `IssuesResult`. A route reads once; the two blocks of one drawer read twice, bounded by
   `MaxCommits`.
2. **Two routes, not one.** `GET {P}/issues?deploymentId=` is the manifest block, beside
   `{P}/sessions`, with the same range rule (the one function, not a copy). `GET
   {P}/issue-shipped?issueId=&landCommit=` is the issue's own derivation: per environment, the
   earliest confirmed release whose manifest lists the issue. The scope names one route; the
   issue view cannot read a manifest per release per environment, so the second is recorded
   under *Corrections*.
3. **"Lists it" is decided by ancestry, and the two routes are tested to agree.** Task 21 stamps
   the issue's trailer on the land commit, so a release's range `(From, Commit]` holds a commit
   carrying it exactly when `landCommit` is an ancestor of `Commit` and not of `From`. The
   per-issue route checks `git merge-base --is-ancestor` per release row, and a test holds that
   the release it names is one whose `ReadIssues` lists the issue.
4. **Only a confirmed release counts.** A row counts when its build is `succeeded` and its
   `release.state` is `confirmed`; the time is the pin's `confirmedAt` (the Hub's confirm), shown
   `HH:MM` in mono. A reserved, abandoned or expired pin, or `releaseLoaded: false`, never reads as
   shipped.
5. **The environment is named, never assumed.** The line uses the environment's `name` verbatim.
   The head shows the `primary` environment's fact first, then the others by name. A release to
   `staging` reads *"shipped to staging"*, never *"production"*.
6. **A rollback is said.** When an environment's current release does not hold `landCommit`, the
   fact reads *"was on production from 14:02; not serving now"*. It is never dropped to
   *"not shipped"*, which would erase a release that happened.
7. **Ids only, never titles** (05 §3.6). The manifest carries `issueId` and commit shas; the
   drawer resolves each id to its alias and chip from the replica (task 67), and an id the
   replica cannot read shows as the bare id in mono.

**Files:**

| Path | Change |
|---|---|
| `control-plane/internal/deploy/sessions/sessions.go` ⚑ (P7-19) | `walk` shared by `Read` and the new `ReadIssues`; `ReleaseIssue`, `IssuesResult`; `Contains` |
| `control-plane/internal/deploy/sessions/issues_test.go` | new: over `gitxtest.Repo`, beside P7-19's `sessions_test.go` |
| `control-plane/internal/api/deploy_drill.go` ⚑ (P7-12, P7-19) | `GET {P}/issues`, `GET {P}/issue-shipped` |
| `control-plane/internal/api/deploy_drill_test.go` ⚑ | the two routes' rows |
| `control-plane/internal/api/routes_audit_test.go` ⚑ | two `catalogReadOnlyRoutes` lines |
| `desktop/src/deploy/types.ts`, `api.ts` ⚑ (P7-13, P7-19) | `ReleaseIssues`, `IssueShipped`; `DeployApi.issues`, `DeployApi.issueShipped` |
| `desktop/src/deploy/ReleaseIssues.tsx` (+ test) | new: the drawer's issues block |
| `desktop/src/deploy/ReleaseDrawer.tsx` ⚑ (P7-17), `DeployLens.tsx` ⚑ | an `issues` slot beside `sessions`; the lens passes `<ReleaseIssues>` |
| `desktop/src/work/shipped.ts` (+ test) | new: `shippedFact`, the words, shared by the issue view and the Desk |
| `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) | the RELEASE fact row reads `shippedFact` |
| `desktop/src/components/desk/deskRows.tsx` ⚑ (SW.17) | task 71's block: kind `issue.shipped` in LANDED |

#### Interfaces — complete: `sessions.go`, the delta

```go
// ReleaseIssue is one Hub issue a release shipped: its id and the commits in the range that
// carry its Panacea-Issue trailer, newest first. Ids only: the manifest never holds a title.
type ReleaseIssue struct {
	IssueID string   `json:"issueId"` // gitx.ValidIssueTrailer
	Commits []string `json:"commits"` // 40 hex each; never empty
}

// IssuesResult is Result's shape with Issues in place of Sessions. State, Message, From,
// Commit, Commits and Truncated mean exactly what they mean in Result.
type IssuesResult struct {
	State     string         `json:"state"`
	Message   string         `json:"message"`
	From      string         `json:"from"`
	Commit    string         `json:"commit"`
	Commits   int            `json:"commits"`
	Truncated bool           `json:"truncated"`
	Issues    []ReleaseIssue `json:"issues"`  // by first appearance, newest first; [] never null
	Outside   int            `json:"outside"` // commits in the range with no Panacea-Issue trailer
}

// commitTrailers is one commit of a walk: its sha and each trailer's valid values.
type commitTrailers struct {
	SHA      string
	Sessions []string
	Issues   []string // gitx.ValidIssueTrailer only; an invalid value is dropped, never repaired
}

// walk runs Read's three git commands once, with both trailers in the format:
//   --format=%H%x00<TrailerFormat(TrailerSession)>%x00<TrailerFormat(TrailerIssue)>%x1e
// Its answer is Result's State and Message rules, the commits newest first, and Truncated.
func (rd *Reader) walk(ctx context.Context, r Range) (state, message string, commits []commitTrailers, truncated bool, err error)

// Read is P7-19's, rebuilt on walk. Its wire is unchanged byte for byte.
func (rd *Reader) Read(ctx context.Context, r Range) (Result, error)

// ReadIssues lists the Hub issues in r, from the same walk.
func (rd *Reader) ReadIssues(ctx context.Context, r Range) (IssuesResult, error)

// Contains reports whether commit landCommit is reachable from commit (or is it):
//   git merge-base --is-ancestor <landCommit> <commit>   exit 0 yes, 1 no, else an error.
// Class Read, Scope Project, no optional locks, as every call here.
func (rd *Reader) Contains(ctx context.Context, dir, landCommit, commit string) (bool, error)
```

`ReleaseIssue.Commits` never lists a sha twice; a commit with two issue trailers appears under
both. `Outside` counts only this trailer: a commit with a session and no issue is outside here.

#### Control-plane routes — complete wire

```
GET {P}/issues?deploymentId=engagement-portal-build-x7k2p
  200 IssuesResult, e.g.
      { "state": "ok", "message": "", "from": "<40 hex>", "commit": "<40 hex>",
        "commits": 7, "truncated": false, "outside": 3,
        "issues": [ { "issueId": "is-3f2a91c07b1e4d2a",
                      "commits": ["<40 hex, the land commit>", "<40 hex>", "<40 hex>"] },
                    { "issueId": "is-0123456789abcdef", "commits": ["<40 hex>"] } ] }
  400, 404, 409, 503   exactly {P}/sessions's (P7 task-19.md:184-188), from the same code path

GET {P}/issue-shipped?issueId=is-3f2a91c07b1e4d2a&landCommit=<40 hex>
  200 { "issueId": "is-3f2a91c07b1e4d2a", "landCommit": "<40 hex>",
        "environments": [
          { "environment": "production", "primary": true, "deploymentId": "…-x7k2p",
            "releaseId": "rl-…", "version": "2026.09.26-1", "commit": "<40 hex>",
            "confirmedAt": "2026-09-26T04:02:11Z", "briefKey": "brief:bf-…@6",
            "serving": true } ],
        "unread": [] }
  400 bad_request           issueId not gitx.ValidIssueTrailer, or landCommit not 40 hex
  409 deploy_precondition   details.reason "commit_not_local": this clone lacks landCommit
  503 upstream_unavailable  the watcher has no snapshot yet
```

**The per-issue rule.** For each environment of the view (the view's own join, `environmentRow`,
as P7-19's rule uses), take its run rows whose `build` is `succeeded` and whose `release.state` is
`confirmed`, oldest go-live first (P7-19's `cut`). The first row whose `commit` `Contains`
`landCommit` is the release that shipped it: its `deploymentId`, `releaseId`, `version`, `commit`,
`confirmedAt` and `briefKey` (the pin's opaque key, never parsed, P5) fill the entry. `serving`
is whether the environment's current commit (`current`, resolved as P7-19 resolves From) also
`Contains` it. An environment with no such row has no entry. An environment with a row whose
`releaseLoaded` is `false` is named in `unread` instead, and the desktop says *"release not read
yet"* for it, never *"not shipped"*. `environments` is `primary` first, then by name; `[]` never
null. At most one `merge-base` per candidate row, 200 rows per environment (P7's history cap).

**Why the two routes agree.** `landCommit` carries the issue's trailer (task 21). For the row the
rule picks, the previous serving commit does not contain `landCommit` (else an older row would
have been picked, or the serving lineage came from a rollback, which the rule's *current* check
covers), so the land commit lies in `(From, Commit]` and `ReadIssues` lists the issue.
`TestShippedAgreesWithTheManifest` holds it on a real repository.

`catalogReadOnlyRoutes` lines: *"Reads Panacea-Issue trailers from the local clone with git log
(Class Read, no optional locks); writes nothing."* and *"Checks commit ancestry in the local clone
with git merge-base (Class Read); writes nothing."*

#### Desktop — contract

```ts
// deploy/types.ts: mirrors sessions.IssuesResult and the issue-shipped answer field for field.
export interface ReleaseIssues {
  state: "ok" | "commit_not_local" | "previous_not_local" | "previous_unknown";
  message: string;
  from: string;
  commit: string;
  commits: number;
  truncated: boolean;
  outside: number;
  issues: Array<{ issueId: string; commits: string[] }>;
}
export interface ShippedOn {
  environment: string; primary: boolean; deploymentId: string; releaseId: string;
  version: string | null; commit: string; confirmedAt: string; briefKey: string | null; serving: boolean;
}
export interface IssueShipped { issueId: string; landCommit: string; environments: ShippedOn[]; unread: string[] }

// deploy/api.ts: DeployApi gains
//   issues(projectId: string, deploymentId: string): Promise<ReleaseIssues>;
//   issueShipped(projectId: string, issueId: string, landCommit: string): Promise<IssueShipped>;

// deploy/ReleaseIssues.tsx
export function ReleaseIssues(props: { projectId: string; row: ReleaseRow }): JSX.Element | null;
// null when row.deploymentId is null, as ReleaseSessions.

// deploy/ReleaseDrawer.tsx ⚑: one more optional slot, drawn under sessions.
//   issues?: React.ReactNode;   // absent → no issues block at all
```

**The drawer block's words** (heading *"ISSUES IT SHIPPED"*, beside P7-19's *"SESSIONS THAT
PRODUCED IT"*):

| Case | Drawn |
|---|---|
| an issue the replica can read | its alias in mono and task 67's chip; *"`N` commits"*; a click opens the issue view |
| an id the replica cannot read | the id in mono and *"an issue you cannot open here"*; no click, no title |
| `outside > 0` | *"`N` commits carry no issue"*, never a warning colour |
| `issues` empty | only the line above, never *"no issues"* as if that were a finding |
| not `ok` | `message` verbatim, and **Look again** (P7-19's) |

It reads once when the drawer opens on the row, and once more when that row's build turns
`succeeded` while open (P7-19's rule), never on the cursor or a timer.

#### Interfaces — complete: `desktop/src/work/shipped.ts`

```ts
// shipped.ts: the one place "shipped" becomes words. Shipped is derived from the release
// manifest at read time and never stored on the issue (05 §3.3); nothing here writes.
import type { IssueShipped, ShippedOn } from "../deploy/types";

export type ShippedFact =
  | { kind: "not_landed" }
  | { kind: "not_read" }                                   // the route has not answered yet
  | { kind: "landed_not_shipped"; unread: string[] }
  | { kind: "shipped"; on: ShippedOn; others: ShippedOn[] };

export interface FactWords { value: string; foot: string }

export function shippedFact(landCommit: string, s: IssueShipped | null): ShippedFact {
  if (!/^[0-9a-f]{40}$/.test(landCommit)) return { kind: "not_landed" };
  if (!s || s.landCommit !== landCommit) return { kind: "not_read" };
  if (s.environments.length === 0) return { kind: "landed_not_shipped", unread: s.unread };
  const [on, ...others] = s.environments;                  // the server's order: primary first
  return { kind: "shipped", on, others };
}

/** The RELEASE fact row (prototype/work-layer.html:687-689). time is HH:MM, set in mono. */
export function shippedWords(f: ShippedFact, alias: string, time: (iso: string) => string): FactWords {
  switch (f.kind) {
    case "not_landed":
      return { value: "Not shipped", foot: "derived from the release manifest, never from moving a card" };
    case "not_read":
      return { value: "Landed", foot: "this project's releases are not read yet" };
    case "landed_not_shipped":
      return { value: "Landed, not shipped yet", foot: "the next release will list it; shipped is derived, never set"
        + (f.unread.length ? ` · not read yet: ${f.unread.join(", ")}` : "") };
    case "shipped": {
      const at = time(f.on.confirmedAt);
      const value = f.on.serving ? `Shipped to ${f.on.environment}, ${at}` : `Was on ${f.on.environment} from ${at}; not serving now`;
      const pin = f.on.briefKey ? `pinned ${f.on.briefKey}` : "no brief pinned";
      const also = f.others.map((o) => ` · ${o.serving ? "also" : "was on"} ${o.environment} ${time(o.confirmedAt)}`).join("");
      return { value, foot: `${pin} · manifest lists ${alias}${f.on.version ? ` · release ${f.on.version}` : ""}${also}` };
    }
  }
}

/** The Desk's LANDED sentence, 05 §5.3's W1b frame: "EP-142 shipped to production at 14:02." */
export function shippedSentence(alias: string, on: ShippedOn, time: (iso: string) => string): string {
  return `${alias} shipped to ${on.environment} at ${time(on.confirmedAt)}.`;
}
```

`IssueHead.tsx` ⚑ (task 46): the RELEASE row calls `api.deploy.issueShipped(projectId, id,
head.landCommit)` once when the view opens on a head with a `landCommit`, and once more when
`landCommit` changes; the answer lives in the component, never in the replica or the store. The
row renders `shippedWords(shippedFact(head.landCommit, answer), head.alias, deskTime)`.

#### Interfaces — complete: `deskRows.tsx` ⚑ — task 71's block

SW.17's names are the scope's (`tasks-61-80.md:73-82`: `DeskItem<T>` with `rankReason` and
`ifNothing`, `DeskRank`, `registerDeskRow`, `registerDeskSource`). The row joins LANDED; it never
replaces task 64's landed row, which is the *proven* fact, while this one is the *shipped* fact.

```tsx
// Appended after task 65's block (task 62 owns the file's head and imports).
import { shippedSentence } from "../../work/shipped";
import type { ShippedOn } from "../../deploy/types";

export interface ShippedPayload { issueId: string; alias: string; on: ShippedOn }

registerDeskSource("issue.shipped", (s) => shippedItems(s));
registerDeskRow<ShippedPayload>("issue.shipped", "landed",
  (item) => (
    <p className="desk-fact">
      {shippedSentence(item.payload.alias, item.payload.on, deskTime)}{" "}
      <span className="mono">{item.payload.on.briefKey ? `pinned ${item.payload.on.briefKey}` : "no brief pinned"}</span>
    </p>
  ),
  () => ({ tier: 2, waitingSinceMs: 0 }));                  // a fact, never urgent

/** One row per issue of mine that a confirmed release shipped and still serves. s.shipped is
 *  read once at Desk entry, for my landed and done issues with a landCommit (task 62's load
 *  hook, gap 2); a teammate's issue never makes a row on my Desk. */
function shippedItems(s: DeskSnapshot): DeskItem<ShippedPayload>[] {
  const out: DeskItem<ShippedPayload>[] = [];
  for (const [issueId, answer] of s.shipped) {
    const head = headOf(issueId);
    if (!head || head.assignee?.id !== s.me.id || answer.landCommit !== head.landCommit) continue;
    const on = answer.environments.find((e) => e.serving);
    if (!on) continue;
    out.push({ kind: "issue.shipped", id: `issue.shipped:${issueId}:${on.releaseId}`,
      payload: { issueId, alias: head.alias, on }, arrivedAt: on.confirmedAt,
      rankReason: "shipped", ifNothing: "" });
  }
  return out;
}
```

The id includes the release, so a second release to another environment is a second row on the
next entry, and the same release never makes two. `headOf`, `deskTime` and `DeskSnapshot` are
task 62's (**UNVERIFIED** names, as task 65 records at `task-65.md:813-815`); `s.shipped` is this
task's addition to the snapshot (`Map<string, IssueShipped>`), loaded by the hook of gap 2.

#### Tests to write first

Go rows run on P3-03's `gitxtest.Repo` (real git) unless they say `gitxtest.Script`; route rows use
P7-19's snapshot defaults (`ReleasesState "ok"`, `ReleasesAt` after the latest cut). Desktop rows
run in vitest with jsdom. Each row fails on the mutant beside it.

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestTwoCommitsOneIssue` | c2 and c3 both carry `Panacea-Issue: is-3f2a91c07b1e4d2a`; From c1, Commit c3 → one `ReleaseIssue` with `commits [c3, c2]` | one row per commit (the scope's test 1) |
| `TestMentionIsNotATrailer` | c2's body reads *"follow-up to is-3f2a91c07b1e4d2a"* on a line mid-body, with no trailer block → `issues []`, `outside 1` | a substring scan in place of SW.9's trailer grammar (scope test 2) |
| `TestInvalidTrailerDropped` | `Panacea-Issue: EP-142` and `Panacea-Issue: is-XYZ` → neither listed; both commits outside | a value repaired or trusted without `gitx.ValidIssueTrailer` |
| `TestTwoIssuesOneCommit` | one commit with two issue trailers → two entries, the commit under each | the second value of a multi-valued trailer dropped |
| `TestSessionWithoutIssueIsOutside` | c2 carries only `Panacea-Session` → `ReadIssues` counts it in `outside`; `Read` lists its session | the two trailers' outside counts shared |
| `TestReadWireUnchanged` | P7-19's `sessions_test.go` rows, run unedited after `walk` → all pass; `Read`'s JSON for the `TestRangeIsExclusiveInclusive` repo equals a golden captured before the change | `Read` rebuilt on `walk` with its wire drifted |
| `TestOneWalkBothTrailers` (`gitxtest.Script`) | `ReadIssues` and `Read` each issue exactly one `git log` whose `--format` holds both `TrailerFormat` terms; every call `Class Read`, `Scope Project` | a second log per trailer, or a lock-taking call racing an agent's git |
| `TestIssuesRefusalsAreSessions` | commit absent → `commit_not_local`, From absent → `previous_not_local`, the unknown-previous history → `previous_unknown`; `git log` never run in all three | a refusal copied and drifted from `{P}/sessions` |
| `TestContainsAncestry` | c1 ← c2 ← c3 and a side commit s1 off c1: `Contains(c2, c3)` true, `Contains(c3, c3)` true, `Contains(s1, c3)` false; exit 128 → an error, never false | a missing commit read as *"not shipped"* |
| `TestIssuesJSONHasNoTitle` (route) | the `{P}/issues` answer decoded with `DisallowUnknownFields` into `IssuesResult`; no key in it matches `(?i)title|name|description` | titles copied into the manifest (scope test 3) |
| `TestShippedOnlyWhenConfirmed` (route) | production d1 (c1), d2 (c2 holds landCommit, `release.state "reserved"`) → `environments []`; the same d2 `confirmed` → one entry, `releaseId` and `confirmedAt` the pin's | shipped on `landed`, or on a reserved pin (scope test 4) |
| `TestStagingNeverReadsProduction` (route) | staging d1 confirmed with landCommit 09:10, production d4 confirmed 14:02 → `environments` is production (primary, 14:02) then staging (09:10); with no production row → staging only, and `primary false` | the first environment in time wins, or the primary assumed |
| `TestEarliestConfirmedRelease` (route) | production d2 (c2 ∋ land) 11:00 and d3 (c3 ∋ land) 14:02 → the entry is d2 at 11:00 | the newest release credited, moving the ship time |
| `TestRollbackIsSaid` (route) | d2 ships land at 11:00, a rollback to d1 confirmed 12:00 → the entry is d2 with `serving false` | a rolled-back release dropped to *"not shipped"* |
| `TestUnreadIsNamed` (route) | staging's row with `releaseLoaded false` → staging in `unread`, never in `environments` | an unread release drawn as *"not shipped"* |
| `TestShippedAgreesWithTheManifest` (route + real git) | for every entry the route returns over a four-release history with a rollback, `ReadIssues` on that entry's `deploymentId` lists the issue | the per-issue rule and the manifest drifting apart |
| `TestIssueShippedBadInput` (route) | `issueId=EP-142` → 400; `landCommit=abc` → 400; landCommit absent from the clone → 409 `commit_not_local` | a bad id reaching git, or a missing commit answered as `[]` |
| `TestDrillRoutesReadOnly` | both routes are in `catalogReadOnlyRoutes`; the §10a audit passes | a GET registered outside the recorder |
| `TestShippedWritesNothingToTheHub` (route) | both routes run with a `hubclient.NewFake()` wrapped in a write recorder → zero write calls; the issue's head re-read after, marshalled, has exactly task 03's key set, no `shipped`, `shippedAt` or `release` | the fact stored on the head (scope test 5) |

Desktop rows (`shipped.test.ts`, `ReleaseIssues.test.tsx`, `IssueHead.test.tsx` ⚑, `deskRows.test.tsx` ⚑):

| Test | Assertion | Mutant it kills |
|---|---|---|
| `shippedFact: landed is not shipped` | `landCommit` 40 hex, answer with `environments []` → `landed_not_shipped`; words *"Landed, not shipped yet"* (`prototype/work-layer.html:687`) | shipped read from `landed` (scope test 4) |
| `shippedFact: a stale answer is not read` | answer for another `landCommit` → `not_read`, *"Landed"* | an answer for a previous land shown after a re-land |
| `shippedWords: staging says staging` | `on.environment "staging"` → value *"Shipped to staging, 09:10"*; the string `production` appears nowhere in value or foot | the environment assumed |
| `shippedWords: the pin is named` | `briefKey "brief:bf-7c1e@6"` → foot starts *"pinned brief:bf-7c1e@6"*; null → *"no brief pinned"* | the brief key dropped or parsed |
| `shippedWords: rollback` | `serving false` → *"Was on production from 14:02; not serving now"* | a rollback dropped to *"not shipped"* |
| `ReleaseIssues: ids resolve, titles never sent` | the mocked `api.issues` answer has two ids, one in the replica → alias `EP-142` in mono and the chip; the other → the id in mono and *"an issue you cannot open here"*; no text the answer or replica did not carry | a title fetched from the Hub per id, or an unreadable id dropped |
| `ReleaseIssues: outside is said once` | `outside 3`, `issues []` → *"3 commits carry no issue"*; no element reads *"no issues"* | an empty list presented as a finding |
| `ReleaseIssues: reads once` | drawer open → one `api.issues`; three cursor changes → one; the row re-rendered with `build "succeeded"` → two | a read per cursor tick, or never read after go-live |
| `IssueHead: one read per land` | head with `landCommit` → one `api.deploy.issueShipped`; re-rendered → still one; `landCommit` changed → two; no `landCommit` → none | a read per render, or a stale answer kept |
| `IssueHead: nothing written` | after the fact row renders, the Hub client mock saw no write method, and the replica's op log is empty | the derived fact written back as a Hub field or op |
| `Desk: shipped appears on the next entry` | Desk open, snapshot taken; `s.shipped` gains an answer → no new row while open; leave and re-enter → one `issue.shipped` row in LANDED reading *"EP-142 shipped to production at 14:02."* | a live row that moves under the cursor (scope test 6, task 62's rule) |
| `Desk: one row per release, mine only` | the same release twice → one row; a second environment's release → two; a teammate's issue → none; `answer.landCommit` ≠ the head's → none | duplicate rows, a teammate's work on my Desk, or a stale land credited |
| `no prediction` | `expectNoPrediction(container)` (task 20, `desktop/src/test/expectNoPrediction.ts`) on the drawer block, the RELEASE fact row and the Desk row | *"ships Thursday"* or *"likely in the next release"* (scope test 7, OD-27) |

`TestReadWireUnchanged`'s golden is captured on the sync base **before** `walk` lands (step 2), so
it records P7-19's wire, not this task's.

#### Steps

1. **Sync and check.** Sync-merge the base that holds P7-17 and P7-19 (rule H2). Confirm
   `sessions.Reader`, `Range`, `Result`, the `{P}/sessions` handler, `ReleaseDrawer`'s `sessions`
   slot, `ReleaseRow` and `ReleasePin` match the anchors above, and that task 21's
   `gitx.TrailerIssue` and task 03's `IssueHead.LandCommit` are merged. P7-19 absent → this task
   does not start (rule H2). Correct any name here that differs before typing it.
2. **Golden first.** On the unchanged base, capture `Read`'s JSON for
   `TestRangeIsExclusiveInclusive`'s repository into `sessions/testdata/read_golden.json`; commit
   it alone.
3. **Red, Go.** Write `issues_test.go` and the route rows in `deploy_drill_test.go`. Run the Go
   test command below; expect a build failure naming `ReadIssues`, `ReleaseIssue` and `Contains`.
4. **Green, Go.** `walk`, `Read` on `walk`, `ReadIssues`, `Contains`; the two handlers beside
   `{P}/sessions`, sharing its range function; the two `catalogReadOnlyRoutes` lines.
5. **Red, desktop.** The four test files; run vitest; expect failures on the missing modules.
6. **Green, desktop.** `types.ts`, `api.ts`, `shipped.ts`, `ReleaseIssues.tsx`, the drawer slot and
   the lens line, the RELEASE row in `IssueHead.tsx`, task 71's block in `deskRows.tsx`, and the
   `s.shipped` load in task 62's entry hook (gap 2).
7. **Run** every acceptance command; commit by pathspec, Go and desktop in two commits.

#### Acceptance

```bash
go build ./control-plane/... && go vet ./control-plane/internal/deploy/sessions/ ./control-plane/internal/api/
# (no output)
go test ./control-plane/internal/deploy/sessions/ ./control-plane/internal/api/ -race -count=1
# ok  .../deploy/sessions   ok  .../api
go test ./control-plane/internal/deploy/sessions/ -count=1 -v -run 'TestTwoCommitsOneIssue|TestMentionIsNotATrailer|TestReadWireUnchanged|TestOneWalkBothTrailers|TestContainsAncestry' 2>&1 | grep -c '^--- PASS'
# 5
go test ./control-plane/internal/api/ -count=1 -v -run 'TestShipped|TestStagingNeverReadsProduction|TestEarliestConfirmedRelease|TestRollbackIsSaid|TestUnreadIsNamed|TestIssuesJSONHasNoTitle|TestIssueShippedBadInput|TestDrillRoutesReadOnly' 2>&1 | grep -c '^--- PASS'
# 10   (TestShipped matches three: OnlyWhenConfirmed, AgreesWithTheManifest, WritesNothingToTheHub)
npm --prefix desktop run typecheck
# (no errors)
npm --prefix desktop test -- src/work/shipped src/deploy/ReleaseIssues src/components/issue/IssueHead src/components/desk/deskRows
# all pass
git grep -nE "shipped(At)?\"?\s*:" -- control-plane/internal/hub desktop/src/work/replica
# (no output: no shipped field on a Hub record or a replica op)
```

**Owner's eyes (batched with task 70's run):** land EP-142, ship the branch to production through
phase 7, open the release drawer and the issue; read *"ISSUES IT SHIPPED"*, *"Shipped to
production, 14:02"*, and the Desk's LANDED row on the next entry. **Unverified until then.**

#### Reconciliation with task 62's part (on the tree since this part began; task 62 wins)

Task 62 (`task-62.md:143-168`) fixed `DeskSnapshot`: `me` is the `u-` id string, `headOf` is the
snapshot's method, and a later task **adds** a `DeskInputs` member rather than a snapshot field
(`:62-63`). So `s.shipped` above is replaced by a signal, and the block's source reads:

```tsx
// deskRows.tsx ⚑ — task 71's source, as built (supersedes shippedItems above).
function shippedItems(s: DeskSnapshot): DeskItem<ShippedPayload>[] {
  const out: DeskItem<ShippedPayload>[] = [];
  if (s.me === null) return out;
  for (const sig of s.signals("issue.shipped")) {
    const answer = sig.data.answer as IssueShipped;          // shippedCache's value, below
    const head = s.headOf(answer.issueId);
    if (!head || head.assignee?.id !== s.me || answer.landCommit !== head.landCommit) continue;
    const on = answer.environments.find((e) => e.serving);
    if (!on) continue;
    out.push({ kind: "issue.shipped", id: `${answer.issueId}:${on.releaseId}`, issueId: answer.issueId,
      payload: { issueId: answer.issueId, alias: head.alias, on }, arrivedAt: on.confirmedAt,
      rankReason: "shipped", ifNothing: "" });
  }
  return out;
}
```

```ts
// desktop/src/work/shipped.ts — appended: the Desk's cache. Module memory only: never the
// replica, never the store, never storage (task 55's rule); lost on reload, re-read on entry.
import type { IssueHead } from "../api/work";                              // task 37
import type { DeskSignal } from "../components/desk/deskModel";           // task 62, type only
const shippedCache = new Map<string, IssueShipped>();   // by issue id

/** DeskInputs.signals feeds "issue.shipped" from this, read synchronously at entry. */
export function shippedSignals(): DeskSignal[] {
  return [...shippedCache.values()].map((answer) => ({ kind: "issue.shipped", id: answer.issueId,
    at: answer.environments[0]?.confirmedAt ?? "", data: { answer } }));
}

/** Called by Desk.tsx after the entry snapshot is taken. One read per my landed or done head
 *  with a landCommit; answers land in the cache and draw on the NEXT entry (task 62's rule). */
export async function refreshShipped(heads: readonly IssueHead[], me: string,
  read: (projectId: string, issueId: string, landCommit: string) => Promise<IssueShipped>): Promise<void> {
  for (const h of heads) {
    if (h.assignee?.id !== me || !/^[0-9a-f]{40}$/.test(h.landCommit)) continue;
    try { shippedCache.set(h.id, await read(h.projectId, h.id, h.landCommit)); }
    catch { /* a failed read keeps the last answer; the Desk never says "not shipped" for it */ }
  }
}
```

`Desk.tsx` ⚑ (task 62) passes `signals: (k) => (k === "issue.shipped" ? shippedSignals() : [])`
beside whatever other sources a later task has already added (composed, never replaced), and
calls `refreshShipped` once after `takeDeskSnapshot`. That is *gap 2*'s hook. The reads are
sequential, one per head, so a Desk with 30 landed heads makes 30 small reads per entry; this is
recorded, not measured.

#### What this task does NOT claim

- **That an issue with no trailer in the range was not shipped.** The trailer is task 21's, stamped
  at land. Work landed outside the product (an outside PR, a hand merge) carries none; the drawer
  counts it as *"N commits carry no issue"*, never as *"no issue"*. Outside PRs are W5.
- **Release notes by criterion, or freeze** (W4, task 131, L-148); **requester updates** (W5,
  L-140). This task serves both and closes neither.
- **That "shipped" is the time users saw it.** `confirmedAt` is the Hub's confirm of the release
  pin (P7-13's `ReleasePin`), which can differ from go-live by the lag P7-08 records.
- **Anything about a clone that has not fetched the release commit.** Both routes refuse with
  `commit_not_local` rather than guess.
- **Phase 7's names.** Every P7 identifier here is from its plan, not code on this branch
  (**UNVERIFIED until merge**); step 1 corrects them.
- **A number.** The 30-reads-per-entry figure is arithmetic, not a measurement (OD-27).

**Parity.** Closes **L-146's release half** (the manifest lists issues, and the issue and the Desk
read *shipped* from it); task 70 records L-146's W1 half (the trailers at land). Serves L-148 (W4,
task 131) and L-140 (W5).

#### Corrections to the scope paragraph (recorded; the skeleton row wins where they differ)

1. **Two routes, not one** (decision 2): `{P}/issues` is the scope's manifest route;
   `{P}/issue-shipped` is added so the issue view does not read every release's manifest.
2. **`s.shipped` is a signal** (reconciliation above), because task 62 fixed the snapshot.
3. **The scope's `"shipped to production, 14:02"`** is drawn as *"Shipped to production, 14:02"*
   on the fact row (a sentence start) and *"EP-142 shipped to production at 14:02."* on the Desk
   (05 §5.3's W1b frame).

#### Gaps found while writing this part

1. **L-146 is claimed by two tasks.** The skeleton's task 71 row says it closes L-146, and task
   70's part records a W1 half (`task-70.md`, gap 3). The coverage map should split L-146 into
   its trailer half (21-22, proven in 70) and its release half (71).
2. **Task 62's `Desk.tsx` has no documented way to compose `signals`** from several tasks (127,
   170, this one). Each part adds its own; whichever merges second must compose, not replace.
   Task 62's part should own a `signals` registry the way it owns `registerDeskSource`.
3. **`{P}/issue-shipped` is in no scope and no skeleton row.** Added here (decision 2).
4. **Phase 7 has no part for tests over `deploywatch` release rows with `releaseLoaded`** on this
   branch; `TestUnreadIsNamed` assumes P7-13's field name (**UNVERIFIED**).
5. **Task 64 has no part on the tree**, so the LANDED section's render contract is taken from task
   62's registry and task 65's idiom only.
