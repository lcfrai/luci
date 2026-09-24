### Task 06: The linked-worktree notice, deleted — `kind` in the model, and the shared-index note (3-D2)

The sandbox git fix added `LinkedWorktreeNote`, committed as `58d607f` on `fix/sandbox-git-ca`
and in phase 3's base through `f898f5e` (re-read 2026-09-24 at `4f6d698` and at
`feat/phase-2-cockpit`'s `10fa5fd`, review I12; the condition and sentence are the same at both,
and phase 2's round 1 rewrote only its doc comment and added tests, `c53621a`): for every
worktree with `isPrimary: false` it says git is unavailable and that git in linked worktrees
"arrives in Phase 3". The fix's contract sweep (`5e32000`, then `d3a2824`) wrote that condition
into UI §6.5's `LinkedWorktreeNote.tsx` row and §7.7's linked-worktree state row, so this task
amends both. After task 04 a cockpit worktree is a clone
with `isPrimary: false` **and working git**, and under ruling R4.1 (task 05) no session runs in
a linked worktree at all: a session on one runs in its backing lane, where git works. So the
notice's claim is false for every kind, and narrowing it would leave a condition that renders
for no worktree. This task **deletes** the notice, its sentence and its tests, keeps `kind` in
the model, and adds the one other git fact a session pane must state: that two live sessions
in one worktree share one index (3-D2, survey §3.2).

**Why a model field and not a phase 3 slice** (plan decision 7): `kind` arrives with the
worktree itself, so nothing that reads it has a window in which a clone looks like something
else. Task 04's catalog sends it, and task 04's delete route refuses a worktree of kind
`primary` or `linked` (`not_a_cockpit_worktree`), so the desktop can tell the two apart.

**Files:**
- `desktop/src/store/types.ts` — `WorktreeKind`, `Worktree.kind` (p2; contract §2 amended in the same commit)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` — §2's fenced `Worktree` block, byte-identical to `types.ts`; §6.5's `LinkedWorktreeNote.tsx` row deleted and a `SharedIndexNote` row added; §7.7's linked-worktree state row deleted (review I12: the fix wrote those rows)
- `desktop/src/test/fixtures.ts` — `aWorktree` derives `kind` from `isPrimary` unless given (p2)
- `desktop/src/components/develop/LinkedWorktreeNote.tsx` + `.test.tsx` — deleted (the fix's files, `58d607f`)
- `desktop/src/components/develop/SharedIndexNote.tsx` + `.test.tsx` — new
- `desktop/src/components/develop/SessionPane.tsx` — the fix's `<LinkedWorktreeNote worktree={worktree} />` (`:208`) and its import (`:9`) become `SharedIndexNote`; the comment above it (`:205-207`) keeps its grid-row reason and drops "a pane on a primary worktree renders nothing at all" (p2)
- `desktop/src/components/develop/EmptyDevelop.tsx` — the fix's `<LinkedWorktreeNote worktree={target} />` (`:123`), its comment (`:122`) and its import (`:10`) deleted (p2)
- `desktop/src/components/develop/develop.css` — nothing new: `SharedIndexNote` uses the fix's `.pane-gitnote`; the rule's header comment (`:460-461`) names the shared-index note instead

#### Interfaces — complete

```ts
// store/types.ts — the Worktree block after this task. Everything but `kind` is unchanged.

/** What a worktree is in git. No sandbox mounts a primary or linked worktree: a session on
 *  one runs in its backing lane clone (phase 3 ruling R4.1), so git works in every session.
 *  "primary" — the repository's main worktree: its .git is a directory.
 *  "linked"  — made with `git worktree add` outside the cockpit: its .git is a FILE.
 *  "clone"   — a cockpit worktree (docs/superpowers/planning/2026-09-23-owner-decisions.md
 *              OD-2): a local clone whose .git is a directory. Git works; it cannot fetch or
 *              push, by design — the control plane does both. */
export type WorktreeKind = "primary" | "linked" | "clone";

export interface Worktree {
  id: WorktreeId;
  projectId: ProjectId;
  branchId: BranchId;
  name: string;
  path: string;
  /** Kept for the phase 2 readers; equals `kind === "primary"`. */
  isPrimary: boolean;
  kind: WorktreeKind;
  /** … unchanged comment … */
  sessionCount: number;
  /** … unchanged comment … */
  ready?: boolean;
}
```

```ts
// test/fixtures.ts — aWorktree's default kind follows isPrimary, so every existing test that
// builds `aWorktree({ isPrimary: false })` keeps meaning "a linked worktree".
export function aWorktree(over: Partial<Worktree> = {}): Worktree {
  const isPrimary = over.isPrimary ?? false;
  return {
    id: IDS.worktree,
    projectId: IDS.project,
    branchId: IDS.branch,
    name: "evidence-upload",
    path: "C:\\work\\panacea\\.worktrees\\evidence-upload",
    isPrimary,
    kind: isPrimary ? "primary" : "linked",
    sessionCount: 1,
    ready: false,
    ...over,
  };
}
```

```tsx
// components/develop/SharedIndexNote.tsx — new. LinkedWorktreeNote.tsx and its
// LINKED_WORKTREE_GIT_NOTE are deleted: under R4.1 no session runs in a linked worktree.

/** One sentence, 3-D2: two or more live sessions in one worktree share its index and HEAD
 *  (a primary's or linked worktree's sessions all run in its one backing lane, task 05). */
export const SHARED_INDEX_NOTE =
  "These sessions share one git index: a commit in either takes whatever the other has staged, " +
  "and its Panacea-Session trailer names only the one that ran git commit.";

/** Renders when worktree.sessionCount >= 2 (live sessions, from the server — never shown as a
 *  number), for every kind, and nothing for an unloaded worktree. It carries one button,
 *  "Start a session in a new worktree", which calls onNewWorktree. */
export function SharedIndexNote(props: {
  worktree: Worktree | undefined;
  onNewWorktree?: () => void;
}): JSX.Element | null;
```

`SessionPane` mounts `<SharedIndexNote worktree={worktree} onNewWorktree={…} />` where the
fix's `<LinkedWorktreeNote worktree={worktree} />` was, in the same auto grid row. Until task 22
exists, `onNewWorktree` is `undefined` and the button is not rendered (a control that cannot act
is not drawn); task 22 passes the create-then-start action. `EmptyDevelop` mounts nothing in the
note's place: with no session there is no index to share.

#### The tests to write first

The fix's `LinkedWorktreeNote.test.tsx` is deleted with its component (R4.1: the notice renders
for no kind, so none of its cases survives as written). The rows below are new.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `no pane shows the linked-worktree notice` (SessionPane and EmptyDevelop) | `seed` with each kind in turn (`primary`, `linked`, `clone`), a live session and none → no `[data-linked-worktree-note]` and no text matching `/^Git is unavailable/` | **the defect this task exists for**: the fix's `!isPrimary` notice, false on every worktree once sessions run in lanes |
| `SHARED_INDEX_NOTE is one sentence naming the index and the trailer` | one full stop; contains `one git index` and `Panacea-Session` | a note that hides why it matters |
| `SharedIndexNote renders at two live sessions and not at one` | `sessionCount: 2` → one `[data-shared-index-note]`; `1` → none | an off-by-one that warns on every worktree |
| `SharedIndexNote renders for every kind` | `kind` each of `primary`, `linked`, `clone`, with `sessionCount: 2` → one note each (their sessions share one lane's index, R4.1) | the fix's linked-worktree exclusion carried over |
| `SharedIndexNote draws its button only with an action` | no `onNewWorktree` → no `button`; with it → one button whose click calls it once | a dead control |
| `the pane mounts the note after the body` (SessionPane) | with `kind: "clone", sessionCount: 2` the shared note's `previousElementSibling` is the body | the note landing in the 1fr grid row (the fix's own comment on `.pane`'s rows) |
| `types.ts and the UI contract's Worktree block are byte-identical` | read both files; the `export interface Worktree {…}` blocks and `WorktreeKind` match exactly | the SX.2 drift, until task 26's gate exists |

No jest-dom (shared-shape §3C): plain DOM property assertions, as the fix's suite does.

#### Steps

- [ ] **Step 1: Preflight.** G1 holds: `ls desktop/src/components/develop/LinkedWorktreeNote.tsx`
  succeeds and `grep -c "isPrimary" desktop/src/components/develop/LinkedWorktreeNote.tsx` ≥ 1.
  Task 04 has landed: `grep -c '"kind"' control-plane/internal/api/catalog.go` ≥ 1. If the fix
  landed with a different sentence or file name, apply this task to what landed and say so.
  Task 05 has landed (R4.1: `TestPrimarySessionRunsInALane` passes). If it has not, a linked
  worktree's sessions still mount it and have no git, the notice is still true, and this task
  waits: stop and report.
  Then G1's contract diff: `git diff 3303fd6 HEAD -- docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md`
  and read every hunk in §2, §6.5 and §7.7 before editing them (the fix's `5e32000` and
  `d3a2824` wrote the rows this task amends; review I12).
- [ ] **Step 2:** rewrite and add the tests; run them red.
- [ ] **Step 3:** `types.ts` and the contract block in one edit each, then the fixture. In the
  same commit (R4.1): UI §6.5's `LinkedWorktreeNote.tsx` row and its fenced signature are
  deleted; the `SessionPane.tsx` and `EmptyDevelop.tsx` rows drop their `<LinkedWorktreeNote/>`
  clauses, and `SessionPane.tsx`'s names `<SharedIndexNote/>` instead; §6.5 gains a
  `SharedIndexNote` row; §7.7's linked-worktree state row is deleted. In this worktree's copy
  those are lines 1749, 1761, 1744, 1748 and 2154 (the fix's row was line 1770 at `10fa5fd`,
  re-review b03 Minor 5; its "arrives in Phase 3" and "not primary" phrases go with it). Locate
  them at G1 with `grep -n "LinkedWorktreeNote\|linked worktree" docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md`.
- [ ] **Step 4:** delete `LinkedWorktreeNote.tsx` and its test, add `SharedIndexNote.tsx`, and
  make the `SessionPane`, `EmptyDevelop` and `develop.css` comment edits.
- [ ] **Step 5:** `npx tsc --noEmit` — every object literal typed `Worktree` without `kind` now
  fails to compile; fix each by adding `kind` (tests use the fixture). List every file touched in
  the commit message.
- [ ] **Step 6:** run and commit by pathspec.

#### Design-system checks (UI)

The note reuses the fix's `.pane-gitnote` rule: `--ink-2` text on `--inset`, a hairline by
`box-shadow` (no `1px solid`), no motion — nothing about the note changes while it is on
screen, so it has no motion to explain. The button is the existing `.btn` class. No new token, no
colour, no radius. **The look is unverified until the owner sees it (OD-12).**

#### Acceptance

```
cd desktop && npx vitest run src/components/develop/   → PASS
cd desktop && npx tsc --noEmit                          → no output
cd desktop && npx vitest run                            → PASS (whole suite: the kind field reaches every fixture)
```

#### What this task does not claim

- It does not make git work in a **linked** worktree's own directory; nothing does. Task 05
  runs its sessions in a backing lane instead (R4.1), which is why the notice goes.
- It does not stop a second session starting in an occupied worktree (3-D2 option c was not
  chosen).
