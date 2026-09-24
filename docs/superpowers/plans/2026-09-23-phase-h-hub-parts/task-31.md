### Task 31: `entraauth` — the `TokenSource`: refresh, rotation, sign-out, start-up; and the Azure DevOps credential (HA-2, H-1)

**Lane C. Depends on: 30; 04 (`hubclient.Invalidator` and `ErrSignedOut`, through 30 too). About 4.5 hours. Contract task C1; corrections A12 and A13.**

After sign-in, every Hub call, every phase 8 broker call and — if ADR-H1 Q5 is yes — every Azure
DevOps push asks `Token(ctx, scope)` for the user's own access token (contract §6.2). **One refresh
token serves every resource** the desktop client is consented for, so there is one sign-in, not
three. This task is phase 8's **HA-2** exactly, and the Azure DevOps half of phase 3's **H-1**.

**One refinement the contract does not state, and why it is needed.** The contract makes refresh
single-flight **per scope**. But the scopes share **one** refresh token and Entra rotates it on
redemption, so two scopes refreshing at once would redeem the same token twice — and `devissuer`,
stricter than Entra by design (contract §6.4), refuses the second as spent, which would sign the
user out. So redemptions are **serialised across all scopes** by one lock, and each redemption
reads the refresh token current at its start; callers of one scope still share one redemption.

**Files:**
- `control-plane/internal/entraauth/{tokens.go,account.go,devops.go}` + `{tokens_test.go,devops_test.go}` (new)

(No `hubclient.go` edit: task 04 declares `Invalidator`, so this lane-C task touches no lane-A file,
review M15.)

#### Contract — complete

```go
// control-plane/internal/entraauth/tokens.go
package entraauth

// Scopes in v1 (contract §6.2).
const (
	// AzureDevOpsScope is Azure DevOps' resource id + user_impersonation; the id is
	// from the contract, unverified (contract §15). Used only when Q5 is yes.
	AzureDevOpsScope = "499b84ac-1321-427f-aa17-267ca6975798/user_impersonation"
)

// Scope-specific refusals (review I2). Neither signs the user out; each wraps a
// *ScopeRefusal naming the scope and Entra's code, so the caller can surface
// "Azure DevOps refused PanaceaApp's sign-in" rather than "sign in again".
// Neither names an interactive step (re-review b04 N1): v1's only interactive
// sign-in is BeginSignIn, which asks for Hub.Access alone, so no product action
// performs a per-resource step-up. The remedies are the store's (task 39's
// POST /v1/credentials/azure-devops) and IT's (consent, a policy change).
var (
	ErrScopeNeedsConsent = errors.New("entraauth: this resource needs consent")
	ErrScopeRefused      = errors.New("entraauth: this resource refused this sign-in")
)

type ScopeRefusal struct {
	Scope    string
	Code     string // "invalid_grant" | "interaction_required" | "invalid_scope"
	Suberror string // Entra's suberror, e.g. "consent_required"; "" if none
	Err      error  // ErrScopeNeedsConsent or ErrScopeRefused
}

func (r *ScopeRefusal) Error() string // "entraauth: <scope>: <Code>/<Suberror>"
func (r *ScopeRefusal) Unwrap() error // r.Err

// refreshSkew is how long before exp a cached token stops being served
// (review I9): min(5 min, lifetime/2), lifetime = exp − the Now() at which the
// token response was received. An hour's token is refreshed 5 min early, as
// before; a 90 s token is served for 45 s, not redeemed on every call. All times
// come from Config.Now, never time.Now.
func refreshSkew(lifetime time.Duration) time.Duration

// Token serves the cached access token for scope while exp − now > refreshSkew(lifetime);
// otherwise it redeems the refresh token: POST token_endpoint with
// grant_type=refresh_token, client_id, refresh_token, scope="<scope> offline_access".
//   - Callers for one scope share one redemption (single-flight); redemptions for
//     all scopes are serialised by one lock (the refinement above).
//   - A response carrying a new refresh_token is written BEFORE the access token is
//     returned, by task 28's generation sequence (review I1): chunks under a NEW
//     generation (NewGeneration, EntraRefreshGenPrefix); then the account record,
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
//     resource is not a dead refresh token.
//       · Redeeming HubScope: invalid_grant (any suberror) or interaction_required
//         → both entries deleted, every cached token dropped, Status
//         {signed_out, reauth_required}, ErrSignedOut to every caller waiting on it.
//         The Hub is the sign-in's own resource, so a refusal for it means the
//         session is over.
//       · Redeeming any OTHER scope (the broker's, AzureDevOpsScope):
//         invalid_grant with suberror consent_required or error_codes holding
//         65001 → ErrScopeNeedsConsent; interaction_required (or invalid_grant
//         with error_codes holding 50076 or 50079) → ErrScopeRefused.
//         Both wrap a *ScopeRefusal{Scope, Code, Suberror}. NOTHING is deleted,
//         the refresh token is kept, Status is unchanged, only that scope's
//         cached token is dropped, and the other scopes keep serving.
//       · Any OTHER invalid_grant on a non-Hub scope (no consent suberror, no
//         50076/50079) may be a dead refresh token or a per-resource refusal, and
//         only the Hub scope can tell: under the same redemption lock, Token
//         redeems HubScope once. Hub refuses too → the sign-out path above, and
//         ErrSignedOut to the non-Hub caller as well; Hub succeeds (its rotation
//         written as above) → ErrScopeRefused for that scope. Root-cause
//         ruling: a non-Hub refusal never deletes the one credential the Hub
//         still redeems, and a dead token is not left in the store for an hour
//         while the Hub's cached access token lives. If the probe's own HubScope
//         redemption answers unreachable, a timeout or a 5xx → ErrEntraUnavailable
//         to the non-Hub caller; if it answers any 4xx the Hub rule does not list
//         → the error naming that code (the "any other 4xx" rule below). In both,
//         nothing is deleted, Status is unchanged and nothing is remembered
//         (re-review b04 m4).
//       · A *ScopeRefusal is REMEMBERED per scope (re-review b04 m4) and served
//         with no request until the first of: refreshGen changes (a new sign-in,
//         or any rotation written as above), Invalidate(scope), or refusalBackoff
//         (15 min) passes on Config.Now. It is remembered against the refreshGen
//         current AFTER the probe's own rotation, so the probe never clears the
//         refusal it produced. So phase 3's Fetcher, asking AuthFor per
//         project per tick (phase-3 task-12.md:56, task-13.md:190), costs one
//         redemption per scope per back-off, not one per project, and the plain
//         invalid_grant probe does not rotate the refresh token on every call. A
//         sign-out forgets every remembered refusal with the rest.
//       · invalid_scope on a non-Hub scope (the resource is not in the client's
//         granted permissions: task 32's PANACEA_APP_ENTRA_DEVOPS=true before IT's
//         consent) → ErrScopeNeedsConsent, the same keep-everything rule.
//       · Any other 4xx body, on any scope (invalid_client, unauthorized_client,
//         invalid_request, invalid_scope on HubScope, an unknown code) → an error
//         naming the code (and error_codes), NOT ErrSignedOut: nothing is deleted
//         and Status is unchanged. Only invalid_grant and interaction_required
//         prove a refresh token dead, and only for HubScope; a misconfiguration
//         never costs the user their sign-in.
//       · The refresh token is NOT rotated by a refused redemption (no new
//         refresh_token in an error body), so the store is not written.
//   - Entra unreachable, a timeout, a 5xx → ErrEntraUnavailable; the stored token is kept.
//   - Not signed in → ErrSignedOut, with no network call.
func (a *Authenticator) Token(ctx context.Context, scope string) (string, error)

// Invalidate drops the cached access token for scope, so the next Token redeems.
// hubclient calls it before its one retry on a 401 with reason "expired" (contract §4.3).
func (a *Authenticator) Invalidate(scope string)

// SignOut deletes the account record FIRST, then every generation's chunks
// (DeleteChunked(EntraRefreshPrefix)), and every cached token; a crash in between
// leaves chunks with no record, which start-up sweeps. Status becomes {signed_out, ""}. It does not call Entra's logout
// endpoint in v1: the browser session is the user's, not ours (contract §6.2).
func (a *Authenticator) SignOut(ctx context.Context) error
```

```go
// control-plane/internal/entraauth/account.go — start-up (contract §6.2).
//
// New reads EntraAccountTarget(tid, ClientID) for the configured authority's tenant:
// tid is the authority's one path segment, which task 32's config guarantees is a
// lowercase tenant GUID (review M8), so New needs no network and no List. New
// itself refuses a Config.Authority whose path is not one GUID (an error, not a
// guess), so a caller that bypasses task 32's loader cannot look under a domain
// or "organizations" and find nothing.
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
//     Status {signed_in}, Account set, NOTHING redeemed eagerly: the first Token call
//     proves it; then SweepGenerations(EntraRefreshPrefix, refreshGen) deletes every
//     generation the record does not name (a rotation that failed or died before its
//     record Put, or after it but before the old generation's delete).
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
// control-plane/internal/entraauth/devops.go

// DevOps implements phase 3's credstore.Store for Azure DevOps with the Entra
// token (ADR-H1 Q5): Scheme bearer, Source "entra". Enabled is Q5's answer,
// PANACEA_APP_ENTRA_DEVOPS (task 32); false makes it answer ErrNoCredential.
// In phase 3's chain it comes AFTER credstore.LocalDevOps (task 29, review I10):
// Local → LocalDevOps → DevOps → GitHelper, so a PAT the user pasted for an
// organisation wins, and with Enabled false the store still serves Azure DevOps.
type DevOps struct {
	Tokens  TokenSource
	Enabled bool
}

// Credential: a host that is not Azure DevOps, or Enabled false → ErrNoCredential.
// ErrSignedOut → ErrNoCredential (a Chain then moves on to the git credential
// helper). ErrEntraUnavailable → returned as is, stopping the Chain: "Entra is
// down" must not silently become "use another credential" (phase 3 task 13 :87-89).
// ErrScopeNeedsConsent and ErrScopeRefused (review I2) → returned as is,
// wrapped *ScopeRefusal and all, also stopping the Chain, by phase 3's Chain rule
// (phase 3 task 13 :86-89): a store that answers with a refusal has spoken, and a
// refusal must not silently become another credential. ErrSignedOut differs
// because it is absence, not refusal: this store has no user to ask for, which is
// exactly ErrNoCredential (re-review b04 N1). The Hub session is untouched.
func (d DevOps) Credential(ctx context.Context, h credstore.Host) (credstore.Credential, error)
```

`hubclient.Invalidator` (`Invalidate(scope string)`) is declared by **task 04**, in lane A's
`hubclient.go` (review M15). This task only implements it: `*Authenticator` gets
`func (a *Authenticator) Invalidate(scope string)`, and a compile-time
`var _ hubclient.Invalidator = (*Authenticator)(nil)` in `tokens.go` holds the two together.

#### Tests to write first — `tokens_test.go`, `devops_test.go`

Each against `devissuer` in-process with `AccessTTL` set short where expiry matters, and an
injected clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACachedTokenIsServedUntilFiveMinutesLeft` | `AccessTTL` 60 min: at exp − 6 min → the cached token, no request; at exp − 4 min → a redemption | a redemption per call; a token served until it expires in flight |
| `TestTheRefreshSkewIsHalfAShortLifetime` | `refreshSkew` table: 60 min → 5 min, 10 min → 5 min, 9 min → 4 min 30 s, 90 s → 45 s, 0 → 0. Then `AccessTTL` 90 s through `Token`, on the injected clock: at issue + 44 s → the cached token, no request; at issue + 46 s → one redemption (review I9) | the fixed 5-minute skew, under which any token shorter than 5 min is redeemed on every call; a skew read from `time.Now` instead of `Config.Now` |
| `TestTenCallersOneRedemption` | 10 goroutines, one scope, expired cache → exactly 1 token-endpoint request (contract §13 C1) | a thundering herd on the token endpoint |
| `TestRedemptionsAreSerialisedAcrossScopes` | Hub and broker scopes expire together; 5 callers each, concurrently → both succeed; the endpoint saw 2 requests, never two with the same refresh token | the per-scope-only single-flight, which signs the user out under rotation |
| `TestRotationIsWrittenBeforeTheTokenIsReturned` | a recording `Secrets`: in order, the new generation's chunks, then the account record naming that `refreshGen` and `refreshChunks`, then the old generation's deletes — all before `Token` returns; **no `Put` targets any chunk of the old generation** | a crash between return and write losing the only valid refresh token; the in-place overwrite (review I1); an old generation deleted before the record names the new one |
| `TestAFailedRotationWriteKeepsTheOldToken` | signed in (the old token, generation A); `Config.HTTPClient`'s `RoundTripper` swaps the refresh grant's `refresh_token` for a **6,000-byte** value (3 chunks — devissuer issues 43-character tokens, task-12.md:71, too small to need a second chunk); a faulting `Secrets` fails the **second** chunk's `Put` of that write → `Token` errors; the record still names A; `GetChunked(A, n)` returns the old token **whole, byte for byte**; `List(EntraRefreshPrefix)` holds only A's chunks; a fresh `New` over the same store → `signed_in`. A second case fails the record's `Put` instead: the same assertions | a torn token (new chunk 0 + old chunk 1, review I1) that signs the user out at the next start; a fault injected only on the first `Put`, which never tests the tear. (Whether the old token still redeems is Entra's rule, not this test's: devissuer revokes on rotation, task-12.md:101) |
| `TestStartUpSweepsUnnamedGenerations` | the store holds the record naming B, B's chunks, and a stray generation C (a rotation that died before its record `Put`) and A (one that died before its old-generation delete); `New` → `signed_in`, and `List(EntraRefreshPrefix)` is exactly B's chunks | leaked refresh tokens piling up in Credential Manager; a sweep that deletes the named generation |
| `TestNewFindsTheRecordByTheAuthoritysTenant` (review M8) | a record stored under `EntraAccountTarget(t1, c)`: `New` with `Authority` `…/<t1>` → `signed_in`, and with `…/<T1 uppercased>` → `signed_in` (read lowercased); `…/organizations` and `…/kpmg.com.au` → an error from `New`, not `signed_out`; a recording `Secrets` sees that exact target as the first `Get`, and every `List` (the start-up sweep's) under `PanaceaApp/entra/<t1>/<c>/`, none wider | a restart that looks under the wrong tid and signs the user out for good; a lookup that reads every account on the machine |
| `TestInvalidGrantSignsOut` | `devissuer.RevokeRefreshTokens(grace)`; `Token(HubScope)` with its cache expired → `ErrSignedOut`; `Status` `reauth_required`; the store is empty | a dead refresh token retried on every call |
| `TestInteractionRequiredSignsOut` | `RequireInteraction(grace, true)`; `Token(HubScope)` → the same | a conditional-access sign-in frequency never surfaced to the user |
| `TestASecondaryScopeRefusalKeepsTheSession` | signed in, the Hub token cached. `Config.HTTPClient`'s `RoundTripper` answers the **broker** scope's refresh grant itself, never forwarding it (so devissuer's refresh token is not spent), with Entra's body `{"error":"invalid_grant","suberror":"consent_required","error_codes":[65001]}` → `Token(brokerScope)` is `ErrScopeNeedsConsent`, `errors.As` a `*ScopeRefusal` whose `Scope` is the broker's; `Status` still `signed_in`; the store, compared entry by entry, **unchanged**; then, the Hub's cache expired, `Token(HubScope)` succeeds with a real redemption of the kept refresh token. Case 2: `{"error":"interaction_required","error_codes":[50076]}` → `ErrScopeRefused`, the same assertions. Case 3, devissuer's own body: `RequireInteraction(grace, true)` with the Hub token still cached → `Token(brokerScope)` is `ErrScopeRefused`, the store unchanged. Case 4: `{"error":"invalid_scope","error_codes":[70011]}` for the broker's grant → `ErrScopeNeedsConsent`, the same assertions (review I2) | the scope-blind rule, under which a conditional-access policy on Azure DevOps signs the user out of the Hub at every push; a refusal that drops every scope's cached token |
| `TestAPlainInvalidGrantOnASecondaryScopeAsksTheHub` | Case 1: `RevokeRefreshTokens(grace)`, the Hub token cached → `Token(brokerScope)` is `ErrSignedOut`; the endpoint saw exactly two requests, the broker's then the Hub's; `Status` `reauth_required`; the store is empty. Case 2: the `RoundTripper` answers the broker's grant with a bare `{"error":"invalid_grant"}` and forwards the Hub's → `Token(brokerScope)` is `ErrScopeRefused`; `Status` `signed_in`; the record names the Hub redemption's new generation (review I2). Case 3: the broker's grant answered with a bare `invalid_grant`, the Hub's probe answered `503` → `ErrEntraUnavailable`. Case 4: the probe answered `400 {"error":"invalid_client"}` → an error naming `invalid_client`, neither `ErrSignedOut` nor a `*ScopeRefusal`. In both, `Status` `signed_in`, the store entry by entry unchanged, and the next `Token(brokerScope)` makes a request (nothing remembered; re-review b04 m4) | a dead refresh token kept for the life of the Hub's cached token; a per-resource refusal that signs the user out; a probe outage read as a dead token, or remembered as a refusal |
| `TestARefusalIsRememberedUntilTheGenerationChanges` | the `RoundTripper` answers the broker's grant with `consent_required`/65001, counting requests. `Token(brokerScope)` → `ErrScopeNeedsConsent`; 10 more calls → the same `*ScopeRefusal`, **0** further requests. The plain-`invalid_grant` case: 10 more calls → 0 further Hub probes, and the record's `refreshGen` is unchanged. Then each of three clears, separately: the Hub redeems and rotates (`refreshGen` changes); `Invalidate(brokerScope)`; the injected clock advanced 15 min → the next `Token(brokerScope)` makes exactly 1 request. At 14 min 59 s → still 0. After `SignOut` and a new sign-in → 1 request (re-review b04 m4) | a refusal re-redeemed on every call, and a full rotation written to Credential Manager per project per fetch tick; a refusal remembered forever, so IT's consent is never seen; a back-off read from `time.Now` |
| `TestAnUnlistedRefusalDeletesNothing` | the `RoundTripper` answers the **Hub** scope's refresh grant with `400 {"error":"invalid_client"}`, then `{"error":"invalid_scope"}`, then `{"error":"something_new"}`, never forwarding → each `Token(HubScope)` errors naming the code, and is neither `ErrSignedOut` nor `ErrEntraUnavailable`; `Status` still `signed_in`; the store, entry by entry, unchanged; with the `RoundTripper` removed, the next `Token(HubScope)` redeems the kept refresh token (review I2) | an "any 4xx signs out" rule, under which a misconfigured client id or scope deletes every user's refresh token |
| `TestAnUnreachableEntraKeepsTheToken` | the issuer stopped → `ErrEntraUnavailable`; `Status` still `signed_in`; the store unchanged | a network blip signing the user out |
| `TestStartUpFromTheAccountRecord` | a store pre-filled by a previous sign-in; `New` → `signed_in` with the account, **no** request until `Token`. A second case wraps the same store in a faulting `Secrets` whose `Get` of the record returns an error that is **not** `ErrNotFound` → `New` returns `signed_out` without error, and the store, compared entry by entry before and after, is **untouched**: no `Delete`, no sweep (plan re-review b03 m3) | an eager redemption at every start; a transient read failure treated as "absent", which deletes a valid refresh token |
| `TestAPartialWriteIsNotAnAccount` | chunks but no account record → `signed_out`, every generation deleted; a record whose named generation lacks a chunk → `signed_out`, record and every generation deleted; and a record with `"refreshGen":"XYZ"`, and one with `"refreshChunks":0` and one with `17`, each over a valid generation's chunks → `New` returns `signed_out` **without panicking**, record and every generation deleted (plan re-review b03 N1) | a half-written sign-in treated as complete; a stored gen passed unchecked to a builder, which panics at every start |
| `TestSignOutDeletesEverything` | → store empty, `Token` → `ErrSignedOut` without a request | a refresh token left behind (the machine's next user) |
| `TestInvalidateForcesARedemption` | a fresh cached token; `Invalidate`; `Token` → a request | the 401-expired retry of contract §4.3 retrying with the same token |
| `TestOneRefreshTokenServesTwoResources` | after one sign-in, `Token(HubScope)` and `Token(brokerScope)` both succeed, each `aud` its API | a second sign-in for the broker (phase 8) |
| `TestErrSignedOutIsHubclients` | `errors.Is(entraauth.ErrSignedOut, hubclient.ErrSignedOut)` | the relay missing a signed-out error |
| `TestDevOpsCredential` | GitHub host → `ErrNoCredential`; `Enabled: false` → `ErrNoCredential`; enabled → `{Scheme: bearer, Source: "entra"}`; signed out → `ErrNoCredential`; Entra unreachable → that error, not `ErrNoCredential`; a `TokenSource` answering `ErrScopeNeedsConsent`, and one answering `ErrScopeRefused` → that error, not `ErrNoCredential`, and a `credstore.Chain{DevOps, <a store that records calls>}` never asks the second store (review I2) | an ADO token sent to GitHub; an outage silently falling through the chain; an ADO refusal silently becoming another credential, against phase 3's `Chain` rule (re-review b04 N1) |

#### Steps

- [ ] **Step 1:** Re-read contract §6.2 and §9.1; phase 8's HA-2 (`2026-09-23-phase-8-own-environment.md:220-238`, as ADR-H1 cites it); phase 3 task 13's `Chain`.
- [ ] **Step 2:** Tests; they fail. Then `tokens.go`, `account.go`, `devops.go`, and `Invalidate` with its compile-time check against task 04's `hubclient.Invalidator`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/entraauth/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 40 (18 from task 30 + 22; review M8 added one)
go test ./control-plane/internal/entraauth/ -race -count=20 -run 'TestTenCallers|TestRedemptionsAreSerialised'   # ok — twenty runs, no flake
```

#### What this task does not claim

- **That one KPMG refresh token redeems the Hub, the broker and Azure DevOps** (ADR-H1 §11): the
  consent and the ADO resource id are checked at task 43's real-tenant sign-in.
- That Entra keeps an old refresh token valid after rotation; the code does not rely on it.
- That two control planes signed in to one account on one machine are safe. The single lock and
  the start-up sweep assume one control plane per Windows user. The generations still stop a torn
  token: random names mean two writers never share a chunk. But a second process starting mid-rotation
  could sweep the first process's new generation before its record is written. The next start then
  reads a record naming a missing generation, which is a clean `signed_out`, not a wrong token.
  Unverified: nothing in the tree enforces one control plane per user.
- Continuous Access Evaluation: not requested (the client declares no `cp1` capability, §6.4).
- **Entra's per-resource refusal bodies** (A12): `consent_required`/65001, 50076/50079 and
  `invalid_scope` for one resource while the refresh token redeems another are from Microsoft's
  documentation as this plan understands it, not observed; the tests forge them, and task 43's R-8
  records the real ones.
- **The broker and Azure DevOps scopes are not added to task 30's authorize request** (the review's
  optional step, declined). The desktop client is **admin**-consented for each resource (contract
  §6.2 `:404-407`, §6.5), so asking the user at sign-in adds no consent. And a scope the tenant has
  not consented, such as Azure DevOps while Q5 is unanswered, would fail the whole sign-in,
  the Hub's included, where A12 fails only that resource. Task 30's step 8 also sends step 4's
  scope unchanged, and a token request naming two resources is refused (from knowledge, unverified).
- **No per-resource step-up sign-in in v1** (re-review b04 N1). An authorize request naming one
  non-Hub scope with the refusal's `claims` challenge is not built, so nothing in the product clears
  an `ErrScopeRefused`. That is why its text names no interactive step. Its remedies are a pasted
  Azure DevOps PAT (task 39's `POST /v1/credentials/azure-devops`, served first by `LocalDevOps`),
  `PANACEA_APP_ENTRA_DEVOPS=false`, or IT. Whether a fresh Hub-only `BeginSignIn` clears an Azure
  DevOps 50076 is unknown (from knowledge: a loopback public-client flow carries no device claim, so
  a device-compliance policy is never satisfied by it; unverified). Task 43's R-8 records it, so the
  owner can rule on step-up with evidence.
