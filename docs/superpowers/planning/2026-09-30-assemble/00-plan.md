# 00 - The assemble plan of record

**Status:** PLAN OF RECORD (2026-10-01). It replaces the phase plans wherever they disagree about
what we build, and it is superseded only by a later owner decision.
**Date:** 2026-10-01. **Question:** what PanaceaApp adopts, what it still builds, how long that
takes, how far along it is, and how much of it comes from open source.
**The owner's direction (2026-09-30), as `08-whole-product.md` quotes it:** *"focus on using third
party solutions, and solving the main problem statement and building the tool. I need this tool
ASAP, not in weeks."* Orca is the cockpit pick. Panacea is the strong default, and *"if we want to
use Coolify plus Supabase, and enhance with snippets from Panacea, that is fine as well. As long as
we get the best solution that is fully fledged."* Then, the same day, the one-app bar
(`09-cohesion.md:6-8`). These are recorded as `OD-37` to `OD-41` in
`2026-09-23-owner-decisions.md`. **The owner answered section 5 on 2026-10-01** (`OD-42` to
`OD-52`, and the register's "answered, not gated" list of `note` entries), including the one-app
rule restated: *"never does it feel seperate, or needing to go outside to do anything"*
(`OD-49`). This version applies them, corrected by their verify round (section 9), and his second
round the same day (section 10), which adds `OD-53`: Panacea first-class inside the app, its
shape and cost pending `10-panacea-in-app.md`.
**The product is called Panacea** (`OD-43`). Below, "Panacea" alone still means the running
platform we deploy through, as the register's note on the name explains.
**Precedence inside this folder:** `09-cohesion` > `08-whole-product-verify` > each cluster's
`-verify` > each cluster's first report. Every figure below is the latest one in that chain.
**Method:** the 19 files in this folder, re-read where a figure is used; a read-only inventory of
every branch (`git ls-tree`, `git diff` against each merge-base, the plans' task tables). Nothing
was installed or run. Section 6 lists what is not verified.

## 0. The answer

**Assemble. We adopt a dozen products and build the glue and the product spine around them. The
remaining work is about 132 engineer-days (120 to 150): the 118 of the first version plus about
14.25 for the owner's answers of 2026-10-01, almost all of it in-app replacements for steps that
sent a user outside (`OD-49`), the in-app provider page (`OD-46`) and cost codes (`OD-47`). With
five builders (`OD-31`'s cap is now five) that is the first app in about 13 to 17 working days
and all of it in about 26 to 27; with two, about 5.5 to 6.5 weeks and about 13 weeks. The clock
starts when the day-0 items clear. There is no new subscription to wait for (`OD-42`) and no IT
ticket for sign-in (`OD-44`), so day 0 is shorter than it was; real users' sign-in now waits on
one block of Panacea realm configuration instead (day 0 item 3).**

**Not in these figures: `OD-53`** (section 10). The owner wants Panacea first-class inside the
app, built as part of this: *"No API key, nothing. We will build our own Panacea as a part of
this."* Every figure in this file is the plan before it, with Deploy as glue over Panacea's
running API (section 1.6). Its shape and cost are pending `10-panacea-in-app.md` and the owner's
sign-off on that cost. That file's verify round also found two figures here wrong, each noted in
place: section 3.3's calendar, which sits below its own floor, and section 5.2's Portal verdict.

**The three percentages** (method and arithmetic in section 2):

| Measure | Figure |
|---|---|
| **Done** | **About 11%** (10 to 14). About 16.5 engineer-days of kept, built work, against 16.5 + 132.275 |
| **From open source**, by capability (the headline) | **35%** of 110 capabilities. With Panacea (15%) and managed services (13%), **63%** come from something we adopt |
| **We build ourselves**, by capability (the headline) | **37%** |
| By remaining effort, for the same question | 39% of the 132 days is glue on adopted pieces (open source 24%, Panacea 4%, managed services 11%); **61%** is our own code |

Was, before the owner's answers: 12% done; 37% open source, 66% adopted, 34% ours by capability;
44% glue and 56% ours by effort, of 118 days. All six new rows are ours: keeping the user inside
the app is our code over other products' APIs.

Every one of the 132 days is engineering work we do: code, configuration and proofs. "From open
source" means the capability arrives already built and we connect it; it does not mean the days
disappear.

1. **What we adopt.** Orca as the cockpit, shipped as our own build of the pinned upstream, named
   Panacea (`OD-43`). AKS Pod Sandboxing (Kata) with `kubernetes-sigs/agent-sandbox` for
   sandboxes. GitHub Spec Kit for Define. Penpot 2.18 and its MCP server for Design (`OD-45`).
   Azure DevOps Boards and GitHub Issues, headless behind our in-app work views, and Microsoft
   Teams for work and team (`OD-48`). Panacea's API and `panacea-mcp` as the Deploy step. LiteLLM
   for the AI gateway, over any provider the owner configures (`OD-46`). Keycloak for sign-in,
   brokering to Panacea's realm (`OD-44`). Azure's managed Postgres, Blob and Key Vault for our
   own state, in our own resource group in Panacea's subscription (`OD-42`).
2. **What we build.** The Hub (briefs, approvals, the provenance ledger, releases), the one-app
   layer (journey bar, app-level view, the Desk, the realm hand-off, the thread drawer, one
   palette, one vocabulary), the lane path (lane git endpoint, landing, the Panacea plugin), the
   gateway's `keysmith` and `gatekeeper`, every page where the brief is published, approved or
   released, and the in-app pages that replace every outside step that is not forced (`OD-49`):
   PR review, the shipped app's settings, team settings and approvals, production approval, and
   the provider page.
3. **The acceptance gate** is 09-cohesion §6's journey walk, K1 to K14, ending with the owner's
   eyes: *"Does this feel like one app?"* A no fails the build. K2's and K3's allowed exits are
   restated for section 5.2 (section 3.4).
4. **What still waits on the owner** (section 5.3): his sign-off on the cost of `OD-53`, once
   `10-panacea-in-app.md` scopes it, with the shapes that replace forced exceptions 2, 3 and 4,
   which he did not accept (section 5.2). He accepted forced exception 1, the weekly sign-in
   tab. The rest of section 5.3 is closed: calls 6 to 20 are his own answers, so rows 104 to 109
   wait on nothing; cost codes follow Panacea's model; one register with `note` rows; no veto on
   `OD-48`. The "Later" list stands.

## 1. The components

One row per capability, deduplicated across the clusters. **Kind** is who delivers the
capability: **OSS** (an open-source product), **Panacea** (Panacea's running service or its copied
code), **Managed** (a commercial or cloud service), **Ours** (code we write that no product
provides). **Glue** is days spent connecting an adopted piece; **Build** is days on a row that is
ours. Days are engineer-days, verify figures. A 0 means the days are in another row or the piece is
used as it is. **The Kind rule:** code we write is Ours even when it wraps an adopted piece (rows
3, 66, 76b); an open-source piece that Panacea runs for us is Panacea (rows 59, 67); a smoke test,
canary or proof of an adopted piece is glue on that piece (rows 9, 31, 43); a mixed row takes the
Kind of its main store (row 35: Azure Flexible Server, with Valkey and S3Proxy beside it).

**Deduplication applied** (the folder warns of double counting: 02-verify §8 "Days", 08-verify §6
note, 09 §7):
- 02-verify's journey bar carries "2, plus 0.5 in the Hub". The 0.5 is the Hub-side session
  hand-off, which 07-verify's "Hub pages signed in" (1 day) already builds. Counted once, in 07's
  row. 02-verify's own "count 13.5" is not used: 08-verify §7.2 places all of 02's 5 one-app days
  inside 02's 15.5, and its own 2 are different pieces (Penpot in our frame, the page template).
- 00-verify's P3 (create the Panacea project, 0.75 to 1) and 06-verify's G5 (the same call, 0.75)
  are one row at 0.875.
- 00-verify's G4 (1 day) is 06-verify's G4, counted once at 06-verify's 1.5.
- 09 §2 row 14 ("Connect deploy", 0.25 counted) sits inside 00-verify's cohesion glue (0.75).
- Panacea snippets (tenant baseline, `audit_pg.go`, key-minting code, `deploy/*.tf`) shorten rows
  that already exist (00 §12). They are not rows of their own.
- Row 12's `login` (the cluster files' `panacea login`) and the Hub's client (row 74): Hub tasks
  28, 30 and 31 build the Credential Manager store, the PKCE exchange and the refresh (11 plan-hours,
  `feat/hub:docs/superpowers/plans/2026-09-23-phase-h-hub.md:431-441`). Row 12 keeps only the CLI
  over them and the anchor clone, at 02's own 0.5 (02-fleet-cockpit.md:247). Row 74 drops Hub task
  38, the Tauri desktop half of the local caller key (3.5 h), which the frozen desktop no longer
  needs (07-verify §5 Hub row flags 37-38; task 37, the control-plane half, stays).
- 00-verify's cohesion glue (0.5 to 1) includes "previews opened where the Keycloak session lives",
  which is 09's share-link previews (row 96). Row 73 takes the low end, 0.5.
- The sandbox cluster (row 1) and our environment (row 78) are **one AKS**: phase 8 task 06 builds
  one cluster with three pools, the Kata pool among them. Row 1's day is the cluster, the Kata
  pool, ACR, the Firewall and the feature registration (01-verify §7); row 78's day is the Hub's
  Flexible Server, Key Vault, DNS, ingress and the other two pools (07-verify §5).

### 1.1 Sandboxes (01-sandboxes-verify §7): 10 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 1 | Sandbox cluster: AKS in Australia East, Kata pool, ACR, Azure Firewall, feature registration | Platform | AKS Pod Sandboxing, with Panacea's Terraform as a pattern | Azure; Kata Apache-2.0 | Managed | 1 | | 01-verify §7 row 1 |
| 2 | Warm pool, claim, suspend and resume | Platform | `kubernetes-sigs/agent-sandbox` v1.0.4 | Apache-2.0 | OSS | 0.5 | | 01-verify §7 row 2 |
| 3 | `katabe` over `SandboxClaim`s (client-go, RuntimeClass fixed to `kata-vm-isolation`) | Platform | Our backend over agent-sandbox's CRDs | ours | Ours | | 1 | 01-verify §7 row 3, §8 items 1-2 |
| 4 | Sandbox image and late-bind: Orca headless runtime, `panacea-mcp`, `POST /bind` | Develop | Orca runtime plus our `sandboxd` | MIT | OSS | 1.5 | | 01-verify §7 row 4 |
| 5 | Default-deny egress, FQDN allowlist, npm and PyPI mirrors | Platform | Azure Firewall, Verdaccio, proxpi | MIT; Firewall commercial | OSS | 1.5 | | 01-verify §7 row 5 |
| 6 | Worktree into the sandbox (git bundle over the bind channel, lanes back the same way) | Develop | Ours | ours | Ours | | 1.5 | 01-verify §7 row 6 |
| 7 | Broker in the cluster behind Keycloak, per-sandbox `wss://` route | Platform | Ours (phase 8 tasks 17-18) | ours | Ours | | 2 | 01-verify §7 row 7 |
| 8 | Orca environment recipe (create, suspend, resume, destroy) | Develop | Orca `environmentRecipes` | MIT | OSS | 0.5 | | 01-verify §7 row 8 |
| 9 | Smoke: 10 sessions, claim p50/p95, refill, timed clone and build | Platform | AKS Pod Sandboxing plus agent-sandbox | Apache-2.0 | OSS | 0.5 | | 01-verify §7 row 9 |

### 1.2 The cockpit (02-fleet-cockpit-verify §6-7): 17.5 days

**Ruling, not an owner decision:** Orca ships as our own thin build of the pinned upstream, under
our name (02-verify §7-8; settled by the one-app ask, `OD-41`, in 08-verify §7.4 call 2). **The
name is Panacea** (`OD-43`); the icon is deferred and does not block rows 19-20.

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 10 | Fleet cockpit engine: parallel sessions, worktrees, terminals, diff, editor | Develop | Orca (`stablyai/orca`), pinned | MIT | OSS | 0 | | 02-verify §0, §8 item 1 |
| 11 | Rollout package: pinned version, Intune, telemetry off, plugin preinstalled | Develop | Orca | MIT | OSS | 0.5 | | 02-verify §6 row 1 |
| 12 | Our CLI's `login` (the cluster files' `panacea login`; the binary is not named `panacea`, which Panacea's own CLI already is, `OD-43`): the CLI over the Hub client's PKCE, Credential Manager and refresh (row 74), and the anchor clone | Platform | Ours | ours | Ours | | 0.5 | 02 §5 "First ship" (0.5); 02-verify §6 row 2's extra 0.5 is Hub tasks 28-31 |
| 13 | Lane git endpoint: session-token auth, lane-branch-only hook, lease-guarded force | Develop | Ours | ours | Ours | | 2 | 02-verify §6 row 3 |
| 14 | Landing from the laptop: fetch back, push, PR with brief trailer, merge queue or auto-complete | Develop | GitHub and Azure DevOps pull requests | commercial | Managed | 2.5 | | 02-verify §6 row 4, §8 item 4 |
| 15 | Panacea PR preview per lane, with the session and brief stamp (GitHub only in v1) | Deploy | Panacea's webhook-receiver previews | KPMG own | Panacea | 0.5 | | 02-verify §6 row 5 |
| 16 | The Panacea plugin: lane card, commands, recipe, pending-brief step | Develop | Ours, on Orca's plugin API | ours | Ours | | 2.5 | 02-verify §6 row 6 |
| 17 | Resume and fork through the recipe, from phase 2's transcript store | Develop | Ours | ours | Ours | | 1 | 02-verify §6 row 7 |
| 18 | AI first-pass PR review, opt-in, through our gateway | Develop | `anthropics/claude-code-action`; `claude -p` in Azure Pipelines | MIT | OSS | 0.5 | | 02-verify §6 row 8 |
| 19 | Our build's identity: the name Panacea, app id, Panacea's current mark until a new icon, KPMG signature through Azure Artifact Signing, our update feed | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 change row 1; `OD-43`; call 4 |
| 20 | Build-time rename over the English catalogue: Orca's words to Panacea's | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 row 2; `OD-43` |
| 21 | Our tokens on Orca's shadcn variables (first five mappings) | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 row 3 |
| 22 | Journey bar, app-level view for Hub pages, `lane` deep link | Cohesion | Ours, inside our Orca build | ours | Ours | | 2 | 02-verify §7 row 4 (its Hub 0.5 is row 75) |
| 23 | OSC status off for hook-capable agents (`OD-8` patch) | Cohesion | Ours, a patch in our build | ours | Ours | | 0.75 | 02-verify §7 row 5, §2.1 |
| 24 | Orca's cloud features off: telemetry, mobile relay, share-as-artifact, computer use | Cohesion | Orca, our build | MIT | OSS | 0.25 | | 02-verify §7 row 6 |
| 106 | **In-app PR review** for reviewers: the PR's diff in Orca's diff view over the fetched PR ref, its comment threads with reply in place, approve and request changes, all over the GitHub and Azure DevOps REST APIs; review comments into the Desk and the lane card. Replaces "reviewers on the host's PR page" | Develop | Ours | ours | Ours | | 3 | `OD-49`; 09 §7 exception 1 (the 2 days "after v1"), plus 1 for the diff and the review verdict |

### 1.3 Define (03-define-verify §6): 10.25 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 25 | Define engine: interview and brief, rendered into the image, `panacea` preset | Define | GitHub Spec Kit v1.0.13 | MIT | OSS | 1.75 | | 03-verify §6 G1 |
| 26 | Glossary and decision-record interview skills | Define | `mattpocock/skills` (aihero), pinned | MIT | OSS | 0 | | 03-verify §5.1 (inside G1) |
| 27 | Define session start: a pending opening prompt through 02's recipe | Define | Orca, unforked, plus our recipe | MIT | OSS | 0.75 | | 03-verify §6 G2 |
| 28 | Publish route: reads the pushed blob, per-entity access check, writes a Hub revision | Define | Ours | ours | Ours | | 2.25 | 03-verify §6 G3 |
| 29 | Hub record kind for terms and decisions (`OD-36`) | Define | Ours | ours | Ours | | 1 | 03-verify §6 G4 |
| 30 | Publish page: the exact pushed blob, drift line, definition of done, brief chip | Define | Ours | ours | Ours | | 2.25 | 03-verify §6 G5 |
| 31 | Spec Kit canary and end-to-end run | Define | Spec Kit | MIT | OSS | 1.25 | | 03-verify §6 G6 |
| 32 | Data class at creation (none, synthetic, protected), handed to Panacea | Define | Panacea's data-class ceiling | KPMG own | Panacea | 0.5 | | 03-verify §6 G7 |
| 33 | Kept phase 4 thread tasks outside the glue | Define | Ours | ours | Ours | | 0.5 | 03-verify §6 total |

### 1.4 Design (04-design-verify §2.7): 12.75 days

Penpot for v1 is the owner's decision (`OD-45`): MPL-2.0 admitted, frames as vectors with CSS flex
and grid, HTML through the markup export; OpenPencil revisited at its 1.0.

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 34 | Canvas deploy: chart 1.10.0, admin console off, OIDC only, telemetry off | Design | Penpot 2.18 and its Helm chart | MPL-2.0 | OSS | 1.5 | | 04-verify §2.7 G1 |
| 35 | Penpot's stores: Postgres, cache, objects | Design | Azure Flexible Server; Valkey 8; S3Proxy on Blob | commercial; BSD-3; Apache-2.0 | Managed | 0 | | 04-verify §5 (inside G1) |
| 36 | Penpot identity and roster: realm client, invitations, per-lane bots | Design | Penpot OIDC and Keycloak | MPL-2.0 | OSS | 1 | | 04-verify §2.7 G2 |
| 37 | Design pane | Design | Penpot | MPL-2.0 | OSS | 0.5 | | 04-verify §2.7 G3 |
| 38 | Agent MCP relay: per-lane token routing, file-id guard, audit | Design | Penpot's MCP server, our relay (phase 5 task 18, built) | MPL-2.0 | OSS | 2 | | 04-verify §2.7 G4 |
| 39 | Headless MCP runner per active lane file, in the Kata pool | Design | Penpot MCP plugin in headless Chromium | MPL-2.0; Apache-2.0 | OSS | 2.5 | | 04-verify §2.7 G5 |
| 40 | Design tokens converter (DTCG to `tokens.css`) | Design | Penpot token export | MPL-2.0 | OSS | 0.5 | | 04-verify §2.7 G6 |
| 41 | Brief binding and design approval: Hub record, drift webhook, save and pin, our panel | Design | Ours | ours | Ours | | 2 | 04-verify §2.7 G7 |
| 42 | Design hand-off bundle into the worktree's `design/` | Design | Ours | ours | Ours | | 1 | 04-verify §2.7 G8 |
| 43 | Penpot upgrade canary and end-to-end run | Design | Penpot | MPL-2.0 | OSS | 0.75 | | 04-verify §2.7 G9 |
| 44 | CSP and plugin allowlist at our ingress | Design | Penpot behind our ingress | MPL-2.0 | OSS | 1 | | 04-verify §2.7 G10 |

### 1.5 Work and team (05-work-team-verify §8): 21.5 days

Tasks are created, assigned, worked and closed inside Panacea; ADO Boards and GitHub Issues are
storage and sync behind it, never opened (`OD-48`; the owner chose no veto, section 5.3 item 4).

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 45 | Tracker connector for Azure DevOps Boards (WIQL, live status, hooks, Proof fields) | Develop | Azure DevOps Boards, over its REST API | commercial | Managed | 3.5 | | 05-verify §8 piece 1 |
| 46 | Tracker connector for GitHub Issues and Projects (Global projects only) | Develop | GitHub Issues and Projects | commercial | Managed | 2.5 | | 05-verify §8 piece 1; 05 §8 |
| 47 | Provenance edge and proof: issue serves criterion, the done evaluator | Develop | Ours | ours | Ours | | 2.5 | 05-verify §8 piece 2 |
| 48 | Start from an issue: control-plane route, first status in 10 s | Develop | Ours | ours | Ours | | 2.5 | 05-verify §8 piece 3 |
| 49 | Tracker MCP servers for sandboxed agents, read-only | Develop | `microsoft/azure-devops-mcp`, `github/github-mcp-server` | MIT | OSS | 0 | | 05-verify §9 (inside piece 3) |
| 50 | Work page and team view | Develop | Ours | ours | Ours | | 2.5 | 05-verify §8 piece 4 |
| 51 | Team roster, members, invites, budgets | Platform | Panacea's team API | KPMG own | Panacea | 0 | | 05-work-team §1; 05-verify §4 |
| 52 | Teams bot for inform and decide cards | Cohesion | Microsoft Teams, Azure Bot Service, Agents SDK | commercial; SDK MIT | Managed | 3 | | 05-verify §8 piece 5 |
| 53 | Tracker notifications in Teams channels, **through our bot's cards**, each opening its Desk row or lane. The first-party Boards and GitHub apps for Teams are not installed: their cards open ADO Boards or GitHub, outside the app | Cohesion | Microsoft Teams, with our bot | commercial; SDK MIT | Managed | 0.5 | | 05-verify §6; `OD-49` (was 0, the first-party apps) |
| 54 | Preview comments on the running app, filed as a ticket | Develop | Panacea `preview_comments.go` | KPMG own | Panacea | 1.5 | | 05-verify §8 piece 6 |
| 55 | Azure DevOps Start without a fork (pending start through the recipe) | Develop | Orca, unforked | MIT | OSS | 1.5 | | 05-verify §8 piece 7 |
| 56 | Tracker audit into the SIEM | Platform | Microsoft Sentinel connectors | commercial | Managed | 0 | | 05-work-team §7 |
| 108 | **Team settings and approvals in-app**: members, invitations, budget and the team's cost centre, over Panacea's team API (`GET`/`POST`/`DELETE /api/v1/teams/{name}/members`, `routes.go:1962-1971`; `PUT /api/v1/teams/{name}/billing`, `:1947`); and the two approvals every new user waits on (00-verify S2), each a NEEDS YOU row on the Desk with the act in place: a team admin's join-request approval (`:1927-1929`) and a platform admin's new-team approval (`:1924-1926`). The requester sees what is pending and who approves. Row 51 had no surface, which left Panacea's Portal as the only place to do it | Platform | Ours, over Panacea's team API | ours | Ours | | 1.5 | `OD-49`, `OD-51` (1, plus 0.5 for the approvals and their Desk rows, this file's estimate) |

### 1.6 Deploy (06-deploy-verify §2.5, 00-panacea-verify §7): 16.075 days

Shipped apps and preview databases deploy through Panacea into Panacea's subscription and
resource group, as today (`OD-47`). **`OD-53` changes this cluster** (section 10): Panacea is
first-class inside the app, with no external deploy site and no API key. The rows below are the
plan before it, with Deploy as glue over Panacea's running API, until `10-panacea-in-app.md`
re-plans them and the owner signs off its cost.

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 57 | Deploy engine: previews per branch, ship, promote, rollback, versions, evidence | Deploy | Panacea API on `dev.panacea.kpmg` | KPMG own | Panacea | 0 | | 06-verify §2.2, §7 item 1 |
| 58 | Shipped-app backend: Postgres, storage, secrets, functions, flags. A preview **shares the app's production database** unless a branch-data mode is chosen (not costed, see below) | Deploy | Panacea (CloudNativePG, Vault, Knative, Flagsmith, MinIO) | KPMG own; mixed stack | Panacea | 0 | | 00-panacea §7, §11; 00-verify §2 row 9 |
| 59 | Build pipeline, scans and the posture gate | Deploy | Panacea (Tekton, Trivy, gitleaks, OSV, Opengrep) | KPMG own; scanners Apache-2.0/MIT | Panacea | 0 | | 00-verify §2 rows 10-11 |
| 60 | Agent deploy and fix loop from inside the sandbox | Deploy | `panacea-mcp` (31 tools) | KPMG own | Panacea | 0 | | 06-verify §2.2 |
| 61 | G1: our Deploy page and lane card (logs, scans, governance, approve), no Portal | Deploy | Ours, over Panacea's API | ours | Ours | | 2.75 | 06-verify §2.5 G1, §6 |
| 62 | G2: signed brief-to-release provenance (in-toto, DSSE, stored evidence bytes) | Deploy | cosign and sigstore Go libraries, key in Key Vault | Apache-2.0 | OSS | 1.25 | | 06-verify §2.5 G2, §3 |
| 63 | G3: promotion by data class through Panacea's stage ladder. The refusal to promote when Panacea's class annotation differs from the brief's is not in the 0.75 (see below) | Deploy | Panacea stage ladder | KPMG own | Panacea | 0.75 | | 06-verify §2.5 G3; 03-verify §5.3 |
| 64 | G4: agent allowlist proxy in front of `panacea-mcp` (method, route, body rules, SSE) | Deploy | Ours | ours | Ours | | 1.5 | 06-verify §2.5 G4; 00-verify §7 G4 |
| 65 | G5 and P3: create the Panacea project (labels, scan posture, team state, and the project's `jobCode` and `phase` in the create body, `models.go:652-660`) | Deploy | Panacea `POST /api/v1/projects` | KPMG own | Panacea | 0.875 | | 06-verify §2.5 G5; 00-verify §7 P3; `OD-47` |
| 66 | Kept phase 7 connector tasks (07 rest, 08, 09, 10, 19, 20) | Deploy | Our connector behind `deploy.Deployer`, over Panacea | ours | Ours | | 1.7 | 06-verify §2.5 ("control-plane tasks") |
| 67 | Build provenance signing and admission check: **audit only in v1**, enforced once Panacea's P-4 lands (Chains v0.23.0 signs only while Vault is unsealed; Kyverno v1.12.5 in Audit) | Deploy | Tekton Chains, Kyverno, as Panacea runs them | KPMG own; Apache-2.0 | Panacea | 0 | | 06-verify §2.2, §3, §7 item 5.5 |
| 68 | P1: `panacea-mcp` built into every sandbox image | Develop | Panacea `panacea-mcp`, copied | KPMG own | Panacea | 0.25 | | 00-verify §7 P1 |
| 69 | P2: agent-context bootstrap skill into the worktree | Develop | Panacea agent-context endpoint | KPMG own | Panacea | 0.25 | | 00-verify §7 P2 |
| 70 | P5: six Panacea-side asks, written and checked | Platform | Panacea's stream; our coordination | KPMG own | Panacea | 0.5 | | 00-verify §7 P5 |
| 71 | P6: live proof on Panacea dev (create, preview, SSO, rollback) | Deploy | Panacea dev estate | KPMG own | Panacea | 0.5 | | 00-verify §7 P6 |
| 72 | P4: snippet ledger and attribution-header CI check | Platform | Ours | ours | Ours | | 0.25 | 00-verify §7 P4 |
| 73 | Panacea's steps inside our cockpit: team-request state, Connect deploy, our error copy (preview sign-in is row 96) | Cohesion | Ours | ours | Ours | | 0.5 | 00-verify §7 "cohesion glue" (0.5-1), §8 |
| 107 | **The shipped app's settings in-app**: environment variables and secrets per environment (`PUT`/`DELETE /api/v1/projects/{name}/env/...`, `routes.go:1401-1403`), feature flags (`:2415-2419`), app users and the branch access mode (`:1431`), database and storage connection details; and the settings only the Portal had: access bindings and ownership transfer (`:1408-1410`, owner), the IP allow-list and SSO enforcement (`:1415-1416`), deployment protection (`:1436`), the custom domain and its DNS check (`:1250-1252`), the backup policy, back-up-now and revert (`:1355-1357`), and the service tier (`:2455`, owner). Over Panacea's API, in the shared Hub page template. Rows 57 and 58 had no surface for these, which left Panacea's Portal as the only place to do any of it | Deploy | Ours, over Panacea's API | ours | Ours | | 4.5 | `OD-49`; routes read 2026-10-01 (2.5, plus 2 for the second group: access 0.5, security and protection 0.5, domain 0.5, backup 0.25, tier 0.25, this file's estimate) |
| 109 | **Production-rung approval in-app** for the Panacea admins who hold it, on our Deploy page. 06-verify §6 left it on Panacea's Portal as an operator act. **Panacea's step-up gate (F4)** refuses every API-token caller on `prod-deploy` once a platform admin maps that action to an authentication context (`handlers/stepup.go:205-213`, `:271`; reached from the production deploy, `handlers/projects.go:2927`); it ships inert, and Panacea's own `deploy/f4-step-up.sh` declines to map `prod-deploy`. Our control plane calls with the user's `pnc_`, so an armed gate would send production ships to the Portal (section 5.2, forced exception 4) | Deploy | Ours, over Panacea's approval route | ours | Ours | | 0.5 | `OD-49`; 06-verify §6; 00-verify S5 left step-up untraced |

### 1.7 Platform (07-platform-verify §5): 26.95 days

Our platform (the Hub, the gateway, the sandbox cluster, our Keycloak) is its own resource group
and its own AKS cluster inside Panacea's existing Azure subscription (`OD-42`), not on Panacea's
AKS. No new subscription is requested.

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 74 | The Hub: briefs, approvals, ledger, releases (routes and client): tasks 08-10, 13-24, 28-35, 37, 39 and P1 | Platform | Ours (Go, `cmd/hub`) | ours | Ours | | 9.5 | 07-verify §5 Hub (10), less task 38 (3.5 h) |
| 75 | Signed-in Hub pages: one-time ticket to an `HttpOnly` cookie | Platform | Ours | ours | Ours | | 1 | 07-verify §5, §6 C1 |
| 76 | AI gateway, LiteLLM glue: catalogue and generated config, admin client, capability probe, the MIT-tree image and its hash lock, its share of the local stack and chart | Platform | LiteLLM from the MIT tree | MIT | OSS | 1.9 | | phase 6 tasks 02, 04, 18, 27 (11.5 h) plus 17, 19 in part (5.75 h) and the 2 h hash lock, as 18-26% of 07-verify's 8.7 |
| 76b | AI gateway, ours: `keysmith`, `gatekeeper` (routes, per-session keys, budgets, refusal rewrite), our CLI's `token` (the plans' `panacea token`, row 12), the residency canary, the panel, the proofs | Platform | Ours | ours | Ours | | 6.8 | the rest of 07-verify §5 Gateway's 8.7 |
| 77 | Inference providers: any LiteLLM supports, among them the Anthropic API, Amazon Bedrock Sydney and KPMG Workbench (`/anthropic`, `Ocp-Apim-Subscription-Key`) | Platform | The providers the owner configures | commercial | Managed | 0 | | 07-platform §7 call 2; `OD-46` |
| 104 | **The provider page, in-app**: the owner adds, tests, rotates and removes provider endpoints and keys, and moves routes between them without a code change; each provider carries its residency and retention claims, shown beside it, and an AU-class project is offered only AU-marked providers; `OD-33`'s leak canary for **every** provider with a custom endpoint, KPMG Workbench included, not only AU-marked ones, because LiteLLM's `count_tokens` defect ignores a custom `api_base` and calls `api.anthropic.com` with the deployment's own key; a provider that fails it is labelled so and offered to no project until the pinned LiteLLM fixes it. Over LiteLLM's admin API with models and credentials stored in its database, never its own UI | Platform | Ours, over LiteLLM | ours | Ours | | 2.5 | `OD-46` (new work: page 1.5, routes and class gating 0.5, per-provider claims and canary 0.5) |
| 78 | Environment: its own resource group and AKS in Panacea's subscription, Flexible Server, Key Vault, DNS, ingress, OpenTofu from Panacea's `.tf` (copied whole, `OD-50`) | Platform | Azure | Azure; OpenTofu MPL-2.0 | Managed | 1 | | 07-verify §5 Environment; `OD-42` |
| 79 | Our own state: Hub database, blobs, secrets | Platform | Azure Flexible Server, Blob, Key Vault | commercial | Managed | 0 | | 00-verify §6; 07-verify §2 |
| 80 | Public AU edge WAF for the Hub and gateway | Platform | Azure Application Gateway WAF_v2 | commercial | Managed | 0 | | 07-verify §8 item 4 |
| 81 | The identity source: Panacea's realm, brokering Entra, with one client for our broker plus the ruling's K3 and K4's user-profile items, which only Panacea's realm can carry (day 0 item 3) | Platform | Panacea's dev Keycloak 25 realm | Apache-2.0 | Panacea | 0 | | 07-verify §5 Realm; `OD-44` |
| 82 | Our small Keycloak, Keycloak 26 on two replicas, **whose only identity provider is Panacea's realm** (Keycloak-to-Keycloak OIDC): tasks 08 and 29, the realm file's provider block, chained provider hints, `tid`/`oid` pass-through (no `groups`: the ruling's token profile leaves it out), the desktop client (K1), the four audience scopes (K2), the 7-day offline cap and `hub.org_admin`. No Entra change, so no IT request | Platform | Keycloak, keycloak-config-cli | Apache-2.0 | OSS | 2 | | 07-verify 3.11 (2 days, whose IT ask goes); `OD-44`: equal cost |
| 83 | Keycloak login and error theme in our design system | Cohesion | Keycloakify | MIT | OSS | 0 | | 07-verify §6 C2 (inside row 82) |
| 84 | One spend line per project (agent spend plus hosted-app spend) | Platform | Ours | ours | Ours | | 0.5 | 07-verify §6 C4 (outside every earlier total) |
| 105 | **Cost Centre, Job Code and Phase mapped through from day one**, as in Panacea: job code and phase captured at project creation in Define and editable later, optional as Panacea has them (`handlers/project_billing_codes.go:22`), stored in the Hub, sent to Panacea on create (row 65) and on change (`PUT /api/v1/projects/{name}/billing/codes`); the cost centre is the team's, as Panacea keeps it (`handlers/billing.go:104`); codes stay optional and the cost centre the team's, which closes section 5.3 item 2 (the orchestrator's reading of *"like we have in Panacea"*, which the owner may override). Carried as **annotations** on sandbox pods, as Panacea carries them on namespaces (free text such as "Ph 2" cannot be a Kubernetes label value), and as tags on gateway keys; our statement in Panacea's shape (project, job code, phase, amount). Panacea's ≤10-character rule and validation reused under `OD-40` | Platform | Ours, on Panacea's pattern | ours | Ours | | 1.75 | `OD-47` (Hub fields and form 0.5, Panacea hand-off and edit 0.35, annotations and key tags 0.4, statement 0.5) |
| 85 | Code signing for our desktop build | Platform | Azure Artifact Signing; an OV certificate in Key Vault if the entity does not qualify | commercial | Managed | 0 | | 02-verify §8 call 6; call 4 |

### 1.8 The one-app layer (08-whole-product-verify §7.2, 09-cohesion §7): 17.25 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 86 | Journey test with the one-app checks (K1 to K13 automated) | Cohesion | Ours | ours | Ours | | 2 | 08-verify §6; 09 §6 |
| 87 | Penpot same-site inside our frame (`frame-ancestors`, silent OIDC) | Cohesion | Penpot, behind our ingress | MPL-2.0 | OSS | 1 | | 08-verify §7.2 |
| 88 | One shared Hub page template (layout, states, brief card) | Cohesion | Ours | ours | Ours | | 1 | 08-verify §7.2 |
| 89 | The Desk: the one attention queue | Cohesion | Ours | ours | Ours | | 3 | 09 §5.4, §7 row 2 |
| 90 | Desk source adapters (hooks, trackers, git host, Penpot, Panacea) | Cohesion | Ours | ours | Ours | | 1.5 | 09 §5.4, §7 row 2 |
| 91 | Hook relay: Claude Code HTTP hooks to `sandboxd` to the Hub | Cohesion | Ours | ours | Ours | | 1.5 | 02 R6 "later"; moved to first ship by 09 §7, and in no earlier total |
| 92 | Teams mirror of Desk decide cards | Cohesion | Teams, with our bot | commercial; SDK MIT | Managed | 0.5 | | 09 §2 row 17, §7 row 2 |
| 93 | Orca's bell, feed, notifications and dashboard off | Cohesion | Orca, our build's defaults | MIT | OSS | 0.25 | | 09 §2 row 2 |
| 94 | Penpot's editor themed and de-branded (stylesheet, OIDC-start shim, flags off) | Cohesion | Penpot | MPL-2.0 | OSS | 0.75 | | 09 §3.2, §7 row 4 |
| 95 | Realm hand-off authenticator: one realm session inside the frame, installed in our Keycloak (row 82) | Cohesion | Ours, on Keycloak's SPI | ours | Ours | | 1.5 | 09 §4.1, §7 row 5; `OD-44` |
| 96 | Share-link previews, for classes `none` and `synthetic`. No end condition: the hand-off (row 95) never gives Panacea's previews a session (`OD-44`) | Cohesion | Panacea's share-links API | KPMG own | Panacea | 0.25 | | 09 §4.2 |
| 97 | Spec Kit preset in our nouns (brief, criteria) | Cohesion | Spec Kit preset | MIT | OSS | 0.25 | | 09 §5.5, §7 row 6 |
| 98 | One type pairing: Geist Sans and Commit Mono | Cohesion | Geist and Commit Mono (OFL fonts) | OFL | OSS | 0.25 | | 09 §3, §7 row 7 |
| 99 | One palette across every surface | Cohesion | Orca's Cmd+J palette, through our plugin | MIT | OSS | 1 | | 09 §5.3, §7 row 8 |
| 100 | The thread drawer | Cohesion | Ours | ours | Ours | | 1 | 09 §5.2, §7 row 9 |
| 101 | One vocabulary: banned-word CI scan over 09 §5.5's list (`CONTEXT.md`'s "Words we do not use" plus Orca, Penpot, Backstage, Keycloak, LiteLLM, spec, PRD, workspace and dashboard), to which `OD-43` adds "PanaceaApp" and "Spec Kit"; one scoped exception, the billing **Phase** field (`OD-47`, `CONTEXT.md`); the product is Panacea. Copy review, `CONTEXT.md` entries | Cohesion | Ours | ours | Ours | | 0.75 | 09 §5.5, §7 row 10; `OD-43` |
| 102 | Orca theme file, all 125 variables | Cohesion | Orca, themed by our generated CSS | MIT | OSS | 0.5 | | 09 §3.1, §7 row 10 |
| 103 | Terminal theme from our tokens | Cohesion | Orca custom terminal theme | MIT | OSS | 0.25 | | 09 §2 row 5 |

**Outside the total:** the Windows sign-in bridge (1 day, a day-1 spike, 09 §4.2), which is no
longer a contingency on the hand-off: the hand-off sits in our realm and never reaches Panacea's
pages (`OD-44`), so the bridge is the only route that could bring Panacea's key page and a
`protected` preview into the frame before P-3, together with Panacea's `kc_idp_hint=entra` on the
Portal's and oauth2-proxy's sign-in (section 5.2, forced exception 3, now superseded by `OD-53`;
the spike stays as the Windows-native sign-in that may remove the weekly tab, forced exception 1);
the owner's veto on `OD-48` (about 4 to 44 days), not used (section 5.3); the upstream ADO
task-source pull request (2 days, optional, 05-verify §8); the
Tauri Deploy lens (2.3 days, only if IT refuses Orca, 06-verify §2.5); Forge's ship gates as a
pre-deploy check (1 day, later, 00 §12); the image-bump canary pipeline (not costed, 01 §5 item 5); Hub tasks 25-27 and 36
(evidence, the change feed, the audit route, the feed poller: 8 plan-hours, about 1 day, outside
07-verify's 10; tasks 40-42 are Tauri UI and drop under Orca); the day-1 spikes of section 3.2
(the browser route is 0.5, 02-verify §5.3; the rest are not sized).

**Named gaps, not costed.** No source sizes these, and each is real work before customers:
- **The preview's branch-data mode** (seed or sanitised clone) chosen per data class when the
  preview is created. With nothing requested, an agent's preview writes to the app's production
  database (00-verify §2 row 9; 08-verify §6 calls it unsized glue).
- **The Deploy refusal** to promote a project whose Panacea data-class annotation differs from the
  brief's class. 03-verify §5.3 assigns it to Deploy; 06-verify does not pick it up.
- **A repo, and a tracker, for a person who starts from nothing.** No step in 03, 06, 08 or the
  phase 3 plans provisions one (05-verify §9); our own Australian ADO organisation (`OD-48`) is
  its home, and the provisioning step is not sized.
- **A `protected` preview inside the frame.** Previews behind Panacea's oauth2-proxy sign in
  against Panacea's realm, not ours, so the realm hand-off (row 95) gives them no session
  (`OD-44`). Share links (row 96) cover `none` and `synthetic`, the pilot's classes (call 13a). A
  `protected` preview needs a Panacea-side step: the hand-off installed in Panacea's realm as well
  (a Panacea image change, behind the cutover freeze), or the Windows bridge with Panacea's
  `kc_idp_hint=entra` on oauth2-proxy. **Not a remedy:** Panacea's realm trusting ours as an
  identity provider. It would be a second identity provider in Panacea's realm, which breaks the
  ruling's one-IdP invariant (a hint could then pick it,
  `2026-09-25-identity-keycloak-broker.md:503-504`), and each realm would broker to the other.
  Needed before any `protected` release; not sized.

### 1.9 The sum

| Cluster | Days | Against the chain's figure |
|---|---|---|
| Sandboxes | 10 | 10 (08-verify §6) |
| Cockpit | 17.5 | 15.5, less the 0.5 Hub hand-off counted in row 75, less row 12's 0.5 counted in row 74 = 14.5; plus row 106's 3 (`OD-49`) |
| Define | 10.25 | 10; the rows sum to 10.25 (03-verify §6) |
| Design | 12.75 | 13 rounded (04-verify §2.7) |
| Work and team | 21.5 | 20 rounded (05-verify §8) = 19.5; plus row 53's 0.5 and row 108's 1.5 (`OD-49`) |
| Deploy and Panacea | 16.075 | 8.5 + 2; 06-verify's pieces sum to 8.7, and 00-verify's unique rows add 2.375 (cohesion glue at 0.5, row 73) against 08-verify's "2, a floor" = 11.075; plus rows 107 (4.5) and 109 (0.5) (`OD-49`) |
| Platform | 26.95 | 08-verify's components 20.4 (Hub 10, pages 1, gateway 8.4, environment 1) + 0.3 (07-verify's final gateway is 8.7) + realm 2 + C4's 0.5 = 23.2, less Hub task 38's 0.5 = 22.7; plus rows 104 (2.5, `OD-46`) and 105 (1.75, `OD-47`). The realm stays 2 under `OD-44` |
| One-app layer | 17.25 | 4 + 11.75, plus the hook relay's 1.5, which no total carried |
| **Total** | **132.275, about 132** | 118.025 before the owner's answers, plus 14.25: rows 104 (2.5), 105 (1.75), 106 (3), 107 (4.5), 108 (1.5), 109 (0.5) and row 53's 0.5 |

**Range: about 120 to 150.** Before the answers it was about 108 to 132: 08-verify §6 puts its sum
at 95 to 114, 09 §7 puts the new work at 10 to 15, plus the realm's 2 (09 §7 "Pulled into the
plan"), and this file's corrections add about 0.75 net over 09's 117.25. The answers' 14.25 is
taken as 12 to 18 (the PR review view and the app settings pages are the least certain), which
gives 120 to 150. Verify round 1 of the answers added 2.5 of those days, all ours: the Portal-only
settings and approvals that rows 107 and 108 first missed (section 9).

**What the answers did not change in days.** No new subscription (`OD-42`) and the realm brokering
Panacea's instead of Entra (`OD-44`, equal at 2 days) move no row. The rename to Panacea (`OD-43`)
sits inside rows 19, 20 and 101. Penpot (`OD-45`) was already the plan. The trackers behind in-app
views (`OD-48`) were already the plan's shape.

## 2. The percentages

### 2.1 Percent done: about 11%

**Method.** The plan's 132 days are net of what is built: every cluster costed only what is still
to do (the inventory's key point, and 08-verify §6). So built code does not reduce the 132. It is
work already banked on top of it. Done is therefore banked days over banked plus remaining. Banked
days are the plan-hours of kept, built code tasks divided by 8, the convention this folder uses;
documents-only tasks and spikes are left out. That is conservative: real effort ran higher, with
fix rounds and hundreds of commits per branch.

| Branch | What is built and kept | Banked days |
|---|---|---|
| `feat/phase-2-cockpit` (with phase 1 and the sandbox git fix) | Control plane, sessions API, attach relay, transcript store, resume and fork, `sandboxd`, `dockerbe`. **An assumption, not plan-hours:** neither the phase 1 nor the phase 2 plan has an hour column. Part of it (sandboxd's PTY session hub, the attach relay) shrinks once `orca serve` owns the PTY (section 4), which is what the half-value case below covers | 5 |
| `feat/hub` | Tasks 01-07b: `hubapi`, the client and its fake, the migrate runner, schema, row-level security, the audit guards (22.5 h; 44a and 00 are documents only) | 2.8 |
| `feat/phase-4-discover` | Tasks 01-10 (24.5 h) less task 04 (1.5 h) and about 1 h of task 03's stepper ops, which lose their caller (03-define:80-85): 22 h | 2.75 |
| `feat/phase-7-deploy` | Tasks 01-06 (14 h) and part of 07: the deploy domain, transport, reads, acts | 2 |
| `feat/env` | Tasks 01-04, 09, 11, 16 and 28 (19 h): OpenTofu layout, bootstrap, network and observability modules, the realm file, platform chart, `internal/entra` (the base of `internal/oidcauth`), the pipelines and licence gate. Task 14 (pod hardening, 2.5 h) is built and not counted: agent-sandbox's template now carries the pod spec | 2.375 |
| `feat/phase-5-canvas` | Tasks 04 and 18 (7 h): the licence gate, the MCP relay. Tasks 02-03 (the canvas module and editor package) serve the sunk editor; task 01 is a spike | 0.875 |
| `feat/phase-3-git` | Tasks 02-03 (5.5 h): `gitx`, the lane index, clones. Task 01 is a spike | 0.7 |
| **Total** | | **16.5** |

**Arithmetic.** 16.5 / (16.5 + 132.275) = 16.5 / 148.775 = **11.1%, about 11%.** Valuing the
phase 2 backend at half (2.5) gives 14 / 146.275 = 9.6%; at double (10), 21.5 / 153.775 = 14.0%.
Hence 10 to 14%. The phase 2 figure is the swing factor and is not sourced. Nothing was built
between the two versions: done fell from 12.3% only because the remaining work grew by 14.25.

**Built code being replaced, and not counted as done:**
- Phase 2's Tauri Develop views, the whole `desktop/src` tree of 21,132 TypeScript lines, about
  13.2k of them tests (phase 2 tasks 14-24). Orca replaces
  them as the product surface. They are **frozen as the fallback cockpit**, not deleted
  (02-verify §5.1).
- Phase 5 tasks 05-06, the clean-room canvas editor start. Penpot replaces it: sunk cost
  (04-design §5).
- Phase 1's Kata node-pool Terraform, retired by phase 8 task 01.
- Part of `internal/entra` (phase 8 task 16), superseded by `OD-30` and reworked into
  `internal/oidcauth`.
- None of these is valued in days: the plans do not split their hours finely enough.

### 2.2 Percent from open source and percent we build

**Two views, and the headline.** The owner asked what share comes from open source and what share
we build. **The headline is the capability view**, because it answers "where does each piece of
the product come from". The effort view answers a different question, "what are the remaining
days spent on", and is shown so neither can be read as the other.

**(i) By capability: 110 rows in section 1.**

| Kind | Rows | Share |
|---|---|---|
| Open source | 38 | **35%** |
| Panacea reuse | 17 | **15%** |
| Managed service | 14 | **13%** |
| **Adopted, together** | **69** | **63%** |
| **Ours** | **41** | **37%** |
| Total | 110 | 100% |

Arithmetic: 38/110 = 34.5%, 17/110 = 15.5%, 14/110 = 12.7%, 41/110 = 37.3%. Rounded by largest
remainder so the four add to 100 (floors 34, 15, 12, 37 = 98; the two largest remainders, 0.73
and 0.55, go to managed services and open source). Adopted is 69/110 = 62.7%. Rows per cluster:
sandboxes 5 OSS, 1 managed, 3 ours; cockpit 7 OSS, 1 Panacea, 1 managed, 7 ours; Define 4 OSS, 1
Panacea, 4 ours; Design 8 OSS, 1 managed, 2 ours; work 2 OSS, 2 Panacea, 5 managed, 4 ours;
Deploy 1 OSS, 11 Panacea, 7 ours; platform 3 OSS, 1 Panacea, 5 managed, 6 ours; one-app 8 OSS, 1
Panacea, 1 managed, 8 ours.

**The owner's answers added six rows (104 to 109), all ours**, and changed no row's Kind (row 53
stays managed: Teams, with our bot, as rows 52 and 92). Before: 38 / 17 / 14 / 35 of 104, that is
37% / 16% / 13% / 34%, and 66% adopted. The same method applied to the same kinds; only the count
of our rows moved.

**Verify round 1 moved five rows** under the Kind rule of section 1: `katabe` (row 3) and the phase
7 connector (row 66) to Ours, Chains and Kyverno (row 67) to Panacea, the fonts (row 98) to open
source, and the gateway split into LiteLLM glue (row 76) and our `keysmith` and `gatekeeper` (row
76b). Before: 38 / 16 / 14 / 32 of 103.

**Challenge it here:** the count depends on how finely capabilities are cut. Section 1 cuts at the
granularity the cluster files cost at, so a large product (Orca, row 10) is one row, the same as a
half-day rename (row 20). That understates what the adopted pieces deliver. Two rows are borderline
the other way: rows 27 and 55 (the pending start through Orca's recipe) are our recipe scripts on
Orca's extension point, counted as Orca glue. As ours, open source is 33% (36/110) and ours 39%
(43/110).

**(ii) By remaining effort: 132.275 days.**

| Where the days go | Days | Share |
|---|---|---|
| Glue on open-source pieces | 31.65 | **24%** |
| Glue on Panacea | 5.875 | **4%** |
| Glue on managed services | 14.5 | **11%** |
| **Glue on adopted pieces, together** | **52.025** | **39%** |
| **Our own code** | **80.25** | **61%** |
| Total | 132.275 | 100% |

Arithmetic: 31.65/132.275 = 23.9%, 5.875/132.275 = 4.4%, 14.5/132.275 = 11.0%, 80.25/132.275 =
60.7%; adopted 52.025/132.275 = 39.3%. Rounded by largest remainder so the four add to 100
(floors 23, 4, 10, 60 = 97; the three largest remainders, 0.96, 0.93 and 0.67, go to managed
services, open source and ours). The answers moved managed glue by row 53's 0.5 and our code by
rows 104 to 109's 13.75 (66.5 + 13.75 = 80.25). Before: 27% / 5% / 12% / 56% of 118.025.
Open-source glue by product: Penpot 11.5, Orca 8.5 (rows 4, 8, 11, 19-21, 24, 27, 55, 93,
99, 102, 103), Spec Kit 3.25, Keycloak 2, LiteLLM 1.9, mirrors 1.5, cosign 1.25, agent-sandbox with
the smoke 1, claude-code-action 0.5, the fonts 0.25.

**The gateway split is from source.** The phase 6 plan's task table
(`feat/phase-6-gateway:docs/superpowers/plans/2026-09-25-phase-6-gateway.md:166-194`, 70 h)
separates the LiteLLM tasks (02, 04, 18, 27: 11.5 h) and two mixed ones (17, 19: 5.75 h) from
`keysmith`, `gatekeeper`, the token CLI, the panel and the proofs. With 07-verify's 2 h hash lock
on the LiteLLM side, LiteLLM glue is 18 to 26% of the gateway's 8.7, 1.6 to 2.3 days; row 76 takes
1.9 and row 76b the other 6.8. **Challenge it here:** with rows 27 and 55 counted as ours as well,
our own code by effort is about 62% (82.5 / 132.275).

**(iii) By size of software delivered: not measured.** The folder gives one size, Orca's 30,735
files at `59b746ff` (02-fleet-cockpit.md:76). It gives none for Penpot, Keycloak, LiteLLM, Spec Kit
or Panacea. On this branch the kept Go is 11,710 source lines in `control-plane/` and 6,234 in
`sandboxd/` (non-test, `git ls-files`); the other branches' kept Go is not counted here. The
21,132 TypeScript lines (about 13.2k tests) are the frozen fallback desktop, not kept product code.
By size, the adopted share is far above either view, but no figure is claimed.

## 3. Build order

### 3.1 Day 0: none of it engineering, and it sets the date

Asked on day 0, by the owner or the build lead, in this order of lead time:

1. **A new resource group in Panacea's existing subscription** (`OD-42`; no new subscription),
   tagged `panacea-app-estate=<env>`, with resource names that differ from every Panacea estate's.
   In that subscription: D-series v5 Kata vCPU quota in Australia East **with headroom beside
   Panacea's own pools** (the quota is per subscription, so it is shared), the
   `AllowBringYourOwnPublicIpAddress` registration (subscription-wide), and no FIPS or Trusted
   Launch policy reaching the Kata pool, by an exemption scoped to our resource group (01-verify
   §7 "Day 0", §8 item 9). Panacea's subscription owners can do all of it if any blocking policy
   is assigned at subscription scope, so this is days, not the week a new subscription risked.
   **If the blocking assignment sits at a KPMG management group** above the subscription
   (`KPMGAU-MGT-GOVCYBERINNO`, `deploy/environments/panacea-dev.env:22`), the exemption is an ask
   of KPMG cloud governance: exempting from an assignment needs rights on that assignment's own
   scope (from knowledge, not checked).
2. **KPMG IT and SOC:** an MDE suppression rule and an ASR allowlist for our signed build, and the
   code-signing route: Azure Artifact Signing, checking that the entity qualifies, else an OV
   certificate in Key Vault (call 4; 02-verify §6 "Day 0", §8 item 7).
3. **Panacea's side:** our AKS egress IP on `edge_allowed_ip_ranges`. **One block in
   `deploy/portal/setup-keycloak.sh`, through Panacea's own pipeline, for our Keycloak's broker**
   (`OD-44`; ruling §10's shape):
   - a confidential client for our broker, with the redirector-only browser flow straight to its
     `entra` provider (K1's flow shape) and P1/P2 as dedicated mappers, so its tokens carry `oid`
     and `tid` (K6);
   - **the ruling's K3, unchanged**: "Pass `login_hint`" on the `entra` provider (off today),
     M1/M2 importing `oid` and `tid` with `FORCE`, and the `email` mapper on `FORCE`;
   - **K4's user-profile items**: `entra_oid`, `entra_tid` and `email` editable by admins only,
     and no `ENABLED` unmanaged-attribute policy, in the same block and before K3.

   These stay in Panacea's realm because only it sees Entra's tokens. Without them its tokens to
   our broker carry no `oid` or `tid`, so every validator refuses them; `login_hint` is not
   forwarded; and the email the Hub's invite match trusts stays user-editable. What moves to our
   realm (row 82): the desktop client (K1), the four audience scopes (K2), the 7-day offline cap
   and `hub.org_admin`. Also asked: Keycloak 26 on two replicas (Panacea's realm is the identity
   source, and a fresh sign-in needs it up); `prod-deploy`, `serve-without-sso` and
   `external-domain` left unmapped in F4's step-up map on the estates we use (section 5.2, forced
   exception 4); and P-1 to P-3 queued behind the cutover freeze, **P-3 first**, because it
   removes "Connect deploy", a forced exception (section 5.2) (00-verify §7 P5; 06-verify §7 item
   5; 07-verify §5). Forced exceptions 3 and 4 are now superseded by `OD-53` (section 10), so
   which of these asks still stand is `10-panacea-in-app.md`'s to say. **Stall risk:** real users' sign-in now waits on this block, and the plan has
   no fallback for it. 07-verify §8's fallback, our realm brokering Entra with IT's redirect URI
   and second secret filed on day 0, is withdrawn by `OD-44`. The build does not wait, because
   `devissuer` runs the same flow (ruling §10). If the block stalls, that IT request (ruling §9)
   is the only route left; its text stays drafted, not filed, and filing it is the owner's call.
4. **No IT request for sign-in.** The redirect URI and second secret that our own realm needed are
   not asked: our Keycloak brokers Panacea's realm, not Entra (`OD-44`).
5. **The rest:** a DNS name and edge allowlist for the Hub and gateway (07-verify §5); our own
   Australian ADO organisation and its admins (`OD-48`), the KPMG PAT policy (global PATs stop
   2026-12-01), a GitHub App owner, a Teams admin, and a member-readable roster route from Panacea
   (05-verify §8); at least one inference provider's endpoint and key for the provider page, which
   the owner holds (KPMG Workbench today; Anthropic with a zero-retention agreement and Bedrock
   Sydney as he chooses, `OD-46`).

### 3.2 Before the build

**The first build step is phase 2's close-out and merge to `main`** (tasks 25 and 26, the final
review and X-3; about a day), as the assembly's base. That is sequencing, not an owner gate (call
1). Then the five streams start (`OD-31`'s cap is now five).

The owner reviews a clickable prototype of the shell: the bar, the Desk, one Hub page and the
Develop frame (09 §6; 08-verify §8; `CLAUDE.md` working agreements). Day-1 spikes run beside it:
the browser route (0.5), the realm hand-off and the Windows bridge on a managed laptop (the
Windows-native sign-in, which may remove forced exception 1's weekly tab), the journey bar on the
first Orca pin, and Penpot's headless runner (09 §8; 04-verify §5).

### 3.3 Five builders: what each starts on

| Builder | Weeks 1-2: the first app | Weeks 3-5: the rest |
|---|---|---|
| **B1, sandboxes** (the long pole) | Rows 1-9 in 01-verify's order: cluster, agent-sandbox, `katabe`, image, egress, worktree, broker, recipe, smoke. About 10 days from the resource group. If day 1 has no Kata nodes or claim p95 is over about 5 s, the pre-approved pilot fallback is ACI with standby pools, with the isolation statement saying ACI, back to Kata before customers (`OD-52`) | Penpot's headless runners in the Kata pool (row 39), then Design stream B |
| **B2, our Orca build** (its own stream from day 1) | Rows 19-24, 102-103, 93: identity as Panacea, rename, theme, journey bar and app-level view, OSC off, surfaces off. Then row 99 (the palette) | Design stream A (rows 34, 36-38, 44), then rows 87, 94, then the in-app PR review (row 106) |
| **B3, the lane path** | Rows 11-18: rollout, our CLI's `login`, lane git endpoint, landing, Panacea preview; then the plugin, resume and fork, AI review | Define (rows 25-33), then the tracker Start (rows 48, 55) |
| **B4, the Hub** | Row 74's minimum Hub (about 5 working days on two engineers, 08-verify §6), row 75, row 88, the cost codes' Hub side and creation form (row 105), then the Desk (rows 89-91) and the thread drawer (row 100) | The rest of the Hub, then the Work page (rows 47, 50), team settings and approvals (row 108) and Design's approval rows 41-42 |
| **B5, gateway and Deploy** | Rows 76 and 76b's minimal gateway and the provider page (row 104), then Deploy's first-ship glue (about 5 of rows 61-73, with row 105's hand-off to Panacea), the realm brokering Panacea's (row 82) and the hand-off (row 95) | The rest of Deploy, the app settings and production approval (rows 107, 109), the tracker connectors (rows 45-46), the Teams bot (rows 52, 53, 92) |

**Critical path.** Journey steps 2, 5 and 7 all wait on the Hub service (08-verify §2 row 3), and
the first real session waits on B1. The Desk and the realm hand-off sit on the first app's path,
which is why 09 §7 adds 2 to 3 working days.

**Calendar.**
- **Five builders:** the first app (journey steps 1-7) in about 13 to 17 working days after day 0
  and phase 2's merge (08-verify §6's 10-12, plus 09 §7's 2-3, plus 1 to 2 for rows 104 and 105,
  4.25 engineer-days on the first-app path split between B4 and B5). All of it in about 26 to 27
  working days (08-verify's 20-21 for 103 days, scaled to 132.275: 25.7 to 27.0). Five streams are
  within `OD-31`'s cap, now five (call 1). **Since found low** (`10-panacea-in-app.md` §2.6): five
  builders deliver at most five engineer-days a working day, so 132.275 days need at least 26.5,
  and about 28 to 31 at 85 to 95% of a day each; and B5 below carries about 19.7 engineer-days of
  first-app work, so 13 to 17 holds only if the first-app work is spread across all five.
- **Two builders:** the first app in about 5.5 to 6.5 weeks (08-verify's "about a month", plus
  about 13.75 engineer-days on the first-ship path: rows 82, 89, 90, 91 and 95, 2 + 3 + 1.5 + 1.5 +
  1.5 = 9.5, and rows 104 and 105, 2.5 + 1.75 = 4.25; 13.75 / 2 = about 7 working days). All of it
  in about 13 weeks (132.275 / 2 = about 66 working days).
- **Rows 106 to 109 and row 53 (10 days) sit after the first app**, in weeks 3-5: the first app's
  journey has one author and a class-`none` pilot, so reviewers, app settings, team settings and
  production approval are needed before the pilot widens, not before its first walk.

### 3.4 The acceptance gate

09-cohesion §6 is the gate for v1. Walk J1 to J9 on the pilot build from a cold start on a managed
KPMG laptop, and check K1 to K14 at every step. K1 to K13 are automated inside row 86 and run on
every Orca or Penpot pin move. **K14 is the owner's eyes, and a no fails the build whatever the
others say.** Until that walk, every visual claim stays unverified (`OD-12`; `CLAUDE.md`).

**K2 and K3, restated for section 5.2.** 09 §6 allows two exits, the first-run tab and a
reviewer's host PR page. The reviewer's exit is gone (row 106), and section 5.2 names the forced
ones, so the only allowed exits are now: the first-run sign-in tab and its weekly re-check; Azure
DevOps' own page where a person creates the PAT; Connect deploy in the system browser until P-3;
and, only if a platform admin arms Panacea's step-up gate, the gated acts on the Portal. K2's only
allowed token pastes are the ADO PAT and Connect deploy's key, each on our page. K2 still counts
any sign-in page inside the frame as a failure: had Connect deploy embedded Panacea's key page,
as the previous version of this plan had it, K2 would fail at J7 for every new user (section
5.2, forced exception 3). **Since the owner's second round (section 10), only the first of these
exits is accepted.** The other three, and both token pastes, are superseded and being scoped in
`10-panacea-in-app.md`; the gate's target is the sign-in tab alone.

Other gates: a security review of the agent sharing a sandbox with `orca serve` before any
customer tenancy (02-verify §8 item 8); a security review of the hand-off authenticator (09 §4.1);
Panacea's P-3 realm trust and legacy-key mint guard before first customers (00-verify §9).

## 4. What happens to each branch

| Branch | Fate | What carries on |
|---|---|---|
| `feat/phase-2-cockpit` (incl. `feat/phase-1-live-session`, `fix/sandbox-git-ca`, merged) | **Kept, in part.** The base of the assembly | Everything under `control-plane/` and `sandboxd/` becomes the lane and sandbox API the recipe calls (02 R4). `sandboxd` shrinks, because `orca serve` owns the PTY for Orca lanes. `katabe` is fixed in row 3. `tokens.css` is the source for every generated theme. The Tauri Develop views are **frozen** as the fallback cockpit |
| `feat/hub` | **Kept.** Resumes first | Tasks 08-10, 13-24, 28-35, 37 and 39 are row 74 (07-verify §5). Tasks 25-27 and 36 are outside the total (section 1); task 38 and 40-42 (Tauri UI) drop under Orca. Tasks 11-12 (`devissuer`) drop for a Keycloak dev container |
| `feat/phase-4-discover` | **Kept.** Tasks 01-10 stay | Spec Kit replaces the unbuilt Define UX (12 of 22 unbuilt tasks, about 71 of about 109 h). Rows 28-33 finish the thread |
| `feat/phase-7-deploy` | **Kept.** | Tasks 07 rest, 08-10, 19-20 are row 66; 11, 12, 17 and 18 come back as row 61. Tasks 13-16 are removed |
| `feat/env` | **Kept, mostly.** | Tasks 05-08 start from Panacea's `.tf` under OpenTofu, and are rows 1 and 78 (one AKS). The realm file seeds the dev Keycloak. `internal/entra` becomes `internal/oidcauth`. Tasks 17-18 are row 7. Task 14's pod hardening moves into agent-sandbox's template |
| `feat/phase-5-canvas` | **Kept in part; the editor is sunk.** | The licence gate (task 04) and the MCP relay (task 18, re-pointed at Penpot, row 38) stay. Tasks 05-06 and `OD-34`'s op-log are replaced by Penpot (`OD-45`) |
| `feat/phase-3-git` | **Kept.** | Tasks 01-05, 07, 11-15, 23 stay (clones per lane; 12-13 now serve the laptop CLI). Orca replaces 06 and 18-22; the git host replaces 08 and 10 |
| `feat/phase-6-gateway` | **Kept as a plan, unbuilt.** | All 29 tasks are rows 76 (the LiteLLM tasks) and 76b (ours); tasks 04 and 07 start from Panacea's key-minting code |
| `feat/team-mode` | **Paused.** No code of its own | Phase T's tracker, queue and branch-policy tasks go to the trackers, Teams and the git host (05-work-team). The Orchestrator core comes after team v1 on the tracker (`OD-51`) |
| `feat/work-layer` | **Paused; plan largely replaced.** No code | 127 of 190 Phase W tasks go to ADO Boards, GitHub Issues and Teams behind our in-app views (`OD-48`) |

Each worktree under `.claude/worktrees/` is left as it is. Merges into the assembly happen when
each stream resumes, not in this change.

## 5. Owner calls

**The owner answered every call on 2026-10-01.** Calls 1 to 5 are quoted from the owner's message;
calls 6 to 20 are his own answers of the same day, given through the in-session question form,
and quoted as he wrote them; rows 104 to 109 wait on nothing. The register records each answer
as an `OD`, or as a `note` entry in its "answered, not gated" list where the answer fails the
three-part gate (`2026-09-23-owner-decisions.md`, the section after `OD-41`). **His second round,
the same day** (section 10), answered sections 5.2 and 5.3: it accepts forced exception 1, does
not accept 2, 3 and 4, closes section 5.3's questions, and adds `OD-53`. What remains open is
`OD-53`'s shape and cost (`10-panacea-in-app.md`) and section 5.3's "Later" list.

### 5.1 Answered

| # | Call | Answer, in the owner's words (calls 6 to 20 through the in-session question form, 2026-10-01) | Recorded as |
|---|---|---|---|
| 1 | Staffing, and `OD-31`'s four-stream cap | *"Lift it to 5 then."* Five builders; the cap is five. Phase 2's close-out and merge to `main` stays the first build step, as sequencing (section 3.2) | Answered, not gated; amends `OD-31` |
| 2 | The Azure subscription and its US$2,000 a month ask | *"Don't worry, I will use our existing Panacea subscription, we will just make a seperate resource group."* Our own resource group and our own AKS, not Panacea's AKS, so `OD-5`'s own-environment intent holds; no new subscription | `OD-42` |
| 3 | The app's name and icon | *"We will continue calling it Panacea. This is the evolved version of Panacea. We can deal with a new logo/icon later."* Rows 19, 20 and 101 updated; the icon does not block; our CLI is not named `panacea`, which Panacea's own CLI already is (row 12) | `OD-43` |
| 4 | Code signing | *"Sure, do that."* Azure Artifact Signing; an OV certificate in Key Vault if the entity does not qualify | Answered, not gated |
| 5 | Our own realm for the hand-off | *"Do we need that? Why would we not just piggy back off of Panacea?"* We piggyback: our small Keycloak's only identity provider is Panacea's realm. No Entra change, no IT ticket, one sign-in. Equal cost, 2 days (row 82). Panacea's realm keeps the ruling's K3 and K4's user-profile items (day 0 item 3) | `OD-44`; amends `OD-30` |
| 6, 6a | Penpot as the canvas; frames as vectors with CSS flex and grid | *"Accept Penpot (Recommended)"*: Penpot for v1, MPL-2.0 admitted, HTML through export; OpenPencil revisited at 1.0 | `OD-45`; relaxes `OD-11`, withdraws `OD-34` for v1 |
| 7 | Penpot's radius and font | Accepted (*"Accept all"*) | Answered, not gated |
| 8 | The six declared v1 exceptions | Accepted, then re-tested under the restated one-app rule (section 5.2), which wins | `OD-49` |
| 9 | Define, not Discover; four lenses | Accepted | Answered, not gated |
| 10 | Geist Sans and Commit Mono | Accepted | Answered, not gated; amends `OD-28` |
| 11 | Orca's OSC status until row 23 | Accepted for the KPMG pilot, display only | Answered, not gated |
| 12 | Land from the laptop | Accepted; `OD-6` unchanged | Answered, not gated |
| 13a | Pilots on the dev estate | Accepted | Answered, not gated |
| 13 | Who can open a shipped app | Accepted: KPMG network only in v1; previews GitHub-only | Answered, not gated |
| 14 | Who pays | *"We will use Panaceas Subscription and Resource Group as we currently do. But from day one, we will still need the Cost Centre, Job Code and Phase map through for costing like we have in Panacea"* Row 105, on Panacea's model: optional codes, the team's cost centre. Whether to go further was asked in section 5.3 item 2, and closed on Panacea's model | `OD-47` |
| 15 | Work: retire `OD-26`'s tracker; own AU ADO organisation | *"Accept all, except ADO Boards, idk what that means."* Explained: nobody opens ADO Boards; tasks live and are worked inside Panacea, with ADO Boards and GitHub Issues as storage and sync behind it. Recorded on those terms. No veto: in the second round the owner chose *"Also in ADO/GitHub (Recommended)"* (section 5.3 item 4) | `OD-48` |
| 16 | Team mode v1 on the tracker; Panacea's teams | Accepted | `OD-51` |
| 17 | Inference accounts | *"Give us the option for any. We should have a litellm backing, where I can manage the API endpoints and keys, so I can shuffle them around and no stick to one purely."* Row 104. `OD-19`'s user trigger stays, as a switch between provider classes | `OD-46`; amends `OD-19` |
| 18 | Define's four items | Accepted | Answered, not gated |
| 19 | The sandbox fallback | Accepted: ACI with standby pools for the pilot only, on the day-1 trigger | `OD-52` |
| 20 | How far `OD-40`'s copying reaches | Accepted: whole services and Terraform stacks, with the ledger and attribution; `CLAUDE.md`'s reuse section updated | `OD-50` |

**The one-app rule, restated.** *"remember that I want as much of it to be in app and feel like it
is all in app, never does it feel seperate, or needing to go outside to do anything. Okay?"*
(`OD-49`, strengthening `OD-41`).

### 5.2 The one-app re-test (`OD-49`)

Every declared exception (09 §7) and every row of section 1 and of 09 §2 was re-tested against
"never needing to go outside". **Pulled into the app: 12.5 engineer-days, five new rows (104 and
106 to 109, 12) and row 53's 0.5.** The sixth new row, 105 (cost codes, 1.75), is `OD-47`'s.

| Where a user went outside, or would have | Now | Days |
|---|---|---|
| Exception 1: reviewers on the host's PR page (09 §2 row 10) | **In-app PR review**, row 106 | 3 |
| Row 51: team members, invitations, budget, cost centre had no surface but Panacea's Portal; nor did approving a join request (team admin) or a new team (platform admin), which every new user waits on (00-verify S2) | **Team settings and approvals in-app**, row 108; each approval a NEEDS YOU row on the Desk | 1.5 |
| Rows 57 and 58: the shipped app's environment variables, secrets, flags, app users and connection details, and its access bindings, ownership, IP allow-list, SSO enforcement, protection, custom domain, backups and tier had no surface but Panacea's Portal | **App settings in-app**, row 107 | 4.5 |
| Production-rung approval on Panacea's Portal (06-verify §6) | **On our Deploy page**, row 109 | 0.5 |
| Row 53: the first-party Boards and GitHub apps' Teams cards open ADO Boards or GitHub | **Our bot's cards** open the Desk; the first-party apps are not installed | 0.5 |
| Provider endpoints and keys on LiteLLM's own UI | **The provider page**, row 104 (`OD-46`) | 2.5 |
| Exception 5: Keycloak's stock error pages | **Closed for our realm**: our Keycloak carries our theme from day one (rows 82-83, `OD-44`). **Panacea's realm pages remain**, in the system browser only: on a Panacea realm error at sign-in (forced exception 1 below), and at Connect deploy once the realm session there has lapsed (forced exception 3, now superseded, below) | 0 |
| Exceptions 3 and 6: Penpot's font and radii; Spec Kit's file paths | **Inside the app already.** They are look and wording, not a step outside; they stay as accepted in calls 7 and 8 | 0 |
| 09 §2 row 17: Teams | **Passes.** An optional doorway back in; every card's act is also on the Desk, so no one has to use Teams | 0 |
| 09 §2 rows 13, 15, 16: Panacea's Portal, Keycloak, LiteLLM | **Pass, with the exits named.** Headless and never linked. Rows 107-109 cover the Portal acts traced in Panacea's routes on 2026-10-01 (read for these groups, not exhaustively). What still needs the Portal is listed below: Connect deploy until P-3 (forced exception 3), and step-up-gated acts if a platform admin arms that gate (forced exception 4); both are now superseded (below). **Since found not to hold** (`10-panacea-in-app.md` §1.4): the plan brings about 90 of Panacea's 357 user routes into the app, so about 28 engineer-days of in-app pages are owed under `OD-49` whatever the owner answers on `OD-53`'s cost; they are not yet in this file's total | 0 |

Every other row of section 1 is headless behind our screens, embedded in our frame, or
infrastructure with no user act (rows 1-9, 35, 56, 59, 67, 78-80).

**Kept, because they are technically forced. For the owner's eyes** (numbered here as forced
exceptions 1 to 4; 09 §7's own numbering is used only in the table above). **The owner answered
them on 2026-10-01, second round (section 10): he accepted 1 and did not accept 2, 3 and 4.**

1. **The sign-in, in the system browser: on first run, and again at least once a week.**
   **Accepted by the owner** (2026-10-01, second round, through the in-session question form):
   *"Accept weekly tab (Recommended)"*. The day-1 Windows-native sign-in spike (the Windows sign-in
   bridge, section 3.2, 09 §4.2) stays, and may remove the tab if it passes. One tab
   each time, zero clicks, showing our own "you can close this tab" page after Entra signs in
   silently. The weekly tab is the ruling's 7-day cap on the desktop client's offline session,
   which cuts a leaver off within a week: when the silent refresh fails, the zero-click flow
   re-checks the person against Entra (`2026-09-25-identity-keycloak-broker.md:374-379`, `:494`).
   Lifting the cap would make the tab once per laptop and reopen that leaver window, so it stays.
   For a GitHub project, GitHub's one-time App authorisation rides the first run (not verified).
   **Why forced:** a native app's OAuth sign-in must use the system browser (RFC 8252), and only
   there, or in Windows' own account broker, does Entra get the device's single sign-on and
   satisfy Conditional Access's device-compliance checks. Inside our frame it would be a password and MFA prompt, and fail
   compliance. The in-app alternative, Windows' account broker with a token exchange into our realm,
   needs Keycloak's external-to-internal token exchange across two realms, a preview feature, and
   is not taken. If Panacea's realm errors during that sign-in, its stock page shows in the tab.
2. **An Azure DevOps personal access token, pasted once per person** (and on renewal), for ADO
   projects. **Superseded by the owner's answers of 2026-10-01 (second round), being scoped in
   `docs/superpowers/planning/2026-09-30-assemble/10-panacea-in-app.md`.** Owner: *"Instead, could
   we build in an authenticator, because people would authenticate through EntraID, could we have
   them SSO into Github or ADO, and that automatically connect?"* As it stood: the paste is in-app, on our page, with the scopes named and the expiry warned; the
   token is created on Azure DevOps' own page. **Why forced:** Microsoft lets a PAT be created only
   there, or through its API with a user-delegated Entra token for Azure DevOps, which needs
   consent on the Entra registration: the IT ticket that `OD-30` and `OD-44` avoid. **Closes if**
   the owner asks IT for that consent; an in-app token flow is then about 1 day (not in the total).
3. **"Connect deploy", in the system browser, until Panacea's P-3: once per person and at each
   renewal** (a Panacea key lasts 90 days by default, 365 at most, 00-verify §2 row 5).
   **Superseded by the owner's answers of 2026-10-01 (second round), being scoped in
   `docs/superpowers/planning/2026-09-30-assemble/10-panacea-in-app.md`** (`OD-53`). Owner: *"Huh,
   maybe you have the wrong idea. I want Panacea to be ingrained into this. I do not want Panacea
   to be this external site that is used for Deployments. I want it to be first classed, inside
   the app, so everything is done within the app. No API key, nothing. We will build our own
   Panacea as a part of this."* As it stood: our app
   opens Panacea's key page in the system browser; the person mints a key there, copies it, and
   pastes it on our page, which validates it, shows its expiry and warns before it lapses. **Why
   forced, and why outside the frame:** Panacea's API accepts only its own pasted key today and
   refuses a realm token (00-verify §4, `auth.go:245-251`), and a key is minted only by a person
   signed in to Panacea's Portal (`api_keys.go:112-116`). The Portal sits behind Panacea's realm,
   and the realm hand-off (row 95) makes a session only in ours (`OD-44`), so inside our frame the
   key page would show Panacea's Keycloak page and then a Microsoft password and MFA prompt, which
   fails compliance (forced exception 1) and K2 at J7 for every new user. The system browser has
   Entra's device sign-in, and the Panacea realm session that the sign-in tab leaves there while
   it lasts (dev: 12 hours idle, 24 at most); after that, Panacea's own Keycloak page shows there
   first. **Closes when** P-3 lands, code in Panacea (2 to 3 of Panacea's days); day 0 item 3 asks
   for it first. Under `OD-44` our broker can hold the user's token from Panacea's own realm, so
   P-3 becomes "trust your own realm's token". **Could move into the frame sooner** if the Windows
   sign-in bridge's day-1 spike passes (1 day, outside the total) and Panacea sends
   `kc_idp_hint=entra` on the Portal's sign-in (a Panacea change; whether config or code was not
   traced).
4. **Step-up-gated acts on the Portal, only if a platform admin arms Panacea's step-up gate.**
   **Superseded by the owner's answers of 2026-10-01 (second round), being scoped in
   `docs/superpowers/planning/2026-09-30-assemble/10-panacea-in-app.md`** (`OD-53`). Owner, in the
   same answer as exception 3: *"I do not want Panacea to be this external site that is used for
   Deployments. I want it to be first classed, inside the app, so everything is done within the
   app."* As it stood: Panacea's F4 gate refuses every API-token caller on an action mapped to an authentication
   context: "needs a fresh sign in, which an API token cannot provide. Run this action while
   signed in to the portal" (`handlers/stepup.go:205-213`, `:271`). The mappable actions include
   `prod-deploy` (a production ship, `handlers/projects.go:2927`), `serve-without-sso` (SSO
   enforcement and access mode, row 107) and `external-domain` (the custom domain, row 107). Our
   control plane calls with the user's `pnc_` (`middleware/api_key.go:189`), so a mapped action
   would send that act to the Portal. **Why forced:** an API token can never satisfy an
   authentication context, and after P-3 the realm token we would present still carries no Entra
   `acrs` claim unless both realms map it through.
   **Today it does not bite:** the gate ships inert, and Panacea's own `deploy/f4-step-up.sh`
   declines to map `prod-deploy` because it "stops every production deploy that runs on a pnc_
   API token"; the live map is runtime data and was not checked. **Closes if** Panacea keeps those
   three actions unmapped on the estates we use (day 0 item 3 asks), or accepts after P-3 a realm
   token whose `acrs` is mapped from Entra through both realms.

### 5.3 Still open

**Questions for the owner, as asked, and how his second round (2026-10-01, section 10) closed
them.** Items 2 and 3 were closed by the orchestrator's rulings, which the owner may override.

1. **Confirm calls 6 to 20 in your own words.** **Closed.** Calls 6 to 20
   are his own answers, given through the in-session question form earlier on 2026-10-01,
   and quoted as he wrote them: Penpot, *"Accept Penpot (Recommended)"*; inference, *"Give us the
   option for any. We should have a litellm backing, where I can manage the API endpoints and keys,
   so I can shuffle them around and no stick to one purely."*; who pays, *"We will use Panaceas
   Subscription and Resource Group as we currently do. But from day one, we will still need the
   Cost Centre, Job Code and Phase map through for costing like we have in Panacea"*; and the
   rest of calls 6 to 20, *"Accept all, except ADO Boards, idk what that means. but remember that
   I want as much of it to be in app and feel like it is all in app, never does it feel seperate,
   or needing to go outside to do anything. Okay?"* Rows 104 to 109 (104 and 105 on the first
   app's path) wait on nothing. Call 20 (`OD-50`, how far `OD-40`'s copying reaches) was first
   left on its old tag; `10-panacea-in-app.md`'s verify round (§7) corrected it.
2. **Cost codes: required, and per project?** You asked for Cost Centre, Job Code and Phase to
   *"map through for costing like we have in Panacea"*. In Panacea the job code and phase are
   optional, and the cost centre is the team's (`handlers/project_billing_codes.go:22`,
   `handlers/billing.go:104`). **Closed: Panacea's model** (optional job code and phase, the cost
   centre from the team), which row 105 builds, and no Panacea ask. It is the orchestrator's
   reading of *"like we have in Panacea"*, a ruling the owner may override.
3. **Where an answer that fails the register's gate is recorded.** Asked on 2026-09-28
   (`2026-09-25-owner-questions-pending.md`, the register-work item): (a) every answer an `OD`,
   the audit naming the notes; (b) a failing answer goes into the plan it governs, with no number;
   (c) one register with a `note` marker on the row. **Closed: (c)**, the orchestrator's
   bookkeeping ruling, which the owner may override. The register's "answered, not gated" list is
   final in that form.
4. **The veto on `OD-48`:** tasks living only in the app, the Hub's records as the store, the
   trackers as optional sync, from about 4 days (this file's estimate) up to about 44 if it grew
   into Phase W's tracker (`05-work-team.md:142`, `:176`). **Closed: no veto.** Asked whether tasks also live in ADO and GitHub, the owner chose
   *"Also in ADO/GitHub (Recommended)"*. `OD-48`'s retirement of `OD-35` for Phase W stands
   unconditionally, and those days are not spent.
5. **The four forced exceptions of section 5.2.** **Answered.** Exception 1, the weekly sign-in
   tab, accepted (*"Accept weekly tab (Recommended)"*), with the day-1 Windows-native sign-in spike
   kept. Exceptions 2, 3 and 4 not accepted: superseded by the owner's answers of 2026-10-01
   (second round), being scoped in `10-panacea-in-app.md` (section 10).

**Open:** `OD-53`'s technical shape and its cost, and the Azure DevOps authenticator's, both in
`10-panacea-in-app.md`, for the owner's sign-off on the cost.
**Pending the owner's sign-off:** `10-panacea-in-app.md` (2026-10-01) proposes both, with costs.

**Later: before the first customer, not before the first app.** Penpot Enterprise quote (from
US$950 a month) and its open-core admin console (04-verify §5 calls A-B); licence treatment of
Semgrep rules, Vault BUSL, MinIO, Grafana and ClamAV before any customer-hosted tier (00 §13 call
4; 06-verify §7 item 6); the customer isolation statement (01 §6 call 4); the WAF_v2 edge at
about US$381 a month (07-verify §8); GitHub Enterprise Cloud at US$21 a user a month for queued
landing (02 §7); warm-pool size and idle policy (01 §6); no whole-product seats, GitLab only on
demand, and a quarterly "buy" re-check (08-verify §7.4 calls 1, 5, 6); Octopus Deploy only if a
customer mandates ServiceNow tickets (06-verify §7 call 4); a Figma adapter only when a customer
asks (04-design §7); the new icon (`OD-43`).

## 6. Not verified

- **Nothing was installed, built or run** for this plan, and no test was run for the inventory.
  "Working" rests on committed tests and the plans' as-built records.
- **Every day figure is an estimate composed from estimates.** The range is about 120 to 150.
- **The banked days** are plan-hours divided by 8, not measured effort. Phase 2's 5 is an
  assumption: its plan has no hour column.
- **The capability view depends on row granularity** (section 2.2). A different cut moves it.
- **The gateway's split** (rows 76 and 76b) applies the phase 6 plan's task hours to 07-verify's
  8.7; the two are not reconciled line by line.
- **The named gaps** of section 1 (preview branch data, the class-mismatch refusal, a repo for a
  person from nothing, a `protected` preview inside the frame) are real work with no day figure.
- **The sum corrections in 1.9** (00's unique rows, 07's 8.7, the hook relay's 1.5, C4's 0.5, and
  verify round 1's rows 12, 73 and 74) are this file's reading of the verify documents. 08-verify
  and 09 did not have them.
- **The calendar** assumes day 0 is clear and five builders share no bottleneck besides the Hub.
- **Rows 104 to 109 are this file's estimates**, made on 2026-10-01 from Panacea's routes and the
  cluster files, with no cluster verify behind them. The PR review view (106) and the app settings
  (107) are the least certain. Their verify round added 2.5 days to rows 107 and 108 for
  Portal-only acts found in Panacea's routes; those routes were read for the groups named, not
  exhaustively, so another Portal-only act may still exist.
- **The Keycloak-to-Keycloak broker** (`OD-44`): chained provider hints, `login_hint` pass-through
  and auto-link in both realms keeping zero clicks is from knowledge of Keycloak, not tried; so is
  K3 and K4 in Panacea's realm carrying `oid`, `tid` and a trusted `email` through to ours. The
  day-1 spike of the realm hand-off (section 3.2) should cover it.
- **Panacea's realm admins can become any of our users, permanently** (`OD-44`; ruling U-15).
  They can set any user's `entra_oid` or `entra_tid`, or impersonate a user, in Panacea's realm,
  and our realm imports those claims. The ruling bounded this trust "until phase 8"; under
  `OD-44` it lasts as long as our realm brokers Panacea's. Nothing in our app can detect it.
- **Shared-subscription limits** (`OD-42`): the Kata vCPU headroom beside Panacea's pools and any
  policy on the Kata pool are not checked, nor is the scope of any blocking assignment. If it is a
  management group's, the exemption is KPMG cloud governance's ask (day 0 item 1); that exempting
  needs rights at the assignment's scope is from knowledge.
- **Panacea's step-up map** (forced exception 4, now superseded by `OD-53`): whether `step_up_auth_contexts` maps
  `prod-deploy`, `serve-without-sso` or `external-domain` on the estates we use is runtime data,
  not read.
- **LiteLLM's `count_tokens` defect** (row 104): that the pinned LiteLLM still carries #31149 and
  #42073 is assumed from `OD-33` (verified open on 2026-09-28), not re-checked.
- **`OD-53`'s shape and cost** (section 10) are not scoped. No figure in this file carries them,
  and the Deploy rows (section 1.6) are the plan before it.
- **Our CLI's name** (row 12): only that it must not be `panacea` is settled; the name itself is
  row 12's to pick. No user needs to type it: the app, the recipe and the sandbox call it.
- **Every visual claim** waits on K14, the owner's eyes (`OD-12`). Headless screenshots are
  blocked on the corporate network.
- **Each cluster's own "not verified" section stands:** 00-verify §10, 01-verify §9, 02-verify §9,
  03-verify §8, 04-verify §6, 05-verify §10, 06-verify §8, 07-verify §9, 08-verify §8, 09 §8.

## 7. Sources

- **This folder:** all 19 files, `00-panacea.md` to `09-cohesion.md`, in the sections cited. Figures
  re-read at `01-sandboxes-verify.md` §7, `02-fleet-cockpit-verify.md` §6-8,
  `06-deploy-verify.md` §2.5, `07-platform-verify.md` §5, `00-panacea-verify.md` §7,
  `08-whole-product-verify.md` §6-7, `09-cohesion.md` §0-7.
- **The register:** `docs/superpowers/planning/2026-09-23-owner-decisions.md`, `OD-1` to `OD-53`.
- **Panacea's source, read 2026-10-01** (`Projects/Panacea/services/panacea-api`):
  `handlers/project_billing_codes.go`, `handlers/projects.go:606-629` and `:2894-2927`,
  `models/models.go:652-660`, `clients/kubernetes.go:84-88` and `:2337-2358`,
  `handlers/billing.go:104`, `handlers/billing_statement.go:39-60`, `handlers/stepup.go:30-100`
  and `:163-284`, `handlers/project_security.go:100-110`, `middleware/api_key.go:185-190`,
  `routes/routes.go` (`:1246-1257`, `:1350-1357`, `:1398-1442`, `:1899-1971`, `:2410-2459`, the
  route wrappers `:3226-3355`). Beside it: `deploy/f4-step-up.sh` (header),
  `deploy/environments/panacea-dev.env:22`, `services/panacea-cli/cmd/root.go:56` and
  `cmd/login.go:16-31`.
- **The tree:** a read-only inventory on 2026-10-01 of every branch (`git worktree list`,
  `git ls-tree -r`, `git diff --name-status --numstat` against each merge-base with
  `feat/phase-2-cockpit`, `git log`), and the task-hour tables of the phase 3, 4, 7 and 8 plans and
  the Hub plan.
- **Project rules:** `CLAUDE.md`, `CONTEXT.md`, `docs/DESIGN-SYSTEM.md`.

## 8. Verify round 1 (2026-10-01)

Two refuters re-derived every figure. Their arithmetic of the first version held (103 rows,
119.275 days, 12.25% done); the corrections were to classification, double counts and sources.
Applied in place: the gateway split (rows 76, 76b); rows 3, 66, 67 and 98 reclassified; row 12
against Hub tasks 28-31; row 74 less task 38; row 73 against row 96; one AKS for rows 1 and 78;
the banked table traced task by task; the named gaps; owner calls 1, 2, 6a, 13a, 19 and 20; the
citation fixes. **Not applied, with the reason:**
- Row 9 (the smoke) to Ours: a smoke of adopted pieces is glue on them, as the canaries in rows
  31 and 43 are (the Kind rule, section 1).
- Row 35 (Valkey, S3Proxy) to open source: the row's main store is Azure Flexible Server, and the
  row carries 0 days.
- Rows 27 and 55 to Ours: they are recipe scripts on Orca's extension point; the effect of moving
  them is stated in section 2.2 instead.

## 9. The owner's answers, verify round 1 (2026-10-01)

A refuter re-checked the owner's answers as this file, the register and `CLAUDE.md` applied them.
Its arithmetic held (110 rows, 38 / 17 / 14 / 41 by Kind; 129.775 days; 16.5 / 146.275 = 11.3%).
Each finding was checked at source before it was applied. The total moves from 129.775 to
132.275 days, all of the 2.5 in our own code; no row was added, so the capability view stands.
**Applied in place:**
- **Connect deploy opens Panacea's key page in the system browser** (forced exception 3). The
  realm hand-off makes a session only in our realm, so in the frame that page would have shown
  Panacea's Keycloak page and a Microsoft sign-in, failing compliance and K2 at J7. Row 96 has no
  end condition; exception 5 is closed for our realm only; the Windows bridge's trigger is
  corrected; "Panacea's realm trusting ours" is struck as a remedy (the one-IdP invariant).
- **Day 0 item 3 asks Panacea for the ruling's K3 and K4's user-profile items** beside the broker
  client, since only Panacea's realm sees Entra's tokens, and states the stall risk. `groups` is
  dropped from the chain: the ruling's token profile leaves it out, and `OD-51` reads teams
  through Panacea's team API.
- **Rows 107 and 108 gain the Portal-only acts found in Panacea's routes:** access, security,
  protection, domain, backup and tier settings; join-request and new-team approval as Desk rows
  (+2.5 days). "No user act needs the Portal" is replaced by named exits.
- **Forced exception 4:** Panacea's step-up gate refuses API-token callers on mapped actions; day
  0 asks that they stay unmapped. Row 109 notes it.
- **K2 and K3 restated** for the forced list (section 3.4). The sign-in tab is weekly, not once
  (the 7-day leaver cap).
- **Row 104's canary** covers every provider with a custom endpoint, not only AU-marked ones.
- **Row 105:** codes optional and the cost centre the team's, as in Panacea, carried as
  annotations, not labels. "Required" and a per-project cost centre go to the owner (section
  5.3), and that Panacea ask leaves day 0.
- **Row 101's list** is 09 §5.5's plus "PanaceaApp" and "Spec Kit". `CONTEXT.md` gains the
  billing Phase exception and points its lens questions to call 9.
- Day 0 item 1's management-group caveat; section 6's range and named gaps; our CLI is not named
  `panacea`.
- **The register:** an "as relayed" tag on each answer after call 5 (since replaced by
  *(question form, 2026-10-01)*, section 10); `OD-44`'s consequence (K3
  and K4, ruling U-15 now permanent) and its third option in the gate; `OD-46`'s user trigger;
  `OD-48`'s veto, sized, with `OD-35`'s retirement conditional on it; `OD-51`'s amendment
  re-pointed from `OD-14` to the Hub ADR's membership rule; the "answered, not gated" list marked
  provisional. `CLAUDE.md` follows.

**Not applied, with the reason:**
- **"K4 unchanged" in Panacea's realm.** K4's offline-session switch (which K1's 7-day cap needs)
  and `hub.org_admin` serve the desktop client and the Hub, which now live in our realm. Asked of
  Panacea, the switch would only cap its other clients' offline sessions at 60 days (ruling
  U-10). Only K4's user-profile items are asked.
- **The Windows bridge as the planned path for Panacea's pages.** On its own it still shows
  Panacea's Keycloak page in the frame without a Panacea change, and it is from knowledge, not
  verified. It stays a day-1 spike that can bring Connect deploy into the frame.

## 10. The owner's second round (2026-10-01)

The owner answered sections 5.2 and 5.3 himself, later the same day, through the in-session
question form. His words, verbatim:

| Question | The owner's answer | Applied |
|---|---|---|
| Connect deploy and Panacea's P-3 (forced exception 3) | *"Huh, maybe you have the wrong idea. I want Panacea to be ingrained into this. I do not want Panacea to be this external site that is used for Deployments. I want it to be first classed, inside the app, so everything is done within the app. No API key, nothing. We will build our own Panacea as a part of this."* | `OD-53`. Forced exceptions 3 and 4 superseded (section 5.2), being scoped in `10-panacea-in-app.md` |
| The Azure DevOps personal access token, or IT's consent (forced exception 2) | *"Instead, could we build in an authenticator, because people would authenticate through EntraID, could we have them SSO into Github or ADO, and that automatically connect?"* | Forced exception 2 superseded, being scoped in `10-panacea-in-app.md`; not yet an `OD` |
| Tasks also in ADO and GitHub? (section 5.3 item 4) | Chose *"Also in ADO/GitHub (Recommended)"* | No veto on `OD-48`: its retirement of `OD-35` for Phase W stands unconditionally |
| The weekly zero-click sign-in tab (forced exception 1) | Chose *"Accept weekly tab (Recommended)"* | Accepted; the day-1 Windows-native sign-in spike stays (section 3.2) and may remove the tab |

**The orchestrator's rulings, which the owner may override.** Section 5.3 item 2, cost codes:
Panacea's model, the reading of *"like we have in Panacea"*: job code and phase optional, the cost
centre from the team; row 105 is unchanged. Section 5.3 item 3, the register: option (c), one
register with a `note` marker on the row; its "answered, not gated" list is final in that form.

**A source corrected.** Calls 6 to 20 are the owner's own answers, given through the in-session
question form earlier the same day, and this file, the register and `CONTEXT.md` now say so; rows
104 to 109 wait on nothing. Call 20 (`OD-50`) was first left on its old tag here;
`10-panacea-in-app.md`'s verify round (§7) corrected it.

**What this round does not change yet.** Every figure in sections 0 to 3 is the plan before
`OD-53`. What it does to the Deploy rows (57 to 73, 107 and 109), to day 0 item 3's Panacea asks,
to K2 and K3 and to the totals is pending `10-panacea-in-app.md` and the owner's sign-off on its
cost. The register records `OD-53` and this round in its last section.
