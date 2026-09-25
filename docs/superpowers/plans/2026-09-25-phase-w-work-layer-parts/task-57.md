### Task 57: the replica, part 3: only what the caller may read; resync on a gap; erase

Stream **WD1**, the visibility and recovery half of seam **SW.16** (the replica), about 2.5 hours
(an estimate, not a measurement). Depends on task 55 (`createReplica`, `ReplicaDeps`, `status`,
`cursor`, `get`, `byProject`). Task 55's part file was not on the tree when this was written, so
every task 55 name below is its **scope's** words (`tasks-41-60.md:457-490`), **UNVERIFIED** until
its part lands; task 55 wins any disagreement. It also reads task 56's `resume()` (the write queue,
`task-56.md:354-355`) and task 41's hold model (`reconcile`, `task-41.md:141`).

After this task the replica holds **only** Work records of projects the caller can read, drops them
the moment that stops being true, loads a project in full the moment it starts being true, recovers
from an outage or a restored Hub without a blank screen, and keeps no text an erase removed.

**What the reader sees** (05 §5.3, §3.6):

- Removed from the *Engagement Portal* team with EP-142 open: the list empties of EP's issues in
  the same frame, and the open issue view says one fact line in place of its body,
  *You no longer have access to EP*. No dialog, no error colour: it is a fact, not a failure.
- Added to a team: its project's issues appear in the list through the hold model (task 41), so
  nothing jumps under the reader's cursor; the held line says how many arrived.
- The Hub restored from a backup, or the relay back after an outage: the rows stay as they are, and
  the view header's fact line reads *Reloading · since 11:02* in `--ink-3` (the time in mono,
  `tabular-nums`) until the load settles. No spinner, no per-row state, no count of what is left.
- An erased issue: its row and every chip or link that names it say *erased* with its number, key
  and digest intact (05 §3.6); its title is gone from search (task 58) and from memory.

**Decisions made here, so nobody re-derives them:**

1. **The readable list is the Hub's answer, never the desktop's guess.** A project's access is
   exactly its team's membership (`hub-contract.md:333`), and `GET /v1/projects` answers *own
   teams' projects only* (`:347`). The replica keeps a `scope` map `projectId → ProjectScope` from
   that list and calls task 55's `listAll(projectId)` **only** for ids in it. `admit(record)`
   refuses any record whose `projectId` is outside `scope`: a defence for the case where the relay
   or a Work kind is not filtered, since 05 §5.3 marks the Hub-side filter for Work kinds
   **UNVERIFIED** (task 13 owns it).
2. **A subject the Hub will not give back is gone.** A non-member reading a project gets 404
   (`:349`); a viewer's write gets 403. Task 55's `fetchSubjects(ids)` answers the records it could
   read; this task requires that it **omits** an id answered 403 or 404 rather than throwing, and
   throws only for a transport failure. An id asked and not answered is dropped in the same
   `transact`, and its project is re-listed once (decision 3's fallback).
3. **Membership, both ways.** `team.member_removed` and `team.member_set` carry the team as
   `teamId` and the user as `subject` (`:1140-1141`). Removed, naming the caller: every record of
   every project of that team is dropped in **one** `transact`, and `accessLost(projectId)` answers
   `{projectKey, at}` for each. Set, naming the caller: re-list, and every project new to `scope`
   gets a **full `listAll`**, because its earlier events were filtered out of this caller's feed
   (`:1103-1104`) and will never arrive. A `member_set` that only changes a role finds nothing new
   and loads nothing. **Fallback**: the contract does not say whether the feed filter reads
   membership at write time or at read time; if read time, the caller never sees its own removal.
   So any dropped subject (decision 2) and every resync also re-list, and a project missing from
   the new list is dropped exactly as a removal is (a gap, named at the end).
4. **A gap is never read from `seq`.** The org's sequence is gapless (`:1099`) but each caller's
   feed is filtered (`:1103`), so holes are normal and the replica never checks contiguity. It
   resyncs on three signals only: (a) **restored** — a page whose `next` is below the last applied
   `seq` (a Hub restored from backup): full resync, cursor taken again with `after=latest` (`:1101`)
   *before* the load, as task 55's first load does; (b) **recovered** — a `changes` call that failed
   as `unavailable` (task 37's `workProblem`, which counts `upstream_unavailable` and
   `network_unavailable`, `task-37.md:472-475`) followed by one that succeeds: resume from the last
   applied cursor, **no** full load, then call task 56's `resume()`; (c) **membership** (decision 3).
5. **A resync never blanks the screen.** While it runs `status` is `"resyncing"` and the records
   already held stay readable. The new load is built **beside** the old records and swapped in with
   **one** `transact`, so `useWork` selectors see one change and task 41's `reconcile(prev, next,
   focus)` holds every move under the reader. `resyncSince()` answers the local time the resync
   began; the view header renders it through `resyncLine` as one fact line. There is no progress:
   the line says what is happening and since when, never how much is left (`OD-27`).
6. **Erase keeps the identity and loses the words.** `issue.erased` (`task-13.md:129`) takes the
   issue's comments with it, and `comment.erased` is a comment's own (`task-14.md:32-36`). On
   either, before task 55's subject re-fetch, `eraseInMemory` rewrites **every** copy the replica
   holds: each revision in memory (the head and any task 47 loaded) keeps `key`, `digest`,
   `revision`, `issueId` and `projectId`, sets `erasedAt`, and has its title and description text
   emptied; the head's `stateReason` is emptied (task 14 blanks it too, `task-14.md:37-38`); each
   erased comment keeps `id`, `issueId` and `createdAt`, sets `erasedAt`, and has its body emptied.
   `serves` and `implements` hold keys, not words, and stay. The record is kept, so chips and links
   still resolve, as *erased*.

**Files** (the skeleton's File map lists `src/work/replica/{…,visibility,…}.ts` for 55-58, `:304`;
it omits the two surface edits and `ResyncLine.tsx`, gaps named at the end):
- `desktop/src/work/replica/visibility.ts` — `ProjectScope`, `classify`, `resyncFor`,
  `eraseInMemory`, `resyncLine` (new)
- `desktop/src/work/replica/visibility.test.ts` — the pure rules (new)
- `desktop/src/work/replica/replica.ts` (task 55) — `scope`, `admit`, `accessLost`, `resyncSince`,
  the membership and resync paths (modified)
- `desktop/src/work/replica/access.test.ts` — the replica against a fake Hub (new)
- `desktop/src/components/work/ResyncLine.tsx` — the one fact line (new)
- `desktop/src/components/work/WorkLens.tsx` ⚑ (task 40) — mounts `ResyncLine` in its header (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) — the *no longer have access* fact (modified)

#### Interfaces — complete: `desktop/src/work/replica/visibility.ts`

```ts
// desktop/src/work/replica/visibility.ts — pure rules; replica.ts owns the state they act on.
import type { IssueComment, IssueHead, IssueRevision } from "../../api/work";

/** One readable project, from GET /v1/hub/projects (hub-contract.md:347, relayed by task 16). */
export interface ProjectScope { projectId: string; teamId: string; key: string }

/** The fields of a feed event these rules read (hubapi.Event, hub-contract.md:1096-1097). */
export interface FeedEventLike {
  seq: number; at: string; kind: string; teamId: string | null; projectId: string | null;
  subject: string;
}

export type VisibilityAction =
  | { kind: "none" }
  | { kind: "drop_team"; teamId: string }                 // team.member_removed naming the caller
  | { kind: "relist"; teamId: string }                    // team.member_set naming the caller
  | { kind: "erase"; what: "issue" | "comment"; subject: string };

export const ERASE_KINDS = { "issue.erased": "issue", "comment.erased": "comment" } as const;

export function classify(e: FeedEventLike, callerId: string): VisibilityAction {
  if (e.kind === "team.member_removed" && e.subject === callerId && e.teamId)
    return { kind: "drop_team", teamId: e.teamId };
  if (e.kind === "team.member_set" && e.subject === callerId && e.teamId)
    return { kind: "relist", teamId: e.teamId };
  const what = (ERASE_KINDS as Record<string, "issue" | "comment" | undefined>)[e.kind];
  return what ? { kind: "erase", what, subject: e.subject } : { kind: "none" };
}

/** The feed's health as the replica sees it. `lastSeq` is the highest seq applied. */
export interface FeedHealth { readonly lastSeq: number; readonly down: boolean }
/** "full": drop the cursor, take after=latest, load every readable project, swap in one transact.
 *  "resume": the feed answered again after an outage; nothing to reload, the cursor stands. */
export type ResyncPlan = "none" | "full" | "resume";

/** A page arrived. A `next` below lastSeq means the Hub went back in time: a restore (decision 4a).
 *  Holes between seqs mean nothing (the feed is filtered per caller, hub-contract.md:1103). */
export function planForPage(h: FeedHealth, next: string): ResyncPlan {
  const n = Number(next);
  if (Number.isSafeInteger(n) && n < h.lastSeq) return "full";
  return h.down ? "resume" : "none";
}

/** A changes() call failed. Only `unavailable` (task 37's workProblem) marks the feed down: a
 *  refusal or signed_out is not an outage, and a resume after it would hide the real cause. */
export function healthAfterFailure(h: FeedHealth, problem: "unavailable" | "refused" | "signed_out" | "other"): FeedHealth {
  return problem === "unavailable" ? { lastSeq: h.lastSeq, down: true } : h;
}

/** The words an erase leaves behind (decision 6). Identity stays; text goes. */
export function eraseRevision(r: IssueRevision, at: string): IssueRevision {
  return { ...r, erasedAt: r.erasedAt ?? at, content: { ...r.content,
    title: { text: "", author: r.content.title.author },
    description: { text: "", author: r.content.description.author } } };
}
export function eraseComment(c: IssueComment, at: string): IssueComment {
  return { ...c, erasedAt: c.erasedAt ?? at, body: { text: "", author: c.body.author }, mentions: [] };
}
export function eraseHead(h: IssueHead): IssueHead {
  return { ...h, stateReason: "" };
}

/** "Reloading · since 11:02" while resyncing, else "". 24-hour local time; no count, no ETA. */
export function resyncLine(status: string, since: number | null, timeZone: string): string {
  if (status !== "resyncing" || since === null) return "";
  const hhmm = new Intl.DateTimeFormat("en-AU", { hour: "2-digit", minute: "2-digit",
    hourCycle: "h23", timeZone }).format(since);
  return `Reloading · since ${hhmm}`;
}
```

`IssueRevision`, `IssueComment` and `IssueHead` are task 37's wire types. Their fields here
(`content.title`, `content.description`, `erasedAt`, `body`, `mentions`, `stateReason`) are the
ones task 20's fixtures mirror (`task-20.md:180-197`); if task 37 names them differently, task 37
wins and these three functions follow. The replica passes the event's `at` as the erase time.

#### Interfaces — complete: the deltas on `replica.ts` (task 55)

```ts
// desktop/src/work/replica/replica.ts — task 57 adds these; task 55's Replica keeps its members.
import type { ProjectScope } from "./visibility";

/** Task 55's ReplicaDeps gains the two things visibility needs. */
export interface ReplicaVisibilityDeps {
  /** The signed-in user's Hub id (u-…), from phase H's identity (GET /v1/me, hub-contract.md:343). */
  callerId: string;
  /** GET /v1/hub/projects through task 16's relay: own teams' projects only (hub-contract.md:347). */
  readableProjects(): Promise<ProjectScope[]>;
  /** The IANA zone the fact line prints in; the store's, as task 41's causeLine takes it. */
  timeZone: string;
  clock(): number;
}

export interface ReplicaVisibility {
  /** The projects the replica may hold, as last listed. Nothing outside it is ever loaded. */
  scope(): readonly ProjectScope[];
  /** Set when the caller lost a project this session; the issue view shows it as a fact. */
  accessLost(projectId: string): { projectKey: string; at: number } | null;
  /** When the current resync began, or null when status is not "resyncing". */
  resyncSince(): number | null;
}
// `interface Replica extends ReplicaVisibility` — task 55's Replica gains these members.
```
