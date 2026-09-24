### Task 43: the end-to-end proof — two teammates through `devissuer`; the real-tenant checklist; the results record

**`feat/hub`, after every other task. Depends on: 01–42. About 3 hours. Contract tasks H1 (its manual step), all.**

Every task so far proved its piece. This one proves **the thread**: two teammates, each with their
own control plane and their own credential store, sign in through the real PKCE code, share one
repository's project, and see each other's brief revision, design approval, release pin and
evidence — through every layer that will run on the owner's laptop, with nothing mocked but the
browser's click. Then it writes down, as a checklist for the owner's batched session (`OD-12`),
what only a **real KPMG tenant** can prove, and records honestly what was and was not observed.

**Files:**
- `control-plane/internal/hub/e2e_integration_test.go` (new)
- `docs/verification/2026-09-23-phase-h-results.md` — the record, named for the day it is run (new)

#### The end-to-end test — `e2e_integration_test.go` (`//go:build integration`)

One test, `TestTwoTeammatesShareTheThread`, in steps, each a `t.Run` so a failure names its step.
The world: `hubtest.Migrated`; `devissuer` in-process with `Auto` and `AccessTTL` **10 min**; the
Hub's real handler (`server.New` + `entra.New`, `AllowInsecureAuthority`) on an `httptest.Server`;
**two** control planes (`api.New`), one per teammate, each with its own `credstore.Memory()`, its
own `entraauth.Authenticator`, `hubclient.Client`, `hublink.Resolver`, `hubfeed.Poller` (interval
1 s) and local key; a temporary git repository whose `origin` is
`https://github.com/lcfrai/panaceaapp.git` as each control plane's only project.

**One clock (review I9).** A single test clock, `clk` — a mutex-guarded `time.Time` with `Now()`
and `Advance(d)` — is passed as `Now` to `devissuer` (task 11's `Config.Now`), to both teammates'
`entraauth.Config.Now` (task 30), and to the Hub's validator, `entra.Config.Now` (task 10), and
its authentication middleware, `server.AuthConfig.Now` (task 13). It starts at the wall time the test begins and stays **frozen**
through steps 1–8: nothing expires, so no refresh grant is made, and every token a teammate holds is
the one sign-in gave them. Only step 9 moves it. Two clocks would break the test the other way:
tokens minted on an advanced issuer clock carry an `nbf` in the Hub's future. The poll intervals
and step 5's latency bound are **wall** time, on the ticker and `time.Since`, not on `clk`. Each
teammate's `entraauth.Config.HTTPClient` wraps its transport in a counting `RoundTripper` that
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
| 9 `tokens_refresh_unseen` | before: each teammate's refresh-grant count is **0** (steps 1–8 made none). `clk.Advance(10 min)`, past the access token's life; a Hub call from Grace's control plane; then wait (bounded, 3 s wall) for Vera's poller to make its next Hub call | Grace's call succeeds; Grace's count is exactly **1** and Vera's exactly **1** (her poller's, sharing one redemption), and neither moves over a further 2 s of polling: the new tokens live 10 min on a frozen clock; each store's account record names a new `refreshGen` (task 31's rotation) |
| 10 `revocation_signs_out` | `devissuer.RevokeRefreshTokens(vera)`; expire her token with `Invalidate(HubScope)` on her `Authenticator` (task 31), not the clock, so Grace's session is not touched; a Hub call | 401 `signed_out` from her control plane; `GET /v1/auth/status` → `reauth_required`; her store is empty |
| 11 `the_local_key_holds` | a request to Grace's control plane without the key; one with Vera's key | both 401 `local_key_required` |
| 12 `the_chain_verifies` | `hub verify` against the database | exit 0, `ok revisions`, `ok audit` |

`hubclient/conformance`'s `TestAllGroupsAreDetected` and `TestTheRealHubConforms` (task 34) must
also pass in this run, with **all eight groups**.

#### The real-tenant checklist — written into the results record, run in the owner's batched session

Nothing below can be run until KPMG IT has created the registrations of ADR-H1 §9 (items 1–6).
Each item gets **expected**, **observed** and **verdict** columns in the record, left empty until
the session. The configuration for it: the control plane with `PANACEA_APP_ENTRA_AUTHORITY=https://login.microsoftonline.com/<KPMG tenant>`,
the desktop client id and the Hub scope; the Hub run **on the host** (so it uses the Windows
certificate store, and the corporate proxy's CA, for `login.microsoftonline.com`) with
`PANACEA_APP_HUB_ENV=local`, `PANACEA_APP_HUB_DEV_ISSUER=false`, the real tenant, audience and
allowed client.

| # | Check | Expected | Risk it retires |
|---|---|---|---|
| R-1 | The two registrations and the enterprise app match contract §6.5 row by row | every row | a misconfigured registration mistaken for a code bug |
| R-2 | *Sign in* opens the default browser on Microsoft's page; after sign-in the tab says *"You're signed in to PanaceaApp"* | as described | ADR-H1 R2: conditional access blocking a public client with a loopback redirect |
| R-3 | The loopback redirect is accepted on a random port with `127.0.0.1`; if not, with `PANACEA_APP_ENTRA_REDIRECT_HOST=localhost` | one of the two works; record which | contract §6.5's unverified redirect matching |
| R-4 | A real Hub access token's **claim names** (never values beyond `ver`, `scp`, `acct` and `xms_edov`) against task 11's table: `ver 2.0`, `aud` a GUID, `azp`, `scp` containing `Hub.Access`, `tid`, `oid`, `name`, `preferred_username`, `email`, **`acct`** and **`xms_edov`** (both optional claims, configured per contract §6.5 as A15 amends it, by IT from ADR-H1 §9 item 3 or by phase 8's task 09 Terraform, each as amended — plan re-review b05 N1). If either is absent, first read the registration's `optionalClaims.accessToken` in its manifest: a missing entry there is the cause, not the Hub. Record the JSON **type and value** of `acct` (expected the number `0` for the owner, a member) and of `xms_edov` (expected a boolean), and that an invite for the owner's `email` binds at their `GET /v1/me` (`invitesBound: 1`). If a B2B guest test account is available, record its `acct` (expected `1`) and that an invite for its `email` does **not** bind unless its `xms_edov` is `true` | all present; `acct` a number, `xms_edov` a boolean; the owner's invite binds | ADR-H1 R9: `devissuer` diverging from Entra; review I12: binding gated on two optional claims whose presence and JSON types are known only from documentation — an absent or string-typed claim fails closed, and then no invite binds |
| R-5 | The refresh token's size in bytes, and so the chunk count | recorded | ADR-H1 R1 |
| R-6 | `cmdkey /list` shows the `PanaceaApp/entra/…` entries, persisted on this machine | present | `OD-6`'s store actually used |
| R-7 | After the access token expires (60–90 min), a Hub call still succeeds without a prompt | no prompt | refresh and rotation against Entra |
| R-8 | `Token(Sandboxes.Access)` succeeds from the same sign-in; then `Token` for Azure DevOps' scope, **whether or not** IT granted it (Q5). A refusal for either secondary scope is recorded with its `error`, `suberror` and `error_codes` (the AADSTS numbers) as Entra sent them, and after it `Status` is still `signed_in` and a Hub call still succeeds with no prompt (correction A12). If Azure DevOps answers 50076 or 50079 (or any `ErrScopeRefused`): sign out, sign in again with a fresh `BeginSignIn` (Hub-only, completing any MFA it asks for), and record whether the next `Token` for Azure DevOps still refuses and with which codes (re-review b04 N1). Then paste a PAT for the same organisation (R-11) and record that `LocalDevOps` serves it while the refusal stands | success; or `ErrScopeNeedsConsent` / `ErrScopeRefused` with the Hub session intact; and, for a 50076, "cleared by a fresh sign-in: yes/no" as observed | one refresh token for three resources (ADR-H1 §11); review I2's per-resource refusal bodies, known so far only from documentation, and a secondary refusal signing the user out of the Hub; the evidence the owner needs to rule on a per-resource step-up sign-in (task 31, "does not claim") |
| R-9 | The owner's token carries `roles: ["Hub.OrgAdmin"]`; `GET /v1/audit` answers 200 for them | as stated | the app-role assignment |
| R-10 | A colleague outside *PanaceaApp Users* cannot obtain a Hub token | Entra refuses | "assignment required" (ADR-H1 §9 item 4) |
| R-11 | A real Azure DevOps PAT, minted by the owner for one organisation, pasted through `POST /v1/credentials/azure-devops` (task 39); record its **length** and whether it is all `[A-Za-z0-9]` (never its value); then `credstore.LocalDevOps` serves it and a `git ls-remote` of a repository in that organisation, with `PANACEA_APP_ENTRA_DEVOPS` unset, succeeds through phase 3's chain | 204; listed by `GET /v1/credentials`; `ls-remote` exit 0 | review I10: task 39's PAT bound `^[A-Za-z0-9]{52,256}$` is from knowledge, and the local ADO path is the only one until Q5 |

#### The results record

`docs/verification/2026-09-23-phase-h-results.md` holds, in this order: the commit it was run at;
every acceptance command of tasks 01–42 **as run**, with its output's last line; the end-to-end
test's output; what the implementer observed in `npm run tauri dev` in tasks 38, 41 and 42 —
labelled **the implementer's observation**; the real-tenant checklist with its three empty
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
grep -c "^| R-" docs/verification/2026-09-23-phase-h-results.md                                                                                   # 11
grep -n "Unverified" docs/verification/2026-09-23-phase-h-results.md                                                                              # present
```

#### What this task does not claim

- **Anything the R-list covers**: until the owner's session, every R-item is unverified, and the
  record says so.
- That the owner has seen the UI: the implementer's observation is not acceptance (`CLAUDE.md`,
  working agreements).
- That phase 8's environment runs this: the chart is phase 8's, never applied (`OD-5`).
