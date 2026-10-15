# Develop orchestration and quality amendment

## Authority and priority

Dylan's 15–16 October direction makes Develop Luci's main product differentiator: design-derived orchestrators, large communicating swarms, fast delivery, consistently high quality and very little unnecessary human supervision. Herdr and Firstmate are references. The other four Ds remain required and provide approved context, evidence and feedback to Develop. A usable local end-to-end build is the first delivery milestone.

This extends the reviewed five-D contract at `0de1be9b87b8bb66e949e3cdb988cc52c1a980aa`. LC-01–22 are preserved verbatim; LC-23–24 make this direction explicit. Root reviews this exact committed map and records active worker acknowledgments before added-scope implementation. No repeated user approval is needed. Existing LC-17 and authentication/correctness work may finish without interruption. Scope alignment is not product acceptance.

The product's orchestrator hierarchy is distinct from the team building Luci: this development chat remains the sole Astra orchestrator, execution workers remain Sol, and nested development delegation remains disabled.

## LC-23: Design produces an executable orchestration contract

The personal Luci remains the person's primary contact. From an approved Design revision it prepares a durable execution contract for project or specialist orchestrators and their upstream coding agents. The person reviews outcomes and meaningful decisions rather than manually moving work between agents.

The contract binds:

- Exact project, actor/team, goal, requirements, design/scene, evidence and source revisions; dependency-linked task scopes, ownership and integration targets.
- Team standards and applicable risk controls at explicit revisions, with evidence expectations, unresolved conflicts and exceptions. Existing free-form standards must retain provenance until a structured library is available; do not pretend that a missing library has been assessed.
- Agent roles and selected upstream harness/model connections; scoped read/write permissions, environment identity, resource/concurrency/spending limits and credential boundaries.
- Observable task and release acceptance criteria, required checks, independent review, repair/retry limits and completion evidence.
- A decision policy identifying routine authorized choices, reversible experiments, assumptions that need validation and matters requiring human input. Record each consequential decision's basis and outcome.
- Cancellation, stale-context invalidation, restart/checkpoint and unresolved external-outcome reconciliation behavior.

Preparation is side-effect free: it starts no agents, models, containers or VMs and grants no new authority. Approval is bound to the exact contract digest. A changed design, team standard, control or human intent marks affected work for reconciliation; never rewrite a running agent's captured instructions. Compatible updates should continue automatically under policy. Contradictory intent, unavailable required evidence, missing credentials or actions outside authority produce a precise request.

The ~99% ambition is a measurable target for eligible routine decisions, not a guarantee, permission to skip controls, or a reason to classify difficult decisions away. Report numerator, denominator, exclusions, task mix and observation period. Required human approvals remain visible. The 100x aspiration is not a measured productivity claim.

## LC-24: Communicating swarms carry work through verification and delivery

Orchestrators decompose work into useful independently scoped tasks, run ready tasks concurrently, coordinate dependencies and share relevant discoveries. Use existing upstream coding harnesses, durable run/scheduler/context machinery and ordinary Luci connections. Do not build another coding engine or execute ordinary assistant prose as a tool call.

Messages and results have durable identities, sender/recipient and project/team scope, causal context revisions, task/attempt identity and typed intent. Evidence and artifacts are referenced by immutable identity instead of copying entire histories. Delivery, acknowledgment and replay are idempotent; duplicates, delayed events, disconnected participants, revoked access and stale results cannot silently cause repeated execution or overwrite accepted work. Agent-supplied content remains untrusted evidence, not permission to expand another agent's authority.

Supervision is event driven. Persist decisions, checkpoints and obligations; resume from them after restart. Separate working, queued, blocked, awaiting permission, verifying, stopped, failed and unknown states. Idle terminal output or a model's completion statement cannot establish completion. Avoid always-hot coordinator models and high-frequency model polling. Use bounded projection/event subscriptions for the UI; changing concurrency must not create per-agent polling storms.

Each executing task attempt owns a scoped local Docker environment or a newly allocated cloud microVM with its own workspace and lifecycle. Resource limits, provider quotas and explicit cost policy determine admission and backpressure; no arbitrary tiny product ceiling or promise of unlimited capacity. The existing representative cloud benchmark of 12 tasks across four projects with four concurrent independently owned microVMs remains a required observation, not a maximum product size. Pure coordinators can sleep between events without keeping task workers hot.

An integration loop accepts actual changes only after evidence-based checks and independent review. It reconciles branches and compatible changes, rejects stale or conflicting results, repairs observed failures within limits, verifies the runnable app and proceeds to policy-permitted deployment. Preserve failed attempts and review findings. Unknown runtime/provider outcomes remain unknown until reconciled. Stop, cancel, retry and restart must not duplicate execution, discard outputs, abandon owned resources or hide charges.

Develop presents the useful result of this coordination: current objective and critical path, meaningful progress, live agent/work panes, dependencies and messages, reviewed diffs/checks, attention requests and a clear next action. Progressive detail and the Cube-style tiled workspace serve real work. The person can inspect, redirect or stop work without babysitting routine handoffs. Keep keyboard access, reduced motion and constrained-host responsiveness.

## Implementation map and ownership

The five-D amendment's LC-01–22 map remains controlling. This is the additive map, plus delivery dependencies; existing source is not a verified end-to-end pass.

| Obligation | Existing foundation and gap | Owner | Required evidence |
|---|---|---|---|
| LC-23 execution contract | Versioned design/context, project runs and approvals exist. A complete design-derived orchestration contract, team-policy binding and decision audit are incomplete. | Cloud Sol backend; local Sol UI when assigned by root | Prepared contract has no effects; exact approval and policy/context revisions; current grants enforced; design/control changes invalidate affected work; independent tasks remain usable. |
| LC-24 durable coordination | Project swarm scheduler, integration, operation/lease and team context stores exist. Audit actual message, supervision, replay and completion gaps before adding another abstraction. | Cloud Sol scheduler/backend; local Sol Develop UI and bounded acceptance | Multi-agent scoped messages/results, duplicate/reorder/disconnect/restart, amendment/conflict handling, real code, independent checks, repair and retained outputs. |
| LC-04/05/13 local delivery | Source improvements and scoped Windows passes exist; normal Codex account connection and real coding remain incomplete. | Local Sol, root exact runtime review; cloud Sol defects | Real upstream task produces code, reviewed checks and working app with positive cleanup and responsive host. |
| LC-15/16 elastic cloud operation | Azure and swarm preparation exist; real environment has not been supplied. | Cloud Sol offline implementation; later root acceptance | Required per-task microVM fan-out, owned retirement, recovery and charge reconciliation; remain explicitly untested until permitted. |
| LC-19/20 user control | Versioned design and real terminal/files/diff foundations exist; orchestration controls and scene handoff need integration. | Local Sol desktop; cloud Sol typed backend | Exact approved design reaches orchestrators; usable progress, attention, redirect/stop and reviewed output across narrow/wide layouts. |
| LC-22 standards/control feedback | Existing team/context authority, new XLSX workflow still incomplete. | Cloud Sol | Team policy revisions become execution constraints; uncovered gaps return to Design and invalidate stale evidence without inventing compliance. |

## Work sequence and measurable quality

1. Finish the already-active LC-17 slice and the matched LOCAL-065 sign-in recovery path. Do not interrupt active integration or restart unchanged tests. User account approval is a genuine prerequisite for the selected Codex connection; continue independent source work while unavailable.
2. Ship LC-23 through the existing Design/run/approval path, not as an unused JSON type: prepare, inspect, approve, capture and invalidate exact context and policy. Preserve old project data and explicit legacy/unassessed state.
3. Extend the existing scheduler/supervisor for LC-24. Implement one complete local path from accepted contract through real upstream tasks, durable communication, review/repair, integration and verified run. In parallel only where file ownership is disjoint, expose useful orchestrator/agent activity in Develop. Scale from a working vertical rather than calling a populated task graph a swarm pass.
4. Add meaningful concurrent-task and four-project scheduling, amendment, failure/cancel/restart checks. Prepare cloud parity offline, retaining the live provider gate. Integrate risk/standards feedback and the remaining Discover/Design/Distribute work without displacing the core delivery loop.
5. Maintain reproducible quality/speed cases: time to first useful reviewed diff, time to verified working result, accepted-task throughput, review/rework and regression rates, human intervention with reasons, model/compute cost per accepted outcome, queue fairness and whole-host resource/interaction latency. Record workload, exact revisions, model/harness, cold/warm state, concurrency and failures. No synthetic speedup or model self-grading presented as product evidence.
6. Push reviewed completed work to main. Invite user testing only with a usable local handoff and precise remaining cloud, package and signing limitations. Source tests, fixtures, compiler success and screenshots cannot close real execution gates.

## Reference and reuse decisions

- [Firstmate](https://github.com/kunchenguid/firstmate/tree/9b1ada0401f9fa7ea3ee07a8a7f005a99ce0de58), pinned commit `9b1ada0401f9fa7ea3ee07a8a7f005a99ce0de58`: inspected README, VISION and MIT licence (Copyright 2026 Kun Chen). Useful patterns are one user liaison, scoped crew worktrees, durable supervision, specialist coordination and event-driven watches. This amendment imports no source. Any later source reuse must preserve its licence and attribution and fit Luci's authority/isolation boundaries; no unsafe upstream defaults are inherited.
- [Herdr agent documentation](https://herdr.dev/docs/agents/): inspected real terminal and integration-reported activity, durable sessions and attention. Adopt truthful observable state; idle is not verified completion. No Herdr source, service or licence assumption is imported by this reference.
- [Cube workspace reference](https://cube.computer/simple/app-builder-2480.webp): retain the inspected tiled workspace interaction direction from the five-D amendment. Visual similarity alone does not fulfill Develop.
- Glasses discovery was unavailable at the recorded local endpoint. Primary source inspection is the documented fallback; no heavy discovery service was started.

## Preserved boundaries

No live cloud account query, provisioning, spending or public deployment is added. All normal resource reserves, exact reviewed runtime exceptions, provider/credential authority, per-agent isolation, unresolved producer/observer automatic-review holds and Defender/package/signing holds remain. Approved context is not blanket permission for dangerous or irreversible actions. The UI, benchmark and handoff must distinguish implemented source, observed runtime behavior, provider proof and unresolved outcomes.
