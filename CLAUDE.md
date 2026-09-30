# Panacea (repository: PanaceaApp)

The product is called **Panacea**: the evolved version of Panacea (`OD-43`). In the planning docs, "Panacea" alone, "Panacea's API" and `Projects/Panacea` still mean the running platform we deploy through; users never see that distinction. The icon is deferred.

An enterprise platform that takes a person from **nothing to a shipped application**, and owns the whole journey: **Define → Design → Develop → Deploy**. Each user gets isolated cloud machines that run coding agents in parallel, each session in its own sandbox on an isolated worktree, streaming back to a single cockpit.

It starts as an app that builds apps. Treat it as a platform that may host more phases later — do not hard-code the assumption that there are exactly four.

**Read `CONTEXT.md` before using a project noun.** It fixes what *machine · worktree · lane · brief · seam · lens · stream · Orchestrator* and the rest mean here, each cited to the file that binds it, plus the words we do not use.

## Why this exists

cube.computer proved the agent-fleet cockpit. Nobody has done it for regulated enterprise, on the customer's own cloud, with the journey in front of it. Our wedge is **usability and iteration on powerful tools** — not raw capability. If a feature is powerful but takes a wall of configuration to reach, it has failed our bar.

## Product spine

The four phases are states of one project, not four tools. The connective tissue is **provenance**: the brief written in Define is visibly referenced in Design, carried into Develop, and attached to the release in Deploy. Same artifact, one thread, shown throughout. Any feature that breaks that thread is wrong regardless of how good it is in isolation.

**One app, never outside** (`OD-41`, `OD-49`). Owner: *"I want as much of it to be in app and feel like it is all in app, never does it feel seperate, or needing to go outside to do anything."* Every adopted product is headless behind our screens or embedded and themed in our frame; a step outside the app is allowed only where it is technically forced, and is then named for the owner (`2026-09-30-assemble/00-plan.md` §5.2).

## Architecture decisions already locked

**Owner decisions (`OD-1`…`OD-52`) are in `docs/superpowers/planning/2026-09-23-owner-decisions.md` and win over anything older here.** Since 2026-09-30 the product is **assembled** from third-party parts plus our glue and spine (`OD-37`); the plan of record is `docs/superpowers/planning/2026-09-30-assemble/00-plan.md`.

- **Isolation: Kata Containers (AKS Pod Sandboxing) microVMs from day one.** Per-pod microVM + own kernel, `workload_runtime=KataVmIsolation`, AzureLinux, nested-virt SKU. Warm pools are mandatory — Kata cold start is seconds. Not Microsoft Dev Box (retiring 2026–2028). If day 1 has no Kata nodes or claim p95 is over about 5 s, the KPMG pilot alone may fall back to ACI with standby pools, saying so in the isolation statement (`OD-52`).
- **Cockpit: Orca** (`stablyai/orca`, MIT), shipped as our own build named Panacea (`OD-38`, `OD-43`). Phase 2's Tauri desktop is frozen as the fallback.
- **Hosting: we-host SaaS on our own environment** (`OD-5`): **its own resource group and its own AKS cluster inside Panacea's existing Azure subscription** (`OD-42`), not on Panacea's AKS and not a new subscription. Shipped apps and preview databases deploy through Panacea into Panacea's subscription and resource group, as today (`OD-47`). Modelled on Panacea and built in this monorepo; customer-hosted Azure Kubernetes App tier later. Panacea stays a separate system for now — phase 7 calls its API — and is lifted into this repo after everything else is built (`OD-13`). *Was: "on Panacea's AKS first", then "a new subscription".*
- **Shared state from day one** (`OD-3`): briefs, approvals, the provenance ledger and releases live server-side in **the Hub** (Phase H). The desktop signs in **through our own small Keycloak, whose only identity provider is Panacea's realm, which brokers Entra: auth-code + PKCE, zero clicks** (`OD-30`, `OD-44`). Panacea stays the identity source; nothing new is registered in Entra, so no IT ticket; the realm hand-off that gives Penpot one sign-in installs in our Keycloak. Tokens in Windows Credential Manager (`OD-6`). Ruling: `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`. **Every project carries Cost Centre, Job Code and Phase from creation**, on Panacea's pattern (`OD-47`). *Was: "directly against Entra", then "our realm brokering Entra".*
- **AI inference: org-level API through a LiteLLM gateway over any provider, no personal subscriptions** (`OD-19`, amended by `OD-46`). The owner adds endpoints and keys and moves routes between providers **on an admin page in the app**, never LiteLLM's own UI, with no code change: Anthropic direct, Amazon Bedrock in Sydney, KPMG Workbench and any other LiteLLM supports. Data retention and residency are per-provider claims, shown, not assumed; a project's data class still routes (AU class to an AU-resident provider). Sandboxes get a scoped per-session gateway token, never an org key, and usage is attributed per user, team and project. Users still authenticate as themselves (Entra, through the Keycloak broker, `OD-30`). *Was: "per-user rotation across that user's OWN Claude subscriptions", then two fixed paths (Anthropic Global, Bedrock AU).*
- **Cloud: Azure** (`OD-18`, chosen as the fastest path for everything else). Bedrock is reached across clouds through the gateway.
- **Team mode** (`OD-14`–`OD-17`): team agents run centrally as managed sessions; per-user Orchestrators are the only cross-user channel, negotiating and escalating to inform or to ask. Team v1 runs on the tracker first and the negotiation comes after; our teams are Panacea's teams (`OD-51`). **Work** is created, assigned, worked and closed in the app; ADO Boards and GitHub Issues are headless storage and sync behind it (`OD-48`). **Local mode stays for now** (`OD-20`); the product is cloud-first, the target is 90% of people never running locally, and the cloud tracks the latest Claude Code CLI quickly.
- **Design canvas (Design phase): Penpot 2.18 (MPL-2.0) and its MCP server, for v1** (`OD-45`, relaxing `OD-11`) — vector frames with CSS flex and grid, HTML through its export, agent editing over MCP, embedded and themed in our frame. OpenPencil is revisited at its 1.0. **Never open, read or copy `Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas source.** No tldraw (commercial licence). *Was: "adopt `Projects/Paperdesign`" (AGPL-3.0), withdrawn by the owner; then a clean-room canvas (`OD-11`, `OD-34`), withdrawn for v1.*
- **Worktrees are local clones** (`OD-2`), one per worktree, work fetched back under `refs/panacea/lanes/…`. **Git hosts: Azure DevOps and GitHub, both** (`OD-4`).
- **Harness records**: a harness's on-disk transcript may be read, read-only, through a version-pinned adapter with a canary test; nothing may require it, and nothing derives meaning from PTY bytes (`OD-8`, `PROGRAM.md` cross-cutting constraints).

## Reuse from the Panacea monorepo

`Projects/Panacea` is the model for how we run the web and deploy side, and the Deploy step's API. Since `OD-5` the product runs on **its own** environment (`OD-42`), not on Panacea's. Reuse Panacea rather than reinventing it: **Panacea source may be copied in, from snippets up to whole services and Terraform stacks** (`OD-40`, `OD-50`), each copied file carrying an attribution header and an entry in the snippet ledger, which CI checks. The later lift of all of Panacea into this repo still stands (`OD-13`). The places to start:

- Sandbox pod template: `services/forge-orchestrator/preview/manager.go`
- Deploy/branch/version control plane: `services/panacea-api/routes/routes.go` and `services/panacea-mcp`
- Tenant namespace isolation: `clients/tenant_ensure.go`
- SSO chain: oauth2-proxy → Keycloak (realm `panacea`) → Entra ID
- Client BYO-Entra blueprint: `apothecary/docs/02-tenancy-and-identity.md` (design only, not built)

**Gotcha:** two Terraform trees exist and they disagree. `deploy/main.tf` is the live cluster. The `terraform/` tree is aspirational and applying it against a live cluster can destroy it.

## Frontend rules

Read `docs/DESIGN-SYSTEM.md` before writing any UI. It exists because the first prototype failed by looking machine-generated. The non-negotiables:

- No indigo/violet gradients. No gradient-hero + 3-card-grid composition. No blanket glassmorphism.
- Elevation is a 1px ring plus layered shadow, never a flat border. Radius is a scale tied to element size, not one value everywhere.
- Motion must explain causality or be deleted. `transform`/`opacity` only; never `transition: all`.
- Many concurrent live processes share **one global heartbeat clock**. Never a spinner per row.
- Live numbers use `tabular-nums`. Log panes throttle to ~10Hz and never scroll-jump when the user has scrolled up.

## Working agreements

- The product owner reviews prototypes before implementation plans, and plans before code.
- State what is unverified. A screenshot is not evidence that something works.
- Corporate network blocks cube.computer and headless browser screenshots. Visual acceptance is the owner's eyes, not a self-report.
