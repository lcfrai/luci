### Task 18: Shell, rail, sub-rail and top bar

This is the widest task in Group D and the contract says so in the note under §9's task table: four surfaces and six shared primitives, and it is the only task that could reasonably be two. It is kept whole because the rail, the sub-rail and the top bar all share `Popover`, `ContextMenu` and `StatusDot`, and splitting them would produce an 18a whose only consumer is 18b. Nothing is on screen before this task and everything after it depends on it: task 20 imports `StatusDot` and `useHeartbeat` and mounts its `DevelopLens` into the slot this task reserves, task 22 imports `Popover`, `MenuItem`, `useDismissable` and `useCreateSession` and edits this task's `ContextMenu.tsx` and `rail/*Row.tsx`, and task 23 imports `useDismissable` and `useCreateSession` and edits this task's `AppShell.tsx`. Tasks 21 and 24 import nothing from `components/common/*` or `hooks/heartbeat.ts` at all — task 21 inherits the shell through task 20's `SessionPane`, and task 24 renders `<AppShell/>` — which is what the Required-by bullet below already says. If the insertion points are wrong here, tasks 20, 22 and 23 inherit the error.

**The heartbeat is one clock for the whole application and it is not a preference.** `docs/DESIGN-SYSTEM.md` names it a non-negotiable — "Every live indicator shares a phase… **never a spinner per row**" — and contract §7.1 makes `hooks/heartbeat.ts` its single owner: `beatDelayStyle()` and `useHeartbeat()`, and no third mechanism anywhere in the tree. `beatDelayStyle()` returns a **negative** `animationDelay` of `(now − EPOCH) mod BEAT_MS`, so a CSS animation that would otherwise begin its cycle at the moment its element mounted instead begins part-way through, already in phase with every dot that mounted before it. It costs one inline style and zero JavaScript per frame, because the animation itself runs on the compositor. `useHeartbeat()` is the other half and is for **live numbers only** — one `setInterval` for the whole app behind `useSyncExternalStore`, returning epoch-ms rounded to the second so two components rendering in the same tick print the same digits. A rail row must subscribe to neither: forty rows re-rendering once a second is, in the contract's own words, the second most likely performance defect in Group D.

Where the UI contract predates this task or states something loosely, these are the points it must get right. **`GET /v1/harnesses` returns `[]`, never `null`** — Group B's task 8 made the Go side allocate unconditionally and pinned it with a marshalled-bytes test, precisely because `null.map()` throws where `[].map()` returns `[]`; so `harness.models`, `harness.efforts` and `harness.permissionModes` are always present and nothing in this task writes a defensive `?? []` or describes them as nullable. **The error-code enum has sixteen members**, not fifteen: Group C's ruling 7 added `non_loopback_publish`, emitted when the daemon reports a sandbox published off loopback, and that sandbox is **running** — so the rail footer's single error line renders the server's `message` verbatim and never substitutes a phrase like "not running" for a code it does not recognise. **Session status is four values and only four** — `starting`, `running`, `idle`, `exited`; `docs/AGENTIC-UX.md` §4 rule 8 lists seven, only EXECUTING and DONE have mechanical analogues in this phase, and the other five need the agent-runtime signal this phase forbids — that is contract §0's count, not §2's comment above `SessionStatus`, which says three and which task 15 transcribes unchanged under its diff gate. `StatusDot`'s `SHAPE` map is exhaustive over those four plus the six `MachineState` values and nothing else. **A dropped WebSocket destroys nothing**: destruction is an explicit kill, the idle TTL, or the reaper collecting an orphan, so an `exited` session keeps its rail row and its transcript, which is why this task departs from §6.2 on the worktree chevron (step 11).

There is no bearer token on the desktop-to-control-plane hop and there is not meant to be one before Phase 8 (§4.5). The boundary is loopback bind plus the `Host` guard plus the `Origin` allowlist, all of it server-side in Groups B and C. Nothing in this task sends, stores, prompts for or displays a credential, and a reviewer who finds one here has found a defect.

**Reserved insertion points, per Group D shared-shape rule 1.** The files this task creates that a later task edits each carry a comment naming the consumer and saying what it will add: `AppShell.tsx` reserves the Develop slot for task 20 and the `<CommandPalette/>` mount for task 23; `menu/ContextMenu.tsx` reserves the session and worktree lifecycle verbs for task 22; and `rail/SessionRow.tsx` marks where task 22's optimistic pending affordance goes, the right-click dispatch itself already being complete in every row. Rule 3 applies to every one of them: the guard the consumer runs must be **anchored** so the reserving comment cannot satisfy it, because an unanchored `grep -c "<CanvasLayout"` counts the comment and reports a mount that does not exist. Step 19, not step 17, is where this task runs the anchored form for its own mounts and prints the greps the later tasks inherit.

**A note on the expected-output blocks in this file, read it before judging one.** `desktop/node_modules` contains no vitest at the time of writing — task 14 installs it — so **no command in this plan has been run and no reporter transcript below is transcribed from a real run**. Rather than invent one, every verification step states *what you must see*: the failing test's name, the assertion that fails, and the reason. Where reporter chrome is shown it is marked approximate. If the chrome you get differs, the chrome in this document is wrong and the named assertion is what matters; do not edit a test to match a transcript in a plan.

**Fixtures come from task 15, and this task declares none.** Group D shared-shape §3 settles what
the first authoring round left open. `desktop/src/test/fixtures.ts` is **created by task 14** with
the untyped constants (`FIXED_NOW`, `FIXED_NOW_ISO`, `IDS`) and a reserved insertion point, and
**filled by task 15** with six typed builders — `aMachine`, `aProject`, `aBranch`, `aWorktree`,
`aSession`, `aHarness` — because task 14 cannot type them (the types are task 15's) and task 17
is too late (task 16 runs first and already imports them). This task imports **five of the six**,
`aProject`, `aBranch`, `aWorktree`, `aSession` and `aHarness`; it needs no `aMachine`, because no
test here seeds a machine. It **declares no builder of its own and adds none** — a variant is an
override object passed to one of the six, never a seventh builder and never a local
`makeSessionFixture`. It therefore does **not** modify `fixtures.ts`, which is why that file is
absent from the Files list below and from step 20's `git add`. Step 1 greps for the five before
anything else is done.

**Files:**
- `desktop/src/hooks/heartbeat.ts` — `EPOCH`, `BEAT_MS`, `beatDelayStyle`, `useHeartbeat`, `__resetHeartbeat` (new)
- `desktop/src/hooks/heartbeat.test.ts` — phase alignment, one interval for many subscribers, the tokens.css pin (new)
- `desktop/src/hooks/useDismissable.ts` — the overlay stack, outside-pointerdown, Escape, focus return (new)
- `desktop/src/hooks/useCreateSession.ts` — **an addition to §9's file list, argued in step 9.** The one place a session is created from resolved defaults, shared by the split button's left half, the picker's Create, the rail's create verbs and task 23's `⌘⏎` (new)
- `desktop/src/components/common/StatusDot.tsx` — colour **plus shape**, the shared beat, the 600 ms change flash (new)
- `desktop/src/components/common/StatusDot.test.tsx` — the eight states, the shape classes, the phase, the flash (new)
- `desktop/src/components/common/Popover.tsx` — anchored positioning, viewport clamp, dismissal, focus return (new)
- `desktop/src/components/common/Toasts.tsx` — the Undo window; plain text, no HTML (new)
- `desktop/src/components/common/common.css` — `.dot-*`, `.pop-*`, `.toast-*`, the `beat` keyframe (new)
- `desktop/src/components/menu/ContextMenu.tsx` — the single right-click menu, clamped, with task 22's reserved verbs (new)
- `desktop/src/components/menu/menu.css` — `.menu-*` (new)
- `desktop/src/components/rail/rail.css` — `.rail-*` (new)
- `desktop/src/components/rail/MachineChip.tsx`, `ProjectRow.tsx`, `BranchRow.tsx`, `WorktreeRow.tsx`, `SessionRow.tsx`, `RailFooter.tsx`, `Rail.tsx` — the four-level tree, the machine chip, the footer (new)
- `desktop/src/components/rail/Rail.test.tsx` — nesting, selection, the branch dot, the chevron rule, keyboard traversal (new)
- `desktop/src/components/subrail/SubRail.tsx`, `subrail.css` — `LENSES` and the six rows, none disabled (new)
- `desktop/src/components/lens/LensPlaceholder.tsx` — the honest empty state for the five lenses Phase 2 does not build (new)
- `desktop/src/components/topbar/LayoutToggle.tsx`, `NewSessionButton.tsx`, `SessionPicker.tsx`, `TopBar.tsx`, `topbar.css` (new)
- `desktop/src/components/topbar/NewSessionButton.test.tsx` — one click creates with resolved defaults and no dialog; the caret opens the picker (new)
- `desktop/src/components/topbar/SessionPicker.test.tsx` — the descriptor-driven controls, the unavailable harness, `toCreateRequest` (new)
- `desktop/src/components/AppShell.tsx` — the three-column grid, `.grain`, `<Toasts/>`, `<ContextMenu/>`, and the slots for tasks 20 and 23 (new)
- `desktop/src/components/shell.css` — **an addition to §9's file list.** §9 gives `AppShell.tsx` no stylesheet and task 14's `app.css` is resets, scrollbars and body type only; the three-column grid and the layout change §6.1 names — the rail collapse and the sub-rail reveal, both off the one `grid-template-columns` transition — need rules and they belong with the component that owns them (new)
- `desktop/src/store/slices/uiSlice.ts` — **task 17's file.** Adds `machinesOpen`, `openMachines()`, `closeMachines()`; argued in step 11 (modify)
- `desktop/src/store/selectors.ts` — **task 17's file, per shared-shape rule 4.** Adds `selectProjectById`, `selectBranchById`, `selectWorktreeById` and `selectBranchAttentionSessionId`; argued in step 10 (modify)

**Interfaces:**
- Produces:
  - `hooks/heartbeat.beatDelayStyle(): { animationDelay: string }`, `useHeartbeat(): number`, `BEAT_MS`, `EPOCH`, `__resetHeartbeat()`
  - `hooks/useDismissable.useDismissable(open, onClose) → React.RefObject<HTMLDivElement>`, plus the module-level overlay stack: `closeTopOverlay(): boolean` and `hasOpenOverlay(): boolean`, which **task 23's `useKeyboardMap` calls instead of re-implementing §6.9's Escape ordering**
  - `hooks/useCreateSession.useCreateSession() → (worktreeId, opts?) => Promise<void>`
  - `components/common/StatusDot.StatusDot`, `components/common/Popover.Popover`, `components/common/Toasts.Toasts`
  - `components/menu/ContextMenu.ContextMenu`, and the `MenuItem` type task 22 builds its verbs against
  - `components/rail/*` — `Rail`, `MachineChip`, `ProjectRow`, `BranchRow`, `WorktreeRow`, `SessionRow`, `RailFooter`, all with contract §6.2's props
  - `components/subrail/SubRail.SubRail` and `LENSES`; `components/lens/LensPlaceholder.LensPlaceholder`
  - `components/topbar/*` — `TopBar`, `LayoutToggle`, `NewSessionButton`, `SessionPicker`, with contract §6.4's props
  - `components/AppShell.AppShell`
  - `store/selectors.selectBranchAttentionSessionId(branchId) → (s) => SessionId | null`, and `selectProjectById(id)` / `selectBranchById(id)` / `selectWorktreeById(id)`, each `→ (s) => T | undefined`
  - `store/slices/uiSlice` — `machinesOpen`, `openMachines`, `closeMachines`
- Consumes: task 14's `tokens.css`, `app.css`, `setup.ts` and `fixtures.ts`' untyped constants; task 15's `store/types.ts`, `lib/ids.ts`, `lib/time.ts` and the six typed fixture builders it adds to `fixtures.ts` (shared-shape §3.1); task 16's `api` singleton, `CockpitClient` and `ApiError`; task 17's `useCockpit`, `resetCockpit`, every selector in §3.7, `resolveDefaults` and `toCreateRequest`. Nothing from task 19 — the rail carries no terminal — and nothing from task 17's canvas surface: `CANVAS_MAX_TILES` is read by task 21, and no code, test or step here touches it.
- Required by: task 20 (`SessionPane` uses `StatusDot`; `DevelopLens` fills `AppShell`'s reserved slot), task 21 (none directly; it inherits the shell), task 22 (`ContextMenu`, `MenuItem`, `rail/*Row.tsx`, `useCreateSession`, `Toasts`' Undo window), task 23 (`AppShell` mounts the palette, `closeTopOverlay` gives it §6.9's Escape ordering, `useCreateSession` gives it `⌘⏎`, `openMachines` gives it `⌘⇧M`), task 24 (`App.tsx` renders `<AppShell/>`).
- Amends contract §3.3: adds `machinesOpen`/`openMachines`/`closeMachines` to `UiSlice`. §6.9 requires `⌘⇧M` to open the machines popover from a global key handler, and a popover held in `MachineChip`'s own `useState` is unreachable from there. Additive; changes no signature §3.3 fixes.
- Amends contract §3.7: adds **four** selectors, per shared-shape rule 4. `selectBranchAttentionSessionId` — §6.2 requires a branch row's dot to be the highest-attention status among its sessions **and** requires clicking it to navigate to the first session carrying that status; one selector returning that session's id yields both facts and returns a scalar, so it is referentially stable across a poll that changed nothing. And `selectProjectById` / `selectBranchById` / `selectWorktreeById` — §3.7 has by-id reads for a session and a harness but none for the three catalogue levels, and `ContextMenu` is handed nothing but an id and a kind; the branch and worktree forms are flatten-then-find over a `Record` of arrays, which is a derivation rule 4 keeps out of components. Argued in step 10.
- Amends contract §6.4: `SessionPicker` gains a secondary action, "Save as branch default", which writes the current draft through `setBranchDefaults`. §6.2 puts "Set branch defaults…" in the branch row's verbs and no section gives that verb a surface; adding a second button to the picker is one surface rather than two and needs no new store field. Argued in step 16.
- Departs from contract §6.2 on one point: the worktree chevron is drawn when `sessionCount > 0` **or** the store already holds session ids for that worktree. Argued in step 11.
- Departs from contract §6.4 on one signature: `SessionPicker` returns `JSX.Element | null`, where §6.4's declaration block fixes `JSX.Element`. It renders nothing when `open` is false, and returning an empty fragment instead would leave it mounted — running its effects and holding its draft — which is the state the `useEffect` at its top exists to clear. `ContextMenu` (`target === null`) and `Popover` (`anchor === null`) narrow the same way for the same reason; §6.7's table gives those two their props but no return type, so only the picker's is a departure from something the contract wrote down.
- Departs from contract §7.1 on the export surface, not on the mechanism: §7.1's declaration block says "Two exports, one file, one owner" and writes `EPOCH` with no `export`. This module also exports `EPOCH`, `BEAT_MS` and `__resetHeartbeat`, because the phase test measures a delay from the epoch, the CSS pin compares `BEAT_MS` against `--beat` in task 14's stylesheet, and vitest does not reset module state (§1.6). `beatDelayStyle()` and `useHeartbeat()` remain the only two the application calls, and the test seam is named `__`-prefixed so it reads as one.

- [ ] **Step 1: Preflight — confirm the substrate, and do not patch another task's file to get past it**

Tasks 14 to 17 own every file this task builds on. If one of these checks fails, **stop and report it to the orchestrator**. Silently adding the missing symbol locally is how Group C produced nine Criticals: two authors wrote the same thing differently and both suites were green.

```bash
cd desktop
ls src/store/types.ts src/store/store.ts src/store/selectors.ts src/store/defaults.ts
ls src/styles/tokens.css src/test/setup.ts src/test/fixtures.ts src/api/index.ts
grep -cE '^export function resolveDefaults' src/store/defaults.ts
grep -cE '^export function toCreateRequest' src/store/defaults.ts
grep -cE '^export const useCockpit' src/store/store.ts
grep -cE '^export function resetCockpit' src/store/store.ts
grep -cE '^[[:space:]]*--beat:' src/styles/tokens.css
grep -cE '^export function a(Project|Branch|Worktree|Session|Harness)\(' src/test/fixtures.ts
```

**What you must see:** every `ls` prints its path with no `No such file`; the four `export` greps each print `1`; the `--beat:` grep prints `1`; the fixture grep prints `5`.

Every one of those is anchored at the first character of the declaration, per shared-shape §3A, and the `--beat` line is why the rule exists: the unanchored `grep -c "beat" src/styles/tokens.css` prints **2**, not 1, because task 14 writes `--beat: 2400ms;` under a comment that begins "One global heartbeat". A guard whose number was reasoned about rather than measured fails on correct code and sends you hunting a defect that is not there. A `0` on that line means the token layer is absent and `beatDelayStyle` has no animation to align.

`5` on the last line is the one to read carefully. It is `aProject`, `aBranch`, `aWorktree`, `aSession` and `aHarness` — the five of shared-shape §3.2's six that this task imports. Anything less means **task 15 has not filled the builders task 14 reserved**. Stop and report it. Do not write a local `aSession` here and do not fall back to a `makeSessionFixture` spelling: that exact omission produced six of the last round's sixteen Criticals, when seven authors wrote one name, three wrote another, and the file meant to hold either shipped with neither.

One fact worth confirming rather than assuming, because later steps depend on it:

```bash
grep -n "harnesses" src/store/slices/dataSlice.ts | head -5
grep -rnE '(models|efforts|permissionModes)[[:space:]]*\?\?[[:space:]]*\[\]' src/api/client.ts src/store/
```

**What you must see:** `harnesses` is a `Harness[]` on the data slice, and the second grep prints **nothing**. `GET /v1/harnesses` returns `[]` and never `null` — Group B task 8 pinned it — so a defensive `?? []` **over those three fields** is dead code that hides a real contract change if the server ever regresses.

The grep names the three fields on purpose, and a bare `grep -rn "?? \[\]" src/store/` is **not** this check: it fails against correct code, which is the defect shared-shape §3A was added to stop. Task 16's client writes `filter?.status ?? []` when it builds a query string, and task 17's `dataSlice.upsertSession` writes `s.sessionIdsByWorktree[session.worktreeId] ?? []` for a worktree whose session list has not been fetched yet. Both are `Record`/optional-argument lookups on the client's own index, not a defence against a nullable server array, and deleting either would make `upsertSession` throw on the first session in a new worktree. Step 10 draws the same line about this task's own selector.

- [ ] **Step 2: Write the failing test for the one clock**

`hooks/heartbeat.ts` is the one clock the whole shell shares and the file task 20 imports directly, so its behaviour is pinned before it exists. Each property below is a separate defect if it is wrong. **Phase alignment:** two elements mounting 900 ms apart must receive delays that differ by 900 ms, not two delays of zero — that is the whole point of the negative-delay trick and a `beatDelayStyle` returning `"0ms"` would pass a naive "returns a string" test while shipping the exact failure `docs/DESIGN-SYSTEM.md` bans. **One interval:** ten subscribers cost one timer, not ten. **The CSS pin:** `BEAT_MS` in TypeScript and `--beat` in `tokens.css` are the same duration written twice, and the day they diverge every dot is permanently out of phase with its own animation — so the test reads task 14's stylesheet off disk and compares.

Create `desktop/src/hooks/heartbeat.test.ts`:

**Read the stylesheet through Vite's `?raw` loader, not `node:fs`.** This step originally wrote `readFileSync` plus `fileURLToPath(new URL(..., import.meta.url))`, and **that version does not compile in this package** — corrected 2026-09-23 after task 18's implementer hit it and task 18's reviewer verified every premise. Two measured reasons, neither a preference: `@types/node` is **not** a dependency here (`vite.config.ts:3-4` carries a `@ts-expect-error` for `node:process` for precisely that reason), so the `node:fs` import fails typecheck with **TS2591**; and `import.meta.url` under vitest's transform is **not** a `file:` URL, so `fileURLToPath` throws `The URL must be of scheme file`. Task 14's `src/styles/tokens.test.ts:2` already reads the same file the same way, and `vitest.config.ts:26` opts `?raw` past the css-disable plugin for exactly this. Same subject, same assertion, no new dependency.

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import tokensCss from "../styles/tokens.css?raw";
import { BEAT_MS, EPOCH, beatDelayStyle, useHeartbeat, __resetHeartbeat } from "./heartbeat";

describe("beatDelayStyle", () => {
  it("returns a negative delay so a late mount is already in phase", () => {
    const spy = vi.spyOn(performance, "now").mockReturnValue(EPOCH + 900);
    expect(beatDelayStyle()).toEqual({ animationDelay: "-900ms" });
    spy.mockRestore();
  });

  it("wraps at one beat, so the delay is never longer than the cycle", () => {
    const spy = vi.spyOn(performance, "now").mockReturnValue(EPOCH + BEAT_MS * 3 + 120);
    expect(beatDelayStyle()).toEqual({ animationDelay: "-120ms" });
    spy.mockRestore();
  });

  it("agrees with the --beat token in tokens.css", () => {
    const match = /--beat:\s*(\d+)ms/.exec(tokensCss);
    expect(match).not.toBeNull();
    expect(Number(match![1])).toBe(BEAT_MS);
  });
});

describe("useHeartbeat", () => {
  // ORDER MATTERS, and getting it wrong fails by days rather than by milliseconds.
  // __resetHeartbeat() seeds the module's cached tick from Date.now(), so the fake clock has
  // to already BE at the fixed instant when it runs. Call setSystemTime inside the test
  // instead and the cached tick is a real-clock value while the interval fires on the fake
  // one, so `result.current - first` is the gap between 2026-09-18 and today, not 1000.
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-18T00:00:00.400Z"));
    __resetHeartbeat();
  });
  afterEach(() => {
    __resetHeartbeat();
    vi.useRealTimers();
  });

  it("ticks once a second and reports whole seconds", () => {
    const { result } = renderHook(() => useHeartbeat());
    const first = result.current;
    expect(first % 1000).toBe(0);
    act(() => void vi.advanceTimersByTime(1000));
    expect(result.current - first).toBe(1000);
  });

  it("shares one interval across many subscribers", () => {
    const setInterval = vi.spyOn(globalThis, "setInterval");
    const a = renderHook(() => useHeartbeat());
    const b = renderHook(() => useHeartbeat());
    const c = renderHook(() => useHeartbeat());
    expect(setInterval).toHaveBeenCalledTimes(1);
    expect(a.result.current).toBe(b.result.current);
    expect(b.result.current).toBe(c.result.current);
    setInterval.mockRestore();
  });

  it("stops the interval when the last subscriber unmounts", () => {
    const clearInterval = vi.spyOn(globalThis, "clearInterval");
    const a = renderHook(() => useHeartbeat());
    const b = renderHook(() => useHeartbeat());
    a.unmount();
    expect(clearInterval).not.toHaveBeenCalled();
    b.unmount();
    expect(clearInterval).toHaveBeenCalledTimes(1);
    clearInterval.mockRestore();
  });
});
```

Run it:

```bash
cd desktop && npm test -- src/hooks/heartbeat.test.ts
```

**What you must see:** the run fails at **import time**, naming `src/hooks/heartbeat.test.ts` and the unresolved import `./heartbeat`, with a message of the form `Failed to load url ./heartbeat … Does the file exist?`, and **zero tests collected**. Reporter chrome around that line is approximate here. What is not negotiable is that the failure is the missing module rather than a green suite.

- [ ] **Step 3: Implement the one clock**

One file, one owner, one clock — contract §7.1. `beatDelayStyle()` and `useHeartbeat()` are the two the application calls; `EPOCH`, `BEAT_MS` and `__resetHeartbeat` are exported for the test, which is the §7.1 departure declared above. The interval is lazy: it is created by the first subscriber and cleared by the last, so a window with no live number on screen runs no timer at all. `useSyncExternalStore` needs a **cached** snapshot, so the tick value is held in a module variable and only reassigned when it actually changes; returning `Date.now()` from `getSnapshot` would make React re-render on every store read and, under `useSyncExternalStore`'s development-mode double-invocation check, throw `The result of getSnapshot should be cached`.

Create `desktop/src/hooks/heartbeat.ts`:

```ts
// The ONE clock. Contract §7.1; docs/DESIGN-SYSTEM.md: "never a spinner per row".
//
// Two functions and no third mechanism. beatDelayStyle() aligns a CSS animation's PHASE to a
// shared epoch so ten dots that mounted at ten different moments breathe together.
// useHeartbeat() is a 1 Hz tick for LIVE NUMBERS ONLY — elapsed, waiting — behind one interval
// for the whole window. EPOCH, BEAT_MS and the __resetHeartbeat seam are exported for the test.
import { useSyncExternalStore } from "react";

/** The shared epoch. Module load time, once per window. */
export const EPOCH = performance.now();

/** Must equal --beat in src/styles/tokens.css. heartbeat.test.ts reads the file and pins it. */
export const BEAT_MS = 2400;

/** Returns the style that aligns a CSS animation's PHASE to the shared epoch.
 *
 *  WHY a negative animation-delay rather than a JS-driven opacity: a CSS animation runs on the
 *  compositor and costs nothing per frame, but it starts its cycle when the ELEMENT mounts — so
 *  ten rows appearing at ten moments breathe out of phase, which is exactly the failure
 *  DESIGN-SYSTEM names. A negative delay of (now - EPOCH) mod BEAT_MS starts the animation
 *  already part-way through its cycle, so every element, whenever it mounted, sits at the same
 *  point of the same breath. One line, zero JS per frame, perfect phase. */
export function beatDelayStyle(): { animationDelay: string } {
  const offset = Math.round(performance.now() - EPOCH) % BEAT_MS;
  return { animationDelay: `-${offset}ms` };
}

let tick = Math.floor(Date.now() / 1000) * 1000;
let timer: ReturnType<typeof setInterval> | null = null;
const listeners = new Set<() => void>();

function start(): void {
  if (timer !== null) return;
  timer = setInterval(() => {
    const next = Math.floor(Date.now() / 1000) * 1000;
    if (next === tick) return;
    tick = next;
    for (const listener of listeners) listener();
  }, 1000);
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  start();
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/** getSnapshot must be referentially stable across calls with no tick between them, or React
 *  throws "The result of getSnapshot should be cached". Hence the module variable. */
function getSnapshot(): number {
  return tick;
}

/** A 1 Hz tick for LIVE NUMBERS ONLY. Returns epoch-ms rounded down to the second, so two
 *  components rendering in the same tick print the same number. A component that needs neither
 *  this nor beatDelayStyle subscribes to neither: this hook re-renders its caller once a second
 *  and a rail row doing that for 40 rows is a performance defect, not a heartbeat. */
export function useHeartbeat(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** Test seam. Clears the interval and every listener so module state does not leak between
 *  files — vitest's restoreMocks/clearMocks do not reset module state (§1.6). */
export function __resetHeartbeat(): void {
  if (timer !== null) clearInterval(timer);
  timer = null;
  listeners.clear();
  tick = Math.floor(Date.now() / 1000) * 1000;
}
```

Run it:

```bash
cd desktop && npm test -- src/hooks/heartbeat.test.ts
npm run typecheck
```

**What you must see:** six passing tests in one file — three under `beatDelayStyle`, three under `useHeartbeat` — and `npm run typecheck` exiting 0 after printing **npm's own banner and nothing else**:

```
> desktop@0.1.0 typecheck
> tsc --noEmit

```

`tsc --noEmit` is silent on success, so anything after that banner is a diagnostic, in the form `src/hooks/heartbeat.ts(NN,NN): error TS####: …` — the coordinates are whatever your file has, the code and the message are what to read. Every later step in this file that says "a clean `typecheck`" means exactly this: the banner, then nothing.

If `agrees with the --beat token in tokens.css` fails, **change neither number blindly** — read `tokens.css` and decide which is right, then fix the wrong one. The token is the designed value; `BEAT_MS` is its mirror.

- [ ] **Step 4: Write the failing test for `StatusDot`**

`StatusDot` is the most-reused component in the cockpit — the machine chip, every branch row, every session row and task 20's pane header all render one — and it is the only component that may call `beatDelayStyle()`. Contract §7.5 fixes its vocabulary: status is **colour plus shape**, because colour alone fails for colourblind users, and the four session statuses map to a breathing filled circle, a hollow ring, a still filled circle and a small square. It must also accept all six `MachineState` values, because §6.2 gives the machine chip a dot and §2's `MachineState` has six members even though Phase 2's control plane emits two of them.

Create `desktop/src/components/common/StatusDot.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { StatusDot } from "./StatusDot";

function dot(): HTMLElement {
  return screen.getByRole("img");
}

describe("StatusDot", () => {
  it("carries shape as well as colour, for every status the server can send", () => {
    const cases: Array<[string, string]> = [
      ["starting", "dot-ring"],
      ["running", "dot-disc"],
      ["idle", "dot-disc"],
      ["exited", "dot-square"],
      ["provisioning", "dot-ring"],
      ["sleeping", "dot-ring"],
      ["stopped", "dot-square"],
      ["error", "dot-square"],
    ];
    for (const [status, shape] of cases) {
      const { unmount } = render(
        <StatusDot status={status as "running"} live={false} label={status} />,
      );
      expect(dot().className).toContain(shape);
      expect(dot().className).toContain(`dot-tone-${status}`);
      unmount();
    }
  });

  it("names the state for a screen reader and for a hover", () => {
    render(<StatusDot status="running" live label="running · claude" />);
    expect(dot().getAttribute("aria-label")).toBe("running · claude");
    expect(dot().getAttribute("title")).toBe("running · claude");
  });

  it("joins the shared beat only when live, and never spins per row", () => {
    const { unmount } = render(<StatusDot status="running" live label="running" />);
    expect(dot().className).toContain("dot-live");
    expect(dot().style.animationDelay).toMatch(/^-\d+ms$/);
    unmount();
    render(<StatusDot status="idle" live={false} label="idle" />);
    expect(dot().className).not.toContain("dot-live");
    expect(dot().style.animationDelay).toBe("");
  });

  it("flashes for 600 ms when the status changes, then stops", () => {
    vi.useFakeTimers();
    const { rerender } = render(<StatusDot status="starting" live={false} label="starting" />);
    expect(dot().className).not.toContain("dot-flash");
    rerender(<StatusDot status="running" live label="running" />);
    expect(dot().className).toContain("dot-flash");
    act(() => void vi.advanceTimersByTime(600));
    expect(dot().className).not.toContain("dot-flash");
    vi.useRealTimers();
  });
});
```

Run it:

```bash
cd desktop && npm test -- src/components/common/StatusDot.test.tsx
```

**What you must see:** the run fails at import time on the unresolved `./StatusDot`, zero tests collected. Same shape as step 2's failure.

One detail that will bite if it is missed: the flash test asserts **no** flash on first render. A component that sets `dot-flash` in a `useEffect` with `[status]` flashes on mount, and forty rail rows all flashing the moment the app opens is a light show, not a signal. The previous status is therefore held in a ref that is initialised to the first status, and the flash fires only on a genuine transition.

- [ ] **Step 5: Implement `StatusDot` and the shared `common.css`**

Two TypeScript facts about this tree that every remaining step depends on, established here once. **`JSX.Element` is not a global.** `desktop/package.json` pins `@types/react@^19.1.8`, and React 19's types removed the global `JSX` namespace; contract §6 writes `JSX.Element` in every signature and that spelling no longer compiles. Every component file in this task therefore begins `import type { JSX } from "react";` and keeps the contract's return type verbatim. **`tsconfig.json` sets `"lib": ["ES2020", "DOM"]` with no `DOM.Iterable`**, so a `NodeList` cannot be spread or `for…of`-ed; step 13's `Rail` wraps its `querySelectorAll` in `Array.from` for exactly this reason. Both facts were read off the committed config, not assumed.

Create `desktop/src/components/common/common.css`. It carries the three primitives' rules and nothing else; the `beat` keyframe here is the one contract §7.1 specifies.

```css
@import "../../styles/tokens.css";

/* Shared primitives: the status dot, the popover, the toast stack.
   Owns .dot-*, .pop-* and .toast-*. No other file writes these prefixes.
   The @import above is the first line of EVERY stylesheet in this task, and of every
   stylesheet in tasks 20 to 23: contract §5.5 requires it, Group D shared-shape §3B rule 1
   binds it, and task 24's designsystem.test.ts strips comments and asserts that the first
   line left is exactly that import. A component stylesheet that opens with a selector fails
   the audit, and the audit is the phase's only mechanical defence against a tree of
   hard-coded colour. */

@keyframes beat {
  0%,
  100% {
    opacity: 0.45;
  }
  50% {
    opacity: 1;
  }
}

/* The status-change halo, drawn by .dot-flash::after. One outward pulse that fades to
   nothing: transform and opacity only, so it stays on the compositor (§7.5). It ends at
   opacity 0 and the rule below holds it there with animation-fill-mode, because React strips
   the class a frame later and a snap back to full opacity would read as a second flash. */
@keyframes flash {
  from {
    opacity: 0.9;
    transform: scale(1);
  }
  to {
    opacity: 0;
    transform: scale(2.2);
  }
}

.dot {
  /* position: relative is what the ::after halo is measured against. Without it the halo is
     laid out from the nearest positioned ancestor — a rail row, or the viewport. */
  position: relative;
  display: inline-block;
  flex: none;
  inline-size: 8px;
  block-size: 8px;
  background: currentColor;
  color: var(--ink-3);
}

/* Shape, not just colour — docs/DESIGN-SYSTEM.md, contract §7.5. */
.dot-disc {
  border-radius: 50%;
}
.dot-ring {
  border-radius: 50%;
  background: none;
  box-shadow: inset 0 0 0 1.5px currentColor;
}
.dot-square {
  inline-size: 6px;
  block-size: 6px;
  border-radius: 1px;
}

.dot-tone-starting,
.dot-tone-provisioning {
  color: var(--accent);
}
.dot-tone-running {
  color: var(--run);
}
.dot-tone-idle,
.dot-tone-sleeping {
  color: var(--idle);
}
.dot-tone-exited,
.dot-tone-stopped {
  color: var(--exited);
}
.dot-tone-error {
  color: var(--fail);
}

/* opacity only, so it stays on the compositor. The PHASE comes from the inline
   animation-delay beatDelayStyle() returns; the duration comes from the token. */
.dot-live {
  animation: beat var(--beat) var(--ease) infinite;
}
/* THE FLASH IS ON A PSEUDO-ELEMENT, AND THAT IS NOT DECORATION. A second `.dot-flash` rule
   setting `animation` on the dot itself would beat `.dot-live` on source order and stop the
   shared breath for 600 ms — and, worse, it would never be seen at all: StatusDot puts
   beatDelayStyle()'s inline `animation-delay` on the same element, that value is NEGATIVE and
   usually larger than 600 ms, and an inline style outranks any stylesheet, so the one-shot
   would be over before it began. animation-delay is not an inherited property and an inline
   style does not reach ::after, so the halo starts at 0 while the dot underneath keeps
   breathing, in phase, throughout. */
.dot-flash::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  box-shadow: inset 0 0 0 1.5px currentColor;
  pointer-events: none;
  animation: flash 600ms var(--ease) 1;
  /* A longhand AFTER the shorthand, which resets fill-mode to none. */
  animation-fill-mode: forwards;
}

.pop {
  position: fixed;
  z-index: 60;
  min-inline-size: 180px;
  padding: 6px;
  background: var(--surface);
  border-radius: var(--r-card);
  box-shadow: var(--e-overlay);
  color: var(--ink);
  font: inherit;
}

.toast-stack {
  position: fixed;
  inset-block-end: 16px;
  inset-inline-end: 16px;
  z-index: 70;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: var(--surface);
  border-radius: var(--r-ctl);
  box-shadow: var(--e-raised);
  color: var(--ink);
  pointer-events: auto;
}
.toast-undo {
  padding: 2px 8px;
  background: none;
  border: 0;
  border-radius: var(--r-chip);
  box-shadow: var(--e-hair);
  color: var(--accent-hi);
  cursor: pointer;
  font: inherit;
  transition: opacity var(--dur-fast) var(--ease);
}
.toast-undo:hover {
  opacity: 0.8;
}
```

Create `desktop/src/components/common/StatusDot.tsx`:

```tsx
// The one status dot. Contract §6.7 and §7.5: colour PLUS shape, and the shared heartbeat
// phase when live. This is the ONLY component that may call beatDelayStyle().
import { useEffect, useRef, useState } from "react";
import type { JSX } from "react";
import { beatDelayStyle } from "../../hooks/heartbeat";
import type { MachineState, SessionStatus } from "../../store/types";
import "./common.css";

/** Four session statuses and six machine states. `running` and `idle` are in both, so the
 *  union has eight members. docs/AGENTIC-UX.md §4 rule 8 lists seven session statuses; only
 *  EXECUTING and DONE have mechanical analogues in this phase, and the OTHER FIVE need the
 *  agent-runtime signal Phase 2 forbids, so they are not in SessionStatus and must not be
 *  rendered here. Contract §0 is the count that is right; §2's comment above SessionStatus
 *  says three, and types.ts keeps it because task 15 diffs that file against the contract
 *  byte for byte. */
export type DotStatus = SessionStatus | MachineState;

const SHAPE: Record<DotStatus, "dot-ring" | "dot-disc" | "dot-square"> = {
  starting: "dot-ring",
  provisioning: "dot-ring",
  sleeping: "dot-ring",
  running: "dot-disc",
  idle: "dot-disc",
  exited: "dot-square",
  stopped: "dot-square",
  error: "dot-square",
};

const FLASH_MS = 600;

export function StatusDot(props: {
  status: DotStatus;
  live: boolean;
  label: string;
}): JSX.Element {
  const { status, live, label } = props;
  // Initialised to the FIRST status, so a fresh mount never flashes. Forty rail rows all
  // flashing when the window opens is a light show, not a signal.
  const previous = useRef<DotStatus>(status);
  const [flashing, setFlashing] = useState(false);

  useEffect(() => {
    if (previous.current === status) return undefined;
    previous.current = status;
    setFlashing(true);
    const timer = setTimeout(() => setFlashing(false), FLASH_MS);
    return () => clearTimeout(timer);
  }, [status]);

  const className = ["dot", SHAPE[status], `dot-tone-${status}`]
    .concat(live ? ["dot-live"] : [])
    .concat(flashing ? ["dot-flash"] : [])
    .join(" ");

  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={className}
      style={live ? beatDelayStyle() : undefined}
    />
  );
}
```

Run it:

```bash
cd desktop && npm test -- src/components/common/StatusDot.test.tsx
npm run typecheck
```

**What you must see:** four passing tests, and a clean `typecheck` — npm's banner, then nothing. If `flashes for 600 ms` fails on the *first* assertion — `dot-flash` present before any change — the ref was initialised to something other than the first status, or the effect has no equality guard.

Two things the test cannot check and review must. `.dot-live` animates **`opacity` only**: the moment someone adds `background` or `box-shadow` to the `beat` keyframe it leaves the compositor, and forty rows repainting at 2.4 s intervals is the cost. And **the flash must stay on `::after`**. It is tempting to collapse `.dot-flash::after` into a plain `.dot-flash { animation: flash 600ms … }`, and that is the one change that silently deletes the feature: the rule would win the cascade over `.dot-live`, stopping the shared beat for 600 ms, and StatusDot's inline `animation-delay` — negative, and usually larger than 600 ms — would apply to the one-shot as well, so the flash would already be finished at the moment it started and nothing would ever appear on screen. The suite cannot catch it: jsdom computes no animations, so `flashes for 600 ms when the status changes` asserts the class and not the pixels. Both keyframes animate `opacity` and `transform` only, which is what §7.5 allows.

- [ ] **Step 6: The overlay stack and `Popover`**

Contract §6.7 gives `Popover` "outside-click, Escape, and focus return", while §6.8 says "no component adds a `window` keydown listener" and §6.9 gives Escape to `useKeyboardMap`. Those read as a contradiction and they are not, once you notice the ordering rule §6.9 states: "Close the topmost overlay; if none, **do nothing**." Topmost is a *stack* property, and a stack cannot live in one popover's closure.

**The decision, where the contract is silent on the mechanism:** the stack is a module-level singleton in `hooks/useDismissable.ts` — the same shape as task 19's `attachRegistry`, and not a component — and its `keydown` listener is attached **only while at least one overlay is open** and removed when the last one closes. So when no overlay is open there is literally no listener, Escape reaches xterm, and the agent CLI interrupts itself, which is §6.9's whole point. The listener is capture-phase so an open overlay wins over a focused terminal. `hasOpenOverlay()` and `closeTopOverlay()` let task 23's `useKeyboardMap` delegate rather than re-derive; **task 23 must call them and must not add its own Escape branch for overlays.**

Create `desktop/src/hooks/useDismissable.ts`:

```ts
// Outside-click, Escape and focus return for popovers, menus and the palette. Contract §6.7.
//
// The stack is module-level on purpose. §6.9 says Escape closes the TOPMOST overlay and does
// nothing when there is none; "topmost" is a property of all open overlays at once, which no
// single component can know. Listeners are attached only while the stack is non-empty, so with
// no overlay open Escape is not intercepted at all and reaches the terminal (§6.9).
import { useLayoutEffect, useRef } from "react";
import type { RefObject } from "react";

interface Entry {
  close: () => void;
}

const stack: Entry[] = [];

/** True while any overlay registered through useDismissable is open. Task 23's keyboard map
 *  reads this instead of tracking overlay state itself. */
export function hasOpenOverlay(): boolean {
  return stack.length > 0;
}

/** Closes the topmost overlay and returns true, or returns false when there is none. This is
 *  the ONLY implementation of §6.9's Escape ordering. */
export function closeTopOverlay(): boolean {
  const top = stack[stack.length - 1];
  if (top === undefined) return false;
  top.close();
  return true;
}

/** Test seam: module state survives vitest's restoreMocks/clearMocks (§1.6). */
export function __resetOverlays(): void {
  stack.length = 0;
}

/** Attach the returned ref to the overlay's outermost element. While `open`, a pointerdown
 *  outside it or Escape closes it, and on close DOM focus returns to whatever had it before —
 *  without which ⌘K, Escape, keep-typing is impossible (§6.8).
 *
 *  ONE CONTRACT FOR CALLERS, and it is two attributes, not one: the control that opens an
 *  overlay must carry `data-overlay-trigger`, must own the toggle itself, and must report that
 *  overlay's open state in `aria-expanded`. This listener is capture-phase on `document`, so it
 *  runs BEFORE the trigger's own click; without the attributes it closes the overlay and the
 *  click that follows re-opens it, giving a button that can never close what it opened.
 *  MachineChip's chip and the split button's caret are the two in this task; task 22's ⋯ button
 *  and task 23's palette launcher need both attributes too. */
export function useDismissable<T extends HTMLElement>(
  open: boolean,
  onClose: () => void,
): RefObject<T | null> {
  const ref = useRef<T | null>(null);
  const onCloseRef = useRef(onClose);

  // After every render, before paint: keeps the latest callback without making it a dependency
  // of the effect below, which would push and pop the stack on every parent re-render.
  useLayoutEffect(() => {
    onCloseRef.current = onClose;
  });

  useLayoutEffect(() => {
    if (!open) return undefined;
    const restoreTo = document.activeElement as HTMLElement | null;
    const entry: Entry = { close: () => onCloseRef.current() };
    stack.push(entry);

    const isTop = (): boolean => stack[stack.length - 1] === entry;

    const onPointerDown = (event: PointerEvent): void => {
      if (!isTop()) return;
      const target = event.target;
      const el = ref.current;
      if (el !== null && target instanceof Node && el.contains(target)) return;
      // The control that opened THIS overlay is not "outside" it — but another overlay's
      // trigger is, and matching every [data-overlay-trigger] document-wide would treat them
      // as the same thing: with the machines popover open, a pointerdown on the split button's
      // caret would be exempted, the popover would stay open behind the picker with no
      // outside-click left to dismiss it, and task 23's chords — gated on hasOpenOverlay() —
      // would go quiet. So the exemption is scoped by aria-expanded, which the trigger already
      // carries for the screen reader: only the trigger of an OPEN overlay reports "true".
      const trigger =
        target instanceof Element ? target.closest("[data-overlay-trigger]") : null;
      if (trigger !== null && trigger.getAttribute("aria-expanded") === "true") return;
      onCloseRef.current();
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key !== "Escape" || !isTop()) return;
      event.preventDefault();
      event.stopPropagation();
      onCloseRef.current();
    };

    document.addEventListener("pointerdown", onPointerDown, true);
    document.addEventListener("keydown", onKeyDown, true);

    return () => {
      document.removeEventListener("pointerdown", onPointerDown, true);
      document.removeEventListener("keydown", onKeyDown, true);
      const index = stack.indexOf(entry);
      if (index >= 0) stack.splice(index, 1);
      if (restoreTo !== null && document.contains(restoreTo)) restoreTo.focus();
    };
  }, [open]);

  return ref;
}
```

**Why the trigger exemption is scoped rather than document-wide, since the unscoped form is the obvious one to write.** `target.closest("[data-overlay-trigger]") !== null` reads as "the click landed on a trigger, so leave it to the trigger", and it is wrong the moment a second trigger exists: with the machines popover open, a pointerdown on the split button's caret matches, the handler returns, the popover is never dismissed, and it sits behind the session picker with no outside-click affordance left — and because task 23 gates its chords on `hasOpenOverlay()`, a stuck overlay silently disables them. `aria-expanded` is the discriminator that is already there for the screen reader and already true only for an overlay that is open. One case remains ambiguous and is worth naming rather than hiding: if two overlays are open at once, both triggers report `true`, so a pointerdown on the lower one's trigger does not dismiss the upper one — it closes its own overlay instead. That is reachable only by opening a second overlay from the keyboard while the first is up, it leaves no overlay stuck, and closing it properly would need a trigger-to-overlay link the contract's `useDismissable(open, onClose)` signature does not carry.

Create `desktop/src/components/common/Popover.tsx`:

```tsx
// Positioning, outside-click, Escape and focus return. Contract §6.7. Used by the session
// picker, the machine chip and (task 22) the session menu.
import { useLayoutEffect, useState } from "react";
import type { JSX, ReactNode } from "react";
import { useDismissable } from "../../hooks/useDismissable";
import "./common.css";

const GUTTER = 8;

export function Popover(props: {
  anchor: DOMRect | null;
  onClose: () => void;
  children: ReactNode;
}): JSX.Element | null {
  const { anchor, onClose, children } = props;
  const ref = useDismissable<HTMLDivElement>(anchor !== null, onClose);
  const [placement, setPlacement] = useState<{ top: number; left: number } | null>(null);

  // Measure after mount and clamp into the viewport, so a popover anchored near the right edge
  // opens leftwards instead of off screen. jsdom reports every rect as zero, which is fine:
  // the clamp is a no-op there and the tests assert content, not geometry.
  useLayoutEffect(() => {
    const el = ref.current;
    if (anchor === null || el === null) {
      setPlacement(null);
      return;
    }
    const self = el.getBoundingClientRect();
    const left = Math.max(
      GUTTER,
      Math.min(anchor.left, window.innerWidth - self.width - GUTTER),
    );
    const below = anchor.bottom + 6;
    const top =
      below + self.height + GUTTER > window.innerHeight && anchor.top - self.height - 6 > GUTTER
        ? anchor.top - self.height - 6
        : below;
    setPlacement({ top, left });
    el.focus();
  }, [anchor, ref]);

  if (anchor === null) return null;

  return (
    <div
      ref={ref}
      role="dialog"
      tabIndex={-1}
      className="pop"
      style={{ top: placement?.top ?? anchor.bottom + 6, left: placement?.left ?? anchor.left }}
    >
      {children}
    </div>
  );
}
```

`anchor === null` is the closed state, which is why `Popover` has no separate `open` prop: contract §6.7 gives it exactly three props and a second source of truth for "is it open" is how a popover ends up rendered at `0,0` behind the rail.

- [ ] **Step 7: `Toasts` — the Undo window**

`docs/DESIGN-SYSTEM.md` requires an Undo window rather than a confirm dialog for agent actions, and contract §2 fixes the shape: plain text, an optional `undo`, a `ttl` of 6000 ms with an undo and 3200 ms without. Each toast owns its own timer, in its own child component, because one timer over an array restarts whenever any toast is added and a toast that will not go away is worse than one that goes too soon. `Toast.message` is **plain text** — the prototype's `innerHTML` toasts are not carried forward and `dangerouslySetInnerHTML` must not appear in this file.

Create `desktop/src/components/common/Toasts.tsx`:

```tsx
// The optimistic-mutation Undo window. Contract §6.7. Plain text only.
import { useEffect } from "react";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import type { Toast } from "../../store/types";
import "./common.css";

function ToastRow(props: { toast: Toast; onDismiss: (id: string) => void }): JSX.Element {
  const { toast, onDismiss } = props;

  useEffect(() => {
    const remaining = Math.max(0, toast.createdAt + toast.ttl - Date.now());
    const timer = setTimeout(() => onDismiss(toast.id), remaining);
    return () => clearTimeout(timer);
  }, [toast.id, toast.createdAt, toast.ttl, onDismiss]);

  return (
    <div className="toast" role="status">
      <span className="toast-text">{toast.message}</span>
      {toast.undo === undefined ? null : (
        <button
          type="button"
          className="toast-undo"
          onClick={() => {
            toast.undo?.();
            onDismiss(toast.id);
          }}
        >
          Undo
        </button>
      )}
    </div>
  );
}

export function Toasts(): JSX.Element {
  const toasts = useCockpit(useShallow((s) => s.toasts));
  const dismissToast = useCockpit((s) => s.dismissToast);

  return (
    <div className="toast-stack" aria-live="polite">
      {toasts.map((toast) => (
        <ToastRow key={toast.id} toast={toast} onDismiss={dismissToast} />
      ))}
    </div>
  );
}
```

The `remaining` computation rather than a flat `toast.ttl` matters for one real case: a toast pushed while the window was hidden, or re-rendered after a store rehydrate, must expire on its original schedule rather than restarting. `aria-live="polite"` and not `assertive`, because an Undo window is an offer, not an alarm.

- [ ] **Step 8: The single `ContextMenu`, with task 22's insertion points**

Contract §6.2: "Every one is dispatched through `openContextMenu` and rendered by the single `<ContextMenu/>`; no row renders its own menu." §9 then has task 22 modify this file to add the session lifecycle verbs with optimistic mutation and an Undo window. That is shared-shape rule 1 territory, and the rule is followed literally: each row kind gets its own item function, and the two functions task 22 replaces say so in a comment that names the task and the file it will import.

**What task 18 renders is only what task 18 can honestly make work.** A menu row that does nothing, or one disabled with the reason "wired in a later task", puts build order in front of a user. So the project, branch and worktree rows get their full non-mutating verb sets — those need only the clipboard, the Tauri opener and `useCreateSession` — and the session row gets the one verb that needs no mutation at all. Every lifecycle verb arrives with task 22, complete with its Undo, or not at all.

Create `desktop/src/components/menu/menu.css`:

```css
@import "../../styles/tokens.css";

/* The single right-click menu. Owns .menu-*. */
.menu {
  display: flex;
  flex-direction: column;
  min-inline-size: 190px;
}
.menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 6px 10px;
  background: none;
  border: 0;
  border-radius: var(--r-chip);
  color: var(--ink);
  cursor: pointer;
  font: inherit;
  text-align: start;
  transition: background-color var(--dur-fast) var(--ease);
}
.menu-item:hover:not(:disabled) {
  background: var(--hover);
}
.menu-item:disabled {
  color: var(--ink-3);
  cursor: default;
}
.menu-item-danger {
  color: var(--fail);
}
.menu-sep {
  block-size: 1px;
  margin: 4px 6px;
  background: var(--line);
}
```

Create `desktop/src/components/menu/ContextMenu.tsx`:

```tsx
// The one right-click menu, for all four row kinds. Contract §6.2 and §6.7.
//
// TASK 22 INSERTION POINTS: sessionItems() and the "Mark ready" entry in worktreeItems().
// Both are marked below. Task 22 must add its call IN THE SAME STEP that defines what the
// call reaches (shared-shape rule 2), and its pre-commit guard must be ANCHORED (rule 3) —
// an unanchored grep for "sessionVerbs" matches this comment.
import { Fragment, useMemo } from "react";
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import {
  selectBranchById,
  selectProjectById,
  selectWorktreeById,
  selectWorktrees,
} from "../../store/selectors";
import { useCreateSession } from "../../hooks/useCreateSession";
import { Popover } from "../common/Popover";
import "./menu.css";

/** One row of the menu. `run: undefined` means disabled, and `disabledReason` is then
 *  mandatory — a control the user cannot operate must say why (contract §6.4, §7.7). */
export interface MenuItem {
  id: string;
  label: string;
  run?: () => void;
  disabledReason?: string;
  danger?: boolean;
  separatorBefore?: boolean;
}

function copyText(text: string): void {
  void navigator.clipboard?.writeText(text);
}

/** Dynamic import so a jsdom test that renders the shell never loads the Tauri IPC bridge. */
function revealInExplorer(path: string): void {
  void import("@tauri-apps/plugin-opener").then((mod) => mod.revealItemInDir(path));
}
```

The same file continues — this is one file, split only so the comments stay readable:

```tsx
export function ContextMenu(): JSX.Element | null {
  const target = useCockpit((s) => s.contextMenu);
  const closeContextMenu = useCockpit((s) => s.closeContextMenu);
  const selectBranch = useCockpit((s) => s.selectBranch);
  const openPicker = useCockpit((s) => s.openPicker);
  // Shared-shape rule 4: the derived reads live in selectors.ts, never inline here. Each hook
  // call is UNCONDITIONAL — the `kind` test is inside the selector argument, so the hook order
  // is identical on every render — and each returns the store's own object, so a poll that
  // changed nothing re-renders nothing.
  const project = useCockpit((s) =>
    target !== null && target.kind === "project" ? selectProjectById(target.id)(s) : undefined,
  );
  const branch = useCockpit((s) =>
    target !== null && target.kind === "branch" ? selectBranchById(target.id)(s) : undefined,
  );
  const worktree = useCockpit((s) =>
    target !== null && target.kind === "worktree" ? selectWorktreeById(target.id)(s) : undefined,
  );
  // The branch verbs create into the branch's FIRST worktree; §3.7's selectWorktrees already
  // returns them in server order, so "first" needs no second ordering rule.
  const branchWorktree = useCockpit((s) =>
    branch === undefined ? undefined : selectWorktrees(branch.id)(s)[0],
  );
  const createSession = useCreateSession();

  // A context menu anchors to a POINT, so the rect is zero-sized. Popover's clamp then does
  // the right thing at a screen edge without a second positioning implementation.
  const anchor = useMemo(
    () => (target === null ? null : new DOMRect(target.x, target.y, 0, 0)),
    [target],
  );

  if (target === null) return null;

  const act = (fn: () => void) => (): void => {
    closeContextMenu();
    fn();
  };

  const projectItems = (): MenuItem[] => {
    if (project === undefined) return [];
    return [
      {
        id: "new-branch",
        label: "New branch",
        disabledReason:
          "Phase 2 has no create-branch endpoint (contract §10.3). Create it in git and it appears here.",
      },
      { id: "copy-path", label: "Copy path", run: act(() => copyText(project.path)) },
      { id: "reveal", label: "Reveal in Explorer", run: act(() => revealInExplorer(project.path)) },
    ];
  };

  const branchItems = (): MenuItem[] => {
    if (branch === undefined) return [];
    const first = branchWorktree;
    return [
      {
        id: "new-session",
        label: "New session here",
        run: first === undefined ? undefined : act(() => void createSession(first.id)),
        disabledReason: first === undefined ? "This branch has no worktrees yet." : undefined,
      },
      { id: "copy-branch", label: "Copy branch name", run: act(() => copyText(branch.name)) },
      {
        id: "branch-defaults",
        label: "Set branch defaults…",
        run: act(() => {
          selectBranch(branch.projectId, branch.id);
          openPicker();
        }),
      },
    ];
  };

  const worktreeItems = (): MenuItem[] => {
    if (worktree === undefined) return [];
    return [
      {
        id: "new-session",
        label: "New session here",
        run: act(() => void createSession(worktree.id)),
      },
      {
        id: "new-shell",
        label: "Open a shell here",
        run: act(() => void createSession(worktree.id, { harness: "shell" })),
      },
      { id: "copy-path", label: "Copy path", run: act(() => copyText(worktree.path)) },
      // ── TASK 22 INSERTION POINT ──────────────────────────────────────────────────────
      // Task 22 appends "Mark ready" here, and ONLY when worktree.ready !== undefined
      // (contract §10.2 and defect 9): while the server does not report the flag the verb
      // is not rendered at all. It PATCHes the worktree and the flag must survive the next
      // list. Anchored guard for task 22's commit step:
      //   grep -cE '^[[:space:]]+id: "mark-ready"' src/components/menu/ContextMenu.tsx
      // ─────────────────────────────────────────────────────────────────────────────────
    ];
  };

  const sessionItems = (id: string): MenuItem[] => {
    // ── TASK 22 INSERTION POINT ──────────────────────────────────────────────────────
    // Task 22 replaces this body with the six lifecycle verbs — Resume, Fork into…,
    // Restart, Rename…, Kill, Forget — built by components/develop/sessionVerbs.ts with
    // optimistic mutation and an Undo toast. Per shared-shape rule 2, the step that adds
    // the call here is the step that defines sessionVerbs.ts; do not add the import first
    // and the module later. Anchored guard for task 22's commit step:
    //   grep -cE '^[[:space:]]+return sessionVerbs\(' src/components/menu/ContextMenu.tsx
    // Until then the only session verb is the one that mutates nothing.
    // ─────────────────────────────────────────────────────────────────────────────────
    return [{ id: "copy-session-id", label: "Copy session id", run: act(() => copyText(id)) }];
  };

  const items =
    target.kind === "project"
      ? projectItems()
      : target.kind === "branch"
        ? branchItems()
        : target.kind === "worktree"
          ? worktreeItems()
          : sessionItems(target.id);

  return (
    <Popover anchor={anchor} onClose={closeContextMenu}>
      <div className="menu" role="menu">
        {items.map((item) => (
          <Fragment key={item.id}>
            {item.separatorBefore === true ? <div className="menu-sep" /> : null}
            <button
              type="button"
              role="menuitem"
              className={item.danger === true ? "menu-item menu-item-danger" : "menu-item"}
              disabled={item.run === undefined}
              title={item.disabledReason}
              onClick={item.run}
            >
              {item.label}
            </button>
          </Fragment>
        ))}
      </div>
    </Popover>
  );
}
```

Notes for review. The menu imports no type from `store/types.ts`: `tsconfig.json` sets `noUnusedLocals`, the four selector reads are already typed by their own return types, and the store types `contextMenu`, so a type-only import here would be a build error rather than documentation. `disabledReason` is rendered as `title`, not as a second line — contract §6.2 says the New-branch reason goes "in the tooltip", and a disabled row that grows to two lines reflows the menu every time the catalogue changes. And the four `useCockpit` reads sit **above** the `if (target === null) return null;` early return, which is not style: a hook called after a conditional return is called on some renders and not others, and React throws `Rendered fewer hooks than expected` the first time the menu closes.

- [ ] **Step 9: `useCreateSession` — the one create path**

**This file is not in §9's list for task 18 and taking it is a deliberate addition.** Every surface that creates a session goes through it: the split button's left half (§6.4), the rail's "New session here" verbs and the worktree row's `＋` (§6.2), the picker's Create (§6.4), and task 23's `⌘⏎` (§6.9). The behaviour is identical at all of them — resolve the inheritance chain, clamp it against the live catalogue, strip the fields the harness does not accept, create, focus the result, and translate the failure into a sentence a user can act on — and writing it once per call site is how they come to differ. The alternative considered and rejected was exporting it from `NewSessionButton.tsx`; that makes the rail's context menu import from `components/topbar/`, which is a dependency edge in the wrong direction and would have task 22 and task 23 both importing a component to get at a function.

The hook reads the store through `getState()` inside the callback rather than subscribing. That is not a micro-optimisation: `selectResolvedDefaults` builds a fresh object on every call, so a subscribing component would re-render on every 2 s poll, and the rail's context menu is mounted whenever the shell is.

Create `desktop/src/hooks/useCreateSession.ts`:

```ts
// The one create-a-session path. Contract §3.6 (inheritance and clamping), §4.4 (createSession
// is 201, not 200), §6.4 (one click for the common case, no dialog).
import { useCallback } from "react";
import { api } from "../api";
import type { ApiError } from "../api/client";
import { resolveDefaults, toCreateRequest } from "../store/defaults";
import { selectResolvedDefaults } from "../store/selectors";
import { useCockpit } from "../store/store";
import type { HarnessId, SessionDefaultsOverride, WorktreeId } from "../store/types";

export interface CreateSessionOptions {
  /** Forces a harness — "Open a shell here" passes "shell". Re-clamped like any override. */
  harness?: HarnessId;
  /** Everything the picker collected. Absent keys inherit; they never reset (§3.6). */
  overrides?: SessionDefaultsOverride;
  openingPrompt?: string;
  title?: string;
}

/** network_unavailable and backend_unavailable are different sentences and must not be
 *  collapsed (§4.2): the first means our control plane is not up, the second means Docker is
 *  not. Telling a user to start Docker when the control plane is dead costs them ten minutes.
 *  Every other code renders the server's own message verbatim — including the sixteenth,
 *  non_loopback_publish, whose sandbox is RUNNING and must never be described as stopped. */
function sentenceFor(error: ApiError): string {
  if (error.code === "network_unavailable") {
    return "The Panacea control plane is not running, so the session was not created.";
  }
  if (error.code === "backend_unavailable") {
    return "Docker is not running on this machine, so the sandbox could not be started.";
  }
  return error.message;
}

export function useCreateSession(): (
  worktreeId: WorktreeId,
  options?: CreateSessionOptions,
) => Promise<void> {
  return useCallback(async (worktreeId: WorktreeId, options?: CreateSessionOptions) => {
    const state = useCockpit.getState();
    const wanted: SessionDefaultsOverride = {
      ...options?.overrides,
      ...(options?.harness === undefined ? {} : { harness: options.harness }),
    };
    // Re-resolve rather than spread: switching harness invalidates model, effort and
    // permission mode, and resolveDefaults is the only place that clamp lives (§3.6).
    const resolved = resolveDefaults(selectResolvedDefaults(state), wanted, undefined, state.harnesses);
    const harness = state.harnesses.find((h) => h.id === resolved.harness);
    if (harness === undefined) {
      state.pushToast("No harness is available on this machine, so no session can be started.");
      return;
    }

    state.setLoading("sessions", true);
    try {
      const session = await api.createSession(
        toCreateRequest(worktreeId, resolved, harness, {
          openingPrompt: options?.openingPrompt,
          title: options?.title,
        }),
      );
      state.upsertSession(session);
      state.setError(null);
      state.selectWorktree(worktreeId);
      // Focus is what makes defect 4's scroll-into-view fire in task 20's ColumnsLayout.
      state.focusSession(session.id);
    } catch (caught) {
      const error = caught as ApiError;
      state.setError({ code: error.code, message: error.message });
      state.pushToast(sentenceFor(error));
    } finally {
      useCockpit.getState().setLoading("sessions", false);
    }
  }, []);
}
```

The `finally` re-reads `getState()` because the actions captured at the top of the callback are bound to the snapshot taken before an `await`; every other action call happens before one. It is one line and it removes a whole class of "the spinner never cleared" bug.

- [ ] **Step 10: Add four selectors to task 17's `selectors.ts`**

Group D shared-shape rule 4: the store is task 17's, nobody else declares a selector, and a later task that needs a derived read **adds it to `selectors.ts`** with a doc comment naming the surface, rather than computing it inline in a component. These four are the whole of task 18's addition to that file, and there is no fifth.

**The handshake with task 17, by name rather than by count.** The four this step adds are `selectProjectById`, `selectBranchById`, `selectWorktreeById` and `selectBranchAttentionSessionId`, and task 17's reserving comment — ``// TASK 18 adds its selectors here, and there is no fifth.`` — names all four with their signatures, so the comment describes what lands rather than a subset of it. If the copy of task 17 you are working from names only `selectBranchAttentionSessionId`, it predates this correction: add all four anyway and report the stale comment rather than dropping three selectors to match it. `ContextMenu` needs the three by-id reads to render anything at all.

**Three of them are by-id reads for the catalogue, and they exist because of that rule.** §3.7 ships `selectSession(id)` and `selectHarness(id)` but nothing for a project, a branch or a worktree by id, and `ContextMenu` is handed nothing but an id and a kind by `openContextMenu`. Two of those three are not lookups but **derivations**: `branchesByProject` and `worktreesByBranch` are `Record`s of arrays, so finding one by id is a flatten-then-find, and doing it inline in the component is precisely what rule 4 forbids — it is untestable without rendering, and task 22, which edits this same menu, would spell it a second way. Each returns the store's own object, so the reference is stable and none of the three needs `useShallow`.

**`selectBranchAttentionSessionId` answers two questions with one read.** Contract §6.2 asks a branch row for two facts about the same thing: its dot is "the **highest-attention status** among its sessions, resolved by the fixed precedence `running > starting > idle > exited`", and "clicking the dot navigates to **the first session carrying that status**". One selector returning that session's **id** answers both — the row reads the status off the session — and because it returns a string or null rather than a freshly built object, it is referentially stable across a poll that changed nothing and needs no `useShallow`.

Append to `desktop/src/store/selectors.ts`, extending its existing `import type { … } from "./types"` list with `Project` and `SessionStatus` — task 17 imports `Branch`, `BranchId`, `ProjectId`, `Worktree` and `WorktreeId` already, and neither of those two:

```ts
/** The project, branch or worktree a surface is about, by id.
 *
 *  ADDED BY TASK 18 — not in contract §3.7, which has selectSession and selectHarness but no
 *  by-id read for the three catalogue levels. `ContextMenu` needs all three: openContextMenu
 *  hands it an id and a kind and nothing else. The branch and worktree forms are DERIVATIONS,
 *  not lookups — both indexes are Records of arrays — which is why shared-shape rule 4 puts
 *  them here rather than inline in the component. Each returns the store's own object, so the
 *  reference is stable across a poll that changed nothing and no useShallow is needed. */
export const selectProjectById =
  (id: ProjectId) =>
  (s: CockpitState): Project | undefined =>
    s.projects.find((p) => p.id === id);

export const selectBranchById =
  (id: BranchId) =>
  (s: CockpitState): Branch | undefined => {
    for (const branches of Object.values(s.branchesByProject)) {
      const found = branches.find((b) => b.id === id);
      if (found !== undefined) return found;
    }
    return undefined;
  };

export const selectWorktreeById =
  (id: WorktreeId) =>
  (s: CockpitState): Worktree | undefined => {
    for (const worktrees of Object.values(s.worktreesByBranch)) {
      const found = worktrees.find((w) => w.id === id);
      if (found !== undefined) return found;
    }
    return undefined;
  };

/** Precedence from contract §6.2. Lower is more attention-worthy. */
const STATUS_RANK: Record<SessionStatus, number> = {
  running: 0,
  starting: 1,
  idle: 2,
  exited: 3,
};

/** The session a branch row's dot is about, or null when the branch has none.
 *
 *  ADDED BY TASK 18 — not in contract §3.7. The rail needs it (§6.2): the dot shows the
 *  highest-attention status among the branch's sessions, and clicking it navigates to the
 *  first session carrying that status. Returning the id rather than the status gives the row
 *  both facts from one read, and a string is referentially stable where a { status, id }
 *  object would re-render every rail row on every 2 s poll.
 *
 *  Worktrees and sessions are walked in server order and the comparison is strict, so "first"
 *  means first in that order — the client never re-sorts (§3.2). */
export const selectBranchAttentionSessionId =
  (branchId: BranchId) =>
  (s: CockpitState): SessionId | null => {
    let best: SessionId | null = null;
    let bestRank = Number.POSITIVE_INFINITY;
    for (const worktree of s.worktreesByBranch[branchId] ?? []) {
      for (const id of s.sessionIdsByWorktree[worktree.id] ?? []) {
        const session = s.sessions[id];
        if (session === undefined) continue;
        const rank = STATUS_RANK[session.status];
        if (rank < bestRank) {
          bestRank = rank;
          best = id;
        }
      }
    }
    return best;
  };
```

The `?? []` here is **not** the banned defensive spelling from shared-shape rule 5. That rule is about `harness.models`, `harness.efforts` and `harness.permissionModes`, which the Go side now allocates unconditionally. These two are `Record` lookups on the client's own index for a key that genuinely may not have been fetched yet, and the only alternative is a non-null assertion that crashes on first paint.

```bash
cd desktop && npm run typecheck
```

**What you must see:** npm's banner and nothing after it. If `tsc` reports `Cannot find name 'SessionStatus'`, `'Project'` or `'CockpitState'`, add them to the file's existing type import rather than re-importing from a second path — one import site per module keeps task 17's file reviewable.

- [ ] **Step 11: Add `machinesOpen` to task 17's `uiSlice`, then write the failing rail test**

First the store addition, and the reason it cannot be component-local. Contract §6.2 gives the machine chip a popover and §6.9 gives `⌘1`-style global chord `⌘⇧M` the job of opening it. Task 23's `useKeyboardMap` is mounted once in `AppShell` and has no handle on `MachineChip`'s internals, so a `useState` inside the chip is unreachable from the keyboard and the chord would have to be dropped or the chip would have to grow a ref forwarded through two components. One boolean in the slice that already holds `paletteOpen` and `pickerOpen` is the consistent answer.

`desktop/src/store/slices/uiSlice.ts` takes **four** additions, not two, and the fourth is the one a reader walks past. Task 17 does not build its initial state from an untyped literal: it declares `type UiState = Pick<UiSlice, "selection" | … | "toasts">` and types `initialUiState` as `UiState`. Add a field to the interface but not to that `Pick` and `machinesOpen: false` becomes an excess property on a typed object literal — `tsc` fails with `TS2353: Object literal may only specify known properties, and 'machinesOpen' does not exist in type 'UiState'`. Task 17 marks three of the four points with a `// TASK 18 adds …` comment; the `Pick` is the one it does not, which is exactly why it gets missed.

**1.** In the `UiSlice` interface, at task 17's reserving comment ``// TASK 18 adds `machinesOpen: boolean;` here.``:

```ts
  /** ADDED BY TASK 18 — not in contract §3.3. The machine chip's popover. It lives here and
   *  not in MachineChip's own state because §6.9's ⌘⇧M must open it from useKeyboardMap,
   *  which is mounted in AppShell and cannot reach a component's local state. */
  machinesOpen: boolean;
```

**2.** In the same interface, at task 17's ``// TASK 18 adds `openMachines(): void;` and `closeMachines(): void;` here.`` comment:

```ts
  openMachines(): void;
  closeMachines(): void;
```

**3.** In the `UiState` **`Pick`**, which carries no reserving comment. Put it on its own line after `"toasts"`, so the guard in step 19 can count it:

```ts
type UiState = Pick<
  UiSlice,
  | "selection" | "lens" | "layout" | "railCollapsed"
  | "expandedProjects" | "expandedBranches" | "expandedWorktrees"
  | "paletteOpen" | "pickerOpen" | "contextMenu" | "sessionMenuFor" | "toasts"
  | "machinesOpen"
>;
```

**4.** In `initialUiState`, at task 17's ``// TASK 18 adds `machinesOpen: false,` here.`` — and the two actions in `createUiSlice`'s body, beside the other `open*`/`close*` pairs:

```ts
  machinesOpen: false,
```

```ts
  openMachines: () => set({ machinesOpen: true }),
  closeMachines: () => set({ machinesOpen: false }),
```

The boolean goes in `initialUiState` and **not** in the creator body. `createUiSlice` opens with `...initialUiState` and `resetCockpit()` re-applies that same object, so a `machinesOpen` written only into the creator would survive a reset and leave the popover open across `resetCockpit()` in a test that never opened it. The two actions are functions, so they stay out of `UiState`, which picks state only, and live in the creator where every other overlay action already is.

Now the test. Seven properties, each one a thing the rail gets wrong if nobody pins it.

Create `desktop/src/components/rail/Rail.test.tsx`:

```tsx
import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Rail } from "./Rail";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { CockpitState } from "../../store/store";

const project = aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\portal" });
const branch = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false });
const worktree = aWorktree({
  id: "w-1",
  projectId: "p-1",
  branchId: "b-1",
  name: "wt-evidence",
  path: "C:\\work\\wt\\evidence",
  sessionCount: 2,
});
const base = { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "claude-code" };
const running = aSession({ ...base, id: "s-run", status: "running", title: "wire the upload route" });
const idling = aSession({ ...base, id: "s-idle", status: "idle", title: "read the spec" });

function seed(over: Partial<CockpitState> = {}): void {
  resetCockpit();
  useCockpit.setState({
    projects: [project],
    branchesByProject: { "p-1": [branch] },
    worktreesByBranch: { "b-1": [worktree] },
    sessions: { "s-run": running, "s-idle": idling },
    sessionIdsByWorktree: { "w-1": ["s-run", "s-idle"] },
    expandedProjects: { "p-1": true },
    expandedBranches: { "b-1": true },
    expandedWorktrees: { "w-1": true },
    ...over,
  });
}

describe("Rail", () => {
  beforeEach(() => seed());

  it("nests project, branch, worktree and session as four levels", () => {
    render(<Rail />);
    const levelOf = (name: RegExp): string | null =>
      screen.getByRole("treeitem", { name }).getAttribute("aria-level");
    expect(levelOf(/engagement-portal/)).toBe("1");
    expect(levelOf(/evidence-upload$/)).toBe("2");
    expect(levelOf(/wt-evidence/)).toBe("3");
    expect(levelOf(/wire the upload route/)).toBe("4");
  });

  it("selects a branch and sets the lens to develop in one click", async () => {
    render(<Rail />);
    await userEvent.click(screen.getByRole("treeitem", { name: /evidence-upload$/ }));
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().lens).toBe("develop");
  });

  it("gives the branch the highest-attention status among its sessions", () => {
    render(<Rail />);
    expect(screen.getByRole("img", { name: "running · evidence-upload" })).toBeDefined();
    expect(screen.queryByRole("img", { name: "idle · evidence-upload" })).toBeNull();
  });

  it("keeps a worktree expandable when only exited sessions remain", () => {
    const done = aSession({ ...base, id: "s-done", status: "exited", title: "read the spec" });
    seed({
      worktreesByBranch: { "b-1": [{ ...worktree, sessionCount: 0 }] },
      sessions: { "s-done": done },
      sessionIdsByWorktree: { "w-1": ["s-done"] },
    });
    render(<Rail />);
    const row = screen.getByRole("treeitem", { name: /wt-evidence/ });
    expect(row.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("treeitem", { name: /read the spec/ })).toBeDefined();
  });

  it("carries names and no numbers", () => {
    render(<Rail />);
    expect(screen.getByRole("tree").textContent ?? "").not.toMatch(/\d/);
  });

  it("moves with the arrow keys and collapses with ArrowLeft", async () => {
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    rows[0].focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(rows[1]);
    await userEvent.keyboard("{ArrowLeft}");
    expect(useCockpit.getState().expandedBranches["b-1"]).toBe(false);
  });

  it("dispatches every right-click through the one context menu", () => {
    render(<Rail />);
    fireEvent.contextMenu(screen.getByRole("treeitem", { name: /evidence-upload$/ }));
    expect(useCockpit.getState().contextMenu?.kind).toBe("branch");
    expect(useCockpit.getState().contextMenu?.id).toBe("b-1");
  });
});
```

Run it:

```bash
cd desktop && npm test -- src/components/rail/Rail.test.tsx
```

**What you must see:** an import-time failure on `./Rail`, zero tests collected.

**`keeps a worktree expandable when only exited sessions remain` is a deliberate departure from contract §6.2**, which says the chevron is drawn "only when `sessionCount > 0`". `Worktree.sessionCount` counts sessions whose status is **not** `exited` (§2), so following §6.2 literally means that killing the last session on a worktree removes its chevron and makes every finished session under it unreachable from the rail — while the same contract insists a killed session keeps its record, stays listed, and is still worth reading because the transcript is the artifact. Those cannot both be true. The rule implemented is therefore: **draw the chevron when `sessionCount > 0` or the store already holds session ids for that worktree.** No number is printed either way, so `docs/AGENTIC-UX.md` §10 rule 10 is untouched.

- [ ] **Step 12: `rail.css`, the machine chip, and the project and branch rows**

One markup decision, made here because every row inherits it. Each row renders as a `role="treeitem"` **element with its children in a sibling `role="group"`**, both inside a plain wrapper, rather than nesting the group inside the treeitem. Strict ARIA allows either, and the sibling form is chosen for a concrete reason: with the group nested, `within(branchRow)` in a test reaches every descendant row's status dot, so "the branch's dot is running" and "one of the branch's sessions is running" become the same assertion and the precedence rule in step 10 is untestable. Every row also carries `data-kind` and `data-id`, which is how step 13's keyboard handler acts on the focused row without a parallel index.

Create `desktop/src/components/rail/rail.css`:

```css
@import "../../styles/tokens.css";

/* The rail. Owns .rail-*. */
.rail {
  display: grid;
  grid-template-rows: auto 1fr auto;
  min-inline-size: 0;
  block-size: 100%;
  background: var(--canvas);
  box-shadow: inset -1px 0 0 var(--line);
  overflow: hidden;
}
.rail-scroll {
  overflow-y: auto;
  padding: 4px;
}
.rail-row {
  display: flex;
  align-items: center;
  gap: 8px;
  inline-size: 100%;
  padding: 4px 6px;
  border-radius: var(--r-ctl);
  color: var(--ink-2);
  cursor: default;
  user-select: none;
  white-space: nowrap;
  transition: background-color var(--dur-fast) var(--ease);
}
.rail-row:hover {
  background: var(--hover);
}
.rail-row[aria-selected="true"] {
  background: var(--accent-tint);
  box-shadow: inset 0 0 0 1px var(--accent-ring);
  color: var(--ink);
}
.rail-name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.rail-chev {
  inline-size: 10px;
  color: var(--ink-3);
  text-align: center;
}
.rail-chev-empty {
  visibility: hidden;
}
.rail-dot-btn {
  display: flex;
  align-items: center;
  padding: 0;
  background: none;
  border: 0;
  color: inherit;
  cursor: pointer;
}
.rail-level-2 {
  padding-inline-start: 18px;
}
.rail-level-3 {
  padding-inline-start: 32px;
}
.rail-level-4 {
  padding-inline-start: 46px;
}
.rail-machine,
.rail-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: none;
  border: 0;
  color: var(--ink-2);
  font: inherit;
  text-align: start;
}
.rail-machine {
  inline-size: 100%;
  box-shadow: inset 0 -1px 0 var(--line);
  cursor: pointer;
}
.rail-badge {
  padding: 1px 6px;
  border-radius: var(--r-chip);
  background: var(--need-tint);
  color: var(--need);
}
/* The machines popover's CONTENT. `.pop` in common.css owns the surface, the elevation and
   the placement; this owns only what goes inside it, which is why the prefix is .rail-*. */
.rail-machine-pop {
  display: grid;
  gap: 6px;
  max-inline-size: 280px;
  padding: 2px 4px;
  color: var(--ink-2);
  white-space: normal;
}
.rail-machine-pop p {
  margin: 0;
  color: var(--ink);
}
.rail-machine-pop ul {
  display: grid;
  gap: 4px;
  margin: 0;
  padding-inline-start: 16px;
}
.rail-footer {
  display: grid;
  gap: 6px;
  box-shadow: inset 0 1px 0 var(--line);
}
.rail-footer-error {
  color: var(--fail);
  white-space: normal;
}
.rail-kbd {
  padding: 1px 5px;
  border-radius: var(--r-chip);
  box-shadow: var(--e-hair);
  color: var(--ink-3);
  font-family: var(--mono);
}
.rail-empty {
  padding: 12px;
  color: var(--ink-3);
  white-space: normal;
}
.rail-add {
  margin-inline-start: auto;
  padding: 0 6px;
  background: none;
  border: 0;
  border-radius: var(--r-chip);
  color: var(--accent-hi);
  cursor: pointer;
  font: inherit;
  opacity: 0;
  transition: opacity var(--dur-fast) var(--ease);
}
.rail-row:hover .rail-add,
.rail-add:focus-visible {
  opacity: 1;
}
```

Create `desktop/src/components/rail/MachineChip.tsx`:

```tsx
// Contract §6.2: the machine name, its state dot, and the `unsandboxed` badge when
// isolation !== "microvm". Click opens the machines popover. Limitations are honest
// sentences rendered VERBATIM and never summarised into a badge (§2).
import { useEffect, useRef, useState } from "react";
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { StatusDot } from "../common/StatusDot";
import { Popover } from "../common/Popover";
import type { Machine } from "../../store/types";
import "./rail.css";

export function MachineChip(props: { machine: Machine | undefined }): JSX.Element {
  const { machine } = props;
  const machinesOpen = useCockpit((s) => s.machinesOpen);
  const openMachines = useCockpit((s) => s.openMachines);
  const closeMachines = useCockpit((s) => s.closeMachines);
  const ref = useRef<HTMLButtonElement | null>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  useEffect(() => {
    setAnchor(machinesOpen && ref.current !== null ? ref.current.getBoundingClientRect() : null);
  }, [machinesOpen]);

  if (machine === undefined) {
    return <div className="rail-empty">No machine yet. The control plane reports none.</div>;
  }

  return (
    <>
      <button
        ref={ref}
        type="button"
        className="rail-machine"
        aria-haspopup="dialog"
        aria-expanded={machinesOpen}
        data-overlay-trigger=""
        onClick={() => (machinesOpen ? closeMachines() : openMachines())}
      >
        <StatusDot
          status={machine.state}
          live={machine.state === "running"}
          label={`${machine.state} · ${machine.name}`}
        />
        <span className="rail-name">{machine.name}</span>
        {machine.isolation === "microvm" ? null : (
          <span className="rail-badge">unsandboxed</span>
        )}
      </button>
      <Popover anchor={anchor} onClose={closeMachines}>
        <div className="rail-machine-pop">
          <p>
            {machine.name} · {machine.state} · {machine.backend}
          </p>
          {machine.message === undefined ? null : <p>{machine.message}</p>}
          <ul>
            {machine.limitations.map((limitation) => (
              <li key={limitation}>{limitation}</li>
            ))}
          </ul>
        </div>
      </Popover>
    </>
  );
}
```

The chip is the owner of contract §7.7's "Docker not running" state: nothing here special-cases it, because `machine.state` is already `error` and `machine.message` already names the cause. Inventing a second sentence for it would be the cockpit guessing at something the server knows.

`data-overlay-trigger` and `aria-expanded` on the button are both load-bearing, and together they are the only reason the toggle works. `useDismissable` listens for `pointerdown` in the **capture** phase on `document`, which fires before the button's `click`; with no attribute, clicking an open chip would close the popover on pointerdown and re-open it on click, so the chip could open the popover and never close it. `data-overlay-trigger` tells the hook that this element is a trigger and owns its own toggle; `aria-expanded={machinesOpen}` tells it *which* overlay this trigger owns is open, which is what keeps the exemption from covering the split button's caret as well. It is also the same fact for a screen reader, which is why it costs nothing.

Create `desktop/src/components/rail/ProjectRow.tsx`:

```tsx
// Contract §6.2: one project row + its chevron. Renders BranchRow children when expanded.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectBranches } from "../../store/selectors";
import { BranchRow } from "./BranchRow";
import type { Project } from "../../store/types";
import "./rail.css";

export function ProjectRow(props: {
  project: Project;
  selected: boolean;
  expanded: boolean;
}): JSX.Element {
  const { project, selected, expanded } = props;
  const branches = useCockpit(useShallow(selectBranches(project.id)));
  const toggleProject = useCockpit((s) => s.toggleProject);
  const selectProject = useCockpit((s) => s.selectProject);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const selectedBranchId = useCockpit((s) => s.selection.branchId);
  const expandedBranches = useCockpit((s) => s.expandedBranches);

  return (
    <div className="rail-node">
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={1}
        aria-expanded={expanded}
        aria-selected={selected}
        data-kind="project"
        data-id={project.id}
        className="rail-row"
        onClick={() => {
          selectProject(project.id);
          toggleProject(project.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({
            kind: "project",
            id: project.id,
            x: event.clientX,
            y: event.clientY,
          });
        }}
      >
        <span className="rail-chev" aria-hidden="true">
          {expanded ? "▾" : "▸"}
        </span>
        <span className="rail-name">{project.name}</span>
      </div>
      {expanded ? (
        <div role="group">
          {branches.map((branch) => (
            <BranchRow
              key={branch.id}
              branch={branch}
              selected={branch.id === selectedBranchId}
              expanded={expandedBranches[branch.id] === true}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
```

Create `desktop/src/components/rail/BranchRow.tsx`:

```tsx
// Contract §6.2: one branch row. Click selects the branch AND sets lens "develop". Its dot is
// the highest-attention status among its sessions, and clicking the dot navigates to the first
// session carrying that status — AGENTIC-UX §10 rule 7, every aggregate names its fact.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectBranchAttentionSessionId, selectWorktrees } from "../../store/selectors";
import { StatusDot } from "../common/StatusDot";
import { WorktreeRow } from "./WorktreeRow";
import type { Branch } from "../../store/types";
import "./rail.css";

export function BranchRow(props: {
  branch: Branch;
  selected: boolean;
  expanded: boolean;
}): JSX.Element {
  const { branch, selected, expanded } = props;
  const worktrees = useCockpit(useShallow(selectWorktrees(branch.id)));
  const attentionId = useCockpit(selectBranchAttentionSessionId(branch.id));
  const attentionStatus = useCockpit((s) =>
    attentionId === null ? undefined : s.sessions[attentionId]?.status,
  );
  const selectBranch = useCockpit((s) => s.selectBranch);
  const toggleBranch = useCockpit((s) => s.toggleBranch);
  const focusSession = useCockpit((s) => s.focusSession);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const expandedWorktrees = useCockpit((s) => s.expandedWorktrees);

  return (
    <div className="rail-node">
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={2}
        aria-expanded={worktrees.length > 0 ? expanded : undefined}
        aria-selected={selected}
        data-kind="branch"
        data-id={branch.id}
        className="rail-row rail-level-2"
        onClick={() => {
          selectBranch(branch.projectId, branch.id);
          toggleBranch(branch.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({ kind: "branch", id: branch.id, x: event.clientX, y: event.clientY });
        }}
      >
        <span
          className={worktrees.length > 0 ? "rail-chev" : "rail-chev rail-chev-empty"}
          aria-hidden="true"
        >
          {expanded ? "▾" : "▸"}
        </span>
        {attentionStatus === undefined ? null : (
          <button
            type="button"
            className="rail-dot-btn"
            onClick={(event) => {
              event.stopPropagation();
              if (attentionId !== null) focusSession(attentionId);
            }}
          >
            <StatusDot
              status={attentionStatus}
              live={attentionStatus === "running"}
              label={`${attentionStatus} · ${branch.name}`}
            />
          </button>
        )}
        <span className="rail-name">{branch.name}</span>
        {branch.isHead ? <span className="rail-badge">HEAD</span> : null}
      </div>
      {expanded ? (
        <div role="group">
          {worktrees.map((worktree) => (
            <WorktreeRow
              key={worktree.id}
              worktree={worktree}
              expanded={expandedWorktrees[worktree.id] === true}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
```

`HEAD` is a word, not a count, so it does not touch `docs/AGENTIC-UX.md` §10 rule 10 — and it is the one fact that tells a user which worktree `git status` would be talking about.

- [ ] **Step 13: The worktree and session rows, the footer, and the `Rail` that owns traversal**

Create `desktop/src/components/rail/WorktreeRow.tsx`:

```tsx
// Contract §6.2, with one departure argued in step 11: the chevron is drawn when
// sessionCount > 0 OR the store already holds session ids for this worktree, so a worktree
// whose sessions have all exited stays expandable and its transcripts stay reachable.
// The count itself is NEVER printed (AGENTIC-UX §10 rule 10).
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectSessionIdsForWorktree } from "../../store/selectors";
import { useCreateSession } from "../../hooks/useCreateSession";
import { SessionRow } from "./SessionRow";
import type { Worktree } from "../../store/types";
import "./rail.css";

export function WorktreeRow(props: { worktree: Worktree; expanded: boolean }): JSX.Element {
  const { worktree, expanded } = props;
  const sessionIds = useCockpit(useShallow(selectSessionIdsForWorktree(worktree.id)));
  const toggleWorktree = useCockpit((s) => s.toggleWorktree);
  const selectWorktree = useCockpit((s) => s.selectWorktree);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const focusedSessionId = useCockpit((s) => s.selection.sessionId);
  const createSession = useCreateSession();

  const hasChildren = worktree.sessionCount > 0 || sessionIds.length > 0;

  return (
    <div className="rail-node">
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={3}
        aria-expanded={hasChildren ? expanded : undefined}
        data-kind="worktree"
        data-id={worktree.id}
        className="rail-row rail-level-3"
        onClick={() => {
          selectWorktree(worktree.id);
          if (hasChildren) toggleWorktree(worktree.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({
            kind: "worktree",
            id: worktree.id,
            x: event.clientX,
            y: event.clientY,
          });
        }}
      >
        <span
          className={hasChildren ? "rail-chev" : "rail-chev rail-chev-empty"}
          aria-hidden="true"
        >
          {expanded ? "▾" : "▸"}
        </span>
        <span className="rail-name">{worktree.name}</span>
        <button
          type="button"
          className="rail-add"
          aria-label={`New session in ${worktree.name}`}
          onClick={(event) => {
            event.stopPropagation();
            void createSession(worktree.id);
          }}
        >
          ＋
        </button>
      </div>
      {expanded ? (
        <div role="group">
          {sessionIds.map((id) => (
            <SessionRow key={id} sessionId={id} selected={id === focusedSessionId} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
```

Create `desktop/src/components/rail/SessionRow.tsx`:

```tsx
// Contract §6.2: status dot + harness command + title. Click FOCUSES the session; it does not
// scroll anything. §6.8 gives horizontal scroll to task 20's ColumnsLayout, which reacts to
// the focus change — a row that called scrollIntoView itself would be a second owner.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { selectHarness, selectSession } from "../../store/selectors";
import { StatusDot } from "../common/StatusDot";
import type { SessionId } from "../../store/types";
import "./rail.css";

export function SessionRow(props: { sessionId: SessionId; selected: boolean }): JSX.Element {
  const { sessionId, selected } = props;
  const session = useCockpit(selectSession(sessionId));
  const command = useCockpit((s) => {
    const found = session === undefined ? undefined : selectHarness(session.harness)(s);
    return found?.command ?? session?.harness ?? "";
  });
  const focusSession = useCockpit((s) => s.focusSession);
  const openContextMenu = useCockpit((s) => s.openContextMenu);

  if (session === undefined) return <></>;

  return (
    <div
      role="treeitem"
      tabIndex={-1}
      aria-level={4}
      aria-selected={selected}
      data-kind="session"
      data-id={session.id}
      className="rail-row rail-level-4"
      onClick={() => focusSession(session.id)}
      onContextMenu={(event) => {
        event.preventDefault();
        openContextMenu({ kind: "session", id: session.id, x: event.clientX, y: event.clientY });
      }}
    >
      {/* TASK 22 NOTE — the rail's session verbs arrive through ContextMenu.tsx's
          sessionItems(); this row's dispatch is already complete and needs no change. If task
          22 wants an optimistic "kill in flight" affordance on the row, it belongs here and
          nowhere else. Anchored guard if it adds one:
            grep -cE '^[[:space:]]+data-pending=' src/components/rail/SessionRow.tsx */}
      <StatusDot
        status={session.status}
        live={session.status === "running"}
        label={`${session.status} · ${session.title}`}
      />
      <span className="rail-name">{command}</span>
      <span className="rail-name">{session.title}</span>
    </div>
  );
}
```

`exitReason` is deliberately not rendered in the rail: §6.2 allows a row one dot and a name, and the reason belongs beside the Restart action in task 20's pane, where there is room to read it.

Create `desktop/src/components/rail/RailFooter.tsx`:

```tsx
// Contract §6.2: the user chip, the ⌘K hint, and THE single API-error line. §7.7 makes this
// the owner of the "control plane not running" state in the rail.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import "./rail.css";

export function RailFooter(): JSX.Element {
  const lastError = useCockpit((s) => s.lastError);

  return (
    <div className="rail-footer">
      <div>
        <span className="rail-name">This machine</span> <span className="rail-kbd">⌘K</span>
      </div>
      {lastError === null ? null : (
        // The server's own sentence, verbatim. There are sixteen error codes and the cockpit
        // does not paraphrase any of them — non_loopback_publish in particular describes a
        // sandbox that IS running, so any locally invented "not running" phrasing is a lie.
        <p className="rail-footer-error" role="status">
          {lastError.message}
        </p>
      )}
    </div>
  );
}
```

Create `desktop/src/components/rail/Rail.tsx`:

```tsx
// Contract §6.2: the scroll container, the role="tree" wrapper, and keyboard traversal within
// the tree. Traversal reads the DOM rather than a parallel flat list, because a second copy of
// "which rows are visible" is a second thing to keep in step with the render — and defect 10's
// whole class is an index taken against one list and applied to another.
import type { JSX, KeyboardEvent } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectDefaultMachine } from "../../store/selectors";
import { MachineChip } from "./MachineChip";
import { ProjectRow } from "./ProjectRow";
import { RailFooter } from "./RailFooter";
import "./rail.css";

export function Rail(): JSX.Element {
  const projects = useCockpit(useShallow((s) => s.projects));
  const machine = useCockpit(selectDefaultMachine);
  const expandedProjects = useCockpit((s) => s.expandedProjects);
  const selectedProjectId = useCockpit((s) => s.selection.projectId);
  const toggleProject = useCockpit((s) => s.toggleProject);
  const toggleBranch = useCockpit((s) => s.toggleBranch);
  const toggleWorktree = useCockpit((s) => s.toggleWorktree);

  const toggle = (kind: string, id: string): void => {
    if (kind === "project") toggleProject(id);
    else if (kind === "branch") toggleBranch(id);
    else if (kind === "worktree") toggleWorktree(id);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    // tsconfig has lib ["ES2020","DOM"] and no DOM.Iterable, so a NodeList cannot be spread.
    const rows = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>('[role="treeitem"]'),
    );
    const active = document.activeElement as HTMLElement | null;
    const index = active === null ? -1 : rows.indexOf(active);
    if (index < 0) return;
    const row = rows[index];
    const kind = row.dataset.kind ?? "";
    const id = row.dataset.id ?? "";
    const expanded = row.getAttribute("aria-expanded");
    const level = Number(row.getAttribute("aria-level") ?? "1");

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        rows[Math.min(index + 1, rows.length - 1)]?.focus();
        break;
      case "ArrowUp":
        event.preventDefault();
        rows[Math.max(index - 1, 0)]?.focus();
        break;
      case "ArrowRight":
        event.preventDefault();
        if (expanded === "false") toggle(kind, id);
        else rows[index + 1]?.focus();
        break;
      case "ArrowLeft":
        event.preventDefault();
        if (expanded === "true") {
          toggle(kind, id);
        } else {
          for (let i = index - 1; i >= 0; i -= 1) {
            if (Number(rows[i].getAttribute("aria-level") ?? "1") < level) {
              rows[i].focus();
              break;
            }
          }
        }
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        row.click();
        break;
      default:
        break;
    }
  };

  return (
    <nav className="rail" aria-label="Projects">
      <MachineChip machine={machine} />
      <div className="rail-scroll" role="tree" aria-label="Projects" onKeyDown={onKeyDown}>
        {projects.length === 0 ? (
          <p className="rail-empty">
            No projects. Set PANACEA_APP_PROJECTS_DIR to a folder of git repositories and
            restart the control plane.
          </p>
        ) : (
          projects.map((project) => (
            <ProjectRow
              key={project.id}
              project={project}
              selected={project.id === selectedProjectId}
              expanded={expandedProjects[project.id] === true}
            />
          ))
        )}
      </div>
      <RailFooter />
    </nav>
  );
}
```

Run the suite:

```bash
cd desktop && npm test -- src/components/rail/Rail.test.tsx
npm run typecheck
```

**What you must see:** all seven tests passing, and a clean `typecheck`. Two failures are likely on the first run and both are real. If `carries names and no numbers` fails, something printed a count — `sessionCount` and `attachCount` are the two candidates and neither may appear in this tree. If `nests project, branch, worktree and session as four levels` reports *multiple elements* for a name, two rows share an accessible name; the fixtures above avoid it deliberately, which is why the worktree is `wt-evidence` and not the branch's own name, and the fix is the fixture rather than a `getAllBy*`.

- [ ] **Step 14: The sub-rail and the honest lens placeholder**

Contract §6.3 and §0.2, and the rule they both turn on: **no lens row is ever disabled, numbered, ordered with arrows, or marked complete.** `docs/AGENTIC-UX.md` §4 rule 12 requires entry into any lens to be legal from any state, so a Files row greyed out because the branch has no worktrees would be the cockpit deciding what the user is allowed to look at. Every lens Phase 2 does not build routes to `LensPlaceholder`, which says what the lens will do and which phase brings it — no spinner, no "coming soon" badge, no progress bar. At the end of *this* task that is all six, because Develop's arm is the slot task 20 fills (step 18); when Phase 2 ships it is the five that are not Develop.

Create `desktop/src/components/lens/LensPlaceholder.tsx`:

```tsx
// Contract §6.4-adjacent (§6.3's table): the honest empty state. What the lens will do, and
// which phase brings it. §7.7: every state is designed, and a skeleton that shifts layout is
// worse than no skeleton.
import type { JSX } from "react";
import type { Lens } from "../../store/types";

export function LensPlaceholder(props: {
  lens: Lens;
  phase: number;
  what: string;
}): JSX.Element {
  const { lens, phase, what } = props;
  return (
    <section className="lens-placeholder" aria-labelledby={`lens-${lens}-title`}>
      <h2 id={`lens-${lens}-title`}>{lens}</h2>
      <p>{what}</p>
      <p>Arrives in phase {phase}.</p>
    </section>
  );
}
```

Create `desktop/src/components/subrail/subrail.css`:

```css
@import "../../styles/tokens.css";

/* The sub-rail. Owns .sub-*. */
.sub {
  display: grid;
  grid-template-rows: auto 1fr;
  block-size: 100%;
  background: var(--canvas);
  box-shadow: inset -1px 0 0 var(--line);
  overflow: hidden;
}
.sub-head {
  display: grid;
  gap: 2px;
  padding: 10px 12px;
  box-shadow: inset 0 -1px 0 var(--line);
}
.sub-back {
  padding: 0;
  background: none;
  border: 0;
  color: var(--ink-3);
  cursor: pointer;
  font: inherit;
  text-align: start;
}
.sub-branch {
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub-list {
  padding: 6px;
}
.sub-row {
  display: flex;
  align-items: center;
  gap: 8px;
  inline-size: 100%;
  padding: 6px 8px;
  background: none;
  border: 0;
  border-radius: var(--r-ctl);
  color: var(--ink-2);
  cursor: pointer;
  font: inherit;
  text-align: start;
  transition: background-color var(--dur-fast) var(--ease);
}
.sub-row:hover {
  background: var(--hover);
}
.sub-row[aria-current="page"] {
  background: var(--accent-tint);
  box-shadow: inset 0 0 0 1px var(--accent-ring);
  color: var(--ink);
}
.sub-glyph {
  inline-size: 14px;
  text-align: center;
}
```

Create `desktop/src/components/subrail/SubRail.tsx`:

```tsx
// Contract §6.3. Six rows, none disabled, none numbered, none marked complete —
// docs/AGENTIC-UX.md §4 rule 12 and §9 item 1.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import type { Lens } from "../../store/types";
import "./subrail.css";

export const LENSES: ReadonlyArray<{ id: Lens; label: string; glyph: string }> = [
  { id: "discover", label: "Discover", glyph: "◇" },
  { id: "design", label: "Design", glyph: "◈" },
  { id: "develop", label: "Develop", glyph: "◆" },
  { id: "files", label: "Files", glyph: "▤" },
  { id: "merge", label: "Merge", glyph: "⑂" },
  { id: "deploy", label: "Deploy", glyph: "▲" },
];

export function SubRail(): JSX.Element {
  const lens = useCockpit((s) => s.lens);
  const setLens = useCockpit((s) => s.setLens);
  const branchId = useCockpit((s) => s.selection.branchId);
  const projectId = useCockpit((s) => s.selection.projectId);
  const selectProject = useCockpit((s) => s.selectProject);
  const branchName = useCockpit((s) => {
    if (projectId === null || branchId === null) return "";
    return (s.branchesByProject[projectId] ?? []).find((b) => b.id === branchId)?.name ?? "";
  });
  const projectName = useCockpit(
    (s) => s.projects.find((p) => p.id === projectId)?.name ?? "",
  );

  return (
    <aside className="sub" aria-label="Phases">
      <div className="sub-head">
        <button type="button" className="sub-back" onClick={() => selectProject(projectId)}>
          ← {projectName}
        </button>
        <span className="sub-branch">{branchName}</span>
      </div>
      <nav className="sub-list">
        {LENSES.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className="sub-row"
            aria-current={lens === entry.id ? "page" : undefined}
            onClick={() => setLens(entry.id)}
          >
            <span className="sub-glyph" aria-hidden="true">
              {entry.glyph}
            </span>
            {entry.label}
          </button>
        ))}
      </nav>
    </aside>
  );
}
```

`LensPlaceholder` has no stylesheet of its own; its two rules live in `shell.css` with the main area that positions it (step 18). §9 gives it no css file and inventing another prefix, in another stylesheet, for two selectors would be worse than the join.

- [ ] **Step 15: The failing test for the split button, then `topbar.css`, `LayoutToggle` and `NewSessionButton`**

Contract §0.1 supersedes the prototype here and the difference is the whole point of the control: the prototype's `＋ session` always opened a dialog, and the spec requires **one click for the common case** with the caret reserved for the picker. A split button whose left half opens the same dialog as its right half is not a split button.

Create `desktop/src/components/topbar/NewSessionButton.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionButton } from "./NewSessionButton";
import { resetCockpit, useCockpit } from "../../store/store";
import { aHarness, aSession, aWorktree } from "../../test/fixtures";

// vi.hoisted, not a bare const: vi.mock is hoisted above the module body, so a factory that
// closed over a plain const would read it in the temporal dead zone.
const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));

const claude = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "",
});

describe("NewSessionButton", () => {
  beforeEach(() => {
    createSession.mockReset();
    createSession.mockResolvedValue(aSession({ id: "s-new", worktreeId: "w-1" }));
    resetCockpit();
    useCockpit.setState({
      harnesses: [claude],
      worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1" })] },
    });
  });

  it("creates with resolved defaults on one click, and opens no dialog", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    await waitFor(() => expect(createSession).toHaveBeenCalledTimes(1));
    expect(createSession.mock.calls[0][0]).toMatchObject({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
    });
    expect(useCockpit.getState().pickerOpen).toBe(false);
  });

  it("opens the picker from the caret and creates nothing", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: "Session options" }));
    expect(useCockpit.getState().pickerOpen).toBe(true);
    expect(createSession).not.toHaveBeenCalled();
  });

  it("says why it cannot create when no worktree is selected", () => {
    render(<NewSessionButton worktreeId={null} />);
    const button = screen.getByRole("button", { name: /new session/i });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.getAttribute("title") ?? "").toMatch(/worktree/i);
  });

  it("focuses the created session, which is what makes defect 4's scroll fire", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
  });
});
```

Run it and see it fail on the unresolved `./NewSessionButton`. Then create `desktop/src/components/topbar/topbar.css`:

```css
@import "../../styles/tokens.css";

/* The top bar, and the panel the session picker renders into. Owns .top-* and .pick-*. */
.top {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: var(--page);
  box-shadow: inset 0 -1px 0 var(--line);
}
.top-spacer {
  margin-inline-start: auto;
}
.top-seg {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: var(--r-ctl);
  box-shadow: var(--e-hair);
}
.top-seg-btn {
  padding: 3px 10px;
  background: none;
  border: 0;
  border-radius: var(--r-chip);
  color: var(--ink-2);
  cursor: pointer;
  font: inherit;
  transition:
    background-color var(--dur-fast) var(--ease),
    color var(--dur-fast) var(--ease);
}
.top-seg-btn[aria-pressed="true"] {
  background: var(--inset);
  color: var(--ink);
}
.top-btn {
  padding: 4px 10px;
  background: var(--surface);
  border: 0;
  border-radius: var(--r-ctl);
  box-shadow: var(--e-btn);
  color: var(--ink);
  cursor: pointer;
  font: inherit;
  transition: opacity var(--dur-fast) var(--ease);
}
.top-btn:disabled {
  color: var(--ink-3);
  cursor: default;
  opacity: 0.7;
}
.top-split {
  display: flex;
}
.top-split-main {
  padding: 4px 12px;
  background: var(--accent);
  border: 0;
  border-start-start-radius: var(--r-ctl);
  border-end-start-radius: var(--r-ctl);
  color: var(--accent-ink);
  cursor: pointer;
  font: inherit;
  transition: background-color var(--dur-fast) var(--ease);
}
.top-split-main:hover:not(:disabled) {
  background: var(--accent-hi);
}
.top-split-main:disabled {
  background: var(--inset);
  color: var(--ink-3);
  cursor: default;
}
.top-split-caret {
  padding: 4px 8px;
  margin-inline-start: 1px;
  background: var(--accent);
  border: 0;
  border-start-end-radius: var(--r-ctl);
  border-end-end-radius: var(--r-ctl);
  color: var(--accent-ink);
  cursor: pointer;
  font: inherit;
}
/* §6.4 gives SessionPicker no anchor prop, so it cannot be positioned from a rect. It is
   placed under the top bar's trailing edge, where the split button that opens it sits. */
.pick-panel {
  inset-block-start: 52px;
  inset-inline-end: 12px;
}
.pick {
  display: grid;
  gap: 8px;
  min-inline-size: 300px;
  padding: 4px;
}
.pick-field {
  display: grid;
  gap: 3px;
}
.pick-note {
  color: var(--ink-3);
  white-space: normal;
}
.pick-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
```

Create `desktop/src/components/topbar/LayoutToggle.tsx`:

```tsx
// Contract §6.4: the Columns/Canvas segmented control. Rendered in every lens, because §6.4
// lists it unconditionally and the choice is persisted — flipping it in Files and finding
// Develop already in Canvas when you return is the behaviour the persisted preference implies.
import type { JSX } from "react";
import type { Layout } from "../../store/types";
import "./topbar.css";

const OPTIONS: ReadonlyArray<{ id: Layout; label: string }> = [
  { id: "columns", label: "Columns" },
  { id: "canvas", label: "Canvas" },
];

export function LayoutToggle(props: {
  layout: Layout;
  onChange: (l: Layout) => void;
}): JSX.Element {
  const { layout, onChange } = props;
  return (
    <div className="top-seg" role="group" aria-label="Layout">
      {OPTIONS.map((option) => (
        <button
          key={option.id}
          type="button"
          className="top-seg-btn"
          aria-pressed={layout === option.id}
          onClick={() => onChange(option.id)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
```

Create `desktop/src/components/topbar/NewSessionButton.tsx`:

```tsx
// Contract §6.4 and §0.1: the LEFT half creates with resolved defaults and opens no dialog;
// the caret opens the picker. The prototype opened a dialog for both and the spec supersedes it.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { useCreateSession } from "../../hooks/useCreateSession";
import type { WorktreeId } from "../../store/types";
import "./topbar.css";

export function NewSessionButton(props: { worktreeId: WorktreeId | null }): JSX.Element {
  const { worktreeId } = props;
  const createSession = useCreateSession();
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const openPicker = useCockpit((s) => s.openPicker);
  const closePicker = useCockpit((s) => s.closePicker);

  return (
    <div className="top-split">
      <button
        type="button"
        className="top-split-main"
        disabled={worktreeId === null}
        title={
          worktreeId === null
            ? "Select a worktree in the rail first — a session runs in one."
            : undefined
        }
        onClick={() => {
          if (worktreeId !== null) void createSession(worktreeId);
        }}
      >
        ＋ New session
      </button>
      <button
        type="button"
        className="top-split-caret"
        aria-label="Session options"
        aria-haspopup="dialog"
        aria-expanded={pickerOpen}
        data-overlay-trigger=""
        onClick={() => (pickerOpen ? closePicker() : openPicker())}
      >
        ▾
      </button>
    </div>
  );
}
```

Run it:

```bash
cd desktop && npm test -- src/components/topbar/NewSessionButton.test.tsx
npm run typecheck
```

**What you must see:** four passing tests. If the first fails with `createSession` called zero times, the mock path is wrong — `useCreateSession` imports `{ api } from "../api"`, so the `vi.mock` specifier must resolve to the same module, which from `src/components/topbar/` is `"../../api"`. If it fails on `model: "opus"`, `resolveDefaults` clamped an empty app-level model to the harness's `defaultModel` and the fixture's `defaultModel` is not `"opus"`; fix the fixture, not the clamp.

- [ ] **Step 16: The failing test for the picker, then `SessionPicker`**

Every rule in contract §6.4's picker list is a rule about **the harness descriptor deciding what is on screen**, never a hard-coded `harness === "claude-code"`. The model control is *hidden*, not disabled, when `models` is empty; effort appears only when `efforts` is non-empty; permission mode only when `permissionModes` is; the opening prompt only when `supportsOpeningPrompt`, and it is never required. An unavailable harness is **listed with its note verbatim**, because hiding Codex because it is not installed makes "why can't I use Codex?" unanswerable from the interface. And submitting goes through `toCreateRequest`, so a field the harness does not accept is never sent and a 400 the client could have avoided never happens.

**The amendment to §6.4, and why.** §6.2 puts "Set branch defaults…" in the branch row's verbs and no section of the contract gives that verb a surface — there is no defaults editor anywhere in §6. The picker already collects exactly the four fields `SessionDefaultsOverride` holds, so it gains a **secondary action, "Save as branch default"**, which writes the current draft through `prefsSlice.setBranchDefaults` and closes without creating anything. The alternatives were a second dialog (two surfaces for one set of fields) or a new `uiSlice` field for a defaults mode (a bigger edit to task 17's file than a selector). This one adds a button.

Create `desktop/src/components/topbar/SessionPicker.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPicker } from "./SessionPicker";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";

const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));

const claude = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "",
});
const shell = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
  supportsOpeningPrompt: false,
  supportsResume: false,
  note: "Always available: the sandbox image has bash.",
});
const codex = aHarness({
  id: "codex",
  name: "Codex",
  command: "codex",
  installed: false,
  flagsVerified: false,
  available: false,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
  supportsOpeningPrompt: true,
  supportsResume: false,
  note: "Not installed on this machine.",
});

describe("SessionPicker", () => {
  beforeEach(() => {
    createSession.mockReset();
    createSession.mockResolvedValue(aSession({ id: "s-new", worktreeId: "w-1" }));
    resetCockpit();
    useCockpit.setState({
      harnesses: [claude, shell, codex],
      projects: [aProject({ id: "p-1" })],
      branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
      worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1", name: "wt-one" })] },
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
    });
  });

  it("lists an unavailable harness with its note, rather than hiding it", () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    expect(screen.getByRole("option", { name: /Not installed on this machine\./ })).toBeDefined();
  });

  it("hides model, effort and permission mode for a harness that declares none", async () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    expect(screen.getByLabelText("Model")).toBeDefined();
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    expect(screen.queryByLabelText("Model")).toBeNull();
    expect(screen.queryByLabelText("Effort")).toBeNull();
    expect(screen.queryByLabelText("Permission mode")).toBeNull();
    expect(screen.queryByLabelText("Opening prompt")).toBeNull();
  });

  it("sends only the fields the chosen harness accepts", async () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(screen.getByRole("button", { name: "Create session" }));
    const body = createSession.mock.calls[0][0] as Record<string, unknown>;
    expect(body.harness).toBe("shell");
    expect(body).not.toHaveProperty("model");
    expect(body).not.toHaveProperty("effort");
    expect(body).not.toHaveProperty("permissionMode");
  });

  it("saves the draft as the branch default and creates nothing", async () => {
    const onClose = vi.fn();
    render(<SessionPicker open worktreeId="w-1" onClose={onClose} />);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(screen.getByRole("button", { name: "Save as branch default" }));
    expect(useCockpit.getState().branchDefaults["b-1"]).toMatchObject({ harness: "shell" });
    expect(createSession).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders nothing when closed", () => {
    render(<SessionPicker open={false} worktreeId="w-1" onClose={() => {}} />);
    expect(screen.queryByLabelText("Harness")).toBeNull();
  });
});
```

Run it and see the import-time failure on `./SessionPicker`.

Create `desktop/src/components/topbar/SessionPicker.tsx`:

```tsx
// Contract §6.4. Every control is gated on the HARNESS DESCRIPTOR, never on a harness id.
// The draft lives here and dies with the popover (§3.3: there is no drafts map in the store).
import { useEffect, useState } from "react";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { resolveDefaults } from "../../store/defaults";
import { useCreateSession } from "../../hooks/useCreateSession";
import { useDismissable } from "../../hooks/useDismissable";
import type {
  Effort,
  PermissionMode,
  SessionDefaultsOverride,
  WorktreeId,
} from "../../store/types";
import "./topbar.css";

const EFFORT_LABEL: Record<Effort, string> = {
  "": "harness default",
  low: "low",
  medium: "medium",
  high: "high",
  xhigh: "xhigh",
};

export function SessionPicker(props: {
  open: boolean;
  worktreeId: WorktreeId | null;
  onClose: () => void;
}): JSX.Element | null {
  const { open, worktreeId, onClose } = props;
  const harnesses = useCockpit(useShallow((s) => s.harnesses));
  const inherited = useCockpit(useShallow(selectResolvedDefaults));
  const branchId = useCockpit((s) => s.selection.branchId);
  const worktrees = useCockpit(useShallow(selectWorktrees(branchId ?? "")));
  const setBranchDefaults = useCockpit((s) => s.setBranchDefaults);
  const createSession = useCreateSession();
  const ref = useDismissable<HTMLDivElement>(open, onClose);

  const [draft, setDraft] = useState<SessionDefaultsOverride>({});
  const [prompt, setPrompt] = useState("");
  const [target, setTarget] = useState<WorktreeId | null>(worktreeId);

  useEffect(() => {
    if (!open) return;
    setDraft({});
    setPrompt("");
    setTarget(worktreeId);
  }, [open, worktreeId]);

  // Re-resolve rather than merge: the clamp in §3.6 is what keeps a stale branch default from
  // offering a model the chosen harness does not have.
  const effective = resolveDefaults(inherited, draft, undefined, harnesses);
  const harness = harnesses.find((h) => h.id === effective.harness);

  if (!open || harness === undefined) return null;

  return (
    <div ref={ref} role="dialog" aria-label="New session" tabIndex={-1} className="pop pick-panel">
      <div className="pick">
        <label className="pick-field">
          Harness
          <select
            value={effective.harness}
            // Replacing the draft rather than spreading is deliberate: a new harness
            // invalidates model, effort and permission mode, and resolveDefaults re-derives
            // all three from the descriptor on the next render.
            onChange={(event) => setDraft({ harness: event.target.value })}
          >
            {harnesses.map((entry) => (
              <option key={entry.id} value={entry.id}>
                {entry.available ? entry.name : `${entry.name} — ${entry.note}`}
              </option>
            ))}
          </select>
        </label>

        {harness.models.length === 0 ? null : (
          <label className="pick-field">
            Model
            <select
              value={effective.model}
              onChange={(event) => setDraft({ ...draft, model: event.target.value })}
            >
              {harness.models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </label>
        )}

        {harness.efforts.length === 0 ? null : (
          <label className="pick-field">
            Effort
            <select
              value={effective.effort}
              onChange={(event) =>
                setDraft({ ...draft, effort: event.target.value as Effort })
              }
            >
              {harness.efforts.map((effort) => (
                <option key={effort} value={effort}>
                  {EFFORT_LABEL[effort]}
                </option>
              ))}
            </select>
          </label>
        )}

        {harness.permissionModes.length === 0 ? null : (
          <label className="pick-field">
            Permission mode
            <select
              value={effective.permissionMode}
              onChange={(event) =>
                setDraft({ ...draft, permissionMode: event.target.value as PermissionMode })
              }
            >
              {harness.permissionModes.map((mode) => (
                <option key={mode} value={mode}>
                  {mode}
                </option>
              ))}
            </select>
          </label>
        )}

        <label className="pick-field">
          Worktree
          <select value={target ?? ""} onChange={(event) => setTarget(event.target.value)}>
            {worktrees.map((worktree) => (
              <option key={worktree.id} value={worktree.id}>
                {worktree.name}
              </option>
            ))}
          </select>
        </label>

        {harness.supportsOpeningPrompt ? (
          <label className="pick-field">
            Opening prompt
            <textarea
              rows={3}
              value={prompt}
              placeholder="Blank just opens the session."
              onChange={(event) => setPrompt(event.target.value)}
            />
          </label>
        ) : null}

        {harness.note === "" ? null : <p className="pick-note">{harness.note}</p>}

        <div className="pick-actions">
          <button
            type="button"
            className="top-btn"
            disabled={branchId === null}
            title={branchId === null ? "Select a branch first." : undefined}
            onClick={() => {
              if (branchId !== null) setBranchDefaults(branchId, draft);
              onClose();
            }}
          >
            Save as branch default
          </button>
          <button
            type="button"
            className="top-split-main"
            disabled={target === null || !harness.available}
            title={harness.available ? undefined : harness.note}
            onClick={() => {
              if (target !== null) {
                void createSession(target, { overrides: draft, openingPrompt: prompt });
              }
              onClose();
            }}
          >
            Create session
          </button>
        </div>
      </div>
    </div>
  );
}
```

Run it:

```bash
cd desktop && npm test -- src/components/topbar/SessionPicker.test.tsx
npm run typecheck
```

**What you must see:** five passing tests. The one most likely to fail first is *sends only the fields the chosen harness accepts*, and if it fails with `model` present, `toCreateRequest` was bypassed — `useCreateSession` must be the only path to `api.createSession` in this component.

- [ ] **Step 17: `TopBar` — including the two controls that are honest about being disabled**

Contract §6.4: Authority and Run locally are in `docs/FUNCTIONAL-SPEC.md`'s top bar, so deleting them would be a silent scope cut, and rendering them live would be a lie. **A disabled control with a reason is the honest third option**, and the reason names the phase that makes it real. The tooltip is the reason, not "coming soon".

The worktree the split button acts on is `selection.worktreeId`, falling back to the selected branch's first worktree — the same fallback §6.9 specifies for `⌘⏎`, written once here so task 23 can reuse the resolved value rather than re-deriving it and drifting.

Create `desktop/src/components/topbar/TopBar.tsx`:

```tsx
// Contract §6.4: the bar. Layout toggle, Authority, Run locally, the split New-session button.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { LayoutToggle } from "./LayoutToggle";
import { NewSessionButton } from "./NewSessionButton";
import { SessionPicker } from "./SessionPicker";
import "./topbar.css";

export function TopBar(): JSX.Element {
  const layout = useCockpit((s) => s.layout);
  const setLayout = useCockpit((s) => s.setLayout);
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const closePicker = useCockpit((s) => s.closePicker);
  const branchId = useCockpit((s) => s.selection.branchId);
  // §6.9's ⌘⏎ rule, written once: the selected worktree, else the branch's first.
  const worktreeId = useCockpit((s) =>
    s.selection.worktreeId ??
    (branchId === null ? null : ((s.worktreesByBranch[branchId] ?? [])[0]?.id ?? null)),
  );

  return (
    <header className="top">
      <LayoutToggle layout={layout} onChange={setLayout} />
      <span className="top-spacer" />
      <button
        type="button"
        className="top-btn"
        disabled
        title="Authority needs the agent-runtime signal (docs/AGENTIC-UX.md §7). It arrives in Phase 3."
      >
        Authority
      </button>
      <button
        type="button"
        className="top-btn"
        disabled
        title="Run locally needs the deploy connector. It arrives in Phase 5."
      >
        Run locally
      </button>
      <NewSessionButton worktreeId={worktreeId} />
      <SessionPicker open={pickerOpen} worktreeId={worktreeId} onClose={closePicker} />
    </header>
  );
}
```

- [ ] **Step 18: `AppShell`, `shell.css`, and the slots later tasks fill**

Contract §6.1: the three-column grid, the `.grain` layer, the single `useKeyboardMap()` mount and `<Toasts/>`; it owns **no** focus and **no** scroll. The grid transitions `grid-template-columns`, which is a layout property, and §6.1 names this as the one exception the plan allows — bounded to two elements, once per toggle, and inert under `prefers-reduced-motion` via the blanket `@media` block task 14 put in `tokens.css`. Do not add a second layout transition anywhere else.

**The reserved points in this file, and one of them adds a file to another task's list.** §9 has task 20 create `DevelopLens.tsx` and gives no task the job of mounting it, so `AppShell` reserves the Develop arm of its lens switch and **task 20 must also modify `desktop/src/components/AppShell.tsx`** — an addition to §9's task-20 row, flagged here so its author does not discover it as a compile error. Task 23 already has `AppShell` in its list, and takes two points of its own: `useKeyboardMap()` at the top of the component and `<CommandPalette/>` in the tree. Each point carries an anchored guard, per shared-shape rule 3, because an unanchored `grep -c "<DevelopLens"` matches the reserving comment and reports a mount that is not there.

Create `desktop/src/components/shell.css`:

```css
@import "../styles/tokens.css";

/* The shell grid. Owns .shell-* and the two rules LensPlaceholder needs.
   One `../`, not two: this file sits in src/components/, not in a subdirectory of it. */
.shell {
  --rail-w: 248px;
  --sub-w: 0px;
  display: grid;
  grid-template-columns: var(--rail-w) var(--sub-w) 1fr;
  block-size: 100vh;
  background: var(--page);
  color: var(--ink);
  overflow: hidden;
  /* The ONE layout transition this plan allows (§6.1). A rail that appears instantaneously
     reads as a glitch. prefers-reduced-motion disables it via tokens.css. */
  transition: grid-template-columns var(--dur) var(--ease);
}
.shell-deep {
  --sub-w: 182px;
}
.shell-rail-collapsed {
  --rail-w: 0px;
}
.shell-main {
  display: grid;
  grid-template-rows: auto 1fr;
  min-inline-size: 0;
  overflow: hidden;
}
.shell-lens {
  min-block-size: 0;
  overflow: hidden;
}
.lens-placeholder {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 8px;
  block-size: 100%;
  padding: 24px;
  color: var(--ink-2);
  text-align: center;
}
.lens-placeholder h2 {
  color: var(--ink);
  text-transform: capitalize;
}
```

Create `desktop/src/components/AppShell.tsx`:

```tsx
// Contract §6.1. Owns the grid, the grain layer, the one keyboard-map mount (task 23) and the
// overlays that belong to no surface. Owns NO focus and NO scroll — §6.8.
import type { JSX } from "react";
import { useCockpit } from "../store/store";
import { Rail } from "./rail/Rail";
import { SubRail } from "./subrail/SubRail";
import { TopBar } from "./topbar/TopBar";
import { LensPlaceholder } from "./lens/LensPlaceholder";
import { ContextMenu } from "./menu/ContextMenu";
import { Toasts } from "./common/Toasts";
import type { Lens } from "../store/types";
import "./shell.css";

function LensRouter(props: { lens: Lens }): JSX.Element {
  switch (props.lens) {
    case "develop":
      // ── TASK 20 INSERTION POINT ────────────────────────────────────────────────────
      // Task 20 replaces the next line with `return <DevelopLens />;` and adds the import.
      // This makes AppShell.tsx a MODIFIED file for task 20, which §9's task-20 row does
      // not list; §9 gives DevelopLens no mounter at all, so the addition is unavoidable.
      // Per shared-shape rule 2, the step that adds the line is the step that defines
      // DevelopLens. Anchored guard for task 20's commit step:
      //   grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx
      // ───────────────────────────────────────────────────────────────────────────────
      return (
        <LensPlaceholder
          lens="develop"
          phase={2}
          what="Sessions on this branch, as columns or a canvas."
        />
      );
    case "discover":
      return (
        <LensPlaceholder
          lens="discover"
          phase={4}
          what="Turn a conversation into a spec this branch can be measured against."
        />
      );
    case "design":
      return (
        <LensPlaceholder
          lens="design"
          phase={5}
          what="A canvas of real HTML frames an agent can edit directly."
        />
      );
    case "files":
      return (
        <LensPlaceholder
          lens="files"
          phase={3}
          what="The worktree's files, with what each session changed."
        />
      );
    case "merge":
      return (
        <LensPlaceholder
          lens="merge"
          phase={3}
          what="Review a worktree's diff and land it on the branch."
        />
      );
    case "deploy":
      return (
        <LensPlaceholder
          lens="deploy"
          phase={5}
          what="Ship the branch, with the spec attached to the release."
        />
      );
    default:
      return <></>;
  }
}

export function AppShell(): JSX.Element {
  const railCollapsed = useCockpit((s) => s.railCollapsed);
  const branchId = useCockpit((s) => s.selection.branchId);
  const lens = useCockpit((s) => s.lens);

  // ── TASK 23 INSERTION POINT ────────────────────────────────────────────────────────
  // Task 23 adds `useKeyboardMap();` here — ONE window keydown listener for the whole app
  // (§6.8), and it must delegate Escape to closeTopOverlay() from hooks/useDismissable
  // rather than tracking overlay state itself. Anchored guard for task 23:
  //   grep -cE '^[[:space:]]+useKeyboardMap\(\);' src/components/AppShell.tsx
  // ───────────────────────────────────────────────────────────────────────────────────

  const className = ["shell", "grain"]
    .concat(branchId === null ? [] : ["shell-deep"])
    .concat(railCollapsed ? ["shell-rail-collapsed"] : [])
    .join(" ");

  return (
    <div className={className}>
      <Rail />
      {branchId === null ? <div aria-hidden="true" /> : <SubRail />}
      <main className="shell-main">
        <TopBar />
        <div className="shell-lens">
          <LensRouter lens={lens} />
        </div>
      </main>
      <ContextMenu />
      <Toasts />
      {/* ── TASK 23 INSERTION POINT ──────────────────────────────────────────────────
          Task 23 mounts <CommandPalette /> here. Anchored guard:
            grep -cE '^[[:space:]]+<CommandPalette' src/components/AppShell.tsx
          ───────────────────────────────────────────────────────────────────────── */}
    </div>
  );
}
```

The empty `<div aria-hidden="true" />` in the sub-rail column is not filler: the grid always has three columns, and the middle one animating from `0px` to `182px` is the sub-rail reveal §6.1 describes. Unmounting the column entirely would make the grid jump instead of slide, and mounting `SubRail` with no branch would render a back-link to nothing.

`default: return <></>;` is unreachable — `Lens` is a closed six-member union — but `tsconfig.json` sets `noFallthroughCasesInSwitch` and a `switch` returning `JSX.Element` needs a terminal return for the compiler. It is one line and it is cheaper than a non-null assertion.

- [ ] **Step 19: Run everything, and prove the mounts exist rather than the comments**

```bash
cd desktop
npm test
npm run typecheck
```

**What you must see:** every test file in the repository passing, including tasks 14 to 17's, and a `typecheck` that prints npm's banner and nothing after it. This task adds **26 tests across five files** — six in `hooks/heartbeat.test.ts`, four in `common/StatusDot.test.tsx`, seven in `rail/Rail.test.tsx`, four in `topbar/NewSessionButton.test.tsx` and five in `topbar/SessionPicker.test.tsx`. If a test in another file starts failing only when the whole suite runs, suspect module-level state: `__resetHeartbeat()` and `__resetOverlays()` exist precisely because vitest's `restoreMocks`/`clearMocks` do not reset it (§1.6).

Now the guards. Shared-shape rule 3 exists because a naive `grep -c "<CanvasLayout"` returns two when the reserving comment contains the same string — measured on the Go side as 2 where the truth was 0. Every one of these is anchored on a line whose first non-space character begins the code, so a `//` or `{/* */}` comment cannot satisfy it:

```bash
grep -cE '^[[:space:]]+<Toasts />' src/components/AppShell.tsx          # expect 1
grep -cE '^[[:space:]]+<ContextMenu />' src/components/AppShell.tsx     # expect 1
grep -cE '^[[:space:]]+<Rail />' src/components/AppShell.tsx            # expect 1
grep -cE '^[[:space:]]+return <DevelopLens' src/components/AppShell.tsx # expect 0 — task 20
grep -cE '^[[:space:]]+useKeyboardMap\(\);' src/components/AppShell.tsx # expect 0 — task 23
grep -cE '^[[:space:]]+<CommandPalette' src/components/AppShell.tsx     # expect 0 — task 23
```

**What you must see:** `1 1 1 0 0 0`. The three zeros are the reserved points and they must stay zero until their task lands; the three ones are this task's own mounts and a zero there means a component that exists only in its own tests.

Then the other half of the handshake: the two files of task 17's that this task edits. Task 17's own commit step greps those two files expecting `0`, which is how it proves it left task 18's code unwritten; after this task the same declarations are present. Anchored for the reason §3A gives — task 17 writes `machinesOpen` and `selectBranchAttentionSessionId` into its reserving comments, so an unanchored `grep -c machinesOpen src/store/slices/uiSlice.ts` counts those comments alongside the code and cannot tell you whether the code landed at all:

```bash
grep -cE '^[[:space:]]*machinesOpen: boolean;' src/store/slices/uiSlice.ts   # expect 1
grep -cE '^[[:space:]]*machinesOpen: false,' src/store/slices/uiSlice.ts     # expect 1
grep -c '"machinesOpen"' src/store/slices/uiSlice.ts                         # expect 1 — the UiState Pick
grep -cE '^export const select(Project|Branch|Worktree)ById' src/store/selectors.ts          # expect 3
grep -cE '^export const selectBranchAttentionSessionId' src/store/selectors.ts                # expect 1
```

**What you must see:** `1 1 1 3 1`. The third line is the one step 11 warns about: it is the only guard that catches a `machinesOpen` added to the interface and to `initialUiState` but not to the `UiState` `Pick`, and the double-quoted spelling is what keeps it off task 17's two reserving comments, which write the name in backticks and never in quotes.

Four design-system checks this task can run itself, ahead of task 24's audit:

```bash
grep -rnE '#[0-9a-fA-F]{3,8}' src/components src/hooks | grep -v 'components/Terminal.tsx'
grep -rnE '^[[:space:]]*transition:[[:space:]]*all\b' src/
grep -rn "dangerouslySetInnerHTML" src/
grep -cE '^@import "(\.\./)+styles/tokens\.css";' \
  src/components/common/common.css src/components/menu/menu.css \
  src/components/rail/rail.css src/components/subrail/subrail.css \
  src/components/topbar/topbar.css src/components/shell.css
```

**What you must see:** nothing at all from the first three, and `:1` from each of the six stylesheets on the fourth. Every colour comes from a token, no transition is a blanket, `Toast.message` is text, and every stylesheet opens with the token layer — shared-shape §3B rule 1, and the thing task 24's audit asserts first.

Two of those are **scoped on purpose**, per shared-shape §3A, because the obvious form fails against correct code. `src/components/Terminal.tsx` is Phase 1's and carries the xterm palette as two literal hexes; it is not this task's file, task 19 owns it, and an unfiltered hex grep reports it as this task's defect. And a bare `grep -rn "transition: all" src/` matches **task 14's own test title** — ``it("never uses `transition: all`", …)`` in `src/styles/tokens.test.ts` — so it prints a line against a tree in which nothing is wrong. Anchoring on the declaration's first character fixes it: that line begins `it(`, not `transition:`.

**One thing to hand task 24 rather than let it discover.** The properties this task transitions are `background-color`, `color`, `opacity` and — once, in `shell.css` — `grid-template-columns`. §7.5 says "`transform` and `opacity` only", and the two deviations are deliberate and bounded: `grid-template-columns` is the exception §6.1 names by hand for the rail collapse and the sub-rail reveal, and `background-color`/`color` appear only on hover and selected states of small row-sized elements, which §7.5's own flash rule already permits ("on `opacity`/`background` only"). If task 24's `designsystem.test.ts` asserts the strict two-property form, it should encode these as the named allow-list rather than either test being softened to nothing.

**A note on what this task did not verify.** No screenshot was taken and none would be evidence: `CLAUDE.md` says the corporate network blocks headless-browser screenshots and that visual acceptance is the product owner's eyes. Everything above is a unit assertion under jsdom. The shell has never been rendered in a real webview at the end of this task, because `App.tsx` still does not mount it — task 24 does that, and that is the first moment anyone sees this.

- [ ] **Step 20: Commit**

Stage by name. Never `git add -A`, `-u` or `.` — Group D shared-shape §6, and other agents are working in this tree.

```bash
# from the repository root, not from desktop/
git add \
  desktop/src/hooks/heartbeat.ts \
  desktop/src/hooks/heartbeat.test.ts \
  desktop/src/hooks/useDismissable.ts \
  desktop/src/hooks/useCreateSession.ts \
  desktop/src/components/common/common.css \
  desktop/src/components/common/StatusDot.tsx \
  desktop/src/components/common/StatusDot.test.tsx \
  desktop/src/components/common/Popover.tsx \
  desktop/src/components/common/Toasts.tsx \
  desktop/src/components/menu/ContextMenu.tsx \
  desktop/src/components/menu/menu.css \
  desktop/src/components/rail/rail.css \
  desktop/src/components/rail/MachineChip.tsx \
  desktop/src/components/rail/ProjectRow.tsx \
  desktop/src/components/rail/BranchRow.tsx \
  desktop/src/components/rail/WorktreeRow.tsx \
  desktop/src/components/rail/SessionRow.tsx \
  desktop/src/components/rail/RailFooter.tsx \
  desktop/src/components/rail/Rail.tsx \
  desktop/src/components/rail/Rail.test.tsx \
  desktop/src/components/subrail/SubRail.tsx \
  desktop/src/components/subrail/subrail.css \
  desktop/src/components/lens/LensPlaceholder.tsx \
  desktop/src/components/topbar/topbar.css \
  desktop/src/components/topbar/LayoutToggle.tsx \
  desktop/src/components/topbar/NewSessionButton.tsx \
  desktop/src/components/topbar/NewSessionButton.test.tsx \
  desktop/src/components/topbar/SessionPicker.tsx \
  desktop/src/components/topbar/SessionPicker.test.tsx \
  desktop/src/components/topbar/TopBar.tsx \
  desktop/src/components/AppShell.tsx \
  desktop/src/components/shell.css \
  desktop/src/store/selectors.ts \
  desktop/src/store/slices/uiSlice.ts
git status --short
```

**What you must see:** thirty-four paths, thirty-two of them `A` and the last two — `selectors.ts` and `uiSlice.ts` — `M`. `TopBar.tsx` is easy to drop from this list and expensive to drop: step 17 creates it and `AppShell.tsx` imports it, so a commit without it is a tree that does not compile. `src/test/fixtures.ts` is deliberately **not** here — task 15 owns the builders (shared-shape §3.1) and this task only reads them. Anything else staged belongs to another agent; unstage it rather than committing it.

The message goes to a file first. A multi-line heredoc can terminate early when the command crosses an extra `bash -c` boundary, and the symptom — `here-document delimited by end-of-file`, then the message body executing as shell commands — is worth one line to avoid.

```bash
printf '%s\n' \
'feat(cockpit): the shell, the rail, the sub-rail and the top bar' \
'' \
'First pixels. AppShell owns the three-column grid, the grain layer and the' \
'overlays that belong to no surface; the rail nests project, branch, worktree' \
'and session as four levels with every right-click dispatched through the one' \
'ContextMenu; the sub-rail renders all six lenses with none disabled and each' \
'routed to an honest placeholder until the task that builds it lands; the top' \
'bar carries the layout toggle, two controls disabled with the phase that makes' \
'them real, and a split New-session button whose left half creates with resolved' \
'defaults and opens no dialog.' \
'' \
'hooks/heartbeat.ts is the ONE clock. beatDelayStyle() aligns a CSS animation to' \
'a shared epoch with a negative delay, so ten dots that mounted at ten moments' \
'breathe together for zero JS per frame, and useHeartbeat() is one interval for' \
'the whole window behind useSyncExternalStore. Never a spinner per row.' \
'' \
'The overlay stack lives in hooks/useDismissable.ts and attaches its listeners' \
'only while something is open, so with no overlay Escape is not intercepted and' \
'reaches the terminal, which is what lets the agent CLI interrupt itself.' \
'closeTopOverlay() is what task 23 delegates its Escape ordering to.' \
'' \
'The departures are argued in the plan. A worktree keeps its chevron when only' \
'exited sessions remain, because a finished session stays listed and readable —' \
'the transcript is the artifact — and the literal sessionCount > 0 rule would' \
'strand it. The session picker gains "Save as branch default", because the rail' \
'gives that verb no other surface.' \
'' \
'Adds machinesOpen to uiSlice (⌘⇧M cannot reach a component useState) and four' \
'selectors to selectors.ts: selectBranchAttentionSessionId, so one scalar read' \
'gives a branch dot both its status and its navigation target, and by-id reads' \
'for a project, a branch and a worktree, so the context menu derives nothing' \
'inline (shared-shape rule 4).' \
'' \
'Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>' \
> /tmp/task-18-commit.txt
git commit -F /tmp/task-18-commit.txt
```

Verify the trailer survived, exactly:

```bash
git log -1 --pretty=%B | tail -2
```

**What you must see:** the last non-empty line is `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`, with nothing after it.

---

### What tasks 20 to 24 inherit, in one place

- **`hooks/heartbeat.ts` is the only clock.** `StatusDot` is the only caller of `beatDelayStyle()`; `SessionPaneFooter` and any "waiting" label are the only callers of `useHeartbeat()`. A rail row subscribing to `useHeartbeat` re-renders forty rows a second and is a defect, not a heartbeat.
- **Every live number uses `var(--numeric)` — `tabular-nums`.** Task 18 ships none, so this rule has no case in its own tree and `Rail.test.tsx` instead pins the opposite fact, that the rail prints no digits at all. **Task 20 ships the first ones** — elapsed, waiting — and they are exactly the values the rule exists for: proportional figures make a 1 Hz counter jitter on every tick, which reads cheap and drags the eye to a number that has not meaningfully changed. `tokens.css` declares `--numeric` and `app.css` applies it to `<body>`, so it is inherited and the usual case needs nothing; a number rendered inside an element that resets `font-variant-numeric` must set it back. Added by fix round 1, ruling B.
- **`components/common/*` is shared and finished.** `StatusDot` takes the four session statuses and the six machine states and nothing else — do not pass it a value outside `DotStatus`. `Popover` is closed when `anchor === null`; it has no `open` prop. `Toasts` is mounted once, in `AppShell`; do not mount a second stack.
- **Escape ordering is `closeTopOverlay()`.** Task 23's `useKeyboardMap` calls it and does not re-derive "is an overlay open". When it returns `false`, the key belongs to the terminal (§6.9).
- **A control that opens an overlay carries `data-overlay-trigger` *and* `aria-expanded`.** `useDismissable` listens for `pointerdown` in the capture phase on `document`, so it runs before the trigger's own `click`; without `data-overlay-trigger` it closes the overlay and the click re-opens it, and the control can never close what it opened. The trigger owns its toggle. `aria-expanded` is what scopes that exemption to the overlay this trigger actually owns — a trigger that reports `"false"` is treated as outside, so clicking it dismisses whatever is open before opening its own. Task 22's ⋯ button and task 23's palette launcher need both attributes; a trigger with the data attribute and a hard-coded or missing `aria-expanded` leaves the other overlays undismissable.
- **The six fixture builders are task 15's and there is no seventh.** `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession`, `aHarness`, all from `src/test/fixtures.ts`. A variant is an override object; a task that genuinely needs a new builder adds it to task 15 and says so, per shared-shape §3.2.
- **`useCreateSession` is the one create path**, used by the split button, the picker's Create, the rail's New-session verbs, the worktree row's `＋` and task 23's `⌘⏎`. It resolves, clamps, strips, creates, focuses, and turns `network_unavailable` and `backend_unavailable` into two different sentences. Do not call `api.createSession` directly.
- **Every reserved insertion point comes with an anchored guard.** Task 20 adds `return <DevelopLens />;` to `AppShell.tsx`'s Develop arm — an addition to §9's task-20 file list. Task 22 fills `sessionItems()` and the "Mark ready" entry in `ContextMenu.tsx`, and puts its optimistic pending affordance on `rail/SessionRow.tsx`, whose reserving comment carries the `data-pending=` guard. Task 23 adds `useKeyboardMap();` and `<CommandPalette />` to `AppShell.tsx`. Each comment carries the exact `grep -cE` that proves the code and not the comment is there; keep the guards in the consuming task's commit step.

