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
