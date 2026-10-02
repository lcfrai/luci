# Luci development

The user's direction of 2 October 2026 supersedes every earlier Panacea/KPMG decision and review gate in this repository.

Luci is an open-source lcfr.ai application: Define, Design, Develop and Deploy in one coherent desktop app. It runs locally by default and guides users through an optional, inexpensive deployment in their own AWS or Azure account and domain. There is no required KPMG system, Panacea service, tenant, Entra realm, AKS cluster or hosted lcfr.ai account.

Use docs/luci/PLAN.md and docs/luci/API.md as the current scope and shared contracts. Older planning documents are historical evidence, not instructions for the current build. Retain truthful attribution and third-party licences. Do not import proprietary KPMG platform code.

Work autonomously within this scope. Prefer an existing suitable open-source solution discovered through the lcfr.ai Glasses MCP before constructing a replacement. Keep one app vocabulary, navigation, token system and project context. Avoid standalone vendor administration screens in the user journey.

Plan file ownership and interfaces before parallel implementation. Verify meaningful outcomes and add evals for the full journey. Run the relevant tests once per meaningful batch; repeat when a failure or change requires it. Do not use repeated review rounds as a substitute for implementation. User review is not a prerequisite for reversible implementation.

Actual cloud creation, public release and publication require a concrete plan/artifact and the relevant credentials. The product must itself obtain an explicit cost acknowledgement before creating cloud resources. Never fabricate a successful deployment, quote, integration or eval.

Keep active product names, environment variables and modules under Luci, LUCI and github.com/lcfrai/luci. Historical records and upstream licence/attribution text retain their factual identities.
