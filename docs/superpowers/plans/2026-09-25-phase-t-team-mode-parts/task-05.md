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

#### Contract — complete

**`entra`, the admission (decisions a and h).** Built file, built lines: `Config` is
`entra.go:23-41`, `Claims` `:44-54`, `JWKSValidator` `:90-106`, `New`'s client loop `:158-164`;
the checks this replaces are `verify.go:116-122`. The text is `task-17.md:128-139` and
`:241-252` with the helper names the built file uses (`v.clients`, `contains`).

```go
// control-plane/internal/entra/entra.go — additions

// Config gains, after AllowedClients:

	// AppCallers admits an app-only token (no scp) whose azp is ClientID and
	// whose roles hold Role (ADR-H1 §5.10 rule 1). Empty keeps the validator
	// delegated-only, as before. No ClientID may also be in AllowedClients.
	AppCallers []AppCaller

// AppCaller is one app-only identity a resource admits.
type AppCaller struct {
	ClientID string // a GUID, trimmed and lower-cased as AllowedClients are
	Role     string // the app role it must hold, e.g. "Hub.Fleet"
}

// Claims gains, after Roles:

	AppOnly bool // true only for a token admitted through Config.AppCallers

// JWKSValidator gains, after clients:

	appCallers []AppCaller

// New, straight after the AllowedClients loop (:158-164):

	for _, ac := range cfg.AppCallers {
		id := normGUID(ac.ClientID)
		if !isGUID(id) {
			return nil, fmt.Errorf("entra: config: app caller %q is not a GUID", ac.ClientID)
		}
		if strings.TrimSpace(ac.Role) == "" {
			return nil, fmt.Errorf("entra: config: app caller %s has no role", id)
		}
		if v.clients[id] { // decision (h)
			return nil, fmt.Errorf("entra: config: %s is both an allowed client and an app caller", id)
		}
		v.appCallers = append(v.appCallers, AppCaller{ClientID: id, Role: ac.Role})
	}
```

```go
// control-plane/internal/entra/verify.go — :116-122 become one decision

	scopes := strings.Fields(p.Scp)
	appOnly := false
	switch {
	case len(scopes) == 0 && v.appCaller(p.Azp, p.Roles):
		appOnly = true // no scp, a listed azp, the listed role
	case !v.clients[p.Azp]: // :116-118, unchanged
		return Claims{}, fmt.Errorf("%w: azp is not an allowed client", ErrClient)
	case !contains(scopes, v.scope): // :119-122, unchanged
		return Claims{}, fmt.Errorf("%w: scp does not hold %s", ErrScope, v.scope)
	}
	// … the oid check (:123-125) is unchanged: an app-only token's oid is its
	// service principal's, a GUID; the returned Claims gains AppOnly: appOnly.

// appCaller is true when some AppCaller has ClientID == azp and Role in roles.
func (v *JWKSValidator) appCaller(azp string, roles []string) bool {
	for _, ac := range v.appCallers {
		if ac.ClientID == azp && contains(roles, ac.Role) {
			return true
		}
	}
	return false
}
```

`TestAppOnlyTokensFailTheScopeCheck` (`entra_test.go:327-333`) keeps passing unchanged: its token's
`azp` is the desktop, which is no app caller, so it still falls to `ErrScope`.

**`server`, the header rules and the route name (decisions b, d and g).**

```go
// control-plane/internal/hub/server/auth.go — additions to Hub task 13's text

// Claims gains, after BindEmail:

	// AppOnly: entra.Claims.AppOnly, Fleet's workload identity (§5.10 rule 1).
	// For an app-only token Name, Email and BindEmail are "" and OrgAdmin is
	// false, whatever roles it holds: an app never administers an org.
	AppOnly bool
	// DelegationID is the X-Panacea-Delegation value, set only when AppOnly and
	// FleetFeed is false. resolveCaller reads it; Read has no request.
	DelegationID string
	// FleetFeed: AppOnly on the one exempt route (isFeedPath).
	FleetFeed bool

// Authenticate, step 3 gains a last sub-step, straight after Validate succeeds
// and before step 4's limits (so step 4 can key on its result):
//  3b. delegationHeader(c, r) → a refusal is written as the problem and the
//      request ends; nothing is logged but tid, the code and the reason.
// Step 4's per-user key becomes userKey(c); the org key (tid) is unchanged.

// delegationHeader is §5.10 rule 2's refusals that need no database.
func delegationHeader(c *Claims, r *http.Request) *APIError {
	vals := r.Header.Values(hubapi.HeaderDelegation)
	switch {
	case !c.AppOnly && len(vals) > 0:
		return Refuse(403, hubapi.CodeForbidden, "Only Fleet may name a delegation.",
			map[string]string{"required": "fleet"})
	case !c.AppOnly:
		return nil
	case isFeedPath(r) && len(vals) > 0:
		return Refuse(400, hubapi.CodeBadRequest, "The org feed takes no delegation.",
			map[string]string{"field": hubapi.HeaderDelegation})
	case isFeedPath(r):
		c.FleetFeed = true
		return nil
	case len(vals) == 0:
		return Refuse(401, hubapi.CodeDelegationRequired, "Fleet must name a delegation.", nil)
	case len(vals) > 1:
		return Refuse(400, hubapi.CodeBadRequest, "Name one delegation.",
			map[string]string{"field": hubapi.HeaderDelegation})
	case !hubapi.ValidID(hubapi.PrefixDelegation, vals[0]):
		// The answer an unknown id gets (task 01 §4.2), without echoing the value.
		return Refuse(401, hubapi.CodeDelegationRevoked, "That delegation is not in force.",
			map[string]string{"delegationId": "", "reason": "unknown"})
	}
	c.DelegationID = vals[0]
	return nil
}

// userKey is decision (g): one bucket per delegation, so one member's
// Orchestrator cannot drain another's.
func userKey(c Claims) string {
	switch {
	case !c.AppOnly:
		return c.TenantID + "/" + c.ObjectID // Hub task 13, unchanged
	case c.FleetFeed:
		return "fleet/feed"
	default:
		return "fleet/" + c.DelegationID
	}
}
```

The 403 and 400 answers here record no `auth.forbidden` event: `Authenticate` has no principal
and no org yet (Hub task 14's `Forbidden` needs both), so the log line is the record, as for every
other refusal of step 3. The 403 of a route check (below) runs inside the transaction and does
record one.

```go
// control-plane/internal/hub/server/delegation.go
package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5" // the Hub's driver (Hub task 06), not in feat/env's go.mod

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

// DelegationView is the delegation a Fleet request acts under, as resolveCaller
// loaded it. The zero value for a desktop caller and for the feed.
type DelegationView struct {
	ID        string
	ProjectID string
	Class     hubapi.DelegationClass
	Routes    []hubapi.RouteName
	ExpiresAt time.Time
}

type routeKey struct{}

// Delegable registers a team route under its contract §7.0 name (decision d).
// A route registered without it has no name, and Fleet calling it is 403.
func Delegable(name hubapi.RouteName, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, name)))
	})
}

// RouteFrom returns the name Delegable stored.
func RouteFrom(ctx context.Context) (hubapi.RouteName, bool) {
	n, ok := ctx.Value(routeKey{}).(hubapi.RouteName)
	return n, ok
}

// isFeedPath is decision (b)'s exact match: GET, and the path's segments are
// exactly "v1", "orgs", one non-empty id, "changes". Never a prefix match.
func isFeedPath(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	s := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	return len(s) == 4 && s[0] == "v1" && s[1] == "orgs" && s[2] != "" && s[3] == "changes"
}

// resolveCaller is contract §5.1 steps 3a and 5 (decision c), the first statement
// of every Read and Write transaction, where store.ResolvePrincipal was. created
// is true only when a desktop caller was provisioned by this call.
func (s *Server) resolveCaller(ctx context.Context, tx pgx.Tx, now time.Time) (p Principal, created bool, err error) {
	c, _ := ClaimsFrom(ctx)
	switch {
	case !c.AppOnly:
		rp, err := store.ResolvePrincipal(ctx, tx, c.TenantID, c.ObjectID, c.Name, c.Email)
		if err != nil {
			return Principal{}, false, err // ErrNotEnrolled: 403, as Hub task 14 maps it
		}
		return Principal{OrgID: rp.OrgID, UserID: rp.UserID, TenantID: c.TenantID,
			ObjectID: c.ObjectID, Name: c.Name, Email: c.Email, OrgAdmin: c.OrgAdmin}, rp.Created, nil
	case c.FleetFeed:
		org, err := store.FleetOrg(ctx, tx, c.TenantID)
		if err != nil {
			return Principal{}, false, err // ErrNotEnrolled
		}
		return Principal{OrgID: org, TenantID: c.TenantID, Via: hubapi.ViaFleet}, false, nil
	}
	return s.resolveDelegated(ctx, tx, now, c)
}

// resolveDelegated is §5.10 rules 2, 3 and 6 for a named delegation: the lookup,
// then expiry, revocation and the route, in that order (decision c). The
// principal is built before the route check, so the 403's auth.forbidden event
// has an org and an actor (the owner, via Fleet).
func (s *Server) resolveDelegated(ctx context.Context, tx pgx.Tx, now time.Time, c Claims) (Principal, bool, error) {
	id := c.DelegationID
	d, err := store.FleetDelegation(ctx, tx, c.TenantID, id)
	if errors.Is(err, store.ErrDelegationUnknown) {
		return Principal{}, false, Refuse(401, hubapi.CodeDelegationRevoked,
			"That delegation is not in force.", map[string]string{"delegationId": id, "reason": "unknown"})
	}
	if err != nil {
		return Principal{}, false, err
	}
	if !now.Before(d.ExpiresAt) {
		return Principal{}, false, Refuse(401, hubapi.CodeDelegationExpired,
			"That delegation has expired.", map[string]string{"delegationId": id})
	}
	if d.RevokedAt != nil {
		return Principal{}, false, Refuse(401, hubapi.CodeDelegationRevoked,
			"That delegation was revoked.", map[string]string{"delegationId": id, "reason": string(d.RevokedReason)})
	}
	p := Principal{
		OrgID: d.OrgID, UserID: d.OwnerUserID, TenantID: c.TenantID, ObjectID: d.OwnerOID,
		Name: d.OwnerName, Email: d.OwnerEmail,
		OrgAdmin:     false, // decision (e): read from a token, and Fleet holds none of the owner's
		Via:          hubapi.ViaFleet,
		DelegationID: id,
		Delegation: DelegationView{ID: id, ProjectID: d.ProjectID, Class: d.Class,
			Routes: d.Routes, ExpiresAt: d.ExpiresAt},
	}
	name, ok := RouteFrom(ctx)
	if !ok {
		return p, false, Forbidden("route") // routes 46, 48, 49 and every non-team route
	}
	// Listed, and allowed for the class: task 06 refuses a delegation whose routes
	// exceed its class, and this second check holds even for a row written around it.
	if !slices.Contains(d.Routes, name) || !d.Class.Allows(name) {
		return p, false, Forbidden(string(name))
	}
	return p, false, nil
}
```

`Principal` (Hub task 14's `tx.go`) gains three fields, after `OrgAdmin`:

```go
	Via          string         // "" for a desktop caller; hubapi.ViaFleet under a delegation or on the feed
	DelegationID string         // the dl-… acted under; "" on the feed and for a desktop caller
	Delegation   DelegationView // zero unless DelegationID is set
```

**The one change to `Read` and `Write`.** Each replaces its `store.ResolvePrincipal` call with
`p, created, err := s.resolveCaller(ctx, tx, now)` and keeps everything after it: `err` rolls
back and is answered as any handler's error is (an `*APIError` as its problem, `ErrNotEnrolled`
as 403 `tenant_not_enrolled`, anything else 500 `internal`); `set_config('hub.org_id', p.OrgID,
true)` follows; `created` gates `auth.user_provisioned` exactly as `rp.Created` did. `tx.P = p`.
A returned `Forbidden` is recorded by Hub task 14's own short transaction with that `p`.

**Every audit row comes from one function (decision f).** Hub task 14 builds a `store.AuditRow`
in two places, `Write`'s pending events and `Forbidden`'s short transaction. Both call:

```go
// auditRow is the only constructor of a store.AuditRow in package server.
func auditRow(p Principal, at time.Time, e Event) store.AuditRow {
	return store.AuditRow{At: at, Kind: e.Kind, TeamID: e.TeamID, ProjectID: e.ProjectID,
		Subject: e.Subject, ActorUserID: p.UserID, Data: e.Data,
		Via: p.Via, DelegationID: p.DelegationID}
}
```

On the feed `p.Via` is `fleet` and `p.DelegationID` is `""`. The feed is a read and writes no
event; if it ever did, `AuditEnvelope.check()` refuses `via` without `delegationId`
(`task-03.md:1336`), so the write fails closed with 500, never a v2 event with no delegation.

**`store`, the three lookups and the audit row.**

```go
// control-plane/internal/hub/store/delegation.go
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// ErrDelegationUnknown: fleet_delegation returned no row, for an unknown id or
// another tenant's. The server answers both 401 delegation_revoked, "unknown".
var ErrDelegationUnknown = errors.New("store: delegation unknown to this tenant")

// FleetDelegationRow is task 04's fleet_delegation row (task-04.md:815-818).
type FleetDelegationRow struct {
	OrgID, OwnerUserID, ProjectID string
	Class                         hubapi.DelegationClass
	Routes                        []hubapi.RouteName
	ExpiresAt                     time.Time
	RevokedAt                     *time.Time
	RevokedReason                 hubapi.RevokeReason // "" unless RevokedAt is set
	OwnerOID, OwnerName, OwnerEmail string
}

// FleetDelegation runs hub.fleet_delegation($tenant, $id). It runs before
// hub.org_id is set, which is why it is an owner function (task 04, decision g).
func FleetDelegation(ctx context.Context, tx pgx.Tx, tenantID, id string) (FleetDelegationRow, error) {
	var d FleetDelegationRow
	var class, reason string
	var routes []string
	err := tx.QueryRow(ctx, `
		SELECT org_id, owner_user_id, project_id, class::text, routes, expires_at, revoked_at,
		       coalesce(revoked_reason::text, ''), owner_entra_oid::text,
		       coalesce(owner_name, ''), coalesce(owner_email, '')
		  FROM hub.fleet_delegation($1::uuid, $2)`, tenantID, id).Scan(
		&d.OrgID, &d.OwnerUserID, &d.ProjectID, &class, &routes, &d.ExpiresAt, &d.RevokedAt,
		&reason, &d.OwnerOID, &d.OwnerName, &d.OwnerEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return FleetDelegationRow{}, ErrDelegationUnknown
	}
	if err != nil {
		return FleetDelegationRow{}, err
	}
	d.Class, d.RevokedReason = hubapi.DelegationClass(class), hubapi.RevokeReason(reason)
	for _, r := range routes {
		d.Routes = append(d.Routes, hubapi.RouteName(r))
	}
	return d, nil
}

// FleetOrg runs hub.fleet_org($tenant): the feed's org. NULL is ErrNotEnrolled.
func FleetOrg(ctx context.Context, tx pgx.Tx, tenantID string) (string, error) {
	var org *string
	if err := tx.QueryRow(ctx, `SELECT hub.fleet_org($1::uuid)`, tenantID).Scan(&org); err != nil {
		return "", err
	}
	if org == nil {
		return "", ErrNotEnrolled
	}
	return *org, nil
}

// AgentOwnedBy reports whether agentID is an agents row of userID's in the
// session's org (row-level security bounds it). An unknown id is false, not an
// error: the caller answers 403 agent_not_owner either way, so the answer never
// says whether another owner's agent exists (§5.10 rule 5).
func AgentOwnedBy(ctx context.Context, tx pgx.Tx, agentID, userID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM hub.agents
		WHERE id = $1 AND owner_user_id = $2)`, agentID, userID).Scan(&ok)
	return ok, err
}
```

`store.AuditRow` (Hub task 14, `task-14.md:159-168`) gains, after `Data`, `Via string` and
`DelegationID string`, and `Append` copies both onto the `hubapi.AuditEnvelope` it builds. It adds
no check of its own: the envelope's `check()` is the one rule (`task-03.md:1336-1341`).

**`devissuer`, the client-credentials grant (decision i; ADR-H1 §5.10 rule 8).** Hub task 11's
`Client` (`task-11.md:92-95`) gains three fields; the grant is a third row of Hub task 12's
`/token` table (`task-12.md:71-74`).

```go
// control-plane/internal/devissuer/config.go — Client, complete after this task
type Client struct {
	ClientID string `json:"clientId"`
	Name     string `json:"name"`
	// Confidential marks a workload identity. It may use client_credentials
	// only; a public client may not use it (unauthorized_client both ways).
	Confidential bool `json:"confidential"`
	// ServicePrincipals maps a tenant id to the client's service-principal oid
	// there: an app-only token's oid, as Entra writes it. A tenant absent here
	// cannot issue this client a token. Required non-empty when Confidential.
	ServicePrincipals map[string]string `json:"servicePrincipals"`
	// AppRoles maps an API's client id to the app roles assigned to this client
	// on it, carried as roles in its app-only token for that API.
	AppRoles map[string][]string `json:"appRoles"`
}

// LoadConfig gains: a Confidential client has ≥ 1 service principal, each key a
// configured tenant and each value a GUID unique across every oid in the file
// (users and service principals); every AppRoles key is a configured API and
// every role non-empty; a public client has neither map. The desktop stays the
// only public client the file names.
```

```go
// control-plane/internal/devissuer/issuer.go — addition

// MintAppToken signs the app-only token the client_credentials grant answers,
// without HTTP — for tests only, as MintAccessToken is. It refuses a tenant,
// client or API not in the configuration, and a public client.
func (i *Issuer) MintAppToken(tid, clientID, apiClientID string) (string, error)

// Env gains, last, PANACEA_APP_HUB_FLEET_CLIENT_ID: the first confidential
// client whose AppRoles for the Hub API hold "Hub.Fleet", or the line is absent.
```

| Grant | Required | Checks | Answer |
|---|---|---|---|
| `client_credentials` | `client_id`, `scope`, `client_assertion_type`, `client_assertion` | the client is configured (`invalid_client`) and confidential, and has a service principal in the path's tenant (both else `unauthorized_client`); `client_assertion_type` is `urn:ietf:params:oauth:client-assertion-type:jwt-bearer` and `client_assertion` is non-empty (else `invalid_client`); `scope` is exactly one value, `api://<API client id>/.default`, for a configured API (else `invalid_scope`) | `{"token_type":"Bearer","expires_in":<AccessTTL in s>,"ext_expires_in":<same>,"access_token":"<JWS>"}`: no refresh token, no `id_token`, no `scope` key, as Entra answers this grant |

A confidential client presenting `authorization_code` or `refresh_token`, or reaching
`/authorize`, is `unauthorized_client`, a code Hub task 12's list gains. The **app-only access
token**'s claims: `ver` `"2.0"`, `iss`, `aud` (the API's client id), `azp` (the client id), `tid`
(the path's), `oid` and `sub` (both the service principal's oid, as Entra's app tokens have them),
`roles` (the client's `AppRoles` for that API, omitted when none), `iat`, `nbf`, `exp`, `uti`.
**Never** `scp`, `name`, `preferred_username`, `email`, `acct`, `xms_edov` or `idtyp`: decision (a)
reads none of them for an app caller, and `devissuer` not writing `idtyp` keeps any offline code
from coming to depend on a claim Entra writes only when configured.

Discovery's `grant_types_supported` gains `"client_credentials"` and
`token_endpoint_auth_methods_supported` gains `"private_key_jwt"`.

`cmd/devissuer/dev-tenant.json`'s `clients` gains one entry, fictional and secret-free:

```json
{ "clientId": "77777777-7777-4777-8777-777777777777", "name": "PanaceaApp Fleet (dev)",
  "confidential": true,
  "servicePrincipals": { "11111111-1111-4111-8111-111111111111": "dddddddd-0000-4000-8000-000000000001",
                         "44444444-4444-4444-8444-444444444444": "dddddddd-0000-4000-8000-000000000002" },
  "appRoles": { "33333333-3333-4333-8333-333333333333": ["Hub.Fleet"],
                "55555555-5555-4555-8555-555555555555": ["Sandboxes.Fleet"] } }
```

The second tenant's service principal is what the cross-tenant test needs; the broker role is
task 17's (`Sandboxes.Fleet`, `task-17.md:37`), listed here so one file serves both.

**`cmd/hub`, the one variable.** Hub task 08's `config.go` (`task-08.md:37-100`):

```go
	envFleetClientID = "PANACEA_APP_HUB_FLEET_CLIENT_ID" // "": Fleet disabled (contract §12.2)

// Config gains, after AllowedClients:
	FleetClientID string // a GUID, lower-cased, or ""

// loadConfig gains: a non-empty value must be a GUID ("PANACEA_APP_HUB_FLEET_CLIENT_ID:
// not a GUID"), and must not be in AllowedClients ("…_FLEET_CLIENT_ID is also in
// …_ALLOWED_CLIENTS"), the same rule entra.New enforces (decision h), stated at the
// variable so the operator reads which one to change.
```

```go
// control-plane/cmd/hub/serve.go — the entra.Config literal gains one field
	AppCallers: fleetCallers(cfg),

// fleetCallers is nil when Fleet is disabled: no app-only token passes, as
// before this task.
func fleetCallers(cfg Config) []entra.AppCaller {
	if cfg.FleetClientID == "" {
		return nil
	}
	return []entra.AppCaller{{ClientID: cfg.FleetClientID, Role: hubapi.AppRoleFleet}}
}
```

**`export_test.go`, four test-only routes.** Hub task 13 proves its pipeline through a test-only
route (`task-13.md:20`); this task adds four beside it, registered only in the test binary:

```go
// registerFleetTestRoutes: every route runs behind Authenticate, as a real one does.
func registerFleetTestRoutes(s *Server, mux *http.ServeMux) {
	// A delegable read that answers the principal it ran as.
	mux.Handle("GET /v1/_test/whoami", Delegable(hubapi.RouteTasksRead,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p Principal
			if err := s.Read(r.Context(), func(tx *Tx) error { p = tx.P; return nil }); err != nil {
				writeError(w, err) // Hub task 14's mapping of a returned error
				return
			}
			WriteJSON(w, 200, p)
		})))
	// A delegable write that names an agent, checks it (rule 5) and emits one event.
	mux.Handle("POST /v1/_test/agent-write", Delegable(hubapi.RouteTasksCreate,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.Write(w, r, func(tx *Tx, body []byte) (WriteResult, error) {
				var b struct {
					ProjectID string `json:"projectId"`
					AgentID   string `json:"agentId"`
				}
				if err := Decode(body, &b); err != nil {
					return WriteResult{}, err
				}
				ok, err := store.AgentOwnedBy(r.Context(), tx, b.AgentID, tx.P.UserID)
				if err != nil {
					return WriteResult{}, err
				}
				if !ok {
					return WriteResult{}, Refuse(403, hubapi.CodeAgentNotOwner, "That agent is not yours.",
						map[string]string{"agentId": b.AgentID})
				}
				tx.Emit(Event{Kind: "task.created", ProjectID: b.ProjectID, Subject: "tk-00000000000000a1",
					Data: map[string]any{"test": true}})
				return WriteResult{Status: 201, Body: map[string]string{"ok": "yes"}}, nil
			})
		})))
	// A route with no name: Fleet is 403 required "route" on it.
	mux.Handle("GET /v1/_test/undelegable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = s.Read(r.Context(), func(*Tx) error { return nil })
		w.WriteHeader(204)
	}))
	// A stand-in for task 08's feed, so the exemption is tested before task 08 exists.
	mux.Handle("GET /v1/orgs/{orgId}/changes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Principal
		_ = s.Read(r.Context(), func(tx *Tx) error { p = tx.P; return nil })
		WriteJSON(w, 200, map[string]string{"orgId": p.OrgID, "via": p.Via})
	}))
}
```

`writeError`, `WriteJSON` and `Decode` stand for Hub task 14's helpers (`task-14.md:130-138`); the
implementer uses whatever names the merged `tx.go` has (**UNVERIFIED until merge**).
`task.created` is a real team kind (`task-04.md:243`), so the row passes task 04's kind check.

#### Tests to write first

**Unit tests** (no database; `go test ./internal/entra/ ./internal/hub/server/ ./internal/devissuer/
./cmd/hub/`). Each row names the mutant it kills.

| Test (file) | Asserts | Kills |
|---|---|---|
| `TestAppCallerAdmitted` (`appcaller_test.go`) | no `scp`, Fleet's `azp`, `roles` ∋ `Hub.Fleet` → `Claims.AppOnly`, `ObjectID` the service principal's oid, `Scopes` empty | `appOnly` never copied into `Claims` |
| `TestAppCallerNeedsBothFacts` | Fleet's `azp`, no role → `ErrClient`; Fleet's `azp`, the role **and** `scp: Hub.Access` → `ErrClient` (a token with a scope takes the delegated path, where Fleet is no client); the desktop's `azp`, the role, no `scp` → `ErrScope` | admitting on the `azp` alone, or on the role alone |
| `TestDelegatedTokenUnchanged` | with `AppCallers` set, the rejections of `entra_test.go:291-333` (unlisted client, scope match, app-only desktop) are unchanged and a desktop token has `AppOnly` false | a branch that skips the `scp` check for every token once `AppCallers` is set |
| `TestAppCallerConfigIsChecked` | `New` refuses a non-GUID client id, an empty role, and a client id that is also an allowed client **in another case** (`AB…` against `ab…`) | decision (h) dropped, or compared before `normGUID` |
| `TestDelegationHeaderRules` (`delegation_test.go`) | the table below, through `delegationHeader` | each row's mutant is named in the table |
| `TestIsFeedPathIsExact` | `GET /v1/orgs/org-x/changes` true; `POST` it false; `/v1/orgs/org-x/changes/`, `/v1/orgs//changes`, `/v1/orgs/org-x/changes/extra`, `/v1/orgs/org-x/projects`, `/v1/projects/p/changes` false | a prefix match, which exempts every `/v1/orgs/…` route (`task-08.md:387`) |
| `TestUserKeyIsPerDelegation` | desktop → `tid/oid`; Fleet under `dl-a` and `dl-b` → two keys; the feed → `fleet/feed` | Fleet keyed on `tid/oid`, one bucket for every member |
| `TestRouteFromIsDelegablesName` | inside `Delegable(RouteTasksRead, h)` → `(tasks.read, true)`; a bare handler → `("", false)` | `RouteFrom` defaulting to a name |
| `TestAppOnlyClaimsCarryNoAdmin` (`auth_test.go`) | a fake validator's `AppOnly` claims with `roles` ∋ `Hub.OrgAdmin` → `server.Claims.OrgAdmin` false, `Name`, `Email`, `BindEmail` "" | `OrgAdmin` read from an app token's roles |
| `TestAppTokenFromTheGrant` (`token_test.go`) | the grant → 200 with no `refresh_token`, `id_token` or `scope`; the token validates under `entra.New(… AppCallers: [{fleet, Hub.Fleet}])` with `AppOnly`; its payload has no `scp`, `name`, `email`, `idtyp` | `scp` written into an app token (the Hub then answers 401 `client`), or `idtyp` written |
| `TestAppGrantRefusals` | a public client → `unauthorized_client`; an unknown one → `invalid_client`; no assertion, or another assertion type → `invalid_client`; `scope` `…/Hub.Access` or an unknown API → `invalid_scope`; the unenrolled tenant (no service principal) → `unauthorized_client`; the Fleet client on `authorization_code` → `unauthorized_client` | the assertion check removed; a public client given an app token |
| `TestFleetdsFormIsAccepted` | the exact `url.Values` of `task-12.md:723-729` → 200 | the two forms drifting apart |
| `TestEnvNamesFleet` | `Env()`'s last line is `PANACEA_APP_HUB_FLEET_CLIENT_ID=77777777-…`; with no confidential client the line is absent | a devstack whose Hub refuses Fleet |
| `TestFleetClientIDIsChecked` (`config_test.go`) | `""` → `fleetCallers` nil; a non-GUID → the error names the variable; a value in `ALLOWED_CLIENTS` → the error names both | `serve` passing an `AppCaller` with an empty id |

`TestDelegationHeaderRules`' table:

| Caller | Header | Path | Answer | Kills |
|---|---|---|---|---|
| desktop | none | any | nil | — |
| desktop | `dl-00000000000000a1` | `/v1/_test/whoami` | 403 `forbidden`, `required: fleet` | a borrowed delegation admitted |
| desktop | `dl-…` | the feed | 403 `forbidden`, `required: fleet` | the feed exemption checked before the caller |
| Fleet | none | the feed | nil, `FleetFeed` true | — |
| Fleet | `dl-…` | the feed | 400 `bad_request`, `field: X-Panacea-Delegation` | a feed read in one member's name |
| Fleet | none | `/v1/_test/whoami` | 401 `delegation_required` | Fleet acting as nobody |
| Fleet | two `dl-…` values | `/v1/_test/whoami` | 400 `bad_request` | the first value taken, the second ignored |
| Fleet | `dl-XYZ` | `/v1/_test/whoami` | 401 `delegation_revoked`, `reason: unknown`, `delegationId: ""` | the raw value echoed |
| Fleet | `dl-00000000000000a1` | `/v1/_test/whoami` | nil, `DelegationID` set | — |
