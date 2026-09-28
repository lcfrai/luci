# Panacea App

An enterprise platform that takes a person from **nothing to a shipped application**, and owns the whole journey: **Define → Design → Develop → Deploy**. Each user gets isolated cloud machines that run coding agents in parallel, each session in its own sandbox on an isolated worktree, streaming back to a single cockpit.

It starts as an app that builds apps. Treat it as a platform that may host more phases later — do not hard-code the assumption that there are exactly four.

**Read `CONTEXT.md` before using a project noun.** It fixes what *machine · worktree · lane · brief · seam · lens · stream · Orchestrator* and the rest mean here, each cited to the file that binds it, plus the words we do not use.

## Why this exists

cube.computer proved the agent-fleet cockpit. Nobody has done it for regulated enterprise, on the customer's own cloud, with the journey in front of it. Our wedge is **usability and iteration on powerful tools** — not raw capability. If a feature is powerful but takes a wall of configuration to reach, it has failed our bar.

## Product spine

The four phases are states of one project, not four tools. The connective tissue is **provenance**: the brief written in Define is visibly referenced in Design, carried into Develop, and attached to the release in Deploy. Same artifact, one thread, shown throughout. Any feature that breaks that thread is wrong regardless of how good it is in isolation.

## Architecture decisions already locked

**Owner decisions of 2026-09-23 (`OD-1`…`OD-13`) are in `docs/superpowers/planning/2026-09-23-owner-decisions.md` and win over anything older here.**

- **Isolation: Kata Containers (AKS Pod Sandboxing) microVMs from day one.** Per-pod microVM + own kernel, `workload_runtime=KataVmIsolation`, AzureLinux, nested-virt SKU. Warm pools are mandatory — Kata cold start is seconds. Not Microsoft Dev Box (retiring 2026–2028).
- **Hosting: we-host SaaS on our own environment** (`OD-5`), modelled on Panacea and written from scratch in this monorepo; customer-hosted Azure Kubernetes App tier later. Panacea stays a separate system for now — phase 7 calls its API — and is lifted into this repo after everything else is built (`OD-13`). *Was: "on Panacea's AKS first".*
- **Shared state from day one** (`OD-3`): briefs, approvals, the provenance ledger and releases live server-side in **the Hub** (Phase H). The desktop signs in **through a Keycloak realm brokering Entra, auth-code + PKCE, zero clicks** (`OD-30`), on Panacea's existing Entra registration, so no IT ticket for sign-in; tokens in Windows Credential Manager (`OD-6`). Ruling: `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`. *Was: "directly against Entra".*
- **AI inference: org-level API through a LiteLLM-style gateway, zero data retention, no personal subscriptions** (`OD-19`). Two paths: **Global** = the direct Anthropic API (the default), and **AU** = Amazon Bedrock in Sydney, switched per project (data classification) or by the user. Sandboxes get a scoped per-session gateway token, never an org key, and usage is attributed per user, team and project. Users still authenticate as themselves (Entra, through the Keycloak broker, `OD-30`). *Was: "per-user rotation across that user's OWN Claude subscriptions"; no consumer subscription is used at all now.*
- **Cloud: Azure** (`OD-18`, chosen as the fastest path for everything else). Bedrock is reached across clouds through the gateway.
- **Team mode** (`OD-14`–`OD-17`): team agents run centrally as managed sessions; per-user Orchestrators are the only cross-user channel, negotiating and escalating to inform or to ask. **Local mode stays for now** (`OD-20`); the product is cloud-first, the target is 90% of people never running locally, and the cloud tracks the latest Claude Code CLI quickly.
- **Design canvas (Design phase): a clean-room, paper.design-class canvas with a first-class MCP server** (`OD-11`) — real-HTML frames, agent editing over MCP, multiplayer — on MIT/Apache/BSD/ISC parts only. **Never open, read or copy `Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas source.** No tldraw (commercial licence). *Was: "adopt `Projects/Paperdesign`" (AGPL-3.0), withdrawn by the owner.*
- **Worktrees are local clones** (`OD-2`), one per worktree, work fetched back under `refs/panacea/lanes/…`. **Git hosts: Azure DevOps and GitHub, both** (`OD-4`).
- **Harness records**: a harness's on-disk transcript may be read, read-only, through a version-pinned adapter with a canary test; nothing may require it, and nothing derives meaning from PTY bytes (`OD-8`, `PROGRAM.md` cross-cutting constraints).

## Reuse from the Panacea monorepo

`Projects/Panacea` is the model for how we run the web and deploy side, and phase 7's API target. Since `OD-5` the product runs on **its own** environment, not on Panacea's. Reuse Panacea's patterns rather than reinventing them, but **copy no Panacea source into this repo yet** (`OD-13`: the owner lifts Panacea in after everything else is built). The patterns to follow:

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
