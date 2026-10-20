# Document prebuilt candidate finalizer

`finalize.py` is a maintainer-only, offline conversion of one retained bare
OCI tar from the `document-flat-oci-v1` assembler capture sink. The wire
magic, length, digest and trailer have already been consumed by the native
capture reader; they are not present in `capture.oci.partial`. The finalizer
requires independent SHA-256 pins
for the capture, the positive private assembly receipt, and a notice/source
closure. It refuses a reserved, unknown, failed, or unretired assembly. The
output is a candidate, not installation or distribution authority.

The notice closure is a bounded JSON object with `schema: 1`,
`kind: "luci-document-notice-source-closure-v1"`, the exact
`inputClosureSha256` and `sourceRetentionDigest`, and 1–512 `notices`. Each
notice names the retained `notices/` path, `sha256`, `bytes`, `component`,
`version`, `grant`, and `provenance`. The finalizer compares these claims to
the original embedded assembly manifest and actual layer bytes. The closure
itself must be separately reviewed and pinned; this script cannot establish
that a legal grant or source obligation is complete.

The candidate package contains `document-runtime-manifest-v1.json`,
`document-image-linux-amd64.tar`, and
`document-runtime-notices-v1.json`, and exact `notices/<relative path>`
sidecars for every retained notice. The candidate receipt lists each sidecar
path, SHA-256 and byte count so package staging can copy and verify it without
parsing the image archive. The exact first file needs an independent
release SHA-256 pin, supplied to `NewDocumentRuntimeInstaller`. Package staging
must preserve all three files. The finalizer receipt explicitly says
`releaseAuthority: "none"`.

The current source does not supply a completed assembler capture, notice
closure, signed release pin, package, image import, or runtime verification.
