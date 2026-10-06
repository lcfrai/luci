# Upgrading and rolling back Luci

Use a stopped-state upgrade. Keep the previous application package and a
verified, private backup made before the upgrade. Replacing the application
folder does not back up projects, account identities, provider credentials,
private approvals, conversations or application databases.

The 0.5.0 context writer uses format 2. It reads existing format-1 context
without rewriting it on a GET; the next explicit context save migrates that
project. **Never start an older writer against state that 0.5.0 has changed.**
Rollback restores the complete pre-upgrade state first. It deliberately loses
subsequent changes from the active workspace; retain the stopped upgraded state
separately for recovery rather than merging private state files between versions.
See [context compatibility](CONTEXT-STORAGE.md).

## Before upgrading an installation

1. Identify the exact current and proposed release versions and package hashes.
   Obtain packages and checksums through the intended trusted distribution
   channel. An adjacent manifest verifies package consistency; it does not
   establish publisher identity. Keep the current package available for rollback.
2. Record the absolute state directory, project directories, service account,
   service configuration and external credential bindings. Keep the same paths
   and account during this procedure. Private review authority and runtime
   configuration are not reconstructed by importing a project ZIP.
3. Stop application writers, agent sessions, checks and deployed workloads that
   write to the state or project trees. Stop the gateway and its workers. Confirm
   the owning processes and overlapping writable containers are stopped before
   taking a backup. Merely closing the browser is insufficient.
4. Back up the complete private state and any project trees outside it, with
   their permissions. Back up application databases/volumes using their own
   supported procedures, and retain the operator configuration and credentials
   outside the state directory. Keep backups private: they contain password
   hashes, provider credentials and other application secrets. Verify hashes and
   rehearse restoration before treating the backup as a rollback plan.

For an already bound 0.5.0 managed team, use the existing
[`managed-host-backup.py`](../../scripts/managed-host-backup.py) operator workflow
in [managed host recovery](MANAGED-HOST-RECOVERY.md). It validates the exact team,
release, stopped process leases, container mounts and private files. Its restore
requires the recorded release and original empty state path. This tool does
**not** accept historical 0.4.18 personal state, does not include external
databases/volumes or external project directories, and is not an in-place schema
downgrade tool.

## Upgrade and acceptance

Extract the new package into a separate application directory and verify its
manifest. Point the existing service configuration at the new package while
retaining the same private state/project paths, service account and explicit
provider configuration. Start one gateway with its own supervised workers.

For portable use, close the old launch terminal before starting the new package.
The new launcher refuses to reuse an older/different package or a service that
cannot report its portable identity, even when the release number is the same.
It does not stop that existing process or change its state. Follow the explicit
close-old-launcher instruction; do not reset the state directory to bypass it.

Before reopening access, verify the reported release, existing account login,
intended personal/team access, saved projects and context, review state,
provider configuration and history. A configured connection is not proof that
upstream credentials remain valid. Test any required provider, database,
deployment and public HTTPS operations using the installation's normal explicit
authorization and cost controls. Observe private service logs and stop on a
failed verification; do not reset accounts or overwrite data to force startup.

An actual change to saved context checks write compatibility. Keep the original
backup until the deployment's acceptance and retention policy allows disposal.
Luci has no configured automatic update or automatic rollback service.

## Rollback

Stop the new gateway, workers and every writer again. Preserve the upgraded
state and any corresponding database changes separately. Restore the verified
pre-upgrade state/project backup into the original, empty paths with the
original account ownership and private permissions. Restore external database
state consistently if it changed; the Luci state backup alone cannot reverse
database migrations. Then switch the service back to the matching old package,
start it and verify account, context, provider and history recovery. Do not
reuse an upgraded state tree with an old binary, restore over live state, or
copy individual identity/provider records between generations.

## Reproducible Linux rehearsal

The fixture runner starts actual services on private ephemeral ports, provisions
two accounts and a personal project using the historical package, and never
accepts an existing state directory. It points Docker at a nonexistent fixture
socket and does not create containers, invoke models or contact cloud services.
Its stopped copy reuses the operator recovery tool's owned-file, no-link,
stable-inventory and process-lease implementation. It additionally binds the
gateway and each separately grouped worker to the fixture state, executable and
PID/start time, then confirms they all stopped. Historical 0.4.18 does not have
the new state leases; its shutdown proof uses these actual process identities.
The fixture copy is not a
general-purpose operator backup interface.

```sh
node scripts/managed-upgrade-smoke.mjs \
  --old-package artifacts/releases/0.4.18/luci-linux-amd64.tar.gz \
  --new-package artifacts/releases/0.5.0/luci-linux-amd64.tar.gz \
  --output artifacts/uat/managed-upgrade-package/result.json
```

Both archives must have adjacent `release-manifest.json` files identifying clean
source and matching package byte counts/SHA-256. Linux, Node.js, Python 3 with
tar extraction filters, and Git are required. The result identifies both
package and binary hashes. `--keep-fixture` retains private diagnostic state;
otherwise successful fixtures are removed. Failed fixtures are retained and
their private path is recorded. Do not publish their state or logs.

During development, `--new-binary PATH --new-version 0.5.0` replaces
`--new-package`. That result is bound to the compiled binary hash; the observed
runner checkout is not proof of its build provenance and does not count as
matching clean-package acceptance.

The compiled candidate rehearsal passed six checks against the clean historical
0.4.18 package (`50ea11cc4568676dc716687ddd9405782fe515c4`) and 0.5.0 binary
SHA-256 `6cccf28f9550cbcafedf0a2846b4466baa268a204fae5a38a8c868ce780bfe05`.
The receipt is `artifacts/uat/managed-upgrade-current/result.json`. It verifies
existing bearer sessions and password login, account isolation, exact private
provider bytes, current approved context, older specification revisions, Git
history, explicit format-2 migration/restart, and byte-verified restoration
before an old format-1 write. Version 0.4.18 predates teams; this is **personal
state migration**, not evidence of team migration. Managed-team restart/restore,
native installers, external databases, real providers and public infrastructure
have separate acceptance requirements. Matching packaged 0.5.0 acceptance must
use the package command above and retain its separate receipt.
