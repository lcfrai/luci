### Task 37: the local caller key — control-plane side (corrections A3, A4)

**Lane C. Depends on: 32. About 2 hours. Contract task C3.**

Once the Hub exists, a process that reaches `127.0.0.1:8730` can act on shared org data **as the
signed-in user**, not only drive local sessions (ADR-H1 §5.4). Loopback bind, the Host guard and
the Origin allowlist stop a web page; they do not stop **another OS user**, or **a sandboxed agent
that can reach the host's loopback**. So the control plane mints a random key at every start,
writes it to a file only the user can read, and requires it on every route but two. It does not
stop another process of the same OS user — nothing short of an OS peer check does (ADR-H1 R4).

**Two corrections make it work.** **A3**: the exempt health routes are `GET /v1/healthz` and `GET
/api/v1/healthz` (`control-plane/internal/api/api.go:303-306`); there is no `/healthz`. **A4**: the
CORS preflight today allows only `Content-Type` (`control-plane/internal/httpx/origin.go:168-171`),
so an `Authorization` header would fail **every** request at the preflight; it must also allow
`Authorization`, and the key check must sit **inside** the origin middleware so an allowed
preflight is still answered 204 before any key is asked for (`origin.go:231-237`).

**Files:**
- `control-plane/internal/localkey/localkey.go` + `localkey_test.go` (new)
- `control-plane/internal/httpx/origin.go` + `origin_test.go` — `corsHeaders` (modify ⚑: one constant)
- `control-plane/internal/api/api.go` — `Deps.LocalKey`; the wrap (modify ⚑)
- `control-plane/internal/api/localkey_test.go` — every route walked (new)
- `control-plane/cmd/api/main.go` — mint at start (modify ⚑)

#### Contract — complete

```go
// control-plane/internal/localkey/localkey.go

// Package localkey is the control plane's local caller key (contract §9.4).
package localkey

// Key is 32 random bytes, base64url without padding: 43 characters. Its String
// and GoString redact; it is never logged and never sent in a response.
type Key struct{ /* unexported value */ }

func (k Key) String() string   // "localkey.Key{redacted}"
func (k Key) GoString() string // the same

// Mint makes a new key and writes it to path — creating the directory (0700)
// and the file (0600; on Windows the file inherits the profile directory's ACL,
// which admits only the user, SYSTEM and Administrators) — by writing a temp
// file beside it and renaming it over any previous one. A new key every start:
// nothing to rotate, nothing stale to trust.
func Mint(path string) (Key, error)

// Middleware requires "Authorization: Bearer <key>" on every request exempt
// does not excuse. Missing or wrong → 401 local_key_required, compared with
// crypto/subtle.ConstantTimeCompare, with WWW-Authenticate: Bearer realm="panacea-app-local".
func Middleware(k Key, exempt func(*http.Request) bool, next http.Handler) http.Handler

// Exempt is the control plane's exemption: GET …/v1/sessions/{id}/attach, on
// both the canonical and the /api alias. A WebSocket from the webview cannot
// carry a header; attach reaches a local PTY, never the Hub, and keeps phase 2's
// defences (loopback, Host guard, upgrade-gated writes). The health routes are
// exempt by being mounted outside the guard (api.go:303-306), not by this func.
func Exempt(r *http.Request) bool
```

```go
// control-plane/internal/httpx/origin.go — the one constant changes (correction A4):
const (
	corsMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type, Authorization"
	corsMaxAge  = "600"
)
```

**Where the wrap goes** (`api.go`, `New`): `root.Handle("/", d.Origin.Middleware(localkey.Middleware(d.LocalKey, localkey.Exempt, guarded)))`
— inside the origin guard, outside the routes. `/v1/healthz` and `/api/v1/healthz` stay on `root`,
outside both.

**The file's path** is `PANACEA_APP_LOCAL_KEY_FILE`, default `statepath.Dir("auth")` +
`/local-key` — so `%LOCALAPPDATA%\PanaceaApp\state\auth\local-key` on Windows, following
`control-plane/internal/statepath/statepath.go:30-45`'s ladder. Task 38's Tauri command walks the
**same** ladder; both sides pin the same example paths in their tests.

The `client.ts:6-8` comment (*"no Authorization header … before Phase 8"*) is replaced by task 38.

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMintWritesAFreshKey` | the file holds 43 base64url characters; a second `Mint` replaces it with a different key; no temp file is left beside it | a key reused across restarts |
| `TestTheKeyPathFollowsStatepath` | with `PANACEA_APP_STATE_DIR=/tmp/s` → `/tmp/s/auth/local-key`; with only `LOCALAPPDATA=C:\Users\u\AppData\Local` → `C:\Users\u\AppData\Local\PanaceaApp\state\auth\local-key`; `PANACEA_APP_LOCAL_KEY_FILE` overrides both — the same examples task 38's Rust test pins | a control plane and a shell looking in two places |
| `TestTheKeyFileIsPrivate` (non-Windows) | mode `0600`, directory `0700` | a key any local user can read |
| `TestTheKeyNeverPrints` | `%v`, `%+v`, `%#v`, `slog` of a `Key` → `redacted`; no response body in any test below contains it | the key in a log |
| `TestEveryGuardedRouteNeedsTheKey` | for every pattern in `API.RegisteredPatterns()` except the exemptions, a request with ids filled in and no key → 401 `local_key_required`; with the key → not 401 | a route added later that forgets the key (the §10a audit's own technique) |
| `TestAWrongKeyIs401` | `Bearer <another key>`, `Bearer`, `Basic …` → 401 | a prefix match or an empty-string match |
| `TestHealthzNeedsNoKey` | `GET /v1/healthz` and `GET /api/v1/healthz` → 200 without a key (A3) | a supervisor restarting a healthy control plane |
| `TestAttachIsExempt` | `GET /v1/sessions/s-3f2a91c0/attach` and the `/api` alias without a key → not 401 (phase 2's attach checks answer instead); `POST` to that path → 401 | the WebSocket broken; a write smuggled under the exemption |
| `TestAPreflightNeedsNoKey` | `OPTIONS /v1/sessions` with an allowed `Origin` → 204 with `Access-Control-Allow-Headers: Content-Type, Authorization` (A4) | every request failing at the preflight — the failure A4 exists to prevent |
| `TestADisallowedOriginIsStillRefusedFirst` | a bad `Origin` with a good key → 403 `forbidden_origin` | the key replacing the origin check instead of adding to it |

#### Steps

- [ ] **Step 1:** Re-read contract §9.4; `api.go:292-310` and `origin.go:164-240` as they stand.
- [ ] **Step 2:** Tests; they fail. Then `localkey.go`, the constant, the wrap, `main.go`'s `Mint`
  before the listener starts (a key that cannot be written stops the process with that sentence).
- [ ] **Step 3:** **The desktop now gets 401 on every call until task 38 lands.** Land 37 and 38
  together — the orchestrator merges both, or neither, into `feat/hub`.
- [ ] **Step 4:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/localkey/ ./control-plane/internal/api/ ./control-plane/internal/httpx/ -race -count=1   # ok
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8730/v1/sessions                                             # 401
curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer $(cat "$LOCALAPPDATA/PanaceaApp/state/auth/local-key")" http://127.0.0.1:8730/v1/sessions   # 200
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8730/v1/healthz                                              # 200
```

#### What this task does not claim

- Protection from another process **of the same OS user**: it can read the file (ADR-H1 R4).
- That Docker Desktop lets a container reach the host's `127.0.0.1` at all (unverified, ADR-H1
  §5.4): the key makes the answer not matter.
- The attach WebSocket carrying the key (in `Sec-WebSocket-Protocol`): later work (contract §9.4).
