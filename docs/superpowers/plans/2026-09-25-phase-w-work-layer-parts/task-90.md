### Task 90: label groups, the `Group/Label` shorthand, org labels

Stream **WD2**, about 2 focused hours (an estimate, not a measurement). Depends on task 83 (the
W2 wire types and replica kinds), and on tasks 46 (the label picker in the issue head), 53 (the
`L` binding) and 56 (`mutate`). The Hub half is task 78, whose label-group and org-label routes
this task reads. A sync-merge of `feat/hub` precedes the first green run (rule H1).

After this task labels have structure without gaining ceremony: a group may be exclusive, so
picking *Area/Upload* replaces *Area/Auth* in one act; typing `Area/Upload` in the picker finds
the label or offers to create it in that group; and an org label is visible to every team, marked
as the org's, editable only by an org admin.

**Anchors, re-read on 2026-09-27.** In this branch's parts: task 78's routes
(`task-78.md:126-129` — team and org label groups, `POST /v1/org/labels` with `groupId`), its
`LabelGroup`/`LabelPlan` codes (`task-78.md:7`). Task 83's `LabelGroup` is the scope's
(`tasks-81-100.md:92`), **UNVERIFIED** (no `task-83.md` at this writing). The `L` key is task 53's
(scope's `tasks-41-60.md`, **UNVERIFIED** in line). On the tree
`desktop/src/components/issue/` and `desktop/src/components/plan/` do **not** exist yet (checked
2026-09-27); tasks 46 and 84 create them. In the parity docs: 05 §1 rows 17 (the org half is
W2's), 18 and 19; `docs/DESIGN-SYSTEM.md`'s rule that colour is never the only signal.

#### Decisions the scope left open, made here

1. **Exclusivity is enforced in one write, not two.** Picking a second label in an exclusive group
   sends **one** op carrying both the add and the remove, so the optimistic paint shows the swap
   once and Undo restores the first label in one step. Two ops would paint a moment with two
   labels in a group that forbids two, and the Hub's refusal would then flicker the row — test 1's
   mutant.
2. **The client enforces the same rule the Hub does, and the Hub still wins.** `swapIn` is a pure
   function over the issue's labels and the group table; on `onRefused` the row rolls back and
   names the code. The client rule exists so the common case never round-trips, not so the Hub
   can be skipped.
3. **`Group/Label` never creates silently.** Typing `Area/Upload` where `Area` exists and `Upload`
   does not offers *Create Upload in Area* as a focused row the person presses Enter on. Where
   `Area` does not exist either, the row reads *Create group Area and label Upload* — one confirm
   for both, in place, no dialog. A silent create is test 2's mutant, because a typo would
   otherwise become a permanent group.
4. **The separator is the first `/` only.** `Area/Upload/v2` is label `Upload/v2` in group `Area`.
   A label name may hold a slash; a group name may not (the picker refuses a second-level split
   rather than guessing). This is a client parsing rule; the Hub stores whatever names it is sent.
5. **Ungrouped labels keep working, and sort first.** A label with no group is legal and is listed
   before the groups, under a mono heading *no group*. Nothing forces a migration into groups.
6. **Org labels are marked, not styled differently.** An org label carries a mono `org` suffix in
   the picker and on the chip. It is not a different colour, not a different shape: the marking is
   a word, so it survives `forced-colors` and colour-blind reading.
7. **A team member sees no edit control on an org label.** Not a disabled one — none. The settings
   list renders org rows read-only with the mono *org label* marker and a line naming who may
   change it (*org admins*). A control that exists and then refuses is test 3's mutant.
8. **Colour is optional and never alone.** A label chip is its name in Segoe UI Variable with an
   optional 8px swatch. Two labels with the same name cannot exist in one scope, so the name is
   always sufficient; the swatch is decoration. Task 34's `forced-colors` test covers it.
9. **The group table comes from the replica, not from the picker's props.** The picker reads
   groups through the replica so a group created in settings on another device reaches an open
   picker on the next feed apply, with no reload.

#### Files

| Path | Change |
|---|---|
| `desktop/src/work/labels.ts` (+ `.test.ts`) | new: `parseShorthand`, `swapIn`, `groupedLabels` — pure, no React |
| `desktop/src/components/plan/LabelSettings.tsx` (+ `.test.tsx`) | new: team and org labels and groups |
| `desktop/src/components/plan/labels.css` | new: the settings and chip rules (tokens only) |
| `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) | the picker gains groups, the shorthand and the `org` marker |
| `desktop/src/components/issue/LabelChip.tsx` ⚑ (task 46, if it is a file there) | the `org` suffix; else the chip markup in `IssueHead.tsx` |
| `desktop/src/api/client.ts` ⚑ (task 83) | `labelGroups`, `createLabelGroup`, `createOrgLabel`, `createOrgLabelGroup` |

Task 46 names the picker's module; if it is not `IssueHead.tsx` the edit lands in whichever file
task 46 printed, and the commit names it. Stated because task 46 is not on the tree at this
writing.

#### Interfaces — complete: `desktop/src/work/labels.ts`

```ts
import type { Label, LabelGroup } from "../api/work";        // task 83 (UNVERIFIED)

/** The first `/` splits; later ones belong to the label name (decision 4). */
export type Shorthand =
  | { kind: "plain";          name: string }                  // "Upload"
  | { kind: "grouped";        group: string; name: string };  // "Area/Upload/v2" -> Upload/v2

export function parseShorthand(typed: string): Shorthand;

export type PickerRow =
  | { kind: "label";       label: Label; group: LabelGroup | null }
  | { kind: "newLabel";    name: string; group: LabelGroup }            // confirm in place
  | { kind: "newInNewGroup"; name: string; groupName: string }          // one confirm for both
  | { kind: "heading";     text: string };                              // "no group", then each group

/** Rows for the typed query: matches first, then the create offers (never auto-run). */
export function pickerRows(
  typed: string,
  labels: readonly Label[],
  groups: readonly LabelGroup[],
): readonly PickerRow[];

/** The one op a pick produces. In an exclusive group the add carries the remove (decision 1). */
export type LabelChange = { add: string; remove: readonly string[] };

export function swapIn(
  current: readonly string[],          // label ids on the issue
  picked: Label,
  labels: readonly Label[],
  groups: readonly LabelGroup[],
): LabelChange;
```

`swapIn` returns `{add: picked.id, remove: []}` for an ungrouped or non-exclusive label, and
`{add: picked.id, remove: [theOtherLabelOfThatGroup]}` when the group is `exclusive` and the
issue already holds one of its labels. Picking a label the issue already holds returns
`{add: picked.id, remove: []}` and the caller treats an unchanged set as a no-op write — task 79
(b)'s "a patch that changes nothing writes nothing", applied on the client.

#### Interfaces — complete: the picker's rows and the chip

```tsx
// desktop/src/components/issue/IssueHead.tsx ⚑ — the picker body, replacing its flat list
{pickerRows(typed, r.labels(head.teamId), r.labelGroups(head.teamId)).map((row) => {
  if (row.kind === "heading") return <li key={`h:${row.text}`} className="lb-head lb-mono">{row.text}</li>;
  if (row.kind === "label") return (
    <li key={row.label.id}>
      <button type="button" className="lb-row" onClick={() => apply(swapIn(head.labelIds, row.label, labels, groups))}>
        {row.label.color ? <span className="lb-dot" style={{ background: row.label.color }} aria-hidden="true" /> : null}
        <span className="lb-name">{row.label.name}</span>
        {row.group ? <span className="lb-mono">{row.group.name}{row.group.exclusive ? " · one only" : ""}</span> : null}
        {row.label.scope === "org" ? <span className="lb-mono">org</span> : null}
      </button>
    </li>);
  if (row.kind === "newLabel") return (
    <li key={`n:${row.name}`}><button type="button" className="lb-row lb-new"
      onClick={() => createThenApply(row.group.id, row.name)}>
      Create <span className="lb-name">{row.name}</span> in {row.group.name} <kbd>Enter</kbd>
    </button></li>);
  return (
    <li key={`g:${row.groupName}/${row.name}`}><button type="button" className="lb-row lb-new"
      onClick={() => createGroupThenLabel(row.groupName, row.name)}>
      Create group <span className="lb-name">{row.groupName}</span> and label
      <span className="lb-name">{row.name}</span> <kbd>Enter</kbd>
    </button></li>);
})}
```

The chip on the row and in the head is the same markup: name in sans, optional swatch, mono `org`
where the label's scope is the org. No chip is distinguished by colour alone (decision 8).

#### Interfaces — complete: `LabelSettings.tsx`

```tsx
export function LabelSettings(p: {
  teamId: string;
  canEditTeam: boolean;      // team admin
  canEditOrg: boolean;       // org admin
}): JSX.Element {
  const r = useReplica();
  const groups = r.labelGroups(p.teamId);          // the team's, then the org's (task 78's order)
  const labels = r.labels(p.teamId);
  return (
    <section className="ls" aria-label="Labels">
      {groupedLabels(labels, groups).map((g) => (
        <section key={g.key} className="ls-group">
          <h3 className="ls-gname">
            {g.group ? g.group.name : <span className="ls-mono">no group</span>}
            {g.group?.exclusive ? <span className="ls-mono">one only</span> : null}
            {g.scope === "org" ? <span className="ls-mono">org</span> : null}
          </h3>
          <ul>
            {g.labels.map((l) => (
              <li key={l.id} className="ls-row">
                <LabelChip label={l} />
                {editable(l, p) ? <RenameButton label={l} /> : <span className="ls-mono">org admins</span>}
              </li>))}
          </ul>
          {editable(g.group, p) ? <NewLabelRow groupId={g.group?.id ?? null} /> : null}
        </section>))}
      {p.canEditTeam ? <NewGroupRow teamId={p.teamId} /> : null}
      {p.canEditOrg ? <NewOrgGroupRow /> : null}
    </section>
  );
}
```

`editable` is `(x, p) => x == null ? p.canEditTeam : x.scope === "org" ? p.canEditOrg : p.canEditTeam`.
Where it is false the row renders the mono *org admins* marker and **no control at all**
(decision 7). `groupedLabels` returns the ungrouped bucket first, then each group in the replica's
order, and is pure, so test 5 reads it without a DOM.

#### Tests to write first, each with the mutant it kills

1. Picking a second label in an exclusive group produces **one** op carrying the add and the
   remove, paints the swap once, and Undo restores the first in one step.
   **Mutant:** two ops, so the row briefly shows both, then the Hub refuses and it flickers.
2. Typing `Area/Upload` where `Area` exists and `Upload` does not shows a *Create Upload in Area*
   row and creates nothing until it is pressed. **Mutant:** a silent create on keystroke, so a
   typo becomes a permanent label.
3. A team member (`canEditOrg: false`) sees no rename or delete control on an org label, only the
   mono marker. **Mutant:** the control is rendered and the Hub refuses it.
4. `parseShorthand("Area/Upload/v2")` is group `Area`, label `Upload/v2`.
   **Mutant:** a second split, so the label is `Upload` and `v2` is dropped.
5. `groupedLabels` puts ungrouped labels first under *no group*, then the team's groups, then the
   org's. **Mutant:** org groups interleaved, so a team admin meets refusals mid-list.
6. Under `forced-colors: active`, two labels differing only in colour are still distinguishable
   because each renders its name. **Mutant:** the swatch is the only content of the chip
   (task 34's matcher).
7. Picking a label the issue already holds writes nothing. **Mutant:** an op that changes nothing
   still reaches the Hub and appends an event.
8. A group created in settings appears in an already-open picker after one feed apply.
   **Mutant:** the picker copied the group list into props at mount.

#### Steps

- [ ] **Step 1: Check the base.** Task 46's picker module and chip, task 78's route list, task 83's
  `LabelGroup` and `Label.scope` spelling, and whether task 56's op kinds carry a label op that can
  hold both an add and a remove (decision 1).
- [ ] **Step 2: The eight tests.** Run them: they fail.
- [ ] **Step 3: `work/labels.ts`.** Tests 4, 5 and 7 go green with no component written.
- [ ] **Step 4: The picker edit** in task 46's module (tests 1, 2, 8).
- [ ] **Step 5: `LabelSettings.tsx`** and `labels.css` (tests 3, 6).
- [ ] **Step 6: Green**, then commit by pathspec, naming task 46's file as touched.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/labels.test.ts \
  src/components/plan/LabelSettings.test.tsx src/components/issue/IssueHead.test.tsx \
  --reporter=dot 2>&1 | tail -3                                          # 8 passed
grep -rnE "disabled=\{!can(EditOrg|EditTeam)" desktop/src/components/plan/LabelSettings.tsx | wc -l   # 0
cd desktop && npx tsc --noEmit                                           # no output
```

Expected: `8 passed`, `0` (there is no disabled control; the control is absent), no output. None
has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not label views** (L-110, task 91). A label in this task is a chip and a picker row, not a
  saved list.
- **Not label colour policy.** A team may pick any colour; nothing here checks contrast between
  two labels, because colour is never the only signal.
- **Not a migration of existing labels into groups.** Ungrouped labels stay ungrouped (decision 5).
- **Not the Hub's exclusivity.** Task 78 refuses a second label in an exclusive group; this task's
  client rule exists to avoid the round-trip, not to replace it.
- **No measured picker latency** with thousands of labels; `pickerRows` is a linear scan and has
  not been timed.

#### Gaps reported to the skeleton

1. **A label op that carries a remove is assumed.** Decision 1 needs one op with `{add, remove[]}`.
   If task 56's label op is add-only, this task widens it — a change to a task-56 file, stated and
   named in the commit.
2. **`Label.scope` is assumed.** The org marker needs a field distinguishing an org label from a
   team one. Task 11's `Label` (W1) may carry only `teamId`, in which case a null `teamId` means
   the org and the marker reads that instead. Asked of task 83, which owns the desktop wire.
