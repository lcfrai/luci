### Task 47: the issue view, part 2: revisions, restore, `serves` with `thread.node`, revision-pinned links

Stream **WD3** (the issue view), about 3 hours (an estimate, not a measurement). Depends on task 36
(SW.14: `ThreadNode`, `ThreadLine`, `useThreadLit`) and task 46 (the view's frame, `AuthorClass`,
`issueData.ts`, `clock`). Uses SW.1 (the immutable, digested revision `issue:is-…@r` and its
`serves [{briefKey, criterionId}]`), SW.2's relay reads of revisions (task 16), task 42's
`CriterionPicker` and SW.16's `mutate`.

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
   (`IssueRevisionContent`, `task-02.md:239-243`), so removal is `revise` with the head as
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

/** The criteria this revision serves. Editable only on the head: removing or adding writes a
 *  new revision on top of it (decision 3). `play` is true on a drill-in (ThreadTrace, SW.14). */
export function Serves(props: { issueId: string; revision: IssueRevision; isHead: boolean; play: boolean }): JSX.Element {
  const { issueId, revision, isHead, play } = props;
  const [adding, setAdding] = useState(false);
  const serves = revision.content.serves;
  const write = (next: Serve[]) =>
    mutate({ kind: "revise", issueId, baseRev: revision.revision, patch: { serves: next } });
  const onKey = (e: KeyboardEvent<HTMLUListElement>) => {
    // Keys fire only with focus in a list (05 §4.3); `T` is KeyT, no modifier (useKeyboardMap rule).
    if (!isHead || e.code !== "KeyT" || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
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

