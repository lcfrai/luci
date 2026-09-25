### Task 24: Control plane: Start from an issue, *Undo Start*, the data-class route

Stream **WP**, defines seam **SW.10**, about 3 hours (an estimate, not a measurement). Depends on
task 16 (the relay, `Deps.Work`), task 19 (`Session.IssueRef`, the create request's `issueKey`,
SW.7) and task 21 (`IssueBranchName`, `Lane.IssueID`); P3-04 (create a worktree), P3-11 (remove
one, with its confirm token) and P3-24 (the lane runner's status) arrive through task 21's sync
base, and task 22's `IssueFacts.LaneBound` writes `started`. Phase 3's names are **UNVERIFIED
until merge**, cited from its plan at `9935278`.

`Shift+R` on a row is one keystroke; this task is everything it sets off on the control plane.
One route, `POST /v1/work/start`, does five things in order and **undoes the ones before it when
any step fails**: it reads the issue and **pins its revision**, optionally accepts it out of
`triage`, makes a lane named for the issue, starts a session on it with the issue in its managed
block, and writes `started`. A second route undoes all of it for as long as the agent has not
written a file. A third discards the lane's work after the person has seen what would be lost.

**The revision is pinned, never "latest".** Start reads the head and its head revision `@r` in one
call and puts `issue:is-…@r` in the session. A teammate who edits the issue a second later writes
`@r+1`; the agent keeps working from `@r`, and the issue view says so (task 47). This is 05 §3.4's
rule that issue text enters an agent only after its owner adopted it, and Start is the adoption.

**The route is decided here, and it fails safe.** A project's data class is set at its creation
(`OD-24`). An *AU data only* project forces the `au` route and a slug-free branch name
(`IssueBranchName(…, auOnly: true)`, task 21), whatever the person picked. **Where the class lives
is UNVERIFIED until merge** (Phase T `task-01.md:484-486` at `04e6944`: *"its values and its home
on the project record are UNVERIFIED until merge"*), so this task reads it through a one-method
interface, and **a class it cannot read is treated as AU**. Otherwise the person's choice wins,
default `global` (`OD-19`). In local mode there is no gateway, so the route is returned and shown
(the chip, task 61) and phase 6's token mint consumes it later; nothing here sends inference.

***Undo Start* has no timer.** 05 §4.6 allows it until the **first file write**, observed
mechanically: the lane's `git status` turning non-empty, or its `HEAD` moving off the commit the
lane was created at. Both are read through phase 3's lane runner inside the sandbox (P3-24's
`Status` op: porcelain v2 `-z` bytes and the head), never the PTY (`OD-8`). So an agent that thinks
for thirty seconds before touching a file can still be undone at second twenty-nine, and a
six-second toast (`--undo-window`, task 26) is the desktop's affordance, not the rule. After the
first write, undo answers `409 first_write` with `next: "discard"`, and the desktop offers *Stop and
discard*, which is P3-11's two-step removal: the first call returns what would be lost and a
confirm token, the second carries the token.

**Undo is exact, and it is the person's.** It stops the session, removes the worktree, and moves the
issue back to the state it had **before Start** (so an *accept and Start* from `triage` returns to
`triage`), with cause `start_undone`, as the person who started it. If anyone moved the issue after
Start (it is no longer `started`), undo removes the session and the lane and **leaves the state
alone**: it never overwrites a state someone else set. `start_undone` is the one backward fact move;
task 10's cause list, which calls itself *the complete W1 set*, does not have it, so **SW.4 must
gain it** (a gap, raised in the return).

**What Start remembers, and for how long.** The control plane keeps each Start (its session, lane,
issue, prior state, base commit and route) **in memory**, keyed by session id and by `requestId`.
A repeated `requestId` returns the first result, so a double press is one Start. The record goes
when the session is removed or the process exits; an undo after a restart answers `404
start_not_found`, and the desktop falls back to the ordinary session and worktree verbs. That is a
deliberate limit, named again at the end: an undo record on disk would be one more file of Hub
state on the person's machine.

**Files:**
- `control-plane/internal/workstart/start.go` — `Request`, `Result`, the collaborator interfaces,
  `Starter`, `Start` (new)
- `control-plane/internal/workstart/undo.go` — `Undo`, `Discard`, `FirstWrite` (new)
- `control-plane/internal/workstart/start_test.go`, `undo_test.go` — the rows below, on fakes (new)
- `control-plane/internal/api/workstart.go` — the three routes; the adapters over `lanes.Creator`,
  P3-11's removal, the lane runner and the built session handlers (new)
- `control-plane/internal/api/workstart_test.go` — the wire rows (new)
- `control-plane/internal/api/api.go` ⚑ — `Deps.WorkStart *workstart.Starter` (optional) and
  `a.registerWorkStart(guarded)` beside task 16's `a.registerWork(guarded)` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — one test that the three patterns are POSTs (modified)
- `control-plane/cmd/api/config.go` ⚑, `main.go` ⚑ — build the `Starter` when `Deps.Work` and
  `GitDeps` both exist; no new variable (modified)

**Interfaces:**
- Produces (seam **SW.10**): the wire below; `workstart.Starter` with `Start`, `Undo`, `Discard`;
  `workstart.DataClass`; the problem codes `start_not_found`, `first_write`, `issue_not_startable`
- Consumes:
  - task 02 — `hubapi.IssueKey{IssueID, Revision}.String()` (task 02 part `:108-115`);
    task 08 — `hubapi.IssueWithRevision{Head, Revision}` (task 08 part `:176-180`);
    task 03 — `IssueHead.{State, Alias, ProjectID}`, `IssueRevision.{Revision, Content}`,
    `Serve.BriefKey`; task 10 — `StateMove`, `hubapi.CauseHand`, `hubapi.CauseStartUndone`
    (**the second is not in task 10's list**: gap)
  - task 16 — `Deps.Work`'s `Issue` and `MoveIssueState`
  - task 19 — the create request's `issueKey` field and `Session.IssueRef` from the result
  - task 21 — `lanes.IssueBranchName`, `lanes.CreateRequest.IssueID`, `lanes.Lane.IssueID`
  - task 22 — `(*converge.IssueFacts).LaneBound`
  - built — `sessionCreateRequest` (`sessions.go:696-707`), `handleSessionCreate` (`:759`),
    `handleSessionDelete` (`:1681`), `httpx.DecodeJSON`, `httpx.WriteProblem`
  - P3-03/04/11/24 — `lanes.Creator.Create` (P3 `task-03.md:199`), `converge.Inspect` and
    `lanes.Remove` (P3 `task-11.md:160-165`, `:210-211`), `RemovalState.NothingToLose` and
    `.Confirm` (`:96`, `:324`), `lanegit.Lanes.For` and `Runner.Status` (P3 `task-24.md:565-567`,
    `:659`)
- Amends: SW.4 gains `start_undone` (task 10's owner); nothing else in the Hub.

#### Contract — the wire (SW.10)

```jsonc
// POST /v1/work/start
{ "projectId": "p-…", "issueId": "is-3f2a91c07b1e4d2a", "branch": "main",
  "accept": false,            // optional; true moves triage → todo first, as the person (L-47)
  "route": "global",          // optional: "global" | "au"; ignored (forced au) for an AU-only project
  "harness": "claude-code",   // optional; "" is the session create's own default
  "requestId": "rq-local-7f3a…" }  // required, 8-64 of [A-Za-z0-9._-]; a repeat returns the first result

// 201
{ "issueKey": "issue:is-3f2a91c07b1e4d2a@3", "worktreeId": "w-4a77b1c9", "sessionId": "s-…",
  "branch": "ep-142-resume-upload-fails-on-safari", "route": "global", "routeForced": false,
  "undo": "undo_start" }

// POST /v1/work/start/{sessionId}/undo       (no body)
// 200 { "undone": true, "stateRestored": "todo", "cost": null }   // stateRestored null when left alone
// 409 { "error": "first_write", "details": { "next": "discard" } }
// 404 { "error": "start_not_found" }

// POST /v1/work/start/{sessionId}/discard
{ "confirm": "" }   // first call: "" → 409 confirm_required with details.paths and details.confirm
{ "confirm": "9c1e…" } // second call: P3-11's 16-hex token → 200 { "discarded": true, "stateRestored": … }
```

| Refusal | Status, code | `details` |
|---|---|---|
| a field missing, `requestId` malformed, `route` not `global`/`au`, `issueId` not an `is-` id | 400 `bad_request` | `field` |
| no Hub (`Deps.Work` nil) or no git (`GitDeps` nil) | 503 `hub_not_configured` / `git_not_ready` | — |
| the issue is not in `triage`, `backlog` or `todo`, or it is `triage` and `accept` is false | 409 `issue_not_startable` | `state` |
| the issue belongs to another project | 404 `issue_not_found` (the Hub's code, task 08) | `issueId` |
| the Hub refuses or is down | the relay rule (hub-contract §4.3), unchanged | as the Hub's |
| a lane or session step fails | that step's own code, after the earlier steps are undone | as the step's |

`cost` is `null` in local mode: provider cost comes through the gateway (phase 6, **UNVERIFIED**)
and nothing is estimated here (`OD-27`).

#### Interfaces — complete

```go
// workstart/start.go
package workstart

// Request is one Start, decoded and validated by the handler.
type Request struct {
	ProjectID, IssueID, Branch string
	Accept                     bool
	Route                      string // "" | "global" | "au"
	Harness                    string // "" means the session create's default
	RequestID                  string
}

// Result is SW.10's 201 body.
type Result struct {
	IssueKey    string `json:"issueKey"`
	WorktreeID  string `json:"worktreeId"`
	SessionID   string `json:"sessionId"`
	Branch      string `json:"branch"`
	Route       string `json:"route"`
	RouteForced bool   `json:"routeForced"`
	Undo        string `json:"undo"` // always "undo_start"
}

// Issues is the part of hubclient.Work Start needs; Deps.Work satisfies it.
type Issues interface {
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
	MoveIssueState(ctx context.Context, issueID string, m hubapi.StateMove) (hubapi.IssueHead, error)
}

// DataClass says whether a project is AU data only (OD-24). Its home is UNVERIFIED until merge;
// any error, including "this project has no class", is treated as AU by Start.
type DataClass interface {
	AUOnly(ctx context.Context, projectID string) (bool, error)
}

// LaneMaker is lanes.Creator plus P3-11's removal, behind the api adapter.
type LaneMaker interface {
	Create(ctx context.Context, projectID, branch, name, issueID string) (lanes.Lane, error)
	// Remove removes a lane that has nothing to lose (RemovalState.NothingToLose), and refuses
	// ErrWouldLose otherwise; it never takes a confirm token.
	Remove(ctx context.Context, lane lanes.Lane) error
	// Discard is P3-11's two-step removal: confirm "" returns the paths and a fresh token as a
	// *ConfirmNeeded; the token shown returns nil after removing; a stale one *ConfirmNeeded again.
	Discard(ctx context.Context, lane lanes.Lane, confirm string) error
}

// Sessions is the built session handlers, called in process by the api adapter.
type Sessions interface {
	Create(ctx context.Context, s SessionSpec) (sessionID string, err error)
	Stop(ctx context.Context, sessionID string) error
}

// SessionSpec is what Start asks of POST /v1/sessions.
type SessionSpec struct {
	WorktreeID string
	IssueKey   string // "issue:is-…@r"; task 19's Materialiser stamps Session.IssueRef from it
	ContextRef string // the first serves pair's brief key, or ""
	Harness    string
	Title      string // the display alias and the title, e.g. "EP-142 Resume upload fails on Safari"
}

// Workspace reads the lane mechanically, through the lane runner in the sandbox (OD-8).
type Workspace interface {
	// Base is the lane's HEAD sha now; Start records it right after Create.
	Base(ctx context.Context, lane lanes.Lane) (string, error)
	// Written reports the first file write: status non-empty, or HEAD != base.
	Written(ctx context.Context, lane lanes.Lane, base string) (bool, error)
}

// Facts is task 22's IssueFacts, narrowed.
type Facts interface {
	LaneBound(ctx context.Context, lane lanes.Lane) error
}

// ErrWouldLose, *ConfirmNeeded and the problem codes the handler maps.
var (
	ErrWouldLose     = errors.New("workstart: the lane has work that would be lost")
	ErrNotFound      = errors.New("workstart: no Start with that session id") // 404 start_not_found
	ErrFirstWrite    = errors.New("workstart: the agent has written a file")  // 409 first_write
	ErrNotStartable  = errors.New("workstart: the issue is not in a state Start takes")
	ErrIssueElsewhere = errors.New("workstart: the issue is not in this project") // 404 issue_not_found
)

type ConfirmNeeded struct {
	Paths   []string // [] when the loss is unlanded commits only
	Confirm string   // P3-11's 16-hex token over what was shown
}

func (c *ConfirmNeeded) Error() string { return "workstart: confirm the discard" }
```

```go
// workstart/start.go, continued

// Starter holds every Start this process made, in memory only. Safe for concurrent use; one
// mutex serialises Start, Undo and Discard for the same request id or session id.
type Starter struct {
	Issues    Issues
	Class     DataClass // nil means every class is unknown, so every project routes AU
	Lanes     LaneMaker
	Sessions  Sessions
	Workspace Workspace
	Facts     Facts // nil when no Hub: then Start is refused before this is reached

	mu        sync.Mutex
	byRequest map[string]Result
	bySession map[string]started
}

// started is what Undo needs. prior is the issue's state before Start (before the accept).
type started struct {
	lane  lanes.Lane
	issue string
	prior hubapi.IssueState
	base  string
}

var startable = map[hubapi.IssueState]bool{
	hubapi.StateTriage: true, hubapi.StateBacklog: true, hubapi.StateTodo: true,
}

// Start, in this order; a failure at step n runs the undo of steps n-1 … 1, newest first, and
// returns step n's error. The undo of a step never fails the return: its error is logged.
//  0. byRequest[RequestID] → that Result, and nothing else runs.
//  1. iw = Issues.Issue(IssueID); iw.Head.ProjectID != ProjectID → ErrIssueElsewhere (404
//     issue_not_found, the Hub's own code, so another project's issue is indistinguishable from
//     none); state not startable, or triage without Accept → ErrNotStartable.
//     key = hubapi.IssueKey{IssueID, iw.Revision.Revision}.String(): pinned here, once.
//  2. Accept and triage → MoveIssueState(todo, cause hand, reason "accepted to start").
//     undo: MoveIssueState(triage, cause start_undone).
//  3. auOnly, err = Class.AUOnly(ProjectID); err != nil or Class nil → auOnly = true.
//     route = "au" when auOnly, else Route or "global"; routeForced = auOnly && Route == "global".
//  4. lane = Lanes.Create(ProjectID, Branch, IssueBranchName(alias, title, auOnly), IssueID);
//     base = Workspace.Base(lane).    undo: Lanes.Remove(lane).
//  5. sid = Sessions.Create(SessionSpec{lane.ID, key, first serves brief key, Harness, title}).
//     undo: Sessions.Stop(sid).
//  6. Facts.LaneBound(lane): best effort, as task 22 defines it; never undoes 1-5.
//  7. record started{lane, IssueID, iw.Head.State, base} under sid and the Result under RequestID.
func (s *Starter) Start(ctx context.Context, req Request) (Result, error)
```

```go
// workstart/undo.go

// Undo, in this order:
//  1. bySession[sid] missing → ErrNotFound.
//  2. Workspace.Written(lane, base) true → ErrFirstWrite; nothing changes. An error reading it
//     is returned as is: undo never guesses that nothing was written.
//  3. Sessions.Stop(sid); Lanes.Remove(lane) (ErrWouldLose here means a write landed between 2
//     and 3: returned as ErrFirstWrite, the session already stopped, the lane kept).
//  4. head = Issues.Issue(issue); head.State == started → MoveIssueState(prior, cause
//     start_undone) and stateRestored = prior; otherwise the state is left alone, restored "".
//  5. delete the record. There is no clock anywhere in this function.
func (s *Starter) Undo(ctx context.Context, sessionID string) (stateRestored hubapi.IssueState, err error)

// Discard is Stop and discard: Sessions.Stop(sid) on the first call, then Lanes.Discard(lane,
// confirm). confirm "" or stale → *ConfirmNeeded, and the lane stays. Removed → step 4 of Undo
// runs with the same rule, and the record is deleted. Discard is allowed before the first write
// too; it then needs no token, because NothingToLose holds and P3-11 removes without one.
func (s *Starter) Discard(ctx context.Context, sessionID, confirm string) (stateRestored hubapi.IssueState, err error)
```

The `Starter` never reads the clock. The six seconds of `--undo-window` (task 26) belong to the
desktop's toast; this side answers the same way at second 5 and at minute 5, and a test drives it
with no sleep at all.

```go
// api/workstart.go — the routes and the adapters. Only the shapes that are not obvious are shown.

func (a *API) registerWorkStart(mux *routes) {
	mux.Handle("POST /v1/work/start", http.HandlerFunc(a.handleWorkStart))
	mux.Handle("POST /v1/work/start/{sessionId}/undo", http.HandlerFunc(a.handleWorkUndo))
	mux.Handle("POST /v1/work/start/{sessionId}/discard", http.HandlerFunc(a.handleWorkDiscard))
}

// workStartBody is the strict request; httpx.DecodeJSON refuses unknown fields.
type workStartBody struct {
	ProjectID string `json:"projectId"`
	IssueID   string `json:"issueId"`
	Branch    string `json:"branch"`
	Accept    bool   `json:"accept"`
	Route     string `json:"route"`
	Harness   string `json:"harness"`
	RequestID string `json:"requestId"`
}

var requestIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// apiSessions calls the built handlers in process, so Start gets every check phase 2 and task
// 19 wrote (the worktree resolve, the harness, the Materialiser, the IssueRef stamp) and no copy.
type apiSessions struct{ a *API }

func (s apiSessions) Create(ctx context.Context, sp workstart.SessionSpec) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"machineId": LocalMachineID, "worktreeId": sp.WorktreeID, "harness": sp.Harness,
		"title": sp.Title, "contextRef": sp.ContextRef, "issueKey": sp.IssueKey,
	})
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/sessions", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := newCapture()
	s.a.handleSessionCreate(w, r)
	if w.status != http.StatusCreated {
		return "", &inProcessProblem{status: w.status, body: w.body.Bytes()}
	}
	var out struct {
		ID string `json:"id"`
	}
	return out.ID, json.Unmarshal(w.body.Bytes(), &out)
}

func (s apiSessions) Stop(ctx context.Context, id string) error {
	r := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/sessions/"+id, nil)
	r.SetPathValue("id", id)
	w := newCapture()
	s.a.handleSessionDelete(w, r)
	if w.status >= 300 && w.status != http.StatusNotFound { // gone already is stopped
		return &inProcessProblem{status: w.status, body: w.body.Bytes()}
	}
	return nil
}
```

`httptest.NewRequestWithContext` is in the standard library's `net/http/httptest` (Go 1.23), and
using it outside a test is legal; if review prefers, `http.NewRequestWithContext` plus
`RequestURI` works the same. `newCapture` is a 15-line `http.ResponseWriter` recording status,
headers and body; `inProcessProblem` carries the handler's problem body so the Start handler
writes **the same** status and body back (a harness refusal reads exactly as it does on
`POST /v1/sessions`). `LocalMachineID` is `api.go:45` (`"local"`); the session's JSON id is
`sessionJSON.ID`, tag `"id"` (`sessions.go:594`); `routes.Handle` is `api.go:256-260`; all read in
this worktree at `c5841bf`. The `issueKey` field is task 19's addition to `sessionCreateRequest`
(**UNVERIFIED until task 19 lands**).

```go
// api/workstart.go, continued — the lane adapters over phase 3 (names UNVERIFIED until merge).

// apiLanes: Create resolves projectID through Deps.Projects (the enumerator) and calls
// GitDeps.Creator.Create(ctx, lanes.CreateRequest{Project: p, Branch: branch, Name: name,
// IssueID: issueID}). Remove runs converge.Inspect with no live sessions and, only when
// st.NothingToLose(), lanes.Remove; otherwise workstart.ErrWouldLose. Discard runs Inspect and
// compares confirm with st.Confirm: equal, or NothingToLose → Remove; else *ConfirmNeeded{Paths:
// st's paths, Confirm: st.Confirm}. After a removal it clears the ready flag and forgets the
// worktree's session records and conversation store, exactly as P3-11's DELETE handler does
// (P3 task-11.md:205-209 lists the three), by calling the same unexported helpers.

// apiWorkspace: Base and Written go through GitDeps.LaneGit.For(ctx, lane) and Runner.Status;
// Base is StatusResponse.Head.Sha; Written is len(PorcelainV2Z) > 0 || Head.Sha != base. The
// release func is called exactly once on every path. No host git runs in the lane (R4.2).
```

`Status` reports the status as `PorcelainV2Z` (porcelain **v2**, `-z`: P3 `task-24.md:144-147`),
not v1 as the scope wrote. Only emptiness is read, so the version does not matter to the rule, and
this part never parses a status line.

```go
// api/api.go — in type Deps, after task 16's Work
	// WorkStart is Start from an issue (task 24). Nil when Work or git is not configured: the three
	// /v1/work/start routes then answer hub_not_configured or git_not_ready. Never required by New.
	WorkStart *workstart.Starter

// in New, after a.registerWork(guarded)
	a.registerWorkStart(guarded)
```

`WorkStart` needs the adapters, which need the `*API`, so `cmd/api` cannot build it before `New`.
`New` therefore builds it: when `d.Work != nil && d.Git != nil`, `a.deps.WorkStart = &workstart.
Starter{Issues: d.Work, Class: d.WorkDataClass, Lanes: apiLanes{a}, Sessions: apiSessions{a},
Workspace: apiWorkspace{a}, Facts: d.Git.IssueFacts}`, and a `Deps.WorkStart` passed in (a test's
fake) wins. `Deps.WorkDataClass workstart.DataClass` is the one input `cmd/api` supplies; it is
nil in W1, because no project record carries a class yet, so **every project routes AU and gets a
slug-free branch name** until the class's home lands (the fail-safe, stated again below). Assign
`Facts` only when `d.Git.IssueFacts != nil`: a nil `*IssueFacts` in a non-nil interface would
panic in `LaneBound`.

- [ ] **Step 1: Write the failing `workstart` tests**

The rig is all fakes: `hubclient.NewFake().As(dev)` for `Issues` (an issue `EP-142` in `todo`
at revision 3, serving `brief:bf-…@5`/`c1`), and recording fakes for `LaneMaker`, `Sessions`,
`Workspace` and `Facts`, each appending to one shared `calls []string` so order is asserted.

```go
func TestStartPinsTheRevision(t *testing.T) {
	rg := newStartRig(t)
	res, err := rg.s.Start(ctx, rg.req())
	if err != nil {
		t.Fatal(err)
	}
	rg.writeRevision(t, 4) // a teammate edits the issue after Start
	if res.IssueKey != "issue:"+rg.issue+"@3" || rg.sessions.last.IssueKey != res.IssueKey {
		t.Fatalf("key %q, session told %q; want @3 in both", res.IssueKey, rg.sessions.last.IssueKey)
	}
	if n := rg.hubCalls("Issue"); n != 1 {
		t.Fatalf("Start read the issue %d times; it must read it once and pin", n)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStartBindsLaneSessionAndIssue` | `calls` is exactly `Issue`, `Create lane(name ep-142-…, issue is-…)`, `Base`, `Create session(issueKey @3, contextRef brief:bf-…@5)`, `LaneBound` | a session without `IssueRef`; a lane without `IssueID`; `started` before the session exists |
| `TestStartPinsTheRevision` | the key and the session both say `@3` after the Hub moves to `@4`; one `Issue` read | re-reading latest |
| `TestStartAUOnlyForcesAUAndNoSlug` | `Class` says AU only, `Route: "global"` → `route: "au"`, `routeForced: true`, lane name `ep-142` | the person's choice overriding the class; a slug on an AU project |
| `TestStartUnknownClassFailsSafe` | `Class` returns an error, and separately `Class == nil` → `au`, `ep-142` | a Global default for an unreadable class |
| `TestStartFromTriageAcceptsFirst` | issue in `triage`, `Accept: false` → `ErrNotStartable`, no lane; `Accept: true` → a `todo` move with cause `hand` **before** the lane is created | starting an unaccepted issue; accepting after the agent runs |
| `TestStartRollsBackOnSessionFailure` | `Sessions.Create` fails → `calls` ends `Remove lane`, then the move back to `triage` (for an accepted issue); no record kept; the error is the session's | a half-made Start |
| `TestStartIsIdempotentByRequestID` | the same `RequestID` twice → one lane, one session, equal results | a double press making two agents |
| `TestStartRefusesAnotherProjectsIssue` | the issue's `ProjectID` differs → `ErrIssueElsewhere`, no lane | starting across projects |

Run: `go test ./control-plane/internal/workstart/... -race -count=1 -run 'Start'`
Expected: FAIL, `no Go files` / `undefined: workstart.Starter`. Write `start.go`; re-run. Expected: `ok`.

- [ ] **Step 2: Write the failing undo tests**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestUndoBeforeFirstWriteRemovesAll` | `Written` false → `calls` ends `Stop session`, `Remove lane`, `Issue`, move to `todo` with cause `start_undone`; `stateRestored == "todo"`; the record is gone (a second Undo → `ErrNotFound`) | a clone or session left behind |
| `TestUndoAfterFirstWriteRefused` | `Written` true → `ErrFirstWrite`; no `Stop`, no `Remove`, no move | an undo that throws work away |
| `TestUndoHoldsPastSixSeconds` | the rig's clock is never read (the `Starter` has none); Undo after the fake `Workspace` reports "no write" at a simulated minute 5 succeeds | a timer-based undo |
| `TestUndoLeavesAMovedStateAlone` | after Start a teammate moves the issue to `in_review`; Undo → session and lane removed, no move, `stateRestored == ""` | overwriting a state someone else set |
| `TestUndoRaceWithAWriteKeepsTheLane` | `Written` false, then `Remove` returns `ErrWouldLose` → `ErrFirstWrite`, the lane kept, the session stopped | removing a lane that just gained work |
| `TestDiscardNeedsConfirm` | `confirm: ""` → `*ConfirmNeeded` with the paths and a token, lane kept; a stale token → `*ConfirmNeeded` again; the shown token → removed and `todo` restored | a one-step discard |

Run: `go test ./control-plane/internal/workstart/... -race -count=1 -run 'Undo|Discard'`
Expected: FAIL, `undefined: (*workstart.Starter).Undo`. Write `undo.go`; re-run. Expected: `ok`.

- [ ] **Step 3: Write the failing wire tests, then the handlers**

`control-plane/internal/api/workstart_test.go`, on the phase 2 API rig with `Deps.WorkStart` a
`Starter` over fakes (the adapters are unit-tested against P3's rigs in the same file):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorkStartWire` | a valid body → 201 and exactly the seven SW.10 keys; `undo == "undo_start"` | a renamed or missing field the desktop reads |
| `TestWorkStartRejectsUnknownFields` | `"state":"started"` in the body → 400 naming the field; nothing created | a client choosing the state |
| `TestWorkStartValidates` | a missing `requestId`, `requestId` of 7 characters, `route: "eu"`, `issueId: "EP-142"` → 400 each, `details.field` naming it | a lenient decoder |
| `TestWorkStartWithoutHub` | `Deps.Work` nil → 503 `hub_not_configured` on all three routes | a nil dereference with no Hub (`OD-20`) |
| `TestWorkStartSessionRefusalReadsTheSame` | the in-process session create answers 400 `unknown_harness` → Start answers 400 `unknown_harness` with the same body, and the lane is gone | a Start that rewrites the session's refusal, or leaves its lane |
| `TestWorkUndoCodes` | undo on an unknown session → 404 `start_not_found`; after a write → 409 `first_write` with `details.next == "discard"`; before → 200 `{"undone":true,"stateRestored":"todo","cost":null}` | the wrong code, or an estimated cost |
| `TestApiWorkspaceReadsTheRunnerNotThePTY` | the adapter's `Written` over P3-24's fake runner: `PorcelainV2Z` empty and the same head → false; a byte of status → true; a moved head → true; the fake PTY stream is never read | a first-write signal taken from agent output (`OD-8`) |

And in `routes_audit_test.go`:

```go
func TestWorkStartRoutesArePosts(t *testing.T) {
	want := []string{"POST /v1/work/start", "POST /v1/work/start/{sessionId}/undo",
		"POST /v1/work/start/{sessionId}/discard"}
	got := registeredPatterns(t) // the recorder's list, as TestNoRegisteredGetCanChangeState reads it
	for _, p := range want {
		if !slices.Contains(got, p) {
			t.Errorf("%q is not registered", p)
		}
	}
	for _, p := range got {
		if strings.HasPrefix(p, "GET /v1/work/start") {
			t.Errorf("%q: Start and its undo change state; they are never a GET (contract §10a)", p)
		}
	}
}
```

`registeredPatterns` stands for however `TestNoRegisteredGetCanChangeState` (`routes_audit_test.go:133`)
already collects the patterns; reuse that, do not add a second collector.

Run: `go test ./control-plane/internal/api/... -race -count=1 -run 'WorkStart|WorkUndo|ApiWorkspace'`
Expected: FAIL, `undefined: workstart`. Write `api/workstart.go`, the `Deps` field, the
registration and the `New` wiring; re-run. Expected: `ok`.

- [ ] **Step 4: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/workstart/... ./control-plane/internal/api/... ./control-plane/cmd/api/... -race -count=1'
```

Expected: one `ok` per package, no `FAIL`. `git diff -- go.work control-plane/go.mod` is empty.
Commit with `git commit -F <msgfile> -- <the paths above>`.

#### What this task does not claim

- **Not the desktop.** The Start chip, `Shift+R`, the toast with *Undo* and the 10-second first
  sign are task 61's; *Delegate* is W3 (task 102); cloud sessions are phase 6 stream A and phase 8
  stream C.
- **Not a correct route per project in W1.** No project record carries a data class yet (its home
  is **UNVERIFIED until merge**), so every project routes `au` with a slug-free name. That is the
  safe failure the scope asks for; it is visible (`routeForced`), and it ends when task 43 of
  Phase T, or whichever task gives the class a home, lands and `Deps.WorkDataClass` is wired.
- **Not *Undo Start* across a restart.** The record is in memory; after a restart undo answers
  `404 start_not_found` and the ordinary session and worktree verbs remain.
- **Not the backward move on the Hub.** `start_undone` is not in task 10's cause list; until SW.4
  gains it, the fake and the real Hub refuse the move back (422), Undo still removes the session
  and the lane, and `stateRestored` is `null`. The test rows above assume the cause exists.
- **Not a cost.** `cost` is `null` in local mode (phase 6's gateway, **UNVERIFIED**).
- Design: 05 §2.1, §3.4 (issue text enters only after its owner adopts it, here by Start), §4.5
  (the Develop row), §4.6. Parity: closes the W1 parts of **L-39** (the delegate is the assignee's
  own agent), **L-175** (Start with the issue in context, local mode first), **L-181** and
  **L-198**; with task 19 **L-191**; with tasks 42 and 61 **L-47**'s *accept and Start*.