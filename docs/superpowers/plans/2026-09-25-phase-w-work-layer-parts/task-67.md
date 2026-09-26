### Task 67: issue chips in Develop and Merge

Stream **WD2**, about 2 hours (an estimate, not a measurement). Deltas on phase 4's and phase 3's
desktop files under rule H2: the owner's file wins, and this task **waits** for it on the sync base
and never writes a second copy. Depends on task 48 (the issue view's revision pin, beside tasks 46
and 47), P4-17 (`ContextChip`, `SessionContextChip`) and P3-19 (`LaneCard`), and through their seams
on task 18 (`Ref.Kind`, the issue fields), task 19 (`Session.issueRef`, the second key source) and
task 21 (the lane index's issue id).

After this task a session's pane header shows what the agent was told twice, side by side: the
brief chip it always had, and `EP-142@3` beside it. A lane in Merge names its issue. Every chip
opens the one issue view at the revision it names, and says so when that is not the head.

**Decisions made here, so nobody re-derives them:**

1. **One chip component, two arms.** P4-17's `ContextChip` takes a `ProvenanceRef`
   (P4 `task-17.md:49-61`). The issue arm is chosen by `refValue.kind === "issue"` (task 18's
   field, `task-18.md:104-108`), never by the key's prefix: the desktop never splits a key
   (`task-19.md:300-303`). A brief ref renders exactly as P4-17 wrote it.
2. **The alias is read, never kept.** The chip prints the replica head's `alias` when the head is
   in the replica, else the ref's `issueAlias` (resolve-time, `task-18.md:113`). An admin's
   project-key change moves the head's alias at once and the key not at all (05 §3.6), so the chip
   follows it without a re-resolve. The alias is never a cache key, a map key or a route.
3. **The revision is the key's, not the head's.** `EP-142@3` prints `refValue.revision`. When the
   replica's `headRevision` is higher the chip adds `· head is @4` in the same mono run. The
   issue view opens through task 37's `openIssue(id, revision)` (`task-37.md:560`) with the chip's
   revision, so the view shows `@3`, and task 48's pin says it is not the head.
4. **The seven states map onto P4's words.** `resolving` and `unavailable` render as P4-17 renders
   them for a brief (the key in mono, P4's words); `missing` prints the key verbatim; `erased`
   prints `EP-142@3 · erased` with no title (hub-contract §7.12) and still opens the view, where
   task 46 shows the erased state. `superseded` does not apply: P4-16's rule 3 reads
   `headsByBrief`, which holds no issue; the issue arm uses decision 3's head check instead. `none`
   and `declined` never reach the issue arm: a session with no `issueRef` renders no issue chip.
5. **Beside, never instead.** `SessionPaneHeader` mounts `SessionContextChip` for the brief (P4
   plan `:364`) and, **after it**, `SessionIssueChip` for `Session.issueRef`. A session with no
   issue ref renders nothing in that slot: no placeholder, no empty chip, no reserved width.
6. **Lanes name their issue from the lane index.** The lane's `issueId` is task 21's `Lane.IssueID`
   (`task-21.md:183`, `json:"issueId,omitempty"`). P3's `LaneObservation` does not carry it (P3
   `task-07.md:228-236`, `task-18.md:42-47`), so this task adds `IssueID` to the observation
   (Go and TS, one field each, copied from the lane index by the observer). The branch name is
   never parsed: an AU-only project's branch is `ep-142` with no title (task 21, `OD-24`), and a
   parsed branch would break the day a project key changes.
7. **A lane chip names no revision of its own.** A lane is the work, not the brief the agent read.
   The chip prints the alias; the revision comes from the lane's newest session's `issueRef` when
   the store holds one (`LaneView.sessions`, P3 `task-18.md:63`), and the chip then reads like the
   pane's. With no session record the chip reads `EP-142` and opens the head, which it says in
   its `title`.
8. **The thread node lights with its criterion.** The issue chip renders `<ThreadNode/>` (SW.14)
   for the first criterion the ref's `serves` names, bound with `useThreadLit` (as task 47 binds
   its serve rows, `task-47.md:451`), so hovering the chip lights that criterion everywhere; a ref
   that serves nothing renders no node.

**Files:**
- `desktop/src/provenance/ContextChip.tsx` ⚑ (P4-17) — the `kind: "issue"` arm, `IssueChipBody`
  (modified; **UNVERIFIED until P4 merges**)
- `desktop/src/provenance/SessionIssueChip.tsx` (+ `SessionIssueChip.test.tsx`) — the store-reading
  mount for `Session.issueRef` (new; **not in the skeleton's file map**)
- `desktop/src/provenance/provenance.css` ⚑ (P4-17) — `.pv-chip-issue`, `.pv-chip-behind` (modified)
- `desktop/src/components/develop/SessionPaneHeader.tsx` ⚑ — one mount after the brief chip (modified)
- `desktop/src/components/lens/merge/LaneCard.tsx` ⚑ (P3-19, `task-19.md:140`) — `LaneIssueChip`
  (modified; **UNVERIFIED until P3 merges**)
- `control-plane/internal/converge/observe.go` ⚑ (P3-07's `LaneObservation`, `task-07.md:228`) and
  `desktop/src/api/converge.ts` ⚑ (P3-18's mirror, `task-18.md:42`) — `issueId` (modified; the file
  names are P3's, **UNVERIFIED**; the field goes where the type is declared)
- `desktop/src/components/lens/merge/LaneCard.test.tsx` ⚑ — tests 5 and 6 (modified)

**Interfaces:**
- Produces: the issue arm of `ContextChip`; `SessionIssueChip`; `LaneIssueChip`;
  `LaneObservation.issueId`. Consumed by task 71 (Deploy's chip reuses the arm) and task 70's run.
- Consumes: P4-16's `selectProvenanceState` and `selectRef` (P4 `task-16.md:161-179`); task 19's
  `selectSessionIssueRefs` (`task-19.md:314`), which puts every `issueRef` on the loader's list;
  task 37's `openIssue(id, revision)`; task 55's `useWork`; SW.14's `ThreadNode` and
  `useThreadLit`; task 47's `criterionHref`; task 20's `expectNoPrediction`.

#### Interfaces — complete: the issue arm of `ContextChip.tsx` ⚑ (P4-17)

```tsx
// desktop/src/provenance/ContextChip.tsx ⚑ — task 67 adds the branch and IssueChipBody.
// ContextChipProps (P4 task-17.md:49-60) gains two optional members, used by the issue arm only:
//   /** The replica's head of the ref's issue, when the replica holds it (decisions 2 and 3). */
//   issueHead?: { alias: string; headRevision: number };
//   /** Opens the one issue view at this revision (task 37's openIssue). */
//   onOpenIssue?(issueId: string, revision: number | null): void;
import { ThreadNode, useThreadLit } from "../components/thread";   // SW.14; path UNVERIFIED
import { criterionHref } from "../components/issue/criterionHref";  // task 47; path UNVERIFIED

// First line of ContextChip's body:
//   if (props.refValue?.kind === "issue") return <IssueChipBody {...props} />;
// A caller that knows its key is an issue key before it resolves (SessionIssueChip, LaneIssueChip)
// renders IssueChipBody directly, so a resolving issue ref never takes the brief arm.

export function IssueChipBody(props: ContextChipProps): JSX.Element {
  const { keyText, state, refValue: r, issueHead: h, onOpenIssue } = props;
  const alias = h?.alias || r?.issueAlias || "";
  const first = r?.serves[0];
  const lit = useThreadLit(first?.criterionId ?? "");
  if (!r || state === "resolving" || state === "unavailable" || state === "missing" || alias === "") {
    return <span className="pv-chip pv-chip-issue" data-state={state}><code>{keyText}</code></span>;
  }
  const erased = state === "erased";
  const behind = !erased && h !== undefined && h.headRevision > r.revision;
  return (
    <button type="button" className="pv-chip pv-chip-issue" data-state={erased ? "erased" : "current"}
      title={erased ? `${alias}@${r.revision} was erased` : r.title}
      onClick={() => onOpenIssue?.(r.issueId, r.revision)} {...(first ? lit.bind : {})}>
      {first ? <ThreadNode criterionId={first.criterionId} href={criterionHref(first.briefKey, first.criterionId)}
        label={`${alias} serves ${first.criterionId}`} /> : null}
      <span className="pv-chip-key">{alias}@{r.revision}</span>
      {behind ? <span className="pv-chip-behind"> · head is @{h!.headRevision}</span> : null}
      {erased ? <span className="pv-chip-behind"> · erased</span> : null}
    </button>
  );
}
```

`useThreadLit("")` binds nothing (SW.14's rule for an empty id, **UNVERIFIED**; if SW.14 throws on
`""`, the node and the hook move into a child component rendered only when `first` exists). The
chip never prints the title in its face: the face is `EP-142@3`, and the title is the `title`
attribute and the issue view's (05 §4.5 rule 1). An erased ref's `title` is `""` by task 18's
`ErasedIssueRef`, and the chip never reaches for a cached one.

#### Interfaces — complete: `desktop/src/provenance/SessionIssueChip.tsx`

```tsx
// The one store-reading issue chip. P4-17's SessionContextChip is its model (task-17.md:64-69).
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../store/store";
import { selectProvenanceState, selectRef } from "../store/selectors";   // P4-16
import { useWork } from "../work/replica/useWork";                        // task 55; path UNVERIFIED
import { IssueChipBody } from "./ContextChip";
import type { SessionId } from "../store/types";

export function SessionIssueChip({ sessionId }: { sessionId: SessionId }): JSX.Element | null {
  const key = useCockpit((s) => s.sessions[sessionId]?.issueRef ?? "");
  const state = useCockpit((s) => (key === "" ? "none" : selectProvenanceState(key)(s)));
  const ref = useCockpit(useShallow((s) => (key === "" ? undefined : selectRef(key)(s))));
  const head = useWork((w) => (ref?.issueId ? w.get(ref.issueId)?.head : undefined));
  const openIssue = useCockpit((s) => s.openIssue);
  if (key === "") return null;                                          // decision 5: no slot
  return (
    <IssueChipBody keyText={key} state={state} refValue={ref} head={undefined}
      issueHead={head ? { alias: head.alias, headRevision: head.headRevision } : undefined}
      onOpenDiscover={() => {}} onOpenIssue={(id, rev) => openIssue(id, rev)} />
  );
}
```

`onOpenDiscover` is required by P4-17's props and never called by the issue arm; a brief key
never reaches this mount because `issueRef` holds issue keys only (`task-19.md:300`).

#### Interfaces — complete: the pane header mount (`SessionPaneHeader.tsx` ⚑)

```tsx
// desktop/src/components/develop/SessionPaneHeader.tsx ⚑ — after P4-17's mount (P4 plan :364):
        <SessionContextChip sessionId={session.id} />
        <SessionIssueChip sessionId={session.id} />
```

The two sit in the header's existing flex run; the issue chip adds no wrapper and no gap token
of its own, so a session with no issue ref renders the header exactly as P4-17 left it.

#### Interfaces — complete: the lane's issue (`LaneObservation` ⚑, `LaneCard.tsx` ⚑)

```go
// control-plane/internal/converge — LaneObservation (P3 task-07.md:228-236) gains, last:
	IssueID string `json:"issueId"` // the lane index's Lane.IssueID (task 21); "" when none
// The observer copies it from the lane it observes, next to WorktreeID and Name; it reads no Hub.
```

```ts
// desktop/src/api/converge.ts ⚑ — LaneObservation (P3 task-18.md:42-47) gains, last:
//   issueId: string;   // "" when the lane was not made from an issue
```

```tsx
// desktop/src/components/lens/merge/LaneCard.tsx ⚑ — LaneIssueChip, mounted in the card's title
// line after the lane's name (P3-19's layout; its slot is P3's to place).
import { useCockpit } from "../../../store/store";
import { useWork } from "../../../work/replica/useWork";
import { SessionIssueChip } from "../../../provenance/SessionIssueChip";

export function LaneIssueChip({ lane }: { lane: LaneView }): JSX.Element | null {
  const withRef = useCockpit((s) =>
    [...lane.sessions].reverse().map((ls) => s.sessions[ls.sessionId]).find((x) => x?.issueRef)?.id ?? null);
  const head = useWork((w) => (lane.issueId ? w.get(lane.issueId)?.head : undefined));
  const openIssue = useCockpit((s) => s.openIssue);
  if (!lane.issueId) return null;
  if (withRef) return <SessionIssueChip sessionId={withRef} />;     // decision 7: the pane's chip
  return (
    <button type="button" className="pv-chip pv-chip-issue" title="opens the issue at its head"
      onClick={() => openIssue(lane.issueId, null)}>
      <span className="pv-chip-key">{head ? head.alias : lane.issueId}</span>
    </button>
  );
}
```

`LaneSession.sessionId` is P3-18's member name as this part reads it (**UNVERIFIED**; P3
`task-18.md:63` names the array, not its fields). `lane.sessions` is oldest first in P3's order,
so the reverse finds the newest. The chip reads `lane.issueId` only; `lane.name` and the branch
are never read for it (decision 6), and a source test holds that (test 5).

#### Interfaces — complete: appended to `desktop/src/provenance/provenance.css` ⚑ (P4-17)

```css
/* Task 67. The issue chip is P4's chip; only its face differs. */
.pv-chip-issue .pv-chip-key { font-family: var(--mono); font-variant-numeric: tabular-nums; }
.pv-chip-behind { font-family: var(--mono); color: var(--ink-2); }
.pv-chip-issue[data-state="erased"] .pv-chip-key { color: var(--ink-3); }
```

`--ink-2`, `--ink-3` and `--mono` are on the tree (`desktop/src/styles/tokens.css:46-47`, `:152`;
`--mono` becomes Commit Mono under `OD-28` through task 26, and this rule names no family). Task
26's `--t-fact`, if it lands first, replaces the `font-family` lines with `font: var(--t-fact)`.

#### Tests to write first, part 1 — `desktop/src/provenance/SessionIssueChip.test.tsx`

P4-16's two selectors and task 55's `useWork` are mocked at their module seams, so the test pins
this task's reading of them and not their internals. `SessionContextChip` is replaced by a marker
so test 1 can see *beside*. `SessionPaneHeader` takes phase 2's props (`SessionPaneHeader.tsx:16-24`).

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../test/expectNoPrediction";
import { useCockpit } from "../store/store";
import { SessionPaneHeader } from "../components/develop/SessionPaneHeader";

const KEY = "issue:is-3f2a91c07b1e4d2a@3";
let refs: Record<string, unknown> = {};
let states: Record<string, string> = {};
let heads: Record<string, { alias: string; headRevision: number }> = {};
vi.mock("../store/selectors", async (orig) => ({
  ...(await orig<object>()),
  selectRef: (k: string) => () => refs[k],
  selectProvenanceState: (k: string) => () => states[k] ?? "resolving",
}));
vi.mock("../work/replica/useWork", () => ({
  useWork: (sel: (w: unknown) => unknown) => sel({ get: (id: string) => (heads[id] ? { head: heads[id] } : undefined) }),
}));
vi.mock("./SessionContextChip", () => ({ SessionContextChip: () => <span data-testid="brief-chip" /> }));

const ref = (over = {}) => ({ key: KEY, kind: "issue", revision: 3, issueId: "is-3f2a91c07b1e4d2a",
  issueNumber: 142, issueAlias: "EP-142", title: "Resume a session after a restart",
  serves: [{ briefKey: "brief:bf-1@5", criterionId: "c-40aa" }], ...over });
const session = (issueRef: string | null) => ({ id: "s-1", status: "running", harness: "claude",
  title: "t", exitReason: "", issueRef }) as never;
function mount(issueRef: string | null) {
  useCockpit.setState({ sessions: { "s-1": session(issueRef) } } as never);
  return render(<SessionPaneHeader session={session(issueRef)} worktree={undefined} variant="column" />);
}
beforeEach(() => { refs = { [KEY]: ref() }; states = { [KEY]: "current" };
  heads = { "is-3f2a91c07b1e4d2a": { alias: "EP-142", headRevision: 4 } }; });

describe("the issue chip", () => {
  it("renders EP-142@3 beside the brief chip, after it", () => {
    const { container } = mount(KEY);
    const brief = screen.getByTestId("brief-chip");
    const chip = container.querySelector(".pv-chip-issue")!;
    expect(chip.querySelector(".pv-chip-key")!.textContent).toBe("EP-142@3");
    expect(brief.compareDocumentPosition(chip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // mutant: the issue chip replaces the brief chip
  });
  it("opens the issue view at @3 while the head is @4, and says so", () => {
    const openIssue = vi.fn();
    useCockpit.setState({ openIssue } as never);
    const { container } = mount(KEY);
    expect(container.querySelector(".pv-chip-behind")!.textContent).toBe(" · head is @4");
    fireEvent.click(container.querySelector(".pv-chip-issue")!);
    expect(openIssue).toHaveBeenCalledWith("is-3f2a91c07b1e4d2a", 3);
    // mutant: opens the head (revision null or 4)
  });
  it("an erased ref says erased and shows no title, and still opens", () => {
    refs = { [KEY]: ref({ title: "" }) }; states = { [KEY]: "erased" };
    const { container } = mount(KEY);
    const chip = container.querySelector(".pv-chip-issue")!;
    expect(chip.textContent).toContain("erased");
    expect(chip.textContent).not.toContain("Resume a session");
    expect(chip.getAttribute("title")).toBe("EP-142@3 was erased");
    // mutant: a stale cached title in the face or the tooltip
  });
  it("a session with no issueRef renders no issue chip and no empty slot", () => {
    const { container } = mount(null);
    expect(container.querySelector(".pv-chip-issue")).toBeNull();
    const alone = render(<SessionIssueChip sessionId={"s-1" as never} />);
    expect(alone.container.firstChild).toBeNull();                 // nothing, not an empty span
    // mutant: a placeholder chip or a reserved-width slot
  });
  it("the alias follows a project-key change; the key does not move", () => {
    const { container, rerender } = mount(KEY);
    heads = { "is-3f2a91c07b1e4d2a": { alias: "PAY-142", headRevision: 4 } };
    rerender(<SessionPaneHeader session={session(KEY)} worktree={undefined} variant="column" />);
    expect(container.querySelector(".pv-chip-key")!.textContent).toBe("PAY-142@3");
    expect(useCockpit.getState().sessions["s-1"].issueRef).toBe(KEY);
    // mutant: the alias cached (a map keyed by alias, or the ref's resolve-time alias preferred)
  });
  it("the pane header predicts nothing", () => {
    const { container } = mount(KEY);
    expectNoPrediction(container);
  });
});
```

The file also imports `SessionIssueChip` from `./SessionIssueChip` for test 4. `useWork`'s
`get(id)` returning `{ head }` is task 60's reading of task 55's replica (`task-60.md:291-295`).

#### Tests to write first, part 2 — `LaneCard.test.tsx` ⚑ (P3-19's file) and the observer

```tsx
// Appended to P3-19's LaneCard.test.tsx; `lane(over)` is P3-19's LaneView fixture (UNVERIFIED name).
// The file takes part 1's `useWork` mock and its `heads` map, copied at its top.
it("a lane names EP-142 from the lane index, never from its branch", () => {
  const view = lane({ name: "feature/resume-after-restart", issueId: "is-3f2a91c07b1e4d2a", sessions: [] });
  heads = { "is-3f2a91c07b1e4d2a": { alias: "EP-142", headRevision: 4 } };
  const { container } = render(<LaneIssueChip lane={view} />);
  expect(container.querySelector(".pv-chip-key")!.textContent).toBe("EP-142");
  const bare = render(<LaneIssueChip lane={lane({ name: "ep-142", issueId: "", sessions: [] })} />);
  expect(bare.container.firstChild).toBeNull();                 // an "ep-142" branch names nothing
  // mutant: parses the branch name (AU-only branches are title-free, Q8, and keys change)
});
it("the lane chip's source never reads the lane's name or branch", async () => {
  const src = (await import("./LaneCard.tsx?raw")).default as string;
  const body = src.slice(src.indexOf("export function LaneIssueChip"));
  expect(body.slice(0, body.indexOf("\n}\n"))).not.toMatch(/lane\.(name|branch|tip)\b/);
});
```

```go
// converge's observer test (P3-07's file) gains one row:
// a lane created with CreateRequest{IssueID: "is-3f2a91c07b1e4d2a"} observes with
// IssueID "is-3f2a91c07b1e4d2a"; a lane created without one observes with "".
// mutant: the observer leaves IssueID empty, so every lane chip renders nothing.
```

That makes 9 cases: 6 in `SessionIssueChip.test.tsx`, 2 in `LaneCard.test.tsx`, 1 Go row.

#### Steps

- [ ] Wait for P4-17 and P3-19 on the sync base (rule H2). If either is absent, stop and report
      it; do not write `ContextChip.tsx` or `LaneCard.tsx`.
- [ ] Add `LaneObservation.IssueID` (Go and TS) and the observer row; `go test
      ./control-plane/internal/converge/...` passes.
- [ ] Write `SessionIssueChip.test.tsx`; it fails to import. Add `IssueChipBody` to
      `ContextChip.tsx`, then `SessionIssueChip.tsx`, then the header mount, until the six pass.
      Mutants, one at a time, then revert: mount the issue chip in place of the brief chip (test 1);
      call `openIssue(id, null)` (test 2); fall back to a cached `title` when erased (test 3); render
      an empty `<span>` for no ref (test 4); prefer `r.issueAlias` over the head's alias (test 5).
- [ ] Add `LaneIssueChip` and its two tests. Mutant: `lane.name.match(/^[a-z]+-\d+/)` as a
      fallback alias (test 7).
- [ ] P4-17's and P3-19's own suites still pass unchanged: the brief arm is untouched.
- [ ] Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/provenance src/components/lens/merge'
# expected: SessionIssueChip 6 passing, LaneCard's new 2 passing, and every P4-17 and P3-19 case passing
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/converge/... -count=1'
# expected: ok
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -nE "split\(.@.\)|issueRef\.(split|slice|indexOf)" desktop/src/provenance desktop/src/components/lens/merge -r'
# expected: no output (the key is never split, task 19)
```

#### What this task does NOT claim

- **Deploy's chip** (task 71), Design's `implements` (later, 05 §5.5), or proposed evidence under
  each criterion (W3, task 104).
- **That P4-17 and P3-19 exist**: both are **UNVERIFIED until merge**; this task waits for them.
- **External links** (`https://…/EP-142@3`): task 93. This task links inside the app only.
- **Visual acceptance**: the owner's eyes; this part proves the order, the text and the call.
- **Parity**: closes L-191's visible half (what the agent was told, with task 19) and L-98's
  revision-pinned in-app link (with task 47); serves L-201.

#### Gaps found while writing this part

1. P3's `LaneObservation` does not carry the lane index's `IssueID` (P3 `task-07.md:228-236`,
   `task-18.md:42-47`); task 21 added `Lane.IssueID` to the store only. This task adds the field;
   the skeleton's file map lists neither `observe.go` nor `api/converge.ts` for it.
2. `SessionIssueChip.tsx` is not in the skeleton's file map (P4-17 has `SessionContextChip.tsx`
   as the model, so the issue mount is its own file).
3. The scope cites P3's plan at `:556-558` for `LaneCard`; on the phase-3 worktree the file map
   row is at `:575` and the component at P3 `task-19.md:140`.
4. `LaneSession`'s member names (P3-18) are not printed; `sessionId` is this part's reading.
5. SW.14's behaviour for `useThreadLit("")` is not printed; decision 8's fallback covers it.
