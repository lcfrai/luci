# Luci production roadmap

6 October 2026. This roadmap implements the owner's confirmed direction:
MIT-licensed self-hosted Luci plus optional paid lcfr.ai-managed team workspaces.
[GOAL.md](GOAL.md) remains the full completion contract. Stages describe work and
exit criteria; they are not claims of completed features or release readiness.

## Product outcome and delivery model

A person or team can define a solution together, maintain useful versioned
context, review its design, give agents the exact approved inputs, inspect real
changes and checks, and deploy and maintain the result through one polished
application. Progress is based on observed behavior and durable evidence, not
agent confidence, a completed task checkbox or a process exit code.

Self-hosted Luci remains useful locally with Docker, personal provider access
and owned source/data. AWS, Azure and existing hosts remain optional execution
locations. The managed service provides operations and a coherent team
experience; it does not make a hosted account mandatory for local use. Keep
context, source and supported backups portable between supported installations.
Luci's code remains MIT with accurate upstream licences and attribution.

For the first paid offering, allocate a dedicated execution boundary to each
team: a dedicated VM/host or another independently verified equivalent with its
own worker state, project storage, secrets and execution resources. Do not sell
shared-host process separation as tenant isolation. A shared management service
may authenticate and route requests only through explicit team and project
authorization; its presence does not grant access to another team's runtime.
Shared-host execution is a separate future architecture decision requiring its
own threat model and isolation evidence.

Prefer existing suitable resources and the smallest viable host with enough
build/runtime headroom. Reuse Luci's gateway, persistence, Docker, Git, context,
check and release foundations before adding services. Inspect suitable
open-source assets through lcfr.ai Glasses, including the existing Fluid
Functionalism UI foundation, and verify their actual licences before adoption.
Add managed infrastructure only for a measured operational requirement.

[Project understanding and reviewed amendments](ORCHESTRATION.md) defines the
current cross-phase contract: explain agreed intent and immutable run scope,
make material ambiguity visible, and apply amendments through explicit review
and revision checks. Optional private discussion has an export and bounded
continuation path; retaining chat is not a stage exit criterion.

## Current baseline

The [6 October handoff](HANDOFF-2026-10-06.md) records the pushed 0.4.18 baseline
and its evidence limitations. Private review authority, reviewed task handoff,
provider routing and preview ownership checks are foundations to retain.
Historical source and installed results remain bound to their original commits
and packages. They do not establish current clean installation or live service
readiness.

The working source is now the **0.5.0 candidate**. It contains the durable
coordinator specified in [PROJECT-RUN-CONTRACT.md](PROJECT-RUN-CONTRACT.md),
private evidence joins, exact source/review/model binding and explicit recovery.
[Context format 2](CONTEXT-STORAGE.md) separates public lineage from local
permissions, preserves bounded archived revisions, supports clone/restore and
makes generated-export repair explicit. Legacy format 1 remains readable;
older writers must not edit format 2.

[Dedicated team HTTPS ingress](PUBLIC-HTTPS-INGRESS.md) now connects the remote
web UI to owned app/data origins through static app-origin and separate preview
site settings. It retains authenticated browser grants and active revocation.
Eleven local TLS Chromium checks cover the browser boundary, and the supplied
operator-certificate Nginx template passes its syntax check. Real DNS,
certificate renewal and public-host acceptance remain separate release gates.

Team membership and project grants, shared routing, separate execute/secret/
deploy permissions, context attribution, scoped recovery drafts and operator
service admission now have source implementations and targeted tests. These
replace the earlier private-account-only limitation. Matching Linux package
acceptance now covers the team worker, two-browser collaboration, actual reviewed
CLI execution and revocation. These results do not prove a managed team's
dedicated host boundary or real merchant payment processing. [Team service operations](TEAM-SERVICE-OPERATIONS.md)
keeps those limits explicit.

Ten portable-context regressions cover actual Git clone/backup/restore, large
scenes, bounded history, stale cleanup/repair, interrupted chunks/staging and hostile
JSON/archives. The combined context, backup, private-review and attribution
suite passed in the cloud workspace. Release tooling independently checks
version alignment and candidate integrity; its five regression tests pass.
An opt-in GitHub/Sigstore integrity-attestation workflow and optional Windows
certificate-store signing path are implemented. Neither has been executed with
production distribution authority in this environment. See [RELEASE.md](RELEASE.md)
and the current [full-product checkpoint](FULL-PRODUCT.md).

The historical handoff's host failures and installed receipts remain tied to
their original machine/source. Clean Linux 0.5.0 package acceptance now includes
personal and team journeys, project understanding/amendments, launcher and
restart recovery, and stopped-state upgrade/rollback. Current evidence and CI
failures are recorded in [FULL-PRODUCT.md](FULL-PRODUCT.md). Other native platforms,
live model/provider, real cloud/domain/email and public/paid acceptance gates
remain open; no whole-stage completion follows from the local results.

## Required coherent project orchestration

The owner's latest clarification prioritizes understanding the current stage,
what is being built (especially during agent execution), and the consequences of
amendments. A persistent chat is optional supporting history, not the primary
experience or an acceptance requirement. Reuse private assistance/provider jobs,
canonical project context and durable runs. Keep phase editors and terminal
details available as focused views of the same solution.

Measurable exits:

- Every phase shows the agreed purpose, current requirements, selected task/run,
  observed stage and one useful next step. Changes of phase, restart and an
  authorized teammate's return preserve this understanding. Account, workspace,
  team and project changes never mix contexts or supporting history.
- The actual selected provider receives the bounded current brief, definition,
  design, task plan, previous conversation and observed execution summary.
  Missing provider access shows a remedy and never invents an assistant result.
- Material uncertainty produces a clear question with its effect on the work.
  New coordinated implementation pauses until an authenticated answer resolves
  it; read, cancel and independent manual terminals remain usable.
- Amendments to definition, design or task plan show current/proposed content,
  rationale and downstream effects. Explicit review applies a draft using the
  exact saved-context and conversation revisions. A concurrent teammate's edit
  conflicts without overwriting either person's work. Affected approvals become
  stale; historical run inputs and outcomes remain immutable.
- One visible next step derives from real saved context and execution evidence,
  leading through existing review, build, checks, preview and publish controls.
  A model suggestion cannot bypass identity, secret, execution, source-binding,
  health, publishing or cost-acknowledgement gates.
- Duplicated requests, interrupted inference and restart reconcile an existing
  durable turn/job without repeating paid work. Read-only history never invokes
  a provider. Private conversation storage and model inputs are bounded, and
  retention/capacity limits are visible rather than silently erasing decisions.
- An actual connected journey demonstrates an initial idea, a meaningful
  clarification, accepted design/task amendments, implementation/check evidence,
  a working preview and the explicit path to a live release. Fixture evidence
  and externally verified live-provider/public-domain outcomes stay separate.

## Stage 1: Trustworthy project execution

Complete the coordinator over existing session/check/release operations. Show a
single useful next action with a compact history; place detailed receipts and
diagnostics behind deliberate disclosures. Preserve independent terminals and
sessions. No read, boot or reconciliation operation may silently launch model
work, retry a command, provision infrastructure or publish an application.

Exit evidence must demonstrate:

- Repeated and concurrent requests allocate one reserved child; changed content
  with a reused key fails. Crashes before and after allocation recover the same
  child or a truthful unknown outcome without duplicate resources.
- Review revocation, changed source, a disabled model route and a foreign
  account/project child prevent continuation without changing the user's
  model, billing account or reviewed requirements.
- Successful CLI exit and task completion without passing independent checks
  cannot complete a run. Checks cover the selected requirements and bind the
  exact candidate source; releases bind that same committed source and retain
  actual ownership/health observations.
- Restart, cancellation, explicit retry and ordinary history pruning preserve
  provenance. A completed historical run can show later drift or health loss
  without rewriting its original evidence.
- The matching installed app exercises the connected path, including a failed
  check and recovery, with readable progress and no unexplained dead controls.

## Stage 2: Portable context and real team collaboration

Separate portable project/context identity from local paths and private
authorization. Clone/import/restore must retain the useful brief, requirements,
design and plan while requiring appropriate fresh private approvals. Bound
drawing/history storage, recover partial exports and make optimistic save
conflicts visible. Bind a session to a server-resolved immutable context
snapshot and deliver that snapshot to its actual supported worktree.

Add explicit teams, membership lifecycle and project permissions. Define
owner/editor/viewer behavior, with independent grants for agent execution,
secret use, deployments and administration. Attribute context changes and
reviews to authenticated actors; show revisions and changed dependencies.
Concurrent contributors must be able to retain their work and reconcile a
conflict. Real-time presence is optional unless it improves the tested workflow;
correct shared state and permission enforcement are mandatory.

Exit evidence must demonstrate:

- A saved project cloned into a different directory or restored on a supported
  installation remains readable and editable; copied public files cannot
  create private review authority or acquire another project's permissions.
- Repeated substantial design saves remain within documented storage bounds;
  interrupted export and a stale writer have recoverable outcomes without
  silently replacing newer context.
- The actual agent receives the selected context bytes/revisions. Unavailable
  or stale worktree context blocks with a remedy. Historical inputs remain
  inspectable after later context edits and restart.
- Two authorized people contribute to one project, review an attributed
  change, resolve a concurrent save, and continue a run using its actual
  reviewed snapshot. The API and rendered experience agree on their roles.
- A nonmember or removed member cannot continue using project routes, preview
  links, subscriptions or execution controls. Existing sessions and resources
  follow an explicit revocation policy; viewer access cannot grant secrets or
  execution. Cross-project and cross-team copies fail without side effects.

Stage 2 is a paid-team gate. Its portable context and collaboration work may
proceed alongside later independent installation and UI work.

## Stage 3: A polished complete installed journey

Finish the connected experience from download and first launch through account
creation/recovery, workspace preparation, project import/create, guided Define,
meaningful Design, agent development, Git, preview, deployment and maintenance.
Design must include testable interactions and an implementation handoff; a saved
sketch alone is insufficient. Supported CLI tools must write in their assigned
project while preserving the execution boundary. Keep provider connections
consistent across phases and retain explicit model/billing selection.

Exit evidence must demonstrate:

- Matching supported packages pass clean installation, returning sign-in,
  restart, upgrade and the documented uninstall/data-retention path. Missing
  prerequisites, failed preparation and recovery are understandable in the UI.
- An actual official Codex subscription run produces the benchmark application
  through Define → Design → Develop → Deploy; independent checks verify its
  behavior and a reachable public site matches the checked commit/release.
  The verifier must not supply the agent's implementation.
- Live configured lcfr.ai LiteLLM access works across the supported phases;
  disabled, incompatible and failed routes retain a useful recovery path and
  never silently switch provider, model or billing account. Supported direct
  alternatives remain available without losing saved selections.
- One bounded visual/accessibility pass covers keyboard navigation, focus,
  labels, readable contrast, responsive sizes and loading/empty/error states;
  correction verification confirms the actual fixed interactions. Supported
  controls perform their stated operation and preserve state after restart.
- Working data, application authentication, permissions, storage, live updates,
  supported functions and email flows remain part of the same project journey.
  Scope claims to the operations actually verified in the installed package.

## Stage 4: Operable infrastructure and maintenance

Verify local Docker, existing hosts and the supported user-owned AWS and Azure
paths against real resources. Keep provider administration behind the project
workflow. Before creating cloud resources, show current region/currency quotes,
storage, addresses, traffic, backups, domain/tax exclusions and separately billed
models; obtain the product's explicit cost acknowledgement. A budget setting is
not a provider billing cap. This roadmap does not authorize unattended spending.

Exit evidence must demonstrate:

- Each cloud path provisions or connects its owned resources, verifies SSH
  trust and prerequisites, deploys the application, connects an owned domain
  with valid TLS, reconnects after interruption and cleans up only identified
  owned resources. Generated plans and fixture responses do not pass this gate.
- Deployment inspection, observed health, stop/start and supported rollback
  identify the exact release and resource. Lost ownership or an unreachable URL
  cannot continue to display a verified ready state.
- Data backup and restoration recover actual application data after restart or
  an interrupted operation. Failed first installation preserves original
  volumes; a stopped or unmarked database is never assumed empty.
- External SMTP acceptance and actual inbox delivery are recorded separately;
  supported account recovery and application email links work with appropriate
  expiry and one-time behavior. A local mail fixture proves only its own scope.
- Managed-team operations have tested provisioning, restoration, upgrade,
  revocation, capacity and resource ownership. Record measured recovery results
  and capacity limits before promising availability or recovery targets.
  Monitoring, retained audit events and incident/support procedures have an
  identified operator without exposing project content or secrets to other
  teams.

## Stage 5: Release gates

Open-source distribution and paid service availability require separate
decisions and evidence. Neither is implied by a merge, a source suite or the
completion of one small deployment. Prepare concrete reviewable candidates
before requesting the credentials or authorization needed to publish.

| Gate | Required evidence |
| --- | --- |
| Source and legal | Clean pinned source, matching version/manifests, MIT grant for Luci, complete upstream grants/attribution, no restricted proprietary source or embedded credentials, documented supported scope. |
| Build and integrity | Required CI at the candidate commit, immutable package hashes, supported platform signing/notarization where required, verified update and signature behavior, and retained failed build evidence. |
| Public distribution | Actual signed release/download configuration, a working public download surface, source links, installation/upgrade/recovery instructions, and fresh installation of those exact downloaded bytes on supported hosts. A local archive is not a public download. |
| Full product acceptance | Current installed end-to-end outcomes and the separate live subscription, gateway, AWS, Azure, domain/TLS and external SMTP gates. Any unsupported platform or operation is explicitly excluded from the released support claim. |
| Paid team availability | Stage 2 collaboration and project authorization, independently verified dedicated team boundaries, restore/upgrade and revocation evidence, measured capacity, clear model/infrastructure charges, and operational support ownership. |
| Commercial operations | Reviewed service terms and privacy/data-retention/export/deletion behavior, usable onboarding and cancellation, tested billing/entitlement boundaries and truthful service commitments. Disabling billing must not silently destroy project data. |

The first managed pilot uses a deliberately bounded supported configuration.
Agree its team size, resource budget and operational commitments from measured
results before taking payment. Expansion to more regions, larger teams or
shared-host execution requires new relevant evidence rather than inheriting the
pilot's readiness claim.

## Evidence and continuation rules

Record each stage outcome in [FULL-PRODUCT.md](FULL-PRODUCT.md), including source
commit, package digest when applicable, execution environment, operation,
observed result, retained artifact path, failure/retry history and remaining
prerequisites. Keep these evidence classes distinct:

| Evidence class | What it establishes |
| --- | --- |
| Source inspection and unit/integration tests | The scoped code or contract tested; no installed, provider or public-service claim. |
| Deterministic fixture or synthetic provider | Repeatable integration behavior under the fixture; no actual subscription, cloud resource or external delivery claim. |
| Matching installed package | Behavior of the identified downloaded/built package on the recorded host; no broader native/platform claim. |
| Live external operation | The actual provider/account/resource/domain/inbox operation observed, with its source/package binding and limitations. |

Check existing authorized connections and the presence of configured credentials
before requesting missing access. Never place secret values in documentation or
chat. Live subscription sign-in, a gateway URL/access, authorized AWS/Azure test
accounts and spending, an owned domain, external SMTP inbox and release signing/
public-download access remain prerequisites until verified in the current
environment. Missing access leaves a gate pending while independent work
continues; it cannot become a pass or justify invented results.

Use these stages as a persistent backlog, assigning concrete next outcomes and
file ownership before parallel work. Update implementation and acceptance
status separately as evidence arrives. Do not claim a percentage, completion
date, total cost or full production readiness without supporting measurements.
