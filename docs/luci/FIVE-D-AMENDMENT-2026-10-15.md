# Five-D workspace amendment

## Authority and delivery boundary

Dylan's direct product review on 15 October 2026 adds a project home, activity-aware pins, Distribute, document-led Discover, an editable Design canvas and a Cube.computer-style Develop workspace. His follow-up confirms the team XLSX workflow and distinct working, needs-input and stopped project states. This is authorized product scope. Root must record review of this exact committed reconciliation and both implementation workers' acknowledgments before new-scope implementation. Existing sign-in and local correctness fixes may continue independently. No additional user scope approval is needed.

This amendment extends the audited b3ecb011 contract. LC-02 now names five communicating stages. LC-01 and LC-03–16 retain their obligations; LC-17–22 make the new obligations explicit. Previous evidence remains tied to its actual revision and never becomes a pass for these additions. Provider order, Local / BYO Azure / unavailable Luci Cloud setup, isolated Docker and per-task cloud microVMs, authority, resource reserves, signing and installer holds all remain controlling. No live cloud access or spending is added.

## Product behavior

### Home and project rail — LC-17

Home shows all accessible projects with search, current activity, clear attention and pin/unpin actions. The rail has Pinned and Active sections. A pinned project appears once, under Pinned, including all applicable status indicators. Unpinning does not stop work or dismiss attention. Home remains the route to inactive projects.

Working uses a progress icon. Needs input uses a distinct attention icon and accessible text; it must remain obvious if other tasks in that project continue working. Stopped uses a separate terminal marker and remains in the active/recent rail for 24 hours from the observed stop, or until the person opens that project, whichever comes first. Clicking into a project acknowledges only the stop events observed on opening; a later stop is new. Opening does not clear an unresolved question or permission request. Completed, cancelled, failed and interrupted reasons remain inspectable; a failed result never looks like success. Unknown runtime state remains unknown rather than being fabricated as stopped.

Pins and stop acknowledgments are durable per-person, per-workspace preferences; they do not mutate teammates' pins or task lifecycle. If a pinned project becomes inactive it stays pinned. Activity aggregation is project-scoped, combines assistance, execution, queue and attention records and preserves simultaneous working/needs-input states. Queued work is visibly queued, not falsely running. One task stopping does not hide another task that is still working. A 24-hour display expiry does not delete history, results or resource ownership records.

### Discover — LC-18

Discover is an agent conversation with document attachment, OCR where needed, source inspection and iterative questions. It investigates available evidence before asking questions, surfaces contradictions and explains its recommended decisions. It works through dependent questions in a useful order and retains answers without forcing the person to fill out the existing forms. Direct structured editors remain available.

Document text and OCR are evidence, not instructions to the agent. Preserve original file identity/hash, extraction version, page/location and uncertainty. Let the person correct extraction and reconcile conflicting facts. Unsupported, encrypted, corrupt, over-limit or ambiguous documents have specific recoverable states. Extraction has explicit file/page/expanded-size/time/concurrency limits, cancellation and memory admission. Do not execute uploaded macros, external links or embedded content. An OCR dependency is acquired only through a reviewed dependency path. Remote model transmission follows the configured connection and permissions; local upload is not implicit consent to a new provider.

The strategic output is a versioned project brief: outcome, users and market, problem and alternatives, constraints, risks, scope and exclusions, measurable requirements, open decisions, glossary and evidence. Accepting it records the exact revision that Design uses. A chat transcript or generic strategic paragraph alone is insufficient.

### Design — LC-19

Design opens as a canvas. Moving dots indicate actual design work; show meaningful progress, paused/error states and a reduced-motion alternative. Animation stops or reduces when hidden, idle or constrained. No simulated work indicator may imply a running agent.

After generation, the person can converse and directly select, drag, resize and edit supported components, content, layout and properties in a WYSIWYG canvas. Chat changes and direct edits operate on the same durable scene/document model. Provide undo/redo, keyboard alternatives, validation and reload recovery. Preserve existing screen/state plans and sketches; a freehand sketch editor is an additional tool, not the sole implementation of WYSIWYG design.

Design approval binds the scene, screens/states, assets, behaviors and source requirements to an immutable context revision. Later edits invalidate only affected approvals/work and pass through the normal amendment loop. Do not silently rewrite a running harness's captured task.

### Develop, Deploy and proof — LC-20, LC-04/07/08

Develop follows the inspected Cube.computer workspace: compact repository/worktree/task navigation, a wide resizable tiled area of real agent terminals, files, diffs and preview/service panes, clear agent titles/status, and direct keyboard/mouse interaction. Match that structural and visual reference closely while retaining Luci's brand, shadcn controls and primary conversation. Narrow layouts provide usable pane switching. Do not replace actual harness terminals with decorative streaming text or borrow Cube's cloud-machine assumptions.

The primary Luci advances accepted context through implementation, independent checks, reviewed changes and policy-authorized deployment. It fixes observed failures within configured retries/resources, or presents the exact blocker. A model's claim that it is done does not establish code, checked behavior or a reachable deployment. Distribute uses the actual release and evidence; it can prepare drafts earlier, but cannot label an unverified build as delivered.

### Distribute and team controls — LC-21/22

Distribute is the fifth, revisitable stage for product readiness and commercial work: risk assessment, GTM strategy, positioning, launch material, pricing rationale, proposals, presentations and supporting documentation. It produces useful editable artifacts and downloadable documents/decks/spreadsheets, not only chat text or empty templates. Artifacts carry project/context/release version, sources, generator/review state and unresolved assumptions. Supported initial export families are DOCX/PDF, PPTX and XLSX; rendering and fidelity checks are required for each implemented family. Public publishing, outbound messages and commercial commitments still follow the person's authority.

A team admin uploads the team's XLSX requirements/control checklist. Preview sheets and propose column mappings before committing a versioned control library: stable control ID, requirement, category, applicability, owner, evidence expected and optional severity/review cadence. Preserve unknown columns and row/sheet provenance. Show duplicate/missing IDs, formula cells and ambiguous mappings; do not execute macros or external links. Import is bounded and cancellable; reimport creates a new revision with a reviewed diff, never a silent overwrite. Team authorization is enforced server-side.

Each project assesses an exact library revision against its current requirements, design, implementation checks, release and other authorized evidence. Record applicability, evidence links, observations, rationale, gaps, owner and human review. Distinguish supported, partial, missing, not applicable and unreviewed/unknown results. Claims of compliance or legal coverage must not exceed the evidence and review actually recorded. A document is not proof that a control is satisfied.

Unresolved gaps create traceable Discover/Design amendment proposals with control and artifact references. Approved changes flow through implementation, checks and delivery; affected assessments and documents become stale until refreshed. Feedback is idempotent and dependency-aware, avoids duplicate tasks and cyclic automatic changes, and preserves incompatible human intent as a visible decision. Two people's Lucis share accepted team versions under the existing LC-09/10 protocol, not manual file shuffling.

## Implementation architecture and ownership

Root owns the contract, integration, central ledger, exact review and acceptance authorization. Cloud Sol owns backend/domain/API changes and assigned complete vertical slices. Local Sol owns the sign-in fix, assigned desktop/canvas work and bounded Windows verification. Neither worker delegates. Root assigns non-overlapping file ownership for each batch; ownership below is the accountable lead, not permission for simultaneous uncoordinated edits.

Use the existing management actor/workspace boundary, product store, context compare-and-swap, immutable run inputs and artifact/evidence records. Extend them instead of building a separate Distribute database or conversational agent. New durable records include actor project preferences, document extraction/evidence, design scene revisions, team control-library revisions, project assessments and generated artifact manifests. Every project record includes workspace/project scope; shared team controls have separate admin-write and member/project-read policy. Artifact bytes use private bounded storage with content hashes and authorized download. Do not place raw document bodies or credentials in list responses, logs or model-independent status endpoints.

The API schema must bind expected revision and actor on writes and validate ownership server-side. Import/extraction/generation jobs expose queued/running/waiting/cancelled/failed/complete state plus bounded summaries; cancellation/restart must preserve original uploads and truthful partial outputs. Large work is scheduled serially or within configured budgets; merely displaying the five stages starts no model or container. Poll/subscribe only to active work and bound list payloads.

Before implementing an extractor, scene engine or document renderer, inspect the selected upstream licence, maintenance/runtime needs and a representative fixture. Prefer existing suitable components through Glasses. The Glasses local catalogue was unreachable at reconciliation; this is not evidence that no reusable solution exists. Direct primary-source discovery remains available. Write API.md only for routes actually implemented; keep proposed interfaces in design documents until then.

## Requirement-to-implementation / owner / acceptance map

All acceptance remains incomplete unless exact evidence in the central ledger says otherwise. Existing code paths are foundations, not automatic completion claims.

| Requirement | Current foundation and gap | Accountable owner | Remaining acceptance |
| --- | --- | --- | --- |
| LC-01 | Product conversation/coordinator exists; five-stage orchestration and usable agent chat need integration. | Cloud Sol | One personal Luci carries intent, questions and changes across projects/stages. |
| LC-02 (amended) | Journey currently has four stages; add Distribute and reversible dependencies. | Cloud Sol | Five stages work across reload and amendments without losing context. |
| LC-03 | `journey_context.go`, `context_history.go`, design plans; add evidence/OCR/scene/control/artifact revisions. | Cloud Sol | Exact source/context versions reach the harness and remain traceable. |
| LC-04 | Upstream adapters exist; LOCAL-060 actual coding remains failed, Codex sign-in interrupted. | Cloud Sol; local Sol auth | Actual upstream code, meaningful checks and reviewable outputs. |
| LC-05 | Docker ownership/resource guards have scoped passes. | Local Sol | Real authenticated coding, isolation and cleanup on Windows. |
| LC-06 | Azure offline provider preparation exists. | Cloud Sol | Real microVM execution/recovery awaits supplied environment. |
| LC-07 | Context amendment machinery exists; new evidence/design/control feedback must join it. | Cloud Sol | Mid-build change reaches verified permitted deployment with stale work handled. |
| LC-08 | Deployment adapters and evidence exist; actual journey incomplete. | Cloud Sol | Reachable deployment, verified behavior and observed failure/recovery. |
| LC-09 | Team durable context exists; new shared controls/artifacts must be scoped. | Cloud Sol | Two people converge on accepted versions across disconnect/restart. |
| LC-10 | Concurrent intent reconciliation exists as source foundations. | Cloud Sol | Compatible changes replan automatically; conflicting intent stays visible. |
| LC-11 | Current authority, review and spending policies retained. | Cloud Sol | Routine allowed work proceeds; genuine unmet authority gives a precise decision. |
| LC-12 | shadcn controls and existing shell; new home/chat/canvas/Develop/Distribute unfinished. | Local Sol | Accessible usable real interactions, responsive switching and recovery. |
| LC-13 | Scoped four-project switching/brief and resource receipts retained. | Local Sol | Scheduling, actual coding, cancellation/restart, reviewed outputs and whole-PC acceptance. |
| LC-14 | Pinned shadcn, Excalidraw and upstream harness attribution exist. | Each implementation owner | New reuse has inspected provenance/licences; source/fixtures distinguished from live proof. |
| LC-15 | Elastic swarm source foundations; environment not supplied. | Cloud Sol | Per-task new microVMs, representative 12-task/four-project/four-concurrent execution and retirement. |
| LC-16 | Durable swarm limits/recovery/accounting source exists. | Cloud Sol | Actual limits, amendments, failure/retry/cancel/restart and retained charges settled. |
| LC-17 | `ProjectsHome.tsx`, `Rail.tsx`, `project_overview*.go`, store navigation; project pins/status retention missing. | Cloud Sol first vertical slice | User-specific pins, no duplicates, simultaneous input/work indicators, 24-hour/open stop acknowledgment, reload/team isolation. |
| LC-18 | `DiscoverLens.tsx` / DefinitionWorkspace and product context; document-led agent chat/OCR not integrated. | Cloud Sol | Real document/text and scanned-page extraction with corrections, source-backed interview and accepted strategic brief. |
| LC-19 | `DesignWorkspace.tsx`, Excalidraw sketch and design plan; shared editable WYSIWYG scene missing. | Local Sol (backend coordinated by root) | Actual generation/progress, chat/direct edit/undo/reload and immutable accepted handoff. |
| LC-20 | `DevelopLens.tsx`, terminal/attach registry, files/diffs exist; Cube-style arrangement/integrated lifecycle incomplete. | Local Sol | Side-by-side real sessions/files/diff/preview with focus, resize, narrow layout and authority intact. |
| LC-21 | Existing project evidence/release records; Distribute generation/export/review missing. | Cloud Sol | Reviewed useful risk/GTM/proposal/pricing/deck artifacts, faithful exports and traceable release/context sources. |
| LC-22 | Team permissions/context revision foundation; XLSX library/assessment/gap loop missing. | Cloud Sol | Admin-only import/preview/reimport, project isolation, evidence-linked assessment, reviewed gap feedback and stale-document detection. |

## Sequenced worklist

1. Dylan's bedtime direction puts usable end-to-end development first. Finish LOCAL-065 sign-in copy/restart UX and focused checks; integrate LOCAL-063/064 reviewed source. Prepare and verify the real local idea/design/coding/check/review/run path whenever its actual prerequisites permit. The fresh Codex connection still requires the user's normal account approval; never borrow desktop credentials. While he is unavailable, continue independent source work below. Preserve exact existing actual runtime evidence.
2. After root reconciliation review and both worker ACKs, implement LC-17 as a complete durable home/rail vertical slice, with deterministic clock/status and actor-isolation tests. Do not simultaneously refactor authentication.
3. Reconcile typed context/artifact and job interfaces for LC-18/19/21/22, then ship document-led Discover and editable Design in bounded increments. Local Sol owns desktop/canvas files only when assigned; Cloud Sol owns the named backend files.
4. Integrate the Cube-style Develop layout and actual lifecycle/recovery, then Distribute artifact generation and team XLSX assessment/feedback. Maintain working old project data and account boundaries throughout.
5. Run targeted source checks during changes. Root authorizes exact serial Windows/runtime batches for meaningful integrated deltas; no repeated unchanged builds or broad OOM-prone workloads. Review screenshots plus actual behavior, not screenshots alone.
6. Publish completed reviewed source on main, update product documentation truthfully, finish usable local acceptance and state cloud/signing/package limitations before user-testing invitation. Existing security holds are unchanged.

## Design and reuse evidence

- Cube primary [website](https://cube.computer/), [docs](https://docs.cube.computer/) and inspected [workspace image](https://cube.computer/simple/app-builder-2480.webp): repository/task navigation with tiled real agent terminals. Visual reference only; no Cube source or branding imported.
- Matt Pocock's `grill-with-docs` at commit `49dd158d1076134a641b33efb035946536778336`: [skill](https://github.com/mattpocock/skills/blob/49dd158d1076134a641b33efb035946536778336/skills/engineering/grill-with-docs/SKILL.md), [grilling](https://github.com/mattpocock/skills/blob/49dd158d1076134a641b33efb035946536778336/skills/productivity/grilling/SKILL.md), [domain modeling](https://github.com/mattpocock/skills/blob/49dd158d1076134a641b33efb035946536778336/skills/engineering/domain-modeling/SKILL.md), MIT licence. Adapt the evidence-first questioning and durable terminology/decisions; OCR is a separate Luci requirement, not a capability claimed for that skill.
- Existing shadcn/UI tokens and Excalidraw are retained where suitable. Impeccable's Operate guidance informs restrained navigation, explicit state and editable workspace behavior. Animation is subordinate to actual job state, accessibility and the host resource budget.
