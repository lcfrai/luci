### Task 23: The command palette and the keyboard map

This is the task that makes the cockpit drivable without a mouse, and it is the task where contract §8's **known defect 10** is actually contained. Defect 10 is "merge reorder indexes the unfiltered array", and §8's middle column says where the same mistake would otherwise be made in Phase 2: *"the palette while a query is typed"*. A command palette holds a cursor over a list that the user is actively shrinking with every keystroke. Hold that cursor as an **array index** and the row under it changes meaning between the keypress that filters and the Enter that fires — the user reads "Kill orbit-two", presses Enter, and the app runs whatever now occupies position 2. There is no worse defect available in a command palette, because the feedback is a destructive action the user believes they chose. So the cursor here is an **id**, the list is looked up by id at the moment Enter fires, and `idAtVisiblePosition` from §3.9 is the only thing that ever converts a position into a command.

The second half is `hooks/useKeyboardMap.ts`, which §6.8's ownership table makes **the one `window` keydown listener in the application** — "No component adds a `window` keydown listener." It implements every row of §6.9's chord table — with one declared departure on `⌘⇧M`'s guard, argued in step 8 — and three of that table's rows are not intercepted at all. §6.9 spells the reason out for two of them, *"because both look like omissions"*; the third carries its reason in the table itself. **`shift+tab` is not intercepted**: Claude Code cycles its own permission mode on shift+tab inside its own TUI, so a cockpit that intercepted it, wrote `session.permissionMode` and told the agent nothing would put the two out of sync permanently — that is known defect 8 wearing a different hat, and §8 names this task as its fix. **`/` is not intercepted** — the row whose reason the table states in line: slash commands belong to the agent CLI and a palette that ate the key would make `/compact` untypeable. **`Escape` is the other entry §6.9 spells out** — "the app must therefore **not** intercept Escape while a terminal has focus" — and this hook honours it by omission: **it has no Escape branch at all**. Task 18 owns Escape in `hooks/useDismissable.ts`, as a capture-phase listener attached *only while an overlay is open*, so when nothing is open there is literally no listener and `\x1b` reaches xterm and the agent interrupts itself. Task 18's own hand-off note is explicit — "task 23 must call them and must not add its own Escape branch for overlays" — so this hook **reads** `hasOpenOverlay()` and **calls** `closeTopOverlay()`, and never re-derives either. A later reader who adds `if (e.key === "Escape")` here will break the agent's interrupt and the test named in step 7 is what stops them.

One chord is a foot-gun in a webview and is written defensively for it. `⌘W` closes the window in WebView2 unless something calls `preventDefault`, and §6.9 marks that mandatory. The guard "requires a focused session" must therefore run **after** `preventDefault`, not before it: a `⌘W` pressed with nothing focused must still swallow the key, or the honest no-op becomes "the app vanished". Step 7 tests exactly that ordering.

**What this task inherits that the UI contract predates.** `GET /v1/harnesses` returns `[]`, never `null` — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test — so nothing here writes a defensive `?? []` against a server array; every `?? []` that does appear is on a `Record` lookup with a possibly-absent **key**, which is a different thing. The error-code enum has **sixteen** members, not fifteen: Group C's ruling 7 added `non_loopback_publish`, emitted when the daemon reports a sandbox published off loopback, and that sandbox is *running* — no surface here switches over codes, but the palette must never describe such a session as stopped. `SessionStatus` is four values and only four (`starting`, `running`, `idle`, `exited`); `docs/AGENTIC-UX.md` §4 rule 8 lists seven and five of those need PTY parsing this phase forbids. A dropped WebSocket destroys nothing, so **exited sessions stay in the palette** — the transcript is the artifact and jumping to a finished session to read it is a legitimate command. And there is no bearer token on the desktop-to-control-plane hop before Phase 8; the boundary is loopback bind, `Host` guard and `Origin` allowlist, all server-side, so no keyboard path here carries or needs a credential.

**Group D shared-shape rules that bind this task**, section by section.

**§1 rules 1–3.** This task **modifies `components/AppShell.tsx`, which task 18 created**, at the two insertion points task 18 reserved by comment. Rule 2 says the line arrives in the same step that defines what it calls; this task satisfies it more strictly — `useKeyboardMap` and `CommandPalette` are both written and green *before* step 9 adds either line, and step 9 adds both in one edit. Rule 3 says the guard must be anchored, because an unanchored `grep -c "<CommandPalette"` counts task 18's reserving comment and reports a mount that does not exist; step 9 and the commit step use the anchored `grep -cE` forms task 18 printed.

**§2.** This task creates `palette/palette.css` on the `.pal-*` prefix, which contract §5.5 reserves for it. It writes nothing into `develop.css` and touches neither task 20's `.pane-*`/`.cols-*` nor task 21's `.cv-*`.

**§3 — the fixtures.** The typed builders are **task 15's**, filled into the point task 14 reserves, and they are named `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession` and `aHarness`. This task declares **no builder of its own and no local object literal standing in for one**: its three test files import `aProject`, `aBranch`, `aWorktree` and `aSession` from `../test/fixtures` and pass override objects. (Round one of this plan built its `Project` and `Branch` rows as inline literals; they matched contract §2 at the time, but a literal is a near-duplicate builder by another name and goes stale the day §2 gains a field.)

**§3A — every guard is anchored.** Step 1's greps anchor on `^export`, which no doc comment can satisfy. Step 9's anchor on `^[[:space:]]+` so a `//` or `{/* */}` line cannot match. Step 9's one unanchored guard, `grep -c 'TASK 23 INSERTION POINT' # expect 0`, was measured against task 18's real file rather than reasoned about: that literal appears in `AppShell.tsx` exactly twice, both inside the blocks step 9 replaces, and nowhere in its header — so 0 is the count after the edit. It is the guard §3A's own example (task 22's identical grep, which reads 1 because task 18's *menu* header says `TASK 22 INSERTION POINTS:`) warns about, and here it is clean.

**§3B — the CSS rules.** `palette.css` opens with `@import "../../styles/tokens.css";`, writes no colour function at all (the backdrop is `var(--scrim)`, the token task 14 added to `tokens.css` for exactly this rule rather than letting this file write a raw `oklch()`), ships no gradient, and has no `transition` property anywhere. Step 5 states each of these.

**§3C — no jest-dom.** None of the three test files uses a jest-dom matcher. Presence is `expect(...).not.toBeNull()` / `toBeDefined()`, absence is `toBeNull()`, and DOM state is read off properties.

**§4.** The one derived read this task needs that §3.7 does not name is added to **task 17's `selectors.ts`** as `selectTargetWorktreeId`, not computed inline, because `⌘⏎` and the palette's "New session…" row are the same read and two copies would drift.

**Files:**
- `desktop/src/components/palette/commands.ts` — `Command`, `buildCommands`, `filterCommands` (new)
- `desktop/src/components/palette/commands.test.ts` — block order, the `⌘1`–`⌘9` hints, and that filtering only removes (new)
- `desktop/src/components/palette/CommandPalette.tsx` — the `⌘K` surface; id-based cursor, focus capture and return (new)
- `desktop/src/components/palette/CommandPalette.test.tsx` — defect 10's regression test, verbatim from §8 (new)
- `desktop/src/components/palette/palette.css` — `.pal-*`, a prefix no other task owns (new)
- `desktop/src/hooks/useKeyboardMap.ts` — the one `window` keydown listener and §6.9's whole table (new)
- `desktop/src/hooks/useKeyboardMap.test.ts` — the deliberate non-interceptions, and `⌘W` not closing the window (new)
- `desktop/src/components/AppShell.tsx` — **task 18's file.** Adds `useKeyboardMap();` and `<CommandPalette />` at task 18's two reserved points (modify)
- `desktop/src/store/selectors.ts` — **task 17's file.** Adds `selectTargetWorktreeId`; an addition to §3.7 per shared-shape §4 (modify)

**Interfaces:**
- Produces:
  - `components/palette/commands` — `Command` (the §6.7 interface, declared here), `buildCommands(state: CockpitState): Command[]`, and `filterCommands(commands, query): Command[]`
  - `components/palette/CommandPalette` — `CommandPalette(): JSX.Element | null`
  - `hooks/useKeyboardMap` — `useKeyboardMap(): void`
  - `store/selectors` — `selectTargetWorktreeId(s: CockpitState): WorktreeId | null`
- Consumes:
  - task 15 — `lib/immutable.idAtVisiblePosition`; `store/types` for `CockpitState`'s member types; `test/fixtures.{aProject, aBranch, aWorktree, aSession}`, the shared-shape §3 builders task 15 fills into task 14's file
  - task 17 — `store/store.useCockpit`, `resetCockpit`, `CockpitState`; `store/selectors.selectPaneSessionIds`
  - task 18 — `hooks/useDismissable.{useDismissable, hasOpenOverlay, closeTopOverlay, __resetOverlays}`; `hooks/useCreateSession.useCreateSession`; `store/slices/uiSlice`'s `openMachines`; `components/AppShell`
  - task 22 — `components/develop/sessionVerbs.sessionVerbs(sessionId, deps): MenuItem[]`, from which `⌘W` runs the one item whose `id` is `"kill"`. **This is the only symbol this task imports from task 22**, it is used at exactly one call site, and step 1 verifies its name and arity before anything is written against it.
- Required by: task 24 — `defects.test.tsx` covers defects 8 and 10 **off the file system, not against a mounted shell**: its manifest asserts that `useKeyboardMap.test.ts` and `CommandPalette.test.tsx` exist and hold at least one `it(`, and its source anchors assert that `useKeyboardMap.ts` names neither `"Tab"` nor `permissionMode` once comments are stripped and that `CommandPalette.tsx` still calls `idAtVisiblePosition`. Nothing there reads a test *name*, which is why §8's wording is the only trail from either defect to its evidence and why steps 4 and 7 forbid rewording it. `designsystem.test.ts` reads `palette.css` as part of its whole-tree audit.
- **Corrects task 22's Required-by line**, which says "task 23's `buildCommands` lifts Kill, Restart and Resume into the palette by calling `sessionVerbs`". It does not, and should not: the palette lists **no session verbs at all**, because a palette row that killed a session would be a destructive act one keystroke from a filter change, which is defect 10's own failure shape. The only caller of `sessionVerbs` in this task is `useKeyboardMap`'s `⌘W`, which §6.9 requires. Nothing in task 22 breaks either way: `sessionVerbs` is a plain function over a store snapshot with no registered consumers.
- Amends contract §3.7: adds `selectTargetWorktreeId`. §6.9's `⌘⏎` guard ("requires a selected branch; falls back to the branch's first worktree") is a derived read and shared-shape §4 puts derived reads in `selectors.ts`.
- Amends contract §6.7: adds `filterCommands` beside `buildCommands`. §6.7 fixes the stable-ranking rule but names no function for it, and a filter written inline in the component could not be tested without rendering, which is the same reason `buildCommands` is a pure module.

- [ ] **Step 1: Check the ground, and pin down the one symbol task 22 owns**

Nothing is written in this step. It exists because this task sits on top of nine others and one of its imports — task 22's kill verb — is named by task 22 and not by the contract. Finding that out after three files are written costs a rewrite; finding it out now costs one command.

```bash
cd desktop
grep -cE "^export function useKeyboardMap" src/hooks/useKeyboardMap.ts   # expect: No such file
grep -cE "^export function CommandPalette" src/components/palette/CommandPalette.tsx  # expect: No such file
grep -cE "^export function useDismissable" src/hooks/useDismissable.ts   # expect 1
grep -cE "^export function hasOpenOverlay" src/hooks/useDismissable.ts   # expect 1
grep -cE "^export function closeTopOverlay" src/hooks/useDismissable.ts  # expect 1
grep -cE "^export function __resetOverlays" src/hooks/useDismissable.ts  # expect 1
grep -cE "^export function useCreateSession" src/hooks/useCreateSession.ts  # expect 1
grep -cE "^export function idAtVisiblePosition" src/lib/immutable.ts     # expect 1
grep -cE "^export const selectPaneSessionIds" src/store/selectors.ts     # expect 1
grep -cE "^export const selectTargetWorktreeId" src/store/selectors.ts   # expect 0 — step 3 adds it
grep -cE "^export function aProject" src/test/fixtures.ts               # expect 1
grep -cE "^export function aBranch" src/test/fixtures.ts                # expect 1
grep -cE "^export function aWorktree" src/test/fixtures.ts              # expect 1
grep -cE "^export function aSession" src/test/fixtures.ts               # expect 1
grep -nE "^export function sessionVerbs" src/components/develop/sessionVerbs.ts
grep -nE "^export interface VerbDeps" src/components/develop/sessionVerbs.ts
```

The first two are expected to print `grep: ...: No such file or directory` and exit 2 — that is the pre-condition, not a failure. A hit on either means another agent has already been in `palette/` or `hooks/`; stop and report it rather than overwriting.

The fixture greps are shared-shape §3: the builders are **task 15's**, filled into the point task 14 reserved, and there is one grep per builder this task imports — `aProject`, `aBranch`, `aWorktree` and `aSession`. A `0` from any of them means task 15 has not landed in this worktree — stop, because writing a local builder is the exact thing §3 forbids and it is what cost the first authoring round six Criticals.

The last two lines are the ones that matter. This task is written against **`sessionVerbs(sessionId: SessionId, deps: VerbDeps): MenuItem[]`**, from which it takes the single item whose `id` is `"kill"` — task 22's optimistic kill with the Undo toast, which §9's task-22 row places in `components/develop/sessionVerbs.ts`. Two properties of that item are load-bearing and both are task 22's, not this task's: `run` is `undefined` while the session has already exited, so `?.run?.()` is an honest no-op rather than a second gate written here; and `run` calls `deps.close()` first, which is why step 8 passes a no-op for it. If task 22 shipped the verb list under a different name or arity, **change these places and nothing else**: the import line in `src/hooks/useKeyboardMap.ts`, the single call site in the same file, and the `vi.mock("../components/develop/sessionVerbs", …)` factory in `src/hooks/useKeyboardMap.test.ts`. Do not reimplement the kill here — the Undo window, the optimistic store write and the `network_unavailable` / `backend_unavailable` sentences all live in task 22, and a second copy is a second behaviour.

`grep -cE "^export const selectTargetWorktreeId"` returning 1 means another Group D task added it first; step 3 must then be read against what is already there rather than appending a duplicate.

- [ ] **Step 2: Write the failing tests for `buildCommands`**

`commands.ts` is pure — no React, no DOM, no rendering — so its tests are the cheapest place to pin the rules §6.7 states and gives no function for: the **fixed block order** (verbs, then sessions, then worktrees, then branches, then projects) and **filtering that only removes rows**. Both are ranking rules, both are invisible in a screenshot, and both are the kind of thing a later "small tidy" deletes.

Each detail below is a decision, argued where it is made rather than in a footnote. **Sessions come from every worktree, not only the selected branch's**, because a palette that cannot reach the session you are not looking at is a filtered rail. **Their order is `selectPaneSessionIds` first, then everything else by worktree id**, because §3.7 calls `selectPaneSessionIds` "the one true ordering" and because `Object.keys` on `sessions` is insertion order — which changes when the poller replaces a worktree's sessions, and an order that changes under a poll is the reflow §6.7 bans. **A session command calls `selectBranch` → `selectWorktree` → `focusSession` in that order**, because §3.3 says `selectBranch` "clears worktree+session selection", so any other order throws the focus away.

Create `desktop/src/components/palette/commands.test.ts`:

```ts
import { beforeEach, describe, expect, it } from "vitest";
import { buildCommands, filterCommands } from "./commands";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";

// Shared-shape §3: the builders are task 15's and every override is an object passed to one
// of them. No literal stands in for a fixture here — a literal is a near-duplicate builder
// that goes stale the day contract §2 gains a field.
const PROJECT = aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" });
const BRANCH = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false });

function seed(sessionIds: string[]): void {
  resetCockpit();
  const sessions = Object.fromEntries(
    sessionIds.map((id, index) => [
      id,
      aSession({
        id,
        title: `orbit-${index + 1}`,
        projectId: "p-1",
        branchId: "b-1",
        worktreeId: "w-1",
        status: "running",
      }),
    ]),
  );
  useCockpit.setState({
    projects: [PROJECT],
    branchesByProject: { "p-1": [BRANCH] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "wt-evidence" })],
    },
    sessions,
    sessionIdsByWorktree: { "w-1": sessionIds },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    lens: "develop",
    layout: "columns",
  });
}

describe("buildCommands", () => {
  beforeEach(() => seed(["s-1", "s-2", "s-3"]));

  it("returns the fixed block order: verbs, sessions, worktrees, branches, projects", () => {
    const kinds = buildCommands(useCockpit.getState()).map((c) => c.id.split(":")[0]);
    const firstOf = (kind: string): number => kinds.indexOf(kind);
    expect(firstOf("verb")).toBe(0);
    expect(firstOf("verb")).toBeLessThan(firstOf("session"));
    expect(firstOf("session")).toBeLessThan(firstOf("worktree"));
    expect(firstOf("worktree")).toBeLessThan(firstOf("branch"));
    expect(firstOf("branch")).toBeLessThan(firstOf("project"));
  });

  it("names the layout it switches TO, not the one it is on", () => {
    const columns = buildCommands(useCockpit.getState()).find((c) => c.id === "verb:layout");
    expect(columns?.label).toBe("Switch to Canvas");
    useCockpit.setState({ layout: "canvas" });
    const canvas = buildCommands(useCockpit.getState()).find((c) => c.id === "verb:layout");
    expect(canvas?.label).toBe("Switch to Columns");
  });

  it("gives a session its project and worktree as meta", () => {
    const row = buildCommands(useCockpit.getState()).find((c) => c.id === "session:s-2");
    expect(row?.label).toBe("orbit-2");
    expect(row?.meta).toBe("engagement-portal / wt-evidence");
  });

  it("hints ⌘1–⌘9 for the first nine pane sessions and nothing for the tenth", () => {
    seed(Array.from({ length: 10 }, (_, i) => `s-${i + 1}`));
    const rows = buildCommands(useCockpit.getState());
    expect(rows.find((c) => c.id === "session:s-1")?.hint).toBe("⌘1");
    expect(rows.find((c) => c.id === "session:s-9")?.hint).toBe("⌘9");
    // The row must EXIST before its missing hint means anything: `undefined?.hint` is also
    // undefined, so without this line a tenth session dropped from the list entirely would
    // pass the assertion below and the ⌘1–9 rule would be untested at its only boundary.
    expect(rows.find((c) => c.id === "session:s-10")).toBeDefined();
    expect(rows.find((c) => c.id === "session:s-10")?.hint).toBeUndefined();
  });

  it("running a session command selects its branch, then its worktree, then focuses it", () => {
    buildCommands(useCockpit.getState()).find((c) => c.id === "session:s-3")?.run();
    const after = useCockpit.getState().selection;
    expect(after.branchId).toBe("b-1");
    expect(after.worktreeId).toBe("w-1");
    expect(after.sessionId).toBe("s-3");
  });

  it("offers no layout verb when no branch is selected", () => {
    useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } });
    const ids = buildCommands(useCockpit.getState()).map((c) => c.id);
    expect(ids).not.toContain("verb:layout");
    expect(ids).not.toContain("verb:new-session");
    expect(ids).toContain("verb:machines");
  });
});

describe("filterCommands", () => {
  beforeEach(() => seed(["s-1", "s-2", "s-3"]));

  it("only removes rows; the survivors keep their original relative order", () => {
    const all = buildCommands(useCockpit.getState());
    const kept = filterCommands(all, "orbit").map((c) => c.id);
    const expected = all.map((c) => c.id).filter((id) => kept.includes(id));
    expect(kept).toEqual(expected);
    expect(kept).toEqual(["session:s-1", "session:s-2", "session:s-3"]);
  });

  it("matches the label case-insensitively and ignores meta", () => {
    const all = buildCommands(useCockpit.getState());
    expect(filterCommands(all, "ORBIT-2").map((c) => c.id)).toEqual(["session:s-2"]);
    // "engagement-portal" is the project NAME, so it matches the project row by label and no
    // session row by meta. §6.7 ties matching to `label`; widening it to `meta` would make one
    // project name match every session under it, which reads as a rank change to the user.
    expect(filterCommands(all, "engagement-portal").map((c) => c.id)).toEqual(["project:p-1"]);
  });

  it("returns every command for an empty or whitespace query", () => {
    const all = buildCommands(useCockpit.getState());
    expect(filterCommands(all, "")).toHaveLength(all.length);
    expect(filterCommands(all, "   ")).toHaveLength(all.length);
  });
});
```

Run it:

```bash
cd desktop
npm test -- src/components/palette/commands.test.ts
```

**What you must see:** the suite cannot even load, because `./commands` does not exist. Vitest reports that as a transform failure rather than a test failure, and the two lines to match are:

```
 FAIL  src/components/palette/commands.test.ts [ src/components/palette/commands.test.ts ]
Error: Failed to resolve import "./commands" from "src/components/palette/commands.test.ts". Does the file exist?
```

followed by `Test Files  1 failed (1)` in the summary. The code frame Vite prints under that line carries absolute paths that differ per machine and per Vite version, so it is not reproduced here — match on the two lines above. **If instead you see `Tests  9 failed (9)`, a `commands.ts` already exists** and another agent has been in this directory; stop and report it rather than overwriting.

- [ ] **Step 3: Add `selectTargetWorktreeId` to task 17's `selectors.ts`, then write `commands.ts`**

First the selector, because `commands.ts` imports it. Shared-shape §4: the store is task 17's, nobody else declares a selector inline, and a later task that needs a derived read **adds it to `selectors.ts`** with a doc comment naming the surface. §6.9's `⌘⏎` guard and the palette's "New session…" row are the same derived read, so there is exactly one of it.

Append to `desktop/src/store/selectors.ts`, below the last existing export, changing nothing above. The file already imports `CockpitState` and `WorktreeId` — task 17 writes one `import type { … } from "./types"` line that carries `WorktreeId`, and a separate `import type { CockpitState } from "./store"` — so **add no import at all**. Check before you assume it: `grep -n "^import type" src/store/selectors.ts`. If some later edit has dropped `WorktreeId` from that list, add it *to that list*; a second `import type` line from the same module is a duplicate-identifier error, not a second import.

```ts
/** The worktree a new session should land on: the explicitly selected one, else the selected
 *  branch's FIRST worktree in server order, else null. Contract §6.9 states that fallback as
 *  the ⌘⏎ guard, and the palette's "New session…" row needs the same answer — two inline
 *  copies would drift the day one of them starts preferring the primary worktree.
 *  Addition to contract §3.7 by task 23, per Group D shared-shape §4. */
export const selectTargetWorktreeId = (s: CockpitState): WorktreeId | null => {
  const { branchId, worktreeId } = s.selection;
  if (worktreeId !== null) return worktreeId;
  if (branchId === null) return null;
  const worktrees = s.worktreesByBranch[branchId] ?? [];
  return worktrees.length === 0 ? null : worktrees[0].id;
};
```

The `?? []` there is on a `Record` lookup whose **key** may be absent, which is a different thing from the `?? []` this plan's header bans: server array *fields* such as `harness.models` are always present because task 8's fix round made the Go side allocate unconditionally.

Now create `desktop/src/components/palette/commands.ts`:

```ts
// Contract §6.7. The command set as DATA: pure over a store snapshot, no React, no DOM, no
// import from a component — which is what lets commands.test.ts run without rendering.
//
// Ranking is fixed HERE and filtering only removes rows. docs/DESIGN-SYSTEM.md: "Stable
// ranking; rows never reflow while filtering."
import { selectPaneSessionIds, selectTargetWorktreeId } from "../../store/selectors";
import type { CockpitState } from "../../store/store";
import type { SessionId } from "../../store/types";

export interface Command {
  id: string;
  /** Rendered verbatim; matched case-insensitively against the query. */
  label: string;
  /** Right-aligned context, e.g. "engagement-portal / evidence-upload". */
  meta?: string;
  glyph: string;
  /** Shown inline so the palette teaches its own shortcuts. */
  hint?: string;
  run(): void;
}

/** Kind markers, not status markers. Status is colour PLUS shape and belongs to StatusDot
 *  (§7.5); a palette row that drew a status glyph would be a second vocabulary for the same
 *  fact. Three of the five — ▸ U+25B8, ▥ U+25A5, ▣ U+25A3 — are Geometric Shapes, the block
 *  SubRail's LENSES glyphs are drawn from. The other two are not, and are here anyway for
 *  named reasons: ⌘ U+2318 (Miscellaneous Technical) is the one glyph the user already reads
 *  as "a command", and ⑂ U+2442 (Optical Character Recognition) is the fork mark SubRail
 *  already uses for Merge, so a second branch glyph would be a second vocabulary. None is a
 *  webfont glyph — all five resolve from the system stack — and the two outside Geometric
 *  Shapes are both already on screen elsewhere in the shell, so neither is a new font risk. */
const GLYPH = {
  verb: "⌘",      // ⌘
  session: "▸",   // ▸
  worktree: "▥",  // ▥
  branch: "⑂",    // ⑂ — the glyph SubRail already uses for Merge
  project: "▣",   // ▣
} as const;

/** Every known session id, deterministically. selectPaneSessionIds first — §3.7 calls it "the
 *  one true ordering" and it is what ⌘1–9 indexes — then every other worktree's sessions, by
 *  worktree id. Object.keys(sessions) is NOT used: it is insertion order, and replaceSessions
 *  rewrites it on a poll, so the palette's order would change under the user's cursor. */
function allSessionIdsInStableOrder(state: CockpitState): SessionId[] {
  const ordered = [...selectPaneSessionIds(state)];
  const seen = new Set(ordered);
  for (const worktreeId of Object.keys(state.sessionIdsByWorktree).sort()) {
    for (const id of state.sessionIdsByWorktree[worktreeId] ?? []) {
      if (!seen.has(id)) {
        seen.add(id);
        ordered.push(id);
      }
    }
  }
  return ordered;
}

/** The command set for one snapshot, in §6.7's fixed block order: verbs, then sessions, then
 *  worktrees, then branches, then projects.
 *
 *  Pure over the snapshot, including its ACTIONS — zustand actions are created once and keep
 *  their identity for the life of the store, so a `run` closure taken here is still correct
 *  after any number of later writes. That is what lets CommandPalette build this list once,
 *  when it opens, instead of rebuilding it on every 2 s poll. */
export function buildCommands(state: CockpitState): Command[] {
  const commands: Command[] = [];
  const paneIds = selectPaneSessionIds(state);
  const hintFor = (id: SessionId): string | undefined => {
    const position = paneIds.indexOf(id) + 1;
    return position >= 1 && position <= 9 ? `⌘${position}` : undefined;
  };
  const projectName = (id: string): string => state.projects.find((p) => p.id === id)?.name ?? id;

  /* ── verbs ──────────────────────────────────────────────────────────────────────────── */
  if (state.lens === "develop" && state.selection.branchId !== null) {
    const next = state.layout === "columns" ? "canvas" : "columns";
    commands.push({
      id: "verb:layout",
      label: next === "canvas" ? "Switch to Canvas" : "Switch to Columns",
      glyph: GLYPH.verb,
      run: () => state.setLayout(next),
    });
  }
  commands.push({
    id: "verb:rail",
    label: state.railCollapsed ? "Show the rail" : "Hide the rail",
    glyph: GLYPH.verb,
    hint: "⌘B",
    run: () => state.toggleRail(),
  });
  commands.push({
    id: "verb:machines",
    label: "Machines",
    glyph: GLYPH.verb,
    hint: "⌘⇧M",
    run: () => state.openMachines(),
  });
  if (selectTargetWorktreeId(state) !== null) {
    commands.push({
      id: "verb:new-session",
      // The picker, not a create: ⌘⏎ is the no-dialog path (§6.4) and duplicating it here
      // would give the same words two different outcomes.
      label: "New session…",
      glyph: GLYPH.verb,
      run: () => state.openPicker(),
    });
  }

  /* ── sessions ───────────────────────────────────────────────────────────────────────── */
  for (const id of allSessionIdsInStableOrder(state)) {
    const session = state.sessions[id];
    if (session === undefined) continue;
    const worktree = (state.worktreesByBranch[session.branchId] ?? []).find(
      (w) => w.id === session.worktreeId,
    );
    commands.push({
      id: `session:${session.id}`,
      label: session.title,
      meta: `${projectName(session.projectId)} / ${worktree?.name ?? session.worktreeId}`,
      glyph: GLYPH.session,
      hint: hintFor(session.id),
      // selectBranch CLEARS the worktree and session selection (§3.3), so this order is
      // load-bearing: branch, then worktree, then focus. Any other order discards the focus
      // it just set. Exited sessions are listed and reachable on purpose — a dropped socket
      // destroys nothing and the transcript is the artifact.
      run: () => {
        state.selectBranch(session.projectId, session.branchId);
        state.selectWorktree(session.worktreeId);
        state.focusSession(session.id);
      },
    });
  }

  /* ── worktrees ──────────────────────────────────────────────────────────────────────── */
  for (const branchId of Object.keys(state.worktreesByBranch).sort()) {
    for (const worktree of state.worktreesByBranch[branchId] ?? []) {
      commands.push({
        id: `worktree:${worktree.id}`,
        label: worktree.name,
        meta: projectName(worktree.projectId),
        glyph: GLYPH.worktree,
        run: () => {
          state.selectBranch(worktree.projectId, worktree.branchId);
          state.selectWorktree(worktree.id);
        },
      });
    }
  }

  /* ── branches ───────────────────────────────────────────────────────────────────────── */
  for (const projectId of Object.keys(state.branchesByProject).sort()) {
    for (const branch of state.branchesByProject[projectId] ?? []) {
      commands.push({
        id: `branch:${branch.id}`,
        label: branch.name,
        meta: projectName(branch.projectId),
        glyph: GLYPH.branch,
        run: () => state.selectBranch(branch.projectId, branch.id),
      });
    }
  }

  /* ── projects ───────────────────────────────────────────────────────────────────────── */
  for (const project of state.projects) {
    commands.push({
      id: `project:${project.id}`,
      label: project.name,
      meta: project.path,
      glyph: GLYPH.project,
      run: () => state.selectProject(project.id),
    });
  }

  return commands;
}

/** §6.7: filtering only REMOVES rows; it never re-ranks. Matching is case-insensitive over
 *  `label` alone, which is what §6.7's comment on that field says — widening it to `meta`
 *  would make one project name match every session beneath it, and a result set that grows
 *  by a factor of ten on one keystroke reads as a rank change even though the order is
 *  untouched. Addition to §6.7: the contract fixes the rule and names no function for it. */
export function filterCommands(commands: readonly Command[], query: string): Command[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") return [...commands];
  return commands.filter((command) => command.label.toLowerCase().includes(needle));
}
```

Run it:

```bash
cd desktop
npm test -- src/components/palette/commands.test.ts
```

**What you must see:** nine passing tests in one file. The vitest default reporter prints a `✓` line per file and then the summary block; the counts that matter are `Test Files  1 passed (1)` and `Tests  9 passed (9)`.

The likeliest first failure is *"returns the fixed block order"* with `firstOf("session")` at `-1`, which means `allSessionIdsInStableOrder` returned nothing: `selectPaneSessionIds` reads worktrees of the **selected branch**, so the fixture's `selection.branchId` and `worktreesByBranch` key must agree. The second likeliest is *"hints ⌘1–⌘9"* comparing `⌘1` against `⌘1` and failing with two strings that look identical in the diff. That is the literal being **U+2318 PLACE OF INTEREST SIGN** in one file and a look-alike in the other — U+2318 is written in both `commands.ts` and `commands.test.ts` and must be the same code point in both, so copy it rather than retyping it. When the two look the same and still differ, measure instead of staring:

```bash
node --eval 'console.log([..."⌘1"].map((c) => c.codePointAt(0).toString(16)).join(" "))'
```

which prints `2318 31`. Any other first value is the look-alike.

- [ ] **Step 4: Write the failing tests for `CommandPalette`**

The first test in this file is named **verbatim** from contract §8's defect-10 row — *"Enter on the second visible row runs the second VISIBLE command, not commands[1]"* — so do not reword it. Nothing greps the name: task 24's `defects.test.tsx` checks only that the file §8 names exists and holds tests. §8's row is the authority, and a rename breaks the trail from the defect to its evidence silently. It is built so that an index-based cursor passes nothing: the query filters away every verb, so the unfiltered `commands[1]` (the rail toggle) and the second **visible** row (a session) have completely different, independently observable effects. A palette that indexed would collapse the rail; this one focuses a session and leaves the rail alone, and the test asserts both.

Create `desktop/src/components/palette/CommandPalette.test.tsx`:

```tsx
import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CommandPalette } from "./CommandPalette";
import { __resetOverlays } from "../../hooks/useDismissable";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";

const TITLES = ["orbit-one", "orbit-two", "orbit-three"];

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  useCockpit.setState({
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false })] },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "wt-evidence" })],
    },
    sessions: Object.fromEntries(
      TITLES.map((title, index) => [
        `s-${index + 1}`,
        aSession({
          id: `s-${index + 1}`,
          title,
          projectId: "p-1",
          branchId: "b-1",
          worktreeId: "w-1",
          status: "running",
        }),
      ]),
    ),
    sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    lens: "develop",
    layout: "columns",
  });
});

function open(): void {
  act(() => {
    useCockpit.getState().openPalette();
  });
}

function selectedRowName(): string {
  const on = screen.getAllByRole("option").find((row) => row.getAttribute("aria-selected") === "true");
  return on?.textContent ?? "<none>";
}

describe("CommandPalette", () => {
  it("renders nothing while the palette is closed", () => {
    render(<CommandPalette />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("Enter on the second visible row runs the second VISIBLE command, not commands[1]", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    // Unfiltered, commands[0] is "Switch to Canvas" and commands[1] is "Hide the rail" — the
    // row an index-held cursor would fire. The query removes both.
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    expect(useCockpit.getState().railCollapsed).toBe(false);
    expect(useCockpit.getState().layout).toBe("columns");
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("keeps the selected row selected while a narrower query still matches it", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-two");
    await user.type(screen.getByRole("combobox"), "-t");
    expect(screen.getAllByRole("option")).toHaveLength(2);
    expect(selectedRowName()).toContain("orbit-two");
  });

  it("falls back to the first visible row when the selected id is filtered out", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-two");
    await user.type(screen.getByRole("combobox"), "-three");
    expect(screen.getAllByRole("option")).toHaveLength(1);
    expect(selectedRowName()).toContain("orbit-three");
  });

  it("wraps the cursor at both ends of the visible list", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit");
    await user.keyboard("{ArrowUp}");
    expect(selectedRowName()).toContain("orbit-three");
    await user.keyboard("{ArrowDown}");
    expect(selectedRowName()).toContain("orbit-one");
  });

  it("runs the row a click lands on", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "orbit-three");
    await user.click(screen.getByRole("option", { name: /orbit-three/ }));
    expect(useCockpit.getState().selection.sessionId).toBe("s-3");
  });

  it("names the query instead of rendering an empty box", async () => {
    const user = userEvent.setup();
    render(<CommandPalette />);
    open();
    await user.type(screen.getByRole("combobox"), "zzzzz");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByText(/Nothing matches/)).toBeDefined();
  });

  it("returns DOM focus to the element that had it when the palette closes", () => {
    render(
      <>
        <button type="button">prior</button>
        <CommandPalette />
      </>,
    );
    const prior = screen.getByRole("button", { name: "prior" });
    prior.focus();
    open();
    expect(document.activeElement).toBe(screen.getByRole("combobox"));
    act(() => {
      useCockpit.getState().closePalette();
    });
    expect(document.activeElement).toBe(prior);
  });
});
```

Run it:

```bash
cd desktop
npm test -- src/components/palette/CommandPalette.test.tsx
```

**What you must see:** the same load failure shape as step 2, naming this file's missing import:

```
 FAIL  src/components/palette/CommandPalette.test.tsx [ src/components/palette/CommandPalette.test.tsx ]
Error: Failed to resolve import "./CommandPalette" from "src/components/palette/CommandPalette.test.tsx". Does the file exist?
```

**One test in this file makes a claim about task 18's code rather than about ours**, and it is flagged here so its failure is read correctly. *"returns DOM focus to the element that had it"* assumes `useDismissable` restores focus on **unmount** (the palette body unmounts when `paletteOpen` flips false), not only on an `open: true → false` transition of its own argument. Task 18's doc comment says "on close DOM focus returns to whatever had it before" and unmount is a close, so this should hold. If it does not, the fix is one layout effect in `PaletteBody` — capture `document.activeElement` on mount and restore it in the cleanup — and it must be written as a *fallback for that hook*, with a comment saying so, because §6.8 allows exactly one owner of overlay focus and silently adding a second is how a cursor starts jumping.

- [ ] **Step 5: Write `palette.css`**

Written before the component, because the component imports it and Vite fails the whole transform on a missing stylesheet rather than warning. `.pal-*` is a prefix no other Group D task owns — shared-shape §2 assigns `.pane-*`/`.cols-*` to task 20 and `.cv-*` to task 21, and this file touches neither.

Task 24's `designsystem.test.ts` is **eleven** assertions over the whole `src` tree, not three, and this file is written to pass every one of them that reads a stylesheet. The rules that bind it, each a separate assertion there:

1. **The token layer is imported first** — the file's first line after comments must match `@import "../../styles/tokens.css";` exactly. Task 24 strips comments and tests the first non-empty line, so the header below sits *under* the import, not above it. Contract §5.5 requires it and shared-shape §3B repeats it.
2. **No colour function outside the token layer** — no `oklch()`, `rgb()`, `hsl()`, `lab()`, `lch()`, `oklab()` or `color-mix()`. The backdrop is the one place this file needs a colour of its own, and it is `var(--scrim)`: task 14 added that token to `tokens.css` under shared-shape §3B precisely so this stylesheet does not write `oklch(0% 0 0 / 0.42)` at the call site. It is an amendment to contract §5.3, argued in task 14's step 7, and it is the twentieth `PHASE_2_NAMES` entry.
3. **No literal hex**, and nothing in the indigo/violet arc — every colour here is a `var()` from §5.3.
4. **No `transition: all`, and no transition that names no property** — there is no `transition` property in this file at all.
5. **Every `box-shadow` built from a token or `currentColor`** — every one here is a `var(--)`.
6. **No gradient anywhere** — no `linear-`, `radial-` or `conic-gradient(`.

The one piece of motion is a keyframe on `opacity` and `transform` only, per §7.5, and it earns its place under `docs/DESIGN-SYSTEM.md`'s "motion must explain causality or be deleted": the palette rises the last six pixels into position, which is what says *this arrived over the app and will leave again*, as against a modal that blinks into existence and reads as a repaint bug. It is disabled under `prefers-reduced-motion` both by §5.3's global floor and by the explicit rule at the foot of this file.

Radius is `--r-window`, not `--r-card`: §5.3 rule 4 ties radius to element size and reserves 14px for "the palette, modals — anything that reads as its own window". Elevation is `--e-overlay`, the ring-plus-layered-shadow form, never a flat border.

Create `desktop/src/components/palette/palette.css`:

```css
@import "../../styles/tokens.css";

/* palette.css — the ⌘K surface. Contract §6.7; tokens from §5.3.
 *
 * The @import is the FIRST non-comment line, which is what contract §5.5 requires and what
 * task 24 asserts — it strips comments and reads the first non-empty line that remains.
 *
 * Owns .pal-* and nothing else. No literal hex, no colour function (the backdrop is
 * var(--scrim), the token task 14 added for it), no gradient, no `transition` property at
 * all, every shadow from a token: task 24's designsystem.test.ts asserts every one of those.
 *
 * The keyboard cursor and the pointer hover are DELIBERATELY different marks. The cursor is
 * the accent wash plus its ring (the "current" vocabulary, §5.3 rule 2); hover is a neutral
 * surface step. A mouse crossing the list must never look like it moved the keyboard cursor,
 * because it does not move it. */

.pal-scrim {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 14vh;
  /* --scrim, not a raw oklch(): shared-shape §3B routes a colour a component needs into
     tokens.css, and task 14 added this one for this rule. Task 24 fails any colour function
     outside the token layer, and this was the line that failed it. */
  background: var(--scrim);
}

.pal {
  display: flex;
  flex-direction: column;
  width: min(640px, calc(100vw - 48px));
  max-height: 60vh;
  overflow: hidden;
  background: var(--surface);
  border-radius: var(--r-window);
  box-shadow: var(--e-overlay);
  animation: pal-in var(--dur) var(--ease) both;
}

@keyframes pal-in {
  from {
    opacity: 0;
    transform: translateY(-6px) scale(0.985);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

/* A field is a WELL, not a raised plane (§5.3). The hairline under it is the only structure
 * between the query and its results; a full border would fight the window's own ring. */
.pal-input {
  flex: 0 0 auto;
  padding: 14px 16px;
  font: inherit;
  font-size: 15px;
  color: var(--ink);
  background: var(--field);
  border: 0;
  box-shadow: 0 1px 0 0 var(--line);
  outline-offset: -3px;
}

.pal-input::placeholder {
  color: var(--ink-3);
}

.pal-list {
  margin: 0;
  padding: 6px;
  overflow-y: auto;
  list-style: none;
}

.pal-row {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--r-ctl);
  color: var(--ink);
  cursor: default;
}

.pal-row:hover {
  background: var(--hover);
}

.pal-row-on {
  background: var(--accent-tint);
  box-shadow: 0 0 0 1px var(--accent-ring);
}

.pal-glyph {
  color: var(--ink-3);
  text-align: center;
}

.pal-label {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.pal-meta {
  font-size: 12px;
  color: var(--ink-3);
}

.pal-hint {
  padding: 2px 6px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-2);
  background: var(--inset);
  border-radius: var(--r-chip);
  box-shadow: var(--e-hair);
}

.pal-empty {
  margin: 0;
  padding: 18px 16px;
  color: var(--ink-2);
}

@media (prefers-reduced-motion: reduce) {
  .pal {
    animation: none;
  }
}
```

- [ ] **Step 6: Write `CommandPalette.tsx`**

The structural decisions below are load-bearing and none of them is stated by the contract, so each is argued here.

**The list is built once, when the palette opens.** `CommandPalette` reads one boolean and renders `PaletteBody` or nothing; every hook lives in `PaletteBody`, so mounting *is* the open event and `useMemo(…, [])` is honest rather than a lie about dependencies. The alternative — rebuilding on every store write — re-ranks the list under the user's cursor twice a second as the poller lands, which is exactly the reflow §6.7 bans and §3.7 warns about. It is safe because zustand actions keep their identity for the life of the store, so a `run` closure captured at open is still correct when Enter fires. The cost is honest and small: a session created while the palette is open is not in the list until the next open.

**`useDismissable` is called before the input is focused, and that ordering is the feature.** Its layout effect captures whatever had DOM focus — normally the xterm helper textarea — and our layout effect then moves focus to the input. Reverse them and the palette "returns" focus to itself. It also owns Escape and outside-click for this overlay, which is why there is no Escape branch in this component or in `useKeyboardMap` (§6.9).

**The cursor is `selectedId`, and the rendered selection is `activeId`.** They are different on purpose: `selectedId` is what the user last chose, `activeId` is that id *if it is still visible*, else the first visible row. That one expression is the whole of defect 10's fix in this surface — a cursor kept as a number cannot express "the row I chose is gone".

Create `desktop/src/components/palette/CommandPalette.tsx`:

```tsx
// Contract §6.7 (the ⌘K surface) and §6.8 (it owns DOM focus WHILE OPEN and returns it on
// close, and owns no other focus and no scroll).
//
// Selection is by ID, never by array index — §6.7 and known defect 10. Every position that
// enters this file is converted back to an id through idAtVisiblePosition (§3.9) before it is
// used for anything.
import { useLayoutEffect, useMemo, useRef, useState } from "react";
import type { JSX, KeyboardEvent as ReactKeyboardEvent } from "react";
import { buildCommands, filterCommands } from "./commands";
import { useDismissable } from "../../hooks/useDismissable";
import { idAtVisiblePosition } from "../../lib/immutable";
import { useCockpit } from "../../store/store";
import "./palette.css";

export function CommandPalette(): JSX.Element | null {
  const open = useCockpit((s) => s.paletteOpen);
  // Every hook lives in PaletteBody, so a mount IS the open event and the command list can be
  // memoised with no dependencies. Putting them here would need [open] in every dep array and
  // would keep a stale list alive between opens.
  return open ? <PaletteBody /> : null;
}

function PaletteBody(): JSX.Element {
  const closePalette = useCockpit((s) => s.closePalette);

  // Called FIRST: its layout effect records the previously focused element before ours steals
  // focus for the input (§6.8). It also owns Escape and outside-click for this overlay, which
  // is why neither appears below and neither appears in useKeyboardMap (§6.9).
  const shellRef = useDismissable<HTMLDivElement>(true, closePalette);
  const inputRef = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    inputRef.current?.focus();
  }, []);

  const commands = useMemo(() => buildCommands(useCockpit.getState()), []);
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const visible = filterCommands(commands, query);
  const visibleIds = visible.map((command) => command.id);
  // Defect 10, contained: the cursor follows the ID and only falls back to the first row when
  // that id is no longer visible. An index here would silently re-point at a different command
  // on every keystroke that shortens the list.
  const activeId =
    selectedId !== null && visibleIds.includes(selectedId) ? selectedId : (visibleIds[0] ?? null);

  function move(delta: number): void {
    if (visibleIds.length === 0) return;
    // 1-based, and 0 when the cursor is not in the visible list at all.
    const position = activeId === null ? 0 : visibleIds.indexOf(activeId) + 1;
    let next = position + delta;
    if (next < 1) next = visibleIds.length;
    if (next > visibleIds.length) next = 1;
    setSelectedId(idAtVisiblePosition(visibleIds, next));
  }

  function runId(id: string | null): void {
    if (id === null) return;
    // Looked up in the VISIBLE list, by id, at the moment Enter fires. Not commands[index].
    const command = visible.find((candidate) => candidate.id === id);
    if (command === undefined) return;
    // Close first: the overlay is gone before the command's writes land, so the user never
    // sees a palette rendered over an app that has already changed underneath it.
    closePalette();
    command.run();
  }

  function onKeyDown(event: ReactKeyboardEvent<HTMLInputElement>): void {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      move(1);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      move(-1);
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      runId(activeId);
    }
    // Escape is deliberately absent. useDismissable owns it for every overlay (§6.9).
  }

  return (
    <div className="pal-scrim" role="presentation">
      <div className="pal" role="dialog" aria-modal="true" aria-label="Command palette" ref={shellRef}>
        <input
          ref={inputRef}
          className="pal-input"
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls="pal-list"
          aria-activedescendant={activeId === null ? undefined : `pal-row-${activeId}`}
          placeholder="Search commands, sessions, worktrees, branches and projects"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={onKeyDown}
        />
        {visible.length === 0 ? (
          <p className="pal-empty">
            Nothing matches <b>{query}</b>.
          </p>
        ) : (
          <ul className="pal-list" id="pal-list" role="listbox" aria-label="Commands">
            {visible.map((command) => (
              <li
                key={command.id}
                id={`pal-row-${command.id}`}
                role="option"
                aria-selected={command.id === activeId}
                className={command.id === activeId ? "pal-row pal-row-on" : "pal-row"}
                // Keeps DOM focus in the input: a pointerdown that blurred it would fire
                // useDismissable's focus return mid-click.
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => runId(command.id)}
              >
                <span className="pal-glyph" aria-hidden="true">
                  {command.glyph}
                </span>
                <span className="pal-label">{command.label}</span>
                <span className="pal-meta">{command.meta ?? ""}</span>
                <kbd className="pal-hint" hidden={command.hint === undefined}>
                  {command.hint ?? ""}
                </kbd>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
```

`meta` and `hint` render as an always-present cell rather than a conditional one, because the grid has four fixed tracks and a row that drops a cell reflows its neighbours' alignment — the list would ripple as the query changes which rows carry a hint. The empty `<kbd>` is `hidden`, so it is absent from the accessibility tree and from `textContent` while still holding its track.

Run it:

```bash
cd desktop
npm test -- src/components/palette/CommandPalette.test.tsx
```

**What you must see:** eight passing tests. The most likely first failure is *"Enter on the second visible row…"* reporting `expected 's-2' to be null`, which means Enter reached the form default instead of `onKeyDown` — check that `onKeyDown` is on the `<input>` and not on the `<div className="pal">`. The second most likely is *"keeps the selected row selected"* failing with `orbit-one`, which means `activeId` was computed from an index rather than from `selectedId`.

- [ ] **Step 7: Write the failing tests for `useKeyboardMap`, starting with the harness**

Two of these test names are quoted **verbatim** from contract §8 — defect 8's row names *"shift+tab is not intercepted and session.permissionMode is unchanged"* and *"Escape is not intercepted while a terminal has focus and no overlay is open"*. Do not reword them. Nothing greps the names — task 24's `defects.test.tsx` checks only that this file exists and holds tests — so §8's rows are the only authority and a rename breaks the trail from the defect to its evidence silently.

The whole file turns on one detail of jsdom: `new KeyboardEvent("keydown", { key })` is **not cancelable by default**, and `preventDefault()` on a non-cancelable event is a silent no-op that leaves `defaultPrevented` false. Get that wrong and every "is not intercepted" assertion here passes without testing anything, while the `⌘W` assertion fails and looks like a bug in the hook. The helper sets `cancelable: true` once so no test can forget it.

The hook subscribes to nothing — it reads `useCockpit.getState()` inside the handler — so store writes in these tests need no `act()` wrapper and produce no act warning. That is a property of the hook, not a shortcut: a keyboard map that re-subscribed would re-register its listener on every poll.

One test needs an overlay to be genuinely open, and the only honest way to get one is to mount a `useDismissable` — the stack in task 18's module is private and `__resetOverlays()` is the only other door into it. That is why `useDismissable` is imported below alongside `hasOpenOverlay`, and why the test asserts `hasOpenOverlay()` is `true` before it presses anything: a test that silently had no overlay open would pass for the wrong reason, which is the same trap `cancelable: true` closes.

Create `desktop/src/hooks/useKeyboardMap.test.ts`, first its harness:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useKeyboardMap } from "./useKeyboardMap";
import { __resetOverlays, hasOpenOverlay, useDismissable } from "./useDismissable";
import { resetCockpit, useCockpit } from "../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";

// Task 22 owns the kill verb and task 18 owns the create path; this file proves the CHORDS
// reach them, not what they do. If step 1 found a different name or arity for the verb list,
// this factory and the import in useKeyboardMap.ts are the only two places that change.
//
// The fake returns task 22's SHAPE — a MenuItem[] with one `kill` entry — rather than a bare
// function, because the shape is the seam: the hook finds the item by id and calls `run`, and
// a fake that skipped the list would let a hook that called something else pass.
const { killMock, createSessionMock } = vi.hoisted(() => ({
  killMock: vi.fn(),
  createSessionMock: vi.fn(),
}));
vi.mock("../components/develop/sessionVerbs", () => ({
  sessionVerbs: (sessionId: string) => [
    { id: "resume", label: "Resume", run: undefined },
    { id: "kill", label: "Kill", run: () => killMock(sessionId) },
  ],
}));
vi.mock("./useCreateSession", () => ({ useCreateSession: () => createSessionMock }));

/** cancelable:true is MANDATORY. Without it preventDefault() is a no-op, defaultPrevented
 *  stays false, and every "is not intercepted" assertion below passes for the wrong reason. */
function press(key: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  window.dispatchEvent(event);
  return event;
}

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  killMock.mockReset().mockResolvedValue(undefined);
  createSessionMock.mockReset().mockResolvedValue(undefined);
  useCockpit.setState({
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" })] },
    sessions: Object.fromEntries(
      ["s-1", "s-2", "s-3"].map((id) => [
        id,
        aSession({ id, projectId: "p-1", branchId: "b-1", worktreeId: "w-1", permissionMode: "ask" }),
      ]),
    ),
    sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
  });
  renderHook(() => useKeyboardMap());
});

afterEach(() => {
  document.querySelectorAll("textarea.xterm-helper-textarea").forEach((node) => node.remove());
});
```

and then its tests, appended to the same file:

```ts
describe("useKeyboardMap — the deliberate non-interceptions", () => {
  it("shift+tab is not intercepted and session.permissionMode is unchanged", () => {
    const event = press("Tab", { shiftKey: true });
    expect(event.defaultPrevented).toBe(false);
    expect(useCockpit.getState().sessions["s-1"].permissionMode).toBe("ask");
  });

  it("Escape is not intercepted while a terminal has focus and no overlay is open", () => {
    const term = document.createElement("textarea");
    term.className = "xterm-helper-textarea";
    document.body.appendChild(term);
    term.focus();
    expect(hasOpenOverlay()).toBe(false);
    const event = press("Escape");
    expect(event.defaultPrevented).toBe(false);
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("/ is not intercepted, because slash commands belong to the agent CLI", () => {
    expect(press("/").defaultPrevented).toBe(false);
  });
});

describe("useKeyboardMap — the chord table", () => {
  it("⌘W preventDefaults even with nothing focused, so the webview never closes", () => {
    expect(press("w", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(killMock).not.toHaveBeenCalled();
  });

  it("⌘W kills the focused session", () => {
    useCockpit.getState().focusSession("s-2");
    expect(press("w", { metaKey: true }).defaultPrevented).toBe(true);
    expect(killMock).toHaveBeenCalledWith("s-2");
  });

  it("⌘K toggles the palette, including while a terminal has focus", () => {
    expect(press("k", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(useCockpit.getState().paletteOpen).toBe(true);
    press("k", { ctrlKey: true });
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("⌘1 focuses the first pane session and ⌘9 past the end is a no-op", () => {
    press("1", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    press("9", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
  });

  it("⌘\\ moves to the next pane session and wraps at the end", () => {
    useCockpit.getState().focusSession("s-3");
    press("\\", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
  });

  it("⌘B toggles the rail and ⌘⇧M opens the machines popover", () => {
    press("b", { ctrlKey: true });
    expect(useCockpit.getState().railCollapsed).toBe(true);
    press("M", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().machinesOpen).toBe(true);
  });

  // §6.9 marks ⌘B "Always", so it survives an open overlay: the rail is not behind the
  // palette's scrim, it is beside it, and a user who opened the palette to find a session
  // can still widen the rail to look for one. ⌘⇧M is the declared departure — see the hook.
  it("⌘B still toggles the rail while an overlay is open, and ⌘⇧M does not", () => {
    // The overlay stack is module-private; the only way to push onto it is to mount a
    // useDismissable. The explicit type argument is required — T appears in no parameter,
    // so there is nothing to infer it from. setup.ts's cleanup() unmounts it after the test
    // and __resetOverlays() in beforeEach backstops that.
    renderHook(() => useDismissable<HTMLDivElement>(true, vi.fn()));
    expect(hasOpenOverlay()).toBe(true);
    press("b", { ctrlKey: true });
    expect(useCockpit.getState().railCollapsed).toBe(true);
    press("M", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().machinesOpen).toBe(false);
  });

  it("⌘⏎ creates a session on the selected branch's first worktree", () => {
    press("Enter", { ctrlKey: true });
    expect(createSessionMock).toHaveBeenCalledWith("w-1");
  });
});
```

Run it:

```bash
cd desktop
npm test -- src/hooks/useKeyboardMap.test.ts
```

**What you must see:** the load failure, naming the hook that does not exist yet:

```
 FAIL  src/hooks/useKeyboardMap.test.ts [ src/hooks/useKeyboardMap.test.ts ]
Error: Failed to resolve import "./useKeyboardMap" from "src/hooks/useKeyboardMap.test.ts". Does the file exist?
```

If instead it fails on `Failed to resolve import "../components/develop/sessionVerbs"`, task 22 has not landed in this worktree and step 1 was skipped. Go back and run step 1 — writing the hook against a guessed export name is how the Group C attach-route defect happened.

- [ ] **Step 8: Write `useKeyboardMap.ts`**

Read the first guard before writing the rest, because it is what makes the non-interceptions structural rather than a promise. **The handler returns immediately unless `metaKey || ctrlKey` is held.** Escape, `shift+tab`, `/` and every printable character therefore never reach a branch at all — there is no code path that could grow an Escape case by accident, and no "is the user typing in a field?" guard is needed, because every chord §6.9 asks this hook to intercept carries a modifier. `⌘` is `metaKey || ctrlKey` because the app runs on Windows, where the physical key is Ctrl (§6.9).

Then the orderings that are not cosmetic. **`⌘W` calls `preventDefault()` before it checks anything** — before the overlay check and before the focused-session check — because WebView2 closes the window on the default action, and a `⌘W` pressed with nothing focused must still swallow the key or the honest no-op becomes "the app vanished". **`⌘K` and `⌘B` are handled before the overlay bail-out**, because §6.9's Guard column marks both of them "Always": `⌘K` "Always, including while a terminal has focus", `⌘B` "Always" flat. `⌘K` is the one chord that must work *on top of* an open overlay, and `⌘B` acts on the rail, which is beside the palette rather than behind it and is still on screen and still legible while one is open. Every other chord belongs to the surface the overlay covers and is dropped while one is open — creating a session under an open picker, or moving focus between panes the user cannot see, are both worse than doing nothing.

**One declared departure from §6.9.** The table marks `⌘⇧M` "Always" and this hook drops it while an overlay is open. `openMachines()` opens a **second overlay**, and `.pal-scrim` is `z-index: 60` over the whole viewport, so obeying the table literally would put the machines popover *underneath* the palette's backdrop: a surface the user opened, cannot see and cannot click, with Escape closing the palette on top of it rather than the popover they asked for. That is the same failure task 22's `SessionMenu` anchor chain exists to avoid. `⌘K` and `⌘B` are the "Always" chords that can be honoured literally and they are; this one is not, and the test added in step 7 pins the departure so it cannot be quietly reverted into a stacking bug. The alternative considered and rejected was closing the top overlay first, the way `⌘K` does — rejected because `⌘⇧M` from inside the picker would then destroy a half-filled form to show a machine list.

**And one thing the overlay stack cannot do, which the `⌘K` path used to assume.** Task 18's `closeTopOverlay()` calls `top.close()` and returns `true`; it does **not** pop the stack. The pop happens in `useDismissable`'s effect cleanup, when React unmounts the overlay — which is after this handler returns. So no synchronous loop can drain the stack: `hasOpenOverlay()` stays `true` and `closeTopOverlay()` keeps returning `true` for the same entry however many times it is called. `⌘K` therefore calls it **once**, and the comment below says why, because a bounded retry loop here reads like defensive code and is in fact repeated `close()` calls on the same overlay.

Create `desktop/src/hooks/useKeyboardMap.ts`:

```ts
// Contract §6.9 (the chord table) and §6.8 ("the keydown listener" row): this is the ONE
// window keydown listener in the application, mounted once by AppShell. No component adds
// another.
//
// THREE KEYS ARE DELIBERATELY NOT HANDLED HERE. Do not "fix" them:
//   • shift+tab — Claude Code cycles its own agent-side permission state on it, inside its
//     own TUI. A cockpit that intercepted it, wrote that field in the store and told the
//     agent nothing would desynchronise the two permanently. That is known defect 8 (§8).
//     This file names neither the key nor the field on purpose: task 24's defect suite
//     strips comments and asserts that NEITHER string appears in what is left, which is a
//     cheap mechanical proof that no branch here can be reading or writing either one.
//   • /        — slash commands belong to the agent CLI. Eating the key makes /compact
//     untypeable.
//   • Escape   — task 18's useDismissable owns it, as a capture-phase listener attached only
//     while an overlay is open. With nothing open there is no listener, xterm sends \x1b down
//     the PTY and the agent interrupts itself (§6.9). This hook READS hasOpenOverlay() and
//     CALLS closeTopOverlay(); it never re-derives either.
import { useEffect } from "react";
import { sessionVerbs } from "../components/develop/sessionVerbs";
import { useCreateSession } from "./useCreateSession";
import { closeTopOverlay, hasOpenOverlay } from "./useDismissable";
import { idAtVisiblePosition } from "../lib/immutable";
import { selectPaneSessionIds, selectTargetWorktreeId } from "../store/selectors";
import { useCockpit } from "../store/store";

/** Two of task 22's three VerbDeps are unreachable from a keyboard chord: there is no host
 *  menu to close and no rename input to arm. They are no-ops rather than throws, because the
 *  one verb this file runs (Kill) calls deps.close() before it commits anything. */
const NOOP = (): void => {};

export function useKeyboardMap(): void {
  const createSession = useCreateSession();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent): void {
      // ⌘ is metaKey || ctrlKey — the app runs on Windows, where the physical key is Ctrl.
      // Everything without it leaves here, which is where the three non-interceptions live.
      if (!(event.metaKey || event.ctrlKey)) return;
      // AltGr on several European layouts reports ctrlKey AND altKey; hijacking it would eat
      // composed characters the user is typing into the agent.
      if (event.altKey) return;

      // Shifted letters arrive as "M", not "m", so the table is matched in lower case.
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      const state = useCockpit.getState();

      // ⌘W FIRST, and preventDefault BEFORE every guard: without it WebView2 closes the
      // window, and "no session is focused" must still swallow the key (§6.9).
      if (key === "w" && !event.shiftKey) {
        event.preventDefault();
        if (hasOpenOverlay()) return;
        const focused = state.selection.sessionId;
        if (focused === null) return;
        // Optimistic kill plus the Undo window: task 22's, not reimplemented here. The verb
        // list is a plain function over a store snapshot, so it is built here in the handler
        // rather than at render. `run` is undefined on a session that has already exited —
        // task 22 gates Kill on `alive` — so `?.run?.()` is an honest no-op and NOT a second
        // gate written here, which would be a second answer to the same question.
        sessionVerbs(focused, { createSession, close: NOOP, beginRename: NOOP })
          .find((verb) => verb.id === "kill")
          ?.run?.();
        return;
      }

      // ⌘K works from anywhere, including a focused terminal and an open overlay. The palette
      // is a peer of the picker and the menus, never a child, so the topmost one closes.
      if (key === "k" && !event.shiftKey) {
        event.preventDefault();
        if (state.paletteOpen) {
          state.closePalette();
          return;
        }
        // ONCE, not in a loop. closeTopOverlay() calls close() and returns true; the stack is
        // popped by useDismissable's effect cleanup, i.e. after React unmounts the overlay,
        // i.e. after this handler returns. A loop would therefore call close() repeatedly on
        // the same entry and never reach a second one — it would read as draining and drain
        // nothing. A second overlay under this one stays open and its own Escape still works.
        closeTopOverlay();
        state.openPalette();
        return;
      }

      // §6.9 marks ⌘B "Always", and it can be honoured literally: the rail is beside the
      // palette, not behind it, so toggling it while an overlay is open changes something the
      // user can see. This branch is therefore ABOVE the overlay bail-out.
      if (key === "b" && !event.shiftKey) {
        event.preventDefault();
        state.toggleRail();
        return;
      }

      // Every remaining chord acts on the surface an overlay COVERS, so an open overlay takes
      // them: the palette owns its own arrows and Enter, and the picker owns its own fields.
      if (hasOpenOverlay()) return;

      // DECLARED DEPARTURE from §6.9, which marks ⌘⇧M "Always". It is dropped while an
      // overlay is open because openMachines() opens a SECOND overlay and .pal-scrim is
      // z-index 60 over the whole viewport: obeying the table literally would put the
      // machines popover under the palette's backdrop, where the user cannot see or click
      // it and Escape closes the wrong surface. Step 7's test pins this.
      if (key === "m" && event.shiftKey) {
        event.preventDefault();
        state.openMachines();
        return;
      }
      if (event.shiftKey) return;

      if (key === "Enter") {
        event.preventDefault();
        // "Requires a selected branch; falls back to the branch's first worktree" (§6.9) is
        // one selector, shared with the palette's New-session row.
        const worktreeId = selectTargetWorktreeId(state);
        if (worktreeId === null) return;
        void createSession(worktreeId);
        return;
      }

      if (key === "\\") {
        event.preventDefault();
        const ids = selectPaneSessionIds(state);
        if (ids.length === 0) return;
        const focused = state.selection.sessionId;
        // 1-based, and 0 when the focused session is not in the pane list — both land on the
        // first session, which is the only sensible "next" from outside the list.
        const position = focused === null ? 0 : ids.indexOf(focused) + 1;
        const next = idAtVisiblePosition(ids, position >= ids.length ? 1 : position + 1);
        if (next !== null) state.focusSession(next);
        return;
      }

      if (/^[1-9]$/.test(key)) {
        event.preventDefault();
        // §3.9's rule: never index an array across a filter boundary, map the position to an
        // id. Past the end this returns null and the chord is a no-op, per §6.9.
        const id = idAtVisiblePosition(selectPaneSessionIds(state), Number(key));
        if (id !== null) state.focusSession(id);
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [createSession]);
}
```

The listener is on `window` in the bubble phase, not capture, so an overlay's capture-phase Escape handler and an input's own key handling both run first. Under `React.StrictMode` the effect mounts, unmounts and mounts again (§1.7), which adds, removes and re-adds exactly one listener — safe here only because `createSession` is a stable identity: task 18 wraps it in `useCallback(…, [])`. If it ever becomes a fresh function per render, this effect re-registers on every store write; that is the one thing to check if a chord starts firing twice. `sessionVerbs` is not in the dependency list and must not be — it is a module import, not a value this component holds.

Run it:

```bash
cd desktop
npm test -- src/hooks/useKeyboardMap.test.ts
```

**What you must see:** eleven passing tests across the file's two `describe` blocks — three non-interceptions and eight chords. If *"⌘⇧M opens the machines popover"* fails with `expected undefined to be true`, `machinesOpen` is missing from `uiSlice` — that is task 18's step 11 addition, not something to add here. If *"⌘B still toggles the rail while an overlay is open"* fails at `expect(hasOpenOverlay()).toBe(true)`, `useDismissable` did not push: check the first argument is the literal `true` and not a state value that has not flipped yet.

- [ ] **Step 9: Fill task 18's two reserved insertion points in `AppShell.tsx`**

This is the step Group D's shared shape was written for. Task 18 created `components/AppShell.tsx` and reserved two points in it by comment, each naming this task and carrying the anchored `grep -cE` that proves the code and not the comment is present. Group C's worst defect was a consuming task assuming a reserved line had already been added; the guard below is what makes that impossible to repeat.

**Shared-shape rule 2 says the consuming task adds its line in the same step that defines what the line calls.** This task satisfies it more strictly than the rule requires: `useKeyboardMap` and `CommandPalette` were both written and green in steps 6 and 8, before either line exists, so `AppShell.tsx` compiles at every point in this task's history. Both lines go in **one** edit, so there is no intermediate state with a mount and no listener.

Add two imports to `desktop/src/components/AppShell.tsx`, below its existing `Toasts` import and above the `import type { Lens }` line:

```tsx
import { CommandPalette } from "./palette/CommandPalette";
import { useKeyboardMap } from "../hooks/useKeyboardMap";
```

Then replace the first reserved block — everything from `// ── TASK 23 INSERTION POINT ──` down to its closing rule line, inside `AppShell` and below the three `useCockpit` reads — with:

```tsx
  // §6.8: the ONE window keydown listener in the app, mounted once, here. Escape is NOT in
  // it — hooks/useDismissable owns the overlay stack and therefore owns Escape (§6.9).
  useKeyboardMap();
```

And replace the second reserved block — the `{/* ── TASK 23 INSERTION POINT ── … */}` comment after `<Toasts />` — with:

```tsx
      <CommandPalette />
```

The reserving comments are **removed, not kept beside the code**. A reservation that has been filled is stale documentation, and the next reader would have to work out whether the comment or the line is authoritative. What survives is the guard, which is a check rather than a claim.

`<CommandPalette />` goes last in the shell, after `<ContextMenu />` and `<Toasts />`, so it paints above them. `.pal-scrim` is `z-index: 60`, below `.grain::after`'s 9999 — deliberate: the grain is a global film over the whole window and `pointer-events: none`, so it tints the palette exactly as it tints everything else.

Now the guards. Run them from `desktop/`, and read the numbers rather than the exit codes:

```bash
cd desktop
grep -cE '^[[:space:]]+useKeyboardMap\(\);' src/components/AppShell.tsx  # expect 1
grep -cE '^[[:space:]]+<CommandPalette' src/components/AppShell.tsx      # expect 1
grep -cE '^[[:space:]]+<Toasts />' src/components/AppShell.tsx           # expect 1
grep -cE '^[[:space:]]+<ContextMenu />' src/components/AppShell.tsx      # expect 1
grep -c 'TASK 23 INSERTION POINT' src/components/AppShell.tsx            # expect 0
```

The anchor is the point. Task 18 measured the failure on the Go side: an unanchored `grep -c "<CommandPalette"` counts the reserving comment and reports `1` for a mount that does not exist. `^[[:space:]]+` requires the first non-space character to begin the code, so no `//` or `{/* */}` line can satisfy it. The last line is this task's own addition — it proves the comments were replaced rather than duplicated, which an anchored grep alone cannot tell you.

- [ ] **Step 10: Run the whole suite and the typecheck**

```bash
cd desktop
npm test
npm run typecheck
```

**What you must see from `npm test`:** every test file in the repository passing, tasks 14 to 22's included. This task adds **twenty-eight tests across three files** — nine in `palette/commands.test.ts`, eight in `palette/CommandPalette.test.tsx` and eleven in `hooks/useKeyboardMap.test.ts`. The summary's totals depend on every earlier task's suites, so the number to check is `0 failed` and the presence of these three files in the passed list, not a specific total.

A failure that appears only in the full run and not in the per-file runs of steps 3, 6 and 8 is almost certainly module-level state: `hooks/useDismissable`'s overlay stack survives vitest's `restoreMocks`/`clearMocks` (§1.6), which is why `__resetOverlays()` is in two of this task's `beforeEach` blocks. If a palette test fails with the palette already open, an earlier file left `paletteOpen` true — `resetCockpit()` is missing from that file's `beforeEach`, not from this one's.

**What you must see from `npm run typecheck`:** npm's own banner, nothing after it, and exit 0. `tsc --noEmit` (§1.2) prints nothing at all on success, but `npm run` always prints four lines of its own, and mistaking them for output is how a green typecheck gets read as a failure:

```

> desktop@0.1.0 typecheck
> tsc --noEmit

```

— a blank line, those two, and a trailing blank. Anything between the second `>` line and the end is a real diagnostic. Its format depends on whether stdout is a terminal: attached to one, tsc prints its pretty form, `src/hooks/useKeyboardMap.ts:NN:CC - error TS2345: …` followed by the source line and a caret; piped or redirected, it prints the flat form, `src/hooks/useKeyboardMap.ts(NN,CC): error TS2345: …`. The coordinates are shown as `NN`/`CC` because they depend on how this plan's block was transcribed into the file; **match on the code and the message, not on the coordinates or the layout.**

Two diagnostics are likely enough to name. `error TS2345: Argument of type 'string | null' is not assignable to parameter of type 'SessionId'` on a `focusSession` call means an `idAtVisiblePosition` result was used without its null check — that check is the defect-10 guard and must not be replaced with `!`. `error TS6133: 'closeTopOverlay' is declared but its value is never read` means the `⌘K` branch's single `closeTopOverlay()` call was deleted; `noUnusedLocals` is on and it is telling you that the rule step 8 argues — the palette is a peer of the other overlays and never a child of one, which is what lets §6.9 mark `⌘K` "Always" — went with it.

- [ ] **Step 11: Commit**

Stage by name. Shared-shape rule 6 forbids `git add -A`, `-u` and `.`, because other Group D agents are working in this tree and a wildcard commits their half-finished files.

```bash
cd desktop/..
git add \
  desktop/src/components/palette/commands.ts \
  desktop/src/components/palette/commands.test.ts \
  desktop/src/components/palette/CommandPalette.tsx \
  desktop/src/components/palette/CommandPalette.test.tsx \
  desktop/src/components/palette/palette.css \
  desktop/src/hooks/useKeyboardMap.ts \
  desktop/src/hooks/useKeyboardMap.test.ts \
  desktop/src/components/AppShell.tsx \
  desktop/src/store/selectors.ts
git status --short
```

**What you must see:** nine paths — seven `A`, and `AppShell.tsx` and `selectors.ts` as `M`. Anything else staged belongs to another agent; unstage it rather than committing it.

The message goes to a file first. A multi-line heredoc can terminate early when the command crosses an extra `bash -c` boundary, and the symptom — `here-document delimited by end-of-file`, then the message body executing as shell commands — is worth one line to avoid.

```bash
printf '%s\n' \
'feat(cockpit): the command palette and the keyboard map' \
'' \
'buildCommands is pure over a store snapshot and returns one fixed order —' \
'verbs, sessions, worktrees, branches, projects — and filterCommands only ever' \
'removes rows from it. The palette holds its cursor as an ID, resolves positions' \
'through idAtVisiblePosition and looks the command up in the VISIBLE list at the' \
'moment Enter fires, so a list shrinking under the cursor can never run a' \
'different command than the one on screen. That is known defect 10, contained in' \
'the Phase 2 surface where it would otherwise be made.' \
'' \
'useKeyboardMap is the one window keydown listener in the app. It returns before' \
'any branch unless Cmd/Ctrl is held, which is what makes the deliberate' \
'non-interceptions structural: shift+tab stays with Claude Code, whose own TUI' \
'cycles permission mode and which the cockpit must never contradict (defect 8);' \
'/ stays with the agent CLI; and Escape stays with the overlay stack in' \
'useDismissable, so with nothing open xterm sends \x1b and the agent interrupts' \
'itself. Cmd-W preventDefaults before every guard, because WebView2 closes the' \
'window on the default action and a no-op must still swallow the key.' \
'' \
'Of the chords 6.9 marks Always, Cmd-K and Cmd-B are honoured literally over an' \
'open overlay: Cmd-K closes the topmost one first, and Cmd-B moves a rail the' \
'overlay does not cover. Cmd-Shift-M is a declared departure: it opens a second' \
'overlay, and the palette scrim would render it invisible.' \
'' \
'Adds selectTargetWorktreeId to selectors.ts: the Cmd-Enter fallback to the' \
'branch first worktree and the palette New-session row are one derived read.' \
'Fills both insertion points task 18 reserved in AppShell.tsx, with the anchored' \
'greps that prove the code and not the comment is there.' \
'' \
'Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>' \
> /tmp/task-23-commit.txt
git commit -F /tmp/task-23-commit.txt
```

---

**What this task hands task 24.**

- **Every regression-test name contract §8 states exists here, verbatim.** `CommandPalette.test.tsx` carries *"Enter on the second visible row runs the second VISIBLE command, not commands[1]"* (defect 10) and `useKeyboardMap.test.ts` carries *"shift+tab is not intercepted and session.permissionMode is unchanged"* and *"Escape is not intercepted while a terminal has focus and no overlay is open"* (defect 8). `defects.test.tsx` checks only that the files exist and hold tests, so nothing mechanical catches a rename — §8's rows are the authority.
- **`palette.css` is clean for `designsystem.test.ts`**: `@import "../../styles/tokens.css";` is its first line, there is no colour function in it at all (the backdrop is `var(--scrim)`, task 14's token), no literal hex, no hue in the indigo/violet arc, no gradient, every `box-shadow` from a token, and no `transition` property. Its one animation is `opacity` and `transform` only and is switched off under `prefers-reduced-motion`.
- **`useKeyboardMap.ts` is clean for `defects.test.tsx`'s defect-8 anchors**, which strip comments and assert that neither the key name nor the store field appears in the code that remains. The hook's own doc comment names the defect and cites §8 without writing either string anyway; the two regression tests that *do* name them live in `useKeyboardMap.test.ts`, which the anchors do not read.
- **One declared departure from §6.9, and no other.** `⌘K` and `⌘B` fire while an overlay is open, as the table's "Always" requires. `⌘⇧M` does not, because it would open a popover under the palette's scrim; step 8 argues it and step 7 pins it.
- **The palette snapshots its command list when it opens.** A session created while it is open is not in the list until the next open. That is deliberate (§6.7's stable ranking) and is the one behaviour a live-wiring task might mistake for a bug.
- **Two chords depend on other tasks' code, not on this one.** `⌘⏎` goes through task 18's `useCreateSession` and `⌘W` through task 22's kill verb, so a failure in either is a failure there; this task's tests mock both and assert only that the chord arrives.
- **Decisions taken where the contract is silent**, each with its reason: the palette offers no lens-navigation rows (the sub-rail is already on screen whenever they would be legal, and importing `LENSES` from a `.tsx` component would drag React and a stylesheet into a pure module); matching is over `label` only, per §6.7's comment on that field; the arrow cursor wraps at both ends, matching `⌘\`; pointer hover does not move the keyboard cursor; and `⌘K` closes the topmost open overlay rather than stacking the palette on top of it — **once**, because task 18's `closeTopOverlay()` does not pop the stack and no synchronous loop can drain it.

