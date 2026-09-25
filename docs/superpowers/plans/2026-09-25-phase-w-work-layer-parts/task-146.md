### Task 146: requests `rq-`: the requester, the verbatim quote as `text·H`, *important*, accept or link in `triage`, erase

W5, stream WH; no seam of its own (it consumes **SW.21**, task 143's W5 wire, and **SW.3**, task
145's guards). Depends on task 14 (the erase pattern this follows), task 145 (the W5 migration that
holds `work_requests`) and Hub task H27 (the audit route and brief erase). Estimate 0.375 (3 h).
Closes, jointly with task 170 (the Triage surface): **L-132** and **L-133**; serves **L-137**,
**L-138** and **L-140** (tasks 149 and 157 use this task's store function). Every `server/` and
`store/` path is **UNVERIFIED until merge** (rule H1); `server/erase.go` is Hub task 27's file and
this task adds routes beside task 14's, never a change to them.

A **request** is how someone outside the team asks for something (05 §3.5): a business owner, a
client, a person in another team. It keeps **who asked** and **their words, verbatim**, and it
waits in the team's Triage until a person accepts it into a new issue, links it to an issue that
already exists, or declines it. After this task a member can record a request on someone's behalf,
the team can list and read its requests, flag one *important*, accept, link or decline it, an issue
lists who asked for it by name, and an org admin can erase a request's text. The form (task 148),
mail (task 149) and Teams (task 157) paths all create through the one store function this task
writes, so a request is the same record whichever door it came in by.

**Task 143 and task 145 are not written at this writing** (gap 1 in *Gaps*). This part therefore
states, complete, the wire types it needs from `hubapi/work_intake.go` (SW.21) and the table it
needs from `NNNN_work_w5.sql`. They are **requirements on those two parts**: when they land, their
names win and this part follows them, as rule H1 has a WH task follow a moved Hub file.

Eight decisions the scope left open, made here.

1. **One request, one outcome.** A request is `open` until exactly one of *accept*, *link* or
   *decline* closes it, and then it is `accepted`, `linked` or `declined` for good. A second close
   of any kind is 409 `request_closed`, naming the state it is in. A requester who asks for two
   things sends two requests; a person who wants one ask to feed two issues accepts it into one and
   relates the second (task 10's relations). This is what makes the test *"a second accept is
   `request_closed`"* a rule and not a race: the close is one `UPDATE … WHERE state = 'open'`
   inside the org's write lock, and zero rows is the 409.
2. **The quote becomes the issue's description, and the request stays out of the agent's
   context.** The scope pins accept to *"revision 1 whose description is the quote, `author: "H"`"*,
   and 05 §3.5 says *"its text never enters an agent's managed block"*. Task 19 renders an issue's
   `H` description into the managed block (`task-19.md:10-26`), so these two meet. They are
   reconciled like this: by accepting, a **person** adopts the words as the issue's description,
   and the accept dialog says so (task 170: *"The description will be these words. Agents that
   start from this issue read them."*); what never enters is the **request record**: its id,
   requester, email, channel, answers and rule. The test that kills *"request text reaches the
   agent"* checks exactly that. Whether the owner wants the stricter reading, an empty
   description with the quote reachable only as data through `panacea issue show`, is **owner
   question Q-146** in *Gaps*; the change would be one line in `acceptRequest` and one test.
3. **Link writes no revision and moves nothing.** It sets the request's `issue_id` and appends
   `request.linked`. The issue gains a requester in `GET /v1/issues/{issueId}/requests` and nothing
   else: no revision, no state, no `issue.updated`. A person who wants the requester's words in the
   issue pastes them into a revision, which is then their act.
4. **Accept is task 08's create, not a copy of it.** It builds the revision through
   `hubapi.IssueRevisionDigest`, checks `serves` with task 08's `checkServes`, takes the state from
   the team's policy (`defaultState`, task 11) and the thread state from task 08's
   `threadStateOf`, exactly as `importOne` does (`task-15.md:420-486`). With no serve the issue is
   `not_tied` with reason **`support`** (task 03's closed list, `task-03.md:171-177`): an ask from
   outside the team is the case `support` exists for, and the accepter can re-tie it later.
5. **The project must be the request's team's.** A `projectId` outside the request's team is 400
   `field: projectId`, whether or not the project exists and whoever can see it, so the answer is
   no oracle for another team's projects. Rules (task 147) route only inside the team by the same
   check.
6. **Requests are team-scoped events.** Every `request.*` kind is `ScopeTeam` and
   `VisibleToReaders` in task 13's registry, with `projectId` null: an open request belongs to the
   team's Triage, not to a project. `request.accepted` and `request.linked` carry the `issueId`,
   and the issue's own `issue.created` (accept only) is task 13's, unchanged.
7. **Erase is not a state.** It blanks the text and sets `erased_at`; an erased request keeps its
   state and `issue_id`, so a triage count and an issue's requester list still add up, reading
   *erased* where the name was. Accept, link and decline of an erased open request are 409
   `request_erased`: an issue made from nothing is not an accept. `important` may still be set.
8. **`channel_ref` survives erase.** The `Message-ID` (task 149) or Teams message id (task 157) is
   an identifier, not a person's words, and keeping it is what stops a mail retry from re-creating
   a request an admin erased. It is capped at 998 bytes (the RFC 5322 line limit) and never shown
   in the feed.
