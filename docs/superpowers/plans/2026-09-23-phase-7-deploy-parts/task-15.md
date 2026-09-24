### Task 15: Desktop — the environment card, the release table, the pin cell

**Half:** 7a. **Hours:** 3. **Depends on:** 14. **Elsewhere:** phase 4's `Ref` cache (ruling R6).

The heart of the lens: for the selected branch, **three facts that are never merged** — build,
serving, scans (plan §5 rule 9; survey §3.3: *a crash-looping app reports `Succeeded` while its URL
is down*) — and the release history, each row carrying **the brief it was shipped against, or saying
there is none** (P9). Nothing here fetches: everything comes from the `DeployView` in the slice
(task 13) and from phase 4's cache for the pin text.

**Read `docs/DESIGN-SYSTEM.md` first.** This task's non-negotiables, each checked by a test where
one can be: **one global heartbeat clock** — the only live element is `StatusDot` with `live`, whose
phase comes from `beatDelayStyle()` (`hooks/heartbeat.ts:23`; `StatusDot.tsx:2` makes it the only
caller), and elapsed time reads `useHeartbeat()` (`heartbeat.ts:64`), **never a timer of its own**;
**no spinner per row**; **`tabular-nums`** on every number, time and id (`--numeric`,
`tokens.css:156`); a changed fact **flashes once and decays over 600 ms**, opacity only
(`StatusDot.tsx`'s `FLASH_MS`); status is **colour plus shape** (prototype-fact-sheet §10); the table
**virtualises past 50 rows** (§*Floor*); hover changes background or ring, never layout.
**Visual acceptance is the owner's, batched (OD-12); unverified until then.**

**Files:**
- `desktop/src/deploy/EnvironmentCard.tsx` (new) — the three facts for the selected branch's environment
- `desktop/src/deploy/EnvironmentList.tsx` (new) — the project's other environments, one line each
- `desktop/src/deploy/ReleaseTable.tsx` (new) — history, windowed
- `desktop/src/deploy/PinCell.tsx` (new) — the seven provenance states
- `desktop/src/deploy/usePinRef.ts` (new) — the adapter over phase 4's cache
- `desktop/src/deploy/Banners.tsx` (new) — stale, collision
- `desktop/src/components/common/StatusDot.tsx`, `common.css` (modified) — the fact statuses below
- `desktop/src/styles/designsystem.test.ts` (modified) — the `--drift` guard, if phase 4 has not already amended it
- `*.test.tsx` beside each

#### Contracts — complete

```ts
// StatusDot.tsx — a declared amendment to phase 2's one status component.
/** Deploy's facts, in the shared vocabulary of prototype-fact-sheet.md §10. */
export type FactStatus = "ok" | "warn" | "drift" | "fail" | "unmeasured";
export type DotStatus = SessionStatus | MachineState | FactStatus;
// SHAPE gains: ok → "dot-disc", warn → "dot-triangle", drift → "dot-half",
//              fail → "dot-square", unmeasured → "dot-ring" (hollow, --ink-4)
```

```ts
// usePinRef.ts — the ONE place phase 7 touches phase 4's cache. If phase 4's selector is named or
// shaped differently, this file changes and PinCell does not.
export type PinRef =
  | { state: "resolving" }
  | { state: "unavailable"; message: string }
  | { state: "resolved"; revision: number; headRevision: number; headline: string }
  | { state: "missing" }
  | { state: "erased"; erasedAt: string };

/** Reads the cached resolution for an OPAQUE key. Never fetches (R6: the loader does that on the
 *  tick), never parses the key (P5). A key the cache has never seen is "resolving". */
export function usePinRef(briefKey: string | null): PinRef | null; // null ⇔ briefKey null
```

```tsx
// selected: view.selected, verbatim. selectedEnvironment: view.selected?.environment ?? null.
// No component takes the git branch as a comparable value (b02 round 2, R5): the only use a
// component could make of it is a comparison with a Panacea label. The branch reaches the DOM
// only as display text, read from selected.branch.
export function EnvironmentCard(props: { env: EnvironmentView | null; selected: DeployView["selected"]; stale: boolean; staleSince: string | null }): JSX.Element | null;
export function EnvironmentList(props: { environments: EnvironmentView[]; unmatched: EnvironmentView[]; selectedEnvironment: string | null }): JSX.Element;
export function ReleaseTable(props: { rows: ReleaseRow[]; selectedEnvironment: string | null; onOpen(row: ReleaseRow): void; stale: boolean }): JSX.Element;
export function PinCell(props: { row: ReleaseRow }): JSX.Element;
export function StaleBanner(props: { since: string | null; reason: { code: string; message: string } | null }): JSX.Element | null;
export function CollisionBanner(props: { env: EnvironmentView }): JSX.Element | null;
```

`EnvironmentList` marks the row whose `name === selectedEnvironment`, and `ReleaseTable` marks the
rows whose `environmentRow === selectedEnvironment`: both compare two names the view itself
returned (task 08 rules 1 and 1a), never a git branch with a label.

**Which environment is "this branch's".** The control plane's answer, never the desktop's:
`view.selected.environment` (task 08 rule 1a), then `environments.find(e => e.name ===
view.selected.environment)` — a lookup by the name the view itself returned, not a join. The
desktop **never** compares the git branch (`selected.branch`) with `e.branch`, `e.name` or
`branchesSeen`: Panacea's row label for `feat/x` is `feat-x` (`projects.go:2109-2118`), so any such
comparison finds nothing for a branch with `/`, `_`, capitals or more than 20 characters (review
b02, C2). Five states, checked in this order:
- `view.selected == null` → no branch is selected, so no `?branch=` was sent: the lens draws **no**
  environment card, only the list (b02 round 2, R6). Never the asking sentence, which would claim a
  request that was never made.
- `selected.error` set → *"Panacea did not name this branch's environment: `<error.message>`"* —
  Panacea's refusal, verbatim (task 03 rewrites the router's `404 page not found` on this route to
  the `/` sentence). The watcher retries it at the branch's recheck, once a minute (task 07, `BranchEnvErrs`); the
  cursor bump re-renders; no timer here (b02 round 2, R2).
- `selected.target == null` → the card's empty state reads *"Asking Panacea which environment
  this branch deploys to."* (the watcher resolves it on its next cycle, and the cursor bump
  re-renders; no timer here). With `error` checked first, this state means only *not asked yet*.
- `selected.target` set, `selected.environment == null` → *"Not deployed yet. Panacea will
  deploy this branch to `<target>`."* — Panacea's own name, shown before the first Ship.
- both set → the card for that row.

**Recorded against another environment** (task 08 rule 5a; b02 round 2, R1). A `ReleaseTable` row
with `environmentMismatch` set carries one `warn` line: *"The release was recorded against
`<recorded>`, but Panacea deployed it to `<deployed>`."* Both names verbatim, in mono. The row sits
under its `environmentRow` (the deployment's), never under `recorded`.

`unmatched` groups render in `EnvironmentList` under their raw names, never folded into a row. The
one group whose name is `""` (task 08 rule 1b: legacy runs with no environment label that no row's
branch label claims) is titled *"no environment recorded"*, never a blank title (review M5). Its
React key is not `""` alone (prefix it, `unmatched:`), and nothing tests it for truth: `""` is
falsy, so `name || …` or `if (name)` would drop the group.

#### The three facts — words (from `prototype/phase-7-deploy.html:617-661`, kept where true)

| Fact | Condition | Dot | Words |
|---|---|---|---|
| BUILD | `running` / `pending` | `running`, live | *"Building"* + elapsed from the tracked run's `startedAt` via `useHeartbeat()`; then `completedSteps` ✓ and `currentStep`; *"Steps after the current one are not listed: Panacea does not say what remains."* |
| BUILD | `failed` | `fail` | *"Failed at `<failedStep>`"* + `errorText` verbatim |
| BUILD | `succeeded` | `ok` | *"Built"* · version (or *"version unknown"*) · short commit |
| BUILD | `historical` / `unknown` / `none` | `unmeasured` | *"Reconstructed from Panacea's record"* / the raw word / *"Never built"* |
| SERVING | `measured:false` | `unmeasured` | *"Not measured"* — *"Panacea reported no pod count. That is not the same as serving."* |
| SERVING | `ready == 0` | `fail` | *"Down — 0 pods ready"* — *"The build passing does not mean the site is up."* |
| SERVING | `ready > 0` | `ok` | *"Serving — N pods ready"* + `replicas.desired` when present |
| SCANS | `not_read` | `unmeasured` | *"No result for this release yet"* |
| SCANS | `not_run` | `unmeasured` | *"No scan ran for this release."* — never *"passed"* |
| SCANS | `FAIL` | `fail` | *"Blocking — "* + each `blocking[].scan`; each reason on its own line |
| SCANS | `INCOMPLETE` | `warn` | *"Incomplete — "* + scans; *"ran and produced no result … Not a pass."* |
| SCANS | `WARN` / `PASS` / `SKIPPED` | `warn` / `ok` / `unmeasured` | the word, and `detailsSource` when it is `unverified`: *"findings not attributable to this release"* |
| CURRENT | `current.source == "rollback"` | — | a line under BUILD: *"Serving `<version>` since `<time>` — rolled back from this cockpit."* |
| CURRENT | `current.source == "deployment"` and `current.deploymentId != build.deploymentId` | — | a line under BUILD: *"Serving `<version>` since `<time>` — the newest build has not replaced it."* (task 08 rule 6: a failed or running newest build is never current, review I3) |

#### The pin cell — seven states, none merged (provenance contract §4.1; ADR-H1 Q2 adds `erased`)

| `row.release` / `usePinRef` | Drawn |
|---|---|
| `release == null`, `releaseLoaded` | *"no brief is recorded for this release"*, quiet — **shown, never hidden** (P9) |
| `release == null`, `!releaseLoaded` | *"pin not loaded"*, quiet, in the same fixed width as `resolving` — never *"no brief is recorded"*: the cockpit holds no answer from the Hub about this release — not read yet, not readable while signed out, or not readable under this binding's key; the drawer (task 17) names which (task 08 rule 5b, review M11; plan re-review b09-1, M-2) |
| `pin.status "none"` | the same words, and a reason line: `no_brief` *"no brief was bound to this branch"*; `no_complete_revision` *"the brief had no complete revision yet"*; `source_unpinned` *"rolled back or rebuilt from a release with no brief"*; `source_unknown` *"rolled back or rebuilt from a release shipped outside any cockpit"* |
| `resolving` | a fixed-width placeholder the width of *"rev 12 · brief now at rev 13"*; **the cell never changes width** when it resolves |
| `resolved`, `revision == headRevision` | *"rev 6"* (current) |
| `resolved`, `revision < headRevision` | `drift` half-disc + *"rev 5 · brief now at rev 6"* (superseded) |
| `missing` | `fail` square + the key in mono + *"not found"* — *"data loss in our own store"* in its title |
| `erased` | *"erased on 2 Nov 2026"* — its own words, not `missing` |
| `unavailable` | `!` + *"brief not read"*, with one banner above the table carrying the message |

The table's footnote, **corrected for OD-3** (†): *"Releases shipped outside any cockpit —
auto-deploy on push, or the Panacea portal — carry no brief: Panacea has nowhere to store one, and
the cockpit never guesses it afterwards."* The prototype's *"a teammate's cockpit"* is removed: a
teammate's Ship reserves its pin in the same Hub (plan §3.1, OD-3).

**The origin column** (who shipped it) reads `release.reservedBy.displayName` for a pinned or
pinned-none release, and *"outside any cockpit"* for `release == null` with `releaseLoaded` — never
*"this cockpit"*, which was true only while releases were per-laptop. With `!releaseLoaded` it reads
*"not loaded yet"*: who shipped it is not known yet (review M11). Test `origin is the reserver` gains that case.

#### Banners

- **Stale** (`view.stale`): *"Panacea has not answered since `<fetchedAt, absolute>`. Everything below
  is what it said then. Ship and rollback are off until it answers."* The whole lens body takes the
  `dim` treatment (opacity only). When `reason.code` is `deploy_not_found`, `deploy_forbidden`,
  `deploy_conflict` or `deploy_rejected`, Panacea **did** answer — it refused this project (task 07
  step 2, review M13) — so the banner reads instead *"Panacea refused to show this project:
  `<reason.message>` Everything below is what it said at `<fetchedAt, absolute>`. Ship and
  rollback are off while the cockpit checks which Panacea project deploys this repository."*
  `DeployView.staleReason.code` is already `string` (task 13), so no type changes. Test
  `a refusal is not silence`.
- **Collision** (`env.collision` on the selected row, i.e. `branchesSeen.length > 1`; review b02
  removed `env.branch !== branchName`, which fired for every `/` branch because `env.branch` is
  Panacea's label): *"Panacea recorded deployments to `<env>` under more than one branch label:
  `<branchesSeen, joined>`. These are labels, not git branch names. A Ship and a push write the
  label differently (a push keeps `_`; a Ship turns it into `-`), so one branch can appear under
  two labels. If these are different branches, they share this environment — Panacea turns `/`
  and `_` into `-` and shortens names to 20 characters (`handlers/projects.go:3814-3826`) — so
  each Ship replaces the other's release, and Panacea reports no error."* (Re-review b03, I-1: a
  count of labels is never presented as a count of branches.) The values are `Deployment.branch`,
  written by two Panacea writers that disagree: the API's `sanitizeLabelValue(branch)`
  (`clients/pipeline.go:609`) and the webhook-receiver's `sanitizeLabel(event.Branch)`
  (`webhook-receiver/pipeline/trigger.go:1270`, `:1553-1574`) (review I4, plan §12). They are
  shown exactly as written — the desktop never folds `_` into `-` or otherwise normalises them —
  so the banner never calls one a branch name, never renders one as a link to a branch or lane,
  and never offers one to Ship or Rebuild. The banner does not say which branch is serving now:
  the labels cannot be compared with this branch's name without a copy of Panacea's rules. A fact
  from source, stated once; the cockpit does not predict it (never a fourth copy of the rule).
  **Blind spot, stated not fixed** (b02 round 2, R7): branches whose names differ only in `/`
  versus `_` versus `-` (`feat/x`, `feat_x`, `feat-x`) share the environment `feat-x`, and when
  each reaches it by a Ship (or `feat/x` by a push and `feat_x` by a Ship) they also share the run
  label `feat-x` (`sanitizeLabelValue`, `clients/pipeline.go:2351-2364`, applied at `:609`), so
  `branchesSeen` holds one entry and no banner is drawn while they overwrite each other. Twins
  that differ in case (both writers keep capitals) or past the 20th character (the label is cut at
  63) do show. The banner never claims *"one branch"*: its absence is not a proof.

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `three facts always three` | a view with build `succeeded`, serving `ready 0`, scans `FAIL` renders three separate fact blocks with `ok`, `fail`, `fail` | a single "status" pill (survey §3.3) |
| `unmeasured is not down` | `serving {measured:false}` → *"Not measured"*, hollow ring; never the word *"Down"* | nil painted as 0 |
| `failed newest names what serves` | `build {state:"failed", deploymentId:"d2"}`, `current {source:"deployment", deploymentId:"d1"}`, `d1`'s row `version "manual-4e7a91c0"` → BUILD says *"Failed at …"* and the line *"Serving `manual-4e7a91c0` since … — the newest build has not replaced it."*; `current.deploymentId == build.deploymentId` → no such line (review I3) | a card that shows only the failed build, leaving the user to guess what is live |
| `not run is not pass` | `scans.state "not_run"` → the not-run sentence; the DOM lacks *"Pass"* | absence read as clean |
| `version unknown` | `version: null` → *"version unknown"* | a blank cell |
| `pin: seven states` | one test per row of the pin table | two provenance states merged |
| `pin never from head` | a row pinned to rev 5 with the cache's head at 6 → *"rev 5 · brief now at rev 6"*, and nowhere *"rev 6"* alone | rendering a release from the head (P1) |
| `resolving keeps width` | the cell's width style is identical in `resolving` and `resolved` | layout shift when the tick lands (AGENTIC-UX §10 rule 7) |
| `key never parsed` | `usePinRef.ts` and `PinCell.tsx` contain no `split(`, `indexOf(`, `match(` or `RegExp` applied to a key (a source grep in the test) | P5 |
| `elapsed uses the one clock` | the building fact re-renders on the heartbeat tick; `EnvironmentCard.tsx` contains no `setInterval`/`setTimeout` | a timer per card |
| `table windows past 50` | 200 rows → fewer than 60 `role="row"` in the DOM; scrolling reveals later rows | 200 live rows re-rendering each tick |
| `origin is the reserver` | a teammate's pinned release → their display name; `release: null` → *"outside any cockpit"*; `release: null, releaseLoaded: false` → the origin *"not loaded yet"* and the pin cell's *"pin not loaded"*, never *"outside any cockpit"* or *"no brief is recorded"* (review M11) | the per-laptop "this cockpit" wording surviving OD-3; a release not yet read claimed to have been shipped outside any cockpit |
| `collision banner words` | two labels seen (`feat-a-x`, `feat-a-y`) → the banner with both labels after *"under more than one branch label:"* and the cited rule, and neither label inside an `<a>` or a button; one label → none | a silent overwrite (brief R2), or a label presented as a git branch the user could act on (review I4) |
| `push and Ship labels are not two branches` | `branchesSeen ["feat-x","feat_x"]` (one branch `feat_x`, pushed then Shipped; re-review b03, I-1) → the banner is drawn; it contains `feat_x` with its `_` and `feat-x`, and the sentence *"A Ship and a push write the label differently"*; the DOM lacks *"more than one branch."*, *"deployed from more than one branch"* and *"two branches"* | a banner that counts labels as branches (a false claim for one branch), or a desktop that folds `_` into `-` and so hides the push label |
| `slash branch finds its card` | `selected {branch:"feat/x", target:"feat-x", environment:"feat-x", error:null}`, a row `{name:"feat-x", branch:"feat-x", branchesSeen:["feat-x"]}` → the card for `feat-x`, and **no** collision banner | `e.branch === branchName`, which drew the empty state and a false collision for every `/` branch (review C2) |
| `long branch finds its card` | `selected {branch:"lane/7-deploy-connector-watcher", target:"lane-7-deploy-connec", environment:"lane-7-deploy-connec", error:null}`, that row → its card | a join that works only for 20 characters or fewer |
| `production branch finds its card` | `selected {branch:"main", target:"prod", environment:"production", error:null}`, the row `production` → its card, and `ReleaseTable` marks the rows with `environmentRow "production"` | an exact `target === e.name` compare in the desktop |
| `never deployed names the target` | `selected {target:"feat-new", environment:null, error:null}` → *"Not deployed yet. Panacea will deploy this branch to `feat-new`."*; `target:null, error:null` → the asking sentence | a card that guesses, or says "never deployed" while Panacea is still being asked |
| `refused names the refusal` | `selected {branch:"live-check/p7", target:null, environment:null, error:{code:"not_found", message:"Panacea could not read this branch name. It may contain \`/\`, which the route cannot carry."}}` → *"Panacea did not name this branch's environment: "* + the message verbatim, and the DOM lacks *"Asking Panacea"* (b02 round 2, R2) | a refusal drawn as the asking state for ever |
| `no selection, no card` | `selected: null` → `EnvironmentCard` renders nothing; the DOM lacks *"Asking Panacea"* and *"Not deployed yet"*; the list still renders (b02 round 2, R6) | a lens that says it is asking when it sent no branch |
| `mismatch is drawn` | a row `{environmentRow:"production", environmentMismatch:{recorded:"develop", deployed:"prod"}}` → *"The release was recorded against `develop`, but Panacea deployed it to `prod`."*, under `production` (b02 round 2, R1) | a false provenance record shown as if it held |
| `no environment recorded is titled` | `unmatched: [{name:""}, {name:"legacy"}]` → two groups, titled *"no environment recorded"* and `legacy`; no group has an empty title (review M5) | a blank group heading, or `""` dropped as falsy |
| `a refusal is not silence` | `stale: true`, `staleReason {code:"deploy_forbidden", message:"You do not have access to this project."}` → *"Panacea refused to show this project: "* + the message verbatim, and the DOM lacks *"has not answered"*; the same with `deploy_not_found`; `upstream_unavailable` → the *"has not answered since"* sentence (review M13) | a deleted or revoked project drawn as a network blip that will pass |
| `no branch comparisons` | a source grep of `desktop/src/deploy/*.tsx` (b02 round 2, R5, widened): no identifier `branchName` or `selectedBranch`; no `.branch` on either side of `===`, `!==`, `==` or `!=` (`/\.branch\s*[!=]==?/` and `/[!=]==?\s*[\w.?]*\.branch\b/`); no `.includes(`, `.startsWith(` or `.endsWith(` whose receiver or argument ends in `branch` or `branchesSeen` | a later edit re-introducing the name join, in either operand order |
| `drift guard` | `designsystem.test.ts` allows `var(--drift)` in `common.css`'s `dot-tone-drift` and nowhere else | drift colour leaking to decoration |

#### Steps

- [ ] Check first: `grep -n "var(--drift)" desktop/src -r` — if phase 4 already amended the guard,
  reuse its rule; otherwise amend it here to *"references it only in `common.css` for
  `dot-tone-drift`"*.
- [ ] Tests first; `StatusDot` amendment; `usePinRef` (over phase 4's selector, or a stub returning
  `resolving` for every key if phase 4 has not merged — say which in the commit message); the
  components; the banners.
- [ ] `npx tsc --noEmit`, `npx vitest run`; commit by pathspec with *"Visual acceptance pending the
  owner (OD-12)."*

#### Acceptance

```
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass
```
**Owner's eyes (batched):** the live states — building, built-but-down, scan blocking, a superseded
pin, a release with no brief — against the prototype. **Unverified until then.**

#### What this task does NOT claim

- That phase 4's cache exists. With the stub every pin reads `resolving` — honest, never a head.
- Pixel fidelity to the prototype. The prototype is the owner's reference; the owner judges.
