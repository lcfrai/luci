### Task 06: The linked-worktree notice, superseded — `kind` in the model, the notice narrowed, and the shared-index note (3-D2)

The sandbox git fix added `LinkedWorktreeNote` (uncommitted in `.claude/worktrees/sandbox-git-fix`
on 2026-09-23, read there): for every worktree with `isPrimary: false` it says git is unavailable
and that git in linked worktrees "arrives in Phase 3". After task 04 a cockpit worktree is a clone
with `isPrimary: false` **and working git**, so the fix's condition becomes a false claim on
every worktree phase 3 creates. This task makes the condition exact, rewrites the sentence so it
names what the user can do instead of promising a phase, and adds the one other git fact a
session pane must state: that two live sessions in one worktree share one index (3-D2, survey
§3.2).

**Why a model field and not a phase 3 slice** (plan decision 7): the notice must render nothing
until the cockpit has seen the fact that justifies it (the fix's own rule, "a worktree the store
has not loaded proves nothing either way"). A lane set loaded by a separate request would leave a
window where a clone renders the linked notice. `kind` arrives with the worktree itself.

**Files:**
- `desktop/src/store/types.ts` — `WorktreeKind`, `Worktree.kind` (p2; contract §2 amended in the same commit)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` — §2's fenced `Worktree` block, byte-identical to `types.ts`
- `desktop/src/test/fixtures.ts` — `aWorktree` derives `kind` from `isPrimary` unless given (p2)
- `desktop/src/components/develop/LinkedWorktreeNote.tsx` + `.test.tsx` — narrowed, reworded, and `SharedIndexNote` (the fix's files)
- `desktop/src/components/develop/SessionPane.tsx` — mount `SharedIndexNote` beside the fix's mount (p2, one line)
- `desktop/src/components/develop/develop.css` — nothing new: both notes use the fix's `.pane-gitnote`

#### Interfaces — complete

```ts
// store/types.ts — the Worktree block after this task. Everything but `kind` is unchanged.

/** What a worktree is in git, which decides whether git works inside its sandbox.
 *  "primary" — the repository's main worktree: its .git is a directory. Git works.
 *  "linked"  — made with `git worktree add` outside the cockpit: its .git is a FILE naming a
 *              host path the Linux sandbox cannot see, so git is unavailable there.
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
// components/develop/LinkedWorktreeNote.tsx — after this task.

/** One sentence. It names the fact (a linked worktree's .git points at a host path) and the
 *  thing the user can do instead (a cockpit worktree is a clone, and git works there). It no
 *  longer promises a phase: phase 3 has shipped the fix, which is a different worktree. */
export const LINKED_WORKTREE_GIT_NOTE =
  "Git is unavailable in this worktree's sessions because it is a linked worktree, whose .git " +
  "points to a path on this machine the sandbox cannot see; a worktree created from the " +
  "cockpit is a clone, and git works in it.";

/** Renders for kind === "linked" only. Nothing for "primary" or "clone", whose git works, and
 *  nothing for an unloaded worktree. */
export function LinkedWorktreeNote({ worktree }: { worktree: Worktree | undefined }): JSX.Element | null;

/** One sentence, 3-D2: two or more live sessions in one worktree share its index and HEAD. */
export const SHARED_INDEX_NOTE =
  "These sessions share one git index: a commit in either takes whatever the other has staged, " +
  "and its Panacea-Session trailer names only the one that ran git commit.";

/** Renders when worktree.sessionCount >= 2 (live sessions, from the server — never shown as a
 *  number) and worktree.kind !== "linked" (a linked worktree has no git to share). It carries
 *  one button, "Start a session in a new worktree", which calls onNewWorktree. */
export function SharedIndexNote(props: {
  worktree: Worktree | undefined;
  onNewWorktree?: () => void;
}): JSX.Element | null;
```

`SessionPane` mounts `<SharedIndexNote worktree={worktree} onNewWorktree={…} />` on the line
after the fix's `<LinkedWorktreeNote worktree={worktree} />`, in the same auto grid row. Until
task 22 exists, `onNewWorktree` is `undefined` and the button is not rendered (a control that
cannot act is not drawn); task 22 passes the create-then-start action. The two notes cannot both
render: one requires `kind === "linked"`, the other excludes it.

#### The tests to write first

The fix's suite stays; three of its cases change meaning and are rewritten, and the rest are new.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `is one sentence naming what is unavailable, the fact behind it, and the way out` (rewritten) | one terminal full stop; contains `/^Git is unavailable/`, `/linked worktree/`, `/\.git/`, `/sandbox cannot see/`, `/created from the cockpit is a clone/`; does **not** match `/Phase \d/` | the stale promise surviving |
| `renders the sentence for a linked worktree` (kept) | `aWorktree({ isPrimary: false })` (kind `linked` by the fixture) → one note, `role="note"` | narrowing the condition past linked |
| `renders nothing for a clone, whose git works` (new) | `aWorktree({ isPrimary: false, kind: "clone" })` → no child nodes | **the defect this task exists for**: the fix's `!isPrimary` condition |
| `renders nothing for the primary worktree` (kept) | unchanged | — |
| `shows nothing in a pane on a clone` (new, SessionPane) | `seed` with a clone worktree → no `[data-linked-worktree-note]` | the pane using `isPrimary` directly |
| `SHARED_INDEX_NOTE is one sentence naming the index and the trailer` | one full stop; contains `one git index` and `Panacea-Session` | a note that hides why it matters |
| `SharedIndexNote renders at two live sessions and not at one` | `sessionCount: 2` → one `[data-shared-index-note]`; `1` → none | an off-by-one that warns on every worktree |
| `SharedIndexNote renders nothing on a linked worktree` | `kind: "linked", sessionCount: 2` → none | two contradictory notes in one pane |
| `SharedIndexNote draws its button only with an action` | no `onNewWorktree` → no `button`; with it → one button whose click calls it once | a dead control |
| `the pane mounts both notes after the body` (SessionPane) | with `kind: "clone", sessionCount: 2` the shared note's `previousElementSibling` is the body or the linked note's slot | the note landing in the 1fr grid row (the fix's own comment on `.pane`'s rows) |
| `types.ts and the UI contract's Worktree block are byte-identical` | read both files; the `export interface Worktree {…}` blocks and `WorktreeKind` match exactly | the SX.2 drift, until task 26's gate exists |

No jest-dom (shared-shape §3C): plain DOM property assertions, as the fix's suite does.

#### Steps

- [ ] **Step 1: Preflight.** G1 holds: `ls desktop/src/components/develop/LinkedWorktreeNote.tsx`
  succeeds and `grep -c "isPrimary" desktop/src/components/develop/LinkedWorktreeNote.tsx` ≥ 1.
  Task 04 has landed: `grep -c '"kind"' control-plane/internal/api/catalog.go` ≥ 1. If the fix
  landed with a different sentence or file name, apply this task to what landed and say so.
- [ ] **Step 2:** rewrite and add the tests; run them red.
- [ ] **Step 3:** `types.ts` and the contract block in one edit each, then the fixture.
- [ ] **Step 4:** the component changes and the `SessionPane` line.
- [ ] **Step 5:** `npx tsc --noEmit` — every object literal typed `Worktree` without `kind` now
  fails to compile; fix each by adding `kind` (tests use the fixture). List every file touched in
  the commit message.
- [ ] **Step 6:** run and commit by pathspec.

#### Design-system checks (UI)

The notes reuse the fix's `.pane-gitnote` rule: `--ink-2` text on `--inset`, a hairline by
`box-shadow` (no `1px solid`), no motion — nothing about either note changes while it is on
screen, so it has no motion to explain. The button is the existing `.btn` class. No new token, no
colour, no radius. **The look is unverified until the owner sees it (OD-12).**

#### Acceptance

```
cd desktop && npx vitest run src/components/develop/   → PASS
cd desktop && npx tsc --noEmit                          → no output
cd desktop && npx vitest run                            → PASS (whole suite: the kind field reaches every fixture)
```

#### What this task does not claim

- It does not make git work in a **linked** worktree; nothing does. It tells the user a cockpit
  worktree works, and task 22 gives them the one click to make one.
- It does not stop a second session starting in an occupied worktree (3-D2 option c was not
  chosen).
