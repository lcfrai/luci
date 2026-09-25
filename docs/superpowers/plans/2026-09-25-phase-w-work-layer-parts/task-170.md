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
   that is their quote, not our projection. The label reads *Priya Nair's words* in sans with a
   small `H` provenance mark (task 20's `text·H` glyph), never *"Summary"*.
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
4. **Requesters show on the issue by name, never as a count.** `IssueView`'s header (task 47)
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
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 47) — the *Requested by* line (modified)

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 20, 42, 44, 46, 49
and 169. There is **no part** for tasks 47, 62, 146, 147, 148 or 157, so `IssueView`, the Desk
registry, and every request, rule and form rule are their scopes' names and **UNVERIFIED**. The
prototype `prototype/work-layer.html` draws no intake surface (grep `request|intake` finds only a
negotiation line at `:747`), so the look follows the Triage rows it does draw and DESIGN-SYSTEM.md.
