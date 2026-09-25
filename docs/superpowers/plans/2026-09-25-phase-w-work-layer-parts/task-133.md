### Task 133: *since you last looked* names issues: four delta classes, a handover that links every claim

W4, stream WD, AX §6.2-6.5 and 05 §2.5's example (*"EP-142 landed; EP-150 waits on your decision
since 23:40"*). Depends on task 123 (read cursors and the scoped `/v1/changes`, through task 125's
`WorkClient` members) and on Phase T's TM-54 (its rows share the band). Estimate 0.375 (3 h).
Closes **L-118** (with task 123) and **L-119** without audio.

After this task the Desk's *SINCE YOU LAST LOOKED* band says, in product words, what changed on the
issues that involve the person since they last looked: what landed, what waits on them and for how
long, what failed, and what was decided without asking them. Four lists, never merged. Every
sentence is built from fixed templates with no model call, and every claim in it is a link to the
event that makes it true. Opening an issue from the band moves only that issue's cursor and lands
its activity on the first event the person has not seen.

**Decisions made here, so nobody re-derives them:**

1. **Involvement is the filter, before any class.** An event is in scope only when its issue is
   one the person is assigned, reviews, is subscribed to (task 13), or is named in (a mention in a
   comment or a question addressed to them). Another member's issue that does not involve the
   person is absent, however busy the team is (no team firehose). A teammate's agent never appears
   by itself (TM-54's rule): its events reach the band only through an issue that involves the
   person.
2. **Four classes, never merged** (AX §6.2). `classify(events, me)` returns
   `{landed, waiting, failed, assumed}`, each a list:
   - *landed*: `issue.state` into `landed`; into `done` **with evidence** (task 10's `proof`
     `proven`); `issue.shipped` (task 71); a criterion the issue serves turning `settled`.
   - *waiting*: a question to the person; `in_review` with the person as reviewer; the person's
     issue blocking someone's running agent. Each carries its aging from the event's `at`
     (*"waiting 5h 51m"*, AX §6.7), measured on the Hub's clock against one read of `now`.
   - *failed*: a lane bound to the person's issue failing.
   - *assumed*: what was decided without asking: a DECIDED WITHOUT YOU agreement, a `done` moved by
     hand without evidence (*done, not proven*, §3.3), a criterion the issue serves drifting.
   An event that fits none is dropped, never forced into a class. A self-attested `done` is
   *assumed*, never *landed*.
3. **The handover is mechanical.** `handover(classes) → Sentence[]` fills one fixed template per
   item. The module imports no gateway client; a drafted version is task 139's. Each
   `Sentence {text, links [{ref, eventSeq}]}` carries the alias or key it names and the event it
   rests on, and the renderer turns every `ref` in `text` into a link to that event's span in the
   issue's activity (AX §6.4). A sentence with no link is a bug, and a test walks the text nodes.
4. **Time is aging, never *ago*.** Waiting items read *"waiting 5h 51m"*; landed and failed items
   read the clock time in mono (*"landed 14:02"*). Nothing reads *"ago"* (AX anti-pattern 9), so
   nothing changes meaning as the page sits.
5. **Cursors move by the person's act only.** Opening an issue from the band advances **only that
   issue's** cursor (task 125's `advanceReadCursor({scope: {kind: "issue", id}, seq})`), and the
   issue view scrolls to the first event after the old cursor with older events dimmed (AX §6.5).
   *Mark all seen* advances every project cursor in **one** `markSeen` call (task 123's one
   transaction). Rendering, scrolling and hovering move nothing.
6. **The band holds, like the Desk.** It is one of task 62's bands (`DeskBand "since"`), so it
   takes one snapshot on entry and never re-ranks while open; an event arriving while the Desk is
   open shows task 62's hold line, not a new sentence.
7. **Memory only.** Cursors and the classified events live in the band's snapshot, never in the
   replica's store or on disk (task 125 decision 3).

**Files:**
- `desktop/src/work/since/classify.ts` — `SinceFact`, `normalize`, `classify` (new)
- `desktop/src/work/since/handover.ts` — `Sentence`, `handover`, the templates (new)
- `desktop/src/work/since/since.test.ts` — the pure tests (new)
- `desktop/src/components/desk/SinceIssues.tsx` — the band's issue rows, *mark all seen* (new)
- `desktop/src/components/desk/SinceIssues.test.tsx` — the rendered tests (new)
- `desktop/src/components/desk/deskRows.tsx` ⚑ (task 62) — `registerDeskRow("since.issues",
  "since", …)` and `registerDeskSource` (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) — lands on the watermark (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 13 (the kinds and
`IssueStateData {from, to, cause, proof, landCommit}`, `task-13.md:121-165`), 23 (the proof
words, `task-23.md:137-142`), 123 and 125 (`ReadCursor`, `ScopeRef`, `MarkSeenRequest`,
`ScopedChangesPage`, `task-125.md:177-185`), and Phase T's TM-54 (`team/…/task-54.md`: its
`team.since` rows, one per project, in the same band, and the rule that a teammate's agent never
appears by itself, `:25-27`). There is **no part for task 62**: `registerDeskRow`,
`registerDeskSource`, `DeskItem`, `DeskRank` and `DeskSnapshot` are its scope's
(`tasks-61-80.md:73-84`), and two written parts import them from different files (task 127 from
`./deskFrame`, task 170 from `../desk/registry`); this part uses task 62's scope file,
`./deskModel`, and says so (gap 1).

#### Interfaces — complete: `desktop/src/work/since/classify.ts`

```ts
// desktop/src/work/since/classify.ts — the four delta classes (AX §6.2). Pure: no clock read, no
// store, no network. `now` and every lookup are passed in.
import type { FeedEvent } from "../../api/work";               // task 55's decoded hubapi.Event

/** One fact the band can state, already joined to its issue. `seq` is the event it rests on. */
export type SinceFact =
  | { fact: "landed"; seq: number; at: string; issueId: string; alias: string }
  | { fact: "done_proven"; seq: number; at: string; issueId: string; alias: string }
  | { fact: "done_unproven"; seq: number; at: string; issueId: string; alias: string; proof: string }
  | { fact: "shipped"; seq: number; at: string; issueId: string; alias: string; environment: string }
  | { fact: "criterion_settled"; seq: number; at: string; issueId: string; alias: string; criterionKey: string }
  | { fact: "criterion_drifted"; seq: number; at: string; issueId: string; alias: string; criterionKey: string }
  | { fact: "question"; seq: number; at: string; issueId: string; alias: string; from: string }
  | { fact: "review"; seq: number; at: string; issueId: string; alias: string }
  | { fact: "blocking_agent"; seq: number; at: string; issueId: string; alias: string; blocked: string }
  | { fact: "lane_failed"; seq: number; at: string; issueId: string; alias: string; laneId: string }
  | { fact: "decided_without_you"; seq: number; at: string; issueId: string; alias: string; what: string };

export interface Classes {
  landed: SinceFact[]; waiting: SinceFact[]; failed: SinceFact[]; assumed: SinceFact[];
}

/** What the replica answers about the person and an issue (decision 1). */
export interface Involvement {
  meId: string;
  involves(issueId: string): boolean;        // assignee, reviewer, subscribed, or named in
  alias(issueId: string): string | null;     // null: the caller cannot read it, so it is dropped
  reviewerOf(issueId: string): string | null;
  commentKind(commentId: string): string | null;   // task 51's comment kind, "question" among them
  commentAuthor(commentId: string): string | null; // a person's name
}

/** A Hub event to at most one fact. Kinds and data are task 13's (task-13.md:121-165). */
export function normalize(e: FeedEvent, inv: Involvement): SinceFact | null {
  const d = (e.data ?? {}) as Record<string, string>;
  const issueId = e.kind.startsWith("comment.") ? d.issueId : e.subject;
  if (!issueId || !inv.involves(issueId)) return null;          // decision 1, before any class
  const alias = inv.alias(issueId);
  if (alias === null) return null;
  const base = { seq: e.seq, at: e.at, issueId, alias };
  if (e.kind === "issue.state") {
    if (d.to === "landed") return { fact: "landed", ...base };
    if (d.to === "done") return d.proof === "proven"
      ? { fact: "done_proven", ...base }
      : { fact: "done_unproven", ...base, proof: d.proof ?? "" };   // self-attested or by hand
    if (d.to === "in_review" && inv.reviewerOf(issueId) === inv.meId) return { fact: "review", ...base };
    return null;
  }
  if (e.kind === "comment.mentioned" && e.subject === inv.meId && inv.commentKind(d.commentId) === "question") {
    return { fact: "question", ...base, from: inv.commentAuthor(d.commentId) ?? "someone" };
  }
  return null;   // every other kind: no fact, never forced into a class
}

/** The class of each fact (decision 2). A fact kind in no class does not exist: the union is total. */
const CLASS_OF: Record<SinceFact["fact"], keyof Classes> = {
  landed: "landed", done_proven: "landed", shipped: "landed", criterion_settled: "landed",
  question: "waiting", review: "waiting", blocking_agent: "waiting",
  lane_failed: "failed",
  done_unproven: "assumed", decided_without_you: "assumed", criterion_drifted: "assumed",
};

/** Hub events after each cursor, plus the facts the Hub feed does not carry (below), to four
 *  lists. Oldest first within a list; the band never re-ranks while open (decision 6). */
export function classify(events: readonly FeedEvent[], local: readonly SinceFact[], inv: Involvement): Classes {
  const out: Classes = { landed: [], waiting: [], failed: [], assumed: [] };
  const facts = [...events.map((e) => normalize(e, inv)).filter((f): f is SinceFact => f !== null),
                 ...local.filter((f) => inv.involves(f.issueId))];
  facts.sort((a, b) => (a.at < b.at ? -1 : a.at > b.at ? 1 : a.seq - b.seq));
  for (const f of facts) out[CLASS_OF[f.fact]].push(f);
  return out;
}

/** Aging from the Hub's `at` against one `now` (decision 4): "5h 51m", never "ago". */
export function aging(at: string, now: number): string {
  const m = Math.max(0, Math.floor((now - Date.parse(at)) / 60_000));
  return m < 60 ? `${m}m` : `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}
```

**The facts the Hub feed does not carry** come from the desktop's own sources, each already
built by another task, and enter `classify` as `local`: `shipped` from task 71's `issue.shipped`
Desk source; `criterion_settled` and `criterion_drifted` from task 45's `convergenceWord` over
the issue's served criteria (a change of word since the band's last snapshot); `blocking_agent`
from task 63's *blocking someone's agent* selector; `lane_failed` from task 22's lane index (the
lane names its issue, task 21); `decided_without_you` from task 64's DECIDED WITHOUT YOU items.
Their `seq` is the issue's newest Hub `seq` at the time, so the link lands on the issue's activity
at that point. They are admitted only when their `at` is after `sinceAt`, the `at` of the oldest
unseen Hub event across the scopes; with no unseen event there is no local fact, so the band never
invents a window (gap 2: `ReadCursor` has no `at`). Task 45's `convergenceWord` is verified
(`task-45.md:90`); the other four sources are their scopes' names (**UNVERIFIED**: there are no
parts for tasks 63, 64 or 71, and task 22's lane index is read by name only).
