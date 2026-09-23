### Task 32: the control plane's `/v1/auth/*`, its variables, its codes, and the wiring

**Lane C. Depends on: 31. About 2.5 hours. Contract task C1.**

The renderer's only way into sign-in: four control-plane routes (contract §9.3), every variable of
contract §12.2's control-plane table read in the existing one block (`control-plane/cmd/api/config.go:18-51`),
four new `httpx` codes, and `main.go` wiring the credential store, the authenticator and the Hub
client together. **With `PANACEA_APP_HUB_URL` unset, nothing changes for the user**: sessions,
terminals and worktrees work exactly as phase 2 built them, and only Hub-backed routes answer 503
`hub_not_configured`.

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

// AuthService is what the auth routes need; *entraauth.Authenticator satisfies it.
type AuthService interface {
	BeginSignIn(ctx context.Context) (authorizeURL string, expiresAt time.Time, err error)
	Cancel()
	Status() entraauth.Status
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
| POST | `/v1/auth/sign-in` | `200 {"authorizeUrl":"https://…","expiresAt":"<RFC 3339>"}` | 503 `hub_not_configured` (Auth nil); 409 `conflict` (`entraauth.ErrAlreadySignedIn`); 503 `upstream_unavailable`, `details.upstream: "entra"` (`ErrEntraUnavailable`) |
| POST | `/v1/auth/cancel` | `204` | — (nil Auth: 204) |
| GET | `/v1/auth/status` | `200 {"state":"signed_in","reason":"","account":{"tenantId":"…","name":"…","username":"…"},"hubConfigured":true}` — `account` is `null` unless signed in; with Auth nil, `{"state":"signed_out","reason":"","account":null,"hubConfigured":false}` | — |
| POST | `/v1/auth/sign-out` | `204` | 503 `hub_not_configured` (Auth nil) |

`GET /v1/auth/status` only reads `Status()`; it goes in `catalogReadOnlyRoutes` with that sentence.
No response of these routes carries a token, a code or an object id.

**The variables** (contract §12.2's control-plane table, appended to `config.go`'s block):

| Variable | Default | Rule |
|---|---|---|
| `PANACEA_APP_HUB_URL` | unset | `http://127.0.0.1:…` or `https://…`; unset disables every Hub-backed route |
| `PANACEA_APP_HUB_SCOPE` | — | required with `HUB_URL` |
| `PANACEA_APP_HUB_POLL_INTERVAL` | `5s` | minimum `1s` (task 36 reads it) |
| `PANACEA_APP_ENTRA_AUTHORITY` | — | required with `HUB_URL`; `https://`, or `http://` to a loopback host only |
| `PANACEA_APP_ENTRA_CLIENT_ID` | — | required with `HUB_URL`; a GUID |
| `PANACEA_APP_ENTRA_REDIRECT_HOST` | `127.0.0.1` | `127.0.0.1` or `localhost` |
| `PANACEA_APP_ENTRA_DEVOPS` | `false` | ADR-H1 Q5: `true` makes Azure DevOps use the Entra token (task 31's `DevOps.Enabled`). **Default false** until IT grants the desktop client Azure DevOps consent: without it every ADO token request fails `invalid_scope` |
| `PANACEA_APP_CREDSTORE` | `wincred` on Windows, else `memory` | `wincred` off Windows is a start-up error |
| `PANACEA_APP_LOCAL_KEY_FILE` | `<state dir>/local-key` | task 37 reads it |

A `PANACEA_APP_HUB_*` or `PANACEA_APP_ENTRA_*` variable set **without** `HUB_URL` is a warning, not
silence — `config.go:63-82`'s rule that no variable is ever silently ignored.

**The wiring** (`main.go`, only when `HUB_URL` is set): the `Secrets` from `CREDSTORE`; an
`entraauth.New` whose `AfterSignIn` calls `hub.Me(ctx)` and returns `me.Org.TenantID`, mapping a
`hubclient.Code(err) == "tenant_not_enrolled"` to `entraauth.ErrTenantNotEnrolled`; a
`hubclient.New(HubURL, HubScope, auth, nil)`. The authenticator is built first with a closure over
a `hub` variable assigned on the next line: each needs the other, and neither is called until a
request arrives.

#### Tests to write first

`auth_test.go` uses a fake `AuthService`; `config_test.go` and `main_test.go` extend the files
phase 2 wrote.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSignInAnswersTheAuthorizeURL` | 200 with the URL and `expiresAt` in RFC 3339 | — |
| `TestSignInWhenSignedInIs409` | → 409 `conflict` | two concurrent sign-ins overwriting a refresh token |
| `TestSignInWithEntraDownIs503` | → 503 `upstream_unavailable`, `details.upstream: "entra"` | an outage reported as the user's fault |
| `TestWithoutAHubAuthSaysSo` | Auth nil: sign-in → 503 `hub_not_configured`; status → `hubConfigured: false`, `signed_out` | a desktop that offers a sign-in that cannot work |
| `TestStatusShape` | signed out → `account: null`; signed in → the three account fields and nothing else — no `objectId`, no token | an identifier or token reaching the renderer |
| `TestCancelAndSignOutAre204` | both → 204 | — |
| `TestAuthRoutesAreBehindTheOriginGuard` | `Origin: https://evil.example` on each → 403 `forbidden_origin` | a web page starting a sign-in in the user's name |
| `TestAuthStatusIsAuditedReadOnly` | phase 2's route audit (`routes_audit_test.go`) passes with the new `GET` listed | a `GET` added outside the audit |
| `TestHubVariablesAreAllOrNothing` | `HUB_URL` without each of `HUB_SCOPE`, `ENTRA_AUTHORITY`, `ENTRA_CLIENT_ID` → an error naming it | a half-configured Hub failing at the first sign-in instead of at start |
| `TestHubVariablesWithoutAHubWarn` | `ENTRA_CLIENT_ID` set, `HUB_URL` unset → one warning naming it | a variable silently ignored |
| `TestCredstoreDefaultsAndRefusals` | Windows → `wincred`; otherwise `memory`; `wincred` off Windows → error | a Linux build pretending to persist a token |
| `TestPollIntervalAndRedirectHost` | `500ms` → error; `localhost` accepted; `0.0.0.0` refused | — |
| `TestSignInThroughTheControlPlane` | the real `api` handler, a real `entraauth` against in-process `devissuer`, a fake `Hub` whose `Me` answers the dev tenant → POST sign-in, the scripted browser, `GET /v1/auth/status` → `signed_in` as Grace | the pieces working alone and not together |
| `TestTheProcessStartsWithoutAHub` | `run()` with no Hub variables starts and serves `GET /v1/healthz` (phase 2's `main_test.go` pattern) | the Hub becoming a hard dependency of the laptop product |

#### Steps

- [ ] **Step 1:** Re-read contract §9.3, §12.2 (control plane), §4.3; `cmd/api/config.go` and
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
```

#### What this task does not claim

- The Hub client's HTTP behaviour: tasks 33–34. Until they land, `hubclient.New` is task 04's stub
  and every Hub call answers `ErrNotConfigured`.
- The local caller key on these routes: task 37 puts every route behind it.
- Any desktop code: task 40.
