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
`2026-09-23-owner-decisions.md`.
**Precedence inside this folder:** `09-cohesion` > `08-whole-product-verify` > each cluster's
`-verify` > each cluster's first report. Every figure below is the latest one in that chain.
**Method:** the 19 files in this folder, re-read where a figure is used; a read-only inventory of
every branch (`git ls-tree`, `git diff` against each merge-base, the plans' task tables). Nothing
was installed or run. Section 6 lists what is not verified.

## 0. The answer

**Assemble. We adopt a dozen products and build the glue and the product spine around them. The
remaining work is about 119 engineer-days (109 to 133). With five builders that is the first app
in about 12 to 15 working days and all of it in about 24; with two, about 5 to 6 weeks and about
12 weeks. The clock starts when the day-0 items clear, and they can decide the date more than the
build does.**

**The three percentages** (method and arithmetic in section 2):

| Measure | Figure |
|---|---|
| **Done** | **About 12%** (10 to 15). About 16.7 engineer-days of kept, built code, against 16.7 + 119 |
| **From open source**, by capability (the headline) | **38%** of 103 capabilities. With Panacea (16%) and managed services (14%), **68%** come from something we adopt |
| **We build ourselves**, by capability (the headline) | **32%** |
| By remaining effort, for the same question | 51% of the 119 days is glue on adopted pieces (open source 33%, Panacea 6%, managed services 12%); **49%** is our own code |

Every one of the 119 days is code we write. "From open source" means the capability arrives
already built and we connect it; it does not mean the days disappear.

1. **What we adopt.** Orca as the cockpit, shipped as our own build of the pinned upstream. AKS Pod
   Sandboxing (Kata) with `kubernetes-sigs/agent-sandbox` for sandboxes. GitHub Spec Kit for
   Define. Penpot 2.18 and its MCP server for Design. Azure DevOps Boards, GitHub Issues and
   Microsoft Teams for work and team. Panacea's API and `panacea-mcp` as the Deploy step. LiteLLM
   for the AI gateway. Keycloak for sign-in. Azure's managed Postgres, Blob and Key Vault for our
   own state.
2. **What we build.** The Hub (briefs, approvals, the provenance ledger, releases), the one-app
   layer (journey bar, app-level view, the Desk, the realm hand-off, the thread drawer, one
   palette, one vocabulary), the lane path (lane git endpoint, landing, the Panacea plugin), the
   gateway's `keysmith` and `gatekeeper`, and every page where the brief is published, approved or
   released.
3. **The acceptance gate** is 09-cohesion §6's journey walk, K1 to K14, ending with the owner's
   eyes: *"Does this feel like one app?"* A no fails the build.
4. **What waits on the owner** (section 5): staffing, the app's name and icon, code signing, and
   the Azure subscription above all.

## 1. The components

One row per capability, deduplicated across the clusters. **Kind** is who delivers the
capability: **OSS** (an open-source product), **Panacea** (Panacea's running service or its copied
code), **Managed** (a commercial or cloud service), **Ours** (code we write that no product
provides). **Glue** is days spent connecting an adopted piece; **Build** is days on a row that is
ours. Days are engineer-days, verify figures. A 0 means the days are in another row or the piece is
used as it is.

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

### 1.1 Sandboxes (01-sandboxes-verify §7): 10 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 1 | Sandbox cluster: AKS in Australia East, Kata pool, ACR, Azure Firewall, feature registration | Platform | AKS Pod Sandboxing, with Panacea's Terraform as a pattern | Azure; Kata Apache-2.0 | Managed | 1 | | 01-verify §7 row 1 |
| 2 | Warm pool, claim, suspend and resume | Platform | `kubernetes-sigs/agent-sandbox` v1.0.4 | Apache-2.0 | OSS | 0.5 | | 01-verify §7 row 2 |
| 3 | `katabe` over `SandboxClaim`s (client-go, RuntimeClass fixed to `kata-vm-isolation`) | Platform | Our backend over agent-sandbox's CRDs | Apache-2.0 | OSS | 1 | | 01-verify §7 row 3, §8 items 1-2 |
| 4 | Sandbox image and late-bind: Orca headless runtime, `panacea-mcp`, `POST /bind` | Develop | Orca runtime plus our `sandboxd` | MIT | OSS | 1.5 | | 01-verify §7 row 4 |
| 5 | Default-deny egress, FQDN allowlist, npm and PyPI mirrors | Platform | Azure Firewall, Verdaccio, proxpi | MIT; Firewall commercial | OSS | 1.5 | | 01-verify §7 row 5 |
| 6 | Worktree into the sandbox (git bundle over the bind channel, lanes back the same way) | Develop | Ours | ours | Ours | | 1.5 | 01-verify §7 row 6 |
| 7 | Broker in the cluster behind Keycloak, per-sandbox `wss://` route | Platform | Ours (phase 8 tasks 17-18) | ours | Ours | | 2 | 01-verify §7 row 7 |
| 8 | Orca environment recipe (create, suspend, resume, destroy) | Develop | Orca `environmentRecipes` | MIT | OSS | 0.5 | | 01-verify §7 row 8 |
| 9 | Smoke: 10 sessions, claim p50/p95, refill, timed clone and build | Platform | AKS Pod Sandboxing plus agent-sandbox | Apache-2.0 | OSS | 0.5 | | 01-verify §7 row 9 |

### 1.2 The cockpit (02-fleet-cockpit-verify §6-7): 15 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 10 | Fleet cockpit engine: parallel sessions, worktrees, terminals, diff, editor | Develop | Orca (`stablyai/orca`), pinned | MIT | OSS | 0 | | 02-verify §0, §8 item 1 |
| 11 | Rollout package: pinned version, Intune, telemetry off, plugin preinstalled | Develop | Orca | MIT | OSS | 0.5 | | 02-verify §6 row 1 |
| 12 | `panacea login`: loopback PKCE through the realm, Credential Manager, refresh | Platform | Ours | ours | Ours | | 1 | 02-verify §6 row 2 |
| 13 | Lane git endpoint: session-token auth, lane-branch-only hook, lease-guarded force | Develop | Ours | ours | Ours | | 2 | 02-verify §6 row 3 |
| 14 | Landing from the laptop: fetch back, push, PR with brief trailer, merge queue or auto-complete | Develop | GitHub and Azure DevOps pull requests | commercial | Managed | 2.5 | | 02-verify §6 row 4, §8 item 4 |
| 15 | Panacea PR preview per lane, with the session and brief stamp (GitHub only in v1) | Deploy | Panacea's webhook-receiver previews | KPMG own | Panacea | 0.5 | | 02-verify §6 row 5 |
| 16 | The Panacea plugin: lane card, commands, recipe, pending-brief step | Develop | Ours, on Orca's plugin API | ours | Ours | | 2.5 | 02-verify §6 row 6 |
| 17 | Resume and fork through the recipe, from phase 2's transcript store | Develop | Ours | ours | Ours | | 1 | 02-verify §6 row 7 |
| 18 | AI first-pass PR review, opt-in, through our gateway | Develop | `anthropics/claude-code-action`; `claude -p` in Azure Pipelines | MIT | OSS | 0.5 | | 02-verify §6 row 8 |
| 19 | Our build's identity: name, app id, icon, KPMG signature, our update feed | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 change row 1 |
| 20 | Build-time rename over the English catalogue | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 row 2 |
| 21 | Our tokens on Orca's shadcn variables (first five mappings) | Cohesion | Orca, our build | MIT | OSS | 0.5 | | 02-verify §7 row 3 |
| 22 | Journey bar, app-level view for Hub pages, `lane` deep link | Cohesion | Ours, inside our Orca build | ours | Ours | | 2 | 02-verify §7 row 4 (its Hub 0.5 is row 75) |
| 23 | OSC status off for hook-capable agents (`OD-8` patch) | Cohesion | Ours, a patch in our build | ours | Ours | | 0.75 | 02-verify §7 row 5, §2.1 |
| 24 | Orca's cloud features off: telemetry, mobile relay, share-as-artifact, computer use | Cohesion | Orca, our build | MIT | OSS | 0.25 | | 02-verify §7 row 6 |

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

### 1.5 Work and team (05-work-team-verify §8): 19.5 days

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
| 53 | Tracker notifications in Teams channels | Cohesion | Azure Boards and GitHub apps for Teams | first-party | Managed | 0 | | 05-verify §6 |
| 54 | Preview comments on the running app, filed as a ticket | Develop | Panacea `preview_comments.go` | KPMG own | Panacea | 1.5 | | 05-verify §8 piece 6 |
| 55 | Azure DevOps Start without a fork (pending start through the recipe) | Develop | Orca, unforked | MIT | OSS | 1.5 | | 05-verify §8 piece 7 |
| 56 | Tracker audit into the SIEM | Platform | Microsoft Sentinel connectors | commercial | Managed | 0 | | 05-work-team §7 |

### 1.6 Deploy (06-deploy-verify §2.5, 00-panacea-verify §7): 11.325 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 57 | Deploy engine: previews per branch, ship, promote, rollback, versions, evidence | Deploy | Panacea API on `dev.panacea.kpmg` | KPMG own | Panacea | 0 | | 06-verify §2.2, §7 item 1 |
| 58 | Shipped-app backend: Postgres branched per preview, storage, secrets, functions, flags | Deploy | Panacea (CloudNativePG, Vault, Knative, Flagsmith, MinIO) | KPMG own; mixed stack | Panacea | 0 | | 00-panacea §7, §11 |
| 59 | Build pipeline, scans and the posture gate | Deploy | Panacea (Tekton, Trivy, gitleaks, OSV, Opengrep) | KPMG own; scanners Apache-2.0/MIT | Panacea | 0 | | 00-verify §2 rows 10-11 |
| 60 | Agent deploy and fix loop from inside the sandbox | Deploy | `panacea-mcp` (31 tools) | KPMG own | Panacea | 0 | | 06-verify §2.2 |
| 61 | G1: our Deploy page and lane card (logs, scans, governance, approve), no Portal | Deploy | Ours, over Panacea's API | ours | Ours | | 2.75 | 06-verify §2.5 G1, §6 |
| 62 | G2: signed brief-to-release provenance (in-toto, DSSE, stored evidence bytes) | Deploy | cosign and sigstore Go libraries, key in Key Vault | Apache-2.0 | OSS | 1.25 | | 06-verify §2.5 G2, §3 |
| 63 | G3: promotion by data class through Panacea's stage ladder | Deploy | Panacea stage ladder | KPMG own | Panacea | 0.75 | | 06-verify §2.5 G3 |
| 64 | G4: agent allowlist proxy in front of `panacea-mcp` (method, route, body rules, SSE) | Deploy | Ours | ours | Ours | | 1.5 | 06-verify §2.5 G4; 00-verify §7 G4 |
| 65 | G5 and P3: create the Panacea project (labels, scan posture, team state, job code) | Deploy | Panacea `POST /api/v1/projects` | KPMG own | Panacea | 0.875 | | 06-verify §2.5 G5; 00-verify §7 P3 |
| 66 | Kept phase 7 connector tasks (07 rest, 08, 09, 10, 19, 20) | Deploy | Our connector behind `deploy.Deployer`, over Panacea | KPMG own | Panacea | 1.7 | | 06-verify §2.5 |
| 67 | Build provenance signing and admission check | Deploy | Tekton Chains, Kyverno (inside Panacea) | Apache-2.0 | OSS | 0 | | 06-verify §3 |
| 68 | P1: `panacea-mcp` built into every sandbox image | Develop | Panacea `panacea-mcp`, copied | KPMG own | Panacea | 0.25 | | 00-verify §7 P1 |
| 69 | P2: agent-context bootstrap skill into the worktree | Develop | Panacea agent-context endpoint | KPMG own | Panacea | 0.25 | | 00-verify §7 P2 |
| 70 | P5: six Panacea-side asks, written and checked | Platform | Panacea's stream; our coordination | KPMG own | Panacea | 0.5 | | 00-verify §7 P5 |
| 71 | P6: live proof on Panacea dev (create, preview, SSO, rollback) | Deploy | Panacea dev estate | KPMG own | Panacea | 0.5 | | 00-verify §7 P6 |
| 72 | P4: snippet ledger and attribution-header CI check | Platform | Ours | ours | Ours | | 0.25 | 00-verify §7 P4 |
| 73 | Panacea's steps inside our cockpit: team-request state, Connect deploy, our error copy | Cohesion | Ours | ours | Ours | | 0.75 | 00-verify §7 "cohesion glue", §8 |

### 1.7 Platform (07-platform-verify §5): 23.2 days

| # | Capability | Phase | Delivered by | Licence | Kind | Glue | Build | Citation |
|---|---|---|---|---|---|---|---|---|
| 74 | The Hub: briefs, approvals, ledger, releases, change feed (routes and client) | Platform | Ours (Go, `cmd/hub`) | ours | Ours | | 10 | 07-verify §5 Hub |
| 75 | Signed-in Hub pages: one-time ticket to an `HttpOnly` cookie | Platform | Ours | ours | Ours | | 1 | 07-verify §5, §6 C1 |
| 76 | AI gateway: Global and AU routes, per-session keys, budgets, refusal rewrite | Platform | LiteLLM from the MIT tree, plus our `keysmith` and `gatekeeper` | MIT | OSS | 8.7 | | 07-verify §5 Gateway |
| 77 | Inference providers | Platform | Anthropic API; Amazon Bedrock Sydney | commercial | Managed | 0 | | 07-platform §7 call 2 |
| 78 | Environment: AKS, Flexible Server, Key Vault, DNS, ingress, OpenTofu from Panacea's `.tf` | Platform | Azure | Azure; OpenTofu MPL-2.0 | Managed | 1 | | 07-verify §5 Environment |
| 79 | Our own state: Hub database, blobs, secrets | Platform | Azure Flexible Server, Blob, Key Vault | commercial | Managed | 0 | | 00-verify §6; 07-verify §2 |
| 80 | Public AU edge WAF for the Hub and gateway | Platform | Azure Application Gateway WAF_v2 | commercial | Managed | 0 | | 07-verify §8 item 4 |
| 81 | Pilot sign-in: Entra brokered through Panacea's realm | Platform | Panacea's dev Keycloak 25 realm | Apache-2.0 | Panacea | 0 | | 07-verify §5 Realm |
| 82 | Our own Keycloak realm, Keycloak 26 on two replicas | Platform | Keycloak, keycloak-config-cli | Apache-2.0 | OSS | 2 | | 07-verify §5; 09 §7 "Pulled into the plan" |
| 83 | Keycloak login and error theme in our design system | Cohesion | Keycloakify | MIT | OSS | 0 | | 07-verify §6 C2 (inside row 82) |
| 84 | One spend line per project (agent spend plus hosted-app spend) | Platform | Ours | ours | Ours | | 0.5 | 07-verify §6 C4 (outside every earlier total) |
| 85 | Code signing for our desktop build | Platform | Azure Artifact Signing, or an OV certificate | commercial | Managed | 0 | | 02-verify §8 call 6 |

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
| 95 | Realm hand-off authenticator: one realm session inside the frame | Cohesion | Ours, on Keycloak's SPI | ours | Ours | | 1.5 | 09 §4.1, §7 row 5 |
| 96 | Share-link previews until the hand-off exists | Cohesion | Panacea's share-links API | KPMG own | Panacea | 0.25 | | 09 §4.2 |
| 97 | Spec Kit preset in our nouns (brief, criteria) | Cohesion | Spec Kit preset | MIT | OSS | 0.25 | | 09 §5.5, §7 row 6 |
| 98 | One type pairing: Geist Sans and Commit Mono | Cohesion | Ours (OFL fonts) | OFL | Ours | | 0.25 | 09 §3, §7 row 7 |
| 99 | One palette across every surface | Cohesion | Orca's Cmd+J palette, through our plugin | MIT | OSS | 1 | | 09 §5.3, §7 row 8 |
| 100 | The thread drawer | Cohesion | Ours | ours | Ours | | 1 | 09 §5.2, §7 row 9 |
| 101 | One vocabulary: banned-word CI scan, copy review, `CONTEXT.md` entries | Cohesion | Ours | ours | Ours | | 0.75 | 09 §5.5, §7 row 10 |
| 102 | Orca theme file, all 125 variables | Cohesion | Orca, themed by our generated CSS | MIT | OSS | 0.5 | | 09 §3.1, §7 row 10 |
| 103 | Terminal theme from our tokens | Cohesion | Orca custom terminal theme | MIT | OSS | 0.25 | | 09 §2 row 5 |

**Outside the total:** the Windows sign-in bridge (1 day, contingent on the hand-off not being
ready, 09 §7); the upstream ADO task-source pull request (2 days, optional, 05-verify §8); the
Tauri Deploy lens (2.3 days, only if IT refuses Orca, 06-verify §2.5); Forge's ship gates as a
pre-deploy check (1 day, later, 00 §12); reviewers' comments into the Desk (about 2 days, after
v1, 09 §7); the image-bump canary pipeline (not costed, 01 §5 item 5).

### 1.9 The sum

| Cluster | Days | Against the chain's figure |
|---|---|---|
| Sandboxes | 10 | 10 (08-verify §6) |
| Cockpit | 15 | 15.5, less the 0.5 Hub hand-off counted in row 75 |
| Define | 10.25 | 10; the rows sum to 10.25 (03-verify §6) |
| Design | 12.75 | 13 rounded (04-verify §2.7) |
| Work and team | 19.5 | 20 rounded (05-verify §8) |
| Deploy and Panacea | 11.325 | 8.5 + 2; 06-verify's pieces sum to 8.7, and 00-verify's unique rows add 2.625 against 08-verify's "2, a floor" |
| Platform | 23.2 | 20.5 + realm 2; 07-verify's final text says 8.7 for the gateway, not 8.4; plus C4's 0.5 |
| One-app layer | 17.25 | 4 + 11.75, plus the hook relay's 1.5, which no total carried |
| **Total** | **119.275, about 119** | 09-cohesion's "about 117" is 117.25 before rounding |

**Range: about 109 to 133.** 08-verify §6 puts its sum at 95 to 114, 09 §7 puts the new work at 10
to 15, and this file's corrections add about 2.

## 2. The percentages

### 2.1 Percent done: about 12%

**Method.** The plan's 119 days are net of what is built: every cluster costed only what is still
to do (the inventory's key point, and 08-verify §6). So built code does not reduce the 119. It is
work already banked on top of it. Done is therefore banked days over banked plus remaining. Banked
days are the plan-hours of kept, built tasks divided by 8, the convention this folder uses. That is
conservative: real effort ran higher, with fix rounds and hundreds of commits per branch.

| Branch | What is built and kept | Banked days |
|---|---|---|
| `feat/phase-2-cockpit` (with phase 1 and the sandbox git fix) | Control plane, sessions API, attach relay, transcript store, resume and fork, `sandboxd`, `dockerbe` | 5 |
| `feat/hub` | Schema, row-level security, hash-chained audit, the client and its fake (tasks 44a, 00, 01-07b) | 3 |
| `feat/phase-4-discover` | Tasks 01-10: the provenance thread, Materialiser, `discoverhub` | 2.75 |
| `feat/phase-7-deploy` | Tasks 01-06 and part of 07: the deploy domain, transport, reads, acts | 2 |
| `feat/env` | Tasks 01-04, 09, 11, 16: OpenTofu layout, network and observability modules, platform chart, realm file | 1.75 |
| `feat/phase-5-canvas` | Tasks 02-04 and 18: the licence gate, the MCP relay | 1.25 |
| `feat/phase-3-git` | Tasks 01-03: `gitx`, the lane index, clones | 0.9 |
| **Total** | | **16.65** |

**Arithmetic.** 16.65 / (16.65 + 119.275) = 16.65 / 135.925 = **12.2%, about 12%.** Valuing the
phase 2 backend at half (2.5) gives 10.6%; at double (10), 15.4%. Hence 10 to 15%.

**Built code being replaced, and not counted as done:**
- Phase 2's Tauri Develop views, about 21k TypeScript lines (phase 2 tasks 14-24). Orca replaces
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

**(i) By capability: 103 rows in section 1.**

| Kind | Rows | Share |
|---|---|---|
| Open source | 39 | **38%** |
| Panacea reuse | 17 | **16%** |
| Managed service | 14 | **14%** |
| **Adopted, together** | **70** | **68%** |
| **Ours** | **33** | **32%** |
| Total | 103 | 100% |

Arithmetic: 39/103 = 37.9%, 17/103 = 16.5%, 14/103 = 13.6%, 33/103 = 32.0%. Rounded by largest
remainder so the four add to 100. Rows per cluster: sandboxes 6 OSS, 1 managed, 2 ours; cockpit 7 OSS,
1 Panacea, 1 managed, 6 ours; Define 4 OSS, 1 Panacea, 4 ours; Design 8 OSS, 1 managed, 2 ours;
work 2 OSS, 2 Panacea, 5 managed, 3 ours; Deploy 2 OSS, 11 Panacea, 4 ours; platform 3 OSS, 1
Panacea, 5 managed, 3 ours; one-app 7 OSS, 1 Panacea, 1 managed, 9 ours.

**Challenge it here:** the count depends on how finely capabilities are cut. Section 1 cuts at the
granularity the cluster files cost at, so a large product (Orca, row 10) is one row, the same as a
half-day rename (row 20). That understates what the adopted pieces deliver.

**(ii) By remaining effort: 119.275 days.**

| Where the days go | Days | Share |
|---|---|---|
| Glue on open-source pieces | 39.2 | **33%** |
| Glue on Panacea | 7.575 | **6%** |
| Glue on managed services | 14.0 | **12%** |
| **Glue on adopted pieces, together** | **60.775** | **51%** |
| **Our own code** | **58.5** | **49%** |
| Total | 119.275 | 100% |

Arithmetic: 39.2/119.275 = 32.9%, 7.575/119.275 = 6.4%, 14.0/119.275 = 11.7%, 58.5/119.275 =
49.0%. Open-source glue by product: LiteLLM 8.7, Penpot 11.5, Orca 8.5 (rows 4, 8, 11, 19-21, 24,
27, 55, 93, 99, 102, 103), Spec Kit 3.25, agent-sandbox with `katabe` and the smoke 2, mirrors 1.5, Keycloak 2,
cosign 1.25, claude-code-action 0.5.

**Challenge it here:** the gateway's 8.7 (row 76) contains our own `keysmith` and `gatekeeper`, and
07-verify does not split it. Moved wholly to ours, open source falls to 26% and ours rises to 56%.
So by effort, **our own code is 49 to 56%.**

**(iii) By size of software delivered: not measured.** The folder gives one size, Orca's 30,735
files at `59b746ff` (02-fleet-cockpit.md:76). It gives none for Penpot, Keycloak, LiteLLM, Spec Kit
or Panacea. Our kept code is about 40k Go source lines and 21k TypeScript lines across the branches
(the inventory). By size, the adopted share is far above either view, but no figure is claimed.

## 3. Build order

### 3.1 Day 0: none of it engineering, and it sets the date

Asked on day 0, by the owner or the build lead, in this order of lead time:

1. **The Azure subscription**, tagged `panacea-app-estate=<env>`, with D-series v5 Kata vCPU quota
   in Australia East, the `AllowBringYourOwnPublicIpAddress` registration, and no FIPS or Trusted
   Launch policy on the Kata pool (01-verify §7 "Day 0", §8 item 9). If it takes a week, the first
   real session is about three weeks out.
2. **KPMG IT and SOC:** an MDE suppression rule and an ASR allowlist for our signed build, and a
   code-signing route (02-verify §6 "Day 0", §8 item 7).
3. **Panacea's side:** our AKS egress IP on `edge_allowed_ip_ranges`, the PKCE and broker clients
   in the realm, Keycloak 26 on two replicas, and P-1 to P-3 queued behind the cutover freeze
   (00-verify §7 P5; 06-verify §7 item 5; 07-verify §5).
4. **IT's request for our own realm:** one redirect URI and a second secret (09 §7 call 4;
   07-verify §8).
5. **The rest:** a DNS name and edge allowlist for the Hub and gateway (07-verify §5); ADO process
   and org admins, the KPMG PAT policy (global PATs stop 2026-12-01), a GitHub App owner, a Teams
   admin, and a member-readable roster route from Panacea (05-verify §8).

### 3.2 Before the build

The owner reviews a clickable prototype of the shell: the bar, the Desk, one Hub page and the
Develop frame (09 §6; 08-verify §8; `CLAUDE.md` working agreements). Day-1 spikes run beside it:
the browser route (0.5), the realm hand-off and the Windows bridge on a managed laptop, the
journey bar on the first Orca pin, and Penpot's headless runner (09 §8; 04-verify §5).

### 3.3 Five builders: what each starts on

| Builder | Weeks 1-2: the first app | Weeks 3-5: the rest |
|---|---|---|
| **B1, sandboxes** (the long pole) | Rows 1-9 in 01-verify's order: cluster, agent-sandbox, `katabe`, image, egress, worktree, broker, recipe, smoke. About 10 days from the subscription. Switch to ACI if day 1 has no Kata nodes or claim p95 is over about 5 s (01-verify §8) | Penpot's headless runners in the Kata pool (row 39), then Design stream B |
| **B2, our Orca build** (its own stream from day 1) | Rows 19-24, 102-103, 93: identity, rename, theme, journey bar and app-level view, OSC off, surfaces off. Then row 99 (the palette) | Design stream A (rows 34, 36-38, 44), then rows 87, 94 |
| **B3, the lane path** | Rows 11-18: rollout, `panacea login`, lane git endpoint, landing, Panacea preview; then the plugin, resume and fork, AI review | Define (rows 25-33), then the tracker Start (rows 48, 55) |
| **B4, the Hub** | Row 74's minimum Hub (about 5 working days on two engineers, 08-verify §6), row 75, row 88, then the Desk (rows 89-91) and the thread drawer (row 100) | The rest of the Hub, then the Work page (rows 47, 50) and Design's approval rows 41-42 |
| **B5, gateway and Deploy** | Row 76's minimal gateway, then Deploy's first-ship glue (about 5 of rows 61-73), the realm (row 82) and the hand-off (row 95) | The rest of Deploy, the tracker connectors (rows 45-46), the Teams bot (rows 52, 92) |

**Critical path.** Journey steps 2, 5 and 7 all wait on the Hub service (08-verify §2 row 3), and
the first real session waits on B1. The Desk and the realm hand-off sit on the first app's path,
which is why 09 §7 adds 2 to 3 working days.

**Calendar.**
- **Five builders:** the first app (journey steps 1-7) in about 12 to 15 working days after day 0
  (08-verify §6's 10-12, plus 09 §7's 2-3). All of it in about 24 working days (08-verify's 20-21
  for 103 days, scaled to 119).
- **Two builders:** the first app in about 5 to 6 weeks (08-verify's "about a month", plus about 9
  engineer-days that 09 put on the first-ship path). All of it in about 12 weeks (119 / 2 = about
  60 working days).

### 3.4 The acceptance gate

09-cohesion §6 is the gate for v1. Walk J1 to J9 on the pilot build from a cold start on a managed
KPMG laptop, and check K1 to K14 at every step. K1 to K13 are automated inside row 86 and run on
every Orca or Penpot pin move. **K14 is the owner's eyes, and a no fails the build whatever the
others say.** Until that walk, every visual claim stays unverified (`OD-12`; `CLAUDE.md`).

Other gates: a security review of the agent sharing a sandbox with `orca serve` before any
customer tenancy (02-verify §8 item 8); a security review of the hand-off authenticator (09 §4.1);
Panacea's P-3 realm trust and legacy-key mint guard before first customers (00-verify §9).

## 4. What happens to each branch

| Branch | Fate | What carries on |
|---|---|---|
| `feat/phase-2-cockpit` (incl. `feat/phase-1-live-session`, `fix/sandbox-git-ca`, merged) | **Kept, in part.** The base of the assembly | Everything under `control-plane/` and `sandboxd/` becomes the lane and sandbox API the recipe calls (02 R4). `sandboxd` shrinks, because `orca serve` owns the PTY for Orca lanes. `katabe` is fixed in row 3. `tokens.css` is the source for every generated theme. The Tauri Develop views are **frozen** as the fallback cockpit |
| `feat/hub` | **Kept.** Resumes first | Tasks 08, 13-27 (routes) and 28-42 (client) are row 74. Tasks 11-12 (`devissuer`) drop for a Keycloak dev container |
| `feat/phase-4-discover` | **Kept.** Tasks 01-10 stay | Spec Kit replaces the unbuilt Define UX (12 of 22 unbuilt tasks, about 71 of about 109 h). Rows 28-33 finish the thread |
| `feat/phase-7-deploy` | **Kept.** | Tasks 07 rest, 08-10, 19-20 are row 66; 11, 12, 17 and 18 come back as row 61. Tasks 13-16 are removed |
| `feat/env` | **Kept, mostly.** | Tasks 05-08 start from Panacea's `.tf` under OpenTofu. The realm file seeds the dev Keycloak. `internal/entra` becomes `internal/oidcauth`. Tasks 17-18 are row 7 |
| `feat/phase-5-canvas` | **Kept in part; the editor is sunk.** | The licence gate (task 04) and the MCP relay (task 18, re-pointed at Penpot, row 38) stay. Tasks 05-06 and `OD-34`'s op-log are replaced by Penpot, pending owner call 6 |
| `feat/phase-3-git` | **Kept.** | Tasks 01-05, 07, 11-15, 23 stay (clones per lane; 12-13 now serve the laptop CLI). Orca replaces 06 and 18-22; the git host replaces 08 and 10 |
| `feat/phase-6-gateway` | **Kept as a plan, unbuilt.** | All 29 tasks are row 76; tasks 04 and 07 start from Panacea's key-minting code |
| `feat/team-mode` | **Paused.** No code of its own | Phase T's tracker, queue and branch-policy tasks go to the trackers, Teams and the git host (05-work-team). The Orchestrator core waits on owner call 16 |
| `feat/work-layer` | **Paused; plan largely replaced.** No code | 127 of 190 Phase W tasks go to ADO Boards, GitHub Issues and Teams, pending owner call 15 |

Each worktree under `.claude/worktrees/` is left as it is. Merges into the assembly happen when
each stream resumes, not in this change.

## 5. Owner calls still open

Only money or customer-visible calls. Each has a recommendation and what it blocks.

**Do now: these block day 0 or the first app.**

| # | Call | Recommendation | Blocks |
|---|---|---|---|
| 1 | **Staffing** (money) | About five builders if ASAP is the bar (08-verify §7.4 call 3) | The calendar: 12-15 working days to the first app with five, 5-6 weeks with two |
| 2 | **The Azure subscription** and about US$2,000 a month dev baseline at list; its lead time (01-verify §8 call 0; 01 §6 call 1) | Ask KPMG today. Co-host on Panacea only if the lead time is long | Every sandbox row, the Hub, the gateway |
| 3 | **The app's name and icon** (customer-visible; 02-verify call 5; 09 §7 call 1) | Decide before the thin build starts | Rows 19-20, the rename list, the CI word scan |
| 4 | **Code signing** (money): Azure Artifact Signing at US$9.99 a month if an Australian entity qualifies, else an OV certificate in Key Vault (02-verify call 6) | Artifact Signing | IT's ASR allowlist, so the first ship |
| 5 | **Our own Keycloak realm for the hand-off** (customer-visible: sign-in moves off Panacea's dev estate; 09 §7 call 4) | Yes, with IT's request filed on day 0 | Row 95, and K2 for Penpot and previews |

**Decide this week: these block a cluster's stream.**

| # | Call | Recommendation | Blocks |
|---|---|---|---|
| 6 | **Penpot as the canvas**, which withdraws `OD-11`'s clean-room build and `OD-34`'s op-log. MPL-2.0 is not on `OD-11`'s licence list (04-design:484; upheld 04-verify §5) | Accept. MPL-2.0 is file-level copyleft, not AGPL | Every Design row. `OD-37` allows adopting a canvas; this call is whether MPL-2.0 is inside `OD-11`'s floor |
| 7 | **Penpot's radius and font**: accept Work Sans and 4/6/8/12, or patch under MPL-2.0 and publish the changed files (09 §3.2, §7 exception 3) | Accept for v1; try the font-alias spike | K9 and K14 only |
| 8 | **The six declared v1 exceptions**: reviewers on the host PR page, the first-run sign-in tab, Penpot's font and radii, Connect deploy until P-3, Keycloak's stock error pages, Spec Kit's file paths (09 §7 call 3) | Accept, each with its closing step | K1-K3 passing with named exceptions |
| 9 | **Define, not Discover; four lenses**, with Files and Merge inside Develop (09 §5.1, call 2) | Define, and four | The journey bar's labels and the `Lens` type |
| 10 | **Type**: Geist Sans and Commit Mono everywhere. The sans reverses `OD-28`'s Segoe UI Variable; the mono is free, or Berkeley Mono at about US$100 once (09 §3, call 5) | Geist Sans and Commit Mono | Row 98 |
| 11 | **`OD-8` and Orca's OSC status** until our patch lands (02-verify §8 call 1) | Accept for the KPMG pilot, display only | The first ship, until row 23 lands |
| 12 | **Land from the laptop**, or hand the git token to the cloud, which changes `OD-6` (02-verify §8 call 3) | Land from the laptop | Row 14 |
| 13 | **Who can open a shipped app**: KPMG network only in v1, behind Panacea's 14-IP allowlist; previews GitHub-only in v1, with ADO PR events asked of Panacea (00 §13 call 1; 06-verify §7 call 2; 02-verify call 4) | Accept for pilots | Any customer-facing preview |
| 14 | **Who pays** for apps and preview databases: the team and job code (00 §13 call 2; 06-verify §7 call 3) | Name one for the pilot | Row 65 |
| 15 | **Work: retire `OD-26`'s own tracker and reverse `OD-35` for Phase W**; ADO Boards as the system of record; PanaceaApp's own ADO organisation in the Australia geography; done-means-proven as two fields and three rules (05 §9 calls; 05-verify §9) | Retire; own AU org | Rows 45-50 |
| 16 | **Team mode v1 on the tracker**, Orchestrator negotiation after, which changes `OD-16`'s order; PanaceaApp teams are Panacea teams (05 §9) | Accept both | `feat/team-mode`'s resume |
| 17 | **Inference accounts**: an Anthropic org with a ZDR agreement (Global) and an AWS account for Bedrock Sydney (AU); budget defaults; no "AU only" claim until the count_tokens canary passes (07-platform §7 calls 2, 3, 5) | Open both accounts | Row 76's first use |
| 18 | **Define**: the agent-led conversation instead of the reviewed stepper; brief files committed into customer repos; Panacea's three data classes in plain words; the brief chip in the panel under unforked Orca (03 §9; 03-verify §7) | Accept all four | Rows 25-33 |

**Later: before the first customer, not before the first app.** Penpot Enterprise quote (from
US$950 a month) and its open-core admin console (04-verify §5 calls A-B); licence treatment of
Semgrep rules, Vault BUSL, MinIO, Grafana and ClamAV before any customer-hosted tier (00 §13 call
4; 06-verify §7 item 6); the customer isolation statement (01-verify §8 item 6); the WAF_v2 edge at
about US$381 a month (07-verify §8); GitHub Enterprise Cloud at US$21 a user a month for queued
landing (02 §7); warm-pool size and idle policy (01 §6); no whole-product seats, GitLab only on
demand, and a quarterly "buy" re-check (08-verify §7.4 calls 1, 5, 6); Octopus Deploy only if a
customer mandates ServiceNow tickets (06-verify §7 call 4).

## 6. Not verified

- **Nothing was installed, built or run** for this plan, and no test was run for the inventory.
  "Working" rests on committed tests and the plans' as-built records.
- **Every day figure is an estimate composed from estimates.** The range is about 109 to 133.
- **The banked days** are plan-hours divided by 8, not measured effort.
- **The capability view depends on row granularity** (section 2.2). A different cut moves it.
- **The gateway's split** between LiteLLM glue and our own services is not stated at source.
- **The sum corrections in 1.9** (00's 2.625, 07's 8.7, the hook relay's 1.5, C4's 0.5) are this
  file's reading of the verify documents. 08-verify and 09 did not have them.
- **The calendar** assumes day 0 is clear and five builders share no bottleneck besides the Hub.
- **Every visual claim** waits on K14, the owner's eyes (`OD-12`). Headless screenshots are
  blocked on the corporate network.
- **Each cluster's own "not verified" section stands:** 00-verify §10, 01-verify §9, 02-verify §9,
  03-verify §8, 04-verify §6, 05-verify §10, 06-verify §8, 07-verify §9, 08-verify §8, 09 §8.

## 7. Sources

- **This folder:** all 19 files, `00-panacea.md` to `09-cohesion.md`, in the sections cited. Figures
  re-read at `01-sandboxes-verify.md` §7, `02-fleet-cockpit-verify.md` §6-8,
  `06-deploy-verify.md` §2.5, `07-platform-verify.md` §5, `00-panacea-verify.md` §7,
  `08-whole-product-verify.md` §6-7, `09-cohesion.md` §0-7.
- **The register:** `docs/superpowers/planning/2026-09-23-owner-decisions.md`, `OD-1` to `OD-41`.
- **The tree:** a read-only inventory on 2026-10-01 of every branch (`git worktree list`,
  `git ls-tree -r`, `git diff --name-status --numstat` against each merge-base with
  `feat/phase-2-cockpit`, `git log`), and the task-hour tables of the phase 3, 4, 7 and 8 plans and
  the Hub plan.
- **Project rules:** `CLAUDE.md`, `CONTEXT.md`, `docs/DESIGN-SYSTEM.md`.
