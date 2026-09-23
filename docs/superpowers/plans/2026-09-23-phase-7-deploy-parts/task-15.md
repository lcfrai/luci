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
export function EnvironmentCard(props: { env: EnvironmentView | null; branchName: string; stale: boolean; staleSince: string | null }): JSX.Element;
export function EnvironmentList(props: { environments: EnvironmentView[]; unmatched: EnvironmentView[]; selectedBranch: string }): JSX.Element;
export function ReleaseTable(props: { rows: ReleaseRow[]; selectedBranch: string; onOpen(row: ReleaseRow): void; stale: boolean }): JSX.Element;
export function PinCell(props: { row: ReleaseRow }): JSX.Element;
export function StaleBanner(props: { since: string | null; reason: { code: string; message: string } | null }): JSX.Element | null;
export function CollisionBanner(props: { env: EnvironmentView; branchName: string }): JSX.Element | null;
```

**Which environment is "this branch's".** `environments.find(e => e.branch === branchName)`, exact
(task 08 rule 9). None → the card's empty state. `unmatched` groups render in `EnvironmentList`
under their raw names, never folded into a row.

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

#### The pin cell — seven states, none merged (provenance contract §4.1; ADR-H1 Q2 adds `erased`)

| `row.release` / `usePinRef` | Drawn |
|---|---|
| `release == null` | *"no brief is recorded for this release"*, quiet — **shown, never hidden** (P9) |
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
pinned-none release, and *"outside any cockpit"* for `release == null` — never *"this cockpit"*,
which was true only while releases were per-laptop.

#### Banners

- **Stale** (`view.stale`): *"Panacea has not answered since `<fetchedAt, absolute>`. Everything below
  is what it said then. Ship and rollback are off until it answers."* The whole lens body takes the
  `dim` treatment (opacity only).
- **Collision** (`env.collision` and `env.branch !== branchName`, or `branchesSeen.length > 1`):
  *"`<env>` is serving `<env.branch>`, not this branch. Panacea gives both branches one environment
  name — it turns `/` and `_` into `-` and shortens names to 20 characters
  (`handlers/projects.go:3814-3826`) — so each Ship replaces the other's release, and Panacea reports
  no error."* — a fact from source, stated once; the cockpit does not predict it (never a fourth
  copy of the rule).

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `three facts always three` | a view with build `succeeded`, serving `ready 0`, scans `FAIL` renders three separate fact blocks with `ok`, `fail`, `fail` | a single "status" pill (survey §3.3) |
| `unmeasured is not down` | `serving {measured:false}` → *"Not measured"*, hollow ring; never the word *"Down"* | nil painted as 0 |
| `not run is not pass` | `scans.state "not_run"` → the not-run sentence; the DOM lacks *"Pass"* | absence read as clean |
| `version unknown` | `version: null` → *"version unknown"* | a blank cell |
| `pin: seven states` | one test per row of the pin table | two provenance states merged |
| `pin never from head` | a row pinned to rev 5 with the cache's head at 6 → *"rev 5 · brief now at rev 6"*, and nowhere *"rev 6"* alone | rendering a release from the head (P1) |
| `resolving keeps width` | the cell's width style is identical in `resolving` and `resolved` | layout shift when the tick lands (AGENTIC-UX §10 rule 7) |
| `key never parsed` | `usePinRef.ts` and `PinCell.tsx` contain no `split(`, `indexOf(`, `match(` or `RegExp` applied to a key (a source grep in the test) | P5 |
| `elapsed uses the one clock` | the building fact re-renders on the heartbeat tick; `EnvironmentCard.tsx` contains no `setInterval`/`setTimeout` | a timer per card |
| `table windows past 50` | 200 rows → fewer than 60 `role="row"` in the DOM; scrolling reveals later rows | 200 live rows re-rendering each tick |
| `origin is the reserver` | a teammate's pinned release → their display name; `release: null` → *"outside any cockpit"* | the per-laptop "this cockpit" wording surviving OD-3 |
| `collision banner words` | two branches seen → the banner with both names and the cited rule; one branch → none | a silent overwrite (brief R2) |
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
