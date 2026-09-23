### Task 16: Desktop — the Ship bar and the held acts

**Half:** 7b. **Hours:** 3. **Depends on:** 15, 09, 10.

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
- `desktop/src/deploy/types.ts`, `api.ts`, `deploySlice.ts`, `loader.ts` (modified, task 13's) — `ShipReadiness`, `DeployApi.shipReadiness(projectId, branchId)`, `deployReadiness`, and the readiness load on the loader's three edges

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
| `readiness.ready == false` | `readiness.message` verbatim (*"2 commits are not pushed…"*, with phase 3's **Push** button beside it when `reason == "unpushed"` and phase 3 task 12 is merged) |

Label: *"Ship"*; *"Ship anyway"* in `--need` on a collision; *"Sends in 4 s · Undo"* while held;
*"Sending…"* while sending.

**Cancel run** is not held — it is the undo of a Ship already sent. It calls `cancel` once;
`already_finished` shows *"The build had already finished."*

#### Rollback and rebuild buttons (drawn in the drawer, task 17)

| Button | Offered when | The sentence beside it, before the press |
|---|---|---|
| **Roll back to this** | the row is `kind "run"`, `build "succeeded"`, not `isCurrent`, and its environment has no running build | *"Returns `<env>` to `<version>`. Keeps this release's brief pin — not the brief as it reads today. How fast it is, Panacea says only once it starts."* |
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
| `rebuild needs a Hub release` | a 40-hex row with `release: null` → **Rebuild** disabled with *"Panacea recorded only a label for this release's branch. Ship the branch instead."*; with a release → offered (b02 round 2, R8) | offering a Rebuild the control plane can only refuse |
| `cancel not held` | pressing Cancel run calls `cancel` at once | a 5 s delay on the one act that must be immediate |
| `readiness on the three edges` (`loader.test.ts`) | a cursor change, a branch selection and a lens open each call `api.shipReadiness` once for the selected branch; 60 s of fake time with none of them → zero calls | a readiness poll on a timer — a second clock (DESIGN-SYSTEM one-clock rule) |
| `readiness failure keeps the bar honest` | `shipReadiness` rejects `upstream_unavailable` → `readiness null`, Ship off with the stale line, the old readiness not shown | a Ship button left on from a readiness the server no longer vouches for |

#### Steps

- [ ] Extend task 13's `loadDeployView` to also load task 09's `ship-readiness` for the selected
  branch, on the same three edges (cursor, selection, lens open) — never on a timer. The slice gains
  `deployReadiness: Record<branchId, ShipReadiness>`, not persisted.
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
