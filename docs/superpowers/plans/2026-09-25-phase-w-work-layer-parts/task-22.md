### Task 22: Phase 3 delta: the `started`, `in_review` and `landed` fact moves, written as the person

Stream **WP**, seam **SW.4** (consumed; task 10 defines `StateMove` and its causes), about 3 hours
(an estimate, not a measurement). Depends on task 05 (the Work fake), task 21 (`Lane.IssueID`,
`LandResult.Issues`) and **P3-09** (land) sync-merged. Phase 3's files are **UNVERIFIED until
merge**, cited from its plan at `9935278`; the owner phase wins every conflict (rule H2).

Three of an issue's states are **facts**, and this task is where they come from. `started` is the
fact that a lane exists for the issue. `in_review` is the fact that its person marked the lane
ready to land. `landed` is the fact that a land commit carrying `Panacea-Issue: <id>` is on the
branch. None of them is a button. An agent cannot claim progress (05 §2.1 point 3), and a sync
token cannot write one (task 07's guard, SW.3). They are written here, by the control plane, **as
the person whose act caused them** (05 §3.6).

**"As the person" costs nothing new.** The control plane is per user and on loopback; its one
Hub client is the signed-in person's, holding that person's Entra token from Credential Manager
(hub-contract §9.1). `Deps.Work` (task 16) *is* that client. So a move sent through it is the
person's move, with the person as the event's actor. There is no service token in the control
plane to send it with by mistake, and the part adds none. `actor: null` cannot happen: the Hub
fills the actor from the token, never from the body.

**Facts are monotonic.** A move to a state the issue has already passed is not sent. The order is
`triage < backlog < todo < started < in_review < landed < done`; `cancelled` and `duplicate` take
no fact move at all (a person closed the issue, and a later land does not reopen it: it is
recorded in git, and the Hub's head stays as the person left it). So a lane marked ready again
after its issue landed sends nothing, and a land of an issue already `done` sends nothing.

**Git is the truth; the Hub is told.** The land already happened when `update-ref` returned. A
refused or failed send **never** fails the land, the ready mark or the Start: it is logged
(metadata only: the issue id, the target state, the Hub's code) and put on an in-memory retry
list. A sweep on the lane-sync interval retries the list, and a **rescan** re-derives `landed`
from git: after every land, and once per known branch at start, it reads the last 20 land commits
on the branch (`--first-parent`, the `Panacea-Land` trailer present), and for each issue named
there whose Hub head is still before `landed`, it sends `landed` with that land's commit. So a
lost send heals with no outbox on disk. What it does not heal is named at the end.

**Two corrections to the scope, made against the seam's owner.**

1. **The cause names are task 10's.** The scope for this task said `lane_ready` and `land`. Task
   10's scope, which defines SW.4 and says its list is *"the complete W1 set"*, names them
   `lane_queued` (→ `in_review`) and `land_trailer` (→ `landed`, carrying `landCommit`), with
   `lane_bound` (→ `started`) the same in both. This part uses task 10's. The Go identifiers are
   assumed to be `hubapi.CauseLaneBound`, `hubapi.CauseLaneQueued` and `hubapi.CauseLandTrailer`;
   if task 10 declared other names, use its names, and nothing else here changes.
2. **`LaneBound` has one caller, Start (task 24), not P3-04's route.** The scope put a caller in
   P3-04's `POST /v1/projects/{projectId}/worktrees`. That route builds its `CreateRequest` from a
   body with no issue field, and giving it one would make a second, unguarded way to bind a lane to
   an issue: no revision pin, no data-class route, no *Undo Start*. So P3-04's route is untouched,
   and `lanes.CreateRequest.IssueID` (task 21) is set only by Start, which calls `LaneBound` after
   the session exists.

**What it does not do:** it never sends `done` (task 23), never decides who may write a fact (task
07's rows), and draws nothing (the flash is task 41).

**Files:**
- `control-plane/internal/converge/issuefacts.go` — `IssueMover`, `IssueFacts`, `LaneBound`,
  `LaneReady`, `Landed`, `Sweep`, `Rescan`, `Run` (new)
- `control-plane/internal/converge/issuefacts_test.go` — the rows below, on `hubclient.NewFake()` (new)
- `control-plane/internal/api/catalog.go` ⚑ (built; `handlePatchWorktree` at `catalog.go:33`, its
  `SetWorktreeReady` call at `:414`) — one call after the flag is written (modified)
- `control-plane/internal/api/lands.go` ⚑ (P3 `task-09.md:18`) — one call after `Land` returns (modified)
- `control-plane/internal/api/gitdeps.go` ⚑ (P3-04; `// ── task 09 ──` blocks, P3 `task-09.md:19`) —
  `IssueFacts *converge.IssueFacts` in a `// ── Phase W task 22 ──` block, optional (modified)
- `control-plane/internal/api/issuefacts_test.go` — the handler rows (new)
- `control-plane/cmd/api/main.go` ⚑ — build `IssueFacts` when `Deps.Work` is not nil; start `Run` (modified)

**Interfaces:**
- Produces: `converge.IssueMover`, `converge.IssueFacts` and its methods (below). `LaneBound` is
  consumed by task 24; `Landed` and `Rescan` by task 23 (which reads the same land) and task 71
- Consumes:
  - task 04/16 — `hubclient.Work`'s `Issue(ctx, issueID) (hubapi.IssueWithRevision, error)` and
    `MoveIssueState(ctx, issueID, hubapi.StateMove) (hubapi.IssueHead, error)` (task 16 part,
    `:258`, `:309`); `hubclient.NewFake()`, `(*Fake).As(FakeUser)` (`fake.go:91` on `feat/hub` at
    `9f2fa31`, verified), `SetUnavailable` (`fake.go:114`, verified)
  - task 03 — `hubapi.IssueState` and its nine values; `hubapi.IssueWithRevision`'s head field is
    assumed to be `Head` (task 04 declares the type; **UNVERIFIED**)
  - task 10 — `hubapi.StateMove{To, Cause, Reason, LandCommit, Evidence}` and the three causes
  - task 21 — `lanes.Lane.IssueID`, `converge.LandResult.Issues`, `gitx.TrailerIssue`,
    `gitx.IssueTrailerValues`, `gitx.ValidIssueTrailer`
  - P3-02/07/09 — `gitx.Machine`, `gitx.Call`, `gitx.TrailerLand`, `gitx.TrailerFormat`,
    `gitx.FieldSep`/`RecordSep`, `converge.Syncer.Interval` (P3 `task-07.md:71`), `Lander.Land`
- Amends nothing in the Hub.

#### Interfaces — complete

```go
// converge/issuefacts.go
package converge

// IssueMover is the part of hubclient.Work the fact moves need. hubclient.Work satisfies it,
// so converge imports only hubapi and hubclient (hub-contract.md:78-82).
type IssueMover interface {
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
	MoveIssueState(ctx context.Context, issueID string, m hubapi.StateMove) (hubapi.IssueHead, error)
}

// IssueFacts writes an issue's fact states from lane facts, as the signed-in person: Mover is
// the control plane's one Hub client (Deps.Work), which carries that person's token. It never
// fails the git act that caused a move; a send that fails is retried by Sweep and re-derived by
// Rescan. Safe for concurrent use.
type IssueFacts struct {
	Mover   IssueMover
	Machine gitx.Machine // Rescan's git reads, in the project, Scope Project
	Log     *slog.Logger // metadata only: issue id, target state, cause, the Hub's code
	// SendTimeout bounds one Hub call; zero means 3s. A land's response waits at most this long.
	SendTimeout time.Duration

	mu      sync.Mutex
	pending map[string]pendingMove // issue id → the latest move not yet accepted
}

type pendingMove struct {
	move  hubapi.StateMove
	tries int
	next  time.Time
}

// factOrder ranks the states a fact move can pass. cancelled and duplicate are absent: they take
// no fact move (rank reports ok == false for them).
var factOrder = map[hubapi.IssueState]int{
	hubapi.StateTriage: 0, hubapi.StateBacklog: 0, hubapi.StateTodo: 0,
	hubapi.StateStarted: 1, hubapi.StateInReview: 2, hubapi.StateLanded: 3, hubapi.StateDone: 4,
}

// LaneBound moves the lane's issue to started, cause lane_bound. A lane with no IssueID moves
// nothing. Task 24's Start is the one caller, after the session exists.
func (f *IssueFacts) LaneBound(ctx context.Context, lane lanes.Lane) error

// LaneReady moves the lane's issue to in_review, cause lane_queued. Called by
// handlePatchWorktree when ready is written true; a queue reorder never calls it.
func (f *IssueFacts) LaneReady(ctx context.Context, lane lanes.Lane) error

// Landed moves each issue to landed, cause land_trailer, LandCommit landCommit. issueIDs is
// LandResult.Issues; a value ValidIssueTrailer refuses is skipped before any Hub call.
func (f *IssueFacts) Landed(ctx context.Context, landCommit string, issueIDs []string) error
```

`hubapi.StateTriage` … `hubapi.StateDone` are assumed to be task 03's constant names for the
`IssueState` values; take task 03's if they differ.

```go
// converge/issuefacts.go, continued — the one path every move takes.

// move sends m for issueID unless the head has passed m.To. It returns nil when the move was
// sent and accepted, or when nothing needed sending; any other outcome is logged, queued for
// Sweep, and returned so a caller's test can see it. No caller fails its git act on it.
func (f *IssueFacts) move(ctx context.Context, issueID string, m hubapi.StateMove) error {
	if !gitx.ValidIssueTrailer(issueID) {
		return nil // a forged or empty value: nothing to move, nothing to retry
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), f.timeout())
	defer cancel()
	cur, err := f.Mover.Issue(ctx, issueID)
	if err == nil && !f.ahead(cur.Head.State, m.To) {
		f.forget(issueID, m.To)
		return nil
	}
	if err == nil {
		_, err = f.Mover.MoveIssueState(ctx, issueID, m)
	}
	if err == nil {
		f.forget(issueID, m.To)
		return nil
	}
	if permanent(err) { // 403, 404, 422: the Hub has decided; retrying cannot change it
		f.Log.Warn("issue fact refused", "issue", issueID, "to", m.To, "cause", m.Cause, "code", codeOf(err))
		f.forget(issueID, m.To)
		return err
	}
	f.Log.Warn("issue fact not sent; will retry", "issue", issueID, "to", m.To, "code", codeOf(err))
	f.remember(issueID, m)
	return err
}

// ahead reports whether to is strictly after cur in factOrder. cancelled and duplicate are never
// behind anything, so they take no move.
func (f *IssueFacts) ahead(cur, to hubapi.IssueState) bool {
	c, ok := factOrder[cur]
	return ok && factOrder[to] > c
}

// remember keeps the furthest move per issue: a queued landed replaces a queued in_review.
func (f *IssueFacts) remember(issueID string, m hubapi.StateMove) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending == nil {
		f.pending = map[string]pendingMove{}
	}
	p, ok := f.pending[issueID]
	if ok && factOrder[p.move.To] > factOrder[m.To] {
		return
	}
	f.pending[issueID] = pendingMove{move: m, tries: p.tries + 1, next: time.Now().Add(backoff(p.tries + 1))}
}

// forget drops a pending move for issueID at or before to.
func (f *IssueFacts) forget(issueID string, to hubapi.IssueState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pending[issueID]; ok && factOrder[p.move.To] <= factOrder[to] {
		delete(f.pending, issueID)
	}
}

// backoff is 5s, 10s, 20s … capped at 5 minutes. A pending move is never dropped for age:
// the list is bounded by the issues this person moved since the process started.
func backoff(tries int) time.Duration {
	d := 5 * time.Second << min(tries-1, 6)
	return min(d, 5*time.Minute)
}

func (f *IssueFacts) timeout() time.Duration {
	if f.SendTimeout > 0 {
		return f.SendTimeout
	}
	return 3 * time.Second
}

// permanent reports a *hubclient.Error whose Status is 403, 404 or 422. ErrUnavailable,
// ErrSignedOut, a timeout and every 5xx are retried.
func permanent(err error) bool {
	var he *hubclient.Error
	return errors.As(err, &he) && (he.Status == 403 || he.Status == 404 || he.Status == 422)
}

// codeOf is the Hub's problem code, or a fixed word for the sentinel cases. It never returns
// err.Error(): (*hubclient.Error).Error() includes the problem's message, which may quote a title.
func codeOf(err error) string {
	switch {
	case hubclient.Code(err) != "":
		return hubclient.Code(err)
	case errors.Is(err, hubclient.ErrSignedOut):
		return "signed_out"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, hubclient.ErrUnavailable):
		return "unavailable"
	default:
		return "error"
	}
}
```

`hubclient.Error{Status, Problem, RetryAfter}`, `hubclient.Code`, `ErrUnavailable` and
`ErrSignedOut` are verified at `hubclient.go:111-139` on `feat/hub` at `9f2fa31`. `min` is the Go
1.21 builtin.

```go
// converge/issuefacts.go, continued — the three facts, the sweep and the rescan.

func (f *IssueFacts) LaneBound(ctx context.Context, lane lanes.Lane) error {
	if lane.IssueID == "" {
		return nil
	}
	return f.move(ctx, lane.IssueID, hubapi.StateMove{To: hubapi.StateStarted, Cause: hubapi.CauseLaneBound})
}

func (f *IssueFacts) LaneReady(ctx context.Context, lane lanes.Lane) error {
	if lane.IssueID == "" {
		return nil
	}
	return f.move(ctx, lane.IssueID, hubapi.StateMove{To: hubapi.StateInReview, Cause: hubapi.CauseLaneQueued})
}

func (f *IssueFacts) Landed(ctx context.Context, landCommit string, issueIDs []string) error {
	var errs []error
	for _, id := range issueIDs {
		m := hubapi.StateMove{To: hubapi.StateLanded, Cause: hubapi.CauseLandTrailer, LandCommit: landCommit}
		if err := f.move(ctx, id, m); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Sweep retries every pending move whose backoff has passed, oldest issue id first. Run calls
// it; a test calls it directly.
func (f *IssueFacts) Sweep(ctx context.Context, now time.Time) {
	f.mu.Lock()
	due := make([]string, 0, len(f.pending))
	for id, p := range f.pending {
		if !now.Before(p.next) {
			due = append(due, id)
		}
	}
	f.mu.Unlock()
	slices.Sort(due)
	for _, id := range due {
		f.mu.Lock()
		p, ok := f.pending[id]
		f.mu.Unlock()
		if ok {
			_ = f.move(ctx, id, p.move)
		}
	}
}

// rescanDepth is how many first-parent land commits Rescan reads per branch.
const rescanDepth = 20

// Rescan re-derives landed from git for one branch: the last rescanDepth first-parent commits
// that carry a Panacea-Land trailer, and every valid Panacea-Issue value on each. For each such
// issue whose head is before landed, it sends landed with that land's commit (the newest land
// naming it wins). It runs in the PROJECT, Class Read, Scope Project.
func (f *IssueFacts) Rescan(ctx context.Context, projectPath, branch string) error {
	res, err := f.Machine.Git().Run(ctx, gitx.Call{
		Dir: projectPath,
		Args: []string{"log", "--first-parent", "-n", strconv.Itoa(rescanDepth),
			"--fixed-strings", "--grep=" + gitx.TrailerLand + ":",
			// %x00 and %x1e are git's escapes for FieldSep and RecordSep: an argv cannot hold a
			// literal NUL, so the format names them and the output is split on the bytes.
			"--format=%H%x00" + gitx.TrailerFormat(gitx.TrailerLand) + "%x00" +
				gitx.TrailerFormat(gitx.TrailerIssue) + "%x1e",
			"refs/heads/" + branch, "--"},
		Class: gitx.Read,
		Scope: gitx.Project,
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, rec := range strings.Split(string(res.Stdout), gitx.RecordSep) {
		fields := strings.Split(strings.TrimSpace(rec), gitx.FieldSep)
		if len(fields) != 3 || strings.TrimSpace(fields[1]) == "" {
			continue // no Panacea-Land value: --grep is a prefilter, the trailer decides
		}
		commit := fields[0]
		for _, id := range gitx.IssueTrailerValues([]byte(fields[2])) {
			if !seen[id] { // newest first: the first land naming an issue is its latest
				seen[id] = true
				_ = f.move(ctx, id, hubapi.StateMove{To: hubapi.StateLanded,
					Cause: hubapi.CauseLandTrailer, LandCommit: commit})
			}
		}
	}
	return nil
}

// Run sweeps every interval (the lane-sync interval, P3 task-07.md:71) until ctx ends. Before
// the first tick it rescans each (project path, branch) the caller lists once.
func (f *IssueFacts) Run(ctx context.Context, every time.Duration, branches func() []ProjectBranch) {
	for _, b := range branches() {
		_ = f.Rescan(ctx, b.ProjectPath, b.Branch)
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			f.Sweep(ctx, now)
		}
	}
}

// ProjectBranch is one branch Run rescans at start: every distinct (project, branch) in the
// lane index, resolved to the project's path by cmd/api through the enumerator.
type ProjectBranch struct{ ProjectPath, Branch string }
```

`branch` reaches git as `refs/heads/<branch>`, so it can never be read as an option; P3-09's
handler has already resolved it from a `branchId` against the project (it is never a client
string). A branch name holding a newline cannot exist in git.

```go
// api/gitdeps.go — a Phase W block in GitDeps, beside phase 3's.
	// ── Phase W task 22 ──
	// IssueFacts writes started/in_review/landed as the signed-in person. Nil when Deps.Work is
	// nil (no Hub, OD-20): every caller below then does nothing. Never required by New.
	IssueFacts *converge.IssueFacts

// api/catalog.go — in handlePatchWorktree, after SetWorktreeReady succeeds (catalog.go:414)
// and before the response is written. Only a true write calls it; false never moves back.
	if ready {
		a.issueReady(r.Context(), worktreeID)
	}

// Appended to catalog.go. issueReady is best effort by construction: it never fails the PATCH.
func (a *API) issueReady(ctx context.Context, worktreeID string) {
	g := a.deps.Git
	if g == nil || g.IssueFacts == nil || g.Lanes == nil {
		return
	}
	if lane, ok := g.Lanes.ByID(worktreeID); ok && lane.IssueID != "" {
		_ = g.IssueFacts.LaneReady(ctx, lane)
	}
}

// api/lands.go — after Lander.Land returns a result and before the 201 is written.
	if g := a.deps.Git; g != nil && g.IssueFacts != nil {
		_ = g.IssueFacts.Landed(r.Context(), res.Merge, res.Issues)
		_ = g.IssueFacts.Rescan(r.Context(), req.ProjectPath, req.Branch)
	}
```

The `a.deps.Git`, `g.Lanes` spellings are P3-04's `GitDeps` as the phase 3 plan writes them, and
`res`/`req` stand for the land handler's `LandResult` and `LandRequest` locals (**UNVERIFIED until
merge**); follow the merged names. `Rescan` after the land costs one `git log`
and at most 20 Hub reads, and it is what heals a `landed` lost on an earlier land of the same
branch.

```go
// cmd/api/main.go — after Deps.Work and GitDeps are built, before serve.
	if deps.Work != nil && git != nil {
		git.IssueFacts = &converge.IssueFacts{Mover: deps.Work, Machine: git.Machine, Log: log}
		go git.IssueFacts.Run(ctx, git.Syncer.Interval, func() []converge.ProjectBranch {
			return knownBranches(git.Lanes, deps.Projects)
		})
	}

// knownBranches is every distinct (project path, branch) among the lane index's lanes, sorted,
// with a project the enumerator no longer knows skipped.
func knownBranches(ls lanes.Store, projects api.Enumerator) []converge.ProjectBranch
```

- [ ] **Step 1: Write the failing `converge` tests**

Create `control-plane/internal/converge/issuefacts_test.go`. The rig: `hubclient.NewFake()`, a
project with key `EP`, one issue `is-…` created through the fake as `lander` (so `created` is the
lander), and `f := &IssueFacts{Mover: fake.As(lander), Machine: <P3-02's gitxtest machine>, Log:
<a slog.Logger writing to a buffer the test reads>}`.

```go
func TestLandMovesEachIssueToLandedAsTheLander(t *testing.T) {
	rig := newFactsRig(t)                // fake, lander, two issues a and b at started
	err := rig.facts.Landed(ctx, rig.merge, []string{rig.a, rig.b})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{rig.a, rig.b} {
		got := rig.head(t, id)
		if got.State != hubapi.StateLanded || got.LandCommit != rig.merge {
			t.Fatalf("%s = %s at %q, want landed at %q", id, got.State, got.LandCommit, rig.merge)
		}
		if ev := rig.lastStateEvent(t, id); ev.Actor.ID != rig.lander.ID || ev.Cause != "land_trailer" {
			t.Fatalf("%s: event actor %q cause %q, want the lander and land_trailer", id, ev.Actor.ID, ev.Cause)
		}
	}
}
```

`rig.lastStateEvent` reads the fake's feed for the last `issue.state` event on the issue (task
13's kind, SW.5; the fake's feed is H05's). `IssueHead.LandCommit` is task 10's additive field.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLandMovesEachIssueToLandedAsTheLander` | both issues `landed` at the merge; each event's actor is the lander, cause `land_trailer` | a move written as the system, or with no `landCommit` |
| `TestReadyMovesToInReview` | `LaneReady` on a lane with an issue at `started` → `in_review`, cause `lane_queued`; the api row below proves `PUT …/queue` sends nothing | a move sent on queue reorder rather than on ready |
| `TestBindWithoutIssueMovesNothing` | `LaneBound`, `LaneReady` on a lane with `IssueID == ""` → the fake's `Calls()` holds no `Issue` and no `MoveIssueState` | a move, or a Hub read, on every lane |
| `TestMovesNeverGoBack` | issue at `landed`; `LaneReady` then `LaneBound` → `Calls()` holds two `Issue` reads and no `MoveIssueState`; the head is still `landed`. The same for an issue at `cancelled` and at `duplicate` under `Landed` | a regression to `in_review`; a land that reopens a cancelled issue |
| `TestLandSucceedsWhenTheHubRefuses` | `fake.SetUnavailable(true)`; `Landed` returns an error wrapping `hubclient.ErrUnavailable` and the issue is pending; the api row below proves the land's 201 body is byte-identical to the Hub-up case | a land gated on the Hub |
| `TestSweepResendsALostLanded` | after the failed send, `SetUnavailable(false)`; `Sweep(ctx, now)` before the backoff → no call; `Sweep(ctx, now+6s)` → the issue is `landed` and nothing is pending | a fact lost after one failed send; a retry with no backoff |
| `TestRescanHealsAfterARestart` | a branch holding a real land commit (P3-09's rig) with `Panacea-Issue: <a>`; a **new** `IssueFacts` (empty pending list) runs `Rescan` → `a` is `landed` at that commit; a second `Rescan` sends no move | a heal that needs the in-memory list, so a restart loses the fact |
| `TestLandedIgnoresInvalidTrailers` | `Landed(ctx, m, []string{"--octopus", "EP-142", "issue:is-…@3", ""})` → `Calls()` is empty | a move, or a Hub read, for a forged value |
| `TestFactLogsCarryNoText` | an issue titled `Secret merger codename`; a refused move (422 from the fake) → the log buffer holds the issue id, `to`, `cause` and the code, and not the title | a log line built from `err.Error()` |

Run: `go test ./control-plane/internal/converge/... -race -count=1 -run 'Issue|Land|Ready|Bind|Moves|Sweep|Rescan|Fact'`
Expected: FAIL, `undefined: converge.IssueFacts`.

- [ ] **Step 2: Implement `issuefacts.go`**

Write the file from the interface blocks above. Re-run step 1's command. Expected: `ok`.

- [ ] **Step 3: Write the failing handler tests**

Create `control-plane/internal/api/issuefacts_test.go` on P3-04's API rig with `GitDeps.IssueFacts`
over a fake `As(person)`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPatchReadyMovesTheLanesIssue` | `PATCH …/worktrees/{id}` `{"ready":true}` on a lane with an issue → 200 and the issue is `in_review`; `{"ready":false}` → no move | the call missing, or a move on unready |
| `TestQueueReorderMovesNothing` | two ready lanes with issues at `in_review`; `PUT …/queue` reversing them → no `MoveIssueState` in `Calls()` | a move hooked on the queue |
| `TestLandBodyIsTheSameWhenTheHubIsDown` | the same land twice on twin rigs, one with the fake unavailable → both 201, equal bodies except `merge`/`before` shas | a land gated on, or reshaped by, the Hub |
| `TestNoHubMeansNoMoves` | `IssueFacts == nil` → PATCH and land behave exactly as phase 3's own tests pin | a nil dereference with no Hub (`OD-20`) |

Run: `go test ./control-plane/internal/api/... -race -count=1 -run 'Ready|Queue|Land|NoHub'`
Expected: FAIL, `GitDeps has no field or method IssueFacts`. Add the `GitDeps` block, the two calls
and `issueReady`, and the `cmd/api` wiring; re-run. Expected: `ok`.

- [ ] **Step 4: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/converge/... ./control-plane/internal/api/... ./control-plane/cmd/api/... -race -count=1'
```

Expected: one `ok` per package, no `FAIL`. `git diff -- go.work control-plane/go.mod` is empty.
Commit with `git commit -F <msgfile> -- <the paths above>`.

#### What this task does not claim

- **Not `done`.** Task 23 evaluates and sends it; this task never sends `to: done`.
- **Not who may write a fact.** Task 07's rows (SW.3) refuse a sync or integration token; this
  task only ever uses the person's client, and its tests do not re-prove the Hub's refusal.
- **Not a heal for every loss.** A `landed` lost while the control plane ran heals on the next
  sweep; one lost across a restart heals at start **only if** its branch still has a lane in the
  index, or on the branch's next land, and only within its last 20 first-parent lands. A branch
  with no lane left and no further land keeps the Hub's earlier state until someone lands there
  again. An outbox on disk would close that gap; `ADR-H1` forbids keeping Hub data on the desktop,
  and this plan does not add one to the control plane either.
- **Not the flash** (task 41) and not the Desk row (tasks 62-65).
- **Not that a phase 3 name is as cited**: `GitDeps`, `lands.go` and the Syncer are from the plan at
  `9935278`, **UNVERIFIED until merge**.
- Design: 05 §3.3 (the state table), §3.6 (written as the person), §2.1 point 3; 04 §7.3. Parity:
  closes the W1 parts of **L-8** (fact states set by facts) and **L-201** (states follow lane
  facts), and with task 21 **L-146** and **L-200**.
