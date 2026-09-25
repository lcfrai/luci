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
