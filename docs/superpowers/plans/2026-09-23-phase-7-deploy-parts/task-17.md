### Task 17: Desktop — the release drawer (the manifest), scans, why it is down, the log pane

**Half:** 7a. **Hours:** 3. **Depends on:** 15, 12. **Elsewhere:** phase 4 task 17's
`ShippedManifest` and phase 4 task 16's `selectProvenanceState` / `selectRef` / `selectHead` /
`openPast` — **if they are not merged, the fallback below**. Task 16 fills the drawer's act slot;
task 19 fills its sessions slot.

One click on any release row opens its **manifest**. The pin comes first — *"`v1.6.0` shipped
against brief `bf-…`, revision 6"* — then the brief as it read **at that revision**, never the head
(P1), the design key the release carried, the sessions that produced it (task 19), and the
release's own facts. The drawer is also where the lens drills in, **one thing at a time** (plan
7P-14): the release's scans, why its environment is down, and **one** live stream — build log or
readiness — which closes when the drawer closes or another stream opens.

**Read `docs/DESIGN-SYSTEM.md` first.** This task's non-negotiables: the drawer enters by
`transform: translateX` plus `opacity`, which shows where it came from (the row), and not at all
under `prefers-reduced-motion`; elevation is `--e-raised` (ring plus layered shadow), radius
`--r-card`; the log pane **batches appends at ~10 Hz and never scroll-jumps when the user has
scrolled up** (`DESIGN-SYSTEM.md:65`); **no spinner** — a drill-in that is loading shows its heading
over a fixed-height, static placeholder; `tabular-nums` on every count, time and id. **Visual
acceptance is the owner's, batched (OD-12); unverified until then.**

**Files:**
- `desktop/src/deploy/ReleaseDrawer.tsx` (new) — the manifest and its slots
- `desktop/src/deploy/PinManifest.tsx` (new) — the one mount of phase 4's `ShippedManifest`, and the fallback
- `desktop/src/deploy/ScanList.tsx` (new) — one release's scans, read once on open
- `desktop/src/deploy/DiagnosePanel.tsx` (new) — pods, events, and (on press) the diagnosis, for one environment
- `desktop/src/deploy/LogPane.tsx` (new) — draws frames at ~10 Hz; the pinned-to-bottom rule
- `desktop/src/deploy/drillStream.ts` (new) — the window's one open drill-in stream
- `desktop/src/deploy/usePinRef.ts` (modified, task 15's) — `usePinManifest`, only if phase 4 is merged
- `desktop/src/deploy/DeployLens.tsx` (modified, task 14's) — holds the open row; mounts the drawer beside the table
- `desktop/src/deploy/deploy.css` (modified) — the drawer and the log pane
- `*.test.tsx`, `drillStream.test.ts` beside each

#### Contracts — complete

```ts
// drillStream.ts — THE one open drill-in stream in the window (plan 7P-14; task 12 holds one on
// the server too). Module state, not React state, so two components cannot each hold one.
import type { DeployApi, StreamFrame } from "./api";

export type DrillTarget = { kind: "run-logs"; run: string } | { kind: "readiness"; env: string };
export type DrillEnd = "end" | "replaced" | "closed" | "failed";

export interface DrillStreamHandle {
  readonly target: DrillTarget;
  close(): void; // idempotent; aborts the fetch; onEnd("closed") if it had not already ended
}

/** Opens a stream through DeployApi.stream (task 13). Any open stream, whatever its target, is
 *  closed first and receives onEnd("replaced"). Frames are delivered as they parse; the caller
 *  batches them (LogPane). onEnd fires EXACTLY ONCE: "end" (event: end), "replaced", "closed", or
 *  "failed" with the problem frame's message or the ApiError's. */
export function openDrillStream(
  api: DeployApi,
  projectId: string,
  target: DrillTarget,
  onFrame: (f: Exclude<StreamFrame, { event: "end" }>) => void,
  onEnd: (why: DrillEnd, message?: string) => void,
): DrillStreamHandle;

/** The target of the open stream, or null. The drawer's "Following …" line reads it. */
export function currentDrillTarget(): DrillTarget | null;
```

```tsx
export function ReleaseDrawer(props: {
  projectId: string;
  row: ReleaseRow;
  view: DeployView;
  onClose(): void;
  /** Task 16's RollbackButton / RebuildButton. The drawer lays them out; it decides nothing. */
  acts?: React.ReactNode;
  /** Task 19's ReleaseSessions. Absent → the drawer draws no sessions block at all. */
  sessions?: React.ReactNode;
}): JSX.Element;

export function PinManifest(props: { row: ReleaseRow }): JSX.Element;
export function ScanList(props: { projectId: string; environment: string | null; deploymentId: string; stale: boolean }): JSX.Element; // environment: the ReleaseRow's raw environment; "" or null → no call (re-review b03, M-1)
export function DiagnosePanel(props: { projectId: string; env: EnvironmentView; stale: boolean }): JSX.Element;
export function LogPane(props: { projectId: string; target: DrillTarget }): JSX.Element;
```

```ts
// usePinRef.ts (task 15's file, amended) — still the ONE place phase 7 touches phase 4.
// Written ONLY if phase 4's desktop/src/provenance/ShippedManifest.tsx exists (see Steps).
import type { ProvenanceRef, Head } from "../api/discover";
import type { ProvenanceState } from "../store/selectors";

export interface PinManifestInput {
  refValue: ProvenanceRef | undefined;
  state: ProvenanceState;
  head: Head | undefined;
}

/** Phase 4's own selectors for an OPAQUE key — selectProvenanceState(key), selectRef(key),
 *  selectHead(ref.briefId) — each through useShallow. null ⇔ briefKey null. Never fetches (the
 *  R6 loader does, from task 13's selectReleasePinKeys); never parses the key (P5). */
export function usePinManifest(briefKey: string | null): PinManifestInput | null;
```

**The open row.** `DeployLens` holds `openRow: string | null` in React state — the row's
`deploymentId`, else `release.releaseId` — never in the slice and never persisted. A cursor update
that drops the row closes the drawer; Escape and the ✕ close it. Opening another row replaces it.

#### The manifest — sections, in order, and their words

| Section | Shown when | Words |
|---|---|---|
| Heading | always | `row.version` or *"This release"*; *" · rollback"* when `kind "rollback"`; *" · refused"* when `kind "refused"`; *" · pending"* when `kind "pending"`; *" · not found in Panacea"* when `kind "not_found"` |
| **Pin** (`PinManifest`) | always | the rules below — first, above everything else (provenance contract §4.5) |
| **Design** | `release` non-null | `release.designKey` verbatim in mono — *"Approved design `<key>`, carried at Ship."* — or *"No approved design is linked to this release."* Never resolved or parsed here (P5) |
| **Sessions** | `props.sessions` given | task 19's block, unchanged |
| **Refusal** | `kind "refused"` | `refusal.message` verbatim, and *"Nothing was built. The reservation was abandoned with this sentence."* |
| **Not found** | `kind "not_found"` | *"The Hub confirmed run `<deploymentId>` at `<release.confirmedAt>`, and Panacea's list does not hold it. It may have been deleted, or removed before Panacea recorded it."* No **Scans** or **Log** section, because Panacea lists no such run (task 08 rule 7, review M18) |
| **Release** | always | the facts table below |
| **Acts** | `props.acts` given | task 16's buttons, each with its sentence |
| **Scans** | `deploymentId` non-null, and `kind` is not `"not_found"` | `ScanList` |
| **Why it is down** | the row **serves now** (below), and the row's environment is found | `DiagnosePanel` — open at once when `serving` is `{measured:true, ready:0}`, else behind **Show pods and events** |
| **Log** | a run row with `deploymentId`, or a row that serves now | **Follow build log** → `LogPane` with `run-logs`, for a run row with `deploymentId`; for a row that serves now, with its environment found, also **Follow readiness** → `readiness` |

**Serves now** is `row.isCurrent || (row.deploymentId != null && row.deploymentId ===
env.current.deploymentId)`, with `env` the row's environment found as below — the same key the
Roll back gate uses (task 16). After a rollback, task 08 rule 6 marks the `rollback` row
`isCurrent` and leaves the target's run row unmarked, while `current.deploymentId` names that
target. Gating on `isCurrent` alone left the target's run row with neither block and the rollback
row (`deploymentId: null`) with no Log section, so no row offered **Follow readiness** at the
moment the user most wants to watch the pods come back (plan re-review b04-1, I-1). Both rows now
offer both.

The row's environment is `view.environments.find(e => e.name === row.environmentRow)` — the row
name the view already joined (task 08 rule 1, `SameEnvironment`; or rule 1b, the run label, for
a row with no environment recorded), never `row.environment`
and never re-derived from the branch. The drawer does no join of its own: a production
deployment's `environment` is `prod` while its row is named `production` (`environmentForBranch`,
`clients/pipeline.go:2195-2207`, called at `:237`; `projects.go:2116-2118`), so an exact compare with `row.environment` lost *Why it is
down* and *Follow readiness* for every production release (review I8). `environmentRow == null`
(an `unmatched` row) → neither block, and the Release table says *"Panacea lists no environment
named `<row.environment>`."* When `row.environment` is `""` (task 08 rule 1b) it says instead
*"Panacea recorded no environment for this release."*, never an empty name (review M5).

**`PinManifest` — the rules, none merged** (provenance contract §4.5; task 15's pin states):

| Condition | Drawn |
|---|---|
| `release == null`, `!releaseLoaded` | keyed to the view's `releasesState` (task 08 rule 5b, *Why not loaded*; plan re-review b09-1, M-2): `signed_out` → *"The cockpit has not loaded this release's pin from the Hub. Sign in to the Hub to see it."*; `hub_key_spelling` → *"The cockpit cannot load this release's pin: it names this repository `<binding.repoKey>` (t06:197), which is not a form the Hub accepts."* (task 09's HA7-6 cause, no promise it will arrive); anything else → *"The cockpit has not loaded this release's pin from the Hub yet. It is read in the background; this shows it when it arrives."* — never the sentence below, which would call a real pin absent (review M11) |
| `release == null`, `releaseLoaded` | *"No brief is recorded for this release. It was shipped outside any cockpit — auto-deploy on push, or the Panacea portal — so no brief was pinned when it shipped, and none is pinned now."* (prototype `:783-784`, corrected for OD-3 as task 15's footnote is) |
| `pin.status "none"` | *"No brief is recorded for this release."* + task 15's reason line for `pin.reason` |
| `pinned`, phase 4 merged | `<ShippedManifest refValue state head version={row.version ?? "This release"} designKey={release.designKey ?? undefined} onOpenDiscover={openPast} />` from `usePinManifest(pin.briefKey)`. **No `mergeRef`**: phase 7 holds no merge link (see *does NOT claim*) |
| `pinned`, phase 4 merged, `headRevisionAtShip` ≠ `refValue.revision` | one line under it: *"When it shipped, the brief's head was revision `<headRevisionAtShip>`. The Hub pinned the latest complete revision (ADR-H1 Q1)."* (plan §3.2 Q1) |
| `pinned`, phase 4 **not** merged (the fallback) | task 15's `<PinCell row>` at drawer size, and *"The brief's text shows here once Discover is built. The pin itself is recorded; only its text is not shown yet."* No criteria, no link |

Criteria are `ShippedManifest`'s to draw: every one `open`, *"no evidence recorded yet"*, until
phase 3's evaluator exists (seam S7-P1). The drawer adds no criterion words of its own.

**The Release facts** (prototype `viewFacts`, `:864-881`, corrected for OD-3):

| Key | Value |
|---|---|
| version | `version`, or *"version unknown"* |
| commit | the 40-hex `commit` in mono, or *"not recorded"* |
| environment | `environment`, or *"—"* |
| shipped | `at`, absolute, `tabular-nums`; *"Reconstructed from Panacea's record"* when `reconstructed` |
| deployed by | `deployedBy` as Panacea recorded it, or *"not recorded"* |
| origin | task 15's origin rule: `release.reservedBy.displayName`, or *"outside any cockpit"* when `release == null` and `releaseLoaded`, or *"not loaded yet"* when `!releaseLoaded` |
| trigger | `trigger` verbatim |
| gates | one line per `gates` entry: name, `verdict`, and `criticals` / `highs` / `findings` where non-null |
| provisioning | one line per note: `component`, `status`, `detail` |
| deployment | `deploymentId` in mono |

#### Scans — `ScanList`

One `api.scans(projectId, row.environment, row.deploymentId)` when it mounts — the
`ReleaseRow`'s raw `environment` (task 08: the Deployment's own word, `"prod"` for production),
never the card's name or `environmentRow` (review C3) — again only on
**Try again**. **An empty or null `environment` makes no call** (re-review b03, M-1): Panacea
reads a missing `env` as production (`handlers/security_status.go:192-201`), so the answer would
be production's verdict drawn as this release's. `ScanList` shows instead *"Panacea recorded no
environment for this release, so its scans are not read."* and no **Try again**. From source this
is legacy-only: every current writer sets a non-empty environment label (`environmentForBranch`,
`clients/pipeline.go:2195-2253`; the webhook's `TargetEnvironment`, `models/event.go:116-148`;
advisory runs, `pipeline.go:842`). One row per
`scans[]`: name; a dot by status (`PASS` ok, `FAIL` fail, `SKIPPED` unmeasured, anything else
warn) and the status word, or *"no result"* when it is empty; `mode`, with `enforce` marked; the
counts, else `errorReason`, else `skipReason`, else `summary`. Under the table, always: *"The
cockpit sends no scan modes: each scan runs as the project sets it. `off` means the scan did not
run — never a pass."* (prototype `:722`). `detailsSource "unverified"` → *"Findings not
attributable to this release."*; `detailsRetained false` → *"Panacea kept no finding details for
this release."* A failure shows its `ApiError.message` and **Try again** — never an empty table.

#### Why it is down — `DiagnosePanel`

On mount, `api.pods(projectId, env.name)` and `api.events(projectId, env.name)` in parallel; again
only on **Refresh**. **Diagnose** calls `api.diagnosis` once per press — never on mount, because
Panacea's diagnosis may consult its AI and then answers `source "rules+ai"`
(`handlers/diagnose.go:650-674`) — a model call on Panacea's side for every read.

- Pods: name (mono), `phase`, ready as a dot plus *"ready"* / *"not ready"*, `restarts`
  (`tabular-nums`), `reason` and `message` verbatim.
- Events: newest first by `lastSeen` (null last), `Warning` with a warn dot, *"×`count`"* when
  `count > 1`, `object` in mono, `message` verbatim.
- Diagnosis: `summary`, then each finding as *problem*, *likely cause*, *suggested fix* and its
  `evidence` lines. `source "rules+ai"` → *"Part of this reading is Panacea's AI. Check it against
  the events above."*
- `stale` → every button off with *"Off while Panacea is not answering."* (task 16's sentence).

#### The log pane — `LogPane` and `drillStream`

- **One stream.** `LogPane` calls `openDrillStream` on mount and `close()` on unmount. A second
  `LogPane` (another row, or readiness after build log) replaces the first; the first shows
  *"Stopped: another stream is open."* with **Follow again**. The drawer closing unmounts it.
- **~10 Hz.** Frames go into a buffer; one `setTimeout(flush, FLUSH_INTERVAL_MS)` is armed per batch
  — the constant exported by `api/attachRegistry.ts:17` (100 ms), imported, not copied — and never
  a `setInterval`. One React state update per flush, however many frames arrived.
- **Never scroll-jump.** *Pinned* is measured **before** the flush's DOM write: `scrollTop +
  clientHeight >= scrollHeight - 4`. Pinned → scroll to the bottom after the write; not pinned →
  `scrollTop` untouched, and a quiet *"`N` new lines · Jump to latest"* counts what arrived below.
- **Bounded.** The pane keeps the last 5,000 lines; above that the oldest drop, with the line
  *"Earlier lines are not kept here."* at the top.
- **Log frames**: `step` in a fixed-width mono gutter, `line` verbatim as text (never HTML).
  **Readiness frames**: one status line, replaced in place — *"`ready` pods ready"*, or *"Not
  measured"* when `measured` is false (never 0), plus `crashReason`, *"restarted `restarts` times"*,
  *"out of memory"* when `oomKilled`, *"exit `exitCode`"*.
- **Ends**: `end` → *"The stream ended."*; `failed` → the message, and **Follow again**; `closed`
  draws nothing.

#### Tests to write first (`vitest` + Testing Library, jsdom; fake timers for the flush only)

| Test | Assertion | Mutant it kills |
|---|---|---|
| `pin comes first` | a pinned row → the first child of the drawer body is the pin block, above design and facts | a manifest that leads with the build and buries the thread |
| `pinned renders the pinned ref` (phase 4 merged) | the store holds ref rev 5 and head 6 → `ShippedManifest` receives `refValue.revision 5`, and the drawer's text nowhere says *"rev 6"* without *"brief now at"* | rendering a release from the head (P1) |
| `no brief is shown, never hidden` | `release: null` → the outside-any-cockpit sentence; `release: null, releaseLoaded: false` → the not-loaded sentence and **not** the outside-any-cockpit one (review M11), with `releasesState "signed_out"` the sign-in sentence and `"hub_key_spelling"` the key-spelling sentence, neither containing *"when it arrives"* (plan re-review b09-1, M-2); `pin.status "none"`, `no_complete_revision` → task 15's reason line | an absent pin drawn as an empty block (P9); a pin not yet read drawn as absent; a promise that the pin will arrive when nothing is being read, or when it never can |
| `fallback is honest` (phase 4 absent) | the fallback sentence and a `PinCell`; no criteria list; no Discover link | a fallback that fakes the brief's text or a link that goes nowhere |
| `head at ship is shown when it differs` | `headRevisionAtShip 7`, ref rev 6 → the Q1 line with *"7"*; equal → no line | hiding that the Hub skipped an incomplete head |
| `no merge link is claimed` | `ShippedManifest`'s props have no `mergeRef`; the DOM lacks *"merge"* | inventing a merge link phase 7 never read |
| `design key verbatim` | `designKey "design:cv-…@11"` → exactly that text in mono; the component source has no `split(` / `match(` on it | parsing a key outside its owner (P5) |
| `scans read once` | mount → one `api.scans` call; 60 s of fake time → still one; **Try again** → two | a scan poll per open drawer |
| `scans read in the row's environment` | a row `{deploymentId:"d-feat", environment:"feat-x", environmentRow:"feat-x"}` → `api.scans("p-1", "feat-x", "d-feat")`; a row `{environment:"prod", environmentRow:"production"}` → `"prod"`, never `"production"` (review C3) | dropping the environment, which reads production's verdict for a branch release, or sending the card's name |
| `no environment, no scan read` | a row `{deploymentId:"d-old", environment:""}` and one with `environment:null` → zero `api.scans` calls; the text *"Panacea recorded no environment for this release, so its scans are not read."*; no **Try again**; the DOM lacks *"Pass"* and every status word (re-review b03, M-1) | `row.environment ?? ""` sent as-is, which draws production's verdict as a legacy release's own |
| `off is not a pass` | a scan with `status "SKIPPED"`, `mode "off"` → the unmeasured dot and the footnote; the DOM lacks *"pass"* for it | absence read as clean |
| `prod row keeps its drill-ins` | `row {environment:"prod", environmentRow:"production", isCurrent:true}`, `view.environments` holds `production` serving `{measured:true, ready:0}` → *Why it is down* is open and **Follow readiness** is offered for `production` | `e.name === row.environment`, which found no row for any production release (review I8) |
| `rollback target keeps its drill-ins` | `view.environments` holds `feat-x` with `current {source:"rollback", deploymentId:"d1"}`, serving `{measured:true, ready:0}`; `d1`'s run row `{kind:"run", deploymentId:"d1", environmentRow:"feat-x", isCurrent:false}` → *Why it is down* is open, **Follow build log** and **Follow readiness** are offered; the rollback row `{kind:"rollback", deploymentId:null, environmentRow:"feat-x", isCurrent:true}` → *Why it is down* and **Follow readiness**, no **Follow build log**; an older succeeded run row `d0` of `feat-x` → neither (plan re-review b04-1, I-1) | gating on `isCurrent` alone, which leaves no row offering **Follow readiness** after a rollback |
| `unmatched row has no drill-ins` | `environmentRow: null` → neither *Why it is down* nor **Follow readiness**, and the *"Panacea lists no environment named"* line; with `environment: ""` too → *"Panacea recorded no environment for this release."* and no *"named"* line (review M5) | a drawer that guesses a row by name |
| `not found says so` | a row `{kind:"not_found", deploymentId:"r9", release:<pinned>}` → the heading ends *" · not found in Panacea"*, the **Not found** sentence holds `r9`, the pin block is drawn, and zero `api.scans` calls, no **Follow build log**; the same row as `kind "pending"` → *" · pending"* and no *"not found"* (review M18) | a run Panacea no longer lists drawn as pending for ever, or a scan read for a run that does not exist |
| `diagnose only on press` | mount → `pods` and `events` once each, `diagnosis` zero; press **Diagnose** → one | an AI call on Panacea's side every time a drawer opens |
| `one stream in the window` | open `run-logs r1`, then `readiness env-a` → the first handle's `onEnd("replaced")` once, its `AbortSignal` aborted, `currentDrillTarget()` is `env-a` | two live streams — a second clock (plan 7P-14) |
| `onEnd exactly once` | `close()` twice, then an `end` frame → `onEnd` called once, with `"closed"` | a double "stream ended" line, or a leaked listener |
| `flush at ten hertz` | 50 frames inside 100 ms → one state update; `LogPane.tsx` and `drillStream.ts` contain no `setInterval`; `FLUSH_INTERVAL_MS` is imported from `api/attachRegistry` | a render per frame, or a copied constant that drifts |
| `never scroll-jump` | scroll up 200 px, deliver frames → `scrollTop` unchanged and the *"new lines"* count shows; at the bottom → it follows | yanking the reader back to the tail (DESIGN-SYSTEM `:65`) |
| `readiness unmeasured is not zero` | a frame `measured:false, ready:null` → *"Not measured"*, never *"0 pods ready"* | nil painted as 0 |
| `bounded` | 6,000 lines → 5,000 in the DOM and the *"not kept"* line | a pane that grows until the window dies |
| `closing the drawer closes the stream` | open the drawer, follow the log, press ✕ → the stream's signal aborted | a stream left open behind a closed drawer |

#### Steps

- [ ] Check first: `test -f desktop/src/provenance/ShippedManifest.tsx`. Present → write
  `usePinManifest` and the phase-4 branch of `PinManifest`. Absent → write only the fallback
  branch, and say which in the commit message. The same check again when phase 4 merges.
- [ ] Tests first; `drillStream.ts`; `LogPane`; `ScanList`; `DiagnosePanel`; `PinManifest`;
  `ReleaseDrawer`; the `DeployLens` open-row state and mount; the CSS.
- [ ] `npx tsc --noEmit`, `npx vitest run`; commit by pathspec with *"Visual acceptance pending the
  owner (OD-12)."*

#### Acceptance

```
cd desktop && npx tsc --noEmit         → no output
cd desktop && npx vitest run           → all pass
```
**Owner's eyes (batched):** open a pinned release, a superseded one and one with no brief; follow a
build log while scrolled up; open *why it is down* on a crash-looping environment. **Unverified
until then.**

#### What this task does NOT claim

- That the drawer shows a **merge link**. Phase 4's `ShippedManifest` accepts one (`mergeRef`), but
  no route phase 7 reads carries it — the Hub contract has no merge record (§7.10 holds evidence
  only). Seam S7-P2 in task 20.
- That criteria are met or unmet. They are `open` until phase 3's evaluator (S7-P1).
- That Panacea's diagnosis is right. It is Panacea's reading, labelled as Panacea's.
- Pixel fidelity to the prototype. The owner judges.
