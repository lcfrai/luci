### Task 44: Hub delta: leavers, member removal, delegation revocation end to end

**Slice TM1. Depends on: 06, 11, 39 (and reads 08, 12, 20, 23; Hub task 15). About 2.5 hours.
Seam: ST.3.**

A person who leaves a team must stop acting on it, and so must every agent that acts for them
(`05-design.md` §2.4 item 4, `:174-181`). Task 11 built Fleet's half: a `delegation.revoked` on
the feed, or a lapse on the clock, reaches one `Revoker` within 60 seconds (`task-11.md:47-56`).
Tasks 20 and 39 built two of the effects: sandboxes destroyed and agents ended `access_removed`
(`task-20.md:73-80`), sockets closed (`task-39.md:330-334`). Three pieces are missing, and this task
builds them, then proves the whole chain against the **real Hub**:

1. **The Hub revokes a leaver's delegations.** Removing a member from a team (Hub task 15's
   `DELETE /v1/teams/{t}/members/{u}`), or lowering them to `viewer`, revokes every live delegation
   they own for that team's projects, reason `member_removed` (`task-01.md:339-340`), in the same
   transaction as the removal. Task 08's fake already does this and its conformance row skips the
   real Hub "until task 44" (`task-08.md:287-290`); this task removes the skip.
2. **Leases are withdrawn.** No part implements the lease effect of a revocation or a lapse: task
   23's `Engine` has `EndAgent` (`task-23.md:143`) and no `Revoker`. This task adds a four-method
   adapter and joins it to task 12's fan-out (`task-12.md:976-999`).
3. **A leaver still in the team but disabled in Entra is caught.** The 24-hour lapse catches them
   by itself, since only their own desktop renews (`task-06.md:132-141`). This task adds a check
   at every renewal and a daily sweep, both behind an `AccountState` interface with a fake, and
   names the Graph read to confirm (**UNVERIFIED**, §14.1).

It is a delta on the Hub branch, built after the sync-merge of `feat/hub`. The Hub files it
extends are Hub task 15's `server/teams.go` and `store/teams.go` (`hub …/task-15.md:10-12`) and
task 06's `server/delegations.go`, all **planned**, so every Hub path is **UNVERIFIED until merge**.

**The one thing this task must not get wrong is that removal and revocation are one act.** If the
membership row goes in one transaction and the delegations in a later one, a crash between them
leaves an agent acting for a non-member for up to 24 hours. So the `UPDATE delegations` runs inside
the removal's own transaction, and its events are emitted by the same `Tx` (Hub task 14's write
shape), so the feed sees `team.member_removed` and every `delegation.revoked` together or not at all.

**The second thing is that "within 60 seconds" is measured, end to end, on the real Hub.** The test
removes Linus through the Hub's own route, runs Fleet's real feed, cache, fan-out, sessions, lease
engine and relay in process against a fake broker, and advances one fake clock. The bound is 10 s
poll + one fan-out call (`task-11.md:565-567`), asserted as ≤ 60 s.

**Files:**
- `control-plane/internal/hub/store/teams.go` (Hub task 15) — `RevokeForLeaver` called from
  `RemoveMember` and from a role change to `viewer` (modify)
- `control-plane/internal/hub/store/delegations.go` (task 06) — `RevokeForLeaver`,
  `RevokeForAccount`, `LiveOwners` (modify)
- `control-plane/internal/hub/server/delegations.go` (task 06) — route 48's account check (modify)
- `control-plane/internal/hub/server/accountstate.go` — `AccountState`, `ErrAccountUnknown`, the
  fake, the Graph implementation behind a build of `cmd/hub` (new)
- `control-plane/internal/hub/server/accountsweep.go` — the daily sweep, one replica (new)
- `control-plane/cmd/hub/{main.go,config.go}` — `PANACEA_APP_HUB_ACCOUNT_CHECK`, the sweep start (modify)
- `control-plane/internal/hub/hubapi/codes.go` — `CodeAccountDisabled` (modify)
- `control-plane/internal/fleet/leases/revoke.go` + `revoke_test.go` — the lease effect (new)
- `control-plane/cmd/fleetd/main.go` (task 12) — `fan.add(leases.Revoker{…})` (modify: one line)
- `control-plane/internal/hub/server/leaver_integration_test.go` — the Hub half (new)
- `control-plane/internal/fleet/e2e/leaver_integration_test.go` — the chain, real Hub (new)
- `control-plane/internal/hub/hubclient/conformance/conformance.go` (task 08) — the skip removed (modify)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` — §7.13's revocation sentence, route 48's
  `account_disabled`, the §8 `delegation.revoked` reasons (modify)

**Interfaces:**
- Consumes:
  - task 01: §7.13's delegation record and `revokedReason` (`task-01.md:329-340`); the §8 row
    `delegation.revoked`; `team.member_removed` on route 78's feed (`:512`).
  - task 03: `hubapi.RevokeReason` and its four values (`task-03.md:115-127`), `Delegation`.
  - task 04: the `delegations` table and the `revoke_reason` enum (`task-04.md:118`).
  - task 06: routes 48 and 49 (`task-06.md:132-146`), the delegation store.
  - task 08: `hubclient.OrgFeed`, the fake's `RemoveMember`, the conformance row
    `team-feed/member-removed-revokes` (`task-08.md:372`).
  - task 11: `delegation.Revoker`, `Reason` (`Revoked`, `Lapsed`), `Feed`, `Cache`, `Sweep`,
    `PollEvery` (`task-11.md:565-600`).
  - task 12: the `fanout` (`task-12.md:976-999`); task 20: `Sessions.Revoke`/`Resume`; task 23:
    `Engine.EndAgent` (`task-23.md:143`); task 39: `Relay.Revoke`/`Resume`.
  - Hub task 14: `Tx`, `Emit`, `Event`; Hub task 15: `RemoveMember`, the member role change.
- Produces: `store.RevokeForLeaver`, `store.RevokeForAccount`, `store.LiveOwners`;
  `server.AccountState`, `server.FakeAccounts`, `server.AccountSweep`; `hubapi.CodeAccountDisabled`;
  `leases.Revoker`.
- Required by: task 57 (a leaver is part of the TM1 proof), task 52 (the card's *ended: access
  removed* line reads the agent's `access_removed` exit), task 45 (the revocations are audited).

#### The Hub half — complete source

`store/delegations.go` (task 06), added:

```go
// RevokeForLeaver revokes every live delegation userID owns for any project
// of teamID, reason member_removed. It runs inside the caller's Tx (the
// removal's own), and returns the revoked rows so the caller emits one
// delegation.revoked each. A second call finds nothing and returns nil.
// Store functions take pgx.Tx, as Hub task 14's LockHead and Append do
// (Hub task-14.md:157, :174); server.Tx and Emit live in the server package.
func RevokeForLeaver(ctx context.Context, tx pgx.Tx, now time.Time, teamID, userID string) ([]hubapi.Delegation, error) {
	rows, err := tx.Query(ctx, `
		UPDATE delegations d
		   SET revoked_at = $3, revoked_reason = 'member_removed'
		  FROM projects p
		 WHERE d.org_id = p.org_id AND d.project_id = p.id
		   AND p.team_id = $1 AND d.owner_user_id = $2
		   AND d.revoked_at IS NULL
		RETURNING d.id, d.project_id, d.owner_user_id, d.class, d.routes,
		          d.created_at, d.expires_at, d.renewed_at, d.revoked_at, d.revoked_reason`,
		teamID, userID, now)
	if err != nil {
		return nil, err
	}
	return scanDelegations(rows) // task 06's scanner
}

// RevokeForAccount is the same act for an account the directory reports
// disabled: every live delegation the user owns in the org, any team.
func RevokeForAccount(ctx context.Context, tx pgx.Tx, now time.Time, userID string) ([]hubapi.Delegation, error)

// Owner is one live owner the sweep asks about.
type Owner struct{ UserID, TenantID, ObjectID string }

// LiveOwners lists the distinct owners of live (unrevoked, unexpired)
// delegations in the org, user id after `after`, at most 500 per call.
func LiveOwners(ctx context.Context, tx pgx.Tx, now time.Time, after string) ([]Owner, error)
```

`server/teams.go` (Hub task 15), in route 9's handler after `store.RemoveMember`, and in route 8's
handler when the new role is `viewer` (a viewer launches nothing, §3.4), inside the same `Write`:

```go
	revoked, err := store.RevokeForLeaver(ctx, tx, tx.Now, teamID, userID)
	if err != nil {
		return err
	}
	for _, d := range revoked { // task 01 §8: subject = the delegation id, data {"reason"}
		tx.Emit(Event{Kind: "delegation.revoked", TeamID: teamID, ProjectID: d.ProjectID,
			Subject: d.ID, Data: map[string]any{"reason": string(hubapi.RevokeReasonMemberRemoved)}})
	}
	// RemoveMember then emits Hub task 15's own team.member_removed {} (subject =
	// userID) as it already does; the role write emits its team.member_set.
```

A downgrade to `viewer` emits Hub task 15's `team.member_set {"role":"viewer"}` and not
`team.member_removed`: the person is still a member, and Fleet acts on the `delegation.revoked`
events, not on the membership event (`Event` shape: Hub `task-14.md:67-73`; route 9's emit:
Hub `task-15.md:28`, both checked at `9f2fa31`). Rows stay: the leaver's
tasks, agents, messages and escalations are accountability facts and are never deleted or
reassigned (Global constraint 3: nobody writes another person's records).

#### The account check — complete source

```go
// control-plane/internal/hub/server/accountstate.go

// AccountState answers one question: may this directory account still act?
// The Hub asks it at every renewal (route 48) and in the daily sweep. Which
// Graph read KPMG's tenant permits is UNVERIFIED (§14.1); the candidate is
// GET /v1.0/users/{oid}?$select=accountEnabled under the app permission
// User.Read.All. The Hub never asks it on a read or a Fleet write: the
// 24-hour lapse bounds those.
type AccountState interface {
	Enabled(ctx context.Context, tenantID, objectID string) (bool, error)
}

// ErrAccountUnknown means the directory has no such account: treated as
// disabled. Any other error is "cannot tell", and never revokes.
var ErrAccountUnknown = errors.New("account unknown to the directory")

// AccountCheck is PANACEA_APP_HUB_ACCOUNT_CHECK: "off" (the default until
// §14.1 is answered), "fake" (tests and dev), "graph".
type AccountCheck string

const (
	AccountCheckOff   AccountCheck = "off"
	AccountCheckFake  AccountCheck = "fake"
	AccountCheckGraph AccountCheck = "graph"
)

// FakeAccounts is the test double: every account is enabled until Disable.
type FakeAccounts struct {
	mu       sync.Mutex
	disabled map[string]bool // objectID -> true
	unknown  map[string]bool
	Fail     error // when set, every call returns it ("cannot tell")
	Calls    int
}

func NewFakeAccounts() *FakeAccounts
func (f *FakeAccounts) Disable(objectID string)
func (f *FakeAccounts) Forget(objectID string) // the directory no longer knows it
func (f *FakeAccounts) Enabled(ctx context.Context, tenantID, objectID string) (bool, error)

// GraphAccounts is the one real implementation. Its token comes from the
// Hub's workload identity (phase 8 task 17's federated credential); it holds
// no secret. Built into cmd/hub, used only when the check is "graph".
type GraphAccounts struct {
	HTTP    *http.Client
	Token   func(ctx context.Context) (string, error) // scope https://graph.microsoft.com/.default
	BaseURL string                                    // https://graph.microsoft.com/v1.0
}

func (g GraphAccounts) Enabled(ctx context.Context, tenantID, objectID string) (bool, error)
// 200 {"accountEnabled": b} → b, nil; 404 → false, ErrAccountUnknown;
// anything else (403, 429, 5xx, timeout 5 s) → false, a wrapped error.
```

Route 48, before the renewal `UPDATE`, when the check is not `off`:

```go
	ok, err := s.accounts.Enabled(ctx, tx.P.TenantID, tx.P.ObjectID)
	switch {
	case err == nil && ok:
		// renew as task 06 does
	case err == nil || errors.Is(err, ErrAccountUnknown):
		revoked, rerr := store.RevokeForAccount(ctx, tx, tx.Now, tx.P.UserID)
		if rerr != nil {
			return 0, nil, rerr
		}
		for _, d := range revoked {
			tx.Emit(Event{Kind: "delegation.revoked", ProjectID: d.ProjectID, Subject: d.ID,
				Data: map[string]any{"reason": string(hubapi.RevokeReasonAccountDisabled)}})
		}
		// NOT server.Refuse: an APIError rolls back (Hub task-14.md:25-26), and the
		// revocations must commit. A plain 403 result commits, then is sent.
		return http.StatusForbidden, httpx.Problem{Error: hubapi.CodeAccountDisabled,
			Message: "Your directory account is disabled, so your agents have stopped."}, nil
	default:
		s.log.Warn("account check failed; renewing", "err", err) // cannot tell: never revoke
	}
```

"Cannot tell" renews on purpose: a Graph outage must not stop every agent in the org. The lapse
still bounds a disabled leaver at 24 hours, and the next renewal or sweep asks again.

```go
// control-plane/internal/hub/server/accountsweep.go

// SweepConfig: Interval 24 h; Pause 100 ms between Graph calls (a 429 backs
// off to the next round, never retried in a loop).
type SweepConfig struct {
	Interval time.Duration
	Pause    time.Duration
	Now      func() time.Time
	Log      *slog.Logger
}

// RunAccountSweep is Hub task 24's janitor shape (Hub task-24.md:80-90): each
// round takes pg_try_advisory_lock(hashtext('panacea-app-hub-accounts')) on one
// connection, so one replica sweeps; for each org of hub.org_ids(), in its own
// transaction with set_config('hub.org_id', org, true), it pages
// store.LiveOwners and asks AccountState about each owner outside the
// transaction. A disabled or unknown owner gets RevokeForAccount in a new
// transaction with the head locked, one delegation.revoked each, actor NULL.
// It returns the number revoked this round, for the log and the test.
func RunAccountSweep(ctx context.Context, s *Server, a AccountState, c SweepConfig) error
func (s *Server) SweepAccountsOnce(ctx context.Context, a AccountState) (revoked int, err error)
```

`cmd/hub/config.go` reads `PANACEA_APP_HUB_ACCOUNT_CHECK` (`off` · `fake` · `graph`, default `off`,
anything else a start-up error); `main.go` starts `RunAccountSweep` when it is not `off`. `fake` is
refused when `PANACEA_APP_HUB_ENV=prod` (**UNVERIFIED**: the variable's name on the Hub branch).

#### The lease effect — complete source

```go
// control-plane/internal/fleet/leases/revoke.go
package leases

// Revoker is the leases half of task 11's delegation.Revoker
// (task-11.md:580-585). Revoke withdraws every live lease held by an agent
// started under d, for either reason: a lapsed owner holds nothing while
// paused, so a teammate is not blocked by a person who cannot act. The
// withdrawn leases are released through EndAgent (task-23.md:139-143), which
// emits lease.released to the Hub. Resume does nothing: a resumed agent asks
// again, and Request runs the overlap check afresh.
type Revoker struct {
	Engine *Engine
	Store  store.Store
}

func (r Revoker) Revoke(ctx context.Context, d delegation.Delegation, why delegation.Reason) error {
	agents, err := r.Store.Agents(ctx, d.Key.OrgID, d.Key.ProjectID) // task-10.md:94
	if err != nil {
		return err
	}
	var errs []error
	for _, a := range agents {
		if a.Delegation != d.ID { // task-09.md:139; ended agents hold nothing, EndAgent finds 0
			continue
		}
		if _, err := r.Engine.EndAgent(ctx, d.Key.OrgID, a.ID); err != nil {
			errs = append(errs, fmt.Errorf("agent %s: %w", a.ID, err))
		}
	}
	return errors.Join(errs...) // idempotent: a repeat finds no live lease
}

func (r Revoker) Resume(context.Context, delegation.Delegation) error { return nil }
```

`cmd/fleetd/main.go` (task 12), after the line at `task-12.md:850`:

```go
	revokers.add(leases.Revoker{Engine: leaseEngine, Store: st})
```

The fan-out tries every effect whichever fails (`task-12.md:984-991`), so a lease error does not
stop the sessions half, and the feed retries the whole act (`task-11.md:693-700`).

#### Tests first

The people are the design's: Grace (admin), Linus and Margaret (members), Vera (viewer), in team T
with team project P and solo project S. The Hub tests run on Hub task 06's integration harness
(`-tags integration`, a real Postgres); the Fleet unit tests on task 10's fake store; the chain test
on both.

**P becomes a team project the way every other suite makes one: Grace calls route 87**
(`POST /v1/projects/{p}/mode`, task 06's, fix batch b01 X-C1) in the setup — never by SQL. Since
b01, route 46 answers 409 `conflict` `not_team_project` on a solo project
(`task-06.md:134-138`), so the delegations these tests open with cannot exist until P is `team`.
Route 87 is behind the audit gate, so this harness is started with
`PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` **and** `PANACEA_APP_HUB_AUDIT_SINK=fake`, with
`PANACEA_APP_HUB_ENV=local` — task 06's `teamWorld` rule (`task-06.md:552-558`,
`task-45.md:65-69`), so that task 45's gate swap leaves this suite green. S stays `solo`: no row
below puts a delegation on it (route 46 would refuse), and it is the contrast that proves removal
touches team projects only — the out-of-scope rule at the end of this part. The second team, U,
whose project `TestRemovalRevokesOnlyThatTeamsProjects` uses, needs route 87 as well. Reuse task
06's `teamWorld` where the people line up rather than building a second world.

**Hub, `server/leaver_integration_test.go`:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemovalRevokesInTheSameTransaction` | Linus has a worker and an orchestrator delegation on P; Grace removes him (route 9) → both `revokedAt` = the removal's `at`, `revokedReason = "member_removed"`; the audit holds `team.member_removed` and two `delegation.revoked` with consecutive `seq` | revocation in a second transaction |
| `TestRemovalRollsBackAsOne` | a test hook fails the second `delegation.revoked` insert → Linus is still a member **and** both delegations are live | removal committed without its revocations |
| `TestRemovalRevokesOnlyThatTeamsProjects` | Linus also owns a delegation in team U's project → it stays live | `WHERE` without `p.team_id` |
| `TestSelfRemovalRevokes` | Linus removes himself (route 9's "or self") → same result as Grace removing him | only the admin path calls `RevokeForLeaver` |
| `TestDowngradeToViewerRevokes` | Grace sets Linus to `viewer` (route 8) → both revoked, `member_removed`; the audit holds `team.member_set {"role":"viewer"}` and **no** `team.member_removed` | viewers keep acting agents; or a false removal event |
| `TestUpgradeDoesNotRevoke` | Vera set to `member` → no `delegation.revoked` | every role write revokes |
| `TestRemovalOfANonOwnerEmitsNoRevocation` | Margaret, who owns none, removed → zero `delegation.revoked` | an empty `RETURNING` still emitting |
| `TestRenewalOfADisabledAccountRevokes` | check `fake`; `Disable(linus.oid)`; Linus renews (route 48) → 403 `account_disabled`, and **after the response** every live delegation he owns, in any team, is `account_disabled` | the 403 built with `Refuse`, so the revocations roll back |
| `TestRenewalWhenTheDirectoryCannotTell` | `Fail = errors.New("429")` → 200, renewed, zero revocations | an outage revoking the org |
| `TestUnknownAccountIsDisabled` | `Forget(linus.oid)` → 403 `account_disabled` | 404 read as "enabled" |
| `TestCheckOffNeverCalls` | check `off` → `Calls == 0`, renewal 200 | the check running when off |
| `TestSweepRevokesAndIsSingleReplica` | two `SweepAccountsOnce` at once, Linus disabled → total `revoked` = 2, one pair of events; a third round → 0 | two replicas double-sweeping; a non-idempotent sweep |
| `TestSweepSkipsTheLapsed` | Linus's delegations past `expiresAt` → not listed by `LiveOwners`, no Graph call | the sweep asking about people who already cannot act |

**Fleet, `leases/revoke_test.go`:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRevokeWithdrawsEveryLeaseOfTheDelegation` | two agents under `dl-a`, one under `dl-b` → `Revoke(dl-a, Revoked)` releases the first two agents' leases, not the third's; `lease.released` in the outbox for each | matching by owner, so a second delegation's agents lose theirs |
| `TestLapseAlsoWithdraws` | `Revoke(dl-a, Lapsed)` → the same releases | leases kept while the owner cannot act |
| `TestRevokeIsIdempotent` | a second call → zero releases, `nil` | an error on "nothing to release", which would loop the feed |
| `TestResumeRestoresNothing` | `Revoke` then `Resume` → the leases stay `released`; the agent's next `Request` re-runs the overlap check against Margaret's lease and comes back `contested` | a resumed agent silently re-granted over a teammate |

**The chain, `fleet/e2e/leaver_integration_test.go`** (build tag `integration`; the real Hub
in process on Hub task 34's harness; Fleet's real `delegation.Feed`, `Cache`, `fanout`,
`sessions.Sessions`, `leases.Engine` and task 39's `relay.Relay` on task 10's Postgres store; task 20's
broker fake; one `fakeClock` driving the feed ticker and Fleet's clocks):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRemovalStopsEverythingWithinSixtySeconds` | Linus's worker running, a hold on `src/**`, a viewer socket open; Grace removes him through route 9 at `t0`; the clock advances in 1 s steps → by `t0 + 60 s`: the broker sandbox destroyed, the agent's exit `access_removed`, the lease `released`, the socket closed `4003` *access removed* (`task-39.md:58`), and the Hub's projection shows `agent.ended {"exit":"access_removed"}`; the measured lag is ≤ `PollEvery` + one fan-out (`task-11.md:568-570`) | any effect missing from the fan-out; the Hub's removal not reaching the feed |
| `TestLapsePausesAndWithdrawsThenResumes` | no renewal for 24 h + 1 s → sessions paused (not destroyed), leases released, sockets closed; Linus renews (route 48) → the next poll resumes the session and the hold is not re-granted until asked | a lapse treated as a removal; a lapse that never resumes |
| `TestDisabledAccountChain` | check `fake`, Linus disabled, his desktop renews → 403; within 60 s the same effects as a removal | the renewal check not reaching Fleet |

The conformance row `team-feed/member-removed-revokes` (`task-08.md:372`) now runs against the real
Hub: its `t.Skip("task 44")` is deleted, and the run has zero `--- SKIP` lines in the team groups.

#### The contract lines

Hub contract (`2026-09-23-hub-contract.md`, as task 01 extends it):
- §7.13, after the delegation record: *"Removing a member from a team, or setting them to
  `viewer`, revokes in the same transaction every live delegation they own for that team's projects,
  `revokedReason = "member_removed"`. A renewal by an account the directory reports disabled or
  unknown revokes every live delegation its owner holds in the org, `account_disabled`, and answers
  403 `account_disabled`; a directory that cannot answer renews."*
- §4.2: `account_disabled` | 403 | the caller's directory account is disabled | — .
- §8, `delegation.revoked`: `data.reason` is one of the four `RevokeReason` values
  (`task-03.md:117-122`); actor NULL for the sweep.

#### Steps

- [ ] **Step 1: Read, after the sync-merge of `feat/hub`.** Hub task 15's `RemoveMember` and route
  8 handler as merged; Hub task 06's `server/delegations.go` route 48; Hub task 24's janitor. If a
  signature differs from this part's, the store functions follow the merged one and this part's
  tests do not change.
- [ ] **Step 2: Hub tests.** Write `leaver_integration_test.go`; it fails (`undefined:
  RevokeForLeaver`, `FakeAccounts`).
- [ ] **Step 3: Store and handlers.** `RevokeForLeaver`, `RevokeForAccount`, `LiveOwners`; the
  calls in routes 8 and 9; the Hub tests of removal pass.
- [ ] **Step 4: Account check.** `accountstate.go`, route 48's check, `CodeAccountDisabled`,
  `config.go`; then `accountsweep.go` and its start in `main.go`. The Hub tests pass.
- [ ] **Step 5: Leases.** `revoke_test.go` fails, then `revoke.go` and the `fleetd` line; it passes.
- [ ] **Step 6: The chain and the conformance row.** `fleet/e2e/leaver_integration_test.go`;
  delete the `t.Skip("task 44")`; both pass.
- [ ] **Step 7: Contract.** The three edits above.
- [ ] **Step 8: Commit.** `git add` the new files; `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestRemoval|TestSelfRemoval|TestDowngrade|TestUpgrade|TestRenewal|TestUnknownAccount|TestCheckOff|TestSweep' -count=1
# ok  	…/internal/hub/server	…   (13 tests)
cd control-plane && go test ./internal/fleet/leases/ -run 'TestRevoke|TestLapse|TestResume' -count=1
# ok  	…/internal/fleet/leases	…
cd control-plane && go test -tags integration ./internal/fleet/e2e/ -run 'TestRemovalStops|TestLapsePauses|TestDisabledAccountChain' -count=1 -v | grep -E '^(--- |ok|FAIL)'
# --- PASS: TestRemovalStopsEverythingWithinSixtySeconds   (the ≤ 60 s bound is asserted inside)
# --- PASS: TestLapsePausesAndWithdrawsThenResumes
# --- PASS: TestDisabledAccountChain
cd control-plane && go test -tags integration ./internal/hub/... -run Conformance -count=1 -v | grep -c -- '--- SKIP: .*team'
# 0
```

#### What this task does not claim

- **That the Graph read works in KPMG's tenant.** `GraphAccounts` is built and unit-tested against
  an `httptest` server only; `User.Read.All` as an application permission needs a tenant admin's
  consent, which is §14.1's open item. Until it is granted the check ships `off`, and a disabled
  leaver who is still a team member is bounded by the 24-hour lapse alone.
- **That an Entra disablement reaches the Hub faster than the next renewal or the daily sweep.**
  There is no push from Entra; a change-notification subscription is a later option.
- **That a leaver's past work is removed.** Their tasks, messages, escalations and agents stay, as
  accountability facts, and a teammate may re-own an open task through the normal routes.
- **Anything about a removed member's own solo projects.** Removal is per team; solo delegations
  are revoked only by the account check.

#### Gaps found in the skeleton

1. **The skeleton's scope says "a daily account-state check at renewal"**; a renewal and a daily
   sweep are two triggers, and this part builds both. Without the sweep, a disabled leaver whose
   desktop is off is caught only by the lapse, which is the same bound, so the sweep is cheap
   insurance, not a new guarantee.
2. **The fake's `RemoveMember` records `team.member_removed` for a downgrade?** Task 08 does not say
   (`task-08.md:287-290`). This part emits `team.member_set` for a downgrade on the real Hub; if the
   fake records `team.member_removed` for it, task 08's fake should change to match.
3. **`LiveOwners` needs each owner's `tenantId` and `objectId`**, which this part assumes Hub task
   15's `users` table keeps (Hub task 14's `Principal` carries both, Hub `task-14.md:50-55`).
   **UNVERIFIED until merge.**
4. **`PANACEA_APP_HUB_ENV`** as the guard against `fake` in production is **UNVERIFIED**: the Hub
   branch may name its environment differently.
