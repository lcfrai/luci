### Task 26: The cockpit shows cloud machines — states, switching, implicit wake, the badge, signed-out

HELD until task 24b lands, which waits for phase 3's tasks 03 to 05 on feat/env (plan P3A-6; `.superpowers/sdd/env/held.md`): the build stream skips this task until then.

`docs/FUNCTIONAL-SPEC.md:42-48`: one cloud machine exists at first sign-in "with no create form";
six states; **wake is implicit**, "with inline 'waking… 6s'"; the local machine "appears in the
same list with an honest list of what it cannot do … badged `unsandboxed`"; the rail has "machine
chip at top, then projects → branches → worktrees → sessions". Phase 2 built the chip for one
machine (`desktop/src/components/rail/MachineChip.tsx:1-64`), the six-state dot
(`components/common/StatusDot.tsx:16-27` — `provisioning` and `sleeping` are rings, `running`
and `idle` discs, `stopped` and `error` squares), and a rail that shows only the default machine
(`components/rail/Rail.tsx:16`, `store/selectors.ts:32-33`). This task makes it many machines.

**The only UI task in this phase.** It follows `docs/DESIGN-SYSTEM.md`, and the parts of it that
bind here are named at each point below. **Visual acceptance is the owner's eyes** (OD-12; `CLAUDE.md`
"Working agreements"): tests prove structure and behaviour; nothing here claims the result looks
right, and no screenshot is offered as evidence.

**Files:**
- `desktop/src/store/types.ts` **and** `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` §2 — amended together, byte-identical (rule SX.2) (modify)
- `desktop/src/api/client.ts` — the new calls; the comment at `:6-8` (S8.6) (modify)
- `desktop/src/api/poller.ts` — the machines snapshot, projects for the current machine, ensure-default once (modify)
- `desktop/src/store/store.ts` — `selectedMachineId` (persisted: version bump and migration, rule SX.3), `cloud`, `wakingSince` (modify)
- `desktop/src/store/selectors.ts` — `selectCurrentMachine`, `selectCloudMachines` (modify)
- `desktop/src/components/rail/MachineChip.tsx`, `MachinesPopover.tsx` (new), `rail.css` (modify)
- `desktop/src/components/common/common.css` — the reduced-motion form of a live ring, and the
  comment at `:108-118` that lists the static forms (modify; this file is the one owner of
  `.dot-*`, `common.css:3-4`; b09 re-review 1)
- tests beside each (new/modify), and `desktop/src/hooks/useCreateSession.test.ts` (new; the
  `machine_full` case, review I5; `useCreateSession.ts` itself does not change)

#### Contract — the TypeScript additions (complete; they go into `types.ts` and UI contract §2 verbatim)

```ts
/** GET /v1/machines' "cloud" block (task 24a). Absent from a server with no broker. */
export type CloudStatus = "ok" | "signed_out" | "unavailable";

export interface CloudState {
  status: CloudStatus;
  /** One sentence for signed_out / unavailable, rendered verbatim. "" when ok. */
  message: string;
}

export interface MachinesSnapshot {
  machines: Machine[];
  cloud?: CloudState;
}

// Project, Branch and Worktree each gain, as their last field:
//   /** The machine the object lives on. Absent from a phase 2 server; read as "local". */
//   machineId?: MachineId;
```

```ts
// client.ts — additions to the client interface; every existing member keeps its signature.
machinesSnapshot(): Promise<MachinesSnapshot>;                         // GET /v1/machines, whole body
listProjects(machineId?: MachineId): Promise<Project[]>;              // ?machineId= when not "local"
listBranches(projectId: ProjectId, machineId?: MachineId): Promise<Branch[]>;
listWorktrees(projectId: ProjectId, branchId?: BranchId, machineId?: MachineId): Promise<Worktree[]>;
ensureDefaultMachine(): Promise<Machine>;                              // POST /v1/machines/default
wakeMachine(id: MachineId): Promise<Machine>;                          // POST /v1/machines/{id}/wake
```

```ts
// store.ts — state added.
selectedMachineId: MachineId | null;   // PERSISTED. null = the default machine
cloud: CloudState | null;              // not persisted; null = no broker configured
wakingSince: Partial<Record<MachineId, number>>; // not persisted; performance.now() at wake
selectMachine(id: MachineId | null): void;
```

**Persistence (SX.3).** `selectedMachineId` is a new root key, so `PERSIST_VERSION` bumps from
its value **at the time this task runs** (1 on `3303fd6`, `store/store.ts:17`; phases 3, 4 and 5
may have bumped it — phase 2 task 26 rule 2, `phase-2-parts/task-26.md:922-925`: sequence the migrations)
to the next integer, with a migration that **adds** `selectedMachineId: null` and drops nothing.

#### Behaviour

- **The current machine** is `selectedMachineId` if the snapshot still has it, else the default
  (`selectDefaultMachine`). A selection pointing at a deleted machine falls back silently.
- **The rail** lists the current machine's projects: the poller calls `listProjects(current)`,
  and branches/worktrees with the same id. Switching machine is instant in the chip and fills the
  rail on the next poll tick; the previous machine's rows are not shown meanwhile (no stale mix).
- **Ensure-default, once.** When a snapshot arrives with `cloud.status === "ok"` and no machine of
  `kind === "cloud"`, the poller calls `ensureDefaultMachine()` — once per app run, guarded by a
  module flag, never on every tick, never when signed out. That is "exists at first sign-in with
  no create form".
- **The chip** (`MachineChip`): the current machine's name, its `StatusDot` (`live` when
  `running`, and while waking: the `provisioning` ring is the live signal below), and the `unsandboxed` badge exactly when `isolation !== "microvm"` (unchanged,
  `MachineChip.tsx:45-47`) — so a cloud machine carries no badge (S8.5) and the local one keeps it.
- **Waking.** From a wake (explicit, or a session create on a sleeping machine — task 24b wakes it
  server-side, and the client records `wakingSince` when it sends that create) until the machine
  reports `idle` or `running`, the chip shows `waking… {n}s` in a `span.rail-waking`, `n` from
  **one** `useHeartbeat()` subscription in the chip (`hooks/heartbeat.ts:64`, seam SX.8), in
  `var(--numeric)` (tabular numerals, so the digits do not jitter). **No spinner** — the ring dot
  of `provisioning` is the one live signal, on the global clock.
- **Reduced motion (review m18; `DESIGN-SYSTEM.md:90`).** `tokens.css` cuts every
  `animation-duration` to `0.01ms` and every iteration count to 1 under
  `prefers-reduced-motion: reduce` (`tokens.css:178-179`, inside the
  `@media` block that opens at `:174`), so the waking ring's breath stops. A waking
  (`provisioning`) and a sleeping machine are then both `dot-ring`, and would differ by colour
  alone.
  - **Today's CSS does not separate them, and this task fixes that (b09 re-review 1).** `.dot-ring`
    draws its ring as `box-shadow: inset 0 0 0 1.5px currentColor` (`common.css:53-57`). The
    reduced-motion rule `.dot-live { box-shadow: 0 0 0 1.5px currentColor; }`
    (`common.css:119-123`) has the same specificity and comes later, so it **replaces** the inset
    ring rather than adding a halo to it. That halo was designed for discs (`:108-118`), and no
    `live` dot was a ring before this task (only `running`, a disc, is `live` today:
    `MachineChip.tsx:41`, `SessionRow.tsx:54`, `BranchRow.tsx:76`). A waking dot would be a hollow
    ring stroked outside its box, a sleeping dot a hollow ring stroked inside it: the same shape.
  - **The fix, in `common.css`'s reduced-motion block**, after the `.dot-live` rule:
    `.dot-ring.dot-live { box-shadow: inset 0 0 0 1.5px currentColor; }` (specificity 0,2,0, so
    the ring keeps its inset stroke and its 8px size) and
    `.dot-ring.dot-live::before { content: ""; position: absolute; inset: 2.5px; border-radius:
    50%; background: currentColor; }` (a 3px centre point, 1px inside the ring). Under reduced
    motion a waking machine is a ring with a centre point, and a sleeping one is a hollow ring:
    a shape difference, not a colour one. `.dot` is already `position: relative` (`:40`), and
    `::after` stays the flash's (`:96`), so `::before` is free. Both declarations pass
    `desktop/src/styles/designsystem.test.ts`'s audits as they stand: the shadow carries
    `currentColor` (`:431`, test `:549`), and there is no gradient (`:239`, test `:555`); a
    `radial-gradient` centre point would fail the second, which is why this is a pseudo-element.
    The `.dot-live` rule stays as it is, so `StatusDot.test.tsx:88` (the disc halo, non-inset)
    stays green, and `:532`'s looping-animation stand-in check still finds `.dot-live` in the
    block. The comment at `:108-118` gains the fifth form: "ring with a centre point
    (provisioning, while waking)".
  - **The counter is a second, textual difference.** It is text on the JS heartbeat
    (`heartbeat.ts:29-34`, a `setInterval`), which reduced motion does not stop, so
    `waking… {n}s` keeps counting: it is information, not motion. A sleeping chip renders no
    `span.rail-waking`. With the dot's `aria-label`/`title` carrying the state word, that also
    covers WCAG 1.4.1 if the shape were ever lost. `rail.css` gives `.rail-waking` no `animation`
    and no `transition`.
  - The look of the centre point is the owner's to accept (OD-12); the test pins the rule, not
    the rendering.
- **The machines popover** (`MachinesPopover`, opened by the chip and by `⌘⇧M`): one row per
  machine — dot, name, state word, the badge rule — and the current row marked with the accent
  (interactive/current state only). Selecting a row calls `selectMachine`. A sleeping cloud machine
  shows a **Wake** button. The current machine's `limitations` render verbatim, never summarised.
  When `cloud` is `signed_out` or `unavailable`, one line renders `cloud.message` verbatim under
  the rows; if the Hub's C4 sign-in surface exists, the line carries its sign-in action, and
  otherwise nothing is invented.
- **Problems.** `ensureDefaultMachine` and `wakeMachine` failures go through `classify`
  (`api/problems.ts:13,37`, seam SX.7) with their own sentences, switched at the call site with a
  declared return type and no `default:` arm; `quota_exceeded` gets the sentence "You are at your
  cloud machine limit."
- **A full machine (review I5).** A session create on a cloud machine whose node has no room
  answers `409 machine_full` with one of the control plane's three sentences (task 24b, chosen by
  the count; for 3: "This cloud machine's node has no room for another session right now (it
  already runs this machine's 3 sessions), so the session was not created. Stop one of them, or
  try again later."). The cockpit renders it verbatim through `useCreateSession`'s existing
  `server-said` arm (`hooks/useCreateSession.ts:37-46`), within seconds of the click (the broker
  refuses on the scheduler's verdict, task 18), never after a two-minute wait. It needs no new
  `ProblemKind`, and no `ProblemCode` member: `toProblem` passes an unknown server code through
  intact (`api/client.ts:199-207`), so `types.ts` and UI contract §2 do not change for it.

#### DESIGN-SYSTEM rules this task must meet (each checked by a test or by the owner)

| Rule (`docs/DESIGN-SYSTEM.md`) | Here |
|---|---|
| "One global heartbeat clock … Never a spinner per row" (`:61-62`) | the waking counter and the ring dot use `useHeartbeat`/`beatDelayStyle` only; test asserts no element with a spinner role or class |
| "Tabular numerals … on every live number" (`:64`) | the seconds counter carries the numeric token; `designsystem.test.ts`'s `var(--numeric)` guard covers it |
| "Accent … only for interactive and current state" (`:38`) | the accent marks only the current row and focus rings; no accent on the cloud badge, the dot or the popover frame |
| "Elevation is a 1px ring plus a layered drop shadow" / radius by size (`:27-29`) | the popover reuses `Popover` (`components/common/Popover.tsx`) and its existing elevation tokens; rows use the control radius (8) |
| "Motion must explain causality … transform/opacity only; never `transition: all`" (`CLAUDE.md`; `:63`, `:90`) | the only motion is the state-change flash `StatusDot` already owns (600 ms, opacity) |
| "`prefers-reduced-motion` variants on everything" (`:90`) | under reduced motion the waking ring keeps a static, non-colour difference from a sleeping one (the `.dot-ring.dot-live` rules add a centre point and keep the inset ring; `.dot-live`'s halo alone would replace the ring, b09 re-review 1), a sleeping chip has no `waking…` text, the counter keeps counting, and `.rail-waking` has no motion of its own; test `MachineChip / waking respects prefers-reduced-motion` (review m18) |
| "Hover never changes layout" (`:73`) | row hover changes background only |
| "Every state designed: loading, empty, … offline, unauthorized, error" (`:90`) | signed-out, unavailable, provisioning, sleeping, error rows each have a test that renders them |

#### Tests to write first (vitest + Testing Library; `jest-dom` is not installed — assert DOM properties)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `MachineChip / a cloud machine has no unsandboxed badge` | `isolation: "microvm"` → no `.rail-badge` | the badge hidden client-side for the wrong reason, or shown on a microVM (S8.5) |
| `MachineChip / the local machine keeps its badge` | `isolation: "container"` → `.rail-badge` with text `unsandboxed` | honesty about the local machine lost |
| `MachineChip / sleeping renders a ring` | the dot has the ring shape class | a sleeping machine that looks running |
| `MachineChip / waking counts on the heartbeat, in tabular numerals` | fake timers; `wakingSince` set; after two ticks the text is `waking… 2s` and the counter element carries the numeric token class | a `setInterval` of its own (a second clock) |
| `MachineChip / no spinner while waking` | no element with `role="progressbar"` or a spinner class | a spinner per row |
| `MachineChip / waking respects prefers-reduced-motion` | jsdom applies no media query, so this reads the CSS source (`?raw`), as `StatusDot.test.tsx:82-88` does: (1) the waking dot has `dot-ring` **and** `dot-live`; the sleeping dot has `dot-ring` and not `dot-live`; inside `common.css?raw`'s `@media (prefers-reduced-motion: reduce)` block (the regex `StatusDot.test.tsx:82-84` uses), a rule whose selector is exactly `.dot-ring.dot-live` declares a `box-shadow` that starts with `inset` (the ring survives), and a rule whose selector is exactly `.dot-ring.dot-live::before` declares `content` and `background: currentColor` (the centre point). This clause is red on today's `common.css` (b09 re-review 1: the old clause, "`.dot-live` has a non-inset `box-shadow`", passed on today's CSS and pinned the false premise). The sleeping chip renders no `span.rail-waking`; the waking chip renders one. (2) With `window.matchMedia` stubbed to match `(prefers-reduced-motion: reduce)` and fake timers, after two ticks the text is still `waking… 2s`. (3) No rule in `rail.css?raw` whose selector names `.rail-waking` declares `animation*` or `transition*` (review m18) | a waking dot that is not `live`, so under reduced motion it matches a sleeping one by colour alone; the `.dot-ring.dot-live` rules deleted, so `.dot-live`'s halo replaces the ring and waking and sleeping are the same hollow ring; the `::before` centre point dropped; a `waking…` text on a sleeping chip; a counter that freezes under reduced motion (a `matchMedia` check that stops the count); a pulse on the counter |
| `MachinesPopover / lists every machine and marks the current one` | two rows; the current row has `aria-current="true"` | a popover that only ever shows the default |
| `MachinesPopover / selecting a machine switches the rail` | click the cloud row → `selectedMachineId` set; the next poll calls `listProjects("m-…")` | the rail stuck on local projects |
| `MachinesPopover / signed out shows the server's sentence verbatim` | `cloud.status: "signed_out"`, message `M` → the text `M` exactly, no cloud rows | a client-invented sign-in message |
| `MachinesPopover / wake is offered only for a sleeping cloud machine` | Wake button present for `sleeping`, absent for `idle` and for `local` | a wake verb on the laptop |
| `useCreateSession / a full machine shows the server's sentence verbatim` | three cases, each `S` one of task 24b's forms copied exactly: `ApiError("machine_full", S3, 409, {sessionsPerMachine: "3"})`, `S1` with `"1"` (the singular form), and `S0` with no details → the surfaced text is that `S` exactly each time, not the Docker or control-plane sentence and not re-pluralised from the count (review I5; b05 re-review 6) | `machine_full` classified as `backend-down`, which tells a cloud user "Docker is not running" |
| `poller / ensure-default is called once, and only when signed in` | three `ok` snapshots with no cloud machine → one call; `signed_out` → none | a POST on every tick |
| `store / the migration adds selectedMachineId and keeps everything` | a vN persisted blob → vN+1 with `selectedMachineId: null` and every old key equal | a migration that drops user state |
| `selectors / a vanished selection falls back to the default` | `selectedMachineId` not in `machines` → the default | a blank rail after a machine is deleted |
| `contract / types.ts equals UI contract §2` | the existing SX.2 byte gate passes after both are amended | the two drifting |

#### Steps

- [ ] **Step 1:** Read the current `PERSIST_VERSION` and the UI contract §2; confirm tasks 24a and 24b have
  landed (`GET /v1/machines` has `cloud`).
- [ ] **Step 2:** Amend UI contract §2 and `types.ts` together; the SX.2 gate passes.
- [ ] **Step 3:** Tests; client, poller, store, selectors; chip; popover; CSS through tokens only.
  The reduced-motion clause (1) goes red first, then `common.css` gains the two
  `.dot-ring.dot-live` rules and the fifth comment form (b09 re-review 1).
- [ ] **Step 4:** Update `client.ts:6-8`'s comment: the desktop hop's authentication is the Hub's
  local caller key (ADR-H1 §5.4, `67609ab` `:439-449`, its task C3); the user's Entra token never enters the
  webview and travels control plane → broker only.
- [ ] **Step 5:** `npm run typecheck`, `npm test`; commit. **Leave the branch unmerged for the
  owner's review** (OD-1: "UI branches stay unmerged until the owner has looked at them").

#### Acceptance

```bash
cd desktop && npm run typecheck        # exit 0
cd desktop && npm test                 # every suite passes, the 15 above among them
```

**Owner's eyes, at the batched hands-on session (OD-12), not before:** the chip on a cloud
machine and on the local one; the popover with both; a sleeping machine woken by typing, the
"waking… Ns" line and its ring; the same with the OS's reduce-motion setting on (the ring with
a centre point); the signed-out line. Until then each is **unverified**.

#### What this task does not claim

- The sign-in button: the Hub's C4 (ADR-H1 §5.9, `67609ab` `:567`).
- Machine sizing, a second-machine form, or delete: FUNCTIONAL-SPEC asks for sizing "only … for a
  second machine, or after an OOM"; the broker supports a second machine (task 19) and the
  cockpit surface for it is later.
