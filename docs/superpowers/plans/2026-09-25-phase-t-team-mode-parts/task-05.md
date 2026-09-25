### Task 05: Hub delegated-caller authentication; `devissuer` client-credentials grant

**Slice:** TM1-α. **Size:** 3 focused hours. **dependsOn:** 04 (`fleet_org`, `fleet_delegation`,
`agents`), 03 (`HeaderDelegation`, `AppRoleFleet`, `ViaFleet`, `RouteName`, the codes,
`AuditEnvelope.Via`), Hub tasks 11–12 (`devissuer`), 13 (`server/auth.go`), 14 (`Write`, `Read`,
`store.Append`); phase 8 task 16 (`internal/entra`, **built** on `feat/env`). **Seam defined:**
ST.3, in Go.

ADR-H1 §5.10 (task 01) admits a second caller: Fleet's workload identity, app-only, acting in a
member's name under a delegation. This task is that rule in the Hub's pipeline, and the grant that
lets the offline stack mint such a token. It creates no delegation (task 06) and serves no team
route (tasks 06 to 08); it proves the pipeline with a test-only route, the way Hub task 13 proves
its own (`export_test.go`).

**What is built and what is planned** (read 2026-09-25):
- `control-plane/internal/entra` is **built** on `feat/env` (`entra.go` 239 lines, `verify.go` 216).
  `Validate` refuses any token whose `azp` is not in `AllowedClients` (`verify.go:116-118`) or whose
  `scp` lacks the required scope (`:119-122`), so today no app-only token can pass.
- `server/auth.go` (Hub task 13), `server/tx.go` and `store/audit.go` (Hub task 14), and
  `internal/devissuer` (Hub tasks 11–12) are **planned**; every signature below that extends one is
  that part's text (`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/`),
  **UNVERIFIED until merge**. Step 1 re-reads them.

Seven decisions.

**(a) `entra` gains task 17's `AppCallers`, verbatim, here.** Task 17 needs the same admission for
the broker and says it consumes the field if this task lands it first (`task-17.md:36-41`,
`:128-139`). This task is first (TM1-α), so it writes `Config.AppCallers`, `Claims.AppOnly` and the
one `switch` of `task-17.md:241-252`, with task 17's three tests. The signal is **no `scp`, a
listed `azp`, the listed role**; `idtyp` is not read, because Entra emits it only as a configured
optional claim (**UNVERIFIED** for our registration), and the three facts suffice.

**(b) The header rules need no database, so they run in `Authenticate`.** Before the mux routes,
straight after `Validate`: a desktop token carrying `X-Panacea-Delegation` is 403 `forbidden`,
`required: "fleet"`; an app-only token without it is 401 `delegation_required`; a header that is
not a `dl-` id is 401 `delegation_revoked`, `reason: "unknown"`, the answer an unknown id gets, so
the format is no oracle. **One route is exempt by exact match**: `GET` on a path whose segments are
exactly `v1`, `orgs`, one id, `changes`. There Fleet must send **no** header (400 `bad_request`,
`field: "X-Panacea-Delegation"`, `task-08.md:195-196`), because the feed is not an owner's act.

**(c) The delegation is resolved where the principal is: the first statement of the
transaction.** Hub task 14's `Write` and `Read` begin with `store.ResolvePrincipal`
(`task-14.md:77-80`, `:98-101`). This task replaces that one call with `s.resolveCaller`, which for
a desktop token is `ResolvePrincipal` unchanged, and for Fleet is `store.FleetDelegation` (task
04's owner function, which joins on the token's tenant, §5.10 rule 6), then the expiry, revocation
and route checks, in that order. `set_config('hub.org_id', …)` follows with the delegation's org,
exactly as it follows `ResolvePrincipal` today, so row-level security is unchanged.

**(d) The route a request names is carried in its context, not parsed from the path.** A team route
registers through `Delegable(name, handler)`, which puts its `hubapi.RouteName` in the request
context; `resolveCaller` reads it back with `RouteFrom`. A route registered without `Delegable` has
no name, and Fleet calling it is 403 `forbidden`, `required: "route"`: this is how routes 46 to 49
stay the owner's own (`task-06.md:102-104`). A delegation that does not list the named route is 403
`forbidden`, `required: <the route name>`.

**(e) The principal is the owner, with less than the owner's token would carry.** `UserID`, `OrgID`,
`TenantID`, `ObjectID`, `Name` and `Email` are the owner's, from task 04's `fleet_delegation` row.
**`OrgAdmin` is always false under a delegation**: it is read from each token (`task-13.md:36`), and
Fleet holds no token of the owner's, so no org-admin act is ever delegated. `Via = "fleet"`,
`DelegationID`, and `Delegation`, a `DelegationView` of the row, are set. Fleet's own `(tid, oid)` is
never provisioned as a user: `resolve_principal` is not called for an app-only token.

**(f) Every event of a delegated write is audit-event v2, by construction.** `store.AuditRow`
gains `Via` and `DelegationID`; `Write` fills both from `tx.P` for every pending event, and
`store.Append` puts them on the `hubapi.AuditEnvelope` (task 03's v2). No handler sets them, so no
handler can forget. A desktop write leaves both empty and stays v1, byte for byte.

**(g) Fleet's per-user rate bucket is per delegation.** Hub task 13 keys the user bucket on
`tid/oid` (`task-13.md:94`). Every Fleet call has one `tid/oid`, so one bucket would let one
member's Orchestrator throttle every other member's. Fleet's key is `fleet/<dl-…>`, one bucket per
owner and class; the org bucket (key `tid`) is unchanged and still caps the whole org. The feed
route's key is `fleet/feed`.

Two more, the part's own, each closing a hole the scope leaves open:

**(h) `entra.New` refuses a client id that is both a desktop client and an app caller.** If the
same GUID were in `AllowedClients` and `AppCallers`, a user token minted for Fleet's registration
would pass as a desktop token and dodge decision (b). The two lists are disjoint, or the Hub does
not start.

**(i) `devissuer`'s grant checks the form, not the assertion.** Fleet sends a workload-identity
client assertion (`task-12.md:723-729`). Offline there is no kubelet and no federated credential,
so `devissuer` requires `client_assertion_type` to be the JWT-bearer URN and `client_assertion`
to be non-empty, and verifies nothing else about it. That is the whole difference from Entra; it
joins contract §6.4's list beside Hub task 12's spent-refresh rule (Hub `task-12.md:90-92`), and
`devissuer` still listens on loopback only.

**Files:**
- `control-plane/internal/entra/entra.go` — `AppCaller`, `Config.AppCallers`, `Claims.AppOnly`,
  `New`'s checks (modify)
- `control-plane/internal/entra/verify.go` — the one `switch` of decision (a) (modify)
- `control-plane/internal/entra/appcaller_test.go` — task 17's three tests and decision (h)'s (new)
- `control-plane/internal/hub/server/auth.go` — `Claims.AppOnly`; the header rules; the rate
  key (modify)
- `control-plane/internal/hub/server/delegation.go` + `delegation_test.go` — `Delegable`,
  `RouteFrom`, `DelegationView`, `isFeedPath`, `resolveCaller` (new)
- `control-plane/internal/hub/server/tx.go` — `Principal` gains three fields; `Read` and `Write`
  call `resolveCaller`; `Write` stamps every `AuditRow` (modify)
- `control-plane/internal/hub/server/export_test.go` — two delegable test routes (modify)
- `control-plane/internal/hub/server/fleet_integration_test.go` — the pipeline, end to end (new)
- `control-plane/internal/hub/store/delegation.go` + `delegation_integration_test.go` —
  `FleetDelegation`, `FleetOrg`, `AgentOwnedBy` (new)
- `control-plane/internal/hub/store/audit.go` + `audit_test.go` — `AuditRow.Via`,
  `AuditRow.DelegationID` onto the envelope (modify)
- `control-plane/cmd/hub/config.go` + `config_test.go` — `PANACEA_APP_HUB_FLEET_CLIENT_ID` (modify)
- `control-plane/cmd/hub/serve.go` — `AppCallers` from it (modify)
- `control-plane/internal/devissuer/config.go`, `token.go` + `token_test.go`, `issuer.go`,
  `discovery.go` — the grant, `MintAppToken`, the discovery list, `Env` (modify)
- `control-plane/cmd/devissuer/dev-tenant.json` — one confidential client, Fleet (modify)

`PANACEA_APP_HUB_DELEGATION_TTL` is read by task 06, which creates delegations; this task only
reads `expires_at` back.

**Interfaces:**
- Produces: `entra.AppCaller`, `entra.Config.AppCallers`, `entra.Claims.AppOnly`;
  `server.Claims.AppOnly`; `server.Delegable(name, h)`, `server.RouteFrom(ctx)`;
  `server.DelegationView`; `Principal.Via`, `Principal.DelegationID`, `Principal.Delegation`
  (a value, zero for a desktop caller); `store.FleetDelegation`, `store.FleetDelegationRow`,
  `store.ErrDelegationUnknown`, `store.FleetOrg`, `store.AgentOwnedBy(ctx, tx, agentID, userID)
  (bool, error)` (the signature task 06 consumes, `task-06.md:65`); `store.AuditRow.Via`,
  `store.AuditRow.DelegationID`; `devissuer.Client.{Confidential,AppRoles,ServicePrincipalOID}`,
  `(*Issuer).MintAppToken`; the variable `PANACEA_APP_HUB_FLEET_CLIENT_ID`.
- Consumes: task 03's `HeaderDelegation`, `AppRoleFleet`, `ViaFleet`, `RouteName`,
  `DelegationClass`, `RevokeReason`, `ValidID`, the five codes of `task-03.md:1433-1437`,
  `AuditEnvelope.Via` and `.DelegationID` (`task-03.md:1329-1336`); task 04's
  `fleet_delegation(uuid, text)` and `fleet_org(uuid)` (`task-04.md:803-829`) and `agents`
  (`task-04.md:325-341`); Hub task 13's `Authenticate`, `Claims`, `ClaimsFrom`, the limiter;
  Hub task 14's `Principal`, `Tx`, `Read`, `Write`, `Forbidden`, `Refuse`, `AuditRow`, `Append`;
  Hub tasks 11–12's `Config`, `Client`, `/token`; Hub task 08's `cmd/hub/config.go`. All Hub
  parts are **planned** (**UNVERIFIED until merge**).
- Required by: 06 and 07 (every team handler reads `tx.P`; `AgentOwnedBy`), 08 (the exempt feed
  route and `FleetOrg`), 11 (Fleet's delivery calls), 12 (`fleetd`'s token against `devissuer`),
  17 (the same `AppCallers`, consumed not re-added), 43 (the offline proof), 44 (revocation),
  66 (the variable, `task-66.md:297`).
