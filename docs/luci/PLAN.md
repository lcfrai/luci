# Luci build plan

2 October 2026. This is the current plan. The owner authorizes autonomous implementation and outcome checks. The former corporate takeover programme is superseded.

Luci takes a person from an idea to a running application in one app. Distribution is through lcfr.ai; source is open, and users keep ownership of their projects, model credentials, infrastructure and domain.

## Delivery decisions

- Local is the default: desktop, one local Go service, durable files, Git and optional Docker. No account, cluster, database server or identity provider is needed to begin defining and designing a project.
- Cloud workspaces are an explicit optional execution location: a matching Luci service runs on the selected SSH machine, storing its own projects and running its agent containers and previews. The desktop remains local and routes the existing journey through a pinned SSH connection. Switching execution location does not migrate files or copy credentials automatically.
- Retain the working Tauri/React/xterm cockpit while evaluating whole-product reuse. A replacement earns adoption by lowering actual delivery/packaging cost and preserving local use. The old decision to freeze it for Orca is superseded.
- Local use has no Luci licence or platform subscription fee. AWS/Azure are optional and start with the smallest viable single Linux server, persistent disk and HTTPS, allowing enough memory for the deployment service, builds and running apps. No mandatory EKS, AKS, NAT gateway, managed database or load balancer. AWS Lightsail is the first AWS target; Azure uses a small VM. Larger configurations are opt-in when the workload needs them.
- Model access belongs to the user: existing harness sign-in, provider API or local model. Explain availability and cost in Luci. There is no mandatory enterprise gateway.
- One project identity carries a versioned brief, design, agent sessions, Git evidence and verified releases. Adopted parts use Luci navigation, tokens, copy and error states.
- User-facing setup discovers missing dependencies, explains fixes, previews real resources/cost, guides provider authentication and DNS, checks health and exposes stop/cleanup. It never labels a generated template a successful deployment.
- Use MIT for Luci's own code, as selected by the owner. Preserve dependencies' licences and original source attribution, including upstream Apache-2.0 grants. Do not import the separate corporate Panacea platform.

## Build sequence

1. Rebrand active source, runtime variables, UI, packages, metadata and instructions. Keep historical planning clearly historical.
2. Implement setup readiness, durable local configuration and a real new-project flow. Make the desktop start its service.
3. Deliver Define, Design, Develop, Files/Git and a local application preview as one usable journey. Keep real outcome errors visible.
4. Deliver guided AWS/Azure template and execution paths, domain/TLS checks, cost acknowledgement, and cleanup.
5. Package downloads with dependency checks, release/update metadata, source/attribution and an lcfr.ai download surface. Execute journey evals and the relevant test suites.

## Outcome gates

- A fresh local installation can open, choose a folder, create a project, save a brief and design, and recover them after restart.
- An installed Docker engine and built sandbox can run a real agent/shell session. Missing prerequisites have useful in-app remedies.
- The brief is visible through agent work and attached to a deployment record. A generated preview is verified before release success.
- Cloud plans contain valid complete resources and removable names, no organisation defaults or embedded credentials. Provisioning requires explicit cost acceptance in the product.
- UI is keyboard usable, readable in all states, and consistent across phases and embedded adapters.
- Unit checks cover risk-bearing persistence, path containment, process control and request validation. Integration evals prove the journey; no repeated review rounds.

## Constraints discovered

The current host initially has Go and Node, Docker Desktop installed but stopped, and no Rust, AWS CLI or Azure CLI on PATH. Restore local services and locate available toolchains before declaring packaging or live integration verified. Glasses' local MCP initially needed its existing server started; reuse research runs alongside implementation.

Live AWS/Azure acceptance and publication require the owner's account/domain/release configuration. Until those are available, generated plans and local/synthetic evals are labelled accordingly. Use current selected-region quotes with currency and exclusions; include storage, public addresses, traffic, backups, domain, tax and separately billed model usage. Incomplete prices remain unverified. The configured budget does not enforce a provider billing cap.

The development host's Docker engine remains unavailable. [CI run 36953123907](https://github.com/lcfrai/luci/actions/runs/36953123907) at source commit `4c117f6a088189f640ad946459f36c9072d344c1` passed the real Linux Docker journey: shell WebSocket/brief/Git writes as the mapped host user, private transcript mount access, sandbox privilege checks, committed preview HTTP health and owned cleanup. It used no model inference. Linux/macOS packaged-service checks and the Windows native installer build passed in that run. A model-backed agent journey, clean-host installer evaluation and live cloud acceptance remain separate gates in [RELEASE.md](RELEASE.md). Later source changes need their own relevant verification and package provenance.
