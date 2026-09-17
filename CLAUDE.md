# Panacea App

An enterprise platform that takes a person from **nothing to a shipped application**, and owns the whole journey: **Define → Design → Develop → Deploy**. Each user gets isolated cloud machines that run coding agents in parallel, one git worktree per agent, streaming back to a single cockpit.

It starts as an app that builds apps. Treat it as a platform that may host more phases later — do not hard-code the assumption that there are exactly four.

## Why this exists

cube.computer proved the agent-fleet cockpit. Nobody has done it for regulated enterprise, on the customer's own cloud, with the journey in front of it. Our wedge is **usability and iteration on powerful tools** — not raw capability. If a feature is powerful but takes a wall of configuration to reach, it has failed our bar.

## Product spine

The four phases are states of one project, not four tools. The connective tissue is **provenance**: the spec written in Define is visibly referenced in Design, carried into Develop, and attached to the release in Deploy. Same artifact, one thread, shown throughout. Any feature that breaks that thread is wrong regardless of how good it is in isolation.

## Architecture decisions already locked

- **Isolation: Kata Containers (AKS Pod Sandboxing) microVMs from day one.** Per-pod microVM + own kernel, `workload_runtime=KataVmIsolation`, AzureLinux, nested-virt SKU. Warm pools are mandatory — Kata cold start is seconds. Not Microsoft Dev Box (retiring 2026–2028).
- **Hosting: we-host SaaS on Panacea's AKS first**, customer-hosted Azure Kubernetes App tier later.
- **AI capacity: per-user rotation across that user's OWN Claude subscriptions**, with API/Bedrock/Vertex as backstop. Users authenticate as themselves. **Never pool subscriptions across users** — that violates Anthropic's Consumer Terms and is blocked server-side.
- **Design canvas (Design phase): adopt `Projects/Paperdesign`** (a fork of `kgoedecke/doop`, an open-source paper.design equivalent: real-HTML frames in sandboxed iframes, built-in MCP for agent edits, multiplayer). AGPL-3.0 — must be cleared before any external sale.

## Reuse from the Panacea monorepo

`Projects/Panacea` is the infrastructure platform this runs on. Reuse, do not rebuild:

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
