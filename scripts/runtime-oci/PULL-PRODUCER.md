# V2 source and ordinary release entry points

These are maintainer-only Python tools. Installed portable/native Luci packages
carry the verified store, bootstrap, receipts, canonical inputs and grants;
ordinary setup needs Docker and the packaged Luci executable, with no Go,
Node, Python, source checkout or download. This source does not establish
Docker Desktop routing, a measured memory pass or actual upstream model work.

## Sequential producer prerequisites

Use clean exact source checkouts, a positively completed original builder and
its unchanged original image. Never rebuild the existing runtime to change its
transport. Preserve original failed runs. A genuine original v1 build receipt,
manifest, normalized payload and runtime grants are required before v2 export.
The original receipt is retained byte-for-byte; no cancelled export can invent
it. If absent, run the existing explicitly authorized no-recompile finalizer:

```powershell
py -3 scripts/runtime-oci/finalize.py --revision <clean-tooling-40-hex> --build-repo <original-clean-checkout> --original <original-positive-producer-directory> --output <fresh-finalized-directory>
```

Its initial admission is 2 GiB native available, no running Docker containers
and observed CPU busy at most 90%. Probes are 512 MiB/one CPU and licence
collection is 256 MiB/one CPU. Export separately requires 1 GiB host reserve,
512 MiB margin and `max(512 MiB, exact inspected image.Size)`. A 100 ms native
watch cancels below 1.5 GiB. It requires 18 GiB free output storage for bounded
8 GiB raw plus 8 GiB normalized archives and 2 GiB reserve. Optional existing
`--producer-wsl-cache-assistance` is dedicated-producer assistance, only when
separately authorized. It is never ordinary installed setup behavior.

## Small bootstrap

`bootstrap.py stage-source --repo <clean-checkout> --revision <full-HEAD>
--output <fresh-directory>` stages every unchanged Git blob in the complete
`scripts/runtime-oci/pull-gate/` subtree and prints its source/recipe hashes.
Build the staged static gate with the pinned local Go toolchain, offline:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off GOTOOLCHAIN=local
go build -trimpath -buildvcs=false -o luci-registry-gate .
```

Use the staged Dockerfile, resulting gate and separately verified pinned
Distribution binary as the only scratch-image context members. Build with
explicitly bounded memory/CPU, `--network=none`, no pull, and no runtime
payload. Retain exact command argv, tool version, allocations, exit status and
stdout/stderr. Save that exact bootstrap image and run its actual
`/luci-registry-gate --version` under an isolated bounded container. The probe
must return exactly `luci-registry-gate 1\n` and empty stderr. The bootstrap
archive must preserve config/diffID order and contain only the two binaries.

The publication evidence directory has `bootstrap-execution.json` with exact
`builder={argv,version,memoryMiB,cpuMillis,exitStatus}` and
`probe={argv,exitStatus}` objects and corresponding `builder.stdout.log`,
`builder.stderr.log`, `probe.stdout.log`, `probe.stderr.log`. Both exit statuses
are zero; missing/nonzero terminal evidence refuses. Also retain
`gate-compile.json` with exactly `sourceCommit`, `gateSourceSHA256`,
`gateBinarySHA256`, `argv` (the seven arguments above), `environment` (the five
values above), `exitStatus:0`, `stdoutSHA256`, `stderrSHA256`, and the matching
`gate-compile.stdout.log`/`gate-compile.stderr.log`. This binds every clean
compile input and the actual emitted binary; a version string alone does not
authorize source provenance.

```powershell
py -3 scripts/runtime-oci/bootstrap.py publish --repo <clean-checkout> --revision <full-HEAD> --source <raw-bootstrap-save.tar> --image-id <exact-config-ID> --gate-binary <actual-static-gate> --distribution <verified-official-distribution-directory> --evidence <actual-command-evidence-directory> --output <fresh-bootstrap-output>
```

Publication writes the deterministic tagless bootstrap, strict canonical
bootstrap receipt, manifest fragment, clean subtree inventory, compile proof
and original Distribution Apache grant. It performs no build or Docker effect.

## Owned compressed export

```powershell
py -3 scripts/runtime-oci/pull_producer.py --repo <clean-checkout> --revision <full-HEAD> --build-repo <original-clean-checkout> --original <original-positive-producer-directory> --v1-payload <genuine-finalized-v1-output> --bootstrap <bootstrap-output> --distribution <verified-official-distribution-directory> --daemon-probe <matching-source-compiled-probe> --instance-id <private-producer-instance> --output <fresh-v2-evidence-directory>
```

The existing helper placement proves host and selected-daemon loopback routing
before tag/push. It admits its 512 MiB/one CPU/64 PID helper separately from
the larger of 1 GiB or the original largest-layer compression estimate, plus
unchanged 1 GiB reserve and 512 MiB margin. Daemon compression is not cgroup
capped. Push uses a 100 ms native watch, 1.5 GiB cancellation margin, bounded
logs and 20-minute deadline. A private retained tag protects the original image
before the fresh transport tag is created. An uncertain client, upload,
readback or helper outcome writes retained generation evidence and never
retries or publishes a success receipt.

After positive CLI terminal and immutable manifest readback, only compressed
blobs are saved and streamed through gzip/diffID verification. The helper must
positively retire before publication. Conversion/store plus producer receipts
bind the original config, ordered diffIDs, full source reuse and exact helper
terminal/retirement receipts. The private retained tag remains; no shared image
or layer is deleted. V2 requires 10 GiB free output scratch and holds compressed
blobs plus the at-most-4-GiB store; it never creates another expanded layer or
large v1 tar. All actual producer phases still require the external whole-host
resource watch and current root effect authorization.

## Ordinary release

`pull_payload.py verify --payload <v2-output/payload>` performs bounded offline
source, receipt, licence, store/blob/diffID and bootstrap verification.
`stage --payload ... --repo ... --revision ... --package ...` binds the chosen
clean release source; a later source revision requires independently equal
canonical input bytes and the exact original v1 manifest/receipt siblings.
`core-from-package --package ... --output ...` creates the small Linux core
with a strict v2 companion marker and excludes both large assets and their
runtime-only receipts/grants. The matching outer package carries one copy.

`scripts/build-release.ps1 -RuntimePayloadDir <v2-output/payload>` defaults to
`-RuntimeFormat v2` and stages portable and native assets, including bootstrap
grants. Explicit `-RuntimeFormat advanced-v1` preserves the original v1 path.
Missing v2 assets refuse; no fallback builds or downloads exist. Release
building is a separate maintainer effect with its existing toolchain/resource
requirements. `scripts/package.py` binds every required v2 runtime file and
optional reuse sibling in package identity. Bootstrap bytes count toward the
512 MiB core bound; the store has its separate 4 GiB manifest bound. No package
consumer runs any of these Python scripts.

Focused inert checks:

```text
py -3 -m unittest discover -s scripts/runtime-oci -p "test_*.py"
```
