# 08 - Whole products that already do most of it

**Status:** IN PROGRESS (skeleton written first; sections fill in one at a time).
**Date:** 2026-09-30. **Cluster:** 08-whole-product. **Feeds:** the assemble plan, alongside 00-06.
**Question:** is there an end-to-end product that already covers most of Define -> Design ->
Develop -> Deploy with parallel cloud agents for an enterprise, which we could build on
(white-label, extend, embed) or buy instead of building PanaceaApp's phases, and what would still
be ours (the provenance thread, the cockpit UX, KPMG compliance)?

**Direction this was written under (2026-09-30, owner):** "focus on using third party solutions,
and solving the main problem statement and building the tool. I need this tool ASAP, not in
weeks." Second direction, same day: Orca (`stablyai/orca`, MIT) is the cockpit pick; Panacea is the
strong default for platform services and the best fully-fledged solution wins ("Panacea is fine to
use majority of, if we want to use Coolify plus Supabase, and enhance with snippets from Panacea,
that is fine as well. As long as we get the best solution that is fully fledged."). OD-13 relaxed:
Panacea snippets may be copied with attribution in the file header.

**Method:** vendor pages and docs (WebSearch, WebFetch), the GitHub API for stars, licences and
releases, the GLASSES catalogue (untrusted discovery data), and the sibling clusters 00-06 in this
folder. Nothing installed or run. Every claim that could not be checked is marked unverified.

---

## 1. Answer first

_(pending)_

## 2. The bar a whole product has to clear

A "whole product" here means one vendor's product that already runs **parallel coding agents in
the cloud for a team** and covers at least two of Define, Design, Develop and Deploy end to end.
Libraries and single parts are the other clusters' business (01-06). Each candidate is scored on
the ten tests below; B1-B7 are hard rules from the owner decisions, B8-B10 are the owner's
direction of today.

| # | Test | Source | What fails it |
|---|---|---|---|
| B1 | **Runs on Azure, in our own environment first, in a customer's own Azure later** | `OD-5`, `OD-18` | SaaS with no self-hosted or bring-your-own-cloud mode; a self-hosted mode on AWS or GCP only |
| B2 | **An AU data-residency option** for code, transcripts and inference | `OD-19`, `OD-24` | A vendor region list without Australia; inference that leaves the region |
| B3 | **Entra sign-in through our Keycloak broker** (OIDC or SAML to Keycloak), zero clicks | `OD-30` | Sign-in only through a git host or the vendor's own accounts |
| B4 | **Inference through our LiteLLM gateway, zero data retention**, attributed per user, team and project; no personal subscriptions | `OD-19`, `OD-21`, `OD-33` | The vendor's own model hosting, or a seat that bundles inference we cannot route |
| B5 | **Unmodified agent CLIs** (Claude Code, Codex, Gemini CLI), each session in its own sandbox on its own clone; nothing reads PTY bytes for meaning | `OD-2`, `OD-8`, `CLAUDE.md` | The vendor's own agent only, or a third-party CLI wrapped behind the vendor's chat |
| B6 | **Untrusted-code isolation**: microVM class (Kata on AKS in our plan) | `CLAUDE.md` "Isolation" | Shared-kernel containers on a shared host |
| B7 | **Both Azure DevOps and GitHub** for repos and pull requests | `OD-4` | GitHub only, or GitHub, GitLab and Bitbucket without Azure Repos |
| B8 | **Licence**: permissive (MIT, Apache-2.0, BSD, ISC, MPL-2.0) for anything we deploy into a customer tenancy, or a commercial product KPMG can buy | task brief; `OD-32` | AGPL, GPL, SSPL, BUSL, fair-code, Elastic, PolyForm and other source-available terms, unless bought |
| B9 | **Fits beside Orca**, the owner's cockpit pick | second direction | A product whose value is its own cockpit, so adopting it means a second cockpit |
| B10 | **Days, not weeks** to a first real use by a KPMG team | first direction | A procurement, a platform migration or a git-host move before first use |

**What no vendor will cover, whatever it is.** The provenance thread (the brief written in Define,
referenced in Design, carried into Develop, attached to the release), "done means proven"
(`OD-27`), KPMG's compliance evidence per release, and the cockpit's journey shell. Every
candidate below is judged for what it removes from our plan, never for those four.

**How a candidate is counted as covering a phase.** *Define*: turns an idea into a written,
reviewable spec or brief that later steps read. *Design*: a canvas or UI prototyping step whose
output the build uses. *Develop*: parallel agents in cloud sandboxes, each on its own branch.
*Deploy*: a running URL per branch, promote, rollback, on the customer's infrastructure.

## 3. The candidates at a glance

"Fails" lists section 2's tests; "Days" is integration effort where a sibling or this run costed
it. Detail and evidence are in section 4.

| § | Candidate | Phases it covers | Licence | Deploys | Fails | Days | Verdict |
|---|---|---|---|---|---|---|---|
| 4.1 | OpenHands Agent Canvas + Enterprise | Develop, automations | MIT; server PolyForm Free Trial (buy) | Self-host on Kubernetes | B5, B7, B8 (server), B9 | 10-15 | reference |
| 4.2 | Devin | Develop | Commercial | Cognition cloud; Devbox in a Cognition-run VPC | B1, B2, B4, B5, B9 | — | reject |
| 4.3 | GitHub Copilot cloud agent, Agent HQ (Spark, Workspace retired) | Develop, landing | Commercial | GitHub SaaS; runners may be ours | B2, B4, B7, B9 | — | reject (landing already adopted, 02) |
| 4.4 | Cursor cloud agents | Develop | Commercial | Cursor cloud; workers ours | B2, B4, B5, B7, B9 | — | reject |
| 4.5 | Claude Code cloud sessions, self-hosted environments, Managed Agents | Develop, landing (GitHub) | Commercial | Anthropic control plane; runners ours | B2, B3, B4, B7, B9 | — | reject now |
| 4.6 | Replit, Lovable, Bolt, v0 | Idea to hosted app | Commercial (bolt.diy MIT plus a WebContainer licence) | Vendor clouds | B1, B4, B5, B7, B9 | — | reject |
| 4.7 | Factory | Develop | Commercial | Droid ours; sessions in Factory's Global or EU cloud | B2, B5, B9 | — | reject |
| 4.8 | AWS Kiro | Define to Develop | Commercial | AWS SaaS: US, EU, GovCloud | B1, B2, B4, B5, B9 | — | reject |
| 4.9 | Google Jules, Firebase Studio | Develop; prototype to deploy | Commercial | Google Cloud; Firebase Studio shuts 2027-03-22 | B1, B2, B4, B5, B7, B9 | — | reject |
| 4.10 | Coder | Develop environments | AGPL-3.0 + Premium | Self-host on AKS | B8 unless bought, B9 | 8-12 (02) | reference |
| 4.11 | Ona, OpenAI Codex | Develop | Commercial | AWS or GCP runners; Codex US only | B1, B2, B4, B5, B9 | — | reject |
| 4.12 | GitLab Self-Managed + Duo Agent Platform | Plan to deploy | Commercial EE (CE MIT) | Self-host on Azure | B7 (a third host), B4 as shipped, B9 | weeks | reference |
| 4.14 | **Panacea** (Forge not adopted) | Deploy, app backend, sign-in, teams | KPMG's own | KPMG's Azure, Australia | Forge: B5, B6 | 6 + 2 | **adopt as the platform** |
| 4.15 | Coolify + Supabase + Panacea snippets | Deploy, app backend | Apache-2.0 | Docker on Azure VMs | B7; weak regulated half | 16-24 | reject |
| 4.16 | **Orca** | The Develop cockpit | MIT | Desktop, `orca serve` in our sandboxes | No SSO (glue covers it) | 7.5 on top of 01's 8 | **adopt** |

**The pattern.** Every commercial agent product that offers "your infrastructure" moves only the
*execution* there: Devin's Devbox, Cursor's workers, Anthropic's runners, GitHub's runners,
Factory's Droid. The planning loop, the inference, the session store or the control plane stays
in the vendor's cloud, which is exactly what B2 and B4 forbid. The products that do self-host
whole (OpenHands Enterprise, Coder, GitLab) replace either the agent, the cockpit or the git host.

## 4. Candidates in detail

Tests B1-B10 are section 2's. "Phases" names what the product covers end to end. Facts are dated
2026-09-30 unless marked; sibling clusters are cited where they already verified a fact.

### 4.1 OpenHands: Agent Canvas (MIT) and OpenHands Enterprise (source-available) - REFERENCE

- **Does.** Agent Canvas is "the self-hosted developer control center for coding agents and
  automations": it runs "OpenHands, Claude Code, Codex, Gemini, or any ACP-compatible agent across
  local, remote, and cloud backends", one Docker sandbox per conversation if asked, plus
  automations on a schedule or on Slack, GitHub and Linear events (`README.md`, read today).
  OpenHands Enterprise is the multi-user server behind it, self-hosted "in your own Kubernetes
  cluster".
- **Licence.** Agent Canvas: **MIT** (`OpenHands/OpenHands`, API). The multi-user server is not:
  `OpenHands/enterprise` and `OpenHands/OpenHands-Cloud` (the Helm charts) are **PolyForm Free
  Trial 1.0.0**, "NOT an open source license. Usage is limited to 30 days per calendar year without
  a commercial license". So the enterprise half is a **buy**, price by quote.
- **Deploys.** Helm or Replicated into any Kubernetes; the chart bundles Keycloak, **LiteLLM (or
  "your own LiteLLM instance")**, Postgres, Redis, ClickHouse and a `runtime-api` that gives each
  sandbox a PVC with snapshots. Examples are EKS and GKE; AKS is not documented.
- **Enterprise fit.** Sign-in through the bundled Keycloak with GitHub, GitLab or Bitbucket, or
  "Enterprise SSO (SAML)", so Entra works through SAML. RBAC; audit is claimed, not read.
  Residency is wherever we run it. **No Azure DevOps** in the chart's providers.
- **Maturity.** 89,544 stars, v1.24.0 on 2026-09-25, pushed today; All Hands AI. Agent Canvas
  carries a **beta** badge.
- **Phases.** Develop, plus automations that reach into Define (issue decomposition). No Design,
  no Deploy.
- **Fails.** B5 (Claude Code runs behind an ACP adapter in OpenHands' chat, not as the unmodified
  CLI in a live terminal), B7 (no Azure Repos), B8 for the server (PolyForm), B9 (a second
  cockpit beside Orca). Passes B1, B2 and B4 because it self-hosts and takes our LiteLLM.
- **Days.** 10-15 as a backend under our cockpit (02 §3b); 5-8 to stand the Enterprise chart up
  on AKS, plus procurement.
- **Verdict: reference.** The closest self-hostable Develop product, and the only one that takes
  our gateway as is. Not adopted because the owner's cockpit is Orca and our sessions are the
  unmodified CLI. Read its EventStream and automation design for team mode (as the 2026-09-28
  study already said).

### 4.2 Devin (Cognition) - REJECT

- **Does.** An autonomous engineer with its own shell, browser and editor; Slack, Jira and Linear
  triggers; pull requests.
- **Licence and deploy.** Commercial. "Enterprise Cloud" (multi-tenant) or "Customer Dedicated
  Deployment" (Cognition-hosted, single-tenant VPC reached by AWS PrivateLink or IPsec). In every
  model **"The Brain ... always residing in Cognition's Cloud"**; only the Devbox moves. The docs
  say "Cognition's Tenant is Hosted on Azure"; no region list, no Australia (docs.devin.ai).
- **Enterprise fit.** SAML, OIDC, Okta and Entra SSO. "Devin is a compound AI system and does not
  currently support third-party LLM API keys".
- **Fails.** B1, B2, B4 (no gateway, no key of ours), B5 (its own agent only), B9.
- **Verdict: reject.** Inference and planning never leave Cognition.

### 4.3 GitHub: Copilot cloud agent, Agent HQ, Spark, Copilot Workspace - REJECT as the whole

- **Workspace** ended: the technical preview was sunset on 2025-05-30 and folded into the coding
  agent. **Spark** is gone: no new users or apps from 2026-08-04, shut down 2026-08-31, and GitHub
  Models, which its `llm()` used, retired 2026-07-30 (GitHub docs and changelog).
- **Copilot cloud agent** (the coding agent, renamed) takes an issue and opens a pull request from
  GitHub Actions. **Self-hosted runners through Actions Runner Controller are supported** since
  2025-10-28 (Docker-in-Docker mode, outbound to `api.githubcopilot.com`), with organisation runner
  controls since 2026-04-03, so execution could sit on our AKS. Agent HQ adds Claude and Codex as
  assignees (05 §2).
- **Enterprise fit.** Entra through EMU or OIDC, audit, GHE.com with **Australian residency for
  code at rest** (GA 2025-02-04). **Copilot's in-region inference is US and EU only**
  (2026-04-13); Japan and Australia are "on the roadmap for later in 2026" (02 §3b found Australia
  still unlisted today). Models are GitHub's; bring-your-own-key exists for the CLI and VS Code,
  not found for the cloud agent.
- **Fails.** B2 today, B4 (inference is GitHub's, not our gateway), B7 (GitHub repos only; Azure
  Boards hands work to GitHub's agent), B9 (its own mission control).
- **Verdict: reject as a whole product.** Its landing half (merge queue, auto-merge) is already
  adopted by 02 §4. Re-check when Copilot lists Australia.

### 4.4 Cursor cloud agents, self-hosted workers - REJECT

- **Does.** Cursor's background agents, now "cloud agents", run tasks in parallel from the IDE,
  web and Slack. **Self-hosted cloud agents are GA since 2026-03-25**; team worker pools and eight
  more backends (Lambda, Cloudflare, Coder, Daytona, E2B, Modal, Namespace, Vercel) followed on
  2026-09-02, and Kubernetes starts "from the anysphere/k8s-workers template".
- **The split.** "Cursor runs the agent loop, inference, and planning"; the worker in our network
  runs "file edits, terminal commands", tools and local MCP servers, and reaches out to
  `api2.cursor.sh` (cursor.com/docs/cloud-agent/self-hosted). Team pools need the Enterprise plan.
- **Enterprise fit.** SAML SSO with Entra, SCIM, privacy mode ("not used for training"). No region
  statement on that page; no Azure DevOps named.
- **Fails.** B2, B4 (inference is Cursor's), B5 (Cursor's agent, not the unmodified CLIs), B7, B9.
- **Verdict: reject.** The same shape as Devin's: execution moves to us, the brain does not.

### 4.5 Anthropic: Claude Code cloud sessions, self-hosted environments, Managed Agents - REJECT now

- **Does.** Claude Code on the web runs cloud sessions in parallel ("each `--cloud` command
  creates its own cloud session"), with diff review, PR creation, auto-fix of a PR's CI failures
  and review comments, and teleport back to the terminal. **Self-hosted environments** (public
  beta, Team and Enterprise, off by default) move execution onto runners in our network: a runner
  polls `api.anthropic.com`, claims a session, clones, spawns a Claude Code child, locks to one
  owner, and the session requeues after about 60 s without a poll.
- **The limits, verbatim from the docs** (code.claude.com, read today):
  - "Zero Data Retention: unavailable for organizations with Zero Data Retention enabled", and ZDR
    organisations "can't use `/web-setup` or other cloud session features".
  - "inference can't be routed through Amazon Bedrock, Google Cloud's Agent Platform, Microsoft
    Foundry, or an LLM gateway in self-hosted environments".
  - "repository cloning and pull request creation require GitHub"; other hosts can be uploaded as a
    bundle "but the session can't push results back".
  - "The control plane remains Anthropic-hosted", and "Anthropic stores the session transcript".
- **Managed Agents** (beta since 2026-04): Anthropic's own harness with managed or self-hosted
  sandboxes, US$0.08 a session-hour plus tokens, "Not currently eligible for Zero Data Retention"
  (02 §3b).
- **Fails.** B2 (no Bedrock Sydney path), B3 (claude.ai accounts, not our realm), B4 (no gateway,
  no ZDR), B7 (GitHub only), B9 (claude.ai is its own cockpit).
- **Verdict: reject now; reference the runner.** It is the same unmodified CLI we already run, so
  what it sells is hosting, and hosting is exactly what `OD-5` and `OD-19` keep ours. Its runner
  contract (claim, lease, owner lock, requeue) is a good check on our control plane's. Re-read
  when self-hosted environments accept ZDR and a gateway.

### 4.6 The prompt-to-app builders: Replit, Lovable, Bolt, v0 - REJECT as the base

These are the products that most literally take "a person from nothing to a shipped application",
so they are the nearest thing to our whole journey. They are also all vendor-hosted.

| Product | Journey it covers | Deploy and region | Enterprise fit | Licence | Fails |
|---|---|---|---|---|---|
| **Replit** (Agent 3) | Prompt, plan, build, database, auth, deploy, all in one hosted IDE | Replit's cloud; Enterprise lists "VPC peering", "single-tenant environments", "region selection" (review sites; regions not published). The Microsoft deal (2025-07-08) deploys to "Microsoft Azure's reliable infrastructure" today and calls "direct deployment to customers' own Azure environments" **planned**; an Azure Marketplace listing exists | SAML SSO, SOC 2 Type II | Commercial | B1 (not in our Azure yet), B2 (unpublished), B4, B5, B7, B9 |
| **Lovable** | Prompt to full-stack app on Lovable Cloud (a Supabase backend), GitHub sync | Lovable's cloud; regions "Americas, Europe, and Asia Pacific", country regions on request for Enterprise; the region cannot change after enabling | OIDC and SAML with Entra, SCIM, audit logs kept 13 weeks, ZDR models by default (docs.lovable.dev) | Commercial; annual quote | B1, B4 (its models), B5, B7 (GitHub, GitLab, Bitbucket), B9 |
| **Bolt** (bolt.new; bolt.diy) | Prompt to running app in the browser, deploy | bolt.new is StackBlitz's SaaS. **bolt.diy is MIT**, but it runs on the WebContainer API, which "requires licensing for production usage in a commercial, for-profit setting" | Not verified | MIT code plus a commercial runtime licence | B4, B5, B6 (runs in the user's browser tab), B8 without a WebContainer licence, B9 |
| **v0** (Vercel) | Prompt to Next.js app, one-click deploy to Vercel, a v0 Platform API | Vercel's cloud (a Sydney function region exists); Vercel's bring-your-own-cloud is AWS only, private beta (July 2026 report) | SAML SSO, RBAC, audit, "data residency controls" (vendor summary, unverified); Team plan US$30 per user per month includes the API | Commercial | B1, B4, B5, B7, B9 |

**Reading.** They win on usability for a first app, which is our wedge too. But every one of them
is its own IDE, runs its own agent on its own models, and hosts the result on its own cloud.
Building PanaceaApp on one would mean replacing Orca, the gateway, the sandboxes and Panacea at
once, for a product none of whose code or data sits in our Azure. 04 §3.13 rejected the same
four as a canvas.

**Verdict: reject as the base; reference for the first-run experience** (a prompt box that ends
at a live URL). Our version of that moment is Spec Kit's interview (03) handing to an Orca lane
that deploys through Panacea.

### 4.7 Factory (Droids) - REJECT, the most gateway-friendly SaaS

- **Does.** Factory's own agent, Droid, in the CLI, IDE, Slack, CI and a web app.
- **Deploys.** Three patterns (docs.factory.com): cloud-managed ("Factory doesn't broker model
  access"), hybrid ("Droid operates entirely within your infrastructure"), and fully airgapped
  ("Factory cloud is not reachable at runtime"). "LLM traffic goes through your gateways", so it
  would take our LiteLLM. Factory's own cloud holds session content, Global or EU; identity is
  WorkOS SSO. No Azure-specific or Australian option was found.
- **Maturity.** US$150M Series C at US$1.5B, April 2026 (press).
- **Fails.** B2 (session content in Factory's Global or EU cloud), B5 (Droid only, not the
  unmodified CLIs), B9.
- **Verdict: reject.** It passes B4 where Devin and Cursor fail, but it replaces the agent, which
  is the one thing we do not replace.

### 4.8 AWS Kiro (IDE, CLI, autonomous agent) - REJECT as the whole; its spec format is referenced

- **Does.** Spec-driven development: requirements, design and tasks files (EARS notation), then
  agents that build to them, in an IDE, a CLI and an autonomous agent. The nearest product to our
  Define-to-Develop thread. 03 §4.5 already reviewed it for Define.
- **Deploys.** AWS SaaS, US$0-200 per user per month (03). Kiro "stores data and performs
  inference in the region where your Kiro profile is created": **US East (N. Virginia), Europe
  (Frankfurt) and GovCloud only**. IAM Identity Center may sit in Sydney, the data may not
  (kiro.dev/docs/enterprise/supported-regions). Entra works as an external IdP.
- **Fails.** B1 (AWS only, no self-host), B2, B4 (its models), B5, B9.
- **Verdict: reject.** Its spec layout is the reference 03 already takes through Spec Kit.

### 4.9 Google: Jules, Firebase Studio - REJECT

- **Jules** is Google's asynchronous coding agent: a task runs "in a secure Google Cloud VM" and
  opens a pull request; GA at Google I/O on 2026-05-19, GitHub plus GitLab in beta (June 2026), an
  Enterprise plan with dedicated VMs (press and review sites). Its own agent on Gemini; no
  self-host; no Azure DevOps.
- **Firebase Studio** is closing: sunset announced 2026-03-19, new workspaces and sign-ups off
  from 2026-06-22, shut down 2027-03-22, with users sent to Google AI Studio and Antigravity
  (firebase.google.com migration page).
- **Fails.** B1, B2, B4, B5, B7, B9; Firebase Studio also fails "still exists".
- **Verdict: reject both.**

### 4.10 Coder (workspaces, AI Gateway, Agent Firewall) - REFERENCE; a buy question for phase 8, not here

- **Does.** Self-hosted cloud workspaces on Kubernetes from Terraform templates; Claude Code
  installs into a template; its own agent ("Coder Agents"); the **AI Governance add-on**
  (Premium) with **AI Gateway** (was AI Bridge: central auth, audit and LLM access) and **Agent
  Firewall** (was Agent Boundaries: runtime network and process limits), GA in v2.30 on
  2026-02-03 (coder.com changelog). **Coder Tasks** went to a 12-month extended-support release
  on 2026-06-02 and is announced to leave new releases from v2.37.
- **Licence.** **AGPL-3.0** core (API) plus commercial Premium. Unmodified self-hosting is fine;
  a modified copy served to users must publish its source; shipping it into a customer tenancy
  needs Coder's commercial terms (02 §3b).
- **Deploys.** Helm on AKS, in our region. OIDC with Entra or Keycloak, audit logs, HA.
- **Maturity.** 16,761 stars, v2.36.6 on 2026-09-18, Coder Technologies.
- **Phases.** Develop environments only. No Define, Design, landing or Deploy.
- **Fails.** B8 unless bought; B9 (its dashboard is a second cockpit); its AI Gateway duplicates
  `OD-21`'s LiteLLM.
- **Verdict: reference.** The best-governed environment product on the list, but an environment,
  not a whole product; 01 §4.4 rejected it for sandboxes against AKS Pod Sandboxing.

### 4.11 Ona (formerly Gitpod) and OpenAI Codex - REJECT

- **Ona** ran agents in sandboxed environments with enterprise runners in the customer's AWS or
  GCP. **OpenAI announced it is acquiring Ona on 2026-06-11**, to give Codex self-hosted
  sandboxes (SiliconANGLE, The Next Web; not closed at announcement; no primary source read).
- **Codex cloud and the Agents API** are OpenAI's harness. Reports say the Agents API beta keeps
  data in the **United States only** and does not support ZDR, even with a self-hosted sandbox
  (secondary sources).
- **Fails.** B1 (no Azure runners confirmed), B2, B4, B5 (Codex only), B7 unverified, B9.
- **Verdict: reject.** A model vendor's harness, now owning the environment vendor.

### 4.12 GitLab Self-Managed with Duo Agent Platform (added by this run) - REFERENCE

Not on the brief's list; added because it is the one **self-hostable whole-SDLC product** that
already runs the unmodified Claude Code.

- **Does.** Plan (issues, epics), code, merge requests, CI/CD, environments and review apps,
  security scanning, audit. The Duo Agent Platform went **GA on 2026-01-15 (18.8) on GitLab.com,
  Self-Managed and Dedicated**: a Planner agent, flows such as "Issue to Merge Request", an AI
  Catalog, and **external agents "Claude Code from Anthropic and Codex CLI from OpenAI"**
  (about.gitlab.com). The Claude Code example installs `@anthropic-ai/claude-code` from npm in a
  CI job (`node:22-slim`), so it is the unmodified CLI, headless in a runner.
- **Model path.** The example points Claude Code at
  `https://cloud.gitlab.com/ai/v1/proxy/anthropic` with a GitLab-issued token, that is GitLab's
  own AI gateway. Self-Managed "supports self-hosted models"; whether an external agent can be
  pointed at our LiteLLM instead was not verified.
- **Licence and price.** Premium or Ultimate (commercial EE; the CE core is MIT). Credits-based
  AI: US$12 a user a month of GitLab Credits with Premium, US$24 with Ultimate.
- **Fails.** B7 in the way that matters: it is a **third git host and tracker** beside the Azure
  DevOps and GitHub that `OD-4` names and 05 keeps; B4 as shipped (GitLab's proxy); B9 (a live
  terminal is not its model, so Orca still sits beside it). Passes B1, B2, B3, B6 (runners on our
  AKS) and B8 as a buy.
- **Days.** Weeks, not days: a GitLab estate, repository migration or mirroring, runners on AKS,
  SSO, then the agent flows.
- **Verdict: reference.** The right answer for a customer who already runs GitLab, and a later
  host adapter for us (owner call 4, section 9). Not our base: moving KPMG's repositories and
  boards is the opposite of days.

### 4.13 Also considered, one line each

| Candidate | Why it is not the whole product | Verdict |
|---|---|---|
| **Microsoft Power Apps (Vibe, code apps)** | Vibe went to public preview in April 2026; code apps are GA and the app-builder skill "can be used with ... GitHub Copilot CLI or Claude Code"; runs in the customer's own Power Platform tenant with Entra, DLP and **an Australian region** (Microsoft, press). Apps are Power Platform apps on Dataverse, not the customer's git repo on Panacea | **reference**: a possible later deploy target for internal apps, not the journey |
| **Warp Oz** | Orchestrator stays in Warp's cloud; docs blocked by the KPMG proxy (02 §3b) | reject |
| **AQ** (aq.dev) | Data plane in your cloud, control plane at aq.dev; GitHub only (02 §3b) | reference |
| **cube.computer** | The product we set out to beat; vendor cloud, no SSO (02 §3b) | reference |
| **Northflank, Qovery** | BYOC PaaS that runs in our Azure, with previews and sandboxes, but the control plane is theirs (01 §4.5, 06) | reference |

### 4.14 Panacea, the owner's platform, judged as a whole product - ADOPT as the platform (not as the build engine)

Read-only, `Projects/Panacea` at `efc637879` (2026-09-30); 00-panacea holds the inventory.

- **What it already is.** A regulated deploy platform on KPMG's Azure (`australiacentral` today,
  an Australia East rebuild in cutover, 00 §10): per-branch previews with SSO, a branched database
  per preview, a stage ladder that turns the data class into required scans, rollback, a
  per-release evidence export, 18 scan checks behind a posture gate, per-app Postgres, storage,
  secrets and AI key, the Keycloak realm brokering Entra, a team API, and `panacea-mcp` with 31
  tools an unmodified agent can call (00 §3-8, 06 §0, 05 §0.1).
- **It also has its own prompt-to-app product: Forge** ("Agent Builder", `services/forge-orchestrator/`):
  a chat and preview split view in the Portal, its own tool loop on Kelios or Anthropic, verify,
  authorisation-first and browser-test gates, templates, fork, and "Ship It" to **Azure DevOps,
  Gitea or GitHub** (`ship/providers.go:14`, `:40-56`) then Panacea's pipeline. On B1-B4 and B7
  it is the best-fitting whole product in this study.
- **Why Forge is still not the base.** It is a closed agent loop, not unmodified Claude Code (B5);
  one chat per build, not a parallel fleet; its preview pods are `runc` with egress closed to the
  gateway and git host (B6, 00 §9.3); and `/api/v1/forge/*` sits behind a platform-admin-only,
  no-API-key gate, so "A product like ours cannot drive it" (00 §9.2). Its gates and spec rules
  are copied instead (00 §9.2, 03 §0).
- **Phases.** Deploy, whole; the shipped app's backend, whole; sign-in and teams. Not Define
  (beyond Forge's one interview), Design, the cockpit or the sandboxes.
- **Licence.** KPMG's own code; its stack carries BUSL Vault and AGPL MinIO, Grafana, Loki and
  Tempo, which matters only if Panacea itself is shipped into a customer tenancy (00 §7).
- **Days.** 6 for deploy (06 §0) plus 2 of platform glue (00 §11).
- **Verdict: adopt as the platform.** The best complete answer after a lane becomes a pull
  request (02 §R5, 06 §0). **Trade-off:** our shipped apps share Panacea's estate and its
  incidents; in exchange we build almost none of the regulated half.

### 4.15 Coolify plus Supabase, enhanced with Panacea snippets - REJECT as the platform

- **Does.** Coolify (Apache-2.0) deploys containers with previews to Docker hosts; Supabase
  (Apache-2.0) gives each app a database, auth, storage and realtime. Both permissive, both
  active (00 §10: 62.4k and 110.9k stars).
- **Against our need.** Coolify has no AKS target, OIDC only in a release candidate, and no Azure
  DevOps (B7); self-hosted Supabase is one project per stack, with no branching, managed PITR or
  management API; Supabase Branching is hosted only (06 §3.9-3.10, 00 §10).
- **Days.** 16-24 of ported Panacea code (scans, posture, stage ladder, evidence, SSO previews,
  ADO previews) to approach what Panacea gives on day one (06 §3.9).
- **Verdict: reject as the platform.** Supabase stays a reference for the Hub's store (00 §10 C4).
  **Trade-off:** permissive and portable to a customer's cloud, but weeks to match Panacea.

### 4.16 Orca, the owner's cockpit pick - ADOPT (verified by 02, not re-argued)

- **Does.** "The ADE for working with a fleet of parallel agents": any agent CLI, a worktree per
  agent, live terminals, a status store with an inbox, hosted review, recipes that put a
  workspace on a cloud machine, and a plugin API (02 §R2).
- **Licence and maturity.** MIT (Lovecast Inc.); 80,872 stars in GLASSES, 81,599 in 02's first
  run; `package.json` 1.4.214 at the 2026-09-29 commit 02 read; releases about daily. (The GitHub
  API rate-limited this run before it could re-check.)
- **Enterprise fit.** No SSO or RBAC (team auth is an open issue); telemetry to PostHog US unless
  `ORCA_TELEMETRY_DISABLED=1`; Defender for Endpoint needs a suppression rule and an ASR
  allowlist on managed Windows (02 §R2).
- **Days.** 7.5, 4.5 of them to first ship, on top of 01's 8 for the sandbox (02 §R6).
- **Verdict: adopt.** It is the cockpit half of the whole product; everything above either sits
  under it (sandboxes, gateway, Panacea) or beside it (Spec Kit, Penpot, the tracker).

## 5. Which can run in a customer's own Azure tenancy

_(pending)_

## 6. Revised for the owner's second direction

_(pending)_

## 7. The recommendation

_(pending)_

## 8. What we still build (glue)

_(pending)_

## 9. Owner calls

_(pending)_

## 10. What this run could not verify

_(pending)_

## 11. Sources

_(pending)_
