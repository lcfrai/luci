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

#### Interfaces — complete: `desktop/src/work/start/register.ts` and the edits to task 40's files

```ts
// desktop/src/work/start/register.ts — Shift+R through task 53's one verb registry.
import { registerVerb, type RowContext } from "../keys/keymap";          // task 53 (scope words, UNVERIFIED)
import type { IssueHead } from "../../api/work";
import type { StartStore } from "./useStart";

/** Called once by WorkLens on mount; returns the unregister. task 53's matchListKey has already
 *  held the key to event.code "KeyR" with shiftKey, list focus and no terminal host. */
export function registerStartVerb(store: Pick<StartStore, "start">, headOf: (id: string) => IssueHead | undefined): () => void {
  return registerVerb("issue.start", (row: RowContext) => {
    const head = headOf(row.issueId);
    if (head !== undefined) void store.start(head);
  });
}
```

- **`WorkLens.tsx`** (task 40's). On mount: `const store = createStartStore({work, replica, me,
  headBranchOf})`, `setStartStore(store)`, `const off = registerStartVerb(store, (id) =>
  replica.head(id))`; on unmount `off()` and `setStartStore(null)`. `work` is the lens's
  `work` client (the line task 37 adds to `createClient`, `task-37.md:521`); `replica` is task 55's; `headBranchOf(p)` is
  `useCockpit.getState().branchesByProject[p]?.find((b) => b.isHead)?.name ?? null`
  (`store/slices/dataSlice.ts:32`, `store/types.ts:57-63`). Task 44's bulk bar gets
  `onStart={(ids) => { for (const id of ids) { const h = replica.head(id); if (h) void store.start(h); } }}`
  (`task-44.md:186`): one Start per id, each its own chip, no batch route.
- **`IssueRow.tsx`** (task 40's). After the row's trailing cells: `<StartChip issueId={head.id} />`.
  The chip renders nothing without an entry, so a row with no Start is unchanged.

`RowContext` and `registerVerb`'s run signature are task 53's scope words (`tasks-41-60.md:402`;
task 102 uses the same shape, `task-102.md:253`). If task 53's part names the verb `start` rather
than `issue.start`, this file follows it.

#### Interfaces — complete: appended to `desktop/src/components/work/work.css`

```css
/* Start chip (task 61). A chip is the chip radius step; its button is text, not a fill. Nothing
   here animates: the chip changes words, and words need no motion to explain them. */
.sc { display: inline-flex; flex-wrap: wrap; align-items: baseline; gap: var(--s-2);
  padding: 0 var(--s-2); border-radius: var(--r-chip); box-shadow: 0 0 0 1px var(--line); }
.sc-word, .sc-note, .sc-consequence { font: var(--t-ui) / var(--t-ui-lh) var(--sans); color: var(--ink); }
.sc-note { color: var(--ink-2); }
.sc-sign { font-family: var(--mono); font-size: var(--t-fact); line-height: var(--t-fact-lh);
  font-variant-numeric: tabular-nums; color: var(--ink-2); }
.sc-act { background: none; border: 0; padding: 0; color: var(--ink); text-decoration: underline;
  cursor: pointer; }
.sc-danger { color: var(--fail); }
.sc-consequence { color: var(--fail); }
```

`--r-chip` (`tokens.css:110`), `--ink-2` (`:46`) and `--fail` (`:76`) are the tree's; `--s-*`,
`--t-ui-lh`, `--t-fact` and `--t-fact-lh` are task 26's (`task-26.md:109`, `:420`), and task 26
wins a differing name. `--line` is the tree's hairline (`tokens.css:54`). The ring is the chip's
1px elevation; the chip sits in a row, so it takes no layered shadow.

#### Tests to write first, part 1 — `desktop/src/work/start/useStart.test.ts` (the rule, the sign, the store)

```ts
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../api/client";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead } from "../../test/workFixtures";
import { createStartStore, firstSign, startRule, type StartDeps } from "./useStart";

const RESULT = { issueKey: `issue:${WORK_IDS.ep142}@3`, worktreeId: "w-4a77b1c9", sessionId: "s-3f2a91c0",
  branch: "ep-142-resume-upload", route: "au", routeForced: true, undo: "undo_start" } as const;
const T0 = Date.parse("2026-09-25T09:14:02Z");
const rec = (over = {}) => ({ status: "running" as const, exitReason: "", createdAt: "2026-09-25T09:14:02Z",
  lastOutputAt: "0001-01-01T00:00:00Z", ...over });
function deps(over: Partial<StartDeps> = {}) {
  const work = { start: vi.fn(async () => RESULT), undoStart: vi.fn(), discardStart: vi.fn() };
  const undo = vi.fn();
  const mutate = vi.fn(() => ({ id: "op-1", undoable: true, undo, settled: Promise.resolve("applied" as const) }));
  const d = { work, replica: { mutate }, me: DYLAN, headBranchOf: () => "main", newRequestId: () => "rq-test-0001", ...over };
  return { d: d as unknown as StartDeps, work, mutate, undo };
}

describe("startRule", () => {
  it("decides by owner, then state, then assignee", () => {
    expect(startRule(makeIssueHead({ assignee: ALICE }), DYLAN).kind).toBe("not_mine");
    expect(startRule(makeIssueHead({ state: "started" }), DYLAN)).toEqual({ kind: "not_startable", state: "started" });
    expect(startRule(makeIssueHead({ assignee: null, state: "triage" }), DYLAN)).toEqual({ kind: "assign_then_start", accept: true });
    expect(startRule(makeIssueHead({ state: "todo" }), DYLAN)).toEqual({ kind: "start", accept: false });
    // mutant: Start for any row (Alice's issue starts an agent of mine)
  });
});

describe("firstSign", () => {
  it("says nothing before 10 s, the fact at 10 s, and output once there is some", () => {
    expect(firstSign(rec(), T0 + 9_999)).toBeNull();
    expect(firstSign(rec(), T0 + 10_000)).toBe("no output yet · 10s");
    expect(firstSign(rec({ lastOutputAt: "2026-09-25T09:14:40Z" }), T0 + 41_000)).toBe("last output 3s ago");
    expect(firstSign(rec({ status: "exited", exitReason: "process exited" }), T0)).toBe("exited · process exited");
    // mutants: Go's zero time read as output at year 1; a percentage or an elapsed bar
  });
});

describe("createStartStore", () => {
  it("assigns me first on an unassigned row, then starts once, sending no revision", async () => {
    const { d, work, mutate } = deps();
    const s = createStartStore(d);
    await s.start(makeIssueHead({ assignee: null, state: "triage" }));
    expect(mutate).toHaveBeenCalledWith({ kind: "setAssignee", issueId: WORK_IDS.ep142, assignee: DYLAN });
    expect(work.start).toHaveBeenCalledTimes(1);
    expect(work.start).toHaveBeenCalledWith({ projectId: WORK_IDS.project, issueId: WORK_IDS.ep142,
      branch: "main", accept: true, requestId: "rq-test-0001" });           // no revision, no headKey
    expect(s.get(WORK_IDS.ep142)?.phase.kind).toBe("undoable");
    // mutant: Start sent before the assign stood (mutate's settled not awaited)
  });
  it("a refused assign sends no Start and shows the Hub's words", async () => {
    const refusal = { op: {}, issueId: WORK_IDS.ep142, code: "forbidden", message: "Only a team member can take this issue.", details: {} };
    const { d, work } = deps({ replica: { mutate: vi.fn(() => ({ id: "op-1", undoable: true, undo: vi.fn(),
      settled: Promise.resolve(refusal) })) } as unknown as StartDeps["replica"] });
    const s = createStartStore(d);
    await s.start(makeIssueHead({ assignee: null }));
    expect(work.start).not.toHaveBeenCalled();
    expect(s.get(WORK_IDS.ep142)?.phase).toEqual({ kind: "said", words: "Only a team member can take this issue." });
    // mutant: Start for a row the Hub would not let me take
  });
  it("a refused Start takes the assign back as my own write", async () => {
    const { d, work, undo } = deps();
    work.start.mockRejectedValueOnce(new ApiError("conflict" as never, "EP-142 is started.", 409, {}));
    const s = createStartStore(d);
    await s.start(makeIssueHead({ assignee: null }));
    expect(undo).toHaveBeenCalledTimes(1);
    expect(s.get(WORK_IDS.ep142)?.phase).toEqual({ kind: "said", words: "EP-142 is started." });
    // mutant: the row stays assigned to me by a Start that never happened
  });
  it("a double press is one Start", async () => {
    const { d, work } = deps();
    const s = createStartStore(d);
    const head = makeIssueHead();
    await Promise.all([s.start(head), s.start(head)]);
    expect(work.start).toHaveBeenCalledTimes(1);
    // mutant: two lanes for one double press
  });
  it("a first_write answer turns the chip to Stop and discard, and nothing retries", async () => {
    const { d, work } = deps();
    work.undoStart.mockRejectedValue(new ApiError("first_write" as never, "The agent has written a file.", 409, { next: "discard" }));
    const s = createStartStore(d);
    await s.start(makeIssueHead());
    await s.undo(WORK_IDS.ep142);
    expect(work.undoStart).toHaveBeenCalledTimes(1);
    expect(s.get(WORK_IDS.ep142)?.phase).toMatchObject({ kind: "discard_only",
      note: "Too late to undo: the agent changed a file." });
    // mutants: the chip keeps offering Undo Start; the store retries the undo
  });
  it("Stop and discard asks first, and only the second press sends the token", async () => {
    const { d, work } = deps();
    work.undoStart.mockRejectedValue(new ApiError("first_write" as never, "", 409, { next: "discard" }));
    work.discardStart
      .mockRejectedValueOnce(new ApiError("confirm_required" as never, "Confirm the discard.", 409,
        { paths: "upload/resume.go\nupload/resume_test.go", confirm: "9c1e44a0b2d3f6e7" }))
      .mockResolvedValueOnce({ discarded: true, stateRestored: "todo" });
    const s = createStartStore(d);
    await s.start(makeIssueHead());
    await s.undo(WORK_IDS.ep142);
    await s.discard(WORK_IDS.ep142);
    expect(work.discardStart).toHaveBeenLastCalledWith(RESULT.sessionId, "");
    expect(s.get(WORK_IDS.ep142)?.phase).toMatchObject({ kind: "confirming",
      paths: ["upload/resume.go", "upload/resume_test.go"] });
    await s.discard(WORK_IDS.ep142);
    expect(work.discardStart).toHaveBeenLastCalledWith(RESULT.sessionId, "9c1e44a0b2d3f6e7");
    expect(s.get(WORK_IDS.ep142)?.phase).toEqual({ kind: "discarded", stateRestored: "todo" });
    // mutant: one press discards (the token is fetched and sent in the same press)
  });
});
```

`ApiError`'s `code` is typed `ProblemCode` (`desktop/src/api/client.ts:40-48`), which lists none of
task 24's codes; the tests cast, and gap 5 asks task 37 (the owner of `work.ts`'s problem kinds)
whether `ProblemCode` widens.

#### Tests to write first, part 2 — `desktop/src/components/work/StartChip.test.tsx` (keys)

The lens is rendered on task 20's `workScenario()` through task 40's test seam, as task 60 does
(`task-60.md:567-592`); the seam's `work` option, which hands the lens a fake client, is this
part's assumption of that seam (**UNVERIFIED**, gap 6). `DYLAN` is signed in; EP-167 is his and in
`todo`, EP-163 is Alice's (`task-20.md:299-330`). The cockpit store gets the project's head branch.

```tsx
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../api/client";
import { __resetHeartbeat } from "../../hooks/heartbeat";
import { resetCockpit, useCockpit } from "../../store/store";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { DYLAN, WORK_IDS, workScenario } from "../../test/workFixtures";
import { NOTE_MS } from "./StartChip";
import { WorkLens, setWorkSourceForTest } from "./WorkLens";

const T0 = Date.parse("2026-09-25T09:14:02Z");
const result = (issueId: string) => ({ issueKey: `issue:${issueId}@3`, worktreeId: "w-4a77b1c9",
  sessionId: `s-${issueId.slice(3, 11)}`, branch: "ep-167", route: "au" as const, routeForced: true, undo: "undo_start" as const });
let work: { start: ReturnType<typeof vi.fn>; undoStart: ReturnType<typeof vi.fn>; discardStart: ReturnType<typeof vi.fn> };

function lens() {
  setWorkSourceForTest(workScenario(), { me: DYLAN, work });
  render(<WorkLens />);
  const list = screen.getByRole("grid");
  list.focus();
  return list;
}
const row = (alias: string) => screen.getByText(alias).closest<HTMLElement>("[data-row-id]")!;
const shiftR = (el: Element, key = "R") => fireEvent.keyDown(el, { code: "KeyR", key, shiftKey: true });
function session(issueId: string, over = {}) {
  const id = result(issueId).sessionId;
  useCockpit.setState((s) => ({ sessions: { ...s.sessions, [id]: { id, status: "running", exitReason: "",
    createdAt: new Date(T0).toISOString(), lastOutputAt: "0001-01-01T00:00:00Z", ...over } as never } }));
}

beforeEach(() => {
  vi.useFakeTimers({ now: T0 });
  resetCockpit(); __resetHeartbeat();
  useCockpit.setState({ branchesByProject: { [WORK_IDS.project]: [
    { id: "b-1", projectId: WORK_IDS.project, name: "main", isHead: true }] } } as never);
  work = { start: vi.fn(async (req: { issueId: string }) => result(req.issueId)), undoStart: vi.fn(), discardStart: vi.fn() };
});

describe("Start keys", () => {
  it("Shift+R starts once by the physical key, whatever the layout types", async () => {
    const list = lens();
    fireEvent.focus(row("EP-167"));                                   // task 40's row cursor
    shiftR(list, "К");                                                // a Cyrillic layout's key
    await act(async () => { await Promise.resolve(); });
    expect(work.start).toHaveBeenCalledTimes(1);
    expect(work.start.mock.calls[0][0]).toMatchObject({ issueId: WORK_IDS.ep167, accept: false, branch: "main" });
    // mutant: matching event.key === "R"
  });
  it("Caps Lock R without Shift starts nothing", () => {
    const list = lens();
    fireEvent.focus(row("EP-167"));
    fireEvent.keyDown(list, { code: "KeyR", key: "R", shiftKey: false });
    expect(work.start).not.toHaveBeenCalled();
    // mutant: case-folding the key instead of reading shiftKey
  });
  it("Shift+R inside a terminal host does nothing", () => {
    lens();
    const { container } = render(<div data-terminal-host=""><textarea aria-label="term" /></div>);
    shiftR(within(container).getByLabelText("term"));
    expect(work.start).not.toHaveBeenCalled();
    // mutant: no isInsideTerminal check (a key bound on document)
  });
  it("another person's issue calls no route and says whose it is, in the row", () => {
    const list = lens();
    fireEvent.focus(row("EP-163"));
    shiftR(list);
    expect(work.start).not.toHaveBeenCalled();
    expect(within(row("EP-163")).getByRole("status").textContent)
      .toBe("EP-163 is assigned to Alice Example. Only their agents work on it.");
    // mutant: Start for any row (OD-16)
  });
});
```

#### Tests to write first, part 3 — `StartChip.test.tsx` (surfaces and source guards)

Same file, same `lens()`, `row`, `shiftR` and `session` helpers as part 2. `chip(alias)` is the
row's `.sc`. Times advance with the fake clock, so the one heartbeat ticks as it does in the app.

```tsx
import chipSrc from "./StartChip.tsx?raw";
import storeSrc from "../../work/start/useStart.ts?raw";
import cssSrc from "./work.css?raw";

const chip = (alias: string) => row(alias).querySelector<HTMLElement>(".sc");
async function started(alias = "EP-167") {
  const list = lens();
  fireEvent.focus(row(alias));
  shiftR(list);
  await act(async () => { await Promise.resolve(); });
  return list;
}

describe("Start surfaces", () => {
  it("the chip says what happened in words, with no spinner, bar or countdown", async () => {
    await started();
    expect(chip("EP-167")!.textContent).toContain("Started · AU route · set by the project's data class");
    expect(within(chip("EP-167")!).getByRole("button", { name: "Undo Start" })).toBeTruthy();
    expect(chip("EP-167")!.querySelector('[role="progressbar"], svg, .spinner')).toBeNull();
    expectNoPrediction(document.body);
    // mutants: a spinner per row; an "undo in 6s" countdown
  });
  it("the first sign is empty before 10 s, then climbs on the one heartbeat", async () => {
    await started();
    session(WORK_IDS.ep167);
    act(() => { vi.advanceTimersByTime(9_000); });
    expect(chip("EP-167")!.querySelector(".sc-sign")).toBeNull();
    act(() => { vi.advanceTimersByTime(1_000); });
    expect(chip("EP-167")!.querySelector(".sc-sign")!.textContent).toBe("no output yet · 10s");
    act(() => { vi.advanceTimersByTime(1_000); });
    expect(chip("EP-167")!.querySelector(".sc-sign")!.textContent).toBe("no output yet · 11s");
    session(WORK_IDS.ep167, { lastOutputAt: "2026-09-25T09:14:13Z" });  // T0 + 11 s
    act(() => { vi.advanceTimersByTime(1_000); });
    expect(chip("EP-167")!.querySelector(".sc-sign")!.textContent).toBe("last output 1s ago");
    // mutants: a sign before 10 s; the chip's own setInterval (the source guard below holds it too)
  });
  it("Undo Start is still offered a minute in, and a press undoes and says so for 4 s", async () => {
    work.undoStart.mockResolvedValue({ undone: true, stateRestored: "todo" });
    await started();
    act(() => { vi.advanceTimersByTime(60_000); });
    fireEvent.click(within(chip("EP-167")!).getByRole("button", { name: "Undo Start" }));
    await act(async () => { await Promise.resolve(); });
    expect(work.undoStart).toHaveBeenCalledWith(result(WORK_IDS.ep167).sessionId);
    expect(within(row("EP-167")).getByRole("status").textContent)
      .toBe("Start undone · back to todo · cost not metered in local mode");
    act(() => { vi.advanceTimersByTime(NOTE_MS); });
    expect(chip("EP-167")).toBeNull();
    // mutant: the chip hides Undo Start after the six seconds of --undo-window
  });
  it("too late to undo turns the chip to Stop and discard, and only Discard deletes", async () => {
    work.undoStart.mockRejectedValue(new ApiError("first_write" as never, "", 409, { next: "discard" }));
    work.discardStart
      .mockRejectedValueOnce(new ApiError("confirm_required" as never, "", 409,
        { paths: "upload/resume.go\nupload/resume_test.go", confirm: "9c1e44a0b2d3f6e7" }))
      .mockResolvedValueOnce({ discarded: true, stateRestored: "todo" });
    await started();
    fireEvent.click(within(chip("EP-167")!).getByRole("button", { name: "Undo Start" }));
    await act(async () => { await Promise.resolve(); });
    expect(chip("EP-167")!.textContent).toContain("Too late to undo: the agent changed a file.");
    fireEvent.click(within(chip("EP-167")!).getByRole("button", { name: "Stop and discard" }));
    await act(async () => { await Promise.resolve(); });
    expect(within(chip("EP-167")!).getByRole("alert").textContent)
      .toBe("Discards 2 files: upload/resume.go, upload/resume_test.go");
    expect(work.discardStart).toHaveBeenCalledTimes(1);
    fireEvent.click(within(chip("EP-167")!).getByRole("button", { name: "Discard" }));
    await act(async () => { await Promise.resolve(); });
    expect(work.discardStart).toHaveBeenLastCalledWith(result(WORK_IDS.ep167).sessionId, "9c1e44a0b2d3f6e7");
    // mutant: Stop and discard deletes on its first press
  });
  it("Start 5 worktrees is five Starts, one chip per row, and no batch route", async () => {
    const list = lens();
    for (const a of ["EP-160", "EP-166", "EP-167", "EP-163", "EP-150"]) {
      fireEvent.focus(row(a));
      fireEvent.keyDown(list, { code: "KeyX", key: "x" });           // task 44's select key
    }
    const bar = screen.getByRole("toolbar");
    fireEvent.click(within(bar).getByRole("button", { name: "Start 5 worktrees" }));
    fireEvent.click(within(bar).getByRole("button", { name: "Start 5 worktrees" }));  // task 44's second press
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(work.start.mock.calls.map((c) => c[0].issueId).sort())
      .toEqual([WORK_IDS.ep160, WORK_IDS.ep166, WORK_IDS.ep167].sort());
    expect(work.start.mock.calls.find((c) => c[0].issueId === WORK_IDS.ep160)![0].accept).toBe(true);
    for (const a of ["EP-160", "EP-166", "EP-167"]) expect(chip(a)!.dataset.startPhase).toBe("undoable");
    expect(within(row("EP-163")).getByRole("status").textContent).toContain("Only their agents work on it.");
    expect(within(row("EP-150")).getByRole("status").textContent)
      .toBe("EP-150 is in review. Start takes triage, backlog or todo.");
    // mutants: one call for the batch; Start on Alice's row; accept dropped on the triage row
  });
  it("no head branch sends nothing and says why in the row", () => {
    useCockpit.setState({ branchesByProject: {} } as never);
    const list = lens();
    fireEvent.focus(row("EP-167"));
    shiftR(list);
    expect(work.start).not.toHaveBeenCalled();
    expect(within(row("EP-167")).getByRole("status").textContent)
      .toBe("No branch is checked out for this project yet.");
    // mutant: Start against a guessed "main"
  });
  it("the chip starts no timer of its own and the store writes nothing to disk", () => {
    expect(chipSrc).not.toMatch(/setInterval|requestAnimationFrame/);
    expect(storeSrc).not.toMatch(/setInterval|localStorage|sessionStorage|indexedDB|persist\(/);
    expect(storeSrc).not.toMatch(/headRevision|revision:/);             // decision 2: none sent
    const sc = cssSrc.slice(cssSrc.indexOf("/* Start chip"));
    expect(sc).not.toMatch(/transition|@keyframes|animation/);
    // mutants: a per-chip interval (one heartbeat rule); Hub data on disk; a client-pinned revision
  });
});
```

The fixture's five rows give three Starts and two in-place words: EP-160 is triage and unassigned
(assign, accept and Start), EP-166 and EP-167 are mine (`makeIssueHead`'s assignee is `DYLAN`,
`task-20.md:299-301`), EP-163 is Alice's and EP-150 is `in_review`. The bulk bar's accessible
roles (`toolbar`, the button's name) are task 44's (`task-44.md:141`, `:253`); if its part names
them another way, the queries follow it. That task 40's fixture world applies the `setAssignee`
write that EP-160 needs is **UNVERIFIED** (task 55's replica over task 40's world, gap 6).

#### Steps

- [ ] Write `useStart.test.ts`; it fails to import `./useStart`. Write `useStart.ts` part 1 until
      the `startRule` and `firstSign` tests pass. Mutants, one at a time, then revert: drop the
      `not_mine` branch (test 1); read Go's zero time as output (test 2).
- [ ] Write the store until the six store tests pass. Mutants: send the Start before `settled`
      (test 3); send the Start on a `Refusal` (test 4); skip `handle.undo()` on a refused Start
      (test 5); drop the `sending` guard (test 6); retry the undo on `first_write` (test 7); send
      the token on the first discard press (test 8).
- [ ] Write `StartChip.tsx`, `register.ts` and the `.sc-*` CSS; make the `IssueRow.tsx` and
      `WorkLens.tsx` edits; print the `Shift+R` row in `keymap.ts` only if task 53 did not.
- [ ] Write `StartChip.test.tsx` parts 2 and 3. Mutants: match `event.key` (part 2, test 1);
      bind on `document` (part 2, test 3); a `setInterval` in `StartChip.tsx` (part 3, test 7);
      `transition: all` on `.sc` (part 3, test 7).
- [ ] Tasks 40, 44, 53 and 56's suites still pass. Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/start/useStart.test.ts src/components/work/StartChip.test.tsx'
# expected: 2 files, 19 tests, all passing (8 rule/sign/store, 4 keys, 7 surfaces)
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 40, 44, 53, 56 and 60 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "setInterval|requestAnimationFrame|localStorage|headRevision" desktop/src/components/work/StartChip.tsx desktop/src/work/start/useStart.ts'
# expected: no output (decisions 2, 7 and 8; the source test holds it too, a review still reads the files)
~/.claude/tools/claudezsh/claudezsh 'git diff --stat -- desktop/src/api/work.ts'
# expected: no output (task 37's three calls are used as they are)
```

#### What this task does NOT claim

- **Any cost figure.** Local mode has no gateway meter (task 24's `cost: null`); the chip says so in
  words. Phase 6's gateway figure is its own delta.
- **Knowing when the agent first wrote a file.** SW.10 has no such read (gap 2); the chip learns
  `discard_only` from the control plane's `409 first_write` answer to a press, and says so then.
- **Delegate** (`Shift+O`) and any Start of a teammate's issue: W3 and OD-16.
- **`budget.ack` as met**: 10 s is when the first sign appears, a design value; no time to first
  output is measured or asserted here.
- **A chip that survives a desktop restart** (decision 8); the session and its worktree stay in
  Develop's verbs.
- **The route's own behaviour**: task 24 proves the control plane; these tests fake its client.
- **Visual acceptance**: the owner's eyes (CLAUDE.md, working agreements).
- **Parity**: closes L-47's *accept and Start* half (with task 42), L-39's solo half, L-181 and
  L-198's W1 half; serves L-36 and L-190 (closed in W3), as the scope says
  (`tasks-61-80.md:62-64`).

#### Gaps found while writing this part

1. The scope's `StartResult {…, startState, createdAt}` is not task 24's wire; nothing is added to
   `work.ts` (decision 1). The skeleton's file map (`:306`) lists only `StartChip.tsx` for task 61
   and omits `useStart.ts`, `register.ts` and the `IssueRow.tsx`/`WorkLens.tsx` edits.
2. SW.10 has no read of *"has the agent written a file yet"* (`task-24.md:92-115`). Asking is the
   only way the desktop learns it; a read would let the chip say it before the press.
3. That the control plane's `projectId` and the Hub's are one value is task 24's assumption
   (`task-24.md:256`); this task sends `head.projectId`.
4. `details` is `map[string]string` (`control-plane/internal/httpx/problem.go:33`), so `paths`
   travels newline-joined; task 24's part should print that encoding beside `ConfirmNeeded`.
5. `ApiError.code` is `ProblemCode` (`desktop/src/api/client.ts:40-48`), which lists none of task
   24's codes; the tests cast. Task 37, the owner of `work.ts`'s problem kinds, decides whether it
   widens.
6. Task 40's test seam is `setWorkSourceForTest(world)` in `desktop/src/work/fixtureSource.ts`,
   one argument (`task-40.md:112`, `:304`). Parts 2 and 3 import it from `./WorkLens` with a second
   `{me, work}` argument: at implementation they follow task 40's file and pass the fake client
   through whatever seam task 40 or 55 names for the lens's `work` client (**UNVERIFIED**), and
   whether the fixture world applies `setAssignee` is task 55's.
7. Task 53's `KEYMAP`, `registerVerb` and `RowContext` had no part on the tree at this writing; the
   verb name `issue.start` yields to task 53's.
