### Task 91: views — the editor, save, favourites, the default, ad hoc lists, search prefixes, a person's page

Stream **WD2**, about 3 focused hours (an estimate, not a measurement). Depends on task 83 (the
W2 wire types and replica kinds), and on tasks 40 (the work lens), 55 and 56 (the replica and
`mutate`), 58 (the replica's `search(q)`, SW.16) and 89 (the shared `rank` an ordered view reads).
The Hub half is task 80; a sync-merge of `feat/hub` precedes the first green run (rule H1).

After this task a person can shape a list once, save it, favourite it, and find it again by name —
and a team admin can make one of them the team's default. The rail grows a Favourites group with
**names only**: no counts, no dots, because the rail is navigation and attention lives on the Desk.

**Anchors, re-read on 2026-09-27.** On the tree: `docs/AGENTIC-UX.md:144` — *"The rail is
navigation only. No counts, no dot clusters on projects and branches and worktrees"* (verified
2026-09-27); `desktop/src/components/rail/Rail.tsx` and `rail.css` exist (verified).
`desktop/src/components/plan/` and `desktop/src/work/` do **not** exist yet; tasks 55-58 and 84
create them, so paths under them are **UNVERIFIED until those parts merge**. Task 80's routes and
its shared filter vectors (`tasks-61-80.md:665-683`: `POST/GET/PATCH/archive` on team views,
`PUT /v1/views/{viewId}/team-default`, personal views as desktop preferences, and
`hubapi/testdata/view-filter/*.json`, which this task's evaluator runs too) — `task-80.md` is not
written at this writing, so all of it is **UNVERIFIED**. Task 83's `View` and `ViewFilter` are the
scope's (`tasks-81-100.md:92`). In the parity docs: 05 §1 rows 93-111 and L-12, and §4.1's
*"Current cycle, by state survives as a saved view (W2)"*.

#### Decisions the scope left open, made here

1. **One filter evaluator, two languages, one set of vectors.** Task 80 writes the Go evaluator
   and owns `hubapi/testdata/view-filter/*.json`. This task writes the TypeScript evaluator and
   runs **the same files by path**, not retyped, so a divergence is a red test rather than a list
   that differs between the rail and the Hub. The vectors are the contract; neither side may add a
   case without adding a vector.
2. **Personal is a file, shared is the Hub, and neither leaks.** A personal view is a desktop
   preference (task 80's rule, L-94) written to the preferences store; a shared view is a Hub
   object and is never written to disk. Two tests, one each way, because each leak is a different
   bug: a personal view on the Hub is a privacy surprise, a shared view on disk is a stale list.
3. **Favourites are names in folders, and nothing else.** The rail group renders a name per
   favourite, grouped by a folder name the person chooses. No count, no dot, no time. `AX:144` is
   the rule and test 1 is the guard, including a fixture where every favourite has unread activity.
4. **Ad hoc lists are not views and cannot be saved without naming.** `issues?ids=EP-1,EP-4`
   renders a list in id order as given, headed *4 issues, listed by id*, with *Save as view…*
   offered only as an explicit act that asks for a name. An unnamed saved list is how a views
   pane fills with *Untitled*.
5. **Search prefixes are parsed, then handed to the replica.** `@person`, `#label` and `EP-` in
   `/` are parsed on the client into structured terms; free text still goes to task 58's
   `search(q)`. Archived issues are included and each archived row carries a mono *archived*
   marker (L-12), because a search that silently hides them makes a person think the issue is gone.
6. **A person's page is their issues, and nothing about their agents until phase T lands.** The
   page shows the person's assigned and created issues. The one-line summary of their agents'
   work is rendered only when phase T's read model (TM-52) is mounted, and it is a **summary
   line**, never a transcript and never per-agent dots (OD-15). Until then the page carries no
   placeholder at all — an empty promise is worse than no promise.
7. **The team default is applied on first open only.** `PUT .../team-default` sets which view a
   person lands on the first time they open the lens on that team. A person who has since chosen
   another view keeps theirs; the default never reaches in and re-points a working person.
8. **Set as default is an admin act with its consequence written.** The control says *"New members
   of this team will land here"*, so the admin knows it is not a reset of everyone.
9. **Ordering by `rank` is the shared order, not a per-view order.** A view may order by `rank`,
   and that is task 89's team-wide order. A view does not store its own order; a second order per
   view would make *manual order* mean two things.

#### Files

| Path | Change |
|---|---|
| `desktop/src/work/views.ts` (+ `.test.ts`) | new: the filter evaluator, `groupRows`, the vector runner |
| `desktop/src/work/searchTerms.ts` (+ `.test.ts`) | new: `parsePrefixes` for `@`, `#`, `EP-` |
| `desktop/src/components/plan/ViewEditor.tsx` (+ `.test.tsx`) | new: nested AND/OR groups, grouping, ordering |
| `desktop/src/components/plan/ViewList.tsx` (+ `.test.tsx`) | new: the team's views, save, set as default |
| `desktop/src/components/plan/AdHocList.tsx` (+ `.test.tsx`) | new: `ids=` lists and *Save as view…* |
| `desktop/src/components/plan/PersonPage.tsx` (+ `.test.tsx`) | new: a person's issues; the TM-52 line when mounted |
| `desktop/src/components/plan/views.css` | new: the editor's and list's rules (tokens only) |
| `desktop/src/components/rail/Rail.tsx` ⚑ | a Favourites group, names only |
| `desktop/src/components/rail/rail.css` ⚑ | the group's rules; no count, no dot |
| `desktop/src/work/replica/search.ts` ⚑ (task 58) | prefixes and the archived marker |
| `desktop/src/work/keys/sequences.ts` ⚑ (task 53) | one row: `O F` → open a favourite |
| `desktop/src/api/client.ts` ⚑ (task 83) | `views`, `createView`, `patchView`, `archiveView`, `setTeamDefault` |

#### Interfaces — complete: `desktop/src/work/views.ts`

```ts
import type { IssueHead, View, ViewFilter } from "../api/work";   // task 83 (UNVERIFIED)

/** A nested filter tree (L-99). Mirrors task 80's Go FilterGroup field for field. */
export type FilterNode =
  | { op: "and" | "or"; nodes: readonly FilterNode[] }
  | { field: FilterField; cmp: FilterCmp; value: string | readonly string[] | null };

export type FilterField =
  | "state" | "subState" | "assigneeId" | "creatorId" | "labelId" | "priority"
  | "projectId" | "teamId" | "cycleId" | "featureId" | "criterionId"
  | "estimate" | "dueOn" | "createdAt" | "updatedAt" | "archivedAt";

export type FilterCmp = "is" | "isNot" | "in" | "notIn" | "before" | "after" | "isSet" | "isNotSet";

/** True when the head matches. Pure, total, and never throws on an unknown field:
 *  an unknown field is `false`, matching task 80's Go evaluator (vector `unknown-field.json`). */
export function matches(head: IssueHead, node: FilterNode, ctx: MatchContext): boolean;

export type MatchContext = {
  labelIdsOf(issueId: string): readonly string[];
  criterionIdsOf(issueId: string): readonly string[];
  today: string;                 // so `dueOn before today` is deterministic in tests
};

export type GroupedRows = readonly {
  key: string; label: string; rows: readonly IssueHead[];
  sub?: readonly { key: string; label: string; rows: readonly IssueHead[] }[];
}[];

/** Grouping and sub-grouping (L-102's W2 part); `order` is "rank" (task 89's shared order),
 *  "updated", "created", "priority" or "due". */
export function groupRows(
  heads: readonly IssueHead[],
  by: FilterField | null,
  subBy: FilterField | null,
  order: "rank" | "updated" | "created" | "priority" | "due",
  ctx: MatchContext,
): GroupedRows;
```

The vector runner is the whole of `views.test.ts`'s first block:

```ts
import { readdirSync, readFileSync } from "node:fs";
const dir = "../control-plane/internal/hub/hubapi/testdata/view-filter";   // task 80's files
for (const f of readdirSync(dir).filter((n) => n.endsWith(".json"))) {
  it(`filter vector ${f}`, () => {
    const v = JSON.parse(readFileSync(`${dir}/${f}`, "utf8")) as
      { filter: FilterNode; heads: IssueHead[]; ctx: unknown; matchIds: string[] };
    expect(v.heads.filter((h) => matches(h, v.filter, ctxOf(v))).map((h) => h.id))
      .toEqual(v.matchIds);
  });
}
```

If task 80's directory is absent at the time this task runs, the block skips with a named reason
and the suite still fails on the hand-written cases below — it never passes silently on zero
vectors, and test 2 asserts the count is greater than zero once task 80 has merged.

#### Interfaces — complete: `searchTerms.ts` and the search edit

```ts
export type Term =
  | { kind: "person"; text: string }     // "@priya"
  | { kind: "label";  text: string }     // "#upload"
  | { kind: "alias";  text: string }     // "EP-142", any TEAM- prefix followed by digits
  | { kind: "text";   text: string };

/** Splits on whitespace outside quotes; an alias term is any [A-Z][A-Z0-9]{1,9}-\d+. */
export function parsePrefixes(q: string): readonly Term[];
```

```ts
// desktop/src/work/replica/search.ts ⚑ (task 58) — search(q) gains, before its text match:
//   const terms = parsePrefixes(q);
//   rows = rows.filter((h) => terms.every((t) => termMatches(h, t, ctx)));
// and every row keeps `archived: h.archivedAt != null` so the list can mark it (L-12).
// An alias term short-circuits: one exact alias hit is returned alone, first.
```

#### Interfaces — complete: the rail's Favourites group

```tsx
// desktop/src/components/rail/Rail.tsx ⚑ — one group, above the projects
<nav className="rail-group" aria-label="Favourites">
  {folders.map((f) => (
    <section key={f.name} className="rail-folder">
      <h3 className="rail-folder-name">{f.name}</h3>
      <ul>
        {f.items.map((it) => (
          <li key={it.key}>
            <button type="button" className="rail-fav" onClick={() => open(it)}>{it.name}</button>
          </li>))}
      </ul>
    </section>))}
</nav>
```

A favourite is `{kind: "view" | "feature" | "issue", id, name, folder}`, held in the same
preferences store as personal views. The button's content is the name and nothing else: no badge
element exists in the markup, so a later *small* count cannot be added without editing this file
and meeting test 1.

#### Interfaces — complete: `ViewEditor.tsx` and `PersonPage.tsx` (shapes)

```tsx
export function ViewEditor(p: {
  teamId: string;
  initial: View | null;                    // null when saving the current list as a new view
  scope: "personal" | "shared";            // decision 2; the control that sets it is explicit
  canSetDefault: boolean;                  // team admin (L-105)
  onSaved(v: View): void; onClose(): void;
}): JSX.Element;

export function PersonPage(p: { personId: string }): JSX.Element {
  const r = useReplica();
  const assigned = r.issuesAssignedTo(p.personId);
  const created = r.issuesCreatedBy(p.personId);
  const agentLine = useTeamReadModel()?.summaryFor(p.personId) ?? null;   // TM-52, phase T
  return (
    <article className="pp">
      <h1>{r.personName(p.personId)}</h1>
      {agentLine ? <p className="pp-agents">{agentLine}</p> : null}
      <IssueList rows={assigned} heading="Assigned" />
      <IssueList rows={created} heading="Created" />
    </article>
  );
}
```

`useTeamReadModel` returns null when phase T is not mounted, and the page then renders no agent
element at all (decision 6). It is **UNVERIFIED until Phase T merges**; task 107 fills the line.

#### Tests to write first, each with the mutant it kills

1. A favourite with unread activity in the fixture renders its name and **no count and no dot**;
   the rendered subtree contains no numeric text node under `.rail-fav`. **Mutant:** an unread
   count on the rail (`AGENTIC-UX.md:144`).
2. Every vector in task 80's `view-filter` directory matches, and the vector count is > 0.
   **Mutant:** the TypeScript evaluator diverges from the Go one on `or` inside `and`.
3. A nested `(A or B) and C` filter matches the fixture set exactly. **Mutant:** the tree is
   flattened to one `and`, so `(A or B)` becomes `A and B` and the list quietly shrinks.
4. A personal view is never sent to the Hub (the relay spy records no call) **and** a shared view
   is never written to the preferences store (the store spy records no write). Two assertions,
   two mutants.
5. The person page for a teammate shows no transcript element and no per-agent dot, with phase T
   mounted and with it absent. **Mutant:** per-agent dots on a colleague's page (OD-15).
6. `parsePrefixes("@priya #upload EP-142 resume")` yields the four terms in kind order, and
   `EP-142` short-circuits to the exact issue. **Mutant:** `EP-142` treated as free text.
7. An archived issue appears in `/` results with a mono *archived* marker (L-12).
   **Mutant:** archived rows filtered out, so a person believes the issue is gone.
8. `expectNoPrediction` on the view list, the editor and the person page. **Mutant:** a *finishes
   by* or a completion percentage on a grouped list.
9. *Save as view…* from an ad hoc list refuses an empty name in place and saves nothing.
   **Mutant:** an *Untitled* view is created.
10. The team default applies on first open and not to a person who has chosen a view.
    **Mutant:** the default re-points an open lens when an admin changes it.

#### Steps

- [ ] **Step 1: Check the base.** Task 80's `view-filter` vectors on the sync base (if absent, note
  it and carry on), task 58's `search(q)` signature, task 55's preferences store, the rail's group
  markup, and whether phase T's read model is mounted.
- [ ] **Step 2: The ten tests.** Run them: they fail.
- [ ] **Step 3: `work/views.ts` and `work/searchTerms.ts`.** Tests 2, 3 and 6 go green with no
  component written; this is where the divergence from the Go evaluator is caught.
- [ ] **Step 4: `ViewEditor.tsx` and `ViewList.tsx`** with the personal/shared split (tests 4, 9, 10).
- [ ] **Step 5: The rail group** (test 1), **the search edit** (test 7), **`AdHocList.tsx`** and
  **`PersonPage.tsx`** (tests 5, 8), then the `O F` sequence row.
- [ ] **Step 6: Green**, then commit by pathspec, naming the rail's and task 58's files as touched.

#### Acceptance

```bash
cd desktop && npx vitest run src/work/views.test.ts src/work/searchTerms.test.ts \
  src/components/plan/ViewEditor.test.tsx src/components/plan/ViewList.test.tsx \
  src/components/plan/AdHocList.test.tsx src/components/plan/PersonPage.test.tsx \
  src/components/rail/Rail.test.tsx --reporter=dot 2>&1 | tail -3        # 10 passed
ls control-plane/internal/hub/hubapi/testdata/view-filter/*.json | wc -l  # > 0 after task 80
grep -rnE "badge|count|unread" desktop/src/components/rail/Rail.tsx | wc -l   # 0
cd desktop && npx tsc --noEmit                                            # no output
```

Expected: `10 passed`, a count greater than zero once task 80 has merged, `0`, no output. None has
been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not parity with Linear's favourites and user pages in detail.** 01 §7.6 marks both
  **UNVERIFIED** beyond their existence, and nothing here was compared against the product.
- **Not natural-language filters** (L-100, task 138, W4).
- **Not the agent summary on a person's page.** TM-52 is **UNVERIFIED until Phase T merges**; task
  107 fills the line, and until then the page renders no agent element.
- **Not the read-only browser page** (L-98, task 80's `/p/issues/…`, itself a proposed
  Hub-contract addition). This task's surfaces are all in the desktop.
- **Not CSV export** (task 80's route; the desktop's button for it is task 92's neighbourhood, not
  named in this scope).
- **No measured filter cost.** `matches` is a tree walk per head and has not been timed at 9,000
  issues; the view list is virtualised only if task 40 already virtualises it.

#### Gaps reported to the skeleton

1. **The shared vectors live in the Hub tree, which the desktop suite must reach.** The runner
   above uses a relative path out of `desktop/`; if the desktop test runner is rooted so that path
   is unreachable, task 80 copies the vectors to `desktop/src/work/testdata/view-filter/` as part
   of its own commit. Asked of task 80 here; a retyped copy is not acceptable (decision 1).
2. **Favourites have no Hub object.** They are preferences (decision 3), so a person's favourites
   do not follow them to a second device. The scope does not say which it wants; this task chose
   the cheaper one and names it, because L-109 prices favourites as a rail affordance, not as
   shared state.
3. **`O F` needs a favourites picker** and task 53's sequence table may already hold `O` prefixes
   for issue, cycle and feature. If `O F` collides, the picker is reached from `O` with a type
   filter instead, and the key row is dropped.
