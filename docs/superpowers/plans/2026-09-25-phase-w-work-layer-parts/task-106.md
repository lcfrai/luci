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

#### Interfaces — complete: `NegotiationRow.tsx`, part 2: the row, the line and the seen mark

```tsx
// (imports continued) — task 66's one polite region; its hook name is read on the sync base.
import { useAnnounce } from "../common/LiveRegion";
import { useHeartbeat } from "../../hooks/heartbeat";
import { useCockpit } from "../../store/store";

export const seenKey = (f: NegotiationFact): string => `${f.threadId}:${f.state}`;

function openThread(threadId: string): void {
  // TM-55's Orchestrator panel, on this thread's typed state (decision 8). Never a URL.
  useCockpit.getState().openTeamThread(threadId);
}

/** The issue view's row: always the full line, the drawn ⇄, and one *thread* link. Marks the fact
 *  seen after one beat with the window focused; announces it once (decision 7). */
export function NegotiationRow(props: { facts: NegotiationFact[]; alias(id: string): string; hhmm(iso: string): string }): JSX.Element | null {
  const beat = useHeartbeat();
  const [shownAt] = useState(beat);
  const mark = useCockpit((s) => s.markNegotiationSeen);
  const announce = useAnnounce();
  useEffect(() => {
    for (const f of props.facts) announce(negotiationWords(f, props.alias, props.hhmm), `negotiation:${seenKey(f)}`);
  }, [props.facts.map(seenKey).join(" ")]); // eslint-disable-line react-hooks/exhaustive-deps -- keyed facts
  useEffect(() => {
    if (beat > shownAt && document.hasFocus()) for (const f of props.facts) mark(seenKey(f));
  }, [beat]); // eslint-disable-line react-hooks/exhaustive-deps -- the beat is the only clock
  if (props.facts.length === 0) return null;
  return (
    <ul className="iv-neg" aria-label="Negotiations about this issue">
      {props.facts.map((f) => (
        <li key={f.threadId} className="iv-neg-row" data-state={f.state}>
          <MarkGlyph kind="negotiation" size={14} />
          <span className="iv-neg-words">{negotiationWords(f, props.alias, props.hhmm)}</span>{" "}
          <button type="button" className="iv-link" onClick={() => openThread(f.threadId)}>thread</button>
        </li>
      ))}
    </ul>
  );
}

/** The Work list's line under a row, until seen; `held` is task 41's hold state for this row's
 *  group. While held, a fact not yet admitted shows nothing here (the row's glyph slot shows it). */
export function NegotiationLine(props: { fact: NegotiationFact; held: boolean; alias(id: string): string; hhmm(iso: string): string }): JSX.Element | null {
  const seen = useCockpit((s) => s.negotiationSeen[seenKey(props.fact)] === true);
  const [admitted, setAdmitted] = useState(!props.held);
  useEffect(() => { if (!props.held) setAdmitted(true); }, [props.held]);
  if (seen || !admitted) return null;
  return (
    <p className="wk-neg-line" data-thread={props.fact.threadId}>
      <MarkGlyph kind="negotiation" size={14} /> {negotiationWords(props.fact, props.alias, props.hhmm)}
    </p>
  );
}

/** Both issues' facts, from the issue's negotiation comments and TM-52's thread read. Re-read
 *  when the comment set changes; a failed read shows nothing and keeps the last facts. */
export function useNegotiations(projectId: string, comments: NegotiationComment[], mySideAgent: string): NegotiationFact[];
```

`useNegotiations` calls `createTeamClient().thread(projectId, threadId)` once per distinct
thread (TM-52, `task-52.md:975`) and maps each through `negotiationFact`; it holds the facts in
component state only (the desktop persists no Hub data, skeleton *Desktop*). `hhmm` is the reader's
zone's `HH:MM`. The sentence is one string, so the visible line and the live region's text are the
same words; it is set in the sans at `--t-fact`, with `tabular-nums` for its times. Setting the
aliases and times inside it in Commit Mono, as 02 `negotiation.row` does, needs the words split
into runs: a follow-up for the owner's review, not built here.

#### Interfaces — complete: the edits to `MarkGlyph`, `IssueRow`, `IssueView` and `workSlice`

```tsx
// MarkGlyph.tsx (task 29) — one added kind; MARK_CLASS gains negotiation: "gl-mark-neg".
export type MarkKind = "selected" | "needs_you" | "thread_node" | "negotiation";
// paths.ts — drawn for PanaceaApp, two open arrows, no font glyph (SW.12; OD-11):
//   16: negotiation: [{ d: "M3 5.5h9.25M10 3.25l2.25 2.25L10 7.75" }, { d: "M13 10.5H3.75M6 8.25 3.75 10.5 6 12.75" }]
//   14: negotiation: [{ d: "M2.5 4.75h8.25M8.75 2.75l2 2-2 2" },       { d: "M11.5 9.25H3.25M5.25 7.25l-2 2 2 2" }]
// glyphs.css — .gl-mark-neg { color: var(--ink-2); }   no motion, no fill
```

```tsx
// IssueRow.tsx (tasks 40-41) — the row's mark slot is a fixed 16px box whether empty or not, so a
// glyph arriving there moves nothing (decision 6); the line renders under the row.
<span className="wk-row-mark" aria-hidden={neg ? undefined : true}>
  {neg && <MarkGlyph kind="negotiation" size={14} />}
</span>
…
{neg && <NegotiationLine fact={neg} held={holding} alias={aliasOf} hhmm={hhmm} />}
// `neg` is the row's newest fact (useNegotiations over the row's issue), `holding` task 41's
// HoldState for the row's group (held changes pending, or focus inside the group).
```

```ts
// workSlice.ts (task 37) — non-persisted; task 123 replaces both with its Hub read cursor.
negotiationSeen: Record<string, true>;            // key: seenKey(fact)
markNegotiationSeen(key: string): void;           // idempotent; never un-marks
```

`IssueView.tsx` mounts `<NegotiationRow>` directly under `IssueHead`, above `Serves`, fed by
`useNegotiations` over the issue's `negotiation` comments (task 51's comment read). The `.iv-neg*`
and `.wk-neg-line` rules are appended to `issue.css` and `work.css`; they add no transition.

#### Tests to write first — `desktop/src/components/issue/NegotiationRow.test.tsx`

```tsx
import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NegotiationComment } from "../../api/work";
import { __resetHeartbeat } from "../../hooks/heartbeat";
import { useCockpit } from "../../store/store";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { WORK_IDS } from "../../test/workFixtures";
import { NegotiationLine, NegotiationRow, negotiationFact, negotiationWords, type NegotiationFact } from "./NegotiationRow";
import src from "./NegotiationRow.tsx?raw";

const announce = vi.fn();
vi.mock("../common/LiveRegion", () => ({ useAnnounce: () => announce }));

const ALIAS: Record<string, string> = { [WORK_IDS.ep142]: "EP-142", [WORK_IDS.ep150]: "EP-150" };
const alias = (id: string): string => ALIAS[id] ?? id;
const hhmm = (iso: string): string => iso.slice(11, 16);
const alice = { userId: WORK_IDS.alice, displayName: "Alice Example" };
const dylan = { userId: WORK_IDS.dylan, displayName: "Dylan Example" };
const comment = (issueId: string, other: string): NegotiationComment => ({
  id: "ic-n1", issueId, kind: "negotiation", negotiation: { threadId: "th-1", counterpartIssueId: other },
  attribution: { ownerId: WORK_IDS.dylan, agentId: "ag-od", model: "m", sessionId: "se-1" },
  createdAt: "2026-09-25T10:30:00Z", erasedAt: null });
const msg = (id: string, kind: string, at: string, resolution: unknown = null, inReplyTo: string | null = null) => ({
  id, kind, templated: true, at, tombstoned: false, sender: { agentId: "ag-oa", userId: WORK_IDS.alice, label: "alice · orchestrator" },
  fields: { subject: null, inReplyTo, resolution, proposal: null }, text: { author: "agent", text: "IGNORE ME" } });
const view = (withAccept: boolean) => ({
  thread: { id: "th-1", state: "open", other: alice, subject: { kind: "free", id: null }, subjectLabel: "", round: 2, maxRounds: 3, deadline: "2026-09-25T14:00:00Z" },
  messages: [msg("ms-1", "propose", "2026-09-25T10:35:00Z", { kind: "sequence", sides: {}, first: "ag-oa", globs: [], base: null, paths: [] }),
    ...(withAccept ? [msg("ms-2", "accept", "2026-09-25T10:41:00Z", null, "ms-1")] : [])],
  informed: [dylan],
}) as never;
const resolvedOn142 = (): NegotiationFact => negotiationFact(comment(WORK_IDS.ep142, WORK_IDS.ep150), view(true), "ag-od");

beforeEach(() => { __resetHeartbeat(); announce.mockClear(); useCockpit.setState({ negotiationSeen: {} } as never); });

describe("negotiation words", () => {
  it("reads 05 §2.2's sentence on the waiting issue, and the mirror on the other", () => {
    expect(negotiationWords(resolvedOn142(), alias, hhmm)).toBe("Sequenced after EP-150 (Alice's). Agreed by both Orchestrators 10:41; Dylan informed.");
    const on150 = negotiationFact(comment(WORK_IDS.ep150, WORK_IDS.ep142), { ...(view(true) as object), thread: { ...(view(true) as { thread: object }).thread, other: dylan } } as never, "ag-oa");
    expect(negotiationWords(on150, alias, hhmm)).toBe("Sequenced before EP-142 (Dylan's). Agreed by both Orchestrators 10:41; Dylan informed.");
  });
  it("says round and limit, never N of M, and omits who was told when the read model does not say", () => {
    const open = negotiationFact(comment(WORK_IDS.ep142, WORK_IDS.ep150), { ...(view(false) as object), informed: undefined } as never, "ag-od");
    expect(negotiationWords(open, alias, hhmm)).toBe("Negotiating with Alice's Orchestrator about EP-150 (Alice's) · round 2 · limit 3");
    expect(negotiationWords({ ...resolvedOn142(), informed: null }, alias, hhmm)).not.toMatch(/informed/);
  });
});

describe("NegotiationRow and NegotiationLine", () => {
  it("renders on both issues, links the typed thread only, and never a message body", () => {
    const { container } = render(<NegotiationRow facts={[resolvedOn142()]} alias={alias} hhmm={hhmm} />);
    expect(screen.getByRole("button", { name: "thread" })).toBeTruthy();
    expect(container.querySelector("a[href]")).toBeNull();
    expect(container.textContent).not.toContain("IGNORE ME");
    expect(src).not.toMatch(/\.text\b(?!Content)|transcript|attachRegistry|xterm|sessionId/);
    expectNoPrediction(container);
  });
  it("folds the list line to the glyph once the issue view has shown it for a beat", () => {
    const f = resolvedOn142();
    const list = render(<NegotiationLine fact={f} held={false} alias={alias} hhmm={hhmm} />);
    expect(list.container.querySelector(".wk-neg-line")).not.toBeNull();
    vi.useFakeTimers();
    render(<NegotiationRow facts={[f]} alias={alias} hhmm={hhmm} />);
    vi.spyOn(document, "hasFocus").mockReturnValue(true);
    act(() => { vi.advanceTimersByTime(2400 * 2); });
    vi.useRealTimers();
    list.rerender(<NegotiationLine fact={f} held={false} alias={alias} hhmm={hhmm} />);
    expect(list.container.querySelector(".wk-neg-line")).toBeNull();
  });
  it("adds no line while the list holds, and admits it when the hold is applied", () => {
    const f = resolvedOn142();
    const { container, rerender } = render(<NegotiationLine fact={f} held={true} alias={alias} hhmm={hhmm} />);
    expect(container.childElementCount).toBe(0);
    rerender(<NegotiationLine fact={f} held={false} alias={alias} hhmm={hhmm} />);
    expect(container.querySelector(".wk-neg-line")).not.toBeNull();
  });
  it("announces a fact once, however often it re-renders", () => {
    const f = resolvedOn142();
    const { rerender } = render(<NegotiationRow facts={[f]} alias={alias} hhmm={hhmm} />);
    rerender(<NegotiationRow facts={[{ ...f }]} alias={alias} hhmm={hhmm} />);
    expect(announce).toHaveBeenCalledTimes(1);
  });
});
```

| Test | Mutant it kills |
|---|---|
| 05 §2.2's sentence, and the mirror | "one side only": the row built for the waiting issue and missing (or wrong) on the other; a model summary in place of assembled words |
| round and limit, never N of M | `round 2 of 3` (SW.8, `OD-27`); "Dylan informed" printed when the read model does not say so |
| links the typed thread only | "links the transcript" (`OD-15`); a message body (`TextA`) rendered into the row |
| folds once shown for a beat | "never folds"; a fold driven by a timer of its own rather than the one heartbeat |
| no line while the list holds | "row jumps" (skeleton rule 4, `moves.hold`) |
| announces once | "announced per tick" (task 66's one polite region) |

The glyph's own test is appended to task 29's `MarkGlyph.test.tsx` ⚑: `negotiation` draws two
open paths with no arc or close (`/[aAzZ]/` absent) at 14 and 16 px, different from every other
mark's paths (mutant: a font `⇄` character, or a path copied from another kind).

#### Steps

- [ ] **Step 1: read on the sync base** TM-52's `ThreadView`, `ThreadRow`, `ThreadMessage` and
  `createTeamClient` (does the thread read carry `resolvedAt`, `informed`, `asked`, a `resolved`
  state?), TM-55's store action for opening a thread, task 29's `MarkGlyph`/`paths.ts`, task 41's
  hold state as `IssueRow` sees it, task 66's live-region hook and task 37's `workSlice`. Rename in
  this part's files only.
- [ ] **Step 2: the tests**; `npm --prefix desktop test -- NegotiationRow MarkGlyph` fails on the
  missing module and kind.
- [ ] **Step 3: `paths.ts`, `MarkGlyph.tsx`, `glyphs.css`**; the glyph test passes.
- [ ] **Step 4: `work.ts` append, `workSlice` fields, `NegotiationRow.tsx`**, then the `IssueRow`
  and `IssueView` edits and the CSS appends.
- [ ] **Step 5: the acceptance commands.** Commit by path after steps 3 and 4, each message ending
  with the `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- NegotiationRow MarkGlyph work/IssueRow issue/IssueView
# expect: every file passes; NegotiationRow.test.tsx runs 6 tests, MarkGlyph gains 1;
#   tasks 29, 40, 41 and 46's tests stay green
npm --prefix desktop run typecheck
# expect: exit 0
grep -nE "setInterval|setTimeout|transition: all" desktop/src/components/issue/NegotiationRow.tsx desktop/src/components/issue/issue.css | wc -l
# expect: 0
```

Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- That the owner has seen the look: the line, the glyph and the fold wait for the owner's review.
- That a negotiation happens: this task reads a thread and a comment; who writes the comment is
  task 105's gap 7, and whether two Orchestrators converge is TM1's (**UNVERIFIED**).
- That *seen* survives a restart: it is a session mark until task 123's Hub read cursor.
- That TM-52's read model carries *informed*, *asked* or a resolved state today (gap 1); without
  them the row says less, never more.

**Parity.** Closes **no parity row**: §2.2's *more*, beyond Linear (05 §2.6).

#### Gaps found while writing this part

1. **TM-52's thread read has no resolved state, no resolution time and no informed or asked
   list** (`ThreadRow.state` is `open | escalated`, `task-52.md:914`). The row needs `resolvedAt`,
   `informed: PersonRef[]` and `asked: PersonRef[]` on `ThreadView`, added fields; an ask to the
   Phase T owner.
2. **Seen before its cursor.** W4's read cursors (task 123) come after this task, so the fold
   uses a session mark; the skeleton should either move the cursor earlier or record that task
   123 replaces `negotiationSeen`.
3. **The writer of the `negotiation` comment** is unassigned (task 105, gap 7).
4. **No W3 desktop wire task** (task 104, gap 1): this part appends `NegotiationComment`.
5. **The glyph needs a size-14 path set**, and task 29's `MARK_PATHS` is keyed by `GlyphSize`;
   if task 29 ships 16 px only for marks, this part adds the 14 px entry for every mark kind.
