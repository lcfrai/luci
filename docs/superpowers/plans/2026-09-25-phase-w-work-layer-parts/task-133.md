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

#### Interfaces — complete: `desktop/src/work/since/handover.ts`

```ts
// desktop/src/work/since/handover.ts — one product-language sentence per fact, from fixed
// templates (decision 3). This module imports no gateway, inference or draft client: a test
// reads its source. A drafted handover is task 139's, never this.
import { aging, type Classes, type SinceFact } from "./classify";

/** A claim and what makes it true. Every `ref` occurs in `text`; the renderer links each one. */
export interface SinceLink { ref: string; eventSeq: number; issueId: string }
export interface Sentence { cls: keyof Classes; text: string; links: SinceLink[] }

/** "14:02", the person's clock, mono in the renderer (decision 4). */
export type ClockFmt = (iso: string) => string;

function template(f: SinceFact, now: number, clock: ClockFmt): string {
  switch (f.fact) {
    case "landed":            return `${f.alias} landed ${clock(f.at)}`;
    case "done_proven":       return `${f.alias} is done, proven ${clock(f.at)}`;
    case "shipped":           return `${f.alias} shipped to ${f.environment} ${clock(f.at)}`;
    case "criterion_settled": return `${f.alias}'s criterion ${f.criterionKey} settled ${clock(f.at)}`;
    case "question":          return `${f.alias} waits on your answer to ${f.from}, waiting ${aging(f.at, now)}`;
    case "review":            return `${f.alias} waits on your review, waiting ${aging(f.at, now)}`;
    case "blocking_agent":    return `${f.alias} blocks ${f.blocked}'s agent, waiting ${aging(f.at, now)}`;
    case "lane_failed":       return `${f.alias}'s worktree ${f.laneId} failed ${clock(f.at)}`;
    case "done_unproven":     return `${f.alias} was moved to done, not proven ${clock(f.at)}`;
    case "decided_without_you": return `${f.alias}: ${f.what}, decided without you ${clock(f.at)}`;
    case "criterion_drifted": return `${f.alias} serves ${f.criterionKey}, which the brief no longer has`;
  }
}

/** Every link names the issue at the event; a criterion fact links its key too. */
function linksOf(f: SinceFact): SinceLink[] {
  const l: SinceLink[] = [{ ref: f.alias, eventSeq: f.seq, issueId: f.issueId }];
  if (f.fact === "criterion_settled" || f.fact === "criterion_drifted") {
    l.push({ ref: f.criterionKey, eventSeq: f.seq, issueId: f.issueId });
  }
  return l;
}

/** The four classes, in AX §6.2's order, never merged: one sentence per fact. */
export function handover(c: Classes, now: number, clock: ClockFmt): Sentence[] {
  const order: (keyof Classes)[] = ["waiting", "failed", "assumed", "landed"];
  return order.flatMap((cls) => c[cls].map((f) => ({ cls, text: template(f, now, clock), links: linksOf(f) })));
}
```

AX §6.2 (`docs/AGENTIC-UX.md:159`) names the classes LANDED, WAITING, FAILED, ASSUMED and ranks
none. The rendering order here is this part's choice, from the same section's weights: WAITING
first (*"aging matters"*), then FAILED, then ASSUMED (*"highest risk"*), then LANDED (*"low
attention"*). It is an order of four headed lists, never an interleaving: the lists stay four
(decision 2). The templates follow §6.3's product language, and §6.4's *"every claim … is a link
to its span"* is the `links` field.

#### Interfaces — complete: `desktop/src/components/desk/SinceIssues.tsx`

```tsx
// desktop/src/components/desk/SinceIssues.tsx — the band's issue rows (DeskBand "since"). One
// headed list per class; each sentence's refs are links to the event they rest on.
import type { JSX, ReactNode } from "react";
import type { ScopeRef } from "../../api/work";
import type { Classes } from "../../work/since/classify";
import type { Sentence } from "../../work/since/handover";

export interface SincePayload {
  sentences: Sentence[];            // handover(classes, now, clock), taken once at snapshot
  counts: Record<keyof Classes, number>;
  projectScopes: ScopeRef[];        // every project cursor the band read
  headSeq: number;                  // the newest seq the snapshot saw: mark-all-seen's target
}

const HEAD: Record<keyof Classes, string> = {
  waiting: "Waiting on you", failed: "Failed", assumed: "Decided without asking", landed: "Landed" };

/** Splits text at each link's ref, in order, so every claim is a link (decision 3). */
export function linked(s: Sentence, open: (issueId: string, seq: number) => void): ReactNode[] {
  const out: ReactNode[] = [];
  let rest = s.text;
  for (const l of s.links) {
    const i = rest.indexOf(l.ref);
    if (i < 0) continue;                                   // a ref not in the text: the test catches it
    out.push(rest.slice(0, i),
      <a key={`${l.ref}-${l.eventSeq}`} href={`#${l.issueId}@${l.eventSeq}`} className="since-ref"
         onClick={(e) => { e.preventDefault(); open(l.issueId, l.eventSeq); }}>{l.ref}</a>);
    rest = rest.slice(i + l.ref.length);
  }
  out.push(rest);
  return out;
}

export function SinceIssues(p: { item: { payload: SincePayload };
  openAt(issueId: string, seq: number): void; markAllSeen(scopes: ScopeRef[], seq: number): void }): JSX.Element {
  const { sentences, counts, projectScopes, headSeq } = p.item.payload;
  const classes = (["waiting", "failed", "assumed", "landed"] as const).filter((c) => counts[c] > 0);
  if (classes.length === 0) return <p className="since-zero">Nothing on your issues since you last looked.</p>;
  return (
    <div className="since-issues">
      {classes.map((c) => (
        <section key={c} className="since-class" data-class={c}>
          <h4>{HEAD[c]} <span className="since-n">{counts[c]}</span></h4>
          <ul>{sentences.filter((s) => s.cls === c).map((s, i) => (
            <li key={i} className="since-line">{linked(s, p.openAt)}</li>
          ))}</ul>
        </section>
      ))}
      <button type="button" className="since-seen" onClick={() => p.markAllSeen(projectScopes, headSeq)}>
        Mark all seen
      </button>
    </div>
  );
}
```

`.since-n` is `tabular-nums` (a section count, the one place counts are allowed, task 62's
scope). The clock times inside the text are wrapped by the renderer in `.since-at` (mono) where
`handover` puts them; a plain-text version for task 94's `DeskDigest` is `sentences.map((s) =>
s.text)`, the same words (L-119 without audio).

#### Interfaces — complete: the registration on ⚑ `deskRows.tsx`, and the watermark

```tsx
// deskRows.tsx ⚑ (task 62) — task 133's registration. "since" is a DeskBand: not ranked.
import { registerDeskRow, registerDeskSource } from "./deskModel";          // SW.17 (gap 1)
import { SinceIssues, type SincePayload } from "./SinceIssues";
import { classify } from "../../work/since/classify";
import { handover } from "../../work/since/handover";

registerDeskRow<SincePayload>("since.issues", "since",
  (item) => <SinceIssues item={item} openAt={openAtWatermark} markAllSeen={markAllSeen} />,
  () => ({ tier: 2, waitingSinceMs: 0 }));                                 // a band: rank unused
registerDeskSource("since.issues", (snap) => {
  const s = snap.since;          // gathered on Desk entry (below); the frame never re-ranks while open
  const c = classify(s.events, s.local, s.involvement);
  return [{ kind: "since.issues", id: "since.issues", arrivedAt: s.takenAt,
    payload: { sentences: handover(c, s.now, s.clock), projectScopes: s.scopes, headSeq: s.headSeq,
      counts: { landed: c.landed.length, waiting: c.waiting.length, failed: c.failed.length, assumed: c.assumed.length } },
    rankReason: "what changed on your issues", ifNothing: "" }];
});
```

**On Desk entry** (task 62's snapshot hook, `DeskSnapshot.since`, a member this task adds; gap 1)
the band, for each project scope: `readCursor({kind: "project", id})` then `scopedChanges(scope,
String(cursor.seq))`, following `more` to at most 200 events per scope (task 123's bound; a
`truncated` page adds the line *"Older changes were not read."*). `takenAt` and `now` are one
`Date.now()` read, so aging does not tick while the Desk is open.

`openAtWatermark(issueId, seq)`: reads the issue's own cursor, then `advanceReadCursor({scope:
{kind: "issue", id: issueId}, seq})` for **that issue only**, and opens the issue with
`openIssue(issueId)` (task 37) plus a `watermark` of the old cursor's `seq`. ⚑ `IssueView.tsx`
(task 46) scrolls its activity to the first event with `seq > watermark` and gives older ones
`data-seen` (dimmed by `opacity`, no transition). `markAllSeen(scopes, seq)` is **one**
`markSeen({scopes, seq})` call, then the band shows *"Marked seen."* and keeps its rows until the
next visit (the frame never re-ranks while open). TM-54's `team.since` rows run their own
`markTeamSeen` from the same control (`team/…/task-54.md:57-60`).

#### Tests to write first — `desktop/src/work/since/since.test.ts`

```ts
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import type { FeedEvent } from "../../api/work";
import { aging, classify, type Involvement } from "./classify";
import { handover } from "./handover";

const ME = "u-dylan";
const mine = new Set(["is-142", "is-150"]);            // assigned to me; is-900 is a teammate's
const inv: Involvement = {
  meId: ME, involves: (id) => mine.has(id), alias: (id) => ({ "is-142": "EP-142", "is-150": "EP-150", "is-900": "EP-900" }[id] ?? null),
  reviewerOf: (id) => (id === "is-150" ? ME : null), commentKind: () => "question", commentAuthor: () => "Priya",
};
const ev = (seq: number, subject: string, to: string, proof = "", at = "2026-09-26T04:02:00Z"): FeedEvent =>
  ({ seq, kind: "issue.state", subject, at, actor: "u-x", data: { from: "started", to, cause: "", proof, landCommit: "" } }) as never;
const clock = (iso: string) => iso.slice(11, 16);

describe("classify", () => {
  // mutant killed: "a merged stream"
  it("a landed and a failed event land in two classes, never one list", () => {
    const c = classify([ev(1, "is-142", "landed")],
      [{ fact: "lane_failed", seq: 2, at: "2026-09-26T05:00:00Z", issueId: "is-150", alias: "EP-150", laneId: "wt-3" }], inv);
    expect(c.landed.map((f) => f.alias)).toEqual(["EP-142"]);
    expect(c.failed.map((f) => f.alias)).toEqual(["EP-150"]);
    expect(Object.keys(c).sort()).toEqual(["assumed", "failed", "landed", "waiting"]);
  });
  // mutant killed: "optimistic class"
  it("a self-attested done is assumed, not landed; a proven done is landed", () => {
    const c = classify([ev(1, "is-142", "done", "proven_self_attested"), ev(2, "is-150", "done", "proven")], [], inv);
    expect(c.assumed.map((f) => f.alias)).toEqual(["EP-142"]);
    expect(c.landed.map((f) => f.alias)).toEqual(["EP-150"]);
  });
  // mutant killed: "team firehose"
  it("a teammate's issue that does not involve me is absent", () => {
    const c = classify([ev(1, "is-900", "landed")], [], inv);
    expect([...c.landed, ...c.waiting, ...c.failed, ...c.assumed]).toEqual([]);
  });
});

describe("handover", () => {
  // mutant killed: AX anti-pattern 9 ("5h ago")
  it("aging reads 'waiting 5h 51m' and nothing reads 'ago'", () => {
    const now = Date.parse("2026-09-26T09:53:00Z");
    const c = classify([ev(1, "is-150", "in_review", "", "2026-09-26T04:02:00Z")], [], inv);
    const text = handover(c, now, clock).map((s) => s.text).join("\n");
    expect(text).toContain("waiting 5h 51m");
    expect(text).not.toMatch(/\bago\b/);
    expect(aging("2026-09-26T09:00:00Z", now)).toBe("53m");
  });
  // mutant killed: "a claim with no span" (checked on the text too, in SinceIssues.test.tsx)
  it("every sentence has a link, and every link's ref is in its text", () => {
    const c = classify([ev(1, "is-142", "landed"), ev(2, "is-142", "done", "waiting")], [], inv);
    for (const s of handover(c, Date.now(), clock)) {
      expect(s.links.length).toBeGreaterThan(0);
      for (const l of s.links) expect(s.text).toContain(l.ref);
    }
  });
  // mutant killed: "an unlinked model summary"
  it("the handover module imports no gateway, inference or draft client", () => {
    const src = readFileSync(resolve(__dirname, "handover.ts"), "utf8");
    expect(src.match(/^import .*$/gm) ?? []).toEqual([`import { aging, type Classes, type SinceFact } from "./classify";`]);
  });
});
```

#### Tests to write first — `desktop/src/components/desk/SinceIssues.test.tsx`

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import type { Sentence } from "../../work/since/handover";
import { SinceIssues, type SincePayload } from "./SinceIssues";

const S = (cls: Sentence["cls"], text: string, ref: string, issueId: string, seq: number): Sentence =>
  ({ cls, text, links: [{ ref, eventSeq: seq, issueId }] });
const payload: SincePayload = {
  sentences: [S("waiting", "EP-150 waits on your review, waiting 5h 51m", "EP-150", "is-150", 41),
              S("landed", "EP-142 landed 14:02", "EP-142", "is-142", 40)],
  counts: { waiting: 1, failed: 0, assumed: 0, landed: 1 },
  projectScopes: [{ kind: "project", id: "pj-1" }, { kind: "project", id: "pj-2" }], headSeq: 41 };
const mount = () => {
  const openAt = vi.fn(), markAllSeen = vi.fn();
  return { openAt, markAllSeen, ...render(<SinceIssues item={{ payload }} openAt={openAt} markAllSeen={markAllSeen} />) };
};

// mutant killed: "a claim with no span", walked over the rendered text nodes
it("every rendered sentence holds at least one link", () => {
  const { container } = mount();
  const lines = [...container.querySelectorAll(".since-line")];
  expect(lines).toHaveLength(2);
  for (const li of lines) expect(li.querySelectorAll("a.since-ref").length).toBeGreaterThan(0);
  expectNoPrediction(container);
});

// mutant killed: "open marks all seen"
it("opening one issue opens it at its event and moves nothing else", () => {
  const { openAt, markAllSeen } = mount();
  fireEvent.click(screen.getByRole("link", { name: "EP-142" }));
  expect(openAt.mock.calls).toEqual([["is-142", 40]]);
  expect(markAllSeen).not.toHaveBeenCalled();
});

// mutant killed: "partial mark" (one call per scope, or a scope left out)
it("mark all seen is one call over every project scope", () => {
  const { markAllSeen } = mount();
  fireEvent.click(screen.getByRole("button", { name: "Mark all seen" }));
  expect(markAllSeen.mock.calls).toEqual([[payload.projectScopes, 41]]);
});

it("the classes are headed lists, never one merged list", () => {
  const { container } = mount();
  expect([...container.querySelectorAll(".since-class")].map((s) => s.getAttribute("data-class")))
    .toEqual(["waiting", "landed"]);
});
```

**Rows that need task 62's frame and task 125's client** (written in `deskRows.test.tsx` beside
task 62's own tests, with its fixtures; **UNVERIFIED** helper names until task 62's part lands):

| Test | Setup and expectation | Mutant killed |
|---|---|---|
| `openAtWatermark moves one cursor` | three project scopes and two issue cursors held; open EP-142 → exactly one `advanceReadCursor` with `{kind:"issue", id:"is-142"}` and the event's seq; the other cursors read unchanged | open marks all seen |
| `an arrival while open holds` | Desk mounted; the replica delivers a new `issue.state` for EP-150 → no new `.since-line`; task 62's hold line shows; `N` applies it | live band |
| `a teammate's agent alone never shows` | an `issue.state` from a teammate's agent on a teammate's issue → nothing in the band (TM-54's rule) | agent on the Desk by itself |
| `IssueView lands on the watermark` | watermark 40; events 38-43 → focus and scroll at 41; 38-40 carry `data-seen` | lands at the bottom |

#### Steps

1. Write `since.test.ts`; it fails. Write `classify.ts` and `handover.ts`; it passes.
2. Write `SinceIssues.test.tsx`; it fails. Write `SinceIssues.tsx`; it passes.
3. Register the kind in ⚑ `deskRows.tsx`; add `DeskSnapshot.since` to task 62's snapshot hook;
   write `openAtWatermark` and `markAllSeen` over `createClient().work`.
4. Land on the watermark in ⚑ `IssueView.tsx`; add the four frame rows.
5. Run the acceptance; commit.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/since src/components/desk/SinceIssues.test.tsx src/components/desk
# expect: since.test.ts 6 passed; SinceIssues.test.tsx 4 passed; deskRows' four rows and task 62's
#         and TM-54's registry tests pass (registering "since.issues" once, never twice)
cd desktop && npx tsc --noEmit
# expect: exit 0
cd desktop && grep -n -E "localStorage|sessionStorage|indexedDB" src/work/since/*.ts src/components/desk/SinceIssues.tsx
# expect: no output (decision 7)
```

#### What this task does NOT claim

- Audio playback of the handover (later), or session and branch deltas (AX §6.1's other scopes).
- A drafted, prose handover: task 139's, from these facts, never in place of them.
- That the local facts are bounded exactly by the cursor: they are bounded by the oldest unseen
  Hub event's time until `ReadCursor` carries `at` (gap 2).
- That `comment.mentioned`'s subject is the mentioned person (task 13's data carries
  `commentId` and `issueId` only, `task-13.md:195-198`; **UNVERIFIED**, gap 3).

#### Gaps found while writing this part

1. **Task 62 has no part.** `registerDeskRow`, `registerDeskSource`, `DeskSnapshot` and the
   *mark all seen* control are its scope's; two parts import them from different files (task 127:
   `./deskFrame`; task 170: `../desk/registry`); this part uses the scope's `deskModel`. Task 62
   should fix one path, and a `DeskSnapshot.since` member for this band.
2. **`ReadCursor` has no `at`** (task 123/114). Local facts need the time of the cursor's event.
3. **Who a mention names.** `CommentMentionedData` has no mentioned-user field; this part reads
   the event's `subject` as the person named (task 13 should say so or add `userId`).
4. **No part for tasks 63, 64 or 71**, whose Desk sources feed `local`.
