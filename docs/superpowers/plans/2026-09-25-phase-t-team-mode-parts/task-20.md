### Task 20: Fleet sessions over the broker; `ag-…` ids; gateway tokens with team tags

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 11 (outbox, delegation cache, `Revoker`), 14
(`policy.Store.Effective`), 15 (`Spec.Class`, the policy mount), 19 (`control.Streams`), P6-15
(`gatewayclient`), P8-22 (`remotebe.Client`). **Seams:** S6.3 (the mint API), S8.1 (unchanged; this is
Fleet's side, not a `backend.Backend`).

A team agent is born here. `Launch` takes a request that has already passed task 12's role check,
mints the agent's `ag-…` id, writes the agent row and its Hub `agents.start` record in one
transaction, and returns — then, in the background, asks the broker for a Kata sandbox under the
owner's delegation, reads the pod's source address, mints a gateway key bound to that address and
tagged for the team, and hands the sandbox to task 19's streams. Everything the pod needs to prove who
it is (its agent id, its class, its control and attach tokens, its policy digest) is rendered into
the spec here and nowhere else.

Three rules from the design shape it. **One Orchestrator per person per project**, created on that
person's first team launch and never for a `viewer` (OD-16, `05-design.md` §4.1). **The owner is the
delegation's**, never Fleet's and never a body's (§3.3 rule 1; `task-01.md:122-126`). **No org key
and no Global fallback**: a sandbox holds a one-hour key bound to its pod, and an AU project whose AU
route is down gets no key at all (P6-D5, `.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:69`).

**Built after the sync-merges of `feat/env` and `feat/phase-6-gateway`.** `remotebe` is phase 8 task
22's plan (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-22.md:60-116`),
`gatewayclient` is phase 6 task 15's plan (`2026-09-25-phase-6-gateway.md:250`) over task 03's wire
(`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-03.md:26-40`).
Neither exists on any branch on 2026-09-25; both are **UNVERIFIED until merge**. `ids.Wide` is built on
`feat/hub` (`control-plane/internal/ids/ids.go:61`) and not yet on `feat/team-mode`.

**Decisions made here.**

1. **`Launch` answers before the pod is ready.** The broker's create waits for `/healthz` for up to
   its `READY_TIMEOUT`, 120 s (phase 8 `task-22.md:91-92`). Task 21's `fleetbe` already expects Fleet to
   answer first and polls for readiness (`task-21.md:231-233`). So `Launch` commits the row with status
   `starting` and returns `201`; a goroutine does the broker create, the address, the mint and the
   stream. A failure there ends the agent with `exitClass: failed` through the same outbox path, so
   the Hub's record is never left `starting` forever.
2. **The id is `ids.Wide("ag")`** (task 09, `task-09.md:70`), and it is also the broker sandbox id,
   the keysmith `SessionID` and the pod's `PANACEA_APP_AGENT_ID`. One id end to end means the audit
   line, the binding and the Hub record can be joined without a mapping table. It matches
   `backend.Spec.ID`'s pattern (lower-case, `-`, ≤ 40), which the broker validates.
3. **One Orchestrator, enforced by the database.** Migration `0003_one_orchestrator.sql` adds a
   partial unique index on live Orchestrators per `(org, project, owner)`. `EnsureOrchestrator` inserts;
   on `store.ErrConflict` it reads the winner and returns it. Two replicas launching at once therefore
   yield one Orchestrator and two identical answers, with no lock.
4. **The Orchestrator needs its own delegation.** A worker delegation covers worker routes only
   (`task-01.md:292-297`), and the broker lets only an `orchestrator` delegation create an
   Orchestrator (task 17 decision 5). So a member's first team launch creates the Orchestrator **only
   if** a live `orchestrator`-class delegation for that owner and project is in task 11's cache;
   otherwise the worker launches alone, which task 01 allows ("a worker the member launched with no
   Orchestrator", `:294`), and the answer says `orchestrator: "awaiting-delegation"`. Who creates that
   delegation is *Gap S20-1*.
5. **The policy is read, never composed, here.** `policy.Store.Effective(ctx, org, project, class)`
   (task 14, `task-14.md:360`) returns the rendered file, version and digest; the file goes into the
   spec's policy field (task 15), the digest into `PANACEA_APP_POLICY_SHA256` (task 13's `EnvDigest`,
   `task-13.md:133`), and both into the `agents.start` body's `policy` (`task-01.md:350-360`). A render
   error fails the launch with `409 policy_invalid` before any sandbox exists.
6. **Tokens and control secrets are derived or minted, never stored.** The attach and control tokens
   are task 19's HMAC derivation (decision 6 there) with labels `panacea-attach/v1/` and
   `panacea-control/v1/`, so no replica stores them and task 39 can recompute the attach token. The
   gateway key is minted per session (worker) or per turn (Orchestrator) and sent down the stream with
   `Streams.Send` (task 16 step 6b), never `Push`, so it never touches Postgres.
7. **The tags are the mint request's fields.** `Owner{TID, OID}` is the delegation owner's (Gap S17-1:
   the Hub's delegation answer must carry them), `TeamID` and `ProjectID` from the delegation's project,
   `AgentID` the `ag-…`, `SessionClass` the class, `PodAddr` the broker's recorded source (task 17
   decision 9), and for a peer-triggered Orchestrator turn `Counterpart`, the other owner, which phase
   6's `MintRequest` does not have yet (*Gap S20-2*). Route is not sent: keysmith decides it from the
   project's route source and returns it (`task-03.md:31`).
8. **AU fails closed, twice.** Keysmith refuses to mint when the AU route is unknown (P6-D5). Fleet
   also reads the project's required route through phase 6's `wire.RouteSource` before minting and
   checks `MintResponse.Route` against it after; a mismatch revokes the key at once and fails the
   session with `gateway_route_mismatch`. There is no retry with different parameters, no Global
   default, and no path in this file that constructs a Global route.
