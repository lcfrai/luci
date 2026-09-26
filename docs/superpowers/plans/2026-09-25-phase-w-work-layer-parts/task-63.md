### Task 63: the Desk: issue rows in NEEDS YOU, aging, *blocking someone's agent*

Stream **WD2**, about 3 hours (an estimate, not a measurement). Depends on task 62 (SW.17:
`registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskSnapshot`, `takeDeskSnapshot`,
`task-62.md`) and task 55 (the replica behind task 40's `WorkReader`; SW.16). Uses task 56's
`mutate` and `WorkOp` (`task-56.md:156-169`), task 42's `acceptTarget`, `nextMorning` and its
`snooze` op (`task-42.md:174-222`), task 46's `handMove` (`task-46.md:248`), task 51's
`comments(issueId)` read (`task-51.md:601-611`, `task-56.md:184`), task 29's `MarkGlyph`
(`task-29.md:145-158`), task 53's `useVerb` and the `triage`, `issueRow` scopes
(`task-53.md:106`, `:183-193`, `:310-316`), SW.5's feed kinds (`task-13.md:122-138`) and the built
`formatWaiting` (`desktop/src/lib/time.ts:49-52`) and heartbeat (`hooks/heartbeat.ts:64`). Task 55
has no part on the tree at this writing: its feed read is **UNVERIFIED** (gap 2).

After this task the Desk's NEEDS YOU section holds the person's own issue work: a triage issue
assigned to them, an urgent issue of theirs, a comment that names them, and an issue of theirs
that blocks a started issue; each says why it ranks where it does, what happens if they do
nothing, and how long it has waited.

**What the reader sees** (05 §4.2's NEEDS YOU row, `05-design.md:932-937`; §4.4 rule 2; AX §5;
`prototype/work-layer.html:764-766`):

- **A blocking row first**: *"EP-150 blocks EP-142 (Alice) · started 1h 12m ago"*, `waiting 1h
  12m` in mono, and under it *"ranked first: blocks EP-142, which is started"* and *"if you do
  nothing: EP-142 stays blocked by EP-150"*.
- **A triage row**: *EP-160*, its title, *"in triage, assigned to you"*, `waiting 2h 10m`, and two
  buttons, *Accept `1`* and *Snooze `4`*, that answer in place.
- **An urgent row**: the `needs_you` mark in `--need` and the words *"urgent, assigned to you"*.
  Urgency is colour **here only**; elsewhere priority is a notch in ink (05 §4.4 rule 2).
- **A mention row**: *"Alice named you on EP-163"* and the comment's first line, `text·H`.
- **No reviewer row**: `IssueHead` has no reviewer field, so *"in review where you are the
  reviewer"* is not built (gap 1).
- The age ticks on the one heartbeat, and a row that stops being true stays while the Desk is
  open; it is gone on the next visit (task 62's decision 3).

**Decisions made here, so nobody re-derives them:**

1. **Four kinds, one pure file.** `needs.ts` holds four selectors over a `DeskSnapshot` and returns
   `DeskItem<NeedPayload>[]`; it has no React, no store and no clock (`snap.takenAt` is the now).
   `deskRows.tsx` registers each kind's row and source (task 62's decision 2).
2. **Mine means assigned to me.** `issue.triage_mine` is `state === "triage"` and `assignee.id ===
   snap.me`; `issue.urgent_mine` is `priority === "urgent"`, assignee me, and a state not in
   `CLOSED` (`landed`, `done`, `cancelled`, `duplicate`). An unassigned triage issue is the Triage
   view's (task 42), not a Desk row: nobody is named, so nobody is needed.
3. **A mention is an unresolved comment whose `mentions` holds my id**, on any readable issue, not
   written by me. W1 does not infer *"a question"* from text (scope), so any unresolved mention
   qualifies; `resolvedAt !== null` makes no row.
4. **Blocking reads the stored relation.** X blocks Y when X is mine, X's `relations` holds
   `{kind: "blocks", issueId: Y}` (`task-37.md:118`), X is not `landed` or closed, and Y is `started`
   or `in_review`. Y's state is the Hub's convergence word that a lane is bound to it (task 22); the
   Desk never reads Alice's sessions (OD-15). Tier 1; the rest are tier 2 (AX §5, task 62's
   `DeskRank`).
5. **Age comes from the feed, then the record.** `since` is the newest SW.5 event on the subject
   that made the row true (triage: `issue.created` or `issue.state` with `to: "triage"`, or
   `issue.updated` naming `assignee`; urgent: `issue.updated` naming `priority` or `assignee`;
   mention: `comment.mentioned` with `data.commentId`; blocking: Y's `issue.state` with `to` in
   `started`/`in_review`), else the head's `updatedAt`, else the comment's `createdAt`. It is fixed
   in the snapshot, so a re-render never restarts it; `formatWaiting(since, useHeartbeat())` prints
   it. `issue.updated`'s `fields` are wire names (`task-13.md:150`), matched as `assignee` and
   `priority` (**UNVERIFIED** against task 11's field list, gap 3).
6. **Words carry the rank and the consequence, as rules.** `rankReason` and `ifNothing` are
   sentences built from the snapshot's facts (L-114), never a time: *"if you do nothing: EP-160
   stays in triage and nothing starts on it"*. No `~`, no *"will"* with a time, no cost of waiting
   in money (no gateway in W1; the wait is shown as time, `OD-27`).
7. **Answer in place, through the verbs that exist.** A triage row's *Accept* sends task 42's
   `{kind: "setState", move: handMove(acceptTarget(policy))}` and *Snooze* task 42's `{kind:
   "snooze", until: nextMorning(now, zone)}`, both through `mutate`; a refusal comes back through
   `settled` and the row says the Hub's words in place (task 56). `I` assigns to me (task 53's
   `issue.assignMe`) and `Enter` opens the issue view beside the Desk (`open.issue`). The Desk does
   not navigate away (AX §5). The team policy is read as task 42's mount reads it (**UNVERIFIED**,
   gap 4); with none, `acceptTarget(null)` is `backlog` (`task-42.md:179-182`).
8. **An answered row stays, answered.** After *Accept* the row reads *"Accepted · backlog"* and
   keeps its place; the next visit drops it (task 62's decision 3). Nothing slides out.
9. **Two additive deltas on task 62's files, no rename.** `DeskItem` gains `keyScopes?: readonly
   KeyScope[]`, which `DeskSection` joins after `issueRow` on the wrapper, so `1` and `4` reach a
   triage row through task 53's `triage` scope; `DeskSnapshot` gains `comments(issueId)`, read once
   per issue at entry from the reader's `comments` (task 51's sixth read). `DeskInputs` gains
   nothing: `events` is task 62's optional input, which `liveDeskInputs` now fills from task 55's
   feed.
10. **The snapshot moves onto the replica by installing it, not by a new read.** Task 62's snapshot
    already reads `workSource()` (task 40's seam). Task 55 installs its Replica there
    (`task-40.md:162`), so once it boots, the Desk reads the replica with no change here. This task
    adds only the feed: `liveDeskInputs.events = () => replicaEvents()` (gap 2).

**Files** (the skeleton's file map covers `desk/*` for tasks 62-65, `:311`; the scope names
`rows/IssueNeedRow.tsx` and `work/desk/needs.ts`, `tasks-61-80.md:103-105`):

- `desktop/src/work/desk/needs.ts` — the four selectors, `since`, the words (new)
- `desktop/src/work/desk/needs.test.ts` — the pure tests (new)
- `desktop/src/components/desk/rows/IssueNeedRow.tsx` — the row and its answers (new)
- `desktop/src/components/desk/rows/IssueNeedRow.test.tsx` — the surface tests (new)
- `desktop/src/components/desk/deskRows.tsx` ⚑ — task 63's block, first after the head (modified)
- `desktop/src/components/desk/deskModel.ts` ⚑ — `keyScopes?` and `comments` (modified, additive)
- `desktop/src/components/desk/DeskSection.tsx` ⚑ — the wrapper joins `keyScopes` (modified)
- `desktop/src/components/desk/Desk.tsx` ⚑ — `events` in `liveDeskInputs`; `useDeskIssueVerbs()` (modified)
- `desktop/src/components/desk/desk.css` ⚑ — `.dn-*`, appended (modified)

#### Interfaces — complete: `desktop/src/work/desk/needs.ts`, part 1: the payload, `since`, the words

```ts
// desktop/src/work/desk/needs.ts — task 63. NEEDS YOU's issue rows, pure over one DeskSnapshot.
import type { IssueHead, IssueState, User } from "../../api/work";
import type { DeskItem, DeskRank, DeskSnapshot } from "../../components/desk/deskModel";

export type NeedKind = "issue.triage_mine" | "issue.urgent_mine" | "issue.mention" | "issue.blocking";
export const NEED_KINDS: readonly NeedKind[] =
  Object.freeze(["issue.triage_mine", "issue.urgent_mine", "issue.mention", "issue.blocking"]);

export interface NeedPayload {
  need: NeedKind;
  issueId: string; alias: string; title: string;       // the issue the row acts on (X for blocking)
  state: IssueState;
  since: string;                                       // RFC 3339, fixed at entry (decision 5)
  sinceFrom: "event" | "record";
  blocked?: { issueId: string; alias: string; owner: string; state: IssueState; startedAt: string };
  mention?: { commentId: string; author: User; firstLine: string };
}

export const CLOSED: ReadonlySet<IssueState> = new Set<IssueState>(["landed", "done", "cancelled", "duplicate"]);
const mine = (h: IssueHead, me: string | null): boolean => me !== null && h.assignee?.id === me;
const titleOf = (s: DeskSnapshot, h: IssueHead): string =>
  s.revisionAt(h.id, h.headRevision)?.content.title.text ?? h.alias;
const fieldsOf = (d: Record<string, unknown>): string[] => String(d.fields ?? "").split(",");

/** Decision 5: the newest matching feed event on the subject, else the record's own time. */
export function sinceOf(s: DeskSnapshot, subject: string, match: (kind: string, data: Record<string, unknown>) => boolean,
  fallback: string): { since: string; sinceFrom: "event" | "record" } {
  let best: string | null = null;
  for (const e of s.events) {
    if (e.subject !== subject || !match(e.kind, e.data)) continue;
    if (best === null || Date.parse(e.at) > Date.parse(best)) best = e.at;
  }
  return best === null ? { since: fallback, sinceFrom: "record" } : { since: best, sinceFrom: "event" };
}

export const MATCH = {
  triage: (k: string, d: Record<string, unknown>) => k === "issue.created" || (k === "issue.state" && d.to === "triage")
    || (k === "issue.updated" && fieldsOf(d).includes("assignee")),
  urgent: (k: string, d: Record<string, unknown>) => k === "issue.updated"
    && (fieldsOf(d).includes("priority") || fieldsOf(d).includes("assignee")),
  started: (k: string, d: Record<string, unknown>) => k === "issue.state" && (d.to === "started" || d.to === "in_review"),
};

/** The row's first line: what is true, in words. */
export function needWords(p: NeedPayload): string {
  switch (p.need) {
    case "issue.triage_mine": return "in triage, assigned to you";
    case "issue.urgent_mine": return "urgent, assigned to you";
    case "issue.mention": return `${p.mention!.author.displayName} named you on ${p.alias}`;
    case "issue.blocking": return `${p.alias} blocks ${p.blocked!.alias} (${p.blocked!.owner}) · started`;
  }
}

/** AX §5's two lines, as rules (decision 6). */
export function rankWords(p: NeedPayload): { rankReason: string; ifNothing: string } {
  switch (p.need) {
    case "issue.blocking": return { rankReason: `ranked first: blocks ${p.blocked!.alias}, which is ${p.blocked!.state.replace("_", " ")}`,
      ifNothing: `if you do nothing: ${p.blocked!.alias} stays blocked by ${p.alias}` };
    case "issue.triage_mine": return { rankReason: "in triage and assigned to you",
      ifNothing: `if you do nothing: ${p.alias} stays in triage and nothing starts on it` };
    case "issue.urgent_mine": return { rankReason: "urgent and assigned to you",
      ifNothing: `if you do nothing: ${p.alias} stays ${p.state.replace("_", " ")} until you or your agent act` };
    case "issue.mention": return { rankReason: `${p.mention!.author.displayName} named you`,
      ifNothing: `if you do nothing: the comment on ${p.alias} stays open` };
  }
}

/** Decision 4: blocking is tier 1, the rest tier 2; the wait is from `since`. */
export function needRank(item: DeskItem<NeedPayload>, snap: DeskSnapshot): DeskRank {
  return { tier: item.payload.need === "issue.blocking" ? 1 : 2,
    waitingSinceMs: Math.max(0, snap.takenAt - Date.parse(item.payload.since)) };
}
```

`User.displayName` is task 37's (`task-61.md:162` reads it the same way). The owner in the blocking
line is Y's assignee's `displayName`, *"you"* when it is me, and *"unassigned"* when there is none.

#### Interfaces — complete: `needs.ts`, part 2: the four selectors

```ts
type Item = DeskItem<NeedPayload>;
function item(s: DeskSnapshot, id: string, base: Omit<NeedPayload, "title" | "state">, h: IssueHead,
  scopes: DeskItem["keyScopes"] = undefined): Item {
  const payload: NeedPayload = { ...base, title: titleOf(s, h), state: h.state };
  return { kind: base.need, id, payload, arrivedAt: base.since, issueId: h.id, ...rankWords(payload),
    ...(scopes ? { keyScopes: scopes } : {}) };
}

export function triageMine(s: DeskSnapshot): Item[] {
  return [...s.heads.values()].filter((h) => h.state === "triage" && mine(h, s.me)).map((h) =>
    item(s, h.id, { need: "issue.triage_mine", issueId: h.id, alias: h.alias,
      ...sinceOf(s, h.id, MATCH.triage, h.updatedAt) }, h, ["triage"]));
}

export function urgentMine(s: DeskSnapshot): Item[] {
  return [...s.heads.values()].filter((h) => h.priority === "urgent" && mine(h, s.me) && !CLOSED.has(h.state)
    && h.state !== "triage")                                        // a triage row already says it
    .map((h) => item(s, h.id, { need: "issue.urgent_mine", issueId: h.id, alias: h.alias,
      ...sinceOf(s, h.id, MATCH.urgent, h.updatedAt) }, h));
}

export function mentionsOfMe(s: DeskSnapshot): Item[] {
  const out: Item[] = [];
  if (s.me === null) return out;
  for (const h of s.heads.values()) {
    for (const c of s.comments(h.id)) {
      if (c.resolvedAt !== null || c.erasedAt !== null || c.createdBy.id === s.me || !c.mentions.includes(s.me)) continue;
      const at = sinceOf(s, s.me, (k, d) => k === "comment.mentioned" && d.commentId === c.id, c.createdAt);
      out.push(item(s, `${h.id}/${c.id}`, { need: "issue.mention", issueId: h.id, alias: h.alias, ...at,
        mention: { commentId: c.id, author: c.createdBy, firstLine: c.body.text.split("\n")[0].slice(0, 120) } }, h));
    }
  }
  return out;
}

export function blockingMine(s: DeskSnapshot): Item[] {
  const out: Item[] = [];
  for (const x of s.heads.values()) {
    if (!mine(x, s.me) || CLOSED.has(x.state)) continue;
    for (const r of x.relations) {
      const y = r.kind === "blocks" ? s.headOf(r.issueId) : undefined;
      if (!y || (y.state !== "started" && y.state !== "in_review")) continue;
      const started = sinceOf(s, y.id, MATCH.started, y.updatedAt);
      const owner = y.assignee === null ? "unassigned" : y.assignee.id === s.me ? "you" : y.assignee.displayName;
      out.push(item(s, `${x.id}>${y.id}`, { need: "issue.blocking", issueId: x.id, alias: x.alias, ...started,
        blocked: { issueId: y.id, alias: y.alias, owner, state: y.state, startedAt: started.since } }, x));
    }
  }
  return out;
}

export const NEED_SOURCES: Readonly<Record<NeedKind, (s: DeskSnapshot) => Item[]>> = Object.freeze({
  "issue.triage_mine": triageMine, "issue.urgent_mine": urgentMine,
  "issue.mention": mentionsOfMe, "issue.blocking": blockingMine,
});
```

An urgent triage issue of mine is one row, the triage row: the triage row asks for the act that
unblocks it, and two rows for one issue would count one need twice. The mention's subject in SW.5
is the mentioned user (`task-20.md:337`, `subject: WORK_IDS.dylan`), so `sinceOf` looks it up
under `s.me`. `IssueComment.body` is `AuthoredText` and its writer is `createdBy`
(`task-37.md:154-158`); the first line is shown as the author wrote it, never summarised, and an
erased comment makes no row.

#### Interfaces — complete: `desktop/src/components/desk/rows/IssueNeedRow.tsx`

```tsx
// desktop/src/components/desk/rows/IssueNeedRow.tsx — one NEEDS YOU issue row (task 63).
// Answers in place (AX §5). The age is the only thing that ticks, on the one heartbeat.
import { useState } from "react";
import { useHeartbeat } from "../../../hooks/heartbeat";
import { formatElapsed, formatWaiting } from "../../../lib/time";
import { MarkGlyph } from "../../glyphs/MarkGlyph";
import type { DeskItem } from "../deskModel";
import { needWords, type NeedPayload } from "../../../work/desk/needs";

export interface NeedVerbs {
  /** Resolves "applied" or rejects with the Hub's message (task 56's settled, as task 65's `stood`). */
  accept(issueId: string): Promise<string>;              // the state it went to, "backlog"
  snooze(issueId: string): Promise<string>;              // the morning it wakes, "09:00"
}

type Answer = { kind: "idle" } | { kind: "sending" } | { kind: "done"; words: string } | { kind: "refused"; words: string };

function Age({ since }: { since: string }): JSX.Element {
  const now = useHeartbeat();
  return <span className="dn-age">{formatWaiting(since, now)}</span>;
}
function Started({ at }: { at: string }): JSX.Element {
  const now = useHeartbeat();
  return <span className="dn-age">{` ${formatElapsed(at, now)} ago`}</span>;
}

export function IssueNeedRow({ item, verbs }: { item: DeskItem<NeedPayload>; verbs: NeedVerbs }): JSX.Element {
  const p = item.payload;
  const [answer, setAnswer] = useState<Answer>({ kind: "idle" });
  const act = (run: () => Promise<string>, done: (w: string) => string) => {
    setAnswer({ kind: "sending" });
    run().then((w) => setAnswer({ kind: "done", words: done(w) }),
      (e: unknown) => setAnswer({ kind: "refused", words: e instanceof Error ? e.message : String(e) }));
  };
  return (
    <div className="dn" data-need={p.need}>
      <div className="dn-line">
        {p.need === "issue.urgent_mine" ? <MarkGlyph kind="needs_you" /> : null}
        <span className="dn-alias">{p.alias}</span>
        <span className="dn-title">{p.title}</span>
        <span className={p.need === "issue.urgent_mine" ? "dn-what dn-urgent" : "dn-what"}>
          {needWords(p)}{p.blocked ? <Started at={p.blocked.startedAt} /> : null}
        </span>
        <span className="dn-sp" />
        <Age since={p.since} />
      </div>
      {p.mention ? <p className="dn-quote">{p.mention.firstLine}</p> : null}
      <p className="dn-why">{item.rankReason}</p>
      <p className="dn-why">{item.ifNothing}</p>
      {p.need === "issue.triage_mine" ? (
        <div className="dn-acts">
          {answer.kind === "done" || answer.kind === "refused"
            ? <span className="dn-said" role="status">{answer.words}</span>
            : (<>
              <button type="button" disabled={answer.kind === "sending"}
                onClick={() => act(() => verbs.accept(p.issueId), (s) => `Accepted · ${s.replace("_", " ")}`)}>Accept <kbd>1</kbd></button>
              <button type="button" disabled={answer.kind === "sending"}
                onClick={() => act(() => verbs.snooze(p.issueId), (t) => `Snoozed until ${t}`)}>Snooze <kbd>4</kbd></button>
            </>)}
        </div>) : null}
    </div>
  );
}
```

The row shows no count, bar or `%`. *"Accepted · backlog"* names the state the Hub took, read from
`acceptTarget`, and the row keeps its place (decision 8). `--need` colours only the urgent row's
glyph and its words (05 §4.4 rule 2); task 29's `MarkGlyph` draws the shape, so urgency is never
colour alone.

#### Interfaces — complete: task 63's block in `deskRows.tsx`, and the verbs

```tsx
// deskRows.tsx — task 63's block, first after task 62's head.
import { acceptTarget, nextMorning } from "../work/TriageView";       // task 42 (task-42.md:70, :179-196)
import { handMove } from "../issue/pickers/StatePicker";              // task 46 (task-46.md:236, :248)
import { mutate } from "../../work/replica/replica";                  // SW.16, task 55/56
import { workSource } from "../../work/source";
import { useVerb } from "../../work/keys/keymap";
import { NEED_KINDS, NEED_SOURCES, needRank, type NeedPayload } from "../../work/desk/needs";
import { IssueNeedRow, type NeedVerbs } from "./rows/IssueNeedRow";
import { registerDeskRow as row, registerDeskSource as source } from "./deskModel";

const stood = async (h: { settled: Promise<"applied" | { message: string }> }): Promise<void> => {
  const r = await h.settled;
  if (r !== "applied") throw new Error(r.message);                    // the Hub's words, in place
};
const zone = (): string => Intl.DateTimeFormat().resolvedOptions().timeZone;
const hhmm = (iso: string): string => new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });

export const needVerbs: NeedVerbs = {
  accept: async (issueId) => {
    const to = acceptTarget(teamPolicyOf(issueId));
    await stood(mutate({ kind: "setState", issueId, move: handMove(to) }));
    return to;
  },
  snooze: async (issueId) => {
    const until = nextMorning(Date.now(), zone());
    await stood(mutate({ kind: "snooze", issueId, until }));
    return hhmm(until);
  },
};

for (const kind of NEED_KINDS) {
  source(kind, NEED_SOURCES[kind]);
  row<NeedPayload>(kind, "needs_you", (item) => <IssueNeedRow item={item} verbs={needVerbs} />, needRank);
}

/** The Desk's issue verbs (decision 7). Desk.tsx calls it once; the Work lens is unmounted then. */
export function useDeskIssueVerbs(): void {
  useVerb("triage.accept", (r) => { if (r.issueId) void needVerbs.accept(r.issueId).catch(() => {}); });
  useVerb("triage.snooze", (r) => { if (r.issueId) void needVerbs.snooze(r.issueId).catch(() => {}); });
  useVerb("issue.assignMe", (r) => {
    const me = workSource()?.me();
    if (r.issueId && me) void mutate({ kind: "setAssignee", issueId: r.issueId, assignee: me });
  });
}
```

`teamPolicyOf(issueId)` is the read task 42's mount uses for `TriageView`'s `policy` prop
(`task-42.md:204`); its name and home are task 42's and task 55's (**UNVERIFIED**, gap 4). A key's
answer is shown by the row's own buttons only when pressed there; a `1` pressed on the row reaches
the same `needVerbs.accept`, and the replica's paint changes the head, which the open Desk does not
re-rank (task 62's decision 3), so the row stays with its buttons until the next visit (gap 5).
`open.issue` (`Enter`) is task 45's issue view, registered by its owner; the Desk adds no runner.

#### Interfaces — complete: the additive deltas on task 62's files

```ts
// deskModel.ts ⚑ — DeskItem gains one optional member; DeskSnapshot gains one read.
import type { KeyScope } from "../../work/keys/keymap";
import type { IssueComment } from "../../api/work";
//   in DeskItem:     keyScopes?: readonly KeyScope[];           // joined after "issueRow"
//   in DeskSnapshot: comments(issueId: string): readonly IssueComment[];
//   in takeDeskSnapshot, after heads are read:
  const comments = new Map<string, readonly IssueComment[]>();
  for (const id of heads.keys()) comments.set(id, work?.comments?.(id) ?? []);
//   and in the frozen result:  comments: (id: string) => comments.get(id) ?? [],
```

```tsx
// DeskSection.tsx ⚑ — the wrapper's scope attribute.
{...(item.issueId ? { "data-issue-id": item.issueId,
  "data-key-scope": ["issueRow", ...(item.keyScopes ?? [])].join(" ") } : {})}
```

```tsx
// Desk.tsx ⚑ — the feed input (decision 10) and the verbs.
import { replicaEvents } from "../../work/replica/replica";          // task 55 (UNVERIFIED name, gap 2)
import { useDeskIssueVerbs } from "./deskRows";
export const liveDeskInputs: DeskInputs = {
  now: () => Date.now(), cockpit: () => useCockpit.getState(), work: () => workSource(),
  events: () => replicaEvents(),
};
//   first line of Desk's body:  useDeskIssueVerbs();
```

`work?.comments?.(id)`: `comments` is task 51's sixth read on the reader (`task-51.md:957`); the
optional call keeps task 62's tests, whose reader is `null`, unchanged. Every head's comments are
copied at entry, so the snapshot stays a value (task 62's decision 3).

#### Interfaces — complete: appended to `desktop/src/components/desk/desk.css`

```css
/* NEEDS YOU issue rows (task 63). --need colours the urgent row only (05 §4.4 rule 2). */
.dn { display: grid; gap: var(--s-1); padding: var(--s-2) var(--s-3); }
.dn-line { display: flex; align-items: baseline; gap: var(--s-2); font: var(--t-ui) / var(--t-ui-lh) var(--sans); }
.dn-alias, .dn-age { font-family: var(--mono); font-variant-numeric: tabular-nums; color: var(--ink-2); }
.dn-title { color: var(--ink); }
.dn-what { color: var(--ink-2); }
.dn-urgent { color: var(--need); }
.dn-sp { flex: 1; }
.dn-quote, .dn-why, .dn-said { margin: 0; font: var(--t-ui) / var(--t-ui-lh) var(--sans); color: var(--ink-2); }
.dn-quote { color: var(--ink); }
.dn-acts { display: flex; gap: var(--s-2); }
.dn-acts button { border-radius: var(--r-ctl); box-shadow: var(--e-btn); background: var(--surface);
  color: var(--ink); border: 0; padding: 0 var(--s-2); cursor: pointer; }
.dn-acts kbd { font-family: var(--mono); color: var(--ink-2); }
```

`--need` is the tree's (`tokens.css:75`), `--e-btn` too (`:119`). Nothing here animates.

#### Tests to write first, part 1 — `desktop/src/work/desk/needs.test.ts` (the pure selectors)

```ts
import { describe, expect, it } from "vitest";
import type { DeskSnapshot } from "../../components/desk/deskModel";
import { ALICE, DYLAN, WORK_IDS, makeComment, makeIssueHead, makeWorkEvent } from "../../test/workFixtures";
import { blockingMine, mentionsOfMe, needRank, triageMine, urgentMine } from "./needs";
import type { IssueHead } from "../../api/work";

const T = (hhmm: string) => `2026-09-25T${hhmm}:00Z`;
function snap(heads: IssueHead[], over: Partial<DeskSnapshot> = {}): DeskSnapshot {
  const m = new Map(heads.map((h) => [h.id, h]));
  return { takenAt: Date.parse(T("11:10")), lastVisitAt: null, me: DYLAN.id, cockpit: {} as never, sessions: [],
    heads: m, headOf: (id) => m.get(id), events: [], revisionAt: () => null, refOf: () => null,
    briefCriteria: () => null, signals: () => [], comments: () => [], ...over } as DeskSnapshot;
}
const X = (over: Partial<IssueHead>) => makeIssueHead({ id: WORK_IDS.ep150, alias: "EP-150", state: "todo", ...over });
const Y = (over: Partial<IssueHead>) => makeIssueHead({ id: WORK_IDS.ep142, alias: "EP-142", assignee: ALICE, ...over });

describe("needs", () => {
  it("a triage issue assigned to Alice makes no row of mine", () => {
    expect(triageMine(snap([X({ state: "triage", assignee: ALICE })]))).toEqual([]);
    expect(triageMine(snap([X({ state: "triage" })])).map((i) => i.id)).toEqual([WORK_IDS.ep150]);
    // mutant: every triage issue
  });
  it("urgent and mine makes a row; high and mine does not", () => {
    expect(urgentMine(snap([X({ priority: "urgent" })])).length).toBe(1);
    expect(urgentMine(snap([X({ priority: "high" })]))).toEqual([]);
    // mutant: priority as the need
  });
  it("age is the feed event that made the row true, else the record", () => {
    const h = X({ state: "triage", updatedAt: T("10:30") });
    expect(triageMine(snap([h]))[0].payload).toMatchObject({ since: T("10:30"), sinceFrom: "record" });
    const e = makeWorkEvent({ seq: 9, at: T("09:00"), kind: "issue.state", subject: h.id, data: { to: "triage", cause: "hand" } });
    expect(triageMine(snap([h], { events: [e] }))[0].payload).toMatchObject({ since: T("09:00"), sinceFrom: "event" });
    // mutant: age from first render (Date.now at build)
  });
  it("X blocks Y: Y in todo makes no row, Y started makes a tier-1 row", () => {
    const x = X({ relations: [{ kind: "blocks", issueId: WORK_IDS.ep142 }] });
    expect(blockingMine(snap([x, Y({ state: "todo" })]))).toEqual([]);
    const [row] = blockingMine(snap([x, Y({ state: "started", updatedAt: T("09:58") })]));
    expect(row.payload.blocked).toMatchObject({ alias: "EP-142", owner: "Alice Example", state: "started" });
    expect(needRank(row, snap([])).tier).toBe(1);
    expect(row.ifNothing).toBe("if you do nothing: EP-142 stays blocked by EP-150");
    // mutant: any blocks relation, whatever Y's state
  });
  it("a resolved mention makes no row; an open one names its author", () => {
    const c = makeComment({ issueId: WORK_IDS.ep163, mentions: [DYLAN.id], createdBy: ALICE });
    const h = makeIssueHead({ id: WORK_IDS.ep163, alias: "EP-163", assignee: ALICE });
    expect(mentionsOfMe(snap([h], { comments: () => [{ ...c, resolvedAt: T("10:00"), resolvedBy: ALICE }] }))).toEqual([]);
    expect(mentionsOfMe(snap([h], { comments: () => [c] }))[0].payload.mention!.author).toEqual(ALICE);
    // mutant: resolvedAt ignored
  });
  it("the blocking row ranks above an older urgent row", () => {
    const s = snap([]);
    const urgent = { ...urgentMine(snap([X({ priority: "urgent", updatedAt: T("06:00") })]))[0] };
    const block = { ...urgent, payload: { ...urgent.payload, need: "issue.blocking" as const, since: T("11:00") } };
    expect(needRank(block, s).tier).toBeLessThan(needRank(urgent, s).tier);
    // mutant: waitingSinceMs before tier
  });
});
```

`makeComment` takes `issueId`, `mentions` and `createdBy` overrides (task 20's builder,
`task-20.md:217-241`); if its override names differ, the test follows the builder. `ALICE`'s
`displayName` is *Alice Example* (`task-61.md:685`).

#### Tests to write first, part 2 — `desktop/src/components/desk/rows/IssueNeedRow.test.tsx` (on the Desk)

The Desk is rendered with the kinds `deskRows.tsx` registers at load (task 62's gap 7: no registry
reset here), over task 40's fixture world, with the store's one project so the snapshot reads its
heads. The replica's `mutate` and feed are faked: task 55's module has no part on the tree.

```tsx
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __resetHeartbeat } from "../../../hooks/heartbeat";
import { resetCockpit, useCockpit } from "../../../store/store";
import { expectNoPrediction } from "../../../test/expectNoPrediction";
import { ALICE, WORK_IDS } from "../../../test/workFixtures";
import { setWorkSourceForTest, type WorldHandle } from "../../../work/fixtureSource";
import { workSource } from "../../../work/source";
import { Desk } from "../Desk";
import type { DeskInputs } from "../deskModel";

const mutate = vi.fn(() => ({ id: "op-1", undoable: true, undo: vi.fn(), settled: Promise.resolve("applied" as const) }));
vi.mock("../../../work/replica/replica", () => ({ mutate: (op: unknown) => mutate(op), replicaEvents: () => [] }));

const T0 = Date.parse("2026-09-25T11:10:00Z");
let world: WorldHandle;
const inputs: DeskInputs = { now: () => Date.now(), cockpit: () => useCockpit.getState(), work: () => workSource() };
const head = (id: string, over: object) => world.update((w) => ({ ...w,
  items: w.items.map((i) => (i.head.id === id ? { ...i, head: { ...i.head, ...over } } : i)) }));
const needs = () => document.querySelector<HTMLElement>('[data-desk-section="needs_you"]')!;

beforeEach(() => {
  vi.useFakeTimers({ now: T0 });
  resetCockpit(); __resetHeartbeat(); mutate.mockClear();
  useCockpit.setState({ projects: [{ id: WORK_IDS.project, name: "Evidence", path: "C:/p" }] } as never);
  world = setWorkSourceForTest();
  head(WORK_IDS.ep142, { state: "started", assignee: ALICE, updatedAt: "2026-09-25T09:00:00Z" });  // EP-150 blocks it
});
afterEach(() => { world.dispose(); vi.useRealTimers(); });

describe("NEEDS YOU issue rows", () => {
  it("the blocking row says whom it blocks, and its age survives a re-render", () => {
    const { rerender } = render(<Desk inputs={inputs} />);
    const row = within(needs()).getByText(/EP-150 blocks EP-142 \(Alice Example\)/).closest<HTMLElement>(".dn")!;
    expect(within(row).getByText("waiting 2h 10m")).toBeTruthy();
    act(() => { vi.advanceTimersByTime(60_000); });
    rerender(<Desk inputs={inputs} />);
    expect(within(row).getByText("waiting 2h 11m")).toBeTruthy();
    expect(row.textContent).toContain("ranked first: blocks EP-142, which is started");
    // mutants: age from first render; age restarting on re-render
  });
  it("after X lands the row stays while open and is gone on the next visit", () => {
    const first = render(<Desk inputs={inputs} />);
    act(() => { head(WORK_IDS.ep150, { state: "landed" }); });
    expect(within(needs()).queryByText(/EP-150 blocks/)).not.toBeNull();
    first.unmount();
    render(<Desk inputs={inputs} />);
    expect(within(needs()).queryByText(/EP-150 blocks/)).toBeNull();
    // mutants: live removal under the reader; a stale row after the visit
  });
  it("a triage row answers Accept in place and keeps its place", async () => {
    head(WORK_IDS.ep167, { state: "triage" });
    render(<Desk inputs={inputs} />);
    const row = within(needs()).getByText("EP-167").closest<HTMLElement>(".dn")!;
    fireEvent.click(within(row).getByRole("button", { name: /Accept/ }));
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(mutate).toHaveBeenCalledWith(expect.objectContaining({ kind: "setState", issueId: WORK_IDS.ep167 }));
    expect(within(row).getByRole("status").textContent).toMatch(/^Accepted · /);
    expect(row.isConnected).toBe(true);
    // mutants: navigating to the issue; the row removed on answer
  });
  it("urgent is colour and a shape on the urgent row only", () => {
    head(WORK_IDS.ep167, { priority: "urgent" });
    head(WORK_IDS.ep166, { priority: "high" });
    render(<Desk inputs={inputs} />);
    const urgent = within(needs()).getByText("urgent, assigned to you");
    expect(urgent.className).toContain("dn-urgent");
    expect(within(needs()).queryByText("EP-166")).toBeNull();
    expect(needs().querySelectorAll(".dn-urgent").length).toBe(1);
    // mutant: priority as colour on every row
  });
  it("no prediction in NEEDS YOU", () => {
    head(WORK_IDS.ep167, { state: "triage", priority: "urgent" });
    render(<Desk inputs={inputs} />);
    expectNoPrediction(needs());
  });
});
```

`MarkGlyph`'s accessible name is *Needs you* (`task-29.md:152-154`); the urgent test checks the
class, and task 29's own tests hold the shape. `head(ep167, {state: "triage", priority: "urgent"})`
makes one row, the triage row (part 2's note on `urgentMine`).

#### Steps

- [ ] Write `needs.test.ts`; it fails to import `./needs`. Write `needs.ts` until the six pass.
      Mutants, one at a time, then revert: drop `mine` from `triageMine` (test 1); test `priority
      !== "none"` in `urgentMine` (test 2); return `snap.takenAt` from `sinceOf` (test 3); skip the
      state check on Y (test 4); drop `resolvedAt` (test 5); put `waitingSinceMs` before `tier` in
      task 62's `rankItems` (test 6).
- [ ] Make task 62's additive deltas (`keyScopes`, `comments`, the wrapper's scope, the feed
      input). Task 62's twelve tests still pass unchanged.
- [ ] Write `IssueNeedRow.tsx`, the `.dn-*` CSS and task 63's block in `deskRows.tsx`.
- [ ] Write `IssueNeedRow.test.tsx`. Mutants: `useState(Date.now())` for the age (test 1);
      `setModel(fresh)` in task 62's `check` (test 2); remove the row on `done` (test 3); `dn-urgent`
      on every row (test 4).
- [ ] Tasks 42, 53, 56 and 62's suites still pass. Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/desk/needs.test.ts src/components/desk'
# expected: 4 files, 23 tests, all passing (6 needs, 5 rows, and task 62's 12)
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 42, 53, 56 and 62 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "Date\.now|useCockpit|from \"react\"" desktop/src/work/desk/needs.ts'
# expected: no output (decision 1: pure over the snapshot)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "sessions|attachRegistry" desktop/src/work/desk/needs.ts'
# expected: no output (decision 4: the Desk never reads another person's sessions)
```

#### What this task does NOT claim

- **The reviewer row** (*"in review where you are the reviewer"*): `IssueHead` has no reviewer
  field (gap 1).
- **A `decide` escalation from an Orchestrator**: W3 and TM-54.
- **The cost of waiting in money**: no gateway in W1; the wait is shown as time.
- **Inferring a question from a comment's text**: any unresolved mention is a row.
- **That a blocked issue has a live agent**: `started` is the Hub's convergence word; the row says
  *started*, not *agent running* (OD-15: no teammate's sessions are read).
- **Snooze hiding the row until the morning**: the head carries no `snoozedUntil` (gap 6).
- **Visual acceptance**: the owner's eyes (CLAUDE.md, working agreements).
- **Parity**: closes L-15 and L-24's Desk half (the relation itself is tasks 11 and 48); serves
  L-46, L-112 and L-114, as the scope says (`tasks-61-80.md:127-129`).

#### Gaps found while writing this part

1. SW.1's `IssueHead` has no reviewer (`task-37.md:121-128`); 05 §4.2's reviewer row
   (`05-design.md:936`) needs one, from task 03's scope.
2. Task 55 has no part on the tree: `replicaEvents()` (the feed events the replica has applied,
   in `seq` order) and `mutate`'s module path `work/replica/replica` are this part's assumption,
   shared with task 65 (`task-65.md:774`). `WorkEvent`'s production home is task 62's gap 5.
3. `issue.updated`'s `fields` names (`task-13.md:150`) are matched as `assignee` and `priority`;
   task 11's PATCH wire names should be printed beside `IssueUpdatedData`.
4. `teamPolicyOf(issueId)`: task 42's `TriageView` takes `policy` as a prop (`task-42.md:204`) and
   no part names the read behind it; the Desk needs the same read.
5. A key pressed on a triage row (`1`, `4`) sends the op but leaves the row's buttons, because the
   row's answer state is the row's own; lifting it into a per-visit book, as task 65's
   `AnswerBook` does (`task-65.md:818-823`), would show the key's answer too. Task 65 owns the book.
6. Task 42's `snooze` writes `snoozedUntil` (`task-42.md:221-222`), but `IssueHead` has no such
   member, so a snoozed triage issue still makes a row on the next visit.
7. The skeleton's file map (`:311`) has no `work/desk/needs.ts` or `rows/IssueNeedRow.tsx`; the
   scope names them (`tasks-61-80.md:103-105`).
