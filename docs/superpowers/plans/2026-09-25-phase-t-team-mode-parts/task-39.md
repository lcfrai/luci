### Task 39: Fleet's own attach handler and socket revocation

**Slice:** TM1. **Focused time:** 3 h. **dependsOn:** 11 (delegations, the `Revoker`, the
per-replica cache), 12 (Entra per request, `Server.Route`, the §3.4 table), 19 (one control stream
per sandbox, the broker dial); 17 (the broker's attach relay under a delegation) and 21 (`fleetbe`
dials `GET /v1/agents/{ag}/attach`) through them.

Phase 2's attach handler was written for a loopback control plane that one person runs on their
own laptop. It authorises by a per-session attach secret and a loopback origin check
(`control-plane/internal/api/attach.go:62`, `:127-140`). Fleet is a **network service** that
holds every member's delegations, so its trust model is new, not ported (§7.4, global constraint
11). This task writes Fleet's own handler, in a new package, and pins four properties with tests:

1. **Every upgrade is authorised per request, per owner.** The Entra token (task 12's middleware)
   names a person; the agent's row names its owner; they must be the same person, whatever that
   person's role. An `admin` does not see a teammate's terminal (OD-15: no teammate terminal by
   default; the watch view is TM3 and opt-in).
2. **Knowing an `ag-…` id grants nothing.** Another owner's agent and a non-existent agent get the
   same answer, byte for byte, so the route is not an oracle for which ids exist.
3. **The relay carries a terminal only to its owner, and forwards only the owner's keystrokes.**
   It never originates a frame (task 38's `ptywriter` is the only Fleet code that does).
4. **Every open socket closes within 60 seconds of a revocation, on every replica.** Task 11's
   `Revoker` is called only on the replica holding the `fleet.feed` lock (`task-11.md:591-593`), but
   a socket lives on whichever replica the load balancer picked; so each replica closes its own.

And, per global constraints, the route joins the Z5 role matrix (§3.4, §7.4 item 5).

Five decisions the scope left open, made here so nobody re-derives them.

**(a) The route has no `{p}`, so the handler resolves the agent before it authorises.** Task 21
fixed the path as `GET /v1/agents/{ag}/attach` (`task-21.md:87`). Task 12's `authorize` looks up
membership by `r.PathValue("p")` (`task-12.md:419-420`), which is empty here. This task adds
`Server.RouteAgent`: it reads `{ag}` from task 10's agent rows, and runs task 12's `authorize` with
that agent's project in place of `{p}`, then requires `agent.Owner == principal.UserID`. The verb
is `VerbSessionsOwn` (`task-12.md:1076`: viewer no, member own, admin own, auditor no), so the
route sits in the matrix like every other.

**(b) Not yours and not there are one answer: `404 agent_not_found`.** The owner check runs
before any other refusal that could differ by agent (running, class, project), and a missing row
and another owner's row both produce `httpx` problem `agent_not_found` with the same title and no
`details`. `TestNotYoursEqualsNotThere` compares the two responses' bytes. Task 21's table lists
`404` and `409 agent_not_running`; the `409` is reached only by the owner.

**(c) Revocation is a per-replica sweep plus the leader's fast path.** Every replica runs task
11's `Feed`, so every replica's `delegation.Cache` drops a revoked delegation within one poll (10
s, `task-11.md:568-570`). Each replica's relay keeps a registry of its own open sockets and, every
`SweepEvery` (10 s), closes each socket whose delegation is gone from its cache or past expiry, or
whose owner's membership no longer grants `VerbSessionsOwn` (read **uncached**, one Hub call per
distinct owner and project per sweep, so the membership cache's 60 s TTL does not stack on top).
The leader replica's `Revoke` also closes its own matching sockets at once. Worst case: poll 10 s
+ sweep 10 s + one Hub round trip, inside 60 s with room.

**(d) A socket also closes when the token that opened it expires.** An upgrade is authorised
once, by one token; a socket that outlived it would be authorised by nothing. At the token's
`exp` the relay closes with `4001` *"sign in again"*; `fleetbe` re-dials with a fresh token and
phase 2's replay restores the screen. Revocation closes with `4003` *"access removed"* (task 17
uses `4003` for the same condition on the broker's side), and `fleetbe` maps `4003` to
`ErrNotRunning`, so the cockpit says the session is no longer available and does not retry.

**(e) Attached is cluster-wide, and fails towards "attached".** Task 38 skips a nudge while the
owner is attached (`task-38.md` decision e), but its ladder runs on the replica holding the
worker's control stream, not the one holding the owner's socket. Each open socket is a row in
`fleet.attachments`, heartbeated every 15 s; `Attached(agent)` is true when a row for that agent
has a heartbeat inside 30 s. A store error answers **true**: the cost is a missed nudge, never a
template typed under someone's hands.

