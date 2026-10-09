# Reproduction and artifact limits

Run `sha256sum -c SHA256SUMS` in this capsule to verify public member bytes. `original-bindings.json` keeps exact original small-file paths/hashes; original private binaries/images/fixtures are not supplied, so a clone cannot repeat their private-byte verification from these public receipts alone.

`helpers/build-package-next-v1` preserves the actually executed wrapper, original version, narrow parameterization patch and ten disposable controls. Running that wrapper is a new build with side effects: it needs the exact prior5f4 release directory, source299 on a fetched remote-tracking ref, the supported toolchain, an exclusive build window, adequate storage and distinct new output paths. Do not replay it now against the already produced299 release. The original helper paths are provenance, not an assertion that a fresh clone has the same cloud tools or private retained archives.

`helpers/relocate-historical425-v1` and `storage/older425-operation` describe a completed, root-authorized historical artifact placement. The script requires two ordinary same-inode old hardlinks and therefore should refuse replay after successful relocation. Never replay against private failed fixtures or substitute the current333 bootstrap/760 runtime. Restore old byte placement only through a separately reviewed exact hash-verified copy, with sufficient space and explicit topology/retention limitations; no generic deletion or prune is authorized here.

No package/archive binaries, raw private logs, state stores or credentials accompany this publication. The supported Windows development instructions elsewhere in the repository remain the route for isolated local testing; this Linux artifact-maintenance helper must not run on Dylan's shared Windows host.
