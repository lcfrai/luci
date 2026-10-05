# Full product implementation and acceptance

3 October 2026. The user's installed-app inspection supersedes the earlier narrow delivery checklist. The current application is not a complete UAT handback.

## Work in progress

1. Dedicated, resumable onboarding: account entry, execution location, actual prerequisite checks and automatic runtime preparation, provider connection and workspace ready. A configured directory does not imply a ready runtime. Local, user-owned AWS/Azure and existing workspaces each have their own recovery and permission states.
2. Projects home: create, import and restore independent of Settings, understandable repository setup and next actions. Appearance includes system/light/dark, accent, density and terminal text size.
3. Versioned specification: guided one-question interview; repository analysis; stable acceptance IDs; scope, roles, entities, integrations, terms, decisions and unresolved questions; explicit approved snapshots. Existing briefs and revision history are preserved.
4. Design workspace: requirement-linked screens/states and responsive targets, sketch and working prototype as distinct surfaces, assets/tokens and a recorded implementation handoff. A saved canvas does not imply an approved design.
5. Develop: account connection before project execution, durable runs, useful startup recovery, requirements/tasks/check evidence and live previews tied to source. AI assistance uses the person's actual selected provider; deterministic guidance is labelled honestly.
6. Application platform: inspect the newer private Panacea backend before adapting its services. Implement genuine local Docker application/data operations and an authenticated external Panacea connection without shipping restricted source in Luci's MIT package. Expose only operational capabilities; no simulated successful databases, sign-in, deployment or health.
7. Packaging and acceptance: rebuild matching source; install and complete realistic account/onboarding/project/restart flows; exercise each project phase with a Codex subscription in Docker; deploy a real site, verify its behavior and preserve evidence.

## Backend provenance

The separate checkout is `../panacea`. Default main is `1a76d094d9b50afe563377a29098bea4d290c715`; the newer `finish-dev-outstanding` candidate is `30580a7e34d16e9b04eeb31ac2c2f002e38d2bac`. Candidate selection is deliberate, and existing corporate estate migration scripts must not run. Panacea's local Kubernetes platform and application backing-services devstack are distinct. Its documentation acknowledges unexercised runtime paths; source/API tests alone do not prove a turnkey local deployment.

## Evidence rules

Every capability needs an actual outcome in the environment that is claimed. Record source revision, package provenance, execution location and external prerequisites. Preserve failed evidence. Verify the connected installed journey, meaningful failure/retry/restart cases and isolated-account boundaries. Use one batched visual inspection and one correction confirmation rather than repeated cosmetic reviews. Completion requires the full product journey and operating capabilities, not just the small public-site benchmark.

## 0.4.4 implementation checkpoint

The first corrective source batch adds Projects home, first-run workspace onboarding, appearance preferences, versioned requirements and design handoffs, project-independent Codex account sign-in, actual subscription-backed interview jobs, persistent Docker application deployments and PostgreSQL with downloadable backups. The Panacea adapter uses a separately supplied scoped HTTPS account connection; it contains no private Panacea source.

The complete control-plane Go suite passes. The broad frontend run found five appearance-rule failures among 1,026 checks; the corrected affected group subsequently passes. Focused new specification/design race checks and type checking pass. Browser entry, installed-upgrade, connected UAT, Docker and SSH acceptance scripts have been migrated, but their new packaged execution is still pending at this checkpoint.

This is an implementation checkpoint, not a full-product release claim. Host dependency installation, production cloud provisioning, complete authentication/storage/realtime services, backup restore and the real subscription-driven public deployment remain outstanding. PostgreSQL alone is not Supabase, and local loopback deployments are not public hosting.

## 0.4.5 integration checkpoint — 5 October 2026

The source now separates first-visit onboarding from workspace readiness, offers
local, AWS, Azure and existing-workspace entry paths, and connects prerequisite
checks to consented installer jobs. Cloud selection includes an optional preset
with enough CPU for the full data stack alongside an application and agent.
Provider prices still require a current account/region quote; no live AWS or Azure
provisioning is claimed by source or template checks.

The actual compiled 0.4.4 development candidate passed nine browser entry and
recovery checks, nineteen connected project/account checks, restart persistence,
and the batched desktop/mobile appearance inspection. Nine axe surfaces had no
violations; the canvas retained incomplete contrast checks. Evidence is under
`artifacts/uat/entry-frontend-5451563d-662a-47e6-bc2b-0ab29ddb1168`.
These receipts do not admit a new release or prove model inference.

The new optional data platform uses pinned upstream Supabase services and native
Luci controls for tables, Auth users, RLS, Storage, Realtime and the shipped
function. Real Docker acceptance verified eleven healthy services, user creation,
stored rows/policies, realtime publication, exact nested file bytes, function
execution and a downloaded backup with matching SHA-256. Those runs uncovered
and corrected cold-start health grace, normal tar directory handling and fresh
restore schema/ownership issues and storage tenant bindings. A fresh real Docker
run then passed twelve checks, including restored password sign-in, owner JWT
row access, anonymous denial, retained RLS/realtime, exact object bytes and real
stop/start persistence with browser record display. Its receipt is
`artifacts/data-uat-4fcf9300/results.json`; this used a host controller and does
not prove the complete Docker distribution. Custom function publishing,
SMTP/OAuth configuration and public data-domain provisioning are not implied by
the available controls.

The complete Docker distribution packages the controller, a private execution
engine, persistent workspace and provider tools. Browser previews and data APIs
use isolated local origins. The controller runs with the same non-root identity
as its private agent mounts. Docker install/restart and the subscription-backed
four-phase journey must still run against a matching clean-source package.
The broad frontend run passed 1,044 checks and identified two audit failures;
the token import and browser-gateway seam were corrected, and the affected
104-check audit group passed. The gateway preview URL correction also passed
27 deployment checks and type checking. These checks do not substitute for the
subscription-backed journey or real cloud-provider provisioning.

A matching clean-source Docker distribution subsequently started its non-root
controller and distinct private engine with no host bind mounts, created the
owner/Admin through real account entry, installed the agent runtime through the
UI, and saved/committed the benchmark's Describe, Design and implementation plan.
The instance is recorded under
`artifacts/subscription-e2e/docker-instance-602960d5-0755-433f-b269-fd0a67ebf7c9`.
This run exposed a cancelled automatic preparation handoff, stale ready-job UI
and a lost managed-worker engine setting. Those defects now have focused fixes
and passing regressions; the updated clean package must repeat their installed
outcomes. Official subscription sign-in remains a user step. No successful
inference, implementation or public site is claimed at this checkpoint.

The installed Docker candidate also passed 55 of 56 functional cloud-entry
checks: AWS/Azure selection and plan validation, cost acknowledgements and
invalidation, existing-server connection prerequisites and recovery. No account
sign-in or cloud resource creation was attempted. The failure was a bundled
Azure CLI readiness decoder rejecting its valid nested extensions object.
Fresh-restore browser addresses also need to remain stable for connected
applications, without routing to a replacement stack before healthy promotion.
These corrections require a new matching package and installed acceptance.

Cloud installation now keeps the executable ancestors traversable by the
system-service user while retaining private workspace/state permissions. Eleven
checks using the generated installer in an isolated Linux container verified
that correction; its executable and systemctl were explicit fixtures. Protected
cloud application/data browser transport uses the authenticated workspace
connection and separate local origins. New Docker deployments receive the
desktop's verified client data address while private server bindings stay on the
selected Docker network. Existing apps need redeployment after connecting or
reconnecting; restoring an active stack retains the data address. These transport
checks use real HTTP/WebSocket fixtures, not live AWS/Azure provisioning. Public
data-domain publication and automatic Coolify data bindings remain unimplemented.

Windows conversation storage now has compact physical names with unchanged
logical identities and retained legacy state. A real isolated managed-account
probe verified official Codex device sign-in stayed running and nested rollout
writes succeeded. This proves the storage fix, not completed account sign-in or
subscription inference. The user's provider sign-in, implementation, immutable
checks and working public deployment are still mandatory goal outcomes.
