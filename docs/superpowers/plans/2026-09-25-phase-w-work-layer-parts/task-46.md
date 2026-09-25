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
- `desktop/src/components/issue/issueData.ts` — the one file here that reads the replica's shape
  (new; not yet in the File map)
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
import { useEffect, useId, useMemo, useRef, useState, type JSX, type KeyboardEvent, type ReactNode } from "react";
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
  // A passive effect, not autoFocus: Popover's layout effect focuses its own box first
  // (`common/Popover.tsx:39`), and this runs after it, so typing filters at once.
  const q = useRef<HTMLInputElement>(null);
  useEffect(() => q.current?.focus(), []);
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
        ref={q} className="iv-picker-q" value={query} aria-label={`${label}: filter`}
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

#### Interfaces — complete: `desktop/src/components/issue/pickers/PriorityPicker.tsx`

```tsx
import type { JSX } from "react";
import { mutate } from "../../../work/replica/ops";
import { StateGlyph, type IssueState, type Priority } from "../../glyphs/StateGlyph";
import { Picker } from "./Picker";

export const PRIORITY_ORDER: readonly Priority[] = ["none", "urgent", "high", "medium", "low"];
export const PRIORITY_LABEL: Readonly<Record<Priority, string>> = {
  none: "No priority", urgent: "Urgent", high: "High", medium: "Medium", low: "Low",
};

/** Each option draws the issue's own glyph with that notch: priority is ink, never a colour
 *  (05 §4.4 work rule 2). No class, style or colour is set on an option for its priority. */
export function PriorityPicker(props: {
  issueId: string; state: IssueState; priority: Priority; onClose(): void;
}): JSX.Element {
  const { issueId, state, priority, onClose } = props;
  const items = PRIORITY_ORDER.map((p) => ({
    id: p, label: PRIORITY_LABEL[p], selected: p === priority,
    lead: <StateGlyph state={state} priority={p} size={14} />,
  }));
  const onChoose = (id: string) => {
    if (id !== priority) mutate({ kind: "setPriority", issueId, priority: id as Priority });
  };
  return <Picker label="Change priority" items={items} onChoose={onChoose} onClose={onClose} />;
}
```

#### Interfaces — complete: `desktop/src/components/issue/pickers/AssigneePicker.tsx`

```tsx
import type { JSX } from "react";
import type { IssueHead, User } from "../../../api/work";
import { mutate } from "../../../work/replica/ops";
import { Picker } from "./Picker";

/** Decision 2: someone else's lane is bound to this issue, so assigning is a request. */
export function othersLiveWork(head: IssueHead, me: User): boolean {
  return (head.state === "started" || head.state === "in_review")
    && head.assignee !== null && head.assignee.id !== me.id;
}

/** One op for a choice of person. `null` is *No assignee*. The `I` key (task 53) calls it with me. */
export function assign(head: IssueHead, me: User, to: User | null): void {
  if ((head.assignee?.id ?? null) === (to?.id ?? null)) return;
  if (othersLiveWork(head, me)) {
    if (to === null) return;                     // not offered: see the note below
    mutate({ kind: "requestReassign", issueId: head.id, toUserId: to.id });
    return;
  }
  mutate({ kind: "setAssignee", issueId: head.id, assigneeId: to?.id ?? null });
}

export const assignToMe = (head: IssueHead, me: User): void => assign(head, me, me);

/** People only (OD-16): `people` is the team's members (decision 5). Nothing else is listed. */
export function AssigneePicker(props: {
  head: IssueHead; me: User; people: readonly User[]; onClose(): void;
}): JSX.Element {
  const { head, me, people, onClose } = props;
  const asking = othersLiveWork(head, me);
  const owner = head.assignee?.displayName.split(" ")[0] ?? "";
  const note = asking ? `asks ${owner} to hand over` : "";
  const byId = new Map(people.map((p) => [p.id, p]));
  const rest = people.filter((p) => p.id !== me.id)
    .sort((a, b) => a.displayName.localeCompare(b.displayName));
  const items = [
    { id: me.id, label: "Assign to me", note, selected: head.assignee?.id === me.id },
    { id: "", label: "No assignee", selected: head.assignee === null, disabled: asking,
      note: asking ? `${owner}'s agents are on it` : "" },
    ...rest.map((p) => ({ id: p.id, label: p.displayName, note, selected: head.assignee?.id === p.id })),
  ];
  const onChoose = (id: string) => {
    const who = id === "" ? null : id === me.id ? me : byId.get(id);
    if (who !== undefined) assign(head, me, who);  // an id not in the list sends nothing
  };
  return (
    <Picker label={asking ? `Ask ${owner} to hand over` : "Assign"} items={items}
      onChoose={onChoose} onClose={onClose} />
  );
}
```

*No assignee* is not a request while someone else's lane is bound: clearing the assignee of a
person's running work is the same act as taking it, and the only way to take it is to ask (§3.7
rule 2). The first name is the display name's first word, as the scope's *Ask Dylan to hand
over* reads; a display name with one word reads whole.

#### Interfaces — complete: `desktop/src/components/issue/pickers/LabelPicker.tsx`

```tsx
import type { JSX } from "react";
import type { Label } from "../../../api/work";
import { mutate } from "../../../work/replica/ops";
import { Picker } from "./Picker";

/** The team's labels (task 11). Multi-select: each choice is one op, and the picker stays open. */
export function LabelPicker(props: {
  issueId: string; labels: readonly string[]; teamLabels: readonly Label[]; onClose(): void;
}): JSX.Element {
  const { issueId, labels, teamLabels, onClose } = props;
  const on = new Set(labels);
  const items = [...teamLabels]
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((l) => ({
      id: l.id, label: l.name, selected: on.has(l.id),
      lead: <span className="iv-label-dot" data-color={l.color} />,
    }));
  const onChoose = (id: string) =>
    mutate(on.has(id)
      ? { kind: "labels", issueId, add: [], remove: [id] }
      : { kind: "labels", issueId, add: [id], remove: [] });
  return <Picker label="Labels" multi items={items} onChoose={onChoose} onClose={onClose} />;
}
```

A label's `color` is a token name, never a hex (task 03, `task-03.md:338`); `issue.css` maps each
name task 11 allows to its token under `.iv-label-dot[data-color="…"]`, so no inline style
carries a colour. A label is the one place on the head where colour appears, and it is paired with
the label's name everywhere, never alone.

#### Interfaces — complete: `desktop/src/components/issue/AuthorClass.tsx`

```tsx
// Drawn for PanaceaApp from no third-party source (OD-11). Two marks, 12px, in currentColor:
// a person is a solid upright bar; an agent is a bracket open to the right, "text held apart".
import type { JSX } from "react";

export type Author = "H" | "A";
export const AUTHOR_WORD: Readonly<Record<Author, string>> = { H: "person", A: "agent" };
export const AUTHOR_CLASS: Readonly<Record<Author, string>> = { H: "text·H", A: "text·A" };

const PATH: Readonly<Record<Author, string>> = {
  H: "M5 2.5h2v7H5z",
  A: "M8.5 2.5H4.5v7h4M4.5 2.5v7",
};

/** The mark and the word, always together (never colour alone, 05 §4.4 C3). `word={false}` is
 *  for the editor gutter, where the passage's accessible name carries the word instead. */
export function AuthorClass(props: { author: Author; word?: boolean }): JSX.Element {
  const { author, word = true } = props;
  return (
    <span className="iv-author" data-author={author}>
      <svg width={12} height={12} viewBox="0 0 12 12" aria-hidden="true" focusable="false">
        <path d={PATH[author]} className={author === "H" ? "iv-author-fill" : "iv-author-ring"} />
      </svg>
      {word ? <span className="iv-author-word">{AUTHOR_CLASS[author]} · {AUTHOR_WORD[author]}</span> : null}
    </span>
  );
}
```

The word is the class and its gloss, `text·A · agent`, in `--t-fact` mono: `text·A` is a fact a
reader can search for, and `agent` says what it means to someone who has never seen the notation.

#### Interfaces — complete: `desktop/src/components/issue/issueData.ts`

```ts
// The ONLY file in components/issue/ that reads the replica's shape. Every other file takes plain
// records, so a rename in tasks 55-57 changes this file alone.
import type { IssueHead, IssueRevision, Label, User } from "../../api/work";
import { useWork } from "../../work/replica/replica";

export interface IssueViewData {
  head: IssueHead;
  headRevision: IssueRevision;          // the head's content: the title, and serving or not
  teamLabels: readonly Label[];
  people: readonly User[];              // the team's members (decision 5)
}

export function useIssueViewData(issueId: string): IssueViewData | null {
  return useWork((r) => {
    const rec = r.issue(issueId);                    // IssueWithRevision | undefined
    if (!rec) return null;
    const teamId = r.teamOfProject(rec.head.projectId);
    return {
      head: rec.head,
      headRevision: rec.revision,
      teamLabels: teamId ? r.labels(teamId) : [],
      people: teamId ? r.members(teamId).map((m) => m.user) : [],
    };
  });
}
```

`r.issue`, `r.teamOfProject`, `r.labels` and `r.members` are the reads this view needs from the
replica; task 55's scope names only `get(id)` and `byProject(projectId)`, so these four are a gap
for tasks 55-57 (named below). `useWork` must compare the selection structurally, or this
selector's fresh object re-renders on every event; task 55's test *re-renders only when its
selection changes* is where that is held (**UNVERIFIED** until its part says how).

#### Interfaces — complete: `desktop/src/components/issue/IssueView.tsx`

```tsx
import type { JSX, ReactNode } from "react";
import type { User } from "../../api/work";
import { ViewHeader, type Route } from "../common/ViewHeader";
import { IssueHead } from "./IssueHead";
import { useIssueViewData } from "./issueData";
import "./issue.css";

/** The one issue view (05 §4.1). `revision` is the revision a chip or link named, or null for
 *  the head; task 47 honours it. The slots are filled by tasks 47, 48, 51 and 52, in that order. */
export function IssueView(props: {
  issueId: string;
  revision: number | null;
  me: User;
  route: Route | null;
  slots?: { body?: ReactNode; relations?: ReactNode; activity?: ReactNode };
}): JSX.Element {
  const { issueId, me, route, slots = {} } = props;
  const data = useIssueViewData(issueId);
  if (data === null) {
    return <section className="iv" aria-label="Issue"><p className="iv-empty">This issue is not in your replica.</p></section>;
  }
  const { head, headRevision, teamLabels, people } = data;
  return (
    <section className="iv" aria-label={`Issue ${head.alias}`} data-issue={head.id}>
      <ViewHeader
        title={headRevision.content.title.text}
        route={route}
        badge={<span className="iv-alias iv-fact" data-fact="alias">{head.alias}</span>}
      />
      <IssueHead head={head} title={headRevision.content.title} serving={headRevision.content.serves.length > 0}
        me={me} people={people} teamLabels={teamLabels} />
      {slots.body ?? null}
      {slots.relations ?? null}
      {slots.activity ?? null}
    </section>
  );
}
```

The empty case names what is true (the record is not in memory) and draws no spinner: the replica
is either ready or resyncing, and task 57 shows that once, for the window. `data-issue` carries
the `is-` id; the alias appears only as text.

#### Interfaces — complete: `desktop/src/components/issue/IssueHead.tsx`

```tsx
import { useState, type JSX, type MouseEvent } from "react";
import type { AuthoredText, IssueHead as Head, Label, User } from "../../api/work";
import { Popover } from "../common/Popover";
import { StateGlyph } from "../glyphs/StateGlyph";
import { AuthorClass } from "./AuthorClass";
import { AssigneePicker } from "./pickers/AssigneePicker";
import { LabelPicker } from "./pickers/LabelPicker";
import { PRIORITY_LABEL, PriorityPicker } from "./pickers/PriorityPicker";
import { StatePicker } from "./pickers/StatePicker";
import { STATE_LABEL } from "./pickers/stateOptions";

type Field = "state" | "priority" | "assignee" | "labels";

/** "14:02": a clock time in mono, never "N ago" as the primary (lib/time.ts:1-4). */
export const clock = (iso: string): string =>
  new Date(iso).toLocaleTimeString("en-AU", { hour: "2-digit", minute: "2-digit", hour12: false });

export function IssueHead(props: {
  head: Head; title: AuthoredText; serving: boolean;
  me: User; people: readonly User[]; teamLabels: readonly Label[];
}): JSX.Element {
  const { head, title, serving, me, people, teamLabels } = props;
  const [open, setOpen] = useState<{ field: Field; at: DOMRect } | null>(null);
  const close = () => setOpen(null);
  const opener = (field: Field) => (e: MouseEvent<HTMLButtonElement>) =>
    setOpen({ field, at: e.currentTarget.getBoundingClientRect() });
  const btn = (field: Field) => ({
    type: "button" as const, className: "iv-field", "aria-haspopup": "dialog" as const,
    "aria-expanded": open?.field === field, onClick: opener(field),
  });
  const names = new Map(teamLabels.map((l) => [l.id, l.name]));
  const req = head.reassignRequest;               // task 65's added field; null when none
  return (
    <div className="iv-head">
      <div className="iv-fields" role="group" aria-label="Issue fields">
        <button {...btn("state")} aria-label={`State: ${STATE_LABEL[head.state]}`}>
          <StateGlyph state={head.state} priority={head.priority}
            notProven={head.state === "done" && head.proof === "not_proven"} size={16} />
          <span>{STATE_LABEL[head.state]}</span>
        </button>
        <button {...btn("priority")} aria-label={`Priority: ${PRIORITY_LABEL[head.priority]}`}>
          <span>{PRIORITY_LABEL[head.priority]}</span>
        </button>
        <button {...btn("assignee")} aria-label={`Assignee: ${head.assignee?.displayName ?? "none"}`}>
          <span>{head.assignee?.displayName ?? "No assignee"}</span>
        </button>
        <button {...btn("labels")} aria-label="Labels">
          {head.labels.length === 0 ? <span>No labels</span>
            : head.labels.map((id) => <span key={id} className="iv-label">{names.get(id) ?? id}</span>)}
        </button>
      </div>
      <p className="iv-facts">
        <AuthorClass author={title.author} />
        <span className="iv-fact" data-fact="key">{head.headKey}</span>
        <span className="iv-fact" data-fact="created">
          opened <time dateTime={head.createdAt}>{clock(head.createdAt)}</time> by {head.createdBy.displayName}
        </span>
        <span className="iv-fact" data-fact="updated">
          changed <time dateTime={head.updatedAt}>{clock(head.updatedAt)}</time>
        </span>
      </p>
      {req ? (
        <p className="iv-request" role="status">
          reassign requested by {req.by.displayName} to {req.to.displayName} ·{" "}
          <time className="iv-fact" data-fact="requested" dateTime={req.at}>{clock(req.at)}</time>
        </p>
      ) : null}
      <Popover anchor={open?.at ?? null} onClose={close}>
        {open?.field === "state" ? <StatePicker issueId={head.id} state={head.state} serving={serving} onClose={close} /> : null}
        {open?.field === "priority" ? <PriorityPicker issueId={head.id} state={head.state} priority={head.priority} onClose={close} /> : null}
        {open?.field === "assignee" ? <AssigneePicker head={head} me={me} people={people} onClose={close} /> : null}
        {open?.field === "labels" ? <LabelPicker issueId={head.id} labels={head.labels} teamLabels={teamLabels} onClose={close} /> : null}
      </Popover>
    </div>
  );
}
```

`head.reassignRequest` is `{ to: User; by: User; at: string } | null`, the field task 65's scope
adds to `IssueHead` (`tasks-61-80.md:190`). This part is written before task 65, so SW.15's desktop
`IssueHead` must carry it as nullable from task 37 on, always `null` until the fake or the Hub sets
it; a gap for task 37, named below. The head shows no `--need` for it: the request is the
**owner's** decision, and the owner sees it on the Desk; here it is a fact line.

The facts line is **all mono and all `--ink-2`**: the head key `issue:is-…@3` (the one identity a
reader can copy), the times with who, and the author class. The test below holds this by class
and by reading `issue.css` as text, not by computed style (jsdom computes no fonts), the way
`StatusDot.test.tsx:4-7` already imports `common.css?raw`.

#### Interfaces — complete: `desktop/src/components/issue/issue.css`, the head's rules

```css
/* issue.css owns .iv-* and no other prefix. Facts: Commit Mono at --t-fact, --ink-2 or stronger,
   never the third ink (SW.11; tokens.css:47 gives it 3.43:1; a test holds that this file never
   names it, so this comment does not either). No colour codes a priority. */
.iv { display: grid; grid-template-rows: auto auto 1fr; min-height: 0; }
.iv-head { display: grid; gap: var(--s-2); padding: 0 var(--s-4) var(--s-3); }
.iv-fields { display: flex; flex-wrap: wrap; gap: var(--s-1); }
.iv-field {
  display: inline-flex; align-items: center; gap: var(--s-1);
  min-height: var(--row-h); padding: 0 var(--s-2); border-radius: var(--r-sm);
  font: inherit; font-family: var(--sans); color: var(--ink); background: transparent;
  box-shadow: 0 0 0 1px var(--ring);
  transition: background-color var(--dur-fast) var(--ease);
}
.iv-field:hover { background: var(--hover); }
.iv-field:focus-visible { outline: 2px solid var(--accent-ring); outline-offset: 1px; }
.iv-field[aria-expanded="true"] { background: var(--hover); }
.iv-fact, .iv-alias { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2);
  font-variant-numeric: tabular-nums; }
.iv-facts { display: flex; flex-wrap: wrap; gap: var(--s-3); margin: 0; }
.iv-request { margin: 0; font-family: var(--sans); color: var(--ink); }
.iv-label { font-family: var(--sans); }
.iv-label-dot { inline-size: 8px; block-size: 8px; border-radius: 50%; background: currentColor; }
.iv-author { display: inline-flex; align-items: center; gap: 4px; color: var(--ink-2); }
.iv-author-word { font-family: var(--mono); font-size: var(--t-fact); }
.iv-author-fill { fill: currentColor; }
.iv-author-ring { fill: none; stroke: currentColor; stroke-width: 1.25; }
.iv-picker { display: grid; min-inline-size: 240px; border-radius: var(--r-md); background: var(--surface-2);
  box-shadow: 0 0 0 1px var(--ring), var(--shadow-2); }
.iv-picker-q { font: inherit; font-family: var(--sans); padding: var(--s-2); border: 0; background: transparent; color: var(--ink); }
.iv-picker-list { list-style: none; margin: 0; padding: var(--s-1); max-block-size: 320px; overflow: auto; }
.iv-picker-opt { display: grid; grid-template-columns: 20px 1fr; align-items: center; min-block-size: var(--row-h);
  padding: 0 var(--s-2); border-radius: var(--r-sm); color: var(--ink); }
.iv-picker-opt[data-active] { background: var(--hover); }
.iv-picker-opt[aria-disabled="true"] { cursor: default; }
.iv-picker-note { grid-column: 2; font-size: var(--t-small); color: var(--ink-2); }
.iv-empty { padding: var(--s-4); color: var(--ink-2); }
```

Every token named here is **UNVERIFIED** against task 26's final list except `--ink-2`, `--ink-3`,
`--t-fact`, `--row-h`, `--mono` and `--sans` (SW.11): `--s-*`, `--r-*`, `--ring`, `--hover`,
`--surface-2`, `--shadow-2`, `--t-small`, `--dur-fast` and `--ease` are read on the sync base and
renamed to task 26's words if they differ. The fact-state option is not greyed: a disabled look in
`--ink-3` would put a fact (what the state waits on) in the one ink that may not carry one. The
label dot's `currentColor` is set per `[data-color]` by task 11's token list, appended here when
task 11's allowed names are read (**UNVERIFIED**: the list is task 11's).

#### Tests to write first — `desktop/src/components/issue/pickers/stateOptions.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { WORK_STATES } from "../../../test/workFixtures";
import { stateOptions } from "./stateOptions";

describe("stateOptions (SW.4 as the picker reads it)", () => {
  it("never offers a fact state as a choice, from any state, serving or not", () => {
    for (const from of WORK_STATES) for (const serving of [false, true]) {
      const facts = stateOptions(from, serving).filter((o) => o.kind === "fact").map((o) => o.state);
      expect(facts).toEqual(["started", "in_review", "landed"]);
    }
  });
  it("names the fact each fact state waits on", () => {
    for (const o of stateOptions("todo", false).filter((x) => x.kind === "fact")) {
      expect(o.note).toMatch(/^moves when /);
    }
  });
  it("asks a reason for done only on an issue that serves a criterion", () => {
    expect(stateOptions("landed", false).find((o) => o.state === "done")?.kind).toBe("choose");
    expect(stateOptions("landed", true).find((o) => o.state === "done")?.kind).toBe("needs_reason");
  });
  it("offers triage, backlog, todo and cancelled as hand moves, and duplicate through its picker", () => {
    const kinds = Object.fromEntries(stateOptions("todo", true).map((o) => [o.state, o.kind]));
    expect(kinds).toMatchObject({ triage: "choose", backlog: "choose", todo: "choose",
      cancelled: "choose", duplicate: "pick_duplicate" });
  });
  it("lists all nine states, in order, once each", () => {
    expect(stateOptions("todo", false).map((o) => o.state)).toEqual(WORK_STATES);
  });
});
```

| Test | Mutant it kills |
|---|---|
| never offers a fact state | a picker that fakes a fact (`started` as `choose`) |
| names the fact each waits on | a fact state listed with no reason why it cannot be picked |
| asks a reason for done only when serving | `done` always free (a reasonless done on a serving issue), or `done` never free |
| offers the hand moves | `triage` dropped (task 10 decision 1 allows it); duplicate sent with no *of which* |
| lists all nine | a state hidden instead of shown as a fact |

#### Tests to write first — `desktop/src/components/issue/pickers/pickers.test.tsx`, part 1

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead } from "../../../test/workFixtures";
import issueCss from "../issue.css?raw";
import { mutate } from "../../../work/replica/ops";
import { StatePicker } from "./StatePicker";
import { PriorityPicker } from "./PriorityPicker";
import { AssigneePicker } from "./AssigneePicker";

vi.mock("../../../work/replica/ops", () => ({ mutate: vi.fn(() => ({ undo: vi.fn(), settled: Promise.resolve("applied") })) }));
vi.mock("../../../work/replica/replica", () => ({ useWork: vi.fn(() => []), search: vi.fn(() => []) }));

const sent = () => vi.mocked(mutate).mock.calls.map((c) => c[0]);
const head = (over = {}) => ({ ...makeIssueHead(over), reassignRequest: null });
beforeEach(() => vi.mocked(mutate).mockClear());

describe("StatePicker", () => {
  it("sends nothing when a fact state is clicked or entered", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="todo" serving={false} onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Started/ }));
    await user.type(screen.getByRole("combobox"), "in review{Enter}");
    expect(sent()).toEqual([]);
    // No jest-dom in this repo (test/setup.ts): attributes are read with getAttribute.
    expect(screen.getByRole("option", { name: /Started/ }).getAttribute("aria-disabled")).toBe("true");
  });
  it("sends one hand move for a chosen state", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="todo" serving={false} onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Backlog/ }));
    expect(sent()).toEqual([{ kind: "setState", issueId: WORK_IDS.ep142,
      move: { to: "backlog", cause: "hand", reason: "", landCommit: "", evidence: [], evidenceCommit: "" } }]);
  });
  it("keeps done shown and unsendable on a serving issue until task 48", async () => {
    const user = userEvent.setup();
    render(<StatePicker issueId={WORK_IDS.ep142} state="landed" serving onClose={() => {}} />);
    const done = screen.getByRole("option", { name: /Done/ });
    expect(within(done).getByText("needs a reason")).toBeTruthy();
    await user.click(done);
    expect(sent()).toEqual([]);
  });
});
```

#### Tests to write first — `pickers.test.tsx`, part 2

```tsx
describe("PriorityPicker", () => {
  it("draws urgent in ink: no signal colour on an option, none in issue.css", () => {
    render(<PriorityPicker issueId={WORK_IDS.ep142} state="todo" priority="none" onClose={() => {}} />);
    const urgent = screen.getByRole("option", { name: /Urgent/ });
    for (const el of [urgent, ...Array.from(urgent.querySelectorAll("*"))]) {
      expect(`${el.getAttribute("class") ?? ""} ${el.getAttribute("style") ?? ""}`).not.toMatch(/need|fail|warn|danger|red|urgent/i);
    }
    expect(issueCss).not.toMatch(/--need|--fail|--warn/);
    expect(urgent.querySelector("[data-glyph]")).not.toBeNull();   // the notch, drawn by StateGlyph
  });
});

describe("AssigneePicker", () => {
  const people = [DYLAN, ALICE];
  it("lists people only: me first, then no one, then the team", () => {
    render(<AssigneePicker head={head()} me={DYLAN} people={people} onClose={() => {}} />);
    const opts = screen.getAllByRole("option");
    expect(opts.map((o) => o.textContent)).toEqual(["Assign to me", "No assignee", "Alice Example"]);
    for (const o of opts) expect(o.textContent).not.toMatch(/agent|session/i);
  });
  it("assigns at once when no one else's lane is bound", async () => {
    const user = userEvent.setup();
    render(<AssigneePicker head={head({ state: "todo", assignee: DYLAN })} me={ALICE} people={people} onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Assign to me/ }));
    expect(sent()).toEqual([{ kind: "setAssignee", issueId: WORK_IDS.ep142, assigneeId: ALICE.id }]);
  });
  it("asks the owner to hand over, and moves no assignee, when their lane is bound", async () => {
    const user = userEvent.setup();
    const h = head({ state: "started", assignee: DYLAN });
    render(<AssigneePicker head={h} me={ALICE} people={people} onClose={() => {}} />);
    expect(screen.getByRole("dialog", { name: "Ask Dylan to hand over" })).toBeTruthy();
    await user.click(screen.getByRole("option", { name: /Assign to me/ }));
    await user.click(screen.getByRole("option", { name: /No assignee/ }));
    expect(sent()).toEqual([{ kind: "requestReassign", issueId: WORK_IDS.ep142, toUserId: ALICE.id }]);
  });
  it("treats my own started issue as mine to reassign", async () => {
    const user = userEvent.setup();
    render(<AssigneePicker head={head({ state: "started", assignee: DYLAN })} me={DYLAN} people={people} onClose={() => {}} />);
    await user.click(screen.getByRole("option", { name: /Alice Example/ }));
    expect(sent()).toEqual([{ kind: "setAssignee", issueId: WORK_IDS.ep142, assigneeId: ALICE.id }]);
  });
});
```

| Test | Mutant it kills |
|---|---|
| sends nothing for a fact state | a picker that fakes a fact, by click or by Enter |
| one hand move | a patch instead of `StateMove`; a cause other than `hand` |
| done unsendable on a serving issue | a reasonless hand-done shipped before task 48 |
| urgent in ink | colour-coded priority (`--need` or a red on urgent) |
| people only | an agent or session as assignee (`OD-16`) |
| assigns at once | a decide for every reassign (task 65's mutant) |
| asks the owner | steering another person's agent; *No assignee* as a back door |
| my own started issue | a request sent to myself |

`AssigneePicker`'s *Assign to me* note reads *asks Dylan to hand over* in the third test, so
`name: /Assign to me/` still matches: the note is inside the option.

#### Tests to write first — `desktop/src/components/issue/IssueHead.test.tsx`

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { ALICE, DYLAN, WORK_IDS, makeIssueHead, makeIssueRevision, makeLabel } from "../../test/workFixtures";
import issueCss from "./issue.css?raw";
import { useIssueViewData } from "./issueData";
import { IssueView } from "./IssueView";

vi.mock("./issueData", () => ({ useIssueViewData: vi.fn() }));
vi.mock("../../work/replica/ops", () => ({ mutate: vi.fn() }));
vi.mock("../../work/replica/replica", () => ({ useWork: vi.fn(() => []), search: vi.fn(() => []) }));

function show(over: Record<string, unknown> = {}) {
  vi.mocked(useIssueViewData).mockReturnValue({
    head: { ...makeIssueHead(over), reassignRequest: null, ...over } as never,
    headRevision: makeIssueRevision(), teamLabels: [makeLabel()], people: [DYLAN, ALICE],
  });
  return render(<IssueView issueId={WORK_IDS.ep142} revision={null} me={DYLAN} route="au" />);
}

/** Every attribute value in the tree except visible text: ids, keys, aria-*, data-*. */
const attrs = (root: HTMLElement) => Array.from(root.querySelectorAll("*")).flatMap((el) =>
  Array.from(el.attributes).map((a) => a.value));

describe("IssueView head", () => {
  it("uses the alias as display only: a new project key changes text and no identity", () => {
    const { container, rerender } = show({ alias: "EP-142" });
    const before = attrs(container);
    expect(screen.getByText("EP-142")).toBeTruthy();
    vi.mocked(useIssueViewData).mockReturnValue({
      head: { ...makeIssueHead({ alias: "EPX-142" }), reassignRequest: null } as never,
      headRevision: makeIssueRevision(), teamLabels: [makeLabel()], people: [DYLAN, ALICE],
    });
    rerender(<IssueView issueId={WORK_IDS.ep142} revision={null} me={DYLAN} route="au" />);
    expect(screen.getByText("EPX-142")).toBeTruthy();
    expect(attrs(container).filter((v) => !v.includes("EP")))       // aria-label "Issue EP-142" is text
      .toEqual(before.filter((v) => !v.includes("EP")));
    expect(container.querySelector("[data-issue]")?.getAttribute("data-issue")).toBe(WORK_IDS.ep142);
  });
  it("puts no fact in --ink-3 and every fact in mono", () => {
    const { container } = show();
    for (const f of Array.from(container.querySelectorAll("[data-fact]"))) {
      expect(f.className).toMatch(/\biv-fact\b/);
      expect(f.className).not.toMatch(/ink-3|muted|faint/);
    }
    expect(issueCss).not.toContain("--ink-3");
    expect(/\.iv-fact, \.iv-alias \{[^}]*font-family: var\(--mono\)[^}]*color: var\(--ink-2\)/.test(issueCss)).toBe(true);
  });
  it("shows the route badge from its prop, never a default", () => {
    show();
    expect(screen.getByLabelText("Inference route: AU")).toBeTruthy();
  });
  it("reads reassign requested by Alice while the owner decides", () => {
    show({ reassignRequest: { to: ALICE, by: ALICE, at: "2026-09-25T01:04:00Z" }, state: "started" });
    expect(screen.getByRole("status").textContent).toMatch(/^reassign requested by Alice Example/);
    expect(screen.getByLabelText(/Assignee: Dylan Example/)).toBeTruthy();
  });
  it("shows the title's author class as a word", () => {
    show();
    expect(screen.getByText(/text·H · person/)).toBeTruthy();
  });
  it("makes no prediction", () => {
    const { container } = show({ state: "in_review" });
    expectNoPrediction(container);
  });
});
```

| Test | Mutant it kills |
|---|---|
| alias as display only | the alias used as a React key, an id, `aria-controls` or `data-issue` |
| no fact in `--ink-3` | an unreadable fact (SW.11); a fact in sans |
| route badge from its prop | a hard-coded `Global` |
| reassign requested | the assignee moved at once on a request; the request hidden |
| author class as a word | colour-only authorship on the title |
| no prediction | a `%`, a bar or an ETA on the head (`OD-27`) |

The alias test filters out attribute values containing `EP` because the view's `aria-label` is
`Issue EP-142`, which is text a screen reader speaks, not identity; every other attribute must be
byte-identical across the key change.

#### Steps

- [ ] **Step 1: read before writing.** `docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md`,
  `prototype/work-layer.html` (the issue view), task 39's review file for the owner's words on the
  issue view, and on the sync base: `src/api/work.ts` (task 37), `work/replica/{replica,ops}.ts`
  (tasks 55-56, or task 40's fixture-backed `useWork` if they have not landed), and task 26's
  token names. Rename anything in this part that differs, in this part's files only.
- [ ] **Step 2: the three test files first**, as above; run them and see them fail on the missing
  modules: `npm --prefix desktop test -- issue` exits non-zero.
- [ ] **Step 3: `stateOptions.ts`**, then its test passes alone.
- [ ] **Step 4: `Picker.tsx` and `issue.css`**, then the four pickers and `DuplicateOfPicker.tsx`.
- [ ] **Step 5: `AuthorClass.tsx`, `issueData.ts`, `IssueHead.tsx`, `IssueView.tsx`.**
- [ ] **Step 6: run the tests and the typecheck** below; fix until both are clean.
- [ ] **Step 7: commit** by path, one commit per step 3-5, each message ending with the
  `Co-Authored-By` line. Never `git add -A`.

#### Acceptance

```bash
npm --prefix desktop test -- issue/pickers issue/IssueHead
# expect: Test Files 3 passed (3); Tests 19 passed (19)
npm --prefix desktop run typecheck
# expect: exit 0, no output from tsc
grep -rn "ink-3" desktop/src/components/issue/ | wc -l
# expect: 0
grep -rnE "key=\{[^}]*alias" desktop/src/components/issue/ | wc -l
# expect: 0 (no React key built from an alias)
```

The test count is this part's: 5 in `stateOptions.test.ts`, 8 in `pickers.test.tsx`, 6 in
`IssueHead.test.tsx`. The owner's eyes are the visual acceptance (`CLAUDE.md`); nothing here
claims the head *looks* right.

#### What this task does NOT claim

- Estimate `E` (L-16), the due date (L-37), cycle and feature fields: W2.
- Org labels and label groups (L-17's W2 part); only the team's labels are offered.
- The reason step for a hand-set `done` on a serving issue: task 48. This part leaves it shown
  and unsendable.
- The Hub route for a reassign request, or that the owner is asked: task 65 builds that on the
  fake, and names the real route **UNVERIFIED**.
- The row keys `S`, `P`, `A`, `I`, `L` and the pickers opening beside a row: task 53.
- That the Hub refuses a hand-set fact state: task 07's guard and task 10's `MoveAllowed` hold
  that; this part only never offers it.
- Any visual verdict. The prototype review (task 39) and the owner's look at this branch are it.

**Parity** (rows read in `05-design.md:107-348`). Closes **L-13** (five priorities, drawn as the
notch, `05:122`), **L-38** (*one human assignee; `I` = me*, `05:147`; the `I` key itself is task
53's), **L-17**'s team-labels part (`05:126`; org labels are W2), **L-181** (*delegate is not
assignee*, `05:348`) with task 48, the display half of **L-3** (`05:107`, with task 08), and
**L-8**'s W1 part (`05:117`: the fact states shown as facts, never chosen).

#### Gaps found while writing this part

1. **`WorkOp` needs `requestReassign`** (`{kind, issueId, toUserId}`), which task 56's closed
   union does not list; task 65 names the Hub half (`RequestReassign`) as its own gap.
2. **SW.15's `IssueHead` needs `reassignRequest: {to; by; at} | null`** from task 37 on, and task
   20's `makeIssueHead` needs it `null` by default; this part's tests add it by spread until then.
3. **The replica reads this view needs** (`issue`, `teamOfProject`, `labels`, `members`) are not
   in task 55's scope, which names only `get` and `byProject`; `issueData.ts` isolates them.
4. **No signed-in-user seam on the desktop.** `IssueView` takes `me`; its mount (task 40) needs
   `GET /v1/me` through the relay, which task 16's table must carry (**UNVERIFIED**).
5. **SW.12 has no author-class mark.** `AuthorClass.tsx` draws one here for tasks 47, 49 and 51;
   task 29's owner may move it into `MarkGlyph`.
6. **Task 46 uses SW.16 but depends only on 37 and 39.** It builds on task 40's fixture-backed
   `useWork` until tasks 55-58 land, as task 40 does; `search` (task 58) is used by
   `DuplicateOfPicker` with a signature **UNVERIFIED** here.
7. The File map's `src/components/issue/` row lists none of `pickers/*`, `issueData.ts`,
   `AuthorClass.tsx` or `issue.css`.

