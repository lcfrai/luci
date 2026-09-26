### Task 136: assists, part 1: triage suggestions with `serves` and a "why"; the calibrated dial

W4, control plane and desktop; no seam. Depends on task 124 (`hubclient`'s W4 methods) and phase 6's
gateway pieces: P6 task 03 (`wire.MessagesURL`, `wire.RouteSource`), P6 task 13 (the project route
source), P6 task 16 (`fakeanthropic`), all **UNVERIFIED until merge**. Reads task 42's
`suggestCriteria` and `CriterionPicker` (`task-42.md:74-160`), task 46's pickers and `IssueHead`
(`task-46.md:554-613`), task 100's `AgentSwitch` (`task-100.md:48-53`, `:312-318`), task 14's title
search (`task-14.md:284`, `task-16.md:398`) and task 20's `expectNoPrediction`. Estimate 0.375 (3 h).

This is the first gateway-backed assist. Tasks 137 to 140 reuse its service, its routes file and its
test rig. **Tasks 138 and 139 were written first and assumed this part's names**
(`task-138.md:62-68`, `task-139.md:79-89`); every one of them is kept here, so neither renames.

Closes **L-51** and **L-52**; **L-53** once the calibration record lands (gap 2).

**What the reader sees** (05 §1 C L-51 to L-53, §2.4; AX §7, AX Principle 7):

- **In Triage and on the issue page, a suggestion sits beside its picker**, in the agent class:
  *"Suggested: bug · because EP-120 and EP-131, both labelled bug, share 'upload fails'"*, where
  EP-120 and EP-131 are links. A `serves` suggestion names the criterion and its brief: *"Suggested:
  serves C3 'Resume a dropped upload' of Upload brief · because the title says 'resume'"*, with C3
  a link to the criterion. There is no score, no percentage and no confidence word.
- **Accept** is one write, as the person. **Dismiss** hides it for this issue revision.
- **No suggestion ever moves an issue's state.** State is not a property an assist may name.
- **The dial** (team settings → *Assists*, and the `…` on any suggestion): per property, one of
  *Hide*, *Show* (PROPOSE AND WAIT), *Auto-apply* (ACT AND TELL ME). Auto-applied values are
  written as the person and listed under DECIDED WITHOUT YOU on the Desk with *Undo*.
- **Under each dial row, no number**, until the calibration record exists: *"No record yet of how
  you answered label suggestions."* When it exists: *"Label suggestions you answered: 23 unchanged,
  2 changed, none rejected."*
- **Failures are words.** *"Suggestions unavailable: assists are off for this org"*, *"Suggestions
  unavailable: the gateway did not answer"*. Task 42's text-match suggestion for `serves` stays,
  as it was in W1.

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 14, 16, 24, 42, 46,
100, 124, 125, 138 and 139. There is no part for tasks 39's DECIDED WITHOUT YOU filler (task 64),
83, 84 or 99; their names here are their scopes' and **UNVERIFIED**. The desktop's `ProblemCode`
union is `desktop/src/api/client.ts:40-48` (task 138 cites `:59`; the union ends at `:48` today).
`docs/AGENTIC-UX.md` §7 (`:175-194`) names the four levels and the calibration sentence.
`control-plane/internal/api/` has no `workassist.go`; the file map has no row for the assists'
control-plane side (gap 5).

**Decisions made here, so nobody re-derives them:**

1. **One service, one call shape.** `workassist.Service.Complete(ctx, projectID, Call) (Reply,
   error)` is the only function in the product that sends text to a model outside a session. It
   checks the kill switch, then reads the project's effective route, then calls the gateway on that
   route. Every later assist (137-140) goes through it and adds no second path.
2. **The route is the project's, read fresh.** `Routes wire.RouteSource` (P6 task 13) gives
   `ProjectInference.Effective`; an unknown route (`wire.ErrRouteUnknown`) refuses with
   `ErrGatewayDown` and makes no call. An AU-only project's assist goes to AU (OD-19), never Global
   as a fallback.
3. **The kill switch fails closed.** The org switch is task 100's `AgentSwitch.Orchestrators`
   (`task-100.md:314`). While it is `false`, or while it cannot be read, `Complete` returns
   `ErrAssistsOff` **before** any request. There is no separate assists switch in SW.19 (gap 3).
4. **No session, so no session key: the gateway client is an interface, and its real
   implementation is a proposal.** P6's `MintRequest.SessionClass` is `solo`, `worker` or
   `orchestrator` (`task-03.md:386-392`). An assist has no session and no pod. This part writes the
   proposal to the phase 6 owner (gap 1: an `assist` class, minted per person and project, bound to
   the control plane's address, drawn from the person's own budget slice) and **does not apply it**.
   What is built: `Gateway` (the interface) and `DirectGateway`, which posts to one upstream per
   route and is wired only when both upstreams are loopback addresses, i.e. P6 task 16's fakes in
   local mode. Every other deployment builds `Service` with `Gateway: nil`, and every assist answers
   `502 gateway_unavailable` until phase 6 decides. That is the honest state, and the desktop shows
   it in words.
5. **Closed vocabulary, checked twice.** The prompt lists, with short keys, the only values each
   property may take: the team's labels, the project's features, the team's members, the caller's
   readable teams, the four priorities, and the criteria of the project's readable briefs. A value
   outside its list is dropped on the control plane, whatever the model says.
6. **Every item needs a why that points at something it was given.** A why is `{kind, key}` with
   `kind` one of `issue`, `criterion`, `comment`, `guidance`, and `key` one of the keys the prompt
   carried. An item with no why, or whose why names a key not in the request, is **dropped before it
   reaches the desktop** (AX Principle 7). The desktop also refuses to render an item with no why
   (belt and braces: a future route could forget).
7. **What the prompt carries, and what it never does.** The issue's **current** revision's title and
   description (never an erased revision: an erased revision's texts read `""` and `ErasedAt` is
   set, `task-03.md:306-307`, so an erased issue is refused with `issue_not_found` before any
   call); up to 10 of the caller's readable issues from the Hub's title search on the issue's title
   (task 14), each as alias, title and its current labels, priority and feature, as examples the why
   may cite (`kind: issue`); the criteria. **No comment text** in part 1 (the `comment` why kind is
   accepted by the type for 137-139 and dropped here because no comment key is given). No erased
   text, no other team's issue (the Hub's search returns the caller's readable issues only).
8. **The control plane logs no issue text.** Handlers log the route, the issue id, the item count
   and the error class. A test reads the log buffer and finds neither the title nor the reply.
9. **Accepting is the person's write, through the desktop's own client.** The control plane only
   suggests; it never writes. Priority, assignee and labels are one `patchIssue` (task 37's
   `PatchIssueRequest`, `task-37.md:190-192`, which has exactly those three); `serves` is one
   `writeIssueRevision` with `baseRevision` the suggestion's `revision` and the head's content plus
   the criterion (task 09; a stale base is `409 revision_conflict`, `task-37.md:193`); feature and
   team go through their own routes (task 83's feature assignment and task 84's team move, both
   **UNVERIFIED** names, gap 6). The desktop's client carries the person's token, so the actor is
   the person. A suggestion for a revision older than the replica's head is not applied: it reads
   *"The issue changed since this suggestion. Ask again."*
10. **The dial's levels are AX §7's, three of the four.** *Hide*, *Show* = PROPOSE AND WAIT,
    *Auto-apply* = ACT AND TELL ME; FULL (apply and tell nobody) is not offered, because every
    auto-applied value is listed under DECIDED WITHOUT YOU with Undo. The dial is the person's own
    setting, per property, kept on this desktop (`localStorage`, key `panacea.assist.<property>`);
    the default is *Show*. It is not synced (gap 4). `state` is not a property, so it has no row.
11. **Calibration shows a count or nothing.** The words are *"N unchanged, N changed, N rejected"*,
    never *"N of M"* (SW.8's rule) and never a percentage. The count comes from the person's own
    answered suggestions, recorded on the Hub (gap 2); until that route exists the row says there is
    no record, and no number appears anywhere on the dial (AX anti-pattern 12).
12. **Auto-apply writes one property at a time, and never over a person.** An auto-applied value is
    written only when the property is still empty on the replica's head (no label, no assignee,
    priority `none`, no feature, no serves: a person has not set it) and the head's revision is
    still the suggestion's. `patchIssue` carries no base revision, so between that check and the
    write a person's change can race it; the Hub's last write wins and both writes are events with
    their actors. `serves` carries `baseRevision` and races as a `409`, shown in words, not retried.
