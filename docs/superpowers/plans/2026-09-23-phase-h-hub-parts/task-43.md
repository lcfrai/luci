### Task 43: the end-to-end proof — two teammates through `devissuer`; the dev-realm check; the results record

**`feat/hub`, after every other task. Depends on: 01–42. About 5 hours (3 before review M4: a 12-step end-to-end test, debugged
across two control planes and a Hub, plus a re-run of every other task's acceptance block, does
not fit 3). Contract tasks H1 (its manual step), all.**

Every task so far proved its piece. This one proves **the thread**: two teammates, each with their
own control plane and their own credential store, sign in through the real PKCE code, share one
repository's project, and see each other's brief revision, design approval, release pin and
evidence — through every layer that will run on the owner's laptop, with nothing mocked but the
browser's click. Then it writes down, as a checklist for the owner's batched session (`OD-12`),
what only **Panacea's dev realm brokering KPMG's Entra** can prove, and records honestly what was and
was not observed.

**OD-30** (main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`; Hub contract §17).
The issuer is a Keycloak realm, not Entra: the test runs `devissuer` under the `keycloak` profile, the
Hub validates with `oidcauth`, the control planes sign in with `signin`. The real-tenant checklist is
**replaced** by the ruling's section 10 check against Panacea's dev realm, plus the section 12 items
this task settles (U-4, U-4b, U-5, U-6, U-7, U-15). Nothing is asked of KPMG IT; the precondition is
Panacea applying K1-K4.

**Files:**
- `control-plane/internal/hub/e2e_integration_test.go` (new)
- `docs/verification/2026-09-23-phase-h-results.md` — the record, named for the day it is run (new)

#### The end-to-end test — `e2e_integration_test.go` (`//go:build integration`)

One test, `TestTwoTeammatesShareTheThread`, in steps, each a `t.Run` so a failure names its step.
The world: `hubtest.Migrated`; `devissuer` in-process with `Auto` and `AccessTTL` **10 min**; the
Hub's real handler (`server.New` + `oidcauth.New` under `keycloak`, `AllowInsecureAuthority`) on an `httptest.Server`;
**two** control planes (`api.New`), one per teammate, each with its own `credstore.Memory()`, its
own `signin.Authenticator`, `hubclient.Client`, `hublink.Resolver`, `hubfeed.Poller` (interval
1 s) and local key; a temporary git repository whose `origin` is
`https://github.com/lcfrai/panaceaapp.git` as each control plane's only project.

**One clock (review I9).** A single test clock, `clk` — a mutex-guarded `time.Time` with `Now()`
and `Advance(d)` — is passed as `Now` to `devissuer` (task 11's `Config.Now`), to both teammates'
`signin.Config.Now` (task 30), and to the Hub's validator, `oidcauth.Config.Now` (task 10), and
its authentication middleware, `server.AuthConfig.Now` (task 13). It starts at the wall time the test begins and stays **frozen**
through steps 1–8: nothing expires, so no refresh grant is made, and every token a teammate holds is
the one sign-in gave them. Only step 9 moves it. Two clocks would break the test the other way:
tokens minted on an advanced issuer clock carry an `nbf` in the Hub's future. The poll intervals
and step 5's latency bound are **wall** time, on the ticker and `time.Since`, not on `clk`. Each
teammate's `signin.Config.HTTPClient` wraps its transport in a counting `RoundTripper` that
counts `POST`s to the token endpoint with `grant_type=refresh_token`; that count, per teammate, is
what step 9 reads (`devissuer` itself keeps no count).

| Step | Does, through the control plane's HTTP surface where one exists | Asserts |
|---|---|---|
| 1 `grace_signs_in` | `POST /v1/auth/sign-in` → the scripted browser (`&login_hint=grace@example.test`) → poll `GET /v1/auth/status` | `signed_in` as Grace; her store holds the account record and chunks; every request carried her local key |
| 2 `grace_shares` | `POST /v1/projects/{p}/hub-link` with a new team | `linked`; the Hub project's remote is `github.com/lcfrai/panaceaapp` |
| 3 `grace_invites_vera` | `POST /v1/hub/teams/{tm}/invites` | 201 |
| 4 `vera_signs_in_and_is_linked` | Vera's control plane signs in (her own store), then `GET /v1/projects/{p}/hub-link` | the **same** Hub project, found by the repository, not by any id — H-3 in practice |
| 5 `the_brief_thread` | as Grace, through `hubclient.Briefs` (phase 4's calls): create a branch brief, write revision 1 `complete`; as Vera, `Resolve` its key | Vera resolves it `resolved` with the same digest; her feed subscriber received `brief.revised` **within 3 s** of the write, measured on the wall clock with `clk` frozen (1 s poll ×2 + slack; ADR-H1 §5.5) |
| 6 `the_design_thread` | as Grace (phase 5's calls): upload a manifest and a file, approve `design:cv-…@1` against revision 1 | Vera resolves the design key with the approval; `approvedBy` is Grace |
| 7 `the_release_thread` | as Vera (phase 7's calls): reserve a run on the branch, confirm, join `d-e2e1` | the pin is revision 1 `ship_head`; Grace's `Releases(deploymentId: d-e2e1)` finds it; `reservedBy` is Vera |
| 8 `the_evidence_thread` | as Grace (phase 3's call): record evidence against revision 1 on the repo | Vera's `EvidenceByDigest` returns it; `recordedBy` formats as `Grace Example <grace@example.test>` |
| 9 `tokens_refresh_unseen` | before: each teammate's refresh-grant count is **0** (steps 1–8 made none). `clk.Advance(10 min)`, past the access token's life; a Hub call from Grace's control plane; then wait (bounded, 3 s wall) for Vera's poller to make its next Hub call | Grace's call succeeds; Grace's count is exactly **1** and Vera's exactly **1** (her poller's, sharing one redemption), and neither moves over a further 2 s of polling: the new tokens live 10 min on a frozen clock; each store, entry by entry, is **unchanged**: the sign-in's original offline token is kept and no narrowed grant's refresh token is written (task 31, OD-30 U-4b) |
| 10 `revocation_signs_out` | `devissuer.RevokeRefreshTokens(vera)`; expire her token with `Invalidate(HubAudience)` on her `Authenticator` (task 31), not the clock, so Grace's session is not touched; a Hub call | 401 `signed_out` from her control plane; `GET /v1/auth/status` → `reauth_required`; her store is empty |
| 11 `the_local_key_holds` | a request to Grace's control plane without the key; one with Vera's key | both 401 `local_key_required` |
| 12 `the_chain_verifies` | `hub verify` against the database | exit 0, `ok revisions`, `ok audit` |

`hubclient/conformance`'s `TestAllGroupsAreDetected` and `TestTheRealHubConforms` (task 34) must
also pass in this run, with **all eight groups**.

#### The dev-realm check — written into the results record, run in the owner's batched session

Ruling section 10's check, plus the section 12 items this task settles. Nothing below can be run
until Panacea has applied K1-K4 to realm `panacea` on its dev estate (ruling §10; nothing is asked of
KPMG IT, OD-30). Each item gets **expected**, **observed** and **verdict** columns in the record, left
empty until the session. The configuration for it: the control plane with
`PANACEA_APP_OIDC_ISSUER=https://auth.dev.panacea.kpmg/realms/panacea`, `PANACEA_APP_OIDC_PROFILE=keycloak`,
`PANACEA_APP_OIDC_CLIENT_ID=panacea-app-desktop`, `PANACEA_APP_OIDC_IDP_HINT=entra` and
`PANACEA_APP_HUB_AUDIENCE=panacea-app-hub`; the Hub run **on the host** (so it uses the Windows
certificate store, and the corporate proxy's CA, for the realm's JWKS) with `PANACEA_APP_HUB_ENV=local`,
`PANACEA_APP_HUB_DEV_ISSUER=false`, the same issuer and profile, KPMG's tenant id, audience
`panacea-app-hub` and allowed client `panacea-app-desktop`. Claims are recorded by **name and type**,
never a value beyond `typ`, `aud`, `azp`, `scope`, `email_verified` and `exp - iat`.

| # | Check | Expected | Risk it retires |
|---|---|---|---|
| R-1 | Panacea's realm matches ruling §10 K1-K4 row by row, read by its realm admin: the client, the `panacea app desktop browser` flow binding, the four scopes, P1-P3, M1/M2 and the `email` mapper at `FORCE`, `loginHint`, the user profile (`entra_oid`, `entra_tid`, `email` admin-edit; `unmanagedAttributePolicy` not `ENABLED`), the offline caps, `hub.org_admin`, and **exactly one enabled IdP, `entra`** (K1's invariant) | every row | a live realm that differs from the script (ruling U-2) mistaken for a code bug |
| R-2 | Launch PanaceaApp signed out: the tab reaches **"Signed in. You can return to PanaceaApp."** with **no click**; `GET /v1/auth/status` shows a GUID `oid` and `tid`; record whether `login_hint` was sent and whether any Entra screen appeared | no click; GUIDs | ruling §10 check 1; U-6 (PRT browser SSO), U-7 (the Windows UPN), U-3 (the loopback port form of `http://127.0.0.1/callback`) |
| R-3 | Paste the authorize URL **without `kc_idp_hint`**; then with `kc_idp_hint=` (empty) and with `kc_idp_hint=nosuch` | without it: still lands on Entra, no Keycloak username/password page; empty or unknown: an error page, no password form, no token | ruling §10 check 2; review-1 I1 (the realm's password form and Cookie step bypassing Entra); review-2 I1 (the hint is read before `defaultProvider`, so only the one-IdP invariant makes it harmless; fail closed is UNVERIFIED until this row) |
| R-4 | As the owner, `POST` a changed `entra_oid`, and separately a changed `email`, to `/realms/panacea/account` | both refused | ruling §10 check 3; review-1 M2 (a forged `oid`), I3 (a self-edited invite address) |
| R-5 | A real Hub access token's claims: `typ` `Bearer`; `aud` contains `panacea-app-hub` and **no other `panacea-app-*`**; `azp` `panacea-app-desktop`; `scope` contains `panacea-app-hub`; `oid`, `tid` GUID strings; `roles`; `email`; `email_verified` the boolean `true`. An invite for the owner's `email` binds at their `GET /v1/me` (`invitesBound: 1`) | as listed; the invite binds | U-4 (narrowing honoured) and review-1 I2; I3's binding rule on the real claims; ADR-H1 R9 (`devissuer` diverging from the realm) |
| R-6 | One stored token, four calls in turn: Hub → broker → gateway → Hub (`Token` for each audience, the last with the Hub's cache invalidated). Each token's `aud` holds its one audience; the last Hub call succeeds; `cmdkey /list` and the account record are unchanged across all four | single audiences; the last call 200; the store unchanged | U-4b and review-1 I4 (a narrowed refresh token written back, the Hub dark); if any token has two audiences every validator refuses it (`ErrAudience`), recorded for the owner |
| R-7 | `exp - iat` of one access token | `600` | U-15 and review-1 M4: the 600 s override applies on Keycloak 25.0.0; otherwise 1800 s, 30 minutes of revocation latency |
| R-8 | The offline refresh token's size in bytes, and so the chunk count | recorded | ADR-H1 R1 |
| R-9 | `cmdkey /list` shows the `PanaceaApp/oidc/auth.dev.panacea.kpmg/panacea/…` entries, persisted on this machine | present | `OD-6`'s store actually used (task 28's targets) |
| R-10 | Quit and relaunch: signed in with **no** browser tab; if Panacea restarts its Keycloak pod, relaunch again | silent; after a restart at most one zero-click sign-in | ruling §3 silent refresh; U-5 (offline sessions over a one-replica, `--cache=local` restart) |
| R-11 | The owner's token carries `roles: ["hub.org_admin"]`; `GET /v1/audit` answers 200 for them | as stated | K4's realm role |
| R-12 | A real Azure DevOps PAT, minted by the owner for one organisation, pasted through `POST /v1/credentials/azure-devops` (task 39); record its **length** and whether it is all `[A-Za-z0-9]` (never its value); then `credstore.LocalDevOps` serves it and a `git ls-remote` of a repository in that organisation succeeds through phase 3's chain | 204; listed by `GET /v1/credentials`; `ls-remote` exit 0 | review I10: task 39's PAT bound `^[A-Za-z0-9]{52,256}$` is from knowledge; the PAT is the only ADO path (OD-30 ruling §7) |
| R-13 | **A first-time user**: Panacea's realm admin deletes a test user from the dev realm (or a colleague who has never used it signs in), then that person launches PanaceaApp signed out; decode one access token and read the realm user | no click; the token carries `oid`, `tid`, `email`, `email_verified` `true`; the realm user holds `email`, `entra_oid`, `entra_tid`; R-4 still refused for that user | ruling §10 check 4, U-16, review-2 I2: K4's admin-only `email` edit may break first-broker-login's user creation, and R-2, R-4 and R-10 all use an existing user. If it fails, record it for the owner, who picks the fallback |

Dropped from the earlier Entra checklist, with the reason: the registration rows (§6.5 is deleted),
`PANACEA_APP_ENTRA_REDIRECT_HOST=localhost` (the realm registers `127.0.0.1` only), `acct`/`xms_edov`
(a realm token carries neither), Azure DevOps' Entra scope (deleted), and "assignment required" (the
realm's `tid` filter admits the tenant; ruling §9).

#### The results record

`docs/verification/2026-09-23-phase-h-results.md` holds, in this order: the commit it was run at;
every acceptance command of tasks 01–42 **as run**, with its output's last line; the end-to-end
test's output; what the implementer observed in `npm run tauri dev` in tasks 38, 41 and 42 —
labelled **the implementer's observation**; the dev-realm check with its three empty
columns; and a closing list, **Unverified**, that names at least: the owner's visual acceptance of
tasks 41 and 42 (`OD-12`), every R-item, Azure Postgres (only `postgres:16-alpine` was used), the
workload-identity database login, and phase 8's deployed chart.

#### Steps

- [ ] **Step 1:** Merge state: every lane merged into `feat/hub`; `go test ./sandboxd/... ./control-plane/... -race -count=1` and `cd desktop && npm test` green.
- [ ] **Step 2:** `devstack.sh up`; write and run the end-to-end test until every step passes.
- [ ] **Step 3:** Run every task's acceptance block; write the record as above.
- [ ] **Step 4:** Commit by pathspec, the test and the record.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/ -run TestTwoTeammatesShareTheThread -race -count=1 -v 2>&1 | grep -c "^    --- PASS"   # 12
go test -tags integration ./control-plane/internal/hub/... ./control-plane/cmd/hub/ -race -count=1                                                 # ok
grep -c "^| R-" docs/verification/2026-09-23-phase-h-results.md                                                                                   # 12
grep -n "Unverified" docs/verification/2026-09-23-phase-h-results.md                                                                              # present
```

#### What this task does not claim

- **Anything the R-list covers**: until the owner's session, every R-item is unverified, and the
  record says so.
- That the owner has seen the UI: the implementer's observation is not acceptance (`CLAUDE.md`,
  working agreements).
- That phase 8's environment runs this: the chart is phase 8's, never applied (`OD-5`).
