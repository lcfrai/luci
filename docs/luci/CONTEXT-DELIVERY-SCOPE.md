# Context and delivery scope

Owner clarification received 11 October 2026 UTC (12 October on the owner's local date). This supersedes earlier replacement-harness emphasis, while retaining existing permissions, source protection, recovery defects and honest acceptance requirements.

## Objective and product boundary

Luci helps people build the best possible project context and turn it into a verified, delivered application using existing development tools. The user should understand what is agreed, what is ambiguous, what changed, and what action comes next across **Discover, Design, Develop and Deploy**. Activities can be revisited together; they are not a rigid wizard.

The priority workflow is:

1. Discover the purpose, users, constraints, risks, exclusions, decisions and measurable acceptance outcomes. Ask about consequential ambiguity rather than silently inventing intent.
2. Design screens, states, responsive behavior, data and interaction boundaries. Review the specification and design, link stable requirement/task IDs, and approve exact versions.
3. Export immutable, task-specific context for Codex, Claude Code or another development tool. Show its source/version binding and what the tool is being asked to deliver. Use the tool's own authentication; do not copy its credentials or replace its execution engine.
4. Bring back actual code and a report. Inspect source changes independently, distinguish reported checks from verified checks, detect changed context or a stale review, and retain the original assignment.
5. Review and commit the result, run independent checks, preview the exact source, deploy with explicit authority/cost controls, and amend the context when the idea changes.

Across four or more projects, retain goals, decisions, drafts, active work, evidence and next actions without leakage or repeated explanation. Keep existing resource limits and measured responsiveness targets; more simultaneous workers is not itself success.

Existing managed sessions and swarm scheduling are supported execution options. Their correctness, ownership, cancellation and recovery defects remain obligations. New general-purpose runtime features, additional harness abstractions and repeated unchanged fixture exercises are subordinate to the primary context-to-delivery workflow. Do not expand those mechanisms to substitute for a missing usable external handoff.

## Audited baseline at edc742e8

The current product source is `edc742e8b6ce4216a1726e74cceef892663ce3d5`. Later scope/evidence publications do not change those product bytes.

| Capability | Current support and limitation |
| --- | --- |
| Structured context | Discover specification contains purpose, audience, constraints, exclusions, requirements/acceptance, journeys, roles, data, integrations, decisions and questions. Design contains screens/states/responsive targets, tokens, assets and notes. The plan contains stable task IDs, requirement links, dependencies, write paths and acceptance. |
| Portable generated files | Saving writes `LUCI.md`, `.luci/specification.json`, `.luci/specification.md`, `.luci/design-handoff.json`, `.luci/design-handoff.md`, `.luci/plan.json` and `.luci/design.json` when their corresponding state exists. These are Markdown/JSON context data; not all files exist before that context is saved. |
| Canonical versions and approval | `.luci/state.json` and bounded history retain project context. Private approval receipts bind exact reviewed contents. Copied files or a tool's reported approval do not confer permission. Discover/Design Context storage reports missing/changed exports; explicit repair exists. |
| Immutable selected-task bundle | Existing internal capture adds task-specific instructions and `.luci/brief.json`, manifest/file hashes and exact source binding, but is delivered through Luci-managed sessions. `GET /v1/sessions/{id}/reviewed-context` inspects an existing owned capture; it is not a standalone exporter. |
| Other exports | Project backup preserves a larger project/history archive; design export is static HTML/sketch export; orchestrator export is conversation. None is a substitute for an immutable approved external assignment. |
| Existing implementation UI | Develop exposes Prepare project run / Start implementation session. These invoke Luci-managed work rather than a separately authenticated external tool. |
| Returned work | Existing Git views/commit, checks, releases and managed task reports cover their own paths. There is no standalone generic return-review API/UI at this baseline. A manual filesystem/CLI trial must be labelled manual glue. |

**LOCAL-025 (P1, fixing)** closes the last two gaps. The minimal implementation reuses approved canonical capture and existing Git/source protections, exposes a downloadable ZIP containing exact context and a hash manifest, retains export history, and adds a reviewed return report. It does not launch a tool, create a sandbox, overwrite `AGENTS.md`/`CLAUDE.md`, or import arbitrary commands as execution authority. Exact API/UI instructions and the tested implementation revision will follow with its source checkpoint; this scope document is not a feature-completion receipt.

## Return contract and stale context

The frozen assignment names project, selected tasks, approved context revisions/digests and base source. The returned implementation may legitimately differ from that base: those are the changes to review. Luci must bind the review to the actual current source and reject a submission if that source changes again before saving the review. Changed approved context is a separate amendment requiring explicit reconciliation/new handoff; it must not silently relabel the old work.

Retain reported summary, changed paths, decisions, checks, limitations and unresolved questions as **unverified tool claims**. Independently observed Git changes are distinct from that report. Saving a return does not approve context, mark tasks complete, merge code, execute a command, pass acceptance or deploy. Preserve original exports and returns across project switching and restart. The person can inspect the return, run/review checks and continue to preview using the existing source-bound paths.

## Real-provider trial and exact blockers

The Windows tester reports Codex CLI `0.162.0-alpha.17.2` is signed in using ChatGPT. That supplies a real external provider path without embedded container authentication. No auth file was read/copied. The tester owns the isolated Windows invocation, checks supported CLI flags locally and retains its actual JSON events, exit, model-generated changes and independent checks. A command-level smoke test alone is insufficient.

Current blockers and work that can proceed:

- **Integrated handoff/return:** LOCAL-025 is an implementation gap, now being fixed. Meanwhile the existing saved Markdown/JSON exports can support a clearly labelled manual preliminary trial; do not describe it as the finished workflow.
- **Context readiness:** save and approve an actual specification/design and select actionable tasks. Record consequential ambiguity and acceptance, rather than feeding a prewritten implementation to the provider.
- **Provider:** local signed-in Codex is available by tester report. Its actual inference, generated output quality, limits and failed/interrupted behavior remain unverified for this trial. No new cloud key or container sign-in is required for the external-tool portion.
- **Returned source:** use an isolated owned project, preserve the original assignment and repository identity, inspect all actual changes, and do not allow the tool to rewrite Luci's approval/context records as a way to pass review.
- **Checks/preview:** local toolchain/dependencies and Docker/base/runtime admission are separate prerequisites for the relevant commands. External context export must not depend on runtime installation. Keep real installer, preview cancellation/recovery and resource gates open at their recorded scopes.
- **Product polish:** CLOUD-032 switching still fails its unchanged 300 ms acceptance target. Existing native close/external-API ownership, four-project resource, recovery and full idea-to-app checks remain open.

## Concrete acceptance sequence

1. In a fresh isolated Luci project, author an ordinary-language idea. Resolve a material ambiguity, save measurable requirements and design states, approve exact versions, and create linked tasks. Keep a second project with distinct context for isolation checks.
2. Use the standalone product control to review and export the assignment. Verify archive file hashes and source/version/task identity; extract into a fresh context directory rather than overwriting the working project or user agent files. Preserve original bytes.
3. Give only that packet plus the owned source checkout to the already-signed-in external tool. Ask it to implement the selected task, record unresolved questions and produce the specified return report. Record actual provider execution, not a scripted response or verifier-written solution.
4. Return through Luci's product UI. Review actual source changes alongside reported checks. Test a foreign handoff, context amendment and source change after review; each must refuse or require explicit fresh review without losing the earlier packet/report.
5. Independently exercise the changed application and its requirements. Preserve failing checks, fix through the context/tool loop, then review/commit and preview exact source. An agent's claim or exit code cannot be the only evidence.
6. Amend one requirement, export a successor packet, and verify old work retains its original context. Switch among four projects and restart Luci; export/return/history/drafts must remain project-scoped and understandable.
7. Record observed quality, human intervention, elapsed time, provider usage if available, all owned-process resources and UI timing. Keep missing observations explicit. No market-leadership claim follows from one example.

## Comparison and reuse

Evaluate tools by their actual role: Codex/Claude Code for implementation and their own authentication/execution; GSD-family approaches for useful planning/context decomposition; Nimbalyst for understandable project work and artifact review. Existing research/evidence is retained. Recheck official current workflows when implementing an adapter; do not copy an entire competing harness or infer compatibility from a brand name. The Windows tester is independently checking official external-tool workflows. No fresh comparative benchmark or unsupported superiority claim is made here. Glasses MCP is not callable in the currently exposed tool set; that does not block reuse of Luci's existing canonical context machinery.

## Goal and coordination status

The documented working objective is active and unfinished. Persistent platform-goal controls are unavailable; the last confirmed platform status remains blocked. Thread activity and a scope edit do not reactivate it.

The authorized local test loop continues at thread `01a1199f-7500-7843-b02b-081081e536fa`. This session currently has no thread-message tool, so immutable checkpoint links and clearly labelled updates in the cloud thread are the handoff mechanism. Adopt source only between test batches. Scope changes do not erase existing issue IDs, failed receipts, unknown charged leases or prior passing scopes.
