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
   they own for that team's projects, reason `member_removed` (`task-01.md:330-331`), in the same
   transaction as the removal. Task 08's fake already does this and its conformance row skips the
   real Hub "until task 44" (`task-08.md:280-283`); this task removes the skip.
2. **Leases are withdrawn.** No part implements the lease effect of a revocation or a lapse: task
   23's `Engine` has `EndAgent` (`task-23.md:143`) and no `Revoker`. This task adds a four-method
   adapter and joins it to task 12's fan-out (`task-12.md:976-999`).
3. **A leaver still in the team but disabled in Entra is caught.** The 24-hour lapse catches them
   by itself, since only their own desktop renews (`task-06.md:111-114`). This task adds a check
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
