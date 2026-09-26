### Task 40: the Work lens: the list grouped by criterion, one agent line, coarse liveness

Stream **WD1**, about 3 hours, **provisional** (an estimate, not a measurement): this part was
written before task 39's review, and the review record's *Sizes of tasks 40 to 45*
(`docs/verification/2026-09-25-phase-w-prototype-review.md`, `task-39.md:585`) resizes it; the
plan's owner applies that size, not this part. Depends on task 39. Uses SW.8 (task 20's
`expectNoPrediction` and `workScenario()`), SW.12 (task 28's `StateGlyph`, task 29's `AgentGlyph`),
SW.13 (task 31's `ViewHeader`), SW.14 (task 36's `ThreadLine` and `ThreadNode`), SW.15 (task 37's
types, `workSlice` and `Lens = … | "work"`), and SW.16's read signature `useWork(selector)`, which
this part defines as an interface over a fixture reader so that it runs before task 55 lands.

Parity: closes the W1 parts of **L-101** (the list) and **L-102** (grouping, by criterion); gives
**L-186**'s one live line its local-mode form (W3 closes it); shows **L-3**'s alias with tasks 02
and 08. Design: 05 §4.1 (the drawing, `05-design.md:879-912`, and every bullet), §4.4 (C4, C10,
rule 3), §2.1 point 2; 07 D C1, D C3, D I10.

**What the reader sees** (05 §4.1's drawing, as W1 ships it: no cycles, no negotiation, plain sizes):

| Part of the lens | What it shows | Never |
|---|---|---|
| the header | `ViewHeader` with the title *Work*, the route `EP · <feature>  <branch>`, and a filter field | a count of agents, a progress figure |
| a group header | the criterion's words in sans, then `open · 3 issues` (its convergence word, a plain size) | `1 of 3`, a bar, a percentage |
| the last group | *Serves no criterion*, always last; absent only when every shown issue serves a criterion | a group by workflow state |
| a row | `ThreadNode`, the `StateGlyph` column, the alias in mono, the title, the assignee (a person) | an agent as assignee |
| the reader's own issue | one line under the row: `dylan · agent 1` in mono, a meaning in sans (`editing upload.go`), one `AgentGlyph`, and `live` or `quiet 2m` | a dot per agent, `+2`, a parsed terminal line |
| a teammate's issue | the person, and no dot and no line in W1 (*touching …* needs Fleet, W3) | their agents, their count |
| an unserving row | `not tied · chore` in the row's trailing slot (the thread state and its reason) | an empty trailing slot that reads as a missing fact |

**Decisions made here, so nobody re-derives them:**

1. **SW.16's read half is an interface defined here; task 55 implements it.** `desktop/src/work/source.ts`
   holds `WorkReader` (the one read surface every Work selector takes), `useWork(selector)` and
   `setWorkSource(reader)`. Before task 55 the only reader is `fixtureReader(world)` over task 20's
   `workScenario()`. Task 55's `Replica` is declared `extends WorkReader`, its `createReplica` calls
   `setWorkSource` at boot, and its `replica.ts` re-exports `useWork` from here, so the import path
   task 46 uses (`work/replica/replica`, `task-46.md:478`) holds once task 55 lands. One interface,
   two implementations, no second store.
2. **Every read the later parts already use is on `WorkReader`**, so none of them needs a second
   source: `issue(id)` (task 46), `byProject(projectId)`, `teamOfProject`, `labels`, `members`
   (task 46), `criteria(briefKey)` and `criterion(briefKey, criterionId)` (task 47,
   `task-47.md:383-391`), `briefKeyOf(projectId, branch)`, and `me()`, which settles *where the
   mount gets `me`* for tasks 45 and 46 (`task-45.md:216`, `task-46.md:990`): the reader knows its
   caller, as task 57's `callerId` shows (`task-57.md:356`).
3. **`useWork` re-renders a caller only when its selection changes.** It is `useSyncExternalStore`
   over `reader.subscribe`; the snapshot is the selection, compared with the previous one by
   `sameData` (structural over arrays, plain objects, `Map` and `Set`; `Object.is` for the rest),
   so an equal selection returns the previous reference **even when the selector is a new closure
   each render**. `grouped` is therefore referentially stable between reader changes, which task
   41's effect needs (`task-41.md:342-344`); a shallow compare would hand it a new array every
   render and loop.
4. **The group key is the thread key `${briefKey}#${criterionId}`**; *Serves no criterion* is the key
   `"none"` and sorts last. Criteria keep the brief's order. **An issue serving two criteria hangs
   from the first its head revision's `serves` names**; the other is in peek (task 60). 05 §4.1 does
   not draw the case, and a row drawn twice would give one id two cursors (decision 8): named in
   the gaps for the owner.
5. **Rows sort by issue number, ascending, and nothing else.** A state change never reorders a
   group, so the only movement the reader can see is a change of group or of view, which task 41
   holds. Display options (task 44) may add an ordering; the default is this.
6. **The default Work view** shows `todo`, `started`, `in_review`, `landed` and `done`
   (`WORK_DEFAULT_STATES`). `triage` is Triage's (task 42), `backlog` is Plan's (task 45), and
   `cancelled` and `duplicate` are departures (task 41's ghost, `EP-128 left this view · cancelled`).
   The selector returns those in `left` with their one word, the shape task 41's `GroupedIssues`
   reads (`task-41.md:100-105`).
7. **The lead agent, and its label, follow task 48's numbering** (`task-48.md:339-345`): every
   session in the store whose `issueRef` starts `issue:<id>@` is numbered by `createdAt`, exited
   ones included, so an agent keeps its number; the lead is the lowest-numbered one not `exited`;
   `whoLabel` is `<first name, lower case> · agent <n>`. Local mode's store holds only the reader's
   sessions, so "yours only" (05 §4.1) holds by construction. An issue on which the reader runs
   no live agent gets no line and no dot, whoever it is assigned to: a teammate's issue shows the
   person and nothing else.
8. **Identity is the `is-` id, everywhere.** React keys, `workSlice.cursor`, `workSlice.selection`
   and `data-issue-id` are ids; the alias from `DisplayAlias` (SW.1, `task-02.md:43`) is printed and
   nothing else. A project key change re-renders every alias and keeps focus and selection.
9. **The meaning is built from observed facts only** (`OD-8`). `agentMeaning(status, exitReason,
   newestPath)`: `running` with a path is `editing <basename>`, `running` without one is `running`,
   `starting` is `starting`, `idle` is `idle`, `exited` is `exited · <exitReason>` (or `exited`).
   `newestPath` is the lane's newest changed path from phase 3's lane index (P3-03's
   `internal/lanes`, the skeleton gap `tasks-21-40.md:726`). No desktop read of it exists on this
   tree (a grep of `desktop/src` for changed-path data finds none), so it is **UNVERIFIED until
   phase 3 merges** (rule H2): `AgentLine` passes `null` today and the meaning is the status word
   alone, which the scope allows ("else the status alone"). `agentMeaning` already takes and tests
   the path, so the merge is one argument at one call site, named in the gaps; no stub hook is
   written. `AgentLine`'s props take no text, so no caller can hand it a transcript line; a type
   test holds that.
10. **Liveness coarsens on the one clock.** `livenessWord(lastOutputAt, nowMs)`: under 10 s `live`,
    under a minute `quiet <1m`, then `quiet 2m`, `quiet 14m` (whole minutes, rounded down), and
    from an hour `quiet 1h`. `nowMs` is `useHeartbeat()`'s value (`hooks/heartbeat.ts:64`, epoch ms
    rounded to the second), so no row starts a timer; only `AgentLine` subscribes, never the row,
    and the printed word changes at most once a minute past the first ten seconds.
11. **Past 50 rows the list windows itself**, with no library: groups and rows flatten into one
    list of fixed-height items (`--row-h`, 28 px in `compact`, `docs/DESIGN-SYSTEM.md:90`), an agent
    line is its own item, and `windowOf(scrollTop, viewportH, itemH, count, overscan = 6)` picks the
    slice. At or under 50 rows every item mounts. Grid roles and roving focus are task 66's; this
    task sets `data-work-list` on the list element (task 53's `isListKeyTarget` finds it) and no
    `role="grid"`.
12. **The prototype leaves the lens, not the tree.** ⚑ `AppShell.tsx`'s `"work"` arm (task 38's
    `WorkArm`, `task-38.md:139-151`) now mounts `WorkMount`, with no flag. What the owner kept in
    task 39's review is built here as named in that record; what the owner cut is not. The
    `prototype/work/` directory stays reachable as task 38's stress screens, because Plan, the
    issue view and the Desk (tasks 45, 46, 62) are still compared against it; no task row deletes it
    yet (a gap).
13. **The filter field is local and plain.** A text field in the `ViewHeader`'s actions narrows rows
    to titles containing its text, case-folded. It is not search (task 58) and it is not saved. It
    is also the field task 41's test types `N` into (`task-41.md:472`).
14. **`setWorkSourceForTest(world)`** is the test seam tasks 41 to 45 name (`task-41.md:463`): it
    installs `fixtureReader(world)` and returns a `WorldHandle` whose `update(fn)` edits the world
    and bumps the version, which is how a test makes an issue arrive, move or leave.

**Files** (the skeleton's file map names the four components and `work.css`; it omits
`source.ts`, `fixtureSource.ts`, `workView.ts`, `windowing.ts` and `WorkMount`, a gap named below):
- `desktop/src/work/source.ts` — `WorkReader`, `useWork`, `setWorkSource`, `sameData` (new)
- `desktop/src/work/fixtureSource.ts` — `worldFromScenario`, `fixtureReader`, `setWorkSourceForTest`; tests only (new)
- `desktop/src/work/workView.ts` — the pure selectors and words (new)
- `desktop/src/work/windowing.ts` — `windowOf` (new)
- `desktop/src/work/workView.test.ts` — the pure tests (new)
- `desktop/src/components/work/WorkLens.tsx` — `WorkLens` and `WorkMount` (new)
- `desktop/src/components/work/CriterionGroup.tsx`, `IssueRow.tsx`, `AgentLine.tsx` (new)
- `desktop/src/components/work/work.css` — owns `.wk-*` (new)
- `desktop/src/components/work/WorkLens.test.tsx` — the surface tests (new)
- `desktop/src/components/AppShell.tsx` ⚑ — the `"work"` arm mounts `WorkMount` (modified)

#### Interfaces — complete: `desktop/src/work/source.ts`

```ts
// desktop/src/work/source.ts — task 40. SW.16's read half: the one interface every Work selector
// reads, and the hook. Task 55's Replica implements WorkReader and re-exports useWork; until it
// lands the reader is a fixture. Nothing here is persisted and nothing here fetches.
import { useCallback, useRef, useSyncExternalStore } from "react";
import type { IssueWithRevision, Label, User } from "../api/work";

export type ConvergenceWord = "settled" | "open" | "contested" | "drifting" | "uncovered";

/** A criterion as the Work layer shows it; task 47's CriterionView is this type (`task-47.md:377`). */
export interface CriterionView { id: string; text: string; word: ConvergenceWord }

/** A team member as a picker reads one (task 46 maps `.user`). The role words are UNVERIFIED
 *  against the Hub's membership record; task 55 fixes them from hub-contract.md. */
export interface TeamMemberView { user: User; role: string }

export interface WorkReader {
  /** Bumped once per applied change. */
  readonly version: number;
  subscribe(listener: () => void): () => void;
  /** The signed-in caller; null only before the first load. */
  me(): User | null;
  issue(id: string): IssueWithRevision | undefined;
  /** Every readable issue of the project, head plus head revision, in no promised order. */
  byProject(projectId: string): readonly IssueWithRevision[];
  teamOfProject(projectId: string): string | null;
  labels(teamId: string): readonly Label[];
  members(teamId: string): readonly TeamMemberView[];
  /** The brief the branch's thread hangs from (`brief:<id>@<rev>`); null when there is none. */
  briefKeyOf(projectId: string, branch: string | null): string | null;
  /** The brief's criteria in the brief's order, each with its convergence word. */
  criteria(briefKey: string): readonly CriterionView[];
  criterion(briefKey: string, criterionId: string): CriterionView | undefined;
}

let current: WorkReader | null = null;
const swapListeners = new Set<() => void>();

/** Installs the reader every useWork reads. Task 55 calls it once at boot with its Replica. */
export function setWorkSource(reader: WorkReader | null): void {
  current = reader;
  for (const l of [...swapListeners]) l();
}

export function workSource(): WorkReader | null {
  return current;
}

/** Called after every setWorkSource; WorkMount uses it to leave its not-connected state. */
export function onWorkSourceChange(listener: () => void): () => void {
  swapListeners.add(listener);
  return () => { swapListeners.delete(listener); };
}

/** Structural equality over the plain data a selector returns. Functions compare by identity. */
export function sameData(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
  if (Array.isArray(a)) {
    return Array.isArray(b) && a.length === b.length && a.every((x, i) => sameData(x, b[i]));
  }
  if (a instanceof Map) {
    if (!(b instanceof Map) || a.size !== b.size) return false;
    for (const [k, v] of a) if (!b.has(k) || !sameData(v, b.get(k))) return false;
    return true;
  }
  if (a instanceof Set) {
    if (!(b instanceof Set) || a.size !== b.size) return false;
    for (const v of a) if (!b.has(v)) return false;
    return true;
  }
  if (Object.getPrototypeOf(a) !== Object.prototype || Object.getPrototypeOf(b) !== Object.prototype) return false;
  const ka = Object.keys(a as object), kb = Object.keys(b as object);
  return ka.length === kb.length
    && ka.every((k) => sameData((a as Record<string, unknown>)[k], (b as Record<string, unknown>)[k]));
}

/** SW.16's read hook. Re-renders the caller only when sameData says its selection changed. */
export function useWork<T>(selector: (r: WorkReader) => T): T {
  const last = useRef<{ reader: WorkReader; version: number; value: T } | null>(null);
  const subscribe = useCallback((notify: () => void) => {
    let off = current?.subscribe(notify) ?? (() => {});
    const onSwap = () => { off(); off = current?.subscribe(notify) ?? (() => {}); notify(); };
    swapListeners.add(onSwap);
    return () => { swapListeners.delete(onSwap); off(); };
  }, []);
  const snapshot = (): T => {
    const r = current;
    if (r === null) throw new Error("useWork: no work source is installed (WorkMount, or setWorkSourceForTest)");
    const prev = last.current;
    const next = selector(r);
    if (prev !== null && sameData(prev.value, next)) {
      last.current = { reader: r, version: r.version, value: prev.value };
      return prev.value;
    }
    last.current = { reader: r, version: r.version, value: next };
    return next;
  };
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
```

`useSyncExternalStore` calls the snapshot twice per render in development and requires the same
reference both times; `sameData` gives it, because the second call's selection equals the first's.

#### Interfaces — complete: `desktop/src/work/fixtureSource.ts` (test support; no production import)

```ts
// desktop/src/work/fixtureSource.ts — task 40. Imported by tests only, so the fixtures never ship;
// a structural test (below) holds that no file outside *.test.* and src/test/ imports it.
import type { IssueWithRevision, Label, User } from "../api/work";
import { ALICE, DYLAN, makeIssueRevision, workScenario } from "../test/workFixtures"; // SW.8
import { setWorkSource, type CriterionView, type TeamMemberView, type WorkReader } from "./source";

/** The world a fixture reader serves. Plain data, so a test edits it with a spread. */
export interface WorkWorld {
  me: User;
  projectId: string;
  branch: string | null;
  briefKey: string;
  teamId: string;
  criteria: readonly { id: string; text: string }[];
  items: readonly IssueWithRevision[];
  labels: readonly Label[];
  members: readonly TeamMemberView[];
}

/** Task 20's scenario as a WorkWorld. A head with no revision in the scenario gets one whose
 *  title is `Fixture issue EP-n`, so every row has a title and no fixture string predicts. */
export function worldFromScenario(s = workScenario()): WorkWorld {
  const revs = Object.values(s.revisions);
  const items = s.issues.map((head) => ({
    head,
    revision: revs.find((r) => r.issueId === head.id && r.revision === head.headRevision)
      ?? makeIssueRevision({ issueId: head.id, revision: head.headRevision, content: {
        title: { text: `Fixture issue ${head.alias}`, author: "H" }, description: { text: "", author: "H" },
        serves: [], implements: [] } }),
  }));
  return {
    me: DYLAN, projectId: s.issues[0]!.projectId, branch: "feat/resumable-uploads", briefKey: s.briefKey,
    teamId: s.labels[0]!.teamId, criteria: s.criteria.map(({ id, text }) => ({ id, text })), items,
    labels: s.labels, members: [DYLAN, ALICE].map((user) => ({ user, role: "member" })),
  };
}

/** A reader over a world. The convergence word is the fixture's own rule (uncovered when no issue
 *  serves the criterion, else open); the real words are task 55's, from phase 4's reading. */
export function fixtureReader(get: () => WorkWorld, version: () => number, subscribe: WorkReader["subscribe"]): WorkReader {
  const view = (w: WorkWorld, id: string, text: string): CriterionView => ({
    id, text,
    word: w.items.some((i) => i.revision.content.serves.some((s) => s.criterionId === id)) ? "open" : "uncovered",
  });
  return {
    get version() { return version(); },
    subscribe,
    me: () => get().me,
    issue: (id) => get().items.find((i) => i.head.id === id),
    byProject: (pid) => get().items.filter((i) => i.head.projectId === pid),
    teamOfProject: (pid) => (pid === get().projectId ? get().teamId : null),
    labels: (tid) => get().labels.filter((l) => l.teamId === tid),
    members: (tid) => (tid === get().teamId ? get().members : []),
    briefKeyOf: (pid) => (pid === get().projectId ? get().briefKey : null),
    criteria: (bk) => (bk === get().briefKey ? get().criteria.map((c) => view(get(), c.id, c.text)) : []),
    criterion: (bk, cid) => {
      const c = bk === get().briefKey ? get().criteria.find((x) => x.id === cid) : undefined;
      return c ? view(get(), c.id, c.text) : undefined;
    },
  };
}

export interface WorldHandle {
  readonly reader: WorkReader;
  world(): WorkWorld;
  /** Replaces the world and notifies every useWork caller once: one applied change. */
  update(fn: (w: WorkWorld) => WorkWorld): void;
  /** Uninstalls the source. Call it in afterEach. */
  dispose(): void;
}

/** The seam tasks 41-45 name (`task-41.md:463`). Takes a WorkWorld or task 20's raw scenario. */
export function setWorkSourceForTest(world: WorkWorld | ReturnType<typeof workScenario> = worldFromScenario()): WorldHandle {
  let w: WorkWorld = "items" in world ? world : worldFromScenario(world);
  let v = 0;
  const listeners = new Set<() => void>();
  const reader = fixtureReader(() => w, () => v, (l) => { listeners.add(l); return () => { listeners.delete(l); }; });
  setWorkSource(reader);
  return {
    reader,
    world: () => w,
    update(fn) { w = fn(w); v += 1; for (const l of [...listeners]) l(); },
    dispose() { setWorkSource(null); listeners.clear(); },
  };
}
```

#### Interfaces — complete: `desktop/src/work/workView.ts`

```ts
// desktop/src/work/workView.ts — task 40. The Work lens's selectors and words. Pure: no React,
// no store, no DOM. Tasks 41-45 reuse issuesOf and groupByCriterion (`task-45.md:137-138`).
import type { IssueHead, IssueState, Priority, Serve, User } from "../api/work";
import type { Session, SessionStatus } from "../store/types";
import type { CriterionView, WorkReader } from "./source";

/** The default Work view (decision 6). Triage is task 42's, backlog is Plan's, and cancelled and
 *  duplicate leave the view as task 41's ghosts. */
export const WORK_DEFAULT_STATES: readonly IssueState[] = ["todo", "started", "in_review", "landed", "done"];
export const NO_CRITERION = "none";
export const threadKey = (briefKey: string, criterionId: string): string => `${briefKey}#${criterionId}`;
/** `brief:<id>@<rev>` → `brief:<id>`: a serve matches by brief and criterion id, not revision. */
const briefOf = (key: string): string => key.split("@")[0] ?? key;

/** A head with the two revision facts a list reads, flattened so a filter reads `h.state`. */
export interface WorkIssue extends IssueHead { title: string; serves: readonly Serve[] }

export function issuesOf(r: WorkReader, projectId: string): WorkIssue[] {
  return r.byProject(projectId).map(({ head, revision }) => ({
    ...head, title: revision.content.title.text, serves: revision.content.serves,
  }));
}

/** One row as the lens draws it. The first four fields are task 41's RowFact (`task-41.md:93-98`). */
export interface WorkRowFact {
  issueId: string; alias: string; state: IssueState; cause: string;
  title: string; priority: Priority; notProven: boolean; assignee: User | null;
  /** The row's trailing fact when it hangs from no criterion: "not tied · chore", or "". */
  trailing: string;
}

export interface WorkGroup { key: string; criterion: CriterionView | null; rows: readonly WorkRowFact[] }

/** Satisfies task 41's GroupedIssues (`groups` with `key` and `rows`, and `left`). */
export interface WorkViewModel {
  briefKey: string | null;
  groups: readonly WorkGroup[];
  left: ReadonlyMap<string, string>;
  shown: number;
}

function trailingOf(h: WorkIssue, hung: boolean): string {
  if (hung) return "";
  if (h.threadState.kind === "not_tied") return h.threadState.reason === "" ? "not tied" : `not tied · ${h.threadState.reason}`;
  if (h.threadState.kind === "project_level") return "project level";
  return h.serves.length > 0 ? "serves a criterion not in this brief" : "";
}

function rowFact(h: WorkIssue, hung: boolean, cause: string): WorkRowFact {
  return {
    issueId: h.id, alias: h.alias, state: h.state, cause, title: h.title, priority: h.priority,
    notProven: h.state === "done" && h.proof === "not_proven", assignee: h.assignee, trailing: trailingOf(h, hung),
  };
}

/** Criteria in the brief's order, then "none" last (05 §4.1). An issue hangs from the first
 *  criterion its serves names that is in this brief (decision 4). Rows by number (decision 5). */
export function groupByCriterion(
  issues: readonly WorkIssue[], briefKey: string | null, criteria: readonly CriterionView[],
  causeOf: (issueId: string) => string = () => "",
): WorkGroup[] {
  const rows = new Map<string, WorkRowFact[]>(criteria.map((c) => [c.id, []]));
  const none: WorkRowFact[] = [];
  for (const h of [...issues].sort((a, b) => a.number - b.number)) {
    const hit = briefKey === null ? undefined
      : h.serves.find((s) => briefOf(s.briefKey) === briefOf(briefKey) && rows.has(s.criterionId));
    (hit ? rows.get(hit.criterionId)! : none).push(rowFact(h, hit !== undefined, causeOf(h.id)));
  }
  const groups: WorkGroup[] = briefKey === null ? []
    : criteria.map((c) => ({ key: threadKey(briefKey, c.id), criterion: c, rows: rows.get(c.id)! }));
  if (none.length > 0) groups.push({ key: NO_CRITERION, criterion: null, rows: none });
  return groups;
}

function leftWord(h: WorkIssue, filtered: boolean): string {
  if (h.state === "cancelled" || h.state === "duplicate") return h.state;
  if (h.state === "triage" || h.state === "backlog") return `moved to ${h.state}`;
  return filtered ? "hidden by the filter" : "no longer in this view";
}

/** The lens's one selector. `causeOf` is task 41's (it formats causeLine); task 40 passes none. */
export function selectWorkView(
  r: WorkReader, projectId: string, branch: string | null, filter = "",
  causeOf?: (issueId: string) => string,
): WorkViewModel {
  const briefKey = r.briefKeyOf(projectId, branch);
  const criteria = briefKey === null ? [] : r.criteria(briefKey);
  const needle = filter.trim().toLocaleLowerCase();
  const all = issuesOf(r, projectId);
  const inState = (h: WorkIssue) => WORK_DEFAULT_STATES.includes(h.state);
  const matches = (h: WorkIssue) => needle === "" || h.title.toLocaleLowerCase().includes(needle);
  const shown = all.filter((h) => inState(h) && matches(h));
  const ids = new Set(shown.map((h) => h.id));
  const left = new Map(all.filter((h) => !ids.has(h.id)).map((h) => [h.id, leftWord(h, inState(h))] as const));
  return { briefKey, groups: groupByCriterion(shown, briefKey, criteria, causeOf), left, shown: shown.length };
}

/** "no issues", "1 issue", "3 issues". Task 41's groupSize adds "· N moving". Never "of". */
export function plainSize(n: number): string {
  return n === 0 ? "no issues" : n === 1 ? "1 issue" : `${n} issues`;
}

/** Task 48's numbering (decision 7): by createdAt, exited included; the lead is the lowest live. */
export function leadAgent(sessions: readonly Session[], issueId: string, me: User): { sessionId: string; whoLabel: string } | null {
  const on = sessions.filter((s) => s.issueRef.startsWith(`issue:${issueId}@`))
    .sort((x, y) => x.createdAt.localeCompare(y.createdAt));
  const n = on.findIndex((s) => s.status !== "exited");
  return n < 0 ? null : { sessionId: on[n]!.id, whoLabel: whoLabel(me, n + 1) };
}

export function whoLabel(me: User, n: number): string {
  return `${(me.displayName.split(" ")[0] ?? me.displayName).toLowerCase()} · agent ${n}`;
}

/** Decision 9: observed facts only. `newestPath` is the lane index's, or null (UNVERIFIED read). */
export function agentMeaning(status: SessionStatus, exitReason: string, newestPath: string | null): string {
  switch (status) {
    case "running": return newestPath ? `editing ${newestPath.split("/").pop()}` : "running";
    case "starting": return "starting";
    case "idle": return "idle";
    case "exited": return exitReason.trim() === "" ? "exited" : `exited · ${exitReason}`;
  }
}

export const LIVE_UNDER_MS = 10_000;

/** Decision 10. `live` under 10 s, then quiet in whole minutes (rounded down), then hours. */
export function livenessWord(lastOutputAt: string, nowMs: number): string {
  const age = nowMs - Date.parse(lastOutputAt);
  if (!Number.isFinite(age) || age < LIVE_UNDER_MS) return "live";
  if (age < 60_000) return "quiet <1m";
  if (age < 3_600_000) return `quiet ${Math.floor(age / 60_000)}m`;
  return `quiet ${Math.floor(age / 3_600_000)}h`;
}
```

An unparsable `lastOutputAt` reads `live` only because the built wire always carries one
(`store/types.ts`, `Session.lastOutputAt`, RFC 3339); the test pins the parsed cases, not this one.

#### Interfaces — complete: `desktop/src/work/windowing.ts`

```ts
// desktop/src/work/windowing.ts — task 40, decision 11. Fixed-height items; no library.
export const WINDOW_OVER = 50;
/** jsdom and a not-yet-laid-out pane measure 0; the window then assumes one screen of rows. */
export const FALLBACK_VIEWPORT_PX = 720;

export interface ItemWindow { start: number; end: number; padTop: number; padBottom: number }

export function windowOf(scrollTop: number, viewportH: number, itemH: number, count: number, overscan = 6): ItemWindow {
  const h = viewportH > 0 ? viewportH : FALLBACK_VIEWPORT_PX;
  const start = Math.max(0, Math.floor(scrollTop / itemH) - overscan);
  const end = Math.min(count, Math.ceil((scrollTop + h) / itemH) + overscan);
  return { start, end, padTop: start * itemH, padBottom: Math.max(0, count - end) * itemH };
}
```

#### Interfaces — complete: the components' props

```tsx
// desktop/src/components/work/WorkLens.tsx
export function WorkLens(props: { projectId: string; branch: string }): JSX.Element;
/** AppShell's "work" arm. Reads workSlice's projectId and branch; with no Work source installed
 *  (before task 55) it renders the not-connected line; with no project, the choose-a-feature line. */
export function WorkMount(): JSX.Element;

// desktop/src/components/work/CriterionGroup.tsx — null is "Serves no criterion".
export function CriterionGroup(props: {
  criterion: { id: string; text: string; word: "settled" | "open" | "contested" | "drifting" | "uncovered" } | null;
  issueIds: string[];
}): JSX.Element;

// desktop/src/components/work/IssueRow.tsx
export function IssueRow(props: { issueId: string; selected: boolean; cursor: boolean }): JSX.Element | null;

// desktop/src/components/work/AgentLine.tsx — no text prop: nothing can hand it a transcript line.
export function AgentLine(props: { sessionId: string; whoLabel: string }): JSX.Element | null;
```

These are the scope's props exactly (`tasks-21-40.md:683-687`). Each component reads its own
records through `useWork` or `useCockpit` by id, so a row re-renders when its issue changes and not
when a neighbour does. `CriterionGroup` reads its rows' order from the lens's `WorkViewModel`
(passed down as `issueIds`, already ordered), and the lens flattens groups for the window.

#### Reference implementation: `WorkLens.tsx` (the tests below are the contract; this is one way to pass them)

```tsx
// desktop/src/components/work/WorkLens.tsx — task 40. 05 §4.1's Work lens as W1 ships it.
import { useMemo, useRef, useState, useSyncExternalStore, type JSX } from "react";
import { useCockpit } from "../../store/store";
import { ViewHeader } from "../common/ViewHeader";                         // SW.13
import { onWorkSourceChange, useWork, workSource } from "../../work/source"; // SW.16, read half
import { leadAgent, selectWorkView, type WorkGroup } from "../../work/workView";
import { WINDOW_OVER, windowOf } from "../../work/windowing";
import { CriterionGroup, WorkViewContext } from "./CriterionGroup";
import "./work.css";

type Item = { kind: "head"; g: WorkGroup } | { kind: "row"; g: WorkGroup; id: string } | { kind: "line"; id: string };

export function WorkLens(props: { projectId: string; branch: string }): JSX.Element {
  const { projectId, branch } = props;
  const [filter, setFilter] = useState("");
  const view = useWork((r) => selectWorkView(r, projectId, branch, filter));
  const me = useWork((r) => r.me());
  const sessions = useCockpit((s) => s.sessions);
  const lined = useMemo(() => {
    const all = Object.values(sessions);
    return new Set(view.groups.flatMap((g) => g.rows).filter((row) => me !== null && leadAgent(all, row.issueId, me) !== null).map((row) => row.issueId));
  }, [sessions, view, me]);
  const items = useMemo<Item[]>(() => view.groups.flatMap((g) => [
    { kind: "head", g } as Item,
    ...g.rows.flatMap((row) => lined.has(row.issueId)
      ? [{ kind: "row", g, id: row.issueId } as Item, { kind: "line", id: row.issueId } as Item]
      : [{ kind: "row", g, id: row.issueId } as Item]),
  ]), [view, lined]);
  const listRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const rowPx = readRowPx(listRef.current);
  const windowed = view.shown > WINDOW_OVER;
  const w = windowed ? windowOf(scrollTop, listRef.current?.clientHeight ?? 0, rowPx, items.length)
    : { start: 0, end: items.length, padTop: 0, padBottom: 0 };
  // Groups with any item in the window render, each with only its windowed row ids.
  const visible = new Map<string, { g: WorkGroup; ids: string[] }>();
  for (const it of items.slice(w.start, w.end)) {
    if (it.kind === "line") continue;
    const e = visible.get(it.g.key) ?? { g: it.g, ids: [] };
    if (it.kind === "row") e.ids.push(it.id);
    visible.set(it.g.key, e);
  }
  return (
    <section className="wk-lens" aria-label="Work">
      <ViewHeader title="Work" route={null} badge={<span className="wk-branch">{branch}</span>}
        actions={<input className="wk-filter" type="search" aria-label="Filter issues" value={filter}
          onChange={(e) => setFilter(e.currentTarget.value)} />} />
      <div ref={listRef} className="wk-list" data-work-list="" onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)}>
        <WorkViewContext.Provider value={view}>
          <div style={{ blockSize: w.padTop }} aria-hidden="true" />
          {[...visible.values()].map(({ g, ids }) => (
            <CriterionGroup key={g.key} criterion={g.criterion} issueIds={ids} />
          ))}
          <div style={{ blockSize: w.padBottom }} aria-hidden="true" />
        </WorkViewContext.Provider>
        {view.groups.length === 0 ? <p className="wk-empty">No issues in this feature yet.</p> : null}
      </div>
    </section>
  );
}

function readRowPx(el: HTMLElement | null): number {
  const v = el ? Number.parseFloat(getComputedStyle(el).getPropertyValue("--row-h")) : Number.NaN;
  return Number.isFinite(v) && v > 0 ? v : 28;
}
```

`CriterionGroup` receives only the ids in the window; its header's size is the group's whole
count, which it reads from `WorkViewContext` (the lens's `WorkViewModel`, found by the criterion's
id), so a windowed group never reports a partial size and its props stay the scope's two. The
spacer `div`s are sized in `blockSize`, set once per scroll event, never animated.

#### Reference implementation: `WorkMount` (appended to `WorkLens.tsx`)

```tsx
/** AppShell's "work" arm (decision 12). No flag: the lens is the product now. */
export function WorkMount(): JSX.Element {
  const ready = useSyncExternalStore(onWorkSourceChange, () => workSource() !== null);
  const projectId = useCockpit((s) => s.work.projectId);
  const branch = useCockpit((s) => s.work.branch);
  if (!ready) return <p className="wk-empty">Work reads your team's issues from the Hub. This app is not connected to it yet.</p>;
  if (projectId === null || branch === null) return <p className="wk-empty">Open a feature to see its work.</p>;
  return <WorkLens projectId={projectId} branch={branch} />;
}
```

`workSlice.branch` is `string | null` (null is the project brief alone, `task-37.md:545-546`) and
`WorkLens` takes a `string`, as the scope fixes; a project-level Work view is Plan's (task 45), so a
null branch shows the choose-a-feature line rather than a second lens shape.

#### Reference implementation: `CriterionGroup.tsx`

```tsx
// desktop/src/components/work/CriterionGroup.tsx — task 40. One criterion, in words, and its rows.
import { createContext, useContext, type JSX } from "react";
import { useCockpit } from "../../store/store";
import { ThreadLine } from "../thread/ThreadLine";   // SW.14 (task 36; path per its part)
import { NO_CRITERION, plainSize, type WorkViewModel } from "../../work/workView";
import { IssueRow } from "./IssueRow";

export const WorkViewContext = createContext<WorkViewModel | null>(null);

export function CriterionGroup(props: {
  criterion: { id: string; text: string; word: "settled" | "open" | "contested" | "drifting" | "uncovered" } | null;
  issueIds: string[];
}): JSX.Element {
  const { criterion, issueIds } = props;
  const view = useContext(WorkViewContext);
  const cursor = useCockpit((s) => s.work.cursor);
  const selection = useCockpit((s) => s.work.selection);
  const group = view?.groups.find((g) => (criterion === null ? g.key === NO_CRITERION : g.criterion?.id === criterion.id));
  const size = plainSize(group?.rows.length ?? issueIds.length);
  const rows = issueIds.map((id) => (
    <IssueRow key={id} issueId={id} cursor={cursor === id} selected={selection.includes(id)} />
  ));
  return (
    <div className="wk-group" data-group={group?.key ?? NO_CRITERION}>
      <div className="wk-group-head">
        <span className="wk-group-words">{criterion === null ? "Serves no criterion" : criterion.text}</span>
        <span className="wk-group-size">
          {criterion === null ? null : <><span className="wk-conv" data-word={criterion.word}>{criterion.word}</span> · </>}
          <span className="wk-count">{size}</span>
        </span>
      </div>
      {criterion === null ? <div className="wk-rows">{rows}</div> : <ThreadLine criterionId={criterion.id}>{rows}</ThreadLine>}
    </div>
  );
}
```

*Serves no criterion* hangs from no line: the gutter is empty because no thread exists, which
is the fact (05 §4.1: the last group "is itself a fact worth seeing").

#### Reference implementation: `IssueRow.tsx` and `AgentLine.tsx`

```tsx
// desktop/src/components/work/IssueRow.tsx — task 40. One issue: node, state, alias, title, person.
import { useContext, type JSX } from "react";
import { useCockpit } from "../../store/store";
import { StateGlyph } from "../glyphs/StateGlyph";                 // SW.12
import { ThreadNode } from "../thread/ThreadNode";                 // SW.14
import { useWork } from "../../work/source";
import { leadAgent } from "../../work/workView";
import { AgentLine } from "./AgentLine";
import { WorkViewContext } from "./CriterionGroup";

export function IssueRow(props: { issueId: string; selected: boolean; cursor: boolean }): JSX.Element | null {
  const { issueId, selected, cursor } = props;
  const view = useContext(WorkViewContext);
  const me = useWork((r) => r.me());
  const sessions = useCockpit((s) => s.sessions);
  const openIssue = useCockpit((s) => s.openIssue);
  const moveCursor = useCockpit((s) => s.moveCursor);
  const g = view?.groups.find((x) => x.rows.some((r) => r.issueId === issueId));
  const row = g?.rows.find((r) => r.issueId === issueId);
  if (!g || !row) return null;
  const lead = me === null ? null : leadAgent(Object.values(sessions), issueId, me);
  const c = g.criterion;
  return (
    <div className="wk-item" data-issue-id={issueId}>
      <div className="wk-row" data-cursor={cursor ? "" : undefined} data-selected={selected ? "" : undefined}
        onClick={() => moveCursor(issueId)}>
        <span className="wk-node">
          {c !== null && view?.briefKey
            ? <ThreadNode criterionId={c.id} href={`#peek/${view.briefKey}/${c.id}`} label={c.text} /> : null}
        </span>
        <StateGlyph state={row.state} priority={row.priority} notProven={row.notProven} size={14} />
        <span className="wk-alias">{row.alias}</span>
        <button type="button" className="wk-title" onClick={() => openIssue(issueId)}>{row.title}</button>
        <span className="wk-person">{row.assignee?.displayName ?? ""}</span>
        {row.trailing === "" ? null : <span className="wk-trailing">{row.trailing}</span>}
      </div>
      {lead === null ? null : <AgentLine sessionId={lead.sessionId} whoLabel={lead.whoLabel} />}
    </div>
  );
}
```

```tsx
// desktop/src/components/work/AgentLine.tsx — task 40. The reader's lead agent, from observed
// facts only (OD-8). Imports no terminal, no attach registry, no transcript; a test holds it.
import type { JSX } from "react";
import { useHeartbeat } from "../../hooks/heartbeat";               // the one clock, :64
import { useCockpit } from "../../store/store";
import { AgentGlyph } from "../glyphs/AgentGlyph";                 // SW.12
import { agentMeaning, livenessWord } from "../../work/workView";

export function AgentLine(props: { sessionId: string; whoLabel: string }): JSX.Element | null {
  const s = useCockpit((st) => st.sessions[props.sessionId]);
  const now = useHeartbeat();
  if (s === undefined) return null;
  const meaning = agentMeaning(s.status, s.exitReason, null); // phase 3's newest path: decision 9
  return (
    <div className="wk-agent">
      <AgentGlyph status={s.status} size={14} label={`${props.whoLabel}: ${meaning}`} />
      <span className="wk-who">{props.whoLabel}</span>
      <span className="wk-meaning">{meaning}</span>
      <span className="wk-live">{livenessWord(s.lastOutputAt, now)}</span>
    </div>
  );
}
```

The row puts the person in `.wk-person` and never an agent: `assignee` is a `User` (SW.1) and the
agent is only ever on the line beneath. `AgentGlyph` gets `SessionStatus`, which is `AgentStatus`
less `failed`; `failed` is never passed here (task 29's rule: observed failure only).

#### `work.css` — complete (owns `.wk-*`; tasks 41-45 append)

```css
/* desktop/src/components/work/work.css — task 40. Tokens are task 26's and the built ones.
   No transition here: nothing on the list animates in this task (task 41 adds the one flash). */
.wk-lens { display: flex; flex-direction: column; min-block-size: 0; block-size: 100%; }
.wk-filter { font: inherit; font-size: var(--t-ui); color: var(--ink); background: transparent;
  border: 0; box-shadow: 0 0 0 1px var(--line); border-radius: var(--r-ctl); padding: 0 var(--s-2);
  min-block-size: calc(var(--row-h) - 4px); }
.wk-branch { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); }
.wk-list { flex: 1; overflow-y: auto; overflow-anchor: none; padding-block-end: var(--s-4); }
.wk-group-head { position: sticky; inset-block-start: 0; z-index: 1; display: flex;
  align-items: baseline; justify-content: space-between; gap: var(--s-3);
  min-block-size: var(--row-h); padding-inline: var(--s-3); background: var(--surface); }
.wk-group-words { font-size: var(--t-ui); color: var(--ink); }
.wk-group-size, .wk-count { font-size: var(--t-ui); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-row { display: grid; grid-template-columns: 16px 16px 7ch 1fr auto auto; align-items: center;
  gap: var(--s-2); min-block-size: var(--row-h); padding-inline: var(--s-3); border-radius: var(--r-ctl); }
.wk-row[data-cursor] { box-shadow: 0 0 0 1px var(--accent-ring); }
.wk-row[data-selected] { background: var(--accent-tint); }
.wk-alias { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-title { all: unset; cursor: default; font-size: var(--t-ui); color: var(--ink); overflow: hidden;
  text-overflow: ellipsis; white-space: nowrap; }
.wk-title:focus-visible { outline: 2px solid var(--accent-ring); outline-offset: 2px; }
.wk-person, .wk-trailing { font-size: var(--t-ui); color: var(--ink-2); white-space: nowrap; }
.wk-agent { display: grid; grid-template-columns: 16px auto 1fr auto; align-items: center; gap: var(--s-2);
  min-block-size: var(--row-h); padding-inline: calc(var(--s-3) + 16px + var(--s-2)) var(--s-3); }
.wk-who, .wk-live { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-meaning { font-size: var(--t-ui); color: var(--ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wk-empty { padding: var(--s-4) var(--s-3); color: var(--ink-2); font-size: var(--t-ui); }
@media (forced-colors: active) { .wk-row[data-cursor] { outline: 1px solid Highlight; box-shadow: none; } }
```

Tokens: `--t-ui`, `--t-fact`, `--s-*` and `--row-h` are task 26's (`task-31.md:732` lists them);
`--ink`, `--ink-2`, `--surface`, `--line`, `--accent-ring`, `--accent-tint`, `--r-ctl` and
`--mono` are on the built `desktop/src/styles/tokens.css` (checked by grep at this writing). The
one `all: unset` resets a button's own styles; it is not a transition. The cursor is a ring (elevation rule), not a wash; selected is a wash, so
the two read as two shapes (05 §4.1 D I1, which task 44 extends). `.wk-agent`'s start padding puts
the glyph under the alias column, so the line reads as the row's. Class names `.wk-row`,
`.wk-group-head`, `.wk-count` and `.wk-conv` are the ones tasks 41 and 45 already use
(`task-41.md:368`, `task-45.md:142`); `.wk-lead`, which task 44 names (`task-44.md:239`), is not
drawn here: task 44 adds it to `.wk-agent`.

#### Interfaces — complete: the delta on ⚑ `components/AppShell.tsx`

```tsx
// Replaces task 38's WorkArm (task-38.md:139-151) and its three imports: no flag, no Suspense.
import { WorkMount } from "./work/WorkLens";

    case "work":
      return <WorkMount />;
```

Task 38's `WorkArm` function, its `useFlags`, `Suspense` and `prototype/work/mount` imports leave
`AppShell.tsx`; `mount.ts`, `register.tsx` and the stress screens stay (decision 12). Task 38's own
AppShell test that asserted *nothing renders with the flag off* is replaced by this task's
*the arm mounts WorkMount* test, since the arm is no longer gated.

#### Tests to write first, part 1 — `desktop/src/work/workView.test.ts` (pure)

```ts
import { describe, expect, it } from "vitest";
import { DYLAN, WORK_IDS } from "../test/workFixtures";
import type { Session } from "../store/types";
import { worldFromScenario, fixtureReader } from "./fixtureSource";
import { sameData } from "./source";
import { agentMeaning, leadAgent, livenessWord, NO_CRITERION, plainSize, selectWorkView } from "./workView";
import { windowOf } from "./windowing";

const at = (ms: number) => new Date(Date.UTC(2026, 8, 25, 9) + ms).toISOString();
const readerOf = (w = worldFromScenario()) => fixtureReader(() => w, () => 0, () => () => {});
const view = (w = worldFromScenario(), filter = "") => selectWorkView(readerOf(w), w.projectId, w.branch, filter);
/** Every Session field longhand in the real file (store/types.ts:136-160); the ones read here: */
const sess = (over: Partial<Session>): Session => ({
  ...BASE_SESSION, id: "s-1", issueRef: `issue:${WORK_IDS.ep142}@3`, status: "running",
  createdAt: at(0), lastOutputAt: at(0), exitReason: "", ...over,
} as Session);

describe("the Work view model", () => {
  it("groups by criterion, in the brief's order, by default", () => {
    const v = view();
    expect(v.groups.slice(0, 2).map((g) => g.criterion?.id)).toEqual([WORK_IDS.criterionResume, WORK_IDS.criterionUpload]);
    expect(v.groups.flatMap((g) => g.rows).every((r) => typeof r.state === "string")).toBe(true);
  }); // kills grouping by workflow state (05 §4.4 C13): a state grouping has no criterion on any group
  it("puts Serves no criterion last", () => {
    const keys = view().groups.map((g) => g.key);
    expect(keys.at(-1)).toBe(NO_CRITERION);
    expect(keys.indexOf(NO_CRITERION)).toBe(keys.length - 1);
  }); // kills a none group sorted by name or by size
  it("orders rows by number and hangs a two-criterion issue once", () => {
    const w = worldFromScenario();
    const extra = { briefKey: w.briefKey, criterionId: WORK_IDS.criterionUpload };
    const two = { ...w, items: w.items.map((i) => i.head.id !== WORK_IDS.ep142 ? i : { ...i, revision: { ...i.revision,
      content: { ...i.revision.content, serves: [...i.revision.content.serves, extra] } } }) };
    const groups = view(two).groups;
    expect(groups.flatMap((g) => g.rows).filter((r) => r.issueId === WORK_IDS.ep142)).toHaveLength(1);
    for (const g of groups) {
      const n = g.rows.map((r) => Number(r.alias.slice(3)));
      expect(n).toEqual([...n].sort((a, b) => a - b));
    }
  }); // kills drawing an issue under every criterion it serves (one id, two cursors), and a state order
  it("sends cancelled and duplicate to left with their word", () => {
    const v = view();
    expect(v.left.get(WORK_IDS.ep168)).toBe("cancelled");
    expect(v.left.get(WORK_IDS.ep169)).toBe("duplicate");
    expect(v.left.get(WORK_IDS.ep160)).toBe("moved to triage");
  }); // kills dropping departures silently, which task 41's ghost needs
  it("writes only a plain size", () => {
    expect([plainSize(0), plainSize(1), plainSize(3)]).toEqual(["no issues", "1 issue", "3 issues"]);
  }); // kills "1 of 3" and any fraction
  it("returns equal data for an unchanged world, by sameData", () => {
    expect(sameData(view(), view())).toBe(true);
  }); // kills a selector whose output differs run to run (Map order, a Date), which would loop task 41
});
```

`BASE_SESSION` is a `const` in the test file holding every other `Session` field with a fixed
value (`store/types.ts:136-160`: `title`, `machineId`, `projectId`, `branchId`, `worktreeId`,
`harness`, `model`, `effort`, `permissionMode`, `openingPrompt`, `backend`, `isolation`,
`updatedAt`, `attachCount`, `attachPath`, `contextRef`), written out there; it carries no
meaning, so it is not repeated here. `WORK_IDS.criterionResume` is `makeCriterion()`'s default id
(`task-20.md:162`, 05 §5.3's *"the resume test passes"*).

#### Tests to write first, part 1 (continued) — the agent line's words and the window

```ts
describe("the agent line's words", () => {
  it("numbers agents as task 48 does and leads with the lowest live one", () => {
    const s = [sess({ id: "s-a", createdAt: at(0), status: "exited" }), sess({ id: "s-b", createdAt: at(1) }),
      sess({ id: "s-c", createdAt: at(2) })];
    expect(leadAgent(s, WORK_IDS.ep142, DYLAN)).toEqual({ sessionId: "s-b", whoLabel: "dylan · agent 2" });
  }); // kills renumbering after an exit, and kills leading with the newest agent
  it("draws no line when the reader runs no live agent on the issue", () => {
    expect(leadAgent([sess({ status: "exited" })], WORK_IDS.ep142, DYLAN)).toBeNull();
    expect(leadAgent([sess({ issueRef: `issue:${WORK_IDS.ep163}@2` })], WORK_IDS.ep142, DYLAN)).toBeNull();
  }); // kills a line for a dead agent, and kills matching by prefix of the id without "@"
  it("builds the meaning from observed facts only", () => {
    expect(agentMeaning("running", "", "control-plane/upload/upload.go")).toBe("editing upload.go");
    expect(agentMeaning("running", "", null)).toBe("running");
    expect(agentMeaning("exited", "process exited", null)).toBe("exited · process exited");
    expect(agentMeaning("idle", "", "a.go")).toBe("idle");
  }); // kills an invented meaning ("thinking", "almost done") and a path claimed while idle
  it("coarsens liveness", () => {
    const t0 = Date.parse(at(0));
    expect([5_000, 30_000, 130_000, 14 * 60_000 + 59_000, 2 * 3_600_000].map((d) => livenessWord(at(0), t0 + d)))
      .toEqual(["live", "quiet <1m", "quiet 2m", "quiet 14m", "quiet 2h"]);
  }); // kills ticking seconds ("quiet 130s") and rounding up (a "quiet 1m" at 30 s)
});

describe("the window", () => {
  it("mounts one screen plus overscan past fifty rows", () => {
    const w = windowOf(0, 0, 28, 500);
    expect(w.end - w.start).toBeLessThan(60);
    expect(w.padTop + (w.end - w.start) * 28 + w.padBottom).toBe(500 * 28);
  }); // kills rendering all 500, and kills spacers that make the scroll height lie
});
```

A structural test in the same file reads `src/components/work/AgentLine.tsx` as text and expects
no import from `terminal`, `attachRegistry`, `xterm` or `transcript` (kills a parsed PTY line,
`OD-8`), and reads every `.ts`/`.tsx` under `src/` except `*.test.*` and `src/test/` and expects none
to import `work/fixtureSource` (kills shipping the fixtures).

#### Tests to write first, part 2 — `desktop/src/components/work/WorkLens.test.tsx` (the lens)

Each renders `<WorkLens projectId={w.projectId} branch={w.branch!} />` after
`const h = setWorkSourceForTest()` and `useCockpit.setState(...)` for `sessions`, with
`h.dispose()` and `__resetHeartbeat()` in `afterEach`. Times use `vi.useFakeTimers({ now: at(0) })`.

| Test | Setup and assertion | Mutant it kills |
|---|---|---|
| `expectNoPrediction(container)` | the default world with one running agent on EP-142; SW.8's check passes on the whole lens | any `%`, bar, ETA or "N of M" |
| `groups by criterion by default` | the first group header's text is *the resume test passes*; no header is a state word | grouping by state (05 §4.4 C13) |
| `puts Serves no criterion last` | the last `.wk-group` has `data-group="none"` and reads *Serves no criterion* | a none group sorted among the criteria |
| `shows only a plain size in a group header` | every `.wk-count` matches `/^(no issues|1 issue|\d+ issues)$/` | `1 of 3`, a fraction |
| `draws one agent line and one glyph with three agents on the issue` | three running sessions on EP-142: one `.wk-agent`, one `[data-glyph^="agent:"]` in EP-142's item, text `dylan · agent 1` | a dot per agent, a count |
| `draws a teammate's issue with no glyph` | EP-163 (Alice's, started): its item has `Alice Example` and no `[data-glyph^="agent:"]` and no `+` | `+2`, a teammate's dot |
| `builds the meaning from observed facts only` | a type test: `// @ts-expect-error` on `<AgentLine sessionId="s-1" whoLabel="x" text="…" />` | a transcript-fed meaning prop |
| `coarsens liveness on the beat` | session `lastOutputAt = at(-5_000)`: `live`; `vi.advanceTimersByTime(125_000)`: `quiet 2m`; advancing 400 ms more leaves the text the same; `vi.getTimerCount()` after mounting three lines is the heartbeat's one | a per-row timer; ticking seconds |
| `keys rows by id, not alias` | move the cursor to EP-150, select EP-142; `h.update` renames every alias to `UP-n`; the cursor ring and selection stay on the same `data-issue-id`s and the text reads `UP-150` | the alias as a key or selection |
| `virtualises past fifty rows` | a world of 500 issues serving one criterion (`h.update`): fewer than 60 `.wk-item` mount; at 50 exactly, all 50 mount | rendering every row, or windowing small lists |
| `the arm mounts WorkMount` | ⚑ `AppShell.test.tsx`: `lens: "work"` with no flag set renders `.wk-lens` when a source is installed and the not-connected line when none is | a flag-gated arm; a blank arm |

`vi.getTimerCount()` counts the heartbeat's single `setInterval` (`heartbeat.ts:34`) and must equal
1 with three agent lines mounted; the `expectNoPrediction` check and the plain-size regex both run
on the 500-row world too.

#### Steps

- [ ] **Step 1: read on the sync base** task 39's review record and the kept/cut list; tasks 28,
      29, 31, 36 and 37's merged files, so every import above resolves or its name is corrected.
- [ ] **Step 2: write the tests** in `workView.test.ts` and `WorkLens.test.tsx`; run them and see
      each fail for its stated reason (no module, or the wrong grouping).
- [ ] **Step 3: write `source.ts`, `fixtureSource.ts`, `workView.ts`, `windowing.ts`**; the pure
      suite passes.
- [ ] **Step 4: write `CriterionGroup`, `IssueRow`, `AgentLine`, `WorkLens`, `work.css`**, then the
      ⚑ `AppShell.tsx` arm; the lens suite passes, and so do task 38's and 39's prototype suites.
- [ ] **Step 5: promote what the review kept** (the record's list; decision 12) and nothing it cut.
- [ ] **Step 6: commit** with `git add` on the new files and `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- work
# expected: 2 files (workView.test.ts, WorkLens.test.tsx), 22 tests, all passing
npm --prefix desktop test -- AppShell prototype
# expected: every suite passing, the replaced arm test included
npm --prefix desktop run typecheck
# expected: exit 0; the @ts-expect-error in the meaning test is consumed
git grep -n "fixtureSource" -- desktop/src ':!*.test.*' ':!desktop/src/test/'
# expected: no output
git grep -nE "localStorage|indexedDB|setInterval|setTimeout" -- desktop/src/components/work desktop/src/work
# expected: no output (the one clock is heartbeat.ts's; nothing here persists)
```

The test count is this part's: 12 pure (6 view, 4 words, 1 window, 1 structural pair counted as
one) plus 11 lens, less the AppShell test, which runs in the AppShell suite: 22 in the `work` run.

#### What this task does NOT claim

- `arrivals.hold`, `moves.hold` and departures, and `N` (task 41). The lens renders straight from
  `selectWorkView` here; task 41 puts its `HoldState` between them.
- Triage (42), the board (43), display options and the bulk bar (44), Plan (45), keys (53-54): the
  only interactions here are a click that moves the cursor and a title that opens the issue.
- Real Hub data. Until task 55 installs its `Replica`, the app shows the not-connected line and the
  tests run on task 20's world. Grid roles and roving focus (66). The Desk (62).
- The *touching …* line for a teammate (W3, Fleet), and the lane's newest path in the meaning
  (phase 3's read; decision 9). No measured density or frame budget: none is asserted here.

#### Gaps found while writing this part

1. **File map omissions:** `work/source.ts`, `work/fixtureSource.ts`, `work/workView.ts`,
   `work/windowing.ts`, and `WorkMount` in `WorkLens.tsx`.
2. **SW.16's read half is defined here, not in 55-58.** Task 55 must declare
   `Replica extends WorkReader`, call `setWorkSource`, and re-export `useWork`; task 46 imports
   `useWork` from `work/replica/replica`, which exists only after task 55 (task 46's own gap 6).
   `WorkReader` carries every read tasks 45-47 use, and `me()`, which answers tasks 45 and 46's
   *where does the mount get `me`*.
3. **Two criteria on one issue** is not drawn by 05 §4.1; decision 4 hangs it from the first. The
   owner may prefer another rule.
4. **The inference route** in the `ViewHeader` (`Global ▾` in 05's drawing) has no desktop
   source for a project in any part read here, so the lens passes `route={null}`.
5. **Phase 3's newest changed path** has no desktop read on this tree; the meaning is the status
   alone until it merges (decision 9), at one call site in `AgentLine.tsx`.
6. **Nobody deletes `prototype/work/`.** Task 38 says task 40 deletes it; tasks 45, 46 and 62 still
   compare against it, so this part keeps it and no task row removes it.
7. **The size is provisional** (task 39's review resizes 40-45); the skeleton's row still reads 3 h.
