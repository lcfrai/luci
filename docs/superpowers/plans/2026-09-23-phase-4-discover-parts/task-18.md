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
**Typing is text** (review I12). The rules below hold for **every answer field in the question
region**: the base question's free-text field (state 3, prototype `:482`) and the follow-up's
answer field (state 5, prototype `:513`), which is this task's too (`:4`, row 5 below). Re-review
b07-1 R-M3.
- **The region's target check.** Each answer field sits inside the question region, so its
  keydown events bubble to the region's `onKeyDown`. The region handler returns first, doing
  nothing, when the target is a form field. `event.target` is an `EventTarget`, which has no
  `closest`, so the check narrows first (re-review b07-1 N3):
  `const t = event.target; if (t instanceof Element && t.closest('input, textarea, select,
  [contenteditable]:not([contenteditable="false"])')) return;`
  Inside a field, `1`–`4` and `E` are characters.
- **Enter on a control is the control's** (re-review b07-1 R-M5). The options, the ↵ Save and
  the per-answer *Edit* are buttons (prototype `:481`, `:482`, `:433`). `Enter` on a focused
  button fires its native click **and** bubbles to the region. So the region's `Enter` also
  returns when `event.defaultPrevented`, or when the target is inside `button, a[href],
  [role="button"]`. Without this, one `Enter` could send two `record` ops (the second a
  `revision_conflict`), or pick one option and send another. `1`–`4` and `E` are not affected: a
  button does not act on them. What each button's activation does, so the one act is defined:
  an option button sends its option at once, as the prototype's `onclick="P.answer(j)"` does
  (`:481`); ↵ Save sends the typed answer (`:482`, `P.free()`); *Edit* opens that answer
  (`:433`). The digits only select, and the region's `Enter` sends the selection.
- **Where focus lands** (re-review b07-1 R-M4). When a new step renders (its key changes), focus
  moves to the question region itself: `tabIndex={-1}`, labelled by the prompt through
  `aria-labelledby`, so a screen reader reads the question. Focus never starts in a field,
  because an auto-focused field would turn `1`–`4` into text and leave the keyboard-first flow
  dead until a Tab. It moves only when focus is already inside the lens or on `document.body`,
  so a re-read that brings a new step never pulls focus out of the rail or another surface. A
  failed save (state 14) keeps the step, so focus stays in the field with the typed text.
- **The field's rule is the prototype's** (`prototype/phase-4-discover.html:689-693`, the
  specification this task follows; `:690` sends, `:691` returns for form fields). `Enter` sends
  the typed answer. `Shift+Enter` is a new line. The field's own `onKeyDown` handles this and
  calls `preventDefault` on every unshifted `Enter`. An empty or whitespace-only field sends
  nothing on `Enter` and gains no new line (the prototype's `if(v)` at `:685`, with
  `preventDefault` at `:690` either way; re-review b07-1 N4). Nothing is sent while an IME
  composition is open (`event.nativeEvent.isComposing`), because that `Enter` confirms the
  composition. To pick an option again, the person tabs out of the field.
- **The follow-up field follows the same rule.** In the prototype a plain `Enter` there is a new
  line: the rule at `:690` is bound to `id="free"`, and the follow-up `textarea` at `:513` has
  no id. That is a gap in the prototype, not a second rule. Its ↵ Save is unwired and only
  toasts (`:513`), and two fields that look alike (`div.ff`, a `textarea` and ↵) with different
  `Enter` behaviour would surprise the person. The owner question on `Enter` versus `Ctrl+Enter`
  covers both fields.

The review offered `Ctrl+Enter` or a stated rule. This plan states the prototype's rule, because
the owner reviewed the prototype and a chat-style field that ignores a plain `Enter` would surprise
its users.
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
| — | `unbound` | *Share this project's brief with your team*: a team picker from `binding.teams` (members and above only) and **Share**, which calls `bind`. When `binding.matches` is not empty (review M8, a fork's clone), one line per match comes first: *Your `upstream` remote (`github.com/team/portal`) is already shared as* ***Engagement Portal****. Sharing creates a separate project for `origin`, `github.com/me/portal`.* Remote names and keys in mono. The choice is **Share** as its own project, or leaving it unshared; nothing binds to the matched project (OQ-12). A name field sits beside the team picker, pre-filled with `binding.defaultName` (a fork's is qualified, `portal (me/portal)`, so it does not collide with the team's `portal`; b10 re-review I-2) and editable; **Share** sends it. On 409 `project_name_taken` the server's message shows at the name field, the typed name stays, the field takes focus, and there is no Retry: the person changes the name and presses **Share** again |
| — | a `signed_out` problem | *Sign in to see and edit this brief*, pointing at the rail footer's sign-in (the Hub's task C4 surface) |
| 1 | bound, no brief anywhere | *No brief yet. Sessions here inherit nothing from Discover.* Start answering, or *Skip for now* (no revision is written) |
| 2 | branch, no branch brief, project complete | what the branch inherits, and from which project revision (`inherited.key` in mono) |
| 3 | asking | `QUESTION n OF 5`, the prompt, help, options `1`–`4`, the free-text field, *Keep the project answer* (branch) or *Skip this question* (project), and the inherited project answer beside it |
| 4 | the save that made the brief complete | the key, named, with one flash; `preview.block` verbatim in mono; where it is written (task 19's materialisations list) and who inherits it — *nobody yet* when no session runs |
| 5 | a follow-up step | the question marked *drafted by the model*, its `because`, **no denominator**; *Not relevant — skip* dismisses it |
| 6 | `assist: "model_off"` or `"model_failed"` | one sentence naming why follow-ups are missing (`PANACEA_APP_MODEL_API_KEY` and `PANACEA_APP_MODEL_BASE_URL` for model_off: both must be set, task 02 amendment 4, review I13), and the flow carries on |
| 14 | `control-plane-down`, or `upstream_unavailable` | *Not saved.* The typed text stays where it was typed, and **Retry** re-sends the same op |
| — | `revision_conflict` | *Someone saved revision n first.* The view re-reads; the typed text stays; nothing is merged |
| — | 400 `bad_request`, `details.field: "text"` (the marker rule, plan §4 Security, S3; and the length caps, task 03 rule 13 and task 08's `RevisionTooLargeError`, S4, review M6; task 11) | the server's `message` verbatim, inline at the answer field of the question `details.key` names (the current step's field when it is that question); when `details.key` is `""` (a restore or a proposed follow-up, which name no question, b10 re-review M-2), at the head of the brief instead, above the current step; the typed text stays; **no Retry**, because re-sending the same text is refused again. Typically a pasted copy of a `CLAUDE.md` block, not an attack |
| — | 409 `fanout_too_large` (a project edit one Hub save cannot carry: too many branch briefs, too many bytes, or one branch brief too large; task 08, task 11, b10 re-review M-3) | the server's `message` verbatim at the head of the brief, which names the count or the size, and `details.branchRef` in mono when a branch is the cause; the typed text stays; **no Retry**, because the same edit is refused again until the branches change |

**The model follow-up is asked once, automatically**, after the third settled base answer, by the
op `propose_follow_up`; the server refuses a second (`already_asked`) and asks nothing before three
(`not_yet`), so the lens never counts.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `AppShell routes discover to the lens` | `lens: "discover"` renders `DiscoverLens`, not `LensPlaceholder` | the placeholder surviving |
| `the flow shows QUESTION n OF 5 and nothing else numeric` | step 2 → `QUESTION 2 OF 5`; no other `n of m` in the lens | a progress bar in words |
| `a follow-up has no denominator` | a `follow_up` step shows no `OF` | G5 |
| `1-4 pick and Enter sends, scoped to the region` | after render, with no manual focus and no dispatch on an element: `document.activeElement` is the question region, not a field; `userEvent.keyboard("2")` selects option 2; `userEvent.keyboard("{Enter}")` sends one `record` op with `source: "picked"`. After that save the next step renders and focus is on its region again. The same keys dispatched on `window` do nothing (re-review b07-1 R-M4) | a second window listener (G9); a field auto-focused on render, so `1`–`4` are text until a Tab |
| `Enter on a focused button acts once` | Tab to option 3's button, `userEvent.keyboard("{Enter}")` → exactly one `record` op, for option 3; Tab to ↵ Save with typed text, `{Enter}` → exactly one `record`; focus on an answer's *Edit*, `{Enter}` → the edit opens and no op is sent (re-review b07-1 R-M5) | a region `Enter` that also fires on a button, so one key sends two ops (the second a `revision_conflict`) or picks one option and sends another |
| `typing digits and E in the field is text` | `userEvent` (`@testing-library/user-event` 14.6.7, already in `desktop/package.json:28`) types `10 MB`, then `{Shift>}{Enter}{/Shift}`, then `Evidence` into the free-text field, whose events bubble to the region → the field's value is `10 MB\nEvidence`; no option is selected; the edit affordance does not open; no op is sent. Then `{Enter}` sends one `record` with `source: "typed"` and that text, trimmed. A keydown `Enter` with `isComposing: true` on the field sends nothing. In an empty field, and in one holding only spaces, `{Enter}` sends nothing and the value gains no new line (re-review b07-1 N4). **The same rows run on the follow-up's answer field** (state 5): digits and `E` are text, `Shift+Enter` is a new line, `Enter` sends its answer once (review I12, re-review b07-1 R-M3) | a region handler that does not check the target, so "10 MB" picks option 1 and "Evidence" opens the edit; a field with no new line, so a multi-line answer is sent at its first line; the follow-up field left on a different rule; an empty answer sent |
| `typing a free answer sends source typed` | `record` with the trimmed text | a typed answer labelled as picked |
| `branch skip is "Keep the project answer"` | branch scope shows that label and sends `skip` | a project-scope label on a branch |
| `a failed save keeps the typed text` | client rejects `network_unavailable` → text still in the field, *Not saved*, **Retry** re-sends the identical op | state 14's promise |
| `reserved text is refused at the field, without Retry` | client rejects 400 `bad_request` with `details: {field: "text", key: "purpose"}` on the `purpose` step → the server's message is shown at that answer field; the typed text is still in the field; no **Retry** control and no *Not saved*; no toast | the marker rule's 400 shown as state 14 (a Retry that is refused forever) or as a toast that discards the text (re-review M3) |
| `a too-long answer is refused at the field, without Retry` (review M6) | the same, with `details: {field: "text", key: "purpose", limit: 4000}` and the message "This answer is too long. Keep it to 4,000 characters." → shown at the `purpose` field, text kept, no **Retry** | a length refusal routed to a generic error that drops the 4,000 characters the person typed |
| `a too-large brief with no key is refused at the head` (b10 re-review M-2) | a restore rejected 400 `bad_request` with `details: {field: "text", key: "", bytes: 65600, limit: 65536}` → the server's message at the head of the brief; no answer field carries it; no **Retry** | an empty key matched to no field, so the refusal is never shown |
| `a fan-out the Hub cannot carry is refused at the head, without Retry` (b10 re-review M-3) | a project `record` rejected 409 `fanout_too_large` with `details: {needed: 1, limit: 15, bytes: 65700, limitBytes: 65536, branchRef: "refs/heads/feat/x"}` → the server's message at the head of the brief and `refs/heads/feat/x` in mono; the typed text still in the field; no **Retry** and no *Not saved* | `fanout_too_large` routed to state 14, a Retry refused for ever, or to a generic error that drops the text |
| `a Hub outage says the Hub, not the control plane` | `upstream_unavailable` → a sentence about the Hub | "the control plane is down" when it is not |
| `a conflict re-reads and keeps the text` | 409 `revision_conflict` → one `view()` call; text kept | a silent overwrite of a teammate |
| `the key flashes once, on mint only` | the save that makes the brief complete sets `justMinted`; re-rendering the same view does not re-flash | motion with no cause |
| `the follow-up is asked once after three answers` | the third settled answer is followed by one `propose_follow_up`; the fourth and fifth are not | a model call per answer |
| `model off carries on` | `assist: "model_off"` → the sentence names the variable; step 4 is still asked | a flow that stalls without a model |
| `unbound offers Share with the caller's teams` | picker lists teams with role ≥ member; the name field holds `binding.defaultName`; Share calls `bind(projectId, teamId, name)` with the field's text | a viewer offered an action the Hub will refuse |
| `a fork's upstream match is stated, not bound` (review M8) | `unbound` with one `matches` entry (`upstream`, `github.com/team/portal`, *Engagement Portal*) and `repoKeys[0]` `github.com/me/portal` → the match line names the remote, both keys and the project; **Share** still calls `bind(projectId, teamId, "portal (me/portal)")`, the `defaultName` given; no control calls anything with `pj-2`; with `matches: []` the line is absent | a match rendered as the project this one is bound to, or a control that binds the fork to the team's project |
| `a name the team holds is refused at the name field` (b10 re-review I-2) | `bind` rejects 409 `project_name_taken` with `details: {field: "name", name: "portal"}` → the server's message at the name field, which keeps `portal` and has focus; no **Retry**, no toast; editing the name to `portal-fork` and pressing **Share** calls `bind(projectId, teamId, "portal-fork")` | a name refusal shown as a generic error that clears the field, or a Retry that sends the refused name again |
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
