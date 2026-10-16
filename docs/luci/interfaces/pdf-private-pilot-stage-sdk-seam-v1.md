# PDF private pilot: smallest stage and selected-SDK seam

Source plan only. No PDF pilot image, font, reader driver, numeric host-bind
ownership profile, or selected package grant is accepted. Nothing here is a
runnable fallback, and the four dedicated Go files remain unwired. The shared
ledger and admission insertions belong to the separate bridge owner.

## API-owned stage, before either Create

Use `executionAdmission.prepareExecutionDirectory()` for the existing private
state root, then a dedicated `pdfPilotStage.open` under
`ExecutionStateDir/pdf-pilots/<AttemptID>`. Its only children are
`renderer/pilot-<AttemptID>` and `reader/pilot-<AttemptID>`. This exact child
basename is required by `scripts/distribute-pdf-pilot/main.go:pilot` (`ownedStage`
and empty output directory); mounting a bare `renderer` at `/out` and passing
`--out-dir /out` would be refused by the writer. The existing
`PDFPilotSpec.OutputStagePath` describes the role parent; the adapter must
derive and verify the single `pilot-<AttemptID>` child rather than accept a
caller-selected path.

The API retains `os.Root` handles for the private attempt and each child,
records their ordinary directory identities, and refuses symlinks, reparse
points, hard links to foreign data, changed parent identity, or paths inside a
project/home/credential tree. On POSIX the child must be created with a
reviewed API-owner/renderer-UID/GID permission arrangement and checked through
open handles; on Windows a reviewed host ACL and Docker Desktop Linux bind
mapping must give the selected nonroot container UID write access to **only**
its child. Go `os.Chown` or an assumed `1001:1001` mapping is not proof of that
Windows arrangement. A selected-filesystem ownership/mount probe and exact
post-Create mount inspection are mandatory before enabling `Prepare`; absent
that proof, refuse before Create. Never chmod the shared state root, a project,
or a user home. The stage gets the existing 48 MiB reserved disk charge and
the whole attempt retains the 64 MiB host-capture allowance.

After a positive renderer exit, `pdfPilotStage.captureRenderer` opens only
`pilot.pdf` relative to the retained child handle, checks `Lstat` ordinary
single-link status and file identity before/after open/read, uses a no-follow
nonblocking open on the selected OS, reads at most 16 MiB+1, hashes the exact
bytes, and durably records the digest, size and file identity. The same
retained file must be mounted read-only for the reader only after renderer
capture **and** positive retirement; reader pre-Start must recheck its original
identity/digest. Output capture can fail independently of exact-CID retirement;
its error retains an unknown/failed outcome, never a replacement Create.

## Selected Docker SDK adapter, not a second engine

The later dedicated `internal/backend/dockerbe/pdf_pilot.go` would use the
existing `Backend.api` selected client and `LaunchFenceIdentity`/image inspect,
as `dockerbe/checks.go` does. It must verify Linux engine identity, exact
accepted image digest/receipts, limit support, fixed expected config, stage
bind identity, and `ImageInspect` before returning a prepared spec. No CLI,
daemon socket, endpoint fallback, image pull, build, or credential forwarding.

Renderer `container.Config` is fixed to the independently receipted binary
path, with argv `--font /input/NotoSansCJK-Regular.ttc --out-dir
/out/pilot-<AttemptID>`, numeric `User: "<reviewedUID>:<reviewedGID>"`, a
fixed minimal environment, `NetworkDisabled:true`, exact owner labels and
`Healthcheck: NONE`. `HostConfig` uses `ReadonlyRootfs:true`,
`AutoRemove:false`, `NetworkMode:none`, `IpcMode:none`, no ports/devices,
`CapDrop:[ALL]`, `no-new-privileges:true`, exactly two bind mounts: the accepted
original font read-only at `/input/NotoSansCJK-Regular.ttc` and the accepted
empty child directory read-write at `/out/pilot-<AttemptID>`. Memory and
memory-plus-swap are both 512 MiB, CPU 1, PIDs 32. Create warnings that
weaken limits fail closed. Inspect exact immutable image, entrypoint/argv,
user, env, labels, mounts, limits and owned CID before Start, then fence Start
using the existing exact selected-engine/instance protocol. The reader's
Create is prohibited until renderer capture and retirement are durable; its
source mount is the exact PDF file read-only and its output child is the only
write mount. One 512 MiB child charge remains held across both serial roles.

The fixed reader operations are:

1. `pdftotext -enc UTF-8 -nopgbrk /input/pilot.pdf /out/pilot-<AttemptID>/pilot.txt`
2. `pdftoppm -f 1 -l 1 -singlefile -r 72 -png /input/pilot.pdf /out/pilot-<AttemptID>/page`

The selected source pilot has exactly one 595×842 page
(`scripts/distribute-pdf-pilot/main.go` page tree), but the reader still must
check the PDF/page structure and one resulting PNG. The two operations require
one **independently sourced, receipted, packaged serial driver** inside the
single reader child; a shell command, host-installed reader, second uncharged
container, or invented helper is not an implementation. That driver and its
binary/library/grant closure do not exist as accepted release inputs, so the
reader adapter must refuse. The API must capture bounded UTF-8 text (at most
1 MiB) and exactly one PNG (at most 16 MiB) through the retained reader child
handle, then verify the original Unicode line `日本語`, PDF digest and visual
receipt without normalizing omitted glyphs.

## Completion and remaining review gates

The normal effect lane journals exact reservation/Create/Start/capture/retire
facts. A separately scheduled watchdog receives the immutable selected client
and each exact CID as soon as known; at cancel, phase expiry or host pressure
it attempts exact retirement independently of a blocked SDK call, stage read
or publisher. Lost ACK/failed retirement holds both allocation identity and
capacity through restart. Both roles share one 120-second envelope with no
overlap; the reader cannot begin until the complete renderer chronology has
been validated. A late positive retirement is historical cleanup, not pilot
acceptance.

Next source review must supply actual selected image/font/reader/driver
identities and grants, host bind UID/ACL proof for the target OS/engine, stage
opener and selected-SDK adapter fixtures, shared current-actor/source admission
and lease insertion, and whole-attempt watchdog/unknown recovery. This plan
does not authorize their effects or claim those components exist.
