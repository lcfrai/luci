### Task 22: Session lifecycle verbs

This is the task that makes the cockpit a cockpit rather than a viewer. Everything before it reads: task 18 draws the rail and dispatches a right-click, task 20 draws the pane and its `⋯` button, task 19 streams the PTY. Nothing so far ends a session, renames one, restarts one or takes one back. This task adds the verbs contract §9's task 22 row names — Resume, Fork into…, Restart, Rename…, Kill, Forget, Open a shell here, Mark ready — in one module, reachable from both menus, each one wired to exactly one client method and each one honest about what it did.

**Optimistic mutation with an Undo window is two different mechanisms and collapsing them is the defect this task exists to avoid.** `docs/DESIGN-SYSTEM.md` requires an Undo window rather than a confirm dialog, and contract §2 fixes the toast: plain text, an optional `undo`, 6000 ms with one. But "Undo" means two incompatible things depending on the verb. **Kill and Forget have no compensating request** — you cannot un-kill a PTY and `purgeSession` deletes the record — so for those two the store changes now and the request is *not sent until the window closes*; Undo cancels the send and restores the snapshot, and nothing ever reached the server. **Rename and Restart do have a server-side inverse**, so those send immediately, patch the store optimistically, and roll the snapshot back if the server refuses. Resume, Fork and Open-a-shell create rather than mutate; they have nothing to roll back and get no Undo at all. A single "optimistic" helper that sent everything immediately would offer an Undo button on Forget that could not work, which is worse than no button.

**What happens to a verb that fails after the window closes**, stated once because every deferred verb behaves this way: the toast has already gone, so there is no Undo button left to press, and the failure is reported as a **plain** toast carrying the server's own sentence. The optimistic patch is rolled back — the killed row comes back as running, the forgotten session reappears — and task 19's 2 s poller reconciles whatever the rollback guessed wrong within one tick. The cockpit never leaves the user looking at a row that the server does not have, and it never silently swallows a refusal that arrived six seconds late.

**`Mark ready` renders only when the server actually reports the flag, and step 1 checks which world this is rather than assuming.** Contract §10.2 requested `ready` on the worktree object and a `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}`; the Go side shipped it (the registry carries `WorktreeFlags` and `SetWorktreeReady`, and task 16's client carries `setWorktreeReady`). If that half is ever reverted, `Worktree.ready` is `undefined`, the verb is **not rendered at all**, and known defect 9 moves to Phase 3 with the merge queue — exactly as §10.2 says. The gate is a runtime check on the value, not a build-time constant, so a cockpit talking to an older control plane degrades by itself.

**What this task edits that it did not create, and the rules that follow from it.** Group D's shared-shape §1 lists task 22 as an editor of `components/menu/ContextMenu.tsx` and `components/rail/*Row.tsx`, both task 18's. **Rule 1 is satisfied wherever task 18 reserved a point, and this task writes into two places nobody reserved.** Task 18 reserved two commented insertion points in `ContextMenu.tsx` — `sessionItems()` and the `Mark ready` entry in `worktreeItems()` — and one in `SessionRow.tsx` for the pending affordance. `WorktreeRow.tsx` has **none**: task 18 did not anticipate the ready mark there. And task 20's `SessionPaneHeader.tsx` is not in §1's table for this task at all, yet task 18's dismissal rule names *task 22's `⋯` button* as a control that must carry `data-overlay-trigger` and task 20 shipped the button without it, so step 12 adds it there. Both unreserved insertions are declared in the departures below and carry guards written here rather than inherited. Rule 2 binds: the step that adds the `sessionVerbs(` call to `ContextMenu.tsx` runs *after* `sessionVerbs.ts` exists and its own suite is green, never before. Rule 3 and shared-shape §3A bind hardest: task 18's reserving comments contain the literal strings `sessionVerbs`, `mark-ready` and `data-pending=` — an unanchored `grep -c sessionVerbs ContextMenu.tsx` matches that comment prose and returns a non-zero count over a file with no call in it — so every step-13 guard that expects a positive count is anchored on a line whose first non-space character begins the code, and every count beside a guard is a measured one. The guards that expect **zero** are the opposite case: one of them exists precisely to count a comment string down to zero, and step 13 says so where it writes them.

**Facts the UI contract predates.** `GET /v1/harnesses` returns `[]`, never `null` — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test, so `harness.models`, `harness.efforts` and `harness.permissionModes` are always present and nothing here writes `?? []`. The error-code enum has **sixteen** members: Group C's ruling 7 added `non_loopback_publish`, which describes a sandbox that **is running**, so no verb in this file paraphrases a code into "not running" — every unrecognised code renders the server's `message` verbatim and only `network_unavailable` and `backend_unavailable` get a locally written sentence. Session status is **four** values — `starting`, `running`, `idle`, `exited` — so the verb gates read those four and nothing from AGENTIC-UX §4 rule 8's seven. And **a dropped WebSocket destroys nothing**: destruction is an explicit Kill, the idle TTL or the reaper, which is why an `exited` session keeps its row and its scrollback and why **Forget**, not a disconnect, is what removes it. There is no bearer token on the desktop→control-plane hop and none is meant to exist before Phase 8; nothing in this task sends, stores or displays a credential.

**On the expected-output blocks below.** `desktop/node_modules` contains no vitest — task 14 installs it — so **no command in this plan has been run and no reporter transcript here is transcribed from a real run**. Rather than invent one, every verification step states *what you must see*: the failing test's name, the assertion that fails and the reason it fails. Where reporter chrome appears it is marked approximate. If the chrome you get differs, the chrome in this document is wrong and the named assertion is what matters. Do not edit a test to match a transcript in a plan.

**The pre-commit greps in step 13 are the one exception, and they are measured.** Shared-shape §3A makes a guard whose number was reasoned about worse than no guard, so I reconstructed every file a guard reads — task 18's `ContextMenu.tsx`, `SessionRow.tsx` and `WorktreeRow.tsx` and task 20's `SessionPaneHeader.tsx`, each with this task's step-11 and step-12 edits applied, alongside the `sessionVerbs.ts` and `SessionMenu.tsx` steps 4, 5 and 9 create — in a scratch directory and ran every guard against them. The counts beside them in step 13 are that run's output, not an expectation.

One naming note that will otherwise cost an hour, and it has changed since this plan's first draft. **Shared-shape §3 now settles the fixtures completely**: there are exactly six typed builders — `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession`, `aHarness` — **task 15** adds all six into the insertion point task 14 reserves in `src/test/fixtures.ts`, there is no seventh, and no `make*Fixture` spelling survives anywhere. This task imports `aSession`, `aWorktree` and `aHarness`, passes an override object for every variant, and declares no builder of its own.

Two consequences run through every assertion below. **Ids come from task 14's `IDS`**, so the session under test is `IDS.session` (`"s-3f2a91c0"`), its worktree is `IDS.worktree`, its branch `IDS.branch` and its project `IDS.project` — no test here writes a literal id except the two extra rows it invents, `"w-2"` for the fork sibling and `"s-3"` for the third session in the rename test. And **the defaults are task 15's**: `aSession()` is `running`, titled `"Evidence upload"`, on harness `"claude-code"`; `aWorktree()` has `ready` deliberately **absent**, which is what makes the §10.2 gate testable; and `aHarness()` is the **shell** descriptor with `supportsResume: false`, which is why the resume test seeds a second, `claude-code` entry and overrides `supportsResume: true` on it — that override, not the `id` and `name` beside it, is what the Resume gate reads.

**Files:**
- `desktop/src/components/develop/sessionVerbs.ts` — the eight verbs, the two commit shapes, the pending registry, `renameSessionTo`, `markWorktreeReady` and the §10.2 gate `worktreeReadyItems` (new)
- `desktop/src/components/develop/sessionVerbs.test.ts` — gating, both commit shapes, the late failure, the pending registry, and contract §8's two named regression tests for defects 7 and 9 (new)
- `desktop/src/components/develop/SessionMenu.tsx` — the `⋯` menu: anchor resolution from task 20's attribute, and the inline rename input (new)
- `desktop/src/components/develop/SessionMenu.test.tsx` — anchor fallback, verb rendering, rename submit and cancel (new)
- `desktop/src/components/menu/ContextMenu.tsx` — **task 18's file.** Replaces `sessionItems`' body with `sessionVerbs(…)`, appends `Mark ready` to `worktreeItems`, and mounts `<SessionMenu/>`; argued in step 11 (modify)
- `desktop/src/components/menu/menu.css` — **task 18's file.** Three rules for the inline rename input. Shared-shape §2 sanctions this file explicitly: "its menu is styled by task 18's `menu/` CSS, which it also modifies", and §3B binds what those rules may contain (modify)
- `desktop/src/components/rail/SessionRow.tsx` — **task 18's file.** The `data-pending` affordance at the point task 18 reserved for it (modify)
- `desktop/src/components/rail/WorktreeRow.tsx` — **task 18's file.** The ready mark, so `Mark ready` has a visible effect; argued in step 12 (modify)
- `desktop/src/components/develop/SessionPaneHeader.tsx` — **task 20's file.** The `⋯` button gains `data-overlay-trigger`, the `aria-expanded` that makes the exemption apply to it, and a toggling `onClick`, so the control that opens the `⋯` menu can also close it; argued in step 12 (modify)

**Interfaces:**
- Produces:
  - `components/develop/sessionVerbs.sessionVerbs(sessionId: SessionId, deps: VerbDeps): MenuItem[]` — the session verb list for both menus, built from a store snapshot at open time
  - `components/develop/sessionVerbs.VerbDeps` — `{ createSession; close; beginRename }`, the three things a verb needs from its host surface
  - `components/develop/sessionVerbs.markWorktreeReady(worktree: Worktree): void` — the §10.2 verb's runner, optimistic with rollback
  - `components/develop/sessionVerbs.worktreeReadyItems(worktree: Worktree, close: () => void): MenuItem[]` — contract §8 defect 9's gate: the `Mark ready` row, or an **empty array** while the server does not report the flag. A function rather than JSX inside `ContextMenu`, because §8 puts defect 9's second regression test in `sessionVerbs.test.ts` and this task writes no `ContextMenu` test
  - `components/develop/sessionVerbs.renameSessionTo(sessionId: SessionId, title: string): void` — the rename runner: immediate commit, optimistic title, snapshot rollback on refusal
  - `components/develop/sessionVerbs.useSessionPending(id: SessionId): boolean` and `isSessionPending(id)` — the deferred-commit registry, read by `SessionRow`
  - `components/develop/sessionVerbs.sentenceFor(error: ApiError): string` — the two locally written sentences and the verbatim fallback
  - `components/develop/sessionVerbs.__resetSessionVerbs(): void` — clears the module-level registry and every in-flight timer between tests, in the manner of task 18's `__resetHeartbeat`
  - `components/develop/SessionMenu.SessionMenu` — `{ sessionId; anchor; onClose }`, exactly contract §6.5's props
  - `components/develop/SessionMenu.anchorRectFor(sessionId: SessionId): DOMRect` — the pane-attribute-then-rail-row-then-centre fallback chain
- Consumes: task 15's `Session`, `SessionId`, `Worktree`, `WorktreeId`, `Harness` (`store/types.ts`) and its `replaceById` (`lib/immutable.ts`, contract §8 defect 7); task 16's `api` singleton and `ApiError`; task 17's `useCockpit`, `selectSession`, `selectWorktrees`, `selectHarness`, the `upsertSession` / `removeSession` / `replaceSessions` / `setWorktrees` / `pushToast` / `setError` / `focusSession` / `openSessionMenu` / `closeSessionMenu` actions and `TOAST_TTL_UNDO`; task 18's `MenuItem`, `Popover`, `useCreateSession`, `useDismissable` (with `__resetOverlays` in the tests) and the three reserved insertion points; task 20's `data-session-menu-anchor` attribute; task 14's `IDS`; task 15's `aSession`, `aWorktree`, `aHarness` builders in `src/test/fixtures.ts`.
- Required by: task 23 (`buildCommands` lifts Kill, Restart and Resume into the palette by calling `sessionVerbs` rather than re-deriving the gates), task 24 (the defect suite asserts defect 9 against `Mark ready`, and the ten-defect proof runs the Undo window end to end).
- **Departs from contract §6.5 on one label.** §6.5's table says "Restart clean"; this menu says **Restart**. Phase 2 has no worktree mutation endpoint (§10.3), so the cockpit cannot promise a clean checkout, and a verb that claims one it cannot deliver is the failure mode `docs/AGENTIC-UX.md` §2 property 7 forbids. Same call, honest word. If a reviewer prefers the literal reading it is one string.
- **Amends contract §6.7**: `<SessionMenu/>` is mounted by `ContextMenu.tsx`, not by `AppShell.tsx`. Shared-shape §1 does not grant task 22 an edit to `AppShell.tsx` — that row belongs to task 23 — and `ContextMenu` is the only file this task may edit that AppShell already mounts exactly once. Argued in step 11.
- **Adds to contract §6.5**: `SessionMenu` gains an inline rename input rather than a dialog or `window.prompt`. Argued in step 9.
- **Adds to contract §6.5 and §6.2 one row neither lists**: `Copy session id`. It is the one session verb task 18 already shipped in `sessionItems()` — the single `return [{ id: "copy-session-id", … }]` under its reserving comment — and this task replaces that body wholesale, so dropping the row would *remove* a working verb to satisfy a list. It mutates nothing, needs no Undo, and is carried into both menus unchanged.
- **Departs from task 18's reserved worktree insertion point.** Task 18 suggested the guard `grep -cE '^[[:space:]]+id: "mark-ready"' ContextMenu.tsx`, which assumes the row's object literal is written in that file. It is not: contract §8 defect 9 puts the test *"the verb is absent when the server does not report `ready`"* in `sessionVerbs.test.ts`, and this task writes no `ContextMenu` test, so the gate is `worktreeReadyItems()` in `sessionVerbs.ts` and `ContextMenu` spreads it. The wiring guard moves with it, anchored, and step 13 runs both halves.
- **`WorktreeRow.tsx` has no reserved insertion point** (task 18 reserved one in `SessionRow.tsx` and two in `ContextMenu.tsx`, and none there). Step 12's ready mark is therefore an unreserved insertion into another task's file; it adds one element and one class name, reflows nothing, and carries its own anchored guard.
- **Edits one file shared-shape §1 does not list for this task at all**: task 20's `SessionPaneHeader.tsx`. Task 18's dismissal rule — *"A control that opens an overlay carries `data-overlay-trigger` … Task 22's `⋯` button and task 23's palette launcher both need it"* — names this button, and task 20 shipped it with the anchor attribute and neither the trigger attribute nor the `aria-expanded` that `useDismissable` reads to decide whether the exemption applies. Task 20 is done, so the consumer closes the gap: step 12 adds them, with the `aria-haspopup` that pairs with `aria-expanded`, and makes the `onClick` a toggle; step 13 guards it. No structural change, nothing removed, and task 20's own test on `data-session-menu-anchor` still passes.
- **Does not use `replaceById` for the rename**, which contract §8 defect 7 names task 22 as the consumer for. `sessions` is a `Record<SessionId, Session>` (§3.2), `replaceById` is `(readonly T[], T) => T[]` (task 15), and there is no array to splice — §3.2's own guarantee that `upsertSession` leaves the position unchanged is what defect 7 needs here, and the regression test §8 names asserts it. `replaceById` **is** consumed, in `markWorktreeReady`, where this task does hold an array — which is §8's other example, *"replacing one worktree in a list"*, verbatim.

- [ ] **Step 1: Preflight — confirm the substrate, and find out which world the `ready` flag is in**

Two things to establish before a line is written, and the second is the one §10.2 asks for by name. **First, that tasks 18 and 20 landed the seams this task needs.** If any of these prints a count other than the one beside it, stop: the earlier task is incomplete or was edited, and patching another task's file to get past a failed preflight is the Group C defect this whole document exists to prevent.

```bash
cd desktop
grep -cE '^export interface MenuItem' src/components/menu/ContextMenu.tsx            # 1 — task 18
grep -cE '^export function useCreateSession' src/hooks/useCreateSession.ts           # 1 — task 18
grep -cE 'data-session-menu-anchor=\{session\.id\}' src/components/develop/SessionPaneHeader.tsx  # 1 — task 20
grep -cE '^export function aSession' src/test/fixtures.ts                            # 1 — task 15
grep -cE '^export function a[A-Z]' src/test/fixtures.ts                              # 6 — task 15, §3.2's closed inventory
grep -cE '^export const TOAST_TTL_UNDO' src/store/slices/uiSlice.ts                  # 1 — task 17
```

**Second, the `ready` world check.** Contract §10.2 says in as many words: "If this is refused: `Worktree.ready` stays `undefined`, task 22 does not render the verb, and defect 9 moves to Phase 3 with the merge queue. Task 22's first step checks which world it is in." Run both halves — the Go side that produces the flag, and task 16's client that carries it:

```bash
grep -c 'SetWorktreeReady' ../control-plane/internal/registry/filestore.go   # ≥ 2 if the registry shipped it (4 today)
grep -cE '^  setWorktreeReady' src/api/client.ts                            # 1 if task 16's client carries it
grep -cE '^  ready\?: boolean;' src/store/types.ts                          # 1 — task 15 declares it optional either way
```

**I ran that grep against the live repository and it returns 4**, on lines 41, 419, 422 and 424: `control-plane/internal/registry/filestore.go` declares `SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error)` on the interface at line 41, documents it at 419, implements it at 422 and names it again in the empty-id error at 424, with the storage shape §10.2 asked for (`WorktreeFlags map[string]WorktreeFlags \`json:"worktreeFlags"\``). So the Go half exists. The guard is `≥ 2` rather than `= 4` deliberately: the count moves whenever someone rewords that error string, and a guard that fails on a reworded comment is the §3A failure mode. **Reading a Go file is the only thing this task does to one** — shared-shape §6 forbids a Group D task running `go test` or editing a `.go` file, and this step does neither. Note what that does **not** prove: `control-plane/internal/api` did not exist yet when this was checked, so the `PATCH` route itself is task 10's and is verified by task 10's tests, not here.

**Whichever way the greps come out, the code this task writes is the same.** The verb is gated at runtime on `worktree.ready !== undefined`, never on a build-time constant, because the cockpit is a desktop app that can meet an older control plane than the one it was built against. If the second grep returns 0, the client method is missing and step 5's `markWorktreeReady` will not typecheck — in that case, and only that case, drop `markWorktreeReady`, `worktreeReadyItems`, their three tests and step 12's `WorktreeRow` mark, and record in the commit body that §10.2's client half is absent. The test counts in steps 6 and 13 then read thirteen and twenty rather than sixteen and twenty-three. Do not add the method to `client.ts`; that file is task 16's and a structural twin of a shape another file owns is the defect class shared-shape §1 was written against.

- [ ] **Step 2: Write the failing tests for `sessionVerbs.ts`**

This suite is where the two commit shapes are pinned, and it is written first because the gates are the part a reviewer cannot check by eye. What it proves that prose cannot: a deferred verb sends **nothing** until the window closes; Undo inside the window cancels the send *and* restores the store; an Undo that arrives *after* the send changes nothing; a failure **after** the window closes rolls the row back **into its original position** and pushes a toast with no Undo on it; `Forget` is not offered on a session that is still alive; and the two regression tests contract §8 names by title — defect 7's *"renaming the middle session leaves its neighbours unchanged"* and defect 9's *"the verb is absent when the server does not report `ready`"* — both live here, because §8 puts them here and this task writes no `ContextMenu` test.

`vi.useFakeTimers()` is what makes the window testable at all. The deferred helper's timer and the toast's own timer both read `TOAST_TTL_UNDO`, so advancing by exactly that value is the moment the request is sent, and advancing by one millisecond less is the last moment Undo still works. Both are asserted.

Create `desktop/src/components/develop/sessionVerbs.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MenuItem } from "../menu/ContextMenu";
import { IDS, aHarness, aSession, aWorktree } from "../../test/fixtures";
import { TOAST_TTL_UNDO } from "../../store/slices/uiSlice";
import { resetCockpit, useCockpit } from "../../store/store";
import type { Session } from "../../store/types";
import { ApiError } from "../../api/client";

const { killMock, purgeMock, restartMock, renameMock, resumeMock, forkMock, readyMock } = vi.hoisted(
  () => ({
    killMock: vi.fn(),
    purgeMock: vi.fn(),
    restartMock: vi.fn(),
    renameMock: vi.fn(),
    resumeMock: vi.fn(),
    forkMock: vi.fn(),
    readyMock: vi.fn(),
  }),
);
vi.mock("../../api", () => ({
  api: {
    killSession: killMock,
    purgeSession: purgeMock,
    restartSession: restartMock,
    renameSession: renameMock,
    resumeSession: resumeMock,
    forkSession: forkMock,
    setWorktreeReady: readyMock,
  },
}));

import {
  __resetSessionVerbs,
  isSessionPending,
  markWorktreeReady,
  renameSessionTo,
  sessionVerbs,
  worktreeReadyItems,
} from "./sessionVerbs";

const DEPS = { createSession: vi.fn(), close: vi.fn(), beginRename: vi.fn() };
/** The fork target. IDS has one worktree and this suite needs two on the branch. */
const SIBLING = "w-2";
/** The third session in the rename test. IDS carries `session` and `sessionB` only. */
const THIRD = "s-3";
const find = (items: MenuItem[], id: string): MenuItem | undefined => items.find((i) => i.id === id);
```

The same file continues — one file, split only so the comments stay readable:

```ts
function seed(overrides: Partial<Session> = {}): void {
  // aHarness() IS the shell descriptor (task 15), so only the claude-code entry needs overrides.
  const claude = aHarness({ id: "claude-code", name: "Claude Code", supportsResume: true });
  useCockpit.getState().setHarnesses([claude, aHarness()]);
  useCockpit
    .getState()
    .setWorktrees(IDS.branch, [aWorktree(), aWorktree({ id: SIBLING, name: "retention-rules" })]);
  useCockpit.getState().upsertSession(aSession(overrides));
}

beforeEach(() => {
  vi.useFakeTimers();
  resetCockpit();
  __resetSessionVerbs();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("sessionVerbs — the gates", () => {
  it("offers Forget only on an exited session, and says why when it does not", () => {
    seed({ status: "running" });
    const running = find(sessionVerbs(IDS.session, DEPS), "forget");
    expect(running?.run).toBeUndefined();
    expect(running?.disabledReason).toContain("Kill it first");

    useCockpit.getState().upsertSession(aSession({ status: "exited", exitReason: "killed" }));
    expect(find(sessionVerbs(IDS.session, DEPS), "forget")?.run).toBeTypeOf("function");
  });

  it("offers Kill only while the session is alive", () => {
    seed({ status: "exited" });
    expect(find(sessionVerbs(IDS.session, DEPS), "kill")?.run).toBeUndefined();
    useCockpit.getState().upsertSession(aSession({ status: "idle" }));
    expect(find(sessionVerbs(IDS.session, DEPS), "kill")?.run).toBeTypeOf("function");
  });

  it("offers Resume only when the harness supports it", () => {
    seed({ harness: "claude-code" });
    expect(find(sessionVerbs(IDS.session, DEPS), "resume")?.run).toBeTypeOf("function");
    useCockpit.getState().upsertSession(aSession({ harness: "shell" }));
    const shellResume = find(sessionVerbs(IDS.session, DEPS), "resume");
    expect(shellResume?.run).toBeUndefined();
    expect(shellResume?.disabledReason).toContain("Shell");
  });

  it("renders one Fork row per other worktree on the branch, and disables the verb when there is none", () => {
    seed();
    expect(sessionVerbs(IDS.session, DEPS).map((i) => i.id)).toContain(`fork-${SIBLING}`);
    expect(sessionVerbs(IDS.session, DEPS).map((i) => i.id)).not.toContain(`fork-${IDS.worktree}`);

    useCockpit.getState().setWorktrees(IDS.branch, [aWorktree()]);
    const only = find(sessionVerbs(IDS.session, DEPS), "fork");
    expect(only?.run).toBeUndefined();
    expect(only?.disabledReason).toContain("§10.3");
  });

  it("returns a single honest row when the store does not hold the session", () => {
    seed();
    const items = sessionVerbs("s-nope", DEPS);
    expect(items).toHaveLength(1);
    expect(items[0]?.run).toBeUndefined();
  });
});
```

And the third block, in the same file — the two commit shapes and the late failure:

```ts
describe("sessionVerbs — the deferred commit and its window", () => {
  it("sends nothing until the window closes, and marks the row pending meanwhile", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(isSessionPending(IDS.session)).toBe(true);
    expect(killMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO - 1);
    expect(killMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    expect(killMock).toHaveBeenCalledWith(IDS.session);
    expect(isSessionPending(IDS.session)).toBe(false);
  });

  it("Undo inside the window cancels the request and restores the session", async () => {
    seed({ status: "running" });

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    const toast = useCockpit.getState().toasts.at(-1);
    expect(toast?.undo).toBeTypeOf("function");
    toast?.undo?.();

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO * 2);
    expect(killMock).not.toHaveBeenCalled();
    expect(isSessionPending(IDS.session)).toBe(false);
  });

  it("Undo pressed after the request has gone changes nothing", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    const toast = useCockpit.getState().toasts.at(-1);
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);
    expect(killMock).toHaveBeenCalledTimes(1);

    // The button is already off the screen; this is the millisecond-wide race, not a
    // reachable click. If `undone` could still be set here the rollback would put a running
    // row back over a session the server has already killed.
    toast?.undo?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
  });

  it("Forget removes the row now and purges when the window closes", async () => {
    seed({ status: "exited" });
    purgeMock.mockResolvedValue(undefined);

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]).toBeUndefined();

    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);
    expect(purgeMock).toHaveBeenCalledWith(IDS.session);
  });

  it("a failure AFTER the window closes rolls back, in place, and toasts with no Undo", async () => {
    seed({ status: "exited" });
    // A second session, prepended, so the restored row has a neighbour to be wrong about.
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    purgeMock.mockRejectedValue(new ApiError("conflict", "The session is still attached.", 409));

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);

    expect(useCockpit.getState().sessions[IDS.session]?.id).toBe(IDS.session);
    // Back where it was, not at the top: upsertSession PREPENDS an id the store no longer holds.
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      IDS.sessionB,
      IDS.session,
    ]);
    const last = useCockpit.getState().toasts.at(-1);
    expect(last?.message).toBe("The session is still attached.");
    expect(last?.undo).toBeUndefined();
    expect(useCockpit.getState().lastError?.code).toBe("conflict");
  });
});

describe("sessionVerbs — the immediate commit", () => {
  it("Restart patches to starting at once and adopts the server's answer", async () => {
    seed({ status: "exited", exitReason: "process exited" });
    restartMock.mockResolvedValue(aSession({ status: "running", exitReason: "" }));

    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("starting");
    expect(useCockpit.getState().sessions[IDS.session]?.exitReason).toBe("");

    await vi.runOnlyPendingTimersAsync();
    expect(restartMock).toHaveBeenCalledWith(IDS.session);
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");
  });

  it("a refused Restart rolls the snapshot back and renders the server's sentence verbatim", async () => {
    seed({ status: "exited", exitReason: "process exited" });
    restartMock.mockRejectedValue(
      new ApiError("non_loopback_publish", "The sandbox published 8080 off loopback.", 500),
    );

    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    await vi.runOnlyPendingTimersAsync();

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(useCockpit.getState().sessions[IDS.session]?.exitReason).toBe("process exited");
    expect(useCockpit.getState().toasts.at(-1)?.message).toBe(
      "The sandbox published 8080 off loopback.",
    );
  });

  it("renaming the middle session leaves its neighbours unchanged", async () => {
    seed();
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    useCockpit.getState().upsertSession(aSession({ id: THIRD, title: "Access review" }));
    renameMock.mockResolvedValue(aSession({ id: IDS.sessionB, title: "retention rules" }));

    renameSessionTo(IDS.sessionB, "retention rules");
    await vi.runOnlyPendingTimersAsync();

    expect(renameMock).toHaveBeenCalledWith(IDS.sessionB, "retention rules");
    expect(useCockpit.getState().sessions[IDS.sessionB]?.title).toBe("retention rules");
    expect(useCockpit.getState().sessions[THIRD]?.title).toBe("Access review");
    expect(useCockpit.getState().sessions[IDS.session]?.title).toBe("Evidence upload");
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      THIRD,
      IDS.sessionB,
      IDS.session,
    ]);
  });
});
```

And the last block of the same file — §10.2's verb, and the two tests contract §8 defect 9 names by title:

```ts
describe("markWorktreeReady and the §10.2 gate", () => {
  it("Mark ready PATCHes the worktree and the flag survives the next list", async () => {
    seed();
    const w = aWorktree({ ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [w]);
    readyMock.mockResolvedValue({ ...w, ready: true });

    markWorktreeReady(w);
    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(true);

    await vi.runOnlyPendingTimersAsync();
    expect(readyMock).toHaveBeenCalledWith(IDS.project, IDS.worktree, true);
    // "Survives the next list" is the second half of contract §8 defect 9: what the store
    // holds afterwards is the SERVER's object, so the next poll cannot un-set the flag.
    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(true);
  });

  it("rolls the flag back when the server refuses", async () => {
    seed();
    const w = aWorktree({ ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [w]);
    readyMock.mockRejectedValue(new ApiError("worktree_not_found", "No such worktree.", 404));

    markWorktreeReady(w);
    await vi.runOnlyPendingTimersAsync();

    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(false);
    expect(useCockpit.getState().toasts.at(-1)?.message).toBe("No such worktree.");
  });

  it("the verb is absent when the server does not report `ready`", () => {
    const close = vi.fn();
    // aWorktree() leaves `ready` undefined, which is the older-control-plane world.
    expect(worktreeReadyItems(aWorktree(), close)).toHaveLength(0);

    const shown = worktreeReadyItems(aWorktree({ ready: false }), close);
    expect(shown).toHaveLength(1);
    expect(shown[0]?.id).toBe("mark-ready");
    expect(shown[0]?.run).toBeTypeOf("function");

    const already = worktreeReadyItems(aWorktree({ ready: true }), close);
    expect(already[0]?.run).toBeUndefined();
  });
});
```

Two details that will otherwise read as mistakes. `ApiError` is imported from `../../api/client` as a **value**, not a type, while `api` is mocked from `../../api`; only the second module is mocked, so the real class is still constructible and `instanceof` still holds inside the implementation. And `vi.runOnlyPendingTimersAsync()` rather than `await Promise.resolve()` after an immediate verb: the implementation's `catch` runs in a microtask that fake timers do not themselves flush, and the `Async` variants of vitest's timer helpers drain the microtask queue between ticks, which is the whole reason they exist.

- [ ] **Step 3: Watch the suite fail, for the right reason**

```bash
cd desktop
npx vitest run src/components/develop/sessionVerbs.test.ts
```

**What you must see:** the file fails to collect, not to assert. `./sessionVerbs` does not exist, so vitest reports a collection error naming the unresolved import — the message is from Vite's resolver and reads along the lines of `Failed to resolve import "./sessionVerbs" from "src/components/develop/sessionVerbs.test.ts". Does the file exist?` — and reports **0 tests run**. That is the correct failure. The exact reporter chrome around it is approximate; the unresolved-import line and the zero-test count are what matter.

If instead you see sixteen failing assertions, the module exists already and something in the preflight was wrong. If you see a TDZ error naming one of the `*Mock` constants, `vi.hoisted` was dropped from the factory — task 20's step 2 records the same trap.

- [ ] **Step 4: Implement `sessionVerbs.ts` — the registry, the sentences and the two commit shapes**

This first half of the module has no verbs in it. It is the machinery every verb shares, and it is written separately because the two commit shapes are the part of this task that a reviewer must be able to read in one screen.

**The pending registry is module-level and not in the store, and that is a decision the contract does not make.** `SessionRow` needs one boolean — "is a deferred commit in flight for this session" — and putting it in `uiSlice` would mean editing task 17's file for a fact that lives for six seconds, never persists, and has exactly one reader. `useSyncExternalStore` over a module-level `Set` gives the same tearing-free read with no store surface, and it mirrors what task 18 already does for the heartbeat and the overlay stack. `__resetSessionVerbs()` exists for the same reason `__resetHeartbeat()` does: vitest's `restoreMocks` does not clear module state, and a pending id leaking between tests would make a later test fail in a way that names the wrong file.

Create `desktop/src/components/develop/sessionVerbs.ts`:

```ts
// The eight session lifecycle verbs, shared by the rail's right-click menu and the pane's ⋯
// menu. Contract §6.5 and §9's task 22 row. Two commit shapes, chosen per verb:
//
//   deferred  — the store changes now, the request is HELD for the Undo window, and Undo
//               cancels it. For verbs with no compensating request: Kill, Forget.
//   immediate — the store changes now, the request goes now, a refusal restores the
//               snapshot. For verbs the server can undo: Rename, Restart.
//
// Resume, Fork and Open-a-shell create rather than mutate and get no Undo at all.
import { useSyncExternalStore } from "react";
import { api } from "../../api";
import type { ApiError } from "../../api/client";
import type { CreateSessionOptions } from "../../hooks/useCreateSession";
import { replaceById } from "../../lib/immutable";
import type { MenuItem } from "../menu/ContextMenu";
import { selectHarness, selectSession, selectWorktrees } from "../../store/selectors";
import { TOAST_TTL_UNDO } from "../../store/slices/uiSlice";
import { useCockpit } from "../../store/store";
import type { Session, SessionId, Worktree, WorktreeId } from "../../store/types";

/** What a verb needs from whichever surface is rendering it. */
export interface VerbDeps {
  /** Task 18's useCreateSession() result. The one create path; this module does not add a second. */
  createSession: (worktreeId: WorktreeId, options?: CreateSessionOptions) => Promise<void>;
  /** Closes the host menu before the verb runs, so an Undo toast is not hidden behind it. */
  close: () => void;
  /** Arms the inline rename input. SessionMenu owns the input; the rail menu re-routes to it. */
  beginRename: (id: SessionId) => void;
}
```

The same file continues:

```ts
/* ── the pending registry ────────────────────────────────────────────────────────────── */

const pending = new Set<SessionId>();
const listeners = new Set<() => void>();
const timers = new Set<ReturnType<typeof setTimeout>>();

function emit(): void {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function isSessionPending(id: SessionId): boolean {
  return pending.has(id);
}

/** One boolean per row. A primitive snapshot, so useSyncExternalStore never sees a new
 *  reference for an unchanged value and a rail of forty rows re-renders none of them. */
export function useSessionPending(id: SessionId): boolean {
  return useSyncExternalStore(
    subscribe,
    () => pending.has(id),
    () => false,
  );
}

/** Test-only. vitest's restoreMocks does not clear module state, and a leaked pending id
 *  makes a later test fail in a file that did nothing wrong. Mirrors __resetHeartbeat. */
export function __resetSessionVerbs(): void {
  for (const timer of timers) clearTimeout(timer);
  timers.clear();
  pending.clear();
  emit();
}

function setPending(id: SessionId, value: boolean): void {
  if (value) pending.add(id);
  else pending.delete(id);
  emit();
}

/* ── the one place a failure becomes a sentence ──────────────────────────────────────── */

/** Two codes get a locally written sentence because the server's own wording cannot know
 *  which process is missing (§4.2). EVERY other code renders `message` verbatim — including
 *  the sixteenth, non_loopback_publish, whose sandbox is RUNNING and must never be
 *  paraphrased into "not running". */
export function sentenceFor(error: ApiError): string {
  if (error.code === "network_unavailable") {
    return "The Panacea control plane is not running, so nothing was changed.";
  }
  if (error.code === "backend_unavailable") {
    return "Docker is not running on this machine, so nothing was changed.";
  }
  return error.message;
}

function report(error: ApiError): void {
  const state = useCockpit.getState();
  state.setError({ code: error.code, message: error.message });
  // No undo argument: by the time this runs the window has closed and the button is gone.
  // An Undo offered here would be a button with nothing to take back.
  state.pushToast(sentenceFor(error));
}
```

And the two commit shapes, still the same file:

```ts
/* ── commit shape 1: deferred. The request is held for the Undo window. ──────────────── */

function runDeferred(opts: {
  sessionId: SessionId;
  message: string;
  /** The optimistic patch. Applied synchronously, before anything is sent. */
  apply: () => void;
  /** The exact inverse of `apply`. Used by Undo, and by a late failure. */
  rollback: () => void;
  commit: () => Promise<unknown>;
}): void {
  const { sessionId, message, apply, rollback, commit } = opts;
  let undone = false;
  // Set the instant the request leaves. The Undo button is already off the screen by then, but
  // the store does not drop the toast until ToastRow's own timer fires, and a click landing in
  // that gap would roll back a change the server had already accepted — leaving the cockpit
  // showing a running row over a killed PTY until task 19's poller disagreed with it.
  let sent = false;

  apply();
  setPending(sessionId, true);

  async function settle(): Promise<void> {
    if (undone) return;
    sent = true;
    try {
      await commit();
    } catch (caught) {
      // The window has closed. Put the row back, say why in plain text, and let task 19's
      // 2 s poller reconcile anything the rollback guessed wrong on its next tick.
      rollback();
      report(caught as ApiError);
    } finally {
      setPending(sessionId, false);
    }
  }

  const timer = setTimeout(() => {
    timers.delete(timer);
    void settle();
  }, TOAST_TTL_UNDO);
  timers.add(timer);

  // The toast's own TTL is the same constant, so the button disappears at the moment the
  // request is sent. One constant, two timers, and they cannot drift apart.
  useCockpit.getState().pushToast(message, () => {
    if (sent) return;
    undone = true;
    clearTimeout(timer);
    timers.delete(timer);
    setPending(sessionId, false);
    rollback();
  });
}

/* ── commit shape 2: immediate. Sent now; a refusal restores the snapshot. ───────────── */

function runImmediate(opts: {
  sessionId: SessionId;
  apply: () => void;
  rollback: () => void;
  commit: () => Promise<Session>;
}): void {
  const { sessionId, apply, rollback, commit } = opts;
  apply();
  setPending(sessionId, true);
  void (async () => {
    try {
      // The server's object replaces the optimistic one outright: it carries updatedAt and
      // an attachPath the cockpit cannot derive, and guessing either is how a pane ends up
      // attached to a socket that no longer exists.
      useCockpit.getState().upsertSession(await commit());
      useCockpit.getState().setError(null);
    } catch (caught) {
      rollback();
      report(caught as ApiError);
    } finally {
      setPending(sessionId, false);
    }
  })();
}

/** Restores one session object exactly as it was. Captured before the optimistic patch. */
function restoreSession(snapshot: Session): () => void {
  return () => {
    useCockpit.getState().upsertSession(snapshot);
  };
}

/** Forget's rollback, and it cannot be `restoreSession`. `removeSession` drops the id from
 *  `sessionIdsByWorktree`, and contract §3.2 makes `upsertSession` PREPEND an id the store no
 *  longer holds — so the forgotten row would come back at the top of its worktree rather than
 *  where the user left it, which is a silent reorder of a list the server owns. Capturing the
 *  worktree's whole list and handing it to `replaceSessions`, which rebuilds the index in the
 *  order it is given, puts it back in place. The capture happens when this is CALLED, which is
 *  while the argument list of `runDeferred` is being evaluated — before `apply()` runs. */
function restoreWorktreeSessions(worktreeId: WorktreeId): () => void {
  const state = useCockpit.getState();
  const snapshot = (state.sessionIdsByWorktree[worktreeId] ?? [])
    .map((id) => state.sessions[id])
    .filter((session): session is Session => session !== undefined);
  return () => {
    useCockpit.getState().replaceSessions(snapshot, [worktreeId]);
  };
}
```

`runImmediate` returns `void` rather than the promise, deliberately: a `MenuItem.run` is `() => void`, and a menu row that returned a floating promise would make every call site write `void`. The failure path is inside, so nothing is swallowed.

- [ ] **Step 5: Implement `sessionVerbs.ts` — the eight verbs, the rename runner and §10.2's gate**

The gates below are decisions the contract does not make, and each is written down here rather than left to the reader. **Forget is offered only on an `exited` session.** `purgeSession` is `DELETE …?purge=true` and the Go contract does not say whether the daemon kills a live process first; offering "Forget" on a running session would hide a second destructive act behind a benign word, and if it turns out the server kills-and-purges then enabling it is one predicate. **Kill is offered only while the session is alive** — `killSession` is idempotent and a second kill is a 200, so this costs nothing and removes a row that would do nothing. **Fork renders one row per other worktree on the branch** rather than a submenu: contract §6.5 gives `SessionMenu` no nested surface, task 22 creates no new overlay, and §10.3 means the candidate list is whatever git already has, which for a branch is rarely more than two or three.

This step finishes the module: the verbs, the rename runner `renameSessionTo` that step 9's input calls, §10.2's `markWorktreeReady` and the gate `worktreeReadyItems` that decides whether the verb exists at all. Nothing is left for a later step to define, which is the strongest reading of shared-shape rule 2 — every call site added in steps 9 and 11 reaches something that already exists **and already has a green test**.

**Why the fork candidates are filtered here rather than in a selector.** Shared-shape §4 sends a *derived read* to `store/selectors.ts`, and contract §3.7 gives the reason in full: "Components never inline a derivation, because an inline `.filter()` inside `useCockpit(...)` returns a new array on every store write and re-renders the component on every 2 s poll." Neither half of that reaches this line. `sessionVerbs` is not a component and subscribes to nothing — it reads one `useCockpit.getState()` snapshot at the instant the menu opens — so the filtered array is built once per right-click and never enters a render path. Adding a `selectForkTargets` would put a ninth file, owned by task 17, into this task's commit for a derivation with one caller that its own test already covers. The base read **is** §3.7's `selectWorktrees`; only the one-line exclusion of the session's own worktree is local, and `markWorktreeReady` below reads through the same selector rather than touching `worktreesByBranch` directly.

Append to `desktop/src/components/develop/sessionVerbs.ts`:

```ts
/* ── the verbs ───────────────────────────────────────────────────────────────────────── */

/** Built from a store snapshot at the moment the menu opens. A momentary menu does not
 *  relabel itself mid-hover, and the alternative — subscribing the always-mounted
 *  ContextMenu to the whole session map — re-renders it on every 2 s poll. */
export function sessionVerbs(sessionId: SessionId, deps: VerbDeps): MenuItem[] {
  const state = useCockpit.getState();
  const session = selectSession(sessionId)(state);
  if (session === undefined) {
    // A purge that raced the render. Not a crash, and not an empty menu the user cannot
    // read: one row that says what happened.
    return [
      {
        id: "gone",
        label: "This session is no longer in the list",
        disabledReason: "It was forgotten, or the poll has not caught up yet.",
      },
    ];
  }

  const harness = selectHarness(session.harness)(state);
  // §3.7's selector for the read; the exclusion is local because this module is not a
  // component and holds no subscription — see the note above this snippet.
  const siblings = selectWorktrees(session.branchId)(state).filter((w) => w.id !== session.worktreeId);
  const alive = session.status !== "exited";
  const act = (fn: () => void) => (): void => {
    deps.close();
    fn();
  };

  const items: MenuItem[] = [
    {
      id: "resume",
      label: "Resume",
      run:
        harness?.supportsResume === true
          ? act(() => {
              void (async () => {
                try {
                  const next = await api.resumeSession(session, harness);
                  useCockpit.getState().upsertSession(next);
                  useCockpit.getState().focusSession(next.id);
                } catch (caught) {
                  report(caught as ApiError);
                }
              })();
            })
          : undefined,
      disabledReason:
        harness === undefined
          ? "This session's harness is not in the catalogue."
          : harness.supportsResume
            ? undefined
            : `${harness.name} cannot resume a session.`,
    },
  ];

  if (siblings.length === 0) {
    items.push({
      id: "fork",
      label: "Fork into…",
      disabledReason:
        "This branch has only one worktree, and Phase 2 cannot create another (contract §10.3).",
    });
  } else {
    for (const worktree of siblings) {
      items.push({
        id: `fork-${worktree.id}`,
        label: `Fork into ${worktree.name}`,
        run:
          harness === undefined
            ? undefined
            : act(() => {
                void (async () => {
                  try {
                    const next = await api.forkSession(session, worktree.id, harness);
                    useCockpit.getState().upsertSession(next);
                    useCockpit.getState().focusSession(next.id);
                  } catch (caught) {
                    report(caught as ApiError);
                  }
                })();
              }),
        disabledReason:
          harness === undefined ? "This session's harness is not in the catalogue." : undefined,
      });
    }
  }

  return items.concat(tailVerbs(session, alive, deps, act));
}
```

The remaining verbs are a separate function in the same file, because one function holding all of them runs past a screen and the two commit shapes stop being visible side by side:

```ts
function tailVerbs(
  session: Session,
  alive: boolean,
  deps: VerbDeps,
  act: (fn: () => void) => () => void,
): MenuItem[] {
  const id = session.id;
  return [
    {
      id: "restart",
      // §6.5 says "Restart clean". Phase 2 has no worktree mutation endpoint (§10.3), so the
      // cockpit cannot promise a clean checkout and does not claim one. Same call.
      label: "Restart",
      run: act(() =>
        runImmediate({
          sessionId: id,
          apply: () =>
            useCockpit.getState().upsertSession({ ...session, status: "starting", exitReason: "" }),
          rollback: restoreSession(session),
          commit: () => api.restartSession(id),
        }),
      ),
    },
    { id: "rename", label: "Rename…", run: act(() => deps.beginRename(id)) },
    {
      id: "shell",
      label: "Open a shell here",
      separatorBefore: true,
      run: act(() => void deps.createSession(session.worktreeId, { harness: "shell" })),
    },
    {
      id: "kill",
      label: "Kill",
      danger: true,
      separatorBefore: true,
      run: alive
        ? act(() =>
            runDeferred({
              sessionId: id,
              message: `Killed ${session.title}.`,
              apply: () =>
                useCockpit
                  .getState()
                  .upsertSession({ ...session, status: "exited", exitReason: "killed" }),
              rollback: restoreSession(session),
              commit: () => api.killSession(id),
            }),
          )
        : undefined,
      disabledReason: alive ? undefined : "This session has already exited.",
    },
    {
      id: "forget",
      label: "Forget",
      danger: true,
      run: alive
        ? undefined
        : act(() =>
            runDeferred({
              sessionId: id,
              message: `Forgot ${session.title}.`,
              apply: () => useCockpit.getState().removeSession(id),
              // NOT restoreSession: see the comment on this helper. Forget is the one verb
              // whose rollback has to put a row back at an index, not just at an id.
              rollback: restoreWorktreeSessions(session.worktreeId),
              commit: () => api.purgeSession(id),
            }),
          ),
      disabledReason: alive
        ? "Kill it first — Forget removes the record, not the process."
        : undefined,
    },
    { id: "copy-session-id", label: "Copy session id", separatorBefore: true, run: act(() => copyId(id)) },
  ];
}

function copyId(id: SessionId): void {
  void navigator.clipboard?.writeText(id);
}

/* ── the rename runner, called by SessionMenu's inline input in step 9 ──────────────── */

/** Optimistic and immediate: the title is the user's own word, so showing it at once is not
 *  a guess. The client raises `bad_request` locally for an empty title (task 16), so a blank
 *  submit never reaches the network and still lands in the same failure path.
 *
 *  Contract §8 defect 7 is *"an edit of one of many splices the later ones"*, and the store is
 *  where that is settled: `sessions` is a `Record`, and §3.2 pins `upsertSession` to leave the
 *  position in `sessionIdsByWorktree` UNCHANGED for an id it already holds. So the rename does
 *  not need `replaceById` — there is no array here to splice — and the defect's regression test
 *  *"renaming the middle session leaves its neighbours unchanged"* asserts exactly that, in the
 *  file §8 names. `replaceById` is consumed below, where this task DOES hold an array. */
export function renameSessionTo(sessionId: SessionId, title: string): void {
  const snapshot = selectSession(sessionId)(useCockpit.getState());
  if (snapshot === undefined) return;
  runImmediate({
    sessionId,
    apply: () => useCockpit.getState().upsertSession({ ...snapshot, title: title.trim() }),
    rollback: restoreSession(snapshot),
    commit: () => api.renameSession(sessionId, title),
  });
}

/* ── contract §10.2's verb, and known defect 9 ───────────────────────────────── */

/** Optimistic and immediate: the flag is a boolean the user set, the server's answer is the
 *  same worktree object, and a refusal puts the tick back where it was. The list write goes
 *  through task 15's `replaceById` — contract §8 defect 7's helper, consumed here because
 *  "replacing one worktree in a list" is the other half of that defect's Phase 2 shape, and a
 *  hand-rolled `.map` is how the two spellings drift apart. */
export function markWorktreeReady(worktree: Worktree): void {
  const branchId = worktree.branchId;
  const before = selectWorktrees(branchId)(useCockpit.getState());
  const write = (next: Worktree): void => {
    useCockpit
      .getState()
      .setWorktrees(branchId, replaceById(selectWorktrees(branchId)(useCockpit.getState()), next));
  };

  write({ ...worktree, ready: true });
  void (async () => {
    try {
      write(await api.setWorktreeReady(worktree.projectId, worktree.id, true));
    } catch (caught) {
      useCockpit.getState().setWorktrees(branchId, before);
      report(caught as ApiError);
    }
  })();
}

/** §10.2's gate, and defect 9's contingency made checkable. `ready === undefined` is an older
 *  control plane: the verb is **not rendered at all** — not disabled, because a disabled row
 *  would advertise a feature the server does not have and §6.4 would then demand a reason the
 *  cockpit cannot honestly give. This is a function rather than JSX inside `ContextMenu`
 *  because contract §8 puts the test *"the verb is absent when the server does not report
 *  `ready`"* in `sessionVerbs.test.ts`, and task 22 writes no `ContextMenu` test. */
export function worktreeReadyItems(worktree: Worktree, close: () => void): MenuItem[] {
  if (worktree.ready === undefined) return [];
  return [
    {
      id: "mark-ready",
      label: worktree.ready ? "Ready to merge" : "Mark ready",
      separatorBefore: true,
      run: worktree.ready
        ? undefined
        : () => {
            close();
            markWorktreeReady(worktree);
          },
      disabledReason: worktree.ready
        ? "Already marked ready. Phase 2 has no un-mark; the merge queue in Phase 3 owns that."
        : undefined,
    },
  ];
}
```

`markWorktreeReady` writes forward with `replaceById`, which touches one element and keeps every later one, but rolls back to the **whole array** it captured rather than to the single worktree: `setWorktrees` replaces the branch's list wholesale, and a rollback that rebuilt the list from the current one would silently drop anything a poll added in between. Forward by id, backward by snapshot — the two directions have different failure modes and the same helper cannot serve both.

- [ ] **Step 6: Watch the verb suite pass**

```bash
cd desktop
npx vitest run src/components/develop/sessionVerbs.test.ts
npm run typecheck
```

**What you must see:** sixteen passing tests across four `describe` blocks — five gates, five deferred-commit cases, three immediate-commit cases and three in `markWorktreeReady and the §10.2 gate`. If your count differs from sixteen, a test was dropped in transcription. The typecheck must be silent. `tsc --noEmit` prints nothing at all on success — not "0 errors", not a summary line — so an empty output is the pass, and any output at all is a failure carrying `file(line,col): error TSxxxx: message`.

Two failures worth naming in advance because they look like product bugs and are not. If `a failure AFTER the window closes rolls back, in place, and toasts with no Undo` fails on `expect(last?.undo).toBeUndefined()` and the toast it found is the *Undo* toast rather than the failure toast, the deferred toast was not dismissed — check that the test advances the full `TOAST_TTL_UNDO`, because `toasts.at(-1)` reads the newest and the store does not remove an expired toast by itself (task 18's `ToastRow` does, and this suite renders nothing). If `Undo inside the window cancels the request and restores the session` fails with `killMock` called once, the `undone` flag is being read inside `settle` after the closure captured a stale copy — it must be a `let` in `runDeferred`'s scope, not a parameter.

- [ ] **Step 7: Write the failing tests for `SessionMenu`**

`SessionMenu`'s own job is small and entirely about seams: resolve a `DOMRect` from an id, render the verb list, and own the one surface a verb cannot supply for itself — the rename input. Everything else is `sessionVerbs`, already green.

**The anchor chain is the part that must be tested rather than assumed.** Contract §6.5 gives `SessionMenu` an `anchor: DOMRect | null` prop; `uiSlice.sessionMenuFor` carries only a `SessionId`; task 20 bridged them with a `data-session-menu-anchor` attribute on the `⋯` button because shared-shape §1 does not list this task as an editor of `SessionPaneHeader.tsx`. That works when a pane is on screen. It does not when the menu was opened from a **rail row** for a session on a branch the Develop lens is not showing, and in that case a null anchor would mean a `Popover` that renders nothing and a menu the user opened and never saw. So the chain is: the pane's `⋯` button, then the rail row for the same session, then a zero-sized rect at the viewport centre — which `Popover`'s clamp then resolves into a menu the user can read.

Create `desktop/src/components/develop/SessionMenu.test.tsx`:

```tsx
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { IDS, aHarness, aSession, aWorktree } from "../../test/fixtures";
import { __resetOverlays } from "../../hooks/useDismissable";
import { resetCockpit, useCockpit } from "../../store/store";

const { killMock, renameMock } = vi.hoisted(() => ({ killMock: vi.fn(), renameMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { killSession: killMock, renameSession: renameMock } }));

import { SessionMenu, anchorRectFor } from "./SessionMenu";
import { __resetSessionVerbs } from "./sessionVerbs";

const RECT = new DOMRect(40, 60, 0, 0);

beforeEach(() => {
  resetCockpit();
  __resetSessionVerbs();
  // Task 18's overlay stack is module state and survives restoreMocks, exactly like the
  // pending registry. A leaked entry makes the NEXT test's Escape close the wrong overlay.
  __resetOverlays();
  useCockpit.getState().setHarnesses([aHarness({ id: "claude-code", name: "Claude Code" })]);
  useCockpit.getState().setWorktrees(IDS.branch, [aWorktree()]);
  useCockpit.getState().upsertSession(aSession({ status: "running" }));
});

afterEach(() => {
  // task 14's setup.ts already calls RTL's cleanup() in a global afterEach; this clears the
  // elements the anchorRectFor tests append by hand, which cleanup() does not own.
  document.body.innerHTML = "";
});
```

The same test file continues:

```tsx
describe("anchorRectFor", () => {
  it("prefers the pane's ⋯ button", () => {
    const button = document.createElement("button");
    button.setAttribute("data-session-menu-anchor", IDS.session);
    button.getBoundingClientRect = () => new DOMRect(10, 20, 24, 24);
    document.body.append(button);
    expect(anchorRectFor(IDS.session).x).toBe(10);
  });

  it("falls back to the rail row when no pane is on screen", () => {
    const row = document.createElement("div");
    row.setAttribute("data-kind", "session");
    row.setAttribute("data-id", IDS.session);
    row.getBoundingClientRect = () => new DOMRect(200, 300, 180, 24);
    document.body.append(row);
    expect(anchorRectFor(IDS.session).x).toBe(200);
  });

  it("falls back to a centred point when neither exists, rather than returning null", () => {
    const rect = anchorRectFor(IDS.session);
    expect(rect.width).toBe(0);
    expect(rect.x).toBe(Math.round(window.innerWidth / 2));
  });
});

describe("SessionMenu", () => {
  it("renders the lifecycle verbs and closes on a verb", () => {
    const onClose = vi.fn();
    render(<SessionMenu sessionId={IDS.session} anchor={RECT} onClose={onClose} />);

    // jest-dom is NOT installed (shared-shape §3C, and task 14 says so): read the DOM property.
    const kill = screen.getByRole("menuitem", { name: "Kill" }) as HTMLButtonElement;
    const forget = screen.getByRole("menuitem", { name: "Forget" }) as HTMLButtonElement;
    expect(kill.disabled).toBe(false);
    expect(forget.disabled).toBe(true);

    fireEvent.click(kill);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Rename… swaps the list for an input seeded with the current title, and submits it", () => {
    renameMock.mockResolvedValue(aSession({ title: "retention rules" }));
    render(<SessionMenu sessionId={IDS.session} anchor={RECT} onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
    const input = screen.getByRole("textbox", { name: "Session title" }) as HTMLInputElement;
    expect(input.value).toBe("Evidence upload");
    expect(screen.queryByRole("menuitem", { name: "Kill" })).toBeNull();

    fireEvent.change(input, { target: { value: "retention rules" } });
    fireEvent.submit(input.closest("form") as HTMLFormElement);
    expect(renameMock).toHaveBeenCalledWith(IDS.session, "retention rules");
    expect(useCockpit.getState().sessions[IDS.session]?.title).toBe("retention rules");
  });

  it("Escape in the rename input returns to the verb list and closes nothing else", () => {
    const onClose = vi.fn();
    render(<SessionMenu sessionId={IDS.session} anchor={RECT} onClose={onClose} />);
    fireEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: "Session title" }), { key: "Escape" });

    expect(screen.queryByRole("menuitem", { name: "Kill" })).not.toBeNull();
    expect(renameMock).not.toHaveBeenCalled();
    // The assertion the first draft of this plan was missing, and the one that actually pins
    // the behaviour: the menu is still open. Without the rename's own useDismissable entry
    // sitting ABOVE the Popover's on task 18's stack, this Escape closes the whole menu.
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing when the anchor is null", () => {
    const { container } = render(
      <SessionMenu sessionId={IDS.session} anchor={null} onClose={vi.fn()} />,
    );
    expect(container.childNodes.length).toBe(0);
  });
});
```

The Escape test asserts three things: the verb list is back, `renameMock` was not called, and **`onClose` was not called either**. Escape inside the input is one step back rather than two, and a key that closed the whole menu would make a mistyped character cost the user the menu as well.

**The mechanism is not `stopPropagation` on the input, and the first draft of this plan was wrong about that.** Task 18's `useDismissable` listens for Escape on `document` in the **capture** phase — `document.addEventListener("keydown", onKeyDown, true)` — and its handler calls `event.stopPropagation()` itself, so it runs before React's root-container listener ever sees the event — the input's own `onKeyDown` would not fire at all, and the menu would close outright. What makes one step back real is task 18's overlay **stack**: while `renaming` is true, `SessionMenu` registers a **second** `useDismissable`, which is pushed above the `Popover`'s entry, so the Popover's handler sees `isTop() === false` and returns, and the rename's handler — registered later on the same target, so `stopPropagation` from a sibling listener cannot suppress it — closes the input and nothing else. That is §6.9's "close the topmost overlay" ordering used as intended rather than worked around. Step 9 writes it, and the `onClose` assertion above is what fails if it is dropped.

- [ ] **Step 8: Watch the `SessionMenu` suite fail**

```bash
cd desktop
npx vitest run src/components/develop/SessionMenu.test.tsx
```

**What you must see:** a collection error naming `./SessionMenu` as unresolved, and zero tests run — the same shape as step 3. If you instead see seven failures with `Element type is invalid: expected a string … but got: undefined`, the file exists but exports nothing named `SessionMenu`, which is a different and easier problem.

- [ ] **Step 9: Implement `SessionMenu`**

Every decision in this step is one the contract does not make.

**The rename surface is an inline input inside the menu, not a dialog and not `window.prompt`.** `window.prompt` is the obvious one-liner and it is wrong twice over: Tauri's webview does not implement it on every platform, so the verb would silently do nothing on one of the three, and a modal prompt blocks the render loop that task 19's terminals are streaming into. A dialog would be a new overlay, and contract §6.7's table fixes the overlays by file while §6.8 gives focus ownership while one is open to `CommandPalette` and `Popover` alone — a new overlay would have to re-derive that rule for itself. An input rendered in place of the verb list reuses the `Popover` that is already open, already owns focus, and already restores it on close.

**`SessionMenu` renders its own rows rather than importing a shared list component from `ContextMenu.tsx`.** `ContextMenu.tsx` imports `SessionMenu` in step 11 to mount it, so a `MenuList` exported from `ContextMenu.tsx` and imported here would be a genuine ES module cycle. Vite tolerates cycles; they fail later, in a bundler or a test runner, in a way that names neither file. The block of markup below, repeated with the same class names, is the cheaper of the two, and `menu.css` keeps both honest.

**Escape inside the input is one step back, and the mechanism is task 18's overlay stack rather than `stopPropagation`.** `useDismissable` listens on `document` in the capture phase and stops propagation itself, so a React `onKeyDown` on the input never runs — the first draft of this plan claimed otherwise and was wrong. Registering a second `useDismissable` while `renaming` is true puts the rename above the `Popover` on task 18's stack, which is what `isTop()` reads: the Popover's handler returns, the rename's closes the input, and §6.9's "close the topmost overlay" does the work with no new listener and no edit to task 18's file. The same entry gives the input outside-click for free: a pointerdown anywhere but the form abandons the rename and leaves the menu open, which is the same one step back by a different gesture.

That second stack entry is **not** a new overlay in §6.7's sense, and the distinction matters because the paragraph above rejects a dialog on exactly that ground. A new overlay would take a row in §6.7's table, with its own file, and §6.8 gives focus capture and return to `CommandPalette` and `Popover` alone, so it would have to re-derive that rule for itself. This is instead one nested surface inside `SessionMenu`'s existing popover, which returns focus through the popover it never left. It adds a row to `useDismissable`'s runtime stack, which is what §6.9's ordering is defined over, and no row to §6.7's table.

`renameSessionTo` is **not** written here. Step 5 wrote it and step 6 proved it green, which is the strongest available reading of shared-shape rule 2 — not merely "defined in the step that adds the call", but defined, tested and passing before the first call site exists.

Create `desktop/src/components/develop/SessionMenu.tsx`:

```tsx
// Contract §6.5: the ⋯ menu. Props are exactly { sessionId, anchor, onClose } — the anchor is
// resolved by the caller, because uiSlice.sessionMenuFor carries only an id and task 20's
// header exposes the button through a data attribute rather than a ref.
import { Fragment, useState } from "react";
import type { FormEvent, JSX } from "react";
import { Popover } from "../common/Popover";
import { useDismissable } from "../../hooks/useDismissable";
import { useCreateSession } from "../../hooks/useCreateSession";
import { selectSession } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";
import { renameSessionTo, sessionVerbs } from "./sessionVerbs";
import "../menu/menu.css";

/** The pane's ⋯ button, then the rail row for the same session, then a point near the top
 *  centre of the viewport. Never null: a null anchor closes `Popover`, and a menu the user
 *  opened from a rail row for an off-screen session would then open and never appear. */
export function anchorRectFor(sessionId: SessionId): DOMRect {
  const pane = document.querySelector(`[data-session-menu-anchor="${sessionId}"]`);
  if (pane !== null) return pane.getBoundingClientRect();
  const row = document.querySelector(`[data-kind="session"][data-id="${sessionId}"]`);
  if (row !== null) return row.getBoundingClientRect();
  return new DOMRect(Math.round(window.innerWidth / 2), Math.round(window.innerHeight / 3), 0, 0);
}
```

The same file continues:

```tsx
export function SessionMenu(props: {
  sessionId: SessionId;
  anchor: DOMRect | null;
  onClose: () => void;
}): JSX.Element {
  const { sessionId, anchor, onClose } = props;
  const title = useCockpit((s) => selectSession(sessionId)(s)?.title ?? "");
  const createSession = useCreateSession();
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState("");
  // Task 18's overlay stack, not a second Escape listener. Pushed ABOVE the Popover's entry
  // while renaming, so §6.9's "close the topmost overlay" returns to the verb list rather
  // than closing the menu. Unconditional, above the early return: hook order never varies.
  const renameRef = useDismissable<HTMLFormElement>(renaming, () => setRenaming(false));

  if (anchor === null) return <></>;

  const items = sessionVerbs(sessionId, {
    createSession,
    close: onClose,
    beginRename: () => {
      setDraft(title);
      setRenaming(true);
    },
  });

  const submit = (event: FormEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setRenaming(false);
    renameSessionTo(sessionId, draft);
    onClose();
  };

  const close = (): void => {
    setRenaming(false);
    onClose();
  };

  return (
    <Popover anchor={anchor} onClose={close}>
      {renaming ? (
        <form ref={renameRef} className="menu menu-rename" onSubmit={submit}>
          <label className="menu-rename-label" htmlFor={`rename-${sessionId}`}>
            Session title
          </label>
          <input
            id={`rename-${sessionId}`}
            className="menu-rename-input"
            value={draft}
            autoFocus
            onChange={(event) => setDraft(event.target.value)}
          />
        </form>
      ) : (
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
      )}
    </Popover>
  );
}
```

`beginRename` does not call `deps.close`, and that is why it is a dep rather than a verb that runs itself: every other verb closes the menu on its way out, and Rename is the one that has to keep it open to render the input. The `act` wrapper in `sessionVerbs` closes first and then runs — so the rail's Rename, which re-routes to this menu, closes the context menu and opens this one, which is the behaviour the user expects from a menu that changes shape.

Append the rename block to `desktop/src/components/menu/menu.css` — the form, its label and its input. Shared-shape §2 sanctions this file for task 22 by name; the block goes at the foot of the file under a header, and no rule task 18 wrote is reflowed:

```css
/* ── task 22: the inline rename input ──────────────────────────────────────────────── */
.menu-rename {
  gap: 4px;
  padding: 6px;
}
.menu-rename-label {
  color: var(--ink-3);
  font-size: 11px;
}
.menu-rename-input {
  background: var(--inset);
  border: 0;
  border-radius: var(--r-ctl);
  box-shadow: var(--e-hair);
  color: var(--ink);
  font: inherit;
  padding: 5px 8px;
}
```

**Shared-shape §3B is binding on the rules above and all four of its clauses hold.** The `@import "../../styles/tokens.css";` clause is already satisfied at the head of the file — task 18 wrote it as `menu.css`'s first line and this block appends at the foot, so the import stays first. There is **no colour function and no hex**: every colour is a `var(--token)`, and each token used — `--inset`, `--r-ctl`, `--e-hair`, `--ink` and `--ink-3` — already exists in task 14's set, so nothing has to be added there. There is **no gradient**. And there is no `transition` at all, so `transition: all` cannot arise. Elevation is the 1px ring `--e-hair` already carries, not a flat border, which is why `border: 0`.

- [ ] **Step 10: Watch the `SessionMenu` suite pass**

```bash
cd desktop
npx vitest run src/components/develop/SessionMenu.test.tsx
npm run typecheck
```

**What you must see:** seven passing tests — three for `anchorRectFor` and four for the component. The typecheck prints nothing.

If `Rename… swaps the list for an input seeded with the current title, and submits it` fails on `expect(input.value).toBe("Evidence upload")` with an empty string, `beginRename` captured `title` from a render that ran before the store had the session; the subscription `useCockpit((s) => selectSession(sessionId)(s)?.title ?? "")` is what makes it current, and reading it through `getState()` inside the callback instead would reintroduce the bug. If `renders nothing when the anchor is null` fails, `Popover` is being given a non-null rect somewhere — this component must return before calling it, because §6.7 makes a null anchor the closed state and a second source of truth for "is it open" is how a popover ends up at `0,0` behind the rail.

- [ ] **Step 11: Wire `ContextMenu.tsx` — the verbs, `Mark ready`, and the `SessionMenu` mount**

This is the shared-shape rule 2 step: everything it calls already exists and its suites are already green, so the tree compiles at every point in the edit. **Seven edits to task 18's file** — two at the insertion points it reserved, four it did not anticipate, and one to its own header comment so the file stops advertising points that are now filled.

**The `SessionMenu` mount is the change task 18 did not anticipate, and it needs its reason on the record.** `SessionMenu` is a presentational component with three props; something has to hold `uiSlice.sessionMenuFor` and turn it into those props. The natural home is `AppShell.tsx`, beside `<ContextMenu/>` and `<Toasts/>` — but shared-shape §1 gives that file's edit row to task 23, not to this task, and two tasks editing one file is the collision the document exists to prevent. `ContextMenu.tsx` is the only file this task may edit that `AppShell` already mounts exactly once, and the two menus are the same concern. **Amends contract §6.7**, which lists the overlays by file and does not say who mounts which.

One consequence: `ContextMenu` stops early-returning `null`. It now returns a fragment whose two halves open independently, because the ⋯ menu must be able to open while no right-click menu is up — which is the normal case.

Add to the imports at the top of `desktop/src/components/menu/ContextMenu.tsx`:

```tsx
import { SessionMenu, anchorRectFor } from "../develop/SessionMenu";
import { sessionVerbs, worktreeReadyItems } from "../develop/sessionVerbs";
```

`markWorktreeReady` is **not** imported here: `worktreeReadyItems` is the only thing this file
needs, and it calls the runner itself. Two imports where a reviewer might expect three is the
point of putting the gate and the runner in one module.

And replace the file's own header lines — task 18 wrote a `TASK 22 INSERTION POINTS:` paragraph
naming the two points and the rules the consumer had to follow. Both points are now filled, so
the paragraph is stale documentation pointing at code that no longer exists; the replacement says
what the file does today:

```tsx
// The session verbs and the worktree ready mark come from ../develop/sessionVerbs (task 22).
// MenuItem is declared here and imported back by that module as a TYPE ONLY, so the cycle is
// erased at compile time and the value edge runs one way: ContextMenu -> SessionMenu ->
// sessionVerbs.
```

`sessionVerbs.ts` imports `MenuItem` back from this file, so the two modules form a cycle — but that edge is `import type`, erased before the bundler sees it, so no runtime cycle exists. The value edge runs one way only: `ContextMenu → SessionMenu → sessionVerbs`.

Add three subscriptions beside the existing ones in the component body, and **memoise the
anchor beside task 18's own**:

```tsx
  const sessionMenuFor = useCockpit((s) => s.sessionMenuFor);
  const closeSessionMenu = useCockpit((s) => s.closeSessionMenu);
  const openSessionMenu = useCockpit((s) => s.openSessionMenu);
  // useMemo is NOT an optimisation here, it is a correctness fix. Popover's positioning
  // effect depends on [anchor, ref] and ends with el.focus(); a fresh DOMRect on every
  // render re-fires it and pulls focus out of the rename input mid-keystroke. Task 18
  // memoised its own `const anchor = useMemo(...)` for exactly this reason, and `useMemo`
  // is already imported at the head of this file.
  const sessionAnchor = useMemo(
    () => (sessionMenuFor === null ? null : anchorRectFor(sessionMenuFor)),
    [sessionMenuFor],
  );
```

Replace the early return `if (target === null) return null;` with nothing — delete the line — and make `items` tolerate a null target. The block keeps its position, below the four item functions, and the three no-argument calls are task 18's signatures unchanged: `projectItems()`, `branchItems()` and `worktreeItems()` close over the hook results and take nothing, and only `sessionItems` takes an id.

```tsx
  const items =
    target === null
      ? []
      : target.kind === "project"
        ? projectItems()
        : target.kind === "branch"
          ? branchItems()
          : target.kind === "worktree"
            ? worktreeItems()
            : sessionItems(target.id);
```

Replace the **body** of `sessionItems` — the whole reserved comment block and the single `copy-session-id` row it guarded — with the call it named. The declaration line and its closing `};` are task 18's and stay exactly as written, which is not a cosmetic choice: task 18's reserving comment nominates the guard `grep -cE '^[[:space:]]+return sessionVerbs\('`, and an arrow function with an expression body would make that guard return 0 against correct code. Keep the block body, and task 18's own guard is the one step 13 runs.

```tsx
  const sessionItems = (id: string): MenuItem[] => {
    return sessionVerbs(id, {
      createSession,
      close: closeContextMenu,
      // The rail row has no room for a text field, so Rename re-routes to the ⋯ menu, which
      // owns the only rename surface in the app. One surface, two ways in.
      beginRename: (sessionId) => openSessionMenu(sessionId),
    });
  };
```

Replace the **worktree** insertion point — the whole reserved comment block — with a spread of
step 5's gate, still inside `worktreeItems`' returned array:

```tsx
      ...worktreeReadyItems(worktree, closeContextMenu),
```

Task 18's comment suggested the guard `grep -cE '^[[:space:]]+id: "mark-ready"'` against **this**
file, which assumed the row's object literal would be written here. It is not, and the reason is
contract §8: defect 9's second regression test is *"the verb is absent when the server does not
report `ready`"* and §8 puts it in `sessionVerbs.test.ts`, where this task's only two test files
are. A gate expressed as JSX in `ContextMenu.tsx` could not be reached from there. So the gate is
`worktreeReadyItems()`, the row is built and tested beside the runner that fills it, and step 13
runs an anchored guard on each half — the spread here, the row there.

And replace the `return (` block at the foot of the component:

```tsx
  return (
    <>
      {sessionMenuFor === null ? null : (
        <SessionMenu
          sessionId={sessionMenuFor}
          anchor={sessionAnchor}
          onClose={closeSessionMenu}
        />
      )}
      {target === null ? null : (
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
      )}
    </>
  );
```

The component's return type narrows from `JSX.Element | null` to `JSX.Element`. That is a widening of what it can render, not of its contract, and no caller passes it anywhere a `null` was required — `AppShell` mounts it as a bare element.

`anchorRectFor` reads the DOM during render. That is a layout read, not a write, and the `useMemo` is what makes "once, when `sessionMenuFor` changes from null" true rather than aspirational — the same moment a `ref` callback would have fired. Two things go wrong without it, and the second is the serious one. `ContextMenu` re-renders on every store write it subscribes to, so the rect would be re-measured on a 2 s poll; and because the value is a **new `DOMRect` object each time**, `Popover`'s `useLayoutEffect([anchor, ref])` would re-run and call `el.focus()`, taking focus off the rename input while the user was typing into it — §6.8's "overlays capture focus on open" turned into "overlays capture focus forever". Doing the read in an effect instead would render the popover at the fallback position for one frame and then jump, which is exactly the layout shift `docs/AGENTIC-UX.md` §10 rule 7 bans.

- [ ] **Step 12: Wire the rail rows, and the `⋯` button that opens the menu**

Small edits to two of task 18's rows and to task 20's pane header. The first two exist because a verb without a visible effect is not a verb; the third because a menu whose own button cannot close it is a menu the user has to click elsewhere to escape.

**`SessionRow` gets the pending affordance at the point task 18 reserved for it.** During a deferred commit the row already shows the optimistic state — a killed session reads `exited` — but nothing says the request has not been sent yet, and a user who clicks Kill twice in six seconds should be able to see why the second click changed nothing. `data-pending` is the attribute task 18's comment named, and the guard in step 13 is anchored on it.

**The opacity is an inline style rather than a rule in `rail.css`, and that is a deliberate trade.** `rail.css` is task 18's and shared-shape §2 sanctions only `menu/` CSS for this task; one inline `opacity` avoids a fourth file of someone else's. It is a property `docs/DESIGN-SYSTEM.md` permits (`transform`/`opacity` only) and it carries no transition, so the change is instant — which is correct here, because the fade is a state readout and not a causal animation.

In `desktop/src/components/rail/SessionRow.tsx`, add the import:

```tsx
import { useSessionPending } from "../develop/sessionVerbs";
```

read the flag beside the other hooks:

```tsx
  const pending = useSessionPending(sessionId);
```

and replace task 18's reserved `{/* TASK 22 NOTE … */}` comment block with the two attributes, which go on the row element itself — move them up into the `<div role="treeitem" …>` opening tag, directly under `data-id`:

```tsx
      data-pending={pending ? "true" : undefined}
      style={pending ? { opacity: 0.55 } : undefined}
```

`data-pending={undefined}` omits the attribute entirely rather than rendering `data-pending="false"`, which keeps a CSS selector written later honest and keeps the DOM snapshot in task 24's defect suite unchanged for the ordinary case.

**`WorktreeRow` gets the ready mark.** `Mark ready` writes a flag the rail is the only place to see, and §10.2's whole point is that the flag survives a restart — a user who marks a worktree ready, quits, and comes back must be able to tell that they did. The mark is a glyph with a `title`, not a count: `docs/AGENTIC-UX.md` §10 rule 10 bans counts outside the queue and Phase 2 has no queue, and a tick is not a count.

In `desktop/src/components/rail/WorktreeRow.tsx`, insert one element between the name span and the `＋` button:

```tsx
        <span className="rail-name">{worktree.name}</span>
        {worktree.ready === true ? (
          <span className="rail-ready" title="Marked ready to merge" aria-label="ready">
            ✓
          </span>
        ) : null}
```

`.rail-ready` has no rule in `rail.css` and is not given one here: an unstyled `<span>` inherits the row's type and colour, which is the correct rendering for a mark that must not compete with the status dot. The class name exists so a later task can style it without re-finding the element. If a reviewer wants it dimmed, one rule in task 18's `rail.css` does it and this task deliberately did not take that file. Task 18 reserved no point for this edit, so step 13 carries an anchored guard for it that the earlier task did not write.

**The `⋯` button becomes a real overlay trigger, and this is a gap between two shipped tasks rather than a preference.** Task 18's dismissal rule says a control that opens an overlay carries `data-overlay-trigger`, and names *task 22's `⋯` button* as one of the two that need it; `useDismissable` then exempts that control from its capture-phase `pointerdown` — but **only when the control's `aria-expanded` reads `"true"`**, because an unscoped exemption would let one overlay's trigger block another's dismissal. Task 20 shipped the button with `data-session-menu-anchor` and neither the attribute nor the state, and `openSessionMenu` sets an id rather than toggling one. Any one of those missing leaves the same symptom: pointerdown closes the menu, the click re-opens it, and the button can never close what it opened. Shared-shape §1 does not list `SessionPaneHeader.tsx` for this task, so this is the second unreserved insertion — declared in the Interfaces block, guarded in step 13, and reported to the controller so task 20's own plan can carry the same lines if it is ever re-run.

In `desktop/src/components/develop/SessionPaneHeader.tsx`, read the menu's state beside task 20's existing `openSessionMenu`:

```tsx
  const closeSessionMenu = useCockpit((s) => s.closeSessionMenu);
  const sessionMenuFor = useCockpit((s) => s.sessionMenuFor);
```

then give the `⋯` button the attribute, the state and the toggle. The first three lines below are new and go under `data-session-menu-anchor`; the fourth **replaces** task 20's one-line `onClick={() => openSessionMenu(session.id)}`. Nothing else in the tag changes, and nothing is deleted:

```tsx
        data-overlay-trigger=""
        aria-haspopup="menu"
        aria-expanded={sessionMenuFor === session.id}
        onClick={() =>
          sessionMenuFor === session.id ? closeSessionMenu() : openSessionMenu(session.id)
        }
```

`aria-haspopup="menu"` comes with them because the same fact has to reach a screen reader, and task 18's `MachineChip` pairs `aria-haspopup` with `aria-expanded` on its own trigger. Task 20's test asserts `data-session-menu-anchor` on this button and still passes: nothing is removed.

- [ ] **Step 13: Run everything, and prove the wiring rather than the comments**

```bash
cd desktop
npm test
npm run typecheck
```

**What you must see:** every test file in the repository passing, including tasks 14 to 21's, and a silent typecheck. This task adds **twenty-three tests across two files** — sixteen in `develop/sessionVerbs.test.ts` (five gates, five deferred, three immediate, three on §10.2's verb and its gate) and seven in `develop/SessionMenu.test.tsx`. If a test in another file starts failing only when the whole suite runs, suspect module-level state: `__resetSessionVerbs()` exists for exactly that, and a `beforeEach` that forgot it leaves a pending id and a live `setTimeout` behind. The same is true of task 18's overlay stack, which is why `SessionMenu.test.tsx` calls `__resetOverlays()`.

Then the guards, and **every number beside them was measured rather than reasoned about**. Shared-shape §3A is explicit that a guard whose count was assumed is worse than no guard, and the first draft of this plan shipped two that fail against correct code. So I rebuilt the post-edit files in a scratch directory — task 18's `ContextMenu.tsx`, `SessionRow.tsx` and `WorktreeRow.tsx` and task 20's `SessionPaneHeader.tsx` with this task's step-11 and step-12 edits applied, alongside the two modules steps 4, 5 and 9 create — and ran every guard below against them. These are that run's output:

```bash
cd desktop
grep -cE '^[[:space:]]+return sessionVerbs\(' src/components/menu/ContextMenu.tsx            # 1
grep -cE '^[[:space:]]+\.\.\.worktreeReadyItems\(' src/components/menu/ContextMenu.tsx       # 1
grep -cE '^[[:space:]]+<SessionMenu$' src/components/menu/ContextMenu.tsx                    # 1
grep -cE '^[[:space:]]+id: "mark-ready",' src/components/develop/sessionVerbs.ts             # 1
grep -cE '^[[:space:]]+data-pending=' src/components/rail/SessionRow.tsx                     # 1
grep -cE '^[[:space:]]+const pending = useSessionPending' src/components/rail/SessionRow.tsx # 1
grep -cE '^[[:space:]]+<span className="rail-ready"' src/components/rail/WorktreeRow.tsx     # 1
grep -cE '^[[:space:]]+data-overlay-trigger=""' src/components/develop/SessionPaneHeader.tsx # 1
grep -c 'TASK 22 INSERTION POINT' src/components/menu/ContextMenu.tsx                        # 0
grep -cE 'window\.prompt\(' src/components/develop/SessionMenu.tsx                           # 0
grep -cE 'code === "(sandbox_not_running|non_loopback_publish)"' src/components/develop/sessionVerbs.ts  # 0
```

The guards expecting 1 are the wiring guards, and they are the checks that catch a verb existing only in its own tests. Each is anchored on a line whose first non-space character begins the code, so a `//` or `{/* */}` comment cannot satisfy it: measured, each matches exactly one line of the step-11 and step-12 snippets and zero lines of task 18's reserving comments. The first is task 18's own suggested spelling, honoured rather than replaced — which is why step 11 keeps `sessionItems`' block body. Two moved since the first draft, for the reason on the record in step 11: the `mark-ready` row is counted in `sessionVerbs.ts`, where the gate lives, and the spread that wires it is counted in `ContextMenu.tsx`. The last two of them have no reserved point behind them at all — `WorktreeRow.tsx` and task 20's `SessionPaneHeader.tsx` are the unreserved insertions, so this task writes their guards itself.

**The `TASK 22 INSERTION POINT` guard is the one that catches the failure mode this whole document was written against, and the first draft got its number wrong.** The string appears **three** times in task 18's file, not two: once in its file-header paragraph and once in each of the two reserved blocks. I measured 3 before the edit and 0 after. Step 11 removes all three — the two blocks because the calls that replace them make them false, and the header because a file still advertising unfilled insertion points after they are filled is documentation pointing at code that does not exist. If any survive, a verb set was added **beside** a reservation instead of replacing it, and the menu renders both the placeholder and the real thing — or, worse, the placeholder only, with the real list computed and thrown away. Group C shipped exactly that defect on the Go side and it cost two fix rounds.

That guard is also the one place where the anchoring rule runs backwards: its whole purpose is to count a comment string down to zero, so it is deliberately unanchored, and the reserving comments are exactly what it must stop finding.

The two zero guards after it cost nothing and catch a real mistake. `window.prompt(` is step 9's decision made checkable. The last is the sixteen-code rule made checkable, and **its spelling is the whole point**: the obvious form, `grep -cE 'sandbox_not_running|non_loopback_publish' sessionVerbs.ts`, returns **1** — I measured that too — because the doc comment above `sentenceFor` names `non_loopback_publish` in order to say it must never be paraphrased. A guard that fails on correct code sends the implementer hunting a defect that is not there, which is §3A's exact complaint. Counting the *comparison* `code === "…"` cannot be satisfied by prose, because a sentence contains no strict-equality operator. `sentenceFor` special-cases exactly two codes — `network_unavailable` and `backend_unavailable`, the two the server cannot word for itself — and every other code, `non_loopback_publish` above all, reaches the user as the server's own `message`. A file that names a code in order to rewrite it has started paraphrasing, and `non_loopback_publish` is the one where paraphrase becomes a lie: the sandbox is running.

- [ ] **Step 14: Commit**

```bash
cd desktop
npm test
npm run typecheck
cd ..
git add \
  desktop/src/components/develop/sessionVerbs.ts \
  desktop/src/components/develop/sessionVerbs.test.ts \
  desktop/src/components/develop/SessionMenu.tsx \
  desktop/src/components/develop/SessionMenu.test.tsx \
  desktop/src/components/menu/ContextMenu.tsx \
  desktop/src/components/menu/menu.css \
  desktop/src/components/rail/SessionRow.tsx \
  desktop/src/components/rail/WorktreeRow.tsx \
  desktop/src/components/develop/SessionPaneHeader.tsx
git commit
```

Staged by name, never `git add -A`, `-u` or `.` — shared-shape §6. The list is the four files this task creates, the four of task 18's it modifies, and task 20's pane header. If `git status` shows anything that is not on it, find out what it is before committing; a stray file here is usually a test artefact or a half-finished edit to a file this task was not granted.

Commit message:

```
feat(cockpit): session lifecycle verbs with an Undo window

Adds the verbs of contract §9's task 22 row — Resume, Fork into…,
Restart, Rename…, Kill, Forget, Open a shell here, Mark ready — in one
module shared by the rail's right-click menu and the pane's ⋯ menu.

Two commit shapes, chosen per verb rather than one applied to all:

  deferred  — Kill and Forget. The store changes now; the request is HELD
              for the Undo window and Undo cancels it outright, because
              neither verb has a compensating request the cockpit could
              send afterwards.
  immediate — Rename and Restart. Sent at once, optimistic in the store,
              and a refusal restores the snapshot.

A failure that lands after the window has closed rolls the optimistic
patch back and reports the server's own sentence as a plain toast with no
Undo button, since there is nothing left to take back; the 2 s poller
reconciles anything the rollback guessed wrong.

Mark ready renders only when the server reports `ready` on the worktree
(contract §10.2, known defect 9). The gate is a runtime check on the
value, so a cockpit meeting an older control plane hides the verb by
itself rather than offering one that cannot work.

Amends §6.7: <SessionMenu/> is mounted by ContextMenu.tsx. Shared-shape
§1 gives AppShell.tsx's edit row to task 23, and ContextMenu is the only
file this task may edit that AppShell already mounts once.

Departs from §6.5 on one label: "Restart", not "Restart clean". Phase 2
has no worktree mutation endpoint (§10.3) and cannot promise a clean
checkout.

Adds an inline rename input rather than window.prompt, which Tauri's
webview does not implement on every platform, or a new overlay.

Gives task 20's ⋯ button data-overlay-trigger, the aria-expanded that
useDismissable reads with it, and a toggling onClick. Task 18's dismissal
rule names this button and task 20 shipped it with none of the three, so
the control that opened the menu could not close it.

Carries contract §8's two named regression tests: defect 7's "renaming the
middle session leaves its neighbours unchanged" and defect 9's "the verb is
absent when the server does not report ready". Defect 9's gate is
worktreeReadyItems() in sessionVerbs.ts rather than JSX in ContextMenu.tsx,
so the test file §8 names can reach it.

23 tests: 16 in sessionVerbs.test.ts, 7 in SessionMenu.test.tsx.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

