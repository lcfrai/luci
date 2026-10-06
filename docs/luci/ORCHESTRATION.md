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

## Provider-backed assistance

`POST /v1/product/projects/{id}/assist/orchestrator` accepts the expected
conversation revision, expected context digest, a browser-generated idempotency
key, the message, an optional exact `replyToTurnId`, and the selected provider
route. An authenticated actor and immutable context are durably reserved before
inference. Identical retries join the original job; changed key payloads conflict.
A missing or failed provider is a real failure, without a fabricated answer.

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
