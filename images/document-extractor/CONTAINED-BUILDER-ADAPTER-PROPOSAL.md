# Dedicated offline document-image assembler: source seam for root review

**Proposal only.** No compiler, acquisition, image/container/engine operation or
shared product edit is authorized. Inspector source73550716 remains frozen. This
is a specific replacement for the missing worker/cache containment primitive;
it is not an alternate retry of held producer/installer/observer transports.

## Chosen resource domain

Use one ordinary positively owned Docker container through the existing selected
SDK backend. Do **not** call ImageBuild, BuildKit, a daemon socket inside the worker,
ImagePull, nested Docker, WSL, privileged mode or a new engine. Its immutable runner
image is the already declared Debian amd64 base, exact config/image ID
`sha256:db9f02c6bde9fa90cc8074c92754b2b046947392f1a857726e77f041febb7b82`,
only after base bytes, authentication, licence and local registration are separately
reviewed. No additional builder image is silently acquired.

Mount one exact closed input directory RO and one reviewed static Linux-amd64 Go
assembler binary RO. The binary is Luci source compiled later in a separately
bounded offline cloud/source batch, with source/toolchain/binary/SBOM pins before
use; it is not present or compiled now. No host home, credentials, project data,
arbitrary argv/script/URL or Docker socket enters this container.

| Container property | Fixed requirement |
| --- | --- |
| Root filesystem | read-only; no writable layer/cache path |
| Workspace | `/work` tmpfs hard size2GiB, mode0700, exec/nosuid; all rootfs/input/staged OCI bytes live here |
| Temporary files | `/tmp` tmpfs128MiB, nosuid/nodev/noexec; charged to same memory cgroup |
| Memory/swap | 2GiB memory and2GiB total memory+swap: no additional swap allowance |
| CPU/PIDs | 1CPU; pids.max64 covering assembler, chroot and all maintainer-script children |
| Connectivity | network none, no exposed ports/host aliases; IPC none; no host PID/UTS/user namespace |
| Process | fixed `/tool/luci-document-image-assembler`, one fixed profile, no shell/argv override |
| Security | nonprivileged, no-new-privileges, default restrictive seccomp; no SYS_ADMIN or host device mounts/requests |
| Output | stdout-only owned framed OCI stream<=2GiB, diagnostics<=1MiB, Docker log driver none |
| Time | compute/export<=300s, cleanup reserve120s, one absolute420s attempt |

The workspace consumes memory **and** has a hard tmpfs size. Rootfs extraction,
package configuration, compressed layer staging and OCI material all compete for
that same bound; no separate unbounded daemon build cache exists. tmpfs uses the
container cgroup memory and cannot escape its memory+swap cap. A real Docker
acceptance must verify those actual configured/enforced limits; source is not proof.
Daemon bookkeeping remains a separately admitted small overhead, never labelled
as contained build memory/disk. No successful build is promised within2GiB/300s.

## Exact assembly profile and compatibility prerequisite

The proposed recipe is explicitly new `document-flat-oci-v1`; it does not claim
byte equivalence to the existing classic Dockerfile or reuse its held producer
identity. The fixed inputs are closed manifest/base layer, all selected exact debs,
two wheels, extractor entrypoint, full notices and expected installed-file hashes.
No user supplied Dockerfile, build instruction or mutable source path is accepted.

The static helper verifies every input and safely unpacks the one base layer into
`/work/rootfs`, copies only hashed installation inputs into that tmpfs rootfs, then
invokes the existing fixed Debian `chroot` tool and the reviewed offline equivalent
of the current recipe: dpkg unpack/configure exact closed set, pip no-index/no-deps/
only-binary/hash-lock, eng/osd copy and exact installed-file/version/config checks.
No interpreter/module from an input wheel is imported by the outer controller.
The eventual isolated image verification is a distinct explicitly reviewed build
step; it cannot parse a user document or initialize forms/JS.

This is trusted package installation, so the worker runs UID0 **inside its own
container**, unlike the later nonroot document parser. Drop all capabilities then
allow only CHOWN, DAC_OVERRIDE, FOWNER, FSETID, SETUID, SETGID, SYS_CHROOT and MKNOD.
MKNOD is solely for a positively owned temporary `/work/rootfs/dev/null` (1:3),
removed before image export. Normal container device-cgroup restrictions remain;
no host devices are mounted. `/work` must permit that one device node; this is an
explicit difference from a nodev parser tmpfs, not an implicit weakened fallback.
No SYS_ADMIN/mount capability, host `/proc` bind or unconfined seccomp is allowed.

**Finite functional prerequisite:** exact dpkg/maintainer-script/pip recipe behavior
in this chroot, without a mounted inner `/proc`, is unproved. If it requires a
forbidden mount/capability or exceeds bounds, report that exact failed operation;
do not grant SYS_ADMIN, change engine/mode, fake `/dev/null`/proc semantics, skip
configuration scripts or call the result a build. Inert source tests can cover
argv/config/identity now; the genuine bounded compatibility run comes only after
input/source/image/profile release. This prerequisite does not undo the proposed
hard resource domain or imply current runtime permission.

After exact installed verification, remove only owned transient input/dev-null
paths and stage a single compressed rootfs layer inside `/work`'s quota. Produce
OCI config/manifest/index with fixed parser USER/ENV/ENTRYPOINT, exact source/recipe
labels and full SBOM/notice references. Emit a flattened OCI artifact through a
pre-Start attached SDK stream with length/digest/complete framing; rootfs+staged
compressed output must fit the same2GiB tmpfs. Streaming does not add a writable
host mount or another cache. Complete capture, exit0/non-OOM, exact container
retirement and stream-reader settlement remain separate facts.

## Additive typed adapter seam

Proposed dedicated files, allocated only after root agrees this seam:

- `control-plane/internal/backend/document_image_assembly.go`: immutable
  `DocumentImageAssemblyIdentity` and fixed profile/spec/prepared/result types;
  resources are server constants, not caller-selected values.
- `control-plane/internal/backend/dockerbe/document_image_assembly*.go`: existing
  selected SDK Prepare/Create/pre-Start Attach/Start/Inspect/Retire/Fence wrappers,
  no alternate transport. Preserve endpoint/TLS/auth/negotiated version explicitly.
- `control-plane/internal/api/document_image_assembly*.go`: dedicated serial lease,
  current root installation authority, capture/cleanup receipts and recovery.
  No public route or automatic boot build; root controls the one reviewed attempt.
- `images/document-extractor/assembler/`: static Go helper, fixed recipe/profile,
  bounded OCI encoder and inert fault tests. No shared CheckExecutor semantics or
  document-extraction lifecycle changes.

Identity binds instance/selected engine/allocation/actor/workspace/team, profile,
base config ID, input manifest/recipe/source/helper digests. Every wrapper requires
that identity; exact name+complete labels+configuration distinguish it from agent,
check, parser, preview and historic runtime-image containers. A lost Create/Start
ACK or stale/current-authority failure fences; a matching later container readback
is not substituted for an ACK. No cross-purpose adoption, late attach or replay.

Minimal executor methods are `PrepareDocumentImageAssembly(spec)`,
`CreateDocumentImageAssembly(prepared)`, `AttachDocumentImageAssembly(identity,cid)`,
`StartDocumentImageAssembly(identity,cid)`, `InspectDocumentImageAssembly(identity,cid)`,
`RetireDocumentImageAssembly(identity,cid)` and typed fence create/verify. The SDK
stream must attach to the exact acknowledged CID before Start, with raw framing,
payload and total deadline bounds; no stdout CLI or private metadata proxy.

Root/cloud must allocate the narrow shared admission counters/recognizer calls
before an implementation can admit resources: assembly leases/fences count in
**every** existing admission path, retain capacity past deadline until positive
retirement+reader settlement+durable receipts, and exclude another heavy batch.
Emergency exact-CID stop must precede blocked readers/publishers, as in accepted
LOCAL089; it never releases uncertain capacity or accepts partial OCI data.

Normal fresh8GiB physical/commit and CPU<50 start,4GiB/unknown brake, serial heavy
work, exact output disk reserve and root external whole420s supervision remain.
No model, sign-in, source installer or package fetching occurs in this attempt.

## Review and acceptance order

1. Root accepts or corrects this fixed cap/mount/device/recipe seam and allocates
   dedicated source/shared accounting ownership. No effect follows the document.
2. Finish trusted metadata/signature/notice and binary-member closure. Review the
   bounded static-helper compile and its provenance/licence manifest separately.
3. Inert tests: altered limit/mount/cap/profile/identity refusal, unrelated resource
   non-adoption, lost ACKs, over-quota/OOM/cancel/blocked reader/publication fences,
   actual OCI framing/member/config hashes and immutable expected output mapping.
4. Separately admit one real bounded assembler compatibility/build run. Verify
   no daemon build cache/worker allocation, actual cgroups/tmpfs quotas and positive
   container/client/reader retirement; retain failures truthfully.
5. Root reviews the captured OCI artifact and actual SBOM/notice/install checks.
   **Image registration is a separate effect**, through an explicitly reviewed
   ordinary image-import path with its own bounds/identity/cleanup. No import,
   tag/publish, held installer/gate invocation or automatic runtime follows.
6. Actual Discover extraction and model-backed conversation acceptance remain
   required separately. Normal sign-in/real Develop belongs to its assigned worker.
