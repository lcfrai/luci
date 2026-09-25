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
- `desktop/src/components/intake/testWork.ts` — the tests' `setupWork` harness; tasks 171-173 reuse it (new)
- `desktop/src/components/intake/registerIntakeDesk.ts` and `UnroutedRow.tsx` — the `request.unrouted` Desk row (new)
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
import { suggestCriteria, type BriefCriterion, type CriterionSuggestion } from "../../work/suggestCriterion";   // task 42

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

#### The ops this task sends (SW.16 `mutate`; the union is task 169's `IntakeOp`)

```ts
// Accept (decision 2): ONE op. The body is only what the person typed, plus the chosen serve.
mutate({ kind: "request.accept", requestId: r.id, body: {
  title: draft.title.trim(),
  description: { text: draft.description, author: "H" },   // "" is legal; never r.quote.text
  projectId,
  serves: draft.serves ? [{ briefKey: draft.serves.briefKey, criterionId: draft.serves.criterionId }] : [],
} });
mutate({ kind: "request.link", requestId: r.id, issueId });                           // Link
mutate({ kind: "request.important", requestId: r.id, important: !r.important, prev: r.important });
mutate({ kind: "request.decline", requestId: r.id });                                 // Decline
```

`mutate` takes one op, so accept is one `Idempotency-Key` and one Undo entry (task 56's `keyed`).
The replica's optimistic copy of an accept moves the request to `accepted` and adds a placeholder
issue head in the team's default state; a refusal (`request_closed`, `policy_refused`) removes
**both** in one `transact` and `onRefused` shows the reason under the accept form: *Someone else
accepted this request* for `request_closed`, task 42's policy words for `policy_refused`. The
title field refuses an empty title before any send (*A title is needed*), so the Hub's 422 is not
the only check.

#### Interfaces — complete: `desktop/src/components/intake/LinkPicker.tsx`

```tsx
// desktop/src/components/intake/LinkPicker.tsx — task 170 (decision 3), on task 46's Picker.
import type { JSX } from "react";
import { Picker, type PickerItem } from "../issue/pickers/Picker";   // task 46
import { useWork } from "../../work/replica/useWork";                 // SW.16, task 55

export function LinkPicker(props: { teamId: string; onChoose(issueId: string): void; onClose(): void }): JSX.Element {
  const { teamId, onChoose, onClose } = props;
  const heads = useWork((w) => w.openIssuesOfTeam(teamId));   // alias order, open states only
  const items: PickerItem[] = heads.map((h) => ({ id: h.id, label: `${h.alias} ${h.title}` }));
  return <Picker label="Link to which issue" items={items} onClose={onClose} onChoose={(id) => onChoose(id)} />;
}
```

`openIssuesOfTeam` is a replica read over task 55's issue map (every non-`done`, non-`cancelled`,
non-`duplicate` head of the team's projects, by alias); if task 55's part names it otherwise, this
file follows. The item id is the `is-` id, never the alias (task 46's `PickerItem.id` rule).

#### Interfaces — complete: the delta on ⚑ `TriageView.tsx` (task 42)

```tsx
// TriageView.tsx — task 170 adds request rows above the issue rows, under one sub-heading.
import { RequestRow } from "../intake/RequestRow";
import { RequestPanel } from "../intake/RequestPanel";

// inside TriageView, after task 42's issue list is computed:
const teamId = useWork((w) => w.teamOfProject(props.projectId));
const hasRequests = useWork((w) => w.teamHasRequests(teamId));            // decision 8
const open = useWork((w) => (hasRequests ? w.openRequestsOfTeam(teamId) : []));   // oldest first
// Rendered only when hasRequests: <h3 className="ik-sub">Requests</h3> and one RequestRow each,
// then task 42's <h3>Issues</h3> and its rows. j/k walk both lists as one (task 41's listKeys).
// Enter on a request row opens RequestPanel beside the list, as task 42's CriterionPicker opens.
// The number keys 1-4 and T are issue keys: on a request row they do nothing (decision 7).
```

Requests sort **oldest first** (the longest wait on top), a measured order, not a rank. While
`hasRequests` is `false` the `Requests` heading, its rows and the list's *requests* filter chip are
absent from the DOM, not hidden with CSS.

#### Interfaces — complete: `IntakeRulesEditor.tsx` and `IntakeFormEditor.tsx`, the exported halves

```tsx
// desktop/src/components/intake/IntakeRulesEditor.tsx — task 170 over task 147's rules.
import type { JSX } from "react";
import type { IntakeRule, RuleMatch } from "../../api/work";

/** One rule's match, in words: "from the form Bug report, sender @contoso.com, text has 'upload'". */
export function matchWords(m: RuleMatch, formName: (id: string) => string): string {
  const parts: string[] = [];
  if (m.channel) parts.push(`by ${m.channel}`);
  if (m.formId) parts.push(`from the form ${formName(m.formId)}`);
  if (m.senderDomain) parts.push(`sender @${m.senderDomain}`);
  if (m.textContains) parts.push(`text has "${m.textContains}"`);
  return parts.length === 0 ? "every request" : parts.join(", ");
}

/** Reorder as a new position list; the Hub writes it with the person as author (task 147). */
export function moveRule(rules: readonly IntakeRule[], from: number, to: number): IntakeRule[] {
  const next = rules.slice();
  const [r] = next.splice(from, 1);
  if (r !== undefined) next.splice(Math.max(0, Math.min(to, next.length)), 0, r);
  return next.map((x, i) => ({ ...x, position: i + 1 }));
}

export function IntakeRulesEditor(props: { teamId: string; canEdit: boolean }): JSX.Element;
```

```tsx
// desktop/src/components/intake/IntakeFormEditor.tsx — task 170 over task 148's forms.
import type { JSX } from "react";
import type { FormField, IntakeForm } from "../../api/work";

/** A field is valid when it has a label and, for select, at least two distinct options. */
export function fieldProblem(f: FormField): string | null {
  if (f.label.trim() === "") return "A field needs a label";
  if (f.kind === "select" && new Set(f.options.map((o) => o.trim()).filter(Boolean)).size < 2)
    return "A choice field needs at least two options";
  return null;
}

export function IntakeFormEditor(props: {
  teamId: string; formId: string; canEdit: boolean;
  form: IntakeForm & { url: string };   // task 169's form(formId) answer
}): JSX.Element;
```

**The rules editor**, top to bottom: *First match wins* once, in sans, above the list; one row per
rule in `position` order with `matchWords`, the route (*to Payments · labels intake, mobile*),
and *by Dana Wu · 24 Sep 10:14* (author in sans, time in mono). A team admin reorders with the
row's handle or `Alt+↑`/`Alt+↓`, and each reorder sends **one** `rules.put` op with the whole list
and the `version` read (a stale version comes back `conflict` and the list reloads, saying *Dana
changed the rules; showing hers*). A member sees the same rows with no handle and no button, and
*Only a team admin edits intake rules* once above the list. `canEdit` is the token's team role
(hub-contract §5.2), never an optimistic guess.

**The form editor**: one row per `FormField` with its label, a `kind` picker over task 169's
`FORM_FIELD_KINDS` (words: *short text, long text, choice, date, number, person*), a *Required*
switch, and the options list when `kind` is `select`. Two default blocks follow, *For members* and
*For people outside the team*, each with labels, priority and assignee through task 46's
`LabelPicker`, `PriorityPicker` and `AssigneePicker`. The form's `url` (the Hub-hosted https
page, task 148) shows in mono with *Copy*; the editor never renders the public page itself. *Save*
sends one `form.put`; `fieldProblem` blocks it in place, naming the field.

#### Interfaces — complete: `desktop/src/components/intake/registerIntakeDesk.ts`

```ts
// desktop/src/components/intake/registerIntakeDesk.ts — task 170. SW.17's one new row kind.
import { registerDeskRow, registerDeskSource } from "../desk/registry";   // task 62, UNVERIFIED path
import { UnroutedRow } from "./UnroutedRow";

export interface UnroutedPayload { requestId: string; teamId: string; requester: string; reason: string }

export function registerIntakeDesk(): void {
  registerDeskRow<UnroutedPayload>("request.unrouted", "failed", UnroutedRow,
    (item) => ({ tier: 1, waitingSinceMs: Date.parse(item.arrivedAt) }));   // blocks a requester
  registerDeskSource("request.unrouted", (snap) => snap.signals("request.unrouted").map((e) => ({
    kind: "request.unrouted", id: e.subject, arrivedAt: e.at,
    payload: { requestId: e.subject, teamId: String(e.data?.teamId ?? ""),
               requester: String(e.data?.requesterName ?? ""), reason: String(e.data?.reason ?? "") },
    rankReason: "a request no rule could route", ifNothing: "It waits, unseen by any team's Triage",
  })));
}
```

`UnroutedRow` is a six-line row in the same file set (*No rule routed Priya Nair's request ·
waiting 3h 12m*, and *Open* to the team's Triage), counted in `RequestRow.test.tsx`. The row's
data is metadata only: `requesterName` and `reason` are task 144's event `data` fields
(**UNVERIFIED**), never the quote.

#### Tests to write first — `RequestPanel.test.tsx`

```tsx
// Fixtures: task 20's workFixtures (FIXED_NOW, tm-pay, pj-ep) plus one request built here.
const quoteXss = `<img src=x onerror="window.__pwned=1"> **bold** 50% of uploads fail`;
const rq = (over: Partial<Request> = {}): Request => ({
  id: "rq-00000000000000a1", teamId: "tm-pay", projectId: "pj-ep",
  requester: { kind: "person", userId: null, email: null, displayName: "Priya Nair" },
  quote: { text: "Uploads keep failing on the train", author: "H" }, channel: "form", channelRef: "fm-1",
  important: false, answers: [], state: "open", issueId: null, suggestedCriterion: null, ruleId: "ir-1",
  createdAt: "2026-09-25T04:02:00Z", ...over });

describe("RequestPanel", () => {
  // Kills: HTML rendered from the quote (dangerouslySetInnerHTML or a Markdown renderer).
  it("renders a quote holding markup as text", () => {
    const { container } = render(<RequestPanel request={rq({ quote: { text: quoteXss, author: "H" } })} {...base} />);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("strong")).toBeNull();
    expect(container.querySelector(".ik-quote")!.textContent).toBe(quoteXss);
    expect((window as { __pwned?: number }).__pwned).toBeUndefined();
  });

  // Kills: two ops (issue, then link), and a half-done accept after a refusal.
  it("accept sends one op with one Undo; a refused accept leaves neither issue nor link", async () => {
    const { user, mutate, replica } = setupWork({ refuse: { "request.accept": "request_closed" } });
    render(<RequestPanel request={rq()} {...base} />);
    await user.click(screen.getByRole("button", { name: "Accept" }));
    await user.type(screen.getByLabelText("Title"), "Resume uploads after a dropped connection{Enter}");
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0][0].kind).toBe("request.accept");
    await waitFor(() => expect(screen.getByText("Someone else accepted this request")).toBeVisible());
    expect(replica.issuesOfProject("pj-ep").some((h) => h.title.startsWith("Resume uploads"))).toBe(false);
    expect(replica.intake().requests.get(rq().id)!.state).toBe("open");
  });

  // Kills: the quote copied into the new issue's description.
  it("the accept body's description is empty until the person types", async () => {
    const { user, mutate } = setupWork();
    render(<RequestPanel request={rq()} {...base} />);
    await user.click(screen.getByRole("button", { name: "Accept" }));
    await user.type(screen.getByLabelText("Title"), "Resume uploads{Enter}");
    const body = mutate.mock.calls[0][0].body as AcceptRequestBody;
    expect(body.description).toEqual({ text: "", author: "H" });
    expect(JSON.stringify(body)).not.toContain("failing on the train");
  });

  // Kills: important with no undo.
  it("important toggles and U restores it", async () => {
    const { user, undo, replica } = setupWork();
    render(<RequestPanel request={rq()} {...base} />);
    await user.click(screen.getByRole("button", { name: "Important" }));
    expect(replica.intake().requests.get(rq().id)!.important).toBe(true);
    await undo();
    expect(replica.intake().requests.get(rq().id)!.important).toBe(false);
  });

  // Kills: a similarity percentage or score on the suggestion.
  it("the suggestion shows matched words and no prediction", () => {
    const { container } = render(<RequestPanel request={rq()} {...base}
      criteria={[{ briefKey: "brief:bf-1@3", criterionId: "c2", text: "Uploads survive a dropped connection" }]} />);
    expect(screen.getByText(/matches: uploads/)).toBeVisible();
    expect(container.textContent).not.toMatch(/\d+\s?%|score|confidence/i);
    expectNoPrediction(container);   // the quote's own "50%" would be exempt: data-verbatim
  });
});
```

`setupWork` is written here, in `desktop/src/components/intake/testWork.ts` (no part on the tree
exports a shared harness; grep `setupWork|renderWith` finds only per-file helpers): about 25
lines that build task 56's `makeWorkClient` over a fake `send`, a replica, and return `user`
(`@testing-library/user-event`), `mutate`, `undo` and `replica`, with `refuse` mapping an op kind
to a Hub code the fake answers 409 or 422 with. `base` holds `thread: []`, `projectId: "pj-ep"`,
`criteria: []` and `onClose`.

#### Tests to write first — `TriageView` (appended to task 42's test), the editors, the Desk row

1. **No request noun before one exists** — task 42's Triage fixture with `teamHasRequests` false:
   `queryByRole("heading", { name: "Requests" })` is null and no element has `data-kind="request"`
   or the text *requests* in the filter bar. Add one request to the replica: the heading and one
   row appear. **Kills** "the noun shown before it exists" (05:861).
2. **Oldest first, by the one clock** — two requests created 3h and 2d apart render the 2d one
   first; with `now` advanced 60 s by the heartbeat fixture the age text changes and no
   `setInterval` was registered by the row (a spy on `window.setInterval` counts zero). **Kills**
   "newest first" and "a per-row timer".
3. **The row holds no quote text** — the row's `textContent` contains no word of the quote.
   **Kills** "a truncated quote shown as the ask".
4. **Number keys do nothing on a request row** — focus a request row, press `1`: `mutate` is not
   called. **Kills** "`1` accepts a request with no title".
5. **Link writes one op** — *Link*, choose `EP-7`: one `request.link` with `issueId` equal to the
   `is-` id, never the alias. **Kills** "the alias sent as the key".
6. **Requesters by name on the issue** — after an accept and a link to `EP-7`, `IssueView` for
   `EP-7` reads *Requested by Priya Nair, Tom Ruiz* and contains no digit next to *request*.
   **Kills** "a request count badge" (L-135 SKIP).
7. **A member reads rules** — `IntakeRulesEditor` with `canEdit: false`: no button, no
   `draggable`, no input, and the line *Only a team admin edits intake rules*. **Kills** "an
   editable list the Hub will refuse".
8. **Reorder is one write, with the version** — as admin, `Alt+↓` on rule 1: one `rules.put` whose
   `body.rules` positions are `1, 2` in the new order and whose `version` is the one read. A
   `conflict` answer reloads and shows *changed the rules; showing* theirs. **Kills** "a reorder
   per row" and "last writer wins silently".
9. **The form refuses a bad field in place** — a `select` with one option: *Save* is blocked and
   the field shows *A choice field needs at least two options*; zero sends. **Kills** "the Hub's
   422 as the only check".
10. **`request.unrouted` appears in FAILED** — a signal event of that kind puts one row under the
    FAILED header of task 62's Desk (on the next entry, per its hold rule), naming the requester.
    **Kills** "an unrouted request vanishes".
11. **No new key bound** — `KEYMAP` (task 53) has the same keys before and after
    `registerIntakeDesk()` and the intake modules load. **Kills** "a request key added without 05".

#### Steps

1. Write the tests above and `testWork.ts`; run them; they fail (no intake components).
2. Write `RequestRow.tsx`, `RequestPanel.tsx`, `LinkPicker.tsx` and `intake.css` (`.ik-row`,
   `.ik-quote`, `.ik-sub`; elevation by DESIGN-SYSTEM's ring plus shadow, radius from the scale,
   `transform`/`opacity` only). The panel tests pass.
3. Apply the `TriageView.tsx` and `IssueView.tsx` deltas; tests 1-6 pass.
4. Write both editors; tests 7-9 pass. Write `registerIntakeDesk.ts` and `UnroutedRow.tsx`, call it
   where task 62 registers its own kinds; tests 10-11 pass.
5. `npm --prefix desktop run typecheck`; commit with `git commit -F <msgfile> -- <paths>`.
