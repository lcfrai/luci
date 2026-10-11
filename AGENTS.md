# Luci development

## Current product focus — 11 October 2026 UTC / 12 October local

The owner's latest clarification supersedes the earlier ambition to build a replacement coding-agent harness. Luci's differentiator is excellent shared project context and delivery around existing tools: **Discover → Design → approved/versioned context → external Codex, Claude Code or another development tool → returned changes and evidence → review/preview/deploy → amend**. Keep the four activities concurrent and the project scope understandable throughout. Use existing tools' execution capabilities and authentication rather than rebuilding them. Existing managed sessions, coordination and resource/recovery controls remain supported infrastructure; resolve their safety/correctness defects without letting new runtime features displace the context-to-delivery journey. Do not overwrite user-owned AGENTS.md or CLAUDE.md when exporting context. External reports are claims, not approvals or independently verified checks.


The user's direction of 2 October 2026 supersedes every earlier Panacea/KPMG decision and review gate in this repository.

Luci is an open-source lcfr.ai application: Discover, Design, Develop, Git delivery, previews, deployment and maintenance in one coherent desktop app. Its management gateway owns identity, per-user workspaces, Admin/Super Admin policy and audit; Docker is the default execution boundary on the local or selected cloud host. It runs locally by default and guides users through an optional, inexpensive deployment in their own AWS or Azure account and domain. There is no required KPMG system, Panacea service, tenant, Entra realm, AKS cluster or hosted lcfr.ai account.

The owner's 7 October naming clarification, relayed by Windows acceptance on 11 October 2026, names the four concurrent activities Discover, Design, Develop and Deploy. This supersedes earlier Define/Describe activity labels. Activities can be revisited and progressed together; preserve existing approval and execution controls. Stored lens and API identifiers remain unchanged. Historical evidence retains the wording observed at its recorded revision.

Use docs/luci/PLAN.md and docs/luci/API.md as the current scope and shared contracts. Older planning documents are historical evidence, not instructions for the current build. Retain truthful attribution and third-party licences. Do not import proprietary KPMG platform code.

The owner selected MIT for Luci's own code. Keep the root MIT licence and attribution with distributions; preserve the original grants of upstream dependencies and historical material. Do not replace upstream Apache-2.0, font or other licence texts with MIT.

Minimize ongoing cost while preserving working outcomes. Prefer local execution and existing suitable resources; for optional cloud use, choose the smallest viable single host with enough build/runtime headroom. Avoid extra managed services without a concrete workload need. Keep current provider quotes, currency, exclusions and unverified amounts explicit; never invent a low price or claim the user's budget caps billing. Model usage is separate, and stronger models should be used where their task quality justifies their cost.

Work autonomously within this scope. Prefer an existing suitable open-source solution discovered through the lcfr.ai Glasses MCP before constructing a replacement. Keep one app vocabulary, navigation, token system and project context. Avoid standalone vendor administration screens in the user journey.

Completion is governed by `docs/luci/GOAL.md` and `docs/luci/PRODUCT-QUALITY.md`. The owner requires a polished, understandable idea-to-live application journey, including amendments and operation after launch. A clean launch, a test count or deterministic provider fixtures cannot establish full product or real-model quality. Keep those release gates and their evidence explicit while resolving concrete product gaps.

Plan file ownership and interfaces before parallel implementation. Verify meaningful outcomes and add evals for the full journey. Run the relevant tests once per meaningful batch; repeat when a failure or change requires it. Do not use repeated review rounds as a substitute for implementation. User review is not a prerequisite for reversible implementation.

Actual cloud creation, public release and publication require a concrete plan/artifact and the relevant credentials. The product must itself obtain an explicit cost acknowledgement before creating cloud resources. Never fabricate a successful deployment, quote, integration or eval.

Keep active product names, environment variables and modules under Luci, LUCI and github.com/lcfrai/luci. Historical records and upstream licence/attribution text retain their factual identities.
