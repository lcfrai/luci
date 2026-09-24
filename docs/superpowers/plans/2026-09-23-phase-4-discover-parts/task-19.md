### Task 19: Desktop — moved on, project changed, undo, past revision, missing, erased, hand-edited

The second half of the lens: the states where the Brief and what agents inherited have **come
apart**, and the one-click past revision every chip opens. Prototype states 8, 9, 11, 12 and 13,
plus `erased` (ADR-H1 §5.6), which no prototype draws.

**Read `docs/DESIGN-SYSTEM.md` first.** Held here, beyond task 18's list: **optimistic mutation
with an Undo window, not a confirm dialog** — an edit saves at once, and *Undo* for five seconds
writes a `restore` revision (a revision is never deleted, P3); **no relative timestamps** as a
primary display (AGENTIC-UX property 1) — history rows show revision, change and who, and no save
time, as the prototype does; **no counts outside the queue** (AGENTIC-UX principle 4) — a project
edit lists the branch briefs it moved **by name**, never *"3 branches updated"*; a read-only view
is visibly read-only (no fields, a *Back to the live brief* control) rather than a disabled form;
colour plus shape for superseded, missing and erased, from task 17's components. The owner's eyes
are the visual acceptance (`OD-12`).

**Files:**
- `desktop/src/discover/BriefHistory.tsx`, `PastRevision.tsx`, `SessionsOnBrief.tsx`,
  `Materialisations.tsx`, `SectionChanges.tsx` (new)
- `desktop/src/discover/BriefHistory.test.tsx`, `PastRevision.test.tsx`,
  `SessionsOnBrief.test.tsx`, `Materialisations.test.tsx` (new)
- `desktop/src/discover/DiscoverLens.tsx`, `BriefPanel.tsx`, `discover.css` (modify, task 18's)
- `desktop/src/store/selectors.ts` — one key source, `selectPastKey` (modify)

**Interfaces — complete source:**

```ts
// desktop/src/store/selectors.ts — one more key source (R6): the revision Discover shows read-only
export const selectPastKey: KeySource; // [pastKey] when set, else []
export const keySources: readonly KeySource[] = [selectSessionContextRefs, selectPastKey];
```

```ts
// desktop/src/discover/SectionChanges.tsx
/** Which sections differ between two resolved refs, by section key and body text. Compares
 *  strings only: it renders nothing the server did not resolve, and computes no digest. This is
 *  the "what changed" of states 8, 9 and 12 — a section-level before and after, not a line diff
 *  of CLAUDE.md (a proposal for the owner's eyes; the prototype draws a file diff). */
export interface SectionChange {
  key: string; heading: string; before: string | null; after: string | null; origin: Origin;
}
export function sectionChanges(from: ProvenanceRef, to: ProvenanceRef): SectionChange[];
export function SectionChanges(props: { from: ProvenanceRef; to: ProvenanceRef }): JSX.Element;
```

```ts
// desktop/src/discover/SessionsOnBrief.tsx
/** Every session on the target's branch, by name, each with ITS OWN chip (task 17) — so a
 *  session that inherited rev 5 and one that inherited rev 6 read differently — and, on a
 *  superseded one, "Restart to inherit rev n" through phase 2's restart verb. "Nobody yet" when
 *  none runs. */
export function SessionsOnBrief(props: { target: Target }): JSX.Element;

// desktop/src/discover/Materialisations.tsx
/** Where the brief is written and what each worktree's file holds (task 12's route): fetched on
 *  open of the panel and after a save, never on a timer. "edited" shows the server's diff lines
 *  and says the next session start rewrites the block. Each entry of `committed` names the
 *  TRACKED file that holds a block and says no session start changes it (review I3): it offers
 *  no rewrite, no strip and no git action, because Panacea never writes a tracked file (3-D6).
 *  It is shown BESIDE the target's own state, never instead of it, so an edited target keeps
 *  its diff and warning and a broken or superseded one renders as it would alone (re-review
 *  R-M1). A non-empty `excluded` says, beside the target's path, that git stops listing the
 *  file once the brief is written there, text of the person's own in it included (review M9). */
export function Materialisations(props: { projectId: ProjectId; branchId: BranchId }): JSX.Element;

// desktop/src/discover/BriefHistory.tsx
export function BriefHistory(props: { briefId: string; onOpen(key: string): void }): JSX.Element;

// desktop/src/discover/PastRevision.tsx
/** discoverSlice.pastKey, read-only. Resolved through the loader like any key — never fetched
 *  here (P8). Shows the revision's sections, what has changed since (against the head reading),
 *  and the missing, erased and unavailable states in their own words. */
export function PastRevision(props: { pastKey: string }): JSX.Element;
```

**What each state shows:**

| # | When | Shows |
|---|---|---|
| 8 | an edit saved while sessions on the branch run | the new key; each running session by name with the revision it inherited; *Restart to inherit rev n* on superseded ones; the section changes; the Undo window |
| 9 | a project edit whose `written` includes branch keys (`4-D3`) | on the project view: each branch brief moved, **by branch name**, with its new key. On a branch view after the feed marks it stale: *The project brief changed; this branch's brief is now rev 7*, and the section changes show **only inherited sections moving** |
| — | Undo pressed within five seconds | a `restore` op for the previous revision; the new key; the history shows `restore` · `restores: 6` |
| 11 | a worktree's inspection is `edited` | the file path, the changed lines (`added`/`removed`), *The next session started here rewrites this block. Edit the brief in Discover instead.* |
| — | a worktree's inspection has a non-empty `committed` (review I3; no prototype draws it) | below the target's own state, whatever that is (re-review R-M1): the tracked file's path in mono; the marker's key when there is one; *This repository has a brief block committed in this file. Sessions here read it as well as the brief Panacea writes, and no session start will change it. Remove it in a commit.* No action control |
| — | a worktree's inspection has a non-empty `excluded` (review M9; no prototype draws it) | under the target's path, in every state: *Before it writes the brief here, Panacea adds `<excluded>` to this clone's `.git/info/exclude`. From then on git does not list this file: `git status` and `git add` skip it, and any text of your own in it.* When `excluded` is `/CLAUDE.md`, also: *To commit text of your own in it, delete the Panacea block, then run `git add -f CLAUDE.md`. Once git tracks the file, Panacea writes the brief to `.claude/CLAUDE.md` instead and never changes this one.* No action control. Empty `excluded`: neither sentence |
| 12 | `pastKey` resolved | *You are reading revision 5, read-only.* Its sections; *Changed since:* the section changes to the head reading; *Back to the live brief* |
| 13 | `pastKey` missing | the key, verbatim, in mono, *not found*, framed as **data loss in our store**, distinct from *no brief* |
| — | `pastKey` erased | *This brief was erased on <date>. Its key and digest remain; its text does not.* |
| — | `pastKey` unavailable | *The Hub did not answer, so this revision could not be read.* No retry control: the loader retries on its own cadence (task 16) |

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `sessions show their own revisions` | two sessions at `@5` and `@6`, head latest-complete 6 → one superseded chip with a restart offer, one current | the branch's chip painted on every session |
| `restart offer calls the restart verb` | clicking calls phase 2's restart for that session id | an offer that navigates instead of acting |
| `nobody yet` | no sessions → *Nobody yet* | an empty list read as a loading state |
| `undo writes a restore, not a delete` | Undo within 5 s sends `{kind: "restore", restoreRevision: 6}`; after 5 s the control is gone | an undo that tries to remove a revision (P3) |
| `project edit lists branches by name, never a count` | `written` with two branch keys → both branch names; the text matches no `/\d+\s+branch/` | a count outside the queue |
| `section changes compare text only` | two refs differing in the done section → one change with before and after; identical refs → none | a diff that flags every section |
| `inherited-only movement on a branch` | a branch ref whose own answers are unchanged and whose audience origin is project → the only change is audience, marked *inherited* | D3's visible cost hidden |
| `past revision is read-only` | no `input`/`textarea` rendered; *Back to the live brief* calls `closePast` | an editable old revision — a second head |
| `past revision is resolved by the loader, not fetched` | rendering `PastRevision` makes no client call; `selectPastKey` returns the key | P8 |
| `missing is data loss, not none` | missing → the key in mono and the data-loss sentence; none → never this sentence | two states merged |
| `erased keeps key and digest in view` | the key and *erased on* are shown; no section text | an erased brief shown as missing |
| `edited block shows the server's lines` | an `edited` inspection → one removed and one added line, verbatim | a client-computed diff of file text |
| `committed block names the tracked file and offers nothing` | a `current` inspection with one `committed` entry → the entry's `path` verbatim and the *Remove it in a commit* sentence; no `button` inside the committed row; the *rewrites this block* sentence absent from it | a committed block shown as `edited`, promising a rewrite that never happens; a control that invites Panacea to edit a tracked file (3-D6) |
| `committed never hides the target's own state` | an `edited` inspection with one `committed` entry → the edited diff lines and the *rewrites this block* sentence, AND the committed path with *Remove it in a commit*; for each of `broken` and `superseded`, the inspection with one entry renders every text node the same inspection renders with `committed: []`, plus the committed path (task 19 draws no special words for those two, so the test compares renders instead of naming a sentence) | the committed row painted instead of the target's state, so a hand edit is silently overwritten at the next start, or a `block_broken` refusal has no visible cause (re-review R-M1) |
| `the exclude line is stated where the file is named` (review M9) | `excluded: "/CLAUDE.md"` → the *git does not list this file* sentence naming `/CLAUDE.md`, and the *delete the Panacea block, then run `git add -f CLAUDE.md`* sentence; `excluded: "/.claude/CLAUDE.md"` → the first sentence naming that line and not the second; `excluded: ""` → neither; no `button` in any row | a person's own untracked `CLAUDE.md` (notes, `/init` output) hidden from `git add` with no word in the product; a way back that tells them to commit the brief block (3-D6) |
| `no relative time` | no rendered text matches `/\b(ago|just now|yesterday)\b/i` | AGENTIC-UX property 1 |

**Steps:**

- [ ] **Step 1:** Open prototype states 8, 9, 11, 12, 13 and 15 and read their notes.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The five components; the key source; wire them into `DiscoverLens` and
  `BriefPanel`; CSS under a `/* task 19 */` header at the foot of `discover.css`, reflowing no rule
  above it.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. **Do not merge** (`OD-1`).

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
```

Owner-observed, **unverified until the batched session** (`OD-12`): whether a section-level
before/after is enough, or the owner wants the prototype's file diff (then task 12 gains a
`GET /v1/discover/diff?from=&to=` rendering both keys server-side — the desktop still renders
nothing itself).

**Does NOT claim:** the session-start refusal (task 20).
