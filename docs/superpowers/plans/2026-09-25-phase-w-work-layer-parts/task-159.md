### Task 159: apps: Entra OAuth apps that act as a person, their manifests, admin approval

W5, stream WH, seam **SW.3** extended (who may write what: an app is a principal kind whose rights
are its person's, narrowed by its manifest, and never a fact state or `done`). Depends on task 145
(the `app` actor kind in the guard; `work_apps`; the *approved app acting for M* fixture). Types
from task 143's part (`AppManifest`, `App`, `DecideAppRequest`, `ActorApp`, `CodeAppNotApproved`,
`task-143.md:400-424`). Hub anchors read in `feat/hub` at `3df9fab8`: the contract (cited
`hub-contract:N`), and Hub task 13's part (the auth pipeline, `H13:N`). `server/` and `store/` are
**planned, not on `feat/hub`**, and every path in them is **UNVERIFIED until merge**. 05 §1 L-250,
L-244 (its API half; the MCP half is task 161).

**The scope predates OD-30, and this part follows OD-30.** The scope names `entra.Validator` and an
`azp` allowlist of the desktop's Entra client id (`hub-contract:440`, `:492`). hub-contract §17
(OD-30, 2026-09-25) replaced both: the Hub validates with `oidcauth.Validator` under the `keycloak`
profile, and `AllowedClients` is `panacea-app-desktop`, a client of the Keycloak realm that brokers
to Entra (§17, *"§5.1, from token to principal"*). The ruling says third-party apps are **realm
clients** under its rule K6 (`docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md:543-549`
on the main tree): each carries a person, gets the `oid` and `tid` mappers, and only the
`panacea-app-hub` audience scope. So an "Entra OAuth app" here is a realm client whose sign-in
still ends at Entra; its person is still `(tid, oid)`. SW.21's field keeps its name
`entraClientId` (task 143 fixed it), and it holds the **realm client id** the token's `azp` carries.

An app is how a team's own tool (a report, a bot, a script with a user) calls the Hub as the person
using it. It never acts alone: a token with no person in it is refused before anything else, and an
app sees and writes only what its person could, less what its manifest leaves out.

**The one thing this task must not get wrong** is letting a token in because of what it claims to
be. The client id in `azp` is accepted only after the token has passed every other check the
desktop's token passes, and only when an org admin has approved that exact client id **in the
token's own org**. The lookup is made on every request; a revoke takes effect on the next call.

Six decisions the scope left open, made here.

1. **The hook is inside the validator, after every other check.** An `azp` refusal can only be
   overridden once the signature, `iss`, `typ`, `aud` (with no second `panacea-app-*` audience),
   `scope`, `exp` and `nbf` have all passed and the token has `tid` and `oid`. That order lives in
   `oidcauth` (env task 17's package, which the Hub takes unchanged, H-plan `:229`), so the hook is a
   **proposed edit to `oidcauth`** for the `feat/env` owner, and its use in `auth.go` a proposed
   edit for the `feat/hub` owner (as W0's proposals are). Never a second validation pass.
2. **An app never creates a user.** Step 5's `hub.resolve_principal` inserts a user on first sight
   (`hub-contract:305-308`). An app's token instead calls a new `hub.find_principal(tid, oid)` (no
   insert; task 145's migration adds it); a person the Hub has never seen is 403 `forbidden`,
   `details.required: "desktop_sign_in"`. A person's first contact with the Hub is their own.
3. **An app is never an org admin.** `Claims.OrgAdmin` is forced `false` for an app's token, whatever
   its `roles` say, and `BindEmail` is already `""` (§17: only `azp == panacea-app-desktop` binds an
   invite). An admin's power is exercised by the admin, in the desktop.
4. **Every route is classified for apps, and the default is never.** A table in `server/work_apps.go`
   gives each `/v1` pattern `read`, `write` or `never`; `work.read` reaches `read` routes, `work.write`
   adds `write` routes, and a meta-test fails on an unclassified pattern. `never` holds erase, audit,
   team settings and membership, webhooks (task 156), keys (task 160), apps (this task), MCP
   connectors (task 161) and every org-admin route.
5. **Reserved client ids are refused at registration.** A manifest whose `entraClientId` starts
   `panacea-app-` is 400: the desktop's and the service clients' ids (`panacea-app-desktop`,
   `-gateway`, `-fleet`, `-hub-pages`) cannot be registered as an app, so approving one can never
   change how those clients are treated.
6. **Revoked is final.** A revoked app is not re-approved; a new registration (a new `oa-` id) is
   decided afresh. One client id has at most one `pending` or `approved` app per org.

**Files:**

- `control-plane/internal/hub/server/work_apps.go` (new): the four routes; the route classification
  table `appAccess`; `appGate`, the per-request check after the principal step
- `control-plane/internal/hub/store/work_apps.go` (new), `store/work_apps_integration_test.go` (new)
- `control-plane/internal/hub/server/work_apps_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/auth.go` ⚑ (Hub task 13's file, `H13:124`): the hook's use,
  and `Claims` gains `AppID` — a **proposed Hub-contract edit** for the `feat/hub` owner
- `control-plane/internal/oidcauth/` ⚑ (env task 17 step 0's package; not on disk at `3df9fab8`):
  the `ExtraClients` hook — a **proposed edit** for the `feat/env` owner, **UNVERIFIED until merge**
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's file,
  its decision 7; the scope's `work_matrix_rows.go`): the four routes' rows and the app fixture rows

**Interfaces:**

- Produces: `POST /v1/apps`, `GET /v1/apps`, `GET /v1/apps/{appId}`, `POST /v1/apps/{appId}/decision`;
  `oidcauth.Config.ExtraClients` (proposed); `server.Claims.AppID`; `appAccess`; the settings
  `hub.work_actor_kind = 'app'` and `hub.work_actor_app` on every app-made transaction
- Consumes: task 143's types and codes; task 145's guard (`app` is refused every state column write)
  and its `hub.find_principal`, `hub.app_for_client`; Hub task 13's `Authenticate`, `Claims`
- Consumed by: task 160 (keys reuse `appAccess`'s `never` list and the `app` rate bucket's key
  shape), task 161 (the MCP half of L-244 reuses the decision route), task 173 (the desktop's apps
  admin), task 158 (the README's auth section)

#### The `oidcauth` hook (proposed; complete as an API)

```go
// In oidcauth.Config (env task 17's type), one added field:

	// ExtraClients, when set, is asked about a token whose azp is in neither
	// AllowedClients nor ServiceClients, and ONLY after every other check has
	// passed: signature, iss, typ, aud (panacea-app-hub and no other
	// panacea-app-* audience), scope, exp, nbf, and tid and oid both present. It
	// returns ok=true to accept the token as a person's; the Claims then carry
	// Client = azp and ExtraClient = true. ok=false is ErrClientNotApproved (new,
	// and returned ONLY when every other check passed; the Hub answers it 403
	// app_not_approved); an error is returned wrapped, and the Hub answers it 503
	// unavailable. Without the hook set, behaviour is exactly today's ErrClient.
	// A token with no tid or no oid never reaches it.
	ExtraClients func(ctx context.Context, tid, azp string) (ok bool, err error)

// ErrClientNotApproved: a valid person's token from a client that is neither
// allowed nor approved.
var ErrClientNotApproved = errors.New("oidcauth: client not approved")

// And in oidcauth.Claims, one added field:

	ExtraClient bool // accepted through ExtraClients; false for the desktop and services
```

`oidcauth`'s check order is env task 17's; the requirement on it is only that `ExtraClients` is
consulted last. `Validate` gains a `ctx` if it has none (the lookup is a database read). An app-only
token (`roles` and no `scp`/`scope`) fails `ErrScope` before the hook, as it does today
(`hub-contract:493`), so an app with no person is refused 401 and never looked up.

#### `server/auth.go` ⚑ (the added lines)

```go
// In AuthConfig: the hook's Hub side. serve.go sets it to st.ApprovedAppForClient.
	AppClients func(ctx context.Context, tid, azp string) (appID string, scopes []string, err error)

// In Claims:
	AppID     string   // "" unless ExtraClient; the oa- id of the approved app
	AppScopes []string // its manifest's scopes, read on THIS request

// In Authenticate, step 3: the validator's ExtraClients is a closure over
// cfg.AppClients that records appID and scopes for this request; step 3 then sets
// Claims.AppID, Claims.AppScopes, Claims.OrgAdmin = false (decision 3) and
// BindEmail = "" for an ExtraClient token. Step 4's per-user limit keys an app's
// call as "app/<appId>/<tid>/<oid>" (task 160's "app" bucket).
```

The lookup is **not cached**: `store.ApprovedAppForClient` runs `SELECT * FROM
hub.app_for_client($1, $2)` (a `SECURITY DEFINER` function, task 145: the org by `entra_tenant_id`,
then the one `approved` row with that client id, returning `app_id` and `scopes`; no row, no
access). One indexed read per app request; desktop requests never make it.

#### `server/work_apps.go`: the gate and the classification (complete)

```go
package server

// Access is what an app scope may reach on one route.
type Access int

const (
	AccessNever Access = iota // the zero value: an unlisted pattern is never reachable by an app
	AccessRead                // work.read
	AccessWrite               // work.write (which also holds work.read)
)

// appAccess classifies every /v1 pattern. TestEveryPatternIsClassifiedForApps fails
// on a pattern of server.RoutePatterns() that is missing here, so a new route is
// closed to apps until someone decides otherwise. Examples of each class:
var appAccess = map[string]Access{
	"GET /v1/projects/{projectId}/issues":  AccessRead,
	"GET /v1/issues/{issueId}":             AccessRead,
	"GET /v1/changes":                      AccessRead,
	"POST /v1/projects/{projectId}/issues": AccessWrite,
	"PATCH /v1/issues/{issueId}":           AccessWrite,
	"POST /v1/issues/{issueId}/comments":   AccessWrite,
	"POST /v1/issues/{issueId}/state":      AccessWrite, // the guard still refuses every fact state and done
	"POST /v1/issues/{issueId}/erase":      AccessNever,
	"POST /v1/teams/{teamId}/webhooks":     AccessNever,
	"POST /v1/apps":                        AccessNever,
	// … every other pattern, explicitly …
}

// appGate runs after the principal step for a request whose Claims.AppID is set.
// A route needing more than the app's scopes → 403 forbidden, details.required
// "work.read" or "work.write" (or "never" for AccessNever). Otherwise the request
// proceeds, and the handler's transaction runs with hub.work_actor_kind = 'app'
// and hub.work_actor_app = AppID set by Write's prologue (Hub task 14's Tx), so
// task 145's guard and task 167's actor both see it.
func appGate(pattern string, c Claims) *APIError {
	need := appAccess[pattern]
	switch {
	case need == AccessNever:
		return forbidden("never")
	case need == AccessRead && hasScope(c.AppScopes, "work.read"):
		return nil
	case need == AccessWrite && hasScope(c.AppScopes, "work.write"):
		return nil
	case need == AccessRead:
		return forbidden("work.read")
	default:
		return forbidden("work.write")
	}
}
```

`POST /v1/issues/{issueId}/state` is `AccessWrite` on purpose: an app may move an issue between the
**hand** states its person could (`backlog`, `todo`, `canceled`, and so on), and the database guard
of task 145 refuses `app` every fact state (`started`, `in_review`, `landed`) and `done` whatever the
cause (05 §5.4 W5). The test proves both halves.

`GET /v1/changes` is `read`: an app sees the feed its person sees (hub-contract §7.11's rule is by
user), which is the same metadata a webhook sends.

#### The routes and their wire

```jsonc
// POST /v1/apps                                  any member of the org (a person, not an app or key)
{ "manifest": { "name": "Release notes to SharePoint", "entraClientId": "notes-exporter",
    "publisher": "KPMG AU Delivery Tools", "contact": "tools@kpmg.com.au",
    "scopes": ["work.read"], "redirectUris": ["https://notes.example.kpmg.com.au/callback"],
    "webhookUrl": null } }
// → 201 App, state "pending", registeredBy the caller, decidedBy null
// name 1-80; publisher and contact 1-120; entraClientId ^[A-Za-z0-9._-]{1,100}$ and not
//   "panacea-app-…" (decision 5); scopes exactly ["work.read"] or ["work.read","work.write"];
//   redirectUris 1-10, each https with a host and no fragment; webhookUrl null or https
//   → each failure 400 bad_request naming the field
// the same client id pending or approved in the org → 409 conflict, details.appId

// GET /v1/apps?state=&after=&limit=  → 200 Page[App]: an org admin sees every app; a member
//   sees approved apps and the ones they registered
// GET /v1/apps/oa-…                  → 200 App, on the same rule; otherwise 404 app_not_found

// POST /v1/apps/oa-…/decision        org admin only (hub-contract §5.1's org admin, §17's role)
{ "approve": true, "reason": "Reviewed with the owning team." }
// pending + approve true  → 200 App "approved"; pending + false → "revoked"
// approved + false        → "revoked" (every token of it refused from the next request)
// approved + true, or revoked + anything → 409 conflict (decision 6); reason 1-500, required
```

`app_not_found` is not in task 143's list at this writing (`task-143.md:778` adds
`webhook_not_found` only); it is a gap for that part, as task 156's was.

**Events**, audit-only (`feed = false`, the class Hub task 26 gives `auth.*`, `H26:35-37`), because
an app is an org's record and the feed shows team and project events only: `app.registered
{state}`, `app.decided {state}`, with the actor the person. Whether Hub task 14's `Tx.Emit` takes an
audit-only kind by a registry flag is **UNVERIFIED**; the part follows whatever Hub task 26 used for
`auth.user_provisioned`.

#### The store (`store/work_apps.go`, signatures complete)

What this part needs from task 145's `work_apps` (its file wins on names): `id`, `org_id`,
`client_id`, `manifest jsonb`, `scopes text[]`, `state`, `registered_by`, `decided_by NULL`,
`decided_at NULL`, `reason NULL`, a partial unique index on `(org_id, client_id) WHERE state IN
('pending','approved')`, RLS on `org_id`, and `hub_app` granted `SELECT, INSERT, UPDATE (state,
decided_by, decided_at, reason)` only; plus the two `SECURITY DEFINER` functions
`hub.app_for_client(tid, client_id) RETURNS TABLE (org_id text, app_id text, scopes text[])` and
`hub.find_principal(tid, oid) RETURNS TABLE (org_id text, user_id text)`, neither of which inserts.

```go
package store

func RegisterApp(ctx context.Context, tx pgx.Tx, id, registeredBy string, m hubapi.AppManifest, at time.Time) (hubapi.App, error)
func ListApps(ctx context.Context, q Querier, viewer string, orgAdmin bool, state, after string, limit int) (hubapi.Page[hubapi.App], error)
func GetApp(ctx context.Context, q Querier, id, viewer string, orgAdmin bool) (hubapi.App, bool, error)
func LockApp(ctx context.Context, tx pgx.Tx, id string) (hubapi.App, error) // FOR UPDATE
func DecideApp(ctx context.Context, tx pgx.Tx, id, decidedBy, state, reason string, at time.Time) (hubapi.App, error)
// ApprovedAppForClient is AuthConfig.AppClients: hub.app_for_client, no cache.
func (s *Store) ApprovedAppForClient(ctx context.Context, tid, azp string) (appID string, scopes []string, err error)
// FindPrincipal is decision 2's step 5 for an app token: hub.find_principal, no insert.
func FindPrincipal(ctx context.Context, tx pgx.Tx, tid, oid string) (ResolvedPrincipal, bool, error)
```

#### Tests to write first

`server/work_apps_integration_test.go` (`//go:build integration`). `devissuer` (Hub task 11's
`keycloak`-profile issuer) mints tokens for a second client `notes-exporter` with the same `oid`
and `tid` mappers as the desktop's (K6), so no token is forged by hand. Org `o1` with admin Ada,
member Linus, team `tm-a` (Linus a member), and a second org `o2`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestUnregisteredClientIsRefused` | Linus's token from `notes-exporter`, no app → 403 `app_not_approved` on `GET /v1/me` | any client accepted |
| `TestPendingRefusedApprovedActsAsItsPerson` | registered, `pending` → 403 `app_not_approved`; Ada approves → the same call 200; `GET` of a project in a team Linus is not in → 404, exactly as for Linus's desktop token | app sees more than its person |
| `TestRevokeTakesEffectOnTheNextCall` | approved; a call 200; Ada revokes; the very next call → 403 `app_not_approved` | revocation cached |
| `TestReadScopeCannotWrite` | a `work.read` app: `GET` issues 200; `POST` an issue → 403, `details.required "work.write"` | scope ignored |
| `TestAppCannotCloseOrFakeAFact` | a `work.write` app: `todo` → `canceled` by hand 200; `landed` with `land_trailer` → refused; `done` with `evidence_observed` → refused (task 145's guard, the database's error surfaced as 403) | an app closes an issue |
| `TestAppOnlyTokenIs401` | a client-credentials token for `notes-exporter` (roles, no scope, no `oid`) → 401 `scope`, and `app_for_client` was never called (a counting wrapper) | an app as a free agent |
| `TestOnlyAnOrgAdminDecides` | Linus decides → 403; an admin of `o2` deciding `o1`'s app → 404; Ada → 200 | a missing role row |
| `TestAppWritesCarryTheAppID` | an app's comment → its event's actor has `kind "app"`, `userId` Linus, `appId` the `oa-` id (read through `/v1/changes` once task 167 lands; before it, from the audit row's actor columns) | app acts invisibly (L-239) |
| `TestAppIsNeverOrgAdmin` | Ada's own token from an approved app → `POST /v1/apps/{id}/decision` 403 `never`; `GET /v1/apps` lists as a member would | an app wielding its person's admin role |
| `TestAppNeverProvisionsAUser` | an approved app, a person `oid` the Hub has never seen → 403 `desktop_sign_in`; `users` count unchanged | an app creating users |
| `TestReservedClientIDRefused` | registering `panacea-app-desktop` or `panacea-app-gateway` → 400 naming `manifest.entraClientId` | the desktop's client id registered as an app |
| `TestAppInAnotherOrgIsNotApproved` | `notes-exporter` approved in `o1`; a token of a person of `o2` from it → 403 `app_not_approved` | approval leaking across tenants |
| `TestEveryPatternIsClassifiedForApps` | every `/v1` pattern of `RoutePatterns()` has an `appAccess` entry; every `AccessNever` pattern named in decision 4's list is `AccessNever` | a new route open to apps by default |
| `TestAppRoutesMatrix` | the four routes' rows: `POST` ✓ for org members, 403 for the app, key and integration fixtures; `decision` ✓ for the org admin only | a missing role row |

`oidcauth`'s own unit tests (env task 17's file, the proposed edit) gain two rows: a token failing
any earlier check with an unknown `azp` returns that earlier error and never calls the hook (kills
"the hook as a bypass"); a token passing every check with an unknown `azp` and a hook answering
false returns `ErrClientNotApproved` (kills "not approved reported as a bad token").

#### Steps

- [ ] **Step 1: Check the base.** `oidcauth` merged from env task 17 with the `keycloak` profile; Hub
  task 13's `auth.go`; task 145's `work_apps`, `hub.app_for_client`, `hub.find_principal` and the
  guard's `app` branch. If the two proposed edits are not accepted, stop at step 2 and report: the
  routes may land, but no app token can pass until the hook exists.
- [ ] **Step 2: The proposals.** Write the `oidcauth` hook and its two unit rows as a proposal for
  the `feat/env` owner, and the `auth.go` lines as one for the `feat/hub` owner, each citing this
  part. Add a realm client `notes-exporter` to `devissuer`'s test realm (Hub task 11's fixture)
  with K6's two mappers and the `panacea-app-hub` optional scope only.
- [ ] **Step 3: The integration tests**; they fail. Then the store, the routes, `appAccess` for
  every pattern, `appGate` in the middleware after the principal step, until green.
- [ ] **Step 4: Run the neighbours**: Hub task 13's auth tests (the desktop path unchanged), task
  145's guard rows, task 156's webhook matrix (apps 403). Commit by pathspec.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(UnregisteredClientIsRefused|PendingRefusedApprovedActsAsItsPerson|RevokeTakesEffectOnTheNextCall|ReadScopeCannotWrite|AppCannotCloseOrFakeAFact|AppOnlyTokenIs401|OnlyAnOrgAdminDecides|AppWritesCarryTheAppID|AppIsNeverOrgAdmin|AppNeverProvisionsAUser|ReservedClientIDRefused|AppInAnotherOrgIsNotApproved|EveryPatternIsClassifiedForApps|AppRoutesMatrix)$' \
  2>&1 | grep -c '^--- PASS'                                                                  # 14
go test ./control-plane/internal/oidcauth/ -race -count=1 -run 'ExtraClient' -v 2>&1 | grep -c '^--- PASS'   # 2
go test ./control-plane/internal/hub/... -tags integration -race -count=1                    # every package ok
```

Expected: `14`, `2`, every package `ok`.

#### What this task does NOT claim

- **Not that any tenant has consented to a third-party app.** An app's users sign in through the
  realm to Entra; whether Entra or the realm's admin must consent for a new client is a tenant and
  realm act (**UNVERIFIED**), separate from the Hub's approval here. Under OD-30 the realm client is
  created by the realm's admin (ruling K6), which is Panacea's realm until phase 8's exists.
- **Not that the realm issues app tokens today.** No third-party client exists in Panacea's realm;
  ruling K6 says phase W names them "when they are asked for".
- **Not the MCP half of L-244** (task 161), nor keys (task 160).
- **Not that the two proposed edits are accepted** by the `feat/env` and `feat/hub` owners; until
  they are, `hub-contract:36` and §17 hold: only the desktop's client reaches `/v1`.
- **Not rate limits for apps**: the `app` bucket is task 160's.

**Parity rows:** closes **L-250** and the API half of **L-244** (its MCP half with task 161).

#### Gaps found while writing this part

1. The scope predates OD-30: `entra.Validator` and an Entra client id are now `oidcauth` under the
   `keycloak` profile and a realm client id. SW.21's `AppManifest.entraClientId` keeps its name but
   holds a realm client id; renaming it `clientId` is a question for task 143's part.
2. `app_not_found` is not in task 143's code list.
3. Task 145 must add `hub.app_for_client`, `hub.find_principal`, and the `work_apps` columns and
   partial unique index above; none is in its scope's text.
4. `oidcauth` is env task 17's package (`feat/env`), so the hook is a proposal to a second owner
   the scope does not name.
5. Whether Hub task 14's `Tx.Emit` has an audit-only flag for `app.*` kinds is **UNVERIFIED**.

#### Addendum (fix batch b02, round 2; review N-I4): an app cannot set a check

Appended so no line above moves. SW.3's check rule (task 07's addendum) lets only a person's own
revision add or change a serves pair's `check`, the command the lander's control plane runs and
attests to (tasks 23, 23b). This task gives a `work.write` app the create route as its person
(`:157`), and until this round the create and edit routes passed a literal `true`. Tasks 08 and 09
now read `byPerson` from the transaction's `hub.work_actor_kind` (`store.WriterIsPerson`, task
08's addendum), which `appGate`'s transaction sets to `app` before the handler runs (`:170-172`),
so this task writes no rule of its own; it proves the door is shut. One row joins the table above,
and the acceptance's first count goes from `14` to `15`, its `-run` list gaining `AnAppCannotSetACheck`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnAppCannotSetACheck` | a `work.write` app: create an issue serving `c1` with `check: "true"` → 422 `bad_request` naming `serves[0].check`, no number taken; a revision adding a check to Linus's issue → 422 naming `content.serves[0].check`, head unchanged; the same revision from Linus's desktop token → 201; the app's revision carrying that check unchanged → 201 | an app whose texts read `H` adding a command a team member's control plane runs in their name (review S03-I5's failure, through the door task 07 says is shut) |
