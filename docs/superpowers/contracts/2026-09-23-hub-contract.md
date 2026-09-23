# The Hub — contract (Phase H)

**Status:** the contract phases 3, 4, 5 and 7 build against, from 2026-09-23. It implements
`docs/superpowers/adr/ADR-H1-hub-architecture.md`; where the two disagree, **the ADR's decision wins
and this file is wrong** — raise it. Owner-reviewable under `OD-1`; nothing here is built.
**Branch:** `feat/hub`. **Nothing in this file was compiled or run**, except the two digest vectors
in §3.5, which were computed twice by independent means.

**Words.** *Must* is a requirement a test checks. *The Hub* is the server `control-plane/cmd/hub`.
*The control plane* is the existing local process `control-plane/cmd/api`. *The principal* is the
signed-in user a request's access token names.

---

## 0. What each phase needs from this file

| Phase | Read | Build against |
|---|---|---|
| **4 Discover** | §1, §3, §4, §7.4–§7.5, §8, §9.2 | `hubclient.Briefs` and its fake; routes `/v1/projects/…/briefs`, `/brief-revisions`, `/v1/refs` |
| **5 Design** | §1, §3, §4, §7.6–§7.8, §9.2 | `hubclient.Designs`; blobs, design approvals, session links, `design:` keys in `/v1/refs` |
| **7 Deploy** | §1, §3, §4, §7.9, §9.2 | `hubclient.Releases`; reserve → confirm or abandon, rollback, the deployment join |
| **3 Git** | §1, §4, §7.10, §9.1 | `hubclient.Evidence`; `credstore` in the control plane |
| **8 Environment** | §6, §10.1, §12 | the chart values in §12.3, the registrations in §6.5 |
| **Desktop** | §4.3, §9.3–§9.5 | control-plane `/api/v1/auth/*`, the local caller key, the change cursor header |

---

## 1. Conventions — every route

- **Base URL.** `PANACEA_APP_HUB_URL` in the control plane (§12.2); `http://127.0.0.1:8750` locally,
  `https://hub.<env-domain>` deployed. Every path below is relative to it. No trailing slash.
- **Version.** Every API path starts `/v1`. Additive changes (a new optional request field, a new
  response field, a new route, a new error code, a new event kind) are not breaking. Consumers
  **must** ignore unknown response fields and unknown event kinds. The Hub **rejects** unknown request
  fields (`httpx.DecodeJSON`, 400 `bad_request`).
- **Auth.** Every `/v1` route requires `Authorization: Bearer <Entra access token for Hub.Access>`
  (§6.3). `/healthz` and `/readyz` require nothing.
- **No browsers.** A request carrying an `Origin` header is refused, 403 `forbidden_origin`, before
  authentication. The Hub sends no `Access-Control-*` header, ever (§11.4).
- **Bodies.** `Content-Type: application/json; charset=utf-8`, one JSON object, at most **1 MiB**
  (`httpx.MaxBodyBytes`). The one exception is the blob upload (§7.6), which is raw bytes.
- **Names.** JSON fields are `camelCase`. Enum values are `snake_case` strings.
- **Times.** Every time the Hub writes is RFC 3339 UTC with exactly three fractional digits:
  `2026-09-23T04:02:11.123Z`. Times the Hub reads are RFC 3339 with any offset and are normalised to
  that form before storage.
- **Ids.** Hub-minted, `<prefix>-<16 lowercase hex>` (§3.1). They are opaque to callers: compare
  them, never parse them.
- **Lists.** `?limit=` (default 50, maximum 200) and `?after=<cursor>`. The response is
  `{"items":[…],"next":"<cursor>"|null}`. A cursor is opaque. Order is stated per route.
- **Idempotency.** Any `POST` may carry `Idempotency-Key: <1–64 of [A-Za-z0-9_-]>`. The Hub stores
  the response under *(principal, key)* for 24 hours in the same transaction as the write. A repeat
  with the same method, path and body returns the stored status and body with
  `Idempotent-Replay: true`. A repeat with a different body is 422 `idempotency_mismatch`. **The
  control plane must send one** on every reserve, confirm, approval and revision write (§9.2), so a
  retry after a lost response cannot double-write.
- **Errors.** Every non-2xx JSON response is an `httpx.Problem` (§4).
- **Response headers on every response:** `Content-Type` as above, `X-Content-Type-Options:
  nosniff`, `Cache-Control: no-store`, `Request-Id: <uuid>` (also logged; quote it in a bug).

---

## 2. Where the code lives

| Path | Package | Owner task | Imports allowed |
|---|---|---|---|
| `control-plane/cmd/hub/` | `main` — subcommands `serve`, `migrate`, `verify` | H0 | `internal/hub/...`, `internal/httpx`, `internal/ids`, `internal/entra`, stdlib |
| `control-plane/cmd/hub/compose.yaml` | local stack | H0 | — |
| `control-plane/cmd/devissuer/` | `main` — the local OIDC issuer (§6.4) | H1 | stdlib only |
| `control-plane/internal/entra/` | `entra` — token validation (§6.3) | phase 8 task 16, or H1 to its interface | stdlib only |
| `control-plane/internal/hub/hubapi/` | wire types, error codes, key grammar, canonical encoding (§3, §4, §7) | H0 | stdlib only |
| `control-plane/internal/hub/server/` | routes, authorisation, handlers | H2–H7 | `hubapi`, `store`, `httpx`, `ids`, `entra`, `x/time/rate` |
| `control-plane/internal/hub/store/` | Postgres access, one file per aggregate | H2–H7 | `hubapi`, `pgx/v5` |
| `control-plane/internal/hub/migrate/` | the runner and `sql/*.sql`, embedded (§10) | H0 | `pgx/v5`, stdlib |
| `control-plane/internal/hub/hubclient/` | the control plane's client and its in-memory fake (§9.2) | H0 (interface, fake), C2 (HTTP) | `hubapi`, stdlib |
| `control-plane/internal/entraauth/` | PKCE sign-in and `TokenSource` (§9.1) | C1 | stdlib, `x/sys/windows` via `credstore` |
| `control-plane/internal/credstore/` | Windows Credential Manager (§9.1); phase 3's `Store` interface | C1 (phase 3 task 13 owns the interface) | stdlib, `x/sys/windows` |

**The boundary test** (H0, `control-plane/internal/hub/boundary_test.go`) runs
`go list -deps -f '{{.ImportPath}}' ./cmd/hub` and fails on any module-local import outside the
"imports allowed" column, and runs `go list -deps ./cmd/api` and fails if it reaches
`internal/hub/server` or `internal/hub/store`. The control plane may import only `hubapi` and
`hubclient`.

**New module requirements** (`control-plane/go.mod`): `github.com/jackc/pgx/v5` (MIT);
`golang.org/x/time` and `golang.org/x/sys` move from indirect to direct. Nothing else.

---

## 3. Identifiers, keys, and the canonical encoding

### 3.1 Id prefixes

One new function in `control-plane/internal/ids/ids.go`, appended at the end of the file:

```go
// Wide returns prefix + "-" + 16 random lowercase hex characters, e.g.
// "bf-3f2a91c07b1e4d2a". It is for ids shared across every user of the Hub,
// where New's 32 bits would collide: the Hub also retries once on a unique
// violation, so a collision costs a retry, never a wrong row.
func Wide(prefix string) string
```

| Prefix | Entity | | Prefix | Entity |
|---|---|---|---|---|
| `org-` | org | | `bf-` | brief |
| `u-` | user | | `ap-` | design approval |
| `tm-` | team | | `ln-` | session → design link |
| `iv-` | invite | | `rl-` | release |
| `pj-` | project | | `ev-` | evidence record |

Disjoint from the control plane's `s-`, `p-`, `b-`, `w-`, `m-`. Phase 3's local evidence store mints
`ev-` + 12 hex (`task-15.md:48`); Hub evidence ids are `ev-` + 16 hex and never collide in length.

### 3.2 Key grammar — normative for the Hub

The key grammar belongs to phase 4's `provenance` package (provenance contract §1.2, P5). These are
the forms **the Hub mints and accepts**; the `provenance` parser must accept at least these.

```
brief key    ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$            e.g. brief:bf-3f2a91c07b1e4d2a@5
design key   ^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$   e.g. design:cv-7c01d2e3f4a5b6c7@11
```

- A brief revision number is the Hub's, assigned by the revision write (§7.5), from 1, gapless per
  brief. Revision 0 means "the brief exists and has no revision yet" and never appears in a key.
- The canvas id in a design key is **minted by phase 5 in the control plane** (v1 keeps canvases
  there). The Hub stores it verbatim, scoped to the project. Phase 5 **should** mint it with
  `ids.Wide("cv")`, so a canvas id stays unique when v2 moves canvases into the Hub.
- `design_revision` is phase 5's canvas revision. The Hub never interprets it beyond the grammar.

### 3.3 Branches

A branch is named by its full git ref, `refs/heads/<name>`, at most 255 bytes, validated by the
rules of `git check-ref-format` that can be checked without git: no `..`, no ASCII control
characters, no space, `~`, `^`, `:`, `?`, `*`, `[` or `\`, not ending in `/` or `.lock`, no `@{`.
The empty string means *project-scoped*. Local `BranchID`s (`b-…`, path-hashed) never reach the
Hub.

### 3.4 The canonical encoding and the digest

A brief revision's digest is `"sha256:" + hex(SHA-256(C))`, where **C** is the canonical encoding
of this envelope:

```json
{
  "v": 1,
  "kind": "brief-revision",
  "briefId": "bf-…",
  "revision": 5,
  "projectId": "pj-…",
  "scope": "project" | "branch",
  "branchRef": "refs/heads/…" | "",
  "content": { … the object the writer sent, §7.5 … }
}
```

The canonical encoding is **RFC 8785 (JCS) restricted to what the Hub accepts**:

1. Objects: members sorted by key, comparing keys as sequences of UTF-16 code units; no duplicate
   keys (a duplicate is 400 `bad_request`); no whitespace anywhere.
2. Strings: UTF-8. Escape `"` as `\"`, `\` as `\\`, U+0008 `\b`, U+0009 `\t`, U+000A `\n`, U+000C
   `\f`, U+000D `\r`, every other code point below U+0020 as `\u00xx` with **lowercase** hex. Every
   other code point, including `/`, U+007F, U+2028 and non-ASCII, is written as its UTF-8 bytes.
   No Unicode normalisation is applied.
3. Numbers: **integers only**, in [−(2⁵³−1), 2⁵³−1], written in decimal with no leading zeros, no
   plus sign and no exponent; `-0` is written `0`. A fraction or exponent in `content` is 400
   `bad_request` — so the float rules of JCS never arise.
4. `true`, `false`, `null` as themselves.

`C` is **stored**, byte for byte (§10, `brief_revisions.canonical`). The Hub serves `content` by
parsing `C`, never from a second copy. Implementation: one function,
`hubapi.Canonical(v any) ([]byte, error)`, used by the Hub to write and by `hubclient` to verify
(§9.2). **No TypeScript implementation exists or may be written** (P5; the desktop never computes a
digest).

### 3.5 Golden vectors — checked into `hubapi/canonical_test.go`

Computed on 2026-09-23 with a Python `json.dumps(sort_keys=True, separators=(",",":"),
ensure_ascii=False)` encoder, and vector 1 again with `printf '%s' … | sha256sum`; both agree.

**Vector 1** — 209 bytes:

```
{"branchRef":"refs/heads/evidence-upload","briefId":"bf-00000000000000a1","content":{"answers":[],"status":"open"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":1,"scope":"branch","v":1}
sha256:a72a06a155457ac321227e660a220ab030f0367a5184f3f8ac8e8f636cd5e9fa
```

**Vector 2** — 256 bytes. Input `content` is `{"status":"complete","note":"Café<TAB>line<U+0001>end",
"n":-0,"big":9007199254740991,"ok":true,"none":null}`, where `é` is U+00E9 (bytes `C3 A9`). The
canonical text, with the two escapes shown as they appear in the bytes:

```
{"branchRef":"","briefId":"bf-00000000000000a1","content":{"big":9007199254740991,"n":0,"none":null,"note":"Café\tline\u0001end","ok":true,"status":"complete"},"kind":"brief-revision","projectId":"pj-00000000000000b2","revision":2,"scope":"project","v":1}
sha256:fa6e8ac2a66bf14e2b0ad2ffb02afac6b6143d042e52f5bdc5a051a0c43d8537
```

A Go implementation passes when it reproduces both byte strings and both digests exactly.

### 3.6 The other canonical records

The same encoding, with the same function, gives:

| Record | Envelope `kind` | Fields in the envelope | Stored as |
|---|---|---|---|
| brief revision | `brief-revision` | §3.4 | `brief_revisions.canonical` |
| audit event | `audit-event` | `v`, `kind`, `orgId`, `seq`, `at`, `actorUserId` (or `""`), `event` (the event kind, §8), `projectId` (or `""`), `subject`, `data` | `audit_events.canonical`; `hash = SHA-256(prev_hash ‖ canonical)`, where `prev_hash` is 32 zero bytes for `seq` 1 |

---

## 4. Errors

### 4.1 The body

Every non-2xx JSON response is exactly the control plane's `httpx.Problem`
(`control-plane/internal/httpx/problem.go`), written with `httpx.WriteProblem`:

```json
{ "error": "revision_conflict", "message": "Someone saved revision 6 of this brief first.", "details": { "briefId": "bf-3f2a91c07b1e4d2a", "headRevision": "6" } }
```

`details` values are **strings**, as `Problem.Details` is `map[string]string`. `message` is one human
sentence, safe to render; it never holds a token, a Go error chain or another tenant's data.
A 401 also carries `WWW-Authenticate: Bearer error="invalid_token"` (or `insufficient_scope` for a
missing scope). A 429 also carries `Retry-After: <seconds>`.

### 4.2 The codes

Constants in `control-plane/internal/hub/hubapi/codes.go`. Codes marked **shared** are spelled,
and mean, exactly what `httpx` means; the Hub imports them from `httpx`, it does not redeclare
them.

| Code | Status | Meaning | `details` |
|---|---|---|---|
| `invalid_json` **shared** | 400 | not one JSON object, or over 1 MiB | — |
| `bad_request` **shared** | 400 | a field is missing, unknown, the wrong type, or out of range; a malformed id, key or ref | `field` |
| `unauthenticated` | 401 | no token, or a token that fails §6.3 | `reason`: `missing` · `malformed` · `signature` · `expired` · `not_yet_valid` · `issuer` · `audience` · `scope` · `client` · `version` |
| `tenant_not_enrolled` | 403 | a valid token from an Entra tenant with no org | `tenantId` |
| `forbidden` | 403 | a member whose role is too low, or not an org admin where one is required | `required`: `member` · `admin` · `org_admin` |
| `forbidden_origin` **shared** | 403 | the request carried an `Origin` header | — |
| `not_reserver` | 403 | confirm or abandon by someone other than the user who reserved | `releaseId` |
| `team_not_found` | 404 | no such team **that the caller belongs to** | `teamId` |
| `project_not_found` | 404 | no such project **that the caller can read**; also every route under it | `projectId` |
| `brief_not_found` | 404 | as above, for a brief | `briefId` |
| `member_not_found` | 404 | the user is not a member of that team | `userId` |
| `invite_not_found` | 404 | | `inviteId` |
| `approval_not_found` | 404 | | `approvalId` |
| `release_not_found` | 404 | | `releaseId` |
| `blob_not_found` | 404 | not uploaded to this project | `digest` |
| `route_not_found` | 404 | no such path | — |
| `method_not_allowed` | 405 | known path, wrong method; `Allow` header set | — |
| `conflict` **shared** | 409 | a generic wrong-state conflict not covered below | — |
| `revision_conflict` | 409 | `baseRevision` is not the brief's head | `briefId`, `headRevision` |
| `brief_exists` | 409 | a live brief already has that scope and branch | `briefId` |
| `brief_archived` | 409 | a write to an archived brief | `briefId` |
| `design_already_approved` | 409 | that design key already has an approval | `designKey`, `approvalId` |
| `repo_already_linked` | 409 | the repository is a remote of another project in this org | `repoKey` (the other project's id only if the caller can read it: `projectId`) |
| `last_admin` | 409 | the change would leave a team with no admin | `teamId` |
| `release_state` | 409 | confirm or abandon on a release that is not `reserved` | `releaseId`, `state` |
| `release_expired` | 409 | confirm after `expiresAt` | `releaseId` |
| `quota_exceeded` | 409 | the project's blob total would pass 1 GiB | `limitBytes` |
| `blob_too_large` | 413 | a blob over 16 MiB | `limitBytes` |
| `brief_incomplete` | 422 | an approval against a revision whose `status` is not `complete` (ADR Q1) | `key` |
| `key_unresolvable` | 422 | a key in a write names nothing in this project | `key` |
| `digest_mismatch` | 422 | a digest in a write is not the named record's digest; or an uploaded blob's bytes do not hash to its path | `key` or `digest` |
| `blob_missing` | 422 | an approval names a blob not uploaded to this project | `digest` |
| `design_not_approved` | 422 | a release names a design key with no approval | `designKey` |
| `repo_not_linked` | 422 | an evidence record's `repo` is no project's remote the caller can read | `repo` |
| `idempotency_mismatch` | 422 | an `Idempotency-Key` reused with a different request | — |
| `rate_limited` | 429 | §11.3 | `scope`: `user` · `user_write` · `org` · `blob_bytes` |
| `internal` **shared** | 500 | a bug; the detail is in the log under `Request-Id` | — |
| `unavailable` | 503 | Postgres unreachable or the schema not at this build's version | — |

**Why 404 and not 403 for things the caller cannot see** (`project_not_found` and the rest): a 403
would confirm that the id exists, and ids are then probeable. A 403 is returned only where the
caller can already see the object and lacks the role to change it.

### 4.3 What the control plane does with them — the relay rule

The control plane's Hub-backed handlers (phase 4's, 5's, 7's, 3's) apply one rule, implemented once
in `hubclient` (§9.2) as `hubclient.Error`:

| Hub answered | The control plane answers the desktop |
|---|---|
| any 4xx `Problem` except 401 | **the same status, code, message and details, unchanged** |
| 401 `unauthenticated` with `reason` `expired` | refreshes once and retries; if it fails again, 401 as below |
| any other 401 | 401 **`signed_out`** (a new control-plane code, C1), `details.reason` = the Hub's reason |
| 5xx, a timeout (10 s), a refused connection, a TLS failure | 503 **`upstream_unavailable`**, `details.upstream` = `"hub"` (ruling R4) |

`signed_out` and `upstream_unavailable` are control-plane codes and go in `httpx`'s constant block
by the tasks that add them (C1; whichever of phases 5 and 7 lands R4 first). Every other Hub code
the desktop may see is listed above; `desktop/src/api/problems.ts` maps them all to the
`server-said` kind except `signed_out`, which C4 maps to the sign-in surface.

---

## 5. Principals and authorisation

### 5.1 From token to principal — every `/v1` request, in this order

1. Refuse any request with an `Origin` header: 403 `forbidden_origin`.
2. Apply the per-IP pre-auth limit (§11.3).
3. Validate the bearer token with `entra.Validator` (§6.3). Failure: 401 `unauthenticated` with the
   reason, or 403 `tenant_not_enrolled`.
4. Apply the per-user and per-org limits (§11.3).
5. Begin one transaction. Call `hub.resolve_principal(tid, oid, name, email)` (§10.4), a
   `SECURITY DEFINER` function that finds the org by `entra_tenant_id`, inserts the user on first
   sight (`auth.user_provisioned` audit event), refreshes `display_name` and `email` when they
   changed, and returns `(org_id, user_id)`.
6. `SELECT set_config('hub.org_id', <org_id>, true)` — the row-level-security key (§10.3) for the
   rest of the transaction.
7. Run the handler. Authorisation is checked in Go against §5.3 **before** any read of project
   content. Commit.

**Org enrolment is configuration in v1.** `hub serve` upserts one org per entry of
`PANACEA_APP_HUB_TENANTS` at start (§12.2). There is no enrolment route until the BYO tier.

**`email`** is the token's `email` claim if present, else `preferred_username`, lowercased. It is
used for one thing: binding invites (§7.3). It is never a key.

**Org admin** is true when the token's `roles` claim contains `Hub.OrgAdmin` — an Entra app role IT
assigns (§6.5). It is read from each token, never stored, so removing the assignment in Entra takes
effect at the next token.

### 5.2 Roles

| Role | Held in | Grants |
|---|---|---|
| `viewer` | `team_members.role` | read the team, its projects and everything under them |
| `member` | `team_members.role` | viewer, plus every write in §5.3 marked **M** |
| `admin` | `team_members.role` | member, plus every write marked **A** |
| org admin | token `roles` ∋ `Hub.OrgAdmin` | the audit log and erasure for its org. **No read of project content** it is not a member of |

A project's access is exactly its team's membership. There are no per-project grants in v1.

### 5.3 The matrix — the authorisation test (H2) runs every row as every column

`V` viewer, `M` member, `A` admin, `O` org admin who is not a member, `N` an authenticated
non-member in the same org, `X` a principal from another enrolled org. **✓** allowed; **403** is
`forbidden`; **404** is the route's `*_not_found`.

| Operation | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/me`, `GET /v1/teams`, `POST /v1/teams` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| read a team, its members, its invites | ✓ | ✓ | ✓ | 404 | 404 | 404 |
| rename a team; set or remove a member; create or revoke an invite | 403 | 403 | ✓ | 404 | 404 | 404 |
| leave a team (remove yourself) | ✓ | ✓ | ✓ unless `last_admin` | 404 | 404 | 404 |
| `GET /v1/projects?repo=` | own teams' projects only | ← | ← | ← | empty list | empty list |
| create a project in a team | 403 | ✓ | ✓ | 404 | 404 | 404 |
| read a project and anything under it | ✓ | ✓ | ✓ | 404 | 404 | 404 |
| rename a project; add or remove a remote | 403 | 403 | ✓ | 404 | 404 | 404 |
| create a brief; write revisions; rebind; archive | 403 | ✓ | ✓ | 404 | 404 | 404 |
| upload a blob; approve a design; write a session link | 403 | ✓ | ✓ | 404 | 404 | 404 |
| reserve a release; add a deployment join | 403 | ✓ | ✓ | 404 | 404 | 404 |
| confirm or abandon a release | 403 | reserver only, else `not_reserver` | ← | 404 | 404 | 404 |
| record evidence | 403 | ✓ | ✓ | `repo_not_linked` | `repo_not_linked` | `repo_not_linked` |
| `GET /v1/refs` | resolves keys in readable projects; every other key is `missing` | ← | ← | ← | ← | ← |
| `GET /v1/changes` | events of readable projects and own teams only | ← | ← | ← | ← | ← |
| `GET /v1/audit` | 403 | 403 | 403 | ✓ own org | 403 | 403 |
| erase a brief | 403 | 403 | 403 | ✓ own org | 403 | 403 |

`X` can never reach another org's rows even if the Go check were missing: row-level security
(§10.3) returns none. The test asserts both layers separately by running the `X` column once with
the Go check disabled through a test-only hook.

---

## 6. Identity — the flow, step by step

### 6.1 Sign-in (task C1; control plane)

| Step | Who | What, exactly |
|---|---|---|
| 1 | renderer → control plane | `POST /api/v1/auth/sign-in` with the local caller key (§9.4). No body |
| 2 | control plane | cancels any pending sign-in. Makes `verifier` = base64url(32 random bytes), 43 characters; `challenge` = base64url(SHA-256(verifier)); `state` and `nonce` = base64url(32 random bytes) each. Opens a listener on `127.0.0.1:0` |
| 3 | control plane | fetches `<authority>/v2.0/.well-known/openid-configuration` (cached 24 h), where `<authority>` is `PANACEA_APP_ENTRA_AUTHORITY` — `https://login.microsoftonline.com/<tenant-id>`, or `http://127.0.0.1:8740/<tenant-id>` for `devissuer` |
| 4 | control plane → renderer | `200 {"authorizeUrl":"…","expiresAt":"…"}`. The URL is the discovered `authorization_endpoint` with `client_id=<desktop client id>`, `response_type=code`, `response_mode=query`, `redirect_uri=http://127.0.0.1:<port>/`, `scope=openid profile offline_access email api://<hub-client-id>/Hub.Access`, `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`, `prompt=select_account`. `expiresAt` is now + 10 min |
| 5 | renderer | `openUrl(authorizeUrl)` from `@tauri-apps/plugin-opener` — the system browser. The renderer never sees the redirect |
| 6 | browser ↔ Entra | the user signs in, under KPMG's conditional access |
| 7 | browser → listener | `GET /?code=…&state=…` (or `?error=…&error_description=…&state=…`). The listener serves **only** `GET /`. A request whose `state` does not match, compared in constant time, is answered 400 and **ignored** — it does not end the sign-in, so a racing local page cannot abort it. A matching one is answered with a static page (`Content-Security-Policy: default-src 'none'`, no script): *"You're signed in to PanaceaApp. You can close this tab."* The listener then closes |
| 8 | control plane → Entra | `POST token_endpoint`, form-encoded: `grant_type=authorization_code`, `client_id`, `code`, `redirect_uri` (identical to step 4), `code_verifier`, `scope` (identical to step 4). **No client secret; no `Origin` header** |
| 9 | control plane | validates the `id_token`: signature against the discovered `jwks_uri`, `iss`, `aud` = the desktop client id, `nonce` = step 2's, `exp`. Takes `tid`, `oid`, `name`, `preferred_username` from it |
| 10 | control plane | writes the refresh token and the account record to Credential Manager (§9.1); keeps the access token in memory with its `exp` |
| 11 | control plane → Hub | `GET /v1/me` with the access token — provisions the user and binds invites (§7.2) |
| 12 | renderer | its next `GET /api/v1/auth/status` reads `signed_in` |

**Failures.** Entra's `error` at step 7 → `signed_out`, `reason: "entra_error"`, with
`error_description` logged, not shown. No callback in 10 minutes → `reason: "timeout"`, listener
closed. The token exchange refused → `reason: "exchange_failed"`. `/v1/me` answering 403
`tenant_not_enrolled` → `reason: "tenant_not_enrolled"`; the tokens are discarded, not stored.

**Why `127.0.0.1` and not `localhost` in the redirect**: the listener binds IPv4 loopback only, and
`localhost` may resolve to `::1` first. `PANACEA_APP_ENTRA_REDIRECT_HOST=localhost` switches it if
KPMG's registration can only carry `http://localhost` (§6.5).

### 6.2 Tokens after sign-in

- **Serving a token.** `TokenSource.Token(ctx, scope)` returns the cached access token for `scope`
  while `exp − now > 5 min`. Otherwise it redeems the refresh token: `POST token_endpoint` with
  `grant_type=refresh_token`, `client_id`, `refresh_token`, `scope=<scope> offline_access`.
  Concurrent callers for one scope share one redemption (single-flight).
- **Rotation.** When the response carries a new `refresh_token`, it is written to Credential Manager
  **before** the access token is returned. A crash in between leaves the old one, which Entra still
  honours until it expires.
- **Scopes in v1.** `api://<hub-client-id>/Hub.Access`; phase 8's
  `api://<broker-client-id>/Sandboxes.Access`; if ADR Q5 is yes,
  `499b84ac-1321-427f-aa17-267ca6975798/user_impersonation` (Azure DevOps). One refresh token
  redeems all three because the desktop client is admin-consented for each (§6.5).
- **Failure.** `invalid_grant` or `interaction_required` → both Credential Manager entries deleted,
  `signed_out` with `reason: "reauth_required"`, and `ErrSignedOut` to every caller. Entra
  unreachable → an error the control plane maps to 503 `upstream_unavailable`,
  `details.upstream = "entra"`; the stored token is kept.
- **Start-up.** If Credential Manager holds an account record, status is `signed_in` immediately;
  the first `Token` call proves it. Nothing is redeemed eagerly.
- **Sign-out.** `POST /api/v1/auth/sign-out` deletes both entries and the in-memory tokens. It does
  not call Entra's logout endpoint in v1: the browser session is the user's, not ours.

### 6.3 Token validation — `control-plane/internal/entra` (HA-1)

Phase 8's task 16 owns this package; its part file did not exist on 2026-09-23. This is the
interface the Hub needs. **If task 16 lands first with a different shape, the Hub adapts to it in
one file (`internal/hub/server/auth.go`); the rules in the table must hold either way**, because the
broker and the Hub must accept and reject the same tokens.

```go
// Package entra validates Microsoft Entra ID v2 access tokens for a resource API.
// Standard library only: RS256 over crypto/rsa, JWKS over net/http.
package entra

type Config struct {
	// AuthorityHost is "https://login.microsoftonline.com" in every deployed
	// environment, or devissuer's base URL locally. The tenant id is appended.
	AuthorityHost string
	// TenantIDs are the enrolled tenants. A token from any other is ErrTenant.
	TenantIDs []string
	// Audience is the resource API's client id, a GUID: a v2 token's aud.
	Audience string
	// RequiredScope must appear in scp, e.g. "Hub.Access" or "Sandboxes.Access".
	RequiredScope string
	// AllowedClients is the azp allowlist: the desktop's client id.
	AllowedClients []string
	// ClockSkew defaults to 120 s.
	ClockSkew time.Duration
	// AllowInsecureAuthority permits an http AuthorityHost, and only a loopback one.
	AllowInsecureAuthority bool
	HTTPClient             *http.Client // default: 10 s timeout
	Now                    func() time.Time
}

// Claims is what a validated token says. It holds no token text.
type Claims struct {
	TenantID          string   // tid
	ObjectID          string   // oid
	ClientID          string   // azp
	Name              string   // name
	PreferredUsername string   // preferred_username
	Email             string   // email, when the optional claim is configured
	Scopes            []string // scp, split on spaces
	Roles             []string // roles
	ExpiresAt         time.Time
}

type Validator interface {
	Validate(ctx context.Context, bearer string) (Claims, error)
}

func New(cfg Config) (*JWKSValidator, error) // errors on an unusable Config; fetches nothing

// Every rejection wraps exactly one of these. The Hub maps each to details.reason.
var (
	ErrMalformed   = errors.New("entra: malformed token")     // "malformed"
	ErrSignature   = errors.New("entra: bad signature")        // "signature"
	ErrExpired     = errors.New("entra: token expired")        // "expired"
	ErrNotYetValid = errors.New("entra: token not yet valid")  // "not_yet_valid"
	ErrIssuer      = errors.New("entra: wrong issuer")         // "issuer"
	ErrAudience    = errors.New("entra: wrong audience")       // "audience"
	ErrScope       = errors.New("entra: required scope absent") // "scope"
	ErrClient      = errors.New("entra: client not allowed")   // "client"
	ErrVersion     = errors.New("entra: not a v2 token")       // "version"
	ErrTenant      = errors.New("entra: tenant not enrolled")  // 403 tenant_not_enrolled
)
```

| Check, in order | Rule | Failure |
|---|---|---|
| shape | three base64url segments; header JSON has `alg` = `RS256` and a `kid`; anything else, including `none` and `HS*`, fails before claims are read | `ErrMalformed` |
| tenant | `tid` present and in `TenantIDs` | `ErrTenant` |
| signature | RS256 over `header.payload` with the JWKS key whose `kid` matches, from `<AuthorityHost>/<tid>/discovery/v2.0/keys`. JWKS cached 24 h per tenant; an unknown `kid` triggers one refetch, at most once per 5 min per tenant | `ErrSignature` |
| `ver` | `"2.0"` | `ErrVersion` |
| `iss` | exactly `<AuthorityHost>/<tid>/v2.0` | `ErrIssuer` |
| `aud` | exactly `Audience` | `ErrAudience` |
| `exp`, `nbf` | `now < exp + skew`; `nbf − skew ≤ now` | `ErrExpired`, `ErrNotYetValid` |
| `azp` | in `AllowedClients` | `ErrClient` |
| `scp` | contains `RequiredScope`. A token with `roles` but no `scp` (app-only) fails here | `ErrScope` |
| `oid` | present, a GUID | `ErrMalformed` |

### 6.4 `devissuer` — the local OIDC issuer (task H1)

`control-plane/cmd/devissuer`, standard library only, **binds loopback only** and refuses any other
address. It mirrors Entra's path layout, so the validator's issuer rule is the same string with a
different host.

| Path | Serves |
|---|---|
| `GET /<tid>/v2.0/.well-known/openid-configuration` | `issuer` `http://127.0.0.1:8740/<tid>/v2.0`, and the three endpoints below, `code_challenge_methods_supported: ["S256"]` |
| `GET /<tid>/oauth2/v2.0/authorize` | an HTML page listing the tenant's configured users; choosing one redirects to `redirect_uri?code=…&state=…`. With `login_hint=<username>` and `PANACEA_APP_DEVISSUER_AUTO=true`, it redirects immediately — for end-to-end tests |
| `POST /<tid>/oauth2/v2.0/token` | `authorization_code` (PKCE **S256 required**; a missing or wrong verifier is `invalid_grant`) and `refresh_token` (rotating, single use) |
| `GET /<tid>/discovery/v2.0/keys` | the JWKS: one RSA-2048 key, generated at start, `kid` = first 16 hex of SHA-256 of its modulus |

- **Configuration** is one JSON file, `PANACEA_APP_DEVISSUER_CONFIG` (default
  `control-plane/cmd/devissuer/dev-tenant.json`, committed, fictional users only):
  `{"tenants":[{"id":"<guid>","users":[{"oid","name","username","email","roles":[]}]}],
  "clients":[{"clientId":"<guid>"}],"apis":[{"clientId":"<guid>","scopes":["Hub.Access"]}]}`.
  It holds no secret.
- **Tokens**: access tokens with `ver`, `iss`, `aud` (the API's client id), `azp`, `scp`, `tid`,
  `oid`, `name`, `preferred_username`, `email`, `roles`, `iat`, `nbf`, `exp`; ID tokens with `aud` =
  the client id and the request's `nonce`. Lifetimes `PANACEA_APP_DEVISSUER_ACCESS_TTL` (default
  60 m; set 2 m to test refresh) and 24 h for refresh tokens.
- **Redirect rule**: `http://127.0.0.1` or `http://localhost`, any port, the registered path — the
  rule §6.1 relies on from Entra.
- **Differences from Entra, known** — each is covered by H1's one real-tenant acceptance test, not
  by `devissuer`: conditional access and MFA; consent prompts; real token sizes (R1); group overage
  (unused); Entra keeping an old refresh token valid after rotation (`devissuer` is stricter);
  Continuous Access Evaluation (not requested: the client declares no `cp1` capability).

The Hub accepts `devissuer` only when `PANACEA_APP_HUB_DEV_ISSUER=true` **and**
`PANACEA_APP_HUB_ENV=local`; otherwise an `http` authority is a start-up error.

### 6.5 The registrations — exact values for phase 8's task 09 (HA-3)

| Setting | Desktop — *PanaceaApp Desktop* | Hub API — *PanaceaApp Hub API* |
|---|---|---|
| Sign-in audience | `AzureADMyOrg` | `AzureADMyOrg` |
| Platform | *Mobile and desktop applications* (`publicClient`) | none |
| Redirect URIs | `http://127.0.0.1` and `http://localhost` | none |
| Credentials | **none** | **none** |
| `isFallbackPublicClient` | `false` | `false` |
| Application ID URI | — | `api://<its own client id>` |
| `api.requestedAccessTokenVersion` | — | `2` |
| Exposed scope | — | `Hub.Access`, type *Admins and users*, admin display name *"Use PanaceaApp's shared workspace as the signed-in user"* |
| Pre-authorised clients | — | the desktop's client id, for `Hub.Access` |
| App roles | — | `Hub.OrgAdmin`, allowed member types *Users/Groups*, display name *"PanaceaApp org administrator"* |
| Optional claims | — | access token: `email` |
| `groupMembershipClaims` | `None` | `None` |
| Required resource access | Microsoft Graph delegated `openid`, `profile`, `offline_access`, `email`; the Hub API `Hub.Access`; phase 8's broker `Sandboxes.Access`; if ADR Q5 is yes, Azure DevOps `user_impersonation` | none |
| Admin consent | **granted tenant-wide** for every row above | — |
| Enterprise app: assignment required | no | **yes**, group *PanaceaApp Users* |
| Role assignments | — | `Hub.OrgAdmin` → group *PanaceaApp Org Admins* |

**Unverified**: whether a registration's `http://127.0.0.1` redirect is accepted by the portal or
needs the manifest; that Entra matches a loopback redirect on any port with the root path. If only
`http://localhost` can be registered, set `PANACEA_APP_ENTRA_REDIRECT_HOST=localhost` (§6.1).

---

## 7. Routes

### 7.0 The whole table

Role is the minimum from §5.3. Go 1.23 `ServeMux` patterns; `{sha}` is 64 lowercase hex.

| # | Method | Path | Role | Task | Section |
|---|---|---|---|---|---|
| 1 | GET | `/healthz` | none | H0 | §12.4 |
| 2 | GET | `/readyz` | none | H0 | §12.4 |
| 3 | GET | `/v1/me` | any | H2 | §7.2 |
| 4 | GET | `/v1/teams` | any | H2 | §7.3 |
| 5 | POST | `/v1/teams` | any | H2 | §7.3 |
| 6 | GET | `/v1/teams/{teamId}` | V | H2 | §7.3 |
| 7 | PATCH | `/v1/teams/{teamId}` | A | H2 | §7.3 |
| 8 | PUT | `/v1/teams/{teamId}/members/{userId}` | A | H2 | §7.3 |
| 9 | DELETE | `/v1/teams/{teamId}/members/{userId}` | A, or self | H2 | §7.3 |
| 10 | GET | `/v1/teams/{teamId}/invites` | V | H2 | §7.3 |
| 11 | POST | `/v1/teams/{teamId}/invites` | A | H2 | §7.3 |
| 12 | DELETE | `/v1/teams/{teamId}/invites/{inviteId}` | A | H2 | §7.3 |
| 13 | GET | `/v1/projects` | any | H3 | §7.4 |
| 14 | POST | `/v1/projects` | M of the team | H3 | §7.4 |
| 15 | GET | `/v1/projects/{projectId}` | V | H3 | §7.4 |
| 16 | PATCH | `/v1/projects/{projectId}` | A | H3 | §7.4 |
| 17 | POST | `/v1/projects/{projectId}/remotes` | A | H3 | §7.4 |
| 18 | DELETE | `/v1/projects/{projectId}/remotes?repoKey=` | A | H3 | §7.4 |
| 19 | GET | `/v1/projects/{projectId}/briefs` | V | H3 | §7.5 |
| 20 | POST | `/v1/projects/{projectId}/briefs` | M | H3 | §7.5 |
| 21 | GET | `/v1/projects/{projectId}/briefs/for-branch?branchRef=` | V | H3 | §7.5 |
| 22 | GET | `/v1/briefs/{briefId}` | V | H3 | §7.5 |
| 23 | PATCH | `/v1/briefs/{briefId}` | M | H3 | §7.5 |
| 24 | POST | `/v1/briefs/{briefId}/archive` | M | H3 | §7.5 |
| 25 | GET | `/v1/briefs/{briefId}/revisions` | V | H3 | §7.5 |
| 26 | POST | `/v1/projects/{projectId}/brief-revisions` | M | H3 | §7.5 |
| 27 | GET | `/v1/refs?key=…` | any; per key | H3 (brief), H4 (design) | §7.5 |
| 28 | PUT | `/v1/projects/{projectId}/blobs/{sha}` | M | H4 | §7.6 |
| 29 | GET, HEAD | `/v1/projects/{projectId}/blobs/{sha}` | V | H4 | §7.6 |
| 30 | POST | `/v1/projects/{projectId}/design-approvals` | M | H4 | §7.7 |
| 31 | GET | `/v1/projects/{projectId}/design-approvals` | V | H4 | §7.7 |
| 32 | GET | `/v1/design-approvals/{approvalId}` | V | H4 | §7.7 |
| 33 | POST | `/v1/projects/{projectId}/session-links` | M | H4 | §7.8 |
| 34 | GET | `/v1/projects/{projectId}/session-links` | V | H4 | §7.8 |
| 35 | POST | `/v1/projects/{projectId}/releases` | M | H5 | §7.9 |
| 36 | GET | `/v1/projects/{projectId}/releases` | V | H5 | §7.9 |
| 37 | GET | `/v1/releases/{releaseId}` | V | H5 | §7.9 |
| 38 | POST | `/v1/releases/{releaseId}/confirm` | the reserver | H5 | §7.9 |
| 39 | POST | `/v1/releases/{releaseId}/abandon` | the reserver | H5 | §7.9 |
| 40 | POST | `/v1/releases/{releaseId}/deployments` | M | H5 | §7.9 |
| 41 | POST | `/v1/evidence` | M of the repo's project | H6 | §7.10 |
| 42 | GET | `/v1/evidence?repo=&digest=` | V of the repo's project | H6 | §7.10 |
| 43 | GET | `/v1/changes?after=` | any; filtered | H7 | §7.11 |
| 44 | GET | `/v1/audit?after=` | org admin | H7 | §7.12 |
| 45 | POST | `/v1/briefs/{briefId}/erase` | org admin | H7 | §7.12 |

Nothing else is routed. An unknown path is 404 `route_not_found`; a known path with the wrong
method is 405 `method_not_allowed`.

### 7.1 Shared objects

Every response object below is one of these. Fields are always present; a field that has no value
is `null` (objects, strings that may be absent) or `[]` (lists), never omitted.

```jsonc
// User — as others see a principal
{ "id": "u-3f2a91c07b1e4d2a", "displayName": "Dylan Mackie", "email": "dmackie1@kpmg.com.au" }

// Org
{ "id": "org-0c1d2e3f4a5b6c7d", "displayName": "KPMG Australia", "tenantId": "<guid>" }

// Team — `role` is the CALLER's role in it
{ "id": "tm-…", "name": "Engagement Portal", "role": "admin", "createdAt": "…", "createdBy": User }

// Member
{ "user": User, "role": "admin" | "member" | "viewer", "addedAt": "…", "addedBy": User | null }

// Invite — `boundUser` set once state is "bound"
{ "id": "iv-…", "teamId": "tm-…", "email": "a.person@kpmg.com.au", "role": "member",
  "state": "pending" | "bound" | "revoked", "createdAt": "…", "createdBy": User, "boundUser": User | null }

// Remote
{ "repoKey": "github.com/lcfrai/panaceaapp", "host": "github" | "azure-devops", "addedAt": "…" }

// Project
{ "id": "pj-…", "teamId": "tm-…", "name": "Engagement Portal", "remotes": [Remote],
  "createdAt": "…", "createdBy": User }

// BriefHead — the living brief; head* are null while headRevision is 0
{ "id": "bf-…", "projectId": "pj-…", "scope": "project" | "branch", "branchRef": "refs/heads/…" | "",
  "headRevision": 6, "headKey": "brief:bf-…@6", "headDigest": "sha256:…", "headStatus": "complete",
  "latestCompleteRevision": 6, "archivedAt": null, "createdAt": "…", "createdBy": User, "updatedAt": "…" }

// BriefRevision — immutable; content is null exactly when erasedAt is set
{ "key": "brief:bf-…@5", "briefId": "bf-…", "revision": 5, "digest": "sha256:…",
  "projectId": "pj-…", "scope": "branch", "branchRef": "refs/heads/evidence-upload",
  "status": "open" | "complete" | "skipped", "content": { … }, "savedAt": "…", "savedBy": User,
  "erasedAt": null }
```

### 7.2 `GET /v1/me`

Provisions the caller if new (§5.1) and **binds pending invites**: every `pending` invite in the
caller's org whose `email` equals the caller's email (§5.1) becomes `bound`, and the caller becomes
a member with the invite's role — or keeps a higher role already held. The control plane calls this
once after every sign-in (§6.1 step 11).

```jsonc
// 200
{ "user": User, "org": Org, "orgAdmin": false,
  "teams": [Team], "invitesBound": 1 }
```

### 7.3 Teams, members, invites

**`GET /v1/teams`** → `200 {"items":[Team],"next":null}` — the caller's teams, by name. Unpaginated
in v1 (a person is in few teams); `next` is always `null`.

**`POST /v1/teams`** `{"name":"Engagement Portal"}` → `201 Team`. `name`: 1–120 characters after
trimming, unique per org among teams (409 `conflict`, `details.field = "name"`). The caller becomes
its `admin`. Event `team.created`.

**`GET /v1/teams/{teamId}`** → `200 {"team": Team, "members": [Member]}`, members by display name.

**`PATCH /v1/teams/{teamId}`** `{"name":"…"}` → `200 Team`. Event `team.updated`.

**`PUT /v1/teams/{teamId}/members/{userId}`** `{"role":"member"}` → `200 Member` (changed) or
`201 Member` (added). `userId` must be a user of the caller's org (else 404 `member_not_found`); in
v1 a user appears only after their first sign-in, so the usual path to add someone is an invite.
Demoting the last admin is 409 `last_admin`. Event `team.member_set`.

**`DELETE /v1/teams/{teamId}/members/{userId}`** → `204`. An admin removes anyone; anyone removes
themselves. Removing the last admin is 409 `last_admin`. Event `team.member_removed`.

**`GET /v1/teams/{teamId}/invites`** → `200 {"items":[Invite],"next":…}`, newest first.

**`POST /v1/teams/{teamId}/invites`** `{"email":"a.person@kpmg.com.au","role":"member"}` →
`201 Invite`. `email` is lowercased, 3–254 characters, one `@`. A second pending invite for the same
email and team is 409 `conflict`. An email that already belongs to a member is 409 `conflict`,
`details.field = "email"`. Event `team.invite_created`. No email is sent in v1: the admin tells the
person, who signs in.

**`DELETE /v1/teams/{teamId}/invites/{inviteId}`** → `204`; state becomes `revoked` (the row is
kept). A `bound` invite cannot be revoked: 409 `conflict`. Event `team.invite_revoked`.

### 7.4 Projects and remotes

**`GET /v1/projects?repo=<repoKey>&teamId=<teamId>`** → `200 {"items":[Project],"next":…}`, by name.
Both filters optional. **This is how the control plane maps a local project to a Hub project**: it
computes each remote's `githost.RepoKey` (phase 3, H-3) and asks. At most one project in an org has a
given remote, so `repo=` returns zero or one item.

**`POST /v1/projects`**
```json
{ "teamId": "tm-…", "name": "Engagement Portal",
  "remotes": [ { "repoKey": "github.com/lcfrai/panaceaapp", "host": "github" } ] }
```
→ `201 Project`. `name` 1–120 characters, unique per team. `remotes` may be empty. A `repoKey`
already linked anywhere in the org is 409 `repo_already_linked`. `repoKey` must match
`^github\.com/[A-Za-z0-9-]{1,39}/[A-Za-z0-9._-]{1,100}$` for `github`, or
`^[a-z0-9-]{1,50}\.visualstudio\.com/[^/]{1,64}/[^/]{1,64}$` for `azure-devops` (Azure DevOps
project names may contain spaces) — phase 3's two forms (`2026-09-23-phase-3-git-convergence.md:299`),
canonicalised by phase 3's `githost` before sending; the Hub compares them byte for byte. Event
`project.created`.

**`GET /v1/projects/{projectId}`** → `200 Project`.

**`PATCH /v1/projects/{projectId}`** `{"name":"…"}` → `200 Project`. Event `project.updated`.

**`POST /v1/projects/{projectId}/remotes`** `{"repoKey":"…","host":"…"}` → `201 Remote`; 409
`repo_already_linked` as above. Event `project.remote_added`.

**`DELETE /v1/projects/{projectId}/remotes?repoKey=<repoKey>`** → `204`. Evidence already recorded
under that repo stays (§7.10). Event `project.remote_removed`.

There is **no project deletion** in v1 (provenance contract P13: a release outlives its branch, and
its project).

### 7.5 Briefs, revisions, and the resolver — phase 4

**The division of labour.** Phase 4's domain logic — the questions, `discover.Resolve`'s
inheritance, what makes a brief complete — stays in the control plane. The Hub stores heads and
immutable revisions, numbers them, digests them, and resolves keys. It reads exactly one field of a
revision's content: `status`.

**`content`** is the object phase 4 writes: in v1, `discover.Brief`'s `questions`, `answers` and
`status` fields as its plan defines them (`2026-09-18-phase-4-discover.md:969-980`), and nothing that
the envelope already holds (`briefId`, `revision`, `projectId`, `scope`, `branchRef`). The Hub
requires: a JSON object; `status` one of `open`, `complete`, `skipped`; integers only (§3.4); depth
at most 16; canonical envelope at most **64 KiB**. Anything else inside is phase 4's to define and
may grow additively. Phase 4 **should** write every time inside `content` in §1's format, because the
Hub does not normalise inside `content` and the digest covers the bytes.

**`POST /v1/projects/{projectId}/briefs`**
```json
{ "scope": "branch", "branchRef": "refs/heads/evidence-upload" }
```
→ `201 BriefHead` with `headRevision: 0`. `scope: "project"` requires `branchRef` absent or `""`.
One live project brief per project, and one live branch brief per `(project, branchRef)`: a second
is 409 `brief_exists` with the existing `briefId`. Event `brief.created`.

**`GET /v1/projects/{projectId}/briefs?scope=&branchRef=&includeArchived=false`** →
`200 {"items":[BriefHead],"next":…}`, oldest first.

**`GET /v1/projects/{projectId}/briefs/for-branch?branchRef=refs/heads/…`** — the head reading
(provenance contract §3.1's branch endpoint). **Never 404 for an absent brief**; 404 only for the
project.
```jsonc
// 200
{ "branchBrief":  { "head": BriefHead, "revision": BriefRevision | null } | null,
  "projectBrief": { "head": BriefHead, "revision": BriefRevision | null } | null }
```
`revision` is the head revision (null while `headRevision` is 0). `branchBrief` is the live branch
brief for that ref; `projectBrief` is the project's live project brief — both, because phase 4's
`API.resolve` builds a branch's document from the pair (`:4250-4261`).

**`GET /v1/briefs/{briefId}`** → `200 {"head": BriefHead, "revision": BriefRevision | null}`.

**`PATCH /v1/briefs/{briefId}`** `{"branchRef":"refs/heads/new-name"}` → `200 BriefHead` — after a
branch rename. Branch briefs only (a project brief is 400 `bad_request`); another live brief on the
new ref is 409 `brief_exists`. Writes no revision; the next revision's envelope carries the new ref.
Event `brief.rebound`.

**`POST /v1/briefs/{briefId}/archive`** `{}` → `200 BriefHead` with `archivedAt` set. For branch
deletion (provenance contract P13): **no revision is touched**, every key still resolves, and the
ref is free for a new brief. Writing to an archived brief is 409 `brief_archived`. There is no
un-archive in v1. Event `brief.archived`.

**`GET /v1/briefs/{briefId}/revisions?after=&limit=`** → history, oldest first, **without content**:
```jsonc
{ "items": [ { "key": "brief:bf-…@1", "revision": 1, "digest": "sha256:…", "status": "open",
               "savedAt": "…", "savedBy": User, "erasedAt": null } ], "next": "…" | null }
```
Discover's past-revision view (provenance contract §4.1's one click) fetches a revision's text through
`/v1/refs`.

**`POST /v1/projects/{projectId}/brief-revisions`** — **the only write of brief text**.
```json
{ "revisions": [
    { "briefId": "bf-3f2a91c07b1e4d2a", "baseRevision": 5,
      "content": { "status": "complete", "questions": [], "answers": [] } }
] }
```
- 1–16 items, distinct `briefId`s, each a live brief of this project. The batch is **one
  transaction, all or nothing**: a project-brief edit that re-revisions every inheriting branch brief
  (critique-2 §7.1 #3, accepted under `OD-1`) is one call.
- Heads are locked in `briefId` order. For each item, `baseRevision` must equal the head's
  `headRevision`, else the whole batch is 409 `revision_conflict` naming the first brief that
  failed and its `headRevision`. The caller re-reads and retries; the Hub never merges.
- Each new revision is `baseRevision + 1`. Its envelope (§3.4) is built and stored; its digest is
  computed; the head moves.
- `201 {"revisions":[BriefRevision]}`, in request order. One `brief.revised` event per revision.
- The control plane sends an `Idempotency-Key` (§1), so a retry after a lost response replays the
  201 rather than tripping its own `revision_conflict`.

**`GET /v1/refs?key=<k1>&key=<k2>…`** — the batch resolver (provenance contract §3.1's by-key
endpoint, and ruling R6's one loader). 1–100 `key` parameters, `brief:` and `design:` mixed; a
repeated key is answered in each slot.
```jsonc
// 200 — one result per key, in request order
{ "results": [
    { "key": "brief:bf-…@5",  "kind": "brief",  "resolution": "resolved", "revision": BriefRevision, "approval": null },
    { "key": "brief:bf-…@9",  "kind": "brief",  "resolution": "missing",  "revision": null, "approval": null },
    { "key": "brief:bf-…@2",  "kind": "brief",  "resolution": "erased",   "revision": BriefRevision /* content null */, "approval": null },
    { "key": "design:cv-…@11","kind": "design", "resolution": "resolved", "revision": null, "approval": DesignApproval },
    { "key": "b-42",          "kind": null,     "resolution": "invalid",  "revision": null, "approval": null }
  ],
  "heads": { "bf-…": { "headRevision": 6, "latestCompleteRevision": 6, "archived": false } } }
```
| `resolution` | When |
|---|---|
| `resolved` | the record exists, is readable by the caller, and (brief) its stored bytes still hash to its digest |
| `missing` | no such record, **or one the caller cannot read** — indistinguishable on purpose (§4.2). Also a brief revision whose bytes no longer match its digest: that is data loss and is logged as an `internal` error |
| `erased` | an org admin erased it (§7.12). Key, digest, `savedAt` and `savedBy` remain |
| `invalid` | not a key in §3.2's grammar |

`heads` has one entry for every brief named by a `resolved` or `erased` brief key, and for the brief
of every resolved design approval's `briefKey`. It is the only time-varying part of the response:
the `provenanceSlice` caches results forever and `heads` per brief (provenance contract §6.4).
**`erased` is a new provenance state** (ADR §5.6), an amendment phase 4 makes to `Ref.resolution`
and to the six-state table in provenance contract §4.1.
