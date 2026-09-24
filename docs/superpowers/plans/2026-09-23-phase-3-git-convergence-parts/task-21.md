### Task 21: Files lens — changed paths across the branch, one path's hunks, their back-links, and the span drawer

3-D9: Files is **what is changing, and who is changing it**, per path, across every worktree of
the branch — not a file browser (the agent has an editor; survey §8.6). The composition is the
prototype's (`filesLens()`, `prototype/phase-3-convergence.html:647-672`, states `f-now`,
`f-open`, `f-empty`, `f-gitfail`): a title row, a list of changed paths grouped by directory with
the worktrees touching each and the mark that says which kind of fact an overlap is, and a view of
the open path's hunks per worktree, each carrying its back-links — tier 1 (commit and session)
always, tier 2 (the tool call that wrote it, OD-8) when the adapter could read the transcript, and
the reason when it could not (`hunkHtml`, lines 638-646).

**Files:**
- `desktop/src/components/lens/FilesLens.tsx` — task 26's stub, filled (the only lens file this task edits)
- `desktop/src/components/lens/files/PathList.tsx`, `PathDiff.tsx`, `HunkView.tsx`, `BackLinks.tsx`, `SpanDrawer.tsx`, `model.ts` (new)
- `desktop/src/components/lens/files/files.css` (new; first line the tokens import)
- `desktop/src/styles/tokens.css` — `--diff-add`, `--diff-del`, **only if absent** (check first; phase 2 rule 2)
- tests beside each (new)

#### Interfaces — complete

```ts
// files/model.ts
import type { BackLink, Changes, LaneDiff, PathMark, PathRow } from "../../../api/convergence";

/** Paths grouped by directory, directories sorted, paths sorted within each. */
export function groupByDir(changes: Changes): { dir: string; rows: PathRow[] }[];

/** The mark's sentence, or null:
 *  "conflicts" → "Conflicts on landing — whichever of <a> and <b> lands second"
 *  "both-edit" → "Both edit this file — not committed, so it cannot be forecast yet" */
export function markLine(row: PathRow, name: (id: string) => string): string | null;

/** The shape for a mark: conflicts → "need" (triangle), both-edit → "none" (hollow ring). */
export function markShape(mark: PathMark): "need" | "none" | null;

/** Tier 1's sentence: "commit a41c9e2 · session s-3f2a91c0 · 14:02", with "(shell)" after a shell
 *  session; "commit a41c9e2 · no session recorded · 14:02"; for tier 0: "Not committed, so no
 *  commit and no session is recorded for it yet." plus, when the lane has two live sessions,
 *  " Two sessions share this worktree, so the commit that takes it may be either's." */
export function tier1Line(back: BackLink, sharedSessions: boolean): string;

/** Tier 2's sentence when tier === 2 ("the Edit tool call that wrote these lines"), else the
 *  adapter's reason verbatim ("written by claude 2.1.300; the adapter reads 2.1.274"), or null
 *  for tier 0. */
export function tier2Line(back: BackLink): string | null;

/** A lane's notes, in order: its readError pair ("<name>'s change to this file is not shown."
 *  then the server's sentence), "Binary file — no text diff.", then one sentence per cap set
 *  (truncated, untrackedCut). Empty for a lane with none. See States below. */
export function laneNotes(lane: LaneDiff): string[];
```

```tsx
export function FilesLens(): JSX.Element; // mounts useConvergenceLoader("files")
export function PathList(props: { changes: Changes; open: string | null; onOpen(path: string): void }): JSX.Element;
export function PathDiff(props: { diff: PathDiff; row: PathRow | undefined; onOpenSession(id: string): void; onOpenSpan(sessionId: string, toolUseId: string): void }): JSX.Element;
export function HunkView(props: { worktreeName: string; hunk: Hunk; committed: boolean; sharedSessions: boolean;
  onOpenSession(id: string): void; onOpenSpan(sessionId: string, toolUseId: string): void }): JSX.Element;
export function BackLinks(props: { back: BackLink; sharedSessions: boolean;
  onOpenSession(id: string): void; onOpenSpan(sessionId: string, toolUseId: string): void }): JSX.Element;
/** A drawer, not a modal: the diff stays visible beside it. Shows the tool, the file, the
 *  time, the agent's words before the call ("before"), and the old and new strings, all as the
 *  transcript has them — rendered as text, never as HTML. */
export function SpanDrawer(props: { sessionId: string; toolUseId: string; onClose(): void }): JSX.Element;
```

**Opening a path** sets `openPathByBranch` (task 18) and the loader fetches its diff on the next
load (and immediately, through `reload()`). **Open session** focuses that session in Develop
(`focusSession` then `setLens("develop")`, both phase 2 actions); it does not seek the terminal to
a time, and the link says "open session", not "open at 14:02". **Transcript span** opens the drawer
through `api.getTranscriptSpan`; a `transcript_unreadable` answer shows `details.why` in the
drawer. Keyboard: the path list is a listbox (↑/↓ move, Enter opens), the drawer closes on Esc and
returns focus to the link that opened it.

**States.** `f-empty`: *"Nothing is changing on <branch>."* / *"Every worktree's work has landed
and none has changed a file since."*; a branch with no cockpit worktrees: *"No worktree on
<branch> yet."*; no path open: *"Choose a path to see each worktree's change to it, and where each
change was written."*; `f-gitfail`: per `Unreadable` entry, *"<name>'s changes are not shown — git
could not read it. The other worktrees are current."* with git's sentence; the control plane down:
the last reading dimmed, as Merge does. A binary change says *"Binary file — no text diff."*.
Each cap has its own sentence, and a lane shows both when both flags are set (task 16, re-review
b11-1 issue 2):
- `truncated`: *"Showing the first 2000 changed lines."*
- `untrackedCut` with lines: *"This file is not tracked and is over 1 MiB, so only its first
  1 MiB is shown."*
- `untrackedCut` with no lines: *"This file is not tracked and is over 1 MiB, with no line break in
  its first 1 MiB, so none of it is shown."*

A lane's own `readError` is shown in that lane's place in the diff, as *"<name>'s change to this
file is not shown."* with the server's sentence verbatim as text beneath it: git's, or task 16's
*"<path> is a link or not a regular file in this worktree; not read"*. This is apart from the
path-list `Unreadable` banner above. The lane keeps its heading, so the user never sees an empty
lane with no reason. The other lanes render as usual.

Rows **never reflow** while the poll refreshes: a path keeps its row, and a path that stops
changing leaves at the next load without animating out (`DESIGN-SYSTEM.md`: hover and refresh
never change layout; motion must explain causality). Lists over 50 paths are virtualised (the
Floor rule) with a fixed row height and a window computed from `scrollTop` — **no new
dependency**: `desktop/package.json` has no virtualisation or motion library (read 2026-09-23), and
task 19's reorder FLIP uses WAAPI (`element.animate`) for the same reason (`DESIGN-SYSTEM.md`:
CSS > WAAPI > JS library).

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `groupByDir groups and sorts` | six paths in three directories → three groups, sorted | a flat unsorted list |
| `markLine distinguishes fact from suspicion` | `conflicts` → the landing sentence naming both; `both-edit` → the not-forecast sentence; `""` → null | an overlap drawn as a conflict (R9) |
| `a conflict mark is a triangle and a suspicion a hollow ring` | `markShape` → `need` / `none` | colour alone carrying the state |
| `tier1Line pins every form` | agent session, shell session, no trailer, uncommitted, uncommitted-and-shared | "no session recorded" worded two ways for two unknowable causes |
| `tier2Line shows the adapter's reason verbatim` | tier 1 with reason "written by claude 2.1.300; …" → that string | hiding why the deeper link is missing |
| `the list shows only changed paths with their worktrees` | `aChanges()` → one row per path, a "not committed" tag on uncommitted touches | a full file tree |
| `opening a path loads its diff now` | click → `openPath` set and `getPathDiff` called without waiting for a tick | a two-second pause on every click |
| `hunks render per worktree with committed and not` | a `PathDiff` with a committed and an uncommitted hunk → both, labelled, `+` and `-` lines distinguishable by more than colour (a leading sign in mono) | a diff readable only in colour |
| `open session focuses Develop` | click → `focusSession(id)` and `setLens("develop")` | a link that goes nowhere |
| `the span drawer shows the call and the words before it, as text` | a span whose `before` contains `<img src=x onerror=…>` → rendered as literal text, no `img` element | transcript text injected as HTML |
| `the drawer says why when unreadable` | `transcript_unreadable` → `details.why` in the drawer | a blank drawer |
| `an unreadable worktree is withheld with git's sentence` | `unreadable: [scan-hook]` → the banner; its paths absent | one failure blanking the lens |
| `a lane's readError is shown in its place` (re-review b11-1 issue 2) | a `PathDiff` of two lanes, one with `readError` "src/a.ts is a link or not a regular file in this worktree; not read" and no hunks → that lane's heading, "upload-api's change to this file is not shown." and the sentence verbatim; the other lane's hunks render | an empty lane with no reason, or a security refusal hidden |
| `each cap says which it is` (re-review b11-1 issue 2) | `truncated` → the 2000-line sentence only; `untrackedCut` with lines → the 1 MiB sentence only; `untrackedCut` with none → the no-line-break sentence; both flags → both sentences | a byte cap reported as the 2000-line cap, or a blank lane for a one-line bundle |
| `rows keep their order across a refresh` | two loads with the same paths in a different server order → the same DOM order | rows jumping under the pointer |
| `over 50 paths virtualise` | 200 paths → fewer than 200 row elements mounted | a 2000-row list stalling the window |
| `no interval, no spinner` | as task 19's | a second clock |
| `design-system audit` | `files.css`: no `gradient(`, `transition: all`, `1px solid`, raw colour; `--diff-add`/`--diff-del` used only as backgrounds with the sign column carrying meaning | the tells DESIGN-SYSTEM.md bans |

#### Steps

- [ ] **Step 1: Preflight.** G2 for `FilesLens.tsx`; task 18 merged; `grep -c "diff-add" desktop/src/styles/tokens.css`
  — **0: add both tokens** under phase 2's rule 2 (OKLCH, derived from `--run` and `--fail` at low
  alpha, e.g. `oklch(69.8% 0.1671 148 / 0.10)` and `oklch(66.7% 0.1830 32.3 / 0.10)`, with a comment
  naming this task), and update `tokens.test.ts` if it enumerates tokens; **≥ 1: use them as they
  are.** `--src` from the prototype is prototype chrome and is **not** added.
- [ ] **Step 2:** `model.ts` tests and functions; components with tests.
- [ ] **Step 3:** run; commit by pathspec.

#### Design-system checks (UI)

Split view: the list on `--canvas`, the diff on `--surface`, divided by a `--line` hairline
(`box-shadow`, not a border). Hunks are cards (`--e-card`, `--r-card`) with the code in `--mono`
and a sign column; added and removed lines take `--diff-add` / `--diff-del` backgrounds, and the
sign carries the meaning for anyone who cannot see the colour. Back-links are one line each, tier
label in `--ink-3`, links in `--accent-hi` (accent text is the interactive state). The drawer is
`--e-overlay` at `--r-window` and slides on `transform` only, 180 ms (`--dur`), house curve, and
appears without motion under `prefers-reduced-motion`; it explains where the span came from by
opening beside the hunk that opened it. No gradient, glass or indigo. **The look is unverified
until the owner sees it (OD-12).**

#### Acceptance

```
cd desktop && npx vitest run src/components/lens/files/ src/components/lens/FilesLens src/styles/  → PASS
cd desktop && npx tsc --noEmit                                                                     → no output
```

#### What this task does not claim

- Tier 2 appears only for Claude Code sessions whose transcript the adapter can read, and it is
  unverified against the pinned CLI until task 17's captured fixture exists.
- `CLAUDE.md`'s absence from the list (the prototype's 3-D6 footnote) is phase 4's to make true;
  this lens lists whatever git reports as changed.
- The look is the owner's to judge.
