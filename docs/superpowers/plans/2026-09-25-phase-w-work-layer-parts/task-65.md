### Task 65: the Desk: §3.7's informs and the reassign *decide*; answer in place; the zero state

Stream **WD2**, the last row kinds in seam **SW.17** (the Desk frame), about 3 hours (an estimate,
not a measurement). Depends on task 63 (the NEEDS YOU rows, and the frame's snapshot moved onto the
replica) and task 64 (FAILED, LANDED, RUNNING), and through their seams on task 19
(`Session.issueRef`, SW.7), task 56 (`mutate`, `onRefused`, `failedOf`) and task 62 (the frame).
**Tasks 62, 63 and 64 have no part file on the tree** at this writing: the SW.17 names used below
(`registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskRank`, `DeskSnapshot`) are the scope's
words for task 62 (`tasks-61-80.md:67-99`) and are **UNVERIFIED** until task 62's part lands; task
62 wins every conflict.

After this task the Desk says what changed under my agents since they read their brief, asks me
once when someone wants to take an issue my agent is working on, answers every verb where the row
stands, and says *"Nothing needs you."* only when that is true, with FAILED still visible below it.

**Decisions made here, so nobody re-derives them:**

1. **An inform is derived, never stored.** §3.7 rule 1 (05-design) says nothing reaches a running
   agent until its owner presses a verb, so the inform needs no Hub record: it is a pure selector
   over the replica and the loader's refs, recomputed on each Desk entry (the frame never re-ranks
   while open, task 62). A re-derive after a restart gives the same rows.
2. **The desktop never splits a key.** `Session.issueRef` is opaque (`task-19.md:300-303`: *"never
   split, never compared except for equality"*). The adopted revision comes from the loader's
   resolved `ProvenanceRef` for that key (`kind: "issue"`, `issueId`, `revision`, `serves`,
   task 18's fields, `task-18.md:110-114`), and the head from the replica's `IssueHead`
   (`headRevision`, `headKey`). A ref still resolving makes no inform (it says nothing it cannot
   back).
3. **Whose change it is.** Rule 1 fires when the head is newer than the adopted revision **and the
   head revision was written by someone else**. A head I wrote is not news to me: my write took the
   previous head as its `baseRevision` (`WriteIssueRevisionRequest`, `task-37.md:194`), so I read
   every revision under it. Scope test 1 pins that; a mutant that informs on any gap fails it.
4. **What changed, in one line.** The adopted ref's `serves` against the head revision's `serves`,
   as `serves c-40aa added` or `serves c-11ef removed` (criterion ids in mono); a title change is
   `title changed`; anything else is `text changed`. It never quotes the text (the ref's text may be
   erased under it, and an inform is not a diff view; *Enter* opens the issue view at the head).
5. **Re-brief is the only path that moves `issueRef`.** The verb calls the control plane once per
   session of mine on that issue at an older revision. Task 19's Materialiser rewrites the managed
   block on create and restart only (`task-19.md:137-139`); a live **re-brief** route is not in any
   part. This task prints its wire (below) and **reports it as a gap** for task 19's owner. Until
   the route exists the call answers 404 and the row takes the normal refusal path (`--fail` and the
   server's sentence), never a pretend success. Whether a running CLI re-reads the block before its
   next turn is **UNVERIFIED**: we never type into it (`OD-8`).
6. **The reassign decide is a request, not a patch.** A reassign of an issue whose state is
   `started` or `in_review` (a lane is bound, 05 §3.7 rule 2) goes as `RequestReassign`, and the
   assignee does not move until the current assignee answers. A reassign of any other state is the
   plain `setAssignee` op (task 56). The routing is one pure function, `reassignWrite(head, to)`,
   which task 46's assignee picker calls; the fake refuses a request on an unbound issue, so the two
   paths cannot be confused from either side.
7. **While unanswered, nothing moves.** The pending request pauses the *reassign*, never the agent:
   the session keeps running, the worktree stays, the assignee stays, and every issue surface shows
   *reassign requested by Alice*. *Keep working, decline* (`2`) clears the request and changes
   nothing else. *Hand over* (`1`) moves the assignee through the Hub and then stops my sessions on
   that issue with phase 2's `killSession`; the worktree and its commits stay (its commits are the
   checkpoint, as task 64 says of *Restart*). **No checkpoint-commit route exists** in the control
   plane (searched `control-plane/internal` for `checkpoint` on 2026-09-26: no match), so uncommitted
   changes are left in the worktree, not committed for me; the hand-over line says so.
8. **Answer in place.** Every verb on a Desk row runs through `mutate` or a named client call, and
   the row shows its answer where it stands: `answered · handed over to Alice`, `answered · kept`,
   `re-briefed at @4`. A refusal flashes `--fail` with the Hub's code and sentence (task 56's
   `failedOf`). No verb changes the lens, the route or the open issue; only `Enter` opens the issue
   view, beside the Desk (task 63). A resolved row stays in its place, dimmed, until the next entry.
9. **The zero state counts unresolved rows.** When every NEEDS YOU row in the snapshot is resolved
   (or there were none), `DeskZero` renders at the top of NEEDS YOU. Resolved rows stay under it,
   so nothing jumps, and FAILED keeps its header and rows (07 D C5). The sentence is built only from
   facts W1 has: the newest LANDED row of this snapshot, and the live sessions on my issues.
10. **Hand-over moves the issue, not the worktree, in W1.** 05 §3.7 rule 2 says the worktree and its
    branch move to the new owner. Worktrees are local clones on my machine (`OD-2`), and W1 runs in
    local mode (`OD-20`): nothing can carry my clone to Alice's machine, and pushing the lane's
    branch is phase 3's act, not this task's. So W1's hand-over moves the assignee and stops my
    agents; Alice starts her own from the issue. The branch move is listed under *does not claim*.

**Files:**
- `desktop/src/work/desk/informs.ts` (+ `informs.test.ts`) — the pure selectors for the three
  informs, `changeLine`, `reassignWrite`, `zeroSentence` (new)
- `desktop/src/components/desk/rows/InformRow.tsx` (+ `InformRow.test.tsx`) — one row for the three
  inform kinds, its verb, its answered line (new)
- `desktop/src/components/desk/rows/ReassignDecide.tsx` (+ `ReassignDecide.test.tsx`) — the tier-0
  decide with keys `1` and `2` (new)
- `desktop/src/components/desk/DeskZero.tsx` (+ `DeskZero.test.tsx`) — task 62's empty file, filled
  (modified)
- `desktop/src/components/desk/deskRows.tsx` ⚑ — four `registerDeskRow` and four
  `registerDeskSource` calls, after tasks 63 and 64's (modified)
- `desktop/src/components/desk/desk.css` ⚑ — `.desk-row[data-answered]`, `.desk-inform` (modified)
- `desktop/src/components/work/ReassignMark.tsx` — the *reassign requested by Alice* fact line that
  `IssueRow` (task 40) and `IssueHead` (task 46) mount (new; **not in the skeleton's file map**)
- `desktop/src/components/work/IssueRow.tsx` ⚑, `desktop/src/components/issue/IssueHead.tsx` ⚑ — one
  mount each (modified)
- `desktop/src/api/work.ts` ⚑ (task 37) — `IssueHead.reassignRequest`, two `WORK_RELAY` rows, two
  `WorkClient` methods; `desktop/src/api/client.ts` ⚑ — `rebriefSession` (modified)
- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03), `hubapi/workevents.go` ⚑ (task 13),
  `hubclient/work.go` ⚑ (task 04), `hubclient/fake_work.go` ⚑ (tasks 04-05) — the added type, field,
  two event kinds and two methods under WS0's *added method or field* rule (modified)

**Interfaces:**
- Produces: `informsOf`, `reassignWrite`, `zeroSentence`; the four row kinds `issue.inform_changed`,
  `issue.inform_related`, `issue.inform_drift`, `issue.reassign_decide`; `WorkClient.requestReassign`
  and `answerReassign`; `hubclient.Work.RequestReassign` and `AnswerReassign` on the fake.
  Consumed by task 46's assignee picker (`reassignWrite`), task 66 (the rows' roles), task 70 (the
  demo's last frame reads `zeroSentence`), and the WH task that builds the real route (a gap).
- Consumes: task 62's SW.17 (`registerDeskRow`, `registerDeskSource`, `DeskSnapshot`); task 56's
  `mutate`, `onRefused`, `failedOf`; task 55's `useWork`; task 19's `selectSessionIssueRefs` and
  `Session.issueRef`; task 18's `ProvenanceRef.kind` and issue fields; P4-16's `selectRef`
  (`phase-4 task-16.md:180`, **UNVERIFIED until merge**); task 42's `CriterionPicker`; phase 2's
  `killSession` (`desktop/src/api/client.ts:155`, built); task 20's `expectNoPrediction`.

#### Interfaces — complete: the Hub delta (added type, field, kinds and methods; WS0's rule)

**The Hub half is a gap.** SW.1 has no pending-reassign record (`task-03.md`'s `IssueHead` has no
such field). This task adds the wire and the fake only, and the real route is **UNVERIFIED** until a
WH task (store, migration column, route, matrix rows) owns it. That task is not in the skeleton's
table; it is reported in the return.

```go
// control-plane/internal/hub/hubapi/work.go ⚑ — appended (task 65; WS0: an added type and field)

// ReassignRequest is a pending reassign of an issue a lane is bound to (05 §3.7
// rule 2). The assignee does not move until the current assignee answers.
type ReassignRequest struct {
	To User   `json:"to"`
	By User   `json:"by"`
	At string `json:"at"` // RFC 3339, the Hub's clock
}

// In type IssueHead, after Assignee — nil when no request is pending:
//	ReassignRequest *ReassignRequest `json:"reassignRequest"`

// RequestReassignRequest is the body of POST /v1/issues/{issueId}/reassign.
type RequestReassignRequest struct {
	ToUserID string `json:"toUserId"` // u-…; never the current assignee
}

// AnswerReassignRequest is the body of POST /v1/issues/{issueId}/reassign/answer.
// Only the current assignee may answer; accept moves the assignee to the
// request's To and clears it, decline only clears it.
type AnswerReassignRequest struct {
	Accept bool `json:"accept"`
}
```

```go
// control-plane/internal/hub/hubapi/workevents.go ⚑ — two added lines (task 13's rule: "a new
// kind is an added line here and an added struct")
	EventIssueReassignRequested = "issue.reassign_requested" // data: ReassignRequestedData
	EventIssueReassignAnswered  = "issue.reassign_answered"  // data: ReassignAnsweredData

type ReassignRequestedData struct {
	ToUserID string `json:"toUserId"`
}
type ReassignAnsweredData struct {
	Accept bool `json:"accept"`
}
```

```go
// control-plane/internal/hub/hubclient/work.go ⚑ — appended to WorkIssues (task 65; WS0)

	// RequestReassign records a pending reassign of an issue a lane is bound to
	// (state started or in_review). The assignee does not move. 422 on an
	// unbound issue, on a request to the current assignee, or by the assignee.
	RequestReassign(ctx context.Context, issueID string, req hubapi.RequestReassignRequest) (hubapi.IssueHead, error)
	// AnswerReassign is the current assignee's answer. Accept moves the
	// assignee to the request's To; decline changes nothing but the request.
	AnswerReassign(ctx context.Context, issueID string, req hubapi.AnswerReassignRequest) (hubapi.IssueHead, error)
```

```go
// control-plane/internal/hub/hubclient/routes_work.go ⚑ (task 16's WorkRoutes) — two rows, last
	{"RequestReassign", http.MethodPost, "/v1/issues/{issueId}/reassign", false},
	{"AnswerReassign", http.MethodPost, "/v1/issues/{issueId}/reassign/answer", false},

// control-plane/internal/api/work.go ⚑ (task 16's workCalls) — two entries
		"RequestReassign": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.RequestReassignRequest) (any, error) {
			return wk.RequestReassign(r.Context(), r.PathValue("issueId"), req)
		}),
		"AnswerReassign": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.AnswerReassignRequest) (any, error) {
			return wk.AnswerReassign(r.Context(), r.PathValue("issueId"), req)
		}),
```

Two more files join the list for those rows: `control-plane/internal/hub/hubclient/methods_work.go` ⚑
(task 16: two HTTP methods, each `c.workCall(ctx, "RequestReassign", …)` in that file's pattern)
and `control-plane/internal/api/work.go` ⚑ (the two `workCalls` entries above). Task 16's
`registerWork` panics when a row has no handler or a handler no row, so both land in one commit.
The file name `routes_work.go` is **UNVERIFIED**: task 16's part declares `WorkRoutes` in its
`hubclient` block (`task-16.md:124-127`) without a file heading in that fence; the row goes
wherever that `var` lives.

#### Interfaces — complete: the fake's rules (`hubclient/fake_work.go` ⚑)

```go
// fakeIssue gains one field (task 04's struct, task-04.md:287-300):
//	reassign *fakeReassign // nil when no request is pending

type fakeReassign struct{ to, by, at string } // user ids and the fake's stamp

const (
	fkReassignNotBound = "reassign_not_bound" // 422: state is not started or in_review
	fkReassignSelf     = "reassign_self"      // 422: to is the assignee, or the caller is
	fkReassignPending  = "reassign_pending"   // 409: another To is already pending
	fkNotAssignee      = "not_assignee"       // 403: only the assignee answers
	fkNoReassign       = "no_reassign_pending" // 409: nothing to answer
)

// renderIssue gains, after the assignee (task-04.md:344):
//	if is.reassign != nil {
//		h.ReassignRequest = &hubapi.ReassignRequest{To: w.userRef(is.reassign.to),
//			By: w.userRef(is.reassign.by), At: is.reassign.at}
//	}

func (f *Fake) RequestReassign(_ context.Context, issueID string, req hubapi.RequestReassignRequest) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RequestReassign", issueID, req)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.IssueHead{}, err
	}
	if err := w.erasedWrite(is); err != nil {
		return hubapi.IssueHead{}, err
	}
	switch {
	case is.head.State != hubapi.StateStarted && is.head.State != hubapi.StateInReview:
		return hubapi.IssueHead{}, problem(422, fkReassignNotBound, "No lane is bound to this issue. Assign it directly.", "state", string(is.head.State))
	case is.assignee == "" || is.assignee == me || req.ToUserID == is.assignee:
		return hubapi.IssueHead{}, problem(422, fkReassignSelf, "There is nobody else to ask. Assign it directly.", "toUserId", req.ToUserID)
	case !w.assignable(is.teamID, req.ToUserID):
		return hubapi.IssueHead{}, badRequest("toUserId", "An assignee is a member of this team.")
	case is.reassign != nil && is.reassign.to == req.ToUserID:
		return w.renderIssue(is), nil // a repeat: the same head, no event
	case is.reassign != nil:
		return hubapi.IssueHead{}, problem(409, fkReassignPending, "Another reassign is waiting for an answer.", "toUserId", is.reassign.to)
	}
	is.reassign = &fakeReassign{to: req.ToUserID, by: me, at: w.stamp()}
	is.head.UpdatedAt = is.reassign.at
	w.workEvent(me, hubapi.EventIssueReassignRequested, is.teamID, is.head.ProjectID, is.head.ID,
		map[string]any{"toUserId": req.ToUserID})
	return w.renderIssue(is), nil
}
