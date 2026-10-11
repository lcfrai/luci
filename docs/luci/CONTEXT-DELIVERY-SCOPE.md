# Context, orchestration and delivery scope

## Current status — scope audit passed, implementation resumed (12 October 2026)

The originating local thread explicitly lifted the scope gate for contract revision `b3ecb0114dba6b6e0dfcfedbf1d3a3d177794707`. **Aligned implementation, delegation and verification are authorized to resume now; no further scope confirmation is needed.** This is scope alignment, not product acceptance or platform-goal activation. Existing provider credentials, cloud cost/authority policies and all unfinished release gates remain applicable. See [the audit-lift record](SCOPE-AUDIT-LIFT-2026-10-12.md). Any preserved paused-state wording below describes the prior gate, not a current stop instruction.


## Controlling contract and scope audit

[PRODUCT-CONTRACT.md](PRODUCT-CONTRACT.md) is the single controlling product direction and requirement-to-code map. **The owner scope gate is held: no new implementation, delegated work, tests/builds or provisioning until the originating local audit explicitly lifts it.** Contract publication does not resume work. Existing safe diagnostics/owned cleanup may finish; preserve draft source and failed evidence.

One primary persistent conversational Luci per person coordinates Discover, Design, Develop and Deploy. The four phases are revisitable tools of that continuing workflow. Local Develop boots existing upstream harnesses in isolated Docker; cloud Develop requires a real supported provider microVM backend. Management/app-deployment Docker and SSH compatibility do not satisfy cloud microVM execution.

CLOUD-085 must carry conversational amendments through canonical reviewed context/design/tasks, affected upstream work, independent checks/review, preview and actual policy-permitted deployment. CLOUD-086 must coordinate distinct people's Lucis through durable authoritative intent/revision/event/ownership/dependency state, reconcile compatible changes, surface genuinely contradictory intent and continue affected implementation/delivery without human handoff management. Draft-only or preview-only slices are not completion. All current owners are paused; the controlling contract records acceptance criteria, source gaps and worker acknowledgements.


CLOUD-088 is a required P1 workstream dependent on CLOUD-084/085/086: easy elastic fan-out of many small dependency-aware upstream tasks, each with its own newly allocated isolated microVM and observed spin-down/settlement. Concurrency/resource/spend policy, durable task artifacts, shared-context amendments and duplicate-safe restart/cancel remain explicit. The controlling contract sets a representative12-task/4-project/4-concurrent-VM live target, not a product ceiling.

**Corrected owner clarification: 11 October 2026 UTC / 12 October local.** The earlier relay overcorrected “do not replace Codex/Claude Code” into manual host-tool handoff. The owner explicitly corrected that interpretation. This document replaces that mistaken priority; the earlier commit remains historical evidence.

## Primary product and four connected activities

Luci owns **project context, environments/orchestration, review and deployment**. Existing upstream coding harnesses own the coding engines. Luci must boot and orchestrate those harnesses, not merely export files and send the person elsewhere.

1. **Discover:** develop purpose, users, constraints, risks, exclusions, decisions and measurable acceptance outcomes. Resolve consequential ambiguity and keep assumptions explicit.
2. **Design:** develop screens/states, responsive behavior, data and integration boundaries; review and version the context, link tasks and requirements, and approve exact inputs.
3. **Develop:** select an actually supported upstream Codex, Claude Code or other harness and boot it in **isolated Docker locally**. The cloud target is **provider microVMs**, with provider-specific capabilities and limits stated honestly. Give the harness the frozen approved scope, observe actual work, retain useful output/questions, and review changes and independently checked evidence.
4. **Deploy:** preview and deliver the reviewed exact source, observe application health, and support operation, repair and amendments with explicit permissions and cost acknowledgement.

All four activities exchange feedback. Code or preview findings can refine Discover/Design; an amendment must preserve the original assignment and show its effect on work and evidence. The phases are not a one-way wizard. Four or more projects must remain understandable, isolated, resumable and responsive without consuming the host's available resources indiscriminately.

Do not build a replacement coding engine or new custom hypervisor. Preserve proven CLI adapters and use suitable existing/open-source execution solutions. Existing source/authority/resource/cancellation/recovery protections remain required. Shadcn/ui controls should make the integrated path clear and quick; generic runtime mechanisms are justified by that path, not by feature count.

## Supported baseline and missing capability

Audited committed baseline: `22b0ec898de84c00364825caa826f7e8d6239520`, including the optional interchange at2cb58a8a. The earlier edc runtime/package evidence remains scoped to edc; this documentation commit does not change product bytes and excludes preserved uncommitted CLOUD-085 drafts. The currently checked source contains these capabilities; real-provider and platform results retain their independent acceptance scopes.

| Area | Implemented source / actual limitation |
| --- | --- |
| Canonical context | Specification, design, sketch and task plan with stable IDs, revisions, private digest-bound approvals, generated Markdown/JSON and bounded history. Copying exported approval text grants no authority. |
| Reviewed delivery | Canonical selected-task capture is persisted with a manifest/source binding and mounted read-only at `/home/dev/.luci-reviewed-context` for managed implementation sessions. New amendments cannot silently replace the old assignment. |
| Local coding engines | `internal/harness` builds argv for upstream CLIs rather than reimplementing their coding logic. The runtime Dockerfile pins Codex 0.160.0, Claude Code 2.1.274 and OpenCode 1.18.34. Installed/catalogue/option support is distinct from a signed-in account or verified model result. Gemini is not installed; do not advertise it as usable. |
| Local execution | Docker backend, session and task allocation, isolated worktrees, source/review controls, shared resource admission, terminal/task output and recovery machinery exist. Scripted-provider Linux runs prove selected plumbing, not real-model app quality. |
| Codex sign-in | Settings exposes Open Codex account sign-in, upstream terminal device flow, then Save Codex sign-in. The API starts an auth session and saves from its owned session; it does not import an arbitrary host credential path. Host Codex being signed in does not establish the Luci sandbox is signed in. |
| Existing remote workspace | SSH connects to a matching remote Luci service. Current AWS/Azure setup provisions a VM running Docker/Coolify. This is remote Docker execution/deployment, **not per-session provider microVM isolation**. |
| Kata backend | `internal/backend/katabe` is explicitly dormant. Its Create rejects host mounts and WorkDir, both needed by normal sessions. Managed workers currently select Docker. Kata's presence and compilation do not make cloud microVM Develop functional. |
| Delivery | Existing Git review/commit, independent command checks, source-bound preview and deployment controls exist. A handwritten warm Linux Docker build/serve/Stop fixture passed at80462898. It is not a real-provider idea-to-app pass. Cancellation/recovery, cold preparation, Windows and response gates remain separately open. |
| Standalone interchange | Portable files exist; frozen external export/return is committed under LOCAL-025 at2cb58a8a with selected source tests, without matching installed/provider acceptance as **secondary interoperability**. It must not replace or delay primary integrated Develop acceptance. |

**CLOUD-084 (P1, open, cloud execution capability):** usable provider microVM execution is missing. [The source-bound reuse evaluation and implementation contract](CLOUD-MICROVM-EXECUTION.md) records E2B runtime Embed as the preferred first-adapter candidate, replacing the older managed-E2B-first proposal; no provider is yet implemented or verified for microVM Develop. CLOUD-088 explicitly requires elastic per-task microVM swarms, dependent on CLOUD-084/085/086. It needs a concrete reusable backend, actual project/context transfer rather than host-path mounts, supported credential delivery, owned lifecycle and recovery, result return, resource/cost controls and provider-specific verification. Existing remote Docker and dormant Kata cannot be relabelled to close this gap. Local-first acceptance proceeds independently; cloud provisioning waits for a concrete plan and supplied access. Existing source research is retained; further evaluation/implementation is paused under the owner gate. Fresh-storage host replacement, backup/restore, KVM and protected management are explicit obligations, not solved by the candidate choice.

## Immediate real-provider path and blockers

The next trial follows the product UI: save/review context → prepare implementation → choose a supported upstream harness/provider → Docker execution → inspect returned work/questions → reviewed integration → independent checks → source-bound preview → actual policy-permitted deployment → amendment.

The local tester reports an authenticated host Codex CLI, but has **cancelled the proposed manual host-CLI inference trial before execution**. Do not copy its auth file or infer sandbox access. Use Luci's supported container sign-in/provider connection. Exact model availability and account limits must be observed there.

Current actionable gates:

- Complete native Docker runtime installation or use an exact provenance-verified compatible image. Preserve actual host and Docker backing-volume headroom and current CPU/RAM/storage guards; no unbounded installer or unverifiable retagging.
- Establish the selected provider through the intended runtime path. No real-provider full application journey is yet demonstrated by the published scripted fixtures. A CLI version check, auth status or account existence is not inference evidence.
- Build useful approved context in Luci and give a real implementation task to the upstream harness. Do not seed the answer or let the verifier implement it. Retain exact context/source/worker identity and actual output.
- Review and independently check real changes before preview/deployment. Keep reported tests, actual executed checks, source integration, preview health and completed acceptance distinct.
- Complete amendments and four-project continuity/resource checks. CLOUD-032 switching still fails its unchanged300ms target; no layout/helper change or instrumentation subtraction establishes a pass.
- Preserve unfinished native lifecycle and supported-runtime recovery scopes. LOCAL-024 parent-death, reopen, external `LUCI_API` preservation and ordinary titlebar Close are now locally reported verified at577; the exact latest receipt hash is in the controlling contract. This closes the scoped owned-lifecycle regression only. None establishes full native/provider acceptance.

Cloud microVM implementation is an explicit product gap but does not block independent local Docker/provider work. Optional paid service/billing/public cloud access remain later gates under the local-first sequence.

## Secondary interoperability: LOCAL-025

The original P1 classification was based on the misinterpreted primary workflow and is superseded by the owner correction. The bounded implementation is committed at2cb58a8a as optional interchange; no new work proceeds under the gate. It adds no execution engine or credentials path.

Existing portable files, where their context has been saved, are `LUCI.md`, `.luci/specification.json`, `.luci/specification.md`, `.luci/design-handoff.json`, `.luci/design-handoff.md`, `.luci/plan.json` and `.luci/design.json`. `.luci/state.json` is canonical; exports are not private review authority. Context storage can detect and explicitly repair generated-export drift. Backups, conversation export and sketch/static HTML export have separate purposes.

The new optional ZIP retains approved task-specific context, source/version binding and file hashes without overwriting user `AGENTS.md`/`CLAUDE.md`. Its return review observes current Git changes, binds imported reports to the exact handoff/project and labels all reported checks **unverified**. Expected code changes are reviewable; context amendments and changes after review require fresh reconciliation. Saving a report cannot grant execution/deploy authority, mark checks passed or silently merge code. Export/return testing is not evidence for the primary managed Docker workflow.

## Reuse and comparison

Use official upstream CLI behavior for Codex/Claude/OpenCode adapters. Evaluate GSD-family ideas for context/task structure and Nimbalyst for understandable artifacts/coordination; preserve licences and existing research. Cloud-provider microVM reuse must establish the actual isolation boundary, transfer/credential/lifecycle contract and costs before implementation or claims. Glasses MCP is not callable in this cloud session's exposed tools; the local tester supplied pinned reuse research through its restored service, then stopped that owned service after diagnostics. That limitation does not justify inventing reuse evidence or a generic-provider claim.

## Goal and coordination status

The documented working objective is unfinished; implementation is paused for the owner scope audit. Persistent platform-goal controls are unavailable; last confirmed status remains blocked. Scope edits/thread activity do not reactivate it.

Continue the authorized local loop at thread `01a1199f-7500-7843-b02b-081081e536fa`. This session has no thread-message tool, so exact immutable handoffs and labelled cloud-thread updates remain the coordination route. Adopt revisions between test batches. Preserve issue IDs, prior failed receipts, unknown charges and completed scoped passes.
