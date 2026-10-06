# Production foundation continuation

This work starts from `690adbb1836269767b8f096274d2d57dbc340b38` on
`codex/luci-subscription-e2e` (PR #5), including its
[handoff](HANDOFF-2026-10-06.md). The confirmed delivery model is self-hosted
MIT open source plus optional lcfr.ai-managed team workspaces. The continuing
goal and measurable stages are in [GOAL.md](GOAL.md) and
[PRODUCTION-ROADMAP.md](PRODUCTION-ROADMAP.md). This is a production foundation
change, not authorization or evidence for a public release or paid launch.

## Implemented behavior

The reviewed implementation plan can create a durable project run without
starting a session. A separate action reserves and starts coding through the
existing session API. Run inputs retain exact private review selections,
requirements, worktree and model route; each numbered attempt records its actual
starting source. Independent sessions remain available.

The controller persists an operation and its child identity before allocation.
Concurrent and repeated requests recover the same child. Interrupted allocation
stays unknown when its outcome cannot be established; reconciliation never
replays it. Reserved but undispatched work can be continued explicitly. A new
attempt follows a known terminal outcome. Run-bound resume/fork is intentionally
not supported in this first adapter: reserve a new clean attempt instead.

Checks and releases are joined by owned identity, with actual command results,
complete selected requirement coverage, exact clean candidate source, private
review authority and fresh release ownership/health observations. New release
admission records immutable provenance before side effects. Older releases
without that provenance remain historical. Pruning ordinary check history
does not erase the run's bounded evidence snapshot.

Explicit acceptance requires matching current evidence. A CLI exit, completed
task or saved deployment status cannot complete a run. Later source, review,
connection or health changes invalidate the current observation while preserving
historical acceptance. Reads and service restart do not dispatch or repair work.

Related reliability changes make account mutations transactional across failed
state writes, defer connection revocation until a successful commit, constrain
project Git metadata and environment to owned repositories, cap sandbox process
counts at 256, and make the image entrypoint explicitly readable/executable by
its non-root user.

## Verification and its limits

The cloud workspace uses Go 1.26.3, Node 24.19.0, Docker 28.4.0 and system
Chromium 151 through Playwright's supported executable override. The installed
Playwright package's pinned browser could not be downloaded under the current
network policy. That exact browser and native Windows/macOS installers remain
separate gates.

- The broad Go run exercised all control-plane and sandbox packages. Its one
  failure was the registry reference-field audit missing the already-cloned
  implementation context; the audit was corrected. The final four changed
  integration packages passed 923 tests/subtests, with four skips.
- The complete frontend run initially passed 1,341 of 1,344 tests. The exact
  DTO audit, CSS token import audit and a nondeterministic service-down fixture
  were corrected; all affected files passed afterward. Typecheck and production
  build passed.
- Focused race tests cover run storage/HTTP, concurrent gateway/subscription
  dispatch, interrupted allocation, exact source validation, evidence/native
  release adapters and management persistence. Full Go vet passed.
- The compiled management service smoke passed all seven outcomes.
- The real browser journey passed authenticated review, named route selection,
  create/read/cancel, process restart persistence, and backup restoration
  without inherited private run history. Accessibility and account-entry
  assertions remained enabled.

Local evidence is under `artifacts/onboarding`,
`artifacts/uat/coordinator-current/evidence`, and `artifacts/acceptance`.
These ignored directories can include private disposable fixture state; publish
only reviewed evidence files, never entire fixture trees. Failed attempts remain
retained rather than overwritten with a pass. Runtime and package results must
be read from their own final manifests; source tests do not establish those
outcomes.

Provider fixtures exercise actual supported CLIs and Docker plumbing against
local deterministic providers. They do not establish paid provider availability,
live subscription inference, or that a real agent produced the benchmark app.
No cloud resources, paid inference, public application or release were created.

## Continuation requirements

The current run adapter retains the reviewed main-checkout restriction. Saved
versioned model connections or a retained Codex subscription generation are
required; unversioned legacy API credentials cannot identify a durable billing
route. Storage fails explicitly at 200 runs/project, 100 operations/run or
1 MiB/record instead of silently pruning evidence. Export/retention management
and run-bound conversation continuation need their own future contracts.

Real team membership, project permissions, shared editing/conflict resolution,
durable draft recovery and managed-team operations remain subsequent stages.
Invitations to private per-user workers do not implement collaboration or prove
hostile tenant isolation. Start the managed pilot with dedicated team execution
boundaries as described in the roadmap.

Live model acceptance needs the intended gateway URL/model or an actual connected
subscription. Cloud/domain/TLS, SMTP delivery, signed downloaded installations,
billing and operational recovery need their own real environment and evidence.
These prerequisites do not convert untested release gates into passes. The goal
remains open; this session did not create a native background goal because that
API is not exposed here.
