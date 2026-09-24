### Task 22: Lifecycle verbs in the cockpit — new and remove worktree, branch create, rename and delete, the palette, and four stale promises retired

Tasks 04 and 11 built the routes; this task puts them where the user already looks. Four phase 2
surfaces currently **promise phase 3** in their copy, and each promise is now either kept or
removed (the fourth, `sessionVerbs.ts`, found by review Minor 10: the old grep for
`arrives in Phase 3` missed its wording):

| Where (read 2026-09-23) | Says today | After this task |
|---|---|---|
| `components/develop/EmptyDevelop.tsx:100-101` | "Creating a worktree arrives in Phase 3. Until then, check out this branch in a worktree from the command line and it will appear in the rail." | a **New worktree on <branch>** button that creates a clone and selects it; the doc comment at `:72-73` ("creation is Phase 3, so it must say so in WORDS rather than offer a button that 404s") is rewritten to say the button calls task 04's route |
| `components/menu/ContextMenu.tsx:102-106` | project "New branch", disabled: "Phase 2 has no create-branch endpoint (contract §10.3)…" | enabled; opens the new-branch form |
| `components/topbar/TopBar.tsx:29` | Authority's title: "…It arrives in Phase 3." | "Authority needs the agent-runtime signal (docs/AGENTIC-UX.md §7). No phase owns it yet." — 3-D10: the number comes off until a phase owns it. `components/AppShell.test.tsx:119` asserts that title matches `/Phase 3/`, so its Authority entry becomes `/No phase owns it yet/` in the same commit (the Run locally entry keeps `/Phase 7/`) |
| `components/develop/sessionVerbs.ts:607-608` (doc comment) and `:624` (`worktreeReadyItems`' disabled reason) (review Minor 10) | "Phase 2 has no un-mark to offer — the merge queue in Phase 3 owns that." and "Already marked ready. Phase 2 has no un-mark; the merge queue in Phase 3 owns that." | the reason: "Already marked ready. To take it out of the queue, use Not ready on its card in the Merge lens."; the comment: already-ready stays disabled rather than hidden, and un-marking is task 19's **Not ready** button, where the queue it leaves is in view. `sessionVerbs.test.ts:820`'s `toContain("Already marked ready")` still holds. The copy names task 19's button, so this task waits on 19 |

**The one question in the product** is *Remove worktree…* (survey §6.2): it names the live
sessions, the uncommitted paths and the unlanded commits the server reports, and on **Remove** it
kills each live session through phase 2's existing kill verb (`api.killSession`, contract §5.2's
permitted destroy) and then calls `removeWorktree` with the confirm token it was shown. Every
other verb here is immediate, with a toast.

**Files:**
- `desktop/src/components/menu/lifecycleItems.ts` + test — the worktree, branch and project items (new)
- `desktop/src/components/menu/ContextMenu.tsx` — spread the items; the project "New branch" row replaced (p2)
- `desktop/src/components/lifecycle/NewBranchForm.tsx`, `RenameBranchForm.tsx`, `RemoveWorktreeQuestion.tsx`, `DeleteBranchQuestion.tsx`, `lifecycle.css`, `actions.ts` (new)
- `desktop/src/components/develop/EmptyDevelop.tsx` — the no-worktree branch and its doc comment's `:72-73` sentence (p2, two blocks)
- `desktop/src/components/develop/sessionVerbs.ts` — `worktreeReadyItems`' doc comment `:607-608` and its disabled reason `:624` (p2; review Minor 10)
- `desktop/src/components/AppShell.test.tsx` — line 119's Authority entry, `/Phase 3/` → `/No phase owns it yet/` (p2; the TopBar change breaks it otherwise)
- `desktop/src/components/develop/SessionPane.tsx` — pass `onNewWorktree` to task 06's `SharedIndexNote` (p2, one prop)
- `desktop/src/components/lens/merge/MergeBanner.tsx` — task 19's m-new "New worktree" wired (phase 3's file)
- `desktop/src/components/palette/commands.ts` + test — phase 3's commands appended (p2, seam SX.6)
- `desktop/src/components/topbar/TopBar.tsx` — line 29's title (p2)
- `desktop/src/store/slices/prefsSlice.ts`, `uiSlice.ts` — `rebindBranch(fromId, toId)` (p2, additive)

#### Interfaces — complete

```ts
// lifecycle/actions.ts — the verbs, each: call, then reload what it changed, then one toast.
/** Create a clone on the branch and select it. Toast: "New worktree upload-2 on evidence-upload." */
export function newWorktree(projectId: ProjectId, branchId: BranchId, name?: string): Promise<Worktree | null>;
/** newWorktree, then start a session in it with the branch's resolved defaults — the one click
 *  3-D2 promises ("start in a new worktree instead"). */
export function sessionInNewWorktree(projectId: ProjectId, branchId: BranchId,
  createSession: ReturnType<typeof useCreateSession>): Promise<void>;
/** First DELETE with no token: a 200 is done; a 409 live_sessions / confirm_required returns the
 *  question's facts for RemoveWorktreeQuestion. */
export function inspectRemoval(projectId: ProjectId, worktreeId: WorktreeId): Promise<RemovalQuestion | "removed">;
export interface RemovalQuestion {
  worktreeId: WorktreeId;
  sessions: { id: SessionId; title: string }[]; // from details.sessions ("id\ttitle" lines)
  paths: string[];                               // detailList(error, "paths") (task 18; review I2)
  commits: string[];                             // details.commits
  elsewhere: string[];                           // details.elsewhere ("ref\tsha" lines; review I6)
  unlisted: string;                              // details.unlisted, or "" (re-review b04-1 issue 2)
  unarchivable: string;                          // details.unarchivable, or ""
  confirm: string;                               // details.confirm; "" when only sessions block it
}
/** Kill each session, then DELETE with the token; a confirm_stale re-asks with the fresh facts. */
export function confirmRemoval(projectId: ProjectId, q: RemovalQuestion): Promise<RemovalQuestion | "removed">;
export function createBranch(projectId: ProjectId, name: string, fromBranchId: BranchId): Promise<Branch | null>;
/** On success, rebindBranch(renamedFrom, branch.id) so selection, expansion and branch defaults
 *  follow the rename (A8), and the toast adds "The remote branch keeps its old name." when
 *  remoteKeepsOldName. */
export function renameBranch(projectId: ProjectId, branchId: BranchId, name: string): Promise<RenameResult | null>;
export function deleteBranch(projectId: ProjectId, branchId: BranchId): Promise<DeleteResult | null>;
```

```ts
// menu/lifecycleItems.ts — MenuItem is ContextMenu.tsx's (imported as a type, as sessionVerbs does).
export interface LifecycleDeps {
  close(): void;
  openForm(form: { kind: "new-branch"; projectId: ProjectId } | { kind: "rename-branch"; branch: Branch }
    | { kind: "delete-branch"; branch: Branch } | { kind: "remove-worktree"; question: RemovalQuestion }): void;
  createSession: ReturnType<typeof useCreateSession>;
}
export function projectLifecycleItems(project: Project, deps: LifecycleDeps): MenuItem[];   // "New branch…"
export function branchLifecycleItems(branch: Branch, deps: LifecycleDeps): MenuItem[];      // "New worktree here", "Rename branch…", "Delete branch…" (danger)
/** "Remove worktree…" (danger). For kind "primary" or "linked": disabled, reason "This worktree
 *  was not created by the cockpit; remove it with git worktree remove." */
export function worktreeLifecycleItems(worktree: Worktree, deps: LifecycleDeps): MenuItem[];
```

```ts
// prefsSlice / uiSlice — additive. Moves every branch-keyed entry from one id to another:
// branchDefaults[from] → [to] (prefs); expandedBranches[from] → [to] and selection.branchId (ui).
// A no-op when from === to or from is absent. Persisted keys are MOVED, not reshaped, so no
// persist version is claimed (R3).
rebindBranch(fromId: BranchId, toId: BranchId): void;
```

```ts
// palette/commands.ts — appended after the existing verbs, in one block. Ranking is fixed there
// and filtering only removes (seam SX.6). Only undoable or read-only verbs are here: Remove
// worktree and Delete branch are NOT, because a destructive verb must not sit one keystroke from a
// filter (open carry 4).
//   "verb:new-worktree"   "New worktree on <branch>"          when a branch is selected
//   "verb:fetch"          "Fetch <project>"                   when the project has a remote (convergence loaded)
//   "verb:push"           "Push <branch>"                     when pushBlock(conv) is null
//   "land:<worktreeId>"   "Land <worktree> into <branch>"     for each queued row whose forecast is clean
//   "ready:<worktreeId>"  "Mark <worktree> ready"             for each not-ready lane with commits
```

**The forms** are popovers anchored to the menu row (phase 2's `Popover`), each one text field or
select plus its verb: *New branch* (name, and "from" defaulting to the checked-out branch;
`branch_exists` and the `bad_request` answer shown under the field: git's `check-ref-format`
sentence, or the control plane's own for shorthand such as `@{-1}`; task 11 `ValidBranchName`),
*Rename branch* (the same name answers; `branch_has_worktrees` names them: "Remove or land this
branch's worktrees first — upload-api, scan-hook."; `branch_checked_out`, review Minor 16:
"evidence-upload is checked out in <path>. Renaming it would move that checkout to the new name.
Switch that checkout to another branch first."), *Delete branch* (the one line: "Delete evidence-upload? Its tip is kept as
refs/panacea/archive/… so nothing it named is lost; the remote branch is not deleted.") — a
question because the rail loses the branch and there is no undo verb for it in v1.

**Remove worktree** is `RemoveWorktreeQuestion`: a window (`--r-window`, `--e-overlay`) listing,
under four headings, the sessions that will be ended, the files that are not committed (these
are lost), the commits that never landed (these are kept under
`refs/panacea/archive/lanes/…`), and the agent's other branches or detached commit from
`details.elsewhere` (kept under `refs/panacea/archive/offbranch/…`; review I6). When
`details.unlisted` is present, that fourth heading says the agent's other branches **could not be
listed**, that all of them were archived anyway, and quotes git's sentence. When
`details.unarchivable` is present, it says those commits **cannot be kept and are lost**, and
quotes git's sentence. Buttons: **Remove** (danger) and **Keep it** (default focus).

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `EmptyDevelop offers a new worktree instead of a promise` | a branch with no worktrees → no text matching `/Phase 3/`; a button that calls `createWorktree(projectId, branchId)` then selects the result | the stale promise surviving |
| `New branch is enabled` | the project menu's `new-branch` item has `run` and no `disabledReason` | the phase 2 placeholder left in |
| `TopBar's Authority title names no phase` | the title does not match `/Phase \d/` and still names AGENTIC-UX §7 | 3-D10 unapplied |
| `an already-ready worktree points at Not ready` (review Minor 10) | `worktreeReadyItems` of a `ready: true` worktree → disabled, `disabledReason` names "Not ready" and "Merge lens" and does not match `/Phase \d/` | the phase 2 promise that "the merge queue in Phase 3 owns that" surviving phase 3 |
| `Remove worktree is disabled for primary and linked` | kinds `primary`, `linked` → disabled with the git worktree remove reason | the cockpit deleting a user's checkout |
| `the removal question names all four losses` | `inspectRemoval` answered `confirm_required` with sessions, paths, commits, elsewhere → four headed lists in the window; with `unarchivable` set, the fourth says those commits are lost and quotes it (review I6); with only `unlisted` set, the fourth heading is shown although `elsewhere` is empty, and quotes it (re-review b04-1 issue 2) | a question that hides what is lost |
| `Remove kills the sessions first, then deletes with the token` | order of calls: `killSession(s-1)`, `killSession(s-2)`, `removeWorktree(p, w, confirm)` | a DELETE refused for live sessions, or sandboxes left running |
| `a stale confirmation re-asks` | `confirm_stale` → the window re-renders with the fresh facts; no second DELETE until Remove again | removing on facts the human did not see |
| `Keep it is the default focus` | on open, `document.activeElement` is Keep it | Enter destroying work |
| `rename rebinds branch-keyed state` | branchDefaults, expandedBranches and selection under `b-old` move to `b-new`; PERSIST_VERSION unchanged | a renamed branch losing its defaults (A8) |
| `rename warns the remote keeps its old name` | `remoteKeepsOldName: true` → the toast says so | a silent divergence |
| `rename names the checkout it refuses for` (review Minor 16) | a 409 `branch_checked_out` with `details.path` → the form shows the path and the "Switch that checkout" sentence; no rebind happens | a refusal shown as a bare failure, or a rebind for a rename that did not happen |
| `delete asks, then deletes` | the one-line question; Delete → `deleteBranch`; Keep → nothing | a branch gone on one click with no undo |
| `sessionInNewWorktree creates then starts` | `createWorktree` then `createSession(newId, …)` in that order | a session started in the old, shared worktree |
| `the palette carries no destructive verb` | no command id starts with `remove:` or `delete:`; `land:` rows only for clean forecasts | a destructive verb one keystroke away (SX.6) |
| `palette ranking is untouched` | phase 2's `commands.test.ts` still passes unchanged | reordering phase 2's rows |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 04, 06, 11, 18, 19 merged (19: `MergeBanner.tsx`, which this
  task wires, and the Not ready button the new sessionVerbs copy names). The retirement grep,
  widened from `arrives in Phase 3` to `Phase 3` (review Minor 10):
  `git grep -n "Phase 3" -- desktop/src ":!desktop/src/styles/tokens.css" ":!desktop/src/styles/designsystem.test.ts" ":!desktop/src/defects.test.tsx"`
  lists exactly `AppShell.test.tsx` (the Authority entry), `EmptyDevelop.tsx` (`:73`, `:101`),
  `sessionVerbs.ts` (`:608`, `:624`) and `TopBar.tsx:29`: run on this tree at `80e79a9`, it listed
  those six lines plus four in `LinkedWorktreeNote.tsx` and `.test.tsx`, which task 06 rewrites (its
  test asserts no `/Phase \d/`). If a `LinkedWorktreeNote` line still appears, task 06 is not
  merged; stop. The three files left out say what a token or a defect anchor is for, and promise
  nothing: `tokens.css:73-77` and `designsystem.test.ts:271` (`--need`/`--drift` reserved "so Phase
  3 does not invent a second amber"), `defects.test.tsx:839-842` (why defects 7, 9 and 10 anchor
  outside Merge). Case matters: task 06's new comment says "phase 3 has shipped the fix".
- [ ] **Step 2:** tests; `actions.ts`, `lifecycleItems.ts`, the forms and the question; the edits
  to the phase 2 files, each confined to the block named above.
- [ ] **Step 3:** run; commit by pathspec.

#### Design-system checks (UI)

Popovers and the question use phase 2's `Popover` and the `--e-overlay`/`--r-window` pair; the
danger buttons use `--fail` as text on `--surface`, never a red fill wall; focus rings are the
designed `:focus-visible` ring. Nothing animates except the popover's existing open transition.
**The look is unverified until the owner sees it (OD-12).**

#### Acceptance

```
cd desktop && npx vitest run   → PASS (phase 2's ContextMenu, commands and EmptyDevelop suites included)
cd desktop && npx tsc --noEmit → no output
git grep -n "Phase 3" -- desktop/src ":!desktop/src/styles/tokens.css" ":!desktop/src/styles/designsystem.test.ts" ":!desktop/src/defects.test.tsx"   → no output
```

#### What this task does not claim

- Branch rename and delete act on the local repository only; the remote is untouched and the UI
  says so.
- Renaming a branch that has cockpit worktrees is refused (Q4).
