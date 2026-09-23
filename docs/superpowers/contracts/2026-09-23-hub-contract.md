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
