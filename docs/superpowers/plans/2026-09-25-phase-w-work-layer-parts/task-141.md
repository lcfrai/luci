### Task 141: an agent's edit to an issue or brief is a proposal, shown as a diff

W4, desktop (plus one Go test), about 2.5 hours (an estimate, not a measurement). Depends on task
47 (`lineDiff` and `DiffLine` in `desktop/src/components/issue/revisionWords.ts`,
`task-47.md:151-172`) and task 136 (the assists that file proposals; **scope only, no part on this
tree**, so its names are **UNVERIFIED**). Reads task 46's `IssueView` and its `slots`
(`task-46.md:520-527`), task 49's `authorClass.ts` (`task-49.md:101-129`, decision 1: a field is
`A` while any agent span remains), task 51's `Comments` (`task-51.md:466-526`), task 56's `revise`
and `resolve` ops (`task-56.md:157-168`), task 37's `IssueRevisionContent` and `IssueComment`
(`task-37.md:111-159`), task 19's SW.7 golden (`task-19.md:55-56`), and phase 4's `4-D2` and
`Answer.Author` (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md:143`,
`…-parts/task-03.md:83-91`) with P4 task 18's `BriefPanel.tsx` (plan `:333`, **UNVERIFIED until
merge**, rule H2).

Closes **L-130** (05 §1 group I: *"An agent's edit to a brief or issue is a **proposal**
(`text·A`), shown as a diff, that a person accepts"*). Serves L-124 (closed in W1) and L-78 (task
139's drafted updates stay `text·A` until posted).

**The rule it rests on.** 05 §3.6: *"an agent writes only comments on its own owner's behalf"*; and
`4-D2`: *"A model-written answer is not inherited until a person accepts or changes it."* So an
agent never writes an issue revision. Its edit is an `ic-` comment of kind **`edit_proposal`**,
`text·A`, whose body is an `EditProposal`. That kind is in neither SW.19 (task 99: `note`,
`evidence_proposal`, `negotiation`) nor SW.20 (task 114, checked by grep on this tree: no
`edit_proposal`), so it is **a gap to close in task 114** (the kind constant, the body's
`Validate`, the fake's rule and the migration's kind `CHECK`), written below as complete source
for that owner and **not** applied by this task.

**What the reader sees** (05 §3.4, §3.6, §4.4; 04 §4.4 `:338-351`; AX Principle 7):

- **A strip above the issue's head fields** when an open proposal exists: *"Proposed edit · dylan
  · agent 1 · 11:02 · on @3"* with *Review*. The head fields still show @3: nothing is applied on
  arrival.
- **Review opens `ProposalDiff` in place**: title and description as a word diff, removed words
  struck through **and** announced as *removed*, added words underlined **and** announced as
  *added*, never colour alone. Serves changes read as words: *"adds C4 · removes C2"*. The head
  line reads *Proposed by dylan · agent 1 (claude-sonnet, session s-7f2c) at 11:02, on @3*, with
  ids and times in Commit Mono (`OD-28`), and each `why` key as a link.
- **Three acts, in place** (`confirm.inplace`): **Accept** writes one new revision as the person,
  the accepted texts still marked `text·A`; **Edit** opens the one editor on the proposed text;
  **Decline** writes nothing to the issue and resolves the proposal comment.
- **Stale**: when the head has moved past the proposal's base, the strip reads *"Stale: EP-142
  moved to @5 (Alice, 11:04) since this was proposed on @3"*, the diff is shown against @5, and
  Accept stays disabled until the person presses *I have read the changes against @5*.
- **In the thread** an `edit_proposal` shows as one line, *"Proposed an edit · Review"*, a link to
  the strip. Its body is never rendered as Markdown.
- **In Discover**, a `model_drafted` answer that replaces an existing answer shows as the same
  diff; Accept sends `model_accepted`, Edit sends `person`.

**Decisions made here, so nobody re-derives them:**

1. **`lineDiff` is not lifted.** It already lives in `revisionWords.ts`, outside `Revisions.tsx`
   (`task-47.md:56`, `:154`), so `wordDiff.ts` imports it unchanged and adds a word level inside
   each changed pair of lines. The word level uses the same longest-common-subsequence method as
   `lineDiff` (not Myers): one method, one output shape, no dependency.
2. **The proposer is carried in the body.** The comment's `createdBy` is the owner (the agent
   writes on its owner's behalf); `EditProposal.proposer {agentId, sessionId, model}` names the
   agent. The strip's *agent 1* is task 47's `agentsWhoRead` numbering over the reader's own
   sessions when the reader is the owner, and `agent · s-7f2c` otherwise (nobody else's agent is
   numbered as if it were yours).
3. **Author per text, never per revision** (L-124). Accept writes each proposed text with `author:
   "A"`. After Edit, each field's class is task 49's `fieldAuthor` of what the person committed: a
   title the person rewrote in full is `H`; a title with any drafted words left is still `A`; a
   field the person did not open keeps the proposal's `A`. The person is the revision's writer in
   every case.
4. **Accept always writes against the head it shows.** The `revise` op's `baseRev` is the head
   revision the diff was computed against, so a head that moves again during review is task 56's
   ordinary `409` and a fresh stale line, never an overwrite.
5. **Decline resolves; nothing is deleted.** Comments are append-only (task 12). Decline is task
   56's `resolve` op on the proposal comment. A resolved proposal leaves the strip.
6. **One strip, newest open proposal first.** With several open proposals the strip names the
   newest and *"2 more proposals"* as a link to the thread; each is reviewed one at a time.
7. **Proposals never reach an agent.** The Materialiser renders the issue revision, never
   comments (05 §3.4; skeleton rule 6). A Go test pins that an open `edit_proposal` leaves SW.7's
   managed block byte-identical.

**Files:**

- `desktop/src/work/diff/wordDiff.ts` — `wordDiff`, `textDiff`, `servesDiff` (new)
- `desktop/src/work/diff/wordDiff.test.ts` — the diff tests (new)
- `desktop/src/components/proposal/ProposalDiff.tsx` — the diff and its head line (new)
- `desktop/src/components/proposal/ProposalStrip.tsx` — the strip, the three acts, stale (new)
- `desktop/src/components/proposal/agentLabel.ts` — decision 2's label (new; not in the scope's list)
- `desktop/src/components/proposal/proposal.css` — their styles (new)
- `desktop/src/components/proposal/proposal.test.tsx` — the surface tests (new)
- `desktop/src/api/work.ts` ⚑ — `EditProposal`, `ProposerRef`, `IssueComment.kind` widened (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) — the strip above the head fields (modified)
- `desktop/src/components/issue/Comments.tsx` ⚑ (task 51) — an `edit_proposal` renders as a link line (modified)
- `desktop/src/components/discover/BriefPanel.tsx` ⚑ (P4 task 18, **UNVERIFIED until merge**) — a replacing `model_drafted` answer as `ProposalDiff` (modified)
- `control-plane/internal/claudemd/render_test.go` ⚑ (task 19) — the byte-identical row (modified)

#### The wire — proposed for task 114's `hubapi` (complete; **not applied by this task**)

```go
// CommentKindEditProposal is an agent's edit to an issue, filed as a comment
// on its owner's behalf (05 §3.6; L-130). Body.Author is always "A"; Body.Text
// is EditProposal's canonical JSON.
const CommentKindEditProposal = "edit_proposal"

// ProposerRef names the agent that proposed an edit. The comment's CreatedBy
// is its owner; the agent never appears as a writer.
type ProposerRef struct {
	AgentID   string `json:"agentId"`   // the harness agent id the session carries
	SessionID string `json:"sessionId"` // s-…, the control plane's session id
	Model     string `json:"model"`     // the public model id, e.g. claude-sonnet-…
}

// EditProposal is an edit an agent proposes. At least one of Title,
// Description, ServesAdd or ServesRemove is set; Why keys resolve via /v1/refs.
type EditProposal struct {
	BaseRevision int           `json:"baseRevision"` // ≥ 1, the revision the agent read
	Title        *AuthoredText `json:"title"`        // author "A"; null when unchanged
	Description  *AuthoredText `json:"description"`  // author "A"; null when unchanged
	ServesAdd    []Serve       `json:"servesAdd"`    // never null
	ServesRemove []Serve       `json:"servesRemove"` // never null
	Why          []string      `json:"why"`          // Hub keys, 1-10, each resolvable
	Proposer     ProposerRef   `json:"proposer"`
}

var ErrEditProposalEmpty = errors.New("hubapi: an edit proposal changes nothing")

// Validate is the Hub's check before the comment is stored (task 114 owns the call site).
func (p EditProposal) Validate() error {
	if p.BaseRevision < 1 {
		return errors.New("hubapi: edit proposal baseRevision must be at least 1")
	}
	if p.Title == nil && p.Description == nil && len(p.ServesAdd) == 0 && len(p.ServesRemove) == 0 {
		return ErrEditProposalEmpty
	}
	for _, t := range []*AuthoredText{p.Title, p.Description} {
		if t != nil && t.Author != AuthorAgent {
			return errors.New("hubapi: proposed text must be author A")
		}
	}
	if len(p.Why) == 0 || len(p.Why) > 10 {
		return errors.New("hubapi: edit proposal needs 1-10 why keys")
	}
	if p.Proposer.SessionID == "" || p.Proposer.Model == "" {
		return errors.New("hubapi: edit proposal must name its session and model")
	}
	return nil
}
```

The rest of the 114 gap, in words: `Comment.Kind` accepts `edit_proposal`; a comment of that kind
must carry `Body.Author == "A"` and a `Body.Text` that decodes to a valid `EditProposal` under
`DisallowUnknownFields`; the fake (task 115) and the migration's `work_comments.kind` `CHECK` (task
116) list it; the writer is the owner's principal on an agent's behalf (05 §3.6), so the
authorisation rows are task 99's for `note`, reused. Whether a `why` key resolves is checked with
the Hub's `/v1/refs` at write (L-52's rule).

#### Interfaces — complete: `desktop/src/api/work.ts` ⚑, appended

```ts
// Task 141. Mirrors the proposed hubapi.EditProposal (task 114's gap).
export interface ProposerRef { agentId: string; sessionId: string; model: string }
export interface EditProposal {
  baseRevision: number; title: AuthoredText | null; description: AuthoredText | null;
  servesAdd: Serve[]; servesRemove: Serve[]; why: string[]; proposer: ProposerRef;
}
/** IssueComment.kind widens from task 37's "comment" (task-37.md:155) to the SW.19 set plus this. */
export type CommentKind = "comment" | "note" | "evidence_proposal" | "negotiation" | "edit_proposal";

/** The proposal inside an edit_proposal comment, or null for anything that is not one. */
export function editProposalOf(c: IssueComment): EditProposal | null {
  if (c.kind !== "edit_proposal" || c.body.author !== "A" || c.erasedAt !== null) return null;
  try {
    const p = JSON.parse(c.body.text) as EditProposal;
    return typeof p.baseRevision === "number" && Array.isArray(p.why) ? p : null;
  } catch { return null; }
}
```

#### Interfaces — complete: `desktop/src/work/diff/wordDiff.ts`

```ts
// desktop/src/work/diff/wordDiff.ts — decision 1. Lines by task 47's lineDiff; words inside a
// changed pair by the same LCS. Our own code; no dependency.
import { lineDiff, type DiffLine } from "../../components/issue/revisionWords";
import type { Serve } from "../../api/work";

export interface WordPart { op: "same" | "add" | "del"; text: string }
/** A line of the rendered diff: unchanged, wholly added or removed, or changed word by word. */
export type DiffRow =
  | { kind: "same"; text: string }
  | { kind: "add" | "del"; text: string }
  | { kind: "changed"; parts: WordPart[] };

/** Words and the spaces after them are tokens, so joining the parts gives the text back. */
const tokens = (s: string): string[] => s.match(/\S+\s*|\s+/g) ?? [];

export function wordDiff(from: string, to: string): WordPart[] {
  const a = tokens(from), b = tokens(to);
  const L: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--)
    for (let j = b.length - 1; j >= 0; j--)
      L[i][j] = a[i] === b[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  const out: WordPart[] = [];
  const push = (op: WordPart["op"], text: string) => {
    const last = out[out.length - 1];
    if (last && last.op === op) last.text += text; else out.push({ op, text });
  };
  let i = 0, j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) { push("same", a[i]); i++; j++; }
    else if (L[i + 1][j] >= L[i][j + 1]) push("del", a[i++]);
    else push("add", b[j++]);
  }
  while (i < a.length) push("del", a[i++]);
  while (j < b.length) push("add", b[j++]);
  return out;
}

/** lineDiff's output, unchanged, with each adjacent del+add pair refined to words. */
export function textDiff(from: string, to: string): DiffRow[] {
  const lines: DiffLine[] = lineDiff(from, to);
  const rows: DiffRow[] = [];
  for (let k = 0; k < lines.length; k++) {
    const l = lines[k], next = lines[k + 1];
    if (l.op === "del" && next?.op === "add") { rows.push({ kind: "changed", parts: wordDiff(l.text, next.text) }); k++; }
    else if (l.op === "same") rows.push({ kind: "same", text: l.text });
    else rows.push({ kind: l.op, text: l.text });
  }
  return rows;
}

/** Serves changes as criterion ids, the words task 47's changeWords uses (task-47.md:140-148). */
export function servesDiff(add: readonly Serve[], remove: readonly Serve[]): string {
  const w: string[] = [];
  if (add.length > 0) w.push(`adds ${add.map((s) => s.criterionId).join(", ")}`);
  if (remove.length > 0) w.push(`removes ${remove.map((s) => s.criterionId).join(", ")}`);
  return w.join(" · ");
}
```

Joining a row's parts, `same` and `del` give the old line and `same` and `add` the new one, byte
for byte (a test pins it), so the diff never shows words the texts do not hold. `lineDiff` is
imported, not copied, so task 47's output and this one cannot drift.

#### Interfaces — complete: `desktop/src/components/proposal/ProposalDiff.tsx`

```tsx
// desktop/src/components/proposal/ProposalDiff.tsx — L-130. Shape and word, never colour alone.
import type { ReactNode } from "react";
import { textDiff, type DiffRow } from "../../work/diff/wordDiff";
import { KeyLink } from "../common/KeyLink";

export interface ProposalAuthor {
  owner: string;          // "dylan"
  agent: string;          // "agent 1" (decision 2) or "agent · s-7f2c"
  model: string;          // "claude-sonnet-…"
  sessionId: string;      // "s-7f2c…"
  at: string;             // RFC 3339
  onRevision: number | null; // 3 for an issue; null for a brief answer
}

export interface ProposalDiffProps {
  fields: readonly { label: string; base: string; proposed: string }[];   // Title, Description; or the answer
  extra?: ReactNode;                     // the serves line for an issue
  why: readonly string[];
  author: ProposalAuthor;
  timeZone: string;
}

function Row({ r }: { r: DiffRow }): JSX.Element {
  if (r.kind === "same") return <div className="pd-line">{r.text}</div>;
  if (r.kind === "add") return <div className="pd-line"><ins className="pd-add" aria-label="added">{r.text}</ins></div>;
  if (r.kind === "del") return <div className="pd-line"><del className="pd-del" aria-label="removed">{r.text}</del></div>;
  return (
    <div className="pd-line">
      {r.parts.map((p, i) => p.op === "same" ? <span key={i}>{p.text}</span>
        : p.op === "add" ? <ins key={i} className="pd-add" aria-label="added">{p.text}</ins>
        : <del key={i} className="pd-del" aria-label="removed">{p.text}</del>)}
    </div>
  );
}

export function ProposalDiff(p: ProposalDiffProps): JSX.Element {
  const a = p.author;
  const time = new Intl.DateTimeFormat("en-AU", { hour: "2-digit", minute: "2-digit", timeZone: p.timeZone }).format(new Date(a.at));
  return (
    <section className="pd" aria-label="Proposed edit" data-author="A">
      <p className="pd-head">
        Proposed by {a.owner} · {a.agent} (<span className="pd-mono">{a.model}</span>, session{" "}
        <span className="pd-mono">{a.sessionId}</span>) at <time className="pd-mono" dateTime={a.at}>{time}</time>
        {a.onRevision !== null && <>, on <span className="pd-mono">@{a.onRevision}</span></>}
      </p>
      {p.fields.map((f) => (
        <div key={f.label} className="pd-field">
          <h4 className="pd-label">{f.label}{f.base === f.proposed && <span className="pd-same"> · unchanged</span>}</h4>
          {f.base !== f.proposed && <div className="pd-body">{textDiff(f.base, f.proposed).map((r, i) => <Row key={i} r={r} />)}</div>}
        </div>))}
      {p.extra}
      {p.why.length > 0 && <p className="pd-why">Why: {p.why.map((k) => <KeyLink key={k} k={k} />)}</p>}
    </section>
  );
}
```

`<ins>` and `<del>` carry their meaning to assistive technology, and the `aria-label` names it in
the reader's words; `proposal.css` draws `del` struck through and `ins` underlined with a 2px
offset line, the colour on top of those shapes only (`forced-colors: active` keeps both lines).
Text inside the diff is plain text nodes, never parsed Markdown, so a proposal cannot inject
markup or a live link into the page.

#### Interfaces — complete: `desktop/src/components/proposal/ProposalStrip.tsx`

```tsx
// desktop/src/components/proposal/ProposalStrip.tsx — the strip above the head fields; three acts.
import { useState } from "react";
import { editProposalOf, type AuthoredText, type EditProposal, type IssueComment, type IssueRevision, type Serve, type User } from "../../api/work";
import { mutate, useWork } from "../../work/replica/useWork";
import { Editor } from "../editor/Editor";
import { docFromAuthored, toAuthored } from "../editor/authorClass";
import { servesDiff } from "../../work/diff/wordDiff";
import { ProposalDiff, type ProposalAuthor } from "./ProposalDiff";
import { agentLabel } from "./agentLabel";

export interface ProposalStripProps { issueId: string; head: IssueRevision; me: User; timeZone: string; alias: string }

type Open = { c: IssueComment; p: EditProposal };

/** Decision 6: open proposals, newest first. Resolved or erased ones are gone. */
export function openProposals(cs: readonly IssueComment[]): Open[] {
  return cs.flatMap((c) => { const p = c.resolvedAt === null ? editProposalOf(c) : null; return p ? [{ c, p }] : []; })
    .sort((x, y) => y.c.createdAt.localeCompare(x.c.createdAt));
}

const key = (s: Serve) => `${s.briefKey}#${s.criterionId}`;

/** The serves after the proposal, applied to the head it is shown against (decision 4). */
export function proposedServes(head: readonly Serve[], p: EditProposal): Serve[] {
  const rm = new Set(p.servesRemove.map(key));
  const kept = head.filter((s) => !rm.has(key(s)));
  const have = new Set(kept.map(key));
  return [...kept, ...p.servesAdd.filter((s) => !have.has(key(s)))];
}

export function ProposalStrip(props: ProposalStripProps): JSX.Element | null {
  const comments = useWork((r) => r.commentsOf(props.issueId));          // task 57's replica read (UNVERIFIED name)
  const sessions = useWork((r) => r.sessions());                           // the control plane's session list
  const moverOf = useWork((r) => r.revisionWriter(props.issueId, props.head.revision));
  const open = openProposals(comments);
  const [reviewing, setReviewing] = useState(false);
  const [editing, setEditing] = useState(false);
  const [readStale, setReadStale] = useState(false);
  const [title, setTitle] = useState<AuthoredText | null>(null);
  const [description, setDescription] = useState<AuthoredText | null>(null);
  if (open.length === 0) return null;

  const { c, p } = open[0];
  const stale = p.baseRevision < props.head.revision;
  const h = props.head.content;
  const proposedTitle = title ?? p.title ?? h.title;                       // an edited text wins over the proposal
  const proposedDesc = description ?? p.description ?? h.description;
  const author: ProposalAuthor = { owner: c.createdBy.displayName.split(" ")[0].toLowerCase(),
    agent: agentLabel(p.proposer, c.createdBy, props.me, sessions), model: p.proposer.model,
    sessionId: p.proposer.sessionId, at: c.createdAt, onRevision: p.baseRevision };

  const accept = () => {                                                   // decisions 3 and 4
    mutate({ kind: "revise", issueId: props.issueId, baseRev: props.head.revision, patch: {
      title: proposedTitle, description: proposedDesc, serves: proposedServes(h.serves, p) } });
    mutate({ kind: "resolve", issueId: props.issueId, commentId: c.id, resolved: true });
    setReviewing(false); setEditing(false);
  };
  const decline = () => { mutate({ kind: "resolve", issueId: props.issueId, commentId: c.id, resolved: true }); setReviewing(false); };

  return (
    <div className="ps" role="region" aria-label="Proposed edit">
      <p className="ps-line">
        Proposed edit · {author.owner} · {author.agent} · <time className="ps-mono" dateTime={c.createdAt}>
          {new Intl.DateTimeFormat("en-AU", { hour: "2-digit", minute: "2-digit", timeZone: props.timeZone }).format(new Date(c.createdAt))}
        </time> · on <span className="ps-mono">@{p.baseRevision}</span>
        {open.length > 1 && <> · <a href={`#${open[1].c.id}`}>{open.length - 1} more {open.length === 2 ? "proposal" : "proposals"}</a></>}
        {!reviewing && <button type="button" onClick={() => setReviewing(true)}>Review</button>}
      </p>
      {stale && (
        <p className="ps-stale">Stale: {props.alias} moved to <span className="ps-mono">@{props.head.revision}</span> ({moverOf}
          ) since this was proposed on <span className="ps-mono">@{p.baseRevision}</span>.</p>)}
      {reviewing && !editing && (
        <ProposalDiff timeZone={props.timeZone} author={author} why={p.why}
          fields={[{ label: "Title", base: h.title.text, proposed: proposedTitle.text },
                   { label: "Description", base: h.description.text, proposed: proposedDesc.text }]}
          extra={(p.servesAdd.length + p.servesRemove.length > 0) && <p className="pd-serves">{servesDiff(p.servesAdd, p.servesRemove)}</p>} />)}
      {reviewing && editing && (
        <div className="ps-edit">
          <Editor value={docFromAuthored(proposedTitle)} ariaLabel="Title" onCommit={(d) => setTitle(toAuthored(d))} />
          <Editor value={docFromAuthored(proposedDesc)} ariaLabel="Description" onCommit={(d) => setDescription(toAuthored(d))} />
        </div>)}
      {reviewing && (
        <div className="ps-acts">
          {stale && !readStale && <button type="button" onClick={() => setReadStale(true)}>I have read the changes against @{props.head.revision}</button>}
          <button type="button" onClick={accept} disabled={stale && !readStale}>Accept</button>
          {!editing && <button type="button" onClick={() => setEditing(true)}>Edit</button>}
          <button type="button" onClick={decline}>Decline</button>
        </div>)}
    </div>
  );
}
```

The diff is always **head → proposed**, so a stale proposal is shown against the head the person
would overwrite (the stale line names that head and who wrote it). A field the proposal leaves
`null` is the head's own text, which reads *unchanged* in the diff and is sent unchanged in the
patch with the head's own class. `agentLabel` (in `agentLabel.ts`, this task) is decision 2: task
47's `agentsWhoRead` numbering when the comment's owner is the reader, else `agent · s-7f2c`
(the first six characters of the session id). Both writes go through task 56's `mutate`, so each
has its Undo chip; *Accept* is two ops in two `mutate` calls because a resolve that the Hub refuses
must not roll back an accepted revision.

#### Interfaces — complete: the ⚑ deltas

```tsx
// desktop/src/components/issue/IssueView.tsx (task 46) — above the head fields, one line added:
<ProposalStrip issueId={head.id} head={headRevision} me={me} timeZone={me.timeZone} alias={alias} />

// desktop/src/components/issue/Comments.tsx (task 51) — at the top of the comment renderer, before
// the erased branch (task-51.md:466): an edit proposal is a link line, never a rendered body.
if (c.kind === "edit_proposal") {
  return (
    <article className="iv-cm iv-cm-proposal" data-comment={c.id}>
      <span className="iv-fact">{c.createdBy.displayName} · proposed an edit ·{" "}
        {c.resolvedAt === null ? <a href="#proposal" onClick={(e) => { e.preventDefault(); focusProposalStrip(); }}>Review</a>
                               : <>resolved by {c.resolvedBy?.displayName}</>}</span>
    </article>
  );
}
```

`focusProposalStrip` moves focus to the strip's region (`[aria-label="Proposed edit"]`) and opens
its review; no navigation. The branch comes first so an erased proposal also renders as the link
line's *erased* fact (task 51's erased line, same markup) and never as a body.

**BriefPanel (P4 task 18, UNVERIFIED until merge).** Where the panel renders an answer whose
`Author` is `model_drafted` **and** the brief's previous revision held a different answer to the
same question, it renders `<ProposalDiff fields={[{label: question, base: previous, proposed:
drafted}]} why={[]} author={…onRevision: null} />` in place of the plain drafted text, with
*Accept* sending `model_accepted` and *Edit* opening the answer editor whose save sends `person`,
both through phase 4's existing brief-revision write (P4 task 18's own call; this task adds no
route). A first-time drafted answer (nothing replaced) is unchanged: phase 4 already shows it as
drafted and refuses to settle it (`4-D2`).

#### Tests to write first — `wordDiff.test.ts`, `proposal.test.tsx`, and one Go row

Over task 126's `roadmapHarness` (the fake-Hub replica, `hub.writes()`) with EP-142 at head @3,
title *"Resume uploads after a drop"*, and an `edit_proposal` comment by `u-a` (dylan) whose
proposer is `{agentId: "ag-1", sessionId: "s-7f2c91", model: "claude-sonnet-4"}`, proposing the
title *"Resume uploads after a network drop"* on base @3. Every surface test ends with
`expectNoPrediction(container)`.

```ts
// wordDiff.test.ts
import { describe, expect, it } from "vitest";
import { textDiff, wordDiff } from "./wordDiff";
import { lineDiff } from "../../components/issue/revisionWords";

describe("wordDiff", () => {
  it("joins back to both texts, byte for byte", () => {
    const a = "Resume uploads after a drop", b = "Resume uploads after a network drop";
    const parts = wordDiff(a, b);
    expect(parts.filter((p) => p.op !== "add").map((p) => p.text).join("")).toBe(a);
    expect(parts.filter((p) => p.op !== "del").map((p) => p.text).join("")).toBe(b);
    expect(parts).toContainEqual({ op: "add", text: "network " });
  });
  it("keeps lineDiff's lines unchanged outside a changed pair", () => {           // kills "a lifted diff that drifts"
    const a = "one\ntwo\nthree", b = "one\n2\nthree";
    expect(textDiff(a, b).filter((r) => r.kind === "same").map((r) => (r as { text: string }).text))
      .toEqual(lineDiff(a, b).filter((l) => l.op === "same").map((l) => l.text));
  });
});
```
