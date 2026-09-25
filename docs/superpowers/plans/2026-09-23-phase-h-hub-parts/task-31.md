### Task 31: `signin` — the `TokenSource`: per-audience refresh, the original offline token kept (U-4b), sign-out, the silent start-up (HA-2)

**Lane C. Depends on: 30; 04 (`hubclient.Invalidator` and `ErrSignedOut`, through 30 too). About 4.5 hours. Contract task C1; corrections A12 and A13.**

**OD-30 (ruling `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` on `main`, sections
2, 3 and 7) changes four things here.** The package is `signin` (task 30). The stored token is the
realm's **offline** refresh token, and each `Token(ctx, audience)` narrows it with
`scope=openid <audience>` to a **single-audience** access token (ruling section 2 step 7). **At every
launch the control plane redeems it silently** before the renderer's first status poll: success is
signed in with no browser; `invalid_grant` deletes it and runs task 30's flow **at once, with no click**;
a network failure never opens a browser (ruling section 3). And **Azure DevOps is a PAT only** (ruling
section 7, hub Q5 flipped): `AzureDevOpsScope`, the `DevOps` adapter and `PANACEA_APP_ENTRA_DEVOPS` are
deleted; phase 3's chain is `Local → LocalDevOps → GitHelper` (task 29).

After sign-in, every Hub call and every phase 8 broker call asks `Token(ctx, audience)` for the
user's own access token (contract §6.2). **One refresh token serves every audience** the realm
client holds (`AudienceScopes`), so there is one sign-in, not four. This task is phase 8's **HA-2**
exactly.

**One refinement the contract does not state, and why it is needed.** The contract makes refresh
single-flight **per scope**. The scopes share **one** refresh token. Under OD-30 the realm does not
rotate it (`revokeRefreshToken:false`) and this task never writes a refresh grant's returned token
back (ruling U-4b, below), so a double redemption is harmless; redemptions stay **serialised
across all scopes** by one lock anyway, one request at a time against one stored token, and a realm
that turned rotation on would be a ruling change, not a code one. Each redemption reads the refresh token
current at its start; callers of one scope still share one redemption.

**Files:**
- `control-plane/internal/signin/{tokens.go,account.go,startup.go}` + `{tokens_test.go,startup_test.go}` (new)

- `control-plane/internal/hub/hubclient/hubclient.go` — **comments only**: `entraauth` -> `signin` at
  `:18`, `:23` and `:137` (the OD-30 ruling's section 11 folds this into task 31; verified on
  `feat/hub` at `3b0dcad`). No code line changes: task 04 declares `Invalidator`, so this lane-C
  task still changes no lane-A behaviour, review M15.

#### Contract — complete

```go
// control-plane/internal/signin/tokens.go
package signin

// The audiences are task 30's AudienceScopes. AzureDevOpsScope is DELETED (OD-30
// ruling section 7: Azure DevOps is a PAT, credstore.LocalDevOps, task 29).

// Scope-specific refusals (review I2). Neither signs the user out; each wraps a
// *ScopeRefusal naming the audience and the issuer's code, so the caller can
// surface "the broker refused PanaceaApp's sign-in" rather than "sign in again".
// Neither names an interactive step (re-review b04 N1): v1's only interactive
// sign-in is BeginSignIn, which already asks for every audience, so no product
// action performs a per-audience step-up. The remedy is the realm admin's (the
// optional client scope missing on panacea-app-desktop, ruling section 10).
var (
	ErrScopeNeedsConsent = errors.New("signin: this audience is not granted to the desktop client")
	ErrScopeRefused      = errors.New("signin: this audience refused this sign-in")
)

type ScopeRefusal struct {
	Scope    string
	Code     string // "invalid_grant" | "interaction_required" | "invalid_scope"
	Suberror string // an Entra-style suberror; Keycloak sends none, so "" under the keycloak profile
	Err      error  // ErrScopeNeedsConsent or ErrScopeRefused
}

func (r *ScopeRefusal) Error() string // "signin: <scope>: <Code>/<Suberror>"
func (r *ScopeRefusal) Unwrap() error // r.Err

// refreshSkew is how long before exp a cached token stops being served
// (review I9, OD-30): min(60 s, lifetime/2), lifetime = exp − the Now() at which the
// token response was received. The ruling's "cached until 60 seconds before exp"
// (section 2 step 7) for every token of 2 min or more, so Keycloak's 5-minute
// tokens are served for 4 min; A13's half-lifetime guard kept below that, so a
// 90 s token is served for 45 s, not redeemed on every call. All times come from
// Config.Now, never time.Now.
func refreshSkew(lifetime time.Duration) time.Duration

// Token serves the cached access token for scope while exp − now > refreshSkew(lifetime);
// otherwise it redeems the refresh token through task 30's redeem: POST token_endpoint
// with grant_type=refresh_token, client_id, refresh_token, scope="openid <scope>" (OD-30
// ruling section 2 step 7: one audience per access token). scope must be one of
// AudienceScopes; any other → an error, no request. No redemption ever opens a browser:
// only Start (startup.go) and POST /v1/auth/sign-in call BeginSignIn.
//   - Callers for one scope share one redemption (single-flight); redemptions for
//     all scopes are serialised by one lock (the refinement above).
//   - A refresh_token in a refresh grant's response is DISCARDED, never written
//     (OD-30 ruling U-4b, review-1 I4). Every refresh here is narrowed to one
//     audience, and the realm may copy the narrowed scope into the token it returns;
//     writing it back would make the next Token(hub) fail invalid_scope, a Hub gone
//     dark with no re-sign-in. The stored ORIGINAL offline token is kept and serves
//     every audience (safe: Panacea's revokeRefreshToken is false, facts :94-96,
//     so the original is never spent; devissuer task 12 pins the adverse case).
//     This holds whether U-4b is true or false. The only writer of the stored token
//     is task 30's code exchange, by task 28's generation sequence (review I1): chunks under a NEW
//     generation (NewGeneration, OIDCRefreshGenPrefix(Issuer, gen)); then the account record,
//     naming refreshGen and refreshChunks, written last; only then DeleteChunked of
//     the old generation's prefix. Nothing the old record names is overwritten, so
//     if a chunk Put or the record Put fails, or the process dies, the old record
//     still names the old, whole generation: Token returns the error, best-effort
//     deletes the new generation, and the old refresh token stays usable (contract
//     §6.2: "a crash in between leaves the old one"). A failed delete of the old
//     generation after the record is written is logged at warn and not an error:
//     start-up sweeps it.
//   - A refusal is read by the SCOPE being redeemed, not by the error code alone
//     (review I2): one refresh token serves every resource, and a refusal for one
//     resource is not a dead refresh token. Under the keycloak profile the
//     realm answers invalid_grant (offline session gone, revoked or expired) and
//     invalid_scope (an audience scope not on the client); it sends no suberror
//     and no error_codes, so the Entra-body branches below never fire there and
//     are kept only because the rule is written by code, not by issuer.
//       · Redeeming HubAudience: invalid_grant (any suberror) or interaction_required
//         → both entries deleted, every cached token dropped, Status
//         {signed_out, reauth_required}, ErrSignedOut to every caller waiting on it.
//         The Hub is the sign-in's own resource, so a refusal for it means the
//         session is over.
//       · Redeeming any OTHER scope (panacea-app-broker, -gateway, -fleet):
//         invalid_grant with suberror consent_required or error_codes holding
//         65001 → ErrScopeNeedsConsent; interaction_required (or invalid_grant
//         with error_codes holding 50076 or 50079) → ErrScopeRefused.
//         Both wrap a *ScopeRefusal{Scope, Code, Suberror}. NOTHING is deleted,
//         the refresh token is kept, Status is unchanged, only that scope's
//         cached token is dropped, and the other scopes keep serving.
//       · Any OTHER invalid_grant on a non-Hub scope (no consent suberror, no
//         50076/50079) may be a dead refresh token or a per-resource refusal, and
//         only the Hub scope can tell: under the same redemption lock, Token
//         redeems HubAudience once. Hub refuses too → the sign-out path above, and
//         ErrSignedOut to the non-Hub caller as well; Hub succeeds (its returned
//         refresh token discarded, as above) → ErrScopeRefused for that scope. Root-cause
//         ruling: a non-Hub refusal never deletes the one credential the Hub
//         still redeems, and a dead token is not left in the store for an hour
//         while the Hub's cached access token lives. If the probe's own HubAudience
//         redemption answers unreachable, a timeout or a 5xx → ErrIssuerUnavailable
//         to the non-Hub caller; if it answers any 4xx the Hub rule does not list
//         → the error naming that code (the "any other 4xx" rule below). In both,
//         nothing is deleted, Status is unchanged and nothing is remembered
//         (re-review b04 m4).
//       · A *ScopeRefusal is REMEMBERED per scope (re-review b04 m4) and served
//         with no request until the first of: refreshGen changes (a new sign-in:
//         the only writer, as above), Invalidate(scope), or refusalBackoff
//         (15 min) passes on Config.Now. It is remembered against the refreshGen
//         current after the probe, which the probe never changes, so the probe
//         never clears the refusal it produced. So phase 3's Fetcher, asking AuthFor per
//         project per tick (phase-3 task-12.md:56, task-13.md:190), costs one
//         redemption per scope per back-off, not one per project, and the plain
//         invalid_grant probe does not rotate the refresh token on every call. A
//         sign-out forgets every remembered refusal with the rest.
//       · invalid_scope on a non-Hub scope (the audience scope is not on the realm
//         client panacea-app-desktop: ruling section 10's K1 not yet applied for it)
//         → ErrScopeNeedsConsent, the same keep-everything rule.
//       · Any other 4xx body, on any scope (invalid_client, unauthorized_client,
//         invalid_request, invalid_scope on HubAudience, an unknown code) → an error
//         naming the code (and error_codes), NOT ErrSignedOut: nothing is deleted
//         and Status is unchanged. Only invalid_grant and interaction_required
//         prove a refresh token dead, and only for HubAudience; a misconfiguration
//         never costs the user their sign-in.
//       · The refresh token is NOT rotated by a refused redemption (no new
//         refresh_token in an error body), so the store is not written.
//   - The issuer unreachable, a timeout, a 5xx → ErrIssuerUnavailable; the stored token is kept.
//   - Not signed in → ErrSignedOut, with no network call.
func (a *Authenticator) Token(ctx context.Context, scope string) (string, error)

// Invalidate drops the cached access token for scope, so the next Token redeems.
// hubclient calls it before its one retry on a 401 with reason "expired" (contract §4.3).
func (a *Authenticator) Invalidate(scope string)

// SignOut deletes the account record FIRST, then every generation's chunks
// (DeleteChunked(OIDCRefreshPrefix(Issuer))), and every cached token; a crash in between
// leaves chunks with no record, which start-up sweeps. Status becomes {signed_out, ""}. It does not call the realm's end_session_endpoint
// endpoint in v1: the browser session is the user's, not ours (contract §6.2).
func (a *Authenticator) SignOut(ctx context.Context) error
```

```go
// control-plane/internal/signin/account.go — reading the store at start (contract §6.2).
//
// New reads OIDCAccountTarget(Issuer) (task 28 as OD-30 amends it:
// PanaceaApp/oidc/<issuer host>/<realm>/...), so New needs no network and no List,
// and one machine's records for two issuers never meet. New itself refuses an
// Issuer that is not <scheme>://<host>[:port]/realms/<realm> with no trailing "/"
// (an error, not a guess; task 32's TestTheIssuerIsARealmURL is the loader's
// copy), so a caller that bypasses task 32's loader cannot look under a
// malformed target and find nothing. A record whose "issuer" or "clientId"
// differs from Config's is INVALID (below).
// Parsing the record validates it BEFORE any target builder sees a stored value
// (plan re-review b03 N1): refreshGen must pass credstore.ValidGeneration and
// refreshChunks must be 1 to 16 (16 × MaxBlob = 40,960 bytes, far above the 6,000
// of R1). A record that fails, or does not parse, or has no refreshGen (none exists
// before this plan ships), is INVALID. The builders panic on a bad gen, and a
// stored gen is not validated configuration, so it never reaches one unchecked.
//
// Three outcomes, decided by errors.Is(err, credstore.ErrNotFound) and nothing
// looser (plan re-review b03 m3):
//   - Present, valid, and the refreshChunks chunks of its refreshGen all readable →
//     the account is LOADED, Status {signed_out, ""}, NOTHING redeemed by New: Start
//     (startup.go) redeems it at once; then SweepGenerations(OIDCRefreshPrefix(Issuer),
//     refreshGen) deletes every generation the record does not name (a rotation that
//     failed or died before its record Put, or after it but before the old
//     generation's delete).
//   - The record's Get → ErrNotFound, or the record INVALID, or a chunk Get of the
//     named generation → ErrNotFound → {signed_out}, the record (if any) deleted,
//     then SweepGenerations(…, "") deletes every generation.
//   - ANY other error from a Get or a List (a CredReadW failure that is not
//     ERROR_NOT_FOUND, such as ERROR_NO_SUCH_LOGON_SESSION) → {signed_out, ""} with
//     the error logged at warn (no new Reason value: task 30's enum is the UI
//     contract's), and NOTHING deleted, NOTHING swept: the next start retries, and
//     one transient read error never destroys a valid refresh token.
// A failed SweepGenerations, in either branch, is logged at warn and never fatal:
// the named generation is intact, and the next start sweeps again.
```

```go
// control-plane/internal/signin/startup.go — the silent refresh at every launch
// (OD-30 ruling section 3). devops.go is DELETED: Azure DevOps is a PAT (ruling
// section 7), served by credstore.LocalDevOps (task 29).

// ReasonOffline is Status.Reason while Start's redemption cannot reach the issuer
// ("signed-out-offline", ruling section 3). NEW wire value, added to task 30's
// Reason enum and to /v1/auth/status (task 32); the renderer shows it as offline,
// never as "sign in" (tasks 40, 41).
const ReasonOffline = "offline"

// Start is called once by task 32's wiring BEFORE the control plane's listener
// serves, so it settles before the renderer's first GET /v1/auth/status. It makes
// at most one redemption, bounded by Config.StartTimeout (default 3 s), and then
// returns; the outcomes are decided exactly as Token's refusal rules for
// HubAudience decide them:
//   - an account loaded by New, and Token(ctx, HubAudience) succeeds →
//     {signed_in}, Account set, the Hub token cached. NO browser.
//   - invalid_grant or interaction_required (the refresh token is dead: revoked,
//     expired, a realm reset) → Token's sign-out path deletes both entries, then
//     Start calls BeginSignIn AT ONCE: Config.OpenBrowser opens the system browser
//     and Status is {signing_in}. No click; with the realm's rows (ruling section 3)
//     the tab completes on its own.
//   - no account (the store's ErrNotFound, a partial write, an INVALID record) →
//     the same BeginSignIn at once: the first launch has no sign-in click either
//     (ruling section 11, tasks 40-41).
//   - the issuer unreachable, a timeout, StartTimeout passing, a 5xx →
//     {signed_out, ReasonOffline}; NOTHING deleted and the browser NOT opened; the
//     silent redemption is retried every Config.RetryEvery (default 5 s, on
//     Config.After, a func(time.Duration) <-chan time.Time, default time.After)
//     until it settles into one of the two rows above or ctx ends. A network
//     error never opens a browser.
//   - any other 4xx (invalid_client, invalid_scope on HubAudience, an unknown code)
//     → {signed_out, ""}, the code logged at warn, nothing deleted, NO browser and
//     no retry: a misconfigured client is not fixed by a sign-in page.
//   - a store read error that is not ErrNotFound (account.go's third outcome) →
//     {signed_out, ""}, nothing deleted, NO browser: the next launch retries.
// Start never runs twice: a second call returns the current Status.
func (a *Authenticator) Start(ctx context.Context) Status
```

`hubclient.Invalidator` (`Invalidate(scope string)`) is declared by **task 04**, in lane A's
`hubclient.go` (review M15). This task only implements it: `*Authenticator` gets
`func (a *Authenticator) Invalidate(scope string)`, and a compile-time
`var _ hubclient.Invalidator = (*Authenticator)(nil)` in `tokens.go` holds the two together.

#### Tests to write first — `tokens_test.go`, `startup_test.go`

Each against `devissuer` in-process with `AccessTTL` set short where expiry matters, and an
injected clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACachedTokenIsServedUntilSixtySecondsLeft` | `AccessTTL` 5 min: at exp − 61 s → the cached token, no request; at exp − 59 s → a redemption | a redemption per call; a token served until it expires in flight; the old 5-minute skew, which redeems Keycloak's 5-minute token on every call |
| `TestTheRefreshSkewIsHalfAShortLifetime` | `refreshSkew` table: 60 min → 60 s, 5 min → 60 s, 2 min → 60 s, 90 s → 45 s, 0 → 0. Then `AccessTTL` 90 s through `Token`, on the injected clock: at issue + 44 s → the cached token, no request; at issue + 46 s → one redemption (review I9) | a fixed skew, under which any token shorter than it is redeemed on every call; a skew read from `time.Now` instead of `Config.Now` |
| `TestTenCallersOneRedemption` | 10 goroutines, one scope, expired cache → exactly 1 token-endpoint request (contract §13 C1) | a thundering herd on the token endpoint |
| `TestRedemptionsAreSerialisedAcrossScopes` | Hub and broker scopes expire together; 5 callers each, concurrently → both succeed; the endpoint saw 2 requests, never two with the same refresh token | the per-scope-only single-flight, which signs the user out under rotation |
| `TestANarrowedGrantsRefreshTokenIsNeverWritten` | signed in (the original offline token O); `Token` for each of the four audiences, then `Token(HubAudience)` again with its cache expired: a recording `Secrets` sees **no** `Put` and **no** `Delete`; every request carries O; devissuer's narrowed refresh tokens (task 12's `TestANarrowedRefreshTokenCannotWiden`) are never sent; the last Hub redemption succeeds (OD-30 ruling U-4b, review-1 I4) | a write-back of a narrowed grant's token, after which `Token(hub)` fails `invalid_scope` and the Hub goes dark with no re-sign-in |
| `TestAReSignInIsWrittenByGeneration` | signed in (generation A); a second sign-in's code exchange (task 30): a recording `Secrets` sees, in order, the new generation's chunks, then the account record naming that `refreshGen` and `refreshChunks`, then A's deletes; **no `Put` targets any chunk of A** | the in-place overwrite (review I1); an old generation deleted before the record names the new one |
| `TestAFailedSignInWriteKeepsTheOldToken` | signed in (generation A); the second sign-in's `RoundTripper` swaps the code exchange's `refresh_token` for a **6,000-byte** value (3 chunks — devissuer issues 43-character tokens, too small to need a second chunk); a faulting `Secrets` fails the **second** chunk's `Put` → the sign-in errors; the record still names A; `GetChunked(A, n)` returns the old token **whole, byte for byte**; `List(OIDCRefreshPrefix(Issuer))` holds only A's chunks; a fresh `New` + `Start` over the same store → `signed_in`. A second case fails the record's `Put` instead: the same assertions | a torn token (new chunk 0 + old chunk 1, review I1) that signs the user out at the next start; a fault injected only on the first `Put`, which never tests the tear |
| `TestStartUpSweepsUnnamedGenerations` | the store holds the record naming B, B's chunks, and a stray generation C (a rotation that died before its record `Put`) and A (one that died before its old-generation delete); `New` + `Start` → `signed_in`, and `List(OIDCRefreshPrefix(Issuer))` is exactly B's chunks | leaked refresh tokens piling up in Credential Manager; a sweep that deletes the named generation |
| `TestNewFindsTheRecordByTheIssuer` (review M8, as OD-30 amends it) | a record stored under `OIDCAccountTarget(i1)`: `New` with `Issuer: i1` loads it, and with a second realm `i2` on the same host loads nothing; `…/realms/x/` (trailing `/`), `…/auth/x` and `https://host` → an error from `New`, not `signed_out`; a record whose `issuer` or `clientId` differs from `Config`'s → INVALID, deleted; a recording `Secrets` sees that exact target as the first `Get`, and every `List` (the start-up sweep's) under `PanaceaApp/oidc/<host>/<realm>/`, none wider | a restart that looks under the wrong issuer and signs the user out for good; a lookup that reads every account on the machine; a dev-realm token redeemed at another realm |
| `TestInvalidGrantSignsOut` | `devissuer.RevokeRefreshTokens(grace)`; `Token(HubAudience)` with its cache expired → `ErrSignedOut`; `Status` `reauth_required`; the store is empty | a dead refresh token retried on every call |
| `TestInteractionRequiredSignsOut` | the `RoundTripper` answers the Hub scope's refresh grant `400 {"error":"interaction_required"}` (devissuer's `RequireInteraction` is `entra`-profile only, task 12; the rule is by code, not issuer); `Token(HubAudience)` → the same | a conditional-access sign-in frequency never surfaced to the user |
| `TestASecondaryScopeRefusalKeepsTheSession` | signed in, the Hub token cached. `Config.HTTPClient`'s `RoundTripper` answers the **broker** scope's refresh grant itself, never forwarding it (so devissuer's refresh token is not spent), with Entra's body `{"error":"invalid_grant","suberror":"consent_required","error_codes":[65001]}` → `Token(brokerScope)` is `ErrScopeNeedsConsent`, `errors.As` a `*ScopeRefusal` whose `Scope` is the broker's; `Status` still `signed_in`; the store, compared entry by entry, **unchanged**; then, the Hub's cache expired, `Token(HubAudience)` succeeds with a real redemption of the kept refresh token. Case 2: `{"error":"interaction_required","error_codes":[50076]}` → `ErrScopeRefused`, the same assertions. Case 3: a bare `{"error":"interaction_required"}` for the broker's grant with the Hub token still cached → `Token(brokerScope)` is `ErrScopeRefused`, the store unchanged. Case 4: `{"error":"invalid_scope","error_codes":[70011]}` for the broker's grant → `ErrScopeNeedsConsent`, the same assertions (review I2) | the scope-blind rule, under which a conditional-access policy on Azure DevOps signs the user out of the Hub at every push; a refusal that drops every scope's cached token |
| `TestAPlainInvalidGrantOnASecondaryScopeAsksTheHub` | Case 1: `RevokeRefreshTokens(grace)`, the Hub token cached → `Token(brokerScope)` is `ErrSignedOut`; the endpoint saw exactly two requests, the broker's then the Hub's; `Status` `reauth_required`; the store is empty. Case 2: the `RoundTripper` answers the broker's grant with a bare `{"error":"invalid_grant"}` and forwards the Hub's → `Token(brokerScope)` is `ErrScopeRefused`; `Status` `signed_in`; the record's `refreshGen` is unchanged, since the probe writes nothing (review I2; OD-30 U-4b). Case 3: the broker's grant answered with a bare `invalid_grant`, the Hub's probe answered `503` → `ErrIssuerUnavailable`. Case 4: the probe answered `400 {"error":"invalid_client"}` → an error naming `invalid_client`, neither `ErrSignedOut` nor a `*ScopeRefusal`. In both, `Status` `signed_in`, the store entry by entry unchanged, and the next `Token(brokerScope)` makes a request (nothing remembered; re-review b04 m4) | a dead refresh token kept for the life of the Hub's cached token; a per-resource refusal that signs the user out; a probe outage read as a dead token, or remembered as a refusal |
| `TestARefusalIsRememberedUntilTheGenerationChanges` | the `RoundTripper` answers the broker's grant with `consent_required`/65001, counting requests. `Token(brokerScope)` → `ErrScopeNeedsConsent`; 10 more calls → the same `*ScopeRefusal`, **0** further requests. The plain-`invalid_grant` case: 10 more calls → 0 further Hub probes, and the record's `refreshGen` is unchanged. Then each of three clears, separately: a new sign-in (`refreshGen` changes; a Hub redemption alone changes nothing, OD-30 U-4b); `Invalidate(brokerScope)`; the injected clock advanced 15 min → the next `Token(brokerScope)` makes exactly 1 request. At 14 min 59 s → still 0. After `SignOut` and a new sign-in → 1 request (re-review b04 m4) | a refusal re-redeemed on every call, and a full rotation written to Credential Manager per project per fetch tick; a refusal remembered forever, so IT's consent is never seen; a back-off read from `time.Now` |
| `TestAnUnlistedRefusalDeletesNothing` | the `RoundTripper` answers the **Hub** scope's refresh grant with `400 {"error":"invalid_client"}`, then `{"error":"invalid_scope"}`, then `{"error":"something_new"}`, never forwarding → each `Token(HubAudience)` errors naming the code, and is neither `ErrSignedOut` nor `ErrIssuerUnavailable`; `Status` still `signed_in`; the store, entry by entry, unchanged; with the `RoundTripper` removed, the next `Token(HubAudience)` redeems the kept refresh token (review I2) | an "any 4xx signs out" rule, under which a misconfigured client id or scope deletes every user's refresh token |
| `TestAnUnreachableIssuerKeepsTheToken` | the issuer stopped → `ErrIssuerUnavailable`; `Status` still `signed_in`; the store unchanged | a network blip signing the user out |
| `TestStartUpFromTheAccountRecord` | a store pre-filled by a previous sign-in; `New` → the account loaded, **no** request; `Start` → exactly **one** request (`grant_type=refresh_token`, `scope=openid panacea-app-hub`), `signed_in` with the account, `OpenBrowser` never called. A second case wraps the same store in a faulting `Secrets` whose `Get` of the record returns an error that is **not** `ErrNotFound` → `New` returns `signed_out` without error, `Start` makes no request and does not call `OpenBrowser`, and the store, compared entry by entry before and after, is **untouched**: no `Delete`, no sweep (plan re-review b03 m3) | a launch that needs a click (no silent redemption); a transient read failure treated as "absent", which deletes a valid refresh token or opens a second sign-in |
| `TestAPartialWriteIsNotAnAccount` | chunks but no account record → `signed_out`, every generation deleted; a record whose named generation lacks a chunk → `signed_out`, record and every generation deleted; and a record with `"refreshGen":"XYZ"`, and one with `"refreshChunks":0` and one with `17`, each over a valid generation's chunks → `New` returns `signed_out` **without panicking**, record and every generation deleted (plan re-review b03 N1) | a half-written sign-in treated as complete; a stored gen passed unchecked to a builder, which panics at every start |
| `TestSignOutDeletesEverything` | → store empty, `Token` → `ErrSignedOut` without a request | a refresh token left behind (the machine's next user) |
| `TestInvalidateForcesARedemption` | a fresh cached token; `Invalidate`; `Token` → a request | the 401-expired retry of contract §4.3 retrying with the same token |
| `TestOneRefreshTokenServesEveryAudience` | after one sign-in, `Token` for each of the four `AudienceScopes` succeeds; each request's `scope` is exactly `openid <that audience>`; each token's `aud` contains that audience and **no other of the four** (devissuer narrows; ruling U-4 records whether the dev realm does); `Token(ctx, "Hub.Access")` → an error, no request | a second sign-in for the broker (phase 8); a refresh that asks for every audience, so a broker token replays at the Hub (confused deputy) |
| `TestErrSignedOutIsHubclients` | `errors.Is(signin.ErrSignedOut, hubclient.ErrSignedOut)` | the relay missing a signed-out error |

`startup_test.go` — each with a **recording `OpenBrowser`** (it counts calls and, when a test says
so, drives task 30's scripted browser):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStartOnADeadTokenSignsInAtOnce` | `RevokeRefreshTokens(grace)`, then a fresh `New` + `Start` → the store's old generation deleted; `OpenBrowser` called **once**, with no call to `BeginSignIn` from the test; the scripted browser completes it → `signed_in`, a new generation stored. The same with the `RoundTripper` answering the launch's refresh `interaction_required` (devissuer's `RequireInteraction` is `entra`-only, task 12) | a dead token that leaves the user on a sign-in button (a click at launch); a dead token kept and retried |
| `TestStartWithNoAccountSignsInAtOnce` | an empty store → `Start` calls `OpenBrowser` once and `Status` is `signing_in`; a partial write (chunks, no record) → the same, after the sweep | a first launch that waits for a click |
| `TestStartOfflineNeverOpensTheBrowser` | the issuer stopped (connection refused); then a `503`; then a hang past `StartTimeout` on the injected clock → each: `Start` returns `{signed_out, offline}`, `OpenBrowser` called **0** times, the store entry by entry unchanged. Then the issuer restarted and `RetryEvery` fired on the injected `After` → one request, `signed_in`, `OpenBrowser` still 0 | **a refresh that falls back to the browser when it should not** (a network error read as a dead token); a store deleted by an outage; offline for ever after the network returns |
| `TestStartOnAMisconfiguredClientNeverOpensTheBrowser` | the `RoundTripper` answers the refresh with `400 {"error":"invalid_client"}`, then `{"error":"invalid_scope"}` → `{signed_out, ""}`, `OpenBrowser` 0, nothing deleted, no retry request on the next `RetryEvery` tick | a browser opened (and a second refresh token minted) for a fault a sign-in cannot fix |
| `TestTokenNeverOpensTheBrowser` | signed in; then `RevokeRefreshTokens(grace)` mid-session and `Token(HubAudience)` → `ErrSignedOut`, `reauth_required`, `OpenBrowser` **0** (only `Start` and `POST /v1/auth/sign-in` begin a sign-in) | a browser tab popping up mid-session from a background call |
| `TestStartSettlesBeforeTheFirstPoll` | `Start` returns only after its redemption answered or `StartTimeout` passed; `Status()` read straight after is never `signed_out, ""` for a good stored token | a renderer that sees "signed out" for a moment at every launch |

#### Steps

- [ ] **Step 1:** Re-read contract §6.2 and §9.1; phase 8's HA-2 (`2026-09-23-phase-8-own-environment.md:220-238`, as ADR-H1 cites it); the OD-30 ruling sections 2, 3 and 7 (on `main`).
- [ ] **Step 2:** Tests; they fail. Then `tokens.go`, `account.go`, `startup.go`, and `Invalidate` with its compile-time check against task 04's `hubclient.Invalidator`. Task 30's `Config` gains `StartTimeout`, `RetryEvery` and `After`, and its `Reason` enum gains `offline` (`ReasonOffline`).
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/signin/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 53 on Windows, 52 elsewhere (task 30's 25/24 + 22 in tokens_test + 6 in startup_test; OD-30 U-4b splits the rotation pair into three)
go test ./control-plane/internal/signin/ -race -count=20 -run 'TestTenCallers|TestRedemptionsAreSerialised'   # ok — twenty runs, no flake
go test ./control-plane/internal/signin/ -count=1 -run 'TestStartOfflineNeverOpensTheBrowser|TestStartOnADeadTokenSignsInAtOnce|TestTokenNeverOpensTheBrowser' -v 2>&1 | grep -c "^--- PASS"   # 3
grep -rn "AzureDevOpsScope\|PANACEA_APP_ENTRA_DEVOPS\|DevOps struct" control-plane/internal/signin/   # no output
```

#### What this task does not claim

- **That one offline refresh token redeems all four audiences, one audience per token, at Panacea's
  dev realm** (ruling U-4, U-5): task 43's check against the dev realm. Against devissuer it is
  proved here; if the realm ignores the narrowing `scope`, tokens carry every audience and **every
  validator refuses them** (`ErrAudience`: a second `panacea-app-*` audience, ruling §5, U-4), a loud
  failure at task 43, never a quietly degraded defence (review-1 I2).
- **That the realm's narrowed refresh returns a refresh token with the original scope** (ruling U-4b):
  task 43 checks it. This task does not depend on it either way: it never writes a narrowed grant's
  token back, and relies on Panacea's `revokeRefreshToken:false` keeping the original offline token
  valid (facts `:94-96`; if the realm ever rotates, that is a ruling change).
- **Explicit sign-out, then a relaunch.** No record means `Start` begins a sign-in at once, and with
  zero clicks the user is signed back in. The ruling does not say whether a deliberate sign-out
  should survive a relaunch; nothing here remembers one. Owner's call, recorded, not decided here.
- Mid-session, a dead refresh token is **not** re-signed-in automatically: `Token` signs out with
  `reauth_required` and the renderer's sign-in action (task 41) begins it. The ruling's "at once"
  is for launch (section 3); a browser tab from a background call is refused (`TestTokenNeverOpensTheBrowser`).
- That two control planes signed in to one account on one machine are safe. The single lock and
  the start-up sweep assume one control plane per Windows user. The generations still stop a torn
  token: random names mean two writers never share a chunk. But a second process starting mid-sign-in-write
  could sweep the first process's new generation before its record is written. The next start then
  reads a record naming a missing generation, which is a clean `signed_out`, not a wrong token.
  Unverified: nothing in the tree enforces one control plane per user.
- Continuous Access Evaluation: not requested (the client declares no `cp1` capability, §6.4).
- **Keycloak's refusal bodies** (A12): that a revoked or expired offline session answers
  `invalid_grant` and a scope missing on the client answers `invalid_scope` on the refresh grant is
  from knowledge, not observed at the dev realm; the tests forge the rest, and task 43's R-8 records
  the real ones. The Entra-body branches (`consent_required`/65001, 50076/50079) never fire under
  the `keycloak` profile.
- **Every audience is now in task 30's authorize request** (ruling section 2 step 4; this part used
  to decline it). The four are optional client scopes on one realm client, so none can fail the
  sign-in the way an unconsented Entra resource could; a missing one fails only its own `Token`
  (`invalid_scope` → `ErrScopeNeedsConsent`).
- **No per-audience step-up sign-in in v1** (re-review b04 N1): nothing in the product clears an
  `ErrScopeRefused`, which is why its text names no interactive step; the remedy is the realm admin's.
- **Azure DevOps**: nothing here. It is a PAT per organisation (task 29, ruling section 7).
