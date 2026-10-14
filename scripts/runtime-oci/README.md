# Source-bound runtime producer

Ordinary Luci setup imports the verified Linux runtime included in the desktop
package. It does not compile the upstream agents on the user's PC. This tooling
is an explicit release-producer operation; its success is not installed-user
acceptance.

Use a full clean Git checkout, Python 3.10+, Docker's Linux amd64 engine and the
two cached official bases named by `images/sandbox/Dockerfile`. Base image IDs
and repository digests are inspected and retained. Missing bases are a visible
prerequisite; this tool does not pull them silently or reuse an unbound image.

```powershell
$revision = git rev-parse HEAD
if ($LASTEXITCODE -ne 0) { throw 'Git source identity failed' }
python scripts/runtime-oci/producer.py --repo . --revision $revision --output E:/private/luci-runtime-unique
if ($LASTEXITCODE -ne 0) { throw 'Preserve producer output and failure logs; do not retry blindly' }
python scripts/runtime-oci/payload.py verify --payload E:/private/luci-runtime-unique
if ($LASTEXITCODE -ne 0) { throw 'Runtime payload verification failed' }
./scripts/build-release.ps1 -Version 0.5.0 -Target windows-amd64 -RuntimePayloadDir E:/private/luci-runtime-unique
```

The fresh output must be outside the checkout. The producer reserves 1 GiB host
RAM plus its enforced 1 GiB / one CPU build and 512 MiB additional admission
margin. It requires idle Docker, CPU margin and disk room for both raw and
normalized archives. It observes exact labelled build containers, hard limits,
host RAM and bounded deadlines. Cancellation retains the terminal builder
stream; ambiguous outcomes retain the ownership journal and prohibit a blind
retry. No registry publication, global pruning or host configuration changes
occur. Actual `sandboxd`, `codex`, `claude` and `opencode` version commands and
installed grants are collected without network or credentials.

On a dedicated Windows release producer only, the optional
`--producer-wsl-cache-assistance` flag permits at most six transient cache
reclaims per build/export, spaced by at least 30 seconds, plus bounded reclaims
between effects. Each checks the same Docker Desktop engine and refuses foreign
running containers. It preserves all images, stopped containers and volumes.
Every action is logged and marked in the build receipt. This assistance is
never enabled by ordinary installed setup and is not a product resource pass.

Canonical input staging reads immutable Git blobs using the versioned shared
algorithm; it never relies on Windows CRLF checkout bytes. The normalized
Docker-save payload is tagless and binds config ID and uncompressed layer
digests. Package-root `licenses/runtime-image/` contains every runtime grant;
other application grants remain intact. Outer packages contain one payload.
The embedded Linux management core contains an explicit required-companion
marker instead of another copy; retained SSH/Docker installation must transfer
the verified companion or refuse before allocation. No automatic source build
fallback exists.

Later UI/docs changes may reuse the original payload only when all canonical
runtime bytes and inventory are identical. The package includes the original
raw manifest/build receipt, equal-input reuse evidence and a new strict clean
source receipt. Package identity v2 binds those files and the archive; old
packages retain v1 verification. Release manifests record both installed and
build revisions. The original producer output and logs must be retained.

Focused checks (fixtures/adapters, not Docker acceptance):

```sh
python -m unittest discover -s scripts/runtime-oci -p 'test_*.py' -v
```
