### Task 39: the credential routes — the one write path for a pasted GitHub or Panacea token (correction A8)

**Lane C. Depends on: 29, 37. About 2 hours. Contract task C1 (seam S-CRED).**

The contract names where a GitHub and a Panacea token live (§9.1) but not how one gets there
(correction **A8**). Without a write path, phase 3's `credstore.Local` reads an always-empty store,
and phase 7 builds a second write path into the same store. This task is the one path: three
**control-plane** routes — none on the Hub, which never holds a git or Panacea credential (ADR-H1
§5.2) — **write-only for the secret**: a token goes in, and no route ever returns one. Phase 3's
and phase 7's "connect" surfaces call them (task 40 gives the desktop the client functions).

**Files:**
- `control-plane/internal/api/credentials.go` + `credentials_test.go` (new)
- `control-plane/internal/api/api.go` — `Deps.Secrets`; `a.registerCredentials(guarded)` (modify ⚑)
- `control-plane/internal/api/routes_audit_test.go` — `GET /v1/credentials` (modify ⚑)
- `control-plane/cmd/api/main.go` — pass the `Secrets` (modify ⚑)

#### Contract — the routes, complete

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| POST | `/v1/credentials/{kind}` | `{"scope":"…","token":"…","subject":"…"}` (`subject` optional, ≤ 256 characters) | `204`, no body | 400 `bad_request` `field: kind` / `scope` / `token` / `subject`; 503 `backend_unavailable` is **not** used — a store failure is 500 `internal` with the cause logged, never the token |
| DELETE | `/v1/credentials/{kind}?scope=…` | — | `204` (absent is not an error) | 400 `field: kind` / `scope` |
| GET | `/v1/credentials` | — | `200 {"items":[{"kind":"github","scope":"lcfrai","subject":"octocat","savedAt":"…"}]}` from `credstore.Inventory` — GitHub, Panacea and the Entra account entry; **no token, ever** | — |

The rules per `kind`:

| `kind` | `scope` | `token` shape | Stored by |
|---|---|---|---|
| `github` | `""` (every owner) or an owner, `^[a-z0-9-]{1,39}$` after lowercasing | `^(ghp\|gho\|ghu\|ghs\|ghr)_[A-Za-z0-9]{36,251}$` or `^github_pat_[A-Za-z0-9_]{22,251}$` — GitHub's documented token prefixes | `credstore.GitHubTokens.Put` |
| `panacea` | the API host: a lowercase hostname, optional `:port`, no scheme, no path — e.g. `dev.panacea.kpmg` | `^pnc_[A-Za-z0-9_-]{16,2000}$` — **the `pnc_` prefix is Panacea's** (phase 7 brief `:119`); the length bound is this plan's guess, unverified | `credstore.PanaceaTokens.Put` |

`savedAt` is the control plane's clock. `subject` is whatever the user typed (or phase 3/7's
surface learned from the host); it is display text, never an identity. No token is verified
against its host here: a wrong token fails at its first use with the host's own 401, which phase 3
and 7 already relay. The body is read with `httpx.DecodeJSON` (1 MiB cap, unknown fields refused)
and **never logged**; the access log line names the route pattern and the status only.

**The seam, as the consumers see it (S-CRED):** phase 3 reads GitHub tokens through
`credstore.Local` (task 29) in its `Chain`; phase 7 reads its token with
`credstore.PanaceaTokens{Secrets: deps.Secrets}.Get(apiHost)` in its Panacea client — and neither
writes: both send the user here.

#### Tests to write first — `credentials_test.go`, over `credstore.Memory()`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStoreAGitHubToken` | POST `github`, scope `lcfrai` → 204, empty body; `credstore.Local{…}.Credential(Host{GitHub,"github.com","lcfrai"})` returns it | a write path phase 3 cannot read |
| `TestAWildcardGitHubToken` | scope `""` → found for any owner | — |
| `TestStoreAPanaceaToken` | POST `panacea`, scope `dev.panacea.kpmg` → 204; `PanaceaTokens.Get("dev.panacea.kpmg")` returns it | phase 7 building its own path (A8) |
| `TestShapesAreChecked` | `ghp_short`, `token123`, a Panacea token without `pnc_`, a scope `Evil/Owner`, `https://dev.panacea.kpmg`, a kind `gitlab` → 400 naming the field | a pasted URL or a mistyped secret stored as if valid |
| `TestNothingEverReturnsTheToken` | after storing both, `GET /v1/credentials` lists two entries (plus the Entra account when signed in) whose JSON contains neither token; the POST's response body is empty | a listing that leaks what it lists |
| `TestDeleteIsIdempotent` | delete → 204; again → 204; `GET` no longer lists it | — |
| `TestTheBodyIsNeverLogged` | a captured log across every request above contains no token | T10: a token in a log |
| `TestUnknownFieldsAreRefused` | `{"scope":"x","token":"ghp_…","expiresAt":"…"}` → 400 `bad_request` | a field silently dropped that the caller thought was kept |
| `TestTheListIsAuditedReadOnly` | phase 2's route audit passes with `GET /v1/credentials` listed | — |

(Task 37's route walker already asserts all three routes need the local key.)

#### Steps

- [ ] **Step 1:** Re-read contract §9.1; tasks 28–29's accessors.
- [ ] **Step 2:** Tests; they fail. Then `credentials.go`, the `Deps` field, the wiring.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/api/ -race -count=1 -v -run 'TestStoreA|TestAWildcard|TestShapesAre|TestNothingEver|TestDeleteIs|TestTheBodyIs|TestUnknownFields|TestTheListIs' 2>&1 | grep -c "^--- PASS"   # 9
```

#### What this task does not claim

- A GitHub OAuth app or device flow: v1 pastes a token (open question Q9).
- The exact shape of a `pnc_` token beyond its prefix: unverified; the bound is generous.
- A "connect" UI: phase 3's and phase 7's surfaces, calling task 40's client functions.
