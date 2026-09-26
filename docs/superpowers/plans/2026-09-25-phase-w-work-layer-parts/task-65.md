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
```

```go
func (f *Fake) AnswerReassign(_ context.Context, issueID string, req hubapi.AnswerReassignRequest) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("AnswerReassign", issueID, req)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	is, _, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	switch {
	case is.reassign == nil:
		return hubapi.IssueHead{}, problem(409, fkNoReassign, "No reassign is waiting for an answer.", "issueId", issueID)
	case is.assignee != me:
		return hubapi.IssueHead{}, problem(403, fkNotAssignee, "Only the assignee answers a reassign.", "issueId", issueID)
	}
	to := is.reassign.to
	is.reassign = nil
	is.head.UpdatedAt = w.stamp()
	w.workEvent(me, hubapi.EventIssueReassignAnswered, is.teamID, is.head.ProjectID, is.head.ID,
		map[string]any{"accept": req.Accept})
	if req.Accept {
		is.assignee = to // the one assignee write; issue.updated as PatchIssue emits it
		w.workEvent(me, fkEventIssueUpdated, is.teamID, is.head.ProjectID, is.head.ID,
			map[string]any{"fields": "assigneeId"})
		w.subscribeOn(is, to, hubapi.SubscribeAssign)
	}
	return w.renderIssue(is), nil
}
```

Three more rules hold across the other fake methods, each one line in the method it names:
`PatchIssue` with an `assigneeId` change clears a pending request (a direct assign by an admin
settles it; the request is then moot), `MoveIssueState` out of `started`/`in_review` into any state
but the other of the two clears it, and `EraseIssue` clears it. Each clear emits
`issue.reassign_answered` with `{"accept": false, "cleared": "<method>"}` so a replica never keeps
a request the Hub dropped.

#### Interfaces — complete: the desktop wire (`desktop/src/api/work.ts` ⚑, `client.ts` ⚑)

```ts
// desktop/src/api/work.ts ⚑ (task 37) — added, never reordered.

/** 05 §3.7 rule 2. Pending while the current assignee has not answered. */
export interface ReassignRequest { to: User; by: User; at: string }
// interface IssueHead gains, after `assignee`:
//   reassignRequest: ReassignRequest | null;
export interface RequestReassignRequest { toUserId: string }
export interface AnswerReassignRequest { accept: boolean }

// WORK_RELAY gains two rows, last, in task 16's order (the relay test pins the pairing):
//   ["RequestReassign", "POST", "/v1/issues/{issueId}/reassign"],
//   ["AnswerReassign", "POST", "/v1/issues/{issueId}/reassign/answer"],

// interface WorkClient gains:
//   requestReassign(issueId: string, toUserId: string): Promise<IssueHead>;
//   answerReassign(issueId: string, accept: boolean): Promise<IssueHead>;
// makeWorkClient (task 56, `task-56.md:101`) gains the two bodies, keyed like every write:
//   requestReassign: (id, toUserId) => send("POST", path("RequestReassign", { issueId: id }), { toUserId }, opts) as Promise<IssueHead>,
//   answerReassign: (id, accept) => send("POST", path("AnswerReassign", { issueId: id }), { accept }, opts) as Promise<IssueHead>,

// WIRE_KEYS (task 37) gains "reassignRequest" for IssueHead, and "toUserId", "accept" for the
// two bodies; task 37's wire test compares them with Go's json tags.
```

```ts
// desktop/src/api/client.ts ⚑ (phase 2's CockpitClient, `client.ts:153-155` sit beside it)

  /** 05 §3.7 rule 1. Moves one session's issueRef to the issue's key at a newer revision; the
   *  Materialiser rewrites the managed block. The route is NOT BUILT (decision 5, a reported
   *  gap): until task 19's owner adds `POST /v1/sessions/{id}/rebrief`, this rejects with the
   *  server's 404, and the row shows it as a refusal. */
  rebriefSession(id: SessionId, issueKey: string): Promise<Session>;

// createClient (`client.ts:236`):
    rebriefSession: async (id, issueKey) =>
      parseJson<Session>(await send("POST", `${sessionPath(id)}/rebrief`, { issueKey })),
```

The wire the missing route answers is fixed here so its owner has one thing to build: request
`{"issueKey": "issue:is-…@4"}`; `200` with the `Session` (its `issueRef` the new key); `409
{"code": "issue_ref_not_newer"}` when the key's revision is not above the session's; `422
{"code": "issue_ref_other_issue"}` when the key names another issue; `404` for no such session.
The route checks equality of the issue part only through the Hub's own resolve (task 19's loader
rule), never by splitting the string on the desktop.

#### Interfaces — complete: `desktop/src/work/desk/informs.ts`, part 1: the inputs and the changed inform

`DeskSnapshot` is task 62's; its members are not fixed by any part on the tree. This task reads
five things from it and names them `InformInputs`, so task 62's snapshot either carries them under
these names or its source adapts in one line (`deskRows.tsx`, below). `refOf` is P4-16's loader
read (`selectRef`), **UNVERIFIED until merge**; `revision` on `ProvenanceRef` is P4-15's field
(`task-15.md:161-168`).

```ts
// desktop/src/work/desk/informs.ts — pure: no React, no store, no clock of its own.
import type { IssueHead, IssueRevision, RelationKind, Serve, User, WorkEvent } from "../../api/work";
import type { ProvenanceRef } from "../../api/discover";
import type { Session } from "../../store/types";
import type { WorkOp } from "../replica/ops";

export type InformKind = "issue.inform_changed" | "issue.inform_related" | "issue.inform_drift";

export interface InformInputs {
  me: string;                                            // my u- id
  sessions: readonly Session[];                          // live and ended; filtered here
  heads: ReadonlyMap<string, IssueHead>;                 // the replica's heads by is- id
  revisionAt(issueId: string, rev: number): IssueRevision | null; // null: not in the replica
  refOf(key: string): ProvenanceRef | "resolving" | null;          // P4-16's loader
  events: readonly WorkEvent[];                          // the feed since my last Desk visit
  briefCriteria(briefKey: string): readonly string[] | "resolving" | null; // the brief's CURRENT ids
}

export interface ChangedPayload {
  issueId: string; alias: string; adoptedRev: number; headRev: number; headKey: string;
  by: User; at: string; line: string; sessionIds: string[];
}
export interface RelatedPayload {
  issueId: string; alias: string; otherId: string; otherAlias: string; relation: RelationKind;
  by: User; at: string;
}
export interface DriftPayload {
  issueId: string; alias: string; criterionId: string; briefKey: string; sessionIds: string[];
}

const LIVE = new Set(["starting", "running", "idle"]); // phase 2's Session.status words

/** My live sessions bound to an issue, by issue id, through the loader only (decision 2). */
export function mySessionsByIssue(i: InformInputs): Map<string, { s: Session; ref: ProvenanceRef }[]> {
  const out = new Map<string, { s: Session; ref: ProvenanceRef }[]>();
  for (const s of i.sessions) {
    if (!s.issueRef || !LIVE.has(s.status)) continue;    // W1: every local session is mine
    const ref = i.refOf(s.issueRef);
    if (!ref || ref === "resolving" || ref.kind !== "issue") continue; // says nothing it cannot back
    const list = out.get(ref.issueId) ?? [];
    list.push({ s, ref });
    out.set(ref.issueId, list);
  }
  return out;
}

/** Decision 4: one line, never the text. Criterion ids render in mono by the row. */
export function changeLine(adopted: { serves: Serve[]; title?: string }, head: IssueRevision): string {
  const had = new Set(adopted.serves.map((s) => s.criterionId));
  const has = new Set(head.content.serves.map((s) => s.criterionId));
  const added = [...has].filter((c) => !had.has(c)).sort();
  const removed = [...had].filter((c) => !has.has(c)).sort();
  if (added.length || removed.length) {
    const parts = [...added.map((c) => `${c} added`), ...removed.map((c) => `${c} removed`)];
    return `serves ${parts.join(", ")}`;
  }
  if (adopted.title !== undefined && adopted.title !== head.content.title.text) return "title changed";
  return "text changed";
}
```

**Whose session.** Phase 2's `Session` has no owner field (`desktop/src/store/types.ts`, grep for
`createdBy` on 2026-09-26: no match; `SessionStatus` is `"starting" | "running" | "idle" |
"exited"`, `types.ts:134`). In W1 local mode every session on this machine is mine (`OD-20`), so
the selector filters by status and ref only. Team mode's managed sessions carry an owner (its
plan's rows); when they plug into the Desk, the owner check joins here, and until then a teammate's
managed session is not in `sessions` at all. `AuthoredText.text` is task 37's field
(`task-37.md:107`).

#### Interfaces — complete: `informs.ts`, part 2: the three selectors

```ts
import type { DeskItem } from "../../components/desk/deskModel"; // task 62; path UNVERIFIED

type Item<T> = DeskItem<T>;

/** 05 §3.7 rule 1. One item per issue, not per session; its id is stable across re-derives. */
export function changedInforms(i: InformInputs): Item<ChangedPayload>[] {
  const out: Item<ChangedPayload>[] = [];
  for (const [issueId, bound] of mySessionsByIssue(i)) {
    const head = i.heads.get(issueId);
    if (!head) continue;
    const behind = bound.filter((b) => b.ref.revision < head.headRevision);
    if (behind.length === 0) continue;
    const top = i.revisionAt(issueId, head.headRevision);
    if (!top || top.writtenBy.id === i.me) continue;      // decision 3: my write is not news
    const oldest = behind.reduce((a, b) => (b.ref.revision < a.ref.revision ? b : a));
    const adopted = i.revisionAt(issueId, oldest.ref.revision);
    const line = changeLine({ serves: oldest.ref.serves, title: adopted?.content.title.text }, top);
    out.push({
      kind: "issue.inform_changed", id: `inform_changed:${issueId}@${head.headRevision}`,
      arrivedAt: top.writtenAt, rankReason: "changed since your agent read it",
      ifNothing: `your agents keep working from @${oldest.ref.revision}`,
      payload: { issueId, alias: head.alias, adoptedRev: oldest.ref.revision, headRev: head.headRevision,
        headKey: head.headKey, by: top.writtenBy, at: top.writtenAt, line,
        sessionIds: behind.map((b) => b.s.id).sort() },
    });
  }
  return out;
}

/** 05 §3.7 rule 3, solo form: the relation is already recorded; this only tells me. */
export function relatedInforms(i: InformInputs): Item<RelatedPayload>[] {
  const mine = mySessionsByIssue(i);
  const out: Item<RelatedPayload>[] = [];
  for (const e of i.events) {
    if (e.kind !== "issue.related" || !e.actor || e.actor.id === i.me) continue;
    const other = String(e.data.otherIssueId ?? "");       // task 05's relationEvent keys
    const [issueId, otherId] = mine.has(e.subject) ? [e.subject, other] : mine.has(other) ? [other, e.subject] : ["", ""];
    const head = i.heads.get(issueId);
    if (!head) continue;
    out.push({
      kind: "issue.inform_related", id: `inform_related:${e.seq}`, arrivedAt: e.at,
      rankReason: "a relation was added to your in-flight issue", ifNothing: "the relation stays; nothing moves",
      payload: { issueId, alias: head.alias, otherId, otherAlias: i.heads.get(otherId)?.alias ?? "",
        relation: String(e.data.kind) as RelationKind, by: e.actor, at: e.at },
    });
  }
  return out;
}

/** 05 §3.3: a served criterion the brief's current revision no longer has. */
export function driftInforms(i: InformInputs): Item<DriftPayload>[] {
  const out: Item<DriftPayload>[] = [];
  for (const [issueId, bound] of mySessionsByIssue(i)) {
    const head = i.heads.get(issueId);
    const top = head && i.revisionAt(issueId, head.headRevision);
    if (!head || !top) continue;
    for (const s of top.content.serves) {
      const now = i.briefCriteria(s.briefKey);
      if (now === null || now === "resolving" || now.includes(s.criterionId)) continue;
      out.push({
        kind: "issue.inform_drift", id: `inform_drift:${issueId}:${s.criterionId}`, arrivedAt: top.writtenAt,
        rankReason: "a criterion it serves left the brief", ifNothing: `${head.alias} keeps serving ${s.criterionId}`,
        payload: { issueId, alias: head.alias, criterionId: s.criterionId, briefKey: s.briefKey,
          sessionIds: bound.map((b) => b.s.id).sort() },
      });
    }
  }
  return out;
}

export const informsOf = (i: InformInputs) =>
  ({ changed: changedInforms(i), related: relatedInforms(i), drift: driftInforms(i) });
```

A relation event names the issue it was added on as `subject` and the other end in
`data.otherIssueId` (`task-05.md:473-480`), so the selector matches either end; a relation I added
myself makes no inform. `drift`'s `arrivedAt` is the head revision's time because the brief's
revision time is not on the loader's read (P4-16); the row prints no time for drift for that reason.

#### Interfaces — complete: `informs.ts`, part 3: the reassign routing and decide, the zero sentence

```ts
const BOUND = new Set<string>(["started", "in_review"]);   // a lane is bound (05 §3.7 rule 2)

/** Decision 6. Task 46's assignee picker calls this and nothing else to choose the write. */
export type ReassignWrite =
  | { via: "op"; op: WorkOp }
  | { via: "request"; issueId: string; toUserId: string };

export function reassignWrite(head: IssueHead, to: User | null, me: string): ReassignWrite {
  const current = head.assignee;
  const someoneElses = current !== null && current.id !== me;
  if (BOUND.has(head.state) && someoneElses && to !== null && to.id !== current!.id) {
    return { via: "request", issueId: head.id, toUserId: to.id };
  }
  return { via: "op", op: { kind: "setAssignee", issueId: head.id, assignee: to } };
}

export interface ReassignPayload {
  issueId: string; alias: string; to: User; by: User; at: string; sessionIds: string[];
}

/** NEEDS YOU, tier 0: a request on an issue I am assigned. Bound or not: the Hub refuses the
 *  request on an unbound issue, and a request that outlived its binding still needs my answer. */
export function reassignDecides(i: InformInputs): Item<ReassignPayload>[] {
  const mine = mySessionsByIssue(i);
  const out: Item<ReassignPayload>[] = [];
  for (const head of i.heads.values()) {
    const rq = head.reassignRequest;
    if (!rq || head.assignee?.id !== i.me) continue;
    out.push({
      kind: "issue.reassign_decide", id: `reassign:${head.id}:${rq.at}`, arrivedAt: rq.at,
      rankReason: `${rq.by.displayName} wants to reassign it`,
      ifNothing: "nothing moves: your agents keep working and the issue stays yours",
      payload: { issueId: head.id, alias: head.alias, to: rq.to, by: rq.by, at: rq.at,
        sessionIds: (mine.get(head.id) ?? []).map((b) => b.s.id).sort() },
    });
  }
  return out;
}

/** Decision 9: only what W1 can back. `time` formats an RFC 3339 instant as the Desk's HH:MM. */
export interface ZeroFacts {
  landed: { alias: string; word: string; at: string } | null;   // task 64's newest LANDED row
  live: { agents: number; issues: number };
  time: (iso: string) => string;
}
const n = (k: number, one: string, many: string) => `${k} ${k === 1 ? one : many}`;

export function zeroSentence(f: ZeroFacts): string {
  const parts = ["Nothing needs you."];
  if (f.landed) parts.push(`${f.landed.alias} ${f.landed.word} at ${f.time(f.landed.at)}.`);
  if (f.live.agents > 0) parts.push(`${n(f.live.agents, "agent", "agents")} live on ${n(f.live.issues, "issue", "issues")}.`);
  return parts.join(" ");
}

/** Live agents on my issues, for ZeroFacts.live: sessions counted once, issues distinct. */
export function liveOnMyIssues(i: InformInputs): { agents: number; issues: number } {
  const mine = mySessionsByIssue(i);
  let agents = 0;
  for (const list of mine.values()) agents += list.length;
  return { agents, issues: mine.size };
}
```

`task 64`'s LANDED payload names (`alias`, `word`, `at`) are this part's reading of its scope
(*"EP-142 landed and proven at 14:02"*, `tasks-61-80.md:144-146`); `DeskZero` adapts task 64's
payload in one line if its part names them otherwise. `word` is SW.4's result word verbatim
(`landed and proven`, `proven, self-attested`, `done, not proven`), never recomposed here.

#### Interfaces — complete: the row answer, shared by every row this task adds

```ts
// desktop/src/components/desk/rows/answer.ts — the one shape "answer in place" takes (decision 8).
import { ApiError } from "../../../api/client";

export type RowAnswer =
  | { kind: "pending" }
  | { kind: "ok"; line: string }                                  // "re-briefed at @4"
  | { kind: "fail"; seq: number; code: string; message: string }; // the server's own words

let seq = 0;
export function failOf(e: unknown): RowAnswer {
  seq += 1;
  if (e instanceof ApiError) return { kind: "fail", seq, code: e.code, message: e.message };
  return { kind: "fail", seq, code: "unknown", message: "The request did not complete." };
}
```

#### Interfaces — complete: `desktop/src/components/desk/rows/InformRow.tsx`

```tsx
import { useState } from "react";
import type { DeskItem } from "../deskModel";                       // task 62
import type { ChangedPayload, DriftPayload, RelatedPayload } from "../../../work/desk/informs";
import { failOf, type RowAnswer } from "./answer";

export interface InformVerbs {
  /** Re-brief: one rebriefSession per session behind the head. Resolves when all stood. */
  rebrief(p: ChangedPayload): Promise<void>;
  /** Re-point: opens task 42's CriterionPicker for the brief's current criteria, in place;
   *  resolves with the chosen id once the revise op stood, or null when the picker closed. */
  repoint(p: DriftPayload): Promise<string | null>;
  markSeen(itemId: string): void;                                   // task 64's verb (UNVERIFIED)
  time(iso: string): string;
}

type Any = ChangedPayload | RelatedPayload | DriftPayload;

export function InformRow({ item, verbs }: { item: DeskItem<Any>; verbs: InformVerbs }): JSX.Element {
  const [answer, setAnswer] = useState<RowAnswer | null>(null);
  const run = (p: Promise<string | null>) => {
    setAnswer({ kind: "pending" });
    p.then((line) => setAnswer(line === null ? null : { kind: "ok", line }), (e) => setAnswer(failOf(e)));
  };
  const onVerb = () => {
    if (answer?.kind === "pending" || answer?.kind === "ok") return;  // one answer per entry
    if (item.kind === "issue.inform_changed") {
      const p = item.payload as ChangedPayload;
      run(verbs.rebrief(p).then(() => `re-briefed at @${p.headRev}`));
    } else if (item.kind === "issue.inform_drift") {
      run(verbs.repoint(item.payload as DriftPayload).then((c) => (c === null ? null : `re-pointed to ${c}`)));
    } else {
      verbs.markSeen(item.id);
      setAnswer({ kind: "ok", line: "seen" });
    }
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "1" && !e.ctrlKey && !e.metaKey && !e.altKey) { e.preventDefault(); onVerb(); }
  };
  return (
    <div className="desk-row desk-inform" data-kind={item.kind} tabIndex={-1} onKeyDown={onKeyDown}
      data-answered={answer?.kind === "ok" ? "" : undefined}
      data-fail={answer?.kind === "fail" ? answer.seq : undefined}>
      <p className="desk-inform-line">{informText(item, verbs.time)}</p>
      <button type="button" className="desk-verb" onClick={onVerb} disabled={answer?.kind === "pending"}>
        <kbd>1</kbd> {verbWord(item.kind, item.payload)}
      </button>
      {answer?.kind === "ok" && <p className="desk-answer">{answer.line}</p>}
      {answer?.kind === "fail" && <p className="desk-answer desk-answer-fail">refused · {answer.code} · {answer.message}</p>}
    </div>
  );
}

export function verbWord(kind: string, p: Any): string {
  if (kind === "issue.inform_changed") return `re-brief my agents at @${(p as ChangedPayload).headRev}`;
  if (kind === "issue.inform_drift") return "re-point";
  return "seen";
}

/** The inform's sentence. Ids and revisions sit in <span class="mono"> through the CSS below. */
export function informText(item: DeskItem<Any>, time: (iso: string) => string): string {
  if (item.kind === "issue.inform_changed") {
    const p = item.payload as ChangedPayload;
    return `${p.alias} changed since your agent read @${p.adoptedRev} (${p.by.displayName}, ${time(p.at)}): ${p.line}.`;
  }
  if (item.kind === "issue.inform_drift") {
    const p = item.payload as DriftPayload;
    return `${p.alias} serves ${p.criterionId}, which the brief no longer has.`;
  }
  const p = item.payload as RelatedPayload;
  return `${p.by.displayName} added ${p.otherAlias || p.otherId} ${p.relation} ${p.alias} (${time(p.at)}). Recorded.`;
}
```

`answer.ts` joins the file list (new, no test of its own: `InformRow.test.tsx` and
`ReassignDecide.test.tsx` cover it through the rows).

#### Interfaces — complete: `desktop/src/components/desk/rows/ReassignDecide.tsx`

```tsx
import { useState } from "react";
import type { DeskItem } from "../deskModel";
import type { ReassignPayload } from "../../../work/desk/informs";
import { failOf, type RowAnswer } from "./answer";

export interface ReassignVerbs {
  answer(issueId: string, accept: boolean): Promise<unknown>;       // WorkClient.answerReassign
  stopSessions(ids: readonly string[]): Promise<unknown>;           // phase 2's killSession, each
  time(iso: string): string;
}

/** 05 §3.7 rule 2 at tier 0. Nothing is the default: without a key, nothing moves (decision 7). */
export function ReassignDecide({ item, verbs }: { item: DeskItem<ReassignPayload>; verbs: ReassignVerbs }): JSX.Element {
  const p = item.payload;
  const [answer, setAnswer] = useState<RowAnswer | null>(null);
  const settle = (accept: boolean) => {
    if (answer && answer.kind !== "fail") return;
    setAnswer({ kind: "pending" });
    verbs.answer(p.issueId, accept)
      .then(() => (accept ? verbs.stopSessions(p.sessionIds) : undefined))
      .then(
        () => setAnswer({ kind: "ok", line: accept
          ? `answered · handed over to ${p.to.displayName}; uncommitted changes stay in the worktree`
          : "answered · kept" }),
        (e) => setAnswer(failOf(e)),
      );
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key === "1") { e.preventDefault(); settle(true); }
    if (e.key === "2") { e.preventDefault(); settle(false); }
  };
  return (
    <div className="desk-row desk-decide" data-kind={item.kind} tabIndex={-1} onKeyDown={onKeyDown}
      data-answered={answer?.kind === "ok" ? "" : undefined}
      data-fail={answer?.kind === "fail" ? answer.seq : undefined}>
      <p className="desk-decide-line">
        {p.by.displayName} wants to reassign {p.alias} to {p.to.displayName} ({verbs.time(p.at)}).
        {" "}{p.sessionIds.length > 0 ? `${p.sessionIds.length} of your agents work on it.` : "No agent of yours is on it."}
      </p>
      <p className="desk-ifnothing">If you do nothing: {item.ifNothing}.</p>
      <div className="desk-verbs" role="group" aria-label={`Answer the reassign of ${p.alias}`}>
        <button type="button" className="desk-verb" onClick={() => settle(true)} disabled={answer?.kind === "pending"}>
          <kbd>1</kbd> hand over to {p.to.displayName}
        </button>
        <button type="button" className="desk-verb" onClick={() => settle(false)} disabled={answer?.kind === "pending"}>
          <kbd>2</kbd> keep working, decline
        </button>
      </div>
      {answer?.kind === "ok" && <p className="desk-answer">{answer.line}</p>}
      {answer?.kind === "fail" && <p className="desk-answer desk-answer-fail">refused · {answer.code} · {answer.message}</p>}
    </div>
  );
}
```

The order is fixed: the Hub answer first, the stops after. A Hub refusal stops nothing, so a
refused hand-over never leaves me with no agent and the issue still mine. A stop that fails after
an accepted answer shows the stop's refusal on the row: the issue is Alice's and my agent still
runs, which the row says in the server's words; *Enter* opens the session to stop it by hand.

#### Interfaces — complete: `desktop/src/components/work/ReassignMark.tsx`

```tsx
import type { IssueHead } from "../../api/work";

/** Every issue surface's line while a reassign waits (decision 7). Renders nothing otherwise. */
export function ReassignMark({ head }: { head: Pick<IssueHead, "reassignRequest"> }): JSX.Element | null {
  const rq = head.reassignRequest;
  if (!rq) return null;
  return <span className="reassign-mark">reassign requested by {rq.by.displayName} · to {rq.to.displayName}</span>;
}
```

`IssueRow.tsx` (task 40) mounts `<ReassignMark head={head}/>` in its fact line, after the assignee;
`IssueHead.tsx` (task 46) mounts it under the assignee picker. Neither mount changes a layout
dimension: the mark is one inline span in task 26's `--t-fact` (`task-26.md:19-22`; not in
`tokens.css` on the tree today).

#### Interfaces — complete: two op kinds on `desktop/src/work/replica/ops.ts` ⚑ (task 56)

The scope routes every Desk verb through `mutate` (SW.16), so the reassign's two writes are op
kinds, not side calls: they get the pending mark, the refusal, the queue and the Idempotency-Key
for free. One more file joins the list: `ops.ts` ⚑ and its test ⚑ (two cases each).

```ts
// WorkOp gains two arms (task-56.md:156-168):
  | { kind: "requestReassign"; issueId: string; to: User }
  | { kind: "answerReassign"; issueId: string; accept: boolean; to: User }   // `to` paints the accept
// FieldPath (task-56.md:173) gains "reassignRequest".

// fieldsOf: requestReassign → [] (the Hub decides; the row shows "sending" until the head returns)
//           answerReassign  → accept ? ["assignee", "reassignRequest"] : ["reassignRequest"]
// applyLocal paints answerReassign: reassignRequest ← null; on accept, assignee ← op.to.
// undoable(op): false for both (a hand-over is answered once; decision 8's "one answer per entry").
// sendOp gains:
    case "requestReassign": return { head: await client.requestReassign(id, op.to.id) };
    case "answerReassign": return { head: await client.answerReassign(id, op.accept) };
```

#### Interfaces — complete: `desktop/src/components/desk/deskRows.tsx` ⚑ — task 65's block

```tsx
// Appended after tasks 63 and 64's blocks (task 62 owns the file's head and imports).
import { api } from "../../api";                                      // phase 2, api/index.ts:17
import { mutate } from "../../work/replica/replica";                  // SW.16, task 55/56
import * as inf from "../../work/desk/informs";
import { InformRow, type InformVerbs } from "./rows/InformRow";
import { ReassignDecide, type ReassignVerbs } from "./rows/ReassignDecide";

const since = () => ({ tier: 2 as const, waitingSinceMs: 0 });     // the band keeps arrival order
const waited = (at: string) => Math.max(0, Date.now() - Date.parse(at));
const stood = (h: { settled: Promise<"applied" | { code: string; message: string }> }) =>
  h.settled.then((r) => { if (r !== "applied") throw Object.assign(new Error(r.message), r); });

registerDeskSource("issue.inform_changed", (s) => inf.changedInforms(inputsOf(s)));
registerDeskSource("issue.inform_related", (s) => inf.relatedInforms(inputsOf(s)));
registerDeskSource("issue.inform_drift", (s) => inf.driftInforms(inputsOf(s)));
registerDeskSource("issue.reassign_decide", (s) => inf.reassignDecides(inputsOf(s)));

for (const kind of ["issue.inform_changed", "issue.inform_related", "issue.inform_drift"] as const) {
  registerDeskRow(kind, "since", (item) => <InformRowBound item={item} />, since);
}
registerDeskRow<inf.ReassignPayload>("issue.reassign_decide", "needs_you",
  (item) => <ReassignDecide item={item} verbs={reassignVerbs} />,
  (item) => ({ tier: 0, waitingSinceMs: waited(item.arrivedAt) }));

const reassignVerbs: ReassignVerbs = {
  answer: (issueId, accept) => {
    const to = /* the pending request's To, read at press time */ headOf(issueId)!.reassignRequest!.to;
    return stood(mutate({ kind: "answerReassign", issueId, accept, to }));
  },
  stopSessions: (ids) => Promise.all(ids.map((id) => api.killSession(id))),
  time: deskTime,
};

/** Adapts task 62's snapshot to InformInputs. The member names on the right are task 62/63's
 *  and UNVERIFIED until their parts land; this one function is where a rename lands. */
function inputsOf(s: DeskSnapshot): inf.InformInputs {
  return { me: s.me, sessions: s.sessions, heads: s.heads, revisionAt: s.revisionAt,
    refOf: s.refOf, events: s.events, briefCriteria: s.briefCriteria };
}
```

`headOf` and `deskTime` are task 62's (the snapshot's head read and the Desk's `HH:MM`
formatter, both **UNVERIFIED** names). `stood` turns a `Refusal` into a rejection carrying the
server's `code` and `message`, which `failOf` reads (it is not an `ApiError`, so `failOf` gains one
branch: an object with string `code` and `message` is read as they are).

#### Interfaces — complete: the answer book, `answer.ts` part 2 (what "resolved" means to the zero)

A row's answer is its own state, but the zero state counts answers across rows (decision 9), so
the ok answer is also written to one per-visit book. Task 62's `Desk.tsx` ⚑ wraps its sections in
`<AnswerBookProvider>` (one line; a fresh book per mount, because the frame unmounts on leave,
task 62's test 5). `Desk.tsx` joins the file list.

```tsx
// desktop/src/components/desk/rows/answer.ts, appended (the file becomes answer.tsx).
import { createContext, useContext, useState, useSyncExternalStore, type ReactNode } from "react";

export interface AnswerBook {
  has(itemId: string): boolean;
  mark(itemId: string): void;
  subscribe(fn: () => void): () => void;
  version(): number;
}
export function createAnswerBook(): AnswerBook {
  const ids = new Set<string>();
  const subs = new Set<() => void>();
  let v = 0;
  return {
    has: (id) => ids.has(id),
    mark: (id) => { if (!ids.has(id)) { ids.add(id); v += 1; subs.forEach((f) => f()); } },
    subscribe: (fn) => { subs.add(fn); return () => subs.delete(fn); },
    version: () => v,
  };
}
const Ctx = createContext<AnswerBook | null>(null);
export function AnswerBookProvider({ children }: { children: ReactNode }): JSX.Element {
  const [book] = useState(createAnswerBook);
  return <Ctx.Provider value={book}>{children}</Ctx.Provider>;
}
/** Outside a provider (a row rendered alone in a test) the book is a private one. */
export function useAnswerBook(): AnswerBook {
  const shared = useContext(Ctx);
  const [own] = useState(createAnswerBook);
  const book = shared ?? own;
  useSyncExternalStore(book.subscribe, book.version);
  return book;
}
/** The rows' state: an ok answer also marks the book. InformRow and ReassignDecide above read
 *  `const [answer, setAnswer] = useRowAnswer(item.id);` in place of their `useState` line; task
 *  63's and 64's NEEDS YOU rows take the same hook (UNVERIFIED until their parts land). */
export function useRowAnswer(itemId: string): [RowAnswer | null, (a: RowAnswer | null) => void] {
  const book = useAnswerBook();
  const [answer, set] = useState<RowAnswer | null>(null);
  return [answer, (a) => { set(a); if (a?.kind === "ok") book.mark(itemId); }];
}
```

`failOf` gains the branch decision 8's `stood` needs, after the `ApiError` line:

```ts
  if (typeof e === "object" && e !== null && typeof (e as { code?: unknown }).code === "string"
      && typeof (e as { message?: unknown }).message === "string") {
    const r = e as { code: string; message: string };
    return { kind: "fail", seq, code: r.code, message: r.message };   // a Refusal, verbatim
  }
```

#### Interfaces — complete: `desktop/src/components/desk/DeskZero.tsx` (task 62's file, filled)

```tsx
import type { DeskItem } from "./deskModel";                                   // task 62
import { useAnswerBook } from "./rows/answer";
import { zeroSentence, type ZeroFacts } from "../../work/desk/informs";

export interface LandedFacts { alias: string; word: string; at: string }       // task 64's payload

/** Mounted by the frame at the top of NEEDS YOU on every visit. Renders only when no NEEDS YOU
 *  row of this snapshot is unanswered. Plain text: no role="status" and no aria-live, because
 *  task 66 keeps exactly one live region per window and announces through it. */
export function DeskZero(props: {
  needs: readonly DeskItem<unknown>[];
  landed: readonly DeskItem<LandedFacts>[];
  live: ZeroFacts["live"];
  time: (iso: string) => string;
}): JSX.Element | null {
  const book = useAnswerBook();
  if (props.needs.some((i) => !book.has(i.id))) return null;
  const newest = [...props.landed].sort((a, b) => b.arrivedAt.localeCompare(a.arrivedAt))[0];
  const text = zeroSentence({ landed: newest ? newest.payload : null, live: props.live, time: props.time });
  return <p className="desk-zero">{text}</p>;
}
```

#### Interfaces — complete: `InformRowBound`, the last block of `deskRows.tsx` ⚑

The three inform kinds share one bound row. `rebrief` is one `rebriefSession` per session behind
the head, all or a refusal; `repoint` opens task 42's `CriterionPicker` under the row, in place, and
writes one `revise` op that swaps the drifted criterion for the chosen one.

```tsx
import { useState } from "react";
import { CriterionPicker } from "../work/CriterionPicker";                   // task 42
import type { BriefCriterion } from "../../work/triage/suggestCriterion";   // task 42's type; path UNVERIFIED
import { useWork } from "../../work/replica/useWork";                       // task 55; path UNVERIFIED
import type { DeskItem } from "./deskModel";
import type { ChangedPayload, DriftPayload } from "../../work/desk/informs";

type Picking = { p: DriftPayload; ok: (c: string | null) => void; no: (e: unknown) => void };

function InformRowBound({ item }: { item: DeskItem<any> }): JSX.Element {
  const [picking, setPicking] = useState<Picking | null>(null);
  const issueId: string = item.payload.issueId;
  const rec = useWork((r) => r.get(issueId));                    // { head, revision } (task 60's read)
  const criteria: readonly BriefCriterion[] = useProjectCriteria(rec?.head.projectId); // P4-16, UNVERIFIED
  const verbs: InformVerbs = {
    rebrief: (p: ChangedPayload) =>
      Promise.all(p.sessionIds.map((id) => api.rebriefSession(id, p.headKey))).then(() => undefined),
    repoint: (p) => new Promise<string | null>((ok, no) => setPicking({ p, ok, no })),
    markSeen: deskMarkSeen,                                       // task 64's verb, UNVERIFIED name
    time: deskTime,
  };
  const choose = (c: BriefCriterion) => {
    const pk = picking!;
    setPicking(null);
    if (!rec) return pk.no({ code: "not_in_replica", message: "The issue is not loaded. Open it and retry." });
    const serves = rec.revision.content.serves.map((s) =>
      s.briefKey === pk.p.briefKey && s.criterionId === pk.p.criterionId
        ? { briefKey: c.briefKey, criterionId: c.criterionId } : s);
    stood(mutate({ kind: "revise", issueId, baseRev: rec.head.headRevision, patch: { serves } }))
      .then(() => pk.ok(c.criterionId), pk.no);
  };
  return (
    <>
      <InformRow item={item} verbs={verbs} />
      {picking && rec && (
        <CriterionPicker
          issue={{ title: rec.revision.content.title.text, description: rec.revision.content.description.text }}
          criteria={criteria} serving={rec.revision.content.serves}
          onChoose={choose} onClose={() => { picking.ok(null); setPicking(null); }} />
      )}
    </>
  );
}
```

A `revise` against a stale `baseRev` is task 56's refusal path (`409`, the Hub's sentence on the
row); this task adds no merge of its own. `useProjectCriteria` is the one read here that no part
names: task 42's `TriageView` passes `criteria` to the picker from P4-16's loader, and its hook
name is **UNVERIFIED**; this task imports whatever task 42 exports for it and adds no second read.

#### Interfaces — complete: appended to `desktop/src/components/desk/desk.css` ⚑ (task 62)

```css
/* Task 65. An answered row stays in place, dimmed; nothing moves (decision 8). */
.desk-row[data-answered] { opacity: 0.56; }
.desk-row[data-answered] .desk-verb { visibility: hidden; }  /* the space stays: no jump */
.desk-answer { font: var(--t-fact); color: var(--ink-2); margin: 0; }
.desk-answer-fail { color: var(--fail); }
/* The --fail flash: task 26's keyframes on opacity only, keyed by data-fail's seq so a
   second refusal flashes again. */
.desk-row[data-fail] { animation: flash-fail var(--dur-flash) var(--ease-out) 1; }
.desk-inform-line .mono, .desk-decide-line .mono { font-family: var(--font-mono); }
.desk-inform { border-inline-start: 2px solid transparent; }
.desk-decide { box-shadow: var(--ring-1), var(--shadow-1); border-radius: var(--r-3); }
.desk-zero { font: var(--t-body); color: var(--ink); margin: 0; padding-block: var(--s-2); }
.reassign-mark { font: var(--t-fact); color: var(--ink-2); }
@media (prefers-reduced-motion: reduce) { .desk-row[data-fail] { animation: none; } }
```

`--ink`, `--ink-2` and `--fail` exist today (`desktop/src/styles/tokens.css:45-46`, `:76`). The
rest (`--dur-flash`, `--ease-out`, `--ring-1`, `--shadow-1`, `--r-3`, `--s-2`, `--t-body`,
`--t-fact`, `--font-mono`, `@keyframes flash-fail`) are W1's design-language names from tasks 26
and 33 and are **UNVERIFIED**: a grep of `desktop/src` on 2026-09-26 finds none of them. This block
takes whatever those parts name them. The flash animates `opacity` only; no `transition: all`, no
layout property.

#### Tests to write first, part 1 — `desktop/src/work/desk/informs.test.ts` (the pure selectors)

The inputs are built by hand, not through the replica: each test names the revisions it has. `S`
builds a phase 2 `Session` with the three fields the selector reads; `refs` is the loader.

```ts
import { describe, expect, it } from "vitest";
import { ALICE, DYLAN, makeIssueHead, makeIssueRevision, makeServe, WORK_IDS } from "../../test/workFixtures";
import type { IssueRevision } from "../../api/work";
import type { Session } from "../../store/types";
import * as inf from "./informs";

const EP = WORK_IDS.ep142;
const S = (id: string, rev: number, status = "running"): Session =>
  ({ id, status, issueRef: `issue:${EP}@${rev}` }) as unknown as Session;
function inputs(o: { head: number; writer: typeof DYLAN; sessions: Session[]; headServes?: string[] }): inf.InformInputs {
  const revs = new Map<number, IssueRevision>();
  for (let r = 1; r <= o.head; r++) {
    const serves = r === o.head && o.headServes ? o.headServes.map((c) => makeServe({ criterionId: c })) : [makeServe()];
    revs.set(r, makeIssueRevision({ revision: r, writtenBy: r === o.head ? o.writer : DYLAN,
      writtenAt: `2026-09-26T11:0${r}:00Z`, content: { ...makeIssueRevision().content, serves } }));
  }
  return {
    me: DYLAN.id, sessions: o.sessions,
    heads: new Map([[EP, makeIssueHead({ headRevision: o.head, state: "started" })]]),
    revisionAt: (_id, r) => revs.get(r) ?? null,
    refOf: (key) => { const r = Number(key.split("@")[1]);     // the TEST may split; the code never does
      return { kind: "issue", issueId: EP, revision: r, serves: revs.get(r)!.content.serves } as never; },
    events: [], briefCriteria: () => [WORK_IDS.criterionResume, "c-40aa"],
  };
}

describe("informs", () => {
  it("a newer head written by me makes no inform; by Alice it makes one", () => {
    expect(inf.changedInforms(inputs({ head: 4, writer: DYLAN, sessions: [S("s-1", 3)] }))).toEqual([]);
    const [row] = inf.changedInforms(inputs({ head: 4, writer: ALICE, sessions: [S("s-1", 3)] }));
    expect(row.payload).toMatchObject({ adoptedRev: 3, headRev: 4, by: ALICE, sessionIds: ["s-1"] });
    // mutant: any revision gap informs (drop the writtenBy check)
  });
  it("names what changed in one line and never quotes the text", () => {
    const [row] = inf.changedInforms(inputs({ head: 4, writer: ALICE, sessions: [S("s-1", 3)],
      headServes: [WORK_IDS.criterionResume, "c-40aa"] }));
    expect(row.payload.line).toBe("serves c-40aa added");
    expect(JSON.stringify(row)).not.toContain("A restarted sandbox");
    // mutant: the head's description in the line
  });
  it("one item per issue, from the oldest adopted revision, stable id across re-derives", () => {
    const i = inputs({ head: 4, writer: ALICE, sessions: [S("s-2", 2), S("s-1", 3), S("s-3", 4)] });
    const a = inf.changedInforms(i), b = inf.changedInforms(i);
    expect(a).toHaveLength(1);
    expect(a[0].payload).toMatchObject({ adoptedRev: 2, sessionIds: ["s-1", "s-2"] });
    expect(a[0].id).toBe(b[0].id);
    // mutants: one item per session; the session already at @4 listed
  });
  it("an exited session or a resolving ref says nothing", () => {
    expect(inf.changedInforms(inputs({ head: 4, writer: ALICE, sessions: [S("s-1", 3, "exited")] }))).toEqual([]);
    const i = { ...inputs({ head: 4, writer: ALICE, sessions: [S("s-1", 3)] }), refOf: () => "resolving" as const };
    expect(inf.changedInforms(i)).toEqual([]);
    // mutant: the selector reads the revision out of issueRef by splitting it (decision 2)
  });
  it("a reassign on todo is a plain assign; on started it is a request", () => {
    const todo = makeIssueHead({ state: "todo", assignee: ALICE });
    expect(inf.reassignWrite(todo, DYLAN, DYLAN.id).via).toBe("op");
    const started = makeIssueHead({ state: "started", assignee: ALICE });
    expect(inf.reassignWrite(started, DYLAN, DYLAN.id)).toEqual({ via: "request", issueId: started.id, toUserId: DYLAN.id });
    expect(inf.reassignWrite(makeIssueHead({ state: "started", assignee: DYLAN }), ALICE, DYLAN.id).via).toBe("op");
    // mutants: a decide for every reassign; a request when I move my own issue
  });
  it("the zero sentence says only what W1 backs, with no ~ and no horizon", () => {
    const s = inf.zeroSentence({ landed: { alias: "EP-142", word: "landed and proven", at: "2026-09-26T04:02:00Z" },
      live: { agents: 3, issues: 2 }, time: () => "14:02" });
    expect(s).toBe("Nothing needs you. EP-142 landed and proven at 14:02. 3 agents live on 2 issues.");
    expect(inf.zeroSentence({ landed: null, live: { agents: 0, issues: 0 }, time: () => "" })).toBe("Nothing needs you.");
    expect(s).not.toMatch(/~|expected|next decision/i);
    // mutant: AX's "Next decision expected ~14:22" appended
  });
});
```

The drift and related selectors take two more cases each in the same file, in the same shape:
a served id missing from `briefCriteria` informs once and a `"resolving"` brief says nothing
(mutant: drift on a resolving brief); a relation Alice added on either end informs and one I added
does not (mutant: the actor check dropped). That makes 10 cases in this file.

#### Tests to write first, part 2 — the rows: `ReassignDecide.test.tsx`, `InformRow.test.tsx`, `DeskZero.test.tsx`

The rows render alone with spy verbs (their own answer book), except the zero test, which wraps
the decide and the zero in one `AnswerBookProvider` as the frame does.

```tsx
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { expectNoPrediction } from "../../../test/expectNoPrediction";
import { ALICE, DYLAN } from "../../../test/workFixtures";
import { ReassignDecide } from "./ReassignDecide";
import { InformRow } from "./InformRow";
import { DeskZero } from "../DeskZero";
import { AnswerBookProvider } from "./answer";

const item = { kind: "issue.reassign_decide", id: "reassign:is-1:t", arrivedAt: "2026-09-26T01:00:00Z",
  rankReason: "", ifNothing: "nothing moves: your agents keep working and the issue stays yours",
  payload: { issueId: "is-1", alias: "EP-142", to: ALICE, by: ALICE, at: "2026-09-26T01:00:00Z", sessionIds: ["s-1"] } };
const verbs = () => ({ answer: vi.fn(async () => ({})), stopSessions: vi.fn(async () => []), time: () => "11:00" });

describe("the reassign decide", () => {
  it("2, keep working: answers decline once and stops nothing", async () => {
    const v = verbs();
    render(<ReassignDecide item={item} verbs={v} />);
    fireEvent.keyDown(screen.getByText(/wants to reassign/).parentElement!, { key: "2" });
    await screen.findByText("answered · kept");
    expect(v.answer).toHaveBeenCalledWith("is-1", false);
    expect(v.stopSessions).not.toHaveBeenCalled();
    // mutant: the default (or decline) pauses or stops the session
  });
  it("without a key nothing is sent at all", () => {
    const v = verbs();
    render(<ReassignDecide item={item} verbs={v} />);
    expect(v.answer).not.toHaveBeenCalled();
    expect(screen.getByText(/If you do nothing: nothing moves/)).toBeTruthy();
    // mutant: a timeout that answers for me
  });
  it("1, hand over: the Hub answer first, the stops after; a refusal stops nothing", async () => {
    const v = verbs();
    v.answer.mockRejectedValueOnce({ code: "not_assignee", message: "Only the assignee answers a reassign." });
    render(<ReassignDecide item={item} verbs={v} />);
    fireEvent.click(screen.getByText(/hand over to/));
    await screen.findByText(/refused · not_assignee · Only the assignee/);
    expect(v.stopSessions).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText(/hand over to/));
    await screen.findByText(/handed over to Alice Example/);
    expect(v.stopSessions).toHaveBeenCalledWith(["s-1"]);
    // mutants: stops before the answer; a refusal swallowed as success
  });
});

describe("answer in place", () => {
  it("no verb changes the route or the lens", async () => {
    const before = window.location.href;
    const v = { rebrief: vi.fn(async () => {}), repoint: vi.fn(async () => null), markSeen: vi.fn(), time: () => "11:02" };
    const inform = { ...item, kind: "issue.inform_changed", id: "inform_changed:is-1@4",
      payload: { issueId: "is-1", alias: "EP-142", adoptedRev: 3, headRev: 4, headKey: "issue:is-1@4",
        by: ALICE, at: "2026-09-26T01:02:00Z", line: "serves c-40aa added", sessionIds: ["s-1"] } };
    const { container } = render(<InformRow item={inform as never} verbs={v} />);
    expect(v.rebrief).not.toHaveBeenCalled();                       // nothing reaches the agent unasked
    fireEvent.keyDown(container.firstElementChild!, { key: "1" });
    await screen.findByText("re-briefed at @4");
    expect(window.location.href).toBe(before);
    expect(v.rebrief).toHaveBeenCalledTimes(1);
    expectNoPrediction(container);
    // mutants: the head change auto-re-briefs; a verb that dives into the issue view
  });
  it("answering the last NEEDS YOU row shows the zero; FAILED stays below it", async () => {
    const failed = <section aria-label="FAILED"><h2>FAILED 1</h2><p>EP-150 exit at 02:14</p></section>;
    const { container } = render(
      <AnswerBookProvider>
        <DeskZero needs={[item as never]} landed={[{ ...item, payload: { alias: "EP-142", word: "landed and proven", at: "t" } } as never]}
          live={{ agents: 3, issues: 2 }} time={() => "14:02"} />
        <ReassignDecide item={item} verbs={verbs()} />
        {failed}
      </AnswerBookProvider>);
    expect(container.querySelector(".desk-zero")).toBeNull();
    fireEvent.keyDown(screen.getByText(/wants to reassign/).parentElement!, { key: "2" });
    await waitFor(() => expect(container.querySelector(".desk-zero")?.textContent)
      .toBe("Nothing needs you. EP-142 landed and proven at 14:02. 3 agents live on 2 issues."));
    expect(screen.getByLabelText("FAILED").textContent).toContain("EP-150");
    expect(container.querySelector("[aria-live],[role=status]")).toBeNull();   // task 66 owns the one region
    expectNoPrediction(container);
    // mutants: the zero never reachable (counts rows, not answers); FAILED hidden under a calm state
  });
});
```

The FAILED section here is a stand-in for task 64's rows: this test proves that `DeskZero` neither
hides a sibling nor replaces its section; task 62's test 6 proves the frame keeps FAILED's header.

**The source guard** (in `InformRow.test.tsx`): `import.meta.glob("/src/**/*.{ts,tsx}", { query:
"?raw", eager: true })`, and the only files whose text contains `rebriefSession(` are
`src/api/client.ts` and `src/components/desk/deskRows.tsx` (mutant: a replica listener that
re-briefs on a new head, which is scope test 2's failure).

#### Tests to write first, part 3 — the fake's rules: `hubclient/conformance/work_reassign.go` (new)

Rows in task 05's `WorkRow` shape, in their own slice so `WorkRowsW1`'s count does not move. Their
ids carry the owning task (`W-65.n`): a grep of the parts on 2026-09-26 finds `W-01`…`W-25` only,
and tasks 73, 99, 115 and 144 append their own numbers, so a task-scoped id cannot collide.
`hubclient/fake_work_test.go` ⚑ gains one line; the HTTP run (task 16) does **not** take this
slice until the WH task builds the route (a gap), so the rows prove the fake, not the Hub.

```go
package conformance

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// WorkRowsReassign is task 65's slice (05 §3.7 rule 2).
var WorkRowsReassign = []WorkRow{
	{"W-65.1", "reassign_request_needs_a_bound_lane", wReassignNeedsBound},
	{"W-65.2", "reassign_request_moves_nothing", wReassignMovesNothing},
	{"W-65.3", "reassign_decline_clears_only_the_request", wReassignDecline},
	{"W-65.4", "reassign_accept_moves_the_assignee_once", wReassignAccept},
	{"W-65.5", "a_direct_assign_settles_a_pending_request", wReassignSettledByPatch},
}

func uid(t *testing.T, h hubclient.Hub) string {
	t.Helper()
	me, err := h.Me(wctx) // hubclient.go:32
	if err != nil {
		t.Fatal(err)
	}
	return me.User.ID
}

// bound is a chore assigned to the member and moved to started by hand.
func (e WorkEnv) bound(t *testing.T) hubapi.IssueHead {
	t.Helper()
	iw := e.issue(t, e.project(t, "reassign", "EP"), "resume")
	if _, err := e.Member.PatchIssue(wctx, iw.Head.ID, hubapi.PatchIssueRequest{AssigneeID: hubapi.AssignTo(uid(t, e.Member))}); err != nil {
		t.Fatal(err)
	}
	h, err := e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateStarted, Cause: hubapi.CauseHand, Evidence: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func wReassignNeedsBound(t *testing.T, e WorkEnv) {
	iw := e.issue(t, e.project(t, "reassign", "EP"), "todo one")
	_, err := e.Admin.RequestReassign(wctx, iw.Head.ID, hubapi.RequestReassignRequest{ToUserID: uid(t, e.Admin)})
	wRefused(t, err, 422, "reassign_not_bound") // mutant: a request on any state
}

func wReassignMovesNothing(t *testing.T, e WorkEnv) {
	h := e.bound(t)
	got, err := e.Admin.RequestReassign(wctx, h.ID, hubapi.RequestReassignRequest{ToUserID: uid(t, e.Admin)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Assignee == nil || got.Assignee.ID != uid(t, e.Member) || got.State != hubapi.StateStarted {
		t.Fatalf("assignee %v state %s: a request moved something", got.Assignee, got.State) // mutant: moved at once
	}
	if got.ReassignRequest == nil || got.ReassignRequest.To.ID != uid(t, e.Admin) {
		t.Fatalf("reassignRequest = %v", got.ReassignRequest)
	}
	_, err = e.Member.AnswerReassign(wctx, h.ID, hubapi.AnswerReassignRequest{Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Admin.AnswerReassign(wctx, h.ID, hubapi.AnswerReassignRequest{Accept: true})
	wRefused(t, err, 409, "no_reassign_pending") // answered once
}
```

```go
func wReassignDecline(t *testing.T, e WorkEnv) {
	h := e.bound(t)
	if _, err := e.Admin.RequestReassign(wctx, h.ID, hubapi.RequestReassignRequest{ToUserID: uid(t, e.Admin)}); err != nil {
		t.Fatal(err)
	}
	got, err := e.Member.AnswerReassign(wctx, h.ID, hubapi.AnswerReassignRequest{Accept: false})
	if err != nil {
		t.Fatal(err)
	}
	if got.ReassignRequest != nil || got.Assignee.ID != uid(t, e.Member) || got.State != hubapi.StateStarted {
		t.Fatalf("decline changed more than the request: %+v", got) // mutant: decline unassigns or pauses
	}
}

func wReassignAccept(t *testing.T, e WorkEnv) {
	h := e.bound(t)
	if _, err := e.Admin.RequestReassign(wctx, h.ID, hubapi.RequestReassignRequest{ToUserID: uid(t, e.Admin)}); err != nil {
		t.Fatal(err)
	}
	_, err := e.Admin.AnswerReassign(wctx, h.ID, hubapi.AnswerReassignRequest{Accept: true})
	wRefused(t, err, 403, "not_assignee") // mutant: the asker answers her own request
	got, err := e.Member.AnswerReassign(wctx, h.ID, hubapi.AnswerReassignRequest{Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Assignee == nil || got.Assignee.ID != uid(t, e.Admin) || got.ReassignRequest != nil {
		t.Fatalf("accept: assignee %v request %v", got.Assignee, got.ReassignRequest)
	}
	kinds := wKinds(t, e.Member, "", "issue.reassign_")
	if len(kinds) != 2 || kinds[0] != hubapi.EventIssueReassignRequested || kinds[1] != hubapi.EventIssueReassignAnswered {
		t.Fatalf("events = %v", kinds) // mutant: no answered event, so a replica keeps the request
	}
}

func wReassignSettledByPatch(t *testing.T, e WorkEnv) {
	h := e.bound(t)
	if _, err := e.Admin.RequestReassign(wctx, h.ID, hubapi.RequestReassignRequest{ToUserID: uid(t, e.Admin)}); err != nil {
		t.Fatal(err)
	}
	got, err := e.Admin.PatchIssue(wctx, h.ID, hubapi.PatchIssueRequest{AssigneeID: hubapi.AssignTo(uid(t, e.Admin))})
	if err != nil {
		t.Fatal(err)
	}
	if got.ReassignRequest != nil {
		t.Fatal("a direct assign left the request pending") // mutant: PatchIssue ignores the request
	}
}
```

```go
// hubclient/fake_work_test.go ⚑ — one line after task 05's TestWorkRowsOnTheFake
func TestReassignRowsOnTheFake(t *testing.T) { conformance.RunWork(t, workFakeEnv, conformance.WorkRowsReassign...) }
```

`wKinds` from `""` reads the whole feed of a fresh world (task 05's helper, `task-05.md:186-200`);
a fresh `WorkEnv` per row makes that exact. Whether `PatchIssue` by the team's admin may reassign a
started issue directly, without a request, is SW.1's existing rule (an admin assign is not a
reassign *request*); this row pins only that the pending request does not outlive it.

#### Steps

- [ ] Add the Hub delta (type, field, two kinds, two methods, two route rows, two `workCalls`) in
      one commit; `go build ./control-plane/...` passes and task 16's `registerWork` does not panic.
- [ ] Write `work_reassign.go`; the five rows fail with "not implemented" on the fake. Write the
      fake's two methods and its three clearing lines until they pass. Mutants, one at a time,
      then revert: drop the state check (W-65.1); set `is.assignee` in `RequestReassign` (W-65.2);
      clear the assignee on decline (W-65.3); skip the `is.assignee != me` case (W-65.4); leave
      `PatchIssue` untouched (W-65.5).
- [ ] Add the wire (`work.ts`, `client.ts`, the two op kinds); task 37's wire test and task 56's
      op tests still pass with the added keys.
- [ ] Write `informs.test.ts`; it fails to import. Write `informs.ts` until the ten pass. Mutants:
      drop the `writtenBy` check; split `issueRef` in `mySessionsByIssue`; return `op` for every
      `reassignWrite`; append *"Next decision expected ~14:22"* to `zeroSentence`.
- [ ] Write the row tests; write `answer.tsx`, `InformRow`, `ReassignDecide`, `ReassignMark`,
      `DeskZero`, then the `deskRows.tsx` block and the CSS. Mutants: call `stopSessions` before
      `answer`; `navigate` inside `onVerb`; count `needs.length` in `DeskZero`; add `role="status"`.
- [ ] Mount `ReassignMark` in `IssueRow` and `IssueHead`; tasks 40 and 46's suites still pass.
- [ ] Commit each file as it goes green.
