# Private managed-host recovery

For a version change, follow [the stopped-state upgrade and rollback procedure](MANAGED-UPGRADE.md). It preserves the complete pre-upgrade state; an older binary must never write a newer context format.


`scripts/managed-host-backup.py` backs up and restores one commissioned Linux
managed-team installation. It preserves the entire private `LUCI_STATE_DIR`:
accounts/password hashes, login sessions, membership and project grants, service
access, private agent/provider configuration, conversations, attribution and run
receipts, and the team's source/context under
`control-plane/teams/<teamId>/runtime/projects`.

This is an operator tool. A project ZIP export deliberately omits these private
records and is not a substitute for host recovery. The host archive is not
encrypted. It is created with mode `0600`; keep its checksum through a trusted
channel, encrypt it with your backup system before off-host storage, restrict
access, and set a retention policy. It contains usable credentials and private
source, including ignored project files. Restoring an older snapshot also
restores the accounts, permissions and login sessions from that point in time.

The first supported recovery path uses the **same absolute state path, service
account and exact Luci release version**. There is no automatic path rewrite,
upgrade, cloud provisioning, or service startup. Install the matching verified
Luci release separately, restore, verify the team, then follow the upgrade
procedure. Do not run two copies of the restored identity against the same
Docker engine.

## What remains separate

Docker images and volumes, live PostgreSQL/Supabase data, external databases,
personal project folders outside the state directory, host environment files,
systemd units, TLS/SSH configuration and other external services are not in the
archive. Recorded database dump files already inside the state directory are
included; live database volumes are not. Take and verify application/data
backups before stopping their services. Preserve the operator's service and
Docker configuration separately. The JSON backup report and archive manifest
list these exclusions.

The command supports regular files and directories up to 64 GiB total and
250,000 entries. It refuses symbolic links, hard-linked files, sockets, special
files, foreign team directories, and unsafe permissions. It does not follow a
link or silently omit part of the state. For generated dependency symlinks,
remove only verified disposable build/dependency output after stopping its
writers and regenerate it after recovery; keep source links or unusual storage
in a separately reviewed backup process. An error means no usable new archive
was published.

## Backup

Run as the Linux account that owns the service state, with Python 3.11+ and
Docker CLI access. The state root must be owned by that account with mode `0700`;
identity, binding, lease and archive files must be private regular files.

1. Record the installed release, state path, immutable team ID, and actual local
   Docker socket from the service's configuration. Commission the managed team
   and start/stop this release once so its persistent binding and recovery lease
   are present.
2. Stop agent sessions, command checks, applications or any other writers with
   writable mounts into the state. Take data backups separately.
3. Stop the gateway and prevent the service manager from restarting it during
   recovery, for example `sudo systemctl stop luci` and
   `sudo systemctl mask --runtime luci`. Those service-manager commands are
   operator actions; the backup tool does not stop services or containers.
4. Invoke the backup command with a new filename outside the state directory:

```sh
python3 scripts/managed-host-backup.py backup \
  --state-dir /home/luci/.local/state/luci \
  --team-id team-abcdefghijklmnopqrstuv \
  --release-version 0.5.0 \
  --docker-host unix:///var/run/docker.sock \
  --archive /home/luci/backups/host-2026-10-06.zip \
  --acknowledge-private-secrets \
  --acknowledge-stopped-workloads
```

Use your actual team ID and release version. The archive parent directory must
already exist and belong to the service account. The command never overwrites
an archive. Retain the printed `sha256` with the private backup inventory. The
ZIP manifest also records a SHA-256 for every file, the team identity, original
absolute state path, release version, and successful Docker mount inspection.
A checksum checks integrity, not who supplied the archive; only restore a
backup whose origin and expected checksum you trust.

Every Linux gateway holds `.luci-state.lock` from initial bootstrap. Each
private worker holds its own state lease. The backup takes the root and all
worker leases and retains them for the snapshot, so a live gateway, an orphaned
worker or a concurrent recovery operation causes a refusal. It also inspects
all running Docker mounts before and after the snapshot. A writable mount
inside the state directory, or an ancestor exposing it, causes a refusal.
Docker inspection must succeed even if no resource history exists. There is no
Docker-unavailable override. Use the explicit local socket actually selected by
this managed installation; remote Docker engines are not supported here.

The command checks the source inventory and each file's inode, size, mode and
change times before publishing a completed archive. Keep workloads stopped
until it finishes. It does not claim to coordinate arbitrary programs launched
outside Luci or administrative writes to the state while recovery runs.

## Restore

Keep the gateway masked/stopped and all writers stopped. Retain the old state
by moving it to a different private path on the same host; do not delete it or
restore on top of it. Create an empty `0700` directory at the **original** state
path, owned by the same service account. Keep the source archive outside that
path, owned by the service account with mode `0600`.

```sh
python3 scripts/managed-host-backup.py restore \
  --state-dir /home/luci/.local/state/luci \
  --team-id team-abcdefghijklmnopqrstuv \
  --release-version 0.5.0 \
  --docker-host unix:///var/run/docker.sock \
  --archive /home/luci/backups/host-2026-10-06.zip \
  --sha256 YOUR_RECORDED_64_CHARACTER_SHA256 \
  --acknowledge-private-secrets \
  --acknowledge-stopped-workloads
```

The command verifies the outer checksum, archive structure, exact binding,
release/path, account/team references and every file hash before creating state
files. It rejects extra entries, duplicate paths, path traversal, links,
unlisted files, missing parent directories and existing destination state.
Files are created exclusively; archive modes cannot make restored files public
or grant special permissions. The root lease is the only allowed pre-existing
entry, permitting a retry after a refusal that wrote no state.

A handled write failure removes only the new paths created by that attempt. A
process or machine crash can leave a partial destination with
`.luci-host-restore-incomplete`; Luci refuses to start on that root. Retain the
partial directory for diagnosis and repeat the restore into a new empty
original path. Do not delete the marker to pretend the restore completed.

On success the tool reports `serviceStarted: false`. Restore external service
configuration and data from their separately verified backups. Unmask/start
the exact release, sign in normally, check the bound team, owner and member
access, project context/history, and any required data recovery before resuming
work. Existing Docker receipt files are historical evidence; copying them does
not recreate containers, images, volumes, tunnels, or verified deployments.

## Verification

```sh
python3 scripts/test-managed-host-backup.py
cd control-plane
go test ./internal/statelease ./cmd/api -run 'TestLease|TestManagedRootAndOrphanWorker' -count=1
```

The full compiled-service eval is:

```sh
node scripts/managed-host-recovery-smoke.mjs /absolute/path/to/luci /absolute/path/to/fresh-evidence-dir
```

It uses disposable state, real account/team/context APIs and the local Docker
engine. It requires the already installed `luci/sandbox:dev` image and never
pulls an image. A bounded, labelled sleeping fixture container proves that a
writable Docker bind prevents backup, and is removed by the eval. The eval
checks live gateway and orphan-worker refusals, preserves the original state,
restores into the original empty path, and verifies normal owner/member login,
permissions, service access, shared context and private attribution after
restart. No model inference or cloud resources are involved.

The same eval then creates a real owner deletion request, backs it up, cancels
it and proves that the stale request cannot authorize erasure. A fresh request
and current snapshot permit `scripts/managed-host-delete.py` to erase only the
verified fixture state. The eval checks that backups and the retained original
remain, and that the minimal `.luci-host-deleted` tombstone prevents automatic
fresh bootstrap. This is filesystem removal, not secure-media sanitization or
completion of external/provider/backup deletion obligations.

Controller-owned Git commands suppress automatic maintenance so a completed
command cannot leave a detached metadata writer racing the next operation.
Repository configuration is not rewritten. Operators can perform deliberate Git
maintenance in a quiesced, owned workspace after preserving its backup; stop
concurrent writers first. Maintenance is not dispatched by a read or a restore.
