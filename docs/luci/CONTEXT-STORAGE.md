# Portable context and storage

Luci keeps project content portable and review authority private. The portable
`contextId` in `.luci/state.json` is a public lineage label: it follows supported
Git clones and backup restores. Local API project IDs, project permissions,
private review receipts, execution records and authenticated editor history
remain bound to their actual workspace and project. Knowing or copying a
`contextId` grants no access.

A clone opened at another path is rebound in memory to its local project ID.
Its brief, specification, sketch, design handoff and task plan remain readable.
Former specification/design approval claims and prototype references become
drafts requiring fresh review. Reading does not rewrite the clone. The next
save writes its local binding while preserving the portable lineage. A legacy
project without `contextId` derives a stable label from its original saved ID,
so different clones of the same legacy file agree without a migration write.
The Context storage panel identifies imported context and generated exports
that still reflect its former binding.

## Format compatibility

New canonical saves use context format **2**. Luci reads legacy format **1** and
upgrades it only on a write. This version boundary is intentional: older Luci
writers must refuse an archive-aware file, rather than silently discard the
portable identity and archive pointer as unknown JSON fields. Use an updated
Luci installation to edit context that has been saved in format 2.

The ZIP backup container and manifest remain format 1. Existing backups remain
readable. New backups add validated `context-history/<sha256>.json` entries for
the attached history chain and `context-unused/<sha256>.json` for verified
chunks left by an interrupted save. Restore validates the full inventory,
content hashes, public lineage, project identities and limits before reserving
a destination project. Extra unattached history entries, missing ancestors,
foreign context and mismatched hashes are refused. Unused chunks remain unused
after restore; restoring never grants private review or editor authority.

## Bounded history without silent deletion

The canonical file retains current context and up to eight recent revisions of
each historical content kind. It targets 4 MiB, moving older snapshots out
sooner for substantial scenes, and has an 8 MiB hard limit. Each kind retains
its newest snapshot in recent history where space permits; current snapshots
always remain in their canonical fields. Unusually large current content can
exceed the target while remaining below the hard limit. Individual scene
and request limits still apply. Canonical JSON and raw drawing exports stay
compact, and drawing nesting is limited to 64 levels to prevent pretty-printing
from amplifying a small input into an oversized allocation. Legacy indented
scenes are checked by their compact content, so whitespace is not a migration
size penalty.

Older snapshots move to immutable `.luci/history/<sha256>.json` chunks. Each
chunk names its predecessor and preserves its snapshots, public lineage and
original local project ID. Historical public approval labels are historical
claims, not current private approval. Each chunk is at most 16 MiB. The entire
attached chain is limited to **256 chunks and 256 MiB**; the physical storage
limit also counts unused chunks from interrupted writes. Git's one-click
commit permits only verified attached context chunks. Private receipts,
authenticated editor identities and unrelated `.luci` files stay outside this
public boundary.

At capacity, a save stops before changing canonical context. It explains how
to download a project backup and explicitly remove archived revisions in
Context storage. Cleanup preserves current and recent revisions. The UI
requires a completed backup download and acknowledgement that it was saved;
the server requires explicit acknowledgement and a matching storage digest.
Luci cannot verify that a browser download was retained on someone's device.
The backup includes verified unused chunks so interrupted work is recoverable.
There is no automatic deletion of archived revisions. Git history and existing
backups may still retain files after explicit working-copy cleanup.

The storage digest covers canonical context and the physical archive inventory.
A new interrupted chunk after backup invalidates cleanup even if canonical
revisions did not change. Cleanup detaches the canonical head durably before
removing verified files. An interrupted cleanup can leave unused files;
Context storage exposes them and supports another explicit cleanup. Corrupt or
foreign chunks are not silently deleted: recover the verified files from Git or
a backup before continuing.

## Generated export recovery

`.luci/state.json` is canonical. `LUCI.md`, design/plan JSON, specification
JSON/Markdown and design-handoff JSON/Markdown are generated exports. A save
validates paths, preserves older revisions, writes canonical state atomically,
then replaces exports individually. Directory entries are synced on supported
Unix filesystems before publishing a new history head. Windows uses the
existing atomic replacement mechanism; directory fsync is unavailable there.

A disk or permissions failure during export can leave canonical state saved
and some exports incomplete. The error states that outcome. GET Context
storage only compares files and reports `current`, `missing`, `changed` or
`unavailable`; it never repairs or grants approval. Explicit repair compares
the storage digest and regenerates exports from the verified canonical state
and current private review selection. It neither increments content revisions
nor replaces private authority. Repair deliberately replaces manual edits to
generated exports; durable changes belong in the context editors. Unsafe or
unwritable targets require correction before retrying.

## HTTP contract

All paths are relative to `/v1/product/projects/{id}` and use normal project
and workspace authorization.

- `GET /context`: lineage, import status, storage digest, export observations,
  attached/unused archive usage and limits. This is a read-only observation.
- `GET /context/archive?cursor=<digest>`: one verified reachable chunk with
  `digest`, `nextCursor` and `chunk`. Omit the cursor for the latest page; an
  empty archive returns a null chunk. Unused or foreign cursors return 404.
- `POST /context/exports/repair` with `{ "expectedDigest": "..." }`: repair
  exports and return refreshed Context storage status. A stale digest is 409.
- `POST /context/archive/prune` with `{ "expectedDigest": "...",
  "acknowledgeArchivedHistoryRemoval": true }`: detach and remove validated
  archived/unused chunks, retain current/recent context, and return status.
- Existing specification/design history responses include `archiveHead` when
  older revisions have moved out of their recent `revisions` arrays.

Authenticated change attribution is separate private workspace data under
`GET /context-history`. Public archives and backups do not copy that ledger.
Session inputs and completed project-run evidence retain their own immutable
snapshots even after later context edits or explicit archive cleanup.

Atomic archive writes can leave a reserved `.luci/history/.luci-write-<digits>`
staging file if the process stops before rename. That file has never been an
accepted archive. All previously accepted revisions remain in the canonical
state and attached history; a new unconfirmed save must be retried from the
caller's retained draft. GET reports the incomplete inventory without deleting it.
The next successful-revision context save or explicit Git commit discards only
bounded ordinary files in that reserved staging namespace, then continues.
Stale writes leave them untouched. Symlinks, oversized staging or excessive
entries fail closed for operator inspection. Immutable `<sha256>.json` chunks
remain subject to the backup-and-explicit-cleanup contract above.

## Validation scope

`journey_portability_test.go` exercises actual Git cloning into a different
folder, private approval isolation, twelve substantial drawing saves, full
archive backup/restore, the 256-chunk boundary, explicit capacity recovery,
partial export repair and stale writers, malformed backup chains, interrupted
chunks, pre-rename crash recovery and a chunk appearing after backup. Existing legacy backup and private
approval export-retry regressions also pass. These are filesystem, Git and HTTP
fixture tests; they do not establish paid-service isolation or installed
Windows/macOS acceptance. Those remain separate roadmap gates.
