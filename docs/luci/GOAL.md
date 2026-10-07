# Finish the full Luci product

The owner's direction on 5 October 2026 expands the earlier four-phase Docker
benchmark. That benchmark remains mandatory evidence within this full product
goal; it is not the definition of completion. This goal supersedes earlier
claims that a visual demo, a source test suite, or a single small deployment is
sufficient for handback.

Deliver a complete, polished, MIT-licensed lcfr.ai open-source application that
someone can use to complete a whole solution. It must have a coherent experience
from first launch and account creation through defining, designing, developing,
deploying and maintaining the result. The user can choose local Docker, their
own AWS or Azure resources and domain, or an existing workspace. Minimize ongoing
cost while preserving the quality of the result. Reuse suitable open-source
components discovered through lcfr.ai Glasses where this improves delivery.

## Product-quality completion standard — owner's latest direction

Deliver an exceptionally polished, dependable idea-to-production product, with
the clarity, responsiveness and consistency people expect from Vercel, ChatGPT
and other leading AI-era applications. The standard covers the complete work,
including context building, amendments, collaboration, agent execution,
verification, deployment and operation. A clean launch or passing technical
suite does not complete this goal.

The person must understand the solution being agreed and built at every stage,
know what requires their decision, see the consequence of a change, and have a
clear way forward after an interruption or failure. Luci must carry this
understanding into an independently verified, live application and subsequent
changes. Persistent conversation remains optional.

[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md) defines the observable quality contract,
representative complete journeys and evidence required to close this goal.
Treat it as a release gate alongside the existing functional, isolation,
commercial and distribution gates. Keep improving concrete shortcomings until
the contract is demonstrated; neither blocked Windows CI nor missing external
credentials excuses independent product and interaction work.

Current status: **not complete and not production ready**. Existing scoped
test/package results are valuable evidence, not a verdict on whole-product
polish or live model quality. The target is no known serious product or usability
defect in the supported journeys, with understandable recovery from expected
failures. Do not promise that any application can never encounter a future bug.

The owner's renewed continuation instruction is explicit: a completed corrective
batch, updated PR or saved roadmap is a checkpoint, not the end of this objective.
Continue through the remaining independently actionable product work. A missing
credential or platform capability blocks only the gate that requires it; it does
not close the goal or justify stopping unrelated implementation. Keep concrete
findings and their resolution in [QUALITY-WORKLIST.md](QUALITY-WORKLIST.md), and
keep the whole-product acceptance contract open until its outcomes are observed.

## Open-source and managed-team goal — 6 October 2026

The owner has confirmed two delivery options: the self-hosted MIT open-source
application and optional paid lcfr.ai-managed team workspaces. Both must solve
the same AI software-development lifecycle: people build and review useful
shared context, agents receive the exact authorized context, and the team can
verify, deploy and maintain the resulting application in one coherent product.
Local use remains supported without a mandatory hosted account or subscription.

The first paid deployment uses a dedicated execution boundary for each team,
with private state, secrets, containers and resource ownership. Existing
per-user application workers are useful account separation, but are not proof
of isolation between paying teams on a shared host. The 0.5.0 candidate adds explicit team membership, project grants, shared
workspace routing, authenticated context attribution and scoped draft recovery.
Those source capabilities do not establish dedicated host isolation or a paid
service. Paid team availability requires verified membership/project permissions,
trustworthy review, concurrent-change recovery and revocation across files,
context, agents, previews and deployments. Secret use and execution privileges need separate
authorization from the right to read or edit project context.

[PRODUCTION-ROADMAP.md](PRODUCTION-ROADMAP.md) defines measurable stages for this
continuing goal. The **0.5.0 release candidate** includes the durable coordinator in
[PROJECT-RUN-CONTRACT.md](PROJECT-RUN-CONTRACT.md),
[portable context and bounded storage](CONTEXT-STORAGE.md), team permissions,
scoped recovery drafts, operator service admission and
[dedicated team HTTPS ingress](PUBLIC-HTTPS-INGRESS.md). The public ingress uses
separate app/preview sites and preserves session-bound app/data browser grants;
local TLS browser evidence does not prove public DNS or certificate operation. Implementation and source
tests do not establish shipment or installed acceptance. Continue through the
matching installed collaborative journey, operational maintenance, and separate
open-source and paid release gates. Candidate integrity attestations and optional
Windows publisher-signing support are reviewable tooling; no production signing,
public publication or hosted subscription activation has been performed. Preserve the required live subscription, gateway, AWS, Azure, domain,
SMTP and signed/public-download evidence below; a smaller milestone does not
replace the full completion contract.

## One understandable project journey — 6 October 2026

The owner's latest clarification prioritizes a clear view of the current stage,
exactly what Luci and its agents are building, and how amendments change the
solution. Persistent conversation is useful supporting history, not a required
primary interface or a completion gate. Luci must understand the agreed idea,
build shared context through each phase and carry it through implementation,
verification and a live application. Phase editors are focused views of that
same project; the current purpose, requirements, selected tasks, execution state
and useful next step must remain understandable across them.

Luci asks about material ambiguity and explains consequential recommendations
and downstream effects. A reviewed amendment creates a versioned draft,
invalidates affected approvals and keeps earlier run inputs/evidence immutable.
Unresolved material questions block new coordinated implementation; an
explicit answer and any needed renewed context review provide the path forward.
A saved discussion, recommendation or completed process never proves a working
outcome. Provider-backed assistance must report the actual chosen connection and
failures without simulated answers. Existing checks, release observations,
public publishing consent and cloud cost acknowledgement remain execution gates.

[Project understanding and amendments](ORCHESTRATION.md) specifies the shared
status, immutable build scope, real provider assistance, reviewed changes and
bounded private history recovery. Acceptance must show understandable stage and
selected work during actual agent activity, including a teammate amendment and
its effect on existing run inputs; retained chat alone is not acceptance.

## Overnight completion instruction — 6 October 2026

The owner has explicitly authorized continued autonomous work while away and
reaffirmed that the result must be the full, finished product. Finish implementation
and actual installed acceptance together. Resolve known blocking defects,
unfinished core controls and disconnected workflows before handback. A control
must perform its stated operation, show truthful progress and give a useful
recovery path; obsolete phase/demo guidance does not meet this standard.

The target is an excellent, coherent experience that can create and maintain a
complete application end to end. Check meaningful success, failure, retry,
restart, source/version and account/project isolation outcomes. Use bounded
visual/accessibility checks and appropriate engineering tests rather than
repeated cosmetic reviews. Required external credentials remain prerequisites;
missing credentials never turn an untested outcome into a pass. Literal absence
of every possible future defect cannot be promised, and must not replace concrete
acceptance evidence.

The platform's 7 October continuation now supplies this full objective as the
active goal: finish Luci as a fully fledged, polished application with provable
quality across its supported controls and journeys. The goal remains active
until the outcomes are demonstrated, or all useful independent work is exhausted
and a concrete external prerequisite blocks further progress. The available goal
control can mark the objective complete or blocked; it is not evidence that the
application has passed any release gate. Earlier observations that this cloud
session exposed no goal control are historical. Do not close the full objective
for a smaller benchmark, corrective batch or saved roadmap.

## Required product outcomes

The owner's 6 October CLI instruction is part of this completion contract.
Supported coding harnesses and general CLI work must be able to create and edit
files in their assigned project while retaining account, project and sandbox
boundaries. Read-only verification is distinct from the coding workspace.
Choosing a harness should reuse the connected API access and inject its supported
provider configuration without making the user assemble login-method settings.
Provider subscription sign-in remains a real external protocol where required;
never claim universal account reuse, silently import host credentials or label an
unconnected provider ready. Use the requested GPT-6 Astra review to find and
implement concrete gaps across these contracts and the full polished journey.

The lcfr.ai connection will be a configurable LiteLLM gateway, with Azure OpenAI
or AWS Bedrock available behind that gateway. A person can enable it, switch it
off, or choose their own compatible endpoint or Codex subscription without losing
the saved alternatives. Protocol compatibility is explicit; a selected connection
must never silently change the provider, billing account or model. A supplied
gateway address is required before claiming live access. Astra's review covers
the entire product and its release evidence, including orchestration, recovery,
distribution and the coherence of all four phases. Add stronger orchestration
where concrete findings require it, with measured outcomes rather than a model
name standing in for reliability. This is a full product release contract, not a
demo or a preliminary test.

1. A downloaded matching package launches successfully. Account creation,
   returning sign-in and account recovery are discoverable. Onboarding explains
   execution choices, checks prerequisites, prepares supported dependencies,
   records progress and gives useful remedies when a step fails. Planning can
   continue while installation runs. No configuration field alone proves ready.
2. Projects have a dedicated home with creation, Git import and backup restore.
   Project context is consistent across navigation, files, agents and releases.
   Appearance, density and accessibility controls work and survive restart.
3. Define supports a useful guided interview and genuine existing-repository
   analysis, selected provider/model access, stable acceptance criteria,
   requirements, journeys, roles, entities, integration constraints and versioned
   approval. Partial model findings are labelled and reviewed before saving.
4. Design supports requirement-linked screens, states, responsive targets,
   working prototypes, assets/tokens and an explicit implementation handoff.
   Saved sketches and screenshots alone do not prove the designed interactions.
5. Develop connects the person's provider account, runs real coding agents in
   the selected execution environment, retains run progress and diagnostics,
   exposes real Git changes and previews, and verifies the implementation against
   the approved requirements and design. The benchmark implementation must be
   produced by the actual Codex subscription run, not inserted by its verifier.
6. The application platform exposes working deployment, database, application
   authentication, row policies, storage, live updates, supported functions and
   email workflows inside Luci. Public client settings and private server secrets
   remain distinct. Supported operations need real health and ownership checks,
   durable progress and actionable failure states.
7. Local Docker and user-owned AWS/Azure paths integrate with the same project
   experience. Cloud plans explain prerequisites and actual quoted costs;
   resource creation requires the product's explicit cost acknowledgement.
   Existing-account/server connections, SSH trust, installation, reconnect,
   domains/TLS and deployment have concrete supported paths. No placeholder
   provider success, invented quote or unreachable ready URL is acceptable.
8. Maintenance includes restart persistence, deployment inspection, supported
   rollback/stop, data backups and restoration, and safe recovery of failed
   operations. Failed-first-install recovery retains original volumes and never
   assumes a stopped or unmarked database is empty.
9. The UI is coherent, readable and polished at the level requested by the owner.
   It has clear next actions, accessible labels and keyboard interaction,
   responsive layouts, useful loading/empty/error states and consistent visual
   tokens. No supported core journey ends in an unexplained error or dead control.
   Prefer existing suitable UI through Glasses, especially the official Fluid
   Functionalism controls. Everyday use stays calm and understandable, with
   runtime detail behind deliberate disclosures. Develop exposes actual CLI work.
10. Distribution preserves MIT and upstream licence grants, accurate provenance,
    matching source/package manifests and actual download/install instructions.
    Release readiness includes the supported clean-host installation outcomes;
    a locally built archive is not evidence of a public download or native install.

## Acceptance and handback

Use installed matching packages for complete connected journeys and preserve
both successful and failed evidence. Source tests, fixture providers, generated
plans and static reviews are scoped evidence only. Verify functional outcomes,
restart, failure/retry, recovery and account/project isolation. Run a batched
visual/accessibility inspection and confirm fixes without repeated cosmetic
review loops.

The real Docker/Codex Describe→Design→Develop→Deploy journey must finish with a
working public site whose behavior, commit and release provenance are verified.
Separately, supported live AWS and Azure account/resource/domain journeys need
their own evidence, including recovery/reconnect and owned-resource cleanup.
SMTP acceptance must be distinguished from inbox delivery. User-owned external
credentials and resources are prerequisites; absence of them is not a pass.

Do not hand back the application as fully tested or complete until every required
outcome has adequate current evidence. Keep implementation checkpoints and
unverified external prerequisites explicit. Update `FULL-PRODUCT.md` with
concrete outcomes and evidence as the build advances. The earlier narrower benchmark remains historical; it must not override the
owner-approved full objective supplied by the current platform goal.
