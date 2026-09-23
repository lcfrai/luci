### Task 31: `entraauth` — the `TokenSource`: refresh, rotation, sign-out, start-up; and the Azure DevOps credential (HA-2, H-1)

**Lane C. Depends on: 30. About 3 hours. Contract task C1.**

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
- `control-plane/internal/hub/hubclient/hubclient.go` — the optional `Invalidator` (modify: one declaration)

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

// Token serves the cached access token for scope while exp − now > 5 min;
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
//   - invalid_grant or interaction_required → both entries deleted, every cached
//     token dropped, Status {signed_out, reauth_required}, ErrSignedOut to every
//     caller waiting on it.
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
// New reads EntraAccountTarget(tid, ClientID) for the configured authority's tenant.
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
type DevOps struct {
	Tokens  TokenSource
	Enabled bool
}

// Credential: a host that is not Azure DevOps, or Enabled false → ErrNoCredential.
// ErrSignedOut → ErrNoCredential (a Chain then moves on to the git credential
// helper). ErrEntraUnavailable → returned as is, stopping the Chain: "Entra is
// down" must not silently become "use another credential" (phase 3 task 13 :87-89).
func (d DevOps) Credential(ctx context.Context, h credstore.Host) (credstore.Credential, error)
```

```go
// control-plane/internal/hub/hubclient/hubclient.go — added.

// Invalidator is optionally implemented by a TokenSource (entraauth does). The
// client calls it before its one retry on 401 unauthenticated, reason "expired".
type Invalidator interface {
	Invalidate(scope string)
}
```

#### Tests to write first — `tokens_test.go`, `devops_test.go`

Each against `devissuer` in-process with `AccessTTL` set short where expiry matters, and an
injected clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACachedTokenIsServedUntilFiveMinutesLeft` | at exp − 6 min → the cached token, no request; at exp − 4 min → a redemption | a redemption per call; a token served until it expires in flight |
| `TestTenCallersOneRedemption` | 10 goroutines, one scope, expired cache → exactly 1 token-endpoint request (contract §13 C1) | a thundering herd on the token endpoint |
| `TestRedemptionsAreSerialisedAcrossScopes` | Hub and broker scopes expire together; 5 callers each, concurrently → both succeed; the endpoint saw 2 requests, never two with the same refresh token | the per-scope-only single-flight, which signs the user out under rotation |
| `TestRotationIsWrittenBeforeTheTokenIsReturned` | a recording `Secrets`: in order, the new generation's chunks, then the account record naming that `refreshGen` and `refreshChunks`, then the old generation's deletes — all before `Token` returns; **no `Put` targets any chunk of the old generation** | a crash between return and write losing the only valid refresh token; the in-place overwrite (review I1); an old generation deleted before the record names the new one |
| `TestAFailedRotationWriteKeepsTheOldToken` | signed in (the old token, generation A); `Config.HTTPClient`'s `RoundTripper` swaps the refresh grant's `refresh_token` for a **6,000-byte** value (3 chunks — devissuer issues 43-character tokens, task-12.md:71, too small to need a second chunk); a faulting `Secrets` fails the **second** chunk's `Put` of that write → `Token` errors; the record still names A; `GetChunked(A, n)` returns the old token **whole, byte for byte**; `List(EntraRefreshPrefix)` holds only A's chunks; a fresh `New` over the same store → `signed_in`. A second case fails the record's `Put` instead: the same assertions | a torn token (new chunk 0 + old chunk 1, review I1) that signs the user out at the next start; a fault injected only on the first `Put`, which never tests the tear. (Whether the old token still redeems is Entra's rule, not this test's: devissuer revokes on rotation, task-12.md:101) |
| `TestStartUpSweepsUnnamedGenerations` | the store holds the record naming B, B's chunks, and a stray generation C (a rotation that died before its record `Put`) and A (one that died before its old-generation delete); `New` → `signed_in`, and `List(EntraRefreshPrefix)` is exactly B's chunks | leaked refresh tokens piling up in Credential Manager; a sweep that deletes the named generation |
| `TestInvalidGrantSignsOut` | `devissuer.RevokeRefreshTokens(grace)` → `ErrSignedOut`; `Status` `reauth_required`; the store is empty | a dead refresh token retried on every call |
| `TestInteractionRequiredSignsOut` | `RequireInteraction(grace, true)` → the same | a conditional-access sign-in frequency never surfaced to the user |
| `TestAnUnreachableEntraKeepsTheToken` | the issuer stopped → `ErrEntraUnavailable`; `Status` still `signed_in`; the store unchanged | a network blip signing the user out |
| `TestStartUpFromTheAccountRecord` | a store pre-filled by a previous sign-in; `New` → `signed_in` with the account, **no** request until `Token`. A second case wraps the same store in a faulting `Secrets` whose `Get` of the record returns an error that is **not** `ErrNotFound` → `New` returns `signed_out` without error, and the store, compared entry by entry before and after, is **untouched**: no `Delete`, no sweep (plan re-review b03 m3) | an eager redemption at every start; a transient read failure treated as "absent", which deletes a valid refresh token |
| `TestAPartialWriteIsNotAnAccount` | chunks but no account record → `signed_out`, every generation deleted; a record whose named generation lacks a chunk → `signed_out`, record and every generation deleted; and a record with `"refreshGen":"XYZ"`, and one with `"refreshChunks":0` and one with `17`, each over a valid generation's chunks → `New` returns `signed_out` **without panicking**, record and every generation deleted (plan re-review b03 N1) | a half-written sign-in treated as complete; a stored gen passed unchecked to a builder, which panics at every start |
| `TestSignOutDeletesEverything` | → store empty, `Token` → `ErrSignedOut` without a request | a refresh token left behind (the machine's next user) |
| `TestInvalidateForcesARedemption` | a fresh cached token; `Invalidate`; `Token` → a request | the 401-expired retry of contract §4.3 retrying with the same token |
| `TestOneRefreshTokenServesTwoResources` | after one sign-in, `Token(HubScope)` and `Token(brokerScope)` both succeed, each `aud` its API | a second sign-in for the broker (phase 8) |
| `TestErrSignedOutIsHubclients` | `errors.Is(entraauth.ErrSignedOut, hubclient.ErrSignedOut)` | the relay missing a signed-out error |
| `TestDevOpsCredential` | GitHub host → `ErrNoCredential`; `Enabled: false` → `ErrNoCredential`; enabled → `{Scheme: bearer, Source: "entra"}`; signed out → `ErrNoCredential`; Entra unreachable → that error, not `ErrNoCredential` | an ADO token sent to GitHub; an outage silently falling through the chain |

#### Steps

- [ ] **Step 1:** Re-read contract §6.2 and §9.1; phase 8's HA-2 (`2026-09-23-phase-8-own-environment.md:220-238`, as ADR-H1 cites it); phase 3 task 13's `Chain`.
- [ ] **Step 2:** Tests; they fail. Then `tokens.go`, `account.go`, `devops.go`, the `Invalidator`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/entraauth/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 34 (18 from task 30 + 16)
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
