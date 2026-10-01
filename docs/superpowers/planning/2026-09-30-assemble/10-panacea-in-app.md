# 10 - Panacea in the app: our own Panacea, and the authenticator

**Status:** PROPOSAL (2026-10-01), for the owner's sign-off on its cost before anything is built
(his standing rule: cost and scope first). Until he signs off, `00-plan.md` stays the plan of
record and none of its rows or totals change; its section 5.3 points here.
**Date:** 2026-10-01. **Question:** what `OD-53`'s "Panacea first-class inside the app" means
technically, what it costs, and how the "authenticator" for Azure DevOps and GitHub works.
**The owner, verbatim (2026-10-01, second round, through the in-session question form):** on
Connect deploy and P-3, *"Huh, maybe you have the wrong idea. I want Panacea to be ingrained into
this. I do not want Panacea to be this external site that is used for Deployments. I want it to be
first classed, inside the app, so everything is done within the app. No API key, nothing. We will
build our own Panacea as a part of this."* On the Azure DevOps token, *"Instead, could we build in
an authenticator, because people would authenticate through EntraID, could we have them SSO into
Github or ADO, and that automatically connect?"*
**Standing:** `OD-53` is decided. Its technical shape is ours to pick (`OD-37`: technical picks
are the orchestrator's, money and customer-visible calls the owner's), so section 0 picks one and
section 4 asks only what is his.
**Builds on, does not re-argue:** `00-plan.md` (rows, totals, §5.2's forced exceptions, §10),
`09-cohesion.md` (the one-app layer and its K1 to K14 walk), `00-panacea-verify.md` §2 to §4,
`06-deploy-verify.md` §2, `05-work-team-verify.md` §8, and the register through `OD-53`.
**Method:** five read-only scouts this round (Panacea's inventory, its user functions, identity for
Azure DevOps and GitHub, two Panacea estates side by side, and the deploy-layer options), with every
load-bearing figure re-checked at source. Panacea was read at `ado-gov/dev` `ca92e83eb` (API
v1.165.0, the line live on Panacea dev) with `git show`, `git grep` and `git ls-tree`, never checked
out or written; its cutover documents at `3d45587f0`. Two live reads, both anonymous: Azure DevOps'
answer for `kpmgcomau`, and the Azure Retail Prices API. Nothing was installed, built or run.
Section 5 lists what is not verified.

## 0. The answer

**Our own Panacea is Panacea's deploy side, lifted into this repo from the line live on Panacea dev
and run by us on a second cluster in Panacea's subscription. Every act a person or an operator does
on Panacea's Portal becomes a page in the app, and our sign-in is its only identity, so no key
exists anywhere. Today's Panacea and its users stay as they are until its move to Australia East
signs off; then the two become one Panacea in this repo. Azure DevOps and GitHub connect from one
Connections page in the app: background work needs nothing from anyone, GitHub needs one Authorize
click, once, and Azure DevOps acting as the person is silent after one IT consent. It adds about 91
engineer-days (70 to 120), so the plan becomes about 223 (190 to 270). With five builders the first
app moves from about 13 to 17 working days to about 18 to 23, and all of it from about 26 to 27 to
about 43 to 45. The second cluster costs about US$900 to 1,000 a month in machines at pilot size.
Nothing is built until the owner signs off this cost (section 4).**

1. **What "first-class inside the app" means: our pick.**
   - Deploying becomes one of the app's own capabilities, run by code in this repo on
     infrastructure we run. Panacea's deploy control plane, `panacea-api`, becomes ours and does
     what it does today: previews per branch, ship, promote, roll back, versions, evidence, tenant
     namespaces, databases, storage, secrets, the scan pipeline and its posture gate, cost codes
     and team statements.
   - Nobody sees it. Our pages reach it through our control plane, and agents through
     `panacea-mcp` behind our allowlist proxy (row 64), as `00-plan.md` already plans. Only its
     address and its credential change.
   - It trusts one identity, our realm, and mints no keys. Our control plane takes the place
     Panacea's Portal holds today on Panacea's existing verified-identity path
     (`middleware/auth.go:159-186`): the shared transport secret, plus the person's own short-lived
     token from our realm, checked against our realm's keys (`middleware/jwks.go:147-153`), held
     server-side and never in a sandbox. Key minting is switched off. In our copy that is
     configuration plus a few small changes we own (row 112), not new code in today's Panacea
     queued behind its freeze.
   - Previews sign in against our realm too, so the realm hand-off (row 95) gives them a session
     inside the frame, `protected` previews included.
   - **Not taken:** driving today's Panacea out of sight. It hides the outside site instead of
     removing it, builds no Panacea of ours, and "no key" would still wait on new sign-in code in
     today's Panacea behind its cutover freeze (`06-deploy-verify.md` §2.4).

2. **What we lift, and from where.**
   - **From** `ado-gov/dev` at one pinned commit, today `ca92e83eb` (API v1.165.0), copied whole
     under `OD-50`'s attribution headers and ledger (row 72), the scope `OD-53` left to this file,
     and re-synced weekly after it (row 119).
   - **What:** `panacea-api` with `panacea-shared` and `services/contracts`; `panacea-build` and its
     step images; `webhook-receiver`; the static gateway; `panacea-mcp`; the Tekton pipelines, tasks
     and scanners; `kubernetes/base` and the live Helm values; and the live deploy engine,
     `deploy/`, with `deploy/*.tf` only, run under OpenTofu. About 2,000 files; `panacea-api` alone
     is 177,086 lines of Go and 169,839 of tests.
   - **Left out:** the Portal (Backstage), Forge (our cockpit replaces it), Apothecary, marketing,
     the docs site, the Argo CD app-of-apps, `local/`, `deploy/cutover/`, and `terraform/`, which
     is never copied or applied (`CLAUDE.md`'s gotcha).
   - **Swapped as it lands**, so nothing on the forbidden licence list enters the tree: `madmin-go`
     (AGPL-3.0, `go.mod:9`) removed; Redis 7.4 (RSALv2/SSPLv1) replaced by Valkey 8 for tenant
     caches (`clients/tenant_redis.go:70`); Vault (BUSL-1.1) replaced by OpenBao (`OD-32`); SAST run
     on rules baked into its image, which also ends the false pass it records today
     (`00-panacea-verify.md` §2 row 11).

3. **What our in-app surfaces replace.**
   - **Panacea's Portal, all of it, for our Panacea:** every user act on its 21 project tabs and
     its team, onboarding, account and Pages screens (rows 107 to 109, already planned, and rows 121
     to 129), and its operator console (rows 130 to 132).
   - **Forced exceptions 2, 3 and 4 close** (`00-plan.md` §5.2). No Connect deploy and no key. No
     step-up exit to the Portal: the step-up map is now ours, unmapped in v1 as Panacea ships it,
     with production approval on our Deploy page (row 109). The Azure DevOps token gives way to the
     authenticator (item 6).
   - **Two things stay off the screens, both named** (`OD-49`). Until the operator console lands,
     after the first app, the few operator acts the pilot needs are done by the build team, and no
     user waits on one. The Panacea functions with no Portal screen today (team baselines and
     templates, team git tokens, integrations, policy constraints, compliance and egress views)
     stay reachable by agents only, as now, with their pages later (section 1.7).

4. **Where it runs.**
   - **On its own AKS cluster, the apps cluster,** in a new resource group in Panacea's
     subscription, in Australia East, stood up by Panacea's own new-environment engine
     (`deploy/panacea-deploy.sh`) with a fourth env file. Same bill and same cost codes as today
     (`OD-47`); every resource name distinct from every Panacea estate's (`OD-42`).
   - **Never on the sandbox cluster:** `panacea-api` holds rights over every tenant's secrets and
     runs the apps agents write, and `OD-42` keeps tenant workloads off the agents' cluster.
   - **Its data plane is the managed one** Panacea's own move to Australia East is built on
     (`ADR-029`: Flexible Server, and Blob behind an S3 gateway,
     `deploy/environments/panacea-aue.env:79-87`), with no MinIO.
   - **Its edge admits KPMG's network only** (call 13). Apps get `<app>.apps.panacea.kpmg` if the
     owner agrees (question 3). Our control plane reaches it over a private peering, never its
     public edge.

5. **Existing Panacea users, and the PROD cutover.**
   - **They stay where they are.** Their projects, apps and website are untouched. Apps made in the
     app go to ours. Teams, members and cost centres are copied across once (row 120), so a team
     looks the same in both.
   - **Nothing is written to today's Panacea** before its production move signs off, beyond what
     `OD-44` already asks (one block of realm settings, and our egress address on its edge for
     that realm) and one record delegating the apps zone, written outside the move's windows
     (section 3.1).
   - **After sign-off, the two become one Panacea in this repo** (`OD-13`'s lift, completed):
     today's estates deploy from here, older apps appear in the app, and the Portal retires on the
     owner's date. It is sized then (section 1.7).
   - **`OD-44` stands, as the register asked this file to check.** Its edge from `OD-13` still
     holds: today's Panacea stays a separate running system until that merge, and its realm
     changes only configuration.

6. **The authenticator for Azure DevOps and GitHub.** One sign-in, plus service identities, with
   each person's own tokens fetched on their laptop. Neither GitHub nor Azure DevOps accepts our
   sign-in's token. GitHub has no exchange from a Microsoft sign-in. KPMG's Azure DevOps
   organisation, `kpmgcomau`, answers from Microsoft tenant `eb9a11c9…` (read 2026-10-01), while
   people sign in to tenant `deff24bb…`, where Panacea's Microsoft app lives
   (`deploy/environments/panacea-dev.env:141`, `:153`). Every connection shows on one Connections
   page, with its state, reconnect and revoke.

   | Connection | How it connects | Automatic? | One-time step | Needs IT? |
   |---|---|---|---|---|
   | GitHub, background (task sync, agents reading code, builds) | Our GitHub App's installation tokens | Yes | An owner of the `kpmg-au-emu` organisation installs the App | No |
   | GitHub, as the person (reviews, starting from an issue) | The App's user authorisation, in the first-run sign-in tab | Yes, refreshed for 6 months at a time | One "Authorize" click, once | No |
   | Azure DevOps, background | Our managed identity, in the same tenant as `kpmgcomau` | Yes | Each organisation's admin adds it once | No |
   | Azure DevOps, as the person (reviews, comments, tickets) | Windows' account broker, silently, from a Panacea app in tenant `eb9a11c9…` | Yes, once consented | None | **Yes: one admin consent in that tenant** (question 4) |
   | Azure DevOps, as the person, until that consent | The laptop's own Git Credential Manager sign-in, through phase 3's git helper; a pasted token, on our page, only if that fails as well | If it works on KPMG laptops (not verified) | None | No |
   | Deploying | Our realm's token, server-side | Yes | None | No |

7. **What it costs.**
   - **About 91 engineer-days net** (the rows' own lows and highs sum to 59 and 142; taken as 70 to
     120): about 54 for our own Panacea (the lift, the estate, its identity, the deploy path, the
     swaps, the proofs, the weekly re-sync and the operator console, less the 1 day that rows 70,
     71 and 73 give back); about 9 for the authenticator; and about 28 for in-app pages for
     Panacea's user screens, which `OD-49` required whatever `OD-53` said. `00-plan.md` §5.2's
     verdict on the Portal, "Pass, with the exits named", does not hold: the plan brings about 90
     of Panacea's 357 user routes into the app.
   - **The plan becomes about 223 engineer-days (190 to 270).** Done falls from about 11% to about
     7%. By capability, open source falls from 35% to 28% and ours rises from 37% to 42%; by
     effort, ours stays at 61% (section 2).
   - **Run cost:** about US$900 to 1,000 a month in machines at pilot size, and roughly US$1,400 to
     2,000 with the database, registry, logs and networking (inferred, not priced).

## 1. The costed rows

### 1.1 How to read them

Numbering continues `00-plan.md` §1 (rows 1 to 109, and 76b). The Kind rule is that section's:
**OSS**, **Panacea** (Panacea's running service or its copied code), **Managed**, **Ours**; code we
write is Ours even when it wraps an adopted piece. Under it, lifting, configuring and running
Panacea's code is glue on Panacea, and a capability no product provides is Ours even when it is
written inside our copy (rows 113, 120). Days are engineer-days, low, mid and high; the mid figure
is the one carried into every sum, as `00-plan.md` carries one figure a row. Panacea paths are at
`ado-gov/dev` `ca92e83eb`, with `services/panacea-api/` left off the API's paths, unless marked.

### 1.2 Rows `00-plan.md` carries, changed

| Row | In `00-plan.md` | Under this proposal | Days before | Days after | Kind |
|---|---|---|---|---|---|
| 57 | Deploy engine: Panacea's API on `dev.panacea.kpmg` | Our own Panacea's API, lifted (row 110), on our apps estate (row 111) | 0 | 0 | Panacea |
| 58 | Shipped-app backend on CloudNativePG, Vault, Knative, Flagsmith, MinIO | Flexible Server (shared and dedicated tiers), Blob behind an S3 gateway, OpenBao, Flagsmith, Valkey. Knative stays off, as Panacea's engine leaves it by default (`deploy/cloud-shell-deploy.sh:6282-6288`), so edge functions wait (section 1.7) | 0 | 0 | Panacea |
| 59 | Build pipeline, scans and posture gate, run by Panacea | Ours to run (row 114); SAST on baked rules ends its false pass | 0 | 0 | Panacea |
| 60, 68 | `panacea-mcp`: the agent deploy loop, and a copy in every sandbox image | Unchanged, pointed at our Panacea through G4 | 0; 0.25 | 0; 0.25 | Panacea |
| 61, 62, 65, 66 | G1 Deploy page; G2 provenance; G5 project create; phase 7's connector | Unchanged, over our Panacea's API. The connector's address and credential change inside row 112 | 2.75; 1.25; 0.875; 1.7 | the same | Ours; OSS; Panacea; Ours |
| 63 | G3 promotion by data class; the class-mismatch refusal not costed | Unchanged; the refusal is coded in our copy (row 113) | 0.75 | 0.75 | Panacea |
| 64 | G4 allowlist proxy, holding the user's `pnc_` | Unchanged in days. It presents the person's realm token, server-side, and holds no key | 1.5 | 1.5 | Ours |
| 67 | Chains and Kyverno, audit only; Chains signs only while Vault is unsealed | Ours to run. Chains signs through OpenBao with Key Vault auto-unseal, so a restart no longer stops signing. Still audit only in v1 | 0 | 0 | Panacea |
| 70 | P5: six asks of Panacea's stream | Only `OD-44`'s realm block and our egress address are asked of today's Panacea; nothing at all if question 4's line (a) lands | 0.5 | 0.25 | Panacea |
| 71 | P6: live proof on Panacea dev | **Retired.** The proof runs on our estate (row 117) | 0.5 | - | Panacea |
| 73 | Panacea's steps in the cockpit: team-request state, Connect deploy, our error copy | Connect deploy goes; the team-request state and our error copy stay | 0.5 | 0.25 | Ours |
| 78 | Our environment, "OpenTofu from Panacea's `.tf`" | Unchanged. "Panacea's `.tf`" means `deploy/*.tf` only, never `terraform/` | 1 | 1 | Managed |
| 81, 82 | Panacea's realm as the identity source; our Keycloak brokering it | Unchanged under `OD-44`. Our realm gains the deploy audience, the account our Panacea manages groups through, and a second realm for shipped apps' own users, all inside row 112 | 0; 2 | 0; 2 | Panacea; OSS |
| 95, 96 | The realm hand-off; share-link previews, with no end condition | Unchanged in days. The hand-off now gives our previews a session, `protected` ones included; share links stay, for showing a preview to someone who has no session | 1.5; 0.25 | the same | Ours; Panacea |
| 104, 105 | The provider page; cost codes sent to Panacea | Unchanged, over our Panacea. Shipped apps' AI joins the provider page (row 132) | 2.5; 1.75 | the same | Ours |
| 106 | In-app PR review | Unchanged. It reads with the person's own GitHub and Azure DevOps tokens from the authenticator | 3 | 3 | Ours |
| 107, 108, 109 | App settings; team settings and approvals; production approval, over Panacea's API | Unchanged, over our Panacea's API. Row 109's step-up gate is now our own setting | 4.5; 1.5; 0.5 | the same | Ours |

**Net of the changed rows: -1 day** (rows 70, 71 and 73). Row 71 leaves the row count.

### 1.3 New rows: our own Panacea

| # | Capability | Replaces | Kind | Low | Mid | High | Basis |
|---|---|---|---|---|---|---|---|
| 110 | **The lift.** The trees of section 0 item 2, from `ado-gov/dev` at one pinned commit. Attribution headers by script, with a ledger-only rule for files that cannot carry one (JSON, golden files, fixtures); one ledger row per tree with its source commit; a re-sync script that re-applies the headers, so the weekly pull stays mechanical. The modules build outside our `go.work`, at their own pins (Go 1.25 and `client-go` v0.29.3, `go.mod:3`, `:14-16`, against our Go 1.23 and v0.31.1, `go.work:1`, `control-plane/go.mod:10`), into our registry, with CI running their tests, including the three suites Panacea's CI never ran (the static gateway, the analytics injector, the pipelines) | the source of rows 57 and 68; extends row 72 | Panacea | 1.5 | 2.5 | 4 | 1,993 files in these trees at `ca92e83eb` (`git ls-tree`). Scouts: 1.5 / 3 / 5.5 (inventory, snapshot plus build), 1 / 1.5 / 2 (options) |
| 111 | **Our apps estate.** A second AKS cluster in its own resource group, Australia East, by Panacea's engine with a fourth env file and a new `ESTATE` and `ENVIRONMENT` value no Panacea estate uses: its own registry, Key Vault and OpenTofu state; the managed data plane; the engine's Keycloak, Portal, Forge and LiteLLM phases skipped (Keycloak is phase 12, `deploy/cloud-shell-deploy.sh:5829`); its fail-closed estate guard kept (`:56-66`: with no env file every default is a production value, `:45-54`); the VNet range made a variable (every Panacea estate is `10.1.0.0/16`, `deploy/main.tf:52-57`) and privately peered with our platform's; a delegated subzone with wildcard certificates by DNS-01; the 14-address edge (`deploy/variables.tf:278-297`); Argo CD's applications and the registry name pointed at ours (the name is rewritten at apply, `cloud-shell-deploy.sh:212-213`); the runtime scanners as the engine runs them on dev (phase 15.5, `:6319`); the parent zone's `_panacea_oauth2` cookie stripped before any app an agent wrote; pilot-sized pools with build machines from zero; up to a green lifecycle canary | the runtime of rows 57 to 59 | Panacea | 6 | 9 | 16 | Panacea's own dev estate on this engine took eight bootstraps, three finalize runs and about a dozen fixes over three days to first light (2026-08-17 to 08-19, the owner's session notes), then estate fixes for five more weeks. This shape has never run: no realm, no Portal, OpenBao, the managed plane, our pipeline. Scouts: 10 / 18 / 30 (inventory), 4 / 5 / 7 (options) |
| 112 | **No key: our realm is our Panacea's only identity.** The verifier's JWKS address, issuer and audience (`middleware/jwks.go:147-153`) set to our realm and a `panacea-deploy` audience; its unverified-decode grace set to zero and its no-JWKS fail-open closed (`jwks.go:44-68` describes both); our control plane in the Portal's slot (`auth.go:159-186`), with the person's token held server-side; key minting unrouted (`routes/routes.go:2984-2986`, `:3092-3094`); the Keycloak admin client moved from the master-realm password grant (`clients/keycloak.go:35-45`, `:70-77`) to a service account scoped to our realm, where teams and project groups now live; `groups` on the `panacea-deploy` audience only, named as Panacea's code expects (`middleware/entra_claims.go:159-162`), and the username taken from the email (`auth.go:604-641`); shipped apps' own end users in a second realm (today they share Panacea's, `handlers/project_auth.go:68-75`); KPMG literals moved to configuration (the `@kpmg.com.au`-only registration gate, `handlers/register.go:41-42`; three named approvers, `handlers/team_create_approval.go:58-60`); previews' sign-in pointed at our realm with issuer checking on (`kubernetes/helm-values/oauth2-proxy-values.yaml:79-81`); a security review | forced exception 3 (Connect deploy); P-3; the legacy-key guard | Panacea | 3 | 4.5 | 7 | Scouts: 4.5 / 7.5 / 12 (inventory items 3 and 6), 2 / 2.5 / 4 (options). P-3 alone was judged 2 to 3 of Panacea's days with its review (`00-panacea-verify.md` §4.1) |
| 113 | **Panacea's pending asks, now changes in our own copy:** P-1, the brief pin on every release of an app-made project, and a refusal of a deploy that carries none; P-2, the data-class route; and the refusal to promote a project whose class annotation differs from its brief's (a named gap of `00-plan.md` §1) | P-1 and P-2 (day 0 item 3); the named gap | Ours | 1.25 | 1.75 | 2.5 | `06-deploy.md:127-128` (P-1 0.5 to 1, P-2 about 0.5); the refusal 0.25 (coexistence scout) |
| 114 | **The deploy path on our estate.** `panacea-build` and its step images, built into our registry; Tekton Pipelines and Triggers pinned (installed from `latest` today, `cloud-shell-deploy.sh:168-169`, `:5435-5451`); about 20 tool and scanner images mirrored; SAST on rules baked into its image, with a real error path (`pipelines/tasks/sast-scan.yaml:79`, `:118-128`); `webhook-receiver`, with pushes reaching it through Panacea's own deploy poller (`DEPLOY_POLLER_ENABLED`, `cloud-shell-deploy.sh:69-71`), because a 14-address edge admits no GitHub or Azure DevOps delivery; the static gateway on Blob; build-deploy, preview-deploy and every scanner proven in warn mode | the runtime of row 59 | Panacea | 3.5 | 6 | 11 | Inventory items 7 to 10: 4 / 7.25 / 13.5. Sizes at `ca92e83eb`: `panacea-build` 11,536 Go lines, pipelines 21,025 YAML lines, `webhook-receiver` 4,206, the static gateway 1,232 |
| 115 | **One AI gateway for agents and shipped apps.** `panacea-api`'s per-project and per-branch model keys minted against our gateway (rows 76, 76b) instead of the LiteLLM Panacea builds today (`main-v1.40.0`, `cloud-shell-deploy.sh:709-710`), with the project's codes on each key (row 105) | - | Panacea | 1 | 1.5 | 2.5 | Inventory item 5: 1 / 2 / 3 |
| 116 | **The licence swaps inside the lift.** OpenBao for Vault (`OD-32`): KV v2, Transit for Chains' signer, Key Vault auto-unseal, under the engine's Vault steps (phase 11.5, `cloud-shell-deploy.sh:5647`); Valkey 8 for tenant caches through the existing override (`PANACEA_REDIS_IMAGE`, `clients/tenant_redis.go:65-70`); `madmin-go` removed, its 14 uses in `clients/kubernetes.go`, `minio_admin.go` and `managed_object_storage.go` deleted or replaced, since the managed plane never reaches MinIO; OpenTofu running `deploy/*.tf` | - | OSS | 1.5 | 2.5 | 4 | Options scout: 1 / 1.5 / 2.5 for `OD-32` inside the lift; plus the AGPL and Redis swaps (inventory item 4, in part) |
| 117 | **Live proof on our estate.** Panacea's own acceptance, lifecycle and onboarding suites (`deploy/acceptance/run-acceptance.sh`, `run-lifecycle.sh`, `run-onboarding-access.sh`; wired in `azure-pipelines-dev.yml:61-63`), run from our pipeline before the first app's walk | row 71 | Panacea | 1.5 | 2.5 | 4 | Inventory item 13. Panacea dev's lifecycle canary last ran 17 pass, 3 warn, 0 fail (`docs/cutover/RESUME.md:6-7` at `3d45587f0`) |
| 118 | **Ops readiness.** Certificate-expiry, OpenBao-seal, backup and quota alerts on the estate's own monitoring, and runbooks. Panacea production's certificate renewals failed silently for about six weeks before a break-glass fix on 2026-09-28 (`06-deploy-verify.md` §2.4) | - | Panacea | 1 | 1.5 | 3 | Inventory item 14 |
| 119 | **The weekly re-sync during the build.** Panacea's fixes pulled from `ado-gov/dev` into our copy about once a week, for about eight weeks. Isolation fixes were still landing this week (`06-deploy-verify.md` §2.4), and `ado-gov/dev` took 679 commits in 30 days (inventory scout) | - | Panacea | 2 | 4 | 8 | 0.25 / 0.5 / 1 a week (the options scout's rate; the inventory's is 0.5 / 1 / 2) for 8 weeks |
| 120 | **Teams copied once at go-live.** Teams, members and cost centres read once from today's Panacea's team API into ours, so a team and its cost centre look the same in both (question 2) | - | Ours | 0.5 | 1 | 2 | Inventory scout |

**Sum of rows 110 to 120: 22.75 / 36.75 / 64.**

### 1.4 New rows: Panacea's screens in the app

**Why these rows exist.** The user-functions scout classified all 590 of Panacea's routes (586 in
`routes/routes.go`, 4 in `routes/mcp_wiring.go:60-63`) at Panacea's HEAD `2836cee91`: 357
user-facing, 169 platform-admin, 5 Forge, 14 agent docs, 45 machine-only (`ca92e83eb` registers 587
in `routes.go`). The Portal's project view has 21 tabs, six of them its default "Basic" view
(overview, deployments, service, risk, cost, settings; `ProjectViewPage.tsx:176-185`, the scout's
read). `00-plan.md` brings about 90 of the 357 into the app and covers 7 tabs in full, 5 in part and
8 not at all; the marketplace tab is switched off. Three of the six Basic tabs (service, risk, cost)
have no full page in the plan, and runtime logs have none at all: phase 7 left them out
(`feat/phase-7-deploy:docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/task-12.md:162-163`).
**Under `OD-49` these pages were owed whatever `OD-53` said**; with no Portal in our Panacea, they
are also the only way to reach these functions at all.

**How they are costed.** At row 107's rate (about 0.25 to 0.5 a settings area), reusing the Portal
panels' logic under `OD-50`, re-rendered on our stack in the shared Hub page template (row 88); the
Portal itself is never embedded. Group numbers are the scout's. Domains (group 8) need nothing
beyond row 107, and keys (group 15) need nothing at all, because none exist.

| # | Capability | The Portal's groups | Kind | Low | Mid | High |
|---|---|---|---|---|---|---|
| 121 | **Project and branch pages:** project settings and delete, visibility, favicon, PWA, webhook, seed admin and the create options (tier, database, storage, framework); branch archive, pause, primary and delete; repair environment, and quota in the diagnosis; the preview list with its reap window, share-link list and revoke; set stage and the stage floor; per-branch tier and replicas, usage advice, the elastic range | 1, 2, 3, 4, 14, 19 | Ours | 2.45 | 4.25 | 6.25 |
| 122 | **Data pages:** the Data page (tables, an SQL editor with results, saved queries and history, schema and types, resize, the shared-to-dedicated upgrade, a database added later); buckets and an object browser; the platform-injected variables; per-database and per-bucket restore | 5, 6, 7, 18 | Ours | 3 | 4.85 | 7.5 |
| 123 | **Security and risk pages:** the app-access policy, visitor sessions, bulk app users, reset and verify mails; a per-branch posture editor, advisory acknowledgements, run scans now, runtime security; the risk assessment, a Basic tab that gates production deploys of governed projects, with its requirements, documents and PDF | 9, 10, 11 | Ours | 3.25 | 5.25 | 7.25 |
| 124 | **Cost and team pages:** the project's cost (month to date, per branch, budget, elastic range); the team's overview and projects, forced presets and section visibility, statements and bill history, running cost, subdomain, and invitation acceptance inside the app | 12, 13 | Ours | 2.25 | 3.25 | 4.75 |
| 125 | **Runtime pages:** runtime logs (tail, follow, search, export), the audit trail and its export, the latency card, notification channels, analytics; shipped apps' AI (model, usage, call trail, web search, key regenerate, rate cap); cron jobs. Edge functions wait (section 1.7) | 16, 17, 20 in part | Ours | 4.25 | 5.75 | 8.25 |
| 126 | **The rest:** repository connection status with no token field; Pages (quick static publishing); the Collection Register; help inside the app (the docs site is separate today); "your access" | 21, 22, 23 | Ours | 1.5 | 2.1 | 3.25 |
| 127 | **One allowlisted, audited pass-through** from Hub pages to our Panacea's API: identity added server-side, SSE flushed, errors in our copy. It shares G4's proxy (row 64), so about 250 routes need no connector each | - | Ours | 0.75 | 1 | 1.5 |
| 128 | **The project area and its home**, honouring a team's hidden and locked sections | - | Ours | 0.75 | 1 | 1.5 |
| 129 | **Journey test and copy review** for the new pages (extends rows 86 and 101) | - | Ours | 0.5 | 0.75 | 1 |
| 130 | **Operator console, part 1:** teams administration; platform security and governance (the edge allow-list, read-only mode, secret rotation, the posture baseline, constraints, stage bundles, the step-up map, image admission, compliance controls); audit, the SIEM feed, AI calls and deploy-failure triage; cost, Azure actuals, P&L, FX and markup, statements and the billing export | the Portal's admin areas A2, A3, A5, A7 | Ours | 6.25 | 8.75 | 12 |
| 131 | **Operator console, part 2:** users, invitations and their email, sessions, MFA, identity providers, revoke-all; build and feature settings; resources and operations (a read-only cluster view, maintenance sweeps, backfills, forced scans, rollouts, flags, platform connections); organisation risk templates | A1, A4, A6, A9 | Ours | 5 | 7.25 | 10 |
| 132 | **Shipped apps' AI on the provider page:** pricing, model slots, usage, revenue and budget, beside row 104's providers and keys, so every key stays on one page (`OD-46`) | A8 | Ours | 1 | 1.75 | 2.5 |

Bases: the user-functions scout's day figures for each group (rows 121 to 131), with row 125's cron
jobs taken at 0.5 / 0.75 / 1.25 of its 1 / 1.5 / 2 for jobs and edge functions; row 132 lowered
from the scout's 1.5 / 2.25 / 3 because row 104 already holds the providers and keys. The operator
console's 146 routes are off every person's Define-to-Deploy path, so they come after the first
app (section 3.3). **Sums: rows 121 to 129, 18.7 / 28.2 / 41.25; rows 130 to 132, 12.25 / 17.75 /
24.5.**

### 1.5 New rows: the authenticator

| # | Capability | Replaces | Kind | Low | Mid | High | Basis |
|---|---|---|---|---|---|---|---|
| 133 | **Connections: one service and one page** for every outside account: its state, connect, reconnect and revoke; each Azure DevOps organisation's tenant read from its `X-VSS-ResourceTenant` answer; the sources tried in a fixed order | forced exception 2's paste page | Ours | 1 | 1.5 | 2 | Identity scout. It builds on Hub task 29's credential store and row 88's template |
| 134 | **GitHub as the person:** our App's user authorisation, chained into the first-run tab (one "Authorize" click, once); 8-hour tokens with a rotating 6-month refresh, kept in Windows Credential Manager (`OD-6`). It replaces the Hub plan's pasted GitHub token (`feat/hub:docs/superpowers/plans/2026-09-23-phase-h-hub.md:566`, Q9; tasks 29 and 39 at `:432`, `:442`) | the Hub's pasted GitHub token | Managed | 0.75 | 1 | 1.5 | GitHub's documented web flow and refresh lifetimes, as the identity scout read them. Background installation tokens are already rows 45, 46 and 49 |
| 135 | **Azure DevOps background identity:** our managed identity, which sits in tenant `eb9a11c9…` because Panacea's subscription does, the same tenant `kpmgcomau` answers from; added once to each organisation, with Basic access so it can read code. Tracker sync, the sandbox read proxy and builds use it, never a person's token | the PAT the read proxy held (`05-work-team-verify.md` §8 piece 3) | Managed | 0.5 | 1 | 1.5 | Identity scout. `05-work-team-verify.md` §8 already asks ADO admins to add a managed identity |
| 136 | **Azure DevOps as the person:** a silent token from Windows' account broker in our build's main process, on a Panacea app in tenant `eb9a11c9…` holding Azure DevOps' delegated permission, consented once by an admin (question 4, line b). Until then, phase 3's planned git helper asks Git Credential Manager for a Microsoft token on each call (`feat/phase-3-git:docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-13.md:97-116`). A pasted token, on our page, only if both fail | forced exception 2 | Ours | 1.75 | 2.5 | 4 | Identity scout: the bridge 0.25 / 0.5 / 1, the broker path 1.5 / 2 / 3. `00-plan.md` §5.2 gave an in-app token flow about 1 day once consent exists |
| 137 | **Keyless builds:** every build of our Panacea clones with a credential minted for that run, a GitHub App installation token or our managed identity's token, never a stored one. Today a build clones with a stored token (`pipelines/tasks/git-clone.yaml:47-49`), set per project, per team or per person (`routes.go:1321-1323`, `:2028-2033`, `:3061-3069`) | Panacea's stored git tokens | Ours | 2 | 3 | 4.5 | Coexistence scout 2 / 3 / 4, user-functions scout 2.5 / 3.5 / 5; counted once, here |

**Sum of rows 133 to 137: 6 / 9 / 13.5.**

### 1.6 The forced exceptions and the named gaps

| `00-plan.md` §5.2 | What happens to it | Rows |
|---|---|---|
| 1. The sign-in tab, on first run and weekly | Accepted by the owner, and unchanged. It now also carries GitHub's one Authorize click, once | 134 |
| 2. An Azure DevOps token, pasted once a person | Gone. A paste survives only as the last fallback, if both the consent and the bridge fail, and is then put to the owner's eyes | 133, 135, 136, 137 |
| 3. Connect deploy, in the system browser | Gone: no key exists to mint or paste | 112 |
| 4. Step-up acts on the Portal | Gone: there is no Portal, and the step-up map is ours | 112, 109, 130 |

**Named gaps of `00-plan.md` §1.** Closed: the Deploy refusal of a class mismatch (row 113), and a
`protected` preview inside the frame, by design (rows 95 and 112; not verified). Unchanged: the
preview's branch-data mode, where the managed plane's clone path was not traced; and a repository
and a tracker for a person who starts from nothing. **Two of Panacea's own gaps become ours, unsized:**
release signing stays audit-only in v1 (row 67), and the production rung still waits on DLP
enforcing for the project, as it does in Panacea (`06-deploy.md`, P-5).

### 1.7 Outside the total

- **The merge into one Panacea**, after Panacea's production move signs off: its history imported,
  its pipelines and Argo CD sources re-pointed here, the 09-28 CoreDNS hotfix carried first (Argo
  CD reverts it otherwise), today's estates deploying from this repo, the app fronting them, and
  the Portal retired on the owner's date. The source move alone is about 3 to 6 days (coexistence
  scout); the rest is sized when the move signs off, which has no date yet.
- **The weekly re-sync after the build**, until the merge: about 0.5 engineer-day a week (0.25 to
  1).
- **Our realm brokering Microsoft directly** (question 4, line a): about 0.5 (0.25 to 1), if the
  owner agrees and IT acts.
- **The Windows-native sign-in that removes the weekly tab:** about 2 (1.5 to 3), only if its day-1
  spike passes. It needs lines (a) and (c): while our realm brokers Panacea's, a Windows token has
  no linked provider to grant against (identity scout).
- **A production estate** beside the pilot's, before the pilot widens past class `none`: a fifth
  env file and about the same running cost again; not sized.
- **Later, with no live feature lost** (none of these runs on Panacea dev today, or none has a
  Portal screen there): edge functions and scale-from-zero (Knative and KEDA, not installed by
  default; 1 to 4); the two CLIs merged (`OD-43`; 1 to 3); app templates, the Portal's scaffolder
  starters, if no one starts from them today (not checked; 0.25 to 1); Presidio's AI
  DLP (off by default); Azure DevOps PR previews in our copy (call 13 keeps previews GitHub-only in
  v1); pages for what agents reach today with no Portal screen (team governance, project
  constraints, compliance and egress views, integrations: about 6.5, 4.5 to 8.75); Apothecary
  (switched off; not sized).

### 1.8 The sum

| Part | Low | Mid | High |
|---|---|---|---|
| Rows 110 to 120, our own Panacea | 22.75 | 36.75 | 64 |
| Rows 121 to 129, Panacea's user screens | 18.7 | 28.2 | 41.25 |
| Rows 130 to 132, the operator console and shipped apps' AI | 12.25 | 17.75 | 24.5 |
| Rows 133 to 137, the authenticator | 6 | 9 | 13.5 |
| Changed rows 70, 71, 73 | -1 | -1 | -1 |
| **Net change** | **58.7** | **90.7** | **142.25** |
| **The plan** (132.275 before) | | **222.975, about 223** | |

**By cause, mid:** our own Panacea 53.5 (rows 110 to 120 and 130 to 132, less the 1 back); the
authenticator 9; Panacea's user screens owed under `OD-49` 28.2. **Range:** summing every row's
low and every row's high (58.7 and 142.25) assumes every estimate errs the same way. The estate
(row 111) and the pages are the least certain, so the net is taken as **70 to 120**, which with
`00-plan.md`'s 120 to 150 gives **about 190 to 270**.

## 2. The effect on the totals

### 2.1 Before and after

| Measure | Before (`00-plan.md`) | After (this proposal) |
|---|---|---|
| Remaining work | 132.275 engineer-days (120 to 150) | 222.975 (about 190 to 270) |
| Capability rows | 110 | 137 (28 new, row 71 retired) |
| Done | about 11% (10 to 14) | about 7% (6 to 9) |
| By capability: open source / Panacea / managed / ours | 35% / 15% / 13% / 37% | 28% / 18% / 12% / 42% |
| By capability: adopted, together | 63% | 58% |
| By effort: glue on open source / Panacea / managed; ours | 24% / 4% / 11%; 61% | 15% / 17% / 7%; 61% |
| By effort: glue on adopted pieces, together | 39% | 39% |
| First app, five builders | about 13 to 17 working days | about 18 to 23 |
| All of it, five builders | about 26 to 27 working days | about 43 to 45 |
| With a sixth builder for the first three weeks | - | first app about 15 to 19; all of it about 40 to 42 |
| First app, two builders | about 5.5 to 6.5 weeks | about 8 to 9 weeks |
| All of it, two builders | about 13 weeks | about 22 weeks |
| New running cost | none | about US$900 to 1,000 a month in machines at pilot size; roughly US$1,400 to 2,000 all-in (inferred) |

### 2.2 Percent done: about 7%

**Method,** as `00-plan.md` §2.1: banked days over banked plus remaining. Nothing has been built
since that file, so the banked days stay 16.5.

**Arithmetic.** 16.5 / (16.5 + 222.975) = 16.5 / 239.475 = **6.9%, about 7%.** Phase 2's backend
at half (2.5): 14 / 236.975 = 5.9%; at double (10): 21.5 / 244.475 = 8.8%. Hence 6 to 9%. Done
falls from 11.1% only because the remaining work grows by 90.7.

### 2.3 By capability: 137 rows

| Kind | Rows | Share |
|---|---|---|
| Open source | 39 | **28%** |
| Panacea | 24 | **18%** |
| Managed service | 16 | **12%** |
| **Adopted, together** | **79** | **58%** |
| **Ours** | **58** | **42%** |
| Total | 137 | 100% |

**Arithmetic.** Before: 38 / 17 / 14 / 41 of 110. Row 71 leaves (Panacea, 17 to 16). The 28 new
rows are 1 open source (116), 8 Panacea (110, 111, 112, 114, 115, 117, 118, 119), 2 managed (134,
135) and 17 ours (113, 120, 121 to 133, 136, 137). After: 39 / 24 / 16 / 58 of 137. 39/137 =
28.5%, 24/137 = 17.5%, 16/137 = 11.7%, 58/137 = 42.3%. Rounded by largest remainder so the four add
to 100: floors 28, 17, 11, 42 = 98; the two largest remainders, 0.68 and 0.52, go to managed
services and Panacea. Adopted is 79/137 = 57.7%.

**Challenge it here:** the count depends on how finely capabilities are cut. The user screens are
cut into six rows and the operator console into two. Cut one row per Portal group instead, about 15
more rows of ours, and ours would be about 48% (73 of 152).

### 2.4 By remaining effort: 222.975 days

| Where the days go | Days | Share |
|---|---|---|
| Glue on open-source pieces | 34.15 | **15%** |
| Glue on Panacea | 36.625 | **17%** |
| Glue on managed services | 16.5 | **7%** |
| **Glue on adopted pieces, together** | **87.275** | **39%** |
| **Our own code** | **135.7** | **61%** |
| Total | 222.975 | 100% |

**Arithmetic.** Before: 31.65 / 5.875 / 14.5 / 80.25 of 132.275. Open source +2.5 (row 116), to
34.15. Panacea -0.75 (rows 70 and 71) and +31.5 (rows 110, 111, 112, 114, 115, 117, 118, 119: 2.5 +
9 + 4.5 + 6 + 1.5 + 2.5 + 1.5 + 4), to 36.625. Managed +2 (rows 134 and 135), to 16.5. Ours -0.25
(row 73) and +55.7 (row 113, 1.75; row 120, 1; rows 121 to 129, 28.2; rows 130 to 132, 17.75; row
133, 1.5; row 136, 2.5; row 137, 3), to 135.7. Shares: 34.15/222.975 = 15.3%, 36.625/222.975 =
16.4%, 16.5/222.975 = 7.4%, 135.7/222.975 = 60.9%. Rounded by largest remainder: floors 15, 16, 7,
60 = 98; the two largest remainders, 0.86 and 0.43, go to ours and Panacea. Adopted is
87.275/222.975 = 39.1%. Open-source glue by product: `00-plan.md` §2.2's list, plus OpenBao, Valkey
and OpenTofu, 2.5.

**What moved.** Panacea's share of the effort grows from 4% to 17% because running our own Panacea
is glue on Panacea's code; our own share holds at 61% because the user screens and the operator
console are ours. **Challenge it here:** counted as ours, the changes row 112 makes in the copied
code (4.5) would put our own code at about 63%.

### 2.5 By size of software delivered: measured for the lift, not a percentage

The lift brings 195,166 lines of non-test Go (`panacea-api` 177,086, `panacea-build` 11,536,
`webhook-receiver` 4,206, the static gateway 1,232, `panacea-shared` 655, the analytics injector
451) and 194,899 of tests; 21,025 lines of pipeline YAML, 16,282 of Kubernetes YAML, 26,988 of
engine shell and about 4,500 of HCL (`git grep -c` at `ca92e83eb`). Our own kept Go on this branch
is about 17,900 lines (`00-plan.md` §2.2 (iii)). By size, Panacea's copied code is about eleven
times ours. It is still not a percentage, because lines are not capabilities.

### 2.6 The calendar

- **First app, five builders: about 18 to 23 working days** (was 13 to 17). About 23 engineer-days
  join the first app's path: rows 110 (2.5), 111 (9) and 116 (2.5) whole, and the first-app shares
  of row 112 (about 3.5), row 114 (about 4), row 117 (about 1) and row 137 (about 1.5, GitHub's
  half), less row 71 (0.5) and row 73's Connect deploy share (0.25). Run as section 3.3 runs it,
  B2 on the track from day 1, B3 joining for row 114 after the lane path and row 112 on B5, the
  track ends at about day 15 or 16, later than the sandboxes (about day 10), so it becomes the first
  app's longest pole; and B2's own first-app rows move to B1 after the sandboxes, ending at about
  day 17. Judged: about 5 to 6 working days more.
- **All of it, five builders: about 43 to 45 working days** (was 26 to 27). `00-plan.md` §3.3's
  scaling, 08-verify's 20 to 21 working days for 103 engineer-days: 222.975 × 20 / 103 = 43.3, and
  × 21 / 103 = 45.5.
- **With a sixth builder for the first three weeks** (question 1): B6 starts the track on day 1 and
  B2 joins it after its own first-app rows, so none of them moves to B1, and the track ends at about
  day 12 or 13: the first app in about 15 to 19 working days (the track plus its integration, close
  to the plan's own 13 to 17). The sixth adds about 15
  engineer-days: (222.975 - 15) × 20 / 103 = 40.4, and × 21 / 103 = 42.4, so all of it in about 40
  to 42.
- **Two builders:** the first app in about 8 to 9 weeks (5.5 to 6.5, plus 23.25 / 2 = about 11.6
  working days); all of it in about 22 weeks (222.975 / 2 = about 111.5 working days).
- **Rows 113, 118 and 120 to 136, and the rest of rows 112, 114, 117 and 137, sit after the first
  app,** as rows 106 to 109 already do: the first app's journey has one author and a class-`none`
  pilot. Row 119 runs weekly throughout.
- **Risk to the dates:** the org's monthly spend limit stopped every agent run on 2026-09-30,
  Panacea's included (`docs/cutover/RESUME.md:5` at `3d45587f0`). A longer build at five or six
  streams spends faster, and these dates assume no such stop.

## 3. Build order and day 0

### 3.1 Day 0

Against `00-plan.md` §3.1's items:

1. **Item 1 grows: a second resource group in Panacea's subscription,** for the apps estate,
   tagged and named apart from every Panacea estate, with vCPU quota in Australia East for its
   pools: about 20 vCPU at pilot size (one D2s_v5 system node, two or three D2s_v5 workload nodes,
   one E4s_v5, one D8ds_v5 CI node), and 8 more for each build machine while it runs. The quota is
   per subscription, so it counts beside our Kata pool and Panacea's two Australia East estates,
   each of which starts at 56 vCPU (`docs/cutover/2026-09-24-understand/east-infra.md:515` at
   `3d45587f0`).
2. **Item 3 shrinks.** Of today's Panacea we ask only what `OD-44` already needs: its realm block,
   and our egress address on its edge for that realm. P-1 to P-3, the legacy-key guard, the SAST
   fix and the step-up map are no longer asked; they are our own changes now (rows 112 to 114). If
   the owner agrees to question 4's line (a), the realm block goes too, and with it the stall risk
   on sign-in.
3. **New: one record delegating `apps.panacea.kpmg`** (question 3) in the authoritative
   `panacea.kpmg` zone, never the same-named, undelegated decoy
   (`deploy/environments/panacea-dev.env:41-46`). The owner's own account wrote dev's delegation
   this way on 2026-08-19. It goes in outside the move's read-only windows and soaks, and is
   declared to the move's DNS inventory.
4. **New: the apps estate's pipeline,** in our own Azure DevOps organisation (`OD-48`): a service
   connection whose principal holds Contributor on the apps resource group only, never Panacea's
   deploy principal, and one paid parallel job, because a bootstrap runs past the free 60-minute cap
   (Panacea's dev launch, 2026-08-18).
5. **New, for the authenticator:** an owner of `kpmg-au-emu` installs our GitHub App (item 5's "a
   GitHub App owner" already asks for one); each Azure DevOps organisation's admin adds our managed
   identity with Basic access; and, if the owner agrees, question 4's IT request, filed on day 0 so
   its lead time runs beside the build.

Item 5's KPMG PAT policy (global PATs stop working on 2026-12-01) now matters only to the last
fallback (row 136).

### 3.2 Before the build

- **Phase 2's close-out and merge to `main` stays first.** Before it merges, phase 1's Kata
  node-pool Terraform leaves this branch, as `feat/env` already removed it (`5f499921`):
  `infra/terraform/kata.tf` adds a node pool to whatever cluster it is pointed at, and its
  `dev.tfvars:4` names `aks-panacea-dev`, a cluster name two Panacea estates share.
- **The owner's review of the shell prototype** (`00-plan.md` §3.2) gains one Deploy page over our
  own Panacea. No other prototype changes.
- **Three day-1 spikes are added:** the managed data plane on the estate's first bootstrap, the
  first live run of a path Panacea ships dormant (`clients/managed_object_storage.go:19-21`,
  `clients/managed_postgres.go:25-28`); OpenBao under the engine's Vault steps; and Git Credential
  Manager's Microsoft sign-in for `kpmgcomau` on a managed laptop (the Azure DevOps bridge).

### 3.3 Five builders

| Builder | Weeks 1 to 3: the first app | After it |
|---|---|---|
| **B1, sandboxes** | Rows 1 to 9, as planned (about 10 days), then our Orca build's first-app rows, moved from B2 (19 to 24, 93, 99, 102, 103) | Penpot's runners (39), Design stream B, the operator console's part 2 (131) |
| **B2, our own Panacea** (the new long pole) | From day 1: rows 110, 111 and 116, then 117's first-app share | Rows 117 and 118, the weekly re-sync (119), the teams copy (120) at go-live, then Design stream A (34, 36 to 38, 44), rows 87 and 94 |
| **B3, the lane path** | Rows 11 to 18 (about 10 days), then row 114 with B2, and row 137's GitHub half | Define (25 to 33), the tracker Start (48, 55), rows 134 and 136, row 137's Azure DevOps half |
| **B4, the Hub** | Unchanged: row 74's minimum, 75, 88, row 105's Hub side, the Desk (89 to 91), 100 | The rest of the Hub, the Work page (47, 50), 108, 41 and 42, then the pass-through and project area (127, 128) and half the user screens (121 to 126) |
| **B5, gateway and Deploy** | Unchanged first half (76, 76b, 104, 82, 95), then row 112 and the first-ship Deploy glue over our Panacea (61 to 66, 115) | 107, 109, 113, 106, the other half of the user screens, 129, the operator console's part 1 (130) and 132, the Connections page (133), 135, the tracker connectors (45, 46), the Teams bot (52, 53, 92) |

**With a sixth builder** (question 1): B6 takes rows 110, 111 and 116 from day 1; B2 keeps its own
first-app rows and then joins B6 on the estate and row 117; B3 still joins for row 114 after the
lane path; and B1 is not displaced.

### 3.4 The acceptance gate

- **K2 and K3, restated again** (`00-plan.md` §3.4). The only allowed exit is the first-run sign-in
  tab and its weekly re-check, which the owner accepted; GitHub's one Authorize click rides it, once.
  No token is pasted anywhere, with one named fallback: if both question 4's consent and the Git
  Credential Manager bridge fail, a person on an Azure DevOps project pastes one token on our page,
  and that is put to the owner's eyes as a forced exception.
- **J7 now also checks** that a `protected` preview opens in the frame with no sign-in page, and
  that no step of Deploy shows a Panacea site, a key or a Portal link.
- **Gates added:** a security review of row 112 (our copy's identity path, the transport secret, the
  realm-scoped admin account) before the pilot; the snippet ledger's CI (row 72) failing on any
  AGPL, BUSL, SSPL or Elastic code in what we build; and the merge's own plan before the old Portal
  retires.

### 3.5 What sign-off records

- **`00-plan.md` re-planned from this file:** §1.6's rows as section 1.2 changes them, the new rows
  into §1, §1.9's sum, §2's percentages, §3's day 0 and builders, §3.4's gate, and §5.2's forced
  exceptions 2 to 4 closed. Call 13a's pilots move from Panacea's dev estate to ours, still class
  `none`.
- **The register:** an `OD` for the authenticator, which amends `OD-30`'s "Azure DevOps stays a
  personal access token"; an amendment of `OD-44` only if question 4's line (a) is agreed; `OD-47`'s
  resource group for shipped apps, if question 1 is agreed; `OD-53`'s edge to the re-plan; and
  `OD-44`'s edge from `OD-13`, checked here and holding (section 0 item 5).
- **`CLAUDE.md`'s reuse section,** which still says "the Deploy step's API" and "the later lift ...
  still stands (`OD-13`)", rewritten to this shape.
- **Pending questions it settles** (`2026-09-25-owner-questions-pending.md`): Q-P7-7 (which key users
  mint) is moot, because no key exists; Q-P7-6 (how `branchEnv` resolves the production branch,
  `handlers/branch_security.go:29-37`) and O-2 (the project-group prefix match) become ours to fix
  in our copy, and O-2 reads as fixed at `ca92e83eb` (`clients/keycloak.go:468-480` matches exact
  names); the Hub's question 5 (step-up for Azure DevOps) is answered by row 136's broker path once
  the consent exists.

## 4. Owner questions

Product, money and the owner's own systems only. Every technical choice above is ours.

1. **Build our own Panacea at this cost?** It adds about 91 engineer-days (70 to 120) to the 132
   left, about 223 in all:
   - about 54 for our own Panacea: its own servers, a sign-in with no keys, and the admin screens
     you use today on Panacea's Portal, rebuilt in the app;
   - about 9 for connecting Azure DevOps and GitHub automatically;
   - about 28 for in-app screens the plan already owed under your "never outside" rule and had
     missed: Panacea's data, storage, logs, risk, cost and team screens.

   With five builders the first app moves from about 13 to 17 working days to about 18 to 23, and
   everything from about 26 to 27 to about 43 to 45. The new servers sit in a new resource group
   in Panacea's subscription, on the same bill and the same cost codes: about US$900 to 1,000 a
   month in machines while it is a pilot, roughly US$1,400 to 2,000 with the database, registry
   and logs (not yet priced). Build machines start on demand, so the first build after a quiet
   spell waits a few minutes; keeping two warm would add about US$830 a month.
   **Recommended: yes, with five builders.** If the first app's date matters more than the spend, a
   sixth builder for the first three weeks keeps it at about 15 to 19 working days.

2. **Today's Panacea, and the people on it.** **Recommended: leave them where they are until
   Panacea's move to Australia East is signed off.**
   - Existing projects, apps and users stay on today's Panacea and its website, untouched. Apps
     made in the new app go to the new Panacea.
   - We copy teams, members and cost centres across once, so a team and its cost centre look the
     same in both, and we pull today's Panacea's fixes into ours every week.
   - We change nothing in today's Panacea during its move, except one web-address record for the
     new apps (question 3) and the sign-in settings you already agreed.
   - Once the move signs off, the two become one Panacea in this repo, and older apps appear in the
     app too. Until then, someone with older apps opens them on today's website.

3. **The web address of apps built in the app.** **Recommended: `<app>.apps.panacea.kpmg`,** with
   previews under the same name. "apps" is already a reserved word in today's Panacea, so no
   existing project can clash with it (`handlers/team_subdomain.go:35`). It takes one record in
   the panacea.kpmg zone, which your own account can add, as it did for dev.panacea.kpmg. Apps
   stay reachable from KPMG's network only, as you agreed.

4. **One request to KPMG IT, with three lines.** **Recommended: yes, sent on day 0.** Nothing waits
   on it.
   - (a) Our sign-in talks to Microsoft directly, through Panacea's existing Microsoft app (one more
     sign-in address and secret). Sign-in then stops depending on today's Panacea login staying up
     through its move, and whoever runs that login can no longer sign in as one of our users. This
     changes your "piggy back off Panacea" answer (`OD-44`).
   - (b) A "Panacea for Azure DevOps" app in the Microsoft tenant that holds KPMG's Azure DevOps,
     approved once for everyone, so Azure DevOps connects with nothing to paste.
   - (c) The Windows sign-in option on the same app as (a), so the weekly sign-in tab can go if the
     day-1 test passes.

   Until (b) lands, Azure DevOps tries the laptop's own Microsoft sign-in, and asks for a pasted
   token only if that fails too. If you are listed as an owner of Panacea's Microsoft app
   (`AU-PROD-Panacea-RITM6316333`), you can add (a) and (c) yourself in minutes, with no ticket.

## 5. Not verified

Nothing was installed, built or run. Every day figure is an estimate composed from estimates.

| Area | Not verified | How to close it |
|---|---|---|
| The estate | The engine has never run in this shape: no realm, no Portal, no Forge, OpenBao, the managed plane, Australia East, our pipeline. That a new `ENVIRONMENT` value sizes and names cleanly was not traced | Row 111 and its day-1 bootstrap |
| The managed data plane | It is a dormant path in `panacea-api` (`clients/managed_object_storage.go:19-21`, `managed_postgres.go:25-28`), never run live on any estate. The move's own S3 gateway choice is still open (`docs/cutover/2026-09-25-design/PLAN.md:24`, `:32` at `3d45587f0`). The preview clone path under it was not traced | The day-1 spike (section 3.2) |
| OpenBao | That it replaces Vault under the engine's init, unseal, KV v2 and Transit steps, and Chains' `hashivault://` signer, is from knowledge | The day-1 spike |
| Identity in our copy | Our realm's token on the transport-secret path was read, not exercised. That Keycloak's group mapper can emit Panacea's group names as they are, the realm-scoped account's rights, and the second realm for shipped apps' users are design, not tried. The no-JWKS fail-open is described in Panacea's own comment (`jwks.go:52-68`); the code path was not read line by line | Row 112, then its security review |
| Pushes | That Panacea's deploy poller carries pushes for our estate behind its 14-address edge; whether today's Panacea relies on webhooks or the poller was not checked | Row 114 |
| The authenticator | Whether Git Credential Manager's Microsoft sign-in is silent on KPMG laptops (the scout's probe was refused by its session's safety check); whether Windows' account broker silently issues `eb9a11c9…` tokens to `deff24bb…` accounts that are guests there (no Microsoft document found); MSAL Node's native broker in Orca's main process (from knowledge); KPMG's user-consent policy in both tenants; who owns `37f8bd70…`; who administers `eb9a11c9…`; whether KPMG's GitHub enterprise signs in by OIDC, so that Conditional Access applies; that a GitHub App's re-authorisation completes with no page (documented for OAuth apps only) | The day-1 laptop spike; the owner's check of the app's owners (question 4) |
| The subscription's tenant | That Panacea's subscription sits in `eb9a11c9…` is the identity scout's `az account show`; `deploy/environments/panacea-sandbox.env:86-92` says only that it is a different tenant from the sign-in one | One `az account show` with a working session |
| The routes and screens | The 357 / 169 classification is a static read of `routes.go` at HEAD `2836cee91`, not `ca92e83eb`; paths built at runtime may be missed. No Portal usage data was read. The pages' days assume the Portal's panel logic is reused | Rows 121 to 131, re-summed when their plans exist |
| Running cost | Machines are retail list prices from the Azure Retail Prices API (2026-10-01): D2s_v5 US$0.12, E4s_v5 US$0.302, D8ds_v5 US$0.568 an hour, Linux, pay-as-you-go. Everything else in the all-in range is inferred; the subscription's quota headroom was not checked | Pricing before day 0 |
| The calendar | The first-app figures are judged, with the estate track run as section 3.3 runs it (B2 from day 1, B3 after the lane path, row 112 on B5); no spend-limit stop is assumed | Re-sum at the first weekly check-in |
| The cutover | No dates exist for its sign-off; "weeks away" is inferred from its plan (`PLAN.md:27-29`) | Panacea's own schedule |
| The re-sync | The rate is judged from churn (679 commits in 30 days, the inventory scout's count) | Measure the first three pulls |
| What runs on dev | That `ado-gov/dev` is what Panacea's dev cluster runs was not checked (no `kubectl`) | Read the dev cluster's image tags |
| Licences | `madmin-go`'s pin was read here (`go.mod:9`); its AGPL-3.0 licence is the inventory scout's read of its LICENSE in the module cache. Vault BUSL-1.1, Redis 7.4 RSALv2/SSPLv1, Semgrep's rules licence, Opengrep LGPL-2.1, ClamAV GPL-2.0 and Grafana AGPL-3.0 are from `00-panacea-verify.md` §3 and upstream knowledge; `00-plan.md` §5.3's "Later" licence review still applies to what runs unmodified | Row 72's CI check, and the "Later" review |
| The build | That the lifted modules compile at Go 1.25 outside our `go.work`, and their tests pass in our CI | Row 110 |

Each scout's own "not verified" list stands, and so does each cluster's, as `00-plan.md` §6 lists
them.

## 6. Sources

Read 2026-10-01, read-only.

- **This programme:** `CLAUDE.md`; `CONTEXT.md`; the register,
  `docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-13`, `OD-30`, `OD-32`, `OD-37` to
  `OD-53`); `docs/superpowers/planning/2026-09-25-owner-questions-pending.md`. In this folder:
  `00-plan.md` (§0 to §10), `09-cohesion.md`, `00-panacea-verify.md` (§2 to §4, §7),
  `06-deploy.md:120-132`, `06-deploy-verify.md` (§2.3 to §2.5), `05-work-team-verify.md` (§8).
- **This repository's tree:** `go.work`; `control-plane/go.mod`; `infra/terraform/kata.tf` and
  `dev.tfvars` on this branch; `feat/env` at `5f499921` and
  `infra/terraform/modules/network/variables.tf:23-30`; `feat/hub`'s
  `docs/superpowers/plans/2026-09-23-phase-h-hub.md:432`, `:442`, `:566`; `feat/phase-3-git`'s
  task 13 (`:97-116`); `feat/phase-7-deploy`'s task 12 (`:158-163`).
- **Panacea, at `ado-gov/dev` `ca92e83eb`** (`Projects/Panacea`, read with `git show`, `git grep`,
  `git ls-tree`; never checked out or written): `services/panacea-api/` `go.mod`,
  `middleware/auth.go`, `middleware/jwks.go`, `middleware/entra_claims.go`, `clients/keycloak.go`,
  `clients/tenant_redis.go`, `clients/managed_object_storage.go`, `clients/managed_postgres.go`,
  `clients/kubernetes.go`, `clients/minio_admin.go`, `handlers/project_auth.go`,
  `handlers/register.go`, `handlers/team_create_approval.go`, `handlers/team_subdomain.go`,
  `handlers/projects.go:318-328`, `handlers/branch_security.go`, `routes/routes.go`; beside it,
  `deploy/environments/README.md`, `panacea-dev.env`, `panacea-sandbox.env`, `panacea-aue.env`,
  `deploy/panacea-deploy.sh`, `deploy/cloud-shell-deploy.sh`, `deploy/variables.tf`,
  `deploy/main.tf`, `deploy/acceptance/`, `azure-pipelines-dev.yml`,
  `pipelines/tasks/git-clone.yaml`, `pipelines/tasks/sast-scan.yaml`,
  `kubernetes/helm-values/oauth2-proxy-values.yaml`. Line counts by `git grep -c` and file counts by
  `git ls-tree` at that commit.
- **Panacea's cutover documents, at `3d45587f0`:** `docs/cutover/RESUME.md`,
  `docs/cutover/2026-09-25-design/PLAN.md`, `docs/cutover/2026-09-24-understand/east-infra.md`.
- **Live, anonymous, 2026-10-01:** `https://dev.azure.com/kpmgcomau/_apis/connectionData`
  (`X-VSS-ResourceTenant: eb9a11c9…`), matching the `azureauthority` this machine's git
  configuration pins for `kpmgcomau`; the Azure Retail Prices API
  (`https://prices.azure.com/api/retail/prices`, `australiaeast`).
- **The owner's session notes** of 2026-08-17 to 08-19 (Panacea's dev launch on the same engine) and
  of 2026-09-28 (the cross-estate cookie shadowing, and its estate-specific cookie names).
- **The five scouts of this round,** cited as theirs wherever a figure was not re-derived here: the
  inventory (sizes, the lift's days, the estate's needs), the user functions (the route
  classification and the screens' days), identity for Azure DevOps and GitHub (tenants, GitHub's
  and Microsoft's documented flows, Keycloak's grants), the two estates side by side (the cutover's
  rules, the DNS and naming hazards, keyless builds), and the deploy-layer options (the scoring
  that rejected driving today's Panacea out of sight, and Coolify plus Supabase).
