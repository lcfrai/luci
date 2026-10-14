### Task 47: the issue view, part 2: revisions, restore, `serves` with `thread.node`, revision-pinned links

Stream **WD3** (the issue view), about 3 hours (an estimate, not a measurement). Depends on task 36
(SW.14: `ThreadNode`, `ThreadLine`, `useThreadLit`), task 41 (`isBareListKey`, the list-key gate)
and task 46 (the view's frame, `AuthorClass`, `issueData.ts`, `clock`). Uses SW.1 (the immutable,
digested revision `issue:is-…@r` and its `serves [{briefKey, criterionId}]`), SW.2's relay reads of
revisions (task 16), task 42's `CriterionPicker` and SW.16's `mutate`.

An issue's text changes; what an agent was told must not. Every text change is a new, immutable,
digested revision (05 §3.6; `L-34`), and an agent's managed block names the exact one it read
(task 19). This part makes that visible and usable, with four rules, each one a test:

1. **History is never rewritten.** *Restore* writes a **new** revision whose content equals the
   chosen one (the Hub's `RestoreRevisionRequest`, `task-03.md:476-480`, sets `restoredFrom`);
   restoring @2 on a head of @3 makes @4 and keeps @2 and @3. Undo of a restore is one more
   revision, never a delete.
2. **The view opens at the revision it was sent to.** A chip or link naming @3 opens @3, and a
   non-head revision says so in one line: *You are reading @3; the head is @5 (Alice, 11:02)*,
   with a link to the head (05 §4.5 rule 1). Showing the head silently would make every *"what did
   the agent read?"* answer wrong.
3. **Links name the revision being read.** `issueLink(id, rev)` makes `panacea://issue/is-…@3`;
   `parseIssueLink` accepts only the `is-` form and never an alias (05 §3.6), because an alias
   changes when a project key changes and a link must not.
4. **Lit is not selected.** Hovering or focusing a `serves` row lights its criterion in Work,
   Design and Develop at once (SW.14), and never rings the row or sets `aria-selected` (05 §4.1,
   07 D I1).

**Decisions made here, so nobody re-derives them:**

1. **Revisions are read on demand through the relay, not from the replica.** Task 16 relays
   `GET /v1/hub/work/issues/{issueId}/revisions` and `…/revisions/{rev}` (`task-16.md:792-793`).
   They are immutable, so the view holds what it fetched in component state for as long as it is
   mounted, and never writes it anywhere (the no-persistence rule, task 55). The replica keeps
   heads and head revisions only; this part adds nothing to it.
2. **The restore is a `WorkOp` this part needs.** Task 56's closed union lists `revise {baseRev,
   patch}` but no `restore`; a `revise` copying @2's content would lose `restoredFrom`, which is
   the only record that @4 *is* a restore. This part sends `mutate({kind: "restore", issueId,
   fromRevision})` over task 16's `RestoreIssueRevision` row (`task-16.md:137`); the union member
   is a gap for task 56. Its inverse, for `U`, is `restore` of the head it replaced.
3. **Removing a criterion is a revision, not a patch.** `serves` lives in the revision content
   (`IssueRevisionContent`, `task-02.md:260-264`), so removal is `revise` with the head as
   `baseRevision` and the shorter `serves`; a stale base is the Hub's 409 `revision_conflict`, which
   task 56 keeps and re-offers.
4. **Who read which revision is local fact only.** *dylan · agent 1 read @3* comes from the
   reader's own sessions whose `issueRef` equals that revision's key (`Session.issueRef`, task 19,
   `task-19.md:304`). Another person's agents are never shown here (05 §4.4 rule 3). The label rule
   is `first name · agent N`, N counting the reader's sessions on this issue by `createdAt`; task
   40's `AgentLine` must use the same rule, and if its part words it differently, its rule wins.
5. **The criterion's `href` is a hash**, `#brief:bf-…@6/c-91e2a4b0` from `criterionHref`. Task 36
   leaves the form to this task (`task-36.md:41-46`); a hash never reloads the webview. What opens
   when it changes is Discover's (phase 4), not claimed here.

**Files:**
- `desktop/src/work/links.ts` — `issueLink`, `parseIssueLink`, `criterionHref` (new; not yet in the
  File map)
- `desktop/src/components/issue/revisionWords.ts` — `changeWords`, `lineDiff`, `agentsWhoRead`;
  pure (new; not yet in the File map)
- `desktop/src/components/issue/useRevision.ts` — one revision or the list, fetched on demand (new;
  not yet in the File map)
- `desktop/src/components/issue/Revisions.tsx` — the list, the read-only diff, *Restore*, *Copy
  link* and *Copy link to head* (new)
- `desktop/src/components/issue/ReadingLine.tsx` — *You are reading @3; the head is @5* (new; not
  yet in the File map)
- `desktop/src/components/issue/Serves.tsx` — the criteria this issue serves, with `ThreadNode`,
  add through `T`, remove (new)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46's) — reads the revision it was opened
  at; mounts `ReadingLine`, `Serves` and `Revisions` (modified)
- `desktop/src/components/issue/issueData.ts` ⚑ (task 46's) — `CriterionView` and `useCriterion`,
  appended (modified)
- `desktop/src/components/issue/issue.css` ⚑ (task 46's) — the `.iv-rev*`, `.iv-serve*`,
  `.iv-reading` and `.iv-diff*` rules, appended (modified)
- `desktop/src/work/links.test.ts`, `desktop/src/components/issue/revisionWords.test.ts`,
  `Revisions.test.tsx`, `Serves.test.tsx` — the tests (new)

#### Interfaces — complete: `desktop/src/work/links.ts`

```ts
// The internal link form (05 §1 row 98, §3.6): panacea://issue/is-<16 hex>[@<rev>]. Only the is-
// id is ever a link target; an alias ("EP-142") is display text and parses to null. The https Hub
// link and its read-only page are W2 (task 80); registering the scheme with Windows is not here.
const ISSUE_ID = /^is-[0-9a-f]{16}$/;          // ids.Wide("is"), hub-contract.md:93-100
const REV = /^[1-9][0-9]{0,8}$/;               // no 0, no leading zero: the relay's 400 rule
const PREFIX = "panacea://issue/";

export interface IssueLinkTarget { id: string; rev: number | null }

/** `rev` pins the link to a revision; omit it for *Copy link to head*. */
export function issueLink(id: string, rev?: number): string {
  if (!ISSUE_ID.test(id)) throw new Error(`issueLink: not an issue id: ${JSON.stringify(id)}`);
  if (rev === undefined) return PREFIX + id;
  if (!REV.test(String(rev))) throw new Error(`issueLink: not a revision: ${rev}`);
  return `${PREFIX}${id}@${rev}`;
}

export function parseIssueLink(s: string): IssueLinkTarget | null {
  if (!s.startsWith(PREFIX)) return null;
  const rest = s.slice(PREFIX.length);
  const at = rest.indexOf("@");
  const id = at < 0 ? rest : rest.slice(0, at);
  if (!ISSUE_ID.test(id)) return null;
  if (at < 0) return { id, rev: null };
  const r = rest.slice(at + 1);
  return REV.test(r) ? { id, rev: Number(r) } : null;
}

/** The provenance key of a revision, `issue:is-…@r` (SW.1). The mono form a reader copies. */
export const issueKey = (id: string, rev: number): string => `issue:${id}@${rev}`;

/** A thread node's href (decision 5): a hash, so the webview never reloads. */
export const criterionHref = (briefKey: string, criterionId: string): string =>
  `#${briefKey}/${criterionId}`;
```

`parseIssueLink` is strict in both directions: an uppercase id, a trailing slash, `@03` and `@0`
are all `null`, so a hand-edited link fails where it is pasted rather than opening the wrong
revision. It does not accept `issue:is-…@r` (the key form): a key is identity, not an address, and
task 58's *open by number* is the other way in.

#### Interfaces — complete: `desktop/src/components/issue/revisionWords.ts`

```ts
// Pure words and diffs for the revision list. No React, no replica, no clock.
import type { IssueRevision, Serve, User } from "../../api/work";
import type { Session } from "../../store/types";

const serveKey = (s: Serve): string => `${s.briefKey} ${s.criterionId}`;

/** What a revision changed, in words: "opened", "restored @2", "changed the title and the
 *  description; serves c-40aa17c3; no longer serves c-91e2a4b0". Never a count of edits. */
export function changeWords(prev: IssueRevision | null, cur: IssueRevision): string {
  if (cur.erasedAt !== null) return "erased";
  if (cur.restoredFrom !== null) return `restored @${cur.restoredFrom}`;
  if (prev === null) return "opened";
  const a = prev.content, b = cur.content;
  const changed: string[] = [];
  if (a.title.text !== b.title.text || a.title.author !== b.title.author) changed.push("the title");
  if (a.description.text !== b.description.text || a.description.author !== b.description.author) {
    changed.push("the description");
  }
  const before = new Set(a.serves.map(serveKey));
  const after = new Set(b.serves.map(serveKey));
  const added = b.serves.filter((s) => !before.has(serveKey(s))).map((s) => s.criterionId);
  const removed = a.serves.filter((s) => !after.has(serveKey(s))).map((s) => s.criterionId);
  const words: string[] = [];
  if (changed.length > 0) words.push(`changed ${changed.join(" and ")}`);
  if (added.length > 0) words.push(`serves ${added.join(", ")}`);
  if (removed.length > 0) words.push(`no longer serves ${removed.join(", ")}`);
  return words.length > 0 ? words.join("; ") : "changed what it implements";
}

export interface DiffLine { op: "same" | "add" | "del"; text: string }

/** A line diff, longest common subsequence, `from` → `to`. Descriptions are short; O(n·m). */
export function lineDiff(from: string, to: string): DiffLine[] {
  const a = from.split("\n"), b = to.split("\n");
  const L: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      L[i][j] = a[i] === b[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0, j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) { out.push({ op: "same", text: a[i] }); i++; j++; }
    else if (L[i + 1][j] >= L[i][j + 1]) { out.push({ op: "del", text: a[i] }); i++; }
    else { out.push({ op: "add", text: b[j] }); j++; }
  }
  while (i < a.length) out.push({ op: "del", text: a[i++] });
  while (j < b.length) out.push({ op: "add", text: b[j++] });
  return out;
}

/** Decision 4: the reader's OWN agents that were given this revision's key. "dylan · agent 1". */
export function agentsWhoRead(
  key: string, issueId: string, sessions: readonly Session[], me: User,
): string[] {
  const mine = sessions
    .filter((s) => s.issueRef.startsWith(`issue:${issueId}@`))
    .sort((x, y) => x.createdAt.localeCompare(y.createdAt));
  const first = (me.displayName.split(" ")[0] ?? me.displayName).toLowerCase();
  return mine.flatMap((s, n) => (s.issueRef === key ? [`${first} · agent ${n + 1}`] : []));
}
```

`sessions` is this control plane's session list, `Object.values(useCockpit((s) => s.sessions))`
(`store/slices/dataSlice.ts:34`), which holds only the reader's own sessions: the local control
plane runs no one else's (`OD-20`, local mode). So *another person's agent* cannot reach this
function in W1, and a W3 caller that passes Fleet's sessions must filter to the reader first.

#### Interfaces — complete: `desktop/src/components/issue/useRevision.ts`

```ts
import { useEffect, useState } from "react";
import { api } from "../../api";
import type { IssueRevision } from "../../api/work";

export type Fetched<T> =
  | { status: "fetching" }
  | { status: "ready"; value: T }
  | { status: "refused"; message: string };

const why = (e: unknown): string => (e instanceof Error ? e.message : String(e));

/** The revision the view reads. The head's content is already in the replica, so asking for the
 *  head sends nothing; any other revision is one relay GET (decision 1), held while mounted. */
export function useRevision(issueId: string, rev: number | null, head: IssueRevision): Fetched<IssueRevision> {
  const want = rev ?? head.revision;
  const key = `${issueId}@${want}`;
  const [got, setGot] = useState<{ key: string; f: Fetched<IssueRevision> } | null>(null);
  useEffect(() => {
    if (want === head.revision) return;
    let live = true;
    api.work.issueRevisionAt(issueId, want).then(
      (value) => { if (live) setGot({ key, f: { status: "ready", value } }); },
      (e: unknown) => { if (live) setGot({ key, f: { status: "refused", message: why(e) } }); },
    );
    return () => { live = false; };
  }, [issueId, want, head.revision, key]);
  if (want === head.revision) return { status: "ready", value: head };
  return got?.key === key ? got.f : { status: "fetching" };
}

/** Every revision, newest first, following `next` to the end (the list convention,
 *  hub-contract.md:48-49). Re-read when the head moves, so a new revision appears. */
export function useRevisionList(issueId: string, headRevision: number): Fetched<IssueRevision[]> {
  const [state, setState] = useState<{ at: number; f: Fetched<IssueRevision[]> } | null>(null);
  useEffect(() => {
    let live = true;
    (async () => {
      const all: IssueRevision[] = [];
      let after = "";
      do {
        const page = await api.work.issueRevisions(issueId, { after });
        all.push(...page.items);
        after = page.next;
      } while (after !== "" && live);
      return all.sort((x, y) => y.revision - x.revision);
    })().then(
      (value) => { if (live) setState({ at: headRevision, f: { status: "ready", value } }); },
      (e: unknown) => { if (live) setState({ at: headRevision, f: { status: "refused", message: why(e) } }); },
    );
    return () => { live = false; };
  }, [issueId, headRevision]);
  return state?.at === headRevision ? state.f : { status: "fetching" };
}
```

`api.work.issueRevisionAt(issueId, rev)` and `api.work.issueRevisions(issueId, {after})` are task
37's `WorkClient` methods for task 16's two relay rows, named here after the Go rows in camel case;
**UNVERIFIED** until task 37's part is written. A refusal renders its message where the content
would be (*fetching @3 was refused: not_found*), never a blank. `fetching` renders as the words
*fetching @3*, once, with no spinner and no animation.

#### Interfaces — complete: `desktop/src/components/issue/ReadingLine.tsx`

```tsx
import type { JSX } from "react";
import type { IssueRevision } from "../../api/work";
import { clock } from "./IssueHead";

/** 05 §4.5 rule 1: never show a non-head revision without saying so. `null` on the head. */
export function ReadingLine(props: {
  reading: number; head: IssueRevision; onOpenHead(): void;
}): JSX.Element | null {
  const { reading, head, onOpenHead } = props;
  if (reading === head.revision) return null;
  const who = head.writtenBy.displayName.split(" ")[0] ?? head.writtenBy.displayName;
  return (
    <p className="iv-reading" role="note">
      You are reading <span className="iv-fact" data-fact="reading">@{reading}</span>; the head is{" "}
      <button type="button" className="iv-link" onClick={onOpenHead}>
        <span className="iv-fact" data-fact="head">@{head.revision}</span>
      </button>{" "}
      ({who}, <time className="iv-fact" data-fact="head-at" dateTime={head.writtenAt}>{clock(head.writtenAt)}</time>)
    </p>
  );
}
```

#### Interfaces — complete: `desktop/src/components/issue/Revisions.tsx`

```tsx
import { useState, type JSX } from "react";
import type { IssueRevision, User } from "../../api/work";
import { useCockpit } from "../../store/store";
import { mutate } from "../../work/replica/ops";
import { AuthorClass } from "./AuthorClass";
import { clock } from "./IssueHead";
import { agentsWhoRead, changeWords, lineDiff } from "./revisionWords";
import { useRevisionList } from "./useRevision";

export function Revisions(props: {
  issueId: string; headRevision: IssueRevision; reading: number; me: User; onOpen(rev: number): void;
}): JSX.Element {
  const { issueId, headRevision, reading, me, onOpen } = props;
  const list = useRevisionList(issueId, headRevision.revision);
  const sessions = Object.values(useCockpit((s) => s.sessions));
  const [diffOf, setDiffOf] = useState<number | null>(null);
  if (list.status !== "ready") {
    return (
      <section className="iv-revs" aria-label="Revisions">
        <p className="iv-fact" data-fact="revs">{list.status === "fetching"
          ? "fetching revisions" : `fetching revisions was refused: ${list.message}`}</p>
      </section>
    );
  }
  const revs = list.value;                                  // newest first
  return (
    <section className="iv-revs" aria-label="Revisions">
      <h2 className="iv-h2">Revisions</h2>
      <ol className="iv-rev-list">
        {revs.map((r, n) => {
          const isHead = r.revision === headRevision.revision;
          const { title, description } = r.content;
          return (
            <li key={r.key} className="iv-rev" data-rev={r.revision}
              aria-current={r.revision === reading ? "true" : undefined}>
              <span className="iv-fact" data-fact="rev">@{r.revision}</span>
              <span className="iv-rev-who">{r.writtenBy.displayName}</span>
              <AuthorClass author={description.author} />
              {title.author !== description.author ? <>title <AuthorClass author={title.author} /></> : null}
              <time className="iv-fact" data-fact="written" dateTime={r.writtenAt}>{clock(r.writtenAt)}</time>
              <span className="iv-rev-what">{changeWords(revs[n + 1] ?? null, r)}</span>
              {agentsWhoRead(r.key, issueId, sessions, me).map((w) => (
                <span key={w} className="iv-fact" data-fact="read-by">{w} read @{r.revision}</span>
              ))}
              <span className="iv-rev-actions">
                <button type="button" onClick={() => onOpen(r.revision)}>Open</button>
                {isHead ? null : (
                  <button type="button" aria-expanded={diffOf === r.revision}
                    onClick={() => setDiffOf(diffOf === r.revision ? null : r.revision)}>Compare with head</button>
                )}
                {isHead || r.erasedAt !== null ? null : (
                  <button type="button"
                    onClick={() => mutate({ kind: "restore", issueId, fromRevision: r.revision })}>Restore</button>
                )}
              </span>
              {diffOf === r.revision ? <RevisionDiff from={r} to={headRevision} /> : null}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

const SIGN = { same: " ", add: "+", del: "−" } as const;
const SAID = { same: "", add: "added: ", del: "removed: " } as const;

/** Read-only, `from` → the head. The sign carries the op and a hidden word says it aloud, so
 *  nothing is colour alone. The text is the issue's own: `data-verbatim` exempts it from task
 *  20's scan, since a description may say "50%" without predicting anything. */
function RevisionDiff(props: { from: IssueRevision; to: IssueRevision }): JSX.Element {
  const { from, to } = props;
  const lines = [
    ...(from.content.title.text === to.content.title.text ? [] : lineDiff(from.content.title.text, to.content.title.text)),
    ...lineDiff(from.content.description.text, to.content.description.text),
  ];
  return (
    <ol className="iv-diff" aria-label={`@${from.revision} compared with the head, @${to.revision}`} data-verbatim>
      {lines.map((l, i) => (
        <li key={i} className="iv-diff-line" data-op={l.op}>
          <span className="iv-diff-sign" aria-hidden="true">{SIGN[l.op]}</span>
          <span className="iv-vh">{SAID[l.op]}</span>{l.text}
        </li>
      ))}
    </ol>
  );
}
```

#### Interfaces — complete: appended to `desktop/src/components/issue/issueData.ts` (task 46's)

```ts
/** A criterion as the Work layer shows it: its words, and its convergence word (task 40's
 *  CriterionGroup type, `tasks-21-40.md:684-685`). Undefined when the brief is not in memory. */
export interface CriterionView {
  id: string;
  text: string;
  word: "settled" | "open" | "contested" | "drifting" | "uncovered";
}

export function useCriterion(briefKey: string, criterionId: string): CriterionView | undefined {
  return useWork((r) => r.criterion(briefKey, criterionId));
}
```

`r.criterion(briefKey, criterionId)` joins task 46's four replica reads as a gap for tasks 55-57:
task 40's criterion groups need the same read, so it is one read two tasks share, not a second
source.

#### Interfaces — complete: `desktop/src/components/issue/Serves.tsx`

```tsx
import { useState, type JSX, type KeyboardEvent } from "react";
import type { IssueRevision, Serve } from "../../api/work";
import { criterionHref } from "../../work/links";
import { mutate } from "../../work/replica/ops";
import { ThreadNode, ThreadTrace, useThreadLit } from "../thread";
import { CriterionPicker } from "../work/CriterionPicker";
import { useCriterion } from "./issueData";
import { isBareListKey } from "../../work/listKeys"; // task 41: a key typed in a field is the field's
/** The criteria this revision serves. Editable only on the head: removing or adding writes a
 *  new revision on top of it (decision 3). `play` is true on a drill-in (ThreadTrace, SW.14). */
export function Serves(props: { issueId: string; revision: IssueRevision; isHead: boolean; play: boolean }): JSX.Element {
  const { issueId, revision, isHead, play } = props;
  const [adding, setAdding] = useState(false);
  const serves = revision.content.serves;
  const write = (next: Serve[]) =>
    mutate({ kind: "revise", issueId, baseRev: revision.revision, patch: { serves: next } });
  const onKey = (e: KeyboardEvent<HTMLUListElement>) => {
    // List keys fire in the list, never in a field in it (05 §4.3; 23b's check line); `T` is KeyT, no modifier.
    if (!isHead || e.code !== "KeyT" || e.shiftKey || !isBareListKey(e)) return;
    e.preventDefault();
    setAdding(true);
  };
  return (
    <section className="iv-serves" aria-label="Serves">
      <h2 className="iv-h2">Serves</h2>
      {serves.length === 0 ? <p className="iv-serve-none">Serves no criterion</p> : null}
      <ThreadTrace play={play}>
        <ul className="iv-serve-list" onKeyDown={onKey} aria-label="Criteria this issue serves">
          {serves.map((s) => (
            <ServeRow key={`${s.briefKey} ${s.criterionId}`} serve={s} canRemove={isHead}
              onRemove={() => write(serves.filter((x) => x !== s))} />
          ))}
        </ul>
      </ThreadTrace>
      {isHead ? (
        <button type="button" className="iv-serve-add" onClick={() => setAdding(true)}>
          Add a criterion <kbd className="iv-kbd">T</kbd>
        </button>
      ) : null}
      {adding ? (
        <CriterionPicker
          exclude={serves}
          onChoose={(s: Serve) => { write([...serves, s]); setAdding(false); }}
          onClose={() => setAdding(false)}
        />
      ) : null}
    </section>
  );
}

/** One criterion. Hover or focus lights it everywhere (SW.14); the row itself never changes:
 *  no ring, no class, no aria-selected (lit is not selected). No criterion text, no node. */
function ServeRow(props: { serve: Serve; canRemove: boolean; onRemove(): void }): JSX.Element {
  const { serve, canRemove, onRemove } = props;
  const c = useCriterion(serve.briefKey, serve.criterionId);
  const { bind } = useThreadLit(serve.criterionId);
  return (
    <li className="iv-serve" tabIndex={0} data-criterion={serve.criterionId} {...bind}>
      <ThreadNode
        criterionId={serve.criterionId}
        href={criterionHref(serve.briefKey, serve.criterionId)}
        label={c ? `Brief ${serve.briefKey}, criterion ${serve.criterionId}: ${c.text}` : ""}
      />
      <span className="iv-serve-text">{c ? c.text : "this criterion's brief is not in your replica"}</span>
      {c ? <span className="iv-serve-word">{c.word}</span> : null}
      <span className="iv-fact" data-fact="brief">{serve.briefKey}</span>
      <span className="iv-fact" data-fact="criterion">{serve.criterionId}</span>
      {canRemove ? (
        <button type="button" className="iv-serve-remove" onClick={onRemove}
          aria-label={`Stop serving ${c?.text ?? serve.criterionId}`}>Remove</button>
      ) : null}
    </li>
  );
}
```

`CriterionPicker` is task 42's (`tasks-41-60.md:61-73`); its props here, `exclude`, `onChoose(serve)`
and `onClose`, are what this part passes, and are **UNVERIFIED** until task 42's part is written
(a gap if they differ; the rename is in this file only). The criterion's words are sans and its
keys are mono (`voice.split`, 05 §4.4 C3). The convergence word is a word, never a colour.

#### Interfaces — complete: `desktop/src/components/issue/IssueView.tsx` (task 46's, as this task leaves it)

```tsx
import { useEffect, useState, type JSX, type ReactNode } from "react";
import type { User } from "../../api/work";
import { issueLink } from "../../work/links";
import { ViewHeader, type Route } from "../common/ViewHeader";
import { IssueHead } from "./IssueHead";
import { useIssueViewData, type IssueViewData } from "./issueData";
import { ReadingLine } from "./ReadingLine";
import { Revisions } from "./Revisions";
import { Serves } from "./Serves";
import { useRevision } from "./useRevision";
import "./issue.css";

const copy = (s: string) => void navigator.clipboard.writeText(s);

export function IssueView(props: {
  issueId: string; revision: number | null; me: User; route: Route | null; drillIn?: boolean;
  slots?: { body?: ReactNode; relations?: ReactNode; activity?: ReactNode };
}): JSX.Element {
  const data = useIssueViewData(props.issueId);
  if (data === null) {
    return <section className="iv" aria-label="Issue"><p className="iv-empty">This issue is not in your replica.</p></section>;
  }
  return <IssueViewBody {...props} data={data} />;
}

/** Split out so its hooks run only once the record is in memory, never on a placeholder. */
function IssueViewBody(props: Parameters<typeof IssueView>[0] & { data: IssueViewData }): JSX.Element {
  const { issueId, revision, me, route, drillIn = false, slots = {}, data } = props;
  const [reading, setReading] = useState<number | null>(revision);
  useEffect(() => setReading(revision), [issueId, revision]);  // a new chip or link re-opens
  const { head, headRevision, teamLabels, people } = data;
  const shown = useRevision(issueId, reading, headRevision);
  const at = reading ?? headRevision.revision;
  const actions = (
    <>
      <button type="button" onClick={() => copy(issueLink(head.id, at))}>Copy link</button>
      <button type="button" onClick={() => copy(issueLink(head.id))}>Copy link to head</button>
    </>
  );
  return (
    <section className="iv" aria-label={`Issue ${head.alias}`} data-issue={head.id} data-reading={at}>
      <ViewHeader title={headRevision.content.title.text} route={route} actions={actions}
        badge={<span className="iv-alias iv-fact" data-fact="alias">{head.alias}</span>} />
      <IssueHead head={head} title={headRevision.content.title} serving={headRevision.content.serves.length > 0}
        me={me} people={people} teamLabels={teamLabels} />
      <ReadingLine reading={at} head={headRevision} onOpenHead={() => setReading(null)} />
      {shown.status === "ready" ? (
        <>
          {slots.body ?? <div className="iv-desc" data-verbatim>{shown.value.content.description.text}</div>}
          <Serves issueId={head.id} revision={shown.value} isHead={at === headRevision.revision} play={drillIn} />
        </>
      ) : (
        <p className="iv-fact" data-fact="reading-state">{shown.status === "fetching"
          ? `fetching @${at}` : `fetching @${at} was refused: ${shown.message}`}</p>
      )}
      {slots.relations ?? null}
      <Revisions issueId={head.id} headRevision={headRevision} reading={at} me={me} onOpen={setReading} />
      {slots.activity ?? null}
    </section>
  );
}
```

Four things in that file are deliberate. The title in the header stays the **head's**, because
the header names the issue and the reading line names the revision; showing an old title as the
view title would read as the issue's name. The description falls back to plain, `data-verbatim`
text until task 49's editor fills `slots.body`, so a description saying "50%" is not a
prediction (task 20's exemption). *Copy link* pins `at`, the revision being read, and *Copy link
to head* pins nothing. `navigator.clipboard.writeText` in the Tauri 2 webview is **UNVERIFIED**
here (no call to it exists in `desktop/src` today); if WebView2 refuses it, the fallback is
`@tauri-apps/plugin-clipboard-manager` (licence to be read from its lock entry; MIT or Apache-2.0
expected, **UNVERIFIED**), a dependency this part does not add.

#### Interfaces — complete: appended to `desktop/src/components/issue/issue.css`

```css
/* Task 47: revisions, the reading line, serves. Signs and words carry every op; no colour alone. */
.iv-h2 { margin: 0; font: inherit; font-family: var(--sans); font-weight: 600; color: var(--ink); }
.iv-reading { margin: 0 var(--s-4); font-family: var(--sans); color: var(--ink); }
.iv-link { font: inherit; color: var(--ink); background: none; border: 0; padding: 0;
  text-decoration: underline; text-underline-offset: 2px; }
.iv-desc { padding: var(--s-3) var(--s-4); font-family: var(--sans); white-space: pre-wrap; color: var(--ink); }
.iv-revs, .iv-serves { display: grid; gap: var(--s-2); padding: var(--s-3) var(--s-4); }
.iv-rev-list, .iv-serve-list { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--s-1); }
.iv-rev { display: flex; flex-wrap: wrap; align-items: baseline; gap: var(--s-2); }
.iv-rev[aria-current="true"] .iv-fact[data-fact="rev"] { color: var(--ink); font-weight: 600; }
.iv-rev-what { font-family: var(--sans); color: var(--ink); }
.iv-rev-actions { display: inline-flex; gap: var(--s-1); margin-inline-start: auto; }
.iv-diff { flex-basis: 100%; list-style: none; margin: 0; padding: var(--s-2); border-radius: var(--r-sm);
  font-family: var(--mono); font-size: var(--t-fact); box-shadow: 0 0 0 1px var(--ring); }
.iv-diff-line { white-space: pre-wrap; color: var(--ink); }
.iv-diff-line[data-op="del"] { text-decoration: line-through; text-decoration-thickness: 1px; }
.iv-diff-sign { display: inline-block; inline-size: 2ch; }
.iv-vh { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden; clip-path: inset(50%); }
.iv-serve { display: flex; align-items: center; gap: var(--s-2); min-block-size: var(--row-h);
  border-radius: var(--r-sm); outline: none; }
.iv-serve:focus-visible { outline: 1px dotted var(--ink-2); outline-offset: 2px; }
.iv-serve-text { font-family: var(--sans); color: var(--ink); }
.iv-serve-word { font-family: var(--sans); color: var(--ink-2); }
.iv-serve-none { margin: 0; font-family: var(--sans); color: var(--ink-2); }
.iv-kbd { font-family: var(--mono); font-size: 11px; color: var(--ink-2); }
```

The `serve` row's focus mark is a dotted outline in `--ink-2`, **not** the selection ring
(`--accent-ring`): a focused `serves` row lights its criterion, and a reader must be able to tell
*the keyboard is here* from *this is selected*. `.iv-kbd` is the one 11px mono size SW.11 allows
(`kbd` hints only).

#### Tests to write first — `desktop/src/work/links.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { WORK_IDS } from "../test/workFixtures";
import { criterionHref, issueKey, issueLink, parseIssueLink } from "./links";

const id = WORK_IDS.ep142;                     // "is-3f2a91c07b1e4d2a"

describe("links (05 §3.6)", () => {
  it("pins a link to the revision it names, and to nothing when asked for the head", () => {
    expect(issueLink(id, 3)).toBe("panacea://issue/is-3f2a91c07b1e4d2a@3");
    expect(issueLink(id)).toBe("panacea://issue/is-3f2a91c07b1e4d2a");
  });
  it("round-trips an id and a revision", () => {
    expect(parseIssueLink(issueLink(id, 12))).toEqual({ id, rev: 12 });
    expect(parseIssueLink(issueLink(id))).toEqual({ id, rev: null });
  });
  it("never parses an alias, in any form", () => {
    for (const s of ["EP-142", "panacea://issue/EP-142", "panacea://issue/EP-142@3", "ep-142"]) {
      expect(parseIssueLink(s)).toBeNull();
    }
  });
  it("refuses a malformed revision or id rather than guessing", () => {
    for (const s of [`panacea://issue/${id}@0`, `panacea://issue/${id}@03`, `panacea://issue/${id}@`,
      `panacea://issue/${id}/`, `panacea://issue/${id.toUpperCase()}`, `issue:${id}@3`]) {
      expect(parseIssueLink(s)).toBeNull();
    }
  });
  it("will not build a link from an alias", () => {
    expect(() => issueLink("EP-142", 3)).toThrow(/not an issue id/);
  });
  it("writes the key and the criterion href in their own forms", () => {
    expect(issueKey(id, 3)).toBe(`issue:${id}@3`);
    expect(criterionHref("brief:bf-00000000000000a2@2", "c-91e2a4b0")).toBe("#brief:bf-00000000000000a2@2/c-91e2a4b0");
  });
});
```

#### Tests to write first — `desktop/src/components/issue/revisionWords.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { aSession } from "../../test/fixtures";
import { DYLAN, WORK_IDS, makeIssueRevision, makeServe } from "../../test/workFixtures";
import { agentsWhoRead, changeWords, lineDiff } from "./revisionWords";

const r = (revision: number, over = {}) => makeIssueRevision({ revision, ...over });

describe("changeWords", () => {
  it("says opened, restored and erased", () => {
    expect(changeWords(null, r(1))).toBe("opened");
    expect(changeWords(r(3), r(4, { restoredFrom: 2 }))).toBe("restored @2");
    expect(changeWords(r(3), r(4, { erasedAt: "2026-09-25T02:00:00Z" }))).toBe("erased");
  });
  it("names what changed in words, serves included", () => {
    const a = r(2);
    const b = r(3, { content: { ...a.content, title: { text: "Resume after restart", author: "H" },
      serves: [makeServe({ criterionId: WORK_IDS.criterionUpload })] } });
    expect(changeWords(a, b)).toBe("changed the title; serves c-40aa17c3; no longer serves c-91e2a4b0");
  });
});

describe("lineDiff", () => {
  it("keeps equal lines and marks added and removed ones", () => {
    expect(lineDiff("a\nb\nc", "a\nc\nd")).toEqual([
      { op: "same", text: "a" }, { op: "del", text: "b" }, { op: "same", text: "c" }, { op: "add", text: "d" },
    ]);
  });
});

describe("agentsWhoRead", () => {
  it("names only my agents given exactly this revision, numbered by start", () => {
    const k3 = `issue:${WORK_IDS.ep142}@3`;
    const s = [
      aSession({ id: "s-2", createdAt: "2026-09-25T01:10:00Z", issueRef: k3 }),
      aSession({ id: "s-1", createdAt: "2026-09-25T01:00:00Z", issueRef: `issue:${WORK_IDS.ep142}@2` }),
      aSession({ id: "s-3", createdAt: "2026-09-25T01:20:00Z", issueRef: `issue:${WORK_IDS.ep150}@3` }),
    ];
    expect(agentsWhoRead(k3, WORK_IDS.ep142, s, DYLAN)).toEqual(["dylan · agent 2"]);
  });
});
```

| Test | Mutant it kills |
|---|---|
| pins a link to its revision | links that always point at the head |
| round-trips | a parser that drops the revision |
| never parses an alias | alias parsing (05 §3.6) |
| refuses malformed input | a lenient parser that opens `@0` or `@03` as some revision |
| will not build from an alias | a *Copy link* that writes `EP-142` |
| key and href forms | the key form used as a link, or a navigating href |
| opened, restored, erased | a restore shown as an ordinary edit (history reads as rewritten) |
| names what changed | a change count instead of words |
| line diff | a diff that shows only the new text |
| agents who read | another issue's or another revision's agent claimed as a reader |

#### Tests to write first — `desktop/src/components/issue/Revisions.test.tsx`

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { api } from "../../api";
import { useCockpit } from "../../store/store";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { aSession } from "../../test/fixtures";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead, makeIssueRevision } from "../../test/workFixtures";
import { mutate } from "../../work/replica/ops";
import { useIssueViewData } from "./issueData";
import { IssueView } from "./IssueView";

vi.mock("../../api", () => ({ api: { work: { issueRevisions: vi.fn(), issueRevisionAt: vi.fn() } } }));
vi.mock("../../work/replica/ops", () => ({ mutate: vi.fn() }));
vi.mock("../../work/replica/replica", () => ({ useWork: vi.fn(() => []), search: vi.fn(() => []) }));
vi.mock("./issueData", () => ({ useIssueViewData: vi.fn(), useCriterion: vi.fn(() => undefined) }));

const id = WORK_IDS.ep142;
const rev = (n: number, over = {}) => makeIssueRevision({ revision: n, writtenBy: n === 5 ? ALICE : DYLAN,
  writtenAt: `2026-09-25T01:0${n}:00Z`, ...over });

function world(headRev: number, revs = [1, 2, 3, 4, 5].filter((n) => n <= headRev).map((n) => rev(n))) {
  vi.mocked(useIssueViewData).mockReturnValue({
    head: { ...makeIssueHead({ headRevision: headRev }), reassignRequest: null } as never,
    headRevision: revs[revs.length - 1], teamLabels: [], people: [DYLAN, ALICE],
  });
  vi.mocked(api.work.issueRevisions).mockResolvedValue({ items: revs, next: "" });
  vi.mocked(api.work.issueRevisionAt).mockImplementation(async (_i: string, n: number) => revs[n - 1]);
}
const open = (revision: number | null) =>
  render(<IssueView issueId={id} revision={revision} me={DYLAN} route="global" />);

beforeEach(() => { vi.mocked(mutate).mockClear(); useCockpit.setState({ sessions: {} }); });

describe("Revisions", () => {
  it("restores by writing a new revision, and keeps every old one", async () => {
    const user = userEvent.setup();
    world(3);
    const { rerender } = open(null);
    const row2 = await screen.findByText("@2");
    await user.click(within(row2.closest("li")!).getByRole("button", { name: "Restore" }));
    expect(vi.mocked(mutate).mock.calls.map((c) => c[0])).toEqual([{ kind: "restore", issueId: id, fromRevision: 2 }]);
    world(4, [rev(1), rev(2), rev(3), rev(4, { restoredFrom: 2 })]);
    rerender(<IssueView issueId={id} revision={null} me={DYLAN} route="global" />);
    await waitFor(() => expect(screen.getByText("restored @2")).toBeTruthy());
    for (const n of [1, 2, 3, 4]) expect(screen.getByText(`@${n}`)).toBeTruthy();
  });
  it("gives every revision its author class as a word", async () => {
    world(3, [rev(1), rev(2), rev(3, { content: { ...rev(3).content, description: { text: "x", author: "A" } } })]);
    const { container } = open(null);
    await screen.findByText("@1");
    const revRows = Array.from(container.querySelectorAll("li.iv-rev"));
    expect(revRows).toHaveLength(3);                       // newest first: @3 is revRows[0]
    for (const li of revRows) expect(li.textContent).toMatch(/text·[HA] · (person|agent)/);
    expect(revRows[0].textContent).toContain("text·A · agent");
  });
  it("says which of my agents read which revision", async () => {
    useCockpit.setState({ sessions: { "s-1": aSession({ id: "s-1", issueRef: `issue:${id}@3` }) } as never });
    world(5);
    open(null);
    expect(await screen.findByText("dylan · agent 1 read @3")).toBeTruthy();
  });
});

describe("the view at a revision", () => {
  it("opens at the revision it was sent to, and says the head is elsewhere", async () => {
    world(5);
    open(3);
    const note = await screen.findByRole("note");
    expect(note.textContent).toBe("You are reading @3; the head is @5 (Alice, 11:05)");
    expect(api.work.issueRevisionAt).toHaveBeenCalledWith(id, 3);
  });
  it("copies a link pinned to the revision being read, and one to the head", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    world(5);
    open(3);
    await screen.findByRole("note");
    await user.click(screen.getByRole("button", { name: "Copy link" }));
    await user.click(screen.getByRole("button", { name: "Copy link to head" }));
    expect(writeText.mock.calls).toEqual([[`panacea://issue/${id}@3`], [`panacea://issue/${id}`]]);
  });
  it("goes to the head from the reading line", async () => {
    const user = userEvent.setup();
    world(5);
    open(3);
    await user.click(within(await screen.findByRole("note")).getByRole("button"));
    expect(screen.queryByRole("note")).toBeNull();
  });
  it("makes no prediction", async () => {
    world(5);
    const { container } = open(3);
    await screen.findByText("@1");
    expectNoPrediction(container);
  });
});
```

The clock in the reading line prints local time. The expected `11:05` holds with the test run's
`TZ=Australia/Sydney` (+10:00 on 2026-09-25, before daylight saving starts on 2026-10-04), which
vitest's config must pin; if `desktop/vitest.config.ts` pins no time zone, step 1 adds
`process.env.TZ = "Australia/Sydney"` to `test/setup.ts`, or the expectation uses `clock()` of
the fixture's time (**UNVERIFIED**: no time zone pin was found in `test/setup.ts`, read
2026-09-25).

| Test | Mutant it kills |
|---|---|
| restores by writing a new revision | an in-place rewrite (a `revise` of @2, or @2 replaced in the list) |
| author class as a word | colour-only authorship on a revision |
| which of my agents read which | a reader line on the wrong revision, or none |
| opens at the revision it was sent to | silently showing the head (05 §4.5 rule 1) |
| copies a pinned link | links that always point at the head |
| goes to the head | a reading line with no way out |
| no prediction | a `%`, a bar or an ETA among revisions (`OD-27`) |

#### Tests to write first — `desktop/src/components/issue/Serves.test.tsx`

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { __resetThreadLit } from "../thread/useThreadLit";
import { BRIEF_KEY, WORK_IDS, makeIssueRevision, makeServe } from "../../test/workFixtures";
import { mutate } from "../../work/replica/ops";
import { useCriterion } from "./issueData";
import { Serves } from "./Serves";

vi.mock("../../work/replica/ops", () => ({ mutate: vi.fn() }));
vi.mock("./issueData", () => ({ useCriterion: vi.fn() }));
vi.mock("../work/CriterionPicker", () => ({ CriterionPicker: () => <div role="dialog" aria-label="Choose a criterion" /> }));

const id = WORK_IDS.ep142;
const two = makeIssueRevision({ revision: 4, content: { ...makeIssueRevision().content,
  serves: [makeServe(), makeServe({ criterionId: WORK_IDS.criterionUpload })] } });

beforeEach(() => {
  __resetThreadLit();
  vi.mocked(mutate).mockClear();
  vi.mocked(useCriterion).mockImplementation((_b, c) => ({ id: c, text: `words of ${c}`, word: "open" }));
});

describe("Serves", () => {
  it("lights its criterion on focus and never marks the row selected", () => {
    render(<Serves issueId={id} revision={two} isHead play={false} />);
    const row = screen.getByText(`words of ${WORK_IDS.criterionResume}`).closest("li")!;
    const before = row.className;
    act(() => row.focus());                // React's onFocus listens to focusin, which focus() fires
    const node = row.querySelector("a.thr-node")!;
    expect(node.getAttribute("data-lit")).toBe("true");
    expect(row.className).toBe(before);
    expect(row.hasAttribute("aria-selected")).toBe(false);
  });
  it("removes a criterion by writing a revision on the head", () => {
    render(<Serves issueId={id} revision={two} isHead play={false} />);
    fireEvent.click(screen.getByRole("button", { name: `Stop serving words of ${WORK_IDS.criterionResume}` }));
    expect(vi.mocked(mutate).mock.calls.map((c) => c[0])).toEqual([{ kind: "revise", issueId: id, baseRev: 4,
      patch: { serves: [{ briefKey: BRIEF_KEY, criterionId: WORK_IDS.criterionUpload }] } }]);
  });
  it("opens the picker on T in the list, and not on an old revision", () => {
    const { rerender } = render(<Serves issueId={id} revision={two} isHead play={false} />);
    fireEvent.keyDown(screen.getByRole("list"), { code: "KeyT", key: "t" });
    expect(screen.getByRole("dialog", { name: "Choose a criterion" })).toBeTruthy();
    rerender(<Serves issueId={WORK_IDS.ep150} revision={two} isHead={false} play={false} />);
    expect(screen.queryByRole("button", { name: /Stop serving/ })).toBeNull();
  });
  it("draws no node for a criterion whose brief is not in memory", () => {
    vi.mocked(useCriterion).mockReturnValue(undefined);
    const { container } = render(<Serves issueId={id} revision={two} isHead play={false} />);
    expect(container.querySelector("a.thr-node")).toBeNull();
  });
});
```

| Test | Mutant it kills |
|---|---|
| lights on focus, never selected | lit rendered as selection (a ring, a class or `aria-selected`) |
| removes by a revision | a mutable field patch on `serves`; a write on a stale base |
| `T` in the list, not on an old revision | a global `T`; an edit offered on a non-head revision |
| no node without its brief | a decorative, sourceless node (02 C2) |

*Undo returns it* (the scope's half of the removal test) is held where the inverse is made: task
56's `U` sends the inverse of a `revise` as another `revise` (`tasks-41-60.md:507-509`). This part
holds the half it owns, that removal **is** a `revise`, so its inverse cannot be a field patch.

#### Steps

- [ ] **Step 1: read on the sync base** task 36's `components/thread/` (SW.14 as built), task 42's
  `CriterionPicker` props, task 37's `WorkClient` names for the two revision reads, task 56's
  `WorkOp` members, and task 46's `IssueView.tsx` as landed. Rename in this part's files only.
- [ ] **Step 2: the four test files first**; `npm --prefix desktop test -- links revisionWords
  Revisions Serves` fails on the missing modules.
- [ ] **Step 3: `links.ts`** and **`revisionWords.ts`**; their two test files pass.
- [ ] **Step 4: `useRevision.ts`, `ReadingLine.tsx`, `Revisions.tsx`**, the `useCriterion`
  append to `issueData.ts`, then **`Serves.tsx`**.
- [ ] **Step 5: `IssueView.tsx`** as above, and the `issue.css` append; task 46's tests stay green.
- [ ] **Step 6: run the acceptance commands**; commit by path, one commit per step 3-5, each
  message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- links revisionWords Revisions Serves IssueHead pickers
# expect: Test Files 6 passed (6); Tests 40 passed (40)   (21 here, 19 from task 46)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rn "EP-[0-9]" desktop/src/work/links.ts | wc -l
# expect: 0 (no alias form in the link code)
```

This part's count: 6 in `links.test.ts`, 4 in `revisionWords.test.ts`, 7 in `Revisions.test.tsx`,
4 in `Serves.test.tsx` = 21. Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- The https Hub link and the read-only page (`L-98`'s W2 part, task 80).
- That `panacea://` is registered with Windows, so a pasted link opens the app: **UNVERIFIED**
  whether the desktop has the scheme; nothing here registers it.
- That the clipboard write works in the Tauri webview (**UNVERIFIED**, see the `IssueView` note).
- That the Hub writes a restore as a new revision: task 09's store and task 04's fake hold it;
  this part sends the op and renders what comes back.
- What the criterion's hash `href` opens (Discover, phase 4).
- Moving an issue between projects (`L-35`, W2); editing the description (task 49).

**Parity** (`05-design.md` rows). Closes **L-34** (`05:143`: history, restore as a new revision,
the exact revision an agent read) with task 09, and **L-98**'s W1 part (`05:232`: the
revision-pinned `panacea://` link). Serves **L-191** (`05:358`): task 19 writes the revision into
the managed block; this view shows which one each of the reader's agents was given.

#### Gaps found while writing this part

1. **`WorkOp` needs `restore {issueId, fromRevision}`** (decision 2); task 56's union lists none.
2. **The replica read `criterion(briefKey, criterionId)`** is a fifth read for tasks 55-57, shared
   with task 40's groups.
3. **Task 37's `WorkClient` names** `issueRevisions` and `issueRevisionAt` are assumed from task
   16's Go rows.
4. **Task 42's `CriterionPicker` props** (`exclude`, `onChoose(serve)`, `onClose`) are assumed.
5. **No time zone pin for tests** was found in `test/setup.ts`; any test printing a clock time
   needs one (task 46's tests print none).
6. `ThreadTrace`'s `play` is set by the caller (`task-36.md:224-227`); this part passes
   `drillIn`, which task 40's row must set when it opens the view. That prop is new to the mount.
7. The File map's `src/components/issue/` row lists none of `ReadingLine.tsx`, `useRevision.ts`,
   `revisionWords.ts`, and there is no row for `src/work/links.ts`.

