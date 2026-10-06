# Durable project runs

Implementation contract for the next batch after the private review-authority
fix. This is proposed behavior, not a claim that project runs are shipped or
that installed acceptance has passed. Existing independent sessions, checks and
releases remain supported.

## Outcome and scope

From a saved implementation plan, a person can start a run, return after a
restart, and see its reviewed inputs, actual coding session, selected check
evidence and exact release together. The next action is explicit. A session's
exit, an agent's statement, a task marked done, or a saved release status never
proves the application works.

Start with a private durable record and reconciliation over existing operations.
Do not add an autonomous executor, another daemon, a required model brand,
automatic paid inference, or automatic publication. A model may suggest work;
typed server operations and independent evidence determine state.

## Existing seams to reuse

| Existing code/API | Role in the run |
| --- | --- |
| `product.ImplementationContextRequest`, `BindImplementationContext`, `ValidateImplementationContext` | Resolve saved task IDs; bind actual brief, plan, approved specification/design and starting source. Retain the current main-checkout restriction. |
| `registry.ImplementationContext`, `registry.Session`, `api.sessionCreateRequest`, `POST /v1/sessions` | Persist the coding child before allocating its sandbox. Preserve normal connection, quota, permission, Git admission and sandbox checks. |
| `product.ResolveAgentConnection` and the session's `AgentConnectionID/Revision/Kind/Protocol`, model and harness | Resolve an explicit route once, then reject disabled, changed or incompatible selections. Never fall back to another model or billing account. |
| `product.ProjectCheckRun`, `POST /v1/product/projects/{id}/checks`, `readProjectEvidence` / `evidenceRunView` | Execute user-reviewed commands; retain actual command outcomes, requirement snapshots and source digests. |
| `product.Release`, `storedRelease`, `CloudRelease`, `pagesRelease`; existing local/cloud/Pages release endpoints | Retain the exact built commit and target; use existing ownership/health observation and refresh paths. Native platform deployment must use the same run-binding validator. |
| `product.readApprovedJourney`, `specificationApprovalMatches`, `designApprovalMatches` | Read private approval authority, including full dependency content. Project JSON and exported hashes are never authority. |
| `product.workspace.go` forwarding and management-owned worker state | Route run APIs and children to the selected workspace and account together. A local run cannot attach a remote or foreign-account child. |

Use typed internal functions behind these handlers rather than invoking Luci's
own HTTP endpoints or copying admission logic into a second implementation.
Product must not depend on API: inject bounded registry lookup/observation
callbacks, or place the narrow record types in an independent package.

## Private record and approval interface

Store records under the worker's private `Server.stateDir`, outside every project
and coding mount, with the existing link/path guards and restrictive permissions.
Resolve project/worktree ownership server-side. Client paths, actor IDs, digests,
completion flags and receipt bodies are not accepted. A current worker/account
scope and canonical project scope prevent copied records from becoming valid.

The narrow future helper `ApprovalSelection(projectID)` returns, only after
private verification:

```text
specificationApprovalId, specificationRevision, specificationDigest,
designApprovalId, designRevision, designDigest, briefDigest, sketchDigest
```

Recovery reviewed these names against `journeyApprovalReceipt`. Receipt IDs are
opaque selection identifiers, not bearer credentials. Keep them in the private
run and controlled API projection; never write private receipts, actor metadata,
account paths or authentication generations into the project or agent prompt.
Keep historical approvals separate from the current selection. A fresh approval
or revocation makes an old run's inputs stale even if text appears identical.

Each run stores: version, ID, project/worktree, revision, creation/update times,
immutable `ImplementationContext` and approval selection, resolved public model
route receipt, explicit intended outcome (`preview` or `publish`), selected
requirement snapshots, operation reservations, child identities, evidence
snapshots and the latest explicitly persisted observation. Preserve the private
Codex connection generation when needed for restart validation; never expose it
through the public DTO. Store no provider keys, attach tokens, environment maps,
transcripts or raw command output in the run record.

## Create before allocating a child

Proposed endpoints under `/v1/product/projects/{id}/runs`:

- `POST /runs`: client supplies a fresh idempotency key, expected reviewed
  revisions/task IDs, worktree, harness/connection choice and intended outcome.
  Server resolves and validates all immutable inputs, then atomically persists
  the run and key/request digest. No session or model call starts here. Same key
  plus same normalized request returns the same run; different content is `409`.
- `GET /runs` and `GET /runs/{runId}`: bounded, side-effect-free projections.
  They may report current observations but never persist, dispatch or repair.
- `POST /runs/{runId}/operations`: reserve a specific explicit operation with
  `expectedRevision`, idempotency key and kind. Persist its immutable request
  digest and child ID before handing the reservation to the existing executor.
- `POST /runs/{runId}/reconcile`: explicitly reconcile private child records and
  persist an observation. It never allocates/replays a missing operation.
- `POST /runs/{runId}/checks` and `/releases`: explicitly join existing owned
  evidence by ID, subject to the validations below; they do not execute it.
- `POST /runs/{runId}/cancel`: stops future dispatch and records cancellation.
  Stopping a running child uses its existing explicit cancel/stop control.

Extend session creation with `projectRunId` and `projectRunOperationId`. Reserve
a server-minted session ID in the run before the existing `Store.Create`; then
persist both run IDs in `registry.Session` **before** `backend.Create`. The
session request must match the reserved immutable implementation/connection
selection. Independent sessions omit both fields. Unknown/partial pairs fail.

Serialize reservation/admission under the same per-account run lock; persist
deduplication and reservation together. Revalidate approvals, connection and
worktree immediately before allocation. A second request finds the reserved
registry child; it cannot allocate another. An explicit retry after a terminal
failure creates a new numbered attempt while retaining the earlier one.

Crash after `Store.Create` but before run attachment is recovered by scanning the
actual private registry for the exact project/run/operation IDs and reserved
child ID, then observing the existing backend through normal lifecycle logic.
Zero matches means pending/unknown, not permission to replay; multiple matches,
wrong ownership or conflicting identities block with an actionable diagnostic.
Apply the same reservation-before-side-effect principle to any later integrated
check/release dispatch. Do not hide an unkeyed HTTP call behind a run operation.

## Explicit evidence joins

At check join, load the private saved `ProjectCheckRun`, not a client outcome.
Require the same project, exact current approved selections and requirement
snapshots, successful recorded commands, `SourceChanged == false`, and current
source/commit equality through `evidenceRunView`. Run-selected requirement
coverage must be complete; unlinked commands and task status do not count. The
starting source may differ after legitimate coding: record the checked candidate
separately rather than rewriting the initial snapshot. Re-check after commit if
the check was run against a different or dirty source.

Persist a bounded copy of the accepted check provenance and command outcomes
without raw output, plus the original check ID. Check history currently retains
only 20 runs; pruning that list must not silently erase run provenance or turn a
missing row into success. Keep a durable reference/pinned evidence policy or the
immutable bounded snapshot, and label whether the original is still available.

At release join, independently resolve the existing release through its typed
local/cloud/Pages/native adapter. Its clean source commit and source snapshot
must equal the checked candidate, its project and selected target must match,
and its approved selection must still be current. Add the source digest and
review selection to new private release records at admission; do not backfill
old rows from current state or infer provenance from a URL. Unknown old rows
remain historical, unverified evidence for a new run.

Require fresh supported ownership/health observation for the selected release;
retain observation time and limitations. Public reachability, inbox delivery
and provider-specific behavior require their own actual evidence. Local preview
is never described as public deployment. An explicit final user acceptance can
record that selected behavior was reviewed, but it cannot manufacture failed or
missing technical evidence.

## State, restart and a calm UI

Use a small state set: `ready`, `coding`, `awaiting-checks`, `awaiting-release`,
`blocked`, `cancelled`, `complete`. Accompany it with a precise reason and current
child observation. `complete` means the chosen run outcome has matching required
evidence and explicit acceptance; it is not a claim that every product goal or
every possible requirement is proved. Historical completion remains recorded;
later drift/health loss is shown separately as current validity.

After restart, reconcile children using existing registry/backend and release
observation. A stopped or lost coding child leads to `awaiting-checks` with an
unverified-outcome notice or `blocked` on actual failure, never automatic
completion. Interrupted checks/releases remain unknown/failed until their
existing recovery path establishes an outcome. No automatic inference, cloud
creation, command retry, publication or overwrite occurs during boot or GET.

Show one project-level next-action card and a compact run history. Put receipts,
operation attempts and diagnostics behind details. Keep normal terminals and
independent sessions available. Use existing command trust, publication consent
and cloud cost acknowledgement at the moment of the corresponding operation;
starting a run is not consent for all future actions. Unanchored historical
project review statuses must be labelled as historical claims, not current
private approval.

## Meaningful acceptance

1. Create the run twice with the same key: one record, no child; changed request
   with that key is rejected. Two simultaneous starts allocate one session.
2. Inject crashes before child store, after child store, after backend allocation
   and before attachment. Reopen/reconcile retains the same child or a truthful
   unknown state and creates no replacement resources.
3. Exit a CLI successfully without producing the required files; mark tasks
   done. The run remains unverified. A failed command or missing check blocks it.
4. Edit/revoke/reapprove definition or design; rotate/disable the selected route;
   change source after checks. Continuation rejects stale bindings without
   changing model, account, requirements or the saved historical run.
5. Join a passing check and observed release for the exact checked commit and
   full selected requirement coverage. A different commit, incomplete coverage,
   old release without provenance, or ready row with lost ownership is rejected.
6. Copy IDs/records across accounts/projects/workspaces; forge run/receipt fields
   in project files. All admissions and joins fail without side effects.
7. Restart, cancel, retry a confirmed failed attempt and prune ordinary check
   history. Provenance remains readable, attempts stay distinct, and GET does
   not write or launch anything. No implicit cloud/publication acknowledgement.
8. Repeat the connected journey against a matching installed package, checking
   produced file bytes and actual UI/API behavior. Fixtures prove integration
   only; retain the separate real subscription/public-site and live cloud gates.

## Reuse decision

Glasses discovered and inspected `cschleiden/go-workflows`, catalogue
`0884f8a6a77cf5f1d5c6`, revision
`48e811947bece0a2a9993d85edcf9a0f3a7e486b`. Its exact MIT licence was fetched
(SHA-256 `467f04a45c9a89a37ea99eef77501a22c8089da4b58f193b0c70a010cc26ec97`).
The [SQLite backend](https://github.com/cschleiden/go-workflows/blob/48e811947bece0a2a9993d85edcf9a0f3a7e486b/backend/sqlite/sqlite.go)
is embedded and durable; its
[retry defaults](https://github.com/cschleiden/go-workflows/blob/48e811947bece0a2a9993d85edcf9a0f3a7e486b/workflow/retries.go)
allow three attempts, so side effects would still need Luci-specific idempotency
and permission handling. This record/evidence join can reuse Luci's existing
private persistence without adopting a replay engine. Reconsider the library if
automatic multi-step execution becomes a measured requirement. No dependency
has been adopted or validated in Luci by this document.
