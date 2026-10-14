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

#### Tests to write first — `desktop/src/components/issue/ProofLine.test.tsx`

```tsx
import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { ALICE, makeIssueHead } from "../../test/workFixtures";
import { ProofLine, proofHeadOf, type ProofCitation, type ProofHead } from "./ProofLine";

const LAND = "7c1f0a2" + "0".repeat(33);
const cite = (over: Partial<ProofCitation> = {}): ProofCitation => ({
  criterionId: "c-91e2a4b0", cause: "evidence_observed", commit: LAND,
  because: "test -f resume.ok, exit 0, run by the platform at 14:02", recordedAt: "2026-09-25T04:02:00Z", ...over,
});
const at = (over: Partial<ProofHead>): ProofHead => ({
  ...proofHeadOf(makeIssueHead({ state: "done", landCommit: LAND })), proofCitations: [cite()], ...over,
});
const text = (h: ProofHead) => render(<ProofLine head={h} />).container.textContent ?? "";

describe("ProofLine (SW.4's words, rendered)", () => {
  it("renders observed evidence with its commit and exit code, and never 'not proven'", () => {
    const t = text(at({ proof: "proven" }));
    expect(t).toContain("own evidence at 7c1f0a2");
    expect(t).toContain("exit 0");
    expect(t).not.toMatch(/not proven/);
  });
  it("says self-attested when the Hub says so", () => {
    expect(text(at({ proof: "proven_self_attested", proofCitations: [cite({ cause: "evidence_recorded" })] })))
      .toContain("proven, self-attested");
  });
  it("shows a hand-set done's reason, its author and its time", () => {
    const t = text(at({ proof: "not_proven", stateReason: "shipped behind a flag, 80% of users",
      stateMovedBy: ALICE, stateMovedAt: "2026-09-25T01:02:00Z", proofCitations: [] }));
    expect(t).toMatch(/^done, not proven · shipped behind a flag, 80% of users · Alice Example, /);
  });
  it("says set by hand when the Hub has not said who", () => {
    expect(text(at({ proof: "not_proven", stateReason: "r", proofCitations: [] }))).toMatch(/set by hand$/);
  });
  it("draws nothing before landed", () => {
    expect(text(at({ proof: "", state: "started" }))).toBe("");
  });
  it("makes no prediction in any of the six words, a verbatim 80% included", () => {
    for (const proof of ["proven", "proven_self_attested", "not_proven", "no_criteria", "waiting"] as const) {
      const { container } = render(<ProofLine head={at({ proof, stateReason: "80% of users" })} />);
      expectNoPrediction(container);
    }
  });
});
```

| Test | Mutant it kills |
|---|---|
| observed evidence with commit and exit code | conflating observed evidence with *not proven*; a line with no commit |
| self-attested says so | hiding *self-attested* behind a plain *proven* |
| reason, author, time | the mark without its reason (task 39's mutant, now in the product) |
| set by hand | an invented author when the Hub gave none |
| nothing before landed | a proof line on a started issue (a prediction by another name) |
| no prediction | a `%` outside the verbatim reason, a bar or an ETA on the line |

#### Tests to write first — `desktop/src/components/issue/Relations.test.tsx`

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { api } from "../../api";
import { useCockpit } from "../../store/store";
import { FIXED_NOW, aSession } from "../../test/fixtures";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead } from "../../test/workFixtures";
import { mutate } from "../../work/replica/ops";
import { search } from "../../work/replica/replica";
import { useIssueSummaries } from "./issueData";
import { Relations, relationOp } from "./Relations";

// Every session verb CockpitClient has (desktop/src/api/client.ts, the interface as read on
// 2026-09-25); none may be called by a relation, whoever owns the other issue.
const SESSION_VERBS = ["createSession", "renameSession", "restartSession", "killSession",
  "purgeSession", "resumeSession", "forkSession", "openAttach"] as const;
vi.mock("../../api", () => ({ api: Object.fromEntries(
  ["createSession", "renameSession", "restartSession", "killSession", "purgeSession",
   "resumeSession", "forkSession", "openAttach"].map((k) => [k, vi.fn()])) }));
vi.mock("../../hooks/heartbeat", () => ({ useHeartbeat: () => FIXED_NOW }));
vi.mock("../../work/replica/ops", () => ({ mutate: vi.fn() }));
vi.mock("../../work/replica/replica", () => ({ useWork: (f: (r: unknown) => unknown) => f({}), search: vi.fn() }));
vi.mock("./issueData", () => ({ useIssueSummaries: vi.fn(() => new Map()) }));

const h = (id: string, over = {}) => ({ ...makeIssueHead({ id, ...over }), reassignRequest: null } as never);
beforeEach(() => { vi.mocked(mutate).mockClear(); useCockpit.setState({ sessions: {} }); });

describe("Relations", () => {
  it("writes blocked-by as the other issue blocking this one, and each end reads its word", () => {
    expect(relationOp(WORK_IDS.ep142, "blocked_by", WORK_IDS.ep150)).toEqual(
      { kind: "relation", issueId: WORK_IDS.ep150, add: { kind: "blocks", issueId: WORK_IDS.ep142 } });
    const { unmount } = render(<Relations me={DYLAN}
      head={h(WORK_IDS.ep150, { relations: [{ kind: "blocks", issueId: WORK_IDS.ep142 }] })} />);
    expect(screen.getByText("blocking")).toBeTruthy();
    unmount();
    render(<Relations me={DYLAN} head={h(WORK_IDS.ep142, { relations: [{ kind: "blocked_by", issueId: WORK_IDS.ep150 }] })} />);
    expect(screen.getByText("blocked by")).toBeTruthy();
  });
  it("sends one relation op to an issue another person owns, and no act at any session", async () => {
    const user = userEvent.setup();
    vi.mocked(search).mockReturnValue([{ id: WORK_IDS.ep150, alias: "EP-150", title: "Alice's issue" }] as never);
    vi.mocked(useIssueSummaries).mockReturnValue(new Map([[WORK_IDS.ep150,
      { head: h(WORK_IDS.ep150, { assignee: ALICE, state: "started" }), title: "Alice's issue" }]]));
    render(<Relations me={DYLAN} head={h(WORK_IDS.ep142)} />);
    await user.click(screen.getByRole("button", { name: "Add a relation" }));
    await user.click(screen.getByRole("option", { name: "related" }));
    await user.click(screen.getByRole("option", { name: /EP-150/ }));
    expect(vi.mocked(mutate).mock.calls.map((c) => c[0])).toEqual([
      { kind: "relation", issueId: WORK_IDS.ep142, add: { kind: "relates", issueId: WORK_IDS.ep150 } }]);
    const verbs = api as unknown as Record<(typeof SESSION_VERBS)[number], ReturnType<typeof vi.fn>>;
    for (const k of SESSION_VERBS) expect(verbs[k]).not.toHaveBeenCalled();
  });
  it("shows my idle agent on the issue this one blocks, with the wait in mono", () => {
    useCockpit.setState({ sessions: { "s-1": aSession({ id: "s-1", status: "idle",
      issueRef: `issue:${WORK_IDS.ep142}@3`, lastOutputAt: new Date(FIXED_NOW - 42 * 60_000).toISOString() }) } as never });
    render(<Relations me={DYLAN} head={h(WORK_IDS.ep150, { state: "todo", relations: [{ kind: "blocks", issueId: WORK_IDS.ep142 }] })} />);
    const line = screen.getByText("blocking dylan · agent 1 · waiting 42m");
    expect(line.className).toContain("iv-fact");
  });
  it("shows no stall once the blocker has landed", () => {
    useCockpit.setState({ sessions: { "s-1": aSession({ id: "s-1", status: "idle", issueRef: `issue:${WORK_IDS.ep142}@3` }) } as never });
    render(<Relations me={DYLAN} head={h(WORK_IDS.ep150, { state: "landed", relations: [{ kind: "blocks", issueId: WORK_IDS.ep142 }] })} />);
    expect(screen.queryByText(/blocking dylan/)).toBeNull();
  });
});
```

The mock factory repeats the verb list because a `vi.mock` factory is hoisted above the module's
constants and cannot read `SESSION_VERBS` (the rule `AppShell.test.tsx:31` already notes). Step 1
re-reads `CockpitClient` on the sync base and adds any session verb task 37 or later has added.

#### Tests to write first — `desktop/src/components/issue/IssueAgentLine.test.tsx`

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { useCockpit } from "../../store/store";
import { aSession } from "../../test/fixtures";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead } from "../../test/workFixtures";
import { IssueAgentLine } from "./IssueAgentLine";
import agentSrc from "./IssueAgentLine.tsx?raw";
import relSrc from "./Relations.tsx?raw";
import proofSrc from "./ProofLine.tsx?raw";
import pickerSrc from "./pickers/StatePicker.tsx?raw";

vi.mock("../work/AgentLine", () => ({
  AgentLine: (p: { sessionId: string; whoLabel: string }) => <span data-testid="agent-line">{p.whoLabel}</span>,
}));

const head = (over = {}) => ({ ...makeIssueHead(over), reassignRequest: null } as never);
const ref = (n: number) => `issue:${WORK_IDS.ep142}@${n}`;
beforeEach(() => useCockpit.setState({ sessions: {} }));

describe("IssueAgentLine", () => {
  it("draws one line per own live agent, numbered by start, exited ones keeping their number", () => {
    useCockpit.setState({ sessions: {
      a: aSession({ id: "a", createdAt: "2026-09-25T01:00:00Z", issueRef: ref(2), status: "exited" }),
      b: aSession({ id: "b", createdAt: "2026-09-25T01:10:00Z", issueRef: ref(3), status: "running" }),
      c: aSession({ id: "c", createdAt: "2026-09-25T01:20:00Z", issueRef: `issue:${WORK_IDS.ep150}@1` }),
    } } as never);
    render(<IssueAgentLine head={head({ assignee: DYLAN, state: "started" })} me={DYLAN} />);
    expect(screen.getAllByTestId("agent-line").map((e) => e.textContent)).toEqual(["dylan · agent 2"]);
    expect(screen.getByRole("button", { name: "open session" })).toBeTruthy();
  });
  it("shows a teammate's work as the person, with no dot and no count", () => {
    const { container } = render(<IssueAgentLine head={head({ assignee: ALICE, state: "started" })} me={DYLAN} />);
    expect(container.textContent).toBe("Alice Example is working on this issue");
    expect(container.querySelector("[data-glyph^='agent:']")).toBeNull();
    expect(container.textContent).not.toMatch(/\d/);
  });
  it("derives nothing from the terminal: no attach registry, no xterm, no transcript", () => {
    expect(agentSrc).not.toMatch(/attachRegistry|Terminal|xterm|transcript|openAttach/);
  });
  it("starts no timer of its own anywhere in this part", () => {
    for (const src of [agentSrc, relSrc, proofSrc, pickerSrc]) {
      expect(src).not.toMatch(/setInterval|setTimeout|requestAnimationFrame/);
    }
  });
});
```

| Test | Mutant it kills |
|---|---|
| one line per own live agent | a dot per agent, a count, or an agent renumbered when an earlier one exits |
| a teammate's work | fleet dots or `+2` for another person's agents (05 §4.4 rule 3) |
| nothing from the terminal | PTY-derived meaning (`OD-8`) |
| no timer of its own | a per-row clock (one heartbeat, `DESIGN-SYSTEM.md`) |

`expectNoPrediction` runs in `ProofLine.test.tsx` over all six words; the view as a whole is run
through it by task 46's `IssueHead.test.tsx`, which, after this task's `IssueView` edit, renders
the proof line, the agent line and the relations too. Step 6 re-runs that file.

#### Steps

- [ ] **Step 1: read on the sync base** task 40's `AgentLine.tsx` and `IssueRow.tsx`, task 43's
  `BoardView.tsx` (the card's `StateGlyph` call), task 37's `IssueHead` (does it carry
  `stateMovedBy`, `stateMovedAt`, `proofCitations`, the five relation words?), task 56's `WorkOp`
  (`relation`, `setState`), and `CockpitClient`'s session verbs. Rename in this part's files only.
- [ ] **Step 2: the six test files first**; `npm --prefix desktop test -- issue` fails on the
  missing modules and on task 46's `StatePicker` still refusing `done`.
- [ ] **Step 3: `glyphOf.ts`**, and the one-line edits in `IssueHead.tsx`, `IssueRow.tsx` and
  `BoardView.tsx`; `glyphOf.test.ts` passes.
- [ ] **Step 4: `ProofLine.tsx`**, then the reason step in `StatePicker.tsx`.
- [ ] **Step 5: the `issueData.ts` append, `Relations.tsx`, `IssueAgentLine.tsx`**, the
  `IssueView.tsx` edits and the `issue.css` append.
- [ ] **Step 6: run the acceptance commands**; tasks 46 and 47's tests stay green. Commit by path,
  one commit per step 3-5, each message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- issue work/IssueRow work/BoardView
# expect: every file passes; this part adds 5 files and 19 tests
#   (reason 3, glyphOf 2, ProofLine 6, Relations 4, IssueAgentLine 4)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rnE "setInterval|setTimeout" desktop/src/components/issue/ | grep -v test | wc -l
# expect: 0
```

Tasks 40, 43, 46 and 47's files run in the same command and must stay green. Visual acceptance
is the owner's eyes.

#### What this task does NOT claim

- Sub-issues (`L-20`), the W3 negotiation line (§4.1), the *touching …* line for a teammate (W3,
  needs Fleet).
- Land order from `blocks` (phase 3's hint), and a resolved blocker becoming *related* (`L-25`, W2).
- That an agent is stalled **because of** its blocker: the line puts two facts side by side.
- The Desk row with the cost of the wait (task 63), or the other owner's *inform* (task 65).
- That the Hub keeps one record for both ends (task 11's `TestTheSameRelationTwiceIsOneRowAndOneEvent`
  and its inverse-word test hold it), or that the replica refreshes the other end (gap 1).
- The evidence detail on a real Hub: `proofCitations` is not on the wire yet (gap 4).
- *done · c-91e2 still open (EP-150 not landed)*: that phrase needs the other issues serving the
  criterion, a replica read no W1 task names (gap 5); the `serves` row's convergence word (task 47)
  says *open* meanwhile.

**Parity** (`05-design.md` rows). Closes **L-24**'s W1 part (`05:133`: the kinds and the
stalled-agent fact), **L-10**'s relation half (`05:119`, with task 46's duplicate-of step),
**L-39**'s W1 part (`05:148`: the only agents shown are the reader's own, started by the reader)
and, with task 46, **L-181** (`05:348`).

#### Gaps found while writing this part

1. **The replica must re-fetch `otherIssueId` on `issue.related` and `issue.unrelated`**
   (`task-13.md:166-170`); task 55's scope re-fetches the subject only, so the other end would
   read a stale relation list until its own next event.
2. **Tasks 40 and 43 are not in this task's dependsOn**, but this part edits `IssueRow.tsx` and
   `BoardView.tsx` (decision 8); the skeleton row for 48 should read `46, 40, 43`.
3. **`IssueHead` has no author or time for a state move** (`stateMovedBy`, `stateMovedAt`), which
   the scope's *"the reason, its author and time"* needs; a gap for tasks 10 and 37.
4. **No W1 read returns an issue's cited evidence.** `proofCitations` on the head (or a
   `GET /v1/issues/{id}/proof`) is a gap for task 10, whose `issue_proof_citations` already holds it.
5. **No replica read lists the issues serving a criterion**, which *"c-91e2 still open (EP-150 not
   landed)"* needs; task 40's groups need the same list, so it is one read for tasks 55-57.
6. **Task 20's `Relation.kind` fixture type** has the three writable words only
   (`task-20.md:178`); `blocked_by` and `duplicated_by` must be added for these tests.
7. The File map's `src/components/issue/` row lists neither `ProofLine.tsx` nor `glyphOf.ts`.


