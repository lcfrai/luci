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

