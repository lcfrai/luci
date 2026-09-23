### Task 18: Desktop — the Discover lens: binding, asking, the Brief born, the model paths, failures

Replaces `AppShell.tsx`'s `discover` placeholder with the lens (survey §2: "the lens arm is one
line"). **The specification is `prototype/phase-4-discover.html`** — states 1–6 and 14 in this task
(19 takes the rest) — read with the fact sheet. **Kept from the 2026-09-18 plan's task 11**
(`:4958-5580`): one question at a time, numbered pickable options, a free-text escape always on
screen, the settled list with per-answer *Edit*, a completion state naming the file. **Changed:**
prop-less, reading `discoverSlice` (C11); ops through task 15's client; the Hub's failure states.

**Read `docs/DESIGN-SYSTEM.md` first.** Held here: no gradient and no indigo/violet; elevation is a
1px ring plus layered shadow; radius by size (`--r-ctl` on options and fields, `--r-card` on the
panel); one accent, `--accent`, for interactive and current state **only** (the selected option,
the focused field), never decoration; `transform`/`opacity` only, never `transition: all`;
**motion only where it explains a cause** — the key flashes and decays over ~600 ms on
`opacity` **only when a save mints a new complete revision**, never on revisit and never as an
entrance animation; `QUESTION n OF 5` is the one denominator (G5), and a follow-up shows none;
`tabular-nums` on `rev n`; no spinner — an in-flight save changes the send button's words to
*Saving* and disables it; the one heartbeat (`hooks/heartbeat.ts`) for any live dot; keyboard
first: `1`–`4` pick, `Enter` sends, `E` edits the focused answer — **element-scoped handlers on
the question region, never a window listener** (G9); `:focus-visible` rings designed, not default.
**The owner's eyes are the only visual acceptance** (`OD-12`).

**Files:**
- `desktop/src/discover/DiscoverLens.tsx` — the lens, prop-less (new)
- `desktop/src/discover/QuestionFlow.tsx`, `BriefPanel.tsx`, `BindingState.tsx`, `useBriefView.ts` (new)
- `desktop/src/discover/discover.css` (new)
- `desktop/src/discover/DiscoverLens.test.tsx`, `QuestionFlow.test.tsx`, `BriefPanel.test.tsx` (new)
- `desktop/src/components/AppShell.tsx` — the `discover` arm returns `<DiscoverLens />` (modify)
- `desktop/src/components/AppShell.test.tsx` — the placeholder expectation for `discover` repointed (modify)

**Interfaces — complete source:**

```ts
// desktop/src/discover/useBriefView.ts
/** Loads the BriefView for the current selection: once on mount, once per target change, and
 *  once when discoverStale names it (task 16's feed). Never on a timer, never per render.
 *  Returns the view and its load state from discoverSlice. */
export function useBriefView(target: Target | null): {
  view: BriefView | undefined; load: DiscoverLoad; error: { code: string; message: string } | null;
};

/** One op at the head the view shows. Resolves with the response and writes the new view to
 *  the slice; rejects with the ApiError unchanged so the caller keeps the typed text. */
export function useApplyOp(target: Target): (op: Op) => Promise<OpsResponse>;
```

```ts
// desktop/src/discover/DiscoverLens.tsx
export function DiscoverLens(): JSX.Element; // reads selection.projectId / selection.branchId
```

```ts
// desktop/src/discover/QuestionFlow.tsx
export interface QuestionFlowProps {
  view: BriefView;                     // step, scope and inherited come from here
  onAnswer(op: Op): Promise<void>;     // record / skip / propose_follow_up / dismiss_follow_up
  saving: boolean;
  /** The typed text survives every failure (state 14): the flow owns the draft and only
   *  clears it after the op resolves. */
}
export function QuestionFlow(props: QuestionFlowProps): JSX.Element;

// desktop/src/discover/BriefPanel.tsx
export interface BriefPanelProps {
  view: BriefView;
  justMinted: string | null; // the key a save just made complete; drives the one flash
  onEdit(key: string, text: string): Promise<void>;
  onAccept(key: string): Promise<void>;
}
export function BriefPanel(props: BriefPanelProps): JSX.Element;

// desktop/src/discover/BindingState.tsx
export function BindingState(props: { projectId: ProjectId; binding: Binding }): JSX.Element;
```

**What each state shows** (prototype state numbers; copy from the prototype, final wording the
owner's):

| # | When | Shows |
|---|---|---|
| — | `binding.state: "hub_off"` | *Discover keeps briefs in the Hub, which this machine is not connected to.* The variable name, `PANACEA_APP_HUB_URL`, in mono. Develop is unaffected |
| — | `no_remote` | OQ-1's sentence and the one command, `git remote add origin <url>`, in mono |
| — | `unbound` | *Share this project's brief with your team*: a team picker from `binding.teams` (members and above only) and **Share**, which calls `bind` |
| — | a `signed_out` problem | *Sign in to see and edit this brief*, pointing at the rail footer's sign-in (the Hub's task C4 surface) |
| 1 | bound, no brief anywhere | *No brief yet. Sessions here inherit nothing from Discover.* Start answering, or *Skip for now* (no revision is written) |
| 2 | branch, no branch brief, project complete | what the branch inherits, and from which project revision (`inherited.key` in mono) |
| 3 | asking | `QUESTION n OF 5`, the prompt, help, options `1`–`4`, the free-text field, *Keep the project answer* (branch) or *Skip this question* (project), and the inherited project answer beside it |
| 4 | the save that made the brief complete | the key, named, with one flash; `preview.block` verbatim in mono; where it is written (task 19's materialisations list) and who inherits it — *nobody yet* when no session runs |
| 5 | a follow-up step | the question marked *drafted by the model*, its `because`, **no denominator**; *Not relevant — skip* dismisses it |
| 6 | `assist: "model_off"` or `"model_failed"` | one sentence naming why follow-ups are missing (`PANACEA_APP_MODEL_API_KEY` for model_off), and the flow carries on |
| 14 | `control-plane-down`, or `upstream_unavailable` | *Not saved.* The typed text stays where it was typed, and **Retry** re-sends the same op |
| — | `revision_conflict` | *Someone saved revision n first.* The view re-reads; the typed text stays; nothing is merged |
| — | 400 `bad_request`, `details.field: "text"` (the marker rule, plan §4 Security, S3; task 11) | the server's `message` verbatim, inline at the answer field of the question `details.key` names (the current step's field when it is that question); the typed text stays; **no Retry**, because re-sending the same text is refused again. Typically a pasted copy of a `CLAUDE.md` block, not an attack |

**The model follow-up is asked once, automatically**, after the third settled base answer, by the
op `propose_follow_up`; the server refuses a second (`already_asked`) and asks nothing before three
(`not_yet`), so the lens never counts.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `AppShell routes discover to the lens` | `lens: "discover"` renders `DiscoverLens`, not `LensPlaceholder` | the placeholder surviving |
| `the flow shows QUESTION n OF 5 and nothing else numeric` | step 2 → `QUESTION 2 OF 5`; no other `n of m` in the lens | a progress bar in words |
| `a follow-up has no denominator` | a `follow_up` step shows no `OF` | G5 |
| `1-4 pick and Enter sends, scoped to the region` | keydown `2` on the region selects option 2; `Enter` sends a `record` op with `source: "picked"`; the same keys dispatched on `window` do nothing | a second window listener (G9) |
| `typing a free answer sends source typed` | `record` with the trimmed text | a typed answer labelled as picked |
| `branch skip is "Keep the project answer"` | branch scope shows that label and sends `skip` | a project-scope label on a branch |
| `a failed save keeps the typed text` | client rejects `network_unavailable` → text still in the field, *Not saved*, **Retry** re-sends the identical op | state 14's promise |
| `reserved text is refused at the field, without Retry` | client rejects 400 `bad_request` with `details: {field: "text", key: "purpose"}` on the `purpose` step → the server's message is shown at that answer field; the typed text is still in the field; no **Retry** control and no *Not saved*; no toast | the marker rule's 400 shown as state 14 (a Retry that is refused forever) or as a toast that discards the text (re-review M3) |
| `a Hub outage says the Hub, not the control plane` | `upstream_unavailable` → a sentence about the Hub | "the control plane is down" when it is not |
| `a conflict re-reads and keeps the text` | 409 `revision_conflict` → one `view()` call; text kept | a silent overwrite of a teammate |
| `the key flashes once, on mint only` | the save that makes the brief complete sets `justMinted`; re-rendering the same view does not re-flash | motion with no cause |
| `the follow-up is asked once after three answers` | the third settled answer is followed by one `propose_follow_up`; the fourth and fifth are not | a model call per answer |
| `model off carries on` | `assist: "model_off"` → the sentence names the variable; step 4 is still asked | a flow that stalls without a model |
| `unbound offers Share with the caller's teams` | picker lists teams with role ≥ member; Share calls `bind(projectId, teamId)` | a viewer offered an action the Hub will refuse |
| `hub_off leaves Develop alone` | the lens renders its sentence; no ApiError toast is pushed | Discover's absence reported as an error on every screen |
| `stylesheet obeys the design system` | `discover.css`: no hex, no `-gradient(`, no `transition: all`, no `color: var(--ink-4)`, no `@keyframes` except the one flash | the phase 2 audit failing at merge |

**Steps:**

- [ ] **Step 1:** Open `prototype/phase-4-discover.html` states 1–6 and 14 in a browser and read
  its CSS; read `DESIGN-SYSTEM.md`, `tokens.css`, `components/common/common.css`.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The hook, the four components, the CSS; the `AppShell.tsx` arm and its test.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. **Do not merge** (`OD-1`).

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
$ grep -cE "^[[:space:]]+return <DiscoverLens" desktop/src/components/AppShell.tsx
1
$ grep -rn "addEventListener(\"keydown\"\|addEventListener('keydown'" desktop/src/discover/
(no output — G9)
```

Owner-observed, **unverified until the batched session** (`OD-12`): the look of every state, the
flash, keyboard feel, that the copy reads as intended.

**Does NOT claim:** moved-on, project-changed, undo, past revision, missing, erased and hand-edited
states (task 19); the session-start refusal (task 20); the repo import (task 21).
