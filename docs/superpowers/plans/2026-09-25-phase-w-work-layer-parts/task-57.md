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
- `desktop/src/work/replica/visibility.test.tsx` — the pure rules and the line (new)
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

**How the paths run inside `createReplica`**, in order:

1. **Start.** `scope = await readableProjects()` before task 55's first load, which then calls
   `listAll` only for ids in `scope`. Every record entering by any path goes through `admit`.
2. **Each applied event** is first run through `classify(e, callerId)`:
   `drop_team` → one `transact` deletes every record of every `scope` project of `e.teamId`,
   removes them from `scope` and sets `accessLost` for each; `relist` → `readableProjects()`, then
   each new id gets `listAll` and one `transact` puts its records, and each id no longer listed is
   dropped as `drop_team` drops; `erase` → one `transact` rewrites every held copy through
   `eraseHead`, `eraseRevision` and `eraseComment` **before** task 55's usual subject re-fetch.
3. **A subject fetch** that omits an asked id drops that record in the same `transact` as the
   answer, then re-lists once for the batch (decision 3's fallback).
4. **A failed `changes` call** goes through task 37's `workProblem`; `healthAfterFailure` marks
   the feed down on `unavailable` only. The first time it goes down, `status = "resyncing"` and
   `since = clock()`. Refusals keep today's status and surface as task 55 already does.
5. **Each page** is run through `planForPage` before its events apply. `"full"`: `status =
   "resyncing"`, `since = clock()`; take the cursor with `after=latest`; re-list; `listAll` every
   `scope` project into **new** maps; swap them in with one `transact`; then read the feed after
   that cursor; `status = "ready"`. `"resume"`: apply the page, `status = "ready"`, `since =
   null`, re-list, then call task 56's `resume()` so the painted writes go out in op order.

#### Interfaces — complete: `ResyncLine.tsx` and the two surface edits

```tsx
// desktop/src/components/work/ResyncLine.tsx — one fact line in a view header; never per row.
import { resyncLine } from "../../work/replica/visibility";

export interface ResyncLineProps { status: string; since: number | null; timeZone: string }

/** No role: task 66's one live region speaks for the window. No motion: the line appearing is
 *  the whole message. The time is mono and tabular so it does not jitter when it changes. */
export function ResyncLine({ status, since, timeZone }: ResyncLineProps): JSX.Element | null {
  const text = resyncLine(status, since, timeZone);
  if (!text) return null;
  const cut = text.lastIndexOf(" ") + 1;
  return (
    <p className="wk-resync">
      {text.slice(0, cut)}<span className="wk-resync-at">{text.slice(cut)}</span>
    </p>
  );
}
```

```css
/* desktop/src/components/work/work.css ⚑ — appended (the `.wk-` prefix is task 40's) */
.wk-resync { margin: 0; color: var(--ink-3); font-family: var(--sans);
  font-size: var(--t-fact); line-height: var(--t-fact-lh); }
.wk-resync-at { font-family: var(--mono); font-variant-numeric: tabular-nums; }
```

- **`WorkLens.tsx`** (task 40) renders `<ResyncLine status={r.status} since={r.resyncSince()}
  timeZone={tz} />` as the last child of its `ViewHeader` (task 31). One per view, never per row.
- **`IssueView.tsx`** (task 46): when `r.accessLost(head.projectId)` is set, the body is replaced
  by one fact line, `You no longer have access to {projectKey}`, in `--ink-2`, with the issue's
  alias kept in the header so the reader knows what they were looking at. No retry button: there
  is nothing to retry until someone adds them back, and then decision 3 loads it.

One more file joins the list: `desktop/src/components/work/work.css` ⚑ (modified).

#### Tests to write first, part 1 — `desktop/src/work/replica/visibility.test.tsx` (pure rules, one render)

```ts
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { DYLAN, ALICE, WORK_IDS, makeComment, makeIssueRevision } from "../../test/workFixtures";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { ResyncLine } from "../../components/work/ResyncLine";
import { classify, eraseComment, eraseRevision, healthAfterFailure, planForPage, resyncLine } from "./visibility";

const ev = (kind: string, subject: string, seq = 1044) =>
  ({ seq, at: "2026-09-25T01:02:00Z", kind, teamId: WORK_IDS.team, projectId: null, subject });

describe("membership events act only when they name the caller", () => {
  it("removal naming the caller drops the team; naming Alice does nothing", () => {
    expect(classify(ev("team.member_removed", DYLAN.id), DYLAN.id))
      .toEqual({ kind: "drop_team", teamId: WORK_IDS.team });
    expect(classify(ev("team.member_removed", ALICE.id), DYLAN.id)).toEqual({ kind: "none" });
  }); // kills: dropping a team's records whenever anyone leaves it
  it("being set as a member re-lists", () => {
    expect(classify(ev("team.member_set", DYLAN.id), DYLAN.id)).toEqual({ kind: "relist", teamId: WORK_IDS.team });
  }); // kills: ignoring an add and waiting for feed events that were filtered out
  it("both erase kinds are recognised", () => {
    expect(classify(ev("issue.erased", WORK_IDS.ep142), DYLAN.id)).toMatchObject({ what: "issue" });
    expect(classify(ev("comment.erased", WORK_IDS.comment), DYLAN.id)).toMatchObject({ what: "comment" });
  }); // kills: an erase applied as an ordinary revise (text re-fetched into memory)
});

describe("a gap is never read from seq", () => {
  const h = { lastSeq: 1043, down: false };
  it("a next below the last applied seq is a restore", () => {
    expect(planForPage(h, "12")).toBe("full");
  }); // kills: ignoring a restored Hub (its new events would all be dropped as repeats)
  it("a hole in the seqs is normal", () => {
    expect(planForPage(h, "1100")).toBe("none");
  }); // kills: a contiguity check that resyncs on every filtered-out event
  it("an outage then an answer resumes; a refusal is not an outage", () => {
    const down = healthAfterFailure(h, "unavailable");
    expect(planForPage(down, "1043")).toBe("resume");
    expect(healthAfterFailure(h, "refused").down).toBe(false);
  }); // kills: a full reload per network blip; kills: hiding a refusal behind a resume
});

describe("erase keeps identity and loses words", () => {
  it("a JSON dump of an erased revision and comment holds key and digest, not text", () => {
    const r = makeIssueRevision({ issueId: WORK_IDS.ep142, key: "issue:is-3f2a91c07b1e4d2a@3",
      digest: "d".repeat(64), content: { title: { text: "Resume an upload after a drop", author: "H" },
        description: { text: "When the network drops at 80 MB", author: "H" }, serves: [], implements: [] } });
    const c = makeComment({ body: { text: "The 80 MB case is the one", author: "H" } });
    const dump = JSON.stringify([eraseRevision(r, "t"), eraseComment(c, "t")]);
    expect(dump).toContain("issue:is-3f2a91c07b1e4d2a@3");
    expect(dump).toContain("d".repeat(64));
    for (const words of ["Resume an upload", "80 MB"]) expect(dump).not.toContain(words);
  }); // kills: an erase that sets erasedAt and keeps the text
});

describe("the resync line", () => {
  it("says what and since when, and nothing while ready", () => {
    const at = Date.UTC(2026, 8, 25, 1, 2);
    expect(resyncLine("resyncing", at, "UTC")).toBe("Reloading · since 01:02");
    expect(resyncLine("ready", at, "UTC")).toBe("");
    const { container } = render(<ResyncLine status="resyncing" since={at} timeZone="UTC" />);
    expect(container.querySelectorAll(".wk-resync")).toHaveLength(1);
    expectNoPrediction(container);
  }); // kills: a progress count on the resync line (OD-27)
});
```
