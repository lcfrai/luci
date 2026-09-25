### Task 170: requests in Triage: the quote, *important*, accept or link, the criterion suggestion; the form and rules editors

W5, stream WD, about 3 hours (an estimate, not a measurement). Depends on task 42 (Triage,
`CriterionPicker`, `suggestCriteria`, `threadWords`: `task-42.md:74-209`) and task 169 (the W5
desktop wire: `Request`, `IntakeRule`, `IntakeForm`, `FormField`, the `WorkIntakeClient` members
and the `IntakeOp` union, `task-169.md`). It also uses task 46's `Picker` (`task-46.md:158-175`),
task 44's `ConfirmInPlace` (`task-44.md:130-150`), task 20's `expectNoPrediction`
(`task-20.md:126-131`), SW.16 (`useWork`, `mutate`, `onRefused`) and SW.17 (task 62's
`registerDeskRow` and `registerDeskSource`, scope `tasks-61-80.md:73-82`).

Parity: closes, jointly with the Hub tasks named, **L-132** and **L-133** (146), **L-49** (147),
**L-27** and **L-139** (148); records **L-135**'s alternative (*an issue lists its requesters by
name*, 05:287). Design: 05 §3.5 (`05-design.md:783-796`), §4.1 (`:849`, the row at `:861`,
*"sub-issue, milestone, request · shown only once a team creates one"*), §4.2 (`:930`, FAILED's
row at `:938`, *"an intake rule could not route a request"*); 05 L-27, L-49, L-132, L-133, L-139.

**What the reader does.** A business owner filed *"Uploads keep failing on the train"* through the
team's form yesterday. In Triage the reader sees it beside the issue rows: *Priya Nair · form*,
and *waiting 1d 3h* in mono. `Enter` opens the panel: the quote, verbatim, labelled as Priya's
words; under it *Suggested: Uploads survive a dropped connection · matches: uploads, failing*. The
reader presses *Accept*, types a title and a description of their own, and presses `Enter`: one
new issue in the team's default state, linked, and the issue now lists *Requested by Priya Nair*.

**Decisions made here, so nobody re-derives them:**

1. **The quote is text, never markup.** `RequestPanel` renders `quote.text` as a React text child
   inside `<blockquote data-verbatim>`, never through the Markdown `Editor`, `dangerouslySetInnerHTML`
   or a Markdown renderer. `data-verbatim` exempts the requester's own words from
   `expectNoPrediction` (task 20's rule), because a requester may write *"50% of uploads fail"* and
   that is their quote, not our projection. The label reads *Priya Nair's words* in sans with task
   46's `<AuthorClass author="H" />` (`task-46.md:437-466`, *text·H · person*), never *"Summary"*.
2. **Accept is one op, and the quote is not copied.** *Accept* opens an inline form in the panel
   (title, required, 1-200 characters; description, the one `Editor` of task 49, empty) and sends
   **one** `request.accept` op (task 169's `IntakeOp`) whose body carries only what the person
   typed. The Hub creates the issue and the link in one transaction (task 146). A refusal rolls
   both back in the replica (task 56); nothing half-done is drawn. The quote stays on the request,
   so erasing the request (L-137) erases the words, and the issue never holds them (task 169 G2).
3. **Link is one op, through a picker of the team's issues.** *Link* opens task 46's `Picker` over
   the replica's open issues of the request's team, by alias and title; choosing one sends
   `request.link`. Task 42 has no issue picker of its own (the scope's *"task 42's issue picker"*
   is not in task 42's part), so `LinkPicker` is written here, on task 46's `Picker` (gap 1).
4. **Requesters show on the issue by name, never as a count.** `IssueView`'s header (task 46)
   gains one line, *Requested by Priya Nair, Tom Ruiz* in sans, from task 169's `requestsOf`. No
   *"2 requests"* badge, no sort by count (L-135 is SKIP).
5. ***Important* is one toggle, one op, one Undo.** `request.important` with `prev`; `U` inverts
   it (task 169's inverse). It renders as the word *important* on the row in `--ink`, never a
   colour-only mark.
6. **The suggestion is task 42's, over the quote.** `suggestCriteria({title: "", description:
   quote.text}, criteria)` with the team's project briefs; the first suggestion shows as one line
   with its `matched` words. `Enter` on it attaches the criterion to the **new issue** at accept
   (the accept form's *Serves* field is pre-filled and removable); it is never a score. When W4's
   gateway suggestions are on (tasks 136-137), their line shows under it with its own why.
7. **No new key.** 05 §4.3 (`:960-1040`) names no key for requests: checked, none mentions a
   request or intake. `Enter` opens and closes the panel through task 42's list keys; the panel's
   buttons are reached by `Tab`.
8. **The request column appears only once it exists.** `TriageView` asks the replica
   `teamHasRequests(teamId)` (task 169); while it is `false` no request row, filter chip or
   column header renders, and the Triage view is exactly task 42's (05:861).
9. **Rules and forms are the team admin's.** Team role comes from the token (hub-contract §5.2);
   a member sees both editors read-only, with *Only a team admin edits intake rules* once above the
   list, and no drag handle or input. The Hub's 403 is never the only guard (task 147's matrix row).

**Files:**
- `desktop/src/components/intake/RequestRow.tsx` (+ `RequestRow.test.tsx`) — the Triage row (new)
- `desktop/src/components/intake/RequestPanel.tsx` (+ `RequestPanel.test.tsx`) — quote, thread, accept, link, important (new)
- `desktop/src/components/intake/LinkPicker.tsx` — the issue picker on task 46's `Picker` (new)
- `desktop/src/components/intake/IntakeRulesEditor.tsx` (+ `IntakeRulesEditor.test.tsx`) — ordered rules (new)
- `desktop/src/components/intake/IntakeFormEditor.tsx` (+ `IntakeFormEditor.test.tsx`) — typed fields (new)
- `desktop/src/components/intake/intake.css` — `.ik-*` (new)
- `desktop/src/components/intake/registerIntakeDesk.ts` — the `request.unrouted` Desk row (new)
- `desktop/src/components/work/TriageView.tsx` ⚑ (task 42) — request rows beside issue rows (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46) — the *Requested by* line (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 20, 42, 44, 46
(which owns `IssueView.tsx`, as `task-106.md:55` and `task-131.md:86` cite it), 49 and 169. There
is **no part** for tasks 62, 146, 147, 148 or 157, so the Desk registry and every request, rule and
form rule are their scopes' names and **UNVERIFIED**. The
prototype `prototype/work-layer.html` draws no intake surface (grep `request|intake` finds only a
negotiation line at `:747`), so the look follows the Triage rows it does draw and DESIGN-SYSTEM.md.

#### Interfaces — complete: `desktop/src/components/intake/RequestRow.tsx`

```tsx
// desktop/src/components/intake/RequestRow.tsx — task 170. One Triage row for one open request.
import type { JSX } from "react";
import type { Request, RequestChannel } from "../../api/work";   // task 169
import "./intake.css";

export const CHANNEL_WORD: Readonly<Record<RequestChannel, string>> = {
  form: "form", email: "email", teams: "Teams", tracker: "tracker",
};

/** A measured age, as a fact: "waiting 2d 4h", "waiting 3h 12m", "waiting 40m". Never "~2 days". */
export function waitingWords(createdAt: string, nowMs: number): string {
  const m = Math.max(0, Math.floor((nowMs - Date.parse(createdAt)) / 60_000));
  const d = Math.floor(m / 1440), h = Math.floor((m % 1440) / 60), mm = m % 60;
  if (d > 0) return `waiting ${d}d ${h}h`;
  if (h > 0) return `waiting ${h}h ${mm}m`;
  return `waiting ${mm}m`;
}

export function RequestRow(props: {
  request: Request;
  now(): number;          // the one heartbeat's clock (hooks/heartbeat.ts); FIXED_NOW in tests
  selected: boolean;
  onOpen(): void;
}): JSX.Element {
  const { request: r, now, selected, onOpen } = props;
  return (
    <li className="ik-row" role="option" aria-selected={selected} data-kind="request" data-id={r.id}
        onClick={onOpen}>
      <span className="ik-row-who">{r.requester.displayName}</span>
      <span className="ik-row-where">{CHANNEL_WORD[r.channel]}</span>
      {r.important ? <span className="ik-row-important">important</span> : null}
      <span className="ik-row-age">{waitingWords(r.createdAt, now())}</span>
    </li>
  );
}
```

The requester and the channel are sans (`--t-body`), the age is mono (`--t-fact`, `tabular-nums`),
and the row holds **no** text of the quote: the list is not a place to read customer content, and
a row that truncated the quote would show a fragment as if it were the ask. The age re-renders on
the one heartbeat, never its own interval (DESIGN-SYSTEM, one global clock).

#### Interfaces — complete: `desktop/src/components/intake/RequestPanel.tsx`, the exported half

```tsx
// desktop/src/components/intake/RequestPanel.tsx — task 170. Opened from a RequestRow by Enter.
import type { JSX } from "react";
import type { Request, RequestMessage } from "../../api/work";
import type { BriefCriterion, CriterionSuggestion } from "../../work/suggestCriterion";   // task 42

/** The one suggestion line (decision 6): the first of task 42's suggestCriteria over the quote. */
export function requestSuggestion(r: Pick<Request, "quote">, criteria: readonly BriefCriterion[]): CriterionSuggestion | null {
  return suggestCriteria({ title: "", description: r.quote.text }, criteria, 1)[0] ?? null;
}

/** The accept form's state. description starts empty: the quote is never copied (decision 2). */
export interface AcceptDraft { title: string; description: string; serves: BriefCriterion | null }
export const emptyAcceptDraft = (s: CriterionSuggestion | null): AcceptDraft =>
  ({ title: "", description: "", serves: s ? { briefKey: s.briefKey, criterionId: s.criterionId, text: s.text } : null });

export function RequestPanel(props: {
  request: Request;
  thread: readonly RequestMessage[] | null;   // null until loaded on open (task 169, load step 1)
  criteria: readonly BriefCriterion[];        // the project's briefs, in brief order
  projectId: string;                          // the rule's route.projectId, or the team's first
  onClose(): void;
}): JSX.Element;
```

The panel's body, top to bottom: the requester line (*Priya Nair · form · 25 Sep 14:02*, name in
sans, time in mono); the quote in `<blockquote className="ik-quote" data-verbatim>` as a text child
with `<AuthorClass author="H" />` and *Priya Nair's words* above it; the thread, when `channel` is
`teams`, as a list of `authorName` and `body` (also text children, `data-verbatim`); the
suggestion line; the form answers as a two-column list (`label` in sans, `value` as text); then the
four actions in one row: *Accept*, *Link*, *Important* (a toggle, `aria-pressed`), *Decline*.
*Decline* is in place: it asks *Decline this request? Priya is not told.* through `ConfirmInPlace`
and sends `request.decline`. The panel uses `useWork`, `mutate` and `onRefused` (SW.16).
