# Luci 0.5.0 production continuation

The active goal is the complete product in [GOAL.md](GOAL.md), with measurable
release gates in [PRODUCTION-ROADMAP.md](PRODUCTION-ROADMAP.md). The owner selected
MIT self-hosting plus optional managed team workspaces, priced as a fixed Stripe
team subscription with model and infrastructure costs billed separately.

This branch starts from `690adbb1836269767b8f096274d2d57dbc340b38`, PR #5 and its
[original handoff](HANDOFF-2026-10-06.md). Draft PR #6 contains the production
continuation. The earlier production handoff at `51a1d8fc` describes the historical
coordinator-only checkpoint; its package results are not current release evidence.

## Current product behavior

The primary project overview explains the current stage, agreed purpose and
outcomes, exact tasks and reviewed revisions in an active build, unresolved
material decisions and one next step. It remains available across Define,
Design, Develop and Deploy. Discussion is optional; the user's priority is
understanding what is being built and how to change it.
Unfinished coding work remains visible independently of a newer prepared build.
Cancelling further work does not imply an existing agent stopped. Stored activity
is labelled as recorded until explicit reconciliation observes its outcome, and
each inspection action opens the selected run in the current account/project.
Large histories retain every unfinished identity and count while explicitly
omitting oversized details from the compact overview.

Provider-backed orchestration receives the saved context and bounded private
history through the selected existing provider. It cannot edit source or launch
implementation tools. Missing access and interrupted jobs remain visible and
recoverable without automatic inference. Consequential unanswered questions block
new coordinated implementation. Explicit reviewed amendments show the proposed
content and effects, check the original revisions, invalidate affected approvals
and preserve immutable historical run inputs. See [ORCHESTRATION.md](ORCHESTRATION.md).

Durable project runs reserve child identities before allocation, join actual
owned coding/check/release evidence, retain exact context/model/source inputs and
require independent verification. Retries and crash recovery cannot silently
repeat paid work. A successful process, a task checkbox or a saved deployment
status alone cannot complete a run. [PROJECT-RUN-CONTRACT.md](PROJECT-RUN-CONTRACT.md)
records the supported adapter and retention limits.

The managed foundation includes explicit teams and project permissions, separate
execution/secret/deployment grants, invitations and revocation, attributed context
changes, concurrent edit recovery and portable context format 2. Teams use an
explicit dedicated execution boundary. Operator admission does not establish
physical isolation. Public HTTPS ingress preserves authenticated browser grants
and requires separate app and preview sites.

Stripe uses the official SDK, a reviewed fixed recurring team price, durable
idempotent Checkout requests and signed webhooks followed by authoritative
subscription reconciliation. Redirects cannot grant service; manual suspension
remains independent. Cancellation retains data. Host backup/restore and explicit
owner-requested erasure have actual local acceptance. Commercial terms and privacy
text are drafts requiring the service operator's decisions. Read
[STRIPE-BILLING.md](STRIPE-BILLING.md),
[TEAM-SERVICE-OPERATIONS.md](TEAM-SERVICE-OPERATIONS.md) and
[MANAGED-DATA-LIFECYCLE.md](MANAGED-DATA-LIFECYCLE.md).

## Verification and evidence

The cloud environment has verified Go 1.26.3, Node 24.19.0, Docker 28.4.0 and system
Chromium 151. Tests use Playwright's supported executable override; they do not
claim its unavailable pinned Chromium build. Docker uses vfs and requires at
least 5 GiB free for one full sandbox; run expensive sandbox fixtures sequentially.

The managed foundation passed a complete 1,503-test frontend run and 1,100 passing
race test/subtest outcomes across seven integration packages (18 skips). The
new project-understanding batch passed 1,538 frontend tests in 108 files; the
sketch correction passed all 1,542 tests in the same 108 files. The final
active-work and exact-run navigation correction passed 1,562 frontend tests,
type checking, production build, and 1,967 Go test/subtest outcomes (22 skips). Its
broad Go run identified one newly registered read route absent from the
read-only audit; the audit was updated with the actual bounded read behavior and
passed. Final changed-package race results and installed candidate receipts are
recorded in [FULL-PRODUCT.md](FULL-PRODUCT.md).

Actual local acceptance includes two-browser collaboration and conflict recovery
(10 outcomes), portable team-worker lifecycle (17), invitation flows (5), browser
handoffs (11), local TLS ingress (11), real PostgreSQL restoration, managed service
admission and suspension, and host recovery/owner-requested erasure (7). The actual
reviewed team run passed all seven outcomes through the installed Codex CLI:
exact reviewed context, one reserved child, active stream revocation, independent
checks and a healthy release at the same commit. Its provider was a local
deterministic fixture. This proves execution and context delivery, not model
quality or paid provider availability.

Retained evidence is under `artifacts/onboarding`, `artifacts/uat`,
`artifacts/acceptance` and `artifacts/package-eval`. Each receipt binds its own
binary/source/image and scope. Private fixture directories can contain disposable
credentials; publish only reviewed summaries and release archives. Failed attempts
remain retained. Historical 0.4.18 packages must not be relabelled as 0.5.0.

GitHub's clean-source Go and race checks passed at `5d074fbc`; later candidate
runs found real sketch and strict Git metadata failures during expanded
acceptance. Each failed candidate and its own receipts remain retained. The
sketch fix prevents an unchanged Excalidraw callback from causing a render loop.
Controller Git commands suppress asynchronous automatic maintenance; raw native
test-fixture writers now do the same without weakening hostile-path tests.
[FULL-PRODUCT.md](FULL-PRODUCT.md) records native before/after tracing and the
latest candidate outcomes. GitHub's Actions artifact storage quota is a separate
external failure. Diagnostics upload after application checks; a quota failure
remains a failed workflow, not a waived release gate.

## Remaining real release gates

The full roadmap remains active. Live gateway/subscription inference, actual
AWS/Azure resources with acknowledged spending, owned public DNS/TLS, external
SMTP inbox delivery, merchant Stripe acceptance, dedicated paid-team host
isolation/capacity, signed native installations and exact public downloaded bytes
need their real accounts and environments. None was simulated into a pass.

No live charges, cloud provisioning, public release or public application was
created in this cloud session. Local reviewable release packages and deterministic
fixtures do not establish production service availability. The scoped GitHub
connection can push source; release uploads and CI raw-log hosts additionally need
the saved environment network additions. GitHub artifact quota needs an operator
resolution, not deletion of unrelated evidence by this task.

Use the existing checkout; do not create a worktree without a user request.
Activate `/workspace/.luci-tools/activate.sh` in every shell. Complete reusable
installation/start instructions live in the environment configuration draft.
Saved configuration is distinct from publication or a tested new-task restore.
There is no callable native goal-control API here; the goal and roadmap files
carry the continuing objective.
