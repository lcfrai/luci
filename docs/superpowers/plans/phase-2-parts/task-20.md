### Task 20: Columns layout and the session pane

This is the first task in Group D's second half, and it is the one that puts a session on screen. Tasks 14 to 19 produced a substrate with nothing visible in it: a token layer, a domain model, a typed client, a store, a shell with a rail that can select a branch, and a terminal binding that keeps a PTY alive across a detach. **Nothing renders a session.** This task builds the surface those six exist for — `DevelopLens` picks a layout, `ColumnsLayout` lays panes out along a horizontal scroll, and `SessionPane` is one session: a header, a live `<Terminal/>`, and a footer. It also builds the zero state, because a branch with no worktrees is the state a new user is actually in and `docs/DESIGN-SYSTEM.md`'s floor says every state is designed.

**Two things here are owned nowhere else and must not be re-implemented later.** The first is **focus intent**: contract §6.8's ownership table gives *which session is focused* to `uiSlice.focusSession`, *DOM focus inside a pane* to `Terminal.tsx`, and *horizontal scroll of Columns* to `ColumnsLayout` — three owners, and `SessionPane` is the component that translates a pointer landing anywhere in the pane into the first of them plus a handover to the second. Two components moving DOM focus is the defect that produces a cursor that jumps mid-keystroke, so this file is the only one that does it. The second is **known defect 4**: a `POST /v1/sessions` that lands outside the viewport with no focus. `ColumnsLayout` watches its own id list, and when an id it has not seen before appears it calls `focusSession` on it and scrolls the matching pane into view with `{ inline: "center", block: "nearest" }`. Its first pass seeds the seen-set without focusing anything, or opening the app with nine sessions would yank the viewport to a pane the user did not ask for.

**The footer is where this task is most likely to be got wrong, and the failure is a product failure rather than a test failure.** `prototype/app.html` puts `ctxdots`, `cost` and `acct` in the pane chrome, and every one of those is synthetic in the prototype and underivable in Phase 2: a PTY is a byte stream, it carries no token accounting, no spend and no context window. `docs/AGENTIC-UX.md` §2 property 7 is absolute — if you cannot name the fact that justifies the element, the element must not exist — so the footer is `claude · opus · 41m · attached` and nothing more, per contract §6.5. Capacity arrives in Phase 3 with the runtime signal that makes it real. A reviewer should be able to point at each of those four and name the field it came from: `harness.command`, `session.model`, `session.createdAt` through `formatElapsed`, and the attach registry's own `AttachState.phase`.

**This task creates two files that task 21 then modifies**, which is the exact shape that produced Group C's worst defect, so the Group D shared-shape document's rules 1 to 3 apply directly and are honoured in steps 4 and 13. `DevelopLens.tsx` carries a reserving comment at the Canvas branch naming task 21 and saying what it will put there, and that comment deliberately contains the string `<CanvasLayout` **mid-line** so task 21's pre-commit guard `grep -cE '^[[:space:]]*<CanvasLayout' …` reads 0 before its change and 1 after, instead of the 2 a naive grep would read. `develop.css` is split by prefix: this task owns `.pane-*` and `.cols-*`, task 21 owns `.cv-*` and appends under a header at the foot of the file without reflowing a rule above it. Shared-shape rule 2 binds task 21, not this one, but the reservation only pays off if it is written here.

The facts below postdate the UI contract and override it wherever they touch this task. **`GET /v1/harnesses` returns `[]`, never `null`** — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test, so `harness.models` and `harness.efforts` are arrays and no file in this task writes a defensive `?? []`. **The error-code enum has sixteen members**, `non_loopback_publish` being the sixteenth from Group C's ruling 7; this task switches on no code directly, but `startSessionOnWorktree` routes anything that is not `backend_unavailable` or `network_unavailable` into a toast carrying the server's own sentence, which is exhaustive by construction. **`SessionStatus` is four values and only four**, so the pane's status vocabulary is `starting · running · idle · exited` and nothing renders `need` or `drift`. **A dropped WebSocket destroys nothing**: an `exited` pane keeps its scrollback, stays listed and stays readable, because the transcript is the artifact — which is why the exited bar carries `exitReason` and a Restart action rather than unmounting the pane.

**Files:**
- `desktop/src/components/develop/develop.css` — `.pane-*` and `.cols-*` in full, plus the reserved `.cv-*` header for task 21 (new)
- `desktop/src/components/develop/SessionPaneFooter.tsx` — the four honest facts, and `attachLabel` (new)
- `desktop/src/components/develop/SessionPaneHeader.tsx` — status dot, worktree name, harness name, the `⋯` button and its anchor attribute (new)
- `desktop/src/components/develop/SessionPane.tsx` — one session, focus intent, the exited bar, and `startSessionOnWorktree` (new)
- `desktop/src/components/develop/ColumnsLayout.tsx` — the horizontal scroll container and defect 4 (new)
- `desktop/src/components/develop/EmptyDevelop.tsx` — the zero state and its two named actions (new)
- `desktop/src/components/develop/DevelopLens.tsx` — the layout chooser, with task 21's reserved branch (new)
- `desktop/src/components/develop/SessionPane.test.tsx` — defect 3, focus intent, the exited bar, the honest footer (new)
- `desktop/src/components/develop/ColumnsLayout.test.tsx` — defect 4, pane order, and the `DevelopLens`/`EmptyDevelop` cases (new)
- `desktop/src/store/selectors.ts` — **modified**: one selector added to task 17's file, per shared-shape §4 (see Interfaces)

**Interfaces:**
- Produces:
  - `components/develop/DevelopLens.DevelopLens` — `{}`, per contract §6.5. Chooses Columns vs Canvas vs `EmptyDevelop` and imports `develop.css`; owns nothing else
  - `components/develop/ColumnsLayout.ColumnsLayout` — `{ sessionIds: SessionId[] }`. The horizontal scroll container, and **the only caller of `scrollIntoView` in Group D** (§6.8)
  - `components/develop/SessionPane.SessionPane` — `{ sessionId: SessionId; variant: "column" | "tile" }`. Task 21 renders it with `variant="tile"`; task 22 wires its `⋯` anchor
  - `components/develop/SessionPane.startSessionOnWorktree(worktreeId, defaults) → Promise<void>` — one create path shared by the exited bar's Restart and `EmptyDevelop`'s two actions. **Addition to contract §6.5**, justified in step 7
  - `components/develop/SessionPaneHeader.SessionPaneHeader` and `SessionPaneFooter.SessionPaneFooter` — exactly the §6.5 props
  - `components/develop/SessionPaneHeader.statusLabel(session: Session) → string` — the dot's accessible name, exported so a test names the sentence rather than matching prose; `exited` carries its `exitReason`
  - `components/develop/SessionPaneFooter.attachLabel(state: AttachState) → string` — the six-phase word list, exported so a test names phases rather than matching prose
  - `components/develop/EmptyDevelop.EmptyDevelop` — `{ branch: Branch | undefined }`
  - `components/develop/develop.css` — `.pane-*` and `.cols-*`; `.cv-*` reserved for task 21 under a header at the foot of the file
- Consumes: task 15's `Session`, `SessionId`, `Worktree`, `Branch`, `Harness`, `SessionDefaults`, `SessionStatus` (`store/types.ts`) and `formatElapsed` (`lib/time.ts`); task 16's `api` singleton (`api.createSession`) and `ApiError` as a type only; task 17's `useCockpit`, `selectSession`, `selectHarness`, `selectWorktrees`, `selectPaneSessionIds`, `selectFocusedSessionId`, `selectResolvedDefaults`, `toCreateRequest`, and the `focusSession` / `upsertSession` / `pushToast` / `openSessionMenu` actions; task 18's `StatusDot` (`components/common/StatusDot.tsx`), `useHeartbeat` (`hooks/heartbeat.ts`) and `selectWorktreeById` (task 18's own addition to `store/selectors.ts`); task 19's `Terminal` (`components/Terminal.tsx`) and `useAttachState` (`api/attachRegistry.ts`); task 15's `aSession`, `aWorktree`, `aBranch`, `aProject`, `aHarness` fixtures — shared-shape §3 puts all six builders in task 15, filling the point task 14 reserves in `test/fixtures.ts` — used with override objects and **not** extended.
- Required by: task 21 (`CanvasLayout` renders `SessionPane variant="tile"`, appends `.cv-*` to `develop.css`, and replaces the reserved branch in `DevelopLens.tsx`), task 22 (`SessionMenu` resolves its anchor rect from the `data-session-menu-anchor` attribute this task puts on the `⋯` button), task 23 (`⌘1`–`⌘9` and `⌘\` call `focusSession`, and this task's scroll effect is what makes that visible), task 24 (the defect suite asserts defects 3 and 4 against these files, and the design-system audit greps `develop.css`).
- **Adds one selector to contract §3.7**, in task 17's `store/selectors.ts`, per shared-shape §4 (which says a derived read becomes a selector rather than an inline `.filter()` in a component): `selectSelectedBranch` — the `Branch` for `selection.branchId`, needed by `DevelopLens` to give `EmptyDevelop` its `branch` prop. It is additive, it changes no signature §3.7 fixes, and step 3 writes it with the doc comment shared-shape §4 requires. **It adds no worktree-by-id selector.** Task 18 already adds `selectWorktreeById(id)` to the same file, with exactly the flatten-then-find body this task would have written, so `SessionPane` imports that — two names for one derivation is the defect §4 exists to stop. The name this task first reached for, `selectWorktree`, is in any case already taken by the `uiSlice` **action** of contract §3.3, which navigates rather than reads.
- **Departs from contract §6.5** in one place, argued in step 6: the header renders the harness **name** where §6.5's table says "harness command", because the footer already prints the command and printing `claude` twice in one 520-pixel pane is noise. Same fact, better-chosen representation; if a reviewer disagrees the change is one identifier.

- [ ] **Step 1: Preflight — confirm the substrate tasks 18 and 19 left, and do not patch it**

Every symbol the greps below check belongs to an earlier task, and every one of them is a compile error rather than a test failure if it is missing — which sounds safe until you notice that a missing symbol makes *this* task's author reach into *another* task's file. Shared-shape §1 is explicit that a silent local patch is how Group C produced nine Criticals. **If any check below fails, stop and report it to the orchestrator.** The single selector addition in step 3 is the only edit this task makes outside `components/develop/`, and it is additive by construction. Every grep below is anchored at `^`, per shared-shape §3A: an unanchored count is satisfied by a doc comment and so fails against correct code.

```bash
cd desktop
# (a) task 19 — the terminal binding and the state the footer reads
grep -cE "^export const Terminal" src/components/Terminal.tsx          # expect 1
grep -cE "^export function useAttachState" src/api/attachRegistry.ts   # expect 1
# (b) task 18 — the status dot, the one clock, and the worktree read this task REUSES
grep -cE "^export function StatusDot" src/components/common/StatusDot.tsx  # expect 1
grep -cE "^export function useHeartbeat" src/hooks/heartbeat.ts        # expect 1
grep -cE "^export const selectWorktreeById" src/store/selectors.ts     # expect 1
# (c) task 17 — the store reads this task depends on
grep -cE "^export const selectPaneSessionIds" src/store/selectors.ts   # expect 1
grep -cE "^export const selectResolvedDefaults" src/store/selectors.ts # expect 1
grep -cE "^export function toCreateRequest" src/store/defaults.ts      # expect 1
# (d) task 15 — the five typed builders this task uses, in task 14's fixtures.ts
grep -cE "^export function a(Session|Worktree|Branch|Project|Harness)" src/test/fixtures.ts  # expect 5
```

Notes on what these checks are *not*. They do not check `selectSelectedBranch`: it is this task's addition and step 3 creates it, so a hit there means another Group D task added it first and step 3 must be re-read against what is already present rather than duplicating it. They do not check `CanvasLayout`, which does not exist yet and must not — task 21 creates it, and step 13 reserves the branch it goes in. And check (d) is one grep over five names rather than five greps: `grep -cE` counts matching **lines**, the five builders are five lines, and a count below 5 names the fixture file as the thing to fix rather than sending you to five different plans. **If it reads 0, stop** — shared-shape §3 puts the six typed builders in task 15, not in task 14 and not in task 17, and a 0 here means task 15 shipped without filling the point task 14 reserved.

- [ ] **Step 2: Write the failing test for `SessionPane`**

Each behaviour below is one a later refactor would otherwise quietly delete. **Known defect 3** — "denying a permission dead-ends the session" — becomes, in Phase 2, *the pane renders an input surface for every status*: the cockpit owns no permission model at all (defect 8), so a prompt answered "no" inside the PTY must leave a live, typeable terminal behind it. That is two assertions: a `<Terminal/>` for all four statuses, and `readOnly` false for three of them. **Focus intent** (§6.5, §6.8) is next: a pointer landing on the pane chrome sets `selection.sessionId` *and* moves DOM focus onto xterm's own focus target, while a pointer landing on the `⋯` button sets the selection and leaves focus alone, because a menu that opens with focus already stolen cannot be driven from the keyboard. **The exited bar** keeps its scrollback, prints `exitReason` verbatim and offers Restart, because a dropped socket destroys nothing. And **the footer** is asserted by what it does **not** contain.

The `Terminal` stand-in carries a real `textarea.xterm-helper-textarea`, which is the element xterm.js actually renders as its focus target and the element `SessionPane` looks for. That is the one detail in this file that is a claim about a third-party library rather than about our code; step 7 explains the fallback for the case where it is ever not true.

Create `desktop/src/components/develop/SessionPane.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPane } from "./SessionPane";
import { resetCockpit, useCockpit } from "../../store/store";
import { aHarness, aSession, aWorktree } from "../../test/fixtures";
import type { Session, SessionStatus } from "../../store/types";

// §1.5: a component test never renders a glyph. The stand-in records the two props
// SessionPane passes, and carries xterm's own focus target — handing DOM focus to that
// element is the behaviour under test (§6.8).
vi.mock("../Terminal", () => ({
  Terminal: ({ sessionId, readOnly }: { sessionId: string; readOnly: boolean }) => (
    <div data-testid="terminal" data-session={sessionId} data-readonly={String(readOnly)}>
      <textarea className="xterm-helper-textarea" data-testid="xterm" readOnly />
    </div>
  ),
}));

// vi.hoisted, not a bare const: vi.mock is hoisted above the imports, and the factory runs
// while ./SessionPane's import graph is evaluated — before a top-level `const` has been
// initialised. A bare const here throws "Cannot access 'createSessionMock' before
// initialization" and looks like a mocking bug rather than a TDZ one.
const { createSessionMock } = vi.hoisted(() => ({ createSessionMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession: createSessionMock } }));

const HARNESS = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask", "plan"],
});

const WORKTREE = aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "evidence-upload" });

/** 41 minutes and 30 seconds old, so formatElapsed prints "41m" and the second boundary
 *  cannot flip it mid-run. */
function seed(over: Partial<Session> = {}): Session {
  const session = aSession({
    id: "s-1",
    title: "Claude Code · evidence-upload",
    projectId: "p-1",
    branchId: "b-1",
    worktreeId: "w-1",
    harness: "claude-code",
    model: "opus",
    effort: "high",
    permissionMode: "ask",
    status: "running",
    exitReason: "",
    createdAt: new Date(Date.now() - 41 * 60_000 - 30_000).toISOString(),
    ...over,
  });
  useCockpit.setState({
    harnesses: [HARNESS],
    worktreesByBranch: { "b-1": [WORKTREE] },
    sessions: { [session.id]: session },
    sessionIdsByWorktree: { "w-1": [session.id] },
  });
  return session;
}

describe("SessionPane", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
  });

  it("renders a terminal for starting, running, idle and exited", () => {
    const statuses: SessionStatus[] = ["starting", "running", "idle", "exited"];
    for (const status of statuses) {
      resetCockpit();
      const session = seed({ status });
      const view = render(<SessionPane sessionId={session.id} variant="column" />);
      expect(screen.getByTestId("terminal").getAttribute("data-session")).toBe("s-1");
      view.unmount();
    }
  });

  it("wires stdin for every status except exited", () => {
    const cases: [SessionStatus, string][] = [
      ["starting", "false"],
      ["running", "false"],
      ["idle", "false"],
      ["exited", "true"],
    ];
    for (const [status, readOnly] of cases) {
      resetCockpit();
      const session = seed({ status });
      const view = render(<SessionPane sessionId={session.id} variant="column" />);
      expect(screen.getByTestId("terminal").getAttribute("data-readonly")).toBe(readOnly);
      view.unmount();
    }
  });
```

```tsx
  it("a pointer on the pane chrome focuses the session and hands DOM focus to the terminal", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    await userEvent.click(screen.getByRole("region", { name: /evidence-upload/ }));

    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    expect(document.activeElement).toBe(screen.getByTestId("xterm"));
  });

  it("the ⋯ button focuses the session, opens its menu, and does NOT steal DOM focus", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    const more = screen.getByRole("button", { name: /session menu/i });
    await userEvent.click(more);

    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    expect(useCockpit.getState().sessionMenuFor).toBe("s-1");
    expect(document.activeElement).not.toBe(screen.getByTestId("xterm"));
    // Task 22 resolves the menu's DOMRect from this attribute rather than editing this file.
    expect(more.getAttribute("data-session-menu-anchor")).toBe("s-1");
  });

  it("an exited pane keeps its scrollback, prints exitReason verbatim and offers Restart", async () => {
    const session = seed({ status: "exited", exitReason: "idle timeout" });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-2", worktreeId: "w-1", branchId: "b-1", status: "starting" }),
    );
    render(<SessionPane sessionId={session.id} variant="column" />);

    expect(screen.getByTestId("terminal").getAttribute("data-readonly")).toBe("true");
    expect(screen.getByText("idle timeout")).toBeDefined();

    await userEvent.click(screen.getByRole("button", { name: /restart/i }));

    // Restart reuses the dead session's own settings, not the resolved defaults: the user
    // chose them once and a restart is not a chance to silently change them.
    expect(createSessionMock).toHaveBeenCalledWith({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
      effort: "high",
      permissionMode: "ask",
    });
    await waitFor(() => expect(useCockpit.getState().sessions["s-2"]).toBeDefined());
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    // The dead session stays listed: the transcript is the artifact.
    expect(useCockpit.getState().sessions["s-1"]).toBeDefined();
  });

  it("the footer names four facts and invents no fifth", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const foot = container.querySelector(".pane-foot");
    expect(foot).not.toBeNull();
    const text = (foot?.textContent ?? "").toLowerCase();

    expect(text).toContain("claude");   // harness.command
    expect(text).toContain("opus");     // session.model
    expect(text).toContain("41m");      // formatElapsed(session.createdAt)
    expect(text).toContain("detached"); // attachRegistry's own phase, nothing attached here

    // docs/AGENTIC-UX.md §2 property 7: no element whose justifying fact cannot be named.
    // A PTY carries no spend, no token accounting and no context window.
    for (const invented of ["$", "token", "context", "%", "cost"]) {
      expect(text).not.toContain(invented);
    }
  });
});
```

- [ ] **Step 3: Watch it fail, then add the selector it will need**

```bash
cd desktop
npm test -- src/components/develop/SessionPane.test.tsx
```

**What you must see:** a transform-time resolution failure, not an assertion failure, because none of the modules under test exist yet. Vitest reports it as a failed *file* with zero tests collected:

```
 FAIL  src/components/develop/SessionPane.test.tsx [ src/components/develop/SessionPane.test.tsx ]
Error: Failed to resolve import "./SessionPane" from "src/components/develop/SessionPane.test.tsx". Does the file exist?
```

The frames printed under that line come from Vite's transform pipeline and their paths change with the Vite version, so they are not reproduced here — match on the two lines above and on the summary's `Test Files  1 failed (1)`. **If instead you see `Tests  6 failed (6)` with `TypeError: Cannot read properties of undefined`, a `components/develop/SessionPane.tsx` already exists** and another agent has been in this directory; stop and report it rather than overwriting.

Now add the one selector this task needs, to **task 17's** `desktop/src/store/selectors.ts`. Shared-shape §4: a derived read becomes a selector, with a doc comment naming the surface that needs it, and the plan says it is an addition to §3.7. This is that. Append it at the foot of the file, below the last existing export, and change nothing above:

**The worktree read is not added here.** `SessionPane` owes `SessionPaneHeader` a `Worktree | undefined` and the derivation is a flatten-then-find over `worktreesByBranch` — which is exactly `selectWorktreeById(id)`, already added to this file by task 18 and already checked by step 1. Writing a second one under a second name is the thing shared-shape §4 forbids, and the obvious second name, `selectWorktree`, is the `uiSlice` action of contract §3.3: one identifier, two concepts, and TS2440 for any file that imports both.

```ts
/** The Branch named by `selection.branchId`. Needed by DevelopLens, which owes EmptyDevelop
 *  a `Branch | undefined` (§6.5). Returns the stored object, so its identity is stable
 *  across a session poll (the poller refetches sessions only).
 *  Addition to contract §3.7 by task 20, per Group D shared-shape §4. */
export const selectSelectedBranch = (s: CockpitState): Branch | undefined => {
  const { projectId, branchId } = s.selection;
  if (projectId === null || branchId === null) return undefined;
  return (s.branchesByProject[projectId] ?? []).find((b) => b.id === branchId);
};
```

`Branch` is almost certainly already in `selectors.ts`'s type import — task 18's `selectBranchById` returns one. If it is not, add it to the single `import type { … } from "./types"` rather than adding a second import line.

- [ ] **Step 4: Write `develop.css`, and reserve task 21's prefix in it**

The rules below govern this file and task 24's `designsystem.test.ts` asserts every one of them: **the first line is `@import "../../styles/tokens.css";`** (contract §5.5 and shared-shape §3B.1 — task 24 strips comments, takes the first non-empty line and matches it against `^@import "(?:\./|(?:\.\./)+styles/)tokens\.css";$`), **no literal hex** (every colour is a token from §5.3), **no colour function at all outside the token layer** — no `oklch()`, `rgb()`, `hsl()` or `color-mix()`, so a colour this file needs and cannot name is a token added to task 14's `tokens.css`, never a local literal — and **no `transition: all`**, no gradient, and no indigo or violet hue. One more is self-imposed and worth stating because it looks like an omission: **this file declares no transition at all.** `docs/DESIGN-SYSTEM.md` says motion must explain causality or be deleted, and nothing here has causality to explain — a focus ring that fades in is a ring that is wrong for 180 ms, and `box-shadow` is neither `transform` nor `opacity` so animating it would break the rule on the property list as well. The pane's focused ring appears on the frame the focus moves.

Elevation is the house form: a 1px ring plus a layered shadow, never a flat border, which is why every raised surface here reaches for an elevation token — `var(--e-card)` on the pane, `var(--e-raised)` under the focused pane's accent ring, `var(--e-btn)` on the exited bar's Restart and the zero state's two actions — rather than `border: 1px solid`; every `border` declaration in the file is `border: 0`. Radius follows element size — `--r-card` for the pane, `--r-ctl` for the buttons inside it — rather than one value everywhere. The footer's mono strings carry no radius of their own and so reference no `--r-*` token: they are text in a flush bar, not chips, and a token named in prose but referenced by no rule is how a reader ends up hunting for a rule that was never written.

Create `desktop/src/components/develop/develop.css`:

```css
@import "../../styles/tokens.css";

/* develop.css — the Develop lens.
 *
 * The @import is the FIRST line, not a nicety: contract §5.5 requires it, shared-shape
 * §3B.1 restates it, and task 24's audit strips comments before reading the first line,
 * so this header block may sit under it but no rule may.
 *
 * PREFIX OWNERSHIP (Group D shared-shape §2, which overrides contract §6.5 where they
 * differ): task 20 owns `.pane-*` and `.cols-*`. Task 21 owns `.cv-*` and appends its
 * block under the header at the FOOT of this file, without reflowing a rule above it.
 * Task 22 adds nothing here; its menu is styled by task 18's menu CSS.
 *
 * No literal hex, no colour function, no gradient, no `transition: all`, no indigo/violet
 * — task 24 asserts every one of those. No transition of any kind here: see the plan's
 * step 4. The word appears twice in this header and nowhere else in the file, which is
 * why step 15's guard is anchored at the start of a line (shared-shape §3A).
 */

/* ── Columns: the horizontal scroll container ────────────────────────────────────── */

.cols-scroll {
  /* Not a token: a pane width is a layout decision local to this lens, and promoting it
     to tokens.css would invite a second consumer that does not exist. */
  --pane-w: 520px;

  height: 100%;
  min-height: 0;
  display: flex;
  align-items: stretch;
  gap: 12px;
  padding: 12px;
  overflow-x: auto;
  overflow-y: hidden;
  /* proximity, not mandatory: a mandatory snap fights scrollIntoView and makes a two-pane
     drag feel like it is resisting the user. */
  scroll-snap-type: x proximity;
  background: var(--page);
}

.cols-pane {
  flex: 0 0 var(--pane-w);
  min-width: 0;
  display: flex;
  scroll-snap-align: center;
}

.cols-empty {
  height: 100%;
  display: grid;
  place-items: center;
  padding: 32px;
  background: var(--page);
}

/* ── The pane ────────────────────────────────────────────────────────────────────── */

.pane {
  width: 100%;
  min-height: 0;
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  overflow: hidden;
  border-radius: var(--r-card);
  background: var(--surface);
  box-shadow: var(--e-card);
}

.pane--focused {
  box-shadow: 0 0 0 1px var(--accent-ring), var(--e-raised);
}

.pane--tile {
  height: 100%;
}

/* The session is gone from the store but something still renders it — a purge that raced
   a render. Say so in words rather than showing an empty frame. */
.pane--gone {
  display: grid;
  /* Resets .pane's `auto 1fr auto auto`. Without this the single child lands in the first
     auto row and `place-items: center` centres it horizontally only, which reads as a
     message stuck to the top of an empty frame. */
  grid-template-rows: 1fr;
  place-items: center;
  padding: 24px;
  color: var(--ink-3);
  font: 400 12px/1.5 var(--sans);
}

.pane-head {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 8px 8px 8px 10px;
  background: var(--surface);
  box-shadow: 0 1px 0 0 var(--line-soft);
}

/* Task 21's drag handle. The attribute, not the class, is what its pointer handler binds
   to; the class only says the cursor is a grab. */
.pane-head--tile {
  cursor: grab;
  user-select: none;
}

.pane-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ink);
  font: 500 12px/1.4 var(--sans);
}

.pane-sub {
  color: var(--ink-3);
  font: 400 11px/1.4 var(--sans);
  white-space: nowrap;
}
```

```css
.pane-more {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  border-radius: var(--r-ctl);
  background: transparent;
  color: var(--ink-3);
  font: 400 13px/1 var(--sans);
  cursor: pointer;
}

.pane-more:hover {
  background: var(--hover);
  color: var(--ink);
}

.pane-body {
  min-height: 0;
  overflow: hidden;
  background: var(--page);
  /* tabIndex={-1} in the TSX makes this focusable as the fallback focus target; the outline
     would be wrong on a container the user never tabs to. */
  outline: none;
}

/* The exited bar. Neutral, not red: an exited session is a completed one, and colouring
   every terminal state as a failure is how a UI teaches people to ignore red. */
.pane-exit {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 6px 8px 6px 10px;
  background: var(--inset);
  box-shadow: 0 -1px 0 0 var(--line-soft);
}

.pane-exit-why {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ink-2);
  font: 400 11px/1.4 var(--sans);
}

.pane-exit-restart {
  padding: 3px 9px;
  border: 0;
  border-radius: var(--r-ctl);
  background: var(--surface);
  box-shadow: var(--e-btn);
  color: var(--ink);
  font: 500 11px/1.4 var(--sans);
  cursor: pointer;
}

.pane-exit-restart:hover:not(:disabled) {
  background: var(--hover-2);
}

.pane-exit-restart:disabled {
  color: var(--ink-3);
  cursor: default;
}

.pane-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  background: var(--surface);
  box-shadow: 0 -1px 0 0 var(--line-soft);
  color: var(--ink-3);
  font: 400 11px/1.4 var(--mono);
}

.pane-foot-fact {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-foot-sep {
  color: var(--ink-4);
}

/* ── Canvas (`.cv-*`) — TASK 21 APPENDS BELOW THIS LINE ───────────────────────────────
 * Shared-shape §2: the canvas plane, tiles, drag affordances and z-order are task 21's,
 * they live under this header, and no rule above it is reflowed. If task 21 needs a
 * `.pane-*` change it asks rather than editing one in place, so this diff stays readable.
 */
```

The `.pane` grid is four rows — head, body, exit bar, foot — and the exit bar row collapses to zero when the session is not exited, because a `grid-template-rows: auto 1fr auto auto` with three children distributes exactly as `auto 1fr auto` would. That is why the exited state costs no layout shift when it arrives: the pane's other rows do not move, the terminal shortens.

- [ ] **Step 5: Implement `SessionPaneFooter`**

Four facts, one word each, and `attachLabel` exported so a test can name a phase rather than matching prose. The phase comes from `useAttachState` — **the registry, not the store** — because the store knows whether the server thinks the session is running and the registry knows whether *this window* has a socket to it, and those are different questions. Contract §6.5's interfaces list makes the footer the registry's only component-side reader besides `Terminal`.

`useHeartbeat()` is called here and in no other file this task writes. It re-renders its caller once a second, which is correct for an elapsed counter and wrong for anything else — §7.1 names a rail row doing it as the second most likely performance defect in Group D. The footer is a leaf, so the once-a-second re-render stops here and never reaches the terminal.

Create `desktop/src/components/develop/SessionPaneFooter.tsx`:

```tsx
import type { JSX } from "react";
import { useAttachState } from "../../api/attachRegistry";
import type { AttachState } from "../../api/attachRegistry";
import { useHeartbeat } from "../../hooks/heartbeat";
import { formatElapsed } from "../../lib/time";
import { selectHarness } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Session } from "../../store/types";

/** One word per phase, exhaustive over AttachPhase. "failed" reads as "not attached"
 *  because the session itself is alive — the socket is what failed, and telling a user
 *  their agent failed when it is still working is the lie this wording avoids. */
export function attachLabel(state: AttachState): string {
  switch (state.phase) {
    case "detached":
      return "detached";
    case "connecting":
      return "connecting";
    case "attached":
      return "attached";
    case "reconnecting":
      return state.attempts > 1 ? `reconnecting ${state.attempts}` : "reconnecting";
    case "ended":
      return "ended";
    case "failed":
      return "not attached";
  }
}
```

```tsx
export function SessionPaneFooter({ session }: { session: Session }): JSX.Element {
  const nowMs = useHeartbeat();
  const harness = useCockpit(selectHarness(session.harness));
  const attach = useAttachState(session.id);

  // harness.command is the fact; session.harness (the id) is the honest fallback while the
  // catalogue is still loading. Never a hard-coded "claude".
  const facts: string[] = [harness?.command ?? session.harness];
  if (session.model !== "") facts.push(session.model);
  facts.push(formatElapsed(session.createdAt, nowMs));
  facts.push(attachLabel(attach));

  return (
    <footer className="pane-foot" title={attach.reason === "" ? undefined : attach.reason}>
      {facts.map((fact, i) => (
        <span key={fact + String(i)} className="pane-foot-fact">
          {i > 0 ? <span className="pane-foot-sep" aria-hidden="true">{" · "}</span> : null}
          {fact}
        </span>
      ))}
    </footer>
  );
}
```

`attach.reason` is rendered as a `title`, verbatim, and nowhere else — the registry's contract is that the reason is a sentence for a human and never a code or a stack (task 19, `AttachState`). Putting it in the footer's body would make the footer's width jump when a socket drops, which is the layout shift AGENTIC-UX §10 rule 7 bans; putting it nowhere would make a failed attach unexplainable. A `title` is the third option and it costs no pixels.

Tabular figures are not set here. §7.3 applies `font-variant-numeric: var(--numeric)` once on `<body>` in `app.css`, inherited everywhere, so a component that opts in can also forget — the point of doing it once is that none of them can.

- [ ] **Step 6: Implement `SessionPaneHeader`**

The header is the pane's identity line and, in the `tile` variant, task 21's drag handle. Two details in it exist for other tasks. The first is `data-pane-drag-handle`, which task 21's pointer handler binds to — an attribute rather than a class, because a class is a styling decision that a later redesign may legitimately rename and a handle is a contract. The second is `data-session-menu-anchor`, which is how task 22 resolves the `⋯` menu's `DOMRect` without editing this file: `uiSlice.sessionMenuFor` carries only a `SessionId`, `SessionMenu`'s props (§6.5) want a `DOMRect | null`, and the two are bridged by `document.querySelector('[data-session-menu-anchor="…"]')?.getBoundingClientRect()`. Shared-shape §1 does not list task 22 as an editor of this file, so the seam has to be an attribute.

**The one declared departure from contract §6.5.** Its table says the header shows "harness command"; this header shows `harness.name` and falls back to the command. The footer already prints the command, and a 520-pixel pane that prints `claude` twice spends its most valuable line on a repeat. `Claude Code · evidence-upload` in the header and `claude · opus · 41m · attached` in the footer say strictly more than the contract's reading does, with no new fact invented — `name` and `command` are both fields of the same catalogue entry. If a reviewer prefers the literal reading it is a one-identifier change.

`StatusDot` takes `live`, and `live` is `status === "running"` and nothing else. §7.5 gives `starting` a hollow ring, `idle` a dot at `--idle` and `exited` a square at `--exited`, none of them animated, so passing `live` for any of the other three would put a breathing dot on a session that is doing nothing — and the shared phase exists precisely so that the dots that *are* breathing read as one system.

Create `desktop/src/components/develop/SessionPaneHeader.tsx`:

```tsx
import type { JSX } from "react";
import { StatusDot } from "../common/StatusDot";
import { selectHarness } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Session, Worktree } from "../../store/types";

/** The screen-reader and hover sentence for the dot. `exited` carries its reason because
 *  "exited" alone does not tell a user whether they killed it or it fell over. */
export function statusLabel(session: Session): string {
  if (session.status === "exited" && session.exitReason !== "") {
    return `exited — ${session.exitReason}`;
  }
  return session.status;
}

export function SessionPaneHeader({
  session,
  worktree,
  variant,
}: {
  session: Session;
  worktree: Worktree | undefined;
  variant: "column" | "tile";
}): JSX.Element {
  const harness = useCockpit(selectHarness(session.harness));
  const openSessionMenu = useCockpit((s) => s.openSessionMenu);

  return (
    <header
      className={variant === "tile" ? "pane-head pane-head--tile" : "pane-head"}
      data-pane-drag-handle={variant === "tile" ? "true" : undefined}
    >
      <StatusDot status={session.status} live={session.status === "running"} label={statusLabel(session)} />
      <span className="pane-title">{worktree?.name ?? session.title}</span>
      <span className="pane-sub">{harness?.name ?? session.harness}</span>
      <button
        type="button"
        className="pane-more"
        // Task 22 reads this attribute to anchor SessionMenu; see the plan's step 6.
        data-session-menu-anchor={session.id}
        aria-label={`Session menu for ${worktree?.name ?? session.title}`}
        onClick={() => openSessionMenu(session.id)}
      >
        {"⋯"}
      </button>
    </header>
  );
}
```

The `⋯` button has no `onPointerDown` of its own. `SessionPane`'s handler runs first on the way down, sees a `<button>` under the pointer, sets the selection and returns without moving DOM focus — so the click lands on the button with focus where the browser put it, which is what makes the menu keyboard-drivable. That ordering is the whole mechanism and step 7 is where it is written.

- [ ] **Step 7: Implement `SessionPane` — focus intent, the exited bar, and the one create path**

`SessionPane` owns **focus intent** and it is the only component in Group D that does. §6.8 splits the concern three ways and the split only works if exactly one component translates a pointer into it: the pane calls `focusSession` on every pointer-down, then hands DOM focus to the terminal, then gets out of the way. Three exceptions, each of which is a bug if it is missing:

1. **A pointer on a `<button>`, `<a>`, `<input>`, `<select>` or `<textarea>`** sets the selection and returns. The `⋯` button is the case that matters: a menu that opens with focus already yanked into a terminal cannot be driven with the arrow keys, and §6.7 has overlays capturing and restoring focus, which they cannot do if it moved out from under them on the way down.
2. **A pointer inside the terminal body** sets the selection and returns. xterm owns DOM focus in its own subtree and it owns the selection drag; calling `focus()` mid-drag fights it, and xterm has already focused itself by the time the event finishes bubbling.
3. **A session the store no longer holds** renders a named frame rather than crashing. A purge that races a render is rare and recoverable; a `Cannot read properties of undefined` in a pane is not.

**The hand-off calls `e.preventDefault()` and that is not optional.** Pointer Events makes "fire the compatibility mouse events" the default action of `pointerdown`, and the browser's `mousedown` handling then focuses the nearest focusable ancestor of the event target — or, finding none, blurs whatever is focused. `@testing-library/user-event` 14 emulates exactly that, and skips it when `pointerdown` was default-prevented. A `<section>` with no `tabindex` has no focusable ancestor, so without the `preventDefault` this handler focuses the terminal on `pointerdown` and the browser un-focuses it on `mousedown`, leaving `document.activeElement === document.body`. The call sits **after** both early returns so the two cases that want the browser's own behaviour still get it.

The focus target is `textarea.xterm-helper-textarea`, xterm.js's own hidden input, with two fallbacks — any `textarea` in the body, then the body element itself, which carries `tabIndex={-1}` for exactly that reason. The fallback chain exists because the first selector is a claim about a third-party library's internals: if a future xterm renames it, focus lands on the body instead of the document, the keyboard map still works, and nothing throws.

**`startSessionOnWorktree` lives in this file, and that is a decision the contract does not make.** Two callers need one create path — this pane's Restart and `EmptyDevelop`'s two named actions — and contract §9 fixes task 20's file list at seven modules, so a shared `startSession.ts` would be an eighth. Of the seven, `SessionPane` is the one both callers can import without a cycle (`DevelopLens → EmptyDevelop → SessionPane` and `DevelopLens → ColumnsLayout → SessionPane` both terminate). **`sessionVerbs.ts` is where it belongs, and no Phase 2 task moves it there.** Task 22 creates that module, but its plan names `startSessionOnWorktree` nowhere, and its Files list carries neither `SessionPane.tsx` nor `EmptyDevelop.tsx` — so both call sites a relocation has to rewrite sit outside the files that task may edit. The comment on the export records the destination for whoever eventually makes the move rather than assigning it to task 22. Restart deliberately reuses the dead session's own harness, model, effort and permission mode rather than the resolved defaults: the user chose them once, and a restart that silently upgrades the model is the class of surprise this product exists to avoid.

Create `desktop/src/components/develop/SessionPane.tsx`:

```tsx
import { useRef, useState } from "react";
import type { JSX, PointerEvent } from "react";
import { api } from "../../api";
import type { ApiError } from "../../api/client";
import { Terminal } from "../Terminal";
import { SessionPaneFooter } from "./SessionPaneFooter";
import { SessionPaneHeader } from "./SessionPaneHeader";
import { toCreateRequest } from "../../store/defaults";
import { selectSession, selectWorktreeById } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionDefaults, SessionId, WorktreeId } from "../../store/types";

/** The one create path in Group D's develop surface: this pane's Restart and EmptyDevelop's
 *  two named actions. It is here rather than in a module of its own because contract §9
 *  fixes this task's file list at seven, and here rather than in EmptyDevelop because both
 *  directions of import have to terminate.
 *
 *  Not permanent: it belongs in `sessionVerbs.ts` alongside Resume/Fork/Restart/Kill. No
 *  Phase 2 task moves it — task 22 creates that module but rewrites neither call site —
 *  so whoever moves it leaves a re-export here or updates both call sites together. */
export async function startSessionOnWorktree(
  worktreeId: WorktreeId,
  defaults: SessionDefaults,
): Promise<void> {
  const before = useCockpit.getState();
  const harness = before.harnesses.find((h) => h.id === defaults.harness);
  if (harness === undefined) {
    // Not a crash and not silence: the catalogue has not loaded, or the harness was removed
    // server-side between the render and the click.
    before.pushToast(`The ${defaults.harness} harness is not in the catalogue.`);
    return;
  }

  try {
    // toCreateRequest strips every field this harness does not accept (§3.6); sending
    // `effort` to a shell is a 400 the client could have avoided.
    const session = await api.createSession(toCreateRequest(worktreeId, defaults, harness));
    const after = useCockpit.getState();
    after.upsertSession(session);
    after.focusSession(session.id);
  } catch (e) {
    // Every client method rejects with ApiError and nothing else (§4). These two codes are
    // different sentences and must not be collapsed: the first means our process is down,
    // the second means Docker is. Every other code carries the server's own message.
    const err = e as ApiError;
    const message =
      err.code === "network_unavailable"
        ? "The control plane is not running. Start it, then try again."
        : err.code === "backend_unavailable"
          ? "Docker is not running, so a sandbox cannot be started."
          : err.message;
    useCockpit.getState().pushToast(message);
  }
}

/** xterm.js renders a hidden `textarea.xterm-helper-textarea` and focuses it; that is the
 *  element a keystroke reaches. Two fallbacks, because the selector is a claim about a
 *  third-party's internals and the pane must still be keyboard-reachable if it changes. */
function focusTerminalIn(host: HTMLElement | null): void {
  if (host === null) return;
  const target =
    host.querySelector<HTMLElement>("textarea.xterm-helper-textarea") ??
    host.querySelector<HTMLElement>("textarea") ??
    host;
  target.focus();
}
```

```tsx
export function SessionPane({
  sessionId,
  variant,
}: {
  sessionId: SessionId;
  variant: "column" | "tile";
}): JSX.Element {
  const session = useCockpit(selectSession(sessionId));
  // selectWorktreeById is task 18's (§3.7 + shared-shape §4); this task adds no second
  // one. Its parameter is a required WorktreeId, and `WorktreeId` is a string alias, so
  // the absent case passes "" — no worktree carries that id, the selector returns
  // undefined, and the header renders session.title. Same idiom as EmptyDevelop's
  // selectWorktrees(branch?.id ?? "").
  const worktree = useCockpit(selectWorktreeById(session?.worktreeId ?? ""));
  const focused = useCockpit((s) => s.selection.sessionId === sessionId);
  const focusSession = useCockpit((s) => s.focusSession);
  const bodyRef = useRef<HTMLDivElement>(null);
  const [restarting, setRestarting] = useState(false);

  const onPointerDown = (e: PointerEvent<HTMLElement>): void => {
    focusSession(sessionId);
    const target = e.target as HTMLElement | null;
    if (target === null) return;
    // Exception 1: an interactive control keeps the focus the browser is about to give it.
    if (target.closest("button, a, input, select, textarea") !== null) return;
    // Exception 2: inside the terminal, xterm owns DOM focus and the selection drag.
    if (bodyRef.current !== null && bodyRef.current.contains(target)) return;
    // Firing the compatibility mouse events is pointerdown's DEFAULT ACTION, so preventing
    // it suppresses the mousedown focus fix-up that would otherwise run after this line.
    // Without it the fix-up looks for a focusable ancestor of the pane chrome, finds none
    // — <section> is not focusable — and blurs to <body>, undoing the hand-off §6.8
    // requires. Only on this path: the two returns above deliberately leave the browser's
    // own focus behaviour alone, which is what keeps the ⋯ menu keyboard-drivable.
    e.preventDefault();
    focusTerminalIn(bodyRef.current);
  };

  const onRestart = (): void => {
    if (session === undefined || restarting) return;
    setRestarting(true);
    void startSessionOnWorktree(session.worktreeId, {
      harness: session.harness,
      model: session.model,
      effort: session.effort,
      permissionMode: session.permissionMode,
    }).finally(() => setRestarting(false));
  };

  // Exception 3. A purge that raced a render: say so, do not throw.
  if (session === undefined) {
    return <section className="pane pane--gone">This session is no longer listed.</section>;
  }

  const className = ["pane", variant === "tile" ? "pane--tile" : "", focused ? "pane--focused" : ""]
    .filter((c) => c !== "")
    .join(" ");

  return (
    <section
      className={className}
      data-variant={variant}
      data-session-id={session.id}
      aria-label={`${worktree?.name ?? session.title} — ${session.status}`}
      onPointerDown={onPointerDown}
    >
      <SessionPaneHeader session={session} worktree={worktree} variant={variant} />
      <div className="pane-body" ref={bodyRef} tabIndex={-1}>
        {/* Unconditional, for every status — known defect 3. readOnly is the ONLY thing
            `exited` changes, and even then the scrollback stays: the transcript is the
            artifact, and a dropped socket destroys nothing. */}
        <Terminal sessionId={session.id} readOnly={session.status === "exited"} />
      </div>
      {session.status === "exited" ? (
        <div className="pane-exit">
          <span className="pane-exit-why">
            {session.exitReason === "" ? "the process exited" : session.exitReason}
          </span>
          <button
            type="button"
            className="pane-exit-restart"
            onClick={onRestart}
            disabled={restarting}
          >
            Restart
          </button>
        </div>
      ) : null}
      <SessionPaneFooter session={session} />
    </section>
  );
}
```

`exitReason` is rendered verbatim, with a neutral fallback only when the server sent an empty string. §2's comment on the field is explicit that it is free text for the user — "killed", "process exited", "idle timeout", "sandbox disappeared while the app was not running" — and summarising those four into one badge would throw away the only sentence that distinguishes a session the user ended from one that fell over.

- [ ] **Step 8: Watch the `SessionPane` suite pass**

```bash
cd desktop
npm test -- src/components/develop/SessionPane.test.tsx
npm run typecheck
```

**What you must see:** six passing tests in one file. Vitest's default reporter prints a `✓` line per file and the four-line summary block; the durations and the version in the `RUN` banner vary, so do not match on those:

```
 ✓ src/components/develop/SessionPane.test.tsx (6 tests)

 Test Files  1 passed (1)
      Tests  6 passed (6)
```

`tsc --noEmit` prints nothing at all on success. If it prints, the diagnostic carries four things — file, line and column, `TS` code, message — in one of two layouts: `src/components/develop/SessionPane.tsx(NN,5): error TS2345: …` when output is piped, and the pretty form `src/components/develop/SessionPane.tsx:NN:5 - error TS2345: …` with the source line and a caret under it when TypeScript 6 sees a TTY. The line is shown as `NN` because it depends on how the file above was transcribed; match on the code and the path, not on the coordinates or the punctuation between them.

The failures below are likely enough to name in advance. **`expected null not to be null` on `container.querySelector(".pane-foot")`** means the footer rendered under a different class; the test queries by class because the footer has no handle of its own — a `<footer>` inside a sectioning element maps to no ARIA role and carries no accessible name — and the assertion that matters is a negative one, no spend and no token count and no context window, which has to be scoped to the footer rather than run over the whole container. Task 24's design-system audit never reads a class name: its eleven checks cover hex literals, colour functions, gradients, shadows, the token import and the banned hue arc. **`expected <body> to be <textarea …>` in the focus test** means the terminal never kept focus, and there are two causes in that order of likelihood: the `e.preventDefault()` is missing or sits above one of the early returns, so user-event's mousedown fix-up blurred to `<body>` after the hand-off; or the `closest(…)` guard matched the `<section>` itself, which it will if a wrapper element was given `role="button"`. The reverse assertion failing in the `⋯` test means the `preventDefault` is **not** early-returned past and the menu lost its focus. **A hang, or `Test timed out in 5000ms`, in the Restart test** means `createSessionMock` has no `mockResolvedValue` for that path and the `await` never settles; the `finally` then never clears `restarting` and the button stays disabled.

- [ ] **Step 9: Write the failing test for `ColumnsLayout`, `DevelopLens` and `EmptyDevelop`**

Contract §9 gives this task two test files, and this is the second, so the layout chooser and the zero state are asserted here rather than in two more. That is a decision, not an oversight: `DevelopLens` is a short block of branching whose only observable behaviour is which of these three renders, and a third file would test the same three renders through a different door.

**Defect 4 is the centre of this file.** A session created off-screen with no focus is the prototype's failure, and the fix is two effects in `ColumnsLayout`: one that notices an id it has not seen and calls `focusSession` on it, one that scrolls the focused pane into view. Both need negative assertions as much as positive ones — the seeding pass must **not** focus anything, or opening the app with nine sessions yanks the viewport to whichever pane sorted first.

Create `desktop/src/components/develop/ColumnsLayout.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ColumnsLayout } from "./ColumnsLayout";
import { DevelopLens } from "./DevelopLens";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";

vi.mock("../Terminal", () => ({
  Terminal: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="terminal" data-session={sessionId} />
  ),
}));

const { createSessionMock } = vi.hoisted(() => ({ createSessionMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession: createSessionMock } }));
```

```tsx
const CLAUDE = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask"],
});

const SHELL = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  available: true,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
});

/** One project, one branch, and as many worktrees/sessions as the case needs. */
function seedBranch(opts: { worktrees: string[]; sessionIdsByWorktree: Record<string, string[]> }): void {
  const sessions = Object.fromEntries(
    Object.entries(opts.sessionIdsByWorktree).flatMap(([worktreeId, ids]) =>
      ids.map((id) => [id, aSession({ id, projectId: "p-1", branchId: "b-1", worktreeId, status: "running" })]),
    ),
  );
  useCockpit.setState({
    harnesses: [CLAUDE, SHELL],
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload" })] },
    worktreesByBranch: {
      "b-1": opts.worktrees.map((id) => aWorktree({ id, projectId: "p-1", branchId: "b-1", name: id })),
    },
    sessions,
    sessionIdsByWorktree: opts.sessionIdsByWorktree,
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    layout: "columns",
  });
}

describe("ColumnsLayout", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    // setup.ts defines scrollIntoView on Element.prototype (jsdom implements none); spy on
    // the same object it was defined on, or the spy is installed on a prototype that only
    // inherits the property and never sees the call.
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("renders one pane per id, in the order given", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    const { container } = render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    const panes = [...container.querySelectorAll("[data-session-id]")];
    expect(panes.map((p) => p.getAttribute("data-session-id"))).toEqual(["s-1", "s-2"]);
  });

  it("the first pass seeds its id set and steals no focus", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    expect(useCockpit.getState().selection.sessionId).toBeNull();
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it("a newly created session is focused and scrolled into view", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    const view = render(<ColumnsLayout sessionIds={["s-1"]} />);

    // Server order is createdAt descending, so a new session arrives at the head. The
    // store write lands while the component is mounted, so it is act-wrapped too, or React
    // logs "An update to ColumnsLayout inside a test was not wrapped in act(...)" over an
    // otherwise green run.
    act(() => {
      seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-2", "s-1"] } });
    });
    view.rerender(<ColumnsLayout sessionIds={["s-2", "s-1"]} />);

    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ inline: "center", block: "nearest" });
  });

  it("focusing an existing session scrolls its pane into view without creating anything", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    act(() => {
      useCockpit.getState().focusSession("s-2");
    });

    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ inline: "center", block: "nearest" });
    expect(createSessionMock).not.toHaveBeenCalled();
  });
});
```

**Both mid-test store writes are wrapped in `act`, and a bare call would be a real failure rather than a flake.** zustand notifies its subscribers synchronously, but the subscriber is `useSyncExternalStore`, and React 19 schedules the resulting re-render on SyncLane and flushes it **in a microtask** — so a synchronous `expect` on the next line runs before the component has re-rendered and before the scroll effect has run, and the assertion reads zero calls every time. `act` flushes the render and its effects before it returns. Use `act`, not `waitFor`: a `waitFor` would pass even if the effect's dependency array were wrong, which is the one thing these two tests exist to pin.

Append the two remaining blocks to the same file:

```tsx
describe("DevelopLens", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("renders Columns for the selected branch's sessions", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    const { container } = render(<DevelopLens />);

    expect(container.querySelectorAll("[data-session-id]")).toHaveLength(2);
  });

  it("renders Columns for the canvas layout too, until task 21 lands", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    useCockpit.setState({ layout: "canvas" });
    const { container } = render(<DevelopLens />);

    // Task 21 REPLACES this expectation with one about CanvasLayout. It is here so the
    // interim behaviour is a decision on the record rather than an accident.
    expect(container.querySelector(".cols-scroll")).not.toBeNull();
  });

  it("renders the zero state when the branch has no sessions", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    const { container } = render(<DevelopLens />);

    expect(container.querySelector("[data-session-id]")).toBeNull();
    expect(screen.getByRole("button", { name: /Claude Code/ })).toBeDefined();
  });
});

describe("EmptyDevelop", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("says a branch with no worktrees can hold no sessions, and offers no button that would 404", () => {
    seedBranch({ worktrees: [], sessionIdsByWorktree: {} });
    render(<DevelopLens />);

    expect(screen.getByText(/no worktrees/i)).toBeDefined();
    expect(screen.queryByRole("button", { name: /Claude Code/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /shell/i })).toBeNull();
  });

  it("offers the two named actions on a worktree with no sessions, and creates with resolved defaults", async () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-9", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "starting" }),
    );
    render(<DevelopLens />);

    await userEvent.click(screen.getByRole("button", { name: /Claude Code/ }));

    // effort is absent: resolveDefaults leaves it "" and toCreateRequest omits an empty one.
    expect(createSessionMock).toHaveBeenCalledWith({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
      permissionMode: "ask",
    });
  });

  it("the shell action sends no model, effort or permission mode", async () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-10", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "shell" }),
    );
    render(<DevelopLens />);

    await userEvent.click(screen.getByRole("button", { name: /open a shell/i }));

    expect(createSessionMock).toHaveBeenCalledWith({ worktreeId: "w-1", harness: "shell" });
  });
});
```

- [ ] **Step 10: Watch it fail**

```bash
cd desktop
npm test -- src/components/develop/ColumnsLayout.test.tsx
```

**What you must see:** the same transform-time resolution failure as step 3, naming the first unresolvable import in the file:

```
 FAIL  src/components/develop/ColumnsLayout.test.tsx [ src/components/develop/ColumnsLayout.test.tsx ]
Error: Failed to resolve import "./ColumnsLayout" from "src/components/develop/ColumnsLayout.test.tsx". Does the file exist?
```

Vite reports one unresolved import at a time, so after step 11 creates `ColumnsLayout.tsx` this same command fails again on `./DevelopLens`. That is expected and is why steps 11 to 13 are not individually green — step 14 is the first point at which this file can pass.

- [ ] **Step 11: Implement `ColumnsLayout` — the scroll container and defect 4**

Two effects, and the split matters. The first watches the id list and focuses an id it has not seen before; the second watches `selectFocusedSessionId` and scrolls. Keeping them apart is what makes `⌘1`–`⌘9`, the palette and a rail-row click all scroll for free — §6.8 says those callers "request it via `focusSession`; they never call `scrollIntoView` themselves", and that is only true if the scroll is driven by the focused id rather than by the create.

The seen-set is a `useRef<Set<SessionId> | null>`, `null` until the first effect pass. A `Set` initialised to the first render's ids would look equivalent and is not: the effect would then never see the first pass at all and could not tell "mounted with nine sessions" from "nine sessions just arrived".

The pane is found by walking `children` and reading `dataset.columnFor` rather than with a `querySelector` on an interpolated id. Session ids are server-generated and currently `s-` plus hex, so a selector would work today; walking a nine-element list costs nothing and cannot be broken by an id that later contains a character `CSS.escape` would have had to handle.

Create `desktop/src/components/develop/ColumnsLayout.tsx`:

```tsx
import { useEffect, useRef } from "react";
import type { JSX } from "react";
import { SessionPane } from "./SessionPane";
import { selectFocusedSessionId } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";

export function ColumnsLayout({ sessionIds }: { sessionIds: SessionId[] }): JSX.Element {
  const focusedId = useCockpit(selectFocusedSessionId);
  const focusSession = useCockpit((s) => s.focusSession);
  const scrollRef = useRef<HTMLDivElement>(null);
  /** null until the first effect pass. See the plan's step 11. */
  const seen = useRef<Set<SessionId> | null>(null);

  // Defect 4, half one: a session that appears while the app is open takes focus.
  useEffect(() => {
    const previous = seen.current;
    seen.current = new Set(sessionIds);
    if (previous === null) return; // the seeding pass never steals focus
    const fresh = sessionIds.filter((id) => !previous.has(id));
    if (fresh.length === 0) return;
    focusSession(fresh[0]); // server order is createdAt descending: the newest is first
  }, [sessionIds, focusSession]);

  // Defect 4, half two: the focused pane is brought into the viewport, whoever focused it.
  useEffect(() => {
    if (focusedId === null) return;
    const host = scrollRef.current;
    if (host === null) return;
    const pane = [...host.children].find(
      (el) => (el as HTMLElement).dataset.columnFor === focusedId,
    );
    // `inline: center` on the horizontal axis, `block: nearest` so a pane never drags the
    // page vertically (§6.8: this is the only scrollIntoView call in Group D).
    (pane as HTMLElement | undefined)?.scrollIntoView({ inline: "center", block: "nearest" });
  }, [focusedId, sessionIds]);

  return (
    <div className="cols-scroll" ref={scrollRef} data-testid="cols-scroll">
      {sessionIds.map((id) => (
        <div className="cols-pane" key={id} data-column-for={id}>
          <SessionPane sessionId={id} variant="column" />
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 12: Implement `EmptyDevelop` — three zero states, not one**

`docs/DESIGN-SYSTEM.md`'s floor is that every state is designed, and "empty" here is three different situations that deserve three different sentences. **No branch selected** is a navigation state and its answer is the rail. **A branch with no worktrees** is the one contract §10.3 calls out explicitly: `POST /v1/sessions` requires an existing `worktreeId`, worktree creation is Phase 3, so this state must *say so in words* rather than offering a button that 404s. **A worktree with no sessions** is the only one with actions, and it gets the two §7.7 names.

The primary action is labelled from the resolved defaults — `＋ Claude Code` when that is what would start, `＋ Codex` when it is not. Hard-coding the label would make it lie the first time a user changes their default, and §6.4's rule that the gate is always the descriptor and never `harness === "claude-code"` applies to the wording as much as to the logic. The shell action renders only when a `shell` harness is actually in the catalogue; there is no spinner in either state, and no skeleton, because §7.7 allows none that shifts a pixel.

Which worktree gets the session is the same rule §6.9 gives `⌘⏎`: the selected worktree, falling back to the branch's first. Stating it once here and once there is deliberate — two surfaces that resolve it differently is how a user ends up with a session on a worktree they were not looking at.

`selectResolvedDefaults` is read through `useShallow` and `selectWorktrees` is not, and the difference is task 17's amendment to contract §3.7: `resolveDefaults` builds a fresh `SessionDefaults` object on every call, so a bare `useCockpit(selectResolvedDefaults)` re-renders this component on every 2 s poll under zustand 5's `Object.is` — the same defect §3.7 names for the two array selectors. `selectWorktrees` returns the stored array, or a stable empty constant when the branch is absent, so it compares equal and needs neither `useShallow` nor a `?? []`.

Create `desktop/src/components/develop/EmptyDevelop.tsx`:

```tsx
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { startSessionOnWorktree } from "./SessionPane";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Branch } from "../../store/types";

export function EmptyDevelop({ branch }: { branch: Branch | undefined }): JSX.Element {
  // §3.7: selectWorktrees returns a stable empty-array constant when the branch is absent,
  // so this needs no useShallow and no `?? []`.
  const worktrees = useCockpit(selectWorktrees(branch?.id ?? ""));
  const selectedWorktreeId = useCockpit((s) => s.selection.worktreeId);
  // useShallow is mandatory: resolveDefaults builds a fresh SessionDefaults on every call, so
  // a bare read re-renders this on every 2 s poll (task 17's amendment to §3.7).
  const defaults = useCockpit(useShallow(selectResolvedDefaults));
  const harnesses = useCockpit((s) => s.harnesses);

  if (branch === undefined) {
    return (
      <div className="pane-empty">
        <p className="pane-empty-line">Select a branch in the rail to start working on it.</p>
      </div>
    );
  }

  if (worktrees.length === 0) {
    return (
      <div className="pane-empty">
        <p className="pane-empty-line">
          {branch.name} has no worktrees yet, and a session runs inside one.
        </p>
        <p className="pane-empty-note">
          Creating a worktree arrives in Phase 3. Until then, check out this branch in a
          worktree from the command line and it will appear in the rail.
        </p>
      </div>
    );
  }

  const target = worktrees.find((w) => w.id === selectedWorktreeId) ?? worktrees[0];
  const primary = harnesses.find((h) => h.id === defaults.harness);
  const shell = harnesses.find((h) => h.id === "shell");

  return (
    <div className="pane-empty">
      <p className="pane-empty-line">No sessions on {target.name}.</p>
      <div className="pane-empty-actions">
        <button
          type="button"
          className="pane-empty-go"
          onClick={() => void startSessionOnWorktree(target.id, defaults)}
        >
          {`＋ ${primary?.name ?? defaults.harness}`}
        </button>
        {shell === undefined ? null : (
          <button
            type="button"
            className="pane-empty-go"
            onClick={() =>
              void startSessionOnWorktree(target.id, {
                harness: shell.id,
                model: "",
                effort: "",
                permissionMode: "ask",
              })
            }
          >
            Open a shell
          </button>
        )}
      </div>
    </div>
  );
}
```

The shell call passes `permissionMode: "ask"` and `toCreateRequest` strips it, because `shell.permissionModes` is empty — that is §3.6's rule, not a special case here, and it is why the shell action's test asserts the body is exactly `{ worktreeId, harness }`. Passing `""` instead would be a lie about a legal `PermissionMode`; passing the field and letting the pure function drop it keeps one place that knows what a harness accepts.

Add the `.pane-empty*` rules below — including the `:hover` on the last of them — to `develop.css`, under the pane block and above the reserved Canvas header:

```css
.pane-empty {
  max-width: 420px;
  display: grid;
  gap: 10px;
  justify-items: start;
}

.pane-empty-line {
  margin: 0;
  color: var(--ink);
  font: 500 13px/1.5 var(--sans);
}

.pane-empty-note {
  margin: 0;
  color: var(--ink-3);
  font: 400 12px/1.6 var(--sans);
}

.pane-empty-actions {
  display: flex;
  gap: 8px;
}

.pane-empty-go {
  padding: 5px 11px;
  border: 0;
  border-radius: var(--r-ctl);
  background: var(--surface);
  box-shadow: var(--e-btn);
  color: var(--ink);
  font: 500 12px/1.4 var(--sans);
  cursor: pointer;
}

.pane-empty-go:hover {
  background: var(--hover-2);
}
```

- [ ] **Step 13: Implement `DevelopLens`, and reserve task 21's branch inside it**

A short block of branching, and one comment worth more than all of it. Shared-shape rule 1 puts the reservation here; rule 2 binds task 21 to add its `layout` read, its `selectCanvasSessionIds` read and its `<CanvasLayout/>` line **in one step**, which is why this file reads neither of those today — a `layout` variable sitting unused for a task would either be deleted by `noUnusedLocals` or kept by a needless `void`, and neither survives review. Rule 3 is why the string `<CanvasLayout` appears mid-line in a block comment and never at the start of one.

`selectPaneSessionIds` is consumed with `useShallow`. Without it, zustand 5's `Object.is` sees a freshly built array on every 2 s poll and re-renders the whole lens thirty times a minute — §3.7 calls this the single most likely performance defect in Group D and says it is invisible until ten panes are open. `selectSelectedBranch` needs none: it returns a stored object.

Create `desktop/src/components/develop/DevelopLens.tsx`:

```tsx
import "./develop.css";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { ColumnsLayout } from "./ColumnsLayout";
import { EmptyDevelop } from "./EmptyDevelop";
import { selectPaneSessionIds, selectSelectedBranch } from "../../store/selectors";
import { useCockpit } from "../../store/store";

export function DevelopLens(): JSX.Element {
  // useShallow is mandatory here: this selector builds a new array every call (§3.7).
  const sessionIds = useCockpit(useShallow(selectPaneSessionIds));
  const branch = useCockpit(selectSelectedBranch);

  if (sessionIds.length === 0) {
    return (
      <div className="cols-empty">
        <EmptyDevelop branch={branch} />
      </div>
    );
  }

  /* TASK 21 — the Canvas branch goes HERE, and this comment is its reservation.
     In ONE step (Group D shared-shape rule 2), task 21 adds the `layout` and
     `selectCanvasSessionIds` reads above and replaces the return below with a ternary whose
     canvas arm is <CanvasLayout sessionIds={canvasIds}/>. Neither read exists yet on
     purpose: an unused local is either deleted by the compiler's check or kept alive by a
     `void`, and both are worse than adding it with its consumer.
     Until task 21 lands, Canvas renders Columns rather than an empty plane — task 18's
     layout toggle already ships, and a main area that goes blank when a user flips a toggle
     reads as a crash. The component name above is deliberately mid-line so task 21's
     anchored guard reads 0 here and 1 after its change (shared-shape rule 3). */
  return <ColumnsLayout sessionIds={sessionIds} />;
}
```

`develop.css` is imported here and in no other file in this task. One import point keeps the cascade order deterministic — Vite would dedupe a second import, but it would not guarantee where in the bundle it landed relative to task 18's component CSS, and two rules of equal specificity are resolved by order.

- [ ] **Step 14: Watch the Columns suite pass**

```bash
cd desktop
npm test -- src/components/develop/ColumnsLayout.test.tsx
```

**What you must see:** ten passing tests in one file — four in `ColumnsLayout`, three in `DevelopLens`, three in `EmptyDevelop`:

```
 ✓ src/components/develop/ColumnsLayout.test.tsx (10 tests)

 Test Files  1 passed (1)
      Tests  10 passed (10)
```

Each failure below points at a different mistake. **`expected null to be "s-2"`** in *a newly created session is focused* means the seeding guard is wrong — a `useRef(new Set(sessionIds))` initialiser instead of `useRef(null)` makes the first pass consume the ids, so the second pass sees no new one. **`expected "scrollIntoView" to be called with …` reporting zero calls** means the pane lookup missed: check that `data-column-for` is on the `.cols-pane` wrapper, which is a direct child of the scroll container, and not on the `<section>` inside it, because the effect walks `children` and does not descend. **`expected "s-1" to be null` in *the first pass steals no focus*** means the focus effect ran before the seeding guard. And **`Unable to find an accessible element with the role "button" and name /Claude Code/`** in the zero-state case means `selectResolvedDefaults` resolved to something else — print it, because the likely cause is that the fixture harness has `available: false` and `resolveDefaults` rule 1 fell through to the first available harness, which is correct behaviour and a wrong fixture.

- [ ] **Step 15: Run the whole suite, typecheck, and the guards**

```bash
cd desktop
npm test
npm run typecheck
```

**What you must see:** every test file in the desktop tree passing, including tasks 14 to 19's. The summary block names the file and test totals; those totals grow with each Group D task, so match on the absence of a `failed` segment rather than on a number. `npm run typecheck` prints nothing on success.

Then the guards, in five groups. Two protect task 21's contract with this task, one protects task 22's, one is task 24's design-system audit run early so a violation is found by the author rather than by the task that audits everything, and one proves the single selector addition landed once. **Every one of them is anchored**, per shared-shape §3A: an unanchored count is satisfied by a comment and so fails against correct code. Every expected number below was read off a run of that exact line against the files these steps produce.

```bash
cd desktop
# (a) shared-shape rule 3: the reservation is visible to a naive grep and invisible to an
#     anchored one, which is exactly what makes task 21's own guard meaningful.
grep -c '<CanvasLayout' src/components/develop/DevelopLens.tsx                        # expect 1
grep -cE '^[[:space:]]*<CanvasLayout' src/components/develop/DevelopLens.tsx          # expect 0
# (b) shared-shape §2: task 20 writes no .cv-* rule, and leaves the header that says so.
#     The second guard targets a COMMENT, so it anchors on the comment opener instead.
grep -cE '^\.cv-' src/components/develop/develop.css                                  # expect 0
grep -cE '^/\*.*TASK 21 APPENDS BELOW' src/components/develop/develop.css             # expect 1
# (c) task 22's anchor seam exists exactly once, as an attribute and not in prose.
grep -cE '^[[:space:]]*data-session-menu-anchor=' \
     src/components/develop/SessionPaneHeader.tsx                                     # expect 1
# (d) task 24's audit, early: token import first, no literal hex, no colour function, no
#     gradient, no transition. `transition` is unanchored-unsafe — the file's own header
#     says the word twice, so a bare `grep -ci transition` reads 2 and even `transition:`
#     reads 1 (the header quotes `transition: all`). Anchor on the declaration.
grep -cE '^@import "(\.\./)+styles/tokens\.css";' src/components/develop/develop.css  # expect 1
grep -ciE '#[0-9a-f]{3,8}' src/components/develop/develop.css                         # expect 0
grep -ciE '^[[:space:]]*[a-z-]+[[:space:]]*:.*(oklch|rgba?|hsla?|lab|lch|oklab|color-mix)\(' \
     src/components/develop/develop.css                                               # expect 0
grep -c -- '-gradient(' src/components/develop/develop.css                            # expect 0
grep -ciE '^[[:space:]]*transition' src/components/develop/develop.css                # expect 0
# (e) the one addition to task 17's file landed once — not zero, not twice.
grep -cE '^export const selectSelectedBranch' src/store/selectors.ts                  # expect 1
grep -cE '^export const selectWorktree =' src/store/selectors.ts                      # expect 0
```

`grep -c` exits **1** when the count is zero, so the **seven** guards that expect `0` — the anchored `<CanvasLayout`, `^\.cv-`, hex, colour function, `-gradient(`, the anchored `transition`, and `selectWorktree =` — will look like failures to a shell running under `set -e` or to a CI step that checks exit codes. Read the printed number, not the exit status; if you want them scripted, append `|| true` to each. Run as a block the twelve lines print, in order: `1 0 0 1 1 1 0 0 0 0 1 0`.

The patterns that look over-engineered are each a measured correction. `-gradient(` rather than `gradient`, because this file's own header says "no gradient" and a bare `grep -c gradient` reads **1**. `^[[:space:]]*transition` rather than `transition` or even `transition:`, because the header says the word twice and quotes `transition: all` once, so those read **2** and **1**. And `^export const selectWorktree =` rather than a prefix match, because `selectWorktrees` (task 17) and `selectWorktreeById` (task 18) both live in `selectors.ts` and a prefix match reads **2** against a correct file. Every number on every guard above came from running it against the files these steps produce.

- [ ] **Step 16: Commit**

```bash
cd ..
git add desktop/src/components/develop/develop.css \
        desktop/src/components/develop/DevelopLens.tsx \
        desktop/src/components/develop/ColumnsLayout.tsx \
        desktop/src/components/develop/SessionPane.tsx \
        desktop/src/components/develop/SessionPaneHeader.tsx \
        desktop/src/components/develop/SessionPaneFooter.tsx \
        desktop/src/components/develop/EmptyDevelop.tsx \
        desktop/src/components/develop/SessionPane.test.tsx \
        desktop/src/components/develop/ColumnsLayout.test.tsx \
        desktop/src/store/selectors.ts
git commit -F - <<'MSG'
feat(cockpit): Columns layout and the session pane

DevelopLens chooses the layout, ColumnsLayout owns the horizontal scroll, and
SessionPane is one session: header, live Terminal, footer. EmptyDevelop covers
the three zero states.

Fixes known defect 3: the pane renders a Terminal for every status, and
readOnly is set only for `exited`, where the scrollback stays because the
transcript is the artifact.

Fixes known defect 4: ColumnsLayout focuses an id it has not seen before and
scrolls the focused pane into view with { inline: "center", block: "nearest" }.
Its first pass seeds the id set without stealing focus.

SessionPane owns focus intent (contract §6.8): a pointer anywhere in the pane
calls focusSession and hands DOM focus to the terminal, except on an
interactive control or inside the terminal itself.

The footer shows harness command, model, elapsed and attach state and nothing
else. No cost, no token count, no context meter: none is derivable from a PTY,
and docs/AGENTIC-UX.md §2 property 7 forbids an element whose justifying fact
cannot be named.

develop.css owns .pane-* and .cols-*; .cv-* is reserved for task 21 under a
header at the foot of the file. DevelopLens carries task 21's Canvas branch as
a comment whose component name sits mid-line, so task 21's anchored guard reads
0 before its change and 1 after (Group D shared-shape rules 1 and 3).

Adds selectSelectedBranch to store/selectors.ts, per shared-shape §4. It is an
addition to contract §3.7 and changes no signature it fixes. The pane's worktree
read reuses task 18's selectWorktreeById rather than adding a second name for
the same derivation.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
```

The heredoc is the one thing in this task that has bitten before: a multi-line commit message passed through an extra shell boundary can terminate early, and the symptom is `here-document delimited by end-of-file` followed by the message body executing as commands. If that happens, write the message to a file and use `git commit -F /tmp/task20.txt` instead.

Ten paths on the `git add`, listed by name. No `-A`, no `-u`, no `.` — other agents are working in this tree and a wildcard stage takes their files with it. `desktop/src/store/selectors.ts` is the only one outside `components/develop/`, and it is there because shared-shape §4 says a derived read becomes a selector; if `git status` shows any other file staged, unstage it and find out who wrote it before committing.

