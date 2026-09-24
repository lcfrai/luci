### Task 20: Merge lens, part 2 — the remote bar, push, open a PR, the landed history with its two undos, and recording evidence

This task finishes the Merge lens. The **remote bar** says where the branch stands against its
remote as of the last fetch, and carries Fetch, Push and Open PR (prototype `topMerge`, lines
449-460; state `m-push`). The **landed history** is first-parent, newest first, each land with its
merge link and the undo git allows — exact or a revert, never both offered as one (prototype
`landedList`, lines 539-556; state `m-landed`). The **evidence dialog** records 3-D5's verdict and
required *because* at a commit.

**Files:**
- `desktop/src/components/lens/merge/RemoteBar.tsx`, `LandedList.tsx`, `BriefKeyChip.tsx`, `EvidenceDialog.tsx`, `remote.ts` (new)
- `desktop/src/components/lens/merge/model.ts` — the sentences below appended (task 19's file)
- `desktop/src/components/lens/MergeLens.tsx` — mount the bar and the list (task 19's file)
- `desktop/src/components/lens/merge/DoDSlot.tsx` — the evidence control (task 19's file)
- `desktop/src/components/lens/merge/merge.css` — appended under a `/* ── task 20 ── */` header
- tests beside each (new)

#### Interfaces — complete

```ts
// merge/model.ts — appended.

/** The remote bar's status line:
 *  down          → "origin/evidence-upload · last reading 15:40"
 *  no remote     → "evidence-upload · this project has no remote"
 *  never pushed  → "evidence-upload · never pushed"
 *  ahead > 0     → "1 commit not pushed · origin/evidence-upload fetched 15:36"
 *  behind > 0    → "…, and 2 commits on origin/evidence-upload are not here" (task 19's
 *                  Bring in row is the way in; review I5)
 *  up to date    → "origin/evidence-upload · up to date · fetched 15:36"
 *  never fetched → "… · never fetched"
 *  fetchError    → appended: " · the last fetch failed" (the sentence is the bar's title text)
 *  Times are local HH:MM from RFC 3339, in tabular-nums. */
export function remoteLine(conv: Convergence, down: boolean, lastReadAt: string): string;

/** One history row's second line, e.g.
 *  land   → "reviewed against <chip> · sessions s-3f2a91c0 · not pushed — undo is exact"
 *  direct → "no brief — not a land · no session recorded"
 *  revert → "takes back d5f0a17"
 *  A land a later row reverts adds: "reverted by 9ab01c2 — landing this worktree again brings
 *  nothing back; its agent must revert the revert or make new commits". */
export function historyMeta(entry: HistoryEntry, all: HistoryEntry[]): string;

/** Whether Push is offered, and the reason when not ("Nothing to push: the branch matches
 *  origin", "The control plane is not answering", "This project has no remote"). */
export function pushBlock(conv: Convergence, down: boolean): string | null;
```

```ts
// merge/remote.ts — the three remote verbs. Each runs, then reload(), then one toast or one banner.
export type RemoteProblem =
  | { verb: "fetch" | "push" | "pr"; reason: ConflictReason | undefined; message: string; gitSays?: string; hostSays?: string };

export function fetchNow(projectId: string, reload: () => void): Promise<RemoteProblem | null>;
export function pushNow(conv: Convergence, reload: () => void): Promise<RemoteProblem | null>;
/** Opens the PR, then opens its webUrl in the system browser through @tauri-apps/plugin-opener
 *  (dynamic import, as ContextMenu.tsx:43-45 does). Toast: "Opened PR #7 on github.com." or
 *  "PR #7 was already open." */
export function openPR(conv: Convergence, briefKey: string, reload: () => void): Promise<RemoteProblem | null>;
```

```tsx
export function RemoteBar(props: { conv: Convergence | undefined; down: boolean; lastReadAt: string;
  problem: RemoteProblem | null; onProblem(p: RemoteProblem | null): void }): JSX.Element;
export function LandedList(props: { conv: Convergence; history: HistoryEntry[]; down: boolean; reload(): void }): JSX.Element;
/** The phase 4 seam for rendering a brief key in the contract's six-state vocabulary. Until
 *  phase 4's resolver exists: "" → `none` ("no brief was shown when this landed"); a key →
 *  `unavailable` (the key in mono, "cannot be resolved on this machine"). Phase 4 replaces the
 *  body with its own chip over the resolved Ref; nothing here parses the key (P5). */
export function BriefKeyChip(props: { briefKey: string }): JSX.Element;
export function EvidenceDialog(props: { conv: Convergence; briefKey: string;
  criterion: { id: string; digest: string; text: string }; onClose(): void; onRecorded(): void }): JSX.Element;
```

**Push failures** (`m-push`). `git_failed` on push shows: *"Push did not run: <message>."* then
git's sentence verbatim in mono, then *"The control plane pushes with this machine's own git
sign-in and never opens a sign-in prompt. Sign in once with git in a terminal on this machine,
then push again. The land stays; its undo is still exact."* — the same text for every `git_failed`
on push, so it never depends on reading git's words. `push_rejected`: *"The remote has commits
evidence-upload does not have. Fetch, then bring them in above the queue, and push again; the
cockpit never force-pushes."* with git's sentence, and **Fetch** (`api.fetchProject`, then
reload), after which task 19's **Bring in origin/evidence-upload** row appears. **Push again**
retries. Every verb that sentence names exists (review I5: the old copy promised a way in the
cockpit did not have).

**Open PR** is disabled while down or with no remote. `not_pushed` shows *"Push first: the pull
request would not show the commits that are only here."* with **Push**. `no_credential`: *"No
sign-in for <host> is stored for the control plane. Sign in to <host> with git on this machine
(its credential manager keeps it), then try again."* — the Hub replaces this sentence with its
own sign-in once H-1 exists. `unknown_host`: the server's message. `host_failed`: the host's own
words (`hostSays`) and the status.

**History rows**: the sha in mono; *land <name>* / the subject with "— committed on <branch>
directly, not a land" / *revert of <sha>* / the subject with "— the cockpit brought in the
remote's commits" (`kind: "bring-in"`, task 10; re-review b05 issue 5); `historyMeta`; the time in `tabular-nums`. The verb:
`undo: "exact"` → **Undo** (runs, reloads, toast "Undid the land of upload-ui." — no undo of the
undo is offered, the lane simply re-enters the queue); `undo: "revert"` → **Revert…**, which
expands an inline line in the row, *"Land a commit that takes back d5f0a17? [Revert] [Keep it]"*,
because a revert of pushed history has no exact undo; `undo: "none"` → no verb. An `undoWhy` of
`pushed` or `built_upon` is the title text of the Revert button.

**Evidence.** `DoDSlot` (task 19) gains, once phase 4's `DefinitionOfDone` mounts there, a
*Record evidence…* control per criterion (through phase 4's `onRecordEvidence` prop, ask P4-5; if
phase 4 ships without it, one control under the component that opens a criterion picker listing
the Ref's criteria verbatim). `EvidenceDialog` shows the criterion's text as the Ref has it, the
commit it records against — **at the branch head** (`conv.tip`) or **in <worktree>** (that lane's
`tip`), chosen by the human, with the short sha shown — the verdict (*settled* or *contested*),
and *Because*, required, 1-2000 characters, the Record button disabled until it has one. When the
store is not shared it says *"Recorded on this machine only — teammates will not see it until the
Hub stores evidence."* Until phase 4 exists the dialog is built and tested but unreachable, and
the report says so.

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `remoteLine pins every state` | the eight lines from fixtures, byte for byte | "0 ahead" for a never-pushed branch |
| `Push is disabled with its reason` | up to date → disabled, title "Nothing to push: the branch matches origin" | a push offered for nothing |
| `a push failure shows git's sentence and the fixed why` | `git_failed` with `gitSays` → the banner with the sentence in mono and the fixed text | parsing git's words to decide the advice |
| `a rejected push never offers force` | `push_rejected` → the banner; no element in the lens contains "force" as an action | a force-push button |
| `a rejected push leads to Bring in` (review I5) | `push_rejected` → the sentence names "bring them in" and a **Fetch** button; click → `api.fetchProject` once, then a reload whose `bringIn` is set → task 19's row is present | copy that promises a way in the lens does not offer |
| `Open PR asks for a push first` | `not_pushed` → the sentence and a Push button that calls `pushBranch` | a PR against the remote's stale commits |
| `Open PR opens the browser once` | success → `openUrl(webUrl)` called once; `existing: true` → the "already open" toast | a PR link the user has to hunt for |
| `no credential says where to sign in` | `no_credential` with `details.host` → the sentence naming the host | "Something went wrong" |
| `history renders land, direct, revert and bring-in` | four entries → four rows with the prototype's titles; the direct row's chip is `none` "no brief — not a land"; the bring-in row reads "the cockpit brought in the remote's commits" and has no verb | a terminal commit dressed as a land, or the cockpit's bring-in dressed as a terminal commit |
| `BriefKeyChip never parses` | a key `brief:b-42@6` renders the whole string in mono under `unavailable`; the component source contains no `split(`, `indexOf("@")` or a regex over the key | P5 |
| `exact undo is offered only when exact` | `undo: "exact"` → Undo; `"revert"` → Revert…; `"none"` → nothing | offering an exact undo of pushed history |
| `Undo undoes with the tip the human saw` | click → `undoLand(sha, expectedTip = conv.tip)` | undoing a land that is no longer the tip |
| `Revert asks inline, then reverts` | Revert… → the inline line; Keep it → no call; Revert → `revertLand(sha, conv.tip)` | an unasked history-changing commit |
| `a reverted land says it cannot simply re-land` | a land followed by a revert naming it → the "reverted by …" sentence on the land's row | a user re-landing and seeing nothing happen |
| `EvidenceDialog requires a because` | empty or whitespace → Record disabled; typed → enabled; 2001 chars → refused with the limit | a non-open state without its because (P12) |
| `EvidenceDialog records at the chosen commit` | choose "in upload-ui" → `recordEvidence` with that lane's tip and the criterion's id and digest | evidence at a commit the human did not pick |
| `the unshared label` | `shared: false` → "Recorded on this machine only …" | a local record passed off as shared |
| `design-system audit` | `merge.css` still passes task 19's audit after the append | the append reflowing or adding a banned rule |

#### Steps

- [ ] **Step 1: Preflight.** Task 19 merged; `grep -c "── task 20 ──" desktop/src/components/lens/merge/merge.css` is 0.
- [ ] **Step 2:** `model.ts` additions with tests; `remote.ts`; components; mounts.
- [ ] **Step 3:** run; commit by pathspec.

#### Design-system checks (UI)

The bar is one row on `--canvas` with a `--line-soft` hairline by `box-shadow`; its status text is
`--ink-2` with numbers in `tabular-nums`; Fetch, Push and Open PR are `.btn` at `--r-ctl`, and
none is the accent unless it is the one next action (Push when ahead). History rows are a single
card (`--e-card`, `--r-card`) of rows separated by hairlines; the fresh row flashes its background
for 600 ms once, which is the causality the land created. Git's and the host's sentences are mono
at `--ink-2`, never red text — the failed square shape carries the state. **The look is unverified
until the owner sees it (OD-12).**

#### Acceptance

```
cd desktop && npx vitest run src/components/lens/merge/   → PASS
cd desktop && npx tsc --noEmit                             → no output
```

#### What this task does not claim

- Evidence recording works end to end only after phase 4 (resolver and `DefinitionOfDone`) and is
  shared only after the Hub (H-2).
- The PR's review, checks and merge are the host's.
- Whether the owner's machine's credential manager has a token for the remote is the machine's
  state; this task renders what the server says about it.
