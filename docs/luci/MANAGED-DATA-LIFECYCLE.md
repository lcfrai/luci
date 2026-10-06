# Managed team data requests and state removal

Team owners can request operator review of permanent data deletion from **Team
access → Service and data → Team data**. This saves an authenticated, private
request receipt. It does not stop workloads, cancel a subscription, change team
access, delete a project, or start a deletion timer. A current owner can cancel a
pending request. Cancellation changes only that request and its audit receipt.

The supported processing tool removes the verified state files of **one stopped,
persistently bound managed-team host**, after matching them against a trusted
backup. It does not implement deletion of one team from an arbitrary shared
installation, secure-media wiping, or complete deletion across providers.

## Request and review

The owner-only API is `GET /v1/teams/{teamId}/data-lifecycle`, returning
`{team,dataLifecycle}`. The initial state is `{revision:0,status:"none"}`.

`POST /v1/teams/{teamId}/data-lifecycle/request` accepts
`{expectedRevision:CURRENT_TEAM_REVISION,acknowledgeDeletion:true}`. On success,
the team revision advances and the lifecycle contains `status:"requested"`, a
unique `requestId`, its own revision, the authenticated `requestedBy`, and
`requestedAt`/`updatedAt` timestamps. The client cannot choose the actor, request
identity, timestamps or a completed status. A pending duplicate or stale team
revision returns `409`.

`POST /v1/teams/{teamId}/data-lifecycle/cancel` accepts
`{expectedRevision:CURRENT_TEAM_REVISION,requestId:EXACT_PENDING_ID}`. It preserves
the original request and attribution, adds `cancelledBy`/`cancelledAt`, and sets
`status:"cancelled"`. A later new request receives a new ID. Both mutations save
their private management audit event atomically with the state and roll back if
persistence fails. Responses do not expose a fabricated erasure-complete state.

A current Super Admin can inspect pending requests through
`GET /v1/admin/team-data-requests`, which returns
`{requests:[{team,dataLifecycle}]}`. On a bound managed host it includes only the
bound team. This read does not start a worker or process the request. Operators
should verify the request with the team owner, the agreed retention/legal
obligations, billing cancellation and intended external-resource disposition.
The CLI independently requires that the requester still be an enabled owner.

## Before removing files

1. Confirm the exact managed team ID, request ID, state directory, installed
   release version, service account and Docker socket. Keep the installed release
   and the matching `managed-host-backup.py` and `managed-host-delete.py` together.
2. Give the team an opportunity to export source/context and retain required
   application data. Project exports omit private authority and credentials; they
   are not complete host or database backups.
3. Inventory and address live apps, sessions, checks, Docker volumes, databases,
   external storage, cloud resources, Git publications, host configuration and
   billing separately. Stop all state writers and applications that mount the
   state directory. Prevent the gateway's service manager from restarting it.
4. Follow [private host recovery](MANAGED-HOST-RECOVERY.md) to create a new,
   stopped-host backup **after** the current pending request. Keep its trusted
   checksum outside the state root. The archive contains credentials and is not
   encrypted by the tool; protect it and encrypt off-host copies through the
   operator's backup system.
5. Keep the host stopped. A subsequent login, cancellation, setting change,
   source edit or other changed state byte invalidates the backup comparison.
   Reinspect the current request and create a new stopped backup if appropriate.
   Never edit state to make an old backup appear current.

The command takes the root and nested worker leases throughout comparison and
removal. It verifies archive origin through the supplied trusted checksum, its
manifest and per-file hashes, exact team/path/release binding, current private
request, and current file inventory/bytes. A running state writer, unavailable
Docker daemon, overlapping writable Docker mount, unexpected path/link, stale
backup or missing/cancelled request causes refusal. These locks do not coordinate
arbitrary administrative programs; operators must keep all writers stopped.

## Bounded operator command

Run as the owning Linux service account with Python 3.11+ and access to the actual
local Docker socket. Use actual identifiers and existing private parent
directories; the archive and a new receipt path must be outside the state root.

```sh
python3 scripts/managed-host-delete.py \
  --state-dir /home/luci/.local/state/luci \
  --team-id team-abcdefghijklmnopqrstuv \
  --request-id delete-abcdefghijklmnopqrstuv \
  --release-version 0.5.0 \
  --archive /home/luci/backups/host-before-removal.zip \
  --sha256 YOUR_RECORDED_64_CHARACTER_SHA256 \
  --docker-host unix:///var/run/docker.sock \
  --receipt /home/luci/receipts/state-removal.json \
  --acknowledge-state-erasure
```

The explicit acknowledgement permits removal of matched state files; it is not
an instruction to remove arbitrary directories or provider resources. The tool
never overwrites an archive or receipt. Refusals before the removal barrier do
not remove state. Check the command's exit status and its actual receipt.

## What the receipt proves

Before unlinking verified state files, the tool creates a durable
`.luci-host-deleted` tombstone. It retains that marker, the original root
`.luci-state.lock` inode, the state-root directory and an external private receipt.
Luci refuses to start on the tombstoned root, preventing accidental fresh
bootstrap after its identity was removed.

A successful receipt has `status:"state-erased"` and
`resourceCleanupComplete:false`, records request/team/release and backup checksum,
and lists retained/excluded resources. It proves the verified state files were
removed by that operation. It does not prove secure sanitization of underlying
media, removal of Docker volumes or running external applications, deletion of
provider records, or expiry of archives, replicas and cloud snapshots. The proof
archive deliberately remains available for recovery and must later expire under
the operator's disclosed retention policy.

An interrupted operation retains the tombstone and an incomplete receipt. Normal
startup and automatic deletion retry are blocked. Preserve the receipt and exact
backup, inspect the remaining state and resources, and use a reviewed recovery
plan. Do not remove the tombstone to claim completion or restart a partially
erased workspace. The operator must report completed and outstanding scopes to
the owner separately and keep the minimal processing receipt under its own
retention/access policy. The software does not invent a final all-data-deleted
confirmation or retention deadline.

## Verification evidence

Focused unit checks are `python3 scripts/test-managed-host-delete.py` and
`python3 scripts/test-managed-host-backup.py`, plus the management lifecycle tests
and root/worker lease tests. `scripts/managed-host-recovery-smoke.mjs` exercises
the compiled service with disposable identities and source: real API request,
backup, cancellation and stale-proof refusal; a fresh request and exact backup;
state removal; retained original/archives; and tombstone startup refusal.

The local 6 October 2026 fixture run passed seven checks; its exact binary and
results are recorded in
`artifacts/onboarding/managed-host-recovery-erasure/result.json`. That is local
fixture evidence, not an erasure of a customer's host, production cloud storage,
payment records or external backups.

The [terms](MANAGED-SERVICE-TERMS-DRAFT.md) and
[privacy](MANAGED-SERVICE-PRIVACY-DRAFT.md) drafts require an operator's actual
retention, contact and legal decisions before publication.
