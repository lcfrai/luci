### Task 106: the negotiation row on both issues: a `⇄` line until seen, then a glyph

Slice **W3** (agents on the work), about 2.5 hours (an estimate, not a measurement). Depends on
task 105 (Fleet seeds the editor's Orchestrator; its gap 7 names the writer of the `negotiation`
comment this part reads). Uses SW.19's `negotiation` comment kind (task 99:
`Negotiation{threadId, counterpartIssueId}`), Phase T's team read model and client (TM-52:
`ThreadView`, `ThreadRow`, `ThreadMessage`, `Resolution`, `createTeamClient().thread`; Phase T
`task-52.md:892-939`, **UNVERIFIED until merge**), SW.12's `MarkGlyph` (task 29), task 41's hold
state, task 37's `workSlice`, task 66's live region and SW.8.

05 §2.2 point 5: *"Both issues show the outcome, as one negotiation row: 'Sequenced after EP-150
(Alice's). Agreed by both Orchestrators 10:41; Dylan informed.' The row links to the thread's
typed state, never to the other side's transcript (OD-15)."* And 05 §4.1 (`05-design.md:924-926`):
the row shows *"as a `⇄` line under the issue only until you have seen it (the read cursor), then
folds to a `⇄` glyph on the row"*.

**Decisions made here, so nobody re-derives them:**

1. **The words are assembled from typed fields, never summarised.** `negotiationWords(fact)` is a
   pure function over a `NegotiationFact` built from the `negotiation` comment (which thread,
   which other issue) and TM-52's `ThreadView` (state, round, limit, the accepting message's
   resolution and time). No model text and no `TextA` body is ever shown in the row; a
   `ThreadMessage.text` is not read at all.
2. **A clause is shown only when its field exists.** TM-52's `ThreadRow.state` is `open` or
   `escalated` only, and the view carries no *informed* or *asked* list (gap 1). The fact reads
   `resolvedAt`, `informed` and `asked` as optional added fields; without them the row says what
   it knows (*"Sequenced after EP-150 (Alice's). Agreed by both Orchestrators 10:41."*) and never
   guesses who was told.
3. **`round 2 · limit 3`, never `round 2 of 3`.** 02's example (`02:322`) is an "N of M" string
   that SW.8's guard fails by design (`OD-27`); the limit is a fact about the thread, not progress.
4. **The alias is read at render time.** *EP-150* is the replica's display alias for the other
   issue's `is-…` (skeleton rule 1); the fact holds ids only.
5. **Seen is a session mark until task 123.** W4's Hub read cursors are task 123, which comes after
   this task, so here *seen* is a non-persisted `workSlice` mark (`negotiationSeen`, keyed
   `threadId:state`, so a thread that moves from `open` to resolved is unseen again). It is set
   when the issue view has shown the row with the window focused through one heartbeat beat
   (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts:64`). Task 123 replaces the mark with its
   cursor; a skeleton gap (gap 2).
6. **An arriving line never moves a row.** While task 41's list is holding (focus in the group, or
   held changes pending), a new negotiation shows only the fixed-width `⇄` glyph slot on the row,
   which moves nothing; the line under the row joins when the held changes are applied (`N`, leave
   the view, or focus outside the group), as every other arrival does (skeleton rule 4).
7. **One announcement.** A new or changed fact reaches task 66's one polite live region once, keyed
   by `threadId:state`; a re-render or a heartbeat never announces it again.
8. **The link is the thread's typed state.** *thread* opens Phase T's Orchestrator panel on that
   thread (TM task 55; its store action is read on the sync base); no `href` to a session, a
   transcript or the other person's anything is ever rendered (`OD-15`).

**Files:**
- `desktop/src/components/issue/NegotiationRow.tsx` (+ `NegotiationRow.test.tsx`) —
  `negotiationFact`, `negotiationWords`, `NegotiationRow`, `NegotiationLine`, `useNegotiations` (new)
- `desktop/src/components/glyphs/MarkGlyph.tsx` ⚑, `paths.ts` ⚑, `glyphs.css` ⚑ (tasks 28-29) —
  `MarkKind` gains `"negotiation"`, our drawn `⇄`, 14 and 16 px (modified)
- `desktop/src/components/work/IssueRow.tsx` ⚑ (tasks 40-41) — the glyph slot and the line (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) — `NegotiationRow` mounted under the
  head (modified)
- `desktop/src/store/slices/workSlice.ts` ⚑ (task 37) — `negotiationSeen`, `markNegotiationSeen`
  (modified)
- `desktop/src/api/work.ts` ⚑ — the `negotiation` comment type, appended beside task 104's (modified)

#### Interfaces — complete: appended to `desktop/src/api/work.ts`

```ts
// SW.19's negotiation comment: metadata pointing at Phase T's thread; its text stays in Phase T's
// messages (task 99). An Orchestrator writes one on each of the two issues (task 105, gap 7).
export interface NegotiationComment {
  id: string; issueId: string; kind: "negotiation";
  negotiation: { threadId: string; counterpartIssueId: string };
  attribution: AgentAttribution; createdAt: string; erasedAt: string | null;
}
```

#### Interfaces — complete: `desktop/src/components/issue/NegotiationRow.tsx`, part 1: the fact and the words

```tsx
// Product text from typed fields only (decision 1). Nothing here reads ThreadMessage.text, a
// transcript, the terminal or a session (OD-8, OD-15); a test holds that by source.
import { useEffect, useState, type JSX } from "react";
import type { NegotiationComment } from "../../api/work";
import type { PersonRef, Resolution, ThreadView } from "../../store/types"; // TM-52's
import { MarkGlyph } from "../glyphs/MarkGlyph";

export type NegotiationState = "open" | "escalated" | "resolved";
export interface NegotiationFact {
  threadId: string; state: NegotiationState;
  thisIssueId: string; otherIssueId: string;
  other: PersonRef;                       // the other side's owner
  resolution: Resolution | null;          // from the accepting message's fields.resolution
  thisGoesFirst: boolean | null;          // sequence only: null when the proposal names no first
  agreedAt: string | null;                // the accept message's `at`, or the added resolvedAt
  deadline: string | null;                // escalated: the Desk decide's deadline
  informed: PersonRef[] | null;           // null = the read model does not say (gap 1)
  asked: PersonRef[] | null;
  round: number; roundLimit: number;
}

type ViewPlus = ThreadView & { resolvedAt?: string; informed?: PersonRef[]; asked?: PersonRef[] };

/** One comment and one thread view → one fact. `mySideAgent` is this issue's Orchestrator id, so
 *  a sequence proposal's `first` can be read as "this issue goes first" or not. */
export function negotiationFact(c: NegotiationComment, v: ViewPlus, mySideAgent: string): NegotiationFact {
  const accept = [...v.messages].reverse().find((m) => m.kind === "accept" && !m.tombstoned);
  const proposed = v.messages.find((m) => m.id === accept?.fields.inReplyTo)?.fields.resolution ?? null;
  const resolved = accept !== undefined && v.thread.state !== "escalated";
  return {
    threadId: v.thread.id, state: resolved ? "resolved" : v.thread.state,
    thisIssueId: c.issueId, otherIssueId: c.negotiation.counterpartIssueId, other: v.thread.other,
    resolution: resolved ? (proposed?.kind ?? null) : null,
    thisGoesFirst: proposed?.kind === "sequence" && proposed.first ? proposed.first === mySideAgent : null,
    agreedAt: resolved ? (v.resolvedAt ?? accept!.at) : null,
    deadline: v.thread.state === "escalated" ? v.thread.deadline : null,
    informed: v.informed ?? null, asked: v.asked ?? null,
    round: v.thread.round, roundLimit: v.thread.maxRounds,
  };
}

const first = (p: PersonRef): string => p.displayName.split(" ")[0] ?? p.displayName;
const names = (ps: PersonRef[]): string => ps.map(first).join(" and ");

/** The row's sentence. `alias` is the replica's display alias for an is-… (decision 4); `hhmm`
 *  formats a time in the reader's zone. Every clause is present only when its field is. */
export function negotiationWords(f: NegotiationFact, alias: (id: string) => string, hhmm: (iso: string) => string): string {
  const other = `${alias(f.otherIssueId)} (${first(f.other)}'s)`;
  const told = (): string =>
    (f.informed && f.informed.length > 0 ? `; ${names(f.informed)} informed` : "") +
    (f.asked && f.asked.length > 0 ? `; ${names(f.asked)} asked` : "");
  if (f.state === "open") return `Negotiating with ${first(f.other)}'s Orchestrator about ${other} · round ${f.round} · limit ${f.roundLimit}`;
  if (f.state === "escalated") return `Waiting for a person's decision about ${other}${f.deadline ? ` · by ${hhmm(f.deadline)}` : ""}`;
  const what = f.resolution === "sequence" && f.thisGoesFirst !== null
    ? `${f.thisGoesFirst ? "Sequenced before" : "Sequenced after"} ${other}`
    : `Resolved with ${other}${f.resolution ? `: ${f.resolution}` : ""}`;
  return `${what}. Agreed by both Orchestrators${f.agreedAt ? ` ${hhmm(f.agreedAt)}` : ""}${told()}.`;
}
```

The resolved sentence for the scope's example, read on EP-142 by Dylan with `informed: [Dylan]`, is
exactly 05 §2.2's: *"Sequenced after EP-150 (Alice's). Agreed by both Orchestrators 10:41; Dylan
informed."*; on EP-150 it reads *"Sequenced before EP-142 (Dylan's). …"*. A resolution other than
`sequence` prints its TM-52 word verbatim in mono (*"Resolved with EP-150 (Alice's): yield."*).
