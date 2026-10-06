# Project understanding and reviewed amendments

The primary product outcome is an understandable project: its agreed purpose,
requirements, current stage, exact selected build scope, next step and a clear
way to change direction. Optional discussion supports that outcome. A long-lived
chat is neither the project specification nor proof that the application works.

## Current understanding

`GET /v1/product/projects/{id}/orchestrator` reads the current saved brief,
specification, design plan and task plan, bounded source observations, and
recorded check/release summaries. The current run's tasks, requirement acceptance
criteria, specification/design revisions and requested outcome come from its
immutable saved input. Editing today's plan does not rewrite what an existing
agent was authorized to build. A cached accepted run does not establish public
availability or current health; release inspection and reconciliation remain
explicit actions.

The latest prepared/recorded run and unfinished coding work are separate views.
`currentRun` retains the newest run; `activeRuns` retains every run with a stored
running, dispatching, unknown or reserved coding operation, including a cancelled
parent. Preparing a newer build cannot replace an older agent's immutable scope.
Cancelling further work prevents future dispatch; stopping an existing agent is
an explicit session action. Only explicit reconciliation records a terminal child
outcome and removes its activity from this view. GET does not contact the runtime
or turn a stored running status into a claim about current liveness.

Run details share a 64 KiB budget, including arbitrary reasons and operation
lists. When a row exceeds that budget, the view retains its identity, recorded
activity, cancellation state and task/requirement/operation counts, with explicit
omission flags. Opening that exact run retrieves its full saved scope. It never
substitutes today's task plan or silently drops an older unfinished run. Current
account/workspace/project scope also binds navigation to a selected run.

`context.digest` binds the current saved context documents and sketch. It is the
amendment compare-and-swap token, not a continuously refreshed fingerprint of
processes, Git status, release health or conversation history. Each assistance
turn also retains the complete bounded observation snapshot actually supplied
to the selected provider. Its conversation revision is a separate concurrency
token. Reads do not persist observations or repair storage.

The common project panel stays available across Define, Design, Develop and
Deploy. Stage-specific editors remain direct, reviewable ways to amend the same
context. The panel shows the exact existing run scope and navigates to existing
review, execution and release controls; a model recommendation does not silently
invoke those controls.

Manual definition questions have a direct, stable-ID route to their existing
Answer field. Navigation respects unfinished drafts and cancelled navigation
leaves no hidden target for a later visit. Saved answers remain available in the
shared overview, explicitly labelled as needing definition review or belonging
to the reviewed revision. Reading or navigating never saves or approves an answer.

The implementation-task next action opens the existing plan disclosure and
focuses its review region, including from an already-open Develop stage. This
read destination remains available to authorized viewers. Guarded cancellation,
a changed scope or a newer focus choice cannot leave a delayed task destination
that interrupts other work. Develop summarizes the structured design handoff and
its actual review dependencies; an optional sketch is a separate artifact.

Prepared builds separately project changes to reviewed context and unresolved
private conversation decisions. The latter opens the existing current question
through scoped navigation and focuses its answer control after a fresh read;
it does not alter the composer, recover a request or send a message. Earlier run
inputs, acceptance and active coding sessions remain inspectable.

Verification setup retains commands and exact unfinished arguments as a bounded
browser draft in the account/workspace/project scope. Explicit restoration
revalidates its build reference and criteria against the current reviewed context
and source. It never restores command trust or host-execution mode. Commands
already recorded by an explicit check request can remain a reusable local
template without repeated navigation warnings; unfinished and later edits stay
protected. This local setup is not shared context or passing check evidence.
It therefore does not block an explicit assistance request. The composer makes
clear that local command text is not included automatically; the person can paste
relevant commands or unfinished arguments into the message. Sending a message
does not execute them. Unsaved shared context still blocks Send, and all unsaved
work retains navigation, branch-change and amendment-application protection.
When there are no coding panes, planning and verification use one continuous
scroll area instead of reserving space for an empty terminal.

## Provider-backed assistance

`POST /v1/product/projects/{id}/assist/orchestrator` accepts the expected
conversation revision, expected context digest, a browser-generated idempotency
key, the message, an optional exact `replyToTurnId`, and the selected provider
route. An authenticated actor and immutable context are durably reserved before
inference. Identical retries join the original job; changed key payloads conflict.
A missing or failed provider is a real failure, without a fabricated answer.

The browser retains unsent discussion text and its reply target in the same
account/workspace/project scope; restoring it is explicit. These drafts expire
after 30 days. Before sending, it durably records the complete original request
and idempotency key in a separate immutable slot. Local recovery is bounded to
128 records, 128 KiB per record and 2 MiB total. Storage failure blocks dispatch
with a remedy. Unresolved request identities do not expire into fresh messages.
Reload and conversation refresh never silently generate a new key for an
uncertain send, and no recovery operation sends automatically.

The person can explicitly retry the original request with its original body and
key, or check its outcome through `POST .../orchestrator/resolve`. The latter
requires readable project access, is bound to the authenticated actor, and never
leases credentials, repairs a job or dispatches inference. It returns an exact
accepted receipt, a durable not-sent result, or continued uncertainty. A not-sent
result first fences that key against delayed original requests; a bare absent
lookup would not be safe. Only then can the browser recover that message for
editing. A reader without execution permission can resolve their own old request
but cannot retry or start a new one. Loss of read access hides local recovery.
See [API.md](API.md#recovering-an-uncertain-orchestrator-request) for the receipt,
archive and admission contract.

No-dispatch fences are private hash-only records, bounded to 1,024 per project,
and remain in the active ledger through every history rotation without expiry.
Capacity or incomplete/corrupt history returns continued uncertainty rather
than silently reopening an old request key.

The selected existing assistance runtime receives saved context and a bounded
recent discussion excerpt. Older proposal payloads are omitted from the excerpt;
the current context remains authoritative. A specifically answered question is
included even when older than the normal excerpt. Assistance runs read-only and
has no authority to edit project documents, approve them, execute implementation
or deploy an application.

Existing assistance job read/cancel endpoints remain the lifecycle interface.
`POST /v1/product/projects/{id}/assist/{jobId}/reconcile` repairs a durable
terminal-job receipt into its reserved discussion turn after interrupted storage.
It does not rerun inference. Startup similarly reconciles pending ownership;
a reservation without a durable job becomes a failed outcome, never automatic
inference replay.

A result explains its summary, consequential question/options, rationale,
impacts, proposed amendment and suggested next action. Successful explicit replies
consume the exact parent question; a follow-up material question replaces that
block. Failed or cancelled replies do not clear it. Applying a proposed amendment
alone does not answer a material question.

Unresolved material questions block new coordinated runs, reservations and
reviewed-task creation/resume/fork/restart, including dispatch of a previously
reserved operation. Already allocated child receipt recovery, reads, cancellation,
cleanup and independent manual terminals remain available. An answer clears the
question only: changed context still requires current private approvals and a
new matching task selection. Publishing a question takes exclusive source
admission so it cannot race the final allocation of a newly coordinated session.

## Explicit reviewed amendments

`POST /v1/product/projects/{id}/orchestrator/turns/{turnId}/apply` accepts the
expected conversation revision and context digest. It uses only the previously
validated server-stored proposal. The user reviews the original and proposed
specification, design plan or task plan before confirming. Unsaved local editor
work must be saved or discarded first. A teammate's intervening edit conflicts
without silently retrying or overwriting the newer context.

One application creates one versioned draft. A specification amendment updates
the brief and revokes affected private reviews; a design amendment revokes its
design review. Linked task plans bind actual current reviewed requirements.
An assistant cannot approve a document, fake task completion, select an arbitrary
prototype receipt, or rewrite earlier run inputs/evidence. The authenticated
person applying the change is recorded in private context attribution.

A private write-ahead amendment receipt allows recovery when canonical context
was saved but a generated export failed. Retrying observes the exact persisted
target and finalizes the receipt without incrementing its revision twice.
Generated exports still use the separate explicit Context storage repair flow.

## Private bounded history and recovery

Discussion belongs to the private account/project/path scope and is excluded
from portable project context and repository exports. Copying a private ledger
into another scope cannot grant access. It contains people, messages and saved
context; downloads must be treated as private data.

The current ledger supports at most 200 turns and 16 MiB. The selected context is
bounded to 224 KiB and space for a bounded terminal answer is reserved before
provider work begins. At most 16 unresolved material questions may accumulate;
further requests must explicitly answer an existing question. Capacity errors
preserve accepted history and link the supported recovery path.

In **Conversation history and storage**, the user downloads the complete retained
history, confirms the file was saved, and explicitly frees conversation space:

- `GET /v1/product/projects/{id}/orchestrator/export` is a read-only JSON download
  containing the current ledger and all verified private archives.
- `POST /v1/product/projects/{id}/orchestrator/rotate` requires the current
  conversation revision, the exact exported `storageDigest`, and
  `acknowledgeExportedHistory: true`.
- Rotation retains the latest eight turns, unresolved material questions and
  pending amendment receipts. It retains one full previous ledger as a private
  archive; the next acknowledged rotation replaces that archive. The download
  contains it before replacement. Project documents and approvals do not change.
- Active provider ownership cannot be rotated away. Revisions and turn sequences
  remain monotonic, so an old discarded request with its original revision cannot
  allocate work after rotation. Only current retained keys can join their prior
  result directly; older history remains in the acknowledged download/archive.
- An interrupted pointer change may leave two verified archives. Both enter the
  next download and storage token. A bounded uncommitted temporary write is also
  included in the token by name, length and digest; its accepted source remains
  the unchanged current ledger. Explicit rotation removes it after acknowledgement.
  A stale download cannot authorize cleanup of a later archive or temporary file.

Normal accepted storage is at most 32 MiB per project (current plus one archive).
Crash recovery allows a second 16 MiB archive and one bounded 16 MiB temporary
write, then returns to the normal bound on explicit cleanup. Corrupt or foreign
archives fail closed without silent deletion. Downloads use the existing team
response stream and membership-revocation registration, not the 8 MiB catalogue
or session projection buffer.

## Validation scope

Source regressions cover authenticated identity, duplicate admission, private
scope rejection, read-only restart, follow-up question resolution, all three
amendment targets, stale human edits, no invented approval/completion, interrupted
canonical/export persistence, bounded export/rotation, crash inventory and
active ownership. Separate API/provider, team authorization, UI and compiled
browser acceptance bind these product contracts to actual entry points. These
checks do not establish paid model quality, public DNS/TLS operation, cloud
creation or a publicly shipped application.
