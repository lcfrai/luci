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
- `desktop/src/components/work/Peek.tsx` — `usePeekController`, `Peek`, three of the bodies (new)
- `desktop/src/components/work/PeekPerson.tsx` — `peek.person` alone, so its source test means
  something (decision 7) (new)
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

#### Interfaces — complete: `desktop/src/components/work/Peek.tsx`, part 1: the controller

```tsx
// Space tap opens and closes; Space held peeks while held; arrows move the peek, never the list.
// Bound on the list element, gated by task 53's isListKeyTarget, so a text field or a terminal
// host types a space. No setInterval, no mutate, no read-cursor write (decisions 2, 4, 5).
import { useCallback, useRef, useState, type KeyboardEvent } from "react";
import { isListKeyTarget } from "../../work/keys/keymap";     // task 53 (scope words, UNVERIFIED)

export const HOLD_MS = 300;                                    // a design value, not a measurement

export interface PeekController {
  peekId: string | null;
  openedAt: number;                                            // performance.now() of the press
  onKeyDown(e: KeyboardEvent<HTMLElement>): void;
  onKeyUp(e: KeyboardEvent<HTMLElement>): void;
  close(): void;
}

export function usePeekController(p: {
  rowIds: readonly string[]; cursorId: string | null; onCursor(id: string): void;
}): PeekController {
  const [peekId, setPeekId] = useState<string | null>(null);
  const down = useRef<{ at: number; repeated: boolean; wasOpen: boolean } | null>(null);
  const openedAt = useRef(0);
  const close = useCallback(() => { setPeekId(null); down.current = null; }, []);

  const onKeyDown = useCallback((e: KeyboardEvent<HTMLElement>) => {
    if (!isListKeyTarget(e.target)) return;                    // Space in a field types a space
    if (e.code === "Space") {
      e.preventDefault();
      if (e.repeat) { if (down.current) down.current.repeated = true; return; }
      const wasOpen = peekId !== null;
      down.current = { at: performance.now(), repeated: false, wasOpen };
      if (!wasOpen && p.cursorId !== null) { openedAt.current = down.current.at; setPeekId(p.cursorId); }
      return;
    }
    if (peekId === null) return;
    if (e.code === "Escape") { e.preventDefault(); close(); return; }
    if (e.code === "ArrowDown" || e.code === "ArrowUp") {
      e.preventDefault();                                      // the list never scrolls under peek
      const i = p.rowIds.indexOf(peekId);
      const j = e.code === "ArrowDown" ? Math.min(i + 1, p.rowIds.length - 1) : Math.max(i - 1, 0);
      const next = p.rowIds[j];
      if (next !== undefined && next !== peekId) { setPeekId(next); p.onCursor(next); }
    }
  }, [peekId, p, close]);

  const onKeyUp = useCallback((e: KeyboardEvent<HTMLElement>) => {
    if (e.code !== "Space" || down.current === null) return;
    const d = down.current;
    down.current = null;
    const held = d.repeated || performance.now() - d.at >= HOLD_MS;
    if (d.wasOpen) { close(); return; }                        // a second tap closes
    if (held) close();                                         // a hold peeks only while held
  }, [close]);

  return { peekId, openedAt: openedAt.current, onKeyDown, onKeyUp, close };
}
```

`p.onCursor` moves the list's cursor with the peek (decision 4), and the lens owns scrolling: it
calls `scrollIntoView` only on its own `j`/`k`, never on `onCursor` from peek, which the
`scrollTop` test holds. `isListKeyTarget(target: EventTarget | null): boolean` is task 53's
signature as its scope words it; if its part names it otherwise, its name wins.

#### Interfaces — complete: `Peek.tsx`, part 2: the panel, the issue body and the span body

```tsx
import { useState, type JSX } from "react";
import type { CommentAnchor, IssueHead, User } from "../../api/work";
import { useCockpit } from "../../store/store";
import { useWork } from "../../work/replica/replica";           // task 55 (scope words, UNVERIFIED)
import { mySessionsOn, peekFor, spanFacts, type PeekKind } from "../../work/peek";
import { StateGlyph } from "../glyphs/StateGlyph";
import { AgentGlyph } from "../glyphs/AgentGlyph";
import { CommentComposer } from "../issue/CommentComposer";
import { refAnchor } from "../issue/commentModel";

/** Wall-clock time of a record's timestamp, in the reader's zone, for a mono fact. */
export function clock(at: string): string {
  return new Date(at).toLocaleTimeString("en-GB", { hour12: false });
}

export function Peek(p: {
  issueId: string; me: User; people: readonly User[]; onOpenSession(sessionId: string): void;
}): JSX.Element | null {
  const rec = useWork((r) => r.get(p.issueId));
  const sessions = useCockpit((s) => s.sessions);
  const [anchor, setAnchor] = useState<CommentAnchor | null>(null);
  if (!rec) return null;
  const head = rec.head;
  const all = Object.values(sessions);
  const kind: PeekKind = peekFor(head, p.me, all);
  return (
    <aside className="pk" data-peek-kind={kind} aria-label={`Peek: ${head.alias}`}>
      <header className="pk-head">
        <span className="pk-id">{head.alias}</span>
        <StateGlyph state={head.state} priority={head.priority} notProven={head.proof === "not_proven"} />
        <span className="pk-title">{rec.revision.content.title.text}</span>
      </header>
      {kind === "issue" && <IssueBody head={head} serves={rec.revision.content.serves} />}
      {kind === "span" && (
        <SpanBody sessions={mySessionsOn(head, all)} onOpen={p.onOpenSession}
          onComment={(ref) => setAnchor(refAnchor("span", ref))} />
      )}
      {kind === "hunk" && <HunkBody sha={head.landCommit} onComment={(ref) => setAnchor(refAnchor("hunk", ref))} />}
      {kind === "person" && head.assignee && <PersonBody person={head.assignee} projectId={head.projectId} />}
      {anchor && (
        <CommentComposer issueId={head.id} parentId={null} anchor={anchor} people={p.people}
          label={`Comment on ${head.alias}`} onPosted={() => setAnchor(null)} />
      )}
    </aside>
  );
}

function IssueBody(p: { head: IssueHead; serves: readonly { briefKey: string; criterionId: string }[] }): JSX.Element {
  const { head } = p;
  return (
    <dl className="pk-fields">
      <dt>Assignee</dt><dd>{head.assignee?.displayName ?? "No one"}</dd>
      <dt>Labels</dt><dd>{head.labels.length ? head.labels.join(", ") : "None"}</dd>
      <dt>Serves</dt>
      <dd>{p.serves.length === 0 ? "No criterion" : p.serves.map((s) => (
        <span key={s.criterionId} className="pk-fact">{s.criterionId} <span className="pk-src">{s.briefKey}</span></span>
      ))}</dd>
    </dl>
  );
}

function SpanBody(p: {
  sessions: readonly import("../../store/types").Session[]; onOpen(id: string): void; onComment(ref: string): void;
}): JSX.Element {
  return (
    <div className="pk-spans">
      {p.sessions.map((s) => (
        <section key={s.id} className="pk-span">
          <p className="pk-who"><AgentGlyph status={s.status} /> <span className="pk-id">{s.id}</span></p>
          <ol className="pk-facts">
            {spanFacts(s).map((f) => (
              <li key={f.ref + f.what} className="pk-fact">
                <time dateTime={f.at}>{clock(f.at)}</time> · {f.what} · <span className="pk-src">{f.source}</span>
                <button type="button" className="pk-link" onClick={() => p.onComment(f.ref)}>Comment on this</button>
              </li>
            ))}
          </ol>
          <button type="button" className="pk-link" onClick={() => p.onOpen(s.id)}>open session</button>
        </section>
      ))}
    </div>
  );
}
```

`Peek.tsx` also imports `PeekPerson` from `./PeekPerson` (part 4) and `useEffect` for `HunkBody`
(part 3); the import lines above list what parts 2 and 3 need, and the part-3 block adds its own.

#### Interfaces — complete: `Peek.tsx`, part 3: the hunk body

```tsx
import { useEffect } from "react";
import { getHunkSource, hunkRef, type LandHunk } from "../../work/peek";
import { PeekPerson } from "./PeekPerson";

type HunkState = { sha: string; hunks: LandHunk[] | null; reason: string };

function HunkBody(p: { sha: string; onComment(ref: string): void }): JSX.Element {
  const [st, setSt] = useState<HunkState>({ sha: p.sha, hunks: null, reason: "" });
  useEffect(() => {
    let live = true;                                           // a newer peek replaces this answer
    getHunkSource().hunksOf(p.sha).then(
      (hunks) => { if (live) setSt({ sha: p.sha, hunks, reason: "" }); },
      (err: unknown) => { if (live) setSt({ sha: p.sha, hunks: null, reason: String((err as Error).message ?? err) }); },
    );
    return () => { live = false; };
  }, [p.sha]);
  return (
    <div className="pk-hunks">
      <p className="pk-fact">land commit <span className="pk-id">{p.sha.slice(0, 12)}</span></p>
      {st.reason !== "" && <p className="pk-fact pk-reason">{st.reason}</p>}
      {st.hunks?.map((h) => (
        <section key={hunkRef(p.sha, h)} className="pk-hunk">
          <p className="pk-fact">
            <span className="pk-path">{h.path}</span>{" "}
            @@ -{h.oldStart},{h.oldLines} +{h.newStart},{h.newLines} @@
          </p>
          <pre className="pk-diff">{h.lines.join("\n")}</pre>
          {h.spanRef !== null && <p className="pk-fact pk-src">span {h.spanRef}</p>}
          <button type="button" className="pk-link" onClick={() => p.onComment(hunkRef(p.sha, h))}>
            Comment on this
          </button>
        </section>
      ))}
    </div>
  );
}
```

While the hunks are on their way the body shows the commit line only: no spinner, no
placeholder rows (the one heartbeat is the only liveness, and a peek is not a live process). The
body never measures or predicts how long the diff takes.

#### Interfaces — complete: `desktop/src/components/work/PeekPerson.tsx`

```tsx
// peek.person: a teammate's issues in this project, their states and reasons. It imports no
// session, no attach registry, no terminal and no transcript (OD-15, OD-8); a source test holds it.
import type { JSX } from "react";
import type { User } from "../../api/work";
import { useWork } from "../../work/replica/replica";           // task 55 (scope words, UNVERIFIED)
import { personIssues } from "../../work/peek";
import { StateGlyph } from "../glyphs/StateGlyph";

export function PeekPerson(p: { person: User; projectId: string }): JSX.Element {
  const rows = useWork((r) => personIssues(
    r.byProject(p.projectId).map((x) => x.head),
    (id) => r.get(id)?.revision.content.title.text ?? "",
    p.person.id, p.projectId,
  ));
  return (
    <div className="pk-person">
      <p className="pk-who">{p.person.displayName}</p>
      {rows.length === 0 && <p className="pk-fact">No issues assigned in this project.</p>}
      <ul className="pk-list">
        {rows.map((r) => (
          <li key={r.alias} className="pk-row">
            <span className="pk-id">{r.alias}</span> <StateGlyph state={r.state} priority="none" notProven={false} />
            <span className="pk-title">{r.title}</span>
            {r.stateReason !== "" && <span className="pk-reason">{r.stateReason}</span>}
            {r.files.length > 0 && <span className="pk-path">{r.files.join(", ")}</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

`useWork` compares selections to decide a re-render (task 55's scope, `tasks-41-60.md:483`);
`personIssues` returns a new array each call, so the part's implementer passes task 55's
equality helper if its part defines one, else memoises on `r.cursor` (**UNVERIFIED**, gap 4).

#### Interfaces — complete: the edits to `WorkLens.tsx`, `IssueRow.tsx` and `BoardView.tsx`

- **`WorkLens.tsx`** (task 40's). `const peek = usePeekController({rowIds, cursorId, onCursor:
  setCursor})`, where `rowIds` is the flat order of `hold.shown` (task 41), so a held arrival is
  never peeked before `N` shows it. The list element's existing `onKeyDown` (task 41's `N`) calls
  `peek.onKeyDown(e)` first and returns when `e.defaultPrevented`; `onKeyUp={peek.onKeyUp}` is
  added. When `peek.peekId !== null` the lens renders `<Peek issueId={peek.peekId} me={me}
  people={people} onOpenSession={openSession}/>` in a `.pk-host` sibling of the list, positioned
  beside the anchored row by `transform: translateY(<row top - list top>px)`, clamped to the lens.
  `openSession` is the three store actions task 48 uses (`task-48.md:329-333`). `me` and `people`
  are what the lens already holds for its assignee picker.
- **`IssueRow.tsx`** (task 40's). The row element gains `data-row-id={head.id}` (the peek host
  reads its `offsetTop`). No criterion id, no session id and no brief key renders on the row: the
  row's `Serves` is the criterion's words, and the machine id is peek's (05 §4.1).
- **`BoardView.tsx`** (task 43's, `task-43.md:125-130`). The board's focused card list gets the
  same controller with `rowIds` in column-then-card order, the same `.pk-host`, and the same
  `onKeyDown` chain before its own keys. The board adds no peek rule of its own.

#### Interfaces — complete: appended to `desktop/src/components/work/work.css`

```css
/* Peek: a panel beside the anchored row. Elevation is the ring plus layered shadow of --e-raised
   (tokens.css:121); radius is the card step. Motion is opacity and transform only (02 C7). */
.pk-host { position: absolute; inset-inline-end: var(--s-3); inset-block-start: 0; width: 360px;
  pointer-events: none; }
.pk { pointer-events: auto; background: var(--surface); box-shadow: var(--e-raised);
  border-radius: var(--r-card); padding: var(--s-3); max-block-size: 60vh; overflow: auto;
  animation: pk-in var(--dur-peek) var(--ease) both; }
@keyframes pk-in { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: none; } }
.pk-head { display: flex; gap: var(--s-2); align-items: center; }
.pk-title { font: var(--t-ui) / var(--t-ui-lh) var(--sans); color: var(--ink); }
.pk-id, .pk-fact, .pk-path, .pk-diff, .pk-src, .pk-fact time {
  font-family: var(--mono); font-size: var(--t-fact); line-height: var(--t-fact-lh);
  font-variant-numeric: tabular-nums; }
.pk-src, .pk-reason { color: var(--ink-2); }             /* never --ink-3 on a fact (Global constraints) */
.pk-diff { white-space: pre; overflow-x: auto; margin: 0; }
.pk-link { background: none; border: 0; padding: 0; color: var(--ink); text-decoration: underline;
  cursor: pointer; }                                       /* --accent-ink is text ON a fill (tokens.css:66) */
@media (prefers-reduced-motion: reduce) { .pk { animation: none; } }
```

Token names are the tree's (`tokens.css:110-152`) and task 26's (`--t-fact`, `--t-fact-lh`,
`--dur-peek`). `--sans` is the tree's (`tokens.css:151`); `--s-*` and `--t-ui-lh` are task 26's
(`task-26.md:109`, `:420`); where a name differs, task 26 wins.

#### Tests to write first, part 1 — `desktop/src/work/peek.test.ts` (the pure choices)

```ts
import { describe, expect, it } from "vitest";
import { makeIssueHead, ALICE, DYLAN, WORK_IDS } from "../test/workFixtures";
import type { Session } from "../store/types";
import { mySessionsOn, peekFor, personIssues, spanFacts } from "./peek";

const ZERO = "0001-01-01T00:00:00Z";
function session(over: Partial<Session> = {}): Session {
  return {
    id: "s-3f2a91c0", title: "", machineId: "m-local", projectId: "p-1", branchId: "b-1",
    worktreeId: "w-1", harness: "claude-code", model: "m", effort: "medium", permissionMode: "default",
    openingPrompt: "", status: "running", exitReason: "", backend: "docker", isolation: "container",
    createdAt: "2026-09-25T09:14:02Z", updatedAt: "2026-09-25T09:14:02Z", lastOutputAt: ZERO,
    attachCount: 0, attachPath: "/v1/sessions/s-3f2a91c0/attach", contextRef: "",
    issueRef: `issue:${WORK_IDS.ep142}@3`, ...over,
  } as Session;                                   // issueRef is task 19's field
}

describe("peekFor", () => {
  it("a landed row with a land commit peeks the hunks, even with my session on it", () => {
    const head = makeIssueHead({ state: "landed", landCommit: "a".repeat(40) });
    expect(peekFor(head, DYLAN, [session()])).toBe("hunk");
    // mutant: span before hunk, so a landed row shows the session instead of what landed
  });
  it("my session bound to the issue peeks the span; a session on another issue does not", () => {
    const head = makeIssueHead({ state: "started" });
    expect(peekFor(head, DYLAN, [session()])).toBe("span");
    expect(peekFor(head, DYLAN, [session({ issueRef: `issue:${WORK_IDS.ep150}@1` })])).toBe("issue");
    // mutant: any session in the project counts as this row's agent
  });
  it("a teammate's row peeks the person; my own unassigned row peeks the record", () => {
    expect(peekFor(makeIssueHead({ assignee: ALICE, state: "started" }), DYLAN, [])).toBe("person");
    expect(peekFor(makeIssueHead({ assignee: null }), DYLAN, [])).toBe("issue");
    // mutant: an unassigned row treated as someone else's
  });
});

describe("spanFacts", () => {
  it("never makes a fact from Go's zero time, and states the exit reason verbatim", () => {
    const f = spanFacts(session({ status: "exited", exitReason: "process exited", updatedAt: "2026-09-25T10:02:11Z" }));
    expect(f.map((x) => x.what)).toEqual(["created", "status exited", "exit · process exited"]);
    expect(f.every((x) => !x.at.startsWith("0001-"))).toBe(true);
    // mutant: "last output" at 0001-01-01 shown as a fact
  });
  it("every fact carries its record source and a verbatim session ref", () => {
    const f = spanFacts(session({ lastOutputAt: "2026-09-25T09:14:40Z" }));
    expect(f.find((x) => x.what === "last output")?.ref).toBe("session:s-3f2a91c0@2026-09-25T09:14:40Z");
    expect(new Set(f.map((x) => x.source))).toEqual(new Set(["session record"]));
  });
});

describe("personIssues", () => {
  it("lists only that person's issues in that project, open before closed", () => {
    const heads = [
      makeIssueHead({ id: "is-1", alias: "EP-1", assignee: ALICE, state: "done", updatedAt: "2026-09-25T12:00:00Z" }),
      makeIssueHead({ id: "is-2", alias: "EP-2", assignee: ALICE, state: "started", updatedAt: "2026-09-25T09:00:00Z" }),
      makeIssueHead({ id: "is-3", alias: "EP-3", assignee: DYLAN, state: "started" }),
      makeIssueHead({ id: "is-4", alias: "UP-4", assignee: ALICE, projectId: "p-other" }),
    ];
    const rows = personIssues(heads, () => "", ALICE.id, heads[0]!.projectId);
    expect(rows.map((r) => r.alias)).toEqual(["EP-2", "EP-1"]);
    expect(rows.every((r) => r.files.length === 0)).toBe(true);    // no observed file fact in W1
    // mutants: another project's issue listed; newest first regardless of closed
  });
  it("mySessionsOn orders by createdAt so agent numbers match task 48", () => {
    const a = session({ id: "s-b", createdAt: "2026-09-25T10:00:00Z" });
    const b = session({ id: "s-a", createdAt: "2026-09-25T09:00:00Z" });
    expect(mySessionsOn(makeIssueHead(), [a, b]).map((s) => s.id)).toEqual(["s-a", "s-b"]);
  });
});
```

#### Tests to write first, part 2 — `desktop/src/components/work/Peek.test.tsx` (keys)

The lens is rendered on task 20's `workScenario()` through task 40's test seam
(`setWorkSourceForTest`, its scope's word, **UNVERIFIED**); `DYLAN` is signed in. The helper
`press(el, code, {holdMs})` fires `keyDown` then, after `vi.advanceTimersByTime(holdMs)` with
`performance.now` faked, `keyUp`.

```tsx
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { DYLAN, workScenario } from "../../test/workFixtures";
import * as ops from "../../work/replica/ops";
import { WorkLens, setWorkSourceForTest } from "./WorkLens";
import { HOLD_MS } from "./Peek";

function press(el: Element, code: string, o: { holdMs?: number } = {}): void {
  fireEvent.keyDown(el, { code, key: code === "Space" ? " " : code });
  if (o.holdMs) act(() => { vi.advanceTimersByTime(o.holdMs!); });
  fireEvent.keyUp(el, { code, key: code === "Space" ? " " : code });
}
function lens() {
  setWorkSourceForTest(workScenario(), { me: DYLAN });
  const view = render(<WorkLens />);
  const list = screen.getByRole("grid");                       // task 40's list element
  list.focus();
  return { view, list };
}
const peek = () => document.querySelector<HTMLElement>(".pk");

beforeEach(() => { vi.useFakeTimers({ toFake: ["setTimeout", "setInterval", "performance"] }); });

describe("peek keys", () => {
  it("a tap opens, a second tap closes, and a hold closes on release", () => {
    const { list } = lens();
    press(list, "Space");
    expect(peek()).not.toBeNull();
    press(list, "Space");
    expect(peek()).toBeNull();
    press(list, "Space", { holdMs: HOLD_MS + 50 });
    expect(peek()).toBeNull();
    // mutant: tap-only peek, where a hold leaves the panel open after release
  });
  it("a hold shows the peek while the key is down", () => {
    const { list } = lens();
    fireEvent.keyDown(list, { code: "Space", key: " " });
    fireEvent.keyDown(list, { code: "Space", key: " ", repeat: true });
    expect(peek()).not.toBeNull();
    fireEvent.keyUp(list, { code: "Space", key: " " });
    expect(peek()).toBeNull();
  });
  it("down moves the peek to the next row and never scrolls the list", () => {
    const { list } = lens();
    list.scrollTop = 120;
    press(list, "Space");
    const first = peek()!.getAttribute("aria-label");
    fireEvent.keyDown(list, { code: "ArrowDown", key: "ArrowDown" });
    expect(peek()!.getAttribute("aria-label")).not.toBe(first);
    expect(list.scrollTop).toBe(120);
    // mutant: moving the list (scrollIntoView on the peek's row)
  });
  it("Esc closes", () => {
    const { list } = lens();
    press(list, "Space");
    fireEvent.keyDown(list, { code: "Escape", key: "Escape" });
    expect(peek()).toBeNull();
  });
  it("opening peek marks nothing seen and writes nothing", () => {
    const spy = vi.spyOn(ops, "mutate");
    const { list } = lens();
    const held = screen.queryByText(/new item|arrived/i)?.textContent ?? null;   // task 41's hold line
    press(list, "Space");
    fireEvent.keyDown(list, { code: "ArrowDown", key: "ArrowDown" });
    expect(spy).not.toHaveBeenCalled();
    expect(screen.queryByText(/new item|arrived/i)?.textContent ?? null).toBe(held);
    // mutant: peek applies the held arrivals or advances a read cursor through mutate
  });
  it("Space in a text field types a space and opens nothing", () => {
    const { view } = lens();
    const input = document.createElement("input");
    view.container.appendChild(input);
    input.focus();
    fireEvent.keyDown(input, { code: "Space", key: " " });
    expect(peek()).toBeNull();
    // mutant: a keys.list leak, Space bound on document
  });
});
```

#### Tests to write first, part 3 — `Peek.test.tsx` (surfaces and source guards)

```tsx
import peekSrc from "./Peek.tsx?raw";
import personSrc from "./PeekPerson.tsx?raw";
import cssSrc from "./work.css?raw";
import { setHunkSource } from "../../work/peek";
import { useCockpit } from "../../store/store";

function openOn(alias: string) {
  const { list } = lens();
  const row = screen.getByText(alias).closest<HTMLElement>("[data-row-id]")!;
  fireEvent.click(row);                                        // task 40: a click sets the cursor
  const t0 = performance.now();
  press(list, "Space");
  return { list, row, ms: performance.now() - t0 };
}

describe("peek surfaces", () => {
  it("the criterion id is in peek in mono and never on the row", () => {
    const { row } = openOn("EP-167");                          // todo, mine, serves c-91e2a4b0
    expect(within(row).queryByText(/c-91e2/)).toBeNull();
    const id = within(peek()!).getByText(/c-91e2a4b0/);
    expect(id.closest(".pk-fact")).not.toBeNull();
    // mutant: machine ids on the row
  });
  it("a teammate's peek renders her issues and no session, attach or transcript", () => {
    const attach = vi.fn();
    useCockpit.setState({ sessions: {} });
    openOn("EP-163");                                          // Alice's, started
    const p = peek()!;
    expect(p.dataset.peekKind).toBe("person");
    expect(within(p).getByText("Alice Example")).toBeTruthy();
    expect(p.querySelector(".pk-span, .pk-diff, [data-terminal-host]")).toBeNull();
    expect(attach).not.toHaveBeenCalled();
    expect(personSrc).not.toMatch(/attachRegistry|xterm|sessions|useCockpit|transcript/);
    // mutant: showing a teammate's session (OD-15)
  });
  it("Peek imports nothing from the attach registry or a terminal", () => {
    expect(peekSrc).not.toMatch(/api\/attachRegistry|@xterm|TerminalPane/);
    // mutant: PTY-derived meaning (OD-8)
  });
  it("a landed row peeks the land commit, and a missing diff source says so in place", async () => {
    setHunkSource({ hunksOf: () => Promise.reject(new Error("the land diff is not on this build yet")) });
    openOn("EP-142");                                          // done, proven, landCommit LAND
    expect(peek()!.dataset.peekKind).toBe("hunk");
    await act(async () => { await Promise.resolve(); });
    expect(within(peek()!).getByText("the land diff is not on this build yet")).toBeTruthy();
  });
  it("a hunk's Comment on this opens the composer with a hunk anchor", async () => {
    setHunkSource({ hunksOf: async () => [{ path: "upload/resume.go", oldStart: 40, oldLines: 2,
      newStart: 40, newLines: 3, lines: [" a", "-b", "+c", "+d"], spanRef: null }] });
    openOn("EP-142");
    await act(async () => { await Promise.resolve(); });
    fireEvent.click(within(peek()!).getByRole("button", { name: "Comment on this" }));
    expect(within(peek()!).getByLabelText(/Comment on EP-142/)).toBeTruthy();
  });
  it("the peek's CSS animates only opacity and transform", () => {
    const pk = cssSrc.slice(cssSrc.indexOf("/* Peek:"));
    expect(pk).not.toMatch(/transition:\s*all/);
    const frames = pk.match(/@keyframes pk-in\s*\{[^}]*\{[^}]*\}[^}]*\{[^}]*\}/)![0];
    const props = [...frames.matchAll(/([a-z-]+)\s*:/g)].map((m) => m[1]);
    expect(new Set(props)).toEqual(new Set(["opacity", "transform"]));
    // mutant: layout motion (height, top)
  });
  it("no prediction anywhere, and budget.peek is recorded", () => {
    const { ms } = openOn("EP-167");
    expectNoPrediction(document.body);
    console.info(`budget.peek first paint: ${ms.toFixed(1)} ms`);   // a record, not a verdict
  });
});
```

The composer's accessible name comes from task 51's `label` prop (`task-51.md:267`); if its part
labels the editor another way, the query follows it. The `EP-167` row serves `c-91e2a4b0` only if
task 20's `head(167, …)` revision serves it; if not, the test builds that revision with
`makeIssueRevision` (**UNVERIFIED**, `task-20.md:299-320`).

#### Steps

- [ ] Write `peek.test.ts`; it fails to import `./peek`. Write `peek.ts` until the seven pass.
      Mutants, one at a time, then revert: test the session before the land commit (test 1); match
      any session in the project instead of `issueRef` (test 2); treat `assignee === null` as
      someone else's (test 3); drop the zero-time guard in `spanFacts` (test 4); sort
      `mySessionsOn` by `updatedAt` (test 7).
- [ ] Write `Peek.test.tsx` part 2; it fails to import `./Peek`. Write `usePeekController` until the
      six key tests pass. Mutants: close on every `keyup` (test 1); call `scrollIntoView` in the
      arrow branch (test 3); call `mutate` or `hold.apply` on open (test 5); bind on `document`
      instead of the list's `onKeyDown` (test 6).
- [ ] Write `Peek`, its issue, span and hunk bodies, and `PeekPerson.tsx`; append the `.pk-*` CSS.
- [ ] Write part 3 of the tests. Mutants: render `c-91e2` in `IssueRow` (test 1); import
      `useCockpit` into `PeekPerson.tsx` (test 2); add `transition: height` to `.pk` (test 6).
- [ ] Make the three edits (`WorkLens.tsx`, `IssueRow.tsx`, `BoardView.tsx`). Tasks 40, 41, 43 and
      53's suites still pass.
- [ ] Commit each file as it goes green. Paste the recorded `budget.peek` line into the commit
      message of `Peek.tsx`, as a measurement with its machine named, not as a pass.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/peek.test.ts src/components/work/Peek.test.tsx'
# expected: 2 files, 20 tests, all passing (7 pure, 6 keys, 7 surfaces), and one line
# "budget.peek first paint: <n> ms" in the output: a record, not a verdict
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 40, 41, 43, 48, 51, 53 and 55 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "attachRegistry|@xterm|TerminalPane|setInterval|useCockpit" desktop/src/components/work/PeekPerson.tsx'
# expected: no output (decision 7; the source test holds it too, a review still reads the file)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "setInterval|scrollIntoView|scrollTop *=" desktop/src/components/work/Peek.tsx'
# expected: no output (decisions 2 and 4)
```

#### What this task does NOT claim

- **02 C9's twelve transcript lines around a span**: they need a harness transcript read through
  `OD-8`'s pinned adapter (L-185, W3). No W1 source satisfies `OD-8`, so `peek.span` shows only
  session-record facts.
- **The lane event and the files observed in `peek.span`**, and any hunk in `peek.hunk`: both wait
  for phase 3's lane and diff records (**UNVERIFIED until merge**, rule H2). Before then the peek
  says so in place.
- **Leases in `peek.person`** (W3), and any fact about a teammate beyond her issues' heads.
- **Create an issue from a span or hunk**: task 68.
- **`budget.peek ≤ 100 ms` as met**: the test records the time on the machine that ran it.
- **Visual acceptance**: the owner's eyes (CLAUDE.md, working agreements); this part proves the
  structure, the keys and the CSS properties, not the look.
- **Parity**: closes L-108's W1 part (row 108, `05-design.md:242`).

#### Gaps found while writing this part

1. The skeleton's file map (`:306`) lists `Peek.tsx` for task 60 but omits `src/work/peek.ts`,
   `PeekPerson.tsx` and the edits to `WorkLens.tsx`, `IssueRow.tsx` and `BoardView.tsx`.
2. Task 37's `IssueHead` has no `title` (`task-37.md:121-128`; task 58's gap 2); `Peek` takes
   `titleOf` until task 37 declares it.
3. `isListKeyTarget` and `setWorkSourceForTest` are task 53's and task 55's scope words; their parts
   were not on the tree at this writing, and they win any disagreement.
4. `useWork`'s selection equality (task 55's scope, `tasks-41-60.md:483`) is not yet a named
   helper; `PeekPerson` memoises on `r.cursor` until task 55's part names one.
