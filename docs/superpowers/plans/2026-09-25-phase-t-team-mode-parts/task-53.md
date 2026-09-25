### Task 53: The team strip and click-in

**Slice:** TM1. **Size:** 3 focused hours. **dependsOn:** 51 (**approved**: the review record
`docs/verification/2026-09-25-phase-t-prototype-review.md` says so), 52 (ST.8, `teamSlice`,
`TeamClient`). **Seam:** consumes ST.8.

This is the one element team mode adds to a project's view (`05-design.md` §9, *"add one element
to the project view and nothing to the rail"*): a band at the top of a team project's view, one row
per person with a live agent, and a click-in that expands a row in place. It is built to the
approved prototype, state keys `2`–`5` (task 51), and it renders ST.8 exactly as task 52 serves it:
it computes no status, no label and no line of its own.

What the band shows, row by row (§9.1): one dot on the global clock; the name; the **assembled**
line, `<brief section title> · <task title> · N agents`, where a `text·A` title shows its author
class on hover; up to three observed paths in Commit Mono, then `+N`, with a 1px ring on a path
someone else also holds; and on the right one link, *"ask mine ↗"*. `●` working, `○` waiting;
*"waiting on <name>"* only when `waitingOn.kind === "human"`, which task 52 sets only from
`blocked --on human` or the hook's `Notification` (`task-09.md:105-107`); `lease` reads *"waiting
on a lease"* and `peer` *"waiting on a thread"*, Fleet's own facts; otherwise `idle`. Zero state:
*"Only you on this project."* A solo project renders nothing.

What click-in shows (§9.2): the person's tasks, each with its brief-section link (the provenance
thread); their agents as STATE / MEANING / NEXT cards (`docs/AGENTIC-UX.md:148`), from
`PersonDetail.agents[].card`; files and leases, with an open conflict against one of your agents
marked; and **one action**, *"Ask my Orchestrator about this"*, which opens **your own**
Orchestrator panel (task 55) with the subject attached. There is no control that steers a
teammate's agent, because none may exist (OD-16), and no view of their terminal, transcript,
prompts or Orchestrator conversation (§9.2, `05-design.md:982-983`).

#### Decisions this part makes

**(a) Motion stays inside the desktop's audited list.** The design-system audit allows transitions
on `opacity`, `background-color` and `color` at `var(--dur-fast)` and nothing else
(`desktop/src/styles/designsystem.test.ts:180-190`, *"A declaration that needs a fifth entry is a
design-system decision"*). So this task adds **no transition**: the change flash follows
the pattern of `common.css`'s `.dot-flash::after` (`components/common/common.css:22-26`, `:96`)
with its own `@keyframes team-row-flash`, opacity only, on a row-sized `::after`, restarted by
keying the pseudo-element's host on `changedAt`; click-in uses the View Transitions
API (`document.startViewTransition`, capability-checked) with a `view-transition-name` per row, as
`docs/DESIGN-SYSTEM.md:70` names, and swaps instantly where it is absent. Neither is a CSS
transition, so the list does not move.

**(b) One clock, reused.** Every live dot takes `beatDelayStyle()` from `hooks/heartbeat.ts`
(`:23-26`), the shared epoch that makes dots mounted at different moments breathe together. No row
owns a timer. The only timer this task adds is the strip's 5 s poll (task 52 decision a), one per
mounted band, and exactly one band is ever mounted.

**(c) OD-28's mono face is one line in the token layer.** `tokens.css:151` is `--mono: "Cascadia
Mono", …`. This task puts `"Commit Mono", "CommitMono"` first, unless phase W's sync-merge has
already (then this step is skipped and says so). xterm reads the same stack (`tokens.css:148-149`),
and Commit Mono is a real monospace, so the terminal is unaffected; where the font is not
installed the stack falls back exactly as today.

**(d) The brief link is a store value, not a URL.** The Discover lens is still a placeholder on
this branch (`components/AppShell.tsx:30-38`, `LensPlaceholder`), so the provenance link calls
`setLens("discover")` and a new `focusBriefSection(ref)` on the UI slice. Phase 4's Discover lens
reading `briefFocus` is **UNVERIFIED until the phase 4 lens lands here**; until then the link opens
the lens and the value waits in the store. It is still the thread, visible and clickable.

**Files:**
- `desktop/src/components/team/TeamStrip.tsx` — the band, the zero state, `+N more` (new)
- `desktop/src/components/team/TeamRow.tsx` — one person's row (new)
- `desktop/src/components/team/TeamDot.tsx` — the row's dot on the shared clock (new)
- `desktop/src/components/team/PersonDetail.tsx`, `AgentCard.tsx` — click-in (new)
- `desktop/src/components/team/AuthoredText.tsx` — every `TeamText` on screen, as plain text with
  its label; tasks 54 and 55 import it (new)
- `desktop/src/components/team/useTeamStrip.ts` — the one poll (new)
- `desktop/src/components/team/team.css` — imports the token layer first, like every stylesheet
  the audit reads (new)
- `desktop/src/components/team/{TeamStrip,PersonDetail,AuthoredText}.test.tsx` (new)
- `desktop/src/components/AppShell.tsx` — mounts `<TeamStrip />` between `<TopBar />` and
  `.shell-lens` (modify, one line, anchored on `<TopBar />` at `:92`)
- `desktop/src/store/slices/uiSlice.ts` — `briefFocus`, `focusBriefSection` (modify)
- `desktop/src/styles/tokens.css` — the `--mono` first entry (modify, one line; decision c)

**Interfaces:**
- Consumes: ST.8's `TeamStrip`, `PersonRow`, `PersonDetail`, `TeamText`, `TextA` (task 52's
  `types.ts` team block); `teamSlice`'s `teamStrips`, `teamPeople`, `setTeamStrip`,
  `setTeamPerson`, `openOrchestrator`; `createTeamClient().strip`/`.person`; `beatDelayStyle` and
  `BEAT_MS` (`hooks/heartbeat.ts:13`, `:23`); `selection.projectId` (`store/types.ts:196-201`).
- Produces: `AuthoredText` for 54 and 55; `focusBriefSection` for phase 4's lens.

#### Component surface (complete)

```tsx
// components/team/AuthoredText.tsx
/** Renders one TeamText as PLAIN TEXT: React text children only. No markdown, no link
 *  detection, no dangerouslySetInnerHTML, no terminal rendering (§7.3). The label is always
 *  on screen beside the body for text·A ("Alice's Orchestrator (agent)"), and in `title` for
 *  a compact placement. `data-author` carries the class for tests and for styling. */
export function AuthoredText(props: {
  text: TeamText;
  /** "inline": body, then the label in --ink-label. "compact": body only, label in title. */
  placement?: "inline" | "compact";
}): JSX.Element;

// components/team/TeamStrip.tsx
/** Mounted by AppShell for the selected project. Renders nothing for a solo project or before
 *  the first answer; the one-line zero state when only the viewer has a live agent. Shows the
 *  first MAX_ROWS people in ST.8's order, then "+N more", which expands the band to all. */
export const MAX_ROWS = 5;
export function TeamStrip(props: { projectId: ProjectId }): JSX.Element | null;

// components/team/TeamRow.tsx
export function TeamRow(props: {
  projectId: ProjectId;
  row: PersonRow;
  expanded: boolean;
  onToggle(): void;
}): JSX.Element;

// components/team/TeamDot.tsx
/** ● working (disc, breathing on the shared clock), ○ waiting (ring, still), idle (disc,
 *  still). Reuses common.css's .dot, .dot-disc, .dot-ring, .dot-live classes; the phase
 *  comes from beatDelayStyle(), so no dot owns a timer. */
export function TeamDot(props: { status: RowStatus; label: string }): JSX.Element;

// components/team/PersonDetail.tsx
export function PersonDetail(props: { projectId: ProjectId; userId: string }): JSX.Element;

// components/team/AgentCard.tsx
/** Three lines, always three: STATE / MEANING / NEXT (docs/AGENTIC-UX.md:148). */
export function AgentCard(props: { card: AgentCard; name: string; harness: string }): JSX.Element;

// components/team/useTeamStrip.ts
/** Fetches the strip now, then every STRIP_POLL_MS while the document is visible, and
 *  stores each answer with setTeamStrip. One interval; paused while hidden, as poller.ts is. */
export const STRIP_POLL_MS = 5_000;
export function useTeamStrip(projectId: ProjectId | null, client?: TeamClient): void;
```

The row's text, in order, with the element that carries each (task 51's state `3`):

| Part | Rendered | Element |
|---|---|---|
| dot | `TeamDot` | `span.dot` with `aria-label` "Alice: working" |
| name | `displayName`, `(you)` appended for the viewer | sans, `--ink` |
| line | `briefSection.title`, then ` · `, `AuthoredText compact` of `taskTitle` when present, then ` · N agents` (`1 agent`) | sans, `--ink-2`; the count `tabular-nums` |
| waiting | `waiting on Priya` / `waiting on a lease` / `waiting on a thread`; for `human` with `words`, the words as `AuthoredText compact` in the `title` | sans, `--need` shape and tone from the token layer |
| files | each of `recent` in Commit Mono; `heldByOthers` members get class `team-path-held` (`box-shadow: var(--e-hair)`); then `+N` when `more > 0` | `code.team-path` |
| action | `ask mine ↗`, a button that calls `openOrchestrator(projectId)` with no subject | right-aligned, one per band, in the band's footer, not per row |

UI slice addition (complete):

```ts
// uiSlice.ts: in UiSlice
  /** A brief section id the Discover lens should scroll to, set by a provenance link. */
  briefFocus: string | null;
  focusBriefSection(ref: string): void; // sets lens "discover" and briefFocus in one set()
```
