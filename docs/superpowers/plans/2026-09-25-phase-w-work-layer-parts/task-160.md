### Task 160: keys and limits: personal and team-scoped API keys, admin-revocable; per-token rate limits with headers

W5, stream WH, seam **SW.3** extended (a key is a principal kind whose rights are its owner's,
narrowed to one team for a team key, and never a state move). Depends on task 159 (the principal
step's additive hook, `appAccess` and its `never` class, the `app/<appId>/<tid>/<oid>` bucket key,
`task-159.md:124-128`, `:141-187`). Types from task 143's part (`APIKey`, `CreatedAPIKey`,
`CreateAPIKeyRequest`, `MaxKeyLifetime`, `task-143.md:427-454`; `ValidateCreateAPIKey`, `:651-661`;
`ActorKey` and `Actor.KeyID`, `:162`, `:175`; `CodeKeyRevoked`, `:781`; the client methods and
route names, `:866-868`, `:949`). Hub anchors read in `feat/hub` at `3df9fab8`: the contract
(cited `hub-contract:N`), Hub task 13's part (the auth pipeline) and Hub task 15's `authz.go`
(`TeamRole`, `ProjectRole`, `H15:39-42`). `server/` and `store/` are **planned, not on
`feat/hub`**, and every path in them is **UNVERIFIED until merge**. 05 §1 L-245, L-252.

A key is how a person's script, or a team's CI job, calls the Hub without a browser sign-in. It is
a long random secret shown once. A **personal** key acts as its owner; a **team** key acts as a
member of one team and nothing else. Either is revoked by its owner, by a team admin (team keys),
or by an org admin (any key in the org), and the revoke holds from the next call.

**The one thing this task must not get wrong** is accepting a credential by its shape. A bearer
that starts `pak_` is checked in full — format, id lookup, a constant-time compare of the hash,
revocation, expiry, and the owner's standing — or it is 401. It never falls through to the
`oidcauth` validator or to any unauthenticated path, and a validator token never reaches the key
path. A prefix that routes a credential is fine; a prefix that *admits* one is the bypass class
this part exists to close.

Seven decisions the scope left open, made here.

1. **Route names are task 143's**, which fixed them after the scope: `/v1/me/api-keys`,
   `/v1/teams/{teamId}/api-keys`, `/v1/api-keys/{keyId}/revoke` (`task-143.md:949`). The scope's
   `/keys` is not used; the client methods already say `APIKey`.
2. **The key path sits in step 3 of §5.1, beside the validator, never before it or after it.**
   Step 3 reads the bearer once: `pak_` goes to `keyauth`, everything else to `oidcauth`, and each
   answer is final. Step 2's per-IP limit (`hub-contract:301`) runs first, so a flood of made-up
   keys costs one indexed read each and is capped at 20/s per address like a flood of bad tokens.
3. **A wrong secret and an unknown id are the same 401.** Only a key whose secret matched learns
   `key_revoked` or `key_expired`; a guesser cannot tell a revoked id from a missing one.
4. **A key never manages credentials.** Keys reuse task 159's `appAccess`: they reach `read` and
   `write` routes, and every `never` route (keys, apps, webhooks, MCP connectors, erase, audit,
   team settings and membership, every org-admin route) is 403 `forbidden`, `details.required
   "never"`. So a leaked key cannot mint a second key, revoke the owner's others, or add a member.
   Revoking is done from the desktop, by a person.
5. **A team key is a member of its team, and its team is the only one it can see.** Two layers:
   `TeamRole` and `ProjectRole` answer `member` for the key's team and "not a member" for every other
   (Go, §5.3's line, `hub-contract:1931-1932`); and a **restrictive** row policy on
   `hub.team_members` under the transaction-local `hub.key_team` hides the creator's other
   memberships, so every "own teams" list (`GET /v1/teams`, `/v1/changes`, `/v1/refs`, search)
   narrows without a per-route change. The policy is one comparison and no join, as §10.3 asks (`hub-contract:1923-1924`).
6. **A key lives on its owner's own sign-ins.** A key call skips `oidcauth`, and the Hub has no
   "left the org" state (`users` has no such column, `hub-contract:1487-1496`), so an offboarded
   person's key would otherwise run to its expiry. The lookup reads the owner's standing on every
   call: `users.last_seen_at` must be within `PANACEA_APP_HUB_KEY_OWNER_IDLE` (default `720h`, 30
   days); `hub.resolve_principal` refreshes it at most hourly on the person's **own** sign-in
   (`hub-contract:1968-1971`), and a key call never touches it. A team key also needs its creator
   still `member` or `admin` of its team. Either failure is 401 `key_revoked`, `details.reason`
   `owner_inactive` or `creator_left_team`. The lookup never writes; the list shows the explicit
   revokes only, and the desktop (task 173) shows an idle owner beside the key.
7. **Keys never move an issue's state.** Task 145's guard refuses `key` every state column write
   (its scope: *"`integration`, `app` and `key` are refused every state column write at all"*), so a
   key cannot close, reopen or fake a fact. A script files, edits, comments and labels; a person
   moves the work.

**Files:**

- `control-plane/internal/hub/server/work_keys.go` (new): the six routes; `keyAuth`, step 3's key
  path; `keyGate` (task 159's `appAccess` read for a key)
- `control-plane/internal/hub/server/work_keys_secret.go` + `work_keys_secret_test.go` (new, pure):
  `MintKey`, `ParseKey`, `HashKey`, `SameHash`
- `control-plane/internal/hub/store/work_keys.go` (new), `store/work_keys_integration_test.go` (new)
- `control-plane/internal/hub/server/work_keys_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/auth.go` ⚑ (Hub task 13's file, `H13:43-124`): step 3's
  branch, `Claims.KeyID` and `Claims.KeyTeamID`, step 4's buckets — the **same proposed
  Hub-contract edit** as task 159's, for the `feat/hub` owner
- `control-plane/internal/hub/server/ratelimit.go` ⚑ (Hub task 13's file, `H13:29`): the `key` and
  `app` buckets, `BucketState`, the three headers — part of the same proposal; its
  `ratelimit_test.go` ⚑ gains `TestTightestAndHeaders`
- `control-plane/internal/hub/server/authz.go` ⚑ (Hub task 15's file, `H15:39-42`): `TeamRole` and
  `ProjectRole` cap a team key — part of the same proposal
- `control-plane/internal/hub/server/tx.go` ⚑ (Hub task 14's file, `H14:46-80`): a key's prologue
  skips `resolve_principal` and sets four settings — part of the same proposal
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's file, its
  decision 7; the scope's `work_matrix_rows.go`): the six routes' rows and the key fixture rows
- `control-plane/cmd/hubopenapi/routes.go` ⚑ (task 158's file): the six entries; then
  `docs/api/openapi.json`, `docs/api/README.md`, `docs/api/roles.json` and the two SDKs'
  generated files regenerated (task 158's gap 5: its drift test forces this)

**Interfaces:**

- Produces: `POST`/`GET /v1/me/api-keys`, `POST`/`GET /v1/teams/{teamId}/api-keys`,
  `GET /v1/api-keys` (org admin), `POST /v1/api-keys/{keyId}/revoke`; `Claims.KeyID`,
  `Claims.KeyTeamID`, `KeyOrgID`, `KeyUserID`; `keyGate`; the buckets `key` and
  `app` and the variables `PANACEA_APP_HUB_RATE_KEY`, `_RATE_KEY_WRITE`, `_RATE_APP`,
  `_RATE_APP_WRITE`, `PANACEA_APP_HUB_KEY_OWNER_IDLE`; `RateLimit-Limit`, `RateLimit-Remaining`,
  `RateLimit-Reset` on every `/v1` response; the settings `hub.work_actor_kind = 'key'`,
  `hub.work_actor_key` and `hub.key_team` on every key-made transaction
- Consumes: task 143's types, validator and codes; task 145's `work_api_keys`, `hub.key_by_id` and
  the guard's `key` branch; task 159's `appAccess`, `Access`, `forbidden`; Hub task 13's
  `Authenticate`, `Claims`, `Limits`; Hub task 15's `TeamRole`, `ProjectRole`
- Consumed by: task 158 (the README's keys and limits sections, `task-158.md:213-218`; the TS
  client's `RateLimit`, `:277-278`); task 161 (`workmcp` refuses a `pak_` bearer, and names this
  part's limits as the Hub's); task 173 (the desktop's keys admin)

#### The secret (`server/work_keys_secret.go`, complete)

```go
package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"regexp"
	"strings"
)

// A key is "pak_" + the 16 hex of its ak- id + "_" + 32 random bytes in
// lowercase base32 without padding (52 characters): 73 characters in all.
// The id is not secret; it is how the row is found without a scan.
const KeyPrefix = "pak_"

var (
	keyShape = regexp.MustCompile(`^pak_([0-9a-f]{16})_([a-z2-7]{52})$`)
	keyB32   = base32.StdEncoding.WithPadding(base32.NoPadding)
)

// MintKey returns the whole secret for the id "ak-<hex16>". It is returned to
// the caller once, in CreatedAPIKey, and never stored.
func MintKey(id string) (secret string, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", err
	}
	return KeyPrefix + strings.TrimPrefix(id, "ak-") + "_" + strings.ToLower(keyB32.EncodeToString(b[:])), nil
}

// ParseKey returns the ak- id of a well-formed key, or ok=false. Anything else,
// including a validator token or a key with one character changed in length,
// is not a key.
func ParseKey(bearer string) (id string, ok bool) {
	m := keyShape.FindStringSubmatch(bearer)
	if m == nil {
		return "", false
	}
	return "ak-" + m[1], true
}

// HashKey is what the row holds: SHA-256 of the WHOLE secret, prefix and id
// included, so a hash cannot be replayed under another id.
func HashKey(secret string) []byte { h := sha256.Sum256([]byte(secret)); return h[:] }

// SameHash compares in constant time; a length mismatch is false.
func SameHash(a, b []byte) bool { return len(a) == sha256.Size && subtle.ConstantTimeCompare(a, b) == 1 }

// ShownOf is APIKey.Shown: the first 8 characters ("pak_" + 4 hex of the id),
// which task 143 fixes and which reveal nothing of the random part.
func ShownOf(secret string) string { return secret[:8] }
```

#### The table and functions — for task 145's migration (complete SQL; **not** applied by this task)

Task 145's scope names `work_api_keys` (*"the secret stored only as a SHA-256 hash; the first 8
characters kept for display"*); its file wins on names. What this part needs, in the style of
`hub-contract:1936-1942` (schema `hub`, `SECURITY DEFINER` with a pinned `search_path`):

```sql
CREATE TABLE work_api_keys (
  id            text        PRIMARY KEY CHECK (id ~ '^ak-[0-9a-f]{16}$'),
  org_id        text        NOT NULL REFERENCES orgs (id),
  owner_id      text        NOT NULL REFERENCES users (id),  -- the person; a team key's creator
  team_id       text        REFERENCES teams (id),           -- NULL: a personal key
  name          text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  shown         text        NOT NULL CHECK (shown ~ '^pak_[0-9a-f]{4}$'),
  secret_sha256 bytea       NOT NULL CHECK (octet_length(secret_sha256) = 32),
  created_at    timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz NOT NULL,
  last_used_at  timestamptz,
  revoked_at    timestamptz,
  revoked_by    text        REFERENCES users (id),
  CHECK (expires_at > created_at AND expires_at <= created_at + interval '8760 hours'),
  CHECK ((revoked_at IS NULL) = (revoked_by IS NULL))
);
CREATE INDEX work_api_keys_owner ON work_api_keys (org_id, owner_id) WHERE team_id IS NULL;
CREATE INDEX work_api_keys_team  ON work_api_keys (org_id, team_id)  WHERE team_id IS NOT NULL;
ALTER TABLE work_api_keys ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_api_keys TO hub_app
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_api_keys TO hub_app;
GRANT UPDATE (revoked_at, revoked_by) ON work_api_keys TO hub_app;  -- never the hash

-- Revoked is final: a revoke is never undone or re-dated.
CREATE FUNCTION work_api_keys_revoke_final() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF OLD.revoked_at IS NOT NULL AND (NEW.revoked_at IS DISTINCT FROM OLD.revoked_at
                                  OR NEW.revoked_by IS DISTINCT FROM OLD.revoked_by) THEN
    RAISE EXCEPTION 'work_api_keys: revoked is final' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER work_api_keys_revoke_final BEFORE UPDATE ON work_api_keys
  FOR EACH ROW EXECUTE FUNCTION work_api_keys_revoke_final();

-- Decision 5's second layer. Unset, a custom setting reads NULL; set once with
-- is_local and committed, it reads '' on the same pooled connection. Both mean
-- "not a team key", hence the coalesce.
CREATE POLICY key_team ON team_members AS RESTRICTIVE TO hub_app
  USING (coalesce(current_setting('hub.key_team', true), '') = ''
         OR team_id = current_setting('hub.key_team', true));

-- Step 3's lookup, before hub.org_id is known (so it must run as the owner).
-- One row or none. It never writes.
CREATE FUNCTION key_by_id(p_id text)
RETURNS TABLE (org_id text, tenant_id uuid, user_id text, entra_oid uuid, team_id text,
               secret_sha256 bytea, expires_at timestamptz, revoked_at timestamptz,
               owner_seen_at timestamptz, creator_role team_role)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT k.org_id, o.entra_tenant_id, k.owner_id, u.entra_oid, k.team_id, k.secret_sha256,
         k.expires_at, k.revoked_at, u.last_seen_at, m.role
  FROM work_api_keys k
  JOIN orgs o  ON o.id = k.org_id
  JOIN users u ON u.id = k.owner_id
  LEFT JOIN team_members m ON m.team_id = k.team_id AND m.user_id = k.owner_id
  WHERE k.id = p_id
$$;

-- lastUsedAt, at most once a minute per key, after the secret matched.
CREATE FUNCTION key_used(p_id text) RETURNS void
LANGUAGE sql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  UPDATE work_api_keys SET last_used_at = now()
  WHERE id = p_id AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')
$$;
REVOKE ALL ON FUNCTION key_by_id(text), key_used(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION key_by_id(text), key_used(text) TO hub_app;
```

`key_by_id` runs as the owner, so it bypasses both policies; that is why it returns the raw row
and Go decides. A **`SECURITY DEFINER` function that reads `team_members` bypasses `key_team`**
too, which is why decision 5 keeps the Go cap as well, and the matrix test is the proof for both.

#### `server/auth.go` and `server/tx.go` ⚑ (the added lines; part of the one proposed edit)

```go
// In AuthConfig (H13:80), three fields. serve.go sets Keys to st.KeyByID and
// KeyUsed to st.KeyUsed. Keys nil → every "pak_" bearer is 401 "key".
	Keys         func(ctx context.Context, keyID string) (store.KeyRow, bool, error)
	KeyUsed      func(ctx context.Context, keyID string) error
	KeyOwnerIdle time.Duration // PANACEA_APP_HUB_KEY_OWNER_IDLE, default 720h

// In Claims (H13:43), four fields. A key's Claims also carry TenantID and
// ObjectID (the owner's, from the row) so step 4's org bucket is unchanged.
	KeyID     string // "" unless the bearer was a key
	KeyTeamID string // a team key's team; "" for a personal key
	KeyOrgID  string // from the row; step 5 uses these and never resolve_principal
	KeyUserID string

// Authenticate, step 3 (H13:110-113), becomes: read the bearer once;
//   strings.HasPrefix(bearer, KeyPrefix) → keyAuth, and its answer is final;
//   otherwise → cfg.Validator.Validate exactly as today (task 159's hook inside it).
// Nothing else changes order: step 2's IP limit is before both, step 3b's service
// rules never see a key (a key's Claims have Service false).

// keyAuth is step 3 for a "pak_" bearer. Every non-nil *APIError is written and
// the request ends. It never calls cfg.Validator, and nothing after it runs
// unless it returned nil.
func keyAuth(ctx context.Context, cfg AuthConfig, bearer string, now time.Time) (Claims, *APIError) {
	id, ok := ParseKey(bearer)
	if !ok || cfg.Keys == nil {
		return Claims{}, unauthenticatedReason("malformed")
	}
	row, found, err := cfg.Keys(ctx, id)
	if err != nil {
		return Claims{}, unavailableErr() // 503; the key is neither accepted nor refused
	}
	if !found || !SameHash(HashKey(bearer), row.SecretSHA256) {
		return Claims{}, unauthenticatedReason("key") // decision 3: one answer for both
	}
	switch {
	case row.RevokedAt != nil:
		return Claims{}, keyRevoked("revoked")
	case !now.Before(row.ExpiresAt):
		return Claims{}, unauthenticatedReason("key_expired") // never "expired": §4.3 refreshes on that
	case now.Sub(row.OwnerSeenAt) > cfg.KeyOwnerIdle:
		return Claims{}, keyRevoked("owner_inactive")
	case row.TeamID != "" && row.CreatorRole != hubapi.RoleMember && row.CreatorRole != hubapi.RoleAdmin:
		return Claims{}, keyRevoked("creator_left_team") // "" (no row) or viewer
	}
	if err := cfg.KeyUsed(ctx, id); err != nil {
		cfg.Log.Warn("key_used", "keyId", id, "err", err) // lastUsedAt is a hint, never a gate
	}
	return Claims{TenantID: row.TenantID, ObjectID: row.EntraOID, ClientID: "key",
		KeyID: id, KeyTeamID: row.TeamID, KeyOrgID: row.OrgID, KeyUserID: row.UserID}, nil
}

// In tx.go (H14:46-80), Principal gains KeyID and KeyTeamID. The Read and Write
// prologue, for Claims.KeyID != "": BEGIN; NO store.ResolvePrincipal (it would
// refresh last_seen_at, which decision 6 reads); Principal from KeyOrgID and
// KeyUserID, OrgAdmin false, Name and Email ""; then, before fn:
//   set_config('hub.org_id', KeyOrgID, true)
//   set_config('hub.work_actor_kind', 'key', true)
//   set_config('hub.work_actor_key', KeyID, true)
//   set_config('hub.key_team', KeyTeamID, true)   -- '' for a personal key
// Events carry Actor{Kind: "key", UserID: KeyUserID, KeyID: KeyID} (task 143).
```

`unauthenticatedReason`, `unavailableErr` and `keyRevoked` stand for Hub task 13's and task 14's
error helpers (their real names are **UNVERIFIED until merge**). Every key 401 carries
`WWW-Authenticate: Bearer error="invalid_token"` (`hub-contract:223`); `key_revoked` is task 143's
401 code with `details.reason`; the others are `unauthenticated` with `details.reason` `malformed`,
`key` or `key_expired`, three reasons added to H13's table.

**The Go cap** (`authz.go` ⚑, Hub task 15): `TeamRole(ctx, tx, teamID)` with `tx.P.KeyTeamID != ""`
returns `(member, true)` when `teamID == KeyTeamID` and the stored role is `member` or `admin`, and
`ok=false` for every other team; `ProjectRole` follows through its team. So an admin's team key is
a member (it cannot rename the team, whatever its creator can), and a key sees no second team even
if the `key_team` policy were dropped.

#### `server/ratelimit.go` ⚑ (the added lines, complete) and which buckets a caller meets

```go
// In Limits (H13:75-78), four rates. Each defaults to §11.3's user rates:
// PANACEA_APP_HUB_RATE_KEY and _RATE_APP "20:60", _RATE_KEY_WRITE and
// _RATE_APP_WRITE "5:20" (hub-contract:2101-2102, :2175).
	Key, KeyWrite, App, AppWrite Rate

// BucketState is one bucket after this request took its token.
type BucketState struct {
	Scope     string        // "ip" | "org" | "user" | "user_write" | "key" | "app"
	Limit     int           // the burst: the most the bucket holds
	Remaining int           // whole tokens left after this request, never below 0
	Reset     time.Duration // until the bucket is full again; 0 when it is full
}

// take reserves one token at now. ok=false when the reservation would wait: the
// reservation is cancelled, Remaining is 0, and wait is the Retry-After.
func take(lim *rate.Limiter, scope string, r Rate, now time.Time) (s BucketState, wait time.Duration, ok bool)

// Tightest is the state the headers report: the least Remaining; a tie goes to
// the smaller Limit, then to the earlier scope in ip, org, user, user_write,
// key, app. A 429's refusing bucket has Remaining 0, so it is always reported.
func Tightest(states []BucketState) BucketState

// WriteRateHeaders sets RateLimit-Limit, RateLimit-Remaining and RateLimit-Reset
// (whole seconds, rounded up) on h. Authenticate calls it after step 2 with the
// ip state, and again after step 4 with Tightest of every state, before any
// status is written, so every /v1 response carries the three, a 401 included.
func WriteRateHeaders(h http.Header, s BucketState)
```

| Caller | Step 4's buckets (limiter key) | 429 `scope` |
|---|---|---|
| a person (desktop) | `user` (`tid/oid`); non-GET `user_write`; `org` (`tid`) — unchanged | `user`, `user_write`, `org` |
| an app (task 159) | `app` (`app/<appId>/<tid>/<oid>`); non-GET the `AppWrite` bucket (`appw/<appId>/<tid>/<oid>`); `org` | `app`, `org` |
| a key | `key` (`key/<ak-id>`); non-GET the `KeyWrite` bucket (`keyw/<ak-id>`); `org` | `key`, `org` |
| a service | `client/<azp>` as `user` — unchanged | `user` |

**A token's bucket is its own, not its person's.** An app's or a key's calls are not charged to
the person's `user` bucket, so a runaway script cannot lock its owner out of the desktop; the `org`
bucket still bounds the sum. What bounds one person's keys is the count: at most **10 live personal
keys** per person and **25 live team keys** per team (live: not revoked, not expired), a
`POST` past it 409 `conflict`, `details.limit`. `rate_limited`'s `scope` list (`hub-contract:269`)
gains `key` and `app`, an additive change. Both write buckets answer with their token's scope, so
the scope names whose bucket ran out and `RateLimit-Limit` says which rate.

`rate.Limiter`'s `TokensAt`, which `take` reads for `Remaining`, is in `golang.org/x/time/rate` from
v0.3.0 (**UNVERIFIED** from this network; the module version is Hub task 13's). The header names are
the ones the scope fixes; that they match any one draft of the IETF rate-limit header work is not
claimed.

#### The routes and their wire (`server/work_keys.go`)

Every key route is `AccessNever` in task 159's `appAccess`, so only a person's own token (the
desktop's) reaches them: an app or a key is 403 `forbidden`, `details.required "never"`.

```jsonc
// POST /v1/me/api-keys                          any person in the org
{ "name": "Nightly release-notes export", "expiresAt": "2027-03-31T00:00:00Z" }
// → 201 CreatedAPIKey { "key": APIKey (scope "personal", teamId null, ownerId the caller),
//                       "secret": "pak_3f2a91c07b1e4d2a_…52 base32…" }  — the only time it is sent
// task 143's ValidateCreateAPIKey → 400 bad_request naming "name" or "expiresAt"
// an 11th live personal key → 409 conflict, details.limit "10"

// POST /v1/teams/{teamId}/api-keys              team admin (H15's Require, want admin)
// the same body → 201 CreatedAPIKey, scope "team", teamId, ownerId the creator
// not a member → 404 team_not_found; viewer or member → 403 forbidden; a 26th live → 409, "25"

// GET /v1/me/api-keys?after=&limit=             → 200 Page[APIKey]: the caller's personal keys
// GET /v1/teams/{teamId}/api-keys?after=&limit= → 200 Page[APIKey]: team admin only, else 403/404
// GET /v1/api-keys?owner=&team=&after=&limit=   → 200 Page[APIKey]: org admin only, every key in
//                                                  the org (§5.3's O column; else 403)
// every list: newest first, revoked and expired included, never a secret or a hash

// POST /v1/api-keys/{keyId}/revoke   {}         → 200 APIKey, revokedAt and revokedBy set
// who: the owner of a personal key; an admin of a team key's team; an org admin, any key
// anyone else, an unknown or malformed id → 404 key_not_found (one answer); revoked → 409 conflict
```

`GET /v1/api-keys` is a sixth route task 143's table does not have (`task-143.md:949`); without it
an org admin cannot find the key they are allowed to revoke. **Events**, audit-only (the admin
class, `task-169.md:578-579`): `api_key.created {scope}` and `api_key.revoked {scope, by}` with `by`
`owner`, `team_admin` or `org_admin`, the actor the person; the key's `teamId` is the event's team.

```go
// keyGate runs after the principal step when Claims.KeyID is set (beside task
// 159's appGate, in the same middleware). A key holds both work scopes, so its
// reach is exactly "not AccessNever".
func keyGate(pattern string) *APIError {
	if appAccess[pattern] == AccessNever {
		return forbidden("never")
	}
	return nil
}
```

#### The store (`store/work_keys.go`, signatures complete)

```go
package store

// KeyRow is hub.key_by_id's row. It holds the hash, so it never leaves the auth step.
type KeyRow struct {
	OrgID, TenantID, UserID, EntraOID, TeamID string // TeamID "" for a personal key
	SecretSHA256                              []byte
	ExpiresAt, OwnerSeenAt                    time.Time
	RevokedAt                                 *time.Time
	CreatorRole                               hubapi.Role // "" when not a member of TeamID
}

// KeyByID is AuthConfig.Keys: hub.key_by_id, no cache, one indexed read.
func (s *Store) KeyByID(ctx context.Context, keyID string) (KeyRow, bool, error)
// KeyUsed is AuthConfig.KeyUsed: hub.key_used, at most one UPDATE a minute per key.
func (s *Store) KeyUsed(ctx context.Context, keyID string) error

// CreateKey inserts the row with HashKey(secret) and ShownOf(secret); the secret
// itself is never a parameter of any statement. It counts live keys FOR UPDATE of
// the owner's (or team's) rows first, under the org's audit lock (Hub task 14).
func CreateKey(ctx context.Context, tx pgx.Tx, id, ownerID string, teamID *string, name string,
	hash []byte, shown string, expiresAt, at time.Time, maxLive int) (hubapi.APIKey, error)
var ErrKeyLimit = errors.New("store: live key limit")
func ListKeys(ctx context.Context, q Querier, f KeyFilter, after string, limit int) (hubapi.Page[hubapi.APIKey], error)
type KeyFilter struct{ OwnerID, TeamID string; PersonalOnly bool }
func LockKey(ctx context.Context, tx pgx.Tx, id string) (hubapi.APIKey, bool, error) // FOR UPDATE
func RevokeKey(ctx context.Context, tx pgx.Tx, id, by string, at time.Time) (hubapi.APIKey, error)
```

`CreateKey` takes the hash, not the secret, so the plaintext cannot reach a column by mistake; the
route mints, hashes, inserts, and puts the secret only into the response body.

#### Tests to write first, part 1 — pure (`server/work_keys_secret_test.go`; `ratelimit_test.go` ⚑)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMintedKeyParsesToItsID` | `MintKey("ak-3f2a91c07b1e4d2a")` is 73 characters, matches `keyShape`, and `ParseKey` returns the id; two mints differ | a key whose id cannot be read back; a fixed random part |
| `TestParseRejectsNearMisses` | 72 and 74 characters, upper-case base32, `pak-…`, `PAK_…`, 15 hex, a `=` pad, a trailing space, a JWT `eyJ…` → `ok=false` each | a parser that checks the prefix only |
| `TestHashCoversTheWholeSecret` | the same random part under two ids → two hashes | a hash of the random part alone, replayable under another id |
| `TestSameHashRefusesShortHashes` | `SameHash(h, nil)`, 31 bytes, and a different 32 bytes → false; equal → true | a compare that accepts an empty stored hash |
| `TestTightestAndHeaders` | `{user 60/59, key 20/3, org 400/390}` → `key`; a tie on Remaining → the smaller Limit; `Reset` 1.2 s → `RateLimit-Reset: 2`; full → `0` | the widest bucket reported; the reset rounded down |

#### Tests to write first, part 2 — `server/work_keys_integration_test.go` (`//go:build integration`)

Org `o1`: Ada (org admin), Linus (member of `tm-a`), Grace (admin of `tm-a`, member of `tm-b`);
org `o2`: Xavier (org admin). Person tokens from `devissuer` (Hub task 11); keys only through the
routes. `AuthConfig.Now` is a settable clock; `Validator` and `Keys` are wrapped by call counters.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheSecretIsShownOnce` | the `POST` body has `secret`; `GET` lists and `GET /v1/api-keys` never contain it, its random part or a hash; `row_to_json` of the stored row (as `hub_owner`) contains neither | plaintext key |
| `TestRevokedKeyFailsTheNextCall` | a key call 200; the owner revokes; the very next call → 401 `key_revoked`, `details.reason "revoked"` | revocation cached |
| `TestExpiredKeyIs401` | the clock past `expiresAt` → 401 `unauthenticated`, reason `key_expired`, never `expired` | no expiry; an expired key the control plane tries to refresh |
| `TestAKeyNeverReachesTheValidator` | `pak_<real id>_<wrong 52>` → 401 reason `key`; `pak_x` → 401 `malformed`; the `Validator` counter is 0 after both | prefix-only check; a failed key falling through to `oidcauth` |
| `TestUnknownIDAndWrongSecretMatch` | an unknown id and a wrong secret give byte-identical bodies, and neither says `key_revoked` for a revoked id | an oracle for which ids exist or are revoked |
| `TestAPersonTokenNeverReachesTheKeys` | Linus's desktop call 200 with the `Keys` counter at 0 | a token parsed as a key |
| `TestTeamKeySeesOneTeam` | Grace's `tm-a` key: a `tm-a` project 200; a `tm-b` project 404; `GET /v1/teams` lists `tm-a` only; `/v1/changes` has no `tm-b` event | team key is org-wide |
| `TestTeamKeyIsAMember` | under that key `TeamRole(tm-a)` is `member` though Grace is `admin`; with `key_team` dropped (a test-only `DROP POLICY`) `tm-b` is still 404 | a team key with its creator's admin role; one layer only |
| `TestKeysNeverMoveState` | a personal key: `todo` → `done` refused; `backlog` → `todo` refused (task 145's guard, surfaced as 403) | keys close issues |
| `TestKeyWriteLimitIsItsOwn` | `KeyWrite 5:20`, the clock frozen: 20 key comments succeed, the 21st → 429 `scope "key"`, `Retry-After ≥ 1`; Linus's desktop write then succeeds | no per-token limit; a key charged to its owner's bucket |
| `TestEveryResponseCarriesRateHeaders` | a 200, a 401 (bad key: the ip bucket) and that 429 (`RateLimit-Remaining: 0`) each carry the three headers as integers | headers missing |
| `TestOrgAdminRevokesAnyKeyInTheOrg` | Ada revokes Linus's personal key 200 and it fails next call; Grace (not the owner, not an org admin) → 404; Xavier → 404 | admin cannot revoke; a revoke across orgs |
| `TestAKeyCannotManageKeys` | a key `POST /v1/me/api-keys` and `…/revoke` → 403 `never` | a leaked key minting another |
| `TestKeyLivesOnItsOwnersSignIns` | `last_seen_at` set 31 days back → 401 `key_revoked` `owner_inactive`; 100 key calls leave `last_seen_at` unchanged; Grace removed from `tm-a` → her team key `creator_left_team` | a key outliving offboarding; key calls refreshing `last_seen_at` |
| `TestLiveKeyLimits` | an 11th live personal key → 409 `details.limit "10"`; after one revoke, 201 | unbounded keys multiplying the per-token rate |
| `TestKeyWritesCarryTheKeyID` | a key's comment: the audit row's actor is `kind "key"`, `userId` Linus, `keyId` the `ak-` id | a key acting invisibly (L-239) |
| `TestKeyRoutesMatrix` | the six routes' rows (task 07's file): `me` routes ✓ for every person; team routes A only; `GET /v1/api-keys` O only; every route 403 for the app, key and integration fixtures | a missing role row |

`store/work_keys_integration_test.go`: `TestRevokedIsFinal` (as `hub_app`, `UPDATE … SET revoked_at
= NULL` → `42501`; kills "un-revoke"); `TestKeyTeamPolicyNarrowsMembers` (with `hub.key_team` set,
`SELECT … FROM team_members` returns that team's rows only; the next transaction on the same
connection, not setting it, sees all; kills "a setting carried across pooled requests");
`TestHashIsNotUpdatable` (`UPDATE … SET secret_sha256` as `hub_app` → `42501`; kills "a swapped
hash").

#### Steps

- [ ] **Step 1: Check the base.** Hub tasks 13, 14 and 15 merged (`auth.go`, `ratelimit.go`,
  `tx.go`, `authz.go`); task 145's `work_api_keys`, `key_by_id`, `key_used`, the `key_team` policy
  and the guard's `key` branch; task 159 merged (`appAccess`, `appGate`); task 158's
  `cmd/hubopenapi`. If the proposed Hub edit (task 159's, with this part's lines) is not accepted,
  stop after step 2 and report: the routes may land, but no key can authenticate until it is.
- [ ] **Step 2: The proposal.** Add this part's `auth.go`, `tx.go`, `authz.go` and `ratelimit.go`
  lines, the three new 401 reasons, the two new 429 scopes and the five variables to task 159's
  proposal for the `feat/hub` owner, citing this part. Give task 145's owner the SQL above if its
  migration lacks any of it.
- [ ] **Step 3: Part 1's tests**, then `work_keys_secret.go` and the `ratelimit.go` lines, until
  green.
- [ ] **Step 4: Part 2's tests** (they fail), then the store, the six routes, `keyAuth` in step 3,
  `keyGate` beside `appGate`, the key prologue in `tx.go`, the `TeamRole` cap, until green.
- [ ] **Step 5: Regenerate the published API.** Add the six entries to `cmd/hubopenapi/routes.go`,
  run the generator and both SDK generators as task 158's step 4 does; `-check` exits 0.
- [ ] **Step 6: Run the neighbours**: Hub task 13's auth and limiter tests (the desktop path and
  `TestIPLimitRunsBeforeValidation` unchanged), task 159's app tests (the `app` bucket is now its
  own), task 145's guard rows, task 156's webhook matrix (keys 403). Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(MintedKeyParsesToItsID|ParseRejectsNearMisses|HashCoversTheWholeSecret|SameHashRefusesShortHashes|TightestAndHeaders)$' \
  2>&1 | grep -c '^--- PASS'                                                                  # 5
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(TheSecretIsShownOnce|RevokedKeyFailsTheNextCall|ExpiredKeyIs401|AKeyNeverReachesTheValidator|UnknownIDAndWrongSecretMatch|APersonTokenNeverReachesTheKeys|TeamKeySeesOneTeam|TeamKeyIsAMember|KeysNeverMoveState|KeyWriteLimitIsItsOwn|EveryResponseCarriesRateHeaders|OrgAdminRevokesAnyKeyInTheOrg|AKeyCannotManageKeys|KeyLivesOnItsOwnersSignIns|LiveKeyLimits|KeyWritesCarryTheKeyID|KeyRoutesMatrix)$' \
  2>&1 | grep -c '^--- PASS'                                                                  # 17
go test ./control-plane/internal/hub/store/ -tags integration -race -count=1 -v \
  -run '^Test(RevokedIsFinal|KeyTeamPolicyNarrowsMembers|HashIsNotUpdatable)$' 2>&1 | grep -c '^--- PASS'   # 3
go run ./control-plane/cmd/hubopenapi -check; echo $?                                          # 0
git grep -cE 'pak_[0-9a-f]{16}_[a-z2-7]{52}'; echo $?                                          # 1: no key-shaped literal anywhere
go test ./control-plane/internal/hub/... -tags integration -race -count=1                     # every package ok
```

Expected as commented: `5`, `17`, `3`, exit `0`, no match (exit `1`), every package `ok`.

#### What this task does NOT claim

- **Not that the `feat/hub` owner accepts a non-person credential on `/v1`.** `hub-contract:36`
  requires a bearer for `Hub.Access` on every route today (§17 made it a realm token); the proposed
  edit changes that. Until it is accepted, no key authenticates.
- **Not that limits are exact across replicas.** §11.3 is per replica, at most double
  (`hub-contract:2094-2096`); the `key` and `app` buckets inherit that.
- **Not that an offboarded person's key stops at once.** It stops at its revoke, its expiry, or
  30 days after the owner's last own sign-in, whichever comes first. The Hub learns nothing from
  Entra about a disabled account; an org's offboarding step should revoke keys (task 173's list).
- **Not that keys reach the Work MCP server.** Task 161 refuses a `pak_` bearer.
- **Not secret scanning.** The `pak_` shape is easy to scan for, but registering it with any host's
  secret-scanning programme is not done here.
- **Not the desktop's keys screen** (task 173), nor any key for a service or an agent: a key always
  has a person behind it.

**Parity rows:** closes **L-245** and **L-252**.

#### Gaps found while writing this part

1. The scope's routes (`/v1/me/keys`, `/v1/keys/{keyId}/revoke`) differ from task 143's
   (`/api-keys`, `task-143.md:949`); this part follows task 143, and the scope text is stale.
2. `GET /v1/api-keys` (the org admin's list) is in neither task 143's route table nor its client
   interface (`:866-868`), and `key_not_found` is not in its code list.
3. Task 145 must carry the `work_api_keys` columns, the `revoke_final` trigger, `hub.key_by_id`,
   `hub.key_used`, the settings `hub.work_actor_key` and `hub.key_team`, and a restrictive policy on
   the **Hub's** `team_members` table, which is itself a Hub-contract change; its scope names only
   the table.
4. Task 159's `TestAppCannotCloseOrFakeAFact` has an app move `todo` → `canceled` by hand, but task
   145's scope refuses `app` (and `key`) every state column write; one of them must change. This
   part follows task 145 for keys.
5. The skeleton lists task 160's dependency as 159 alone, but step 5 edits task 158's
   `cmd/hubopenapi/routes.go`; 158 should be a dependency, or 158's drift test must tolerate the
   six routes arriving later.
6. hub-contract §12.2 (`:2175`) must gain `PANACEA_APP_HUB_RATE_KEY`, `_RATE_KEY_WRITE`,
   `_RATE_APP`, `_RATE_APP_WRITE` and `PANACEA_APP_HUB_KEY_OWNER_IDLE`, and `:269`'s scope list
   `key` and `app`; both are in step 2's proposal, and neither is named by the scope.

#### Addendum (fix batch b02, round 2; review N-I4): a key cannot set a check

Appended so no line above moves. A key reaches the create and edit routes through task 159's
`appAccess` (decision 4), and its prologue sets `hub.work_actor_kind = 'key'` before the handler
(`:296-300`). Tasks 08 and 09 read `byPerson` from that setting (`store.WriterIsPerson`, task 08's
addendum), so a key's revision cannot add or change a serves pair's `check` (SW.3's rule, task
07's addendum). One row joins part 2's table, and the acceptance's second count goes from `17` to
`18`, its `-run` list gaining `AKeyCannotSetACheck`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAKeyCannotSetACheck` | a personal key (it reaches `write` routes, decision 4): create an issue serving `c1` with `check: "true"` → 422 naming `serves[0].check`; a revision adding one → 422, head unchanged; the key's revision carrying an existing check unchanged → 201 | a leaked key adding a command every lander's control plane runs and attests to |
