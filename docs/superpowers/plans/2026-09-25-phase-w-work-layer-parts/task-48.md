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
          onChoose={(k) => setAdding(k as AddKind)} onClose={() => {}} />
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
      onChoose={(other) => mutate(relationOp(subject, kind, other))} />
  );
}
```

`r.kind as RelationWord` is safe because task 11 widens `RelationKind` to the five words, and
SW.15's desktop `Relation.kind` mirrors it (task 37); task 20's fixture type lists only the
three writable words (`task-20.md:178`), so the fixtures need the two inverse words too (a gap).
The first `Picker`'s `onClose` does nothing because its single choice immediately swaps it for the
second; the second closes the whole add. `useHeartbeat` is the one clock (`heartbeat.ts:64`, a
1 Hz tick shared by the window): no component in this part starts a timer, so a *waiting* figure
changes only on the beat.

