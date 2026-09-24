### Task 16: Desktop — the Ship bar and the held acts

**Half:** 7b. **Hours:** 3. **Depends on:** 15, 09, 10. **Elsewhere — hard, no fallback:** phase 3
task 18 merged (`pushBranch`, `addGoodTickListener`, `selectConvergence`, `useConvergenceLoader`),
and phase 3 task 12 merged (the server push that `pushBranch` calls). The Push button and readiness
edge 4 (review I7) both come from task 18, so without it there is no Push to press and Ship reads
*"not pushed"* after every push — the I7 defect. Task 16 does not start until both are merged
(plan re-review b04-1, M-1).

Ship, rollback and rebuild are **held for five seconds with Undo, not confirmed in a dialog**
(DESIGN-SYSTEM *Interaction standards*: *"Optimistic mutation with an Undo window, not confirm
dialogs"*; brief `B3`; plan 7P-15). Panacea cannot un-start a pipeline or un-flip a Service, so the
only honest undo is **"not sent yet"**: the request leaves the desktop only when the hold ends.
What an act will do is printed **beside its button before it is pressed** — the hold is the undo,
the sentence is the confirmation.

**Read `docs/DESIGN-SYSTEM.md` first.** Motion must explain causality: the hold is drawn as a
hairline that shrinks over 5 s by `transform: scaleX` — it **is** the time until send — with a
`prefers-reduced-motion` stand-in, *"Sends in 5 s"*, counted by `useHeartbeat()` (the 1 Hz shared
tick, `hooks/heartbeat.ts:64`) and no timer of its own for display. The send itself is one
`setTimeout` per act (not a `setInterval`; the one-clock rule's two sites are unchanged). No spinner
while sending: the button's label reads *"Sending…"* and the row appears when the cursor moves.
**Visual acceptance is the owner's, batched (OD-12).**

**Files:**
- `desktop/src/deploy/useHeldAct.ts` (new, unless task 18 landed it first from this contract) — the hold
- `desktop/src/deploy/ShipBar.tsx` (new) — the lens's top bar: Ship, Cancel run, the *will* line
- `desktop/src/deploy/ReleaseActs.tsx` (new) — `RollbackButton`, `RebuildButton`, used by the drawer (task 17)
- `desktop/src/deploy/ActBanners.tsx` (new) — the outcome of the last act
- `*.test.tsx` beside each
- `desktop/src/deploy/DeployLens.tsx` (modified, task 14's) — mounts `ShipBar` above the environment card, and passes `RollbackButton` / `RebuildButton` into task 17's `ReleaseDrawer` `acts` slot
- `desktop/src/deploy/types.ts`, `api.ts`, `deploySlice.ts`, `loader.ts` (modified, task 13's) — `ShipReadiness`, `DeployApi.shipReadiness(projectId, branchId)`, `deployReadiness`, and the readiness load on its four edges (below; review I7)
- `desktop/src/deploy/DeployLens.tsx` also mounts phase 3's `useConvergenceLoader("merge")` (phase 3 task 18), so the selected branch's convergence is read on the existing good tick while the lens is open

#### Contracts — complete

```ts
export type HeldState<R> =
  | { phase: "idle" }
  | { phase: "held"; startedAt: number; key: string }  // key: the Idempotency-Key, minted at start
  | { phase: "sending"; key: string }
  | { phase: "done"; result: R }
  | { phase: "failed"; error: ApiError; key: string };  // retry reuses the key

/** Holds an act for holdMs, then runs it once with a stable Idempotency-Key. undo() in "held"
 *  cancels with NO request sent. retry() in "failed" re-sends with the SAME key, so a lost response
 *  cannot ship twice (task 09 step 1). Unmounting in "held" cancels, never sends. */
export function useHeldAct<R>(
  send: (idempotencyKey: string) => Promise<R>,
  holdMs?: number, // default 5000
): { state: HeldState<R>; start(): void; undo(): void; retry(): void };
```

```tsx
export function ShipBar(props: {
  projectId: string;
  branch: { id: string; name: string };
  view: DeployView;
  readiness: ShipReadiness | null; // task 09's readiness route, loaded by task 13's loader
}): JSX.Element;

export function RollbackButton(props: { projectId: string; row: ReleaseRow; view: DeployView }): JSX.Element;
export function RebuildButton(props: { projectId: string; row: ReleaseRow; view: DeployView }): JSX.Element;
export function ActBanners(props: { projectId: string }): JSX.Element | null;
```

`ShipReadiness` is the body of `GET /v1/projects/{projectId}/deploy/ship-readiness?branchId=` (task
09, amended): `{ "ready": boolean, "reason": "" | <task 09's precondition reasons> | "stale" |
"not_connected" | "signed_out", "message": "…", "commit": "<40 hex>" | null, "environment": "…" | null,
"pinPreview": { "status": "pinned" | "none", "reason": "…", "briefId": "bf-…" | null,
"revision": 6 | null } | null }`. One function computes readiness for both this display and Ship's
enforcement, so the button and the server cannot disagree about why.

#### The Ship bar — what it says, and when Ship is off

**The *will* line** (before the press, B3): *"Builds `<short commit>` from the remote · pins brief
rev `<revision>`"* from `pinPreview` — or *"· pins no brief — none is bound to this branch"*, or
*"· the Hub picks the brief at Ship"* when `pinPreview` is null — plus, on a collision, *"· replaces
the release of `<other branch>`"* in `--need`. `pinPreview` is the Hub's rule (latest complete
revision, ADR-H1 Q1) read ahead of time; the pin itself is still taken by the Hub at reserve, and
the row shows what it actually took.

| Ship is disabled when | Its reason (the button's `title` and a line under the bar) |
|---|---|
| `connection.state != "connected"` | *"Connect to Panacea first."* |
| `view.stale` | *"Panacea is not answering."* |
| `releasesState` `signed_out` / `no_hub_project` / `unavailable` | *"Sign in first."* / task 14's line / *"The Hub is not answering, so the release could not be recorded."* |
| a run for this branch is not terminal | *"A build of this branch is already running."* — and **Cancel run** is shown |
| `readiness.ready == false` | `readiness.message` verbatim (*"2 commits are not pushed…"*, with a **Push** button beside it, calling phase 3's `pushBranch`, when `reason == "unpushed"`) |

Label: *"Ship"*; *"Ship anyway"* in `--need` on a collision; *"Sends in 4 s · Undo"* while held;
*"Sending…"* while sending.

#### When readiness reloads — four edges, no timer (review I7)

`ahead`, `behind` and the tip are phase 3's **local** git facts (task 09's `Branch`, plan 7P-7).
A push changes them and bumps no deploy cursor, so the first three edges alone leave Ship reading
*"2 commits are not pushed"* after the user pressed **Push** beside it. Readiness for the selected
branch is loaded, once per edge and never stacked behind a load in flight, when:

1. the deploy cursor changes on a poll tick (task 13);
2. the branch selection changes;
3. the Deploy lens opens;
4. **the branch's git facts move.** On each good poll tick — phase 3's `addGoodTickListener`
   (phase 3 task 18), the existing tick, never a new interval — the loader reads
   `selectConvergence(branchId)`. When its ready value's `tip`, `remote.upstream`, `remote.ahead`
   or `remote.behind` differs from the branch's recorded baseline (the four values below), it
   reloads readiness. The value is fresh because `DeployLens` mounts phase 3's own
   `useConvergenceLoader("merge")` for the selected branch while it is open (reuse, not a copy:
   phase 3's hook, unchanged). A `loading` or `error` value is not a move. **And Push
   completing:** the Push button beside Ship (phase 3's `pushBranch`) reloads readiness when it
   resolves, and calls the convergence loader's `reload()`, so Ship turns on without waiting for
   a tick. A rejected push reloads nothing.

A tick with no move sends nothing, so the one-clock rule holds: readiness has no clock of its own.

**The baseline edge 4 compares against** (plan re-review b04-1, M-2). Three edges load readiness
while convergence is not yet ready or is about to change: a lens open and a new selection (phase
3's hook is still on its first read), and Push (its `reload()` has not landed). So a load on edge
4 records the value that moved as the baseline, and a load on any other edge — cursor, selection,
lens open, Push — clears the branch's baseline and notes the instant it was sent. On a good tick,
a ready convergence value whose `Load.at` (phase 3 t18:231) is **at or after** that instant, while
there is no baseline, is **recorded as the baseline without a reload**. A ready value older than
the load, or a `loading`/`error` value, records nothing. Only a ready value compared with a
recorded baseline can be a move. So a lens open, a selection and a Push each cost exactly one
`shipReadiness` call, and the value Push's `reload()` brings back becomes the baseline rather
than a second call. What this leaves: a push that lands between readiness's own git read and the
first ready convergence read after it is seen at the next move, not at once (does-not-claim).

**Cancel run** is not held — it is the undo of a Ship already sent. It calls `cancel` once;
`already_finished` shows *"The build had already finished."*

#### Rollback and rebuild buttons (drawn in the drawer, task 17)

| Button | Offered when | The sentence beside it, before the press |
|---|---|---|
| **Roll back to this** | the row is `kind "run"`, `build "succeeded"`, not `isCurrent`, not its environment's `current.deploymentId` (a rollback's target serves while its own run row is not current: task 08 rule 6, review I3), and its environment has no running build | *"Returns `<env>` to `<version>`. Keeps this release's brief pin — not the brief as it reads today. How fast it is, Panacea says only once it starts."* |
| **Rebuild** | the row has a 40-hex `commit` and a Hub release (`release != null`) | *"Builds `<short commit>` again as a new release, with this release's brief pin."* |

**Its environment** is the drawer's own lookup, `view.environments.find(e => e.name ===
row.environmentRow)` (task 17), never a compare with `row.environment`: a production release's
`environment` is `prod` and its row is `production` (task 08 rule 1). *"No running build"* means
that row's `build.state` is neither `running` nor `pending`. The check **fails closed**: with
`environmentRow == null`, or no row found, **Roll back** is not offered — a lookup that finds nothing
must never read as *"nothing is building"* (b02 round 2, R9).

**Rebuild** needs the branch from the Hub release's `branchRef` (task 10): a row with
`release == null` came from outside any cockpit, and Panacea recorded only a run label for its
branch (`clients/pipeline.go:609` for the API, `webhook-receiver/pipeline/trigger.go:1270` for a
push; plan §12). A Hub release without a `branchRef` gets the control plane's
`409 no_branch`, shown verbatim (b02 round 2, R8).

Disabled rows say why: *"A failed build has no image to roll back to."*, *"This is what `<env>` is
serving."*, *"`<env>` has a build running. Roll back once it finishes."*, *"Panacea lists no
environment named `<row.environment>`."*, *"Off while Panacea is not answering."*, *"This release
did not record a full commit."*, *"Panacea recorded only a label for this release's branch. Ship the
branch instead."*.

#### Outcome banners (`ActBanners`)

| Outcome | Banner |
|---|---|
| rollback, `warmth "warm"` | *"Rolled back `<env>` to `<version>` — warm: Panacea re-pointed the service to pods still running."* + **Roll forward to `<newer version>`** (itself a held rollback) |
| rollback, `warmth "cold"` | *"… — cold: new pods are starting. Panacea estimates about `<coldStartSeconds>` s; advisory."* (prototype `:694-701`) |
| rollback refused, `deploy_conflict` | the Panacea sentence verbatim, and **Rebuild `<short commit>`** — offered because the code is `deploy_conflict` on a rollback, never by reading the sentence |
| any act, `upstream-down` | the call site's own sentence through `classify` (task 13): *"`<upstreamName>` could not be reached, so nothing was sent."* — or, for `details.upstream "panacea"` after the Hub reserved, *"… the release was not started."* |
| Ship `confirmed: false` | *"Panacea started the build. The shared record is not confirmed yet; the cockpit keeps trying until `<expiresAt>`."* |
| Ship refused (`deploy_*`) | the sentence verbatim; the history shows it as a `refused` row too (task 08 rule 8) |

#### Tests to write first (fake timers for the hold only)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `undo sends nothing` | `start()`, advance 4.9 s, `undo()`, advance 10 s → the `send` spy was never called | an undo that only hides the result of a request already sent |
| `hold then one send` | `start()`, advance 5 s → one call, with a key matching `/^[A-Za-z0-9_-]{1,64}$/` | a hold that sends twice, or a key the server refuses |
| `retry reuses the key` | first send rejects `upstream_unavailable`; `retry()` → the second call has the same key | a retried Ship reserving a second release |
| `unmount cancels` | unmount while held → no call | a Ship fired from a lens the user has left |
| `will line before the press` | idle bar with `pinPreview rev 6` shows *"pins brief rev 6"* | a consequence shown only after the act |
| `disabled reasons` | one test per row of the disabled table, asserting the reason text and `disabled` | a live 403 discovered instead of designed (brief R7) |
| `countdown on the one clock` | the reduced-motion countdown text changes on heartbeat ticks; `useHeldAct.ts` and `ShipBar.tsx` contain no `setInterval` | a display timer per act |
| `rebuild offered on pruned` | a rollback rejected `deploy_conflict` → the banner has **Rebuild**; rejected `deploy_rejected` → it does not | offering Rebuild by parsing Panacea's sentence |
| `rollback copy keeps the pin` | the rollback sentence names *"this release's brief pin"* | implying rollback re-reads today's brief (P1) |
| `rollback off while production builds` | row `{kind:"run", build:"succeeded", isCurrent:false, environment:"prod", environmentRow:"production"}`, `view.environments` holds `production` with `build.state "running"` → **Roll back** disabled with *"`production` has a build running. Roll back once it finishes."*; the same row with `environmentRow: null` → not offered, with the *"Panacea lists no environment named `prod`."* line (b02 round 2, R9) | `e.name === row.environment`, which finds no row for `prod` and so offers Roll back while production is building |
| `roll back follows what is serving` | environment `production` with newest `d2` `build "failed"` and `current {source:"deployment", deploymentId:"d1"}`; rows `d1` (`succeeded`, `isCurrent:true`) and `d0` (`succeeded`) → **Roll back** is not offered on `d1` (*"This is what `production` is serving."*) and is offered on `d0`; with `current {source:"rollback", deploymentId:"d0"}` and the `rollback` row current → `d0`'s run row (`isCurrent:false`) says *"This is what `production` is serving."* and `d1` offers Roll back (review I3) | reading "current" as the newest row, so a failed build hides Roll back on the release that really serves, or a rollback's target offers a rollback to itself |
| `rebuild needs a Hub release` | a 40-hex row with `release: null` → **Rebuild** disabled with *"Panacea recorded only a label for this release's branch. Ship the branch instead."*; with a release → offered (b02 round 2, R8) | offering a Rebuild the control plane can only refuse |
| `cancel not held` | pressing Cancel run calls `cancel` at once | a 5 s delay on the one act that must be immediate |
| `readiness on the four edges` (`loader.test.ts`) | a cursor change, a branch selection and a lens open each call `api.shipReadiness` once for the selected branch; a good tick whose convergence has `remote.ahead` 2 → 0 calls it once; 60 s of fake time and ten good ticks with the same convergence value, or with it `loading`, → zero calls. **Baseline (re-review b04-1, M-2):** open the lens with convergence `loading` → one call; the convergence then lands ready (`at` after the load) and one good tick fires → zero further calls. Press Push: the store still holds the pre-push value (`ahead 2`, `at` before the press), `pushBranch` resolves → one call; a good tick with that old value → zero; the `reload()` value (`ahead 0`, `at` after the press) lands and a good tick fires → zero further calls | a readiness poll on a timer — a second clock (DESIGN-SYSTEM one-clock rule) — or one fired on every tick; or a baseline taken from whatever value is in the store, so every lens open, selection and Push costs a second call on the next tick |
| `push, then Ship turns on with no reselect` (`ShipBar.test.tsx`) | readiness `{ready:false, reason:"unpushed"}` → Ship disabled with Push beside it; press Push, `pushBranch` resolves, `shipReadiness` now answers `{ready:true}` → Ship is enabled, with no branch selection, lens reopen or cursor change in between; `pushBranch` rejecting `push_rejected` → no second `shipReadiness` call (review I7) | readiness reloaded only on the cursor, selection and lens edges, so Ship keeps saying *"2 commits are not pushed"* after the push |
| `a push elsewhere turns Ship on on the next tick` (`loader.test.ts`) | readiness `unpushed` at `{tip:T, ahead:2}`; the store's convergence becomes `{tip:T, ahead:0}` (a terminal `git push`) and a good tick fires → one `shipReadiness` call, and Ship is enabled; a tick where only `readAt` changed → none (review I7) | a Push-button-only edge that misses a push from the terminal |
| `readiness failure keeps the bar honest` | `shipReadiness` rejects `upstream_unavailable` → `readiness null`, Ship off with the stale line, the old readiness not shown | a Ship button left on from a readiness the server no longer vouches for |

#### Steps

- [ ] Extend task 13's `loadDeployView` to also load task 09's `ship-readiness` for the selected
  branch, on the four edges above (cursor, selection, lens open, and the branch's git facts moving
  on the good tick or Push completing; review I7) — never on a timer. The slice gains
  `deployReadiness: Record<branchId, ShipReadiness>`, not persisted, and the loader keeps, per
  branch, the instant its last readiness load was sent and the `{tip, upstream, ahead, behind}`
  baseline recorded after it, or none yet (the baseline rule above; in memory, not in the store).
- [ ] Tests first; `useHeldAct`; `ShipBar`; `ReleaseActs`; `ActBanners`.
- [ ] `npx tsc --noEmit`, `npx vitest run`; commit by pathspec with *"Visual acceptance pending the
  owner (OD-12)."*

#### Acceptance

```
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass
```
**Owner's eyes (batched):** press Ship, Undo inside 5 s, see nothing happen in Panacea; press Ship
and wait, see the run appear; roll back and read the warmth line. **Unverified until then.**

#### What this task does NOT claim

- That five seconds is the right hold. It is the house default; `holdMs` is one argument.
- That the pin preview is the pin. The Hub takes the pin at reserve (G1); the preview reads the same
  rule a moment earlier, and a teammate's edit in between makes the row differ — which the row then
  shows truthfully.
- That mounting phase 3's `useConvergenceLoader("merge")` in the Deploy lens is cheap (edge 4,
  review I7). It is the load the Merge lens already makes on every good tick (a convergence read,
  and history only when the tip moved). Its cost in the Deploy lens is **unmeasured**. If phase 3
  later offers a lighter `what` that reads only `remote` and `tip`, the swap is one argument.
- That edge 4 sees a push the moment it happens. It sees it on the next good tick after phase 3's
  convergence read reflects it; a push from this lens's Push button is seen at once. A push from
  elsewhere that lands after readiness read git but before the first ready convergence read that
  follows it becomes the baseline and is seen only at the next move (the baseline rule; re-review
  b04-1, M-2). The window is one convergence read; its length is unmeasured.
