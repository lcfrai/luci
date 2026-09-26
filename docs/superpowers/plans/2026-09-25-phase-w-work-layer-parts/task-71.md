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
