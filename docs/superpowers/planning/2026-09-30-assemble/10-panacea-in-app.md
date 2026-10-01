# 10 - Panacea in the app: our own Panacea, and the authenticator

**Status:** PROPOSAL (2026-10-01), corrected the same day by its verify round (section 7), for the
owner's sign-off on its cost before anything is built (his standing rule: cost and scope first).
Until he signs off, `00-plan.md` stays the plan of record and none of its rows or totals change;
its section 5.3 points here, and its sections 3.3 and 5.2 now note the two figures this file
found wrong in it (section 2.6, section 1.4).
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
**Method:** five read-only scouts (Panacea's inventory, its user functions, identity for Azure
DevOps and GitHub, two Panacea estates side by side, and the deploy-layer options), with every
load-bearing figure re-checked at source, then a verify round of two refuters whose every finding
was checked at source before it was applied (section 7). Panacea was read at `ado-gov/dev`
`ca92e83eb` (API v1.165.0, the line live on Panacea dev) with `git show`, `git grep`, `git log`
and `git ls-tree`, never checked out or written; its cutover documents at `3d45587f0`. Live reads,
all anonymous: Azure DevOps' answer for `kpmgcomau`, and the Azure Retail Prices API (re-read in
the verify round). Nothing was installed, built or run. Section 5 lists what is not verified.

## 0. The answer

**Our own Panacea is Panacea's deploy side, lifted into this repo from the line live on Panacea dev
and run by us on a second cluster, in a resource group of its own in Panacea's subscription, on the
same in-cluster data plane Panacea dev runs today. Every act a person or an operator does on
Panacea's Portal becomes a page in the app, and our sign-in is its only identity for people, so no
person mints, holds or pastes a key. Today's Panacea and its users stay as they are until its move
to Australia East signs off; until then older apps stay on today's website, a named forced
exception; then the two become one Panacea in this repo. Azure DevOps and GitHub connect from one
Connections page in the app: background work needs one-time admin steps only, GitHub needs one
Authorize click, once, and Azure DevOps acting as the person is silent after one IT consent. It
adds about 107 engineer-days (85 to 140): about 71 for our own Panacea, about 8 for the
authenticator, and about 28 for in-app screens the plan already owed whatever this decides. The
plan becomes about 239 (205 to 290). With five builders the first app moves from about 13 to 17
working days to about 22 to 25, and all of it from about 26 to 27 to about 50 to 56. The second
cluster costs about US$1,400 a month in machines at pilot size, about US$1,700 to 2,100 all-in.
Nothing is built until the owner signs off this cost (section 4).**

1. **What "first-class inside the app" means: our pick.**
   - Deploying becomes one of the app's own capabilities, run by code in this repo on
     infrastructure we run. Panacea's deploy control plane, `panacea-api`, becomes ours and does
     what it does today: previews per branch, ship, promote, roll back, versions, evidence, tenant
     namespaces, databases, storage, secrets, the scan pipeline and its posture gate, cost codes
     and team statements.
   - Nobody sees it. Every call to it goes server-side through one proxy in our platform cluster:
     G4 (row 64), which `panacea-mcp`'s agent calls already use and row 127's pass-through for Hub
     pages shares. Phase 7's client in the laptop's control plane (row 66) calls through it too, so
     no laptop holds a secret or reaches our Panacea directly.
   - It trusts one identity for people, our realm, and mints no keys. Our copy's auth middleware
     gains a bearer-token branch: the person's own short-lived token from our realm, checked
     with Panacea's already-exported verifier (`middleware/auth.go:444-457`) against our realm's
     keys, issuer and a `panacea-deploy` audience (`middleware/jwks.go:147-153`), its
     unverified-decode grace at zero and its no-JWKS fail-open closed. The proxy adds that token
     and strips any `X-Auth-Request-*` or `X-Backstage-Identity` header a caller sends. Panacea's
     shared transport secret then carries in-cluster service calls only (the pipelines,
     `webhook-receiver`), never a person: in our copy the branch that trusts identity headers on
     that secret alone (`auth.go:188-216`, which accepts any user and any group,
     `platform-admins` included) and legacy `panacea_` project keys (`:231-242`) are removed, and
     key minting is unrouted. The same secret also derives every tenant's object-store key and
     AI-proxy token (`clients/minio_admin.go:356-364` with `handlers/provisioner.go:871`;
     `clients/pipeline.go:1011`), so our copy gives that derivation a secret of its own, as
     Panacea's own review proposes (S-9, `docs/cutover/2026-09-25-design/PLAN.md:442-444` at
     `3d45587f0`). All of it is changes we own in our copy (row 112), not new code in today's
     Panacea queued behind its freeze.
   - Previews sign in against our realm too, so the realm hand-off (row 95) gives them a session
     inside the frame, `protected` previews included.
   - **Not taken:** driving today's Panacea out of sight. It hides the outside site instead of
     removing it, builds no Panacea of ours, and "no key" would still wait on new sign-in code in
     today's Panacea behind its cutover freeze (`06-deploy-verify.md` §2.4).

2. **What we lift, and from where.**
   - **From** `ado-gov/dev` at one pinned commit, today `ca92e83eb` (API v1.165.0), copied whole
     under `OD-50`'s attribution headers and ledger (row 72), the scope `OD-53` left to this file
     and question 1 puts to the owner in plain words, and re-synced weekly after it (row 119).
   - **What:** `panacea-api` with `panacea-shared` and `services/contracts`; `panacea-build`, its
     step images and `panacea-migrate-runner` (the `db-migrate` step image); `webhook-receiver`;
     the static gateway; the analytics injector; `panacea-mcp`; the Tekton pipelines, tasks and
     scanners; `kubernetes/base`, the live Helm values, the Argo CD applications the engine uses
     (re-pointed at our repo), Tekton Chains, CloudNativePG and the security-policy tests; and the
     live deploy engine, `deploy/panacea-deploy.sh` and `deploy/cloud-shell-deploy.sh`, with only
     the files they read (`deploy/lib`, `deploy/gates`, `deploy/acceptance`, `deploy/*.tf` under
     OpenTofu, and the manifests and charts their phases apply), traced from the engine in row
     110. About 1,900 files; `panacea-api` alone is 177,086 lines of Go and 169,839 of tests.
   - **Left out:** the Portal (Backstage), Forge (our cockpit replaces it) and its preview mirror,
     Apothecary, marketing, the docs site, the Argo CD app-of-apps root, `local/`,
     `deploy/cutover/`, and `terraform/`, which is never copied or applied (`CLAUDE.md`'s gotcha).
     **Left out because they target live production:** Panacea's env files (one of them,
     `panacea-sandbox.env`, is live production's own, `ESTATE="prod"`, `:21-27`),
     `deploy/terraform.tfvars` (production values, `:1-31`), its root `azure-pipelines*.yml`,
     `move-dns-zone`, which moves the live `panacea.kpmg` zone (`deploy/panacea-deploy.sh:1781`),
     and every ad-hoc or production-operations script in `deploy/` (55 files there name or default
     to production's resource group).
   - **Swapped as it lands**, so no AGPL, BUSL, SSPL or RSAL code enters what we build:
     `madmin-go` (AGPL-3.0, `go.mod:9`), used for five MinIO admin calls (service-account add,
     update and info, bucket quota, storage info: `clients/minio_admin.go:58-62`,
     `clients/kubernetes.go:8544`, `:8573`, `:8679`), replaced by a small admin client of ours
     behind the same interface, written from MinIO's Apache-2.0 SDKs' admin clients and never
     from `madmin-go`; Redis 7.4 (RSALv2/SSPLv1) replaced by Valkey 8 for tenant caches
     (`clients/tenant_redis.go:70`); Vault (BUSL-1.1) replaced by OpenBao (`OD-32`); SAST run on
     rules baked into its image, which also ends the false pass it records today
     (`00-panacea-verify.md` §2 row 11). MinIO's server and its `mc` tool (AGPL-3.0) run
     unmodified as images, not as code we build; they fall under `00-plan.md` §5.3's "Later"
     licence review, which already names MinIO.

3. **What our in-app surfaces replace.**
   - **Panacea's Portal, all of it, for our Panacea:** every user act on its 21 project tabs and
     its team, onboarding, account and Pages screens (rows 107 to 109, already planned, and rows 121
     to 129), and its operator console (rows 130 to 132).
   - **Forced exceptions 2, 3 and 4 close** (`00-plan.md` §5.2). No Connect deploy and no key. No
     step-up exit to the Portal: the step-up map is now ours, unmapped in v1 as Panacea ships it,
     with production approval on our Deploy page (row 109). The Azure DevOps token gives way to the
     authenticator (item 6), and nothing is pasted anywhere.
   - **Three things stay off the screens, all named** (`OD-49`). Until the operator console lands,
     after the first app, the few operator acts the pilot needs are done by the build team with its
     own realm sign-in, and no user waits on one. The Panacea functions with no Portal screen today
     (team baselines and templates, integrations, policy constraints, compliance and egress views)
     stay reachable by agents only, as now, with their pages later (section 1.7); stored git
     tokens, the one key among them, are unrouted in our copy (row 137). And until the merge,
     older apps stay on today's Panacea website: **forced exception 5** (section 1.6, question 2).

4. **Where it runs.**
   - **On its own AKS cluster, the apps cluster,** in a new resource group in Panacea's
     subscription, in Australia East, stood up by Panacea's own new-environment engine
     (`deploy/panacea-deploy.sh`) with a sixth env file (`deploy/environments/` already holds five:
     `aue`, `dev-hardened`, `dev`, `prod` and `sandbox`, the last live production's). Same
     subscription, bill and cost codes as today, but a resource group of its own, which changes
     `OD-47`'s "Panacea's ... resource group" (question 1). Every resource name, `ESTATE` and
     `ENVIRONMENT` differs from every Panacea estate's, the move's dev-East (`devaue`) included
     (`OD-42`; `PLAN.md:116`).
   - **Never on the sandbox cluster:** `panacea-api` holds rights over every tenant's secrets and
     runs the apps agents write, and `OD-42` keeps tenant workloads off the agents' cluster.
   - **Its data plane is the one Panacea dev runs today:** CloudNativePG and MinIO in the cluster,
     with OpenBao for Vault and Valkey for Redis (row 58). It needs no data-plane change under
     `services/` (`docs/cutover/2026-09-24-understand/plan-of-record.md:505-510` at `3d45587f0`)
     and keeps every function the in-app pages expose: backups, revert, high availability, branch
     data and dedicated databases. **Not taken: the managed plane** (Flexible Server, and Blob
     behind an S3 gateway) that Panacea's move is building. At the pin it cannot run: its S3 proxy
     image cannot build and no deploy path installs it, its Blob Terraform pairs a hierarchical
     namespace with versioning, it changes every tenant's key (`PLAN.md:425-426`;
     `plan-of-record.md:161`), and its tenant isolation is an open design gate
     (`docs/runbooks/aus-east-rebuild-roadmap.md:232-254` at `ca92e83eb`). Panacea sizes the
     plane it will run at about 15 engineer-days of `services/` code (its slices SL-3 and SL-5,
     7.5 days each, `PLAN.md:667`, `:706`), and that design refuses backups, revert, high
     availability, branch data and new dedicated databases with a 409 (`PLAN.md:503`, `:509`).
     The managed plane comes with the merge, once Panacea has built and proven it.
   - **Its edge admits KPMG's network only** (call 13). Apps get `<app>.apps.panacea.kpmg` if the
     owner agrees (question 3). Its API is not on that edge: our platform cluster reaches it over
     a private peering.
   - **Its gates are Panacea dev's:** no Azure Firewall, web firewall, paid cluster tier, Defender
     or customer-managed keys; Panacea's own move pins the firewall, the web firewall and
     customer-managed keys off for its East estates too (`PLAN.md:116`, P-8). So apps built
     during the pilot can reach the internet, as on Panacea dev today: the tenant baseline carries
     no egress allowlist and leaves egress to the firewall (`clients/tenant_ensure.go:45-48`).
     Question 1 says so. The production estate (section 1.7) turns the gates on.

5. **Existing Panacea users, and the PROD cutover.**
   - **They stay where they are.** Their projects, apps and website are untouched; until the merge,
     older apps are opened and changed on today's website (forced exception 5). Apps made in the
     app go to ours. Teams, members and cost centres are copied across once, at go-live (row 120);
     after that a team has two homes until the merge, today's Panacea for older apps and ours for
     new ones, which amends `OD-51`'s "one home" (question 2). The first app's walk uses one team
     made by hand in ours.
   - **Nothing is written to today's Panacea** before its production move signs off, beyond:
     `OD-44`'s one block of realm settings; our platform's egress address on its edge, a Terraform
     variable there, so it needs the owner's yes during the move's freeze
     (`docs/cutover/2026-09-24-understand/east-infra.md:527` at `3d45587f0`); one record
     delegating the apps zone; and, if question 4 is agreed, IT's change to the Microsoft app live
     Panacea signs in with. Each goes in outside the move's windows and is declared to the move's
     inventory (section 3.1).
   - **After sign-off, the two become one Panacea in this repo** (`OD-13`'s lift, completed):
     today's estates deploy from here, older apps appear in the app, and the Portal retires on the
     owner's date. It is sized then (section 1.7).
   - **`OD-44` stands, as the register asked this file to check.** Its edge from `OD-13` still
     holds: today's Panacea stays a separate running system until that merge, and its realm
     changes only configuration.

6. **The authenticator for Azure DevOps and GitHub.** One sign-in, plus service identities, with
   each person's own tokens fetched on their laptop. Neither GitHub nor Azure DevOps accepts our
   sign-in's token. GitHub has no exchange from a Microsoft sign-in. KPMG's Azure DevOps
   organisation, `kpmgcomau`, answers from Microsoft tenant `eb9a11c9…` (read 2026-10-01), the
   management tenant Panacea's subscription sits in, while people sign in to the corporate tenant
   `deff24bb…`, where Panacea's Microsoft app lives, IT-owned
   (`deploy/environments/panacea-dev.env:141-142`, `:153`). Our own Azure DevOps organisation
   (`OD-48`) is created connected to `eb9a11c9…` too, so one managed identity and one app serve
   both. Every connection shows on one Connections page, with its state, reconnect and revoke.

   | Connection | How it connects | Automatic? | One-time step | Who must act |
   |---|---|---|---|---|
   | GitHub, background (task sync, agents reading code, builds) | Our GitHub App's installation tokens | Yes | The App created and installed | An owner of the `kpmg-au-emu` organisation (the programme's owner is a member there, not an owner) |
   | GitHub, as the person (reviews, starting from an issue) | The App's user authorisation, in the first-run sign-in tab | Yes, refreshed for 6 months at a time | One "Authorize" click, once | The person |
   | Azure DevOps, background | Our managed identity, in tenant `eb9a11c9…` | Yes | Added once to each organisation | `kpmgcomau`'s Azure DevOps administrators, and ours |
   | Azure DevOps, as the person (reviews, comments, tickets) | Windows' account broker, silently, from a new Panacea app in tenant `eb9a11c9…` | Yes, once consented | None for the person | **IT: the app and one admin consent in that tenant** (question 4) |
   | Azure DevOps, as the person, until that consent | The laptop's own Git Credential Manager sign-in, through phase 3's git helper. If that fails too, comments post through our managed identity naming the person, and a vote that must be the person's own waits (question 4). No token is pasted | If Git Credential Manager works on KPMG laptops (not verified) | None | Nobody |
   | Deploying | Our realm's token, server-side | Yes | None | Nobody |

7. **What it costs.**
   - **About 107 engineer-days net** (the rows' own lows and highs sum to 68 and 172; taken as 85 to
     140), in three parts: about 71 for our own Panacea (the lift, the estate, its identity, the
     deploy path, the swaps, the proofs, the weekly re-sync, the teams copy and the operator
     console, less the 0.75 that rows 71 and 73 give back); about 8 for the authenticator (less the
     0.25 row 74 gives back); and about 28 for in-app pages for Panacea's user screens, which
     `OD-49` required whatever `OD-53` said. `00-plan.md` §5.2's verdict on the Portal, "Pass,
     with the exits named", does not hold: the plan brings about 90 of Panacea's 357 user routes
     into the app (section 1.4).
   - **The plan becomes about 239 engineer-days (205 to 290).** Counted like for like, the plan of
     record with the owed screens is about 160, so our own Panacea and the authenticator add about
     79. Done falls from about 11% (9% with the owed screens) to about 6%. By capability, open
     source falls from 35% to 28% and ours rises from 37% to 43%; by effort, ours falls from 61%
     to 57% and glue on Panacea rises from 4% to 22% (section 2).
   - **Run cost:** about US$1,400 a month in machines at pilot size, about US$1,700 to 2,100
     all-in (machines at list price, the rest inferred). The production estate, needed before the
     pilot widens past class `none`, costs about US$3,300 to 5,200 a month more (section 1.7).

## 1. The costed rows

### 1.1 How to read them

Numbering continues `00-plan.md` §1 (rows 1 to 109, and 76b). The Kind rule is that section's:
**OSS**, **Panacea** (Panacea's running service or its copied code), **Managed**, **Ours**; code we
write is Ours even when it wraps an adopted piece. Under it, lifting, configuring and running
Panacea's code is glue on Panacea, and a capability no product provides is Ours even when it is
written inside our copy (rows 113, 120); our own sign-in flows over an outside service are Ours
(rows 134, 136). Days are engineer-days, low, mid and high; the mid figure is the one carried into
every sum, as `00-plan.md` carries one figure a row. Panacea paths are at `ado-gov/dev`
`ca92e83eb`, with `services/panacea-api/` left off the API's paths, unless marked.

### 1.2 Rows `00-plan.md` carries, changed

| Row | In `00-plan.md` | Under this proposal | Days before | Days after | Kind |
|---|---|---|---|---|---|
| 57 | Deploy engine: Panacea's API on `dev.panacea.kpmg` | Our own Panacea's API, lifted (row 110), on our apps estate (row 111) | 0 | 0 | Panacea |
| 58 | Shipped-app backend on CloudNativePG, Vault, Knative, Flagsmith, MinIO | CloudNativePG (shared and dedicated tiers), MinIO, OpenBao, Flagsmith, Valkey: the in-cluster plane Panacea dev runs, so backups, revert, high availability and branch data work as there. Knative stays off, as Panacea's engine leaves it by default (`deploy/cloud-shell-deploy.sh:6282-6288`), so edge functions wait (section 1.7) | 0 | 0 | Panacea |
| 59 | Build pipeline, scans and posture gate, run by Panacea | Ours to run (row 114); SAST on baked rules ends its false pass | 0 | 0 | Panacea |
| 60, 68 | `panacea-mcp`: the agent deploy loop, and a copy in every sandbox image | Unchanged, pointed at our Panacea through G4 | 0; 0.25 | 0; 0.25 | Panacea |
| 61, 62, 65, 66 | G1 Deploy page; G2 provenance; G5 project create; phase 7's connector | Unchanged, over our Panacea's API. The connector, in the laptop's control plane, calls through G4's proxy with the person's realm token, inside row 112 | 2.75; 1.25; 0.875; 1.7 | the same | Ours; OSS; Panacea; Ours |
| 63 | G3 promotion by data class; the class-mismatch refusal not costed | Unchanged; the refusal is coded in our copy (row 113) | 0.75 | 0.75 | Panacea |
| 64 | G4 allowlist proxy, holding the user's `pnc_` | Unchanged in days. It adds the person's realm token server-side, strips identity headers callers send, holds no key, and carries phase 7's client and row 127's pass-through too | 1.5 | 1.5 | Ours |
| 67 | Chains and Kyverno, audit only; Chains signs only while Vault is unsealed | Ours to run. Chains signs through OpenBao with Key Vault auto-unseal, so a restart no longer stops signing. Still audit only in v1 | 0 | 0 | Panacea |
| 70 | P5: six asks of Panacea's stream | Only `OD-44`'s realm block and our egress address on its edge are asked of today's Panacea, both declared to its move's inventory with a sign-in check on its flip (section 3.1); nothing at all if question 4's line (a) lands | 0.5 | 0.5 | Panacea |
| 71 | P6: live proof on Panacea dev | **Retired.** The proof runs on our estate (row 117) | 0.5 | - | Panacea |
| 73 | Panacea's steps in the cockpit: team-request state, Connect deploy, our error copy | Connect deploy goes; the team-request state and our error copy stay | 0.5 | 0.25 | Ours |
| 74 | The Hub, including tasks 29 and 39: a pasted-token store and routes for GitHub, Azure DevOps and Panacea tokens | Their pasted-token parts go: the person's GitHub token comes from row 134, Azure DevOps' from row 136, and no Panacea token exists. The store keeps the GitHub token | 9.5 | 9.25 | Ours |
| 78 | Our environment, "OpenTofu from Panacea's `.tf`" | Unchanged. "Panacea's `.tf`" means `deploy/*.tf` only, never `terraform/` | 1 | 1 | Managed |
| 81, 82 | Panacea's realm as the identity source; our Keycloak brokering it | Unchanged under `OD-44`. Our realm gains the `panacea-deploy` audience, a `groups` mapper for it and for the previews' sign-in client, the realm-scoped account our Panacea manages groups through, and a test account for the proofs, all inside row 112. Shipped apps' own users stay in our realm as `app-<project>-users` groups, as in Panacea | 0; 2 | 0; 2 | Panacea; OSS |
| 95, 96 | The realm hand-off; share-link previews, with no end condition | Unchanged in days. The hand-off now gives our previews a session, `protected` ones included; share links stay, for showing a preview to someone who has no session | 1.5; 0.25 | the same | Ours; Panacea |
| 104, 105 | The provider page; cost codes sent to Panacea | Unchanged, over our Panacea. Shipped apps' AI joins the provider page (row 132) | 2.5; 1.75 | the same | Ours |
| 106 | In-app PR review | Unchanged. It reads with the person's own GitHub and Azure DevOps tokens from the authenticator; until IT's consent, Azure DevOps comments may post through our identity naming the person (question 4) | 3 | 3 | Ours |
| 107, 108, 109 | App settings; team settings and approvals; production approval, over Panacea's API | Unchanged, over our Panacea's API. On CloudNativePG, row 107's backup, back-up-now and revert work as on Panacea dev. Row 109's step-up gate is now our own setting | 4.5; 1.5; 0.5 | the same | Ours |

**Net of the changed rows: -1 day** (rows 71, 73 and 74). Row 71 leaves the row count.

### 1.3 New rows: our own Panacea

| # | Capability | Replaces | Kind | Low | Mid | High | Basis |
|---|---|---|---|---|---|---|---|
| 110 | **The lift.** The trees of section 0 item 2, from `ado-gov/dev` at one pinned commit, with the engine's share of `deploy/` traced from what `panacea-deploy.sh` and `cloud-shell-deploy.sh` read, never Panacea's env files, `terraform.tfvars`, its root pipelines or its ad-hoc and production-operations scripts. Attribution headers by script, with a ledger-only rule for files that cannot carry one (JSON, golden files, fixtures); one ledger row per tree with its source commit; a re-sync script that re-applies the headers, so the weekly pull stays mechanical; a CI check that fails if a production name (`rg-panacea-platform-dev`, `aks-panacea-dev`, `panaceacr`, `stpanaceatfstate`, `panacea.dev.tfstate`) appears anywhere in the copied code. The modules build outside our `go.work`, at their own pins (Go 1.25 and `client-go` v0.29.3, `go.mod:3`, `:14-16`, against our Go 1.23 and v0.31.1, `go.work:1`, `control-plane/go.mod:10`), into our registry, with CI running their tests, including the three suites Panacea's CI never ran (the static gateway, the analytics injector, the pipelines) | the source of rows 57 and 68; extends row 72 | Panacea | 1.75 | 2.75 | 4.5 | About 1,900 files: 1,813 outside `deploy/` at `ca92e83eb` (`git ls-tree`), plus the engine's share of `deploy/`'s 184. Scouts: 1.5 / 3 / 5.5 (inventory, snapshot plus build), 1 / 1.5 / 2 (options); plus 0.25 for the trace and the production-name check (verify round) |
| 111 | **Our apps estate.** A second AKS cluster in its own resource group, Australia East, by Panacea's engine with a sixth env file and a new `ESTATE` and `ENVIRONMENT` value no Panacea estate uses: its own registry, Key Vault and OpenTofu state; the in-cluster data plane (row 58); dev's gates (section 0 item 4). **The engine has no switch to skip what we leave out, so this row edits it:** `deploy/portal/deploy-portal.yaml`, which also defines `panacea-api`'s Deployment, ServiceAccount, ClusterRole, Service and HPA (`:386-389`, `:910-1140`) and is applied whole by phase 9 (`cloud-shell-deploy.sh:4735`), split so only `panacea-api` and the oauth2-proxy route stay, in the `panacea-portal` namespace that `panacea-api`'s own addresses hard-code (`clients/ai_env_names.go:9`, `clients/ingress.go:1234`, `handlers/connection_envvars.go:17`); `forge-orchestrator` dropped from phases 8 and 9 (`:4143-4144`, `:4888`); the `keycloak` helm release dropped (`deploy/main.tf:915`); phase 12 split so oauth2-proxy, pointed at our realm, and the final PSA re-assert still run without the realm setup (`:5829-6017`); every production default in the engine (`:45-54`, `:172-174`) made to fail when unset, with its fail-closed estate guard kept (`:56-66`), and `DNS_AUTO_DELEGATE` left false (its default) so it never writes into the parent zone. Also: the VNet range made a variable (every Panacea estate is `10.1.0.0/16`, `deploy/main.tf:52-57`) and privately peered with our platform's; a delegated subzone with wildcard certificates by DNS-01, with a CoreDNS block scoped to that subzone and forwarding every other name (the engine bakes one that rewrites every `*.panacea.kpmg` name and has no forward, `:604-642`; Panacea's 09-28 fix, `b78699c81`, is not in `ca92e83eb`) and cert-manager's DNS-01 check sent to Azure's resolver; the 14-address edge (`deploy/variables.tf:278-297`); the Git-sourced Argo CD applications and the registry name pointed at ours (the name is rewritten at apply, `cloud-shell-deploy.sh:212-213`); the runtime scanners as the engine runs them on dev (phase 15.5, `:6319`); at the edge, before any app an agent wrote, the parent zone's `_panacea_oauth2` cookie stripped from requests and any `Set-Cookie` for the parent domain dropped from responses; pools at the engine's own system and workload shape with build machines from zero, one warm CI node and capped maxima, and its quota pre-flight following those minimums (`deploy/panacea-deploy.sh:518-523`); up to a green lifecycle canary | the runtime of rows 57 to 59 | Panacea | 10 | 15 | 26 | Panacea's own dev estate on this engine took eight bootstraps, three finalize runs and about a dozen fixes over three days to first light (2026-08-17 to 08-19, the owner's session notes), then estate fixes for five more weeks (287 commits touched `deploy/` from 2026-08-17 to 09-25). The engine is 6,427 lines: 248 mention Keycloak, 220 the Portal, 197 LiteLLM, 77 Forge. This shape has never run: no realm, no Portal, no Forge, OpenBao, our pipeline. Scouts: 10 / 18 / 30 (inventory), 4 / 5 / 7 (options). Verify round: 9 / 14 / 24 for the engine's edits, plus 1 / 1 / 2 for the defaults, CoreDNS and the cookie rule |
| 112 | **No key: our realm is our Panacea's only identity for people.** A bearer-token branch in our copy's auth middleware calling Panacea's exported `VerifyBearerToken` (`middleware/auth.go:444-457`, which has no caller yet, `:449`), set to our realm's JWKS address, issuer and a `panacea-deploy` audience (`middleware/jwks.go:147-153`), its unverified-decode grace at zero and its no-JWKS fail-open closed (`jwks.go:44-68` describes both); the header-asserted branch (`auth.go:188-216`) and legacy `panacea_` project keys (`:231-242`) removed, so the shared transport secret carries only in-cluster service calls; key minting unrouted (`routes/routes.go:2984-2986`, `:3092-3094`); the key-derivation secret split from the transport secret (`clients/minio_admin.go:356-364`, `handlers/provisioner.go:871`, `clients/pipeline.go:1011`; Panacea's S-9); Panacea's acceptance, lifecycle and onboarding suites (row 117) moved from the shared secret and asserted `platform-admins` headers (`deploy/acceptance/run-acceptance.sh:78-81`, `run-lifecycle.sh:88-91`, `run-onboarding-access.sh:117-122`) to a test account's realm tokens; the Keycloak admin client moved from the master-realm password grant (`clients/keycloak.go:35-45`, `:70-77`) to a service account scoped to our realm, where teams, project groups and shipped apps' `app-<project>-users` groups now live (`handlers/project_auth.go:68-75`), app users signing in through Entra as everyone else does, so `/auth/verify` keeps its one oauth2-proxy (`handlers/auth_verify.go:21-28`); `groups` on the `panacea-deploy` audience and the previews' sign-in client only, named as Panacea's code expects (`middleware/entra_claims.go:159-162`), and the username taken from the email (`auth.go:604-641`); KPMG literals moved to configuration (the `@kpmg.com.au`-only registration gate, `handlers/register.go:41-42`; three named approvers, `handlers/team_create_approval.go:58-60`); previews' sign-in pointed at our realm with issuer checking on (`kubernetes/helm-values/oauth2-proxy-values.yaml:79-81`); a security review (section 3.4) | forced exception 3 (Connect deploy); P-3; the legacy-key guard | Panacea | 4 | 6 | 9 | Scouts: 4.5 / 7.5 / 12 (inventory items 3 and 6), 2 / 2.5 / 4 (options). P-3 alone was judged 2 to 3 of Panacea's days with its review (`00-panacea-verify.md` §4.1). Verify round: plus 1 / 1.5 / 2 for the bearer branch, the header-path and legacy-key removal, the suites' tokens and the secret split; a second realm for shipped apps' users (a second admin client, a second oauth2-proxy and a gate accepting two issuers) is not taken |
| 113 | **Panacea's pending asks, now changes in our own copy:** P-1, the brief pin on every release of an app-made project, and a refusal of a deploy that carries none; P-2, the data-class route; the refusal to promote a project whose class annotation differs from its brief's (a named gap of `00-plan.md` §1); and the MinIO path's bucket takeover closed: a new project named `<victim>-files` can claim bucket `victim-files` (`clients/minio_admin.go:326-345`), which Panacea leaves open (`PLAN.md:440-441`) | P-1 and P-2 (day 0 item 3); the named gap | Ours | 1.5 | 2 | 3 | `06-deploy.md:127-128` (P-1 0.5 to 1, P-2 about 0.5); the refusal 0.25 (coexistence scout); the bucket rule 0.25 / 0.25 / 0.5 (verify round) |
| 114 | **The deploy path on our estate.** `panacea-build` and its step images, built into our registry; Tekton Pipelines and Triggers pinned (installed from `latest` today, `cloud-shell-deploy.sh:168-169`, `:5435-5451`); about 20 tool and scanner images mirrored; SAST on rules baked into its image, with a real error path (`pipelines/tasks/sast-scan.yaml:79`, `:118-128`); `webhook-receiver`, with pushes reaching it through Panacea's own deploy poller (`DEPLOY_POLLER_ENABLED`, off by default because today's Panacea takes webhooks, `cloud-shell-deploy.sh:69-71`), because a 14-address edge admits no GitHub or Azure DevOps delivery; Azure DevOps pushes routed to build-deploy in our copy, because Panacea sends every `azuredevops` push to agent-deploy (`services/webhook-receiver/pipeline/trigger.go:1511-1517`, `:1556-1562`), whose `finally` task calls Forge and fails the run when it cannot reach it (`pipelines/pipelines/agent-deploy.yaml:362-369`, `pipelines/tasks/agent-report.yaml:184-188`); the static gateway on MinIO; build-deploy, preview-deploy and every scanner proven in warn mode | the runtime of row 59 | Panacea | 4.25 | 7.75 | 14 | Inventory items 7 to 10: 4 / 7.25 / 13.5; plus 0.25 / 0.5 / 0.5 for the Azure DevOps route (verify round). Sizes at `ca92e83eb`: `panacea-build` 11,536 Go lines, pipelines 21,025 YAML lines, `webhook-receiver` 4,206, the static gateway 1,232 |
| 115 | **One AI gateway for agents and shipped apps.** `panacea-api`'s per-project and per-branch model keys minted against our gateway (rows 76, 76b) instead of the LiteLLM Panacea builds today (`main-v1.40.0`, `cloud-shell-deploy.sh:709-710`), with the project's codes on each key (row 105) | - | Panacea | 1 | 2 | 3 | Inventory item 5: 1 / 2 / 3 |
| 116 | **The licence swaps inside the lift.** OpenBao for Vault (`OD-32`): KV v2, Transit for Chains' signer, Key Vault auto-unseal, under the engine's Vault steps (phase 11.5, `cloud-shell-deploy.sh:5647`); Valkey 8 for tenant caches through the existing override (`PANACEA_REDIS_IMAGE`, `clients/tenant_redis.go:65-70`); `madmin-go` replaced by a small admin client of ours for its five calls, behind the interface the code already has for tests (`clients/minio_admin.go:58-62`), written from MinIO's Apache-2.0 SDKs' admin clients and tested in CI against the pinned MinIO; OpenTofu running `deploy/*.tf` | - | OSS | 2 | 3.25 | 5 | Options scout: 1 / 1.5 / 2.5 for `OD-32` inside the lift; the Redis swap and OpenTofu 0.25 / 0.5 / 0.5; the admin client 0.75 / 1.25 / 2 (verify round; it was costed as deleting `madmin-go`'s uses, which only the managed plane allowed) |
| 117 | **Live proof on our estate.** Panacea's own acceptance, lifecycle and onboarding suites (`deploy/acceptance/run-acceptance.sh`, `run-lifecycle.sh`, `run-onboarding-access.sh`; wired in `azure-pipelines-dev.yml:61-63`), run from our pipeline with a test account's realm tokens (row 112), plus one DNS-01 issuance and one forced certificate renewal, before the first app's walk | row 71 | Panacea | 1.75 | 2.75 | 4.5 | Inventory item 13; plus 0.25 for the certificate proof (verify round). Panacea dev's lifecycle canary last ran 17 pass, 3 warn, 0 fail (`docs/cutover/RESUME.md:6-7` at `3d45587f0`) |
| 118 | **Ops readiness.** Certificate-expiry, OpenBao-seal, backup, MinIO-volume and quota alerts on the estate's own monitoring, and runbooks. Panacea production's certificate renewals failed silently for about six weeks before a break-glass fix on 2026-09-28 (`06-deploy-verify.md` §2.4) | - | Panacea | 1 | 1.5 | 3 | Inventory item 14 |
| 119 | **The weekly re-sync during the build.** Panacea's fixes pulled from `ado-gov/dev` into our copy about once a week, for about ten weeks (section 2.6). In the 30 days to `ca92e83eb`, `ado-gov/dev` took 713 commits, 412 of them in the lifted trees (639 files, about +61,000 / -9,900 lines), and most in the files our copy patches: `cloud-shell-deploy.sh` 64, `panacea-deploy.sh` 49, `clients/kubernetes.go` 40, `routes/routes.go` 33, `handlers/provisioner.go` 15, `main.go` 13. Panacea's in-flight move (10 slices, about 72.5 engineer-days, `PLAN.md:27`) lands in the same files during our build | - | Panacea | 5 | 10 | 20 | 0.5 / 1 / 2 a week (the inventory scout's rate; the verify round set aside the options scout's 0.25 / 0.5 / 1) for 10 weeks. Counts re-measured with `git log` and `git diff --shortstat` |
| 120 | **Teams copied once at go-live.** Teams, members and cost centres read once from today's Panacea's team API into ours, by the build team with its own access there, keeping no key. After it, a team has two homes until the merge (question 2); the first app's walk uses one team made by hand in ours | - | Ours | 0.5 | 1 | 2 | Inventory scout |

**Sum of rows 110 to 120: 32.75 / 54 / 94.**

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
are also the only way to reach these functions at all. `00-plan.md` §5.2 now notes that its Portal
verdict fails, so the plan of record no longer reads as passing; its total is re-planned at
sign-off.

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
| 127 | **One allowlisted, audited pass-through** from Hub pages to our Panacea's API: identity added server-side, incoming identity headers stripped, SSE flushed, errors in our copy. It shares G4's proxy (row 64), so about 250 routes need no connector each | - | Ours | 0.75 | 1 | 1.5 |
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
| 134 | **GitHub as the person:** our App's user authorisation, chained into the first-run tab (one "Authorize" click, once); 8-hour tokens with a rotating 6-month refresh, kept in Windows Credential Manager (`OD-6`). It replaces the Hub plan's pasted GitHub token (`feat/hub:docs/superpowers/plans/2026-09-23-phase-h-hub.md:566`, Q9; tasks 29 and 39 at `:432`, `:442`), so row 74 gives back 0.25 | the Hub's pasted GitHub token | Ours | 0.75 | 1 | 1.5 | GitHub's documented web flow and refresh lifetimes, as the identity scout read them. Background installation tokens are already rows 45, 46 and 49 |
| 135 | **Azure DevOps background identity:** the managed identity row 45 already adds to write Proof (`05-work-team-verify.md` §8 piece 1, `:327-330`), given Basic access instead of Stakeholder so it can read code; the sandbox read proxy and builds use it, never a person's token. It sits in tenant `eb9a11c9…` because Panacea's subscription does, the tenant `kpmgcomau` answers from and the one our own organisation is created in (day 0) | the PAT the read proxy held (`05-work-team-verify.md` §8 piece 3) | Managed | 0.25 | 0.5 | 1 | Identity scout. Row 45 already carries the identity and its token, so this row is the access and its two new users (verify round; was 0.5 / 1 / 1.5) |
| 136 | **Azure DevOps as the person:** a silent token from Windows' account broker in our build's main process, on a Panacea app in tenant `eb9a11c9…` holding Azure DevOps' delegated permission, consented once by an admin there (question 4, IT request 2). Until then, phase 3's planned git helper asks Git Credential Manager for a Microsoft token on each call (`feat/phase-3-git:docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-13.md:97-116`). If both fail, comments post through our managed identity (row 135), naming the person in the comment and in our audit, and a vote Azure DevOps must count as the person's waits. No token is pasted (question 4) | forced exception 2 | Ours | 1.75 | 2.5 | 4 | Identity scout: the bridge 0.25 / 0.5 / 1, the broker path 1.5 / 2 / 3. `00-plan.md` §5.2 gave an in-app token flow about 1 day once consent exists. The service-identity fallback takes the place of the paste page it drops |
| 137 | **Keyless builds:** every build of our Panacea clones with a credential minted for that run, a GitHub App installation token (minted as rows 45 and 46 already do) or our managed identity's token, never a stored one; the stored-token routes, per project, per team and per person (`routes.go:1321-1323`, `:2028-2033`, `:3061-3069`), are unrouted in our copy. Today a build clones with a stored token (`pipelines/tasks/git-clone.yaml:47-49`) | Panacea's stored git tokens | Ours | 2 | 2.75 | 4.25 | Coexistence scout 2 / 3 / 4, user-functions scout 2.5 / 3.5 / 5; counted once, here; less 0.25 for the token minting rows 45 and 46 already build (verify round) |

**Sum of rows 133 to 137: 5.75 / 8.25 / 12.75.**

### 1.6 The forced exceptions and the named gaps

| `00-plan.md` §5.2 | What happens to it | Rows |
|---|---|---|
| 1. The sign-in tab, on first run and weekly | Accepted by the owner, and unchanged. It now also carries GitHub's one Authorize click, once | 134 |
| 2. An Azure DevOps token, pasted once a person | Gone: nothing is pasted. Until IT's consent, the laptop's own sign-in; if that fails too, comments post through our identity naming the person, and a vote that must be the person's waits (question 4) | 133, 135, 136, 137 |
| 3. Connect deploy, in the system browser | Gone: no key exists to mint or paste | 112 |
| 4. Step-up acts on the Portal | Gone: there is no Portal, and the step-up map is ours | 112, 109, 130 |
| **5. New: older apps on today's Panacea website, until the merge** | **Kept, forced.** Reaching today's Panacea from the app needs a key, which `OD-53` rules out, or a change there behind its move's freeze (its own realm token trusted, Panacea's P-3). It ends at the merge. The alternative is priced in question 2 | - |

**Named gaps of `00-plan.md` §1.** Closed: the Deploy refusal of a class mismatch (row 113), and a
`protected` preview inside the frame, by design (rows 95 and 112; not verified). Unchanged: the
preview's branch-data mode (on CloudNativePG, Panacea's branch-data code, `clients/db_branch_ops.go`
and `branch_data_ops.go`, exists and was not traced); and a repository and a tracker for a person
who starts from nothing, whose home is our own Azure DevOps organisation (day 0). **Panacea's own
gaps that become ours, unsized:** release signing stays audit-only in v1 (row 67); the production
rung still waits on DLP enforcing for the project, as it does in Panacea (`06-deploy.md`, P-5); and,
at the pilot's gates, the registry is pulled with its admin credential, as on Panacea dev
(`NO_RBAC_MODE`, `deploy/environments/panacea-dev.env:89-92`), until the production estate's
role-assignment identity exists (section 1.7).

### 1.7 Outside the total

- **The merge into one Panacea**, after Panacea's production move signs off: its history imported,
  its pipelines and Argo CD sources re-pointed here, the 09-28 CoreDNS hotfix carried first (Argo
  CD reverts it otherwise), today's estates deploying from this repo, the app fronting them, the
  managed data plane once Panacea's move has built it (its SL-3 and SL-5), and the Portal retired
  on the owner's date. The source move alone is about 3 to 6 days (coexistence scout); the rest is
  sized when the move signs off, which has no date yet.
- **The weekly re-sync after the build**, until the merge: about 1 engineer-day a week (0.5 to 2),
  the rate row 119 uses.
- **Our realm brokering Microsoft directly** (question 4, line a): about 0.5 (0.25 to 1), if the
  owner agrees and IT acts.
- **The Windows-native sign-in that removes the weekly tab:** about 2 (1.5 to 3), only if its day-1
  spike passes. It needs lines (a) and (c): while our realm brokers Panacea's, a Windows token has
  no linked provider to grant against (identity scout).
- **A production estate** beside the pilot's, before the pilot widens past class `none`: a seventh
  env file at `aue`'s gates (Azure Firewall, web firewall, the paid cluster tier, Defender,
  customer-managed keys), the managed data plane once Panacea's move has proven it, and an identity
  with role-assignment rights scoped to its resource group (RBAC Administrator, constrained),
  granted by a subscription owner (`east-infra.md:543`), never Panacea's deploy principal. Not sized
  in days. Running cost about US$3,300 to 5,200 a month more: at list, the firewall US$912.50, the
  web firewall's fixed part US$381 and the paid tier US$73 a month, plus Defender at about US$7 a
  vCPU a month, which also changes billing for the whole subscription, Panacea's estates included
  (`east-infra.md:536-537`); on the pilot's pools at the low end, and at the high end on the
  engine's full floor (56 vCPU, US$2,649 a month in machines) with the managed plane's
  zone-redundant Flexible Server, which every environment hard-codes
  (`deploy/postgres-flexible.tf:269`; about US$356 a month for two vCores); logs and the rest
  inferred.
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
| Rows 110 to 120, our own Panacea | 32.75 | 54 | 94 |
| Rows 121 to 129, Panacea's user screens | 18.7 | 28.2 | 41.25 |
| Rows 130 to 132, the operator console and shipped apps' AI | 12.25 | 17.75 | 24.5 |
| Rows 133 to 137, the authenticator | 5.75 | 8.25 | 12.75 |
| Changed rows 71, 73, 74 | -1 | -1 | -1 |
| **Net change** | **68.45** | **107.2** | **171.5** |
| **The plan** (132.275 before) | | **239.475, about 239** | |

**By cause, mid:** our own Panacea 71 (rows 110 to 120 and 130 to 132, less rows 71 and 73's 0.75);
the authenticator 8 (rows 133 to 137, less row 74's 0.25); Panacea's user screens owed under
`OD-49`, 28.2. **Range:** summing every row's low and every row's high (68.45 and 171.5) assumes
every estimate errs the same way. The estate (row 111), the re-sync (row 119) and the pages are the
least certain, so the net is taken as **85 to 140**, which with `00-plan.md`'s 120 to 150 gives
**about 205 to 290**.

## 2. The effect on the totals

### 2.1 Before and after

"Before, with the owed screens" adds rows 121 to 129 (28.2 days, nine rows, all ours) to
`00-plan.md`'s figures, because `OD-49` already owed them; it is the like-for-like base. Calendar
figures assume each builder delivers 85 to 95% of an engineer-day a working day; "at least" is the
floor at 100% (section 2.6).

| Measure | Before (`00-plan.md`) | Before, with the owed screens | After (this proposal) |
|---|---|---|---|
| Remaining work | 132.275 engineer-days (120 to 150) | 160.475 (about 140 to 190) | 239.475 (about 205 to 290) |
| Capability rows | 110 | 119 | 137 (28 new, row 71 retired) |
| Done | about 11% (10 to 14) | about 9% (8 to 12) | about 6% (5.5 to 8) |
| By capability: open source / Panacea / managed / ours | 35% / 15% / 13% / 37% | 32% / 14% / 12% / 42% | 28% / 18% / 11% / 43% |
| By capability: adopted, together | 63% | 58% | 57% |
| By effort: glue on open source / Panacea / managed; ours | 24% / 4% / 11%; 61% | 20% / 4% / 9%; 67% | 15% / 22% / 6%; 57% |
| By effort: glue on adopted pieces, together | 39% | 32% | 43% |
| First app, five builders | about 13 to 17 working days (holds only if rebalanced, section 2.6) | the same: the screens come after it | about 22 to 25 (at least 20) |
| All of it, five builders | about 26 to 27 (below its floor of 26.5; about 28 to 31 by this file's method) | about 34 to 38 (at least 32) | about 50 to 56 (at least 48) |
| With a sixth builder for the first three weeks | - | - | first app about 19 to 22; all of it about 47 to 53 (at least 45) |
| First app, two builders | about 5.5 to 6.5 weeks (about 7 to 8 by this file's method) | the same | about 11 to 12 weeks |
| All of it, two builders | about 13 weeks (its floor; about 14 to 16 by this file's method) | about 17 to 19 weeks | about 25 to 28 weeks (at least 24) |
| New running cost | none | none | about US$1,400 a month in machines at pilot size, about US$1,700 to 2,100 all-in; the production estate, later, about US$3,300 to 5,200 a month more |

### 2.2 Percent done: about 6%

**Method,** as `00-plan.md` §2.1: banked days over banked plus remaining. Nothing has been built
since that file, so the banked days stay 16.5.

**Arithmetic.** 16.5 / (16.5 + 239.475) = 16.5 / 255.975 = **6.4%, about 6%.** Phase 2's backend
at half (2.5): 14 / 253.475 = 5.5%; at double (10): 21.5 / 260.975 = 8.2%. Hence about 5.5 to 8%.
Like for like, with the owed screens in the before: 16.5 / 176.975 = 9.3%. Done falls from 11.1%
only because the remaining work grows: by 28.2 the plan already owed, and by 79 for our own
Panacea and the authenticator.

### 2.3 By capability: 137 rows

| Kind | Rows | Share |
|---|---|---|
| Open source | 39 | **28%** |
| Panacea | 24 | **18%** |
| Managed service | 15 | **11%** |
| **Adopted, together** | **78** | **57%** |
| **Ours** | **59** | **43%** |
| Total | 137 | 100% |

**Arithmetic.** Before: 38 / 17 / 14 / 41 of 110. Row 71 leaves (Panacea, 17 to 16). The 28 new
rows are 1 open source (116), 8 Panacea (110, 111, 112, 114, 115, 117, 118, 119), 1 managed (135)
and 18 ours (113, 120, 121 to 134, 136, 137). After: 39 / 24 / 15 / 59 of 137. 39/137 = 28.5%,
24/137 = 17.5%, 15/137 = 10.9%, 59/137 = 43.1%. Rounded by largest remainder so the four add to
100: floors 28, 17, 10, 43 = 98; the two largest remainders, 0.95 and 0.52, go to managed services
and Panacea. Adopted is 78/137 = 56.9%. Like for like, the before with the owed screens' nine rows
is 38 / 17 / 14 / 50 of 119: 32% / 14% / 12% / 42%, adopted 58%.

**Challenge it here:** the count depends on how finely capabilities are cut. The user screens are
cut into six rows and the operator console into two. Cut one row per Portal group instead, about 15
more rows of ours, and ours would be about 49% (74 of 152).

### 2.4 By remaining effort: 239.475 days

| Where the days go | Days | Share |
|---|---|---|
| Glue on open-source pieces | 34.9 | **15%** |
| Glue on Panacea | 53.125 | **22%** |
| Glue on managed services | 15 | **6%** |
| **Glue on adopted pieces, together** | **103.025** | **43%** |
| **Our own code** | **136.45** | **57%** |
| Total | 239.475 | 100% |

**Arithmetic.** Before: 31.65 / 5.875 / 14.5 / 80.25 of 132.275. Open source +3.25 (row 116), to
34.9. Panacea -0.5 (row 71) and +47.75 (rows 110, 111, 112, 114, 115, 117, 118, 119: 2.75 + 15 + 6 +
7.75 + 2 + 2.75 + 1.5 + 10), to 53.125. Managed +0.5 (row 135), to 15. Ours -0.5 (rows 73 and 74)
and +56.7 (row 113, 2; row 120, 1; rows 121 to 129, 28.2; rows 130 to 132, 17.75; row 133, 1.5; row
134, 1; row 136, 2.5; row 137, 2.75), to 136.45. Shares: 34.9/239.475 = 14.6%, 53.125/239.475 =
22.2%, 15/239.475 = 6.3%, 136.45/239.475 = 57.0%. Rounded by largest remainder: floors 14, 22, 6,
56 = 98; the two largest remainders, 0.98 and 0.57, go to ours and open source. Adopted is
103.025/239.475 = 43.0%. Open-source glue by product: `00-plan.md` §2.2's list, plus OpenBao,
Valkey, OpenTofu and the MinIO admin client's share, 3.25. Like for like, the before with the owed
screens is 31.65 / 5.875 / 14.5 / 108.45 of 160.475: 20% / 4% / 9% / 67%, adopted 32%.

**What moved.** Panacea's share of the effort grows from 4% to 22% because running our own Panacea
is glue on Panacea's code, and its two largest rows, the estate (row 111) and the weekly re-sync
(row 119), are exactly that. Our own share falls from 61% (67% like for like) to 57%. **Challenge it
here:** counted as ours, the changes row 112 makes in the copied code (6) would put our own code at
about 59%.

### 2.5 By size of software delivered: measured for the lift, not a percentage

The lift brings 195,166 lines of non-test Go (`panacea-api` 177,086, `panacea-build` 11,536,
`webhook-receiver` 4,206, the static gateway 1,232, `panacea-shared` 655, the analytics injector
451) and 194,899 of tests; 21,025 lines of pipeline YAML; 15,937 of Kubernetes YAML (all of
`kubernetes/` but Forge's preview mirror, 345); 13,293 of engine shell (the two engine scripts and
the libraries, gates, suites and Portal scripts they read, of `deploy/`'s 26,988); and 4,489 of HCL
(`git grep -c` at `ca92e83eb`). Our own kept Go on this branch is about 17,900 lines (`00-plan.md`
§2.2 (iii)). By size, Panacea's copied code is about eleven times ours. It is still not a
percentage, because lines are not capabilities.

### 2.6 The calendar

**Method.** A builder delivers at most one engineer-day a working day, so five deliver at most five:
a floor, not a forecast. The figures below assume 85 to 95% of that, and a first-app path whose
work is spread so no builder carries much more than the average (section 3.3).

- **First app, five builders: about 22 to 25 working days** (was 13 to 17). About 34.5
  engineer-days join the first app's path: rows 110 (2.75), 111 (15), 115 (2) and 116 (3.25) whole,
  and the first-app shares of row 112 (about 4.5), row 114 (about 5.25), row 117 (about 1.25) and
  row 137 (about 1.25, GitHub's half), less row 71 (0.5) and row 73's Connect deploy share (0.25).
  With row 74's 0.25 back, the path holds about 100.7 engineer-days (the plan of record's 66.45
  plus 34.25): at least 20.1 working days with five builders. Section 3.3 spreads it so the
  most-loaded builder carries about 21.2, which at 85 to 95% is about 22 to 25. The estate track
  (rows 110, 111 and 117 on B2, about 19) is no longer the pole on its own: the Hub (B4) and the
  gateway and Deploy work (B5) carry as much. **The plan of record has the same imbalance:** its B5
  holds about 19.7 engineer-days of first-app work (rows 76 and 76b, 104, about 5 of Deploy glue,
  82, 95) against 13 to 17 working days, so its 13 to 17 holds only with the first-app work spread
  across all five, as here (`00-plan.md` §3.3 now notes it).
- **All of it, five builders: about 50 to 56 working days** (was 26 to 27). 239.475 / 5 = 47.9 at
  least; at 85 to 95%, 50.4 to 56.3. `00-plan.md`'s 26 to 27 (08-verify's 20 to 21 working days for
  103 engineer-days, scaled) sits below its own floor of 26.5 (132.275 / 5); by this method it is
  about 28 to 31.
- **With a sixth builder for the first three weeks** (question 1): B6 takes row 116, all of row 112,
  about 4 of row 111's engine edits and the Desk's hook relay and thread drawer (rows 91, 100), about
  14 days in its three weeks; no first-app builder then carries more than about 18.5, so the first
  app comes in about 19 to 22 working days. All of it: at least 44.9 (15 days at six, then
  149.475 / 5 at five); about 47 to 53 at 85 to 95%. The sixth lifts the owner's five-builder cap
  (call 1) for those three weeks.
- **Two builders:** the first app in about 11 to 12 weeks (100.7 engineer-days: at least 50.4
  working days, 53 to 59 at 85 to 95%); all of it in about 25 to 28 weeks (at least 119.7 working
  days, 126 to 141).
- **Rows 113, 118 and 120 to 136, and the rest of rows 112, 114, 117 and 137, sit after the first
  app,** as rows 106 to 109 already do: the first app's journey has one author and a class-`none`
  pilot, and its walk uses one team made by hand. Row 119 runs weekly throughout, for about ten
  weeks.
- **Risk to the dates:** the org's monthly spend limit stopped every agent run on 2026-09-30,
  Panacea's included (`docs/cutover/RESUME.md:5` at `3d45587f0`). A longer build at five or six
  streams spends faster, and these dates assume no such stop.

## 3. Build order and day 0

### 3.1 Day 0

Against `00-plan.md` §3.1's items:

1. **Item 1 grows: a second resource group in Panacea's subscription,** for the apps estate,
   tagged and named apart from every Panacea estate (the five env files' names and the move's
   `devaue`), with vCPU quota in Australia East. Our apps estate starts at 32 vCPU (B-series 20: one
   system and four workload B4ms nodes, the workload minimum `deploy/main.tf:424` hard-codes;
   DDSv5 8: one CI D8ds_v5; ESv5 4: one E4s_v5) and is capped at 76 (build and CI pools at two
   D8ds_v5 each, through the engine's own `build_max_count` and `ci_max_count`); each build machine
   adds 8 while it runs. The quota is per subscription and shared: our Kata pool takes up to 32
   more (`infra/terraform/dev.tfvars:5-7`), and each of Panacea's two Australia East estates starts
   at 56 and reaches about 188 at autoscale maximum (`east-infra.md:515-517` at `3d45587f0`). So
   the request covers about 376 for the move plus at least 108 for us, after reading what is free
   (`az vm list-usage -l australiaeast`), and our build pool stays at its minimum during the move's
   windows.
2. **Item 3 shrinks.** Of today's Panacea we ask only what `OD-44` already needs: its realm block,
   and our egress address on its edge for that realm, a Terraform variable there
   (`edge_allowed_ip_ranges`), so it needs the owner's yes during the move's freeze. P-1 to P-3, the
   legacy-key guard, the SAST fix and the step-up map are no longer asked; they are our own changes
   now (rows 112 to 114). **Both are declared to the move's inventory, so its DNS flip keeps our
   sign-in working:** the realm block is applied before the move's final Keycloak copy and checked
   by its `kc-parity`, our egress address goes on East's allowlist before the flip, and a sign-in
   through our realm joins the flip's checks (`PLAN.md:262`, `:553`). If the owner agrees to
   question 4's line (a), the realm block goes too, and with it the stall risk on sign-in.
3. **New: one record delegating `apps.panacea.kpmg`** (question 3) in the authoritative
   `panacea.kpmg` zone, never the same-named, undelegated decoy
   (`deploy/environments/panacea-dev.env:41-46`). "apps" is reserved in today's Panacea for team
   prefixes, project names and Pages names alike (`handlers/team_subdomain.go:35`, `:48-55`), so no
   existing project can clash. The owner's own account wrote dev's delegation this way on
   2026-08-19. It goes in outside the move's read-only windows and soaks, and is declared to the
   move's DNS inventory.
4. **New: the apps estate's pipeline,** in our own Azure DevOps organisation (`OD-48`): a service
   connection whose principal holds Contributor on the apps resource group only. At the pilot's
   gates the engine needs nothing more: like Panacea dev it runs Contributor-only (`NO_RBAC_MODE`,
   `panacea-dev.env:89-92`), pulling from the registry with its admin credential and granting the
   unseal Key Vault by access policy (`deploy/panacea-deploy.sh:761-822`). The production estate
   needs more (section 1.7). Never Panacea's deploy principal, and never run in Cloud Shell as the
   owner, whose account reaches production. One paid parallel job, because a bootstrap runs past
   the free 60-minute cap (Panacea's dev launch, 2026-08-18).
5. **New, for the authenticator:** our own Azure DevOps organisation created connected to tenant
   `eb9a11c9…`, the tenant `kpmgcomau` and our subscription sit in, so one managed identity and
   one app serve both; an owner of `kpmg-au-emu` creates and installs our GitHub App (item 5's "a
   GitHub App owner"; the programme's owner is a member there, not an owner,
   `05-work-team-verify.md:152-153`);
   `kpmgcomau`'s Azure DevOps administrators, and ours, add our managed identity with Basic access;
   and, if the owner agrees, question 4's two IT requests, filed on day 0 so their lead time runs
   beside the build, with the change to the Microsoft app live Panacea signs in with made outside
   the move's windows.

Item 5's KPMG PAT policy (global PATs stop working on 2026-12-01) no longer applies: no personal
access token is used.

### 3.2 Before the build

- **Phase 2's close-out and merge to `main` stays first.** Before it merges, phase 1's Kata
  node-pool Terraform leaves this branch, as `feat/env` already removed it (`5f499921`):
  `infra/terraform/kata.tf` adds a node pool to whatever cluster it is pointed at, and its
  `dev.tfvars:4` names `aks-panacea-dev`, a cluster name two Panacea estates share.
- **The owner's review of the shell prototype** (`00-plan.md` §3.2) gains one Deploy page over our
  own Panacea. No other prototype changes.
- **Two day-1 spikes are added:** OpenBao under the engine's Vault steps; and Git Credential
  Manager's Microsoft sign-in for `kpmgcomau` on a managed laptop (the Azure DevOps bridge). The
  data plane needs no spike, because it is the one Panacea dev runs; our MinIO admin client is
  proven in CI against the pinned MinIO (row 116).

### 3.3 Five builders

| Builder | Weeks 1 to 5: the first app (engineer-days) | After it |
|---|---|---|
| **B1, sandboxes** | Rows 1 to 9, as planned (10), then our Orca build's first-app rows, moved from B2 (19 to 24, 93, 99, 102, 103: 6.5), then the realm broker and its hand-off (82, 95: 3.5). About 20 | Penpot's runners (39), Design stream B, the operator console's part 2 (131) |
| **B2, our own Panacea** | From day 1: rows 110 and 111 (17.75), then row 117's first-app share (1.25). About 19 | Rows 117 and 118, the weekly re-sync (119), the teams copy (120) at go-live, then Design stream A (34, 36 to 38, 44), rows 87 and 94 |
| **B3, the lane path and the deploy path** | Rows 11 to 18 (about 10), then row 114's first-app share (5.25), row 137's GitHub half (1.25) and row 116 (3.25). About 19.75 | Define (25 to 33), the tracker Start (48, 55), rows 134 and 136, row 137's Azure DevOps half |
| **B4, the Hub** | Row 74's minimum (about 9.25: 08-verify's "about 5 working days on two engineers"), 75, 88, row 105's Hub side, the Desk (89 to 91), 100: about 19.25; then row 112's realm side (the `panacea-deploy` audience, the `groups` mapper, the test account: about 1.5). About 20.75 | The rest of the Hub, the Work page (47, 50), 108, 41 and 42, then the pass-through and project area (127, 128) and half the user screens (121 to 126) |
| **B5, gateway and Deploy** | Rows 76 and 76b, 104, row 105's hand-off, the first-ship Deploy glue over our Panacea (about 4.25 of rows 61 to 66) and row 115: about 18.2; then row 112's changes in our copy's middleware (about 3). About 21.2 | 107, 109, 113, 106, the other half of the user screens, 129, the operator console's part 1 (130) and 132, the Connections page (133), 135, the tracker connectors (45, 46), the Teams bot (52, 53, 92) |

**With a sixth builder** (question 1): B6 takes row 116, all of row 112, about 4 of row 111's engine
edits and rows 91 and 100 from day 1; B2 also takes the realm broker and hand-off (82, 95) from B1;
no first-app builder then carries more than about 18.5.

### 3.4 The acceptance gate

- **K2 and K3, restated again** (`00-plan.md` §3.4). The allowed exits are the first-run sign-in
  tab and its weekly re-check, which the owner accepted, with GitHub's one Authorize click riding
  it, once; and, until the merge, older apps on today's Panacea website (forced exception 5). No
  token is pasted anywhere.
- **J7 now also checks** that a `protected` preview opens in the frame with no sign-in page, and
  that no step of Deploy shows a Panacea site, a key or a Portal link.
- **Gates added:** a security review of row 112 before the pilot: the bearer-token branch and its
  verifier settings; the removal of the header-asserted branch and of legacy project keys; the
  identity-header strip at G4 and the pass-through; the transport and key-derivation secrets, now
  apart; the realm-scoped admin account; the reach of the `groups` mapper; and the pilot's registry
  admin credential (section 1.6). Also: the snippet ledger's CI (row 72) failing on any AGPL, BUSL,
  SSPL or Elastic code in what we build; row 110's production-name check; and the merge's own plan
  before the old Portal retires.

### 3.5 What sign-off records

- **`00-plan.md` re-planned from this file:** §1.6's rows as section 1.2 changes them, the new rows
  into §1, §1.9's sum, §2's percentages, §3's day 0, builders and calendar, §3.4's gate, and §5.2's
  forced exceptions 2 to 4 closed and 5 added. Call 13a's pilots move from Panacea's dev estate to
  ours, still class `none`.
- **The register:** an `OD` for the authenticator, which amends `OD-30`'s "Azure DevOps stays a
  personal access token"; an amendment of `OD-44` only if question 4's line (a) is agreed; `OD-47`'s
  resource group for shipped apps, and the "stand" in `OD-53`'s consequence, if question 1 is
  agreed; `OD-51`'s "one home" amended to two homes until the merge (question 2); `OD-53`'s edge to
  the re-plan; and `OD-44`'s edge from `OD-13`, checked here and holding (section 0 item 5).
- **The identity ruling's token profile** (`2026-09-25-identity-keycloak-broker.md:200`, `:497`:
  `groups` absent) gains `groups` on the `panacea-deploy` audience and the previews' sign-in client.
- **`CLAUDE.md`,** which still says that "Panacea's API" means the running platform we deploy
  through (line 3), that shipped apps deploy into Panacea's resource group "as today" (line 27), "the
  Deploy step's API" and "the later lift ... still stands (`OD-13`)" (line 38), and that
  "`deploy/main.tf` is the live cluster" without saying whose (line 46), rewritten to this shape. The
  verify round did not edit it: a change to `CLAUDE.md` waits for the user's own instruction
  (section 7).
- **Pending questions it settles** (`2026-09-25-owner-questions-pending.md`): Q-P7-7 (which key users
  mint) is moot, because no key exists; Q-P7-6 (how `branchEnv` resolves the production branch,
  `handlers/branch_security.go:29-37`) and O-2 (the project-group prefix match) become ours to fix
  in our copy, and O-2 reads as fixed at `ca92e83eb` (`clients/keycloak.go:468-480` matches exact
  names); the Hub's question 5 (step-up for Azure DevOps) is answered by row 136's broker path once
  the consent exists.

## 4. Owner questions

Product, money and the owner's own systems only. Every technical choice above is ours.

1. **Build our own Panacea at this cost?** It adds about 107 engineer-days to the 132 left, about
   239 in all, in three parts:
   - **(a) about 71 for our own Panacea:** a copy of Panacea's deploy side that we build and run,
     with its own servers, a sign-in with no keys, and the admin screens you use today on Panacea's
     Portal, rebuilt in the app. Your yes also confirms how much we copy: Panacea's deploy services,
     pipelines and deploy scripts, whole, with Panacea named on every copied file;
   - **(b) about 8 for the authenticator:** GitHub connects after one click; Azure DevOps once IT
     approves (question 4);
   - **(c) about 28 for in-app screens the plan already owed** under your "never outside" rule and
     had missed: Panacea's data, storage, logs, risk, cost and team screens. A no to (a) still
     leaves these owed, or leaves Panacea's Portal as a site people must visit.

   With five builders the first app moves from about 13 to 17 working days to about 22 to 25, and
   everything from about 26 to 27 to about 50 to 56. **This changes your "Panacea's Subscription and
   Resource Group as we currently do" answer:** same subscription, same bill and the same cost
   codes, but the new Panacea gets its own resource group, so nothing we run can touch live
   Panacea's resources. It costs about US$1,400 a month in machines while it is a pilot, about
   US$1,700 to 2,100 all-in (machines at list price, the rest estimated). Build machines start on
   demand, so the first build after a quiet spell waits a few minutes; keeping two warm would add
   about US$830 a month. Like Panacea's dev site today, apps built during the pilot can reach the
   internet; locking that down takes a firewall at about US$910 a month, which comes with the
   production copy. That copy, needed before apps hold real client data, costs about US$3,300 to
   5,200 a month more.
   **Recommended: yes to all three, with five builders.** If the first app's date matters more than
   the spend, a sixth builder for the first three weeks brings it to about 19 to 22 working days;
   that lifts your five-builder cap for those three weeks.

2. **Today's Panacea, and the people on it.** **Recommended: leave them where they are until
   Panacea's move to Australia East is signed off.**
   - Existing projects, apps and users stay on today's Panacea and its website, untouched. **That
     includes your own apps (RAPTR, GenFR, Eunomia, Manifold, Trace): until the merge, you open and
     change them on today's website,** the one place anyone still leaves the app. Apps
     made in the new app go to the new Panacea.
   - We copy teams, members and cost centres across once, at go-live. After that a team lives in
     two places until the merge: a new member or a new cost centre on today's Panacea does not reach
     the new one, or the other way round. We pull today's Panacea's fixes into ours every week.
   - We change nothing in today's Panacea during its move, except one web-address record for the
     new apps (question 3), the sign-in settings you already agreed, our address on its allow-list
     (a change to its infrastructure, so it needs your yes during the move's freeze), and, if you
     agree question 4, IT's change to the Microsoft app it signs in with. Each goes in outside the
     move's outage windows.
   - Once the move signs off, the two become one Panacea in this repo, and older apps appear in the
     app too.
   - **The alternative:** older apps shown and deployed in the app before the merge. Today's
     Panacea would have to accept our sign-in (2 to 3 of its own days, during its freeze, with your
     yes), plus about 3 to 5 of ours. Not recommended while the move is under way.

3. **The web address of apps built in the app.** **Recommended: `<app>.apps.panacea.kpmg`,** with
   previews under the same name. No existing project can clash with it. It takes one record in the
   panacea.kpmg zone, which your own account can add, as it did for dev.panacea.kpmg. Apps stay
   reachable from KPMG's network only, as you agreed. **One trade-off:** apps under panacea.kpmg
   share a browser cookie scope with today's Panacea. Our edge drops any cookie an app tries to set
   for all of panacea.kpmg, but a script inside an app could still set one, which could sign people
   out of today's Panacea. A web address outside panacea.kpmg would close that gap, but needs KPMG
   IT to register it.

4. **Two requests to KPMG IT, and one choice.** **Recommended: send both requests on day 0;**
   nothing waits on them.
   - **To IT, for Panacea's Microsoft app** (`AU-PROD-Panacea-RITM6316333`, in KPMG's corporate
     directory; IT owns it, and live Panacea signs in through it): (a) one more sign-in address and
     a second secret, so our sign-in talks to Microsoft directly. Sign-in then stops depending on
     today's Panacea login staying up through its move, and whoever runs that login can no longer
     sign in as one of our users. This changes your "piggy back off Panacea" answer (`OD-44`).
     (c) The Windows sign-in option on the same app, so the weekly sign-in tab can go if the day-1
     test passes. Made outside the move's windows.
   - **To the administrators of the Microsoft directory that holds KPMG's Azure DevOps** (a
     different directory; who runs it is not known yet): (b) a new "Panacea for Azure DevOps" app,
     approved once for everyone, so Azure DevOps connects with nothing to paste.
   - **Two admin steps outside your control:** an owner of KPMG's GitHub organisation
     (`kpmg-au-emu`; you are a member, not an owner) installs our GitHub App, and Azure DevOps
     administrators add our service account to their organisation.
   - **The choice, until (b) lands.** Azure DevOps first tries the laptop's own Microsoft sign-in.
     If that fails too, **recommended:** comments go up under our Panacea account, naming the
     person, and a vote Azure DevOps must count as the person's own waits for IT's approval; no one
     pastes anything. The other way is that each person pastes one Azure DevOps token, which you
     asked not to have.

## 5. Not verified

Nothing was installed, built or run. Every day figure is an estimate composed from estimates.

| Area | Not verified | How to close it |
|---|---|---|
| The estate | The engine has never run in this shape: no realm, no Portal, no Forge, OpenBao, Australia East, our pipeline, dev's gates under a new `ESTATE`. That a new `ENVIRONMENT` value sizes and names cleanly was not traced, nor which of the engine's fresh-estate installers run only on dev estates (`east-infra.md:538-539`). That the platform's pods and the in-cluster data plane fit the pilot's pools is assumed from Panacea dev, which runs the same system and workload shape | Row 111 and its day-1 bootstrap |
| The data plane | CloudNativePG and MinIO run on Panacea dev, proven by its suites at the frozen version (`plan-of-record.md:494-496`); on our estate they are new. Our MinIO admin client is new code. MinIO's community edition stopped publishing images and updates in late 2025 (from knowledge, not checked); our copy runs the image the engine's MinIO chart pins (chart 5.2.0, `cloud-shell-deploy.sh:5091`) | Row 116's CI against the pinned MinIO; row 117 |
| OpenBao | That it replaces Vault under the engine's init, unseal, KV v2 and Transit steps, and Chains' `hashivault://` signer, is from knowledge | The day-1 spike |
| Identity in our copy | The bearer-token branch is design: `VerifyBearerToken` is exported but has no caller (`auth.go:449`). That only the suites and operators use the header-asserted branch was read for `webhook-receiver`, the pipelines and the build (none sends `X-Auth-Request-*` to `panacea-api`), not proven at run time. That Keycloak's group mapper can emit Panacea's group names as they are, and the realm-scoped account's rights, are design, not tried. The no-JWKS fail-open is described in Panacea's own comment (`jwks.go:52-68`); the code path was not read line by line | Row 112, then its security review |
| Pushes | That Panacea's deploy poller carries pushes for our estate behind its 14-address edge. Today's Panacea takes webhooks; the poller is off by default (`cloud-shell-deploy.sh:69-71`) | Row 114 |
| The authenticator | Whether Git Credential Manager's Microsoft sign-in is silent on KPMG laptops (the scout's probe was refused by its session's safety check); whether Windows' account broker silently issues `eb9a11c9…` tokens to `deff24bb…` accounts that are guests there (no Microsoft document found); MSAL Node's native broker in Orca's main process (from knowledge); KPMG's user-consent policy in both tenants; who administers `eb9a11c9…`, and who can create an Azure DevOps organisation connected to it; whether KPMG's GitHub enterprise signs in by OIDC, so that Conditional Access applies; that a GitHub App's re-authorisation completes with no page (documented for OAuth apps only); that a vote by our managed identity counts for no branch policy, so a person's vote must wait (from knowledge) | The day-1 laptop spike; IT's answers (question 4) |
| The subscription's tenant | That Panacea's subscription sits in `eb9a11c9…` is the identity scout's `az account show`; `deploy/environments/panacea-sandbox.env:86-92` says only that it is a different tenant from the sign-in one | One `az account show` with a working session |
| The routes and screens | The 357 / 169 classification is a static read of `routes.go` at HEAD `2836cee91`, not `ca92e83eb`; paths built at runtime may be missed. No Portal usage data was read. The pages' days assume the Portal's panel logic is reused | Rows 121 to 131, re-summed when their plans exist |
| Running cost | Retail list prices from the Azure Retail Prices API, re-read 2026-10-01, `australiaeast`, Linux, pay-as-you-go: B4ms US$0.211, D8ds_v5 US$0.568 and E4s_v5 US$0.302 an hour; for the production estate, Azure Firewall Standard US$1.25, Application Gateway WAF_v2's fixed part US$0.522 and the AKS Standard tier US$0.10 an hour, and Defender for Containers US$0.00941 a vCPU-hour; 730 hours a month. Logs, disks, registry and networking are inferred; the subscription's quota headroom was not checked | Pricing and `az vm list-usage` before day 0 |
| The calendar | 85 to 95% of an engineer-day a working day, the first-app work spread as section 3.3 shows, and no spend-limit stop, are assumptions | Re-sum at the first weekly check-in |
| The cutover | No dates exist for its sign-off; "weeks away" is inferred from its plan (`PLAN.md:27-29`) | Panacea's own schedule |
| The re-sync | The rate is judged from churn (713 commits in 30 days, 412 in the lifted trees, re-counted in the verify round) | Measure the first three pulls |
| The cookie scope | That our edge can drop an app's `Set-Cookie` for the parent domain is design; a script in an app can still set one (question 3) | Row 111 |
| What runs on dev | That `ado-gov/dev` is what Panacea's dev cluster runs was not checked (no `kubectl`) | Read the dev cluster's image tags |
| Licences | `madmin-go`'s pin was read here (`go.mod:9`); its AGPL-3.0 licence is the inventory scout's read of its LICENSE in the module cache. That MinIO's Java and Python SDKs, whose admin clients ours follows, are Apache-2.0 is from knowledge. MinIO server and `mc` AGPL-3.0, Vault BUSL-1.1, Redis 7.4 RSALv2/SSPLv1, Semgrep's rules licence, Opengrep LGPL-2.1, ClamAV GPL-2.0 and Grafana AGPL-3.0 are from `00-panacea-verify.md` §3 and upstream knowledge; `00-plan.md` §5.3's "Later" licence review still applies to what runs unmodified | Row 72's CI check, and the "Later" review |
| The build | That the lifted modules compile at Go 1.25 outside our `go.work`, and their tests pass in our CI | Row 110 |

Each scout's own "not verified" list stands, and so does each cluster's, as `00-plan.md` §6 lists
them.

## 6. Sources

Read 2026-10-01, read-only.

- **This programme:** `CLAUDE.md`; `CONTEXT.md`; the register,
  `docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-13`, `OD-30`, `OD-32`, `OD-37` to
  `OD-53`); `docs/superpowers/planning/2026-09-25-owner-questions-pending.md`;
  `2026-09-25-identity-keycloak-broker.md` (`:164-168`, `:200`, `:449-451`, `:497`). In this folder:
  `00-plan.md` (§0 to §10), `08-whole-product-verify.md` (§6), `09-cohesion.md`,
  `00-panacea-verify.md` (§2 to §4, §7), `06-deploy.md:120-132`, `06-deploy-verify.md` (§2.3 to
  §2.5), `05-work-team-verify.md` (§2 to §3, §8).
- **This repository's tree:** `go.work`; `control-plane/go.mod`; `infra/terraform/kata.tf` and
  `dev.tfvars` on this branch; `feat/env` at `5f499921` and
  `infra/terraform/modules/network/variables.tf:23-30`; `feat/hub`'s
  `docs/superpowers/plans/2026-09-23-phase-h-hub.md:432`, `:442`, `:566`; `feat/phase-3-git`'s
  task 13 (`:97-116`); `feat/phase-7-deploy`'s task 12 (`:158-163`).
- **Panacea, at `ado-gov/dev` `ca92e83eb`** (`Projects/Panacea`, read with `git show`, `git grep`,
  `git log`, `git ls-tree`; never checked out or written): `services/panacea-api/` `go.mod`,
  `main.go`, `middleware/auth.go`, `middleware/jwks.go`, `middleware/entra_claims.go`,
  `clients/keycloak.go`, `clients/tenant_redis.go`, `clients/tenant_ensure.go`,
  `clients/managed_object_storage.go`, `clients/managed_postgres.go`, `clients/kubernetes.go`,
  `clients/minio_admin.go`, `clients/pipeline.go`, `clients/interfaces.go`, `clients/ingress.go`,
  `clients/ai_env_names.go`, `handlers/project_auth.go`, `handlers/auth_verify.go`,
  `handlers/provisioner.go`, `handlers/connection_envvars.go`, `handlers/register.go`,
  `handlers/team_create_approval.go`, `handlers/team_subdomain.go`, `handlers/projects.go:318-328`,
  `handlers/branch_security.go`, `routes/routes.go`; `services/webhook-receiver/pipeline/trigger.go`;
  `services/panacea-migrate-runner/README.md`; beside them, `deploy/environments/` (all five env
  files and the README), `deploy/panacea-deploy.sh`, `deploy/cloud-shell-deploy.sh`,
  `deploy/variables.tf`, `deploy/main.tf`, `deploy/postgres-flexible.tf`,
  `deploy/object-storage-blob.tf`, `deploy/terraform.tfvars`, `deploy/portal/deploy-portal.yaml`,
  `deploy/acceptance/`, `azure-pipelines.yml`, `azure-pipelines-dev.yml`,
  `pipelines/tasks/git-clone.yaml`, `pipelines/tasks/sast-scan.yaml`,
  `pipelines/tasks/agent-report.yaml`, `pipelines/pipelines/agent-deploy.yaml`,
  `kubernetes/base/dns/coredns-custom.yaml`, `kubernetes/helm-values/oauth2-proxy-values.yaml`,
  `kubernetes/argocd-apps/`, `docs/runbooks/aus-east-rebuild-roadmap.md`. The 09-28 hotfix
  `b78699c81` on `kpmg/hotfix-prod-cert-renewal`, not an ancestor of `ca92e83eb`. Line counts by
  `git grep -c`, file counts by `git ls-tree`, churn by `git log` and `git diff --shortstat`.
- **Panacea's cutover documents, at `3d45587f0`:** `docs/cutover/RESUME.md`,
  `docs/cutover/2026-09-25-design/PLAN.md`, `docs/cutover/2026-09-24-understand/east-infra.md`,
  `docs/cutover/2026-09-24-understand/plan-of-record.md`.
- **Live, anonymous, 2026-10-01:** `https://dev.azure.com/kpmgcomau/_apis/connectionData`
  (`X-VSS-ResourceTenant: eb9a11c9…`), matching the `azureauthority` this machine's git
  configuration pins for `kpmgcomau`; the Azure Retail Prices API
  (`https://prices.azure.com/api/retail/prices`, `australiaeast`), re-read in the verify round.
- **The owner's session notes** of 2026-08-17 to 08-19 (Panacea's dev launch on the same engine),
  of 2026-09-28 (the cross-estate cookie shadowing, production's `_panacea_oauth2` cookie on
  `.panacea.kpmg`, and the certificate-renewal break-glass) and of the production cutover plan (the
  tenants live on today's Panacea).
- **The five scouts of this round,** cited as theirs wherever a figure was not re-derived here: the
  inventory (sizes, the lift's days, the estate's needs), the user functions (the route
  classification and the screens' days), identity for Azure DevOps and GitHub (tenants, GitHub's
  and Microsoft's documented flows, Keycloak's grants), the two estates side by side (the cutover's
  rules, the DNS and naming hazards, keyless builds), and the deploy-layer options (the scoring
  that rejected driving today's Panacea out of sight, and Coolify plus Supabase).

## 7. Verify round 1 (2026-10-01)

Two refuters checked this file and the second round's edits (`OD-53` in the register, `00-plan.md`,
`CLAUDE.md`, `CONTEXT.md`). Each finding was checked at source before it was applied: Panacea at
`ca92e83eb` and `3d45587f0`, this repository, and the Azure Retail Prices API. The net moves from
90.7 to 107.2 engineer-days and the plan from 222.975 to 239.475; the capability view gains one row
of ours (row 134) and loses one managed row.

**Applied in place:**
- **The data plane** is the in-cluster one Panacea dev runs, not the managed plane, which cannot run
  at the pin, leaves tenant isolation open and refuses backups, revert, HA, branch data and new
  dedicated databases (section 0 item 4; rows 58, 116). `madmin-go` is replaced, not deleted.
- **No user-held key, not "no key anywhere":** a bearer-token branch for people; the header-asserted
  branch and legacy keys removed; identity headers stripped at G4; the derivation secret split;
  every call through our platform's proxy, none from a laptop (section 0 item 1; rows 64, 112).
- **The engine has no skip switches:** row 111 lists its edits and rises to 10 / 15 / 26.
- **Production defaults in the lifted engine:** only the engine and what it reads are lifted; every
  production default fails when unset; a production-name check in CI (section 0 item 2; rows 110,
  111).
- **The CoreDNS rewrite** that stopped production's renewals is replaced in our estate, with a
  DNS-01 proof (rows 111, 117).
- **The calendar:** floors and an 85 to 95% band; the first-app work spread across five builders;
  first app about 22 to 25, all of it about 50 to 56; the plan of record's own figures noted
  (sections 2.1, 2.6, 3.3).
- **Run cost:** the engine's own pool shape at dev's gates, about US$1,400 a month in machines; the
  egress trade-off and the production estate's cost put to the owner (sections 0 item 4, 1.7; Q1).
- **Question 4:** the self-service line struck; two IT requests across two directories; the
  GitHub and Azure DevOps admin asks named; no pasted token, with a service-identity fallback for
  the owner to choose (section 0 item 6; row 136; Q4).
- **The move's DNS flip:** the realm block, our egress address on East's allowlist and a sign-in
  check declared to the move's inventory; the allowlist entry needs the owner's freeze yes
  (section 0 item 5, section 3.1 item 2; row 70 stays at 0.5).
- **Older apps on today's website** named as forced exception 5, with the alternative's cost (Q2);
  `OD-51`'s two homes until the merge said plainly (Q2, section 3.5).
- **Q1:** split into its three parts; the resource-group change and the sixth builder's lift of the
  cap said plainly; the copy scope confirmed by the owner's own yes.
- **Smaller corrections:** rows 114 and 115 at their bases; Forge's ADO pipeline routed away (row
  114); one realm for shipped apps' users, with `groups` recorded against the identity ruling (row
  112, section 3.5); row 134 to Ours; rows 74, 135 and 137 less their overlaps with rows 45 and 46
  and the Hub's tasks 29 and 39; row 119 at the inventory rate for ten weeks; the cookie rule at the
  edge (row 111, Q3); quota for both East estates at maximum (section 3.1 item 1); our own Azure
  DevOps organisation in `eb9a11c9…` (section 3.1 item 5); a sixth env file, not a fourth; team git
  tokens unrouted (row 137); the lift's tree list and sizes (section 2.5); the code path moved out of
  question 3; the bucket-prefix takeover closed (row 113).
- **Outside this file:** `00-plan.md` §5.2 now notes its Portal verdict fails, §3.3 that its
  calendar sits below its floor; call 20's source re-tagged as the owner's own question-form answer
  in the register and `00-plan.md`, because the second round's instruction listed calls 6 to 20 as
  his own words; the register's two edited paragraphs re-wrapped so every unedited line from 530 on
  is back at its number before the second round (no file cites those lines).

**Not applied, with the reason:**
- **Editing `CLAUDE.md`** (its gotcha, and lines 3, 27 and 38 marked as superseded): a change to
  `CLAUDE.md` needs the user's own instruction, and an agent's request does not carry it. Section 3.5
  lists the lines and the shape.
- **"`CLAUDE.md` still says copy no Panacea source":** at this commit it allows whole services and
  stacks under `OD-50` (`CLAUDE.md:38`).
- **The managed plane's options (a) and (b)** (about 15 more days, or a wait with no date): the
  in-cluster plane is proven on dev, keeps every function and costs about 1 day.
- **The 20-vCPU pilot pools:** replaced by the engine's own system and workload shape, which Panacea
  dev runs, rather than proving a smaller one.
- **Role-assignment rights on day 0:** at the pilot's gates the engine runs Contributor-only, as on
  Panacea dev; the production estate gets that identity (section 1.7).
- **A second realm for shipped apps' users:** keeping them in our realm, as Panacea does, costs
  nothing and keeps one forward-auth gate.
