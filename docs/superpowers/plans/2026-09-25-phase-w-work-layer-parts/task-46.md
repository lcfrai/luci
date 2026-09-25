### Task 46: the issue view, part 1: head fields and their pickers

Stream **WD3** (the issue view), about 3 hours (an estimate, not a measurement). Depends on task 37
(SW.15, the desktop Work types and `workSlice`) and task 39 (the owner's review of the in-app
prototype, whose words this view must not contradict). Uses SW.1 (`DisplayAlias`, the issue head),
SW.4 (`StateMove`), SW.11 (faces and `--t-fact`), SW.12 (`<StateGlyph/>`), SW.13 (`<ViewHeader/>`)
and SW.16 (`useWork`, `mutate`). Tasks 47, 48, 49 and 53 build on what this part exports.

An issue's head is the part a person reads before deciding to act, and it is also where the product
can lie most easily. A picker that lets someone click *Started* has said "work has begun" when no
worktree exists; a red urgent badge has said "someone is waiting on you" when nobody is. So the head
holds four rules, each one a test:

1. **Fact states are shown and never chosen.** `started`, `in_review` and `landed` are facts the
   control plane observes (a worktree bound, a review open, a land recorded; 05 §3.3). The picker
   lists them, each with the fact it waits on in words, and they cannot be selected. Clicking one
   sends nothing. The Hub refuses a hand-set fact state anyway (SW.3's guard, task 07); the picker
   refusing first is so the person is never shown a choice that will bounce.
2. **Priority is ink, never colour.** The five levels draw as `StateGlyph`'s notch (task 28), urgent
   included. `--need` amber means only *someone is waiting on you* (05 §4.4 work rule 2).
3. **The alias is display only.** `EP-142` comes from `DisplayAlias(projectKey, n)` (SW.1) and is
   shown in mono; every link, React key, `aria-controls` and write uses the `is-` id (05 §3.6). A
   project renamed from `EP` to `EPX` changes what the head reads and nothing else.
4. **People are assigned; agents are not.** The assignee list is people only (`OD-16`). When the
   issue has **another owner's live agents**, assigning is not a unilateral act: the commit becomes
   *Ask Dylan to hand over*, a reassign request, and the assignee stays as it is until the owner
   decides on their Desk (task 65; 05 §3.7 rule 2).

`done` has two cases. On an issue whose `serves` is empty, `done` is an ordinary choice. On an
issue that serves criteria, `done` by hand is *done, not proven* (05 §3.3) and needs a reason; this
part marks the option *needs a reason* and leaves it disabled, and task 48 adds the reason step and
enables it. The split is deliberate: this part must not ship a path that sends a reasonless done on
a serving issue, even for one commit.

**Files:**
- `desktop/src/components/issue/IssueView.tsx` — the view's frame: `ViewHeader`, the head, and the
  slots tasks 47, 48, 51 and 52 fill (new)
- `desktop/src/components/issue/IssueHead.tsx` — title, alias, the four field buttons, the facts
  line, the reassign-requested line (new)
- `desktop/src/components/issue/pickers/Picker.tsx` — the one listbox the four pickers share (new;
  not yet in the File map)
- `desktop/src/components/issue/pickers/{StatePicker,PriorityPicker,AssigneePicker,LabelPicker}.tsx`
  — the four pickers (new; not yet in the File map)
- `desktop/src/components/issue/pickers/DuplicateOfPicker.tsx` — *Duplicate of which issue* (new;
  not yet in the File map)
- `desktop/src/components/issue/AuthorClass.tsx` — the author-class mark and word, `text·H` or
  `text·A`, which tasks 47, 49 and 51 reuse (new; not yet in the File map; see decision 7)
- `desktop/src/components/issue/pickers/stateOptions.ts` — the nine states, which are choosable, and
  the fact each fact state waits on (new; not yet in the File map)
- `desktop/src/components/issue/issue.css` — owns the `.iv-*` prefix and no other (new; not yet in
  the File map)
- `desktop/src/components/issue/IssueHead.test.tsx`, `pickers/pickers.test.tsx`,
  `pickers/stateOptions.test.ts` — the tests (new)

No existing file changes. The Work lens (task 40) opens `IssueView` from a row; task 53 wires the
row keys `S`, `P`, `A` and `L` to open these pickers beside the row, and `I` to *assign to me*
(05 §4.3, the `A` · `I` row), which is `AssigneePicker`'s first option exported as a function.

**Decisions made here, so nobody re-derives them:**

1. **`triage` is choosable.** The scope's list of choosable states omits it; task 10's decision 1
   (`task-10.md:17-23`) allows a person's move into `triage` with cause `hand`, so the picker offers
   it. A reopen (from `landed` or `done` back to `todo`) is also `hand` and is offered.
2. **"Another owner's live agents" is the lane-bound state, in W1.** The scope derives it from the
   other owner's sessions' `IssueRef` (task 19), but in local mode those sessions live on the other
   person's control plane and this desktop cannot see them. What the Hub does carry is the fact
   state: an issue at `started` or `in_review` has a bound lane (task 22). So the test is
   `state ∈ {started, in_review}` and `assignee.id ≠ me.id`, which is also task 65's rule (its test 3:
   a reassign on `todo` is a plain assign, on `started` it is a decide). A W3 task may widen it with
   Fleet's live sessions.
3. **The reassign request is a `WorkOp` this part needs and task 56 does not list.** Task 65's scope
   names the Hub half, `RequestReassign(issueId, toUserId)` and `IssueHead.reassignRequest`, as a
   gap it builds on the fake. This part sends `mutate({kind: "requestReassign", …})`; the union
   member is a gap for task 56 (named in *Gaps*), and until task 65 lands the fake answers it.
4. **The reader is `GET /v1/me`.** The desktop has no signed-in-user seam today (grep of
   `desktop/src` for `currentUser`, `whoami`, `me`: none, 2026-09-25). The Hub has `GET /v1/me`
   (`2026-09-23-hub-contract.md:565`, route 3), which the control plane relays; `IssueView` takes
   `me: User` as a prop and its mount passes it. Where the mount gets it is task 40's, a gap.
5. **Assignable people are the team's members**, from `GET /v1/teams/{teamId}` (route 6,
   `hub-contract.md:568`), whose body is `hubapi.TeamDetail{team, members}`
   (`hubapi/types.go:226-229` on `feat/hub` at `9f2fa31`). A member is a `User`; there is no agent
   or session in that list by construction, and the picker adds none.
6. **The title is in the `ViewHeader`, the alias in its badge slot.** SW.13 says no other file
   renders a view title, so the issue title is `ViewHeader`'s `title` and `IssueHead` renders the
   fields below it. The alias goes in `badge` as mono text, the route in `route`.
7. **The author class gets its own small mark here.** SW.12's `MarkKind` is `selected`,
   `needs_you` and `thread_node` (`task-29.md:145`); none says who wrote a text. The title is an
   `AuthoredText` (`task-02.md:224-229`), so the head needs the class now, and tasks 47, 49 and 51
   need it next. `AuthorClass.tsx` draws two shapes of our own (a solid bar for a person, an open
   bracket for an agent) in `currentColor` with the word beside them. If task 29's owner lifts it
   into `MarkGlyph`, the three callers change one import (a gap, named below).

#### Interfaces — complete: `desktop/src/components/issue/pickers/stateOptions.ts`

```ts
// SW.4 as the picker reads it (task 10, decision 1): which states a person may choose, and what
// each fact state waits on. Pure; no React, no replica.
import type { IssueState } from "../../glyphs/StateGlyph";

export const STATE_ORDER: readonly IssueState[] = [
  "triage", "backlog", "todo", "started", "in_review", "landed", "done", "cancelled", "duplicate",
];

export const STATE_LABEL: Readonly<Record<IssueState, string>> = {
  triage: "Triage", backlog: "Backlog", todo: "Todo", started: "Started", in_review: "In review",
  landed: "Landed", done: "Done", cancelled: "Cancelled", duplicate: "Duplicate",
};

/** The three fact states and the fact each waits on, in words (05 §3.3). */
export type FactState = "started" | "in_review" | "landed";
export const FACT_WAITS_ON: Readonly<Record<FactState, string>> = {
  started: "moves when a worktree is bound to this issue",
  in_review: "moves when its lane is queued for review",
  landed: "moves when a commit with this issue's trailer lands",
};

export const isFactState = (s: IssueState): s is FactState =>
  s === "started" || s === "in_review" || s === "landed";

/** choose: a hand move; fact: shown, never sent; needs_reason: done on a serving issue (task 48
 *  enables it); pick_duplicate: opens the duplicate-of picker before anything is sent. */
export type StateOptionKind = "choose" | "fact" | "needs_reason" | "pick_duplicate";

export interface StateOption {
  state: IssueState;
  label: string;
  kind: StateOptionKind;
  note: string;      // the words under the label: the fact waited on, "needs a reason", or ""
  current: boolean;
}

/** `serving` is `serves.length > 0` on the head revision being read. */
export function stateOptions(current: IssueState, serving: boolean): StateOption[] {
  return STATE_ORDER.map((state) => {
    const base = { state, label: STATE_LABEL[state], current: state === current };
    if (isFactState(state)) return { ...base, kind: "fact" as const, note: FACT_WAITS_ON[state] };
    if (state === "done" && serving) return { ...base, kind: "needs_reason" as const, note: "needs a reason" };
    if (state === "duplicate") return { ...base, kind: "pick_duplicate" as const, note: "" };
    return { ...base, kind: "choose" as const, note: "" };
  });
}
```

The label of a fact state stays in the list, in the same ink as the others, so a reader learns
where the issue will go; only `aria-disabled="true"` and the note set it apart. The current state
is marked `aria-selected="true"` and is never re-sent: choosing it closes the picker.

#### Interfaces — complete: `desktop/src/components/issue/pickers/Picker.tsx`

```tsx
// The one listbox the four pickers share. Keyboard: type to filter, Up/Down to move, Enter to
// choose, Esc to close. A disabled item is reachable (its note is read aloud) and never chosen.
import { useId, useMemo, useState, type JSX, type KeyboardEvent, type ReactNode } from "react";
import "../issue.css";

export interface PickerItem {
  id: string;          // a state, a priority, a u- id, an lb- id; never an alias (05 §3.6)
  label: string;       // sans
  note?: string;       // sans, under the label
  lead?: ReactNode;    // a glyph; decorative, the label carries the meaning
  disabled?: boolean;
  selected?: boolean;
}

export function Picker(props: {
  label: string;
  items: readonly PickerItem[];
  multi?: boolean;
  onChoose(id: string): void;
  onClose(): void;
}): JSX.Element {
  const { label, items, multi = false, onChoose, onClose } = props;
  const base = useId().replace(/[^A-Za-z0-9_-]/g, "");
  const [query, setQuery] = useState("");
  const shown = useMemo(
    () => items.filter((i) => i.label.toLowerCase().includes(query.trim().toLowerCase())),
    [items, query],
  );
  const [active, setActive] = useState(0);
  const at = Math.min(active, Math.max(shown.length - 1, 0));
  const choose = (item: PickerItem | undefined) => {
    if (!item || item.disabled) return;          // a fact state or a needs-reason done: nothing sent
    onChoose(item.id);
    if (!multi) onClose();
  };
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") { e.preventDefault(); setActive(Math.min(at + 1, shown.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setActive(Math.max(at - 1, 0)); }
    else if (e.key === "Enter") { e.preventDefault(); choose(shown[at]); }
    else if (e.key === "Escape") { e.preventDefault(); onClose(); }
    e.stopPropagation();                          // keys.list never sees a key typed here (task 53)
  };
  const optId = (i: PickerItem) => `${base}-${i.id}`;
  return (
    <div className="iv-picker" role="dialog" aria-label={label}>
      <input
        className="iv-picker-q" autoFocus value={query} aria-label={`${label}: filter`}
        role="combobox" aria-expanded="true" aria-controls={`${base}-list`}
        aria-activedescendant={shown[at] ? optId(shown[at]) : undefined}
        onChange={(e) => { setQuery(e.target.value); setActive(0); }} onKeyDown={onKey}
      />
      <ul className="iv-picker-list" id={`${base}-list`} role="listbox" aria-label={label}
        aria-multiselectable={multi || undefined}>
        {shown.map((item, n) => (
          <li
            key={item.id} id={optId(item)} role="option"
            aria-selected={item.selected ? true : false}
            aria-disabled={item.disabled || undefined}
            data-active={n === at || undefined}
            className="iv-picker-opt"
            onPointerMove={() => setActive(n)}
            onClick={() => choose(item)}
          >
            {item.lead ? <span className="iv-picker-lead" aria-hidden="true">{item.lead}</span> : null}
            <span className="iv-picker-label">{item.label}</span>
            {item.note ? <span className="iv-picker-note">{item.note}</span> : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

The picker has no positioning of its own: its caller places it (under the head field here, beside
the row in task 53). It draws no spinner and waits on nothing: every write is optimistic through
`mutate`, so it closes the moment a choice is made.

#### Interfaces — complete: `desktop/src/components/issue/pickers/StatePicker.tsx`

```tsx
import { useState, type JSX } from "react";
import type { StateMove } from "../../../api/work";
import { mutate } from "../../../work/replica/ops";
import { StateGlyph, type IssueState } from "../../glyphs/StateGlyph";
import { Picker } from "./Picker";
import { DuplicateOfPicker } from "./DuplicateOfPicker";
import { stateOptions } from "./stateOptions";

/** SW.4's hand move. `reason` is "" except for a hand-set done on a serving issue (task 48). */
export function handMove(to: IssueState, reason = ""): StateMove {
  return { to, cause: "hand", reason, landCommit: "", evidence: [], evidenceCommit: "" };
}

export function StatePicker(props: {
  issueId: string;
  state: IssueState;
  serving: boolean;
  onClose(): void;
}): JSX.Element {
  const { issueId, state, serving, onClose } = props;
  const [dupOf, setDupOf] = useState(false);
  if (dupOf) return <DuplicateOfPicker issueId={issueId} onClose={onClose} />;
  const items = stateOptions(state, serving).map((o) => ({
    id: o.state,
    label: o.label,
    note: o.note,
    lead: <StateGlyph state={o.state} size={14} />,
    selected: o.current,
    // Task 48 turns needs_reason into its reason step; until then it is shown and never sent.
    disabled: o.kind === "fact" || o.kind === "needs_reason",
  }));
  const onChoose = (id: string) => {
    const to = id as IssueState;
    if (to === state) return;
    if (to === "duplicate") { setDupOf(true); return; }
    mutate({ kind: "setState", issueId, move: handMove(to) });
  };
  return (
    <Picker label="Change state" items={items} onChoose={onChoose}
      onClose={() => { if (!dupOf) onClose(); }} />
  );
}
```

`Picker` calls `onClose` after a single choice; the wrapper swallows it when *Duplicate* was chosen,
so the second picker opens in place of the first.

#### Interfaces — complete: `desktop/src/components/issue/pickers/DuplicateOfPicker.tsx`

```tsx
import type { JSX } from "react";
import { mutate } from "../../../work/replica/ops";
import { search, useWork } from "../../../work/replica/replica";
import { handMove } from "./StatePicker";
import { Picker } from "./Picker";

/** Duplicate of which issue. One batch, so one undo chip (task 56): the relation, then the move. */
export function DuplicateOfPicker(props: { issueId: string; onClose(): void }): JSX.Element {
  const { issueId, onClose } = props;
  const others = useWork((r) => search(r, "").filter((h) => h.id !== issueId));
  const items = others.map((h) => ({ id: h.id, label: `${h.alias} ${h.title}` }));
  const onChoose = (otherId: string) =>
    mutate([
      { kind: "relation", issueId, add: { kind: "duplicates", issueId: otherId } },
      { kind: "setState", issueId, move: handMove("duplicate") },
    ]);
  return <Picker label="Duplicate of" items={items} onChoose={onChoose} onClose={onClose} />;
}
```

The item's `id` is the other issue's `is-` id and its label is the alias and title for reading
only, so filtering by `EP-150` works and nothing is keyed on it. `search(r, q)` is SW.16's
search (task 58) returning `{id, alias, title}` hits; its signature here is **UNVERIFIED** until
task 58's part is written, and is listed in *Gaps*.

