# Continuing product-quality work

The active objective remains [GOAL.md](GOAL.md), judged against
[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md). This is an implementation worklist,
not a replacement completion standard or an automatic background scheduler.
The current cloud session exposes no persistent-goal scheduling API.

## Current findings and required outcomes

| Finding | User consequence | Required correction and evidence | Status |
| --- | --- | --- | --- |
| Restoring a stale structured draft replaces a teammate's independently changed fields. | Recovering one person's constraints can silently undo another person's agreed outcome. | Preserve the actual original baseline; combine independent definition/design/task edits; require explicit choices for overlapping edits or unknown legacy baselines. Verify two people through the actual UI, retaining old run scope while reviewing the new scope. | Implemented; merge and UI regressions pass. Two-person rendered acceptance pending. |
| Prototype review hides the screen's expected behavior and can reuse a consumed team browser invitation after navigation. | The person cannot compare the working result with the agreed experience or reliably return to it. | Keep routes, states and outcomes alongside the actual prototype; record source-bound human observations as unverified draft context; obtain a fresh explicit browser connection. | Focused checks passed; actual rendered prototype acceptance pending. |
| Deployment operations can say completed when a successful HTTP response carries a failed operation status. | The person receives a false success and no useful recovery path. | Display the actual operation outcome and remedy. Keep returning users connected to their retained release, source and health, with explicit supported cleanup/replacement controls. | Implemented; operation, permission and ownership checks pass. Rendered recovery pending. |
| An unsent discussion disappears on remount; an uncertain send can lose its request identity. | Context is lost or the same intent can become a second paid request after refresh. | Scoped private draft recovery and durable exact-request identity; no automatic resend. Resolve acceptance or authoritative non-dispatch without racing a delayed original request. | Implemented; frontend, admission, archive and race checks pass. Rendered fault-recovery proof pending. |
| Collaboration evidence stops before a complete amendment during active work. | Mechanism checks alone do not show people can understand and continue the combined workflow. | Two people preserve concurrent contributions, see old active scope versus new agreed context, renew review, build the amended behavior, retain a meaningful regression check, and inspect/accept the matching preview. | Driver prepared; matching-build execution pending. |

The current source batch passes 1,701 frontend tests across 117 files, type
checking and production build, 1,997 Go test/subtest outcomes (22 skips), and
1,170 race outcomes across the three changed integration packages (five skips).
These checks do not replace the pending rendered journeys or live model proof.

Current implementation owners coordinate shared files before editing. Each
correction needs its scoped regression and rendered behavior where applicable;
failures stay recorded. Earlier `340f1590` results retain their original identities.

## Gates that still need real external access

Live model reasoning and implementation, merchant commissioning, separately
billed model/infrastructure policy, owned managed/cloud hosts and domains, SMTP
delivery, signed distribution, and installed Windows diagnosis remain open.
GitHub currently rejects Actions runner admission for account payment/spending
reasons. No permission, credential, live result or production readiness is
inferred from elapsed time or from deterministic local fixtures.

After this worklist is verified, continue with the next unproven complete journey
and any independently actionable defect it exposes. Do not mark the overall goal
complete because these rows or a test suite pass.
