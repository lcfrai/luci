### Task 100: the W3 migration, routes, matrix rows and HTTP methods

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 99; TM-05. **Seam:** **SW.19** on the real Hub, as tasks 06-16 did SW.1 and SW.2
for W1, under rule H1 (additive only; the migration takes the next free number at sync time).

Task 99 fixed the W3 wire, the fake and the conformance rows. This task makes the real Hub pass
those rows: one migration, one store file, one route file, the authorisation rows, the HTTP
client methods, and the control plane's relay rows. Every rule is held twice (hub-contract §10.7,
`:2047`): once in Go, once in the database, so a handler bug cannot store what the rule forbids.

**Anchors, re-read on 2026-09-25.** Hub contract on `feat/hub` at `63e7232`: §4.3 the relay rule
(`:277`), §5.3 the matrix (`:335-352`), §7.10 evidence (`:1060-1088`: *"`recordedBy` is the
principal"*, append-only, event `evidence.recorded`), §7.11 the change feed (`:1089`), §8 events
(`:1131`), §10.7 (`:2047`). Phase T on `feat/team` at `64ab2c4`: TM-05's scope (plan `:466-477`:
*"the principal becomes the delegation's owner with `via` set"*); its part (`task-05.md:50-66`:
`Delegable(name, handler)`, `Principal.Via = "fleet"`, `DelegationID`, `OrgAdmin` always false
under a delegation; `:305-320`: `DelegationView`, `Delegable`); TM-03's `hubapi.ViaFleet`,
`RouteName`, `AgentRoleOrchestrator` (`task-03.md:146-153`, `:355-371`); TM-04's `agents` table,
whose `UNIQUE (org_id, owner_user_id, id)` is *"the target of every acts key"*
(`task-04.md:325-345`), and `delegations` (`:289`). Phase W on `feat/work-layer`: task 06's
`issue_comments` (`task-06.md:291-316`, `kind` checked `IN ('comment')`, *"W3 widens (task 99)"*)
and `issues.source` (`:181`, `'orchestrator'` already allowed); task 08's `registerWorkIssues` and
`WriteReplayable` (`task-08.md:313-335`); task 07's matrix helpers (`task-07.md:355-395`); task
16's `WorkRoute` and `Relay()` (`task-16.md:106-160`); task 104's `Linker` (`task-104.md:127-131`);
task 109's `SwitchReader` and `ErrAgentsDisabled` (`task-109.md:94-131`).

**Not on any branch, so UNVERIFIED until merge:** task 99's part (not written when this part was;
every SW.19 Go name below is task 99's scope's, `tasks-81-100.md:439-470`, and Step 1 re-reads
them); TM-05's `server/auth.go` code; Hub task 13's `resolveCaller`; Hub task 14's `Tx`.

Eight decisions the scope left open, made here.

**(a) Paths follow task 16's table, not a `/v1/work/…` prefix.** The scope says *"task 08's
`/v1/work/…` convention"*; task 08 and task 16 register `/v1/issues/…`, `/v1/comments/…` and
`/v1/projects/{projectId}/…` (`task-16.md:127-156`). W3's nine routes use the same shapes, so the
relay derives them with `Relay()` unchanged.

**(b) Attribution is the principal's, never the body's.** `attribution.ownerId` in a body must
equal the principal's `UserID` (TM-05: the delegation's owner), else 403 `agent_not_owner`
(TM-03's code). The stored `agent_owner_id` is written from `tx.P`, and the database pins
`agent_owner_id = created_by` and `(org_id, agent_owner_id, agent_id)` to TM-04's `agents` key, so
Fleet holding Dylan's delegation cannot store a note that names Alice even if Go forgets.

**(c) One voice per owner, at the Hub.** An agent kind needs `via = 'fleet'`, an agent whose
`role` is `orchestrator`, and an issue whose assignee is the owner. A worker's `ag-…` is 403
`agent_not_owner`'s sibling, `forbidden` with `required: "orchestrator"`. Workers report to their
Orchestrator in Fleet (task 103); the Hub never sees five streams.

**(d) The kill switch is checked where the delegated principal is built.** TM-05's
`resolveCaller` gains one call, `s.agentSwitchGate(r, p)`: while the org's `orchestrators` is
false, every non-`GET` request with `p.Via == "fleet"` is 403 `agents_disabled`, and one
metadata-only event `org.agent_write_refused` `{route, delegationId}` is appended in its own
transaction. Reads pass, so Fleet can read the switch it is refused by. A person's write never
reaches the gate. The database half: an insert of an agent-kind comment, or of an issue with
`source = 'orchestrator'`, while the switch is off raises `agents_disabled` (the trigger below).

**(e) An Orchestrator files into `triage`.** W1's `POST /v1/projects/{projectId}/issues` is
registered through `Delegable(RouteWorkIssueCreate, …)`; under `p.Via == "fleet"` the handler
overwrites `state` with `triage` and `source` with `orchestrator` before validation (05 §1 L-193:
*"it lands in `triage` for a person"*). The database refuses an `orchestrator` insert in any other
state. `threadState` stays the request's; a `not_tied` reason is still required.

**(f) Only the assignee delegates; one live delegation per issue.** `POST
/v1/issues/{issueId}/delegate` is the assignee's own (not `Delegable`); anyone else is 403
`forbidden`, `required: "assignee"`. A repeat while live is 200 with the same record and no event.
With no live Orchestrator delegation for the owner in that project (TM-04's `delegations`, class
`orchestrator`), it is 422 `orchestrator_unavailable`: the key is team mode's.

**(g) A proposal never writes evidence; a link is its own write.** `evidence_proposal` rows live
in `issue_comments`. `POST /v1/comments/{commentId}/recorded-as` `{evidenceId}` (task 104's
`LinkEvidenceProposal`) is a person's route: the record must exist in the issue's project, carry
the proposal's `criterionId` and `commit`, and have been recorded by the caller. It sets
`recorded_as` once (a repeat with the same id is 200, no event; another id is 409 `conflict`) and
appends `comment.proposal_linked`. The Hub's `POST /v1/evidence` is unchanged.

**(h) Every write appends one metadata-only event, and no text reaches `data`.** Kinds, ids,
versions and booleans only; guidance text, comment text, `because` and the switch's `reason` never
(hub-contract §8). The events are task 99's spellings (UNVERIFIED), listed with the routes.
