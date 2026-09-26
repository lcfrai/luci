### Task 61: Start on the desktop: `Shift+R`, the *Undo Start* chip, *Stop and discard*, the 10 s first sign

Stream **WD1**, about 3 hours (an estimate, not a measurement). The desktop half of seam **SW.10**.
Depends on task 24 (the three control-plane routes and their wire, `task-24.md:92-127`) and task 56
(`mutate`, `MutateHandle`, `Refusal`, `onRefused`, `task-56.md:324-356`). Uses task 37's client
members `start`, `undoStart` and `discardStart` and their types (`task-37.md:231-241`, `:353-358`,
`:446-448`), task 37's `workProblem` (`task-37.md:478-485`), task 20's fixtures
(`task-20.md:155-165`, `:217-241`, `:299-330`) and `expectNoPrediction` (`task-20.md:126`), task
44's `onStart` prop (`task-44.md:186`), and the built heartbeat (`desktop/src/hooks/heartbeat.ts:64`,
one interval per window) and `isInsideTerminal` (`desktop/src/hooks/useKeyboardMap.ts:90`). Task
53's `KEYMAP`, `registerVerb`, `isListKeyTarget` and `RowContext`, and task 40's `IssueRow`, have
no part on the tree at this writing: their names are their **scopes'** words (`tasks-41-60.md:390-427`,
`tasks-21-40.md`), **UNVERIFIED**, and they win any disagreement.

After this task a person presses `Shift+R` on an issue of theirs and an agent is working on it in a
lane of its own, with the issue pinned in its context; the row says so at once and offers to take
it back.

**What the reader sees** (05 §4.3 `Shift+R`, `05-design.md:1009`; §4.6 *Undo, revision 2*,
`:1105-1114`; §2.1 point 1, `:517-520`; §5.3's demo, `:1176`):

- **On my issue**, `Shift+R` with the list focused: the row gains a chip in place, *Starting…* as
  words (no spinner), and within the control plane's answer *Started · AU route · Undo Start*. The
  issue's state word becomes `started` when task 22's convergence writes it, through the replica.
- **On an unassigned issue**: the row's assignee becomes me first (one `optimistic.undo` write),
  then it starts. **On a triage issue of mine**: Start accepts it on the way (*accept and Start*,
  L-47), in the same one request (`accept: true`, `task-24.md:97`).
- **On Alice's issue**: nothing is sent, and the row says in place *"EP-163 is assigned to Alice
  Example. Only their agents work on it."* (OD-16), for 4 s. The scope's *"Only her agents"*
  would guess a pronoun from a name; the words here do not.
- **The first sign.** Ten seconds after the session's `createdAt` with no output, the chip's second
  line reads *no output yet · 10s* in mono, and the number climbs with the one heartbeat. Once the
  session record has output, it reads *last output 3s ago*. Before ten seconds it reads nothing.
- ***Undo Start*** stays for as long as the control plane allows it, **past the six seconds** of
  `--undo-window` and with no countdown. Pressed, it stops the session, removes the lane, puts the
  state back, and shows *Start undone · cost not metered in local mode* for 4 s.
- **After the agent's first file write**, pressing *Undo Start* turns the chip to *Stop and
  discard* at once, with *"Too late to undo: the agent changed a file."* Pressing *Stop and
  discard* shows the consequence line (*"Discards 3 files: upload/resume.go, …"*) and a *Discard*
  button; only that second press deletes anything.
- **Select five rows and press *Start 5 worktrees*** (task 44's bulk bar): five chips, one per row,
  each its own Start.

**Decisions made here, so nobody re-derives them:**

1. **Task 24's wire wins over the scope's working names.** The scope's `StartResult {…, startState,
   createdAt}` is not the wire; task 37 types the real one (`StartResult {issueKey, worktreeId,
   sessionId, branch, route, routeForced, undo}`, `task-37.md:235-238`). So `startState` is the
   **desktop's** state, not a field: `undoable` from the 201, `discard_only` from the first
   `409 first_write` (`task-24.md:109`). `createdAt` is the session record's (`store/types.ts:155`),
   read from the cockpit store once the poller lists the session. No field is added to `work.ts`
   (gap 1).
2. **The revision is pinned by the control plane, not sent.** Task 24 reads the head and its head
   revision in one call and returns `issueKey: "issue:is-…@r"` (`task-24.md:17-20`). The desktop
   sends `issueId`, and the test holds that it sends no revision of its own, so a stale replica can
   never pin an older text than the Hub's head.
3. **One pure rule decides what `Shift+R` does to a row**, `startRule(head, me)`, in the order:
   assigned to someone else → `not_mine`; in a state other than `triage`, `backlog` or `todo` →
   `not_startable`; unassigned → `assign_then_start`; mine → `start`. Both starting kinds carry
   `accept: state === "triage"`. It mirrors task 24's refusal table (`task-24.md:121`) so no call is made that
   the route would refuse; the route still decides, and its refusal is shown if they disagree.
4. **Assign first, and only then start.** On an unassigned row, `start` calls `mutate({kind:
   "setAssignee", issueId, assignee: me})` and awaits its `settled`. A `Refusal` means the replica
   has already taken the paint back (`task-56.md:378-380`); the row shows the Hub's own message and
   **no Start is sent**. `"applied"` sends the Start. If the Start is then refused, `handle.undo()`
   sends the inverse assign as the person's own write, so the row is not left assigned by a Start
   that never happened. The undo chip task 56 would show for the assign is not shown: the Start chip
   is the one chip on the row (`mutate`'s handle is held, not surfaced).
5. **The chip learns `discard_only` by asking, never by guessing.** SW.10 has no read of "has the
   agent written a file yet" (`task-24.md:92-115`; gap 2). So the chip offers *Undo Start* until a
   press is answered `409 first_write`; then it says why in place and turns to *Stop and discard*.
   Nothing polls and nothing is retried. The chip never claims undo is still possible: it offers
   the act, and the control plane's answer is the fact.
6. ***Stop and discard* is task 24's two-step discard, surfaced as `confirm.inplace`.** The first
   press sends `discardStart(id, "")`, which only asks: the `409 confirm_required` answer carries
   `details.paths` and `details.confirm` (`task-24.md:113`). The chip shows the consequence line
   from those paths and a *Discard* button; the second press sends the token. A stale token is
   answered with a fresh `confirm_required`, which the chip shows again, so the person always
   confirms what is lost **now**. `details` is `map[string]string` on the wire
   (`control-plane/internal/httpx/problem.go:33`), so the desktop reads `paths` as newline-joined
   (gap 4).
7. **The first sign reads the session record and ticks on the one clock.** `firstSign(session,
   nowMs)` is pure: Go's zero time (`0001-01-01T00:00:00Z`) or an empty `lastOutputAt` is *no
   output*; with no output and `nowMs - createdAt ≥ 10 000` it returns `no output yet · <n>s`;
   with output it returns `last output <n>s ago`; otherwise `null`. `nowMs` is `useHeartbeat()`
   (`heartbeat.ts:64`), the window's one 1 Hz tick. The chip starts no timer of its own, and the
   4 s notes (*Start undone*, the refusal words) are one `setTimeout` each, cleared on unmount.
8. **Start state is in memory and per issue.** `createStartStore()` keeps a `StartEntry` per issue
   id in a module store read with `useSyncExternalStore`, like task 24's in-memory record
   (`task-24.md:49-55`). Nothing is written to disk (Global constraints: no Hub data on disk).
   After a desktop restart the chip is gone and the session and worktree verbs in Develop remain;
   task 24 would answer `404 start_not_found` in any case. An entry in `sending` swallows a second
   `Shift+R` on that issue, so a double press is one Start even before the control plane's
   `requestId` rule (`task-24.md:51-52`) is reached. The `requestId` is `makeId("rq")`
   (`desktop/src/lib/ids.ts:12-15`, `rq-<n>-<8 hex>`), inside the wire's 8-64 rule.
9. **The base branch is the project's checked-out branch.** Task 24 needs `branch` (the base).
   The desktop sends the cockpit store's `isHead` branch of the issue's project
   (`store/types.ts:57-63`). Task 24 compares the request's `projectId` with the issue's
   (`task-24.md:256`), so the desktop sends `head.projectId`; that the control plane's project id
   and the Hub's are one value is task 24's assumption (gap 3). No head branch known: nothing is
   sent, and the row says *"No branch is checked out for this project yet."*
10. **Words for the route and the cost, and no number that was not observed.** `route` reads
    *Global route* or *AU route*; `routeForced` adds *· set by the project's data class* (task 24:
    every project is forced AU in W1, `task-24.md:512-515`). `cost: null` reads *cost not metered in
    local mode* (`OD-27`). The chip has no bar, no `%` and no countdown.

**Files** (the skeleton's File map lists `StartChip.tsx` for task 61, `:306`):

- `desktop/src/work/start/useStart.ts` — `startRule`, `firstSign`, `StartEntry`, the store,
  `useStart`, `useStartEntry` (new)
- `desktop/src/work/start/useStart.test.ts` — the rule, the sign and the store (new)
- `desktop/src/work/start/register.ts` — registers `issue.start` with task 53's `registerVerb` (new)
- `desktop/src/components/work/StartChip.tsx` — the chip, its first-sign line, the discard step (new)
- `desktop/src/components/work/StartChip.test.tsx` — keys and surfaces (new)
- `desktop/src/components/work/IssueRow.tsx` (task 40's) — renders the chip and the in-place note
  (modified)
- `desktop/src/components/work/WorkLens.tsx` (task 40's) — registers the verb on mount; passes
  `onStart` to task 44's bulk bar (modified)
- `desktop/src/components/work/work.css` (task 40's) — the `.sc-*` rules, appended (modified)
- `desktop/src/work/keys/keymap.ts` ⚑ (task 53's) — only if task 53 did not print the `Shift+R`
  row: `{verb: "issue.start", keys: ["Shift+R"], code: "KeyR", shift: true, scope: "issueRow",
  slice: "W1", label: "Start", linear: "none"}` (modified, conditional)

`desktop/src/api/work.ts` ⚑ is **not** touched: task 37 already has the three calls.

#### Interfaces — complete: `desktop/src/work/start/useStart.ts`, part 1: the rule, the sign, the entry

```ts
// desktop/src/work/start/useStart.ts — SW.10's desktop half, task 61.
//
// Start is the person's own act on their own issue (OD-16). This file decides what Shift+R does
// to a row, holds each Start in memory while its chip is on screen, and reads the first sign
// from the session record. It sends through task 37's client and writes through task 56's mutate.
import { useSyncExternalStore } from "react";
import { makeId } from "../../lib/ids";
import { formatElapsed } from "../../lib/time";
import { ApiError } from "../../api/client";
import { workProblem } from "../../api/work";
import type { IssueHead, IssueState, StartResult, User, WorkClient } from "../../api/work";
import type { Session } from "../../store/types";
import type { ReplicaWrites } from "../replica/replica";

export type StartRule =
  | { kind: "start"; accept: boolean }
  | { kind: "assign_then_start"; accept: boolean }
  | { kind: "not_mine"; owner: User }
  | { kind: "not_startable"; state: IssueState };

/** Task 24's startable states (task-24.md:121). The route still decides; this saves a call. */
const STARTABLE: ReadonlySet<IssueState> = new Set<IssueState>(["triage", "backlog", "todo"]);

export function startRule(head: IssueHead, me: User): StartRule {
  if (head.assignee !== null && head.assignee.id !== me.id) return { kind: "not_mine", owner: head.assignee };
  if (!STARTABLE.has(head.state)) return { kind: "not_startable", state: head.state };
  const accept = head.state === "triage";
  return head.assignee === null ? { kind: "assign_then_start", accept } : { kind: "start", accept };
}

/** The in-place words for a rule that sends nothing. No pronoun is guessed from a name. */
export function ruleWords(rule: StartRule, head: IssueHead): string | null {
  if (rule.kind === "not_mine")
    return `${head.alias} is assigned to ${rule.owner.displayName}. Only their agents work on it.`;
  if (rule.kind === "not_startable")
    return `${head.alias} is ${rule.state.replace("_", " ")}. Start takes triage, backlog or todo.`;
  return null;
}

/** 05 §4.6's budget.ack: a design value, not a measurement. */
export const FIRST_SIGN_MS = 10_000;

/** Go's zero time is how the record says "never" (registry/session.go:99). */
function observedMs(iso: string): number | null {
  if (iso === "" || iso.startsWith("0001-01-01")) return null;
  const t = Date.parse(iso);
  return Number.isFinite(t) ? t : null;
}

/** "7s", "59s", then time.ts's own "1m", "1h 06m" (floors, never early). */
function since(iso: string, fromMs: number, nowMs: number): string {
  const ms = Math.max(0, nowMs - fromMs);
  return ms < 60_000 ? `${Math.floor(ms / 1000)}s` : formatElapsed(iso, nowMs);
}

/** The chip's second line. Pure: nowMs is useHeartbeat()'s tick. null draws nothing. */
export function firstSign(
  s: Pick<Session, "status" | "exitReason" | "createdAt" | "lastOutputAt">, nowMs: number,
): string | null {
  if (s.status === "exited") return s.exitReason === "" ? "exited" : `exited · ${s.exitReason}`;
  const out = observedMs(s.lastOutputAt);
  if (out !== null) return `last output ${since(s.lastOutputAt, out, nowMs)} ago`;
  const born = observedMs(s.createdAt);
  if (born === null || nowMs - born < FIRST_SIGN_MS) return null;
  return `no output yet · ${since(s.createdAt, born, nowMs)}`;
}

/** The desktop's state of one Start (decision 1). `startState` of the scope is `undoable` and
 *  `discard_only` here; the rest are the steps between presses. */
export type StartPhase =
  | { kind: "sending" }
  | { kind: "undoable"; result: StartResult; note?: string }   // note: why the last undo did not
  | { kind: "asking"; result: StartResult }            // an undo or a discard is in flight
  | { kind: "discard_only"; result: StartResult; note: string }
  | { kind: "confirming"; result: StartResult; paths: readonly string[]; token: string }
  | { kind: "undone"; stateRestored: IssueState | null }
  | { kind: "discarded"; stateRestored: IssueState | null }
  | { kind: "said"; words: string };                   // a local rule or the server's refusal
export interface StartEntry { issueId: string; phase: StartPhase }
```

`exited` is the record's own word and its `exitReason` verbatim (`store/types.ts:149-151`); an
exit is not interpreted here (FAILED is the Desk's, task 64).

#### Interfaces — complete: `useStart.ts`, part 2: the store and the two hooks

```ts
export interface StartDeps {
  work: Pick<WorkClient, "start" | "undoStart" | "discardStart">;
  replica: Pick<ReplicaWrites, "mutate">;
  me: User;
  /** The cockpit store's isHead branch name for a project, or null (decision 9). */
  headBranchOf(projectId: string): string | null;
  /** Test seam; makeId("rq") by default. */
  newRequestId?: () => string;
}

export interface StartStore {
  get(issueId: string): StartEntry | undefined;
  subscribe(listener: () => void): () => void;
  /** Shift+R and the bulk bar. Resolves when the chip has its next phase. */
  start(head: IssueHead): Promise<void>;
  /** Undo Start. A 409 first_write turns the entry to discard_only (decision 5). */
  undo(issueId: string): Promise<void>;
  /** Stop and discard: in discard_only it asks; in confirming it sends the token (decision 6). */
  discard(issueId: string): Promise<void>;
  /** The chip calls this 4 s after an undone, discarded or said phase. */
  clear(issueId: string): void;
}

export function createStartStore(deps: StartDeps): StartStore;

/** The one store per window, built by WorkLens on mount (the replica and the client are its). */
export function setStartStore(store: StartStore | null): void;
/** The row's entry, or undefined; re-renders only that row (useSyncExternalStore). */
export function useStartEntry(issueId: string): StartEntry | undefined;
/** The store's verbs, for the key, the chip and the bulk bar. Throws if no store is set. */
export function useStart(): Pick<StartStore, "start" | "undo" | "discard" | "clear">;
```

#### Reference implementation: `createStartStore` (the tests are the contract; this is one way to pass them)

```ts
export function createStartStore(deps: StartDeps): StartStore {
  const entries = new Map<string, StartEntry>();
  const listeners = new Set<() => void>();
  const put = (issueId: string, phase: StartPhase): void => {
    entries.set(issueId, { issueId, phase });
    for (const l of listeners) l();
  };
  const words = (e: unknown): string =>
    e instanceof ApiError ? workProblem(e).message : "The control plane did not answer.";

  async function start(head: IssueHead): Promise<void> {
    if (entries.get(head.id)?.phase.kind === "sending") return;          // a double press is one Start
    const rule = startRule(head, deps.me);
    const said = ruleWords(rule, head);
    if (said !== null) return put(head.id, { kind: "said", words: said });
    const branch = deps.headBranchOf(head.projectId);
    if (branch === null) return put(head.id, { kind: "said", words: "No branch is checked out for this project yet." });
    if (rule.kind !== "start" && rule.kind !== "assign_then_start") return;
    put(head.id, { kind: "sending" });
    let assign: ReturnType<ReplicaWrites["mutate"]> | null = null;
    if (rule.kind === "assign_then_start") {
      assign = deps.replica.mutate({ kind: "setAssignee", issueId: head.id, assignee: deps.me });
      const settled = await assign.settled;
      if (settled !== "applied") return put(head.id, { kind: "said", words: settled.message });
    }
    try {
      const result = await deps.work.start({
        projectId: head.projectId, issueId: head.id, branch, accept: rule.accept,
        requestId: (deps.newRequestId ?? (() => makeId("rq")))(),
      });
      put(head.id, { kind: "undoable", result });
    } catch (e) {
      assign?.undo();                                                    // decision 4
      put(head.id, { kind: "said", words: words(e) });
    }
  }
```

```ts
  async function undo(issueId: string): Promise<void> {
    const cur = entries.get(issueId)?.phase;
    if (cur?.kind !== "undoable") return;
    put(issueId, { kind: "asking", result: cur.result });
    try {
      const r = await deps.work.undoStart(cur.result.sessionId);
      put(issueId, { kind: "undone", stateRestored: r.stateRestored });
    } catch (e) {
      const p = e instanceof ApiError ? workProblem(e) : null;
      if (p?.kind === "first_write")                                      // decision 5: no retry
        return put(issueId, { kind: "discard_only", result: cur.result, note: "Too late to undo: the agent changed a file." });
      put(issueId, { kind: "undoable", result: cur.result, note: words(e) });   // still offered
    }
  }

  async function discard(issueId: string): Promise<void> {
    const cur = entries.get(issueId)?.phase;
    if (cur?.kind !== "discard_only" && cur?.kind !== "confirming") return;
    const token = cur.kind === "confirming" ? cur.token : "";            // the first press only asks
    put(issueId, { kind: "asking", result: cur.result });
    try {
      const r = await deps.work.discardStart(cur.result.sessionId, token);
      put(issueId, { kind: "discarded", stateRestored: r.stateRestored });
    } catch (e) {
      const p = e instanceof ApiError ? workProblem(e) : null;
      if (p?.kind === "confirm") {                                        // a first ask, or a stale token
        const paths = (p.details.paths ?? "").split("\n").filter((s) => s !== "");
        return put(issueId, { kind: "confirming", result: cur.result, paths, token: p.details.confirm ?? "" });
      }
      put(issueId, { kind: "discard_only", result: cur.result, note: words(e) });
    }
  }

  return {
    get: (id) => entries.get(id),
    subscribe: (l) => { listeners.add(l); return () => { listeners.delete(l); }; },
    start, undo, discard,
    clear: (id) => { if (entries.delete(id)) for (const l of listeners) l(); },
  };
}

let current: StartStore | null = null;
export function setStartStore(store: StartStore | null): void { current = store; }

const none = (): (() => void) => () => {};
export function useStartEntry(issueId: string): StartEntry | undefined {
  const s = current;
  return useSyncExternalStore(s ? s.subscribe : none, () => s?.get(issueId), () => s?.get(issueId));
}

export function useStart(): Pick<StartStore, "start" | "undo" | "discard" | "clear"> {
  if (current === null) throw new Error("useStart: no StartStore; WorkLens sets it on mount");
  return current;
}
```

`get` returns the same `StartEntry` object until `put` replaces it, so `useSyncExternalStore`'s
snapshot is stable between changes and a row re-renders only when its own entry changes. An undo
that fails for any reason but `first_write` (the control plane down, `404 start_not_found` after a
restart) keeps *Undo Start* offered with the server's words beneath it: the words are the
control plane's, never a guess, and the person can press again or use Develop's session verbs.
`"The control plane did not answer."` is shown only for a rejection that is not an `ApiError`,
which task 37's client never raises (`task-37.md:317`); it is a floor, not a path.

#### Interfaces — complete: `desktop/src/components/work/StartChip.tsx`

```tsx
// desktop/src/components/work/StartChip.tsx — the row's one chip for a Start (task 61).
//
// Words, never a spinner or a countdown. The first-sign line is the only thing that ticks, and
// it ticks on the window's one heartbeat (DESIGN-SYSTEM; heartbeat.ts:64).
import { useEffect } from "react";
import { useHeartbeat } from "../../hooks/heartbeat";
import { useCockpit } from "../../store/store";
import type { StartResult } from "../../api/work";
import { firstSign, useStart, useStartEntry } from "../../work/start/useStart";

/** How long a note (said, undone, discarded) stays before the chip clears. A design value. */
export const NOTE_MS = 4000;

const routeWords = (r: StartResult): string =>
  (r.route === "au" ? "AU route" : "Global route") + (r.routeForced ? " · set by the project's data class" : "");

/** Its own component so only a live chip subscribes to the heartbeat (heartbeat.ts:59-63). */
function FirstSign({ sessionId }: { sessionId: string }): JSX.Element | null {
  const now = useHeartbeat();
  const session = useCockpit((s) => s.sessions[sessionId]);
  const line = session === undefined ? null : firstSign(session, now);
  return line === null ? null : <span className="sc-sign">{line}</span>;
}

export function StartChip({ issueId }: { issueId: string }): JSX.Element | null {
  const entry = useStartEntry(issueId);
  const s = useStart();
  const phase = entry?.phase;
  useEffect(() => {
    if (phase === undefined || !["said", "undone", "discarded"].includes(phase.kind)) return;
    const t = setTimeout(() => s.clear(issueId), NOTE_MS);
    return () => clearTimeout(t);
  }, [phase, issueId, s]);
  if (phase === undefined) return null;

  const body = (() => {
    switch (phase.kind) {
      case "sending": return <span className="sc-word">Starting…</span>;
      case "asking": return <span className="sc-word">Asking the control plane…</span>;
      case "said": return <span className="sc-note" role="status">{phase.words}</span>;
      case "undone": case "discarded": return (
        <span className="sc-note" role="status">
          {phase.kind === "undone" ? "Start undone" : "Discarded"}
          {phase.stateRestored !== null ? ` · back to ${phase.stateRestored.replace("_", " ")}` : ""}
          {" · cost not metered in local mode"}
        </span>);
      case "undoable": return (<>
        <span className="sc-word">Started · {routeWords(phase.result)}</span>
        <button type="button" className="sc-act" onClick={() => void s.undo(issueId)}>Undo Start</button>
        {phase.note ? <span className="sc-note" role="status">{phase.note}</span> : null}
        <FirstSign sessionId={phase.result.sessionId} />
      </>);
      case "discard_only": return (<>
        <span className="sc-note" role="status">{phase.note}</span>
        <button type="button" className="sc-act" onClick={() => void s.discard(issueId)}>Stop and discard</button>
        <FirstSign sessionId={phase.result.sessionId} />
      </>);
      case "confirming": return (<>
        <span className="sc-consequence" role="alert">
          {phase.paths.length === 0
            ? "Discards the lane's commits that have not landed."
            : `Discards ${phase.paths.length} ${phase.paths.length === 1 ? "file" : "files"}: ${phase.paths.join(", ")}`}
        </span>
        <button type="button" className="sc-act sc-danger" onClick={() => void s.discard(issueId)}>Discard</button>
      </>);
    }
  })();
  return <span className="sc" data-start-phase={phase.kind}>{body}</span>;
}
```

The `undone` and `discarded` notes say *cost not metered in local mode* because task 24's `cost` is
`null` in local mode (`task-24.md:108`, `:126-127`); when phase 6's gateway returns a figure, the
note shows it verbatim and this line changes, which is phase 6's delta, not this task's. The
empty-paths wording is task 24's `ConfirmNeeded.Paths` comment (`task-24.md:216`: *"[] when the
loss is unlanded commits only"*). The consequence line is a count of observed paths, which is a
fact the control plane returned, not a prediction.

