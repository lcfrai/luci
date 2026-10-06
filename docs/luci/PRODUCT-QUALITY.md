# Luci product-quality completion contract

The owner's latest direction makes exceptional product quality the finish line:
a polished, dependable experience comparable in clarity and care to Vercel,
ChatGPT and leading AI-era applications. This is part of [GOAL.md](GOAL.md), not
an optional visual pass after engineering. The current application has not
demonstrated this contract and must not be described as finished.

## What the person should experience

Luci helps someone express an idea, build useful shared context, understand the
proposed solution, make informed amendments and get a working application live.
It continues to support changes and operation after launch. Its assistance and
phase editors use one project understanding. The person should not have to
reconstruct the process from terminals, configuration forms or implementation
records. Technical controls remain available when the task needs them.

At each meaningful transition, the rendered product must answer:

1. What are we trying to achieve, and what has actually been agreed?
2. What is being built now, with which scope and decisions?
3. What has been observed to work, and what still needs verification?
4. What decision or action is needed next, and why?
5. How can I change the solution, and what happens to work already in progress?

Material ambiguity needs a specific question and an understandable consequence.
Assumptions and recommendations stay distinguishable from accepted decisions.
The assistant helps complete the context; it does not simply present an empty
technical specification for the person to fill unaided. Context refinement,
review and implementation must preserve continuity across sessions and teammates.

## Required complete journeys

These journeys complement the existing security and mechanism tests. Run them
through the actual supported UI using a matching installation. Record evaluator
familiarity, confusion, wrong turns, assistance needed and actual outcomes.
Instructions may describe the desired result; they must not supply hidden
selectors, API mutations or implementation knowledge needed to finish the task.

| Journey | Required outcome |
| --- | --- |
| Idea to live application | Start from an ordinary-language idea for an application with authentication, roles and persistent data. Clarify a consequential ambiguity, review the proposed behavior/design, let the real selected agent implement it, independently verify the agreed outcomes, and deploy a reachable application with the checked source identity. Exercise its principal user tasks and data persistence. |
| Change during development | Amend an agreed behavior while work is active. Understand what remains in the existing build, review downstream effects, preserve both contributors' work, renew necessary approvals and inspect the resulting implementation. |
| Existing application | Import an owned existing repository, discover and review its actual behavior and constraints, make a meaningful change, detect an introduced regression and recover without losing unrelated work. |
| Team handoff | A second authorized person can understand the idea, decisions, active work and evidence, contribute a revision, resolve a real concurrent edit and continue. Revoked or insufficient access has a clear explanation and safe recovery path. |
| Operate and recover | Inspect the running application, diagnose an unhealthy or failed deployment, perform supported rollback or repair, recover backed-up data, and verify the public outcome again. A returning person can identify the real current state after restart or lost connectivity. |

A fixture provider delivering a prewritten application or test proves scoped
execution plumbing. It cannot pass idea-to-application reasoning or implementation
quality. API/file seeding can isolate an interaction test, but cannot replace the
user steps being claimed by a complete-journey result. Record such shortcuts.
Never let the verifier silently implement the benchmark on the agent's behalf.

## Quality gates

| Dimension | Completion evidence |
| --- | --- |
| Understanding | An independent walkthrough can answer the five questions above from the current screen. Active scope, amended context, completed work and unverified outcomes do not contradict each other. Useful summaries precede technical details. |
| Continuous workflow | The next action reaches the relevant project, build, check, preview or release with the necessary context preserved. The person does not need to discover a second unrelated workflow to finish the stated task. |
| Interaction and visual quality | Consistent hierarchy, language, spacing, typography, controls and responsive behavior across supported surfaces. Loading, empty, success, permission and failure states are designed. Primary actions are discoverable; dense configuration has a purposeful order and progressive disclosure. |
| Editing and recovery | Drafts, selections and useful progress survive supported navigation and restart. Unsaved changes, stale revisions, failed requests and retry effects are clear. No ordinary recovery depends on editing internal files or guessing whether paid work already ran. |
| Honest verification | Command success, task completion, requirement coverage, preview health and production health remain distinct. The product explains missing evidence before inviting publication. Source or context changes invalidate the appropriate evidence visibly. |
| Responsiveness | Record action acknowledgement, navigation, heavy-editor loading and long-job progress on representative hardware. Ordinary actions should visibly acknowledge within 100 ms; target warm navigation p95 within 500 ms. Longer operations need immediate visible progress, a bounded failure path and cancellation where supported. These are targets to measure, not existing performance claims. |
| Accessibility | Verify keyboard navigation, focus, labels, contrast, zoom/reflow and error recovery in real rendered journeys, with WCAG 2.2 AA as the target. Resolve automated findings and inspect incomplete/manual items. Passing an automated scan alone does not close this gate. |
| Trust and operation | Real permissions, provider/billing selection, cost acknowledgement, tenant isolation, data recovery, service health and distribution match the release claims. The managed service also meets the commercial and operational gates in the roadmap. |

No known data-loss, security, false-success, broken-core-task or blocking
comprehension defect may be deferred into a claim of production readiness.
Repeated confusion and workaround-dependent core journeys require implementation
changes, even when an automation script can eventually finish them. A polished
error state preserves the work, explains what happened and offers a useful next
action; expected provider or network failures need not become dead ends.

## How work reaches completion

Maintain concrete findings, with the affected journey, reproducible actions,
observed user-visible result, consequence, owner, correction and verification.
Prioritize work that prevents understanding or completing the solution, then
repair inconsistent interaction and presentation. Use one bounded assessment
per changed journey; do not substitute repeated cosmetic review for fixes.

Track implementation, product-quality acceptance and external commissioning
separately. Existing unit, integration, fixture and package receipts retain their
original meaning and identities. Record live provider, cloud, domain, inbox,
billing and signed-download outcomes only when they actually occur.

The goal can close only when representative complete journeys and these quality
gates have adequate current evidence, serious findings are resolved, and the
supported open-source/paid release gates are met. Preserve any remaining minor
limitations explicitly. Missing credentials or CI admission leave the affected
gate open while independent product work continues. Do not replace the goal with
another launch check, test-count milestone or percentage-complete claim.
