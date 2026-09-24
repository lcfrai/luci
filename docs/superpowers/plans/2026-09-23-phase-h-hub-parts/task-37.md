### Task 37: the local caller key — control-plane side (corrections A3, A4; review C2, I3)

**Lane C. Depends on: 32. About 3.5 hours (1 more than first planned: the attach subprotocol, I3,
and the key in every API test file, C2; and 0.5 for the Windows owner-only DACL, review M9).
Contract task C3.**

Once the Hub exists, a process that reaches `127.0.0.1:8730` can act on shared org data **as the
signed-in user**, not only drive local sessions (ADR-H1 §5.4). Loopback bind, the Host guard and
the Origin allowlist stop a web page; they do not stop **another OS user**, or **a sandboxed agent
that can reach the host's loopback**. So the control plane mints a random key at every start,
writes it to a file only the user can read, and requires it on every route but two. It does not
stop another process of the same OS user — nothing short of an OS peer check does (ADR-H1 R4).

**The attach WebSocket needs the key too (review I3).** It is the one route that types into a
terminal, and the defences it kept (loopback bind, Host guard, Origin allowlist) stop browsers
only: a non-browser process writes `Host` and `Origin` as it likes, and a session id is only 32
bits (`control-plane/internal/ids/ids.go:17-26`). A browser WebSocket cannot set a header, but it
can offer subprotocols, which travel as `Sec-WebSocket-Protocol`. So the desktop offers
`["panacea.v1", "panacea.key." + key]` (task 38), the middleware compares the second entry in
constant time, and the attach handler echoes only `panacea.v1`, so the key never comes back in a
response. The two health routes are the only routes without the key.

**Two corrections make it work.** **A3**: the exempt health routes are `GET /v1/healthz` and `GET
/api/v1/healthz` (`control-plane/internal/api/api.go:358-361` on `feat/hub` after the phase-2
merge; it was `:303-306` when this part was written); there is no `/healthz`. **A4**: the
CORS preflight today allows only `Content-Type` (`control-plane/internal/httpx/origin.go:168-171`),
so an `Authorization` header would fail **every** request at the preflight; it must also allow
`Authorization`, and the key check must sit **inside** the origin middleware so an allowed
preflight is still answered 204 before any key is asked for (`origin.go:231-237`).

**Files:**
- `control-plane/internal/localkey/localkey.go` + `localkey_test.go` (new)
- `control-plane/internal/localkey/private_windows.go` + `private_windows_test.go`, `private_other.go` — the owner-only DACL on Windows, the 0600/0700 path elsewhere (new; review M9). `golang.org/x/sys/windows` is already a direct requirement by then (task 28, `control-plane/go.mod`), so no `go.mod` change
- `control-plane/internal/httpx/origin.go` + `origin_test.go` — `corsHeaders` (modify ⚑: one constant)
- `control-plane/internal/api/api.go` — `Deps.LocalKey`; the zero-key refusal in `New`; the wrap (modify ⚑)
- `control-plane/internal/api/attach.go` — `AcceptOptions.Subprotocols` echoes `panacea.v1` (modify ⚑, one field; review I3)
- `control-plane/internal/api/localkey_test.go` — every route walked; attach's subprotocol (new)
- `control-plane/internal/api/export_test.go` — `WithTestKey`, the one header helper (modify)
- `control-plane/cmd/api/main.go` — mint at start (modify ⚑)
- `control-plane/cmd/api/main_test.go` — `TestTheProcessRefusesToStartWithoutAKey` (modify)

**Every API test file (review C2).** `New` now refuses a zero key, and the guard refuses a request
without one, so every file in `control-plane/internal/api/` that builds the API or sends it a
request changes — **each only by `localkey.ForTest()` in its `Deps` builder and `WithTestKey` on
its requests**, nothing else. The list, as the tree stands on `feat/hub`:

| File | Builds `Deps` at | Sends requests at | Its change |
|---|---|---|---|
| `api_test.go` | `catalogNewDeps`, `:135-160` | `catalogDo`, `:177-195` | `LocalKey: localkey.ForTest()` in the literal at `:142`; `catalogDo` sends `api.WithTestKey(r)` at `:194` |
| `catalog_test.go` | via `catalogNewDeps` (`:27`, `:315`) | via `catalogDo` | none: it inherits both |
| `routes_audit_test.go` | via `catalogNewDeps` (`:140`) | via `catalogDo` | none: it inherits both |
| `sandboxenv_new_test.go` | via `catalogNewDeps` (`:17`, `:23`) | none | none: its `New` calls inherit the key (not in the review's count) |
| `attach_test.go` | its own literal, `:303-316` | `:547-552` (`TestNewActuallyRoutesTheAttachRoute`), through `New`; the rest through a local recorder with no guard (`:332-334`) | `LocalKey: localkey.ForTest()` in the literal; `WithTestKey(r)` at `:552` (a plain GET, so the header, and it still gets 426); the local-recorder tests unchanged |
| `sessions_test.go` | `sessionsNewEnvOverWith`'s literal, `:590` | `throughNew`, `:1213-1217`; `:1245`. `do` (`:632-646`) drives the bare recorder, not `New`'s guard, and is unchanged | `LocalKey: localkey.ForTest()` at `:590`; `WithTestKey(r)` in `throughNew` at `:1215`. `:1245` stays keyless on purpose: it asserts `forbidden_host`, which the Host guard answers before the key is asked for |
| `sessions_sandboxenv_test.go` | via `sandboxEnvNew` → `sessionsNewEnvOverWith` (`:498`) | via `env.do` | none: it inherits both (not in the review's count) |
| `sessions_resume_test.go` | via `sessionsNewEnv` | via `env.do` | none: it inherits both |

Step 1 re-derives this table with `git grep -n "New(\|ServeHTTP(\|httptest.NewServer" --
control-plane/internal/api/*_test.go`, because the line numbers move whenever a sibling merges.
A file that needs more than those two changes means the helper is wrong: stop and fix the helper.

**Siblings (tell the orchestrator when 37 merges).** Phases 3, 4, 5 and 7 add handler tests to the
same package. Each such file needs the same one-line change per builder and per request site, and
fails with `api: Deps.LocalKey is zero` or `401 local_key_required` until it has it. The phase-2
tree also has `sessions_gitfiles_test.go`, uncommitted (not on `feat/hub`; unverified whether it
builds `Deps` itself or goes through `sessionsNewEnvOverWith`).

#### Contract — complete

```go
// control-plane/internal/localkey/localkey.go

// Package localkey is the control plane's local caller key (contract §9.4).
package localkey

// Key is 32 random bytes, base64url without padding: 43 characters. Its String
// and GoString redact; it is never logged and never sent in a response.
// The zero Key is NOT a key: it matches nothing, and api.New refuses it (review C2).
type Key struct{ /* unexported value */ }

func (k Key) String() string   // "localkey.Key{redacted}"
func (k Key) GoString() string // the same

// IsZero reports the zero Key — a Deps that forgot to set it. Never "no check".
func (k Key) IsZero() bool

// ForTest is a fixed, non-zero key for tests in any package. It is not secret
// and never minted: Mint is the only constructor main.go may call, and
// TestNoCommandUsesForTest (a scan of every non-test .go file in control-plane/
// outside internal/localkey) pins that.
func ForTest() Key

// ForTestHeader is "Bearer " + ForTest()'s text, and ForTestSubprotocols is
// {"panacea.v1", "panacea.key." + ForTest()'s text}. No method on Key returns
// its text, so no caller can print or send a real key by accident.
func ForTestHeader() string
func ForTestSubprotocols() []string

// The attach subprotocols (review I3). A browser WebSocket cannot set a header,
// so the desktop offers exactly {Subprotocol, SubprotocolKeyPrefix + key}.
const (
	Subprotocol          = "panacea.v1"
	SubprotocolKeyPrefix = "panacea.key."
)

// Mint makes a new key and writes it to path — creating the directory (0700)
// and the file (0600) — by writing a temp file beside it and renaming it over
// any previous one. A new key every start: nothing to rotate, nothing stale to
// trust.
//
// On Windows the mode bits mean nothing, and an inherited ACL is only as private
// as the directory it comes from: under PANACEA_APP_STATE_DIR=C:\somewhere it may
// admit Users (review M9). So Windows never inherits. private_windows.go builds
// one descriptor, SDDL "D:P(A;;FA;;;<the process user's SID>)" — a PROTECTED DACL
// (no inherited entries) with one entry, full access for the user who runs the
// control plane (windows.GetCurrentProcessToken().GetTokenUser()), and nothing for
// SYSTEM, Administrators, Users or Everyone — by windows.SecurityDescriptorFromString:
//   - the temp file is created by windows.CreateFile(…, CREATE_NEW, …) with a
//     SecurityAttributes holding it, so the key is never readable under an
//     inherited ACL, not even between create and a later SetNamedSecurityInfo;
//   - the auth directory, when Mint creates it, by windows.CreateDirectory with
//     the same; a directory that exists is left as it is (the file's own DACL
//     decides who reads the key);
//   - after the rename (a same-volume move keeps the file's own protected DACL),
//     Mint reads the file's DACL back (windows.GetNamedSecurityInfo) and checks it
//     with checkPrivate: present, protected, exactly one ACCESS_ALLOWED entry, for
//     that SID. Anything else — a file system with no ACLs (FAT32, exFAT: a NULL
//     DACL), or a share that rewrites them — is an error naming the path, and the
//     process does not start (fail closed, not a readable key served).
// private_other.go (//go:build !windows) holds the 0600/0700 path.
func Mint(path string) (Key, error)

// Middleware requires the key on EVERY request it sees; it has no exemption
// parameter. The key arrives one of two ways:
//   - "Authorization: Bearer <key>", on any request; or
//   - a WebSocket upgrade whose Sec-WebSocket-Protocol list is exactly
//     {Subprotocol, SubprotocolKeyPrefix+key}, in that order (review I3). Not on any
//     other request: a plain GET that offers the subprotocol key is refused, so the
//     header path stays the only one a non-WebSocket caller can use.
//   "Upgrade" is the rule api's isWebSocketUpgrade applies (attach.go:350-352), restated
//     here because localkey cannot import api: a GET whose Connection header carries the
//     token "upgrade" (any case, in any of its comma-separated values) AND whose Upgrade
//     header is "websocket" (strings.EqualFold). Not a bare Upgrade check.
//   "The list" is the tokens of EVERY Sec-WebSocket-Protocol header line, each value split
//     on "," and strings.TrimSpace'd, empties dropped, in order (re-review m1). That is how
//     coder/websocket's own headerTokens reads it (accept.go:331, v1.8.12), and it accepts
//     both wire spellings: Dial's "a,b" (dial.go:200) and the browser's "a, b", and the
//     list split across two header lines. Comparing the raw header string passes every
//     Go test (they all use Dial) and refuses the real window.
// Both compare with crypto/subtle.ConstantTimeCompare. A zero k matches nothing, on
// both paths: the zero check comes before the comparison, not inside the header branch,
// because ConstantTimeCompare of two empty slices returns 1 (re-review m2).
// Missing or wrong → 401 local_key_required with
// WWW-Authenticate: Bearer realm="panacea-app-local". It never logs a header's value.
func Middleware(k Key, next http.Handler) http.Handler
```

**No `Exempt` (review I3; deviation declared).** The review says *"`Exempt` shrinks to the two
health routes"*. It is deleted instead: the health routes never reach the middleware (they are
mounted on `root`, outside it, `api.go:358-361`), so an exemption list naming them would be dead
code, and a dead exemption list is where the next exemption gets added. The two health routes are
exempt by where they are mounted, and `TestEveryGuardedRouteNeedsTheKey` walks every guarded
pattern with no exceptions.

**The echo** (`attach.go:130-134`, `handleAttach`'s `websocket.Accept`): add
`Subprotocols: []string{localkey.Subprotocol}`. coder/websocket v1.8.12 answers with the first
entry of that list the client also offered (`selectSubprotocol`, `accept.go:240-250` in the module
cache, read for this plan), so the response carries `panacea.v1` and never the key. WebView2 is
Chromium, and Chromium fails a handshake that offered subprotocols and got none back, so the echo
is required, not cosmetic (Chromium behaviour from memory, unverified by a run here; task 38's
Step 4 sees it in the real window). `handleAttach`'s five checks (§3.16) keep their order; the key
is checked by the middleware in front of them, and the local-recorder tests
(`attach_test.go:332-334`) bypass the middleware exactly as they bypass the Origin middleware today.

```go
// control-plane/internal/httpx/origin.go — the one constant changes (correction A4):
const (
	corsMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type, Authorization"
	corsMaxAge  = "600"
)
```

**Where the wrap goes** (`api.go`, `New`, today's `root.Handle("/", d.Origin.Middleware(guarded))`
at `:361`): `root.Handle("/", d.Origin.Middleware(localkey.Middleware(d.LocalKey, guarded)))`
— inside the origin guard, outside the routes. `/v1/healthz` and `/api/v1/healthz` stay on `root`,
outside both.

**`New` fails closed on a zero key (review C2).** In `New`'s required-deps checks
(`api.go:264-283` on `feat/hub`; the review cited `:213-231`, before the phase-2 merge moved it
51 lines), directly after the nil table and in the same style as `ConversationsDir`'s check
(`:286-288`), because a `Key` is a value, not a nil:

```go
if d.LocalKey.IsZero() {
	return nil, errors.New("api: Deps.LocalKey is zero; build it with localkey.Mint (localkey.ForTest in a test)")
}
```

There is no "zero means no check" branch anywhere, in `New` or in `Middleware`: a wiring slip in
`main.go` must stop the process, never open it. (A zero key that did reach `Middleware` would match
nothing, so every route would refuse every caller: a control plane that is up and useless. `New`
refuses it so the slip shows at startup, not as a wall of 401s. Re-review m5 corrected the old
message, which claimed every route would be *open*.)

**The one test helper** (`control-plane/internal/api/export_test.go`, package `api`, so both the
`api` and `api_test` test files can call it):

```go
// WithTestKey sets the header a Deps built with localkey.ForTest() accepts, and
// returns r so a call site stays one expression.
func WithTestKey(r *http.Request) *http.Request {
	r.Header.Set("Authorization", localkey.ForTestHeader())
	return r
}
```

It is the only way an API test sends the key. A test that needs a wrong key, or the subprotocol
form, builds its request by hand in `internal/api/localkey_test.go`.

**The file's path** is `PANACEA_APP_LOCAL_KEY_FILE`, default `statepath.Dir("auth")` +
`/local-key` — so `%LOCALAPPDATA%\PanaceaApp\state\auth\local-key` on Windows, following
`control-plane/internal/statepath/statepath.go:30-45`'s ladder. Task 38's Tauri command walks the
**same** ladder; both sides pin the same example paths in their tests.

The `client.ts:6-8` comment (*"no Authorization header … before Phase 8"*) is replaced by task 38.

#### Tests to write first

Where each lives: `TestMint…`, `TestTheKey…`, `TestAZeroKeyMatchesNothing`, `TestNoCommandUsesForTest`
(it walks `../..`, the whole `control-plane/` module), `TestAWrongKeyIs401`,
`TestTheSubprotocolListIsReadAsTokens` and `TestTheSubprotocolKeyIsOnlyForUpgrades` in
`internal/localkey/localkey_test.go`, against `Middleware` alone, except the two Windows tests
(`TestTheKeyFileIsPrivateOnWindows`, `TestTheKeyACLCheckRefusesAnyOtherDACL`), which are in
`internal/localkey/private_windows_test.go` under `//go:build windows`. `TestEveryGuardedRouteNeedsTheKey`,
`TestHealthzNeedsNoKey`, `TestAttachNeedsTheKeyInItsSubprotocol`, `TestAPreflightNeedsNoKey` and
`TestADisallowedOriginIsStillRefusedFirst` in `internal/api/localkey_test.go`, **package `api`**, so
the attach test reuses `attachNewFixture` (`attach_test.go:230`). The other two are placed in the
table.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMintWritesAFreshKey` | the file holds 43 base64url characters; a second `Mint` replaces it with a different key; no temp file is left beside it | a key reused across restarts |
| `TestTheKeyPathFollowsStatepath` | with `PANACEA_APP_STATE_DIR=/tmp/s` → `/tmp/s/auth/local-key`; with only `LOCALAPPDATA=C:\Users\u\AppData\Local` → `C:\Users\u\AppData\Local\PanaceaApp\state\auth\local-key`; `PANACEA_APP_LOCAL_KEY_FILE` overrides both — the same examples task 38's Rust test pins | a control plane and a shell looking in two places |
| `TestTheKeyFileIsPrivate` (non-Windows) | mode `0600`, directory `0700` | a key any local user can read |
| `TestTheKeyFileIsPrivateOnWindows` (`private_windows_test.go`, `//go:build windows`; review M9) | a `t.TempDir()` given an **inheritable** entry for `Users` first (`SetNamedSecurityInfo`, SDDL `D:(A;OICI;FA;;;<user>)(A;OICI;GR;;;BU)`: the custom state directory the review names); `Mint(<dir>/auth/local-key)` → the file's DACL, read with `GetNamedSecurityInfo`, is protected and holds exactly one entry, full access for the process user; no entry for `BU`, `AU`, `WD` or any other SID; the `auth` directory Mint created, the same | a key file that inherits `Users: read` from a custom state directory; a DACL set without `P`, which re-inherits; a key written by `os.WriteFile`, then locked, readable in between |
| `TestTheKeyACLCheckRefusesAnyOtherDACL` (`private_windows_test.go`; review M9) | `checkPrivate` over descriptors from `SecurityDescriptorFromString`: `D:P(A;;FA;;;<user>)` → nil; `D:(A;;FA;;;<user>)` (not protected), `D:P(A;;FA;;;<user>)(A;;GR;;;BU)`, `D:P(A;;FA;;;S-1-5-18)` (another SID), `D:P(D;;FA;;;<user>)` (a deny entry), and `D:NO_ACCESS_CONTROL` (a NULL DACL, the FAT case) → an error each | a check that passes a FAT volume, or any DACL with the user's entry somewhere in it |
| `TestTheKeyNeverPrints` | `%v`, `%+v`, `%#v`, `slog` of a `Key` → `redacted`; no response body in any test below contains it | the key in a log |
| `TestEveryGuardedRouteNeedsTheKey` | for **every** pattern in `API.RegisteredPatterns()`, attach included, with no exceptions, a request with ids filled in and no key → 401 `local_key_required`; with the key → not 401 **with code `local_key_required`** (a Hub route may still answer 401 `signed_out`, task-32.md:27, task-35.md:131; plain "not 401" would fail falsely on a signed-out fixture — re-review, outside b02) | a route added later that forgets the key (the §10a audit's own technique); an exemption list growing back |
| `TestNewRefusesAZeroLocalKey` (`api_test.go`, through `catalogNewDeps`) | `d.LocalKey = localkey.Key{}` → `New` returns an error containing `Deps.LocalKey is zero` and a nil handler (C2) | a zero key treated as *no check*: fails open, every test still green |
| `TestAZeroKeyMatchesNothing` (`internal/localkey/localkey_test.go`) | `Middleware(localkey.Key{}, next)`: no header, `Bearer ` (empty), `Bearer <43 × "A">` → all 401; **and on the upgrade path** (re-review m2), a WebSocket-upgrade GET offering `{"panacea.v1", "panacea.key."}` (an empty key) and one offering `{"panacea.v1", "panacea.key.<43 × "A">"}` → both 401; `next` never called in any case | the same fail-open, one layer down; a zero-key guard written only in the header branch, which the upgrade path passes because `ConstantTimeCompare` of two empty slices is 1 |
| `TestNoCommandUsesForTest` (`internal/localkey/localkey_test.go`) | no non-test `.go` file anywhere under `control-plane/` outside `internal/localkey/` itself contains `localkey.ForTest` (it walks `../..`; re-review m5 widened it from `cmd/`, at the same cost) | production wired with the public test key, from `main.go` or from an internal package's "helpful default" |
| `TestTheSubprotocolListIsReadAsTokens` (`internal/localkey/localkey_test.go`; re-review m1) | a WebSocket-upgrade GET whose `Sec-WebSocket-Protocol` is the browser's spelling `"panacea.v1, panacea.key.<k>"` → `next` called; the same list as two header lines (`panacea.v1` and `panacea.key.<k>`) → `next` called; `"panacea.v1,panacea.key.<k>"` (Dial's spelling) → `next` called; `Connection: keep-alive, Upgrade` with `Upgrade: WebSocket` → treated as an upgrade; `Upgrade: websocket` with no `Connection` token → not an upgrade, 401 | a raw-string compare against `","` that passes every `Dial`-based test and refuses the real window; an upgrade test that differs from `isWebSocketUpgrade` |
| `TestAWrongKeyIs401` | `Bearer <another key>`, `Bearer`, `Basic …` → 401 | a prefix match or an empty-string match |
| `TestHealthzNeedsNoKey` | `GET /v1/healthz` and `GET /api/v1/healthz` → 200 without a key (A3) | a supervisor restarting a healthy control plane |
| `TestAttachNeedsTheKeyInItsSubprotocol` (review I3; replaces `TestAttachIsExempt`) | through `New`'s handler, on `/v1/sessions/{id}/attach` and the `/api` alias, against a running fixture session served by `httptest.NewServer(a)`: a `websocket.Dial` with no subprotocols → 401 `local_key_required`; with `{"panacea.v1", "panacea.key.<wrong>"}` → 401; with the two entries swapped → 401; with `{"panacea.v1"}` only → 401; with `localkey.ForTestSubprotocols()` → the socket opens and `c.Subprotocol()` is `panacea.v1`; the handshake response's `Sec-WebSocket-Protocol` header and every captured log line contain no `panacea.key.` text | a PTY any local process can type into after guessing 32 bits; a prefix or ordering match; the key echoed back or logged |
| `TestTheSubprotocolKeyIsOnlyForUpgrades` | a plain `GET /v1/sessions` (no `Upgrade`) whose `Sec-WebSocket-Protocol` is `ForTestSubprotocols()` → 401 | the subprotocol turned into a second, header-shaped credential for every route |
| `TestTheProcessRefusesToStartWithoutAKey` (`cmd/api/main_test.go`, written exactly as `TestRunRefusesANonLoopbackBindWithExitCode2` is, `:392-402`: `run(env(runHermetic(t, map[string]string{envLocalKeyFile: …})), &out)`, where `envLocalKeyFile` is the constant this task adds for `PANACEA_APP_LOCAL_KEY_FILE` beside `envAddr` and `envBackend` in `cmd/api/config.go:28-30`) | `PANACEA_APP_LOCAL_KEY_FILE` under a path whose parent is a regular file → a non-zero exit, and `out` names the path and says the local key could not be written. **It must go through `runHermetic`** (`config_sandbox_test.go:228-240`; re-review m4), which forces `kata` with a kubeconfig `kubeClient` refuses, a `t.TempDir()` state directory and port 0. That is what makes it safe **while it is red**: once green, `Mint` is the first thing `serve` (`main.go:66`) does, before `newBackend` (`:67`), `api.New` (`:132`) and the listener (`:154`); but in the mandated red run, and under the mutant that moves `Mint` after `newBackend`, a test without `runHermetic` would start the default Docker backend, whose reaper sweeps the shared daemon at once with an empty registry (`main.go:85-97`), and then serve and hang. Under `runHermetic` both fail in `newBackend` with a kubeconfig error that does not name the key path, so the test is red and nothing is touched. (No `net.Dial` assertion: `run` returns only after `serve` does, so a dial after it proves nothing.) | a control plane that serves with no key file the desktop can read, or with a key it never wrote |
| `TestAPreflightNeedsNoKey` | `OPTIONS /v1/sessions` with an allowed `Origin` → 204 with `Access-Control-Allow-Headers: Content-Type, Authorization` (A4) | every request failing at the preflight — the failure A4 exists to prevent |
| `TestADisallowedOriginIsStillRefusedFirst` | a bad `Origin` with a good key → 403 `forbidden_origin` | the key replacing the origin check instead of adding to it |

#### Steps

- [ ] **Step 1:** Re-read contract §9.4; `api.go:263-361` (`New`, from the required-deps table to
  the wrap) and `origin.go:164-240` as they stand; `attach.go:56-140` (`handleAttach`, to the
  `Accept`). Re-derive the test-file table above with its `git grep`.
- [ ] **Step 2:** Tests; they fail. Then `localkey.go`, the constant, the zero-key refusal in
  `New`, the wrap, the `Subprotocols` field in `attach.go`, `main.go`'s `Mint` first in `serve`
  (a key that cannot be written stops the process with that sentence).
- [ ] **Step 3:** `export_test.go`'s `WithTestKey`, then every row of the test-file table, each
  file changed only by `localkey.ForTest()` in its builder and `WithTestKey` at its request sites.
  `go test ./control-plane/internal/api/ -race -count=1` is green **before** Step 4; a red test
  that needs anything more than the helper is a helper defect, not a test to edit.
- [ ] **Step 4:** **The desktop now gets 401 on every call, and every attach fails, until task 38
  lands.** Land 37 and 38 together — the orchestrator merges both, or neither, into `feat/hub`.
  Tell the orchestrator, in the report, that every sibling phase's API test files need the same
  one-line change per builder and per request site when 37 merges (the Files section's sibling
  note).
- [ ] **Step 5:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/localkey/ ./control-plane/internal/api/ ./control-plane/internal/httpx/ -race -count=1   # ok
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8730/v1/sessions                                             # 401
curl -s -o /dev/null -w "%{http_code}\n" -H "Authorization: Bearer $(cat "$LOCALAPPDATA/PanaceaApp/state/auth/local-key")" http://127.0.0.1:8730/v1/sessions   # 200
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8730/v1/healthz                                              # 200
```

#### What this task does not claim

- Protection from another process **of the same OS user**: it can read the file (ADR-H1 R4).
  Nor from an administrator, who can take ownership of any file; the DACL keeps out other
  non-admin users only (review M9).
- That `SECURITY_DESCRIPTOR.String()` renders the DACL as exactly the SDDL written above:
  unverified until the Windows test runs; the tests compare parsed entries, not strings. The
  x/sys names used (`SecurityDescriptorFromString`, `CreateFile`, `CreateDirectory`,
  `GetNamedSecurityInfo`, `SetNamedSecurityInfo`, `GetAce`, `GetTokenUser`,
  `PROTECTED_DACL_SECURITY_INFORMATION`) were checked in the module cache's x/sys v0.24.0, not in
  the v0.32.0 the graph will hold after task 06.
- That Docker Desktop lets a container reach the host's `127.0.0.1` at all (unverified, ADR-H1
  §5.4): the key makes the answer not matter.
- That session ids need to be longer. With the key on attach (review I3), a 32-bit id is no longer
  the only barrier to another session's PTY; it stays as phase 2 made it.
- That WebView2 fails a handshake whose subprotocol is not echoed: Chromium's documented
  behaviour as understood from memory, not observed or checked against a source here. The echo is
  there either way.
