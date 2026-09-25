### Task 32: the control plane's `/v1/auth/*`, its variables, its codes, and the wiring

**Lane C. Depends on: 31. About 2.5 hours. Contract task C1.**

The renderer's only way into sign-in: four control-plane routes (contract §9.3), every variable of
contract §12.2's control-plane table read in the existing one block (`control-plane/cmd/api/config.go:18-51`),
four new `httpx` codes, and `main.go` wiring the credential store, the authenticator and the Hub
client together. **With `PANACEA_APP_HUB_URL` unset, nothing changes for the user**: sessions,
terminals and worktrees work exactly as phase 2 built them, and only Hub-backed routes answer 503
`hub_not_configured`.

**OD-30 (ruling `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` on `main`, sections
2, 3, 7 and 11)** changes the variables and the wiring, not the routes' shapes: the issuer is
`PANACEA_APP_OIDC_*` (a Keycloak realm that brokers Entra), the Hub's audience is
`PANACEA_APP_HUB_AUDIENCE`, every `PANACEA_APP_ENTRA_*` variable is deleted, the login hint comes from
the Windows UPN, and `main.go` calls task 31's `Start` **before it serves**, so a launch with a stored
token is signed in with no browser and no click.

**Files:**
- `control-plane/internal/httpx/problem.go` — four codes, appended (modify ⚑)
- `control-plane/internal/api/auth.go` + `auth_test.go` (new)
- `control-plane/internal/api/api.go` — `Deps.Auth`, `Deps.Hub`; `a.registerAuth(guarded)` (modify ⚑: two fields, one line)
- `control-plane/internal/api/routes_audit_test.go` — `GET /v1/auth/status` in `catalogReadOnlyRoutes` (modify ⚑)
- `control-plane/cmd/api/config.go` + `config_test.go` — the variables (modify ⚑: appended to the block)
- `control-plane/cmd/api/main.go` + `main_test.go` — the wiring (modify ⚑)

#### Contract — complete

```go
// control-plane/internal/httpx/problem.go — appended at the END of the constant block, each
// with the task that added it. (The block's comment, "the complete set of error codes …
// sixteen", becomes "… sixteen, plus the Hub phase's four below".)
const (
	CodeSignedOut           = "signed_out"           // 401 nobody is signed in, or re-sign-in is required (Hub plan task 32)
	CodeHubNotConfigured    = "hub_not_configured"   // 503 PANACEA_APP_HUB_URL is unset (task 32)
	CodeLocalKeyRequired    = "local_key_required"   // 401 missing or wrong local caller key (task 37)
	CodeUpstreamUnavailable = "upstream_unavailable" // 503 details.upstream = "hub" | "entra" | "panacea" (ruling R4)
)
```

If phase 5 or 7 has already added `CodeUpstreamUnavailable` (ruling R4), this task adds the other
three and uses theirs; the spelling is fixed by R4, so the two are identical.

```go
// control-plane/internal/api/auth.go
package api

// AuthService is what the auth routes need; *signin.Authenticator satisfies it.
// Start is not here: main.go calls it once, before serving (task 31).
type AuthService interface {
	BeginSignIn(ctx context.Context) (authorizeURL string, expiresAt time.Time, err error)
	Cancel()
	Status() signin.Status
	SignOut(ctx context.Context) error
}
```

```go
// control-plane/internal/api/api.go — Deps gains, at the end of the struct:

	// Auth runs sign-in (Hub plan task 32). nil when PANACEA_APP_HUB_URL is unset.
	Auth AuthService
	// Hub is the Hub client every Hub-backed handler uses — phases 3, 4, 5 and 7
	// included. nil when PANACEA_APP_HUB_URL is unset; a handler answers
	// hubclient.WriteError(w, hubclient.ErrNotConfigured) then.
	Hub hubclient.Hub
```

The routes (contract §9.3; canonical `/v1`, and `/api/v1` through the existing alias,
`api.go:292`):

| Method | Path | `200` / `204` | Errors |
|---|---|---|---|
| POST | `/v1/auth/sign-in` | `200 {"authorizeUrl":"https://…","expiresAt":"<RFC 3339>"}` — the control plane has **already opened** the system browser (task 30); the renderer shows the URL only as a fallback link after 15 s (task 41) | 503 `hub_not_configured` (Auth nil); 409 `conflict` (`signin.ErrAlreadySignedIn`); 503 `upstream_unavailable`, `details.upstream: "entra"` (`signin.ErrIssuerUnavailable`; the wire value is unchanged until the contract's §10 is folded by task 44a) |
| POST | `/v1/auth/cancel` | `204` | — (nil Auth: 204) |
| GET | `/v1/auth/status` | `200 {"state":"signed_in","reason":"","account":{"tenantId":"…","name":"…","username":"…"},"hubConfigured":true}` — `account` is `null` unless signed in; with Auth nil, `{"state":"signed_out","reason":"","account":null,"hubConfigured":false}` | — |
| POST | `/v1/auth/sign-out` | `204` | 503 `hub_not_configured` (Auth nil) |

`GET /v1/auth/status` only reads `Status()`; it goes in `catalogReadOnlyRoutes` with that sentence.
It never triggers a redemption or a sign-in (task 31's `Start` retries offline on its own timer), so
the main listener keeps no side-effecting `GET`. `reason` may now be `"offline"` (task 31). No response
of these routes carries a token, a code, an object id or the login hint.

**The variables** (contract §12.2's control-plane table, appended to `config.go`'s block):

| Variable | Default | Rule |
|---|---|---|
| `PANACEA_APP_HUB_URL` | unset | `http://127.0.0.1:…` or `https://…`; unset disables every Hub-backed route |
| `PANACEA_APP_HUB_AUDIENCE` | `panacea-app-hub` | **keycloak profile: one of task 30's `AudienceScopes`**, else a start-up error naming the variable; an Entra-style value (`api://…/Hub.Access`, a GUID, `Hub.Access`) is refused. Replaces `PANACEA_APP_HUB_SCOPE`, which, if set, is a start-up **error** naming its replacement (not a warning: a stale Entra scope would fail every sign-in) |
| `PANACEA_APP_HUB_POLL_INTERVAL` | `5s` | minimum `1s` (task 36 reads it) |
| `PANACEA_APP_OIDC_ISSUER` | — | required with `HUB_URL`. **A realm URL**: `https://<host>[:port]/realms/<realm>`, or `http://<loopback host>[:port]/realms/<realm>` for `devissuer`; `<realm>` one segment of `[A-Za-z0-9._-]`; no trailing `/`, no further segment, no query, no fragment. Anything else is refused at start, with an error naming the variable: task 31's `New` finds the account record by it with no network, and task 30's `iss` check compares it byte for byte. An Entra authority (`https://login.microsoftonline.com/<guid>`) is refused under `keycloak` |
| `PANACEA_APP_OIDC_PROFILE` | `keycloak` | `keycloak` only in v1; `entra` is a start-up error ("no direct-Entra issuer is built yet", ruling section 5); anything else an error |
| `PANACEA_APP_OIDC_CLIENT_ID` | `panacea-app-desktop` | `[A-Za-z0-9._-]{1,64}`; a GUID is refused under `keycloak` (an Entra client id left over) |
| `PANACEA_APP_OIDC_IDP_HINT` | `entra` | `[A-Za-z0-9._-]{0,64}`; set **and empty** omits `kc_idp_hint` (ruling section 2) |
| `PANACEA_APP_OIDC_LOGIN_HINT` | unset: `signin.WindowsUPN()` | overrides the UPN (tests; a user whose Entra UPN differs from the Windows one). One `@`, no whitespace or control character, at most 256 bytes, else an error naming the variable (never echoing the value). Never logged |
| `PANACEA_APP_ENTRA_AUTHORITY`, `_ENTRA_CLIENT_ID`, `_ENTRA_REDIRECT_HOST`, `_ENTRA_DEVOPS` | — | **deleted** (OD-30). Any of them set is a start-up **error** naming its replacement, or for `_ENTRA_DEVOPS` saying Azure DevOps is a PAT (task 29); a user's leftover Entra setting must not be silently ignored |
| `PANACEA_APP_CREDSTORE` | `wincred` on Windows, else `memory` | `wincred` off Windows is a start-up error |
| `PANACEA_APP_LOCAL_KEY_FILE` | `<state dir>/local-key` | task 37 reads it |

A `PANACEA_APP_HUB_*` or `PANACEA_APP_OIDC_*` variable set **without** `HUB_URL` is a warning, not
silence — `config.go:63-82`'s rule that no variable is ever silently ignored.

**The wiring** (`main.go`, only when `HUB_URL` is set): the `Secrets` from `CREDSTORE`; a
`signin.New` with `Issuer`, `Profile`, `ClientID`, `IdPHint`, `HubAudience` from the variables,
`LoginHint` = `PANACEA_APP_OIDC_LOGIN_HINT` if set, else `signin.WindowsUPN()` (read once; `""` on
failure or off Windows, and then omitted), `OpenBrowser` = `signin.OpenSystemBrowser`, and an
`AfterSignIn` that calls `hub.Me(ctx)` and returns `me.Org.TenantID`, mapping a
`hubclient.Code(err) == "tenant_not_enrolled"` to `signin.ErrTenantNotEnrolled`; a
`hubclient.New(HubURL, HubAudience, auth, nil)`. The authenticator is built first with a closure over
a `hub` variable assigned on the next line: each needs the other, and neither is called until a
request arrives. **Then `auth.Start(ctx)`, before the listener serves** (task 31): its one
redemption is bounded by `StartTimeout`, so the renderer's first `GET /v1/auth/status` sees the
settled state; a launch never needs a sign-in click.

#### Tests to write first

`auth_test.go` uses a fake `AuthService`; `config_test.go` and `main_test.go` extend the files
phase 2 wrote.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSignInAnswersTheAuthorizeURL` | 200 with the URL and `expiresAt` in RFC 3339 | — |
| `TestSignInWhenSignedInIs409` | → 409 `conflict` | two concurrent sign-ins overwriting a refresh token |
| `TestSignInWithTheIssuerDownIs503` | `signin.ErrIssuerUnavailable` → 503 `upstream_unavailable`, `details.upstream: "entra"` | an outage reported as the user's fault |
| `TestWithoutAHubAuthSaysSo` | Auth nil: sign-in → 503 `hub_not_configured`; status → `hubConfigured: false`, `signed_out` | a desktop that offers a sign-in that cannot work |
| `TestStatusShape` | signed out → `account: null`; signed in → the three account fields and nothing else — no `objectId`, no token; `{signed_out, offline}` → `"reason":"offline"` | an identifier or token reaching the renderer; offline shown as "sign in" |
| `TestCancelAndSignOutAre204` | both → 204 | — |
| `TestAuthRoutesAreBehindTheOriginGuard` | `Origin: https://evil.example` on each → 403 `forbidden_origin` | a web page starting a sign-in in the user's name |
| `TestAuthStatusIsAuditedReadOnly` | phase 2's route audit (`routes_audit_test.go`) passes with the new `GET` listed | a `GET` added outside the audit |
| `TestHubVariablesAreAllOrNothing` | `HUB_URL` without `OIDC_ISSUER` → an error naming it; `OIDC_PROFILE`, `OIDC_CLIENT_ID`, `OIDC_IDP_HINT`, `HUB_AUDIENCE` unset → their defaults `keycloak`, `panacea-app-desktop`, `entra`, `panacea-app-hub` | a half-configured Hub failing at the first sign-in instead of at start |
| `TestHubVariablesWithoutAHubWarn` | `OIDC_CLIENT_ID` set, `HUB_URL` unset → one warning naming it | a variable silently ignored |
| `TestTheEntraVariablesAreGone` | each of `PANACEA_APP_ENTRA_AUTHORITY`, `_ENTRA_CLIENT_ID`, `_ENTRA_REDIRECT_HOST`, `_ENTRA_DEVOPS` and `PANACEA_APP_HUB_SCOPE` set → a start-up error naming its replacement (`_ENTRA_DEVOPS`: "Azure DevOps uses a personal access token") | a leftover direct-Entra setting silently ignored, so the user wonders why it has no effect |
| `TestCredstoreDefaultsAndRefusals` | Windows → `wincred`; otherwise `memory`; `wincred` off Windows → error | a Linux build pretending to persist a token |
| `TestPollInterval` | `500ms` → error; `1s` accepted | — |
| `TestTheIssuerIsARealmURL` (review M8, as OD-30 amends it) | accepted: `https://auth.dev.panacea.kpmg/realms/panacea`, `http://127.0.0.1:8740/realms/panacea-app-dev`, `http://localhost:8740/realms/x`; refused, each with an error naming `PANACEA_APP_OIDC_ISSUER`: `…/realms/panacea/` (trailing `/`), `…/realms/`, `…/realms/a/b`, `…/auth/realms/panacea` (the pre-17 `/auth` prefix), `…/realms/panacea?x=1`, `…#f`, `http://auth.dev.panacea.kpmg/realms/panacea` (not loopback), `https://login.microsoftonline.com/<guid>` and `…/<guid>/v2.0` (an Entra authority) | a desktop that signs in once and then, at every restart, cannot find its own account record; a first sign-in that fails as `exchange_failed` after the browser round trip, not at start |
| `TestTheKeycloakProfileRefusesEntraValues` | `OIDC_PROFILE=entra` → an error; `OIDC_PROFILE=Keycloak` → an error (exact); `HUB_AUDIENCE` = `api://<guid>/Hub.Access`, `<guid>`, `Hub.Access`, `panacea-app-hub panacea-app-broker` (two) → an error naming it; `OIDC_CLIENT_ID=<guid>` → an error; `panacea-app-broker` as `HUB_AUDIENCE` accepted (it is one of the four; the Hub's own validator refuses it at the first call) | an Entra scope or client id left over, sent as a Keycloak scope, failing every sign-in in the browser instead of at start; an audience list that yields a multi-audience token |
| `TestTheLoginHint` | `OIDC_LOGIN_HINT=grace@example.test` → `LoginHint` is it, `WindowsUPN` not called; unset → `WindowsUPN()`'s value (a swapped reader); `"a b@c"`, `"x"`, a 300-byte value → an error that does not contain the value; the value is in no log line | a UPN guessed or logged; a hint set by the user that is silently replaced |
| `TestSignInThroughTheControlPlane` | the real `api` handler, a real `signin` against in-process `devissuer` (keycloak profile), a fake `Hub` whose `Me` answers the dev tenant, `OIDC_LOGIN_HINT=grace@example.test` → `Start` with an empty store opens the scripted browser **itself** (no `POST /v1/auth/sign-in`), then `GET /v1/auth/status` → `signed_in` as Grace | the pieces working alone and not together; a first launch that waits for a click |
| `TestTheLaunchIsSilent` | a second `run()` over the same `Memory()` store → `GET /v1/auth/status`, the first request served, is already `signed_in`; the recording `OpenBrowser` saw **0** calls; with devissuer stopped → the first status is `{signed_out, offline}` and still 0 calls | `Start` wired after `Serve` (a flash of "signed out"); **a launch that opens the browser when a silent refresh would do, or on a network error** |
| `TestTheProcessStartsWithoutAHub` | `run()` with no Hub variables starts and serves `GET /v1/healthz` (phase 2's `main_test.go` pattern) | the Hub becoming a hard dependency of the laptop product |

#### Steps

- [ ] **Step 1:** Re-read contract §9.3, §12.2 (control plane), §4.3; the OD-30 ruling sections 2,
  3 and 7 (on `main`), which replace §12.2's `ENTRA_*` rows; `cmd/api/config.go` and
  `main.go` as they stand (other phases may have appended to them since 2026-09-23).
- [ ] **Step 2:** Tests; they fail. Then the codes, `auth.go`, the `Deps` fields, the variables,
  the wiring.
- [ ] **Step 3:** `git diff` of `api.go`, `config.go`, `problem.go`: additions only, at the ends —
  the rule that keeps sibling merges mechanical (plan §"Order and parallel lanes").
- [ ] **Step 4:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/api/ ./control-plane/cmd/api/ ./control-plane/internal/httpx/ -race -count=1   # ok
go test ./sandboxd/... ./control-plane/... -race -count=1                                                        # ok
git diff --stat HEAD~1 -- control-plane/internal/api/api.go control-plane/cmd/api/config.go control-plane/internal/httpx/problem.go
# insertions only (the "0 deletions" in each line), except problem.go's one comment sentence
go test ./control-plane/cmd/api/ -count=1 -run 'TestTheIssuerIsARealmURL|TestTheKeycloakProfileRefusesEntraValues|TestTheLaunchIsSilent' -v 2>&1 | grep -c "^--- PASS"   # 3
grep -rn "PANACEA_APP_ENTRA_\|PANACEA_APP_HUB_SCOPE\|entraauth" control-plane/cmd/api/ control-plane/internal/api/   # only the refusal rows of TestTheEntraVariablesAreGone and config.go's refusal list
```

#### What this task does not claim

- The Hub client's HTTP behaviour: tasks 33–34. Until they land, `hubclient.New` is task 04's stub
  and every Hub call answers `ErrNotConfigured`.
- The local caller key on these routes: task 37 puts every route behind it.
- Any desktop code: task 40. That includes bringing the window forward when `/v1/auth/status` turns
  `signed_in` (`unminimize`, `setFocus`, then `requestUserAttention`; ruling section 3, U-11) and the
  15-second fallback link (task 41).
- That Panacea's dev realm has the `panacea-app-desktop` client, its loopback redirect and the four
  audience scopes (ruling section 10's asks, U-2, U-3): task 43. Until then only `devissuer` is a
  working issuer.
