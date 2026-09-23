# Survey — the phase-2 substrate, as it actually exists

**Date:** 2026-09-23. **Method:** read the code, not the plans. Every claim below cites a file and a
line range I opened. Where I am inferring rather than reading, the sentence says so.

**Scope note.** `desktop/`, `sandboxd/`, `control-plane/`, `docs/superpowers/contracts/` and
`docs/superpowers/plans/` were read-only for this pass and another agent was editing them while I
read. Anything I quote is a snapshot of the tree at roughly 2026-09-23 12:40.

---

## 0. The headline, in four sentences

The domain model, the store, the typed client and the token layer are **the stable part** and later
phases should build on them without hesitation. The lens shell (`AppShell.tsx`) is **the one
guaranteed collision** — phases 4, 5 and 7 each have to edit the same `switch` in the same file, and
one of its six arms already carries a wrong phase number. The provenance thread that `CLAUDE.md`
calls the product spine **exists end to end as a field and is wired to nothing** — `Session.contextRef`
is persisted in Go, returned on the wire, typed in TS, carried through resume and fork, and **written
by no surface and read by no surface**. And the cockpit **has never run against a live control plane**:
`App.tsx` is still the placeholder, so everything below is compile-and-test-proven only.

---

## 1. Age and motion: the whole substrate is one day old

`git log --since="2026-09-22 12:30"` returns **41 commits**, and every file this survey covers was
either created or modified inside that window.

| Time (09-23) | Commit | What it did to the substrate |
|---|---|---|
| 01:49 | `2b6fccf` | `tokens.css` — qualified what `--ink-3` clears, per surface |
| 02:06 | `a3dad91` | **created** `store/types.ts`, `lib/ids.ts`, `lib/immutable.ts`, `lib/time.ts` |
| 02:11 | `32b51af` | `types.ts` re-made byte-identical to the contract |
| 02:39 | `dd256ca` | **created** `api/client.ts`, `api/index.ts` |
| 02:59, 03:03 | `bd49d12`, `7d90e06` | `client.ts` — timeout held across the body read; no blank error sentence |
| 03:31 | `f724fd6` | **created** the store: `store.ts`, three slices, `selectors.ts`, `defaults.ts` |
| 03:58 | `a774b44` | all three slices + `selectors.ts` — dangling-ref hazards |
| 04:44 | `2e5cd00` | **created** `AppShell.tsx`, `SubRail.tsx`, `LensPlaceholder.tsx`, the rail, the top bar, `heartbeat.ts` |
| 05:22 | `f0ad388` | `AppShell.tsx` fix round 1 (ten items) |
| 06:46 | `4d2eb56` | **created** `api/poller.ts`, `api/attachRegistry.ts`; rewrote `Terminal.tsx` |
| 08:59 | `4282538` | `AppShell.tsx` — **the develop arm stopped being a placeholder**; created `develop/` |
| 09:56, 10:47 | `e4cea95`, `ad342c0` | task 20 fix rounds; `api/problems.ts` created in the first |
| 10:21 | `39744c4` | `selectors.ts` + `AppShell.test.tsx` |
| 11:19–12:23 | `5089b01`…`7b90504` | `CanvasLayout`; **`selectors.ts` and `prefsSlice.ts` changed again at 12:14** |
| 12:34 | `6bcc9af` | `docs/PROGRAM.md` — the phase order corrected |

**Read that last row against the one above it.** The phase order was corrected **eleven minutes
after** the last code change to the store, and the code was never swept for the renumbering. See
§4.1.

**Still moving right now**, by commit recency: `store/selectors.ts` and
`store/slices/prefsSlice.ts` (12:14, `5f53f4b`), `components/develop/*` (12:23, `7b90504`).
`store/types.ts`, `api/client.ts`, `api/problems.ts` and `styles/tokens.css` have not been touched
since the morning and are the settled end of the range.

---

## 2. The domain model — `desktop/src/store/types.ts`

**221 lines, no imports, no runtime code.** Twenty-nine exported declarations, every one a `type` or
an `interface`. The file is gated: `docs/superpowers/plans/phase-2-parts/task-15.md:280` runs

```
diff -u --strip-trailing-cr /tmp/types.contract.ts desktop/src/store/types.ts && echo IDENTICAL
```

against the fenced block at contract §2 (`2026-09-18-phase-2-ui-contract.md:353`). The
`--strip-trailing-cr` is load-bearing on this checkout — mixed line endings, `core.autocrlf = true`
in the system gitconfig, no `.gitattributes`.

**What this means for a later phase, and it is the sharpest constraint in the whole substrate:**
**you cannot add a field to `types.ts` without amending contract §2 in the same change**, or the
diff gate fails. `32b51af` ("keep types.ts byte-identical to the contract it is gated against") is
the phase discovering this the hard way. Budget the contract amendment as part of any phase-4/5/7
task that touches the model.

### What later phases will find already there

- **`Lens`** (`types.ts:173`) — `"discover" | "design" | "develop" | "files" | "merge" | "deploy"`.
  All six names already exist. Phases 4, 5 and 7 do **not** add a union member; they fill an arm.
- **`Session.contextRef`** (`types.ts:162-163`) — *"Opaque provenance handle from Phase 4. Stored,
  carried, never interpreted."* This is the provenance thread's anchor and it is already through the
  whole stack (§5).
- **`Effort`** (`types.ts:100`) — `"" | "low" | "medium" | "high" | "xhigh"`. **`max` is missing**
  and the omission is known: `progress.md:5175` records that Claude Code 2.1.274 in the pinned image
  offers five levels and `control-plane/internal/harness.go:133` offers four, ruled as a task-25
  ride-along because no Group D task may edit a `.go` file. If a later phase surfaces effort, it
  inherits this gap.
- **`SessionStatus`** (`types.ts:134`) — exactly four: `starting | running | idle | exited`. The
  other three of `docs/AGENTIC-UX.md`'s seven are underivable without reading PTY bytes, which
  `PROGRAM.md` forbids. **A later phase that wants "blocked on you" needs a new server signal, not a
  new client derivation.** `tokens.css:75,77` already defines `--need` and `--drift` for exactly
  that future, so the colour vocabulary will not have to be reinvented.
- **`MachineState`** (`types.ts:22`) — all six rendered, only two emitted before phase 8.
- **`HarnessId`** (`types.ts:93`) — `KnownHarnessId | (string & {})`, deliberately open so a
  server-added sixth harness parses.

---

## 3. The store's public surface

### 3.1 Shape

`CockpitState = DataSlice & UiSlice & PrefsSlice` (`store.ts:14`) and it is **flat by requirement,
not by style** (`store.ts:3-6`): zustand `persist` shallow-merges the rehydrated snapshot at the
root, so a nested `{ prefs: {...} }` would silently drop every key added after the snapshot was
written. **A later phase adding state adds root keys.**

`PERSIST_KEY = "panacea-app.cockpit"`, `PERSIST_VERSION = 1` (`store.ts:16-17`). `partialize`
(`store.ts:29`) persists twelve keys; `migrate` (`store.ts:53`) drops version 0 and is **total over
`unknown`** because it runs at module evaluation, before any error boundary exists. The comment is
explicit that future versions **translate, never drop**. **A phase-4/5/7 task that adds a persisted
key bumps `PERSIST_VERSION` and writes a real migration.**

| Slice | Keys a later phase will read |
|---|---|
| `dataSlice` | `machines`, `projects`, `branchesByProject`, `worktreesByBranch`, `sessions`, `sessionIdsByWorktree`, `harnesses`, `loading`, `lastError` + ten setters |
| `uiSlice` | `selection`, `lens`, `layout`, `railCollapsed`, three `expanded*` records, `paletteOpen`, `pickerOpen`, `contextMenu`, `sessionMenuFor`, `toasts`, `machinesOpen` + 22 actions |
| `prefsSlice` | `defaults`, `projectDefaults`, `branchDefaults`, `tiles`, `pinned` + seven actions |

`initialDataState`, `initialUiState` and `initialPrefsState` are **`Object.freeze`d containers and
all** (`dataSlice.ts:60-64`), deliberately, so a cross-store mutation is a loud TypeError.

### 3.2 The selector rule, and who may add one

Group D shared-shape **§4** (`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-d-shared-shape.md`):

> Tasks 18 through 24 all read the store. Task 17 declares the selectors and actions; nobody else
> declares one. A later task that needs a derived read **adds a selector to `store/selectors.ts`**
> rather than computing it inline in a component… Where a task needs one the contract does not name,
> it adds it to `selectors.ts`, gives it a doc comment saying which surface needs it, and says in its
> plan text that it is an addition to §3.7.

This is not ceremony. `selectors.ts:3-5` gives the mechanical reason: an inline `.filter()` inside
`useCockpit(...)` returns a new array on every store write and re-renders on **every 2 s poll**,
which is the layout shift `docs/AGENTIC-UX.md` §10 rule 7 bans.

**The rule has already been exercised three times** and each addition carries an in-file declaration:
task 18 added `selectProjectById` / `selectBranchById` / `selectWorktreeById` /
`selectBranchAttentionSessionId` (`selectors.ts:140-200`), task 20 added `selectSelectedBranch`
(`selectors.ts:210`), task 21 edited `selectCanvasSessionIds` **inside task 17's file as a declared
departure** (`selectors.ts:90-92`). So the precedent for a later phase is established and cheap:
add to `selectors.ts`, doc-comment which surface needs it, declare it in the plan text.

**Referential-stability conventions a later phase must copy**, or it will ship the re-render defect:

- Frozen module constants `NO_BRANCHES` / `NO_WORKTREES` / `NO_IDS` (`selectors.ts:27-29`) so the
  absent case is referentially stable.
- Selectors that build a fresh array or object — `selectPaneSessionIds`, `selectCanvasSessionIds`,
  `selectResolvedDefaults` — **must be consumed with `useShallow`**, and each says so in its doc
  comment.
- Id arrays, not object arrays, where possible: `dataSlice.replaceSessions` keeps the previous array
  when ids match, so an id array survives a no-op poll unchanged (`selectors.ts:45-46`).

`selectPaneSessionIds` (`selectors.ts:72`) is **THE ordering** — the selected branch's worktrees in
server order, each worktree's sessions in server order. `⌘1-9`, the palette and `ColumnsLayout` all
index this and nothing else. A later phase that needs "the sessions in view" reads this, and **never
re-sorts**: the client re-sorting is a §4.7 violation because the poll must move zero pixels.

---

## 4. The lens shell — the collision, described precisely

### 4.1 What is there

`desktop/src/components/AppShell.tsx` is **106 lines**. `LensRouter` (`:15-69`) is a single
`switch (props.lens)` with six arms plus a `default: return <></>`.

| Arm | Today | Renders |
|---|---|---|
| `develop` | **real** | `<DevelopLens />` (`:25`) |
| `discover` | placeholder | `<LensPlaceholder lens="discover" phase={4} what="Turn a conversation into a spec this branch can be measured against." />` (`:26-33`) |
| `design` | placeholder | `phase={5}`, *"A canvas of real HTML frames an agent can edit directly."* (`:34-41`) |
| `files` | placeholder | `phase={3}` (`:42-49`) |
| `merge` | placeholder | `phase={3}` (`:50-57`) |
| `deploy` | placeholder | **`phase={5}`** (`:58-65`) |

### 4.2 Defect found: the deploy placeholder tells the user the wrong phase

`AppShell.tsx:61` says `phase={5}` for the `deploy` arm. `docs/PROGRAM.md:57` (the table) and
`docs/PROGRAM.md:97` (the prose) both say **Deploy connector is phase 7**; phase 5 is the Design
canvas. `AppShell.tsx` was written at 04:44 (`2e5cd00`); `PROGRAM.md` was corrected at 12:34
(`6bcc9af`), and nothing swept the code. So the shipped UI currently renders *"deploy — Arrives in
phase 5"*, which is the phase Design arrives in.

Low severity — one word of copy — but it is **exactly** the shared-shape §3D failure mode ("grep
every mention of what you changed, not the section you were sent to") landing in code rather than in
the contract. The one-line fix belongs to whoever owns the renumbering, not to this survey; I wrote
nothing.

I checked the other four: `discover`→4, `design`→5, `files`→3, `merge`→3 all agree with
`PROGRAM.md`'s table. Deploy is the only one.

### 4.3 How a later phase fills an arm — the worked precedent

Task 20 did this for `develop` at `4282538`, and it left the recipe in the file (`AppShell.tsx:18-24`):

```
    case "develop":
      // ── TASK 20 FILLED THIS INSERTION POINT ───────────────────────────────────────
      // Develop is the one lens Phase 2 builds, so it is the one arm that is not a
      // placeholder. … Guard, still true:
      //   grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx  # 1
      // ───────────────────────────────────────────────────────────────────────────────
      return <DevelopLens />;
```

So the per-phase edit is exactly four things:

1. One `import { XLens } from "./x/XLens";` at the top of `AppShell.tsx`.
2. Replace the `<LensPlaceholder .../>` in that arm with `<XLens />`.
3. Add an anchored guard of the same shape (`^[[:space:]]+return <XLens`, expect 1) — shared-shape
   §3A requires the anchor so the file's own comments cannot satisfy the grep.
4. Update `AppShell.test.tsx`, which asserts placeholder text — `:112` currently pins
   `/Arrives in phase 3\./`.

**A lens owns its own subtree and nothing else.** `AppShell` owns the grid, the grain layer, the
one keyboard-map mount and the overlays, and explicitly owns **no focus and no scroll** (`:1-2`,
citing §6.8). `.shell-lens` is `min-block-size: 0; overflow: hidden` (`shell.css:29-32`), so a lens
is handed a fixed box and does its own scrolling inside it.

### 4.4 The collision, stated as a scheduling fact

**`AppShell.tsx` is the single file phases 4, 5 and 7 must all edit, and they edit three different
arms of the same `switch` plus the same import block.** Three parallel worktrees each adding an
import line at the top and replacing a different `case` body is a **guaranteed textual conflict in
the import block** and a clean auto-merge everywhere else. It is small — a handful of lines — but it
is certain, not probable.

Two more files have the same property, one certain and one likely:

- **`AppShell.test.tsx` — certain.** Each phase changes what its arm renders, and the placeholder
  assertions live in one file.
- **`SubRail.tsx` — likely, and cheap.** `LENSES` (`:9-16`) is a flat array of
  `{ id, label, glyph }`. Phases 4/5/7 do not add rows (all six exist) but any of them may want to
  change a glyph or a label for its own lens. Same file, different array element.

Two files that phases 4/5/7 will probably **not** need to touch, which is the good news:
`store/types.ts` (the `Lens` union is complete) and `subrail/subrail.css` (the row styling is
lens-agnostic).

### 4.5 One more shared edit point, further out

`components/palette/commands.ts` does not exist yet — it is **task 23's**, still unbuilt. Whatever a
later phase wants in the command palette ("Open Discover", "Attach the spec to this release") is an
edit to a file that does not exist today. **Do not plan against its shape; plan against the fact that
it will exist and will be a single shared registry.**

---

## 5. The typed client — `desktop/src/api/client.ts`

**393 lines.** One exported function per Go endpoint. `CockpitClient` (`:131-165`) is the interface;
`createClient(opts?)` (`:233`) builds it; `api/index.ts:17` constructs **one** singleton
(`export const api = createClient()`) and the comment is explicit that a second client is a second
base URL and a second timeout.

- `DEFAULT_BASE_URL = "http://127.0.0.1:8730"` (`:30`), overridable by `VITE_PANACEA_APP_API`.
- `DEFAULT_TIMEOUT_MS = 10_000` (`:33`), per request; `/attach` is not a fetch and is unaffected.
- **No `Authorization` header, deliberately, until phase 8** (`:6-8`): the boundary on the desktop
  hop is a loopback bind, a Host guard and an Origin allowlist, all server-side. **A later phase must
  not "add auth for safety" here** — it is a phase-8 decision with a server half.

### 5.1 `ProblemCode` — count it correctly

`client.ts:40-48`. **The union has eighteen members**, not sixteen: **sixteen** the control plane
emits (`invalid_json`, `bad_request`, `unknown_harness`, `unsupported_option`, `worktree_not_found`,
`project_not_found`, `session_not_found`, `forbidden_origin`, `forbidden_host`, `conflict`,
`sandbox_not_running`, `harness_unavailable`, `upgrade_required`, `backend_unavailable`, `internal`,
`non_loopback_publish`) plus **two the client raises itself** (`network_unavailable`, `timeout`,
`:46-48`). Both `problems.ts:25` and shared-shape §3A.1 clause 6 say "eighteen"; the "sixteen" in the
brief I was given is the server-emitted subset. Worth pinning because an exhaustive `switch` written
against sixteen will not compile.

`non_loopback_publish` is called out twice (`:38-39`, `problems.ts:18-21`) as the trap: **the sandbox
is running** and published its port off loopback, so it must never be rendered as a "not running"
state.

`toProblem` (`:190`) casts an unrecognised server `error` string **into** the closed union rather
than flattening it to `internal`, so a code added server-side stays visible with its own message.
The stated cost: **the union is closed by convention, not by validation, so every `switch (err.code)`
downstream must carry a `default`.**

### 5.2 `problems.ts` — the shape a later phase must copy

Forty-one lines, **two exports and no more, enforced by a guard**
(`grep -cE '^export ' src/api/problems.ts  # 2`).

```ts
export function classify(error: ApiError): ProblemKind {
  if (error.code === "network_unavailable") return "control-plane-down";
  if (error.code === "backend_unavailable") return "backend-down";
  return "server-said";
}
```

`ProblemKind` is `"control-plane-down" | "backend-down" | "server-said"` (`:13-21`). The design is
deliberate and the reasoning is the part a later phase needs:

- **The prose stays at the call sites.** Three surfaces discriminate the same three cases and each
  needs its own sentence tail — *"…so the session was not created"* / *"…was not restarted"* /
  *"…could not be rebuilt"*. A shared `sentenceFor(error, verb)` would produce worse English at all
  three (`:3-8`).
- **The exhaustiveness that matters is at the call sites, not here** (`:28-36`). Each call site
  switches over `ProblemKind` with a **declared return type and no `default:`**, so under
  `strict: true` a fourth `ProblemKind` fails every one of them with TS2366. `classify` itself needs
  its catch-all and therefore proves nothing.
- Shared-shape §3A.1 clause 6 encodes this as six greps, and the anchoring is load-bearing: all four
  files *mention* `default:` in the comment explaining why they have none, so an unanchored
  `grep -c 'default:'` inverts.

**A phase-4/5/7 surface that talks to the control plane gets its error handling for free by calling
`classify` and writing its own three sentences.** It must not add a `default:` to that switch and must
not drop the return annotation.

### 5.3 A known asymmetry a later phase could trip

`client.ts:243-249`, written down rather than fixed: `send` **concatenates** `baseUrl + route`, so a
base carrying a path keeps it; `attachUrl` (`:108-118`) **resolves** against the base, so it
discards that path. A `VITE_PANACEA_APP_API` behind a path prefix works for REST and silently breaks
attach. Harmless while the base is loopback with no path. **If a later phase ever proxies the control
plane behind a prefix, both ends change together.**

---

## 6. The provenance thread — the most important finding for phases 4, 5 and 7

`CLAUDE.md` calls provenance the product spine: *"the spec written in Define is visibly referenced in
Design, carried into Develop, and attached to the release in Deploy. Same artifact, one thread, shown
throughout."*

### 6.1 The carrier exists, all the way down

I traced `contextRef` through every layer:

| Layer | Evidence |
|---|---|
| Go persistence | `control-plane/internal/registry/session.go:72-74` — *"ContextRef is the Phase 4 brief this session inherited, or ''. Phase 2 stores and returns it and never interprets it."* |
| Go filestore | `registry/filestore_test.go:147` pins `contextRef` in the persisted key set |
| Go wire — read | `api/sessions.go:65` (`json:"contextRef"`), `:96` (copied into the response) |
| Go wire — write | `api/sessions.go:152` on the create request, `:301` copied into the new session |
| Go restart | `handleSessionRestart` (`api/sessions.go:729`) mutates the **same** session record and mints only a new sandbox id, so `ContextRef` survives a restart unchanged |
| TS type | `store/types.ts:162-163` |
| TS create request | `api/client.ts:96` |
| TS resume / fork | `api/client.ts:359`, `:381` — both carry `contextRef` from the source session |
| TS request builder | `store/defaults.ts:110`, `:127` — `toCreateRequest(..., { contextRef })`, omitted when `""` |
| Test coverage | `defaults.test.ts:265-267` — *"carries contextRef only when the caller passes one"* |

**So the thread's plumbing is done and it is better than I expected.** Fork and resume both propagate
it, which is the case most likely to have been missed.

### 6.2 And nothing writes it or shows it

`grep -rn "contextRef" desktop/src` returns **eleven hits and not one is a component.** No picker
field sets it, no pane header renders it, no rail row indicates a session is descended from a brief.
Every session the cockpit can create today gets `contextRef: ""` (`test/fixtures.ts:128` is the only
place a value is even named, and it is `""`).

That is correct for phase 2 — there is no phase 4 to produce a brief — but it means:

- **Phase 4 owns the write side**: a `contextRef` on the create path, threaded from whatever the
  Discover flow produces.
- **Phases 2-consumers own the read side, and no one has built it**: contract §6.5's `SessionPane`
  has no provenance element, and the rail has none. *"Shown throughout"* is currently **zero
  surfaces**. Someone has to decide whether that is phase 4's job across all lenses or each lens's
  own job.
- **The handle is opaque by design.** `session.go:73` and `types.ts:162` both say *never
  interpreted*. So the cockpit cannot render a brief's **title** from a `contextRef` — it can only
  render the ref itself, or fetch the brief from an endpoint that does not exist yet. **Phase 4 needs
  a resolve endpoint (`GET /v1/briefs/{ref}` or equivalent) or the thread stays invisible.** That is
  a Go-contract addition, not a UI change, and no Group D task may edit a `.go` file.

---

## 7. The design token layer — `desktop/src/styles/tokens.css`

**191 lines, all OKLCH**, so light and dark are derived rather than inverted. Requires Chromium 111+
/ Safari 15.4+; the Tauri 2 webview clears both, so there is no fallback ladder (`:5-8`).

What a later phase inherits, ready to use:

- **Seven surfaces** — `--page`, `--canvas`, `--surface`, `--inset`, `--hover`, `--hover-2`, and
  `--field`, which deliberately breaks the ramp downward because a form field is a well, not a raised
  plane (`:36-40`). One warm hue, 67°, chroma rising with lightness so the ramp stays warm.
- **A four-step ink ramp** with `--ink-3` as the **floor for anything a user must read** (3.43:1 on
  `--page`) and `--ink-4` explicitly **never text** (1.92:1).
- **Accent = deep teal, interactive and current state ONLY** (`:59-68`). Six tokens. 74–90° of hue
  away from the indigo/violet tell `CLAUDE.md` bans, and 54.7° off running-green so the two never
  read as the same signal. *"If the accent starts appearing decoratively, the hierarchy is broken."*
- **Status is a separate vocabulary** (`:70-107`): `--run`, `--need`, `--fail`, `--drift` plus three
  tints. `--need` and `--drift` are **defined but unused in phase 2**, explicitly *"so Phase 3 does
  not invent a second amber and a second purple"* — a later phase gets them free.
- `--idle` and `--exited` both resolve to `--ink-3`, with a measured per-surface contrast table in
  the comment (`:90-98`) and an accepted-not-overlooked shortfall on the two hover states. The
  comment ends *"Do not 'fix' the duplication by moving one back to `--ink-4`."*
- **Radius scale tied to element size** (`:109-113`): chip 6 / control 8 / card 10 / window 14.
- **Five elevation steps** (`:115-122`), each a **1px ring plus a layered shadow** — never a flat
  border. Plus `--scrim` (`:129`) for window-class overlays.
- **Motion**: one house curve `--ease`, three durations, and `--beat: 2400ms` — the global heartbeat.
- **Type**: `--sans`, `--mono` (local fallbacks; no webfont ships, because a font that fails to load
  on a corporate laptop is a layout shift), and `--numeric: tabular-nums`.

### 7.1 The six CSS rules that bind any phase writing CSS

From `group-d-shared-shape.md` **§3B**. These are not advice — task 24 ships a **mechanical audit**
that greps for them, and in its first round it failed against what tasks 18, 21 and 23 had written.
Three Criticals that were the stylesheets' fault, not the audit's.

1. **Every stylesheet begins with `@import "../../styles/tokens.css";`** (path relative to its own
   location). The token layer itself is the only exception.
2. **No colour function outside the token layer.** No `oklch()`, `rgb()`, `hsl()` or hex in a
   component stylesheet — a colour is a `var(--token)`. If no token exists, **add one to
   `tokens.css`**, do not work around it locally. (`--scrim` exists because task 23 tried the
   workaround.)
3. **No gradient anywhere.** Task 24 greps for `gradient`. Task 21's `radial-gradient` dot-grid and
   `linear-gradient` resize affordance both failed. Express a canvas grid with `background-image`
   from a token, a border, or a repeating box-shadow — or do without.
4. **`transition: all` is forbidden**, and the transitionable set is an **exhaustive named list**:
   `opacity` at `--dur-fast` anywhere; `background-color` and `color` at `--dur-fast` on row- and
   button-sized elements only; `grid-template-columns` at `--dur` **on `.shell` alone**
   (`shell.css:16`). Anything else fails.
5. **Every live number carries `var(--numeric)`.** A digit that changes width makes a counter jitter.
6. **Shape must separate status without motion and without colour.** `prefers-reduced-motion` zeroes
   every `animation-duration` (`tokens.css:174-182`), so a status pair distinguished only by an
   animation collapses to colour alone for that user — a WCAG 1.4.1 failure for a reader who is also
   colour-blind. The remedy is a **static** differentiator inside the reduced-motion block, not a
   slower animation.

Two more, from elsewhere and just as binding on a later phase:

- **`jest-dom` is NOT installed** (§3C). No `toBeInTheDocument`, `toBeEnabled`, `toHaveValue`,
  `toBeDisabled`, `toBeEmptyDOMElement`. Use plain vitest against DOM properties. Task 22's suite
  could not compile for exactly this.
- **The heartbeat is a hook, not a pattern to reimplement.** `hooks/heartbeat.ts` exports
  `beatDelayStyle()` (a negative `animation-delay` from a shared `EPOCH`, so ten elements that
  mounted at ten moments breathe in phase) and `useHeartbeat()` (a **1 Hz tick for live numbers
  only**, one interval for the whole window, via `useSyncExternalStore`). `BEAT_MS = 2400` **must
  equal `--beat`** and `heartbeat.test.ts` reads the CSS file and pins it. The doc comment warns that
  a rail row subscribing for 40 rows is a performance defect, not a heartbeat.

---

## 8. What is NOT settled — what a later phase must not build against yet

| # | Not settled | Why it matters |
|---|---|---|
| 1 | **`App.tsx` is still the placeholder** (`App.tsx:1-11`: *"The cockpit mounts here in task 24"*). | **The cockpit has never run against a live control plane.** Everything in this survey is proven by `tsc` and vitest only. Per `CLAUDE.md`, that is not evidence it works. Task 24 mounts `AppShell`, starts the poller and loads the catalogue; task 25 is the real-window E2E. |
| 2 | **Tasks 22, 23 and 24 are unbuilt.** | 22 = session lifecycle verbs (`SessionMenu.tsx`, `sessionVerbs.ts`, edits to `ContextMenu.tsx` and the rail rows). 23 = command palette + `useKeyboardMap` (edits `AppShell.tsx`, two reserved insertion points at `:76-81` and `:100-103`). 24 = the wiring + the defect regression suite + the design-system audit. **All three touch `AppShell.tsx` or the files a later phase wants.** |
| 3 | **`selectors.ts` and `prefsSlice.ts` moved at 12:14 today.** | `selectCanvasSessionIds` and `pinSession(id, scopeIds)` both changed shape inside the last hour of the survey window. Treat the store's *pin* surface as in flight; the rest of it as settled. |
| 4 | **The `pinned` prune is an open carry.** `progress.md` records it: `pinned` is now unbounded across branches, nothing prunes pins whose branch was deleted server-side, and the cheapest correct prune lives where the poller replaces sessions — task 17's neighbourhood. Nothing is visibly wrong today because the selector filters at read time. | A later phase adding per-lens persisted state inherits the same unbounded-blob pattern if it copies `pinned`. |
| 5 | **There is no event stream.** `poller.ts:1-3`: *"There is no event stream in the Go contract — §10.5 asks for one — so Phase 2 polls, from exactly one place, with exactly one request per tick."* `POLL_INTERVAL_MS = 2_000`, paused when `document.visibilityState === "hidden"`, one in-flight guard. | A lens that needs push (a design canvas with multiplayer; a deploy pipeline's live status) **cannot get it from this substrate** and must either add §10.5 or run its own transport. Phase 5's Paperdesign already brings multiplayer, which means a second transport in the same window. |
| 6 | **The contract was amended three times today** (`ffcf5f2`, `7cd1af7`, `0b36bfe`, `be2badc`, `304b308`, `a861071`, `15fc93e` — seven `docs(contract)` commits in the window, actually). | Shared-shape §3D exists because four amendments this phase were partial. A later phase quoting the contract should check the commit date of the section it is quoting. |
| 7 | **`Effort` is missing `max`**, and the fix is a Go change ruled to task 25. | See §2. |

---

## 9. Answers to the two questions the brief asked

**What is safe to build against.**

1. `store/types.ts` — the whole domain model, including the six-member `Lens` union and
   `Session.contextRef`. Frozen behind a byte-diff gate; changing it costs a contract amendment.
2. `store/store.ts` + the three slices — flat shape, `persist` at version 1, frozen initial state.
3. `store/selectors.ts` **as a place to add to**, following §4's declare-it rule. Read
   `selectPaneSessionIds` for "what is in view".
4. `api/client.ts` + `api/index.ts`'s `api` singleton + `api/problems.ts`'s `classify`. Eighteen
   codes, three kinds, prose at the call site.
5. `styles/tokens.css` — the full token layer, including the two status colours phase 2 does not use.
6. `hooks/heartbeat.ts` — one clock, two functions, do not build a third mechanism.

**What is not.**

1. Anything that requires the app to actually run (task 24 unbuilt).
2. `AppShell.tsx` between now and task 23 — it has two reserved insertion points and an active owner.
3. `components/develop/*`, `selectors.ts`'s canvas half, `prefsSlice`'s pin half — changed within the
   hour.
4. The command palette's registry — the file does not exist.
5. Push/streaming of any kind.

---

## 10. Things I did not verify, stated plainly

- **I did not run `npm test` or `npm run typecheck`.** Another agent is editing this tree; a run
  would report its half-finished state as mine. Every "proven by tests" statement above is me reading
  a test file's name or a progress note, not me watching it pass.
- **I did not run the `types.ts` diff gate.** I read the command in `task-15.md:280` and read both
  files; I did not execute the comparison.
- **The "21 of 26 tasks" figure is not something I could confirm.** Contract §9's table names tasks
  **14–24** for Group D and the Go groups run 1–13, i.e. **24 planned tasks**. `progress.md` refers
  repeatedly to a **task 25** (the real-window end-to-end proof, carrying the `max`-effort and
  stale-sandbox-image prerequisites). I found **no definition of a task 26** in the contract or the
  plan. Tasks 22, 23 and 24 have no report files in `.superpowers/sdd/`, and task 21 completed at
  12:23 today, which is consistent with "21 done".
- **I read the Go side only where it bore on `contextRef` and restart.** I did not survey the control
  plane generally.
- **No screenshots, no rendering.** Nothing in this survey is a claim about how anything looks.
