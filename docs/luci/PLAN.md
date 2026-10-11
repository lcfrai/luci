# Luci build plan

## Owner's current scope and first priority — 11 October 2026 UTC

The latest clarification makes context quality, external-tool interoperability and delivery the primary product: Discover → Design → approved/versioned context → Codex/Claude Code/other tool → returned source/evidence → review/preview/deploy → amend. It supersedes any earlier replacement-harness framing. Implement [the context/delivery acceptance plan](CONTEXT-DELIVERY-SCOPE.md) first. Existing session/swarm execution remains an optional supported path, not a prerequisite for an external handoff or the basis for a new coding engine.

**P1 LOCAL-025:** expose the existing canonical approved-context capture as a standalone immutable export with file hashes, task/requirement identities and source binding; retain and reconcile returned reports against the exact captured scope and observed Git state. External claims never grant approval, execute commands or mark acceptance passed. Follow this with the local tester's real signed-in Codex trial and reviewed changes/checks/preview. Preserve user AGENTS.md/CLAUDE.md and all existing context, authority and recovery boundaries.

New runtime features and additional generic harness acceptance work are subordinate to this useful product journey. Existing safety/recovery defects are retained, not declared fixed or silently dropped. Four-project responsiveness, Windows/native and live-provider gates still apply. Cloud-only setup does not block the local external-tool trial.


3 October 2026 (Australia/Brisbane). This is the current plan. The owner authorizes autonomous implementation and outcome checks. The former corporate takeover programme is superseded.

Luci takes a person from an idea to a running application in one app. Distribution is through lcfr.ai; source is open, and users keep ownership of their projects, model credentials, infrastructure and domain.

## Delivery decisions

The owner names the concurrent activities **Discover, Design, Develop and Deploy** (7 October clarification, relayed 11 October 2026). These replace the earlier Define/Describe labels in current product guidance; existing identifiers and recorded evidence remain unchanged.

- Local is the default: desktop or packaged browser UI, one managed Go control plane, durable files, Git and Docker execution. The first owner creates a local account. No external identity provider, cluster or database server is required. The same product runs against a user-owned AWS/Azure backing.
- The application is the end user's workspace. The backing owns management: account roles, private per-user runtime state, provider credentials, resource policy, execution and audit. Member, Admin and Super Admin are explicit roles. A member cannot modify backing configuration or run arbitrary host commands. Super Admin host execution is an optional, explicit trusted-operation policy, disabled by default.
- Cloud workspaces are an explicit optional execution location: a matching Luci service runs on the selected SSH machine, storing its own projects and running its agent containers and previews. The desktop remains local and routes the existing journey through a pinned SSH connection. Switching execution location does not migrate files or copy credentials automatically.
- Retain the working Tauri/React/xterm cockpit while evaluating whole-product reuse. A replacement earns adoption by lowering actual delivery/packaging cost and preserving local use. The old decision to freeze it for Orca is superseded.
- Local use has no Luci licence or platform subscription fee. AWS/Azure are optional and start with the smallest viable single Linux server, persistent disk and HTTPS, allowing enough memory for the deployment service, builds and running apps. No mandatory EKS, AKS, NAT gateway, managed database or load balancer. AWS Lightsail is the first AWS target; Azure uses a small VM. Larger configurations are opt-in when the workload needs them.
- Model access belongs to the user: existing harness sign-in, provider API or local model. Explain availability and cost in Luci. There is no mandatory enterprise gateway.
- One project identity carries a versioned brief, design, agent sessions, Git evidence and verified releases. Adopted parts use Luci navigation, tokens, copy and error states.
- User-facing setup discovers missing dependencies, explains fixes, previews real resources/cost, guides provider authentication and DNS, checks health and exposes stop/cleanup. It never labels a generated template a successful deployment.
- Use MIT for Luci's own code, as selected by the owner. Preserve dependencies' licences and original source attribution. Deliver application-platform capabilities through Luci's own implementation and appropriately licensed integrations. The current open-source direction and AGENTS.md supersede the earlier private Panacea integration instruction; do not import restricted proprietary source or inherit a corporate cloud estate or identity default.

## Build sequence

The installed-app inspection on 3 October supersedes the former narrow completion bar. [FULL-PRODUCT.md](FULL-PRODUCT.md) defines the active implementation and complete acceptance requirements: proper onboarding, automatic preparation, Projects home, appearance, rich Discover, meaningful Design, operational Develop and an application/data deployment platform. The application is not yet a full UAT handback.

The owner's latest polish requirement is an explicit completion gate in
[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md) and [GOAL.md](GOAL.md): a coherent,
dependable idea-to-production experience at the quality expected of leading
AI-era applications. Continue implementing and validating complete user journeys;
launch success and passing technical tests do not establish that outcome.

1. Rebrand active source, runtime variables, UI, packages, metadata and instructions. Keep historical planning clearly historical.
2. Implement setup readiness, durable local configuration and a real new-project flow. Make the desktop start its service.
3. Deliver Discover, Design, Develop, Files/Git and a local application preview as one usable journey. Keep real outcome errors visible.
4. Deliver guided AWS/Azure template and execution paths, domain/TLS checks, cost acknowledgement, and cleanup.
5. Connect the whole solution workflow: versioned task plan and design context, Claude/Codex/local-model development, independently recorded command checks, GitHub creation/push, exact commit preview/deployment, project backup/restore and release maintenance.
6. Add control-plane authentication, isolated user runtimes, role/policy enforcement, supervision and audit. Connect cloud backing authentication through verified SSH without copying local credentials.
7. Package downloads with dependency checks, release/update metadata, source/attribution and an lcfr.ai download surface. Execute complete solution and administration outcome evals, then the relevant test suites.

## Outcome gates

- A fresh local installation can open, choose a folder, create a project, save a brief and design, and recover them after restart.
- An installed Docker engine and built sandbox can run a real agent/shell session. Missing prerequisites have useful in-app remedies.
- The brief is visible through agent work and attached to a deployment record. A generated preview is verified before release success.
- Cloud plans contain valid complete resources and removable names, no organisation defaults or embedded credentials. Provisioning requires explicit cost acceptance in the product.
- UI is keyboard usable, readable in all states, and consistent across phases and embedded adapters.
- Bootstrap creates one Super Admin; login survives the application journey, logout/revocation closes access, a member cannot read another user's project or credentials, and the last Super Admin cannot be disabled accidentally.
- A saved plan/design is available to the development session. Command checks record execution location, source digest, exit code and bounded output. Source changes invalidate a passing result; manually completing a task never fabricates test evidence.
- A created project can be published privately, pushed with exact remote commit verification, backed up and restored into a new project, previewed, deployed and maintained through the same application.
- Unit checks cover risk-bearing persistence, path containment, process control and request validation. Integration evals prove the journey; no repeated review rounds.

## Constraints discovered

The current Windows host has Go, Node, Rust/MSVC and authenticated GitHub. The user has started Docker Desktop; the Linux Docker engine is now available at version 28.5.1. The earlier unavailable-engine condition and automatic approval review's refusal of a repair are historical; no repair was retried. AWS/Azure accounts and domain configuration remain absent. Keep the existing terminal/editor integrations and adopt maintained provider/Git/deployment tools where they reduce bespoke implementation.

The real packaged Luci 0.4.3 UI installed the agent runtime successfully on this host. Disposable container version checks passed against image `sha256:8949aaa1b80e52bcb325feef29866c0cb54e1f2e3b393f3a9bb5975b5db1afe1`: Codex CLI 0.160.0, Node 24.21.0 and Go 1.26.8. The original official auth-only Codex Docker session exited without a saved account connection. Authorization, subscription-backed inference and public deployment are not yet proven. These checks concern the packaged host application with Docker execution, not a fully containerized control plane. The local version receipt is retained at `artifacts/subscription-e2e/runtime-versions-8949aaa1.json`; it contains no authentication material.

Live AWS/Azure acceptance and publication require the owner's account/domain/release configuration. Until those are available, generated plans and local/synthetic evals are labelled accordingly. Use current selected-region quotes with currency and exclusions; include storage, public addresses, traffic, backups, domain, tax and separately billed model usage. Incomplete prices remain unverified. The configured budget does not enforce a provider billing cap.

Historical [CI run 36953123907](https://github.com/lcfrai/luci/actions/runs/36953123907), at source commit `4c117f6a088189f640ad946459f36c9072d344c1`, passed the real Linux Docker journey: shell WebSocket/brief/Git writes as the mapped host user, private transcript mount access, sandbox privilege checks, committed preview HTTP health and owned cleanup. It used no model inference. Linux/macOS packaged-service checks and the Windows native installer build also passed in that run. Those results and later installed-application acceptance are separate from the current live subscription and deployment gates; see [RELEASE.md](RELEASE.md). Later source changes need their own relevant verification and package provenance.
