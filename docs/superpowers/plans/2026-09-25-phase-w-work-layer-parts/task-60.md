### Task 60: peek on `Space`: `peek.span`, `peek.hunk`, `peek.person`

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work lens,
`IssueRow`, `AgentLine`, `work.css`) and task 55 (the replica: `useWork(selector)`, `status`).
Uses SW.8 (task 20's `expectNoPrediction`, `task-20.md:126`), SW.11 (`--dur-peek` 120ms and
`--dur-fast`, `task-26.md:84`), SW.12 (`StateGlyph`, `AgentGlyph`), SW.15 (task 37's `IssueHead`,
`CommentAnchor`, `task-37.md:121-160`) and task 51's comment start (`refAnchor`,
`CommentComposer`, `task-51.md:158-161`, `:262-268`). It touches task 43's `BoardView.tsx` so the
board peeks the same way. Task 40's, 53's and 55's parts were not on the tree when this was
written: their names here are their **scopes'** words (`tasks-21-40.md` for 40,
`tasks-41-60.md` for 53 and 55), **UNVERIFIED** until their parts land, and they win any
disagreement.

After this task a person holds `Space` on any row and sees what a decision about that row rests
on, without leaving the list and without marking anything seen.

**What the reader sees** (05 §4.3 `Space`, `05-design.md:997`; 05 §4.1; 02 C9,
`02-linear-design-language.md:307-312`; row 108 of 05's parity table, `05-design.md:242`):

- **An issue row, no agent of mine on it**: a panel opens beside the row (not over the list) with
  the issue's head: alias, `StateGlyph` and state word, assignee, priority as its notch, labels,
  and *Serves* with each criterion id in mono (`c-91e2`), the brief key beside it. Machine ids
  live here, never on the row (05 §4.1).
- **A row carrying my own agent line**: `peek.span`. The observed facts behind that line, each on
  one mono line with its time and its source: *09:14:02 · status running · session record*,
  *09:14:40 · last output · session record*, *exit at 10:02:11 · process exited · session record*,
  and, once phase 3's lane record is on the tree, the lane event and the files observed. Below:
  *open session* and *Comment on this*.
- **A landed row**: `peek.hunk`. The land commit (short sha, mono), then its hunks: path, the
  `@@` header, the lines in mono, each hunk followed by its span where one exists. *Comment on
  this* sits on each hunk.
- **A teammate's row** (assignee is not me): `peek.person`. *Alice · EP*: her issues in this
  project with state word and `stateReason`, and files only where an observed fact names them.
  Never a transcript line, a terminal, or an attach.
- `↑`/`↓` with peek open move the peek to the row above or below; the list does not scroll.
  `Space` tapped again, or `Esc`, closes it. Held `Space` peeks only while held.
- Just after a restart, while the replica loads, peek still opens on what the list shows, and the
  later facts arrive in place without moving what is already drawn.

**Decisions made here, so nobody re-derives them:**

1. **One pure choice, `peekFor(row, me)`.** The kind is decided from the row's data, in this order:
   `landed`/`done` with a `landCommit` gives `hunk`; a session of mine bound to the issue
   (`issueRef` starts `issue:<id>@`, task 19, the same test as `task-48.md:338-340`) gives `span`;
   an assignee who is not me gives `person`; anything else gives `issue`. It lives in
   `src/work/peek.ts` with no JSX so the order is tested without a DOM.
2. **Tap and hold, one timer and no per-row timer.** On `keydown` of `Space` (`event.code ===
   "Space"`, never `event.key`) the controller records `performance.now()`. On `keyup` it compares
   against `HOLD_MS = 300` (a design value, not a measurement): under it the press was a tap and the
   peek stays open; at or over it (or if a `repeat` keydown arrived) the press was a hold and the
   peek closes on release. A second tap while open closes. Nothing starts a `setInterval`.
3. **Keys only with list focus.** The controller is bound through task 53's list-focus gate
   (`isListKeyTarget`, `tasks-41-60.md`, task 53's scope): in an `input`, a `textarea`, a
   contenteditable or a `data-terminal-host`, `Space` types a space and peek never opens. This
   part adds no gate of its own.
4. **`↑`/`↓` move the peek, not the list.** With peek open, the arrows call `preventDefault` and
   move the peek anchor to the next row by the list's own order (the `rowIds` the lens passes);
   the list's cursor moves with it, as in Linear, but `scrollTop` is not written. When the next
   row is below the viewport, the peek panel follows it by `transform` only, clamped to the view.
5. **Peek marks nothing seen.** It reads, it never writes: no read cursor (task 123, W4), no
   `arrivals.hold` apply (task 41), no `mutate`. A test holds the read cursor where it was.
6. **Replica first, facts after.** The head comes from `useWork` synchronously; session facts come
   from the cockpit store (`useCockpit((s) => s.sessions)`, `desktop/src/store/types.ts:136-164`);
   hunks come through a `HunkSource` port that phase 3's diff record fills (**UNVERIFIED until
   merge**, rule H2). The first paint never waits on a network call; the time from `Space` to that
   paint is recorded as `budget.peek` (05 §6, `05-design.md:1104`), a record, not a verdict.
7. **`peek.person` has no session source at all.** Its component takes the head list and the
   person, and imports neither the cockpit's sessions nor `src/api/attachRegistry.ts`. A source
   test holds both (`OD-15`, `OD-8`).
8. **Comment on this** uses task 51's composer as it is: `CommentComposer({issueId, parentId:
   null, anchor: refAnchor(kind, ref), …})` inside the peek. The scope's working name
   `addComment({anchor})` yields to task 51's part. The `ref` for a span is `session:<id>@<rfc3339>`
   of the fact; for a hunk it is `commit:<sha>:<path>:<oldStart>`; both are stored verbatim
   (`task-51.md:158`).

**Files** (the skeleton's File map lists `Peek.tsx` for task 60, `:306`; it omits `peek.ts`, gap 1):

- `desktop/src/work/peek.ts` — `PeekKind`, `peekFor`, `spanFacts`, `personIssues`, `HunkSource` (new)
- `desktop/src/work/peek.test.ts` — the pure choices (new)
- `desktop/src/components/work/Peek.tsx` — `usePeekController`, `Peek`, the four bodies (new)
- `desktop/src/components/work/Peek.test.tsx` — keys, surfaces, source guards (new)
- `desktop/src/components/work/IssueRow.tsx` (task 40's) — `data-row-id`, and no machine id on the
  row (modified)
- `desktop/src/components/work/BoardView.tsx` (task 43's) — mounts the controller over its cards
  (modified)
- `desktop/src/components/work/WorkLens.tsx` (task 40's) — mounts the controller over the list
  (modified)
- `desktop/src/components/work/work.css` (task 40's) — the `.pk-*` rules, appended (modified)

#### Interfaces — complete: `desktop/src/work/peek.ts`, part 1: the kind and the span facts

```ts
// Peek chooses what a decision about a row rests on. No JSX, no fetch, no store import: the
// caller hands in the head, the reader and the sessions, so every rule is tested without a DOM.
// Nothing here reads a transcript or a terminal (OD-8); the span facts are record fields only.
import type { IssueHead, User } from "../api/work";
import type { Session } from "../store/types";

export type PeekKind = "issue" | "span" | "hunk" | "person";

/** Sessions of `me` bound to this issue, oldest first; the test task 48 uses (task-48.md:338). */
export function mySessionsOn(head: IssueHead, sessions: readonly Session[]): Session[] {
  return sessions
    .filter((s) => s.issueRef.startsWith(`issue:${head.id}@`))
    .sort((x, y) => x.createdAt.localeCompare(y.createdAt));
}

/** Decision 1's order: hunk, span, person, issue. */
export function peekFor(head: IssueHead, me: User, sessions: readonly Session[]): PeekKind {
  if ((head.state === "landed" || head.state === "done") && head.landCommit !== "") return "hunk";
  if (mySessionsOn(head, sessions).length > 0) return "span";
  if (head.assignee !== null && head.assignee.id !== me.id) return "person";
  return "issue";
}

/** One observed fact behind an agent line: when, what, and which record said so. */
export interface SpanFact {
  at: string;                 // RFC 3339, from the record; formatted only at display
  what: string;               // "status running", "last output", "exit · process exited"
  source: "session record" | "lane record";
  ref: string;                // the comment anchor ref: `session:<id>@<at>` (decision 8)
}

/** The session record's own fields, newest last. A zero time ("" or year 1) is no fact. */
export function spanFacts(s: Session): SpanFact[] {
  const ref = (at: string) => `session:${s.id}@${at}`;
  const out: SpanFact[] = [{ at: s.createdAt, what: "created", source: "session record", ref: ref(s.createdAt) }];
  if (isTime(s.updatedAt) && s.updatedAt !== s.createdAt) {
    out.push({ at: s.updatedAt, what: `status ${s.status}`, source: "session record", ref: ref(s.updatedAt) });
  }
  if (isTime(s.lastOutputAt)) {
    out.push({ at: s.lastOutputAt, what: "last output", source: "session record", ref: ref(s.lastOutputAt) });
  }
  if (s.status === "exited" && s.exitReason !== "") {
    out.push({ at: s.updatedAt, what: `exit · ${s.exitReason}`, source: "session record", ref: ref(s.updatedAt) });
  }
  return out.sort((a, b) => a.at.localeCompare(b.at));
}

/** Go's zero time marshals as "0001-01-01T00:00:00Z" (registry/session.go:99's LastOutputAt). */
function isTime(t: string): boolean {
  return t !== "" && !t.startsWith("0001-");
}
```

`Session.issueRef` is task 19's addition to `desktop/src/store/types.ts` (`task-19.md:70`); the
type on the tree today (`types.ts:136-164`) has `contextRef` and not yet `issueRef`.

#### Interfaces — complete: `peek.ts`, part 2: the person and the hunks

```ts
/** One line of `peek.person`: an issue, its state and reason. No session field exists here. */
export interface PersonIssue {
  alias: string; title: string; state: IssueHead["state"]; stateReason: string;
  files: string[];            // only paths an observed fact named (a lane record); [] in W1
}

/** That person's issues in this project, open first, then newest update. */
export function personIssues(
  heads: readonly IssueHead[], titleOf: (id: string) => string, personId: string, projectId: string,
): PersonIssue[] {
  const closed = new Set<IssueHead["state"]>(["done", "cancelled", "duplicate"]);
  return heads
    .filter((h) => h.projectId === projectId && h.assignee?.id === personId)
    .sort((a, b) => Number(closed.has(a.state)) - Number(closed.has(b.state))
      || b.updatedAt.localeCompare(a.updatedAt))
    .map((h) => ({ alias: h.alias, title: titleOf(h.id), state: h.state, stateReason: h.stateReason, files: [] }));
}

/** One hunk of the land commit. `spanRef` is the session fact that wrote it, where one exists. */
export interface LandHunk {
  path: string; oldStart: number; oldLines: number; newStart: number; newLines: number;
  lines: string[];            // each starts " ", "+" or "-", as git prints them
  spanRef: string | null;
}

/** Phase 3's diff record fills this (UNVERIFIED until merge, rule H2). A rejection is shown as
 *  its reason, in place; it is never retried by a timer. */
export interface HunkSource { hunksOf(landCommit: string): Promise<LandHunk[]> }

let hunkSource: HunkSource = {
  hunksOf: () => Promise.reject(new Error("the land diff is not on this build yet")),
};
export function setHunkSource(s: HunkSource): void { hunkSource = s; }
export function getHunkSource(): HunkSource { return hunkSource; }

/** Decision 8: the anchor refs, stored verbatim by task 12. */
export function hunkRef(sha: string, h: LandHunk): string {
  return `commit:${sha}:${h.path}:${h.oldStart}`;
}
```

`LandHunk.lines` is display text only; peek never derives meaning from it and never feeds it to
a model. The default `HunkSource` rejects with a plain reason, so before phase 3 lands a landed row
peeks the commit sha and *"the land diff is not on this build yet"*: a fact, not a placeholder that
pretends to be data.

`IssueHead` carries no `title` in task 37's type (`task-37.md:121-128`; task 58's gap 2): the title
comes from the head revision, `r.get(id)?.revision.content.title.text`, which the caller passes as
`titleOf`.
