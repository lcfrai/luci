### Task 135: phase 6 delta: the session token carries the issue id; spend per issue and per loop

W4, stream WH (rule H2: a delta on another phase's files); no seam. Depends on task 24 (SW.10,
`workstart.Start`, `task-24.md:132-191`) and task 19 (`Session.IssueRef`, `task-19.md:103-131`),
and on phase 6's tasks 03, 07, 09 and 15 (`.claude/worktrees/phase-6/docs/superpowers/plans/
2026-09-25-phase-6-gateway-parts/task-03.md`, `task-07.md`, `task-09.md`, `task-15.md`), all
**UNVERIFIED until merge**: phase 6 is a plan, and none of its files are on `feat/work-layer`.
Estimate 0.25 (2 h).

After this task every gateway key minted for a session that is working an issue carries that
issue's id, spend is summed per issue and per loop beside phase 6's person, project and team, and
the issue page says what the issue has cost this month.

**What is on the tree today (verified 2026-09-26 against the phase 6 worktree's plan).**
`wire.MintRequest` has eight fields and no work reference (`task-03.md:414-423`); `wire.Binding`
likewise (`:502-516`); the three scopes are `person`, `project`, `team` (`:526-530`).
keysmith's LiteLLM `metadata` is exactly `{tid, oid, team, project, agent, session, route}`
(`task-07.md:1044-1045`), and `Refresh` re-mints from the previous binding's subject
(`:877-900`). `usage.KeyRef` and `usage.Record` carry no issue (`task-09.md:522-541`), the column
map is `person → user_id`, `project → project_id`, `team → team_id` (`:563`),
`keysmith.key_spend` is keyed on `key_hash` (`:472-482`), and `Reader.Rows` returns exactly one
row, the current UTC month's (`:746-770`). `gatewayclient.Launcher.Start(sessionID, projectID,
agentID string, generation int, pod netip.Addr)` mints with `SessionClass: "solo"`
(`task-15.md:1188-1197`). Task 21's `api.UsageReader` is `Usage(ctx, scope, id)`
(`task-21.md:523-527`). Issue ids are `is-` + 16 lowercase hex and loop ids `lp-`, both minted
with `ids.Wide` (`task-01.md:117-119`).

**Decisions made here, so nobody re-derives them:**

1. **Ids only, never words (§3.6).** The key carries `issue` and `loop` as ids. `WorkRef.Validate`
   accepts `""` or `^is-[0-9a-f]{16}$` for the issue and `""` or `^lp-[0-9a-f]{16}$` for the loop,
   and nothing else, so a title, an alias (`EP-142`) or a key with a revision (`issue:is-…@3`)
   fails at mint. The loop pattern is the issue's with W5's prefix; that `ids.Wide` makes 16 hex
   for every prefix is read from `task-01.md:117` and the issue pattern, not from a loop fixture.
2. **The issue is the one the Materialiser wrote, not the one Start asked for.** Task 19 stamps
   `Session.IssueRef` from what was rendered (`task-19.md:13-14`). The session create reads the id
   out of that key (`issueIDOf("issue:is-0123456789abcdef@4") == "is-0123456789abcdef"`) and passes
   it to the launcher. So Start (task 24) needs no new field: its `SessionSpec.IssueKey`
   (`task-24.md:187`) already reaches the Materialiser, and a session whose issue section was
   refused (`issue_not_adoptable`) never starts, so it never mints.
3. **Metadata keys appear only when set.** A key minted with no issue has phase 6's seven
   metadata keys exactly, so `task-07.md:340`'s exact-map test passes unchanged; a key with an
   issue has `issue` as an eighth, a loop adds `loop`.
4. **A refresh carries the binding now, not the binding at mint.** `RefreshRequest` gains `Work
   *WorkRef`. `nil` keeps the previous binding's ids (phase 6's callers, unchanged); a value
   replaces them, and an empty `IssueID` drops the issue. The launcher's refresh loop always
   sends a value, read through `WorkOf(sessionID)` from the registry at each refresh. So an issue
   unbound mid-session leaves the key at the next generation (at most one TTL, an hour, later),
   and spend already made stays with the issue because `key_spend` keeps a key's first
   attribution (`task-09.md:585-597`).
5. **The issue and loop scopes have no budget.** None is designed (05 §2.4). `Reader.Rows` answers
   `BudgetUSD: 0` for them without calling `Limits`, and the desktop never shows a budget.
6. **Spend is summed over keys, never taken from the project.** The issue scope's `Sum` is
   `where issue_id = $2`; a key with no issue (`''`) is never in any issue's sum, and a project's
   sessions on other issues are not in this one's.
7. **The desktop's read is the control plane's, and it checks the issue first.** New local route
   `GET /v1/work/issues/{issueId}/spend`. The handler reads the issue through `Deps.Work.Issue`
   (task 16); a 404 there is `404 issue_not_found`, so an id guessed from another team reads
   nothing. Then it calls `Usage(ctx, "issue", id)`. Gateway off, keysmith unreachable or a
   non-200 is `503 spend_unavailable`, and the desktop draws **no line**, never *$0.00*.
8. **"This month" is phase 6's UTC month.** `budget.PeriodStart` is UTC (`task-09.md:3`). The
   words say *this month*; the `<time>` element's `title` says *"since 1 Oct, 00:00 UTC"*, so a
   Sydney reader on the morning of the 1st is not misled about which month is meant.
9. **Words, not a number with no unit.** *"Spend on this issue: $4.20 this month, at the provider's
   rate"*, US dollars to the cent in `tabular-nums`; zero reads *"Spend on this issue: none this
   month"*. No share of the project, no rate per day, no projection (OD-27).

**Files:**

- `control-plane/internal/inference/wire/wire.go` ⚑ (P6 task 03) — `WorkRef`, its `Validate`;
  `IssueID`, `LoopID` on `MintRequest` and `Binding`; `Work *WorkRef` on `RefreshRequest`;
  `ScopeIssue`, `ScopeLoop` (modified)
- `control-plane/internal/inference/wire/wire_test.go` ⚑ — the id table (modified)
- `control-plane/internal/keysmith/keysmith.go` ⚑ (P6 task 07) — `subject.work`, the two metadata
  keys, `Refresh`'s rebind (modified); `keysmith_test.go` ⚑ (modified)
- `control-plane/internal/binding/migrations/0003_key_spend_issue.sql` (new) — after P6's `0002`
- `control-plane/internal/binding/pgstore.go`, `memstore.go` ⚑ (P6 task 05) — write and read the
  two columns on `keysmith.bindings` (modified; the scope names neither, gap 1)
- `control-plane/internal/usage/usage.go` ⚑ (P6 task 09) — `KeyRef`, `Record`, the column map,
  `recentSQL`, `MemStore.Sum`, `Rows` for the two new scopes (modified); `sweep.go` ⚑ copies the
  ids from `KeyRef` to `Record` (modified); `usage_test.go` ⚑ (modified)
- `control-plane/internal/gatewayclient/refresh.go` ⚑ (P6 task 15) — `Launcher.Start` takes a
  `wire.WorkRef`; `Launcher.WorkOf`; the refresh loop sends `Work` (modified); `client_test.go` ⚑
- `control-plane/internal/api/sessions.go` ⚑ (P6 task 15's create and restart) — `issueIDOf`,
  and `Session.IssueRef` into `Launcher.Start` (modified)
- `control-plane/internal/api/workspend.go`, `workspend_test.go` (new) — `GET
  /v1/work/issues/{issueId}/spend`
- `control-plane/internal/api/api.go` ⚑ — `a.registerWorkSpend(guarded)` beside task 24's
  `a.registerWorkStart(guarded)` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — one `catalogReadOnlyRoutes` line
  (`routes_audit_test.go:32`) (modified)
- `control-plane/internal/workstart/start_test.go` ⚑ (task 24) — one row: Start's session carries
  the issue into the mint (modified; `start.go` itself does not change, decision 2)
- `desktop/src/api/work.ts` ⚑ (task 37) — one `WORK_LOCAL` row, `IssueSpend`, `issueSpend`
  (modified)
- `desktop/src/work/spend.ts`, `spend.test.ts` (new) — `spendLine`, pure
- `desktop/src/components/issue/IssueHead.tsx` ⚑ (task 46) — the spend fact, and the host's one
  read in `IssueView.tsx` ⚑ (modified); `IssueHead.test.tsx` ⚑ (modified)

**Produces:** `wire.WorkRef`; the `issue` and `loop` usage scopes on keysmith's `GET /v1/usage`;
`GET /v1/work/issues/{issueId}/spend` → `IssueSpend`; the problem code `spend_unavailable`.
**Consumes:** P6's `MintRequest`, `Binding`, `RefreshRequest`, `UsageRow`, `usage.Reader`,
`gatewayclient.Launcher`, `api.GatewayDeps.Usage` (`task-21.md:533-534`); task 19's
`Session.IssueRef`; task 16's `Deps.Work`.

#### Interfaces — complete: `control-plane/internal/inference/wire/wire.go` ⚑

```go
// The two work scopes (W task 135). They have no budget (05 §2.4): Rows answers
// BudgetUSD 0 for them and task 06's budget.Dimension never names them.
const (
	ScopeIssue = "issue"
	ScopeLoop  = "loop"
)

var (
	issueIDRe = regexp.MustCompile(`^is-[0-9a-f]{16}$`)
	loopIDRe  = regexp.MustCompile(`^lp-[0-9a-f]{16}$`)
)

// WorkRef is the work a session's key is spent on: ids only, never a title
// (05 §3.6). Both are optional; "" means none.
type WorkRef struct {
	IssueID string `json:"issueId,omitempty"`
	LoopID  string `json:"loopId,omitempty"`
}

// Validate refuses anything but an empty id or a well-formed one, so free text
// (a title, an alias, a revisioned key) never reaches LiteLLM's metadata.
func (w WorkRef) Validate() error {
	if w.IssueID != "" && !issueIDRe.MatchString(w.IssueID) {
		return fmt.Errorf("issueId %q is not an issue id (is- and 16 hex)", w.IssueID)
	}
	if w.LoopID != "" && !loopIDRe.MatchString(w.LoopID) {
		return fmt.Errorf("loopId %q is not a loop id (lp- and 16 hex)", w.LoopID)
	}
	return nil
}
```

`MintRequest` gains the two fields after `PodAddr`, and `Validate` gains one check before its
address parse. `Binding` gains the same two after `AgentID`; `RefreshRequest` gains `Work`:

```go
	// in MintRequest, after PodAddr
	IssueID string `json:"issueId,omitempty"` // W task 135; "" for no issue
	LoopID  string `json:"loopId,omitempty"`  // W task 135; "" for no loop

	// in MintRequest.Validate, before `_, err := r.Addr()`
	if err := (WorkRef{IssueID: r.IssueID, LoopID: r.LoopID}).Validate(); err != nil {
		return err
	}

	// in Binding, after AgentID (json names equal MintRequest's)
	IssueID string `json:"issueId,omitempty"`
	LoopID  string `json:"loopId,omitempty"`

	// in RefreshRequest, after Generation. nil keeps the previous binding's ids;
	// a value replaces them, and an empty IssueID drops the issue (decision 4).
	Work *WorkRef `json:"work,omitempty"`

	// in RefreshRequest.Validate, before `return nil`
	if r.Work != nil {
		if err := r.Work.Validate(); err != nil {
			return err
		}
	}
```

`regexp` joins the file's imports. `Binding` is what the gatekeeper reads; it ignores the two
fields (task 10 authorises by key hash and pod address only).

#### Interfaces — complete: `control-plane/internal/keysmith/keysmith.go` ⚑

`subject` (`task-07.md:844-848`) gains `work wire.WorkRef`. The two `subject{…}` literals
(`:868`, `:899`) and `mint`'s binding and metadata (`:1037-1046`) change as below; nothing else in
the file does. `mint`'s local `issue := s.bindThenGenerate` (`:1047`) keeps its name: it is the
issuing function, and `sub.work.IssueID` never shadows it.

```go
type subject struct {
	session, team, project, agent string
	owner                         wire.Owner
	pod                           netip.Addr
	work                          wire.WorkRef // W task 135: ids only
}

// in Mint (:868)
	return s.mint(ctx, subject{session: req.SessionID, team: req.TeamID, project: req.ProjectID,
		agent: req.AgentID, owner: req.Owner, pod: pod.Unmap(),
		work: wire.WorkRef{IssueID: req.IssueID, LoopID: req.LoopID}}, req.Generation)

// in Refresh: the first check grows, and the subject (:899) takes the rebind
	if req.SessionID == "" || req.Generation < 2 {
		return wire.MintResponse{}, fmt.Errorf("%w: refresh needs a session and a generation of 2 or more", ErrInvalid)
	}
	if req.Work != nil {
		if err := req.Work.Validate(); err != nil {
			return wire.MintResponse{}, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	}
	// … unchanged to the mint …
	work := wire.WorkRef{IssueID: prev.IssueID, LoopID: prev.LoopID}
	if req.Work != nil {
		work = *req.Work // decision 4: the binding now, and "" drops it
	}
	resp, err := s.mint(ctx, subject{session: prev.SessionID, team: prev.TeamID, project: prev.ProjectID,
		agent: prev.AgentID, owner: prev.Owner, pod: prev.PodAddr, work: work}, req.Generation)

// in mint (:1037-1046)
	b := wire.Binding{SessionID: sub.session, Generation: gen, Owner: sub.owner, TeamID: sub.team,
		ProjectID: sub.project, AgentID: sub.agent, IssueID: sub.work.IssueID, LoopID: sub.work.LoopID,
		PodAddr: sub.pod, RouteAtMint: route, CreatedAt: now, ExpiresAt: now.Add(s.d.TTL)}
	meta := map[string]string{"tid": sub.owner.TID, "oid": sub.owner.OID, "team": sub.team,
		"project": sub.project, "agent": sub.agent, "session": sub.session, "route": string(route)}
	if sub.work.IssueID != "" {
		meta["issue"] = sub.work.IssueID // decision 3: present only when set
	}
	if sub.work.LoopID != "" {
		meta["loop"] = sub.work.LoopID
	}
	req := litellmadmin.GenerateKeyRequest{
		KeyAlias: fmt.Sprintf("sess/%s/%d", sub.session, gen), Duration: duration(s.d.TTL),
		UserID: userID, TeamID: sub.project, Models: s.groupsFor(route), MaxBudget: maxUSD,
		Metadata: meta,
	}
```

The key alias stays `sess/<session>/<gen>`: an alias is shown in LiteLLM's admin UI, and an issue
id there adds nothing the metadata does not already hold.

#### Migration — complete: `control-plane/internal/binding/migrations/0003_key_spend_issue.sql`

Task 05's runner applies it once, after `0002`, and records version 3 (`task-09.md:466`).

```sql
-- W task 135: the work a key was spent on. Ids only; '' means none, so the
-- sums below never meet a NULL and phase 6's rows read as "no issue".
alter table keysmith.bindings
    add column issue_id text not null default ''
        check (issue_id = '' or issue_id ~ '^is-[0-9a-f]{16}$'),
    add column loop_id  text not null default ''
        check (loop_id = '' or loop_id ~ '^lp-[0-9a-f]{16}$');

alter table keysmith.key_spend
    add column issue_id text not null default ''
        check (issue_id = '' or issue_id ~ '^is-[0-9a-f]{16}$'),
    add column loop_id  text not null default ''
        check (loop_id = '' or loop_id ~ '^lp-[0-9a-f]{16}$');

-- Partial: most keys carry no loop, and '' is never summed as an id.
create index key_spend_issue on keysmith.key_spend (period_start, issue_id) where issue_id <> '';
create index key_spend_loop  on keysmith.key_spend (period_start, loop_id)  where loop_id  <> '';
```

The database repeats `WorkRef.Validate`'s patterns on purpose: a store written by some other
path than `Mint` still cannot hold a title in `issue_id`.

**`binding/pgstore.go` and `memstore.go` ⚑.** `Put`'s insert gains `issue_id, loop_id` from
`b.IssueID`, `b.LoopID`; every `select` that builds a `wire.Binding` (by key hash, by session)
reads them back. `MemStore` stores the struct whole and changes only if it copies fields one by
one (**UNVERIFIED** until task 05 merges). Task 05's behaviour suite (`bindingtest.Run`) gains one
row, *"a binding's work round-trips"*, so both stores are held to it.

#### Interfaces — complete: `control-plane/internal/usage/usage.go` ⚑ and `sweep.go` ⚑

```go
// KeyRef (task-09.md:522-528) and Record (:531-541) each gain the two ids.
type KeyRef struct {
	KeyHash, KeyID, SessionID string
	Owner                     wire.Owner
	TeamID, ProjectID         string
	IssueID, LoopID           string // W task 135; "" for none
	CreatedAt                 time.Time
}

// in Record, after ProjectID
	IssueID string // W task 135; the key's first attribution is kept (decision 4)
	LoopID  string

// column (:563) gains the two scopes; it stays the only text spliced into Sum's SQL.
var column = map[string]string{wire.ScopePerson: "user_id", wire.ScopeProject: "project_id",
	wire.ScopeTeam: "team_id", wire.ScopeIssue: "issue_id", wire.ScopeLoop: "loop_id"}

// ErrScope's words grow to name the two new scopes.
var ErrScope = errors.New("usage: the scope is person, project, team, issue or loop, with an id")

// in MemStore.Sum, the per-row value
	v := map[string]string{wire.ScopePerson: r.UserID, wire.ScopeProject: r.ProjectID,
		wire.ScopeTeam: r.TeamID, wire.ScopeIssue: r.IssueID, wire.ScopeLoop: r.LoopID}[scope]

// recentSQL (:656-657) reads the two columns task 135's migration added to bindings.
const recentSQL = `select key_hash, key_id, session_id, owner_tid, owner_oid, team_id, project_id,
issue_id, loop_id, created_at from keysmith.bindings where expires_at > $1`
```

`PGStore.Upsert`'s insert (`:623-628`) gains `issue_id, loop_id`; its `on conflict` clause is
unchanged, so a key's first attribution stands (it updates spend, requests and `updated_at`
only). `RecentKeys` scans the two new columns in order. `Sweeper` (`:869`) copies
`IssueID: k.IssueID, LoopID: k.LoopID` into the `Record` it upserts.

`Record.validate` is unchanged: an empty issue is the normal case, not an error.

```go
// Rows (:746-770): the two work scopes have no budget and skip the Limits read (decision 5).
func (r *Reader) Rows(ctx context.Context, scope, id string) ([]wire.UsageRow, error) {
	if _, ok := column[scope]; !ok || id == "" {
		return nil, ErrScope
	}
	period := budget.PeriodStart(r.now())
	t, err := r.store.Sum(ctx, scope, id, period)
	if err != nil {
		return nil, err
	}
	row := wire.UsageRow{Scope: scope, ID: id, PeriodStart: period, SpendUSD: t.SpendUSD, Requests: t.Requests}
	if scope == wire.ScopeIssue || scope == wire.ScopeLoop {
		return []wire.UsageRow{row}, nil // BudgetUSD 0: none is designed
	}
	var team, project string
	switch scope {
	case wire.ScopeTeam:
		team = id
	case wire.ScopeProject:
		project = id
	}
	l, err := r.limits.Limits(ctx, team, project)
	if err != nil {
		return nil, err
	}
	row.BudgetUSD = map[string]float64{wire.ScopePerson: l.PersonUSD, wire.ScopeProject: l.ProjectUSD,
		wire.ScopeTeam: l.TeamUSD}[scope]
	return []wire.UsageRow{row}, nil
}
```

keysmith's handler for `GET /v1/usage` (task 08, `task-08.md:35`) validates `scope` against
`person`, `project`, `team`; its allow-list gains `issue` and `loop`, and its 400 words name all
five. `Reader.Spent` (task 07's `SpentReader`) is **unchanged**: budgets stay three-dimensional.

#### Interfaces — complete: `gatewayclient/refresh.go` ⚑ and `api/sessions.go` ⚑

```go
// Launcher (task-15.md:1108-1124) gains one field, after Record.
	// WorkOf reads a session's current work from the registry at each refresh,
	// so an issue unbound mid-session leaves the next generation's key (decision 4).
	// nil: the refresh sends no Work, and keysmith keeps the previous binding's.
	WorkOf func(sessionID string) wire.WorkRef

// Start takes the work the session was created with (task-15.md:1188).
func (l *Launcher) Start(sessionID, projectID, agentID string, generation int, pod netip.Addr, work wire.WorkRef)

// in Start's goroutine, the mint (task-15.md:1196-1197)
		resp, err := l.Client.Mint(ctx, wire.MintRequest{SessionID: sessionID, Generation: generation, Owner: l.Owner,
			TeamID: l.TeamID, ProjectID: projectID, AgentID: agentID, SessionClass: "solo", PodAddr: pod.String(),
			IssueID: work.IssueID, LoopID: work.LoopID})

// in refreshLoop (task-15.md:1251)
		req := wire.RefreshRequest{SessionID: sessionID, Generation: gen + 1}
		if l.WorkOf != nil {
			w := l.WorkOf(sessionID)
			req.Work = &w
		}
		resp, err := l.Client.Refresh(ctx, req)
```

`api.GatewayLauncher.Start` (`task-15.md:1483`) takes the same sixth argument. A retried
refresh sends the body it sent the first time: `req` is built once per generation, outside task
15's retry (decision (f)), so a registry change between two retries does not split a generation.

```go
// api/sessions.go ⚑ — the session create's call (task-15.md:1528) and restart's re-mint pass the
// issue the Materialiser wrote (decision 2). The restart call's line is UNVERIFIED until merge.
		a.deps.Gateway.Start(sessionID, projectID, string(harnessID), 1, sb.NetworkAddr,
			wire.WorkRef{IssueID: issueIDOf(sess.IssueRef)}) // returns at once

// issueIDOf is the issue id inside task 19's key, "issue:is-…@<revision>", or "" for
// anything else: an empty ref, an erased ref (task 19's ErasedIssueRef) or a malformed one.
// Never an error: a session with no issue is a normal session, and it mints without one.
func issueIDOf(ref string) string {
	rest, ok := strings.CutPrefix(ref, "issue:")
	if !ok {
		return ""
	}
	id, _, _ := strings.Cut(rest, "@")
	if (wire.WorkRef{IssueID: id}).Validate() != nil {
		return ""
	}
	return id
}

// cmd/api/main.go ⚑ — the launcher's WorkOf, beside task 15's Record (GatewayRecorder).
	launcher.WorkOf = func(sessionID string) wire.WorkRef {
		s, err := store.Get(sessionID)
		if err != nil {
			return wire.WorkRef{} // a session gone or unreadable spends on no issue
		}
		return wire.WorkRef{IssueID: issueIDOf(s.IssueRef)}
	}
```

`issueIDOf` is exported from `api` as `IssueIDOf` for `cmd/api` (one rename; the tests call the
exported name). `registry.Store.Get` is phase 2's read (**UNVERIFIED** name until merge). No
caller sets `LoopID` in this task: W5's loop runner does, when it launches a loop's session (task
172's scope); here it is plumbed and tested, not used.

#### Interfaces — complete: `control-plane/internal/api/workspend.go`

```go
package api

// IssueSpend is GET /v1/work/issues/{issueId}/spend's 200 body: one issue's spend this UTC
// month, at the provider's rate, summed over every key that carried the issue (W task 135).
type IssueSpend struct {
	IssueID     string    `json:"issueId"`
	PeriodStart time.Time `json:"periodStart"` // the UTC month's first instant
	SpendUSD    float64   `json:"spendUsd"`
	Requests    int64     `json:"requests"`
}

func (a *API) registerWorkSpend(mux *routes) {
	mux.Handle("GET /v1/work/issues/{issueId}/spend", http.HandlerFunc(a.handleIssueSpend))
}

// handleIssueSpend reads the issue first, so a guessed id reads nothing (decision 7), then
// the rollup. Every failure after the issue read is 503 spend_unavailable: the desktop draws
// no line rather than a zero it cannot stand behind.
func (a *API) handleIssueSpend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("issueId")
	if (wire.WorkRef{IssueID: id}).Validate() != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, "bad_request", "That is not an issue id.", map[string]string{"issueId": id})
		return
	}
	usage := a.usageReader() // task 21's GatewayDeps.Usage, or nil with the gateway off
	if a.deps.Work == nil || usage == nil {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, "spend_unavailable", "Spend is not available: the gateway is off.", nil)
		return
	}
	if _, err := a.deps.Work.Issue(r.Context(), id); err != nil {
		var p *hubclient.Problem // task 144's reading of the Hub's problem (UNVERIFIED name)
		if errors.As(err, &p) && p.Status == http.StatusNotFound {
			httpx.WriteProblem(w, http.StatusNotFound, "issue_not_found", "No such issue.", map[string]string{"issueId": id})
			return
		}
		httpx.WriteProblem(w, http.StatusServiceUnavailable, "spend_unavailable", "Spend is not available: the issue could not be read.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := usage.Usage(ctx, wire.ScopeIssue, id)
	if err != nil || len(rows) != 1 || rows[0].Scope != wire.ScopeIssue || rows[0].ID != id {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, "spend_unavailable", "Spend is not available: the gateway did not answer.", nil)
		return
	}
	writeJSON(w, http.StatusOK, IssueSpend{IssueID: id, PeriodStart: rows[0].PeriodStart,
		SpendUSD: rows[0].SpendUSD, Requests: rows[0].Requests})
}
```

`a.usageReader()` is one line over task 21's `GatewayDeps.Usage` (`task-21.md:533-534`, the
`Gateway *GatewayDeps` field at `:924`). Task 15 also names a `Deps.Gateway`, of type
`GatewayLauncher` (`task-15.md:1476`); the two P6 parts disagree on that field (gap 2), so this
part reads usage through the helper and never names the field twice. `writeJSON` is phase 2's helper (**UNVERIFIED** name). The route is a `GET` with no side effect;
its `catalogReadOnlyRoutes` line reads: *"Reads one issue through the Hub and one usage row
through keysmith. Nothing is written."*

#### Interfaces — complete: the desktop

```ts
// desktop/src/api/work.ts ⚑ — appended to WORK_LOCAL (task 37, :301-307)
  ["issueSpend", "GET", "/v1/work/issues/{issueId}/spend"],

/** GET /v1/work/issues/{issueId}/spend (W task 135). US dollars, the UTC month's spend. */
export interface IssueSpend {
  issueId: string;
  periodStart: string;   // RFC 3339, the UTC month's first instant
  spendUsd: number;
  requests: number;
}

// in WorkClient
  /** Rejects with ApiError 503 spend_unavailable when the gateway is off or does not answer. */
  issueSpend(issueId: string): Promise<IssueSpend>;

// in makeWorkClient, beside task 37's local members
    issueSpend: (issueId) => local("issueSpend", [issueId]),
```

```ts
// desktop/src/work/spend.ts — pure; no clock, no locale beyond en-AU's own digits.
import type { IssueSpend } from "../api/work";

export interface SpendLine {
  words: string;         // "Spend on this issue: $4.20 this month, at the provider's rate"
  amount: string | null; // "$4.20", the span set in tabular-nums; null for "none"
  since: string;         // "since 1 Oct, 00:00 UTC", the <time> element's title (decision 8)
}

const usd = new Intl.NumberFormat("en-AU", { style: "currency", currency: "USD",
  currencyDisplay: "narrowSymbol", minimumFractionDigits: 2, maximumFractionDigits: 2 });
const month = new Intl.DateTimeFormat("en-AU", { day: "numeric", month: "short", timeZone: "UTC" });

/** null when there is nothing honest to say: no read, or a read for another issue. */
export function spendLine(s: IssueSpend | null, issueId: string): SpendLine | null {
  if (s === null || s.issueId !== issueId || !Number.isFinite(s.spendUsd) || s.spendUsd < 0) return null;
  const since = `since ${month.format(new Date(s.periodStart))}, 00:00 UTC`;
  // Under half a cent rounds to $0.00, which reads as nothing spent when something was.
  if (s.spendUsd === 0) return { words: "Spend on this issue: none this month", amount: null, since };
  const amount = s.spendUsd < 0.005 ? "under $0.01" : usd.format(s.spendUsd);
  return { words: `Spend on this issue: ${amount} this month, at the provider's rate`, amount, since };
}
```

```tsx
// desktop/src/components/issue/IssueHead.tsx ⚑ (task 46, :574-577) — one optional prop and one fact.
//   spend?: IssueSpend | null;     // absent or null: no line (decision 7)
// inside <p className="iv-facts">, after the "opened … by …" fact (:611-613)
        {line && (
          <span className="iv-fact" data-fact="spend">
            {line.amount === null ? line.words : (<>
              Spend on this issue: <span className="iv-num">{line.amount}</span> this month, at the provider's rate
            </>)}{" "}
            <time dateTime={spend!.periodStart} title={line.since} className="iv-since" aria-label={line.since} />
          </span>
        )}
// with, at the top of the body: const line = spendLine(props.spend ?? null, head.id);
// (IssueHead.id is the issue id, task-37.md:122)
```

`.iv-num { font-variant-numeric: tabular-nums; }` joins task 46's stylesheet beside `.iv-fact`
(DESIGN-SYSTEM: live numbers use `tabular-nums`). The empty `<time>` carries the month for the
title and the screen reader only; it draws nothing. **The host** (`IssueView.tsx`, task 46,
`:514-540`) calls `client.issueSpend(issueId)` once when the issue opens and again when the
window regains focus; a rejection of any kind sets `spend` to `null`. There is no timer and no
per-row read: the spend fact is on the issue page only, never on a list row.

#### Tests to write first — Go

Each names the mutant it kills. The keysmith rows use task 07's rig (`newRig`, `mintReq`,
`r.admin.generated`, `task-07.md:92-103`, `:170-222`); the usage rows task 09's `MemStore`.

```go
// keysmith/keysmith_test.go ⚑
const issue = "is-0123456789abcdef"

func TestMintPutsTheIssueInTheKeysMetadata(t *testing.T) { // kills "id dropped at mint"
	r := newRig(t)
	req := mintReq("p-global", 1)
	req.IssueID = issue
	if _, err := r.svc.Mint(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := r.admin.generated[0].Metadata["issue"]; got != issue {
		t.Errorf("metadata issue %q, want %q", got, issue)
	}
	if _, ok := r.admin.generated[0].Metadata["loop"]; ok {
		t.Error("a mint with no loop wrote a loop key") // decision 3
	}
	b := liveBindings(t, r.store, t0)[0]
	if b.IssueID != issue {
		t.Errorf("binding issue %q, want %q", b.IssueID, issue)
	}
}

func TestRefreshCarriesTheIssueAndARebindDropsIt(t *testing.T) { // kills "unbind never reaches the key"
	r := newRig(t)
	req := mintReq("p-global", 1)
	req.IssueID = issue
	mustMint(t, r, req)
	two, _ := r.svc.Refresh(context.Background(), wire.RefreshRequest{SessionID: "s-1", Generation: 2})
	three, _ := r.svc.Refresh(context.Background(), wire.RefreshRequest{SessionID: "s-1", Generation: 3,
		Work: &wire.WorkRef{}})
	if r.admin.generated[1].Metadata["issue"] != issue {
		t.Error("a refresh with no Work lost the issue (decision 4: nil keeps it)")
	}
	if _, ok := r.admin.generated[2].Metadata["issue"]; ok || two.KeyID == three.KeyID {
		t.Error("a refresh with an empty Work kept the issue")
	}
}
```

```go
// inference/wire/wire_test.go ⚑ (package wire, as task 03's tests are) — kills "free text on the token"
func TestWorkRefTakesIdsOnly(t *testing.T) {
	for _, bad := range []WorkRef{
		{IssueID: "Resume upload fails on Safari"}, {IssueID: "EP-142"},
		{IssueID: "issue:is-0123456789abcdef@4"}, {IssueID: "is-0123456789ABCDEF"},
		{IssueID: "is-0123456789abcde"}, {LoopID: "is-0123456789abcdef"}, {LoopID: "nightly triage"},
	} {
		if bad.Validate() == nil {
			t.Errorf("%+v validated", bad)
		}
		req := okMint // task 03's fixture (task-03.md:71-72), copied by value
		req.IssueID, req.LoopID = bad.IssueID, bad.LoopID
		if req.Validate() == nil {
			t.Errorf("a mint carrying %+v validated", bad)
		}
		if (RefreshRequest{SessionID: "s-1", Generation: 2, Work: &bad}).Validate() == nil {
			t.Errorf("a refresh carrying %+v validated", bad)
		}
	}
	for _, good := range []WorkRef{{}, {IssueID: "is-0123456789abcdef"},
		{IssueID: "is-0123456789abcdef", LoopID: "lp-fedcba9876543210"}} {
		if err := good.Validate(); err != nil {
			t.Errorf("%+v: %v", good, err)
		}
	}
}
```

```go
// usage/usage_test.go ⚑
func TestIssueSpendSumsAcrossGenerations(t *testing.T) { // kills "latest generation only"
	st := NewMemStore()
	period := budget.PeriodStart(t0)
	for i, spend := range []float64{1.50, 2.70} { // generations 1 and 2 of one session
		must(t, st.Upsert(ctx, Record{KeyHash: hashOf(fmt.Sprint("g", i)), SessionID: "s-1", UserID: "t:o",
			TeamID: "tm-1", ProjectID: "p-1", IssueID: issue, PeriodStart: period, SpendUSD: spend}))
	}
	rows, _ := NewReader(st, policy, func() time.Time { return t0 }).Rows(ctx, wire.ScopeIssue, issue)
	if math.Abs(rows[0].SpendUSD-4.20) > 1e-9 || rows[0].BudgetUSD != 0 {
		t.Errorf("issue row %+v, want 4.20 spent and no budget", rows[0])
	}
}

func TestIssueScopeIsNotTheProject(t *testing.T) { // kills "project total shown as issue spend"
	st := NewMemStore()
	period := budget.PeriodStart(t0)
	for k, iss := range map[string]string{"a": issue, "b": "is-fedcba9876543210", "c": ""} {
		must(t, st.Upsert(ctx, Record{KeyHash: hashOf(k), SessionID: "s-" + k, UserID: "t:o",
			TeamID: "tm-1", ProjectID: "p-1", IssueID: iss, PeriodStart: period, SpendUSD: 1}))
	}
	r := NewReader(st, policy, func() time.Time { return t0 })
	issueRows, _ := r.Rows(ctx, wire.ScopeIssue, issue)
	projectRows, _ := r.Rows(ctx, wire.ScopeProject, "p-1")
	if issueRows[0].SpendUSD != 1 || projectRows[0].SpendUSD != 3 {
		t.Errorf("issue %v, project %v: want 1 and 3", issueRows[0].SpendUSD, projectRows[0].SpendUSD)
	}
	if _, err := r.Rows(ctx, wire.ScopeIssue, ""); !errors.Is(err, ErrScope) {
		t.Error("the empty issue id was summed: every key with no issue would be one issue")
	}
}
```

`hashOf`, `must`, `ctx`, `policy` and `t0` are task 09's test helpers (`task-09.md:98`, **names
UNVERIFIED** beyond `hashOf`). The sweep row: a `KeyRef` with `IssueID` upserts a `Record` with
the same `IssueID` (kills "the sweep drops the id"). The Postgres store runs the same two rows
through task 09's durable suite, and one more: an `insert` of `issue_id = 'Resume upload'` fails
the table's check (kills "a title in the ledger").

```go
// api/sessions_gateway_test.go ⚑ (P6 task 15's file) — kills "Start forgets the binding"
func TestSessionCreateMintsForTheIssueTheMaterialiserWrote(t *testing.T) {
	gw := &fakeLauncher{}                              // task 15's fake GatewayLauncher, recording Start's args
	h := newGatewayAPI(t, gw, materialiserWrote("issue:is-0123456789abcdef@4")) // task 19's fake result
	createSession(t, h, `{"worktreeId":"w-1","issueKey":"issue:is-0123456789abcdef@4"}`)
	if got := gw.started[0].work; got != (wire.WorkRef{IssueID: "is-0123456789abcdef"}) {
		t.Errorf("Start's work %+v, want the issue", got)
	}
}

func TestIssueIDOfTakesTheIdOrNothing(t *testing.T) {
	for ref, want := range map[string]string{
		"issue:is-0123456789abcdef@4": "is-0123456789abcdef", "issue:is-0123456789abcdef": "is-0123456789abcdef",
		"": "", "brief:bf-0123456789abcdef@2": "", "issue:EP-142@1": "", "issue:@3": "",
	} {
		if got := api.IssueIDOf(ref); got != want {
			t.Errorf("IssueIDOf(%q) = %q, want %q", ref, got, want)
		}
	}
}
```

`workstart/start_test.go` ⚑ gains one row on task 24's fakes: a Start of `is-0123456789abcdef`
creates a session whose `SessionSpec.IssueKey` names that id at the head revision, so the chain
Start → Materialiser → `IssueRef` → mint has a test at each link (kills "Start forgets the binding"
at its own end). `fakeLauncher`, `newGatewayAPI`, `materialiserWrote` and `createSession` are the
names this part gives the helpers; task 15's own are **UNVERIFIED** until merge.

```go
// api/workspend_test.go — kills "a default zero", "another team's spend", "a wrong row passed on"
func TestIssueSpend(t *testing.T) {
	const id = "is-0123456789abcdef"
	row := wire.UsageRow{Scope: "issue", ID: id, PeriodStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), SpendUSD: 4.2}
	for _, c := range []struct {
		name   string
		work   fakeWork            // task 16's fake hubclient.Work
		usage  api.UsageReader     // nil: the gateway is off
		status int
		code   string
	}{
		{"spent", workWith(id), fixedUsage(row, nil), 200, ""},
		{"gateway off", workWith(id), nil, 503, "spend_unavailable"},
		{"keysmith down", workWith(id), fixedUsage(wire.UsageRow{}, errors.New("dial tcp: refused")), 503, "spend_unavailable"},
		{"row for another id", workWith(id), fixedUsage(wire.UsageRow{Scope: "issue", ID: "is-fedcba9876543210"}, nil), 503, "spend_unavailable"},
		{"not readable", workWithout(), fixedUsage(row, nil), 404, "issue_not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := get(t, newSpendAPI(t, c.work, c.usage), "/v1/work/issues/"+id+"/spend")
			if w.Code != c.status || (c.code != "" && problemCode(w) != c.code) {
				t.Fatalf("%d %s, want %d %s", w.Code, w.Body, c.status, c.code)
			}
		})
	}
}
```

The 404 row also asserts the usage fake was **never called** (kills "the rollup read before the
issue check"). A path id that fails `WorkRef.Validate` is `400 bad_request` with no Hub call.

#### Tests to write first — the desktop

- `desktop/src/work/spend.test.ts`: `spendLine({issueId: A, periodStart: "2026-10-01T00:00:00Z",
  spendUsd: 4.2, requests: 9}, A)` is `{words: "Spend on this issue: $4.20 this month, at the
  provider's rate", amount: "$4.20", since: "since 1 Oct, 00:00 UTC"}` (kills "a number with no
  unit" and "a local-time month"); `spendUsd: 0` gives `"… none this month"` and `amount: null`
  (kills "$0.00 for nothing"); `spendUsd: 0.001` gives `"under $0.01"` (kills "a cost that rounds
  to nothing"); `null` and a read for another issue give `null` (kills "a stale read shown on the
  wrong issue").
- `desktop/src/components/issue/IssueHead.test.tsx` ⚑: with `spend` absent, `null`, or a
  rejected read in the host, there is no `[data-fact="spend"]` and no `$0.00` anywhere in the
  head (kills "a default zero"); with a read, the amount's span has class `iv-num` and the
  stylesheet sets `tabular-nums` on it; `expectNoPrediction(container)` (task 20's helper, `desktop/src/test/expectNoPrediction`) passes,
  and no text in the fact matches `/%|of the project|per day|forecast|on track|budget/i` (OD-27,
  decision 5).
- `desktop/src/api/work.test.ts` ⚑ (task 37's route table test): `issueSpend` sends `GET
  /v1/work/issues/is-0123456789abcdef/spend` through `local`, never `relayPath` (kills "the Hub
  asked for keysmith's number").

#### Steps

- [ ] **Step 1: Sync.** Phase 6's tasks 03, 05, 07, 09, 15 and 21 must be merged into
  `feat/work-layer`; if any is not, stop and report which (this is a delta on their files, rule
  H2). Record `git log --oneline -1` of each file in the commit message.
- [ ] **Step 2: Write the Go tests above.** Run `go test ./internal/inference/wire/
  ./internal/keysmith/ ./internal/usage/ ./internal/api/ ./internal/workstart/`: each new test
  fails, and task 07's `TestMintBindsBeforeItGenerates` still passes (decision 3).
- [ ] **Step 3: `wire.go`.** `WorkRef`, the two scopes, the fields on `MintRequest`, `Binding`,
  `RefreshRequest`, and both `Validate` checks. The wire tests pass.
- [ ] **Step 4: Migration `0003` and the two binding stores.** Task 05's migration test sees
  version 3; the suite's round-trip row passes on both stores.
- [ ] **Step 5: keysmith.** `subject.work`, the metadata, `Refresh`'s rebind. keysmith's tests
  pass, the old ones unchanged.
- [ ] **Step 6: usage.** `KeyRef`, `Record`, the column map, `recentSQL`, the sweep's copy,
  `Rows`; keysmith's `/v1/usage` scope allow-list. usage's and `cmd/keysmith`'s tests pass.
- [ ] **Step 7: the launcher and the session create.** `Start`'s sixth argument, `WorkOf`,
  `IssueIDOf`, `cmd/api`'s wiring. Every caller of `Start` in task 15's tests gains
  `wire.WorkRef{}`; nothing else in them changes.
- [ ] **Step 8: `workspend.go`,** its route, its audit line. The api tests pass.
- [ ] **Step 9: the desktop.** The `WORK_LOCAL` row, `IssueSpend`, `spendLine`, the fact, the
  host's read. The Vitest rows pass.
- [ ] **Step 10: Commit** with `git add` for the new files and `git commit -F <msgfile> --
  <paths>`, the paths being exactly the files above.

#### Acceptance

```bash
cd control-plane && go test ./internal/inference/wire/ ./internal/keysmith/ ./internal/binding/... ./internal/usage/ ./internal/gatewayclient/ ./internal/api/ ./internal/workstart/ ./cmd/keysmith/ ./cmd/api/
# expect: ok for each package; the rows above among them, task 07's metadata test unchanged
cd control-plane && go vet ./... && gofmt -l internal/ cmd/
# expect: no output from gofmt
cd control-plane && grep -n -E '"issue": *(sub|req)\.[A-Za-z]*[Tt]itle|Title' internal/keysmith/keysmith.go
# expect: no output (decision 1: no title reaches the key)
cd control-plane && grep -n -E 'ScopeIssue|ScopeLoop' internal/budget/*.go
# expect: no output (decision 5: no budget per issue or loop)
cd desktop && npx vitest run src/work/spend.test.ts src/components/issue src/api/work.test.ts
# expect: spend 4 passed; IssueHead's rows plus 3 new passed; work.test's route rows pass
cd desktop && npx tsc --noEmit
# expect: exit 0
```

#### What this task does NOT claim

- Budgets per issue or per loop: none is designed, and `budget.Spent` stays three-dimensional.
- The loop's spend view (W5, task 172), or any caller that sets `LoopID`: it is plumbed here.
- Spend finer than phase 6's 60-second sweep, or spend in any currency but US dollars. The
  figure is LiteLLM's at its price table (P6 skeleton, `2026-09-25-phase-6-gateway.md:237`); a provider's
  invoice may differ, which *"at the provider's rate"* says and no test proves.
- That an issue unbound mid-session stops spending at once: the current key keeps the id until
  the next generation, at most one TTL (an hour) later (decision 4).
- Spend for sessions minted before this task lands: their keys carry no issue, and nothing
  back-fills them.
- An assist class on the gateway, or any spend by an assist (task 136, its gap).

#### Gaps found while writing this part

1. **The scope names neither binding store.** The sweep reads bindings (`task-09.md:656`), so
   the issue must be on `keysmith.bindings` as well as `key_spend`; `pgstore.go` and
   `memstore.go` (P6 task 05) are in this part's file list and not the skeleton's.
2. **`api.Deps.Gateway` is two things in phase 6.** Task 15 makes it `GatewayLauncher`
   (`task-15.md:1476`) and task 21 `*GatewayDeps` (`task-21.md:924`). Phase 6 must rename one;
   this part reads through `a.usageReader()` so the rename is one line here.
3. **No new local route is in the skeleton for spend.** The scope says *"served on the existing
   GET /v1/usage"*, which is keysmith's and not reachable by the desktop; this part adds
   `GET /v1/work/issues/{issueId}/spend` on the control plane.
4. **Who unbinds an issue mid-session is unnamed.** The refresh reads `Session.IssueRef` at each
   generation (decision 4); task 19 stamps it on create and restart only. If a later W task
   (reassignment, delegation) should drop a session's issue, it must write `IssueRef`.
5. **The Hub's problem type.** `hubclient.Problem` with a `Status` is task 144's reading
   (`task-144.md:605-607`) and **UNVERIFIED**; task 16 should name it.
