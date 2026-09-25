### Task 48: the issue view, part 3: relations, the live agent line, *done, not proven* and its reason

Stream **WD3** (the issue view), about 2.5 hours (an estimate, not a measurement). Depends on task
46 (the view, the pickers, `AuthorClass`, `clock`). Uses SW.1's relations (task 11's five words),
SW.4's `StateMove` and `IssueHead.proof`, SW.12's `StateGlyph notProven`, SW.16's `mutate`, task
40's `AgentLine`, the one heartbeat (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts:64`) and
`formatWaiting` (`desktop/src/lib/time.ts:49`). **Runs after tasks 40 and 43** as well: it edits
their `IssueRow.tsx` and `BoardView.tsx` by one call each (a gap in the skeleton's dependsOn,
named below).

This part closes the three places where an issue view is most tempted to guess:

1. **A relation is one record seen from both ends.** EP-142 *blocked by* EP-150 is EP-150
   *blocking* EP-142, because the Hub stores one row and each end reads its own word (task 11,
   `task-11.md:102-112`). The view never keeps a relation of its own.
2. **The agent line is built from facts, never from the terminal.** One line per the reader's
   **own** agent on the issue, from the control plane's sessions with an `issueRef` (task 19) and
   the session status; never PTY bytes (`OD-8`). A teammate's work is the person, with no dot and
   no count (05 §4.4 rule 3).
3. **Done by hand is not done by evidence.** On an issue that serves criteria, choosing `done`
   asks for a one-line reason and sends `StateMove{to: "done", cause: "hand", reason}`; the Hub
   records `proof: not_proven` (task 10, `task-10.md:47-55`), and the issue then carries
   `StateGlyph notProven` everywhere it appears. The **word** is the Hub's; this part maps it to a
   sentence and never decides it.

**Decisions made here, so nobody re-derives them:**

1. **Five words.** `blocks` reads *blocking*, `blocked_by` *blocked by*, `duplicates` *duplicate
   of*, `duplicated_by` *duplicated by*, `relates` *related*. A person adds only the three writable
   kinds plus *blocked by*, which is sent as the other issue's `blocks` from this end: task 11's
   `NormaliseRelation` accepts either word from either end (`task-11.md:263-264`) for delete, but a
   `RelationRequest` naming `blocked_by` is 400 (`:24`). So *blocked by EP-150* is sent as
   `{kind: "blocks"}` on EP-150's id, subject EP-150.
2. **The other end refreshes through the feed.** The Hub answers with the subject's head only and
   emits `issue.related` carrying `otherIssueId` (`task-13.md:166-170`). The replica must re-fetch
   **both** the subject and `otherIssueId` on that event; task 55's scope re-fetches the subject
   only. A gap for task 55, named below.
3. **A cross-owner relation is one op, and nothing else.** The Hub's event carries
   `otherAssigneeId` when the other issue's assignee is someone else (`task-11.md:329-332`), and
   task 65 turns it into that owner's Desk *inform*. This view sends the relation and no act at
   any session, its own or anyone's (05 §3.7 rule 3).
4. **"Blocking dylan · agent 1 · waiting 42m" is shown only from the reader's own facts.** On a
   blocker that is not `landed` or `done`, for each issue it `blocks`, the reader's own sessions on
   that issue whose status is `idle` are listed with `formatWaiting(lastOutputAt, beat)`. It says
   the two facts sit together, not that one caused the other; the Desk row with the cost of the
   wait is task 63. Another person's stalled agent cannot be seen in local mode (`OD-20`).
5. **The hand-done reason step lives in the state picker**, where task 46 left `done` shown and
   unsendable. The reason is trimmed and must be non-empty; there is no length rule beyond the
   Hub's (task 10's 400 names `reason`).
6. **Who set it and when.** *"the head shows the reason, its author and time"* needs the author
   and time of the state move, which `IssueHead` does not carry (`task-03.md:282-303`: only
   `stateReason` and `updatedAt`). This part reads `stateMovedBy` and `stateMovedAt` as nullable
   head fields, a gap for tasks 10 and 37; until they exist it shows the reason and *set by hand*.
7. **The evidence detail is a cited record's own words.** *own evidence at 7c1f0a2 (test run, exit
   0)* needs the cited record's `because` line (task 23 writes it: `task-23.md:553`), which no W1
   read returns: the citation lives in task 10's `issue_proof_citations`. This part reads
   `IssueHead.proofCitations` (a gap, named below) and renders each `because` **verbatim**; with
   none, the line is the word and the land commit alone.
8. **`glyphPropsOf(head)` is the one rule for a glyph.** Every place that draws an issue's glyph
   (the view, the row, the board card) calls it, so *done, not proven* cannot be drawn in one place
   and missed in another.

**Files:**
- `desktop/src/components/issue/Relations.tsx` — the relation list and its add and remove (new)
- `desktop/src/components/issue/IssueAgentLine.tsx` — own agents' lines, the teammate line (new)
- `desktop/src/components/issue/ProofLine.tsx` — the result words and the reason (new; not yet
  in the File map)
- `desktop/src/components/issue/glyphOf.ts` — `glyphPropsOf(head)` (new; not yet in the File map)
- `desktop/src/components/issue/pickers/StatePicker.tsx` ⚑ (task 46's) — the reason step (modified)
- `desktop/src/components/issue/IssueHead.tsx` ⚑, `IssueView.tsx` ⚑ (tasks 46-47's) — the glyph
  through `glyphPropsOf`; `ProofLine`, `Relations` and `IssueAgentLine` mounted (modified)
- `desktop/src/components/work/IssueRow.tsx` ⚑ (task 40's), `BoardView.tsx` ⚑ (task 43's) — their
  `StateGlyph` props through `glyphPropsOf` (modified; one call each)
- `desktop/src/components/issue/issue.css` ⚑ — the `.iv-rel*`, `.iv-agents*`, `.iv-proof*` and
  `.iv-reason*` rules, appended (modified)
- `desktop/src/components/issue/{Relations,IssueAgentLine,ProofLine}.test.tsx`,
  `pickers/reason.test.tsx`, `glyphOf.test.ts` — the tests (new)

#### Interfaces — complete: `desktop/src/components/issue/glyphOf.ts`

```ts
import type { IssueHead } from "../../api/work";
import type { IssueState, Priority } from "../glyphs/StateGlyph";

export interface GlyphProps { state: IssueState; priority: Priority; notProven: boolean }

/** Decision 8: the one rule for an issue's glyph. `notProven` is the Hub's word, read, never
 *  inferred from the cause or the reason (SW.4). */
export function glyphPropsOf(head: Pick<IssueHead, "state" | "priority" | "proof">): GlyphProps {
  return {
    state: head.state,
    priority: head.priority,
    notProven: head.state === "done" && head.proof === "not_proven",
  };
}
```

The edits to the three callers are one line each: `<StateGlyph {...glyphPropsOf(head)} size={…} />`
in `IssueHead.tsx` (replacing task 46's three props), in task 40's `IssueRow.tsx` and in task 43's
board card. Each file keeps its own `size`.

#### Interfaces — complete: `desktop/src/components/issue/ProofLine.tsx`

```tsx
import type { JSX } from "react";
import type { IssueHead, User } from "../../api/work";
import { clock } from "./IssueHead";

/** One cited evidence record, as the head would carry it (decision 7; a gap for task 10). */
export interface ProofCitation {
  criterionId: string;
  cause: "evidence_observed" | "evidence_recorded";
  commit: string;         // 40 hex
  because: string;        // the record's own line, rendered verbatim
  recordedAt: string;
}

/** The head fields this line reads. The last three are decisions 6 and 7's gaps (null or empty
 *  until tasks 10 and 37 carry them). */
export type ProofHead = Pick<IssueHead, "state" | "proof" | "landCommit" | "stateReason"> & {
  stateMovedBy: User | null;
  stateMovedAt: string | null;
  proofCitations: readonly ProofCitation[];
};

const short = (c: string): string => c.slice(0, 7);

function Cited(props: { c: ProofCitation }): JSX.Element {
  return (
    <span className="iv-proof-cite">
      {" "}(<span className="iv-proof-because" data-verbatim>{props.c.because}</span>) ·{" "}
      <time className="iv-fact" data-fact="evidence-at" dateTime={props.c.recordedAt}>{clock(props.c.recordedAt)}</time>
    </span>
  );
}

/** SW.4's word, as words. The switch maps; it decides nothing. "" (before landed) draws nothing. */
export function ProofLine(props: { head: ProofHead }): JSX.Element | null {
  const { head } = props;
  const commit = <span className="iv-fact" data-fact="land-commit">{short(head.landCommit)}</span>;
  const cites = head.proofCitations.map((c) => <Cited key={`${c.criterionId} ${c.commit}`} c={c} />);
  switch (head.proof) {
    case "":
      return null;
    case "proven":
      return <p className="iv-proof" data-proof="proven">done · own evidence at {commit}{cites}</p>;
    case "proven_self_attested":
      return <p className="iv-proof" data-proof="proven_self_attested">done · proven, self-attested at {commit}{cites}</p>;
    case "waiting":
      return <p className="iv-proof" data-proof="waiting">landed at {commit} · waiting for evidence on every criterion it serves</p>;
    case "no_criteria":
      return <p className="iv-proof" data-proof="no_criteria">{head.state} · serves no criterion, so there is nothing to prove</p>;
    case "not_proven":
      return (
        <p className="iv-proof" data-proof="not_proven">
          done, not proven · <span className="iv-reason-text" data-verbatim>{head.stateReason}</span> ·{" "}
          {head.stateMovedBy === null || head.stateMovedAt === null ? "set by hand" : (
            <>{head.stateMovedBy.displayName},{" "}
              <time className="iv-fact" data-fact="moved-at" dateTime={head.stateMovedAt}>{clock(head.stateMovedAt)}</time></>
          )}
        </p>
      );
  }
}
```

The reason and each `because` are `data-verbatim`: they are a person's and a checker's own words
(*"shipped behind a flag, 80% of users"* is a reason, not a prediction), so task 20's scan skips
them and checks everything else on the line. `proven_self_attested` says so in the line itself
(task 10 decision 6: recorded by the issue's own starter). No branch of the switch uses a colour.

#### Interfaces — complete: appended to `desktop/src/components/issue/issueData.ts`

```ts
export interface IssueSummary { head: IssueHead; title: string }

/** The other ends of a relation list, through the same `r.issue` read as the view (task 46). An
 *  id not in memory is left out; the row then reads the id alone (Relations, below). */
export function useIssueSummaries(ids: readonly string[]): ReadonlyMap<string, IssueSummary> {
  return useWork((r) => {
    const out = new Map<string, IssueSummary>();
    for (const id of ids) {
      const rec = r.issue(id);
      if (rec) out.set(id, { head: rec.head, title: rec.revision.content.title.text });
    }
    return out;
  });
}
```

#### Interfaces — complete: `desktop/src/components/issue/Relations.tsx`, part 1: the words and ops

```tsx
import { useState, type JSX } from "react";
import type { IssueHead, Relation, User } from "../../api/work";
import { useHeartbeat } from "../../hooks/heartbeat";
import { formatWaiting } from "../../lib/time";
import { useCockpit } from "../../store/store";
import { mutate, type WorkOp } from "../../work/replica/ops";
import { search, useWork } from "../../work/replica/replica";
import { StateGlyph } from "../glyphs/StateGlyph";
import { glyphPropsOf } from "./glyphOf";
import { useIssueSummaries } from "./issueData";
import { Picker } from "./pickers/Picker";

/** Task 11's five words, as each end reads them (decision 1). */
export type RelationWord = "blocks" | "blocked_by" | "duplicates" | "duplicated_by" | "relates";
export const RELATION_LABEL: Readonly<Record<RelationWord, string>> = {
  blocks: "blocking", blocked_by: "blocked by", duplicates: "duplicate of",
  duplicated_by: "duplicated by", relates: "related",
};

/** What a person adds from this end. `M` then `B` `X` `R` `D` (task 54) call relationOp. */
export type AddKind = "blocked_by" | "blocks" | "relates" | "duplicates";
export const ADD_ORDER: readonly AddKind[] = ["blocked_by", "blocks", "relates", "duplicates"];

export function relationOp(subject: string, kind: AddKind, other: string): WorkOp {
  // blocked_by is read-only on the wire (task 11): write it as the other issue blocking this one.
  return kind === "blocked_by"
    ? { kind: "relation", issueId: other, add: { kind: "blocks", issueId: subject } }
    : { kind: "relation", issueId: subject, add: { kind, issueId: other } };
}

/** Either end may remove, with its own word (task 11: the pair is normalised before lookup). */
export const removeOp = (subject: string, r: Relation): WorkOp =>
  ({ kind: "relation", issueId: subject, remove: { kind: r.kind, issueId: r.issueId } });

type SessionFacts = { id: string; issueRef: string; status: string; lastOutputAt: string; createdAt: string };

/** Decision 4: my idle agents on an issue this one blocks, while this one is not landed. Pure;
 *  the wait is formatted on the beat by StalledLine, below. */
export function stalledAgents(
  head: IssueHead, blocked: string, sessions: readonly SessionFacts[], me: User,
): { label: string; since: string }[] {
  if (head.state === "landed" || head.state === "done") return [];
  const mine = sessions.filter((s) => s.issueRef.startsWith(`issue:${blocked}@`))
    .sort((x, y) => x.createdAt.localeCompare(y.createdAt));
  const first = (me.displayName.split(" ")[0] ?? me.displayName).toLowerCase();
  return mine.flatMap((s, n) => s.status === "idle"
    ? [{ label: `blocking ${first} · agent ${n + 1}`, since: s.lastOutputAt }] : []);
}

/** The only subscriber to the heartbeat in this file, mounted only for a stalled agent, so a
 *  relation list with none never re-renders once a second (heartbeat.ts:59-63). */
function StalledLine(props: { label: string; since: string }): JSX.Element {
  const now = useHeartbeat();
  return <span className="iv-fact" data-fact="stalled">{props.label} · {formatWaiting(props.since, now)}</span>;
}
```

`stalledAgents` numbers agents the way task 47's `agentsWhoRead` does (first name, `agent N` by
`createdAt` among the reader's sessions on that issue), so the same agent has the same name in the
revision list, the relation line and the agent line.

#### Interfaces — complete: `Relations.tsx`, part 2: the list and the add

```tsx
export function Relations(props: { head: IssueHead; me: User }): JSX.Element {
  const { head, me } = props;
  const others = useIssueSummaries(head.relations.map((r) => r.issueId));
  const sessions = Object.values(useCockpit((s) => s.sessions));
  const [adding, setAdding] = useState<AddKind | "choose" | null>(null);
  return (
    <section className="iv-rels" aria-label="Relations">
      <h2 className="iv-h2">Relations</h2>
      <ul className="iv-rel-list">
        {head.relations.map((r) => {
          const o = others.get(r.issueId);
          return (
            <li key={`${r.kind} ${r.issueId}`} className="iv-rel" data-other={r.issueId}>
              <span className="iv-rel-word">{RELATION_LABEL[r.kind as RelationWord]}</span>
              {o ? <StateGlyph {...glyphPropsOf(o.head)} size={14} /> : null}
              <span className="iv-fact" data-fact="alias">{o ? o.head.alias : r.issueId}</span>
              <span className="iv-rel-title">{o ? o.title : "not in your replica"}</span>
              <span className="iv-rel-owner">{o?.head.assignee?.displayName ?? "no assignee"}</span>
              {r.kind === "blocks"
                ? stalledAgents(head, r.issueId, sessions, me).map((a) => <StalledLine key={a.label} {...a} />)
                : null}
              <button type="button" className="iv-rel-remove" onClick={() => mutate(removeOp(head.id, r))}
                aria-label={`Remove ${RELATION_LABEL[r.kind as RelationWord]} ${o?.head.alias ?? r.issueId}`}>Remove</button>
            </li>
          );
        })}
      </ul>
      <button type="button" className="iv-rel-add" aria-expanded={adding !== null}
        onClick={() => setAdding("choose")}>Add a relation</button>
      {adding === "choose" ? (
        <Picker label="Relation" items={ADD_ORDER.map((k) => ({ id: k, label: RELATION_LABEL[k] }))}
          onChoose={(k) => { setAdding(k as AddKind); return "stay"; }} onClose={() => setAdding(null)} />
      ) : null}
      {adding !== null && adding !== "choose" ? (
        <OtherIssuePicker subject={head.id} kind={adding} onClose={() => setAdding(null)} />
      ) : null}
    </section>
  );
}

function OtherIssuePicker(props: { subject: string; kind: AddKind; onClose(): void }): JSX.Element {
  const { subject, kind, onClose } = props;
  const hits = useWork((r) => search(r, "").filter((h) => h.id !== subject));
  return (
    <Picker label={`${RELATION_LABEL[kind]} which issue`} onClose={onClose}
      items={hits.map((h) => ({ id: h.id, label: `${h.alias} ${h.title}` }))}
      onChoose={(other) => { mutate(relationOp(subject, kind, other)); }} />
  );
}
```

`r.kind as RelationWord` is safe because task 11 widens `RelationKind` to the five words, and
SW.15's desktop `Relation.kind` mirrors it (task 37); task 20's fixture type lists only the
three writable words (`task-20.md:178`), so the fixtures need the two inverse words too (a gap).
The first `Picker` returns `"stay"` (task 46's `Picker` contract) so its choice swaps it for the
second instead of closing; `Esc` on either closes the whole add. `useHeartbeat` is the one clock (`heartbeat.ts:64`, a
1 Hz tick shared by the window): no component in this part starts a timer, so a *waiting* figure
changes only on the beat.

#### Interfaces — complete: `desktop/src/components/issue/IssueAgentLine.tsx`

```tsx
// The line's source is the control plane's sessions with an issueRef (task 19) and task 40's
// AgentLine, which reads status and lane facts. Nothing here imports the terminal, the attach
// registry (src/api/attachRegistry.ts) or any transcript (OD-8); a test holds that by source.
import type { JSX } from "react";
import type { IssueHead, User } from "../../api/work";
import { useCockpit } from "../../store/store";
import type { Session } from "../../store/types";
import { AgentLine } from "../work/AgentLine";

function openSession(s: Session): void {
  const st = useCockpit.getState();
  st.selectWorktree(s.worktreeId);                   // uiSlice.ts:58
  st.focusSession(s.id);                             // uiSlice.ts:60
  st.setLens("develop");                             // uiSlice.ts:61
}

export function IssueAgentLine(props: { head: IssueHead; me: User }): JSX.Element | null {
  const { head, me } = props;
  const all = useCockpit((s) => s.sessions);
  // Numbered over every session this reader ran on the issue, exited ones included, so an agent
  // keeps its number when an earlier one exits (the rule of task 47's agentsWhoRead).
  const mine = Object.values(all)
    .filter((s) => s.issueRef.startsWith(`issue:${head.id}@`))
    .sort((x, y) => x.createdAt.localeCompare(y.createdAt));
  const first = (me.displayName.split(" ")[0] ?? me.displayName).toLowerCase();
  const live = mine.map((s, n) => ({ s, who: `${first} · agent ${n + 1}` })).filter((x) => x.s.status !== "exited");
  if (live.length === 0) {
    const other = head.assignee !== null && head.assignee.id !== me.id
      && (head.state === "started" || head.state === "in_review");
    // A teammate's work: the person, no dot, no count (05 §4.4 rule 3). *Touching …* is W3.
    return other ? <p className="iv-agents-other">{head.assignee!.displayName} is working on this issue</p> : null;
  }
  return (
    <ul className="iv-agents" aria-label="Your agents on this issue">
      {live.map(({ s, who }) => (
        <li key={s.id} className="iv-agent">
          <AgentLine sessionId={s.id} whoLabel={who} />
          <button type="button" className="iv-link" onClick={() => openSession(s)}>open session</button>
        </li>
      ))}
    </ul>
  );
}
```

`AgentLine(props: { sessionId: string; whoLabel: string })` is task 40's complete prop list
(`tasks-21-40.md:687`): it draws `who.label` in mono, the meaning in sans from observed facts
only, one `AgentGlyph`, and coarsens liveness (`live` under 10 s, then `quiet 2m`) on the beat.
This part adds no liveness of its own. *open session* is a button styled as a link: the desktop
has no router (`task-36.md:41-43`), so "open" is three store actions, and nothing navigates away.

#### Interfaces — complete: `desktop/src/components/issue/pickers/StatePicker.tsx` (task 46's, as this task leaves it)

```tsx
import { useEffect, useRef, useState, type JSX } from "react";
import type { StateMove } from "../../../api/work";
import { mutate } from "../../../work/replica/ops";
import { StateGlyph, type IssueState } from "../../glyphs/StateGlyph";
import { DuplicateOfPicker } from "./DuplicateOfPicker";
import { Picker } from "./Picker";
import { stateOptions } from "./stateOptions";

export function handMove(to: IssueState, reason = ""): StateMove {
  return { to, cause: "hand", reason, landCommit: "", evidence: [], evidenceCommit: "" };
}

export function StatePicker(props: {
  issueId: string; state: IssueState; serving: boolean; onClose(): void;
}): JSX.Element {
  const { issueId, state, serving, onClose } = props;
  const [step, setStep] = useState<"list" | "duplicate" | "reason">("list");
  if (step === "duplicate") return <DuplicateOfPicker issueId={issueId} onClose={onClose} />;
  if (step === "reason") return <ReasonStep issueId={issueId} onBack={() => setStep("list")} onDone={onClose} />;
  const items = stateOptions(state, serving).map((o) => ({
    id: o.state, label: o.label, note: o.note, selected: o.current,
    lead: <StateGlyph state={o.state} size={14} />,
    disabled: o.kind === "fact",                 // needs_reason is now enabled: it opens the step
  }));
  const onChoose = (id: string): void | "stay" => {
    const opt = stateOptions(state, serving).find((o) => o.state === id);
    if (!opt || opt.current) return;
    if (opt.kind === "pick_duplicate") { setStep("duplicate"); return "stay"; }
    if (opt.kind === "needs_reason") { setStep("reason"); return "stay"; }
    mutate({ kind: "setState", issueId, move: handMove(opt.state) });
  };
  return <Picker label="Change state" items={items} onChoose={onChoose} onClose={onClose} />;
}

/** Decision 5: one line, trimmed, required. Enter sends; Esc goes back to the list. */
function ReasonStep(props: { issueId: string; onBack(): void; onDone(): void }): JSX.Element {
  const { issueId, onBack, onDone } = props;
  const [text, setText] = useState("");
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => input.current?.focus(), []);
  const reason = text.trim();
  const send = () => {
    if (reason === "") return;                   // a reasonless hand-done cannot be sent
    mutate({ kind: "setState", issueId, move: handMove("done", reason) });
    onDone();
  };
  return (
    <form className="iv-reason" role="dialog" aria-label="Done, not proven"
      onSubmit={(e) => { e.preventDefault(); send(); }}>
      <label className="iv-reason-label" htmlFor={`${issueId}-reason`}>
        This issue serves criteria and has no evidence of its own. Why is it done?
      </label>
      <input id={`${issueId}-reason`} ref={input} className="iv-reason-input" value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => { e.stopPropagation(); if (e.key === "Escape") { e.preventDefault(); onBack(); } }} />
      <button type="submit" disabled={reason === ""}>Mark done, not proven</button>
    </form>
  );
}
```

A choice that opens a step returns `"stay"` (task 46's `Picker` contract), so the popover stays
open and the step replaces the list. The input's `id` joins the `is-` id,
never the alias. Nothing in this step changes the glyph by itself: the replica's optimistic write
(task 56) sets `state: "done"`, and `proof: "not_proven"` arrives with the Hub's head, so until the
answer the row draws an ordinary done for a moment. That is the Hub deciding the word, as SW.4
says; a desktop that set `not_proven` itself would be computing the rule (a gap only if the owner
finds the moment visible: task 56's reconcile is one frame after the answer, **UNVERIFIED**).

#### Interfaces — complete: the edits to `IssueView.tsx` and `IssueHead.tsx`

```tsx
// IssueView.tsx (task 47's IssueViewBody), three additions and one replacement:
import { IssueAgentLine } from "./IssueAgentLine";
import { ProofLine, proofHeadOf } from "./ProofLine";
import { Relations } from "./Relations";
//   after <IssueHead … />:
      <ProofLine head={proofHeadOf(head)} />
      <IssueAgentLine head={head} me={me} />
//   in place of {slots.relations ?? null}:
      {slots.relations ?? <Relations head={head} me={me} />}

// IssueHead.tsx (task 46's), the state button's glyph:
import { glyphPropsOf } from "./glyphOf";
          <StateGlyph {...glyphPropsOf(head)} size={16} />

// ProofLine.tsx, beside ProofHead: the three gap fields read as absent until SW.15 has them.
export function proofHeadOf(head: IssueHead): ProofHead {
  const h = head as IssueHead & Partial<Pick<ProofHead, "stateMovedBy" | "stateMovedAt" | "proofCitations">>;
  return {
    state: h.state, proof: h.proof, landCommit: h.landCommit, stateReason: h.stateReason,
    stateMovedBy: h.stateMovedBy ?? null, stateMovedAt: h.stateMovedAt ?? null,
    proofCitations: h.proofCitations ?? [],
  };
}
```

`proofHeadOf` is the one place the three gap fields (decisions 6 and 7) are read; once task 37's
`IssueHead` carries them, its cast becomes a no-op and the typecheck still passes.

#### Interfaces — complete: appended to `desktop/src/components/issue/issue.css`

```css
/* Task 48: relations, agents, the proof line, the reason step. */
.iv-rels, .iv-agents-wrap { display: grid; gap: var(--s-2); padding: var(--s-3) var(--s-4); }
.iv-rel-list, .iv-agents { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--s-1); }
.iv-rel { display: flex; flex-wrap: wrap; align-items: center; gap: var(--s-2); min-block-size: var(--row-h); }
.iv-rel-word { font-family: var(--sans); color: var(--ink-2); min-inline-size: 9ch; }
.iv-rel-title { font-family: var(--sans); color: var(--ink); }
.iv-rel-owner { font-family: var(--sans); color: var(--ink-2); }
.iv-rel-remove, .iv-rel-add { font: inherit; font-family: var(--sans); }
.iv-agent { display: flex; align-items: center; gap: var(--s-2); }
.iv-agents-other { margin: 0 var(--s-4); font-family: var(--sans); color: var(--ink); }
.iv-proof { margin: 0 var(--s-4); font-family: var(--sans); color: var(--ink); }
.iv-proof-because, .iv-reason-text { font-family: var(--sans); }
.iv-reason { display: grid; gap: var(--s-2); padding: var(--s-3); min-inline-size: 320px;
  border-radius: var(--r-md); background: var(--surface-2); box-shadow: 0 0 0 1px var(--ring), var(--shadow-2); }
.iv-reason-label { font-family: var(--sans); color: var(--ink); }
.iv-reason-input { font: inherit; font-family: var(--sans); padding: var(--s-1) var(--s-2);
  border: 0; border-radius: var(--r-sm); background: var(--page); color: var(--ink);
  box-shadow: inset 0 0 0 1px var(--ring); }
.iv-reason-input:focus-visible { outline: 2px solid var(--accent-ring); outline-offset: 1px; }
```

No rule here names `--need`, `--fail` or a status hue: *done, not proven* is a shape (task 28's
open ring) and a sentence, and the stalled line is a fact in mono, not an alarm. The Desk (task
63) is where a wait earns amber.

#### Tests to write first — `pickers/reason.test.tsx` and `glyphOf.test.ts`

```tsx
// desktop/src/components/issue/pickers/reason.test.tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WORK_IDS } from "../../../test/workFixtures";
import { mutate } from "../../../work/replica/ops";
import { StatePicker, handMove } from "./StatePicker";

vi.mock("../../../work/replica/ops", () => ({ mutate: vi.fn() }));
vi.mock("../../../work/replica/replica", () => ({ useWork: vi.fn(() => []), search: vi.fn(() => []) }));
const sent = () => vi.mocked(mutate).mock.calls.map((c) => c[0]);
beforeEach(() => vi.mocked(mutate).mockClear());

describe("the hand-done reason step", () => {
  it("cannot send a hand-done on a serving issue without a reason", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="landed" serving onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Done/ }));
    const box = screen.getByRole("textbox");
    await user.type(box, "   {Enter}");
    expect(sent()).toEqual([]);
    expect((screen.getByRole("button", { name: "Mark done, not proven" }) as HTMLButtonElement).disabled).toBe(true);
  });
  it("sends the trimmed reason with cause hand", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="landed" serving onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Done/ }));
    await user.type(screen.getByRole("textbox"), "  shipped behind a flag  {Enter}");
    expect(sent()).toEqual([{ kind: "setState", issueId: WORK_IDS.ep142, move: handMove("done", "shipped behind a flag") }]);
  });
  it("sends a plain done, with no step, on an issue that serves nothing", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="landed" serving={false} onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Done/ }));
    expect(sent()).toEqual([{ kind: "setState", issueId: WORK_IDS.ep142, move: handMove("done") }]);
  });
});
```

```ts
// desktop/src/components/issue/glyphOf.test.ts
import { describe, expect, it } from "vitest";
import { makeIssueHead } from "../../test/workFixtures";
import { glyphPropsOf } from "./glyphOf";
import headSrc from "./IssueHead.tsx?raw";
import rowSrc from "../work/IssueRow.tsx?raw";
import boardSrc from "../work/BoardView.tsx?raw";

describe("glyphPropsOf (decision 8)", () => {
  it("draws not proven from the Hub's word, on done only", () => {
    expect(glyphPropsOf(makeIssueHead({ state: "done", proof: "not_proven" })).notProven).toBe(true);
    expect(glyphPropsOf(makeIssueHead({ state: "done", proof: "proven" })).notProven).toBe(false);
    expect(glyphPropsOf(makeIssueHead({ state: "done", proof: "proven_self_attested" })).notProven).toBe(false);
    expect(glyphPropsOf(makeIssueHead({ state: "todo", proof: "not_proven" })).notProven).toBe(false);
  });
  it("is the one rule in the view, the row and the board card", () => {
    for (const src of [headSrc, rowSrc, boardSrc]) {
      expect(src).toContain("glyphPropsOf(");
      expect(src).not.toMatch(/notProven=\{/);          // nobody passes the flag by hand
    }
  });
});
```

| Test | Mutant it kills |
|---|---|
| cannot send without a reason | a done without reason, by Enter or by the button |
| sends the trimmed reason | an untrimmed reason; a cause other than `hand` |
| plain done when serving nothing | a reason demanded where the Hub wants none (`no_criteria`) |
| not proven from the word | an ordinary done glyph on a hand-set done; *proven, self-attested* drawn as not proven |
| one rule in three places | the row or the board card drawing an ordinary done glyph |


